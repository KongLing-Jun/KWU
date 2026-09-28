package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"kwu/desktop/internal/diagnostics"
	"kwu/desktop/internal/operationaldiag"
	"kwu/internal/agent"
	"kwu/internal/boot"
	"kwu/internal/brand"
	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/eventwire"
	"kwu/internal/expert"
	"kwu/internal/fileutil"
	"kwu/internal/modelbilling"
	"kwu/internal/project"
	"kwu/internal/provider"
	"kwu/internal/scene"
	"kwu/internal/userctx"
)

// --- WorkspaceTab -----------------------------------------------------------

// WorkspaceTab is one open conversation tab in the desktop. Each tab owns an
// independent controller (its own agent, session, tool registry, plugin host,
// memory, permissions) scoped to a workspace root, so multiple projects and
// topics can be active concurrently without interfering.
type WorkspaceTab struct {
	ID            string // stable random id
	Scope         string // "project" | "global"
	WorkspaceRoot string // project root dir (empty for global)
	SharedHostKey string // opaque key for the shared plugin host (set by buildTabController)
	TopicID       string // topic within the project
	TopicTitle    string // display title
	SessionPath   string // exact .jsonl file this tab continues
	// pendingCloudProjectID keeps a newly prepared automation session under its
	// logical Cloud Project before the authoritative ProjectContext pin can be
	// committed. It is runtime-only and cleared after the binding attempt.
	pendingCloudProjectID string
	sessionTurns          *int               // exact user-turn count for SessionPath; nil until proven
	Scene                 scene.Scene        // Desktop scene; unset only for legacy sessions
	SceneState            scene.State        // draft | locked | legacy
	ReadOnly              bool               // true for external channel transcripts opened for browsing
	Ctrl                  control.SessionAPI // nil while booting / on error
	Label                 string             // model label (for the tab badge)
	Ready                 bool               // true once boot.Build completes
	StartupErr            string             // build error, surfaced to the frontend
	// subscriptionAuthPending is runtime-only. Platform models remain visible
	// before login, but their controller is built only after cloud auth succeeds.
	subscriptionAuthPending bool
	startupErrorCode        modelbilling.ErrorCode
	startupFailureKind      tabStartupFailureKind
	startupDetail           string
	sink                    *tabEventSink // routes events with this tab's ID
	actor                   userctx.ActorContext
	buildCancel             context.CancelFunc // cancels in-flight boot for tabs removed before Ready
	buildGeneration         uint64             // identifies the current in-flight build
	removed                 bool               // set when the visible tab is pruned/closed before build completes

	ActivityStatus string // transient project-tree status for the in-flight turn
	activityTurnID string // runtime owner of ActivityStatus; guarded by App.mu

	// Per-turn autosave per tab.
	saveMu                    sync.Mutex
	saving                    bool
	saveAgain                 bool
	pendingArtifactDeliveries atomic.Pointer[artifactDeliveryQueue]

	// closing is set under saveMu when the tab is being torn down. Once set,
	// tabSnapshotLoop stops taking new snapshot work and CloseTab waits on
	// saveCond until any in-flight snapshot finishes - so no background
	// snapshot can write a session file back to disk after CloseTab returns.
	// Without this, deleting a just-closed session races that write and the
	// session "resurrects" (#4384).
	closing  bool
	saveCond *sync.Cond

	// readTelemetry tracks files read during this tab's session.
	readTelemetry  []readFileRecord
	usageTelemetry sessionUsageStats
	telemMu        sync.Mutex

	// plannerDisplay keeps display-only planner output for the in-flight turn.
	// The executor session remains the model-facing transcript; this sidecar
	// lets frontend history restore the planner cards after a rebuild/reload.
	plannerDisplay      []HistoryMessage
	plannerDisplayTools map[string]string
	plannerDisplayMu    sync.Mutex

	model            string // active model ref (for meta)
	billingSource    string // explicit future-task route; empty stays empty for legacy tabs
	effort           *string
	tokenMode        string
	mode             string // "normal" | "plan" | "yolo" | "plan-yolo"; yolo/full access is runtime-only
	goal             string
	toolApprovalMode string
	// toolApprovalModeKnown separates legacy sidecars with no posture from an
	// explicit Ask. Missing values run safely as Ask without being backfilled by
	// unrelated session metadata writes.
	toolApprovalModeKnown bool
	disabledMCP           map[string]ServerView
	mcpOrder              []string
}

type tabStartupFailureKind string

const (
	tabStartupFailureNone       tabStartupFailureKind = ""
	tabStartupFailureConfig     tabStartupFailureKind = "configuration"
	tabStartupFailureController tabStartupFailureKind = "controller"
	tabStartupFailureSession    tabStartupFailureKind = "session"
)

type TabSendAvailability struct {
	State     string `json:"state"`
	Retryable bool   `json:"retryable"`
	Detail    string `json:"detail,omitempty"`
	Action    string `json:"action,omitempty"`
}

const (
	tabSendStarting                  = "starting"
	tabSendReady                     = "ready"
	tabSendSubscriptionAuthRequired  = "subscription_auth_required"
	tabSendSubscriptionAccountAction = "subscription_account_action_required"
	tabSendBYOKConfigurationRequired = "byok_configuration_required"
	tabSendServiceUnavailable        = "service_unavailable"
	tabSendStartupFailed             = "startup_failed"
)

const (
	topicStatusThinking            = "thinking"
	topicStatusStreaming           = "streaming"
	topicStatusWaitingConfirmation = "waiting_confirmation"
	topicStatusBackgroundJob       = "background_job"
	topicStatusPaused              = "paused"
	topicStatusError               = "error"
)

type readFileRecord struct {
	Path      string `json:"path"`
	Turn      int    `json:"turn"`
	Time      int64  `json:"time"`
	Offset    int    `json:"offset,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

type sessionUsageStats struct {
	PromptTokens     int                         `json:"promptTokens"`
	CompletionTokens int                         `json:"completionTokens"`
	TotalTokens      int                         `json:"totalTokens"`
	ReasoningTokens  int                         `json:"reasoningTokens"`
	CacheHitTokens   int                         `json:"cacheHitTokens"`
	CacheMissTokens  int                         `json:"cacheMissTokens"`
	RequestCount     int                         `json:"requestCount"`
	ElapsedMs        int64                       `json:"elapsedMs"`
	SessionCost      float64                     `json:"sessionCost,omitempty"`
	SessionCurrency  string                      `json:"sessionCurrency,omitempty"`
	SessionCostUsd   float64                     `json:"sessionCostUsd,omitempty"`
	Sources          map[string]usageSourceStats `json:"sources,omitempty"`

	activeTurnStartedAt            int64
	activeTurnCompletionTokens     int
	activeTurnID                   string
	executorSessionCacheHitTokens  int
	executorSessionCacheMissTokens int
}

type usageSourceStats struct {
	PromptTokens     int     `json:"promptTokens"`
	CompletionTokens int     `json:"completionTokens"`
	TotalTokens      int     `json:"totalTokens"`
	ReasoningTokens  int     `json:"reasoningTokens"`
	CacheHitTokens   int     `json:"cacheHitTokens"`
	CacheMissTokens  int     `json:"cacheMissTokens"`
	RequestCount     int     `json:"requestCount"`
	SessionCost      float64 `json:"sessionCost,omitempty"`
	SessionCurrency  string  `json:"sessionCurrency,omitempty"`
	SessionCostUsd   float64 `json:"sessionCostUsd,omitempty"`
}

type tabTelemetrySnapshot struct {
	Version   int               `json:"version"`
	ReadFiles []readFileRecord  `json:"readFiles"`
	Usage     sessionUsageStats `json:"usage"`
}

func cloneStringPtr(v *string) *string {
	if v == nil {
		return nil
	}
	cp := *v
	return &cp
}

func cloneServerViewMap(in map[string]ServerView) map[string]ServerView {
	out := make(map[string]ServerView, len(in))
	for name, view := range in {
		view.EnvKeys = append([]string(nil), view.EnvKeys...)
		view.HeaderKeys = append([]string(nil), view.HeaderKeys...)
		out[name] = view
	}
	return out
}

func cloneUsageTelemetry(in sessionUsageStats) sessionUsageStats {
	out := in
	if len(in.Sources) > 0 {
		out.Sources = make(map[string]usageSourceStats, len(in.Sources))
		maps.Copy(out.Sources, in.Sources)
	}
	return out
}

func (t *WorkspaceTab) currentSessionPath() string {
	if t == nil {
		return ""
	}
	if t.Ctrl != nil {
		if path := strings.TrimSpace(t.Ctrl.SessionPath()); path != "" {
			return path
		}
	}
	return strings.TrimSpace(t.SessionPath)
}

func (t *WorkspaceTab) hasActiveRuntimeWork() bool {
	if t == nil || t.Ctrl == nil {
		return false
	}
	status := t.Ctrl.RuntimeStatus()
	return status.Running || status.Completing || status.PendingPrompt || status.BackgroundJobs > 0
}

func sessionRuntimeKey(path string) string {
	return canonicalTabSessionPath(path)
}

func detachedRuntimeTabID(key string) string {
	sum := sha256.Sum256([]byte(key))
	return "detached_" + hex.EncodeToString(sum[:8])
}

func (a *App) ensureDetachedSessionsLocked() {
	if a.detachedSessions == nil {
		a.detachedSessions = map[string]*WorkspaceTab{}
	}
}

func (a *App) runtimeTabsLocked() []*WorkspaceTab {
	seen := map[*WorkspaceTab]bool{}
	out := make([]*WorkspaceTab, 0, len(a.tabRuntimes)+len(a.detachedSessions))
	for _, tab := range a.tabRuntimes {
		if tab != nil && !seen[tab] {
			seen[tab] = true
			out = append(out, tab)
		}
	}
	for _, tab := range a.detachedSessions {
		if tab != nil && !seen[tab] {
			seen[tab] = true
			out = append(out, tab)
		}
	}
	return out
}

func (a *App) tabByEventSinkIDLocked(tabID string) *WorkspaceTab {
	if tab := a.tabRuntimes[tabID]; tab != nil {
		return tab
	}
	for _, tab := range a.detachedSessions {
		if tab != nil && tab.ID == tabID {
			return tab
		}
	}
	return nil
}

func (a *App) detachSessionRuntime(tab *WorkspaceTab) bool {
	if tab == nil {
		return false
	}
	a.mu.Lock()
	detached := a.detachSessionRuntimeLocked(tab)
	a.mu.Unlock()
	if detached && tab.sink != nil {
		tab.sink.clearContext()
	}
	return detached
}

// detachSessionRuntimeLocked publishes a background runtime before its visible
// route is removed. Callers that also delete tabRuntimes must do both while
// holding App.mu so terminal events can always resolve one of the two routes.
func (a *App) detachSessionRuntimeLocked(tab *WorkspaceTab) bool {
	if tab == nil {
		return false
	}
	key := sessionRuntimeKey(tab.currentSessionPath())
	return a.detachSessionRuntimeAtKeyLocked(tab, key)
}

func (a *App) detachSessionRuntimeAtKeyLocked(tab *WorkspaceTab, key string) bool {
	if tab == nil {
		return false
	}
	if key == "" {
		return false
	}
	a.ensureDetachedSessionsLocked()
	if existing := a.detachedSessions[key]; existing != nil && existing != tab {
		return false
	}
	tab.SessionPath = key
	a.detachedSessions[key] = tab
	return true
}

func cloneDetachedRuntimeTab(tab *WorkspaceTab, key string) *WorkspaceTab {
	if tab == nil {
		return nil
	}
	tab.telemMu.Lock()
	readTelemetry := append([]readFileRecord(nil), tab.readTelemetry...)
	usageTelemetry := cloneUsageTelemetry(tab.usageTelemetry)
	tab.telemMu.Unlock()
	tab.plannerDisplayMu.Lock()
	plannerDisplay := cloneHistoryMessages(tab.plannerDisplay)
	plannerDisplayTools := cloneStringMap(tab.plannerDisplayTools)
	tab.plannerDisplayMu.Unlock()

	return &WorkspaceTab{
		ID:                    detachedRuntimeTabID(key),
		Scope:                 tab.Scope,
		WorkspaceRoot:         tab.WorkspaceRoot,
		SharedHostKey:         tab.SharedHostKey,
		TopicID:               tab.TopicID,
		TopicTitle:            tab.TopicTitle,
		SessionPath:           key,
		Scene:                 tab.Scene,
		SceneState:            tab.SceneState,
		Ctrl:                  tab.Ctrl,
		Label:                 tab.Label,
		Ready:                 tab.Ready,
		StartupErr:            tab.StartupErr,
		sink:                  tab.sink,
		ActivityStatus:        tab.ActivityStatus,
		activityTurnID:        tab.activityTurnID,
		readTelemetry:         readTelemetry,
		usageTelemetry:        usageTelemetry,
		plannerDisplay:        plannerDisplay,
		plannerDisplayTools:   plannerDisplayTools,
		model:                 tab.model,
		billingSource:         tab.billingSource,
		effort:                cloneStringPtr(tab.effort),
		tokenMode:             tab.tokenMode,
		mode:                  tab.mode,
		goal:                  tab.goal,
		toolApprovalMode:      tab.toolApprovalMode,
		toolApprovalModeKnown: tab.toolApprovalModeKnown,
		disabledMCP:           cloneServerViewMap(tab.disabledMCP),
		mcpOrder:              append([]string(nil), tab.mcpOrder...),
	}
}

func (a *App) detachRuntimeForReplacement(tab *WorkspaceTab) bool {
	if tab == nil {
		return false
	}
	a.mu.RLock()
	if tab.removed || a.tabRuntimes[tab.ID] != tab {
		a.mu.RUnlock()
		return false
	}
	ctrl := tab.Ctrl
	fallbackPath := tab.SessionPath
	sink := tab.sink
	a.mu.RUnlock()
	currentPath := fallbackPath
	if ctrl != nil {
		if path := strings.TrimSpace(ctrl.SessionPath()); path != "" {
			currentPath = path
		}
	}
	key := sessionRuntimeKey(currentPath)
	if key == "" {
		return false
	}
	if sink != nil && !sink.beginRouteChangeWithin(runtimeAttachWaitTimeout) {
		return false
	}
	releaseRoute := func() {
		if sink != nil {
			sink.endRouteChange()
			sink = nil
		}
	}
	if !a.tryQuiesceTabAutosave(tab) {
		releaseRoute()
		return false
	}
	detached := cloneDetachedRuntimeTab(tab, key)
	if detached == nil {
		releaseRoute()
		return false
	}

	a.mu.Lock()
	validRuntime := detached.Ctrl != nil && tab.Ctrl == detached.Ctrl && tab.sink == sink && detached.sink == sink
	existingDetached := a.detachedSessions[key]
	if tab.removed || a.tabRuntimes[tab.ID] != tab || sessionRuntimeKey(tab.currentSessionPath()) != key || !validRuntime || existingDetached != nil {
		canResume := !tab.removed && a.tabRuntimes[tab.ID] == tab
		requested := false
		if canResume {
			requested = reopenTabAutosave(tab)
		}
		a.mu.Unlock()
		releaseRoute()
		if canResume && (requested || tab.hasPendingArtifactDeliveries()) {
			a.scheduleTabSnapshotForTab(tab)
		}
		return false
	}
	a.ensureDetachedSessionsLocked()
	tab.transferArtifactDeliveriesTo(detached)
	if detached.sink != nil {
		detached.sink.setTabID(detached.ID)
		detached.sink.clearContext()
	}
	a.detachedSessions[key] = detached
	// The controller now belongs exclusively to detached. Clearing it while
	// app.mu is held prevents a late schedule that retained the visible tab
	// pointer from snapshotting the same controller through a second saveMu.
	tab.Ctrl = nil
	requested := reopenTabAutosave(tab)
	a.mu.Unlock()
	releaseRoute()
	if requested || detached.hasPendingArtifactDeliveries() {
		a.scheduleTabSnapshotForTab(detached)
	}
	return true
}

// Production callers hold App.mu and the source sink's route-change gate, so
// publishing the new tab ID cannot split an in-flight Emit.
func applyRuntimeTab(target, source *WorkspaceTab, key string, wailsCtx context.Context, app *App) {
	if target == nil || source == nil {
		return
	}
	source.transferArtifactDeliveriesTo(target)
	source.telemMu.Lock()
	readTelemetry := append([]readFileRecord(nil), source.readTelemetry...)
	usageTelemetry := cloneUsageTelemetry(source.usageTelemetry)
	source.telemMu.Unlock()
	source.plannerDisplayMu.Lock()
	plannerDisplay := cloneHistoryMessages(source.plannerDisplay)
	plannerDisplayTools := cloneStringMap(source.plannerDisplayTools)
	source.plannerDisplayMu.Unlock()

	if source.sink != nil {
		source.sink.setTabID(target.ID)
		source.sink.app = app
		source.sink.setContext(wailsCtx)
	}

	target.Ctrl = source.Ctrl
	target.sink = source.sink
	target.SessionPath = key
	target.Scene = source.Scene
	target.SceneState = source.SceneState
	target.SharedHostKey = source.SharedHostKey
	target.Label = source.Label
	target.Ready = true
	clearTabStartupFailureLocked(target)
	target.ActivityStatus = source.ActivityStatus
	target.activityTurnID = source.activityTurnID
	target.model = source.model
	target.billingSource = source.billingSource
	target.effort = cloneStringPtr(source.effort)
	target.tokenMode = source.tokenMode
	target.mode = source.mode
	target.goal = source.goal
	target.toolApprovalMode = source.toolApprovalMode
	target.disabledMCP = cloneServerViewMap(source.disabledMCP)
	target.mcpOrder = append([]string(nil), source.mcpOrder...)
	target.readTelemetry = readTelemetry
	target.usageTelemetry = usageTelemetry
	target.plannerDisplay = plannerDisplay
	target.plannerDisplayTools = plannerDisplayTools
}

func (a *App) attachExistingSessionRuntime(tab *WorkspaceTab, path string, wailsCtx context.Context) bool {
	return a.waitForExistingSessionRuntimeAttach(tab, path, wailsCtx, 0) == runtimeAttachComplete
}

type runtimeAttachStatus uint8

const (
	runtimeAttachAbsent runtimeAttachStatus = iota
	runtimeAttachComplete
	runtimeAttachBusy
)

const runtimeAttachWaitTimeout = 5 * time.Second

func (a *App) waitForExistingSessionRuntimeAttach(tab *WorkspaceTab, path string, wailsCtx context.Context, generation uint64) runtimeAttachStatus {
	deadline := time.Now().Add(runtimeAttachWaitTimeout)
	for {
		status := a.attachExistingSessionRuntimeForBuild(tab, path, wailsCtx, generation)
		if status != runtimeAttachBusy || time.Now().After(deadline) || a.tabUnavailableForBuild(tab, generation) {
			return status
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (a *App) attachExistingSessionRuntimeForBuild(tab *WorkspaceTab, path string, wailsCtx context.Context, generation uint64) runtimeAttachStatus {
	key := sessionRuntimeKey(path)
	if tab == nil || key == "" {
		return runtimeAttachAbsent
	}

	a.mu.Lock()
	if tab.removed || a.tabRuntimes[tab.ID] != tab || (generation != 0 && tab.buildGeneration != generation) {
		a.mu.Unlock()
		return runtimeAttachAbsent
	}
	source := a.detachedSessions[key]
	fromDetached := source != nil
	if fromDetached {
		if !tabRuntimeActorsMatch(tab, source) {
			a.mu.Unlock()
			return runtimeAttachAbsent
		}
	} else {
		for _, candidate := range a.tabRuntimes {
			if candidate == nil || candidate == tab {
				continue
			}
			if sessionRuntimeKey(candidate.currentSessionPath()) == key {
				if !tabRuntimeActorsMatch(tab, candidate) {
					continue
				}
				source = candidate
				break
			}
		}
	}
	var sink *tabEventSink
	if source != nil {
		sink = source.sink
	}
	a.mu.Unlock()
	if source == nil {
		return runtimeAttachAbsent
	}
	if sink != nil && !sink.tryBeginRouteChange() {
		return runtimeAttachBusy
	}
	releaseRoute := func() {
		if sink != nil {
			sink.endRouteChange()
			sink = nil
		}
	}

	// Retire the source autosave writer before moving its controller and queue.
	// TurnDone can still publish while closing; scheduleTabSnapshotForTab records
	// that request in saveAgain and the hand-off schedules the target below.
	if !a.tryQuiesceTabAutosave(source) {
		releaseRoute()
		return runtimeAttachBusy
	}

	a.mu.Lock()
	validTarget := !tab.removed && a.tabRuntimes[tab.ID] == tab && (generation == 0 || tab.buildGeneration == generation)
	validSource := tabRuntimeActorsMatch(tab, source)
	if fromDetached {
		validSource = validSource && a.detachedSessions[key] == source && source.sink == sink
	} else {
		validSource = validSource && a.tabRuntimes[source.ID] == source && source.sink == sink && sessionRuntimeKey(source.currentSessionPath()) == key
	}
	if !validTarget || !validSource {
		canResume := !source.removed
		if fromDetached {
			canResume = canResume && a.detachedSessions[key] == source
		} else {
			canResume = canResume && a.tabRuntimes[source.ID] == source
		}
		requested := false
		if canResume {
			requested = reopenTabAutosave(source)
		}
		a.mu.Unlock()
		releaseRoute()
		if canResume && (requested || source.hasPendingArtifactDeliveries()) {
			a.scheduleTabSnapshotForTab(source)
		}
		return runtimeAttachBusy
	}
	if fromDetached {
		delete(a.detachedSessions, key)
	} else {
		delete(a.tabRuntimes, source.ID)
		a.removeTabOrderLocked(source.ID)
		if a.cachedActiveTabID() == source.ID {
			a.setActiveTabIDLocked(tab.ID)
		}
	}
	applyRuntimeTab(tab, source, key, wailsCtx, a)
	requested := consumeClosedTabSnapshotRequest(source)
	a.saveTabsLocked()
	a.mu.Unlock()
	releaseRoute()

	if requested || tab.hasPendingArtifactDeliveries() {
		a.scheduleTabSnapshotForTab(tab)
	}
	if tab.Ctrl != nil {
		tab.Ctrl.ReplayPendingPrompts()
	}
	return runtimeAttachComplete
}

func (t *WorkspaceTab) recordReadFile(rec readFileRecord) {
	t.telemMu.Lock()
	t.readTelemetry = append(t.readTelemetry, rec)
	t.telemMu.Unlock()
}

func (t *WorkspaceTab) recordTurnStarted(now int64) {
	t.recordTurnStartedFor("", now)
}

func (t *WorkspaceTab) recordTurnStartedFor(turnID string, now int64) {
	turnID = strings.TrimSpace(turnID)
	t.telemMu.Lock()
	if t.usageTelemetry.activeTurnStartedAt == 0 {
		t.usageTelemetry.activeTurnStartedAt = now
		t.usageTelemetry.activeTurnCompletionTokens = 0
		t.usageTelemetry.activeTurnID = turnID
	} else if turnID != "" && t.usageTelemetry.activeTurnID != "" && t.usageTelemetry.activeTurnID != turnID {
		if now >= t.usageTelemetry.activeTurnStartedAt {
			t.usageTelemetry.ElapsedMs += now - t.usageTelemetry.activeTurnStartedAt
		}
		t.usageTelemetry.activeTurnStartedAt = now
		t.usageTelemetry.activeTurnCompletionTokens = 0
		t.usageTelemetry.activeTurnID = turnID
	} else if turnID != "" && t.usageTelemetry.activeTurnID == "" {
		t.usageTelemetry.activeTurnID = turnID
	}
	t.telemMu.Unlock()
}

func (t *WorkspaceTab) recordTurnDone(now int64) {
	t.recordTurnDoneFor("", now)
}

func (t *WorkspaceTab) recordTurnDoneFor(turnID string, now int64) bool {
	turnID = strings.TrimSpace(turnID)
	t.telemMu.Lock()
	defer t.telemMu.Unlock()
	if turnID != "" && t.usageTelemetry.activeTurnID != "" && t.usageTelemetry.activeTurnID != turnID {
		return false
	}
	if started := t.usageTelemetry.activeTurnStartedAt; started > 0 && now >= started {
		t.usageTelemetry.ElapsedMs += now - started
	}
	t.usageTelemetry.activeTurnStartedAt = 0
	t.usageTelemetry.activeTurnCompletionTokens = 0
	t.usageTelemetry.activeTurnID = ""
	return true
}

func (t *WorkspaceTab) recordUsage(e event.Event) {
	if e.Usage == nil {
		return
	}
	u := e.Usage
	source := strings.TrimSpace(e.UsageSource)
	if source == "" {
		source = event.UsageSourceExecutor
	}
	t.telemMu.Lock()
	t.usageTelemetry.PromptTokens += u.PromptTokens
	t.usageTelemetry.CompletionTokens += u.CompletionTokens
	t.usageTelemetry.TotalTokens += u.TotalTokens
	t.usageTelemetry.ReasoningTokens += u.ReasoningTokens
	if t.usageTelemetry.activeTurnStartedAt > 0 &&
		(strings.TrimSpace(e.TurnID) == "" || t.usageTelemetry.activeTurnID == "" || t.usageTelemetry.activeTurnID == strings.TrimSpace(e.TurnID)) {
		t.usageTelemetry.activeTurnCompletionTokens += u.CompletionTokens
	}
	if source == event.UsageSourceExecutor && e.SessionHit+e.SessionMiss > 0 {
		if e.SessionHit < t.usageTelemetry.executorSessionCacheHitTokens || e.SessionMiss < t.usageTelemetry.executorSessionCacheMissTokens {
			t.usageTelemetry.CacheHitTokens += u.CacheHitTokens
			t.usageTelemetry.CacheMissTokens += u.CacheMissTokens
		} else {
			t.usageTelemetry.CacheHitTokens += e.SessionHit - t.usageTelemetry.executorSessionCacheHitTokens
			t.usageTelemetry.CacheMissTokens += e.SessionMiss - t.usageTelemetry.executorSessionCacheMissTokens
		}
		t.usageTelemetry.executorSessionCacheHitTokens = e.SessionHit
		t.usageTelemetry.executorSessionCacheMissTokens = e.SessionMiss
	} else {
		t.usageTelemetry.CacheHitTokens += u.CacheHitTokens
		t.usageTelemetry.CacheMissTokens += u.CacheMissTokens
	}
	t.usageTelemetry.RequestCount++
	if t.usageTelemetry.Sources == nil {
		t.usageTelemetry.Sources = map[string]usageSourceStats{}
	}
	src := t.usageTelemetry.Sources[source]
	src.PromptTokens += u.PromptTokens
	src.CompletionTokens += u.CompletionTokens
	src.TotalTokens += u.TotalTokens
	src.ReasoningTokens += u.ReasoningTokens
	src.CacheHitTokens += u.CacheHitTokens
	src.CacheMissTokens += u.CacheMissTokens
	src.RequestCount++
	if e.Pricing != nil {
		cost := e.Pricing.Cost(u)
		t.usageTelemetry.SessionCost += cost
		t.usageTelemetry.SessionCostUsd = t.usageTelemetry.SessionCost
		t.usageTelemetry.SessionCurrency = e.Pricing.Symbol()
		src.SessionCost += cost
		src.SessionCostUsd = src.SessionCost
		src.SessionCurrency = e.Pricing.Symbol()
	}
	t.usageTelemetry.Sources[source] = src
	t.telemMu.Unlock()
}

func (t *WorkspaceTab) contextUsageSnapshot() (sessionTokens int, activeTurnStartedAt int64, activeTurnCompletionTokens int) {
	t.telemMu.Lock()
	defer t.telemMu.Unlock()
	return t.usageTelemetry.TotalTokens, t.usageTelemetry.activeTurnStartedAt, t.usageTelemetry.activeTurnCompletionTokens
}

func (t *WorkspaceTab) telemetrySnapshot() tabTelemetrySnapshot {
	t.telemMu.Lock()
	defer t.telemMu.Unlock()
	records := make([]readFileRecord, len(t.readTelemetry))
	copy(records, t.readTelemetry)
	usage := t.usageTelemetry
	if started := usage.activeTurnStartedAt; started > 0 {
		now := time.Now().UnixMilli()
		if now >= started {
			usage.ElapsedMs += now - started
		}
	}
	if len(t.usageTelemetry.Sources) > 0 {
		usage.Sources = make(map[string]usageSourceStats, len(t.usageTelemetry.Sources))
		for source, stats := range t.usageTelemetry.Sources {
			usage.Sources[source] = stats
		}
	}
	usage.activeTurnStartedAt = 0
	usage.activeTurnCompletionTokens = 0
	usage.activeTurnID = ""
	usage.executorSessionCacheHitTokens = 0
	usage.executorSessionCacheMissTokens = 0
	return tabTelemetrySnapshot{Version: 2, ReadFiles: records, Usage: usage}
}

func (t *WorkspaceTab) resetTelemetry() {
	t.telemMu.Lock()
	t.readTelemetry = nil
	t.usageTelemetry = sessionUsageStats{}
	t.telemMu.Unlock()
}

func (t *WorkspaceTab) resetSessionRuntimeSidecars() {
	if t == nil {
		return
	}
	// Artifact batches freeze their owning session path and workspace root. Keep
	// them across a rebind so a failed or not-yet-started snapshot can retry
	// without attributing the files to the newly active session.
	t.resetTelemetry()
	t.plannerDisplayMu.Lock()
	t.plannerDisplay = nil
	t.plannerDisplayTools = nil
	t.plannerDisplayMu.Unlock()
}

func (t *WorkspaceTab) resetPlannerDisplayTurn() {
	t.plannerDisplayMu.Lock()
	if len(t.plannerDisplay) == 0 {
		t.plannerDisplayTools = nil
	}
	t.plannerDisplayMu.Unlock()
}

func (t *WorkspaceTab) recordPlannerDisplayEvent(e event.Event) {
	if strings.TrimSpace(e.Source) != event.UsageSourcePlanner {
		return
	}
	t.plannerDisplayMu.Lock()
	defer t.plannerDisplayMu.Unlock()
	switch e.Kind {
	case event.Phase:
		if strings.TrimSpace(e.Text) != "" {
			t.plannerDisplay = append(t.plannerDisplay, HistoryMessage{Role: "phase", Content: e.Text})
		}
	case event.Reasoning:
		if e.Text != "" {
			hm := t.ensurePlannerAssistantLocked()
			hm.Reasoning += e.Text
		}
	case event.Text:
		if e.Text != "" {
			hm := t.ensurePlannerAssistantLocked()
			hm.Content += e.Text
		}
	case event.Message:
		if e.Text != "" || e.Reasoning != "" || len(e.MemoryCitations) > 0 {
			hm := t.ensurePlannerAssistantLocked()
			if e.Text != "" {
				hm.Content = e.Text
			}
			if e.Reasoning != "" {
				hm.Reasoning = e.Reasoning
			}
			if len(e.MemoryCitations) > 0 {
				hm.MemoryCitations = append([]provider.MemoryCitation(nil), e.MemoryCitations...)
			}
		}
	case event.ToolDispatch:
		if e.Tool.Partial || strings.TrimSpace(e.Tool.Name) == "" {
			return
		}
		hm := t.ensurePlannerAssistantForToolLocked()
		call := HistoryToolCall{
			ID:        e.Tool.ID,
			Name:      e.Tool.Name,
			Arguments: e.Tool.Args,
			Subject:   historyToolSubject(e.Tool.Name, e.Tool.Args),
			Summary:   historyToolSummary(e.Tool.Name, e.Tool.Args, ""),
			Diff:      e.Tool.Diff,
			Added:     e.Tool.Added,
			Removed:   e.Tool.Removed,
		}
		replaced := false
		if call.ID != "" {
			for i := range hm.ToolCalls {
				if hm.ToolCalls[i].ID == call.ID {
					hm.ToolCalls[i] = call
					replaced = true
					break
				}
			}
			if t.plannerDisplayTools == nil {
				t.plannerDisplayTools = map[string]string{}
			}
			t.plannerDisplayTools[call.ID] = call.Name
		}
		if !replaced {
			hm.ToolCalls = append(hm.ToolCalls, call)
		}
	case event.ToolResult:
		callID := strings.TrimSpace(e.Tool.ID)
		content := firstNonEmpty(e.Tool.Output, e.Tool.Err)
		display, errPreview := plannerToolResultDisplay(content, e.Tool.Err != "")
		if callID != "" {
			updateHistoryToolCallSummary(t.plannerDisplay, callID, content)
		}
		toolName := e.Tool.Name
		if toolName == "" && t.plannerDisplayTools != nil {
			toolName = t.plannerDisplayTools[callID]
		}
		t.plannerDisplay = append(t.plannerDisplay, HistoryMessage{
			Role:            "tool",
			ToolCallID:      callID,
			ToolName:        toolName,
			Content:         display,
			ToolResultError: errPreview,
		})
	case event.Notice:
		if strings.TrimSpace(e.Text) != "" {
			level := "info"
			if e.Level == event.LevelWarn {
				level = "warn"
			}
			t.plannerDisplay = append(t.plannerDisplay, HistoryMessage{Role: "notice", Level: level, Content: e.Text})
		}
	}
}

func (t *WorkspaceTab) ensurePlannerAssistantLocked() *HistoryMessage {
	if n := len(t.plannerDisplay); n > 0 && t.plannerDisplay[n-1].Role == "assistant" {
		return &t.plannerDisplay[n-1]
	}
	t.plannerDisplay = append(t.plannerDisplay, HistoryMessage{Role: "assistant"})
	return &t.plannerDisplay[len(t.plannerDisplay)-1]
}

func (t *WorkspaceTab) ensurePlannerAssistantForToolLocked() *HistoryMessage {
	if n := len(t.plannerDisplay); n > 0 && t.plannerDisplay[n-1].Role == "assistant" && strings.TrimSpace(t.plannerDisplay[n-1].Content) == "" {
		return &t.plannerDisplay[n-1]
	}
	t.plannerDisplay = append(t.plannerDisplay, HistoryMessage{Role: "assistant"})
	return &t.plannerDisplay[len(t.plannerDisplay)-1]
}

func plannerToolResultDisplay(content string, failed bool) (display, errPreview string) {
	if strings.TrimSpace(content) == "" {
		return "", ""
	}
	if failed || historyToolResultFailed(content) {
		display = clipHistoryToolPreview(strings.TrimSpace(content))
		return display, display
	}
	return "", ""
}

func (t *WorkspaceTab) takePlannerDisplayTurn() []HistoryMessage {
	t.plannerDisplayMu.Lock()
	defer t.plannerDisplayMu.Unlock()
	out := cloneHistoryMessages(t.plannerDisplay)
	t.plannerDisplay = nil
	t.plannerDisplayTools = nil
	return out
}

// tabEventSink wraps a parent event.Sink and prepends a tabId to every wire
// event so the frontend can route it to the correct tab's reducer.
type tabEventSink struct {
	tabID           string
	routeTabID      atomic.Pointer[string]
	routeGate       atomic.Uint64
	app             *App
	mu              sync.RWMutex
	ctx             context.Context
	runtimeEvents   asyncRuntimeEmitter
	operationalDiag atomic.Pointer[operationaldiag.Trace]
	operationalMu   sync.Mutex
	identityMu      sync.Mutex
	identity        tabEventIdentityState
}

const tabEventRouteSwitching uint64 = 1 << 63

// beginEmitRoute pins the sink's route for one complete Emit call. Route
// changes publish the switching bit first, so an event either finishes on the
// old owner or starts on the new owner; it cannot observe both.
func (s *tabEventSink) beginEmitRoute() string {
	if s == nil {
		return ""
	}
	for {
		state := s.routeGate.Load()
		if state&tabEventRouteSwitching != 0 {
			time.Sleep(time.Millisecond)
			continue
		}
		if s.routeGate.CompareAndSwap(state, state+1) {
			return s.currentTabID()
		}
	}
}

func (s *tabEventSink) endEmitRoute() {
	if s != nil {
		s.routeGate.Add(^uint64(0))
	}
}

// tryBeginRouteChange takes the route exclusively only when no Emit call is in
// flight. Callers retry outside App.mu, so a stalled event cannot deadlock a
// route transition while holding the application lock.
func (s *tabEventSink) tryBeginRouteChange() bool {
	if s == nil {
		return true
	}
	return s.routeGate.CompareAndSwap(0, tabEventRouteSwitching)
}

func (s *tabEventSink) beginRouteChangeWithin(timeout time.Duration) bool {
	if s == nil {
		return true
	}
	deadline := time.Now().Add(timeout)
	for {
		state := s.routeGate.Load()
		if state&tabEventRouteSwitching == 0 && s.routeGate.CompareAndSwap(state, state|tabEventRouteSwitching) {
			for {
				state = s.routeGate.Load()
				if state&^tabEventRouteSwitching == 0 {
					return true
				}
				if timeout <= 0 || !time.Now().Before(deadline) {
					// Preserve every admitted reader. Each reader will still subtract
					// its own count after the writer preference bit is withdrawn.
					for {
						state = s.routeGate.Load()
						if s.routeGate.CompareAndSwap(state, state&^tabEventRouteSwitching) {
							return false
						}
					}
				}
				time.Sleep(time.Millisecond)
			}
		}
		if timeout <= 0 || !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(time.Millisecond)
	}
}

func (s *tabEventSink) endRouteChange() {
	if s != nil {
		s.routeGate.Store(0)
	}
}

func (s *tabEventSink) currentTabID() string {
	if s == nil {
		return ""
	}
	if tabID := s.routeTabID.Load(); tabID != nil {
		return *tabID
	}
	return s.tabID
}

func (s *tabEventSink) setTabID(tabID string) {
	value := tabID
	s.routeTabID.Store(&value)
}

func (s *tabEventSink) Emit(e event.Event) {
	tabID := s.beginEmitRoute()
	defer s.endEmitRoute()
	snapshotScheduled := false
	currentTurnDone := true
	e = s.identifyAgentEvent(e)
	e = s.scopeOperationalEvent(e)
	s.traceAgentDelivery("agent_event", e)
	if s.app != nil && e.Kind == event.TurnDone && e.Turn != nil {
		s.app.observeTurnToolCharges(*e.Turn)
	}
	if s.app != nil && e.Kind == event.ToolResult && e.Tool.Err == "" && e.Tool.Name == shopifyCatalogSearchTool {
		tool := e.Tool
		go s.app.projectCapabilityMCPToolResult(tabID, tool)
	}
	if s.app != nil {
		switch e.Kind {
		case event.TurnStarted:
			s.resetPlannerDisplayTurn()
			s.recordTurnStarted(e.TurnID)
		case event.Usage:
			s.recordUsageTelemetry(e)
		case event.TurnDone:
			if e.Err != nil || e.Interrupted {
				e.ArtifactDelivery = nil
			}
			currentTurnDone, snapshotScheduled = s.recordTurnDoneAt(
				time.Now().UnixMilli(),
				e.TurnID,
				e.ArtifactDelivery,
				e.OriginSessionPath,
				e.OriginWorkspaceRoot,
				e.Err == nil && !e.Interrupted,
			)
		}
		if m := s.app.metrics.Load(); m != nil {
			m.observe(e)
			if e.Kind == event.TurnDone {
				m.persist()
			}
		}
	}
	wire := toWireTab(e, tabID)
	s.traceAgentDelivery("tab_event_sink", e)
	if e.Kind == event.TurnStarted || e.Kind == event.TurnDone {
		route := s.conversationRoute()
		if e.Kind == event.TurnDone {
			if path := strings.TrimSpace(e.OriginSessionPath); path != "" {
				route.sessionPath = sessionRuntimeKey(path)
			}
			if root := strings.TrimSpace(e.OriginWorkspaceRoot); root != "" {
				route.workspaceRoot = root
			}
		}
		wire.ConversationScope = route.scope
		wire.ConversationWorkspaceRoot = route.workspaceRoot
		wire.ConversationTopicID = route.topicID
		wire.ConversationSessionPath = route.sessionPath
		if s.app != nil {
			s.app.emitRuntimeEvent(conversationActivityEventChannel, wire)
		}
	}
	if e.Kind == event.TurnStarted || e.Kind == event.Usage {
		if tab, _ := s.telemetryTab(); tab != nil {
			_, wire.ActiveTurnStartedAt, wire.ActiveTurnCompletionTokens = tab.contextUsageSnapshot()
		}
	}
	s.traceAgentDelivery("wails_agent_event", e)
	wire = s.persistTranscriptEvent(wire)
	if e.Kind == event.Operational {
		s.emitOperationalRuntimeEvent(e, wire)
	} else {
		s.emitRuntimeEvent(eventChannel, wire)
	}
	if s.app != nil {
		if status, update := topicActivityStatusFromEvent(e); update {
			changed, applied := s.app.setTabActivityStatusFromEvent(tabID, e, status)
			if applied && (changed || isBackgroundJobLifecycleNotice(e)) {
				s.app.emitProjectTreeChanged()
			}
		}
	}
	// Record read_file successes in the tab's telemetry.
	if e.Kind == event.ToolResult && e.Tool.Name == "read_file" && e.Tool.Err == "" {
		s.recordReadTelemetry(e)
	}
	if s.app != nil {
		s.recordPlannerDisplay(e)
	}
	// Persist after each turn so a force-kill loses at most the in-flight prompt.
	if e.Kind == event.TurnDone && s.app != nil {
		if currentTurnDone {
			s.flushPlannerDisplay()
		}
		if !snapshotScheduled {
			s.scheduleSnapshot()
		}
	}
}

func (s *tabEventSink) scheduleSnapshot() {
	if s == nil || s.app == nil {
		return
	}
	// Read the mutable sink route and claim its autosave while the same App read
	// lock excludes detach/attach from moving the controller between tabs.
	s.app.mu.RLock()
	tab := s.app.tabByEventSinkIDLocked(s.currentTabID())
	s.app.scheduleTabSnapshotForTab(tab)
	s.app.mu.RUnlock()
}

type wireConversationRoute struct {
	scope         string
	workspaceRoot string
	topicID       string
	sessionPath   string
}

func (s *tabEventSink) conversationRoute() wireConversationRoute {
	if s == nil || s.app == nil {
		return wireConversationRoute{}
	}
	s.app.mu.RLock()
	tab := s.app.tabByEventSinkIDLocked(s.currentTabID())
	if tab == nil {
		s.app.mu.RUnlock()
		return wireConversationRoute{}
	}
	route := wireConversationRoute{
		scope:         tab.Scope,
		workspaceRoot: tab.WorkspaceRoot,
		topicID:       tab.TopicID,
		sessionPath:   tab.SessionPath,
	}
	ctrl := tab.Ctrl
	s.app.mu.RUnlock()
	if ctrl != nil {
		if path := ctrl.SessionPath(); path != "" {
			route.sessionPath = path
		}
	}
	route.sessionPath = sessionRuntimeKey(route.sessionPath)
	return route
}

func (s *tabEventSink) setContext(ctx context.Context) {
	s.mu.Lock()
	s.ctx = ctx
	s.mu.Unlock()
	s.runtimeEvents.BeginConnectionEpoch()
}

func (s *tabEventSink) clearContext() {
	s.mu.Lock()
	s.ctx = nil
	s.mu.Unlock()
	s.runtimeEvents.Clear()
}

func (s *tabEventSink) context() context.Context {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ctx
}

func (s *tabEventSink) emitRuntimeEvent(name string, payload ...interface{}) {
	if s == nil {
		return
	}
	ctx := s.context()
	if ctx == nil {
		return
	}
	s.runtimeEvents.Emit(ctx, name, payload...)
}

type runtimeEventEmitFunc func(context.Context, string, ...interface{})

type runtimeEventEnvelope struct {
	ctx      context.Context
	name     string
	payload  []interface{}
	epoch    uint64
	control  runtimeControlEnvelopeMetadata
	queuedAt time.Time
}

// asyncRuntimeEmitter decouples Wails' runtime event bridge from agent
// emission. runtime.EventsEmit can block when the single webview event channel
// backs up; callers enqueue in-order work and return without holding the
// agent's event.Sync lock.
type asyncRuntimeEmitter struct {
	mu                 sync.Mutex
	emit               runtimeEventEmitFunc
	queue              []runtimeEventEnvelope
	head               int
	controlQueue       []runtimeEventEnvelope
	controlHead        int
	controlCapacity    int
	epoch              atomic.Uint64
	activeControlEpoch atomic.Uint64
	controlCounters    runtimeControlCounterState
	running            bool
}

func (e *asyncRuntimeEmitter) Emit(ctx context.Context, name string, payload ...interface{}) {
	if ctx == nil {
		return
	}
	e.mu.Lock()
	// Wails' EventsEmit terminates the process when given a context that did
	// not originate from its lifecycle hooks. Unit-test apps intentionally use
	// ordinary contexts, so only queue those when a test emitter is installed.
	if e.emit == nil && ctx.Value("events") == nil {
		e.mu.Unlock()
		return
	}
	item := runtimeEventEnvelope{
		ctx:     ctx,
		name:    name,
		payload: append([]interface{}(nil), payload...),
		epoch:   e.connectionEpoch(),
	}
	e.queue = append(e.queue, item)
	if !e.running {
		e.running = true
		go e.run()
	}
	e.mu.Unlock()
}

func (e *asyncRuntimeEmitter) Clear() {
	e.BeginConnectionEpoch()
	e.mu.Lock()
	droppedControl := len(e.controlQueue) - e.controlHead
	clear(e.queue)
	e.queue = nil
	e.head = 0
	clear(e.controlQueue)
	e.controlQueue = nil
	e.controlHead = 0
	e.controlCounters.depth.Store(0)
	e.mu.Unlock()
	if droppedControl > 0 {
		e.controlCounters.droppedOnDisconnect.Add(uint64(droppedControl))
	}
}

func (e *asyncRuntimeEmitter) run() {
	for {
		e.mu.Lock()
		item, control, ok := e.nextRuntimeEventLocked()
		if !ok {
			clear(e.queue)
			e.queue = nil
			e.head = 0
			clear(e.controlQueue)
			e.controlQueue = nil
			e.controlHead = 0
			e.running = false
			e.mu.Unlock()
			return
		}
		if control {
			if item.epoch != e.epoch.Load() {
				e.mu.Unlock()
				e.controlCounters.late.Add(1)
				continue
			}
			e.activeControlEpoch.Store(item.epoch + 1)
			if item.epoch != e.epoch.Load() {
				e.activeControlEpoch.Store(0)
				e.mu.Unlock()
				e.controlCounters.late.Add(1)
				continue
			}
		}
		emit := e.emit
		if emit == nil {
			emit = runtime.EventsEmit
		}
		e.mu.Unlock()

		emitted := callRuntimeEmitter(emit, item)
		if control {
			e.controlCounters.observeDelivery(time.Since(item.queuedAt))
			e.activeControlEpoch.Store(0)
			if emitted {
				e.controlCounters.emitted.Add(1)
			} else {
				e.controlCounters.emitFailed.Add(1)
			}
		}
	}
}

func topicActivityStatusFromEvent(e event.Event) (string, bool) {
	switch e.Kind {
	case event.TurnStarted, event.Reasoning, event.ToolDispatch, event.ToolProgress, event.ToolResult, event.CompactionStarted, event.CompactionDone, event.Retrying:
		return topicStatusThinking, true
	case event.Text, event.Message:
		return topicStatusStreaming, true
	case event.ApprovalRequest, event.AskRequest:
		return topicStatusWaitingConfirmation, true
	case event.TurnDone:
		if e.Err != nil {
			return topicStatusError, true
		}
		return "", true
	case event.Notice:
		if isBackgroundJobLifecycleNotice(e) {
			return "", true
		}
		return "", false
	default:
		return "", false
	}
}

func isBackgroundJobLifecycleNotice(e event.Event) bool {
	if e.Kind != event.Notice {
		return false
	}
	text := strings.TrimSpace(e.Text)
	return strings.HasPrefix(text, "background ") &&
		(strings.Contains(text, " started: ") ||
			strings.Contains(text, " finished: ") ||
			strings.Contains(text, " failed: ") ||
			strings.Contains(text, " killed: "))
}

func (a *App) emitReady(ctx context.Context) {
	a.mu.RLock()
	hook := a.readyHook
	a.mu.RUnlock()
	if hook != nil {
		hook()
		return
	}
	if ctx != nil {
		runtime.EventsEmit(ctx, "agent:ready")
	}
}

func (s *tabEventSink) recordReadTelemetry(e event.Event) {
	if s.app == nil {
		return
	}
	s.app.mu.RLock()
	tab := s.app.tabByEventSinkIDLocked(s.currentTabID())
	var ctrl control.SessionAPI
	if tab != nil {
		ctrl = tab.Ctrl
	}
	s.app.mu.RUnlock()
	if tab == nil {
		return
	}
	turn := 0
	if ctrl != nil {
		turn = ctrl.Turn()
	}

	// Parse read_file args: {"path": "...", "offset": N, "limit": N}
	var args struct {
		Path   string `json:"path"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
	}
	path := e.Tool.Args
	offset := 0
	limit := 0
	if err := json.Unmarshal([]byte(e.Tool.Args), &args); err == nil && args.Path != "" {
		path = args.Path
		offset = args.Offset
		limit = args.Limit
	}

	truncated := e.Tool.Truncated || strings.Contains(e.Tool.Output, "truncated") ||
		strings.Contains(e.Tool.Output, "File truncated")

	tab.recordReadFile(readFileRecord{
		Path:      path,
		Turn:      turn,
		Time:      time.Now().UnixMilli(),
		Offset:    offset,
		Limit:     limit,
		Truncated: truncated,
	})
	if ctrl == nil {
		return
	}
	if sp := ctrl.SessionPath(); sp != "" {
		_ = saveTelemetry(sp+".telemetry.json", tab.telemetrySnapshot())
	}
}

func (s *tabEventSink) recordTurnStarted(turnID string) {
	tab, sp := s.telemetryTab()
	if tab == nil {
		return
	}
	tab.recordTurnStartedFor(turnID, time.Now().UnixMilli())
	if sp != "" {
		_ = saveTelemetry(sp+".telemetry.json", tab.telemetrySnapshot())
	}
}

func (s *tabEventSink) recordTurnDone() {
	_, _ = s.recordTurnDoneAt(time.Now().UnixMilli(), "", nil, "", "", true)
}

func (s *tabEventSink) recordTurnDoneAt(
	now int64,
	turnID string,
	delivery *provider.ArtifactDelivery,
	originSessionPath string,
	originWorkspaceRoot string,
	captureLegacy bool,
) (current, snapshotScheduled bool) {
	if s == nil || s.app == nil {
		return false, false
	}
	// Keep the first route lookup, delivery capture, queue publication, and
	// writer claim behind one App read-lock barrier. Permanent teardown cannot
	// remove the runtime in the gap between observing TurnDone and recording its
	// final deliverables.
	s.app.mu.RLock()
	tabID := s.currentTabID()
	tab := s.app.tabByEventSinkIDLocked(tabID)
	if tab == nil {
		s.app.mu.RUnlock()
		return false, false
	}
	ctrl := tab.Ctrl
	sp := ""
	if ctrl != nil {
		sp = ctrl.SessionPath()
	}
	current = tab.recordTurnDoneFor(turnID, now)
	if ctrl != nil {
		sessionPath := strings.TrimSpace(originSessionPath)
		workspaceRoot := strings.TrimSpace(originWorkspaceRoot)
		if sessionPath == "" {
			sessionPath = ctrl.SessionPath()
		}
		if workspaceRoot == "" {
			workspaceRoot = ctrl.WorkspaceRoot()
		}
		if batch, ok := artifactDeliveryBatchFromContract(sessionPath, workspaceRoot, delivery); ok {
			tab.enqueueArtifactDelivery(batch)
			// Claim the writer before releasing the route barrier. Teardown paths
			// can then quiesce this exact tab even if Emit has not reached its
			// normal end-of-turn autosave yet.
			s.app.scheduleTabSnapshotForTab(tab)
			snapshotScheduled = true
		} else if delivery == nil && captureLegacy && current {
			if batch, ok := captureArtifactDelivery(ctrl, now); ok {
				tab.enqueueArtifactDelivery(batch)
				s.app.scheduleTabSnapshotForTab(tab)
				snapshotScheduled = true
			}
		}
	}
	s.app.mu.RUnlock()
	if !current {
		if sp != "" {
			_ = saveTelemetry(sp+".telemetry.json", tab.telemetrySnapshot())
		}
		return false, snapshotScheduled
	}
	s.app.refreshTabSessionTurns(tabID)
	// Project binding is committed while the session is still blank. That first
	// tree refresh therefore caches no SessionInfo (empty transcripts are
	// intentionally excluded). Persist the completed first turn's listing fields
	// synchronously and invalidate the tree cache so the session cannot be
	// rendered as its physical global topic until a later refresh/restart.
	if sp != "" && ctrl != nil {
		if meta, ok, err := agent.LoadBranchMeta(sp); err == nil && ok && meta.IsCloudProjectBound() {
			preview, turns := agent.SessionPreviewFromMessages(ctrl.History())
			if err := agent.UpdateSessionMeta(sp, "", preview, turns, true); err != nil {
				slog.Warn("desktop: persist cloud project turn listing", diagnostics.FailureMarkerKey, diagnostics.FailureMarker, "scope", "session.meta.project_turn", "session_path", sp, "err", err)
			} else {
				invalidateTopicSessionIndexForPath(sp)
				s.app.emitProjectTreeChanged()
			}
		}
	}
	if sp != "" {
		_ = saveTelemetry(sp+".telemetry.json", tab.telemetrySnapshot())
	}
	return true, snapshotScheduled
}

func (s *tabEventSink) recordUsageTelemetry(e event.Event) {
	tab, sp := s.telemetryTab()
	if tab == nil {
		return
	}
	tab.recordUsage(e)
	if sp != "" {
		_ = saveTelemetry(sp+".telemetry.json", tab.telemetrySnapshot())
	}
}

func (s *tabEventSink) resetPlannerDisplayTurn() {
	tab, _ := s.eventTabAndController()
	if tab != nil {
		tab.resetPlannerDisplayTurn()
	}
}

func (s *tabEventSink) recordPlannerDisplay(e event.Event) {
	tab, _ := s.eventTabAndController()
	if tab != nil {
		tab.recordPlannerDisplayEvent(e)
	}
}

func (s *tabEventSink) flushPlannerDisplay() {
	tab, ctrl := s.eventTabAndController()
	if tab == nil || ctrl == nil {
		return
	}
	messages := tab.takePlannerDisplayTurn()
	if len(messages) == 0 {
		return
	}
	sessionPath := ctrl.SessionPath()
	if sessionPath == "" {
		return
	}
	userContent := lastUserMessageContent(ctrl.History())
	if strings.TrimSpace(userContent) == "" {
		return
	}
	if err := recordSessionPlannerDisplay(controllerSessionDir(ctrl), sessionPath, userContent, messages); err != nil {
		recordDesktopBackgroundFailure("session.planner_display.persist", err)
	}
}

func (s *tabEventSink) eventTabAndController() (*WorkspaceTab, control.SessionAPI) {
	if s.app == nil {
		return nil, nil
	}
	s.app.mu.RLock()
	defer s.app.mu.RUnlock()
	tab := s.app.tabByEventSinkIDLocked(s.currentTabID())
	if tab == nil {
		return nil, nil
	}
	return tab, tab.Ctrl
}

func lastUserMessageContent(msgs []provider.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == provider.RoleUser {
			return msgs[i].Content
		}
	}
	return ""
}

func (s *tabEventSink) telemetryTab() (*WorkspaceTab, string) {
	if s.app == nil {
		return nil, ""
	}
	s.app.mu.RLock()
	tab := s.app.tabByEventSinkIDLocked(s.currentTabID())
	var ctrl control.SessionAPI
	if tab != nil {
		ctrl = tab.Ctrl
	}
	s.app.mu.RUnlock()
	if tab == nil {
		return nil, ""
	}
	if ctrl == nil {
		return tab, ""
	}
	sp := ctrl.SessionPath()
	if sp == "" {
		return tab, ""
	}
	return tab, sp
}

// --- wire event with tab ----------------------------------------------------

func toWireTab(e event.Event, tabID string) wireEventTab {
	w := eventwire.ToWire(e)
	if e.Kind == event.ToolResult && e.Tool.Name == deliverFilesToolName && e.Tool.Err == "" && w.Tool != nil {
		output := e.Tool.RawOutput
		if output == "" {
			output = e.Tool.Output
		}
		if paths, ok := decodeDeliverFilesResult(output); ok {
			w.Tool.ArtifactPaths = paths
		}
	}
	return wireEventTab{
		Event:             w,
		TabID:             tabID,
		SessionHitTokens:  e.SessionHit,
		SessionMissTokens: e.SessionMiss,
		SessionCost:       0, // filled by frontend accumulator per tab
		SessionCurrency:   "",
		SessionCostUsd:    0, // deprecated compatibility alias
	}
}

// wireEventTab extends the shared event wire with tab routing info. The frontend reducer
// uses tabId to dispatch to the correct per-tab state.
type wireEventTab struct {
	eventwire.Event
	TranscriptJournalID string          `json:"transcriptJournalId,omitempty"`
	TranscriptSequence  int64           `json:"transcriptSequence,omitempty"`
	TranscriptUser      *HistoryMessage `json:"transcriptUser,omitempty"`
	TabID               string          `json:"tabId"`
	// Stable conversation routing survives a running session being detached
	// from the visible tab list, allowing client-side read state to follow it.
	ConversationScope         string `json:"conversationScope,omitempty"`
	ConversationWorkspaceRoot string `json:"conversationWorkspaceRoot,omitempty"`
	ConversationTopicID       string `json:"conversationTopicId,omitempty"`
	ConversationSessionPath   string `json:"conversationSessionPath,omitempty"`
	// Active-turn metrics let the frontend use the backend timestamp as the
	// turn identity and merge live usage with later hydration snapshots.
	ActiveTurnStartedAt        int64 `json:"activeTurnStartedAt,omitempty"`
	ActiveTurnCompletionTokens int   `json:"activeTurnCompletionTokens,omitempty"`
	// Session-cumulative tokens per tab.
	SessionHitTokens  int `json:"sessionHitTokens,omitempty"`
	SessionMissTokens int `json:"sessionMissTokens,omitempty"`
	// SessionCost is filled by the frontend's per-tab accumulator.
	SessionCost     float64 `json:"sessionCost,omitempty"`
	SessionCurrency string  `json:"sessionCurrency,omitempty"`
	// SessionCostUsd is a deprecated compatibility alias. It mirrors
	// SessionCost and does not imply USD.
	SessionCostUsd float64 `json:"sessionCostUsd,omitempty"`
}

// --- Tab management on App --------------------------------------------------

// TabMeta is the frontend-facing shape of one tab.
type TabMeta struct {
	ID                         string              `json:"id"`
	Scope                      string              `json:"scope"`
	WorkspaceRoot              string              `json:"workspaceRoot"`
	WorkspaceName              string              `json:"workspaceName"`
	WorkspacePath              string              `json:"workspacePath,omitempty"`
	GitBranch                  string              `json:"gitBranch,omitempty"`
	TopicID                    string              `json:"topicId"`
	TopicTitle                 string              `json:"topicTitle"`
	SessionPath                string              `json:"sessionPath,omitempty"`
	SessionTurns               *int                `json:"sessionTurns,omitempty"`
	Scene                      string              `json:"scene,omitempty"`
	SceneState                 string              `json:"sceneState,omitempty"`
	ReadOnly                   bool                `json:"readOnly,omitempty"`
	ProjectColor               string              `json:"projectColor,omitempty"`
	Label                      string              `json:"label"`
	ModelRef                   string              `json:"modelRef,omitempty"`
	BillingSource              string              `json:"billingSource,omitempty"`
	Effort                     *string             `json:"effort,omitempty"`
	ExpertID                   string              `json:"expertId,omitempty"`
	ExpertName                 string              `json:"expertName,omitempty"`
	Ready                      bool                `json:"ready"`
	Running                    bool                `json:"running"`
	PendingPrompt              bool                `json:"pendingPrompt,omitempty"`
	BackgroundJobs             int                 `json:"backgroundJobs,omitempty"`
	CancelRequested            bool                `json:"cancelRequested,omitempty"`
	Cancellable                bool                `json:"cancellable"`
	ActiveTurnStartedAt        int64               `json:"activeTurnStartedAt,omitempty"`
	ActiveTurnCompletionTokens int                 `json:"activeTurnCompletionTokens,omitempty"`
	Mode                       string              `json:"mode"`
	CollaborationMode          string              `json:"collaborationMode"`
	ToolApprovalMode           string              `json:"toolApprovalMode"`
	TokenMode                  string              `json:"tokenMode"`
	Goal                       string              `json:"goal,omitempty"`
	GoalStatus                 string              `json:"goalStatus,omitempty"`
	StartupErr                 string              `json:"startupErr,omitempty"`
	SendAvailability           TabSendAvailability `json:"sendAvailability"`
	Active                     bool                `json:"active"`
	Cwd                        string              `json:"cwd"`
}

func enrichTabMeta(meta TabMeta) TabMeta {
	if meta.Active {
		meta.GitBranch = workspaceGitBranchForMeta(meta.WorkspaceRoot)
	}
	return meta
}

func enrichTabMetas(metas []TabMeta) []TabMeta {
	for i := range metas {
		if metas[i].Active {
			metas[i].GitBranch = workspaceGitBranchForMeta(metas[i].WorkspaceRoot)
		}
	}
	return metas
}

func (a *App) tabMeta(tab *WorkspaceTab, active bool) TabMeta {
	composerIdentity := runtimeComposerIdentityView(tab, defaultComposerIdentityView())
	m := TabMeta{
		ID:                tab.ID,
		Scope:             tab.Scope,
		WorkspaceRoot:     tab.WorkspaceRoot,
		WorkspaceName:     workspaceName(tab.WorkspaceRoot),
		WorkspacePath:     tab.WorkspaceRoot,
		TopicID:           tab.TopicID,
		TopicTitle:        tab.TopicTitle,
		SessionPath:       tab.currentSessionPath(),
		SessionTurns:      cloneIntPointer(tab.sessionTurns),
		Scene:             string(tabSceneStatus(tab).Scene),
		SceneState:        string(tabSceneStatus(tab).State),
		ReadOnly:          tab.ReadOnly,
		Label:             tab.Label,
		ModelRef:          composerIdentity.modelRef,
		BillingSource:     composerIdentity.billingSource,
		Effort:            cloneStringPtr(composerIdentity.effort),
		ExpertID:          composerIdentity.expertID,
		ExpertName:        composerIdentity.expertName,
		Ready:             tab.Ready,
		Mode:              currentTabMode(tab),
		CollaborationMode: currentTabCollaborationMode(tab),
		ToolApprovalMode:  currentTabToolApprovalMode(tab),
		TokenMode:         currentTabTokenMode(tab),
		Goal:              currentTabGoal(tab),
		GoalStatus:        currentTabGoalStatus(tab),
		StartupErr:        tab.StartupErr,
		SendAvailability:  tabSendAvailability(tab),
		Active:            active,
		Cwd:               tab.WorkspaceRoot,
	}
	switch tab.Scope {
	case "global":
		m.ProjectColor = globalProjectColor()
		m.WorkspaceName = globalProjectTitle()
	case "project":
		m.ProjectColor = projectColor(tab.WorkspaceRoot)
	}
	if tab.Ctrl != nil {
		status := tab.Ctrl.RuntimeStatus()
		m.Running = status.Running || status.PendingPrompt || status.BackgroundJobs > 0
		m.PendingPrompt = status.PendingPrompt
		m.BackgroundJobs = status.BackgroundJobs
		m.CancelRequested = status.CancelRequested
		m.Cancellable = status.Cancellable
	}
	_, m.ActiveTurnStartedAt, m.ActiveTurnCompletionTokens = tab.contextUsageSnapshot()
	return m
}

func (a *App) refreshTabSessionTurns(tabID string) {
	a.mu.RLock()
	tab := a.tabByEventSinkIDLocked(tabID)
	var ctrl control.SessionAPI
	if tab != nil {
		ctrl = tab.Ctrl
	}
	a.mu.RUnlock()
	if tab == nil || ctrl == nil {
		return
	}
	sessionPath := sessionRuntimeKey(ctrl.SessionPath())
	_, turns := agent.SessionPreviewFromMessages(ctrl.History())
	a.mu.Lock()
	current := a.tabByEventSinkIDLocked(tabID)
	if current == tab && current.Ctrl == ctrl && sessionRuntimeKey(current.currentSessionPath()) == sessionPath {
		current.sessionTurns = intPointer(turns)
	}
	a.mu.Unlock()
}

func (a *App) setTabSessionTurns(tab *WorkspaceTab, turns *int) {
	if tab == nil {
		return
	}
	a.mu.Lock()
	if current := a.tabRuntimes[tab.ID]; current == tab {
		current.sessionTurns = cloneIntPointer(turns)
	}
	a.mu.Unlock()
}

func (a *App) recordAcceptedTabSessionTurn(ctrl control.SessionAPI) {
	if ctrl == nil {
		return
	}
	sessionPath := sessionRuntimeKey(ctrl.SessionPath())
	_, turns := agent.SessionPreviewFromMessages(ctrl.History())
	turns++
	updated := false
	var materializedTab *WorkspaceTab
	a.mu.Lock()
	for _, tab := range a.runtimeTabsLocked() {
		if tab != nil && tab.Ctrl == ctrl && sessionRuntimeKey(tab.currentSessionPath()) == sessionPath {
			if sessionPath != "" && tab.Ready && !tab.ReadOnly && strings.TrimSpace(tab.TopicID) == "" {
				topicID := legacySessionTopicID(sessionPath)
				if topicID != "" {
					if _, _, err := registerTopic(tab.Scope, tab.WorkspaceRoot, topicID, defaultTopicTitle, topicTitleSourceAuto, time.Now().UnixMilli()); err != nil {
						recordDesktopBackgroundFailure("topic.accepted_turn.register", err)
					} else {
						tab.TopicID = topicID
						tab.TopicTitle = defaultTopicTitle
						a.saveTabsLocked()
						snapshot := *tab
						materializedTab = &snapshot
					}
				}
			}
			tab.sessionTurns = intPointer(turns)
			updated = true
			break
		}
	}
	a.mu.Unlock()
	if !updated || sessionPath == "" {
		return
	}
	if materializedTab != nil {
		if err := saveTabSessionMeta(materializedTab, sessionPath); err != nil {
			recordDesktopBackgroundFailure("session.meta.accepted_turn", err)
		}
	}
	a.conversationActivity.Store(sessionPath, a.nextConversationActivityAt())
	a.emitProjectTreeChanged()
}

func (a *App) nextConversationActivityAt() int64 {
	next := time.Now().UnixMilli()
	for {
		current := a.conversationActivityClock.Load()
		if next <= current {
			next = current + 1
		}
		if a.conversationActivityClock.CompareAndSwap(current, next) {
			return next
		}
	}
}

func tabSendAvailability(tab *WorkspaceTab) TabSendAvailability {
	if tab == nil {
		return TabSendAvailability{State: tabSendStarting}
	}
	if tab.Ctrl != nil && tab.Ready {
		return TabSendAvailability{State: tabSendReady}
	}
	detail := strings.TrimSpace(tab.startupDetail)
	if detail == "" {
		detail = strings.TrimSpace(tab.StartupErr)
	}
	if tab.subscriptionAuthPending {
		return TabSendAvailability{State: tabSendSubscriptionAuthRequired, Retryable: true, Detail: detail, Action: "login"}
	}
	if detail == "" && !tab.Ready {
		return TabSendAvailability{State: tabSendStarting}
	}

	source := modelbilling.BillingSource(strings.TrimSpace(tab.billingSource))
	switch {
	case source == modelbilling.BillingSubscriptionPool &&
		(tab.startupErrorCode == modelbilling.ErrorAuthRequired || tab.startupErrorCode == modelbilling.ErrorSessionExpired):
		return TabSendAvailability{State: tabSendSubscriptionAuthRequired, Retryable: true, Detail: detail, Action: "login"}
	case source == modelbilling.BillingSubscriptionPool && tab.startupErrorCode == modelbilling.ErrorInsufficientCredits:
		return TabSendAvailability{State: tabSendSubscriptionAccountAction, Retryable: true, Detail: detail, Action: "account"}
	case source == modelbilling.BillingNewAPIByok &&
		(tab.startupErrorCode == modelbilling.ErrorByokCredentialInvalid || tab.startupFailureKind == tabStartupFailureConfig):
		return TabSendAvailability{State: tabSendBYOKConfigurationRequired, Retryable: true, Detail: detail, Action: "configure_byok"}
	case tab.startupErrorCode == modelbilling.ErrorGatewayUnavailable:
		return TabSendAvailability{State: tabSendServiceUnavailable, Retryable: true, Detail: detail, Action: "retry"}
	default:
		if detail == "" {
			detail = "controller is unavailable"
		}
		return TabSendAvailability{State: tabSendStartupFailed, Retryable: true, Detail: detail, Action: "retry"}
	}
}

func setTabStartupFailureLocked(tab *WorkspaceTab, err error, kind tabStartupFailureKind) {
	if tab == nil {
		return
	}
	tab.startupFailureKind = kind
	tab.startupErrorCode = modelbilling.ErrorCodeOf(err)
	tab.startupDetail = ""
	if err != nil {
		tab.startupDetail = err.Error()
	}
	if desktopSubscriptionAuthenticationPending(modelbilling.BillingSource(tab.billingSource), err) {
		tab.subscriptionAuthPending = true
		tab.StartupErr = ""
		return
	}
	tab.subscriptionAuthPending = false
	tab.StartupErr = tab.startupDetail
}

func clearTabStartupFailureLocked(tab *WorkspaceTab) {
	if tab == nil {
		return
	}
	tab.StartupErr = ""
	tab.subscriptionAuthPending = false
	tab.startupErrorCode = ""
	tab.startupFailureKind = tabStartupFailureNone
	tab.startupDetail = ""
}

// ListTabs returns every open view container's metadata for the frontend chrome and sidebar.
func (a *App) ListTabs() []TabMeta {
	a.mu.RLock()
	out := make([]TabMeta, 0, len(a.tabRuntimes))
	ordered, needsRepair := a.orderedTabIDsSnapshotLocked()
	for _, id := range ordered {
		if tab := a.tabRuntimes[id]; tab != nil {
			out = append(out, a.tabMeta(tab, tab.ID == a.cachedActiveTabID()))
		}
	}
	a.mu.RUnlock()
	if !needsRepair {
		return enrichTabMetas(out)
	}

	a.mu.Lock()
	out = make([]TabMeta, 0, len(a.tabRuntimes))
	for _, id := range a.orderedTabIDsLocked() {
		if tab := a.tabRuntimes[id]; tab != nil {
			out = append(out, a.tabMeta(tab, tab.ID == a.cachedActiveTabID()))
		}
	}
	a.mu.Unlock()
	return enrichTabMetas(out)
}

// OpenProjectTab builds a controller scoped to workspaceRoot and opens the
// session selected by the given topic. Topic selection resolves to a concrete
// session path first; the visible tab is then attached to that session runtime.
func (a *App) OpenProjectTab(workspaceRoot, topicID string) (TabMeta, error) {
	if workspaceRoot == "" {
		return TabMeta{}, fmt.Errorf("workspaceRoot is required")
	}
	if abs, err := filepath.Abs(workspaceRoot); err == nil {
		workspaceRoot = abs
	}
	a.saveWorkspace(workspaceRoot)
	if err := addProject(workspaceRoot, ""); err != nil {
		recordDesktopBackgroundFailure("project.register.open", err)
	}

	actor := a.activeConfigActorForWorkspace(workspaceRoot)
	sessionPath, _ := a.findTopicSessionForTargetForActor("project", workspaceRoot, topicID, actor)
	return a.openTopicTabWithActivationAndActor("project", workspaceRoot, topicID, sessionPath, true, actor)
}

func (a *App) openTopicTab(scope, workspaceRoot, topicID, sessionPath string) (TabMeta, error) {
	return a.openTopicTabWithActivation(scope, workspaceRoot, topicID, sessionPath, true)
}

func (a *App) openProjectTabInactive(workspaceRoot, topicID string) (TabMeta, error) {
	return a.openProjectTabInactiveWithActor(workspaceRoot, topicID, userctx.ActorContext{})
}

func (a *App) openProjectTabInactiveWithActor(workspaceRoot, topicID string, actor userctx.ActorContext) (TabMeta, error) {
	return a.openProjectTabInactiveWithActorAndRoute(workspaceRoot, topicID, actor, nil)
}

func (a *App) openProjectTabInactiveWithActorAndRoute(workspaceRoot, topicID string, actor userctx.ActorContext, route *modelbilling.BillingRouteSelection, pendingCloudProjectIDs ...string) (TabMeta, error) {
	if workspaceRoot == "" {
		return TabMeta{}, fmt.Errorf("workspaceRoot is required")
	}
	if abs, err := filepath.Abs(workspaceRoot); err == nil {
		workspaceRoot = abs
	}
	if err := addProject(workspaceRoot, ""); err != nil {
		recordDesktopBackgroundFailure("project.register.inactive", err)
	}

	if !actorContextSpecified(actor) {
		actor = a.actorForWorkspace(workspaceRoot)
	}
	sessionPath, _ := a.findTopicSessionForTargetForActor("project", workspaceRoot, topicID, actor)
	return a.openTopicTabWithActivationAndActorForRoute("project", workspaceRoot, topicID, sessionPath, false, actor, route, pendingCloudProjectIDs...)
}

func (a *App) openGlobalTabInactive(topicID string) (TabMeta, error) {
	return a.openGlobalTabInactiveWithActor(topicID, userctx.ActorContext{})
}

func (a *App) openGlobalTabInactiveWithActor(topicID string, actor userctx.ActorContext) (TabMeta, error) {
	return a.openGlobalTabInactiveWithActorAndRoute(topicID, actor, nil)
}

func (a *App) openGlobalTabInactiveWithActorAndRoute(topicID string, actor userctx.ActorContext, route *modelbilling.BillingRouteSelection, pendingCloudProjectIDs ...string) (TabMeta, error) {
	globalRoot := globalWorkspaceRoot()
	if err := os.MkdirAll(globalRoot, 0o755); err != nil {
		return TabMeta{}, fmt.Errorf("create global workspace: %w", err)
	}

	if !actorContextSpecified(actor) {
		actor = a.actorForWorkspace(globalRoot)
	}
	sessionPath, _ := a.findTopicSessionForTargetForActor("global", "", topicID, actor)
	return a.openTopicTabWithActivationAndActorForRoute("global", "", topicID, sessionPath, false, actor, route, pendingCloudProjectIDs...)
}

func (a *App) openTopicTabWithActivation(scope, workspaceRoot, topicID, sessionPath string, activate bool) (TabMeta, error) {
	return a.openTopicTabWithActivationAndActor(scope, workspaceRoot, topicID, sessionPath, activate, userctx.ActorContext{})
}

func (a *App) openTopicTabWithActivationAndActor(scope, workspaceRoot, topicID, sessionPath string, activate bool, actor userctx.ActorContext) (TabMeta, error) {
	return a.openTopicTabWithActivationAndActorForRoute(scope, workspaceRoot, topicID, sessionPath, activate, actor, nil)
}

func (a *App) openTopicTabWithActivationAndActorForRoute(scope, workspaceRoot, topicID, sessionPath string, activate bool, actor userctx.ActorContext, route *modelbilling.BillingRouteSelection, pendingCloudProjectIDs ...string) (TabMeta, error) {
	actualRoot := workspaceRoot
	if scope == "global" {
		actualRoot = globalWorkspaceRoot()
	}
	if !actorContextSpecified(actor) {
		actor = a.actorForWorkspace(actualRoot)
	} else {
		actor = actorContextForWorkspace(actor, actualRoot)
	}
	openingExistingSession := strings.TrimSpace(sessionPath) != ""
	preloaded := loadedTabSession{}
	persistedComposerState := sessionComposerState{}
	hasPersistedComposerState := false
	if strings.TrimSpace(sessionPath) != "" {
		candidate := &WorkspaceTab{Scope: scope, WorkspaceRoot: actualRoot, actor: actor}
		var err error
		preloaded, err = a.preloadTabSession(candidate, sessionPath)
		if err != nil {
			return TabMeta{}, err
		}
		sessionPath = preloaded.Path
		persistedComposerState, hasPersistedComposerState = loadSessionComposerState(sessionPath)
	}
	targetKey := sessionRuntimeKey(sessionPath)
	requireActorMatch := actorContextSpecified(actor)
	pendingCloudProjectID := ""
	if len(pendingCloudProjectIDs) > 0 {
		pendingCloudProjectID = strings.TrimSpace(pendingCloudProjectIDs[0])
	}

	a.mu.Lock()
	if targetKey != "" {
		for _, tab := range a.tabRuntimes {
			if tab == nil {
				continue
			}
			if sessionRuntimeKey(tab.currentSessionPath()) == targetKey {
				if requireActorMatch && !tabRuntimeActorMatches(tab, actor, actualRoot) {
					continue
				}
				tabID := tab.ID
				if pendingCloudProjectID != "" {
					tab.pendingCloudProjectID = pendingCloudProjectID
				}
				if !activate {
					meta := a.tabMeta(tab, tab.ID == a.cachedActiveTabID())
					a.saveTabsLocked()
					a.mu.Unlock()
					return enrichTabMeta(meta), nil
				}
				a.mu.Unlock()
				return a.activateTabMetaForOwnerTransition(tabID)
			}
		}
	}

	for _, tab := range a.tabRuntimes {
		if tabMatchesTopicTarget(tab, scope, workspaceRoot, topicID) {
			if requireActorMatch && !tabRuntimeActorMatches(tab, actor, actualRoot) {
				continue
			}
			sameSession := targetKey == "" || sessionRuntimeKey(tab.currentSessionPath()) == targetKey
			tabID := tab.ID
			if pendingCloudProjectID != "" {
				tab.pendingCloudProjectID = pendingCloudProjectID
			}
			if !activate {
				meta := a.tabMeta(tab, tab.ID == a.cachedActiveTabID())
				a.saveTabsLocked()
				a.mu.Unlock()
				if sameSession {
					return enrichTabMeta(meta), nil
				}
				if err := a.rebindTabToLoadedSessionPath(tab, sessionPath, preloaded.Session); err != nil {
					return TabMeta{}, err
				}
				a.mu.RLock()
				meta = a.tabMeta(tab, tab.ID == a.cachedActiveTabID())
				a.mu.RUnlock()
				return enrichTabMeta(meta), nil
			}
			a.mu.Unlock()
			meta, err := a.activateTabMetaForOwnerTransition(tabID)
			if err != nil {
				return TabMeta{}, err
			}
			if sameSession {
				return meta, nil
			}
			if err := a.rebindTabToLoadedSessionPath(tab, sessionPath, preloaded.Session); err != nil {
				return TabMeta{}, err
			}
			a.mu.RLock()
			meta = a.tabMeta(tab, tab.ID == a.cachedActiveTabID())
			a.mu.RUnlock()
			return enrichTabMeta(meta), nil
		}
	}

	tabID := a.newUniqueTabIDLocked()
	topicTitle := topicTitleForTab(scope, workspaceRoot, topicID)
	defaultSource := a.defaultBillingSourceForNewTask(actor)
	model := ""
	var effort *string
	toolApprovalMode := control.ToolApprovalAsk
	toolApprovalModeKnown := false
	if route != nil {
		defaultSource = route.BillingSource
		model = strings.TrimSpace(route.ModelRef)
	} else if hasPersistedComposerState && persistedComposerState.identityKnown {
		defaultSource = persistedComposerState.billingSource
		model = persistedComposerState.model
		effort = cloneStringPtr(persistedComposerState.effort)
	} else if openingExistingSession {
		storedModel := strings.TrimSpace(loadTopicModel(topicTitleRoot(scope, workspaceRoot), topicID))
		if state, ok := composerStateFromValues(storedModel, "", nil); ok {
			defaultSource = state.billingSource
			model = state.model
			effort = cloneStringPtr(state.effort)
		}
	}
	if model == "" {
		model, _ = a.desktopNewSessionDefaultsForRoute(actualRoot, actor, defaultSource)
		if route == nil && !openingExistingSession {
			if storedModel := strings.TrimSpace(loadTopicModel(topicTitleRoot(scope, workspaceRoot), topicID)); topicModelMatchesBillingSource(storedModel, defaultSource) {
				model = storedModel
				if defaultSource == modelbilling.BillingSubscriptionPool {
					model = a.availableSubscriptionModel(actualRoot, actor, model)
				}
			}
		}
	}
	if effort == nil {
		effort = a.defaultEffortForNewSession(actualRoot, actor, defaultSource, model)
	}
	if openingExistingSession && hasPersistedComposerState {
		toolApprovalMode = persistedComposerState.toolApprovalMode
		toolApprovalModeKnown = persistedComposerState.toolApprovalModeKnown
	} else if !openingExistingSession {
		_, toolApprovalMode = a.desktopNewSessionDefaultsForRoute(actualRoot, actor, defaultSource)
		toolApprovalModeKnown = true
	}
	if sessionPath == "" {
		var err error
		sessionPath, err = createEmptySessionFile(desktopSessionDirForActor(actor, actualRoot), model)
		if err != nil {
			a.mu.Unlock()
			return TabMeta{}, err
		}
	}
	status := newDesktopSceneStatus()
	if openingExistingSession {
		status = legacyDesktopSceneStatus()
	}
	tab := &WorkspaceTab{
		ID:                    tabID,
		Scope:                 scope,
		WorkspaceRoot:         actualRoot,
		TopicID:               topicID,
		TopicTitle:            topicTitle,
		SessionPath:           sessionPath,
		pendingCloudProjectID: pendingCloudProjectID,
		Scene:                 status.Scene,
		SceneState:            status.State,
		model:                 model,
		billingSource:         string(defaultSource),
		effort:                effort,
		tokenMode:             boot.TokenModeFull,
		actor:                 actor,
		mode:                  "normal",
		toolApprovalMode:      toolApprovalMode,
		toolApprovalModeKnown: toolApprovalModeKnown,
		disabledMCP:           map[string]ServerView{},
	}
	tab.sink = &tabEventSink{tabID: tabID, app: a}

	a.tabRuntimes[tabID] = tab
	a.tabOrder = append(a.tabOrder, tabID)
	a.saveTabsLocked()
	meta := a.tabMeta(tab, tab.ID == a.cachedActiveTabID())
	a.mu.Unlock()

	if activate {
		var err error
		meta, err = a.activateTabMetaForOwnerTransition(tabID)
		if err != nil {
			return TabMeta{}, err
		}
	}
	if route != nil {
		a.startTabControllerBuildWithLoadedSessionForRoute(tab, preloaded, route)
	} else {
		a.startTabControllerBuildWithLoadedSession(tab, preloaded)
	}
	if scope == "project" {
		a.emitProjectTreeChanged()
	}
	return enrichTabMeta(meta), nil
}

// OpenGlobalTab opens a new global-scope tab (no project root). The global
// workspace root is under the active desktop config directory.
func (a *App) OpenGlobalTab(topicID string) (TabMeta, error) {
	globalRoot := globalWorkspaceRoot()
	if err := os.MkdirAll(globalRoot, 0o755); err != nil {
		return TabMeta{}, fmt.Errorf("create global workspace: %w", err)
	}

	actor := a.activeConfigActorForWorkspace(globalRoot)
	sessionPath, _ := a.findTopicSessionForTargetForActor("global", "", topicID, actor)
	return a.openTopicTabWithActivationAndActor("global", "", topicID, sessionPath, true, actor)
}

// OpenTopicSession opens a concrete saved session from the sidebar. Unlike
// OpenProjectTab/OpenGlobalTab, it does not resolve the topic to the latest
// session first; sessionPath is the runtime identity being selected.
func (a *App) OpenTopicSession(scope, workspaceRoot, topicID, sessionPath string) (TabMeta, error) {
	scope = strings.TrimSpace(scope)
	if scope != "project" {
		scope = "global"
		workspaceRoot = ""
	}
	if scope == "project" {
		workspaceRoot = normalizeProjectRoot(workspaceRoot)
		if workspaceRoot == "" {
			return TabMeta{}, fmt.Errorf("workspaceRoot is required")
		}
		a.saveWorkspace(workspaceRoot)
		if err := addProject(workspaceRoot, ""); err != nil {
			return TabMeta{}, err
		}
	}
	actualRoot := workspaceRoot
	if scope == "global" {
		actualRoot = globalWorkspaceRoot()
	}
	return a.openTopicTabWithActivationAndActor(scope, workspaceRoot, topicID, sessionPath, true, a.activeConfigActorForWorkspace(actualRoot))
}

// ActivateTopic opens a topic into the single visible conversation surface used
// by layouts without a tab strip. It delegates the actual open/reuse behavior to
// the classic tab path, then prunes every non-active visible tab so historical
// clicks do not accumulate hidden startup work.
func (a *App) ActivateTopic(scope, workspaceRoot, topicID, sessionPath string) (TabMeta, error) {
	a.singleSurfaceMu.Lock()
	defer a.singleSurfaceMu.Unlock()

	var meta TabMeta
	var err error
	if strings.TrimSpace(sessionPath) != "" {
		meta, err = a.OpenTopicSession(scope, workspaceRoot, topicID, sessionPath)
	} else if strings.TrimSpace(scope) == "project" {
		meta, err = a.OpenProjectTab(workspaceRoot, topicID)
	} else {
		meta, err = a.OpenGlobalTab(topicID)
	}
	if err != nil {
		return TabMeta{}, err
	}
	return a.keepOnlyVisibleTab(meta.ID)
}

// EnsureBlankSurface mirrors EnsureBlankTab for no-tab-strip layouts: after
// creating or reusing a blank session, it removes other visible tabs while
// preserving running runtimes as detached background sessions.
func (a *App) EnsureBlankSurface(scope, workspaceRoot string) (TabMeta, error) {
	a.singleSurfaceMu.Lock()
	defer a.singleSurfaceMu.Unlock()

	meta, err := a.EnsureBlankTab(scope, workspaceRoot)
	if err != nil {
		return TabMeta{}, err
	}
	return a.keepOnlyVisibleTab(meta.ID)
}

func tabMatchesTopicTarget(tab *WorkspaceTab, scope, workspaceRoot, topicID string) bool {
	if tab == nil || tab.Scope != scope || tab.TopicID != topicID {
		return false
	}
	if scope == "global" {
		return true
	}
	return normalizeProjectRoot(tab.WorkspaceRoot) == normalizeProjectRoot(workspaceRoot)
}

func tabInWorkspace(tab *WorkspaceTab, workspaceRoot string) bool {
	return tab != nil &&
		tab.Scope == "project" &&
		normalizeProjectRoot(tab.WorkspaceRoot) == normalizeProjectRoot(workspaceRoot)
}

// EnsureBlankTab activates the existing blank tab for the target scope, or
// creates one if none exists. Reusing a blank tab keeps repeated "new session"
// clicks from piling up empty conversations.
func (a *App) EnsureBlankTab(scope, workspaceRoot string) (TabMeta, error) {
	scope = strings.TrimSpace(scope)
	if scope != "project" {
		scope = "global"
	}

	globalRoot := ""
	if scope == "project" {
		workspaceRoot = strings.TrimSpace(workspaceRoot)
		if workspaceRoot == "" {
			return TabMeta{}, fmt.Errorf("workspaceRoot is required")
		}
		if abs, err := filepath.Abs(workspaceRoot); err == nil {
			workspaceRoot = abs
		}
		a.saveWorkspace(workspaceRoot)
		if err := addProject(workspaceRoot, ""); err != nil {
			return TabMeta{}, err
		}
	} else {
		workspaceRoot = ""
		globalRoot = globalWorkspaceRoot()
		if err := os.MkdirAll(globalRoot, 0o755); err != nil {
			return TabMeta{}, fmt.Errorf("create global workspace: %w", err)
		}
	}

	var created *WorkspaceTab
	// Compute actual root early — both the indexed-topic fallback and the
	// new-topic path need it when constructing the tab below.
	actualRoot := workspaceRoot
	if scope == "global" {
		actualRoot = globalRoot
	}
	actor := a.actorForWorkspace(actualRoot)
	defaultSource := a.defaultBillingSourceForNewTask(actor)
	defaultModel, defaultToolApprovalMode := a.desktopNewSessionDefaultsForRoute(actualRoot, actor, defaultSource)

	a.mu.Lock()
	for _, id := range a.orderedTabIDsLocked() {
		tab := a.tabRuntimes[id]
		if a.blankTabMatchesTargetLocked(tab, scope, workspaceRoot) &&
			tabRuntimeActorMatches(tab, actor, actualRoot) &&
			modelbilling.BillingSource(tab.billingSource) == defaultSource {
			if err := resetReusableBlankTabTitle(tab, scope, workspaceRoot); err != nil {
				a.mu.Unlock()
				return TabMeta{}, err
			}
			tab.toolApprovalMode = defaultToolApprovalMode
			tab.toolApprovalModeKnown = true
			tab.mode = tabModeFromAxes(tabModeHasPlan(currentTabMode(tab)), defaultToolApprovalMode == control.ToolApprovalYolo)
			ctrl := tab.Ctrl
			tabID := tab.ID
			a.saveTabsLocked()
			a.mu.Unlock()
			applyTabToolApprovalModeToController(ctrl, defaultToolApprovalMode)
			return a.activateTabMetaForOwnerTransition(tabID)
		}
	}

	// New blank sessions start from global session defaults for model and
	// Ask/Auto/YOLO approval posture. Token/MCP remain execution-local continuity;
	// effort is seeded from the selected model's global default for this session.
	inheritedModel := defaultModel
	inheritedEffort := a.defaultEffortForNewSession(actualRoot, actor, defaultSource, inheritedModel)
	inheritedTokenMode := boot.TokenModeFull
	inheritedMode := tabModeFromAxes(false, defaultToolApprovalMode == control.ToolApprovalYolo)
	inheritedToolApprovalMode := defaultToolApprovalMode
	inheritedDisabledMCP := map[string]ServerView{}
	var inheritedMCPOrder []string
	if active := a.activeTabLocked(); active != nil {
		inheritedTokenMode = currentTabTokenMode(active)
		inheritedDisabledMCP = cloneServerViewMap(active.disabledMCP)
		inheritedMCPOrder = append([]string(nil), active.mcpOrder...)
	}

	if topicID := a.indexedBlankTopicIDLocked(scope, workspaceRoot); topicID != "" {
		// Reuse a previously-indexed but unused blank topic instead of
		// creating a new one.  Build it inline (not via OpenProjectTab /
		// OpenGlobalTab) so it inherits settings from the active tab.
		tabID := a.newUniqueTabIDLocked()
		topicTitle := topicTitleForTab(scope, workspaceRoot, topicID)
		created = &WorkspaceTab{
			ID:                    tabID,
			Scope:                 scope,
			WorkspaceRoot:         actualRoot,
			TopicID:               topicID,
			TopicTitle:            topicTitle,
			Scene:                 scene.Office,
			SceneState:            scene.Draft,
			model:                 inheritedModel,
			billingSource:         string(defaultSource),
			effort:                inheritedEffort,
			tokenMode:             inheritedTokenMode,
			mode:                  inheritedMode,
			toolApprovalMode:      inheritedToolApprovalMode,
			toolApprovalModeKnown: true,
			disabledMCP:           inheritedDisabledMCP,
			mcpOrder:              inheritedMCPOrder,
			actor:                 actor,
		}
		created.sink = &tabEventSink{tabID: tabID, app: a}
		prePath, err := createEmptySessionFile(desktopSessionDirForActor(actor, actualRoot), inheritedModel)
		if err != nil {
			a.mu.Unlock()
			return TabMeta{}, err
		}
		created.SessionPath = prePath
		a.tabRuntimes[tabID] = created
		a.tabOrder = append(a.tabOrder, tabID)
		a.saveTabsLocked()
		a.mu.Unlock()

		meta, err := a.activateTabMetaForOwnerTransition(tabID)
		if err != nil {
			return TabMeta{}, err
		}
		a.startTabControllerBuild(created)
		a.emitProjectTreeChanged()
		return meta, nil
	}

	topicID := newTopicID()
	topicTitle := defaultTopicTitle
	if _, _, err := registerTopic(scope, workspaceRoot, topicID, topicTitle, topicTitleSourceAuto, time.Now().UnixMilli()); err != nil {
		a.mu.Unlock()
		return TabMeta{}, err
	}

	tabID := a.newUniqueTabIDLocked()
	created = &WorkspaceTab{
		ID:                    tabID,
		Scope:                 scope,
		WorkspaceRoot:         actualRoot,
		TopicID:               topicID,
		TopicTitle:            topicTitleForTab(scope, workspaceRoot, topicID),
		Scene:                 scene.Office,
		SceneState:            scene.Draft,
		model:                 inheritedModel,
		billingSource:         string(defaultSource),
		effort:                inheritedEffort,
		tokenMode:             inheritedTokenMode,
		mode:                  inheritedMode,
		toolApprovalMode:      inheritedToolApprovalMode,
		toolApprovalModeKnown: true,
		disabledMCP:           inheritedDisabledMCP,
		mcpOrder:              inheritedMCPOrder,
		actor:                 actor,
	}
	created.sink = &tabEventSink{tabID: tabID, app: a}
	prePath, err := createEmptySessionFile(desktopSessionDirForActor(actor, actualRoot), inheritedModel)
	if err != nil {
		a.mu.Unlock()
		return TabMeta{}, err
	}
	created.SessionPath = prePath
	a.tabRuntimes[tabID] = created
	a.tabOrder = append(a.tabOrder, tabID)
	a.saveTabsLocked()
	a.mu.Unlock()

	meta, err := a.activateTabMetaForOwnerTransition(tabID)
	if err != nil {
		return TabMeta{}, err
	}
	a.startTabControllerBuild(created)
	a.emitProjectTreeChanged()
	return meta, nil
}

func topicModelMatchesBillingSource(model string, source modelbilling.BillingSource) bool {
	providerName, modelName, ok := strings.Cut(strings.TrimSpace(model), "/")
	if !ok || strings.TrimSpace(providerName) == "" || strings.TrimSpace(modelName) == "" {
		return false
	}
	switch source {
	case "":
		return true
	case modelbilling.BillingSubscriptionPool:
		return providerName == brand.ProviderID
	case modelbilling.BillingNewAPIByok:
		return providerName != brand.ProviderID
	default:
		return false
	}
}

// blankTabMatchesTargetLocked returns true if tab is a reusable blank tab
// matching the given scope/project root — no running controller, no real history.
func (a *App) blankTabMatchesTargetLocked(tab *WorkspaceTab, scope, workspaceRoot string) bool {
	if tab == nil || tab.Scope != scope {
		return false
	}
	if scope == "project" && tab.WorkspaceRoot != workspaceRoot {
		return false
	}
	// A Cloud Project session can still look blank when its instruction is empty:
	// the durable project pin lives in BranchMeta and there is no visible context
	// message. Never recycle that session for another project composer, because
	// NewCloudProjectSession must reject changing an existing project pin.
	if tabSessionMayBeCloudProjectBound(tab) {
		return false
	}
	if tab.Ctrl == nil {
		return blankTabSessionPathHasNoContent(tab)
	}
	if tab.hasActiveRuntimeWork() {
		return false
	}
	if currentExpertID(tab.Ctrl) != "" {
		return false
	}
	return !messagesHaveConversationContent(tab.Ctrl.History())
}

func tabSessionMayBeCloudProjectBound(tab *WorkspaceTab) bool {
	if tab == nil {
		return false
	}
	path := strings.TrimSpace(tab.SessionPath)
	if tab.Ctrl != nil {
		if agent.InspectProjectContext(tab.Ctrl.History()) != nil {
			return true
		}
		if controllerPath := strings.TrimSpace(tab.Ctrl.SessionPath()); controllerPath != "" {
			path = controllerPath
		}
	}
	if path == "" {
		return false
	}
	meta, found, err := agent.LoadBranchMeta(path)
	// Reuse is an optimization, so a sidecar read failure must fail closed rather
	// than risk crossing a Cloud Project boundary.
	return err != nil || (found && meta.IsCloudProjectBound())
}

func createEmptySessionFile(dir, model string) (string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "", fmt.Errorf("session dir is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	for i := 0; i < 3; i++ {
		path := agent.NewSessionPath(dir, model)
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			if closeErr := f.Close(); closeErr != nil {
				return "", closeErr
			}
			return path, nil
		}
		if os.IsExist(err) {
			continue
		}
		return "", err
	}
	return "", fmt.Errorf("create empty session file: exhausted filename retries")
}

func blankTabSessionPathHasNoContent(tab *WorkspaceTab) bool {
	if tab == nil {
		return false
	}
	if strings.TrimSpace(tab.SessionPath) == "" {
		return true
	}
	path, ok := pinnedTabSessionPath(tabSessionDir(tab), tab.SessionPath)
	if !ok {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}
	if info.Size() == 0 {
		return true
	}
	session, err := agent.LoadSession(path)
	if err != nil {
		return false
	}
	return !session.HasContent()
}

func resetReusableBlankTabTitle(tab *WorkspaceTab, scope, workspaceRoot string) error {
	if tab == nil {
		return nil
	}
	topicID := strings.TrimSpace(tab.TopicID)
	if topicID == "" {
		return nil
	}
	titleRoot := topicTitleRoot(scope, workspaceRoot)
	updated, err := resetAutoTopicTitle(titleRoot, topicID)
	if err != nil {
		return err
	}
	if updated {
		tab.TopicTitle = defaultTopicTitle
	}
	return nil
}

// indexedBlankTopicIDLocked finds a blank topic ID that is indexed on disk
// but not open in any tab — for reusing without creating a new topic.
func (a *App) indexedBlankTopicIDLocked(scope, workspaceRoot string) string {
	titleRoot := topicTitleRoot(scope, workspaceRoot)
	titles := loadTopicTitles(titleRoot)
	f := loadProjectsFile()

	var topicIDs []string
	if scope == "global" {
		topicIDs = orderedTopicIDs(f.GlobalTopics, titles)
	} else {
		for _, project := range f.Projects {
			if project.Root == workspaceRoot {
				topicIDs = orderedTopicIDs(project.Topics, titles)
				break
			}
		}
	}
	if len(topicIDs) == 0 {
		return ""
	}

	openTopics := map[string]bool{}
	for _, tab := range a.tabRuntimes {
		if tab == nil || tab.Scope != scope || strings.TrimSpace(tab.TopicID) == "" {
			continue
		}
		if scope == "project" && tab.WorkspaceRoot != workspaceRoot {
			continue
		}
		openTopics[tab.TopicID] = true
	}
	seenSessionDirs := map[string]bool{}
	sessionIndexes := []topicSessionDirIndex{}
	addSessionIndex := func(dir string) {
		dir = cleanDesktopPath(dir)
		if dir == "" {
			return
		}
		if seenSessionDirs[dir] {
			return
		}
		seenSessionDirs[dir] = true
		if index, err := topicSessionIndexForDir(dir); err == nil {
			sessionIndexes = append(sessionIndexes, index)
		}
	}
	if scope == "project" {
		addSessionIndex(desktopSessionDir(workspaceRoot))
	} else {
		addSessionIndex(config.SessionDir())
		addSessionIndex(desktopSessionDir(globalWorkspaceRoot()))
	}
	for _, topicID := range topicIDs {
		if openTopics[topicID] {
			continue
		}
		if topicTitleForTab(scope, workspaceRoot, topicID) != defaultTopicTitle {
			continue
		}
		hasSession := false
		for _, index := range sessionIndexes {
			if topicSessionIndexHasContentTopic(index, topicID) {
				hasSession = true
				break
			}
		}
		if hasSession {
			continue
		}
		return topicID
	}
	return ""
}

// SetActiveTab switches the frontend's active tab. A no-op when tabID is
// already active or unknown.
func (a *App) SetActiveTab(tabID string) error {
	return a.setActiveTabForOwnerTransition(tabID)
}

func (a *App) setActiveTabForOwnerTransition(tabID string) error {
	tabID = strings.TrimSpace(tabID)
	snapshot := a.activationSnapshot()
	if snapshot == nil {
		return fmt.Errorf("tab runtime index is not ready")
	}
	target, ok := snapshot.byID[tabID]
	if !ok || target.tab == nil {
		return fmt.Errorf("tab %q not found", tabID)
	}
	actorState := a.currentActorSnapshot()
	dir := desktopConfigDirForPaths(config.NewProfilePathResolver(actorState.actor.Normalize()))
	store := newDesktopTabStore(dir)
	activeTabID, version, err := store.activeTabID()
	if err != nil {
		return err
	}
	if activeTabID == tabID {
		a.publishActiveTabID(activeTabID)
		a.advanceTabsSaveVersion(version)
		return nil
	}
	hadActive := activeTabID != ""
	oldActor := actorState.actor.Normalize()
	if old, found := snapshot.byID[activeTabID]; found {
		oldActor = activationActor(old, actorState)
	}
	newActor := activationActor(target, actorState)
	ownerChanged := !hadActive || !sameDesktopActorProfile(oldActor, newActor)
	if ownerChanged {
		// The event worker may currently be settling through the billing manager.
		// Drain it first so billing quiescence cannot wait behind its own caller.
		if a.cloudRuntime != nil {
			a.cloudRuntime.stopEventWorker()
		}
		if err := a.quiesceDesktopBillingWorker(); err != nil {
			return fmt.Errorf("quiesce old active profile: %w", err)
		}
	}
	activation, err := store.activateTab(activeTabID, tabID)
	if err != nil {
		if ownerChanged {
			// We already stopped the previous owner's workers. Restore whichever
			// owner won the database race before returning the conflict.
			if currentID, _, readErr := store.activeTabID(); readErr == nil {
				if current, found := snapshot.byID[currentID]; found {
					recoveryErr := a.recoverDesktopActorState(a.bootContext(), activationActor(current, actorState))
					return errors.Join(err, recoveryErr)
				}
			}
		}
		return err
	}
	a.publishActiveTabID(activation.ActiveID)
	a.advanceTabsSaveVersion(activation.Version)
	if !ownerChanged {
		return nil
	}
	if err := a.recoverDesktopActorState(a.bootContext(), newActor); err != nil {
		if a.cloudRuntime != nil {
			a.cloudRuntime.stopEventWorker()
		}
		cleanupErr := a.quiesceDesktopBillingWorker()
		return errors.Join(err, cleanupErr)
	}
	return nil
}

// activateTabMetaForOwnerTransition activates a tab and reads metadata only
// after the billing owner handoff has either completed or failed closed.
func (a *App) activateTabMetaForOwnerTransition(tabID string) (TabMeta, error) {
	if err := a.setActiveTabForOwnerTransition(tabID); err != nil {
		return TabMeta{}, err
	}
	a.mu.RLock()
	tab := a.tabRuntimes[tabID]
	if tab == nil {
		a.mu.RUnlock()
		return TabMeta{}, fmt.Errorf("tab %q changed during activation", tabID)
	}
	meta := a.tabMeta(tab, tab.ID == a.cachedActiveTabID())
	a.mu.RUnlock()
	return enrichTabMeta(meta), nil
}

// ReorderTabs persists the frontend's manual tab order. The submitted order must
// contain every currently open tab exactly once.
func (a *App) ReorderTabs(tabIDs []string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(tabIDs) != len(a.tabRuntimes) {
		return fmt.Errorf("tab order length mismatch")
	}
	seen := make(map[string]bool, len(tabIDs))
	next := make([]string, 0, len(tabIDs))
	for _, id := range tabIDs {
		if _, ok := a.tabRuntimes[id]; !ok {
			return fmt.Errorf("tab %q not found", id)
		}
		if seen[id] {
			return fmt.Errorf("duplicate tab %q", id)
		}
		seen[id] = true
		next = append(next, id)
	}
	a.tabOrder = next
	a.saveTabsLocked()
	return nil
}

// CloseTab removes a visible tab. If the tab's session still has foreground or
// background work, the controller is detached so closing a view does not destroy
// the session runtime.
func (a *App) CloseTab(tabID string) error {
	a.mu.Lock()
	tab, ok := a.tabRuntimes[tabID]
	if !ok {
		a.mu.Unlock()
		return fmt.Errorf("tab %q not found", tabID)
	}
	if len(a.tabRuntimes) <= 1 {
		a.mu.Unlock()
		return fmt.Errorf("cannot close the last tab")
	}
	ordered := a.orderedTabIDsLocked()
	closedIndex := -1
	for i, id := range ordered {
		if id == tabID {
			closedIndex = i
			break
		}
	}
	wasActive := a.cachedActiveTabID() == tabID
	nextTabID := ""
	if wasActive && len(ordered) > 1 {
		if closedIndex < 0 {
			nextTabID = ordered[0]
		} else if closedIndex+1 < len(ordered) {
			nextTabID = ordered[closedIndex+1]
		} else {
			nextTabID = ordered[closedIndex-1]
		}
	}
	a.mu.Unlock()
	if wasActive {
		if nextTabID == "" {
			return fmt.Errorf("cannot select a tab after closing %q", tabID)
		}
		if err := a.setActiveTabForOwnerTransition(nextTabID); err != nil {
			return fmt.Errorf("switch active tab before close: %w", err)
		}
	}

	a.mu.Lock()
	tab, ok = a.tabRuntimes[tabID]
	if !ok {
		a.mu.Unlock()
		return fmt.Errorf("tab %q changed during close", tabID)
	}
	// Snapshot before removing the tab from a.tabRuntimes. This closes a race window
	// with DeleteSession while still leaving the tab intact if owner handoff
	// failed above.
	if tab.Ctrl != nil && !tab.ReadOnly {
		if err := tab.Ctrl.Snapshot(); err != nil {
			recordDesktopBackgroundFailure("session.snapshot.close", err)
		}
	}
	detached := false
	if tab.Ctrl != nil && tab.hasActiveRuntimeWork() {
		detached = a.detachSessionRuntimeLocked(tab)
		if !detached {
			a.mu.Unlock()
			return fmt.Errorf("cannot detach active session runtime for tab %q", tabID)
		}
	} else {
		a.markTabRemovedLocked(tab)
	}
	delete(a.tabRuntimes, tabID)
	a.removeTabOrderLocked(tabID)
	a.saveTabsLocked()
	a.mu.Unlock()

	// Tear down outside the lock.
	if detached {
		if tab.sink != nil {
			tab.sink.clearContext()
		}
		// Detached runtimes keep running and must keep saving: do not clear
		// their path or drain the controller.
		return nil
	}
	if tab.Ctrl != nil {
		a.quiesceTabAutosave(tab)   // finish any claimed artifact writer first
		tab.Ctrl.SetSessionPath("") // future snapshots become no-ops
		tab.Ctrl.Cancel()
		tab.Ctrl.Close()
		// Release the shared plugin host reference. The host stays alive as
		// long as any other tab for the same workspace root holds a reference;
		// on the last release the host is closed and its subprocesses exit.
		a.releaseTabSharedHost(tab)
	}
	if tab.sink != nil {
		tab.sink.clearContext() // stop further emissions (nil ctx -> Emit becomes no-op)
	}
	return nil
}

func (a *App) keepOnlyVisibleTab(tabID string) (TabMeta, error) {
	type pruneCandidate struct {
		id  string
		tab *WorkspaceTab
	}

	a.mu.Lock()
	active := a.tabRuntimes[tabID]
	if active == nil {
		a.mu.Unlock()
		return TabMeta{}, fmt.Errorf("tab %q not found", tabID)
	}
	candidates := make([]pruneCandidate, 0, len(a.tabRuntimes)-1)
	for id, tab := range a.tabRuntimes {
		if id == tabID {
			continue
		}
		candidates = append(candidates, pruneCandidate{id: id, tab: tab})
	}
	a.mu.Unlock()

	// Snapshot while the tab is still discoverable, but never while holding
	// App.mu: controller persistence can invoke callbacks that need the same
	// lock. Each removed runtime is snapshotted exactly once.
	snapshotted := make(map[string]*WorkspaceTab, len(candidates))
	for _, candidate := range candidates {
		snapshotted[candidate.id] = candidate.tab
		if err := a.snapshotTabForSurfaceSwitch(candidate.tab); err != nil {
			slog.Warn("desktop: snapshot before pruning hidden tab failed", "tab", candidate.id, "err", err)
			return TabMeta{}, fmt.Errorf("save current session before switching tabRuntimes: %w", err)
		}
	}

	a.mu.Lock()
	active = a.tabRuntimes[tabID]
	if active == nil {
		a.mu.Unlock()
		return TabMeta{}, fmt.Errorf("tab %q not found", tabID)
	}
	for id, tab := range a.tabRuntimes {
		if id != tabID && snapshotted[id] != tab {
			a.mu.Unlock()
			return TabMeta{}, fmt.Errorf("visible tabs changed while switching; retry")
		}
	}

	detachKeys := make(map[*WorkspaceTab]string)
	reservedDetached := make(map[string]*WorkspaceTab, len(a.detachedSessions)+len(candidates))
	for key, tab := range a.detachedSessions {
		reservedDetached[key] = tab
	}
	for _, candidate := range candidates {
		tab := candidate.tab
		if tab == nil || a.tabRuntimes[candidate.id] != tab || tab.Ctrl == nil || !tab.hasActiveRuntimeWork() {
			continue
		}
		key := sessionRuntimeKey(tab.currentSessionPath())
		if key == "" {
			a.mu.Unlock()
			return TabMeta{}, fmt.Errorf("cannot detach active session runtime for tab %q", candidate.id)
		}
		if existing := reservedDetached[key]; existing != nil && existing != tab {
			a.mu.Unlock()
			return TabMeta{}, fmt.Errorf("session runtime for tab %q is already detached", candidate.id)
		}
		reservedDetached[key] = tab
		detachKeys[tab] = key
	}

	a.setActiveTabIDLocked(tabID)
	detached := make([]*WorkspaceTab, 0, len(detachKeys))
	removed := make([]*WorkspaceTab, 0, len(candidates)-len(detachKeys))
	for _, candidate := range candidates {
		id, tab := candidate.id, candidate.tab
		if tab == nil || a.tabRuntimes[id] != tab {
			continue
		}
		if key, shouldDetach := detachKeys[tab]; shouldDetach {
			if !a.detachSessionRuntimeAtKeyLocked(tab, key) {
				a.mu.Unlock()
				return TabMeta{}, fmt.Errorf("cannot detach active session runtime for tab %q", id)
			}
			detached = append(detached, tab)
		} else {
			a.markTabRemovedLocked(tab)
			removed = append(removed, tab)
		}
		delete(a.tabRuntimes, id)
		a.removeTabOrderLocked(id)
	}
	a.tabOrder = []string{tabID}
	a.saveTabsLocked()
	meta := a.tabMeta(active, true)
	a.mu.Unlock()

	for _, tab := range detached {
		if tab.sink != nil {
			tab.sink.clearContext()
		}
	}
	for _, tab := range removed {
		a.removeVisibleTabRuntimeAfterSnapshot(tab)
	}
	a.emitProjectTreeChanged()
	return enrichTabMeta(meta), nil
}

const surfaceSwitchSnapshotTimeout = 2 * time.Second

type boundedSnapshotter interface {
	SnapshotWithTimeout(time.Duration) error
}

// snapshotTabForSurfaceSwitch keeps the single-surface transition bounded when
// an earlier autosave is stuck holding the controller snapshot lock. On timeout
// keepOnlyVisibleTab aborts before deleting any tab, so the user can retry and
// no session runtime is discarded without a confirmed snapshot.
func (a *App) snapshotTabForSurfaceSwitch(tab *WorkspaceTab) error {
	if tab == nil {
		return nil
	}
	a.mu.RLock()
	readOnly := tab.ReadOnly
	ctrl := tab.Ctrl
	a.mu.RUnlock()
	if readOnly || ctrl == nil {
		return nil
	}
	if bounded, ok := ctrl.(boundedSnapshotter); ok {
		return bounded.SnapshotWithTimeout(surfaceSwitchSnapshotTimeout)
	}
	return ctrl.Snapshot()
}

func (a *App) applySingleSurfaceTabPolicy() error {
	a.singleSurfaceMu.Lock()
	defer a.singleSurfaceMu.Unlock()

	a.mu.RLock()
	tabID := a.cachedActiveTabID()
	if tabID == "" || a.tabRuntimes[tabID] == nil {
		for _, id := range a.tabOrder {
			if a.tabRuntimes[id] != nil {
				tabID = id
				break
			}
		}
		if tabID == "" {
			for id := range a.tabRuntimes {
				tabID = id
				break
			}
		}
	}
	a.mu.RUnlock()
	if tabID == "" {
		return nil
	}
	_, err := a.keepOnlyVisibleTab(tabID)
	return err
}

func (a *App) removeVisibleTabRuntimeAfterSnapshot(tab *WorkspaceTab) {
	if tab == nil {
		return
	}
	if tab.Ctrl != nil && tab.hasActiveRuntimeWork() && a.detachSessionRuntime(tab) {
		return
	}
	a.markTabRemoved(tab)
	a.closeTabRuntime(tab)
}

func (a *App) markTabRemoved(tab *WorkspaceTab) {
	a.mu.Lock()
	a.markTabRemovedLocked(tab)
	a.mu.Unlock()
}

func (a *App) markTabRemovedLocked(tab *WorkspaceTab) {
	if tab == nil {
		return
	}
	tab.removed = true
	if tab.buildCancel != nil {
		tab.buildCancel()
		tab.buildCancel = nil
	}
}

func (a *App) tabUnavailableForBuild(tab *WorkspaceTab, generation uint64) bool {
	if tab == nil {
		return true
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return tab.removed || a.tabRuntimes[tab.ID] != tab || (generation != 0 && tab.buildGeneration != generation)
}

func (a *App) clearTabBuildCancel(tab *WorkspaceTab, generation uint64, cancel context.CancelFunc, keepContext bool) {
	if cancel == nil {
		return
	}
	if !keepContext {
		defer cancel()
	}
	if tab == nil {
		return
	}
	a.mu.Lock()
	if tab.buildGeneration == generation {
		tab.buildCancel = nil
	}
	a.mu.Unlock()
}

func (a *App) closeTabRuntime(tab *WorkspaceTab) {
	if tab == nil {
		return
	}
	if tab.Ctrl != nil {
		a.quiesceTabAutosave(tab)   // finish any claimed artifact writer first
		tab.Ctrl.SetSessionPath("") // future snapshots become no-ops
		tab.Ctrl.Cancel()
		tab.Ctrl.Close()
		a.releaseTabSharedHost(tab)
	}
	if tab.sink != nil {
		tab.sink.clearContext()
	}
}

// buildTabController assembles a controller for a tab in the background, the
// same way buildController works for the single-controller App. On success it
// wires the controller and flips Ready; on failure it stores StartupErr.
func (a *App) startTabControllerBuild(tab *WorkspaceTab) {
	a.startTabControllerBuildWithLoadedSession(tab, loadedTabSession{})
}

func (a *App) RetryTabStartup(tabID string) (TabMeta, error) {
	tab := a.tabByID(tabID)
	if tab == nil {
		return TabMeta{}, fmt.Errorf("tab %q not found", tabID)
	}
	if tab.Ctrl != nil {
		a.mu.RLock()
		meta := a.tabMeta(tab, tab.ID == a.cachedActiveTabID())
		a.mu.RUnlock()
		return enrichTabMeta(meta), nil
	}
	preloaded := loadedTabSession{}
	if strings.TrimSpace(tab.SessionPath) != "" {
		var err error
		preloaded, err = a.preloadTabSession(tab, tab.SessionPath)
		if err != nil {
			return TabMeta{}, err
		}
	}
	if err := a.scheduleTabControllerBuild(tab, preloaded, a.ctx != nil, func(current *WorkspaceTab) error {
		if current.Ctrl != nil {
			return fmt.Errorf("tab %q is already ready", tabID)
		}
		return nil
	}); err != nil {
		return TabMeta{}, err
	}
	a.mu.RLock()
	meta := a.tabMeta(tab, tab.ID == a.cachedActiveTabID())
	a.mu.RUnlock()
	return enrichTabMeta(meta), nil
}

func (a *App) startTabControllerBuildWithLoadedSession(tab *WorkspaceTab, loadedSession loadedTabSession) {
	_ = a.scheduleTabControllerBuild(tab, loadedSession, a.ctx != nil, nil)
}

func (a *App) startTabControllerBuildWithLoadedSessionForRoute(tab *WorkspaceTab, loadedSession loadedTabSession, route *modelbilling.BillingRouteSelection) {
	_ = a.scheduleTabControllerBuildWithRoute(tab, loadedSession, a.ctx != nil, nil, route)
}

func (a *App) scheduleTabControllerBuild(tab *WorkspaceTab, loadedSession loadedTabSession, async bool, mutateLocked func(*WorkspaceTab) error) error {
	return a.scheduleTabControllerBuildWithRoute(tab, loadedSession, async, mutateLocked, nil)
}

func (a *App) scheduleTabControllerBuildWithRoute(tab *WorkspaceTab, loadedSession loadedTabSession, async bool, mutateLocked func(*WorkspaceTab) error, route *modelbilling.BillingRouteSelection) error {
	return a.scheduleTabControllerBuildModeWithRoute(tab, loadedSession, async, false, mutateLocked, route)
}

func (a *App) scheduleTabControllerBuildDeferred(tab *WorkspaceTab, loadedSession loadedTabSession) error {
	return a.scheduleTabControllerBuildMode(tab, loadedSession, true, true, nil)
}

func (a *App) scheduleTabControllerBuildMode(tab *WorkspaceTab, loadedSession loadedTabSession, async, deferPreparation bool, mutateLocked func(*WorkspaceTab) error) error {
	return a.scheduleTabControllerBuildModeWithRoute(tab, loadedSession, async, deferPreparation, mutateLocked, nil)
}

func (a *App) scheduleTabControllerBuildModeWithRoute(tab *WorkspaceTab, loadedSession loadedTabSession, async, deferPreparation bool, mutateLocked func(*WorkspaceTab) error, route *modelbilling.BillingRouteSelection) error {
	buildCtx, cancel := context.WithCancel(a.bootContext())
	buildRoute := cloneBillingRouteSelection(route)
	a.mu.Lock()
	if tab == nil || tab.removed {
		a.mu.Unlock()
		cancel()
		return fmt.Errorf("tab not found")
	}
	if mutateLocked != nil {
		if err := mutateLocked(tab); err != nil {
			a.mu.Unlock()
			cancel()
			return err
		}
	}
	previousCancel := tab.buildCancel
	clearTabStartupFailureLocked(tab)
	tab.Ready = false
	tab.buildGeneration++
	generation := tab.buildGeneration
	tab.buildCancel = cancel
	a.mu.Unlock()
	if previousCancel != nil {
		previousCancel()
	}

	if deferPreparation {
		go func() {
			prepared := loadedSession
			var err error
			if prepared.Session == nil && prepared.Err == nil && strings.TrimSpace(prepared.Path) != "" {
				prepared.Session, err = preloadTabSessionFile(prepared.Path)
			} else {
				prepared, err = a.prepareTabSessionForBuild(tab, loadedSession)
			}
			if err != nil {
				cancel()
				a.failTabControllerBuild(tab, generation, err)
				return
			}
			if turns := prepared.sessionTurns(); turns != nil {
				a.mu.Lock()
				if !tab.removed && a.tabRuntimes[tab.ID] == tab && tab.buildGeneration == generation {
					tab.sessionTurns = turns
				}
				a.mu.Unlock()
			}
			a.buildTabControllerWithRoute(tab, prepared, buildCtx, generation, cancel, buildRoute)
		}()
		return nil
	}

	prepared, err := a.prepareTabSessionForBuild(tab, loadedSession)
	if err != nil {
		cancel()
		a.failTabControllerBuild(tab, generation, err)
		return nil
	}
	if turns := prepared.sessionTurns(); turns != nil {
		a.mu.Lock()
		if !tab.removed && a.tabRuntimes[tab.ID] == tab && tab.buildGeneration == generation {
			tab.sessionTurns = turns
		}
		a.mu.Unlock()
	}
	if !async {
		a.buildTabControllerWithRoute(tab, prepared, buildCtx, generation, cancel, buildRoute)
		return nil
	}
	go a.buildTabControllerWithRoute(tab, prepared, buildCtx, generation, cancel, buildRoute)
	return nil
}

func (a *App) buildTabController(tab *WorkspaceTab) {
	a.buildTabControllerWithLoadedSession(tab, loadedTabSession{})
}

type loadedTabSession struct {
	Path    string
	Session *agent.Session
	Err     error
}

var preloadTabSessionFile = loadResumableSession

func intPointer(value int) *int {
	return &value
}

func cloneIntPointer(value *int) *int {
	if value == nil {
		return nil
	}
	return intPointer(*value)
}

func (s loadedTabSession) sessionTurns() *int {
	if s.Session == nil {
		return nil
	}
	_, turns := agent.SessionPreviewFromMessages(s.Session.Snapshot())
	return intPointer(turns)
}

func (s loadedTabSession) matches(path string) bool {
	return s.Session != nil && sessionRuntimeKey(s.Path) != "" && sessionRuntimeKey(s.Path) == sessionRuntimeKey(path)
}

func (a *App) preloadTabSession(tab *WorkspaceTab, sessionPath string) (loadedTabSession, error) {
	_, path, err := resolveTabSessionPath(tab, sessionPath)
	if err != nil {
		return loadedTabSession{}, err
	}
	loaded, err := preloadTabSessionFile(path)
	if err != nil {
		return loadedTabSession{Path: path, Err: err}, err
	}
	return loadedTabSession{Path: path, Session: loaded}, nil
}

// restoredTabSessionPath keeps startup recovery tolerant without weakening
// explicit history/session operations. Invalid or missing persisted references
// are replaced with a pinned empty file so topic fallback cannot load unrelated
// history.
func restoredTabSessionPath(tab *WorkspaceTab, sessionPath string) (string, bool) {
	sessionPath = strings.TrimSpace(sessionPath)
	if sessionPath == "" {
		return "", false
	}

	_, validPath, err := resolveTabSessionPath(tab, sessionPath)
	if err == nil {
		if _, statErr := os.Stat(validPath); statErr == nil || !os.IsNotExist(statErr) {
			return validPath, false
		} else {
			err = statErr
		}
	}

	replacement, createErr := createEmptySessionFile(tabSessionDir(tab), tab.model)
	if createErr != nil {
		slog.Warn("invalid persisted tab session path; blank session file creation failed; keeping strict startup failure",
			diagnostics.FailureMarkerKey, diagnostics.FailureMarker,
			"scope", "session.recovery",
			"tab_id", tab.ID,
			"session_path", sessionPath,
			"reason", err,
			"err", errors.Join(err, createErr),
		)
		return sessionPath, false
	}
	slog.Warn("invalid persisted tab session path; using blank session",
		diagnostics.FailureMarkerKey, diagnostics.FailureMarker,
		"scope", "session.recovery",
		"tab_id", tab.ID,
		"session_path", sessionPath,
		"replacement", replacement,
		"err", err,
	)
	return replacement, true
}

func resolveTabSessionPath(tab *WorkspaceTab, sessionPath string) (string, string, error) {
	if tab == nil {
		return "", "", fmt.Errorf("tab not found")
	}
	dirs := []string{tabSessionDir(tab)}
	actor := tabRuntimeActorContext(tab).Normalize()
	if actor.ProfileID == userctx.DefaultProfileID {
		legacyDir := config.SessionDir()
		if filepath.Clean(legacyDir) != filepath.Clean(dirs[0]) {
			dirs = append(dirs, legacyDir)
		}
	}
	var validationErr error
	for _, dir := range dirs {
		path, _, err := validateSessionPath(dir, sessionPath)
		if err != nil {
			validationErr = err
			continue
		}
		if filepath.Clean(dir) == filepath.Clean(config.SessionDir()) && tab.Scope == "project" {
			meta, ok, metaErr := agent.LoadBranchMeta(path)
			if metaErr != nil {
				return "", "", metaErr
			}
			if !ok || meta.DefaultScope() != "project" || normalizeProjectRoot(meta.WorkspaceRoot) != normalizeProjectRoot(tab.WorkspaceRoot) {
				validationErr = fmt.Errorf("session path does not belong to project session dir: %s", sessionPath)
				continue
			}
		}
		return dir, path, nil
	}
	if validationErr == nil {
		validationErr = fmt.Errorf("session path is required")
	}
	return "", "", validationErr
}

func (a *App) prepareTabSessionForBuild(tab *WorkspaceTab, preloaded loadedTabSession) (loadedTabSession, error) {
	if tab == nil {
		return loadedTabSession{}, fmt.Errorf("tab not found")
	}
	a.reconcileTabWithPinnedSessionMeta(tab)
	if preloaded.Err != nil && sessionRuntimeKey(preloaded.Path) == sessionRuntimeKey(tab.SessionPath) {
		return loadedTabSession{}, preloaded.Err
	}
	if preloaded.matches(tab.SessionPath) {
		return preloaded, nil
	}
	if strings.TrimSpace(tab.SessionPath) == "" {
		return loadedTabSession{}, nil
	}
	return a.preloadTabSession(tab, tab.SessionPath)
}

func (a *App) failTabControllerBuild(tab *WorkspaceTab, generation uint64, err error) {
	wailsCtx := a.ctx
	a.mu.Lock()
	if tab == nil || tab.removed || a.tabRuntimes[tab.ID] != tab || (generation != 0 && tab.buildGeneration != generation) {
		a.mu.Unlock()
		return
	}
	setTabStartupFailureLocked(tab, err, tabStartupFailureSession)
	tab.Ready = true
	if tab.buildGeneration == generation {
		tab.buildCancel = nil
	}
	a.saveTabsLocked()
	a.mu.Unlock()
	a.emitReady(wailsCtx)
}

func tabActorContext(tab *WorkspaceTab, workspaceRoot string) userctx.ActorContext {
	var actor userctx.ActorContext
	if tab != nil {
		actor = tab.actor
	}
	actor = actor.Normalize()
	actor.WorkspaceRoot = workspaceRoot
	return actor
}

func actorContextSpecified(actor userctx.ActorContext) bool {
	return actor != (userctx.ActorContext{})
}

func actorContextForWorkspace(actor userctx.ActorContext, workspaceRoot string) userctx.ActorContext {
	actor = actor.Normalize()
	actor.WorkspaceRoot = strings.TrimSpace(workspaceRoot)
	return actor
}

func tabActorWorkspaceRoot(tab *WorkspaceTab) string {
	if tab == nil {
		return ""
	}
	root := strings.TrimSpace(tab.WorkspaceRoot)
	if root == "" && tab.Scope == "global" {
		return globalWorkspaceRoot()
	}
	return root
}

func tabRuntimeActorContext(tab *WorkspaceTab) userctx.ActorContext {
	if tab == nil {
		return userctx.ActorContext{}.Normalize()
	}
	if tab.Ctrl != nil {
		actor := tab.Ctrl.ActorContext().Normalize()
		if strings.TrimSpace(actor.WorkspaceRoot) == "" {
			actor.WorkspaceRoot = tabActorWorkspaceRoot(tab)
		}
		return actor
	}
	return tabActorContext(tab, tabActorWorkspaceRoot(tab))
}

func tabRuntimeActorMatches(tab *WorkspaceTab, actor userctx.ActorContext, workspaceRoot string) bool {
	return actorContextsMatch(tabRuntimeActorContext(tab), actorContextForWorkspace(actor, workspaceRoot))
}

func tabRuntimeActorsMatch(a, b *WorkspaceTab) bool {
	return actorContextsMatch(tabRuntimeActorContext(a), tabRuntimeActorContext(b))
}

func actorProfilesMatch(a, b userctx.ActorContext) bool {
	a = a.Normalize()
	b = b.Normalize()
	return a.ActorID == b.ActorID && a.ProfileID == b.ProfileID && a.AuthMethod == b.AuthMethod
}

func actorContextsMatch(a, b userctx.ActorContext) bool {
	a = a.Normalize()
	b = b.Normalize()
	a.WorkspaceRoot = cleanActorWorkspaceRoot(a.WorkspaceRoot)
	b.WorkspaceRoot = cleanActorWorkspaceRoot(b.WorkspaceRoot)
	return reflect.DeepEqual(a, b)
}

func cleanActorWorkspaceRoot(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		return ""
	}
	return filepath.Clean(root)
}

func (a *App) buildTabControllerWithLoadedSession(tab *WorkspaceTab, loadedSession loadedTabSession) {
	_ = a.scheduleTabControllerBuild(tab, loadedSession, false, nil)
}

func (a *App) buildTabControllerWithContext(tab *WorkspaceTab, loadedSession loadedTabSession, buildCtx context.Context, buildGeneration uint64, buildCancel context.CancelFunc) {
	a.buildTabControllerWithRoute(tab, loadedSession, buildCtx, buildGeneration, buildCancel, nil)
}

func cloneBillingRouteSelection(route *modelbilling.BillingRouteSelection) *modelbilling.BillingRouteSelection {
	if route == nil {
		return nil
	}
	copy := *route
	copy.ModelRef = strings.TrimSpace(copy.ModelRef)
	copy.BillingSource = modelbilling.BillingSource(strings.TrimSpace(string(copy.BillingSource)))
	return &copy
}

func (a *App) buildTabControllerWithRoute(tab *WorkspaceTab, loadedSession loadedTabSession, buildCtx context.Context, buildGeneration uint64, buildCancel context.CancelFunc, buildRoute *modelbilling.BillingRouteSelection) {
	defer a.recoverToPending("buildTabController")
	keepBuildContext := false
	defer func() {
		a.clearTabBuildCancel(tab, buildGeneration, buildCancel, keepBuildContext)
	}()
	wailsCtx := a.ctx
	if a.tabUnavailableForBuild(tab, buildGeneration) {
		return
	}
	billingSource := modelbilling.BillingSource(tab.billingSource)
	explicitModel := ""
	if buildRoute != nil {
		explicitModel = strings.TrimSpace(buildRoute.ModelRef)
		billingSource = modelbilling.BillingSource(strings.TrimSpace(string(buildRoute.BillingSource)))
		if _, err := modelbilling.NewBillingRouteSelection(modelbilling.BillingRouteSelection{ModelRef: explicitModel, BillingSource: billingSource}); err != nil {
			a.mu.Lock()
			if tab.removed || a.tabRuntimes[tab.ID] != tab || (buildGeneration != 0 && tab.buildGeneration != buildGeneration) {
				a.mu.Unlock()
				return
			}
			setTabStartupFailureLocked(tab, err, tabStartupFailureConfig)
			tab.Ready = true
			a.saveTabsLocked()
			a.mu.Unlock()
			a.emitReady(wailsCtx)
			return
		}
	}

	root := tab.WorkspaceRoot
	if root == "" {
		if wd, err := os.Getwd(); err == nil {
			root = wd
		}
	}

	// Subscription tabs use provider metadata only; loading legacy credentials is
	// reserved for the explicit BYOK route.
	if billingSource == modelbilling.BillingNewAPIByok {
		if err := config.MigrateLegacyCredentialsForRoot(root); err != nil {
			recordDesktopBackgroundFailure("config.migrate.credentials", err)
		}
	}
	actor := tabActorContext(tab, root)
	cfg, err := a.desktopConfigForBillingRoute(root, actor, billingSource)
	if err != nil {
		a.mu.Lock()
		if tab.removed || a.tabRuntimes[tab.ID] != tab || (buildGeneration != 0 && tab.buildGeneration != buildGeneration) {
			a.mu.Unlock()
			return
		}
		setTabStartupFailureLocked(tab, err, tabStartupFailureConfig)
		tab.Ready = true
		a.saveTabsLocked()
		a.mu.Unlock()
		a.emitReady(wailsCtx)
		return
	}
	if buildRoute != nil {
		entry, ok := cfg.ResolveModel(explicitModel)
		if !ok || entry.Name+"/"+entry.Model != explicitModel {
			err := fmt.Errorf("automation: model route %q is unavailable from %s", explicitModel, billingSource)
			a.mu.Lock()
			if tab.removed || a.tabRuntimes[tab.ID] != tab || (buildGeneration != 0 && tab.buildGeneration != buildGeneration) {
				a.mu.Unlock()
				return
			}
			setTabStartupFailureLocked(tab, err, tabStartupFailureConfig)
			tab.Ready = true
			a.saveTabsLocked()
			a.mu.Unlock()
			a.emitReady(wailsCtx)
			return
		}
		if billingSource != modelbilling.BillingSubscriptionPool && !modelProviderAccessAllowed(providerAccessSet(cfg.Desktop.ProviderAccess), entry.Name) {
			err := fmt.Errorf("automation: model route %q is unavailable because provider %q is not added", explicitModel, entry.Name)
			a.mu.Lock()
			if tab.removed || a.tabRuntimes[tab.ID] != tab || (buildGeneration != 0 && tab.buildGeneration != buildGeneration) {
				a.mu.Unlock()
				return
			}
			setTabStartupFailureLocked(tab, err, tabStartupFailureConfig)
			tab.Ready = true
			a.saveTabsLocked()
			a.mu.Unlock()
			a.emitReady(wailsCtx)
			return
		}
	}

	a.mu.RLock()
	if tab.removed || a.tabRuntimes[tab.ID] != tab || (buildGeneration != 0 && tab.buildGeneration != buildGeneration) {
		a.mu.RUnlock()
		return
	}
	sink := tab.sink
	a.mu.RUnlock()
	if sink != nil {
		sink.setContext(wailsCtx)
	}

	sessionDir := tabSessionDir(tab)
	topicID := strings.TrimSpace(tab.TopicID)

	// Assign Global topics to legacy sessions in the global session dir so
	// imported history appears in the project tree regardless of which tab
	// triggered the build (the migration now sends everything to global).
	migratedGlobalTopics := migrateLegacySessionsIntoGlobalTopics(config.SessionDir())
	if len(migratedGlobalTopics) > 0 {
		a.emitProjectTreeChanged()
	}
	if tab.Scope == "global" && topicID == "" && len(migratedGlobalTopics) > 0 {
		topicID = migratedGlobalTopics[0]
		topicTitle := topicTitleForTab("global", "", topicID)
		a.mu.Lock()
		if !tab.removed && a.tabRuntimes[tab.ID] == tab && (buildGeneration == 0 || tab.buildGeneration == buildGeneration) && strings.TrimSpace(tab.TopicID) == "" {
			tab.TopicID = topicID
			tab.TopicTitle = topicTitle
			a.saveTabsLocked()
		} else if !tab.removed && a.tabRuntimes[tab.ID] == tab && (buildGeneration == 0 || tab.buildGeneration == buildGeneration) {
			topicID = strings.TrimSpace(tab.TopicID)
		} else {
			a.mu.Unlock()
			return
		}
		a.mu.Unlock()
	}
	if loadedSession.Path != "" {
		sessionDir = filepath.Dir(loadedSession.Path)
	} else if topicID != "" {
		if _, dir := a.findTopicSessionForTargetForActor(tab.Scope, tab.WorkspaceRoot, topicID, tabRuntimeActorContext(tab)); dir != "" {
			sessionDir = dir
		}
	}
	startupSessionPath := loadedSession.Path
	if startupSessionPath == "" {
		if pinnedPath, ok := pinnedTabSessionPath(sessionDir, tab.SessionPath); ok {
			if !agent.IsCleanupPending(pinnedPath) {
				startupSessionPath = pinnedPath
			}
		} else if topicID != "" {
			startupSessionPath = findTopicSession(sessionDir, topicID)
		}
	}

	model := explicitModel
	if buildRoute == nil {
		model = strings.TrimSpace(tab.model)
		if sessionModel, ok := agent.LoadSessionModel(startupSessionPath); ok {
			config.NormalizeLegacyMimoCustomProvidersForRefs(cfg, sessionModel)
			if _, ok := cfg.ResolveModel(sessionModel); ok {
				model = sessionModel
			}
		}
		if model == "" && topicID != "" {
			if topicModel := strings.TrimSpace(loadTopicModel(topicTitleRoot(tab.Scope, tab.WorkspaceRoot), topicID)); topicModel != "" {
				model = topicModel
			}
		}
		if model == "" {
			model = cfg.DefaultModel
		}
	}
	requestedModel := model
	subscriptionFallback := false
	if buildRoute == nil && billingSource == modelbilling.BillingSubscriptionPool {
		model = availableSubscriptionModelFromConfig(cfg, model)
		subscriptionFallback = model != strings.TrimSpace(requestedModel)
	}
	sessionExpertID := ""
	if startupSessionPath != "" {
		if meta, ok, err := agent.LoadSessionExpert(startupSessionPath); err == nil {
			if ok {
				sessionExpertID = meta.ExpertID
			} else {
				sessionExpertID = expert.ExplicitNoneID
			}
		}
	}
	if buildRoute == nil {
		config.NormalizeLegacyMimoCustomProvidersForRefs(cfg, model)
		if resolved, fallback, ok := cfg.ResolveModelWithFallback(model); ok {
			if (subscriptionFallback || fallback) && strings.TrimSpace(tab.model) != "" {
				a.noticeForTab(tab.ID, fmt.Sprintf("model %q is no longer available; switched to %s", requestedModel, resolved))
			}
			model = resolved
		}
	}
	if billingSource == modelbilling.BillingNewAPIByok {
		entry, ok := cfg.ResolveModel(model)
		if !ok || !entry.Configured() {
			err := &modelbilling.Error{Code: modelbilling.ErrorByokCredentialInvalid, Message: fmt.Sprintf("model %q requires a configured Base URL and API key", model)}
			a.mu.Lock()
			if tab.removed || a.tabRuntimes[tab.ID] != tab || (buildGeneration != 0 && tab.buildGeneration != buildGeneration) {
				a.mu.Unlock()
				return
			}
			setTabStartupFailureLocked(tab, err, tabStartupFailureConfig)
			tab.Ready = true
			a.saveTabsLocked()
			a.mu.Unlock()
			a.emitReady(wailsCtx)
			return
		}
	}

	a.mu.Lock()
	if tab.removed || a.tabRuntimes[tab.ID] != tab || (buildGeneration != 0 && tab.buildGeneration != buildGeneration) {
		a.mu.Unlock()
		return
	}
	tab.model = model
	if buildRoute != nil {
		tab.billingSource = string(billingSource)
	}
	tab.Label = model
	a.saveTabsLocked()
	a.mu.Unlock()

	// Acquire a shared plugin host for this profile/workspace pair so MCP
	// processes are launched once per matching tab, not once per tab.
	hostKey := sharedHostKeyForActor(actor, tab.WorkspaceRoot)
	sharedHost := a.acquireSharedHost(hostKey)

	bootOptions := boot.Options{
		Model:                    model,
		RequireKey:               false,
		Sink:                     tab.sink,
		WorkspaceRoot:            root,
		Actor:                    actor,
		SessionDir:               sessionDir,
		EffortOverride:           cloneStringPtr(tab.effort),
		TokenMode:                currentTabTokenMode(tab),
		SharedHost:               sharedHost,
		CleanupPendingReconciler: reconcileDesktopCleanupPending,
		SessionExpertID:          sessionExpertID,
	}
	applyTabSceneBootOptions(&bootOptions, tab)
	ctrl, err := a.buildTabControllerRuntime(buildCtx, bootOptions, actor, root, model, string(billingSource))
	if err != nil {
		a.mu.Lock()
		if tab.removed || a.tabRuntimes[tab.ID] != tab || (buildGeneration != 0 && tab.buildGeneration != buildGeneration) {
			a.mu.Unlock()
			a.releaseSharedHost(hostKey)
			return
		}
		setTabStartupFailureLocked(tab, err, tabStartupFailureController)
		tab.Ready = true
		a.mu.Unlock()
		a.releaseSharedHost(hostKey)
		a.emitReady(wailsCtx)
		return
	}
	if a.tabUnavailableForBuild(tab, buildGeneration) {
		ctrl.Close()
		a.releaseSharedHost(hostKey)
		return
	}

	a.bindControllerDisplayRecorder(ctrl)
	ctrl.EnableInteractiveApproval()
	applyTabModeToController(ctrl, tab.mode)
	applyTabToolApprovalModeToController(ctrl, tab.toolApprovalMode)
	ctrl.SetGoal(tab.goal)

	if dir := ctrl.SessionDir(); dir != "" {
		migratedTopics := migrateLegacySessionsIntoGlobalTopics(dir)
		if len(migratedTopics) > 0 {
			a.emitProjectTreeChanged()
		}
		if tab.Scope == "global" && strings.TrimSpace(tab.TopicID) == "" && len(migratedTopics) > 0 {
			topicID := migratedTopics[0]
			topicTitle := topicTitleForTab("global", "", topicID)
			a.mu.Lock()
			if !tab.removed && a.tabRuntimes[tab.ID] == tab && (buildGeneration == 0 || tab.buildGeneration == buildGeneration) {
				tab.TopicID = topicID
				tab.TopicTitle = topicTitle
				a.saveTabsLocked()
			}
			a.mu.Unlock()
		}
		var path string
		// Prefer the exact session file persisted for this tab. Topic lookup is a
		// compatibility fallback for older desktop-tabs.json files that only stored
		// topicId and could pick the wrong session when one topic had multiple files.
		if loaded, pinnedPath, ok := loadPinnedTabSessionWithPreload(dir, tab.SessionPath, loadedSession); ok {
			if loaded != nil {
				ctrl.Resume(loaded, pinnedPath)
			} else {
				ctrl.SetSessionPath(pinnedPath)
			}
			path = pinnedPath
		}
		if path == "" && tab.TopicID != "" {
			existingPath := findTopicSession(dir, tab.TopicID)
			if existingPath != "" {
				if loaded, err := loadResumableSession(existingPath); err == nil {
					ctrl.Resume(loaded, existingPath)
					path = existingPath
				}
			}
		}
		if path == "" {
			path = agent.NewSessionPath(dir, ctrl.Label())
			ctrl.SetSessionPath(path)
		}
		// Write/update scope/session meta.
		if path != "" {
			if !a.persistTabSessionPathForBuild(tab, path, buildGeneration) {
				ctrl.Close()
				a.releaseSharedHost(hostKey)
				return
			}
			if strings.TrimSpace(tab.TopicID) != "" {
				if err := ensureTopicIndexed(tab.Scope, tab.WorkspaceRoot, tab.TopicID, tab.TopicTitle, loadTopicTitleSource(topicTitleRoot(tab.Scope, tab.WorkspaceRoot), tab.TopicID)); err == nil {
					a.emitProjectTreeChanged()
				}
			}
			// Restore existing telemetry if resuming a session.
			telemetryPath := path + ".telemetry.json"
			if snapshot := loadTelemetry(telemetryPath); len(snapshot.ReadFiles) > 0 || snapshot.Usage.RequestCount > 0 {
				a.mu.RLock()
				if tab.removed || a.tabRuntimes[tab.ID] != tab || (buildGeneration != 0 && tab.buildGeneration != buildGeneration) {
					a.mu.RUnlock()
					ctrl.Close()
					a.releaseSharedHost(hostKey)
					return
				}
				tab.telemMu.Lock()
				tab.readTelemetry = snapshot.ReadFiles
				tab.usageTelemetry = snapshot.Usage
				tab.telemMu.Unlock()
				a.mu.RUnlock()
			}
			attachStatus := a.waitForExistingSessionRuntimeAttach(tab, path, wailsCtx, buildGeneration)
			if attachStatus == runtimeAttachComplete {
				ctrl.Close()
				a.releaseSharedHost(hostKey)
				a.emitReady(wailsCtx)
				return
			}
			if attachStatus == runtimeAttachBusy {
				ctrl.Close()
				a.releaseSharedHost(hostKey)
				err := fmt.Errorf("existing session runtime hand-off did not settle")
				a.mu.Lock()
				if !tab.removed && a.tabRuntimes[tab.ID] == tab && (buildGeneration == 0 || tab.buildGeneration == buildGeneration) {
					setTabStartupFailureLocked(tab, err, tabStartupFailureController)
					tab.Ready = true
				}
				a.mu.Unlock()
				a.emitReady(wailsCtx)
				return
			}
		}
	}

	_, exactSessionTurns := agent.SessionPreviewFromMessages(ctrl.History())
	a.mu.Lock()
	if tab.removed || a.tabRuntimes[tab.ID] != tab || (buildGeneration != 0 && tab.buildGeneration != buildGeneration) {
		a.mu.Unlock()
		ctrl.Close()
		a.releaseSharedHost(hostKey)
		return
	}
	tab.SharedHostKey = hostKey
	tab.Ctrl = ctrl
	tab.sessionTurns = intPointer(exactSessionTurns)
	tab.Label = ctrl.Label()
	tab.Ready = true
	clearTabStartupFailureLocked(tab)
	keepBuildContext = true
	a.mu.Unlock()
	a.emitReady(wailsCtx)
}

func (a *App) buildTabControllerRuntime(ctx context.Context, options boot.Options, actor userctx.ActorContext, root, model, billingSource string) (*control.Controller, error) {
	if a.tabControllerBuilder != nil {
		return a.tabControllerBuilder(ctx, options, actor, root, model, billingSource)
	}
	return a.buildDesktopController(ctx, options, actor, root, model, billingSource)
}

func (a *App) reconcileTabWithPinnedSessionMeta(tab *WorkspaceTab) {
	if tab == nil || strings.TrimSpace(tab.SessionPath) == "" {
		return
	}
	path, meta, ok := a.pinnedSessionMeta(tab)
	if !ok {
		return
	}

	scope, workspaceRoot := pinnedSessionRoute(tab, path, meta)
	if scope == "project" {
		if err := addProject(workspaceRoot, ""); err != nil {
			recordDesktopBackgroundFailure("project.register.pinned", err)
		}
	}

	topicID := strings.TrimSpace(meta.TopicID)
	topicTitle := ""
	if topicID != "" {
		topicTitle = strings.TrimSpace(loadTopicTitle(topicTitleRoot(scope, workspaceRoot), topicID))
	}
	if topicTitle == "" {
		topicTitle = strings.TrimSpace(meta.TopicTitle)
	}
	if topicTitle == "" && topicID != "" {
		topicTitle = topicTitleForTab(scope, workspaceRoot, topicID)
	}
	status := scene.Status{Scene: tab.Scene, State: tab.SceneState}
	if meta.Scene != "" || meta.SceneState != "" {
		var sceneErr error
		status, sceneErr = meta.SceneStatus()
		if sceneErr != nil {
			slog.Warn("invalid session scene metadata; using legacy compatibility", "session_path", path, "error", sceneErr)
			status = legacyDesktopSceneStatus()
		}
	}

	a.mu.Lock()
	current := a.tabRuntimes[tab.ID]
	if current != nil && current != tab {
		a.mu.Unlock()
		return
	}
	changed := tab.Scope != scope ||
		tab.WorkspaceRoot != workspaceRoot ||
		canonicalTabSessionPath(tab.SessionPath) != canonicalTabSessionPath(path) ||
		tab.Scene != status.Scene || tab.SceneState != status.State
	tab.Scope = scope
	tab.WorkspaceRoot = workspaceRoot
	tab.SessionPath = canonicalTabSessionPath(path)
	setTabSceneStatus(tab, status)
	if topicID != "" {
		changed = changed || tab.TopicID != topicID
		tab.TopicID = topicID
	}
	if topicTitle != "" {
		changed = changed || tab.TopicTitle != topicTitle
		tab.TopicTitle = topicTitle
	}
	if changed && current == tab {
		a.saveTabsLocked()
	}
	a.mu.Unlock()
}

// pinnedSessionRoute resolves routing metadata without letting a freshly
// created sidecar erase the tab's project ownership. EnsureBranchMeta can
// create an ID-only sidecar before the first controller snapshot; historically
// DefaultScope interpreted that missing scope as global and moved a new project
// tab away from the directory that already contained its pinned session.
//
// For those ID-only sidecars, keep a valid current route. If an affected build
// already persisted the wrong global route, recover it by matching the pinned
// path to the registered project session directories for the same actor.
func pinnedSessionRoute(tab *WorkspaceTab, path string, meta agent.BranchMeta) (string, string) {
	if strings.TrimSpace(meta.Scope) != "" || strings.TrimSpace(meta.WorkspaceRoot) != "" {
		scope := meta.DefaultScope()
		if scope == "project" {
			if root := normalizeProjectRoot(meta.WorkspaceRoot); root != "" {
				return scope, root
			}
			return currentPinnedSessionRoute(tab)
		}
		return "global", globalTabWorkspaceRoot()
	}

	actor := tabRuntimeActorContext(tab)
	if scope, root, ok := routeForPinnedSessionPath(actor, tab.Scope, tab.WorkspaceRoot, path); ok {
		return scope, root
	}
	return currentPinnedSessionRoute(tab)
}

func currentPinnedSessionRoute(tab *WorkspaceTab) (string, string) {
	if tab != nil && tab.Scope == "project" {
		if root := normalizeProjectRoot(tab.WorkspaceRoot); root != "" {
			return "project", root
		}
	}
	return "global", globalTabWorkspaceRoot()
}

func routeForPinnedSessionPath(actor userctx.ActorContext, currentScope, currentRoot, path string) (string, string, bool) {
	matches := func(dir string) bool {
		_, _, err := validateSessionPath(dir, path)
		return err == nil
	}

	if currentScope == "project" {
		if root := normalizeProjectRoot(currentRoot); root != "" && matches(desktopSessionDirForActor(actor, root)) {
			return "project", root, true
		}
	} else if matches(desktopSessionDirForActor(actor, globalWorkspaceRoot())) {
		return "global", globalTabWorkspaceRoot(), true
	}

	for _, project := range loadProjectsFile().Projects {
		root := normalizeProjectRoot(project.Root)
		if root != "" && matches(desktopSessionDirForActor(actor, root)) {
			return "project", root, true
		}
	}

	if matches(desktopSessionDirForActor(actor, "")) {
		return "global", globalTabWorkspaceRoot(), true
	}
	if actor.Normalize().ProfileID == userctx.DefaultProfileID && matches(config.SessionDir()) {
		return "global", globalTabWorkspaceRoot(), true
	}
	return "", "", false
}

func (a *App) pinnedSessionMeta(tab *WorkspaceTab) (string, agent.BranchMeta, bool) {
	if tab == nil {
		return "", agent.BranchMeta{}, false
	}
	sessionPath := strings.TrimSpace(tab.SessionPath)
	if sessionPath == "" {
		return "", agent.BranchMeta{}, false
	}
	actor := tabRuntimeActorContext(tab)
	for _, dir := range a.sessionDirsForActorProfile(actor) {
		path, _, err := validateSessionPath(dir, sessionPath)
		if err != nil {
			continue
		}
		if meta, ok, err := agent.LoadBranchMeta(path); err == nil && ok {
			return path, meta, true
		}
	}
	return "", agent.BranchMeta{}, false
}

// --- active tab helpers -----------------------------------------------------

func (a *App) cachedActiveTabID() string {
	if a == nil {
		return ""
	}
	if value := a.activeTabIDValue.Load(); value != nil {
		return *value
	}
	// Compatibility for package tests and the short startup interval before the
	// SQLite state has been restored.
	return a.activeTabID
}

func (a *App) publishActiveTabID(tabID string) {
	value := strings.TrimSpace(tabID)
	a.activeTabIDValue.Store(&value)
}

func (a *App) advanceTabsSaveVersion(version uint64) {
	for current := a.tabsSaveVersion.Load(); version > current; current = a.tabsSaveVersion.Load() {
		if a.tabsSaveVersion.CompareAndSwap(current, version) {
			return
		}
	}
}

// setActiveTabIDLocked updates the legacy startup field and its lock-free
// runtime mirror. Callers changing the in-memory tab collection already hold
// a.mu; SetActiveTab itself commits through SQLite and only publishes the
// atomic mirror.
func (a *App) setActiveTabIDLocked(tabID string) {
	a.activeTabID = strings.TrimSpace(tabID)
	a.publishActiveTabID(a.activeTabID)
}

func (a *App) publishTabActivationSnapshotLocked() {
	if a.currentActorValue.Load() == nil {
		a.currentActorValue.Store(&appActorSnapshot{
			actor:    a.currentActor.Normalize(),
			explicit: a.currentActorExplicit,
		})
	}
	byID := make(map[string]tabActivationRuntime, len(a.tabRuntimes))
	for id, tab := range a.tabRuntimes {
		if tab == nil {
			continue
		}
		byID[id] = tabActivationRuntime{tab: tab, actor: tabRuntimeActorContext(tab).Normalize()}
	}
	a.tabActivationIndex.Store(&tabActivationSnapshot{byID: byID})
}

func (a *App) activationSnapshot() *tabActivationSnapshot {
	if a == nil {
		return nil
	}
	return a.tabActivationIndex.Load()
}

func activationActor(record tabActivationRuntime, state appActorSnapshot) userctx.ActorContext {
	if state.explicit {
		return state.actor.Normalize()
	}
	return record.actor.Normalize()
}

// activeTab returns the currently active tab (nil when there are no tabs).
// Self-locking; safe to call from any goroutine without external lock.
func (a *App) activeTab() *WorkspaceTab {
	a.mu.RLock()
	defer a.mu.RUnlock()
	activeTabID := a.persistedActiveTabIDLocked()
	if activeTabID == "" {
		return nil
	}
	return a.tabRuntimes[activeTabID]
}

// activeTabLocked is like activeTab but assumes the caller already holds a.mu
// (either RLock or Lock). Use this inside critical sections that already own
// the lock to avoid double-locking a write-lock holder.
func (a *App) activeTabLocked() *WorkspaceTab {
	activeTabID := a.persistedActiveTabIDLocked()
	if activeTabID == "" {
		return nil
	}
	return a.tabRuntimes[activeTabID]
}

// persistedActiveTabIDLocked reads the active tab directly from SQLite. The
// in-memory value is only a compatibility fallback for a tab that has been
// constructed but not committed yet (primarily package-level test fixtures).
func (a *App) persistedActiveTabIDLocked() string {
	if !a.tabStoreReady.Load() {
		return a.cachedActiveTabID()
	}
	id, _, err := newDesktopTabStore(a.desktopConfigDirLocked()).activeTabID()
	if err == nil {
		id = strings.TrimSpace(id)
		a.publishActiveTabID(id)
		return id
	}
	return a.cachedActiveTabID()
}

// activeCtrl returns the controller of the active tab, or nil.
// Self-locking; safe to call from any goroutine without external lock.
func (a *App) activeCtrl() control.SessionAPI {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.activeCtrlLocked()
}

// activeCtrlLocked is like activeCtrl but assumes the caller already holds a.mu.
func (a *App) activeCtrlLocked() control.SessionAPI {
	t := a.activeTabLocked()
	if t == nil {
		return nil
	}
	return t.Ctrl
}

func (a *App) tabByID(tabID string) *WorkspaceTab {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.tabByIDLocked(tabID)
}

func (a *App) tabByIDLocked(tabID string) *WorkspaceTab {
	if strings.TrimSpace(tabID) == "" {
		return a.activeTabLocked()
	}
	return a.tabRuntimes[tabID]
}

func (a *App) ctrlByTabID(tabID string) control.SessionAPI {
	a.mu.RLock()
	defer a.mu.RUnlock()
	tab := a.tabByIDLocked(tabID)
	if tab == nil {
		return nil
	}
	return tab.Ctrl
}

// --- autosave per tab -------------------------------------------------------

func (a *App) scheduleTabSnapshot(tabID string) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	tab := a.tabByEventSinkIDLocked(tabID)
	a.scheduleTabSnapshotForTab(tab)
}

func (a *App) scheduleTabSnapshotForTab(tab *WorkspaceTab) {
	if tab == nil {
		return
	}
	tab.saveMu.Lock()
	defer tab.saveMu.Unlock()
	if tab.closing {
		// Tab is being torn down: don't start new snapshot work that could
		// race DeleteSession and resurrect a trashed session file (#4384).
		tab.saveAgain = true
		return
	}
	if tab.saving {
		tab.saveAgain = true
		return
	}
	tab.saving = true
	go a.tabSnapshotLoop(tab)
}

// quiesceTabAutosave marks the tab as closing and blocks until any in-flight
// tabSnapshotLoop has finished its current (and final) write. After it returns,
// no background goroutine can call Snapshot on this tab's controller again, so
// a subsequent DeleteSession cannot race a late write. Call it before clearing
// the controller's session path so an already-claimed artifact writer can
// finish both the transcript snapshot and sidecar merge.
func (a *App) quiesceTabAutosave(tab *WorkspaceTab) {
	_ = a.tryQuiesceTabAutosave(tab)
}

// tryQuiesceTabAutosave returns true only to the caller that changed the tab
// from writable to closing. Temporary runtime hand-offs must own that state
// before they can later reopen it; permanent teardown callers may ignore it.
func (a *App) tryQuiesceTabAutosave(tab *WorkspaceTab) bool {
	if tab == nil {
		return false
	}
	tab.saveMu.Lock()
	if tab.saveCond == nil {
		tab.saveCond = sync.NewCond(&tab.saveMu)
	}
	owned := !tab.closing
	if owned {
		tab.closing = true
	}
	for tab.saving {
		tab.saveCond.Wait()
	}
	tab.saveMu.Unlock()
	return owned
}

func reopenTabAutosave(tab *WorkspaceTab) bool {
	if tab == nil {
		return false
	}
	tab.saveMu.Lock()
	requested := tab.saveAgain
	tab.closing = false
	tab.saveAgain = false
	tab.saveMu.Unlock()
	return requested
}

func consumeClosedTabSnapshotRequest(tab *WorkspaceTab) bool {
	if tab == nil {
		return false
	}
	tab.saveMu.Lock()
	requested := tab.saveAgain
	tab.saveAgain = false
	tab.saveMu.Unlock()
	return requested
}

func (a *App) tabSnapshotLoop(tab *WorkspaceTab) {
	defer a.recoverToPending("tabSnapshotLoop")
	for {
		if err := a.snapshotTabOnce(tab); err == nil {
			a.completeTabSnapshot(tab)
		} else {
			recordDesktopBackgroundFailure("session.snapshot.autosave", err)
		}
		tab.saveMu.Lock()
		if tab.saveCond == nil {
			tab.saveCond = sync.NewCond(&tab.saveMu)
		}
		if tab.closing {
			// Tab is being torn down: stop without picking up saveAgain work.
			tab.saving = false
			tab.saveCond.Broadcast()
			tab.saveMu.Unlock()
			return
		}
		if tab.saveAgain {
			tab.saveAgain = false
			tab.saveMu.Unlock()
			continue
		}
		tab.saving = false
		tab.saveCond.Broadcast()
		tab.saveMu.Unlock()
		return
	}
}

func (a *App) completeTabSnapshot(tab *WorkspaceTab) {
	if !a.maybeAutoTitleTopic(tab) {
		a.emitProjectTreeChanged()
	}
}

func (a *App) maybeAutoTitleTopic(tab *WorkspaceTab) bool {
	if tab == nil || strings.TrimSpace(tab.TopicID) == "" || tab.Ctrl == nil {
		return false
	}
	titleRoot := tab.WorkspaceRoot
	if tab.Scope == "global" {
		titleRoot = ""
	}
	if source := loadTopicTitleSource(titleRoot, tab.TopicID); source != topicTitleSourceAuto {
		return false
	}
	sessionPath := tab.Ctrl.SessionPath()
	if sessionPath == "" {
		return false
	}
	nextTitle, updated := autoTitleTopicFromSessionWithReferenceRoot(titleRoot, tabActorWorkspaceRoot(tab), tab.TopicID, sessionPath)
	if !updated {
		return false
	}
	a.updateOpenTopicTitle(tab.TopicID, nextTitle)
	if err := updateTopicSessionTitleAtPath(sessionPath, tab.TopicID, nextTitle); err != nil {
		slog.Warn("desktop: automatic topic title failed", "topic_id", tab.TopicID, "stage", "turn_done_exact_session_sidecar", "err", err)
	}
	if err := a.updateTopicSessionTitlesInDirsResult(a.sessionDirsForActorProfile(tabRuntimeActorContext(tab)), tab.TopicID, nextTitle); err != nil {
		slog.Warn("desktop: automatic topic title failed", "topic_id", tab.TopicID, "stage", "turn_done_session_sidecar", "err", err)
	}
	a.emitProjectTreeChanged()
	return true
}

type acceptedTopicTitleTarget struct {
	topicID       string
	workspaceRoot string
	referenceRoot string
	sessionPath   string
	actor         userctx.ActorContext
}

func (a *App) acceptedTopicTitleTarget(ctrl control.SessionAPI) (acceptedTopicTitleTarget, bool) {
	if a == nil || ctrl == nil {
		return acceptedTopicTitleTarget{}, false
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, tab := range a.runtimeTabsLocked() {
		if tab == nil || tab.ReadOnly || tab.Ctrl != ctrl {
			continue
		}
		topicID := strings.TrimSpace(tab.TopicID)
		if topicID == "" {
			return acceptedTopicTitleTarget{}, false
		}
		currentTitle := strings.TrimSpace(tab.TopicTitle)
		if currentTitle != "" && currentTitle != defaultTopicTitle {
			return acceptedTopicTitleTarget{}, false
		}
		return acceptedTopicTitleTarget{
			topicID:       topicID,
			workspaceRoot: topicTitleRoot(tab.Scope, tab.WorkspaceRoot),
			referenceRoot: tabActorWorkspaceRoot(tab),
			sessionPath:   tab.currentSessionPath(),
			actor:         tabRuntimeActorContext(tab),
		}, true
	}
	return acceptedTopicTitleTarget{}, false
}

func (a *App) autoTitleAcceptedTopicTurn(target acceptedTopicTitleTarget, raw, display, titleText string, preferTitleText bool) {
	text := raw
	if preferTitleText {
		text = titleText
	} else if strings.TrimSpace(display) != "" {
		text = display
	}
	nextTitle := topicTitleFromTextInWorkspace(text, target.referenceRoot)
	if nextTitle == "" {
		return
	}
	updated, err := trySetAutoTopicTitle(target.workspaceRoot, target.topicID, nextTitle)
	if err != nil {
		slog.Warn("desktop: automatic topic title failed", "topic_id", target.topicID, "stage", "accepted_turn_persist", "err", err)
		return
	}
	if !updated {
		return
	}
	a.updateOpenTopicTitle(target.topicID, nextTitle)
	if err := updateTopicSessionTitleAtPath(target.sessionPath, target.topicID, nextTitle); err != nil {
		slog.Warn("desktop: automatic topic title failed", "topic_id", target.topicID, "stage", "exact_session_sidecar", "err", err)
	}
	if err := a.updateTopicSessionTitlesInDirsResult(a.sessionDirsForActorProfile(target.actor), target.topicID, nextTitle); err != nil {
		slog.Warn("desktop: automatic topic title failed", "topic_id", target.topicID, "stage", "session_sidecar", "err", err)
	}
	a.emitProjectTreeChanged()
}

func autoTitleTopicFromSession(workspaceRoot, topicID, sessionPath string) (string, bool) {
	return autoTitleTopicFromSessionWithReferenceRoot(workspaceRoot, workspaceRoot, topicID, sessionPath)
}

func autoTitleTopicFromSessionWithReferenceRoot(workspaceRoot, referenceRoot, topicID, sessionPath string) (string, bool) {
	pending, err := autoTopicTitlePending(workspaceRoot, topicID)
	if err != nil {
		slog.Warn("desktop: automatic topic title failed", "topic_id", topicID, "stage", "turn_done_check", "err", err)
		return "", false
	}
	if !pending {
		return "", false
	}
	nextTitle := topicTitleFromSessionWithReferenceRoot(sessionPath, referenceRoot)
	if nextTitle == "" {
		return "", false
	}
	updated, err := trySetAutoTopicTitle(workspaceRoot, topicID, nextTitle)
	if err != nil {
		slog.Warn("desktop: automatic topic title failed", "topic_id", topicID, "stage", "turn_done_persist", "err", err)
		return "", false
	}
	if !updated {
		return "", false
	}
	return nextTitle, true
}

func topicTitleFromSession(path string) string {
	return topicTitleFromSessionWithReferenceRoot(path, "")
}

func topicTitleFromSessionWithReferenceRoot(path, workspaceRoot string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	resolveDisplay := sessionDisplayResolver(filepath.Dir(path), path)
	for {
		var msg struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}
		if err := dec.Decode(&msg); err != nil {
			return ""
		}
		if msg.Role == "user" {
			content := resolveDisplay(msg.Content)
			if strings.Contains(msg.Content, "<selected-skill") {
				content = agent.UserPreviewText(msg.Content)
			}
			content = agent.HandoffTask(content)
			if control.IsSyntheticUserMessage(content) {
				continue
			}
			content = control.StripReferencedContextPrefix(content)
			if title := topicTitleFromTextInWorkspace(content, workspaceRoot); title != "" {
				return title
			}
		}
	}
}

const (
	topicTitleInputBytes = 2 * 1024
	topicTitleMaxWidth   = 36
)

func topicTitleFromText(text string) string {
	return topicTitleFromTextInWorkspace(text, "")
}

func topicTitleFromTextInWorkspace(text, workspaceRoot string) string {
	text = boundedUTF8Prefix(text, topicTitleInputBytes)
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	text = stripStandaloneTopicFileRefs(text, workspaceRoot)
	text = strings.Join(strings.Fields(text), " ")
	text = strings.Trim(text, " \t\r\n，。！？；：、,.!?;:\"'`“”‘’()（）[]【】")
	if text == "" {
		return ""
	}
	if runewidth.StringWidth(text) > topicTitleMaxWidth {
		text = truncateTopicTitle(text, topicTitleMaxWidth)
	}
	if text == defaultTopicTitle {
		return ""
	}
	return text
}

func boundedUTF8Prefix(text string, maxBytes int) string {
	if maxBytes <= 0 || len(text) <= maxBytes {
		return text
	}
	text = text[:maxBytes]
	for text != "" && !utf8.ValidString(text) {
		text = text[:len(text)-1]
	}
	return text
}

func stripStandaloneTopicFileRefs(text, workspaceRoot string) string {
	if topicTitleFileRef(strings.Trim(text, " \t\r\n\"'`"), workspaceRoot) {
		return ""
	}
	fields := strings.Fields(text)
	kept := fields[:0]
	for _, field := range fields {
		token := strings.Trim(field, " \t\r\n，。！？；：、,.!?;:\"'`“”‘’()（）[]【】<>")
		if strings.HasPrefix(token, "@") && topicTitleFileRef(strings.TrimPrefix(token, "@"), workspaceRoot) {
			continue
		}
		kept = append(kept, field)
	}
	return strings.Join(kept, " ")
}

func topicTitleFileRef(path, workspaceRoot string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	slashed := filepath.ToSlash(path)
	for _, dir := range []string{
		brand.ProjectDirName,
		brand.LegacyProjectDirName,
		brand.SuiYuanProjectDirName,
		brand.ReasonixProjectDirName,
	} {
		if strings.HasPrefix(slashed, dir+"/attachments/") {
			return true
		}
	}
	candidate := ""
	if filepath.IsAbs(path) {
		candidate = path
	} else if workspaceRoot != "" {
		candidate = filepath.Join(workspaceRoot, filepath.FromSlash(path))
	}
	if candidate != "" {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func truncateTopicTitle(text string, maxWidth int) string {
	if maxWidth <= 1 {
		return "…"
	}
	budget := maxWidth - runewidth.RuneWidth('…')
	runes := make([]rune, 0, len(text))
	width := 0
	lastBoundary := 0
	lastBoundaryWidth := 0
	for _, r := range text {
		runeWidth := runewidth.RuneWidth(r)
		if width+runeWidth > budget {
			break
		}
		runes = append(runes, r)
		width += runeWidth
		if unicode.IsSpace(r) || isTopicTitleBoundary(r) {
			lastBoundary = len(runes)
			lastBoundaryWidth = width
		}
	}
	cut := len(runes)
	if lastBoundary > 0 && lastBoundaryWidth*3 >= budget*2 {
		cut = lastBoundary
	}
	prefix := strings.TrimRightFunc(string(runes[:cut]), func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r)
	})
	if prefix == "" {
		prefix = strings.TrimSpace(string(runes))
	}
	return prefix + "…"
}

func isTopicTitleBoundary(r rune) bool {
	return strings.ContainsRune("，。！？；：、,.!?;:", r)
}

// --- persistence: desktop-projects.json -------------------------------------

const desktopProjectsFile = "desktop-projects.json"

// tabsFileName is kept only for the one-time legacy JSON import. New tab state
// is read from and written to tabsDatabaseName.
const tabsFileName = legacyTabsFileName
const desktopGlobalOrderToken = "__global__"

type desktopProject struct {
	Root         string   `json:"root"`
	Title        string   `json:"title,omitempty"`
	Color        string   `json:"color,omitempty"`
	Topics       []string `json:"topics"` // ordered topic IDs
	PinnedTopics []string `json:"pinnedTopics,omitempty"`
}

type desktopProjectFile struct {
	GlobalTitle        string           `json:"globalTitle,omitempty"`
	GlobalColor        string           `json:"globalColor,omitempty"`
	GlobalTopics       []string         `json:"globalTopics,omitempty"`
	GlobalPinnedTopics []string         `json:"globalPinnedTopics,omitempty"`
	PinnedProjects     []string         `json:"pinnedProjects,omitempty"`
	SidebarOrder       []string         `json:"sidebarOrder,omitempty"`
	Projects           []desktopProject `json:"projects"`
}

type desktopTabEntry struct {
	ID               string  `json:"id"`
	Scope            string  `json:"scope"`
	WorkspaceRoot    string  `json:"workspaceRoot"`
	TopicID          string  `json:"topicId"`
	SessionPath      string  `json:"sessionPath,omitempty"`
	Scene            string  `json:"scene,omitempty"`
	SceneState       string  `json:"sceneState,omitempty"`
	ReadOnly         bool    `json:"readOnly,omitempty"`
	Model            string  `json:"model,omitempty"`
	BillingSource    string  `json:"billingSource,omitempty"`
	Effort           *string `json:"effort,omitempty"`
	TokenMode        string  `json:"tokenMode,omitempty"`
	Mode             string  `json:"mode,omitempty"`
	Goal             string  `json:"goal,omitempty"`
	ToolApprovalMode string  `json:"toolApprovalMode,omitempty"`
}

type desktopTabsFile struct {
	Tabs      []desktopTabEntry `json:"tabs"`
	ActiveTab string            `json:"activeTab"`
	Version   uint64            `json:"-"`
}

func singleSurfaceLayoutStyle(style string) bool {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "workbench", "creation":
		return true
	default:
		return false
	}
}

func singleSurfaceTabsFile(f desktopTabsFile) desktopTabsFile {
	if len(f.Tabs) <= 1 {
		return f
	}
	chosen := f.Tabs[0]
	if active := strings.TrimSpace(f.ActiveTab); active != "" {
		for _, entry := range f.Tabs {
			if entry.ID == active {
				chosen = entry
				break
			}
		}
	}
	return desktopTabsFile{Tabs: []desktopTabEntry{chosen}, ActiveTab: chosen.ID}
}

func desktopConfigDir() string {
	if dir := config.KWUHomeDir(); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, brand.ProjectDirName)
}

func desktopConfigDirForPaths(paths config.ProfilePathResolver) string {
	if path := paths.UserConfigPath(); path != "" {
		return filepath.Dir(path)
	}
	return desktopConfigDir()
}

func (a *App) desktopConfigDir() string {
	return desktopConfigDirForPaths(config.NewProfilePathResolver(a.currentActorContext()))
}

func (a *App) desktopConfigDirLocked() string {
	return desktopConfigDirForPaths(config.NewProfilePathResolver(a.currentActorContextLocked()))
}

func (a *App) saveTabsLocked() {
	dir, entries, activeID, version := a.saveTabsCollectLocked()
	a.saveTabsWrite(dir, entries, activeID, version)
}

// saveTabsCollectLocked gathers the tab-snapshot data under the caller's lock
// (it calls orderedTabIDsLocked which requires a.mu). Returns the config dir,
// the serializable entries, the active tab ID, and a monotonic snapshot version.
// The write can happen outside the lock to avoid blocking the UI with disk I/O.
func (a *App) saveTabsCollectLocked() (string, []desktopTabEntry, string, uint64) {
	dir := a.desktopConfigDirLocked()
	if persisted := loadDesktopTabsFromDir(dir); persisted.Version > a.tabsSaveVersion.Load() {
		a.advanceTabsSaveVersion(persisted.Version)
	}
	a.tabStoreReady.Store(true)
	var entries []desktopTabEntry
	for _, id := range a.orderedTabIDsLocked() {
		if tab := a.tabRuntimes[id]; tab != nil {
			status := tabSceneStatus(tab)
			sceneValue, stateValue, _ := status.Persisted()
			toolApprovalMode := ""
			if tab.toolApprovalModeKnown {
				toolApprovalMode = persistedToolApprovalMode(currentTabToolApprovalMode(tab))
			}
			entries = append(entries, desktopTabEntry{
				ID:               tab.ID,
				Scope:            tab.Scope,
				WorkspaceRoot:    tab.WorkspaceRoot,
				TopicID:          tab.TopicID,
				SessionPath:      tab.currentSessionPath(),
				Scene:            sceneValue,
				SceneState:       stateValue,
				ReadOnly:         tab.ReadOnly,
				Model:            tab.model,
				BillingSource:    tab.billingSource,
				Effort:           cloneStringPtr(tab.effort),
				TokenMode:        persistedTabTokenMode(currentTabTokenMode(tab)),
				Mode:             persistedTabMode(currentTabMode(tab)),
				Goal:             strings.TrimSpace(currentTabGoal(tab)),
				ToolApprovalMode: toolApprovalMode,
			})
		}
	}
	a.publishTabActivationSnapshotLocked()
	version := a.tabsSaveVersion.Add(1)
	return dir, entries, a.cachedActiveTabID(), version
}

// saveTabsWrite commits a complete tab snapshot to SQLite. The database version
// rejects a stale snapshot collected before a newer one.
func (a *App) saveTabsWrite(dir string, entries []desktopTabEntry, activeID string, version uint64) bool {
	written, err := newDesktopTabStore(dir).replace(desktopTabsFile{
		Tabs: entries, ActiveTab: activeID, Version: version,
	})
	if err != nil {
		slog.Warn("save desktop tabs failed", diagnostics.FailureMarkerKey, diagnostics.FailureMarker, "scope", "session.tabs.persist", "path", filepath.Join(dir, tabsDatabaseName), "err", err)
		return false
	}
	return written
}

func (a *App) orderedTabIDsLocked() []string {
	ordered, needsRepair := a.orderedTabIDsSnapshotLocked()
	if needsRepair {
		a.tabOrder = append([]string(nil), ordered...)
	}
	return ordered
}

func (a *App) orderedTabIDsSnapshotLocked() ([]string, bool) {
	seen := make(map[string]bool, len(a.tabRuntimes))
	ordered := make([]string, 0, len(a.tabRuntimes))
	for _, id := range a.tabOrder {
		if _, ok := a.tabRuntimes[id]; ok && !seen[id] {
			ordered = append(ordered, id)
			seen[id] = true
		}
	}
	var missing []string
	for id := range a.tabRuntimes {
		if !seen[id] {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	ordered = append(ordered, missing...)
	return ordered, len(ordered) != len(a.tabOrder) || len(missing) > 0
}

func (a *App) removeTabOrderLocked(tabID string) {
	next := a.tabOrder[:0]
	for _, id := range a.tabOrder {
		if id != tabID {
			next = append(next, id)
		}
	}
	a.tabOrder = next
}

func loadTabsFile() desktopTabsFile {
	return loadTabsFileFromDir(desktopConfigDirForPaths(config.NewProfilePathResolver(userctx.Default(""))))
}

func (a *App) loadTabsFile() desktopTabsFile {
	return loadTabsFileFromDir(a.desktopConfigDir())
}

func loadTabsFileFromDir(dir string) desktopTabsFile {
	return loadDesktopTabsFromDir(dir)
}

func desktopMCPMigrationRoots(tabs desktopTabsFile) []string {
	return desktopMCPMigrationRootsFromState(tabs, loadWorkspace(), loadWorkspaces())
}

func (a *App) desktopMCPMigrationRoots(tabs desktopTabsFile) []string {
	return desktopMCPMigrationRootsFromState(tabs, a.loadWorkspace(), a.loadWorkspaces())
}

func desktopMCPMigrationRootsFromState(tabs desktopTabsFile, currentWorkspace string, recentWorkspaces []string) []string {
	seen := map[string]bool{}
	var roots []string
	add := func(root string) {
		root = normalizeProjectRoot(root)
		if root == "" || seen[root] {
			return
		}
		seen[root] = true
		roots = append(roots, root)
	}
	if currentWorkspace != "" {
		add(currentWorkspace)
	}
	for _, root := range recentWorkspaces {
		add(root)
	}
	for _, entry := range tabs.Tabs {
		if entry.Scope == "project" {
			add(entry.WorkspaceRoot)
		}
	}
	for _, project := range loadProjectsFile().Projects {
		add(project.Root)
	}
	return roots
}

func loadProjectsFile() desktopProjectFile {
	path := filepath.Join(desktopConfigDir(), desktopProjectsFile)
	b, err := os.ReadFile(path)
	if err != nil {
		return desktopProjectFile{}
	}
	var f desktopProjectFile
	_ = json.Unmarshal(b, &f)
	return normalizeProjectsFile(f)
}

var projectsFileMu sync.Mutex

func loadProjectsFileForUpdate() (desktopProjectFile, error) {
	path := filepath.Join(desktopConfigDir(), desktopProjectsFile)
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return desktopProjectFile{}, nil
		}
		return desktopProjectFile{}, fmt.Errorf("read desktop projects: %w", err)
	}
	var f *desktopProjectFile
	if err := json.Unmarshal(b, &f); err != nil {
		return desktopProjectFile{}, fmt.Errorf("decode desktop projects: %w", err)
	}
	if f == nil {
		return desktopProjectFile{}, fmt.Errorf("decode desktop projects: expected a JSON object")
	}
	return normalizeProjectsFile(*f), nil
}

func updateProjectsFile(update func(*desktopProjectFile) (bool, error)) error {
	projectsFileMu.Lock()
	defer projectsFileMu.Unlock()

	f, err := loadProjectsFileForUpdate()
	if err != nil {
		return err
	}
	changed, err := update(&f)
	if err != nil || !changed {
		return err
	}
	return saveProjectsFile(f)
}

func saveProjectsFile(f desktopProjectFile) error {
	dir := desktopConfigDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f = normalizeProjectsFile(f)
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, desktopProjectsFile)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return fileutil.ReplaceFile(tmp, path)
}

func normalizeProjectRoot(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		return ""
	}
	if abs, err := filepath.Abs(root); err == nil {
		return abs
	}
	return root
}

func normalizeProjectsFile(f desktopProjectFile) desktopProjectFile {
	out := desktopProjectFile{
		GlobalTitle:        strings.TrimSpace(f.GlobalTitle),
		GlobalColor:        normalizeProjectColor(f.GlobalColor),
		GlobalTopics:       uniqueStrings(f.GlobalTopics),
		GlobalPinnedTopics: uniqueStrings(f.GlobalPinnedTopics),
	}
	index := map[string]int{}
	for _, p := range f.Projects {
		root := normalizeProjectRoot(p.Root)
		if root == "" {
			continue
		}
		p.Root = root
		p.Title = strings.TrimSpace(p.Title)
		p.Color = normalizeProjectColor(p.Color)
		p.Topics = uniqueStrings(p.Topics)
		p.PinnedTopics = uniqueStrings(p.PinnedTopics)
		if i, ok := index[root]; ok {
			if out.Projects[i].Title == "" && p.Title != "" {
				out.Projects[i].Title = p.Title
			}
			if out.Projects[i].Color == "" && p.Color != "" {
				out.Projects[i].Color = p.Color
			}
			out.Projects[i].Topics = uniqueStrings(append(out.Projects[i].Topics, p.Topics...))
			out.Projects[i].PinnedTopics = uniqueStrings(append(out.Projects[i].PinnedTopics, p.PinnedTopics...))
			continue
		}
		index[root] = len(out.Projects)
		out.Projects = append(out.Projects, p)
	}
	projectRoots := make(map[string]bool, len(out.Projects))
	for _, project := range out.Projects {
		projectRoots[project.Root] = true
	}
	for _, root := range uniqueStrings(f.PinnedProjects) {
		root = normalizeProjectRoot(root)
		if root != "" && projectRoots[root] && !containsDesktopString(out.PinnedProjects, root) {
			out.PinnedProjects = append(out.PinnedProjects, root)
		}
	}
	out.SidebarOrder = normalizeSidebarOrder(f.SidebarOrder, out.Projects)
	return out
}

func normalizeSidebarOrder(order []string, projects []desktopProject) []string {
	projectRoots := make(map[string]bool, len(projects))
	for _, project := range projects {
		if project.Root != "" {
			projectRoots[project.Root] = true
		}
	}
	seen := make(map[string]bool, len(order))
	out := make([]string, 0, len(order))
	for _, value := range order {
		value = strings.TrimSpace(value)
		if value == desktopGlobalOrderToken {
			if !seen[value] {
				seen[value] = true
				out = append(out, value)
			}
			continue
		}
		root := normalizeProjectRoot(value)
		if root == "" || !projectRoots[root] || seen[root] {
			continue
		}
		seen[root] = true
		out = append(out, root)
	}
	return out
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func prependUniqueString(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return uniqueStrings(values)
	}
	return uniqueStrings(append([]string{value}, values...))
}

func removeString(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return uniqueStrings(values)
	}
	out := make([]string, 0, len(values))
	for _, item := range uniqueStrings(values) {
		if item != value {
			out = append(out, item)
		}
	}
	return out
}

func containsDesktopString(values []string, value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	for _, item := range uniqueStrings(values) {
		if item == value {
			return true
		}
	}
	return false
}

func pinnedTopicIDs(topicIDs []string, pinned []string) []string {
	if len(topicIDs) == 0 || len(pinned) == 0 {
		return topicIDs
	}
	available := make(map[string]bool, len(topicIDs))
	for _, tid := range topicIDs {
		available[tid] = true
	}
	out := make([]string, 0, len(topicIDs))
	seen := make(map[string]bool, len(topicIDs))
	for _, tid := range uniqueStrings(pinned) {
		if available[tid] && !seen[tid] {
			out = append(out, tid)
			seen[tid] = true
		}
	}
	for _, tid := range topicIDs {
		if !seen[tid] {
			out = append(out, tid)
		}
	}
	return out
}

func orderedTopicIDs(explicit []string, titleMap map[string]string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(explicit)+len(titleMap))
	for _, tid := range explicit {
		tid = strings.TrimSpace(tid)
		if tid == "" || seen[tid] {
			continue
		}
		seen[tid] = true
		out = append(out, tid)
	}
	var remaining []string
	for tid := range titleMap {
		if !seen[tid] {
			remaining = append(remaining, tid)
		}
	}
	sort.Strings(remaining)
	return append(out, remaining...)
}

func projectTreeOrderKey(node ProjectNode) string {
	switch node.Kind {
	case "global_folder":
		return desktopGlobalOrderToken
	case "project":
		return normalizeProjectRoot(node.Root)
	default:
		return ""
	}
}

func applyProjectTreeOrder(nodes []ProjectNode, order []string) []ProjectNode {
	if len(order) == 0 {
		return nodes
	}
	byKey := make(map[string]ProjectNode, len(nodes))
	for _, node := range nodes {
		key := projectTreeOrderKey(node)
		if key != "" {
			byKey[key] = node
		}
	}
	seen := make(map[string]bool, len(nodes))
	out := make([]ProjectNode, 0, len(nodes))
	for _, value := range order {
		key := strings.TrimSpace(value)
		if key != desktopGlobalOrderToken {
			key = normalizeProjectRoot(key)
		}
		if key == "" || seen[key] {
			continue
		}
		node, ok := byKey[key]
		if !ok {
			continue
		}
		seen[key] = true
		out = append(out, node)
	}
	for _, node := range nodes {
		key := projectTreeOrderKey(node)
		if key != "" && seen[key] {
			continue
		}
		if key != "" {
			seen[key] = true
		}
		out = append(out, node)
	}
	return out
}

func applyPinnedProjectOrder(nodes []ProjectNode, pinnedRoots []string) []ProjectNode {
	pinnedRoots = uniqueStrings(pinnedRoots)
	if len(pinnedRoots) == 0 {
		return nodes
	}
	byRoot := make(map[string]ProjectNode, len(nodes))
	for _, node := range nodes {
		if node.Kind == "project" && node.Root != "" {
			byRoot[normalizeProjectRoot(node.Root)] = node
		}
	}
	seen := make(map[string]bool, len(pinnedRoots))
	out := make([]ProjectNode, 0, len(nodes))
	for _, root := range pinnedRoots {
		root = normalizeProjectRoot(root)
		node, ok := byRoot[root]
		if !ok || seen[root] {
			continue
		}
		seen[root] = true
		out = append(out, node)
	}
	for _, node := range nodes {
		if node.Kind == "project" && node.Root != "" && seen[normalizeProjectRoot(node.Root)] {
			continue
		}
		out = append(out, node)
	}
	return out
}

func projectDisplayName(p desktopProject) string {
	if title := strings.TrimSpace(p.Title); title != "" {
		return title
	}
	return workspaceName(p.Root)
}

func normalizeProjectColor(color string) string {
	switch strings.TrimSpace(strings.ToLower(color)) {
	case "red", "orange", "amber", "green", "teal", "blue", "purple", "pink":
		return strings.TrimSpace(strings.ToLower(color))
	default:
		return ""
	}
}

func projectColor(root string) string {
	root = normalizeProjectRoot(root)
	if root == "" {
		return globalProjectColor()
	}
	for _, p := range loadProjectsFile().Projects {
		if p.Root == root {
			return normalizeProjectColor(p.Color)
		}
	}
	return ""
}

func globalProjectColor() string {
	return normalizeProjectColor(loadProjectsFile().GlobalColor)
}

func globalProjectTitle() string {
	if title := strings.TrimSpace(loadProjectsFile().GlobalTitle); title != "" {
		return title
	}
	return "Global"
}

func addProject(root, title string) error {
	root = normalizeProjectRoot(root)
	if root == "" {
		return fmt.Errorf("project root is required")
	}
	title = strings.TrimSpace(title)
	return updateProjectsFile(func(f *desktopProjectFile) (bool, error) {
		for i, p := range f.Projects {
			if p.Root == root {
				if title != "" {
					f.Projects[i].Title = title
				}
				return true, nil
			}
		}
		f.Projects = append(f.Projects, desktopProject{Root: root, Title: title})
		return true, nil
	})
}

func renameProject(root, title string) error {
	title = strings.TrimSpace(title)
	isGlobal := strings.TrimSpace(root) == ""
	root = normalizeProjectRoot(root)
	return updateProjectsFile(func(f *desktopProjectFile) (bool, error) {
		if isGlobal {
			f.GlobalTitle = title
			return true, nil
		}
		for i, p := range f.Projects {
			if p.Root == root {
				f.Projects[i].Title = title
				return true, nil
			}
		}
		f.Projects = append(f.Projects, desktopProject{Root: root, Title: title})
		return true, nil
	})
}

func setProjectColor(root, color string) error {
	root = normalizeProjectRoot(root)
	color = normalizeProjectColor(color)
	return updateProjectsFile(func(f *desktopProjectFile) (bool, error) {
		if root == "" {
			f.GlobalColor = color
			return true, nil
		}
		for i, p := range f.Projects {
			if p.Root == root {
				f.Projects[i].Color = color
				return true, nil
			}
		}
		f.Projects = append(f.Projects, desktopProject{Root: root, Color: color})
		return true, nil
	})
}

func removeProject(root string) error {
	root = normalizeProjectRoot(root)
	return updateProjectsFile(func(f *desktopProjectFile) (bool, error) {
		projects := make([]desktopProject, 0, len(f.Projects))
		for _, p := range f.Projects {
			if p.Root != root {
				projects = append(projects, p)
			}
		}
		f.Projects = projects
		return true, nil
	})
}

// --- topic helpers ----------------------------------------------------------

const (
	topicTitlesFile        = "desktop-topic-titles.json"
	topicTitleSourcesFile  = "desktop-topic-title-sources.json"
	topicCreatedAtsFile    = "desktop-topic-created-at.json"
	topicModelsFile        = "desktop-topic-models.json"
	defaultTopicTitle      = "新的会话"
	topicTitleSourceAuto   = "auto"
	topicTitleSourceManual = "manual"
)

// topicTitleMu serializes every in-process read-modify-write of the separate
// topic title and source maps. Individual file replacement is atomic, but the
// condition and both maps must share this lock to prevent lost updates and to
// make manual-title precedence reliable.
var topicTitleMu sync.Mutex

// saveTopicTitlesForUpdate is a package-private failure seam used to verify the
// safety of the two-file manual write order.
var saveTopicTitlesForUpdate = saveTopicTitles

var saveTopicTitleSourcesForRegistration = saveTopicTitleSources
var saveTopicCreatedAtsForRegistration = saveTopicCreatedAts
var updateProjectsFileForTopicRegistration = updateProjectsFile

type topicMetadataPathResolver func(workspaceRoot, filename string) (string, error)

func currentTopicMetadataPath(workspaceRoot, filename string) (string, error) {
	return resolveTopicMetadataPath(version, workspaceRoot, filename, ensureDesktopProfileInitialized, desktopConfigDir)
}

func resolveTopicMetadataPath(
	buildVersion string,
	workspaceRoot string,
	filename string,
	initializeProfile func() (desktopProfile, error),
	resolveDesktopConfigDir func() string,
) (string, error) {
	if strings.TrimSpace(buildVersion) != "dev" {
		if workspaceRoot == "" {
			if resolveDesktopConfigDir == nil {
				return "", fmt.Errorf("desktop topic metadata: desktop config directory resolver is unavailable")
			}
			return filepath.Join(resolveDesktopConfigDir(), "global", filename), nil
		}
		return projectStateFilePath(workspaceRoot, filename), nil
	}
	if initializeProfile == nil {
		return "", fmt.Errorf("desktop topic metadata: development profile initializer is unavailable")
	}
	profile, err := initializeProfile()
	if err != nil {
		return "", fmt.Errorf("desktop topic metadata: initialize development profile: %w", err)
	}
	if !profile.Development || !validDevEnvironmentID(profile.EnvironmentID) {
		return "", fmt.Errorf("desktop topic metadata: development environment ID is unavailable or invalid")
	}
	if workspaceRoot == "" {
		if resolveDesktopConfigDir == nil {
			return "", fmt.Errorf("desktop topic metadata: desktop config directory resolver is unavailable")
		}
		root := strings.TrimSpace(resolveDesktopConfigDir())
		if root == "" {
			return "", fmt.Errorf("desktop topic metadata: desktop config directory is unavailable")
		}
		return filepath.Join(root, "global", filename), nil
	}
	return projectEnvironmentStateFilePath(workspaceRoot, profile.EnvironmentID, filename), nil
}

func topicTitlesPath(workspaceRoot string) (string, error) {
	return currentTopicMetadataPath(workspaceRoot, topicTitlesFile)
}

func topicTitleSourcesPath(workspaceRoot string) (string, error) {
	return currentTopicMetadataPath(workspaceRoot, topicTitleSourcesFile)
}

func topicCreatedAtsPath(workspaceRoot string) (string, error) {
	return currentTopicMetadataPath(workspaceRoot, topicCreatedAtsFile)
}

func topicModelsPath(workspaceRoot string) (string, error) {
	return currentTopicMetadataPath(workspaceRoot, topicModelsFile)
}

func projectStateFilePath(workspaceRoot, name string) string {
	for _, dir := range []string{
		brand.ProjectDirName,
		brand.LegacyProjectDirName,
		brand.SuiYuanProjectDirName,
		brand.ReasonixProjectDirName,
	} {
		path := filepath.Join(workspaceRoot, dir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return filepath.Join(workspaceRoot, brand.ProjectDirName, name)
}

func projectEnvironmentStateFilePath(workspaceRoot, environmentID, name string) string {
	for _, dir := range []string{
		brand.ProjectDirName,
		brand.LegacyProjectDirName,
		brand.SuiYuanProjectDirName,
		brand.ReasonixProjectDirName,
	} {
		path := filepath.Join(workspaceRoot, dir, "desktop", "environments", environmentID, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return filepath.Join(workspaceRoot, brand.ProjectDirName, "desktop", "environments", environmentID, name)
}

const topicFileReadTimeout = 200 * time.Millisecond

var readFileWithTimeoutSlots = make(chan struct{}, 16)
var metadataReadFile = os.ReadFile

type metadataReadStatus string

const (
	metadataReadSuccess         metadataReadStatus = "success"
	metadataReadFileNotFound    metadataReadStatus = "file_not_found"
	metadataReadInvalidJSON     metadataReadStatus = "invalid_json"
	metadataReadSlotWaitTimeout metadataReadStatus = "slot_wait_timeout"
	metadataReadTimeout         metadataReadStatus = "read_timeout"
	metadataReadPathError       metadataReadStatus = "path_error"
	metadataReadOtherError      metadataReadStatus = "other_error"
)

func (s metadataReadStatus) shouldRetry() bool {
	return s != metadataReadSuccess && s != metadataReadFileNotFound
}

type timedFileReadResult struct {
	data   []byte
	status metadataReadStatus
}

// readFileWithTimeout gives slot acquisition and the file read one shared
// budget. Callers wait for capacity instead of turning temporary saturation
// into an immediate empty result, while a stuck read still releases its slot
// only after the underlying OS call has actually returned.
func readFileWithTimeout(path string, timeout time.Duration) timedFileReadResult {
	if timeout <= 0 {
		data, err := metadataReadFile(path)
		return classifyMetadataFileRead(data, err)
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	slots := readFileWithTimeoutSlots
	select {
	case slots <- struct{}{}:
	case <-timer.C:
		return timedFileReadResult{status: metadataReadSlotWaitTimeout}
	}
	type result struct {
		data []byte
		err  error
	}
	ch := make(chan result, 1)
	readFile := metadataReadFile
	go func() {
		data, err := readFile(path)
		<-slots
		ch <- result{data: data, err: err}
	}()
	select {
	case r := <-ch:
		return classifyMetadataFileRead(r.data, r.err)
	case <-timer.C:
		return timedFileReadResult{status: metadataReadTimeout}
	}
}

func classifyMetadataFileRead(data []byte, err error) timedFileReadResult {
	if err == nil {
		return timedFileReadResult{data: data, status: metadataReadSuccess}
	}
	if errors.Is(err, os.ErrNotExist) {
		return timedFileReadResult{status: metadataReadFileNotFound}
	}
	return timedFileReadResult{status: metadataReadOtherError}
}

type metadataStringMapResult struct {
	values       map[string]string
	status       metadataReadStatus
	usedSnapshot bool
	path         string
}

type metadataInt64MapResult struct {
	values       map[string]int64
	status       metadataReadStatus
	usedSnapshot bool
	path         string
}

func (r metadataStringMapResult) shouldRetry() bool {
	return r.status.shouldRetry() || (r.status == metadataReadFileNotFound && r.usedSnapshot)
}

func (r metadataInt64MapResult) shouldRetry() bool {
	return r.status.shouldRetry() || (r.status == metadataReadFileNotFound && r.usedSnapshot)
}

type metadataSnapshotStore struct {
	mu         sync.RWMutex
	stringMaps map[string]map[string]string
	int64Maps  map[string]map[string]int64
}

var projectTreeMetadataSnapshots = metadataSnapshotStore{
	stringMaps: map[string]map[string]string{},
	int64Maps:  map[string]map[string]int64{},
}

func metadataSnapshotKey(path string) string {
	path = filepath.Clean(strings.TrimSpace(path))
	if abs, err := filepath.Abs(path); err == nil {
		path = filepath.Clean(abs)
	}
	return path
}

func (s *metadataSnapshotStore) stringMap(path string) (map[string]string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.stringMaps[metadataSnapshotKey(path)]
	return maps.Clone(value), ok
}

func (s *metadataSnapshotStore) int64Map(path string) (map[string]int64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.int64Maps[metadataSnapshotKey(path)]
	return maps.Clone(value), ok
}

func (s *metadataSnapshotStore) putStringMap(path string, value map[string]string) {
	s.mu.Lock()
	s.stringMaps[metadataSnapshotKey(path)] = maps.Clone(value)
	s.mu.Unlock()
}

func (s *metadataSnapshotStore) putInt64Map(path string, value map[string]int64) {
	s.mu.Lock()
	s.int64Maps[metadataSnapshotKey(path)] = maps.Clone(value)
	s.mu.Unlock()
}

func (s *metadataSnapshotStore) reset() {
	s.mu.Lock()
	s.stringMaps = map[string]map[string]string{}
	s.int64Maps = map[string]map[string]int64{}
	s.mu.Unlock()
}

func metadataReadScopeToken(path, workspaceRoot, filename string) string {
	identity := metadataSnapshotKey(path)
	if strings.TrimSpace(path) == "" {
		identity = strings.TrimSpace(workspaceRoot) + "\x00" + filename
	}
	sum := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(sum[:6])
}

func observeMetadataRead(workspaceRoot, filename string, status metadataReadStatus, path string, usedSnapshot bool) {
	attrs := []any{
		"status", string(status),
		"metadata_file", filepath.Base(filename),
		"scope_token", metadataReadScopeToken(path, workspaceRoot, filename),
		"last_known_good", usedSnapshot,
	}
	if status == metadataReadFileNotFound && !usedSnapshot {
		slog.Debug("desktop project-tree metadata read", attrs...)
		return
	}
	if status != metadataReadSuccess {
		slog.Warn("desktop project-tree metadata read failed", attrs...)
	}
}

func readMetadataStringMap(path string) metadataStringMapResult {
	result := metadataStringMapResult{values: map[string]string{}, path: path}
	fileRead := readFileWithTimeout(path, topicFileReadTimeout)
	result.status = fileRead.status
	if fileRead.status == metadataReadSuccess {
		if err := json.Unmarshal(fileRead.data, &result.values); err != nil || result.values == nil {
			result.values = map[string]string{}
			result.status = metadataReadInvalidJSON
		} else {
			projectTreeMetadataSnapshots.putStringMap(path, result.values)
			return result
		}
	}
	if snapshot, ok := projectTreeMetadataSnapshots.stringMap(path); ok {
		result.values = snapshot
		result.usedSnapshot = true
	}
	return result
}

func readMetadataInt64Map(path string) metadataInt64MapResult {
	result := metadataInt64MapResult{values: map[string]int64{}, path: path}
	fileRead := readFileWithTimeout(path, topicFileReadTimeout)
	result.status = fileRead.status
	if fileRead.status == metadataReadSuccess {
		if err := json.Unmarshal(fileRead.data, &result.values); err != nil || result.values == nil {
			result.values = map[string]int64{}
			result.status = metadataReadInvalidJSON
		} else {
			projectTreeMetadataSnapshots.putInt64Map(path, result.values)
			return result
		}
	}
	if snapshot, ok := projectTreeMetadataSnapshots.int64Map(path); ok {
		result.values = snapshot
		result.usedSnapshot = true
	}
	return result
}

func loadTopicTitles(workspaceRoot string) map[string]string {
	return loadTopicTitlesResult(workspaceRoot).values
}

func loadTopicTitlesResult(workspaceRoot string) metadataStringMapResult {
	return loadTopicStringMap(workspaceRoot, topicTitlesFile, currentTopicMetadataPath)
}

func loadTopicTitleSources(workspaceRoot string) map[string]string {
	return loadTopicStringMap(workspaceRoot, topicTitleSourcesFile, currentTopicMetadataPath).values
}

func loadTopicCreatedAts(workspaceRoot string) map[string]int64 {
	return loadTopicCreatedAtsResult(workspaceRoot).values
}

func loadTopicCreatedAtsResult(workspaceRoot string) metadataInt64MapResult {
	return loadTopicInt64Map(workspaceRoot, topicCreatedAtsFile, currentTopicMetadataPath)
}

func loadTopicModels(workspaceRoot string) map[string]string {
	return loadTopicStringMap(workspaceRoot, topicModelsFile, currentTopicMetadataPath).values
}

func loadTopicStringMap(workspaceRoot, filename string, resolvePath topicMetadataPathResolver) metadataStringMapResult {
	path, err := resolvePath(workspaceRoot, filename)
	if err != nil {
		result := metadataStringMapResult{values: map[string]string{}, status: metadataReadPathError}
		observeMetadataRead(workspaceRoot, filename, result.status, "", false)
		return result
	}
	result := readMetadataStringMap(path)
	observeMetadataRead(workspaceRoot, filename, result.status, path, result.usedSnapshot)
	return result
}

func loadTopicInt64Map(workspaceRoot, filename string, resolvePath topicMetadataPathResolver) metadataInt64MapResult {
	path, err := resolvePath(workspaceRoot, filename)
	if err != nil {
		result := metadataInt64MapResult{values: map[string]int64{}, status: metadataReadPathError}
		observeMetadataRead(workspaceRoot, filename, result.status, "", false)
		return result
	}
	result := readMetadataInt64Map(path)
	observeMetadataRead(workspaceRoot, filename, result.status, path, result.usedSnapshot)
	return result
}

func loadStringMapForUpdate(path string) (map[string]string, error) {
	m := map[string]string{}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return m, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(b, &m); err != nil || m == nil {
		return map[string]string{}, nil
	}
	return m, nil
}

func loadInt64MapForUpdate(path string) (map[string]int64, error) {
	m := map[string]int64{}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return m, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(b, &m); err != nil || m == nil {
		return map[string]int64{}, nil
	}
	return m, nil
}

func loadStringMapForTopicRegistration(path string) (map[string]string, error) {
	m := map[string]string{}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return m, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, fmt.Errorf("topic metadata must be a JSON object")
	}
	return m, nil
}

func loadInt64MapForTopicRegistration(path string) (map[string]int64, error) {
	m := map[string]int64{}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return m, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, fmt.Errorf("topic metadata must be a JSON object")
	}
	return m, nil
}

func loadTopicRegistrationStringMap(workspaceRoot, filename string) (map[string]string, error) {
	path, err := currentTopicMetadataPath(workspaceRoot, filename)
	if err != nil {
		return nil, err
	}
	return loadStringMapForTopicRegistration(path)
}

func loadTopicRegistrationCreatedAts(workspaceRoot string) (map[string]int64, error) {
	path, err := currentTopicMetadataPath(workspaceRoot, topicCreatedAtsFile)
	if err != nil {
		return nil, err
	}
	return loadInt64MapForTopicRegistration(path)
}

func loadTopicTitlesForUpdate(workspaceRoot string) (map[string]string, error) {
	return loadTopicStringMapForUpdate(workspaceRoot, topicTitlesFile, currentTopicMetadataPath)
}

func loadTopicTitleSourcesForUpdate(workspaceRoot string) (map[string]string, error) {
	return loadTopicStringMapForUpdate(workspaceRoot, topicTitleSourcesFile, currentTopicMetadataPath)
}

func loadTopicCreatedAtsForUpdate(workspaceRoot string) (map[string]int64, error) {
	return loadTopicInt64MapForUpdate(workspaceRoot, topicCreatedAtsFile, currentTopicMetadataPath)
}

func loadTopicModelsForUpdate(workspaceRoot string) (map[string]string, error) {
	return loadTopicStringMapForUpdate(workspaceRoot, topicModelsFile, currentTopicMetadataPath)
}

func loadTopicStringMapForUpdate(workspaceRoot, filename string, resolvePath topicMetadataPathResolver) (map[string]string, error) {
	path, err := resolvePath(workspaceRoot, filename)
	if err != nil {
		return nil, err
	}
	return loadStringMapForUpdate(path)
}

func loadTopicInt64MapForUpdate(workspaceRoot, filename string, resolvePath topicMetadataPathResolver) (map[string]int64, error) {
	path, err := resolvePath(workspaceRoot, filename)
	if err != nil {
		return nil, err
	}
	return loadInt64MapForUpdate(path)
}

func saveTopicTitles(workspaceRoot string, m map[string]string) error {
	return saveTopicMetadataMap(workspaceRoot, topicTitlesFile, m, currentTopicMetadataPath)
}

func saveTopicTitleSources(workspaceRoot string, m map[string]string) error {
	return saveTopicMetadataMap(workspaceRoot, topicTitleSourcesFile, m, currentTopicMetadataPath)
}

func saveTopicCreatedAts(workspaceRoot string, m map[string]int64) error {
	return saveTopicMetadataMap(workspaceRoot, topicCreatedAtsFile, m, currentTopicMetadataPath)
}

func saveTopicModels(workspaceRoot string, m map[string]string) error {
	return saveTopicMetadataMap(workspaceRoot, topicModelsFile, m, currentTopicMetadataPath)
}

func saveTopicMetadataMap(workspaceRoot, filename string, value any, resolvePath topicMetadataPathResolver) error {
	path, err := resolvePath(workspaceRoot, filename)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	if err := fileutil.ReplaceFile(tmp, path); err != nil {
		return err
	}
	switch typed := value.(type) {
	case map[string]string:
		projectTreeMetadataSnapshots.putStringMap(path, typed)
	case map[string]int64:
		projectTreeMetadataSnapshots.putInt64Map(path, typed)
	}
	return nil
}

func loadTopicTitle(workspaceRoot, topicID string) string {
	return loadTopicTitles(workspaceRoot)[topicID]
}

func loadTopicTitleSource(workspaceRoot, topicID string) string {
	return loadTopicTitleSources(workspaceRoot)[topicID]
}

func loadTopicCreatedAt(workspaceRoot, topicID string) int64 {
	return loadTopicCreatedAts(workspaceRoot)[topicID]
}

func loadTopicModel(workspaceRoot, topicID string) string {
	return loadTopicModels(workspaceRoot)[topicID]
}

func topicTitleForTab(scope, workspaceRoot, topicID string) string {
	titleRoot := topicTitleRoot(scope, workspaceRoot)
	if title := strings.TrimSpace(loadTopicTitle(titleRoot, topicID)); title != "" {
		return title
	}
	if scope == "global" {
		return "Global"
	}
	return defaultTopicTitle
}

func topicTitleRoot(scope, workspaceRoot string) string {
	if scope == "global" {
		return ""
	}
	return workspaceRoot
}

func forkTopicTitle(title string) string {
	base := strings.TrimSpace(title)
	if base == "" || base == defaultTopicTitle || base == "Global" {
		return "分叉会话"
	}
	if strings.HasSuffix(base, " · 分叉") {
		return base
	}
	return base + " · 分叉"
}

func setTopicTitle(workspaceRoot, topicID, title string) error {
	return setTopicTitleWithSource(workspaceRoot, topicID, title, topicTitleSourceManual)
}

func setTopicTitleWithSource(workspaceRoot, topicID, title, source string) error {
	topicTitleMu.Lock()
	defer topicTitleMu.Unlock()
	return setTopicTitleWithSourceLocked(workspaceRoot, topicID, title, source)
}

func setTopicTitleWithSourceLocked(workspaceRoot, topicID, title, source string) error {
	topicID = strings.TrimSpace(topicID)
	title = strings.TrimSpace(title)
	source = strings.TrimSpace(source)

	// Persist manual ownership first. If the subsequent title write fails, the
	// old title remains visible but asynchronous auto-title can no longer replace
	// it. Deletion likewise removes ownership first so no stale auto source is
	// left behind.
	if source == topicTitleSourceManual || title == "" {
		if err := writeTopicTitleSource(workspaceRoot, topicID, title, source); err != nil {
			return fmt.Errorf("save topic title source: %w", err)
		}
		if err := writeTopicTitle(workspaceRoot, topicID, title); err != nil {
			return fmt.Errorf("save topic title: %w", err)
		}
		return nil
	}

	if err := writeTopicTitle(workspaceRoot, topicID, title); err != nil {
		return fmt.Errorf("save topic title: %w", err)
	}
	if err := writeTopicTitleSource(workspaceRoot, topicID, title, source); err != nil {
		return fmt.Errorf("save topic title source: %w", err)
	}
	return nil
}

func writeTopicTitle(workspaceRoot, topicID, title string) error {
	m, err := loadTopicTitlesForUpdate(workspaceRoot)
	if err != nil {
		return err
	}
	if strings.TrimSpace(title) == "" {
		delete(m, topicID)
	} else {
		m[topicID] = title
	}
	return saveTopicTitlesForUpdate(workspaceRoot, m)
}

func writeTopicTitleSource(workspaceRoot, topicID, title, source string) error {
	sources, err := loadTopicTitleSourcesForUpdate(workspaceRoot)
	if err != nil {
		return err
	}
	if title == "" || source == "" {
		delete(sources, topicID)
	} else {
		sources[topicID] = source
	}
	return saveTopicTitleSources(workspaceRoot, sources)
}

func trySetAutoTopicTitle(workspaceRoot, topicID, title string) (bool, error) {
	topicID = strings.TrimSpace(topicID)
	title = strings.TrimSpace(title)
	if topicID == "" || title == "" || title == defaultTopicTitle {
		return false, nil
	}

	topicTitleMu.Lock()
	defer topicTitleMu.Unlock()
	sources, err := loadTopicTitleSourcesForUpdate(workspaceRoot)
	if err != nil {
		return false, fmt.Errorf("load topic title source: %w", err)
	}
	if sources[topicID] != topicTitleSourceAuto {
		return false, nil
	}
	titles, err := loadTopicTitlesForUpdate(workspaceRoot)
	if err != nil {
		return false, fmt.Errorf("load topic title: %w", err)
	}
	current := strings.TrimSpace(titles[topicID])
	if current != "" && current != defaultTopicTitle {
		return false, nil
	}
	titles[topicID] = title
	if err := saveTopicTitlesForUpdate(workspaceRoot, titles); err != nil {
		return false, fmt.Errorf("save topic title: %w", err)
	}
	return true, nil
}

func autoTopicTitlePending(workspaceRoot, topicID string) (bool, error) {
	topicTitleMu.Lock()
	defer topicTitleMu.Unlock()
	sources, err := loadTopicTitleSourcesForUpdate(workspaceRoot)
	if err != nil {
		return false, fmt.Errorf("load topic title source: %w", err)
	}
	if sources[topicID] != topicTitleSourceAuto {
		return false, nil
	}
	titles, err := loadTopicTitlesForUpdate(workspaceRoot)
	if err != nil {
		return false, fmt.Errorf("load topic title: %w", err)
	}
	current := strings.TrimSpace(titles[topicID])
	return current == "" || current == defaultTopicTitle, nil
}

func resetAutoTopicTitle(workspaceRoot, topicID string) (bool, error) {
	topicTitleMu.Lock()
	defer topicTitleMu.Unlock()
	sources, err := loadTopicTitleSourcesForUpdate(workspaceRoot)
	if err != nil {
		return false, fmt.Errorf("load topic title source: %w", err)
	}
	if sources[topicID] != topicTitleSourceAuto {
		return false, nil
	}
	if err := writeTopicTitle(workspaceRoot, topicID, defaultTopicTitle); err != nil {
		return false, fmt.Errorf("reset automatic topic title: %w", err)
	}
	return true, nil
}

func setTopicCreatedAt(workspaceRoot, topicID string, createdAt int64) error {
	created, err := loadTopicCreatedAtsForUpdate(workspaceRoot)
	if err != nil {
		return err
	}
	topicID = strings.TrimSpace(topicID)
	if topicID == "" || createdAt <= 0 {
		delete(created, topicID)
	} else {
		created[topicID] = createdAt
	}
	return saveTopicCreatedAts(workspaceRoot, created)
}

func setTopicModel(workspaceRoot, topicID, model string) error {
	models, err := loadTopicModelsForUpdate(workspaceRoot)
	if err != nil {
		return err
	}
	topicID = strings.TrimSpace(topicID)
	model = strings.TrimSpace(model)
	if topicID == "" || model == "" {
		delete(models, topicID)
	} else {
		models[topicID] = model
	}
	return saveTopicModels(workspaceRoot, models)
}

func deleteTopicCreatedAt(workspaceRoot, topicID string) {
	created, err := loadTopicCreatedAtsForUpdate(workspaceRoot)
	if err != nil {
		recordDesktopBackgroundFailure("topic.created_at.delete.load", err)
		return
	}
	delete(created, topicID)
	if err := saveTopicCreatedAts(workspaceRoot, created); err != nil {
		recordDesktopBackgroundFailure("topic.created_at.delete.save", err)
	}
}

func deleteTopicModel(workspaceRoot, topicID string) {
	models, err := loadTopicModelsForUpdate(workspaceRoot)
	if err != nil {
		recordDesktopBackgroundFailure("topic.model.delete.load", err)
		return
	}
	delete(models, topicID)
	if err := saveTopicModels(workspaceRoot, models); err != nil {
		recordDesktopBackgroundFailure("topic.model.delete.save", err)
	}
}

// topicIndexMu serializes complete topic registrations and recovery writes. It
// must always be acquired before projectsFileMu.
var topicIndexMu sync.Mutex

func registerTopic(scope, workspaceRoot, topicID, title, source string, createdAt int64) (string, string, error) {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return "", "", fmt.Errorf("topicID is required")
	}
	if strings.TrimSpace(scope) == "project" {
		workspaceRoot = normalizeProjectRoot(workspaceRoot)
		if workspaceRoot == "" {
			return "", "", fmt.Errorf("workspaceRoot is required")
		}
		scope = "project"
	} else {
		scope = "global"
		workspaceRoot = ""
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = defaultTopicTitle
	}
	source = strings.TrimSpace(source)
	if source == "" {
		source = topicTitleSourceManual
	}
	if source != topicTitleSourceAuto && source != topicTitleSourceManual {
		return "", "", fmt.Errorf("invalid topic title source %q", source)
	}
	if createdAt <= 0 {
		return "", "", fmt.Errorf("createdAt is required")
	}

	topicIndexMu.Lock()
	defer topicIndexMu.Unlock()

	topicTitleMu.Lock()
	titles, err := loadTopicRegistrationStringMap(workspaceRoot, topicTitlesFile)
	if err != nil {
		topicTitleMu.Unlock()
		return "", "", fmt.Errorf("load topic titles for registration: %w", err)
	}
	sources, err := loadTopicRegistrationStringMap(workspaceRoot, topicTitleSourcesFile)
	if err != nil {
		topicTitleMu.Unlock()
		return "", "", fmt.Errorf("load topic title sources for registration: %w", err)
	}
	createdAts, err := loadTopicRegistrationCreatedAts(workspaceRoot)
	if err != nil {
		topicTitleMu.Unlock()
		return "", "", fmt.Errorf("load topic creation times for registration: %w", err)
	}
	titles[topicID] = title
	sources[topicID] = source
	createdAts[topicID] = createdAt
	if source == topicTitleSourceManual {
		err = saveTopicTitleSourcesForRegistration(workspaceRoot, sources)
		if err == nil {
			err = saveTopicTitlesForUpdate(workspaceRoot, titles)
		}
	} else {
		err = saveTopicTitlesForUpdate(workspaceRoot, titles)
		if err == nil {
			err = saveTopicTitleSourcesForRegistration(workspaceRoot, sources)
		}
	}
	if err == nil {
		err = saveTopicCreatedAtsForRegistration(workspaceRoot, createdAts)
	}
	topicTitleMu.Unlock()
	if err != nil {
		return "", "", fmt.Errorf("save topic registration metadata: %w", err)
	}

	err = updateProjectsFileForTopicRegistration(func(f *desktopProjectFile) (bool, error) {
		if scope == "global" {
			f.GlobalTopics = prependUniqueString(f.GlobalTopics, topicID)
			return true, nil
		}
		for i, project := range f.Projects {
			if project.Root == workspaceRoot {
				f.Projects[i].Topics = prependUniqueString(project.Topics, topicID)
				return true, nil
			}
		}
		f.Projects = append(f.Projects, desktopProject{Root: workspaceRoot, Topics: []string{topicID}})
		return true, nil
	})
	if err != nil {
		return "", "", fmt.Errorf("save topic project index: %w", err)
	}
	return scope, workspaceRoot, nil
}

func ensureTopicIndexed(scope, workspaceRoot, topicID, title, source string) error {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return fmt.Errorf("topicID is required")
	}
	topicIndexMu.Lock()
	defer topicIndexMu.Unlock()
	if strings.TrimSpace(scope) == "global" {
		workspaceRoot = ""
	} else {
		workspaceRoot = normalizeProjectRoot(workspaceRoot)
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = defaultTopicTitle
	}
	source = strings.TrimSpace(source)
	if source == "" {
		source = topicTitleSourceManual
	}
	if err := setTopicTitleWithSource(workspaceRoot, topicID, title, source); err != nil {
		return err
	}
	ensureTopicOrder := func(values []string) []string {
		values = uniqueStrings(values)
		if containsDesktopString(values, topicID) {
			return values
		}
		return prependUniqueString(values, topicID)
	}
	return updateProjectsFile(func(f *desktopProjectFile) (bool, error) {
		if workspaceRoot == "" {
			f.GlobalTopics = ensureTopicOrder(f.GlobalTopics)
			return true, nil
		}
		for i, p := range f.Projects {
			if p.Root == workspaceRoot {
				f.Projects[i].Topics = ensureTopicOrder(p.Topics)
				return true, nil
			}
		}
		f.Projects = append(f.Projects, desktopProject{Root: workspaceRoot, Topics: []string{topicID}})
		return true, nil
	})
}

// --- telemetry --------------------------------------------------------------

func saveTelemetry(path string, snapshot tabTelemetrySnapshot) error {
	if snapshot.Version == 0 {
		snapshot.Version = 2
	}
	if snapshot.ReadFiles == nil {
		snapshot.ReadFiles = []readFileRecord{}
	}
	b, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return fileutil.ReplaceFile(tmp, path)
}

func loadTelemetry(path string) tabTelemetrySnapshot {
	b, err := os.ReadFile(path)
	if err != nil {
		return tabTelemetrySnapshot{Version: 2, ReadFiles: []readFileRecord{}}
	}
	var snapshot tabTelemetrySnapshot
	if err := json.Unmarshal(b, &snapshot); err == nil && (snapshot.Version > 0 || snapshot.ReadFiles != nil) {
		if snapshot.ReadFiles == nil {
			snapshot.ReadFiles = []readFileRecord{}
		}
		if snapshot.Usage.SessionCost == 0 && snapshot.Usage.SessionCostUsd > 0 {
			snapshot.Usage.SessionCost = snapshot.Usage.SessionCostUsd
		}
		return snapshot
	}
	var records []readFileRecord
	if err := json.Unmarshal(b, &records); err != nil || records == nil {
		records = []readFileRecord{}
	}
	return tabTelemetrySnapshot{Version: 1, ReadFiles: records}
}

// --- project tree -----------------------------------------------------------

// ProjectNode is one node in the sidebar project tree (a project folder or a
// topic leaf).
type ProjectNode struct {
	Key               string               `json:"key"`  // stable key for React
	Kind              string               `json:"kind"` // "project" | "cloud_project" | "topic" | "session" | "global_folder" | "global_topic" | "global_session"
	Label             string               `json:"label"`
	Root              string               `json:"root,omitempty"` // project workspace root
	CloudProjectID    string               `json:"cloudProjectId,omitempty"`
	TopicID           string               `json:"topicId,omitempty"`
	SessionPath       string               `json:"sessionPath,omitempty"`
	SessionTurns      *int                 `json:"sessionTurns,omitempty"`
	Scene             string               `json:"scene,omitempty"`
	SceneState        string               `json:"sceneState,omitempty"`
	ModelRef          string               `json:"modelRef,omitempty"`
	BillingSource     string               `json:"billingSource,omitempty"`
	Effort            *string              `json:"effort,omitempty"`
	ExpertID          string               `json:"expertId,omitempty"`
	ExpertName        string               `json:"expertName,omitempty"`
	CollaborationMode string               `json:"collaborationMode,omitempty"`
	ToolApprovalMode  string               `json:"toolApprovalMode,omitempty"`
	TokenMode         string               `json:"tokenMode,omitempty"`
	Goal              string               `json:"goal,omitempty"`
	GoalStatus        string               `json:"goalStatus,omitempty"`
	ProjectColor      string               `json:"projectColor,omitempty"`
	Turns             int                  `json:"turns,omitempty"`
	CreatedAt         int64                `json:"createdAt,omitempty"`
	LastActivityAt    int64                `json:"lastActivityAt,omitempty"`
	Open              bool                 `json:"open,omitempty"`
	Running           bool                 `json:"running,omitempty"`
	Status            string               `json:"status,omitempty"`
	Pinned            bool                 `json:"pinned,omitempty"`
	Capability        *CapabilityTaskScope `json:"capability,omitempty"`
	Children          []ProjectNode        `json:"children,omitempty"`
}

func normalizeTopicStatus(status string) string {
	switch status {
	case topicStatusThinking, topicStatusStreaming, topicStatusWaitingConfirmation, topicStatusBackgroundJob, topicStatusPaused, topicStatusError:
		return status
	default:
		return ""
	}
}

func activityStatusForTab(tab *WorkspaceTab) string {
	if tab == nil {
		return ""
	}
	status := normalizeTopicStatus(tab.ActivityStatus)
	if tab.Ctrl == nil {
		return status
	}
	runtimeStatus := tab.Ctrl.RuntimeStatus()
	if runtimeStatus.PendingPrompt {
		return topicStatusWaitingConfirmation
	}
	if runtimeStatus.Running {
		if status == "" || status == topicStatusError {
			return topicStatusThinking
		}
		return status
	}
	if runtimeStatus.BackgroundJobs > 0 {
		return topicStatusBackgroundJob
	}
	if status == topicStatusError || status == topicStatusPaused {
		return status
	}
	return ""
}

// migrateLegacySessionsIntoGlobalTopics makes pre-topic desktop history visible
// in the v2 sidebar. Imported v0.x sessions and older desktop sessions are plain
// .jsonl files, sometimes with branch metadata but no topic metadata; the
// history panel can list them, but the project tree cannot. Give each such
// session a deterministic Global topic so every old conversation has a direct
// sidebar entry without guessing a project workspace.
// legacyMigrationMu serializes the lockless load-modify-save of the projects /
// topic-title files: this migration runs from every concurrent buildTabController
// and from ListProjectTree, so without it parallel runs lose each other's appends.
var legacyMigrationMu sync.Mutex

// topicMigrationMarker, once written into a session dir, records that the
// pre-topic → Global-topic migration pass completed for that dir. Later
// ListProjectTree calls can skip the full session decode while the marker is
// newer than the directory's session files, but a newly-created CLI session
// invalidates the marker and gets a bounded re-scan.
// It is stamped only when the pass left nothing deferred (an empty legacy
// session that could gain content later keeps the dir unmarked), so the gate
// never hides a session that should still be migrated.
const topicMigrationMarker = ".topics-migrated"

func topicMigrationDone(dir string) bool {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return false
	}
	markerInfo, err := os.Stat(filepath.Join(dir, topicMigrationMarker))
	if err != nil {
		return false
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return true
	}
	markerTime := markerInfo.ModTime()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".jsonl" && !strings.HasSuffix(name, ".jsonl.meta") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return false
		}
		if info.ModTime().After(markerTime) {
			return false
		}
	}
	return true
}

func markTopicMigrationDone(dir string) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, topicMigrationMarker), nil, 0o644)
}

func migrateLegacySessionsIntoGlobalTopics(dir string) []string {
	if strings.TrimSpace(dir) == "" {
		return nil
	}
	// One-shot per dir: once the migration pass has completed, skip the full
	// per-render session scan entirely.
	if topicMigrationDone(dir) {
		return nil
	}
	scope, workspaceRoot, topicTitleRoot, ok := legacyMigrationTargetForDir(dir)
	if !ok {
		return nil
	}
	legacyMigrationMu.Lock()
	defer legacyMigrationMu.Unlock()
	topicTitleMu.Lock()
	defer topicTitleMu.Unlock()
	// Re-check under the lock: another render may have completed the pass while
	// this one waited.
	if topicMigrationDone(dir) {
		return nil
	}
	infos, err := agent.ListSessionOrder(dir)
	if err != nil {
		return nil // transient read error — retry on the next render, leave unmarked
	}

	var migratedTopicIDs []string
	var titles map[string]string
	var topicTitles map[string]string
	var topicSources map[string]string
	// deferred stays false only when every session was either migrated or is
	// permanently non-migratable. A transient skip (unreadable meta, empty
	// session that may gain content, failed write) sets it, keeping the dir
	// unmarked so the next render retries instead of the gate hiding it forever.
	deferred := false
	for _, info := range infos {
		if strings.TrimSpace(info.TopicID) != "" {
			continue
		}
		if meta, ok, err := agent.LoadBranchMeta(info.Path); err != nil {
			deferred = true
			continue
		} else if ok && !legacySessionMetaMatchesMigrationTarget(meta, scope, workspaceRoot) {
			continue
		}
		topicID := legacySessionTopicID(info.Path)
		if topicID == "" {
			continue
		}
		preview, turns := agent.SessionPreview(info.Path)
		if turns == 0 {
			deferred = true // empty now, but a later turn could make it migratable
			continue
		}
		if titles == nil {
			titles = loadSessionTitles(dir)
		}
		title := strings.TrimSpace(titles[filepath.Base(info.Path)])
		if title == "" {
			title = topicTitleFromSession(info.Path)
			if title == "" {
				title = topicTitleFromText(preview)
			}
		} else if normalized := topicTitleFromText(title); normalized != "" {
			title = normalized
		}
		if title == "" {
			when := info.LastActivityAt
			if when.IsZero() {
				when = info.ModTime
			}
			if when.IsZero() {
				title = "历史会话"
			} else {
				title = "历史会话 " + when.Local().Format("2006-01-02")
			}
		}

		migrated := false
		err := agent.MutateBranchMeta(info.Path, false, func(meta *agent.BranchMeta) error {
			// Preserve scoped sessions only when their current ownership matches
			// the directory being migrated.
			if !legacySessionMetaMatchesMigrationTarget(*meta, scope, workspaceRoot) {
				return errLegacySessionMetaOwnershipChanged
			}
			meta.Scope = scope
			meta.WorkspaceRoot = workspaceRoot
			meta.TopicID = topicID
			meta.TopicTitle = title
			migrated = true
			return nil
		})
		if errors.Is(err, errLegacySessionMetaOwnershipChanged) {
			continue
		}
		if err != nil {
			deferred = true
			continue
		}
		if !migrated {
			continue
		}
		if topicTitles == nil {
			topicTitles = loadTopicTitles(topicTitleRoot)
		}
		if topicSources == nil {
			topicSources = loadTopicTitleSources(topicTitleRoot)
		}
		if strings.TrimSpace(topicTitles[topicID]) == "" {
			topicTitles[topicID] = title
			topicSources[topicID] = topicTitleSourceManual
		}
		migratedTopicIDs = append(migratedTopicIDs, topicID)
	}
	if len(migratedTopicIDs) == 0 {
		if !deferred {
			markTopicMigrationDone(dir) // nothing left to migrate — gate future scans
		}
		return nil
	}
	if err := updateProjectsFile(func(f *desktopProjectFile) (bool, error) {
		if scope == "global" {
			f.GlobalTopics = uniqueStrings(append(migratedTopicIDs, f.GlobalTopics...))
			return true, nil
		}
		// Find the project entry and add topics.
		for i, p := range f.Projects {
			if p.Root == workspaceRoot {
				f.Projects[i].Topics = uniqueStrings(append(migratedTopicIDs, f.Projects[i].Topics...))
				return true, nil
			}
		}
		return false, nil
	}); err != nil {
		return nil
	}
	if topicTitles != nil {
		if err := saveTopicTitles(topicTitleRoot, topicTitles); err != nil {
			recordDesktopBackgroundFailure("topic.migration.titles.save", err)
		}
	}
	if topicSources != nil {
		if err := saveTopicTitleSources(topicTitleRoot, topicSources); err != nil {
			recordDesktopBackgroundFailure("topic.migration.sources.save", err)
		}
	}
	invalidateTopicSessionIndex(dir)
	projectSessionCache.invalidate()
	if !deferred {
		markTopicMigrationDone(dir) // pass complete with nothing deferred
	}
	return migratedTopicIDs
}

var errLegacySessionMetaOwnershipChanged = errors.New("legacy session metadata ownership changed")

func legacyMigrationTargetForDir(dir string) (scope, workspaceRoot, topicTitleRoot string, ok bool) {
	dir = cleanDesktopPath(dir)
	if dir == "" {
		return "", "", "", false
	}
	if sameDesktopPath(dir, config.SessionDir()) || sameDesktopPath(dir, desktopSessionDir(globalWorkspaceRoot())) {
		return "global", "", "", true
	}
	for _, p := range loadProjectsFile().Projects {
		if sameDesktopPath(config.ProjectSessionDir(p.Root), dir) {
			return "project", p.Root, p.Root, true
		}
	}
	return "", "", "", false
}

func legacySessionMetaMatchesMigrationTarget(meta agent.BranchMeta, scope, workspaceRoot string) bool {
	if strings.TrimSpace(meta.TopicID) != "" {
		return false
	}
	metaScope := strings.TrimSpace(meta.Scope)
	if metaScope != "" && metaScope != scope {
		return false
	}
	metaRoot := normalizeProjectRoot(meta.WorkspaceRoot)
	if scope == "project" {
		return metaRoot == "" || normalizeProjectRoot(workspaceRoot) == metaRoot
	}
	return metaRoot == "" || normalizeProjectRoot(globalWorkspaceRoot()) == metaRoot
}

func cleanDesktopPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return filepath.Clean(path)
}

func sameDesktopPath(a, b string) bool {
	a = cleanDesktopPath(a)
	b = cleanDesktopPath(b)
	if a == "" || b == "" {
		return false
	}
	if os.PathSeparator == '\\' {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func restoreSessionTopicIndex(dir, sessionPath string) error {
	sessionPath = strings.TrimSpace(sessionPath)
	if sessionPath == "" {
		return nil
	}
	meta, ok, err := agent.LoadBranchMeta(sessionPath)
	if err != nil {
		return err
	}
	if !ok || strings.TrimSpace(meta.TopicID) == "" {
		migrateLegacySessionsIntoGlobalTopics(dir)
		return nil
	}

	topicID := strings.TrimSpace(meta.TopicID)
	scope := strings.TrimSpace(meta.Scope)
	workspaceRoot := strings.TrimSpace(meta.WorkspaceRoot)
	if scope != "global" && scope != "project" {
		if workspaceRoot == "" {
			scope = "global"
		} else {
			scope = "project"
		}
	}
	if scope == "global" {
		workspaceRoot = ""
	} else {
		workspaceRoot = normalizeProjectRoot(workspaceRoot)
		if workspaceRoot == "" {
			scope = "global"
		}
	}

	title := restoredSessionTopicTitle(dir, sessionPath, meta)
	if title == "" {
		title = defaultTopicTitle
	}
	if err := setTopicTitleWithSource(workspaceRoot, topicID, title, topicTitleSourceManual); err != nil {
		return err
	}

	if err := updateProjectsFile(func(f *desktopProjectFile) (bool, error) {
		if scope == "global" {
			f.GlobalTopics = prependUniqueString(f.GlobalTopics, topicID)
			return true, nil
		}
		for i, p := range f.Projects {
			if p.Root == workspaceRoot {
				f.Projects[i].Topics = prependUniqueString(p.Topics, topicID)
				return true, nil
			}
		}
		f.Projects = append(f.Projects, desktopProject{Root: workspaceRoot, Topics: []string{topicID}})
		return true, nil
	}); err != nil {
		return err
	}
	_, err = agent.MutateExistingBranchMeta(sessionPath, false, func(meta *agent.BranchMeta) error {
		if scope == "global" {
			meta.Scope = "global"
			meta.WorkspaceRoot = ""
		} else {
			meta.Scope = "project"
			meta.WorkspaceRoot = workspaceRoot
		}
		meta.TopicID = topicID
		meta.TopicTitle = title
		return nil
	})
	if err != nil {
		return err
	}
	invalidateTopicSessionIndexForPath(sessionPath)
	return nil
}

func restoredSessionTopicTitle(dir, sessionPath string, meta agent.BranchMeta) string {
	if title := topicTitleFromText(meta.TopicTitle); title != "" {
		return title
	}
	if title := topicTitleFromText(loadSessionTitles(dir)[filepath.Base(sessionPath)]); title != "" {
		return title
	}
	if s, err := agent.LoadSession(sessionPath); err == nil {
		for _, msg := range s.Messages {
			if msg.Role == provider.RoleUser && !msg.IsProjectContext() {
				if title := topicTitleFromText(control.StripComposePrefixes(agent.HandoffTask(msg.Content))); title != "" {
					return title
				}
			}
		}
	}
	return ""
}

func legacySessionTopicID(path string) string {
	id := agent.BranchID(path)
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(id))
	var b strings.Builder
	b.WriteString("legacy_")
	for _, r := range id {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(r)
		case r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	prefix := strings.TrimRight(b.String(), "_")
	if prefix == "legacy" {
		prefix = "legacy_session"
	}
	return prefix + "_" + hex.EncodeToString(sum[:])[:12]
}

// TopicMeta describes a topic for the project tree.
type TopicMeta struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	CreatedAt int64  `json:"createdAt"`
}

// CreateTopic creates a new topic under a project workspace and returns its metadata.
func (a *App) CreateTopic(scope, workspaceRoot, title string) (TopicMeta, error) {
	return a.createTopic(scope, workspaceRoot, title, true)
}

func (a *App) createTopic(scope, workspaceRoot, title string, notifyProjectTree bool) (TopicMeta, error) {
	trimmedTitle := strings.TrimSpace(title)
	titleSource := topicTitleSourceManual
	if trimmedTitle == "" {
		trimmedTitle = defaultTopicTitle
		titleSource = topicTitleSourceAuto
	}
	topicID := newTopicID()
	createdAt := time.Now().UnixMilli()
	if _, _, err := registerTopic(scope, workspaceRoot, topicID, trimmedTitle, titleSource, createdAt); err != nil {
		return TopicMeta{}, err
	}
	if notifyProjectTree {
		a.emitProjectTreeChanged()
	}
	return TopicMeta{ID: topicID, Title: trimmedTitle, CreatedAt: createdAt}, nil
}

// RenameProject updates the sidebar-only display title for a project folder.
// Empty title clears the override and falls back to the folder name.
func (a *App) RenameProject(workspaceRoot, title string) error {
	if err := renameProject(workspaceRoot, title); err != nil {
		return err
	}
	a.emitProjectTreeChanged()
	return nil
}

// SetProjectColor updates the project-level accent color used by project topics
// in the sidebar and tabs. Empty color restores the default accent.
func (a *App) SetProjectColor(workspaceRoot, color string) error {
	if err := setProjectColor(workspaceRoot, color); err != nil {
		return err
	}
	a.emitProjectTreeChanged()
	return nil
}

// SetProjectPinned controls whether a project folder is pinned above the rest of
// the desktop project tree.
func (a *App) SetProjectPinned(workspaceRoot string, pinned bool) error {
	root := normalizeProjectRoot(workspaceRoot)
	if root == "" {
		return fmt.Errorf("workspaceRoot is required")
	}
	if err := updateProjectsFile(func(f *desktopProjectFile) (bool, error) {
		found := false
		for _, project := range f.Projects {
			if project.Root == root {
				found = true
				break
			}
		}
		if !found {
			return false, fmt.Errorf("project %q not found", root)
		}
		if pinned {
			f.PinnedProjects = prependUniqueString(f.PinnedProjects, root)
		} else {
			f.PinnedProjects = removeString(f.PinnedProjects, root)
		}
		return true, nil
	}); err != nil {
		return err
	}
	a.emitProjectTreeChanged()
	return nil
}

// ReorderProjects persists the user-defined order of project folders and,
// when present, the virtual Global sidebar section.
func (a *App) ReorderProjects(workspaceRoots []string) error {
	if err := updateProjectsFile(func(f *desktopProjectFile) (bool, error) {
		byRoot := make(map[string]desktopProject, len(f.Projects))
		for _, project := range f.Projects {
			byRoot[project.Root] = project
		}
		seen := make(map[string]bool, len(workspaceRoots))
		next := make([]desktopProject, 0, len(workspaceRoots))
		sidebarOrder := make([]string, 0, len(workspaceRoots))
		hasGlobalOrder := false
		for _, root := range workspaceRoots {
			root = strings.TrimSpace(root)
			if root == desktopGlobalOrderToken {
				if seen[root] {
					return false, fmt.Errorf("duplicate global section")
				}
				seen[root] = true
				hasGlobalOrder = true
				sidebarOrder = append(sidebarOrder, root)
				continue
			}
			root = normalizeProjectRoot(root)
			project, ok := byRoot[root]
			if !ok {
				return false, fmt.Errorf("project %q not found", root)
			}
			if seen[root] {
				return false, fmt.Errorf("duplicate project %q", root)
			}
			seen[root] = true
			next = append(next, project)
			sidebarOrder = append(sidebarOrder, root)
		}
		if len(next) != len(f.Projects) {
			return false, fmt.Errorf("project order length mismatch")
		}
		f.Projects = next
		if hasGlobalOrder {
			f.SidebarOrder = sidebarOrder
		} else {
			f.SidebarOrder = nil
		}
		return true, nil
	}); err != nil {
		return err
	}
	a.emitProjectTreeChanged()
	return nil
}

// RenameTopic updates a topic's display title.
func (a *App) RenameTopic(topicID, title string) error {
	trimmed := strings.TrimSpace(title)
	// Find which workspace this topic belongs to by scanning all project topic titles.
	f := loadProjectsFile()
	for _, p := range f.Projects {
		m := loadTopicTitles(p.Root)
		if _, ok := m[topicID]; ok {
			if err := setTopicTitle(p.Root, topicID, trimmed); err != nil {
				return err
			}
			a.updateOpenTopicTitle(topicID, trimmed)
			a.updateTopicSessionTitles(topicID, trimmed)
			a.emitProjectTreeChanged()
			return nil
		}
	}
	// Check global.
	m := loadTopicTitles("")
	if _, ok := m[topicID]; ok {
		if err := setTopicTitle("", topicID, trimmed); err != nil {
			return err
		}
		a.updateOpenTopicTitle(topicID, trimmed)
		a.updateTopicSessionTitles(topicID, trimmed)
		a.emitProjectTreeChanged()
		return nil
	}
	if scope, workspaceRoot, ok := a.findTopicLocation(topicID); ok {
		if err := ensureTopicIndexed(scope, workspaceRoot, topicID, trimmed, topicTitleSourceManual); err != nil {
			return err
		}
		a.updateOpenTopicTitle(topicID, trimmed)
		a.updateTopicSessionTitles(topicID, trimmed)
		a.emitProjectTreeChanged()
		return nil
	}
	return fmt.Errorf("topic %q not found", topicID)
}

func (a *App) findTopicLocation(topicID string) (string, string, bool) {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return "", "", false
	}
	a.mu.RLock()
	for _, tab := range a.tabRuntimes {
		if tab == nil || tab.TopicID != topicID {
			continue
		}
		scope := tab.Scope
		workspaceRoot := tab.WorkspaceRoot
		a.mu.RUnlock()
		if scope == "global" {
			return "global", "", true
		}
		return "project", normalizeProjectRoot(workspaceRoot), true
	}
	a.mu.RUnlock()

	infos, err := agent.ListSessions(config.SessionDir())
	if err != nil {
		return "", "", false
	}
	for _, info := range infos {
		if strings.TrimSpace(info.TopicID) != topicID {
			continue
		}
		scope := strings.TrimSpace(info.Scope)
		if scope == "" {
			scope = "global"
		}
		if scope == "global" {
			return "global", "", true
		}
		return "project", normalizeProjectRoot(info.WorkspaceRoot), true
	}
	return "", "", false
}

func (a *App) updateOpenTopicTitle(topicID, title string) {
	if strings.TrimSpace(topicID) == "" || strings.TrimSpace(title) == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, tab := range a.runtimeTabsLocked() {
		if tab != nil && tab.TopicID == topicID {
			tab.TopicTitle = title
		}
	}
}

func (a *App) updateTopicSessionTitles(topicID, title string) {
	a.updateTopicSessionTitlesInDirs(a.activeProfileSessionDirs(), topicID, title)
}

func (a *App) updateTopicSessionTitlesForActor(topicID, title string, actor userctx.ActorContext) {
	a.updateTopicSessionTitlesInDirs(a.sessionDirsForActorProfile(actor), topicID, title)
}

func (a *App) updateTopicSessionTitlesInDirs(dirs []string, topicID, title string) {
	_ = a.updateTopicSessionTitlesInDirsResult(dirs, topicID, title)
}

func (a *App) updateTopicSessionTitlesInDirsResult(dirs []string, topicID, title string) error {
	if strings.TrimSpace(topicID) == "" || strings.TrimSpace(title) == "" {
		return nil
	}
	var updateErr error
	for _, dir := range dirs {
		for _, match := range topicSessionMatches(dir, topicID) {
			updated, err := agent.MutateExistingBranchMeta(match.path, false, func(meta *agent.BranchMeta) error {
				meta.TopicTitle = title
				return nil
			})
			if err != nil {
				updateErr = errors.Join(updateErr, err)
				continue
			}
			if updated {
				invalidateTopicSessionIndex(dir)
			}
		}
	}
	return updateErr
}

func updateTopicSessionTitleAtPath(sessionPath, topicID, title string) error {
	sessionPath = strings.TrimSpace(sessionPath)
	topicID = strings.TrimSpace(topicID)
	title = strings.TrimSpace(title)
	if sessionPath == "" || topicID == "" || title == "" {
		return nil
	}
	_, err := agent.MutateExistingBranchMeta(sessionPath, false, func(meta *agent.BranchMeta) error {
		if strings.TrimSpace(meta.TopicID) != topicID {
			return fmt.Errorf("session topic %q does not match accepted topic %q", meta.TopicID, topicID)
		}
		meta.TopicTitle = title
		return nil
	})
	return err
}

func (a *App) setTabActivityStatus(tabID, status string) bool {
	changed, _ := a.setTabActivityStatusFromEvent(tabID, event.Event{}, status)
	return changed
}

func (a *App) setTabActivityStatusFromEvent(tabID string, e event.Event, status string) (changed, applied bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	tab := a.tabByEventSinkIDLocked(tabID)
	if tab == nil {
		return false, false
	}
	turnID := strings.TrimSpace(e.TurnID)
	switch e.Kind {
	case event.TurnStarted:
		tab.activityTurnID = turnID
	case event.TurnDone:
		if turnID != "" && tab.activityTurnID != "" && tab.activityTurnID != turnID {
			return false, false
		}
		tab.activityTurnID = ""
	}
	status = normalizeTopicStatus(status)
	if tab.ActivityStatus == status {
		return false, true
	}
	tab.ActivityStatus = status
	return true, true
}

func (a *App) emitProjectTreeChanged() {
	projectSessionCache.invalidate()
	if a == nil {
		return
	}
	rev := a.projectTreeRevision.Add(1)
	if a.projectTreeChangedHook != nil {
		a.projectTreeChangedHook()
		return
	}
	a.emitRuntimeEvent("project-tree:changed", rev)
}

func (a *App) scheduleProjectTreeMetadataRetry() {
	if a == nil || (a.ctx == nil && a.projectTreeChangedHook == nil) {
		return
	}
	if !a.projectTreeMetadataRetryScheduled.CompareAndSwap(false, true) {
		return
	}
	attempt := a.projectTreeMetadataRetryAttempt.Add(1)
	shift := attempt - 1
	if shift > 6 {
		shift = 6
	}
	delay := 250 * time.Millisecond * time.Duration(1<<shift)
	go func() {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		if a.ctx != nil {
			select {
			case <-timer.C:
			case <-a.ctx.Done():
				a.projectTreeMetadataRetryScheduled.Store(false)
				return
			}
		} else {
			<-timer.C
		}
		a.projectTreeMetadataRetryScheduled.Store(false)
		a.emitProjectTreeChanged()
	}()
}

func (a *App) emitModelRouteChanged(tabID string) {
	if a == nil {
		return
	}
	a.emitRuntimeEvent("model-route:changed", tabID)
}

func (a *App) ProjectTreeRevision() uint64 {
	if a == nil {
		return 0
	}
	return a.projectTreeRevision.Load()
}

func (a *App) emitRuntimeEvent(name string, payload ...interface{}) {
	if a == nil || a.ctx == nil {
		return
	}
	a.runtimeEvents.Emit(a.ctx, name, payload...)
}

// DeleteTopic removes a topic and its title metadata.
func (a *App) DeleteTopic(topicID string) error {
	f, err := loadProjectsFileForUpdate()
	if err != nil {
		return err
	}
	found := false
	for _, p := range f.Projects {
		deleted, err := deleteTopicTitleMetadata(p.Root, topicID)
		if err != nil {
			return err
		}
		if deleted {
			deleteTopicCreatedAt(p.Root, topicID)
			deleteTopicModel(p.Root, topicID)
			found = true
			break
		}
	}
	if !found {
		deleted, err := deleteTopicTitleMetadata("", topicID)
		if err != nil {
			return err
		}
		if deleted {
			deleteTopicCreatedAt("", topicID)
			deleteTopicModel("", topicID)
			found = true
		}
	}
	if !found {
		return fmt.Errorf("topic %q not found", topicID)
	}
	if err := updateProjectsFile(func(f *desktopProjectFile) (bool, error) {
		f.GlobalTopics = removeString(f.GlobalTopics, topicID)
		f.GlobalPinnedTopics = removeString(f.GlobalPinnedTopics, topicID)
		for i, p := range f.Projects {
			f.Projects[i].Topics = removeString(p.Topics, topicID)
			f.Projects[i].PinnedTopics = removeString(p.PinnedTopics, topicID)
		}
		return true, nil
	}); err != nil {
		return err
	}
	a.emitProjectTreeChanged()
	return nil
}

func deleteTopicTitleMetadata(workspaceRoot, topicID string) (bool, error) {
	topicTitleMu.Lock()
	defer topicTitleMu.Unlock()
	titles, err := loadTopicTitlesForUpdate(workspaceRoot)
	if err != nil {
		return false, err
	}
	if _, ok := titles[topicID]; !ok {
		return false, nil
	}
	if err := writeTopicTitleSource(workspaceRoot, topicID, "", ""); err != nil {
		return false, fmt.Errorf("delete topic title source: %w", err)
	}
	delete(titles, topicID)
	if err := saveTopicTitlesForUpdate(workspaceRoot, titles); err != nil {
		return false, fmt.Errorf("delete topic title: %w", err)
	}
	return true, nil
}

// SetTopicPinned controls whether a topic is pinned to the top of its project
// or Global section in the desktop project tree.
func (a *App) SetTopicPinned(topicID string, pinned bool) error {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return fmt.Errorf("topicID is required")
	}
	f, err := loadProjectsFileForUpdate()
	if err != nil {
		return err
	}
	targetRoot := ""
	for _, p := range f.Projects {
		m := loadTopicTitles(p.Root)
		if _, ok := m[topicID]; !ok && !containsDesktopString(p.Topics, topicID) {
			continue
		}
		targetRoot = p.Root
		break
	}
	isGlobal := targetRoot == ""
	if isGlobal {
		globalTitles := loadTopicTitles("")
		if _, ok := globalTitles[topicID]; !ok && !containsDesktopString(f.GlobalTopics, topicID) {
			return fmt.Errorf("topic %q not found", topicID)
		}
	}
	if err := updateProjectsFile(func(f *desktopProjectFile) (bool, error) {
		if isGlobal {
			if pinned {
				f.GlobalPinnedTopics = prependUniqueString(f.GlobalPinnedTopics, topicID)
			} else {
				f.GlobalPinnedTopics = removeString(f.GlobalPinnedTopics, topicID)
			}
			return true, nil
		}
		for i, p := range f.Projects {
			if p.Root != targetRoot {
				continue
			}
			if pinned {
				f.Projects[i].PinnedTopics = prependUniqueString(p.PinnedTopics, topicID)
			} else {
				f.Projects[i].PinnedTopics = removeString(p.PinnedTopics, topicID)
			}
			return true, nil
		}
		return false, fmt.Errorf("project %q not found", targetRoot)
	}); err != nil {
		return err
	}
	a.emitProjectTreeChanged()
	return nil
}

// TrashTopic removes a topic from the project tree and moves its saved session
// records into the session trash. Any in-process runtimes for the topic are
// cancelled and detached from the app first, so their autosave/jobs cannot
// recreate state after the topic is gone.
func (a *App) TrashTopic(topicID string) error {
	if strings.TrimSpace(topicID) == "" {
		return fmt.Errorf("topicID is required")
	}

	targets, err := a.topicTrashTargets(topicID)
	if err != nil {
		return err
	}
	removed, fallback, err := a.removeTopicRuntimeBindings(topicID)
	if err != nil {
		return err
	}
	if err := a.prepareRemovedSessionRuntimes(removed); err != nil {
		a.closeRemovedSessionRuntimes(removed)
		return err
	}
	destroyBegun := false
	closedRemoved := map[control.SessionAPI]bool{}
	defer func() {
		if destroyBegun {
			a.closeRemainingRemovedSessionRuntimesAfterDestroy(removed, closedRemoved)
			return
		}
		a.closeRemovedSessionRuntimes(removed)
	}()

	for _, target := range targets {
		destroys := a.destroyHandlesForSession(target.dir, target.sessionPath, removed)
		if len(destroys) > 0 {
			destroyBegun = true
		}
		teardownTimedOut := waitDestroyHandles(destroys)
		a.closeRemovedSessionRuntimesForSessionAfterDestroy(removed, target.dir, target.sessionPath, closedRemoved)
		if teardownTimedOut {
			if err := agent.MarkCleanupPending(target.sessionPath, "delete"); err != nil {
				return err
			}
			go delayedDesktopSessionTrash(target.dir, target.sessionPath, target.key, destroys)
		} else {
			err := trashSessionArtifacts(target.dir, target.sessionPath, target.key)
			finishDestroyHandles(destroys)
			if err != nil {
				return err
			}
		}
	}
	if err := a.DeleteTopic(topicID); err != nil {
		return err
	}
	if fallback.needs {
		fallback.topicID = ""
		if err := a.openFallbackRuntime(fallback); err != nil {
			return err
		}
	}
	a.emitProjectTreeChanged()
	return nil
}

type topicTrashTarget struct {
	dir         string
	sessionPath string
	key         string
}

func (a *App) topicTrashTargets(topicID string) ([]topicTrashTarget, error) {
	var targets []topicTrashTarget
	seen := map[string]bool{}
	dirs := a.activeProfileSessionDirs()
	visibleDirs := map[string]bool{}
	for _, dir := range dirs {
		if abs, err := filepath.Abs(dir); err == nil {
			visibleDirs[abs] = true
		}
	}
	addTarget := func(dir, path string) error {
		sessionPath, key, err := validateSessionPath(dir, path)
		if err != nil {
			return err
		}
		id := dir + "\x00" + sessionPath
		if seen[id] {
			return nil
		}
		seen[id] = true
		if err := validateSessionTrashTarget(dir, sessionPath, key); err != nil {
			return err
		}
		targets = append(targets, topicTrashTarget{dir: dir, sessionPath: sessionPath, key: key})
		return nil
	}
	for _, dir := range dirs {
		infos, err := agent.ListSessions(dir)
		if err != nil {
			return nil, err
		}
		for _, info := range infos {
			if info.TopicID != topicID {
				continue
			}
			if err := addTarget(dir, info.Path); err != nil {
				return nil, err
			}
		}
	}
	a.mu.RLock()
	var runtimeTargets []struct {
		dir  string
		path string
	}
	for _, tab := range a.runtimeTabsLocked() {
		if tab == nil || tab.TopicID != topicID {
			continue
		}
		if path := canonicalTabSessionPath(tab.currentSessionPath()); path != "" {
			dir := tabSessionDir(tab)
			if filepath.IsAbs(path) {
				dir = filepath.Dir(path)
			}
			if abs, err := filepath.Abs(dir); err == nil && !visibleDirs[abs] {
				continue
			}
			runtimeTargets = append(runtimeTargets, struct {
				dir  string
				path string
			}{dir: dir, path: path})
		}
	}
	a.mu.RUnlock()
	for _, target := range runtimeTargets {
		if err := addTarget(target.dir, target.path); err != nil {
			return nil, err
		}
	}
	return targets, nil
}

// ListProjectTree builds the sidebar tree: project folders each containing
// their topics, plus a Global section.
// topicSummary is used by ListProjectTree and mergeSessionInfos to track
// per-topic turn count and last activity.
type topicSummary struct {
	turns          int
	lastActivityAt int64
}

type cloudProjectTreeSession struct {
	info agent.SessionInfo
	meta agent.BranchMeta
}

type cloudProjectTreeClassification struct {
	sessions          []cloudProjectTreeSession
	cloudTopicKeys    map[string]bool
	physicalTopicKeys map[string]bool
	cloudSessionPaths map[string]bool
}

type cloudProjectTreeCatalogResult struct {
	projects              []project.Project
	sessionsByProject     map[string][]cloudProjectTreeSession
	cloudTopicKeys        map[string]bool
	physicalTopicKeys     map[string]bool
	cloudSessionPaths     map[string]bool
	managedWorkspaceRoots map[string]bool
	managedWorkspaceIDs   map[string]string
}

// classifyCloudProjectTreeSessions is deliberately independent of Cloud
// services. Project bindings are immutable device-local branch metadata, so
// they must still classify a transcript while signed out or while an account
// transition has made the project directory unavailable.
func classifyCloudProjectTreeSessions(sessionInfos map[string]agent.SessionInfo) cloudProjectTreeClassification {
	classification := cloudProjectTreeClassification{
		sessions:          make([]cloudProjectTreeSession, 0),
		cloudTopicKeys:    make(map[string]bool),
		physicalTopicKeys: make(map[string]bool),
		cloudSessionPaths: make(map[string]bool),
	}
	for _, info := range sessionInfos {
		meta, ok, metaErr := agent.LoadBranchMeta(info.Path)
		if metaErr != nil || !ok || !meta.IsCloudProjectBound() {
			if strings.TrimSpace(info.TopicID) != "" {
				classification.physicalTopicKeys[topicSummaryKey(info.Scope, info.WorkspaceRoot, info.TopicID)] = true
			}
			continue
		}
		physicalKey := topicSummaryKey(info.Scope, info.WorkspaceRoot, info.TopicID)
		classification.cloudTopicKeys[physicalKey] = true
		classification.cloudSessionPaths[sessionRuntimeKey(info.Path)] = true
		classification.sessions = append(classification.sessions, cloudProjectTreeSession{info: info, meta: meta})
	}
	return classification
}

// cloudProjectTreeCatalog reads only the current account's rebuildable project
// cache. It never creates a local folder and never performs a network request.
// Sessions with a stale account pin stay hidden until their owning account is
// active again.
func (a *App) cloudProjectTreeCatalog(sessionInfos map[string]agent.SessionInfo) cloudProjectTreeCatalogResult {
	classification := classifyCloudProjectTreeSessions(sessionInfos)
	result := cloudProjectTreeCatalogResult{
		cloudTopicKeys:      classification.cloudTopicKeys,
		physicalTopicKeys:   classification.physicalTopicKeys,
		cloudSessionPaths:   classification.cloudSessionPaths,
		managedWorkspaceIDs: make(map[string]string),
	}
	services, err := a.projectServicesForOperation()
	if err != nil {
		return result
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	snapshot, found, err := services.Cache.Load(ctx, services.Scope)
	if err != nil {
		return result
	}

	cachedProjects := make([]project.Project, 0)
	if found {
		cachedProjects = cloudProjectTreeProjects(snapshot, services.Scope)
	}
	// Local transcripts are never an access directory. This remains fail-closed
	// even after a previously authoritative cache expires, so a retained session
	// cannot resurrect a deleted project or a revoked membership. Valid cached
	// owner/member summaries still provide the existing offline presentation.
	result.projects, result.sessionsByProject, result.cloudTopicKeys, result.physicalTopicKeys, result.cloudSessionPaths = buildCloudProjectTreeCatalogFromClassification(cachedProjects, services.Scope, classification, false)
	result.managedWorkspaceRoots = make(map[string]bool, len(result.projects))
	for _, item := range result.projects {
		workspaceRoot, pathErr := cloudProjectWorkspacePath(services, item.ProjectID)
		if pathErr == nil {
			root := normalizeProjectRoot(workspaceRoot)
			result.managedWorkspaceRoots[root] = true
			result.managedWorkspaceIDs[root] = item.ProjectID
		}
	}
	return result
}

// cloudProjectTreeProjects merges the two independently authoritative Cloud
// directories. Owner summaries win duplicate IDs, while a same-owner detail
// may refresh presentation fields. Detail-only entries are admitted only until
// the corresponding directory has completed its first authoritative refresh;
// this preserves direct-open behavior without allowing an old local session or
// detail to resurrect access removed by a later empty/member directory result.
func cloudProjectTreeProjects(snapshot project.ProjectCacheSnapshot, scope project.Scope) []project.Project {
	details := make(map[string]project.Project, len(snapshot.Details))
	for _, detail := range snapshot.Details {
		details[detail.ProjectID] = detail
	}
	projects := make([]project.Project, 0, len(snapshot.Summaries.Items)+len(snapshot.JoinedSummaries.Items)+len(snapshot.Details))
	seen := make(map[string]struct{})
	appendSummary := func(summary project.Project) {
		if _, duplicate := seen[summary.ProjectID]; duplicate {
			return
		}
		seen[summary.ProjectID] = struct{}{}
		if detail, ok := details[summary.ProjectID]; ok && detail.OwnerUserID == summary.OwnerUserID && detail.Version >= summary.Version {
			summary = detail
		}
		projects = append(projects, summary)
	}
	for _, summary := range snapshot.Summaries.Items {
		appendSummary(summary)
	}
	for _, summary := range snapshot.JoinedSummaries.Items {
		appendSummary(summary)
	}
	for _, detail := range snapshot.Details {
		if _, duplicate := seen[detail.ProjectID]; duplicate {
			continue
		}
		owned := detail.OwnerUserID == scope.Binding.CloudUserID
		if (owned && snapshot.OwnedDirectoryRefreshed) || (!owned && snapshot.JoinedDirectoryRefreshed) {
			continue
		}
		seen[detail.ProjectID] = struct{}{}
		projects = append(projects, detail)
	}
	return projects
}

func buildCloudProjectTreeCatalog(cachedProjects []project.Project, scope project.Scope, sessionInfos map[string]agent.SessionInfo, includeSessionOnlyProjects bool) ([]project.Project, map[string][]cloudProjectTreeSession, map[string]bool, map[string]bool, map[string]bool) {
	return buildCloudProjectTreeCatalogFromClassification(cachedProjects, scope, classifyCloudProjectTreeSessions(sessionInfos), includeSessionOnlyProjects)
}

func buildCloudProjectTreeCatalogFromClassification(cachedProjects []project.Project, scope project.Scope, classification cloudProjectTreeClassification, includeSessionOnlyProjects bool) ([]project.Project, map[string][]cloudProjectTreeSession, map[string]bool, map[string]bool, map[string]bool) {
	projects := make([]project.Project, 0)
	seenProjects := make(map[string]struct{})
	for _, item := range cachedProjects {
		if item.Status != project.ProjectActive {
			continue
		}
		if _, exists := seenProjects[item.ProjectID]; exists {
			continue
		}
		seenProjects[item.ProjectID] = struct{}{}
		projects = append(projects, item)
	}

	sessions := make(map[string][]cloudProjectTreeSession)
	for _, entry := range classification.sessions {
		info, meta := entry.info, entry.meta
		if !projectContextPinMatchesScope(*meta.ProjectContext, scope) {
			continue
		}
		projectID := meta.ProjectContext.ProjectID
		if _, exists := seenProjects[projectID]; !exists && !includeSessionOnlyProjects {
			continue
		}
		sessions[projectID] = append(sessions[projectID], cloudProjectTreeSession{info: info, meta: meta})
		if _, exists := seenProjects[projectID]; !exists {
			seenProjects[projectID] = struct{}{}
			projects = append(projects, project.Project{ProjectID: projectID, Name: projectID, Status: project.ProjectActive})
		}
	}
	return projects, sessions, classification.cloudTopicKeys, classification.physicalTopicKeys, classification.cloudSessionPaths
}

var listProjectTreeMu sync.Mutex

func (a *App) ListProjectTree() []ProjectNode {
	listProjectTreeMu.Lock()
	defer listProjectTreeMu.Unlock()

	knownDirs := a.activeProfileSessionDirs()
	for _, dir := range knownDirs {
		migrateLegacySessionsIntoGlobalTopics(dir)
	}
	f := loadProjectsFile()
	out := []ProjectNode{}
	type runtimeSessionStatus struct {
		capability       *CapabilityTaskScope
		sessionPath      string
		label            string
		turns            int
		sessionTurns     *int
		scene            scene.Scene
		sceneState       scene.State
		createdAt        int64
		lastActivityAt   int64
		open             bool
		running          bool
		status           string
		composerIdentity composerIdentityView
	}
	topicSummaries := map[string]topicSummary{}
	sessionInfos := map[string]agent.SessionInfo{}
	sessionTitles := map[string]string{}
	sessionTurnsByPath := map[string]int{}
	latestSessionSelections := map[string]topicSessionSelection{}
	var metadataReadFailed atomic.Bool

	// Read session listings from all known directories concurrently, since
	// each dir is independent I/O. With N workspaces × dozens of sessions,
	// sequential reads add up to seconds of wall time on cold start.
	cacheToken := projectSessionCache.versionToken()
	type sessionDirLoadResult struct {
		dir    string
		infos  []agent.SessionInfo
		titles map[string]string
		latest map[string]topicSessionSelection
		turns  map[string]int
		ok     bool
	}
	results := make(chan sessionDirLoadResult, len(knownDirs))
	pendingLoads := 0
	for _, dir := range knownDirs {
		infos, titles, latest, turns, ok := projectSessionCache.get(dir)
		if ok {
			mergeSessionInfos(dir, infos, titles, sessionInfos, sessionTitles, topicSummaries)
			mergeTopicSessionSelections(latestSessionSelections, latest)
			maps.Copy(sessionTurnsByPath, turns)
			continue
		}
		pendingLoads++
		dir := dir // capture
		go func() {
			result := sessionDirLoadResult{dir: dir}
			defer func() {
				if recover() != nil {
					result.ok = false
				}
				results <- result
			}()

			// Sidecar-backed listing: ListSessions reads turn count + preview from
			// each session's .meta sidecar, so even large directories list in a few
			// milliseconds without decoding any .jsonl body. The in-memory
			// projectSessionCache still elides repeat listings within a session.
			infos, err := a.listSessionInfos(dir)
			if err != nil {
				return
			}
			latest, turns, err := loadLatestTopicSessionSelectionsForDir(dir)
			if err != nil {
				return
			}
			titlesResult := loadSessionTitlesResult(dir)
			if titlesResult.shouldRetry() {
				metadataReadFailed.Store(true)
			} else {
				projectSessionCache.put(dir, infos, titlesResult.values, latest, turns, cacheToken)
			}
			result.infos = infos
			result.titles = titlesResult.values
			result.latest = latest
			result.turns = turns
			result.ok = true
		}()
	}
	if pendingLoads > 0 {
		timer := time.NewTimer(5 * time.Second)
		for received := 0; received < pendingLoads; {
			select {
			case result := <-results:
				received++
				if result.ok {
					mergeSessionInfos(result.dir, result.infos, result.titles, sessionInfos, sessionTitles, topicSummaries)
					mergeTopicSessionSelections(latestSessionSelections, result.latest)
					maps.Copy(sessionTurnsByPath, result.turns)
				}
			case <-timer.C:
				received = pendingLoads
			}
		}
		timer.Stop()
	}

	runtimeSessionsByTopic := map[string][]runtimeSessionStatus{}
	runtimeSessionsByPath := map[string]runtimeSessionStatus{}
	capabilityScopesByTabID := a.capabilityTaskScopesByTabID()
	type pendingCloudProjectSession struct {
		projectID string
		info      agent.SessionInfo
	}
	pendingCloudProjectSessions := map[string]pendingCloudProjectSession{}
	// Empty and in-flight sessions are intentionally absent from the persisted
	// session index. Include their live listing identity before cloud-project
	// classification so a newly bound project session never appears under its
	// physical Global/Tasks topic while the first turn is still running.
	a.mu.RLock()
	mergeLiveSessionInfo := func(tab *WorkspaceTab) {
		if tab == nil || strings.TrimSpace(tab.TopicID) == "" {
			return
		}
		path := sessionRuntimeKey(tab.currentSessionPath())
		if path == "" {
			return
		}
		info, exists := sessionInfos[path]
		if !exists {
			info = agent.SessionInfo{
				Path:          path,
				Scope:         tab.Scope,
				WorkspaceRoot: tab.WorkspaceRoot,
				TopicID:       tab.TopicID,
				TopicTitle:    tab.TopicTitle,
			}
			sessionInfos[path] = info
		}
		if projectID := strings.TrimSpace(tab.pendingCloudProjectID); projectID != "" {
			// The live Topic title is authoritative for the prepared automation
			// session; a persisted Preview may already contain its internal prompt.
			info.Scope = tab.Scope
			info.WorkspaceRoot = tab.WorkspaceRoot
			info.TopicID = tab.TopicID
			info.TopicTitle = tab.TopicTitle
			pendingCloudProjectSessions[path] = pendingCloudProjectSession{
				projectID: projectID,
				info:      info,
			}
		}
	}
	for _, tab := range a.tabRuntimes {
		mergeLiveSessionInfo(tab)
	}
	for _, tab := range a.detachedSessions {
		mergeLiveSessionInfo(tab)
	}
	a.mu.RUnlock()
	cloudCatalog := a.cloudProjectTreeCatalog(sessionInfos)
	cloudProjects := cloudCatalog.projects
	cloudSessionsByProject := cloudCatalog.sessionsByProject
	cloudTopicKeys := cloudCatalog.cloudTopicKeys
	physicalTopicKeys := cloudCatalog.physicalTopicKeys
	cloudSessionPaths := cloudCatalog.cloudSessionPaths
	if cloudSessionsByProject == nil {
		cloudSessionsByProject = map[string][]cloudProjectTreeSession{}
	}
	for path, pending := range pendingCloudProjectSessions {
		if cloudSessionPaths[path] {
			continue
		}
		meta := agent.BranchMeta{
			Scope:         pending.info.Scope,
			WorkspaceRoot: pending.info.WorkspaceRoot,
			TopicID:       pending.info.TopicID,
			TopicTitle:    pending.info.TopicTitle,
		}
		cloudSessionsByProject[pending.projectID] = append(cloudSessionsByProject[pending.projectID], cloudProjectTreeSession{
			info: pending.info,
			meta: meta,
		})
		cloudSessionPaths[path] = true
		topicKey := topicSummaryKey(pending.info.Scope, pending.info.WorkspaceRoot, pending.info.TopicID)
		cloudTopicKeys[topicKey] = true
		// classifyCloudProjectTreeSessions saw the not-yet-pinned branch as a
		// physical session. Remove that provisional classification unless a
		// different, genuinely physical session still belongs to the Topic.
		delete(physicalTopicKeys, topicKey)
		for otherPath, otherInfo := range sessionInfos {
			if _, isPending := pendingCloudProjectSessions[otherPath]; isPending ||
				topicSummaryKey(otherInfo.Scope, otherInfo.WorkspaceRoot, otherInfo.TopicID) != topicKey {
				continue
			}
			otherMeta, ok, err := agent.LoadBranchMeta(otherInfo.Path)
			if err != nil || !ok || !otherMeta.IsCloudProjectBound() {
				physicalTopicKeys[topicKey] = true
				break
			}
		}
	}
	a.mu.RLock()
	seenRuntimePaths := map[string]bool{}
	addRuntimeSession := func(tab *WorkspaceTab, open bool) {
		if tab == nil || strings.TrimSpace(tab.TopicID) == "" {
			return
		}
		sessionPath := sessionRuntimeKey(tab.currentSessionPath())
		if sessionPath == "" || seenRuntimePaths[sessionPath] {
			return
		}
		seenRuntimePaths[sessionPath] = true
		info := sessionInfos[sessionPath]
		storedIdentity := storedSessionComposerIdentityView(info)
		sessionTurns, hasSessionTurns := sessionTurnsByPath[sessionPath]
		var exactSessionTurns *int
		if tab.sessionTurns != nil {
			exactSessionTurns = cloneIntPointer(tab.sessionTurns)
		} else if hasSessionTurns {
			exactSessionTurns = &sessionTurns
		}
		label := runtimeSessionTreeLabel(tab, info, sessionTitles[sessionPath])
		status := activityStatusForTab(tab)
		capability := cloneCapabilityTaskScope(capabilityScopesByTabID[tab.ID])
		if capability == nil {
			capability = a.capabilityTaskScopeForSession(tab.WorkspaceRoot, sessionPath)
		}
		runtimeStatus := control.RuntimeStatus{}
		if tab.Ctrl != nil {
			runtimeStatus = tab.Ctrl.RuntimeStatus()
		}
		running := status != "" || runtimeStatus.Running || runtimeStatus.PendingPrompt || runtimeStatus.BackgroundJobs > 0
		scenePresentation := projectTreeScenePresentationStatus(tabSceneStatus(tab))
		lastActivityAt := unixMilliOrZero(info.LastActivityAt)
		if acceptedAt, ok := a.conversationActivity.Load(sessionPath); ok {
			if value, ok := acceptedAt.(int64); ok && value > lastActivityAt {
				lastActivityAt = value
			}
		}
		runtimeSession := runtimeSessionStatus{
			capability:       capability,
			sessionPath:      sessionPath,
			label:            label,
			turns:            info.Turns,
			sessionTurns:     exactSessionTurns,
			scene:            scenePresentation.Scene,
			sceneState:       scenePresentation.State,
			createdAt:        unixMilliOrZero(info.CreatedAt),
			lastActivityAt:   lastActivityAt,
			open:             open,
			running:          running,
			status:           status,
			composerIdentity: runtimeComposerIdentityView(tab, storedIdentity),
		}
		if !cloudSessionPaths[sessionPath] {
			topicKey := topicSummaryKey(tab.Scope, tab.WorkspaceRoot, tab.TopicID)
			runtimeSessionsByTopic[topicKey] = append(runtimeSessionsByTopic[topicKey], runtimeSession)
			summary := topicSummaries[topicKey]
			if lastActivityAt > summary.lastActivityAt {
				summary.lastActivityAt = lastActivityAt
				topicSummaries[topicKey] = summary
			}
		}
		runtimeSessionsByPath[sessionPath] = runtimeSession
	}
	for _, tab := range a.tabRuntimes {
		addRuntimeSession(tab, true)
	}
	for _, tab := range a.detachedSessions {
		addRuntimeSession(tab, false)
	}
	a.mu.RUnlock()
	for key := range runtimeSessionsByTopic {
		sort.Slice(runtimeSessionsByTopic[key], func(i, j int) bool {
			left := runtimeSessionsByTopic[key][i]
			right := runtimeSessionsByTopic[key][j]
			if left.lastActivityAt != right.lastActivityAt {
				return left.lastActivityAt > right.lastActivityAt
			}
			return left.sessionPath < right.sessionPath
		})
	}
	topicRuntimeStatus := func(key string) (open, running bool, status string) {
		sessions := runtimeSessionsByTopic[key]
		if len(sessions) != 1 {
			return false, false, ""
		}
		session := sessions[0]
		return session.open, session.running, session.status
	}
	topicTargetSessionPath := func(key string, selection topicSessionSelection) string {
		if selectedPath := strings.TrimSpace(selection.path); selectedPath != "" {
			return selectedPath
		}
		sessions := runtimeSessionsByTopic[key]
		if len(sessions) == 1 {
			return sessions[0].sessionPath
		}
		return ""
	}
	topicSessionTurns := func(key string, selection topicSessionSelection) *int {
		selectedPath := sessionRuntimeKey(topicTargetSessionPath(key, selection))
		if selectedPath != "" {
			for _, session := range runtimeSessionsByTopic[key] {
				if sessionRuntimeKey(session.sessionPath) == selectedPath && session.sessionTurns != nil {
					return cloneIntPointer(session.sessionTurns)
				}
			}
		}
		return selection.sessionTurns()
	}
	topicSessionSceneStatus := func(key string, selection topicSessionSelection) scene.Status {
		selectedPath := sessionRuntimeKey(topicTargetSessionPath(key, selection))
		if selectedPath != "" {
			for _, session := range runtimeSessionsByTopic[key] {
				if sessionRuntimeKey(session.sessionPath) == selectedPath {
					return scene.Status{Scene: session.scene, State: session.sceneState}
				}
			}
		}
		return selection.sceneStatus()
	}
	topicSessionComposerIdentity := func(key string, selection topicSessionSelection) composerIdentityView {
		selectedPath := sessionRuntimeKey(topicTargetSessionPath(key, selection))
		if selectedPath != "" {
			for _, session := range runtimeSessionsByTopic[key] {
				if sessionRuntimeKey(session.sessionPath) == selectedPath {
					return session.composerIdentity
				}
			}
		}
		view := defaultComposerIdentityView()
		if info, ok := sessionInfos[selectedPath]; ok {
			view = storedSessionComposerIdentityView(info)
		}
		view.modelRef = selection.modelRef
		view.billingSource = selection.billingSource
		return view
	}
	topicSessionCapability := func(workspaceRoot, key string, selection topicSessionSelection) *CapabilityTaskScope {
		selectedPath := sessionRuntimeKey(topicTargetSessionPath(key, selection))
		if selectedPath == "" {
			return nil
		}
		for _, session := range runtimeSessionsByTopic[key] {
			if sessionRuntimeKey(session.sessionPath) == selectedPath {
				return cloneCapabilityTaskScopeValue(session.capability)
			}
		}
		return a.capabilityTaskScopeForSession(workspaceRoot, selectedPath)
	}
	runtimeSessionNodes := func(scope, workspaceRoot, topicID, projectColor string) []ProjectNode {
		key := topicSummaryKey(scope, workspaceRoot, topicID)
		sessions := runtimeSessionsByTopic[key]
		if len(sessions) <= 1 {
			return nil
		}
		nodes := make([]ProjectNode, 0, len(sessions))
		for _, session := range sessions {
			kind := "session"
			if scope == "global" {
				kind = "global_session"
			}
			node := ProjectNode{
				Key:            projectSessionNodeKey(scope, session.sessionPath),
				Kind:           kind,
				Label:          session.label,
				Root:           workspaceRoot,
				TopicID:        topicID,
				SessionPath:    session.sessionPath,
				SessionTurns:   session.sessionTurns,
				Scene:          string(session.scene),
				SceneState:     string(session.sceneState),
				ProjectColor:   projectColor,
				Turns:          session.turns,
				CreatedAt:      session.createdAt,
				LastActivityAt: session.lastActivityAt,
				Open:           session.open,
				Running:        session.running,
				Status:         session.status,
				Capability:     cloneCapabilityTaskScopeValue(session.capability),
			}
			applyComposerIdentityToProjectNode(&node, session.composerIdentity)
			nodes = append(nodes, node)
		}
		return nodes
	}

	// Global section.
	globalTitlesResult := loadTopicTitlesResult("")
	globalCreatedResult := loadTopicCreatedAtsResult("")
	if globalTitlesResult.shouldRetry() || globalCreatedResult.shouldRetry() {
		metadataReadFailed.Store(true)
	}
	globalTitleMap := globalTitlesResult.values
	globalCreatedMap := globalCreatedResult.values
	if len(globalTitleMap) > 0 || len(f.Projects) == 0 {
		globalTitle := strings.TrimSpace(f.GlobalTitle)
		if globalTitle == "" {
			globalTitle = "Global"
		}
		globalColor := normalizeProjectColor(f.GlobalColor)
		globalTopicIDs := pinnedTopicIDs(orderedTopicIDs(f.GlobalTopics, globalTitleMap), f.GlobalPinnedTopics)
		children := make([]ProjectNode, 0, len(globalTopicIDs))
		for _, id := range globalTopicIDs {
			topicKey := topicSummaryKey("global", "", id)
			if cloudTopicKeys[topicKey] && !physicalTopicKeys[topicKey] {
				continue
			}
			title := strings.TrimSpace(globalTitleMap[id])
			if title == "" {
				title = defaultTopicTitle
			}
			summary := topicSummaries[topicKey]
			selection := latestSessionSelections[topicSessionSelectionKey("global", "", id)]
			scenePresentation := topicSessionSceneStatus(topicKey, selection)
			open, running, status := topicRuntimeStatus(topicKey)
			pinned := containsDesktopString(f.GlobalPinnedTopics, id)
			node := ProjectNode{
				Key:            "global_topic_" + id,
				Kind:           "global_topic",
				Label:          title,
				TopicID:        id,
				SessionPath:    topicTargetSessionPath(topicKey, selection),
				SessionTurns:   topicSessionTurns(topicKey, selection),
				Scene:          string(scenePresentation.Scene),
				SceneState:     string(scenePresentation.State),
				ProjectColor:   globalColor,
				Turns:          summary.turns,
				CreatedAt:      globalCreatedMap[id],
				LastActivityAt: summary.lastActivityAt,
				Open:           open,
				Running:        running,
				Status:         status,
				Pinned:         pinned,
				Capability:     topicSessionCapability("", topicKey, selection),
				Children:       runtimeSessionNodes("global", "", id, globalColor),
			}
			applyComposerIdentityToProjectNode(&node, topicSessionComposerIdentity(topicKey, selection))
			children = append(children, node)
		}
		out = append(out, ProjectNode{
			Key:          "global_folder",
			Kind:         "global_folder",
			Label:        globalTitle,
			Root:         globalWorkspaceRoot(),
			ProjectColor: globalColor,
			Children:     children,
		})
	}

	// Project sections.
	type projectTopics struct {
		project       desktopProject
		titles        map[string]string
		createdAts    map[string]int64
		retryMetadata bool
	}
	visibleProjects := make([]desktopProject, 0, len(f.Projects))
	managedStateRoots := a.cloudProjectWorkspaceStateRoots()
	for _, p := range f.Projects {
		// Internal workspaces remain hidden after deletion or membership removal,
		// when their logical Cloud Project is no longer in the access directory.
		if cloudCatalog.managedWorkspaceRoots[normalizeProjectRoot(p.Root)] ||
			managedCloudProjectWorkspaceInStateRoots(p.Root, managedStateRoots) {
			continue
		}
		visibleProjects = append(visibleProjects, p)
	}
	projectTopicResults := make([]projectTopics, len(visibleProjects))
	var topicLoadWg sync.WaitGroup
	for i, p := range visibleProjects {
		i, p := i, p
		topicLoadWg.Add(1)
		go func() {
			defer topicLoadWg.Done()
			titlesResult := loadTopicTitlesResult(p.Root)
			createdResult := loadTopicCreatedAtsResult(p.Root)
			projectTopicResults[i] = projectTopics{
				project:       p,
				titles:        titlesResult.values,
				createdAts:    createdResult.values,
				retryMetadata: titlesResult.shouldRetry() || createdResult.shouldRetry(),
			}
		}()
	}
	topicLoadWg.Wait()
	for _, loaded := range projectTopicResults {
		if loaded.retryMetadata {
			metadataReadFailed.Store(true)
		}
		p := loaded.project
		title := p.Title
		if title == "" {
			title = workspaceName(p.Root)
		}
		node := ProjectNode{
			Key:    "project_" + p.Root,
			Kind:   "project",
			Root:   p.Root,
			Pinned: containsDesktopString(f.PinnedProjects, p.Root),
		}

		// Gather topics: explicit topic list + all known topic titles.
		titleMap := loaded.titles
		createdMap := loaded.createdAts
		topicIDs := pinnedTopicIDs(orderedTopicIDs(p.Topics, titleMap), p.PinnedTopics)

		children := make([]ProjectNode, 0, len(topicIDs))
		for _, tid := range topicIDs {
			topicKey := topicSummaryKey("project", p.Root, tid)
			if cloudTopicKeys[topicKey] && !physicalTopicKeys[topicKey] {
				continue
			}
			topicTitle := strings.TrimSpace(titleMap[tid])
			if topicTitle == "" {
				topicTitle = defaultTopicTitle
			}
			summary := topicSummaries[topicKey]
			selection := latestSessionSelections[topicSessionSelectionKey("project", p.Root, tid)]
			scenePresentation := topicSessionSceneStatus(topicKey, selection)
			open, running, status := topicRuntimeStatus(topicKey)
			pinned := containsDesktopString(p.PinnedTopics, tid)
			topicNode := ProjectNode{
				Key:            "topic_" + tid,
				Kind:           "topic",
				Label:          topicTitle,
				Root:           p.Root,
				TopicID:        tid,
				SessionPath:    topicTargetSessionPath(topicKey, selection),
				SessionTurns:   topicSessionTurns(topicKey, selection),
				Scene:          string(scenePresentation.Scene),
				SceneState:     string(scenePresentation.State),
				ProjectColor:   p.Color,
				Turns:          summary.turns,
				CreatedAt:      createdMap[tid],
				LastActivityAt: summary.lastActivityAt,
				Open:           open,
				Running:        running,
				Status:         status,
				Pinned:         pinned,
				Capability:     topicSessionCapability(p.Root, topicKey, selection),
				Children:       runtimeSessionNodes("project", p.Root, tid, p.Color),
			}
			applyComposerIdentityToProjectNode(&topicNode, topicSessionComposerIdentity(topicKey, selection))
			children = append(children, topicNode)
		}
		node.Label = title
		node.ProjectColor = p.Color
		node.Children = children
		out = append(out, node)
	}

	// Cloud projects are logical navigation nodes. Their child sessions retain
	// the real global/local execution target so opening one resumes the exact
	// controller route without turning the cloud project into a filesystem path.
	for _, cloudProject := range cloudProjects {
		projectID := cloudProject.ProjectID
		entries := cloudSessionsByProject[projectID]
		children := make([]ProjectNode, 0, len(entries))
		for _, entry := range entries {
			info, meta := entry.info, entry.meta
			sessionPath := sessionRuntimeKey(info.Path)
			if sessionPath == "" {
				continue
			}
			kind, root := "global_session", ""
			if meta.DefaultScope() == "project" && strings.TrimSpace(meta.WorkspaceRoot) != "" {
				kind, root = "session", meta.WorkspaceRoot
			}
			label := cloudProjectSessionTreeLabel(meta, info, sessionTitles[sessionPath])
			turns := info.Turns
			sessionTurns := &turns
			status := runtimeSessionsByPath[sessionPath]
			createdAt, lastActivityAt := cloudProjectSessionActivityTimes(info, meta)
			sceneStatus, sceneErr := meta.SceneStatus()
			if sceneErr != nil {
				sceneStatus = scene.Status{}
			}
			scenePresentation := projectTreeScenePresentationStatus(sceneStatus)
			node := ProjectNode{
				Key: "cloud_" + projectSessionNodeKey(kind, sessionPath), Kind: kind, Label: label,
				Root: root, CloudProjectID: projectID, TopicID: meta.TopicID, SessionPath: sessionPath,
				SessionTurns: sessionTurns, Scene: string(scenePresentation.Scene), SceneState: string(scenePresentation.State),
				Turns: info.Turns, CreatedAt: createdAt, LastActivityAt: lastActivityAt,
				Open: status.open, Running: status.running, Status: status.status, Pinned: meta.ProjectSessionPinned,
				Capability: a.capabilityTaskScopeForSession(root, sessionPath),
			}
			identity := storedSessionComposerIdentityView(info)
			if status.sessionPath != "" {
				node.Label = status.label
				if status.sessionTurns != nil {
					node.SessionTurns = status.sessionTurns
				}
				identity = status.composerIdentity
				if status.lastActivityAt > node.LastActivityAt {
					node.LastActivityAt = status.lastActivityAt
				}
			}
			applyComposerIdentityToProjectNode(&node, identity)
			children = append(children, node)
		}
		sortCloudProjectSessionNodes(children)
		label := strings.TrimSpace(cloudProject.Name)
		if label == "" {
			label = projectID
		}
		out = append(out, ProjectNode{Key: "cloud_project_" + projectID, Kind: "cloud_project", Label: label, CloudProjectID: projectID, Children: children})
	}

	if metadataReadFailed.Load() {
		a.scheduleProjectTreeMetadataRetry()
	} else {
		a.projectTreeMetadataRetryAttempt.Store(0)
	}
	return applyPinnedProjectOrder(applyProjectTreeOrder(out, f.SidebarOrder), f.PinnedProjects)
}

func cloudProjectSessionActivityTimes(info agent.SessionInfo, meta agent.BranchMeta) (int64, int64) {
	createdAt := unixMilliOrZero(info.CreatedAt)
	if createdAt == 0 {
		createdAt = unixMilliOrZero(meta.CreatedAt)
	}
	lastActivityAt := unixMilliOrZero(info.LastActivityAt)
	if lastActivityAt == 0 {
		lastActivityAt = unixMilliOrZero(meta.UpdatedAt)
	}
	if lastActivityAt == 0 {
		lastActivityAt = createdAt
	}
	return createdAt, lastActivityAt
}

func sortCloudProjectSessionNodes(children []ProjectNode) {
	sort.Slice(children, func(i, j int) bool {
		if children[i].Pinned != children[j].Pinned {
			return children[i].Pinned
		}
		if children[i].LastActivityAt != children[j].LastActivityAt {
			return children[i].LastActivityAt > children[j].LastActivityAt
		}
		return children[i].SessionPath < children[j].SessionPath
	})
}

func topicSummaryKey(scope, workspaceRoot, topicID string) string {
	if scope == "global" {
		return "global::" + topicID
	}
	return "project:" + workspaceRoot + ":" + topicID
}

func projectSessionNodeKey(scope, sessionPath string) string {
	sum := sha256.Sum256([]byte(sessionRuntimeKey(sessionPath)))
	return scope + "_session_" + hex.EncodeToString(sum[:8])
}

func runtimeSessionTreeLabel(tab *WorkspaceTab, info agent.SessionInfo, title string) string {
	if title = strings.TrimSpace(title); title != "" {
		return title
	}
	if tab != nil {
		topicTitle := strings.TrimSpace(tab.TopicTitle)
		if topicTitle != "" && topicTitle != strings.TrimSpace(tab.TopicID) {
			return topicTitle
		}
	}
	if preview := topicTitleFromText(info.Preview); preview != "" {
		return preview
	}
	if tab != nil {
		return safeSessionTreeLabel(tab.TopicID, tab.TopicTitle, "")
	}
	return defaultTopicTitle
}

func cloudProjectSessionTreeLabel(meta agent.BranchMeta, info agent.SessionInfo, sessionTitle string) string {
	if sessionTitle = strings.TrimSpace(sessionTitle); sessionTitle != "" {
		return sessionTitle
	}
	topicID := strings.TrimSpace(meta.TopicID)
	for _, candidate := range []string{meta.TopicTitle, info.TopicTitle} {
		candidate = strings.TrimSpace(candidate)
		if candidate != "" && candidate != topicID && candidate != defaultTopicTitle {
			return candidate
		}
	}
	if preview := topicTitleFromText(info.Preview); preview != "" {
		return preview
	}
	return defaultTopicTitle
}

func safeSessionTreeLabel(topicID, title, preview string) string {
	title = strings.TrimSpace(title)
	if title != "" && title != strings.TrimSpace(topicID) {
		return title
	}
	if preview = topicTitleFromText(preview); preview != "" {
		return preview
	}
	return defaultTopicTitle
}

func unixMilliOrZero(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

// ContextPanelInfo is the right-side panel's data for one tab.
type ContextPanelInfo struct {
	UsedTokens       int                         `json:"usedTokens"`
	WindowTokens     int                         `json:"windowTokens"`
	PromptTokens     int                         `json:"promptTokens"`
	CompletionTokens int                         `json:"completionTokens"`
	TotalTokens      int                         `json:"totalTokens"`
	ReasoningTokens  int                         `json:"reasoningTokens"`
	CacheHitTokens   int                         `json:"cacheHitTokens"`
	CacheMissTokens  int                         `json:"cacheMissTokens"`
	RequestCount     int                         `json:"requestCount"`
	ElapsedMs        int64                       `json:"elapsedMs"`
	SessionCost      float64                     `json:"sessionCost"`
	SessionCurrency  string                      `json:"sessionCurrency,omitempty"`
	SessionCostUsd   float64                     `json:"sessionCostUsd,omitempty"`
	Sources          map[string]usageSourceStats `json:"sources,omitempty"`
	Mock             bool                        `json:"mock,omitempty"`
	ReadFiles        []readFileRecord            `json:"readFiles"`
	ChangedFiles     []ChangedFileInfo           `json:"changedFiles"`
}

type ChangedFileInfo struct {
	Path         string   `json:"path"`
	OldPath      string   `json:"oldPath,omitempty"`
	Sources      []string `json:"sources"`
	GitStatus    string   `json:"gitStatus,omitempty"`
	Turns        []int    `json:"turns"`
	LatestPrompt string   `json:"latestPrompt,omitempty"`
	LatestTime   int64    `json:"latestTime,omitempty"`
}

// ContextPanel returns the context usage, read files, and changed files for a
// specific tab.
func (a *App) ContextPanel(tabID string) ContextPanelInfo {
	a.mu.RLock()
	tab, ok := a.tabRuntimes[tabID]
	var ctrl control.SessionAPI
	if ok && tab != nil {
		ctrl = tab.Ctrl
	}
	a.mu.RUnlock()
	if !ok {
		return ContextPanelInfo{ReadFiles: []readFileRecord{}, ChangedFiles: []ChangedFileInfo{}}
	}

	info := ContextPanelInfo{ReadFiles: []readFileRecord{}, ChangedFiles: []ChangedFileInfo{}}
	if ctrl != nil {
		used, window := ctrl.ContextSnapshot()
		info.UsedTokens = used
		info.WindowTokens = window
		// Per-turn token breakdown from LastUsage (same snapshot as UsedTokens)
		// so the donut segments are proportional to the current context fill,
		// not inflated by cumulative session totals.
		if u := ctrl.LastUsage(); u != nil {
			info.PromptTokens = u.PromptTokens
			info.CompletionTokens = u.CompletionTokens
			info.ReasoningTokens = u.ReasoningTokens
			info.CacheHitTokens = u.CacheHitTokens
			info.CacheMissTokens = u.CacheMissTokens
		}
	}

	telemetry := tab.telemetrySnapshot()
	if records := telemetry.ReadFiles; records != nil {
		info.ReadFiles = records
	}
	usage := telemetry.Usage
	info.TotalTokens = usage.TotalTokens
	info.RequestCount = usage.RequestCount
	info.ElapsedMs = usage.ElapsedMs
	info.SessionCost = usage.SessionCost
	info.SessionCurrency = usage.SessionCurrency
	info.SessionCostUsd = usage.SessionCostUsd
	info.Sources = usage.Sources

	// Gather workspace changes for this tab's root.
	if ctrl != nil && tab.WorkspaceRoot != "" {
		for _, meta := range ctrl.Checkpoints() {
			for _, path := range meta.Paths {
				info.ChangedFiles = append(info.ChangedFiles, ChangedFileInfo{
					Path:         path,
					Sources:      []string{"session"},
					Turns:        []int{meta.Turn},
					LatestPrompt: meta.Prompt,
					LatestTime:   meta.Time.UnixMilli(),
				})
			}
		}
	}

	return info
}

// --- utility ----------------------------------------------------------------

func (a *App) newUniqueTabIDLocked() string {
	for {
		id := newTabID()
		if _, exists := a.tabRuntimes[id]; !exists {
			return id
		}
	}
}

func (a *App) restoredTabIDLocked(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return a.newUniqueTabIDLocked()
	}
	if _, exists := a.tabRuntimes[id]; exists {
		return a.newUniqueTabIDLocked()
	}
	return id
}

func normalizeTabMode(mode string) string {
	switch mode {
	case "plan", "yolo", "plan-yolo", "yolo-plan":
		if mode == "yolo-plan" {
			return "plan-yolo"
		}
		return mode
	default:
		return "normal"
	}
}

func tabModeFromAxes(plan, autoApproveTools bool) string {
	switch {
	case plan && autoApproveTools:
		return "plan-yolo"
	case plan:
		return "plan"
	case autoApproveTools:
		return "yolo"
	default:
		return "normal"
	}
}

func tabModeHasPlan(mode string) bool {
	switch normalizeTabMode(mode) {
	case "plan", "plan-yolo":
		return true
	default:
		return false
	}
}

func tabModeHasAutoApproveTools(mode string) bool {
	switch normalizeTabMode(mode) {
	case "yolo", "plan-yolo":
		return true
	default:
		return false
	}
}

func currentTabMode(tab *WorkspaceTab) string {
	if tab == nil {
		return "normal"
	}
	if tab.Ctrl != nil {
		return tabModeFromAxes(tab.Ctrl.PlanMode(), tab.Ctrl.AutoApproveTools())
	}
	return normalizeTabMode(tab.mode)
}

func currentTabGoal(tab *WorkspaceTab) string {
	if tab == nil {
		return ""
	}
	if tab.Ctrl != nil {
		return tab.Ctrl.Goal()
	}
	return strings.TrimSpace(tab.goal)
}

func currentTabGoalStatus(tab *WorkspaceTab) string {
	if tab == nil {
		return control.GoalStatusStopped
	}
	if tab.Ctrl != nil {
		return tab.Ctrl.GoalStatus()
	}
	if strings.TrimSpace(tab.goal) != "" {
		return control.GoalStatusRunning
	}
	return control.GoalStatusStopped
}

func currentTabCollaborationMode(tab *WorkspaceTab) string {
	if tab == nil {
		return "normal"
	}
	if tab.Ctrl != nil && tab.Ctrl.PlanMode() {
		return "plan"
	}
	if strings.TrimSpace(currentTabGoal(tab)) != "" && currentTabGoalStatus(tab) == control.GoalStatusRunning {
		return "goal"
	}
	return "normal"
}

func currentTabToolApprovalMode(tab *WorkspaceTab) string {
	if tab == nil {
		return control.ToolApprovalAsk
	}
	if tab.Ctrl != nil {
		return tab.Ctrl.ToolApprovalMode()
	}
	return normalizeToolApprovalMode(tab.toolApprovalMode)
}

func currentTabTokenMode(tab *WorkspaceTab) string {
	if tab == nil {
		return boot.TokenModeFull
	}
	return boot.NormalizeTokenMode(tab.tokenMode)
}

func persistedTabTokenMode(mode string) string {
	mode = boot.NormalizeTokenMode(mode)
	if mode == boot.TokenModeEconomy {
		return mode
	}
	return ""
}

func normalizeToolApprovalMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case control.ToolApprovalAuto:
		return control.ToolApprovalAuto
	case control.ToolApprovalYolo, "full", "full-access", "bypass":
		return control.ToolApprovalYolo
	default:
		return control.ToolApprovalAsk
	}
}

func persistedToolApprovalMode(mode string) string {
	switch normalizeToolApprovalMode(mode) {
	case control.ToolApprovalAuto, control.ToolApprovalYolo:
		return normalizeToolApprovalMode(mode)
	default:
		return ""
	}
}

// persistedTabMode is the composer mode saved with a tab so it survives reload
// and app relaunch. plan, yolo, and plan-yolo are remembered (a restored yolo
// tab keeps its status-bar indicator); "normal" is the default and isn't
// persisted. (#3517)
func persistedTabMode(mode string) string {
	switch normalizeTabMode(mode) {
	case "plan", "yolo", "plan-yolo":
		return normalizeTabMode(mode)
	}
	return ""
}

func newTabID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		now := time.Now().UTC()
		return "tab_" + now.Format("20060102150405") + "_" + fmt.Sprintf("%09d", now.Nanosecond())
	}
	return "tab_" + hex.EncodeToString(b[:])
}

func newTopicID() string {
	var b [8]byte
	rand.Read(b[:])
	return "topic_" + time.Now().UTC().Format("20060102-150405") + "_" + hex.EncodeToString(b[:])
}

func globalWorkspaceRoot() string {
	return filepath.Join(desktopConfigDir(), "global-workspace")
}

func ensureGlobalWorkspaceRoot() (string, error) {
	root := globalWorkspaceRoot()
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	return root, nil
}

func globalTabWorkspaceRoot() string {
	root, err := ensureGlobalWorkspaceRoot()
	if err != nil {
		return globalWorkspaceRoot()
	}
	return root
}

func loadPinnedTabSession(dir, sessionPath string) (*agent.Session, string, bool) {
	return loadPinnedTabSessionWithPreload(dir, sessionPath, loadedTabSession{})
}

func loadPinnedTabSessionWithPreload(dir, sessionPath string, preloaded loadedTabSession) (*agent.Session, string, bool) {
	path, ok := pinnedTabSessionPath(dir, sessionPath)
	if !ok {
		return nil, "", false
	}
	if agent.IsCleanupPending(path) {
		return nil, "", false
	}
	if preloaded.matches(path) {
		if preloaded.Session != nil && len(preloaded.Session.Snapshot()) == 0 {
			return nil, path, true
		}
		return preloaded.Session, path, true
	}
	loaded, err := agent.LoadSession(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, path, true
		}
		return nil, "", false
	}
	if len(loaded.Snapshot()) == 0 {
		return nil, path, true
	}
	return loaded, path, true
}

func pinnedTabSessionPath(dir, sessionPath string) (string, bool) {
	sessionPath = strings.TrimSpace(sessionPath)
	if sessionPath == "" || dir == "" {
		return "", false
	}
	path, _, err := validateSessionPath(dir, sessionPath)
	if err != nil {
		return "", false
	}
	return path, true
}

func saveTabSessionMeta(tab *WorkspaceTab, path string) error {
	if tab == nil || strings.TrimSpace(path) == "" {
		return nil
	}
	if err := agent.MutateBranchMeta(path, false, func(m *agent.BranchMeta) error {
		m.Scope = tab.Scope
		m.WorkspaceRoot = tab.WorkspaceRoot
		m.TopicID = tab.TopicID
		m.TopicTitle = tab.TopicTitle
		if model := strings.TrimSpace(tab.model); model != "" {
			m.Model = model
		}
		m.BillingSource = strings.TrimSpace(tab.billingSource)
		m.Effort = cloneStringPtr(tab.effort)
		if tab.toolApprovalModeKnown {
			mode := normalizeToolApprovalMode(tab.toolApprovalMode)
			m.ToolApprovalMode = &mode
		}
		if status := tabSceneStatus(tab); status.State == scene.Draft || status.State == scene.Locked {
			return m.SetSceneStatus(status)
		}
		return nil
	}); err != nil {
		return err
	}
	if model := strings.TrimSpace(tab.model); model != "" && strings.TrimSpace(tab.TopicID) != "" {
		if err := setTopicModel(topicTitleRoot(tab.Scope, tab.WorkspaceRoot), tab.TopicID, model); err != nil {
			return err
		}
	}
	invalidateTopicSessionIndexForPath(path)
	return nil
}

func persistSessionToolApprovalMode(path, model, billingSource string, effort *string, mode string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := agent.MutateBranchMeta(path, false, func(m *agent.BranchMeta) error {
		if model = strings.TrimSpace(model); model != "" {
			m.Model = model
		}
		m.BillingSource = strings.TrimSpace(billingSource)
		m.Effort = cloneStringPtr(effort)
		canonical := normalizeToolApprovalMode(mode)
		m.ToolApprovalMode = &canonical
		return nil
	}); err != nil {
		return err
	}
	invalidateTopicSessionIndexForPath(path)
	return nil
}

var saveSessionToolApprovalMode = persistSessionToolApprovalMode

func canonicalTabSessionPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if validPath, _, err := validateSessionPath(config.SessionDir(), path); err == nil {
		return validPath
	}
	return path
}

func (a *App) rememberTabSessionPath(tab *WorkspaceTab, path string) {
	path = canonicalTabSessionPath(path)
	if tab == nil || path == "" {
		return
	}
	a.mu.Lock()
	if current := a.tabRuntimes[tab.ID]; current == tab {
		tab.SessionPath = path
		a.saveTabsLocked()
	} else {
		tab.SessionPath = path
	}
	a.mu.Unlock()
}

func (a *App) persistTabSessionPath(tab *WorkspaceTab, path string) {
	path = canonicalTabSessionPath(path)
	if tab == nil || path == "" {
		return
	}
	if err := saveTabSessionMeta(tab, path); err != nil {
		recordDesktopBackgroundFailure("session.meta.persist", err)
	}
	a.rememberTabSessionPath(tab, path)
}

func (a *App) persistTabSessionPathForBuild(tab *WorkspaceTab, path string, generation uint64) bool {
	path = canonicalTabSessionPath(path)
	if tab == nil || path == "" || a.tabUnavailableForBuild(tab, generation) {
		return false
	}
	if err := saveTabSessionMeta(tab, path); err != nil {
		recordDesktopBackgroundFailure("session.meta.build", err)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if tab.removed || a.tabRuntimes[tab.ID] != tab || (generation != 0 && tab.buildGeneration != generation) {
		return false
	}
	tab.SessionPath = path
	a.saveTabsLocked()
	return true
}

func topicSessionMatchMatchesTarget(match topicSessionMatch, scope, workspaceRoot string) bool {
	if scope == "project" {
		return match.scope == "project" && normalizeProjectRoot(match.workspaceRoot) == normalizeProjectRoot(workspaceRoot)
	}
	return match.scope == "" || match.scope == "global"
}

func (a *App) findTopicSessionForTarget(scope, workspaceRoot, topicID string) (string, string) {
	return a.findTopicSessionForTargetByContentInDirs(a.activeProfileSessionDirs(), scope, workspaceRoot, topicID, false)
}

func (a *App) findTopicContentSessionForTarget(scope, workspaceRoot, topicID string) (string, string) {
	return a.findTopicSessionForTargetByContentInDirs(a.activeProfileSessionDirs(), scope, workspaceRoot, topicID, true)
}

func (a *App) findTopicSessionForTargetForActor(scope, workspaceRoot, topicID string, actor userctx.ActorContext) (string, string) {
	return a.findTopicSessionForTargetByContentInDirs(a.sessionDirsForActorProfile(actor), scope, workspaceRoot, topicID, false)
}

func (a *App) findTopicSessionForTargetByContent(scope, workspaceRoot, topicID string, requireContent bool) (string, string) {
	return a.findTopicSessionForTargetByContentInDirs(a.activeProfileSessionDirs(), scope, workspaceRoot, topicID, requireContent)
}

func (a *App) findTopicSessionForTargetByContentInDirs(dirs []string, scope, workspaceRoot, topicID string, requireContent bool) (string, string) {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return "", ""
	}
	var bestPath string
	var bestDir string
	var bestTime time.Time
	for _, dir := range dirs {
		for _, match := range topicSessionMatches(dir, topicID) {
			if requireContent && !sessionFileHasConversationContent(match.path) {
				continue
			}
			if !topicSessionMatchMatchesTarget(match, scope, workspaceRoot) {
				continue
			}
			if topicSessionMatchPreferred(match, bestPath, bestTime) {
				bestPath = match.path
				bestDir = dir
				bestTime = match.updatedAt
			}
		}
	}
	return bestPath, bestDir
}

func topicSessionMatchPreferred(match topicSessionMatch, bestPath string, bestTime time.Time) bool {
	return bestPath == "" || match.updatedAt.After(bestTime) ||
		(match.updatedAt.Equal(bestTime) && match.path < bestPath)
}

type topicSessionSelection struct {
	path          string
	updatedAt     time.Time
	turns         int
	scene         scene.Scene
	sceneState    scene.State
	modelRef      string
	billingSource string
}

func (s topicSessionSelection) sessionTurns() *int {
	if strings.TrimSpace(s.path) == "" {
		return nil
	}
	turns := s.turns
	return &turns
}

func (s topicSessionSelection) sceneStatus() scene.Status {
	if strings.TrimSpace(s.path) == "" {
		return scene.Status{}
	}
	return projectTreeScenePresentationStatus(scene.Status{Scene: s.scene, State: s.sceneState})
}

func projectTreeScenePresentationStatus(status scene.Status) scene.Status {
	if status.Validate() == nil && status.Scene.IsSelectable() && (status.State == scene.Draft || status.State == scene.Locked) {
		return status
	}
	return newDesktopSceneStatus()
}

func topicSessionSelectionKey(scope, workspaceRoot, topicID string) string {
	topicID = strings.TrimSpace(topicID)
	if topicID == "" {
		return ""
	}
	switch scope {
	case "project":
		return topicSummaryKey("project", normalizeProjectRoot(workspaceRoot), topicID)
	case "", "global":
		return topicSummaryKey("global", "", topicID)
	default:
		return ""
	}
}

func mergeTopicSessionSelections(dst, src map[string]topicSessionSelection) {
	for key, candidate := range src {
		current := dst[key]
		if current.path == "" || candidate.updatedAt.After(current.updatedAt) ||
			(candidate.updatedAt.Equal(current.updatedAt) && candidate.path < current.path) {
			dst[key] = candidate
		}
	}
}

// latestTopicSessionSelectionsForDir resolves every topic's concrete open
// target in one indexed pass. It intentionally includes zero-turn sessions so
// ListProjectTree and ActivateTopic select the same latest runtime.
func latestTopicSessionSelectionsForDir(dir string) (map[string]topicSessionSelection, map[string]int, error) {
	index, err := topicSessionIndexForDir(dir)
	if err != nil {
		return nil, nil, err
	}
	selected := map[string]topicSessionSelection{}
	turnsByPath := map[string]int{}
	for topicID, matches := range index.byTopic {
		for _, match := range matches {
			if agent.IsCleanupPending(match.path) {
				continue
			}
			turnsByPath[sessionRuntimeKey(match.path)] = match.turns
			key := topicSessionSelectionKey(match.scope, match.workspaceRoot, topicID)
			if key == "" {
				continue
			}
			current := selected[key]
			if topicSessionMatchPreferred(match, current.path, current.updatedAt) {
				selected[key] = topicSessionSelection{
					path:          match.path,
					updatedAt:     match.updatedAt,
					turns:         match.turns,
					scene:         match.scene,
					sceneState:    match.sceneState,
					modelRef:      match.modelRef,
					billingSource: match.billingSource,
				}
			}
		}
	}
	return selected, turnsByPath, nil
}

var loadLatestTopicSessionSelectionsForDir = latestTopicSessionSelectionsForDir

type topicSessionFileSignature struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"mod_time"`
}

type topicSessionMatch struct {
	path          string
	updatedAt     time.Time
	scope         string
	workspaceRoot string
	turns         int
	scene         scene.Scene
	sceneState    scene.State
	modelRef      string
	billingSource string
}

type topicSessionDirIndex struct {
	signature []topicSessionFileSignature
	byTopic   map[string][]topicSessionMatch
}

// sessionListCache caches ListSessions results and concrete topic targets per
// directory so ListProjectTree does not re-read every session dir on each
// render. Invalidated by emitProjectTreeChanged — any create/delete/rename
// session bumps the project tree version.
type sessionListCacheEntry struct {
	infos            []agent.SessionInfo
	titles           map[string]string
	latestSelections map[string]topicSessionSelection
	sessionTurns     map[string]int
}

type sessionListCache struct {
	mu      sync.Mutex
	byDir   map[string]sessionListCacheEntry
	version atomic.Uint64
}

func (c *sessionListCache) get(dir string) ([]agent.SessionInfo, map[string]string, map[string]topicSessionSelection, map[string]int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.byDir[dir]
	if !ok {
		return nil, nil, nil, nil, false
	}
	return e.infos, e.titles, e.latestSelections, e.sessionTurns, true
}

func (c *sessionListCache) versionToken() uint64 {
	return c.version.Load()
}

func (c *sessionListCache) put(dir string, infos []agent.SessionInfo, titles map[string]string, latestSelections map[string]topicSessionSelection, sessionTurns map[string]int, token uint64) {
	if c.version.Load() != token {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.version.Load() != token {
		return
	}
	c.byDir[dir] = sessionListCacheEntry{infos: infos, titles: titles, latestSelections: latestSelections, sessionTurns: sessionTurns}
}

func (c *sessionListCache) invalidate() {
	c.version.Add(1)
	c.mu.Lock()
	c.byDir = map[string]sessionListCacheEntry{}
	c.mu.Unlock()
}

var projectSessionCache = &sessionListCache{byDir: map[string]sessionListCacheEntry{}}

// mergeSessionInfos merges one directory's session listing into the maps used by
// ListProjectTree. The result collection loop calls it serially.
func mergeSessionInfos(dir string, infos []agent.SessionInfo, titles map[string]string, sessionInfos map[string]agent.SessionInfo, sessionTitles map[string]string, topicSummaries map[string]topicSummary) {
	for _, info := range infos {
		sessionKey := sessionRuntimeKey(info.Path)
		if sessionKey != "" {
			sessionInfos[sessionKey] = info
			sessionTitles[sessionKey] = titles[filepath.Base(info.Path)]
		}
		if strings.TrimSpace(info.TopicID) == "" {
			continue
		}
		if meta, ok, err := agent.LoadBranchMeta(info.Path); err == nil && ok && meta.IsCloudProjectBound() {
			continue
		}
		key := topicSummaryKey(info.Scope, info.WorkspaceRoot, info.TopicID)
		summary := topicSummaries[key]
		summary.turns += info.Turns
		lastActivityAt := info.LastActivityAt.UnixMilli()
		if lastActivityAt > summary.lastActivityAt {
			summary.lastActivityAt = lastActivityAt
		}
		topicSummaries[key] = summary
	}
}

var topicSessionIndexCache = struct {
	sync.Mutex
	byDir map[string]topicSessionDirIndex
}{byDir: map[string]topicSessionDirIndex{}}

func topicSessionDirKey(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return ""
	}
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return dir
}

func topicSessionDirSnapshot(dir string) ([]topicSessionFileSignature, []string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	signature := []topicSessionFileSignature{}
	sessionNames := []string{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			continue
		}
		isSession := filepath.Ext(name) == ".jsonl"
		isMeta := strings.HasSuffix(name, ".jsonl.meta")
		if !isSession && !isMeta {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		signature = append(signature, topicSessionFileSignature{
			Name:    name,
			Size:    info.Size(),
			ModTime: info.ModTime().UnixNano(),
		})
		if isSession {
			sessionNames = append(sessionNames, name)
		}
	}
	sort.Slice(signature, func(i, j int) bool {
		return signature[i].Name < signature[j].Name
	})
	sort.Strings(sessionNames)
	return signature, sessionNames, nil
}

func topicSessionSignaturesEqual(a, b []topicSessionFileSignature) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func topicSessionIndexForDir(dir string) (topicSessionDirIndex, error) {
	key := topicSessionDirKey(dir)
	if key == "" {
		return topicSessionDirIndex{}, nil
	}
	signature, sessionNames, err := topicSessionDirSnapshot(key)
	if err != nil {
		if os.IsNotExist(err) {
			return topicSessionDirIndex{}, nil
		}
		return topicSessionDirIndex{}, err
	}
	topicSessionIndexCache.Lock()
	cached, ok := topicSessionIndexCache.byDir[key]
	if ok && topicSessionSignaturesEqual(cached.signature, signature) {
		topicSessionIndexCache.Unlock()
		return cached, nil
	}
	topicSessionIndexCache.Unlock()

	index := topicSessionDirIndex{
		signature: signature,
		byTopic:   map[string][]topicSessionMatch{},
	}
	for _, name := range sessionNames {
		path := filepath.Join(key, name)
		meta, ok, err := agent.LoadBranchMeta(path)
		if err != nil || !ok {
			continue
		}
		if meta.IsCloudProjectBound() {
			continue
		}
		topicID := strings.TrimSpace(meta.TopicID)
		if topicID == "" {
			continue
		}
		turns := meta.Turns
		if turns == 0 && meta.SchemaVersion < agent.BranchMetaCountsVersion {
			_, turns = agent.SessionPreview(path)
		}
		sceneStatus, sceneErr := meta.SceneStatus()
		if sceneErr != nil {
			sceneStatus = scene.Status{}
		}
		scenePresentation := projectTreeScenePresentationStatus(sceneStatus)
		index.byTopic[topicID] = append(index.byTopic[topicID], topicSessionMatch{
			path:          path,
			updatedAt:     meta.UpdatedAt,
			scope:         meta.DefaultScope(),
			workspaceRoot: meta.WorkspaceRoot,
			modelRef:      strings.TrimSpace(meta.Model),
			billingSource: strings.TrimSpace(meta.BillingSource),
			turns:         turns,
			scene:         scenePresentation.Scene,
			sceneState:    scenePresentation.State,
		})
	}

	topicSessionIndexCache.Lock()
	topicSessionIndexCache.byDir[key] = index
	topicSessionIndexCache.Unlock()
	return index, nil
}

func topicSessionIndexHasContentTopic(index topicSessionDirIndex, topicID string) bool {
	matches := index.byTopic[strings.TrimSpace(topicID)]
	for _, match := range matches {
		if match.turns > 0 {
			return true
		}
		if sessionFileHasConversationContent(match.path) {
			return true
		}
	}
	return false
}

func topicSessionMatches(dir, topicID string) []topicSessionMatch {
	index, err := topicSessionIndexForDir(dir)
	if err != nil {
		return nil
	}
	matches := index.byTopic[strings.TrimSpace(topicID)]
	if len(matches) == 0 {
		return nil
	}
	out := make([]topicSessionMatch, 0, len(matches))
	for _, match := range matches {
		if agent.IsCleanupPending(match.path) {
			continue
		}
		out = append(out, match)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func invalidateTopicSessionIndex(dir string) {
	key := topicSessionDirKey(dir)
	if key == "" {
		return
	}
	topicSessionIndexCache.Lock()
	delete(topicSessionIndexCache.byDir, key)
	topicSessionIndexCache.Unlock()
}

func invalidateTopicSessionIndexForPath(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	invalidateTopicSessionIndex(filepath.Dir(path))
}

// findTopicSession returns the most recently updated .jsonl file whose .meta
// carries the given topicID, using a directory-level sidecar index cache.
func findTopicSession(dir, topicID string) string {
	if topicID == "" || dir == "" {
		return ""
	}
	var bestPath string
	var bestTime time.Time
	for _, match := range topicSessionMatches(dir, topicID) {
		if match.updatedAt.After(bestTime) {
			bestTime = match.updatedAt
			bestPath = match.path
		}
	}
	return bestPath
}

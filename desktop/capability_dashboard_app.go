package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	capabilityDashboardSchemaVersion = 1
	capabilityDashboardMaxPayload    = 512 << 10
	capabilityDashboardMaxWidgets    = 64
	capabilityDashboardChangedEvent  = "capability:dashboard-changed"
)

var capabilityDashboardSafeID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,79}$`)
var capabilityDashboardOperationID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,159}$`)

// CapabilityTaskBindingRequest contains logical identifiers only. The host
// derives profile and session identity; callers never provide file paths.
type CapabilityTaskBindingRequest struct {
	CapabilityID string `json:"capabilityId"`
	WorkspaceID  string `json:"workspaceId"`
	TaskID       string `json:"taskId"`
	DocumentID   string `json:"documentId"`
}

// CapabilityTaskScope is the durable owner address used by dashboard tools.
type CapabilityTaskScope struct {
	CapabilityID string `json:"capabilityId"`
	WorkspaceID  string `json:"workspaceId"`
	TaskID       string `json:"taskId"`
	DocumentID   string `json:"documentId"`
}

func cloneCapabilityTaskScope(scope CapabilityTaskScope) *CapabilityTaskScope {
	if strings.TrimSpace(scope.CapabilityID) == "" {
		return nil
	}
	copy := scope
	return &copy
}

func cloneCapabilityTaskScopeValue(scope *CapabilityTaskScope) *CapabilityTaskScope {
	if scope == nil {
		return nil
	}
	copy := *scope
	return &copy
}

type capabilitySessionBinding struct {
	SchemaVersion     int                 `json:"schemaVersion"`
	SessionHash       string              `json:"sessionHash"`
	SessionIdentity   string              `json:"sessionIdentity,omitempty"`
	SessionDocumentID string              `json:"sessionDocumentId,omitempty"`
	PackageVersion    string              `json:"packageVersion,omitempty"`
	PackageDigest     string              `json:"packageDigest,omitempty"`
	InstructionHash   string              `json:"instructionHash,omitempty"`
	Scope             CapabilityTaskScope `json:"scope"`
	UpdatedAt         string              `json:"updatedAt"`
}

type capabilityDashboardDocument struct {
	SchemaVersion       int                         `json:"schemaVersion"`
	DocumentID          string                      `json:"documentId"`
	WorkspaceID         string                      `json:"workspaceId"`
	TaskID              string                      `json:"taskId"`
	Revision            int                         `json:"revision"`
	UpdatedAt           string                      `json:"updatedAt"`
	Widgets             []capabilityDashboardWidget `json:"widgets"`
	AppliedOperationIDs []string                    `json:"appliedOperationIds"`
}

type capabilityDashboardWidget struct {
	ID           string          `json:"id"`
	Type         string          `json:"type"`
	Version      string          `json:"version"`
	Surface      string          `json:"surface"`
	Status       string          `json:"status"`
	Title        string          `json:"title"`
	Props        json.RawMessage `json:"props"`
	EvidenceRefs []string        `json:"evidenceRefs"`
	Revision     int             `json:"revision"`
}

type capabilityDashboardAction struct {
	Type           string                     `json:"type"`
	OperationID    string                     `json:"operationId"`
	BaseRevision   int                        `json:"baseRevision"`
	At             string                     `json:"at"`
	Widget         *capabilityDashboardWidget `json:"widget,omitempty"`
	WidgetID       string                     `json:"widgetId,omitempty"`
	Changes        *capabilityWidgetChanges   `json:"changes,omitempty"`
	EvidenceRefs   []string                   `json:"evidenceRefs,omitempty"`
	TargetSurface  string                     `json:"targetSurface,omitempty"`
	BeforeWidgetID string                     `json:"beforeWidgetId,omitempty"`
}

type capabilityWidgetChanges struct {
	Title  *string         `json:"title,omitempty"`
	Status *string         `json:"status,omitempty"`
	Props  json.RawMessage `json:"props,omitempty"`
}

type capabilityActiveContext struct {
	stateRoot       string
	sessionHash     string
	sessionIdentity string
}

func capabilityStableSessionIdentity(sessionPath string) string {
	sessionPath = sessionRuntimeKey(sessionPath)
	if sessionPath == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(sessionPath))
	return hex.EncodeToString(hash[:])
}

func (a *App) capabilityContextForTab(tabID, workspaceRoot, sessionPath string) (capabilityActiveContext, error) {
	tabID = strings.TrimSpace(tabID)
	if tabID == "" {
		return capabilityActiveContext{}, errors.New("session has no stable identity")
	}
	hash := sha256.Sum256([]byte(strings.TrimSpace(workspaceRoot) + "\x00tab:" + tabID))
	return capabilityActiveContext{
		stateRoot:       a.currentProfilePathResolver(strings.TrimSpace(workspaceRoot)).MemoryUserDir(),
		sessionHash:     hex.EncodeToString(hash[:]),
		sessionIdentity: capabilityStableSessionIdentity(sessionPath),
	}, nil
}

func validateCapabilitySessionBinding(binding capabilitySessionBinding) error {
	if binding.SchemaVersion != capabilityDashboardSchemaVersion {
		return errors.New("invalid capability session binding schema")
	}
	if err := validateCapabilityScope(binding.Scope); err != nil {
		return fmt.Errorf("invalid stored capability scope: %w", err)
	}
	if binding.SessionDocumentID != "" && !capabilityDashboardSafeID.MatchString(binding.SessionDocumentID) {
		return errors.New("invalid stored capability session document id")
	}
	pinCount := 0
	for _, value := range []string{binding.PackageVersion, binding.PackageDigest, binding.InstructionHash} {
		if strings.TrimSpace(value) != "" {
			pinCount++
		}
	}
	if pinCount != 0 && pinCount != 3 {
		return errors.New("incomplete capability package pin")
	}
	if pinCount == 3 && (len(binding.PackageDigest) != 64 || len(binding.InstructionHash) != 64) {
		return errors.New("invalid capability package pin")
	}
	return nil
}

func pinCapabilityPackageBinding(binding *capabilitySessionBinding) (bool, error) {
	pkg, found, err := resolveCapabilityPackage(binding.Scope.CapabilityID)
	if err != nil {
		return false, err
	}
	if !found {
		return false, errors.New("capability package is unavailable")
	}
	if binding.PackageVersion == "" && binding.PackageDigest == "" && binding.InstructionHash == "" {
		binding.PackageVersion = pkg.Manifest.Version
		binding.PackageDigest = pkg.ContentDigest
		binding.InstructionHash = pkg.InstructionHash
		return true, nil
	}
	if binding.PackageVersion != pkg.Manifest.Version || binding.PackageDigest != pkg.ContentDigest || binding.InstructionHash != pkg.InstructionHash {
		return false, errors.New("capability package pin mismatch")
	}
	return false, nil
}

func capabilitySessionDocumentID(binding capabilitySessionBinding) string {
	if capabilityDashboardSafeID.MatchString(binding.SessionDocumentID) {
		return binding.SessionDocumentID
	}
	seed := strings.TrimSpace(binding.SessionIdentity)
	if seed == "" {
		seed = strings.TrimSpace(binding.SessionHash)
	}
	if len(seed) > 16 {
		seed = seed[:16]
	}
	return "session-" + seed
}

func capabilitySessionScope(binding capabilitySessionBinding) CapabilityTaskScope {
	scope := binding.Scope
	scope.DocumentID = capabilitySessionDocumentID(binding)
	return scope
}

func completeCapabilitySessionBinding(active capabilityActiveContext, binding *capabilitySessionBinding) bool {
	changed := false
	if active.sessionIdentity != "" && binding.SessionIdentity == "" {
		binding.SessionIdentity = active.sessionIdentity
		changed = true
	}
	if binding.SessionDocumentID == "" {
		binding.SessionDocumentID = capabilitySessionDocumentID(*binding)
		changed = true
	}
	return changed
}

func (a *App) capabilityTaskScopeForSession(workspaceRoot, sessionPath string) *CapabilityTaskScope {
	identity := capabilityStableSessionIdentity(sessionPath)
	if identity == "" {
		return nil
	}
	stateRoot := a.currentProfilePathResolver(strings.TrimSpace(workspaceRoot)).MemoryUserDir()
	var binding capabilitySessionBinding
	if err := readCapabilityJSON(capabilityStableBindingPath(stateRoot, identity), &binding); err != nil {
		return nil
	}
	if binding.SessionIdentity != identity || validateCapabilitySessionBinding(binding) != nil {
		return nil
	}
	scope := capabilitySessionScope(binding)
	return cloneCapabilityTaskScopeValue(&scope)
}

// capabilityTaskScopesByTabID projects durable sidecar bindings onto live or
// restored Desktop tabs. This keeps presentation metadata out of the session
// transcript while allowing the sidebar to identify capability conversations.
func (a *App) capabilityTaskScopesByTabID() map[string]CapabilityTaskScope {
	type tabIdentity struct {
		id            string
		workspaceRoot string
		sessionPath   string
	}

	a.mu.RLock()
	identities := make([]tabIdentity, 0, len(a.tabRuntimes)+len(a.detachedSessions))
	for _, tab := range a.tabRuntimes {
		if tab != nil {
			identities = append(identities, tabIdentity{id: tab.ID, workspaceRoot: tab.WorkspaceRoot, sessionPath: tab.currentSessionPath()})
		}
	}
	for _, tab := range a.detachedSessions {
		if tab != nil {
			identities = append(identities, tabIdentity{id: tab.ID, workspaceRoot: tab.WorkspaceRoot, sessionPath: tab.currentSessionPath()})
		}
	}
	a.mu.RUnlock()

	bindings := make(map[string]CapabilityTaskScope)
	for _, identity := range identities {
		active, err := a.capabilityContextForTab(identity.id, identity.workspaceRoot, identity.sessionPath)
		if err != nil {
			continue
		}
		var binding capabilitySessionBinding
		if err := readCapabilityJSON(capabilityBindingPath(active.stateRoot, active.sessionHash), &binding); err == nil {
			if binding.SessionHash != active.sessionHash || validateCapabilitySessionBinding(binding) != nil {
				continue
			}
			if completeCapabilitySessionBinding(active, &binding) {
				binding.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
				_ = writeCapabilityJSONAtomic(capabilityBindingPath(active.stateRoot, active.sessionHash), binding)
			}
			if binding.SessionIdentity != "" {
				_ = writeCapabilityJSONAtomic(capabilityStableBindingPath(active.stateRoot, binding.SessionIdentity), binding)
			}
		} else if active.sessionIdentity != "" {
			if err := readCapabilityJSON(capabilityStableBindingPath(active.stateRoot, active.sessionIdentity), &binding); err != nil || binding.SessionIdentity != active.sessionIdentity || validateCapabilitySessionBinding(binding) != nil {
				continue
			}
		} else {
			continue
		}
		bindings[identity.id] = capabilitySessionScope(binding)
	}
	return bindings
}

// EnsureCapabilityTaskBinding creates a non-invasive sidecar binding for the
// active session. Existing session schemas and conversation persistence remain
// untouched.
func (a *App) EnsureCapabilityTaskBinding(request CapabilityTaskBindingRequest) (CapabilityTaskScope, error) {
	scope := CapabilityTaskScope(request)
	if err := validateCapabilityScope(scope); err != nil {
		return CapabilityTaskScope{}, err
	}
	active, err := a.capabilityActiveContext()
	if err != nil {
		return CapabilityTaskScope{}, err
	}

	a.capabilityDashboardMu.Lock()
	defer a.capabilityDashboardMu.Unlock()

	path := capabilityBindingPath(active.stateRoot, active.sessionHash)
	var existing capabilitySessionBinding
	if err := readCapabilityJSON(path, &existing); err == nil {
		if existing.SessionHash != active.sessionHash || validateCapabilitySessionBinding(existing) != nil {
			return CapabilityTaskScope{}, errors.New("invalid capability session binding")
		}
		if existing.Scope.CapabilityID != scope.CapabilityID {
			return CapabilityTaskScope{}, errors.New("active session is already bound to another capability")
		}
		packageChanged, err := pinCapabilityPackageBinding(&existing)
		if err != nil {
			return CapabilityTaskScope{}, err
		}
		if completeCapabilitySessionBinding(active, &existing) || packageChanged {
			existing.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
			if err := writeCapabilityJSONAtomic(path, existing); err != nil {
				return CapabilityTaskScope{}, err
			}
		}
		if existing.SessionIdentity != "" {
			if err := writeCapabilityJSONAtomic(capabilityStableBindingPath(active.stateRoot, existing.SessionIdentity), existing); err != nil {
				return CapabilityTaskScope{}, err
			}
		}
		return capabilitySessionScope(existing), nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return CapabilityTaskScope{}, err
	}
	if active.sessionIdentity != "" {
		var stable capabilitySessionBinding
		if err := readCapabilityJSON(capabilityStableBindingPath(active.stateRoot, active.sessionIdentity), &stable); err == nil {
			if stable.SessionIdentity != active.sessionIdentity || validateCapabilitySessionBinding(stable) != nil {
				return CapabilityTaskScope{}, errors.New("invalid stable capability session binding")
			}
			if stable.Scope.CapabilityID != scope.CapabilityID {
				return CapabilityTaskScope{}, errors.New("active session is already bound to another capability")
			}
			stable.SessionHash = active.sessionHash
			completeCapabilitySessionBinding(active, &stable)
			if _, err := pinCapabilityPackageBinding(&stable); err != nil {
				return CapabilityTaskScope{}, err
			}
			stable.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
			if err := writeCapabilityJSONAtomic(path, stable); err != nil {
				return CapabilityTaskScope{}, err
			}
			if err := writeCapabilityJSONAtomic(capabilityStableBindingPath(active.stateRoot, active.sessionIdentity), stable); err != nil {
				return CapabilityTaskScope{}, err
			}
			return capabilitySessionScope(stable), nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return CapabilityTaskScope{}, err
		}
	}

	binding := capabilitySessionBinding{
		SchemaVersion:   capabilityDashboardSchemaVersion,
		SessionHash:     active.sessionHash,
		SessionIdentity: active.sessionIdentity,
		Scope:           scope,
		UpdatedAt:       time.Now().UTC().Format(time.RFC3339Nano),
	}
	completeCapabilitySessionBinding(active, &binding)
	if _, err := pinCapabilityPackageBinding(&binding); err != nil {
		return CapabilityTaskScope{}, err
	}
	if active.sessionIdentity != "" {
		if err := writeCapabilityJSONAtomic(capabilityStableBindingPath(active.stateRoot, active.sessionIdentity), binding); err != nil {
			return CapabilityTaskScope{}, err
		}
	}
	if err := writeCapabilityJSONAtomic(path, binding); err != nil {
		return CapabilityTaskScope{}, err
	}
	return capabilitySessionScope(binding), nil
}

// EnsureCapabilityDashboard loads an existing task document or atomically
// installs the supplied schema-valid seed on first use.
func (a *App) EnsureCapabilityDashboard(scope CapabilityTaskScope, seedJSON string) (string, error) {
	if len(seedJSON) > capabilityDashboardMaxPayload {
		return "", errors.New("dashboard seed exceeds payload limit")
	}
	active, err := a.capabilityActiveContext()
	if err != nil {
		return "", err
	}
	a.capabilityDashboardMu.Lock()
	defer a.capabilityDashboardMu.Unlock()
	if err := validateCapabilityBinding(active, scope); err != nil {
		return "", err
	}

	path := capabilityDashboardPath(active.stateRoot, scope)
	var document capabilityDashboardDocument
	if err := readCapabilityJSON(path, &document); err == nil {
		if err := validateCapabilityDocument(document, scope); err != nil {
			return "", err
		}
		if removeCapabilityPrototypeWidgets(&document) {
			document.Revision++
			document.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
			if err := writeCapabilityJSONAtomic(path, document); err != nil {
				return "", err
			}
		}
		return marshalCapabilityDocument(document)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	if err := decodeCapabilityJSON([]byte(seedJSON), &document); err != nil {
		return "", fmt.Errorf("invalid dashboard seed: %w", err)
	}
	if err := validateCapabilityDocument(document, scope); err != nil {
		return "", err
	}
	if err := writeCapabilityJSONAtomic(path, document); err != nil {
		return "", err
	}
	return marshalCapabilityDocument(document)
}

func removeCapabilityPrototypeWidgets(document *capabilityDashboardDocument) bool {
	kept := document.Widgets[:0]
	changed := false
	for _, widget := range document.Widgets {
		prototype := strings.HasPrefix(widget.ID, "mock-")
		for _, ref := range widget.EvidenceRefs {
			if isCapabilityPrototypeEvidenceRef(ref) {
				prototype = true
				break
			}
		}
		if prototype {
			changed = true
			continue
		}
		kept = append(kept, widget)
	}
	document.Widgets = kept
	keptOperations := document.AppliedOperationIDs[:0]
	for _, operationID := range document.AppliedOperationIDs {
		if strings.HasPrefix(operationID, "seed:") || strings.HasPrefix(operationID, "mock-") || strings.HasPrefix(operationID, "ui:cite:") || strings.HasPrefix(operationID, "op:add-") || strings.HasPrefix(operationID, "op:remove-transient") || strings.HasPrefix(operationID, "op:move-coverage") || strings.HasPrefix(operationID, "op:attach-ebay") || strings.HasPrefix(operationID, "op:finalize-summary") {
			changed = true
			continue
		}
		keptOperations = append(keptOperations, operationID)
	}
	document.AppliedOperationIDs = keptOperations
	return changed
}

func isCapabilityPrototypeEvidenceRef(ref string) bool {
	return strings.HasPrefix(ref, "ev:mock") || strings.HasPrefix(ref, "ev:research:") ||
		ref == "ev:1688:supply-snapshot" || ref == "ev:1688:supply-trend-90d" || ref == "ev:ebay:active-listing-snapshot"
}

// LoadCapabilityDashboard reads the active task document after verifying the
// session binding. It never accepts a caller-controlled path.
func (a *App) LoadCapabilityDashboard(scope CapabilityTaskScope) (string, error) {
	active, err := a.capabilityActiveContext()
	if err != nil {
		return "", err
	}
	a.capabilityDashboardMu.Lock()
	defer a.capabilityDashboardMu.Unlock()
	if err := validateCapabilityBinding(active, scope); err != nil {
		return "", err
	}
	var document capabilityDashboardDocument
	if err := readCapabilityJSON(capabilityDashboardPath(active.stateRoot, scope), &document); err != nil {
		return "", err
	}
	if err := validateCapabilityDocument(document, scope); err != nil {
		return "", err
	}
	return marshalCapabilityDocument(document)
}

// ApplyCapabilityDashboardAction is the controlled write surface intended for
// future MCP tools. The model requests an action; the host owns validation and
// persistence.
func (a *App) ApplyCapabilityDashboardAction(scope CapabilityTaskScope, actionJSON string) (string, error) {
	if len(actionJSON) > capabilityDashboardMaxPayload {
		return "", errors.New("dashboard action exceeds payload limit")
	}
	active, err := a.capabilityActiveContext()
	if err != nil {
		return "", err
	}
	a.capabilityDashboardMu.Lock()
	if err := validateCapabilityBinding(active, scope); err != nil {
		a.capabilityDashboardMu.Unlock()
		return "", err
	}

	var action capabilityDashboardAction
	if err := decodeCapabilityJSON([]byte(actionJSON), &action); err != nil {
		a.capabilityDashboardMu.Unlock()
		return "", fmt.Errorf("invalid dashboard action: %w", err)
	}
	result, changed, err := mutateCapabilityDashboardAtRoot(active.stateRoot, scope, action)
	a.capabilityDashboardMu.Unlock()
	if err != nil {
		return "", err
	}
	if changed && a.ctx != nil {
		runtime.EventsEmit(a.ctx, capabilityDashboardChangedEvent, scope)
	}
	return result, nil
}

func mutateCapabilityDashboardAtRoot(stateRoot string, scope CapabilityTaskScope, action capabilityDashboardAction) (string, bool, error) {
	path := capabilityDashboardPath(stateRoot, scope)
	var document capabilityDashboardDocument
	if err := readCapabilityJSON(path, &document); err != nil {
		return "", false, err
	}
	if err := validateCapabilityDocument(document, scope); err != nil {
		return "", false, err
	}
	changed, err := mutateCapabilityDocument(&document, action)
	if err != nil {
		return "", false, err
	}
	if err := validateCapabilityDocument(document, scope); err != nil {
		return "", false, err
	}
	if changed {
		if err := writeCapabilityJSONAtomic(path, document); err != nil {
			return "", false, err
		}
	}
	result, err := marshalCapabilityDocument(document)
	return result, changed, err
}

func (a *App) capabilityActiveContext() (capabilityActiveContext, error) {
	a.mu.RLock()
	activeTabID := a.cachedActiveTabID()
	if activeTabID == "" || a.tabRuntimes[activeTabID] == nil {
		a.mu.RUnlock()
		return capabilityActiveContext{}, errors.New("no active session")
	}
	tab := a.tabRuntimes[activeTabID]
	workspaceRoot := strings.TrimSpace(tab.WorkspaceRoot)
	tabID := strings.TrimSpace(tab.ID)
	sessionPath := tab.currentSessionPath()
	a.mu.RUnlock()

	// Keep the tab binding for the active controller contract, and additionally
	// derive a stable session identity for historical sidebar projection.
	return a.capabilityContextForTab(tabID, workspaceRoot, sessionPath)
}

func validateCapabilityBinding(active capabilityActiveContext, scope CapabilityTaskScope) error {
	if err := validateCapabilityScope(scope); err != nil {
		return err
	}
	var binding capabilitySessionBinding
	if err := readCapabilityJSON(capabilityBindingPath(active.stateRoot, active.sessionHash), &binding); err != nil {
		return fmt.Errorf("capability session is not bound: %w", err)
	}
	sessionScope := capabilitySessionScope(binding)
	if binding.SchemaVersion != capabilityDashboardSchemaVersion || binding.SessionHash != active.sessionHash || (binding.Scope != scope && sessionScope != scope) {
		return errors.New("capability scope is not bound to the active session")
	}
	return nil
}

func validateCapabilityScope(scope CapabilityTaskScope) error {
	for label, value := range map[string]string{
		"capabilityId": scope.CapabilityID,
		"workspaceId":  scope.WorkspaceID,
		"taskId":       scope.TaskID,
		"documentId":   scope.DocumentID,
	} {
		if !capabilityDashboardSafeID.MatchString(value) {
			return fmt.Errorf("invalid %s", label)
		}
	}
	return nil
}

func validateCapabilityDocument(document capabilityDashboardDocument, scope CapabilityTaskScope) error {
	if document.SchemaVersion != capabilityDashboardSchemaVersion {
		return errors.New("unsupported dashboard schema version")
	}
	if document.WorkspaceID != scope.WorkspaceID || document.TaskID != scope.TaskID || document.DocumentID != scope.DocumentID {
		return errors.New("dashboard document does not match bound scope")
	}
	if document.Revision < 0 || len(document.Widgets) > capabilityDashboardMaxWidgets {
		return errors.New("invalid dashboard document limits")
	}
	seen := make(map[string]struct{}, len(document.Widgets))
	for _, widget := range document.Widgets {
		if _, exists := seen[widget.ID]; exists {
			return fmt.Errorf("duplicate widget id: %s", widget.ID)
		}
		seen[widget.ID] = struct{}{}
		if err := validateCapabilityWidget(widget); err != nil {
			return err
		}
	}
	return nil
}

func validateCapabilityWidget(widget capabilityDashboardWidget) error {
	if !capabilityDashboardSafeID.MatchString(widget.ID) {
		return errors.New("invalid widget id")
	}
	if widget.Version != "1.0.0" || widget.Revision < 1 || strings.TrimSpace(widget.Title) == "" {
		return fmt.Errorf("invalid widget metadata: %s", widget.ID)
	}
	surfaces := map[string]map[string]bool{
		"research.summary": {"conversation-inline": true, "research-canvas": true},
		"trend.chart":      {"research-canvas": true},
		"comparison.bars":  {"research-canvas": true},
		"source.coverage":  {"conversation-inline": true, "research-canvas": true},
		"product.catalog":  {"research-canvas": true},
		"chart.bar":        {"research-canvas": true},
		"chart.donut":      {"research-canvas": true},
		"chart.histogram":  {"research-canvas": true},
	}
	allowed, known := surfaces[widget.Type]
	if !known {
		return fmt.Errorf("unknown widget type: %s", widget.Type)
	}
	if !allowed[widget.Surface] {
		return fmt.Errorf("widget surface is unsupported: %s on %s", widget.Type, widget.Surface)
	}
	validStatuses := map[string]bool{"queued": true, "loading": true, "partial": true, "ready": true, "stale": true, "error": true}
	if !validStatuses[widget.Status] {
		return fmt.Errorf("invalid widget status: %s", widget.Status)
	}
	var props struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(widget.Props, &props); err != nil || props.Kind != widget.Type {
		return fmt.Errorf("widget props kind does not match %s", widget.Type)
	}
	if widget.Type == "research.summary" {
		if err := validateResearchSummaryProps(widget.Props); err != nil {
			return fmt.Errorf("invalid research.summary props: %w", err)
		}
	}
	return nil
}

func validateResearchSummaryProps(raw json.RawMessage) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	for _, name := range []string{"kind", "eyebrow", "conclusion", "confidence", "summary", "metrics", "findings"} {
		if _, ok := fields[name]; !ok {
			return fmt.Errorf("missing %s", name)
		}
	}
	var props struct {
		Kind       string  `json:"kind"`
		Eyebrow    string  `json:"eyebrow"`
		Conclusion string  `json:"conclusion"`
		Confidence float64 `json:"confidence"`
		Summary    string  `json:"summary"`
		Metrics    []struct {
			Label  string `json:"label"`
			Value  string `json:"value"`
			Detail string `json:"detail"`
			Tone   string `json:"tone"`
		} `json:"metrics"`
		Findings []struct {
			Label string `json:"label"`
			Text  string `json:"text"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(raw, &props); err != nil {
		return err
	}
	if props.Kind != "research.summary" || strings.TrimSpace(props.Eyebrow) == "" || strings.TrimSpace(props.Conclusion) == "" || strings.TrimSpace(props.Summary) == "" {
		return errors.New("required text is empty")
	}
	if props.Confidence < 0 || props.Confidence > 100 {
		return errors.New("confidence must be between 0 and 100")
	}
	for index, metric := range props.Metrics {
		if strings.TrimSpace(metric.Label) == "" || strings.TrimSpace(metric.Value) == "" || strings.TrimSpace(metric.Detail) == "" {
			return fmt.Errorf("metric %d is incomplete", index)
		}
		if metric.Tone != "" && metric.Tone != "positive" && metric.Tone != "neutral" && metric.Tone != "warning" {
			return fmt.Errorf("metric %d has invalid tone", index)
		}
	}
	for index, finding := range props.Findings {
		if strings.TrimSpace(finding.Label) == "" || strings.TrimSpace(finding.Text) == "" {
			return fmt.Errorf("finding %d is incomplete", index)
		}
	}
	return nil
}

func applyCapabilityAction(document *capabilityDashboardDocument, action capabilityDashboardAction) error {
	if !capabilityDashboardOperationID.MatchString(action.OperationID) {
		return errors.New("invalid dashboard operationId")
	}
	if _, err := time.Parse(time.RFC3339Nano, action.At); err != nil {
		return errors.New("invalid dashboard action timestamp")
	}

	if action.Type == "dashboard.add_widget" {
		if action.Widget == nil {
			return errors.New("add_widget requires widget")
		}
		widget := *action.Widget
		widget.Revision = 1
		if widget.EvidenceRefs == nil {
			widget.EvidenceRefs = []string{}
		}
		if err := validateCapabilityWidget(widget); err != nil {
			return err
		}
		if findCapabilityWidget(document.Widgets, widget.ID) >= 0 {
			return fmt.Errorf("duplicate widget id: %s", widget.ID)
		}
		document.Widgets = append(document.Widgets, widget)
		commitCapabilityAction(document, action)
		return nil
	}

	targetIndex := findCapabilityWidget(document.Widgets, action.WidgetID)
	if targetIndex < 0 {
		return fmt.Errorf("widget not found: %s", action.WidgetID)
	}
	switch action.Type {
	case "dashboard.remove_widget":
		document.Widgets = append(document.Widgets[:targetIndex], document.Widgets[targetIndex+1:]...)
	case "dashboard.update_widget":
		if action.Changes == nil {
			return errors.New("update_widget requires changes")
		}
		widget := document.Widgets[targetIndex]
		if action.Changes.Title != nil {
			widget.Title = *action.Changes.Title
		}
		if action.Changes.Status != nil {
			widget.Status = *action.Changes.Status
		}
		if len(action.Changes.Props) > 0 {
			widget.Props = append(json.RawMessage(nil), action.Changes.Props...)
		}
		widget.Revision++
		if err := validateCapabilityWidget(widget); err != nil {
			return err
		}
		document.Widgets[targetIndex] = widget
	case "dashboard.attach_evidence":
		widget := document.Widgets[targetIndex]
		for _, evidenceRef := range action.EvidenceRefs {
			if strings.TrimSpace(evidenceRef) == "" || len(evidenceRef) > 240 {
				return errors.New("invalid evidence reference")
			}
			if !containsCapabilityOperation(widget.EvidenceRefs, evidenceRef) {
				widget.EvidenceRefs = append(widget.EvidenceRefs, evidenceRef)
			}
		}
		widget.Revision++
		document.Widgets[targetIndex] = widget
	case "dashboard.move_widget":
		widget := document.Widgets[targetIndex]
		widget.Surface = action.TargetSurface
		widget.Revision++
		if err := validateCapabilityWidget(widget); err != nil {
			return err
		}
		remaining := append([]capabilityDashboardWidget(nil), document.Widgets[:targetIndex]...)
		remaining = append(remaining, document.Widgets[targetIndex+1:]...)
		before := findCapabilityWidget(remaining, action.BeforeWidgetID)
		if before < 0 {
			remaining = append(remaining, widget)
		} else {
			remaining = append(remaining, capabilityDashboardWidget{})
			copy(remaining[before+1:], remaining[before:])
			remaining[before] = widget
		}
		document.Widgets = remaining
	default:
		return fmt.Errorf("unsupported dashboard action: %s", action.Type)
	}
	commitCapabilityAction(document, action)
	return nil
}

func mutateCapabilityDocument(document *capabilityDashboardDocument, action capabilityDashboardAction) (bool, error) {
	if containsCapabilityOperation(document.AppliedOperationIDs, action.OperationID) {
		return false, nil
	}
	if action.BaseRevision != document.Revision {
		return false, fmt.Errorf("dashboard revision conflict: expected %d, received %d", document.Revision, action.BaseRevision)
	}
	if err := applyCapabilityAction(document, action); err != nil {
		return false, err
	}
	return true, nil
}

func commitCapabilityAction(document *capabilityDashboardDocument, action capabilityDashboardAction) {
	document.Revision++
	document.UpdatedAt = action.At
	document.AppliedOperationIDs = append(document.AppliedOperationIDs, action.OperationID)
	if len(document.AppliedOperationIDs) > 100 {
		document.AppliedOperationIDs = append([]string(nil), document.AppliedOperationIDs[len(document.AppliedOperationIDs)-100:]...)
	}
}

func findCapabilityWidget(widgets []capabilityDashboardWidget, id string) int {
	for index := range widgets {
		if widgets[index].ID == id {
			return index
		}
	}
	return -1
}

func containsCapabilityOperation(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func capabilityBindingPath(root, sessionHash string) string {
	return filepath.Join(root, "superpowers", "bindings", sessionHash+".json")
}

func capabilityStableBindingPath(root, sessionIdentity string) string {
	return filepath.Join(root, "superpowers", "session-bindings", sessionIdentity+".json")
}

func capabilityDashboardPath(root string, scope CapabilityTaskScope) string {
	return filepath.Join(root, "superpowers", "runtime", scope.WorkspaceID, "tasks", scope.TaskID, "dashboards", scope.DocumentID+".json")
}

func readCapabilityJSON(path string, target any) error {
	payload, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(payload) > capabilityDashboardMaxPayload {
		return errors.New("capability JSON exceeds payload limit")
	}
	return decodeCapabilityJSON(payload, target)
}

func decodeCapabilityJSON(payload []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("capability JSON contains trailing data")
	}
	return nil
}

func writeCapabilityJSONAtomic(path string, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if len(payload) > capabilityDashboardMaxPayload {
		return errors.New("capability JSON exceeds payload limit")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(dir, ".capability-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(payload); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func marshalCapabilityDocument(document capabilityDashboardDocument) (string, error) {
	payload, err := json.Marshal(document)
	return string(payload), err
}

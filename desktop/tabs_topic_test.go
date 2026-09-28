package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"

	"kwu/internal/agent"
	"kwu/internal/cloud"
	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/modelbilling"
	"kwu/internal/project"
	"kwu/internal/provider"
	"kwu/internal/scene"
	"kwu/internal/userctx"
)

func TestTopicModelMatchesBillingSource(t *testing.T) {
	tests := []struct {
		name   string
		model  string
		source modelbilling.BillingSource
		want   bool
	}{
		{name: "legacy source-less", model: "custom/chat", want: true},
		{name: "official subscription", model: "kwu/claude-fable-5", source: modelbilling.BillingSubscriptionPool, want: true},
		{name: "custom BYOK", model: "custom/chat", source: modelbilling.BillingNewAPIByok, want: true},
		{name: "custom cannot use subscription", model: "custom/chat", source: modelbilling.BillingSubscriptionPool},
		{name: "official cannot use BYOK", model: "kwu/claude-fable-5", source: modelbilling.BillingNewAPIByok},
		{name: "invalid model ref", model: "chat", source: modelbilling.BillingNewAPIByok},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := topicModelMatchesBillingSource(tt.model, tt.source); got != tt.want {
				t.Fatalf("topicModelMatchesBillingSource(%q, %q) = %v, want %v", tt.model, tt.source, got, tt.want)
			}
		})
	}
}

func waitForTabReady(t *testing.T, app *App, tabID string) *WorkspaceTab {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		app.mu.RLock()
		tab := app.tabRuntimes[tabID]
		ready := tab != nil && tab.Ready
		startupErr := ""
		if tab != nil {
			startupErr = tab.StartupErr
		}
		app.mu.RUnlock()
		if tab == nil {
			t.Fatalf("tab %q was not found", tabID)
		}
		if ready {
			if startupErr != "" {
				t.Fatalf("tab %q startup error: %s", tabID, startupErr)
			}
			if tab.Ctrl != nil {
				t.Cleanup(func() { tab.Ctrl.Close() })
			}
			return tab
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("tab %q was not ready before timeout", tabID)
	return nil
}

func writeTopicSession(t *testing.T, dir, name, topicID, topicTitle, workspaceRoot string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(`{"role":"user","content":"hello"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}
	if err := agent.SaveBranchMeta(path, agent.BranchMeta{
		CreatedAt:     time.Now().Add(-time.Minute),
		UpdatedAt:     time.Now(),
		Scope:         "project",
		WorkspaceRoot: workspaceRoot,
		TopicID:       topicID,
		TopicTitle:    topicTitle,
		Turns:         1,
		SchemaVersion: agent.BranchMetaCountsVersion,
	}); err != nil {
		t.Fatalf("save branch meta: %v", err)
	}
	return path
}

func writeTopicSessionWithPrompt(t *testing.T, dir, name, topicID, topicTitle, workspaceRoot, prompt string, updatedAt time.Time) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(`{"role":"user","content":`+strconv.Quote(prompt)+`}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}
	scope := "global"
	if strings.TrimSpace(workspaceRoot) != "" {
		scope = "project"
	}
	if err := agent.SaveBranchMetaPreserveUpdated(path, agent.BranchMeta{
		CreatedAt:     updatedAt.Add(-time.Minute),
		UpdatedAt:     updatedAt,
		Scope:         scope,
		WorkspaceRoot: workspaceRoot,
		TopicID:       topicID,
		TopicTitle:    topicTitle,
		Turns:         1,
		SchemaVersion: agent.BranchMetaCountsVersion,
	}); err != nil {
		t.Fatalf("save branch meta: %v", err)
	}
	return path
}

func writeEmptyTopicSession(t *testing.T, dir, name, topicID, topicTitle, workspaceRoot string, updatedAt time.Time) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("write empty session: %v", err)
	}
	scope := "global"
	if strings.TrimSpace(workspaceRoot) != "" {
		scope = "project"
	}
	if err := agent.SaveBranchMetaPreserveUpdated(path, agent.BranchMeta{
		CreatedAt:     updatedAt.Add(-time.Minute),
		UpdatedAt:     updatedAt,
		Scope:         scope,
		WorkspaceRoot: workspaceRoot,
		TopicID:       topicID,
		TopicTitle:    topicTitle,
		Turns:         0,
		SchemaVersion: agent.BranchMetaCountsVersion,
	}); err != nil {
		t.Fatalf("save empty branch meta: %v", err)
	}
	return path
}

func setTopicSessionScene(t *testing.T, path string, status scene.Status) {
	t.Helper()
	meta, ok, err := agent.LoadBranchMeta(path)
	if err != nil || !ok {
		t.Fatalf("load branch meta for scene: ok=%v err=%v", ok, err)
	}
	if err := meta.SetSceneStatus(status); err != nil {
		t.Fatalf("set branch scene status: %v", err)
	}
	if err := agent.SaveBranchMetaPreserveUpdated(path, meta); err != nil {
		t.Fatalf("save branch scene status: %v", err)
	}
}

func setTopicSessionRawScene(t *testing.T, path, sceneValue, stateValue string) {
	t.Helper()
	meta, ok, err := agent.LoadBranchMeta(path)
	if err != nil || !ok {
		t.Fatalf("load branch meta for raw scene: ok=%v err=%v", ok, err)
	}
	meta.Scene = sceneValue
	meta.SceneState = stateValue
	if err := agent.SaveBranchMetaPreserveUpdated(path, meta); err != nil {
		t.Fatalf("save raw branch scene: %v", err)
	}
}

func writeLegacySession(t *testing.T, dir, name, prompt string, modTime time.Time) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(`{"role":"user","content":`+strconv.Quote(prompt)+`}`+"\n"), 0o644); err != nil {
		t.Fatalf("write legacy session: %v", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("chtimes legacy session: %v", err)
	}
	return path
}

func writeLegacyEventSession(t *testing.T, dir, name, prompt, reply string, modTime time.Time) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir legacy sessions: %v", err)
	}
	path := filepath.Join(dir, name)
	body := `{"type":"user.message","id":1,"ts":"t","turn":0,"text":` + strconv.Quote(prompt) + `}` + "\n" +
		`{"type":"model.final","id":2,"ts":"t","turn":0,"content":` + strconv.Quote(reply) + `,"toolCalls":[],"usage":{},"costUsd":0}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write legacy event session: %v", err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("chtimes legacy event session: %v", err)
	}
	return path
}

func newProfileProjectTopicApp(t *testing.T, projectRoot, profileID string) (*App, string) {
	t.Helper()
	actor := userctx.Default(projectRoot)
	actor.ProfileID = profileID
	dir := config.NewProfilePathResolver(actor).ProjectSessionDir(projectRoot)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir profile session dir: %v", err)
	}
	tabID := "active-" + profileID
	ctrl := control.New(control.Options{
		SessionDir:    dir,
		SessionPath:   filepath.Join(dir, tabID+".jsonl"),
		Label:         profileID,
		Actor:         actor,
		WorkspaceRoot: projectRoot,
	})
	t.Cleanup(ctrl.Close)
	return &App{
		tabRuntimes: map[string]*WorkspaceTab{
			tabID: {ID: tabID, Scope: "project", WorkspaceRoot: projectRoot, Ctrl: ctrl, Ready: true, actor: actor},
		},
		tabOrder:    []string{tabID},
		activeTabID: tabID,
	}, dir
}

func TestSessionListCacheRefillsAfterInvalidate(t *testing.T) {
	cache := &sessionListCache{byDir: map[string]sessionListCacheEntry{}}
	dir := t.TempDir()
	first := []agent.SessionInfo{{Path: filepath.Join(dir, "first.jsonl")}}
	second := []agent.SessionInfo{{Path: filepath.Join(dir, "second.jsonl")}}

	token := cache.versionToken()
	firstLatest := map[string]topicSessionSelection{"global::first": {path: first[0].Path}}
	firstTurns := map[string]int{first[0].Path: 1}
	cache.put(dir, first, map[string]string{"first.jsonl": "First"}, firstLatest, firstTurns, token)
	if infos, titles, latest, turns, ok := cache.get(dir); !ok || len(infos) != 1 || filepath.Base(infos[0].Path) != "first.jsonl" || titles["first.jsonl"] != "First" || latest["global::first"].path != first[0].Path || turns[first[0].Path] != 1 {
		t.Fatalf("initial cache entry = %+v, %+v, %+v, %v", infos, titles, latest, ok)
	}

	cache.invalidate()
	if _, _, _, _, ok := cache.get(dir); ok {
		t.Fatalf("cache entry survived invalidate")
	}
	cache.put(dir, first, map[string]string{"first.jsonl": "stale"}, firstLatest, firstTurns, token)
	if _, _, _, _, ok := cache.get(dir); ok {
		t.Fatalf("stale token repopulated cache after invalidate")
	}

	token = cache.versionToken()
	secondLatest := map[string]topicSessionSelection{"global::second": {path: second[0].Path}}
	secondTurns := map[string]int{second[0].Path: 2}
	cache.put(dir, second, map[string]string{"second.jsonl": "Second"}, secondLatest, secondTurns, token)
	if infos, titles, latest, turns, ok := cache.get(dir); !ok || len(infos) != 1 || filepath.Base(infos[0].Path) != "second.jsonl" || titles["second.jsonl"] != "Second" || latest["global::second"].path != second[0].Path || turns[second[0].Path] != 2 {
		t.Fatalf("refilled cache entry = %+v, %+v, %+v, %v", infos, titles, latest, ok)
	}
}

func TestRenameSessionInvalidatesProjectTreeCache(t *testing.T) {
	isolateDesktopUserDirs(t)
	oldProjectCache := projectSessionCache
	projectSessionCache = &sessionListCache{byDir: map[string]sessionListCacheEntry{}}
	t.Cleanup(func() {
		projectSessionCache = oldProjectCache
	})

	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "rename-me.jsonl")
	if err := os.WriteFile(sessionPath, []byte(`{"role":"user","content":"hello"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}
	ctrl := control.New(control.Options{SessionDir: dir, SessionPath: sessionPath, Label: "test"})
	defer ctrl.Close()
	app := NewApp()
	app.setTestCtrl(ctrl, "")

	token := projectSessionCache.versionToken()
	projectSessionCache.put(dir, []agent.SessionInfo{{Path: sessionPath}}, map[string]string{"rename-me.jsonl": "old"}, nil, nil, token)
	if _, _, _, _, ok := projectSessionCache.get(dir); !ok {
		t.Fatalf("expected primed project tree cache")
	}
	if err := app.RenameSession(sessionPath, "new title"); err != nil {
		t.Fatalf("RenameSession: %v", err)
	}
	if _, _, _, _, ok := projectSessionCache.get(dir); ok {
		t.Fatalf("RenameSession should invalidate project tree cache")
	}
}

func TestTopicMetadataUpdatesPreserveExistingEntriesWhenTimedReadSlotsFull(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	if err := saveTopicTitles(projectRoot, map[string]string{"old": "Old"}); err != nil {
		t.Fatalf("save old title: %v", err)
	}
	if err := saveTopicTitleSources(projectRoot, map[string]string{"old": topicTitleSourceManual}); err != nil {
		t.Fatalf("save old source: %v", err)
	}
	if err := saveTopicCreatedAts(projectRoot, map[string]int64{"old": 100}); err != nil {
		t.Fatalf("save old created-at: %v", err)
	}

	release := occupyReadFileWithTimeoutSlots(t)
	if err := setTopicTitleWithSource(projectRoot, "new", "New", topicTitleSourceAuto); err != nil {
		t.Fatalf("setTopicTitleWithSource: %v", err)
	}
	if err := setTopicCreatedAt(projectRoot, "new", 200); err != nil {
		t.Fatalf("setTopicCreatedAt: %v", err)
	}
	release()

	titles := loadTopicTitles(projectRoot)
	if got := titles["old"]; got != "Old" {
		t.Fatalf("old title = %q, want Old (all titles: %v)", got, titles)
	}
	if got := titles["new"]; got != "New" {
		t.Fatalf("new title = %q, want New (all titles: %v)", got, titles)
	}
	sources := loadTopicTitleSources(projectRoot)
	if got := sources["old"]; got != topicTitleSourceManual {
		t.Fatalf("old source = %q, want %q (all sources: %v)", got, topicTitleSourceManual, sources)
	}
	if got := sources["new"]; got != topicTitleSourceAuto {
		t.Fatalf("new source = %q, want %q (all sources: %v)", got, topicTitleSourceAuto, sources)
	}
	created := loadTopicCreatedAts(projectRoot)
	if got := created["old"]; got != 100 {
		t.Fatalf("old created-at = %d, want 100 (all created: %v)", got, created)
	}
	if got := created["new"]; got != 200 {
		t.Fatalf("new created-at = %d, want 200 (all created: %v)", got, created)
	}
}

func TestDeleteTopicKeepsSessionHistory(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topicID := "topic_keep_history"
	if err := addProject(projectRoot, ""); err != nil {
		t.Fatalf("add project: %v", err)
	}
	if err := setTopicTitle(projectRoot, topicID, "Keep history"); err != nil {
		t.Fatalf("set topic title: %v", err)
	}
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	sessionPath := writeTopicSession(t, dir, "keep.jsonl", topicID, "Keep history", projectRoot)

	if err := NewApp().DeleteTopic(topicID); err != nil {
		t.Fatalf("delete topic: %v", err)
	}
	if _, err := os.Stat(sessionPath); err != nil {
		t.Fatalf("delete topic should keep session history: %v", err)
	}
	if got := loadTopicTitle(projectRoot, topicID); got != "" {
		t.Fatalf("topic title should be removed, got %q", got)
	}
}

func TestSetTopicPinnedOrdersProjectTopics(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	if err := addProject(projectRoot, ""); err != nil {
		t.Fatalf("add project: %v", err)
	}
	if err := setTopicTitle(projectRoot, "topic_a", "Alpha"); err != nil {
		t.Fatalf("set topic a title: %v", err)
	}
	if err := setTopicTitle(projectRoot, "topic_b", "Beta"); err != nil {
		t.Fatalf("set topic b title: %v", err)
	}
	app := NewApp()
	nodes := app.ListProjectTree()
	if got := []string{nodes[0].Children[0].TopicID, nodes[0].Children[1].TopicID}; got[0] != "topic_a" || got[1] != "topic_b" {
		t.Fatalf("initial topic order = %v, want [topic_a topic_b]", got)
	}

	if err := app.SetTopicPinned("topic_b", true); err != nil {
		t.Fatalf("pin topic: %v", err)
	}
	nodes = app.ListProjectTree()
	if got := []string{nodes[0].Children[0].TopicID, nodes[0].Children[1].TopicID}; got[0] != "topic_b" || got[1] != "topic_a" {
		t.Fatalf("pinned topic order = %v, want [topic_b topic_a]", got)
	}
	if !nodes[0].Children[0].Pinned {
		t.Fatalf("pinned topic should expose pinned=true")
	}

	if err := app.SetTopicPinned("topic_b", false); err != nil {
		t.Fatalf("unpin topic: %v", err)
	}
	nodes = app.ListProjectTree()
	if nodes[0].Children[0].Pinned || nodes[0].Children[1].Pinned {
		t.Fatalf("unpin should clear pinned flags: %#v", nodes[0].Children)
	}
}

func TestSetProjectPinnedOrdersProjectFolders(t *testing.T) {
	isolateDesktopUserDirs(t)

	first := t.TempDir()
	second := t.TempDir()
	third := t.TempDir()
	if err := addProject(first, "First"); err != nil {
		t.Fatalf("add first project: %v", err)
	}
	if err := addProject(second, "Second"); err != nil {
		t.Fatalf("add second project: %v", err)
	}
	if err := addProject(third, "Third"); err != nil {
		t.Fatalf("add third project: %v", err)
	}

	app := NewApp()
	if err := app.ReorderProjects([]string{third, first, second}); err != nil {
		t.Fatalf("ReorderProjects: %v", err)
	}
	if err := app.SetProjectPinned(second, true); err != nil {
		t.Fatalf("pin project: %v", err)
	}
	nodes := app.ListProjectTree()
	if got := []string{nodes[0].Root, nodes[1].Root, nodes[2].Root}; got[0] != second || got[1] != third || got[2] != first {
		t.Fatalf("pinned project order = %v, want %v", got, []string{second, third, first})
	}
	if !nodes[0].Pinned {
		t.Fatalf("pinned project should expose pinned=true")
	}

	if err := app.SetProjectPinned(second, false); err != nil {
		t.Fatalf("unpin project: %v", err)
	}
	nodes = app.ListProjectTree()
	if got := []string{nodes[0].Root, nodes[1].Root, nodes[2].Root}; got[0] != third || got[1] != first || got[2] != second {
		t.Fatalf("unpinned project order = %v, want %v", got, []string{third, first, second})
	}
	if nodes[0].Pinned || nodes[1].Pinned || nodes[2].Pinned {
		t.Fatalf("unpin should clear pinned flags: %#v", nodes)
	}
}

func TestDeleteTopicClearsPinnedTopic(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	if err := addProject(projectRoot, ""); err != nil {
		t.Fatalf("add project: %v", err)
	}
	if err := setTopicTitle(projectRoot, "topic_pinned_delete", "Pinned"); err != nil {
		t.Fatalf("set topic title: %v", err)
	}
	app := NewApp()
	if err := app.SetTopicPinned("topic_pinned_delete", true); err != nil {
		t.Fatalf("pin topic: %v", err)
	}
	if err := app.DeleteTopic("topic_pinned_delete"); err != nil {
		t.Fatalf("delete topic: %v", err)
	}
	projects := loadProjectsFile().Projects
	if len(projects) != 1 {
		t.Fatalf("projects len = %d, want 1", len(projects))
	}
	if got := projects[0].PinnedTopics; len(got) != 0 {
		t.Fatalf("pinned topics after delete = %v, want empty", got)
	}
}

func TestRenameProjectUpdatesSidebarTitle(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	if err := addProject(projectRoot, ""); err != nil {
		t.Fatalf("add project: %v", err)
	}
	if err := NewApp().RenameProject(projectRoot, "Client API"); err != nil {
		t.Fatalf("rename project: %v", err)
	}

	nodes := NewApp().ListProjectTree()
	if len(nodes) != 1 {
		t.Fatalf("project tree len = %d, want 1", len(nodes))
	}
	if got := nodes[0].Label; got != "Client API" {
		t.Fatalf("project label = %q, want Client API", got)
	}

	if err := NewApp().RenameProject(projectRoot, ""); err != nil {
		t.Fatalf("clear project title: %v", err)
	}
	nodes = NewApp().ListProjectTree()
	if got, want := nodes[0].Label, filepath.Base(projectRoot); got != want {
		t.Fatalf("cleared project label = %q, want %q", got, want)
	}
}

func TestListWorkspacesUsesProjectRegistryTitles(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	if err := addProject(projectRoot, "Client API"); err != nil {
		t.Fatalf("add project: %v", err)
	}

	workspaces := NewApp().ListWorkspaces()
	if len(workspaces) != 1 {
		t.Fatalf("workspaces len = %d, want 1: %+v", len(workspaces), workspaces)
	}
	if got := workspaces[0].Path; got != projectRoot {
		t.Fatalf("workspace path = %q, want %q", got, projectRoot)
	}
	if got := workspaces[0].Name; got != "Client API" {
		t.Fatalf("workspace name = %q, want Client API", got)
	}
}

func TestListWorkspacesMigratesLegacyWorkspaceList(t *testing.T) {
	isolateDesktopUserDirs(t)

	legacyRoot := t.TempDir()
	rememberWorkspace(legacyRoot)

	workspaces := NewApp().ListWorkspaces()
	if len(workspaces) != 1 {
		t.Fatalf("workspaces len = %d, want 1: %+v", len(workspaces), workspaces)
	}
	if got := workspaces[0].Path; got != legacyRoot {
		t.Fatalf("workspace path = %q, want %q", got, legacyRoot)
	}
	projects := loadProjectsFile().Projects
	if len(projects) != 1 || projects[0].Root != legacyRoot {
		t.Fatalf("legacy workspace was not migrated into projects: %+v", projects)
	}
}

func TestLegacySessionsMigrateIntoGlobalTopics(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	older := writeLegacySession(t, dir, "older.jsonl", "older imported prompt", time.Now().Add(-2*time.Hour))
	newer := writeLegacySession(t, dir, "newer.jsonl", "newer imported prompt", time.Now().Add(-time.Hour))

	nodes := NewApp().ListProjectTree()
	if len(nodes) != 1 || nodes[0].Kind != "global_folder" {
		t.Fatalf("project tree = %#v, want global folder", nodes)
	}
	if got := len(nodes[0].Children); got != 2 {
		t.Fatalf("global migrated topics = %d, want 2: %#v", got, nodes[0].Children)
	}
	if got, want := nodes[0].Children[0].TopicID, legacySessionTopicID(newer); got != want {
		t.Fatalf("newest topic first = %q, want %q", got, want)
	}
	if got, want := nodes[0].Children[1].TopicID, legacySessionTopicID(older); got != want {
		t.Fatalf("older topic second = %q, want %q", got, want)
	}

	meta, ok, err := agent.LoadBranchMeta(newer)
	if err != nil || !ok {
		t.Fatalf("load migrated meta: ok=%v err=%v", ok, err)
	}
	if meta.Scope != "global" || meta.WorkspaceRoot != "" || meta.TopicID != legacySessionTopicID(newer) {
		t.Fatalf("migrated meta = %+v", meta)
	}

	nodes = NewApp().ListProjectTree()
	if got := len(nodes[0].Children); got != 2 {
		t.Fatalf("migration should be idempotent, global topics = %d", got)
	}
}

func TestTopicMigrationMarkerRescansWhenSessionFileChanges(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	writeLegacySession(t, dir, "first.jsonl", "first legacy prompt", time.Now().Add(-time.Hour))

	// First render migrates the legacy session and, with nothing deferred, stamps
	// the one-shot marker so later renders can skip the scan.
	NewApp().ListProjectTree()
	if _, err := os.Stat(filepath.Join(dir, topicMigrationMarker)); err != nil {
		t.Fatalf("expected migration marker after a complete pass: %v", err)
	}

	// A CLI-created session added after the marker invalidates the lightweight
	// gate and gets a fresh migration pass.
	time.Sleep(10 * time.Millisecond)
	second := writeLegacySession(t, dir, "second.jsonl", "second legacy prompt", time.Now())
	NewApp().ListProjectTree()
	meta, ok, err := agent.LoadBranchMeta(second)
	if err != nil {
		t.Fatalf("load second meta: %v", err)
	}
	if !ok || strings.TrimSpace(meta.TopicID) != legacySessionTopicID(second) {
		t.Fatalf("new session after marker should be migrated, got ok=%v meta=%+v", ok, meta)
	}
}

func TestTopicMigrationDefersEmptyLegacySession(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	// An empty legacy session (no user turns) is not migratable yet but could gain
	// content later, so the pass must NOT mark the dir done — otherwise the gate
	// would hide it forever.
	if err := os.WriteFile(filepath.Join(dir, "empty.jsonl"), nil, 0o644); err != nil {
		t.Fatalf("write empty session: %v", err)
	}

	NewApp().ListProjectTree()
	if _, err := os.Stat(filepath.Join(dir, topicMigrationMarker)); err == nil {
		t.Fatal("an empty legacy session must defer marking, but the dir was marked done")
	}
}

func TestV05LegacyEventSessionsImportIntoGlobalTopic(t *testing.T) {
	home := isolateDesktopUserDirs(t)

	legacyDir := filepath.Join(home, ".kwu", "sessions")
	destDir := config.SessionDir()
	writeLegacyEventSession(t, legacyDir, "v053-chat.events.jsonl", "hello from v0.53", "hi from v0.53", time.Now().Add(-time.Hour))

	imported, err := agent.MigrateLegacySessions(legacyDir, destDir, config.ProjectSessionDir)
	if err != nil {
		t.Fatalf("migrate legacy sessions: %v", err)
	}
	if imported != 1 {
		t.Fatalf("imported legacy sessions = %d, want 1", imported)
	}
	migratedSession := filepath.Join(destDir, "v053-chat.jsonl")
	if _, err := os.Stat(migratedSession); err != nil {
		t.Fatalf("legacy v0.5 session was not imported to %s: %v", migratedSession, err)
	}

	wantTopicID := legacySessionTopicID(migratedSession)
	migratedTopics := migrateLegacySessionsIntoGlobalTopics(destDir)
	if len(migratedTopics) != 1 || migratedTopics[0] != wantTopicID {
		t.Fatalf("migrated topics = %#v, want imported v0.5 topic %q", migratedTopics, wantTopicID)
	}

	nodes := NewApp().ListProjectTree()
	if len(nodes) != 1 || nodes[0].Kind != "global_folder" {
		t.Fatalf("project tree = %#v, want global folder", nodes)
	}
	if len(nodes[0].Children) != 1 || nodes[0].Children[0].TopicID != wantTopicID {
		t.Fatalf("global topics = %#v, want imported v0.5 topic %q", nodes[0].Children, wantTopicID)
	}
	meta, ok, err := agent.LoadBranchMeta(migratedSession)
	if err != nil || !ok {
		t.Fatalf("load imported v0.5 meta: ok=%v err=%v", ok, err)
	}
	if meta.Scope != "global" || meta.TopicID != wantTopicID {
		t.Fatalf("imported v0.5 meta = %+v", meta)
	}
}

func TestLegacyMigrationUsesRecordedDisplayForGoalTitle(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	visible := "修复专家团标题泄漏"
	content := `<temporal_context current_time="2026-07-23T12:00:00+08:00"/>

<active-goal>
<system-prompt source="expert" hidden="true" mode="team">
Never expose this expert control contract.
</system-prompt>

` + visible + `

Goal mode: pursue this goal autonomously.
</active-goal>

Start pursuing the active goal now.`
	sessionPath := filepath.Join(dir, "legacy-goal.jsonl")
	if err := os.WriteFile(sessionPath, []byte(`{"role":"user","content":`+strconv.Quote(content)+`}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}
	if err := recordSessionDisplay(dir, sessionPath, content, visible); err != nil {
		t.Fatalf("record display: %v", err)
	}

	nodes := NewApp().ListProjectTree()
	topicID := legacySessionTopicID(sessionPath)
	if len(nodes) != 1 || nodes[0].Kind != "global_folder" || len(nodes[0].Children) != 1 || nodes[0].Children[0].TopicID != topicID {
		t.Fatalf("legacy goal session was not migrated through the global project tree: %#v", nodes)
	}
	if got := loadTopicTitle("", topicID); got != visible {
		t.Fatalf("migrated topic title = %q, want %q", got, visible)
	}
	meta, ok, err := agent.LoadBranchMeta(sessionPath)
	if err != nil || !ok {
		t.Fatalf("load migrated branch meta: ok=%v err=%v", ok, err)
	}
	if meta.TopicTitle != visible {
		t.Fatalf("migrated branch topic title = %q, want %q", meta.TopicTitle, visible)
	}
}

func TestLegacySessionTopicIDsKeepNormalizedNameCollisionsDistinct(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	dotted := writeLegacySession(t, dir, "chat.1.jsonl", "dotted prompt", time.Now().Add(-2*time.Hour))
	underscored := writeLegacySession(t, dir, "chat_1.jsonl", "underscored prompt", time.Now().Add(-time.Hour))

	dottedTopic := legacySessionTopicID(dotted)
	underscoredTopic := legacySessionTopicID(underscored)
	if dottedTopic == underscoredTopic {
		t.Fatalf("normalized legacy topic IDs collided: %q", dottedTopic)
	}

	nodes := NewApp().ListProjectTree()
	if len(nodes) != 1 || nodes[0].Kind != "global_folder" {
		t.Fatalf("project tree = %#v, want global folder", nodes)
	}
	if got := len(nodes[0].Children); got != 2 {
		t.Fatalf("global migrated topics = %d, want 2: %#v", got, nodes[0].Children)
	}
	seen := map[string]bool{}
	for _, child := range nodes[0].Children {
		seen[child.TopicID] = true
	}
	if !seen[dottedTopic] || !seen[underscoredTopic] {
		t.Fatalf("global topics = %#v, want %q and %q", nodes[0].Children, dottedTopic, underscoredTopic)
	}
}

func TestDefaultGlobalTabGetsMigratedTopicID(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	sessionPath := writeLegacySession(t, dir, "legacy-tab.jsonl", "resume this legacy tab", time.Now().Add(-time.Hour))

	tab := &WorkspaceTab{
		ID:            "tab_legacy",
		Scope:         "global",
		WorkspaceRoot: globalTabWorkspaceRoot(),
		Ready:         false,
		disabledMCP:   map[string]ServerView{},
	}
	app := &App{
		tabRuntimes: map[string]*WorkspaceTab{"tab_legacy": tab},
		tabOrder:    []string{"tab_legacy"},
		activeTabID: "tab_legacy",
	}
	app.buildTabController(tab)
	if tab.Ctrl != nil {
		defer tab.Ctrl.Close()
	}

	wantTopicID := legacySessionTopicID(sessionPath)
	if tab.TopicID != wantTopicID {
		t.Fatalf("tab topicID = %q, want %q", tab.TopicID, wantTopicID)
	}
	if tab.Ctrl == nil {
		t.Fatalf("tab controller was not built")
	}
	if tab.Ctrl.SessionPath() != sessionPath {
		t.Fatalf("tab session path = %q, want %q", tab.Ctrl.SessionPath(), sessionPath)
	}
	f := loadTabsFile()
	if len(f.Tabs) != 1 || f.Tabs[0].ID != "tab_legacy" || f.Tabs[0].TopicID != wantTopicID {
		t.Fatalf("desktop tabs file = %+v, want tab id and migrated topic", f)
	}
}

func TestBuildTabControllerRestoresPinnedSessionBeforeTopicFallback(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	configureDesktopTestBYOKRoute(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	topicID := "topic_same"
	topicTitle := "Pinned topic"
	pinned := writeTopicSessionWithPrompt(t, dir, "long.jsonl", topicID, topicTitle, "", "full 64-turn conversation", time.Now().Add(-2*time.Hour))
	_ = writeTopicSessionWithPrompt(t, dir, "short.jsonl", topicID, topicTitle, "", "early 5-turn snapshot", time.Now().Add(time.Hour))

	tab := app.createTabEntryWithID("global", globalTabWorkspaceRoot(), topicID, "tab_pinned")
	setTabSceneStatus(tab, legacyDesktopSceneStatus())
	tab.TopicTitle = topicTitle
	tab.SessionPath = pinned
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}
	app.tabRuntimes[tab.ID] = tab
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	app.buildTabController(tab)
	if tab.Ctrl == nil {
		t.Fatalf("tab controller was not built: %s", tab.StartupErr)
	}
	defer tab.Ctrl.Close()

	if got := filepath.Clean(tab.Ctrl.SessionPath()); got != filepath.Clean(pinned) {
		t.Fatalf("restored session path = %q, want pinned %q", got, pinned)
	}
	history := tab.Ctrl.History()
	if len(history) == 0 || history[0].Content != "full 64-turn conversation" {
		t.Fatalf("restored history = %+v, want pinned long conversation", history)
	}
	f := loadTabsFile()
	if len(f.Tabs) != 1 || filepath.Clean(f.Tabs[0].SessionPath) != filepath.Clean(pinned) {
		t.Fatalf("desktop tabs file = %+v, want pinned session path %q", f, pinned)
	}
}

func TestBuildTabControllerUsesPinnedSessionMetaWorkspace(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	configureDesktopTestBYOKRoute(t)

	projectA := t.TempDir()
	projectB := t.TempDir()
	if err := addProject(projectA, "Project A"); err != nil {
		t.Fatalf("add project A: %v", err)
	}
	if err := addProject(projectB, "Project B"); err != nil {
		t.Fatalf("add project B: %v", err)
	}

	topicID := "topic_restore_workspace"
	topicTitle := "Restore workspace"
	sessionDirA := desktopSessionDir(projectA)
	if err := os.MkdirAll(sessionDirA, 0o755); err != nil {
		t.Fatalf("mkdir project A sessions: %v", err)
	}
	pinned := writeTopicSessionWithPrompt(t, sessionDirA, "project-a.jsonl", topicID, topicTitle, projectA, "project A prompt", time.Now())

	tab := app.createTabEntryWithID("project", projectB, topicID, "tab_stale_workspace")
	setTabSceneStatus(tab, legacyDesktopSceneStatus())
	tab.TopicTitle = topicTitle
	tab.SessionPath = pinned
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}
	app.tabRuntimes[tab.ID] = tab
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	app.buildTabController(tab)
	if tab.Ctrl == nil {
		t.Fatalf("tab controller was not built: %s", tab.StartupErr)
	}
	defer tab.Ctrl.Close()

	if got := filepath.Clean(tab.Ctrl.SessionPath()); got != filepath.Clean(pinned) {
		t.Fatalf("restored session path = %q, want pinned %q", got, pinned)
	}
	if got := normalizeProjectRoot(tab.WorkspaceRoot); got != normalizeProjectRoot(projectA) {
		t.Fatalf("tab workspace root = %q, want project A %q", got, normalizeProjectRoot(projectA))
	}
	history := tab.Ctrl.History()
	if len(history) == 0 || history[0].Content != "project A prompt" {
		t.Fatalf("restored history = %+v, want project A prompt", history)
	}
}

func TestBuildTabControllerIgnoresStaleSessionModelWhenTabModelResolves(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	if _, err := config.SetCredential("KWU_TEST_KEY", "sk-test"); err != nil {
		t.Fatalf("set credential: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte(`
default_model = "default-provider/default-model"

[[providers]]
name = "default-provider"
kind = "openai"
base_url = "https://default.invalid/v1"
model = "default-model"
api_key_env = "KWU_TEST_KEY"

[[providers]]
name = "tab-provider"
kind = "openai"
base_url = "https://tab.invalid/v1"
model = "tab-model"
api_key_env = "KWU_TEST_KEY"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	pinned := writeLegacySession(t, dir, "stale-model.jsonl", "resume with tab model", time.Now())
	meta, err := agent.EnsureBranchMeta(pinned)
	if err != nil {
		t.Fatal(err)
	}
	meta.Model = "missing-provider/missing-model"
	if err := agent.SaveBranchMetaPreserveUpdated(pinned, meta); err != nil {
		t.Fatal(err)
	}

	tab := app.createTabEntryWithID("global", globalTabWorkspaceRoot(), "", "tab_stale_model")
	tab.SessionPath = pinned
	tab.model = "tab-provider/tab-model"
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}
	app.tabRuntimes[tab.ID] = tab
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	app.buildTabController(tab)
	if tab.Ctrl == nil {
		t.Fatalf("tab controller was not built: %s", tab.StartupErr)
	}
	defer tab.Ctrl.Close()
	if tab.model != "tab-provider/tab-model" {
		t.Fatalf("tab model = %q, want valid tab model", tab.model)
	}
}

func TestSetModelForTabPersistsModelInSessionMeta(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	if _, err := config.SetCredential("KWU_TEST_KEY", "sk-test"); err != nil {
		t.Fatalf("set credential: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte(`
default_model = "provider-a/model-a"

[desktop]
provider_access = ["provider-a", "provider-b"]
default_billing_source = "newapi_byok"

[[providers]]
name = "provider-a"
kind = "openai"
base_url = "https://a.invalid/v1"
model = "model-a"
api_key_env = "KWU_TEST_KEY"

[[providers]]
name = "provider-b"
kind = "openai"
base_url = "https://b.invalid/v1"
model = "model-b"
api_key_env = "KWU_TEST_KEY"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	path := writeLegacySession(t, dir, "switch-model.jsonl", "keep this conversation", time.Now())

	app.ctx = context.Background()
	app.runtimeEvents.emit = func(context.Context, string, ...interface{}) {}
	ctrl := control.New(control.Options{SessionDir: dir, SessionPath: path, Label: "provider-a/model-a"})
	tab := app.createTabEntryWithID("global", globalTabWorkspaceRoot(), "topic_model_switch", "tab_model_switch")
	tab.SessionPath = path
	tab.model = "provider-a/model-a"
	tab.Ctrl = ctrl
	tab.Ready = true
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}
	tab.sink.runtimeEvents.emit = func(context.Context, string, ...interface{}) {}
	app.tabRuntimes[tab.ID] = tab
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	if err := app.SetModelForTab(tab.ID, "provider-b/model-b"); err != nil {
		t.Fatalf("SetModelForTab: %v", err)
	}
	defer func() {
		if tab.Ctrl != nil {
			tab.Ctrl.Close()
		}
	}()

	model, ok := agent.LoadSessionModel(tab.SessionPath)
	if !ok || model != "provider-b/model-b" {
		t.Fatalf("persisted session model = %q, %v; want provider-b/model-b", model, ok)
	}
	if got := loadTopicModel("", tab.TopicID); got != "provider-b/model-b" {
		t.Fatalf("persisted topic model = %q, want provider-b/model-b", got)
	}
	if got := config.LoadForEdit(config.UserConfigPath()).DefaultModel; got != "provider-a/model-a" {
		t.Fatalf("default_model = %q, want unchanged provider-a/model-a", got)
	}
	if got, _ := desktopNewSessionDefaults(); got != "provider-a/model-a" {
		t.Fatalf("new session default model = %q, want unchanged provider-a/model-a", got)
	}
	persisted, ok, err := agent.LoadBranchMeta(tab.SessionPath)
	if err != nil || !ok {
		t.Fatalf("LoadBranchMeta = %+v, %v, %v", persisted, ok, err)
	}
	if persisted.BillingSource != string(modelbilling.BillingNewAPIByok) {
		t.Fatalf("persisted billing source = %q, want newapi_byok", persisted.BillingSource)
	}
}

func TestOpenGlobalTopicUsesPersistedTopicModelForEmptySession(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	if _, err := config.SetCredential("KWU_TEST_KEY", "sk-test"); err != nil {
		t.Fatalf("set credential: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(config.UserConfigPath()), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(config.UserConfigPath(), []byte(`
default_model = "provider-a/model-a"

[[providers]]
name = "provider-a"
kind = "openai"
base_url = "https://a.invalid/v1"
model = "model-a"
api_key_env = "KWU_TEST_KEY"

[[providers]]
name = "provider-b"
kind = "openai"
base_url = "https://b.invalid/v1"
model = "model-b"
api_key_env = "KWU_TEST_KEY"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	topicID := "topic_empty_model"
	if err := setTopicTitleWithSource("", topicID, "Empty model topic", topicTitleSourceManual); err != nil {
		t.Fatalf("set topic title: %v", err)
	}
	if err := setTopicModel("", topicID, "provider-b/model-b"); err != nil {
		t.Fatalf("set topic model: %v", err)
	}

	meta, err := app.OpenGlobalTab(topicID)
	if err != nil {
		t.Fatalf("OpenGlobalTab: %v", err)
	}
	tab := waitForTabReady(t, app, meta.ID)
	if tab.model != "provider-b/model-b" {
		t.Fatalf("tab model = %q, want provider-b/model-b", tab.model)
	}
	if tab.Ctrl == nil {
		t.Fatalf("tab controller was not built")
	}
	model, ok := agent.LoadSessionModel(tab.SessionPath)
	if !ok || model != "provider-b/model-b" {
		t.Fatalf("empty session model = %q, %v; want provider-b/model-b", model, ok)
	}
}

func TestLoadPinnedTabSessionDoesNotGuessMigratedBasename(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	path := writeLegacySession(t, dir, "migrated-tab.jsonl", "resume after path migration", time.Now())
	oldPath := filepath.Join(t.TempDir(), "old-kwu", "projects", "slug", "sessions", filepath.Base(path))

	loaded, pinnedPath, ok := loadPinnedTabSession(dir, oldPath)
	if ok || loaded != nil || pinnedPath != "" {
		t.Fatalf("loadPinnedTabSession guessed basename: ok=%v loaded=%v path=%q", ok, loaded, pinnedPath)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("original migrated session changed: %v", err)
	}
}

func TestLoadPinnedTabSessionSkipsCleanupPending(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	path := writeLegacySession(t, dir, "pending-pinned.jsonl", "pending pinned", time.Now())
	if err := agent.MarkCleanupPending(path, "delete"); err != nil {
		t.Fatal(err)
	}

	if loaded, pinnedPath, ok := loadPinnedTabSession(dir, path); ok || loaded != nil || pinnedPath != "" {
		t.Fatalf("loadPinnedTabSession cleanup-pending = loaded:%v path:%q ok:%v, want skipped", loaded, pinnedPath, ok)
	}
}

func TestLoadPinnedTabSessionSkipsEmptyPlaceholder(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	path := filepath.Join(dir, "empty-placeholder.jsonl")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("write empty placeholder: %v", err)
	}

	loaded, pinnedPath, ok := loadPinnedTabSession(dir, path)
	if !ok || loaded != nil || filepath.Clean(pinnedPath) != filepath.Clean(path) {
		t.Fatalf("empty placeholder = loaded:%v path:%q ok:%v, want nil loaded with pinned path", loaded, pinnedPath, ok)
	}
}

func TestLoadPinnedTabSessionKeepsSystemOnlySession(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	path := filepath.Join(dir, "system-only.jsonl")
	sess := agent.NewSession("system prompt")
	if err := sess.Save(path); err != nil {
		t.Fatalf("save system-only session: %v", err)
	}

	loaded, pinnedPath, ok := loadPinnedTabSession(dir, path)
	if !ok || loaded == nil || filepath.Clean(pinnedPath) != filepath.Clean(path) {
		t.Fatalf("system-only session = loaded:%v path:%q ok:%v, want loaded session", loaded, pinnedPath, ok)
	}
	if got := loaded.Snapshot(); len(got) != 1 || got[0].Role != provider.RoleSystem {
		t.Fatalf("loaded system-only messages = %+v", got)
	}
}

func TestLoadPinnedTabSessionSkipsPreloadedEmptyPlaceholder(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	path := filepath.Join(dir, "preloaded-empty-placeholder.jsonl")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("write empty placeholder: %v", err)
	}
	preloaded := loadedTabSession{Path: path, Session: agent.NewSession("")}

	loaded, pinnedPath, ok := loadPinnedTabSessionWithPreload(dir, path, preloaded)
	if !ok || loaded != nil || filepath.Clean(pinnedPath) != filepath.Clean(path) {
		t.Fatalf("preloaded empty placeholder = loaded:%v path:%q ok:%v, want nil loaded with pinned path", loaded, pinnedPath, ok)
	}
}

func TestBuildTabControllerSkipsCleanupPendingPinnedSession(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	configureDesktopTestBYOKRoute(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	pending := writeLegacySession(t, dir, "pending-startup.jsonl", "pending startup", time.Now())
	if err := agent.MarkCleanupPending(pending, "delete"); err != nil {
		t.Fatal(err)
	}

	tab := app.createTabEntryWithID("global", globalTabWorkspaceRoot(), "", "tab_pending")
	tab.SessionPath = pending
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}
	app.tabRuntimes[tab.ID] = tab
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	app.buildTabController(tab)
	if tab.Ctrl != nil || !tab.Ready || !strings.Contains(tab.StartupErr, "pending cleanup") {
		t.Fatalf("cleanup-pending startup = ctrl:%T ready:%t error:%q", tab.Ctrl, tab.Ready, tab.StartupErr)
	}
	if filepath.Clean(tab.SessionPath) != filepath.Clean(pending) {
		t.Fatalf("cleanup-pending path = %q, want preserved %q", tab.SessionPath, pending)
	}
	if history, err := app.HistoryForTab(tab.ID); err == nil || history != nil {
		t.Fatalf("cleanup-pending history = %+v, %v; want error", history, err)
	}
}

func TestBuildTabControllerKeepsMissingPinnedSessionPath(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	configureDesktopTestBYOKRoute(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	topicID := "topic_empty"
	topicTitle := "Empty pinned topic"
	_ = writeTopicSessionWithPrompt(t, dir, "old.jsonl", topicID, topicTitle, "", "old topic history", time.Now())
	pinned := filepath.Join(dir, "empty-new.jsonl")

	tab := app.createTabEntryWithID("global", globalTabWorkspaceRoot(), topicID, "tab_empty")
	tab.TopicTitle = topicTitle
	tab.SessionPath = pinned
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}
	app.tabRuntimes[tab.ID] = tab
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	app.buildTabController(tab)
	if tab.Ctrl != nil || !tab.Ready || tab.StartupErr == "" {
		t.Fatalf("missing pinned startup = ctrl:%T ready:%t error:%q", tab.Ctrl, tab.Ready, tab.StartupErr)
	}
	if got := filepath.Clean(tab.SessionPath); got != filepath.Clean(pinned) {
		t.Fatalf("missing pinned session path = %q, want preserved %q", got, pinned)
	}
	if history, err := app.HistoryForTab(tab.ID); err == nil || history != nil {
		t.Fatalf("missing pinned history = %+v, %v; want error", history, err)
	}
}

func TestRestoredTabSessionPathDoesNotFallbackWhenBlankFileCreationFails(t *testing.T) {
	home := isolateDesktopUserDirs(t)
	blockedStateRoot := filepath.Join(home, "blocked-state")
	if err := os.WriteFile(blockedStateRoot, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KWU_STATE_HOME", blockedStateRoot)

	app := NewApp()
	tab := app.createTabEntryWithID("global", globalTabWorkspaceRoot(), "topic", "tab_restore_failure")
	invalidPath := filepath.Join(t.TempDir(), "outside.jsonl")
	got, replaced := restoredTabSessionPath(tab, invalidPath)
	if replaced || filepath.Clean(got) != filepath.Clean(invalidPath) {
		t.Fatalf("failed blank replacement = %q, replaced:%t; want original strict path", got, replaced)
	}
}

func TestReorderProjectsPersistsSidebarAndWorkspaceOrder(t *testing.T) {
	isolateDesktopUserDirs(t)

	first := t.TempDir()
	second := t.TempDir()
	third := t.TempDir()
	if err := addProject(first, "First"); err != nil {
		t.Fatalf("add first project: %v", err)
	}
	if err := addProject(second, "Second"); err != nil {
		t.Fatalf("add second project: %v", err)
	}
	if err := addProject(third, "Third"); err != nil {
		t.Fatalf("add third project: %v", err)
	}

	app := NewApp()
	if err := app.ReorderProjects([]string{third, first, second}); err != nil {
		t.Fatalf("ReorderProjects: %v", err)
	}

	nodes := app.ListProjectTree()
	if len(nodes) != 3 {
		t.Fatalf("project tree len = %d, want 3: %+v", len(nodes), nodes)
	}
	if got := []string{nodes[0].Root, nodes[1].Root, nodes[2].Root}; got[0] != third || got[1] != first || got[2] != second {
		t.Fatalf("project tree order = %v, want %v", got, []string{third, first, second})
	}
	workspaces := app.ListWorkspaces()
	if len(workspaces) != 3 {
		t.Fatalf("workspaces len = %d, want 3: %+v", len(workspaces), workspaces)
	}
	if got := []string{workspaces[0].Path, workspaces[1].Path, workspaces[2].Path}; got[0] != third || got[1] != first || got[2] != second {
		t.Fatalf("workspace order = %v, want %v", got, []string{third, first, second})
	}
}

func TestReorderProjectsPersistsGlobalSidebarOrder(t *testing.T) {
	isolateDesktopUserDirs(t)

	first := t.TempDir()
	second := t.TempDir()
	if err := addProject(first, "First"); err != nil {
		t.Fatalf("add first project: %v", err)
	}
	if err := addProject(second, "Second"); err != nil {
		t.Fatalf("add second project: %v", err)
	}

	app := NewApp()
	if _, err := app.CreateTopic("global", "", "Global note"); err != nil {
		t.Fatalf("create global topic: %v", err)
	}
	if err := app.ReorderProjects([]string{second, desktopGlobalOrderToken, first}); err != nil {
		t.Fatalf("ReorderProjects with global: %v", err)
	}

	nodes := app.ListProjectTree()
	if len(nodes) != 3 {
		t.Fatalf("project tree len = %d, want 3: %+v", len(nodes), nodes)
	}
	if got := []string{nodes[0].Root, nodes[1].Kind, nodes[2].Root}; got[0] != second || got[1] != "global_folder" || got[2] != first {
		t.Fatalf("project tree order = %v, want [%s global_folder %s]", got, second, first)
	}
	workspaces := app.ListWorkspaces()
	if len(workspaces) != 2 {
		t.Fatalf("workspaces len = %d, want 2: %+v", len(workspaces), workspaces)
	}
	if got := []string{workspaces[0].Path, workspaces[1].Path}; got[0] != second || got[1] != first {
		t.Fatalf("workspace order = %v, want %v", got, []string{second, first})
	}
}

func TestReorderProjectsRejectsInvalidOrder(t *testing.T) {
	isolateDesktopUserDirs(t)

	first := t.TempDir()
	second := t.TempDir()
	if err := addProject(first, "First"); err != nil {
		t.Fatalf("add first project: %v", err)
	}
	if err := addProject(second, "Second"); err != nil {
		t.Fatalf("add second project: %v", err)
	}
	app := NewApp()
	for name, order := range map[string][]string{
		"missing":          {first},
		"unknown":          {first, filepath.Join(t.TempDir(), "missing")},
		"duplicate":        {first, first},
		"duplicate-global": {desktopGlobalOrderToken, first, desktopGlobalOrderToken, second},
	} {
		t.Run(name, func(t *testing.T) {
			if err := app.ReorderProjects(order); err == nil {
				t.Fatalf("ReorderProjects(%v) succeeded, want error", order)
			}
		})
	}

	nodes := app.ListProjectTree()
	if got := []string{nodes[0].Root, nodes[1].Root}; got[0] != first || got[1] != second {
		t.Fatalf("project tree order changed after invalid reorder: %v", got)
	}
}

func TestRemoveWorkspaceUsesSharedProjectRegistryForCurrentProject(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	if err := addProject(projectRoot, "Current Project"); err != nil {
		t.Fatalf("add project: %v", err)
	}
	app := NewApp()
	tab := app.createTabEntryWithID("project", projectRoot, "topic_current", "tab_current")
	app.tabRuntimes[tab.ID] = tab
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	if err := app.RemoveWorkspace(projectRoot); err != nil {
		t.Fatalf("remove current project: %v", err)
	}
	if got := app.ListWorkspaces(); len(got) != 0 {
		t.Fatalf("workspaces after remove = %+v, want empty", got)
	}
	if got := app.ListProjectTree(); len(got) != 1 || got[0].Kind != "global_folder" || len(got[0].Children) != 0 {
		t.Fatalf("project tree after remove = %+v, want empty Global folder", got)
	}
}

func TestRestoredProjectTabUsesStoredTopicTitle(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topicID := "topic_stored_title"
	if err := addProject(projectRoot, ""); err != nil {
		t.Fatalf("add project: %v", err)
	}
	if err := setTopicTitle(projectRoot, topicID, "你是谁"); err != nil {
		t.Fatalf("set topic title: %v", err)
	}

	app := NewApp()
	tab := app.createTabEntryWithID("project", projectRoot, topicID, "tab1")
	app.tabRuntimes[tab.ID] = tab
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	tabs := app.ListTabs()
	if len(tabs) != 1 {
		t.Fatalf("tabs len = %d, want 1", len(tabs))
	}
	if got := tabs[0].TopicTitle; got != "你是谁" {
		t.Fatalf("tab title = %q, want 你是谁", got)
	}
	nodes := app.ListProjectTree()
	if len(nodes) != 1 || len(nodes[0].Children) != 1 {
		t.Fatalf("project tree = %#v, want one project with one topic", nodes)
	}
	if got := nodes[0].Children[0].Label; got != tabs[0].TopicTitle {
		t.Fatalf("tree title = %q, want same as tab title %q", got, tabs[0].TopicTitle)
	}
}

func TestUntitledProjectTopicUsesSameFallbackEverywhere(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topicID := "topic_without_title"
	if err := saveProjectsFile(desktopProjectFile{Projects: []desktopProject{{
		Root:   projectRoot,
		Topics: []string{topicID},
	}}}); err != nil {
		t.Fatalf("save projects: %v", err)
	}

	app := NewApp()
	tab := app.createTabEntryWithID("project", projectRoot, topicID, "tab1")
	app.tabRuntimes[tab.ID] = tab
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	tabs := app.ListTabs()
	if len(tabs) != 1 {
		t.Fatalf("tabs len = %d, want 1", len(tabs))
	}
	if got := tabs[0].TopicTitle; got != defaultTopicTitle {
		t.Fatalf("tab title = %q, want %q", got, defaultTopicTitle)
	}
	nodes := app.ListProjectTree()
	if len(nodes) != 1 || len(nodes[0].Children) != 1 {
		t.Fatalf("project tree = %#v, want one project with one topic", nodes)
	}
	if got := nodes[0].Children[0].Label; got != defaultTopicTitle {
		t.Fatalf("tree title = %q, want %q", got, defaultTopicTitle)
	}
}

func TestCreateTopicDefaultsToAutoNewSessionTitle(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	before := time.Now().UnixMilli()
	topic, err := NewApp().CreateTopic("project", projectRoot, "")
	after := time.Now().UnixMilli()
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	if got := topic.Title; got != defaultTopicTitle {
		t.Fatalf("topic title = %q, want %q", got, defaultTopicTitle)
	}
	if got := loadTopicTitle(projectRoot, topic.ID); got != defaultTopicTitle {
		t.Fatalf("stored title = %q, want %q", got, defaultTopicTitle)
	}
	if got := loadTopicTitleSource(projectRoot, topic.ID); got != topicTitleSourceAuto {
		t.Fatalf("title source = %q, want auto", got)
	}
	if got := loadTopicCreatedAt(projectRoot, topic.ID); got < before || got > after {
		t.Fatalf("createdAt = %d, want between %d and %d", got, before, after)
	}
	nodes := NewApp().ListProjectTree()
	if len(nodes) != 1 || len(nodes[0].Children) != 1 {
		t.Fatalf("project tree = %#v, want one project with one topic", nodes)
	}
	if got := nodes[0].Children[0].CreatedAt; got != topic.CreatedAt {
		t.Fatalf("project tree createdAt = %d, want %d", got, topic.CreatedAt)
	}
}

func TestCreateTopicAppearsFirstInProjectTree(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	app := NewApp()
	first, err := app.CreateTopic("project", projectRoot, "")
	if err != nil {
		t.Fatalf("create first topic: %v", err)
	}
	second, err := app.CreateTopic("project", projectRoot, "")
	if err != nil {
		t.Fatalf("create second topic: %v", err)
	}

	nodes := app.ListProjectTree()
	if len(nodes) != 1 || len(nodes[0].Children) != 2 {
		t.Fatalf("project tree = %#v, want one project with two topics", nodes)
	}
	if got := nodes[0].Children[0].TopicID; got != second.ID {
		t.Fatalf("first visible topic = %q, want newest %q", got, second.ID)
	}
	if got := nodes[0].Children[1].TopicID; got != first.ID {
		t.Fatalf("second visible topic = %q, want older %q", got, first.ID)
	}
}

func TestCreateGlobalTopicAppearsFirstInProjectTree(t *testing.T) {
	isolateDesktopUserDirs(t)

	app := NewApp()
	first, err := app.CreateTopic("global", "", "")
	if err != nil {
		t.Fatalf("create first global topic: %v", err)
	}
	second, err := app.CreateTopic("global", "", "")
	if err != nil {
		t.Fatalf("create second global topic: %v", err)
	}

	nodes := app.ListProjectTree()
	if len(nodes) != 1 || nodes[0].Kind != "global_folder" || len(nodes[0].Children) != 2 {
		t.Fatalf("project tree = %#v, want Global with two topics", nodes)
	}
	if got := nodes[0].Children[0].TopicID; got != second.ID {
		t.Fatalf("first visible global topic = %q, want newest %q", got, second.ID)
	}
	if got := nodes[0].Children[1].TopicID; got != first.ID {
		t.Fatalf("second visible global topic = %q, want older %q", got, first.ID)
	}
}

func TestCreateTopicPersistsCompleteRegistration(t *testing.T) {
	for _, tc := range []struct {
		name  string
		scope string
	}{
		{name: "project", scope: "project"},
		{name: "global", scope: "global"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateDesktopUserDirs(t)
			root := ""
			if tc.scope == "project" {
				root = t.TempDir()
			}
			topic, err := NewApp().CreateTopic(tc.scope, root, "Registered topic")
			if err != nil {
				t.Fatalf("CreateTopic: %v", err)
			}
			titles, err := loadTopicRegistrationStringMap(root, topicTitlesFile)
			if err != nil || titles[topic.ID] != topic.Title {
				t.Fatalf("titles[%q] = %q, err=%v", topic.ID, titles[topic.ID], err)
			}
			sources, err := loadTopicRegistrationStringMap(root, topicTitleSourcesFile)
			if err != nil || sources[topic.ID] != topicTitleSourceManual {
				t.Fatalf("sources[%q] = %q, err=%v", topic.ID, sources[topic.ID], err)
			}
			created, err := loadTopicRegistrationCreatedAts(root)
			if err != nil || created[topic.ID] != topic.CreatedAt {
				t.Fatalf("created[%q] = %d, err=%v, want %d", topic.ID, created[topic.ID], err, topic.CreatedAt)
			}
			projects, err := loadProjectsFileForUpdate()
			if err != nil {
				t.Fatalf("load projects: %v", err)
			}
			if tc.scope == "global" {
				if len(projects.GlobalTopics) == 0 || projects.GlobalTopics[0] != topic.ID {
					t.Fatalf("global topics = %v, want %q first", projects.GlobalTopics, topic.ID)
				}
				return
			}
			found := false
			for _, project := range projects.Projects {
				if project.Root == normalizeProjectRoot(root) {
					found = len(project.Topics) > 0 && project.Topics[0] == topic.ID
				}
			}
			if !found {
				t.Fatalf("project registration missing from %+v", projects.Projects)
			}
		})
	}
}

func TestCreateTopicRejectsCorruptRegistrationIndexesWithoutOverwriting(t *testing.T) {
	for _, filename := range []string{topicTitlesFile, topicTitleSourcesFile, topicCreatedAtsFile} {
		t.Run(filename, func(t *testing.T) {
			isolateDesktopUserDirs(t)
			root := t.TempDir()
			path, err := currentTopicMetadataPath(root, filename)
			if err != nil {
				t.Fatalf("resolve metadata path: %v", err)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			corrupt := []byte("{broken registration index")
			if err := os.WriteFile(path, corrupt, 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := NewApp().CreateTopic("project", root, "must fail"); err == nil {
				t.Fatal("CreateTopic succeeded with corrupt registration index")
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != string(corrupt) {
				t.Fatalf("corrupt index was overwritten: %q", got)
			}
		})
	}
}

func TestCreateTopicRejectsCorruptProjectsFileWithoutOverwriting(t *testing.T) {
	isolateDesktopUserDirs(t)
	path := filepath.Join(desktopConfigDir(), desktopProjectsFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	corrupt := []byte("not-json-projects")
	if err := os.WriteFile(path, corrupt, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewApp().CreateTopic("global", "", "must fail"); err == nil {
		t.Fatal("CreateTopic succeeded with corrupt projects file")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(corrupt) {
		t.Fatalf("corrupt projects file was overwritten: %q", got)
	}
}

func TestConcurrentTopicRegistrationPreservesProjectUpdates(t *testing.T) {
	for _, tc := range []struct {
		name   string
		update func(*App, string, string, string) error
		check  func(*testing.T, desktopProjectFile, string, string, string)
	}{
		{
			name:   "add",
			update: func(_ *App, _, _, added string) error { return addProject(added, "Added") },
			check: func(t *testing.T, f desktopProjectFile, _, _, added string) {
				for _, p := range f.Projects {
					if p.Root == added {
						return
					}
				}
				t.Fatalf("added project %q missing", added)
			},
		},
		{
			name:   "remove",
			update: func(_ *App, _, removed, _ string) error { return removeProject(removed) },
			check: func(t *testing.T, f desktopProjectFile, _, removed, _ string) {
				for _, p := range f.Projects {
					if p.Root == removed {
						t.Fatalf("removed project %q still present", removed)
					}
				}
			},
		},
		{
			name: "reorder",
			update: func(app *App, project, removed, _ string) error {
				return app.ReorderProjects([]string{removed, project})
			},
			check: func(t *testing.T, f desktopProjectFile, project, removed, _ string) {
				if len(f.Projects) != 2 || f.Projects[0].Root != removed || f.Projects[1].Root != project {
					t.Fatalf("project order = %+v", f.Projects)
				}
			},
		},
		{
			name:   "pin",
			update: func(app *App, project, _, _ string) error { return app.SetProjectPinned(project, true) },
			check: func(t *testing.T, f desktopProjectFile, project, _, _ string) {
				if !containsDesktopString(f.PinnedProjects, project) {
					t.Fatalf("pinned projects = %v, want %q", f.PinnedProjects, project)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateDesktopUserDirs(t)
			app := NewApp()
			project := normalizeProjectRoot(t.TempDir())
			other := normalizeProjectRoot(t.TempDir())
			added := normalizeProjectRoot(t.TempDir())
			if err := addProject(project, "Project"); err != nil {
				t.Fatal(err)
			}
			if err := addProject(other, "Other"); err != nil {
				t.Fatal(err)
			}

			originalUpdate := updateProjectsFileForTopicRegistration
			entered := make(chan struct{})
			release := make(chan struct{})
			var releaseOnce sync.Once
			t.Cleanup(func() {
				releaseOnce.Do(func() { close(release) })
				updateProjectsFileForTopicRegistration = originalUpdate
			})
			updateProjectsFileForTopicRegistration = func(register func(*desktopProjectFile) (bool, error)) error {
				return updateProjectsFile(func(f *desktopProjectFile) (bool, error) {
					changed, err := register(f)
					close(entered)
					<-release
					return changed, err
				})
			}

			type topicResult struct {
				topic TopicMeta
				err   error
			}
			topicDone := make(chan topicResult, 1)
			go func() {
				topic, err := app.CreateTopic("project", project, "controlled topic")
				topicDone <- topicResult{topic: topic, err: err}
			}()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("topic registration did not enter the locked project update callback")
			}

			writerStarted := make(chan struct{})
			writerDone := make(chan error, 1)
			go func() {
				close(writerStarted)
				writerDone <- tc.update(app, project, other, added)
			}()
			select {
			case <-writerStarted:
			case <-time.After(5 * time.Second):
				t.Fatal("project writer did not start")
			}
			select {
			case err := <-writerDone:
				t.Fatalf("project writer completed before registration released the project-file lock: %v", err)
			case <-time.After(100 * time.Millisecond):
			}

			releaseOnce.Do(func() { close(release) })
			var topic TopicMeta
			select {
			case result := <-topicDone:
				if result.err != nil {
					t.Fatalf("topic registration failed: %v", result.err)
				}
				topic = result.topic
			case <-time.After(5 * time.Second):
				t.Fatal("topic registration did not finish after releasing project-file lock")
			}
			select {
			case err := <-writerDone:
				if err != nil {
					t.Fatalf("project writer failed: %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("project writer did not finish after releasing project-file lock")
			}

			f, err := loadProjectsFileForUpdate()
			if err != nil {
				t.Fatalf("load projects: %v", err)
			}
			tc.check(t, f, project, other, added)
			var topics []string
			for _, p := range f.Projects {
				if p.Root == project {
					topics = p.Topics
				}
			}
			if !containsDesktopString(topics, topic.ID) {
				t.Fatalf("topic %q lost from %v", topic.ID, topics)
			}
		})
	}
}

func TestEnsureTopicIndexedDoesNotMoveExistingProjectTopic(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	app := NewApp()
	first, err := app.CreateTopic("project", projectRoot, "First")
	if err != nil {
		t.Fatalf("create first topic: %v", err)
	}
	second, err := app.CreateTopic("project", projectRoot, "Second")
	if err != nil {
		t.Fatalf("create second topic: %v", err)
	}

	if err := ensureTopicIndexed("project", projectRoot, first.ID, first.Title, loadTopicTitleSource(projectRoot, first.ID)); err != nil {
		t.Fatalf("ensure first topic indexed: %v", err)
	}

	nodes := app.ListProjectTree()
	if len(nodes) != 1 || len(nodes[0].Children) != 2 {
		t.Fatalf("project tree = %#v, want one project with two topics", nodes)
	}
	if got := nodes[0].Children[0].TopicID; got != second.ID {
		t.Fatalf("first visible topic = %q, want existing order head %q", got, second.ID)
	}
	if got := nodes[0].Children[1].TopicID; got != first.ID {
		t.Fatalf("second visible topic = %q, want existing order tail %q", got, first.ID)
	}
}

func TestListProjectTreeShowsEmptyGlobalWhenNoProjects(t *testing.T) {
	isolateDesktopUserDirs(t)

	nodes := NewApp().ListProjectTree()
	if len(nodes) != 1 {
		t.Fatalf("project tree = %#v, want one Global folder", nodes)
	}
	if nodes[0].Kind != "global_folder" || nodes[0].Label != "Global" || len(nodes[0].Children) != 0 {
		t.Fatalf("project tree = %#v, want empty Global folder", nodes)
	}
}

func TestBuildCloudProjectTreeCatalogGroupsCurrentScopeWithoutLocalFolders(t *testing.T) {
	scope := project.Scope{
		Environment: cloud.EnvironmentIdentity{EnvironmentID: strings.Repeat("a", 64), CloudBaseURL: "https://cloud.example.test"},
		Binding:     cloud.BindingScope{ProfileID: "profile-a", CloudUserID: "user-a", BindingGeneration: 2},
		Key:         strings.Repeat("b", 64),
	}
	makePin := func(projectID, cloudUserID string) *agent.ProjectContextPin {
		return &agent.ProjectContextPin{
			Environment:        agent.ProjectEnvironmentIdentity{EnvironmentID: scope.Environment.EnvironmentID, CloudBaseURL: scope.Environment.CloudBaseURL},
			BindingScope:       agent.ProjectBindingScope{ProfileID: scope.Binding.ProfileID, CloudUserID: cloudUserID, BindingGeneration: scope.Binding.BindingGeneration},
			ProjectID:          projectID,
			InstructionVersion: 1,
			ContentHash:        strings.Repeat("c", 64),
		}
	}
	root := t.TempDir()
	cloudGlobal := filepath.Join(root, "cloud-global.jsonl")
	cloudLocal := filepath.Join(root, "cloud-local.jsonl")
	staleAccount := filepath.Join(root, "cloud-stale.jsonl")
	if err := agent.SaveBranchMeta(cloudGlobal, agent.BranchMeta{Scope: "global", TopicID: "mixed-topic", TopicTitle: "Shared global", ProjectContext: makePin("project-1", "user-a")}); err != nil {
		t.Fatal(err)
	}
	if err := agent.SaveBranchMeta(cloudLocal, agent.BranchMeta{Scope: "project", WorkspaceRoot: filepath.Join(root, "workspace"), TopicID: "local-topic", TopicTitle: "Shared local", ProjectContext: makePin("project-fallback", "user-a")}); err != nil {
		t.Fatal(err)
	}
	if err := agent.SaveBranchMeta(staleAccount, agent.BranchMeta{Scope: "global", TopicID: "stale-topic", ProjectContext: makePin("project-stale", "user-b")}); err != nil {
		t.Fatal(err)
	}
	ordinary := filepath.Join(root, "ordinary.jsonl")
	infos := map[string]agent.SessionInfo{
		cloudGlobal:  {Path: cloudGlobal, Scope: "global", TopicID: "mixed-topic", TopicTitle: "Shared global"},
		cloudLocal:   {Path: cloudLocal, Scope: "project", WorkspaceRoot: filepath.Join(root, "workspace"), TopicID: "local-topic", TopicTitle: "Shared local"},
		staleAccount: {Path: staleAccount, Scope: "global", TopicID: "stale-topic"},
		ordinary:     {Path: ordinary, Scope: "global", TopicID: "mixed-topic", TopicTitle: "Ordinary"},
	}
	projects, sessions, cloudTopics, physicalTopics, cloudPaths := buildCloudProjectTreeCatalog([]project.Project{
		{ProjectID: "project-1", Name: "Shared project", Status: project.ProjectActive},
		{ProjectID: "project-archived", Name: "Archived", Status: project.ProjectArchived},
	}, scope, infos, true)
	if len(projects) != 2 || projects[0].ProjectID != "project-1" || projects[1].ProjectID != "project-fallback" || projects[1].Name != "project-fallback" {
		t.Fatalf("logical cloud projects = %+v", projects)
	}
	if len(sessions["project-1"]) != 1 || len(sessions["project-fallback"]) != 1 || len(sessions["project-stale"]) != 0 {
		t.Fatalf("scope-bound cloud sessions = %+v", sessions)
	}
	mixedKey := topicSummaryKey("global", "", "mixed-topic")
	if !cloudTopics[mixedKey] || !physicalTopics[mixedKey] {
		t.Fatalf("mixed topic ownership cloud=%v physical=%v", cloudTopics, physicalTopics)
	}
	if !cloudPaths[sessionRuntimeKey(cloudGlobal)] || !cloudPaths[sessionRuntimeKey(cloudLocal)] || !cloudPaths[sessionRuntimeKey(staleAccount)] || cloudPaths[sessionRuntimeKey(ordinary)] {
		t.Fatalf("cloud session paths = %+v", cloudPaths)
	}
	scopeB := scope
	scopeB.Binding.CloudUserID = "user-b"
	projectsB, sessionsB, cloudTopicsB, physicalTopicsB, cloudPathsB := buildCloudProjectTreeCatalog([]project.Project{
		{ProjectID: "project-stale", Name: "Account B project", Status: project.ProjectActive},
	}, scopeB, infos, true)
	if len(projectsB) != 1 || projectsB[0].ProjectID != "project-stale" || len(sessionsB["project-stale"]) != 1 || len(sessionsB["project-1"]) != 0 || len(sessionsB["project-fallback"]) != 0 {
		t.Fatalf("account B projection = projects:%+v sessions:%+v", projectsB, sessionsB)
	}
	if !cloudTopicsB[mixedKey] || !physicalTopicsB[mixedKey] || !cloudPathsB[sessionRuntimeKey(cloudGlobal)] || !cloudPathsB[sessionRuntimeKey(staleAccount)] {
		t.Fatalf("account switch lost account-independent exclusion classification: cloud=%v physical=%v paths=%v", cloudTopicsB, physicalTopicsB, cloudPathsB)
	}
}

func TestListProjectTreeHidesManagedWorkspaceForVisibleCloudProject(t *testing.T) {
	isolateDesktopUserDirs(t)
	app, state := newWorkBuddyProjectApp(t, nil)
	projectID := "project-managed-workspace"
	managedRoot, err := ensureCloudProjectWorkspace(context.Background(), state.services, projectID)
	if err != nil {
		t.Fatal(err)
	}
	if err := addProject(managedRoot, "Managed implementation workspace"); err != nil {
		t.Fatal(err)
	}
	ordinaryRoot := t.TempDir()
	if err := addProject(ordinaryRoot, "Ordinary workspace"); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := state.services.Cache.StoreDetail(context.Background(), state.scope, project.Project{
		ProjectID: projectID, OwnerUserID: state.scope.Binding.CloudUserID, Name: "Visible cloud project",
		Status: project.ProjectActive, Version: 1, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	nodes := app.ListProjectTree()
	var sawCloudProject, sawOrdinaryWorkspace bool
	for _, node := range nodes {
		if node.Kind == "project" && sameDesktopPath(node.Root, managedRoot) {
			t.Fatalf("managed Cloud Project workspace leaked into the ordinary project tree: %+v", nodes)
		}
		if node.Kind == "cloud_project" && node.CloudProjectID == projectID {
			sawCloudProject = true
		}
		if node.Kind == "project" && sameDesktopPath(node.Root, ordinaryRoot) {
			sawOrdinaryWorkspace = true
		}
	}
	if !sawCloudProject || !sawOrdinaryWorkspace {
		t.Fatalf("project tree lost a logical Cloud Project or ordinary workspace: %+v", nodes)
	}
}

func TestListWorkspacesHidesManagedCloudProjectRoots(t *testing.T) {
	isolateDesktopUserDirs(t)
	app, state := newWorkBuddyProjectApp(t, nil)
	managedRoot, err := ensureCloudProjectWorkspace(context.Background(), state.services, "project-managed-workspace-list")
	if err != nil {
		t.Fatal(err)
	}
	if err := addProject(managedRoot, "Managed implementation workspace"); err != nil {
		t.Fatal(err)
	}
	previousProfileRoot := filepath.Join(
		config.MemoryUserDir(),
		"profiles",
		"previous-profile",
		cloudProjectWorkspacesDir,
		strings.Repeat("c", 64),
		"project-"+strings.Repeat("d", 64),
	)
	if err := addProject(previousProfileRoot, "Previous profile managed workspace"); err != nil {
		t.Fatal(err)
	}
	ordinaryRoot := t.TempDir()
	if err := addProject(ordinaryRoot, "Ordinary workspace"); err != nil {
		t.Fatal(err)
	}
	workspaces := app.ListWorkspaces()
	if len(workspaces) != 1 || !sameDesktopPath(workspaces[0].Path, ordinaryRoot) || workspaces[0].Name != "Ordinary workspace" {
		t.Fatalf("workspace picker entries = %+v, want only the ordinary workspace", workspaces)
	}
	if projects := loadProjectsFile().Projects; len(projects) != 3 {
		t.Fatalf("workspace filtering changed the internal project registry: %+v", projects)
	}
}

func TestListProjectTreeAnonymousStillExcludesCloudBoundSessions(t *testing.T) {
	isolateDesktopUserDirs(t)
	app, state := newWorkBuddyProjectApp(t, nil)
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cloudOnlyTopic := "cloud-only-topic"
	mixedTopic := "mixed-local-cloud-topic"
	if err := ensureTopicIndexed("global", "", cloudOnlyTopic, "Private cloud title", topicTitleSourceManual); err != nil {
		t.Fatal(err)
	}
	if err := ensureTopicIndexed("global", "", mixedTopic, "Local title", topicTitleSourceManual); err != nil {
		t.Fatal(err)
	}

	makeCloudSession := func(name, topicID string, updatedAt time.Time) string {
		path := writeTopicSessionWithPrompt(t, dir, name, topicID, "Private cloud title", "", "private cloud prompt", updatedAt)
		meta, ok, err := agent.LoadBranchMeta(path)
		if err != nil || !ok {
			t.Fatalf("load cloud branch metadata: ok=%v err=%v", ok, err)
		}
		meta.ProjectContext = &agent.ProjectContextPin{
			Environment:        agent.ProjectEnvironmentIdentity{EnvironmentID: state.scope.Environment.EnvironmentID, CloudBaseURL: state.scope.Environment.CloudBaseURL},
			BindingScope:       agent.ProjectBindingScope{ProfileID: state.scope.Binding.ProfileID, CloudUserID: state.scope.Binding.CloudUserID, BindingGeneration: state.scope.Binding.BindingGeneration},
			ProjectID:          "private-project",
			InstructionVersion: 1,
			ContentHash:        strings.Repeat("b", 64),
		}
		if err := agent.SaveBranchMetaPreserveUpdated(path, meta); err != nil {
			t.Fatal(err)
		}
		return path
	}
	cloudOnlyPath := makeCloudSession("cloud-only.jsonl", cloudOnlyTopic, time.Now())
	mixedCloudPath := makeCloudSession("mixed-cloud.jsonl", mixedTopic, time.Now())
	localPath := writeTopicSessionWithPrompt(t, dir, "mixed-local.jsonl", mixedTopic, "Local title", "", "local prompt", time.Now().Add(-time.Minute))

	loggedOut, err := app.CloudLogout()
	if err != nil || loggedOut.Account.Status != cloud.AccountAnonymous {
		t.Fatalf("CloudLogout = %#v, %v", loggedOut, err)
	}
	if _, err := app.projectServicesForOperation(); err == nil {
		t.Fatal("anonymous app unexpectedly exposed cloud project services")
	}
	nodes := app.ListProjectTree()
	if len(nodes) != 1 || nodes[0].Kind != "global_folder" {
		t.Fatalf("anonymous project tree = %#v", nodes)
	}
	if len(nodes[0].Children) != 1 || nodes[0].Children[0].TopicID != mixedTopic {
		t.Fatalf("anonymous tasks exposed cloud-only topic or lost mixed local topic: %#v", nodes[0].Children)
	}
	if got := sessionRuntimeKey(nodes[0].Children[0].SessionPath); got != sessionRuntimeKey(localPath) {
		t.Fatalf("mixed topic selected path = %q, want local path %q", got, localPath)
	}
	for _, path := range []string{cloudOnlyPath, mixedCloudPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("classification removed cloud transcript %q: %v", path, err)
		}
		if _, err := os.Stat(agent.BranchMetaPath(path)); err != nil {
			t.Fatalf("classification removed cloud branch metadata %q: %v", path, err)
		}
	}
}

func TestBuildCloudProjectTreeCatalogDoesNotRecreateDeletedCachedProject(t *testing.T) {
	scope := project.Scope{
		Environment: cloud.EnvironmentIdentity{EnvironmentID: strings.Repeat("a", 64), CloudBaseURL: "https://cloud.example.test"},
		Binding:     cloud.BindingScope{ProfileID: "profile-a", CloudUserID: "user-a", BindingGeneration: 2},
		Key:         strings.Repeat("b", 64),
	}
	root := t.TempDir()
	path := filepath.Join(root, "deleted.jsonl")
	if err := agent.SaveBranchMeta(path, agent.BranchMeta{ProjectContext: &agent.ProjectContextPin{
		Environment:        agent.ProjectEnvironmentIdentity{EnvironmentID: scope.Environment.EnvironmentID, CloudBaseURL: scope.Environment.CloudBaseURL},
		BindingScope:       agent.ProjectBindingScope{ProfileID: scope.Binding.ProfileID, CloudUserID: scope.Binding.CloudUserID, BindingGeneration: scope.Binding.BindingGeneration},
		ProjectID:          "deleted-project",
		InstructionVersion: 1,
		ContentHash:        strings.Repeat("c", 64),
	}}); err != nil {
		t.Fatal(err)
	}
	infos := map[string]agent.SessionInfo{sessionRuntimeKey(path): {Path: path, Turns: 1}}
	projects, sessions, _, _, cloudPaths := buildCloudProjectTreeCatalog(nil, scope, infos, false)
	if len(projects) != 0 || len(sessions) != 0 {
		t.Fatalf("deleted project was recreated from retained session: projects=%#v sessions=%#v", projects, sessions)
	}
	if !cloudPaths[sessionRuntimeKey(path)] {
		t.Fatal("retained deleted-project transcript was not kept out of the global tree")
	}
}

func TestRegisterWorkspaceDoesNotCreateConversation(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	app := NewApp()
	got, err := app.registerWorkspace(projectRoot)
	if err != nil {
		t.Fatalf("registerWorkspace: %v", err)
	}
	if got != normalizeProjectRoot(projectRoot) {
		t.Fatalf("registerWorkspace root = %q, want %q", got, normalizeProjectRoot(projectRoot))
	}
	if tabs := app.ListTabs(); len(tabs) != 0 {
		t.Fatalf("registerWorkspace created tabRuntimes: %+v", tabs)
	}

	var project *ProjectNode
	for _, node := range app.ListProjectTree() {
		if node.Kind == "project" && node.Root == normalizeProjectRoot(projectRoot) {
			node := node
			project = &node
			break
		}
	}
	if project == nil {
		t.Fatalf("registered project missing from project tree: %+v", app.ListProjectTree())
	}
	if len(project.Children) != 0 {
		t.Fatalf("registerWorkspace created conversations: %+v", project.Children)
	}
}

func TestSwitchWorkspaceRegistersDefaultTopicInProjectTree(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	app := NewApp()
	if got, err := app.SwitchWorkspace(projectRoot); err != nil {
		t.Fatalf("SwitchWorkspace: %v", err)
	} else if got != projectRoot {
		t.Fatalf("SwitchWorkspace root = %q, want %q", got, projectRoot)
	}

	nodes := app.ListProjectTree()
	if len(nodes) != 1 {
		t.Fatalf("project tree len = %d, want 1: %+v", len(nodes), nodes)
	}
	if got := nodes[0].Root; got != projectRoot {
		t.Fatalf("project root = %q, want %q", got, projectRoot)
	}
	if len(nodes[0].Children) != 1 {
		t.Fatalf("project children len = %d, want 1: %+v", len(nodes[0].Children), nodes[0].Children)
	}
	child := nodes[0].Children[0]
	if got := child.Label; got != defaultTopicTitle {
		t.Fatalf("default topic label = %q, want %q", got, defaultTopicTitle)
	}
	if strings.TrimSpace(child.TopicID) == "" {
		t.Fatalf("default topic ID should be persisted in the project tree: %+v", child)
	}
	firstTopicID := child.TopicID
	tabs := app.ListTabs()
	if len(tabs) != 1 || tabs[0].TopicID != child.TopicID {
		t.Fatalf("opened tab should use the persisted topic, tabs=%+v child=%+v", tabs, child)
	}

	if _, err := app.SwitchWorkspace(projectRoot); err != nil {
		t.Fatalf("SwitchWorkspace(reopen): %v", err)
	}
	nodes = app.ListProjectTree()
	if len(nodes) != 1 || len(nodes[0].Children) != 1 {
		t.Fatalf("reopening blank workspace created another conversation: %+v", nodes)
	}
	if got := nodes[0].Children[0].TopicID; got != firstTopicID {
		t.Fatalf("reopened topic ID = %q, want reusable blank topic %q", got, firstTopicID)
	}
}

func TestListProjectTreeTopicRowsMatchConcreteOpenTargets(t *testing.T) {
	isolateDesktopUserDirs(t)

	app := NewApp()
	projectRoot := t.TempDir()
	projectSingle, err := app.CreateTopic("project", projectRoot, "Project single")
	if err != nil {
		t.Fatalf("create project single topic: %v", err)
	}
	projectLatestEmpty, err := app.CreateTopic("project", projectRoot, "Project latest empty")
	if err != nil {
		t.Fatalf("create project latest-empty topic: %v", err)
	}
	projectTie, err := app.CreateTopic("project", projectRoot, "Project tie")
	if err != nil {
		t.Fatalf("create project tie topic: %v", err)
	}
	projectBlank, err := app.CreateTopic("project", projectRoot, "Project blank")
	if err != nil {
		t.Fatalf("create project blank topic: %v", err)
	}
	projectLegacy, err := app.CreateTopic("project", projectRoot, "Project legacy")
	if err != nil {
		t.Fatalf("create project legacy topic: %v", err)
	}
	globalSingle, err := app.CreateTopic("global", "", "Global single")
	if err != nil {
		t.Fatalf("create global single topic: %v", err)
	}
	globalBlank, err := app.CreateTopic("global", "", "Global blank")
	if err != nil {
		t.Fatalf("create global blank topic: %v", err)
	}
	globalLatestEmpty, err := app.CreateTopic("global", "", "Global latest empty")
	if err != nil {
		t.Fatalf("create global latest-empty topic: %v", err)
	}
	globalInvalid, err := app.CreateTopic("global", "", "Global invalid")
	if err != nil {
		t.Fatalf("create global invalid topic: %v", err)
	}

	projectDir := desktopSessionDir(projectRoot)
	globalDir := desktopSessionDir(globalWorkspaceRoot())
	for _, dir := range []string{projectDir, globalDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir session dir %q: %v", dir, err)
		}
	}
	base := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	projectSinglePath := writeTopicSessionWithPrompt(t, projectDir, "project-single.jsonl", projectSingle.ID, projectSingle.Title, projectRoot, "project prompt", base)
	writeTopicSessionWithPrompt(t, projectDir, "project-older.jsonl", projectLatestEmpty.ID, projectLatestEmpty.Title, projectRoot, "older prompt", base.Add(-2*time.Hour))
	projectLatestEmptyPath := writeEmptyTopicSession(t, projectDir, "project-newer-empty.jsonl", projectLatestEmpty.ID, projectLatestEmpty.Title, projectRoot, base.Add(-time.Hour))
	projectLegacyPath := writeEmptyTopicSession(t, projectDir, "project-legacy.jsonl", projectLegacy.ID, projectLegacy.Title, projectRoot, base.Add(-30*time.Minute))
	projectTieLaterPath := writeTopicSessionWithPrompt(t, projectDir, "z-tie.jsonl", projectTie.ID, projectTie.Title, projectRoot, "z prompt", base)
	projectTieEarlierPath := writeTopicSessionWithPrompt(t, projectDir, "a-tie.jsonl", projectTie.ID, projectTie.Title, projectRoot, "a prompt", base)
	globalSinglePath := writeTopicSessionWithPrompt(t, globalDir, "global-single.jsonl", globalSingle.ID, globalSingle.Title, "", "global prompt", base)
	globalLatestEmptyPath := writeEmptyTopicSession(t, globalDir, "global-newer-empty.jsonl", globalLatestEmpty.ID, globalLatestEmpty.Title, "", base.Add(-time.Hour))
	globalInvalidPath := writeEmptyTopicSession(t, globalDir, "global-invalid.jsonl", globalInvalid.ID, globalInvalid.Title, "", base.Add(-30*time.Minute))
	setTopicSessionScene(t, projectSinglePath, scene.Status{Scene: scene.Coding, State: scene.Draft})
	setTopicSessionScene(t, projectLatestEmptyPath, scene.Status{Scene: scene.Design, State: scene.Draft})
	setTopicSessionScene(t, globalSinglePath, scene.Status{Scene: scene.Office, State: scene.Locked})
	setTopicSessionScene(t, globalLatestEmptyPath, scene.Status{Scene: scene.Coding, State: scene.Draft})
	setTopicSessionRawScene(t, globalInvalidPath, "broken", "draft")

	nodes := app.ListProjectTree()
	findTopicNode := func(kind, topicID string) ProjectNode {
		t.Helper()
		for _, parent := range nodes {
			for _, child := range parent.Children {
				if child.Kind == kind && child.TopicID == topicID {
					return child
				}
			}
		}
		t.Fatalf("topic %q kind %q missing from project tree: %#v", topicID, kind, nodes)
		return ProjectNode{}
	}
	assertTarget := func(kind, scope, workspaceRoot, topicID, wantPath string, wantTurns *int, wantScene scene.Scene, wantSceneState scene.State) {
		t.Helper()
		node := findTopicNode(kind, topicID)
		if node.SessionPath != wantPath {
			t.Fatalf("tree target for %s/%s = %q, want %q", scope, topicID, node.SessionPath, wantPath)
		}
		actorRoot := workspaceRoot
		if scope == "global" {
			actorRoot = globalWorkspaceRoot()
		}
		actor := app.activeConfigActorForWorkspace(actorRoot)
		if openedPath, _ := app.findTopicSessionForTargetForActor(scope, workspaceRoot, topicID, actor); openedPath != node.SessionPath {
			t.Fatalf("tree target for %s/%s = %q, concrete open target = %q", scope, topicID, node.SessionPath, openedPath)
		}
		if wantTurns == nil {
			if node.SessionTurns != nil {
				t.Fatalf("tree target turns for %s/%s = %v, want missing", scope, topicID, *node.SessionTurns)
			}
		} else if node.SessionTurns == nil || *node.SessionTurns != *wantTurns {
			t.Fatalf("tree target turns for %s/%s = %v, want %d", scope, topicID, node.SessionTurns, *wantTurns)
		}
		if node.Scene != string(wantScene) || node.SceneState != string(wantSceneState) {
			t.Fatalf("tree target scene for %s/%s = %q/%q, want %q/%q", scope, topicID, node.Scene, node.SceneState, wantScene, wantSceneState)
		}
	}
	zero, one := 0, 1

	assertTarget("topic", "project", projectRoot, projectSingle.ID, projectSinglePath, &one, scene.Coding, scene.Draft)
	assertTarget("global_topic", "global", "", globalSingle.ID, globalSinglePath, &one, scene.Office, scene.Locked)
	assertTarget("topic", "project", projectRoot, projectLatestEmpty.ID, projectLatestEmptyPath, &zero, scene.Design, scene.Draft)
	assertTarget("global_topic", "global", "", globalLatestEmpty.ID, globalLatestEmptyPath, &zero, scene.Coding, scene.Draft)
	assertTarget("topic", "project", projectRoot, projectTie.ID, projectTieEarlierPath, &one, scene.Office, scene.Draft)
	assertTarget("topic", "project", projectRoot, projectLegacy.ID, projectLegacyPath, &zero, scene.Office, scene.Draft)
	assertTarget("global_topic", "global", "", globalInvalid.ID, globalInvalidPath, &zero, scene.Office, scene.Draft)
	assertTarget("topic", "project", projectRoot, projectBlank.ID, "", nil, scene.Unset, scene.StateUnset)
	assertTarget("global_topic", "global", "", globalBlank.ID, "", nil, scene.Unset, scene.StateUnset)
	zeroPayload, err := json.Marshal(findTopicNode("topic", projectLatestEmpty.ID))
	if err != nil {
		t.Fatalf("marshal zero-turn topic node: %v", err)
	}
	if !strings.Contains(string(zeroPayload), `"sessionTurns":0`) {
		t.Fatalf("zero-turn topic JSON = %s, want explicit sessionTurns=0", zeroPayload)
	}
	pathlessPayload, err := json.Marshal(findTopicNode("topic", projectBlank.ID))
	if err != nil {
		t.Fatalf("marshal pathless topic node: %v", err)
	}
	if strings.Contains(string(pathlessPayload), `"sessionTurns"`) {
		t.Fatalf("pathless topic JSON = %s, want sessionTurns omitted", pathlessPayload)
	}
	if projectTieEarlierPath >= projectTieLaterPath {
		t.Fatalf("tie-break fixture paths are not ordered: earlier=%q later=%q", projectTieEarlierPath, projectTieLaterPath)
	}

	projectSessionCache.invalidate()
	invalidateTopicSessionIndex(projectDir)
	invalidateTopicSessionIndex(globalDir)
	nodes = NewApp().ListProjectTree()
	assertTarget("topic", "project", projectRoot, projectLatestEmpty.ID, projectLatestEmptyPath, &zero, scene.Design, scene.Draft)
	assertTarget("global_topic", "global", "", globalLatestEmpty.ID, globalLatestEmptyPath, &zero, scene.Coding, scene.Draft)
}

func TestListProjectTreeRuntimeSessionRowsPreferLiveExactTurns(t *testing.T) {
	for _, tc := range []struct {
		name      string
		scope     string
		childKind string
	}{
		{name: "project", scope: "project", childKind: "session"},
		{name: "global", scope: "global", childKind: "global_session"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateDesktopUserDirs(t)

			app := NewApp()
			workspaceRoot := ""
			if tc.scope == "project" {
				workspaceRoot = t.TempDir()
			}
			topic, err := app.CreateTopic(tc.scope, workspaceRoot, tc.name+" runtime sessions")
			if err != nil {
				t.Fatalf("create topic: %v", err)
			}
			sessionRoot := workspaceRoot
			if tc.scope == "global" {
				sessionRoot = globalWorkspaceRoot()
			}
			dir := desktopSessionDir(sessionRoot)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatalf("mkdir session dir: %v", err)
			}
			base := time.Date(2026, 7, 27, 14, 0, 0, 0, time.UTC)
			blankPath := writeEmptyTopicSession(t, dir, "blank.jsonl", topic.ID, topic.Title, workspaceRoot, base)
			contentPath := writeTopicSessionWithPrompt(t, dir, "content.jsonl", topic.ID, topic.Title, workspaceRoot, "content", base.Add(-time.Minute))
			setTopicSessionScene(t, blankPath, scene.Status{Scene: scene.Coding, State: scene.Draft})
			setTopicSessionScene(t, contentPath, scene.Status{Scene: scene.Office, State: scene.Locked})
			blankCtrl := control.New(control.Options{SessionDir: dir, SessionPath: blankPath, WorkspaceRoot: workspaceRoot, Label: "blank"})
			contentCtrl := control.New(control.Options{SessionDir: dir, SessionPath: contentPath, WorkspaceRoot: workspaceRoot, Label: "content"})
			t.Cleanup(blankCtrl.Close)
			t.Cleanup(contentCtrl.Close)
			app.tabRuntimes = map[string]*WorkspaceTab{
				"blank": {
					ID: "blank", Scope: tc.scope, WorkspaceRoot: workspaceRoot, TopicID: topic.ID,
					TopicTitle: topic.Title, SessionPath: blankPath, sessionTurns: intPointer(1),
					Scene: scene.Design, SceneState: scene.Draft, Ctrl: blankCtrl, Ready: true,
				},
				"content": {
					ID: "content", Scope: tc.scope, WorkspaceRoot: workspaceRoot, TopicID: topic.ID,
					TopicTitle: topic.Title, SessionPath: contentPath,
					Scene: scene.Coding, SceneState: scene.Locked, Ctrl: contentCtrl, Ready: true,
				},
			}
			app.tabOrder = []string{"blank", "content"}
			app.activeTabID = "blank"

			var topicNode *ProjectNode
			for _, parent := range app.ListProjectTree() {
				for i := range parent.Children {
					if parent.Children[i].TopicID == topic.ID {
						topicNode = &parent.Children[i]
						break
					}
				}
			}
			if topicNode == nil || len(topicNode.Children) != 2 {
				t.Fatalf("runtime session rows = %#v, want two children", topicNode)
			}
			if topicNode.SessionPath != blankPath {
				t.Fatalf("topic target path = %q, want latest runtime path %q", topicNode.SessionPath, blankPath)
			}
			if topicNode.SessionTurns == nil || *topicNode.SessionTurns != 1 {
				t.Fatalf("topic target turns = %v, want live runtime value 1", topicNode.SessionTurns)
			}
			if topicNode.Scene != string(scene.Design) || topicNode.SceneState != string(scene.Draft) {
				t.Fatalf("topic target scene = %q/%q, want live runtime design/draft", topicNode.Scene, topicNode.SceneState)
			}
			wantTurns := map[string]int{blankPath: 1, contentPath: 1}
			wantScenes := map[string]scene.Status{
				blankPath:   {Scene: scene.Design, State: scene.Draft},
				contentPath: {Scene: scene.Coding, State: scene.Locked},
			}
			for _, child := range topicNode.Children {
				if child.Kind != tc.childKind {
					t.Fatalf("runtime child kind = %q, want %q", child.Kind, tc.childKind)
				}
				want, ok := wantTurns[child.SessionPath]
				if !ok {
					t.Fatalf("unexpected runtime child path %q", child.SessionPath)
				}
				if child.SessionTurns == nil || *child.SessionTurns != want {
					t.Fatalf("runtime child turns for %q = %v, want %d", child.SessionPath, child.SessionTurns, want)
				}
				wantScene := wantScenes[child.SessionPath]
				if child.Scene != string(wantScene.Scene) || child.SceneState != string(wantScene.State) {
					t.Fatalf("runtime child scene for %q = %q/%q, want %q/%q", child.SessionPath, child.Scene, child.SceneState, wantScene.Scene, wantScene.State)
				}
			}
		})
	}
}

func TestListProjectTreeUsesSoleUnindexedRuntimeSessionAsTopicTarget(t *testing.T) {
	for _, tc := range []struct {
		name  string
		scope string
	}{
		{name: "project", scope: "project"},
		{name: "global", scope: "global"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateDesktopUserDirs(t)

			app := NewApp()
			workspaceRoot := ""
			if tc.scope == "project" {
				workspaceRoot = t.TempDir()
			}
			topic, err := app.CreateTopic(tc.scope, workspaceRoot, defaultTopicTitle)
			if err != nil {
				t.Fatalf("create topic: %v", err)
			}
			sessionRoot := workspaceRoot
			if tc.scope == "global" {
				sessionRoot = globalWorkspaceRoot()
			}
			dir := desktopSessionDir(sessionRoot)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatalf("mkdir session dir: %v", err)
			}
			sessionPath := filepath.Join(dir, "live-unindexed.jsonl")
			ctrl := control.New(control.Options{
				SessionDir:    dir,
				SessionPath:   sessionPath,
				WorkspaceRoot: workspaceRoot,
				Label:         "live-unindexed",
			})
			t.Cleanup(ctrl.Close)
			app.tabRuntimes = map[string]*WorkspaceTab{
				"live": {
					ID: "live", Scope: tc.scope, WorkspaceRoot: workspaceRoot, TopicID: topic.ID,
					TopicTitle: defaultTopicTitle, SessionPath: sessionPath, sessionTurns: intPointer(1),
					Scene: scene.Design, SceneState: scene.Draft, Ctrl: ctrl, Ready: true,
				},
			}
			app.tabOrder = []string{"live"}
			app.activeTabID = "live"

			var topicNode *ProjectNode
			for _, parent := range app.ListProjectTree() {
				for i := range parent.Children {
					if parent.Children[i].TopicID == topic.ID {
						topicNode = &parent.Children[i]
						break
					}
				}
			}
			if topicNode == nil {
				t.Fatal("missing topic node for sole live runtime")
			}
			if topicNode.SessionPath != sessionPath {
				t.Fatalf("topic target path = %q, want sole live path %q", topicNode.SessionPath, sessionPath)
			}
			if topicNode.SessionTurns == nil || *topicNode.SessionTurns != 1 {
				t.Fatalf("topic target turns = %v, want accepted live turn count 1", topicNode.SessionTurns)
			}
			if topicNode.Scene != string(scene.Design) || topicNode.SceneState != string(scene.Draft) {
				t.Fatalf("topic target scene = %q/%q, want live design/draft", topicNode.Scene, topicNode.SceneState)
			}
			if len(topicNode.Children) != 0 {
				t.Fatalf("sole live runtime children = %d, want folded topic target", len(topicNode.Children))
			}
		})
	}
}

func TestListProjectTreeCachesConcreteOpenTargetsUntilInvalidated(t *testing.T) {
	isolateDesktopUserDirs(t)

	oldCache := projectSessionCache
	projectSessionCache = &sessionListCache{byDir: map[string]sessionListCacheEntry{}}
	oldLoader := loadLatestTopicSessionSelectionsForDir
	var loads atomic.Int32
	loadLatestTopicSessionSelectionsForDir = func(dir string) (map[string]topicSessionSelection, map[string]int, error) {
		loads.Add(1)
		return oldLoader(dir)
	}
	t.Cleanup(func() {
		loadLatestTopicSessionSelectionsForDir = oldLoader
		projectSessionCache = oldCache
	})

	app := NewApp()
	projectRoot := t.TempDir()
	topic, err := app.CreateTopic("project", projectRoot, "Cached target")
	if err != nil {
		t.Fatalf("create cached target topic: %v", err)
	}
	dir := desktopSessionDir(projectRoot)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir project session dir: %v", err)
	}
	sessionPath := writeTopicSessionWithPrompt(t, dir, "cached-target.jsonl", topic.ID, topic.Title, projectRoot, "cached prompt", time.Now())

	nodes := app.ListProjectTree()
	if len(nodes) != 1 || len(nodes[0].Children) != 1 || nodes[0].Children[0].SessionPath != sessionPath {
		t.Fatalf("first project tree target = %#v, want %q", nodes, sessionPath)
	}
	firstLoads := loads.Load()
	if firstLoads == 0 {
		t.Fatal("first project tree load did not resolve concrete targets")
	}

	nodes = app.ListProjectTree()
	if got := loads.Load(); got != firstLoads {
		t.Fatalf("cached project tree refresh performed %d new target loads, want 0", got-firstLoads)
	}
	if nodes[0].Children[0].SessionPath != sessionPath {
		t.Fatalf("cached project tree target = %q, want %q", nodes[0].Children[0].SessionPath, sessionPath)
	}

	projectSessionCache.invalidate()
	nodes = app.ListProjectTree()
	if got := loads.Load(); got <= firstLoads {
		t.Fatalf("invalidated project tree target loads = %d, want more than %d", got, firstLoads)
	}
	if nodes[0].Children[0].SessionPath != sessionPath {
		t.Fatalf("reloaded project tree target = %q, want %q", nodes[0].Children[0].SessionPath, sessionPath)
	}
}

func TestAcceptedTurnImmediatelyAdvancesOldTopicActivity(t *testing.T) {
	isolateDesktopUserDirs(t)

	app := NewApp()
	projectRoot := t.TempDir()
	oldTopic, err := app.CreateTopic("project", projectRoot, "旧任务")
	if err != nil {
		t.Fatalf("create old topic: %v", err)
	}
	recentTopic, err := app.CreateTopic("project", projectRoot, "较新任务")
	if err != nil {
		t.Fatalf("create recent topic: %v", err)
	}
	sessionDir := desktopSessionDir(projectRoot)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	oldPath := writeTopicSessionWithPrompt(t, sessionDir, "old-topic.jsonl", oldTopic.ID, oldTopic.Title, projectRoot, "old", time.Now().Add(-time.Hour))
	writeTopicSessionWithPrompt(t, sessionDir, "recent-topic.jsonl", recentTopic.ID, recentTopic.Title, projectRoot, "recent", time.Now().Add(-time.Minute))

	ctrl := control.New(control.Options{SessionDir: sessionDir, SessionPath: oldPath})
	t.Cleanup(ctrl.Close)
	tab := &WorkspaceTab{
		ID:            "old-topic-tab",
		Scope:         "project",
		WorkspaceRoot: projectRoot,
		TopicID:       oldTopic.ID,
		TopicTitle:    oldTopic.Title,
		SessionPath:   oldPath,
		Ctrl:          ctrl,
		Ready:         true,
	}
	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.activeTabID = tab.ID

	activityByTopic := func(nodes []ProjectNode) map[string]int64 {
		activity := map[string]int64{}
		for _, parent := range nodes {
			for _, child := range parent.Children {
				activity[child.TopicID] = child.LastActivityAt
			}
		}
		return activity
	}
	before := activityByTopic(app.ListProjectTree())
	if before[oldTopic.ID] >= before[recentTopic.ID] {
		t.Fatalf("initial activity old=%d recent=%d, want old topic earlier", before[oldTopic.ID], before[recentTopic.ID])
	}

	var treeChanges atomic.Int32
	app.projectTreeChangedHook = func() { treeChanges.Add(1) }
	app.recordAcceptedTabSessionTurn(ctrl)
	after := activityByTopic(app.ListProjectTree())
	if after[oldTopic.ID] <= after[recentTopic.ID] {
		t.Fatalf("accepted activity old=%d recent=%d, want old topic newest", after[oldTopic.ID], after[recentTopic.ID])
	}
	if treeChanges.Load() != 1 {
		t.Fatalf("project tree changes = %d, want 1 immediate refresh", treeChanges.Load())
	}
}

func TestAcceptedFirstLaunchTurnMaterializesTaskBeforeCompletion(t *testing.T) {
	isolateDesktopUserDirs(t)

	app := NewApp()
	actor := userctx.Default(globalWorkspaceRoot())
	sessionDir := desktopSessionDirForActor(actor, globalWorkspaceRoot())
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	sessionPath := agent.NewSessionPath(sessionDir, "first-launch")
	runner := instantTitleBlockingRunner{started: make(chan struct{}), release: make(chan struct{})}
	ctrl := control.New(control.Options{
		Runner:        runner,
		SessionDir:    sessionDir,
		SessionPath:   sessionPath,
		WorkspaceRoot: globalWorkspaceRoot(),
	})
	t.Cleanup(ctrl.Close)
	tab := &WorkspaceTab{
		ID:            "first-launch",
		Scope:         "global",
		WorkspaceRoot: globalTabWorkspaceRoot(),
		TopicTitle:    "Global",
		SessionPath:   sessionPath,
		Ctrl:          ctrl,
		Ready:         true,
		actor:         actor,
	}
	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID
	app.bindControllerDisplayRecorder(ctrl)

	if nodes := app.ListProjectTree(); len(nodes) != 1 || nodes[0].Kind != "global_folder" || len(nodes[0].Children) != 0 {
		t.Fatalf("untouched first-launch tree = %#v, want empty Global folder", nodes)
	}

	firstRefresh := make(chan []ProjectNode, 1)
	app.projectTreeChangedHook = func() {
		select {
		case firstRefresh <- app.ListProjectTree():
		default:
		}
	}
	done := make(chan error, 1)
	go func() {
		done <- ctrl.RunTurnDisplay(context.Background(), "first task", "first task")
	}()
	select {
	case <-runner.started:
	case <-time.After(time.Second):
		t.Fatal("first turn did not reach runner")
	}

	var runningTree []ProjectNode
	select {
	case runningTree = <-firstRefresh:
	case <-time.After(time.Second):
		t.Fatal("accepted first turn did not refresh project tree")
	}
	wantTopicID := legacySessionTopicID(sessionPath)
	if tab.TopicID != wantTopicID {
		t.Fatalf("first-launch topic ID = %q, want %q", tab.TopicID, wantTopicID)
	}
	if len(runningTree) != 1 || runningTree[0].Kind != "global_folder" || len(runningTree[0].Children) != 1 {
		t.Fatalf("running first-launch tree = %#v, want one Global task", runningTree)
	}
	task := runningTree[0].Children[0]
	if task.TopicID != wantTopicID || task.SessionPath != sessionPath || task.SessionTurns == nil || *task.SessionTurns != 1 || !task.Running {
		t.Fatalf("running first-launch task = %#v, want bound running turn", task)
	}
	meta, ok, err := agent.LoadBranchMeta(sessionPath)
	if err != nil || !ok {
		t.Fatalf("load first-launch metadata: ok=%v err=%v", ok, err)
	}
	if meta.TopicID != wantTopicID || meta.TopicTitle != tab.TopicTitle {
		t.Fatalf("first-launch metadata = %+v, want topic %q title %q", meta, wantTopicID, tab.TopicTitle)
	}
	if target, ok := app.acceptedTopicTitleTarget(ctrl); !ok || target.topicID != wantTopicID || target.sessionPath != sessionPath {
		t.Fatalf("accepted title target = %+v, ok=%v", target, ok)
	}

	close(runner.release)
	if err := <-done; err != nil {
		t.Fatalf("run first turn: %v", err)
	}
	completedTree := app.ListProjectTree()
	if len(completedTree) != 1 || len(completedTree[0].Children) != 1 || completedTree[0].Children[0].TopicID != wantTopicID {
		t.Fatalf("completed first-launch tree = %#v, want one stable task", completedTree)
	}
}

func TestListProjectTreeUsesActiveProfileSessionDirs(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topicID := "topic_profile_tree"
	if err := addProject(projectRoot, ""); err != nil {
		t.Fatalf("add project: %v", err)
	}
	if err := setTopicTitle(projectRoot, topicID, "Profile tree"); err != nil {
		t.Fatalf("set topic title: %v", err)
	}
	defaultDir := config.SessionDir()
	if err := os.MkdirAll(defaultDir, 0o755); err != nil {
		t.Fatalf("mkdir default sessions: %v", err)
	}
	writeTopicSessionWithPrompt(t, defaultDir, "default-tree.jsonl", topicID, "Profile tree", projectRoot, "default prompt", time.Now())
	app, _ := newProfileProjectTopicApp(t, projectRoot, "profile-b")

	nodes := app.ListProjectTree()
	if len(nodes) != 1 || len(nodes[0].Children) != 1 {
		t.Fatalf("project tree = %#v, want one project topic", nodes)
	}
	topic := nodes[0].Children[0]
	if topic.TopicID != topicID {
		t.Fatalf("topic ID = %q, want %q", topic.TopicID, topicID)
	}
	if topic.Turns != 0 || topic.LastActivityAt != 0 {
		t.Fatalf("default profile session leaked into profile tree summary: %+v", topic)
	}
}

func TestOpenTopicSessionRejectsDefaultProfilePathForActiveProfile(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topicID := "topic_profile_open"
	if err := addProject(projectRoot, ""); err != nil {
		t.Fatalf("add project: %v", err)
	}
	if err := setTopicTitle(projectRoot, topicID, "Profile open"); err != nil {
		t.Fatalf("set topic title: %v", err)
	}
	defaultDir := config.SessionDir()
	if err := os.MkdirAll(defaultDir, 0o755); err != nil {
		t.Fatalf("mkdir default sessions: %v", err)
	}
	defaultPath := writeTopicSessionWithPrompt(t, defaultDir, "default-open.jsonl", topicID, "Profile open", projectRoot, "default prompt", time.Now())
	app, _ := newProfileProjectTopicApp(t, projectRoot, "profile-b")

	if _, err := app.OpenTopicSession("project", projectRoot, topicID, defaultPath); err == nil {
		t.Fatalf("OpenTopicSession accepted default profile session path %q", defaultPath)
	}
}

func TestTrashTopicLeavesDefaultProfileSessionForActiveProfile(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topicID := "topic_profile_trash"
	if err := addProject(projectRoot, ""); err != nil {
		t.Fatalf("add project: %v", err)
	}
	if err := setTopicTitle(projectRoot, topicID, "Profile trash"); err != nil {
		t.Fatalf("set topic title: %v", err)
	}
	defaultDir := config.SessionDir()
	if err := os.MkdirAll(defaultDir, 0o755); err != nil {
		t.Fatalf("mkdir default sessions: %v", err)
	}
	defaultPath := writeTopicSessionWithPrompt(t, defaultDir, "default-trash.jsonl", topicID, "Profile trash", projectRoot, "default prompt", time.Now())
	app, _ := newProfileProjectTopicApp(t, projectRoot, "profile-b")

	if err := app.TrashTopic(topicID); err != nil {
		t.Fatalf("trash topic: %v", err)
	}
	if _, err := os.Stat(defaultPath); err != nil {
		t.Fatalf("default profile session should remain live, stat err = %v", err)
	}
	trashPath := filepath.Join(defaultDir, sessionTrashDir, "default-trash.jsonl", "default-trash.jsonl")
	if _, err := os.Stat(trashPath); !os.IsNotExist(err) {
		t.Fatalf("default profile session should not move to trash, stat err = %v", err)
	}
}

func TestTrashTopicLeavesOtherProfileRuntimeTabForActiveProfile(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topicID := "topic_profile_runtime_trash"
	if err := addProject(projectRoot, ""); err != nil {
		t.Fatalf("add project: %v", err)
	}
	if err := setTopicTitle(projectRoot, topicID, "Runtime trash"); err != nil {
		t.Fatalf("set topic title: %v", err)
	}
	defaultActor := userctx.Default(projectRoot)
	defaultDir := config.NewProfilePathResolver(defaultActor).ProjectSessionDir(projectRoot)
	if err := os.MkdirAll(defaultDir, 0o755); err != nil {
		t.Fatalf("mkdir default profile sessions: %v", err)
	}
	defaultCtrl := control.New(control.Options{
		SessionDir:    defaultDir,
		SessionPath:   filepath.Join(defaultDir, "default-runtime.jsonl"),
		Label:         "default",
		Actor:         defaultActor,
		WorkspaceRoot: projectRoot,
	})
	t.Cleanup(defaultCtrl.Close)

	actorB := userctx.Default(projectRoot)
	actorB.ProfileID = "profile-b"
	dirB := config.NewProfilePathResolver(actorB).ProjectSessionDir(projectRoot)
	if err := os.MkdirAll(dirB, 0o755); err != nil {
		t.Fatalf("mkdir profile-b sessions: %v", err)
	}
	ctrlB := control.New(control.Options{
		SessionDir:    dirB,
		SessionPath:   filepath.Join(dirB, "active-b.jsonl"),
		Label:         "profile-b",
		Actor:         actorB,
		WorkspaceRoot: projectRoot,
	})
	t.Cleanup(ctrlB.Close)
	app := &App{
		tabRuntimes: map[string]*WorkspaceTab{
			"default": {ID: "default", Scope: "project", WorkspaceRoot: projectRoot, TopicID: topicID, Ctrl: defaultCtrl, Ready: true, actor: defaultActor},
			"b":       {ID: "b", Scope: "project", WorkspaceRoot: projectRoot, Ctrl: ctrlB, Ready: true, actor: actorB},
		},
		tabOrder:    []string{"default", "b"},
		activeTabID: "b",
	}

	if err := app.TrashTopic(topicID); err != nil {
		t.Fatalf("trash topic: %v", err)
	}
	app.mu.RLock()
	_, defaultOpen := app.tabRuntimes["default"]
	app.mu.RUnlock()
	if !defaultOpen {
		t.Fatalf("TrashTopic removed a default-profile runtime tab while profile-b was active")
	}
}

func TestRenameTopicLeavesDefaultProfileBranchMetaForActiveProfile(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topicID := "topic_profile_rename"
	if err := addProject(projectRoot, ""); err != nil {
		t.Fatalf("add project: %v", err)
	}
	if err := setTopicTitle(projectRoot, topicID, "Old profile title"); err != nil {
		t.Fatalf("set topic title: %v", err)
	}
	defaultDir := config.SessionDir()
	if err := os.MkdirAll(defaultDir, 0o755); err != nil {
		t.Fatalf("mkdir default sessions: %v", err)
	}
	defaultPath := writeTopicSessionWithPrompt(t, defaultDir, "default-rename.jsonl", topicID, "Old profile title", projectRoot, "default prompt", time.Now())
	app, _ := newProfileProjectTopicApp(t, projectRoot, "profile-b")

	if err := app.RenameTopic(topicID, "New profile title"); err != nil {
		t.Fatalf("rename topic: %v", err)
	}
	meta, ok, err := agent.LoadBranchMeta(defaultPath)
	if err != nil || !ok {
		t.Fatalf("load default branch meta: ok=%v err=%v", ok, err)
	}
	if meta.TopicTitle != "Old profile title" {
		t.Fatalf("default branch meta title = %q, want Old profile title", meta.TopicTitle)
	}
}

func TestRenameTopicLocksTitleManual(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	app := NewApp()
	topic, err := app.CreateTopic("project", projectRoot, "")
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	if err := app.RenameTopic(topic.ID, "手动标题"); err != nil {
		t.Fatalf("rename topic: %v", err)
	}
	if got := loadTopicTitle(projectRoot, topic.ID); got != "手动标题" {
		t.Fatalf("stored title = %q, want 手动标题", got)
	}
	if got := loadTopicTitleSource(projectRoot, topic.ID); got != topicTitleSourceManual {
		t.Fatalf("title source = %q, want manual", got)
	}
}

func TestRenameTopicUpdatesOpenTabMeta(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	app := NewApp()
	topic, err := app.CreateTopic("project", projectRoot, "旧标题")
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	tab, err := app.OpenProjectTab(projectRoot, topic.ID)
	if err != nil {
		t.Fatalf("open project tab: %v", err)
	}
	waitForTabReady(t, app, tab.ID)
	if tab.TopicTitle != "旧标题" {
		t.Fatalf("opened tab title = %q, want 旧标题", tab.TopicTitle)
	}

	if err := app.RenameTopic(topic.ID, "新标题"); err != nil {
		t.Fatalf("rename topic: %v", err)
	}
	tabs := app.ListTabs()
	if len(tabs) != 1 {
		t.Fatalf("tabs len = %d, want 1: %+v", len(tabs), tabs)
	}
	if got := tabs[0].TopicTitle; got != "新标题" {
		t.Fatalf("open tab title = %q, want 新标题", got)
	}
}

func TestRenameTopicRecreatesDeletedProjectTitleIndexFromOpenTab(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	app := NewApp()
	topic, err := app.CreateTopic("project", projectRoot, "旧标题")
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	tab, err := app.OpenProjectTab(projectRoot, topic.ID)
	if err != nil {
		t.Fatalf("open project tab: %v", err)
	}
	waitForTabReady(t, app, tab.ID)
	titlesPath, err := topicTitlesPath(projectRoot)
	if err != nil {
		t.Fatalf("resolve topic titles: %v", err)
	}
	sourcesPath, err := topicTitleSourcesPath(projectRoot)
	if err != nil {
		t.Fatalf("resolve topic title sources: %v", err)
	}
	if err := os.Remove(titlesPath); err != nil {
		t.Fatalf("remove topic titles: %v", err)
	}
	if err := os.Remove(sourcesPath); err != nil {
		t.Fatalf("remove topic title sources: %v", err)
	}

	if err := app.RenameTopic(topic.ID, "恢复标题"); err != nil {
		t.Fatalf("rename topic after deleting title index: %v", err)
	}
	if got := loadTopicTitle(projectRoot, topic.ID); got != "恢复标题" {
		t.Fatalf("restored topic title = %q, want 恢复标题", got)
	}
	nodes := app.ListProjectTree()
	if len(nodes) != 1 || len(nodes[0].Children) != 1 || nodes[0].Children[0].TopicID != topic.ID {
		t.Fatalf("project tree should still contain topic, got %#v", nodes)
	}
}

func TestRenameTopicRecreatesDeletedProjectTitleIndexFromSessionMeta(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topicID := "topic_missing_index"
	if err := addProject(projectRoot, ""); err != nil {
		t.Fatalf("add project: %v", err)
	}
	if err := setTopicTitle(projectRoot, topicID, "旧标题"); err != nil {
		t.Fatalf("set topic title: %v", err)
	}
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	writeTopicSession(t, dir, "missing-index.jsonl", topicID, "旧标题", projectRoot)
	titlesPath, err := topicTitlesPath(projectRoot)
	if err != nil {
		t.Fatalf("resolve topic titles: %v", err)
	}
	sourcesPath, err := topicTitleSourcesPath(projectRoot)
	if err != nil {
		t.Fatalf("resolve topic title sources: %v", err)
	}
	if err := os.Remove(titlesPath); err != nil {
		t.Fatalf("remove topic titles: %v", err)
	}
	if err := os.Remove(sourcesPath); err != nil {
		t.Fatalf("remove topic title sources: %v", err)
	}

	if err := NewApp().RenameTopic(topicID, "恢复标题"); err != nil {
		t.Fatalf("rename topic from session meta after deleting title index: %v", err)
	}
	if got := loadTopicTitle(projectRoot, topicID); got != "恢复标题" {
		t.Fatalf("restored topic title = %q, want 恢复标题", got)
	}
	nodes := NewApp().ListProjectTree()
	if len(nodes) != 1 || len(nodes[0].Children) != 1 || nodes[0].Children[0].TopicID != topicID {
		t.Fatalf("project tree should contain restored topic, got %#v", nodes)
	}
}

func TestEnsureTopicIndexedPreservesGlobalAutoTitleSource(t *testing.T) {
	isolateDesktopUserDirs(t)

	topicID := "topic_global_auto"
	if err := setTopicTitleWithSource("", topicID, defaultTopicTitle, topicTitleSourceAuto); err != nil {
		t.Fatalf("set global topic title: %v", err)
	}
	source := loadTopicTitleSource(topicTitleRoot("global", globalTabWorkspaceRoot()), topicID)
	if err := ensureTopicIndexed("global", globalTabWorkspaceRoot(), topicID, defaultTopicTitle, source); err != nil {
		t.Fatalf("ensure global topic indexed: %v", err)
	}

	if got := loadTopicTitleSource("", topicID); got != topicTitleSourceAuto {
		t.Fatalf("global title source = %q, want %q", got, topicTitleSourceAuto)
	}
}

func TestAutoTitleTopicFromFirstUserMessage(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topic, err := NewApp().CreateTopic("project", projectRoot, "")
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(sessionPath, []byte(`{"role":"user","content":"讲讲这个代码库的架构"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	title, updated := autoTitleTopicFromSession(projectRoot, topic.ID, sessionPath)
	if !updated {
		t.Fatal("auto title should update")
	}
	if title != "讲讲这个代码库的架构" {
		t.Fatalf("generated title = %q", title)
	}
	if got := loadTopicTitle(projectRoot, topic.ID); got != title {
		t.Fatalf("stored title = %q, want %q", got, title)
	}
	if got := loadTopicTitleSource(projectRoot, topic.ID); got != topicTitleSourceAuto {
		t.Fatalf("title source = %q, want auto", got)
	}
}

type instantTitleBlockingRunner struct {
	started chan struct{}
	release chan struct{}
}

func (r instantTitleBlockingRunner) Run(context.Context, string) error {
	close(r.started)
	<-r.release
	return nil
}

func TestAcceptedUserTurnAutoTitlesWhileTurnIsRunningAndPrefersDisplay(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	app := NewApp()
	topic, err := app.CreateTopic("project", projectRoot, "")
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	actor := userctx.Default(projectRoot)
	sessionDir := desktopSessionDirForActor(actor, projectRoot)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	sessionPath := writeTopicSessionWithPrompt(t, sessionDir, "instant-title.jsonl", topic.ID, defaultTopicTitle, projectRoot, "old prompt", time.Now())
	runner := instantTitleBlockingRunner{started: make(chan struct{}), release: make(chan struct{})}
	ctrl := control.New(control.Options{Runner: runner, SessionDir: sessionDir, SessionPath: sessionPath})
	tab := &WorkspaceTab{
		ID:            "instant-title",
		Scope:         "project",
		WorkspaceRoot: projectRoot,
		TopicID:       topic.ID,
		TopicTitle:    defaultTopicTitle,
		Ctrl:          ctrl,
		Ready:         true,
		actor:         actor,
	}
	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.activeTabID = tab.ID
	app.bindControllerDisplayRecorder(ctrl)

	done := make(chan error, 1)
	go func() {
		done <- ctrl.RunTurnDisplay(context.Background(), "可见的即时标题", "hidden expanded context that must not become the title")
	}()
	select {
	case <-runner.started:
	case <-time.After(time.Second):
		t.Fatal("turn did not reach runner")
	}

	deadline := time.Now().Add(2 * time.Second)
	for loadTopicTitle(projectRoot, topic.ID) != "可见的即时标题" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := loadTopicTitle(projectRoot, topic.ID); got != "可见的即时标题" {
		t.Fatalf("topic title while turn is running = %q, want display text", got)
	}
	if !ctrl.Running() {
		t.Fatal("turn finished before the immediate title assertion")
	}
	if got := tab.TopicTitle; got != "可见的即时标题" {
		t.Fatalf("open tab title = %q, want immediate title", got)
	}
	var meta agent.BranchMeta
	var ok bool
	for time.Now().Before(deadline) {
		meta, ok, err = agent.LoadBranchMeta(sessionPath)
		if err == nil && ok && meta.TopicTitle == "可见的即时标题" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil || !ok || meta.TopicTitle != "可见的即时标题" {
		t.Fatalf("session sidecar after immediate title = %+v ok=%v err=%v", meta, ok, err)
	}

	close(runner.release)
	if err := <-done; err != nil {
		t.Fatalf("run turn: %v", err)
	}
}

func TestSkillTurnAutoTitleUsesConversationTextWithoutSkillToken(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	app := NewApp()
	topic, err := app.CreateTopic("project", projectRoot, "")
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	app.autoTitleAcceptedTopicTurn(acceptedTopicTitleTarget{
		topicID:       topic.ID,
		workspaceRoot: projectRoot,
		referenceRoot: projectRoot,
		actor:         userctx.Default(projectRoot),
	}, "11", "/document 11", "11", true)

	if got := loadTopicTitle(projectRoot, topic.ID); got != "11" {
		t.Fatalf("skill turn title = %q, want conversation text only", got)
	}
}

func TestSkillTurnAutoTitleFallbackUsesPersistedConversationText(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topic, err := NewApp().CreateTopic("project", projectRoot, "")
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	sessionDir := t.TempDir()
	sessionPath := filepath.Join(sessionDir, "skill-title.jsonl")
	content := `<selected-skill name="document">Use the document workflow.</selected-skill>

11`
	if err := os.WriteFile(sessionPath, []byte(`{"role":"user","content":`+strconv.Quote(content)+`}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}
	if err := recordSessionDisplay(sessionDir, sessionPath, content, "/document 11"); err != nil {
		t.Fatalf("record display: %v", err)
	}

	title, updated := autoTitleTopicFromSession(projectRoot, topic.ID, sessionPath)
	if !updated || title != "11" {
		t.Fatalf("fallback skill title = %q updated=%v, want conversation text only", title, updated)
	}
}

func TestAcceptedTopicTitleTargetFindsDetachedRuntime(t *testing.T) {
	app := NewApp()
	ctrl := control.New(control.Options{})
	tab := &WorkspaceTab{
		ID:      "detached",
		Scope:   "global",
		TopicID: "topic-detached",
		Ctrl:    ctrl,
	}
	app.tabRuntimes = map[string]*WorkspaceTab{}
	app.detachedSessions = map[string]*WorkspaceTab{"session": tab}

	target, ok := app.acceptedTopicTitleTarget(ctrl)
	if !ok || target.topicID != tab.TopicID {
		t.Fatalf("detached target = %+v, ok=%v", target, ok)
	}
	app.updateOpenTopicTitle(tab.TopicID, "detached title")
	if tab.TopicTitle != "detached title" {
		t.Fatalf("detached runtime title = %q, want updated title", tab.TopicTitle)
	}
	if target, ok := app.acceptedTopicTitleTarget(ctrl); ok {
		t.Fatalf("already named runtime produced auto-title target: %+v", target)
	}
}

func TestTopicTitleNormalizerUsesDisplayWidthAndRealFileRefs(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("notes"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := topicTitleFromTextInWorkspace("@notes.txt", root); got != "" {
		t.Fatalf("file-only title = %q, want empty", got)
	}
	if got := topicTitleFromTextInWorkspace("@alice 请检查 @notes.txt", root); got != "@alice 请检查" {
		t.Fatalf("mention + file title = %q, want mention preserved and file removed", got)
	}
	if got := topicTitleFromTextInWorkspace(filepath.Join(root, "notes.txt"), root); got != "" {
		t.Fatalf("bare file title = %q, want empty", got)
	}
	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, "cwd-only.txt"), []byte("cwd"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(cwd)
	if got := topicTitleFromTextInWorkspace("@cwd-only.txt", t.TempDir()); got != "@cwd-only.txt" {
		t.Fatalf("cross-workspace cwd reference = %q, want literal mention", got)
	}

	chinese := topicTitleFromText("实现新的会话即时自动命名功能，并确保手工标题优先于异步自动更新")
	if chinese != "实现新的会话即时自动命名功能…" {
		t.Fatalf("Chinese natural-boundary title = %q", chinese)
	}
	english := topicTitleFromText("Implement the new topic title behavior across desktop surfaces")
	if runewidth.StringWidth(english) > topicTitleMaxWidth || !strings.HasSuffix(english, "…") || strings.HasSuffix(english, "behavio…") {
		t.Fatalf("English word-boundary title = %q width=%d", english, runewidth.StringWidth(english))
	}

	bounded := boundedUTF8Prefix(strings.Repeat("a", topicTitleInputBytes-1)+"中suffix", topicTitleInputBytes)
	if len(bounded) > topicTitleInputBytes || !utf8.ValidString(bounded) {
		t.Fatalf("bounded prefix is not UTF-8 safe: len=%d valid=%v", len(bounded), utf8.ValidString(bounded))
	}
}

func TestAutoTitleFallbackSkipsAttachmentOnlyUserMessage(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectRoot, "notes.txt"), []byte("notes"), 0o644); err != nil {
		t.Fatal(err)
	}
	topic, err := NewApp().CreateTopic("project", projectRoot, "")
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
	data := `{"role":"user","content":"@.clouvia/attachments/only.png"}` + "\n" +
		`{"role":"assistant","content":"ok"}` + "\n" +
		`{"role":"user","content":"Host final-answer readiness check failed. Continue the same task."}` + "\n" +
		`{"role":"user","content":"第二条有效文字需求"}` + "\n"
	if err := os.WriteFile(sessionPath, []byte(data), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	title, updated := autoTitleTopicFromSession(projectRoot, topic.ID, sessionPath)
	if !updated || title != "第二条有效文字需求" {
		t.Fatalf("fallback title = %q updated=%v", title, updated)
	}

	const globalTopicID = "global-attachment-fallback"
	if err := setTopicTitleWithSource("", globalTopicID, defaultTopicTitle, topicTitleSourceAuto); err != nil {
		t.Fatalf("seed global topic: %v", err)
	}
	globalSessionPath := filepath.Join(t.TempDir(), "global-session.jsonl")
	globalData := `{"role":"user","content":"@notes.txt"}` + "\n" +
		`{"role":"user","content":"全局会话有效文字"}` + "\n"
	if err := os.WriteFile(globalSessionPath, []byte(globalData), 0o644); err != nil {
		t.Fatalf("write global session: %v", err)
	}
	title, updated = autoTitleTopicFromSessionWithReferenceRoot("", projectRoot, globalTopicID, globalSessionPath)
	if !updated || title != "全局会话有效文字" {
		t.Fatalf("global fallback title = %q updated=%v", title, updated)
	}
}

func TestAutoTopicTitlePendingRequiresAutoDefaultTitle(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	tests := []struct {
		topicID string
		title   string
		source  string
		want    bool
	}{
		{topicID: "auto-default", title: defaultTopicTitle, source: topicTitleSourceAuto, want: true},
		{topicID: "auto-empty", title: "", source: topicTitleSourceAuto, want: true},
		{topicID: "auto-named", title: "已有自动标题", source: topicTitleSourceAuto},
		{topicID: "manual-default", title: defaultTopicTitle, source: topicTitleSourceManual},
	}
	for _, tt := range tests {
		if tt.title == "" {
			sources := loadTopicTitleSources(projectRoot)
			sources[tt.topicID] = tt.source
			if err := saveTopicTitleSources(projectRoot, sources); err != nil {
				t.Fatalf("seed empty %s: %v", tt.topicID, err)
			}
		} else if err := setTopicTitleWithSource(projectRoot, tt.topicID, tt.title, tt.source); err != nil {
			t.Fatalf("seed %s: %v", tt.topicID, err)
		}
		got, err := autoTopicTitlePending(projectRoot, tt.topicID)
		if err != nil {
			t.Fatalf("pending %s: %v", tt.topicID, err)
		}
		if got != tt.want {
			t.Fatalf("pending %s = %v, want %v", tt.topicID, got, tt.want)
		}
	}
}

func TestManualTopicTitleSourcePersistsBeforeTitleWriteFailure(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	const topicID = "manual-write-failure"
	if err := setTopicTitleWithSource(projectRoot, topicID, "旧自动标题", topicTitleSourceAuto); err != nil {
		t.Fatalf("seed auto title: %v", err)
	}
	originalSave := saveTopicTitlesForUpdate
	saveTopicTitlesForUpdate = func(string, map[string]string) error {
		return errors.New("injected title write failure")
	}
	t.Cleanup(func() { saveTopicTitlesForUpdate = originalSave })

	err := setTopicTitle(projectRoot, topicID, "手工标题")
	if err == nil || !strings.Contains(err.Error(), "save topic title") {
		t.Fatalf("manual title error = %v", err)
	}
	if got := loadTopicTitleSource(projectRoot, topicID); got != topicTitleSourceManual {
		t.Fatalf("title source after failure = %q, want manual", got)
	}
	if got := loadTopicTitle(projectRoot, topicID); got != "旧自动标题" {
		t.Fatalf("title after failure = %q, want old title", got)
	}
	if updated, tryErr := trySetAutoTopicTitle(projectRoot, topicID, "异步标题"); tryErr != nil || updated {
		t.Fatalf("auto update after manual-source persistence = updated %v err %v", updated, tryErr)
	}
}

func TestManualTitleWinsConcurrentAutomaticTitle(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	for i := 0; i < 20; i++ {
		topicID := fmt.Sprintf("concurrent-title-%d", i)
		if err := setTopicTitleWithSource(projectRoot, topicID, defaultTopicTitle, topicTitleSourceAuto); err != nil {
			t.Fatalf("seed auto title: %v", err)
		}
		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, _ = trySetAutoTopicTitle(projectRoot, topicID, "异步自动标题")
		}()
		go func() {
			defer wg.Done()
			<-start
			_ = setTopicTitle(projectRoot, topicID, "手工标题")
		}()
		close(start)
		wg.Wait()
		if got := loadTopicTitle(projectRoot, topicID); got != "手工标题" {
			t.Fatalf("iteration %d title = %q, want manual", i, got)
		}
		if got := loadTopicTitleSource(projectRoot, topicID); got != topicTitleSourceManual {
			t.Fatalf("iteration %d source = %q, want manual", i, got)
		}
	}
}

func TestAcceptedTurnTitleFailureLeavesTurnDoneFallbackAvailable(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	app := NewApp()
	topic, err := app.CreateTopic("project", projectRoot, "")
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	originalSave := saveTopicTitlesForUpdate
	saveTopicTitlesForUpdate = func(string, map[string]string) error {
		return errors.New("injected title write failure")
	}
	app.autoTitleAcceptedTopicTurn(acceptedTopicTitleTarget{
		topicID:       topic.ID,
		workspaceRoot: projectRoot,
		referenceRoot: projectRoot,
		actor:         userctx.Default(projectRoot),
	}, "兜底标题", "", "", false)
	saveTopicTitlesForUpdate = originalSave
	t.Cleanup(func() { saveTopicTitlesForUpdate = originalSave })
	if got := loadTopicTitle(projectRoot, topic.ID); got != defaultTopicTitle {
		t.Fatalf("title after immediate failure = %q, want default", got)
	}

	sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(sessionPath, []byte(`{"role":"user","content":"兜底标题"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}
	title, updated := autoTitleTopicFromSession(projectRoot, topic.ID, sessionPath)
	if !updated || title != "兜底标题" {
		t.Fatalf("TurnDone fallback title = %q updated=%v", title, updated)
	}
}

func TestAcceptedTurnTitlePersistsExactSessionWithStaleTopicIndex(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	app := NewApp()
	topic, err := app.CreateTopic("project", projectRoot, "")
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	actor := userctx.Default(projectRoot)
	sessionDir := desktopSessionDirForActor(actor, projectRoot)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	invalidateTopicSessionIndex(sessionDir)
	t.Cleanup(func() { invalidateTopicSessionIndex(sessionDir) })
	if matches := topicSessionMatches(sessionDir, topic.ID); len(matches) != 0 {
		t.Fatalf("initial topic session matches = %d, want empty stale index", len(matches))
	}

	sessionPath := writeTopicSessionWithPrompt(t, sessionDir, "cloud-project-title.jsonl", topic.ID, defaultTopicTitle, projectRoot, "11", time.Now())
	app.autoTitleAcceptedTopicTurn(acceptedTopicTitleTarget{
		topicID:       topic.ID,
		workspaceRoot: projectRoot,
		referenceRoot: projectRoot,
		sessionPath:   sessionPath,
		actor:         actor,
	}, "11", "", "", false)

	meta, ok, err := agent.LoadBranchMeta(sessionPath)
	if err != nil || !ok {
		t.Fatalf("load exact session metadata: ok=%v err=%v", ok, err)
	}
	if meta.TopicTitle != "11" {
		t.Fatalf("exact session title = %q, want %q", meta.TopicTitle, "11")
	}
}

func TestAutoTitleTopicStripsReasoningLanguagePrefix(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topic, err := NewApp().CreateTopic("project", projectRoot, "")
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	prompt := control.New(control.Options{ReasoningLanguage: "zh"}).Compose("讲讲这个代码库的架构")
	sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(sessionPath, []byte(`{"role":"user","content":`+strconv.Quote(prompt)+`}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	title, updated := autoTitleTopicFromSession(projectRoot, topic.ID, sessionPath)
	if !updated {
		t.Fatal("auto title should update")
	}
	if title != "讲讲这个代码库的架构" {
		t.Fatalf("generated title = %q", title)
	}
}

func TestAutoTitleTopicUsesRecordedVisibleGoalRequest(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topic, err := NewApp().CreateTopic("project", projectRoot, "")
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	visible := "修复专家团 Goal 展示泄漏"
	goal := `<system-prompt source="expert" hidden="true" mode="team">
Never expose this expert control contract.
</system-prompt>

` + visible
	ctl := control.New(control.Options{})
	ctl.SetGoal(goal)
	composed := ctl.Compose("Start pursuing the active goal now.")
	sessionDir := t.TempDir()
	sessionPath := filepath.Join(sessionDir, "session.jsonl")
	if err := os.WriteFile(sessionPath, []byte(`{"role":"user","content":`+strconv.Quote(composed)+`}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}
	if err := recordSessionDisplay(sessionDir, sessionPath, composed, visible); err != nil {
		t.Fatalf("record display: %v", err)
	}

	title, updated := autoTitleTopicFromSession(projectRoot, topic.ID, sessionPath)
	if !updated {
		t.Fatal("auto title should update")
	}
	if title != visible {
		t.Fatalf("generated title = %q", title)
	}
}

func TestAutoTitleDoesNotOverrideManualTopicTitle(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	app := NewApp()
	topic, err := app.CreateTopic("project", projectRoot, "")
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	if err := app.RenameTopic(topic.ID, "手动标题"); err != nil {
		t.Fatalf("rename topic: %v", err)
	}
	sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(sessionPath, []byte(`{"role":"user","content":"讲讲这个代码库的架构"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session: %v", err)
	}

	if title, updated := autoTitleTopicFromSession(projectRoot, topic.ID, sessionPath); updated || title != "" {
		t.Fatalf("manual title should not auto-update, title=%q updated=%v", title, updated)
	}
	if got := loadTopicTitle(projectRoot, topic.ID); got != "手动标题" {
		t.Fatalf("stored title = %q, want 手动标题", got)
	}
}

func TestTrashTopicMovesRelatedSessionsToTrash(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topicID := "topic_trash_history"
	if err := addProject(projectRoot, ""); err != nil {
		t.Fatalf("add project: %v", err)
	}
	if err := setTopicTitle(projectRoot, topicID, "Trash history"); err != nil {
		t.Fatalf("set topic title: %v", err)
	}
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	sessionPath := writeTopicSession(t, dir, "trash-me.jsonl", topicID, "Trash history", projectRoot)
	ref := "sa_20260102_030405_000000000_aabbccddeeff"
	writeSubagentArtifact(t, dir, ref, agent.BranchID(sessionPath))

	if err := NewApp().TrashTopic(topicID); err != nil {
		t.Fatalf("trash topic: %v", err)
	}
	if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
		t.Fatalf("topic session should be removed from active history, stat err = %v", err)
	}
	trashPath := filepath.Join(dir, sessionTrashDir, "trash-me.jsonl", "trash-me.jsonl")
	if _, err := os.Stat(trashPath); err != nil {
		t.Fatalf("topic session should be moved to trash: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, sessionTrashDir, "trash-me.jsonl", "subagents", ref+".jsonl")); err != nil {
		t.Fatalf("topic subagent should be moved to trash: %v", err)
	}
	if got := loadTopicTitle(projectRoot, topicID); got != "" {
		t.Fatalf("topic title should be removed, got %q", got)
	}
}

func TestRestoreGlobalTopicSessionReindexesProjectTree(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	sessionPath := writeLegacySession(t, dir, "restore-global.jsonl", "restore global history", time.Now().Add(-time.Hour))
	topicID := legacySessionTopicID(sessionPath)
	app := NewApp()

	nodes := app.ListProjectTree()
	if len(nodes) != 1 || len(nodes[0].Children) != 1 || nodes[0].Children[0].TopicID != topicID {
		t.Fatalf("legacy session should start in Global, got %#v", nodes)
	}
	if err := app.TrashTopic(topicID); err != nil {
		t.Fatalf("trash global topic: %v", err)
	}
	trashPath := filepath.Join(dir, sessionTrashDir, "restore-global.jsonl", "restore-global.jsonl")
	if _, err := os.Stat(trashPath); err != nil {
		t.Fatalf("global session should be in trash: %v", err)
	}
	if got := app.ListProjectTree(); len(got) != 1 || got[0].Kind != "global_folder" || len(got[0].Children) != 0 {
		t.Fatalf("trashed global topic should leave empty Global folder, got %#v", got)
	}

	if err := app.RestoreSession(trashPath); err != nil {
		t.Fatalf("restore global session: %v", err)
	}
	if got := app.ListTrashedSessions(); len(got) != 0 {
		t.Fatalf("trash should be empty after restore, got %#v", got)
	}
	nodes = app.ListProjectTree()
	if len(nodes) != 1 || nodes[0].Kind != "global_folder" || len(nodes[0].Children) != 1 || nodes[0].Children[0].TopicID != topicID {
		t.Fatalf("restored global session should reappear in Global, got %#v", nodes)
	}
}

func TestRestoreProjectTopicSessionReindexesProjectTree(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topicID := "topic_restore_project"
	if err := addProject(projectRoot, ""); err != nil {
		t.Fatalf("add project: %v", err)
	}
	if err := setTopicTitle(projectRoot, topicID, "Project restore"); err != nil {
		t.Fatalf("set topic title: %v", err)
	}
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	writeTopicSession(t, dir, "restore-project.jsonl", topicID, "Project restore", projectRoot)
	app := NewApp()

	if err := app.TrashTopic(topicID); err != nil {
		t.Fatalf("trash project topic: %v", err)
	}
	trashPath := filepath.Join(dir, sessionTrashDir, "restore-project.jsonl", "restore-project.jsonl")
	if _, err := os.Stat(trashPath); err != nil {
		t.Fatalf("project session should be in trash: %v", err)
	}
	if got := loadTopicTitle(projectRoot, topicID); got != "" {
		t.Fatalf("topic title should be removed while trashed, got %q", got)
	}

	if err := app.RestoreSession(trashPath); err != nil {
		t.Fatalf("restore project session: %v", err)
	}
	nodes := app.ListProjectTree()
	if len(nodes) != 1 || nodes[0].Kind != "project" || len(nodes[0].Children) != 1 || nodes[0].Children[0].TopicID != topicID {
		t.Fatalf("restored project session should reappear in project tree, got %#v", nodes)
	}
	if got := loadTopicTitle(projectRoot, topicID); got != "Project restore" {
		t.Fatalf("restored topic title = %q, want Project restore", got)
	}
}

func TestOpenProjectTabResolvesProjectSessionFromLegacyDir(t *testing.T) {
	isolateDesktopUserDirs(t)
	configureDesktopTestBYOKRoute(t)

	projectRoot := t.TempDir()
	topicID := "topic_legacy_project"
	topicTitle := "Legacy project topic"
	if err := addProject(projectRoot, ""); err != nil {
		t.Fatalf("add project: %v", err)
	}
	if err := setTopicTitle(projectRoot, topicID, topicTitle); err != nil {
		t.Fatalf("set topic title: %v", err)
	}
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	sessionPath := writeTopicSessionWithPrompt(t, dir, "legacy-project.jsonl", topicID, topicTitle, projectRoot, "legacy project prompt", time.Now())
	app := NewApp()

	nodes := app.ListProjectTree()
	if len(nodes) != 1 || nodes[0].Kind != "project" || len(nodes[0].Children) != 1 || nodes[0].Children[0].TopicID != topicID {
		t.Fatalf("legacy project session should appear in project tree, got %#v", nodes)
	}
	meta, err := app.OpenProjectTab(projectRoot, topicID)
	if err != nil {
		t.Fatalf("OpenProjectTab: %v", err)
	}
	tab := waitForTabReady(t, app, meta.ID)
	if tab.Ctrl == nil {
		t.Fatalf("tab controller was not built")
	}
	if got := filepath.Clean(tab.Ctrl.SessionPath()); got != filepath.Clean(sessionPath) {
		t.Fatalf("opened session path = %q, want %q", got, sessionPath)
	}
	history := tab.Ctrl.History()
	if len(history) == 0 || history[0].Content != "legacy project prompt" {
		t.Fatalf("opened history = %+v, want legacy project prompt", history)
	}
}

func TestRestoreSessionWithoutTopicMetadataFallsBackToGlobal(t *testing.T) {
	isolateDesktopUserDirs(t)

	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	sessionPath := writeLegacySession(t, dir, "restore-orphan.jsonl", "restore orphan history", time.Now().Add(-time.Hour))
	topicID := legacySessionTopicID(sessionPath)
	app := NewApp()
	ctrl := control.New(control.Options{SessionDir: dir, SessionPath: filepath.Join(dir, "active.jsonl"), Label: "test"})
	app.setTestCtrl(ctrl, "")
	defer ctrl.Close()
	if err := app.DeleteSession(sessionPath); err != nil {
		t.Fatalf("delete orphan session: %v", err)
	}
	trashPath := filepath.Join(dir, sessionTrashDir, "restore-orphan.jsonl", "restore-orphan.jsonl")

	if err := app.RestoreSession(trashPath); err != nil {
		t.Fatalf("restore orphan session: %v", err)
	}
	nodes := app.ListProjectTree()
	if len(nodes) != 1 || nodes[0].Kind != "global_folder" || len(nodes[0].Children) != 1 || nodes[0].Children[0].TopicID != topicID {
		t.Fatalf("restored orphan session should fall back to Global, got %#v", nodes)
	}
}

func TestTrashTopicMovesOpenSessionToTrash(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topicID := "topic_open_trash"
	if err := addProject(projectRoot, ""); err != nil {
		t.Fatalf("add project: %v", err)
	}
	if err := setTopicTitle(projectRoot, topicID, "Open trash"); err != nil {
		t.Fatalf("set topic title: %v", err)
	}
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	sessionPath := filepath.Join(dir, "open-trash.jsonl")
	if err := agent.SaveBranchMeta(sessionPath, agent.BranchMeta{
		CreatedAt:     time.Now().Add(-time.Minute),
		UpdatedAt:     time.Now(),
		Scope:         "project",
		WorkspaceRoot: projectRoot,
		TopicID:       topicID,
		TopicTitle:    "Open trash",
	}); err != nil {
		t.Fatalf("save branch meta: %v", err)
	}
	openTab := &WorkspaceTab{
		ID:            "tab_open",
		Scope:         "project",
		WorkspaceRoot: projectRoot,
		TopicID:       topicID,
		TopicTitle:    "Open trash",
		Ctrl:          controllerWithContent(t, sessionPath),
		Ready:         true,
		disabledMCP:   map[string]ServerView{},
	}
	otherTab := &WorkspaceTab{
		ID:            "tab_other",
		Scope:         "project",
		WorkspaceRoot: projectRoot,
		TopicID:       "topic_keep",
		TopicTitle:    "Keep",
		Ready:         true,
		disabledMCP:   map[string]ServerView{},
	}
	app := &App{
		tabRuntimes: map[string]*WorkspaceTab{"tab_open": openTab, "tab_other": otherTab},
		tabOrder:    []string{"tab_open", "tab_other"},
		activeTabID: "tab_open",
	}
	persistTestTabState(t, app, "tab_open")

	if err := app.TrashTopic(topicID); err != nil {
		t.Fatalf("trash topic: %v", err)
	}
	if _, ok := app.tabRuntimes["tab_open"]; ok {
		t.Fatalf("open tab for trashed topic should be removed")
	}
	if got := app.cachedActiveTabID(); got != "tab_other" {
		t.Fatalf("active tab = %q, want tab_other", got)
	}
	if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
		t.Fatalf("open topic session should be removed from active history, stat err = %v", err)
	}
	trashPath := filepath.Join(dir, sessionTrashDir, "open-trash.jsonl", "open-trash.jsonl")
	if _, err := os.Stat(trashPath); err != nil {
		t.Fatalf("open topic session should be moved to trash: %v", err)
	}
	trashed := app.ListTrashedSessions()
	if len(trashed) != 1 || trashed[0].Path != trashPath {
		t.Fatalf("trashed sessions = %#v, want %q", trashed, trashPath)
	}
	preview, err := app.PreviewSession(trashPath)
	if err != nil {
		t.Fatalf("preview trashed session: %v", err)
	}
	if !hasHistoryContent(preview, "remember this turn") {
		t.Fatalf("preview trashed session = %#v, want remembered turn", preview)
	}
	if got := loadTopicTitle(projectRoot, topicID); got != "" {
		t.Fatalf("topic title should be removed, got %q", got)
	}
}

func TestTrashTopicCancelsRunningSessionRuntime(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topicID := "topic_running_trash"
	if err := addProject(projectRoot, ""); err != nil {
		t.Fatalf("add project: %v", err)
	}
	if err := setTopicTitle(projectRoot, topicID, "Running trash"); err != nil {
		t.Fatalf("set topic title: %v", err)
	}
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	sessionPath := writeTopicSession(t, dir, "running-trash.jsonl", topicID, "Running trash", projectRoot)
	runner := &blockingRunner{started: make(chan struct{}), release: make(chan struct{})}
	ctrl := control.New(control.Options{Runner: runner, SessionDir: dir, SessionPath: sessionPath, Label: "test", WorkspaceRoot: projectRoot})
	defer ctrl.Close()
	app := &App{
		tabRuntimes: map[string]*WorkspaceTab{
			"running": {
				ID:            "running",
				Scope:         "project",
				WorkspaceRoot: projectRoot,
				TopicID:       topicID,
				TopicTitle:    "Running trash",
				Ctrl:          ctrl,
				Ready:         true,
				disabledMCP:   map[string]ServerView{},
			},
			"keep": {
				ID:            "keep",
				Scope:         "project",
				WorkspaceRoot: projectRoot,
				TopicID:       "topic_keep",
				TopicTitle:    "Keep",
				Ready:         true,
				disabledMCP:   map[string]ServerView{},
			},
		},
		tabOrder:    []string{"running", "keep"},
		activeTabID: "running",
	}
	persistTestTabState(t, app, "running")

	ctrl.Submit("long turn")
	<-runner.started
	if err := app.TrashTopic(topicID); err != nil {
		t.Fatalf("trash topic: %v", err)
	}
	waitNotRunning(t, ctrl)
	if _, ok := app.tabRuntimes["running"]; ok {
		t.Fatalf("running topic runtime should be removed")
	}
	if got := app.cachedActiveTabID(); got != "keep" {
		t.Fatalf("active tab = %q, want keep", got)
	}
	if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
		t.Fatalf("running topic session should be moved out of active history, stat err = %v", err)
	}
	trashPath := filepath.Join(dir, sessionTrashDir, "running-trash.jsonl", "running-trash.jsonl")
	if _, err := os.Stat(trashPath); err != nil {
		t.Fatalf("running topic session should be moved to trash: %v", err)
	}
}

func TestTrashTopicFallbackCreatesIndexedTopic(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topicID := "topic_only"
	if err := addProject(projectRoot, ""); err != nil {
		t.Fatalf("add project: %v", err)
	}
	if err := setTopicTitle(projectRoot, topicID, "Only topic"); err != nil {
		t.Fatalf("set topic title: %v", err)
	}
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	sessionPath := writeTopicSession(t, dir, "only-topic.jsonl", topicID, "Only topic", projectRoot)
	ctrl := control.New(control.Options{SessionDir: dir, SessionPath: sessionPath, Label: "test", WorkspaceRoot: projectRoot})
	defer ctrl.Close()
	app := &App{
		tabRuntimes: map[string]*WorkspaceTab{
			"only": {
				ID:            "only",
				Scope:         "project",
				WorkspaceRoot: projectRoot,
				TopicID:       topicID,
				TopicTitle:    "Only topic",
				Ctrl:          ctrl,
				Ready:         true,
				disabledMCP:   map[string]ServerView{},
			},
		},
		tabOrder:    []string{"only"},
		activeTabID: "only",
	}

	if err := app.TrashTopic(topicID); err != nil {
		t.Fatalf("TrashTopic: %v", err)
	}
	if len(app.tabRuntimes) != 1 {
		t.Fatalf("fallback should create exactly one visible tab, got %d", len(app.tabRuntimes))
	}
	for id, tab := range app.tabRuntimes {
		if strings.TrimSpace(tab.TopicID) == "" {
			t.Fatalf("fallback tab %q has empty topic ID", id)
		}
		f := loadProjectsFile()
		if len(f.Projects) != 1 || !containsDesktopString(f.Projects[0].Topics, tab.TopicID) {
			t.Fatalf("fallback topic %q was not indexed in project topics %#v", tab.TopicID, f.Projects)
		}
	}
}

func TestTrashTopicTrashConflictKeepsRunningRuntime(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topicID := "topic_trash_conflict"
	if err := addProject(projectRoot, ""); err != nil {
		t.Fatalf("add project: %v", err)
	}
	if err := setTopicTitle(projectRoot, topicID, "Trash conflict"); err != nil {
		t.Fatalf("set topic title: %v", err)
	}
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	sessionPath := writeTopicSession(t, dir, "trash-conflict.jsonl", topicID, "Trash conflict", projectRoot)
	if err := os.MkdirAll(filepath.Join(dir, sessionTrashDir, filepath.Base(sessionPath)), 0o755); err != nil {
		t.Fatalf("create trash conflict: %v", err)
	}
	runner := &blockingRunner{started: make(chan struct{}), release: make(chan struct{})}
	ctrl := control.New(control.Options{Runner: runner, SessionDir: dir, SessionPath: sessionPath, Label: "test", WorkspaceRoot: projectRoot})
	defer ctrl.Close()
	app := &App{
		tabRuntimes: map[string]*WorkspaceTab{
			"running": {
				ID:            "running",
				Scope:         "project",
				WorkspaceRoot: projectRoot,
				TopicID:       topicID,
				TopicTitle:    "Trash conflict",
				Ctrl:          ctrl,
				Ready:         true,
				disabledMCP:   map[string]ServerView{},
			},
		},
		tabOrder:    []string{"running"},
		activeTabID: "running",
	}

	ctrl.Submit("long turn")
	<-runner.started
	err := app.TrashTopic(topicID)
	if err != nil {
		t.Fatalf("TrashTopic should succeed after cleaning empty trash dir: %v", err)
	}
	if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
		t.Fatalf("session file should be moved to trash, stat err = %v", err)
	}

	close(runner.release)
	waitNotRunning(t, ctrl)
}

func TestTrashTopicValidTrashRemovesEmptyLiveStub(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	topicID := "topic_valid_trash"
	if err := addProject(projectRoot, ""); err != nil {
		t.Fatalf("add project: %v", err)
	}
	if err := setTopicTitle(projectRoot, topicID, "Valid trash"); err != nil {
		t.Fatalf("set topic title: %v", err)
	}
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir sessions: %v", err)
	}
	sessionPath := filepath.Join(dir, "valid-trash.jsonl")
	if err := os.WriteFile(sessionPath, nil, 0o644); err != nil {
		t.Fatalf("write live stub: %v", err)
	}
	if err := agent.SaveBranchMeta(sessionPath, agent.BranchMeta{
		CreatedAt:     time.Now().Add(-time.Minute),
		UpdatedAt:     time.Now(),
		Scope:         "project",
		WorkspaceRoot: projectRoot,
		TopicID:       topicID,
		TopicTitle:    "Valid trash",
	}); err != nil {
		t.Fatalf("save branch meta: %v", err)
	}
	trashPath := filepath.Join(dir, sessionTrashDir, filepath.Base(sessionPath), filepath.Base(sessionPath))
	if err := os.MkdirAll(filepath.Dir(trashPath), 0o755); err != nil {
		t.Fatalf("create trash dir: %v", err)
	}
	if err := os.WriteFile(trashPath, []byte(`{"role":"user","content":"already trashed"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write trash session: %v", err)
	}

	app := &App{
		tabRuntimes: map[string]*WorkspaceTab{
			"stale": {
				ID:            "stale",
				Scope:         "project",
				WorkspaceRoot: projectRoot,
				TopicID:       topicID,
				TopicTitle:    "Valid trash",
				SessionPath:   sessionPath,
				Ready:         true,
				disabledMCP:   map[string]ServerView{},
			},
			"other": {ID: "other", Scope: "project", WorkspaceRoot: projectRoot, TopicID: "other", Ready: true},
		},
		tabOrder:    []string{"stale", "other"},
		activeTabID: "other",
	}

	if err := app.TrashTopic(topicID); err != nil {
		t.Fatalf("TrashTopic should remove stale live stub: %v", err)
	}
	if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
		t.Fatalf("live stub should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(trashPath); err != nil {
		t.Fatalf("existing trash should remain authoritative: %v", err)
	}
}

func hasHistoryContent(messages []HistoryMessage, content string) bool {
	for _, m := range messages {
		if m.Content == content {
			return true
		}
	}
	return false
}

func TestLegacyMigrationSkipsProjectScopedSessions(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := writeLegacySession(t, dir, "scoped.jsonl", "hello", time.Now())
	meta, err := agent.EnsureBranchMeta(path)
	if err != nil {
		t.Fatal(err)
	}
	meta.Scope = "project"
	meta.WorkspaceRoot = filepath.Join(t.TempDir(), "proj")
	meta.TopicID = ""
	if err := agent.SaveBranchMeta(path, meta); err != nil {
		t.Fatal(err)
	}

	migrateLegacySessionsIntoGlobalTopics(dir)

	got, err := agent.EnsureBranchMeta(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Scope != "project" || got.WorkspaceRoot != meta.WorkspaceRoot {
		t.Fatalf("project-scoped legacy session must not be forced into Global: %+v", got)
	}
}

func TestProjectTreeMigratesCLISessionFromProjectDir(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	if err := addProject(projectRoot, ""); err != nil {
		t.Fatalf("add project: %v", err)
	}
	dir := config.ProjectSessionDir(projectRoot)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	sessionPath := writeLegacySession(t, dir, "cli-project.jsonl", "cli project prompt", time.Now())
	wantTopicID := legacySessionTopicID(sessionPath)

	nodes := NewApp().ListProjectTree()
	if len(nodes) != 1 || nodes[0].Kind != "project" || len(nodes[0].Children) != 1 || nodes[0].Children[0].TopicID != wantTopicID {
		t.Fatalf("project CLI session should appear in project tree, got %#v; want topic %q", nodes, wantTopicID)
	}
}

func TestProjectTreeMigratesNewCLISessionAfterProjectDirMarker(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	if err := addProject(projectRoot, ""); err != nil {
		t.Fatalf("add project: %v", err)
	}
	dir := config.ProjectSessionDir(projectRoot)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	first := writeLegacySession(t, dir, "first-cli-project.jsonl", "first cli project prompt", time.Now().Add(-time.Hour))
	firstTopicID := legacySessionTopicID(first)

	nodes := NewApp().ListProjectTree()
	if len(nodes) != 1 || nodes[0].Kind != "project" || len(nodes[0].Children) != 1 || nodes[0].Children[0].TopicID != firstTopicID {
		t.Fatalf("first project CLI session should appear in project tree, got %#v; want topic %q", nodes, firstTopicID)
	}
	if _, err := os.Stat(filepath.Join(dir, topicMigrationMarker)); err != nil {
		t.Fatalf("expected migration marker after first project pass: %v", err)
	}

	time.Sleep(10 * time.Millisecond)
	second := writeLegacySession(t, dir, "second-cli-project.jsonl", "second cli project prompt", time.Now())
	secondTopicID := legacySessionTopicID(second)

	nodes = NewApp().ListProjectTree()
	if len(nodes) != 1 || nodes[0].Kind != "project" || len(nodes[0].Children) != 2 {
		t.Fatalf("second project CLI session should trigger re-scan, got %#v", nodes)
	}
	if nodes[0].Children[0].TopicID != secondTopicID || nodes[0].Children[1].TopicID != firstTopicID {
		t.Fatalf("project CLI topics = %#v, want newest %q then %q", nodes[0].Children, secondTopicID, firstTopicID)
	}
}

func TestProjectTreeMigratesCLISessionFromGlobalWorkspaceDir(t *testing.T) {
	isolateDesktopUserDirs(t)

	globalRoot := globalWorkspaceRoot()
	dir := desktopSessionDir(globalRoot)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	sessionPath := writeLegacySession(t, dir, "cli-global.jsonl", "cli global prompt", time.Now())
	if err := agent.SaveBranchMetaPreserveUpdated(sessionPath, agent.BranchMeta{
		CreatedAt:     time.Now().Add(-time.Minute),
		UpdatedAt:     time.Now(),
		Scope:         "global",
		WorkspaceRoot: globalRoot,
	}); err != nil {
		t.Fatal(err)
	}
	wantTopicID := legacySessionTopicID(sessionPath)

	nodes := NewApp().ListProjectTree()
	if len(nodes) != 1 || nodes[0].Kind != "global_folder" || len(nodes[0].Children) != 1 || nodes[0].Children[0].TopicID != wantTopicID {
		t.Fatalf("global workspace CLI session should appear in Global, got %#v; want topic %q", nodes, wantTopicID)
	}
}

func TestLegacyMigrationConcurrentRunsHaveNoLostUpdates(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	const n = 8
	want := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		p := writeLegacySession(t, dir, fmt.Sprintf("legacy-%d.jsonl", i), "hi", time.Now())
		want[legacySessionTopicID(p)] = true
	}

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			migrateLegacySessionsIntoGlobalTopics(dir)
		}()
	}
	wg.Wait()

	gotSet := map[string]bool{}
	for _, id := range loadProjectsFile().GlobalTopics {
		gotSet[id] = true
	}
	for id := range want {
		if !gotSet[id] {
			t.Fatalf("concurrent migration lost topic %q; GlobalTopics=%v", id, loadProjectsFile().GlobalTopics)
		}
	}
}

func TestFindTopicSessionIndexRefreshesWhenMetaChanges(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	topicID := "topic_cache_refresh"
	now := time.Now().UTC()
	first := writeTopicSessionWithPrompt(t, dir, "first.jsonl", topicID, "First", "", "first prompt", now.Add(-time.Hour))

	if got := findTopicSession(dir, topicID); got != first {
		t.Fatalf("first lookup = %q, want %q", got, first)
	}

	second := writeTopicSessionWithPrompt(t, dir, "second.jsonl", topicID, "Second", "", "second prompt", now)
	if got := findTopicSession(dir, topicID); got != second {
		t.Fatalf("lookup after new session = %q, want newer %q", got, second)
	}

	meta, ok, err := agent.LoadBranchMeta(second)
	if err != nil || !ok {
		t.Fatalf("load second meta: ok=%v err=%v", ok, err)
	}
	meta.TopicID = "topic_cache_other"
	meta.UpdatedAt = now.Add(time.Hour)
	if err := agent.SaveBranchMetaPreserveUpdated(second, meta); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(agent.BranchMetaPath(second), future, future); err != nil {
		t.Fatal(err)
	}

	if got := findTopicSession(dir, topicID); got != first {
		t.Fatalf("lookup after retopic = %q, want remaining %q", got, first)
	}
	if got := findTopicSession(dir, "topic_cache_other"); got != second {
		t.Fatalf("lookup for retopic session = %q, want %q", got, second)
	}
}

func TestFindTopicSessionSkipsCleanupPending(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	topicID := "topic_skip_pending"
	now := time.Now().UTC()
	normal := writeTopicSessionWithPrompt(t, dir, "normal.jsonl", topicID, "Normal", "", "normal prompt", now)
	pending := writeTopicSessionWithPrompt(t, dir, "pending.jsonl", topicID, "Pending", "", "pending prompt", now.Add(time.Hour))

	if got := findTopicSession(dir, topicID); got != pending {
		t.Fatalf("pre-marker lookup = %q, want newest pending %q", got, pending)
	}
	if err := agent.MarkCleanupPending(pending, "delete"); err != nil {
		t.Fatal(err)
	}
	if got := findTopicSession(dir, topicID); got != normal {
		t.Fatalf("lookup with cleanup-pending newest = %q, want normal %q", got, normal)
	}
	if err := agent.MarkCleanupPending(normal, "delete"); err != nil {
		t.Fatal(err)
	}
	if got := findTopicSession(dir, topicID); got != "" {
		t.Fatalf("lookup with only cleanup-pending sessions = %q, want empty", got)
	}
}

func TestOpenProjectTabSkipsCleanupPendingTopicSession(t *testing.T) {
	isolateDesktopUserDirs(t)
	configureDesktopTestBYOKRoute(t)

	projectRoot := t.TempDir()
	app := NewApp()
	topic, err := app.CreateTopic("project", projectRoot, "Pending topic")
	if err != nil {
		t.Fatalf("create topic: %v", err)
	}
	dir := desktopSessionDir(projectRoot)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	pending := writeTopicSessionWithPrompt(t, dir, "pending-topic.jsonl", topic.ID, "Pending topic", projectRoot, "pending topic prompt", time.Now())
	if err := agent.MarkCleanupPending(pending, "delete"); err != nil {
		t.Fatal(err)
	}
	if got := findTopicSession(dir, topic.ID); got != "" {
		t.Fatalf("topic lookup with only cleanup-pending session = %q, want empty", got)
	}
	if got, _ := app.findTopicSessionForTarget("project", projectRoot, topic.ID); got != "" {
		t.Fatalf("target topic lookup with only cleanup-pending session = %q, want empty", got)
	}

	meta, err := app.OpenProjectTab(projectRoot, topic.ID)
	if err != nil {
		t.Fatalf("open project tab: %v", err)
	}
	tab := waitForTabReady(t, app, meta.ID)
	if got := filepath.Clean(tab.Ctrl.SessionPath()); got == filepath.Clean(pending) {
		t.Fatalf("opened cleanup-pending topic session path %q", got)
	}
	for _, msg := range tab.Ctrl.History() {
		if msg.Content == "pending topic prompt" {
			t.Fatalf("opened cleanup-pending topic history at path %q: %+v", tab.Ctrl.SessionPath(), tab.Ctrl.History())
		}
	}
}

func TestUpdateTopicSessionTitlesUsesTopicIndex(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	topicID := "topic_title_index"
	now := time.Now().UTC()
	valid := writeTopicSessionWithPrompt(t, dir, "valid.jsonl", topicID, "Old", "", "hello", now)
	unpreviewable := filepath.Join(dir, "unpreviewable.jsonl")
	if err := os.WriteFile(unpreviewable, []byte("not-json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := agent.SaveBranchMetaPreserveUpdated(unpreviewable, agent.BranchMeta{
		CreatedAt:  now.Add(-time.Minute),
		UpdatedAt:  now,
		Scope:      "global",
		TopicID:    topicID,
		TopicTitle: "Old",
	}); err != nil {
		t.Fatal(err)
	}

	NewApp().updateTopicSessionTitles(topicID, "Renamed")

	for _, path := range []string{valid, unpreviewable} {
		meta, ok, err := agent.LoadBranchMeta(path)
		if err != nil || !ok {
			t.Fatalf("load meta for %s: ok=%v err=%v", path, ok, err)
		}
		if meta.TopicTitle != "Renamed" {
			t.Fatalf("topic title for %s = %q, want Renamed", path, meta.TopicTitle)
		}
	}
}

func TestAcceptedTitleAndSceneMutationPreserveBothFields(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	topicID := "accepted_title_scene_race"
	path := writeTopicSessionWithPrompt(t, dir, "race.jsonl", topicID, "Old", "", "hello", time.Now().UTC())

	sceneEntered := make(chan struct{})
	releaseScene := make(chan struct{})
	sceneDone := make(chan error, 1)
	go func() {
		sceneDone <- agent.MutateBranchMeta(path, false, func(meta *agent.BranchMeta) error {
			close(sceneEntered)
			<-releaseScene
			meta.Model = "custom/design-model"
			return meta.SetSceneStatus(scene.Status{Scene: scene.Design, State: scene.Locked})
		})
	}()
	<-sceneEntered

	titleDone := make(chan error, 1)
	go func() {
		titleDone <- NewApp().updateTopicSessionTitlesInDirsResult([]string{dir}, topicID, "Accepted title")
	}()
	select {
	case err := <-titleDone:
		t.Fatalf("title update completed while scene mutation held the sidecar transaction: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(releaseScene)
	if err := <-sceneDone; err != nil {
		t.Fatal(err)
	}
	if err := <-titleDone; err != nil {
		t.Fatal(err)
	}

	meta, ok, err := agent.LoadBranchMeta(path)
	if err != nil || !ok {
		t.Fatalf("LoadBranchMeta = %+v, %v, %v", meta, ok, err)
	}
	status, err := meta.SceneStatus()
	if err != nil {
		t.Fatal(err)
	}
	if meta.TopicTitle != "Accepted title" || meta.Model != "custom/design-model" || status != (scene.Status{Scene: scene.Design, State: scene.Locked}) {
		t.Fatalf("metadata lost an accepted-title/scene field: %+v", meta)
	}
}

func TestEnsureTopicIndexedConcurrentRunsHaveNoLostProjectUpdates(t *testing.T) {
	isolateDesktopUserDirs(t)

	projectRoot := t.TempDir()
	const n = 12
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			topicID := fmt.Sprintf("topic_recovered_%02d", i)
			if err := ensureTopicIndexed("project", projectRoot, topicID, fmt.Sprintf("Recovered %02d", i), topicTitleSourceManual); err != nil {
				t.Errorf("ensure topic indexed: %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()

	nodes := NewApp().ListProjectTree()
	if len(nodes) != 1 {
		t.Fatalf("project tree len = %d, want 1: %#v", len(nodes), nodes)
	}
	got := map[string]bool{}
	for _, child := range nodes[0].Children {
		got[child.TopicID] = true
	}
	for i := 0; i < n; i++ {
		topicID := fmt.Sprintf("topic_recovered_%02d", i)
		if !got[topicID] {
			t.Fatalf("concurrent topic index recovery lost %q; children=%#v", topicID, nodes[0].Children)
		}
		if title := loadTopicTitle(projectRoot, topicID); title == "" {
			t.Fatalf("title index missing %q", topicID)
		}
	}
}

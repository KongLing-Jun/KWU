package main

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kwu/internal/agent"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/provider"
	"kwu/internal/tool"
)

func TestArtifactDeliveryToListArtifacts(t *testing.T) {
	isolateDesktopUserDirs(t)
	workspaceRoot := t.TempDir()
	if err := addProject(workspaceRoot, "Workspace A"); err != nil {
		t.Fatalf("add project: %v", err)
	}
	sessionDir := desktopSessionDir(workspaceRoot)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	sessionPath := writeTopicSessionWithPrompt(
		t, sessionDir, "artifact-session.jsonl", "topic-artifacts", "Artifacts", workspaceRoot, "create files", time.Now(),
	)

	mainPath := filepath.Join(workspaceRoot, "reports", "result.md")
	taskPath := filepath.Join(workspaceRoot, "outputs", "chart.png")
	hiddenPath := filepath.Join(workspaceRoot, ".support", "trace.txt")
	for path, body := range map[string]string{mainPath: "report", taskPath: "image", hiddenPath: "trace"} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir artifact parent: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write artifact %s: %v", path, err)
		}
	}

	firstDeliveredAt := time.Now().UnixMilli()
	firstDelivery := &provider.ArtifactDelivery{
		Version: provider.ArtifactDeliveryVersion, Paths: []string{"reports/result.md", "outputs/chart.png"}, DeliveredAt: firstDeliveredAt,
	}
	session := agent.NewSession("system")
	session.Add(provider.Message{Role: provider.RoleUser, Content: "create files"})
	session.Add(provider.Message{
		Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "文件已创建。", ArtifactDelivery: firstDelivery,
	})
	executor := agent.New(stubProvider{}, tool.NewRegistry(), session, agent.Options{}, event.Discard)
	ctrl := control.New(control.Options{
		Executor:      executor,
		SessionDir:    sessionDir,
		SessionPath:   sessionPath,
		WorkspaceRoot: workspaceRoot,
		Sink:          event.Discard,
	})
	t.Cleanup(ctrl.Close)
	tab := &WorkspaceTab{
		ID:            "artifact_tab",
		Ctrl:          ctrl,
		Scope:         "project",
		WorkspaceRoot: workspaceRoot,
		Ready:         true,
		disabledMCP:   map[string]ServerView{},
	}
	app := &App{
		tabRuntimes: map[string]*WorkspaceTab{tab.ID: tab},
		activeTabID: tab.ID,
	}
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}
	tab.sink.Emit(event.Event{Kind: event.TurnDone, ArtifactDelivery: firstDelivery})
	waitForFile(t, artifactSidecarPath(sessionPath), "reports/result.md")
	waitForAutosaveIdle(t, tab)

	firstRaw, err := os.ReadFile(artifactSidecarPath(sessionPath))
	if err != nil {
		t.Fatalf("read first artifact sidecar: %v", err)
	}
	var firstSidecar artifactSidecar
	if err := json.Unmarshal(firstRaw, &firstSidecar); err != nil {
		t.Fatalf("decode first artifact sidecar: %v", err)
	}
	firstTimes := map[string]int64{}
	for _, artifact := range firstSidecar.Artifacts {
		firstTimes[artifact.Path] = artifact.DeliveredAt
	}
	if firstTimes["reports/result.md"] != firstDeliveredAt || firstTimes["outputs/chart.png"] != firstDeliveredAt {
		t.Fatalf("first delivery times = %+v", firstTimes)
	}

	// A new turn re-delivering the same path updates only that path. The older
	// task receipt remains in the sidecar without being reprocessed.
	secondDeliveredAt := firstDeliveredAt + 1
	secondDelivery := &provider.ArtifactDelivery{
		Version: provider.ArtifactDeliveryVersion, Paths: []string{"reports/result.md"}, DeliveredAt: secondDeliveredAt,
	}
	session.Add(provider.Message{Role: provider.RoleUser, Content: "update the report"})
	session.Add(provider.Message{
		Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "更新完成。", ArtifactDelivery: secondDelivery,
	})
	tab.sink.Emit(event.Event{Kind: event.TurnDone, ArtifactDelivery: secondDelivery})
	waitForFile(t, sessionPath, "update the report")
	waitForAutosaveIdle(t, tab)
	if err := setSessionTitle(sessionDir, sessionPath, "Delivery session"); err != nil {
		t.Fatalf("set session title: %v", err)
	}

	raw, err := os.ReadFile(artifactSidecarPath(sessionPath))
	if err != nil {
		t.Fatalf("read artifact sidecar: %v", err)
	}
	var sidecar artifactSidecar
	if err := json.Unmarshal(raw, &sidecar); err != nil {
		t.Fatalf("decode artifact sidecar: %v", err)
	}
	if len(sidecar.Artifacts) != 2 {
		t.Fatalf("sidecar artifacts = %+v, want main and verified task output only", sidecar.Artifacts)
	}
	times := map[string]int64{}
	for _, artifact := range sidecar.Artifacts {
		if filepath.IsAbs(artifact.Path) {
			t.Fatalf("sidecar stored absolute path %q", artifact.Path)
		}
		times[artifact.Path] = artifact.DeliveredAt
	}
	if times["reports/result.md"] <= firstTimes["reports/result.md"] || times["outputs/chart.png"] != firstTimes["outputs/chart.png"] {
		t.Fatalf("sidecar delivery times = %+v", times)
	}

	artifacts := app.ListArtifacts()
	if len(artifacts) != 2 {
		t.Fatalf("ListArtifacts = %+v, want 2", artifacts)
	}
	if artifacts[0].Path != "reports/result.md" || artifacts[0].DeliveredAt != times["reports/result.md"] {
		t.Fatalf("newest artifact = %+v", artifacts[0])
	}
	if artifacts[1].Path != "outputs/chart.png" || artifacts[1].DeliveredAt != times["outputs/chart.png"] {
		t.Fatalf("task artifact = %+v", artifacts[1])
	}
	for _, artifact := range artifacts {
		if artifact.SessionTitle != "Delivery session" || artifact.WorkspaceName != "Workspace A" {
			t.Fatalf("artifact display metadata = %+v", artifact)
		}
		if filepath.Clean(artifact.Session.Path) != filepath.Clean(sessionPath) || filepath.Clean(artifact.Session.WorkspaceRoot) != filepath.Clean(workspaceRoot) {
			t.Fatalf("artifact session metadata = %+v", artifact.Session)
		}
	}
}

func TestCommittedArtifactDeliveryPersistsWithoutFinalLink(t *testing.T) {
	isolateDesktopUserDirs(t)
	workspaceRoot := t.TempDir()
	sessionDir := desktopSessionDir(workspaceRoot)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	sessionPath := writeTopicSessionWithPrompt(
		t, sessionDir, "root-write-artifact.jsonl", "topic-root-write", "Root write", workspaceRoot, "create joke", time.Now(),
	)
	if err := os.WriteFile(filepath.Join(workspaceRoot, "joke.md"), []byte("joke"), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	session := agent.NewSession("system")
	session.Add(provider.Message{Role: provider.RoleUser, Content: "create joke"})
	session.Add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{
		ID: "write-1", Name: "write_file", Arguments: `{"path":"joke.md","content":"joke"}`,
	}}})
	session.Add(provider.Message{Role: provider.RoleTool, ToolCallID: "write-1", Name: "write_file", Content: "Wrote joke.md"})
	deliveredAt := time.Now().UnixMilli()
	delivery := &provider.ArtifactDelivery{Version: provider.ArtifactDeliveryVersion, Paths: []string{"joke.md"}, DeliveredAt: deliveredAt}
	session.Add(provider.Message{
		Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "已完成 `joke.md`。", ArtifactDelivery: delivery,
	})
	executor := agent.New(stubProvider{}, tool.NewRegistry(), session, agent.Options{}, event.Discard)
	ctrl := control.New(control.Options{
		Executor: executor, SessionDir: sessionDir, SessionPath: sessionPath, WorkspaceRoot: workspaceRoot, Sink: event.Discard,
	})
	t.Cleanup(ctrl.Close)
	tab := &WorkspaceTab{ID: "root_write_artifact_tab", Ctrl: ctrl, Scope: "project", WorkspaceRoot: workspaceRoot, Ready: true}
	app := &App{tabRuntimes: map[string]*WorkspaceTab{tab.ID: tab}, activeTabID: tab.ID}
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}

	tab.sink.Emit(event.Event{Kind: event.TurnDone, ArtifactDelivery: delivery})
	waitForFile(t, sessionPath, `"artifact_delivery"`)
	waitForFile(t, artifactSidecarPath(sessionPath), "joke.md")
	waitForAutosaveIdle(t, tab)

	history := ctrl.History()
	final := history[len(history)-1]
	if final.ArtifactDelivery == nil || final.ArtifactDelivery.Version != provider.ArtifactDeliveryVersion {
		t.Fatalf("final artifact delivery = %#v", final.ArtifactDelivery)
	}
	if got := final.ArtifactDelivery.Paths; len(got) != 1 || got[0] != "joke.md" {
		t.Fatalf("final artifact paths = %v, want [joke.md]", got)
	}
	projected := historyMessagesForSession(sessionDir, sessionPath, history)
	projectedFinal := projected[len(projected)-1]
	if projectedFinal.ArtifactDelivery == nil || len(projectedFinal.ArtifactDelivery.Paths) != 1 || projectedFinal.ArtifactDelivery.Paths[0] != "joke.md" {
		t.Fatalf("history artifact delivery = %#v", projectedFinal.ArtifactDelivery)
	}

	// Replayed terminal events reuse the first immutable contract instead of
	// appending or re-deriving another delivery record.
	tab.sink.Emit(event.Event{Kind: event.TurnDone, ArtifactDelivery: delivery})
	waitForAutosaveIdle(t, tab)
	if got := ctrl.History()[len(history)-1].ArtifactDelivery.Paths; len(got) != 1 || got[0] != "joke.md" {
		t.Fatalf("replayed artifact paths = %v, want one stable path", got)
	}
	raw, err := os.ReadFile(artifactSidecarPath(sessionPath))
	if err != nil {
		t.Fatalf("read artifact sidecar: %v", err)
	}
	var sidecar artifactSidecar
	if err := json.Unmarshal(raw, &sidecar); err != nil || len(sidecar.Artifacts) != 1 || sidecar.Artifacts[0].DeliveredAt != deliveredAt {
		t.Fatalf("replayed sidecar = %#v, %v", sidecar, err)
	}
}

func TestCompactedArtifactReceiptProjectsOneHistoryCardAndLibraryRecord(t *testing.T) {
	workspaceRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspaceRoot, "report.md"), []byte("report"), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	sessionPath := filepath.Join(t.TempDir(), "compacted-artifact.jsonl")
	delivery := &provider.ArtifactDelivery{
		Version: provider.ArtifactDeliveryVersion, Paths: []string{"report.md"}, DeliveredAt: 1234,
	}
	session := agent.NewSession("system")
	session.Add(provider.Message{Role: provider.RoleUser, Content: "create report"})
	session.Add(provider.Message{
		Role: provider.RoleAssistant, Content: "Report ready.", MessageKind: provider.MessageKindFinalAnswer,
		LocalOnly: true, ArtifactDelivery: delivery,
	})
	session.Add(provider.Message{Role: provider.RoleUser, Content: "<compaction-summary>earlier work</compaction-summary>"})
	if err := session.Save(sessionPath); err != nil {
		t.Fatalf("save compacted session: %v", err)
	}

	projected := historyMessages(session.Snapshot(), func(content string) string { return content })
	cards := 0
	for _, message := range projected {
		if message.ArtifactDelivery == nil {
			continue
		}
		cards++
		if !message.FinalAnswer || len(message.ArtifactDelivery.Paths) != 1 || message.ArtifactDelivery.Paths[0] != "report.md" {
			t.Fatalf("projected receipt = %#v", message)
		}
	}
	if cards != 1 {
		t.Fatalf("projected artifact card count = %d, want 1", cards)
	}

	records := artifactRecordsForSession(sessionPath, workspaceRoot)
	if len(records) != 1 || records[0].Path != "report.md" || records[0].DeliveredAt != delivery.DeliveredAt {
		t.Fatalf("artifact library records = %#v, want one report.md record", records)
	}
}

func TestListArtifactsRecoversPersistedContractWithoutSidecar(t *testing.T) {
	isolateDesktopUserDirs(t)
	workspaceRoot := t.TempDir()
	if err := addProject(workspaceRoot, "Workspace A"); err != nil {
		t.Fatalf("add project: %v", err)
	}
	sessionDir := desktopSessionDir(workspaceRoot)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	sessionPath := writeTopicSessionWithPrompt(
		t, sessionDir, "recover-artifact.jsonl", "topic-recover-artifact", "Recover artifact", workspaceRoot, "create report", time.Now(),
	)
	if err := os.WriteFile(filepath.Join(workspaceRoot, "report.md"), []byte("report"), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "ghost.md"), []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale artifact: %v", err)
	}
	delivery := &provider.ArtifactDelivery{Version: provider.ArtifactDeliveryVersion, Paths: []string{"report.md"}, DeliveredAt: 1234}
	session := agent.NewSession("system")
	session.Add(provider.Message{Role: provider.RoleUser, Content: "create report"})
	session.Add(provider.Message{Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "done", ArtifactDelivery: delivery})
	if err := session.Save(sessionPath); err != nil {
		t.Fatalf("save session: %v", err)
	}
	if err := os.Remove(artifactSidecarPath(sessionPath)); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove sidecar: %v", err)
	}

	app := &App{}
	assertRecovered := func(stage string) {
		t.Helper()
		artifacts := app.ListArtifacts()
		if len(artifacts) != 1 || artifacts[0].Path != "report.md" || artifacts[0].DeliveredAt != delivery.DeliveredAt {
			t.Fatalf("%s ListArtifacts = %#v", stage, artifacts)
		}
	}
	assertRecovered("missing sidecar")
	if err := os.WriteFile(artifactSidecarPath(sessionPath), []byte("not json"), 0o600); err != nil {
		t.Fatalf("corrupt sidecar: %v", err)
	}
	assertRecovered("corrupt sidecar")

	staleSidecar := artifactSidecar{
		Version:                 artifactSidecarVersion,
		ArtifactDeliveryVersion: provider.ArtifactDeliveryVersion,
		Artifacts: []artifactSidecarRecord{
			{Path: "report.md", DeliveredAt: delivery.DeliveredAt + 1000},
			{Path: "ghost.md", DeliveredAt: delivery.DeliveredAt + 2000},
		},
	}
	raw, err := json.Marshal(staleSidecar)
	if err != nil {
		t.Fatalf("marshal stale sidecar: %v", err)
	}
	if err := os.WriteFile(artifactSidecarPath(sessionPath), raw, 0o600); err != nil {
		t.Fatalf("write stale sidecar: %v", err)
	}
	assertRecovered("stale newer sidecar")
}

func TestArtifactRecordsForSessionFallsBackToLegacySidecar(t *testing.T) {
	workspaceRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspaceRoot, "legacy.md"), []byte("legacy"), 0o644); err != nil {
		t.Fatalf("write legacy artifact: %v", err)
	}
	sessionPath := filepath.Join(t.TempDir(), "legacy-session.jsonl")
	legacySession := agent.NewSession("system")
	legacySession.Add(provider.Message{Role: provider.RoleUser, Content: "create legacy report"})
	legacySession.Add(provider.Message{Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "done"})
	if err := legacySession.Save(sessionPath); err != nil {
		t.Fatalf("save legacy session: %v", err)
	}
	sidecar := artifactSidecar{
		Version: legacyArtifactSidecarVersion,
		Artifacts: []artifactSidecarRecord{
			{Path: "legacy.md", DeliveredAt: 5678},
		},
	}
	raw, err := json.Marshal(sidecar)
	if err != nil {
		t.Fatalf("marshal legacy sidecar: %v", err)
	}
	if err := os.WriteFile(artifactSidecarPath(sessionPath), raw, 0o600); err != nil {
		t.Fatalf("write legacy sidecar: %v", err)
	}

	records := artifactRecordsForSession(sessionPath, workspaceRoot)
	if len(records) != 1 || records[0].Path != "legacy.md" || records[0].DeliveredAt != 5678 {
		t.Fatalf("legacy artifact records = %#v", records)
	}
}

func TestArtifactRecordsForSessionPreservesOnlyLegacyHistoryWhenTranscriptIsMalformed(t *testing.T) {
	tests := []struct {
		name    string
		sidecar artifactSidecar
	}{
		{
			name: "legacy v1 sidecar",
			sidecar: artifactSidecar{
				Version: legacyArtifactSidecarVersion,
				Artifacts: []artifactSidecarRecord{
					{Path: "legacy.md", DeliveredAt: 100},
				},
			},
		},
		{
			name: "activated v2 sidecar",
			sidecar: artifactSidecar{
				Version:                 artifactSidecarVersion,
				ArtifactDeliveryVersion: provider.ArtifactDeliveryVersion,
				LegacyArtifacts: []artifactSidecarRecord{
					{Path: "legacy.md", DeliveredAt: 100},
				},
				Artifacts: []artifactSidecarRecord{
					{Path: "modern.md", DeliveredAt: 200},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspaceRoot := t.TempDir()
			for _, name := range []string{"legacy.md", "modern.md"} {
				if err := os.WriteFile(filepath.Join(workspaceRoot, name), []byte(name), 0o644); err != nil {
					t.Fatalf("write %s: %v", name, err)
				}
			}
			sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
			if err := os.WriteFile(sessionPath, []byte("{\"role\":\"user\"}\n{"), 0o600); err != nil {
				t.Fatalf("write malformed session: %v", err)
			}
			raw, err := json.Marshal(tt.sidecar)
			if err != nil {
				t.Fatalf("marshal sidecar: %v", err)
			}
			if err := os.WriteFile(artifactSidecarPath(sessionPath), raw, 0o600); err != nil {
				t.Fatalf("write sidecar: %v", err)
			}

			records := artifactRecordsForSession(sessionPath, workspaceRoot)
			if len(records) != 1 || records[0].Path != "legacy.md" || records[0].DeliveredAt != 100 {
				t.Fatalf("records from malformed transcript = %#v, want legacy history only", records)
			}
		})
	}
}

func TestArtifactRecordsForSessionActivatesContractAndMigratesLegacyHistory(t *testing.T) {
	tests := []struct {
		name        string
		paths       []string
		wantRecords []string
	}{
		{
			name:        "explicit empty contract",
			paths:       []string{},
			wantRecords: []string{"legacy.md"},
		},
		{
			name:        "non-empty contract",
			paths:       []string{"modern.md"},
			wantRecords: []string{"legacy.md", "modern.md"},
		},
		{
			name:        "modern contract supersedes matching legacy record",
			paths:       []string{"legacy.md"},
			wantRecords: []string{"legacy.md"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspaceRoot := t.TempDir()
			for _, name := range []string{"legacy.md", "modern.md"} {
				if err := os.WriteFile(filepath.Join(workspaceRoot, name), []byte(name), 0o644); err != nil {
					t.Fatalf("write %s: %v", name, err)
				}
			}
			sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
			delivery := &provider.ArtifactDelivery{
				Version: provider.ArtifactDeliveryVersion, Paths: tt.paths, DeliveredAt: 200,
			}
			session := agent.NewSession("system")
			session.Add(provider.Message{Role: provider.RoleUser, Content: "create report"})
			session.Add(provider.Message{
				Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "done", ArtifactDelivery: delivery,
			})
			if err := session.Save(sessionPath); err != nil {
				t.Fatalf("save session: %v", err)
			}

			legacy := artifactSidecar{
				Version: legacyArtifactSidecarVersion,
				Artifacts: []artifactSidecarRecord{
					{Path: "legacy.md", DeliveredAt: 100},
				},
			}
			raw, err := json.Marshal(legacy)
			if err != nil {
				t.Fatalf("marshal legacy sidecar: %v", err)
			}
			if err := os.WriteFile(artifactSidecarPath(sessionPath), raw, 0o600); err != nil {
				t.Fatalf("write legacy sidecar: %v", err)
			}

			batch, ok := artifactDeliveryBatchFromContract(sessionPath, workspaceRoot, delivery)
			if !ok {
				t.Fatal("artifactDeliveryBatchFromContract rejected a valid contract")
			}
			if err := mergeArtifactDeliveryBatches([]artifactDeliveryBatch{batch}); err != nil {
				t.Fatalf("merge artifact delivery: %v", err)
			}

			records := artifactRecordsForSession(sessionPath, workspaceRoot)
			if len(records) != len(tt.wantRecords) {
				t.Fatalf("artifact records = %#v, want paths %v", records, tt.wantRecords)
			}
			for index, want := range tt.wantRecords {
				if records[index].Path != want {
					t.Fatalf("artifact records[%d] = %#v, want path %q", index, records[index], want)
				}
				wantDeliveredAt := int64(100)
				for _, modernPath := range tt.paths {
					if modernPath == want {
						wantDeliveredAt = delivery.DeliveredAt
						break
					}
				}
				if records[index].DeliveredAt != wantDeliveredAt {
					t.Fatalf("artifact records[%d] = %#v, want deliveredAt %d", index, records[index], wantDeliveredAt)
				}
			}

			migratedRaw, err := os.ReadFile(artifactSidecarPath(sessionPath))
			if err != nil {
				t.Fatalf("read migrated sidecar: %v", err)
			}
			var migrated artifactSidecar
			if err := json.Unmarshal(migratedRaw, &migrated); err != nil {
				t.Fatalf("decode migrated sidecar: %v", err)
			}
			if migrated.Version != artifactSidecarVersion || migrated.ArtifactDeliveryVersion != provider.ArtifactDeliveryVersion {
				t.Fatalf("migrated sidecar activation = version %d, delivery version %d", migrated.Version, migrated.ArtifactDeliveryVersion)
			}
			if len(migrated.LegacyArtifacts) != 1 || migrated.LegacyArtifacts[0].Path != "legacy.md" {
				t.Fatalf("migrated legacy history = %#v", migrated.LegacyArtifacts)
			}
			if len(migrated.Artifacts) != len(tt.paths) {
				t.Fatalf("modern sidecar projection = %#v, want %v", migrated.Artifacts, tt.paths)
			}
		})
	}
}

func TestArtifactDeliveryRevalidatesAtMerge(t *testing.T) {
	workspaceRoot := t.TempDir()
	artifactPath := filepath.Join(workspaceRoot, "temporary.md")
	if err := os.WriteFile(artifactPath, []byte("temporary"), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	session := agent.NewSession("system")
	session.Add(provider.Message{Role: provider.RoleUser, Content: "create"})
	session.Add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "deliver", Name: deliverFilesToolName}}})
	session.Add(provider.Message{Role: provider.RoleTool, ToolCallID: "deliver", Name: deliverFilesToolName, Content: mustDeliverFilesResultJSON(t, artifactPath)})
	executor := agent.New(stubProvider{}, tool.NewRegistry(), session, agent.Options{}, event.Discard)
	sessionPath := filepath.Join(t.TempDir(), "revalidate.jsonl")
	ctrl := control.New(control.Options{Executor: executor, SessionPath: sessionPath, WorkspaceRoot: workspaceRoot, Sink: event.Discard})
	t.Cleanup(ctrl.Close)

	batch, ok := captureArtifactDelivery(ctrl, time.Now().UnixMilli())
	if !ok {
		t.Fatal("structured result was not captured")
	}
	if err := os.Remove(artifactPath); err != nil {
		t.Fatalf("remove artifact before merge: %v", err)
	}
	if err := mergeArtifactDeliveryBatches([]artifactDeliveryBatch{batch}); err != nil {
		t.Fatalf("merge artifact batch: %v", err)
	}
	if _, err := os.Stat(artifactSidecarPath(sessionPath)); !os.IsNotExist(err) {
		t.Fatalf("artifact sidecar after file disappeared: %v", err)
	}
}

func TestSnapshotTabRetainsArtifactRetryAcrossSessionRuntimeReset(t *testing.T) {
	workspaceRoot := t.TempDir()
	artifactPath := filepath.Join(workspaceRoot, "retry-after-rebind.md")
	if err := os.WriteFile(artifactPath, []byte("retry"), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	sessionPath := filepath.Join(t.TempDir(), "retry-after-rebind.jsonl")
	session := agent.NewSession("system")
	executor := agent.New(stubProvider{}, tool.NewRegistry(), session, agent.Options{}, event.Discard)
	ctrl := control.New(control.Options{Executor: executor, SessionPath: sessionPath, WorkspaceRoot: workspaceRoot, Sink: event.Discard})
	t.Cleanup(ctrl.Close)
	failing := &failOnceArtifactSnapshotController{SessionAPI: ctrl}
	tab := &WorkspaceTab{ID: "retry_after_rebind_tab", Ctrl: failing, Scope: "project", WorkspaceRoot: workspaceRoot, Ready: true}
	app := &App{tabRuntimes: map[string]*WorkspaceTab{tab.ID: tab}, activeTabID: tab.ID}
	tab.enqueueArtifactDelivery(artifactDeliveryBatch{
		sessionPath: sessionPath, workspaceRoot: workspaceRoot, candidates: []string{artifactPath}, deliveredAt: time.Now().UnixMilli(),
	})

	if err := app.snapshotTab(tab); err == nil {
		t.Fatal("first snapshot error = nil")
	}
	tab.resetSessionRuntimeSidecars()
	if err := app.snapshotTab(tab); err != nil {
		t.Fatalf("retry snapshot: %v", err)
	}
	raw, err := os.ReadFile(artifactSidecarPath(sessionPath))
	if err != nil {
		t.Fatalf("read retried artifact sidecar: %v", err)
	}
	if !strings.Contains(string(raw), "retry-after-rebind.md") {
		t.Fatalf("retried artifact sidecar = %s", raw)
	}
}

func TestArtifactRecordsForSessionDoesNotReviveModernProjectionAfterLastContractRemoved(t *testing.T) {
	workspaceRoot := t.TempDir()
	for _, name := range []string{"legacy.md", "modern.md"} {
		if err := os.WriteFile(filepath.Join(workspaceRoot, name), []byte(name), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
	delivery := &provider.ArtifactDelivery{
		Version: provider.ArtifactDeliveryVersion, Paths: []string{"modern.md"}, DeliveredAt: 200,
	}
	session := agent.NewSession("system")
	session.Add(provider.Message{Role: provider.RoleUser, Content: "create report"})
	session.Add(provider.Message{
		Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "done", ArtifactDelivery: delivery,
	})
	if err := session.Save(sessionPath); err != nil {
		t.Fatalf("save modern session: %v", err)
	}
	legacy := artifactSidecar{
		Version: legacyArtifactSidecarVersion,
		Artifacts: []artifactSidecarRecord{
			{Path: "legacy.md", DeliveredAt: 100},
		},
	}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy sidecar: %v", err)
	}
	if err := os.WriteFile(artifactSidecarPath(sessionPath), raw, 0o600); err != nil {
		t.Fatalf("write legacy sidecar: %v", err)
	}
	batch, ok := artifactDeliveryBatchFromContract(sessionPath, workspaceRoot, delivery)
	if !ok {
		t.Fatal("artifactDeliveryBatchFromContract rejected a valid contract")
	}
	if err := mergeArtifactDeliveryBatches([]artifactDeliveryBatch{batch}); err != nil {
		t.Fatalf("merge artifact delivery: %v", err)
	}
	if records := artifactRecordsForSession(sessionPath, workspaceRoot); len(records) != 2 {
		t.Fatalf("records before rewind = %#v, want legacy and modern", records)
	}

	rewound := agent.NewSession("system")
	rewound.Add(provider.Message{Role: provider.RoleUser, Content: "create report"})
	rewound.Add(provider.Message{Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "done"})
	if err := rewound.Save(sessionPath); err != nil {
		t.Fatalf("save rewound session: %v", err)
	}

	records := artifactRecordsForSession(sessionPath, workspaceRoot)
	if len(records) != 1 || records[0].Path != "legacy.md" || records[0].DeliveredAt != 100 {
		t.Fatalf("records after removing last contract = %#v, want migrated legacy history only", records)
	}
	migratedRaw, err := os.ReadFile(artifactSidecarPath(sessionPath))
	if err != nil {
		t.Fatalf("read activated sidecar: %v", err)
	}
	var migrated artifactSidecar
	if err := json.Unmarshal(migratedRaw, &migrated); err != nil {
		t.Fatalf("decode activated sidecar: %v", err)
	}
	if !artifactSidecarUsesDeliveryContract(migrated) || len(migrated.Artifacts) != 1 || migrated.Artifacts[0].Path != "modern.md" {
		t.Fatalf("activated sidecar lost the stale projection needed by the regression: %#v", migrated)
	}
}

func TestArtifactAutosavePreservesCoalescedTurnDoneDeliveries(t *testing.T) {
	isolateDesktopUserDirs(t)
	workspaceRoot := t.TempDir()
	sessionDir := desktopSessionDir(workspaceRoot)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	sessionPath := writeTopicSessionWithPrompt(
		t, sessionDir, "coalesced-artifacts.jsonl", "topic-coalesced-artifacts", "Coalesced artifacts", workspaceRoot, "first", time.Now(),
	)
	firstPath := filepath.Join(workspaceRoot, "first.md")
	secondPath := filepath.Join(workspaceRoot, "second.md")
	for _, path := range []string{firstPath, secondPath} {
		if err := os.WriteFile(path, []byte(path), 0o644); err != nil {
			t.Fatalf("write artifact %s: %v", path, err)
		}
	}

	session := agent.NewSession("system")
	session.Add(provider.Message{Role: provider.RoleUser, Content: "first"})
	firstDelivery := &provider.ArtifactDelivery{Version: provider.ArtifactDeliveryVersion, Paths: []string{"first.md"}, DeliveredAt: 1001}
	session.Add(provider.Message{
		Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "完成。", ArtifactDelivery: firstDelivery,
	})
	executor := agent.New(stubProvider{}, tool.NewRegistry(), session, agent.Options{}, event.Discard)
	ctrl := control.New(control.Options{
		Executor:      executor,
		SessionDir:    sessionDir,
		SessionPath:   sessionPath,
		WorkspaceRoot: workspaceRoot,
		Sink:          event.Discard,
	})
	t.Cleanup(ctrl.Close)
	blocking := &blockingArtifactSnapshotController{
		SessionAPI: ctrl,
		entered:    make(chan struct{}),
		release:    make(chan struct{}),
	}
	tab := &WorkspaceTab{ID: "coalesced_artifacts_tab", Ctrl: blocking, Scope: "project", WorkspaceRoot: workspaceRoot, Ready: true}
	app := &App{tabRuntimes: map[string]*WorkspaceTab{tab.ID: tab}, activeTabID: tab.ID}
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}

	tab.sink.Emit(event.Event{Kind: event.TurnDone, ArtifactDelivery: firstDelivery})
	select {
	case <-blocking.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("first snapshot did not reach blocking point")
	}

	session.Add(provider.Message{Role: provider.RoleUser, Content: "second"})
	secondDelivery := &provider.ArtifactDelivery{Version: provider.ArtifactDeliveryVersion, Paths: []string{"second.md"}, DeliveredAt: 1002}
	session.Add(provider.Message{
		Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "完成。", ArtifactDelivery: secondDelivery,
	})
	tab.sink.Emit(event.Event{Kind: event.TurnDone, ArtifactDelivery: secondDelivery})
	close(blocking.release)
	waitForAutosaveIdle(t, tab)

	raw, err := os.ReadFile(artifactSidecarPath(sessionPath))
	if err != nil {
		t.Fatalf("read artifact sidecar: %v", err)
	}
	var sidecar artifactSidecar
	if err := json.Unmarshal(raw, &sidecar); err != nil {
		t.Fatalf("decode artifact sidecar: %v", err)
	}
	times := make(map[string]int64, len(sidecar.Artifacts))
	for _, artifact := range sidecar.Artifacts {
		times[artifact.Path] = artifact.DeliveredAt
	}
	if got := times["first.md"]; got != firstDelivery.DeliveredAt {
		t.Fatalf("first delivery time = %d, want %d", got, firstDelivery.DeliveredAt)
	}
	if got := times["second.md"]; got != secondDelivery.DeliveredAt {
		t.Fatalf("second delivery time = %d, want %d", got, secondDelivery.DeliveredAt)
	}
}

func TestArtifactDeliveryRetriesAfterSnapshotFailure(t *testing.T) {
	isolateDesktopUserDirs(t)
	workspaceRoot := t.TempDir()
	sessionDir := desktopSessionDir(workspaceRoot)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	sessionPath := writeTopicSessionWithPrompt(
		t, sessionDir, "retry-artifact.jsonl", "topic-retry-artifact", "Retry artifact", workspaceRoot, "create", time.Now(),
	)
	if err := os.WriteFile(filepath.Join(workspaceRoot, "retry.md"), []byte("retry"), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	session := agent.NewSession("system")
	session.Add(provider.Message{Role: provider.RoleUser, Content: "create"})
	delivery := &provider.ArtifactDelivery{Version: provider.ArtifactDeliveryVersion, Paths: []string{"retry.md"}, DeliveredAt: 1001}
	session.Add(provider.Message{
		Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "完成。", ArtifactDelivery: delivery,
	})
	executor := agent.New(stubProvider{}, tool.NewRegistry(), session, agent.Options{}, event.Discard)
	ctrl := control.New(control.Options{
		Executor:      executor,
		SessionDir:    sessionDir,
		SessionPath:   sessionPath,
		WorkspaceRoot: workspaceRoot,
		Sink:          event.Discard,
	})
	t.Cleanup(ctrl.Close)
	failing := &failOnceArtifactSnapshotController{SessionAPI: ctrl}
	tab := &WorkspaceTab{ID: "retry_artifact_tab", Ctrl: failing, Scope: "project", WorkspaceRoot: workspaceRoot, Ready: true}
	app := &App{tabRuntimes: map[string]*WorkspaceTab{tab.ID: tab}, activeTabID: tab.ID}
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}

	tab.sink.Emit(event.Event{Kind: event.TurnDone, ArtifactDelivery: delivery})
	waitForAutosaveIdle(t, tab)
	if _, err := os.Stat(artifactSidecarPath(sessionPath)); !os.IsNotExist(err) {
		t.Fatalf("sidecar after failed snapshot: %v", err)
	}
	app.scheduleTabSnapshot(tab.ID)
	waitForFile(t, artifactSidecarPath(sessionPath), "retry.md")
	waitForAutosaveIdle(t, tab)
}

type blockingArtifactSnapshotController struct {
	control.SessionAPI
	entered chan struct{}
	release chan struct{}
	blocked atomic.Bool
}

func (c *blockingArtifactSnapshotController) Snapshot() error {
	err := c.SessionAPI.Snapshot()
	if c.blocked.CompareAndSwap(false, true) {
		close(c.entered)
		<-c.release
	}
	return err
}

type failOnceArtifactSnapshotController struct {
	control.SessionAPI
	failed atomic.Bool
}

func (c *failOnceArtifactSnapshotController) Snapshot() error {
	if c.failed.CompareAndSwap(false, true) {
		return errors.New("injected snapshot failure")
	}
	return c.SessionAPI.Snapshot()
}

func artifactTestFileURL(path string) string {
	slash := filepath.ToSlash(path)
	if filepath.VolumeName(path) != "" {
		slash = "/" + slash
	}
	return (&url.URL{Scheme: "file", Path: slash}).String()
}

func mustDeliverFilesResultJSON(t testing.TB, paths ...string) string {
	t.Helper()
	raw, err := json.Marshal(struct {
		Type  string   `json:"type"`
		Paths []string `json:"paths"`
	}{Type: "deliver_files_result", Paths: paths})
	if err != nil {
		t.Fatalf("marshal deliver_files result: %v", err)
	}
	return string(raw)
}

func TestArtifactDeliveryContractDoesNotInferMarkdownLinks(t *testing.T) {
	for _, name := range []string{"app.ts", "report.xlsx"} {
		t.Run(name, func(t *testing.T) {
			workspaceRoot := t.TempDir()
			if err := os.WriteFile(filepath.Join(workspaceRoot, name), []byte("existing file"), 0o644); err != nil {
				t.Fatalf("write linked file: %v", err)
			}
			sessionPath := filepath.Join(t.TempDir(), "empty-delivery.jsonl")
			session := agent.NewSession("system")
			session.Add(provider.Message{Role: provider.RoleUser, Content: "inspect the file"})
			session.Add(provider.Message{
				Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer,
				Content: "文件在这里：[" + name + "](" + name + ")",
				ArtifactDelivery: &provider.ArtifactDelivery{
					Version: provider.ArtifactDeliveryVersion, Paths: []string{}, DeliveredAt: 100,
				},
			})
			if err := session.Save(sessionPath); err != nil {
				t.Fatalf("save session: %v", err)
			}
			if records := artifactRecordsForSession(sessionPath, workspaceRoot); len(records) != 0 {
				t.Fatalf("artifact records = %+v, want no inferred delivery from Markdown", records)
			}
		})
	}
}

func TestArtifactDeliveryFromStructuredResult(t *testing.T) {
	isolateDesktopUserDirs(t)
	workspaceRoot := t.TempDir()
	sessionDir := desktopSessionDir(workspaceRoot)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	sessionPath := writeTopicSessionWithPrompt(
		t, sessionDir, "structured-artifact.jsonl", "topic-structured-artifact", "Structured artifact", workspaceRoot, "create", time.Now(),
	)
	firstPath := filepath.Join(workspaceRoot, "reports", "first.md")
	secondPath := filepath.Join(workspaceRoot, "reports", "second.md")
	for _, path := range []string{firstPath, secondPath} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir artifact parent: %v", err)
		}
		if err := os.WriteFile(path, []byte(filepath.Base(path)), 0o644); err != nil {
			t.Fatalf("write artifact: %v", err)
		}
	}

	session := agent.NewSession("system")
	session.Add(provider.Message{Role: provider.RoleUser, Content: "create"})
	session.Add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{
		{ID: "deliver-1", Name: deliverFilesToolName, Arguments: `{}`},
		{ID: "deliver-2", Name: deliverFilesToolName, Arguments: `{}`},
	}})
	session.Add(provider.Message{Role: provider.RoleTool, ToolCallID: "deliver-1", Name: deliverFilesToolName, Content: mustDeliverFilesResultJSON(t, firstPath)})
	session.Add(provider.Message{Role: provider.RoleTool, ToolCallID: "deliver-2", Name: deliverFilesToolName, Content: mustDeliverFilesResultJSON(t, firstPath, secondPath)})
	session.Add(provider.Message{Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "文件已经准备好。"})
	executor := agent.New(stubProvider{}, tool.NewRegistry(), session, agent.Options{}, event.Discard)
	ctrl := control.New(control.Options{
		Executor: executor, SessionDir: sessionDir, SessionPath: sessionPath, WorkspaceRoot: workspaceRoot, Sink: event.Discard,
	})
	t.Cleanup(ctrl.Close)
	tab := &WorkspaceTab{ID: "structured_artifact_tab", Ctrl: ctrl, Scope: "project", WorkspaceRoot: workspaceRoot, Ready: true}
	app := &App{tabRuntimes: map[string]*WorkspaceTab{tab.ID: tab}, activeTabID: tab.ID}
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}

	tab.sink.Emit(event.Event{Kind: event.TurnDone})
	waitForFile(t, artifactSidecarPath(sessionPath), "reports/first.md")
	waitForAutosaveIdle(t, tab)

	raw, err := os.ReadFile(artifactSidecarPath(sessionPath))
	if err != nil {
		t.Fatalf("read artifact sidecar: %v", err)
	}
	var sidecar artifactSidecar
	if err := json.Unmarshal(raw, &sidecar); err != nil {
		t.Fatalf("decode artifact sidecar: %v", err)
	}
	if len(sidecar.Artifacts) != 2 || sidecar.Artifacts[0].Path != "reports/first.md" || sidecar.Artifacts[1].Path != "reports/second.md" {
		t.Fatalf("structured artifacts = %+v", sidecar.Artifacts)
	}
}

func TestModifiedExistingFileDeliveryContractIsListed(t *testing.T) {
	isolateDesktopUserDirs(t)
	workspaceRoot := t.TempDir()
	if err := addProject(workspaceRoot, "Spreadsheet workspace"); err != nil {
		t.Fatalf("add project: %v", err)
	}
	sessionDir := desktopSessionDir(workspaceRoot)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("mkdir session dir: %v", err)
	}
	sessionPath := writeTopicSessionWithPrompt(
		t, sessionDir, "modified-existing-artifact.jsonl", "topic-modified-existing-artifact", "Update spreadsheet", workspaceRoot, "update spreadsheet", time.Now(),
	)

	artifactPath := filepath.Join(workspaceRoot, "reports", "existing.xlsx")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatalf("mkdir artifact parent: %v", err)
	}
	if err := os.WriteFile(artifactPath, []byte("original workbook"), 0o644); err != nil {
		t.Fatalf("write original artifact: %v", err)
	}
	updated := []byte("updated workbook content")
	if err := os.WriteFile(artifactPath, updated, 0o644); err != nil {
		t.Fatalf("modify existing artifact: %v", err)
	}

	session := agent.NewSession("system")
	session.Add(provider.Message{Role: provider.RoleUser, Content: "update spreadsheet"})
	session.Add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "deliver-modified", Name: deliverFilesToolName, Arguments: `{}`}}})
	session.Add(provider.Message{Role: provider.RoleTool, ToolCallID: "deliver-modified", Name: deliverFilesToolName, Content: mustDeliverFilesResultJSON(t, artifactPath)})
	delivery := &provider.ArtifactDelivery{
		Version: provider.ArtifactDeliveryVersion, Paths: []string{"reports/existing.xlsx"}, DeliveredAt: time.Now().UnixMilli(),
	}
	session.Add(provider.Message{Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "表格已经修改完成。", ArtifactDelivery: delivery})
	executor := agent.New(stubProvider{}, tool.NewRegistry(), session, agent.Options{}, event.Discard)
	ctrl := control.New(control.Options{
		Executor: executor, SessionDir: sessionDir, SessionPath: sessionPath, WorkspaceRoot: workspaceRoot, Sink: event.Discard,
	})
	t.Cleanup(ctrl.Close)
	tab := &WorkspaceTab{ID: "modified_existing_artifact_tab", Ctrl: ctrl, Scope: "project", WorkspaceRoot: workspaceRoot, Ready: true}
	app := &App{tabRuntimes: map[string]*WorkspaceTab{tab.ID: tab}, activeTabID: tab.ID}
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}

	tab.sink.Emit(event.Event{Kind: event.TurnDone, ArtifactDelivery: delivery})
	waitForFile(t, artifactSidecarPath(sessionPath), "reports/existing.xlsx")
	waitForAutosaveIdle(t, tab)

	artifacts := app.ListArtifacts()
	if len(artifacts) != 1 {
		t.Fatalf("ListArtifacts = %+v, want one modified existing file", artifacts)
	}
	artifact := artifacts[0]
	if artifact.Path != "reports/existing.xlsx" || artifact.Type != "spreadsheet" || artifact.Extension != "xlsx" {
		t.Fatalf("listed artifact = %+v", artifact)
	}
	if artifact.Size != int64(len(updated)) || artifact.DeliveredAt != delivery.DeliveredAt {
		t.Fatalf("listed artifact metadata = %+v", artifact)
	}
}

func TestArtifactCandidatesIgnoreFailedDeliverFilesResults(t *testing.T) {
	valid := mustDeliverFilesResultJSON(t, "C:/workspace/valid.md")
	for _, tc := range []struct {
		name   string
		output string
	}{
		{name: "execution error", output: "error: disk failure\n" + valid},
		{name: "blocked", output: "blocked: permission denied"},
		{name: "cancelled", output: "cancelled: context cancelled before execution"},
		{name: "wrong discriminator", output: `{"type":"other_result","paths":["C:/workspace/wrong.md"]}`},
		{name: "malformed", output: `{"type":"deliver_files_result"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			turn := []provider.Message{
				{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "deliver", Name: deliverFilesToolName}}},
				{Role: provider.RoleTool, ToolCallID: "deliver", Name: deliverFilesToolName, Content: tc.output},
			}
			if got := artifactCandidatesFromTurn(turn); len(got) != 0 {
				t.Fatalf("artifact candidates = %v, want none", got)
			}
		})
	}
}

func TestArtifactDeliveryRequiresSuccessfulTurnDone(t *testing.T) {
	for _, tc := range []struct {
		name        string
		turnDoneErr error
		interrupted bool
	}{
		{name: "failed", turnDoneErr: errors.New("turn failed")},
		{name: "interrupted", interrupted: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			workspaceRoot := t.TempDir()
			sessionDir := desktopSessionDir(workspaceRoot)
			if err := os.MkdirAll(sessionDir, 0o755); err != nil {
				t.Fatalf("mkdir session dir: %v", err)
			}
			sessionPath := filepath.Join(sessionDir, "unsuccessful-turn.jsonl")
			artifactPath := filepath.Join(workspaceRoot, "result.md")
			if err := os.WriteFile(artifactPath, []byte("result"), 0o644); err != nil {
				t.Fatalf("write artifact: %v", err)
			}
			session := agent.NewSession("system")
			session.Add(provider.Message{Role: provider.RoleUser, Content: "create"})
			session.Add(provider.Message{Role: provider.RoleAssistant, ToolCalls: []provider.ToolCall{{ID: "deliver", Name: deliverFilesToolName}}})
			session.Add(provider.Message{Role: provider.RoleTool, ToolCallID: "deliver", Name: deliverFilesToolName, Content: mustDeliverFilesResultJSON(t, artifactPath)})
			executor := agent.New(stubProvider{}, tool.NewRegistry(), session, agent.Options{}, event.Discard)
			ctrl := control.New(control.Options{
				Executor: executor, SessionDir: sessionDir, SessionPath: sessionPath, WorkspaceRoot: workspaceRoot, Sink: event.Discard,
			})
			t.Cleanup(ctrl.Close)
			tab := &WorkspaceTab{ID: "unsuccessful_turn_tab", Ctrl: ctrl, Scope: "project", WorkspaceRoot: workspaceRoot, Ready: true}
			app := &App{tabRuntimes: map[string]*WorkspaceTab{tab.ID: tab}, activeTabID: tab.ID}
			tab.sink = &tabEventSink{tabID: tab.ID, app: app}

			tab.sink.Emit(event.Event{Kind: event.TurnDone, Err: tc.turnDoneErr, Interrupted: tc.interrupted})
			waitForAutosaveIdle(t, tab)
			if _, err := os.Stat(artifactSidecarPath(sessionPath)); !os.IsNotExist(err) {
				t.Fatalf("artifact sidecar after unsuccessful turn: %v", err)
			}
		})
	}
}

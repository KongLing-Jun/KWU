//go:build integration

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"kwu/desktop/internal/transcript"
	"kwu/internal/agent"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/provider"
	"kwu/internal/userctx"
)

func TestTranscriptSurvivesProcessRestart(t *testing.T) {
	dir := t.TempDir()
	first := transcriptRestartProcess(t, dir, "first")
	second := transcriptRestartProcess(t, dir, "second")
	if first.JournalID != second.JournalID || string(first.Baseline) != string(second.Baseline) {
		t.Fatal("restart replaced the existing journal or baseline")
	}
	if len(first.Entries) != 8 || len(second.Entries) != 16 {
		t.Fatalf("events lost across restart: first=%d second=%d", len(first.Entries), len(second.Entries))
	}
	if !reflect.DeepEqual(first.Entries, second.Entries[:len(first.Entries)]) {
		t.Fatal("restart rewrote old events")
	}
	for _, entry := range second.Entries[len(first.Entries):] {
		if entry.Sequence <= first.Through {
			t.Fatalf("new event reused an old delivery cursor: %d <= %d", entry.Sequence, first.Through)
		}
	}
	for i, run := range []string{"first", "second"} {
		assertRestartTurn(t, second.Entries[i*8:(i+1)*8], run)
	}
	readOnly := transcriptRestartProcess(t, dir, "read")
	if !reflect.DeepEqual(second, readOnly) {
		t.Fatal("reopening the desktop changed the recovered transcript")
	}
}

func transcriptRestartProcess(t *testing.T, dir, run string) transcript.Page {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestTranscriptRestartProcessHelper$")
	cmd.Env = append(os.Environ(), "KWU_TRANSCRIPT_RESTART_DIR="+dir, "KWU_TRANSCRIPT_RESTART_RUN="+run)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("desktop process %s: %v\n%s", run, err, output)
	}
	for _, line := range strings.Split(string(output), "\n") {
		if body, ok := strings.CutPrefix(line, "TRANSCRIPT_PAGE:"); ok {
			var page transcript.Page
			if err := json.Unmarshal([]byte(body), &page); err != nil {
				t.Fatal(err)
			}
			return page
		}
	}
	t.Fatalf("desktop process omitted its transcript: %s", output)
	return transcript.Page{}
}

func assertRestartTurn(t *testing.T, entries []transcript.Entry, run string) {
	t.Helper()
	wantKinds := []string{"notice", "turn_started", "text", "tool_dispatch", "tool_result", "usage", "message", "turn_done"}
	for i, entry := range entries {
		var wire wireEventTab
		if err := json.Unmarshal(entry.Payload, &wire); err != nil {
			t.Fatal(err)
		}
		if wire.Kind != wantKinds[i] {
			t.Fatalf("%s event %d: kind=%q want=%q", run, i, wire.Kind, wantKinds[i])
		}
		switch wire.Kind {
		case "notice":
			if wire.TranscriptUser == nil || wire.TranscriptUser.Content != run+" prompt" {
				t.Fatalf("%s prompt missing", run)
			}
		case "text", "message":
			if wire.Text != run+" complete answer" {
				t.Fatalf("%s answer truncated: %q", run, wire.Text)
			}
		case "tool_result":
			if wire.Tool.ID != run+"-tool" || wire.Tool.Output != run+" tool output" {
				t.Fatalf("%s tool result missing", run)
			}
		}
		if wire.Kind == "message" || wire.Kind == "turn_done" {
			if wire.ArtifactDelivery == nil || wire.ArtifactDelivery.DeliveredAt <= 0 || !reflect.DeepEqual(wire.ArtifactDelivery.Paths, []string{run + ".md"}) {
				t.Fatalf("%s %s delivery contract missing", run, wire.Kind)
			}
		}
	}
}

func TestTranscriptRestartProcessHelper(t *testing.T) {
	dir := os.Getenv("KWU_TRANSCRIPT_RESTART_DIR")
	if dir == "" {
		return
	}
	// TestMain also isolates each process. Override only with the parent's shared
	// temporary profile so the separate processes exercise the same database.
	for key, suffix := range map[string]string{
		"HOME": "", "USERPROFILE": "", "AppData": "appdata", "XDG_CONFIG_HOME": "config",
		"KWU_HOME": "kwu", "KWU_STATE_HOME": "state", "KWU_CACHE_HOME": "cache", devProfileRootEnv: "dev-profile",
	} {
		t.Setenv(key, filepath.Join(dir, suffix))
	}
	root := globalTabWorkspaceRoot()
	actor := userctx.Default(root)
	sessionDir := desktopSessionDirForActor(actor, root)
	path := filepath.Join(sessionDir, "restart.jsonl")
	ctrl := control.New(control.Options{
		Executor:   agent.New(nil, nil, agent.NewSession(""), agent.Options{}, event.Discard),
		SessionDir: sessionDir, SessionPath: path, Actor: actor,
	})
	defer ctrl.Close()
	a := NewApp()
	tab := &WorkspaceTab{ID: "restart-tab", Scope: "global", WorkspaceRoot: root, SessionPath: path, Ctrl: ctrl, Ready: true, actor: actor}
	tab.sink = &tabEventSink{tabID: tab.ID, app: a}
	a.tabRuntimes[tab.ID], a.activeTabID = tab, tab.ID
	if run := os.Getenv("KWU_TRANSCRIPT_RESTART_RUN"); run != "read" {
		a.recordAcceptedTranscript(ctrl, run+" prompt", run+" prompt")
		emitRestartTurn(tab.sink, run)
	}
	page, err := a.TranscriptPageForTab(tab.ID, "", 0, 0)
	if err != nil || page == nil {
		t.Fatalf("read transcript: %v", err)
	}
	data, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("TRANSCRIPT_PAGE:%s\n", data)
}

func emitRestartTurn(sink *tabEventSink, run string) {
	sink.Emit(event.Event{Kind: event.TurnStarted, TurnID: event.NewTurnID()})
	sink.Emit(event.Event{Kind: event.Text, CandidateID: event.NewCandidateID(), Text: run + " complete answer"})
	sink.Emit(event.Event{Kind: event.ToolDispatch, Tool: event.Tool{ID: run + "-tool", Name: "deliver_files"}})
	sink.Emit(event.Event{Kind: event.ToolResult, Tool: event.Tool{ID: run + "-tool", Name: "deliver_files", Output: run + " tool output"}})
	sink.Emit(event.Event{Kind: event.Usage, Usage: &provider.Usage{CompletionTokens: 20}})
	delivery := &provider.ArtifactDelivery{Version: 1, Paths: []string{run + ".md"}, DeliveredAt: 1700000000000}
	sink.Emit(event.Event{Kind: event.Message, Text: run + " complete answer", ArtifactDelivery: delivery})
	sink.Emit(event.Event{Kind: event.TurnDone, ArtifactDelivery: delivery})
}

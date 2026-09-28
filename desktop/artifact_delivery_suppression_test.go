package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"kwu/internal/agent"
	"kwu/internal/provider"
)

func TestHistoryPreservesInvalidArtifactDeliveryAsSuppressionMarker(t *testing.T) {
	tests := []struct {
		name   string
		marker *provider.ArtifactDelivery
	}{
		{name: "unknown zero version", marker: &provider.ArtifactDelivery{Version: 0}},
		{name: "malformed v1", marker: &provider.ArtifactDelivery{Version: provider.ArtifactDeliveryVersion, DeliveredAt: 100}},
		{name: "unknown version", marker: &provider.ArtifactDelivery{Version: provider.ArtifactDeliveryVersion + 1, Paths: []string{"future.md"}, DeliveredAt: 100}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messages := historyMessagesWithPlannerDisplays([]provider.Message{
				{
					Role:             provider.RoleAssistant,
					MessageKind:      provider.MessageKindFinalAnswer,
					Content:          "Created [report.md](report.md)",
					ArtifactDelivery: tt.marker,
				},
			}, func(content string) string { return content }, nil)
			if len(messages) != 1 || !messages[0].FinalAnswer {
				t.Fatalf("history messages = %#v", messages)
			}
			if messages[0].ArtifactDelivery == nil || messages[0].ArtifactDelivery.Version != tt.marker.Version {
				t.Fatalf("history artifact delivery = %#v; missing marker enables legacy inference", messages[0].ArtifactDelivery)
			}
			if (messages[0].ArtifactDelivery.Paths == nil) != (tt.marker.Paths == nil) {
				t.Fatalf("history paths = %#v, source paths = %#v", messages[0].ArtifactDelivery.Paths, tt.marker.Paths)
			}

			cloned := cloneHistoryMessages(messages)
			if len(cloned) != 1 || cloned[0].ArtifactDelivery == nil || cloned[0].ArtifactDelivery.Version != tt.marker.Version {
				t.Fatalf("cloned history artifact delivery = %#v", cloned)
			}
			if (cloned[0].ArtifactDelivery.Paths == nil) != (tt.marker.Paths == nil) {
				t.Fatalf("cloned paths = %#v, source paths = %#v", cloned[0].ArtifactDelivery.Paths, tt.marker.Paths)
			}
		})
	}
}

func TestArtifactRecordsForSessionDoesNotFallbackForInvalidContract(t *testing.T) {
	tooMany := make([]string, 33)
	for index := range tooMany {
		tooMany[index] = fmt.Sprintf("report-%02d.md", index)
	}
	tests := []struct {
		name     string
		delivery *provider.ArtifactDelivery
		files    []string
	}{
		{name: "unknown version", delivery: &provider.ArtifactDelivery{Version: 0}},
		{name: "nil paths", delivery: &provider.ArtifactDelivery{Version: 1, DeliveredAt: 100}},
		{name: "empty path", delivery: &provider.ArtifactDelivery{Version: 1, Paths: []string{""}, DeliveredAt: 100}},
		{name: "whitespace path", delivery: &provider.ArtifactDelivery{Version: 1, Paths: []string{" report.md"}, DeliveredAt: 100}},
		{name: "duplicate path", delivery: &provider.ArtifactDelivery{Version: 1, Paths: []string{"report.md", "report.md"}, DeliveredAt: 100}},
		{name: "too many paths", delivery: &provider.ArtifactDelivery{Version: 1, Paths: tooMany, DeliveredAt: 100}, files: tooMany},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspaceRoot := t.TempDir()
			files := append([]string{"report.md"}, tt.files...)
			for _, relative := range files {
				path := filepath.Join(workspaceRoot, filepath.FromSlash(relative))
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatalf("mkdir %s: %v", relative, err)
				}
				if err := os.WriteFile(path, []byte(relative), 0o644); err != nil {
					t.Fatalf("write %s: %v", relative, err)
				}
			}

			sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
			session := agent.NewSession("system")
			session.Add(provider.Message{Role: provider.RoleUser, Content: "create report"})
			session.Add(provider.Message{
				Role:             provider.RoleAssistant,
				MessageKind:      provider.MessageKindFinalAnswer,
				Content:          "Created [report.md](report.md)",
				ArtifactDelivery: tt.delivery,
			})
			if err := session.Save(sessionPath); err != nil {
				t.Fatalf("save session: %v", err)
			}

			legacy := artifactSidecar{
				Version: legacyArtifactSidecarVersion,
				Artifacts: []artifactSidecarRecord{
					{Path: "report.md", DeliveredAt: 100},
				},
			}
			raw, err := json.Marshal(legacy)
			if err != nil {
				t.Fatalf("marshal legacy sidecar: %v", err)
			}
			if err := os.WriteFile(artifactSidecarPath(sessionPath), raw, 0o600); err != nil {
				t.Fatalf("write legacy sidecar: %v", err)
			}

			if records := artifactRecordsForSession(sessionPath, workspaceRoot); len(records) != 0 {
				t.Fatalf("records = %#v, want invalid contract to suppress legacy fallback", records)
			}
		})
	}
}

func TestArtifactRecordsForSessionProjectsExplicitContractWithoutHeuristicFiltering(t *testing.T) {
	workspaceRoot := t.TempDir()
	paths := []string{
		"src/main.go",
		"logs/run.log",
		"build/output.json",
		".customer-delivery/build/output.json",
	}
	for _, relative := range paths {
		path := filepath.Join(workspaceRoot, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", relative, err)
		}
		if err := os.WriteFile(path, []byte(relative), 0o644); err != nil {
			t.Fatalf("write %s: %v", relative, err)
		}
	}

	sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
	session := agent.NewSession("system")
	session.Add(provider.Message{Role: provider.RoleUser, Content: "deliver the requested project files"})
	session.Add(provider.Message{
		Role:        provider.RoleAssistant,
		MessageKind: provider.MessageKindFinalAnswer,
		Content:     "Delivered the requested files.",
		ArtifactDelivery: &provider.ArtifactDelivery{
			Version: provider.ArtifactDeliveryVersion, Paths: paths, DeliveredAt: 100,
		},
	})
	if err := session.Save(sessionPath); err != nil {
		t.Fatalf("save session: %v", err)
	}

	got := make(map[string]bool)
	for _, record := range artifactRecordsForSession(sessionPath, workspaceRoot) {
		got[record.Path] = true
	}
	for _, path := range paths {
		if !got[path] {
			t.Fatalf("records = %#v, missing explicitly delivered path %q", got, path)
		}
	}
	if len(got) != len(paths) {
		t.Fatalf("records = %#v, want exactly %d explicit paths", got, len(paths))
	}
}

func TestArtifactRecordsForSessionActivatedSidecarFailsClosedForInvalidOnlyTranscript(t *testing.T) {
	tests := []struct {
		name     string
		delivery *provider.ArtifactDelivery
	}{
		{
			name:     "malformed v1",
			delivery: &provider.ArtifactDelivery{Version: provider.ArtifactDeliveryVersion, DeliveredAt: 200},
		},
		{
			name: "unknown future version",
			delivery: &provider.ArtifactDelivery{
				Version: provider.ArtifactDeliveryVersion + 1, Paths: []string{"future.md"}, DeliveredAt: 200,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspaceRoot := t.TempDir()
			for _, name := range []string{"legacy.md", "stale-modern.md"} {
				if err := os.WriteFile(filepath.Join(workspaceRoot, name), []byte(name), 0o644); err != nil {
					t.Fatalf("write %s: %v", name, err)
				}
			}

			sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
			session := agent.NewSession("system")
			session.Add(provider.Message{Role: provider.RoleUser, Content: "create report"})
			session.Add(provider.Message{
				Role:             provider.RoleAssistant,
				MessageKind:      provider.MessageKindFinalAnswer,
				Content:          "Created [legacy.md](legacy.md)",
				ArtifactDelivery: tt.delivery,
			})
			if err := session.Save(sessionPath); err != nil {
				t.Fatalf("save session: %v", err)
			}

			activated := artifactSidecar{
				Version:                 artifactSidecarVersion,
				ArtifactDeliveryVersion: provider.ArtifactDeliveryVersion,
				LegacyArtifacts: []artifactSidecarRecord{
					{Path: "legacy.md", DeliveredAt: 100},
				},
				Artifacts: []artifactSidecarRecord{
					{Path: "stale-modern.md", DeliveredAt: 150},
				},
			}
			raw, err := json.Marshal(activated)
			if err != nil {
				t.Fatalf("marshal activated sidecar: %v", err)
			}
			if err := os.WriteFile(artifactSidecarPath(sessionPath), raw, 0o600); err != nil {
				t.Fatalf("write activated sidecar: %v", err)
			}

			if records := artifactRecordsForSession(sessionPath, workspaceRoot); len(records) != 0 {
				t.Fatalf("records = %#v, want invalid-only transcript to suppress activated legacy history", records)
			}
		})
	}
}

func TestArtifactRecordsForSessionProjectsValidContractBesideUnknownContract(t *testing.T) {
	workspaceRoot := t.TempDir()
	for _, name := range []string{"legacy.md", "modern.md"} {
		if err := os.WriteFile(filepath.Join(workspaceRoot, name), []byte(name), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
	session := agent.NewSession("system")
	session.Add(provider.Message{Role: provider.RoleUser, Content: "create report"})
	session.Add(provider.Message{
		Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "created modern.md",
		ArtifactDelivery: &provider.ArtifactDelivery{
			Version: provider.ArtifactDeliveryVersion, Paths: []string{"modern.md"}, DeliveredAt: 200,
		},
	})
	session.Add(provider.Message{
		Role: provider.RoleAssistant, MessageKind: provider.MessageKindFinalAnswer, Content: "future marker",
		ArtifactDelivery: &provider.ArtifactDelivery{
			Version: provider.ArtifactDeliveryVersion + 1, Paths: []string{"future.md"}, DeliveredAt: 300,
		},
	})
	if err := session.Save(sessionPath); err != nil {
		t.Fatalf("save session: %v", err)
	}

	activated := artifactSidecar{
		Version:                 artifactSidecarVersion,
		ArtifactDeliveryVersion: provider.ArtifactDeliveryVersion,
		LegacyArtifacts: []artifactSidecarRecord{
			{Path: "legacy.md", DeliveredAt: 100},
		},
	}
	raw, err := json.Marshal(activated)
	if err != nil {
		t.Fatalf("marshal activated sidecar: %v", err)
	}
	if err := os.WriteFile(artifactSidecarPath(sessionPath), raw, 0o600); err != nil {
		t.Fatalf("write activated sidecar: %v", err)
	}

	records := artifactRecordsForSession(sessionPath, workspaceRoot)
	if len(records) != 2 || records[0].Path != "legacy.md" || records[1].Path != "modern.md" {
		t.Fatalf("records = %#v, want activated legacy history and valid v1 projection", records)
	}
	if records[0].DeliveredAt != 100 || records[1].DeliveredAt != 200 {
		t.Fatalf("delivery times = %#v, want legacy=100 and modern=200", records)
	}
}

func TestHistoryDoesNotCommitUncommittedFinalAnswer(t *testing.T) {
	workspaceRoot := t.TempDir()
	reportPath := filepath.Join(workspaceRoot, "report.md")
	if err := os.WriteFile(reportPath, []byte("report"), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	if info, err := os.Stat(reportPath); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("report is not a real file: info=%v err=%v", info, err)
	}

	messages := historyMessagesWithPlannerDisplays([]provider.Message{
		{
			Role:        provider.RoleAssistant,
			MessageKind: provider.MessageKindUncommittedFinalAnswer,
			Content:     "Created [report.md](report.md)",
		},
	}, func(content string) string { return content }, nil)
	if len(messages) != 1 {
		t.Fatalf("history messages = %#v", messages)
	}
	if messages[0].FinalAnswer || messages[0].ArtifactDelivery != nil {
		t.Fatalf("uncommitted history message = %#v, want non-final without a contract", messages[0])
	}
}

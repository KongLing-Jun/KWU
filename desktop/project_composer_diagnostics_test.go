package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecordClientErrorProjectComposerPersistsDiagnosticObservations(t *testing.T) {
	record := projectComposerDiagnosticTestRecord()
	body := recordProjectComposerJournal(t, record)
	for _, expected := range []string{
		" projectComposer.send [PROJECT_COMPOSER_SESSION_ACTIVATE_FAILED] | client_diagnostic_id=A0B1C2D3E4",
		"version=1 route=fresh phase_at_start=draft target=cloud step=switch-tab reason=exception",
		"list_tabs=succeeded cause_type=TypeError has_tab=true has_session=false adapter_current=true",
		"tab_found=true path_matches=true build=0123456789ab",
		"frame=app:0:123:456:aBcD_-09", "phase=activate-session",
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("persisted diagnostic missing %q: %s", expected, body)
		}
	}
	fields := strings.Fields(body)
	if len(fields) < 4 || fields[2] == record.DiagnosticID || strings.Count(fields[2], "-") != 1 {
		t.Fatalf("frontend correlation replaced journal-owned ID: %s", body)
	}
	if strings.Contains(body, "[redacted") || strings.Contains(body, "[truncated]") {
		t.Fatalf("validated observations did not survive journal formatting: %s", body)
	}
}

func TestRecordClientErrorProjectComposerRejectsInvalidCore(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ClientErrorRecord)
	}{
		{"different scope", func(r *ClientErrorRecord) { r.Scope = "workspace.preview" }},
		{"scope lookalike", func(r *ClientErrorRecord) { r.Scope = "projectComposer.send.extra" }},
		{"missing extension", func(r *ClientErrorRecord) { r.ProjectComposer = nil }},
		{"unknown version", func(r *ClientErrorRecord) { r.ProjectComposer.Version = 2 }},
		{"code does not match stage", func(r *ClientErrorRecord) { r.Code = "PROJECT_COMPOSER_PREPARE_FAILED" }},
		{"unknown stage", func(r *ClientErrorRecord) { r.ProjectComposer.Stage = "private-stage" }},
		{"unknown route", func(r *ClientErrorRecord) { r.ProjectComposer.Route = "private-route" }},
		{"unknown phase", func(r *ClientErrorRecord) { r.ProjectComposer.PhaseAtStart = "private-phase" }},
		{"unknown target", func(r *ClientErrorRecord) { r.ProjectComposer.ExecutionTarget = "private-target" }},
		{"unknown lookup outcome", func(r *ClientErrorRecord) { r.ProjectComposer.ListTabsOutcome = "private-outcome" }},
		{"unknown step", func(r *ClientErrorRecord) { r.ProjectComposer.Step = "private-step" }},
		{"step from other stage", func(r *ClientErrorRecord) { r.ProjectComposer.Step = "set-model" }},
		{"unknown reason", func(r *ClientErrorRecord) { r.ProjectComposer.Reason = "private-reason" }},
		{"reason from other step", func(r *ClientErrorRecord) { r.ProjectComposer.Reason = "path-mismatch" }},
		{"contradictory lookup", func(r *ClientErrorRecord) { no := false; r.ProjectComposer.TabFound = &no }},
		{"fresh route with pinned step", func(r *ClientErrorRecord) { r.ProjectComposer.Step = "verify-pinned" }},
		{"pinned route with prepare step", func(r *ClientErrorRecord) {
			r.Code = "PROJECT_COMPOSER_PREPARE_FAILED"
			r.ProjectComposer.Stage, r.ProjectComposer.Step, r.ProjectComposer.Route = "prepare-tab", "prepare-tab", "pinned"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := projectComposerDiagnosticTestRecord()
			test.mutate(&record)
			body := recordProjectComposerJournal(t, record)
			if !strings.HasSuffix(body, " | renderer failure\n") {
				t.Fatalf("invalid extension changed legacy event: %s", body)
			}
			for _, forbidden := range []string{"client_diagnostic_id=", "private-", "A0B1C2D3E4", "frame=", "phase="} {
				if strings.Contains(body, forbidden) {
					t.Errorf("rejected extension leaked %q: %s", forbidden, body)
				}
			}
		})
	}
}

func TestRecordClientErrorProjectComposerOmitsInvalidOptionalCorrelation(t *testing.T) {
	tests := []struct {
		name, value string
	}{
		{"missing", ""},
		{"short", "ABC123"},
		{"lowercase", "a0b1c2d3e4"},
		{"injected", "ABCDEFGHI\n"},
		{"long", strings.Repeat("A", 10000)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := projectComposerDiagnosticTestRecord()
			record.DiagnosticID = test.value
			body := recordProjectComposerJournal(t, record)
			if !strings.Contains(body, " | project composer failure version=1 ") ||
				!strings.Contains(body, "step=switch-tab") || !strings.Contains(body, "cause_type=TypeError") {
				t.Fatalf("invalid optional correlation discarded valid observations: %s", body)
			}
			if strings.Contains(body, "client_diagnostic_id=") || (test.value != "" && strings.Contains(body, test.value)) {
				t.Fatalf("invalid correlation escaped validation: %s", body)
			}
		})
	}
}

func TestRecordClientErrorProjectComposerAcceptsCompatibleStagesAndReasons(t *testing.T) {
	tests := []struct {
		name, stage, code, step, reason, route string
	}{
		{"prepare", "prepare-tab", "PROJECT_COMPOSER_PREPARE_FAILED", "prepare-tab", "tab-missing", "fresh"},
		{"prepare refresh", "prepare-tab", "PROJECT_COMPOSER_PREPARE_FAILED", "refresh-tabs", "exception", "fresh"},
		{"adapter changed", "prepare-tab", "PROJECT_COMPOSER_PREPARE_FAILED", "verify-adapter", "scope-changed", "fresh"},
		{"model admission", "prepare-tab", "PROJECT_COMPOSER_PREPARE_FAILED", "model-admission", "model-blocked", "fresh"},
		{"create", "create-session", "PROJECT_COMPOSER_SESSION_CREATE_FAILED", "create-session", "exception", "fresh"},
		{"snapshot", "reconcile-session", "PROJECT_COMPOSER_SESSION_RECONCILE_FAILED", "verify-session", "snapshot-mismatch", "fresh"},
		{"lookup", "reconcile-session", "PROJECT_COMPOSER_SESSION_RECONCILE_FAILED", "refresh-tabs", "exception", "fresh"},
		{"missing target", "activate-session", "PROJECT_COMPOSER_SESSION_ACTIVATE_FAILED", "verify-target", "tab-missing", "pinned"},
		{"mismatched path", "activate-session", "PROJECT_COMPOSER_SESSION_ACTIVATE_FAILED", "verify-target", "path-mismatch", "pinned"},
		{"not pinned", "activate-session", "PROJECT_COMPOSER_SESSION_ACTIVATE_FAILED", "verify-pinned", "not-pinned", "pinned"},
		{"profile", "apply-profile", "PROJECT_COMPOSER_PROFILE_FAILED", "set-goal", "exception", "fresh"},
		{"capabilities", "resolve-capabilities", "PROJECT_COMPOSER_CAPABILITIES_FAILED", "resolve-capabilities", "exception", "pinned"},
		{"send", "send-message", "PROJECT_COMPOSER_SEND_FAILED", "send-message", "exception", "fresh"},
		{"steer", "activate-session", "PROJECT_COMPOSER_SESSION_ACTIVATE_FAILED", "steer", "exception", "pinned"},
		{"shell", "activate-session", "PROJECT_COMPOSER_SESSION_ACTIVATE_FAILED", "run-shell", "exception", "pinned"},
		{"connect", "activate-session", "PROJECT_COMPOSER_SESSION_ACTIVATE_FAILED", "connect-command", "exception", "pinned"},
		{"management", "activate-session", "PROJECT_COMPOSER_SESSION_ACTIVATE_FAILED", "management-command", "exception", "pinned"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := projectComposerDiagnosticTestRecord()
			record.Code = test.code
			detail := record.ProjectComposer
			detail.Stage, detail.Step, detail.Reason, detail.Route = test.stage, test.step, test.reason, test.route
			body := recordProjectComposerJournal(t, record)
			for _, expected := range []string{
				"[" + test.code + "]", "client_diagnostic_id=A0B1C2D3E4", "step=" + test.step,
				"reason=" + test.reason, "route=" + test.route, "phase=" + test.stage,
			} {
				if !strings.Contains(body, expected) {
					t.Errorf("compatible observation missing %q: %s", expected, body)
				}
			}
		})
	}
}

func TestRecordClientErrorProjectComposerPreservesUnknownObservations(t *testing.T) {
	for _, outcome := range []string{"not-called", "failed", "succeeded"} {
		t.Run(outcome, func(t *testing.T) {
			record := projectComposerDiagnosticTestRecord()
			detail := record.ProjectComposer
			detail.ListTabsOutcome = outcome
			detail.HasTab, detail.HasSession, detail.AdapterCurrent = nil, nil, nil
			no := false
			detail.TabFound, detail.PathMatches = &no, &no
			if outcome != "succeeded" {
				yes := true
				detail.PathMatches = &yes
			}
			body := recordProjectComposerJournal(t, record)
			for _, key := range []string{"has_tab=", "has_session=", "adapter_current="} {
				if strings.Contains(body, key) {
					t.Errorf("missing observation was serialized: %s", body)
				}
			}
			for _, key := range []string{"tab_found=false", "path_matches=false"} {
				if strings.Contains(body, key) != (outcome == "succeeded") {
					t.Errorf("lookup observation %s disagrees with outcome %s: %s", key, outcome, body)
				}
			}
		})
	}
}

func TestRecordClientErrorProjectComposerRejectsTextAndMalformedFrames(t *testing.T) {
	record := projectComposerDiagnosticTestRecord()
	raw := `{"projectComposer":{"message":"private-message","stack":"private-stack","context":{"prompt":"private-prompt"}}}`
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		t.Fatal(err)
	}
	body := recordProjectComposerJournal(t, record)
	if !strings.Contains(body, "client_diagnostic_id=A0B1C2D3E4") || strings.Contains(body, "private-") {
		t.Fatalf("unknown text fields crossed the typed boundary: %s", body)
	}
	tests := []struct {
		name   string
		mutate func(*ProjectComposerDiagnostic)
	}{
		{"cause text", func(d *ProjectComposerDiagnostic) { d.CauseType = "TypeError private-cause" }},
		{"build path", func(d *ProjectComposerDiagnostic) { d.BuildCommit = `C:\Users\private-build` }},
		{"frame category", func(d *ProjectComposerDiagnostic) { d.Frames[0].Category = "private-category" }},
		{"negative ordinal", func(d *ProjectComposerDiagnostic) { d.Frames[0].Ordinal = -1 }},
		{"large ordinal", func(d *ProjectComposerDiagnostic) { d.Frames[0].Ordinal = 64 }},
		{"negative line", func(d *ProjectComposerDiagnostic) { d.Frames[0].Line = -1 }},
		{"large line", func(d *ProjectComposerDiagnostic) { d.Frames[0].Line = 1_000_000_001 }},
		{"negative column", func(d *ProjectComposerDiagnostic) { d.Frames[0].Column = -1 }},
		{"large column", func(d *ProjectComposerDiagnostic) { d.Frames[0].Column = 1_000_000_001 }},
		{"missing asset", func(d *ProjectComposerDiagnostic) { d.Frames[0].AssetHash = "" }},
		{"invalid asset", func(d *ProjectComposerDiagnostic) { d.Frames[0].AssetHash = "private/" }},
		{"long asset", func(d *ProjectComposerDiagnostic) { d.Frames[0].AssetHash = strings.Repeat("x", 10000) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := projectComposerDiagnosticTestRecord()
			test.mutate(record.ProjectComposer)
			body := recordProjectComposerJournal(t, record)
			if !strings.Contains(body, "client_diagnostic_id=A0B1C2D3E4") || strings.Contains(body, "private") {
				t.Fatalf("optional malformed field escaped validation: %s", body)
			}
			if test.name == "cause text" && !strings.Contains(body, "cause_type=unknown") {
				t.Errorf("unrecognized cause type was not unknown: %s", body)
			}
			if strings.Contains(test.name, "ordinal") || strings.Contains(test.name, "line") || strings.Contains(test.name, "column") || strings.Contains(test.name, "asset") || test.name == "frame category" {
				if strings.Contains(body, "frame=") {
					t.Errorf("invalid frame was retained: %s", body)
				}
			}
		})
	}
}

func TestRecordClientErrorProjectComposerBoundsFramesAndPreservesIDs(t *testing.T) {
	record := projectComposerDiagnosticTestRecord()
	detail := record.ProjectComposer
	detail.CauseType = "AggregateError"
	detail.PhaseAtStart = "creating-session"
	detail.BuildCommit = strings.Repeat("a", 40)
	detail.Frames = make([]ProjectComposerFrame, 100)
	for index := range detail.Frames {
		detail.Frames[index] = ProjectComposerFrame{Category: "framework", Ordinal: 63, Line: 1_000_000_000, Column: 1_000_000_000, AssetHash: "aBcD_-09"}
	}
	second := record
	second.DiagnosticID = "A1B2C3D4E5"
	body := recordProjectComposerJournal(t, record, record, second)
	lines := strings.Split(strings.TrimSpace(body), "\n")
	if len(lines) != 2 {
		t.Fatalf("wanted two unique IDs and one suppressed duplicate, got %d lines: %s", len(lines), body)
	}
	for _, id := range []string{record.DiagnosticID, second.DiagnosticID} {
		if strings.Count(body, "client_diagnostic_id="+id) != 1 {
			t.Errorf("correlation ID %q was lost or duplicated: %s", id, body)
		}
	}
	for _, line := range lines {
		if len(line)+1 > 2048 || strings.Contains(line, "[truncated]") || strings.Count(line, " frame=") != 4 {
			t.Errorf("frame or byte bound violated: %s", line)
		}
		if !strings.Contains(line, "build=aaaaaaaaaaaa ") || strings.Contains(line, "[redacted") {
			t.Errorf("bounded revision or frame was scrubbed: %s", line)
		}
	}
}

func projectComposerDiagnosticTestRecord() ClientErrorRecord {
	yes, no := true, false
	return ClientErrorRecord{
		Scope: "projectComposer.send", Code: "PROJECT_COMPOSER_SESSION_ACTIVATE_FAILED", ErrorType: "Error",
		OccurredAt: "2026-09-05T10:00:00.123Z", DiagnosticID: "A0B1C2D3E4",
		ProjectComposer: &ProjectComposerDiagnostic{
			Version: 1, Stage: "activate-session", Route: "fresh", PhaseAtStart: "draft", ExecutionTarget: "cloud",
			Step: "switch-tab", Reason: "exception", ListTabsOutcome: "succeeded", CauseType: "TypeError",
			HasTab: &yes, HasSession: &no, AdapterCurrent: &yes, TabFound: &yes, PathMatches: &yes,
			BuildCommit: "0123456789abcdef0123456789abcdef01234567",
			Frames:      []ProjectComposerFrame{{Category: "app", Ordinal: 0, Line: 123, Column: 456, AssetHash: "aBcD_-09"}},
		},
	}
}

func recordProjectComposerJournal(t *testing.T, records ...ClientErrorRecord) string {
	t.Helper()
	root := t.TempDir()
	t.Cleanup(closeDesktopErrorJournal)
	installDesktopErrorJournal(desktopProfile{Development: true, EnvironmentID: "0123456789abcdef", CacheRoot: root})
	for _, record := range records {
		(&App{}).RecordClientError(record)
	}
	closeDesktopErrorJournal()
	return readDesktopJournal(t, filepath.Join(root, "diagnostics", "errors"))
}

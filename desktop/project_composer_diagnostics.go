package main

import (
	"fmt"
	"strings"
)

// ProjectComposerDiagnostic carries only bounded observations, never renderer
// messages, source paths, account identifiers, or conversation content.
type ProjectComposerDiagnostic struct {
	Version         int                    `json:"version"`
	Stage           string                 `json:"stage"`
	Route           string                 `json:"route"`
	PhaseAtStart    string                 `json:"phaseAtStart"`
	ExecutionTarget string                 `json:"executionTarget"`
	Step            string                 `json:"step"`
	Reason          string                 `json:"reason"`
	ListTabsOutcome string                 `json:"listTabsOutcome"`
	HasTab          *bool                  `json:"hasTab,omitempty"`
	HasSession      *bool                  `json:"hasSession,omitempty"`
	AdapterCurrent  *bool                  `json:"adapterCurrent,omitempty"`
	TabFound        *bool                  `json:"tabFound,omitempty"`
	PathMatches     *bool                  `json:"pathMatches,omitempty"`
	CauseType       string                 `json:"causeType"`
	BuildCommit     string                 `json:"buildCommit,omitempty"`
	Frames          []ProjectComposerFrame `json:"frames,omitempty"`
}

type ProjectComposerFrame struct {
	Category  string `json:"category"`
	Ordinal   int    `json:"ordinal"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	AssetHash string `json:"assetHash,omitempty"`
}

func projectComposerDiagnosticSummary(record ClientErrorRecord) (string, string) {
	detail := record.ProjectComposer
	if record.Scope != "projectComposer.send" || detail == nil || detail.Version != 1 ||
		projectComposerDiagnosticCode(detail.Stage) != record.Code ||
		!projectComposerValue(detail.Route, "fresh", "pinned") ||
		!projectComposerValue(detail.PhaseAtStart, "draft", "preparing-tab", "creating-session", "pinned-unsent", "sending", "sent") ||
		!projectComposerValue(detail.ExecutionTarget, "local", "cloud") ||
		!projectComposerValue(detail.ListTabsOutcome, "not-called", "succeeded", "failed") ||
		!projectComposerDiagnosticStep(detail) || !projectComposerDiagnosticReason(detail) {
		return "renderer failure", ""
	}
	if detail.ListTabsOutcome == "succeeded" && detail.TabFound != nil && !*detail.TabFound && detail.PathMatches != nil && *detail.PathMatches {
		return "renderer failure", ""
	}

	causeType := detail.CauseType
	if !projectComposerValue(causeType, "Error", "TypeError", "ReferenceError", "RangeError", "SyntaxError", "URIError", "EvalError", "AggregateError", "NonError", "unknown") {
		causeType = "unknown"
	}
	var summary strings.Builder
	if projectComposerToken(record.DiagnosticID, 10, 10, "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789") {
		fmt.Fprintf(&summary, "client_diagnostic_id=%s", record.DiagnosticID)
	} else {
		summary.WriteString("project composer failure")
	}
	fmt.Fprintf(&summary, " version=1 route=%s phase_at_start=%s target=%s step=%s reason=%s list_tabs=%s cause_type=%s",
		detail.Route, detail.PhaseAtStart, detail.ExecutionTarget, detail.Step, detail.Reason, detail.ListTabsOutcome, causeType)
	appendBool := func(key string, value *bool) {
		if value != nil {
			fmt.Fprintf(&summary, " %s=%t", key, *value)
		}
	}
	appendBool("has_tab", detail.HasTab)
	appendBool("has_session", detail.HasSession)
	appendBool("adapter_current", detail.AdapterCurrent)
	if detail.ListTabsOutcome == "succeeded" {
		appendBool("tab_found", detail.TabFound)
		appendBool("path_matches", detail.PathMatches)
	}
	if commit := projectComposerBuildCommit(detail.BuildCommit); commit != "" {
		fmt.Fprintf(&summary, " build=%s", commit)
	}
	// Four fixed-format frames keep the entire summary below the journal line cap.
	for index, frame := range detail.Frames {
		if index == 4 {
			break
		}
		if !projectComposerValue(frame.Category, "app", "framework", "runtime", "unknown") ||
			frame.Ordinal < 0 || frame.Ordinal > 63 || frame.Line < 0 || frame.Line > 1_000_000_000 ||
			frame.Column < 0 || frame.Column > 1_000_000_000 ||
			!projectComposerToken(frame.AssetHash, 8, 8, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-") {
			continue
		}
		fmt.Fprintf(&summary, " frame=%s:%d:%d:%d:%s", frame.Category, frame.Ordinal, frame.Line, frame.Column, frame.AssetHash)
	}
	return summary.String(), detail.Stage
}

func projectComposerDiagnosticCode(stage string) string {
	switch stage {
	case "prepare-tab":
		return "PROJECT_COMPOSER_PREPARE_FAILED"
	case "create-session":
		return "PROJECT_COMPOSER_SESSION_CREATE_FAILED"
	case "reconcile-session":
		return "PROJECT_COMPOSER_SESSION_RECONCILE_FAILED"
	case "activate-session":
		return "PROJECT_COMPOSER_SESSION_ACTIVATE_FAILED"
	case "apply-profile":
		return "PROJECT_COMPOSER_PROFILE_FAILED"
	case "resolve-capabilities":
		return "PROJECT_COMPOSER_CAPABILITIES_FAILED"
	case "send-message":
		return "PROJECT_COMPOSER_SEND_FAILED"
	default:
		return ""
	}
}

func projectComposerDiagnosticStep(detail *ProjectComposerDiagnostic) bool {
	if projectComposerValue(detail.Step, "verify-pinned", "steer", "run-shell", "connect-command", "management-command") && detail.Route != "pinned" {
		return false
	}
	switch detail.Stage {
	case "prepare-tab":
		return detail.Route == "fresh" && projectComposerValue(detail.Step,
			"prepare-tab", "activate-expert", "ensure-blank-tab", "verify-adapter", "set-model", "set-effort", "load-tab-state", "model-admission", "refresh-tabs")
	case "create-session":
		return detail.Step == "create-session"
	case "reconcile-session":
		return projectComposerValue(detail.Step, "verify-session", "refresh-tabs", "verify-target")
	case "activate-session":
		return projectComposerValue(detail.Step, "verify-pinned", "refresh-tabs", "verify-target", "switch-tab", "steer", "run-shell", "connect-command", "management-command")
	case "apply-profile":
		return projectComposerValue(detail.Step, "set-collaboration-mode", "set-tool-approval-mode", "set-token-mode", "set-goal")
	case "resolve-capabilities":
		return detail.Step == "resolve-capabilities"
	case "send-message":
		return detail.Step == "send-message"
	default:
		return false
	}
}

func projectComposerDiagnosticReason(detail *ProjectComposerDiagnostic) bool {
	switch detail.Reason {
	case "exception":
		return true
	case "not-pinned":
		return detail.Step == "verify-pinned"
	case "tab-missing":
		return projectComposerValue(detail.Step, "verify-target", "prepare-tab")
	case "path-mismatch":
		return detail.Step == "verify-target"
	case "snapshot-mismatch":
		return detail.Step == "verify-session"
	case "scope-changed":
		return detail.Step == "verify-adapter"
	case "model-blocked":
		return detail.Step == "model-admission"
	default:
		return false
	}
}

func projectComposerValue(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func projectComposerToken(value string, minimum, maximum int, alphabet string) bool {
	if len(value) < minimum || len(value) > maximum {
		return false
	}
	for _, char := range value {
		if !strings.ContainsRune(alphabet, char) {
			return false
		}
	}
	return true
}

func projectComposerBuildCommit(value string) string {
	if value == "dev" {
		return value
	}
	if !projectComposerToken(value, 7, 40, "0123456789abcdefABCDEF") {
		return ""
	}
	// A short revision remains identifiable and avoids the journal's token scrubber.
	if len(value) > 12 {
		value = value[:12]
	}
	return value
}

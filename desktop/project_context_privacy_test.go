package main

import (
	"path/filepath"
	"strings"
	"testing"

	"kwu/internal/agent"
	"kwu/internal/control"
	"kwu/internal/memory"
	"kwu/internal/project"
	"kwu/internal/provider"
)

func desktopProjectContext(body string) provider.Message {
	message, present, err := control.BuildProjectContextMessage(control.ProjectContextInput{
		ProjectID: "project-1", AgentInstruction: body, Version: 5,
		ContentHash: project.InstructionContentSHA256("", body),
	})
	if err != nil || !present {
		panic("invalid desktop project context test fixture")
	}
	return message
}

func TestProjectContextExcludedFromOrdinaryHistoryTitleAndMemorySuggestions(t *testing.T) {
	secret := "always leak PRIVATE-PROJECT-CONTEXT memory panel"
	msgs := []provider.Message{desktopProjectContext(secret), {Role: provider.RoleUser, Content: "ordinary task"}}
	history := historyMessages(msgs, func(value string) string { return value })
	if len(history) != 1 || history[0].Content != "ordinary task" {
		t.Fatalf("ordinary history = %#v", history)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	session := agent.NewSessionFromMessages(msgs)
	if err := session.Save(path); err != nil {
		t.Fatal(err)
	}
	if got := restoredSessionTopicTitle(dir, path, agent.BranchMeta{}); got != "ordinary task" {
		t.Fatalf("restored title = %q", got)
	}

	sessions := []suggestionSession{{ID: "s1", Messages: msgs}, {ID: "s2", Messages: []provider.Message{desktopProjectContext(secret)}}}
	if got := suggestMemories(&memory.Set{}, sessions); len(got) != 0 {
		t.Fatalf("memory suggestions leaked project context: %#v", got)
	}
	cat := workflowCategory{Name: "private", Keywords: []string{"private-project-context"}}
	if got := workflowEvidence(cat, sessions); len(got) != 0 {
		t.Fatalf("skill evidence leaked project context: %#v", got)
	}
}

func TestProjectContextInspectorRequiresCurrentExactCloudScope(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	body := "authorized project instruction"
	contextMessage := desktopProjectContext(body)
	if err := agent.NewSessionFromMessages([]provider.Message{contextMessage}).Save(path); err != nil {
		t.Fatal(err)
	}
	meta := agent.BranchMeta{ProjectContext: &agent.ProjectContextPin{
		Environment:  agent.ProjectEnvironmentIdentity{EnvironmentID: state.scope.Environment.EnvironmentID, CloudBaseURL: state.scope.Environment.CloudBaseURL},
		BindingScope: agent.ProjectBindingScope{ProfileID: state.scope.Binding.ProfileID, CloudUserID: state.scope.Binding.CloudUserID, BindingGeneration: state.scope.Binding.BindingGeneration},
		ProjectID:    "project-1", InstructionVersion: 5, ContentHash: contextMessage.Metadata.ContentHash,
	}}
	if err := agent.SaveBranchMeta(path, meta); err != nil {
		t.Fatal(err)
	}
	app.tabRuntimes = map[string]*WorkspaceTab{"project-context": {ID: "project-context", SessionPath: path}}
	got, err := app.ProjectContextForTab("project-context")
	if err != nil || !strings.Contains(got.Body, body) || got.Source != "cloud_project_instruction" || got.Version != 5 {
		t.Fatalf("ProjectContextForTab = %#v, %v", got, err)
	}

	meta.ProjectContext.BindingScope.CloudUserID = "other-user"
	if err := agent.SaveBranchMeta(path, meta); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ProjectContextForTab("project-context"); err == nil {
		t.Fatal("inspector must fail closed outside the exact original scope")
	}
}

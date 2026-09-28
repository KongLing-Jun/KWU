package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"testing"
	"time"

	"kwu/internal/project"
)

func TestWorkBuddyProjectRestartReplaysAllOperationsWithOriginalKeyPayloadReadbackAndEviction(t *testing.T) {
	tests := []struct {
		name             string
		operation        project.ProjectCommandOperation
		projectID        string
		resourceID       string
		payload          map[string]any
		wantReadbackPath string
	}{
		{
			name: "project create", operation: project.ProjectCommandCreate,
			payload: map[string]any{
				"name": "Recovered", "description": nil,
				"human_description": "Recovered template", "agent_instruction": "Recovered initial instruction",
			},
			wantReadbackPath: "/api/v1/projects/project-1",
		},
		{
			name: "project aggregate create", operation: project.ProjectCommandCreate,
			payload: map[string]any{
				"name": "Recovered agg", "description": nil,
				"human_description": "Recovered template", "agent_instruction": "Recovered aggregate instruction",
				"bindings": []project.ProjectBindingInput{{Kind: "connector", RegistryRef: "notion", Position: 0, Config: map[string]any{}}},
			},
			wantReadbackPath: "/api/v1/projects/project-1",
		},
		{
			name: "project patch", operation: project.ProjectCommandPatch, projectID: "project-1",
			payload:          map[string]any{"expected_version": uint64(1), "name": "Recovered patch", "description": nil},
			wantReadbackPath: "/api/v1/projects/project-1",
		},
		{
			name: "instruction put", operation: project.ProjectCommandInstructionPut, projectID: "project-1",
			payload:          map[string]any{"expected_version": uint64(1), "human_description": "Recovered human", "agent_instruction": "Recovered agent"},
			wantReadbackPath: "/api/v1/projects/project-1/instructions",
		},
		{
			name: "work item create", operation: project.ProjectCommandWorkItemCreate, projectID: "project-1",
			payload:          map[string]any{"title": "Recovered work", "description": "Recovered detail", "status": "in_progress"},
			wantReadbackPath: "/api/v1/work-items/work-1",
		},
		{
			name: "work item patch", operation: project.ProjectCommandWorkItemPatch, projectID: "project-1", resourceID: "work-1",
			payload:          map[string]any{"expected_version": uint64(1), "title": "Recovered patch", "description": nil},
			wantReadbackPath: "/api/v1/work-items/work-1",
		},
		{
			name: "work item move", operation: project.ProjectCommandWorkItemMove, projectID: "project-1", resourceID: "work-1",
			payload:          map[string]any{"expected_version": uint64(1), "target_status": "done", "before_work_item_id": "work-2"},
			wantReadbackPath: "/api/v1/work-items/work-1",
		},
		{
			name: "work item delete", operation: project.ProjectCommandWorkItemDelete, projectID: "project-1", resourceID: "work-1",
			payload:          map[string]any{"expected_version": uint64(1)},
			wantReadbackPath: "/api/v1/work-items/work-1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, state := newWorkBuddyProjectApp(t, nil)
			services := isolatedProjectReplayServices(t, state)
			record, err := services.Journal.Prepare(context.Background(), services.Scope, project.PrepareProjectCommandInput{
				Operation: test.operation, ProjectID: test.projectID, ResourceID: test.resourceID, Payload: test.payload,
			})
			if err != nil {
				t.Fatal(err)
			}

			if err := replayPendingCloudProjectCommands(context.Background(), services); err != nil {
				t.Fatal(err)
			}
			if state.lastKey != record.IdempotencyKey {
				t.Fatalf("replay key = %q, want original %q", state.lastKey, record.IdempotencyKey)
			}
			assertCanonicalProjectReplayPayload(t, state.lastBody, record.CanonicalPayload)
			if state.lastReadbackPath != test.wantReadbackPath {
				t.Fatalf("authoritative readback = %q, want %q", state.lastReadbackPath, test.wantReadbackPath)
			}
			assertProjectReplayJournalEmpty(t, services)
		})
	}
}

func TestWorkBuddyProjectRestartReplayKeepsUnknownOnNetworkFailure(t *testing.T) {
	_, state := newWorkBuddyProjectApp(t, nil)
	services := isolatedProjectReplayServices(t, state)
	state.networkMutation = true
	record, err := services.Journal.Prepare(context.Background(), services.Scope, project.PrepareProjectCommandInput{
		Operation: project.ProjectCommandCreate,
		Payload:   map[string]any{"name": "Unknown result", "description": nil},
	})
	if err != nil {
		t.Fatal(err)
	}
	replayErr := replayPendingCloudProjectCommands(context.Background(), services)
	if replayErr == nil {
		t.Fatal("network-unknown replay unexpectedly succeeded")
	}
	pending, err := services.Journal.Pending(context.Background(), services.Scope)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].IdempotencyKey != record.IdempotencyKey || pending[0].State != project.ProjectCommandUnknown {
		t.Fatalf("pending after network-unknown replay = %#v; replay error = %v; key = %q; body = %s", pending, replayErr, state.lastKey, state.lastBody)
	}
}

func TestWorkBuddyProjectRestartReplayDiscardsConclusiveRejection(t *testing.T) {
	_, state := newWorkBuddyProjectApp(t, nil)
	services := isolatedProjectReplayServices(t, state)
	state.instructionConflict = true
	_, err := services.Journal.Prepare(context.Background(), services.Scope, project.PrepareProjectCommandInput{
		Operation: project.ProjectCommandInstructionPut,
		ProjectID: "project-1",
		Payload: map[string]any{
			"expected_version": uint64(1), "human_description": "Rejected human", "agent_instruction": "Rejected agent",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := replayPendingCloudProjectCommands(context.Background(), services); err == nil {
		t.Fatal("conclusively rejected replay unexpectedly succeeded")
	}
	assertProjectReplayJournalEmpty(t, services)
}

func TestWorkBuddyProjectRestartReplayCancelsWithScopeLifecycle(t *testing.T) {
	_, state := newWorkBuddyProjectApp(t, nil)
	services := isolatedProjectReplayServices(t, state)
	state.blockMutation = true
	state.mutationStarted = make(chan struct{})
	record, err := services.Journal.Prepare(context.Background(), services.Scope, project.PrepareProjectCommandInput{
		Operation: project.ProjectCommandCreate,
		Payload: map[string]any{
			"name": "Canceled replay", "description": nil,
			"human_description": "Canceled template", "agent_instruction": "Canceled initial instruction",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	operationCtx, cancelOperation := services.operationContext(context.Background())
	defer cancelOperation()
	replayDone := make(chan error, 1)
	go func() { replayDone <- replayPendingCloudProjectCommands(operationCtx, services) }()
	waitForProjectReplaySignal(t, state.mutationStarted, "mutation start")
	services.cancel()
	waitForProjectReplaySignal(t, operationCtx.Done(), "scope lifecycle cancellation")
	if err := <-replayDone; err == nil {
		t.Fatal("scope-canceled replay unexpectedly succeeded")
	}

	pending, err := services.Journal.Pending(context.Background(), services.Scope)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].IdempotencyKey != record.IdempotencyKey {
		t.Fatalf("scope-canceled replay did not retain its command: %#v", pending)
	}
}

func isolatedProjectReplayServices(t *testing.T, state *workBuddyProjectAppState) *cloudProjectServices {
	t.Helper()
	original := state.services
	original.cancel()
	services := *original
	services.lifecycle, services.cancel = context.WithCancel(context.Background())
	root := t.TempDir()
	scopes := cloudProjectScopeResolver{bindings: state.runtime.bindings, environment: state.runtime.projectEnvironment, actor: original.Actor}
	journal, err := project.NewProjectCommandJournal(project.ProjectCommandJournalOptions{Root: root, Scopes: scopes, Now: time.Now, Random: rand.Reader})
	if err != nil {
		t.Fatal(err)
	}
	services.StateRoot = root
	services.Journal = journal
	t.Cleanup(services.cancel)
	return &services
}

func assertProjectReplayJournalEmpty(t *testing.T, services *cloudProjectServices) {
	t.Helper()
	pending, err := services.Journal.Pending(context.Background(), services.Scope)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("pending replay commands = %#v, want empty", pending)
	}
}

func assertCanonicalProjectReplayPayload(t *testing.T, got string, want json.RawMessage) {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewBufferString(got))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	canonical, err := project.CanonicalProjectRequestBytes(value)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(canonical, want) {
		t.Fatalf("replay payload = %s, want original canonical %s", canonical, want)
	}
}

func waitForProjectReplaySignal(t *testing.T, signal <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for replay %s", description)
	}
}

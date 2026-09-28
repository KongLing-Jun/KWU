package project

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"kwu/internal/cloud"
)

func TestProjectCommandJournalCanonicalHashAndDurableReplay(t *testing.T) {
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	scope := gatewayTestScope(t, "user-1", 7)
	resolver := &gatewayTestScopeResolver{scope: scope}
	root := t.TempDir()
	journal := newProjectCommandJournalForTest(t, root, resolver, func() time.Time { return now })
	payload := map[string]any{"name": "Alpha", "description": nil}
	canonical, err := CanonicalProjectRequestBytes(payload)
	if err != nil {
		t.Fatal(err)
	}
	if string(canonical) != `{"description":null,"name":"Alpha"}` {
		t.Fatalf("canonical bytes = %s", canonical)
	}
	record, err := journal.Prepare(context.Background(), scope, PrepareProjectCommandInput{Operation: ProjectCommandCreate, Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	if record.ProjectScopeKey != scope.Key || record.State != ProjectCommandPending || record.LocalPayloadHash != "0cfefb00a54da63c04281d17ff31e49af491da86e41378aa4633718711bbc309" || !bytes.Equal(record.CanonicalPayload, canonical) {
		t.Fatalf("Prepare = %#v", record)
	}
	serverInput := append([]byte("reasonix:projects-v1:server-request-digest:v1\nproject.create\n"), canonical...)
	serverLikeUnkeyed := sha256.Sum256(serverInput)
	if record.LocalPayloadHash == hex.EncodeToString(serverLikeUnkeyed[:]) {
		t.Fatal("local payload hash impersonates the server digest input")
	}

	restarted := newProjectCommandJournalForTest(t, root, resolver, func() time.Time { return now })
	pending, err := restarted.Pending(context.Background(), scope)
	if err != nil || len(pending) != 1 || pending[0].IdempotencyKey != record.IdempotencyKey || !bytes.Equal(pending[0].CanonicalPayload, canonical) {
		t.Fatalf("restart Pending = %#v, %v", pending, err)
	}
	if _, err := restarted.VerifyReplay(context.Background(), scope, record.IdempotencyKey, ProjectCommandCreate, "", "", payload); err != nil {
		t.Fatalf("VerifyReplay same payload: %v", err)
	}
	if _, err := restarted.VerifyReplay(context.Background(), scope, record.IdempotencyKey, ProjectCommandCreate, "", "", map[string]any{"name": "Beta", "description": nil}); err == nil || !strings.Contains(err.Error(), "payload substitution") {
		t.Fatalf("VerifyReplay changed payload error = %v", err)
	}
	if err := restarted.MarkUnknown(context.Background(), scope, record.IdempotencyKey); err != nil {
		t.Fatal(err)
	}
	pending, err = restarted.Pending(context.Background(), scope)
	if err != nil || len(pending) != 1 || pending[0].State != ProjectCommandUnknown {
		t.Fatalf("unknown Pending = %#v, %v", pending, err)
	}
}

func TestProjectCommandJournalCanonicalFixtureVectors(t *testing.T) {
	tests := []struct {
		name      string
		payload   map[string]any
		canonical string
		hash      string
	}{
		{"project create", map[string]any{"name": "Alpha", "description": nil}, `{"description":null,"name":"Alpha"}`, "0cfefb00a54da63c04281d17ff31e49af491da86e41378aa4633718711bbc309"},
		{"instruction put", map[string]any{"expected_version": 1, "human_description": "", "agent_instruction": "Ship safely"}, `{"agent_instruction":"Ship safely","expected_version":1,"human_description":""}`, "49ef4181963dc6d0baad41cec4f86a6dafed72b6217a5ebbf95c9853f504b258"},
		{"work item create", map[string]any{"title": "First task", "description": nil, "status": "todo"}, `{"description":null,"status":"todo","title":"First task"}`, "3c5ff350d7e52ff526f824fb6694c360d714b009bd1fe60280cbdc5b26b9bcd7"},
		{"work item move", map[string]any{"expected_version": 3, "target_status": "in_progress", "before_work_item_id": nil}, `{"before_work_item_id":null,"expected_version":3,"target_status":"in_progress"}`, "a8f2514f469476f262cf514559c31fccdbaed0054d4dc7ebcfc048ec4af5887c"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			canonical, err := CanonicalProjectRequestBytes(test.payload)
			if err != nil {
				t.Fatal(err)
			}
			if string(canonical) != test.canonical {
				t.Fatalf("canonical bytes = %s", canonical)
			}
			sum := sha256.Sum256(canonical)
			if got := hex.EncodeToString(sum[:]); got != test.hash {
				t.Fatalf("local payload hash = %s", got)
			}
		})
	}
}

func TestProjectCommandJournalAcceptsStrictAggregateCreateBindings(t *testing.T) {
	now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	scope := gatewayTestScope(t, "user-1", 7)
	newJournal := func() *ProjectCommandJournal {
		return newProjectCommandJournalForTest(t, t.TempDir(), &gatewayTestScopeResolver{scope: scope}, func() time.Time { return now })
	}
	valid := map[string]any{
		"name": "Aggregate",
		"bindings": []ProjectBindingInput{
			{Kind: "connector", RegistryRef: "notion", Position: 0, Config: map[string]any{}},
			{Kind: "skill", RegistryRef: "@tencent-adm/cloudbase", Position: 0, Config: map[string]any{}},
		},
	}
	record, err := newJournal().Prepare(context.Background(), scope, PrepareProjectCommandInput{Operation: ProjectCommandCreate, Payload: valid})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(record.CanonicalPayload, []byte(`"bindings"`)) || !bytes.Contains(record.CanonicalPayload, []byte(`"registry_ref":"notion"`)) {
		t.Fatalf("aggregate create payload = %s", record.CanonicalPayload)
	}

	invalid := []any{
		map[string]any{"name": "Aggregate", "bindings": []any{}},
		map[string]any{"name": "Aggregate", "bindings": []any{map[string]any{"kind": "connector", "registry_ref": "notion", "position": 0, "config": map[string]any{}, "extra": true}}},
		map[string]any{"name": "Aggregate", "bindings": []any{
			map[string]any{"kind": "connector", "registry_ref": "notion", "position": 0, "config": map[string]any{}},
			map[string]any{"kind": "connector", "registry_ref": "github", "position": 0, "config": map[string]any{}},
		}},
	}
	for index, payload := range invalid {
		if _, err := newJournal().Prepare(context.Background(), scope, PrepareProjectCommandInput{Operation: ProjectCommandCreate, Payload: payload}); err == nil {
			t.Fatalf("invalid aggregate create payload %d was accepted", index)
		}
	}
}

func TestProjectCommandJournalOperationTargetAndCanonicalParity(t *testing.T) {
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	scope := gatewayTestScope(t, "user-1", 7)
	journal := newProjectCommandJournalForTest(t, t.TempDir(), &gatewayTestScopeResolver{scope: scope}, func() time.Time { return now })

	if ProjectCommandWorkItemPatch != "work_item.patch" {
		t.Fatalf("work item patch operation = %q", ProjectCommandWorkItemPatch)
	}
	if ProjectCommandWorkItemDelete != "work_item.delete" {
		t.Fatalf("work item delete operation = %q", ProjectCommandWorkItemDelete)
	}
	if _, err := journal.Prepare(context.Background(), scope, PrepareProjectCommandInput{
		Operation: ProjectCommandCreate,
		Payload:   map[string]any{"name": "Alpha", "title": "cross-operation"},
	}); err == nil {
		t.Fatal("Prepare accepted a cross-operation field")
	}
	canonical, err := CanonicalProjectRequestBytes(map[string]any{"name": "<>&"})
	if err != nil {
		t.Fatal(err)
	}
	if string(canonical) != `{"name":"<>&"}` {
		t.Fatalf("canonical HTML characters = %s", canonical)
	}

	record, err := journal.Prepare(context.Background(), scope, PrepareProjectCommandInput{
		Operation: ProjectCommandWorkItemPatch, ProjectID: "project-1", ResourceID: "work-item-1",
		Payload: map[string]any{"expected_version": 1, "title": "Updated"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.ResourceID != "work-item-1" {
		t.Fatalf("record resource target = %q", record.ResourceID)
	}
	if _, err := journal.VerifyReplay(context.Background(), scope, record.IdempotencyKey, ProjectCommandWorkItemPatch, "project-1", "work-item-2", map[string]any{"expected_version": 1, "title": "Updated"}); err == nil {
		t.Fatal("VerifyReplay accepted another work item route target")
	}
}

func TestProjectCommandJournalAcceptsPausedAndTreatsNullMoveAnchorsAsAbsent(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	journal := newProjectCommandJournalForTest(t, t.TempDir(), &gatewayTestScopeResolver{scope: scope}, time.Now)
	if _, err := journal.Prepare(context.Background(), scope, PrepareProjectCommandInput{Operation: ProjectCommandWorkItemCreate, ProjectID: "project-1", Payload: map[string]any{"title": "Paused", "status": "paused"}}); err != nil {
		t.Fatalf("prepare paused create: %v", err)
	}
	if _, err := journal.Prepare(context.Background(), scope, PrepareProjectCommandInput{Operation: ProjectCommandWorkItemMove, ProjectID: "project-1", ResourceID: "work-1", Payload: map[string]any{"expected_version": 1, "target_status": "paused", "before_work_item_id": nil, "after_work_item_id": nil}}); err != nil {
		t.Fatalf("prepare paused move with null anchors: %v", err)
	}
}

func TestProjectCommandJournalAcceptsWorkItemCollaborationFields(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	journal := newProjectCommandJournalForTest(t, t.TempDir(), &gatewayTestScopeResolver{scope: scope}, time.Now)
	start := time.Date(2026, 8, 8, 9, 0, 0, 0, time.UTC)
	due := start.Add(24 * time.Hour)
	create := map[string]any{
		"title": "Assigned", "assignee_user_id": "user-2", "priority": "high", "start_at": start, "due_at": due,
	}
	if _, err := journal.Prepare(context.Background(), scope, PrepareProjectCommandInput{Operation: ProjectCommandWorkItemCreate, ProjectID: "project-1", Payload: create}); err != nil {
		t.Fatalf("prepare enriched work item create: %v", err)
	}
	patch := map[string]any{
		"expected_version": 1, "assignee_user_id": nil, "priority": "urgent", "start_at": nil, "due_at": due,
	}
	if _, err := journal.Prepare(context.Background(), scope, PrepareProjectCommandInput{Operation: ProjectCommandWorkItemPatch, ProjectID: "project-1", ResourceID: "work-1", Payload: patch}); err != nil {
		t.Fatalf("prepare enriched work item patch: %v", err)
	}
}

func TestProjectCommandJournalRejectsInvalidWorkItemCollaborationFields(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	tests := []struct {
		name    string
		payload map[string]any
	}{
		{name: "unknown priority", payload: map[string]any{"title": "Assigned", "priority": "critical"}},
		{name: "invalid assignee", payload: map[string]any{"title": "Assigned", "assignee_user_id": ""}},
		{name: "invalid time", payload: map[string]any{"title": "Assigned", "start_at": "tomorrow"}},
		{name: "inverted range", payload: map[string]any{"title": "Assigned", "start_at": "2026-08-09T09:00:00Z", "due_at": "2026-08-08T09:00:00Z"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			journal := newProjectCommandJournalForTest(t, t.TempDir(), &gatewayTestScopeResolver{scope: scope}, time.Now)
			if _, err := journal.Prepare(context.Background(), scope, PrepareProjectCommandInput{Operation: ProjectCommandWorkItemCreate, ProjectID: "project-1", Payload: test.payload}); err == nil {
				t.Fatal("Prepare accepted invalid collaboration fields")
			}
		})
	}
}

func TestProjectCommandJournalSafeIntegerBoundaries(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		operation  ProjectCommandOperation
		projectID  string
		resourceID string
		payload    map[string]any
	}{
		{name: "project patch", operation: ProjectCommandPatch, projectID: "project-1", payload: map[string]any{"expected_version": MaxExpectedVersion, "name": "Updated"}},
		{name: "instruction put", operation: ProjectCommandInstructionPut, projectID: "project-1", payload: map[string]any{"expected_version": MaxExpectedVersion, "human_description": "", "agent_instruction": "Ship safely"}},
		{name: "work item patch", operation: ProjectCommandWorkItemPatch, projectID: "project-1", resourceID: "work-item-1", payload: map[string]any{"expected_version": MaxExpectedVersion, "title": "Updated"}},
		{name: "work item move", operation: ProjectCommandWorkItemMove, projectID: "project-1", resourceID: "work-item-1", payload: map[string]any{"expected_version": MaxExpectedVersion, "target_status": "in_progress"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolver := &gatewayTestScopeResolver{scope: scope}
			journal := newProjectCommandJournalForTest(t, t.TempDir(), resolver, func() time.Time { return now })
			input := PrepareProjectCommandInput{Operation: test.operation, ProjectID: test.projectID, ResourceID: test.resourceID, Payload: test.payload}
			if _, err := journal.Prepare(context.Background(), scope, input); err != nil {
				t.Fatalf("maximum expected version: %v", err)
			}
			unsafe := make(map[string]any, len(test.payload))
			for key, value := range test.payload {
				unsafe[key] = value
			}
			unsafe["expected_version"] = MaxSafeJSONInteger
			input.Payload = unsafe
			if _, err := journal.Prepare(context.Background(), scope, input); err == nil {
				t.Fatal("unsafe expected version was journaled")
			}
		})
	}

	journal := newProjectCommandJournalForTest(t, t.TempDir(), &gatewayTestScopeResolver{scope: scope}, func() time.Time { return now })
	record, err := journal.Prepare(context.Background(), scope, PrepareProjectCommandInput{Operation: ProjectCommandCreate, Payload: map[string]any{"name": "Alpha"}})
	if err != nil {
		t.Fatal(err)
	}
	unsafeVersion := MaxSafeJSONInteger + 1
	result := CommandResult{ProjectScopeKey: scope.Key, OperationID: "op-unsafe", ResourceKind: "project", ResourceID: "project-1", ProjectID: "project-1", CommittedVersion: unsafeVersion}
	readback := ProjectCommandReadback{ProjectScopeKey: scope.Key, ResourceKind: "project", ResourceID: "project-1", ProjectID: "project-1", Version: unsafeVersion}
	if err := journal.CompleteAfterReadback(context.Background(), scope, record.IdempotencyKey, result, readback); err == nil {
		t.Fatal("unsafe command/readback version completed the journal")
	}
}

func TestProjectCommandJournalStrictPersistenceAndAllReadbacks(t *testing.T) {
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	scope := gatewayTestScope(t, "user-1", 7)
	resolver := &gatewayTestScopeResolver{scope: scope}
	root := t.TempDir()
	journal := newProjectCommandJournalForTest(t, root, resolver, func() time.Time { return now })
	record, err := journal.Prepare(context.Background(), scope, PrepareProjectCommandInput{
		Operation: ProjectCommandInstructionPut, ProjectID: "project-1",
		Payload: map[string]any{"expected_version": 1, "human_description": "", "agent_instruction": "Ship safely"},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(journal.path(scope))
	if err != nil {
		t.Fatal(err)
	}
	caseAlias := strings.Replace(string(raw), `"schema":"reasonix.project-command-journal"`, `"schema":"reasonix.project-command-journal","Schema":"reasonix.project-command-journal"`, 1)
	if err := os.WriteFile(journal.path(scope), []byte(caseAlias), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := journal.Pending(context.Background(), scope); err == nil {
		t.Fatal("Pending accepted a case-variant persisted alias")
	}

	journal = newProjectCommandJournalForTest(t, t.TempDir(), resolver, func() time.Time { return now })
	if _, err := journal.Prepare(context.Background(), scope, PrepareProjectCommandInput{Operation: ProjectCommandCreate, Payload: map[string]any{"name": "Alpha"}}); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(journal.path(scope))
	if err != nil {
		t.Fatal(err)
	}
	index := bytes.Index(raw, []byte("reasonix.project-command-journal"))
	if index < 0 {
		t.Fatal("schema marker absent")
	}
	raw[index] = 0xff
	if err := os.WriteFile(journal.path(scope), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := journal.Pending(context.Background(), scope); err == nil || !strings.Contains(err.Error(), "UTF-8") {
		t.Fatalf("invalid UTF-8 Pending error = %v", err)
	}

	journal = newProjectCommandJournalForTest(t, t.TempDir(), resolver, func() time.Time { return now })
	record, err = journal.Prepare(context.Background(), scope, PrepareProjectCommandInput{
		Operation: ProjectCommandInstructionPut, ProjectID: "project-1",
		Payload: map[string]any{"expected_version": 1, "human_description": "", "agent_instruction": "Ship safely"},
	})
	if err != nil {
		t.Fatal(err)
	}
	wrongInstructionResult := CommandResult{ProjectScopeKey: scope.Key, OperationID: "op-wrong-instruction", ResourceKind: "instruction", ResourceID: "project-2", ProjectID: "project-1", CommittedVersion: 2}
	wrongInstructionReadback := ProjectCommandReadback{ProjectScopeKey: scope.Key, ResourceKind: "instruction", ResourceID: "project-2", ProjectID: "project-1", Version: 2}
	if err := journal.CompleteAfterReadback(context.Background(), scope, record.IdempotencyKey, wrongInstructionResult, wrongInstructionReadback); err == nil {
		t.Fatal("instruction completion accepted a different instruction resource ID")
	}
	pending, err := journal.Pending(context.Background(), scope)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].IdempotencyKey != record.IdempotencyKey {
		t.Fatalf("wrong instruction result removed pending command: %+v", pending)
	}
	result := CommandResult{ProjectScopeKey: scope.Key, OperationID: "op-instruction", ResourceKind: "instruction", ResourceID: "project-1", ProjectID: "project-1", CommittedVersion: 2}
	readback := ProjectCommandReadback{ProjectScopeKey: scope.Key, ResourceKind: "instruction", ResourceID: "project-1", ProjectID: "project-1", Version: 2}
	if err := journal.CompleteAfterReadback(context.Background(), scope, record.IdempotencyKey, result, readback); err != nil {
		t.Fatalf("instruction completion: %v", err)
	}

	record, err = journal.Prepare(context.Background(), scope, PrepareProjectCommandInput{
		Operation: ProjectCommandWorkItemPatch, ProjectID: "project-1", ResourceID: "work-item-1",
		Payload: map[string]any{"expected_version": 2, "title": "Ready"},
	})
	if err != nil {
		t.Fatal(err)
	}
	result = CommandResult{ProjectScopeKey: scope.Key, OperationID: "op-work-item", ResourceKind: "work_item", ResourceID: "work-item-1", ProjectID: "project-1", CommittedVersion: 3}
	readback = ProjectCommandReadback{ProjectScopeKey: scope.Key, ResourceKind: "work_item", ResourceID: "work-item-1", ProjectID: "project-1", Version: 3}
	if err := journal.CompleteAfterReadback(context.Background(), scope, record.IdempotencyKey, result, readback); err != nil {
		t.Fatalf("work item completion: %v", err)
	}
}

func TestProjectCommandJournalReadbackCompletionAndScope(t *testing.T) {
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	scope := gatewayTestScope(t, "user-1", 7)
	resolver := &gatewayTestScopeResolver{scope: scope}
	journal := newProjectCommandJournalForTest(t, t.TempDir(), resolver, func() time.Time { return now })
	record, err := journal.Prepare(context.Background(), scope, PrepareProjectCommandInput{
		Operation: ProjectCommandPatch, ProjectID: "project-001",
		Payload: map[string]any{"expected_version": 1, "name": "Updated"},
	})
	if err != nil {
		t.Fatal(err)
	}
	project := cacheTestPage(t, scope, 1).Items[0]
	project.Name = "Updated"
	project.Version = 2
	result := CommandResult{ProjectScopeKey: scope.Key, OperationID: "op-1", ResourceKind: "project", ResourceID: "project-001", ProjectID: "project-001", CommittedVersion: 2}
	bad := result
	bad.CommittedVersion = 3
	readback := ProjectCommandReadback{ProjectScopeKey: scope.Key, ResourceKind: "project", ResourceID: project.ProjectID, ProjectID: project.ProjectID, Version: project.Version}
	if err := journal.CompleteAfterReadback(context.Background(), scope, record.IdempotencyKey, bad, readback); err == nil {
		t.Fatal("CompleteAfterReadback accepted mismatched version")
	}
	if pending, err := journal.Pending(context.Background(), scope); err != nil || len(pending) != 1 {
		t.Fatalf("mismatch removed pending command: %#v, %v", pending, err)
	}
	if err := journal.CompleteAfterReadback(context.Background(), scope, record.IdempotencyKey, result, readback); err != nil {
		t.Fatal(err)
	}
	if pending, err := journal.Pending(context.Background(), scope); err != nil || len(pending) != 0 {
		t.Fatalf("completed command remains: %#v, %v", pending, err)
	}
	otherScope := gatewayTestScope(t, "user-2", 8)
	resolver.Set(otherScope)
	if pending, err := journal.Pending(context.Background(), otherScope); err != nil || len(pending) != 0 {
		t.Fatalf("other scope Pending = %#v, %v", pending, err)
	}
}

func TestProjectCommandJournalRejectsPayloadSubstitutionAndSecrets(t *testing.T) {
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	scope := gatewayTestScope(t, "user-1", 7)
	journal := newProjectCommandJournalForTest(t, t.TempDir(), &gatewayTestScopeResolver{scope: scope}, func() time.Time { return now })
	for _, payload := range []any{
		map[string]any{"name": "Alpha", "Authorization": "Bearer secret"},
		map[string]any{"name": "Alpha", "access_token": "secret"},
		map[string]any{"name": "bad\x00name"},
	} {
		if _, err := journal.Prepare(context.Background(), scope, PrepareProjectCommandInput{Operation: ProjectCommandCreate, Payload: payload}); err == nil {
			t.Fatalf("Prepare accepted unsafe payload %#v", payload)
		}
	}
	record, err := journal.Prepare(context.Background(), scope, PrepareProjectCommandInput{Operation: ProjectCommandCreate, Payload: map[string]any{"name": "Alpha"}})
	if err != nil {
		t.Fatal(err)
	}
	path := journal.path(scope)
	if filepath.Base(path) != "project_commands.json" {
		t.Fatalf("journal path = %s", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil {
		t.Fatal(err)
	} else if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("journal permissions = %o", info.Mode().Perm())
	}
	for _, forbidden := range []string{"access_token", "refresh_token", "Authorization", "server_request_digest", "digest_key"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("journal contains forbidden field %q", forbidden)
		}
	}
	tampered := bytes.Replace(raw, []byte(`"Alpha"`), []byte(`"Bravo"`), 1)
	if bytes.Equal(raw, tampered) {
		t.Fatal("test did not alter canonical replay payload")
	}
	if err := os.WriteFile(path, tampered, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := journal.Pending(context.Background(), scope); err == nil || !strings.Contains(err.Error(), "local_payload_hash") {
		t.Fatalf("tampered Pending error = %v", err)
	}
	if matches, _ := filepath.Glob(path + ".invalid-*"); len(matches) != 1 {
		t.Fatalf("tampered journal isolation = %v; key=%s", matches, record.IdempotencyKey)
	}
}

func TestProjectCommandJournalBoundsAndTTL(t *testing.T) {
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	scope := gatewayTestScope(t, "user-1", 7)
	resolver := &gatewayTestScopeResolver{scope: scope}

	t.Run("1000 unresolved blocks new command", func(t *testing.T) {
		journal := newProjectCommandJournalForTest(t, t.TempDir(), resolver, func() time.Time { return now })
		seedProjectCommandJournal(t, journal, scope, projectCommandJournalMaxUnresolved, 8, now)
		if _, err := journal.Prepare(context.Background(), scope, PrepareProjectCommandInput{Operation: ProjectCommandCreate, Payload: map[string]any{"name": "overflow"}}); err == nil || !strings.Contains(err.Error(), "1000 unresolved") {
			t.Fatalf("unresolved bound error = %v", err)
		}
	})

	t.Run("10 MiB blocks new command", func(t *testing.T) {
		journal := newProjectCommandJournalForTest(t, t.TempDir(), resolver, func() time.Time { return now })
		seedProjectCommandJournalNearSizeLimit(t, journal, scope, now)
		large := strings.Repeat("x", 200)
		if _, err := journal.Prepare(context.Background(), scope, PrepareProjectCommandInput{Operation: ProjectCommandCreate, Payload: map[string]any{"name": large}}); err == nil || !strings.Contains(err.Error(), "10 MiB") {
			t.Fatalf("size bound error = %v", err)
		}
	})

	t.Run("30 day expiry permits replacement", func(t *testing.T) {
		journal := newProjectCommandJournalForTest(t, t.TempDir(), resolver, func() time.Time { return now })
		seedProjectCommandJournal(t, journal, scope, 1, 8, now.Add(-projectCommandJournalRetention-time.Second))
		if pending, err := journal.Pending(context.Background(), scope); err != nil || len(pending) != 0 {
			t.Fatalf("expired Pending = %#v, %v", pending, err)
		}
		if _, err := journal.Prepare(context.Background(), scope, PrepareProjectCommandInput{Operation: ProjectCommandCreate, Payload: map[string]any{"name": "fresh"}}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("expired update and completion durably purge replay bytes", func(t *testing.T) {
		for _, action := range []string{"update", "complete"} {
			t.Run(action, func(t *testing.T) {
				journal := newProjectCommandJournalForTest(t, t.TempDir(), resolver, func() time.Time { return now })
				seedProjectCommandJournal(t, journal, scope, 1, 8, now.Add(-projectCommandJournalRetention-time.Second))
				key := strings.Repeat("a", 48) + "0000"
				if action == "update" {
					if err := journal.MarkUnknown(context.Background(), scope, key); err == nil {
						t.Fatal("expired update unexpectedly found command")
					}
				} else {
					result := CommandResult{ProjectScopeKey: scope.Key, OperationID: "op-expired", ResourceKind: "project", ResourceID: "project-expired", ProjectID: "project-expired", CommittedVersion: 1}
					readback := ProjectCommandReadback{ProjectScopeKey: scope.Key, ResourceKind: "project", ResourceID: "project-expired", ProjectID: "project-expired", Version: 1}
					if err := journal.CompleteAfterReadback(context.Background(), scope, key, result, readback); err == nil {
						t.Fatal("expired completion unexpectedly found command")
					}
				}
				var document projectCommandJournalDocument
				raw, err := os.ReadFile(journal.path(scope))
				if err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(raw, &document); err != nil || len(document.Commands) != 0 {
					t.Fatalf("expired commands remain = %d err=%v", len(document.Commands), err)
				}
			})
		}
	})
}

func TestProjectCommandJournalScopeRaceAndConcurrentPrepare(t *testing.T) {
	t.Run("scope switch rejects durable prepare", func(t *testing.T) {
		now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
		scopeA := gatewayTestScope(t, "user-1", 7)
		scopeB := gatewayTestScope(t, "user-2", 8)
		resolver := &gatewayTestScopeResolver{scope: scopeA}
		journal := newProjectCommandJournalForTest(t, t.TempDir(), resolver, func() time.Time { return now })
		entered := make(chan struct{})
		release := make(chan struct{})
		journal.beforeCommit = func() { close(entered); <-release }
		result := make(chan error, 1)
		go func() {
			_, err := journal.Prepare(context.Background(), scopeA, PrepareProjectCommandInput{Operation: ProjectCommandCreate, Payload: map[string]any{"name": "Alpha"}})
			result <- err
		}()
		<-entered
		resolver.Set(scopeB)
		close(release)
		if err := <-result; err == nil || !strings.Contains(err.Error(), string(cloud.ErrorProfileScopeMismatch)) {
			t.Fatalf("late Prepare error = %v", err)
		}
		if _, err := os.Stat(journal.path(scopeA)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("late command journal exists: %v", err)
		}
	})

	t.Run("repository instances preserve concurrent commands", func(t *testing.T) {
		now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
		scope := gatewayTestScope(t, "user-1", 7)
		resolver := &gatewayTestScopeResolver{scope: scope}
		root := t.TempDir()
		first := newProjectCommandJournalForTest(t, root, resolver, func() time.Time { return now })
		second := newProjectCommandJournalForTest(t, root, resolver, func() time.Time { return now })
		var wait sync.WaitGroup
		for index := 0; index < 24; index++ {
			index := index
			wait.Add(1)
			go func() {
				defer wait.Done()
				journal := first
				if index%2 == 1 {
					journal = second
				}
				_, err := journal.Prepare(context.Background(), scope, PrepareProjectCommandInput{Operation: ProjectCommandCreate, Payload: map[string]any{"name": "Project " + string(rune('A'+index))}})
				if err != nil {
					t.Errorf("Prepare(%d): %v", index, err)
				}
			}()
		}
		wait.Wait()
		if pending, err := first.Pending(context.Background(), scope); err != nil || len(pending) != 24 {
			t.Fatalf("concurrent Pending count=%d err=%v", len(pending), err)
		}
	})
}

func newProjectCommandJournalForTest(t *testing.T, root string, scopes ProjectScopeResolver, now func() time.Time) *ProjectCommandJournal {
	t.Helper()
	journal, err := NewProjectCommandJournal(ProjectCommandJournalOptions{Root: root, Scopes: scopes, Now: now, Random: rand.Reader})
	if err != nil {
		t.Fatal(err)
	}
	return journal
}

func seedProjectCommandJournal(t *testing.T, journal *ProjectCommandJournal, scope Scope, count, payloadBytes int, createdAt time.Time) {
	t.Helper()
	document := newProjectCommandJournalDocument(scope)
	for index := 0; index < count; index++ {
		payload := json.RawMessage(`{"name":"` + strings.Repeat("x", payloadBytes) + `"}`)
		sum := sha256.Sum256(payload)
		document.Commands = append(document.Commands, projectCommandWire{
			IdempotencyKey: strings.Repeat("a", 48) + hex.EncodeToString([]byte{byte(index >> 8), byte(index)}),
			Operation:      ProjectCommandCreate, CanonicalPayload: payload, LocalPayloadHash: hex.EncodeToString(sum[:]),
			State: ProjectCommandPending, CreatedAt: createdAt, UpdatedAt: createdAt,
		})
	}
	writeProjectCommandJournalSeed(t, journal, scope, document)
}

func seedProjectCommandJournalNearSizeLimit(t *testing.T, journal *ProjectCommandJournal, scope Scope, now time.Time) {
	t.Helper()
	document := newProjectCommandJournalDocument(scope)
	target := projectCommandJournalMaxBytes - 100
	appendInstruction := func(base projectCommandJournalDocument, agentBytes int) projectCommandJournalDocument {
		candidate := base
		payload := json.RawMessage(`{"agent_instruction":"` + strings.Repeat("a", agentBytes) + `","expected_version":1,"human_description":"` + strings.Repeat("h", 12000) + `"}`)
		sum := sha256.Sum256(payload)
		index := len(candidate.Commands)
		candidate.Commands = append(append([]projectCommandWire(nil), candidate.Commands...), projectCommandWire{
			IdempotencyKey: strings.Repeat("b", 48) + hex.EncodeToString([]byte{byte(index >> 8), byte(index)}),
			Operation:      ProjectCommandInstructionPut, ProjectID: "project-1", CanonicalPayload: payload, LocalPayloadHash: hex.EncodeToString(sum[:]),
			State: ProjectCommandPending, CreatedAt: now, UpdatedAt: now,
		})
		return candidate
	}
	for len(document.Commands) < projectCommandJournalMaxUnresolved {
		candidate := appendInstruction(document, 48000)
		payload, err := json.Marshal(candidate)
		if err != nil {
			t.Fatal(err)
		}
		if len(payload) > target {
			break
		}
		document = candidate
	}
	base := document
	low, high := 0, 48000
	for low <= high {
		middle := (low + high) / 2
		candidate := appendInstruction(base, middle)
		payload, err := json.Marshal(candidate)
		if err != nil {
			t.Fatal(err)
		}
		if len(payload) <= target {
			document = candidate
			low = middle + 1
		} else {
			high = middle - 1
		}
	}
	writeProjectCommandJournalSeed(t, journal, scope, document)
}

func writeProjectCommandJournalSeed(t *testing.T, journal *ProjectCommandJournal, scope Scope, document projectCommandJournalDocument) {
	t.Helper()
	payload, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload) >= projectCommandJournalMaxBytes {
		t.Fatalf("seed already exceeds journal limit: %d", len(payload))
	}
	path := journal.path(scope)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
}

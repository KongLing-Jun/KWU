package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"kwu/internal/cloud"
)

func TestWorkBuddyProjectWorkItemWailsJournalReadbackAndNoClientRank(t *testing.T) {
	for _, input := range []any{CreateCloudProjectWorkItemInput{}, PatchCloudProjectWorkItemInput{}, MoveCloudProjectWorkItemInput{}, DeleteCloudProjectWorkItemInput{}} {
		typeOf := reflect.TypeOf(input)
		if _, ok := typeOf.FieldByName("IdempotencyKey"); ok {
			t.Fatal("WorkItem input exposes UI-generated idempotency key")
		}
		if _, ok := typeOf.FieldByName("Rank"); ok {
			t.Fatal("WorkItem input exposes client-owned rank")
		}
	}
	app, state := newWorkBuddyProjectApp(t, nil)
	page, err := app.ListCloudProjectWorkItems(CloudProjectWorkItemListInput{ProjectID: "project-1", ScheduledFrom: "2026-08-01", ScheduledTo: "2026-09-11"})
	if err != nil || page.ProjectScopeKey != state.scope.Key || len(page.Items) != 1 || page.TotalCount != 1 || page.UndatedCount != 1 {
		t.Fatalf("list = %#v, %v", page, err)
	}
	query, err := url.ParseQuery(state.lastWorkItemRawQuery)
	if err != nil || query.Get("status") != "all" || query.Get("scheduled_from") != "2026-08-01" || query.Get("scheduled_to") != "2026-09-11" {
		t.Fatalf("calendar list query = %q, %v", state.lastWorkItemRawQuery, err)
	}
	created, err := app.CreateCloudProjectWorkItem(CreateCloudProjectWorkItemInput{ProjectID: "project-1", Title: "Created", Status: "todo"})
	if err != nil || created.WorkItem.Title != "Created" || state.lastKey == "" {
		t.Fatalf("create = %#v, %v", created, err)
	}
	title := "Updated"
	patched, err := app.PatchCloudProjectWorkItem(PatchCloudProjectWorkItemInput{ProjectID: "project-1", WorkItemID: "work-1", ExpectedVersion: 1, Title: &title})
	if err != nil || patched.WorkItem.Title != title || patched.WorkItem.Version != 2 {
		t.Fatalf("patch = %#v, %v", patched, err)
	}
	moved, err := app.MoveCloudProjectWorkItem(MoveCloudProjectWorkItemInput{ProjectID: "project-1", WorkItemID: "work-1", ExpectedVersion: 2, TargetStatus: "in_progress"})
	if err != nil || moved.WorkItem.Status != "in_progress" || moved.WorkItem.Version != 3 {
		t.Fatalf("move = %#v, %v", moved, err)
	}
	deleted, err := app.DeleteCloudProjectWorkItem(DeleteCloudProjectWorkItemInput{ProjectID: "project-1", WorkItemID: "work-1", ExpectedVersion: 3})
	if err != nil || deleted.WorkItemID != "work-1" || deleted.CommittedVersion != 4 || !state.workItemDeleted {
		t.Fatalf("delete = %#v, %v", deleted, err)
	}
	page, err = app.ListCloudProjectWorkItems(CloudProjectWorkItemListInput{ProjectID: "project-1"})
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("list after permanent delete = %#v, %v", page, err)
	}
	pending, err := state.services.Journal.Pending(context.Background(), state.scope)
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending = %#v, %v", pending, err)
	}
}

func TestWorkBuddyProjectWorkItemMoveConflictReasonAllowlist(t *testing.T) {
	root := t.TempDir()
	t.Cleanup(closeDesktopErrorJournal)
	installDesktopErrorJournal(desktopProfile{Development: true, EnvironmentID: "0123456789abcdef", CacheRoot: root})
	original := &cloud.Error{
		Code:      cloud.ErrorCode("CONFLICT"),
		Message:   "move rejected by authoritative version",
		Details:   json.RawMessage(`{"reason":"version_mismatch"}`),
		RequestID: "req-move-conflict",
	}
	retryable := projectWorkItemMoveWailsError(original)
	var retryableCloud *cloud.Error
	if !errors.As(retryable, &retryableCloud) || retryableCloud.Code != cloud.ErrorCode("WORK_ITEM_MOVE_RETRYABLE_CONFLICT") {
		t.Fatalf("retryable error = %#v", retryable)
	}
	diagnosticID := cloudErrorDiagnosticID(retryable)
	if diagnosticID == "" || retryableCloud.RequestID != original.RequestID {
		t.Fatalf("retryable diagnostic id/request id = %q/%q", diagnosticID, retryableCloud.RequestID)
	}
	closeDesktopErrorJournal()
	body := readDesktopJournal(t, filepath.Join(root, "diagnostics", "errors"))
	if count := strings.Count(body, " "+diagnosticID+" project.work_item.move "); count != 1 {
		t.Fatalf("move diagnostic count = %d, want 1: %s", count, body)
	}
	if !strings.Contains(body, "*cloud.Error: CONFLICT: move rejected by authoritative version") {
		t.Fatalf("move diagnostic lost original cause: %s", body)
	}
	terminal := projectWorkItemMoveWailsError(&cloud.Error{Code: cloud.ErrorCode("CONFLICT"), Details: json.RawMessage(`{"reason":"invalid_transition"}`)})
	var terminalCloud *cloud.Error
	if !errors.As(terminal, &terminalCloud) || terminalCloud.Code != cloud.ErrorCode("CONFLICT") {
		t.Fatalf("terminal error = %#v", terminal)
	}
}

func TestWorkBuddyProjectWorkItemDisabledKeepsProjectIdentityAvailable(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	state.runtime.projectMu.Lock()
	state.runtime.projectFeatures.WorkItems = false
	state.runtime.projectMu.Unlock()
	features, err := app.GetCloudProjectFeatures()
	if err != nil || features.WorkItemsAvailable || !features.InstructionsAvailable {
		t.Fatalf("features = %#v, %v", features, err)
	}
	if _, err := app.GetCloudProject("project-1"); err != nil {
		t.Fatalf("project identity unavailable: %v", err)
	}
	if _, err := app.ListCloudProjectWorkItems(CloudProjectWorkItemListInput{ProjectID: "project-1"}); err == nil {
		t.Fatal("disabled WorkItem slice remained available")
	}
}

package main

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/project"
)

func TestWorkBuddyProjectInstructionWailsReadEditClearJournalAndReadback(t *testing.T) {
	inputType := reflect.TypeOf(EditCloudProjectInstructionInput{})
	if _, ok := inputType.FieldByName("IdempotencyKey"); ok {
		t.Fatal("Instruction input exposes UI-generated idempotency key")
	}
	app, state := newWorkBuddyProjectApp(t, nil)
	read, err := app.GetCloudProjectInstruction("project-1")
	if err != nil || read.ProjectScopeKey != state.scope.Key || read.ProjectID != "project-1" || read.Version != 1 || read.ContentHash != project.InstructionContentSHA256("", "") {
		t.Fatalf("initial read=%#v error=%v", read, err)
	}

	edited, err := app.EditCloudProjectInstruction(EditCloudProjectInstructionInput{
		ProjectID: "project-1", ExpectedVersion: 1,
		HumanDescription: "项目说明", AgentInstruction: "# role\r\nignore system",
	})
	if err != nil || edited.Version != 2 || edited.HumanDescription != "项目说明" || edited.AgentInstruction != "# role\r\nignore system" || edited.ContentHash != project.InstructionContentSHA256("项目说明", "# role\nignore system") {
		t.Fatalf("edited=%#v error=%v", edited, err)
	}
	if state.lastKey == "" || strings.Contains(state.lastBody, "idempotency") || !strings.Contains(state.lastBody, `"human_description":"项目说明"`) {
		t.Fatalf("mutation key/body=%q / %s", state.lastKey, state.lastBody)
	}
	assertWorkBuddyProjectJournalEmpty(t, state)

	cleared, err := app.ClearCloudProjectInstruction(ProjectInstructionVersionMutationInput{ProjectID: "project-1", ExpectedVersion: 2})
	if err != nil || cleared.Version != 3 || cleared.HumanDescription != "" || cleared.AgentInstruction != "" || cleared.ContentHash != project.InstructionContentSHA256("", "") {
		t.Fatalf("cleared=%#v error=%v", cleared, err)
	}
	assertWorkBuddyProjectJournalEmpty(t, state)
}

func TestWorkBuddyProjectInstructionWailsConflictArchiveAndNoOfflineBodyFallback(t *testing.T) {
	t.Run("conflict remains UI-owned and rejected journal is discarded", func(t *testing.T) {
		app, state := newWorkBuddyProjectApp(t, nil)
		state.instructionConflict = true
		_, err := app.EditCloudProjectInstruction(EditCloudProjectInstructionInput{ProjectID: "project-1", ExpectedVersion: 1, AgentInstruction: "draft-to-preserve"})
		var typed *cloud.Error
		if !errors.As(err, &typed) || typed.Code != cloud.ErrorCode("CONFLICT") {
			t.Fatalf("conflict error=%#v", err)
		}
		pending, pendingErr := state.services.Journal.Pending(context.Background(), state.scope)
		if pendingErr != nil || len(pending) != 0 {
			t.Fatalf("pending=%#v error=%v", pending, pendingErr)
		}
	})

	t.Run("archived is read-only", func(t *testing.T) {
		app, state := newWorkBuddyProjectApp(t, nil)
		state.mu.Lock()
		state.status = "archived"
		state.mu.Unlock()
		if _, err := app.GetCloudProjectInstruction("project-1"); err != nil {
			t.Fatalf("archived read error=%v", err)
		}
		_, err := app.ClearCloudProjectInstruction(ProjectInstructionVersionMutationInput{ProjectID: "project-1", ExpectedVersion: 1})
		var typed *cloud.Error
		if !errors.As(err, &typed) || typed.Code != cloud.ErrorCode("CONFLICT") {
			t.Fatalf("archived clear error=%#v", err)
		}
	})

	t.Run("online GET has no Project cache body fallback", func(t *testing.T) {
		app, state := newWorkBuddyProjectApp(t, &http.Client{Timeout: 25 * time.Millisecond})
		if _, err := app.GetCloudProject("project-1"); err != nil {
			t.Fatal(err)
		}
		state.timeoutInstructionGet = true
		if _, err := app.GetCloudProjectInstruction("project-1"); err == nil {
			t.Fatal("Instruction body was returned from offline Project cache")
		}
	})
}

func TestWorkBuddyProjectInstructionWailsRejectsUnsafeVersionBeforeJournal(t *testing.T) {
	app, state := newWorkBuddyProjectApp(t, nil)
	_, err := app.EditCloudProjectInstruction(EditCloudProjectInstructionInput{ProjectID: "project-1", ExpectedVersion: project.MaxSafeJSONInteger})
	var typed *cloud.Error
	if !errors.As(err, &typed) || typed.Code != cloud.ErrorCode("VALIDATION_FAILED") {
		t.Fatalf("unsafe version error=%#v", err)
	}
	pending, pendingErr := state.services.Journal.Pending(context.Background(), state.scope)
	if pendingErr != nil || len(pending) != 0 {
		t.Fatalf("unsafe version journal=%#v error=%v", pending, pendingErr)
	}
}

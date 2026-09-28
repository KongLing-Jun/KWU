package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"kwu/internal/agent"
	"kwu/internal/project"
)

func TestWorkBuddyProjectNewSessionReadsOnlyCommitSnapshot(t *testing.T) {
	var reads []string
	client := &http.Client{Transport: rtFunc(func(request *http.Request) (*http.Response, error) {
		if strings.HasPrefix(request.URL.Path, "/api/v1/projects/") {
			reads = append(reads, request.URL.Path)
		}
		return http.DefaultTransport.RoundTrip(request)
	})}
	app, state := newWorkBuddyProjectApp(t, client)
	workspace := t.TempDir()
	if _, err := state.services.Bindings.Bind(context.Background(), state.scope, "project-1", workspace); err != nil {
		t.Fatal(err)
	}
	controller, path := installWorkBuddyProjectSessionTab(t, app, workspace)
	t.Cleanup(controller.Close)
	state.instructionVersion = 0
	previousHook := newCloudProjectSessionBeforeCommit
	newCloudProjectSessionBeforeCommit = func() {
		reads = append(reads, "before-commit")
		state.mu.Lock()
		state.instructionVersion = 2
		state.instructionAgent = "commit snapshot"
		state.mu.Unlock()
	}
	t.Cleanup(func() { newCloudProjectSessionBeforeCommit = previousHook })
	view, err := app.NewCloudProjectSession(NewCloudProjectSessionInput{ProjectID: "project-1", TabID: "project"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/api/v1/projects/project-1", "before-commit", "/api/v1/projects/project-1/instructions", "/api/v1/projects/project-1"}
	if !reflect.DeepEqual(reads, want) {
		t.Fatalf("creation order = %v, want %v", reads, want)
	}
	meta, ok, err := agent.LoadBranchMeta(path)
	if err != nil || !ok || meta.ProjectContext == nil || meta.ProjectContext.InstructionVersion != 2 ||
		meta.ProjectContext.ContentHash != project.InstructionContentSHA256("", "commit snapshot") || view.InstructionVersion != 2 {
		t.Fatalf("commit snapshot was not pinned: view=%#v meta=%#v err=%v", view, meta, err)
	}
	reads = nil
	state.instructionVersion = 3
	state.instructionAgent = "later instruction"
	before := controller.History()
	reused, err := app.NewCloudProjectSession(NewCloudProjectSessionInput{ProjectID: "project-1", TabID: "project"})
	if err != nil || !reused.Reused || reused.InstructionVersion != view.InstructionVersion || reused.ContentHash != view.ContentHash {
		t.Fatalf("immutable reuse = %#v, %v", reused, err)
	}
	want = []string{"/api/v1/projects/project-1", "before-commit", "/api/v1/projects/project-1"}
	if !reflect.DeepEqual(reads, want) || !reflect.DeepEqual(before, controller.History()) {
		t.Fatalf("reuse changed the snapshot or performed extra reads: %v", reads)
	}
}

func TestWorkBuddyProjectNewSessionCommitReadFailuresLeaveNoPin(t *testing.T) {
	for _, scenario := range []string{
		"instruction network", "instruction forbidden", "instruction project ID", "instruction version", "instruction hash",
		"final project network", "final project forbidden", "final project archived",
		"binding changed", "scope changed", "canceled during instruction", "canceled after instruction", "capability unavailable",
	} {
		t.Run(scenario, func(t *testing.T) {
			var state *workBuddyProjectAppState
			var projectReads, instructionReads atomic.Int32
			client := &http.Client{Transport: rtFunc(func(request *http.Request) (*http.Response, error) {
				isInstruction := request.URL.Path == "/api/v1/projects/project-1/instructions"
				isFinalProject := request.URL.Path == "/api/v1/projects/project-1" && projectReads.Add(1) == 2
				if isInstruction {
					instructionReads.Add(1)
				}
				if (isInstruction && scenario == "instruction network") || (isFinalProject && scenario == "final project network") {
					return nil, errors.New("synthetic read failure")
				}
				if isInstruction && scenario == "canceled during instruction" {
					state.services.cancel()
					<-request.Context().Done()
					return nil, request.Context().Err()
				}
				response, err := http.DefaultTransport.RoundTrip(request)
				if err != nil {
					return response, err
				}
				if (isInstruction && scenario == "instruction forbidden") || (isFinalProject && scenario == "final project forbidden") {
					response.Body.Close()
					response.StatusCode = http.StatusForbidden
					response.Body = io.NopCloser(strings.NewReader(`{"data":null,"error":{"code":"FORBIDDEN","message":"Denied","details":null,"retryable":false},"request_id":"synthetic","server_time":"2026-08-05T04:00:03Z"}`))
					response.ContentLength = -1
				}
				if !isInstruction {
					return response, nil
				}
				switch scenario {
				case "instruction project ID", "instruction version", "instruction hash":
					var body map[string]json.RawMessage
					if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					response.Body.Close()
					var data map[string]any
					if err := json.Unmarshal(body["data"], &data); err != nil {
						t.Fatal(err)
					}
					switch scenario {
					case "instruction project ID":
						data["project_id"] = "project-other"
					case "instruction version":
						data["version"] = 0
					case "instruction hash":
						data["content_hash"] = strings.Repeat("0", 64)
					}
					body["data"], err = json.Marshal(data)
					if err != nil {
						t.Fatal(err)
					}
					encoded, err := json.Marshal(body)
					if err != nil {
						t.Fatal(err)
					}
					response.Body = io.NopCloser(strings.NewReader(string(encoded)))
					response.ContentLength = int64(len(encoded))
				case "final project archived":
					state.mu.Lock()
					state.status = "archived"
					state.mu.Unlock()
				case "binding changed":
					if _, err := state.services.Bindings.Bind(context.Background(), state.scope, "project-1", t.TempDir()); err != nil {
						t.Fatal(err)
					}
				case "scope changed":
					state.runtime.replaceProjectServices(project.CapabilityUnavailable, nil)
				case "canceled after instruction":
					state.services.cancel()
				}
				return response, nil
			})}
			app, fixture := newWorkBuddyProjectApp(t, client)
			state = fixture
			workspace := t.TempDir()
			if _, err := state.services.Bindings.Bind(context.Background(), state.scope, "project-1", workspace); err != nil {
				t.Fatal(err)
			}
			controller, path := installWorkBuddyProjectSessionTab(t, app, workspace)
			t.Cleanup(controller.Close)
			state.instructionAgent = "must not be pinned"
			if scenario == "capability unavailable" {
				state.runtime.projectMu.Lock()
				state.runtime.projectFeatures.Instructions = false
				state.runtime.projectMu.Unlock()
			}
			before := controller.History()
			var published atomic.Int32
			app.projectTreeChangedHook = func() { published.Add(1) }
			if _, err := app.NewCloudProjectSession(NewCloudProjectSessionInput{ProjectID: "project-1", TabID: "project"}); err == nil {
				t.Fatal("creation unexpectedly succeeded")
			}
			if !reflect.DeepEqual(before, controller.History()) || published.Load() != 0 {
				t.Fatal("failed creation changed history or published a project session")
			}
			if _, ok, err := agent.LoadBranchMeta(path); err != nil || ok {
				t.Fatalf("failed creation wrote metadata: exists=%v err=%v", ok, err)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("failed creation wrote transcript: %v", err)
			}
			wantInstructionReads := int32(1)
			if scenario == "capability unavailable" {
				wantInstructionReads = 0
			}
			if instructionReads.Load() != wantInstructionReads {
				t.Fatalf("instruction reads=%d, want %d", instructionReads.Load(), wantInstructionReads)
			}
			if strings.HasPrefix(scenario, "final project") && projectReads.Load() != 2 {
				t.Fatalf("final project check was not reached: reads=%d", projectReads.Load())
			}
		})
	}
}

package cloud

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPVideoGenerationClientUsesJWTAndIdempotencyKey(t *testing.T) {
	var gotInput VideoGenerationRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != videoGenerationTasksPath {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer reasonix-jwt" {
			t.Fatalf("Authorization = %q", got)
		}
		if got := r.Header.Get("X-Idempotency-Key"); got != "video-test-001" {
			t.Fatalf("X-Idempotency-Key = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotInput); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = io.WriteString(w, `{
			"data":{
				"task_id":"vgen_01","model":"doubao-seedance-1-0-pro-250528",
				"status":"queued","settlement_status":"pending",
				"reserved_credits":"4500.0000","pricing_version":"pricing-v1",
				"created_at":"2026-07-28T10:30:00Z","updated_at":"2026-07-28T10:30:00Z"
			},
			"error":null,"request_id":"req_01","server_time":"2026-07-28T10:30:00Z"
		}`)
	}))
	defer server.Close()

	scope := BindingScope{ProfileID: "profile-01", CloudUserID: "user-01", BindingGeneration: 1}
	sessionStore := NewMemorySessionStore()
	if err := sessionStore.Save(context.Background(), Session{
		Scope: scope, State: SessionAuthenticated, AccessToken: "reasonix-jwt",
		RefreshToken: "refresh", DeviceID: "device-01",
		AccessTokenExpiresAt: time.Now().Add(time.Hour), RefreshTokenExpiresAt: time.Now().Add(24 * time.Hour),
	}); err != nil {
		t.Fatalf("save session: %v", err)
	}
	transport, err := NewHTTPSessionClient(server.URL, server.Client(), DeviceIdentity{DeviceID: "device-01", ClientVersion: "test"}, nil)
	if err != nil {
		t.Fatalf("NewHTTPSessionClient: %v", err)
	}
	executor, err := NewAuthenticatedRequestExecutor(sessionStore, transport, nil)
	if err != nil {
		t.Fatalf("NewAuthenticatedRequestExecutor: %v", err)
	}
	client, err := NewHTTPVideoGenerationClient(executor, transport)
	if err != nil {
		t.Fatalf("NewHTTPVideoGenerationClient: %v", err)
	}
	cameraFixed, watermark := false, true
	task, err := client.Create(context.Background(), scope, "video-test-001", VideoGenerationRequest{
		Model: "doubao-seedance-1-0-pro-250528", Prompt: "fly",
		ImageURL: "https://example.com/input.png", Resolution: "1080p", Duration: 5,
		CameraFixed: &cameraFixed, Watermark: &watermark,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if task.TaskID != "vgen_01" || gotInput.ImageURL != "https://example.com/input.png" ||
		gotInput.Duration != 5 || gotInput.CameraFixed == nil || *gotInput.CameraFixed {
		t.Fatalf("task/input = %+v / %+v", task, gotInput)
	}
}

func TestHTTPVideoGenerationClientGetsOwnedTask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != videoGenerationTasksPath+"/vgen_01" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer reasonix-jwt" {
			t.Fatalf("Authorization = %q", got)
		}
		_, _ = io.WriteString(w, `{
			"data":{
				"task_id":"vgen_01","model":"doubao-seedance-1-0-pro-250528",
				"status":"succeeded","settlement_status":"settled",
				"reserved_credits":"4500.0000","pricing_version":"pricing-v1",
				"video_url":"https://cdn.example/video.mp4",
				"created_at":"2026-07-28T10:30:00Z","updated_at":"2026-07-28T10:31:20Z"
			},
			"error":null,"request_id":"req_02","server_time":"2026-07-28T10:31:20Z"
		}`)
	}))
	defer server.Close()

	scope := BindingScope{ProfileID: "profile-01", CloudUserID: "user-01", BindingGeneration: 1}
	sessionStore := NewMemorySessionStore()
	if err := sessionStore.Save(context.Background(), Session{
		Scope: scope, State: SessionAuthenticated, AccessToken: "reasonix-jwt",
		RefreshToken: "refresh", DeviceID: "device-01",
		AccessTokenExpiresAt: time.Now().Add(time.Hour), RefreshTokenExpiresAt: time.Now().Add(24 * time.Hour),
	}); err != nil {
		t.Fatalf("save session: %v", err)
	}
	transport, _ := NewHTTPSessionClient(server.URL, server.Client(), DeviceIdentity{DeviceID: "device-01", ClientVersion: "test"}, nil)
	executor, _ := NewAuthenticatedRequestExecutor(sessionStore, transport, nil)
	client, _ := NewHTTPVideoGenerationClient(executor, transport)
	task, err := client.Get(context.Background(), scope, "vgen_01")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if task.VideoURL == nil || *task.VideoURL != "https://cdn.example/video.mp4" {
		t.Fatalf("task = %+v", task)
	}
}

func TestHTTPVideoGenerationClientGetsOwnedTaskProgress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != videoGenerationTasksPath+"/vgen_01/progress" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer reasonix-jwt" {
			t.Fatalf("Authorization = %q", got)
		}
		if got := r.Header.Get("X-Request-Id"); got == "" {
			t.Fatal("X-Request-Id is empty")
		}
		if got := r.Header.Get("X-Idempotency-Key"); got != "" {
			t.Fatalf("X-Idempotency-Key = %q, want omitted", got)
		}
		if r.ContentLength > 0 {
			t.Fatalf("Content-Length = %d, want no request body", r.ContentLength)
		}
		_, _ = io.WriteString(w, `{
			"data":{
				"task_id":"vgen_01","status":"succeeded","settlement_status":"settled",
				"progress_percent":100,"progress_basis":"status_stage",
				"terminal":true,"succeeded":true,"charged_credits":"1074.0000","retry_after_seconds":0,
				"updated_at":"2026-07-29T04:00:00Z"
			},
			"error":null,"request_id":"req_01","server_time":"2026-07-29T04:00:01Z"
		}`)
	}))
	defer server.Close()

	scope := BindingScope{ProfileID: "profile-01", CloudUserID: "user-01", BindingGeneration: 1}
	sessionStore := NewMemorySessionStore()
	if err := sessionStore.Save(context.Background(), Session{
		Scope: scope, State: SessionAuthenticated, AccessToken: "reasonix-jwt",
		RefreshToken: "refresh", DeviceID: "device-01",
		AccessTokenExpiresAt: time.Now().Add(time.Hour), RefreshTokenExpiresAt: time.Now().Add(24 * time.Hour),
	}); err != nil {
		t.Fatalf("save session: %v", err)
	}
	transport, _ := NewHTTPSessionClient(server.URL, server.Client(), DeviceIdentity{DeviceID: "device-01", ClientVersion: "test"}, nil)
	executor, _ := NewAuthenticatedRequestExecutor(sessionStore, transport, nil)
	client, _ := NewHTTPVideoGenerationClient(executor, transport)
	progress, err := client.GetProgress(context.Background(), scope, "vgen_01")
	if err != nil {
		t.Fatalf("GetProgress: %v", err)
	}
	if progress.TaskID != "vgen_01" || progress.Status != "succeeded" ||
		progress.ProgressPercent != 100 || progress.ProgressBasis != "status_stage" ||
		!progress.Terminal || !progress.Succeeded || progress.ChargedCredits == nil ||
		*progress.ChargedCredits != "1074.0000" || progress.RetryAfterSeconds != 0 {
		t.Fatalf("progress = %+v", progress)
	}
}

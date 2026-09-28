package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPMediaUnderstandingClientSendsURLWithoutModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != mediaUnderstandingsPath {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer reasonix-jwt" {
			t.Fatalf("Authorization = %q", got)
		}
		if got := r.Header.Get("X-Idempotency-Key"); got != "media-test-001" {
			t.Fatalf("X-Idempotency-Key = %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if _, exists := body["model"]; exists {
			t.Fatal("URL request unexpectedly exposed model")
		}
		if body["media_type"] != "audio" || body["media_url"] != "https://cdn.example.com/meeting.mp3" {
			t.Fatalf("request body = %#v", body)
		}
		writeMediaUnderstandingSuccess(w, "audio")
	}))
	defer server.Close()

	client := newTestHTTPMediaUnderstandingClient(t, server)
	result, err := client.UnderstandURL(context.Background(), testMediaUnderstandingScope(), "media-test-001", MediaUnderstandingRequest{
		MediaType: "audio", MediaURL: "https://cdn.example.com/meeting.mp3", Prompt: "transcribe",
	})
	if err != nil {
		t.Fatalf("UnderstandURL: %v", err)
	}
	if result.Model != MediaUnderstandingModel || result.Text != "understood text" {
		t.Fatalf("result = %+v", result)
	}
}

func TestHTTPMediaUnderstandingClientSendsMultipartFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Idempotency-Key"); got != "media-test-002" {
			t.Fatalf("X-Idempotency-Key = %q", got)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		if r.FormValue("media_type") != "video" || r.FormValue("prompt") != "describe" || r.FormValue("model") != "" {
			t.Fatalf("multipart values = %#v", r.MultipartForm.Value)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("FormFile: %v", err)
		}
		defer file.Close()
		content, _ := io.ReadAll(file)
		if header.Filename != "demo.mp4" || string(content) != "video-bytes" {
			t.Fatalf("file = %q %q", header.Filename, content)
		}
		writeMediaUnderstandingSuccess(w, "video")
	}))
	defer server.Close()

	client := newTestHTTPMediaUnderstandingClient(t, server)
	result, err := client.UnderstandFile(
		context.Background(), testMediaUnderstandingScope(), "media-test-002",
		"video", "describe", `D:\videos\demo.mp4`, []byte("video-bytes"),
	)
	if err != nil {
		t.Fatalf("UnderstandFile: %v", err)
	}
	if result.MediaType != "video" {
		t.Fatalf("result = %+v", result)
	}
}

func TestHTTPMediaUnderstandingClientPreservesSettlementPending(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, `{
			"data":null,
			"error":{"code":"SETTLEMENT_PENDING","message":"pending safe settlement","retryable":false},
			"request_id":"req_pending","server_time":"2026-07-30T12:00:02Z"
		}`)
	}))
	defer server.Close()

	client := newTestHTTPMediaUnderstandingClient(t, server)
	_, err := client.UnderstandURL(context.Background(), testMediaUnderstandingScope(), "media-pending", MediaUnderstandingRequest{
		MediaType: "image", MediaURL: "https://cdn.example.com/image.png",
	})
	var cloudErr *Error
	if !errors.As(err, &cloudErr) || cloudErr.Code != ErrorSettlementPending || cloudErr.Retryable {
		t.Fatalf("error = %#v, want non-retryable settlement pending", err)
	}
}

func newTestHTTPMediaUnderstandingClient(t *testing.T, server *httptest.Server) MediaUnderstandingClient {
	t.Helper()
	scope := testMediaUnderstandingScope()
	store := NewMemorySessionStore()
	if err := store.Save(context.Background(), Session{
		Scope: scope, State: SessionAuthenticated, AccessToken: "reasonix-jwt",
		RefreshToken: "refresh", DeviceID: "device-01",
		AccessTokenExpiresAt: time.Now().Add(time.Hour), RefreshTokenExpiresAt: time.Now().Add(24 * time.Hour),
	}); err != nil {
		t.Fatalf("Save session: %v", err)
	}
	transport, err := NewHTTPSessionClient(server.URL, server.Client(), DeviceIdentity{DeviceID: "device-01", ClientVersion: "test"}, nil)
	if err != nil {
		t.Fatalf("NewHTTPSessionClient: %v", err)
	}
	executor, err := NewAuthenticatedRequestExecutor(store, transport, nil)
	if err != nil {
		t.Fatalf("NewAuthenticatedRequestExecutor: %v", err)
	}
	client, err := NewHTTPMediaUnderstandingClient(executor, transport)
	if err != nil {
		t.Fatalf("NewHTTPMediaUnderstandingClient: %v", err)
	}
	return client
}

func testMediaUnderstandingScope() BindingScope {
	return BindingScope{ProfileID: "profile-01", CloudUserID: "user-01", BindingGeneration: 1}
}

func writeMediaUnderstandingSuccess(w http.ResponseWriter, mediaType string) {
	payload := strings.ReplaceAll(`{
		"data":{
			"task_id":"munder_01","model":"doubao-seed-2-0-lite-260428",
			"media_type":"MEDIA_TYPE","text":"understood text","status":"succeeded",
			"settlement_status":"settled",
			"usage":{"input_tokens":1000,"audio_input_tokens":800,"output_tokens":200,"total_tokens":1200},
			"pricing_version":"pricing-v1","cost_cny":"0.00804000","charged_credits":"16.0800",
			"created_at":"2026-07-30T12:00:00Z","completed_at":"2026-07-30T12:00:01Z"
		},
		"error":null,"request_id":"req_01","server_time":"2026-07-30T12:00:01Z"
	}`, "MEDIA_TYPE", mediaType)
	_, _ = io.WriteString(w, payload)
}

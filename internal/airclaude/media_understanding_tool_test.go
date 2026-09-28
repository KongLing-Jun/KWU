package airclaude

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/tool"
)

func TestMediaUnderstandingToolUnderstandsWorkspaceFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "meeting.mp3")
	if err := os.WriteFile(path, []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	client := &fakeMediaUnderstandingClient{results: []cloud.MediaUnderstandingResult{successfulUnderstanding("audio")}}
	media := newTestMediaUnderstandingTool(t, root, client)

	output, err := media.Execute(context.Background(), json.RawMessage(`{
		"media_type":"audio","file_path":"meeting.mp3","prompt":"请转写"
	}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if client.fileCalls != 1 || client.filename != "meeting.mp3" || string(client.content) != "audio" ||
		client.mediaType != "audio" || client.prompt != "请转写" || !strings.HasPrefix(client.keys[0], "media-") {
		t.Fatalf("client call = %+v", client)
	}
	if !strings.Contains(output, `"model":"`+cloud.MediaUnderstandingModel+`"`) ||
		!strings.Contains(output, `"text":"understood text"`) {
		t.Fatalf("output = %s", output)
	}
}

func TestMediaUnderstandingToolRecordsSettledCredits(t *testing.T) {
	client := &fakeMediaUnderstandingClient{results: []cloud.MediaUnderstandingResult{successfulUnderstanding("image")}}
	media := newTestMediaUnderstandingTool(t, t.TempDir(), client)
	ctx, recorder := tool.WithCreditRecorder(context.Background())
	if _, err := media.Execute(ctx, json.RawMessage(`{"media_type":"image","media_url":"https://cdn.example/image.png"}`)); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	quota, credits := recorder.Summary()
	if quota != "" || credits != "0.264" {
		t.Fatalf("tool credits = quota %q, credits %q", quota, credits)
	}
}

func TestMediaUnderstandingToolReusesKeyWhileSettlementPending(t *testing.T) {
	client := &fakeMediaUnderstandingClient{
		results: []cloud.MediaUnderstandingResult{{}, successfulUnderstanding("image")},
		errs: []error{
			&cloud.Error{Code: cloud.ErrorSettlementPending, Message: "pending", Retryable: false},
			nil,
		},
	}
	media := newTestMediaUnderstandingTool(t, t.TempDir(), client)
	call := json.RawMessage(`{"media_type":"image","media_url":"https://cdn.example.com/shot.png","prompt":"OCR"}`)

	if _, err := media.Execute(context.Background(), call); err == nil ||
		!strings.Contains(err.Error(), "retry the same tool call") {
		t.Fatalf("first error = %v", err)
	}
	if _, err := media.Execute(context.Background(), call); err != nil {
		t.Fatalf("retry Execute: %v", err)
	}
	if len(client.keys) != 2 || client.keys[0] == "" || client.keys[0] != client.keys[1] {
		t.Fatalf("idempotency keys = %v, want same non-empty key", client.keys)
	}
}

func TestMediaUnderstandingToolValidatesSourcesAndFormats(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.mp4")
	if err := os.WriteFile(outside, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	badFormat := filepath.Join(root, "clip.webm")
	if err := os.WriteFile(badFormat, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	oversizedImage := filepath.Join(root, "large.png")
	if err := os.WriteFile(oversizedImage, []byte{0}, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(oversizedImage, maxUnderstandingImageBytes+1); err != nil {
		t.Fatal(err)
	}
	media := newTestMediaUnderstandingTool(t, root, &fakeMediaUnderstandingClient{})

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"missing source", `{"media_type":"image"}`, "exactly one"},
		{"both sources", `{"media_type":"image","file_path":"x.png","media_url":"https://example.com/x.png"}`, "exactly one"},
		{"private URL", `{"media_type":"video","media_url":"https://127.0.0.1/x.mp4"}`, "private address"},
		{"credential URL", `{"media_type":"audio","media_url":"https://user:pass@example.com/x.mp3"}`, "without credentials"},
		{"outside workspace", `{"media_type":"video","file_path":` + mustJSONString(t, outside) + `}`, "inside the current workspace"},
		{"unsupported format", `{"media_type":"video","file_path":"clip.webm"}`, "not supported"},
		{"oversized image", `{"media_type":"image","file_path":"large.png"}`, "10 MiB"},
		{"oversized prompt", `{"media_type":"image","media_url":"https://example.com/x.png","prompt":"` + strings.Repeat("x", maxUnderstandingPromptBytes+1) + `"}`, "8000 UTF-8 bytes"},
		{"model field", `{"media_type":"image","media_url":"https://example.com/x.png","model":"other"}`, "unknown field"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := media.Execute(context.Background(), json.RawMessage(tt.raw)); err == nil ||
				!strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestMediaUnderstandingToolSchemaIsProviderCompatibleAndIsPaid(t *testing.T) {
	media := newTestMediaUnderstandingTool(t, t.TempDir(), &fakeMediaUnderstandingClient{})
	if media.Name() != "understand_media" || media.ReadOnly() {
		t.Fatalf("metadata = %q readOnly=%t", media.Name(), media.ReadOnly())
	}
	var schema struct {
		Type       string                     `json:"type"`
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(media.Schema(), &schema); err != nil {
		t.Fatalf("schema: %v", err)
	}
	if schema.Type != "object" {
		t.Fatalf("schema type = %q, want object", schema.Type)
	}
	if _, exists := schema.Properties["model"]; exists {
		t.Fatal("schema unexpectedly exposes model")
	}
	for _, field := range []string{"media_type", "file_path", "media_url", "prompt"} {
		if _, exists := schema.Properties[field]; !exists {
			t.Fatalf("schema missing %q", field)
		}
	}
	var topLevel map[string]json.RawMessage
	if err := json.Unmarshal(media.Schema(), &topLevel); err != nil {
		t.Fatalf("schema top level: %v", err)
	}
	for _, keyword := range []string{"oneOf", "anyOf", "allOf", "enum", "const", "not"} {
		if _, exists := topLevel[keyword]; exists {
			t.Errorf("schema contains unsupported top-level keyword %q", keyword)
		}
	}
}

func TestMediaUnderstandingToolTimeoutIsAtLeastTwoMinutes(t *testing.T) {
	media := newTestMediaUnderstandingTool(t, t.TempDir(), &fakeMediaUnderstandingClient{})
	for _, tt := range []struct {
		name string
		base time.Duration
		want time.Duration
	}{
		{name: "short configured timeout", base: 30 * time.Second, want: 120 * time.Second},
		{name: "long configured timeout", base: 3 * time.Minute, want: 3 * time.Minute},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := media.ToolTimeoutBudget(tt.base, nil); got != tt.want {
				t.Fatalf("ToolTimeoutBudget(%s) = %s, want %s", tt.base, got, tt.want)
			}
		})
	}
}

func TestMediaUnderstandingToolRejectsUnsettledSuccess(t *testing.T) {
	result := successfulUnderstanding("video")
	result.SettlementStatus = "pending"
	media := newTestMediaUnderstandingTool(t, t.TempDir(), &fakeMediaUnderstandingClient{
		results: []cloud.MediaUnderstandingResult{result},
	})
	_, err := media.Execute(context.Background(), json.RawMessage(`{
		"media_type":"video","media_url":"https://cdn.example.com/clip.mp4"
	}`))
	if err == nil || !strings.Contains(err.Error(), "unsettled") {
		t.Fatalf("error = %v", err)
	}
}

type fakeMediaUnderstandingClient struct {
	keys      []string
	results   []cloud.MediaUnderstandingResult
	errs      []error
	urlCalls  int
	fileCalls int
	mediaType string
	prompt    string
	filename  string
	content   []byte
}

func (f *fakeMediaUnderstandingClient) UnderstandURL(
	_ context.Context,
	_ cloud.BindingScope,
	key string,
	input cloud.MediaUnderstandingRequest,
) (cloud.MediaUnderstandingResult, error) {
	f.keys = append(f.keys, key)
	f.urlCalls++
	f.mediaType, f.prompt = input.MediaType, input.Prompt
	return f.response()
}

func (f *fakeMediaUnderstandingClient) UnderstandFile(
	_ context.Context,
	_ cloud.BindingScope,
	key string,
	mediaType string,
	prompt string,
	filename string,
	content []byte,
) (cloud.MediaUnderstandingResult, error) {
	f.keys = append(f.keys, key)
	f.fileCalls++
	f.mediaType, f.prompt, f.filename = mediaType, prompt, filename
	f.content = append([]byte(nil), content...)
	return f.response()
}

func (f *fakeMediaUnderstandingClient) response() (cloud.MediaUnderstandingResult, error) {
	index := len(f.keys) - 1
	var result cloud.MediaUnderstandingResult
	if len(f.results) > 0 {
		result = f.results[min(index, len(f.results)-1)]
	}
	var err error
	if len(f.errs) > 0 {
		err = f.errs[min(index, len(f.errs)-1)]
	}
	return result, err
}

func newTestMediaUnderstandingTool(t *testing.T, root string, client cloud.MediaUnderstandingClient) *MediaUnderstandingTool {
	t.Helper()
	media, err := NewMediaUnderstandingTool(client, func(context.Context) (cloud.BindingScope, error) {
		return cloud.BindingScope{ProfileID: "profile-01", CloudUserID: "user-01", BindingGeneration: 1}, nil
	}, root)
	if err != nil {
		t.Fatalf("NewMediaUnderstandingTool: %v", err)
	}
	return media
}

func successfulUnderstanding(mediaType string) cloud.MediaUnderstandingResult {
	return cloud.MediaUnderstandingResult{
		TaskID: "munder_01", Model: cloud.MediaUnderstandingModel, MediaType: mediaType,
		Text: "understood text", Status: "succeeded", SettlementStatus: "settled",
		ChargedCredits: "0.2640",
		CreatedAt:      time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC),
		CompletedAt:    time.Date(2026, 7, 30, 12, 0, 1, 0, time.UTC),
	}
}

var _ tool.TimeoutBudgeter = (*MediaUnderstandingTool)(nil)

func mustJSONString(t *testing.T, value string) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

var _ cloud.MediaUnderstandingClient = (*fakeMediaUnderstandingClient)(nil)

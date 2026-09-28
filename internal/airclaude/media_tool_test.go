package airclaude

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/tool"
)

func TestMediaToolGenerateImageUsesOpenAIEndpointAndMaaSCredential(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/images/generations" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer maas-secret" {
			t.Fatalf("Authorization = %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = io.WriteString(w, ` { "data": [ { "url": "https://cdn.example/image.png" } ] } `)
	}))
	defer server.Close()

	media := newTestImageTool(t, server.URL+"/v1", func(context.Context) (string, error) {
		return "maas-secret", nil
	})
	result, err := media.Execute(context.Background(), json.RawMessage(`{"prompt":"a red panda reading"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var generated map[string]any
	if err := json.Unmarshal([]byte(result), &generated); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	resultMedia, _ := generated["media"].([]any)
	if generated["action"] != "generate_image" || len(resultMedia) != 1 {
		t.Fatalf("result = %s", result)
	}
	firstMedia, _ := resultMedia[0].(map[string]any)
	if firstMedia["type"] != "image" || firstMedia["url"] != "https://cdn.example/image.png" {
		t.Fatalf("media = %#v", firstMedia)
	}
	want := map[string]any{
		"model":  defaultImageModel,
		"prompt": "a red panda reading",
	}
	if encoded, _ := json.Marshal(requestBody); string(encoded) != mustJSON(t, want) {
		t.Fatalf("body = %s, want %s", encoded, mustJSON(t, want))
	}
}

func TestMediaToolRecordsImageMaaSQuotaForTurnSettlement(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"billing":{"quota":35800},"data":[{"url":"https://cdn.example/image.png"}]}`)
	}))
	defer server.Close()
	media := newTestImageTool(t, server.URL, staticCredential("key"))
	ctx, recorder := tool.WithCreditRecorder(context.Background())
	if _, err := media.Execute(ctx, json.RawMessage(`{"prompt":"sunset"}`)); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	quota, credits := recorder.Summary()
	if quota != "35800" || credits != "" {
		t.Fatalf("tool credits = quota %q, credits %q", quota, credits)
	}
}

func TestMediaToolRecordsEstimatedImageQuotaFromActualUsageShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{
			"created":1788180174,
			"usage":{
				"input_tokens":43,"input_tokens_details":{"image_tokens":0,"text_tokens":43},
				"output_tokens":186,"output_tokens_details":{"image_tokens":186,"text_tokens":0},
				"total_tokens":229
			},
			"data":[{"url":"https://cdn.example/image.png"}]
		}`)
	}))
	defer server.Close()
	media, err := NewMediaTool(MediaToolOptions{
		BaseURL: server.URL, Credential: staticCredential("key"),
		ImageQuotaPricing: func(context.Context, string) (ImageQuotaPricing, error) {
			return ImageQuotaPricing{
				InputPerMillion: "5", OutputPerMillion: "30", ImageInputPerMillion: "8", QuotaPerUSD: "500000",
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, recorder := tool.WithCreditRecorder(context.Background())
	if _, err := (&ImageTool{media: media}).Execute(ctx, json.RawMessage(`{"prompt":"a fish"}`)); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	quota, credits := recorder.Summary()
	if quota != "2898" || credits != "" {
		t.Fatalf("tool credits = quota %q, credits %q", quota, credits)
	}
}

func TestMediaToolAllowsImageResponseWithoutBillingQuota(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"data":[{"url":"https://cdn.example/image.png"}]}`)
	}))
	defer server.Close()
	media := newTestImageTool(t, server.URL, staticCredential("key"))
	ctx, recorder := tool.WithCreditRecorder(context.Background())
	result, err := media.Execute(ctx, json.RawMessage(`{"prompt":"sunset"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result, "https://cdn.example/image.png") {
		t.Fatalf("result = %s", result)
	}
	quota, credits := recorder.Summary()
	if quota != "" || credits != "" {
		t.Fatalf("tool credits = quota %q, credits %q", quota, credits)
	}
}

func TestMediaToolSavesInlineImageAndReturnsCompactManifest(t *testing.T) {
	rawImage := []byte("\x89PNG\r\n\x1a\nsmall-image")
	encodedImage := base64.StdEncoding.EncodeToString(rawImage)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"created":42,"data":[{"b64_json":"`+encodedImage+`"}]}`)
	}))
	defer server.Close()

	outputDir := t.TempDir()
	engine, err := NewMediaTool(MediaToolOptions{
		BaseURL: server.URL + "/v1", Credential: staticCredential("key"), OutputDir: outputDir,
	})
	if err != nil {
		t.Fatalf("NewMediaTool: %v", err)
	}
	media := &ImageTool{media: engine}
	result, err := media.Execute(context.Background(), json.RawMessage(`{"prompt":"small"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if strings.Contains(result, encodedImage) || len(result) > 2_000 {
		t.Fatalf("result retained inline image payload: len=%d", len(result))
	}
	var manifest struct {
		Media []mediaArtifact `json:"media"`
	}
	if err := json.Unmarshal([]byte(result), &manifest); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if len(manifest.Media) != 1 || manifest.Media[0].Type != "image" {
		t.Fatalf("manifest = %+v", manifest)
	}
	if !strings.Contains(result, "At the end of the final response") {
		t.Fatalf("result does not instruct final path placement: %s", result)
	}
	if filepath.Dir(manifest.Media[0].Path) != outputDir {
		t.Fatalf("path = %q, want under %q", manifest.Media[0].Path, outputDir)
	}
	saved, err := os.ReadFile(manifest.Media[0].Path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(saved) != string(rawImage) {
		t.Fatalf("saved image mismatch: %q", saved)
	}
}

func TestMediaToolGenerateVideoUsesReasonixJWTClient(t *testing.T) {
	videoURL := "https://cdn.example/generated.mp4"
	chargedCredits := "1074.0000"
	client := &fakeVideoGenerationClient{createTask: cloud.VideoGenerationTask{
		TaskID: "vgen_123", Model: defaultVideoModel, Status: "queued",
	}, progresses: []cloud.VideoGenerationProgress{
		{TaskID: "vgen_123", Status: "running", ProgressPercent: 70, ProgressBasis: "status_stage"},
		{TaskID: "vgen_123", Status: "succeeded", SettlementStatus: "settled", ProgressPercent: 100, ProgressBasis: "status_stage", Terminal: true, Succeeded: true},
	}, getTask: cloud.VideoGenerationTask{
		TaskID: "vgen_123", Model: defaultVideoModel, Status: "succeeded", SettlementStatus: "settled",
		ChargedCredits: &chargedCredits, VideoURL: &videoURL,
	}}
	var credentialCalls atomic.Int64
	media := newTestVideoMediaTool(t, client, func(context.Context) (string, error) {
		credentialCalls.Add(1)
		return "must-not-be-used-for-video", nil
	})
	var progressChunks []string
	ctx := tool.WithProgress(context.Background(), func(chunk string) {
		progressChunks = append(progressChunks, chunk)
	})
	ctx, recorder := tool.WithCreditRecorder(ctx)
	result, err := media.Execute(ctx, json.RawMessage(`{"prompt":"ocean at sunrise"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(result, `"task_id":"vgen_123"`) ||
		!strings.Contains(result, `"status":"succeeded"`) ||
		!strings.Contains(result, `"url":"https://cdn.example/generated.mp4"`) {
		t.Fatalf("result = %s", result)
	}
	if client.progressCalls != 2 || client.progressTaskID != "vgen_123" {
		t.Fatalf("video progress queries = %d for %q, want 2 for cloud task ID", client.progressCalls, client.progressTaskID)
	}
	if client.getCalls != 1 {
		t.Fatalf("video task queries = %d, want final result query only", client.getCalls)
	}
	if len(progressChunks) != 2 ||
		!strings.Contains(progressChunks[0], `"progress_percent":70`) ||
		!strings.Contains(progressChunks[1], `"progress_percent":100`) {
		t.Fatalf("progress chunks = %#v", progressChunks)
	}
	if credentialCalls.Load() != 0 {
		t.Fatalf("MaaS credential calls = %d, want 0", credentialCalls.Load())
	}
	if client.createKey == "" || !strings.HasPrefix(client.createKey, "video-") {
		t.Fatalf("idempotency key = %q", client.createKey)
	}
	request := client.createInput
	if request.Model != defaultVideoModel || request.Prompt != "ocean at sunrise" ||
		request.ImageURL != "" || request.Resolution != "" ||
		request.Duration != defaultVideoDuration || request.CameraFixed != nil || request.Watermark != nil {
		t.Fatalf("request = %+v", request)
	}
	if client.createScope != testVideoScope() {
		t.Fatalf("scope = %+v", client.createScope)
	}
	quota, credits := recorder.Summary()
	if quota != "" || credits != "1074" {
		t.Fatalf("tool credits = quota %q, credits %q", quota, credits)
	}
}

func TestMediaToolUsesTerminalProgressChargeWhenFullVideoResultOmitsIt(t *testing.T) {
	videoURL := "https://cdn.example/generated.mp4"
	chargedCredits := "1074.0000"
	client := &fakeVideoGenerationClient{
		createTask: cloud.VideoGenerationTask{TaskID: "vgen_progress_charge", Status: "queued"},
		progress: cloud.VideoGenerationProgress{
			TaskID: "vgen_progress_charge", Status: "succeeded", SettlementStatus: "settled",
			ProgressPercent: 100, Terminal: true, Succeeded: true, ChargedCredits: &chargedCredits,
		},
		getTask: cloud.VideoGenerationTask{
			TaskID: "vgen_progress_charge", Status: "succeeded", SettlementStatus: "settled", VideoURL: &videoURL,
		},
	}
	media := newTestVideoMediaTool(t, client, nil)
	ctx, recorder := tool.WithCreditRecorder(context.Background())
	if _, err := media.Execute(ctx, json.RawMessage(`{"prompt":"ocean at sunrise"}`)); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	quota, credits := recorder.Summary()
	if quota != "" || credits != "1074" {
		t.Fatalf("tool credits = quota %q, credits %q", quota, credits)
	}
}

func TestMediaToolWaitsWhenVideoURLArrivesBeforeSettlement(t *testing.T) {
	videoURL := "https://cdn.example/generated.mp4"
	chargedCredits := "1074.0000"
	client := &fakeVideoGenerationClient{
		createTask: cloud.VideoGenerationTask{
			TaskID: "vgen_late_settlement", Status: "succeeded", SettlementStatus: "pending", VideoURL: &videoURL,
		},
		progress: cloud.VideoGenerationProgress{
			TaskID: "vgen_late_settlement", Status: "succeeded", SettlementStatus: "settled",
			ProgressPercent: 100, Terminal: true, Succeeded: true, ChargedCredits: &chargedCredits,
		},
		getTask: cloud.VideoGenerationTask{
			TaskID: "vgen_late_settlement", Status: "succeeded", VideoURL: &videoURL,
		},
	}
	media := newTestVideoMediaTool(t, client, nil)
	ctx, recorder := tool.WithCreditRecorder(context.Background())
	if _, err := media.Execute(ctx, json.RawMessage(`{"prompt":"ocean at sunrise"}`)); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	quota, credits := recorder.Summary()
	if quota != "" || credits != "1074" {
		t.Fatalf("tool credits = quota %q, credits %q", quota, credits)
	}
}

func TestMediaToolGenerateVideoOmitsResolutionWhenNotProvided(t *testing.T) {
	videoURL := "https://cdn.example/generated.mp4"
	chargedCredits := "1.0000"
	client := &fakeVideoGenerationClient{createTask: cloud.VideoGenerationTask{
		TaskID: "vgen_123", Model: defaultVideoModel, Status: "succeeded", SettlementStatus: "settled",
		ChargedCredits: &chargedCredits, VideoURL: &videoURL,
	}}
	media := newTestVideoMediaTool(t, client, nil)

	if _, err := media.Execute(
		context.Background(),
		json.RawMessage(`{"prompt":"ocean at sunrise"}`),
	); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := client.createInput.Duration; got != defaultVideoDuration {
		t.Fatalf("request duration = %d, want %d", got, defaultVideoDuration)
	}
	if got := client.createInput.Resolution; got != "" {
		t.Fatalf("request resolution = %q, want omitted", got)
	}
	encoded, err := json.Marshal(client.createInput)
	if err != nil {
		t.Fatalf("encode request: %v", err)
	}
	if strings.Contains(string(encoded), `"resolution"`) {
		t.Fatalf("encoded request unexpectedly contains resolution: %s", encoded)
	}
}

func TestMediaToolVideoWaitStopsWhenContextIsCancelled(t *testing.T) {
	client := &fakeVideoGenerationClient{createTask: cloud.VideoGenerationTask{
		TaskID: "vgen_123", Status: "queued",
	}}
	media := newTestVideoMediaTool(t, client, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := media.Execute(ctx, json.RawMessage(`{"prompt":"ocean"}`))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Execute error = %v, want context cancellation", err)
	}
	if client.getCalls != 0 {
		t.Fatalf("video task queries = %d, want 0", client.getCalls)
	}
}

func TestMediaToolVideoFailureReturnsProviderCode(t *testing.T) {
	providerCode := "content_policy_violation"
	client := &fakeVideoGenerationClient{createTask: cloud.VideoGenerationTask{
		TaskID: "vgen_123", Status: "failed", ProviderErrorCode: &providerCode,
	}}
	media := newTestVideoMediaTool(t, client, nil)

	_, err := media.Execute(context.Background(), json.RawMessage(`{"prompt":"ocean"}`))
	if err == nil || !strings.Contains(err.Error(), "failed: content_policy_violation") {
		t.Fatalf("Execute error = %v", err)
	}
}

func TestMediaToolProgressFailureIsTerminal(t *testing.T) {
	client := &fakeVideoGenerationClient{
		createTask: cloud.VideoGenerationTask{TaskID: "vgen_failed", Status: "queued"},
		progress: cloud.VideoGenerationProgress{
			TaskID: "vgen_failed", Status: "failed", ProgressPercent: 100, Terminal: true,
		},
	}
	media := newTestVideoMediaTool(t, client, nil)

	_, err := media.Execute(context.Background(), json.RawMessage(`{"prompt":"ocean"}`))
	if err == nil || !strings.Contains(err.Error(), "video generation failed") {
		t.Fatalf("Execute error = %v, want terminal progress failure", err)
	}
	if client.progressCalls != 1 || client.getCalls != 0 {
		t.Fatalf("progress/task queries = %d/%d, want 1/0", client.progressCalls, client.getCalls)
	}
}

func TestVideoProgressPollIntervalUsesServerRetryHint(t *testing.T) {
	if got := videoProgressPollInterval(time.Second, 5); got != 5*time.Second {
		t.Fatalf("poll interval = %s, want 5s", got)
	}
	if got := videoProgressPollInterval(time.Second, 0); got != time.Second {
		t.Fatalf("fallback poll interval = %s, want 1s", got)
	}
	if got := videoProgressPollInterval(time.Second, 3600); got != time.Minute {
		t.Fatalf("capped poll interval = %s, want 1m", got)
	}
}

func TestMediaToolResolvesCredentialAtEveryCall(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = io.WriteString(w, `{"data":[]}`)
	}))
	defer server.Close()

	var available atomic.Bool
	available.Store(true)
	media := newTestImageTool(t, server.URL+"/v1", func(context.Context) (string, error) {
		if !available.Load() {
			return "", errors.New("account signed out")
		}
		return "current-key", nil
	})
	args := json.RawMessage(`{"prompt":"first"}`)
	if _, err := media.Execute(context.Background(), args); err != nil {
		t.Fatalf("first Execute: %v", err)
	}
	available.Store(false)
	if _, err := media.Execute(context.Background(), args); err == nil || !strings.Contains(err.Error(), "account signed out") {
		t.Fatalf("second Execute error = %v", err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("HTTP requests = %d, want 1", got)
	}
}

func TestMediaToolRejectsInvalidArgumentsBeforeResolvingCredential(t *testing.T) {
	var credentialCalls atomic.Int64
	media := newTestImageTool(t, "https://example.invalid/v1", func(context.Context) (string, error) {
		credentialCalls.Add(1)
		return "unused", nil
	})
	cases := []json.RawMessage{
		json.RawMessage(`{}`),
		json.RawMessage(`{"prompt":""}`),
		json.RawMessage(`{"prompt":"x","size":"1024x1024"}`),
		json.RawMessage(`{"prompt":"x","quality":"high"}`),
		json.RawMessage(`{"prompt":"x","model":"custom"}`),
		json.RawMessage(`{"prompt":"x","unknown":true}`),
	}
	for _, args := range cases {
		if _, err := media.Execute(context.Background(), args); err == nil {
			t.Fatalf("Execute(%s) unexpectedly succeeded", args)
		}
	}
	if got := credentialCalls.Load(); got != 0 {
		t.Fatalf("credential calls = %d, want 0", got)
	}
}

func TestMediaToolSurfacesHTTPErrorWithoutCredential(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"quota exhausted"}}`, http.StatusTooManyRequests)
	}))
	defer server.Close()

	media := newTestImageTool(t, server.URL+"/v1", staticCredential("never-echo-this-key"))
	_, err := media.Execute(context.Background(), json.RawMessage(`{"prompt":"x"}`))
	if err == nil || !strings.Contains(err.Error(), "HTTP 429") || !strings.Contains(err.Error(), "quota exhausted") {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(err.Error(), "never-echo-this-key") {
		t.Fatalf("error leaked credential: %v", err)
	}
}

func TestMediaToolSchemaAndClassification(t *testing.T) {
	image := newTestImageTool(t, "https://example.invalid/v1", staticCredential("key"))
	video := &VideoTool{media: image.media}
	for _, generationTool := range []tool.Tool{image, video} {
		if generationTool.ReadOnly() {
			t.Fatalf("tool %q unexpectedly read-only", generationTool.Name())
		}
		if !json.Valid(generationTool.Schema()) {
			t.Fatalf("schema for %q is invalid JSON: %s", generationTool.Name(), generationTool.Schema())
		}
		var schema struct {
			Properties           map[string]json.RawMessage `json:"properties"`
			Required             []string                   `json:"required"`
			AdditionalProperties bool                       `json:"additionalProperties"`
		}
		if err := json.Unmarshal(generationTool.Schema(), &schema); err != nil {
			t.Fatalf("decode schema: %v", err)
		}
		if len(schema.Properties) != 1 || schema.Properties["prompt"] == nil ||
			len(schema.Required) != 1 || schema.Required[0] != "prompt" || schema.AdditionalProperties {
			t.Fatalf("tool %q schema exposes more than required prompt: %s", generationTool.Name(), generationTool.Schema())
		}
	}
}

func TestMediaToolVideoCallsHaveAtLeastFiveMinuteTimeout(t *testing.T) {
	media := &VideoTool{media: newTestImageTool(t, "https://example.invalid/v1", staticCredential("key")).media}

	for _, tt := range []struct {
		name string
		base time.Duration
		want time.Duration
	}{
		{name: "short configured timeout", base: 2 * time.Minute, want: 5 * time.Minute},
		{name: "long configured timeout", base: 10 * time.Minute, want: 10 * time.Minute},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := media.ToolTimeoutBudget(tt.base, nil); got != tt.want {
				t.Fatalf("ToolTimeoutBudget(%s) = %s, want %s", tt.base, got, tt.want)
			}
		})
	}
}

func TestMediaToolImageCallsHaveAtLeastFiveMinuteTimeout(t *testing.T) {
	media := newTestImageTool(t, "https://example.invalid/v1", staticCredential("key"))

	for _, tt := range []struct {
		name string
		base time.Duration
		want time.Duration
	}{
		{name: "short configured timeout", base: 2 * time.Minute, want: 5 * time.Minute},
		{name: "long configured timeout", base: 10 * time.Minute, want: 10 * time.Minute},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := media.ToolTimeoutBudget(tt.base, nil); got != tt.want {
				t.Fatalf("ToolTimeoutBudget(%s) = %s, want %s", tt.base, got, tt.want)
			}
		})
	}
}

func newTestImageTool(t *testing.T, baseURL string, credential CredentialSource) *ImageTool {
	t.Helper()
	media, err := NewMediaTool(MediaToolOptions{BaseURL: baseURL, Credential: credential})
	if err != nil {
		t.Fatalf("NewMediaTool: %v", err)
	}
	return &ImageTool{media: media}
}

func staticCredential(key string) CredentialSource {
	return func(context.Context) (string, error) { return key, nil }
}

type fakeVideoGenerationClient struct {
	createScope    cloud.BindingScope
	createKey      string
	createInput    cloud.VideoGenerationRequest
	createTask     cloud.VideoGenerationTask
	createErr      error
	getScope       cloud.BindingScope
	getTaskID      string
	getTask        cloud.VideoGenerationTask
	getTasks       []cloud.VideoGenerationTask
	getCalls       int
	getErr         error
	progressScope  cloud.BindingScope
	progressTaskID string
	progress       cloud.VideoGenerationProgress
	progresses     []cloud.VideoGenerationProgress
	progressCalls  int
	progressErr    error
}

func (f *fakeVideoGenerationClient) Create(
	_ context.Context,
	scope cloud.BindingScope,
	key string,
	input cloud.VideoGenerationRequest,
) (cloud.VideoGenerationTask, error) {
	f.createScope, f.createKey, f.createInput = scope, key, input
	return f.createTask, f.createErr
}

func (f *fakeVideoGenerationClient) Get(
	_ context.Context,
	scope cloud.BindingScope,
	taskID string,
) (cloud.VideoGenerationTask, error) {
	f.getScope, f.getTaskID = scope, taskID
	f.getCalls++
	if len(f.getTasks) > 0 {
		index := f.getCalls - 1
		if index >= len(f.getTasks) {
			index = len(f.getTasks) - 1
		}
		return f.getTasks[index], f.getErr
	}
	return f.getTask, f.getErr
}

func (f *fakeVideoGenerationClient) GetProgress(
	_ context.Context,
	scope cloud.BindingScope,
	taskID string,
) (cloud.VideoGenerationProgress, error) {
	f.progressScope, f.progressTaskID = scope, taskID
	f.progressCalls++
	if len(f.progresses) > 0 {
		index := f.progressCalls - 1
		if index >= len(f.progresses) {
			index = len(f.progresses) - 1
		}
		return f.progresses[index], f.progressErr
	}
	return f.progress, f.progressErr
}

func newTestVideoMediaTool(t *testing.T, client cloud.VideoGenerationClient, credential CredentialSource) *VideoTool {
	t.Helper()
	media, err := NewMediaTool(MediaToolOptions{
		BaseURL:     "https://example.invalid/v1",
		Credential:  credential,
		VideoClient: client,
		VideoScope: func(context.Context) (cloud.BindingScope, error) {
			return testVideoScope(), nil
		},
		VideoPollInterval: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewMediaTool: %v", err)
	}
	return &VideoTool{media: media}
}

func testVideoScope() cloud.BindingScope {
	return cloud.BindingScope{
		ProfileID: "profile-01", CloudUserID: "user-01", BindingGeneration: 1,
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	return string(encoded)
}

var _ tool.TimeoutBudgeter = (*VideoTool)(nil)

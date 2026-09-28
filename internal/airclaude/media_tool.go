package airclaude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/config"
	"kwu/internal/provider"
	"kwu/internal/tool"
)

const (
	defaultImageModel         = "gpt-image-2"
	defaultVideoModel         = "doubao-seedance-1-0-pro-250528"
	defaultVideoDuration      = 5
	defaultMaxResponseBytes   = int64(8 << 20)
	defaultVideoPollInterval  = 2 * time.Second
	minMediaGenerationTimeout = 5 * time.Minute
)

var generationPromptSchema = json.RawMessage(`{
	"type":"object",
	"properties":{
		"prompt":{"type":"string","minLength":1,"maxLength":4000,"description":"A detailed description of the image or video to generate."}
	},
	"required":["prompt"],
	"additionalProperties":false
}`)

// CredentialSource resolves the current authenticated MaaS credential at call
// time. Callers should fail closed when the account binding is no longer valid.
type CredentialSource func(context.Context) (string, error)

type VideoScopeSource = ScopeSource

// ImageQuotaPricing is the current MaaS pricing needed to turn an
// OpenAI-compatible image usage payload into the provider's integer quota.
type ImageQuotaPricing struct {
	InputPerMillion       string
	OutputPerMillion      string
	ImageInputPerMillion  string
	ImageOutputPerMillion string
	QuotaPerUSD           string
}

type ImageQuotaPricingSource func(context.Context, string) (ImageQuotaPricing, error)

// MediaToolOptions configures AirClaude image generation and Reasonix Cloud
// video generation. BaseURL and HTTPClient apply only to image requests.
type MediaToolOptions struct {
	Credential        CredentialSource
	ImageQuotaPricing ImageQuotaPricingSource
	VideoClient       cloud.VideoGenerationClient
	VideoScope        VideoScopeSource
	BaseURL           string
	HTTPClient        *http.Client
	OutputDir         string
	ImageModel        string
	MaxResponseBytes  int64
	VideoPollInterval time.Duration
}

// MediaTool contains the shared image request and video polling implementation
// used by the two model-callable generation tools.
type MediaTool struct {
	credential        CredentialSource
	imageQuotaPricing ImageQuotaPricingSource
	videoClient       cloud.VideoGenerationClient
	videoScope        VideoScopeSource
	baseURL           string
	client            *http.Client
	imageModel        string
	maxResponseBytes  int64
	outputDir         string
	videoPollInterval time.Duration
}

func NewMediaTool(options MediaToolOptions) (*MediaTool, error) {
	if options.Credential == nil && (options.VideoClient == nil || options.VideoScope == nil) {
		return nil, fmt.Errorf("media generation: image credential or video client is required")
	}
	baseURL, err := normalizeMediaBaseURL(options.BaseURL)
	if err != nil {
		return nil, err
	}
	return &MediaTool{
		credential:        options.Credential,
		imageQuotaPricing: options.ImageQuotaPricing,
		videoClient:       options.VideoClient,
		videoScope:        options.VideoScope,
		baseURL:           baseURL,
		client:            mediaHTTPClient(options.HTTPClient),
		imageModel:        mediaModel(options.ImageModel, defaultImageModel),
		maxResponseBytes:  mediaResponseLimit(options.MaxResponseBytes),
		outputDir:         strings.TrimSpace(options.OutputDir),
		videoPollInterval: mediaVideoPollInterval(options.VideoPollInterval),
	}, nil
}

// NewDefaultImageTool constructs the production image-generation tool.
func NewDefaultImageTool(credential CredentialSource, pricing ImageQuotaPricingSource, client *http.Client, outputDir string) *ImageTool {
	media := newDefaultMediaTool(credential, pricing, nil, nil, client, outputDir)
	return &ImageTool{media: media}
}

// NewDefaultVideoTool constructs the production video-generation tool.
func NewDefaultVideoTool(videoClient cloud.VideoGenerationClient, videoScope VideoScopeSource, client *http.Client, outputDir string) *VideoTool {
	media := newDefaultMediaTool(nil, nil, videoClient, videoScope, client, outputDir)
	return &VideoTool{media: media}
}

func newDefaultMediaTool(credential CredentialSource, pricing ImageQuotaPricingSource, videoClient cloud.VideoGenerationClient, videoScope VideoScopeSource, client *http.Client, outputDir string) *MediaTool {
	return &MediaTool{
		credential:        credential,
		imageQuotaPricing: pricing,
		videoClient:       videoClient,
		videoScope:        videoScope,
		baseURL:           config.OfficialProviderBaseURL,
		client:            mediaHTTPClient(client),
		imageModel:        defaultImageModel,
		maxResponseBytes:  defaultMaxResponseBytes,
		outputDir:         strings.TrimSpace(outputDir),
		videoPollInterval: defaultVideoPollInterval,
	}
}

func normalizeMediaBaseURL(raw string) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(raw), "/")
	if baseURL == "" {
		baseURL = config.OfficialProviderBaseURL
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("airclaude media: invalid base URL")
	}
	return baseURL, nil
}

func mediaHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		return http.DefaultClient
	}
	return client
}

func mediaModel(model, fallback string) string {
	if model = strings.TrimSpace(model); model != "" {
		return model
	}
	return fallback
}

func mediaResponseLimit(limit int64) int64 {
	if limit > 0 {
		return limit
	}
	return defaultMaxResponseBytes
}

func mediaVideoPollInterval(interval time.Duration) time.Duration {
	if interval > 0 {
		return interval
	}
	return defaultVideoPollInterval
}

type ImageTool struct{ media *MediaTool }

func (*ImageTool) Name() string { return "generate_image" }
func (*ImageTool) Description() string {
	return "Generate an image from the supplied description using the service defaults for model output settings, size, quality, background, and format."
}
func (*ImageTool) Schema() json.RawMessage { return generationPromptSchema }
func (*ImageTool) ReadOnly() bool          { return false }

// ToolTimeoutBudget gives remote image generation enough time to finish while
// preserving any longer timeout configured for tool calls.
func (*ImageTool) ToolTimeoutBudget(base time.Duration, _ json.RawMessage) time.Duration {
	if base < minMediaGenerationTimeout {
		return minMediaGenerationTimeout
	}
	return base
}
func (t *ImageTool) Execute(ctx context.Context, raw json.RawMessage) (string, error) {
	prompt, err := decodeGenerationPrompt(raw, t.Name())
	if err != nil {
		return "", err
	}
	return t.media.executeImage(ctx, mediaToolArgs{Action: t.Name(), Prompt: prompt})
}

type VideoTool struct{ media *MediaTool }

func (*VideoTool) Name() string { return "generate_video" }
func (*VideoTool) Description() string {
	return "Generate a video from the supplied description using the service defaults. The call waits until the finished video is available, the backend reports a terminal failure, or the user cancels the turn."
}
func (*VideoTool) Schema() json.RawMessage { return generationPromptSchema }
func (*VideoTool) ReadOnly() bool          { return false }
func (*VideoTool) ToolTimeoutBudget(base time.Duration, _ json.RawMessage) time.Duration {
	if base < minMediaGenerationTimeout {
		return minMediaGenerationTimeout
	}
	return base
}
func (t *VideoTool) Execute(ctx context.Context, raw json.RawMessage) (string, error) {
	prompt, err := decodeGenerationPrompt(raw, t.Name())
	if err != nil {
		return "", err
	}
	return t.media.executeVideoCreate(ctx, mediaToolArgs{Action: t.Name(), Prompt: prompt})
}

type generationPromptArgs struct {
	Prompt string `json:"prompt"`
}

func decodeGenerationPrompt(raw json.RawMessage, toolName string) (string, error) {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	var args generationPromptArgs
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&args); err != nil {
		return "", fmt.Errorf("%s: invalid arguments: %w", toolName, err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return "", fmt.Errorf("%s: invalid arguments: multiple JSON values", toolName)
	}
	prompt := strings.TrimSpace(args.Prompt)
	if prompt == "" {
		return "", fmt.Errorf("%s: prompt is required", toolName)
	}
	if len(prompt) > 4_000 {
		return "", fmt.Errorf("%s: prompt is too long", toolName)
	}
	return prompt, nil
}

type mediaToolArgs struct {
	Action       string `json:"action"`
	Prompt       string `json:"prompt"`
	Model        string `json:"model"`
	N            int    `json:"n"`
	Size         string `json:"size"`
	Quality      string `json:"quality"`
	Background   string `json:"background"`
	OutputFormat string `json:"output_format"`
}

func (t *MediaTool) executeImage(ctx context.Context, args mediaToolArgs) (string, error) {
	if t.credential == nil {
		return "", fmt.Errorf("media generation: image generation is unavailable")
	}
	apiKey, err := t.credential(ctx)
	if err != nil {
		return "", fmt.Errorf("media generation: MaaS credential unavailable: %w", err)
	}
	if strings.TrimSpace(apiKey) == "" {
		return "", fmt.Errorf("media generation: MaaS credential unavailable")
	}
	model := mediaModel(args.Model, t.imageModel)
	var pricing *ImageQuotaPricing
	if t.imageQuotaPricing != nil {
		resolved, pricingErr := t.imageQuotaPricing(ctx, model)
		if pricingErr != nil {
			return "", fmt.Errorf("media generation: image billing metadata unavailable: %w", pricingErr)
		}
		pricing = &resolved
	}
	request, err := t.newImageRequest(ctx, apiKey, args)
	if err != nil {
		return "", err
	}
	response, err := t.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("media generation: AirClaude request failed: %w", err)
	}
	defer response.Body.Close()
	return t.readResponse(ctx, response, args, pricing)
}

func (t *MediaTool) executeVideoCreate(ctx context.Context, args mediaToolArgs) (string, error) {
	scope, err := t.resolveVideoScope(ctx)
	if err != nil {
		return "", err
	}
	input := cloud.VideoGenerationRequest{
		Model: defaultVideoModel, Prompt: args.Prompt, Duration: defaultVideoDuration,
	}
	idempotencyKey, err := newVideoIdempotencyKey()
	if err != nil {
		return "", err
	}
	task, err := t.videoClient.Create(ctx, scope, idempotencyKey, input)
	if err != nil {
		return "", fmt.Errorf("media generation: Reasonix video task creation failed: %w", err)
	}
	return t.waitForVideoTask(ctx, scope, args, task)
}

func (t *MediaTool) waitForVideoTask(
	ctx context.Context,
	scope cloud.BindingScope,
	args mediaToolArgs,
	task cloud.VideoGenerationTask,
) (string, error) {
	pollInterval := t.videoPollInterval
	lastProgress := ""
	for {
		if result, terminal, err := t.videoTaskTerminalResult(ctx, args, task); terminal {
			return result, err
		}
		progress, nextInterval, signature, err := t.pollVideoProgress(
			ctx, scope, task.TaskID, pollInterval, lastProgress,
		)
		if err != nil {
			return "", err
		}
		task, err = t.videoTaskAfterProgress(ctx, scope, task, progress)
		if err != nil {
			return "", err
		}
		pollInterval, lastProgress = nextInterval, signature
	}
}

func (t *MediaTool) videoTaskTerminalResult(
	ctx context.Context,
	args mediaToolArgs,
	task cloud.VideoGenerationTask,
) (string, bool, error) {
	if videoTaskHasOutput(task) {
		if videoTaskReadyForDelivery(task) {
			result, err := t.formatVideoTask(ctx, args, task)
			return result, true, err
		}
		// Video output and billing settlement can become visible at different
		// instants. Keep polling while the task is identifiable so the turn never
		// drops a real charge merely because the URL arrived first.
		if strings.TrimSpace(task.TaskID) == "" {
			return "", true, fmt.Errorf("media generation: Reasonix video result is not settled")
		}
		return "", false, nil
	}
	if videoTaskFailed(task.Status) {
		message := "media generation: Reasonix video generation " + strings.ToLower(strings.TrimSpace(task.Status))
		if task.ProviderErrorCode != nil && strings.TrimSpace(*task.ProviderErrorCode) != "" {
			message += ": " + strings.TrimSpace(*task.ProviderErrorCode)
		}
		return "", true, fmt.Errorf("%s", message)
	}
	if strings.EqualFold(strings.TrimSpace(task.Status), "succeeded") {
		return "", true, fmt.Errorf("media generation: Reasonix video generation succeeded without a video URL")
	}
	if strings.TrimSpace(task.TaskID) == "" {
		return "", true, fmt.Errorf("media generation: Reasonix video generation returned an empty task ID")
	}
	return "", false, nil
}

func videoTaskReadyForDelivery(task cloud.VideoGenerationTask) bool {
	return strings.EqualFold(strings.TrimSpace(task.Status), "succeeded") &&
		strings.EqualFold(strings.TrimSpace(task.SettlementStatus), "settled") &&
		task.ChargedCredits != nil
}

func (t *MediaTool) pollVideoProgress(
	ctx context.Context,
	scope cloud.BindingScope,
	taskID string,
	interval time.Duration,
	lastSignature string,
) (cloud.VideoGenerationProgress, time.Duration, string, error) {
	if err := waitForMediaPoll(ctx, interval); err != nil {
		return cloud.VideoGenerationProgress{}, interval, lastSignature, err
	}
	progress, err := t.videoClient.GetProgress(ctx, scope, taskID)
	if err != nil {
		return cloud.VideoGenerationProgress{}, interval, lastSignature,
			fmt.Errorf("media generation: Reasonix video progress query failed: %w", err)
	}
	if strings.TrimSpace(progress.TaskID) != strings.TrimSpace(taskID) {
		return cloud.VideoGenerationProgress{}, interval, lastSignature,
			fmt.Errorf("media generation: Reasonix video progress returned a mismatched task ID")
	}
	signature := emitVideoProgress(ctx, progress, lastSignature)
	return progress, videoProgressPollInterval(t.videoPollInterval, progress.RetryAfterSeconds), signature, nil
}

func (t *MediaTool) videoTaskAfterProgress(
	ctx context.Context,
	scope cloud.BindingScope,
	task cloud.VideoGenerationTask,
	progress cloud.VideoGenerationProgress,
) (cloud.VideoGenerationTask, error) {
	if progress.Terminal && !progress.Succeeded {
		status := strings.ToLower(strings.TrimSpace(progress.Status))
		if status == "" {
			status = "failed"
		}
		return cloud.VideoGenerationTask{}, fmt.Errorf("media generation: Reasonix video generation %s", status)
	}
	if progress.Terminal {
		result, err := t.videoClient.Get(ctx, scope, task.TaskID)
		if err != nil {
			return cloud.VideoGenerationTask{}, fmt.Errorf("media generation: Reasonix video task query failed: %w", err)
		}
		// The terminal progress endpoint is itself authoritative for settlement.
		// Preserve its charge when the immediately-following full task read is
		// temporarily missing that optional projection.
		if result.ChargedCredits == nil && progress.ChargedCredits != nil {
			chargedCredits := *progress.ChargedCredits
			result.ChargedCredits = &chargedCredits
		}
		if strings.TrimSpace(result.SettlementStatus) == "" {
			result.SettlementStatus = progress.SettlementStatus
		}
		return result, nil
	}
	task.Status = progress.Status
	task.SettlementStatus = progress.SettlementStatus
	if !progress.UpdatedAt.IsZero() {
		task.UpdatedAt = progress.UpdatedAt
	}
	return task, nil
}

func emitVideoProgress(ctx context.Context, progress cloud.VideoGenerationProgress, lastSignature string) string {
	progress.ProgressPercent = normalizedVideoProgressPercent(progress.ProgressPercent)
	signature := fmt.Sprintf(
		"%s\x00%s\x00%d\x00%t\x00%t",
		progress.TaskID, progress.Status, progress.ProgressPercent, progress.Terminal, progress.Succeeded,
	)
	if signature == lastSignature {
		return signature
	}
	if report, ok := tool.ProgressFrom(ctx); ok {
		payload := struct {
			Kind              string `json:"kind"`
			TaskID            string `json:"task_id"`
			Status            string `json:"status"`
			SettlementStatus  string `json:"settlement_status"`
			ProgressPercent   int    `json:"progress_percent"`
			ProgressBasis     string `json:"progress_basis"`
			Terminal          bool   `json:"terminal"`
			Succeeded         bool   `json:"succeeded"`
			RetryAfterSeconds int    `json:"retry_after_seconds"`
		}{
			Kind:              "video_generation_progress",
			TaskID:            progress.TaskID,
			Status:            progress.Status,
			SettlementStatus:  progress.SettlementStatus,
			ProgressPercent:   progress.ProgressPercent,
			ProgressBasis:     progress.ProgressBasis,
			Terminal:          progress.Terminal,
			Succeeded:         progress.Succeeded,
			RetryAfterSeconds: progress.RetryAfterSeconds,
		}
		if encoded, err := json.Marshal(payload); err == nil {
			report(string(encoded) + "\n")
		}
	}
	return signature
}

func normalizedVideoProgressPercent(percent int) int {
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
}

func videoProgressPollInterval(fallback time.Duration, retryAfterSeconds int) time.Duration {
	if retryAfterSeconds <= 0 {
		return fallback
	}
	const maxProgressPollInterval = time.Minute
	const maxRetryAfterSeconds = int(maxProgressPollInterval / time.Second)
	if retryAfterSeconds > maxRetryAfterSeconds {
		return maxProgressPollInterval
	}
	return time.Duration(retryAfterSeconds) * time.Second
}

func waitForMediaPoll(ctx context.Context, interval time.Duration) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return fmt.Errorf("media generation: video generation wait interrupted: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

func videoTaskHasOutput(task cloud.VideoGenerationTask) bool {
	return task.VideoURL != nil && strings.TrimSpace(*task.VideoURL) != ""
}

func videoTaskFailed(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed", "cancelled", "canceled":
		return true
	default:
		return false
	}
}

func (t *MediaTool) resolveVideoScope(ctx context.Context) (cloud.BindingScope, error) {
	if t.videoClient == nil || t.videoScope == nil {
		return cloud.BindingScope{}, fmt.Errorf("media generation: Reasonix video generation is unavailable")
	}
	scope, err := t.videoScope(ctx)
	if err != nil {
		return cloud.BindingScope{}, fmt.Errorf("media generation: Reasonix authentication is unavailable: %w", err)
	}
	if err := scope.Validate(); err != nil {
		return cloud.BindingScope{}, fmt.Errorf("media generation: Reasonix authentication is unavailable")
	}
	return scope, nil
}

func (t *MediaTool) formatVideoTask(ctx context.Context, args mediaToolArgs, task cloud.VideoGenerationTask) (string, error) {
	if task.ChargedCredits != nil {
		if err := tool.RecordCredit(ctx, provider.ToolChargeMetadata{
			Tool: "generate_video", ServiceTaskID: task.TaskID,
			Kind: provider.ToolChargeCredits, Value: *task.ChargedCredits,
		}); err != nil {
			return "", fmt.Errorf("media generation: record Reasonix video credits: %w", err)
		}
	}
	encoded, err := json.Marshal(task)
	if err != nil {
		return "", fmt.Errorf("media generation: encode Reasonix video task: %w", err)
	}
	return t.formatMediaResponse(args, encoded)
}

func (t *MediaTool) newImageRequest(ctx context.Context, apiKey string, args mediaToolArgs) (*http.Request, error) {
	model := args.Model
	if model == "" {
		model = t.imageModel
	}
	body := map[string]any{"model": model, "prompt": args.Prompt}
	if args.N > 0 {
		body["n"] = args.N
	}
	addNonEmptyAny(body, "size", args.Size)
	addNonEmptyAny(body, "quality", args.Quality)
	addNonEmptyAny(body, "background", args.Background)
	addNonEmptyAny(body, "output_format", args.OutputFormat)
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("media generation: encode image request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL+"/images/generations", bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("media generation: create image request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	setMediaHeaders(request, apiKey)
	return request, nil
}

func setMediaHeaders(request *http.Request, apiKey string) {
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	request.Header.Set("Accept", "application/json")
}

func addNonEmptyAny(values map[string]any, key, value string) {
	if value != "" {
		values[key] = value
	}
}

func (t *MediaTool) readResponse(ctx context.Context, response *http.Response, args mediaToolArgs, pricing *ImageQuotaPricing) (string, error) {
	limited := io.LimitReader(response.Body, t.maxResponseBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("media generation: read AirClaude response: %w", err)
	}
	if int64(len(body)) > t.maxResponseBytes {
		return "", fmt.Errorf("media generation: AirClaude response exceeds %s bytes", strconv.FormatInt(t.maxResponseBytes, 10))
	}
	trimmed := bytes.TrimSpace(body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("media generation: AirClaude returned HTTP %d: %s", response.StatusCode, mediaHTTPErrorMessage(response.StatusCode, trimmed))
	}
	if len(trimmed) == 0 {
		return "", fmt.Errorf("media generation: AirClaude returned an empty response")
	}
	if !json.Valid(trimmed) {
		return "", fmt.Errorf("media generation: AirClaude returned invalid JSON")
	}
	if err := recordImageQuota(ctx, trimmed, pricing); err != nil {
		return "", err
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, trimmed); err != nil {
		return "", fmt.Errorf("media generation: compact AirClaude response: %w", err)
	}
	return t.formatMediaResponse(args, compact.Bytes())
}

func mediaHTTPErrorMessage(statusCode int, body []byte) string {
	message := string(body)
	if len(message) > 2_000 {
		return message[:2_000] + "..."
	}
	if message == "" {
		return http.StatusText(statusCode)
	}
	return message
}

func recordImageQuota(ctx context.Context, payload []byte, pricing *ImageQuotaPricing) error {
	quota, ok, quotaErr := mediaResponseQuota(payload)
	if quotaErr != nil {
		return fmt.Errorf("media generation: invalid AirClaude billing quota: %w", quotaErr)
	}
	if !ok && pricing != nil {
		quota, quotaErr = estimateImageMaaSQuota(payload, *pricing)
		if quotaErr != nil {
			return fmt.Errorf("media generation: estimate AirClaude image quota: %w", quotaErr)
		}
		ok = true
	}
	if !ok {
		// OpenAI-compatible image responses are also valid with only a data
		// field. Standalone callers without a pricing source keep the successful
		// result; production subscription wiring always provides pricing.
		return nil
	}
	if err := tool.RecordCredit(ctx, provider.ToolChargeMetadata{
		Tool: "generate_image", Kind: provider.ToolChargeMaaSQuota, Value: quota,
	}); err != nil {
		return fmt.Errorf("media generation: record AirClaude image quota: %w", err)
	}
	return nil
}

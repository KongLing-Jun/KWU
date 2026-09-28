package airclaude

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"kwu/internal/cloud"
	"kwu/internal/provider"
	"kwu/internal/tool"
)

const (
	maxUnderstandingPromptBytes = 8000
	maxUnderstandingImageBytes  = int64(10 << 20)
	maxUnderstandingAudioBytes  = int64(25 << 20)
	maxUnderstandingVideoBytes  = int64(47 << 20)
	minUnderstandingTimeout     = 120 * time.Second
)

var mediaUnderstandingToolSchema = json.RawMessage(`{
	"type":"object",
	"properties":{
		"media_type":{
			"type":"string",
			"enum":["image","video","audio"],
			"description":"The kind of media being understood."
		},
		"file_path":{
			"type":"string",
			"description":"A local media file inside the current workspace, including a user attachment path. Use exactly one of file_path or media_url."
		},
		"media_url":{
			"type":"string",
			"description":"A public HTTPS media URL. Localhost, private IPs, and URLs containing credentials are rejected. Use exactly one of media_url or file_path."
		},
		"prompt":{
			"type":"string",
			"description":"Optional instructions for transcription, OCR, description, or summarization. Maximum 8000 UTF-8 bytes."
		}
	},
	"required":["media_type"],
	"additionalProperties":false
}`)

// ScopeSource resolves the currently authorized Reasonix account binding at
// call time, so tools cannot retain authorization across logout/account switch.
type ScopeSource func(context.Context) (cloud.BindingScope, error)

type MediaUnderstandingTool struct {
	client        cloud.MediaUnderstandingClient
	scope         ScopeSource
	workspaceRoot string

	pendingMu   sync.Mutex
	pendingKeys map[string]string
}

func NewMediaUnderstandingTool(client cloud.MediaUnderstandingClient, scope ScopeSource, workspaceRoot string) (*MediaUnderstandingTool, error) {
	if client == nil || scope == nil {
		return nil, fmt.Errorf("understand_media: client and account scope are required")
	}
	root, err := filepath.Abs(strings.TrimSpace(workspaceRoot))
	if err != nil || strings.TrimSpace(workspaceRoot) == "" {
		return nil, fmt.Errorf("understand_media: workspace root is invalid")
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("understand_media: resolve workspace root: %w", err)
	}
	return &MediaUnderstandingTool{
		client: client, scope: scope, workspaceRoot: filepath.Clean(root),
		pendingKeys: make(map[string]string),
	}, nil
}

func (*MediaUnderstandingTool) Name() string { return "understand_media" }

func (*MediaUnderstandingTool) Description() string {
	return "Convert an image, video, or audio file into text through authenticated Reasonix Cloud. Use it when direct multimodal input is unavailable or when explicit OCR, transcription, visual description, or media summarization is useful. The service model is fixed and is not a tool argument."
}

func (*MediaUnderstandingTool) Schema() json.RawMessage { return mediaUnderstandingToolSchema }

// Understanding consumes a paid remote service even though it does not mutate
// the workspace, so it must not be parallelized as a read-only host operation.
func (*MediaUnderstandingTool) ReadOnly() bool { return false }

// ToolTimeoutBudget gives media understanding enough time to upload and process
// larger audio or video inputs while preserving any longer configured timeout.
func (*MediaUnderstandingTool) ToolTimeoutBudget(base time.Duration, _ json.RawMessage) time.Duration {
	if base < minUnderstandingTimeout {
		return minUnderstandingTimeout
	}
	return base
}

type mediaUnderstandingToolArgs struct {
	MediaType string `json:"media_type"`
	FilePath  string `json:"file_path"`
	MediaURL  string `json:"media_url"`
	Prompt    string `json:"prompt"`
}

func (t *MediaUnderstandingTool) Execute(ctx context.Context, raw json.RawMessage) (string, error) {
	args, err := decodeMediaUnderstandingArgs(raw)
	if err != nil {
		return "", err
	}
	scope, err := t.authorizedScope(ctx)
	if err != nil {
		return "", err
	}
	source, err := t.prepareUnderstandingSource(scope, args)
	if err != nil {
		return "", err
	}
	key, err := t.pendingKey(source.fingerprint)
	if err != nil {
		return "", err
	}
	result, err := t.callUnderstanding(ctx, scope, key, args, source)
	if err != nil {
		return "", t.understandingRequestError(source.fingerprint, err)
	}
	t.clearPendingKey(source.fingerprint)
	if err := validateUnderstandingResult(result, args.MediaType); err != nil {
		return "", err
	}
	if err := tool.RecordCredit(ctx, provider.ToolChargeMetadata{
		Tool: t.Name(), ServiceTaskID: result.TaskID,
		Kind: provider.ToolChargeCredits, Value: result.ChargedCredits,
	}); err != nil {
		return "", fmt.Errorf("understand_media: record settled credits: %w", err)
	}
	return encodeUnderstandingResult(result, args.MediaType)
}

func (t *MediaUnderstandingTool) authorizedScope(ctx context.Context) (cloud.BindingScope, error) {
	scope, err := t.scope(ctx)
	if err != nil {
		return cloud.BindingScope{}, fmt.Errorf("understand_media: Reasonix authentication is unavailable: %w", err)
	}
	if err := scope.Validate(); err != nil {
		return cloud.BindingScope{}, fmt.Errorf("understand_media: Reasonix authentication is unavailable")
	}
	return scope, nil
}

type understandingSource struct {
	fingerprint string
	filename    string
	content     []byte
}

func (t *MediaUnderstandingTool) prepareUnderstandingSource(
	scope cloud.BindingScope,
	args mediaUnderstandingToolArgs,
) (understandingSource, error) {
	if args.MediaURL != "" {
		if err := validateUnderstandingURL(args.MediaURL); err != nil {
			return understandingSource{}, err
		}
		return understandingSource{fingerprint: understandingFingerprint(scope, args, nil)}, nil
	}
	filename, content, err := t.readLocalMedia(args.MediaType, args.FilePath)
	if err != nil {
		return understandingSource{}, err
	}
	return understandingSource{
		fingerprint: understandingFingerprint(scope, args, content),
		filename:    filename,
		content:     content,
	}, nil
}

func (t *MediaUnderstandingTool) callUnderstanding(
	ctx context.Context,
	scope cloud.BindingScope,
	key string,
	args mediaUnderstandingToolArgs,
	source understandingSource,
) (cloud.MediaUnderstandingResult, error) {
	if args.MediaURL != "" {
		return t.client.UnderstandURL(ctx, scope, key, cloud.MediaUnderstandingRequest{
			MediaType: args.MediaType, MediaURL: args.MediaURL, Prompt: args.Prompt,
		})
	}
	return t.client.UnderstandFile(
		ctx, scope, key, args.MediaType, args.Prompt, source.filename, source.content,
	)
}

func (t *MediaUnderstandingTool) understandingRequestError(fingerprint string, err error) error {
	if retainUnderstandingKey(err) {
		return fmt.Errorf("understand_media: request is pending or its delivery is uncertain; retry the same tool call so the original idempotency key is reused: %w", err)
	}
	t.clearPendingKey(fingerprint)
	return fmt.Errorf("understand_media: Reasonix request failed: %w", err)
}

func encodeUnderstandingResult(result cloud.MediaUnderstandingResult, mediaType string) (string, error) {
	if err := validateUnderstandingResult(result, mediaType); err != nil {
		return "", err
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("understand_media: encode result: %w", err)
	}
	return string(encoded), nil
}

func decodeMediaUnderstandingArgs(raw json.RawMessage) (mediaUnderstandingToolArgs, error) {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	var args mediaUnderstandingToolArgs
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&args); err != nil {
		return mediaUnderstandingToolArgs{}, fmt.Errorf("understand_media: invalid arguments: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return mediaUnderstandingToolArgs{}, fmt.Errorf("understand_media: invalid arguments: multiple JSON values")
	}
	args.MediaType = strings.ToLower(strings.TrimSpace(args.MediaType))
	args.FilePath = strings.TrimSpace(args.FilePath)
	args.MediaURL = strings.TrimSpace(args.MediaURL)
	args.Prompt = strings.TrimSpace(args.Prompt)
	switch args.MediaType {
	case "image", "video", "audio":
	default:
		return mediaUnderstandingToolArgs{}, fmt.Errorf("understand_media: media_type must be image, video, or audio")
	}
	if (args.FilePath == "") == (args.MediaURL == "") {
		return mediaUnderstandingToolArgs{}, fmt.Errorf("understand_media: provide exactly one of file_path or media_url")
	}
	if len([]byte(args.Prompt)) > maxUnderstandingPromptBytes {
		return mediaUnderstandingToolArgs{}, fmt.Errorf("understand_media: prompt exceeds 8000 UTF-8 bytes")
	}
	return args, nil
}

func validateUnderstandingURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(parsed.Scheme, "https") || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("understand_media: media_url must be a public HTTPS URL without credentials")
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return fmt.Errorf("understand_media: media_url must not target localhost or a private address")
	}
	if ip := net.ParseIP(host); ip != nil && blockedUnderstandingIP(ip) {
		return fmt.Errorf("understand_media: media_url must not target localhost or a private address")
	}
	return nil
}

func blockedUnderstandingIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}

var understandingExtensions = map[string]map[string]bool{
	"image": {
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
		".bmp": true, ".tif": true, ".tiff": true,
	},
	"audio": {
		".wav": true, ".mp3": true, ".aac": true, ".m4a": true,
	},
	"video": {
		".mp4": true, ".mov": true, ".avi": true,
	},
}

func (t *MediaUnderstandingTool) readLocalMedia(mediaType, path string) (string, []byte, error) {
	resolved, err := t.resolveLocalMediaPath(path)
	if err != nil {
		return "", nil, err
	}
	info, limit, err := inspectLocalMedia(resolved, mediaType)
	if err != nil {
		return "", nil, err
	}
	content, err := readStableMediaFile(resolved, info, limit)
	if err != nil {
		return "", nil, err
	}
	return filepath.Base(resolved), content, nil
}

func (t *MediaUnderstandingTool) resolveLocalMediaPath(path string) (string, error) {
	target := path
	if !filepath.IsAbs(target) {
		target = filepath.Join(t.workspaceRoot, target)
	}
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		return "", fmt.Errorf("understand_media: resolve file_path: %w", err)
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil || !pathWithin(t.workspaceRoot, resolved) {
		return "", fmt.Errorf("understand_media: file_path must stay inside the current workspace")
	}
	return resolved, nil
}

func inspectLocalMedia(resolved, mediaType string) (os.FileInfo, int64, error) {
	info, err := os.Lstat(resolved)
	if err != nil {
		return nil, 0, fmt.Errorf("understand_media: inspect file_path: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || info.IsDir() {
		return nil, 0, fmt.Errorf("understand_media: file_path must be a regular file")
	}
	extension := strings.ToLower(filepath.Ext(resolved))
	if !understandingExtensions[mediaType][extension] {
		return nil, 0, fmt.Errorf("understand_media: %s file format %q is not supported", mediaType, extension)
	}
	limit := understandingSizeLimit(mediaType)
	if info.Size() <= 0 || info.Size() > limit {
		return nil, 0, fmt.Errorf("understand_media: %s file must be between 1 byte and %d MiB", mediaType, limit>>20)
	}
	return info, limit, nil
}

func readStableMediaFile(resolved string, info os.FileInfo, limit int64) ([]byte, error) {
	file, err := os.Open(resolved)
	if err != nil {
		return nil, fmt.Errorf("understand_media: open file_path: %w", err)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, fmt.Errorf("understand_media: media file changed while opening")
	}
	content, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, fmt.Errorf("understand_media: read media file: %w", err)
	}
	after, statErr := file.Stat()
	if statErr != nil || !os.SameFile(opened, after) || after.Size() != opened.Size() {
		return nil, fmt.Errorf("understand_media: media file changed while reading")
	}
	if int64(len(content)) != info.Size() || int64(len(content)) > limit {
		return nil, fmt.Errorf("understand_media: media file exceeds %d MiB", limit>>20)
	}
	return content, nil
}

func understandingSizeLimit(mediaType string) int64 {
	switch mediaType {
	case "image":
		return maxUnderstandingImageBytes
	case "audio":
		return maxUnderstandingAudioBytes
	default:
		return maxUnderstandingVideoBytes
	}
}

func pathWithin(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	return err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))))
}

func understandingFingerprint(scope cloud.BindingScope, args mediaUnderstandingToolArgs, content []byte) string {
	contentHash := sha256.Sum256(content)
	value := strings.Join([]string{
		scope.ProfileID, scope.CloudUserID, fmt.Sprint(scope.BindingGeneration),
		args.MediaType, args.FilePath, args.MediaURL, args.Prompt, hex.EncodeToString(contentHash[:]),
	}, "\x00")
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (t *MediaUnderstandingTool) pendingKey(fingerprint string) (string, error) {
	t.pendingMu.Lock()
	defer t.pendingMu.Unlock()
	if key := t.pendingKeys[fingerprint]; key != "" {
		return key, nil
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("understand_media: create idempotency key: %w", err)
	}
	key := "media-" + hex.EncodeToString(random)
	t.pendingKeys[fingerprint] = key
	return key, nil
}

func (t *MediaUnderstandingTool) clearPendingKey(fingerprint string) {
	t.pendingMu.Lock()
	delete(t.pendingKeys, fingerprint)
	t.pendingMu.Unlock()
}

func retainUnderstandingKey(err error) bool {
	var cloudErr *cloud.Error
	if !errors.As(err, &cloudErr) {
		return false
	}
	return cloudErr.Code == cloud.ErrorSettlementPending ||
		cloudErr.Code == cloud.ErrorNetworkUnavailable ||
		cloudErr.Retryable
}

func validateUnderstandingResult(result cloud.MediaUnderstandingResult, mediaType string) error {
	if result.Model != cloud.MediaUnderstandingModel {
		return fmt.Errorf("understand_media: service returned unexpected model %q", result.Model)
	}
	if result.MediaType != mediaType {
		return fmt.Errorf("understand_media: service returned mismatched media_type %q", result.MediaType)
	}
	if result.Status != "succeeded" || result.SettlementStatus != "settled" {
		return fmt.Errorf("understand_media: service returned an unsettled result")
	}
	if strings.TrimSpace(result.TaskID) == "" {
		return fmt.Errorf("understand_media: service returned an empty task_id")
	}
	return nil
}

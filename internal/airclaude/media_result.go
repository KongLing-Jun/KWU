package airclaude

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type mediaArtifact struct {
	Type      string `json:"type"`
	Path      string `json:"path,omitempty"`
	URL       string `json:"url,omitempty"`
	MIMEType  string `json:"mime_type,omitempty"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
}

func (t *MediaTool) formatMediaResponse(args mediaToolArgs, compact []byte) (string, error) {
	var payload any
	if err := json.Unmarshal(compact, &payload); err != nil {
		return "", fmt.Errorf("media generation: decode AirClaude response: %w", err)
	}
	artifacts := make([]mediaArtifact, 0, 2)
	seen := map[string]bool{}
	rewritten, err := t.rewriteMediaPayload(payload, "", args, &artifacts, seen)
	if err != nil {
		return "", err
	}
	if len(artifacts) == 0 {
		return string(compact), nil
	}
	root, ok := rewritten.(map[string]any)
	if !ok {
		root = map[string]any{"response": rewritten}
	}
	root["action"] = args.Action
	root["media"] = artifacts
	root["message"] = "Media is available in the chat preview. At the end of the final response, list each media path or URL exactly as returned in media."
	encoded, err := json.Marshal(root)
	if err != nil {
		return "", fmt.Errorf("media generation: encode media result: %w", err)
	}
	return string(encoded), nil
}

func (t *MediaTool) rewriteMediaPayload(value any, key string, args mediaToolArgs, artifacts *[]mediaArtifact, seen map[string]bool) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		return t.rewriteMediaMap(typed, args, artifacts, seen)
	case []any:
		return t.rewriteMediaSlice(typed, key, args, artifacts, seen)
	case string:
		if raw, mimeType, ok := decodeInlineMedia(key, typed, args.Action); ok {
			artifact, err := t.saveInlineMedia(raw, mimeType, args)
			if err != nil {
				return nil, err
			}
			appendMediaArtifact(artifacts, seen, artifact)
			return "[saved to " + artifact.Path + "]", nil
		}
		if artifact, ok := remoteMediaArtifact(key, typed, args.Action); ok {
			appendMediaArtifact(artifacts, seen, artifact)
		}
	}
	return value, nil
}

func (t *MediaTool) rewriteMediaMap(value map[string]any, args mediaToolArgs, artifacts *[]mediaArtifact, seen map[string]bool) (map[string]any, error) {
	for key, child := range value {
		rewritten, err := t.rewriteMediaPayload(child, key, args, artifacts, seen)
		if err != nil {
			return nil, err
		}
		value[key] = rewritten
	}
	return value, nil
}

func (t *MediaTool) rewriteMediaSlice(value []any, key string, args mediaToolArgs, artifacts *[]mediaArtifact, seen map[string]bool) ([]any, error) {
	for index, child := range value {
		rewritten, err := t.rewriteMediaPayload(child, key, args, artifacts, seen)
		if err != nil {
			return nil, err
		}
		value[index] = rewritten
	}
	return value, nil
}

func decodeInlineMedia(key, value, action string) ([]byte, string, bool) {
	lowerKey := strings.ToLower(strings.TrimSpace(key))
	mediaType := mediaTypeForAction(action)
	mimeType := defaultMediaMIME(mediaType)
	encoded := strings.TrimSpace(value)
	if strings.HasPrefix(strings.ToLower(encoded), "data:") {
		header, body, found := strings.Cut(encoded, ",")
		if !found || !strings.Contains(strings.ToLower(header), ";base64") {
			return nil, "", false
		}
		mimeType = strings.TrimSpace(strings.TrimPrefix(strings.SplitN(header, ";", 2)[0], "data:"))
		encoded = body
	} else if !inlineMediaKey(lowerKey) {
		return nil, "", false
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(raw) == 0 {
		return nil, "", false
	}
	if detected := http.DetectContentType(raw); strings.HasPrefix(detected, "image/") || strings.HasPrefix(detected, "video/") {
		mimeType = detected
	}
	return raw, mimeType, true
}

func inlineMediaKey(key string) bool {
	switch key {
	case "b64_json", "base64", "image_base64", "video_base64", "b64":
		return true
	default:
		return false
	}
}

func (t *MediaTool) saveInlineMedia(raw []byte, mimeType string, args mediaToolArgs) (mediaArtifact, error) {
	if strings.TrimSpace(t.outputDir) == "" {
		return mediaArtifact{}, fmt.Errorf("media generation: media output directory is unavailable")
	}
	outputDir, err := filepath.Abs(t.outputDir)
	if err != nil {
		return mediaArtifact{}, fmt.Errorf("media generation: resolve media output directory: %w", err)
	}
	if err := os.MkdirAll(outputDir, 0o700); err != nil {
		return mediaArtifact{}, fmt.Errorf("media generation: create media output directory: %w", err)
	}
	mediaType := mediaTypeForAction(args.Action)
	extension := mediaExtension(mimeType, args.OutputFormat, mediaType)
	filename, err := uniqueMediaFilename(mediaType, extension)
	if err != nil {
		return mediaArtifact{}, err
	}
	path := filepath.Join(outputDir, filename)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return mediaArtifact{}, fmt.Errorf("media generation: save generated %s: %w", mediaType, err)
	}
	return mediaArtifact{Type: mediaType, Path: path, MIMEType: mimeType, SizeBytes: int64(len(raw))}, nil
}

func uniqueMediaFilename(mediaType, extension string) (string, error) {
	random := make([]byte, 6)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("media generation: create media filename: %w", err)
	}
	stamp := time.Now().UTC().Format("20060102T150405.000000000Z")
	return mediaType + "-" + stamp + "-" + hex.EncodeToString(random) + extension, nil
}

func newVideoIdempotencyKey() (string, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("media generation: create video idempotency key: %w", err)
	}
	return "video-" + hex.EncodeToString(random), nil
}

func remoteMediaArtifact(key, value, action string) (mediaArtifact, bool) {
	lowerKey := strings.ToLower(strings.TrimSpace(key))
	if !strings.Contains(lowerKey, "url") {
		return mediaArtifact{}, false
	}
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return mediaArtifact{}, false
	}
	mediaType := mediaTypeForAction(action)
	if action == "generate_video" && !strings.Contains(lowerKey, "video") && !strings.Contains(lowerKey, "output") {
		return mediaArtifact{}, false
	}
	mimeType := mediaMIMEFromPath(parsed.Path, mediaType)
	return mediaArtifact{Type: mediaType, URL: parsed.String(), MIMEType: mimeType}, true
}

func appendMediaArtifact(artifacts *[]mediaArtifact, seen map[string]bool, artifact mediaArtifact) {
	key := artifact.Path + "\x00" + artifact.URL
	if key == "\x00" || seen[key] {
		return
	}
	seen[key] = true
	*artifacts = append(*artifacts, artifact)
}

func mediaTypeForAction(action string) string {
	if action == "generate_image" {
		return "image"
	}
	return "video"
}

func defaultMediaMIME(mediaType string) string {
	if mediaType == "image" {
		return "image/png"
	}
	return "video/mp4"
}

var imageExtensionsByFormat = map[string]string{
	"jpg": ".jpg", "jpeg": ".jpg", "webp": ".webp", "png": ".png",
}

var mediaExtensionsByMIME = map[string]string{
	"image/jpeg": ".jpg", "image/webp": ".webp", "image/gif": ".gif",
	"video/webm": ".webm", "video/quicktime": ".mov", "video/x-m4v": ".m4v", "video/mp4": ".mp4",
}

func mediaExtension(mimeType, requestedFormat, mediaType string) string {
	if mediaType == "image" {
		if extension := imageExtensionsByFormat[strings.ToLower(strings.TrimSpace(requestedFormat))]; extension != "" {
			return extension
		}
	}
	mimeType = strings.ToLower(strings.TrimSpace(strings.SplitN(mimeType, ";", 2)[0]))
	if extension := mediaExtensionsByMIME[mimeType]; extension != "" {
		return extension
	}
	if mediaType == "image" {
		return ".png"
	}
	return ".mp4"
}

func mediaMIMEFromPath(path, mediaType string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".webm":
		return "video/webm"
	case ".mov":
		return "video/quicktime"
	case ".m4v":
		return "video/x-m4v"
	case ".mp4":
		return "video/mp4"
	default:
		return defaultMediaMIME(mediaType)
	}
}

package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	cloud "kwu/internal/cloud"
	"kwu/internal/userctx"
)

const (
	feedbackImageLimit           = 4
	feedbackSelectedFileMaxBytes = 5 * 1024 * 1024
	// Keep a small deterministic-ZIP header allowance so any accepted multi-image
	// draft can still fit the stricter 20 MiB desktop attachment cap.
	feedbackImageStoreMaxBytes     = cloud.ErrorReportAttachmentMaxBytes - 4*1024
	feedbackImageMaxDimension      = 8192
	feedbackImageMaxPixels         = 25_000_000
	feedbackThumbnailMaxDimension  = 320
	feedbackThumbnailMaxDataBytes  = 256 * 1024
	feedbackDescriptionMaxRunes    = 300
	feedbackSubmitTimeout          = 120 * time.Second
	feedbackAttemptIDMaxBytes      = 128
	feedbackSafeFilenameMaxRunes   = 80
	feedbackImageTokenRandomBytes  = 24
	feedbackImageTokenPrefix       = "feedback:"
	feedbackMultiAttachmentName    = "feedback-images.zip"
	feedbackSinglePNGContentType   = "image/png"
	feedbackSingleJPEGContentType  = "image/jpeg"
	feedbackMultiZIPContentType    = "application/zip"
	feedbackThumbnailJPEGDataURL   = "data:image/jpeg;base64,"
	feedbackDeterministicZIPMethod = zip.Store
)

var feedbackZIPTimestamp = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)

type FeedbackImageView struct {
	Token            string `json:"token"`
	Filename         string `json:"filename"`
	ContentType      string `json:"content_type"`
	ThumbnailDataURL string `json:"thumbnail_data_url"`
	SizeBytes        int64  `json:"size_bytes"`
}

type FeedbackImageSelectionView struct {
	Images []FeedbackImageView `json:"images"`
}

type FeedbackSubmitRequest struct {
	Description string   `json:"description"`
	AttemptID   string   `json:"attempt_id"`
	ImageTokens []string `json:"image_tokens"`
}

type FeedbackOperationErrorView struct {
	Code         cloud.ErrorCode `json:"code"`
	Message      string          `json:"message"`
	Retryable    bool            `json:"retryable"`
	RequestID    string          `json:"request_id,omitempty"`
	DiagnosticID string          `json:"diagnostic_id,omitempty"`
}

type FeedbackSubmitView struct {
	ReportID       string                      `json:"report_id,omitempty"`
	Replayed       bool                        `json:"replayed"`
	OperationError *FeedbackOperationErrorView `json:"operation_error,omitempty"`
}

type feedbackImageEntry struct {
	scope            cloud.BindingScope
	filename         string
	contentType      string
	data             []byte
	thumbnailDataURL string
}

func (e feedbackImageEntry) view(token string) FeedbackImageView {
	return FeedbackImageView{
		Token:            token,
		Filename:         e.filename,
		ContentType:      e.contentType,
		ThumbnailDataURL: e.thumbnailDataURL,
		SizeBytes:        int64(len(e.data)),
	}
}

type feedbackImageStore struct {
	mu         sync.Mutex
	byToken    map[string]feedbackImageEntry
	totalBytes int64
}

func newFeedbackImageStore() *feedbackImageStore {
	return &feedbackImageStore{byToken: make(map[string]feedbackImageEntry)}
}

func (s *feedbackImageStore) add(scope cloud.BindingScope, existingTokens []string, entries []feedbackImageEntry) ([]FeedbackImageView, error) {
	if s == nil {
		return nil, feedbackLocalError("FEEDBACK_UNAVAILABLE", "意见反馈暂不可用。")
	}
	if err := scope.Validate(); err != nil {
		return nil, feedbackLocalError("AUTH_REQUIRED", "登录状态已失效，请重新登录。")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.validateTokensLocked(scope, existingTokens); err != nil {
		return nil, err
	}
	if len(existingTokens)+len(entries) > feedbackImageLimit {
		return nil, feedbackLocalError("FEEDBACK_IMAGE_LIMIT", "最多只能添加 4 张图片。")
	}
	var addedBytes int64
	for _, entry := range entries {
		if entry.scope != scope || len(entry.data) == 0 || entry.thumbnailDataURL == "" {
			return nil, feedbackLocalError("FEEDBACK_IMAGE_INVALID", "图片处理失败，请重新选择。")
		}
		addedBytes += int64(len(entry.data))
		if addedBytes > feedbackImageStoreMaxBytes || s.totalBytes > feedbackImageStoreMaxBytes-addedBytes {
			return nil, feedbackLocalError("FEEDBACK_IMAGE_TOTAL_TOO_LARGE", "图片总大小不能超过 20 MiB。")
		}
	}
	tokens := make([]string, 0, len(entries))
	reserved := make(map[string]struct{}, len(entries))
	for range entries {
		token, err := newFeedbackImageToken()
		if err != nil {
			return nil, feedbackLocalError("FEEDBACK_IMAGE_STORE_FAILED", "图片暂时无法保存，请重试。")
		}
		for {
			_, stored := s.byToken[token]
			_, pending := reserved[token]
			if !stored && !pending {
				break
			}
			token, err = newFeedbackImageToken()
			if err != nil {
				return nil, feedbackLocalError("FEEDBACK_IMAGE_STORE_FAILED", "图片暂时无法保存，请重试。")
			}
		}
		reserved[token] = struct{}{}
		tokens = append(tokens, token)
	}
	views := make([]FeedbackImageView, 0, len(entries))
	for index, entry := range entries {
		token := tokens[index]
		s.byToken[token] = entry
		s.totalBytes += int64(len(entry.data))
		views = append(views, entry.view(token))
	}
	return views, nil
}

func (s *feedbackImageStore) snapshot(scope cloud.BindingScope, tokens []string) ([]feedbackImageEntry, error) {
	if s == nil {
		return nil, feedbackLocalError("FEEDBACK_UNAVAILABLE", "意见反馈暂不可用。")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.validateTokensLocked(scope, tokens); err != nil {
		return nil, err
	}
	entries := make([]feedbackImageEntry, 0, len(tokens))
	for _, token := range tokens {
		entry := s.byToken[token]
		entry.data = append([]byte(nil), entry.data...)
		entries = append(entries, entry)
	}
	return entries, nil
}

func (s *feedbackImageStore) validateTokens(scope cloud.BindingScope, tokens []string) error {
	if s == nil {
		return feedbackLocalError("FEEDBACK_UNAVAILABLE", "意见反馈暂不可用。")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.validateTokensLocked(scope, tokens)
}

func (s *feedbackImageStore) validateTokensLocked(scope cloud.BindingScope, tokens []string) error {
	if len(tokens) > feedbackImageLimit {
		return feedbackLocalError("FEEDBACK_IMAGE_LIMIT", "最多只能添加 4 张图片。")
	}
	seen := make(map[string]struct{}, len(tokens))
	for _, rawToken := range tokens {
		token := strings.TrimSpace(rawToken)
		if token == "" || token != rawToken {
			return feedbackLocalError("FEEDBACK_DRAFT_INVALID", "反馈草稿已失效，请重新添加图片。")
		}
		if _, duplicate := seen[token]; duplicate {
			return feedbackLocalError("FEEDBACK_DRAFT_INVALID", "反馈草稿已失效，请重新添加图片。")
		}
		seen[token] = struct{}{}
		entry, ok := s.byToken[token]
		if !ok || entry.scope != scope {
			return feedbackLocalError("FEEDBACK_DRAFT_INVALID", "反馈草稿已失效，请重新添加图片。")
		}
	}
	return nil
}

func (s *feedbackImageStore) removeForProfile(profileID string, tokens []string) error {
	if s == nil {
		return nil
	}
	profileID = strings.TrimSpace(profileID)
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := make(map[string]struct{}, len(tokens))
	for _, rawToken := range tokens {
		token := strings.TrimSpace(rawToken)
		if token == "" || token != rawToken {
			return feedbackLocalError("FEEDBACK_DRAFT_INVALID", "反馈草稿已失效，请重新添加图片。")
		}
		if _, duplicate := seen[token]; duplicate {
			return feedbackLocalError("FEEDBACK_DRAFT_INVALID", "反馈草稿已失效，请重新添加图片。")
		}
		seen[token] = struct{}{}
		entry, ok := s.byToken[token]
		if !ok || entry.scope.ProfileID != profileID {
			return feedbackLocalError("FEEDBACK_DRAFT_INVALID", "反馈草稿已失效，请重新添加图片。")
		}
	}
	for token := range seen {
		entry := s.byToken[token]
		delete(s.byToken, token)
		s.totalBytes -= int64(len(entry.data))
	}
	return nil
}

func (s *feedbackImageStore) clearProfile(profileID string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for token, entry := range s.byToken {
		if entry.scope.ProfileID == profileID {
			delete(s.byToken, token)
			s.totalBytes -= int64(len(entry.data))
		}
	}
}

func (s *feedbackImageStore) removeAfterSubmit(scope cloud.BindingScope, tokens []string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, token := range tokens {
		entry, ok := s.byToken[token]
		if !ok || entry.scope != scope {
			continue
		}
		delete(s.byToken, token)
		s.totalBytes -= int64(len(entry.data))
	}
}

func (s *feedbackImageStore) clearAll() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	clear(s.byToken)
	s.totalBytes = 0
}

func newFeedbackImageToken() (string, error) {
	random := make([]byte, feedbackImageTokenRandomBytes)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return feedbackImageTokenPrefix + hex.EncodeToString(random), nil
}

func (a *App) ensureFeedbackImageStore() *feedbackImageStore {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.feedbackImages == nil {
		a.feedbackImages = newFeedbackImageStore()
	}
	return a.feedbackImages
}

func (a *App) clearFeedbackImagesForProfile(profileID string) {
	if a == nil {
		return
	}
	a.ensureFeedbackImageStore().clearProfile(profileID)
}

func (a *App) PickFeedbackImages(existingTokens []string) (FeedbackImageSelectionView, error) {
	if len(existingTokens) >= feedbackImageLimit {
		return FeedbackImageSelectionView{}, feedbackLocalError("FEEDBACK_IMAGE_LIMIT", "最多只能添加 4 张图片。")
	}
	originalScope, err := a.currentFeedbackScope()
	if err != nil {
		return FeedbackImageSelectionView{}, err
	}
	store := a.ensureFeedbackImageStore()
	if err := store.validateTokens(originalScope, existingTokens); err != nil {
		return FeedbackImageSelectionView{}, err
	}
	if a.ctx == nil {
		return FeedbackImageSelectionView{Images: []FeedbackImageView{}}, nil
	}
	paths, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择反馈图片",
		Filters: []runtime.FileFilter{
			{DisplayName: "图片（PNG、JPEG）", Pattern: "*.png;*.jpg;*.jpeg"},
		},
	})
	if err != nil {
		return FeedbackImageSelectionView{}, feedbackLocalError("FEEDBACK_IMAGE_PICK_FAILED", "无法打开图片选择器，请重试。")
	}
	if len(paths) == 0 {
		return FeedbackImageSelectionView{Images: []FeedbackImageView{}}, nil
	}
	if len(paths) > feedbackImageLimit-len(existingTokens) {
		return FeedbackImageSelectionView{}, feedbackLocalError("FEEDBACK_IMAGE_LIMIT", "最多只能添加 4 张图片。")
	}
	entries := make([]feedbackImageEntry, 0, len(paths))
	for _, path := range paths {
		entry, processErr := sanitizeFeedbackImageFile(path, originalScope)
		if processErr != nil {
			return FeedbackImageSelectionView{}, processErr
		}
		entries = append(entries, entry)
	}
	currentScope, err := a.currentFeedbackScope()
	if err != nil {
		return FeedbackImageSelectionView{}, err
	}
	if currentScope != originalScope {
		return FeedbackImageSelectionView{}, feedbackLocalError("FEEDBACK_DRAFT_INVALID", "登录账号已变化，请重新添加图片。")
	}
	views, err := store.add(currentScope, existingTokens, entries)
	if err != nil {
		return FeedbackImageSelectionView{}, err
	}
	return FeedbackImageSelectionView{Images: views}, nil
}

func (a *App) RemoveFeedbackImage(token string) error {
	if a == nil {
		return feedbackLocalError("FEEDBACK_UNAVAILABLE", "意见反馈暂不可用。")
	}
	return a.ensureFeedbackImageStore().removeForProfile(a.cloudActorContext().ProfileID, []string{token})
}

func (a *App) DiscardFeedbackDraft(tokens []string) error {
	if a == nil {
		return feedbackLocalError("FEEDBACK_UNAVAILABLE", "意见反馈暂不可用。")
	}
	return a.ensureFeedbackImageStore().removeForProfile(a.cloudActorContext().ProfileID, tokens)
}

func (a *App) SubmitFeedback(request FeedbackSubmitRequest) (FeedbackSubmitView, error) {
	runtimeService, actor, end, err := a.cloudRuntimeForActiveOwnerOperation()
	if err != nil {
		return feedbackFailure("feedback.submit.runtime", err), nil
	}
	defer end()
	if runtimeService.errorReports == nil {
		return feedbackFailure("feedback.submit.capability", feedbackLocalError("CLOUD_UNAVAILABLE", "意见反馈暂不可用。")), nil
	}
	scope, err := feedbackScopeForActor(a.bootContext(), runtimeService, actor)
	if err != nil {
		return feedbackFailure("feedback.submit.scope", err), nil
	}
	description, err := validateFeedbackDescription(request.Description)
	if err != nil {
		return feedbackFailure("feedback.submit.description", err), nil
	}
	attemptID, err := validateFeedbackAttemptID(request.AttemptID)
	if err != nil {
		return feedbackFailure("feedback.submit.attempt", err), nil
	}
	entries, err := a.ensureFeedbackImageStore().snapshot(scope, request.ImageTokens)
	if err != nil {
		return feedbackFailure("feedback.submit.images", err), nil
	}
	attachment, err := buildFeedbackAttachment(entries)
	if err != nil {
		return feedbackFailure("feedback.submit.attachment", err), nil
	}
	idempotencyKey, err := cloud.BuildIdempotencyKey(cloud.IdempotencyActionSubmitErrorReport, scope, attemptID)
	if err != nil {
		return feedbackFailure("feedback.submit.idempotency", feedbackLocalError("FEEDBACK_DRAFT_INVALID", "反馈草稿已失效，请重新提交。")), nil
	}
	ctx, cancel := context.WithTimeout(a.bootContext(), feedbackSubmitTimeout)
	defer cancel()
	submission, err := runtimeService.errorReports.SubmitErrorReport(ctx, scope, cloud.SubmitErrorReportInput{
		Description:    description,
		IdempotencyKey: idempotencyKey,
		Attachment:     attachment,
	})
	if err != nil {
		return feedbackFailure("feedback.submit.request", err), nil
	}
	if _, err := runtimeService.errorReports.GetErrorReport(ctx, scope, submission.ReportID); err != nil {
		return feedbackFailure("feedback.submit.readback", err), nil
	}
	a.ensureFeedbackImageStore().removeAfterSubmit(scope, request.ImageTokens)
	return FeedbackSubmitView{ReportID: submission.ReportID, Replayed: submission.Replayed}, nil
}

func (a *App) currentFeedbackScope() (cloud.BindingScope, error) {
	runtimeService, actor, end, err := a.cloudRuntimeForActiveOwnerOperation()
	if err != nil {
		return cloud.BindingScope{}, feedbackSafeError("feedback.scope.runtime", err)
	}
	defer end()
	if runtimeService.errorReports == nil {
		return cloud.BindingScope{}, feedbackLocalError("CLOUD_UNAVAILABLE", "意见反馈暂不可用。")
	}
	return feedbackScopeForActor(a.bootContext(), runtimeService, actor)
}

func feedbackScopeForActor(ctx context.Context, runtimeService *cloudRuntime, actor userctx.ActorContext) (cloud.BindingScope, error) {
	if runtimeService == nil || runtimeService.auth == nil || runtimeService.errorReports == nil {
		return cloud.BindingScope{}, feedbackLocalError("CLOUD_UNAVAILABLE", "意见反馈暂不可用。")
	}
	auth, err := runtimeService.auth.State(ctx, actor)
	if err != nil && auth.State != cloud.AccountOffline {
		return cloud.BindingScope{}, feedbackSafeError("feedback.scope.auth", err)
	}
	switch auth.State {
	case cloud.AccountAuthenticated, cloud.AccountOffline:
	default:
		return cloud.BindingScope{}, feedbackLocalError("AUTH_REQUIRED", "登录状态已失效，请重新登录。")
	}
	if auth.Binding.Status != cloud.BindingBound {
		return cloud.BindingScope{}, feedbackLocalError("AUTH_REQUIRED", "登录状态已失效，请重新登录。")
	}
	scope, err := auth.Binding.Scope()
	if err != nil {
		return cloud.BindingScope{}, feedbackLocalError("AUTH_REQUIRED", "登录状态已失效，请重新登录。")
	}
	return scope, nil
}

func validateFeedbackDescription(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", feedbackLocalError("FEEDBACK_DESCRIPTION_REQUIRED", "请填写反馈内容。")
	}
	if !utf8.ValidString(value) {
		return "", feedbackLocalError("FEEDBACK_DESCRIPTION_INVALID", "反馈内容包含无效字符。")
	}
	if utf8.RuneCountInString(value) > feedbackDescriptionMaxRunes {
		return "", feedbackLocalError("FEEDBACK_DESCRIPTION_TOO_LONG", "反馈内容不能超过 300 个字符。")
	}
	if len([]byte(value)) > cloud.ErrorReportDescriptionMaxBytes {
		return "", feedbackLocalError("FEEDBACK_DESCRIPTION_TOO_LONG", "反馈内容过长，请精简后重试。")
	}
	return value, nil
}

func validateFeedbackAttemptID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > feedbackAttemptIDMaxBytes {
		return "", feedbackLocalError("FEEDBACK_ATTEMPT_INVALID", "反馈提交标识无效，请重新提交。")
	}
	for _, char := range value {
		if !(char >= 'a' && char <= 'z') &&
			!(char >= 'A' && char <= 'Z') &&
			!(char >= '0' && char <= '9') &&
			!strings.ContainsRune("-_.:", char) {
			return "", feedbackLocalError("FEEDBACK_ATTEMPT_INVALID", "反馈提交标识无效，请重新提交。")
		}
	}
	return value, nil
}

func sanitizeFeedbackImageFile(path string, scope cloud.BindingScope) (feedbackImageEntry, error) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return feedbackImageEntry{}, feedbackLocalError("FEEDBACK_IMAGE_READ_FAILED", "无法读取所选图片，请重新选择。")
	}
	if info.Size() <= 0 {
		return feedbackImageEntry{}, feedbackLocalError("FEEDBACK_IMAGE_INVALID", "所选图片无效，请重新选择。")
	}
	if info.Size() > feedbackSelectedFileMaxBytes {
		return feedbackImageEntry{}, feedbackLocalError("FEEDBACK_IMAGE_TOO_LARGE", "单张图片不能超过 5 MiB。")
	}
	file, err := os.Open(path)
	if err != nil {
		return feedbackImageEntry{}, feedbackLocalError("FEEDBACK_IMAGE_READ_FAILED", "无法读取所选图片，请重新选择。")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, feedbackSelectedFileMaxBytes+1))
	if err != nil {
		return feedbackImageEntry{}, feedbackLocalError("FEEDBACK_IMAGE_READ_FAILED", "无法读取所选图片，请重新选择。")
	}
	if len(data) > feedbackSelectedFileMaxBytes {
		return feedbackImageEntry{}, feedbackLocalError("FEEDBACK_IMAGE_TOO_LARGE", "单张图片不能超过 5 MiB。")
	}
	return sanitizeFeedbackImage(filepath.Base(path), data, scope)
}

func sanitizeFeedbackImage(filename string, data []byte, scope cloud.BindingScope) (feedbackImageEntry, error) {
	extension := strings.ToLower(filepath.Ext(filename))
	expectedFormat := ""
	switch extension {
	case ".png":
		expectedFormat = "png"
	case ".jpg", ".jpeg":
		expectedFormat = "jpeg"
	default:
		return feedbackImageEntry{}, feedbackLocalError("FEEDBACK_IMAGE_FORMAT_UNSUPPORTED", "仅支持 PNG 或 JPEG 图片。")
	}
	if len(data) == 0 || len(data) > feedbackSelectedFileMaxBytes {
		return feedbackImageEntry{}, feedbackLocalError("FEEDBACK_IMAGE_TOO_LARGE", "单张图片不能超过 5 MiB。")
	}
	if expectedFormat == "png" && !bytes.HasPrefix(data, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		return feedbackImageEntry{}, feedbackLocalError("FEEDBACK_IMAGE_INVALID", "图片格式与文件名不一致。")
	}
	if expectedFormat == "jpeg" && !(len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff) {
		return feedbackImageEntry{}, feedbackLocalError("FEEDBACK_IMAGE_INVALID", "图片格式与文件名不一致。")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || format != expectedFormat {
		return feedbackImageEntry{}, feedbackLocalError("FEEDBACK_IMAGE_INVALID", "图片已损坏或格式不受支持。")
	}
	if config.Width <= 0 || config.Height <= 0 ||
		config.Width > feedbackImageMaxDimension || config.Height > feedbackImageMaxDimension ||
		int64(config.Width)*int64(config.Height) > feedbackImageMaxPixels {
		return feedbackImageEntry{}, feedbackLocalError("FEEDBACK_IMAGE_DIMENSIONS_EXCEEDED", "图片尺寸过大，请缩小后重试。")
	}
	decoded, format, err := image.Decode(bytes.NewReader(data))
	if err != nil || format != expectedFormat {
		return feedbackImageEntry{}, feedbackLocalError("FEEDBACK_IMAGE_INVALID", "图片已损坏或无法完整读取。")
	}
	var sanitized bytes.Buffer
	contentType := feedbackSinglePNGContentType
	safeExtension := ".png"
	if expectedFormat == "jpeg" {
		contentType = feedbackSingleJPEGContentType
		safeExtension = ".jpg"
		err = jpeg.Encode(&sanitized, decoded, &jpeg.Options{Quality: 95})
	} else {
		err = png.Encode(&sanitized, decoded)
	}
	if err != nil {
		return feedbackImageEntry{}, feedbackLocalError("FEEDBACK_IMAGE_PROCESS_FAILED", "图片处理失败，请重新选择。")
	}
	if sanitized.Len() > feedbackSelectedFileMaxBytes {
		return feedbackImageEntry{}, feedbackLocalError("FEEDBACK_IMAGE_TOO_LARGE", "处理后的单张图片不能超过 5 MiB。")
	}
	thumbnail, err := feedbackThumbnailDataURL(decoded)
	if err != nil {
		return feedbackImageEntry{}, err
	}
	return feedbackImageEntry{
		scope:            scope,
		filename:         safeFeedbackFilename(filename, safeExtension),
		contentType:      contentType,
		data:             append([]byte(nil), sanitized.Bytes()...),
		thumbnailDataURL: thumbnail,
	}, nil
}

func safeFeedbackFilename(filename, extension string) string {
	base := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	var cleaned strings.Builder
	for _, char := range base {
		if unicode.IsControl(char) || char == '/' || char == '\\' {
			continue
		}
		if unicode.IsSpace(char) {
			char = ' '
		}
		cleaned.WriteRune(char)
		if utf8.RuneCountInString(cleaned.String()) >= feedbackSafeFilenameMaxRunes {
			break
		}
	}
	result := strings.Trim(cleaned.String(), " .")
	if result == "" {
		result = "feedback-image"
	}
	return result + extension
}

func feedbackThumbnailDataURL(source image.Image) (string, error) {
	width, height := scaledFeedbackDimensions(source.Bounds().Dx(), source.Bounds().Dy(), feedbackThumbnailMaxDimension)
	for width >= 1 && height >= 1 {
		thumbnail := resizeFeedbackImage(source, width, height)
		for _, quality := range []int{85, 75, 65, 55} {
			var encoded bytes.Buffer
			if err := jpeg.Encode(&encoded, thumbnail, &jpeg.Options{Quality: quality}); err != nil {
				return "", feedbackLocalError("FEEDBACK_IMAGE_PROCESS_FAILED", "图片处理失败，请重新选择。")
			}
			dataURL := feedbackThumbnailJPEGDataURL + base64.StdEncoding.EncodeToString(encoded.Bytes())
			if len(dataURL) <= feedbackThumbnailMaxDataBytes {
				return dataURL, nil
			}
		}
		if width == 1 && height == 1 {
			break
		}
		width = max(1, width*3/4)
		height = max(1, height*3/4)
	}
	return "", feedbackLocalError("FEEDBACK_IMAGE_PROCESS_FAILED", "图片缩略图生成失败，请重新选择。")
}

func scaledFeedbackDimensions(width, height, maxDimension int) (int, int) {
	if width <= maxDimension && height <= maxDimension {
		return width, height
	}
	if width >= height {
		return maxDimension, max(1, height*maxDimension/width)
	}
	return max(1, width*maxDimension/height), maxDimension
}

func resizeFeedbackImage(source image.Image, width, height int) *image.NRGBA {
	target := image.NewNRGBA(image.Rect(0, 0, width, height))
	bounds := source.Bounds()
	sourceWidth, sourceHeight := bounds.Dx(), bounds.Dy()
	for y := 0; y < height; y++ {
		sourceY := bounds.Min.Y + y*sourceHeight/height
		for x := 0; x < width; x++ {
			sourceX := bounds.Min.X + x*sourceWidth/width
			target.SetNRGBA(x, y, flattenFeedbackColor(source.At(sourceX, sourceY)))
		}
	}
	return target
}

func flattenFeedbackColor(value color.Color) color.NRGBA {
	r, g, b, a := value.RGBA()
	return color.NRGBA{
		R: uint8((r + 0xffff - a) >> 8),
		G: uint8((g + 0xffff - a) >> 8),
		B: uint8((b + 0xffff - a) >> 8),
		A: 0xff,
	}
}

func buildFeedbackAttachment(entries []feedbackImageEntry) (*cloud.ErrorReportAttachmentInput, error) {
	switch len(entries) {
	case 0:
		return nil, nil
	case 1:
		entry := entries[0]
		return &cloud.ErrorReportAttachmentInput{
			Filename:    entry.filename,
			ContentType: entry.contentType,
			Data:        append([]byte(nil), entry.data...),
		}, nil
	}
	var payload bytes.Buffer
	writer := zip.NewWriter(&payload)
	for index, entry := range entries {
		extension := ".png"
		if entry.contentType == feedbackSingleJPEGContentType {
			extension = ".jpg"
		}
		header := &zip.FileHeader{
			Name:   fmt.Sprintf("image-%02d%s", index+1, extension),
			Method: feedbackDeterministicZIPMethod,
		}
		header.SetModTime(feedbackZIPTimestamp)
		header.SetMode(0o600)
		part, err := writer.CreateHeader(header)
		if err != nil {
			return nil, feedbackLocalError("FEEDBACK_IMAGE_PROCESS_FAILED", "图片附件生成失败，请重试。")
		}
		if _, err := part.Write(entry.data); err != nil {
			return nil, feedbackLocalError("FEEDBACK_IMAGE_PROCESS_FAILED", "图片附件生成失败，请重试。")
		}
	}
	if err := writer.Close(); err != nil {
		return nil, feedbackLocalError("FEEDBACK_IMAGE_PROCESS_FAILED", "图片附件生成失败，请重试。")
	}
	if payload.Len() > cloud.ErrorReportAttachmentMaxBytes {
		return nil, feedbackLocalError("FEEDBACK_IMAGE_TOTAL_TOO_LARGE", "图片总大小不能超过 20 MiB。")
	}
	return &cloud.ErrorReportAttachmentInput{
		Filename:    feedbackMultiAttachmentName,
		ContentType: feedbackMultiZIPContentType,
		Data:        append([]byte(nil), payload.Bytes()...),
	}, nil
}

func feedbackFailure(operation string, err error) FeedbackSubmitView {
	return FeedbackSubmitView{OperationError: feedbackOperationError(operation, err)}
}

func feedbackOperationError(operation string, err error) *FeedbackOperationErrorView {
	safe := feedbackSafeError(operation, err)
	var typed *cloud.Error
	if errors.As(safe, &typed) {
		return &FeedbackOperationErrorView{
			Code:         feedbackSafeErrorCode(typed.Code),
			Message:      typed.Message,
			Retryable:    typed.Retryable,
			RequestID:    feedbackSafeRequestID(typed.RequestID),
			DiagnosticID: cloudErrorDiagnosticID(safe),
		}
	}
	return &FeedbackOperationErrorView{
		Code:         cloud.ErrorCode("FEEDBACK_ERROR"),
		Message:      "反馈提交失败，请稍后重试。",
		DiagnosticID: cloudErrorDiagnosticID(safe),
	}
}

func feedbackSafeError(operation string, err error) error {
	if err == nil {
		return nil
	}
	var safe error
	if errors.Is(err, context.DeadlineExceeded) {
		safe = &cloud.Error{Code: "TIMEOUT", Message: "提交超时，请检查网络后重试。", Retryable: true}
	} else if errors.Is(err, context.Canceled) {
		safe = &cloud.Error{Code: "CANCELED", Message: "反馈提交已取消。"}
	} else {
		var typed *cloud.Error
		if !errors.As(err, &typed) {
			safe = &cloud.Error{Code: "FEEDBACK_ERROR", Message: "反馈提交失败，请稍后重试。"}
		} else {
			code := feedbackSafeErrorCode(typed.Code)
			message, retryable := feedbackErrorMessage(code, typed.Retryable)
			safe = &cloud.Error{
				Code:      code,
				Message:   message,
				Retryable: retryable,
				RequestID: feedbackSafeRequestID(typed.RequestID),
			}
		}
	}
	return &cloudDiagnosticError{err: safe, diagnosticID: recordCloudRequestFailure(operation, err)}
}

func feedbackErrorMessage(code cloud.ErrorCode, upstreamRetryable bool) (string, bool) {
	switch code {
	case "FEEDBACK_DESCRIPTION_REQUIRED":
		return "请填写反馈内容。", false
	case "FEEDBACK_DESCRIPTION_INVALID":
		return "反馈内容包含无效字符。", false
	case "FEEDBACK_DESCRIPTION_TOO_LONG":
		return "反馈内容不能超过 300 个字符。", false
	case "FEEDBACK_ATTEMPT_INVALID":
		return "反馈提交标识无效，请重新提交。", false
	case "FEEDBACK_IMAGE_LIMIT":
		return "最多只能添加 4 张图片。", false
	case "FEEDBACK_IMAGE_TOO_LARGE":
		return "单张图片不能超过 5 MiB。", false
	case "FEEDBACK_IMAGE_TOTAL_TOO_LARGE", "PAYLOAD_TOO_LARGE":
		return "图片总大小过大，请减少图片后重试。", false
	case "FEEDBACK_IMAGE_INVALID", "FEEDBACK_IMAGE_FORMAT_UNSUPPORTED", "FEEDBACK_IMAGE_DIMENSIONS_EXCEEDED", "FEEDBACK_DRAFT_INVALID":
		return "图片无效或草稿已失效，请重新添加图片。", false
	case "AUTH_REQUIRED", "TOKEN_EXPIRED", "REFRESH_TOKEN_INVALID", "REFRESH_TOKEN_EXPIRED", "REFRESH_TOKEN_REUSED", "DEVICE_MISMATCH", "ACCOUNT_DISABLED":
		return "登录状态已失效，请重新登录。", false
	case cloud.ErrorNetworkUnavailable:
		return "网络连接不可用，请检查网络后重试。", true
	case cloud.ErrorIdempotencyConflict:
		return "反馈内容已发生变化，请修改后重新提交。", false
	case "NOT_FOUND":
		return "云端未确认反馈已保存，请保留内容并重试。", true
	case "VALIDATION_FAILED":
		return "反馈内容无效，请检查后重试。", false
	case "RATE_LIMITED", "TOO_MANY_REQUESTS":
		return "提交过于频繁，请稍后重试。", true
	case "CLOUD_PROTOCOL_ERROR":
		return "云端返回异常，请稍后重试。", true
	case "CLOUD_UNAVAILABLE", "CAPABILITY_UNAVAILABLE", "UPSTREAM_ERROR", "INTERNAL_ERROR":
		return "意见反馈暂不可用，请稍后重试。", true
	case "TIMEOUT":
		return "提交超时，请检查网络后重试。", true
	case "CANCELED":
		return "反馈提交已取消。", false
	default:
		return "反馈提交失败，请稍后重试。", upstreamRetryable
	}
}

func feedbackSafeErrorCode(code cloud.ErrorCode) cloud.ErrorCode {
	value := strings.TrimSpace(string(code))
	if value == "" || len(value) > 64 {
		return "FEEDBACK_ERROR"
	}
	for _, char := range value {
		if !(char >= 'A' && char <= 'Z') && !(char >= '0' && char <= '9') && char != '_' && char != '-' {
			return "FEEDBACK_ERROR"
		}
	}
	return cloud.ErrorCode(value)
}

func feedbackSafeRequestID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 {
		return ""
	}
	for _, char := range value {
		if !(char >= 'a' && char <= 'z') &&
			!(char >= 'A' && char <= 'Z') &&
			!(char >= '0' && char <= '9') &&
			!strings.ContainsRune("-_.:", char) {
			return ""
		}
	}
	return value
}

func feedbackLocalError(code cloud.ErrorCode, message string) error {
	return &cloud.Error{Code: code, Message: message}
}

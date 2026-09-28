package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	cloud "kwu/internal/cloud"
	"kwu/internal/userctx"
)

func TestFeedbackSanitizeImagesRemovesMetadataAndBoundsThumbnail(t *testing.T) {
	t.Parallel()
	scope := testFeedbackScope("usr_image", 1)
	source := testFeedbackImage(640, 360)

	var pngInput bytes.Buffer
	if err := png.Encode(&pngInput, source); err != nil {
		t.Fatal(err)
	}
	pngEntry, err := sanitizeFeedbackImage("screen.png", pngInput.Bytes(), scope)
	if err != nil {
		t.Fatal(err)
	}
	if pngEntry.contentType != feedbackSinglePNGContentType || pngEntry.filename != "screen.png" {
		t.Fatalf("PNG entry = %#v", pngEntry)
	}
	if _, format, err := image.Decode(bytes.NewReader(pngEntry.data)); err != nil || format != "png" {
		t.Fatalf("sanitized PNG format=%q err=%v", format, err)
	}
	assertFeedbackThumbnail(t, pngEntry.thumbnailDataURL)

	var jpegInput bytes.Buffer
	if err := jpeg.Encode(&jpegInput, source, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	jpegWithMetadata := appendJPEGAPP1(jpegInput.Bytes(), []byte("Exif\x00\x00GPS-private-metadata"))
	jpegEntry, err := sanitizeFeedbackImage("camera.jpeg", jpegWithMetadata, scope)
	if err != nil {
		t.Fatal(err)
	}
	if jpegEntry.contentType != feedbackSingleJPEGContentType || jpegEntry.filename != "camera.jpg" {
		t.Fatalf("JPEG entry = %#v", jpegEntry)
	}
	if bytes.Contains(jpegEntry.data, []byte("Exif")) || bytes.Contains(jpegEntry.data, []byte("GPS-private-metadata")) {
		t.Fatal("sanitized JPEG retained metadata")
	}
	if _, format, err := image.Decode(bytes.NewReader(jpegEntry.data)); err != nil || format != "jpeg" {
		t.Fatalf("sanitized JPEG format=%q err=%v", format, err)
	}
	assertFeedbackThumbnail(t, jpegEntry.thumbnailDataURL)
}

func TestFeedbackSanitizeImagesRejectsUnsafeInputs(t *testing.T) {
	t.Parallel()
	scope := testFeedbackScope("usr_invalid", 1)
	var validPNG bytes.Buffer
	if err := png.Encode(&validPNG, testFeedbackImage(8, 8)); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, filename string
		data           []byte
		wantCode       cloud.ErrorCode
	}{
		{name: "unsupported extension", filename: "image.gif", data: validPNG.Bytes(), wantCode: "FEEDBACK_IMAGE_FORMAT_UNSUPPORTED"},
		{name: "signature mismatch", filename: "image.jpg", data: validPNG.Bytes(), wantCode: "FEEDBACK_IMAGE_INVALID"},
		{name: "truncated", filename: "image.png", data: validPNG.Bytes()[:20], wantCode: "FEEDBACK_IMAGE_INVALID"},
		{name: "empty", filename: "image.png", data: nil, wantCode: "FEEDBACK_IMAGE_TOO_LARGE"},
		{name: "file too large", filename: "image.png", data: make([]byte, feedbackSelectedFileMaxBytes+1), wantCode: "FEEDBACK_IMAGE_TOO_LARGE"},
		{name: "dimension", filename: "image.png", data: pngConfigOnly(8193, 1), wantCode: "FEEDBACK_IMAGE_DIMENSIONS_EXCEEDED"},
		{name: "pixels", filename: "image.png", data: pngConfigOnly(5001, 5000), wantCode: "FEEDBACK_IMAGE_DIMENSIONS_EXCEEDED"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := sanitizeFeedbackImage(test.filename, test.data, scope)
			assertCloudErrorCode(t, err, test.wantCode)
		})
	}
}

func TestFeedbackImageStoreEnforcesScopeCountTotalAndLifecycle(t *testing.T) {
	t.Parallel()
	scope := testFeedbackScope("usr_store", 1)
	otherScope := testFeedbackScope("usr_other", 2)
	store := newFeedbackImageStore()
	entry := testFeedbackEntry(scope, "one.png", feedbackSinglePNGContentType, []byte("one"))
	views, err := store.add(scope, nil, []feedbackImageEntry{entry})
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 1 || !strings.HasPrefix(views[0].Token, feedbackImageTokenPrefix) {
		t.Fatalf("views = %#v", views)
	}
	token := views[0].Token
	if _, err := store.snapshot(scope, []string{token}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.snapshot(scope, []string{token, token}); err == nil {
		t.Fatal("duplicate token was accepted")
	}
	if _, err := store.snapshot(otherScope, []string{token}); err == nil {
		t.Fatal("cross-scope token was accepted")
	}
	if _, err := store.add(scope, []string{token}, []feedbackImageEntry{
		testFeedbackEntry(scope, "2.png", feedbackSinglePNGContentType, []byte("2")),
		testFeedbackEntry(scope, "3.png", feedbackSinglePNGContentType, []byte("3")),
		testFeedbackEntry(scope, "4.png", feedbackSinglePNGContentType, []byte("4")),
		testFeedbackEntry(scope, "5.png", feedbackSinglePNGContentType, []byte("5")),
	}); err == nil {
		t.Fatal("more than four images were accepted")
	}
	if len(store.byToken) != 1 || store.totalBytes != int64(len(entry.data)) {
		t.Fatalf("failed add changed store: count=%d bytes=%d", len(store.byToken), store.totalBytes)
	}
	if err := store.removeForProfile(scope.ProfileID, []string{token}); err != nil {
		t.Fatal(err)
	}
	if len(store.byToken) != 0 || store.totalBytes != 0 {
		t.Fatalf("remove left count=%d bytes=%d", len(store.byToken), store.totalBytes)
	}

	largeStore := newFeedbackImageStore()
	largeEntries := make([]feedbackImageEntry, feedbackImageLimit)
	perEntry := feedbackImageStoreMaxBytes/feedbackImageLimit + 2
	for index := range largeEntries {
		largeEntries[index] = testFeedbackEntry(scope, "large.png", feedbackSinglePNGContentType, make([]byte, perEntry))
	}
	if _, err := largeStore.add(scope, nil, largeEntries); err == nil {
		t.Fatal("over-limit total bytes were accepted")
	}
	if len(largeStore.byToken) != 0 || largeStore.totalBytes != 0 {
		t.Fatal("over-limit add was not atomic")
	}
}

func TestFeedbackAttachmentSingleAndMultiAreSafeAndDeterministic(t *testing.T) {
	t.Parallel()
	scope := testFeedbackScope("usr_zip", 1)
	pngEntry := testFeedbackEntry(scope, "screen.png", feedbackSinglePNGContentType, []byte("png-data"))
	jpegEntry := testFeedbackEntry(scope, "photo.jpg", feedbackSingleJPEGContentType, []byte("jpeg-data"))

	single, err := buildFeedbackAttachment([]feedbackImageEntry{pngEntry})
	if err != nil {
		t.Fatal(err)
	}
	if single.Filename != "screen.png" || single.ContentType != feedbackSinglePNGContentType || !bytes.Equal(single.Data, pngEntry.data) {
		t.Fatalf("single attachment = %#v", single)
	}
	first, err := buildFeedbackAttachment([]feedbackImageEntry{pngEntry, jpegEntry})
	if err != nil {
		t.Fatal(err)
	}
	second, err := buildFeedbackAttachment([]feedbackImageEntry{pngEntry, jpegEntry})
	if err != nil {
		t.Fatal(err)
	}
	if first.Filename != feedbackMultiAttachmentName || first.ContentType != feedbackMultiZIPContentType || !bytes.Equal(first.Data, second.Data) {
		t.Fatal("multi-image ZIP is not deterministic")
	}
	reader, err := zip.NewReader(bytes.NewReader(first.Data), int64(len(first.Data)))
	if err != nil {
		t.Fatal(err)
	}
	if len(reader.File) != 2 || reader.File[0].Name != "image-01.png" || reader.File[1].Name != "image-02.jpg" {
		t.Fatalf("ZIP entries = %#v", reader.File)
	}
	for index, want := range [][]byte{pngEntry.data, jpegEntry.data} {
		part, err := reader.File[index].Open()
		if err != nil {
			t.Fatal(err)
		}
		got, readErr := io.ReadAll(part)
		_ = part.Close()
		if readErr != nil || !bytes.Equal(got, want) {
			t.Fatalf("ZIP entry %d = %q err=%v", index, got, readErr)
		}
		if !reader.File[index].Modified.Equal(feedbackZIPTimestamp) {
			t.Fatalf("ZIP entry %d timestamp = %v", index, reader.File[index].Modified)
		}
	}
}

func TestSubmitFeedbackMapsRequestDeadlineSuccessAndFailureLifecycle(t *testing.T) {
	t.Parallel()
	client := &recordingFeedbackClient{
		result: cloud.ErrorReportSubmission{ReportID: "errpt_test", Status: cloud.ErrorReportStatusSubmitted},
	}
	app, scope := newFeedbackTestApp(t, client)
	entry := testFeedbackEntry(scope, "screen.png", feedbackSinglePNGContentType, []byte("sanitized-image"))
	views, err := app.feedbackImages.add(scope, nil, []feedbackImageEntry{entry})
	if err != nil {
		t.Fatal(err)
	}
	request := FeedbackSubmitRequest{
		Description: "  应用导出时闪退  ",
		AttemptID:   "attempt-001",
		ImageTokens: []string{views[0].Token},
	}
	result, err := app.SubmitFeedback(request)
	if err != nil {
		t.Fatal(err)
	}
	if result.OperationError != nil || result.ReportID != "errpt_test" {
		t.Fatalf("result = %#v", result)
	}
	call := client.lastCall()
	wantKey, err := cloud.BuildIdempotencyKey(cloud.IdempotencyActionSubmitErrorReport, scope, request.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	if call.scope != scope || call.input.Description != "应用导出时闪退" || call.input.IdempotencyKey != wantKey {
		t.Fatalf("submit call = %#v", call)
	}
	if call.input.Attachment == nil || call.input.Attachment.Filename != "screen.png" || !bytes.Equal(call.input.Attachment.Data, entry.data) {
		t.Fatalf("submit attachment = %#v", call.input.Attachment)
	}
	if call.deadlineRemaining < 115*time.Second || call.deadlineRemaining > feedbackSubmitTimeout {
		t.Fatalf("deadline remaining = %v", call.deadlineRemaining)
	}
	if client.getCallCount() != 1 {
		t.Fatalf("successful submit query calls = %d, want 1", client.getCallCount())
	}
	query := client.lastGetCall()
	if query.scope != scope || query.reportID != "errpt_test" ||
		query.deadlineRemaining <= 0 || query.deadlineRemaining > feedbackSubmitTimeout {
		t.Fatalf("confirmation query = %#v", query)
	}
	if _, err := app.feedbackImages.snapshot(scope, request.ImageTokens); err == nil {
		t.Fatal("successful submit retained image tokens")
	}

	concurrentClient := &recordingFeedbackClient{
		result: cloud.ErrorReportSubmission{ReportID: "errpt_concurrent", Status: cloud.ErrorReportStatusSubmitted},
	}
	app.cloudRuntime.errorReports = concurrentClient
	concurrentViews, err := app.feedbackImages.add(scope, nil, []feedbackImageEntry{entry})
	if err != nil {
		t.Fatal(err)
	}
	concurrentTokens := []string{concurrentViews[0].Token}
	concurrentClient.beforeReturn = func() {
		_ = app.feedbackImages.removeForProfile(scope.ProfileID, concurrentTokens)
	}
	concurrentResult, err := app.SubmitFeedback(FeedbackSubmitRequest{
		Description: "提交时删除图片", AttemptID: "attempt-concurrent", ImageTokens: concurrentTokens,
	})
	if err != nil {
		t.Fatal(err)
	}
	if concurrentResult.ReportID != "errpt_concurrent" || concurrentResult.OperationError != nil {
		t.Fatalf("concurrent delete result = %#v", concurrentResult)
	}

	failureClient := &recordingFeedbackClient{err: &cloud.Error{
		Code: cloud.ErrorNetworkUnavailable, Message: "secret C:\\private\\file.png",
		Retryable: true, RequestID: "req_safe-01",
	}}
	app.cloudRuntime.errorReports = failureClient
	failureViews, err := app.feedbackImages.add(scope, nil, []feedbackImageEntry{entry})
	if err != nil {
		t.Fatal(err)
	}
	failureTokens := []string{failureViews[0].Token}
	failure, err := app.SubmitFeedback(FeedbackSubmitRequest{
		Description: "网络失败", AttemptID: "attempt-002", ImageTokens: failureTokens,
	})
	if err != nil {
		t.Fatal(err)
	}
	if failure.OperationError == nil ||
		failure.OperationError.Code != cloud.ErrorNetworkUnavailable ||
		failure.OperationError.Message != "网络连接不可用，请检查网络后重试。" ||
		!failure.OperationError.Retryable ||
		failure.OperationError.RequestID != "req_safe-01" {
		t.Fatalf("failure = %#v", failure)
	}
	if strings.Contains(failure.OperationError.Message, "secret") || strings.Contains(failure.OperationError.Message, "private") {
		t.Fatalf("failure leaked internal detail: %#v", failure.OperationError)
	}
	if _, err := app.feedbackImages.snapshot(scope, failureTokens); err != nil {
		t.Fatalf("failed submit did not retain draft: %v", err)
	}

	confirmationClient := &recordingFeedbackClient{
		result: cloud.ErrorReportSubmission{ReportID: "errpt_unconfirmed", Status: cloud.ErrorReportStatusSubmitted},
		getErr: &cloud.Error{
			Code: "NOT_FOUND", Message: "not found", RequestID: "req-query-not-found",
		},
	}
	app.cloudRuntime.errorReports = confirmationClient
	confirmationViews, err := app.feedbackImages.add(scope, nil, []feedbackImageEntry{entry})
	if err != nil {
		t.Fatal(err)
	}
	confirmationTokens := []string{confirmationViews[0].Token}
	unconfirmed, err := app.SubmitFeedback(FeedbackSubmitRequest{
		Description: "等待云端确认", AttemptID: "attempt-unconfirmed", ImageTokens: confirmationTokens,
	})
	if err != nil {
		t.Fatal(err)
	}
	if unconfirmed.ReportID != "" || unconfirmed.OperationError == nil ||
		unconfirmed.OperationError.Code != "NOT_FOUND" ||
		unconfirmed.OperationError.Message != "云端未确认反馈已保存，请保留内容并重试。" ||
		!unconfirmed.OperationError.Retryable ||
		unconfirmed.OperationError.RequestID != "req-query-not-found" {
		t.Fatalf("unconfirmed result = %#v", unconfirmed)
	}
	if _, err := app.feedbackImages.snapshot(scope, confirmationTokens); err != nil {
		t.Fatalf("unconfirmed submit did not retain draft: %v", err)
	}
}

func TestSubmitFeedbackSupportsTextOnlyAndFixtureFailsClosed(t *testing.T) {
	t.Parallel()
	client := &recordingFeedbackClient{
		result: cloud.ErrorReportSubmission{ReportID: "errpt_text", Status: cloud.ErrorReportStatusSubmitted, Replayed: true},
	}
	app, _ := newFeedbackTestApp(t, client)
	result, err := app.SubmitFeedback(FeedbackSubmitRequest{Description: "只有文字", AttemptID: "attempt-text"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ReportID != "errpt_text" || !result.Replayed || result.OperationError != nil {
		t.Fatalf("text result = %#v", result)
	}
	if call := client.lastCall(); call.input.Attachment != nil {
		t.Fatalf("text-only attachment = %#v", call.input.Attachment)
	}

	app.cloudRuntime.errorReports = nil
	unavailable, err := app.SubmitFeedback(FeedbackSubmitRequest{Description: "不可用", AttemptID: "attempt-disabled"})
	if err != nil {
		t.Fatal(err)
	}
	if unavailable.OperationError == nil || unavailable.OperationError.Code != "CLOUD_UNAVAILABLE" || unavailable.ReportID != "" {
		t.Fatalf("fixture fail-closed result = %#v", unavailable)
	}
}

func TestFeedbackFixtureRuntimeDoesNotProvideFakeSuccessClient(t *testing.T) {
	t.Parallel()
	runtimeService, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtimeService.close)
	if runtimeService.errorReports != nil {
		t.Fatal("fixture runtime unexpectedly provides an error report client")
	}
}

func TestSubmitFeedbackValidatesChineseUserInputsBeforeNetwork(t *testing.T) {
	t.Parallel()
	client := &recordingFeedbackClient{
		result: cloud.ErrorReportSubmission{ReportID: "unexpected", Status: cloud.ErrorReportStatusSubmitted},
	}
	app, _ := newFeedbackTestApp(t, client)
	tests := []struct {
		name    string
		request FeedbackSubmitRequest
		code    cloud.ErrorCode
		message string
	}{
		{name: "empty", request: FeedbackSubmitRequest{Description: " ", AttemptID: "attempt"}, code: "FEEDBACK_DESCRIPTION_REQUIRED", message: "请填写反馈内容。"},
		{name: "too long", request: FeedbackSubmitRequest{Description: strings.Repeat("界", 301), AttemptID: "attempt"}, code: "FEEDBACK_DESCRIPTION_TOO_LONG", message: "反馈内容不能超过 300 个字符。"},
		{name: "attempt", request: FeedbackSubmitRequest{Description: "内容", AttemptID: "含空格 id"}, code: "FEEDBACK_ATTEMPT_INVALID", message: "反馈提交标识无效，请重新提交。"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			result, err := app.SubmitFeedback(test.request)
			if err != nil {
				t.Fatal(err)
			}
			if result.OperationError == nil || result.OperationError.Code != test.code || result.OperationError.Message != test.message {
				t.Fatalf("result = %#v", result)
			}
		})
	}
	if client.callCount() != 0 {
		t.Fatalf("invalid inputs made %d network calls", client.callCount())
	}
}

func TestFeedbackSuccessfulLogoutAndAccountSwitchClearDraftImages(t *testing.T) {
	runtimeService, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtimeService.close)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	auth, err := runtimeService.auth.Login(context.Background(), actor, cloud.LoginInput{
		Phone: "13800000000", Password: "test-password-01",
	})
	if err != nil {
		t.Fatal(err)
	}
	scope, err := auth.Binding.Scope()
	if err != nil {
		t.Fatal(err)
	}
	runtimeService.errorReports = &recordingFeedbackClient{}
	app := &App{
		cloudRuntime:         runtimeService,
		currentActor:         actor,
		currentActorExplicit: true,
		tabRuntimes:          map[string]*WorkspaceTab{},
		feedbackImages:       newFeedbackImageStore(),
	}
	addDraft := func() string {
		views, addErr := app.feedbackImages.add(scope, nil, []feedbackImageEntry{
			testFeedbackEntry(scope, "draft.png", feedbackSinglePNGContentType, []byte("draft")),
		})
		if addErr != nil {
			t.Fatal(addErr)
		}
		return views[0].Token
	}
	logoutToken := addDraft()
	if _, err := app.CloudLogout(); err != nil {
		t.Fatal(err)
	}
	if _, err := app.feedbackImages.snapshot(scope, []string{logoutToken}); err == nil {
		t.Fatal("successful logout retained feedback draft")
	}

	auth, err = runtimeService.auth.Login(context.Background(), actor, cloud.LoginInput{
		Phone: "13800000000", Password: "test-password-01",
	})
	if err != nil {
		t.Fatal(err)
	}
	scope, err = auth.Binding.Scope()
	if err != nil {
		t.Fatal(err)
	}
	switchToken := addDraft()
	if _, err := app.CloudSwitchAccount(CloudLoginRequest{
		Phone: "13900000000", Password: "test-password-02",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := app.feedbackImages.snapshot(scope, []string{switchToken}); err == nil {
		t.Fatal("successful account switch retained feedback draft")
	}
}

func TestFeedbackShutdownClearsAllDraftImages(t *testing.T) {
	t.Parallel()
	scope := testFeedbackScope("usr_shutdown", 1)
	store := newFeedbackImageStore()
	if _, err := store.add(scope, nil, []feedbackImageEntry{
		testFeedbackEntry(scope, "draft.png", feedbackSinglePNGContentType, []byte("draft")),
	}); err != nil {
		t.Fatal(err)
	}
	app := &App{feedbackImages: store}
	app.shutdown(context.Background())
	if len(store.byToken) != 0 || store.totalBytes != 0 {
		t.Fatalf("shutdown left count=%d bytes=%d", len(store.byToken), store.totalBytes)
	}
}

type feedbackClientCall struct {
	scope             cloud.BindingScope
	input             cloud.SubmitErrorReportInput
	deadlineRemaining time.Duration
}

type feedbackClientGetCall struct {
	scope             cloud.BindingScope
	reportID          string
	deadlineRemaining time.Duration
}

type recordingFeedbackClient struct {
	mu           sync.Mutex
	result       cloud.ErrorReportSubmission
	err          error
	confirmation cloud.ErrorReportConfirmation
	getErr       error
	calls        []feedbackClientCall
	getCalls     []feedbackClientGetCall
	beforeReturn func()
}

func (c *recordingFeedbackClient) SubmitErrorReport(ctx context.Context, scope cloud.BindingScope, input cloud.SubmitErrorReportInput) (cloud.ErrorReportSubmission, error) {
	call := feedbackClientCall{scope: scope, input: input}
	if input.Attachment != nil {
		attachment := *input.Attachment
		attachment.Data = append([]byte(nil), input.Attachment.Data...)
		call.input.Attachment = &attachment
	}
	if deadline, ok := ctx.Deadline(); ok {
		call.deadlineRemaining = time.Until(deadline)
	}
	c.mu.Lock()
	c.calls = append(c.calls, call)
	c.mu.Unlock()
	if c.beforeReturn != nil {
		c.beforeReturn()
	}
	return c.result, c.err
}

func (c *recordingFeedbackClient) GetErrorReport(ctx context.Context, scope cloud.BindingScope, reportID string) (cloud.ErrorReportConfirmation, error) {
	call := feedbackClientGetCall{scope: scope, reportID: reportID}
	if deadline, ok := ctx.Deadline(); ok {
		call.deadlineRemaining = time.Until(deadline)
	}
	c.mu.Lock()
	c.getCalls = append(c.getCalls, call)
	c.mu.Unlock()
	if c.getErr != nil {
		return cloud.ErrorReportConfirmation{}, c.getErr
	}
	if c.confirmation.ReportID != "" {
		return c.confirmation, nil
	}
	return cloud.ErrorReportConfirmation{
		ReportID: reportID,
		Status:   cloud.ErrorReportStatusSubmitted,
	}, nil
}

func (c *recordingFeedbackClient) lastCall() feedbackClientCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls[len(c.calls)-1]
}

func (c *recordingFeedbackClient) lastGetCall() feedbackClientGetCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.getCalls[len(c.getCalls)-1]
}

func (c *recordingFeedbackClient) callCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.calls)
}

func (c *recordingFeedbackClient) getCallCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.getCalls)
}

func newFeedbackTestApp(t *testing.T, client cloud.ErrorReportClient) (*App, cloud.BindingScope) {
	t.Helper()
	runtimeService, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtimeService.close)
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "default"}
	auth, err := runtimeService.auth.Login(context.Background(), actor, cloud.LoginInput{
		Phone: "13800000000", Password: "test-password-01",
	})
	if err != nil {
		t.Fatal(err)
	}
	scope, err := auth.Binding.Scope()
	if err != nil {
		t.Fatal(err)
	}
	runtimeService.errorReports = client
	return &App{
		cloudRuntime:         runtimeService,
		currentActor:         actor,
		currentActorExplicit: true,
		tabRuntimes:          map[string]*WorkspaceTab{},
		feedbackImages:       newFeedbackImageStore(),
	}, scope
}

func testFeedbackScope(userID string, generation uint64) cloud.BindingScope {
	return cloud.BindingScope{ProfileID: "default", CloudUserID: userID, BindingGeneration: generation}
}

func testFeedbackEntry(scope cloud.BindingScope, filename, contentType string, data []byte) feedbackImageEntry {
	return feedbackImageEntry{
		scope:            scope,
		filename:         filename,
		contentType:      contentType,
		data:             append([]byte(nil), data...),
		thumbnailDataURL: feedbackThumbnailJPEGDataURL + base64.StdEncoding.EncodeToString([]byte("thumbnail")),
	}
}

func testFeedbackImage(width, height int) *image.NRGBA {
	result := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			result.SetNRGBA(x, y, color.NRGBA{
				R: uint8(x % 251),
				G: uint8(y % 241),
				B: uint8((x + y) % 239),
				A: uint8(160 + (x+y)%96),
			})
		}
	}
	return result
}

func appendJPEGAPP1(data, metadata []byte) []byte {
	result := make([]byte, 0, len(data)+len(metadata)+4)
	result = append(result, data[:2]...)
	result = append(result, 0xff, 0xe1)
	length := len(metadata) + 2
	result = append(result, byte(length>>8), byte(length))
	result = append(result, metadata...)
	result = append(result, data[2:]...)
	return result
}

func pngConfigOnly(width, height uint32) []byte {
	var result bytes.Buffer
	result.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
	var header bytes.Buffer
	_ = binary.Write(&header, binary.BigEndian, width)
	_ = binary.Write(&header, binary.BigEndian, height)
	header.Write([]byte{8, 6, 0, 0, 0})
	writePNGChunk(&result, "IHDR", header.Bytes())
	writePNGChunk(&result, "IEND", nil)
	return result.Bytes()
}

func writePNGChunk(target *bytes.Buffer, chunkType string, data []byte) {
	_ = binary.Write(target, binary.BigEndian, uint32(len(data)))
	target.WriteString(chunkType)
	target.Write(data)
	checksum := crc32.NewIEEE()
	_, _ = checksum.Write([]byte(chunkType))
	_, _ = checksum.Write(data)
	_ = binary.Write(target, binary.BigEndian, checksum.Sum32())
}

func assertFeedbackThumbnail(t *testing.T, dataURL string) {
	t.Helper()
	if !strings.HasPrefix(dataURL, feedbackThumbnailJPEGDataURL) || len(dataURL) > feedbackThumbnailMaxDataBytes {
		t.Fatalf("thumbnail length=%d prefix=%t", len(dataURL), strings.HasPrefix(dataURL, feedbackThumbnailJPEGDataURL))
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(dataURL, feedbackThumbnailJPEGDataURL))
	if err != nil {
		t.Fatal(err)
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || format != "jpeg" {
		t.Fatalf("thumbnail format=%q err=%v", format, err)
	}
	if config.Width > feedbackThumbnailMaxDimension || config.Height > feedbackThumbnailMaxDimension {
		t.Fatalf("thumbnail dimensions=%dx%d", config.Width, config.Height)
	}
}

func assertCloudErrorCode(t *testing.T, err error, want cloud.ErrorCode) {
	t.Helper()
	var typed *cloud.Error
	if !errors.As(err, &typed) || typed.Code != want {
		t.Fatalf("error = %v, want code %s", err, want)
	}
}

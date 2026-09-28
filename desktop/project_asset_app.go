package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"kwu/internal/project"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const projectAssetMaxFileBytes = 100 << 20
const projectAssetDownloadFilenameMaxBytes = 180
const projectAssetDownloadExtensionMaxBytes = 32

var projectAssetUploadSemaphore = make(chan struct{}, 1)

func acquireProjectAssetUpload(ctx context.Context) error {
	select {
	case projectAssetUploadSemaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func releaseProjectAssetUpload() { <-projectAssetUploadSemaphore }

type CloudProjectAssetView struct {
	AssetID     string   `json:"assetId"`
	ProjectID   string   `json:"projectId"`
	Filename    string   `json:"filename"`
	ContentType string   `json:"contentType"`
	SizeBytes   int64    `json:"sizeBytes"`
	SHA256      string   `json:"sha256"`
	Version     uint64   `json:"version"`
	WorkItemIDs []string `json:"workItemIds"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
}

type CloudProjectAssetListView struct {
	ProjectScopeKey string                  `json:"projectScopeKey"`
	ProjectID       string                  `json:"projectId"`
	Items           []CloudProjectAssetView `json:"items"`
}

type CloudProjectAssetDirectoryInput struct {
	ProjectID string `json:"projectId"`
	FolderID  string `json:"folderId"`
	Search    string `json:"search"`
	Type      string `json:"type"`
	Sort      string `json:"sort"`
	Direction string `json:"direction"`
	Cursor    string `json:"cursor"`
	Limit     int    `json:"limit"`
}

type CloudProjectAssetDirectoryUpdatedByView struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
}

type CloudProjectAssetDirectoryBreadcrumbView struct {
	FolderID string `json:"folderId"`
	Name     string `json:"name"`
}

type CloudProjectAssetDirectoryItemView struct {
	Kind           string                                  `json:"kind"`
	ID             string                                  `json:"id"`
	Name           string                                  `json:"name"`
	FolderID       *string                                 `json:"folderId"`
	ParentFolderID *string                                 `json:"parentFolderId"`
	ContentType    *string                                 `json:"contentType"`
	SizeBytes      *int64                                  `json:"sizeBytes"`
	UpdatedBy      CloudProjectAssetDirectoryUpdatedByView `json:"updatedBy"`
	Version        uint64                                  `json:"version"`
	UpdatedAt      string                                  `json:"updatedAt"`
}

type CloudProjectAssetDirectoryPageView struct {
	ProjectScopeKey string                                     `json:"projectScopeKey"`
	ProjectID       string                                     `json:"projectId"`
	Breadcrumbs     []CloudProjectAssetDirectoryBreadcrumbView `json:"breadcrumbs"`
	Items           []CloudProjectAssetDirectoryItemView       `json:"items"`
	TotalCount      uint64                                     `json:"totalCount"`
	NextCursor      *string                                    `json:"nextCursor"`
	UsageBytes      int64                                      `json:"usageBytes"`
	QuotaBytes      int64                                      `json:"quotaBytes"`
	QuotaPlan       string                                     `json:"quotaPlan"`
}

type ListCloudProjectAssetOperationRecordsInput struct {
	ProjectID    string `json:"projectId"`
	Operation    string `json:"operation"`
	ResourceKind string `json:"resourceKind"`
	ActorUserID  string `json:"actorUserId"`
	Limit        int    `json:"limit"`
}

type CloudProjectAssetOperationRecordView struct {
	OperationID      string         `json:"operationId"`
	ProjectID        string         `json:"projectId"`
	ResourceKind     string         `json:"resourceKind"`
	ResourceID       string         `json:"resourceId"`
	ResourceName     string         `json:"resourceName"`
	Operation        string         `json:"operation"`
	OperationDetail  string         `json:"operationDetail"`
	ActorUserID      string         `json:"actorUserId"`
	ActorDisplayName string         `json:"actorDisplayName"`
	ResourceVersion  *uint64        `json:"resourceVersion"`
	FolderID         *string        `json:"folderId"`
	Details          map[string]any `json:"details"`
	CreatedAt        string         `json:"createdAt"`
}

type CloudProjectAssetOperationRecordPageView struct {
	ProjectScopeKey string                                 `json:"projectScopeKey"`
	ProjectID       string                                 `json:"projectId"`
	Items           []CloudProjectAssetOperationRecordView `json:"items"`
	Limit           int                                    `json:"limit"`
}

type CreateCloudProjectAssetFolderInput struct {
	ProjectID      string  `json:"projectId"`
	Name           string  `json:"name"`
	ParentFolderID *string `json:"parentFolderId"`
}

type PatchCloudProjectAssetFolderInput struct {
	ProjectID         string  `json:"projectId"`
	FolderID          string  `json:"folderId"`
	ExpectedVersion   uint64  `json:"expectedVersion"`
	Name              *string `json:"name"`
	ParentFolderIDSet bool    `json:"parentFolderIdSet"`
	ParentFolderID    *string `json:"parentFolderId"`
}

type PatchCloudProjectAssetInput struct {
	ProjectID       string  `json:"projectId"`
	AssetID         string  `json:"assetId"`
	ExpectedVersion uint64  `json:"expectedVersion"`
	Name            *string `json:"name"`
	FolderIDSet     bool    `json:"folderIdSet"`
	FolderID        *string `json:"folderId"`
}

type UploadCloudProjectAssetInput struct {
	ProjectID string  `json:"projectId"`
	FolderID  *string `json:"folderId"`
}

type ReplaceCloudProjectAssetContentInput struct {
	ProjectID       string  `json:"projectId"`
	AssetID         string  `json:"assetId"`
	ExpectedVersion uint64  `json:"expectedVersion"`
	FolderID        *string `json:"folderId"`
}

type DeleteCloudProjectAssetFolderInput struct {
	ProjectID       string `json:"projectId"`
	FolderID        string `json:"folderId"`
	ExpectedVersion uint64 `json:"expectedVersion"`
}

type ListCloudProjectAssetProcessingStatesInput struct {
	ProjectID string   `json:"projectId"`
	AssetIDs  []string `json:"assetIds"`
}

type CloudProjectAssetDownloadView struct {
	ProjectScopeKey string `json:"projectScopeKey"`
	ProjectID       string `json:"projectId"`
	AssetID         string `json:"assetId"`
	Filename        string `json:"filename"`
	URL             string `json:"url"`
	ExpiresAt       string `json:"expiresAt"`
	SavedPath       string `json:"savedPath"`
}

type CloudProjectAssetProcessingStateView struct {
	AssetID         string  `json:"assetId"`
	AssetVersionID  string  `json:"assetVersionId"`
	VersionNumber   uint64  `json:"versionNumber"`
	ScanStatus      string  `json:"scanStatus"`
	ParseStatus     string  `json:"parseStatus"`
	IndexStatus     string  `json:"indexStatus"`
	PurgeStatus     string  `json:"purgeStatus"`
	StateVersion    uint64  `json:"stateVersion"`
	ScanAttempts    uint64  `json:"scanAttempts"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
	ScanCompletedAt *string `json:"scanCompletedAt"`
	PurgedAt        *string `json:"purgedAt"`
}

type CloudProjectAssetProcessingStateListView struct {
	ProjectScopeKey string                                 `json:"projectScopeKey"`
	ProjectID       string                                 `json:"projectId"`
	Items           []CloudProjectAssetProcessingStateView `json:"items"`
}

type MutateCloudProjectWorkItemAssetInput struct {
	ProjectID  string `json:"projectId"`
	WorkItemID string `json:"workItemId"`
	AssetID    string `json:"assetId"`
}

func (a *App) ListCloudProjectAssets(projectID string) (CloudProjectAssetListView, error) {
	services, err := a.projectServicesForAssetOperation()
	if err != nil {
		return CloudProjectAssetListView{}, projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	scopeKey, items, err := services.Gateway.ListAssets(ctx, projectID)
	if err != nil {
		return CloudProjectAssetListView{}, projectProtocolWailsError(err)
	}
	return cloudProjectAssetListView(scopeKey, projectID, items), nil
}

func (a *App) ListCloudProjectAssetDirectory(input CloudProjectAssetDirectoryInput) (CloudProjectAssetDirectoryPageView, error) {
	services, err := a.projectServicesForAssetOperation()
	if err != nil {
		return CloudProjectAssetDirectoryPageView{}, projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	scopeKey, page, err := services.Gateway.ListAssetDirectory(ctx, input.ProjectID, project.DirectoryQuery{
		FolderID: input.FolderID, Search: input.Search, Type: input.Type, Sort: input.Sort,
		Direction: input.Direction, Cursor: input.Cursor, Limit: input.Limit,
	})
	if err != nil {
		return CloudProjectAssetDirectoryPageView{}, projectProtocolWailsError(err)
	}
	return cloudProjectAssetDirectoryPageView(scopeKey, input.ProjectID, page), nil
}

func (a *App) ListCloudProjectAssetOperationRecords(input ListCloudProjectAssetOperationRecordsInput) (CloudProjectAssetOperationRecordPageView, error) {
	services, err := a.projectServicesForAssetOperation()
	if err != nil {
		return CloudProjectAssetOperationRecordPageView{}, projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	scopeKey, page, err := services.Gateway.ListAssetOperationRecords(ctx, input.ProjectID, project.AssetOperationRecordsQuery{
		Operation: input.Operation, ResourceKind: input.ResourceKind, ActorUserID: input.ActorUserID, Limit: input.Limit,
	})
	if err != nil {
		return CloudProjectAssetOperationRecordPageView{}, projectProtocolWailsError(err)
	}
	return cloudProjectAssetOperationRecordPageView(scopeKey, page), nil
}

func (a *App) CreateCloudProjectAssetFolder(input CreateCloudProjectAssetFolderInput) (CloudProjectAssetDirectoryPageView, error) {
	services, err := a.projectServicesForAssetOperation()
	if err != nil {
		return CloudProjectAssetDirectoryPageView{}, projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	if _, err := services.Gateway.CreateAssetFolder(ctx, input.ProjectID, project.CreateProjectAssetFolderInput{Name: input.Name, ParentFolderID: input.ParentFolderID}); err != nil {
		return CloudProjectAssetDirectoryPageView{}, projectProtocolWailsError(err)
	}
	return a.ListCloudProjectAssetDirectory(CloudProjectAssetDirectoryInput{ProjectID: input.ProjectID, FolderID: optionalProjectAssetID(input.ParentFolderID)})
}

func (a *App) PatchCloudProjectAssetFolder(input PatchCloudProjectAssetFolderInput) (CloudProjectAssetDirectoryPageView, error) {
	services, err := a.projectServicesForAssetOperation()
	if err != nil {
		return CloudProjectAssetDirectoryPageView{}, projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	patch := project.PatchProjectAssetFolderInput{
		ExpectedVersion: input.ExpectedVersion, Name: input.Name,
		ParentFolderID: project.NullableStringUpdate{Set: input.ParentFolderIDSet, Value: input.ParentFolderID},
	}
	if _, err := services.Gateway.PatchAssetFolder(ctx, input.ProjectID, input.FolderID, patch); err != nil {
		return CloudProjectAssetDirectoryPageView{}, projectProtocolWailsError(err)
	}
	return a.ListCloudProjectAssetDirectory(CloudProjectAssetDirectoryInput{ProjectID: input.ProjectID, FolderID: optionalProjectAssetID(input.ParentFolderID)})
}

func (a *App) DeleteCloudProjectAssetFolder(input DeleteCloudProjectAssetFolderInput) (CloudProjectAssetDirectoryPageView, error) {
	services, err := a.projectServicesForAssetOperation()
	if err != nil {
		return CloudProjectAssetDirectoryPageView{}, projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	if _, err := services.Gateway.DeleteAssetFolder(ctx, input.ProjectID, input.FolderID, input.ExpectedVersion); err != nil {
		return CloudProjectAssetDirectoryPageView{}, projectProtocolWailsError(err)
	}
	return a.ListCloudProjectAssetDirectory(CloudProjectAssetDirectoryInput{ProjectID: input.ProjectID})
}

func (a *App) PatchCloudProjectAsset(input PatchCloudProjectAssetInput) (CloudProjectAssetDirectoryPageView, error) {
	services, err := a.projectServicesForAssetOperation()
	if err != nil {
		return CloudProjectAssetDirectoryPageView{}, projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	patch := project.PatchProjectAssetInput{
		ExpectedVersion: input.ExpectedVersion, Name: input.Name,
		FolderID: project.NullableStringUpdate{Set: input.FolderIDSet, Value: input.FolderID},
	}
	if _, err := services.Gateway.PatchAsset(ctx, input.ProjectID, input.AssetID, patch); err != nil {
		return CloudProjectAssetDirectoryPageView{}, projectProtocolWailsError(err)
	}
	return a.ListCloudProjectAssetDirectory(CloudProjectAssetDirectoryInput{ProjectID: input.ProjectID, FolderID: optionalProjectAssetID(input.FolderID)})
}

// ListCloudProjectAssetProcessingStates keeps operational version state on a
// separate read path so the established asset-list DTO remains compatible.
func (a *App) ListCloudProjectAssetProcessingStates(input ListCloudProjectAssetProcessingStatesInput) (CloudProjectAssetProcessingStateListView, error) {
	services, err := a.projectServicesForAssetOperation()
	if err != nil {
		return CloudProjectAssetProcessingStateListView{}, projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	scopeKey, items, err := services.Gateway.ListAssetProcessingStates(ctx, input.ProjectID, input.AssetIDs)
	if err != nil {
		return CloudProjectAssetProcessingStateListView{}, projectProtocolWailsError(err)
	}
	return cloudProjectAssetProcessingStateListView(scopeKey, input.ProjectID, items), nil
}

// UploadCloudProjectAsset owns the native picker so browser code never receives a local path.
func (a *App) UploadCloudProjectAsset(projectID string) (CloudProjectAssetListView, error) {
	services, err := a.projectServicesForAssetOperation()
	if err != nil {
		return CloudProjectAssetListView{}, projectWailsError(err)
	}
	if a.ctx == nil {
		return CloudProjectAssetListView{}, projectLocalWailsError(errors.New("native file picker is unavailable"), "PROJECT_ASSET_UNAVAILABLE", "asset upload requires the desktop application")
	}
	path, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{Title: "选择项目文件", DefaultDirectory: dialogDefaultDirectory(a.activeWorkspaceRoot())})
	if err != nil {
		return CloudProjectAssetListView{}, projectLocalWailsError(err, "PROJECT_ASSET_UNAVAILABLE", "asset file could not be selected")
	}
	if strings.TrimSpace(path) == "" {
		return a.ListCloudProjectAssets(projectID)
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > projectAssetMaxFileBytes {
		if err == nil {
			err = errors.New("asset must be a non-empty regular file no larger than 100 MiB")
		}
		return CloudProjectAssetListView{}, projectLocalWailsError(err, "PROJECT_ASSET_INVALID", "asset file must be non-empty and no larger than 100 MiB")
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	if err := acquireProjectAssetUpload(ctx); err != nil {
		return CloudProjectAssetListView{}, projectProtocolWailsError(err)
	}
	defer releaseProjectAssetUpload()
	_, _, err = services.Gateway.UploadAssetFileToFolder(ctx, projectID, path, filepath.Base(path), nil)
	if err != nil {
		return CloudProjectAssetListView{}, projectProtocolWailsError(err)
	}
	return a.ListCloudProjectAssets(projectID)
}

// UploadCloudProjectAssetToFolder keeps the native path inside Desktop and
// returns the new directory projection without changing the legacy upload API.
func (a *App) UploadCloudProjectAssetToFolder(input UploadCloudProjectAssetInput) (CloudProjectAssetDirectoryPageView, error) {
	services, err := a.projectServicesForAssetOperation()
	if err != nil {
		return CloudProjectAssetDirectoryPageView{}, projectWailsError(err)
	}
	if a.ctx == nil {
		return CloudProjectAssetDirectoryPageView{}, projectLocalWailsError(errors.New("native file picker is unavailable"), "PROJECT_ASSET_UNAVAILABLE", "asset upload requires the desktop application")
	}
	path, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{Title: "Select project file", DefaultDirectory: dialogDefaultDirectory(a.activeWorkspaceRoot())})
	if err != nil {
		return CloudProjectAssetDirectoryPageView{}, projectLocalWailsError(err, "PROJECT_ASSET_UNAVAILABLE", "asset file could not be selected")
	}
	if strings.TrimSpace(path) == "" {
		return a.ListCloudProjectAssetDirectory(CloudProjectAssetDirectoryInput{ProjectID: input.ProjectID, FolderID: optionalProjectAssetID(input.FolderID)})
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > projectAssetMaxFileBytes {
		if err == nil {
			err = errors.New("asset must be a non-empty regular file no larger than 100 MiB")
		}
		return CloudProjectAssetDirectoryPageView{}, projectLocalWailsError(err, "PROJECT_ASSET_INVALID", "asset file must be non-empty and no larger than 100 MiB")
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	if err := acquireProjectAssetUpload(ctx); err != nil {
		return CloudProjectAssetDirectoryPageView{}, projectProtocolWailsError(err)
	}
	defer releaseProjectAssetUpload()
	_, _, err = services.Gateway.UploadAssetFileToFolder(ctx, input.ProjectID, path, filepath.Base(path), input.FolderID)
	if err != nil {
		return CloudProjectAssetDirectoryPageView{}, projectProtocolWailsError(err)
	}
	return a.ListCloudProjectAssetDirectory(CloudProjectAssetDirectoryInput{ProjectID: input.ProjectID, FolderID: optionalProjectAssetID(input.FolderID)})
}

// ReplaceCloudProjectAssetContent selects a local file without exposing its
// path to the webview and creates a new Cloud asset version.
func (a *App) ReplaceCloudProjectAssetContent(input ReplaceCloudProjectAssetContentInput) (CloudProjectAssetDirectoryPageView, error) {
	services, err := a.projectServicesForAssetOperation()
	if err != nil {
		return CloudProjectAssetDirectoryPageView{}, projectWailsError(err)
	}
	if a.ctx == nil {
		return CloudProjectAssetDirectoryPageView{}, projectLocalWailsError(errors.New("native file picker is unavailable"), "PROJECT_ASSET_UNAVAILABLE", "asset replacement requires the desktop application")
	}
	path, err := wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{Title: "选择替换文件", DefaultDirectory: dialogDefaultDirectory(a.activeWorkspaceRoot())})
	if err != nil {
		return CloudProjectAssetDirectoryPageView{}, projectLocalWailsError(err, "PROJECT_ASSET_UNAVAILABLE", "replacement file could not be selected")
	}
	if strings.TrimSpace(path) == "" {
		return a.ListCloudProjectAssetDirectory(CloudProjectAssetDirectoryInput{ProjectID: input.ProjectID, FolderID: optionalProjectAssetID(input.FolderID)})
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > projectAssetMaxFileBytes {
		if err == nil {
			err = errors.New("asset must be a non-empty regular file no larger than 100 MiB")
		}
		return CloudProjectAssetDirectoryPageView{}, projectLocalWailsError(err, "PROJECT_ASSET_INVALID", "replacement file must be non-empty and no larger than 100 MiB")
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	if err := acquireProjectAssetUpload(ctx); err != nil {
		return CloudProjectAssetDirectoryPageView{}, projectProtocolWailsError(err)
	}
	defer releaseProjectAssetUpload()
	if _, _, err = services.Gateway.ReplaceAssetFileContent(ctx, input.ProjectID, input.AssetID, path, filepath.Base(path), input.ExpectedVersion); err != nil {
		return CloudProjectAssetDirectoryPageView{}, projectProtocolWailsError(err)
	}
	return a.ListCloudProjectAssetDirectory(CloudProjectAssetDirectoryInput{ProjectID: input.ProjectID, FolderID: optionalProjectAssetID(input.FolderID)})
}

func (a *App) DownloadCloudProjectAsset(projectID, assetID string) (CloudProjectAssetDownloadView, error) {
	services, err := a.projectServicesForAssetOperation()
	if err != nil {
		return CloudProjectAssetDownloadView{}, projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	scopeKey, item, err := services.Gateway.AssetDownload(ctx, projectID, assetID)
	if err != nil {
		return CloudProjectAssetDownloadView{}, projectProtocolWailsError(err)
	}
	workspaceRoot, err := ensureGlobalWorkspaceRoot()
	if err != nil {
		return CloudProjectAssetDownloadView{}, projectLocalWailsError(err, "PROJECT_ASSET_DOWNLOAD_FAILED", "global workspace is unavailable")
	}
	client, err := newPublicCloudHTTPClient(item.URL)
	if err != nil {
		return CloudProjectAssetDownloadView{}, projectLocalWailsError(err, "PROJECT_ASSET_DOWNLOAD_FAILED", "asset download could not be started")
	}
	savedPath, err := downloadProjectAssetFile(ctx, client, workspaceRoot, item.Filename, item.URL)
	if err != nil {
		return CloudProjectAssetDownloadView{}, projectLocalWailsError(err, "PROJECT_ASSET_DOWNLOAD_FAILED", "asset could not be downloaded to the global workspace")
	}
	return CloudProjectAssetDownloadView{ProjectScopeKey: scopeKey, ProjectID: projectID, AssetID: item.AssetID, Filename: filepath.Base(savedPath), URL: item.URL, ExpiresAt: item.ExpiresAt.UTC().Format(time.RFC3339Nano), SavedPath: savedPath}, nil
}

func downloadProjectAssetFile(ctx context.Context, client *http.Client, directory, filename, rawURL string) (string, error) {
	if client == nil {
		return "", errors.New("asset download HTTP client is unavailable")
	}
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return "", errors.New("asset download URL is invalid")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", errors.New("asset download request could not be created")
	}
	response, err := client.Do(request)
	if err != nil {
		return "", errors.New("asset download request failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("asset download returned HTTP status %d", response.StatusCode)
	}
	if response.ContentLength > projectAssetMaxFileBytes {
		return "", errors.New("asset download exceeds the maximum file size")
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", fmt.Errorf("create asset destination: %w", err)
	}
	destination, destinationPath, err := createProjectAssetDownloadDestination(directory, safeProjectAssetDownloadFilename(filename))
	if err != nil {
		return "", err
	}
	keep := false
	defer func() {
		_ = destination.Close()
		if !keep {
			_ = os.Remove(destinationPath)
		}
	}()
	written, err := io.Copy(destination, io.LimitReader(response.Body, projectAssetMaxFileBytes+1))
	if err != nil {
		return "", fmt.Errorf("write asset download: %w", err)
	}
	if written > projectAssetMaxFileBytes {
		return "", errors.New("asset download exceeds the maximum file size")
	}
	if response.ContentLength >= 0 && written != response.ContentLength {
		return "", errors.New("asset download size does not match the response")
	}
	if err := destination.Sync(); err != nil {
		return "", fmt.Errorf("sync asset download: %w", err)
	}
	if err := destination.Close(); err != nil {
		return "", fmt.Errorf("close asset download: %w", err)
	}
	keep = true
	return destinationPath, nil
}

func createProjectAssetDownloadDestination(directory, filename string) (*os.File, string, error) {
	stem, extension := splitProjectAssetDownloadFilename(filename)
	for suffix := 0; suffix < 10_000; suffix++ {
		conflictSuffix := ""
		if suffix != 0 {
			conflictSuffix = fmt.Sprintf(" (%d)", suffix)
		}
		candidate := fitProjectAssetDownloadFilename(stem, conflictSuffix, extension)
		candidatePath := filepath.Join(directory, candidate)
		file, err := os.OpenFile(candidatePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			return file, candidatePath, nil
		}
		if !os.IsExist(err) {
			return nil, "", fmt.Errorf("create asset destination: %w", err)
		}
	}
	return nil, "", errors.New("asset destination has too many filename conflicts")
}

func safeProjectAssetDownloadFilename(filename string) string {
	filename = path.Base(strings.ReplaceAll(strings.TrimSpace(filename), "\\", "/"))
	filename = strings.Map(func(value rune) rune {
		if value < 32 || strings.ContainsRune(`<>:"/\|?*`, value) {
			return '_'
		}
		return value
	}, filename)
	filename = strings.TrimRight(filename, ". ")
	if filename == "" || filename == "." || filename == ".." {
		return "download"
	}
	extension := filepath.Ext(filename)
	stem := strings.TrimRight(strings.TrimSuffix(filename, extension), ". ")
	if isWindowsReservedProjectAssetFilenameStem(stem) {
		filename = "_" + filename
	}
	stem, extension = splitProjectAssetDownloadFilename(filename)
	return fitProjectAssetDownloadFilename(stem, "", extension)
}

func isWindowsReservedProjectAssetFilenameStem(stem string) bool {
	stem = strings.ToUpper(stem)
	if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" {
		return true
	}
	suffix, found := strings.CutPrefix(stem, "COM")
	if !found {
		suffix, found = strings.CutPrefix(stem, "LPT")
	}
	if !found {
		return false
	}
	switch suffix {
	case "1", "2", "3", "4", "5", "6", "7", "8", "9", "¹", "²", "³":
		return true
	default:
		return false
	}
}

func splitProjectAssetDownloadFilename(filename string) (string, string) {
	extension := filepath.Ext(filename)
	stem := strings.TrimSuffix(filename, extension)
	if stem == "" || len(extension) > projectAssetDownloadExtensionMaxBytes {
		return filename, ""
	}
	return stem, extension
}

func fitProjectAssetDownloadFilename(stem, suffix, extension string) string {
	budget := projectAssetDownloadFilenameMaxBytes - len(suffix) - len(extension)
	if budget < len("download") {
		extension = ""
		budget = projectAssetDownloadFilenameMaxBytes - len(suffix)
	}
	stem = truncateProjectAssetFilenameUTF8(stem, budget)
	if stem == "" {
		stem = "download"
	}
	return stem + suffix + extension
}

func truncateProjectAssetFilenameUTF8(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	end := 0
	for end < len(value) {
		_, size := utf8.DecodeRuneInString(value[end:])
		if end+size > maxBytes {
			break
		}
		end += size
	}
	return value[:end]
}

func (a *App) DeleteCloudProjectAsset(projectID, assetID string) (CloudProjectAssetListView, error) {
	services, err := a.projectServicesForAssetOperation()
	if err != nil {
		return CloudProjectAssetListView{}, projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	if _, err := services.Gateway.DeleteAsset(ctx, projectID, assetID); err != nil {
		return CloudProjectAssetListView{}, projectProtocolWailsError(err)
	}
	return a.ListCloudProjectAssets(projectID)
}

func (a *App) PutCloudProjectWorkItemAsset(input MutateCloudProjectWorkItemAssetInput) (CloudProjectAssetView, error) {
	return a.mutateCloudProjectWorkItemAsset(input, true)
}

func (a *App) DeleteCloudProjectWorkItemAsset(input MutateCloudProjectWorkItemAssetInput) (CloudProjectAssetView, error) {
	return a.mutateCloudProjectWorkItemAsset(input, false)
}

func (a *App) mutateCloudProjectWorkItemAsset(input MutateCloudProjectWorkItemAssetInput, put bool) (CloudProjectAssetView, error) {
	services, err := a.projectServicesForAssetOperation()
	if err != nil {
		return CloudProjectAssetView{}, projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	_, item, err := services.Gateway.MutateWorkItemAsset(ctx, input.ProjectID, input.WorkItemID, input.AssetID, put)
	if err != nil {
		return CloudProjectAssetView{}, projectProtocolWailsError(err)
	}
	return cloudProjectAssetView(item), nil
}

func cloudProjectAssetListView(scopeKey, projectID string, source []project.ProjectAsset) CloudProjectAssetListView {
	items := make([]CloudProjectAssetView, len(source))
	for index, item := range source {
		items[index] = cloudProjectAssetView(item)
	}
	return CloudProjectAssetListView{ProjectScopeKey: scopeKey, ProjectID: projectID, Items: items}
}

func cloudProjectAssetView(item project.ProjectAsset) CloudProjectAssetView {
	refs := append([]string(nil), item.WorkItemIDs...)
	if refs == nil {
		refs = []string{}
	}
	return CloudProjectAssetView{AssetID: item.AssetID, ProjectID: item.ProjectID, Filename: item.Filename, ContentType: item.ContentType, SizeBytes: item.SizeBytes, SHA256: item.SHA256, Version: item.Version, WorkItemIDs: refs, CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339Nano)}
}

func cloudProjectAssetDirectoryPageView(scopeKey, projectID string, source project.ProjectAssetDirectoryPage) CloudProjectAssetDirectoryPageView {
	breadcrumbs := make([]CloudProjectAssetDirectoryBreadcrumbView, len(source.Breadcrumbs))
	for index, item := range source.Breadcrumbs {
		breadcrumbs[index] = CloudProjectAssetDirectoryBreadcrumbView{FolderID: item.FolderID, Name: item.Name}
	}
	items := make([]CloudProjectAssetDirectoryItemView, len(source.Items))
	for index, item := range source.Items {
		items[index] = CloudProjectAssetDirectoryItemView{
			Kind: item.Kind, ID: item.ID, Name: item.Name,
			FolderID: cloneProjectAssetString(item.FolderID), ParentFolderID: cloneProjectAssetString(item.ParentFolderID),
			ContentType: cloneProjectAssetString(item.ContentType), SizeBytes: cloneProjectAssetInt64(item.SizeBytes),
			UpdatedBy: CloudProjectAssetDirectoryUpdatedByView{UserID: item.UpdatedBy.UserID, DisplayName: item.UpdatedBy.DisplayName},
			Version:   item.Version, UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339Nano),
		}
	}
	return CloudProjectAssetDirectoryPageView{
		ProjectScopeKey: scopeKey, ProjectID: projectID, Breadcrumbs: breadcrumbs, Items: items,
		TotalCount: source.TotalCount, NextCursor: cloneProjectAssetString(source.NextCursor),
		UsageBytes: source.UsageBytes, QuotaBytes: source.QuotaBytes, QuotaPlan: source.QuotaPlan,
	}
}

func cloudProjectAssetOperationRecordPageView(scopeKey string, source project.ProjectAssetOperationRecordPage) CloudProjectAssetOperationRecordPageView {
	items := make([]CloudProjectAssetOperationRecordView, len(source.Items))
	for index, item := range source.Items {
		details := make(map[string]any, len(item.Details))
		for key, value := range item.Details {
			details[key] = value
		}
		items[index] = CloudProjectAssetOperationRecordView{
			OperationID: item.OperationID, ProjectID: item.ProjectID, ResourceKind: item.ResourceKind,
			ResourceID: item.ResourceID, ResourceName: item.ResourceName, Operation: item.Operation,
			OperationDetail: item.OperationDetail, ActorUserID: item.ActorUserID, ActorDisplayName: item.ActorDisplayName,
			ResourceVersion: cloneProjectAssetUint64(item.ResourceVersion), FolderID: cloneProjectAssetString(item.FolderID),
			Details: details, CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339Nano),
		}
	}
	return CloudProjectAssetOperationRecordPageView{ProjectScopeKey: scopeKey, ProjectID: source.ProjectID, Items: items, Limit: source.Limit}
}

func optionalProjectAssetID(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func cloneProjectAssetString(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneProjectAssetInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneProjectAssetUint64(value *uint64) *uint64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloudProjectAssetProcessingStateListView(scopeKey, projectID string, source []project.ProjectAssetProcessingState) CloudProjectAssetProcessingStateListView {
	items := make([]CloudProjectAssetProcessingStateView, len(source))
	for index, item := range source {
		items[index] = CloudProjectAssetProcessingStateView{
			AssetID: item.AssetID, AssetVersionID: item.AssetVersionID, VersionNumber: item.VersionNumber,
			ScanStatus: item.ScanStatus, ParseStatus: item.ParseStatus, IndexStatus: item.IndexStatus, PurgeStatus: item.PurgeStatus,
			StateVersion: item.StateVersion, ScanAttempts: item.ScanAttempts,
			CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339Nano),
			ScanCompletedAt: optionalProjectAssetTime(item.ScanCompletedAt), PurgedAt: optionalProjectAssetTime(item.PurgedAt),
		}
	}
	return CloudProjectAssetProcessingStateListView{ProjectScopeKey: scopeKey, ProjectID: projectID, Items: items}
}

func optionalProjectAssetTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339Nano)
	return &formatted
}

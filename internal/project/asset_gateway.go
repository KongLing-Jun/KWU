package project

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type projectMultipartTransport interface {
	DoAuthenticatedMultipartJSON(context.Context, string, string, []byte, string, string, http.Header, any) error
}

type projectMultipartStreamTransport interface {
	DoAuthenticatedMultipartStreamJSON(context.Context, string, string, func(context.Context) (io.ReadCloser, string, error), string, http.Header, any) error
}

type ProjectAsset struct {
	AssetID     string    `json:"asset_id"`
	ProjectID   string    `json:"project_id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	SHA256      string    `json:"sha256"`
	Version     uint64    `json:"version"`
	WorkItemIDs []string  `json:"work_item_ids"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ProjectAssetDownload struct {
	AssetID   string    `json:"asset_id"`
	Filename  string    `json:"filename"`
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

type DirectoryQuery struct {
	FolderID  string
	Search    string
	Type      string
	Sort      string
	Direction string
	Cursor    string
	Limit     int
}

type ProjectAssetDirectoryUpdatedBy struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
}

type ProjectAssetDirectoryBreadcrumb struct {
	FolderID string `json:"folder_id"`
	Name     string `json:"name"`
}

type ProjectAssetDirectoryItem struct {
	Kind           string                         `json:"kind"`
	ID             string                         `json:"id"`
	Name           string                         `json:"name"`
	FolderID       *string                        `json:"folder_id"`
	ParentFolderID *string                        `json:"parent_folder_id"`
	ContentType    *string                        `json:"content_type"`
	SizeBytes      *int64                         `json:"size_bytes"`
	UpdatedBy      ProjectAssetDirectoryUpdatedBy `json:"updated_by"`
	Version        uint64                         `json:"version"`
	UpdatedAt      time.Time                      `json:"updated_at"`
}

type ProjectAssetDirectoryPage struct {
	ProjectID   string                            `json:"project_id"`
	FolderID    *string                           `json:"folder_id"`
	Breadcrumbs []ProjectAssetDirectoryBreadcrumb `json:"breadcrumbs"`
	Items       []ProjectAssetDirectoryItem       `json:"items"`
	TotalCount  uint64                            `json:"total_count"`
	NextCursor  *string                           `json:"next_cursor"`
	UsageBytes  int64                             `json:"usage_bytes"`
	QuotaBytes  int64                             `json:"quota_bytes"`
	QuotaPlan   string                            `json:"quota_plan"`
}

type CreateProjectAssetFolderInput struct {
	Name           string
	ParentFolderID *string
}

type PatchProjectAssetFolderInput struct {
	ExpectedVersion uint64
	Name            *string
	ParentFolderID  NullableStringUpdate
}

type PatchProjectAssetInput struct {
	ExpectedVersion uint64
	Name            *string
	FolderID        NullableStringUpdate
}

// ProjectAssetProcessingState is the content-free operational projection for
// one Cloud asset version. It deliberately excludes filenames and object keys.
type ProjectAssetProcessingState struct {
	AssetID         string     `json:"asset_id"`
	AssetVersionID  string     `json:"asset_version_id"`
	VersionNumber   uint64     `json:"version_number"`
	ScanStatus      string     `json:"scan_status"`
	ParseStatus     string     `json:"parse_status"`
	IndexStatus     string     `json:"index_status"`
	PurgeStatus     string     `json:"purge_status"`
	StateVersion    uint64     `json:"state_version"`
	ScanAttempts    uint64     `json:"scan_attempts"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	ScanCompletedAt *time.Time `json:"scan_completed_at"`
	PurgedAt        *time.Time `json:"purged_at"`
}

func (g *ProjectGateway) ListAssets(ctx context.Context, projectID string) (string, []ProjectAsset, error) {
	if err := validateOpaqueProjectID(projectID); err != nil {
		return "", nil, err
	}
	scope, err := g.currentScope(ctx)
	if err != nil {
		return "", nil, err
	}
	var payload json.RawMessage
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return g.transport.DoAuthenticatedJSON(callCtx, http.MethodGet, projectPath(projectID)+"/assets", nil, token, nil, &payload)
	})
	if err != nil {
		return "", nil, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return "", nil, err
	}
	var items []ProjectAsset
	if err := decodeV2Payload(payload, &items); err != nil {
		return "", nil, err
	}
	if err := validateProjectAssets(items, projectID); err != nil {
		return "", nil, err
	}
	return scope.Key, items, nil
}

func (g *ProjectGateway) ListAssetDirectory(ctx context.Context, projectID string, input DirectoryQuery) (string, ProjectAssetDirectoryPage, error) {
	if err := validateOpaqueProjectID(projectID); err != nil {
		return "", ProjectAssetDirectoryPage{}, err
	}
	query, err := projectAssetDirectoryQuery(input)
	if err != nil {
		return "", ProjectAssetDirectoryPage{}, err
	}
	scope, err := g.currentScope(ctx)
	if err != nil {
		return "", ProjectAssetDirectoryPage{}, err
	}
	var payload json.RawMessage
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return g.transport.DoAuthenticatedJSONQuery(callCtx, http.MethodGet, projectPath(projectID)+"/asset-directory", query, nil, token, nil, &payload)
	})
	if err != nil {
		return "", ProjectAssetDirectoryPage{}, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return "", ProjectAssetDirectoryPage{}, err
	}
	page, err := decodeProjectAssetDirectoryPage(payload, projectID, input.FolderID)
	if err != nil {
		return "", ProjectAssetDirectoryPage{}, err
	}
	return scope.Key, page, nil
}

func (g *ProjectGateway) CreateAssetFolder(ctx context.Context, projectID string, input CreateProjectAssetFolderInput) (string, error) {
	if err := validateOpaqueProjectID(projectID); err != nil {
		return "", err
	}
	if err := validateProjectAssetName(input.Name); err != nil {
		return "", err
	}
	body := map[string]any{"name": input.Name, "parent_folder_id": input.ParentFolderID}
	if input.ParentFolderID != nil {
		if err := validateOpaqueProjectID(*input.ParentFolderID); err != nil {
			return "", err
		}
	}
	return g.mutateAssetDirectory(ctx, http.MethodPost, projectPath(projectID)+"/asset-folders", body)
}

func (g *ProjectGateway) PatchAssetFolder(ctx context.Context, projectID, folderID string, input PatchProjectAssetFolderInput) (string, error) {
	if err := validateAssetDirectoryMutation(projectID, folderID, input.ExpectedVersion); err != nil {
		return "", err
	}
	if input.Name == nil && !input.ParentFolderID.Set {
		return "", fmt.Errorf("projects-v2: asset folder patch requires a mutation")
	}
	body := map[string]any{"expected_version": input.ExpectedVersion}
	if input.Name != nil {
		if err := validateProjectAssetName(*input.Name); err != nil {
			return "", err
		}
		body["name"] = *input.Name
	}
	if input.ParentFolderID.Set {
		if input.ParentFolderID.Value != nil {
			if err := validateOpaqueProjectID(*input.ParentFolderID.Value); err != nil {
				return "", err
			}
			if *input.ParentFolderID.Value == folderID {
				return "", fmt.Errorf("projects-v2: asset folder cannot be its own parent")
			}
		}
		body["parent_folder_id"] = input.ParentFolderID.Value
	}
	return g.mutateAssetDirectory(ctx, http.MethodPatch, assetFolderPath(projectID, folderID), body)
}

func (g *ProjectGateway) DeleteAssetFolder(ctx context.Context, projectID, folderID string, expectedVersion uint64) (string, error) {
	if err := validateAssetDirectoryMutation(projectID, folderID, expectedVersion); err != nil {
		return "", err
	}
	return g.mutateAssetDirectory(ctx, http.MethodDelete, assetFolderPath(projectID, folderID), map[string]any{"expected_version": expectedVersion})
}

func (g *ProjectGateway) PatchAsset(ctx context.Context, projectID, assetID string, input PatchProjectAssetInput) (string, error) {
	if err := validateAssetDirectoryMutation(projectID, assetID, input.ExpectedVersion); err != nil {
		return "", err
	}
	if input.Name == nil && !input.FolderID.Set {
		return "", fmt.Errorf("projects-v2: asset patch requires a mutation")
	}
	body := map[string]any{"expected_version": input.ExpectedVersion}
	if input.Name != nil {
		if err := validateProjectAssetName(*input.Name); err != nil {
			return "", err
		}
		body["name"] = *input.Name
	}
	if input.FolderID.Set {
		if input.FolderID.Value != nil {
			if err := validateOpaqueProjectID(*input.FolderID.Value); err != nil {
				return "", err
			}
		}
		body["folder_id"] = input.FolderID.Value
	}
	return g.mutateAssetDirectory(ctx, http.MethodPatch, assetPath(projectID, assetID), body)
}

func (g *ProjectGateway) mutateAssetDirectory(ctx context.Context, method, path string, body any) (string, error) {
	scope, err := g.currentScope(ctx)
	if err != nil {
		return "", err
	}
	var payload json.RawMessage
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return g.transport.DoAuthenticatedJSON(callCtx, method, path, body, token, nil, &payload)
	})
	if err != nil {
		return "", err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return "", err
	}
	return scope.Key, nil
}

// ListAssetProcessingStates reads the dedicated version-state projection
// without changing the existing /assets list contract.
func (g *ProjectGateway) ListAssetProcessingStates(ctx context.Context, projectID string, assetIDs []string) (string, []ProjectAssetProcessingState, error) {
	query, err := projectAssetProcessingQuery(projectID, assetIDs)
	if err != nil {
		return "", nil, err
	}
	scope, err := g.currentScope(ctx)
	if err != nil {
		return "", nil, err
	}
	var payload json.RawMessage
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return g.transport.DoAuthenticatedJSONQuery(callCtx, http.MethodGet, projectPath(projectID)+"/assets/processing-states", query, nil, token, nil, &payload)
	})
	if err != nil {
		return "", nil, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return "", nil, err
	}
	var items []ProjectAssetProcessingState
	if err := decodeV2Payload(payload, &items); err != nil {
		return "", nil, err
	}
	for index := range items {
		if err := normalizeProjectAssetProcessingState(&items[index]); err != nil {
			return "", nil, err
		}
	}
	return scope.Key, items, nil
}

func projectAssetProcessingQuery(projectID string, assetIDs []string) (url.Values, error) {
	if err := validateOpaqueProjectID(projectID); err != nil {
		return nil, err
	}
	if len(assetIDs) > 100 {
		return nil, fmt.Errorf("projects-v2: at most 100 asset ids may be queried")
	}
	query := url.Values{}
	for _, assetID := range assetIDs {
		if err := validateOpaqueProjectID(assetID); err != nil {
			return nil, err
		}
		query.Add("asset_id", assetID)
	}
	return query, nil
}

func normalizeProjectAssetProcessingState(item *ProjectAssetProcessingState) error {
	if err := validateProjectAssetProcessingState(*item); err != nil {
		return err
	}
	item.CreatedAt = item.CreatedAt.UTC()
	item.UpdatedAt = item.UpdatedAt.UTC()
	item.ScanCompletedAt = utcTimePointer(item.ScanCompletedAt)
	item.PurgedAt = utcTimePointer(item.PurgedAt)
	return nil
}

func utcTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	normalized := value.UTC()
	return &normalized
}

func validateProjectAssetProcessingState(item ProjectAssetProcessingState) error {
	if !validProjectAssetProcessingIdentity(item) || !validProjectAssetProcessingStatuses(item) {
		return fmt.Errorf("projects-v2: invalid asset processing state response")
	}
	return nil
}

func validProjectAssetProcessingIdentity(item ProjectAssetProcessingState) bool {
	return validateOpaqueProjectID(item.AssetID) == nil && validateOpaqueProjectID(item.AssetVersionID) == nil &&
		item.VersionNumber > 0 && item.VersionNumber <= MaxSafeJSONInteger &&
		item.StateVersion > 0 && item.StateVersion <= MaxSafeJSONInteger && item.ScanAttempts <= MaxSafeJSONInteger &&
		!item.CreatedAt.IsZero() && !item.UpdatedAt.IsZero()
}

func validProjectAssetProcessingStatuses(item ProjectAssetProcessingState) bool {
	return oneOf(item.ScanStatus, "pending", "scanning", "clean", "rejected", "failed_retryable") &&
		oneOf(item.ParseStatus, "not_started", "not_applicable", "processing", "ready", "failed_retryable") &&
		oneOf(item.IndexStatus, "not_started", "not_applicable", "processing", "ready", "failed_retryable") &&
		oneOf(item.PurgeStatus, "active", "pending", "cleanup_pending", "purged")
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func (g *ProjectGateway) UploadAsset(ctx context.Context, projectID, filename, contentType string, content []byte) (string, ProjectAsset, error) {
	return g.UploadAssetToFolder(ctx, projectID, filename, contentType, content, nil)
}

func (g *ProjectGateway) UploadAssetToFolder(ctx context.Context, projectID, filename, contentType string, content []byte, folderID *string) (string, ProjectAsset, error) {
	transport, err := validateAssetUpload(g.transport, projectID, filename, content)
	if err != nil {
		return "", ProjectAsset{}, err
	}
	if folderID != nil {
		if err := validateOpaqueProjectID(*folderID); err != nil {
			return "", ProjectAsset{}, err
		}
	}
	body, multipartContentType, err := buildProjectAssetMultipart(filename, content, folderID)
	if err != nil {
		return "", ProjectAsset{}, err
	}
	scope, err := g.currentScope(ctx)
	if err != nil {
		return "", ProjectAsset{}, err
	}
	var payload json.RawMessage
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return transport.DoAuthenticatedMultipartJSON(callCtx, http.MethodPost, projectPath(projectID)+"/assets", body, multipartContentType, token, nil, &payload)
	})
	if err != nil {
		return "", ProjectAsset{}, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return "", ProjectAsset{}, err
	}
	var item ProjectAsset
	if err := decodeV2Payload(payload, &item, "asset_id", "project_id", "filename", "content_type", "size_bytes", "sha256", "version", "work_item_ids", "created_at", "updated_at"); err != nil {
		return "", ProjectAsset{}, err
	}
	if err := validateProjectAssets([]ProjectAsset{item}, projectID); err != nil {
		return "", ProjectAsset{}, err
	}
	_ = contentType // Cloud detects and owns the authoritative media type.
	return scope.Key, item, nil
}

// UploadAssetFileToFolder streams a replayable local file. The authenticated
// executor may invoke the factory again after token refresh; every invocation
// opens an independent file descriptor and pipe.
func (g *ProjectGateway) UploadAssetFileToFolder(ctx context.Context, projectID, path, filename string, folderID *string) (string, ProjectAsset, error) {
	transport, err := validateAssetFileUpload(g.transport, projectID, filename, folderID)
	if err != nil {
		return "", ProjectAsset{}, err
	}
	scope, err := g.currentScope(ctx)
	if err != nil {
		return "", ProjectAsset{}, err
	}
	var payload json.RawMessage
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return transport.DoAuthenticatedMultipartStreamJSON(callCtx, http.MethodPost, projectPath(projectID)+"/assets", func(context.Context) (io.ReadCloser, string, error) {
			return openProjectAssetMultipart(path, filename, folderID, nil)
		}, token, nil, &payload)
	})
	if err != nil {
		return "", ProjectAsset{}, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return "", ProjectAsset{}, err
	}
	item, err := decodeProjectAssetUpload(payload, projectID)
	if err != nil {
		return "", ProjectAsset{}, err
	}
	return scope.Key, item, nil
}

// ReplaceAssetFileContent streams a new version for an existing asset. The
// Cloud keeps prior versions; expectedVersion prevents overwriting a newer
// version selected by another project member.
func (g *ProjectGateway) ReplaceAssetFileContent(ctx context.Context, projectID, assetID, path, filename string, expectedVersion uint64) (string, ProjectAsset, error) {
	if err := validateAssetDirectoryMutation(projectID, assetID, expectedVersion); err != nil {
		return "", ProjectAsset{}, err
	}
	transport, err := validateAssetFileUpload(g.transport, projectID, filename, nil)
	if err != nil {
		return "", ProjectAsset{}, err
	}
	scope, err := g.currentScope(ctx)
	if err != nil {
		return "", ProjectAsset{}, err
	}
	var payload json.RawMessage
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return transport.DoAuthenticatedMultipartStreamJSON(callCtx, http.MethodPut, assetPath(projectID, assetID)+"/content", func(context.Context) (io.ReadCloser, string, error) {
			return openProjectAssetMultipart(path, filename, nil, &expectedVersion)
		}, token, nil, &payload)
	})
	if err != nil {
		return "", ProjectAsset{}, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return "", ProjectAsset{}, err
	}
	item, err := decodeProjectAssetUpload(payload, projectID)
	if err != nil {
		return "", ProjectAsset{}, err
	}
	if item.AssetID != assetID || item.Version <= expectedVersion {
		return "", ProjectAsset{}, fmt.Errorf("projects-v2: invalid asset replacement response")
	}
	return scope.Key, item, nil
}

func validateAssetFileUpload(rawTransport any, projectID, filename string, folderID *string) (projectMultipartStreamTransport, error) {
	if err := validateOpaqueProjectID(projectID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(filename) == "" {
		return nil, fmt.Errorf("projects-v2: asset file is required")
	}
	if folderID != nil {
		if err := validateOpaqueProjectID(*folderID); err != nil {
			return nil, err
		}
	}
	transport, ok := rawTransport.(projectMultipartStreamTransport)
	if !ok {
		return nil, fmt.Errorf("projects-v2: streaming multipart transport is unavailable")
	}
	return transport, nil
}

func decodeProjectAssetUpload(payload json.RawMessage, projectID string) (ProjectAsset, error) {
	var item ProjectAsset
	if err := decodeV2Payload(payload, &item, "asset_id", "project_id", "filename", "content_type", "size_bytes", "sha256", "version", "work_item_ids", "created_at", "updated_at"); err != nil {
		return ProjectAsset{}, err
	}
	if err := validateProjectAssets([]ProjectAsset{item}, projectID); err != nil {
		return ProjectAsset{}, err
	}
	return item, nil
}

func openProjectAssetMultipart(path, filename string, folderID *string, expectedVersion *uint64) (io.ReadCloser, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, "", fmt.Errorf("projects-v2: open asset upload: %w", err)
	}
	reader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)
	go func() {
		defer file.Close()
		var writeErr error
		if folderID != nil {
			writeErr = writer.WriteField("folder_id", *folderID)
		}
		if writeErr == nil && expectedVersion != nil {
			writeErr = writer.WriteField("expected_version", strconv.FormatUint(*expectedVersion, 10))
		}
		var part io.Writer
		if writeErr == nil {
			part, writeErr = writer.CreateFormFile("file", filename)
		}
		if writeErr == nil {
			_, writeErr = io.Copy(part, file)
		}
		if writeErr == nil {
			writeErr = writer.Close()
		}
		if writeErr != nil {
			_ = pipeWriter.CloseWithError(writeErr)
		} else {
			_ = pipeWriter.Close()
		}
	}()
	return reader, writer.FormDataContentType(), nil
}

func validateAssetUpload(rawTransport any, projectID, filename string, content []byte) (projectMultipartTransport, error) {
	if err := validateOpaqueProjectID(projectID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(filename) == "" || len(content) == 0 {
		return nil, fmt.Errorf("projects-v2: asset file is required")
	}
	transport, ok := rawTransport.(projectMultipartTransport)
	if !ok {
		return nil, fmt.Errorf("projects-v2: multipart transport is unavailable")
	}
	return transport, nil
}

func buildProjectAssetMultipart(filename string, content []byte, folderID *string) ([]byte, string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if folderID != nil {
		if err := writer.WriteField("folder_id", *folderID); err != nil {
			return nil, "", err
		}
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(content); err != nil {
		return nil, "", err
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return body.Bytes(), writer.FormDataContentType(), nil
}

func (g *ProjectGateway) AssetDownload(ctx context.Context, projectID, assetID string) (string, ProjectAssetDownload, error) {
	if err := validateAssetIDs(projectID, assetID); err != nil {
		return "", ProjectAssetDownload{}, err
	}
	scope, err := g.currentScope(ctx)
	if err != nil {
		return "", ProjectAssetDownload{}, err
	}
	var payload json.RawMessage
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return g.transport.DoAuthenticatedJSON(callCtx, http.MethodGet, assetPath(projectID, assetID)+"/download", nil, token, nil, &payload)
	})
	if err != nil {
		return "", ProjectAssetDownload{}, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return "", ProjectAssetDownload{}, err
	}
	var item ProjectAssetDownload
	if err := decodeV2Payload(payload, &item, "asset_id", "filename", "url", "expires_at"); err != nil {
		return "", ProjectAssetDownload{}, err
	}
	if item.AssetID != assetID || strings.TrimSpace(item.Filename) == "" || strings.TrimSpace(item.URL) == "" || item.ExpiresAt.IsZero() {
		return "", ProjectAssetDownload{}, fmt.Errorf("projects-v2: invalid asset download response")
	}
	return scope.Key, item, nil
}

func (g *ProjectGateway) DeleteAsset(ctx context.Context, projectID, assetID string) (string, error) {
	if err := validateAssetIDs(projectID, assetID); err != nil {
		return "", err
	}
	scope, err := g.currentScope(ctx)
	if err != nil {
		return "", err
	}
	var payload json.RawMessage
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return g.transport.DoAuthenticatedJSON(callCtx, http.MethodDelete, assetPath(projectID, assetID), nil, token, nil, &payload)
	})
	if err != nil {
		return "", err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return "", err
	}
	var receipt struct {
		Deleted bool `json:"deleted"`
	}
	if err := decodeV2Payload(payload, &receipt, "deleted"); err != nil || !receipt.Deleted {
		if err != nil {
			return "", err
		}
		return "", fmt.Errorf("projects-v2: asset deletion was not acknowledged")
	}
	return scope.Key, nil
}

func (g *ProjectGateway) MutateWorkItemAsset(ctx context.Context, projectID, workItemID, assetID string, put bool) (string, ProjectAsset, error) {
	if err := validateAssetIDs(projectID, assetID); err != nil {
		return "", ProjectAsset{}, err
	}
	if err := validateOpaqueProjectID(workItemID); err != nil {
		return "", ProjectAsset{}, err
	}
	method := http.MethodDelete
	if put {
		method = http.MethodPut
	}
	scope, err := g.currentScope(ctx)
	if err != nil {
		return "", ProjectAsset{}, err
	}
	var payload json.RawMessage
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		path := projectPath(projectID) + "/work-items/" + url.PathEscape(workItemID) + "/assets/" + url.PathEscape(assetID)
		return g.transport.DoAuthenticatedJSON(callCtx, method, path, nil, token, nil, &payload)
	})
	if err != nil {
		return "", ProjectAsset{}, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return "", ProjectAsset{}, err
	}
	var item ProjectAsset
	if err := decodeV2Payload(payload, &item, "asset_id", "project_id", "filename", "content_type", "size_bytes", "sha256", "version", "work_item_ids", "created_at", "updated_at"); err != nil {
		return "", ProjectAsset{}, err
	}
	if err := validateProjectAssets([]ProjectAsset{item}, projectID); err != nil {
		return "", ProjectAsset{}, err
	}
	return scope.Key, item, nil
}

func validateProjectAssets(items []ProjectAsset, projectID string) error {
	for _, item := range items {
		if err := validateProjectAsset(item, projectID); err != nil {
			return err
		}
	}
	return nil
}

func validateProjectAsset(item ProjectAsset, projectID string) error {
	if err := validateAssetIDs(projectID, item.AssetID); err != nil || !projectAssetMetadataValid(item, projectID) {
		return fmt.Errorf("projects-v2: invalid asset response")
	}
	for _, workItemID := range item.WorkItemIDs {
		if err := validateOpaqueProjectID(workItemID); err != nil {
			return fmt.Errorf("projects-v2: invalid asset work item reference")
		}
	}
	return nil
}

func projectAssetMetadataValid(item ProjectAsset, projectID string) bool {
	return item.ProjectID == projectID &&
		strings.TrimSpace(item.Filename) != "" &&
		item.SizeBytes > 0 &&
		len(item.SHA256) == 64 &&
		item.Version > 0 &&
		item.WorkItemIDs != nil &&
		!item.CreatedAt.IsZero() &&
		!item.UpdatedAt.IsZero()
}

func validateAssetIDs(projectID, assetID string) error {
	if err := validateOpaqueProjectID(projectID); err != nil {
		return err
	}
	return validateOpaqueProjectID(assetID)
}

func assetPath(projectID, assetID string) string {
	return projectPath(projectID) + "/assets/" + url.PathEscape(assetID)
}

func assetFolderPath(projectID, folderID string) string {
	return projectPath(projectID) + "/asset-folders/" + url.PathEscape(folderID)
}

func projectAssetDirectoryQuery(input DirectoryQuery) (url.Values, error) {
	query := url.Values{}
	if err := addProjectAssetDirectoryLocation(query, input); err != nil {
		return nil, err
	}
	if err := addProjectAssetDirectoryFilters(query, input); err != nil {
		return nil, err
	}
	if err := addProjectAssetDirectoryPage(query, input); err != nil {
		return nil, err
	}
	return query, nil
}

func addProjectAssetDirectoryLocation(query url.Values, input DirectoryQuery) error {
	if input.FolderID != "" {
		if err := validateOpaqueProjectID(input.FolderID); err != nil {
			return err
		}
		query.Set("folder_id", input.FolderID)
	}
	search := strings.TrimSpace(input.Search)
	if !validProjectText(search, 200) {
		return fmt.Errorf("projects-v2: invalid asset directory search")
	}
	if search != "" {
		query.Set("search", search)
	}
	return nil
}

func addProjectAssetDirectoryFilters(query url.Values, input DirectoryQuery) error {
	if input.Type != "" && !oneOf(input.Type, "all", "folder", "image", "video", "pdf", "audio", "document", "markdown", "archive", "other") {
		return fmt.Errorf("projects-v2: invalid asset directory type")
	}
	if input.Sort != "" && !oneOf(input.Sort, "name", "updated_at") {
		return fmt.Errorf("projects-v2: invalid asset directory sort")
	}
	if input.Direction != "" && !oneOf(input.Direction, "asc", "desc") {
		return fmt.Errorf("projects-v2: invalid asset directory direction")
	}
	if input.Type != "" {
		query.Set("type", input.Type)
	}
	if input.Sort != "" {
		query.Set("sort", input.Sort)
	}
	if input.Direction != "" {
		query.Set("direction", input.Direction)
	}
	return nil
}

func addProjectAssetDirectoryPage(query url.Values, input DirectoryQuery) error {
	if validateWorkItemListCursor(input.Cursor) != nil {
		return fmt.Errorf("projects-v2: invalid asset directory cursor")
	}
	if input.Limit < 0 || input.Limit > 100 {
		return fmt.Errorf("projects-v2: asset directory limit must be between 1 and 100")
	}
	if input.Cursor != "" {
		query.Set("cursor", input.Cursor)
	}
	if input.Limit != 0 {
		query.Set("limit", strconv.Itoa(input.Limit))
	}
	return nil
}

package project

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type AssetOperationRecordsQuery struct {
	Operation    string
	ResourceKind string
	ActorUserID  string
	Limit        int
}

type ProjectAssetOperationRecord struct {
	OperationID      string         `json:"operation_id"`
	ProjectID        string         `json:"project_id"`
	ResourceKind     string         `json:"resource_kind"`
	ResourceID       string         `json:"resource_id"`
	ResourceName     string         `json:"resource_name"`
	Operation        string         `json:"operation"`
	OperationDetail  string         `json:"operation_detail"`
	ActorUserID      string         `json:"actor_user_id"`
	ActorDisplayName string         `json:"actor_display_name"`
	ResourceVersion  *uint64        `json:"resource_version"`
	FolderID         *string        `json:"folder_id"`
	Details          map[string]any `json:"details"`
	CreatedAt        time.Time      `json:"created_at"`
}

type ProjectAssetOperationRecordPage struct {
	ProjectID string                        `json:"project_id"`
	Items     []ProjectAssetOperationRecord `json:"items"`
	Limit     int                           `json:"limit"`
}

func (g *ProjectGateway) ListAssetOperationRecords(ctx context.Context, projectID string, input AssetOperationRecordsQuery) (string, ProjectAssetOperationRecordPage, error) {
	if err := validateOpaqueProjectID(projectID); err != nil {
		return "", ProjectAssetOperationRecordPage{}, err
	}
	query, err := projectAssetOperationRecordsQuery(input)
	if err != nil {
		return "", ProjectAssetOperationRecordPage{}, err
	}
	scope, err := g.currentScope(ctx)
	if err != nil {
		return "", ProjectAssetOperationRecordPage{}, err
	}
	var payload json.RawMessage
	err = g.executor.Do(ctx, scope.Binding, func(callCtx context.Context, token string) error {
		return g.transport.DoAuthenticatedJSONQuery(callCtx, http.MethodGet, projectPath(projectID)+"/asset-operation-records", query, nil, token, nil, &payload)
	})
	if err != nil {
		return "", ProjectAssetOperationRecordPage{}, err
	}
	if err := g.revalidateScope(ctx, scope); err != nil {
		return "", ProjectAssetOperationRecordPage{}, err
	}
	page, err := decodeProjectAssetOperationRecordPage(payload, projectID)
	if err != nil {
		return "", ProjectAssetOperationRecordPage{}, err
	}
	return scope.Key, page, nil
}

func projectAssetOperationRecordsQuery(input AssetOperationRecordsQuery) (url.Values, error) {
	query := url.Values{}
	if input.Operation != "" {
		if !oneOf(input.Operation, "create", "read", "update", "delete") {
			return nil, fmt.Errorf("projects-v2: invalid asset operation filter")
		}
		query.Set("operation", input.Operation)
	}
	if input.ResourceKind != "" {
		if !oneOf(input.ResourceKind, "asset", "folder", "directory") {
			return nil, fmt.Errorf("projects-v2: invalid asset resource kind filter")
		}
		query.Set("resource_kind", input.ResourceKind)
	}
	if input.ActorUserID != "" {
		if err := validateOpaqueProjectID(input.ActorUserID); err != nil {
			return nil, fmt.Errorf("projects-v2: invalid asset operation actor_user_id")
		}
		query.Set("actor_user_id", input.ActorUserID)
	}
	if input.Limit < 0 || input.Limit > 100 {
		return nil, fmt.Errorf("projects-v2: asset operation limit must be between 1 and 100")
	}
	if input.Limit > 0 {
		query.Set("limit", strconv.Itoa(input.Limit))
	}
	return query, nil
}

func decodeProjectAssetOperationRecordPage(payload []byte, expectedProjectID string) (ProjectAssetOperationRecordPage, error) {
	var page ProjectAssetOperationRecordPage
	if err := decodeV2Payload(payload, &page, "project_id", "items", "limit"); err != nil {
		return ProjectAssetOperationRecordPage{}, err
	}
	if page.Items == nil {
		page.Items = []ProjectAssetOperationRecord{}
	}
	if page.ProjectID != expectedProjectID || page.Limit < 1 || page.Limit > 100 || len(page.Items) > page.Limit {
		return ProjectAssetOperationRecordPage{}, fmt.Errorf("projects-v2: invalid asset operation record page")
	}
	for index := range page.Items {
		if err := validateProjectAssetOperationRecord(page.Items[index], expectedProjectID); err != nil {
			return ProjectAssetOperationRecordPage{}, err
		}
		page.Items[index].CreatedAt = page.Items[index].CreatedAt.UTC()
		if page.Items[index].Details == nil {
			page.Items[index].Details = map[string]any{}
		}
	}
	return page, nil
}

func validateProjectAssetOperationRecord(item ProjectAssetOperationRecord, expectedProjectID string) error {
	if invalidAssetOperationRecordIdentity(item, expectedProjectID) || invalidAssetOperationRecordDescription(item) {
		return fmt.Errorf("projects-v2: invalid asset operation record")
	}
	if item.ResourceID != "" && validateOpaqueProjectID(item.ResourceID) != nil {
		return fmt.Errorf("projects-v2: invalid asset operation resource")
	}
	if item.FolderID != nil && validateOpaqueProjectID(*item.FolderID) != nil {
		return fmt.Errorf("projects-v2: invalid asset operation folder")
	}
	if item.ResourceVersion != nil && (*item.ResourceVersion == 0 || *item.ResourceVersion > MaxSafeJSONInteger) {
		return fmt.Errorf("projects-v2: invalid asset operation resource version")
	}
	return nil
}

func invalidAssetOperationRecordIdentity(item ProjectAssetOperationRecord, expectedProjectID string) bool {
	return validateOpaqueProjectID(item.OperationID) != nil || item.ProjectID != expectedProjectID ||
		!oneOf(item.ResourceKind, "asset", "folder", "directory") ||
		!oneOf(item.Operation, "create", "read", "update", "delete") ||
		!validAssetOperationDetail(item.Operation, item.OperationDetail)
}

func invalidAssetOperationRecordDescription(item ProjectAssetOperationRecord) bool {
	return validateOpaqueProjectID(item.ActorUserID) != nil || strings.TrimSpace(item.ActorDisplayName) == "" ||
		!validProjectText(item.ActorDisplayName, 255) || !validProjectText(item.ResourceName, 1024) || item.CreatedAt.IsZero()
}

func validAssetOperationDetail(operation, detail string) bool {
	allowed := map[string][]string{
		"create": {"upload", "create_folder"},
		"read":   {"list_directory", "list_assets", "download"},
		"update": {"replace_content", "update_metadata", "update_folder"},
		"delete": {"delete_asset", "delete_folder"},
	}
	return oneOf(detail, allowed[operation]...)
}

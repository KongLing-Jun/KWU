package project

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProjectGatewayListsAssetOperationRecordsWithExactFilters(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/projects/project-1/asset-operation-records" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		wantQuery := "actor_user_id=user-1&limit=20&operation=delete&resource_kind=asset"
		if r.URL.RawQuery != wantQuery {
			t.Fatalf("query = %q, want %q", r.URL.RawQuery, wantQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"code":"OK","data":{"project_id":"project-1","items":[{"operation_id":"assetop-1","project_id":"project-1","resource_kind":"asset","resource_id":"asset-1","resource_name":"项目方案.pdf","operation":"delete","operation_detail":"delete_asset","actor_user_id":"user-1","actor_display_name":"张三","resource_version":3,"folder_id":"folder-1","details":{"asset_version_count":3},"created_at":"2026-08-22T08:30:00+08:00"}],"limit":20}}`)
	}))
	defer server.Close()

	gateway := gatewayForServer(t, server, scope, &gatewayTestExecutor{token: "access-secret"})
	resultScope, page, err := gateway.ListAssetOperationRecords(context.Background(), "project-1", AssetOperationRecordsQuery{
		Operation: "delete", ResourceKind: "asset", ActorUserID: "user-1", Limit: 20,
	})
	if err != nil {
		t.Fatalf("ListAssetOperationRecords: %v", err)
	}
	if resultScope != scope.Key || page.ProjectID != "project-1" || page.Limit != 20 || len(page.Items) != 1 {
		t.Fatalf("scope=%q page=%#v", resultScope, page)
	}
	item := page.Items[0]
	if item.OperationDetail != "delete_asset" || item.ActorDisplayName != "张三" || item.ResourceVersion == nil || *item.ResourceVersion != 3 || item.FolderID == nil || *item.FolderID != "folder-1" || item.CreatedAt.Location() != time.UTC {
		t.Fatalf("item = %#v", item)
	}
	if item.Details["asset_version_count"] != float64(3) {
		t.Fatalf("details = %#v", item.Details)
	}
}

func TestProjectGatewayAssetOperationRecordsUsesServerDefaultLimit(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" {
			t.Fatalf("default query = %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"code":"OK","data":{"project_id":"project-1","items":[],"limit":50}}`)
	}))
	defer server.Close()

	gateway := gatewayForServer(t, server, scope, &gatewayTestExecutor{token: "access-secret"})
	_, page, err := gateway.ListAssetOperationRecords(context.Background(), "project-1", AssetOperationRecordsQuery{})
	if err != nil || page.Limit != 50 || page.Items == nil || len(page.Items) != 0 {
		t.Fatalf("page=%#v err=%v", page, err)
	}
}

func TestProjectGatewayAssetOperationRecordFiltersRejectInvalidValues(t *testing.T) {
	for name, input := range map[string]AssetOperationRecordsQuery{
		"operation":     {Operation: "rename"},
		"resource kind": {ResourceKind: "bucket"},
		"actor":         {ActorUserID: " user-1"},
		"limit":         {Limit: 101},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := projectAssetOperationRecordsQuery(input); err == nil {
				t.Fatalf("query unexpectedly accepted %#v", input)
			}
		})
	}
}

func TestDecodeProjectAssetOperationRecordPageRejectsMismatchedOrInvalidRecords(t *testing.T) {
	valid := `{"project_id":"project-1","items":[{"operation_id":"assetop-1","project_id":"project-1","resource_kind":"asset","resource_id":"asset-1","resource_name":"report.pdf","operation":"update","operation_detail":"replace_content","actor_user_id":"user-1","actor_display_name":"Ada","resource_version":2,"folder_id":null,"details":{},"created_at":"2026-08-22T08:30:00Z"}],"limit":50}`
	for name, payload := range map[string]string{
		"project": strings.Replace(valid, `"project_id":"project-1"`, `"project_id":"project-2"`, 1),
		"detail":  strings.Replace(valid, `"operation_detail":"replace_content"`, `"operation_detail":"delete_asset"`, 1),
		"version": strings.Replace(valid, `"resource_version":2`, `"resource_version":0`, 1),
		"limit":   strings.Replace(valid, `"limit":50`, `"limit":101`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeProjectAssetOperationRecordPage([]byte(payload), "project-1"); err == nil {
				t.Fatal("invalid operation record response was accepted")
			}
		})
	}
}

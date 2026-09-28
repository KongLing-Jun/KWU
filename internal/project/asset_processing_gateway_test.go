package project

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProjectGatewayListsAssetProcessingStatesFromDedicatedReadRoute(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/projects/project-1/assets/processing-states" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query()["asset_id"]; len(got) != 1 || got[0] != "asset-1" {
			t.Fatalf("asset_id = %#v", got)
		}
		if r.Header.Get("Authorization") != "Bearer access-secret" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"asset_id":"asset-1","asset_version_id":"asset-version-1","version_number":2,"scan_status":"clean","parse_status":"ready","index_status":"processing","purge_status":"active","state_version":3,"scan_attempts":1,"created_at":"2026-08-10T08:00:00+08:00","updated_at":"2026-08-10T01:00:00Z","scan_completed_at":"2026-08-10T00:30:00Z","purged_at":null}],"error":null,"request_id":"req-asset-processing","server_time":"2026-08-10T01:00:01Z"}`)
	}))
	defer server.Close()

	gateway := gatewayForServer(t, server, scope, &gatewayTestExecutor{token: "access-secret"})
	resultScope, items, err := gateway.ListAssetProcessingStates(context.Background(), "project-1", []string{"asset-1"})
	if err != nil {
		t.Fatalf("ListAssetProcessingStates: %v", err)
	}
	if resultScope != scope.Key || len(items) != 1 {
		t.Fatalf("scope=%q items=%#v", resultScope, items)
	}
	item := items[0]
	if item.AssetID != "asset-1" || item.AssetVersionID != "asset-version-1" || item.VersionNumber != 2 || item.StateVersion != 3 || item.ScanAttempts != 1 {
		t.Fatalf("identity and version fields = %#v", item)
	}
	if item.ScanStatus != "clean" || item.ParseStatus != "ready" || item.IndexStatus != "processing" || item.PurgeStatus != "active" {
		t.Fatalf("processing states = %#v", item)
	}
	if item.CreatedAt.Location().String() != "UTC" || item.ScanCompletedAt == nil || item.PurgedAt != nil {
		t.Fatalf("timestamps = %#v", item)
	}
}

func TestValidateProjectAssetProcessingStateRejectsInvalidIdentityAndStatus(t *testing.T) {
	valid := ProjectAssetProcessingState{
		AssetID: "asset-1", AssetVersionID: "asset-version-1", VersionNumber: 1,
		ScanStatus: "clean", ParseStatus: "ready", IndexStatus: "ready", PurgeStatus: "active",
		StateVersion: 1, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	for name, mutate := range map[string]func(*ProjectAssetProcessingState){
		"missing version": func(item *ProjectAssetProcessingState) { item.VersionNumber = 0 },
		"invalid status":  func(item *ProjectAssetProcessingState) { item.ScanStatus = "unknown" },
	} {
		t.Run(name, func(t *testing.T) {
			item := valid
			mutate(&item)
			if err := validateProjectAssetProcessingState(item); err == nil {
				t.Fatal("expected invalid asset processing state")
			}
		})
	}
}

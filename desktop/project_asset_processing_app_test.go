package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kwu/internal/project"
)

func TestDownloadProjectAssetFileWritesBytesWithoutOverwriting(t *testing.T) {
	want := []byte("project asset bytes")
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Fatalf("download method = %s", request.Method)
		}
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write(want)
	}))
	defer server.Close()

	directory := t.TempDir()
	existing := filepath.Join(directory, "report.json")
	if err := os.WriteFile(existing, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	savedPath, err := downloadProjectAssetFile(context.Background(), server.Client(), directory, `../report.json`, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if wantPath := filepath.Join(directory, "report (1).json"); savedPath != wantPath {
		t.Fatalf("saved path = %q, want %q", savedPath, wantPath)
	}
	got, err := os.ReadFile(savedPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("downloaded bytes = %q, want %q", got, want)
	}
	unchanged, err := os.ReadFile(existing)
	if err != nil || string(unchanged) != "keep me" {
		t.Fatalf("existing file changed: bytes=%q err=%v", unchanged, err)
	}

	longName := strings.Repeat("资料📎", 120) + ".json"
	longPath, err := downloadProjectAssetFile(context.Background(), server.Client(), directory, longName, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if base := filepath.Base(longPath); len(base) > projectAssetDownloadFilenameMaxBytes || !strings.HasSuffix(base, ".json") {
		t.Fatalf("portable downloaded filename = %q (%d bytes)", base, len(base))
	}
	if downloaded, err := os.ReadFile(longPath); err != nil || string(downloaded) != string(want) {
		t.Fatalf("portable downloaded bytes = %q, err=%v", downloaded, err)
	}
}

func TestDownloadProjectAssetFileRemovesFailedDestination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		http.Error(response, "unavailable", http.StatusBadGateway)
	}))
	defer server.Close()

	directory := t.TempDir()
	if _, err := downloadProjectAssetFile(context.Background(), server.Client(), directory, "report.json", server.URL); err == nil {
		t.Fatal("download unexpectedly succeeded")
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("failed download left files behind: %#v", entries)
	}
}

func TestSafeProjectAssetDownloadFilenameRejectsPathAndWindowsDeviceNames(t *testing.T) {
	for input, want := range map[string]string{
		`../folder/report?.json`:  "report_.json",
		`..\\folder\\report.json`: "report.json",
		`CON.txt`:                 "_CON.txt",
		`con .txt`:                "_con .txt",
		`COM¹.log`:                "_COM¹.log",
		`...`:                     "download",
	} {
		if got := safeProjectAssetDownloadFilename(input); got != want {
			t.Errorf("safeProjectAssetDownloadFilename(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSafeProjectAssetDownloadFilenameFitsPortableByteLimitWithoutBreakingUnicode(t *testing.T) {
	filename := strings.Repeat("资料📎", 120) + ".json"
	got := safeProjectAssetDownloadFilename(filename)
	if len(got) > projectAssetDownloadFilenameMaxBytes {
		t.Fatalf("portable filename length = %d, max %d", len(got), projectAssetDownloadFilenameMaxBytes)
	}
	if !strings.HasSuffix(got, ".json") {
		t.Fatalf("portable filename lost extension: %q", got)
	}
	if strings.ContainsRune(got, '\uFFFD') {
		t.Fatalf("portable filename split a Unicode code point: %q", got)
	}

	stem, extension := splitProjectAssetDownloadFilename(got)
	conflict := fitProjectAssetDownloadFilename(stem, " (9999)", extension)
	if len(conflict) > projectAssetDownloadFilenameMaxBytes || !strings.HasSuffix(conflict, " (9999).json") {
		t.Fatalf("portable conflict filename = %q (%d bytes)", conflict, len(conflict))
	}
}

func TestProjectAssetUploadAdmissionAllowsOnlyOneConcurrentUpload(t *testing.T) {
	if err := acquireProjectAssetUpload(context.Background()); err != nil {
		t.Fatalf("acquire first upload: %v", err)
	}
	released := false
	t.Cleanup(func() {
		if !released {
			releaseProjectAssetUpload()
		}
	})

	waiting, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := acquireProjectAssetUpload(waiting); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second upload admission error = %v, want deadline exceeded", err)
	}

	releaseProjectAssetUpload()
	released = true
	if err := acquireProjectAssetUpload(context.Background()); err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	releaseProjectAssetUpload()
}

func TestCloudProjectAssetProcessingStateListViewPreservesSafeVersionState(t *testing.T) {
	completedAt := time.Date(2026, 8, 10, 8, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	view := cloudProjectAssetProcessingStateListView("scope-1", "project-1", []project.ProjectAssetProcessingState{{
		AssetID: "asset-1", AssetVersionID: "asset-version-1", VersionNumber: 2,
		ScanStatus: "clean", ParseStatus: "ready", IndexStatus: "processing", PurgeStatus: "active",
		StateVersion: 3, ScanAttempts: 1, CreatedAt: completedAt.Add(-time.Hour), UpdatedAt: completedAt,
		ScanCompletedAt: &completedAt,
	}})
	if view.ProjectScopeKey != "scope-1" || view.ProjectID != "project-1" || len(view.Items) != 1 {
		t.Fatalf("view = %#v", view)
	}
	item := view.Items[0]
	if item.AssetID != "asset-1" || item.AssetVersionID != "asset-version-1" || item.VersionNumber != 2 || item.StateVersion != 3 || item.ScanAttempts != 1 {
		t.Fatalf("identity and version fields = %#v", item)
	}
	if item.ScanStatus != "clean" || item.ParseStatus != "ready" || item.IndexStatus != "processing" || item.PurgeStatus != "active" {
		t.Fatalf("states = %#v", item)
	}
	if item.ScanCompletedAt == nil || *item.ScanCompletedAt != "2026-08-10T00:30:00Z" || item.PurgedAt != nil {
		t.Fatalf("optional timestamps = %#v", item)
	}
}

func TestCloudProjectAssetDirectoryPageViewPreservesNullableItemShapeAndQuota(t *testing.T) {
	updatedAt := time.Date(2026, 8, 11, 9, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	folderID := "folder-1"
	contentType := "application/pdf"
	sizeBytes := int64(42)
	page := cloudProjectAssetDirectoryPageView("scope-1", "project-1", project.ProjectAssetDirectoryPage{
		Breadcrumbs: []project.ProjectAssetDirectoryBreadcrumb{{FolderID: folderID, Name: "Reports"}},
		Items: []project.ProjectAssetDirectoryItem{{
			Kind: "asset", ID: "asset-1", Name: "Q3.pdf", FolderID: &folderID,
			ContentType: &contentType, SizeBytes: &sizeBytes,
			UpdatedBy: project.ProjectAssetDirectoryUpdatedBy{UserID: "user-1", DisplayName: "Ada"},
			Version:   3, UpdatedAt: updatedAt,
		}},
		TotalCount: 1, UsageBytes: 42, QuotaBytes: 1024, QuotaPlan: "free",
	})
	if page.ProjectScopeKey != "scope-1" || page.ProjectID != "project-1" || page.TotalCount != 1 || page.UsageBytes != 42 || page.QuotaBytes != 1024 || page.QuotaPlan != "free" {
		t.Fatalf("page = %#v", page)
	}
	if len(page.Breadcrumbs) != 1 || len(page.Items) != 1 {
		t.Fatalf("page collections = %#v", page)
	}
	item := page.Items[0]
	if item.FolderID == nil || *item.FolderID != "folder-1" || item.ParentFolderID != nil || item.ContentType == nil || *item.ContentType != "application/pdf" || item.SizeBytes == nil || *item.SizeBytes != 42 {
		t.Fatalf("item = %#v", item)
	}
	if item.UpdatedBy.UserID != "user-1" || item.UpdatedAt != "2026-08-11T01:30:00Z" {
		t.Fatalf("updated projection = %#v", item)
	}
	// Projection must own its nullable values rather than aliasing gateway state.
	*item.FolderID = "changed"
	if folderID != "folder-1" {
		t.Fatalf("folder ID alias leaked: %q", folderID)
	}
}

func TestCloudProjectAssetOperationRecordPageViewPreservesActorAndVersion(t *testing.T) {
	createdAt := time.Date(2026, 8, 22, 16, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	version := uint64(3)
	folderID := "folder-1"
	details := map[string]any{"asset_version_count": float64(3)}
	page := cloudProjectAssetOperationRecordPageView("scope-1", project.ProjectAssetOperationRecordPage{
		ProjectID: "project-1",
		Items: []project.ProjectAssetOperationRecord{{
			OperationID: "assetop-1", ProjectID: "project-1", ResourceKind: "asset", ResourceID: "asset-1",
			ResourceName: "项目方案.pdf", Operation: "delete", OperationDetail: "delete_asset",
			ActorUserID: "user-1", ActorDisplayName: "张三", ResourceVersion: &version, FolderID: &folderID,
			Details: details, CreatedAt: createdAt,
		}},
		Limit: 20,
	})
	if page.ProjectScopeKey != "scope-1" || page.ProjectID != "project-1" || page.Limit != 20 || len(page.Items) != 1 {
		t.Fatalf("page = %#v", page)
	}
	item := page.Items[0]
	if item.ActorDisplayName != "张三" || item.OperationDetail != "delete_asset" || item.ResourceVersion == nil || *item.ResourceVersion != 3 || item.FolderID == nil || *item.FolderID != "folder-1" || item.CreatedAt != "2026-08-22T08:30:00Z" {
		t.Fatalf("item = %#v", item)
	}
	*item.ResourceVersion = 4
	*item.FolderID = "changed"
	item.Details["asset_version_count"] = float64(4)
	if version != 3 || folderID != "folder-1" || details["asset_version_count"] != float64(3) {
		t.Fatalf("projection clone state = version %d folder %q details %#v", version, folderID, details)
	}
}

var _ interface {
	ListCloudProjectAssetProcessingStates(ListCloudProjectAssetProcessingStatesInput) (CloudProjectAssetProcessingStateListView, error)
	ListCloudProjectAssetDirectory(CloudProjectAssetDirectoryInput) (CloudProjectAssetDirectoryPageView, error)
	ListCloudProjectAssetOperationRecords(ListCloudProjectAssetOperationRecordsInput) (CloudProjectAssetOperationRecordPageView, error)
	CreateCloudProjectAssetFolder(CreateCloudProjectAssetFolderInput) (CloudProjectAssetDirectoryPageView, error)
	PatchCloudProjectAssetFolder(PatchCloudProjectAssetFolderInput) (CloudProjectAssetDirectoryPageView, error)
	DeleteCloudProjectAssetFolder(DeleteCloudProjectAssetFolderInput) (CloudProjectAssetDirectoryPageView, error)
	PatchCloudProjectAsset(PatchCloudProjectAssetInput) (CloudProjectAssetDirectoryPageView, error)
	UploadCloudProjectAssetToFolder(UploadCloudProjectAssetInput) (CloudProjectAssetDirectoryPageView, error)
	ReplaceCloudProjectAssetContent(ReplaceCloudProjectAssetContentInput) (CloudProjectAssetDirectoryPageView, error)
} = (*App)(nil)

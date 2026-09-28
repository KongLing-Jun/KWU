package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kwu/internal/project"
	"kwu/internal/userctx"
)

type projectAssetToolGatewayFake struct {
	projectIDs []string
	actions    []string
	paths      []string
	download   project.ProjectAssetDownload
}

func (f *projectAssetToolGatewayFake) record(action, projectID string) {
	f.actions = append(f.actions, action)
	f.projectIDs = append(f.projectIDs, projectID)
}

func (f *projectAssetToolGatewayFake) GetProject(context.Context, string) (project.ProjectResult, error) {
	return project.ProjectResult{}, nil
}

func (f *projectAssetToolGatewayFake) ListAssetDirectory(_ context.Context, projectID string, input project.DirectoryQuery) (string, project.ProjectAssetDirectoryPage, error) {
	f.record("list", projectID)
	return "scope", project.ProjectAssetDirectoryPage{ProjectID: projectID, FolderID: optionalProjectAssetToolID(input.FolderID), TotalCount: 1}, nil
}

func (f *projectAssetToolGatewayFake) AssetDownload(_ context.Context, projectID, assetID string) (string, project.ProjectAssetDownload, error) {
	f.record("get_download_url", projectID)
	item := f.download
	if item.AssetID == "" {
		item.AssetID = assetID
	}
	if item.URL == "" {
		item.URL = "https://assets.example/download"
	}
	return "scope", item, nil
}

func (f *projectAssetToolGatewayFake) UploadAssetFileToFolder(_ context.Context, projectID, path, filename string, _ *string) (string, project.ProjectAsset, error) {
	f.record("upload", projectID)
	f.paths = append(f.paths, path)
	return "scope", project.ProjectAsset{ProjectID: projectID, Filename: filename, Version: 1}, nil
}

func (f *projectAssetToolGatewayFake) ReplaceAssetFileContent(_ context.Context, projectID, assetID, path, filename string, version uint64) (string, project.ProjectAsset, error) {
	f.record("replace_content", projectID)
	f.paths = append(f.paths, path)
	return "scope", project.ProjectAsset{ProjectID: projectID, AssetID: assetID, Filename: filename, Version: version + 1}, nil
}

func (f *projectAssetToolGatewayFake) PatchAsset(_ context.Context, projectID, assetID string, _ project.PatchProjectAssetInput) (string, error) {
	f.record("update_asset:"+assetID, projectID)
	return "scope", nil
}

func (f *projectAssetToolGatewayFake) DeleteAsset(_ context.Context, projectID, assetID string) (string, error) {
	f.record("delete_asset:"+assetID, projectID)
	return "scope", nil
}

func (f *projectAssetToolGatewayFake) CreateAssetFolder(_ context.Context, projectID string, _ project.CreateProjectAssetFolderInput) (string, error) {
	f.record("create_folder", projectID)
	return "scope", nil
}

func (f *projectAssetToolGatewayFake) PatchAssetFolder(_ context.Context, projectID, folderID string, _ project.PatchProjectAssetFolderInput) (string, error) {
	f.record("update_folder:"+folderID, projectID)
	return "scope", nil
}

func (f *projectAssetToolGatewayFake) DeleteAssetFolder(_ context.Context, projectID, folderID string, _ uint64) (string, error) {
	f.record("delete_folder:"+folderID, projectID)
	return "scope", nil
}

func newProjectAssetToolForTest(root string, gateway projectAssetToolGateway) projectAssetTool {
	return projectAssetTool{
		projectID:     "project-pinned",
		workspaceRoot: root,
		authorize: func(ctx context.Context) (projectAssetToolAccess, error) {
			return projectAssetToolAccess{ctx: ctx, gateway: gateway}, nil
		},
	}
}

func TestProjectAssetToolSchemaPinsProjectAndUsesGatewayFilters(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(projectAssetToolSchema, &schema); err != nil {
		t.Fatalf("decode schema: %v", err)
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema properties = %#v", schema["properties"])
	}
	if _, ok := properties["project_id"]; ok {
		t.Fatal("project_id must not be model-controlled")
	}
	if newProjectAssetToolForTest("", &projectAssetToolGatewayFake{}).ReadOnly() {
		t.Fatal("mixed CRUD tool must use the writer approval path")
	}
	typeProperty := properties["type"].(map[string]any)
	typeValues := strings.Join(anyStrings(typeProperty["enum"]), ",")
	for _, want := range []string{"image", "video", "document", "archive"} {
		if !strings.Contains(typeValues, want) {
			t.Fatalf("asset type enum %q does not contain %q", typeValues, want)
		}
	}
	actionProperty := properties["action"].(map[string]any)
	if actionValues := strings.Join(anyStrings(actionProperty["enum"]), ","); !strings.Contains(actionValues, "download_to_workspace") {
		t.Fatalf("asset action enum %q does not contain download_to_workspace", actionValues)
	}
}

func TestProjectAssetToolDownloadURLPreservesSignedQueryCharacters(t *testing.T) {
	wantURL := "https://assets.example/download?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=a%2Fb&X-Amz-Signature=abc123"
	tool := newProjectAssetToolForTest(t.TempDir(), &projectAssetToolGatewayFake{download: project.ProjectAssetDownload{
		AssetID: "asset-1", Filename: "report.txt", URL: wantURL,
	}})

	got, err := tool.Execute(context.Background(), json.RawMessage(`{"action":"get_download_url","asset_id":"asset-1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, `\u0026`) || !strings.Contains(got, "&") {
		t.Fatalf("encoded signed URL did not preserve ampersands: %s", got)
	}
	var result project.ProjectAssetDownload
	if err := json.Unmarshal([]byte(got), &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.URL != wantURL {
		t.Fatalf("round-tripped URL = %q, want %q", result.URL, wantURL)
	}
}

func TestProjectAssetToolDownloadsToBoundWorkspaceWithoutLeakingURL(t *testing.T) {
	want := []byte("project asset contents")
	wantQuery := "X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=a%2Fb&X-Amz-Signature=abc123"
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.RawQuery != wantQuery {
			t.Errorf("download query = %q, want %q", request.URL.RawQuery, wantQuery)
		}
		_, _ = response.Write(want)
	}))
	defer server.Close()

	root := t.TempDir()
	fake := &projectAssetToolGatewayFake{download: project.ProjectAssetDownload{
		AssetID: "asset-1", Filename: `../escaped-report.txt`, URL: server.URL + "/asset?" + wantQuery,
	}}
	tool := newProjectAssetToolForTest(root, fake)
	tool.newHTTPClient = func(rawURL string) (*http.Client, error) {
		if rawURL != fake.download.URL {
			t.Fatalf("HTTP client URL = %q, want %q", rawURL, fake.download.URL)
		}
		return server.Client(), nil
	}

	got, err := tool.Execute(context.Background(), json.RawMessage(`{"action":"download_to_workspace","asset_id":"asset-1"}`))
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(got), &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result["asset_id"] != "asset-1" || result["filename"] != "escaped-report.txt" || result["relative_path"] != "escaped-report.txt" {
		t.Fatalf("download result = %#v", result)
	}
	if _, exists := result["url"]; exists || strings.Contains(got, server.URL) || strings.Contains(got, "X-Amz-") {
		t.Fatalf("download result leaked signed URL: %s", got)
	}
	if len(fake.projectIDs) != 1 || fake.projectIDs[0] != "project-pinned" {
		t.Fatalf("download gateway project ids = %#v", fake.projectIDs)
	}
	downloaded, err := os.ReadFile(filepath.Join(root, "escaped-report.txt"))
	if err != nil || string(downloaded) != string(want) {
		t.Fatalf("downloaded bytes = %q, err=%v", downloaded, err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "escaped-report.txt")); !os.IsNotExist(err) {
		t.Fatalf("asset escaped workspace boundary: %v", err)
	}
}

func TestProjectAssetToolDownloadRejectsInvalidWorkspaceAndCleansUpFailures(t *testing.T) {
	workspaceFile := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(workspaceFile, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	fake := &projectAssetToolGatewayFake{}
	tool := newProjectAssetToolForTest(workspaceFile, fake)
	if _, err := tool.Execute(context.Background(), json.RawMessage(`{"action":"download_to_workspace","asset_id":"asset-1"}`)); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("invalid workspace error = %v", err)
	}
	if len(fake.actions) != 0 {
		t.Fatalf("invalid workspace reached gateway: %#v", fake.actions)
	}

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Length", "20")
		_, _ = response.Write([]byte("short"))
	}))
	defer server.Close()
	root := t.TempDir()
	fake = &projectAssetToolGatewayFake{download: project.ProjectAssetDownload{AssetID: "asset-1", Filename: "partial.txt", URL: server.URL}}
	tool = newProjectAssetToolForTest(root, fake)
	tool.newHTTPClient = func(string) (*http.Client, error) { return server.Client(), nil }
	if _, err := tool.Execute(context.Background(), json.RawMessage(`{"action":"download_to_workspace","asset_id":"asset-1"}`)); err == nil {
		t.Fatal("partial download unexpectedly succeeded")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		names := make([]string, len(entries))
		for index, entry := range entries {
			names[index] = entry.Name()
		}
		t.Fatalf("failed download left files behind: %#v", names)
	}
}

func TestProjectAssetToolPinsProjectAcrossCRUD(t *testing.T) {
	fake := &projectAssetToolGatewayFake{}
	tool := newProjectAssetToolForTest(t.TempDir(), fake)
	cases := []string{
		`{"action":"list","type":"image","sort":"updated_at","limit":20}`,
		`{"action":"get_download_url","asset_id":"asset-1"}`,
		`{"action":"update_asset","asset_id":"asset-1","expected_version":2,"name":"renamed.txt"}`,
		`{"action":"delete_asset","asset_id":"asset-1"}`,
		`{"action":"create_folder","name":"reports"}`,
		`{"action":"update_folder","folder_id":"folder-1","expected_version":3,"move_to_root":true}`,
		`{"action":"delete_folder","folder_id":"folder-1","expected_version":4}`,
	}
	for _, raw := range cases {
		if _, err := tool.Execute(context.Background(), json.RawMessage(raw)); err != nil {
			t.Fatalf("Execute(%s): %v", raw, err)
		}
	}
	if len(fake.projectIDs) != len(cases) {
		t.Fatalf("gateway calls = %d, want %d", len(fake.projectIDs), len(cases))
	}
	for index, projectID := range fake.projectIDs {
		if projectID != "project-pinned" {
			t.Fatalf("gateway project id at call %d = %q", index, projectID)
		}
	}
}

func TestProjectAssetToolUploadsOnlyWorkspaceFiles(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "source.txt")
	if err := os.WriteFile(inside, []byte("project asset"), 0o600); err != nil {
		t.Fatal(err)
	}
	fake := &projectAssetToolGatewayFake{}
	tool := newProjectAssetToolForTest(root, fake)
	if _, err := tool.Execute(context.Background(), json.RawMessage(`{"action":"upload","source_path":"source.txt","name":"uploaded.txt"}`)); err != nil {
		t.Fatalf("upload: %v", err)
	}
	if _, err := tool.Execute(context.Background(), json.RawMessage(`{"action":"replace_content","asset_id":"asset-1","source_path":"source.txt","expected_version":1}`)); err != nil {
		t.Fatalf("replace_content: %v", err)
	}
	expectedInside, err := filepath.EvalSymlinks(inside)
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.paths) != 2 || fake.paths[0] != expectedInside || fake.paths[1] != expectedInside {
		t.Fatalf("uploaded paths = %#v, want %q twice", fake.paths, expectedInside)
	}

	outsideRoot := t.TempDir()
	outside := filepath.Join(outsideRoot, "secret.txt")
	if err := os.WriteFile(outside, []byte("not in workspace"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := tool.Execute(context.Background(), json.RawMessage(`{"action":"upload","source_path":`+quotedJSON(outside)+`}`)); err == nil || !strings.Contains(err.Error(), "outside the current workspace") {
		t.Fatalf("outside upload error = %v", err)
	}
	if len(fake.paths) != 2 {
		t.Fatalf("outside upload reached gateway: %#v", fake.paths)
	}
}

func TestProjectAssetToolRejectsUnknownAndAmbiguousArguments(t *testing.T) {
	tool := newProjectAssetToolForTest(t.TempDir(), &projectAssetToolGatewayFake{})
	for _, raw := range []string{
		`{"action":"list","project_id":"project-attacker"}`,
		`{"action":"update_asset","asset_id":"asset-1","expected_version":1,"folder_id":"folder-1","move_to_root":true}`,
		`{"action":"update_folder","folder_id":"folder-1","expected_version":1,"parent_folder_id":"folder-2","move_to_root":true}`,
	} {
		if _, err := tool.Execute(context.Background(), json.RawMessage(raw)); err == nil {
			t.Fatalf("Execute(%s) unexpectedly succeeded", raw)
		}
	}
}

func TestCloudProjectSessionToolsAreAbsentFromOrdinaryConversations(t *testing.T) {
	app := &App{}
	tools, err := app.cloudProjectSessionTools(context.Background(), userctx.ActorContext{}, t.TempDir(), "")
	if err != nil {
		t.Fatalf("cloudProjectSessionTools: %v", err)
	}
	if len(tools) != 0 {
		t.Fatalf("ordinary conversation received project tools: %#v", tools)
	}
}

func anyStrings(value any) []string {
	values, _ := value.([]any)
	result := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			result = append(result, text)
		}
	}
	return result
}

func quotedJSON(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

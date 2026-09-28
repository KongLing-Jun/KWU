package project

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"kwu/internal/cloud"
)

func TestProjectGatewayListsAssetDirectoryWithExactQueryAndUsage(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/projects/project-1/asset-directory" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		want := "cursor=cursor-1&direction=desc&folder_id=folder-1&limit=25&search=report&sort=updated_at&type=pdf"
		if r.URL.RawQuery != want {
			t.Fatalf("query = %q, want %q", r.URL.RawQuery, want)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"project_id":"project-1","folder_id":"folder-1","breadcrumbs":[{"folder_id":"folder-1","name":"Reports"}],"items":[{"kind":"asset","id":"asset-1","name":"Q3.pdf","folder_id":"folder-1","parent_folder_id":null,"content_type":"application/pdf","size_bytes":42,"updated_by":{"user_id":"user-1","display_name":"Ada"},"version":3,"updated_at":"2026-08-11T09:00:00+08:00"}],"total_count":1,"next_cursor":null,"usage_bytes":42,"quota_bytes":1024,"quota_plan":"free"},"error":null,"request_id":"req-directory","server_time":"2026-08-11T01:00:01Z"}`)
	}))
	defer server.Close()

	gateway := gatewayForServer(t, server, scope, &gatewayTestExecutor{token: "access-secret"})
	resultScope, page, err := gateway.ListAssetDirectory(context.Background(), "project-1", DirectoryQuery{
		FolderID: "folder-1", Search: " report ", Type: "pdf", Sort: "updated_at", Direction: "desc", Cursor: "cursor-1", Limit: 25,
	})
	if err != nil {
		t.Fatalf("ListAssetDirectory: %v", err)
	}
	if resultScope != scope.Key || page.ProjectID != "project-1" || page.FolderID == nil || *page.FolderID != "folder-1" || page.TotalCount != 1 || page.UsageBytes != 42 || page.QuotaBytes != 1024 || page.QuotaPlan != "free" || len(page.Breadcrumbs) != 1 || len(page.Items) != 1 {
		t.Fatalf("scope=%q page=%#v", resultScope, page)
	}
	item := page.Items[0]
	if item.Kind != "asset" || item.ID != "asset-1" || item.FolderID == nil || *item.FolderID != "folder-1" || item.ContentType == nil || *item.ContentType != "application/pdf" || item.SizeBytes == nil || *item.SizeBytes != 42 || item.UpdatedAt.Location().String() != "UTC" {
		t.Fatalf("item = %#v", item)
	}
}

func TestProjectGatewayAssetDirectoryRejectsAnotherProjectOrFolder(t *testing.T) {
	payload := []byte(`{"data":{"project_id":"project-2","folder_id":null,"breadcrumbs":[],"items":[],"total_count":0,"next_cursor":null,"usage_bytes":0,"quota_bytes":1024,"quota_plan":"free"},"error":null,"request_id":"req-directory","server_time":"2026-08-11T01:00:01Z"}`)
	if _, err := decodeProjectAssetDirectoryPage(payload, "project-1", ""); err == nil {
		t.Fatal("cross-project directory response was accepted")
	}
	payload = []byte(`{"data":{"project_id":"project-1","folder_id":"folder-2","breadcrumbs":[],"items":[],"total_count":0,"next_cursor":null,"usage_bytes":0,"quota_bytes":1024,"quota_plan":"free"},"error":null,"request_id":"req-directory","server_time":"2026-08-11T01:00:01Z"}`)
	if _, err := decodeProjectAssetDirectoryPage(payload, "project-1", "folder-1"); err == nil {
		t.Fatal("wrong-folder directory response was accepted")
	}
}

func TestProjectGatewayAssetDirectoryMutationsPreserveNullableMovesAndEmptyDelete(t *testing.T) {
	scope := gatewayTestScope(t, "user-1", 7)
	type observedRequest struct {
		method string
		path   string
		body   map[string]any
	}
	var requests []observedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil {
			payload, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatal(err)
			}
			if len(payload) > 0 {
				if err := json.Unmarshal(payload, &body); err != nil {
					t.Fatalf("body: %v", err)
				}
			}
		}
		requests = append(requests, observedRequest{method: r.Method, path: r.URL.Path, body: body})
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"ok":true},"error":null,"request_id":"req-mutation","server_time":"2026-08-11T01:00:01Z"}`)
	}))
	defer server.Close()

	gateway := gatewayForServer(t, server, scope, &gatewayTestExecutor{token: "access-secret"})
	root := NullableStringUpdate{Set: true, Value: nil}
	if _, err := gateway.CreateAssetFolder(context.Background(), "project-1", CreateProjectAssetFolderInput{Name: "Reports"}); err != nil {
		t.Fatal(err)
	}
	if _, err := gateway.PatchAssetFolder(context.Background(), "project-1", "folder-1", PatchProjectAssetFolderInput{ExpectedVersion: 2, ParentFolderID: root}); err != nil {
		t.Fatal(err)
	}
	if _, err := gateway.PatchAsset(context.Background(), "project-1", "asset-1", PatchProjectAssetInput{ExpectedVersion: 3, FolderID: root}); err != nil {
		t.Fatal(err)
	}
	if _, err := gateway.DeleteAssetFolder(context.Background(), "project-1", "folder-1", 4); err != nil {
		t.Fatal(err)
	}

	want := []observedRequest{
		{method: http.MethodPost, path: "/api/v1/projects/project-1/asset-folders", body: map[string]any{"name": "Reports", "parent_folder_id": nil}},
		{method: http.MethodPatch, path: "/api/v1/projects/project-1/asset-folders/folder-1", body: map[string]any{"expected_version": float64(2), "parent_folder_id": nil}},
		{method: http.MethodPatch, path: "/api/v1/projects/project-1/assets/asset-1", body: map[string]any{"expected_version": float64(3), "folder_id": nil}},
		{method: http.MethodDelete, path: "/api/v1/projects/project-1/asset-folders/folder-1", body: map[string]any{"expected_version": float64(4)}},
	}
	if !reflect.DeepEqual(requests, want) {
		t.Fatalf("requests = %#v, want %#v", requests, want)
	}
}

func TestBuildProjectAssetMultipartIncludesOptionalFolderID(t *testing.T) {
	folderID := "folder-1"
	body, contentType, err := buildProjectAssetMultipart("note.txt", []byte("hello"), &folderID)
	if err != nil {
		t.Fatal(err)
	}
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatal(err)
	}
	reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	form, err := reader.ReadForm(1024)
	if err != nil {
		t.Fatal(err)
	}
	defer form.RemoveAll()
	if !reflect.DeepEqual(form.Value["folder_id"], []string{"folder-1"}) || len(form.File["file"]) != 1 || form.File["file"][0].Filename != "note.txt" {
		t.Fatalf("form = %#v files=%#v", form.Value, form.File)
	}
}

func TestProjectGatewayStreamsFreshMultipartBodyForAuthenticatedReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.txt")
	if err := os.WriteFile(path, []byte("streamed payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	folderID := "folder-1"
	transport := &replayMultipartTransport{testingT: t}
	scope := gatewayTestScope(t, "user-1", 7)
	gateway, err := NewProjectGateway(
		&gatewayReplayExecutor{tokens: []string{"expired-token", "fresh-token"}},
		transport,
		&gatewayTestScopeResolver{scope: scope},
	)
	if err != nil {
		t.Fatal(err)
	}

	resultScope, asset, err := gateway.UploadAssetFileToFolder(t.Context(), "project-1", path, "report.txt", &folderID)
	if err != nil {
		t.Fatal(err)
	}
	if resultScope != scope.Key || asset.AssetID != "asset-1" {
		t.Fatalf("scope=%q asset=%#v", resultScope, asset)
	}
	transport.mu.Lock()
	defer transport.mu.Unlock()
	if !reflect.DeepEqual(transport.tokens, []string{"expired-token", "fresh-token"}) {
		t.Fatalf("tokens = %#v", transport.tokens)
	}
	if len(transport.payloads) != 2 || len(transport.payloads[0]) == 0 || len(transport.payloads[1]) == 0 {
		t.Fatalf("multipart replay payload lengths = %v", []int{len(transport.payloads[0]), len(transport.payloads[1])})
	}
	if len(transport.contentTypes) != 2 || transport.contentTypes[0] == transport.contentTypes[1] {
		t.Fatalf("content types = %#v, want a fresh multipart boundary per attempt", transport.contentTypes)
	}
}

func TestProjectGatewayReplacesAssetContentWithExpectedVersionOnAuthenticatedReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "replacement.txt")
	if err := os.WriteFile(path, []byte("replacement payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	transport := &replayMultipartTransport{
		testingT:        t,
		method:          http.MethodPut,
		path:            "/api/v1/projects/project-1/assets/asset-1/content",
		expectedVersion: "2",
		expectedContent: "replacement payload",
		response:        json.RawMessage(`{"asset_id":"asset-1","project_id":"project-1","filename":"replacement.txt","content_type":"text/plain","size_bytes":19,"sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","version":3,"work_item_ids":[],"created_at":"2026-08-12T00:00:00Z","updated_at":"2026-08-12T01:00:00Z"}`),
	}
	scope := gatewayTestScope(t, "user-1", 7)
	gateway, err := NewProjectGateway(&gatewayReplayExecutor{tokens: []string{"expired-token", "fresh-token"}}, transport, &gatewayTestScopeResolver{scope: scope})
	if err != nil {
		t.Fatal(err)
	}

	resultScope, asset, err := gateway.ReplaceAssetFileContent(t.Context(), "project-1", "asset-1", path, "replacement.txt", 2)
	if err != nil {
		t.Fatal(err)
	}
	if resultScope != scope.Key || asset.AssetID != "asset-1" || asset.Version != 3 {
		t.Fatalf("scope=%q asset=%#v", resultScope, asset)
	}
	if len(transport.payloads) != 2 || !reflect.DeepEqual(transport.tokens, []string{"expired-token", "fresh-token"}) {
		t.Fatalf("tokens=%#v payloads=%d", transport.tokens, len(transport.payloads))
	}
}

type replayMultipartTransport struct {
	testingT        *testing.T
	mu              sync.Mutex
	tokens          []string
	payloads        [][]byte
	contentTypes    []string
	method          string
	path            string
	expectedVersion string
	expectedContent string
	response        json.RawMessage
}

func (*replayMultipartTransport) DoAuthenticatedJSON(context.Context, string, string, any, string, http.Header, any) error {
	return fmt.Errorf("unexpected JSON request")
}

func (*replayMultipartTransport) DoAuthenticatedJSONQuery(context.Context, string, string, url.Values, any, string, http.Header, any) error {
	return fmt.Errorf("unexpected JSON query request")
}

func (t *replayMultipartTransport) DoAuthenticatedMultipartStreamJSON(ctx context.Context, method, path string, stream func(context.Context) (io.ReadCloser, string, error), token string, _ http.Header, destination any) error {
	expectedMethod := t.method
	if expectedMethod == "" {
		expectedMethod = http.MethodPost
	}
	expectedPath := t.path
	if expectedPath == "" {
		expectedPath = "/api/v1/projects/project-1/assets"
	}
	if method != expectedMethod || path != expectedPath {
		return fmt.Errorf("request = %s %s", method, path)
	}
	body, contentType, err := stream(ctx)
	if err != nil {
		return err
	}
	payload, err := io.ReadAll(body)
	closeErr := body.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return err
	}
	form, err := multipart.NewReader(bytes.NewReader(payload), params["boundary"]).ReadForm(1024)
	if err != nil {
		return err
	}
	defer form.RemoveAll()
	validFields := reflect.DeepEqual(form.Value["folder_id"], []string{"folder-1"})
	if t.expectedVersion != "" {
		validFields = reflect.DeepEqual(form.Value["expected_version"], []string{t.expectedVersion}) && len(form.Value["folder_id"]) == 0
	}
	if !validFields || len(form.File["file"]) != 1 {
		t.testingT.Fatalf("form = %#v files=%#v", form.Value, form.File)
	}
	file, err := form.File["file"][0].Open()
	if err != nil {
		return err
	}
	contents, err := io.ReadAll(file)
	_ = file.Close()
	expectedContent := t.expectedContent
	if expectedContent == "" {
		expectedContent = "streamed payload"
	}
	if err != nil || string(contents) != expectedContent {
		t.testingT.Fatalf("contents=%q err=%v", contents, err)
	}
	t.mu.Lock()
	t.tokens = append(t.tokens, token)
	t.payloads = append(t.payloads, bytes.Clone(payload))
	t.contentTypes = append(t.contentTypes, contentType)
	t.mu.Unlock()
	response := t.response
	if response == nil {
		response = json.RawMessage(`{"asset_id":"asset-1","project_id":"project-1","filename":"report.txt","content_type":"text/plain","size_bytes":16,"sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","version":1,"work_item_ids":[],"created_at":"2026-08-12T00:00:00Z","updated_at":"2026-08-12T00:00:00Z"}`)
	}
	return assignGatewayRaw(destination, response)
}

var _ cloud.AuthenticatedRequestExecutor = (*gatewayReplayExecutor)(nil)

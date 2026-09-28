package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFetchPublicSkillsPreservesCursor(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/api/v1/skills" {
			t.Fatalf("path = %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != "20" {
			t.Fatalf("limit = %q", got)
		}
		if _, exists := r.URL.Query()["type"]; exists {
			t.Fatalf("type query parameter must not be present")
		}
		if got := r.URL.Query().Get("cursor"); got != "@user/demo" {
			t.Fatalf("cursor = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"items":[{"name":"Demo","namespace":{"handle":"user","canonicalName":"@user/demo"},"publisher":{"name":"Demo Team"},"slug":"demo","type":"user","install_count":12,"created_at":1785573878514,"labels":{"requires_api_key":"true"},"categories":["表格处理","数据分析","办公自动化"],"subCategories":[{"key":"tools","name":"工具"}],"tags":null}],"next_cursor":"@user/demo","has_more":true},"error":null,"request_id":"req"}`)
	}))
	defer server.Close()

	page, err := fetchPublicSkills(context.Background(), server.Client(), server.URL, 20, "@user/demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].Slug != "demo" || page.Items[0].Namespace != "user" || page.Items[0].Type != "user" || page.Items[0].InstallCount != 12 || page.Items[0].Publisher != "Demo Team" || page.Items[0].CreatedAt == "" || len(page.Items[0].Labels) != 1 || len(page.Items[0].SubCategories) != 1 || page.NextCursor != "@user/demo" || !page.HasMore {
		t.Fatalf("page = %+v", page)
	}
	if got := strings.Join(page.Items[0].Categories, ","); got != "表格处理,数据分析,办公自动化" {
		t.Fatalf("categories = %q", got)
	}
}

func TestSearchPublicSkillsUsesPublicSearchContract(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Method; got != http.MethodGet {
			t.Errorf("method = %q, want GET", got)
		}
		if got := r.URL.Path; got != "/api/v1/skills/search" {
			t.Errorf("path = %q", got)
		}
		if got := r.URL.Query().Get("keyword"); got != `%_\文档` {
			t.Errorf("keyword = %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != "20" {
			t.Errorf("limit = %q", got)
		}
		if got := r.URL.Query().Get("cursor"); got != "@user/demo" {
			t.Errorf("cursor = %q", got)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("Authorization = %q, want empty", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"items":[],"next_cursor":null,"has_more":false},"error":null,"request_id":"req-search"}`)
	}))
	defer server.Close()

	page, err := searchPublicSkills(context.Background(), server.Client(), server.URL, `  %_\文档  `, 20, "@user/demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 0 || page.NextCursor != "" || page.HasMore {
		t.Fatalf("page = %+v", page)
	}
}

func TestSearchPublicSkillsValidatesUnicodeKeywordLength(t *testing.T) {
	t.Parallel()
	client := &http.Client{}
	if _, err := searchPublicSkills(context.Background(), client, "https://example.com", "", 20, ""); err == nil {
		t.Fatal("empty keyword should fail")
	}
	if _, err := searchPublicSkills(context.Background(), client, "https://example.com", strings.Repeat("文", 129), 20, ""); err == nil {
		t.Fatal("129-character keyword should fail")
	}
}

func TestFetchPublicSkillReturnsPublicError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"data":null,"error":{"code":"NOT_FOUND","message":"请求的资源不存在。","retryable":false},"request_id":"request-404"}`)
	}))
	defer server.Close()

	_, err := fetchPublicSkill(context.Background(), server.Client(), server.URL, "user", "missing")
	if err == nil || !strings.Contains(err.Error(), "请求的资源不存在") || !strings.Contains(err.Error(), "request-404") {
		t.Fatalf("error = %v", err)
	}
}

func TestRecordPublicSkillInstallationPostsEmptyRequestWithoutAtPrefix(t *testing.T) {
	t.Parallel()
	var installCount int64 = 12
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Method; got != http.MethodPost {
			t.Errorf("method = %q, want POST", got)
		}
		if got := r.URL.EscapedPath(); got != "/api/v1/skills/tencent-adm/cloudbase/installations" {
			t.Errorf("path = %q", got)
		}
		if r.URL.RawQuery != "" {
			t.Errorf("query = %q, want empty", r.URL.RawQuery)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("Authorization = %q, want empty", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		if len(body) != 0 {
			t.Errorf("body = %q, want empty", body)
		}
		installCount++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"installation": map[string]any{
				"canonical_name": "@tencent-adm/cloudbase",
				"install_count":  installCount,
			}},
			"error": nil, "request_id": "req-install",
		})
	}))
	defer server.Close()

	for want := int64(13); want <= 14; want++ {
		installation, err := recordPublicSkillInstallation(context.Background(), server.Client(), server.URL, "@tencent-adm", "cloudbase")
		if err != nil {
			t.Fatal(err)
		}
		if installation.CanonicalName != "@tencent-adm/cloudbase" || installation.InstallCount != want {
			t.Fatalf("installation = %+v, want count %d", installation, want)
		}
	}
}

func TestRecordPublicSkillInstallationReturnsPublicError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"data":null,"error":{"code":"NOT_FOUND","message":"requested resource does not exist","retryable":false},"request_id":"request-install-404"}`)
	}))
	defer server.Close()

	_, err := recordPublicSkillInstallation(context.Background(), server.Client(), server.URL, "tencent-adm", "missing")
	if err == nil || !strings.Contains(err.Error(), "requested resource does not exist") || !strings.Contains(err.Error(), "request-install-404") {
		t.Fatalf("error = %v", err)
	}
}

func TestInstallPublicSkillArchiveVerifiesAndExtracts(t *testing.T) {
	t.Parallel()
	archive := publicSkillTestArchive(t, map[string]string{
		"web-tools-guide/SKILL.md":            "---\nname: 视频号爆款短视频拆解（付费版：全能）【零一数科·出品】\ndescription: Web tools\n---\n\nUse the web safely.\n",
		"web-tools-guide/references/guide.md": "# Guide\n",
	})
	sum := sha256.Sum256(archive)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(archive) }))
	defer server.Close()
	root := t.TempDir()
	iconURL := "https://cdn.example.com/skills/web-tools-guide.png"

	path, name, err := installPublicSkillArchive(context.Background(), server.Client(), PublicSkillView{
		Namespace: "example", Slug: "web-tools-guide", Name: "网页工具指南", Version: "1.2.3", IconURL: &iconURL,
		DownloadURL: server.URL, DownloadSizeBytes: int64(len(archive)), DownloadSHA256: hex.EncodeToString(sum[:]),
	}, root)
	if err != nil {
		t.Fatal(err)
	}
	if name != "web-tools-guide" || path != filepath.Join(root, "web-tools-guide", "SKILL.md") {
		t.Fatalf("name/path = %q %q", name, path)
	}
	if _, err := os.Stat(filepath.Join(root, "web-tools-guide", "references", "guide.md")); err != nil {
		t.Fatalf("reference missing: %v", err)
	}
	if got := installedPublicSkillIconURL(path); got != iconURL {
		t.Fatalf("installed icon URL = %q, want %q", got, iconURL)
	}
	metadata, ok := installedPublicSkillMetadata(path)
	if !ok || metadata.Name != "网页工具指南" {
		t.Fatalf("installed metadata = %+v, ok=%v; want persisted backend name", metadata, ok)
	}
	if got := installedSkillDisplayName(path, name, metadata); got != "网页工具指南" {
		t.Fatalf("installed display name = %q, want backend name", got)
	}
}

func TestInstalledSkillDisplayNameFallsBackToLegacyFrontmatter(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "SKILL.md")
	if err := os.WriteFile(path, []byte("---\nname: 股票价值投资分析系统\ndescription: test\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := installedSkillDisplayName(path, "valuation-analysis", publicSkillInstallMetadata{}); got != "股票价值投资分析系统" {
		t.Fatalf("legacy display name = %q", got)
	}
}

func TestInstallPublicSkillArchiveIgnoresStaleIntegrityMetadata(t *testing.T) {
	t.Parallel()
	archive := publicSkillTestArchive(t, map[string]string{"SKILL.md": "---\ndescription: test\n---\nbody"})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(archive) }))
	defer server.Close()

	path, name, err := installPublicSkillArchive(context.Background(), server.Client(), PublicSkillView{
		Slug: "demo", DownloadURL: server.URL, DownloadSizeBytes: 1, DownloadSHA256: strings.Repeat("0", 64),
	}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if name != "demo" || filepath.Base(path) != "SKILL.md" {
		t.Fatalf("name/path = %q %q", name, path)
	}
}

func TestUninstallPublicSkillMovesManagedSkillToRecoveryDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	trash := filepath.Join(t.TempDir(), "recovery")
	target := filepath.Join(root, "demo")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("---\ndescription: demo\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writePublicSkillInstallMetadata(target, PublicSkillView{Namespace: "example", Slug: "demo", Version: "1.0.0"}); err != nil {
		t.Fatal(err)
	}

	if err := uninstallPublicSkill(root, trash, "demo"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("installed target still exists: %v", err)
	}
	entries, err := os.ReadDir(trash)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || !strings.HasPrefix(entries[0].Name(), "demo-") {
		t.Fatalf("recovery entries = %v", entries)
	}
	if _, err := os.Stat(filepath.Join(trash, entries[0].Name(), "SKILL.md")); err != nil {
		t.Fatalf("recovered skill missing: %v", err)
	}
}

func TestUninstallPublicSkillRejectsUnmanagedDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	target := filepath.Join(root, "custom-demo")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("---\ndescription: custom\n---\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := uninstallPublicSkill(root, filepath.Join(t.TempDir(), "recovery"), "custom-demo")
	if err == nil || !strings.Contains(err.Error(), "只能卸载由技能库安装的技能") {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("unmanaged skill was moved: %v", err)
	}
}

func TestCommitPublicSkillDirectoryRetriesTransientPermissionFailure(t *testing.T) {
	t.Parallel()
	attempts := 0
	err := commitPublicSkillDirectory(context.Background(), "source", "target", func(source, target string) error {
		attempts++
		if source != "source" || target != "target" {
			t.Fatalf("rename arguments = %q %q", source, target)
		}
		if attempts < 3 {
			return &os.LinkError{Op: "rename", Old: source, New: target, Err: fs.ErrPermission}
		}
		return nil
	}, []time.Duration{0, 0})
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 3 {
		t.Fatalf("rename attempts = %d, want 3", attempts)
	}
}

func TestCommitPublicSkillDirectoryStopsWhenContextIsCancelled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	attempts := 0
	err := commitPublicSkillDirectory(ctx, "source", "target", func(string, string) error {
		attempts++
		return fs.ErrPermission
	}, []time.Duration{time.Hour})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context cancellation", err)
	}
	if attempts != 1 {
		t.Fatalf("rename attempts = %d, want 1", attempts)
	}
}

func TestExtractPublicSkillArchiveRejectsTraversal(t *testing.T) {
	t.Parallel()
	archive := publicSkillTestArchive(t, map[string]string{
		"demo/SKILL.md":      "---\ndescription: test\n---\nbody",
		"demo/../../evil.md": "nope",
	})
	if err := extractPublicSkillArchive(archive, t.TempDir()); err == nil || !strings.Contains(err.Error(), "不安全路径") {
		t.Fatalf("error = %v", err)
	}
}

func publicSkillTestArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

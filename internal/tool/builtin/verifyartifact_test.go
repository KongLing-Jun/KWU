package builtin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type verifiedArtifactResult struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

func verifiedArtifactsFromOutput(t *testing.T, out string) []verifiedArtifactResult {
	t.Helper()
	const prefix = "verified workspace artifacts: "
	if !strings.HasPrefix(out, prefix) {
		t.Fatalf("verification output = %q, want %q prefix", out, prefix)
	}
	var got []verifiedArtifactResult
	if err := json.Unmarshal([]byte(strings.TrimPrefix(out, prefix)), &got); err != nil {
		t.Fatalf("decode verification output %q: %v", out, err)
	}
	return got
}

func TestVerifyArtifactAcceptsWorkspaceRelativeUnicodeFile(t *testing.T) {
	root := t.TempDir()
	name := "中文 游戏.html"
	path := filepath.Join(root, name)
	content := []byte("<!doctype html>")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}

	verify := verifyArtifact{workDir: root, roots: realRoots([]string{root})}
	out, err := verify.Execute(context.Background(), argsJSON(t, map[string]any{
		"paths":               []string{name},
		"expected_extensions": []string{".HTML"},
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got := verifiedArtifactsFromOutput(t, out)
	canonical, err := realPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != canonical || got[0].Size != int64(len(content)) {
		t.Fatalf("verified artifacts = %+v, want path %q size %d", got, canonical, len(content))
	}
}

func TestVerifyArtifactAcceptsWorkspaceAbsoluteFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "report.PDF")
	if err := os.WriteFile(path, []byte("pdf"), 0o644); err != nil {
		t.Fatal(err)
	}

	verify := verifyArtifact{workDir: root, roots: realRoots([]string{root})}
	out, err := verify.Execute(context.Background(), argsJSON(t, map[string]any{
		"paths":               []string{path},
		"expected_extensions": []string{".pdf"},
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := verifiedArtifactsFromOutput(t, out)
	if len(got) != 1 || !filepath.IsAbs(got[0].Path) {
		t.Fatalf("verified artifacts = %+v, want one absolute path", got)
	}
}

func TestVerifyArtifactRejectsInvalidWorkspaceArtifacts(t *testing.T) {
	outer := t.TempDir()
	root := filepath.Join(outer, "workspace")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "folder.html"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "artifact.txt"), []byte("text"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(outer, "escape.html")
	if err := os.WriteFile(outside, []byte("escape"), 0o644); err != nil {
		t.Fatal(err)
	}

	verify := verifyArtifact{workDir: root, roots: realRoots([]string{root})}
	tests := []struct {
		name    string
		path    string
		wantErr string
	}{
		{name: "missing", path: "missing.html", wantErr: "stat artifact"},
		{name: "directory", path: "folder.html", wantErr: "not a regular file"},
		{name: "extension mismatch", path: "artifact.txt", wantErr: "does not match expected extensions"},
		{name: "dot dot escape", path: filepath.Join("..", "escape.html"), wantErr: "outside the readable roots"},
		{name: "absolute outside", path: outside, wantErr: "outside the readable roots"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := verify.Execute(context.Background(), argsJSON(t, map[string]any{
				"paths":               []string{tt.path},
				"expected_extensions": []string{".html"},
			}))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Execute(%q) error = %v, want containing %q", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestVerifyArtifactRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.html")
	if err := os.WriteFile(outside, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "linked.html")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	verify := verifyArtifact{workDir: root, roots: realRoots([]string{root})}
	_, err := verify.Execute(context.Background(), argsJSON(t, map[string]any{"paths": []string{"linked.html"}}))
	if err == nil || !strings.Contains(err.Error(), "outside the readable roots") {
		t.Fatalf("symlink escape error = %v, want readable-root confinement", err)
	}
}

func TestVerifyArtifactFailsAfterArtifactDeletion(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "ephemeral.html")
	if err := os.WriteFile(path, []byte("temporary"), 0o644); err != nil {
		t.Fatal(err)
	}
	verify := verifyArtifact{workDir: root, roots: realRoots([]string{root})}
	args := argsJSON(t, map[string]any{"paths": []string{"ephemeral.html"}})
	if _, err := verify.Execute(context.Background(), args); err != nil {
		t.Fatalf("initial verification: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := verify.Execute(context.Background(), args); err == nil {
		t.Fatal("verification after artifact deletion should fail")
	}
}

func TestWorkspaceToolsBindVerifyArtifact(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "bound.html")
	if err := os.WriteFile(path, []byte("bound"), 0o644); err != nil {
		t.Fatal(err)
	}

	verify := byName((Workspace{Dir: root}).Tools("verify_artifact"))["verify_artifact"]
	if verify == nil {
		t.Fatal("Workspace.Tools missing verify_artifact")
	}
	if !verify.ReadOnly() {
		t.Fatal("verify_artifact must be read-only")
	}
	if _, err := verify.Execute(context.Background(), argsJSON(t, map[string]any{
		"paths":               []string{"bound.html"},
		"expected_extensions": []string{".html"},
	})); err != nil {
		t.Fatalf("workspace-bound verification: %v", err)
	}
}

func TestWorkspaceVerifyArtifactIgnoresAdditionalWriteRoots(t *testing.T) {
	outer := t.TempDir()
	workspace := filepath.Join(outer, "workspace")
	additionalWriteRoot := filepath.Join(outer, "allowed-write")
	for _, dir := range []string{workspace, additionalWriteRoot} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(workspace, "inside.html"), []byte("inside"), 0o644); err != nil {
		t.Fatal(err)
	}
	outsideArtifact := filepath.Join(additionalWriteRoot, "outside.html")
	if err := os.WriteFile(outsideArtifact, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}

	tools := byName((Workspace{
		Dir:        workspace,
		WriteRoots: []string{workspace, additionalWriteRoot},
	}).Tools("verify_artifact", "read_file", "write_file"))

	if _, err := tools["verify_artifact"].Execute(context.Background(), argsJSON(t, map[string]any{
		"paths": []string{"inside.html"},
	})); err != nil {
		t.Fatalf("verify workspace artifact: %v", err)
	}
	if _, err := tools["verify_artifact"].Execute(context.Background(), argsJSON(t, map[string]any{
		"paths": []string{outsideArtifact},
	})); err == nil || !strings.Contains(err.Error(), "outside the readable roots") {
		t.Fatalf("verify artifact in additional write root error = %v, want workspace-only confinement", err)
	}

	if _, err := tools["read_file"].Execute(context.Background(), argsJSON(t, map[string]any{
		"path": outsideArtifact,
	})); err != nil {
		t.Fatalf("read_file should retain additional-root access: %v", err)
	}
	additionalOutput := filepath.Join(additionalWriteRoot, "generated.txt")
	if _, err := tools["write_file"].Execute(context.Background(), argsJSON(t, map[string]any{
		"path": additionalOutput, "content": "generated",
	})); err != nil {
		t.Fatalf("write_file should retain additional-root access: %v", err)
	}
}

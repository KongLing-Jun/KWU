package builtin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"kwu/internal/provider"
	"kwu/internal/tool"
)

func TestDeliverFilesReturnsCanonicalStructuredPaths(t *testing.T) {
	root := t.TempDir()
	relative := "中文 成品.html"
	absolute := filepath.Join(root, "report.pdf")
	for path, content := range map[string]string{
		filepath.Join(root, relative): "html",
		absolute:                      "pdf",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	deliver := deliverFiles{workDir: root, roots: realRoots([]string{root})}
	out, err := deliver.Execute(context.Background(), argsJSON(t, map[string]any{
		"paths": []string{relative, absolute},
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got, ok := DecodeDeliverFilesResult(out)
	if !ok {
		t.Fatalf("DecodeDeliverFilesResult(%q) failed", out)
	}
	want := make([]string, 2)
	for i, path := range []string{filepath.Join(root, relative), absolute} {
		want[i], err = realPath(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("delivered paths = %v, want %v", got, want)
	}
}

func TestDeliverFilesAcceptsExplicitEmptyManifest(t *testing.T) {
	deliver := deliverFiles{workDir: t.TempDir()}
	out, err := deliver.Execute(context.Background(), json.RawMessage(`{"paths":[]}`))
	if err != nil {
		t.Fatalf("Execute explicit empty manifest: %v", err)
	}
	got, ok := DecodeDeliverFilesResult(out)
	if !ok || got == nil || len(got) != 0 {
		t.Fatalf("DecodeDeliverFilesResult(%q) = %#v, %v; want non-nil empty manifest", out, got, ok)
	}
}

func TestDeliverFilesRequiresNonNullPathsArray(t *testing.T) {
	deliver := deliverFiles{workDir: t.TempDir()}
	for _, args := range []string{`{}`, `{"paths":null}`} {
		if _, err := deliver.Execute(context.Background(), json.RawMessage(args)); err == nil || !strings.Contains(err.Error(), "non-null array") {
			t.Fatalf("Execute(%s) error = %v, want non-null paths array error", args, err)
		}
	}
}

func TestDeliverFilesRejectsInvalidSetWithoutPartialSuccess(t *testing.T) {
	outer := t.TempDir()
	root := filepath.Join(outer, "workspace")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "valid.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	deliver := deliverFiles{workDir: root, roots: realRoots([]string{root})}
	out, err := deliver.Execute(context.Background(), argsJSON(t, map[string]any{
		"paths": []string{"valid.txt", "missing.txt"},
	}))
	if err == nil || !strings.Contains(err.Error(), "stat artifact") {
		t.Fatalf("Execute error = %v, want missing artifact error", err)
	}
	if out != "" {
		t.Fatalf("failed delivery returned partial output %q", out)
	}
}

func TestDeliverFilesRejectsWorkspaceEscape(t *testing.T) {
	outer := t.TempDir()
	root := filepath.Join(outer, "workspace")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(outer, "outside.txt")
	if err := os.WriteFile(outside, []byte("no"), 0o644); err != nil {
		t.Fatal(err)
	}

	deliver := deliverFiles{workDir: root, roots: realRoots([]string{root})}
	if _, err := deliver.Execute(context.Background(), argsJSON(t, map[string]any{
		"paths": []string{outside},
	})); err == nil || !strings.Contains(err.Error(), "outside the readable roots") {
		t.Fatalf("Execute escape error = %v, want workspace confinement", err)
	}
}

func TestDeliverFilesContractIsReadOnlyAndExecutionOnly(t *testing.T) {
	builtin, ok := tool.LookupBuiltin("deliver_files")
	if !ok {
		t.Fatal("deliver_files builtin not registered")
	}
	if !builtin.ReadOnly() {
		t.Fatal("deliver_files must be read-only to avoid approval for declaration")
	}
	classifier, ok := builtin.(tool.PlanModeClassifier)
	if !ok || classifier.PlanModeSafe() {
		t.Fatal("deliver_files must explicitly reject plan-mode execution")
	}
}

func TestDeliverFilesDescriptionDefinesResultBoundary(t *testing.T) {
	description := (deliverFiles{}).Description()
	for _, want := range []string{
		"file itself is a result",
		"open, use, download, share, or continue editing",
		"existing file modified in place",
		"source code, tests, configuration, or migrations",
		"workspace changes, not deliverables",
		"particular file as a standalone result",
		"complete final-deliverable manifest",
		"empty paths array to explicitly deliver no files",
	} {
		if !strings.Contains(description, want) {
			t.Fatalf("deliver_files description missing %q:\n%s", want, description)
		}
	}
}

func TestDeliverFilesRejectsMoreThanContractLimit(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "artifact.txt")
	if err := os.WriteFile(path, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	paths := make([]string, provider.MaxArtifactDeliveryPaths+1)
	for i := range paths {
		paths[i] = path
	}

	deliver := deliverFiles{workDir: root, roots: realRoots([]string{root})}
	if _, err := deliver.Execute(context.Background(), argsJSON(t, map[string]any{
		"paths": paths,
	})); err == nil || !strings.Contains(err.Error(), "maximum is 32") {
		t.Fatalf("Execute oversized manifest error = %v, want shared contract limit", err)
	}
}

func TestDecodeDeliverFilesResultRejectsUnrelatedOrInvalidOutput(t *testing.T) {
	absolute := filepath.Join(t.TempDir(), "file.txt")
	valid, err := json.Marshal(map[string]any{
		"type":  DeliverFilesResultType,
		"paths": []string{absolute},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := DecodeDeliverFilesResult(string(valid)); !ok || !reflect.DeepEqual(got, []string{absolute}) {
		t.Fatalf("valid decode = %v, %v", got, ok)
	}
	empty, err := json.Marshal(map[string]any{
		"type":  DeliverFilesResultType,
		"paths": []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := DecodeDeliverFilesResult(string(empty)); !ok || got == nil || len(got) != 0 {
		t.Fatalf("valid empty decode = %#v, %v", got, ok)
	}
	for _, output := range []string{
		`{"type":"other","paths":["/file.txt"]}`,
		`{"type":"deliver_files_result"}`,
		`{"type":"deliver_files_result","paths":null}`,
		`{"type":"deliver_files_result","paths":["relative.txt"]}`,
		`{"type":"deliver_files_result","paths":[`,
	} {
		if got, ok := DecodeDeliverFilesResult(output); ok || got != nil {
			t.Fatalf("DecodeDeliverFilesResult(%q) = %v, %v; want nil, false", output, got, ok)
		}
	}
}

func TestWorkspaceToolsBindDeliverFilesToOwningWorkspace(t *testing.T) {
	outer := t.TempDir()
	root := filepath.Join(outer, "workspace")
	additionalWriteRoot := filepath.Join(outer, "additional-write-root")
	for _, dir := range []string{root, additionalWriteRoot} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(root, "bound.txt")
	if err := os.WriteFile(path, []byte("bound"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(additionalWriteRoot, "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}

	deliver := byName((Workspace{Dir: root, WriteRoots: []string{root, additionalWriteRoot}}).Tools("deliver_files"))["deliver_files"]
	if deliver == nil {
		t.Fatal("Workspace.Tools missing deliver_files")
	}
	out, err := deliver.Execute(context.Background(), argsJSON(t, map[string]any{"paths": []string{"bound.txt"}}))
	if err != nil {
		t.Fatalf("workspace-bound delivery: %v", err)
	}
	if _, ok := DecodeDeliverFilesResult(out); !ok {
		t.Fatalf("workspace-bound delivery output = %q", out)
	}
	if _, err := deliver.Execute(context.Background(), argsJSON(t, map[string]any{"paths": []string{outside}})); err == nil || !strings.Contains(err.Error(), "outside the readable roots") {
		t.Fatalf("delivery from additional write root error = %v, want owning-workspace confinement", err)
	}
}

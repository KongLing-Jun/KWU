package automation

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

func TestAutomationPackageImportBoundaries(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("Glob returned error: %v", err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("ParseFile(%s) returned error: %v", file, err)
		}
		for _, spec := range f.Imports {
			path := strings.Trim(spec.Path.Value, `"`)
			if strings.HasPrefix(path, "kwu/desktop") || strings.HasPrefix(path, "kwu/internal/serve") || strings.HasPrefix(path, "kwu/internal/provider/") {
				t.Fatalf("%s imports forbidden package %q", file, path)
			}
		}
	}
}

package agent

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Session.Messages remains exported for the agent package's hot run-loop reads.
// Keep production writers outside this package on Add/Replace/constructors so
// dirty-generation tracking cannot silently miss a persisted change.
func TestProductionSessionMessagesMutationsUseMethods(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	for _, base := range []string{"internal", "desktop", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, base), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			cleanPath := filepath.Clean(path)
			if cleanPath == filepath.Join(root, "internal", "agent", "session.go") ||
				cleanPath == filepath.Join(root, "internal", "agent", "save.go") {
				return nil
			}
			fileSet := token.NewFileSet()
			file, err := parser.ParseFile(fileSet, path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(file, func(node ast.Node) bool {
				switch value := node.(type) {
				case *ast.AssignStmt:
					for _, expression := range value.Lhs {
						if selector, ok := expression.(*ast.SelectorExpr); ok && selector.Sel.Name == "Messages" {
							t.Errorf("%s writes .Messages directly; use Session.Add or Session.Replace", fileSet.Position(selector.Pos()))
						}
					}
				case *ast.CompositeLit:
					selector, ok := value.Type.(*ast.SelectorExpr)
					if !ok || selector.Sel.Name != "Session" {
						return true
					}
					for _, element := range value.Elts {
						if pair, ok := element.(*ast.KeyValueExpr); ok {
							if name, ok := pair.Key.(*ast.Ident); ok && name.Name == "Messages" {
								t.Errorf("%s initializes agent.Session.Messages directly; use NewSessionFromMessages", fileSet.Position(name.Pos()))
							}
						}
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s: %v", base, err)
		}
	}
}

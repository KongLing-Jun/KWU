package cli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestRegularCLIModesDoNotStartAutomationEngine(t *testing.T) {
	file := parseCLISource(t)
	for _, name := range []string{"runAgent", "chatREPL"} {
		fn := findFunc(t, file, name)
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.SelectorExpr:
				if id, ok := n.X.(*ast.Ident); ok && id.Name == "automation" {
					t.Fatalf("%s references automation.%s; regular CLI modes must not start automation", name, n.Sel.Name)
				}
			case *ast.CallExpr:
				if id, ok := n.Fun.(*ast.Ident); ok && id.Name == "newServeAutomation" {
					t.Fatalf("%s calls newServeAutomation; regular CLI modes must not start automation", name)
				}
			}
			return true
		})
	}
}

func TestServeAutomationIsOnlyStartedByServeCommand(t *testing.T) {
	file := parseCLISource(t)
	calls := map[string]int{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "newServeAutomation" {
				calls[fn.Name.Name]++
			}
			return true
		})
	}
	if len(calls) != 1 || calls["runServe"] != 1 {
		t.Fatalf("newServeAutomation calls = %+v, want exactly one call from runServe", calls)
	}
}

func parseCLISource(t *testing.T) *ast.File {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "cli.go", nil, 0)
	if err != nil {
		t.Fatalf("ParseFile cli.go: %v", err)
	}
	return file
}

func findFunc(t *testing.T, file *ast.File, name string) *ast.FuncDecl {
	t.Helper()
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == name {
			return fn
		}
	}
	t.Fatalf("function %s not found", name)
	return nil
}

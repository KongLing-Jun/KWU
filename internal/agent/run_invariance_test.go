package agent

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func TestAgentRunHasNoActorProfileBranch(t *testing.T) {
	src, err := os.ReadFile("agent.go")
	if err != nil {
		t.Fatalf("read agent.go: %v", err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "agent.go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse agent.go: %v", err)
	}
	var run *ast.FuncDecl
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "Run" || fn.Recv == nil || len(fn.Recv.List) != 1 {
			continue
		}
		recv := string(src[fset.Position(fn.Recv.List[0].Pos()).Offset:fset.Position(fn.Recv.List[0].End()).Offset])
		if strings.Contains(recv, "*Agent") {
			run = fn
			break
		}
	}
	if run == nil {
		t.Fatal("Agent.Run not found")
	}
	body := string(src[fset.Position(run.Body.Pos()).Offset:fset.Position(run.Body.End()).Offset])
	for _, forbidden := range []string{"ActorContext", "ProfileID", "CredentialOwner", "userctx"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("Agent.Run contains user/profile-specific branch marker %q:\n%s", forbidden, body)
		}
	}
}

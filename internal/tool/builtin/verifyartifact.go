package builtin

import (
	"context"
	"encoding/json"
	"fmt"

	"kwu/internal/tool"
)

func init() { tool.RegisterBuiltin(verifyArtifact{}) }

// verifyArtifact confirms that artifact paths currently name regular files
// inside the owning workspace. It does not inspect or mutate file contents.
type verifyArtifact struct {
	workDir string
	roots   []string
}

func (verifyArtifact) Name() string { return "verify_artifact" }

func (verifyArtifact) Description() string {
	return "Verify that one or more generated artifacts currently exist as regular files inside the workspace, optionally requiring specific filename extensions. Returns each artifact's canonical absolute path and byte size."
}

func (verifyArtifact) Schema() json.RawMessage {
	return json.RawMessage(`{
"type":"object",
"properties":{
  "paths":{"type":"array","items":{"type":"string"},"minItems":1,"description":"Workspace-relative or workspace-contained absolute artifact paths"},
  "expected_extensions":{"type":"array","items":{"type":"string"},"description":"Optional allowed filename extensions such as .html or .pdf"}
},
"required":["paths"]
}`)
}

func (verifyArtifact) ReadOnly() bool { return true }

func (v verifyArtifact) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Paths              []string `json:"paths"`
		ExpectedExtensions []string `json:"expected_extensions,omitempty"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if len(p.Paths) == 0 {
		return "", fmt.Errorf("paths is required")
	}

	verified, err := verifyWorkspaceArtifacts(v.workDir, v.roots, p.Paths, p.ExpectedExtensions)
	if err != nil {
		return "", err
	}

	b, _ := json.Marshal(verified)
	return "verified workspace artifacts: " + string(b), nil
}

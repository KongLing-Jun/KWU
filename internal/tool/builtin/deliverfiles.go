package builtin

import (
	"context"
	"encoding/json"
	"fmt"

	"kwu/internal/evidence"
	"kwu/internal/provider"
	"kwu/internal/tool"
)

const (
	// DeliverFilesResultType identifies the stable result contract consumed by
	// hosts that project successful deliveries into file cards and history.
	DeliverFilesResultType = evidence.DeliverFilesResultType

	// Keep successful results below the agent's 32 KiB transcript truncation
	// boundary so the same complete JSON survives in realtime and history.
	maxDeliverFilesResultBytes = 24 * 1024
)

type deliverFiles struct {
	workDir string
	roots   []string
}

type deliverFilesResult struct {
	Type  string   `json:"type"`
	Paths []string `json:"paths"`
}

func init() { tool.RegisterBuiltin(deliverFiles{}) }

func (deliverFiles) Name() string { return "deliver_files" }

func (deliverFiles) Description() string {
	return "Declare the complete final-deliverable manifest after all writing, checks, delegated work, and background waits are complete. A file is a deliverable when the file itself is a result the user is expected to open, use, download, share, or continue editing. This includes an existing file modified in place when it is the requested result. Routine repository implementation changes, such as source code, tests, configuration, or migrations, are workspace changes, not deliverables, unless the user explicitly requests a particular file as a standalone result. Every listed path must be an existing regular file in the owning workspace; use an empty paths array to explicitly deliver no files. Returns canonical absolute paths as stable JSON."
}

func (deliverFiles) Schema() json.RawMessage {
	return json.RawMessage(`{
"type":"object",
"properties":{
  "paths":{"type":"array","items":{"type":"string"},"maxItems":32,"description":"Complete final manifest of workspace-relative or workspace-contained files; use an empty array when this turn delivers no files"}
},
"required":["paths"]
}`)
}

// ReadOnly is true because delivery validates and declares existing files but
// does not mutate them. The agent still schedules it serially as a workflow
// boundary, and PlanModeSafe keeps it out of planning.
func (deliverFiles) ReadOnly() bool { return true }

func (deliverFiles) PlanModeSafe() bool { return false }

func (d deliverFiles) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Paths *[]string `json:"paths"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if p.Paths == nil {
		return "", fmt.Errorf("paths must be a non-null array")
	}
	if len(*p.Paths) > provider.MaxArtifactDeliveryPaths {
		return "", fmt.Errorf("paths has %d items; maximum is %d", len(*p.Paths), provider.MaxArtifactDeliveryPaths)
	}

	verified, err := verifyWorkspaceArtifacts(d.workDir, d.roots, *p.Paths, nil)
	if err != nil {
		return "", err
	}
	paths := make([]string, len(verified))
	for i, artifact := range verified {
		paths[i] = artifact.Path
	}
	encoded, err := json.Marshal(deliverFilesResult{Type: DeliverFilesResultType, Paths: paths})
	if err != nil {
		return "", fmt.Errorf("encode delivered files: %w", err)
	}
	if len(encoded) > maxDeliverFilesResultBytes {
		return "", fmt.Errorf("deliver_files result is %d bytes (limit %d); reduce the number or length of the final paths", len(encoded), maxDeliverFilesResultBytes)
	}
	return string(encoded), nil
}

// DecodeDeliverFilesResult decodes the stable success result emitted by
// deliver_files. It rejects unrelated, truncated, or structurally invalid tool
// output so hosts never need to duplicate the JSON contract.
func DecodeDeliverFilesResult(output string) ([]string, bool) {
	return evidence.DecodeDeliverFilesResult(output)
}

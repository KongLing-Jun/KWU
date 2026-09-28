package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	projectcontract "kwu/internal/project"
)

func TestProjectsV1CapabilityDoesNotChangeLocalWorkspaceBootstrap(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	root := t.TempDir()
	if err := saveProjectsFile(desktopProjectFile{Projects: []desktopProject{{Root: root, Title: "Local only"}}}); err != nil {
		t.Fatalf("save local workspace registry: %v", err)
	}
	registryPath := filepath.Join(desktopConfigDir(), desktopProjectsFile)
	wantRegistry, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatalf("read local workspace registry: %v", err)
	}

	valid := `{"enabled":true,"contract_version":"` + projectcontract.ContractVersion + `","contract_hash_algorithm":"sha256","contract_hash":"` + projectcontract.CanonicalContractSHA256() + `","instructions":true,"work_items":true}`
	cases := []struct {
		name string
		raw  json.RawMessage
	}{
		{name: "absent", raw: nil},
		{name: "disabled", raw: json.RawMessage(`{"enabled":false}`)},
		{name: "supported", raw: json.RawMessage(valid)},
		{name: "unsupported version", raw: json.RawMessage(strings.Replace(valid, projectcontract.ContractVersion, "projects-v2", 1))},
		{name: "same version different hash", raw: json.RawMessage(strings.Replace(valid, projectcontract.CanonicalContractSHA256(), strings.Repeat("0", 64), 1))},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, _ = projectcontract.EvaluateCapability(test.raw)
			workspaces := app.ListWorkspaces()
			if len(workspaces) != 1 || workspaces[0].Path != normalizeProjectRoot(root) || workspaces[0].Name != "Local only" {
				t.Fatalf("ListWorkspaces after %s capability = %#v", test.name, workspaces)
			}
			gotRegistry, err := os.ReadFile(registryPath)
			if err != nil {
				t.Fatalf("read local workspace registry after %s capability: %v", test.name, err)
			}
			if !bytes.Equal(gotRegistry, wantRegistry) {
				t.Fatalf("desktop-projects.json changed after %s capability", test.name)
			}
		})
	}
}

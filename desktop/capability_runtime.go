package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"html"
	"io/fs"
	"strings"

	"kwu/desktop/internal/capabilitypackage"
	"kwu/internal/plugin"
)

const (
	crossborderCommerceCapabilityID = "crossborder-commerce"
	// The result projector is an existing domain adapter keyed by the provider
	// namespace. Runtime registration itself is resolved from the package manifest.
	shopifyGlobalCatalogServer = "shopify-global-catalog"
)

//go:embed capabilities/crossborder-commerce/*
var builtinCapabilityPackages embed.FS

func resolveCapabilityPackage(capabilityID string) (capabilitypackage.Package, bool, error) {
	capabilityID = strings.TrimSpace(capabilityID)
	if capabilityID == "" {
		return capabilitypackage.Package{}, false, nil
	}
	if capabilityID != crossborderCommerceCapabilityID {
		return capabilitypackage.Package{}, false, nil
	}
	source, err := fs.Sub(builtinCapabilityPackages, "capabilities")
	if err != nil {
		return capabilitypackage.Package{}, false, err
	}
	pkg, err := capabilitypackage.Load(source, capabilityID+"/capability.json")
	if err != nil {
		return capabilitypackage.Package{}, false, err
	}
	if pkg.Manifest.ID != capabilityID {
		return capabilitypackage.Package{}, false, errors.New("capability package identity mismatch")
	}
	return pkg, true, nil
}

func ensureCapabilityMCPRuntime(ctx context.Context, capabilityID string, target any) ([]string, error) {
	pkg, found, err := resolveCapabilityPackage(capabilityID)
	if err != nil {
		return nil, err
	}
	if !found || len(pkg.Manifest.MCPServers) == 0 {
		return nil, nil
	}
	registrar, ok := target.(managedMCPRegistrar)
	if !ok {
		return nil, errors.New("capability MCP runtime is unavailable")
	}
	names := make([]string, 0, len(pkg.Manifest.MCPServers))
	for _, server := range pkg.Manifest.MCPServers {
		readOnly := make(map[string]bool, len(server.ReadOnlyTools))
		for _, toolName := range server.ReadOnlyTools {
			readOnly[toolName] = true
		}
		tools, err := registerManagedMCPSpec(ctx, registrar, plugin.Spec{
			Name: server.ID, Type: server.Transport, URL: server.URL, ReadOnlyToolNames: readOnly,
		})
		if err != nil {
			return nil, fmt.Errorf("prepare built-in capability MCP %s: %w", server.ID, err)
		}
		if tools == 0 {
			return nil, fmt.Errorf("built-in capability MCP %s exposed no tools", server.ID)
		}
		names = append(names, server.ID)
	}
	return names, nil
}

func capabilitySubmitContract(capabilityID, input string) (string, error) {
	pkg, found, err := resolveCapabilityPackage(capabilityID)
	if err != nil {
		return "", err
	}
	if !found {
		return input, nil
	}
	contract := `<system-prompt hidden="true" capability="` + pkg.Manifest.ID + `" package-version="` + pkg.Manifest.Version + `" instruction-hash="` + pkg.InstructionHash + `">` + "\n" + pkg.Instructions
	if len(pkg.Skills) > 0 {
		var index strings.Builder
		index.WriteString("\n<capability-skills>\nUse capability_load_skill with the exact name to read a matching playbook. Loading is not execution or a permission grant.\n")
		for _, sk := range pkg.Skills {
			fmt.Fprintf(&index, "- %s: %s\n", html.EscapeString(sk.Name), html.EscapeString(sk.Description))
		}
		index.WriteString("</capability-skills>")
		contract += index.String()
	}
	return contract + "\n</system-prompt>\n\n" + input, nil
}

// EnsureCapabilityRuntime prepares built-in, credential-free integrations for
// the active capability session without persisting them into user MCP config.
func (a *App) EnsureCapabilityRuntime(capabilityID string) ([]string, error) {
	ctrl := a.activeCtrl()
	if ctrl == nil {
		return nil, errors.New("no active session")
	}
	return ensureCapabilityMCPRuntime(context.Background(), capabilityID, ctrl)
}

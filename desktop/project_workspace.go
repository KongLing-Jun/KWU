package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kwu/internal/config"
	"kwu/internal/project"
)

const cloudProjectWorkspacesDir = "project-workspaces"

// ensureCloudProjectWorkspace gives every Cloud Project one stable, account-
// scoped directory in the application's data root. Project workspaces are
// siblings of global-workspace, never descendants of it.
func ensureCloudProjectWorkspace(ctx context.Context, services *cloudProjectServices, projectID string) (string, error) {
	if services == nil || services.Bindings == nil {
		return "", fmt.Errorf("projects-v1: project workspace services are unavailable")
	}
	workspace, err := cloudProjectWorkspacePath(services, projectID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return "", fmt.Errorf("projects-v1: create dedicated project workspace: %w", err)
	}
	binding, err := services.Bindings.Bind(ctx, services.Scope, projectID, workspace)
	if err != nil {
		return "", fmt.Errorf("projects-v1: bind dedicated project workspace: %w", err)
	}
	if binding.Status != project.LocalWorkspaceAvailable || !sameDesktopPath(binding.CanonicalPath, workspace) {
		return "", fmt.Errorf("projects-v1: dedicated project workspace is unavailable")
	}
	return binding.CanonicalPath, nil
}

// cloudProjectWorkspacePath resolves the app-managed directory without
// creating or binding it. The project tree uses this pure projection to keep
// a Cloud Project's implementation workspace out of the ordinary workspace
// list when the logical Cloud Project node is already visible.
func cloudProjectWorkspacePath(services *cloudProjectServices, projectID string) (string, error) {
	if services == nil {
		return "", fmt.Errorf("projects-v1: project workspace services are unavailable")
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return "", fmt.Errorf("projects-v1: project id is required for a workspace")
	}
	root := strings.TrimSpace(services.StateRoot)
	if root == "" {
		return "", fmt.Errorf("projects-v1: project workspace root is unavailable")
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("projects-v1: normalize project workspace root: %w", err)
	}
	projectHash := sha256.Sum256([]byte(projectID))
	workspace := filepath.Join(
		filepath.Clean(absoluteRoot),
		cloudProjectWorkspacesDir,
		services.Scope.Key,
		"project-"+hex.EncodeToString(projectHash[:]),
	)
	globalRoot := filepath.Join(filepath.Clean(absoluteRoot), "global-workspace")
	if projectWorkspacePathWithin(workspace, globalRoot) {
		return "", fmt.Errorf("projects-v1: project workspace cannot be inside the global workspace")
	}
	return workspace, nil
}

func projectWorkspacePathWithin(path, root string) bool {
	path, root = filepath.Clean(path), filepath.Clean(root)
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func (a *App) cloudProjectWorkspaceStateRoots() []string {
	if a == nil {
		return nil
	}
	roots := make([]string, 0, 2)
	appendRoot := func(root string) {
		root = cleanDesktopPath(root)
		if root == "" {
			return
		}
		for _, existing := range roots {
			if sameDesktopPath(existing, root) {
				return
			}
		}
		roots = append(roots, root)
	}
	appendRoot(config.MemoryUserDir())
	appendRoot(a.currentProfilePathResolver("").MemoryUserDir())
	if services, err := a.projectServicesForOperation(); err == nil {
		appendRoot(services.StateRoot)
	}
	return roots
}

func managedCloudProjectWorkspaceInStateRoots(workspaceRoot string, stateRoots []string) bool {
	for _, stateRoot := range stateRoots {
		if isManagedCloudProjectWorkspacePath(stateRoot, workspaceRoot) {
			return true
		}
	}
	return false
}

func isManagedCloudProjectWorkspacePath(stateRoot, workspaceRoot string) bool {
	stateRoot = cleanDesktopPath(stateRoot)
	workspaceRoot = cleanDesktopPath(workspaceRoot)
	if stateRoot == "" || workspaceRoot == "" {
		return false
	}
	scopeDir := filepath.Dir(workspaceRoot)
	workspacesDir := filepath.Dir(scopeDir)
	direct := sameDesktopPath(workspacesDir, filepath.Join(stateRoot, cloudProjectWorkspacesDir))
	profileRoot := filepath.Dir(workspacesDir)
	stateRootIsProfile := sameDesktopPath(filepath.Dir(stateRoot), filepath.Join(filepath.Dir(filepath.Dir(stateRoot)), "profiles"))
	profileScoped := !stateRootIsProfile && sameDesktopPath(filepath.Dir(profileRoot), filepath.Join(stateRoot, "profiles"))
	if !direct && !profileScoped {
		return false
	}
	workspaceName := filepath.Base(workspaceRoot)
	const projectPrefix = "project-"
	if len(workspaceName) < len(projectPrefix) || !strings.EqualFold(workspaceName[:len(projectPrefix)], projectPrefix) {
		return false
	}
	return isSHA256Hex(filepath.Base(scopeDir)) &&
		isSHA256Hex(workspaceName[len(projectPrefix):])
}

func isSHA256Hex(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

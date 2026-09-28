package builtin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type verifiedArtifact struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// verifyWorkspaceArtifacts resolves and validates a complete artifact set. It
// returns no partial result when any path is invalid.
func verifyWorkspaceArtifacts(workDir string, roots, paths, expectedExtensions []string) ([]verifiedArtifact, error) {
	want := normalizedArtifactExtensions(expectedExtensions)
	verified := make([]verifiedArtifact, 0, len(paths))
	for _, raw := range paths {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return nil, fmt.Errorf("artifact path is required")
		}
		path := resolveIn(workDir, raw)
		if err := confineRead(roots, path); err != nil {
			return nil, err
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("stat artifact %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("artifact %s is not a regular file", path)
		}
		if !artifactExtensionMatches(path, want) {
			return nil, fmt.Errorf("artifact %s does not match expected extensions %v", path, want)
		}
		canonical, err := realPath(path)
		if err != nil {
			return nil, fmt.Errorf("resolve artifact %s: %w", path, err)
		}
		verified = append(verified, verifiedArtifact{Path: canonical, Size: info.Size()})
	}
	return verified, nil
}

func normalizedArtifactExtensions(extensions []string) []string {
	seen := make(map[string]bool, len(extensions))
	out := make([]string, 0, len(extensions))
	for _, extension := range extensions {
		extension = strings.ToLower(strings.TrimSpace(extension))
		if extension == "" {
			continue
		}
		if !strings.HasPrefix(extension, ".") {
			extension = "." + extension
		}
		if !seen[extension] {
			seen[extension] = true
			out = append(out, extension)
		}
	}
	return out
}

func artifactExtensionMatches(path string, expected []string) bool {
	if len(expected) == 0 {
		return true
	}
	extension := strings.ToLower(filepath.Ext(path))
	for _, want := range expected {
		if extension == want {
			return true
		}
	}
	return false
}

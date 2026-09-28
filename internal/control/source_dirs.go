package control

import (
	"os"
	"path/filepath"
	"strings"
)

func listSourceDirs(root string, maxDepth int) []string {
	skip := map[string]bool{
		".git": true, ".github": true, "node_modules": true,
		"vendor": true, ".clouvia": true, ".kwu": true, ".suiyuan": true, ".reasonix": true, "desktop": true,
		"dist": true, "build": true, ".cache": true, "bin": true,
	}
	var dirs []string
	walkDir(root, "", skip, maxDepth, &dirs)
	return dirs
}

func walkDir(root, rel string, skip map[string]bool, depth int, out *[]string) {
	if depth <= 0 {
		return
	}
	dir := root
	if rel != "" {
		dir = filepath.Join(root, rel)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || skip[name] || strings.HasPrefix(name, ".") {
			continue
		}
		childRel := name
		if rel != "" {
			childRel = rel + "/" + name
		}
		if hasSourceFiles(filepath.Join(root, childRel)) {
			*out = append(*out, childRel)
		}
		walkDir(root, childRel, skip, depth-1, out)
	}
}

func hasSourceFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			return true
		}
	}
	return false
}

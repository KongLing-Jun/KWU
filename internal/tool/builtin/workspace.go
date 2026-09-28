package builtin

import (
	"path/filepath"
	"time"

	"kwu/internal/netclient"
	"kwu/internal/sandbox"
	"kwu/internal/tool"
)

// Workspace builds a built-in tool set bound to a working directory, so several
// agents can run concurrently with independent path roots — a desktop front-end
// opening one tab per project, say. The process working directory is global and
// cannot be made per-agent (os.Chdir is process-wide), so each tool instead
// resolves relative paths against this directory and bash runs in it.
//
// Dir is that directory (empty yields process-cwd tools, byte-identical to the
// compile-time built-ins). WriteRoots confines the file-writers (as
// ConfineWriters); when empty and Dir is set, Dir itself becomes the sole write
// root, so writes stay inside the project by default. Artifact verification and
// delivery are always confined to Dir itself: extra WriteRoots may support an
// explicit allow_write workflow, but they are not deliverables in the owning
// workspace.
// Bash is the OS-sandbox spec for the bash tool (as ConfineBash).
type Workspace struct {
	Dir         string
	WriteRoots  []string
	Bash        sandbox.Spec
	BashTimeout time.Duration
	Search      SearchSpec
	ProxySpec   netclient.ProxySpec
}

// Tools returns the built-in tools bound to the workspace, ready to Add to a
// per-run tool.Registry. An empty enabled list yields every built-in; otherwise
// only the named ones are returned (unknown names are ignored). This is the
// per-workspace analogue of the cli's process-cwd assembly — a desktop driver
// calls it once per agent instead of relying on the global working directory.
func (w Workspace) Tools(enabled ...string) []tool.Tool {
	roots, artifactRoots := w.toolRoots()

	overrides := map[string]tool.Tool{
		"read_file":       readFile{workDir: w.Dir, roots: roots},
		"verify_artifact": verifyArtifact{workDir: w.Dir, roots: artifactRoots},
		"deliver_files":   deliverFiles{workDir: w.Dir, roots: artifactRoots},
		"write_file":      writeFile{workDir: w.Dir, roots: roots},
		"edit_file":       editFile{workDir: w.Dir, roots: roots},
		"multi_edit":      multiEdit{workDir: w.Dir, roots: roots},
		"move_file":       moveFile{workDir: w.Dir, roots: roots},
		"notebook_edit":   notebookEdit{workDir: w.Dir, roots: roots},
		"delete_range":    deleteRange{workDir: w.Dir, roots: roots},
		"delete_symbol":   deleteSymbol{workDir: w.Dir, roots: roots},
		"code_index":      codeIndex{workDir: w.Dir, roots: roots},
		"complete_step":   completeStep{workDir: w.Dir},
		"bash":            bash{workDir: w.Dir, sb: w.Bash, timeout: w.BashTimeout},
		"ls":              listDir{workDir: w.Dir, roots: roots},
		"glob":            globTool{workDir: w.Dir, roots: roots},
		"grep":            grepTool{workDir: w.Dir, roots: roots, rg: w.Search.RgPath},
		"web_fetch":       webFetch{proxySpec: w.ProxySpec},
	}
	return workspaceTools(enabled, overrides)
}

func (w Workspace) toolRoots() (write, artifacts []string) {
	writeRoots := w.WriteRoots
	if len(writeRoots) == 0 && w.Dir != "" {
		writeRoots = []string{w.Dir}
	}
	write = realRoots(writeRoots)
	if w.Dir != "" {
		artifacts = realRoots([]string{w.Dir})
	}
	return write, artifacts
}

func workspaceTools(enabled []string, overrides map[string]tool.Tool) []tool.Tool {
	all := tool.Builtins()
	if len(enabled) == 0 {
		for i, candidate := range all {
			if bound, ok := overrides[candidate.Name()]; ok {
				all[i] = bound
			}
		}
		return all
	}
	want := make(map[string]bool, len(enabled))
	for _, n := range enabled {
		want[n] = true
	}
	out := make([]tool.Tool, 0, len(enabled))
	for _, t := range all {
		if want[t.Name()] {
			if bound, ok := overrides[t.Name()]; ok {
				t = bound
			}
			out = append(out, t)
		}
	}
	return out
}

// resolveIn maps a tool's path/pattern argument into a working directory. With
// an empty workDir it returns p unchanged — the process-cwd behavior the
// compile-time built-ins have always had, so existing callers are unaffected.
// Otherwise a relative p is joined onto workDir; an absolute p is returned as-is
// (an explicit absolute path is honored verbatim — the write-confiner, not this,
// enforces the workspace boundary). An empty p resolves to workDir itself, so a
// defaulted "." (ls/grep) targets the workspace root.
func resolveIn(workDir, p string) string {
	if workDir == "" {
		return p
	}
	if p == "" || p == "." {
		return workDir
	}
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(workDir, p)
}

// vendorDirs are directory names grep and glob skip during a recursive walk:
// dependency, VCS, and build-cache trees that almost never hold the searched
// source and would otherwise dominate the walk (node_modules alone can be 100k+
// files) and fill the result cap with noise. Only skipped when nested — a walk
// rooted directly at one (an explicit `grep node_modules`) still searches it.
var vendorDirs = map[string]bool{
	".git": true, ".svn": true, ".hg": true, ".jj": true,
	"node_modules": true, "vendor": true, ".venv": true,
	"__pycache__": true, ".mypy_cache": true, ".pytest_cache": true,
}

// skipWalkDir reports whether a directory should be pruned from a recursive walk
// rooted at root. The root itself is never pruned, so explicitly targeting a
// vendor dir still works.
func skipWalkDir(root, path, name string) bool {
	if path == root {
		return false
	}
	return vendorDirs[name] || isProtectedDir(absClean(path))
}

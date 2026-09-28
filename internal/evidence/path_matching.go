package evidence

import (
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"

	"kwu/internal/provider"
)

// HasSuccessfulBashMentioningPathsInWorkspace is the workspace-aware form of
// HasSuccessfulBashMentioningPaths. Paths outside workDir gain no short alias.
func (l *Ledger) HasSuccessfulBashMentioningPathsInWorkspace(paths []string, workDir string) bool {
	if strings.TrimSpace(workDir) == "" {
		return l.HasSuccessfulBashMentioningPaths(paths)
	}
	return l.hasSuccessfulBashMentioningPathsInWorkspace(paths, workDir)
}

func (l *Ledger) HasSuccessfulWriteInWorkspace(paths []string, workDir string) bool {
	if strings.TrimSpace(workDir) == "" {
		return l.HasSuccessfulWrite(paths)
	}
	return l.hasSuccessfulPathsInWorkspace(paths, workDir, func(r Receipt) bool { return r.Write })
}

func (l *Ledger) HasSuccessfulReadOrWriteInWorkspace(paths []string, workDir string) bool {
	if strings.TrimSpace(workDir) == "" {
		return l.HasSuccessfulReadOrWrite(paths)
	}
	return l.hasSuccessfulPathsInWorkspace(paths, workDir, func(r Receipt) bool { return r.Read || r.Write })
}

func (l *Ledger) LatestSuccessfulWriteIndexInWorkspace(paths []string, workDir string) (int, bool) {
	if strings.TrimSpace(workDir) == "" {
		return l.LatestSuccessfulWriteIndex(paths)
	}
	return l.latestSuccessfulWriteIndexInWorkspace(paths, workDir)
}

// PathsProvenInSessionWithWorkspace applies the same lexical workspace path
// equivalence as the per-turn ledger while preserving reader/writer trust.
func PathsProvenInSessionWithWorkspace(msgs []provider.Message, paths []string, wantWrite bool, workDir string) bool {
	if strings.TrimSpace(workDir) == "" {
		return PathsProvenInSession(msgs, paths, wantWrite)
	}
	return pathsProvenInSessionWithWorkspace(msgs, paths, wantWrite, workDir)
}

func (l *Ledger) hasSuccessfulBashMentioningPathsInWorkspace(paths []string, workDir string) bool {
	if l == nil || len(paths) == 0 {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	matchedAny := false
	for _, original := range paths {
		canonical := normalizePathInWorkspace(original, workDir)
		if canonical == "" {
			continue
		}
		matchedAny = true
		if !successfulBashMentionsAliases(l.receipts, pathAliases(original, canonical, workDir)) {
			return false
		}
	}
	return matchedAny
}

func successfulBashMentionsAliases(receipts []Receipt, aliases []string) bool {
	for _, receipt := range receipts {
		if !receipt.Success || receipt.ToolName != "bash" {
			continue
		}
		command := comparablePathText(receipt.Command)
		for _, alias := range aliases {
			if commandMentionsPath(command, comparablePathText(alias)) {
				return true
			}
		}
	}
	return false
}

func (l *Ledger) hasSuccessfulPathsInWorkspace(paths []string, workDir string, accept func(Receipt) bool) bool {
	wanted := pathSet(normalizePathsInWorkspace(paths, workDir))
	if l == nil || len(wanted) == 0 {
		return false
	}
	found := map[string]bool{}
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, receipt := range l.receipts {
		if !receipt.Success || !accept(receipt) {
			continue
		}
		markMatchingPaths(found, wanted, normalizePathsInWorkspace(receipt.Paths, workDir))
	}
	return len(found) == len(wanted)
}

func (l *Ledger) latestSuccessfulWriteIndexInWorkspace(paths []string, workDir string) (int, bool) {
	wanted := pathSet(normalizePathsInWorkspace(paths, workDir))
	if l == nil || len(wanted) == 0 {
		return 0, false
	}
	latest := -1
	l.mu.Lock()
	defer l.mu.Unlock()
	for i, receipt := range l.receipts {
		if receipt.Success && receipt.Write && anyMatchingPath(wanted, normalizePathsInWorkspace(receipt.Paths, workDir)) {
			latest = i
		}
	}
	return latest, latest >= 0
}

func pathsProvenInSessionWithWorkspace(msgs []provider.Message, paths []string, wantWrite bool, workDir string) bool {
	wanted := pathSet(normalizePathsInWorkspace(paths, workDir))
	if len(wanted) == 0 {
		return false
	}
	failed := failedSessionCallIDs(msgs)
	found := map[string]bool{}
	for _, msg := range msgs {
		for _, call := range msg.ToolCalls {
			if failed[call.ID] {
				continue
			}
			receipt := ReceiptFromToolCall(call.Name, json.RawMessage(call.Arguments), true, false)
			if receiptMatchesPathTrust(receipt, wantWrite) {
				markMatchingPaths(found, wanted, normalizePathsInWorkspace(receipt.Paths, workDir))
			}
		}
	}
	return len(found) == len(wanted)
}

func receiptMatchesPathTrust(receipt Receipt, wantWrite bool) bool {
	if wantWrite {
		return receipt.Write
	}
	return receipt.Read || receipt.Write
}

func markMatchingPaths(found, wanted map[string]bool, paths []string) {
	for _, path := range paths {
		if wanted[path] {
			found[path] = true
		}
	}
}

func anyMatchingPath(wanted map[string]bool, paths []string) bool {
	for _, path := range paths {
		if wanted[path] {
			return true
		}
	}
	return false
}

func normalizePathsInWorkspace(paths []string, workDir string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if path = normalizePathInWorkspace(path, workDir); path != "" {
			out = append(out, path)
		}
	}
	return out
}

func normalizePathInWorkspace(path, workDir string) string {
	path = strings.TrimSpace(path)
	if path == "" || strings.TrimSpace(workDir) == "" {
		return normalizePath(path)
	}
	path = filepath.FromSlash(strings.ReplaceAll(path, `\`, `/`))
	if !filepath.IsAbs(path) {
		path = filepath.Join(workDir, path)
	}
	return normalizePath(path)
}

func pathAliases(original, canonical, workDir string) []string {
	aliases := []string{canonical}
	if strings.TrimSpace(workDir) == "" {
		return aliases
	}
	root := normalizePathInWorkspace(workDir, "")
	rel, err := filepath.Rel(root, canonical)
	if err == nil && pathWithinWorkspace(rel) {
		rel = normalizePath(rel)
		aliases = append(aliases, rel, "./"+filepath.ToSlash(rel))
	}
	if !filepath.IsAbs(filepath.FromSlash(strings.ReplaceAll(strings.TrimSpace(original), `\`, `/`))) {
		aliases = append(aliases, normalizePath(original))
	}
	return aliases
}

func pathWithinWorkspace(rel string) bool {
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func comparablePathText(text string) string {
	text = strings.ReplaceAll(text, `\`, `/`)
	if runtime.GOOS == "windows" {
		text = strings.ToLower(text)
	}
	return text
}

func commandMentionsPath(command, path string) bool {
	if path == "" {
		return false
	}
	for offset := 0; offset <= len(command)-len(path); {
		i := strings.Index(command[offset:], path)
		if i < 0 {
			return false
		}
		i += offset
		end := i + len(path)
		if (i == 0 || isShellPathBoundary(command[i-1])) && (end == len(command) || isShellPathBoundary(command[end])) {
			return true
		}
		offset = i + 1
	}
	return false
}

func isShellPathBoundary(b byte) bool {
	switch b {
	case ' ', '\t', '\r', '\n', '\'', '"', '`', '=', '<', '>', '|', '&', ';', ',', '(', ')', '[', ']', '{', '}':
		return true
	default:
		return false
	}
}

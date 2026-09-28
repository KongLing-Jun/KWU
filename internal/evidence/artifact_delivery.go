package evidence

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"kwu/internal/provider"
)

// DeliverFilesResultType identifies the stable result produced by the
// deliver_files workflow boundary.
const DeliverFilesResultType = "deliver_files_result"

type deliverFilesResult struct {
	Type  string   `json:"type"`
	Paths []string `json:"paths"`
}

// DecodeDeliverFilesResult accepts only the complete structured success
// result. Callers therefore share one parser and never infer delivery from
// model-authored text or a tool's input arguments.
func DecodeDeliverFilesResult(output string) ([]string, bool) {
	var result deliverFilesResult
	if err := json.Unmarshal([]byte(output), &result); err != nil ||
		result.Type != DeliverFilesResultType || result.Paths == nil ||
		len(result.Paths) > provider.MaxArtifactDeliveryPaths {
		return nil, false
	}
	paths := make([]string, len(result.Paths))
	for i, path := range result.Paths {
		if path == "" || !filepath.IsAbs(path) {
			return nil, false
		}
		paths[i] = path
	}
	return paths, true
}

func normalizeReceiptArtifactPaths(receipt *Receipt) {
	artifactPaths := cleanArtifactPaths(receipt.ArtifactPaths)
	decisionPaths := cleanArtifactPaths(receipt.Paths)
	receipt.Paths = normalizePaths(receipt.Paths)
	if receipt.ArtifactDecision {
		// Delivery paths are later shown to the user. Preserve filesystem casing;
		// comparison code derives a platform-normalized key separately.
		receipt.Paths = decisionPaths
	}
	receipt.ArtifactPaths = artifactPaths
}

func cleanArtifactPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		out = append(out, filepath.Clean(filepath.FromSlash(strings.ReplaceAll(path, `\`, `/`))))
	}
	return out
}

func (l *Ledger) LatestSuccessfulWriterIndex() (int, bool) {
	if l == nil {
		return 0, false
	}
	latest := -1
	l.mu.Lock()
	defer l.mu.Unlock()
	for i, receipt := range l.receipts {
		if receipt.Success && receipt.Write {
			latest = i
		}
	}
	return latest, latest >= 0
}

// PendingArtifactDelivery returns host-observed candidates not covered by the
// latest explicit artifact decision. Once a decision exists, any later
// mutating call invalidates it even when that call deletes a file or changes an
// existing source-like path that workspace observation intentionally filters.
func (l *Ledger) PendingArtifactDelivery() ([]string, bool) {
	if l == nil {
		return nil, false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	state := artifactDeliveryPendingState{seen: make(map[string]bool)}
	for _, receipt := range l.receipts {
		state.apply(receipt)
	}
	return append([]string(nil), state.paths...), state.pending
}

// LatestArtifactDeliveryDecision reports the latest successful explicit
// manifest and whether it remains valid. hasDecision distinguishes no manifest
// from an explicit empty manifest; valid becomes false after any later
// workspace-mutating receipt, including one whose tool call failed after making
// a partial change.
func (l *Ledger) LatestArtifactDeliveryDecision() (paths []string, hasDecision, valid bool) {
	if l == nil {
		return nil, false, false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	state := artifactDeliveryPendingState{seen: make(map[string]bool)}
	for _, receipt := range l.receipts {
		state.apply(receipt)
	}
	return append([]string(nil), state.decisionPaths...), state.hasDecision, state.decisionValid
}

type artifactDeliveryPendingState struct {
	paths         []string
	decisionPaths []string
	seen          map[string]bool
	hasDecision   bool
	decisionValid bool
	pending       bool
}

func (state *artifactDeliveryPendingState) apply(receipt Receipt) {
	if receipt.Success && receipt.ArtifactDecision {
		state.paths = state.paths[:0]
		state.decisionPaths = append(state.decisionPaths[:0], receipt.Paths...)
		state.seen = make(map[string]bool)
		state.hasDecision = true
		state.decisionValid = true
		state.pending = false
		return
	}
	if receipt.ArtifactMutation {
		state.pending = true
		if state.hasDecision {
			state.decisionValid = false
		}
	}
	if !receipt.Success {
		return
	}
	paths := append([]string(nil), receipt.ArtifactPaths...)
	if state.hasDecision && receipt.ArtifactMutation {
		paths = append(paths, receipt.Paths...)
	}
	state.addPaths(paths)
}

func (state *artifactDeliveryPendingState) addPaths(paths []string) {
	state.pending = state.pending || len(paths) > 0
	for _, path := range paths {
		key := normalizePath(path)
		if key == "" || state.seen[key] {
			continue
		}
		state.seen[key] = true
		state.paths = append(state.paths, path)
	}
}

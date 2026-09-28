package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"kwu/internal/evidence"
	"kwu/internal/provider"
)

// ArtifactDeliveryContractState reports whether a persisted final answer had
// no delivery contract, a supported contract, or a contract that must suppress
// legacy inference without being interpreted by this version of the host.
type ArtifactDeliveryContractState uint8

const (
	ArtifactDeliveryContractAbsent ArtifactDeliveryContractState = iota
	ArtifactDeliveryContractValid
	ArtifactDeliveryContractInvalid
)

// LoadedArtifactDeliveryContracts is the streaming projection of delivery
// metadata from one session. Deliveries contains only durable v1 contracts;
// State also preserves the presence of unknown or malformed contracts.
type LoadedArtifactDeliveryContracts struct {
	State      ArtifactDeliveryContractState
	Deliveries []provider.ArtifactDelivery
}

type persistedArtifactDeliveryRecord struct {
	Role             provider.Role        `json:"role"`
	MessageKind      provider.MessageKind `json:"message_kind,omitempty"`
	ArtifactDelivery json.RawMessage      `json:"artifact_delivery"`
}

// HasContract reports whether at least one final assistant answer explicitly
// carried delivery metadata, including an unsupported suppression marker.
func (loaded LoadedArtifactDeliveryContracts) HasContract() bool {
	return loaded.State != ArtifactDeliveryContractAbsent
}

// ArtifactDeliveryCandidates returns the last successful explicit deliver_files
// manifest from one top-level turn. It is the compatibility path for custom
// runners that write a transcript without participating in the Agent ledger.
// Model-authored final-answer text is excluded.
func ArtifactDeliveryCandidates(messages []provider.Message) []string {
	toolCalls := make(map[string]provider.ToolCall)
	var manifest []string
	for _, message := range messages {
		if message.Role == provider.RoleAssistant {
			for _, call := range message.ToolCalls {
				if call.ID != "" {
					toolCalls[call.ID] = call
				}
			}
			continue
		}
		call, ok := artifactDeliveryToolResultCall(message, toolCalls)
		if !ok || call.Name != "deliver_files" {
			continue
		}
		if paths, valid := artifactDeliveryManifestPaths(message.Content); valid {
			manifest = paths
		}
	}
	return append([]string(nil), manifest...)
}

func artifactDeliveryToolResultCall(message provider.Message, toolCalls map[string]provider.ToolCall) (provider.ToolCall, bool) {
	if message.Role != provider.RoleTool || message.ToolCallID == "" || toolResultFailed(message.Content) {
		return provider.ToolCall{}, false
	}
	call, ok := toolCalls[message.ToolCallID]
	return call, ok && (message.Name == "" || message.Name == call.Name)
}

func artifactDeliveryToolArtifactPaths(artifacts []provider.ToolArtifact) []string {
	paths := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		path := strings.TrimSpace(artifact.Path)
		if path == "" || !artifactDeliverySourceAllowed(artifact.Source) {
			continue
		}
		paths = append(paths, path)
	}
	return paths
}

func artifactDeliverySourceAllowed(source string) bool {
	return source == SubagentArtifactSourceWriter ||
		source == SubagentArtifactSourceSnapshot ||
		source == SubagentArtifactSourceManifest
}

func artifactDeliveryManifestPaths(result string) ([]string, bool) {
	return evidence.DecodeDeliverFilesResult(result)
}

func artifactDeliveryManifestArtifacts(result string) []provider.ToolArtifact {
	paths, ok := artifactDeliveryManifestPaths(result)
	if !ok {
		return nil
	}
	artifacts := make([]provider.ToolArtifact, 0, len(paths))
	for _, path := range paths {
		if path = strings.TrimSpace(path); path != "" {
			artifacts = append(artifacts, provider.ToolArtifact{Path: path, Source: SubagentArtifactSourceManifest})
		}
	}
	return artifacts
}

// PreviousArtifactDeliveryPaths returns the current session's durable v1
// deliverables. Per-call observation uses this immutable set so a later edit to
// an already-delivered source-like file remains a delivery.
func PreviousArtifactDeliveryPaths(messages []provider.Message) []string {
	var paths []string
	for _, message := range messages {
		if message.Role != provider.RoleAssistant || message.MessageKind != provider.MessageKindFinalAnswer ||
			!provider.IsArtifactDeliveryV1(message.ArtifactDelivery) {
			continue
		}
		paths = append(paths, message.ArtifactDelivery.Paths...)
	}
	return paths
}

func automaticArtifactDeliveryPath(path string) bool {
	path = filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(path))))
	if path == "" || path == "." {
		return false
	}
	parts := strings.Split(strings.ToLower(path), "/")
	for _, part := range parts[:len(parts)-1] {
		switch part {
		case ".customer-delivery":
			return true
		}
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown", ".txt", ".rtf",
		".pdf", ".doc", ".docx", ".xls", ".xlsx", ".xlsm", ".csv", ".tsv", ".ppt", ".pptx",
		".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".bmp", ".tif", ".tiff", ".ico",
		".mp4", ".mov", ".mkv", ".avi", ".webm", ".mp3", ".wav", ".m4a", ".aac", ".flac", ".ogg",
		".html", ".htm", ".zip", ".7z", ".tar", ".gz", ".tgz":
		return true
	default:
		return false
	}
}

func artifactDeliveryPathKey(path string) string {
	path = filepath.Clean(filepath.FromSlash(strings.TrimSpace(path)))
	if runtime.GOOS == "windows" {
		path = strings.ToLower(path)
	}
	return path
}

// LoadArtifactDeliveryContracts streams persisted contract metadata from a
// JSONL session without retaining message content or tool output in memory.
// Unknown and malformed non-null contracts are reported as invalid instead of
// being collapsed into absence, so consumers can fail closed.
func LoadArtifactDeliveryContracts(path string) (LoadedArtifactDeliveryContracts, error) {
	file, err := os.Open(path)
	if err != nil {
		return LoadedArtifactDeliveryContracts{}, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	var loaded LoadedArtifactDeliveryContracts
	for {
		var record persistedArtifactDeliveryRecord
		if err := decoder.Decode(&record); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return LoadedArtifactDeliveryContracts{}, err
		}
		loaded.include(record)
	}
	return loaded, nil
}

func (loaded *LoadedArtifactDeliveryContracts) include(record persistedArtifactDeliveryRecord) {
	if record.Role != provider.RoleAssistant || record.MessageKind != provider.MessageKindFinalAnswer {
		return
	}
	rawDelivery := bytes.TrimSpace(record.ArtifactDelivery)
	if len(rawDelivery) == 0 || bytes.Equal(rawDelivery, []byte("null")) {
		return
	}
	var delivery provider.ArtifactDelivery
	if err := json.Unmarshal(rawDelivery, &delivery); err != nil || !durableArtifactDeliveryAllowed(&delivery) {
		loaded.State = ArtifactDeliveryContractInvalid
		return
	}
	if loaded.State == ArtifactDeliveryContractAbsent {
		loaded.State = ArtifactDeliveryContractValid
	}
	loaded.Deliveries = append(loaded.Deliveries, cloneArtifactDelivery(delivery))
}

func durableArtifactDeliveryAllowed(delivery *provider.ArtifactDelivery) bool {
	return provider.IsDurableArtifactDeliveryV1(delivery)
}

// LoadArtifactDeliveries preserves the original valid-v1-only API for callers
// that do not make legacy fallback decisions.
func LoadArtifactDeliveries(path string) ([]provider.ArtifactDelivery, error) {
	loaded, err := LoadArtifactDeliveryContracts(path)
	if err != nil {
		return nil, err
	}
	return loaded.Deliveries, nil
}

func (a *Agent) finalReadinessCheck() finalReadinessCheck {
	if a.evidence == nil {
		return finalReadinessCheck{}
	}
	out, missing := a.initialFinalReadinessCheck()
	writer, hasWriter := a.evidence.LatestSuccessfulWriterIndex()
	if !hasWriter {
		return out.withReasons(missing)
	}
	if len(a.projectChecks) == 0 && !a.evidence.HasSuccessfulTodoWrite() && len(missing) == 0 {
		return finalReadinessCheck{}
	}
	out.applies = true
	projectMissing, count := a.missingFinalReadinessProjectChecks(writer)
	out.missingProjectChecks = count
	missing = append(missing, projectMissing...)
	return out.withReasons(missing)
}

func (a *Agent) initialFinalReadinessCheck() (finalReadinessCheck, []string) {
	out := finalReadinessCheck{}
	missing := []string(nil)
	if reason := a.pendingArtifactDeliveryReason(); reason != "" {
		out.applies = true
		missing = append(missing, reason)
	}
	if a.planMode.Load() {
		return out, missing
	}
	incomplete, hasTodos := a.evidence.IncompleteLatestTodos()
	if !hasTodos && a.evidence.HasAnySuccessfulReceipt() {
		incomplete, hasTodos = a.incompleteCanonicalTodos()
	}
	if hasTodos && len(incomplete) > 0 && a.evidence.HasSuccessfulTodoProgressReceipt() {
		out.applies = true
		out.incompleteTodos = len(incomplete)
		missing = append(missing, finalReadinessIncompleteTodos(incomplete))
	}
	return out, missing
}

func (a *Agent) pendingArtifactDeliveryReason() string {
	if a.session == nil || a.session.artifactTurnID.Load() == nil {
		return ""
	}
	_, declared, valid := a.evidence.LatestArtifactDeliveryDecision()
	if !declared || valid {
		return ""
	}
	pending, _ := a.evidence.PendingArtifactDelivery()
	return finalReadinessArtifactDecision(pending)
}

// ArtifactDeliveryTurnDecision returns the exact current lifecycle's
// delivery selection. A current explicit deliver_files declaration wins; a
// later workspace mutation invalidates it. When the model made no declaration,
// the host-observed candidates are returned directly so ordinary writes do not
// depend on a perfectly timed model tool call. participated distinguishes a
// normal Agent turn from a custom Runner that only wrote a transcript.
func (a *Agent) ArtifactDeliveryTurnDecision(artifactTurnID string) (paths []string, participated, valid bool) {
	if a == nil || a.evidence == nil || artifactTurnID == "" {
		return nil, false, false
	}
	turn := a.artifactEvidenceTurn.Load()
	if turn == nil || *turn != artifactTurnID {
		return nil, false, false
	}
	paths, hasDecision, decisionValid := a.evidence.LatestArtifactDeliveryDecision()
	if hasDecision {
		return paths, true, decisionValid
	}
	paths, _ = a.evidence.PendingArtifactDelivery()
	return paths, true, true
}

// resetEvidenceForRun keeps receipts across synthetic Agent.Run invocations in
// one host-owned artifact lifecycle, while preserving the historical per-Run
// reset for ordinary turns and starting fresh for every new lifecycle.
func (a *Agent) resetEvidenceForRun() {
	if a == nil || a.evidence == nil {
		return
	}
	var active *string
	if a.session != nil {
		active = a.session.artifactTurnID.Load()
	}
	if active == nil {
		a.evidence.Reset()
		a.artifactEvidenceTurn.Store(nil)
		return
	}
	if current := a.artifactEvidenceTurn.Load(); current == active {
		return
	}
	a.evidence.Reset()
	a.artifactEvidenceTurn.Store(active)
}

func (a *Agent) missingFinalReadinessProjectChecks(writer int) ([]string, int) {
	missing := make([]string, 0)
	for _, check := range a.projectChecks {
		command := strings.TrimSpace(check.Command)
		if command == "" || a.evidence.HasSuccessfulCommandAfter(command, writer) {
			continue
		}
		missing = append(missing, fmt.Sprintf("run %q from %s after the latest write", command, finalReadinessCheckSource(check)))
	}
	return missing, len(missing)
}

func (c finalReadinessCheck) withReasons(missing []string) finalReadinessCheck {
	c.reason = strings.Join(missing, "; ")
	return c
}

func (a *Agent) finalReadinessFailure() string {
	return a.finalReadinessCheck().reason
}

func finalReadinessArtifactDecision(paths []string) string {
	const instruction = "call deliver_files again after all file-producing and delegated work with the complete user-facing paths manifest, or paths: [] when none should be delivered"
	if len(paths) == 0 {
		return instruction + "; a workspace-mutating operation occurred after the latest artifact manifest"
	}
	const visibleLimit = 6
	visible := paths
	if len(visible) > visibleLimit {
		visible = visible[:visibleLimit]
	}
	quoted := make([]string, 0, len(visible))
	for _, path := range visible {
		quoted = append(quoted, fmt.Sprintf("%q", path))
	}
	suffix := ""
	if remaining := len(paths) - len(visible); remaining > 0 {
		suffix = fmt.Sprintf(" and %d more", remaining)
	}
	return instruction + "; paths changed after the latest artifact manifest: " + strings.Join(quoted, ", ") + suffix
}

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"kwu/internal/event"
	"kwu/internal/provider"
	"kwu/internal/tool"
)

var subagentToolAliases = map[string]string{
	"directory_tree": "ls",
	"edit":           "edit_file",
	"file_survey":    "glob",
	"list":           "ls",
	"move":           "move_file",
	"read":           "read_file",
	"search":         "grep",
	"write":          "write_file",
}

type taskParams struct {
	Prompt           string                `json:"prompt"`
	Description      string                `json:"description"`
	Tools            []string              `json:"tools"`
	MaxSteps         int                   `json:"max_steps"`
	Model            string                `json:"model"`
	Effort           string                `json:"effort"`
	ContinueFrom     string                `json:"continue_from"`
	ForkFrom         string                `json:"fork_from"`
	MemberName       string                `json:"member_name"`
	Independent      bool                  `json:"independent"`
	DependsOn        []string              `json:"depends_on"`
	InvalidDependsOn bool                  `json:"-"`
	A2A              *expertTeamA2ARequest `json:"a2a"`
}

type readOnlyTaskParams struct {
	Prompt      string                `json:"prompt"`
	Description string                `json:"description"`
	Tools       []string              `json:"tools"`
	MaxSteps    int                   `json:"max_steps"`
	Model       string                `json:"model"`
	Effort      string                `json:"effort"`
	MemberName  string                `json:"member_name"`
	A2A         *expertTeamA2ARequest `json:"a2a"`
}

func decodeTaskParams(args json.RawMessage) (taskParams, error) {
	var params taskParams
	if err := json.Unmarshal(args, &params); err != nil {
		return params, fmt.Errorf("invalid args: %w", err)
	}
	if params.A2A == nil {
		params.DependsOn, params.InvalidDependsOn = normalizeTaskRunIDs(params.DependsOn)
		return params, nil
	}
	prompt, err := validateExpertTeamA2ARequest(params.A2A, params.MemberName)
	if err != nil {
		return params, err
	}
	params.Prompt = prompt
	params.DependsOn, params.InvalidDependsOn = normalizeTaskRunIDs(params.DependsOn)
	return params, nil
}

func normalizeTaskRunIDs(ids []string) ([]string, bool) {
	out := make([]string, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	invalid := false
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			invalid = true
			continue
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out, invalid
}

func decodeReadOnlyTaskParams(args json.RawMessage) (readOnlyTaskParams, error) {
	var params readOnlyTaskParams
	if err := json.Unmarshal(args, &params); err != nil {
		return params, fmt.Errorf("invalid args: %w", err)
	}
	if params.A2A == nil {
		return params, nil
	}
	prompt, err := validateExpertTeamA2ARequest(params.A2A, params.MemberName)
	if err != nil {
		return params, err
	}
	params.Prompt = prompt
	return params, nil
}

// normalizeSubagentToolNames accepts the short file-tool names models commonly
// use in delegated calls. Silently dropping "read" or "write" from a whitelist
// can leave a child with bash as its only file tool, turning a one-call write
// into a fragile, potentially timeout-length shell command. Exact registered
// names always win over aliases.
func normalizeSubagentToolNames(parent *tool.Registry, names []string) []string {
	if parent == nil || len(names) == 0 {
		return names
	}
	out := make([]string, 0, len(names))
	seen := make(map[string]bool, len(names))
	for _, raw := range names {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if _, ok := parent.Get(name); !ok {
			if alias := subagentToolAliases[strings.ToLower(name)]; alias != "" {
				if _, available := parent.Get(alias); available {
					name = alias
				}
			}
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

func schemaWithTaskToolNames(schema json.RawMessage, names []string) json.RawMessage {
	var obj map[string]any
	if err := json.Unmarshal(schema, &obj); err != nil {
		return schema
	}
	props, _ := obj["properties"].(map[string]any)
	if props == nil {
		return schema
	}
	applyTaskToolProperty(props, names)
	out, err := json.Marshal(obj)
	if err != nil {
		return schema
	}
	return out
}

func applyTaskToolProperty(props map[string]any, names []string) {
	tools, _ := props["tools"].(map[string]any)
	if tools == nil {
		return
	}
	items, _ := tools["items"].(map[string]any)
	if items == nil {
		items = map[string]any{"type": "string"}
		tools["items"] = items
	}
	description := strings.TrimSpace(fmt.Sprint(tools["description"]))
	if len(names) == 0 {
		delete(items, "enum")
		tools["description"] = description + " " + subagentToolBoundarySummary
		return
	}
	items["enum"] = names
	tools["description"] = fmt.Sprintf("%s Available exact names: %s. %s",
		description,
		strings.Join(names, ", "),
		subagentToolBoundarySummary,
	)
}

// subagentArtifactHandoff turns per-call host artifact receipts into the
// structured child-to-parent handoff. Model-authored prose is not consulted.
type subagentArtifactHandoff struct {
	root       string
	downstream event.Sink
	artifacts  *toolArtifactCollector
}

type subagentArtifactCandidate struct {
	path   string
	source string
}

type subagentWorkspaceFile = SubagentWorkspaceArtifact

func (t *TaskTool) runSubSessionWithArtifactOutcome(ctx context.Context, prompt string, registry *tool.Registry, sink event.Sink, maxSteps int, runtime subSessionRuntime, session *Session) (string, []subagentWorkspaceFile, error) {
	parentArtifacts := toolArtifactCollectorFromContext(ctx)
	ctx = withToolArtifactCollector(ctx, newToolArtifactCollector())
	ctx = ensureWorkspaceArtifactObservation(ctx, t.workspaceRoot)
	handoff := newSubagentArtifactHandoff(t.workspaceRoot, registry, sink)
	answer, err := t.runSubSession(ctx, prompt, registry, handoff, maxSteps, runtime, session)
	files := subagentArtifactFilesForOutcome(handoff, answer, err)
	if err == nil {
		parentArtifacts.add(subagentWorkspaceFileArtifacts(files))
	}
	return answer, files, err
}

func subagentWorkspaceFileArtifacts(files []subagentWorkspaceFile) []provider.ToolArtifact {
	artifacts := make([]provider.ToolArtifact, 0, len(files))
	for _, file := range files {
		artifacts = append(artifacts, provider.ToolArtifact{Path: file.Path, Source: file.Source})
	}
	return artifacts
}

func subagentArtifactFilesForOutcome(handoff *subagentArtifactHandoff, answer string, err error) []subagentWorkspaceFile {
	if err != nil || handoff == nil {
		return nil
	}
	return handoff.files(answer)
}

func (t *TaskTool) runSubSessionWithArtifactHandoff(ctx context.Context, prompt string, registry *tool.Registry, sink event.Sink, maxSteps int, runtime subSessionRuntime, session *Session, a2a *expertTeamA2ARequest, taskID string) (string, error) {
	answer, files, err := t.runSubSessionWithArtifactOutcome(ctx, prompt, registry, sink, maxSteps, runtime, session)
	if err != nil {
		return appendExpertTeamA2AFailure(answer, a2a, taskID, err), err
	}
	return appendExpertTeamA2AResult(answer, files, a2a, taskID), nil
}

func (t *TaskTool) runReadOnlySubSessionWithA2A(ctx context.Context, prompt string, registry *tool.Registry, maxSteps int, runtime subSessionRuntime, a2a *expertTeamA2ARequest) (string, error) {
	answer, err := t.runSubSession(ctx, prompt, registry, subSink(ctx), maxSteps, runtime, NewSession(DefaultReadOnlyTaskSystemPrompt))
	if err != nil {
		return appendExpertTeamA2AFailure(answer, a2a, newExpertTeamA2AID("task"), err), err
	}
	return appendExpertTeamA2AResult(answer, nil, a2a, newExpertTeamA2AID("task")), nil
}

func newSubagentArtifactHandoff(root string, registry *tool.Registry, downstream event.Sink) *subagentArtifactHandoff {
	_ = registry
	return &subagentArtifactHandoff{
		root:       strings.TrimSpace(root),
		downstream: downstream,
		artifacts:  newToolArtifactCollector(),
	}
}

func (h *subagentArtifactHandoff) Emit(e event.Event) {
	if h == nil {
		return
	}
	if h.downstream != nil {
		h.downstream.Emit(e)
	}
	if e.Kind != event.ToolResult || e.Tool.Err != "" {
		return
	}
	// Final delivery belongs to the root agent. Subagents publish only the
	// host-observed receipts from their actual writer tools; a deliver_files
	// event is ignored even if a custom registry accidentally exposes it.
	if e.Tool.Name == "deliver_files" {
		return
	}
	h.recordArtifacts(e.Tool.Artifacts)
}

func (h *subagentArtifactHandoff) recordArtifacts(artifacts []provider.ToolArtifact) {
	if h == nil || h.artifacts == nil {
		return
	}
	h.artifacts.add(artifacts)
}

func subagentArtifactCandidateFromReceipt(artifact provider.ToolArtifact) (subagentArtifactCandidate, bool) {
	candidate := subagentArtifactCandidate{
		path:   strings.TrimSpace(artifact.Path),
		source: strings.TrimSpace(artifact.Source),
	}
	return candidate, candidate.path != "" && artifactDeliverySourceAllowed(candidate.source)
}

func (h *subagentArtifactHandoff) files(_ string) []subagentWorkspaceFile {
	if h == nil || strings.TrimSpace(h.root) == "" {
		return nil
	}
	candidates := h.candidatePaths()
	filesByPath := make(map[string]subagentWorkspaceFile, len(candidates))
	for _, candidate := range candidates {
		h.addArtifactCandidate(filesByPath, candidate)
	}
	files := make([]subagentWorkspaceFile, 0, len(filesByPath))
	for _, file := range filesByPath {
		files = append(files, file)
	}
	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].Path) < strings.ToLower(files[j].Path)
	})
	return files
}

func (h *subagentArtifactHandoff) addArtifactCandidate(filesByPath map[string]subagentWorkspaceFile, candidate subagentArtifactCandidate) {
	path, info, ok := canonicalSubagentWorkspaceFile(h.root, candidate.path)
	if !ok {
		return
	}
	relative, err := filepath.Rel(h.root, path)
	if err != nil || !workspaceArtifactDeliveryPathAllowed(relative, candidate.source) {
		return
	}
	key := artifactDeliveryPathKey(path)
	previous, seen := filesByPath[key]
	if seen && artifactDeliverySourceRank(previous.Source) >= artifactDeliverySourceRank(candidate.source) {
		return
	}
	filesByPath[key] = subagentWorkspaceFile{Path: path, Size: info.Size(), Source: candidate.source}
}

func (h *subagentArtifactHandoff) candidatePaths() []subagentArtifactCandidate {
	if h == nil || h.artifacts == nil {
		return nil
	}
	artifacts := h.artifacts.snapshot()
	candidates := make([]subagentArtifactCandidate, 0, len(artifacts))
	for _, artifact := range artifacts {
		candidate, ok := subagentArtifactCandidateFromReceipt(artifact)
		if ok {
			candidates = append(candidates, candidate)
		}
	}
	return candidates
}

func appendSubagentArtifactHandoff(answer string, files []subagentWorkspaceFile) string {
	if len(files) == 0 {
		return answer
	}
	data, err := json.Marshal(files)
	if err != nil {
		return answer
	}
	const prefix = "Subagent workspace files (runtime-detected; parent must use these exact paths to read the files): "
	if strings.TrimSpace(answer) == "" {
		return prefix + string(data)
	}
	return strings.TrimSpace(answer) + "\n\n" + prefix + string(data)
}

func canonicalSubagentWorkspaceFile(root, candidate string) (string, fs.FileInfo, bool) {
	rootAbs, pathAbs, ok := absoluteSubagentWorkspaceCandidate(root, candidate)
	if !ok {
		return "", nil, false
	}
	info, err := os.Stat(pathAbs)
	if err != nil || !info.Mode().IsRegular() {
		return "", nil, false
	}
	canonicalRoot, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return "", nil, false
	}
	canonicalPath, err := filepath.EvalSymlinks(pathAbs)
	if err != nil || !subagentPathWithinRoot(canonicalRoot, canonicalPath) {
		return "", nil, false
	}
	return canonicalPath, info, true
}

func absoluteSubagentWorkspaceCandidate(root, candidate string) (string, string, bool) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", "", false
	}
	path := candidate
	if !filepath.IsAbs(path) {
		path = filepath.Join(rootAbs, path)
	}
	pathAbs, err := filepath.Abs(path)
	return rootAbs, pathAbs, err == nil
}

func subagentPathWithinRoot(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

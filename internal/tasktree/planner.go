package tasktree

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"kwu/internal/provider"
)

var (
	// ErrInvalidPlannerOutput reports planner text that cannot become a valid
	// TaskTree. Callers should surface this instead of falling back to prose.
	ErrInvalidPlannerOutput = errors.New("invalid task_tree planner output")
	// ErrPlannerToolCall reports an unexpected tool call during tree planning.
	ErrPlannerToolCall = errors.New("task_tree planner attempted tool use")
	// ErrPlannerUnavailable reports a real planner without a provider backend.
	ErrPlannerUnavailable = errors.New("task_tree planner provider is not configured")
)

// LLMPlannerOptions configures the side-effect-free provider-backed planner.
type LLMPlannerOptions struct {
	Provider       provider.Provider
	Temperature    float64
	MaxTokens      int
	SessionContext string
	Now            func() time.Time
}

// LLMPlanner converts a raw user goal into structured TaskTree JSON through a
// model request that exposes no tools.
type LLMPlanner struct {
	model          provider.Provider
	temperature    float64
	maxTokens      int
	sessionContext string
	now            func() time.Time
}

// NewLLMPlanner returns a planner backed by opts.Provider. A nil provider is
// accepted so boot can still assemble cleanly; planning then returns a clear
// ErrPlannerUnavailable.
func NewLLMPlanner(opts LLMPlannerOptions) *LLMPlanner {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &LLMPlanner{
		model:          opts.Provider,
		temperature:    opts.Temperature,
		maxTokens:      opts.MaxTokens,
		sessionContext: opts.SessionContext,
		now:            now,
	}
}

// PlanTaskTree asks the model for TaskTree JSON, parses it, and validates every
// node boundary before returning structured state.
func (p *LLMPlanner) PlanTaskTree(ctx context.Context, input string) (*TaskTree, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	goal := strings.TrimSpace(input)
	if goal == "" {
		return nil, fmt.Errorf("%w: user goal is required", ErrInvalidPlannerOutput)
	}
	if p == nil || p.model == nil {
		return nil, ErrPlannerUnavailable
	}

	req := provider.Request{
		Messages: []provider.Message{
			{Role: provider.RoleSystem, Content: TaskTreePlannerSystemPrompt},
			{Role: provider.RoleUser, Content: RenderTaskTreePrompt(goal, p.sessionContext)},
		},
		Tools:       nil,
		Temperature: p.temperature,
		MaxTokens:   p.maxTokens,
	}
	stream, err := p.model.Stream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("task_tree planner request failed: %w", err)
	}

	var raw strings.Builder
	for chunk := range stream {
		switch chunk.Type {
		case provider.ChunkText:
			raw.WriteString(chunk.Text)
		case provider.ChunkReasoning, provider.ChunkUsage, provider.ChunkDone:
			continue
		case provider.ChunkToolCallStart, provider.ChunkToolCall:
			name := ""
			if chunk.ToolCall != nil {
				name = chunk.ToolCall.Name
			}
			return nil, fmt.Errorf("%w: model requested tool %q while planner requests expose no tools", ErrPlannerToolCall, name)
		case provider.ChunkError:
			if chunk.Err != nil {
				return nil, fmt.Errorf("task_tree planner stream failed: %w", chunk.Err)
			}
			return nil, errors.New("task_tree planner stream failed")
		default:
			continue
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	text := strings.TrimSpace(raw.String())
	if text == "" {
		return nil, fmt.Errorf("%w: empty planner response", ErrInvalidPlannerOutput)
	}

	tree, err := ParseTreeJSON(text)
	if err != nil {
		return nil, err
	}
	return NormalizeAndValidateGeneratedTree(tree, goal, p.now().UTC())
}

// ParseTreeJSON extracts and decodes the TaskTree JSON returned by a planner.
// It accepts a raw TaskTree object or an object wrapper with a "tree" field.
func ParseTreeJSON(raw string) (*TaskTree, error) {
	candidate, err := extractJSONObject(raw)
	if err != nil {
		return nil, err
	}

	var envelope struct {
		Tree *TaskTree `json:"tree"`
	}
	if err := json.Unmarshal([]byte(candidate), &envelope); err == nil && envelope.Tree != nil {
		return envelope.Tree, nil
	}

	var tree TaskTree
	if err := json.Unmarshal([]byte(candidate), &tree); err != nil {
		return nil, fmt.Errorf("%w: decode task tree JSON: %v", ErrInvalidPlannerOutput, err)
	}
	return &tree, nil
}

func extractJSONObject(raw string) (string, error) {
	text := strings.TrimSpace(raw)
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return "", fmt.Errorf("%w: response does not contain a JSON object", ErrInvalidPlannerOutput)
	}
	return text[start : end+1], nil
}

// NormalizeAndValidateGeneratedTree enforces the first TaskTree generation
// contract. It mutates the returned tree only to trim IDs/goals and fill missing
// timestamps.
func NormalizeAndValidateGeneratedTree(tree *TaskTree, goal string, now time.Time) (*TaskTree, error) {
	goal = strings.TrimSpace(goal)
	if tree == nil {
		return nil, fmt.Errorf("%w: task tree is nil", ErrInvalidPlannerOutput)
	}
	tree.ID = strings.TrimSpace(tree.ID)
	if tree.ID == "" {
		return nil, fmt.Errorf("%w: tree id is required", ErrInvalidPlannerOutput)
	}
	if strings.TrimSpace(tree.Goal) == "" {
		tree.Goal = goal
	} else {
		tree.Goal = strings.TrimSpace(tree.Goal)
	}
	if strings.TrimSpace(string(tree.Status)) == "" {
		return nil, fmt.Errorf("%w: tree status is required", ErrInvalidPlannerOutput)
	}
	if !tree.Status.Valid() {
		return nil, fmt.Errorf("%w: %w: tree %q has %q", ErrInvalidPlannerOutput, ErrInvalidStatus, tree.ID, tree.Status)
	}
	if tree.Nodes == nil {
		return nil, fmt.Errorf("%w: tree nodes are required", ErrInvalidPlannerOutput)
	}
	if len(tree.Nodes) == 0 {
		return nil, fmt.Errorf("%w: tree must contain at least one node", ErrInvalidPlannerOutput)
	}
	if len(tree.RootIDs) == 0 {
		return nil, fmt.Errorf("%w: tree root_ids are required", ErrInvalidPlannerOutput)
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if tree.CreatedAt.IsZero() {
		tree.CreatedAt = now
	}
	if tree.UpdatedAt.IsZero() {
		tree.UpdatedAt = now
	}

	if err := validateGeneratedNodes(tree); err != nil {
		return nil, err
	}
	return tree, nil
}

func validateGeneratedNodes(tree *TaskTree) error {
	seenNodes := make(map[string]struct{}, len(tree.Nodes))
	for id, node := range tree.Nodes {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("%w: node map key is required", ErrInvalidPlannerOutput)
		}
		if node == nil {
			return fmt.Errorf("%w: node %q is nil", ErrInvalidPlannerOutput, id)
		}
		node.ID = strings.TrimSpace(node.ID)
		if node.ID == "" {
			return fmt.Errorf("%w: node id is required for map key %q", ErrInvalidPlannerOutput, id)
		}
		if node.ID != id {
			return fmt.Errorf("%w: node map key %q does not match node id %q", ErrInvalidPlannerOutput, id, node.ID)
		}
		if _, exists := seenNodes[node.ID]; exists {
			return fmt.Errorf("%w: duplicate node id %q", ErrInvalidPlannerOutput, node.ID)
		}
		seenNodes[node.ID] = struct{}{}
		node.ParentID = strings.TrimSpace(node.ParentID)
		node.Title = strings.TrimSpace(node.Title)
		if node.Title == "" {
			return fmt.Errorf("%w: node %q title is required", ErrInvalidPlannerOutput, node.ID)
		}
		if strings.TrimSpace(string(node.Status)) == "" {
			return fmt.Errorf("%w: node %q status is required", ErrInvalidPlannerOutput, node.ID)
		}
		if !node.Status.Valid() {
			return fmt.Errorf("%w: %w: node %q has %q", ErrInvalidPlannerOutput, ErrInvalidStatus, node.ID, node.Status)
		}
		if node.Executor != nil && !node.Executor.Valid() {
			return fmt.Errorf("%w: invalid executor for node %q: %+v", ErrInvalidPlannerOutput, node.ID, node.Executor)
		}
		node.ExpectedOutput = strings.TrimSpace(node.ExpectedOutput)
		if node.ExpectedOutput == "" {
			return fmt.Errorf("%w: node %q expected_output is required", ErrInvalidPlannerOutput, node.ID)
		}
	}

	if err := validateIDList("tree", "root_ids", tree.RootIDs, tree.Nodes); err != nil {
		return err
	}
	for _, rootID := range tree.RootIDs {
		root := tree.Nodes[rootID]
		if root.ParentID != "" {
			return fmt.Errorf("%w: root node %q has parent_id %q", ErrInvalidPlannerOutput, rootID, root.ParentID)
		}
	}
	rootSet := make(map[string]struct{}, len(tree.RootIDs))
	for _, rootID := range tree.RootIDs {
		rootSet[rootID] = struct{}{}
	}

	for id, node := range tree.Nodes {
		if node.ParentID == "" {
			if _, ok := rootSet[id]; !ok {
				return fmt.Errorf("%w: node %q has no parent_id and is not listed in root_ids", ErrInvalidPlannerOutput, id)
			}
		}
		if node.ParentID != "" {
			parent, ok := tree.Nodes[node.ParentID]
			if !ok {
				return fmt.Errorf("%w: node %q parent %q does not exist", ErrInvalidPlannerOutput, id, node.ParentID)
			}
			if !contains(parent.Children, id) {
				return fmt.Errorf("%w: node %q parent %q does not list it as a child", ErrInvalidPlannerOutput, id, node.ParentID)
			}
		}
		if err := validateIDList(id, "children", node.Children, tree.Nodes); err != nil {
			return err
		}
		for _, childID := range node.Children {
			if childID == id {
				return fmt.Errorf("%w: node %q cannot be its own child", ErrInvalidPlannerOutput, id)
			}
			child := tree.Nodes[childID]
			if child.ParentID != id {
				return fmt.Errorf("%w: child %q parent_id = %q, want %q", ErrInvalidPlannerOutput, childID, child.ParentID, id)
			}
		}
		if err := validateIDList(id, "depends_on", node.DependsOn, tree.Nodes); err != nil {
			return err
		}
		for _, depID := range node.DependsOn {
			if depID == id {
				return fmt.Errorf("%w: node %q cannot depend on itself", ErrInvalidPlannerOutput, id)
			}
		}
	}

	if err := validateHierarchyAcyclic(tree); err != nil {
		return err
	}
	if err := validateHierarchyReachable(tree); err != nil {
		return err
	}
	if err := validateDependencyAcyclic(tree); err != nil {
		return err
	}
	return nil
}

func validateIDList(owner, field string, ids []string, nodes map[string]*TaskNode) error {
	seen := map[string]struct{}{}
	for i, id := range ids {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("%w: %s %s contains empty id at index %d", ErrInvalidPlannerOutput, owner, field, i)
		}
		if id != strings.TrimSpace(id) {
			return fmt.Errorf("%w: %s %s contains untrimmed id %q", ErrInvalidPlannerOutput, owner, field, id)
		}
		if _, ok := nodes[id]; !ok {
			return fmt.Errorf("%w: %s %s references missing node %q", ErrInvalidPlannerOutput, owner, field, id)
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("%w: %s %s contains duplicate id %q", ErrInvalidPlannerOutput, owner, field, id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

func validateHierarchyAcyclic(tree *TaskTree) error {
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var walk func(string) error
	walk = func(id string) error {
		if visiting[id] {
			return fmt.Errorf("%w: hierarchy cycle at node %q", ErrInvalidPlannerOutput, id)
		}
		if visited[id] {
			return nil
		}
		visiting[id] = true
		for _, childID := range tree.Nodes[id].Children {
			if err := walk(childID); err != nil {
				return err
			}
		}
		visiting[id] = false
		visited[id] = true
		return nil
	}
	for id := range tree.Nodes {
		if err := walk(id); err != nil {
			return err
		}
	}
	return nil
}

func validateHierarchyReachable(tree *TaskTree) error {
	reached := map[string]bool{}
	var walk func(string)
	walk = func(id string) {
		if reached[id] {
			return
		}
		reached[id] = true
		for _, childID := range tree.Nodes[id].Children {
			walk(childID)
		}
	}
	for _, rootID := range tree.RootIDs {
		walk(rootID)
	}
	for id := range tree.Nodes {
		if !reached[id] {
			return fmt.Errorf("%w: node %q is not reachable from root_ids", ErrInvalidPlannerOutput, id)
		}
	}
	return nil
}

func validateDependencyAcyclic(tree *TaskTree) error {
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var walk func(string) error
	walk = func(id string) error {
		if visiting[id] {
			return fmt.Errorf("%w: dependency cycle at node %q", ErrInvalidPlannerOutput, id)
		}
		if visited[id] {
			return nil
		}
		visiting[id] = true
		for _, depID := range tree.Nodes[id].DependsOn {
			if err := walk(depID); err != nil {
				return err
			}
		}
		visiting[id] = false
		visited[id] = true
		return nil
	}
	for id := range tree.Nodes {
		if err := walk(id); err != nil {
			return err
		}
	}
	return nil
}

func contains(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

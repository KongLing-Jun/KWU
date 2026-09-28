package agent

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync/atomic"

	"kwu/internal/jobs"
	"kwu/internal/provider"
	"kwu/internal/tool"
)

// toolArtifactCollector is a context-bound, lock-free accumulator for host
// artifact receipts produced by successful nested tool calls. Every published
// state is immutable; writers merge into a fresh copy and install it with CAS.
// This lets sibling nested calls complete concurrently without sharing mutable
// slices or requiring a mutex.
type toolArtifactCollector struct {
	state atomic.Pointer[toolArtifactCollectorState]
}

type toolArtifactCollectorState struct {
	artifacts []provider.ToolArtifact
}

type toolArtifactCollectorContextKey struct{}

func newToolArtifactCollector() *toolArtifactCollector {
	return &toolArtifactCollector{}
}

func withToolArtifactCollector(ctx context.Context, collector *toolArtifactCollector) context.Context {
	if ctx == nil || collector == nil {
		return ctx
	}
	return context.WithValue(ctx, toolArtifactCollectorContextKey{}, collector)
}

func toolArtifactCollectorFromContext(ctx context.Context) *toolArtifactCollector {
	if ctx == nil {
		return nil
	}
	collector, _ := ctx.Value(toolArtifactCollectorContextKey{}).(*toolArtifactCollector)
	return collector
}

func (c *toolArtifactCollector) add(artifacts []provider.ToolArtifact) {
	if c == nil || len(artifacts) == 0 {
		return
	}
	for {
		current := c.state.Load()
		var existing []provider.ToolArtifact
		if current != nil {
			existing = current.artifacts
		}
		merged := mergeToolArtifacts(existing, artifacts)
		if toolArtifactsEqual(existing, merged) {
			return
		}
		next := &toolArtifactCollectorState{artifacts: merged}
		if c.state.CompareAndSwap(current, next) {
			return
		}
	}
}

func (c *toolArtifactCollector) snapshot() []provider.ToolArtifact {
	if c == nil {
		return nil
	}
	state := c.state.Load()
	if state == nil || len(state.artifacts) == 0 {
		return nil
	}
	return append([]provider.ToolArtifact(nil), state.artifacts...)
}

// mergeToolArtifacts returns a deterministic, immutable union. Path identity
// deliberately reuses artifactDeliveryPathKey so nested and direct receipts
// cannot create duplicate cards for equivalent filesystem paths. A writer
// receipt wins over a snapshot receipt for the same path, and a validated
// deliver_files receipt wins over both because it carries the root agent's
// explicit delivery selection.
func mergeToolArtifacts(groups ...[]provider.ToolArtifact) []provider.ToolArtifact {
	total := 0
	for _, group := range groups {
		total += len(group)
	}
	if total == 0 {
		return nil
	}

	merged := make([]provider.ToolArtifact, 0, total)
	positions := make(map[string]int, total)
	for _, group := range groups {
		for _, artifact := range group {
			artifact.Path = strings.TrimSpace(artifact.Path)
			if artifact.Path == "" {
				continue
			}
			key := artifactDeliveryPathKey(artifact.Path)
			if position, seen := positions[key]; seen {
				if artifactDeliverySourceRank(artifact.Source) > artifactDeliverySourceRank(merged[position].Source) {
					merged[position] = artifact
				}
				continue
			}
			positions[key] = len(merged)
			merged = append(merged, artifact)
		}
	}
	if len(merged) == 0 {
		return nil
	}
	sort.SliceStable(merged, func(i, j int) bool {
		return artifactDeliveryPathKey(merged[i].Path) < artifactDeliveryPathKey(merged[j].Path)
	})
	return merged
}

func artifactDeliverySourceRank(source string) int {
	switch source {
	case SubagentArtifactSourceManifest:
		return 3
	case SubagentArtifactSourceWriter:
		return 2
	case SubagentArtifactSourceSnapshot:
		return 1
	default:
		return 0
	}
}

func toolArtifactsEqual(left, right []provider.ToolArtifact) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

// toolArtifactCollectionPolicy is implemented by orchestration wrappers whose
// filesystem effects must be attributed to nested host receipts instead of an
// opaque whole-wrapper workspace scan. Candidate observation and manifest
// invalidation are intentionally separate: concrete leaf writers are admitted
// by the host's explicit observation policy, while otherwise-unknown
// non-read-only tools conservatively invalidate an existing manifest.
type toolArtifactCollectionPolicy struct {
	observeWorkspace         bool
	includeNested            bool
	mayMutateOwningWorkspace bool
}

type toolArtifactCollectionPolicyProvider interface {
	toolArtifactCollectionPolicy(context.Context, json.RawMessage) toolArtifactCollectionPolicy
}

func artifactCollectionPolicyFor(ctx context.Context, t tool.Tool, args json.RawMessage) toolArtifactCollectionPolicy {
	if provider, ok := t.(toolArtifactCollectionPolicyProvider); ok {
		policy := provider.toolArtifactCollectionPolicy(ctx, args)
		policy.mayMutateOwningWorkspace = policy.mayMutateOwningWorkspace || policy.observeWorkspace
		return policy
	}
	// Unknown, MCP, and dynamic tools fail closed for candidate observation, but
	// a non-read-only call may still mutate the owning workspace. Treat that as
	// manifest-invalidating even when no artifact receipt can be attributed.
	// Nested receipts remain collectable when such a tool is invoked by a
	// synchronous wrapper.
	observeWorkspace := workspaceArtifactObservationAllowed(t, args)
	return toolArtifactCollectionPolicy{
		observeWorkspace:         observeWorkspace,
		includeNested:            true,
		mayMutateOwningWorkspace: !t.ReadOnly(),
	}
}

func (*ParallelTasksTool) toolArtifactCollectionPolicy(context.Context, json.RawMessage) toolArtifactCollectionPolicy {
	return toolArtifactCollectionPolicy{includeNested: true, mayMutateOwningWorkspace: true}
}

func (*TaskTool) toolArtifactCollectionPolicy(ctx context.Context, args json.RawMessage) toolArtifactCollectionPolicy {
	// Persisted and legacy-background tasks return a dispatch receipt before the
	// child runs. Their collector must remain isolated even if the job happens to
	// create a file before Execute returns. Foreground ephemeral tasks are
	// synchronous and may publish their successful nested receipts.
	_, hasManager := jobs.FromContext(ctx)
	managed := hasManager && (ParentSession(ctx) != "" || taskLegacyBackground(args))
	return toolArtifactCollectionPolicy{includeNested: !managed, mayMutateOwningWorkspace: !managed}
}

func (*StopTaskTool) toolArtifactCollectionPolicy(context.Context, json.RawMessage) toolArtifactCollectionPolicy {
	// stop_task delivery is derived from its validated task receipt, never from
	// filesystem activity racing with cancellation.
	return toolArtifactCollectionPolicy{mayMutateOwningWorkspace: true}
}

func (t expertTeamBoundedTaskTool) toolArtifactCollectionPolicy(ctx context.Context, args json.RawMessage) toolArtifactCollectionPolicy {
	return artifactCollectionPolicyFor(ctx, t.inner, args)
}

func (t expertTeamBoundedParallelTasksTool) toolArtifactCollectionPolicy(ctx context.Context, args json.RawMessage) toolArtifactCollectionPolicy {
	return artifactCollectionPolicyFor(ctx, t.inner, args)
}

func (t expertTeamTaskControlTool) toolArtifactCollectionPolicy(ctx context.Context, args json.RawMessage) toolArtifactCollectionPolicy {
	return artifactCollectionPolicyFor(ctx, t.inner, args)
}

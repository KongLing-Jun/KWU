package agent

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"kwu/internal/fileutil"
	"kwu/internal/tool"
	"kwu/internal/userctx"
)

type SubagentStatus string

const (
	SubagentRunning     SubagentStatus = "running"
	SubagentCompleted   SubagentStatus = "completed"
	SubagentFailed      SubagentStatus = "failed"
	SubagentInterrupted SubagentStatus = "interrupted"

	SubagentArtifactSourceWriter   = "writer"
	SubagentArtifactSourceSnapshot = "snapshot"
	SubagentArtifactSourceManifest = "manifest"
)

// SubagentWorkspaceArtifact is one file observed by the host during the latest
// completed child turn. It is not model-authored evidence.
type SubagentWorkspaceArtifact struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	Source string `json:"source,omitempty"`
}

// SubagentMeta is the sidecar for a persisted sub-agent transcript. It captures
// the execution identity that must stay stable for continuation/fork.
type SubagentMeta struct {
	Ref                string                      `json:"ref"`
	CreatedAt          time.Time                   `json:"createdAt"`
	UpdatedAt          time.Time                   `json:"updatedAt"`
	Status             SubagentStatus              `json:"status"`
	Kind               string                      `json:"kind"` // task | skill
	Name               string                      `json:"name"`
	WorkspaceRoot      string                      `json:"workspaceRoot"`
	ParentSession      string                      `json:"parentSession,omitempty"`
	ParentToolCallID   string                      `json:"parentToolCallId,omitempty"`
	ForkedFrom         string                      `json:"forkedFrom,omitempty"`
	SystemPromptHash   string                      `json:"systemPromptHash"`
	ToolScope          []string                    `json:"toolScope"`
	ToolSchemaHash     string                      `json:"toolSchemaHash"`
	Model              string                      `json:"model"`
	Effort             string                      `json:"effort"`
	ActorID            string                      `json:"actorId,omitempty"`
	ProfileID          string                      `json:"profileId,omitempty"`
	AuthMethod         string                      `json:"authMethod,omitempty"`
	ActorWorkspaceRoot string                      `json:"actorWorkspaceRoot,omitempty"`
	ParentActorID      string                      `json:"parentActorId,omitempty"`
	RemoteIdentity     *SubagentRemoteIdentity     `json:"remoteIdentity,omitempty"`
	TurnArtifacts      []SubagentWorkspaceArtifact `json:"turnArtifacts,omitempty"`
}

// SubagentSpec describes the current invocation identity.
type SubagentSpec struct {
	Kind             string
	Name             string
	WorkspaceRoot    string
	ParentSession    string
	ParentToolCallID string
	SystemPrompt     string
	Registry         *tool.Registry
	Model            string
	Effort           string
	Actor            userctx.ActorContext
}

type SubagentRemoteIdentity struct {
	Provider string `json:"provider,omitempty"`
	Domain   string `json:"domain,omitempty"`
	UserID   string `json:"userId,omitempty"`
	ChatID   string `json:"chatId,omitempty"`
	ThreadID string `json:"threadId,omitempty"`
}

// SubagentRun is a prepared transcript run. Call Release exactly once.
type SubagentRun struct {
	Ref        string
	Session    *Session
	Meta       SubagentMeta
	ForkedFrom string

	store   *SubagentStore
	release func()
}

// SubagentArtifact is a persisted sub-agent transcript and metadata pair owned
// by a parent session. One file may be missing after a crash; lifecycle cleanup
// should operate on the paths that exist.
type SubagentArtifact struct {
	Ref         string
	SessionPath string
	MetaPath    string
	Meta        SubagentMeta
}

func (r *SubagentRun) Release() {
	if r != nil && r.release != nil {
		r.release()
		r.release = nil
	}
}

// EphemeralSubagentRun is a non-persisted run for callers without an owning
// parent session — e.g. headless `kwu run`, which never mints a session
// path. Its empty Ref makes the store's MarkRunning/SaveCompleted/SaveFailed
// methods no-op and keeps FormatSubagentResult from emitting a transcript
// reference, so the sub-agent behaves exactly as it did before persisted
// transcripts existed. It holds no lock, so Release is a no-op.
func EphemeralSubagentRun(systemPrompt string) *SubagentRun {
	return &SubagentRun{Session: NewSession(systemPrompt)}
}

// SubagentStore persists sub-agent transcripts under config.SessionDir()/subagents.
// Its locks are process-local; cross-process mutation is intentionally out of v1.
type SubagentStore struct {
	dir       string
	destroyed func(parentSession string) bool

	mu     sync.Mutex
	locked map[string]bool
}

func NewSubagentStore(dir string) *SubagentStore {
	if strings.TrimSpace(dir) == "" {
		return nil
	}
	return &SubagentStore{dir: dir, locked: map[string]bool{}}
}

// WithDestroyedChecker makes saves for destroyed parent sessions no-op. This is
// used when a background sub-agent is cancelled because its parent session was
// cleared or moved out of active history.
func (s *SubagentStore) WithDestroyedChecker(fn func(parentSession string) bool) *SubagentStore {
	if s != nil {
		s.destroyed = fn
	}
	return s
}

// ListSubagentsByParent returns persisted sub-agent artifacts whose metadata
// declares the given parent session owner.
func ListSubagentsByParent(sessionDir, parentSession string) ([]SubagentArtifact, error) {
	parentSession = strings.TrimSpace(parentSession)
	if strings.TrimSpace(sessionDir) == "" || parentSession == "" {
		return nil, nil
	}
	dir := filepath.Join(sessionDir, "subagents")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := []SubagentArtifact{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".meta.json") {
			continue
		}
		ref := strings.TrimSuffix(entry.Name(), ".meta.json")
		if !validSubagentRef(ref) {
			continue
		}
		metaPath := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(metaPath)
		if err != nil {
			return nil, err
		}
		var meta SubagentMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			continue
		}
		if strings.TrimSpace(meta.ParentSession) != parentSession {
			continue
		}
		out = append(out, SubagentArtifact{
			Ref:         ref,
			SessionPath: filepath.Join(dir, ref+".jsonl"),
			MetaPath:    metaPath,
			Meta:        meta,
		})
	}
	return out, nil
}

// DeleteSubagentsByParent permanently removes sub-agent artifacts owned by a
// parent session. Missing counterpart files are ignored.
func DeleteSubagentsByParent(sessionDir, parentSession string) error {
	artifacts, err := ListSubagentsByParent(sessionDir, parentSession)
	if err != nil {
		return err
	}
	for _, artifact := range artifacts {
		for _, path := range []string{artifact.SessionPath, artifact.MetaPath} {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}

// CleanupStaleRunning marks persisted running sub-agents as interrupted. It is
// intended for startup, before this process accepts new background sub-agent
// work, so a previous crash cannot leave ghost "running" refs behind.
func (s *SubagentStore) CleanupStaleRunning() (int, error) {
	if s == nil {
		return 0, nil
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	now := time.Now().UTC()
	cleaned := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".meta.json") {
			continue
		}
		ref := strings.TrimSuffix(entry.Name(), ".meta.json")
		if !validSubagentRef(ref) {
			continue
		}
		meta, err := s.LoadMeta(ref)
		if err != nil {
			return cleaned, err
		}
		if meta.Status != SubagentRunning {
			continue
		}
		meta.Status = SubagentInterrupted
		meta.UpdatedAt = now
		if err := s.saveMeta(meta); err != nil {
			return cleaned, err
		}
		cleaned++
	}
	return cleaned, nil
}

func (s *SubagentStore) PrepareFresh(spec SubagentSpec) (*SubagentRun, error) {
	if s == nil {
		return nil, fmt.Errorf("subagent transcript store is required")
	}
	if err := requireParentSession(spec); err != nil {
		return nil, err
	}
	ref, err := s.newRef()
	if err != nil {
		return nil, err
	}
	release, err := s.lock(ref)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	meta := metaFromSpec(ref, SubagentRunning, now, now, spec)
	return &SubagentRun{Ref: ref, Session: NewSession(spec.SystemPrompt), Meta: meta, store: s, release: release}, nil
}

func (s *SubagentStore) PrepareContinue(ref string, spec SubagentSpec) (*SubagentRun, error) {
	if s == nil {
		return nil, fmt.Errorf("subagent continuation is not available in this session")
	}
	if err := requireParentSession(spec); err != nil {
		return nil, err
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("continue_from requires a subagent reference")
	}
	release, err := s.lock(ref)
	if err != nil {
		return nil, err
	}
	meta, err := s.LoadMeta(ref)
	if err != nil {
		release()
		return nil, err
	}
	if strings.TrimSpace(meta.ParentSession) != strings.TrimSpace(spec.ParentSession) {
		release()
		return s.prepareContinueFromAncestor(ref, spec)
	}
	if err := validateContinueOwner(meta, spec); err != nil {
		release()
		return nil, err
	}
	if err := validateMeta(meta, spec); err != nil {
		release()
		return nil, err
	}
	sess, err := LoadSession(s.sessionPath(ref))
	if err != nil {
		release()
		return nil, fmt.Errorf("load subagent transcript %q: %w", ref, err)
	}
	meta.ParentSession = spec.ParentSession
	meta.ParentToolCallID = spec.ParentToolCallID
	return &SubagentRun{Ref: ref, Session: sess, Meta: meta, store: s, release: release}, nil
}

func (s *SubagentStore) PrepareLegacyForkFrom(ref string, spec SubagentSpec) (*SubagentRun, error) {
	if s == nil {
		return nil, fmt.Errorf("subagent continuation is not available in this session")
	}
	if err := requireParentSession(spec); err != nil {
		return nil, err
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("fork_from requires a subagent reference")
	}
	release, err := s.lock(ref)
	if err != nil {
		return nil, err
	}
	meta, err := s.LoadMeta(ref)
	if err != nil {
		release()
		return nil, err
	}
	owner := strings.TrimSpace(meta.ParentSession)
	current := strings.TrimSpace(spec.ParentSession)
	if owner == "" {
		release()
		return nil, fmt.Errorf("subagent reference %q has no parent session; run a fresh subagent instead", ref)
	}
	if owner == current {
		release()
		if err := validateMeta(meta, spec); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("fork_from cannot be safely converted for subagent reference %q in the current conversation; use continue_from to continue it in place or start a fresh subagent for independent work", ref)
	}
	release()
	return s.PrepareContinue(ref, spec)
}

func (s *SubagentStore) prepareContinueFromAncestor(sourceRef string, spec SubagentSpec) (*SubagentRun, error) {
	sourceRef, err := s.nearestLineageSource(sourceRef, spec)
	if err != nil {
		return nil, err
	}
	copies, err := s.compatibleCopiesFromSource(sourceRef, spec)
	if err != nil {
		return nil, err
	}
	switch len(copies) {
	case 0:
		return s.prepareFork(sourceRef, spec)
	case 1:
		run, err := s.PrepareContinue(copies[0].Ref, spec)
		if err != nil {
			return nil, err
		}
		run.ForkedFrom = sourceRef
		return run, nil
	default:
		return nil, fmt.Errorf("subagent reference %q has multiple copied transcripts in current parent session %q", sourceRef, spec.ParentSession)
	}
}

func (s *SubagentStore) nearestLineageSource(requestedRef string, spec SubagentSpec) (string, error) {
	ancestors, err := s.sessionAncestors(spec.ParentSession)
	if err != nil {
		return "", err
	}
	for _, ancestor := range ancestors {
		artifacts, err := ListSubagentsByParent(filepath.Dir(s.dir), ancestor)
		if err != nil {
			return "", err
		}
		var candidates []SubagentArtifact
		for _, artifact := range artifacts {
			if artifact.Ref == requestedRef || s.derivesFrom(artifact.Meta, requestedRef) {
				candidates = append(candidates, artifact)
			}
		}
		if len(candidates) > 1 {
			return "", fmt.Errorf("subagent reference %q has multiple candidate transcripts in ancestor parent session %q", requestedRef, ancestor)
		}
		if len(candidates) == 1 {
			if err := validateMeta(candidates[0].Meta, spec); err != nil {
				return "", err
			}
			return candidates[0].Ref, nil
		}
	}
	return "", fmt.Errorf("subagent reference %q is not in current parent session %q lineage", requestedRef, spec.ParentSession)
}

func (s *SubagentStore) sessionAncestors(current string) ([]string, error) {
	current = strings.TrimSpace(current)
	if current == "" {
		return nil, nil
	}
	sessionDir := filepath.Dir(s.dir)
	var ancestors []string
	seen := map[string]bool{}
	for cursor := current; cursor != ""; {
		if seen[cursor] {
			return nil, fmt.Errorf("cycle at session %q", cursor)
		}
		seen[cursor] = true
		meta, ok, err := LoadBranchMeta(filepath.Join(sessionDir, cursor+".jsonl"))
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("missing branch metadata for session %q", cursor)
		}
		if strings.TrimSpace(meta.ID) != cursor {
			return nil, fmt.Errorf("branch metadata for session %q declares id %q", cursor, meta.ID)
		}
		parent := strings.TrimSpace(meta.ParentID)
		if parent == "" {
			break
		}
		ancestors = append(ancestors, parent)
		cursor = parent
	}
	return ancestors, nil
}

func (s *SubagentStore) derivesFrom(meta SubagentMeta, sourceRef string) bool {
	sourceRef = strings.TrimSpace(sourceRef)
	seen := map[string]bool{}
	for cursor := strings.TrimSpace(meta.ForkedFrom); cursor != ""; {
		if cursor == sourceRef {
			return true
		}
		if seen[cursor] {
			return false
		}
		seen[cursor] = true
		parent, err := s.LoadMeta(cursor)
		if err != nil {
			return false
		}
		cursor = strings.TrimSpace(parent.ForkedFrom)
	}
	return false
}

func (s *SubagentStore) compatibleCopiesFromSource(sourceRef string, spec SubagentSpec) ([]SubagentArtifact, error) {
	artifacts, err := ListSubagentsByParent(filepath.Dir(s.dir), spec.ParentSession)
	if err != nil {
		return nil, err
	}
	var copies []SubagentArtifact
	for _, artifact := range artifacts {
		if strings.TrimSpace(artifact.Meta.ForkedFrom) != sourceRef {
			continue
		}
		if err := validateMeta(artifact.Meta, spec); err != nil {
			return nil, err
		}
		copies = append(copies, artifact)
	}
	return copies, nil
}

func (s *SubagentStore) prepareFork(ref string, spec SubagentSpec) (*SubagentRun, error) {
	if s == nil {
		return nil, fmt.Errorf("subagent continuation is not available in this session")
	}
	if err := requireParentSession(spec); err != nil {
		return nil, err
	}
	sourceRef := strings.TrimSpace(ref)
	if sourceRef == "" {
		return nil, fmt.Errorf("subagent copy requires a source reference")
	}
	sourceRelease, err := s.lock(sourceRef)
	if err != nil {
		return nil, err
	}
	meta, err := s.LoadMeta(sourceRef)
	if err != nil {
		sourceRelease()
		return nil, err
	}
	if strings.TrimSpace(meta.ParentSession) == "" {
		sourceRelease()
		return nil, fmt.Errorf("subagent reference %q has no parent session; run a fresh subagent instead", sourceRef)
	}
	if err := validateMeta(meta, spec); err != nil {
		sourceRelease()
		return nil, err
	}
	if err := s.validateForkOwner(meta, spec); err != nil {
		sourceRelease()
		return nil, err
	}
	sess, err := LoadSession(s.sessionPath(sourceRef))
	if err != nil {
		sourceRelease()
		return nil, fmt.Errorf("load subagent transcript %q: %w", sourceRef, err)
	}
	sourceRelease()
	newRef, err := s.newRef()
	if err != nil {
		return nil, err
	}
	newRelease, err := s.lock(newRef)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	newMeta := metaFromSpec(newRef, SubagentRunning, now, now, spec)
	newMeta.ForkedFrom = sourceRef
	return &SubagentRun{Ref: newRef, Session: sess, Meta: newMeta, ForkedFrom: sourceRef, store: s, release: newRelease}, nil
}

func (s *SubagentStore) LoadMeta(ref string) (SubagentMeta, error) {
	var meta SubagentMeta
	if !validSubagentRef(ref) {
		return meta, fmt.Errorf("invalid subagent reference %q", ref)
	}
	var lastErr error
	for attempt := 0; attempt < 10; attempt++ {
		data, err := os.ReadFile(s.metaPath(ref))
		if err != nil {
			if os.IsNotExist(err) {
				return meta, fmt.Errorf("load subagent metadata %q: %w", ref, err)
			}
			lastErr = err
		} else {
			meta = SubagentMeta{}
			if err := json.Unmarshal(data, &meta); err == nil {
				return meta, nil
			} else {
				lastErr = err
			}
		}
		time.Sleep(time.Duration(attempt+1) * 10 * time.Millisecond)
	}
	return SubagentMeta{}, fmt.Errorf("load stable subagent metadata %q: %w", ref, lastErr)
}

func metaFromSpec(ref string, status SubagentStatus, created, updated time.Time, spec SubagentSpec) SubagentMeta {
	scope, schemaHash := toolIdentity(spec.Registry)
	meta := SubagentMeta{
		Ref:              ref,
		CreatedAt:        created,
		UpdatedAt:        updated,
		Status:           status,
		Kind:             strings.TrimSpace(spec.Kind),
		Name:             strings.TrimSpace(spec.Name),
		WorkspaceRoot:    strings.TrimSpace(spec.WorkspaceRoot),
		ParentSession:    strings.TrimSpace(spec.ParentSession),
		ParentToolCallID: strings.TrimSpace(spec.ParentToolCallID),
		SystemPromptHash: bytesHash([]byte(spec.SystemPrompt)),
		ToolScope:        scope,
		ToolSchemaHash:   schemaHash,
		Model:            strings.TrimSpace(spec.Model),
		Effort:           strings.TrimSpace(spec.Effort),
	}
	applyActorContext(&meta, spec.Actor)
	return meta
}

func applyActorContext(meta *SubagentMeta, actor userctx.ActorContext) {
	if meta == nil {
		return
	}
	actorID := strings.TrimSpace(actor.ActorID)
	meta.ActorID = actorID
	meta.ProfileID = strings.TrimSpace(actor.ProfileID)
	meta.AuthMethod = strings.TrimSpace(actor.AuthMethod)
	meta.ActorWorkspaceRoot = strings.TrimSpace(actor.WorkspaceRoot)
	meta.ParentActorID = actorID
	if actor.RemoteIdentity != nil {
		meta.RemoteIdentity = &SubagentRemoteIdentity{
			Provider: strings.TrimSpace(actor.RemoteIdentity.Provider),
			Domain:   strings.TrimSpace(actor.RemoteIdentity.Domain),
			UserID:   strings.TrimSpace(actor.RemoteIdentity.UserID),
			ChatID:   strings.TrimSpace(actor.RemoteIdentity.ChatID),
			ThreadID: strings.TrimSpace(actor.RemoteIdentity.ThreadID),
		}
	}
}

func validateMeta(meta SubagentMeta, spec SubagentSpec) error {
	if meta.Status == SubagentRunning {
		return fmt.Errorf("subagent reference %q is still in progress", meta.Ref)
	}
	want := metaFromSpec(meta.Ref, meta.Status, meta.CreatedAt, meta.UpdatedAt, spec)
	switch {
	case meta.Kind != want.Kind:
		return fmt.Errorf("subagent reference %q has kind %q, want %q", meta.Ref, meta.Kind, want.Kind)
	case meta.Name != want.Name:
		return fmt.Errorf("subagent reference %q has name %q, want %q", meta.Ref, meta.Name, want.Name)
	case meta.WorkspaceRoot != want.WorkspaceRoot:
		return fmt.Errorf("subagent reference %q belongs to workspace %q, current workspace is %q", meta.Ref, meta.WorkspaceRoot, want.WorkspaceRoot)
	case meta.SystemPromptHash != want.SystemPromptHash:
		return fmt.Errorf("subagent reference %q uses a different subagent persona; run a fresh subagent to use the current persona", meta.Ref)
	case !sameStrings(meta.ToolScope, want.ToolScope):
		return fmt.Errorf("subagent reference %q uses a different tool scope", meta.Ref)
	case meta.ToolSchemaHash != want.ToolSchemaHash:
		return fmt.Errorf("subagent reference %q uses different tool schemas", meta.Ref)
	case meta.Model != want.Model || meta.Effort != want.Effort:
		return fmt.Errorf("subagent reference %q uses model/effort %q/%q, current run would use %q/%q", meta.Ref, meta.Model, meta.Effort, want.Model, want.Effort)
	}
	if err := validateActorMeta(meta, want); err != nil {
		return err
	}
	return nil
}

func validateActorMeta(meta, want SubagentMeta) error {
	if !hasActorMeta(meta) {
		return nil
	}
	switch {
	case meta.ActorID != want.ActorID:
		return fmt.Errorf("subagent reference %q uses actor id %q, current run would use %q", meta.Ref, meta.ActorID, want.ActorID)
	case meta.ProfileID != want.ProfileID:
		return fmt.Errorf("subagent reference %q uses actor profile %q, current run would use %q", meta.Ref, meta.ProfileID, want.ProfileID)
	case meta.AuthMethod != want.AuthMethod:
		return fmt.Errorf("subagent reference %q uses actor auth method %q, current run would use %q", meta.Ref, meta.AuthMethod, want.AuthMethod)
	case meta.ActorWorkspaceRoot != want.ActorWorkspaceRoot:
		return fmt.Errorf("subagent reference %q uses actor workspace %q, current run would use %q", meta.Ref, meta.ActorWorkspaceRoot, want.ActorWorkspaceRoot)
	case meta.ParentActorID != want.ParentActorID:
		return fmt.Errorf("subagent reference %q uses parent actor id %q, current run would use %q", meta.Ref, meta.ParentActorID, want.ParentActorID)
	case meta.RemoteIdentity != nil && !sameRemoteIdentity(meta.RemoteIdentity, want.RemoteIdentity):
		return fmt.Errorf("subagent reference %q uses a different remote identity", meta.Ref)
	}
	return nil
}

func hasActorMeta(meta SubagentMeta) bool {
	return strings.TrimSpace(meta.ActorID) != "" ||
		strings.TrimSpace(meta.ProfileID) != "" ||
		strings.TrimSpace(meta.AuthMethod) != "" ||
		strings.TrimSpace(meta.ActorWorkspaceRoot) != "" ||
		strings.TrimSpace(meta.ParentActorID) != "" ||
		meta.RemoteIdentity != nil
}

func sameRemoteIdentity(a, b *SubagentRemoteIdentity) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Provider == b.Provider &&
		a.Domain == b.Domain &&
		a.UserID == b.UserID &&
		a.ChatID == b.ChatID &&
		a.ThreadID == b.ThreadID
}

func requireParentSession(spec SubagentSpec) error {
	if strings.TrimSpace(spec.ParentSession) == "" {
		return fmt.Errorf("subagent transcript parent session is required")
	}
	return nil
}

func validateContinueOwner(meta SubagentMeta, spec SubagentSpec) error {
	current := strings.TrimSpace(spec.ParentSession)
	owner := strings.TrimSpace(meta.ParentSession)
	if owner == current {
		return nil
	}
	if owner == "" {
		return fmt.Errorf("subagent reference %q has no parent session; run a fresh subagent instead", meta.Ref)
	}
	return fmt.Errorf("subagent reference %q belongs to parent session %q, current parent session is %q", meta.Ref, owner, current)
}

func (s *SubagentStore) validateForkOwner(meta SubagentMeta, spec SubagentSpec) error {
	current := strings.TrimSpace(spec.ParentSession)
	owner := strings.TrimSpace(meta.ParentSession)
	if owner == current {
		return nil
	}
	if owner == "" {
		return fmt.Errorf("subagent reference %q has no parent session; run a fresh subagent instead", meta.Ref)
	}
	ok, err := s.isAncestorSession(owner, current)
	if err != nil {
		return fmt.Errorf("subagent reference %q belongs to parent session %q, but current parent session %q lineage could not be verified: %w", meta.Ref, owner, current, err)
	}
	if ok {
		return nil
	}
	return fmt.Errorf("subagent reference %q belongs to parent session %q, which is not in current parent session %q lineage", meta.Ref, owner, current)
}

func (s *SubagentStore) isAncestorSession(ancestor, current string) (bool, error) {
	ancestor = strings.TrimSpace(ancestor)
	current = strings.TrimSpace(current)
	if ancestor == "" || current == "" {
		return false, nil
	}
	sessionDir := filepath.Dir(s.dir)
	seen := map[string]bool{}
	for cursor := current; cursor != ""; {
		if seen[cursor] {
			return false, fmt.Errorf("cycle at session %q", cursor)
		}
		seen[cursor] = true
		meta, ok, err := LoadBranchMeta(filepath.Join(sessionDir, cursor+".jsonl"))
		if err != nil {
			return false, err
		}
		if !ok {
			return false, fmt.Errorf("missing branch metadata for session %q", cursor)
		}
		if strings.TrimSpace(meta.ID) != cursor {
			return false, fmt.Errorf("branch metadata for session %q declares id %q", cursor, meta.ID)
		}
		if cursor == ancestor {
			return true, nil
		}
		parent := strings.TrimSpace(meta.ParentID)
		cursor = parent
	}
	return false, nil
}

func (s *SubagentStore) lock(ref string) (func(), error) {
	if !validSubagentRef(ref) {
		return nil, fmt.Errorf("invalid subagent reference %q", ref)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.locked[ref] {
		return nil, fmt.Errorf("subagent reference %q is already running; retry after it finishes", ref)
	}
	s.locked[ref] = true
	return func() {
		s.mu.Lock()
		delete(s.locked, ref)
		s.mu.Unlock()
	}, nil
}

func (s *SubagentStore) newRef() (string, error) {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "sa_" + time.Now().UTC().Format("20060102_150405_000000000") + "_" + hex.EncodeToString(b[:]), nil
}

func (s *SubagentStore) sessionPath(ref string) string { return filepath.Join(s.dir, ref+".jsonl") }
func (s *SubagentStore) metaPath(ref string) string    { return filepath.Join(s.dir, ref+".meta.json") }

func (s *SubagentStore) saveMeta(meta SubagentMeta) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp, err := os.CreateTemp(s.dir, ".subagent-meta.*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return fileutil.ReplaceFile(tmpPath, s.metaPath(meta.Ref))
}

func (s *SubagentStore) parentDestroyed(run *SubagentRun) bool {
	if s == nil || s.destroyed == nil || run == nil {
		return false
	}
	return s.destroyed(run.Meta.ParentSession)
}

func validSubagentRef(ref string) bool {
	if !strings.HasPrefix(ref, "sa_") {
		return false
	}
	for _, r := range ref {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func toolIdentity(reg *tool.Registry) ([]string, string) {
	if reg == nil {
		return nil, bytesHash(nil)
	}
	names := reg.Names()
	sort.Strings(names)
	schemas := normalizeToolSchemas(reg.Schemas())
	data, _ := json.Marshal(schemas)
	return names, bytesHash(data)
}

func bytesHash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

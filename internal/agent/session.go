// Package agent wires a Provider, a tool Registry, and a Session into the
// harness loop that drives a coding task to completion.
package agent

import (
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"

	"kwu/internal/provider"
)

// Renderer redraws completed assistant text as styled output.
type Renderer interface {
	Render(text string) string
}

// Session holds the conversation history for one task. The run loop (one turn at
// a time) is the only writer, but a frontend can read History/Save from another
// goroutine while a turn appends, so mu guards Messages. Direct Messages reads on
// the run-loop goroutine stay lock-free (serial with its own writes); cross-
// goroutine access goes through Snapshot.
type Session struct {
	mu     sync.RWMutex
	saveMu sync.Mutex
	// Messages remains exported for the agent run loop's existing lock-free
	// reads. Production callers must mutate it only through Add or Replace so
	// contentGeneration stays authoritative.
	Messages            []provider.Message
	rewriteVersion      int // bumped each time the log is rewritten (compact/fold)
	compactionVersion   uint64
	contentGeneration   uint64
	persistedGeneration uint64
	persistedPath       string
	// normalizedDirty is set when LoadSession repaired the history on the way in
	// (empty tool-call names, dangling calls, truncated args, …). The repair
	// already lives in Messages, so the next Save persists it automatically as
	// part of the usual full rewrite; the flag exists for observability and to
	// let callers opt out of work that a dirty session would make redundant.
	normalizedDirty bool
	artifactTurnID  atomic.Pointer[string]
}

// NewSession initializes a session with an optional system prompt.
func NewSession(system string) *Session {
	s := &Session{}
	if system != "" {
		s.Messages = append(s.Messages, provider.Message{Role: provider.RoleSystem, Content: system})
		s.markChangedLocked()
	}
	return s
}

// NewSessionFromMessages constructs a session whose initial content participates
// in dirty-generation tracking. Callers must not initialize Messages directly.
func NewSessionFromMessages(messages []provider.Message) *Session {
	s := &Session{Messages: cloneSessionMessages(messages)}
	if len(s.Messages) > 0 {
		s.markChangedLocked()
	}
	return s
}

func (s *Session) markChangedLocked() {
	s.contentGeneration++
}

// Add appends a message.
func (s *Session) Add(m provider.Message) {
	turnID := s.artifactTurnID.Load()
	if turnID != nil {
		// ArtifactTurnID is host-owned. A custom Runner may replay a message or
		// accidentally carry a stale stamp, but it cannot move new evidence or a
		// final candidate outside the lifecycle that is currently accepting it.
		m.ArtifactTurnID = *turnID
		// ArtifactDelivery is also host-owned and is written only by
		// CommitArtifactDelivery after the outer lifecycle succeeds.
		m.ArtifactDelivery = nil
		m.LocalOnly = false
	}
	// A host-managed artifact turn is not accepted at the inner agent boundary.
	// Keep even custom Runner implementations fail-closed until the outer task
	// lifecycle promotes the exact candidate together with its durable manifest.
	if turnID != nil && m.Role == provider.RoleAssistant && m.MessageKind == provider.MessageKindFinalAnswer {
		m.MessageKind = provider.MessageKindUncommittedFinalAnswer
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Messages = append(s.Messages, m)
	s.markChangedLocked()
}

func (s *Session) finalAnswerMessageKind() provider.MessageKind {
	if s.artifactTurnID.Load() != nil {
		return provider.MessageKindUncommittedFinalAnswer
	}
	return provider.MessageKindFinalAnswer
}

// BeginArtifactDeliveryTurn stamps subsequently appended messages with a
// stable local identity that survives in-turn compaction and index rebasing.
func (s *Session) BeginArtifactDeliveryTurn(turnID string) {
	if turnID == "" {
		return
	}
	value := turnID
	s.artifactTurnID.Store(&value)
}

// EndArtifactDeliveryTurn clears the active stamp only when it still belongs to
// the caller, so a stale cleanup cannot clear a newer lifecycle.
func (s *Session) EndArtifactDeliveryTurn(turnID string) {
	for {
		current := s.artifactTurnID.Load()
		if current == nil || *current != turnID {
			return
		}
		if s.artifactTurnID.CompareAndSwap(current, nil) {
			return
		}
	}
}

// AttachTurnMetadata marks the last assistant message appended at or after
// start. A turn can contain tool-call assistant messages, so the final one owns
// the turn-level model and billing identity shown by transcript frontends.
func (s *Session) AttachTurnMetadata(start int, metadata provider.TurnMetadata) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if start < 0 {
		start = 0
	}
	if start > len(s.Messages) {
		start = len(s.Messages)
	}
	for i := len(s.Messages) - 1; i >= start; i-- {
		if s.Messages[i].Role != provider.RoleAssistant {
			continue
		}
		value := metadata
		s.Messages[i].Turn = &value
		s.markChangedLocked()
		return true
	}
	return false
}

// CommitArtifactDelivery atomically promotes the uncommitted assistant answer
// stamped with artifactTurnID and attaches its host-owned artifact delivery
// contract. Once committed it is immutable: repeated callers receive a deep
// copy of the original contract and cannot overwrite it.
func (s *Session) CommitArtifactDelivery(artifactTurnID string, delivery provider.ArtifactDelivery) (provider.ArtifactDelivery, bool) {
	if artifactTurnID == "" || !provider.IsDurableArtifactDeliveryV1(&delivery) {
		return provider.ArtifactDelivery{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.commitArtifactDeliveryLocked(artifactTurnID, delivery)
}

func (s *Session) commitArtifactDeliveryLocked(artifactTurnID string, delivery provider.ArtifactDelivery) (provider.ArtifactDelivery, bool) {
	for i := len(s.Messages) - 1; i >= 0; i-- {
		message := &s.Messages[i]
		if message.ArtifactTurnID != artifactTurnID || message.Role != provider.RoleAssistant {
			continue
		}
		if message.MessageKind == provider.MessageKindFinalAnswer && provider.IsDurableArtifactDeliveryV1(message.ArtifactDelivery) {
			return cloneArtifactDelivery(*message.ArtifactDelivery), true
		}
		if message.MessageKind != provider.MessageKindUncommittedFinalAnswer {
			continue
		}
		committed := cloneArtifactDelivery(delivery)
		message.MessageKind = provider.MessageKindFinalAnswer
		message.ArtifactDelivery = &committed
		s.markChangedLocked()
		return cloneArtifactDelivery(committed), true
	}
	return provider.ArtifactDelivery{}, false
}

// DowngradeArtifactDeliveryFinal atomically leaves every final candidate for an
// artifact turn uncommitted and clears any delivery contract attached to it.
// It reports whether that turn contains a final candidate, including one that
// was already uncommitted, so callers can durably persist the fail-closed state.
func (s *Session) DowngradeArtifactDeliveryFinal(artifactTurnID string) bool {
	if artifactTurnID == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.downgradeArtifactDeliveryFinalLocked(artifactTurnID)
}

func (s *Session) downgradeArtifactDeliveryFinalLocked(artifactTurnID string) bool {
	found := false
	changed := false
	for i := len(s.Messages) - 1; i >= 0; i-- {
		message := &s.Messages[i]
		if message.ArtifactTurnID != artifactTurnID || message.Role != provider.RoleAssistant ||
			(message.MessageKind != provider.MessageKindFinalAnswer && message.MessageKind != provider.MessageKindUncommittedFinalAnswer) {
			continue
		}
		found = true
		if message.MessageKind != provider.MessageKindUncommittedFinalAnswer {
			message.MessageKind = provider.MessageKindUncommittedFinalAnswer
			changed = true
		}
		if message.ArtifactDelivery != nil {
			message.ArtifactDelivery = nil
			changed = true
		}
	}
	if changed {
		s.markChangedLocked()
	}
	return found
}

func cloneArtifactDelivery(delivery provider.ArtifactDelivery) provider.ArtifactDelivery {
	var paths []string
	if delivery.Paths != nil {
		paths = append([]string{}, delivery.Paths...)
	}
	return provider.ArtifactDelivery{
		Version:     delivery.Version,
		Paths:       paths,
		DeliveredAt: delivery.DeliveredAt,
	}
}

func cloneSessionMessages(messages []provider.Message) []provider.Message {
	cloned := append([]provider.Message(nil), messages...)
	for i := range cloned {
		if len(cloned[i].ToolArtifacts) > 0 {
			cloned[i].ToolArtifacts = append([]provider.ToolArtifact(nil), cloned[i].ToolArtifacts...)
		}
		if cloned[i].ArtifactDelivery != nil {
			delivery := cloneArtifactDelivery(*cloned[i].ArtifactDelivery)
			cloned[i].ArtifactDelivery = &delivery
		}
	}
	return cloned
}

// Replace swaps the whole message log — used by compaction, which rewrites the
// middle of the history.
func (s *Session) Replace(msgs []provider.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := cloneSessionMessages(msgs)
	if turnID := s.artifactTurnID.Load(); turnID != nil {
		trustedPrefix := exactMessagePrefixLen(s.Messages, next)
		for i := range next {
			if i < trustedPrefix {
				continue
			}
			next[i].ArtifactTurnID = *turnID
			next[i].ArtifactDelivery = nil
			next[i].LocalOnly = false
			if next[i].Role == provider.RoleAssistant && next[i].MessageKind == provider.MessageKindFinalAnswer {
				next[i].MessageKind = provider.MessageKindUncommittedFinalAnswer
			}
		}
	}
	s.Messages = next
	s.markChangedLocked()
}

// During an active lifecycle, only the exact unchanged prefix is trusted as
// pre-existing history. Once a Runner inserts, rewrites, removes, duplicates,
// or moves a message, the remainder belongs to the current turn. This keeps an
// old committed final from being moved across a new user-turn boundary while
// avoiding full-history serialization under the session lock.
func exactMessagePrefixLen(previous, replacement []provider.Message) int {
	limit := len(previous)
	if len(replacement) < limit {
		limit = len(replacement)
	}
	for i := 0; i < limit; i++ {
		if !reflect.DeepEqual(previous[i], replacement[i]) {
			return i
		}
	}
	return limit
}

// replaceForCompaction is the trusted rewrite path for host-owned context
// maintenance. Its callers construct the replacement exclusively from the
// current session plus local summary/receipt records, so artifact ownership is
// preserved without applying the untrusted Runner boundary in Replace.
func (s *Session) replaceForCompaction(msgs []provider.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Messages = cloneSessionMessages(msgs)
	s.compactionVersion++
	s.markChangedLocked()
}

// replaceForPrune is the trusted non-structural rewrite path. Pruning edits
// tool-result bodies in place, so existing conversation checkpoint indexes stay
// valid and the compaction generation must not advance.
func (s *Session) replaceForPrune(msgs []provider.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Messages = cloneSessionMessages(msgs)
	s.markChangedLocked()
}

func (s *Session) markPersisted(path string, generation uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.persistedPath = path
	s.persistedGeneration = generation
	s.normalizedDirty = false
}

// Snapshot returns a copy of the messages, safe to read from another goroutine
// while a turn appends. Frontends (History, Save) use it instead of touching the
// live slice.
func (s *Session) Snapshot() []provider.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneSessionMessages(s.Messages)
}

// SnapshotRange returns a stable history window ending before before. A zero
// before means the current end. The start is moved backward when necessary so
// an assistant tool-call message is never split from its following tool results.
func (s *Session) SnapshotRange(before, limit int) (messages []provider.Message, start int, hasMore bool) {
	messages, start, hasMore, _, _ = s.SnapshotRangeWithGeneration(before, limit, "")
	return messages, start, hasMore
}

// SnapshotRangeWithGeneration is SnapshotRange with an optimistic cursor
// guard. A caller that supplies a stale generation must restart from the tail.
func (s *Session) SnapshotRangeWithGeneration(before, limit int, expectedGeneration string) (messages []provider.Message, start int, hasMore bool, generation string, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	generation = fmt.Sprintf("%d", s.contentGeneration)
	if expectedGeneration != "" && expectedGeneration != generation {
		return nil, 0, false, generation, ErrSessionGenerationChanged
	}
	end := before
	if end <= 0 || end > len(s.Messages) {
		end = len(s.Messages)
	}
	if limit <= 0 || limit >= end {
		start = 0
	} else {
		start = end - limit
		for start > 0 && s.Messages[start].Role == provider.RoleTool {
			start--
		}
	}
	return cloneSessionMessages(s.Messages[start:end]), start, start > 0, generation, nil
}

// RewriteVersion returns the current rewrite version.
func (s *Session) RewriteVersion() int { return s.rewriteVersion }

// CompactionVersion changes only when a structural context rewrite commits.
// Controllers use it to invalidate message-index checkpoint boundaries without
// treating in-place tool-result pruning as a conversation restructure.
func (s *Session) CompactionVersion() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.compactionVersion
}

// IncrementRewrite bumps the rewrite version by 1.
func (s *Session) IncrementRewrite() { s.rewriteVersion++ }

// HasContent returns true when the session carries at least one user,
// assistant, or tool message — i.e. more than just a system prompt. An
// "empty" conversation that has never been used should not be persisted.
func (s *Session) HasContent() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.Messages {
		if m.Role != provider.RoleSystem {
			return true
		}
	}
	return false
}

// HasSystemMessage reports whether the session starts with the system message
// that carries the agent identity and behavioural contract.
func (s *Session) HasSystemMessage() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.Messages) > 0 && s.Messages[0].Role == provider.RoleSystem
}

// NormalizeSession runs the persisted-history-safe repairs on a loaded
// conversation and is the agent-side entry point for making old, partially
// saved, or interrupted sessions replayable. It is a thin wrapper over
// provider.NormalizeSessionMessages, which shares assistant-turn repairs with
// the provider send path without applying wire-only cleanup such as dropping
// standalone tool messages.
//
// LoadSession calls this right after decoding so a session that was written by
// an older code version, or that was cut short mid-turn, is corrected in memory
// before anything reads it. The corrected messages are persisted lazily: the
// next Session.Save (naturally triggered by the following turn) rewrites the
// whole file with the repairs baked in, so the same stale-data bug is not
// re-repaired on every turn forever. A session that is only ever read (never
// appended to) stays unmodified on disk and is simply re-normalized on the next
// load — cheap, because the fast path returns the input slice unchanged.
//
// Well-formed histories are returned without allocating (see
// provider.NormalizeSessionMessages), so this is a no-op in both time and memory
// for the common case and cannot perturb a provider's prefix-cache key.
func NormalizeSession(msgs []provider.Message) []provider.Message {
	return provider.NormalizeSessionMessages(msgs)
}

package cloud

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"kwu/internal/userctx"
)

// ErrEventCheckpointNotAdvanced reports an event that cannot safely move a
// checkpoint forward because its ID was already applied or its scope-wide
// event-stream version is not newer. Callers must not treat this as a
// successful checkpoint commit.
var ErrEventCheckpointNotAdvanced = errors.New("cloud: event checkpoint not advanced")

// EventCheckpoint records only locally safe, already-applied normalized events.
// It is scoped to one profile, cloud user, and binding generation.
type EventCheckpoint struct {
	Cursor   string
	versions map[EventType]uint64
	applied  map[string]struct{}
}

// Version retains the highest safely-applied version for one event type so
// existing durable v1 documents remain readable and auditable.
func (c EventCheckpoint) Version(eventType EventType) uint64 {
	return c.versions[eventType]
}

func (c EventCheckpoint) latestVersion() uint64 {
	var latest uint64
	for _, version := range c.versions {
		if version > latest {
			latest = version
		}
	}
	return latest
}

// HasApplied reports whether an event ID was safely applied in this checkpoint.
func (c EventCheckpoint) HasApplied(eventID string) bool {
	_, ok := c.applied[eventID]
	return ok
}

// EventStateRepository persists local L1 checkpoints. It neither connects to a
// cloud transport nor discovers remote events.
type EventStateRepository interface {
	Load(context.Context, BindingScope) (EventCheckpoint, bool, error)
	CommitApplied(context.Context, BindingScope, Event) error
	Clear(context.Context, BindingScope) error
}

// MemoryEventStateRepository retains checkpoints for the lifetime of the
// repository instance. Reusing an instance simulates local process recovery.
type MemoryEventStateRepository struct {
	mu          sync.RWMutex
	checkpoints map[string]EventCheckpoint
}

func NewMemoryEventStateRepository() *MemoryEventStateRepository {
	return &MemoryEventStateRepository{checkpoints: make(map[string]EventCheckpoint)}
}

func (r *MemoryEventStateRepository) Load(ctx context.Context, scope BindingScope) (EventCheckpoint, bool, error) {
	if err := ctx.Err(); err != nil {
		return EventCheckpoint{}, false, err
	}
	if err := scope.Validate(); err != nil {
		return EventCheckpoint{}, false, err
	}
	r.mu.RLock()
	checkpoint, found := r.checkpoints[scope.key()]
	r.mu.RUnlock()
	return cloneEventCheckpoint(checkpoint), found, nil
}

// CommitApplied atomically records a normalized event only after its caller has
// safely applied the event's authoritative refresh or recovery action.
func (r *MemoryEventStateRepository) CommitApplied(ctx context.Context, scope BindingScope, event Event) error {
	if err := validateEventCheckpointCommit(ctx, scope, event); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	current := r.checkpoints[scope.key()]
	if _, exists := current.applied[event.EventID]; exists || current.latestVersion() >= event.Version {
		return ErrEventCheckpointNotAdvanced
	}
	candidate := cloneEventCheckpoint(current)
	if candidate.versions == nil {
		candidate.versions = make(map[EventType]uint64)
	}
	if candidate.applied == nil {
		candidate.applied = make(map[string]struct{})
	}
	candidate.Cursor = event.EventID
	candidate.versions[event.EventType] = event.Version
	candidate.applied[event.EventID] = struct{}{}
	r.checkpoints[scope.key()] = candidate
	return nil
}

func (r *MemoryEventStateRepository) Clear(ctx context.Context, scope BindingScope) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	delete(r.checkpoints, scope.key())
	r.mu.Unlock()
	return nil
}

func validateEventCheckpointCommit(ctx context.Context, scope BindingScope, event Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	if err := event.Validate(); err != nil {
		return err
	}
	if event.CloudUserID != scope.CloudUserID {
		return fmt.Errorf("cloud: event cloud user does not match checkpoint scope")
	}
	return nil
}

func cloneEventCheckpoint(checkpoint EventCheckpoint) EventCheckpoint {
	clone := EventCheckpoint{Cursor: checkpoint.Cursor}
	if checkpoint.versions != nil {
		clone.versions = make(map[EventType]uint64, len(checkpoint.versions))
		for eventType, version := range checkpoint.versions {
			clone.versions[eventType] = version
		}
	}
	if checkpoint.applied != nil {
		clone.applied = make(map[string]struct{}, len(checkpoint.applied))
		for eventID := range checkpoint.applied {
			clone.applied[eventID] = struct{}{}
		}
	}
	return clone
}

var _ EventStateRepository = (*MemoryEventStateRepository)(nil)

const (
	durableEventStateSchema  = "reasonix.cloud.event_state"
	durableEventStateVersion = 1
)

type durableEventStateDocument struct {
	Schema    string                   `json:"schema"`
	Version   int                      `json:"version"`
	ProfileID string                   `json:"profile_id"`
	Entries   []durableEventStateEntry `json:"entries"`
}

type durableEventStateEntry struct {
	Scope           BindingScope          `json:"scope"`
	Key             string                `json:"key"`
	Cursor          string                `json:"cursor"`
	Versions        []durableEventVersion `json:"versions"`
	AppliedEventIDs []string              `json:"applied_event_ids"`
}

type durableEventVersion struct {
	EventType EventType `json:"event_type"`
	Version   uint64    `json:"version"`
}

// DurableEventStateRepository persists only locally applied checkpoints. It
// does not provide a remote cursor or event discovery mechanism.
type DurableEventStateRepository struct {
	paths ProfilePathProvider
}

func NewDurableEventStateRepository(paths ProfilePathProvider) *DurableEventStateRepository {
	return &DurableEventStateRepository{paths: paths}
}

func (r *DurableEventStateRepository) Load(ctx context.Context, scope BindingScope) (EventCheckpoint, bool, error) {
	if err := ctx.Err(); err != nil {
		return EventCheckpoint{}, false, err
	}
	if err := scope.Validate(); err != nil {
		return EventCheckpoint{}, false, err
	}
	path, err := r.path(scope.ProfileID)
	if err != nil {
		return EventCheckpoint{}, false, err
	}
	lock := durablePathLock(path)
	lock.Lock()
	defer lock.Unlock()
	entries, err := readDurableEventState(path, scope.ProfileID)
	if err != nil {
		return EventCheckpoint{}, false, err
	}
	entry, found := entries[scope.key()]
	if !found {
		return EventCheckpoint{}, false, nil
	}
	return checkpointFromDurableEventEntry(entry), true, nil
}

func (r *DurableEventStateRepository) CommitApplied(ctx context.Context, scope BindingScope, event Event) error {
	if err := validateEventCheckpointCommit(ctx, scope, event); err != nil {
		return err
	}
	path, err := r.path(scope.ProfileID)
	if err != nil {
		return err
	}
	lock := durablePathLock(path)
	lock.Lock()
	defer lock.Unlock()
	entries, err := readDurableEventState(path, scope.ProfileID)
	if err != nil {
		return err
	}
	current := checkpointFromDurableEventEntry(entries[scope.key()])
	if _, exists := current.applied[event.EventID]; exists || current.latestVersion() >= event.Version {
		return ErrEventCheckpointNotAdvanced
	}
	if current.versions == nil {
		current.versions = make(map[EventType]uint64)
	}
	if current.applied == nil {
		current.applied = make(map[string]struct{})
	}
	current.Cursor = event.EventID
	current.versions[event.EventType] = event.Version
	current.applied[event.EventID] = struct{}{}
	entries[scope.key()] = durableEventEntryFromCheckpoint(scope, current)
	return writeDurableEventState(path, scope.ProfileID, entries)
}

func (r *DurableEventStateRepository) Clear(ctx context.Context, scope BindingScope) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	path, err := r.path(scope.ProfileID)
	if err != nil {
		return err
	}
	lock := durablePathLock(path)
	lock.Lock()
	defer lock.Unlock()
	entries, err := readDurableEventState(path, scope.ProfileID)
	if err != nil {
		return err
	}
	if _, ok := entries[scope.key()]; !ok {
		return nil
	}
	delete(entries, scope.key())
	return writeDurableEventState(path, scope.ProfileID, entries)
}

func (r *DurableEventStateRepository) path(profileID string) (string, error) {
	return durableCloudFile(r.paths, userctx.ActorContext{ProfileID: profileID}, "event_state.json")
}

func readDurableEventState(path, profileID string) (map[string]durableEventStateEntry, error) {
	var document durableEventStateDocument
	found, err := durableReadJSON(path, &document)
	if err != nil {
		return nil, err
	}
	if !found {
		return make(map[string]durableEventStateEntry), nil
	}
	if document.Schema != durableEventStateSchema || document.Version != durableEventStateVersion {
		return nil, fmt.Errorf("cloud: unsupported durable event state schema")
	}
	if document.ProfileID != profileID {
		return nil, fmt.Errorf("cloud: durable event state belongs to another profile")
	}
	entries := make(map[string]durableEventStateEntry, len(document.Entries))
	for _, entry := range document.Entries {
		if err := validateDurableEventStateEntry(profileID, entry); err != nil {
			return nil, err
		}
		if _, exists := entries[entry.Key]; exists {
			return nil, fmt.Errorf("cloud: duplicate durable event checkpoint")
		}
		entries[entry.Key] = entry
	}
	return entries, nil
}

func validateDurableEventStateEntry(profileID string, entry durableEventStateEntry) error {
	if err := entry.Scope.Validate(); err != nil {
		return fmt.Errorf("cloud: invalid durable event scope: %w", err)
	}
	if entry.Scope.ProfileID != profileID {
		return fmt.Errorf("cloud: durable event checkpoint belongs to another profile")
	}
	if entry.Key != entry.Scope.key() {
		return fmt.Errorf("cloud: durable event checkpoint key mismatch")
	}
	if err := requireTrimmed("cursor", entry.Cursor); err != nil {
		return err
	}
	if len(entry.Versions) == 0 || len(entry.AppliedEventIDs) == 0 {
		return fmt.Errorf("cloud: durable event checkpoint is empty")
	}
	if err := validateDurableEventVersions(entry.Versions); err != nil {
		return err
	}
	return validateDurableAppliedEvents(entry.Cursor, entry.AppliedEventIDs)
}

func validateDurableEventVersions(entries []durableEventVersion) error {
	versions := make(map[EventType]struct{}, len(entries))
	for _, version := range entries {
		if !validDurableEventType(version.EventType) || version.Version == 0 {
			return fmt.Errorf("cloud: invalid durable event version")
		}
		if _, exists := versions[version.EventType]; exists {
			return fmt.Errorf("cloud: duplicate durable event type")
		}
		versions[version.EventType] = struct{}{}
	}
	return nil
}

func validateDurableAppliedEvents(cursor string, eventIDs []string) error {
	applied := make(map[string]struct{}, len(eventIDs))
	for _, eventID := range eventIDs {
		if err := requireTrimmed("event_id", eventID); err != nil {
			return err
		}
		if _, exists := applied[eventID]; exists {
			return fmt.Errorf("cloud: duplicate durable applied event ID")
		}
		applied[eventID] = struct{}{}
	}
	if _, exists := applied[cursor]; !exists {
		return fmt.Errorf("cloud: durable event cursor was not applied")
	}
	return nil
}

func validDurableEventType(eventType EventType) bool {
	switch eventType {
	case EventBalanceUpdated, EventFreezeCreated, EventFreezeExpired,
		EventSettlementCompleted, EventSettlementException, EventSubscriptionUpdated,
		EventPaymentUpdated, EventThirdPartySyncUpdated:
		return true
	default:
		return false
	}
}

func checkpointFromDurableEventEntry(entry durableEventStateEntry) EventCheckpoint {
	checkpoint := EventCheckpoint{Cursor: entry.Cursor}
	if len(entry.Versions) > 0 {
		checkpoint.versions = make(map[EventType]uint64, len(entry.Versions))
		for _, version := range entry.Versions {
			checkpoint.versions[version.EventType] = version.Version
		}
	}
	if len(entry.AppliedEventIDs) > 0 {
		checkpoint.applied = make(map[string]struct{}, len(entry.AppliedEventIDs))
		for _, eventID := range entry.AppliedEventIDs {
			checkpoint.applied[eventID] = struct{}{}
		}
	}
	return checkpoint
}

func durableEventEntryFromCheckpoint(scope BindingScope, checkpoint EventCheckpoint) durableEventStateEntry {
	entry := durableEventStateEntry{Scope: scope, Key: scope.key(), Cursor: checkpoint.Cursor}
	for eventType, version := range checkpoint.versions {
		entry.Versions = append(entry.Versions, durableEventVersion{EventType: eventType, Version: version})
	}
	sort.Slice(entry.Versions, func(i, j int) bool { return entry.Versions[i].EventType < entry.Versions[j].EventType })
	for eventID := range checkpoint.applied {
		entry.AppliedEventIDs = append(entry.AppliedEventIDs, eventID)
	}
	sort.Strings(entry.AppliedEventIDs)
	return entry
}

func writeDurableEventState(path, profileID string, entries map[string]durableEventStateEntry) error {
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ordered := make([]durableEventStateEntry, 0, len(keys))
	for _, key := range keys {
		ordered = append(ordered, entries[key])
	}
	document := durableEventStateDocument{Schema: durableEventStateSchema, Version: durableEventStateVersion, ProfileID: profileID, Entries: ordered}
	return durableWriteJSON(path, document)
}

var _ EventStateRepository = (*DurableEventStateRepository)(nil)

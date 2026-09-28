package cloud

import (
	"context"
	"fmt"
	"sort"

	"kwu/internal/userctx"
)

const (
	durableTaskRecoverySchema  = "reasonix.cloud.task_recovery"
	durableTaskRecoveryVersion = 1
)

type durableTaskRecoveryDocument struct {
	Schema    string                     `json:"schema"`
	Version   int                        `json:"version"`
	ProfileID string                     `json:"profile_id"`
	Entries   []durableTaskRecoveryEntry `json:"entries"`
}

type durableTaskRecoveryEntry struct {
	Scope              BindingScope       `json:"scope"`
	Key                string             `json:"key"`
	Record             TaskRecoveryRecord `json:"record"`
	PendingEventIDs    []string           `json:"pending_event_ids,omitempty"`
	AuthorizedEventIDs []string           `json:"authorized_event_ids,omitempty"`
}

// DurableTaskRecoveryStore persists only task records that were explicitly
// saved locally. It never discovers pending tasks from the cloud.
type DurableTaskRecoveryStore struct {
	paths ProfilePathProvider
}

func NewDurableTaskRecoveryStore(paths ProfilePathProvider) *DurableTaskRecoveryStore {
	return &DurableTaskRecoveryStore{paths: paths}
}

func (s *DurableTaskRecoveryStore) Save(ctx context.Context, record TaskRecoveryRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := record.Validate(); err != nil {
		return err
	}
	path, err := s.path(record.Scope.ProfileID)
	if err != nil {
		return err
	}
	lock := durablePathLock(path)
	lock.Lock()
	defer lock.Unlock()
	entries, err := readDurableTaskRecovery(path, record.Scope.ProfileID)
	if err != nil {
		return err
	}
	entry := durableTaskRecoveryEntry{Scope: record.Scope, Key: taskRecoveryRecordKey(record), Record: record}
	key := durableTaskRecoveryEntryKey(entry)
	if current, found := entries[key]; found && current.Record == record {
		entry.PendingEventIDs = current.PendingEventIDs
		entry.AuthorizedEventIDs = current.AuthorizedEventIDs
	}
	entries[key] = entry
	return writeDurableTaskRecovery(path, record.Scope.ProfileID, entries)
}

func (s *DurableTaskRecoveryStore) Load(ctx context.Context, scope BindingScope) ([]TaskRecoveryRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	path, err := s.path(scope.ProfileID)
	if err != nil {
		return nil, err
	}
	lock := durablePathLock(path)
	lock.Lock()
	defer lock.Unlock()
	entries, err := readDurableTaskRecovery(path, scope.ProfileID)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0)
	for key, entry := range entries {
		if entry.Scope == scope {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	records := make([]TaskRecoveryRecord, 0, len(keys))
	for _, key := range keys {
		records = append(records, entries[key].Record)
	}
	return records, nil
}

func (s *DurableTaskRecoveryStore) Delete(ctx context.Context, record TaskRecoveryRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := record.Validate(); err != nil {
		return err
	}
	path, err := s.path(record.Scope.ProfileID)
	if err != nil {
		return err
	}
	lock := durablePathLock(path)
	lock.Lock()
	defer lock.Unlock()
	entries, err := readDurableTaskRecovery(path, record.Scope.ProfileID)
	if err != nil {
		return err
	}
	entry := durableTaskRecoveryEntry{Scope: record.Scope, Key: taskRecoveryRecordKey(record), Record: record}
	key := durableTaskRecoveryEntryKey(entry)
	current, exists := entries[key]
	if !exists || current.Record != record {
		return nil
	}
	delete(entries, key)
	return writeDurableTaskRecovery(path, record.Scope.ProfileID, entries)
}

func (s *DurableTaskRecoveryStore) StageEventSync(
	ctx context.Context,
	scope BindingScope,
	eventID string,
	records []TaskRecoveryRecord,
) error {
	if err := validateTaskRecoveryEventGCInput(ctx, scope, eventID, records); err != nil {
		return err
	}
	path, err := s.path(scope.ProfileID)
	if err != nil {
		return err
	}
	lock := durablePathLock(path)
	lock.Lock()
	defer lock.Unlock()
	entries, err := readDurableTaskRecovery(path, scope.ProfileID)
	if err != nil {
		return err
	}
	changed := false
	for _, record := range records {
		candidate := durableTaskRecoveryEntry{Scope: scope, Key: taskRecoveryRecordKey(record), Record: record}
		key := durableTaskRecoveryEntryKey(candidate)
		entry, found := entries[key]
		if !found || entry.Record != record || containsString(entry.AuthorizedEventIDs, eventID) || containsString(entry.PendingEventIDs, eventID) {
			continue
		}
		entry.PendingEventIDs = append(entry.PendingEventIDs, eventID)
		sort.Strings(entry.PendingEventIDs)
		entries[key] = entry
		changed = true
	}
	if !changed {
		return nil
	}
	return writeDurableTaskRecovery(path, scope.ProfileID, entries)
}

func (s *DurableTaskRecoveryStore) AuthorizeEventSync(ctx context.Context, scope BindingScope, eventID string) error {
	if err := validateTaskRecoveryEventGCInput(ctx, scope, eventID, nil); err != nil {
		return err
	}
	path, err := s.path(scope.ProfileID)
	if err != nil {
		return err
	}
	lock := durablePathLock(path)
	lock.Lock()
	defer lock.Unlock()
	entries, err := readDurableTaskRecovery(path, scope.ProfileID)
	if err != nil {
		return err
	}
	changed := false
	for key, entry := range entries {
		if entry.Scope != scope || !containsString(entry.PendingEventIDs, eventID) {
			continue
		}
		entry.PendingEventIDs = removeString(entry.PendingEventIDs, eventID)
		if !containsString(entry.AuthorizedEventIDs, eventID) {
			entry.AuthorizedEventIDs = append(entry.AuthorizedEventIDs, eventID)
			sort.Strings(entry.AuthorizedEventIDs)
		}
		entries[key] = entry
		changed = true
	}
	if !changed {
		return nil
	}
	return writeDurableTaskRecovery(path, scope.ProfileID, entries)
}

func (s *DurableTaskRecoveryStore) LoadAuthorizedEventSync(
	ctx context.Context,
	scope BindingScope,
	eventID string,
) ([]TaskRecoveryRecord, error) {
	if err := validateTaskRecoveryEventGCInput(ctx, scope, eventID, nil); err != nil {
		return nil, err
	}
	path, err := s.path(scope.ProfileID)
	if err != nil {
		return nil, err
	}
	lock := durablePathLock(path)
	lock.Lock()
	defer lock.Unlock()
	entries, err := readDurableTaskRecovery(path, scope.ProfileID)
	if err != nil {
		return nil, err
	}
	records := make([]TaskRecoveryRecord, 0)
	for _, entry := range entries {
		if entry.Scope == scope && containsString(entry.AuthorizedEventIDs, eventID) {
			records = append(records, entry.Record)
		}
	}
	sort.Slice(records, func(i, j int) bool { return taskRecoveryRecordKey(records[i]) < taskRecoveryRecordKey(records[j]) })
	return records, nil
}

func (s *DurableTaskRecoveryStore) LoadEventSyncGCIDs(ctx context.Context, scope BindingScope) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	path, err := s.path(scope.ProfileID)
	if err != nil {
		return nil, err
	}
	lock := durablePathLock(path)
	lock.Lock()
	defer lock.Unlock()
	entries, err := readDurableTaskRecovery(path, scope.ProfileID)
	if err != nil {
		return nil, err
	}
	ids := make(map[string]struct{})
	for _, entry := range entries {
		if entry.Scope != scope {
			continue
		}
		for _, eventID := range entry.PendingEventIDs {
			ids[eventID] = struct{}{}
		}
		for _, eventID := range entry.AuthorizedEventIDs {
			ids[eventID] = struct{}{}
		}
	}
	ordered := make([]string, 0, len(ids))
	for eventID := range ids {
		ordered = append(ordered, eventID)
	}
	sort.Strings(ordered)
	return ordered, nil
}

func (s *DurableTaskRecoveryStore) path(profileID string) (string, error) {
	return durableCloudFile(s.paths, userctx.ActorContext{ProfileID: profileID}, "task_recovery.json")
}

func readDurableTaskRecovery(path, profileID string) (map[string]durableTaskRecoveryEntry, error) {
	var document durableTaskRecoveryDocument
	found, err := durableReadJSON(path, &document)
	if err != nil {
		return nil, err
	}
	if !found {
		return make(map[string]durableTaskRecoveryEntry), nil
	}
	if document.Schema != durableTaskRecoverySchema || document.Version != durableTaskRecoveryVersion {
		return nil, fmt.Errorf("cloud: unsupported durable task recovery schema")
	}
	if document.ProfileID != profileID {
		return nil, fmt.Errorf("cloud: durable task recovery belongs to another profile")
	}
	entries := make(map[string]durableTaskRecoveryEntry, len(document.Entries))
	for _, entry := range document.Entries {
		if err := validateDurableTaskRecoveryEntry(profileID, entry); err != nil {
			return nil, err
		}
		key := durableTaskRecoveryEntryKey(entry)
		if _, exists := entries[key]; exists {
			return nil, fmt.Errorf("cloud: duplicate durable task recovery record")
		}
		entries[key] = entry
	}
	return entries, nil
}

func validateDurableTaskRecoveryEntry(profileID string, entry durableTaskRecoveryEntry) error {
	if err := entry.Scope.Validate(); err != nil {
		return fmt.Errorf("cloud: invalid durable task recovery scope: %w", err)
	}
	if entry.Scope.ProfileID != profileID {
		return fmt.Errorf("cloud: durable task recovery entry belongs to another profile")
	}
	if err := entry.Record.Validate(); err != nil {
		return fmt.Errorf("cloud: invalid durable task recovery record: %w", err)
	}
	if entry.Record.Scope != entry.Scope {
		return fmt.Errorf("cloud: durable task recovery record scope mismatch")
	}
	if entry.Key != taskRecoveryRecordKey(entry.Record) {
		return fmt.Errorf("cloud: durable task recovery record key mismatch")
	}
	if err := validateTaskRecoveryEventIDs(entry.PendingEventIDs, entry.AuthorizedEventIDs); err != nil {
		return err
	}
	return nil
}

func validateTaskRecoveryEventIDs(pending, authorized []string) error {
	seen := make(map[string]string, len(pending)+len(authorized))
	for _, group := range []struct {
		name string
		ids  []string
	}{{"pending", pending}, {"authorized", authorized}} {
		for _, eventID := range group.ids {
			if err := requireTrimmed("event_id", eventID); err != nil {
				return err
			}
			if prior, found := seen[eventID]; found {
				return fmt.Errorf("cloud: duplicate task recovery event ID in %s and %s", prior, group.name)
			}
			seen[eventID] = group.name
		}
	}
	return nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func removeString(values []string, target string) []string {
	for i, value := range values {
		if value == target {
			return append(values[:i], values[i+1:]...)
		}
	}
	return values
}

func durableTaskRecoveryEntryKey(entry durableTaskRecoveryEntry) string {
	return entry.Scope.key() + "\x00" + entry.Key
}

func writeDurableTaskRecovery(path, profileID string, entries map[string]durableTaskRecoveryEntry) error {
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ordered := make([]durableTaskRecoveryEntry, 0, len(keys))
	for _, key := range keys {
		ordered = append(ordered, entries[key])
	}
	document := durableTaskRecoveryDocument{Schema: durableTaskRecoverySchema, Version: durableTaskRecoveryVersion, ProfileID: profileID, Entries: ordered}
	return durableWriteJSON(path, document)
}

var _ TaskRecoveryStore = (*DurableTaskRecoveryStore)(nil)

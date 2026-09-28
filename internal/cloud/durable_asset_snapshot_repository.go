package cloud

import (
	"context"
	"fmt"
	"sort"

	"kwu/internal/userctx"
)

const (
	durableAssetSnapshotsSchema  = "reasonix.cloud.asset_snapshots"
	durableAssetSnapshotsVersion = 1
)

type durableAssetSnapshotsDocument struct {
	Schema    string                      `json:"schema"`
	Version   int                         `json:"version"`
	ProfileID string                      `json:"profile_id"`
	Entries   []durableAssetSnapshotEntry `json:"entries"`
}

type durableAssetSnapshotEntry struct {
	Scope        BindingScope      `json:"scope"`
	Balance      *BalanceSnapshot  `json:"balance,omitempty"`
	Subscription *SubscriptionView `json:"subscription,omitempty"`
}

type DurableAssetSnapshotRepository struct {
	paths ProfilePathProvider
}

func NewDurableAssetSnapshotRepository(paths ProfilePathProvider) *DurableAssetSnapshotRepository {
	return &DurableAssetSnapshotRepository{paths: paths}
}

func (r *DurableAssetSnapshotRepository) LoadBalance(ctx context.Context, scope BindingScope) (BalanceSnapshot, bool, error) {
	if err := validateAssetSnapshotContext(ctx, scope); err != nil {
		return BalanceSnapshot{}, false, err
	}
	entries, err := r.read(scope.ProfileID)
	if err != nil {
		return BalanceSnapshot{}, false, err
	}
	entry, ok := entries[scope.key()]
	if !ok || entry.Balance == nil {
		return BalanceSnapshot{}, false, nil
	}
	return *entry.Balance, true, nil
}

func (r *DurableAssetSnapshotRepository) StoreBalance(ctx context.Context, scope BindingScope, snapshot BalanceSnapshot) (BalanceSnapshot, error) {
	if err := validateBalanceAssetSnapshot(ctx, scope, snapshot); err != nil {
		return BalanceSnapshot{}, err
	}
	path, err := r.path(scope.ProfileID)
	if err != nil {
		return BalanceSnapshot{}, err
	}
	lock := durablePathLock(path)
	lock.Lock()
	defer lock.Unlock()
	entries, err := readDurableAssetSnapshots(path, scope.ProfileID)
	if err != nil {
		return BalanceSnapshot{}, err
	}
	entry := entries[scope.key()]
	if entry.Balance == nil || snapshot.Version > entry.Balance.Version {
		stored := snapshot
		entry.Scope = scope
		entry.Balance = &stored
		entries[scope.key()] = entry
		if err := writeDurableAssetSnapshots(path, scope.ProfileID, entries); err != nil {
			return BalanceSnapshot{}, err
		}
	}
	return *entry.Balance, nil
}

func (r *DurableAssetSnapshotRepository) LoadSubscription(ctx context.Context, scope BindingScope) (SubscriptionView, bool, error) {
	if err := validateAssetSnapshotContext(ctx, scope); err != nil {
		return SubscriptionView{}, false, err
	}
	entries, err := r.read(scope.ProfileID)
	if err != nil {
		return SubscriptionView{}, false, err
	}
	entry, ok := entries[scope.key()]
	if !ok || entry.Subscription == nil {
		return SubscriptionView{}, false, nil
	}
	return subscriptionView(entry.Subscription.Plans, entry.Subscription.Subscription), true, nil
}

func (r *DurableAssetSnapshotRepository) StoreSubscription(ctx context.Context, scope BindingScope, view SubscriptionView) error {
	if err := validateSubscriptionAssetSnapshot(ctx, scope, view); err != nil {
		return err
	}
	path, err := r.path(scope.ProfileID)
	if err != nil {
		return err
	}
	lock := durablePathLock(path)
	lock.Lock()
	defer lock.Unlock()
	entries, err := readDurableAssetSnapshots(path, scope.ProfileID)
	if err != nil {
		return err
	}
	entry := entries[scope.key()]
	stored := subscriptionView(view.Plans, view.Subscription)
	entry.Scope = scope
	entry.Subscription = &stored
	entries[scope.key()] = entry
	return writeDurableAssetSnapshots(path, scope.ProfileID, entries)
}

func (r *DurableAssetSnapshotRepository) Clear(ctx context.Context, scope BindingScope) error {
	if err := validateAssetSnapshotContext(ctx, scope); err != nil {
		return err
	}
	path, err := r.path(scope.ProfileID)
	if err != nil {
		return err
	}
	lock := durablePathLock(path)
	lock.Lock()
	defer lock.Unlock()
	entries, err := readDurableAssetSnapshots(path, scope.ProfileID)
	if err != nil {
		return err
	}
	if _, ok := entries[scope.key()]; !ok {
		return nil
	}
	delete(entries, scope.key())
	return writeDurableAssetSnapshots(path, scope.ProfileID, entries)
}

func (r *DurableAssetSnapshotRepository) read(profileID string) (map[string]durableAssetSnapshotEntry, error) {
	path, err := r.path(profileID)
	if err != nil {
		return nil, err
	}
	lock := durablePathLock(path)
	lock.Lock()
	defer lock.Unlock()
	return readDurableAssetSnapshots(path, profileID)
}

func (r *DurableAssetSnapshotRepository) path(profileID string) (string, error) {
	return durableCloudFile(r.paths, userctx.ActorContext{ProfileID: profileID}, "asset_snapshots.json")
}

func readDurableAssetSnapshots(path, profileID string) (map[string]durableAssetSnapshotEntry, error) {
	var document durableAssetSnapshotsDocument
	found, err := durableReadJSON(path, &document)
	if err != nil {
		return nil, err
	}
	if !found {
		return make(map[string]durableAssetSnapshotEntry), nil
	}
	if document.Schema != durableAssetSnapshotsSchema || document.Version != durableAssetSnapshotsVersion {
		return nil, fmt.Errorf("cloud: unsupported durable asset snapshot schema")
	}
	if document.ProfileID != profileID {
		return nil, fmt.Errorf("cloud: durable asset snapshots belong to another profile")
	}
	entries := make(map[string]durableAssetSnapshotEntry, len(document.Entries))
	for _, entry := range document.Entries {
		if err := validateDurableAssetSnapshotEntry(document.ProfileID, entry); err != nil {
			return nil, err
		}
		key := entry.Scope.key()
		if _, exists := entries[key]; exists {
			return nil, fmt.Errorf("cloud: duplicate durable asset snapshot scope")
		}
		entries[key] = entry
	}
	return entries, nil
}

func validateDurableAssetSnapshotEntry(profileID string, entry durableAssetSnapshotEntry) error {
	if err := entry.Scope.Validate(); err != nil {
		return fmt.Errorf("cloud: invalid durable asset snapshot scope: %w", err)
	}
	if entry.Scope.ProfileID != profileID {
		return fmt.Errorf("cloud: durable asset snapshot belongs to another profile")
	}
	if entry.Balance == nil && entry.Subscription == nil {
		return fmt.Errorf("cloud: durable asset snapshot entry is empty")
	}
	if entry.Balance != nil {
		if err := entry.Balance.Validate(); err != nil {
			return fmt.Errorf("cloud: invalid durable balance snapshot: %w", err)
		}
	}
	if entry.Subscription != nil {
		if err := validateSubscriptionAssetSnapshot(context.Background(), entry.Scope, *entry.Subscription); err != nil {
			return fmt.Errorf("cloud: invalid durable subscription snapshot: %w", err)
		}
	}
	return nil
}

func writeDurableAssetSnapshots(path, profileID string, entries map[string]durableAssetSnapshotEntry) error {
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ordered := make([]durableAssetSnapshotEntry, 0, len(keys))
	for _, key := range keys {
		ordered = append(ordered, entries[key])
	}
	document := durableAssetSnapshotsDocument{Schema: durableAssetSnapshotsSchema, Version: durableAssetSnapshotsVersion, ProfileID: profileID, Entries: ordered}
	return durableWriteJSON(path, document)
}

var _ AssetSnapshotRepository = (*DurableAssetSnapshotRepository)(nil)

package automation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"kwu/internal/fileutil"
)

const (
	leasesDirName     = "leases"
	leaseLockSuffix   = ".lock"
	leaseLockWait     = 5 * time.Second
	leaseLockStale    = 30 * time.Second
	leaseLockInterval = 5 * time.Millisecond
)

// Lease records task-level execution ownership for one host/run.
type Lease struct {
	TaskID      string    `json:"taskId"`
	RunID       string    `json:"runId"`
	Owner       string    `json:"owner"`
	AcquiredAt  time.Time `json:"acquiredAt"`
	ExpiresAt   time.Time `json:"expiresAt"`
	HeartbeatAt time.Time `json:"heartbeatAt"`
}

// AcquireLease 用任务级文件租约保证同一任务同一时间只被一个宿主执行。
// 抢不到租约不是任务失败，只表示当前进程不是这次 run 的 owner。
func (s *Store) AcquireLease(taskID, runID, owner string, ttl time.Duration) (Lease, bool, error) {
	taskID, runID, owner, err := normalizeLeaseInput(taskID, runID, owner, ttl)
	if err != nil {
		return Lease{}, false, err
	}
	path, err := s.leasePath(taskID)
	if err != nil {
		return Lease{}, false, err
	}
	unlock, err := lockLeaseFile(path)
	if err != nil {
		return Lease{}, false, err
	}
	defer unlock()

	now := time.Now().UTC()
	existing, err := readLease(path)
	if err == nil && s.leaseHeld(existing, runID, owner, now) {
		return existing, false, nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Lease{}, false, err
	}

	lease := Lease{
		TaskID:      taskID,
		RunID:       runID,
		Owner:       owner,
		AcquiredAt:  now,
		ExpiresAt:   now.Add(ttl),
		HeartbeatAt: now,
	}
	if err := writeLease(path, lease); err != nil {
		return Lease{}, false, err
	}
	return lease, true, nil
}

func (s *Store) leaseHeld(lease Lease, runID, owner string, now time.Time) bool {
	// Local recovery must prove death before a lease may be replaced, even
	// after TTL expiry. Same-owner/run refresh retains the existing contract.
	return (lease.Owner != owner || lease.RunID != runID) && (s.ownerProbe != nil || lease.ExpiresAt.After(now))
}

type OwnerState uint8

const (
	OwnerUnknown OwnerState = iota
	OwnerAlive
	OwnerDead
)

type OwnerProbe func(owner string) OwnerState

// RecoverInterruptedRuns changes only run metadata and matching orphan leases.
// Manifests remain authoritative; SaveRunManifest's dirty marker repairs the
// disposable index after an interruption between the two persistence steps.
func (s *Store) RecoverInterruptedRuns(ctx context.Context, taskID string) ([]RunManifest, error) {
	if s.ownerProbe == nil {
		return nil, nil
	}
	tasks, err := s.recoveryTaskIDs(taskID)
	if err != nil {
		return nil, err
	}
	var recovered []RunManifest
	for _, id := range tasks {
		if err := ctx.Err(); err != nil {
			return recovered, err
		}
		runs, err := s.recoverTaskRuns(ctx, id)
		recovered = append(recovered, runs...)
		if err != nil {
			return recovered, err
		}
	}
	return recovered, nil
}

func (s *Store) recoveryTaskIDs(taskID string) ([]string, error) {
	if taskID != "" {
		return []string{taskID}, nil
	}
	runs, err := s.ListRuns("", 0)
	if err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	for _, run := range runs {
		ids[run.TaskID] = true
	}
	if err := s.addLeaseTaskIDs(ids); err != nil {
		return nil, err
	}
	result := make([]string, 0, len(ids))
	for id := range ids {
		result = append(result, id)
	}
	sort.Strings(result)
	return result, nil
}

func (s *Store) addLeaseTaskIDs(ids map[string]bool) error {
	entries, err := os.ReadDir(filepath.Join(s.dir, leasesDirName))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		lease, err := readLease(filepath.Join(s.dir, leasesDirName, entry.Name()))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		ids[lease.TaskID] = true
	}
	return nil
}

func (s *Store) recoverTaskRuns(ctx context.Context, taskID string) ([]RunManifest, error) {
	path, err := s.leasePath(taskID)
	if err != nil {
		return nil, err
	}
	unlock, err := lockLeaseFile(path)
	if err != nil {
		return nil, err
	}
	defer unlock()
	recovered, err := s.recoverTaskManifests(ctx, taskID)
	if err != nil {
		return recovered, err
	}
	return recovered, s.releaseDeadLease(taskID, path)
}

func (s *Store) recoverTaskManifests(ctx context.Context, taskID string) ([]RunManifest, error) {
	runs, err := s.ListRuns(taskID, 0)
	if err != nil {
		return nil, err
	}
	var recovered []RunManifest
	for _, candidate := range runs {
		if err := ctx.Err(); err != nil {
			return recovered, err
		}
		if terminalRunStatus(candidate.Status) || s.ownerProbe(candidate.Owner) != OwnerDead {
			continue
		}
		run, changed, err := s.recoverRun(candidate)
		if err != nil {
			return recovered, err
		}
		if changed {
			recovered = append(recovered, run)
		}
	}
	return recovered, nil
}

func (s *Store) releaseDeadLease(taskID, path string) error {
	lease, err := readLease(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	// Re-read under the same task lock: a newer owner's lease is never removed
	// merely because an older manifest was recovered above.
	if lease.TaskID == taskID && s.ownerProbe(lease.Owner) == OwnerDead {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (s *Store) recoverRun(candidate RunSummary) (RunManifest, bool, error) {
	s.runIndexMu.Lock()
	defer s.runIndexMu.Unlock()
	run, err := s.ReadRunManifest(candidate.TaskID, candidate.RunID)
	if err != nil {
		return RunManifest{}, false, err
	}
	owner := firstNonEmptyString(run.Owner, run.HostOwner)
	if run.TaskID != candidate.TaskID || run.RunID != candidate.RunID || owner != candidate.Owner || terminalRunStatus(run.Status) || s.ownerProbe(owner) != OwnerDead {
		return run, false, nil
	}
	reason := "automation interrupted: owning desktop process exited"
	if err := run.Transition(RunStatusFailed, reason, time.Now()); err != nil {
		return run, false, err
	}
	run.ErrorCode = RunErrorRunnerFailed
	run.ErrorMessage = reason
	run.Error = reason
	if err := s.saveRunManifestLocked(run); err != nil {
		return run, false, err
	}
	return run, true, nil
}

// RefreshLease extends a lease only when owner/run still match the file.
func (s *Store) RefreshLease(taskID, runID, owner string, ttl time.Duration) error {
	taskID, runID, owner, err := normalizeLeaseInput(taskID, runID, owner, ttl)
	if err != nil {
		return err
	}
	path, err := s.leasePath(taskID)
	if err != nil {
		return err
	}
	unlock, err := lockLeaseFile(path)
	if err != nil {
		return err
	}
	defer unlock()

	lease, err := readLease(path)
	if err != nil {
		return err
	}
	if lease.Owner != owner || lease.RunID != runID {
		return fmt.Errorf("automation: lease not owned by run")
	}
	now := time.Now().UTC()
	lease.HeartbeatAt = now
	lease.ExpiresAt = now.Add(ttl)
	return writeLease(path, lease)
}

// ReleaseLease removes a lease only when owner/run still match the file.
func (s *Store) ReleaseLease(taskID, runID, owner string) error {
	taskID = strings.TrimSpace(taskID)
	runID = strings.TrimSpace(runID)
	owner = strings.TrimSpace(owner)
	if taskID == "" || runID == "" || owner == "" {
		return errors.New("automation: lease task id, run id, and owner are required")
	}
	path, err := s.leasePath(taskID)
	if err != nil {
		return err
	}
	unlock, err := lockLeaseFile(path)
	if err != nil {
		return err
	}
	defer unlock()

	lease, err := readLease(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if lease.Owner != owner || lease.RunID != runID {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("release lease: %w", err)
	}
	return nil
}

func (s *Store) leasePath(taskID string) (string, error) {
	taskPart, err := pathID(taskID)
	if err != nil {
		return "", fmt.Errorf("task id: %w", err)
	}
	path := filepath.Join(s.dir, leasesDirName, taskPart+".json")
	if err := ensureUnderRoot(s.dir, path); err != nil {
		return "", err
	}
	return path, nil
}

func normalizeLeaseInput(taskID, runID, owner string, ttl time.Duration) (string, string, string, error) {
	taskID = strings.TrimSpace(taskID)
	runID = strings.TrimSpace(runID)
	owner = strings.TrimSpace(owner)
	if taskID == "" || runID == "" || owner == "" {
		return "", "", "", errors.New("automation: lease task id, run id, and owner are required")
	}
	if ttl <= 0 {
		return "", "", "", errors.New("automation: lease ttl must be positive")
	}
	return taskID, runID, owner, nil
}

func readLease(path string) (Lease, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Lease{}, err
	}
	var lease Lease
	if err := json.Unmarshal(b, &lease); err != nil {
		return Lease{}, fmt.Errorf("read lease: decode %s: %w", path, err)
	}
	return lease, nil
}

func writeLease(path string, lease Lease) error {
	b, err := json.MarshalIndent(lease, "", "  ")
	if err != nil {
		return fmt.Errorf("write lease: encode: %w", err)
	}
	b = append(b, '\n')
	if err := fileutil.AtomicWriteFile(path, b, 0o600); err != nil {
		return fmt.Errorf("write lease: %w", err)
	}
	return nil
}

func lockLeaseFile(path string) (func(), error) {
	lockPath := path + leaseLockSuffix
	deadline := time.Now().Add(leaseLockWait)
	for {
		if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
			return nil, err
		}
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_ = f.Close()
			return func() { _ = os.Remove(lockPath) }, nil
		}
		info, statErr := os.Stat(lockPath)
		if statErr == nil && time.Since(info.ModTime()) > leaseLockStale {
			_ = os.Remove(lockPath)
			continue
		}
		held := os.IsExist(err) || errors.Is(err, os.ErrPermission) || statErr == nil
		if !held || time.Now().After(deadline) {
			return nil, fmt.Errorf("lock lease: %w", err)
		}
		time.Sleep(leaseLockInterval)
	}
}

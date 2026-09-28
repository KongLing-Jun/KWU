package automation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrRunOwnedByAnotherProcess = errors.New("automation: active run is owned by another process")
	ErrRunCancellationTimeout   = errors.New("automation: run cancellation timed out")
	ErrRunNotDeletable          = errors.New("automation: run did not reach a deletable state")
)

type RenameRunRequest struct {
	TaskID      string `json:"taskId"`
	RunID       string `json:"runId"`
	Scope       string `json:"scope,omitempty"`
	DisplayName string `json:"displayName"`
}

type TrashRunRequest struct {
	TaskID string `json:"taskId"`
	RunID  string `json:"runId"`
	Scope  string `json:"scope,omitempty"`
}

type RestoreRunRequest = TrashRunRequest

func (s *service) RenameRun(ctx context.Context, req RenameRunRequest) (RunManifest, error) {
	if err := ctx.Err(); err != nil {
		return RunManifest{}, err
	}
	if err := s.requireStore(); err != nil {
		return RunManifest{}, err
	}
	manifest, err := s.store.ReadRunManifest(strings.TrimSpace(req.TaskID), strings.TrimSpace(req.RunID))
	if err != nil {
		return RunManifest{}, err
	}
	if err := validateRunScope(manifest, req.Scope); err != nil {
		return RunManifest{}, err
	}
	return s.store.RenameRun(strings.TrimSpace(req.TaskID), strings.TrimSpace(req.RunID), req.DisplayName)
}

func (s *service) TrashRun(ctx context.Context, req TrashRunRequest) (RunManifest, error) {
	if err := ctx.Err(); err != nil {
		return RunManifest{}, err
	}
	if err := s.requireStore(); err != nil {
		return RunManifest{}, err
	}
	manifest, err := s.store.ReadRunManifest(strings.TrimSpace(req.TaskID), strings.TrimSpace(req.RunID))
	if err != nil {
		return RunManifest{}, err
	}
	if err := validateRunScope(manifest, req.Scope); err != nil {
		return RunManifest{}, err
	}
	if _, err := s.prepareRunForTrash(ctx, manifest); err != nil {
		return RunManifest{}, err
	}
	return s.store.TrashRun(strings.TrimSpace(req.TaskID), strings.TrimSpace(req.RunID))
}

func (s *service) prepareRunForTrash(ctx context.Context, manifest RunManifest) (RunManifest, error) {
	if terminalRunStatus(manifest.Status) {
		return manifest, nil
	}
	if !s.running.cancelRun(manifest.TaskID, manifest.RunID) {
		return RunManifest{}, fmt.Errorf("%w and cannot be deleted", ErrRunOwnedByAnotherProcess)
	}
	waitTimeout := s.runCancellationWaitTimeout
	if waitTimeout <= 0 {
		waitTimeout = defaultRunCancellationWaitTimeout
	}
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(waitTimeout)
	defer timer.Stop()
	for s.running.active(manifest.TaskID, manifest.RunID) {
		select {
		case <-ctx.Done():
			return RunManifest{}, fmt.Errorf("automation: wait for run cancellation: %w", ctx.Err())
		case <-timer.C:
			return RunManifest{}, fmt.Errorf("%w after %s", ErrRunCancellationTimeout, waitTimeout)
		case <-ticker.C:
		}
	}
	updated, err := s.store.ReadRunManifest(manifest.TaskID, manifest.RunID)
	if err != nil {
		return RunManifest{}, err
	}
	if !terminalRunStatus(updated.Status) {
		return RunManifest{}, fmt.Errorf("%w after cancellation", ErrRunNotDeletable)
	}
	return updated, nil
}

func (s *service) RestoreRun(ctx context.Context, req RestoreRunRequest) (RunManifest, error) {
	if err := ctx.Err(); err != nil {
		return RunManifest{}, err
	}
	if err := s.requireStore(); err != nil {
		return RunManifest{}, err
	}
	manifest, err := s.store.ReadTrashedRunManifest(strings.TrimSpace(req.TaskID), strings.TrimSpace(req.RunID))
	if err != nil {
		return RunManifest{}, err
	}
	if err := validateRunScope(manifest, req.Scope); err != nil {
		return RunManifest{}, err
	}
	return s.store.RestoreRun(strings.TrimSpace(req.TaskID), strings.TrimSpace(req.RunID))
}

func validateRunScope(manifest RunManifest, expectedScope string) error {
	expectedScope = strings.ToLower(strings.TrimSpace(expectedScope))
	if expectedScope != "" && !strings.EqualFold(strings.TrimSpace(manifest.Scope), expectedScope) {
		return errors.New("automation: run scope identity mismatch")
	}
	return nil
}

package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"kwu/internal/store"
)

// StartControlledForSession launches a session-owned job with a stable,
// host-only control identity. The gate prevents a fast job from finishing
// before its control identity has been attached and persisted.
func (m *Manager) StartControlledForSession(parentSession, kind, controlID, label string, run func(context.Context, io.Writer) (string, error)) *Job {
	return m.StartControlledForSessionWithDependencies(parentSession, kind, controlID, label, nil, run)
}

// StartControlledForSessionWithDependencies starts a controlled job and stores
// task-run lineage as passive metadata. Dependencies never trigger execution.
func (m *Manager) StartControlledForSessionWithDependencies(parentSession, kind, controlID, label string, dependsOn []string, run func(context.Context, io.Writer) (string, error)) *Job {
	return m.startControlled(parentSession, kind, controlID, label, dependsOn, func(run func(context.Context, io.Writer) (string, error)) *Job {
		return m.StartForSession(parentSession, kind, label, run)
	}, run)
}

// StartControlledForTurn is StartControlledForSession with a parent-turn
// lifetime. The stable control identity remains usable by wait/stop while the
// job inherits the launching turn's context values and cancellation.
func (m *Manager) StartControlledForTurn(parentCtx context.Context, parentSession, kind, controlID, label string, run func(context.Context, io.Writer) (string, error)) *Job {
	return m.StartControlledForTurnWithDependencies(parentCtx, parentSession, kind, controlID, label, nil, run)
}

func (m *Manager) StartControlledForTurnWithDependencies(parentCtx context.Context, parentSession, kind, controlID, label string, dependsOn []string, run func(context.Context, io.Writer) (string, error)) *Job {
	return m.startControlled(parentSession, kind, controlID, label, dependsOn, func(run func(context.Context, io.Writer) (string, error)) *Job {
		return m.StartForTurn(parentCtx, parentSession, kind, label, run)
	}, run)
}

func (m *Manager) startControlled(parentSession, kind, controlID, label string, dependsOn []string, start func(func(context.Context, io.Writer) (string, error)) *Job, run func(context.Context, io.Writer) (string, error)) *Job {
	ready := make(chan struct{})
	job := start(func(ctx context.Context, out io.Writer) (string, error) {
		<-ready
		return run(ctx, out)
	})
	job.mu.Lock()
	job.ControlID = strings.TrimSpace(controlID)
	job.DependsOn = append([]string(nil), dependsOn...)
	job.mu.Unlock()
	close(ready)
	return job
}

func (m *Manager) findControlledJobLocked(parentSession, kind, controlID string) *Job {
	parentSession = strings.TrimSpace(parentSession)
	kind = strings.TrimSpace(kind)
	controlID = strings.TrimSpace(controlID)
	for i := len(m.order) - 1; i >= 0; i-- {
		job := m.jobs[m.order[i]]
		if job != nil && sessionMatches(parentSession, job.SessionID) && job.Kind == kind && job.ControlID == controlID {
			return job
		}
	}
	return nil
}

// KillControlledForSession cancels the latest running job addressed by its
// stable control identity without exposing the transient job ID.
func (m *Manager) KillControlledForSession(parentSession, kind, controlID string) bool {
	m.mu.Lock()
	job := m.findControlledJobLocked(parentSession, kind, controlID)
	m.mu.Unlock()
	if job == nil {
		return false
	}
	job.mu.Lock()
	running := job.status == Running
	if running {
		job.status = Killed
	}
	job.mu.Unlock()
	if running {
		job.cancel()
	}
	return running
}

// WaitControlledForSession waits for the latest job addressed by a stable
// control identity. ok is false when no matching job exists.
func (m *Manager) WaitControlledForSession(ctx context.Context, parentSession, kind, controlID string, timeoutSec int) (Result, bool) {
	m.mu.Lock()
	job := m.findControlledJobLocked(parentSession, kind, controlID)
	m.mu.Unlock()
	if job == nil {
		return Result{}, false
	}
	var timeout <-chan time.Time
	if timeoutSec > 0 {
		timer := time.NewTimer(time.Duration(timeoutSec) * time.Second)
		defer timer.Stop()
		timeout = timer.C
	}
	select {
	case <-job.done:
	case <-ctx.Done():
	case <-timeout:
	}
	return m.results([]*Job{job})[0], true
}

const (
	jobLogExt        = ".log"
	jobMetaExt       = ".json"
	defaultTailBytes = 64 * 1024
)

// ArtifactDir returns the sidecar directory for a persistent session transcript.
func ArtifactDir(sessionPath string) string {
	return store.SessionJobsDir(sessionPath)
}

// RemoveArtifacts removes the job sidecar for a session transcript.
func RemoveArtifacts(sessionPath string) error {
	dir := ArtifactDir(sessionPath)
	if dir == "" {
		return nil
	}
	return os.RemoveAll(dir)
}

type artifactMeta struct {
	ID               string   `json:"id"`
	Kind             string   `json:"kind"`
	ControlID        string   `json:"controlId,omitempty"`
	Label            string   `json:"label,omitempty"`
	SessionID        string   `json:"sessionId,omitempty"`
	DependsOn        []string `json:"dependsOn,omitempty"`
	Status           Status   `json:"status"`
	StartedAt        int64    `json:"startedAt"`
	FinishedAt       int64    `json:"finishedAt,omitempty"`
	ArtifactComplete bool     `json:"artifactComplete"`
	ArtifactError    string   `json:"artifactError,omitempty"`
	LogPath          string   `json:"logPath,omitempty"`
}

func (m *Manager) writeJobMetaLocked(job *Job, status Status) error {
	if job.artifactMetaPath == "" {
		return nil
	}
	return writeMeta(job.artifactMetaPath, artifactMeta{
		ID: job.ID, Kind: job.Kind, ControlID: job.ControlID, Label: job.Label,
		SessionID: job.SessionID, DependsOn: append([]string(nil), job.DependsOn...), Status: status, StartedAt: job.startedAt,
		FinishedAt: job.finishedAt, ArtifactComplete: job.artifactComplete && job.artifactErr == "",
		ArtifactError: job.artifactErr, LogPath: filepath.Base(job.artifactPath),
	})
}

func writeMeta(path string, meta artifactMeta) error {
	if path == "" {
		return fmt.Errorf("empty metadata path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".job-meta-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, path)
}

func readMeta(path string) (artifactMeta, error) {
	var meta artifactMeta
	b, err := os.ReadFile(path)
	if err != nil {
		return meta, err
	}
	if err := json.Unmarshal(b, &meta); err != nil {
		return meta, err
	}
	return meta, nil
}

func maxJobSeq(id string) int {
	_, tail, ok := strings.Cut(id, "-")
	if !ok {
		return 0
	}
	n, err := strconv.Atoi(tail)
	if err != nil {
		return 0
	}
	return n
}

func appendTail(buf []byte, p []byte, limit int) []byte {
	if limit <= 0 {
		return nil
	}
	if len(p) >= limit {
		out := make([]byte, limit)
		copy(out, p[len(p)-limit:])
		return out
	}
	total := len(buf) + len(p)
	if total <= limit {
		out := make([]byte, total)
		copy(out, buf)
		copy(out[len(buf):], p)
		return out
	}
	keep := limit - len(p)
	out := make([]byte, limit)
	copy(out, buf[len(buf)-keep:])
	copy(out[keep:], p)
	return out
}

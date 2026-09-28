package jobs

import (
	"context"
	"fmt"
	"io"
	"strings"

	"kwu/internal/event"
)

// StartForTurn launches a session-owned job whose lifetime is additionally
// bounded by parentCtx while retaining the normal wait/stop/artifact surface.
func (m *Manager) StartForTurn(parentCtx context.Context, parentSession, kind, label string, run func(context.Context, io.Writer) (string, error)) *Job {
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx, cancelCtx := context.WithCancel(parentCtx)
	stopManagerCancel := context.AfterFunc(m.root, cancelCtx)
	cancel := func() {
		stopManagerCancel()
		cancelCtx()
	}
	return m.startForSession(parentSession, kind, label, ctx, cancel, run)
}

func (m *Manager) startForSession(parentSession, kind, label string, ctx context.Context, cancel context.CancelFunc, run func(context.Context, io.Writer) (string, error)) *Job {
	j := m.registerJob(parentSession, kind, label, cancel)
	m.emitIfActive(j.SessionID, event.Event{Kind: event.Notice, Level: event.LevelInfo, Text: startedText(kind, j.ID, label)})
	m.startJobWorkers(j.SessionID, j)
	go m.runRegisteredJob(ctx, j, run, cancel)
	return j
}

func (m *Manager) registerJob(parentSession, kind, label string, cancel context.CancelFunc) *Job {
	parentSession = strings.TrimSpace(parentSession)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	id := fmt.Sprintf("%s-%d", kind, m.seq)
	startedAt := nowMs()
	logPath, metaPath, file, artifactErr := m.openArtifactLocked(parentSession, id)
	j := &Job{
		ID: id, Kind: kind, Label: label, SessionID: parentSession,
		status: Running, startedAt: startedAt, activityAt: startedAt,
		cancel: cancel, done: make(chan struct{}),
		artifactPath: logPath, artifactMetaPath: metaPath, artifactFile: file,
		artifactComplete: artifactErr == "", artifactErr: artifactErr,
	}
	key := jobKey(parentSession, id)
	m.jobs[key] = j
	m.order = append(m.order, key)
	return j
}

func (m *Manager) startJobWorkers(parentSession string, j *Job) {
	m.wg.Add(1)
	if m.stalledWarning <= 0 {
		return
	}
	m.wg.Add(1)
	go m.monitorStalled(parentSession, j)
}

func (m *Manager) runRegisteredJob(ctx context.Context, j *Job, run func(context.Context, io.Writer) (string, error), cancel context.CancelFunc) {
	defer m.wg.Done()
	defer cancel()
	result, err := runRecovered(ctx, jobWriter{j}, run)
	j.markRunReturned()
	status, result := terminalJobOutcome(ctx, result, err)
	j.storeTerminalResult(result)
	m.finalizeJobArtifact(j, m.artifactTargetDirForJob(j), nowMs(), status)
	// Record completion before publishing terminal status so waiters cannot
	// observe completion ahead of its durable note and metadata.
	m.recordCompletion(j.SessionID, j.ID, j.Kind, j.Label, status, err)
	j.publishTerminalStatus(status)
}

func terminalJobOutcome(ctx context.Context, result string, err error) (Status, string) {
	switch {
	case ctx.Err() != nil:
		return Killed, result
	case err != nil:
		if result == "" {
			result = err.Error()
		}
		return Failed, result
	default:
		return Done, result
	}
}

func (j *Job) markRunReturned() {
	j.mu.Lock()
	j.runReturned = true
	j.mu.Unlock()
}

func (j *Job) storeTerminalResult(result string) {
	if result == "" {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.artifactFile == nil {
		j.result = result
	} else if _, err := j.artifactFile.WriteString(result); err != nil {
		j.artifactErr = err.Error()
	}
	j.tail = appendTail(j.tail, []byte(result), defaultTailBytes)
}

func (m *Manager) finalizeJobArtifact(j *Job, targetDir string, finishedAt int64, status Status) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.closeArtifactFile()
	j.finishedAt = finishedAt
	if targetDir != "" {
		if err := j.moveArtifactToDirLocked(targetDir); err != nil {
			j.noteArtifactErr("migration: " + err.Error())
		}
	}
	if err := m.writeJobMetaLocked(j, status); err != nil {
		j.noteArtifactErr("metadata: " + err.Error())
	}
}

func (j *Job) closeArtifactFile() {
	if j.artifactFile != nil {
		if err := j.artifactFile.Close(); err != nil && j.artifactErr == "" {
			j.artifactErr = err.Error()
		}
		j.artifactFile = nil
	}
	if j.artifactErr != "" {
		j.artifactComplete = false
	}
}

func (j *Job) publishTerminalStatus(status Status) {
	j.mu.Lock()
	if j.status != Killed {
		j.status = status
	}
	if j.artifactPath != "" && j.artifactComplete {
		j.result = ""
		j.tail = nil
	}
	j.mu.Unlock()
	close(j.done)
}

// RunningKindCountForSession reports how many running jobs of kind are owned by
// parentSession.
func (m *Manager) RunningKindCountForSession(parentSession, kind string) int {
	parentSession = strings.TrimSpace(parentSession)
	kind = strings.TrimSpace(kind)
	m.mu.Lock()
	defer m.mu.Unlock()
	runningCount := 0
	for _, key := range m.order {
		j := m.jobs[key]
		if j == nil || !sessionMatches(parentSession, j.SessionID) || j.Kind != kind {
			continue
		}
		j.mu.Lock()
		running := j.status == Running
		j.mu.Unlock()
		if running {
			runningCount++
		}
	}
	return runningCount
}

// HasRunningKindForSession reports whether parentSession owns a running job of kind.
func (m *Manager) HasRunningKindForSession(parentSession, kind string) bool {
	return m.RunningKindCountForSession(parentSession, kind) > 0
}

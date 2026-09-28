package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"kwu/desktop/internal/operationaldiag"
	"kwu/internal/event"
)

const operationalDiagnosticCapacity = 256

type operationalEvidenceRuntime struct {
	initMu    sync.Mutex
	captureMu sync.Mutex
	trace     *operationaldiag.Trace
	mode      operationaldiag.EvidenceMode
	directory string
	runID     string
	initCode  string
}

func newOperationalEvidenceRuntime(profile desktopProfile) *operationalEvidenceRuntime {
	return newOperationalEvidenceRuntimeForProcess(profile, os.Args[0])
}

func newOperationalEvidenceRuntimeForProcess(profile desktopProfile, executable string) *operationalEvidenceRuntime {
	runtime := &operationalEvidenceRuntime{mode: operationaldiag.ModeMemory}
	// Go tests opt into persistence explicitly with NewPersistent and t.TempDir;
	// they must never inherit a real user's profile as a side effect of NewApp.
	if profile.Development && !strings.HasSuffix(strings.ToLower(filepath.Base(executable)), ".test") {
		runtime.mode = operationaldiag.ModePersistent
		runtime.directory = filepath.Join(profile.CacheRoot, "diagnostics", "operational")
		runtime.runID = event.NewOperationID()
	}
	return runtime
}

func (r *operationalEvidenceRuntime) Trace() *operationaldiag.Trace {
	if r == nil {
		return nil
	}
	r.initMu.Lock()
	defer r.initMu.Unlock()
	if r.trace != nil {
		return r.trace
	}
	if r.mode == operationaldiag.ModePersistent {
		trace, err := operationaldiag.NewPersistent(operationaldiag.PersistentOptions{
			Directory: r.directory, RunID: r.runID, Capacity: operationalDiagnosticCapacity,
			QueueCapacity: 256, TerminalReserve: 32, MaxFileBytes: 10 << 20, MaxFiles: 5,
		})
		if err == nil {
			r.trace = trace
			return trace
		}
		r.initCode = "persistence_init_failed"
	}
	r.trace = operationaldiag.New(operationalDiagnosticCapacity)
	return r.trace
}

func (a *App) closeOperationalEvidence(ctx context.Context) {
	if a == nil || a.operationalEvidence == nil {
		return
	}
	trace := a.operationalEvidence.Trace()
	if trace == nil {
		return
	}
	closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	_ = trace.Close(closeCtx)
}

func (s *tabEventSink) lockOperationalEvidence() func() {
	if s != nil && s.app != nil && s.app.operationalEvidence != nil {
		s.app.operationalEvidence.captureMu.Lock()
		return s.app.operationalEvidence.captureMu.Unlock
	}
	s.operationalMu.Lock()
	return s.operationalMu.Unlock
}

func (s *tabEventSink) emitOperationalRuntimeEvent(e event.Event, wire wireEventTab) {
	if s == nil || e.Operational == nil {
		return
	}
	unlock := s.lockOperationalEvidence()
	defer unlock()
	trace := s.operationalDiagnosticTrace()
	operationaldiag.NewIsolatedProjector(trace, nil).Project(operationalDiagnosticRecord(e.Operational.Diagnostic()))
	disposition := s.runtimeEvents.EmitControl(s.context(), eventChannel, wire)
	switch disposition {
	case controlEnqueueQueueFull:
		trace.ObserveTransport(operationaldiag.OutcomeQueueFull)
	case controlEnqueueDisabled:
		trace.ObserveTransport(operationaldiag.OutcomeDisabled)
	}
}

func (s *tabEventSink) operationalDiagnosticTrace() *operationaldiag.Trace {
	if s == nil {
		return nil
	}
	if s.app != nil && s.app.operationalEvidence != nil {
		return s.app.operationalEvidence.Trace()
	}
	if trace := s.operationalDiag.Load(); trace != nil {
		return trace
	}
	created := operationaldiag.New(operationalDiagnosticCapacity)
	if s.operationalDiag.CompareAndSwap(nil, created) {
		return created
	}
	return s.operationalDiag.Load()
}

func operationalDiagnosticRecord(d event.OperationalDiagnosticDTO) operationaldiag.Record {
	return operationaldiag.Record{
		SchemaVersion: d.SchemaVersion,
		EventID:       d.EventID, OperationID: d.OperationID, CausationID: d.CausationID, OccurrenceID: d.OccurrenceID,
		Aggregate: operationaldiag.Aggregate{Kind: operationaldiag.AggregateKind(d.AggregateKind), ID: d.AggregateID, Sequence: d.Sequence},
		Scope: operationaldiag.Scope{
			TabID: d.TabID, SessionID: d.SessionID, TurnID: d.TurnID, CandidateID: d.CandidateID, PromptID: d.PromptID,
			TaskID: d.TaskID, CallID: d.CallID, ToolCallID: d.ToolCallID, PlanID: d.PlanID, DashboardWriteID: d.DashboardWriteID,
		},
		PromptKind: operationaldiag.PromptKind(d.PromptKind), Kind: operationaldiag.Kind(d.Kind), Stage: operationaldiag.Stage(d.Stage),
		Outcome: operationaldiag.Outcome(d.Outcome), Severity: operationaldiag.Severity(d.Severity), UserImpact: operationaldiag.UserImpact(d.UserImpact),
		Retryable: d.Retryable, PrivacyClass: operationaldiag.PrivacyMetadataOnly,
		OccurredAt: d.OccurredAt,
	}
}

// ExportOperationalPromptEvidence returns a deterministic, metadata-only JSON
// snapshot for one exact tab. It never falls back to the active tab.
func (a *App) ExportOperationalPromptEvidence(tabID string) (string, error) {
	return a.ExportOperationalEvidence(tabID)
}

// ExportOperationalEvidence returns all supported lifecycle aggregates. The
// old prompt-named method remains as a compatibility alias for existing hosts.
func (a *App) ExportOperationalEvidence(tabID string) (string, error) {
	if a == nil || strings.TrimSpace(tabID) == "" {
		return "", errors.New("operational diagnostics require an exact tab id")
	}
	a.mu.RLock()
	tab := a.tabByEventSinkIDLocked(tabID)
	var sink *tabEventSink
	if tab != nil {
		sink = tab.sink
	}
	a.mu.RUnlock()
	if sink == nil {
		return "", errors.New("operational diagnostics tab not found")
	}
	unlock := sink.lockOperationalEvidence()
	defer unlock()
	captureCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	transportQuiescent := sink.runtimeEvents.WaitControlDrained(captureCtx)
	trace := sink.operationalDiagnosticTrace()
	committed, flushErr := trace.Flush(captureCtx)
	transport := sink.runtimeEvents.ControlStats()
	diagnostics := trace.CaptureForTab(tabID, transportQuiescent && flushErr == nil)
	if diagnostics.Mode == operationaldiag.ModePersistent && committed < diagnostics.LastOrdinal {
		diagnostics.Quiescent = false
		diagnostics.Truncated = true
	}
	initCode := ""
	if a.operationalEvidence != nil {
		initCode = a.operationalEvidence.initCode
	}
	payload, err := json.MarshalIndent(struct {
		SchemaVersion        int                      `json:"schemaVersion"`
		PersistenceErrorCode string                   `json:"persistenceErrorCode,omitempty"`
		Transport            runtimeControlStats      `json:"transport"`
		Diagnostics          operationaldiag.Evidence `json:"diagnostics"`
	}{
		SchemaVersion:        operationaldiag.SchemaVersion,
		PersistenceErrorCode: initCode,
		Transport:            transport,
		Diagnostics:          diagnostics,
	}, "", "  ")
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

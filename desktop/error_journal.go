package main

import (
	"errors"
	"hash/fnv"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"kwu/desktop/internal/diagnostics"
	"kwu/internal/cloud"
	"kwu/internal/config"
)

var desktopErrorJournalState struct {
	sync.Mutex
	base slog.Handler
}

var desktopErrorJournalCurrent atomic.Pointer[diagnostics.Journal]

// ClientErrorFrame is the renderer's bounded crash frame projection. It does
// not carry stack text, function names, or source paths across the Wails seam.
type ClientErrorFrame struct {
	Category string `json:"category"`
	Ordinal  int    `json:"ordinal"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
}

// ClientErrorRecord is the only renderer payload accepted by the local
// failure journal. Source and event are fixed by RecordClientError below.
type ClientErrorRecord struct {
	Scope           string                     `json:"scope"`
	Code            string                     `json:"code"`
	ErrorType       string                     `json:"errorType"`
	OccurredAt      string                     `json:"occurredAt"`
	DiagnosticID    string                     `json:"diagnosticId,omitempty"`
	StackFrames     []ClientErrorFrame         `json:"stackFrames,omitempty"`
	ComponentFrames []ClientErrorFrame         `json:"componentFrames,omitempty"`
	ProjectComposer *ProjectComposerDiagnostic `json:"projectComposer,omitempty"`
}

// RecordClientError best-effort enqueues one structured renderer failure. It
// deliberately never exposes journal I/O failures to the WebView.
func (a *App) RecordClientError(record ClientErrorRecord) {
	journal := currentDesktopErrorJournal()
	if journal == nil {
		return
	}
	if journal.OwnsLogID(record.DiagnosticID) {
		return
	}
	var occurredAt time.Time
	if value := strings.TrimSpace(record.OccurredAt); value != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			occurredAt = parsed
		}
	}
	toFrames := func(frames []ClientErrorFrame) []diagnostics.Frame {
		if len(frames) == 0 {
			return nil
		}
		projected := make([]diagnostics.Frame, 0, len(frames))
		for _, frame := range frames {
			projected = append(projected, diagnostics.Frame{
				Category: frame.Category,
				Ordinal:  frame.Ordinal,
				Line:     frame.Line,
				Column:   frame.Column,
			})
		}
		return projected
	}
	summary, phase := projectComposerDiagnosticSummary(record)
	journal.Record(diagnostics.Event{
		Severity:        "error",
		Source:          "frontend",
		Event:           "client.failure",
		Scope:           record.Scope,
		Code:            record.Code,
		ErrorType:       record.ErrorType,
		DiagnosticID:    record.DiagnosticID,
		Summary:         summary,
		Phase:           phase,
		OccurredAt:      occurredAt,
		StackFrames:     toFrames(record.StackFrames),
		ComponentFrames: toFrames(record.ComponentFrames),
	})
}

func installDesktopErrorJournal(profile desktopProfile) {
	root := strings.TrimSpace(profile.CacheRoot)
	environmentID := profile.EnvironmentID
	if !profile.Development {
		root = strings.TrimSpace(config.CacheDir())
		environmentID = "production"
	}
	if root == "" {
		return
	}
	journalPath := filepath.Join(root, "diagnostics", "errors")
	metadata := diagnostics.Metadata{AppVersion: version, Channel: channel, EnvironmentID: environmentID}
	journal := diagnostics.New(journalPath, diagnostics.WithMetadata(metadata))

	desktopErrorJournalState.Lock()
	if desktopErrorJournalState.base == nil {
		// The standard default handler bridges through log.Default after
		// SetDefault. Keeping it as a delegate would recurse into the fan-out;
		// this text handler preserves the existing stderr behavior directly. A
		// custom handler installed by a host is still retained as the delegate.
		base := slog.Default().Handler()
		if handlerType := reflect.TypeOf(base); handlerType == nil || strings.Contains(handlerType.String(), "slog.defaultHandler") {
			base = slog.NewTextHandler(os.Stderr, nil)
		}
		desktopErrorJournalState.base = base
	}
	old := desktopErrorJournalCurrent.Swap(journal)
	slog.SetDefault(slog.New(diagnostics.NewHandler(desktopErrorJournalState.base, journal)))
	desktopErrorJournalState.Unlock()
	if old != nil {
		old.Close()
	}
}

func currentDesktopErrorJournal() *diagnostics.Journal {
	return desktopErrorJournalCurrent.Load()
}

func closeDesktopErrorJournal() {
	desktopErrorJournalState.Lock()
	journal := desktopErrorJournalCurrent.Swap(nil)
	base := desktopErrorJournalState.base
	desktopErrorJournalState.base = nil
	desktopErrorJournalState.Unlock()
	if journal != nil {
		journal.Close()
	}
	if base != nil {
		slog.SetDefault(slog.New(base))
	}
}

func recordGoPanicFailure(site string) {
	if journal := currentDesktopErrorJournal(); journal != nil {
		hash := fnv.New64a()
		_, _ = hash.Write([]byte(site))
		journal.Record(diagnostics.Event{
			Severity:   "error",
			Source:     "go",
			Event:      "go.panic",
			Scope:      "desktop.panic",
			Code:       "GO_PANIC",
			CallsiteID: hash.Sum64(),
			ErrorType:  "Panic",
		})
	}
}

func recordCloudRequestFailure(operation string, err error) string {
	return recordDesktopFailure(operation, err, true, "cloud", "cloud.request.failure", "cloud.request")
}

func recordDesktopBackgroundFailure(operation string, err error) {
	_ = recordDesktopFailure(operation, err, false, "go", "desktop.background.failure", "desktop.background")
}

func recordDesktopFailure(operation string, err error, userVisible bool, source, eventName, scope string) string {
	if err == nil {
		return ""
	}
	event := diagnostics.Event{
		Severity:    "error",
		Source:      source,
		Event:       eventName,
		Scope:       scope,
		Operation:   operation,
		Cause:       err,
		UserVisible: userVisible,
	}
	var typed *cloud.Error
	if errors.As(err, &typed) {
		event.Code = string(typed.Code)
		event.RequestID = typed.RequestID
		event.HTTPStatus = typed.HTTPStatus
		event.Retryable = typed.Retryable
		event.ErrorType = "CloudError"
	} else if errors.Is(err, cloud.ErrCaptchaVerificationRequired) || errors.Is(err, cloud.ErrCaptchaPermitExpired) {
		event.ErrorType = "CaptchaError"
	}
	if journal := currentDesktopErrorJournal(); journal != nil {
		return journal.RecordWithID(event)
	}
	return ""
}

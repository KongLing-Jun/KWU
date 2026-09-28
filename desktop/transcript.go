package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"kwu/desktop/internal/transcript"
	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/eventwire"
	"kwu/internal/userctx"
)

func transcriptStoreForActor(actor userctx.ActorContext) transcript.Store {
	dir := desktopConfigDirForPaths(config.NewProfilePathResolver(actor))
	return transcript.Store{Path: filepath.Join(dir, "desktop-transcripts.sqlite")}
}

func beginControllerTranscript(ctrl control.SessionAPI) error {
	path := sessionRuntimeKey(ctrl.SessionPath())
	if path == "" {
		return nil
	}
	baseline := historyMessagesForSession(controllerSessionDir(ctrl), ctrl.SessionPath(), ctrl.History())
	if baseline == nil {
		baseline = []HistoryMessage{}
	}
	data, err := json.Marshal(baseline)
	if err != nil {
		return err
	}
	return transcriptStoreForActor(ctrl.ActorContext()).Begin(path, event.NewEventID(), data)
}

// Persist the accepted display prompt before the model starts, including turns
// launched outside the foreground composer (automations and background work).
func (a *App) recordAcceptedTranscript(ctrl control.SessionAPI, raw, display string) {
	if ctrl == nil {
		return
	}
	if pinner, ok := ctrl.(interface{ EnsureSessionPath() }); ok {
		pinner.EnsureSessionPath()
	}
	if strings.TrimSpace(ctrl.SessionPath()) == "" {
		return
	}
	if err := beginControllerTranscript(ctrl); err != nil {
		recordDesktopBackgroundFailure("transcript.begin", err)
		return
	}
	a.mu.RLock()
	var sink *tabEventSink
	for _, tab := range a.runtimeTabsLocked() {
		if tab != nil && tab.Ctrl == ctrl {
			sink = tab.sink
			break
		}
	}
	a.mu.RUnlock()
	if strings.TrimSpace(display) == "" {
		display = raw
	}
	wire := wireEventTab{
		Event:                   eventwire.Event{Kind: "notice", EventID: event.NewEventID()},
		TranscriptUser:          &HistoryMessage{Role: "user", Content: display, SubmitText: raw, CreatedAt: time.Now().UnixMilli()},
		ConversationSessionPath: sessionRuntimeKey(ctrl.SessionPath()),
	}
	// Pin the same route used by Emit; detaching a running task must not move
	// its prompt or output to whichever task became visible in the meantime.
	if sink != nil {
		wire.TabID = sink.beginEmitRoute()
		defer sink.endEmitRoute()
	}
	// An accepted background turn can precede runtime attachment. Its prompt
	// must still be recoverable when the frontend later opens the session.
	wire = persistControllerTranscriptEvent(ctrl, wire)
	if sink != nil {
		sink.emitRuntimeEvent(eventChannel, wire)
	}
}

func (s *tabEventSink) persistTranscriptEvent(wire wireEventTab) wireEventTab {
	tab, path := s.telemetryTab()
	if tab == nil || path == "" || tab.Ctrl == nil {
		return wire
	}
	if wire.ConversationSessionPath == "" {
		wire.ConversationSessionPath = path
	}
	return persistControllerTranscriptEvent(tab.Ctrl, wire)
}

func persistControllerTranscriptEvent(ctrl control.SessionAPI, wire wireEventTab) wireEventTab {
	path := sessionRuntimeKey(wire.ConversationSessionPath)
	wire.ConversationSessionPath = path
	store := transcriptStoreForActor(ctrl.ActorContext())
	data, err := json.Marshal(wire)
	if err != nil {
		recordDesktopBackgroundFailure("transcript.encode", err)
		return wire
	}
	id, seq, err := store.Append(path, wire.EventID, data)
	if errors.Is(err, sql.ErrNoRows) && wire.Kind == "turn_started" {
		// Synthetic continuation turns do not invoke the user-turn observer.
		if err = beginControllerTranscript(ctrl); err == nil {
			id, seq, err = store.Append(path, wire.EventID, data)
		}
	}
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			recordDesktopBackgroundFailure("transcript.persist", err)
		}
		return wire
	}
	wire.TranscriptJournalID, wire.TranscriptSequence = id, seq
	return wire
}

// TranscriptPageForTab recovers the durable UI stream using a session identity,
// never the mutable foreground tab. Pages share a fixed database watermark.
func (a *App) TranscriptPageForTab(tabID, journalID string, after, through int64) (*transcript.Page, error) {
	if after < 0 || through < 0 || (after > 0 && journalID == "") {
		return nil, fmt.Errorf("invalid transcript cursor")
	}
	tab, ctrl := a.tabAndCtrlByID(tabID)
	if tab == nil {
		return nil, fmt.Errorf("tab not found")
	}
	path := tab.SessionPath
	if ctrl != nil {
		path = ctrl.SessionPath()
	}
	actor := tabRuntimeActorContext(tab)
	if visible, _, _ := a.projectSessionListVisible(a.bootContext(), actor, path); !visible {
		return nil, fmt.Errorf("session transcript is unavailable in the current scope")
	}
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	return transcriptStoreForActor(actor).Read(sessionRuntimeKey(path), journalID, after, through)
}

func resetControllerTranscript(ctrl control.SessionAPI) error {
	if ctrl == nil || ctrl.SessionPath() == "" {
		return nil
	}
	return transcriptStoreForActor(ctrl.ActorContext()).Reset(sessionRuntimeKey(ctrl.SessionPath()))
}

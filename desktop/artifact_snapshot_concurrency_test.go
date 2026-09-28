package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/userctx"
)

type turnAdmissionRunner struct {
	started chan struct{}
}

func (r turnAdmissionRunner) Run(context.Context, string) error {
	close(r.started)
	return nil
}

type serialArtifactSnapshotController struct {
	control.SessionAPI
	path          string
	enteredFirst  chan struct{}
	enteredSecond chan struct{}
	releaseFirst  chan struct{}
	calls         atomic.Int32
	active        atomic.Int32
	maxActive     atomic.Int32
	runtimeStatus control.RuntimeStatus
}

func (c *serialArtifactSnapshotController) SessionPath() string { return c.path }

func (c *serialArtifactSnapshotController) ActorContext() userctx.ActorContext {
	return userctx.ActorContext{}
}

func (c *serialArtifactSnapshotController) RuntimeStatus() control.RuntimeStatus {
	return c.runtimeStatus
}

func (c *serialArtifactSnapshotController) Snapshot() error {
	active := c.active.Add(1)
	for {
		current := c.maxActive.Load()
		if active <= current || c.maxActive.CompareAndSwap(current, active) {
			break
		}
	}
	defer c.active.Add(-1)

	switch c.calls.Add(1) {
	case 1:
		if c.enteredFirst != nil {
			close(c.enteredFirst)
		}
		if c.releaseFirst != nil {
			<-c.releaseFirst
		}
	case 2:
		if c.enteredSecond != nil {
			close(c.enteredSecond)
		}
	}
	return nil
}

func TestSnapshotTabSerializesWithBackgroundArtifactAutosave(t *testing.T) {
	workspaceRoot := t.TempDir()
	sessionPath := filepath.Join(t.TempDir(), "serial-snapshot.jsonl")
	first := filepath.Join(workspaceRoot, "first.md")
	second := filepath.Join(workspaceRoot, "second.md")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte(filepath.Base(path)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctrl := &serialArtifactSnapshotController{
		enteredFirst:  make(chan struct{}),
		enteredSecond: make(chan struct{}),
		releaseFirst:  make(chan struct{}),
	}
	tab := &WorkspaceTab{ID: "serial-artifact-snapshot", Ctrl: ctrl}
	app := &App{projectTreeChangedHook: func() {}}
	tab.enqueueArtifactDelivery(artifactDeliveryBatch{
		sessionPath: sessionPath, workspaceRoot: workspaceRoot, candidates: []string{first}, deliveredAt: 1,
	})

	app.scheduleTabSnapshotForTab(tab)
	select {
	case <-ctrl.enteredFirst:
	case <-time.After(5 * time.Second):
		t.Fatal("background snapshot did not start")
	}
	tab.enqueueArtifactDelivery(artifactDeliveryBatch{
		sessionPath: sessionPath, workspaceRoot: workspaceRoot, candidates: []string{second}, deliveredAt: 2,
	})
	done := make(chan error, 1)
	go func() { done <- app.snapshotTab(tab) }()
	select {
	case <-ctrl.enteredSecond:
		t.Fatal("synchronous snapshot entered while background snapshot was active")
	case <-time.After(100 * time.Millisecond):
	}
	close(ctrl.releaseFirst)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("synchronous snapshot: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("synchronous snapshot did not finish")
	}
	waitForAutosaveIdle(t, tab)
	if got := ctrl.maxActive.Load(); got != 1 {
		t.Fatalf("maximum concurrent Snapshot calls = %d, want 1", got)
	}

	raw, err := os.ReadFile(artifactSidecarPath(sessionPath))
	if err != nil {
		t.Fatal(err)
	}
	var sidecar artifactSidecar
	if err := json.Unmarshal(raw, &sidecar); err != nil {
		t.Fatal(err)
	}
	if len(sidecar.Artifacts) != 2 || sidecar.Artifacts[0].Path != "first.md" || sidecar.Artifacts[1].Path != "second.md" {
		t.Fatalf("serialized artifacts = %+v", sidecar.Artifacts)
	}
}

func TestTransferArtifactDeliveriesMovesPendingBatchesInOrder(t *testing.T) {
	first := artifactDeliveryBatch{sessionPath: "first.jsonl", candidates: []string{"first.md"}, deliveredAt: 1}
	second := artifactDeliveryBatch{sessionPath: "second.jsonl", candidates: []string{"second.md"}, deliveredAt: 2}
	source := &WorkspaceTab{ID: "source"}
	target := &WorkspaceTab{ID: "target"}
	source.enqueueArtifactDelivery(first)
	target.enqueueArtifactDelivery(second)

	source.transferArtifactDeliveriesTo(target)
	if got := source.takeArtifactDeliveries(); len(got) != 0 {
		t.Fatalf("source deliveries after transfer = %+v", got)
	}
	got := target.takeArtifactDeliveries()
	if len(got) != 2 || got[0].sessionPath != first.sessionPath || got[1].sessionPath != second.sessionPath {
		t.Fatalf("transferred deliveries = %+v", got)
	}
}

func TestSnapshotTabDefersWhenAutosaveIsClosing(t *testing.T) {
	ctrl := &serialArtifactSnapshotController{}
	tab := &WorkspaceTab{ID: "closing-snapshot", Ctrl: ctrl, closing: true}
	app := &App{}

	if err := app.snapshotTab(tab); err != nil {
		t.Fatalf("snapshot while closing: %v", err)
	}
	if got := ctrl.calls.Load(); got != 0 {
		t.Fatalf("Snapshot calls while closing = %d, want 0", got)
	}
	tab.saveMu.Lock()
	deferred := tab.saveAgain
	tab.saveMu.Unlock()
	if !deferred {
		t.Fatal("snapshot request was not retained for hand-off")
	}
}

func TestSnapshotTabPreservesDeferredRequestWhenQuiesced(t *testing.T) {
	ctrl := &serialArtifactSnapshotController{
		enteredFirst: make(chan struct{}),
		releaseFirst: make(chan struct{}),
	}
	tab := &WorkspaceTab{ID: "quiesced-snapshot", Ctrl: ctrl}
	app := &App{}
	done := make(chan error, 1)
	go func() { done <- app.snapshotTab(tab) }()
	select {
	case <-ctrl.enteredFirst:
	case <-time.After(5 * time.Second):
		t.Fatal("synchronous snapshot did not start")
	}
	quiesced := make(chan struct{})
	go func() {
		app.quiesceTabAutosave(tab)
		close(quiesced)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		tab.saveMu.Lock()
		closing := tab.closing
		tab.saveMu.Unlock()
		if closing {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("autosave did not enter closing state")
		}
		time.Sleep(time.Millisecond)
	}
	app.scheduleTabSnapshotForTab(tab)
	close(ctrl.releaseFirst)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("synchronous snapshot: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("synchronous snapshot did not finish")
	}
	select {
	case <-quiesced:
	case <-time.After(5 * time.Second):
		t.Fatal("quiesce did not finish")
	}
	if !consumeClosedTabSnapshotRequest(tab) {
		t.Fatal("snapshot request was cleared while closing")
	}
}

func TestSnapshotTabDrainsTurnDoneSaveBeforeReturning(t *testing.T) {
	workspaceRoot := t.TempDir()
	sessionPath := filepath.Join(t.TempDir(), "sync-drain.jsonl")
	artifactPath := filepath.Join(workspaceRoot, "sync-drain.md")
	if err := os.WriteFile(artifactPath, []byte("sync drain"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctrl := &serialArtifactSnapshotController{
		path:         sessionPath,
		enteredFirst: make(chan struct{}),
		releaseFirst: make(chan struct{}),
	}
	tab := &WorkspaceTab{ID: "sync-drain", Ctrl: ctrl}
	var completionEffects atomic.Int32
	app := &App{projectTreeChangedHook: func() { completionEffects.Add(1) }}
	done := make(chan error, 1)
	go func() { done <- app.snapshotTab(tab) }()
	select {
	case <-ctrl.enteredFirst:
	case <-time.After(5 * time.Second):
		t.Fatal("synchronous snapshot did not start")
	}
	tab.enqueueArtifactDelivery(artifactDeliveryBatch{
		sessionPath: sessionPath, workspaceRoot: workspaceRoot, candidates: []string{artifactPath}, deliveredAt: 1,
	})
	app.scheduleTabSnapshotForTab(tab)
	close(ctrl.releaseFirst)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("synchronous snapshot: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("synchronous snapshot did not drain the deferred save")
	}
	if got := ctrl.calls.Load(); got != 2 {
		t.Fatalf("Snapshot calls before return = %d, want 2", got)
	}
	if got := completionEffects.Load(); got != 1 {
		t.Fatalf("completion effects = %d, want 1", got)
	}
	raw, err := os.ReadFile(artifactSidecarPath(sessionPath))
	if err != nil {
		t.Fatal(err)
	}
	if !containsArtifactPath(raw, "sync-drain.md") {
		t.Fatalf("artifact sidecar before return = %s", raw)
	}
}

func TestDetachRuntimeForReplacementCannotOverwriteDetachedRuntime(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
	ctrl := &serialArtifactSnapshotController{path: sessionPath}
	tab := &WorkspaceTab{ID: "visible", Ctrl: ctrl, SessionPath: sessionPath}
	app := &App{tabRuntimes: map[string]*WorkspaceTab{tab.ID: tab}}
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}

	if !app.detachRuntimeForReplacement(tab) {
		t.Fatal("first detach failed")
	}
	key := sessionRuntimeKey(sessionPath)
	first := app.detachedSessions[key]
	if first == nil || first.Ctrl != ctrl {
		t.Fatalf("first detached runtime = %+v", first)
	}
	if app.detachRuntimeForReplacement(tab) {
		t.Fatal("second detach unexpectedly succeeded")
	}
	if got := app.detachedSessions[key]; got != first {
		t.Fatalf("detached runtime was overwritten: got %p, want %p", got, first)
	}
	tab.saveMu.Lock()
	closing := tab.closing
	tab.saveMu.Unlock()
	if closing {
		t.Fatal("reusable visible tab remained in closing state")
	}
}

func TestDetachRuntimeForReplacementRejectsOccupiedSessionSlot(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "session.jsonl")
	existingCtrl := &serialArtifactSnapshotController{path: sessionPath}
	existing := &WorkspaceTab{ID: "detached-existing", Ctrl: existingCtrl, SessionPath: sessionPath}
	visibleCtrl := &serialArtifactSnapshotController{path: sessionPath}
	visible := &WorkspaceTab{ID: "visible-conflict", Ctrl: visibleCtrl, SessionPath: sessionPath}
	app := &App{
		tabRuntimes:      map[string]*WorkspaceTab{visible.ID: visible},
		detachedSessions: map[string]*WorkspaceTab{sessionRuntimeKey(sessionPath): existing},
	}
	visible.sink = &tabEventSink{tabID: visible.ID, app: app}

	if app.detachRuntimeForReplacement(visible) {
		t.Fatal("detach unexpectedly replaced an occupied session slot")
	}
	if got := app.detachedSessions[sessionRuntimeKey(sessionPath)]; got != existing {
		t.Fatalf("occupied detached runtime was overwritten: got %p, want %p", got, existing)
	}
	if visible.Ctrl != visibleCtrl {
		t.Fatal("visible runtime changed after rejected detach")
	}
	visible.saveMu.Lock()
	closing := visible.closing
	visible.saveMu.Unlock()
	if closing {
		t.Fatal("visible runtime remained closing after rejected detach")
	}
}

func TestCloseTabAtomicallyDetachesCompletingRuntime(t *testing.T) {
	isolateDesktopUserDirs(t)
	sessionPath := filepath.Join(t.TempDir(), "completing.jsonl")
	ctrl := &serialArtifactSnapshotController{
		path:          sessionPath,
		runtimeStatus: control.RuntimeStatus{Completing: true},
	}
	tab := &WorkspaceTab{ID: "completing", Ctrl: ctrl, SessionPath: sessionPath, Ready: true}
	other := &WorkspaceTab{ID: "other", Ready: true}
	app := &App{
		tabRuntimes: map[string]*WorkspaceTab{tab.ID: tab, other.ID: other},
		tabOrder:    []string{tab.ID, other.ID},
		activeTabID: other.ID,
	}
	app.publishActiveTabID(other.ID)
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}

	if err := app.CloseTab(tab.ID); err != nil {
		t.Fatalf("CloseTab: %v", err)
	}
	key := sessionRuntimeKey(sessionPath)
	if got := app.detachedSessions[key]; got != tab {
		t.Fatalf("detached runtime = %p, want %p", got, tab)
	}
	app.mu.RLock()
	routed := app.tabByEventSinkIDLocked(tab.sink.currentTabID())
	app.mu.RUnlock()
	if routed != tab {
		t.Fatalf("terminal event route = %p, want detached runtime %p", routed, tab)
	}
}

func TestDesktopTurnAdmissionRejectsRemovedRuntime(t *testing.T) {
	started := make(chan struct{})
	ctrl := control.New(control.Options{Runner: turnAdmissionRunner{started: started}, Sink: event.Discard})
	t.Cleanup(ctrl.Close)
	tab := &WorkspaceTab{ID: "admission", Ctrl: ctrl, Ready: true}
	app := &App{tabRuntimes: map[string]*WorkspaceTab{tab.ID: tab}}
	app.bindControllerDisplayRecorder(ctrl)

	app.mu.Lock()
	delete(app.tabRuntimes, tab.ID)
	app.mu.Unlock()
	ctrl.Send("must not start")
	select {
	case <-started:
		t.Fatal("removed desktop runtime admitted a new turn")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestDesktopTurnAdmissionRejectsReroutedRuntime(t *testing.T) {
	started := make(chan struct{})
	ctrl := control.New(control.Options{Runner: turnAdmissionRunner{started: started}, Sink: event.Discard})
	t.Cleanup(ctrl.Close)
	sink := &tabEventSink{tabID: "visible"}
	tab := &WorkspaceTab{ID: "visible", Ctrl: ctrl, sink: sink, Ready: true}
	app := &App{tabRuntimes: map[string]*WorkspaceTab{tab.ID: tab}}
	app.bindControllerDisplayRecorder(ctrl)

	if !sink.beginRouteChangeWithin(time.Second) {
		t.Fatal("could not acquire route change gate")
	}
	app.mu.Lock()
	sink.setTabID("detached")
	app.mu.Unlock()
	sink.endRouteChange()

	ctrl.Send("must not start")
	select {
	case <-started:
		t.Fatal("rerouted desktop runtime admitted a new turn")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestTabEventSinkRouteGateExcludesEmitAndRouteChange(t *testing.T) {
	sink := &tabEventSink{tabID: "source"}
	if got := sink.beginEmitRoute(); got != "source" {
		t.Fatalf("first emit route = %q, want source", got)
	}
	if sink.tryBeginRouteChange() {
		sink.endRouteChange()
		t.Fatal("route change acquired while an Emit reader was active")
	}
	if sink.beginRouteChangeWithin(0) {
		sink.endRouteChange()
		t.Fatal("zero-timeout route change acquired while an Emit reader was active")
	}
	if got := sink.routeGate.Load(); got != 1 {
		t.Fatalf("route gate after timed-out writer = %#x, want one preserved reader", got)
	}

	changeAcquired := make(chan bool, 1)
	releaseChange := make(chan struct{})
	go func() {
		ok := sink.beginRouteChangeWithin(5 * time.Second)
		changeAcquired <- ok
		if !ok {
			return
		}
		<-releaseChange
		sink.setTabID("target")
		sink.endRouteChange()
	}()
	deadline := time.Now().Add(time.Second)
	for sink.routeGate.Load()&tabEventRouteSwitching == 0 {
		if time.Now().After(deadline) {
			t.Fatal("route writer did not publish the switching bit")
		}
		time.Sleep(time.Millisecond)
	}

	lateEmit := make(chan string, 1)
	go func() {
		route := sink.beginEmitRoute()
		lateEmit <- route
		sink.endEmitRoute()
	}()
	select {
	case route := <-lateEmit:
		t.Fatalf("late Emit crossed switching gate on route %q", route)
	default:
	}

	sink.endEmitRoute()
	if ok := <-changeAcquired; !ok {
		t.Fatal("route writer timed out after the active Emit drained")
	}
	select {
	case route := <-lateEmit:
		t.Fatalf("late Emit crossed an acquired route writer on route %q", route)
	default:
	}
	close(releaseChange)
	select {
	case route := <-lateEmit:
		if route != "target" {
			t.Fatalf("late emit route = %q, want target", route)
		}
	case <-time.After(time.Second):
		t.Fatal("late Emit did not resume after the route writer released")
	}
}

func TestTabEventSinkEmitUsesOneRouteSnapshot(t *testing.T) {
	isolateDesktopUserDirs(t)
	sessionDir := t.TempDir()
	sessionPath := filepath.Join(sessionDir, "route-gate.jsonl")
	ctrl := control.New(control.Options{
		SessionDir:  sessionDir,
		SessionPath: sessionPath,
		Label:       "route-gate",
		Sink:        event.Discard,
	})
	t.Cleanup(ctrl.Close)

	app := &App{
		ctx:         context.Background(),
		tabRuntimes: map[string]*WorkspaceTab{},
		tabOrder:    []string{"source", "target"},
	}
	source := &WorkspaceTab{
		ID: "source", Scope: "project", WorkspaceRoot: sessionDir,
		TopicID: "source-topic", SessionPath: sessionPath, Ctrl: ctrl, Ready: true,
	}
	target := &WorkspaceTab{
		ID: "target", Scope: "project", WorkspaceRoot: sessionDir,
		TopicID: "target-topic", SessionPath: sessionPath, Ready: true,
	}
	sink := &tabEventSink{tabID: source.ID, app: app, ctx: context.Background()}
	source.sink = sink
	target.sink = &tabEventSink{tabID: target.ID, app: app, ctx: context.Background()}
	app.tabRuntimes[source.ID] = source
	app.tabRuntimes[target.ID] = target

	delivered := make(chan wireEventTab, 2)
	app.runtimeEvents.emit = func(_ context.Context, name string, payload ...interface{}) {
		if name != conversationActivityEventChannel {
			return
		}
		wire, ok := payload[0].(wireEventTab)
		if !ok {
			t.Errorf("payload type = %T, want wireEventTab", payload[0])
			return
		}
		delivered <- wire
	}

	app.mu.Lock()
	firstDone := make(chan struct{})
	go func() {
		sink.Emit(event.Event{Kind: event.TurnDone, Interrupted: true})
		close(firstDone)
	}()
	deadline := time.Now().Add(time.Second)
	for sink.routeGate.Load()&^tabEventRouteSwitching == 0 {
		if time.Now().After(deadline) {
			app.mu.Unlock()
			t.Fatal("TurnDone did not acquire an Emit route reader")
		}
		time.Sleep(time.Millisecond)
	}

	changeDone := make(chan bool, 1)
	go func() {
		if !sink.beginRouteChangeWithin(5 * time.Second) {
			changeDone <- false
			return
		}
		app.mu.Lock()
		delete(app.tabRuntimes, source.ID)
		applyRuntimeTab(target, source, sessionRuntimeKey(sessionPath), context.Background(), app)
		app.mu.Unlock()
		sink.endRouteChange()
		changeDone <- true
	}()
	deadline = time.Now().Add(time.Second)
	for sink.routeGate.Load()&tabEventRouteSwitching == 0 {
		if time.Now().After(deadline) {
			app.mu.Unlock()
			t.Fatal("route replacement did not publish the switching bit")
		}
		time.Sleep(time.Millisecond)
	}
	select {
	case <-changeDone:
		app.mu.Unlock()
		t.Fatal("route replacement completed before the in-flight TurnDone")
	default:
	}

	app.mu.Unlock()
	select {
	case <-firstDone:
	case <-time.After(5 * time.Second):
		t.Fatal("TurnDone did not finish after releasing App.mu")
	}
	select {
	case ok := <-changeDone:
		if !ok {
			t.Fatal("route replacement timed out after TurnDone completed")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("route replacement did not finish")
	}

	sink.Emit(event.Event{Kind: event.TurnStarted})
	var first, second wireEventTab
	select {
	case first = <-delivered:
	case <-time.After(5 * time.Second):
		t.Fatal("source TurnDone conversation event was not delivered")
	}
	select {
	case second = <-delivered:
	case <-time.After(5 * time.Second):
		t.Fatal("target TurnStarted conversation event was not delivered")
	}
	if first.TabID != source.ID || first.ConversationTopicID != source.TopicID || first.ConversationSessionPath != sessionRuntimeKey(sessionPath) {
		t.Fatalf("first TurnDone route = %+v, want source route", first)
	}
	if second.TabID != target.ID || second.ConversationTopicID != target.TopicID || second.ConversationSessionPath != sessionRuntimeKey(sessionPath) {
		t.Fatalf("second TurnStarted route = %+v, want target route", second)
	}
}

func TestEventSinkSnapshotClaimBlocksRuntimeMove(t *testing.T) {
	sourceCtrl := &serialArtifactSnapshotController{}
	targetCtrl := &serialArtifactSnapshotController{}
	source := &WorkspaceTab{ID: "source", Ctrl: sourceCtrl}
	target := &WorkspaceTab{ID: "target", Ctrl: targetCtrl}
	app := &App{tabRuntimes: map[string]*WorkspaceTab{source.ID: source}}
	sink := &tabEventSink{tabID: source.ID, app: app}
	source.sink = sink
	target.sink = sink

	source.saveMu.Lock()
	scheduled := make(chan struct{})
	go func() {
		sink.scheduleSnapshot()
		close(scheduled)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for app.mu.TryLock() {
		app.mu.Unlock()
		if time.Now().After(deadline) {
			source.saveMu.Unlock()
			t.Fatal("event sink did not retain the App read lock while claiming autosave")
		}
		time.Sleep(time.Millisecond)
	}
	moved := make(chan struct{})
	go func() {
		app.mu.Lock()
		delete(app.tabRuntimes, source.ID)
		app.tabRuntimes[target.ID] = target
		sink.setTabID(target.ID)
		app.mu.Unlock()
		close(moved)
	}()
	select {
	case <-moved:
		source.saveMu.Unlock()
		t.Fatal("runtime move passed the route barrier before autosave was claimed")
	case <-time.After(50 * time.Millisecond):
	}
	source.saveMu.Unlock()

	select {
	case <-scheduled:
	case <-time.After(5 * time.Second):
		t.Fatal("snapshot scheduling did not finish")
	}
	select {
	case <-moved:
	case <-time.After(5 * time.Second):
		t.Fatal("runtime move did not finish after autosave claim")
	}
	waitForAutosaveIdle(t, source)
	if got := sourceCtrl.calls.Load(); got != 1 {
		t.Fatalf("source Snapshot calls = %d, want 1", got)
	}
	if got := targetCtrl.calls.Load(); got != 0 {
		t.Fatalf("target Snapshot calls = %d, want 0", got)
	}
}

func TestDetachTransfersDeliveryQueuedWhileQuiescing(t *testing.T) {
	workspaceRoot := t.TempDir()
	sessionPath := filepath.Join(t.TempDir(), "handoff.jsonl")
	artifactPath := filepath.Join(workspaceRoot, "handoff.md")
	if err := os.WriteFile(artifactPath, []byte("handoff"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctrl := &serialArtifactSnapshotController{path: sessionPath}
	tab := &WorkspaceTab{ID: "visible", Ctrl: ctrl, SessionPath: sessionPath, WorkspaceRoot: workspaceRoot}
	app := &App{tabRuntimes: map[string]*WorkspaceTab{tab.ID: tab}, projectTreeChangedHook: func() {}}
	tab.sink = &tabEventSink{tabID: tab.ID, app: app}

	tab.saveMu.Lock()
	tab.saving = true
	tab.saveMu.Unlock()
	detachedDone := make(chan bool, 1)
	go func() { detachedDone <- app.detachRuntimeForReplacement(tab) }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		tab.saveMu.Lock()
		closing := tab.closing
		tab.saveMu.Unlock()
		if closing {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("detach did not start quiescing")
		}
		time.Sleep(time.Millisecond)
	}
	app.mu.RLock()
	tab.enqueueArtifactDelivery(artifactDeliveryBatch{
		sessionPath: sessionPath, workspaceRoot: workspaceRoot, candidates: []string{artifactPath}, deliveredAt: 1,
	})
	app.mu.RUnlock()
	tab.saveMu.Lock()
	tab.saving = false
	if tab.saveCond != nil {
		tab.saveCond.Broadcast()
	}
	tab.saveMu.Unlock()
	select {
	case ok := <-detachedDone:
		if !ok {
			t.Fatal("detach failed")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("detach did not finish")
	}
	detached := app.detachedSessions[sessionRuntimeKey(sessionPath)]
	if detached == nil {
		t.Fatal("detached runtime missing")
	}
	waitForAutosaveIdle(t, detached)
	raw, err := os.ReadFile(artifactSidecarPath(sessionPath))
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(raw) || !containsArtifactPath(raw, "handoff.md") {
		t.Fatalf("transferred artifact sidecar = %s", raw)
	}
}

func TestAttachExistingRuntimeWaitsForBusyHandoff(t *testing.T) {
	isolateDesktopUserDirs(t)
	sessionDir := t.TempDir()
	sessionPath := filepath.Join(sessionDir, "busy-handoff.jsonl")
	ctrl := control.New(control.Options{SessionDir: sessionDir, SessionPath: sessionPath, Label: "busy", Sink: event.Discard})
	t.Cleanup(ctrl.Close)
	source := &WorkspaceTab{ID: "source-busy", Ctrl: ctrl, SessionPath: sessionPath, closing: true}
	target := &WorkspaceTab{ID: "target-busy", SessionPath: sessionPath}
	app := &App{tabRuntimes: map[string]*WorkspaceTab{source.ID: source, target.ID: target}}
	source.sink = &tabEventSink{tabID: source.ID, app: app}
	target.sink = &tabEventSink{tabID: target.ID, app: app}

	reopened := make(chan struct{})
	go func() {
		time.Sleep(20 * time.Millisecond)
		reopenTabAutosave(source)
		close(reopened)
	}()
	if !app.attachExistingSessionRuntime(target, sessionPath, nil) {
		t.Fatal("busy runtime was not attached after its handoff reopened")
	}
	<-reopened
	if target.Ctrl != ctrl {
		t.Fatal("target did not receive the existing controller")
	}
	if app.tabRuntimes[source.ID] != nil {
		t.Fatal("source runtime remained visible after attach")
	}
}

func TestPermanentCloseWaitsForClaimedArtifactWriterBeforeClearingPath(t *testing.T) {
	workspaceRoot := t.TempDir()
	sessionDir := t.TempDir()
	sessionPath := filepath.Join(sessionDir, "closing.jsonl")
	artifactPath := filepath.Join(workspaceRoot, "closing.md")
	if err := os.WriteFile(artifactPath, []byte("closing"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctrl := control.New(control.Options{
		SessionDir: sessionDir, SessionPath: sessionPath, WorkspaceRoot: workspaceRoot, Label: "closing", Sink: event.Discard,
	})
	tab := &WorkspaceTab{ID: "closing", Ctrl: ctrl, SessionPath: sessionPath, WorkspaceRoot: workspaceRoot}
	app := &App{tabRuntimes: map[string]*WorkspaceTab{tab.ID: tab}, projectTreeChangedHook: func() {}}
	tab.enqueueArtifactDelivery(artifactDeliveryBatch{
		sessionPath: sessionPath, workspaceRoot: workspaceRoot, candidates: []string{artifactPath}, deliveredAt: 1,
	})
	tab.saveMu.Lock()
	tab.saving = true
	tab.saveMu.Unlock()

	app.mu.Lock()
	go app.tabSnapshotLoop(tab)
	closed := make(chan struct{})
	go func() {
		app.closeTabRuntime(tab)
		close(closed)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		tab.saveMu.Lock()
		closing := tab.closing
		tab.saveMu.Unlock()
		if closing {
			break
		}
		if time.Now().After(deadline) {
			app.mu.Unlock()
			t.Fatal("close did not quiesce the claimed writer")
		}
		time.Sleep(time.Millisecond)
	}
	app.mu.Unlock()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("close did not finish")
	}
	raw, err := os.ReadFile(artifactSidecarPath(sessionPath))
	if err != nil {
		t.Fatal(err)
	}
	if !containsArtifactPath(raw, "closing.md") {
		t.Fatalf("artifact sidecar after close = %s", raw)
	}
}

func containsArtifactPath(raw []byte, path string) bool {
	var sidecar artifactSidecar
	if json.Unmarshal(raw, &sidecar) != nil {
		return false
	}
	for _, artifact := range sidecar.Artifacts {
		if artifact.Path == path {
			return true
		}
	}
	return false
}

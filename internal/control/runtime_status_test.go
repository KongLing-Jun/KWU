package control

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"kwu/internal/event"
)

type approvalBlockingRunner struct {
	c *Controller
}

func (r *approvalBlockingRunner) Run(ctx context.Context, _ string) error {
	_, _, err := gateApprover{c: r.c}.Approve(ctx, "bash", "go test ./...", nil)
	return err
}

type askBlockingRunner struct {
	c *Controller
}

func (r *askBlockingRunner) Run(ctx context.Context, _ string) error {
	_, err := r.c.Ask(ctx, []event.AskQuestion{{
		ID:      "choice",
		Prompt:  "Pick one",
		Options: []event.AskOption{{Label: "A"}, {Label: "B"}},
	}})
	return err
}

func TestCancelClearsPendingApprovalRuntimeStatus(t *testing.T) {
	approvals := make(chan event.Approval, 1)
	done := make(chan event.Event, 1)
	c := New(Options{Sink: event.FuncSink(func(e event.Event) {
		switch e.Kind {
		case event.ApprovalRequest:
			approvals <- e.Approval
		case event.TurnDone:
			done <- e
		}
	})})
	runner := &approvalBlockingRunner{c: c}
	c.runner = runner

	c.Send("needs approval")
	select {
	case <-approvals:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for approval request")
	}
	if st := c.RuntimeStatus(); !st.Running || !st.PendingPrompt || !st.Cancellable || st.CancelRequested {
		t.Fatalf("status before cancel = %+v, want running pending cancellable", st)
	}

	c.Cancel()
	c.Cancel()
	assertCancelClearedPendingRuntimeStatus(t, c.RuntimeStatus())
	waitTurnDoneEvent(t, done)
	waitControllerCompletionIdle(t, c)
	if st := c.RuntimeStatus(); st.Running || st.PendingPrompt || st.Cancellable || st.CancelRequested {
		t.Fatalf("status after turn done = %+v, want idle", st)
	}
}

func TestCancelClearsPendingAskRuntimeStatus(t *testing.T) {
	asks := make(chan event.Ask, 1)
	done := make(chan event.Event, 1)
	c := New(Options{Sink: event.FuncSink(func(e event.Event) {
		switch e.Kind {
		case event.AskRequest:
			asks <- e.Ask
		case event.TurnDone:
			done <- e
		}
	})})
	runner := &askBlockingRunner{c: c}
	c.runner = runner

	c.Send("ask user")
	select {
	case <-asks:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ask request")
	}
	if st := c.RuntimeStatus(); !st.Running || !st.PendingPrompt || !st.Cancellable || st.CancelRequested {
		t.Fatalf("status before cancel = %+v, want running pending cancellable", st)
	}

	c.Cancel()
	assertCancelClearedPendingRuntimeStatus(t, c.RuntimeStatus())
	waitTurnDoneEvent(t, done)
	waitControllerCompletionIdle(t, c)
	if st := c.RuntimeStatus(); st.Running || st.PendingPrompt || st.Cancellable || st.CancelRequested {
		t.Fatalf("status after turn done = %+v, want idle", st)
	}
}

func TestRuntimeMarksAsyncTurnDoneCallbackCompleting(t *testing.T) {
	assertRuntimeCompletingDuringTurnDone(t, func(c *Controller) <-chan error {
		done := make(chan error, 1)
		c.runGuarded(func(context.Context) error {
			done <- nil
			return nil
		})
		return done
	})
}

func TestRuntimeMarksForegroundTurnDoneCallbackCompleting(t *testing.T) {
	assertRuntimeCompletingDuringTurnDone(t, func(c *Controller) <-chan error {
		done := make(chan error, 1)
		go func() {
			done <- c.runForegroundTurnBound(
				context.Background(),
				func(ctx context.Context) (context.Context, error) { return ctx, nil },
				func(context.Context) error { return nil },
			)
		}()
		return done
	})
}

func assertRuntimeCompletingDuringTurnDone(t *testing.T, start func(*Controller) <-chan error) {
	t.Helper()
	callbackEntered := make(chan struct{})
	releaseCallback := make(chan struct{})
	var c *Controller
	c = New(Options{Sink: event.FuncSink(func(e event.Event) {
		if e.Kind != event.TurnDone {
			return
		}
		// A terminal callback must not retain the session lifecycle read lock;
		// consumers are allowed to synchronously rotate the session here.
		c.sessionLifecycleMu.Lock()
		c.sessionLifecycleMu.Unlock()
		close(callbackEntered)
		<-releaseCallback
	})})
	done := start(c)
	select {
	case <-callbackEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for turn_done callback")
	}
	if status := c.RuntimeStatus(); status.Running || !status.Completing || status.Cancellable {
		t.Fatalf("status during turn_done callback = %+v, want non-running completion", status)
	}
	close(releaseCallback)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("turn failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for turn completion")
	}
	waitControllerCompletionIdle(t, c)
}

func TestAsyncTurnDoneCallbackCanStartNextTurn(t *testing.T) {
	secondTurnQueued := make(chan struct{})
	secondTurnStarted := make(chan struct{})
	secondTurnDone := make(chan struct{})
	releaseFirstCallback := make(chan struct{})
	var c *Controller
	var callbackCount atomic.Int32
	c = New(Options{Sink: event.FuncSink(func(e event.Event) {
		if e.Kind != event.TurnDone {
			return
		}
		if callbackCount.Add(1) == 1 {
			c.runGuarded(func(context.Context) error {
				close(secondTurnStarted)
				return nil
			})
			close(secondTurnQueued)
			<-releaseFirstCallback
			return
		}
		close(secondTurnDone)
	})})
	c.runGuarded(func(context.Context) error { return nil })
	select {
	case <-secondTurnQueued:
	case <-time.After(2 * time.Second):
		t.Fatal("turn_done callback did not queue the next turn")
	}
	select {
	case <-secondTurnStarted:
		t.Fatal("queued turn started before the prior turn_done callback returned")
	case <-time.After(50 * time.Millisecond):
	}
	close(releaseFirstCallback)
	select {
	case <-secondTurnStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("turn_done callback could not start the next turn")
	}
	select {
	case <-secondTurnDone:
	case <-time.After(2 * time.Second):
		t.Fatal("second turn did not complete")
	}
	waitControllerCompletionIdle(t, c)
}

func TestForegroundTurnRejectsCompletionCallbackWindow(t *testing.T) {
	callbackEntered := make(chan struct{})
	releaseCallback := make(chan struct{})
	c := New(Options{Sink: event.FuncSink(func(e event.Event) {
		if e.Kind == event.TurnDone {
			close(callbackEntered)
			<-releaseCallback
		}
	})})
	c.runGuarded(func(context.Context) error { return nil })
	select {
	case <-callbackEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for turn_done callback")
	}
	err := c.runForegroundTurnBound(
		context.Background(),
		func(ctx context.Context) (context.Context, error) { return ctx, nil },
		func(context.Context) error { return nil },
	)
	if err != ErrTurnRunning {
		t.Fatalf("foreground turn error = %v, want ErrTurnRunning", err)
	}
	close(releaseCallback)
	waitControllerCompletionIdle(t, c)
}

func TestCancelDropsQueuedTurnDuringCompletionCallback(t *testing.T) {
	queued := make(chan struct{})
	queuedTurnStarted := make(chan struct{})
	releaseCallback := make(chan struct{})
	var c *Controller
	var callbackCount atomic.Int32
	c = New(Options{Sink: event.FuncSink(func(e event.Event) {
		if e.Kind != event.TurnDone || callbackCount.Add(1) != 1 {
			return
		}
		c.runGuarded(func(context.Context) error {
			close(queuedTurnStarted)
			return nil
		})
		close(queued)
		<-releaseCallback
	})})
	c.runGuarded(func(context.Context) error { return nil })
	select {
	case <-queued:
	case <-time.After(2 * time.Second):
		t.Fatal("turn_done callback did not queue the next turn")
	}
	if status := c.RuntimeStatus(); !status.Cancellable {
		t.Fatal("queued turn should be cancellable")
	}
	c.Cancel()
	if status := c.RuntimeStatus(); status.Cancellable {
		t.Fatal("cancelled queued turn should no longer be cancellable")
	}
	close(releaseCallback)
	select {
	case <-queuedTurnStarted:
		t.Fatal("cancelled queued turn started after the completion callback")
	case <-time.After(50 * time.Millisecond):
	}
	waitControllerCompletionIdle(t, c)
}

func TestTurnAdmissionGuardCoversAsyncRunReservation(t *testing.T) {
	released := make(chan bool, 1)
	var c *Controller
	c = New(Options{})
	c.SetTurnAdmissionGuard(func() (func(), bool) {
		return func() { released <- c.Running() }, true
	})
	c.runGuarded(func(context.Context) error { return nil })
	select {
	case running := <-released:
		if !running {
			t.Fatal("turn admission released before run state was reserved")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("turn admission was not released")
	}
	waitControllerCompletionIdle(t, c)
}

func TestTurnAdmissionWaitsForSessionLifecycle(t *testing.T) {
	admissionEntered := make(chan struct{})
	bodyStarted := make(chan struct{})
	var admissionObserved atomic.Bool
	c := New(Options{})
	c.SetTurnAdmissionGuard(func() (func(), bool) {
		if admissionObserved.CompareAndSwap(false, true) {
			close(admissionEntered)
		}
		return func() {}, true
	})

	c.sessionLifecycleMu.Lock()
	unlocked := false
	defer func() {
		if !unlocked {
			c.sessionLifecycleMu.Unlock()
		}
	}()
	go c.runGuarded(func(context.Context) error {
		close(bodyStarted)
		return nil
	})
	select {
	case <-admissionEntered:
		t.Fatal("turn admission entered before the session lifecycle write completed")
	case <-time.After(50 * time.Millisecond):
	}

	c.sessionLifecycleMu.Unlock()
	unlocked = true
	select {
	case <-admissionEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("turn admission did not resume after the session lifecycle write completed")
	}
	select {
	case <-bodyStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("turn did not start after admission")
	}
	waitControllerCompletionIdle(t, c)
}

func TestTurnAdmissionGuardRejectsRemovedRuntime(t *testing.T) {
	bodyStarted := make(chan struct{}, 1)
	c := New(Options{})
	c.SetTurnAdmissionGuard(func() (func(), bool) { return nil, false })
	c.runGuarded(func(context.Context) error {
		bodyStarted <- struct{}{}
		return nil
	})
	select {
	case <-bodyStarted:
		t.Fatal("async turn ran after host rejected admission")
	case <-time.After(50 * time.Millisecond):
	}
	err := c.runForegroundTurnBound(
		context.Background(),
		func(ctx context.Context) (context.Context, error) { return ctx, nil },
		func(context.Context) error { return nil },
	)
	if err != ErrTurnNotAdmitted {
		t.Fatalf("foreground admission error = %v, want ErrTurnNotAdmitted", err)
	}
}

func waitControllerCompletionIdle(t *testing.T, c *Controller) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		status := c.RuntimeStatus()
		if !status.Running && !status.Completing {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for controller completion callback to become idle")
		}
		time.Sleep(time.Millisecond)
	}
}

func assertCancelClearedPendingRuntimeStatus(t *testing.T, st RuntimeStatus) {
	t.Helper()
	if st.PendingPrompt {
		t.Fatalf("status immediately after cancel = %+v, want pending prompt cleared", st)
	}
	if st.Running {
		if !st.Cancellable || !st.CancelRequested {
			t.Fatalf("status immediately after cancel = %+v, want running cancelling without pending prompt", st)
		}
		return
	}
	if st.Cancellable || st.CancelRequested {
		t.Fatalf("status immediately after cancel = %+v, want idle when turn already completed", st)
	}
}

func waitTurnDoneEvent(t *testing.T, done <-chan event.Event) {
	t.Helper()
	select {
	case e := <-done:
		if e.Kind != event.TurnDone {
			t.Fatalf("event = %v, want TurnDone", e.Kind)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for turn_done")
	}
}

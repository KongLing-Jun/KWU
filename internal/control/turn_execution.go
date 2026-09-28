package control

import (
	"context"
	"errors"
	"fmt"

	"kwu/internal/event"
)

// RuntimeStatus is the frontend-facing snapshot of foreground turn state. It is
// intentionally more explicit than the legacy Running bool so UI code can
// distinguish a cancellable foreground turn from pending prompts and background
// jobs.
type RuntimeStatus struct {
	Running         bool
	Completing      bool
	PendingPrompt   bool
	BackgroundJobs  int
	CancelRequested bool
	Cancellable     bool
}

type turnAdmissionGuardHolder struct {
	guard func() (func(), bool)
}

// runGuarded runs body on a background goroutine under a fresh cancellable
// context, guarding against concurrent turns and emitting a TurnDone event when
// it finishes (Err set on failure; nil also for a user Cancel). A no-op if a
// turn is already in flight.
func (c *Controller) runGuarded(body func(ctx context.Context) error) {
	c.sessionLifecycleMu.RLock()
	releaseAdmission, admitted := c.acquireTurnAdmission()
	if !admitted {
		c.sessionLifecycleMu.RUnlock()
		return
	}
	defer releaseAdmission()
	c.mu.Lock()
	if c.running {
		if c.completing > 0 && c.queuedTurn == nil {
			c.queuedTurn = body
		}
		c.mu.Unlock()
		c.sessionLifecycleMu.RUnlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	runtimeTurnID := event.NewTurnID()
	ctx = event.WithTurnID(ctx, runtimeTurnID)
	c.cancel = cancel
	c.running = true
	c.canceling = false
	c.completedTurn = nil
	c.completedArtifactDelivery.Store(nil)
	c.mu.Unlock()

	autosaveDone := c.startAutosaveWhileRunning(ctx)
	c.launchGuardedTurn(body, ctx, cancel, autosaveDone)
}

// launchGuardedTurn starts a turn whose lifecycle read lock and run-state
// reservation are already owned by the caller.
func (c *Controller) launchGuardedTurn(body func(ctx context.Context) error, ctx context.Context, cancel context.CancelFunc, autosaveDone <-chan struct{}) {
	runtimeTurnID := event.TurnIDFromContext(ctx)
	go func() {
		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("internal error: %v", r)
				}
			}()
			err = body(ctx)
		}()
		cancel()
		<-autosaveDone
		turn := c.takeCompletedTurnMetadata()
		delivery := c.takeCompletedArtifactDelivery()
		c.mu.Lock()
		interrupted := c.canceling && errors.Is(err, context.Canceled)
		originSessionPath := c.sessionPath
		originWorkspaceRoot := c.workspaceRoot
		c.completing++
		c.cancel = nil
		c.canceling = false
		c.mu.Unlock()
		c.sessionLifecycleMu.RUnlock()
		defer c.finishCompletionCallback()
		c.sink.Emit(event.Event{
			Kind:                event.TurnDone,
			TurnID:              runtimeTurnID,
			Err:                 explainError(err),
			Interrupted:         interrupted,
			Turn:                turn,
			ArtifactDelivery:    delivery,
			OriginSessionPath:   originSessionPath,
			OriginWorkspaceRoot: originWorkspaceRoot,
		})
	}()
}

// runForegroundTurn provides the synchronous lifecycle shared by ordinary and
// capability-scoped turns. The callback runs after the task context is bound,
// so per-turn tool registries cannot leak into another turn.
func (c *Controller) runForegroundTurn(ctx context.Context, run func(context.Context) error) (err error) {
	return c.runForegroundTurnBound(ctx, c.bindTaskContext, run)
}

func (c *Controller) runForegroundTurnBound(ctx context.Context, bind func(context.Context) (context.Context, error), run func(context.Context) error) (err error) {
	c.sessionLifecycleMu.RLock()
	releaseAdmission, admitted := c.acquireTurnAdmission()
	if !admitted {
		c.sessionLifecycleMu.RUnlock()
		return ErrTurnNotAdmitted
	}
	lifecycleHeld := true
	defer func() {
		if lifecycleHeld {
			c.sessionLifecycleMu.RUnlock()
		}
	}()
	ctx, cancel := context.WithCancel(ctx)
	runtimeTurnID := event.NewTurnID()
	ctx = event.WithTurnID(ctx, runtimeTurnID)
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		cancel()
		releaseAdmission()
		return ErrTurnRunning
	}
	c.cancel = cancel
	c.running = true
	c.canceling = false
	c.completedTurn = nil
	c.completedArtifactDelivery.Store(nil)
	c.mu.Unlock()
	releaseAdmission()
	taskContextBound := false
	var autosaveDone <-chan struct{}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("internal error: %v", r)
		}
		cancel()
		if autosaveDone != nil {
			<-autosaveDone
		}
		turn := c.takeCompletedTurnMetadata()
		delivery := c.takeCompletedArtifactDelivery()
		c.mu.Lock()
		interrupted := c.canceling && errors.Is(err, context.Canceled)
		originSessionPath := c.sessionPath
		originWorkspaceRoot := c.workspaceRoot
		if taskContextBound {
			c.completing++
		} else {
			c.running = false
		}
		c.cancel = nil
		c.canceling = false
		c.mu.Unlock()
		c.sessionLifecycleMu.RUnlock()
		lifecycleHeld = false
		if taskContextBound {
			defer c.finishCompletionCallback()
			c.sink.Emit(event.Event{
				Kind:                event.TurnDone,
				TurnID:              runtimeTurnID,
				Err:                 explainError(err),
				Interrupted:         interrupted,
				Turn:                turn,
				ArtifactDelivery:    delivery,
				OriginSessionPath:   originSessionPath,
				OriginWorkspaceRoot: originWorkspaceRoot,
			})
		}
	}()
	ctx, err = bind(ctx)
	if err != nil {
		return err
	}
	taskContextBound = true
	autosaveDone = c.startAutosaveWhileRunning(ctx)
	return c.runTaskLifecycle(ctx, func(ctx context.Context) error {
		return run(ctx)
	})
}

// RuntimeStatus reports the active work owned by the foreground controller.
func (c *Controller) RuntimeStatus() RuntimeStatus {
	c.mu.Lock()
	running := c.running && c.completing == 0
	completing := c.completing > 0
	queued := c.queuedTurn != nil
	canceling := c.canceling
	c.mu.Unlock()
	pending := c.approval.hasPending()
	backgroundJobs := len(c.Jobs())
	return RuntimeStatus{
		Running:         running,
		Completing:      completing,
		PendingPrompt:   pending,
		BackgroundJobs:  backgroundJobs,
		CancelRequested: canceling,
		Cancellable:     running || pending || queued,
	}
}

// SetTurnAdmissionGuard installs a host-owned atomic route check. A successful
// guard returns a release callback that remains held until the controller has
// reserved its run state, preventing route teardown from overtaking admission.
func (c *Controller) SetTurnAdmissionGuard(guard func() (func(), bool)) {
	if guard == nil {
		c.turnAdmissionGuard.Store(nil)
		return
	}
	c.turnAdmissionGuard.Store(&turnAdmissionGuardHolder{guard: guard})
}

func (c *Controller) acquireTurnAdmission() (func(), bool) {
	holder := c.turnAdmissionGuard.Load()
	if holder == nil || holder.guard == nil {
		return func() {}, true
	}
	release, ok := holder.guard()
	if !ok {
		return nil, false
	}
	if release == nil {
		release = func() {}
	}
	return release, true
}

func (c *Controller) finishCompletionCallback() {
	// Acquire the session lifecycle reservation before publishing the queued
	// turn. c.running stays true across this hand-off, so another Send cannot
	// overtake it between the terminal callback and the next TurnStarted.
	c.sessionLifecycleMu.RLock()
	releaseAdmission, admitted := c.acquireTurnAdmission()
	if !admitted {
		c.mu.Lock()
		if c.completing > 0 {
			c.completing--
		}
		if c.completing == 0 {
			c.running = false
			c.queuedTurn = nil
		}
		c.mu.Unlock()
		c.sessionLifecycleMu.RUnlock()
		return
	}
	defer releaseAdmission()
	c.mu.Lock()
	if c.completing > 0 {
		c.completing--
	}
	if c.completing != 0 || c.queuedTurn == nil {
		if c.completing == 0 {
			c.running = false
		}
		c.mu.Unlock()
		c.sessionLifecycleMu.RUnlock()
		return
	}
	body := c.queuedTurn
	c.queuedTurn = nil
	ctx, cancel := context.WithCancel(context.Background())
	ctx = event.WithTurnID(ctx, event.NewTurnID())
	c.cancel = cancel
	c.canceling = false
	c.completedTurn = nil
	c.completedArtifactDelivery.Store(nil)
	c.mu.Unlock()
	autosaveDone := c.startAutosaveWhileRunning(ctx)
	c.launchGuardedTurn(body, ctx, cancel, autosaveDone)
}

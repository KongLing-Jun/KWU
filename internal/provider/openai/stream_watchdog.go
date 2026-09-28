package openai

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

type streamWatchdogReason int32

const (
	streamWatchdogNone streamWatchdogReason = iota
	streamWatchdogTransportIdle
	streamWatchdogNoProgress
	streamWatchdogRequestLimit
	streamWatchdogFinishGrace
)

type streamWatchdogConfig struct {
	transportIdle  time.Duration
	progressIdle   time.Duration
	requestLimit   time.Duration
	finishGrace    time.Duration
	requestStarted time.Time
}

type streamWatchdog struct {
	config     streamWatchdogConfig
	done       chan struct{}
	transport  chan struct{}
	progress   chan struct{}
	finish     chan struct{}
	stopOnce   sync.Once
	stopReason atomic.Int32
}

func startStreamWatchdog(ctx context.Context, body io.Closer, config streamWatchdogConfig) *streamWatchdog {
	w := &streamWatchdog{
		config:    config,
		done:      make(chan struct{}),
		transport: make(chan struct{}, 1),
		progress:  make(chan struct{}, 1),
		finish:    make(chan struct{}, 1),
	}
	go w.run(ctx, body)
	return w
}

type streamWatchdogTimers struct {
	transport      *time.Timer
	progress       *time.Timer
	request        *time.Timer
	finish         *time.Timer
	finishDeadline <-chan time.Time
}

func newStreamWatchdogTimers(config streamWatchdogConfig) *streamWatchdogTimers {
	requestRemaining := config.requestLimit - time.Since(config.requestStarted)
	if requestRemaining <= 0 {
		requestRemaining = time.Nanosecond
	}
	return &streamWatchdogTimers{
		transport: time.NewTimer(config.transportIdle),
		progress:  time.NewTimer(config.progressIdle),
		request:   time.NewTimer(requestRemaining),
	}
}

func (t *streamWatchdogTimers) stop() {
	t.transport.Stop()
	t.progress.Stop()
	t.request.Stop()
	if t.finish != nil {
		t.finish.Stop()
	}
}

func (t *streamWatchdogTimers) startFinish(timeout time.Duration) {
	if t.finish != nil {
		return
	}
	t.finish = time.NewTimer(timeout)
	t.finishDeadline = t.finish.C
}

func (w *streamWatchdog) run(ctx context.Context, body io.Closer) {
	timers := newStreamWatchdogTimers(w.config)
	defer timers.stop()
	for !w.watchOnce(ctx, body, timers) {
	}
}

func (w *streamWatchdog) watchOnce(ctx context.Context, body io.Closer, timers *streamWatchdogTimers) bool {
	select {
	case <-ctx.Done():
		_ = body.Close()
		return true
	case <-timers.transport.C:
		w.expire(body, streamWatchdogTransportIdle)
		return true
	case <-timers.progress.C:
		w.expire(body, streamWatchdogNoProgress)
		return true
	case <-timers.request.C:
		w.expire(body, streamWatchdogRequestLimit)
		return true
	case <-timers.finishDeadline:
		w.expire(body, streamWatchdogFinishGrace)
		return true
	case <-w.transport:
		resetStreamTimer(timers.transport, w.config.transportIdle)
	case <-w.progress:
		resetStreamTimer(timers.progress, w.config.progressIdle)
	case <-w.finish:
		timers.startFinish(w.config.finishGrace)
	case <-w.done:
		return true
	}
	return false
}

func resetStreamTimer(timer *time.Timer, timeout time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(timeout)
}

func pingStreamActivity(activity chan<- struct{}) {
	select {
	case activity <- struct{}{}:
	default:
	}
}

func (w *streamWatchdog) expire(body io.Closer, reason streamWatchdogReason) {
	w.stopReason.CompareAndSwap(int32(streamWatchdogNone), int32(reason))
	_ = body.Close()
}

func (w *streamWatchdog) stop() {
	w.stopOnce.Do(func() { close(w.done) })
}

func (w *streamWatchdog) transportActive()  { pingStreamActivity(w.transport) }
func (w *streamWatchdog) semanticProgress() { pingStreamActivity(w.progress) }
func (w *streamWatchdog) finishSeen()       { pingStreamActivity(w.finish) }
func (w *streamWatchdog) reason() streamWatchdogReason {
	return streamWatchdogReason(w.stopReason.Load())
}

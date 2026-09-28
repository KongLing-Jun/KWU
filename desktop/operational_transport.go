package main

import (
	"context"
	"runtime"
	"sync/atomic"
	"time"

	"kwu/internal/event"
	"kwu/internal/eventwire"
)

const defaultOperationalControlLaneCapacity = 64

type controlEnqueueDisposition string

const (
	controlEnqueueAccepted  controlEnqueueDisposition = "accepted"
	controlEnqueueQueueFull controlEnqueueDisposition = "queue_full"
	controlEnqueueDisabled  controlEnqueueDisposition = "disabled"
)

type runtimeControlStats struct {
	Accepted             uint64 `json:"accepted"`
	Emitted              uint64 `json:"emitted"`
	QueueFull            uint64 `json:"queueFull"`
	Disabled             uint64 `json:"disabled"`
	Late                 uint64 `json:"late"`
	DroppedOnDisconnect  uint64 `json:"droppedOnDisconnect"`
	EmitFailed           uint64 `json:"emitFailed"`
	Superseded           uint64 `json:"superseded"`
	EvictedForTerminal   uint64 `json:"evictedForTerminal"`
	QueueDepth           uint64 `json:"queueDepth"`
	QueueHighWater       uint64 `json:"queueHighWater"`
	QueueWaitSamples     uint64 `json:"queueWaitSamples"`
	QueueWaitTotalMicros uint64 `json:"queueWaitTotalMicros"`
	QueueWaitMaxMicros   uint64 `json:"queueWaitMaxMicros"`
	DeliverySamples      uint64 `json:"deliverySamples"`
	DeliveryTotalMicros  uint64 `json:"deliveryTotalMicros"`
	DeliveryMaxMicros    uint64 `json:"deliveryMaxMicros"`
}

func (e *asyncRuntimeEmitter) WaitControlDrained(ctx context.Context) bool {
	if e == nil {
		return true
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		if e.controlCounters.depth.Load() == 0 && e.activeControlEpoch.Load() == 0 {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
		}
	}
}

type runtimeControlCounterState struct {
	accepted            atomic.Uint64
	emitted             atomic.Uint64
	queueFull           atomic.Uint64
	disabled            atomic.Uint64
	late                atomic.Uint64
	droppedOnDisconnect atomic.Uint64
	emitFailed          atomic.Uint64
	superseded          atomic.Uint64
	evictedForTerminal  atomic.Uint64
	depth               atomic.Uint64
	highWater           atomic.Uint64
	queueWaitSamples    atomic.Uint64
	queueWaitTotalNanos atomic.Uint64
	queueWaitMaxNanos   atomic.Uint64
	deliverySamples     atomic.Uint64
	deliveryTotalNanos  atomic.Uint64
	deliveryMaxNanos    atomic.Uint64
}

type runtimeControlEnvelopeMetadata struct {
	aggregateKind string
	aggregateID   string
	terminal      bool
}

// EmitControl enqueues one operational delivery in a bounded lane that is
// selected before the legacy content queue. It reuses asyncRuntimeEmitter's
// existing queue lock and introduces no new lock.
func (e *asyncRuntimeEmitter) EmitControl(ctx context.Context, name string, payload ...interface{}) controlEnqueueDisposition {
	if e == nil || ctx == nil {
		if e != nil {
			e.controlCounters.disabled.Add(1)
		}
		return controlEnqueueDisabled
	}
	e.mu.Lock()
	if e.emit == nil && ctx.Value("events") == nil {
		e.mu.Unlock()
		e.controlCounters.disabled.Add(1)
		return controlEnqueueDisabled
	}
	capacity := e.controlCapacity
	if capacity <= 0 {
		capacity = defaultOperationalControlLaneCapacity
	}
	item := runtimeEventEnvelope{
		ctx: ctx, name: name, payload: append([]interface{}(nil), payload...),
		epoch: e.connectionEpoch(), control: runtimeControlMetadata(payload), queuedAt: time.Now(),
	}
	if len(e.controlQueue)-e.controlHead >= capacity {
		if replacement, sameAggregate := e.terminalReplacementLocked(item.control); replacement >= 0 {
			e.controlQueue[replacement] = item
			if sameAggregate {
				e.controlCounters.superseded.Add(1)
			} else {
				e.controlCounters.evictedForTerminal.Add(1)
			}
			e.mu.Unlock()
			e.controlCounters.accepted.Add(1)
			return controlEnqueueAccepted
		}
		e.mu.Unlock()
		e.controlCounters.queueFull.Add(1)
		return controlEnqueueQueueFull
	}
	e.controlQueue = append(e.controlQueue, item)
	e.controlCounters.observeEnqueue()
	if !e.running {
		e.running = true
		go e.run()
	}
	e.mu.Unlock()
	e.controlCounters.accepted.Add(1)
	return controlEnqueueAccepted
}

func (e *asyncRuntimeEmitter) ControlStats() runtimeControlStats {
	if e == nil {
		return runtimeControlStats{}
	}
	return runtimeControlStats{
		Accepted: e.controlCounters.accepted.Load(), Emitted: e.controlCounters.emitted.Load(),
		QueueFull: e.controlCounters.queueFull.Load(), Disabled: e.controlCounters.disabled.Load(),
		Late: e.controlCounters.late.Load(), DroppedOnDisconnect: e.controlCounters.droppedOnDisconnect.Load(),
		EmitFailed: e.controlCounters.emitFailed.Load(), Superseded: e.controlCounters.superseded.Load(),
		EvictedForTerminal: e.controlCounters.evictedForTerminal.Load(),
		QueueDepth:         e.controlCounters.depth.Load(), QueueHighWater: e.controlCounters.highWater.Load(),
		QueueWaitSamples: e.controlCounters.queueWaitSamples.Load(), QueueWaitTotalMicros: nanosToMicros(e.controlCounters.queueWaitTotalNanos.Load()),
		QueueWaitMaxMicros: nanosToMicros(e.controlCounters.queueWaitMaxNanos.Load()), DeliverySamples: e.controlCounters.deliverySamples.Load(),
		DeliveryTotalMicros: nanosToMicros(e.controlCounters.deliveryTotalNanos.Load()), DeliveryMaxMicros: nanosToMicros(e.controlCounters.deliveryMaxNanos.Load()),
	}
}

func (c *runtimeControlCounterState) observeEnqueue() {
	depth := c.depth.Add(1)
	updateAtomicMax(&c.highWater, depth)
}

func (c *runtimeControlCounterState) observeDequeue(wait time.Duration) {
	c.depth.Add(^uint64(0))
	nanos := uint64(max(wait.Nanoseconds(), 0))
	c.queueWaitSamples.Add(1)
	c.queueWaitTotalNanos.Add(nanos)
	updateAtomicMax(&c.queueWaitMaxNanos, nanos)
}

func (c *runtimeControlCounterState) observeDelivery(duration time.Duration) {
	nanos := uint64(max(duration.Nanoseconds(), 0))
	c.deliverySamples.Add(1)
	c.deliveryTotalNanos.Add(nanos)
	updateAtomicMax(&c.deliveryMaxNanos, nanos)
}

func updateAtomicMax(target *atomic.Uint64, value uint64) {
	for current := target.Load(); value > current; current = target.Load() {
		if target.CompareAndSwap(current, value) {
			return
		}
	}
}

func nanosToMicros(nanos uint64) uint64 { return nanos / uint64(time.Microsecond) }

func (e *asyncRuntimeEmitter) connectionEpoch() uint64 {
	if current := e.epoch.Load(); current != 0 {
		return current
	}
	e.epoch.CompareAndSwap(0, 1)
	return e.epoch.Load()
}

// BeginConnectionEpoch invalidates old control envelopes. activeControlEpoch
// closes the check-to-emit race without adding a lock: a transition waits only
// for an already-entered old delivery, while a runner entering later observes
// the new epoch and classifies its envelope as late.
func (e *asyncRuntimeEmitter) BeginConnectionEpoch() {
	if e == nil {
		return
	}
	current := e.connectionEpoch()
	var next uint64
	for {
		next = current + 1
		if e.epoch.CompareAndSwap(current, next) {
			break
		}
		current = e.epoch.Load()
	}
	for {
		active := e.activeControlEpoch.Load()
		if active == 0 || active-1 >= next {
			return
		}
		runtime.Gosched()
	}
}

func (e *asyncRuntimeEmitter) terminalReplacementLocked(incoming runtimeControlEnvelopeMetadata) (int, bool) {
	if !incoming.terminal {
		return -1, false
	}
	for index := e.controlHead; index < len(e.controlQueue); index++ {
		queued := e.controlQueue[index].control
		if incoming.aggregateKind != "" && incoming.aggregateID != "" && queued.aggregateKind == incoming.aggregateKind && queued.aggregateID == incoming.aggregateID {
			return index, true
		}
	}
	for index := e.controlHead; index < len(e.controlQueue); index++ {
		if !e.controlQueue[index].control.terminal {
			return index, false
		}
	}
	return -1, false
}

func runtimeControlMetadata(payload []interface{}) runtimeControlEnvelopeMetadata {
	if len(payload) == 0 {
		return runtimeControlEnvelopeMetadata{}
	}
	var operational *eventwire.OperationalEnvelope
	switch value := payload[0].(type) {
	case wireEventTab:
		operational = value.Operational
	case *wireEventTab:
		if value != nil {
			operational = value.Operational
		}
	case eventwire.Event:
		operational = value.Operational
	case *eventwire.Event:
		if value != nil {
			operational = value.Operational
		}
	}
	if operational == nil {
		return runtimeControlEnvelopeMetadata{}
	}
	kind := event.OperationalKind(operational.Kind)
	return runtimeControlEnvelopeMetadata{
		aggregateKind: operational.Aggregate.Kind,
		aggregateID:   operational.Aggregate.ID,
		terminal:      isOperationalTerminal(kind),
	}
}

func isOperationalTerminal(kind event.OperationalKind) bool {
	switch kind {
	case event.PromptResolved, event.PromptCancelled, event.PromptReplyFailed,
		event.TaskCompleted, event.TaskFailed, event.TaskCancelled, event.TaskRecoveryRequired,
		event.ModelCallCompleted, event.ModelCallFailed, event.ModelCallCancelled, event.ModelCallInterrupted, event.ModelCallUsageMissing,
		event.ToolCallCompleted, event.ToolCallFailed, event.ToolCallCancelled,
		event.SessionClosed,
		event.DashboardWriteCommitted, event.DashboardWriteRejected, event.DashboardWriteFailed,
		event.PlanExited, event.PlanCancelled:
		return true
	default:
		return false
	}
}

func (e *asyncRuntimeEmitter) nextRuntimeEventLocked() (runtimeEventEnvelope, bool, bool) {
	if e.controlHead < len(e.controlQueue) {
		item := e.controlQueue[e.controlHead]
		var zero runtimeEventEnvelope
		e.controlQueue[e.controlHead] = zero
		e.controlHead++
		e.controlCounters.observeDequeue(time.Since(item.queuedAt))
		if e.controlHead > 64 && e.controlHead*2 >= len(e.controlQueue) {
			e.controlQueue = append([]runtimeEventEnvelope(nil), e.controlQueue[e.controlHead:]...)
			e.controlHead = 0
		}
		return item, true, true
	}
	if e.head < len(e.queue) {
		item := e.queue[e.head]
		var zero runtimeEventEnvelope
		e.queue[e.head] = zero
		e.head++
		if e.head > 64 && e.head*2 >= len(e.queue) {
			e.queue = append([]runtimeEventEnvelope(nil), e.queue[e.head:]...)
			e.head = 0
		}
		return item, false, true
	}
	return runtimeEventEnvelope{}, false, false
}

func callRuntimeEmitter(emit runtimeEventEmitFunc, item runtimeEventEnvelope) (ok bool) {
	if emit == nil {
		return false
	}
	ok = true
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	emit(item.ctx, item.name, item.payload...)
	return ok
}

package event

import (
	"context"
	"sync"
	"time"
)

type operationalEmitterContextKey struct{}
type operationalScopeContextKey struct{}

func WithOperationalEmitter(ctx context.Context, emitter *OperationalEmitter) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if emitter == nil {
		return ctx
	}
	return context.WithValue(ctx, operationalEmitterContextKey{}, emitter)
}

func OperationalEmitterFrom(ctx context.Context) (*OperationalEmitter, bool) {
	if ctx == nil {
		return nil, false
	}
	emitter, ok := ctx.Value(operationalEmitterContextKey{}).(*OperationalEmitter)
	return emitter, ok && emitter != nil
}

func WithOperationalScope(ctx context.Context, scope OperationalScope) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, operationalScopeContextKey{}, scope)
}

func OperationalScopeFrom(ctx context.Context) OperationalScope {
	if ctx == nil {
		return OperationalScope{}
	}
	scope, _ := ctx.Value(operationalScopeContextKey{}).(OperationalScope)
	return scope
}

// OperationalFact is the content-free input accepted by OperationalEmitter.
// Producers call it only after their authoritative transition succeeds.
type OperationalFact struct {
	AggregateKind OperationalAggregateKind
	AggregateID   string
	Scope         OperationalScope
	Kind          OperationalKind
	Stage         string
	Outcome       string
	Severity      string
	UserImpact    string
	Retryable     bool
	CausationID   string
	Payload       *LifecycleOperationalPayload
}

// OperationalEmitter assigns aggregate-local sequence numbers. It is safe for
// concurrent model/tool completions and intentionally owns no business state.
type OperationalEmitter struct {
	Sink Sink
	Now  func() time.Time

	mu               sync.Mutex
	sequences        map[string]uint64
	lastEvents       map[string]string
	scopes           map[string]OperationalScope
	latestCallByTask map[string]string
	latestToolByCall map[string]string
}

// Emit publishes one normalized fact and returns its event identity. Producers
// may use the identity to join a cross-aggregate transition (for example a
// prompt causing a tool approval). Empty means the fact was rejected.
func (e *OperationalEmitter) Emit(fact OperationalFact) string {
	if e == nil || e.Sink == nil || fact.AggregateKind == "" || fact.AggregateID == "" || fact.Kind == "" {
		return ""
	}
	// Sequence assignment and delivery are one ordered operation. Assigning
	// under a lock but delivering after unlock lets concurrent producers expose
	// sequence N+1 before N even when the downstream sink serializes calls.
	e.mu.Lock()
	defer e.mu.Unlock()
	fact.Scope = e.enrichScope(fact)
	if fact.CausationID == "" {
		fact.CausationID = e.inferCausation(fact)
	}
	eventID := NewEventID()
	envelope := &OperationalEnvelope{
		SchemaVersion: OperationalSchemaVersion,
		EventID:       eventID,
		OperationID:   fact.AggregateID,
		CausationID:   fact.CausationID,
		Aggregate:     OperationalAggregate{Kind: fact.AggregateKind, ID: fact.AggregateID, Sequence: e.nextSequence(fact)},
		Scope:         fact.Scope,
		Kind:          fact.Kind,
		Stage:         fact.Stage,
		Outcome:       fact.Outcome,
		Severity:      fact.Severity,
		UserImpact:    fact.UserImpact,
		Retryable:     fact.Retryable,
		PrivacyClass:  PrivacyMetadataOnly,
		OccurredAt:    e.occurredAt(),
	}
	attachOperationalPayload(envelope, fact.AggregateKind, fact.Payload)
	e.remember(eventID, fact)
	e.Sink.Emit(Event{Kind: Operational, EventID: eventID, TurnID: fact.Scope.TurnID, CandidateID: fact.Scope.CandidateID, Operational: envelope})
	return eventID
}

func operationalAggregateKey(kind OperationalAggregateKind, id string) string {
	return string(kind) + ":" + id
}

func (e *OperationalEmitter) inferCausation(fact OperationalFact) string {
	if previous := e.lastEvents[operationalAggregateKey(fact.AggregateKind, fact.AggregateID)]; previous != "" {
		return previous
	}
	parentKind, parentID := parentOperationalAggregate(fact.AggregateKind, fact.Scope)
	if parentID == "" {
		return ""
	}
	return e.lastEvents[operationalAggregateKey(parentKind, parentID)]
}

var operationalParentResolvers = map[OperationalAggregateKind]func(OperationalScope) (OperationalAggregateKind, string){
	AggregatePlan: func(scope OperationalScope) (OperationalAggregateKind, string) {
		return AggregateSession, scope.SessionID
	},
	AggregateTask: func(scope OperationalScope) (OperationalAggregateKind, string) {
		if scope.PlanID != "" {
			return AggregatePlan, scope.PlanID
		}
		return AggregateSession, scope.SessionID
	},
	AggregateModelCall: func(scope OperationalScope) (OperationalAggregateKind, string) {
		return AggregateTask, scope.TaskID
	},
	AggregateToolCall: func(scope OperationalScope) (OperationalAggregateKind, string) {
		if scope.CallID != "" {
			return AggregateModelCall, scope.CallID
		}
		return AggregateTask, scope.TaskID
	},
	AggregateDashboardWrite: func(scope OperationalScope) (OperationalAggregateKind, string) {
		if scope.ToolCallID != "" {
			return AggregateToolCall, scope.ToolCallID
		}
		if scope.CallID != "" {
			return AggregateModelCall, scope.CallID
		}
		return AggregateTask, scope.TaskID
	},
	AggregatePrompt: func(scope OperationalScope) (OperationalAggregateKind, string) {
		return AggregateToolCall, scope.ToolCallID
	},
}

func parentOperationalAggregate(kind OperationalAggregateKind, scope OperationalScope) (OperationalAggregateKind, string) {
	resolve := operationalParentResolvers[kind]
	if resolve == nil {
		return "", ""
	}
	return resolve(scope)
}

func (e *OperationalEmitter) enrichScope(fact OperationalFact) OperationalScope {
	scope := fact.Scope
	merge := func(kind OperationalAggregateKind, id string) {
		if id != "" {
			scope = MergeOperationalScope(e.scopes[operationalAggregateKey(kind, id)], scope)
		}
	}
	switch fact.AggregateKind {
	case AggregatePlan:
		merge(AggregateSession, scope.SessionID)
	case AggregateTask:
		merge(AggregatePlan, scope.PlanID)
		merge(AggregateSession, scope.SessionID)
	case AggregateModelCall:
		merge(AggregateTask, scope.TaskID)
	case AggregateToolCall:
		scope = e.enrichToolScope(scope)
	case AggregateDashboardWrite:
		scope = e.enrichDashboardScope(scope)
	case AggregatePrompt:
		merge(AggregateToolCall, scope.ToolCallID)
		merge(AggregateModelCall, scope.CallID)
		merge(AggregateTask, scope.TaskID)
	}
	return scope
}

func (e *OperationalEmitter) cachedScope(kind OperationalAggregateKind, id string) OperationalScope {
	if id == "" {
		return OperationalScope{}
	}
	return e.scopes[operationalAggregateKey(kind, id)]
}

func (e *OperationalEmitter) enrichToolScope(scope OperationalScope) OperationalScope {
	scope = MergeOperationalScope(e.cachedScope(AggregateTask, scope.TaskID), scope)
	if scope.CallID == "" {
		scope.CallID = e.latestCallByTask[scope.TaskID]
	}
	return MergeOperationalScope(e.cachedScope(AggregateModelCall, scope.CallID), scope)
}

func (e *OperationalEmitter) enrichDashboardScope(scope OperationalScope) OperationalScope {
	scope = e.enrichToolScope(scope)
	if scope.ToolCallID == "" {
		scope.ToolCallID = e.latestToolByCall[scope.CallID]
	}
	return MergeOperationalScope(e.cachedScope(AggregateToolCall, scope.ToolCallID), scope)
}

func (e *OperationalEmitter) remember(eventID string, fact OperationalFact) {
	if e.lastEvents == nil {
		e.lastEvents = make(map[string]string)
		e.scopes = make(map[string]OperationalScope)
		e.latestCallByTask = make(map[string]string)
		e.latestToolByCall = make(map[string]string)
	}
	key := operationalAggregateKey(fact.AggregateKind, fact.AggregateID)
	e.lastEvents[key] = eventID
	e.scopes[key] = fact.Scope
	if fact.AggregateKind == AggregateModelCall && fact.Scope.TaskID != "" {
		e.latestCallByTask[fact.Scope.TaskID] = fact.AggregateID
	}
	if fact.AggregateKind == AggregateToolCall && fact.Scope.CallID != "" {
		e.latestToolByCall[fact.Scope.CallID] = fact.AggregateID
	}
}

// LastEvent returns the latest event identity for one aggregate. It is used
// only where a producer outside OperationalEmitter owns the authoritative
// event, notably the content-bearing prompt lifecycle.
func (e *OperationalEmitter) LastEvent(kind OperationalAggregateKind, id string) string {
	if e == nil || id == "" {
		return ""
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lastEvents[operationalAggregateKey(kind, id)]
}

// MergeOperationalScope preserves inherited routing/correlation fields while
// letting the more specific child scope override them.
func MergeOperationalScope(parent, child OperationalScope) OperationalScope {
	fields := []*string{
		&child.TabID, &child.SessionID, &child.TurnID, &child.CandidateID,
		&child.PromptID, &child.TaskID, &child.CallID, &child.ToolCallID,
		&child.PlanID, &child.DashboardWriteID,
	}
	parents := []string{
		parent.TabID, parent.SessionID, parent.TurnID, parent.CandidateID,
		parent.PromptID, parent.TaskID, parent.CallID, parent.ToolCallID,
		parent.PlanID, parent.DashboardWriteID,
	}
	for index := range fields {
		if *fields[index] == "" {
			*fields[index] = parents[index]
		}
	}
	return child
}

func (e *OperationalEmitter) nextSequence(fact OperationalFact) uint64 {
	key := string(fact.AggregateKind) + ":" + fact.AggregateID
	if e.sequences == nil {
		e.sequences = make(map[string]uint64)
	}
	e.sequences[key]++
	return e.sequences[key]
}

func (e *OperationalEmitter) occurredAt() time.Time {
	if e.Now != nil {
		return e.Now().UTC()
	}
	return time.Now().UTC()
}

func attachOperationalPayload(envelope *OperationalEnvelope, kind OperationalAggregateKind, payload *LifecycleOperationalPayload) {
	switch kind {
	case AggregateTask:
		envelope.Task = payload
	case AggregateModelCall:
		envelope.ModelCall = payload
	case AggregateToolCall:
		envelope.ToolCall = payload
	case AggregateSession:
		envelope.Session = payload
	case AggregateDashboardWrite:
		envelope.DashboardWrite = payload
	case AggregatePlan:
		envelope.Plan = payload
	}
}

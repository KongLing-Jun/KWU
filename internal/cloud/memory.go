package cloud

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"kwu/internal/userctx"
)

func (a *AuthApplication) rollbackCanceledAttempt(attempt authViewAttempt) AuthView {
	view := attempt.prior
	if view.State == "" {
		view = safeViewFromIdentity(attempt.identity)
	}
	if !a.rollbackViewTo(attempt, view) {
		return a.stableViewFromKey(attempt.key)
	}
	return view
}

func safeViewFromIdentity(identity authIdentitySnapshot) AuthView {
	state := AccountAnonymous
	if identity.binding.Status != BindingUnbound && identity.binding.Status != BindingBound {
		state = AccountOffline
	}
	if identity.sessionExists {
		switch identity.session.State {
		case SessionAuthenticated:
			state = AccountAuthenticated
		case SessionExpired:
			state = AccountExpired
		case SessionOffline:
			state = AccountOffline
		}
	}
	return AuthView{State: state, Binding: identity.binding}
}

func (a *AuthApplication) stableView(actor userctx.ActorContext) AuthView {
	actor = actor.Normalize()
	return a.stableViewFromKey(actor.ActorID + "\x00" + actor.ProfileID)
}

func (a *AuthApplication) stableViewFromKey(key string) AuthView {
	a.mu.RLock()
	view := a.views[key].stable
	a.mu.RUnlock()
	return view
}

func (a *AuthApplication) failureView(actor userctx.ActorContext, err error) AuthView {
	if isNetworkUnavailable(err) {
		return a.setTransient(actor, AccountOffline)
	}
	if isProfileMismatch(err) {
		return a.setTransient(actor, AccountOffline)
	}
	return a.setTransient(actor, AccountAnonymous)
}

func (a *AuthApplication) setAnonymous(actor userctx.ActorContext, binding ProfileBinding) error {
	a.setView(actor, AuthView{State: AccountAnonymous, Binding: binding})
	return nil
}

func (a *AuthApplication) setTransient(actor userctx.ActorContext, state AccountStatus) AuthView {
	return a.setView(actor, AuthView{State: state})
}

func (a *AuthApplication) setView(actor userctx.ActorContext, view AuthView) AuthView {
	actor = actor.Normalize()
	key := actor.ActorID + "\x00" + actor.ProfileID
	a.mu.Lock()
	entry := a.views[key]
	entry.view = view
	if view.State != AccountAuthenticating && view.State != AccountRefreshing {
		entry.stable = view
	}
	entry.revision++
	a.views[key] = entry
	a.mu.Unlock()
	return view
}

func (a *AuthApplication) beginTransient(actor userctx.ActorContext, state AccountStatus) authViewAttempt {
	actor = actor.Normalize()
	key := actor.ActorID + "\x00" + actor.ProfileID
	a.mu.Lock()
	entry := a.views[key]
	prior := entry.stable
	entry.view = AuthView{State: state}
	entry.revision++
	a.views[key] = entry
	a.mu.Unlock()
	return authViewAttempt{key: key, prior: prior, revision: entry.revision}
}

func (a *AuthApplication) rollbackView(attempt authViewAttempt) bool {
	return a.rollbackViewTo(attempt, attempt.prior)
}

func (a *AuthApplication) rollbackViewTo(attempt authViewAttempt, view AuthView) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	entry := a.views[attempt.key]
	if entry.revision != attempt.revision {
		return false
	}
	entry.view = view
	entry.stable = view
	entry.revision++
	a.views[attempt.key] = entry
	return true
}

func (a *AuthApplication) cachedView(actor userctx.ActorContext, binding ProfileBinding) AuthView {
	actor = actor.Normalize()
	a.mu.RLock()
	view := a.views[actor.ActorID+"\x00"+actor.ProfileID].view
	a.mu.RUnlock()
	view.Binding = binding
	return view
}

func (a *AuthApplication) transientView(actor userctx.ActorContext, binding ProfileBinding) (AuthView, bool) {
	actor = actor.Normalize()
	a.mu.RLock()
	view := a.views[actor.ActorID+"\x00"+actor.ProfileID].view
	a.mu.RUnlock()
	if view.State != AccountAuthenticating && view.State != AccountRefreshing {
		return AuthView{}, false
	}
	view.Binding = binding
	return view, true
}

// StaticBindingResolver supplies one authenticated binding to local development
// and tests while still enforcing the active profile boundary.
type StaticBindingResolver struct {
	binding ProfileBinding
}

func NewStaticBindingResolver(binding ProfileBinding) (*StaticBindingResolver, error) {
	if err := binding.Validate(); err != nil {
		return nil, err
	}
	if binding.Status != BindingBound {
		return nil, fmt.Errorf("cloud: static resolver requires a bound profile")
	}
	return &StaticBindingResolver{binding: binding}, nil
}

func (r *StaticBindingResolver) ResolveBinding(ctx context.Context, actor userctx.ActorContext) (ProfileBinding, error) {
	if err := ctx.Err(); err != nil {
		return ProfileBinding{}, err
	}
	actor = actor.Normalize()
	if actor.ProfileID != r.binding.ProfileID {
		return ProfileBinding{}, &Error{
			Code:    ErrorProfileScopeMismatch,
			Message: fmt.Sprintf("profile %q cannot access binding for %q", actor.ProfileID, r.binding.ProfileID),
		}
	}
	return r.binding, nil
}

type MemorySessionStore struct {
	mu       sync.RWMutex
	sessions map[string]Session
}

func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{sessions: make(map[string]Session)}
}

func (s *MemorySessionStore) Save(ctx context.Context, session Session) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := session.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	s.sessions[session.Scope.key()] = session
	s.mu.Unlock()
	return nil
}

func (s *MemorySessionStore) Load(ctx context.Context, scope BindingScope) (Session, bool, error) {
	if err := ctx.Err(); err != nil {
		return Session{}, false, err
	}
	if err := scope.Validate(); err != nil {
		return Session{}, false, err
	}
	s.mu.RLock()
	session, ok := s.sessions[scope.key()]
	s.mu.RUnlock()
	return session, ok, nil
}

func (s *MemorySessionStore) Clear(ctx context.Context, scope BindingScope) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := scope.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.sessions, scope.key())
	s.mu.Unlock()
	return nil
}

type FreezeFixture struct {
	Name    string        `json:"name"`
	Request FreezeRequest `json:"request"`
	Result  *FreezeResult `json:"result,omitempty"`
	Error   *Error        `json:"error,omitempty"`
}

func (f FreezeFixture) Validate() error {
	if strings.TrimSpace(f.Name) == "" {
		return fmt.Errorf("cloud: freeze fixture name is required")
	}
	if err := f.Request.Validate(); err != nil {
		return fmt.Errorf("cloud: freeze fixture %q request: %w", f.Name, err)
	}
	if err := f.validateOutcome(); err != nil {
		return err
	}
	return f.validateIdempotencyKey()
}

func (f FreezeFixture) validateOutcome() error {
	if (f.Result == nil) == (f.Error == nil) {
		return fmt.Errorf("cloud: freeze fixture %q must define exactly one result or error", f.Name)
	}
	if f.Result != nil {
		if err := f.Result.Validate(); err != nil {
			return fmt.Errorf("cloud: freeze fixture %q result: %w", f.Name, err)
		}
		if f.Result.Freeze.Scope != f.Request.Scope || f.Result.Freeze.TaskID != f.Request.TaskID {
			return fmt.Errorf("cloud: freeze fixture %q result scope/task does not match request", f.Name)
		}
	}
	if f.Error != nil && f.Error.Code == "" {
		return fmt.Errorf("cloud: freeze fixture %q error code is required", f.Name)
	}
	return nil
}

func (f FreezeFixture) validateIdempotencyKey() error {
	expectedKey, err := BuildIdempotencyKey(IdempotencyActionCreateFreeze, f.Request.Scope, f.Request.TaskID)
	if err != nil {
		return fmt.Errorf("cloud: freeze fixture %q idempotency key: %w", f.Name, err)
	}
	if f.Request.IdempotencyKey != expectedKey {
		return fmt.Errorf("cloud: freeze fixture %q idempotency key does not match the shared rule", f.Name)
	}
	return nil
}

// FixtureCreditClient is deterministic: repeated requests for the same task
// return the same fixture result and never create a second freeze.
type FixtureCreditClient struct {
	byTask   map[string]FreezeFixture
	byFreeze map[string]FreezeResult
}

func NewFixtureCreditClient(fixtures []FreezeFixture) (*FixtureCreditClient, error) {
	client := &FixtureCreditClient{
		byTask:   make(map[string]FreezeFixture, len(fixtures)),
		byFreeze: make(map[string]FreezeResult, len(fixtures)),
	}
	for _, fixture := range fixtures {
		if err := fixture.Validate(); err != nil {
			return nil, err
		}
		if _, exists := client.byTask[fixture.Request.TaskID]; exists {
			return nil, fmt.Errorf("cloud: duplicate freeze fixture task_id %q", fixture.Request.TaskID)
		}
		stored := cloneFreezeFixture(fixture)
		client.byTask[stored.Request.TaskID] = stored
		if stored.Result != nil {
			client.byFreeze[stored.Result.Freeze.FreezeID] = *stored.Result
		}
	}
	return client, nil
}

func (c *FixtureCreditClient) CreateFreeze(ctx context.Context, request FreezeRequest) (FreezeResult, error) {
	if err := ctx.Err(); err != nil {
		return FreezeResult{}, err
	}
	if err := request.Validate(); err != nil {
		return FreezeResult{}, err
	}
	fixture, ok := c.byTask[request.TaskID]
	if !ok {
		return FreezeResult{}, &Error{Code: ErrorFreezeNotFound, Message: "no freeze fixture for task"}
	}
	if fixture.Request.Scope != request.Scope {
		return FreezeResult{}, &Error{Code: ErrorProfileScopeMismatch, Message: "freeze fixture belongs to another binding"}
	}
	if fixture.Request != request {
		return FreezeResult{}, fmt.Errorf("cloud: freeze request for task %q does not match its fixture", request.TaskID)
	}
	if fixture.Error != nil {
		copy := *fixture.Error
		copy.Details = append([]byte(nil), fixture.Error.Details...)
		return FreezeResult{}, &copy
	}
	return *fixture.Result, nil
}

func (c *FixtureCreditClient) GetFreeze(ctx context.Context, scope BindingScope, freezeID string) (FreezeResult, error) {
	if err := ctx.Err(); err != nil {
		return FreezeResult{}, err
	}
	if err := scope.Validate(); err != nil {
		return FreezeResult{}, err
	}
	result, ok := c.byFreeze[strings.TrimSpace(freezeID)]
	if !ok {
		return FreezeResult{}, &Error{Code: ErrorFreezeNotFound, Message: "freeze fixture not found"}
	}
	if result.Freeze.Scope != scope {
		return FreezeResult{}, &Error{Code: ErrorProfileScopeMismatch, Message: "freeze belongs to another binding"}
	}
	return result, nil
}

type FixtureEventSource struct {
	events []Event
}

func NewFixtureEventSource(events []Event) (*FixtureEventSource, error) {
	cloned := make([]Event, len(events))
	for i, event := range events {
		if err := event.Validate(); err != nil {
			return nil, fmt.Errorf("cloud: event fixture %d: %w", i, err)
		}
		cloned[i] = cloneEvent(event)
	}
	return &FixtureEventSource{events: cloned}, nil
}

func (s *FixtureEventSource) Connect(ctx context.Context, scope BindingScope) (<-chan Event, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out := make(chan Event)
	go func() {
		defer close(out)
		for _, event := range s.events {
			select {
			case out <- cloneEvent(event):
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

func cloneEvent(event Event) Event {
	event.Data = append([]byte(nil), event.Data...)
	return event
}

func cloneFreezeFixture(fixture FreezeFixture) FreezeFixture {
	if fixture.Result != nil {
		result := *fixture.Result
		fixture.Result = &result
	}
	if fixture.Error != nil {
		cloudErr := *fixture.Error
		cloudErr.Details = append([]byte(nil), fixture.Error.Details...)
		fixture.Error = &cloudErr
	}
	return fixture
}

var (
	_ BindingResolver   = (*StaticBindingResolver)(nil)
	_ CloudSessionStore = (*MemorySessionStore)(nil)
	_ CloudCreditClient = (*FixtureCreditClient)(nil)
	_ CloudEventSource  = (*FixtureEventSource)(nil)
)

// AssetSnapshotRepository stores cloud-authoritative balance and subscription
// snapshots by the complete binding scope.
type AssetSnapshotRepository interface {
	LoadBalance(context.Context, BindingScope) (BalanceSnapshot, bool, error)
	StoreBalance(context.Context, BindingScope, BalanceSnapshot) (BalanceSnapshot, error)
	LoadSubscription(context.Context, BindingScope) (SubscriptionView, bool, error)
	StoreSubscription(context.Context, BindingScope, SubscriptionView) error
	Clear(context.Context, BindingScope) error
}

type MemoryAssetSnapshotRepository struct {
	mu            sync.RWMutex
	balances      map[string]BalanceSnapshot
	subscriptions map[string]SubscriptionView
}

func NewMemoryAssetSnapshotRepository() *MemoryAssetSnapshotRepository {
	return &MemoryAssetSnapshotRepository{
		balances:      make(map[string]BalanceSnapshot),
		subscriptions: make(map[string]SubscriptionView),
	}
}

func (r *MemoryAssetSnapshotRepository) LoadBalance(ctx context.Context, scope BindingScope) (BalanceSnapshot, bool, error) {
	if err := validateAssetSnapshotContext(ctx, scope); err != nil {
		return BalanceSnapshot{}, false, err
	}
	r.mu.RLock()
	snapshot, ok := r.balances[scope.key()]
	r.mu.RUnlock()
	return snapshot, ok, nil
}

func (r *MemoryAssetSnapshotRepository) StoreBalance(ctx context.Context, scope BindingScope, snapshot BalanceSnapshot) (BalanceSnapshot, error) {
	if err := validateBalanceAssetSnapshot(ctx, scope, snapshot); err != nil {
		return BalanceSnapshot{}, err
	}
	r.mu.Lock()
	current, ok := r.balances[scope.key()]
	if !ok || snapshot.Version > current.Version {
		current = snapshot
		r.balances[scope.key()] = snapshot
	}
	r.mu.Unlock()
	return current, nil
}

func (r *MemoryAssetSnapshotRepository) LoadSubscription(ctx context.Context, scope BindingScope) (SubscriptionView, bool, error) {
	if err := validateAssetSnapshotContext(ctx, scope); err != nil {
		return SubscriptionView{}, false, err
	}
	r.mu.RLock()
	view, ok := r.subscriptions[scope.key()]
	r.mu.RUnlock()
	return subscriptionView(view.Plans, view.Subscription), ok, nil
}

func (r *MemoryAssetSnapshotRepository) StoreSubscription(ctx context.Context, scope BindingScope, view SubscriptionView) error {
	if err := validateSubscriptionAssetSnapshot(ctx, scope, view); err != nil {
		return err
	}
	r.mu.Lock()
	r.subscriptions[scope.key()] = subscriptionView(view.Plans, view.Subscription)
	r.mu.Unlock()
	return nil
}

func (r *MemoryAssetSnapshotRepository) Clear(ctx context.Context, scope BindingScope) error {
	if err := validateAssetSnapshotContext(ctx, scope); err != nil {
		return err
	}
	r.mu.Lock()
	delete(r.balances, scope.key())
	delete(r.subscriptions, scope.key())
	r.mu.Unlock()
	return nil
}

func validateAssetSnapshotContext(ctx context.Context, scope BindingScope) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return scope.Validate()
}

func validateBalanceAssetSnapshot(ctx context.Context, scope BindingScope, snapshot BalanceSnapshot) error {
	if err := validateAssetSnapshotContext(ctx, scope); err != nil {
		return err
	}
	return snapshot.Validate()
}

func validateSubscriptionAssetSnapshot(ctx context.Context, scope BindingScope, view SubscriptionView) error {
	if err := validateAssetSnapshotContext(ctx, scope); err != nil {
		return err
	}
	if err := view.Subscription.Validate(); err != nil {
		return err
	}
	if view.Subscription.Scope != scope {
		return fmt.Errorf("cloud: subscription snapshot belongs to another binding")
	}
	for _, plan := range view.Plans {
		if err := plan.Validate(); err != nil {
			return err
		}
	}
	return nil
}

var _ AssetSnapshotRepository = (*MemoryAssetSnapshotRepository)(nil)

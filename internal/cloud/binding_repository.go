package cloud

import (
	"context"
	"fmt"
	"sync"

	"kwu/internal/userctx"
)

// BindingRepository keeps the cloud identity assigned to each local profile.
// It never selects or creates a local profile; callers supply the current actor.
type BindingRepository struct {
	mu       sync.RWMutex
	bindings map[string]ProfileBinding
}

func NewBindingRepository() *BindingRepository {
	return &BindingRepository{bindings: make(map[string]ProfileBinding)}
}

func (r *BindingRepository) ResolveBinding(ctx context.Context, actor userctx.ActorContext) (ProfileBinding, error) {
	if err := ctx.Err(); err != nil {
		return ProfileBinding{}, err
	}
	actor = actor.Normalize()
	r.mu.RLock()
	binding, ok := r.bindings[actor.ProfileID]
	r.mu.RUnlock()
	if !ok {
		return unboundBinding(actor), nil
	}
	return bindingForActor(binding, actor)
}

// ConfirmBinding confirms a cloud user for the current profile. A different
// cloud user is only a candidate until the caller explicitly rebinds.
func (r *BindingRepository) ConfirmBinding(ctx context.Context, actor userctx.ActorContext, cloudUserID string) (ProfileBinding, error) {
	if err := ctx.Err(); err != nil {
		return ProfileBinding{}, err
	}
	if err := requireTrimmed("cloud_user_id", cloudUserID); err != nil {
		return ProfileBinding{}, err
	}
	actor = actor.Normalize()
	r.mu.Lock()
	defer r.mu.Unlock()

	binding, ok := r.bindings[actor.ProfileID]
	if !ok {
		binding = ProfileBinding{
			ActorID:           actor.ActorID,
			ProfileID:         actor.ProfileID,
			CloudUserID:       cloudUserID,
			Status:            BindingBound,
			BindingGeneration: 1,
		}
		r.bindings[actor.ProfileID] = binding
		return binding, nil
	}
	var err error
	binding, err = bindingForActor(binding, actor)
	if err != nil {
		return ProfileBinding{}, err
	}
	if binding.Status == BindingBound && binding.CloudUserID == cloudUserID {
		return binding, nil
	}
	return binding, profileScopeMismatch(actor, binding)
}

// Rebind changes the cloud user only through an explicit caller action. The
// replacement receives a new generation, leaving all old scoped state isolated.
func (r *BindingRepository) Rebind(ctx context.Context, actor userctx.ActorContext, cloudUserID string) (ProfileBinding, error) {
	if err := ctx.Err(); err != nil {
		return ProfileBinding{}, err
	}
	if err := requireTrimmed("cloud_user_id", cloudUserID); err != nil {
		return ProfileBinding{}, err
	}
	actor = actor.Normalize()
	r.mu.Lock()
	defer r.mu.Unlock()

	binding, ok := r.bindings[actor.ProfileID]
	if !ok {
		return ProfileBinding{}, &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: cannot rebind an unbound profile"}
	}
	var err error
	binding, err = bindingForActor(binding, actor)
	if err != nil {
		return ProfileBinding{}, err
	}
	if binding.CloudUserID == cloudUserID && binding.Status == BindingBound {
		return binding, nil
	}
	binding.CloudUserID = cloudUserID
	binding.Status = BindingBound
	binding.BindingGeneration++
	if binding.BindingGeneration == 0 {
		return ProfileBinding{}, fmt.Errorf("cloud: binding_generation overflow")
	}
	r.bindings[actor.ProfileID] = binding
	return binding, nil
}

// CompareAndRebind changes the cloud user only when the complete persisted
// binding still equals expected. The comparison and mutation share one lock.
func (r *BindingRepository) CompareAndRebind(ctx context.Context, actor userctx.ActorContext, expected ProfileBinding, cloudUserID string) (ProfileBinding, error) {
	if err := ctx.Err(); err != nil {
		return ProfileBinding{}, err
	}
	actor = actor.Normalize()
	if err := validateCompareAndRebind(actor, expected, cloudUserID); err != nil {
		return ProfileBinding{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return ProfileBinding{}, err
	}

	current, ok := r.bindings[actor.ProfileID]
	if !ok || current != expected {
		return ProfileBinding{}, &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: binding changed before account switch"}
	}
	next, err := comparedRebinding(expected, cloudUserID)
	if err != nil {
		return ProfileBinding{}, err
	}
	r.bindings[actor.ProfileID] = next
	return next, nil
}

func validateCompareAndRebind(actor userctx.ActorContext, expected ProfileBinding, cloudUserID string) error {
	if err := requireTrimmed("cloud_user_id", cloudUserID); err != nil {
		return err
	}
	if err := expected.Validate(); err != nil {
		return err
	}
	if expected.ActorID != actor.ActorID || expected.ProfileID != actor.ProfileID || expected.Status != BindingBound {
		return &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: expected binding does not belong to the current actor"}
	}
	if expected.CloudUserID == cloudUserID {
		return &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: account switch requires a different cloud user"}
	}
	return nil
}

func comparedRebinding(expected ProfileBinding, cloudUserID string) (ProfileBinding, error) {
	if expected.BindingGeneration == ^uint64(0) {
		return ProfileBinding{}, fmt.Errorf("cloud: binding_generation overflow")
	}
	next := expected
	next.CloudUserID = cloudUserID
	next.Status = BindingBound
	next.BindingGeneration++
	return next, nil
}

func unboundBinding(actor userctx.ActorContext) ProfileBinding {
	return ProfileBinding{ActorID: actor.ActorID, ProfileID: actor.ProfileID, Status: BindingUnbound}
}

func bindingForActor(binding ProfileBinding, actor userctx.ActorContext) (ProfileBinding, error) {
	if binding.ActorID != actor.ActorID {
		return ProfileBinding{}, &Error{
			Code:    ErrorProfileScopeMismatch,
			Message: fmt.Sprintf("actor %q cannot access binding owned by %q", actor.ActorID, binding.ActorID),
		}
	}
	return binding, nil
}

func profileScopeMismatch(actor userctx.ActorContext, binding ProfileBinding) error {
	return &Error{
		Code:    ErrorProfileScopeMismatch,
		Message: fmt.Sprintf("profile %q is not bound to cloud user %q", actor.ProfileID, binding.CloudUserID),
	}
}

var _ BindingStore = (*BindingRepository)(nil)

// ProfileSessionRepository limits session access to the binding resolved from
// the caller's current ActorContext.
type ProfileSessionRepository struct {
	bindings BindingResolver
	sessions CloudSessionStore
}

type stagedProfileSession struct {
	repository *ProfileSessionRepository
	scope      BindingScope
	prior      Session
	hadPrior   bool
}

type rejectedBindingReplacer interface {
	PrepareRejectedBinding(context.Context, userctx.ActorContext, string) (ProfileBinding, error)
	ReplaceRejectedBinding(context.Context, userctx.ActorContext, ProfileBinding) (ProfileBinding, error)
}

type rejectedSessionReplacer interface {
	ReplaceRejected(context.Context, Session) error
}

func NewProfileSessionRepository(bindings BindingResolver, sessions CloudSessionStore) *ProfileSessionRepository {
	return &ProfileSessionRepository{bindings: bindings, sessions: sessions}
}

func (r *ProfileSessionRepository) Save(ctx context.Context, actor userctx.ActorContext, session Session) error {
	scope, err := r.currentScope(ctx, actor)
	if err != nil {
		return err
	}
	if session.Scope != scope {
		return &Error{Code: ErrorProfileScopeMismatch, Message: "cloud: session belongs to another binding"}
	}
	return r.sessions.Save(ctx, session)
}

// stage saves a validated session by its projected binding scope before the
// binding document is committed. This prevents a failed session write from
// leaving a profile bound without a recoverable session.
func (r *ProfileSessionRepository) stage(ctx context.Context, session Session) (*stagedProfileSession, error) {
	if r == nil || r.sessions == nil {
		return nil, fmt.Errorf("cloud: profile session repository is not configured")
	}
	if err := session.Validate(); err != nil {
		return nil, err
	}
	prior, found, err := r.sessions.Load(ctx, session.Scope)
	if err != nil {
		replacer, ok := r.sessions.(rejectedSessionReplacer)
		if !ok || !isCredentialStateRejected(err) {
			return nil, err
		}
		if err := replacer.ReplaceRejected(ctx, session); err != nil {
			return nil, err
		}
		return &stagedProfileSession{repository: r, scope: session.Scope}, nil
	}
	if err := r.sessions.Save(ctx, session); err != nil {
		return nil, err
	}
	return &stagedProfileSession{repository: r, scope: session.Scope, prior: prior, hadPrior: found}, nil
}

func (s *stagedProfileSession) rollback(ctx context.Context) error {
	if s == nil || s.repository == nil || s.repository.sessions == nil {
		return nil
	}
	if s.hadPrior {
		return s.repository.sessions.Save(ctx, s.prior)
	}
	return s.repository.sessions.Clear(ctx, s.scope)
}

func (r *ProfileSessionRepository) Load(ctx context.Context, actor userctx.ActorContext) (Session, bool, error) {
	scope, err := r.currentScope(ctx, actor)
	if err != nil {
		return Session{}, false, err
	}
	return r.sessions.Load(ctx, scope)
}

func (r *ProfileSessionRepository) Clear(ctx context.Context, actor userctx.ActorContext) error {
	scope, err := r.currentScope(ctx, actor)
	if err != nil {
		return err
	}
	return r.sessions.Clear(ctx, scope)
}

func (r *ProfileSessionRepository) currentScope(ctx context.Context, actor userctx.ActorContext) (BindingScope, error) {
	if r.bindings == nil || r.sessions == nil {
		return BindingScope{}, fmt.Errorf("cloud: profile session repository is not configured")
	}
	binding, err := r.bindings.ResolveBinding(ctx, actor)
	if err != nil {
		return BindingScope{}, err
	}
	if binding.Status != BindingBound {
		return BindingScope{}, profileScopeMismatch(actor.Normalize(), binding)
	}
	return binding.Scope()
}

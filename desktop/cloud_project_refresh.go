package main

import (
	"context"

	"kwu/internal/project"
	"kwu/internal/userctx"
)

// Routine account refresh does not invalidate an already authorized binding.
// Login, logout and identity changes still use beginProjectBootstrap directly.
func (r *cloudRuntime) beginProjectRefresh(ctx context.Context, actor userctx.ActorContext) uint64 {
	if r == nil || r.bindings == nil {
		return r.beginProjectBootstrap(actor)
	}
	resolver := cloudProjectScopeResolver{bindings: r.bindings, environment: r.projectEnvironment, actor: actor}
	scope, err := resolver.CurrentProjectScope(ctx)
	if err != nil {
		return r.beginProjectBootstrap(actor)
	}
	return r.beginProjectBootstrapForScope(actor, scope)
}

func (r *cloudRuntime) projectServicesReusableLocked(actor userctx.ActorContext, scope project.Scope) bool {
	s := r.projects
	return !r.closed.Load() && scope.Key != "" && r.projectCapability == project.CapabilityAvailable &&
		s != nil && s.Gateway != nil && s.Cache != nil && s.Bindings != nil && s.Journal != nil &&
		s.lifecycle != nil && s.lifecycle.Err() == nil && s.Scope == scope &&
		sameDesktopActorProfile(s.Actor, actor)
}

// Reuse the whole bundle, including directory ordering and command replay state.
// An incompatible capability update must cancel old operations before rebuilding.
func (r *cloudRuntime) reuseProjectServices(epoch uint64, actor userctx.ActorContext, scope project.Scope, stateRoot string, features project.CapabilityFeatures) bool {
	r.projectMu.Lock()
	defer r.projectMu.Unlock()
	current := r.projectReadiness.Load()
	if current == nil || current.Epoch != epoch {
		return false
	}
	if !r.projectServicesReusableLocked(actor, scope) || r.projectFeatures != features || r.projects.StateRoot != stateRoot {
		r.replaceProjectServicesLocked(project.CapabilityUnavailable, nil)
		next := *current
		next.Phase = cloudProjectReadinessPending
		next.Refreshing = false
		next.Reason = "cloud project services are initializing"
		r.projectReadiness.Store(&next)
		return false
	}
	next := *current
	next.Phase = cloudProjectReadinessAvailable
	next.Refreshing = false
	next.Reason = ""
	r.projectReadiness.Store(&next)
	return true
}

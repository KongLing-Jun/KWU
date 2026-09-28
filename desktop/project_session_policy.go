package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"kwu/internal/agent"
	"kwu/internal/cloud"
	"kwu/internal/project"
	"kwu/internal/userctx"
)

const (
	projectSessionReasonScope     = "scope_mismatch"
	projectSessionReasonOffline   = "status_unverifiable"
	projectSessionReasonArchived  = "project_archived"
	projectSessionReasonPermanent = "account_removed"
)

type projectSessionPolicyError struct{ reason string }

func (e *projectSessionPolicyError) Error() string {
	switch e.reason {
	case projectSessionReasonScope:
		return "project session is outside its original cloud scope"
	case projectSessionReasonArchived:
		return "archived project session is read only"
	case projectSessionReasonPermanent:
		return "device-local project transcript is permanently read only"
	default:
		return "project status cannot be verified; session is read only"
	}
}

func (a *App) authorizeCloudProjectSessionSend(ctx context.Context, actor userctx.ActorContext, sessionPath string) error {
	meta, bound, err := loadCloudProjectSessionMeta(sessionPath)
	if err != nil {
		return err
	}
	if !bound {
		return nil
	}
	if meta.ProjectContextPermanentlyReadOnly {
		return &projectSessionPolicyError{reason: projectSessionReasonPermanent}
	}
	current, err := a.currentProjectScopeForActor(ctx, actor)
	if err != nil || !projectContextPinMatchesScope(*meta.ProjectContext, current) {
		return &projectSessionPolicyError{reason: projectSessionReasonScope}
	}
	services, err := a.projectServicesForOperation()
	if err != nil || services.Actor.Normalize().ProfileID != actor.Normalize().ProfileID || services.Scope.Key != current.Key {
		return &projectSessionPolicyError{reason: projectSessionReasonOffline}
	}
	operationCtx, cancel := services.operationContext(ctx)
	defer cancel()
	result, err := services.Gateway.GetProject(operationCtx, meta.ProjectContext.ProjectID)
	if err != nil {
		return &projectSessionPolicyError{reason: projectSessionReasonOffline}
	}
	if result.Project.Status != project.ProjectActive {
		return &projectSessionPolicyError{reason: projectSessionReasonArchived}
	}
	return nil
}

func loadCloudProjectSessionMeta(path string) (agent.BranchMeta, bool, error) {
	if strings.TrimSpace(path) == "" {
		return agent.BranchMeta{}, false, nil
	}
	meta, ok, err := agent.LoadBranchMeta(path)
	if err != nil {
		return agent.BranchMeta{}, true, &projectSessionPolicyError{reason: projectSessionReasonOffline}
	}
	if !ok || meta.ProjectContext == nil {
		return meta, false, nil
	}
	if err := meta.ValidateProjectContextPin(); err != nil {
		return agent.BranchMeta{}, true, &projectSessionPolicyError{reason: projectSessionReasonOffline}
	}
	return meta, true, nil
}

func (a *App) currentProjectScopeForActor(ctx context.Context, actor userctx.ActorContext) (project.Scope, error) {
	if a == nil || a.cloudRuntime == nil || a.cloudRuntime.bindings == nil {
		return project.Scope{}, errors.New("cloud scope unavailable")
	}
	actor = actor.Normalize()
	binding, err := a.cloudRuntime.bindings.ResolveBinding(ctx, actor)
	if err != nil || binding.Status != cloud.BindingBound {
		return project.Scope{}, errors.New("cloud scope unavailable")
	}
	scope, err := binding.Scope()
	if err != nil {
		return project.Scope{}, err
	}
	return project.NewScope(a.cloudRuntime.projectEnvironment, scope)
}

// projectSessionVisible permits ordinary list/body access only in the original
// exact scope. Explicitly retained device-local transcripts remain visible to
// their original local profile but are permanently read only.
func (a *App) projectSessionVisible(ctx context.Context, actor userctx.ActorContext, path string) (visible, readOnly bool, reason string) {
	meta, bound, err := loadCloudProjectSessionMeta(path)
	if err != nil {
		return false, true, projectSessionReasonOffline
	}
	if !bound {
		return true, false, ""
	}
	actor = actor.Normalize()
	if meta.ProjectContextPermanentlyReadOnly {
		if meta.ProjectContext.BindingScope.ProfileID == actor.ProfileID {
			return true, true, projectSessionReasonPermanent
		}
		return false, true, projectSessionReasonScope
	}
	current, err := a.currentProjectScopeForActor(ctx, actor)
	if err != nil || !projectContextPinMatchesScope(*meta.ProjectContext, current) {
		return false, true, projectSessionReasonScope
	}
	if err := a.authorizeCloudProjectSessionSend(ctx, actor, path); err != nil {
		var policyErr *projectSessionPolicyError
		if errors.As(err, &policyErr) {
			return true, true, policyErr.reason
		}
		return true, true, projectSessionReasonOffline
	}
	return true, false, ""
}

func (a *App) projectSessionListVisible(ctx context.Context, actor userctx.ActorContext, path string) (visible, readOnly bool, reason string) {
	meta, bound, err := loadCloudProjectSessionMeta(path)
	if err != nil {
		return false, true, projectSessionReasonOffline
	}
	if !bound {
		return true, false, ""
	}
	actor = actor.Normalize()
	if meta.ProjectContextPermanentlyReadOnly {
		return meta.ProjectContext.BindingScope.ProfileID == actor.ProfileID, true, projectSessionReasonPermanent
	}
	current, err := a.currentProjectScopeForActor(ctx, actor)
	if err != nil || !projectContextPinMatchesScope(*meta.ProjectContext, current) {
		return false, true, projectSessionReasonScope
	}
	// Listing is local and bounded: do not issue one network read per row. The
	// resume/send boundaries perform the authoritative active-status check.
	return true, false, ""
}

// closeCloudProjectTabsOutsideScope removes only the in-memory/runtime views
// whose immutable project pin is no longer authorized. It intentionally leaves
// transcript and sidecar files untouched so the original account can restore
// them after signing in again.
func (a *App) closeCloudProjectTabsOutsideScope(actor userctx.ActorContext, scope *project.Scope) bool {
	if a == nil {
		return false
	}
	actor = actor.Normalize()
	type candidate struct {
		id          string
		detachedKey string
		tab         *WorkspaceTab
		sessionPath string
	}
	shouldHide := func(tab *WorkspaceTab, path string) bool {
		if tab == nil || tabRuntimeActorContext(tab).Normalize().ProfileID != actor.ProfileID {
			return false
		}
		meta, ok, err := agent.LoadBranchMeta(path)
		if err != nil || !ok || !meta.IsCloudProjectBound() {
			return false
		}
		return scope == nil || !projectContextPinMatchesScope(*meta.ProjectContext, *scope)
	}

	a.mu.RLock()
	visible := make([]candidate, 0)
	detached := make([]candidate, 0)
	for id, tab := range a.tabRuntimes {
		path := sessionRuntimeKey(tab.currentSessionPath())
		if path != "" {
			visible = append(visible, candidate{id: id, tab: tab, sessionPath: path})
		}
	}
	for key, tab := range a.detachedSessions {
		path := sessionRuntimeKey(tab.currentSessionPath())
		if path != "" {
			detached = append(detached, candidate{detachedKey: key, tab: tab, sessionPath: path})
		}
	}
	activeID := a.cachedActiveTabID()
	a.mu.RUnlock()

	removeVisible := make([]candidate, 0)
	removeDetached := make([]candidate, 0)
	removeActive := false
	for _, item := range visible {
		if shouldHide(item.tab, item.sessionPath) {
			removeVisible = append(removeVisible, item)
			removeActive = removeActive || item.id == activeID
		}
	}
	for _, item := range detached {
		if shouldHide(item.tab, item.sessionPath) {
			removeDetached = append(removeDetached, item)
		}
	}
	if len(removeVisible) == 0 && len(removeDetached) == 0 {
		return false
	}

	var fallback *WorkspaceTab
	if removeActive {
		fallback = a.createTabEntryWithIDAndActor("global", globalTabWorkspaceRoot(), "", newTabID(), actor)
		fallback.TopicTitle = "Global"
		fallback.sink = &tabEventSink{tabID: fallback.ID, app: a, ctx: a.ctx}
	}
	for _, item := range append(append([]candidate{}, removeVisible...), removeDetached...) {
		if item.tab.Ctrl != nil && !item.tab.ReadOnly {
			if err := item.tab.Ctrl.Snapshot(); err != nil {
				slog.Warn("desktop: snapshot cloud project tab before account isolation failed", "tab", item.tab.ID, "err", err)
			}
		}
	}

	removed := make([]*WorkspaceTab, 0, len(removeVisible)+len(removeDetached))
	fallbackInserted := false
	a.mu.Lock()
	for _, item := range removeVisible {
		current := a.tabRuntimes[item.id]
		if current != item.tab || sessionRuntimeKey(current.currentSessionPath()) != item.sessionPath {
			continue
		}
		a.markTabRemovedLocked(current)
		delete(a.tabRuntimes, item.id)
		a.removeTabOrderLocked(item.id)
		if a.cachedActiveTabID() == item.id {
			a.setActiveTabIDLocked("")
		}
		removed = append(removed, current)
	}
	for _, item := range removeDetached {
		current := a.detachedSessions[item.detachedKey]
		if current != item.tab || sessionRuntimeKey(current.currentSessionPath()) != item.sessionPath {
			continue
		}
		a.markTabRemovedLocked(current)
		delete(a.detachedSessions, item.detachedKey)
		removed = append(removed, current)
	}
	if a.cachedActiveTabID() == "" && fallback != nil {
		a.tabRuntimes[fallback.ID] = fallback
		a.tabOrder = append(a.tabOrder, fallback.ID)
		a.setActiveTabIDLocked(fallback.ID)
		fallbackInserted = true
	}
	dir, entries, savedActiveID, version := a.saveTabsCollectLocked()
	a.mu.Unlock()
	if len(removed) == 0 {
		return false
	}
	a.saveTabsWrite(dir, entries, savedActiveID, version)
	for _, tab := range removed {
		a.closeTabRuntime(tab)
	}
	if fallbackInserted {
		a.startTabControllerBuild(fallback)
	}
	return true
}

func (a *App) activeProjectPolicyActor() userctx.ActorContext {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if tab := a.activeTabLocked(); tab != nil {
		return tabRuntimeActorContext(tab).Normalize()
	}
	return userctx.ActorContext{}.Normalize()
}

func (a *App) authorizeProjectSessionMutation(path string, allowPermanentDelete bool) error {
	meta, bound, err := loadCloudProjectSessionMeta(path)
	if err != nil {
		return err
	}
	if !bound {
		return nil
	}
	actor := a.activeProjectPolicyActor()
	if meta.ProjectContextPermanentlyReadOnly {
		if allowPermanentDelete && meta.ProjectContext.BindingScope.ProfileID == actor.ProfileID {
			return nil
		}
		return &projectSessionPolicyError{reason: projectSessionReasonPermanent}
	}
	if allowPermanentDelete {
		current, err := a.currentProjectScopeForActor(a.bootContext(), actor)
		if err != nil || !projectContextPinMatchesScope(*meta.ProjectContext, current) {
			return &projectSessionPolicyError{reason: projectSessionReasonScope}
		}
		return nil
	}
	return a.authorizeCloudProjectSessionSend(a.bootContext(), actor, path)
}

// SetCloudProjectSessionPinned stores a device-local ordering preference only
// after the session's immutable Cloud scope matches the current account.
func (a *App) SetCloudProjectSessionPinned(path string, pinned bool) error {
	dir, sessionPath, err := a.sessionDirForPath(path)
	if err != nil {
		return err
	}
	if _, _, err := validateSessionPath(dir, sessionPath); err != nil {
		return err
	}
	meta, bound, err := loadCloudProjectSessionMeta(sessionPath)
	if err != nil {
		return err
	}
	if !bound {
		return fmt.Errorf("session is not bound to a cloud project")
	}
	currentScope, err := a.currentProjectScopeForActor(a.bootContext(), a.activeProjectPolicyActor())
	if err != nil || !projectContextPinMatchesScope(*meta.ProjectContext, currentScope) {
		return &projectSessionPolicyError{reason: projectSessionReasonScope}
	}
	updated, err := agent.MutateExistingBranchMeta(sessionPath, false, func(current *agent.BranchMeta) error {
		if current.ProjectContext == nil || *current.ProjectContext != *meta.ProjectContext {
			return fmt.Errorf("project session identity changed while updating pin")
		}
		current.ProjectSessionPinned = pinned
		return nil
	})
	if err != nil {
		return err
	}
	if !updated {
		return fmt.Errorf("project session is unavailable")
	}
	a.emitProjectTreeChanged()
	return nil
}

// retainProjectSessionAfterConfirmedAccountDeletion is intentionally not a
// Wails method. Only a trusted deletion lifecycle may call it with the complete
// Cloud identity it has just confirmed deleted.
func (a *App) retainProjectSessionAfterConfirmedAccountDeletion(path string, deleted agent.ProjectContextPin) error {
	if err := deleted.Validate(); err != nil {
		return fmt.Errorf("deleted project identity is invalid")
	}
	meta, bound, err := loadCloudProjectSessionMeta(path)
	if err != nil || !bound {
		return fmt.Errorf("project session is unavailable")
	}
	if *meta.ProjectContext != deleted {
		return fmt.Errorf("deleted project identity does not match the transcript")
	}
	actor := userctx.ActorContext{ActorID: userctx.DefaultActorID, ProfileID: deleted.BindingScope.ProfileID}
	if current, currentErr := a.currentProjectScopeForActor(a.bootContext(), actor); currentErr == nil && projectContextPinMatchesScope(deleted, current) {
		return fmt.Errorf("deleted project identity is still the current cloud scope")
	}
	updated, err := agent.MutateExistingBranchMeta(path, true, func(current *agent.BranchMeta) error {
		if current.ProjectContext == nil || *current.ProjectContext != deleted {
			return fmt.Errorf("deleted project identity does not match the transcript")
		}
		current.ProjectContextPermanentlyReadOnly = true
		return nil
	})
	if err != nil {
		return err
	}
	if !updated {
		return fmt.Errorf("project session is unavailable")
	}
	return nil
}

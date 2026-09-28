package project

import (
	"context"
	"fmt"
	"time"
)

// StoreSummaries atomically replaces the cached server-ordered summary page.
func (c *ProjectCache) StoreSummaries(ctx context.Context, scope Scope, page ProjectPage) error {
	return c.storeDirectory(ctx, scope, page, false)
}

// StoreJoinedSummaries atomically replaces the cached server-ordered member
// project page without changing the owned-project directory.
func (c *ProjectCache) StoreJoinedSummaries(ctx context.Context, scope Scope, page ProjectPage) error {
	return c.storeDirectory(ctx, scope, page, true)
}

func (c *ProjectCache) storeDirectory(ctx context.Context, scope Scope, page ProjectPage, joined bool) error {
	if err := c.validateCurrentScope(ctx, scope); err != nil {
		return err
	}
	if page.scopeKey != scope.Key {
		label := "project"
		if joined {
			label = "joined project"
		}
		return fmt.Errorf("projects-v1: %s summary page belongs to another scope", label)
	}
	if err := validateCachePage(page, scope); err != nil {
		return err
	}
	path := c.path(scope)
	release := acquireProjectCacheLock(path)
	defer release()
	document, found, err := c.loadDocumentLocked(path, scope, true, true)
	if err != nil {
		return err
	}
	if !found {
		document = c.newDocument(scope)
	}
	items := page.Items
	if len(items) > projectCacheMaxSummaries {
		items = items[:projectCacheMaxSummaries]
	}
	wirePage := projectCachePageWire{Items: cacheProjectWires(items), NextCursor: cloneString(page.NextCursor)}
	if joined {
		document.JoinedSummaries = wirePage
		document.JoinedRefreshed = true
	} else {
		document.Summaries = wirePage
		document.OwnedRefreshed = true
	}
	removeUnauthorizedProjectDetails(&document, scope, !joined)
	document.CachedAt = c.now().UTC()
	return c.commitLocked(ctx, path, scope, document)
}

func removeUnauthorizedProjectDetails(document *projectCacheDocument, scope Scope, owned bool) {
	authorized := make(map[string]struct{})
	for _, item := range document.Summaries.Items {
		authorized[item.ProjectID] = struct{}{}
	}
	for _, item := range document.JoinedSummaries.Items {
		authorized[item.ProjectID] = struct{}{}
	}
	details := document.Details[:0]
	for _, detail := range document.Details {
		isOwned := detail.Project.OwnerUserID == scope.Binding.CloudUserID
		_, stillAuthorized := authorized[detail.Project.ProjectID]
		if isOwned != owned || stillAuthorized {
			details = append(details, detail)
		}
	}
	document.Details = append([]projectCacheDetailWire{}, details...)
}

// StoreDetail atomically inserts or refreshes one detail in the 50-entry LRU.
func (c *ProjectCache) StoreDetail(ctx context.Context, scope Scope, project Project) error {
	if err := c.validateCurrentScope(ctx, scope); err != nil {
		return err
	}
	if err := validateCacheProject(project, scope); err != nil {
		return err
	}
	path := c.path(scope)
	release := acquireProjectCacheLock(path)
	defer release()
	document, found, err := c.loadDocumentLocked(path, scope, true, true)
	if err != nil {
		return err
	}
	if !found {
		document = c.newDocument(scope)
	}
	now := c.now().UTC()
	storeProjectCacheDetail(&document, project, now)
	document.CachedAt = now
	return c.commitLocked(ctx, path, scope, document)
}

// StoreCreatedProject atomically publishes an authoritative create readback at
// the head of the cached directory and refreshes its detail entry.
func (c *ProjectCache) StoreCreatedProject(ctx context.Context, scope Scope, project Project) error {
	if err := c.validateCurrentScope(ctx, scope); err != nil {
		return err
	}
	if err := validateCacheProject(project, scope); err != nil {
		return err
	}
	path := c.path(scope)
	release := acquireProjectCacheLock(path)
	defer release()
	document, found, err := c.loadDocumentLocked(path, scope, true, true)
	if err != nil {
		return err
	}
	if !found {
		document = c.newDocument(scope)
	}
	summaries := removeCachedProject(document.Summaries.Items, project.ProjectID)
	document.Summaries.Items = append([]projectCacheProjectWire{cacheProjectWire(project)}, summaries...)
	if len(document.Summaries.Items) > projectCacheMaxSummaries {
		document.Summaries.Items = document.Summaries.Items[:projectCacheMaxSummaries]
	}
	now := c.now().UTC()
	storeProjectCacheDetail(&document, project, now)
	document.CachedAt = now
	return c.commitLocked(ctx, path, scope, document)
}

func storeProjectCacheDetail(document *projectCacheDocument, project Project, now time.Time) {
	details := removeCachedProjectDetail(document.Details, project.ProjectID)
	details = append(details, projectCacheDetailWire{Project: cacheProjectWire(project), AccessedAt: now})
	if len(details) > projectCacheMaxDetails {
		details = details[len(details)-projectCacheMaxDetails:]
	}
	document.Details = append([]projectCacheDetailWire(nil), details...)
}

// RemoveProject atomically evicts one remotely deleted project from both the
// cached directory and detail LRU for the exact current Cloud scope.
func (c *ProjectCache) RemoveProject(ctx context.Context, scope Scope, projectID string) error {
	if err := c.validateCurrentScope(ctx, scope); err != nil {
		return err
	}
	if err := validateOpaqueProjectID(projectID); err != nil {
		return err
	}
	path := c.path(scope)
	release := acquireProjectCacheLock(path)
	defer release()
	document, found, err := c.loadDocumentLocked(path, scope, true, true)
	if err != nil || !found {
		return err
	}
	document.Summaries.Items = append([]projectCacheProjectWire{}, removeCachedProject(document.Summaries.Items, projectID)...)
	document.JoinedSummaries.Items = append([]projectCacheProjectWire{}, removeCachedProject(document.JoinedSummaries.Items, projectID)...)
	document.Details = append([]projectCacheDetailWire{}, removeCachedProjectDetail(document.Details, projectID)...)
	document.CachedAt = c.now().UTC()
	return c.commitLocked(ctx, path, scope, document)
}

func removeCachedProject(items []projectCacheProjectWire, projectID string) []projectCacheProjectWire {
	kept := items[:0]
	for _, item := range items {
		if item.ProjectID != projectID {
			kept = append(kept, item)
		}
	}
	return kept
}

func removeCachedProjectDetail(items []projectCacheDetailWire, projectID string) []projectCacheDetailWire {
	kept := items[:0]
	for _, item := range items {
		if item.Project.ProjectID != projectID {
			kept = append(kept, item)
		}
	}
	return kept
}

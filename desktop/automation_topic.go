package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"kwu/internal/automation"
	"kwu/internal/modelbilling"
)

func (a *App) bindAutomationTaskTopic(task automation.Task, routes ...modelbilling.BillingRouteSelection) (automation.Task, TabMeta, error) {
	var route *modelbilling.BillingRouteSelection
	if len(routes) > 0 {
		candidate := routes[0]
		route = &candidate
	}
	scope, workspaceRoot := automationTopicScope(task)
	cloudProjectID := strings.TrimSpace(task.ProjectID)
	topicID := strings.TrimSpace(task.TopicID)
	if task.NewConversationEachRun {
		topicID = ""
	}
	if topicID == "" {
		// A cloud automation Topic is not publishable under its physical
		// execution scope. Its tab receives a pending logical project identity
		// below, before the first project-tree refresh can observe it.
		meta, err := a.createTopic(scope, workspaceRoot, automationTopicTitle(task), cloudProjectID == "")
		if err != nil {
			return task, TabMeta{}, err
		}
		topicID = meta.ID
		if !task.NewConversationEachRun {
			task.TopicID = topicID
			if err := a.persistAutomationTaskTopic(task); err != nil {
				return task, TabMeta{}, err
			}
		}
	}

	task.Scope = scope
	task.WorkspaceRoot = workspaceRoot
	task.TopicID = topicID
	if scope == "project" {
		actor := a.activeConfigActorForWorkspace(workspaceRoot)
		tab, err := a.openProjectTabInactiveWithActorAndRoute(workspaceRoot, topicID, actor, route, cloudProjectID)
		if err != nil {
			return task, TabMeta{}, err
		}
		if cloudProjectID != "" {
			a.emitProjectTreeChanged()
		}
		return task, tab, nil
	}
	actor := a.activeConfigActorForWorkspace("")
	tab, err := a.openGlobalTabInactiveWithActorAndRoute(topicID, actor, route, cloudProjectID)
	if err != nil {
		return task, TabMeta{}, err
	}
	if cloudProjectID != "" {
		a.emitProjectTreeChanged()
	}
	return task, tab, nil
}

func (a *App) clearPendingAutomationCloudProject(tabID, projectID string) {
	projectID = strings.TrimSpace(projectID)
	if a == nil || projectID == "" {
		return
	}
	a.mu.Lock()
	tab := a.tabByIDLocked(tabID)
	changed := tab != nil && strings.TrimSpace(tab.pendingCloudProjectID) == projectID
	if changed {
		tab.pendingCloudProjectID = ""
	}
	a.mu.Unlock()
	if changed {
		a.emitProjectTreeChanged()
	}
}

func automationTopicScope(task automation.Task) (string, string) {
	projectID := strings.TrimSpace(task.ProjectID)
	if projectID != "" || strings.TrimSpace(task.Scope) == "project" && strings.TrimSpace(task.WorkspaceRoot) != "" {
		workspaceRoot := strings.TrimSpace(task.WorkspaceRoot)
		if workspaceRoot != "" {
			if abs, err := filepath.Abs(workspaceRoot); err == nil {
				workspaceRoot = abs
			}
		}
		return "project", workspaceRoot
	}
	return "global", ""
}

func automationTopicTitle(task automation.Task) string {
	if name := strings.TrimSpace(task.Name); name != "" {
		return name
	}
	return "自动化任务"
}

func (a *App) persistAutomationTaskTopic(task automation.Task) error {
	taskID := strings.TrimSpace(task.ID)
	topicID := strings.TrimSpace(task.TopicID)
	if taskID == "" || topicID == "" {
		return nil
	}
	svc, _ := a.ensureAutomationService()
	ctx := a.bootContext()
	views, err := svc.ListTasks(ctx)
	if err != nil {
		return err
	}
	for _, view := range views {
		stored := view.Task
		if stored.ID != taskID {
			continue
		}
		stored, err = a.normalizeManagedAutomationTaskTarget(stored)
		if err != nil {
			return err
		}
		stored.TopicID = topicID
		_, err := svc.UpdateTask(ctx, automation.UpdateTaskRequest{Task: stored, ExpectedRevision: stored.Revision})
		return err
	}
	return fmt.Errorf("automation task %q not found", taskID)
}

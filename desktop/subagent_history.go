package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"kwu/internal/agent"
	"kwu/internal/control"
	"kwu/internal/expert"
)

type ExpertTeamSessionView struct {
	State   string                        `json:"state"`
	ID      string                        `json:"id"`
	Name    string                        `json:"name"`
	Lead    ExpertTeamSessionMemberView   `json:"lead"`
	Members []ExpertTeamSessionMemberView `json:"members"`
}

type ExpertTeamSessionMemberView struct {
	MemberKey   string `json:"memberKey"`
	ExpertID    string `json:"expertId,omitempty"`
	Name        string `json:"name"`
	Role        string `json:"role,omitempty"`
	AvatarKey   string `json:"avatarKey,omitempty"`
	SubagentRef string `json:"subagentRef,omitempty"`
	Status      string `json:"status,omitempty"`
}

type expertTeamMemberProjection struct {
	view        ExpertTeamSessionMemberView
	createdAt   time.Time
	rosterIndex int
}

// ExpertTeamForTab restores the display-only team snapshot from the parent
// sidecar. A ref is exposed only when its child metadata confirms parent ownership.
func (a *App) ExpertTeamForTab(tabID string) (*ExpertTeamSessionView, error) {
	_, sessionDir, sessionPath, err := a.subagentOwnerForTab(tabID)
	if err != nil {
		return nil, err
	}
	meta, ok, err := agent.LoadSessionExpert(sessionPath)
	if err != nil {
		return nil, err
	}
	if !ok || (meta.TeamState != agent.ExpertTeamActive && meta.TeamState != agent.ExpertTeamArchived) {
		return nil, nil
	}
	cloudDefinition := cloudExpertTeamDefinition(a.cloudExpertDefinitionsSnapshot(), meta.ExpertID)
	artifacts, err := agent.ListSubagentsByParent(sessionDir, agent.BranchID(sessionPath))
	if err != nil {
		return nil, err
	}
	owned := make(map[string]agent.SubagentMeta, len(artifacts))
	for _, artifact := range artifacts {
		owned[artifact.Ref] = artifact.Meta
	}
	view := &ExpertTeamSessionView{
		State: meta.TeamState, ID: meta.ExpertID, Name: meta.ExpertName,
		Members: make([]ExpertTeamSessionMemberView, 0),
	}
	if meta.Lead != nil {
		view.Lead = ExpertTeamSessionMemberView{
			MemberKey: meta.Lead.MemberKey, ExpertID: meta.Lead.ExpertID, Name: meta.Lead.Name,
			Role: meta.Lead.Role, AvatarKey: authoritativeExpertTeamAvatar(*meta.Lead, cloudDefinition),
		}
	}
	members := make([]expertTeamMemberProjection, 0, len(meta.Members))
	for rosterIndex, member := range meta.Members {
		child, exists := owned[strings.TrimSpace(member.SubagentRef)]
		if !exists {
			continue
		}
		item := ExpertTeamSessionMemberView{
			MemberKey: member.MemberKey, ExpertID: member.ExpertID, Name: member.Name,
			Role: member.Role, AvatarKey: authoritativeExpertTeamAvatar(member, cloudDefinition), SubagentRef: child.Ref, Status: string(child.Status),
		}
		members = append(members, expertTeamMemberProjection{
			view: item, createdAt: child.CreatedAt, rosterIndex: rosterIndex,
		})
	}
	sort.SliceStable(members, func(i, j int) bool {
		left, right := members[i], members[j]
		if left.createdAt.IsZero() != right.createdAt.IsZero() {
			return !left.createdAt.IsZero()
		}
		if !left.createdAt.IsZero() && !left.createdAt.Equal(right.createdAt) {
			return left.createdAt.Before(right.createdAt)
		}
		return left.rosterIndex < right.rosterIndex
	})
	for _, member := range members {
		view.Members = append(view.Members, member.view)
	}
	return view, nil
}

func cloudExpertTeamDefinition(definitions []expert.Definition, id string) *expert.Definition {
	id = strings.TrimSpace(id)
	for index := range definitions {
		if definitions[index].Mode == expert.ModeTeam && definitions[index].ID == id {
			return &definitions[index]
		}
	}
	return nil
}

func authoritativeExpertTeamAvatar(member agent.ExpertTeamMemberMeta, definition *expert.Definition) string {
	if definition == nil {
		return member.AvatarKey
	}
	for index, candidate := range definition.Members {
		memberKey := fmt.Sprintf("member:%d", index)
		if expertID := strings.TrimSpace(candidate.ExpertID); expertID != "" {
			memberKey = "expert:" + expertID
		}
		if (member.ExpertID != "" && member.ExpertID == candidate.ExpertID) ||
			member.MemberKey == memberKey ||
			strings.EqualFold(strings.TrimSpace(member.Name), strings.TrimSpace(candidate.Name)) {
			return strings.TrimSpace(candidate.AvatarKey)
		}
	}
	return member.AvatarKey
}

// SubagentToolResultForTab loads archived child tool data by raw child tool ID.
// It never falls back to the parent transcript.
func (a *App) SubagentToolResultForTab(tabID, ref, toolID string) (*control.ToolResultData, error) {
	artifact, _, _, err := a.subagentArtifactForTab(tabID, ref)
	if err != nil {
		return nil, err
	}
	toolID = strings.TrimSpace(toolID)
	if toolID == "" {
		return nil, fmt.Errorf("empty child tool id")
	}
	loaded, err := agent.LoadSession(artifact.SessionPath)
	if err != nil {
		return nil, err
	}
	result := control.ToolResultFromMessages(loaded.Snapshot(), toolID)
	if result == nil {
		return nil, fmt.Errorf("child tool result not found")
	}
	return result, nil
}

func (a *App) subagentArtifactForTab(tabID, ref string) (agent.SubagentArtifact, string, string, error) {
	parentSession, sessionDir, sessionPath, err := a.subagentOwnerForTab(tabID)
	if err != nil {
		return agent.SubagentArtifact{}, "", "", err
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return agent.SubagentArtifact{}, "", "", fmt.Errorf("empty subagent reference")
	}
	artifacts, err := agent.ListSubagentsByParent(sessionDir, parentSession)
	if err != nil {
		return agent.SubagentArtifact{}, "", "", err
	}
	for _, artifact := range artifacts {
		if artifact.Ref == ref {
			return artifact, sessionDir, sessionPath, nil
		}
	}
	return agent.SubagentArtifact{}, "", "", fmt.Errorf("subagent reference not found")
}

func (a *App) subagentOwnerForTab(tabID string) (string, string, string, error) {
	a.mu.RLock()
	tab := a.tabByIDLocked(tabID)
	if tab == nil {
		a.mu.RUnlock()
		return "", "", "", fmt.Errorf("tab not found")
	}
	sessionPath := strings.TrimSpace(tab.SessionPath)
	if tab.Ctrl != nil {
		sessionPath = strings.TrimSpace(tab.Ctrl.SessionPath())
	}
	a.mu.RUnlock()
	if visible, _, _ := a.projectSessionListVisible(a.bootContext(), tabRuntimeActorContext(tab), sessionPath); !visible {
		return "", "", "", fmt.Errorf("subagent history is unavailable in the current scope")
	}
	sessionDir, validPath, err := resolveTabSessionPath(tab, sessionPath)
	if err != nil {
		return "", "", "", err
	}
	parentSession := agent.BranchID(validPath)
	if parentSession == "" {
		return "", "", "", fmt.Errorf("tab has no persisted session")
	}
	return parentSession, filepath.Clean(sessionDir), validPath, nil
}

package control

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kwu/internal/agent"
	"kwu/internal/event"
	"kwu/internal/expert"
	"kwu/internal/provider"
)

var saveControllerExpertMeta = agent.SaveSessionExpert
var deleteControllerExpertMeta = agent.DeleteSessionExpert

func sessionWithSystemPrompt(messages []provider.Message, system string) *agent.Session {
	next := agent.NewSession(system)
	for _, msg := range messages {
		if msg.Role != provider.RoleSystem {
			next.Add(msg)
		}
	}
	return next
}

func (c *Controller) saveExpertMeta(path string) {
	if path == "" {
		return
	}
	if err := c.persistExpertMeta(path, false); err != nil {
		c.expertMetaNotice("save expert metadata failed: " + err.Error())
	}
}

func (c *Controller) persistExpertMeta(path string, markUsed bool) error {
	var messages []provider.Message
	if !markUsed {
		messages = c.sessionMessages()
	}
	c.expertMetaPersistenceMu.Lock()
	defer c.expertMetaPersistenceMu.Unlock()
	return c.saveExpertMetaLocked(path, messages, markUsed)
}

func (c *Controller) saveExpertMetaLocked(path string, messages []provider.Message, markUsed bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	selection := c.expert
	previous := cloneExpertMeta(c.expertMeta)
	if markUsed && (previous.TeamState != agent.ExpertTeamActive || previous.TeamUsage == agent.ExpertTeamUsed) {
		return nil
	}
	meta, metaErr := expertMetaForPersistence(selection, previous, markUsed)
	if metaErr != nil {
		return metaErr
	}
	if strings.TrimSpace(meta.ExpertID) == "" {
		return nil
	}
	meta, metaErr = persistRecoveredExpertMeta(path, messages, meta)
	if metaErr == nil {
		c.expertMeta = meta
	}
	return metaErr
}

func expertMetaForPersistence(selection expert.Selection, previous agent.ExpertMeta, markUsed bool) (agent.ExpertMeta, error) {
	if previous.TeamState == agent.ExpertTeamArchived && !selection.Selected {
		return previous, nil
	}
	meta, err := expertMetaForSelection(selection, previous, true)
	if markUsed && meta.TeamState == agent.ExpertTeamActive {
		meta.TeamUsage = agent.ExpertTeamUsed
	}
	return meta, err
}

func persistRecoveredExpertMeta(path string, messages []provider.Message, meta agent.ExpertMeta) (agent.ExpertMeta, error) {
	if _, _, err := agent.LoadSessionExpert(path); err != nil {
		return meta, err
	}
	if meta.TeamState == agent.ExpertTeamActive {
		var err error
		meta, _, err = recoverExpertTeamBindings(path, messages, meta)
		if err != nil {
			return meta, err
		}
	}
	return meta, saveControllerExpertMeta(path, meta)
}

func (c *Controller) sessionMessages() []provider.Message {
	if c == nil || c.executor == nil || c.executor.Session() == nil {
		return nil
	}
	return c.executor.Session().Snapshot()
}

func (c *Controller) expertMetaNotice(text string) {
	c.sink.Emit(event.Event{Kind: event.Notice, Level: event.LevelWarn, Text: text})
}

func cloneExpertMeta(meta agent.ExpertMeta) agent.ExpertMeta {
	meta.ProjectExpertIDs = append([]string(nil), meta.ProjectExpertIDs...)
	if meta.Lead != nil {
		lead := *meta.Lead
		meta.Lead = &lead
	}
	meta.Members = append([]agent.ExpertTeamMemberMeta(nil), meta.Members...)
	return meta
}

func expertMetaForSelection(selection expert.Selection, previous agent.ExpertMeta, preserveBindings bool) (agent.ExpertMeta, error) {
	if !selection.Selected {
		return agent.ExpertMeta{}, nil
	}
	meta := agent.ExpertMeta{ExpertID: selection.ID, ExpertName: selection.Name, Source: selection.Source}
	if previous.ExpertID == selection.ID {
		meta.ProjectExpertIDs = append([]string(nil), previous.ProjectExpertIDs...)
	}
	if selection.Expert.Mode != expert.ModeTeam {
		return meta, nil
	}
	return expertTeamMetaForSelection(meta, selection, previous, preserveBindings)
}

func expertTeamMetaForSelection(meta agent.ExpertMeta, selection expert.Selection, previous agent.ExpertMeta, preserveBindings bool) (agent.ExpertMeta, error) {
	meta.TeamState = agent.ExpertTeamActive
	meta.TeamUsage = agent.ExpertTeamUnused
	if preserveBindings && previous.TeamState == agent.ExpertTeamActive && previous.ExpertID == selection.ID {
		meta.TeamBindingsReset = previous.TeamBindingsReset
		meta.TeamUsage = previous.TeamUsage
	}
	refs := priorExpertTeamRefs(selection, previous, preserveBindings)
	seenNames := make(map[string]struct{}, len(selection.Expert.Members))
	seenKeys := make(map[string]struct{}, len(selection.Expert.Members))
	for index, member := range selection.Expert.Members {
		if member.Kind == expert.MemberKindLead {
			lead, err := expertTeamMemberMeta(index, member, nil, seenNames, seenKeys)
			if err != nil {
				return agent.ExpertMeta{}, err
			}
			meta.Lead = &lead
			continue
		}
		if member.Kind != expert.MemberKindSub {
			continue
		}
		memberMeta, err := expertTeamMemberMeta(index, member, refs, seenNames, seenKeys)
		if err != nil {
			return agent.ExpertMeta{}, err
		}
		meta.Members = append(meta.Members, memberMeta)
	}
	if meta.Lead == nil {
		return agent.ExpertMeta{}, fmt.Errorf("expert team lead is required")
	}
	return meta, nil
}

func priorExpertTeamRefs(selection expert.Selection, previous agent.ExpertMeta, preserve bool) map[string]string {
	refs := make(map[string]string, len(previous.Members))
	if !preserve || previous.TeamState != agent.ExpertTeamActive || previous.ExpertID != selection.ID {
		return refs
	}
	for _, member := range previous.Members {
		refs[member.MemberKey] = member.SubagentRef
	}
	return refs
}

func expertTeamMemberMeta(index int, member expert.Member, refs map[string]string, seenNames, seenKeys map[string]struct{}) (agent.ExpertTeamMemberMeta, error) {
	name := strings.TrimSpace(member.Name)
	if name == "" {
		return agent.ExpertTeamMemberMeta{}, fmt.Errorf("expert team member %d name is required", index+1)
	}
	nameKey := strings.ToLower(name)
	if _, exists := seenNames[nameKey]; exists {
		return agent.ExpertTeamMemberMeta{}, fmt.Errorf("expert team member name %q is duplicated", name)
	}
	seenNames[nameKey] = struct{}{}
	memberKey := fmt.Sprintf("member:%d", index)
	if id := strings.TrimSpace(member.ExpertID); id != "" {
		memberKey = "expert:" + id
	}
	if _, exists := seenKeys[memberKey]; exists {
		return agent.ExpertTeamMemberMeta{}, fmt.Errorf("expert team member key %q is duplicated", memberKey)
	}
	seenKeys[memberKey] = struct{}{}
	return agent.ExpertTeamMemberMeta{
		MemberKey: memberKey, ExpertID: strings.TrimSpace(member.ExpertID), Name: name,
		Role: strings.TrimSpace(member.Role), AvatarKey: strings.TrimSpace(member.AvatarKey), SubagentRef: refs[memberKey],
	}, nil
}

type controllerExpertTeamRuntime struct{ c *Controller }

func (r controllerExpertTeamRuntime) AcquireExpertTeamMember(memberName string) (string, error) {
	if r.c == nil {
		return "", fmt.Errorf("expert-team runtime is unavailable")
	}
	r.c.mu.Lock()
	defer r.c.mu.Unlock()
	member, ok := expertTeamMemberByName(r.c.expertMeta, memberName)
	if r.c.expertMeta.TeamState != agent.ExpertTeamActive || !ok {
		return "", fmt.Errorf("expert-team member %q is not active in this session", memberName)
	}
	if r.c.expertTeamStarts == nil {
		r.c.expertTeamStarts = make(map[string]bool)
	}
	if r.c.expertTeamStarts[member.MemberKey] {
		return "", fmt.Errorf("expert-team member %q is already starting a task", member.Name)
	}
	r.c.expertTeamStarts[member.MemberKey] = true
	return member.SubagentRef, nil
}

func (r controllerExpertTeamRuntime) CommitExpertTeamMember(memberName, subagentRef string) error {
	if r.c == nil {
		return fmt.Errorf("expert-team runtime is unavailable")
	}
	ref := strings.TrimSpace(subagentRef)
	if ref == "" {
		return fmt.Errorf("subagent reference is required")
	}
	r.c.expertMetaPersistenceMu.Lock()
	defer r.c.expertMetaPersistenceMu.Unlock()
	path := r.c.SessionPath()
	r.c.mu.Lock()
	defer r.c.mu.Unlock()
	next, err := bindExpertTeamMember(r.c.expertMeta, memberName, ref)
	if err != nil {
		return err
	}
	if err := saveControllerExpertMeta(path, next); err != nil {
		if r.c.jobs != nil {
			r.c.jobs.KillControlledForSession(agent.BranchID(path), "task", ref)
		}
		return err
	}
	r.c.expertMeta = next
	return nil
}

func bindExpertTeamMember(meta agent.ExpertMeta, memberName, ref string) (agent.ExpertMeta, error) {
	if meta.TeamState != agent.ExpertTeamActive {
		return agent.ExpertMeta{}, fmt.Errorf("expert team is not active")
	}
	next := cloneExpertMeta(meta)
	for i := range next.Members {
		if strings.EqualFold(next.Members[i].Name, strings.TrimSpace(memberName)) {
			next.Members[i].SubagentRef = ref
			next.TeamUsage = agent.ExpertTeamUsed
			return next, nil
		}
	}
	return agent.ExpertMeta{}, fmt.Errorf("expert-team member %q is not configured", memberName)
}

func (r controllerExpertTeamRuntime) ReleaseExpertTeamMember(memberName string) {
	if r.c == nil {
		return
	}
	r.c.mu.Lock()
	if member, ok := expertTeamMemberByName(r.c.expertMeta, memberName); ok {
		delete(r.c.expertTeamStarts, member.MemberKey)
	}
	r.c.mu.Unlock()
}

func (r controllerExpertTeamRuntime) LookupExpertTeamMember(memberName string) (string, error) {
	if r.c == nil {
		return "", fmt.Errorf("expert-team runtime is unavailable")
	}
	r.c.mu.Lock()
	defer r.c.mu.Unlock()
	member, ok := expertTeamMemberByName(r.c.expertMeta, memberName)
	if r.c.expertMeta.TeamState != agent.ExpertTeamActive || !ok {
		return "", fmt.Errorf("expert-team member %q is not active in this session", memberName)
	}
	if strings.TrimSpace(member.SubagentRef) == "" {
		return "", fmt.Errorf("expert-team member %q has no task session yet", member.Name)
	}
	return member.SubagentRef, nil
}

func expertTeamMemberByName(meta agent.ExpertMeta, memberName string) (agent.ExpertTeamMemberMeta, bool) {
	name := strings.TrimSpace(memberName)
	for _, member := range meta.Members {
		if strings.EqualFold(member.Name, name) {
			return member, true
		}
	}
	return agent.ExpertTeamMemberMeta{}, false
}

func resetExpertTeamBindings(meta agent.ExpertMeta) agent.ExpertMeta {
	meta = cloneExpertMeta(meta)
	for i := range meta.Members {
		meta.Members[i].SubagentRef = ""
	}
	return meta
}

func (c *Controller) stopExpertTeamMembers(meta agent.ExpertMeta) {
	if c == nil || c.jobs == nil {
		return
	}
	parentSession := c.parentSessionID()
	for _, member := range meta.Members {
		ref := strings.TrimSpace(member.SubagentRef)
		if ref == "" || !c.jobs.KillControlledForSession(parentSession, "task", ref) {
			continue
		}
		_, _ = c.jobs.WaitControlledForSession(context.Background(), parentSession, "task", ref, 5)
	}
}

func (c *Controller) resetCurrentExpertTeamBindings() error {
	c.expertMetaPersistenceMu.Lock()
	defer c.expertMetaPersistenceMu.Unlock()
	path := c.SessionPath()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.expertMeta.TeamState != agent.ExpertTeamActive && c.expertMeta.TeamState != agent.ExpertTeamArchived {
		return nil
	}
	next := resetExpertTeamBindings(c.expertMeta)
	next.TeamBindingsReset = true
	if err := saveControllerExpertMeta(path, next); err != nil {
		return err
	}
	c.expertMeta = next
	c.expertTeamStarts = make(map[string]bool)
	return nil
}

func (c *Controller) saveForkExpertMeta(path string) error {
	c.expertMetaPersistenceMu.Lock()
	defer c.expertMetaPersistenceMu.Unlock()
	c.mu.Lock()
	meta := resetExpertTeamBindings(c.expertMeta)
	c.mu.Unlock()
	if strings.TrimSpace(meta.ExpertID) == "" {
		return nil
	}
	return saveControllerExpertMeta(path, meta)
}

func (c *Controller) saveForkExpertMetaWithCleanup(path, operation string) error {
	if err := c.saveForkExpertMeta(path); err != nil {
		result := fmt.Errorf("save %s expert metadata %s: %w", operation, path, err)
		if cleanupErr := removeForkSessionArtifacts(path); cleanupErr != nil {
			result = errors.Join(result, fmt.Errorf("cleanup %s session artifacts %s: %w", operation, path, cleanupErr))
		}
		return result
	}
	return nil
}

func (c *Controller) loadExpertMeta(path string) {
	if path == "" {
		return
	}
	meta, ok, err := agent.LoadSessionExpert(path)
	if err != nil {
		c.expertMetaNotice("load expert metadata failed: " + err.Error())
		return
	}
	if !ok {
		c.restoreNoExpertMeta()
		return
	}
	if meta.TeamState == agent.ExpertTeamArchived {
		c.restoreArchivedExpertTeam(meta)
		return
	}
	c.restoreSelectedExpert(path, meta)
}

func (c *Controller) restoreNoExpertMeta() {
	selection := expert.Selection{Source: expert.SourceNone}
	sysPrompt := ""
	if c.promptBuild != nil || c.scenePromptBuild != nil {
		var err error
		sysPrompt, selection, err = c.buildSystemPrompt(expert.ExplicitNoneID, "")
		if err != nil {
			c.expertMetaNotice("restore no-expert metadata failed: " + err.Error())
			return
		}
	}
	c.mu.Lock()
	if sysPrompt != "" {
		c.systemPrompt = sysPrompt
	}
	c.expert = selection
	c.expertMeta = agent.ExpertMeta{}
	c.expertTeamStarts = make(map[string]bool)
	c.mu.Unlock()
}

func (c *Controller) restoreArchivedExpertTeam(meta agent.ExpertMeta) {
	selection := expert.Selection{Source: expert.SourceNone}
	sysPrompt := ""
	if c.promptBuild != nil || c.scenePromptBuild != nil {
		var err error
		sysPrompt, selection, err = c.buildSystemPrompt(expert.ExplicitNoneID, "")
		if err != nil {
			c.expertMetaNotice("restore archived expert-team metadata failed: " + err.Error())
			return
		}
	}
	c.mu.Lock()
	if sysPrompt != "" {
		c.systemPrompt = sysPrompt
	}
	c.expert = selection
	c.expertMeta = cloneExpertMeta(meta)
	c.expertTeamStarts = make(map[string]bool)
	c.mu.Unlock()
	if sysPrompt != "" && c.executor != nil && c.executor.Session() != nil {
		c.executor.SetSession(sessionWithSystemPrompt(c.executor.Session().Snapshot(), sysPrompt))
	}
}

func (c *Controller) restoreSelectedExpert(path string, meta agent.ExpertMeta) {
	sysPrompt, selection, err := c.restoredExpertSelection(meta)
	if err != nil {
		prefix := "restore expert metadata failed: "
		if len(meta.ProjectExpertIDs) > 1 {
			prefix = "restore project expert team metadata failed: "
		}
		c.expertMetaNotice(prefix + err.Error())
		return
	}
	messages := c.sessionMessages()
	c.expertMetaPersistenceMu.Lock()
	defer c.expertMetaPersistenceMu.Unlock()
	restoredMeta := c.restoredSelectedExpertMeta(path, messages, selection, meta)
	c.mu.Lock()
	if sysPrompt != "" {
		c.systemPrompt = sysPrompt
	}
	c.expert = selection
	c.expertMeta = restoredMeta
	c.expertTeamStarts = make(map[string]bool)
	c.mu.Unlock()
}

func (c *Controller) restoredExpertSelection(meta agent.ExpertMeta) (string, expert.Selection, error) {
	selection := expert.Selection{ID: meta.ExpertID, Name: meta.ExpertName, Source: expert.SourceSession, Selected: true}
	if len(meta.ProjectExpertIDs) > 1 {
		return c.buildProjectExpertTeamPrompt(meta.ProjectExpertIDs, expert.SourceSession)
	}
	if c.promptBuild != nil || c.scenePromptBuild != nil {
		return c.buildSystemPrompt(meta.ExpertID, "")
	}
	return "", selection, nil
}

func (c *Controller) restoredSelectedExpertMeta(path string, messages []provider.Message, selection expert.Selection, meta agent.ExpertMeta) agent.ExpertMeta {
	restored := cloneExpertMeta(meta)
	if selection.Expert.Mode != expert.ModeTeam {
		return restored
	}
	next, changed, err := restoredExpertTeamMeta(path, messages, selection, meta)
	if err != nil {
		c.expertMetaNotice("restore expert-team metadata failed: " + err.Error())
		return restored
	}
	if changed {
		if err := saveControllerExpertMeta(path, next); err != nil {
			c.expertMetaNotice("restore expert-team metadata failed: " + err.Error())
			return restored
		}
	}
	return next
}

func restoredExpertTeamMeta(path string, messages []provider.Message, selection expert.Selection, previous agent.ExpertMeta) (agent.ExpertMeta, bool, error) {
	next := cloneExpertMeta(previous)
	changed := false
	if previous.TeamState != agent.ExpertTeamActive {
		var err error
		next, err = expertMetaForSelection(selection, previous, true)
		if err != nil {
			return agent.ExpertMeta{}, false, err
		}
		// Source describes how the expert was originally selected. Schema repair
		// must not turn an older request/config selection into a session selection.
		if strings.TrimSpace(previous.Source) != "" {
			next.Source = previous.Source
		}
		next.UpdatedAt = previous.UpdatedAt
		next.TeamUsage = previous.TeamUsage
		changed = true
	}
	recovered, bindingsChanged, err := recoverExpertTeamBindings(path, messages, next)
	if err != nil {
		return agent.ExpertMeta{}, false, err
	}
	return recovered, changed || bindingsChanged, nil
}

func recoverExpertTeamBindings(path string, messages []provider.Message, meta agent.ExpertMeta) (agent.ExpertMeta, bool, error) {
	if meta.TeamBindingsReset || meta.TeamState != agent.ExpertTeamActive || !expertTeamHasMissingBindings(meta) {
		return meta, false, nil
	}
	parentSession := agent.BranchID(path)
	if strings.TrimSpace(parentSession) == "" {
		return meta, false, nil
	}
	memberByCallID := expertTeamTaskMembers(messages)
	if len(memberByCallID) == 0 {
		return meta, false, nil
	}
	artifacts, err := agent.ListSubagentsByParent(filepath.Dir(path), parentSession)
	if err != nil {
		return agent.ExpertMeta{}, false, err
	}
	configured, usedRefs := expertTeamBindingSets(meta)
	candidates, err := expertTeamBindingCandidates(artifacts, parentSession, memberByCallID, configured)
	if err != nil {
		return agent.ExpertMeta{}, false, err
	}
	return applyRecoveredExpertTeamBindings(meta, candidates, usedRefs)
}

func expertTeamBindingSets(meta agent.ExpertMeta) (map[string]struct{}, map[string]struct{}) {
	configured := make(map[string]struct{}, len(meta.Members))
	usedRefs := make(map[string]struct{}, len(meta.Members))
	for _, member := range meta.Members {
		configured[strings.ToLower(strings.TrimSpace(member.Name))] = struct{}{}
		if ref := strings.TrimSpace(member.SubagentRef); ref != "" {
			usedRefs[ref] = struct{}{}
		}
	}
	return configured, usedRefs
}

func expertTeamBindingCandidates(artifacts []agent.SubagentArtifact, parentSession string, memberByCallID map[string]string, configured map[string]struct{}) (map[string]map[string]struct{}, error) {
	candidates := make(map[string]map[string]struct{})
	for _, artifact := range artifacts {
		memberKey, ok, err := expertTeamBindingCandidate(artifact, parentSession, memberByCallID, configured)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		refs := candidates[memberKey]
		if refs == nil {
			refs = make(map[string]struct{})
			candidates[memberKey] = refs
		}
		refs[artifact.Ref] = struct{}{}
	}
	return candidates, nil
}

func expertTeamBindingCandidate(artifact agent.SubagentArtifact, parentSession string, memberByCallID map[string]string, configured map[string]struct{}) (string, bool, error) {
	if strings.TrimSpace(artifact.Meta.ParentSession) != parentSession || strings.TrimSpace(artifact.Meta.Kind) != "task" {
		return "", false, nil
	}
	memberName := memberByCallID[strings.TrimSpace(artifact.Meta.ParentToolCallID)]
	memberKey := strings.ToLower(strings.TrimSpace(memberName))
	if _, ok := configured[memberKey]; memberKey == "" || !ok {
		return "", false, nil
	}
	info, err := os.Stat(artifact.SessionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return memberKey, info.Mode().IsRegular(), nil
}

func applyRecoveredExpertTeamBindings(meta agent.ExpertMeta, candidates map[string]map[string]struct{}, usedRefs map[string]struct{}) (agent.ExpertMeta, bool, error) {
	next := cloneExpertMeta(meta)
	changed := false
	for i := range next.Members {
		if strings.TrimSpace(next.Members[i].SubagentRef) != "" {
			continue
		}
		ref, ok := uniqueExpertTeamBinding(candidates[strings.ToLower(strings.TrimSpace(next.Members[i].Name))])
		if !ok {
			continue
		}
		if _, inUse := usedRefs[ref]; inUse {
			continue
		}
		next.Members[i].SubagentRef = ref
		usedRefs[ref] = struct{}{}
		changed = true
	}
	return next, changed, nil
}

func uniqueExpertTeamBinding(refs map[string]struct{}) (string, bool) {
	if len(refs) != 1 {
		return "", false
	}
	for ref := range refs {
		return ref, true
	}
	return "", false
}

func expertTeamHasMissingBindings(meta agent.ExpertMeta) bool {
	for _, member := range meta.Members {
		if strings.TrimSpace(member.SubagentRef) == "" {
			return true
		}
	}
	return false
}

func expertTeamTaskMembers(messages []provider.Message) map[string]string {
	members := make(map[string]string)
	ambiguous := make(map[string]struct{})
	for _, message := range messages {
		for _, call := range message.ToolCalls {
			callID, memberName, ok := expertTeamTaskMember(call)
			if !ok {
				continue
			}
			if existing, ok := members[callID]; ok && !strings.EqualFold(existing, memberName) {
				ambiguous[callID] = struct{}{}
				continue
			}
			members[callID] = memberName
		}
	}
	for callID := range ambiguous {
		delete(members, callID)
	}
	return members
}

func expertTeamTaskMember(call provider.ToolCall) (string, string, bool) {
	callID := strings.TrimSpace(call.ID)
	if strings.TrimSpace(call.Name) != "task" || callID == "" {
		return "", "", false
	}
	var args map[string]json.RawMessage
	if json.Unmarshal([]byte(call.Arguments), &args) != nil {
		return "", "", false
	}
	rawMember, explicitlyNamed := args["member_name"]
	if !explicitlyNamed {
		return "", "", false
	}
	var memberName string
	if json.Unmarshal(rawMember, &memberName) != nil || strings.TrimSpace(memberName) == "" {
		return "", "", false
	}
	return callID, strings.TrimSpace(memberName), true
}

func (c *Controller) UpdateUserExpert(id string, input expert.UserExpertInput) (expert.UserExpertInfo, error) {
	if c == nil || strings.TrimSpace(c.userExpertPath) == "" {
		return expert.UserExpertInfo{}, fmt.Errorf("user expert store is not configured")
	}
	id = strings.TrimSpace(id)
	if expert.IsBuiltinID(id) {
		return expert.UserExpertInfo{}, fmt.Errorf("official expert %q is read-only", id)
	}
	if err := c.ensureUserExpertUpdateAllowed(id); err != nil {
		return expert.UserExpertInfo{}, err
	}
	records, err := expert.LoadUserExperts(c.userExpertPath)
	if err != nil {
		return expert.UserExpertInfo{}, err
	}
	for i, record := range records {
		if record.ID != id {
			continue
		}
		next, err := expert.UpdateUserExpert(record, input, time.Now())
		if err != nil {
			return expert.UserExpertInfo{}, err
		}
		records[i] = next
		if err := expert.SaveUserExperts(c.userExpertPath, records); err != nil {
			return expert.UserExpertInfo{}, err
		}
		return c.finishUserExpertUpdate(id, next)
	}
	return expert.UserExpertInfo{}, fmt.Errorf("unknown user expert %q", id)
}

func (c *Controller) UserExpertStorePath() string {
	if c == nil {
		return ""
	}
	return c.userExpertPath
}

func (c *Controller) ensureUserExpertUpdateAllowed(id string) error {
	if c.CurrentExpert().SelectedID == id && c.Running() {
		return fmt.Errorf("cannot update the active expert while a turn is running")
	}
	return nil
}

func (c *Controller) finishUserExpertUpdate(id string, record expert.UserExpert) (expert.UserExpertInfo, error) {
	c.reloadExpertCatalog()
	if err := c.refreshSelectedExpert(id); err != nil {
		return expert.UserExpertInfo{}, err
	}
	return record.Info(), nil
}

// RefreshSelectedExpert reloads global user-expert data and, when id is active
// in this controller, replaces only the session system message so conversation
// history remains intact.
func (c *Controller) RefreshSelectedExpert(id string) error {
	c.reloadExpertCatalog()
	return c.refreshSelectedExpert(strings.TrimSpace(id))
}

func (c *Controller) refreshSelectedExpert(id string) error {
	c.sessionLifecycleMu.Lock()
	defer c.sessionLifecycleMu.Unlock()
	return c.refreshSelectedExpertLocked(id)
}

func (c *Controller) refreshSelectedExpertLocked(id string) error {
	if id == "" || c.CurrentExpert().SelectedID != id {
		return nil
	}
	if c.Running() {
		return fmt.Errorf("cannot refresh the active expert while a turn is running")
	}
	if c.promptBuild == nil && c.scenePromptBuild == nil {
		return fmt.Errorf("expert prompt builder is not configured")
	}
	sysPrompt, selection, err := c.buildSystemPrompt("", id)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.systemPrompt = sysPrompt
	c.expert = selection
	c.mu.Unlock()
	if c.executor != nil && c.executor.Session() != nil {
		c.executor.SetSession(sessionWithSystemPrompt(c.executor.Session().Snapshot(), sysPrompt))
	}
	c.saveExpertMeta(c.SessionPath())
	return nil
}

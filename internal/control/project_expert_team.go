package control

import (
	"fmt"
	"hash/fnv"
	"strings"

	"kwu/internal/agent"
	"kwu/internal/expert"
	"kwu/internal/scene"
)

const projectExpertTeamIDPrefix = "project-expert-team-"

// SetExpertGroup activates several catalog experts as one session expert team.
// The ordered expert ids are persisted with the session so a resumed project
// task reconstructs the same team instead of silently falling back to the first
// expert.
func (c *Controller) SetExpertGroup(ids []string) (expert.State, error) {
	normalized := normalizeProjectExpertIDs(ids)
	if len(normalized) == 0 {
		return c.ClearExpert()
	}
	if len(normalized) == 1 {
		return c.SetExpert(normalized[0])
	}

	c.sessionLifecycleMu.Lock()
	defer c.sessionLifecycleMu.Unlock()
	if err := c.validateExpertSwitch(); err != nil {
		return c.CurrentExpert(), err
	}
	sysPrompt, selection, err := c.buildProjectExpertTeamPrompt(normalized, expert.SourceRequest)
	if err != nil {
		return c.CurrentExpert(), err
	}
	if c.CurrentExpert().SelectedID == selection.ID {
		return c.CurrentExpert(), nil
	}
	meta, err := expertMetaForSelection(selection, agent.ExpertMeta{}, false)
	if err != nil {
		return c.CurrentExpert(), err
	}
	meta.ProjectExpertIDs = append([]string(nil), normalized...)
	path := c.SessionPath()
	if strings.TrimSpace(path) == "" {
		return c.CurrentExpert(), fmt.Errorf("save expert metadata: session path is not configured")
	}
	if err := c.persistAndPublishSelectedExpert(path, sysPrompt, selection, meta, false); err != nil {
		return c.CurrentExpert(), err
	}
	return selection.State(true), nil
}

func normalizeProjectExpertIDs(ids []string) []string {
	out := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, value := range ids {
		id := strings.TrimSpace(value)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (c *Controller) buildProjectExpertTeamPrompt(ids []string, source string) (string, expert.Selection, error) {
	c.mu.Lock()
	promptScene := c.scene
	c.mu.Unlock()
	return c.buildProjectExpertTeamPromptForScene(promptScene, ids, source)
}

func (c *Controller) buildProjectExpertTeamPromptForScene(promptScene scene.Scene, ids []string, source string) (string, expert.Selection, error) {
	team, err := c.projectExpertTeam(ids)
	if err != nil {
		return "", expert.Selection{}, err
	}
	base, _, err := c.buildSystemPromptForScene(promptScene, expert.ExplicitNoneID, "")
	if err != nil {
		return "", expert.Selection{}, err
	}
	selection := expert.Selection{ID: team.ID, Name: team.Name, Source: source, Selected: true, Expert: team}
	return expert.ApplyPrompt(base, team), selection, nil
}

func (c *Controller) projectExpertTeam(ids []string) (expert.Expert, error) {
	ids = normalizeProjectExpertIDs(ids)
	if len(ids) < 2 {
		return expert.Expert{}, fmt.Errorf("project expert team requires at least two experts")
	}
	catalog, err := c.projectExpertCatalog()
	if err != nil {
		return expert.Expert{}, err
	}
	members, promptSections, err := resolveProjectExpertMembers(catalog, ids)
	if err != nil {
		return expert.Expert{}, err
	}
	if len(members) < 2 {
		return expert.Expert{}, fmt.Errorf("project expert team resolved fewer than two distinct members")
	}
	team := newProjectExpertTeam(ids, members, promptSections)
	if err := team.Validate(); err != nil {
		return expert.Expert{}, err
	}
	return team, nil
}

func (c *Controller) projectExpertCatalog() (*expert.Catalog, error) {
	if c.loadExperts != nil {
		return c.loadExperts()
	}
	c.mu.Lock()
	catalog := c.experts
	c.mu.Unlock()
	if catalog == nil {
		return nil, fmt.Errorf("expert catalog is unavailable")
	}
	return catalog, nil
}

func resolveProjectExpertMembers(catalog *expert.Catalog, ids []string) ([]expert.Member, []string, error) {
	members := make([]expert.Member, 0, len(ids))
	promptSections := []string{"Coordinate the selected project experts. Delegate work to the appropriate members, reconcile disagreements, and return one coherent result."}
	seenMembers := make(map[string]struct{})
	for _, id := range ids {
		definition, err := catalog.Resolve(id)
		if err != nil {
			return nil, nil, err
		}
		promptSections = append(promptSections, fmt.Sprintf("Selected expert: %s\n%s", definition.Name, strings.TrimSpace(definition.Prompt)))
		members = appendDistinctProjectMembers(members, projectExpertCandidates(definition), seenMembers)
	}
	return members, promptSections, nil
}

func projectExpertCandidates(definition expert.Expert) []expert.Member {
	if definition.Mode == expert.ModeTeam && len(definition.Members) > 0 {
		return definition.Members
	}
	role := strings.TrimSpace(definition.Category)
	if role == "" {
		role = "Project expert"
	}
	return []expert.Member{{
		ExpertID: definition.ID,
		Name:     definition.Name,
		Role:     role,
		Task:     strings.TrimSpace(definition.Prompt),
		Output:   "Analysis, conclusions, and actionable recommendations",
		Tone:     "stone",
	}}
}

func appendDistinctProjectMembers(members []expert.Member, candidates []expert.Member, seen map[string]struct{}) []expert.Member {
	for _, candidate := range candidates {
		key := projectExpertMemberKey(candidate)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		members = append(members, normalizeProjectExpertMember(candidate, len(members) == 0))
	}
	return members
}

func projectExpertMemberKey(member expert.Member) string {
	if id := strings.TrimSpace(member.ExpertID); id != "" {
		return "expert:" + id
	}
	return strings.ToLower(strings.TrimSpace(member.Name))
}

func normalizeProjectExpertMember(member expert.Member, lead bool) expert.Member {
	member.Kind = expert.MemberKindSub
	if lead {
		member.Kind = expert.MemberKindLead
	}
	if strings.TrimSpace(member.Role) == "" {
		member.Role = "Project expert"
	}
	if strings.TrimSpace(member.Task) == "" {
		member.Task = "Analyze the assigned project task and contribute your specialist judgment."
	}
	if strings.TrimSpace(member.Output) == "" {
		member.Output = "Analysis, conclusions, and actionable recommendations"
	}
	if strings.TrimSpace(member.Tone) == "" {
		member.Tone = "stone"
	}
	return member
}

func newProjectExpertTeam(ids []string, members []expert.Member, promptSections []string) expert.Expert {
	digest := fnv.New64a()
	_, _ = digest.Write([]byte(strings.Join(ids, "\x00")))
	return expert.Expert{
		ID:          fmt.Sprintf("%s%016x", projectExpertTeamIDPrefix, digest.Sum64()),
		Name:        "项目专家团",
		Description: fmt.Sprintf("由 %d 位项目专家协作", len(members)),
		Category:    "Project",
		Prompt:      strings.Join(promptSections, "\n\n"),
		Language:    "auto",
		Enabled:     true,
		Source:      expert.SourceUser,
		Mode:        expert.ModeTeam,
		Members:     members,
	}
}

func duplicateUserExpertInput(record expert.UserExpert) expert.UserExpertInput {
	return expert.UserExpertInput{
		Name:          record.Name + " Copy",
		Mode:          record.Mode,
		Description:   record.Description,
		Category:      record.Category,
		Tags:          append([]string(nil), record.Tags...),
		AvatarKey:     record.AvatarKey,
		Members:       append([]expert.Member(nil), record.Members...),
		SystemPrompt:  record.SystemPrompt,
		DisplayPrompt: record.DisplayPrompt,
		Prompts:       append([]string(nil), record.Prompts...),
		Language:      record.Language,
		Enabled:       record.Enabled,
	}
}

func duplicateCatalogExpertInput(record expert.Expert) expert.UserExpertInput {
	return expert.UserExpertInput{
		Name:          record.Name + " Copy",
		Mode:          string(record.Mode),
		Description:   record.Description,
		Category:      record.Category,
		Tags:          []string{record.Category},
		Members:       append([]expert.Member(nil), record.Members...),
		SystemPrompt:  record.Prompt,
		DisplayPrompt: firstCatalogPrompt(record),
		Prompts:       append([]string(nil), record.Prompts...),
		Language:      record.Language,
		Enabled:       record.Enabled,
	}
}

func firstCatalogPrompt(record expert.Expert) string {
	if len(record.Prompts) > 0 {
		return record.Prompts[0]
	}
	return "Please handle the current task as " + record.Name + "."
}

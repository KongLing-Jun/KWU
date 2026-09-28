package expert

import (
	"fmt"
	"regexp"
	"strings"
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9]+(?:[-_][A-Za-z0-9]+)*$`)

// Expert is the backend contract for a session-level role. Prompt contains the
// private system instructions; Prompts contains safe user-facing starter
// questions and is intentionally exposed through Info.
type Expert struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category,omitempty"`
	Prompt      string   `json:"prompt"`
	Language    string   `json:"language,omitempty"`
	Enabled     bool     `json:"enabled"`
	Source      string   `json:"source"`
	Mode        Mode     `json:"mode"`
	Members     []Member `json:"members,omitempty"`
	Prompts     []string `json:"prompts"`
}

// Info is safe for frontends. It intentionally omits Prompt.
type Info struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category,omitempty"`
	Language    string   `json:"language,omitempty"`
	Enabled     bool     `json:"enabled"`
	Source      string   `json:"source"`
	Mode        Mode     `json:"mode"`
	Members     []Member `json:"members,omitempty"`
	Prompts     []string `json:"prompts"`
}

func (e Expert) Validate() error {
	if strings.TrimSpace(e.ID) == "" {
		return fmt.Errorf("expert id is required")
	}
	if !idPattern.MatchString(e.ID) {
		return fmt.Errorf("expert id %q must be a kebab-case or snake_case slug", e.ID)
	}
	if strings.TrimSpace(e.Name) == "" {
		return fmt.Errorf("expert %q name is required", e.ID)
	}
	if strings.TrimSpace(e.Prompt) == "" {
		return fmt.Errorf("expert %q prompt is required", e.ID)
	}
	return nil
}

func (e Expert) Info() Info {
	return Info{
		ID:          e.ID,
		Name:        e.Name,
		Description: e.Description,
		Category:    e.Category,
		Language:    e.Language,
		Enabled:     e.Enabled,
		Source:      e.Source,
		Mode:        e.Mode,
		Members:     append([]Member(nil), e.Members...),
		Prompts:     append([]string(nil), e.Prompts...),
	}
}

const (
	SourceOfficial = "official"
	SourceUser     = "user"
)

type Mode string

const (
	ModeExpert Mode = "expert"
	ModeTeam   Mode = "team"
)

const (
	MemberKindLead = "lead"
	MemberKindSub  = "sub"
)

type Member struct {
	Kind      string `json:"kind,omitempty"`
	ExpertID  string `json:"expertId,omitempty"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Task      string `json:"task"`
	Output    string `json:"output"`
	Tone      string `json:"tone"`
	AvatarKey string `json:"avatarKey,omitempty"`
}

func normalizeMembers(mode Mode, members []Member) []Member {
	if mode != ModeTeam {
		return []Member{}
	}
	out := make([]Member, len(members))
	for i, member := range members {
		member.Name = strings.TrimSpace(member.Name)
		member.ExpertID = strings.TrimSpace(member.ExpertID)
		member.Role = strings.TrimSpace(member.Role)
		member.Task = strings.TrimSpace(member.Task)
		member.Output = strings.TrimSpace(member.Output)
		member.Tone = strings.TrimSpace(member.Tone)
		member.AvatarKey = strings.TrimSpace(member.AvatarKey)
		if member.Tone == "" {
			member.Tone = "stone"
		}
		if i == 0 {
			member.Kind = MemberKindLead
		} else {
			member.Kind = MemberKindSub
		}
		out[i] = member
	}
	return out
}

func completeLegacyTeamMembers(teamName string, members []Member) []Member {
	members = normalizeMembers(ModeTeam, members)
	if len(members) == 0 {
		members = []Member{{}}
	}
	teamName = strings.TrimSpace(teamName)
	for i := range members {
		if members[i].Name == "" {
			if teamName == "" {
				members[i].Name = fmt.Sprintf("专家团成员 %d", i+1)
			} else {
				members[i].Name = fmt.Sprintf("%s成员 %d", teamName, i+1)
			}
		}
		if members[i].Role == "" {
			members[i].Role = "专家团协作"
		}
		if members[i].Task == "" {
			members[i].Task = "分析分配的任务并提交结论。"
		}
		if members[i].Output == "" {
			members[i].Output = "分析结论与行动建议"
		}
	}
	return normalizeMembers(ModeTeam, members)
}

func normalizeAndValidateMembers(id string, mode Mode, members []Member) ([]Member, error) {
	members = normalizeMembers(mode, members)
	if mode != ModeTeam {
		return members, nil
	}
	if len(members) == 0 {
		return members, nil
	}
	seenNames := make(map[string]struct{}, len(members))
	seenExpertIDs := make(map[string]struct{}, len(members))
	for i, member := range members {
		for _, required := range []struct {
			field string
			value string
		}{
			{field: "name", value: member.Name},
			{field: "role", value: member.Role},
			{field: "task", value: member.Task},
			{field: "output", value: member.Output},
		} {
			if required.value == "" {
				return nil, fmt.Errorf("expert %q team member %d %s is required", id, i+1, required.field)
			}
		}
		nameKey := strings.ToLower(member.Name)
		if _, exists := seenNames[nameKey]; exists {
			return nil, fmt.Errorf("expert %q team member name %q is duplicated", id, member.Name)
		}
		seenNames[nameKey] = struct{}{}
		if member.ExpertID != "" {
			if _, exists := seenExpertIDs[member.ExpertID]; exists {
				return nil, fmt.Errorf("expert %q team member expert id %q is duplicated", id, member.ExpertID)
			}
			seenExpertIDs[member.ExpertID] = struct{}{}
		}
	}
	return members, nil
}

type Definition struct {
	ID           string   `json:"id"`
	Source       string   `json:"source"`
	Mode         Mode     `json:"mode"`
	Name         string   `json:"name"`
	Role         string   `json:"role"`
	Description  string   `json:"description"`
	SystemPrompt string   `json:"systemPrompt,omitempty"`
	Categories   []string `json:"categories"`
	Tags         []string `json:"tags"`
	IconKey      string   `json:"iconKey"`
	Tone         string   `json:"tone"`
	AvatarKey    string   `json:"avatarKey,omitempty"`
	Members      []Member `json:"members"`
	Prompts      []string `json:"prompts"`
	SortOrder    int      `json:"sortOrder"`
	CreatedAt    int64    `json:"createdAt,omitempty"`
	UpdatedAt    int64    `json:"updatedAt,omitempty"`
}

func validatePrompts(id string, prompts []string) error {
	if len(prompts) < 3 || len(prompts) > 5 {
		return fmt.Errorf("expert %q must have between 3 and 5 prompts", id)
	}
	return nil
}

type ListFilter struct {
	Mode     Mode
	Category string
	Query    string
}

func ParseMode(value string) (Mode, error) {
	switch Mode(value) {
	case ModeExpert:
		return ModeExpert, nil
	case ModeTeam:
		return ModeTeam, nil
	default:
		return "", fmt.Errorf("invalid expert mode %q", value)
	}
}

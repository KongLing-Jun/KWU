package expert

import (
	"context"
	"fmt"
	"strings"
)

func LoadStoreCatalog(ctx context.Context, store *Store, userPath string) (*Catalog, error) {
	if store == nil {
		return nil, fmt.Errorf("expert store is required")
	}
	defs, err := store.List(ctx)
	if err != nil {
		return nil, err
	}
	return LoadDefinitionCatalog(defs, userPath)
}

// LoadDefinitionCatalog builds a runtime catalog from an in-memory definition
// snapshot plus the user expert store. It is used by hosts whose official
// catalog is supplied by a service rather than SQLite.
func LoadDefinitionCatalog(defs []Definition, userPath string) (*Catalog, error) {
	experts, err := expertsFromDefinitions(defs)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(experts))
	for _, ex := range experts {
		seen[ex.ID] = true
	}
	users, err := LoadUserExperts(userPath)
	if err != nil {
		return nil, err
	}
	for _, user := range users {
		if seen[user.ID] {
			continue
		}
		experts = append(experts, user.Expert())
		seen[user.ID] = true
	}
	return NewCatalog(experts)
}

func expertsFromDefinitions(defs []Definition) ([]Expert, error) {
	experts := make([]Expert, 0, len(defs))
	for _, def := range defs {
		ex, err := expertFromDefinition(def)
		if err != nil {
			return nil, err
		}
		experts = append(experts, ex)
	}
	return experts, nil
}

func expertFromDefinition(def Definition) (Expert, error) {
	normalized, err := normalizeDefinition(def)
	if err != nil {
		return Expert{}, err
	}
	category := normalized.Role
	if len(normalized.Categories) > 0 {
		category = normalized.Categories[0]
	}
	ex := Expert{
		ID:          normalized.ID,
		Name:        normalized.Name,
		Description: normalized.Description,
		Category:    category,
		Prompt:      definitionSystemPrompt(normalized),
		Language:    "auto",
		Enabled:     true,
		Source:      normalized.Source,
		Mode:        normalized.Mode,
		Members:     append([]Member(nil), normalized.Members...),
		Prompts:     append([]string(nil), normalized.Prompts...),
	}
	if err := ex.Validate(); err != nil {
		return Expert{}, err
	}
	return ex, nil
}

func definitionSystemPrompt(def Definition) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are %s.", def.Name)
	if def.Role != "" {
		fmt.Fprintf(&b, "\nRole: %s.", def.Role)
	}
	if def.Description != "" {
		fmt.Fprintf(&b, "\nMission: %s", def.Description)
	}
	if def.Mode == ModeTeam {
		b.WriteString("\n\nExpert team runtime:")
		b.WriteString("\n- Act as the lead: understand the goal, delegate concrete work to configured members with task, reconcile their results, and answer the user. Do not read or modify files or run commands directly.")
		b.WriteString("\n- For concrete work, delegate at least one suitable member. Use only an exact configured member_name; if none fits, explain the capability gap.")
		b.WriteString("\n- Delegate only members needed for the goal. Independent tasks may run concurrently; dependent tasks must wait for their prerequisites.")
		b.WriteString("\n- At most 4 member tasks may run concurrently, with at most one running turn per member.")
		b.WriteString("\n- Use wait_task to collect one or more results and stop_task to interrupt a running member. Do not finish while any required member task is running.")
		b.WriteString("\n- Record completed delegated work with complete_step before the final answer.")
		b.WriteString("\n- For file deliverables, delegate creation, finish all member work, and link the actual workspace file path in the final response.")
		if len(def.Members) > 0 {
			b.WriteString("\n\nTeam members:")
			for _, member := range def.Members {
				name := strings.TrimSpace(member.Name)
				if name == "" {
					name = "Unnamed member"
				}
				fmt.Fprintf(&b, "\n- %s", name)
				if member.Role != "" {
					fmt.Fprintf(&b, " (%s)", member.Role)
				}
				if member.Task != "" {
					fmt.Fprintf(&b, ": %s", member.Task)
				}
				if member.Output != "" {
					fmt.Fprintf(&b, " Output: %s.", member.Output)
				}
			}
		}
	} else {
		b.WriteString("\n\nWork in this expert role for the current session. Keep recommendations grounded in the user's task and the available evidence.")
	}
	if def.SystemPrompt != "" {
		b.WriteString("\n\nAdditional expert instructions:")
		fmt.Fprintf(&b, "\n%s", def.SystemPrompt)
	}
	if len(def.Prompts) > 0 {
		b.WriteString("\n\nTypical user intents this expert handles:")
		for _, prompt := range def.Prompts {
			fmt.Fprintf(&b, "\n- %s", prompt)
		}
	}
	return strings.TrimSpace(b.String())
}

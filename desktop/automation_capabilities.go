package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"
	"time"

	"kwu/internal/automation"
	"kwu/internal/expert"
	"kwu/internal/plugin"
)

var automationInvalidMCPNameChars = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

// AutomationListCapabilities returns the safe, read-only capability catalog
// used by the automation editor. Existing provider and connector configuration
// remains authoritative; this projection never returns credentials, prompts,
// commands, URLs, headers, or environment values.
func (a *App) AutomationListCapabilities() (automation.CapabilityCatalog, error) {
	return a.automationCapabilities(nil, a.Models())
}

// A nil selection browses all categories; an empty selection loads none of the
// optional categories. Task validation still uses the strict snapshot resolver.
func (a *App) automationCapabilities(selection *automation.CapabilitySelection, models []ModelInfo) (automation.CapabilityCatalog, error) {
	var experts []ExpertInfo
	if selection == nil || strings.TrimSpace(selection.ExpertID) != "" {
		var err error
		experts, err = a.ListExperts()
		if err != nil {
			return automation.CapabilityCatalog{}, err
		}
	}
	var skills []SkillView
	if selection == nil || len(selection.SkillIDs) > 0 {
		skills = a.SkillsSettings().Skills
	}
	var servers []ServerView
	if selection == nil || len(selection.ConnectorIDs) > 0 {
		servers = a.MCPServers()
	}
	catalog := automationCapabilityCatalog(models, skills, experts, servers)
	if selection == nil || strings.TrimSpace(selection.ExpertID) != "" {
		if userExperts, userErr := a.ListUserExperts(); userErr == nil {
			catalog = appendAutomationUserExpertCapabilities(catalog, userExperts)
		}
	}
	principal := automationActorPrincipal(a)
	for index := range catalog.Connectors {
		connector := &catalog.Connectors[index]
		connector.ServerID = connector.ID
		connector.GrantID = "desktop-mcp:" + connector.ID
		connector.GrantPrincipal = principal
		connector.GrantSource = "desktop_mcp_config"
	}
	return catalog, nil
}

func appendAutomationUserExpertCapabilities(catalog automation.CapabilityCatalog, userExperts []expert.UserExpertInfo) automation.CapabilityCatalog {
	seen := make(map[string]struct{}, len(catalog.Experts)+len(userExperts))
	for _, item := range catalog.Experts {
		seen[item.ID] = struct{}{}
	}
	for _, item := range userExperts {
		if _, duplicate := seen[item.ID]; duplicate {
			continue
		}
		option := automation.CapabilityOption{
			ID: item.ID, Name: item.Name, Available: item.Enabled,
			ContentHash: automationCapabilityHash(item),
		}
		if !option.Available {
			option.UnavailableReason = "disabled"
		}
		catalog.Experts = append(catalog.Experts, option)
		seen[item.ID] = struct{}{}
	}
	return catalog
}

// AutomationResolveCapabilities preflights a task against the same current
// catalog and strict snapshot contract used when the automation engine runs it.
func (a *App) AutomationResolveCapabilities(task automation.Task) (automation.CapabilitySnapshot, error) {
	return a.resolveAutomationCapabilities(task, time.Now().UTC())
}

func (a *App) resolveAutomationCapabilities(task automation.Task, capturedAt time.Time) (automation.CapabilitySnapshot, error) {
	resolved, _, err := a.resolveAutomationTask(task)
	if err != nil {
		return automation.CapabilitySnapshot{}, err
	}
	// The target-aware route resolver above already authorized this exact model.
	// Do not revalidate it against the unrelated active conversation's catalog.
	catalog, err := a.automationCapabilities(&resolved.Capabilities, []ModelInfo{{Ref: resolved.Prompt.Model}})
	if err != nil {
		return automation.CapabilitySnapshot{}, err
	}
	return automation.ResolveCapabilitySnapshotWithContext(
		resolved.Capabilities,
		catalog,
		automation.CapabilitySnapshotContext{
			Prompt: resolved.Prompt.Prompt, Principal: automationActorPrincipal(a),
		},
		capturedAt,
	)
}

func automationCapabilityCatalog(
	models []ModelInfo,
	skills []SkillView,
	experts []ExpertInfo,
	servers []ServerView,
) automation.CapabilityCatalog {
	catalog := automation.CapabilityCatalog{
		Models:     make([]automation.CapabilityOption, 0, len(models)),
		Skills:     make([]automation.CapabilityOption, 0, len(skills)),
		Experts:    make([]automation.CapabilityOption, 0, len(experts)),
		Connectors: make([]automation.CapabilityOption, 0, len(servers)),
	}
	for _, model := range models {
		catalog.Models = append(catalog.Models, automation.CapabilityOption{
			ID: model.Ref, Name: model.Model, Available: true,
		})
	}
	for _, item := range skills {
		option := automation.CapabilityOption{ID: item.Name, Name: item.Name, Available: item.Enabled, ContentHash: automationCapabilityHash(item)}
		if !option.Available {
			option.UnavailableReason = "disabled"
		}
		catalog.Skills = append(catalog.Skills, option)
	}
	for _, item := range experts {
		option := automation.CapabilityOption{ID: item.ID, Name: item.Name, Available: item.Enabled, ContentHash: automationCapabilityHash(item)}
		if !option.Available {
			option.UnavailableReason = "disabled"
		}
		catalog.Experts = append(catalog.Experts, option)
	}
	for _, server := range servers {
		available := server.Status == "connected"
		option := automation.CapabilityOption{
			ID: server.Name, Name: server.Name, Available: available,
			ToolAllowlist: automationConnectorToolNames(server.Name, server.ToolList),
			ReadOnlyTools: automationTrustedToolNames(server.Name, server.TrustedReadOnlyTools),
		}
		if !available {
			option.UnavailableReason = strings.TrimSpace(server.Status)
			if option.UnavailableReason == "" {
				option.UnavailableReason = "unavailable"
			}
		}
		catalog.Connectors = append(catalog.Connectors, option)
	}
	return catalog
}

func automationActorPrincipal(a *App) string {
	actor := a.currentActorContext()
	return actor.ActorID + "/" + actor.ProfileID
}

func automationCapabilityHash(value any) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(payload)
	return fmt.Sprintf("sha256:%x", digest)
}

func automationConnectorToolNames(server string, tools []ToolView) []string {
	out := make([]string, 0, len(tools))
	seen := make(map[string]struct{}, len(tools))
	for _, tool := range tools {
		name := automationMCPToolName(server, tool.Name)
		if _, duplicate := seen[name]; duplicate {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

func automationTrustedToolNames(server string, tools []string) []string {
	out := make([]string, 0, len(tools))
	seen := make(map[string]struct{}, len(tools))
	for _, raw := range tools {
		name := automationMCPToolName(server, raw)
		if _, duplicate := seen[name]; duplicate {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

func automationMCPToolName(server, raw string) string {
	return plugin.ToolPrefix(server) + automationNormalizeMCPName(raw)
}

// Keep raw tool normalization aligned with plugin.toolName. ToolPrefix is
// exported for the server portion; plugin intentionally keeps raw-name
// normalization private.
func automationNormalizeMCPName(value string) string {
	raw := value
	value = strings.Trim(automationInvalidMCPNameChars.ReplaceAllString(value, "_"), "_")
	if value == "" {
		value = "unnamed"
	}
	if value != raw {
		hash := fnv.New32a()
		_, _ = hash.Write([]byte(raw))
		value += "_" + fmt.Sprintf("%08x", hash.Sum32())[:6]
	}
	return value
}

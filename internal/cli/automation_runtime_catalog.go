package cli

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"kwu/internal/automation"
	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/expert"
	"kwu/internal/plugin"
	"kwu/internal/skill"
	"kwu/internal/userctx"
)

func newServeAutomation(ctx context.Context, actor userctx.ActorContext, session control.SessionAPI) automation.Service {
	store := automation.NewStore("")
	runner := automation.NewBusinessDiagnosticsRunner(automation.NewBootRunner(event.Discard))
	actor = actor.Normalize()
	source := newServeAutomationCapabilitySource(session, actor)
	engine := automation.NewEngineWithChecker(store, serveAutomationOwner(), runner, newServeAutomationChecker(actor, source))
	quota := automation.NewFileTaskQuotaResolver("", automation.TaskQuotaStateReady)
	svc := automation.NewServiceWithTaskQuota(store, engine, quota)
	engine.Start(ctx)
	return svc
}

func serveAutomationSameWorkspace(left, right string) bool {
	left, leftErr := filepath.Abs(strings.TrimSpace(left))
	right, rightErr := filepath.Abs(strings.TrimSpace(right))
	if leftErr != nil || rightErr != nil {
		return false
	}
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func serveAutomationCapabilityHash(value any) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(payload)
	return fmt.Sprintf("sha256:%x", digest)
}

func serveAutomationConnectorToolNames(server string, tools []plugin.ToolInfo) []string {
	out := make([]string, 0, len(tools))
	seen := make(map[string]struct{}, len(tools))
	for _, item := range tools {
		name := plugin.ToolPrefix(server) + serveAutomationNormalizeMCPName(item.Name)
		if _, duplicate := seen[name]; duplicate {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out
}

var serveAutomationInvalidMCPNameChars = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

// Keep raw tool normalization aligned with plugin.toolName. The host exposes
// raw MCP names in ServerStatus while snapshots must store model-visible names.
func serveAutomationNormalizeMCPName(value string) string {
	raw := value
	value = strings.Trim(serveAutomationInvalidMCPNameChars.ReplaceAllString(value, "_"), "_")
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

func serveAutomationModelRef(selection automation.CapabilitySelection, promptModel, defaultModel string) (string, error) {
	for _, candidate := range []string{selection.ModelRef, promptModel, defaultModel} {
		if modelRef := strings.TrimSpace(candidate); modelRef != "" {
			return modelRef, nil
		}
	}
	return "", fmt.Errorf("%w: serve automation model is required", automation.ErrCapabilityUnavailable)
}

func serveAutomationOwner() string {
	return fmt.Sprintf("serve:%d", os.Getpid())
}

type serveAutomationCapabilitySource func(workspaceRoot string) (automation.CapabilityCatalog, error)

func newServeAutomationChecker(actor userctx.ActorContext, source serveAutomationCapabilitySource) automation.Checker {
	actor = actor.Normalize()
	principal := actor.ActorID + "/" + actor.ProfileID
	return automation.Checker{
		CapabilityResolver: func(task automation.Task, capturedAt time.Time) (automation.CapabilitySnapshot, error) {
			root := strings.TrimSpace(task.WorkspaceRoot)
			if root == "" {
				root = "."
			}
			taskActor := actor
			taskActor.WorkspaceRoot = root
			cfg, err := config.LoadForRootWithActor(root, taskActor.Normalize())
			if err != nil {
				return automation.CapabilitySnapshot{}, err
			}

			selection := task.Capabilities
			selection.ModelRef, err = serveAutomationModelRef(selection, task.Prompt.Model, cfg.DefaultModel)
			if err != nil {
				return automation.CapabilitySnapshot{}, err
			}
			_, available := cfg.ResolveModel(selection.ModelRef)
			catalog := automation.CapabilityCatalog{Models: []automation.CapabilityOption{{
				ID: selection.ModelRef, Name: selection.ModelRef, Available: available,
				UnavailableReason: "not configured for the serve automation owner",
			}}}
			if serveAutomationNeedsRuntimeCatalog(selection) && source != nil {
				runtimeCatalog, err := source(root)
				if err != nil {
					return automation.CapabilitySnapshot{}, err
				}
				catalog.Skills = runtimeCatalog.Skills
				catalog.Experts = runtimeCatalog.Experts
				catalog.Connectors = runtimeCatalog.Connectors
			}
			return automation.ResolveCapabilitySnapshotWithContext(
				selection,
				catalog,
				automation.CapabilitySnapshotContext{Prompt: task.Prompt.Prompt, Principal: principal},
				capturedAt,
			)
		},
	}
}

func serveAutomationNeedsRuntimeCatalog(selection automation.CapabilitySelection) bool {
	return len(selection.SkillIDs) > 0 || strings.TrimSpace(selection.ExpertID) != "" || len(selection.ConnectorIDs) > 0
}

func newServeAutomationCapabilitySource(session control.SessionAPI, actor userctx.ActorContext) serveAutomationCapabilitySource {
	if session == nil {
		return nil
	}
	actor = actor.Normalize()
	principal := actor.ActorID + "/" + actor.ProfileID
	return func(workspaceRoot string) (automation.CapabilityCatalog, error) {
		if !serveAutomationSameWorkspace(workspaceRoot, session.WorkspaceRoot()) {
			return automation.CapabilityCatalog{}, fmt.Errorf("%w: task workspace does not match the serve runtime", automation.ErrCapabilityUnavailable)
		}
		taskActor := actor
		taskActor.WorkspaceRoot = workspaceRoot
		cfg, err := config.LoadForRootWithActor(workspaceRoot, taskActor.Normalize())
		if err != nil {
			return automation.CapabilityCatalog{}, err
		}
		var servers []plugin.ServerStatus
		if host := session.Host(); host != nil {
			servers = host.Servers()
		}
		return serveAutomationRuntimeCatalog(
			session.AllSkills(), func(name string) bool { return !cfg.IsSkillDisabled(name) }, session.ListExperts(), servers,
			session.DisconnectedMCPNames(), principal,
		), nil
	}
}

func serveAutomationRuntimeCatalog(
	skills []skill.Skill,
	skillEnabled func(string) bool,
	experts []expert.Info,
	servers []plugin.ServerStatus,
	disconnectedServers []string,
	principal string,
) automation.CapabilityCatalog {
	catalog := automation.CapabilityCatalog{
		Skills:     serveAutomationSkillOptions(skills, skillEnabled),
		Experts:    serveAutomationExpertOptions(experts),
		Connectors: make([]automation.CapabilityOption, 0, len(servers)+len(disconnectedServers)),
	}
	connected := make(map[string]struct{}, len(servers))
	for _, server := range servers {
		serverName := strings.TrimSpace(server.Name)
		if serverName == "" {
			continue
		}
		connected[serverName] = struct{}{}
		catalog.Connectors = append(catalog.Connectors, automation.CapabilityOption{
			ID: serverName, Name: serverName, Available: true,
			ToolAllowlist:  serveAutomationConnectorToolNames(serverName, server.ToolList),
			ServerID:       serverName,
			GrantID:        "serve-mcp:" + serverName,
			GrantPrincipal: principal,
			GrantSource:    "serve_runtime_mcp",
		})
	}
	for _, serverName := range disconnectedServers {
		serverName = strings.TrimSpace(serverName)
		if serverName == "" {
			continue
		}
		if _, ok := connected[serverName]; ok {
			continue
		}
		catalog.Connectors = append(catalog.Connectors, automation.CapabilityOption{
			ID: serverName, Name: serverName, Available: false, UnavailableReason: "not connected",
		})
	}
	return catalog
}

func serveAutomationSkillOptions(skills []skill.Skill, enabled func(string) bool) []automation.CapabilityOption {
	options := make([]automation.CapabilityOption, 0, len(skills))
	for _, item := range skills {
		available := enabled != nil && enabled(item.Name)
		option := automation.CapabilityOption{ID: item.Name, Name: item.Name, Available: available, ContentHash: serveAutomationCapabilityHash(item)}
		if !available {
			option.UnavailableReason = "disabled"
		}
		options = append(options, option)
	}
	return options
}

func serveAutomationExpertOptions(experts []expert.Info) []automation.CapabilityOption {
	options := make([]automation.CapabilityOption, 0, len(experts))
	for _, item := range experts {
		option := automation.CapabilityOption{ID: item.ID, Name: item.Name, Available: item.Enabled, ContentHash: serveAutomationCapabilityHash(item)}
		if !option.Available {
			option.UnavailableReason = "disabled"
		}
		options = append(options, option)
	}
	return options
}

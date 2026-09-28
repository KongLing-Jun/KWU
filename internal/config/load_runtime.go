package config

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"

	"kwu/internal/userctx"
)

// LoadForRootMetadataWithActor loads runtime configuration and provider metadata
// without loading or resolving any provider credential. It is for explicit
// subscription routes whose gateway adapter must never observe a local BYOK key.
func LoadForRootMetadataWithActor(root string, actor userctx.ActorContext) (*Config, error) {
	actor = actor.Normalize()
	if strings.TrimSpace(actor.WorkspaceRoot) == "" {
		actor.WorkspaceRoot = root
	}
	return LoadForRootMetadataWithPaths(root, NewProfilePathResolver(actor))
}

// LoadForRootMetadataWithPaths is the path-aware form of
// LoadForRootMetadataWithActor. Provider entries remain usable for model and
// presentation metadata, but APIKey never falls back to a credential store.
func LoadForRootMetadataWithPaths(root string, paths ProfilePathResolver) (*Config, error) {
	return loadForRootWithPaths(root, paths, false)
}

func loadRuntimeTOMLS(cfg *Config, root string, paths ProfilePathResolver) ([]string, error) {
	projectTOML := projectConfigPathForRoot(root)
	var sources []string
	if userConfig := paths.UserConfigLoadPath(); userConfig != "" {
		sources = append(sources, userConfig)
		if err := mergeRuntimeTOMLFile(cfg, userConfig); err != nil {
			return nil, err
		}
	}
	globalMaxSteps := cfg.Agent.MaxSteps
	globalPlannerMaxSteps := cfg.Agent.PlannerMaxSteps
	globalTimezone := cfg.Agent.Timezone
	globalPython := cfg.Tools.Python
	sources = append(sources, projectTOML)
	if err := mergeRuntimeTOMLFile(cfg, projectTOML); err != nil {
		return nil, err
	}
	// Runtime step caps are user/global controls, not project policy.
	cfg.Agent.MaxSteps = globalMaxSteps
	cfg.Agent.PlannerMaxSteps = globalPlannerMaxSteps
	cfg.Agent.Timezone = globalTimezone
	cfg.Tools.Python = globalPython
	return sources, nil
}

func finalizeRuntimeConfig(root string, paths ProfilePathResolver, cfg *Config, expansionEnv map[string]string, loadProviderCredentials bool) {
	cfg.Agent.AutoPlan = userAutoPlanModeWithPaths(paths)
	cfg.CredentialsStore = credentialsStoreModeWithPaths(paths)
	cfg.setExpansionEnv(expansionEnv)
	if loadProviderCredentials {
		resolveProviderCredentialsForRootWithPaths(root, paths, cfg)
		return
	}
	disableProviderCredentialLookup(cfg)
}

func replaceProvidersFromExistingEditFile(cfg *Config, path string, fileExists bool) error {
	if !fileExists {
		return nil
	}
	// TOML decodes an array table into the existing slice element. Loading a
	// user [[providers]] table onto Default would otherwise retain omitted
	// fields from the built-in provider (for example its models list), and a
	// later settings save could corrupt a single-model custom provider.
	var source Config
	if _, err := toml.DecodeFile(path, &source); err != nil {
		return fmt.Errorf("config %s: %w", path, err)
	}
	if len(source.Providers) > 0 {
		cfg.Providers = source.Providers
	}
	return nil
}

package boot

import (
	"fmt"
	"strings"

	"kwu/internal/brand"
	"kwu/internal/config"
	"kwu/internal/netclient"
	"kwu/internal/provider"
	"kwu/internal/userctx"
)

type bootRouteConfiguration struct {
	cfg                          *config.Config
	migrated                     *config.MigrationResult
	migrationErr                 error
	memoryCompilerConfigMigrated bool
	memoryCompilerConfigErr      error
	subscriptionRoute            bool
}

func loadBootRouteConfiguration(opts Options, actor userctx.ActorContext, root string) (bootRouteConfiguration, error) {
	state := bootRouteConfiguration{subscriptionRoute: usesSubscriptionBillingRoute(opts)}
	if !state.subscriptionRoute {
		state.migrated, state.migrationErr = migrateLegacyConfigForActor(actor, root)
	}
	state.memoryCompilerConfigMigrated, state.memoryCompilerConfigErr = config.MigrateLegacyMemoryCompilerForRoot(root, config.NewProfilePathResolver(actor))
	var err error
	if state.subscriptionRoute {
		state.cfg, err = config.LoadForRootMetadataWithActor(root, actor)
	} else {
		state.cfg, err = config.LoadForRootWithActor(root, actor)
	}
	if err == nil && state.subscriptionRoute {
		err = applySubscriptionModelCatalog(state.cfg, opts.SubscriptionModelCatalog)
	}
	return state, err
}

func applySubscriptionModelCatalog(cfg *config.Config, models []string) error {
	models = normalizedSubscriptionModelCatalog(models)
	if len(models) == 0 {
		return nil
	}
	entry := ensureSubscriptionOfficialProvider(cfg)
	entry.Model = ""
	entry.Models = append([]string(nil), models...)
	if !entry.HasModel(entry.Default) {
		entry.Default = models[0]
	}
	return nil
}

// ensureSubscriptionOfficialProvider adds the built-in provider only to the
// config copy used by this subscription boot. A same-named custom entry must
// not leak its endpoint or credentials into the subscription route.
func ensureSubscriptionOfficialProvider(cfg *config.Config) *config.ProviderEntry {
	official := config.OfficialProviderTemplate()
	for i := range cfg.Providers {
		if strings.TrimSpace(cfg.Providers[i].Name) == brand.ProviderID {
			cfg.Providers[i] = official
			return &cfg.Providers[i]
		}
	}
	cfg.Providers = append(cfg.Providers, official)
	return &cfg.Providers[len(cfg.Providers)-1]
}

func normalizedSubscriptionModelCatalog(models []string) []string {
	seen := make(map[string]struct{}, len(models))
	result := make([]string, 0, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		result = append(result, model)
	}
	return result
}

func validateExecutorCredential(opts Options, subscriptionRoute bool, cfg *config.Config, modelName string, entry *config.ProviderEntry, credentialOwner string, credentialPaths config.ProfilePathResolver) error {
	if !opts.RequireKey || subscriptionRoute {
		return nil
	}
	if err := cfg.Validate(modelName); err != nil {
		if validateErrorIsMissingAPIKey(entry) {
			return missingAPIKeyError(modelName, entry, credentialOwner, credentialPaths.UserCredentialsPath())
		}
		return err
	}
	return nil
}

func newExecutorProviderForRoute(subscriptionRoute bool, entry *config.ProviderEntry, proxySpec netclient.ProxySpec, credentialOwner string) (provider.Provider, error) {
	if subscriptionRoute {
		return provider.FailClosedProvider{Message: "subscription main executor is dispatcher-only"}, nil
	}
	if entry == nil {
		return nil, fmt.Errorf("executor provider entry is required")
	}
	return newProviderWithProxyForOwner(entry, proxySpec, credentialOwner)
}

func balanceKeyForRoute(entry *config.ProviderEntry, subscriptionRoute bool) string {
	if subscriptionRoute || entry == nil {
		return ""
	}
	return entry.APIKey()
}

func balanceURLForRoute(entry *config.ProviderEntry, subscriptionRoute bool) string {
	if subscriptionRoute || entry == nil {
		return ""
	}
	return entry.BalanceURL
}

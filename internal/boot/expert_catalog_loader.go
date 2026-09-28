package boot

import (
	"context"
	"path/filepath"
	"strings"

	"kwu/internal/config"
	"kwu/internal/expert"
)

func resolveExpertCatalogLoader(
	ctx context.Context,
	opts Options,
	userExpertPath string,
) func() (*expert.Catalog, error) {
	if opts.ExpertCatalogLoader != nil {
		return opts.ExpertCatalogLoader
	}
	expertStorePath := strings.TrimSpace(opts.ExpertStorePath)
	if expertStorePath == "" {
		if dir := strings.TrimSpace(config.MemoryUserDir()); dir != "" {
			expertStorePath = filepath.Join(dir, "experts.sqlite")
		}
	}
	builtinExperts := expert.BuiltinDefinitions()
	if opts.DisableBuiltinExperts {
		builtinExperts = nil
	}
	return func() (*expert.Catalog, error) {
		if expertStorePath != "" {
			store := expert.NewStore(expertStorePath, builtinExperts)
			return expert.LoadStoreCatalog(ctx, store, userExpertPath)
		}
		return expert.LoadCatalog(userExpertPath)
	}
}

package newapipricing

import "kwu/internal/modelbilling"

func cloneRefreshResult(value RefreshResult) RefreshResult {
	cloned := RefreshResult{CachedSnapshot: cloneCachedSnapshot(value.CachedSnapshot)}
	if value.Catalog != nil {
		catalog := cloneCatalog(*value.Catalog)
		cloned.Catalog = &catalog
	}
	return cloned
}

func cloneCatalog(value Catalog) Catalog {
	cloned := Catalog{
		SourceVersion: cloneString(value.SourceVersion),
		GroupRatios:   cloneGroupRatios(value.GroupRatios),
		UsableGroups:  append([]string(nil), value.UsableGroups...),
		Models:        make([]ModelMetadata, len(value.Models)),
		Prices:        cloneModelPrices(value.Prices),
		Issues:        make([]ModelIssue, len(value.Issues)),
	}
	for index, model := range value.Models {
		cloned.Models[index] = cloneModelMetadata(model)
	}
	for index, issue := range value.Issues {
		cloned.Issues[index] = issue
		if domainErr, ok := issue.Err.(*modelbilling.Error); ok {
			copied := *domainErr
			cloned.Issues[index].Err = &copied
		}
	}
	return cloned
}

func cloneModelMetadata(value ModelMetadata) ModelMetadata {
	value.VendorID = cloneInt(value.VendorID)
	if value.Vendor != nil {
		vendor := *value.Vendor
		value.Vendor = &vendor
	}
	value.EnableGroups = append([]string(nil), value.EnableGroups...)
	value.SupportedEndpointTypes = append([]string(nil), value.SupportedEndpointTypes...)
	value.ModelRatio = cloneDecimal(value.ModelRatio)
	value.CompletionRatio = cloneDecimal(value.CompletionRatio)
	value.CacheRatio = cloneDecimal(value.CacheRatio)
	value.CreateCacheRatio = cloneDecimal(value.CreateCacheRatio)
	value.ImageRatio = cloneDecimal(value.ImageRatio)
	value.AudioRatio = cloneDecimal(value.AudioRatio)
	value.AudioCompletionRatio = cloneDecimal(value.AudioCompletionRatio)
	value.ModelPrice = cloneDecimal(value.ModelPrice)
	return value
}

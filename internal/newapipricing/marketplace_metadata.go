package newapipricing

import (
	"errors"
	"fmt"
	"strings"

	"kwu/internal/modelbilling"
)

const MarketplaceMetadataFormatVersion = 1

type MarketplaceMetadataErrorCode string

const (
	MetadataUnavailable MarketplaceMetadataErrorCode = "metadata_unavailable"
	MetadataWriteFailed MarketplaceMetadataErrorCode = "metadata_write_failed"
)

type MarketplaceMetadataFile struct {
	Version    int                        `json:"version"`
	SourceID   string                     `json:"source_id"`
	SnapshotID string                     `json:"snapshot_id"`
	SourceHash string                     `json:"source_hash"`
	Models     []MarketplaceModelMetadata `json:"models"`
	Issues     []MarketplaceModelIssue    `json:"issues"`
}

type MarketplaceModelMetadata struct {
	ModelID                string   `json:"model_id"`
	ModelName              string   `json:"model_name"`
	Description            string   `json:"description,omitempty"`
	Vendor                 string   `json:"vendor,omitempty"`
	VendorIcon             string   `json:"vendor_icon,omitempty"`
	SupportedEndpointTypes []string `json:"supported_endpoint_types,omitempty"`
}

type MarketplaceModelIssue struct {
	ModelID   string                 `json:"model_id,omitempty"`
	ModelName string                 `json:"model_name,omitempty"`
	Code      modelbilling.ErrorCode `json:"code"`
}

type marketplaceMetadataError struct {
	code MarketplaceMetadataErrorCode
	err  error
}

func (e *marketplaceMetadataError) Error() string {
	return fmt.Sprintf("%s: %v", e.code, e.err)
}

func (e *marketplaceMetadataError) Unwrap() error { return e.err }

func MarketplaceMetadataErrorCodeOf(err error) MarketplaceMetadataErrorCode {
	var metadataErr *marketplaceMetadataError
	if errors.As(err, &metadataErr) {
		return metadataErr.code
	}
	return ""
}

func ProjectMarketplaceMetadata(cached CachedSnapshot, catalog Catalog) (MarketplaceMetadataFile, error) {
	if err := cached.Snapshot.Validate(); err != nil {
		return MarketplaceMetadataFile{}, err
	}
	if !equalOptionalString(cached.Snapshot.SourceVersion, catalog.SourceVersion) {
		return MarketplaceMetadataFile{}, contractError("marketplace Catalog version does not match snapshot")
	}
	metadata := MarketplaceMetadataFile{
		Version: MarketplaceMetadataFormatVersion, SourceID: cached.Snapshot.Source,
		SnapshotID: cached.Snapshot.SnapshotID, SourceHash: cached.Snapshot.SourceHash,
		Models: make([]MarketplaceModelMetadata, 0, len(catalog.Models)),
		Issues: make([]MarketplaceModelIssue, 0, len(catalog.Issues)),
	}
	for _, model := range catalog.Models {
		item := MarketplaceModelMetadata{
			ModelID: model.ModelID, ModelName: model.ModelName, Description: model.Description,
			SupportedEndpointTypes: append([]string(nil), model.SupportedEndpointTypes...),
		}
		if model.Vendor != nil {
			item.Vendor = model.Vendor.Name
			item.VendorIcon = model.Vendor.Icon
		}
		metadata.Models = append(metadata.Models, item)
	}
	for _, issue := range catalog.Issues {
		code := modelbilling.ErrorCodeOf(issue.Err)
		if code == "" {
			code = modelbilling.ErrorContractInvalid
		}
		metadata.Issues = append(metadata.Issues, MarketplaceModelIssue{
			ModelID: issue.ModelName, ModelName: issue.ModelName, Code: code,
		})
	}
	if err := validateMarketplaceMetadata(metadata); err != nil {
		return MarketplaceMetadataFile{}, err
	}
	return metadata, nil
}

func validateMarketplaceMetadata(metadata MarketplaceMetadataFile) error {
	if metadata.Version != MarketplaceMetadataFormatVersion {
		return contractError("unsupported marketplace metadata version %d", metadata.Version)
	}
	if !sourceIDPattern.MatchString(metadata.SourceID) || strings.TrimSpace(metadata.SnapshotID) == "" || strings.TrimSpace(metadata.SourceHash) == "" {
		return contractError("marketplace metadata identity is invalid")
	}
	if len(metadata.Models) == 0 {
		return contractError("marketplace metadata models are required")
	}
	if err := validateMarketplaceMetadataModels(metadata.Models); err != nil {
		return err
	}
	return validateMarketplaceMetadataIssues(metadata.Issues)
}

func validateMarketplaceMetadataModels(models []MarketplaceModelMetadata) error {
	for _, model := range models {
		if strings.TrimSpace(model.ModelID) == "" || strings.TrimSpace(model.ModelName) == "" {
			return contractError("marketplace metadata model identity is required")
		}
		for _, endpoint := range model.SupportedEndpointTypes {
			if strings.TrimSpace(endpoint) == "" {
				return contractError("marketplace metadata endpoint type is invalid")
			}
		}
	}
	return nil
}

func validateMarketplaceMetadataIssues(issues []MarketplaceModelIssue) error {
	for _, issue := range issues {
		if strings.TrimSpace(issue.ModelID) != strings.TrimSpace(issue.ModelName) || issue.Code.Validate() != nil {
			return contractError("marketplace metadata issue is invalid")
		}
	}
	return nil
}

func metadataError(code MarketplaceMetadataErrorCode, err error) error {
	return &marketplaceMetadataError{code: code, err: err}
}

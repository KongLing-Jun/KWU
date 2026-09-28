package newapipricing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"kwu/internal/modelbilling"
)

const snapshotIDPrefix = "pricing-v1"

type hashSnapshot struct {
	Source        string                                `json:"source"`
	SourceVersion *string                               `json:"source_version,omitempty"`
	Currency      string                                `json:"currency"`
	ExchangeRate  *modelbilling.DecimalString           `json:"exchange_rate,omitempty"`
	RechargePrice *modelbilling.DecimalString           `json:"recharge_price,omitempty"`
	GroupRatios   map[string]modelbilling.DecimalString `json:"group_ratios"`
	UsableGroups  []string                              `json:"usable_groups"`
	Models        []modelbilling.ModelPrice             `json:"models"`
	Provenance    hashProvenance                        `json:"provenance"`
}

type hashEvidence struct {
	DependencyID  string             `json:"dependency_id"`
	Authenticity  SourceAuthenticity `json:"authenticity"`
	SourceVersion *string            `json:"source_version,omitempty"`
	SourceHash    *string            `json:"source_hash,omitempty"`
}

type hashProvenance struct {
	Pricing hashEvidence `json:"pricing"`
	Status  hashEvidence `json:"status"`
}

func ContentHash(snapshot modelbilling.PricingSnapshot, provenance SnapshotProvenance) (string, error) {
	canonical := clonePricingSnapshot(snapshot)
	sort.Strings(canonical.UsableGroups)
	for index := range canonical.Models {
		sort.Strings(canonical.Models[index].Capabilities)
		sort.Strings(canonical.Models[index].EnableGroups)
	}
	sort.Slice(canonical.Models, func(i, j int) bool {
		left, _ := json.Marshal(canonical.Models[i])
		right, _ := json.Marshal(canonical.Models[j])
		return string(left) < string(right)
	})
	document := hashSnapshot{
		Source: canonical.Source, SourceVersion: canonical.SourceVersion, Currency: canonical.Currency,
		ExchangeRate: canonical.ExchangeRate, RechargePrice: canonical.RechargePrice,
		GroupRatios: canonical.GroupRatios, UsableGroups: canonical.UsableGroups, Models: canonical.Models,
		Provenance: hashProvenance{
			Pricing: hashSourceEvidence(provenance.Pricing), Status: hashSourceEvidence(provenance.Status),
		},
	}
	data, err := json.Marshal(document)
	if err != nil {
		return "", contractError("encode normalized pricing hash input: %v", err)
	}
	digest := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func hashSourceEvidence(value SourceEvidence) hashEvidence {
	return hashEvidence{
		DependencyID: value.DependencyID, Authenticity: value.Authenticity,
		SourceVersion: cloneString(value.SourceVersion), SourceHash: cloneString(value.SourceHash),
	}
}

func BuildSnapshotID(source, contentHash string) string {
	hash := strings.TrimPrefix(contentHash, "sha256:")
	return fmt.Sprintf("%s:%s:%s", snapshotIDPrefix, source, hash)
}

func validateCacheFile(record cacheFile, policy refreshPolicy, source string) error {
	if record.Version != CacheFormatVersion {
		return contractError("unsupported pricing cache version %d", record.Version)
	}
	if err := validatePersistedSnapshot(record.Snapshot, policy, source); err != nil {
		return err
	}
	if err := validateProvenance(record); err != nil {
		return err
	}
	return validateContentIdentity(record, source)
}

func validatePersistedSnapshot(snapshot modelbilling.PricingSnapshot, policy refreshPolicy, source string) error {
	if snapshot.Source != source || !sourceIDPattern.MatchString(snapshot.Source) {
		return contractError("pricing cache source does not match its cache key")
	}
	if snapshot.State != modelbilling.PricingFresh || len(snapshot.Models) == 0 {
		return contractError("persisted pricing cache must contain a successful fresh snapshot")
	}
	if err := snapshot.Validate(); err != nil {
		return err
	}
	if !snapshot.FreshUntil.Equal(snapshot.FetchedAt.Add(policy.FreshFor)) ||
		!snapshot.StaleUntil.Equal(snapshot.FetchedAt.Add(policy.StaleFor)) {
		return contractError("pricing cache time bounds do not match the refresh policy")
	}
	return nil
}

func validateContentIdentity(record cacheFile, source string) error {
	hash, err := ContentHash(record.Snapshot, record.Provenance)
	if err != nil {
		return err
	}
	if record.Snapshot.SourceHash != hash || record.Snapshot.SnapshotID != BuildSnapshotID(source, hash) {
		return contractError("pricing cache snapshot identity or content hash does not match its normalized content")
	}
	return nil
}

func validateProvenance(record cacheFile) error {
	pricing := record.Provenance.Pricing
	status := record.Provenance.Status
	if pricing.DependencyID != pricingDependency || pricing.Authenticity != SourceReady || pricing.FetchedAt.IsZero() {
		return contractError("pricing cache pricing provenance must remain NEWAPI-PRICING-01 READY")
	}
	if status.DependencyID != statusDependency || status.Authenticity != SourceMockOnly || status.FetchedAt.IsZero() {
		return contractError("pricing cache status provenance must remain NEWAPI-STATUS-01 MOCK_ONLY")
	}
	if !equalOptionalString(pricing.SourceVersion, record.Snapshot.SourceVersion) {
		return contractError("pricing cache source version does not match pricing provenance")
	}
	if record.Snapshot.ExchangeRate == nil || record.Snapshot.RechargePrice == nil {
		return contractError("pricing cache requires normalized status values")
	}
	statusContract := modelbilling.PricingSourceStatus{
		USDExchangeRate: *record.Snapshot.ExchangeRate, RechargePrice: *record.Snapshot.RechargePrice,
		FetchedAt: status.FetchedAt, SourceVersion: status.SourceVersion, SourceHash: status.SourceHash,
	}
	return statusContract.Validate()
}

func equalOptionalString(left, right *string) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

func cloneCacheFile(value cacheFile) cacheFile {
	return cacheFile{Version: value.Version, Snapshot: clonePricingSnapshot(value.Snapshot), Provenance: cloneProvenance(value.Provenance)}
}

func cloneCachedSnapshot(value CachedSnapshot) CachedSnapshot {
	return CachedSnapshot{Snapshot: clonePricingSnapshot(value.Snapshot), Provenance: cloneProvenance(value.Provenance)}
}

func cloneProvenance(value SnapshotProvenance) SnapshotProvenance {
	return SnapshotProvenance{Pricing: cloneSourceEvidence(value.Pricing), Status: cloneSourceEvidence(value.Status)}
}

func cloneSourceEvidence(value SourceEvidence) SourceEvidence {
	value.SourceVersion = cloneString(value.SourceVersion)
	value.SourceHash = cloneString(value.SourceHash)
	return value
}

func clonePricingSnapshot(value modelbilling.PricingSnapshot) modelbilling.PricingSnapshot {
	value.SourceVersion = cloneString(value.SourceVersion)
	value.ExchangeRate = cloneDecimal(value.ExchangeRate)
	value.RechargePrice = cloneDecimal(value.RechargePrice)
	value.GroupRatios = cloneGroupRatios(value.GroupRatios)
	value.UsableGroups = append([]string(nil), value.UsableGroups...)
	value.Models = cloneModelPrices(value.Models)
	return value
}

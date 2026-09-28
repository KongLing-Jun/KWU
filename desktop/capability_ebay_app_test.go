package main

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestEbaySnapshotProjectionProducesLiveAuthorizedEvidence(t *testing.T) {
	snapshot := ebayBrowseSearchResponse{Total: 42}
	first := struct {
		ItemID    string `json:"itemId"`
		Title     string `json:"title"`
		Condition string `json:"condition"`
		Price     struct {
			Value    string `json:"value"`
			Currency string `json:"currency"`
		} `json:"price"`
	}{Title: "A", Condition: "New"}
	first.Price.Value, first.Price.Currency = "19.99", "USD"
	second := first
	second.Title, second.Price.Value = "B", "39.99"
	snapshot.ItemSummaries = append(snapshot.ItemSummaries, first, second)

	artifact, widget, err := ebaySnapshotProjection(testCapabilityScope(), CapabilityEbayResearchRequest{Query: "portable blender", Marketplace: "EBAY_US", Limit: 30}, snapshot, time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("project snapshot: %v", err)
	}
	if artifact.Provenance.DataClass != "live-authorized" || artifact.Provenance.Source != "eBay" {
		t.Fatalf("unexpected provenance: %+v", artifact.Provenance)
	}
	if len(widget.EvidenceRefs) != 1 || widget.EvidenceRefs[0] != artifact.EvidenceRef {
		t.Fatalf("widget evidence refs = %#v, artifact = %q", widget.EvidenceRefs, artifact.EvidenceRef)
	}
	if err := validateCapabilityEvidenceArtifact(artifact, testCapabilityScope()); err != nil {
		t.Fatalf("validate evidence: %v", err)
	}
	if err := validateCapabilityWidget(widget); err != nil {
		t.Fatalf("validate widget: %v", err)
	}
}

func TestEbayProductionTokenRequiresHostCredential(t *testing.T) {
	t.Setenv("EBAY_OAUTH_TOKEN", "")
	t.Setenv("EBAY_CLIENT_ID", "")
	t.Setenv("EBAY_CLIENT_SECRET", "")
	if _, err := ebayProductionToken(context.Background(), http.DefaultClient); err == nil {
		t.Fatal("expected missing production authorization error")
	}
}

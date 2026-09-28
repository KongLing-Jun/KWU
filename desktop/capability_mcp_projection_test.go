package main

import (
	"encoding/json"
	"testing"

	"kwu/internal/event"
)

func TestShopifyCatalogProjectionUsesUntruncatedHostResult(t *testing.T) {
	tool := event.Tool{
		Name:      shopifyCatalogSearchTool,
		Args:      `{"catalog":{"query":"snowboard"}}`,
		Output:    `{"ucp":`,
		RawOutput: `{"ucp":{"status":"success","products":[{"id":"gid://shopify/Product/1","title":"Board","url":"https://store.example/products/board","price_range":{"min":{"amount":12999,"currency":"USD"},"max":{"amount":14999,"currency":"USD"}},"variants":[{"availability":{"available":true}},{"availability":{"available":false}}],"media":[{"type":"image","url":"https://cdn.example/board.jpg"}]}]}}`,
	}
	artifact, widget, _, err := shopifyCatalogProjection(testCapabilityScope(), tool)
	if err != nil {
		t.Fatalf("project catalog: %v", err)
	}
	if artifact.Provenance.DataClass != "live-public" || len(artifact.Facts) < 3 {
		t.Fatalf("unexpected artifact: %#v", artifact)
	}
	if widget.Type != "product.catalog" || len(widget.EvidenceRefs) != 1 || widget.EvidenceRefs[0] != artifact.EvidenceRef {
		t.Fatalf("unexpected widget: %#v", widget)
	}
	var props struct {
		Products []struct {
			Price             string `json:"price"`
			AvailableVariants int    `json:"availableVariants"`
		} `json:"products"`
	}
	if err := json.Unmarshal(widget.Props, &props); err != nil {
		t.Fatalf("decode widget props: %v", err)
	}
	if len(props.Products) != 1 || props.Products[0].Price != "129.99 USD–149.99 USD" || props.Products[0].AvailableVariants != 1 {
		t.Fatalf("unexpected product props: %#v", props.Products)
	}
}

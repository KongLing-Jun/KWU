package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"kwu/internal/event"
)

const shopifyCatalogSearchTool = "mcp__" + shopifyGlobalCatalogServer + "__search_catalog"

type shopifyCatalogSearchResult struct {
	UCP struct {
		Status   string `json:"status"`
		Products []struct {
			ID         string `json:"id"`
			Title      string `json:"title"`
			PriceRange struct {
				Min shopifyMoney `json:"min"`
				Max shopifyMoney `json:"max"`
			} `json:"price_range"`
			Variants []struct {
				URL          string `json:"url"`
				Availability struct {
					Available bool `json:"available"`
				} `json:"availability"`
			} `json:"variants"`
			Media []struct {
				Type string `json:"type"`
				URL  string `json:"url"`
			} `json:"media"`
		} `json:"products"`
	} `json:"ucp"`
}

type shopifyMoney struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

func (a *App) projectCapabilityMCPToolResult(tabID string, tool event.Tool) {
	if a == nil || tool.Name != shopifyCatalogSearchTool || tool.Err != "" || (strings.TrimSpace(tool.RawOutput) == "" && strings.TrimSpace(tool.Output) == "") {
		return
	}
	a.mu.RLock()
	tab := a.tabByEventSinkIDLocked(tabID)
	if tab == nil {
		a.mu.RUnlock()
		return
	}
	workspaceRoot := strings.TrimSpace(tab.WorkspaceRoot)
	sessionPath := tab.currentSessionPath()
	a.mu.RUnlock()
	scope := a.capabilityTaskScopeForSession(workspaceRoot, sessionPath)
	if scope == nil || scope.CapabilityID != "crossborder-commerce" {
		return
	}
	active, err := a.capabilityContextForTab(tabID, workspaceRoot, sessionPath)
	if err != nil {
		return
	}
	artifact, widget, capturedAt, err := shopifyCatalogProjection(*scope, tool)
	if err != nil {
		return
	}
	_, _, _ = a.persistCapabilityLiveProjection(active, *scope, artifact, widget, capturedAt)
}

func shopifyCatalogProjection(scope CapabilityTaskScope, tool event.Tool) (capabilityEvidenceArtifact, capabilityDashboardWidget, time.Time, error) {
	output := tool.RawOutput
	if output == "" {
		output = tool.Output
	}
	if len(output) > 2<<20 {
		return capabilityEvidenceArtifact{}, capabilityDashboardWidget{}, time.Time{}, errors.New("Shopify catalog result exceeds projection limit")
	}
	var args struct {
		Catalog struct {
			Query string `json:"query"`
		} `json:"catalog"`
	}
	if err := json.Unmarshal([]byte(tool.Args), &args); err != nil {
		return capabilityEvidenceArtifact{}, capabilityDashboardWidget{}, time.Time{}, fmt.Errorf("decode Shopify catalog args: %w", err)
	}
	query := strings.TrimSpace(args.Catalog.Query)
	if query == "" {
		query = "商品目录"
	}
	var result shopifyCatalogSearchResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return capabilityEvidenceArtifact{}, capabilityDashboardWidget{}, time.Time{}, fmt.Errorf("decode Shopify catalog result: %w", err)
	}
	if result.UCP.Status != "success" {
		return capabilityEvidenceArtifact{}, capabilityDashboardWidget{}, time.Time{}, errors.New("Shopify catalog result was not successful")
	}

	capturedAt := time.Now().UTC()
	queryHash := sha256.Sum256([]byte(strings.ToLower(query)))
	evidenceRef := "ev:shopify:live-catalog:" + hex.EncodeToString(queryHash[:6])
	type productProps struct {
		ID                string `json:"id"`
		Title             string `json:"title"`
		URL               string `json:"url"`
		ImageURL          string `json:"imageUrl,omitempty"`
		Price             string `json:"price"`
		AvailableVariants int    `json:"availableVariants"`
		TotalVariants     int    `json:"totalVariants"`
	}
	products := make([]productProps, 0, min(len(result.UCP.Products), 8))
	facts := []string{fmt.Sprintf("跨商家公开目录搜索返回 %d 个商品", len(result.UCP.Products))}
	merchantHosts := make(map[string]struct{})
	for _, product := range result.UCP.Products {
		if len(products) >= 8 {
			break
		}
		available := 0
		productURL := ""
		for _, variant := range product.Variants {
			if variant.Availability.Available {
				available++
			}
			if productURL == "" {
				productURL = variant.URL
			}
		}
		imageURL := ""
		for _, media := range product.Media {
			if media.Type == "image" {
				imageURL = media.URL
				break
			}
		}
		currency := product.PriceRange.Min.Currency
		if currency == "" {
			currency = product.PriceRange.Max.Currency
		}
		price := formatShopifyPrice(product.PriceRange.Min.Amount, currency)
		if product.PriceRange.Max.Amount != product.PriceRange.Min.Amount {
			price += "–" + formatShopifyPrice(product.PriceRange.Max.Amount, currency)
		}
		products = append(products, productProps{ID: product.ID, Title: product.Title, URL: productURL, ImageURL: imageURL, Price: price, AvailableVariants: available, TotalVariants: len(product.Variants)})
		facts = append(facts, fmt.Sprintf("%s：%s，可售规格 %d/%d", product.Title, price, available, len(product.Variants)))
		if parsed, err := url.Parse(productURL); err == nil && parsed.Host != "" {
			merchantHosts[parsed.Host] = struct{}{}
		}
	}
	storeLabel := fmt.Sprintf("Shopify Global Catalog · %d 个商家", len(merchantHosts))
	facts = append(facts, "数据来自跨商家公开目录，不代表全市场销量、成交量或需求规模")
	artifact := capabilityEvidenceArtifact{
		EvidenceRef: evidenceRef,
		Scope:       capabilityEvidenceScope{CapabilityID: scope.CapabilityID, WorkspaceID: scope.WorkspaceID, TaskID: scope.TaskID},
		Title:       "Shopify 实时目录 · " + query,
		Summary:     fmt.Sprintf("%s 的公开目录搜索返回 %d 个商品；该结果用于商品与价格核验，不用于推断全市场表现。", storeLabel, len(result.UCP.Products)),
		Facts:       facts,
		Provenance:  capabilityEvidenceProvenance{Source: "Shopify", CapturedAt: capturedAt.Format(time.RFC3339Nano), DataClass: "live-public"},
	}
	props, _ := json.Marshal(map[string]any{
		"kind": "product.catalog", "eyebrow": "Shopify Global Catalog · 实时", "label": query,
		"store": storeLabel, "products": products, "footnote": "跨商家公开目录快照；不代表全市场销量、成交量或需求规模。",
	})
	widget := capabilityDashboardWidget{
		ID: "shopify-live-catalog-" + hex.EncodeToString(queryHash[:4]), Type: "product.catalog", Version: "1.0.0", Surface: "research-canvas", Status: "ready",
		Title: "Shopify 实时商品目录 · " + query, Props: props, EvidenceRefs: []string{evidenceRef}, Revision: 1,
	}
	return artifact, widget, capturedAt, nil
}

func formatShopifyPrice(cents int64, currency string) string {
	return fmt.Sprintf("%.2f %s", float64(cents)/100, currency)
}

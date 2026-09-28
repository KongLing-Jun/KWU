package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	ebayProductionAPIBase = "https://api.ebay.com"
	ebayOAuthScope        = "https://api.ebay.com/oauth/api_scope"
	ebayBrowseMaxItems    = 50
)

type CapabilityEbayResearchRequest struct {
	Query       string `json:"query"`
	Marketplace string `json:"marketplace"`
	Limit       int    `json:"limit"`
}

type CapabilityEbayResearchResult struct {
	Query         string `json:"query"`
	Marketplace   string `json:"marketplace"`
	CapturedAt    string `json:"capturedAt"`
	EvidenceJSON  string `json:"evidenceJson"`
	DashboardJSON string `json:"dashboardJson"`
}

type ebayBrowseSearchResponse struct {
	Total         int `json:"total"`
	ItemSummaries []struct {
		ItemID    string `json:"itemId"`
		Title     string `json:"title"`
		Condition string `json:"condition"`
		Price     struct {
			Value    string `json:"value"`
			Currency string `json:"currency"`
		} `json:"price"`
	} `json:"itemSummaries"`
}

type ebayApplicationTokenResponse struct {
	AccessToken string `json:"access_token"`
}

// RunCapabilityEbayBrowseResearch pulls a production Browse API snapshot and
// projects it into the existing durable evidence/dashboard contracts. It never
// accepts credentials from the renderer and never falls back to mock data.
func (a *App) RunCapabilityEbayBrowseResearch(scope CapabilityTaskScope, request CapabilityEbayResearchRequest) (CapabilityEbayResearchResult, error) {
	request.Query = strings.TrimSpace(request.Query)
	if request.Query == "" || len(request.Query) > 200 {
		return CapabilityEbayResearchResult{}, errors.New("eBay query is required and must not exceed 200 characters")
	}
	request.Marketplace = strings.ToUpper(strings.TrimSpace(request.Marketplace))
	if request.Marketplace == "" {
		request.Marketplace = "EBAY_US"
	}
	if request.Marketplace != "EBAY_US" {
		return CapabilityEbayResearchResult{}, errors.New("the first live adapter currently supports EBAY_US only")
	}
	if request.Limit <= 0 {
		request.Limit = 30
	}
	if request.Limit > ebayBrowseMaxItems {
		request.Limit = ebayBrowseMaxItems
	}

	active, err := a.capabilityActiveContext()
	if err != nil {
		return CapabilityEbayResearchResult{}, err
	}
	if err := validateCapabilityBinding(active, scope); err != nil {
		return CapabilityEbayResearchResult{}, err
	}

	client, err := httpClient()
	if err != nil {
		return CapabilityEbayResearchResult{}, fmt.Errorf("create eBay HTTP client: %w", err)
	}
	ctx, cancel := context.WithTimeout(a.bootContext(), 20*time.Second)
	defer cancel()
	token, err := ebayProductionToken(ctx, client)
	if err != nil {
		return CapabilityEbayResearchResult{}, err
	}
	snapshot, err := fetchEbayBrowseSnapshot(ctx, client, token, request)
	if err != nil {
		return CapabilityEbayResearchResult{}, err
	}
	capturedAt := time.Now().UTC()
	artifact, widget, err := ebaySnapshotProjection(scope, request, snapshot, capturedAt)
	if err != nil {
		return CapabilityEbayResearchResult{}, err
	}

	evidenceJSON, dashboardJSON, err := a.persistCapabilityLiveProjection(active, scope, artifact, widget, capturedAt)
	if err != nil {
		return CapabilityEbayResearchResult{}, err
	}
	return CapabilityEbayResearchResult{
		Query:         request.Query,
		Marketplace:   request.Marketplace,
		CapturedAt:    capturedAt.Format(time.RFC3339Nano),
		EvidenceJSON:  evidenceJSON,
		DashboardJSON: dashboardJSON,
	}, nil
}

func ebayProductionToken(ctx context.Context, client *http.Client) (string, error) {
	if token := strings.TrimSpace(os.Getenv("EBAY_OAUTH_TOKEN")); token != "" {
		return token, nil
	}
	clientID := strings.TrimSpace(os.Getenv("EBAY_CLIENT_ID"))
	clientSecret := strings.TrimSpace(os.Getenv("EBAY_CLIENT_SECRET"))
	if clientID == "" || clientSecret == "" {
		return "", errors.New("eBay production authorization required: configure EBAY_OAUTH_TOKEN or EBAY_CLIENT_ID and EBAY_CLIENT_SECRET")
	}
	form := url.Values{"grant_type": {"client_credentials"}, "scope": {ebayOAuthScope}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ebayProductionAPIBase+"/identity/v1/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(clientID, clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var response ebayApplicationTokenResponse
	if err := doEbayJSON(client, req, &response); err != nil {
		return "", fmt.Errorf("eBay OAuth failed: %w", err)
	}
	if strings.TrimSpace(response.AccessToken) == "" {
		return "", errors.New("eBay OAuth returned no access token")
	}
	return response.AccessToken, nil
}

func fetchEbayBrowseSnapshot(ctx context.Context, client *http.Client, token string, request CapabilityEbayResearchRequest) (ebayBrowseSearchResponse, error) {
	endpoint, _ := url.Parse(ebayProductionAPIBase + "/buy/browse/v1/item_summary/search")
	query := endpoint.Query()
	query.Set("q", request.Query)
	query.Set("limit", strconv.Itoa(request.Limit))
	endpoint.RawQuery = query.Encode()
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return ebayBrowseSearchResponse{}, err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+token)
	httpRequest.Header.Set("X-EBAY-C-MARKETPLACE-ID", request.Marketplace)
	var response ebayBrowseSearchResponse
	if err := doEbayJSON(client, httpRequest, &response); err != nil {
		return ebayBrowseSearchResponse{}, fmt.Errorf("eBay Browse search failed: %w", err)
	}
	if len(response.ItemSummaries) == 0 {
		return ebayBrowseSearchResponse{}, errors.New("eBay Browse returned no item summaries for this query")
	}
	return response, nil
}

func doEbayJSON(client *http.Client, request *http.Request, target any) error {
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
		return fmt.Errorf("HTTP %d", response.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 2<<20))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func ebaySnapshotProjection(scope CapabilityTaskScope, request CapabilityEbayResearchRequest, snapshot ebayBrowseSearchResponse, capturedAt time.Time) (capabilityEvidenceArtifact, capabilityDashboardWidget, error) {
	prices := make([]float64, 0, len(snapshot.ItemSummaries))
	currency := "USD"
	conditions := make(map[string]int)
	for _, item := range snapshot.ItemSummaries {
		value, err := strconv.ParseFloat(item.Price.Value, 64)
		if err == nil && value >= 0 {
			prices = append(prices, value)
		}
		if item.Price.Currency != "" {
			currency = item.Price.Currency
		}
		condition := strings.TrimSpace(item.Condition)
		if condition == "" {
			condition = "未标注"
		}
		conditions[condition]++
	}
	if len(prices) == 0 {
		return capabilityEvidenceArtifact{}, capabilityDashboardWidget{}, errors.New("eBay Browse sample contains no parseable prices")
	}
	sort.Float64s(prices)
	median := prices[len(prices)/2]
	queryHash := sha256.Sum256([]byte(request.Marketplace + "\x00" + strings.ToLower(request.Query)))
	evidenceRef := "ev:ebay:live-browse:" + hex.EncodeToString(queryHash[:6])
	facts := []string{
		fmt.Sprintf("官方 Browse 搜索结果总数 %d", snapshot.Total),
		fmt.Sprintf("本次返回样本 %d 条", len(snapshot.ItemSummaries)),
		fmt.Sprintf("样本价格中位数 %.2f %s", median, currency),
		fmt.Sprintf("样本价格范围 %.2f–%.2f %s", prices[0], prices[len(prices)-1], currency),
		"这是当前在售搜索快照，不是成交量、GMV 或月销量",
	}
	artifact := capabilityEvidenceArtifact{
		EvidenceRef: evidenceRef,
		Scope:       capabilityEvidenceScope{CapabilityID: scope.CapabilityID, WorkspaceID: scope.WorkspaceID, TaskID: scope.TaskID},
		Title:       "eBay US 官方在售快照 · " + request.Query,
		Summary:     fmt.Sprintf("eBay Browse API 在 %s 返回 %d 个搜索结果；当前样本 %d 条，价格中位数 %.2f %s。", request.Marketplace, snapshot.Total, len(snapshot.ItemSummaries), median, currency),
		Facts:       facts,
		Provenance:  capabilityEvidenceProvenance{Source: "eBay", CapturedAt: capturedAt.Format(time.RFC3339Nano), DataClass: "live-authorized"},
	}
	props, _ := json.Marshal(map[string]any{
		"kind": "trend.chart", "eyebrow": "eBay 官方 Browse · 实时", "label": request.Query + " 在售搜索快照",
		"range": capturedAt.Format("2006-01-02 15:04 UTC"), "value": fmt.Sprintf("%d", snapshot.Total), "valueLabel": "搜索结果",
		"change":       fmt.Sprintf("样本中位价 %.2f %s", median, currency),
		"availability": map[string]string{"label": "官方实时数据", "detail": "当前在售快照；不代表成交量或月销量"},
	})
	widget := capabilityDashboardWidget{
		ID: "ebay-live-market-snapshot", Type: "trend.chart", Version: "1.0.0", Surface: "research-canvas", Status: "ready",
		Title: "eBay US 实时在售快照", Props: props, EvidenceRefs: []string{evidenceRef}, Revision: 1,
	}
	return artifact, widget, nil
}

func (a *App) persistCapabilityLiveProjection(active capabilityActiveContext, scope CapabilityTaskScope, artifact capabilityEvidenceArtifact, widget capabilityDashboardWidget, capturedAt time.Time) (string, string, error) {
	a.capabilityDashboardMu.Lock()
	if err := validateCapabilityBinding(active, scope); err != nil {
		a.capabilityDashboardMu.Unlock()
		return "", "", err
	}
	var evidence capabilityEvidenceDocument
	if err := readCapabilityJSON(capabilityEvidencePath(active.stateRoot, scope), &evidence); err != nil {
		a.capabilityDashboardMu.Unlock()
		return "", "", err
	}
	replaced := false
	for index := range evidence.Artifacts {
		if evidence.Artifacts[index].EvidenceRef == artifact.EvidenceRef {
			evidence.Artifacts[index] = artifact
			replaced = true
			break
		}
	}
	if !replaced {
		evidence.Artifacts = append(evidence.Artifacts, artifact)
	}
	evidence.Revision++
	evidence.UpdatedAt = capturedAt.Format(time.RFC3339Nano)
	if err := validateCapabilityEvidenceDocument(evidence, scope); err != nil {
		a.capabilityDashboardMu.Unlock()
		return "", "", err
	}

	var dashboard capabilityDashboardDocument
	if err := readCapabilityJSON(capabilityDashboardPath(active.stateRoot, scope), &dashboard); err != nil {
		a.capabilityDashboardMu.Unlock()
		return "", "", err
	}
	if index := findCapabilityWidget(dashboard.Widgets, widget.ID); index >= 0 {
		widget.Revision = dashboard.Widgets[index].Revision + 1
		dashboard.Widgets[index] = widget
	} else {
		dashboard.Widgets = append(dashboard.Widgets, widget)
	}
	dashboard.Revision++
	dashboard.UpdatedAt = capturedAt.Format(time.RFC3339Nano)
	if err := validateCapabilityDocument(dashboard, scope); err != nil {
		a.capabilityDashboardMu.Unlock()
		return "", "", err
	}
	if err := writeCapabilityJSONAtomic(capabilityEvidencePath(active.stateRoot, scope), evidence); err != nil {
		a.capabilityDashboardMu.Unlock()
		return "", "", err
	}
	if err := writeCapabilityJSONAtomic(capabilityDashboardPath(active.stateRoot, scope), dashboard); err != nil {
		a.capabilityDashboardMu.Unlock()
		return "", "", err
	}
	evidenceJSON, evidenceErr := marshalCapabilityEvidenceDocument(evidence)
	dashboardJSON, dashboardErr := marshalCapabilityDocument(dashboard)
	a.capabilityDashboardMu.Unlock()
	if evidenceErr != nil {
		return "", "", evidenceErr
	}
	if dashboardErr != nil {
		return "", "", dashboardErr
	}
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, capabilityDashboardChangedEvent, scope)
	}
	return evidenceJSON, dashboardJSON, nil
}

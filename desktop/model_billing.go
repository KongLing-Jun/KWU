package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"kwu/internal/airclaude"
	"kwu/internal/boot"
	"kwu/internal/brand"
	"kwu/internal/cloud"
	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/expert"
	"kwu/internal/modelbilling"
	"kwu/internal/modelbillingworker"
	"kwu/internal/netclient"
	"kwu/internal/newapipricing"
	"kwu/internal/tool"
	"kwu/internal/userctx"
)

const (
	desktopCallEvidencePricingSource  = "domestic"
	desktopUniversalSubscriptionModel = "deepseek-v4-flash"
	turnBillingSettledEvent           = "turn-billing:settled"
	desktopSubscriptionModelRetryBase = 250 * time.Millisecond
	desktopSubscriptionModelRetryMax  = 60 * time.Second
)

type TurnBillingSettledView struct {
	TaskID          string `json:"taskId"`
	ChargedCredits  string `json:"chargedCredits"`
	quotaPerUnit    string
	usdExchangeRate string
	creditPerCNY    string
}

type turnBillingSettledSender func(TurnBillingSettledView)

var desktopAirClaudeSubscriptionModels = []string{
	"gpt-5.4",
	"gpt-5.4-mini",
	"deepseek-v4-flash",
	"deepseek-v4-pro",
}

type desktopSubscriptionModelCatalog struct {
	snapshot atomic.Pointer[desktopSubscriptionModelCatalogState]
	fetch    func(context.Context, netclient.ProxySpec, string) ([]string, error)
}

type desktopSubscriptionModelCatalogState struct {
	refreshEpoch uint64
	scope        cloud.BindingScope
	status       cloud.MaaSKeyStatus
	availability string
	key          string
	models       []string
}

func newDesktopSubscriptionModelCatalog() *desktopSubscriptionModelCatalog {
	return &desktopSubscriptionModelCatalog{fetch: airclaude.FetchChatModels}
}

func (c *desktopSubscriptionModelCatalog) current(scope cloud.BindingScope) []string {
	if c == nil {
		return nil
	}
	state := c.snapshot.Load()
	if state == nil || state.scope != scope || state.status != cloud.MaaSKeyReady || state.key == "" {
		return nil
	}
	return append([]string(nil), state.models...)
}

func (c *desktopSubscriptionModelCatalog) fetchModels(ctx context.Context, proxy netclient.ProxySpec, key string) ([]string, error) {
	if c == nil || c.fetch == nil {
		return nil, fmt.Errorf("AirClaude model catalog is unavailable")
	}
	models, err := c.fetch(ctx, proxy, key)
	if err != nil {
		return nil, err
	}
	models = normalizedDesktopSubscriptionModels(models)
	if len(models) == 0 {
		return nil, fmt.Errorf("AirClaude model catalog is empty")
	}
	return models, nil
}

func (c *desktopSubscriptionModelCatalog) replace(scope cloud.BindingScope, account cloud.MaaSKeyAccount, models []string) bool {
	if c == nil {
		return false
	}
	for {
		current := c.snapshot.Load()
		next, changed := desktopSubscriptionModelCatalogReplacement(current, scope, account, models)
		if c.snapshot.CompareAndSwap(current, next) {
			return changed
		}
	}
}

func desktopSubscriptionModelCatalogReplacement(current *desktopSubscriptionModelCatalogState, scope cloud.BindingScope, account cloud.MaaSKeyAccount, models []string) (*desktopSubscriptionModelCatalogState, bool) {
	next := &desktopSubscriptionModelCatalogState{
		scope: scope, status: account.Status, availability: string(account.Status), key: account.Key,
		models: append([]string(nil), models...),
	}
	if current == nil {
		return next, true
	}
	next.refreshEpoch = current.refreshEpoch
	changed := current.scope != scope || current.status != account.Status || current.availability != string(account.Status) ||
		current.key != account.Key || !slices.Equal(current.models, models)
	return next, changed
}

func (c *desktopSubscriptionModelCatalog) advanceRefreshEpoch(epoch uint64) {
	if c == nil || epoch == 0 {
		return
	}
	for {
		current := c.snapshot.Load()
		if current != nil && epoch <= current.refreshEpoch {
			return
		}
		next := &desktopSubscriptionModelCatalogState{refreshEpoch: epoch}
		if current != nil {
			*next = *current
			next.refreshEpoch = epoch
			next.models = append([]string(nil), current.models...)
		}
		if c.snapshot.CompareAndSwap(current, next) {
			return
		}
	}
}

func (c *desktopSubscriptionModelCatalog) replaceForRefresh(epoch uint64, scope cloud.BindingScope, account cloud.MaaSKeyAccount, models []string) (bool, bool) {
	if c == nil || epoch == 0 {
		return false, false
	}
	for {
		current := c.snapshot.Load()
		if current == nil || current.refreshEpoch != epoch {
			return false, false
		}
		next, changed := desktopSubscriptionModelCatalogReplacement(current, scope, account, models)
		if c.snapshot.CompareAndSwap(current, next) {
			return changed, true
		}
	}
}

// replaceOnCatalogFetchError keeps a same-account catalog that was already
// usable. On a first refresh (or a key/scope change), it installs only the
// built-in subscription allowlist with the freshly authenticated MaaS key.
func (c *desktopSubscriptionModelCatalog) replaceOnCatalogFetchError(scope cloud.BindingScope, account cloud.MaaSKeyAccount, fallback []string) bool {
	if c == nil {
		return false
	}
	fallback = normalizedDesktopSubscriptionModels(fallback)
	for {
		current := c.snapshot.Load()
		next, changed := desktopSubscriptionModelCatalogFallback(current, scope, account, fallback)
		if next == current || c.snapshot.CompareAndSwap(current, next) {
			return changed
		}
	}
}

func desktopSubscriptionModelCatalogFallback(current *desktopSubscriptionModelCatalogState, scope cloud.BindingScope, account cloud.MaaSKeyAccount, fallback []string) (*desktopSubscriptionModelCatalogState, bool) {
	if current != nil && current.scope == scope && current.status == cloud.MaaSKeyReady && current.key == account.Key && current.key != "" && len(current.models) > 0 {
		return current, false
	}
	models := []string(nil)
	if account.Status == cloud.MaaSKeyReady && account.Key != "" {
		models = fallback
	}
	return desktopSubscriptionModelCatalogReplacement(current, scope, account, models)
}

func (c *desktopSubscriptionModelCatalog) replaceOnCatalogFetchErrorForRefresh(epoch uint64, scope cloud.BindingScope, account cloud.MaaSKeyAccount, fallback []string) (bool, bool) {
	if c == nil || epoch == 0 {
		return false, false
	}
	fallback = normalizedDesktopSubscriptionModels(fallback)
	for {
		current := c.snapshot.Load()
		if current == nil || current.refreshEpoch != epoch {
			return false, false
		}
		next, changed := desktopSubscriptionModelCatalogFallback(current, scope, account, fallback)
		if next == current || c.snapshot.CompareAndSwap(current, next) {
			return changed, true
		}
	}
}

func (c *desktopSubscriptionModelCatalog) markError(scope cloud.BindingScope) {
	if c == nil {
		return
	}
	for {
		current := c.snapshot.Load()
		next := desktopSubscriptionModelCatalogError(current, scope)
		if c.snapshot.CompareAndSwap(current, next) {
			return
		}
	}
}

func desktopSubscriptionModelCatalogError(current *desktopSubscriptionModelCatalogState, scope cloud.BindingScope) *desktopSubscriptionModelCatalogState {
	next := &desktopSubscriptionModelCatalogState{scope: scope, availability: "error"}
	if current != nil {
		next.refreshEpoch = current.refreshEpoch
	}
	return next
}

func (c *desktopSubscriptionModelCatalog) markErrorForRefresh(epoch uint64, scope cloud.BindingScope) (bool, bool) {
	if c == nil || epoch == 0 {
		return false, false
	}
	for {
		current := c.snapshot.Load()
		if current == nil || current.refreshEpoch != epoch {
			return false, false
		}
		if current.scope == scope && current.status == cloud.MaaSKeyReady && current.key != "" && len(current.models) > 0 {
			return false, true
		}
		next := desktopSubscriptionModelCatalogError(current, scope)
		changed := current.scope != scope || current.status != "" || current.availability != "error" || current.key != "" || len(current.models) != 0
		if c.snapshot.CompareAndSwap(current, next) {
			return changed, true
		}
	}
}

func (c *desktopSubscriptionModelCatalog) credential(scope cloud.BindingScope) (string, bool) {
	if c == nil {
		return "", false
	}
	state := c.snapshot.Load()
	if state == nil || state.scope != scope || state.status != cloud.MaaSKeyReady || state.key == "" || len(state.models) == 0 {
		return "", false
	}
	return state.key, true
}

func (c *desktopSubscriptionModelCatalog) state(scope cloud.BindingScope) (string, []string) {
	if c == nil {
		return "anonymous", nil
	}
	state := c.snapshot.Load()
	if state == nil || state.scope != scope {
		return "preparing", nil
	}
	availability := state.availability
	if availability == "" {
		availability = "preparing"
	}
	return availability, append([]string(nil), state.models...)
}

func (c *desktopSubscriptionModelCatalog) clear() {
	if c == nil {
		return
	}
	for {
		current := c.snapshot.Load()
		next := &desktopSubscriptionModelCatalogState{}
		if current != nil {
			next.refreshEpoch = current.refreshEpoch
		}
		if c.snapshot.CompareAndSwap(current, next) {
			return
		}
	}

}

func (c *desktopSubscriptionModelCatalog) clearForRefresh(epoch uint64) (bool, bool) {
	if c == nil || epoch == 0 {
		return false, false
	}
	for {
		current := c.snapshot.Load()
		if current != nil && current.refreshEpoch > epoch {
			return false, false
		}
		changed := current != nil && (current.scope != (cloud.BindingScope{}) || current.status != "" || current.availability != "" || current.key != "" || len(current.models) != 0)
		if c.snapshot.CompareAndSwap(current, &desktopSubscriptionModelCatalogState{refreshEpoch: epoch}) {
			return changed, true
		}
	}
}

// These values are the current local worker scheduling inputs. They are not a
// cloud SLA and remain explicit so the later real-cloud integration can
// replace them without changing the worker state machine.
const (
	desktopBillingRetryBaseDelay       = 5 * time.Second
	desktopBillingRetryMaxDelay        = 5 * time.Minute
	desktopBillingRetryJitterFraction  = 0.2
	desktopBillingLeaseDuration        = 30 * time.Second
	desktopBillingPollInterval         = time.Second
	desktopBillingOperationTimeout     = 5 * time.Second
	desktopBillingMaxDeliveriesPerPass = 32
	desktopBillingShutdownTimeout      = 12 * time.Second
)

func desktopFixtureTimestamp(hour, minute int) time.Time {
	return time.Date(2026, time.July, 11, hour, minute, 0, 0, time.UTC)
}

func newDesktopFixturePairClock(first, second time.Time) func() time.Time {
	var mu sync.Mutex
	nextIsFirst := true
	return func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		value := second
		if nextIsFirst {
			value = first
		}
		nextIsFirst = !nextIsFirst
		return value
	}
}

func desktopCloudFakeEnabled() bool { return os.Getenv("KWU_CLOUD_FAKE") == "1" }

// desktopBillingRouteLoadPolicy decides whether provider credentials may be
// loaded before any configuration loader runs. Subscription routes always use
// metadata-only provider configuration and must never fall through to BYOK.
type desktopBillingRouteLoadPolicy struct {
	metadataOnly bool
	enableB2B    bool
}

func desktopBillingRouteLoadPolicyFor(source modelbilling.BillingSource, catalog bool) (desktopBillingRouteLoadPolicy, error) {
	source = modelbilling.BillingSource(strings.TrimSpace(string(source)))
	if desktopCloudFakeEnabled() {
		switch source {
		case modelbilling.BillingSubscriptionPool:
			return desktopBillingRouteLoadPolicy{metadataOnly: true, enableB2B: true}, nil
		case modelbilling.BillingNewAPIByok:
			return desktopBillingRouteLoadPolicy{enableB2B: true}, nil
		case "":
			if catalog {
				return desktopBillingRouteLoadPolicy{metadataOnly: true}, nil
			}
		}
		return desktopBillingRouteLoadPolicy{}, &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "desktop B2B explicit billing route is required"}
	}

	if source == modelbilling.BillingSubscriptionPool {
		return desktopBillingRouteLoadPolicy{metadataOnly: true, enableB2B: true}, nil
	}
	// Keep ordinary non-fake legacy and BYOK behavior unchanged.
	return desktopBillingRouteLoadPolicy{}, nil
}

type desktopTaskContextInitializer struct {
	inner modelbilling.TaskContextInitializer
}

func (i desktopTaskContextInitializer) InitializeTaskContext(ctx context.Context, request control.TaskContextInitRequest) (modelbilling.TaskContext, error) {
	return i.inner.InitializeTaskContext(ctx, request.Actor, request.BillingRouteSelection)
}

type desktopBillingRouteResolver struct {
	cfg                  *config.Config
	subscriptionModels   map[string]bool
	subscriptionProvider string
}

func (r desktopBillingRouteResolver) ResolveBillingRoute(_ context.Context, _ userctx.ActorContext, selection modelbilling.BillingRouteSelection) (modelbilling.ResolvedBillingRoute, error) {
	if r.cfg == nil {
		return modelbilling.ResolvedBillingRoute{}, fmt.Errorf("desktop B2B route resolver configuration is required")
	}
	entry, ok := r.cfg.ResolveModel(selection.ModelRef)
	if !ok {
		return modelbilling.ResolvedBillingRoute{}, fmt.Errorf("selected model %q has no profile provider", selection.ModelRef)
	}
	route := modelbilling.ResolvedBillingRoute{ModelRef: entry.Name + "/" + entry.Model, ModelID: entry.Model, Provider: entry.Name, BillingSource: selection.BillingSource}
	if route.ModelRef != selection.ModelRef {
		return modelbilling.ResolvedBillingRoute{}, fmt.Errorf("selected model %q is not canonical", selection.ModelRef)
	}
	if selection.BillingSource == modelbilling.BillingSubscriptionPool {
		if !r.subscriptionModels[entry.Model] {
			return modelbilling.ResolvedBillingRoute{}, fmt.Errorf("subscription gateway does not support model %q", entry.Model)
		}
		route.Provider = r.subscriptionProvider
		if route.Provider == "" {
			return modelbilling.ResolvedBillingRoute{}, fmt.Errorf("subscription gateway provider identity is required")
		}
		return route, nil
	}
	if !entry.Configured() {
		return modelbilling.ResolvedBillingRoute{}, fmt.Errorf("selected model %q has no configured profile provider or credential", selection.ModelRef)
	}
	return route, nil
}

type desktopSubscriptionAuthorizer interface {
	authorizeSubscription(userctx.ActorContext, cloud.BindingScope) error
}

type desktopSubscriptionBindingSource interface {
	desktopSubscriptionAuthorizer
	bindingForController(userctx.ActorContext) (cloud.BindingScope, bool)
}

func currentDesktopSubscriptionBinding(actor userctx.ActorContext, source desktopSubscriptionBindingSource) (cloud.BindingScope, error) {
	if source == nil {
		return cloud.BindingScope{}, &modelbilling.Error{Code: modelbilling.ErrorAuthRequired, Message: "desktop subscription authorization is required"}
	}
	binding, ok := source.bindingForController(actor)
	if !ok {
		return cloud.BindingScope{}, &modelbilling.Error{Code: modelbilling.ErrorAuthRequired, Message: "subscription binding is required"}
	}
	return binding, nil
}

func authorizeDesktopSubscriptionBinding(actor userctx.ActorContext, binding cloud.BindingScope, source desktopSubscriptionBindingSource) error {
	current, err := currentDesktopSubscriptionBinding(actor, source)
	if err != nil {
		return err
	}
	if binding != current {
		return &modelbilling.Error{Code: modelbilling.ErrorScopeMismatch, Message: "subscription binding is not the current desktop scope"}
	}
	return source.authorizeSubscription(actor, current)
}

type desktopFixtureSubscriptionBootstrap struct {
	authorization desktopSubscriptionBindingSource
}

type desktopRemoteSubscriptionBootstrap struct {
	runtime       *cloudRuntime
	authorization desktopSubscriptionBindingSource
}

func (r desktopRemoteSubscriptionBootstrap) ResolveSubscriptionBootstrap(ctx context.Context, actor userctx.ActorContext, _ modelbilling.ResolvedBillingRoute) (modelbilling.SubscriptionBootstrap, error) {
	binding, err := currentDesktopSubscriptionBinding(actor, r.authorization)
	if err != nil {
		return modelbilling.SubscriptionBootstrap{}, err
	}
	if err := r.authorization.authorizeSubscription(actor, binding); err != nil {
		return modelbilling.SubscriptionBootstrap{}, err
	}
	if r.runtime == nil || r.runtime.remoteAccount == nil {
		return modelbilling.SubscriptionBootstrap{}, &modelbilling.Error{Code: modelbilling.ErrorAuthRequired, Message: "cloud account session is unavailable"}
	}
	account, err := r.runtime.remoteAccount.CurrentUser(ctx, actor.Normalize())
	if err != nil {
		return modelbilling.SubscriptionBootstrap{}, err
	}
	accountScope, err := account.Binding.Scope()
	if err != nil {
		return modelbilling.SubscriptionBootstrap{}, err
	}
	if account.State != cloud.AccountAuthenticated || accountScope != binding || account.Subscription.Scope != binding {
		return modelbilling.SubscriptionBootstrap{}, &modelbilling.Error{Code: modelbilling.ErrorScopeMismatch, Message: "cloud subscription does not match the active desktop binding"}
	}
	return modelbilling.SubscriptionBootstrap{Scope: binding, Plan: account.Subscription.Plan}, nil
}

func (r desktopFixtureSubscriptionBootstrap) ResolveSubscriptionBootstrap(_ context.Context, actor userctx.ActorContext, _ modelbilling.ResolvedBillingRoute) (modelbilling.SubscriptionBootstrap, error) {
	binding, err := currentDesktopSubscriptionBinding(actor, r.authorization)
	if err != nil {
		return modelbilling.SubscriptionBootstrap{}, err
	}
	if err := r.authorization.authorizeSubscription(actor, binding); err != nil {
		return modelbilling.SubscriptionBootstrap{}, err
	}
	if actor.ProfileID != binding.ProfileID {
		return modelbilling.SubscriptionBootstrap{}, fmt.Errorf("desktop B2B fixture binding does not match profile %q", actor.ProfileID)
	}
	return modelbilling.SubscriptionBootstrap{Scope: binding, Plan: "plus"}, nil
}

type desktopSubscriptionGateway interface {
	modelbilling.SubscriptionModelGateway
	modelbilling.SubscriptionGatewayEndpointResolver
}

type desktopAuthorizedSubscriptionGateway struct {
	inner         desktopSubscriptionGateway
	materialize   func(cloud.BindingScope) (desktopSubscriptionGateway, error)
	actor         userctx.ActorContext
	authorization desktopSubscriptionBindingSource
}

func (g desktopAuthorizedSubscriptionGateway) gateway(binding cloud.BindingScope) (desktopSubscriptionGateway, error) {
	if g.materialize != nil {
		return g.materialize(binding)
	}
	if g.inner == nil {
		return nil, &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "desktop subscription gateway is required"}
	}
	return g.inner, nil
}

func (g desktopAuthorizedSubscriptionGateway) EndpointFor(request modelbilling.GatewayRequest) (modelbilling.MaasEndpoint, error) {
	binding, err := currentDesktopSubscriptionBinding(g.actor, g.authorization)
	if err != nil {
		return "", err
	}
	if err := g.authorization.authorizeSubscription(g.actor, binding); err != nil {
		return "", err
	}
	gateway, err := g.gateway(binding)
	if err != nil {
		return "", err
	}
	return gateway.EndpointFor(request)
}

func (g desktopAuthorizedSubscriptionGateway) Stream(ctx context.Context, binding cloud.BindingScope, request modelbilling.GatewayRequest) (<-chan modelbilling.GatewayChunk, error) {
	if err := authorizeDesktopSubscriptionBinding(g.actor, binding, g.authorization); err != nil {
		return nil, err
	}
	gateway, err := g.gateway(binding)
	if err != nil {
		return nil, err
	}
	return gateway.Stream(ctx, binding, request)
}

type desktopFixtureCreditConfigProvider struct {
	inner         modelbilling.CreditConfigProvider
	actor         userctx.ActorContext
	authorization desktopSubscriptionBindingSource
}

type desktopMaasCreditConfigProvider struct {
	actor         userctx.ActorContext
	authorization desktopSubscriptionBindingSource
	now           func() time.Time
}

type desktopQuotaCallEvidencePreparer struct{ now func() time.Time }

func (p desktopQuotaCallEvidencePreparer) PrepareForCall(ctx context.Context, input modelbilling.CallEvidencePreparationInput) (modelbilling.PricingEvidence, error) {
	if err := ctx.Err(); err != nil {
		return modelbilling.PricingEvidence{}, err
	}
	now := time.Now
	if p.now != nil {
		now = p.now
	}
	return modelbilling.PricingEvidence{
		EvidenceID: input.EvidenceID, SnapshotID: "maas-quota-compat-v1:" + input.Task.TaskID,
		Scope: input.Task.Scope, TaskID: input.Task.TaskID, ModelID: input.ModelID,
		PricingGroup:   input.InitialEvidence.PricingGroup,
		PricingVersion: cloneDesktopOptionalString(input.InitialEvidence.PricingVersion),
		State:          modelbilling.PricingUnavailable, CapturedAt: now().UTC(),
	}, nil
}

func cloneDesktopOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func optionalDesktopDecimal(value *modelbilling.DecimalString) string {
	if value == nil {
		return ""
	}
	return string(*value)
}

func (p desktopMaasCreditConfigProvider) GetCreditConfig(ctx context.Context, scope cloud.BindingScope) (modelbilling.CreditConfig, error) {
	if err := ctx.Err(); err != nil {
		return modelbilling.CreditConfig{}, err
	}
	if err := authorizeDesktopSubscriptionBinding(p.actor, scope, p.authorization); err != nil {
		return modelbilling.CreditConfig{}, err
	}
	now := time.Now
	if p.now != nil {
		now = p.now
	}
	version := "maas-quota-v1"
	return modelbilling.CreditConfig{
		CreditPerCNY: modelbilling.QuotaSettlementCreditPerCNY,
		PricingGroup: "default", Version: &version, FetchedAt: now().UTC(),
	}, nil
}

type desktopAuthorizedCreditClient struct {
	inner         cloud.CloudCreditClient
	actor         userctx.ActorContext
	authorization desktopSubscriptionBindingSource
}

func (c desktopAuthorizedCreditClient) CreateFreeze(ctx context.Context, request cloud.FreezeRequest) (cloud.FreezeResult, error) {
	if err := authorizeDesktopSubscriptionBinding(c.actor, request.Scope, c.authorization); err != nil {
		return cloud.FreezeResult{}, err
	}
	if c.inner == nil {
		return cloud.FreezeResult{}, &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "cloud freeze client is unavailable"}
	}
	return c.inner.CreateFreeze(ctx, request)
}

func (c desktopAuthorizedCreditClient) GetFreeze(ctx context.Context, scope cloud.BindingScope, freezeID string) (cloud.FreezeResult, error) {
	if err := authorizeDesktopSubscriptionBinding(c.actor, scope, c.authorization); err != nil {
		return cloud.FreezeResult{}, err
	}
	if c.inner == nil {
		return cloud.FreezeResult{}, &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "cloud freeze client is unavailable"}
	}
	return c.inner.GetFreeze(ctx, scope, freezeID)
}

func (p desktopFixtureCreditConfigProvider) GetCreditConfig(ctx context.Context, scope cloud.BindingScope) (modelbilling.CreditConfig, error) {
	if err := ctx.Err(); err != nil {
		return modelbilling.CreditConfig{}, err
	}
	if err := authorizeDesktopSubscriptionBinding(p.actor, scope, p.authorization); err != nil {
		return modelbilling.CreditConfig{}, err
	}
	if p.inner == nil {
		return modelbilling.CreditConfig{}, &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "desktop credit config fixture is required"}
	}
	return p.inner.GetCreditConfig(ctx, scope)
}

// desktopFixtureFreezeClient materializes the registered CLOUD A0 success case
// with the task identity generated at runtime. It never reaches a cloud host.
type desktopFixtureFreezeClient struct {
	template      cloud.FreezeFixture
	actor         userctx.ActorContext
	authorization desktopSubscriptionBindingSource
}

// desktopFixtureQuotaSupplementer is the B4-LOCAL normalized supplement fake.
// It appends one audited complete revision and knows nothing about provider
// URL, authentication, wire lookup, rounding, or production retry policy.
type desktopFixtureQuotaSupplementer struct{ scope modelbilling.TaskScope }

func (s desktopFixtureQuotaSupplementer) Supplement(
	ctx context.Context,
	request modelbillingworker.QuotaSupplementRequest,
) (modelbilling.QuotaEvidenceRevision, error) {
	if err := ctx.Err(); err != nil {
		return modelbilling.QuotaEvidenceRevision{}, err
	}
	if !request.Scope.Equal(s.scope) {
		return modelbilling.QuotaEvidenceRevision{}, &modelbilling.Error{Code: modelbilling.ErrorScopeMismatch, Message: "desktop supplement fixture scope mismatch"}
	}
	if err := request.Validate(); err != nil {
		return modelbilling.QuotaEvidenceRevision{}, err
	}
	requestID := fmt.Sprintf("desktop-supplement-%d:%s:%s", len(request.TaskID), request.TaskID, request.CallID)
	if request.CurrentEvidence.OneAPIRequestID != nil {
		requestID = *request.CurrentEvidence.OneAPIRequestID
	}
	next, err := modelbilling.NormalizeQuotaEvidence(modelbilling.QuotaObservationInput{
		Endpoint: request.Endpoint,
		Headers: map[string][]string{
			"X-Oneapi-Request-Id": {requestID},
		},
		FinalJSON:  json.RawMessage(`{"billing":{"quota":0}}`),
		ObservedAt: request.CurrentEvidence.ObservedAt.Add(time.Second),
	})
	if err != nil {
		return modelbilling.QuotaEvidenceRevision{}, err
	}
	next.Revision = request.CurrentEvidence.Revision + 1
	return next, nil
}

func (c desktopFixtureFreezeClient) CreateFreeze(ctx context.Context, request cloud.FreezeRequest) (cloud.FreezeResult, error) {
	if err := ctx.Err(); err != nil {
		return cloud.FreezeResult{}, err
	}
	if err := authorizeDesktopSubscriptionBinding(c.actor, request.Scope, c.authorization); err != nil {
		return cloud.FreezeResult{}, err
	}
	fixture := c.template
	fixture.Request = request
	fixture.Result.Freeze.Scope = request.Scope
	fixture.Result.Freeze.TaskID = request.TaskID
	client, err := cloud.NewFixtureCreditClient([]cloud.FreezeFixture{fixture})
	if err != nil {
		return cloud.FreezeResult{}, err
	}
	return client.CreateFreeze(ctx, request)
}

func (c desktopFixtureFreezeClient) GetFreeze(_ context.Context, binding cloud.BindingScope, _ string) (cloud.FreezeResult, error) {
	if err := authorizeDesktopSubscriptionBinding(c.actor, binding, c.authorization); err != nil {
		return cloud.FreezeResult{}, err
	}
	return cloud.FreezeResult{}, fmt.Errorf("fixture freeze lookup is not used by B2B")
}

type desktopBillingRuntime struct {
	db          *sql.DB
	tasks       *modelbilling.SQLiteTaskStore
	calls       *modelbilling.SQLiteCallStore
	dispatcher  *modelbilling.MainExecutorDispatcher
	initializer desktopTaskContextInitializer
	closeOnce   sync.Once
}

type desktopBillingWorkerRun func(context.Context) error

// desktopBillingWorkerRuntime keeps the long-running coordinator and the
// event-triggered settlement entrypoint on the same worker instance.
type desktopBillingWorkerRuntime struct {
	run         desktopBillingWorkerRun
	settlements cloud.EventSettlementQuotaReconciler
	recovery    modelbilling.UsagePendingRecovery
}

type desktopBillingWorkerFactory func(*sql.DB, modelbilling.TaskScope) (desktopBillingWorkerRuntime, error)
type desktopStartupRecoverScope func(context.Context, *sql.DB, modelbilling.TaskScope) error

// desktopBillingWorkerManager is the physical Desktop owner of the B3C loop.
// Controller runtimes keep their existing short-lived SQLite handles; this
// manager uses one independent handle for the active profile so tab rebuilds
// cannot duplicate or tear down the worker.
type desktopBillingWorkerManager struct {
	operationMu         sync.Mutex
	factory             desktopBillingWorkerFactory
	openDB              func(userctx.ActorContext) (*sql.DB, error)
	recoverScope        desktopStartupRecoverScope
	waitTimeout         time.Duration
	subscriptionEnabled bool

	started         bool
	closed          bool
	lifecycleCtx    context.Context
	lifecycleCancel context.CancelFunc
	db              *sql.DB
	profileID       string
	activeActor     userctx.ActorContext
	active          *desktopBillingWorkerProcess
	authState       cloud.AccountStatus
	bindingStatus   cloud.BindingStatus
	authorized      bool
	authorizedActor userctx.ActorContext
	authorizedScope cloud.BindingScope
	bindingActor    userctx.ActorContext
	bindingScope    cloud.BindingScope
	hasBinding      bool
	byokRecovered   map[string]bool
}

type desktopBillingWorkerProcess struct {
	scope         modelbilling.TaskScope
	settlements   cloud.EventSettlementQuotaReconciler
	recovery      modelbilling.UsagePendingRecovery
	cancel        context.CancelFunc
	done          chan struct{}
	stopRequested bool
	errMu         sync.Mutex
	err           error
}

func newDesktopBillingWorkerManager(factory desktopBillingWorkerFactory) *desktopBillingWorkerManager {
	return &desktopBillingWorkerManager{
		factory: factory, openDB: openDesktopBillingDB, recoverScope: recoverDesktopBillingScope,
		waitTimeout: desktopBillingShutdownTimeout, subscriptionEnabled: true,
		byokRecovered: make(map[string]bool),
	}
}

func (m *desktopBillingWorkerManager) start(ctx context.Context) {
	if m == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	if m.started || m.closed {
		return
	}
	m.lifecycleCtx, m.lifecycleCancel = context.WithCancel(ctx)
	m.started = true
}

func (m *desktopBillingWorkerManager) activate(actor userctx.ActorContext, binding cloud.BindingScope) error {
	if m == nil {
		return nil
	}
	actor = actor.Normalize()
	return m.reconcileAuth(context.Background(), actor, cloud.AuthView{
		State: cloud.AccountAuthenticated,
		Binding: cloud.ProfileBinding{
			ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudUserID: binding.CloudUserID,
			Status: cloud.BindingBound, BindingGeneration: binding.BindingGeneration,
		},
	})
}

// reconcileAuth is the only cloud-session transition boundary. Only an
// authenticated, bound full identity may own the subscription worker.
func (m *desktopBillingWorkerManager) reconcileAuth(ctx context.Context, actor userctx.ActorContext, auth cloud.AuthView) error {
	if m == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	actor = actor.Normalize()
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	m.authState = auth.State
	m.bindingStatus = auth.Binding.Status
	m.authorized = false
	bindingInvalid := auth.State == cloud.AccountExpired ||
		auth.Binding.Status == cloud.BindingExpired ||
		auth.Binding.Status == cloud.BindingMismatch ||
		auth.Binding.Status == cloud.BindingUnbound
	if bindingInvalid {
		m.bindingActor, m.bindingScope, m.hasBinding = userctx.ActorContext{}, cloud.BindingScope{}, false
	} else if auth.Binding.ActorID == actor.ActorID && auth.Binding.ProfileID == actor.ProfileID {
		if scope, scopeErr := auth.Binding.Scope(); scopeErr == nil && auth.Binding.Status != cloud.BindingMismatch {
			m.bindingActor, m.bindingScope, m.hasBinding = actor, scope, true
		}
	}
	if !m.started {
		return nil
	}
	if m.closed {
		return fmt.Errorf("desktop model-billing worker manager is closed")
	}
	if !m.subscriptionEnabled || auth.State != cloud.AccountAuthenticated || auth.Binding.Status != cloud.BindingBound {
		return m.stopActiveLocked()
	}
	if auth.Binding.ActorID != actor.ActorID || auth.Binding.ProfileID != actor.ProfileID {
		return errors.Join(m.stopActiveLocked(), &modelbilling.Error{Code: modelbilling.ErrorScopeMismatch, Message: "cloud binding does not match desktop actor"})
	}
	binding, err := auth.Binding.Scope()
	if err != nil {
		return errors.Join(m.stopActiveLocked(), &modelbilling.Error{Code: modelbilling.ErrorScopeMismatch, Message: err.Error()})
	}
	scope := modelbilling.TaskScope{ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudBinding: &binding}
	if err := scope.Validate(modelbilling.BillingSubscriptionPool); err != nil {
		return errors.Join(m.stopActiveLocked(), err)
	}
	return m.activateLocked(ctx, actor, scope)
}

func (m *desktopBillingWorkerManager) sameScopeRunningLocked(scope modelbilling.TaskScope) bool {
	if m.active == nil || !m.active.scope.Equal(scope) {
		return false
	}
	select {
	case <-m.active.done:
		m.active = nil
		return false
	default:
		return !m.active.stopRequested
	}
}

func (m *desktopBillingWorkerManager) activateLocked(ctx context.Context, actor userctx.ActorContext, scope modelbilling.TaskScope) error {
	if m.sameScopeRunningLocked(scope) {
		m.authorizeLocked(actor, *scope.CloudBinding)
		return nil
	}
	// Invalidate controller and event access before cancellation. A timeout or
	// stop error can therefore never authorize a replacement concurrently.
	m.authorized = false
	if err := m.stopActiveLocked(); err != nil {
		return err
	}
	if m.db != nil && m.profileID != scope.ProfileID {
		if err := m.closeDBLocked(); err != nil {
			return err
		}
	}
	opened := false
	if m.db == nil {
		db, err := m.openDB(actor)
		if err != nil {
			return err
		}
		m.db, m.profileID, opened = db, scope.ProfileID, true
	}
	runtime, err := m.buildRuntime(m.db, scope)
	if err != nil {
		if opened {
			return errors.Join(err, m.closeDBLocked())
		}
		return err
	}
	if m.recoverScope == nil {
		err = fmt.Errorf("desktop startup recovery is required")
	} else {
		err = m.recoverScope(ctx, m.db, cloneDesktopBillingTaskScope(scope))
	}
	if err != nil {
		if opened {
			return errors.Join(err, m.closeDBLocked())
		}
		return err
	}
	m.activeActor = actor
	m.launchLocked(scope, runtime)
	m.authorizeLocked(actor, *scope.CloudBinding)
	return nil
}

func (m *desktopBillingWorkerManager) buildRuntime(db *sql.DB, scope modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
	if m.factory == nil {
		return desktopBillingWorkerRuntime{}, fmt.Errorf("desktop model-billing worker factory is required")
	}
	runtime, err := m.factory(db, cloneDesktopBillingTaskScope(scope))
	if err != nil {
		return desktopBillingWorkerRuntime{}, err
	}
	if runtime.run == nil {
		return desktopBillingWorkerRuntime{}, fmt.Errorf("desktop model-billing worker factory returned no runner")
	}
	return runtime, nil
}

func (m *desktopBillingWorkerManager) authorizeLocked(actor userctx.ActorContext, binding cloud.BindingScope) {
	m.authorized = true
	m.authorizedActor = actor.Normalize()
	m.authorizedScope = binding
}

func (m *desktopBillingWorkerManager) quiesce() error {
	if m == nil {
		return nil
	}
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	m.authorized = false
	m.authState = cloud.AccountExpired
	return m.stopActiveLocked()
}

func (m *desktopBillingWorkerManager) recoverBYOK(ctx context.Context, actor userctx.ActorContext) error {
	if m == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	actor = actor.Normalize()
	key := actor.ActorID + "\x00" + actor.ProfileID
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	if !m.started || m.closed || m.byokRecovered[key] {
		return nil
	}
	if m.active != nil && m.profileID != actor.ProfileID {
		return &modelbilling.Error{Code: modelbilling.ErrorScopeMismatch, Message: "BYOK recovery profile conflicts with active subscription profile"}
	}
	if m.db != nil && m.profileID != actor.ProfileID {
		if err := m.closeDBLocked(); err != nil {
			return err
		}
	}
	opened := false
	if m.db == nil {
		db, err := m.openDB(actor)
		if err != nil {
			return err
		}
		m.db, m.profileID, opened = db, actor.ProfileID, true
	}
	if m.recoverScope == nil {
		return fmt.Errorf("desktop startup recovery is required")
	}
	scope := modelbilling.TaskScope{ActorID: actor.ActorID, ProfileID: actor.ProfileID}
	if err := m.recoverScope(ctx, m.db, scope); err != nil {
		if opened {
			return errors.Join(err, m.closeDBLocked())
		}
		return err
	}
	m.byokRecovered[key] = true
	return nil
}

func (m *desktopBillingWorkerManager) launchLocked(scope modelbilling.TaskScope, runtime desktopBillingWorkerRuntime) {
	workerCtx, cancel := context.WithCancel(m.lifecycleCtx)
	process := &desktopBillingWorkerProcess{
		scope: cloneDesktopBillingTaskScope(scope), settlements: runtime.settlements, recovery: runtime.recovery,
		cancel: cancel, done: make(chan struct{}),
	}
	m.active = process
	go func() {
		err := runtime.run(workerCtx)
		process.errMu.Lock()
		process.err = err
		process.errMu.Unlock()
		close(process.done)
	}()
}

func (m *desktopBillingWorkerManager) authorizeSubscription(actor userctx.ActorContext, binding cloud.BindingScope) error {
	if m == nil {
		return &modelbilling.Error{Code: modelbilling.ErrorAuthRequired, Message: "subscription session is unavailable"}
	}
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	return m.authorizeSubscriptionLocked(actor, binding)
}

func (m *desktopBillingWorkerManager) bindingForController(actor userctx.ActorContext) (cloud.BindingScope, bool) {
	if m == nil {
		return cloud.BindingScope{}, false
	}
	actor = actor.Normalize()
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	if !m.hasBinding || actor.ActorID != m.bindingActor.ActorID || actor.ProfileID != m.bindingActor.ProfileID {
		return cloud.BindingScope{}, false
	}
	return m.bindingScope, true
}

func (a *App) desktopSubscriptionModelsForActor(actor userctx.ActorContext) []string {
	if desktopCloudFakeEnabled() {
		return append([]string(nil), desktopAirClaudeSubscriptionModels...)
	}
	if a == nil || a.subscriptionModels == nil || a.billingWorkerRuntime == nil {
		return nil
	}
	binding, ok := a.billingWorkerRuntime.bindingForController(actor.Normalize())
	if !ok {
		return nil
	}
	return a.subscriptionModels.current(binding)
}

// desktopAuthorizedSubscriptionCatalogForActor is the single UI catalog gate
// for the built-in points channel. A cached MaaS catalog is not visible after
// the authenticated binding stops being authorized.
func (a *App) desktopAuthorizedSubscriptionCatalogForActor(actor userctx.ActorContext) (cloud.BindingScope, []string, bool) {
	if desktopCloudFakeEnabled() {
		return cloud.BindingScope{}, append([]string(nil), desktopAirClaudeSubscriptionModels...), true
	}
	if a == nil || a.subscriptionModels == nil || a.billingWorkerRuntime == nil {
		return cloud.BindingScope{}, nil, false
	}
	actor = actor.Normalize()
	binding, ok := a.billingWorkerRuntime.bindingForController(actor)
	if !ok || a.billingWorkerRuntime.authorizeSubscription(actor, binding) != nil {
		return cloud.BindingScope{}, nil, false
	}
	return binding, a.subscriptionModels.current(binding), true
}

func (a *App) desktopSubscriptionModels() []string {
	if a == nil {
		return nil
	}
	return a.desktopSubscriptionModelsForActor(a.cloudActorContext())
}

type desktopSubscriptionModelRefresh struct {
	status   cloud.MaaSKeyStatus
	changed  bool
	terminal bool
}

func (a *App) refreshDesktopSubscriptionModels(ctx context.Context, actor userctx.ActorContext) error {
	epoch := a.beginDesktopSubscriptionModelRefresh()
	result, err := a.refreshDesktopSubscriptionModelsSnapshot(ctx, actor, epoch)
	if a.subscriptionModelRefreshEpoch.Load() != epoch {
		return &modelbilling.Error{Code: modelbilling.ErrorScopeMismatch, Message: "subscription model refresh was superseded"}
	}
	a.emitDesktopSubscriptionModelCatalogChange(result)
	if err != nil {
		if desktopSubscriptionModelRefreshRetryable(err) {
			a.retryDesktopSubscriptionModels(actor.Normalize(), epoch)
		}
		return err
	}
	if desktopSubscriptionModelProvisioning(result.status) {
		a.retryDesktopSubscriptionModels(actor.Normalize(), epoch)
	}
	return err
}

func (a *App) beginDesktopSubscriptionModelRefresh() uint64 {
	if a == nil || a.subscriptionModels == nil {
		return 0
	}
	epoch := a.subscriptionModelRefreshEpoch.Add(1)
	a.subscriptionModels.advanceRefreshEpoch(epoch)
	return epoch
}

func (a *App) emitDesktopSubscriptionModelCatalogChange(result desktopSubscriptionModelRefresh) {
	if result.changed && (result.status == cloud.MaaSKeyReady || result.terminal) {
		a.emitModelRouteChanged("")
	}
}

func (a *App) refreshDesktopSubscriptionModelsSnapshot(ctx context.Context, actor userctx.ActorContext, epoch uint64) (desktopSubscriptionModelRefresh, error) {
	if desktopCloudFakeEnabled() {
		return desktopSubscriptionModelRefresh{status: cloud.MaaSKeyReady}, nil
	}
	if a == nil || a.subscriptionModels == nil || a.billingWorkerRuntime == nil || a.cloudRuntime == nil || a.cloudRuntime.maasKeys == nil {
		return desktopSubscriptionModelRefresh{}, fmt.Errorf("AirClaude model catalog is unavailable")
	}
	actor = actor.Normalize()
	binding, ok := a.billingWorkerRuntime.bindingForController(actor)
	if !ok {
		return desktopSubscriptionModelRefresh{}, &modelbilling.Error{Code: modelbilling.ErrorAuthRequired, Message: "subscription binding is required"}
	}
	if err := a.billingWorkerRuntime.authorizeSubscription(actor, binding); err != nil {
		return desktopSubscriptionModelRefresh{}, err
	}
	account, err := a.cloudRuntime.maasKeys.Get(ctx, binding)
	if err != nil {
		current, ok := a.billingWorkerRuntime.bindingForController(actor)
		if !ok || current != binding {
			return desktopSubscriptionModelRefresh{}, &modelbilling.Error{Code: modelbilling.ErrorScopeMismatch, Message: "subscription binding changed during model refresh"}
		}
		if cloud.IsSessionTerminalError(err) {
			changed, committed := a.subscriptionModels.clearForRefresh(epoch)
			if !committed {
				return desktopSubscriptionModelRefresh{}, &modelbilling.Error{Code: modelbilling.ErrorScopeMismatch, Message: "subscription model refresh was superseded"}
			}
			return desktopSubscriptionModelRefresh{changed: changed, terminal: true}, err
		}
		if _, committed := a.subscriptionModels.markErrorForRefresh(epoch, binding); !committed {
			return desktopSubscriptionModelRefresh{}, &modelbilling.Error{Code: modelbilling.ErrorScopeMismatch, Message: "subscription model refresh was superseded"}
		}
		return desktopSubscriptionModelRefresh{}, err
	}
	current, ok := a.billingWorkerRuntime.bindingForController(actor)
	if !ok || current != binding {
		return desktopSubscriptionModelRefresh{}, &modelbilling.Error{Code: modelbilling.ErrorScopeMismatch, Message: "subscription binding changed during model refresh"}
	}
	if account.Status != cloud.MaaSKeyReady {
		changed, committed := a.subscriptionModels.replaceForRefresh(epoch, binding, account, nil)
		if !committed {
			return desktopSubscriptionModelRefresh{}, &modelbilling.Error{Code: modelbilling.ErrorScopeMismatch, Message: "subscription model refresh was superseded"}
		}
		return desktopSubscriptionModelRefresh{status: account.Status, changed: changed}, nil
	}
	result := desktopSubscriptionModelRefresh{status: account.Status}
	workspaceRoot, _, _ := a.desktopActiveRouteSnapshot()
	cfg, err := config.LoadForRootMetadataWithActor(workspaceRoot, actor)
	if err != nil {
		changed, scopeErr := a.replaceDesktopSubscriptionModelsAfterFetchError(actor, epoch, binding, account)
		if scopeErr != nil {
			return result, scopeErr
		}
		result.changed = changed
		return result, err
	}
	models, err := a.subscriptionModels.fetchModels(ctx, cfg.OutboundProxySpec(), account.Key)
	if err != nil {
		changed, scopeErr := a.replaceDesktopSubscriptionModelsAfterFetchError(actor, epoch, binding, account)
		if scopeErr != nil {
			return result, scopeErr
		}
		result.changed = changed
		return result, err
	}
	current, ok = a.billingWorkerRuntime.bindingForController(actor)
	if !ok || current != binding {
		return result, &modelbilling.Error{Code: modelbilling.ErrorScopeMismatch, Message: "subscription binding changed during model refresh"}
	}
	changed, committed := a.subscriptionModels.replaceForRefresh(epoch, binding, account, models)
	if !committed {
		return result, &modelbilling.Error{Code: modelbilling.ErrorScopeMismatch, Message: "subscription model refresh was superseded"}
	}
	result.changed = changed
	return result, nil
}

func (a *App) replaceDesktopSubscriptionModelsAfterFetchError(actor userctx.ActorContext, epoch uint64, binding cloud.BindingScope, account cloud.MaaSKeyAccount) (bool, error) {
	current, ok := a.billingWorkerRuntime.bindingForController(actor)
	if !ok || current != binding {
		return false, &modelbilling.Error{Code: modelbilling.ErrorScopeMismatch, Message: "subscription binding changed during model refresh"}
	}
	changed, committed := a.subscriptionModels.replaceOnCatalogFetchErrorForRefresh(epoch, binding, account, desktopAirClaudeSubscriptionModels)
	if !committed {
		return false, &modelbilling.Error{Code: modelbilling.ErrorScopeMismatch, Message: "subscription model refresh was superseded"}
	}
	return changed, nil
}

func (a *App) refreshDesktopSubscriptionModelsBestEffort(actor userctx.ActorContext) {
	// The output-only WorkBuddy RC executable validates Project behavior against
	// an isolated Cloud that intentionally omits MaaS provisioning. Do not let
	// that unrelated optional catalog request terminate its synthetic session.
	if workBuddyRCTestMode() {
		return
	}
	epoch := a.beginDesktopSubscriptionModelRefresh()
	result, err := a.refreshDesktopSubscriptionModelsSnapshot(a.bootContext(), actor, epoch)
	if a.subscriptionModelRefreshEpoch.Load() != epoch {
		return
	}
	a.emitDesktopSubscriptionModelCatalogChange(result)
	if err != nil {
		slog.Warn("AirClaude model catalog refresh failed; keeping the last catalog", "err", err)
		if desktopSubscriptionModelRefreshRetryable(err) {
			a.retryDesktopSubscriptionModels(actor.Normalize(), epoch)
		}
		return
	}
	if desktopSubscriptionModelProvisioning(result.status) {
		a.retryDesktopSubscriptionModels(actor.Normalize(), epoch)
	}
}

func desktopSubscriptionModelRefreshRetryable(err error) bool {
	if err == nil {
		return false
	}
	if cloud.IsSessionTerminalError(err) {
		return false
	}
	switch modelbilling.ErrorCodeOf(err) {
	case modelbilling.ErrorAuthRequired, modelbilling.ErrorSessionExpired, modelbilling.ErrorScopeMismatch:
		return false
	default:
		return true
	}
}

func desktopSubscriptionModelProvisioning(status cloud.MaaSKeyStatus) bool {
	switch status {
	case cloud.MaaSKeyCreatePending, cloud.MaaSKeyCreating, cloud.MaaSKeyRetryWait:
		return true
	default:
		return false
	}
}

func (a *App) retryDesktopSubscriptionModels(actor userctx.ActorContext, epoch uint64) {
	if a == nil || epoch == 0 {
		return
	}
	ctx := a.bootContext()
	go a.retryDesktopSubscriptionModelsWithWait(ctx, actor, epoch, waitDesktopSubscriptionModelRetry)
}

type desktopSubscriptionModelRetryWait func(context.Context, time.Duration) bool

func desktopSubscriptionModelRetryBackoff(attempt uint) time.Duration {
	delay := desktopSubscriptionModelRetryBase
	for step := uint(0); step < attempt && delay < desktopSubscriptionModelRetryMax; step++ {
		delay *= 2
		if delay > desktopSubscriptionModelRetryMax {
			return desktopSubscriptionModelRetryMax
		}
	}
	return delay
}

func waitDesktopSubscriptionModelRetry(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func (a *App) retryDesktopSubscriptionModelsWithWait(ctx context.Context, actor userctx.ActorContext, epoch uint64, wait desktopSubscriptionModelRetryWait) {
	if a == nil || ctx == nil || epoch == 0 || wait == nil {
		return
	}
	for attempt := uint(0); ; attempt++ {
		if !wait(ctx, desktopSubscriptionModelRetryBackoff(attempt)) || a.subscriptionModelRefreshEpoch.Load() != epoch {
			return
		}
		result, err := a.refreshDesktopSubscriptionModelsSnapshot(ctx, actor, epoch)
		if a.subscriptionModelRefreshEpoch.Load() != epoch {
			return
		}
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if cloud.IsSessionTerminalError(err) {
				a.emitDesktopSubscriptionModelCatalogChange(result)
				return
			}
			if !desktopSubscriptionModelRefreshRetryable(err) {
				return
			}
			slog.Warn("AirClaude model catalog retry failed", "err", err)
			continue
		}
		if result.status == cloud.MaaSKeyReady {
			if err := a.restoreAuthenticatedSubscriptionTabs(actor); err != nil {
				slog.Warn("authenticated subscription tab restore after model provisioning failed", "err", err)
			}
			a.emitModelRouteChanged("")
			return
		}
		if !desktopSubscriptionModelProvisioning(result.status) {
			return
		}
	}
}

func (a *App) clearDesktopSubscriptionModels() {
	if a != nil && a.subscriptionModels != nil {
		epoch := a.subscriptionModelRefreshEpoch.Add(1)
		_, _ = a.subscriptionModels.clearForRefresh(epoch)
	}
}

func (a *App) applyDesktopSubscriptionModels(cfg *config.Config, actor userctx.ActorContext) {
	if cfg == nil {
		return
	}
	models := a.desktopSubscriptionModelsForActor(actor)
	if desktopCloudFakeEnabled() {
		models = desktopAirClaudeSubscriptionModels
	} else if workBuddyRCTestMode() && len(models) == 0 {
		models = desktopAirClaudeSubscriptionModels
	}
	applyDesktopSubscriptionCatalog(cfg, models)
}

func (a *App) applyDesktopAuthorizedSubscriptionModels(cfg *config.Config, actor userctx.ActorContext) {
	if desktopCloudFakeEnabled() || cfg == nil {
		return
	}
	_, models, authorized := a.desktopAuthorizedSubscriptionCatalogForActor(actor)
	if !authorized {
		models = nil
	}
	applyDesktopSubscriptionCatalog(cfg, models)
}

func applyDesktopSubscriptionCatalog(cfg *config.Config, models []string) {
	if len(models) == 0 {
		if official, ok := cfg.Provider("kwu"); ok {
			official.Model = ""
			official.Models = nil
			official.Default = ""
		}
		return
	}
	applyDesktopAirClaudeSubscriptionCatalogModels(cfg, models)
}

func (a *App) desktopNewSessionDefaultsForRoute(workspaceRoot string, actor userctx.ActorContext, source modelbilling.BillingSource) (string, string) {
	model, toolApprovalMode := desktopNewSessionDefaultsForRoute(workspaceRoot, actor, source)
	preferenceStore := newDesktopTabStore(desktopConfigDirForPaths(config.NewProfilePathResolver(actor)))
	if persistedMode, ok, err := preferenceStore.defaultToolApprovalMode(); err != nil {
		slog.Warn("load persisted desktop tool approval mode failed", "path", preferenceStore.path(), "err", err)
	} else if ok {
		toolApprovalMode = persistedMode
	}
	if source == modelbilling.BillingSubscriptionPool {
		model = a.availableSubscriptionModel(workspaceRoot, actor, model)
	}
	return model, toolApprovalMode
}

func (a *App) availableSubscriptionModel(workspaceRoot string, actor userctx.ActorContext, preferred string) string {
	preferred = strings.TrimSpace(preferred)
	cfg, err := a.desktopConfigForBillingRoute(workspaceRoot, actor, modelbilling.BillingSubscriptionPool)
	if err != nil || cfg == nil {
		return preferred
	}
	return availableSubscriptionModelFromConfig(cfg, preferred)
}

func availableSubscriptionModelFromConfig(cfg *config.Config, preferred string) string {
	preferred = strings.TrimSpace(preferred)
	if cfg == nil {
		return preferred
	}
	if entry, ok := cfg.ResolveModel(preferred); ok {
		return entry.Name + "/" + entry.Model
	}
	provider, ok := cfg.Provider("kwu")
	if !ok {
		return preferred
	}
	fallback := strings.TrimSpace(provider.Default)
	if fallback == "" {
		models := provider.ModelList()
		if len(models) > 0 {
			fallback = strings.TrimSpace(models[0])
		}
	}
	if fallback == "" {
		return preferred
	}
	return provider.Name + "/" + fallback
}

// defaultBillingSourceForNewTask keeps the stored model and billing source as
// one route. An unavailable official route remains subscription-scoped so it
// fails closed on authentication instead of falling into the legacy BYOK path.
func (a *App) defaultBillingSourceForNewTask(actor userctx.ActorContext) modelbilling.BillingSource {
	if a == nil {
		return ""
	}
	actor = actor.Normalize()
	profile := config.LoadForEditWithoutCredentialsWithPaths(config.NewProfilePathResolver(actor))
	configured := modelbilling.BillingSource(strings.TrimSpace(profile.Desktop.DefaultBillingSource))
	if configured == modelbilling.BillingSubscriptionPool || configured == modelbilling.BillingNewAPIByok {
		return configured
	}
	if configured != "" {
		return ""
	}
	root := strings.TrimSpace(actor.WorkspaceRoot)
	if root == "" {
		root = "."
	}
	cfg, err := config.LoadForRootMetadataWithActor(root, actor)
	if err != nil {
		return ""
	}
	defaultModel := strings.TrimSpace(cfg.DefaultModel)
	providerName, _, qualified := strings.Cut(defaultModel, "/")
	if qualified && providerName == brand.ProviderID {
		return modelbilling.BillingSubscriptionPool
	}
	entry, ok := cfg.ResolveModel(defaultModel)
	if !ok {
		return ""
	}
	if entry.Name == brand.ProviderID {
		return modelbilling.BillingSubscriptionPool
	}
	return modelbilling.BillingNewAPIByok
}

func (a *App) promoteSourceLessActiveTabToSubscription(actor userctx.ActorContext) error {
	if a == nil || a.defaultBillingSourceForNewTask(actor) != modelbilling.BillingSubscriptionPool {
		return nil
	}
	a.mu.RLock()
	tab := a.activeTabLocked()
	if tab == nil || strings.TrimSpace(tab.billingSource) != "" || !tabRuntimeActorMatches(tab, actor, tab.WorkspaceRoot) {
		a.mu.RUnlock()
		return nil
	}
	tabID := tab.ID
	workspaceRoot := tab.WorkspaceRoot
	a.mu.RUnlock()
	model, _ := a.desktopNewSessionDefaultsForRoute(workspaceRoot, actor, modelbilling.BillingSubscriptionPool)
	return a.SetModelRouteForTab(tabID, model, string(modelbilling.BillingSubscriptionPool))
}

func desktopSubscriptionAuthenticationPending(source modelbilling.BillingSource, err error) bool {
	if source != modelbilling.BillingSubscriptionPool {
		return false
	}
	switch modelbilling.ErrorCodeOf(err) {
	case modelbilling.ErrorAuthRequired, modelbilling.ErrorSessionExpired:
		return true
	default:
		return false
	}
}

// restoreAuthenticatedSubscriptionTabs rebuilds only platform tabs that were
// deliberately left without a controller while the user was signed out. It
// reuses the normal route rebuild path so billing/task initialization cannot be
// bypassed after login or registration.
func (a *App) restoreAuthenticatedSubscriptionTabs(actor userctx.ActorContext) error {
	if err := a.promoteSourceLessActiveTabToSubscription(actor); err != nil {
		return err
	}
	actor = actor.Normalize()
	type pendingTab struct {
		id            string
		model         string
		sessionPath   string
		workspaceRoot string
		actor         userctx.ActorContext
	}
	pending := make([]pendingTab, 0)
	a.mu.RLock()
	for _, tab := range a.tabRuntimes {
		if tab == nil || tab.removed || tab.Ctrl != nil || !tab.subscriptionAuthPending ||
			modelbilling.BillingSource(tab.billingSource) != modelbilling.BillingSubscriptionPool ||
			!actorProfilesMatch(tabRuntimeActorContext(tab), actor) {
			continue
		}
		model := strings.TrimSpace(tab.model)
		if model == "" {
			model, _ = a.desktopNewSessionDefaultsForRoute(tab.WorkspaceRoot, tabRuntimeActorContext(tab), modelbilling.BillingSubscriptionPool)
		}
		pending = append(pending, pendingTab{
			id: tab.ID, model: model, sessionPath: tab.SessionPath,
			workspaceRoot: tab.WorkspaceRoot, actor: tabRuntimeActorContext(tab),
		})
	}
	a.mu.RUnlock()

	var restoreErr error
	for _, tab := range pending {
		defaultModel := ""
		if strings.TrimSpace(tab.sessionPath) == "" {
			defaultModel, _ = a.desktopNewSessionDefaultsForRoute(
				tab.workspaceRoot,
				tab.actor,
				modelbilling.BillingSubscriptionPool,
			)
		}
		tab.model = subscriptionModelForAuthenticationRestore(tab.model, tab.sessionPath, defaultModel)
		model := a.availableSubscriptionModel(tab.workspaceRoot, tab.actor, tab.model)
		if err := a.setModelRouteForTab(tab.id, model, modelbilling.BillingSubscriptionPool); err != nil {
			restoreErr = errors.Join(restoreErr, err)
		}
	}
	return restoreErr
}

func subscriptionModelForAuthenticationRestore(currentModel, sessionPath, defaultModel string) string {
	if strings.TrimSpace(sessionPath) == "" {
		if model := strings.TrimSpace(defaultModel); model != "" {
			return model
		}
	}
	return strings.TrimSpace(currentModel)
}

func (m *desktopBillingWorkerManager) authorizeSubscriptionLocked(actor userctx.ActorContext, binding cloud.BindingScope) error {
	actor = actor.Normalize()
	if m.closed {
		return &modelbilling.Error{Code: modelbilling.ErrorSessionExpired, Message: "subscription worker manager is closed"}
	}
	if err := binding.Validate(); err != nil {
		return &modelbilling.Error{Code: modelbilling.ErrorScopeMismatch, Message: err.Error()}
	}
	switch m.bindingStatus {
	case cloud.BindingUnbound, cloud.BindingBinding, "":
		return &modelbilling.Error{Code: modelbilling.ErrorAuthRequired, Message: "subscription binding is required"}
	case cloud.BindingExpired:
		return &modelbilling.Error{Code: modelbilling.ErrorSessionExpired, Message: "subscription binding expired"}
	case cloud.BindingMismatch:
		return &modelbilling.Error{Code: modelbilling.ErrorScopeMismatch, Message: "subscription binding does not match the desktop profile"}
	}
	switch m.authState {
	case cloud.AccountOffline:
		return &modelbilling.Error{Code: modelbilling.ErrorGatewayUnavailable, Message: "subscription session is offline", Retryable: true}
	case cloud.AccountExpired:
		return &modelbilling.Error{Code: modelbilling.ErrorSessionExpired, Message: "subscription session is not online"}
	case cloud.AccountAuthenticated:
		if !m.authorized {
			return &modelbilling.Error{Code: modelbilling.ErrorSessionExpired, Message: "subscription worker is inactive"}
		}
	default:
		return &modelbilling.Error{Code: modelbilling.ErrorAuthRequired, Message: "subscription authentication is required"}
	}
	if actor.ActorID != m.authorizedActor.ActorID || actor.ProfileID != m.authorizedActor.ProfileID || binding != m.authorizedScope {
		return &modelbilling.Error{Code: modelbilling.ErrorScopeMismatch, Message: "subscription binding is not the active desktop scope"}
	}
	if m.active == nil || m.active.scope.CloudBinding == nil || !m.active.scope.Equal(modelbilling.TaskScope{ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudBinding: &binding}) {
		return &modelbilling.Error{Code: modelbilling.ErrorSessionExpired, Message: "subscription worker is not running"}
	}
	select {
	case <-m.active.done:
		m.active = nil
		m.authorized = false
		return &modelbilling.Error{Code: modelbilling.ErrorSessionExpired, Message: "subscription worker stopped"}
	default:
		return nil
	}
}

func (m *desktopBillingWorkerManager) ReconcileSettlementEvent(ctx context.Context, actor userctx.ActorContext, binding cloud.BindingScope, taskID, settlementID string) error {
	if m == nil {
		return &modelbilling.Error{Code: modelbilling.ErrorAuthRequired, Message: "subscription worker is unavailable"}
	}
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	if err := m.authorizeSubscriptionLocked(actor, binding); err != nil {
		return err
	}
	if m.active.settlements == nil {
		return &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "subscription settlement reconciler is unavailable"}
	}
	return m.active.settlements.ReconcileSettlementEvent(ctx, actor.Normalize(), binding, taskID, settlementID)
}

func (m *desktopBillingWorkerManager) RecoverUsagePendingTask(ctx context.Context, scope modelbilling.TaskScope, taskID string) error {
	if m == nil {
		return &modelbilling.Error{Code: modelbilling.ErrorAuthRequired, Message: "subscription worker is unavailable"}
	}
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	if scope.CloudBinding == nil {
		return &modelbilling.Error{Code: modelbilling.ErrorScopeMismatch, Message: "subscription recovery requires a cloud binding"}
	}
	actor := userctx.ActorContext{ActorID: scope.ActorID, ProfileID: scope.ProfileID}.Normalize()
	if err := m.authorizeSubscriptionLocked(actor, *scope.CloudBinding); err != nil {
		return err
	}
	if m.active.recovery == nil {
		return &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "subscription task recovery is unavailable"}
	}
	return m.active.recovery.RecoverUsagePendingTask(ctx, cloneDesktopBillingTaskScope(scope), taskID)
}

func (m *desktopBillingWorkerManager) close() error {
	if m == nil {
		return nil
	}
	m.operationMu.Lock()
	defer m.operationMu.Unlock()
	m.closed = true
	m.authorized = false
	if m.active == nil && m.db == nil {
		return nil
	}
	if m.lifecycleCancel != nil {
		m.lifecycleCancel()
	}
	workerErr := m.stopActiveLocked()
	if m.active != nil {
		// A timed-out worker may still be using the app-owned handle. Preserve it
		// so a later bounded close attempt can finish without racing sql.DB.Close.
		return workerErr
	}
	dbErr := m.closeDBLocked()
	return errors.Join(workerErr, dbErr)
}

func (m *desktopBillingWorkerManager) stopActiveLocked() error {
	process := m.active
	if process == nil {
		return nil
	}
	timeout := m.waitTimeout
	if timeout <= 0 {
		return fmt.Errorf("desktop model-billing worker wait timeout must be positive")
	}
	process.stopRequested = true
	process.cancel()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-process.done:
		m.active = nil
		process.errMu.Lock()
		err := process.err
		process.errMu.Unlock()
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil
		}
		return err
	case <-timer.C:
		return fmt.Errorf("desktop model-billing worker did not stop within %s", timeout)
	}
}

func (m *desktopBillingWorkerManager) closeDBLocked() error {
	if m.db == nil {
		m.profileID = ""
		m.activeActor = userctx.ActorContext{}
		return nil
	}
	err := m.db.Close()
	m.db = nil
	m.profileID = ""
	m.activeActor = userctx.ActorContext{}
	return err
}

func cloneDesktopBillingTaskScope(scope modelbilling.TaskScope) modelbilling.TaskScope {
	cloned := scope
	if scope.CloudBinding != nil {
		binding := *scope.CloudBinding
		cloned.CloudBinding = &binding
	}
	return cloned
}

func (a *App) activateDesktopBillingWorker(actor userctx.ActorContext, binding cloud.BindingScope) error {
	if a == nil || a.billingWorkerRuntime == nil {
		return nil
	}
	return a.billingWorkerRuntime.activate(actor, binding)
}

func recoverDesktopBillingScope(ctx context.Context, db *sql.DB, scope modelbilling.TaskScope) error {
	if db == nil {
		return fmt.Errorf("desktop startup recovery database is required")
	}
	if scope.CloudBinding != nil {
		if err := backfillDesktopAcceptedQuotaRequests(
			ctx, db, scope, defaultDesktopAcceptedQuotaRequestRegistry(),
		); err != nil {
			return err
		}
	}
	recovery, err := modelbillingworker.NewStartupExecutionRecovery(
		modelbilling.NewSQLiteTaskStore(db),
		modelbilling.NewSQLiteCallStore(db),
		time.Now,
	)
	if err != nil {
		return err
	}
	return recovery.RecoverScope(ctx, cloneDesktopBillingTaskScope(scope))
}

func desktopBillingWorkerFactoryForRuntime(runtime func() *cloudRuntime) desktopBillingWorkerFactory {
	if desktopCloudFakeEnabled() {
		return defaultDesktopBillingWorkerFactory
	}
	return func(db *sql.DB, scope modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		if runtime == nil {
			return desktopBillingWorkerRuntime{}, &modelbilling.Error{Code: modelbilling.ErrorAuthRequired, Message: "cloud runtime is unavailable"}
		}
		return newDesktopRealBillingWorkerRuntime(db, scope, runtime())
	}
}

type desktopRemoteSettlementObserver struct {
	runtime     *cloudRuntime
	actor       userctx.ActorContext
	taskScope   modelbilling.TaskScope
	settlements modelbilling.QuotaSettlementStore
}

func (o desktopRemoteSettlementObserver) CreditSettlementObserved(ctx context.Context, scope cloud.BindingScope, taskID, _ string) error {
	if o.runtime == nil || o.runtime.remoteAccount == nil || o.runtime.assets == nil {
		return &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "cloud settlement observer is unavailable"}
	}
	account, err := o.runtime.remoteAccount.CurrentUser(ctx, o.actor.Normalize())
	if err != nil {
		return err
	}
	accountScope, err := account.Binding.Scope()
	if err != nil {
		return err
	}
	if account.State != cloud.AccountAuthenticated || accountScope != scope || account.Subscription.Scope != scope {
		return &modelbilling.Error{Code: modelbilling.ErrorScopeMismatch, Message: "settlement balance refresh returned another binding"}
	}
	if _, err = o.runtime.assets.StoreBalance(ctx, scope, account.Balance); err != nil {
		return err
	}
	o.runtime.assetNotifier.notify(cloud.EventAssetChanges{BalanceChanged: true})
	if o.settlements != nil {
		result, found, settlementErr := o.settlements.CurrentQuotaSettlement(ctx, o.taskScope, taskID)
		if settlementErr == nil && found && result.Settlement.ChargedCredits != nil {
			view := TurnBillingSettledView{TaskID: taskID, ChargedCredits: string(*result.Settlement.ChargedCredits)}
			if result.Settlement.QuotaPerUnit != nil {
				view.quotaPerUnit = string(*result.Settlement.QuotaPerUnit)
			}
			if result.Settlement.USDExchangeRate != nil {
				view.usdExchangeRate = string(*result.Settlement.USDExchangeRate)
			}
			if result.Settlement.CreditPerCNY != nil {
				view.creditPerCNY = string(*result.Settlement.CreditPerCNY)
			}
			o.runtime.notifyTurnBillingSettled(view)
		}
	}
	return nil
}

func newDesktopRealBillingWorkerRuntime(db *sql.DB, scope modelbilling.TaskScope, runtime *cloudRuntime) (desktopBillingWorkerRuntime, error) {
	if runtime == nil || runtime.quota == nil {
		return desktopBillingWorkerRuntime{}, &modelbilling.Error{Code: modelbilling.ErrorAuthRequired, Message: "cloud quota client is unavailable"}
	}
	actor := userctx.ActorContext{ActorID: scope.ActorID, ProfileID: scope.ProfileID}.Normalize()
	observer := desktopRemoteSettlementObserver{
		runtime: runtime, actor: actor, taskScope: cloneDesktopBillingTaskScope(scope),
		settlements: modelbilling.NewSQLiteQuotaSettlementStore(db),
	}
	releaser, ok := runtime.credit.(cloud.CloudCreditFreezeReleaser)
	if !ok {
		return desktopBillingWorkerRuntime{}, &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "cloud freeze release client is unavailable"}
	}
	return buildDesktopBillingWorkerRuntime(db, scope, runtime.quota, observer, nil, releaser, "desktop model-billing recovery pass failed")
}

func defaultDesktopBillingWorkerFactory(db *sql.DB, scope modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
	if db == nil {
		return desktopBillingWorkerRuntime{}, fmt.Errorf("desktop model-billing worker database is required")
	}
	if err := scope.Validate(modelbilling.BillingSubscriptionPool); err != nil {
		return desktopBillingWorkerRuntime{}, err
	}
	quotaFixtures, err := modelbilling.LoadEmbeddedQuotaFixture()
	if err != nil {
		return desktopBillingWorkerRuntime{}, err
	}
	quotaFixtures.Scope = *scope.CloudBinding
	cloudClient, err := modelbilling.NewMaterializedFixtureCloudQuotaClientWithAcceptedRequests(
		quotaFixtures,
		*scope.CloudBinding,
		modelbilling.MaterializedQuotaFixtureSelection{
			FinalCaseName:      "settled_zero",
			SettlementCaseName: "settled_zero",
		},
		defaultDesktopAcceptedQuotaRequestRegistry(),
	)
	if err != nil {
		return desktopBillingWorkerRuntime{}, err
	}
	tasks := modelbilling.NewSQLiteTaskStore(db)
	observer, err := modelbilling.NewFixtureSettlementObserver(
		*scope.CloudBinding,
		desktopFixtureSettlementIdentityResolver(tasks, scope),
	)
	if err != nil {
		return desktopBillingWorkerRuntime{}, err
	}
	reports := modelbilling.NewSQLiteReportStore(db)
	workflows := modelbilling.NewSQLiteQuotaWorkflowStore(db)
	supplement, err := modelbillingworker.NewQuotaSupplementRunner(
		workflows,
		modelbilling.NewSQLiteSubscriptionUsageReader(db),
		modelbilling.NewSQLiteQuotaEvidenceStore(db),
		reports,
		desktopFixtureQuotaSupplementer{scope: cloneDesktopBillingTaskScope(scope)},
		time.Now,
	)
	if err != nil {
		return desktopBillingWorkerRuntime{}, err
	}
	return buildDesktopBillingWorkerRuntime(db, scope, cloudClient, observer, supplement, nil, "desktop fixture model-billing recovery pass failed")
}

func buildDesktopBillingWorkerRuntime(
	db *sql.DB,
	scope modelbilling.TaskScope,
	cloudClient modelbilling.CloudQuotaClient,
	observer modelbilling.SettlementObserver,
	supplement modelbillingworker.QuotaSupplementPhase,
	freezeReleaser cloud.CloudCreditFreezeReleaser,
	errorMessage string,
) (desktopBillingWorkerRuntime, error) {
	if db == nil {
		return desktopBillingWorkerRuntime{}, fmt.Errorf("desktop model-billing worker database is required")
	}
	if err := scope.Validate(modelbilling.BillingSubscriptionPool); err != nil {
		return desktopBillingWorkerRuntime{}, err
	}
	tasks := modelbilling.NewSQLiteTaskStore(db)
	reports := modelbilling.NewSQLiteReportStore(db)
	workflows := modelbilling.NewSQLiteQuotaWorkflowStore(db)
	worker, err := modelbillingworker.New(
		reports, tasks, cloudClient, observer,
		modelbillingworker.RetryConfig{
			BaseDelay: desktopBillingRetryBaseDelay, MaxDelay: desktopBillingRetryMaxDelay,
			JitterFraction: desktopBillingRetryJitterFraction, LeaseDuration: desktopBillingLeaseDuration,
			Clock: time.Now, Jitter: func(time.Duration) time.Duration { return 0 },
		}, workflows,
	)
	if err != nil {
		return desktopBillingWorkerRuntime{}, err
	}
	coordinator, err := modelbillingworker.NewCoordinator(
		modelbilling.NewSQLiteSubscriptionQuotaReader(db),
		reports,
		tasks,
		worker,
		modelbillingworker.CoordinatorConfig{
			PollInterval:         desktopBillingPollInterval,
			OperationTimeout:     desktopBillingOperationTimeout,
			MaxDeliveriesPerPass: desktopBillingMaxDeliveriesPerPass,
			Clock:                time.Now,
			OnError:              func(err error) { slog.Warn(errorMessage, "err", err) },
			Supplement:           supplement,
			FreezeReleaser:       freezeReleaser,
		},
	)
	if err != nil {
		return desktopBillingWorkerRuntime{}, err
	}
	return desktopBillingWorkerRuntime{
		run: func(ctx context.Context) error {
			return coordinator.Run(ctx, cloneDesktopBillingTaskScope(scope))
		},
		settlements: worker,
		recovery:    coordinator,
	}, nil
}

func desktopFixtureSettlementIdentityResolver(
	tasks *modelbilling.SQLiteTaskStore,
	scope modelbilling.TaskScope,
) modelbilling.FixtureSettlementIdentityResolver {
	scope = cloneDesktopBillingTaskScope(scope)
	return func(ctx context.Context, binding cloud.BindingScope, taskID string) (modelbilling.FixtureSettlementTaskIdentity, error) {
		if scope.CloudBinding == nil || binding != *scope.CloudBinding {
			return modelbilling.FixtureSettlementTaskIdentity{}, &modelbilling.Error{
				Code: modelbilling.ErrorScopeMismatch, Message: "fixture settlement binding does not match worker scope",
			}
		}
		task, err := tasks.Get(ctx, scope, taskID)
		if err != nil {
			return modelbilling.FixtureSettlementTaskIdentity{}, err
		}
		if task.Freeze == nil {
			return modelbilling.FixtureSettlementTaskIdentity{}, &modelbilling.Error{
				Code: modelbilling.ErrorContractInvalid, Message: "fixture settlement task has no persisted freeze",
			}
		}
		return modelbilling.FixtureSettlementTaskIdentity{
			Scope: binding, TaskID: task.TaskID, FreezeID: task.Freeze.FreezeID,
		}, nil
	}
}

func (r *desktopBillingRuntime) Close() {
	if r == nil || r.db == nil {
		return
	}
	r.closeOnce.Do(func() { _ = r.db.Close() })
}

// buildDesktopController is the only desktop controller construction boundary.
// It applies the explicit B2B route before boot.Build, so ordinary rebuilds
// cannot accidentally create an unmetered main executor.
func (a *App) buildDesktopController(ctx context.Context, options boot.Options, actor userctx.ActorContext, workspaceRoot, modelRef, billingSource string) (*control.Controller, error) {
	options.DesktopVersion = version
	previousGuard := options.SessionSendGuard
	options.SessionSendGuard = func(ctx context.Context, sessionPath string) error {
		if previousGuard != nil {
			if err := previousGuard(ctx, sessionPath); err != nil {
				return err
			}
		}
		return a.authorizeCloudProjectSessionSend(ctx, actor, sessionPath)
	}
	options.SessionToolProvider = chainSessionToolProviders(options.SessionToolProvider, func(ctx context.Context, sessionPath string) ([]tool.Tool, error) {
		return a.cloudProjectSessionTools(ctx, actor, workspaceRoot, sessionPath)
	})
	options.SessionToolProvider = chainSessionToolProviders(options.SessionToolProvider, func(ctx context.Context, sessionPath string) ([]tool.Tool, error) {
		return a.capabilitySessionTools(ctx, workspaceRoot, sessionPath)
	})
	a.applyClientTimezone(&options)
	a.applyDesktopExpertCatalogOptions(&options)
	if err := a.applyConnectorCapabilities(ctx, &options, actor); err != nil {
		return nil, err
	}
	a.applyDesktopMediaToolCredential(&options, actor)
	if err := a.applyDesktopB2BOptions(&options, actor, workspaceRoot, modelRef, billingSource); err != nil {
		return nil, err
	}
	return boot.Build(ctx, options)
}

func (a *App) applyDesktopExpertCatalogOptions(options *boot.Options) {
	if options == nil || !a.usesCloudExpertAssets() {
		return
	}
	options.DisableBuiltinExperts = true
	if options.ExpertCatalogLoader != nil {
		return
	}
	userExpertPath := strings.TrimSpace(options.UserExpertPath)
	if userExpertPath == "" {
		userExpertPath = config.UserExpertsPath()
	}
	options.ExpertCatalogLoader = func() (*expert.Catalog, error) {
		return expert.LoadDefinitionCatalog(a.cloudExpertDefinitionsSnapshot(), userExpertPath)
	}
}

func (a *App) applyDesktopMediaToolCredential(options *boot.Options, actor userctx.ActorContext) {
	if options == nil || a == nil || desktopCloudFakeEnabled() || a.billingWorkerRuntime == nil {
		return
	}
	actor = actor.Normalize()
	if a.subscriptionModels != nil {
		options.MediaToolCredential = func(context.Context) (string, error) {
			binding, _, authorized := a.desktopAuthorizedSubscriptionCatalogForActor(actor)
			if !authorized {
				return "", &modelbilling.Error{Code: modelbilling.ErrorAuthRequired, Message: "AirClaude credential is unavailable"}
			}
			key, ok := a.subscriptionModels.credential(binding)
			if !ok {
				return "", &modelbilling.Error{Code: modelbilling.ErrorAuthRequired, Message: "AirClaude credential is unavailable"}
			}
			return key, nil
		}
	}
	if a.cloudRuntime != nil && a.cloudRuntime.maasBalance != nil && a.modelMarketplace != nil {
		options.MediaImageQuotaPricing = a.desktopImageQuotaPricingSource(actor)
	}
	if a.cloudRuntime != nil && (a.cloudRuntime.videoGeneration != nil || a.cloudRuntime.mediaUnderstanding != nil) {
		scopeSource := func(context.Context) (cloud.BindingScope, error) {
			binding, ok := a.billingWorkerRuntime.bindingForController(actor)
			if !ok || a.billingWorkerRuntime.authorizeSubscription(actor, binding) != nil {
				return cloud.BindingScope{}, &modelbilling.Error{
					Code: modelbilling.ErrorAuthRequired, Message: "Reasonix media authentication is unavailable",
				}
			}
			return binding, nil
		}
		options.MediaVideoScope = scopeSource
		options.MediaVideoClient = a.cloudRuntime.videoGeneration
		options.MediaUnderstandingClient = a.cloudRuntime.mediaUnderstanding
	}
}

func (a *App) applyClientTimezone(options *boot.Options) {
	if options == nil {
		return
	}
	if timezone := a.ClientTimezone(); timezone != "" {
		options.Timezone = timezone
	}
}

func (a *App) applyDesktopB2BOptions(options *boot.Options, actor userctx.ActorContext, workspaceRoot, modelRef, billingSource string) error {
	if options == nil {
		return fmt.Errorf("desktop B2B boot options are required")
	}
	source := modelbilling.BillingSource(strings.TrimSpace(billingSource))
	policy, err := desktopBillingRouteLoadPolicyFor(source, false)
	if err != nil {
		return err
	}
	if !policy.enableB2B {
		return nil
	}
	selection, err := modelbilling.NewBillingRouteSelection(modelbilling.BillingRouteSelection{ModelRef: modelRef, BillingSource: source})
	if err != nil {
		return fmt.Errorf("desktop B2B explicit billing route is required: %w", err)
	}
	if source == modelbilling.BillingSubscriptionPool {
		catalog := a.desktopSubscriptionModelsForActor(actor)
		if workBuddyRCTestMode() && len(catalog) == 0 {
			catalog = desktopAirClaudeSubscriptionModels
		}
		options.SubscriptionModelCatalog = catalog
	}
	runtime, err := a.desktopBillingRuntime(actor, workspaceRoot, selection.BillingSource)
	if err != nil {
		return err
	}
	options.RequireTaskContext = true
	options.BillingRouteSelection = selection
	options.TaskContextInitializer = runtime.initializer
	options.ModelBillingTaskStore = runtime.tasks
	options.MainStreamDispatcher = runtime.dispatcher
	if source == modelbilling.BillingSubscriptionPool {
		options.TaskLifecycleRecovery = a.billingWorkerRuntime
	}
	options.ModelBillingCleanup = runtime.Close
	options.RejectBackgroundSubagents = true
	if source == modelbilling.BillingSubscriptionPool && desktopCloudFakeEnabled() {
		options.TaskLifecycleNow = newDesktopFixturePairClock(
			desktopFixtureTimestamp(0, 0),
			desktopFixtureTimestamp(0, 12),
		)
	}
	return nil
}

func desktopConfigForBillingRoute(workspaceRoot string, actor userctx.ActorContext, source modelbilling.BillingSource) (*config.Config, error) {
	source = modelbilling.BillingSource(strings.TrimSpace(string(source)))
	policy, err := desktopBillingRouteLoadPolicyFor(source, false)
	if err != nil {
		return nil, err
	}
	if policy.metadataOnly {
		cfg, err := config.LoadForRootMetadataWithActor(workspaceRoot, actor)
		if err == nil && source == modelbilling.BillingSubscriptionPool {
			applyDesktopAirClaudeSubscriptionCatalog(cfg)
		}
		return cfg, err
	}
	return config.LoadForRootWithActor(workspaceRoot, actor)
}

func (a *App) desktopConfigForBillingRoute(workspaceRoot string, actor userctx.ActorContext, source modelbilling.BillingSource) (*config.Config, error) {
	source = modelbilling.BillingSource(strings.TrimSpace(string(source)))
	cfg, err := desktopConfigForBillingRoute(workspaceRoot, actor, source)
	if err == nil && source == modelbilling.BillingSubscriptionPool {
		a.applyDesktopSubscriptionModels(cfg, actor)
	}
	return cfg, err
}

// desktopCatalogConfigForBillingRoute is deliberately separate from the
// construction loader above. In strict fake mode an old tab can have no route;
// it may list metadata-only choices, but it must not load credentials or create
// a controller until the user selects a valid route.
func desktopCatalogConfigForBillingRoute(workspaceRoot string, actor userctx.ActorContext, source modelbilling.BillingSource) (*config.Config, bool, error) {
	source = modelbilling.BillingSource(strings.TrimSpace(string(source)))
	policy, err := desktopBillingRouteLoadPolicyFor(source, true)
	if err != nil {
		return nil, false, err
	}
	if policy.metadataOnly {
		cfg, err := config.LoadForRootMetadataWithActor(workspaceRoot, actor)
		if err == nil && source == modelbilling.BillingSubscriptionPool {
			applyDesktopAirClaudeSubscriptionCatalog(cfg)
		}
		return cfg, true, err
	}
	cfg, err := config.LoadForRootWithActor(workspaceRoot, actor)
	return cfg, false, err
}

// desktopActiveRouteSnapshot is the shared policy input for desktop reads that
// do not themselves construct a controller. In strict fake mode a missing or
// missing restored route remains metadata-only until the user makes an explicit
// choice; a non-empty invalid route fails closed and neither path hydrates a
// provider credential merely to render UI.
func (a *App) desktopActiveRouteSnapshot() (string, userctx.ActorContext, modelbilling.BillingSource) {
	workspaceRoot := "."
	actor := userctx.Default(workspaceRoot)
	var source modelbilling.BillingSource
	if a == nil {
		return workspaceRoot, actor, source
	}
	a.mu.RLock()
	if tab := a.activeTabLocked(); tab != nil {
		workspaceRoot = strings.TrimSpace(tab.WorkspaceRoot)
		if workspaceRoot == "" {
			workspaceRoot = "."
		}
		actor = actorContextForWorkspace(tabRuntimeActorContext(tab), workspaceRoot)
		source = modelbilling.BillingSource(tab.billingSource)
	}
	if a.currentActorExplicit {
		actor = a.currentActor
		actor.WorkspaceRoot = workspaceRoot
		actor = actor.Normalize()
	}
	a.mu.RUnlock()
	return workspaceRoot, actor, modelbilling.BillingSource(strings.TrimSpace(string(source)))
}

func (a *App) desktopNonRouteConfig() (*config.Config, error) {
	workspaceRoot, actor, source := a.desktopActiveRouteSnapshot()
	cfg, _, err := desktopCatalogConfigForBillingRoute(workspaceRoot, actor, source)
	return cfg, err
}

func (a *App) desktopProviderCredentialsAllowedForActiveRoute(workspaceRoot string) bool {
	activeRoot, _, source := a.desktopActiveRouteSnapshot()
	if strings.TrimSpace(workspaceRoot) != "" && strings.TrimSpace(activeRoot) != "" && filepath.Clean(workspaceRoot) != filepath.Clean(activeRoot) {
		return false
	}
	policy, err := desktopBillingRouteLoadPolicyFor(source, true)
	return err == nil && !policy.metadataOnly
}

func (a *App) desktopBillingRuntime(actor userctx.ActorContext, workspaceRoot string, source modelbilling.BillingSource) (*desktopBillingRuntime, error) {
	actor = actor.Normalize()
	source = modelbilling.BillingSource(strings.TrimSpace(string(source)))
	cfg, err := a.desktopConfigForBillingRoute(workspaceRoot, actor, source)
	if err != nil {
		return nil, err
	}
	if source == modelbilling.BillingSubscriptionPool && !desktopCloudFakeEnabled() {
		binding, err := currentDesktopSubscriptionBinding(actor, a.billingWorkerRuntime)
		if err != nil {
			return nil, err
		}
		if err := a.billingWorkerRuntime.authorizeSubscription(actor, binding); err != nil {
			return nil, err
		}
	}
	db, err := openDesktopBillingDB(actor)
	if err != nil {
		return nil, err
	}
	tasks, calls := modelbilling.NewSQLiteTaskStore(db), modelbilling.NewSQLiteCallStore(db)
	initializer := modelbilling.TaskContextInitializer{Routes: desktopBillingRouteResolver{cfg: cfg}, Tasks: tasks, IDs: modelbilling.NewRandomIDGenerator()}
	dispatcher := &modelbilling.MainExecutorDispatcher{Calls: calls, Tasks: tasks, IDs: modelbilling.NewRandomIDGenerator()}
	evidencePreparer := newapipricing.CallEvidencePreparer{Preparation: a.pricePreparationReader, Source: desktopCallEvidencePricingSource}
	if source == modelbilling.BillingSubscriptionPool {
		if err := a.configureDesktopSubscriptionRuntime(cfg, actor, &initializer, dispatcher, &evidencePreparer); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	var callEvidencePreparer modelbilling.CallEvidencePreparer = evidencePreparer
	if source == modelbilling.BillingSubscriptionPool && !desktopCloudFakeEnabled() {
		callEvidencePreparer = desktopQuotaCallEvidencePreparer{now: time.Now}
	}
	dispatcher.Evidence = modelbilling.DefaultCallEvidenceResolver{
		Store: modelbilling.NewSQLiteCallEvidenceStore(db), Preparer: callEvidencePreparer, IDs: modelbilling.NewRandomIDGenerator(),
	}
	runtime := &desktopBillingRuntime{db: db, tasks: tasks, calls: calls, dispatcher: dispatcher, initializer: desktopTaskContextInitializer{inner: initializer}}
	if a != nil && a.desktopBillingRuntimeObserver != nil {
		a.desktopBillingRuntimeObserver(runtime)
	}
	return runtime, nil
}

func (a *App) configureDesktopSubscriptionRuntime(
	cfg *config.Config,
	actor userctx.ActorContext,
	initializer *modelbilling.TaskContextInitializer,
	dispatcher *modelbilling.MainExecutorDispatcher,
	evidence *newapipricing.CallEvidencePreparer,
) error {
	if desktopCloudFakeEnabled() {
		fixtures, err := loadDesktopSubscriptionFixtures(cfg, actor)
		if err != nil {
			return err
		}
		credit := desktopFixtureCreditConfigProvider{inner: fixtures.creditConfig, actor: actor, authorization: a.billingWorkerRuntime}
		initializer.Routes = fixtures.resolver
		initializer.Subscriptions = desktopFixtureSubscriptionBootstrap{authorization: a.billingWorkerRuntime}
		initializer.CreditConfig = credit
		initializer.Freeze = desktopFixtureFreezeClient{template: fixtures.freezeTemplate, actor: actor, authorization: a.billingWorkerRuntime}
		initializer.Now = func() time.Time { return desktopFixtureTimestamp(0, 0) }
		dispatcher.Gateway = desktopAuthorizedSubscriptionGateway{materialize: fixtures.materializeGateway, actor: actor, authorization: a.billingWorkerRuntime}
		dispatcher.Now = newDesktopFixturePairClock(desktopFixtureTimestamp(0, 9), desktopFixtureTimestamp(0, 10))
		evidence.CreditConfig = credit
		evidence.CreditConfigAuthenticity = newapipricing.SourceMockOnly
		return nil
	}
	if a == nil || a.cloudRuntime == nil || a.cloudRuntime.credit == nil || a.cloudRuntime.remoteAccount == nil {
		return &modelbilling.Error{Code: modelbilling.ErrorAuthRequired, Message: "cloud subscription runtime is unavailable"}
	}
	models := desktopOfficialSubscriptionModels(cfg)
	if len(models) == 0 {
		return &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "AirClaude subscription models are unavailable"}
	}
	credit := desktopMaasCreditConfigProvider{actor: actor, authorization: a.billingWorkerRuntime, now: time.Now}
	initializer.Routes = desktopBillingRouteResolver{cfg: cfg, subscriptionModels: models, subscriptionProvider: "airclaude_subscription"}
	initializer.Subscriptions = desktopRemoteSubscriptionBootstrap{runtime: a.cloudRuntime, authorization: a.billingWorkerRuntime}
	initializer.CreditConfig = credit
	initializer.Freeze = desktopAuthorizedCreditClient{inner: a.cloudRuntime.credit, actor: actor, authorization: a.billingWorkerRuntime}
	initializer.TaskRecovery = a.cloudRuntime.taskStore
	dispatcher.Gateway = desktopAuthorizedSubscriptionGateway{
		materialize: func(binding cloud.BindingScope) (desktopSubscriptionGateway, error) {
			if a.subscriptionModels == nil {
				return nil, &modelbilling.Error{Code: modelbilling.ErrorAuthRequired, Message: "AirClaude credential is unavailable"}
			}
			key, ok := a.subscriptionModels.credential(binding)
			if !ok {
				return nil, &modelbilling.Error{Code: modelbilling.ErrorAuthRequired, Message: "AirClaude credential is unavailable"}
			}
			return airclaude.NewGateway(key, cfg.OutboundProxySpec(), time.Now)
		},
		actor: actor, authorization: a.billingWorkerRuntime,
	}
	evidence.CreditConfig = credit
	evidence.CreditConfigAuthenticity = newapipricing.SourceMockOnly
	return nil
}

func desktopOfficialSubscriptionModels(cfg *config.Config) map[string]bool {
	models := map[string]bool{}
	var catalog []string
	if workBuddyRCTestMode() {
		// The output-only RC executable never sends a model request, but its
		// real workspace controller still validates the official route while
		// creating a Project-pinned session.
		catalog = desktopAirClaudeSubscriptionModels
	} else if desktopCloudFakeEnabled() {
		catalog = desktopAirClaudeSubscriptionModels
	} else if cfg != nil {
		if provider, ok := cfg.Provider("kwu"); ok {
			catalog = provider.ModelList()
		}
	}
	for _, model := range catalog {
		if model = strings.TrimSpace(model); model != "" {
			models[model] = true
		}
	}
	return models
}

func desktopSubscriptionCatalogModels(cfg *config.Config) []string {
	if desktopCloudFakeEnabled() {
		bundle, err := modelbilling.LoadEmbeddedGatewayFixture()
		if err != nil {
			return nil
		}
		models, err := modelbilling.FixtureSubscriptionModels(bundle)
		if err != nil {
			return nil
		}
		return models
	}
	if cfg != nil {
		if provider, ok := cfg.Provider("kwu"); ok {
			return provider.ModelList()
		}
	}
	return nil
}

func applyDesktopAirClaudeSubscriptionCatalog(cfg *config.Config) {
	applyDesktopAirClaudeSubscriptionCatalogModels(cfg, desktopAirClaudeSubscriptionModels)
}

func applyDesktopAirClaudeSubscriptionCatalogModels(cfg *config.Config, models []string) {
	if cfg == nil {
		return
	}
	models = normalizedDesktopSubscriptionModels(models)
	if len(models) == 0 {
		models = append([]string(nil), desktopAirClaudeSubscriptionModels...)
	}
	official := config.OfficialProviderTemplate()
	providerIndex := -1
	for i := range cfg.Providers {
		if strings.TrimSpace(cfg.Providers[i].Name) == official.Name {
			providerIndex = i
			break
		}
	}
	if providerIndex < 0 {
		cfg.Providers = append(cfg.Providers, official)
		providerIndex = len(cfg.Providers) - 1
	} else {
		cfg.Providers[providerIndex] = official
	}
	provider := &cfg.Providers[providerIndex]
	provider.Model = ""
	provider.Models = append([]string(nil), models...)
	if !provider.HasModel(provider.Default) {
		provider.Default = models[0]
	}
}

func normalizedDesktopSubscriptionModels(models []string) []string {
	seen := make(map[string]struct{}, len(models))
	normalized := make([]string, 0, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if _, exists := seen[model]; exists {
			continue
		}
		seen[model] = struct{}{}
		normalized = append(normalized, model)
	}
	return normalized
}

type desktopSubscriptionFixtures struct {
	resolver       desktopBillingRouteResolver
	gatewayBundle  modelbilling.GatewayFixtureBundle
	gatewayCase    string
	creditConfig   modelbilling.CreditConfigProvider
	freezeTemplate cloud.FreezeFixture
}

func (f desktopSubscriptionFixtures) materializeGateway(binding cloud.BindingScope) (desktopSubscriptionGateway, error) {
	bundle := f.gatewayBundle
	bundle.Scope = binding
	return modelbilling.NewMaterializedFixtureGateway(bundle, modelbilling.GatewayFixtureSelection{
		FixtureVersion: bundle.Version,
		CaseName:       f.gatewayCase,
	})
}

func loadDesktopSubscriptionFixtures(
	cfg *config.Config,
	actor userctx.ActorContext,
) (desktopSubscriptionFixtures, error) {
	actor = actor.Normalize()
	bundle, err := modelbilling.LoadEmbeddedGatewayFixture()
	if err != nil {
		return desktopSubscriptionFixtures{}, err
	}
	// The embedded scope is only used to validate and discover the local
	// fixture template. Every task/call materializes it with the manager's
	// current full binding immediately before use.
	bundle.Scope.ProfileID = actor.ProfileID
	if err := bundle.Scope.Validate(); err != nil {
		return desktopSubscriptionFixtures{}, err
	}
	models, err := modelbilling.FixtureSubscriptionModels(bundle)
	if err != nil {
		return desktopSubscriptionFixtures{}, err
	}
	modelSet := make(map[string]bool, len(models))
	for _, model := range models {
		modelSet[model] = true
	}
	creditBundle, err := modelbilling.LoadEmbeddedCreditConfigFixture()
	if err != nil {
		return desktopSubscriptionFixtures{}, err
	}
	creditConfigFixture, err := modelbilling.NewFixtureCreditConfigProvider(creditBundle)
	if err != nil {
		return desktopSubscriptionFixtures{}, err
	}
	freezeTemplate, err := desktopFreezeGrantedTemplate()
	if err != nil {
		return desktopSubscriptionFixtures{}, err
	}
	return desktopSubscriptionFixtures{
		resolver:      desktopBillingRouteResolver{cfg: cfg, subscriptionModels: modelSet, subscriptionProvider: "subscription_gateway_fixture"},
		gatewayBundle: bundle, gatewayCase: "desktop_stream_zero",
		creditConfig: creditConfigFixture, freezeTemplate: freezeTemplate,
	}, nil
}

func desktopFreezeGrantedTemplate() (cloud.FreezeFixture, error) {
	cloudBundle, err := cloud.LoadCanonicalA0Fixture()
	if err != nil {
		return cloud.FreezeFixture{}, err
	}
	for i := range cloudBundle.FreezeCases {
		if cloudBundle.FreezeCases[i].Name == "freeze_granted" {
			return cloudBundle.FreezeCases[i], nil
		}
	}
	return cloud.FreezeFixture{}, fmt.Errorf("desktop B2B freeze fixture is missing")
}

func openDesktopBillingDB(actor userctx.ActorContext) (*sql.DB, error) {
	path := filepath.Join(config.NewProfilePathResolver(actor).MemoryUserDir(), "model-billing.sqlite")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := modelbilling.OpenSQLite(path)
	if err != nil {
		return nil, err
	}
	if err := modelbilling.ApplyMigrations(context.Background(), db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

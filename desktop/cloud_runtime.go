package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	cloud "kwu/internal/cloud"
	"kwu/internal/cloudquota"
	"kwu/internal/cloudregistration"
	"kwu/internal/config"
	"kwu/internal/modelbilling"
	"kwu/internal/netclient"
	project "kwu/internal/project"
	"kwu/internal/userctx"
)

const (
	cloudFakeEnv            = "KWU_CLOUD_FAKE"
	cloudBaseURLEnv         = "KWU_CLOUD_BASE_URL"
	cloudCaptchaDevEnv      = "KWU_CLOUD_CAPTCHA_DEV"
	cloudDevelopmentBaseURL = "http://192.168.1.183:8080"
	cloudProductionBaseURL  = "https://www.suiyuan-ai.com"
)

type cloudRuntimeMode string

const (
	cloudRuntimeDisabled cloudRuntimeMode = "disabled"
	cloudRuntimeFixture  cloudRuntimeMode = "fixture"
	cloudRuntimeReal     cloudRuntimeMode = "real"
)

func cloudRuntimeModeFromValues(fake, baseURL string) (cloudRuntimeMode, error) {
	fake = strings.TrimSpace(fake)
	baseURL = strings.TrimSpace(baseURL)
	if fake != "" && fake != "1" {
		return "", fmt.Errorf("cloud: %s must be 1 when set", cloudFakeEnv)
	}
	if fake == "1" && baseURL != "" {
		return "", fmt.Errorf("cloud: %s and %s are mutually exclusive", cloudFakeEnv, cloudBaseURLEnv)
	}
	if fake == "1" {
		return cloudRuntimeFixture, nil
	}
	if baseURL != "" {
		return cloudRuntimeReal, nil
	}
	return cloudRuntimeDisabled, nil
}

func resolveCloudBaseURL(buildVersion, developmentOverride, developmentDefault, productionDefault string) (string, error) {
	if strings.TrimSpace(buildVersion) == "dev" {
		if override := strings.TrimSpace(developmentOverride); override != "" {
			return override, nil
		}
		return strings.TrimSpace(developmentDefault), nil
	}
	baseURL := strings.TrimSpace(productionDefault)
	if baseURL == "" {
		return "", fmt.Errorf("cloud: production base URL is not configured in source")
	}
	if err := validateProductionCloudBaseURL(baseURL); err != nil {
		return "", err
	}
	return baseURL, nil
}

type cloudRegistrationMode string

const (
	cloudRegistrationUnavailable cloudRegistrationMode = "unavailable"
	cloudRegistrationFixture     cloudRegistrationMode = "fixture"
	cloudRegistrationDevelopment cloudRegistrationMode = "development"
	cloudRegistrationReal        cloudRegistrationMode = "real"
)

func cloudRegistrationModeFromValues(runtimeMode cloudRuntimeMode, _, _ string) (cloudRegistrationMode, error) {
	switch runtimeMode {
	case cloudRuntimeFixture:
		return cloudRegistrationFixture, nil
	case cloudRuntimeReal:
		// Every configured cloud endpoint uses the real Captcha and auth
		// protocol. Private endpoints are integration environments, not mocks.
		return cloudRegistrationReal, nil
	default:
		return cloudRegistrationUnavailable, nil
	}
}

var (
	cloudDeviceIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	cloudDeviceIDMu      sync.Mutex
)

func loadOrCreateCloudDeviceID(path string) (string, error) {
	cloudDeviceIDMu.Lock()
	defer cloudDeviceIDMu.Unlock()
	if data, err := os.ReadFile(path); err == nil {
		if id := string(bytes.TrimSpace(data)); cloudDeviceIDPattern.MatchString(id) {
			if err := os.Chmod(path, 0o600); err != nil {
				return "", err
			}
			return id, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	id := fmt.Sprintf("%x-%x-%x-%x-%x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16])
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", err
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return "", err
	}
	temporary, err := os.CreateTemp(directory, ".cloud-device-id-*.tmp")
	if err != nil {
		return "", err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return "", err
	}
	if _, err := temporary.WriteString(id + "\n"); err != nil {
		_ = temporary.Close()
		return "", err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return "", err
	}
	if err := temporary.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return "", err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return "", err
	}
	return id, nil
}

type cloudEventRunner interface {
	Run(context.Context, userctx.ActorContext) error
}

type cloudEventWorker struct {
	key    string
	actor  userctx.ActorContext
	scope  cloud.BindingScope
	cancel context.CancelFunc
	done   chan struct{}
}

// cloudSettlementReconcilerProxy breaks the Desktop construction cycle: cloud
// event sync is built before the app-owned billing manager, then bound to that
// manager without creating another quota worker or state machine.
type cloudSettlementReconcilerProxy struct {
	mu     sync.RWMutex
	target cloud.EventSettlementQuotaReconciler
}

func (p *cloudSettlementReconcilerProxy) bind(target cloud.EventSettlementQuotaReconciler) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.target = target
	p.mu.Unlock()
}

func (p *cloudSettlementReconcilerProxy) ReconcileSettlementEvent(ctx context.Context, actor userctx.ActorContext, binding cloud.BindingScope, taskID, settlementID string) error {
	if p == nil {
		return &modelbilling.Error{Code: modelbilling.ErrorAuthRequired, Message: "desktop settlement worker is unavailable"}
	}
	p.mu.RLock()
	target := p.target
	p.mu.RUnlock()
	if target == nil {
		return &modelbilling.Error{Code: modelbilling.ErrorAuthRequired, Message: "desktop settlement worker is not bound"}
	}
	return target.ReconcileSettlementEvent(ctx, actor, binding, taskID, settlementID)
}

type cloudCapabilities struct {
	Registration     bool
	RegistrationMode cloudRegistrationMode
	Payments         bool
	Realtime         bool
}

type cloudProjectReadinessPhase string

const (
	cloudProjectReadinessPending     cloudProjectReadinessPhase = "pending"
	cloudProjectReadinessAvailable   cloudProjectReadinessPhase = "available"
	cloudProjectReadinessUnavailable cloudProjectReadinessPhase = "unavailable"
	cloudProjectReadinessFailed      cloudProjectReadinessPhase = "failed"
)

type cloudProjectReadinessSnapshot struct {
	Epoch             uint64
	Phase             cloudProjectReadinessPhase
	Refreshing        bool
	ActorID           string
	ProfileID         string
	CloudUserID       string
	BindingGeneration uint64
	Reason            string
}

type cloudProjectBootstrapFlight struct {
	actorKey     string
	retainResult bool
	waiters      atomic.Int32
	done         chan struct{}
	scope        cloudProjectBootstrapScope
	view         CloudBootstrapView
	err          error
}

type cloudProjectBootstrapScope struct {
	ActorID           string
	ProfileID         string
	CloudUserID       string
	BindingGeneration uint64
	Epoch             uint64
}

// cloudRuntime is the single Desktop owner of cloud account, asset, payment,
// task-recovery, and event-sync state.
type cloudRuntime struct {
	available           bool
	closed              atomic.Bool
	initErr             error
	expertCatalogMode   string
	operationMu         sync.Mutex
	lifecycleMu         sync.Mutex
	started             bool
	lifecycleCtx        context.Context
	lifecycleCancel     context.CancelFunc
	worker              *cloudEventWorker
	workerResults       map[string]error
	bindings            *cloud.DurableBindingRepository
	sessionStore        *cloud.DurableCloudSessionStore
	assets              *cloud.DurableAssetSnapshotRepository
	eventState          *cloud.DurableEventStateRepository
	paymentOrders       *cloud.DurablePaymentOrderRepository
	taskStore           *cloud.DurableTaskRecoveryStore
	sessions            *cloud.ProfileSessionRepository
	authority           *cloud.FixtureCloudAuthority
	accounts            *cloud.FixtureAccountClient
	remoteAccount       *cloud.RemoteAccountApplication
	sessionClient       cloud.CloudSessionClient
	connectorOAuth      cloud.ConnectorOAuthClient
	expertAssets        cloud.ExpertAssetsClient
	errorReports        cloud.ErrorReportClient
	credit              cloud.CloudCreditClient
	maasKeys            cloud.MaaSKeyClient
	maasRecharge        cloud.MaaSRechargeClient
	maasBalance         cloud.MaaSKeyBalanceClient
	maasRechargeBalance cloud.MaaSRechargeKeyBalanceClient
	videoGeneration     cloud.VideoGenerationClient
	mediaUnderstanding  cloud.MediaUnderstandingClient
	quota               modelbilling.CloudQuotaClient
	planClient          cloud.SubscriptionPlanClient
	paymentClient       *cloud.FixturePaymentClient
	eventSource         *cloud.FixtureEventSource
	auth                *cloud.AuthApplication
	balance             *cloud.BalanceApplication
	subscription        *cloud.SubscriptionApplication
	tasks               *cloud.TaskRecoveryApplication
	payment             *cloud.PaymentApplication
	paymentRecovery     *cloud.PaymentRecoveryApplication
	eventSync           cloudEventRunner
	assetNotifier       *cloudAssetsNotifier
	turnBillingSender   turnBillingSettledSender
	settlements         *cloudSettlementReconcilerProxy
	capabilities        cloudCapabilities
	projectMu           sync.RWMutex
	projects            *cloudProjectServices
	projectCapability   project.CapabilityStatus
	projectFeatures     project.CapabilityFeatures
	projectReadiness    atomic.Pointer[cloudProjectReadinessSnapshot]
	projectBootstrap    atomic.Pointer[cloudProjectBootstrapFlight]
	projectEnvironment  cloud.EnvironmentIdentity
	projectPaths        cloud.ProfilePathProvider
	projectExecutor     cloud.AuthenticatedRequestExecutor
	projectTransport    project.ProjectJSONTransport
	deviceID            string
}

// cloudProjectServices is the authenticated, exact-scope dependency bundle
// consumed by later Wails project methods. It contains no access token.
type cloudProjectServices struct {
	Environment            cloud.EnvironmentIdentity
	Actor                  userctx.ActorContext
	Scope                  project.Scope
	StateRoot              string
	Gateway                *project.ProjectGateway
	Cache                  *project.ProjectCache
	Bindings               *project.LocalWorkspaceBindingRepository
	Journal                *project.ProjectCommandJournal
	ActivityJournal        *project.ActivityMessageJournal
	directoryMu            sync.Mutex
	directoryNext          uint64
	directoryMinimum       uint64
	directoryApplied       uint64
	joinedDirectoryNext    uint64
	joinedDirectoryApplied uint64
	commandReplayDone      chan struct{}
	lifecycle              context.Context
	cancel                 context.CancelFunc
}

func (s *cloudProjectServices) operationContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	if s == nil || s.lifecycle == nil {
		return context.WithCancel(parent)
	}
	ctx, cancel := context.WithCancel(s.lifecycle)
	if parent.Err() != nil {
		cancel()
		return ctx, cancel
	}
	stopParent := context.AfterFunc(parent, cancel)
	return ctx, func() {
		stopParent()
		cancel()
	}
}

func (r *cloudRuntime) replaceProjectServices(status project.CapabilityStatus, services *cloudProjectServices, features ...project.CapabilityFeatures) {
	if r == nil {
		return
	}
	r.projectMu.Lock()
	r.replaceProjectServicesLocked(status, services, features...)
	r.projectMu.Unlock()
}

func (r *cloudRuntime) replaceProjectServicesLocked(status project.CapabilityStatus, services *cloudProjectServices, features ...project.CapabilityFeatures) {
	previous := r.projects
	if previous != nil && previous != services && previous.cancel != nil {
		previous.cancel()
	}
	r.projects = services
	r.projectCapability = status
	r.projectFeatures = project.CapabilityFeatures{}
	if len(features) > 0 {
		r.projectFeatures = features[0]
	}
}

func (r *cloudRuntime) projectReadinessSnapshot() cloudProjectReadinessSnapshot {
	if r == nil {
		return cloudProjectReadinessSnapshot{Phase: cloudProjectReadinessUnavailable, Reason: "cloud runtime is unavailable"}
	}
	if current := r.projectReadiness.Load(); current != nil {
		return *current
	}
	if r.initErr != nil {
		return cloudProjectReadinessSnapshot{Phase: cloudProjectReadinessFailed, Reason: r.initErr.Error()}
	}
	if !r.available || r.closed.Load() {
		return cloudProjectReadinessSnapshot{Phase: cloudProjectReadinessUnavailable, Reason: "cloud runtime is unavailable"}
	}
	return cloudProjectReadinessSnapshot{Phase: cloudProjectReadinessPending, Reason: "cloud project services are initializing"}
}

func (r *cloudRuntime) initializeProjectReadiness(phase cloudProjectReadinessPhase, reason string) {
	if r == nil {
		return
	}
	snapshot := &cloudProjectReadinessSnapshot{Epoch: 1, Phase: phase, Reason: strings.TrimSpace(reason)}
	r.projectReadiness.CompareAndSwap(nil, snapshot)
}

func (r *cloudRuntime) beginProjectBootstrap(actor userctx.ActorContext) uint64 {
	return r.beginProjectBootstrapForScope(actor, project.Scope{})
}

func (r *cloudRuntime) beginProjectBootstrapForScope(actor userctx.ActorContext, scope project.Scope) uint64 {
	if r == nil {
		return 0
	}
	actor = actor.Normalize()
	r.projectMu.Lock()
	r.discardCompletedProjectBootstrapLocked()
	current := r.projectReadiness.Load()
	epoch := uint64(1)
	if current != nil {
		epoch = current.Epoch + 1
	}
	next := &cloudProjectReadinessSnapshot{
		Epoch: epoch, Phase: cloudProjectReadinessPending,
		ActorID: actor.ActorID, ProfileID: actor.ProfileID,
		Reason: "cloud project services are initializing",
	}
	if current != nil && current.Phase == cloudProjectReadinessAvailable && r.projectServicesReusableLocked(actor, scope) {
		*next = *current
		next.Epoch = epoch
		next.Refreshing = true
	} else {
		r.replaceProjectServicesLocked(project.CapabilityUnavailable, nil)
	}
	r.projectReadiness.Store(next)
	r.projectMu.Unlock()
	return epoch
}

func (r *cloudRuntime) discardCompletedProjectBootstrapLocked() {
	if r == nil {
		return
	}
	current := r.projectBootstrap.Load()
	if current == nil {
		return
	}
	select {
	case <-current.done:
		r.projectBootstrap.CompareAndSwap(current, nil)
	default:
	}
}

func (r *cloudRuntime) captureProjectBootstrapScope(actor userctx.ActorContext) (cloudProjectBootstrapScope, bool) {
	if r == nil {
		return cloudProjectBootstrapScope{}, false
	}
	actor = actor.Normalize()
	readiness := r.projectReadiness.Load()
	if readiness == nil || readiness.Phase == cloudProjectReadinessPending || readiness.Refreshing || readiness.ActorID != actor.ActorID || readiness.ProfileID != actor.ProfileID {
		return cloudProjectBootstrapScope{}, false
	}
	return cloudProjectBootstrapScope{
		ActorID: actor.ActorID, ProfileID: actor.ProfileID,
		CloudUserID: readiness.CloudUserID, BindingGeneration: readiness.BindingGeneration,
		Epoch: readiness.Epoch,
	}, true
}

func (r *cloudRuntime) projectBootstrapScopeCurrent(scope cloudProjectBootstrapScope) bool {
	if r == nil || scope.Epoch == 0 {
		return false
	}
	readiness := r.projectReadiness.Load()
	return readiness != nil && readiness.Epoch == scope.Epoch &&
		readiness.ActorID == scope.ActorID && readiness.ProfileID == scope.ProfileID &&
		readiness.CloudUserID == scope.CloudUserID && readiness.BindingGeneration == scope.BindingGeneration
}

func (r *cloudRuntime) projectBootstrapResultReusable(scope cloudProjectBootstrapScope, view CloudBootstrapView, resultErr error) bool {
	if resultErr != nil || view.OperationError != nil || !r.projectBootstrapScopeCurrent(scope) {
		return false
	}
	readiness := r.projectReadiness.Load()
	return readiness != nil && !readiness.Refreshing && readiness.Phase != cloudProjectReadinessPending && readiness.Phase != cloudProjectReadinessFailed
}

func (r *cloudRuntime) settleProjectBootstrap(epoch uint64, phase cloudProjectReadinessPhase, actor userctx.ActorContext, binding cloud.ProfileBinding, reason string) bool {
	if r == nil || phase == cloudProjectReadinessPending || phase == cloudProjectReadinessAvailable {
		return false
	}
	actor = actor.Normalize()
	r.projectMu.Lock()
	defer r.projectMu.Unlock()
	current := r.projectReadiness.Load()
	if current == nil || current.Epoch != epoch {
		return false
	}
	next := &cloudProjectReadinessSnapshot{
		Epoch: epoch, Phase: phase, ActorID: actor.ActorID, ProfileID: actor.ProfileID,
		CloudUserID: strings.TrimSpace(binding.CloudUserID), BindingGeneration: binding.BindingGeneration,
		Reason: strings.TrimSpace(reason),
	}
	r.replaceProjectServicesLocked(project.CapabilityUnavailable, nil)
	r.projectReadiness.Store(next)
	return true
}

func (r *cloudRuntime) publishProjectServices(epoch uint64, actor userctx.ActorContext, binding cloud.ProfileBinding, services *cloudProjectServices, features project.CapabilityFeatures) bool {
	if services == nil || services.Gateway == nil || services.Cache == nil || services.Bindings == nil || services.Journal == nil {
		return false
	}
	actor = actor.Normalize()
	r.projectMu.Lock()
	defer r.projectMu.Unlock()
	current := r.projectReadiness.Load()
	if current == nil || current.Epoch != epoch {
		return false
	}
	next := &cloudProjectReadinessSnapshot{
		Epoch: epoch, Phase: cloudProjectReadinessAvailable, ActorID: actor.ActorID, ProfileID: actor.ProfileID,
		CloudUserID: strings.TrimSpace(binding.CloudUserID), BindingGeneration: binding.BindingGeneration,
	}
	r.replaceProjectServicesLocked(project.CapabilityAvailable, services, features)
	r.projectReadiness.Store(next)
	return true
}

func (r *cloudRuntime) finishProjectBootstrap(epoch uint64, actor userctx.ActorContext, view CloudBootstrapView, bootstrapErr error) {
	if r == nil {
		return
	}
	current := r.projectReadiness.Load()
	if current == nil || current.Epoch != epoch || (current.Phase != cloudProjectReadinessPending && !current.Refreshing) {
		return
	}
	binding := cloud.ProfileBinding{
		ActorID: actor.Normalize().ActorID, ProfileID: view.Account.ProfileID,
		CloudUserID: view.Account.CloudUserID, Status: view.Account.BindingStatus,
		BindingGeneration: view.Account.BindingGeneration,
	}
	phase := cloudProjectReadinessUnavailable
	reason := "cloud project capability is unavailable"
	if bootstrapErr != nil || view.OperationError != nil {
		phase = cloudProjectReadinessFailed
		reason = "cloud project bootstrap failed"
	}
	r.settleProjectBootstrap(epoch, phase, actor, binding, reason)
}

func (r *cloudRuntime) projectLifecycleContext() context.Context {
	r.lifecycleMu.Lock()
	defer r.lifecycleMu.Unlock()
	if r.lifecycleCtx != nil {
		return r.lifecycleCtx
	}
	return context.Background()
}

type cloudProjectScopeResolver struct {
	bindings    *cloud.DurableBindingRepository
	environment cloud.EnvironmentIdentity
	actor       userctx.ActorContext
}

func (r cloudProjectScopeResolver) CurrentProjectScope(ctx context.Context) (project.Scope, error) {
	binding, err := r.bindings.ResolveBinding(ctx, r.actor.Normalize())
	if err != nil {
		return project.Scope{}, err
	}
	if binding.Status != cloud.BindingBound {
		return project.Scope{}, &cloud.Error{Code: cloud.ErrorAuthRequired, Message: "authentication is required"}
	}
	scope, err := binding.Scope()
	if err != nil {
		return project.Scope{}, err
	}
	return project.NewScope(r.environment, scope)
}

func (r *cloudRuntime) observeProjectBootstrap(ctx context.Context, actor userctx.ActorContext, state cloud.AccountState) error {
	current := r.projectReadinessSnapshot()
	epoch := current.Epoch
	if current.Epoch == 0 || (current.Phase != cloudProjectReadinessPending && !current.Refreshing) {
		epoch = r.beginProjectRefresh(ctx, actor)
	}
	return r.observeProjectBootstrapAtEpoch(ctx, actor, state, epoch)
}

func (r *cloudRuntime) observeProjectBootstrapAtEpoch(ctx context.Context, actor userctx.ActorContext, state cloud.AccountState, epoch uint64) (returnErr error) {
	defer func() {
		if returnErr == nil {
			return
		}
		r.settleProjectBootstrap(epoch, cloudProjectReadinessFailed, actor, state.Binding, returnErr.Error())
	}()
	status, features, err := project.EvaluateCapabilityFeatures(state.Capabilities.ProjectsV1)
	v2Status, v2Features, v2Err := project.EvaluateV2Capability(state.Capabilities.ProjectsV2)
	features.V2Status = v2Status
	if v2Status == project.CapabilityAvailable {
		features.Members = v2Features.Members
		features.Activity = v2Features.Activity
		features.Bindings = v2Features.Bindings
		features.WorkItemFields = v2Features.WorkItemFields
		features.Assets = v2Features.Assets
	}
	if status != project.CapabilityAvailable {
		if err != nil {
			return &cloud.Error{Code: cloud.ErrorCode("CLOUD_PROTOCOL_ERROR"), Message: err.Error()}
		}
		r.settleProjectBootstrap(epoch, cloudProjectReadinessUnavailable, actor, state.Binding, "cloud project capability is unavailable")
		return nil
	}
	if err != nil {
		return err
	}
	if v2Err != nil {
		return &cloud.Error{Code: cloud.ErrorCode("CLOUD_PROTOCOL_ERROR"), Message: v2Err.Error()}
	}
	if r.projectPaths == nil || r.projectExecutor == nil || r.projectTransport == nil {
		return fmt.Errorf("projects-v1: cloud runtime dependencies are not configured")
	}
	actor = actor.Normalize()
	pathResolver := r.projectPaths(actor)
	if pathResolver.ProfileID() != actor.ProfileID || strings.TrimSpace(pathResolver.MemoryUserDir()) == "" {
		return fmt.Errorf("projects-v1: profile state root is unavailable")
	}
	bindingScope, err := state.Binding.Scope()
	if err != nil {
		return err
	}
	scope, err := project.NewScope(r.projectEnvironment, bindingScope)
	if err != nil {
		return err
	}
	scopes := cloudProjectScopeResolver{bindings: r.bindings, environment: r.projectEnvironment, actor: actor}
	if err := scopesScopeMatches(ctx, scopes, scope); err != nil {
		return err
	}
	if r.reuseProjectServices(epoch, actor, scope, pathResolver.MemoryUserDir(), features) {
		return nil
	}
	gateway, err := project.NewProjectGateway(r.projectExecutor, r.projectTransport, scopes)
	if err != nil {
		return err
	}
	root := pathResolver.MemoryUserDir()
	cache, err := project.NewProjectCache(project.ProjectCacheOptions{
		Root: root, Scopes: scopes, Now: time.Now, ContractVersion: project.ContractVersion, ContractHash: project.CanonicalContractSHA256(),
	})
	if err != nil {
		return err
	}
	bindings, err := project.NewLocalWorkspaceBindingRepository(project.LocalWorkspaceBindingRepositoryOptions{Root: root, Scopes: scopes})
	if err != nil {
		return err
	}
	journal, err := project.NewProjectCommandJournal(project.ProjectCommandJournalOptions{Root: root, Scopes: scopes, Now: time.Now, Random: rand.Reader})
	if err != nil {
		return err
	}
	activityJournal, err := project.NewActivityMessageJournal(project.ActivityMessageJournalOptions{Root: root, Scopes: scopes, Now: time.Now, Random: rand.Reader})
	if err != nil {
		return err
	}
	serviceCtx, serviceCancel := context.WithCancel(r.projectLifecycleContext())
	published := false
	defer func() {
		if !published {
			serviceCancel()
		}
	}()
	services := &cloudProjectServices{Environment: r.projectEnvironment, Actor: actor, Scope: scope, StateRoot: root, Gateway: gateway, Cache: cache, Bindings: bindings, Journal: journal, ActivityJournal: activityJournal, commandReplayDone: make(chan struct{}), lifecycle: serviceCtx, cancel: serviceCancel}
	if err := scopesScopeMatches(ctx, scopes, scope); err != nil {
		return err
	}
	if !r.publishProjectServices(epoch, actor, state.Binding, services, features) {
		return &cloud.Error{Code: cloud.ErrorProfileScopeMismatch, Message: "project bootstrap belongs to a stale binding generation"}
	}
	published = true
	startCloudProjectCommandReplay(services)
	return nil
}

func (r *cloudRuntime) observeProjectBootstrapForAccount(ctx context.Context, actor userctx.ActorContext, state cloud.AccountState) error {
	return r.observeProjectBootstrap(ctx, actor, state)
}

func scopesScopeMatches(ctx context.Context, resolver cloudProjectScopeResolver, expected project.Scope) error {
	current, err := resolver.CurrentProjectScope(ctx)
	if err != nil {
		return err
	}
	if current.Key != expected.Key {
		return &cloud.Error{Code: cloud.ErrorProfileScopeMismatch, Message: "project bootstrap belongs to a stale binding scope"}
	}
	return nil
}

func newRealCloudRuntime(baseURL string, paths cloud.ProfilePathProvider, client *http.Client, device cloud.DeviceIdentity, environment cloud.EnvironmentIdentity) (*cloudRuntime, error) {
	return newRealCloudRuntimeWithRegistration(baseURL, paths, client, device, cloudRegistrationReal, environment)
}

func newRealCloudRuntimeWithRegistration(baseURL string, paths cloud.ProfilePathProvider, client *http.Client, device cloud.DeviceIdentity, registrationMode cloudRegistrationMode, environment cloud.EnvironmentIdentity) (*cloudRuntime, error) {
	if err := environment.Validate(); err != nil {
		return nil, err
	}
	bindings := cloud.NewDurableBindingRepository(paths, environment)
	sessionStore := cloud.NewDurableCloudSessionStore(paths, environment)
	assets := cloud.NewDurableAssetSnapshotRepository(paths)
	eventState := cloud.NewDurableEventStateRepository(paths)
	paymentOrders := cloud.NewDurablePaymentOrderRepository(paths)
	taskStore := cloud.NewDurableTaskRecoveryStore(paths)
	sessions := cloud.NewProfileSessionRepository(bindings, sessionStore)
	sessionClient, err := cloud.NewHTTPSessionClient(baseURL, client, device, nil)
	if err != nil {
		return nil, err
	}
	mediaSessionClient, err := cloud.NewHTTPSessionClient(
		baseURL, newMediaUnderstandingHTTPClient(client), device, nil,
	)
	if err != nil {
		return nil, err
	}
	authDependencies := cloud.AuthApplicationDependencies{
		Bindings: bindings, Sessions: sessions,
		Cleanup:       cloud.NewIdentityCleanup(sessionStore, eventState, assets),
		SessionClient: sessionClient,
	}
	capabilities, err := configureRealRegistration(&authDependencies, baseURL, client, device, registrationMode)
	if err != nil {
		return nil, err
	}
	auth, err := cloud.NewAuthApplication(authDependencies)
	if err != nil {
		return nil, err
	}
	var runtimeService *cloudRuntime
	executor, err := cloud.NewAuthenticatedRequestExecutor(sessionStore, sessionClient, nil)
	if err != nil {
		return nil, err
	}
	remoteAccount, err := cloud.NewRemoteAccountApplication(bindings, sessions, sessionClient, executor, func(ctx context.Context, actor userctx.ActorContext, state cloud.AccountState) error {
		if runtimeService == nil {
			return fmt.Errorf("projects-v1: cloud runtime is not initialized")
		}
		return runtimeService.observeProjectBootstrapForAccount(ctx, actor, state)
	})
	if err != nil {
		return nil, err
	}
	connectorOAuth, err := cloud.NewHTTPConnectorOAuthClient(executor, sessionClient)
	if err != nil {
		return nil, err
	}
	credit, err := cloud.NewHTTPCreditClient(executor, sessionClient)
	if err != nil {
		return nil, err
	}
	maasKeys, err := cloud.NewHTTPMaaSKeyClient(executor, sessionClient)
	if err != nil {
		return nil, err
	}
	maasRecharge, err := cloud.NewHTTPMaaSRechargeClient(executor, sessionClient)
	if err != nil {
		return nil, err
	}
	maasBalance, err := cloud.NewHTTPMaaSKeyBalanceClient(executor, sessionClient)
	if err != nil {
		return nil, err
	}
	maasRechargeBalance, err := cloud.NewHTTPMaaSRechargeKeyBalanceClient(executor, sessionClient)
	if err != nil {
		return nil, err
	}
	videoGeneration, err := cloud.NewHTTPVideoGenerationClient(executor, sessionClient)
	if err != nil {
		return nil, err
	}
	mediaUnderstanding, err := cloud.NewHTTPMediaUnderstandingClient(executor, mediaSessionClient)
	if err != nil {
		return nil, err
	}
	expertAssets, err := cloud.NewHTTPExpertAssetsClient(executor, sessionClient)
	if err != nil {
		return nil, err
	}
	errorReports, err := cloud.NewHTTPErrorReportClient(executor, sessionClient)
	if err != nil {
		return nil, err
	}
	quota, err := cloudquota.New(executor, sessionClient)
	if err != nil {
		return nil, err
	}
	planClient, err := cloud.NewHTTPSubscriptionPlanClient(sessionClient)
	if err != nil {
		return nil, err
	}
	paymentClient, err := cloud.NewHTTPPaymentV3Client(executor, sessionClient)
	if err != nil {
		return nil, err
	}
	payment, err := cloud.NewPaymentApplication(cloud.PaymentApplicationDependencies{
		Bindings: bindings, Payments: paymentClient, Orders: paymentOrders,
		Entitlements: cloudRemotePaymentEntitlementRefresher{
			remote: remoteAccount, assets: assets, plans: planClient,
		},
	})
	if err != nil {
		return nil, err
	}
	paymentRecovery, err := cloud.NewPaymentRecoveryApplication(payment)
	if err != nil {
		return nil, err
	}
	tasks := cloud.NewTaskRecoveryApplicationWithStore(credit, bindings, taskStore)
	runtimeService = &cloudRuntime{
		available: true, bindings: bindings, sessionStore: sessionStore, assets: assets, eventState: eventState, paymentOrders: paymentOrders, taskStore: taskStore, sessions: sessions,
		auth: auth, remoteAccount: remoteAccount, sessionClient: sessionClient, connectorOAuth: connectorOAuth, expertAssets: expertAssets, errorReports: errorReports, credit: credit, maasKeys: maasKeys, maasRecharge: maasRecharge, maasBalance: maasBalance, maasRechargeBalance: maasRechargeBalance, videoGeneration: videoGeneration, mediaUnderstanding: mediaUnderstanding, quota: quota, planClient: planClient, tasks: tasks,
		payment: payment, paymentRecovery: paymentRecovery,
		assetNotifier:     newCloudAssetsNotifier(),
		capabilities:      func() cloudCapabilities { capabilities.Payments = true; return capabilities }(),
		workerResults:     make(map[string]error),
		expertCatalogMode: expertCatalogModeRemote,
		projectCapability: project.CapabilityUnavailable, projectEnvironment: environment, projectPaths: paths,
		projectExecutor: executor, projectTransport: sessionClient, deviceID: device.DeviceID,
	}
	runtimeService.initializeProjectReadiness(cloudProjectReadinessPending, "cloud project services are initializing")
	return runtimeService, nil
}

func configureRealRegistration(dependencies *cloud.AuthApplicationDependencies, baseURL string, client *http.Client, device cloud.DeviceIdentity, mode cloudRegistrationMode) (cloudCapabilities, error) {
	capabilities := cloudCapabilities{RegistrationMode: mode}
	switch mode {
	case cloudRegistrationUnavailable:
		return capabilities, nil
	case cloudRegistrationDevelopment, cloudRegistrationReal:
		registrationClient, err := cloudregistration.NewHTTPClient(baseURL, client, device, nil)
		if err != nil {
			return cloudCapabilities{}, err
		}
		dependencies.RegistrationClient = registrationClient
		dependencies.PasswordResetClient = registrationClient
		dependencies.RegistrationEncryptor = cloud.NewRegistrationPasswordEncryptor(rand.Reader, time.Now)
		if mode == cloudRegistrationDevelopment {
			dependencies.Captcha = cloud.NewDevelopmentCaptchaGateway(rand.Reader, time.Now)
		}
		capabilities.Registration = true
		return capabilities, nil
	default:
		return cloudCapabilities{}, fmt.Errorf("cloud: unsupported real registration mode %q", mode)
	}
}

func newCloudRuntimeFromEnvironment() *cloudRuntime {
	fake := strings.TrimSpace(os.Getenv(cloudFakeEnv))
	if strings.TrimSpace(os.Getenv(cloudCaptchaDevEnv)) != "" {
		return &cloudRuntime{initErr: fmt.Errorf("cloud: %s is no longer supported", cloudCaptchaDevEnv)}
	}
	if strings.TrimSpace(version) != "dev" && fake != "" {
		return &cloudRuntime{initErr: fmt.Errorf("cloud: %s is only available in development builds", cloudFakeEnv)}
	}
	baseURL := ""
	var err error
	if version == "dev" && fake == "1" && strings.TrimSpace(os.Getenv(cloudBaseURLEnv)) == "" {
		// Explicit fixture mode is still available to local tests.
		baseURL = ""
	} else {
		baseURL, err = resolveCloudBaseURL(version, os.Getenv(cloudBaseURLEnv), cloudDevelopmentBaseURL, cloudProductionBaseURL)
		if err != nil {
			return &cloudRuntime{initErr: err}
		}
	}
	mode, err := cloudRuntimeModeFromValues(fake, baseURL)
	if err != nil {
		return &cloudRuntime{initErr: err}
	}
	registrationMode, err := cloudRegistrationModeFromValues(mode, baseURL, os.Getenv(cloudCaptchaDevEnv))
	if err != nil {
		return &cloudRuntime{initErr: err}
	}
	if registrationMode == cloudRegistrationReal {
		if err := validateRealCloudBaseURL(baseURL); err != nil {
			return &cloudRuntime{initErr: err}
		}
	}
	switch mode {
	case cloudRuntimeDisabled:
		return &cloudRuntime{}
	case cloudRuntimeFixture:
		fixtureRuntime, err := newCloudRuntime(cloud.DefaultProfilePathProvider)
		if err != nil {
			return &cloudRuntime{initErr: err}
		}
		return fixtureRuntime
	case cloudRuntimeReal:
		environment, err := cloudEnvironmentIdentity(version, baseURL)
		if err != nil {
			return &cloudRuntime{initErr: err}
		}
		deviceID, err := loadOrCreateCloudDeviceID(filepath.Join(config.MemoryUserDir(), "cloud", "device-id"))
		if err != nil {
			return &cloudRuntime{initErr: err}
		}
		client, err := newCloudHTTPClient(baseURL)
		if err != nil {
			return &cloudRuntime{initErr: err}
		}
		realRuntime, err := newRealCloudRuntimeWithRegistration(baseURL, cloud.DefaultProfilePathProvider, client, cloud.DeviceIdentity{DeviceID: deviceID, DeviceName: cloudDeviceName(), ClientVersion: cloudClientVersion()}, registrationMode, environment)
		if err != nil {
			return &cloudRuntime{initErr: err}
		}
		return realRuntime
	default:
		return &cloudRuntime{initErr: fmt.Errorf("cloud: unsupported runtime mode")}
	}
}

func cloudEnvironmentIdentity(buildVersion, baseURL string) (cloud.EnvironmentIdentity, error) {
	class := cloud.EnvironmentProduction
	if strings.TrimSpace(buildVersion) == "dev" {
		class = cloud.EnvironmentDevelopment
	}
	return cloud.NewEnvironmentIdentity(class, baseURL)
}

func validateProductionCloudBaseURL(baseURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		return fmt.Errorf("cloud: production %s must be an HTTPS URL", cloudBaseURLEnv)
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || privateCloudProxyHost(parsed.Hostname()) {
		return fmt.Errorf("cloud: production %s must use a public HTTPS origin", cloudBaseURLEnv)
	}
	return nil
}

func validateRealCloudBaseURL(baseURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("cloud: %s must be a valid HTTPS origin or private HTTP origin", cloudBaseURLEnv)
	}
	if parsed.Scheme == "https" {
		return nil
	}
	if parsed.Scheme == "http" && privateCloudProxyHost(parsed.Hostname()) {
		return nil
	}
	return fmt.Errorf("cloud: %s must use HTTPS unless it is a private integration endpoint", cloudBaseURLEnv)
}

func newCloudHTTPClient(baseURL string) (*http.Client, error) {
	cfg, err := config.LoadForRootMetadataWithActor(".", userctx.Default("."))
	if err != nil {
		return nil, err
	}
	return newCloudHTTPClientForProxySpec(baseURL, cfg.OutboundProxySpec())
}

// Public catalog reads must not be coupled to unrelated sections of config.toml.
// When the file is malformed, keep the public endpoint usable with the normal
// system/environment proxy behavior. A valid explicit proxy remains authoritative.
func newPublicCloudHTTPClient(baseURL string) (*http.Client, error) {
	cfg, err := config.LoadForRootMetadataWithActor(".", userctx.Default("."))
	if err != nil {
		return newCloudHTTPClientForProxySpec(baseURL, netclient.ProxySpec{Mode: netclient.ModeAuto})
	}
	return newCloudHTTPClientForProxySpec(baseURL, cfg.OutboundProxySpec())
}

func newCloudHTTPClientForProxySpec(baseURL string, proxySpec netclient.ProxySpec) (*http.Client, error) {
	spec, err := cloudProxySpecForBase(proxySpec, baseURL)
	if err != nil {
		return nil, err
	}
	return netclient.NewHTTPClient(spec, netclient.TransportOptions{
		DialTimeout: 10 * time.Second, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 15 * time.Second,
	})
}

// newMediaUnderstandingHTTPClient isolates long-running media requests from
// the ordinary cloud response-header deadline. The understand_media tool's
// context remains the total request deadline and still supports cancellation.
func newMediaUnderstandingHTTPClient(base *http.Client) *http.Client {
	if base == nil {
		return nil
	}
	client := *base
	client.Timeout = 0
	if transport, ok := base.Transport.(*http.Transport); ok {
		longRunningTransport := transport.Clone()
		longRunningTransport.ResponseHeaderTimeout = 0
		client.Transport = longRunningTransport
	}
	return &client
}

func cloudProxySpecForBase(spec netclient.ProxySpec, baseURL string) (netclient.ProxySpec, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Hostname() == "" {
		return netclient.ProxySpec{}, fmt.Errorf("cloud: invalid base URL")
	}
	if netclient.NormalizeMode(spec.Mode) != netclient.ModeCustom && privateCloudProxyHost(parsed.Hostname()) {
		spec.DirectHosts = append(spec.DirectHosts, parsed.Hostname())
	}
	return spec, nil
}

func privateCloudProxyHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate())
}

func cloudDeviceName() string {
	name, _ := os.Hostname()
	name = strings.TrimSpace(name)
	if name == "" {
		name = runtime.GOOS + " device"
	}
	for len(name) > 128 {
		_, size := utf8.DecodeLastRuneInString(name)
		name = name[:len(name)-size]
	}
	return name
}

func cloudClientVersion() string {
	if current := strings.TrimSpace(version); current != "" {
		return current
	}
	return "dev"
}

func newCloudRuntime(paths cloud.ProfilePathProvider) (*cloudRuntime, error) {
	environment, err := cloud.NewEnvironmentIdentity(cloud.EnvironmentFixture, "https://fixture.reasonix.invalid/")
	if err != nil {
		return nil, err
	}
	bundle, err := cloud.LoadCanonicalA0Fixture()
	if err != nil {
		return nil, err
	}
	captcha, err := cloud.NewMockCaptchaGateway("desktop-fake-captcha", time.Now().UTC().Add(24*time.Hour))
	if err != nil {
		return nil, err
	}
	bindings := cloud.NewDurableBindingRepository(paths, environment)
	sessionStore := cloud.NewDurableCloudSessionStore(paths, environment)
	assets := cloud.NewDurableAssetSnapshotRepository(paths)
	eventState := cloud.NewDurableEventStateRepository(paths)
	paymentOrders := cloud.NewDurablePaymentOrderRepository(paths)
	taskStore := cloud.NewDurableTaskRecoveryStore(paths)
	authority, err := cloud.NewFixtureCloudAuthority(bundle, paymentOrders)
	if err != nil {
		return nil, err
	}
	accounts, err := cloud.NewFixtureAccountClientWithAuthority(bundle, cloud.FixtureAccountScenarios{}, authority)
	if err != nil {
		return nil, err
	}
	paymentClient, err := cloud.NewFixturePaymentClient(bundle, authority)
	if err != nil {
		return nil, err
	}
	sessions := cloud.NewProfileSessionRepository(bindings, sessionStore)
	auth, err := cloud.NewAuthApplication(cloud.AuthApplicationDependencies{
		Bindings:       bindings,
		Sessions:       sessions,
		Cleanup:        cloud.NewIdentityCleanup(sessionStore, eventState, assets),
		Captcha:        captcha,
		Encryptor:      cloud.NewTestPasswordEncryptor(),
		Client:         accounts,
		FixtureVersion: cloud.A0FixtureVersion,
	})
	if err != nil {
		return nil, err
	}
	balance, err := cloud.NewBalanceApplication(cloud.BalanceApplicationDependencies{
		Bindings:  bindings,
		Client:    accounts,
		Snapshots: assets,
	})
	if err != nil {
		return nil, err
	}
	subscription, err := cloud.NewSubscriptionApplication(cloud.SubscriptionApplicationDependencies{
		Bindings:  bindings,
		Accounts:  accounts,
		Plans:     paymentClient,
		Snapshots: assets,
	})
	if err != nil {
		return nil, err
	}
	payment, err := cloud.NewPaymentApplication(cloud.PaymentApplicationDependencies{
		Bindings: bindings, Payments: paymentClient, Orders: paymentOrders,
		Entitlements: cloudPaymentEntitlementRefresher{balance: balance, subscription: subscription},
	})
	if err != nil {
		return nil, err
	}
	paymentRecovery, err := cloud.NewPaymentRecoveryApplication(payment)
	if err != nil {
		return nil, err
	}
	credit, err := cloud.NewFixtureCreditClient(bundle.FreezeCases)
	if err != nil {
		return nil, err
	}
	tasks := cloud.NewTaskRecoveryApplicationWithStore(credit, bindings, taskStore)
	settlements := &cloudSettlementReconcilerProxy{}
	eventSource, err := cloud.NewFixtureEventSource(bundle.Events)
	if err != nil {
		return nil, err
	}
	assetNotifier := newCloudAssetsNotifier()
	eventSync, err := cloud.NewEventSyncApplication(cloud.EventSyncApplicationDependencies{
		Bindings: bindings, Source: eventSource, State: eventState,
		Balance: balance, Tasks: tasks, Settlements: settlements, Subscription: subscription,
		Payment: paymentRecovery, Sync: auth, AssetObserver: assetNotifier.notify, MaxReconnects: 0,
	})
	if err != nil {
		return nil, err
	}
	runtimeService := &cloudRuntime{
		available: true, bindings: bindings, sessionStore: sessionStore,
		assets: assets, eventState: eventState, paymentOrders: paymentOrders, taskStore: taskStore, sessions: sessions,
		authority: authority, accounts: accounts, credit: credit, paymentClient: paymentClient, eventSource: eventSource,
		auth: auth, balance: balance, subscription: subscription,
		tasks: tasks, payment: payment, paymentRecovery: paymentRecovery, eventSync: eventSync,
		assetNotifier: assetNotifier, settlements: settlements,
		capabilities: cloudCapabilities{
			Registration: true, RegistrationMode: cloudRegistrationFixture,
			Payments: true, Realtime: true,
		},
		workerResults:     make(map[string]error),
		expertCatalogMode: expertCatalogModeFixture,
	}
	runtimeService.initializeProjectReadiness(cloudProjectReadinessPending, "cloud project services are initializing")
	return runtimeService, nil
}

type cloudPaymentEntitlementRefresher struct {
	balance      *cloud.BalanceApplication
	subscription *cloud.SubscriptionApplication
}

type cloudRemotePaymentEntitlementRefresher struct {
	remote *cloud.RemoteAccountApplication
	assets *cloud.DurableAssetSnapshotRepository
	plans  cloud.SubscriptionPlanClient
}

func (r cloudRemotePaymentEntitlementRefresher) RefreshPaymentEntitlements(ctx context.Context, actor userctx.ActorContext) error {
	state, err := r.remote.CurrentUser(ctx, actor)
	if err != nil {
		return err
	}
	scope, err := state.Binding.Scope()
	if err != nil {
		return err
	}
	if _, err := r.assets.StoreBalance(ctx, scope, state.Balance); err != nil {
		return err
	}
	plans, err := r.plans.ListSubscriptionPlans(ctx, scope)
	if err != nil {
		return err
	}
	return r.assets.StoreSubscription(ctx, scope, cloud.SubscriptionView{Plans: plans, Subscription: state.Subscription})
}

func (r cloudPaymentEntitlementRefresher) RefreshPaymentEntitlements(ctx context.Context, actor userctx.ActorContext) error {
	if _, err := r.balance.Refresh(ctx, actor); err != nil {
		return err
	}
	_, err := r.subscription.Refresh(ctx, actor)
	return err
}

func (r *cloudRuntime) recoverAuth(ctx context.Context, actor userctx.ActorContext) (cloud.AuthView, error) {
	binding, err := r.bindings.ResolveBinding(ctx, actor)
	if err != nil {
		return cloud.AuthView{}, err
	}
	if binding.Status == cloud.BindingUnbound {
		return r.auth.State(ctx, actor)
	}
	return r.auth.Recover(ctx, actor)
}

func (r *cloudRuntime) start(ctx context.Context) {
	if r == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	r.lifecycleMu.Lock()
	if r.started || r.closed.Load() {
		r.lifecycleMu.Unlock()
		return
	}
	r.lifecycleCtx, r.lifecycleCancel = context.WithCancel(ctx)
	r.started = true
	if r.workerResults == nil {
		r.workerResults = make(map[string]error)
	}
	notifier := r.assetNotifier
	r.lifecycleMu.Unlock()
	notifier.start()
}

func (r *cloudRuntime) bindAssetsChangedSender(sender cloudAssetsChangedSender) {
	if r == nil {
		return
	}
	r.assetNotifier.bind(sender)
}

func (r *cloudRuntime) bindTurnBillingSettledSender(sender turnBillingSettledSender) {
	if r == nil {
		return
	}
	r.lifecycleMu.Lock()
	if !r.closed.Load() {
		r.turnBillingSender = sender
	}
	r.lifecycleMu.Unlock()
}

func (r *cloudRuntime) notifyTurnBillingSettled(view TurnBillingSettledView) {
	if r == nil || r.closed.Load() {
		return
	}
	r.lifecycleMu.Lock()
	sender := r.turnBillingSender
	r.lifecycleMu.Unlock()
	if sender != nil {
		sender(view)
	}
}

func (r *cloudRuntime) beginOperation() error {
	if r == nil {
		return &cloud.Error{Code: cloud.ErrorCode("CLOUD_UNAVAILABLE"), Message: "cloud features are unavailable"}
	}
	r.operationMu.Lock()
	if r.closed.Load() || !r.available || r.initErr != nil {
		r.operationMu.Unlock()
		return &cloud.Error{Code: cloud.ErrorCode("CLOUD_UNAVAILABLE"), Message: "cloud features are unavailable"}
	}
	return nil
}

func (r *cloudRuntime) endOperation() {
	r.operationMu.Unlock()
}

func (r *cloudRuntime) ensureEventWorker(actor userctx.ActorContext, scope cloud.BindingScope) error {
	actor = actor.Normalize()
	if err := scope.Validate(); err != nil {
		return err
	}
	key := cloudWorkerKey(actor, scope)
	for {
		r.lifecycleMu.Lock()
		if !r.started {
			r.lifecycleMu.Unlock()
			return nil
		}
		if r.closed.Load() {
			r.lifecycleMu.Unlock()
			return &cloud.Error{Code: cloud.ErrorCode("CLOUD_UNAVAILABLE"), Message: "cloud features are unavailable"}
		}
		if _, completed := r.workerResults[key]; completed {
			r.lifecycleMu.Unlock()
			return nil
		}
		if current := r.worker; current != nil {
			if current.key == key {
				r.lifecycleMu.Unlock()
				return nil
			}
			current.cancel()
			done := current.done
			r.lifecycleMu.Unlock()
			<-done
			continue
		}
		workerCtx, cancel := context.WithCancel(r.lifecycleCtx)
		worker := &cloudEventWorker{key: key, actor: actor, scope: scope, cancel: cancel, done: make(chan struct{})}
		r.worker = worker
		r.lifecycleMu.Unlock()
		go r.runEventWorker(workerCtx, worker)
		return nil
	}
}

func (r *cloudRuntime) runEventWorker(ctx context.Context, worker *cloudEventWorker) {
	err := r.eventSync.Run(ctx, worker.actor)
	deliberateCancel := ctx.Err() != nil || errors.Is(err, context.Canceled)
	r.lifecycleMu.Lock()
	if r.worker == worker {
		r.worker = nil
	}
	if !deliberateCancel {
		r.workerResults[worker.key] = err
	}
	close(worker.done)
	r.lifecycleMu.Unlock()
}

func (r *cloudRuntime) stopEventWorker() {
	if r == nil {
		return
	}
	r.lifecycleMu.Lock()
	worker := r.worker
	if worker != nil {
		worker.cancel()
	}
	r.lifecycleMu.Unlock()
	if worker != nil {
		<-worker.done
	}
}

func (r *cloudRuntime) stopEventWorkerForAuthTransition(actor userctx.ActorContext, auth cloud.AuthView) {
	if r == nil {
		return
	}
	actor = actor.Normalize()
	keepKey := ""
	if auth.State == cloud.AccountAuthenticated && auth.Binding.Status == cloud.BindingBound &&
		auth.Binding.ActorID == actor.ActorID && auth.Binding.ProfileID == actor.ProfileID {
		if scope, err := auth.Binding.Scope(); err == nil {
			keepKey = cloudWorkerKey(actor, scope)
		}
	}
	r.lifecycleMu.Lock()
	worker := r.worker
	if worker == nil || keepKey != "" && worker.key == keepKey {
		r.lifecycleMu.Unlock()
		return
	}
	worker.cancel()
	done := worker.done
	r.lifecycleMu.Unlock()
	<-done
}

func (r *cloudRuntime) invalidateEventWorkerResults(actor userctx.ActorContext) {
	prefix := cloudWorkerActorPrefix(actor)
	r.lifecycleMu.Lock()
	defer r.lifecycleMu.Unlock()
	for key := range r.workerResults {
		if strings.HasPrefix(key, prefix) {
			delete(r.workerResults, key)
		}
	}
}

func cloudWorkerKey(actor userctx.ActorContext, scope cloud.BindingScope) string {
	return cloudWorkerActorPrefix(actor) + scope.ProfileID + "\x00" + scope.CloudUserID + "\x00" + fmt.Sprint(scope.BindingGeneration)
}

func cloudWorkerActorPrefix(actor userctx.ActorContext) string {
	actor = actor.Normalize()
	return actor.ActorID + "\x00" + actor.ProfileID + "\x00"
}

// close is intentionally idempotent and waits for the active event worker.
func (r *cloudRuntime) close() {
	if r == nil {
		return
	}
	r.operationMu.Lock()
	if r.closed.Swap(true) {
		r.operationMu.Unlock()
		return
	}
	r.lifecycleMu.Lock()
	if r.lifecycleCancel != nil {
		r.lifecycleCancel()
	}
	r.turnBillingSender = nil
	r.lifecycleMu.Unlock()
	r.replaceProjectServices(project.CapabilityUnavailable, nil)
	r.stopEventWorker()
	r.assetNotifier.close()
	r.operationMu.Unlock()
}

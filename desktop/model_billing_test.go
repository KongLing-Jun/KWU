package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kwu/internal/agent"
	"kwu/internal/boot"
	"kwu/internal/cloud"
	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/modelbilling"
	"kwu/internal/netclient"
	"kwu/internal/newapipricing"
	"kwu/internal/provider"
	"kwu/internal/userctx"
)

type desktopBillingRunnerFunc func(context.Context, string) error

func (f desktopBillingRunnerFunc) Run(ctx context.Context, input string) error { return f(ctx, input) }

type blockingReleaseSession struct {
	control.SessionAPI
	entered chan struct{}
	release chan struct{}
}

func (s *blockingReleaseSession) ReleaseResources() {
	close(s.entered)
	<-s.release
	s.SessionAPI.ReleaseResources()
}

type pendingThenReadyMaaSKeyClient struct {
	calls     atomic.Uint32
	readyCall chan struct{}
}

type desktopMaaSKeyClientFunc func(context.Context, cloud.BindingScope) (cloud.MaaSKeyAccount, error)

func (f desktopMaaSKeyClientFunc) Get(ctx context.Context, scope cloud.BindingScope) (cloud.MaaSKeyAccount, error) {
	return f(ctx, scope)
}

func (c *pendingThenReadyMaaSKeyClient) Get(ctx context.Context, scope cloud.BindingScope) (cloud.MaaSKeyAccount, error) {
	if err := ctx.Err(); err != nil {
		return cloud.MaaSKeyAccount{}, err
	}
	if c.calls.Add(1) == 1 {
		return cloud.MaaSKeyAccount{Status: cloud.MaaSKeyCreating, KeyName: scope.CloudUserID}, nil
	}
	select {
	case c.readyCall <- struct{}{}:
	default:
	}
	keyID := int64(9)
	return cloud.MaaSKeyAccount{
		Status: cloud.MaaSKeyReady, KeyName: scope.CloudUserID, KeyID: &keyID, Key: "sk-after-provisioning",
	}, nil
}

type desktopForbiddenBYOKProvider struct{ calls int }

func (*desktopForbiddenBYOKProvider) Name() string { return "byok-must-not-run" }
func (p *desktopForbiddenBYOKProvider) Stream(context.Context, provider.Request) (<-chan provider.Chunk, error) {
	p.calls++
	return nil, context.Canceled
}

type desktopCountingGateway struct {
	base  modelbilling.SubscriptionModelGateway
	calls int
}

type desktopRoleGateway struct {
	requests []modelbilling.GatewayRequest
}

type desktopBackgroundGateway struct {
	requests []modelbilling.GatewayRequest
}

type desktopForegroundSubagentGateway struct {
	requests []modelbilling.GatewayRequest
}

type desktopTaskTreeGateway struct {
	requests []modelbilling.GatewayRequest
}

func (*desktopTaskTreeGateway) EndpointFor(modelbilling.GatewayRequest) (modelbilling.MaasEndpoint, error) {
	return modelbilling.MaasEndpointChatCompletions, nil
}

func (g *desktopTaskTreeGateway) Stream(_ context.Context, _ cloud.BindingScope, request modelbilling.GatewayRequest) (<-chan modelbilling.GatewayChunk, error) {
	g.requests = append(g.requests, request)
	text := "node completed"
	if request.Role == modelbilling.CallRoleTasktreePlanner {
		text = `{"id":"tree-r2","goal":"implement change","root_ids":["node"],"status":"pending","nodes":{"node":{"id":"node","title":"Implement change","status":"pending","executor":{"kind":"main"},"expected_output":"done"}}}`
	}
	input, output, total, count := int64(3), int64(2), int64(5), int64(1)
	usage := modelbilling.NormalizedUsage{InputTokens: &input, OutputTokens: &output, ProviderTotalTokens: &total, RequestCount: &count, BreakdownStatus: modelbilling.UsageExact}
	chunks := make(chan modelbilling.GatewayChunk, 3)
	chunks <- modelbilling.GatewayChunk{Chunk: provider.Chunk{Type: provider.ChunkText, Text: text}}
	chunks <- modelbilling.GatewayChunk{
		Chunk: provider.Chunk{Type: provider.ChunkUsage, Usage: &provider.Usage{
			PromptTokens: 3, CompletionTokens: 2, TotalTokens: 5,
			Presence: provider.UsagePresence{Input: true, Output: true, ProviderTotal: true},
		}},
		Usage: &usage, ProviderRequestID: fmt.Sprintf("tasktree-request-%d", len(g.requests)),
	}
	chunks <- modelbilling.GatewayChunk{Chunk: provider.Chunk{Type: provider.ChunkDone}}
	close(chunks)
	return chunks, nil
}

func (g *desktopForegroundSubagentGateway) Stream(_ context.Context, _ cloud.BindingScope, request modelbilling.GatewayRequest) (<-chan modelbilling.GatewayChunk, error) {
	g.requests = append(g.requests, request)
	chunks := make(chan modelbilling.GatewayChunk, 4)
	switch len(g.requests) {
	case 1:
		chunks <- modelbilling.GatewayChunk{Chunk: provider.Chunk{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{
			ID: "foreground-subagent", Name: "task", Arguments: `{"prompt":"inspect the code"}`,
		}}}
	case 2:
		chunks <- modelbilling.GatewayChunk{Chunk: provider.Chunk{Type: provider.ChunkText, Text: "subagent findings"}}
	case 3:
		chunks <- modelbilling.GatewayChunk{Chunk: provider.Chunk{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{
			ID: "foreground-artifact-manifest", Name: "deliver_files", Arguments: `{"paths":[]}`,
		}}}
	default:
		chunks <- modelbilling.GatewayChunk{Chunk: provider.Chunk{Type: provider.ChunkText, Text: "root final"}}
	}
	input, output, total, count := int64(3), int64(2), int64(5), int64(1)
	usage := modelbilling.NormalizedUsage{InputTokens: &input, OutputTokens: &output, ProviderTotalTokens: &total, RequestCount: &count, BreakdownStatus: modelbilling.UsageExact}
	chunks <- modelbilling.GatewayChunk{
		Chunk: provider.Chunk{Type: provider.ChunkUsage, Usage: &provider.Usage{
			PromptTokens: 3, CompletionTokens: 2, TotalTokens: 5,
			Presence: provider.UsagePresence{Input: true, Output: true, ProviderTotal: true},
		}},
		Usage: &usage, ProviderRequestID: fmt.Sprintf("subagent-request-%d", len(g.requests)),
	}
	chunks <- modelbilling.GatewayChunk{Chunk: provider.Chunk{Type: provider.ChunkDone}}
	close(chunks)
	return chunks, nil
}

func (*desktopForegroundSubagentGateway) EndpointFor(modelbilling.GatewayRequest) (modelbilling.MaasEndpoint, error) {
	return modelbilling.MaasEndpointChatCompletions, nil
}

func (g *desktopBackgroundGateway) Stream(_ context.Context, _ cloud.BindingScope, request modelbilling.GatewayRequest) (<-chan modelbilling.GatewayChunk, error) {
	g.requests = append(g.requests, request)
	chunks := make(chan modelbilling.GatewayChunk, 4)
	if len(g.requests) == 1 {
		chunks <- modelbilling.GatewayChunk{Chunk: provider.Chunk{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{
			ID: "background-call", Name: "task", Arguments: `{"prompt":"do work","model":"other","run_in_background":true}`,
		}}}
	} else {
		chunks <- modelbilling.GatewayChunk{Chunk: provider.Chunk{Type: provider.ChunkText, Text: "background rejected"}}
	}
	input, output, total, count := int64(3), int64(2), int64(5), int64(1)
	usage := modelbilling.NormalizedUsage{InputTokens: &input, OutputTokens: &output, ProviderTotalTokens: &total, RequestCount: &count, BreakdownStatus: modelbilling.UsageExact}
	chunks <- modelbilling.GatewayChunk{
		Chunk: provider.Chunk{Type: provider.ChunkUsage, Usage: &provider.Usage{
			PromptTokens: 3, CompletionTokens: 2, TotalTokens: 5,
			Presence: provider.UsagePresence{Input: true, Output: true, ProviderTotal: true},
		}},
		Usage: &usage, ProviderRequestID: fmt.Sprintf("background-request-%d", len(g.requests)),
	}
	chunks <- modelbilling.GatewayChunk{Chunk: provider.Chunk{Type: provider.ChunkDone}}
	close(chunks)
	return chunks, nil
}

func (*desktopBackgroundGateway) EndpointFor(modelbilling.GatewayRequest) (modelbilling.MaasEndpoint, error) {
	return modelbilling.MaasEndpointChatCompletions, nil
}

func (g *desktopRoleGateway) Stream(_ context.Context, _ cloud.BindingScope, request modelbilling.GatewayRequest) (<-chan modelbilling.GatewayChunk, error) {
	g.requests = append(g.requests, request)
	input, output, total, count := int64(3), int64(2), int64(5), int64(1)
	usage := modelbilling.NormalizedUsage{
		InputTokens: &input, OutputTokens: &output, ProviderTotalTokens: &total,
		RequestCount: &count, BreakdownStatus: modelbilling.UsageExact,
	}
	reply := "implemented"
	if request.Role == modelbilling.CallRoleMain {
		reply = "1. inspect\n2. implement"
	}
	chunks := make(chan modelbilling.GatewayChunk, 3)
	chunks <- modelbilling.GatewayChunk{Chunk: provider.Chunk{Type: provider.ChunkText, Text: reply}}
	chunks <- modelbilling.GatewayChunk{
		Chunk: provider.Chunk{Type: provider.ChunkUsage, Usage: &provider.Usage{
			PromptTokens: 3, CompletionTokens: 2, TotalTokens: 5,
			Presence: provider.UsagePresence{Input: true, Output: true, ProviderTotal: true},
		}},
		Usage: &usage, ProviderRequestID: fmt.Sprintf("role-request-%d", len(g.requests)),
	}
	chunks <- modelbilling.GatewayChunk{Chunk: provider.Chunk{Type: provider.ChunkDone}}
	close(chunks)
	return chunks, nil
}

func (*desktopRoleGateway) EndpointFor(modelbilling.GatewayRequest) (modelbilling.MaasEndpoint, error) {
	return modelbilling.MaasEndpointChatCompletions, nil
}

func (g *desktopCountingGateway) Stream(ctx context.Context, scope cloud.BindingScope, request modelbilling.GatewayRequest) (<-chan modelbilling.GatewayChunk, error) {
	g.calls++
	return g.base.Stream(ctx, scope, request)
}

func (g *desktopCountingGateway) EndpointFor(request modelbilling.GatewayRequest) (modelbilling.MaasEndpoint, error) {
	resolver, ok := g.base.(modelbilling.SubscriptionGatewayEndpointResolver)
	if !ok {
		return "", fmt.Errorf("desktop counting gateway base has no endpoint resolver")
	}
	return resolver.EndpointFor(request)
}

type desktopFreezeSpy struct {
	base  cloud.CloudCreditClient
	calls int
}

func (s *desktopFreezeSpy) CreateFreeze(ctx context.Context, request cloud.FreezeRequest) (cloud.FreezeResult, error) {
	s.calls++
	return s.base.CreateFreeze(ctx, request)
}

func (s *desktopFreezeSpy) GetFreeze(ctx context.Context, scope cloud.BindingScope, taskID string) (cloud.FreezeResult, error) {
	return s.base.GetFreeze(ctx, scope, taskID)
}

type desktopCapturingInitializer struct {
	base  control.TaskContextInitializer
	task  modelbilling.TaskContext
	tasks []modelbilling.TaskContext
}

func (i *desktopCapturingInitializer) InitializeTaskContext(ctx context.Context, request control.TaskContextInitRequest) (modelbilling.TaskContext, error) {
	task, err := i.base.InitializeTaskContext(ctx, request)
	if err == nil {
		i.task = task
		i.tasks = append(i.tasks, task)
	}
	return task, err
}

func TestDesktopFakeSubscriptionMainExecutorEndToEnd(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "kwu.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	actor := userctx.Default(root)
	actor.ProfileID = "quota-profile"
	paths := config.NewProfilePathResolver(actor)
	if err := os.MkdirAll(filepath.Dir(paths.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.DefaultModel = "fixture/model_fixture_01"
	cfg.Providers = []config.ProviderEntry{{Name: "fixture", Kind: "openai", BaseURL: "https://fixture.invalid/v1", Model: "model_fixture_01", APIKeyEnv: "MISSING_B2B_BYOK_KEY"}}
	if err := cfg.SaveTo(paths.UserConfigPath()); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	app.billingWorkerRuntime.start(context.Background())
	binding := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "usr_fixture", BindingGeneration: 1}
	if err := app.activateDesktopBillingWorker(actor, binding); err != nil {
		t.Fatal(err)
	}
	runtime, err := app.desktopBillingRuntime(actor, root, modelbilling.BillingSubscriptionPool)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = app.billingWorkerRuntime.close()
		runtime.Close()
	}()
	bootstrap := runtime.initializer.inner.Subscriptions.(desktopFixtureSubscriptionBootstrap)
	resolved, err := bootstrap.ResolveSubscriptionBootstrap(context.Background(), actor, modelbilling.ResolvedBillingRoute{})
	if err != nil || resolved.Scope != binding {
		t.Fatalf("dynamic fixture binding = %+v, %v; want %+v", resolved, err, binding)
	}
	selection := modelbilling.BillingRouteSelection{ModelRef: "fixture/model_fixture_01", BillingSource: modelbilling.BillingSubscriptionPool}
	taskContext, err := runtime.initializer.InitializeTaskContext(context.Background(), controlTaskContextRequest(actor, selection))
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := runtime.tasks.Get(context.Background(), taskContext.Scope, taskContext.TaskID)
	if err != nil || prepared.State != modelbilling.TaskReady || prepared.Freeze == nil || prepared.FreezeRequest == nil || prepared.PricingEvidenceID == "" {
		t.Fatalf("write-ahead task/freeze = %+v, %v", prepared, err)
	}

	base := &desktopForbiddenBYOKProvider{}
	var fixtureTextChunks, fixtureUsageChunks int
	runner := modelbilling.TaskLifecycleRunner{
		Tasks: runtime.tasks,
		Now: newDesktopFixturePairClock(
			desktopFixtureTimestamp(0, 0),
			desktopFixtureTimestamp(0, 12),
		),
	}
	ctx, err := modelbilling.WithTaskContext(context.Background(), taskContext)
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.RunTask(ctx, func(ctx context.Context) error {
		for call := 0; call < 2; call++ {
			chunks, err := runtime.dispatcher.Stream(ctx, base, provider.Request{Messages: []provider.Message{{Role: provider.RoleUser, Content: "runtime prompt is identity only"}}})
			if err != nil {
				return err
			}
			for chunk := range chunks {
				if chunk.Type == provider.ChunkText {
					fixtureTextChunks++
				}
				if chunk.Type == provider.ChunkUsage {
					fixtureUsageChunks++
				}
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if base.calls != 0 || fixtureTextChunks != 2 || fixtureUsageChunks != 2 {
		t.Fatalf("BYOK calls/text/usage = %d/%d/%d", base.calls, fixtureTextChunks, fixtureUsageChunks)
	}
	calls, err := runtime.calls.ListByTask(context.Background(), taskContext.Scope, taskContext.TaskID)
	if err != nil || len(calls) != 2 {
		t.Fatalf("fixture call terminal = %#v, %v", calls, err)
	}
	requestIDs := make(map[string]bool, len(calls))
	for _, call := range calls {
		if call.State != modelbilling.CallCompleted || call.Provider != "subscription_gateway_fixture" || call.Usage.BreakdownStatus != modelbilling.UsageExact ||
			call.QuotaEvidence == nil || call.QuotaEvidence.OneAPIRequestID == nil || call.QuotaEvidence.Quota == nil || *call.QuotaEvidence.Quota != "0" {
			t.Fatalf("fixture call terminal = %+v", call)
		}
		requestID := *call.QuotaEvidence.OneAPIRequestID
		if requestIDs[requestID] {
			t.Fatalf("fixture response request ID was reused: %q", requestID)
		}
		requestIDs[requestID] = true
	}
	finished := waitDesktopBillingTaskState(t, runtime.tasks, taskContext.Scope, taskContext.TaskID, modelbilling.TaskCompleted)
	if finished.Result == nil || *finished.Result != modelbilling.TaskResultCompleted {
		t.Fatalf("subscription task terminal = %+v", finished)
	}
	reports, err := modelbilling.NewSQLiteReportStore(runtime.db).ListByTask(context.Background(), taskContext.Scope, taskContext.TaskID)
	if err != nil || len(reports) != 1 || reports[0].Kind != modelbilling.ReportFinal || reports[0].State != modelbilling.ReportAccepted {
		t.Fatalf("subscription final report = %+v, %v", reports, err)
	}
	var final modelbilling.FinalQuotaReport
	if err := json.Unmarshal(reports[0].CanonicalPayload, &final); err != nil {
		t.Fatal(err)
	}
	if len(final.Calls) != 2 || final.ClientSummary.TaskTotalQuota != "0" {
		t.Fatalf("subscription final quota = %+v", final)
	}
	settlement, found, err := modelbilling.NewSQLiteQuotaSettlementStore(runtime.db).CurrentQuotaSettlement(
		context.Background(), taskContext.Scope, taskContext.TaskID,
	)
	if err != nil || !found || settlement.Settlement.Status != modelbilling.QuotaSettlementSettled ||
		settlement.Settlement.TaskTotalQuota == nil || *settlement.Settlement.TaskTotalQuota != "0" {
		t.Fatalf("subscription settlement = %+v, found=%v, err=%v", settlement, found, err)
	}
}

func TestDesktopBillingWorkerRecoversAcceptedFinalAndSettlingAfterRestart(t *testing.T) {
	for _, test := range []struct {
		name      string
		preparing modelbilling.TaskState
	}{
		{name: "accepted final", preparing: modelbilling.TaskUsagePending},
		{name: "settling", preparing: modelbilling.TaskSettling},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("KWU_CLOUD_FAKE", "1")
			isolateDesktopUserDirs(t)
			root, actor, modelRef := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "MISSING_B3C_RESTART_KEY")

			beforeRestart := NewApp()
			beforeManager, binding := activateDesktopSubscriptionTestSession(t, beforeRestart, actor)
			var firstRuntime *desktopBillingRuntime
			beforeRestart.desktopBillingRuntimeObserver = func(runtime *desktopBillingRuntime) {
				firstRuntime = runtime
			}
			ctrl, err := beforeRestart.buildDesktopController(
				context.Background(),
				boot.Options{Model: modelRef, WorkspaceRoot: root, Actor: actor, Sink: event.Discard},
				actor,
				root,
				modelRef,
				string(modelbilling.BillingSubscriptionPool),
			)
			if err != nil {
				t.Fatal(err)
			}
			ctrlClosed := false
			defer func() {
				if !ctrlClosed {
					ctrl.Close()
				}
			}()
			if err := ctrl.Run(context.Background(), "strict restart recovery"); err != nil {
				t.Fatal(err)
			}
			if firstRuntime == nil {
				t.Fatal("subscription controller did not expose its billing runtime")
			}
			if _, ok := firstRuntime.initializer.inner.Subscriptions.(desktopFixtureSubscriptionBootstrap); !ok {
				t.Fatalf("subscription bootstrap = %T", firstRuntime.initializer.inner.Subscriptions)
			}
			scope := modelbilling.TaskScope{ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudBinding: &binding}
			ids, err := modelbilling.NewSQLiteSubscriptionQuotaReader(firstRuntime.db).ListUsagePendingIDs(context.Background(), scope)
			if err != nil || len(ids) != 1 {
				t.Fatalf("usage_pending IDs = %v, %v", ids, err)
			}
			taskID := ids[0]
			reports := modelbilling.NewSQLiteReportStore(firstRuntime.db)
			build, pending, err := reports.EnqueueFinal(context.Background(), scope, taskID)
			if err != nil || build.Disposition != modelbilling.ReportBuildDeliverable || pending == nil {
				t.Fatalf("enqueue final = disposition:%s pending:%+v err:%v", build.Disposition, pending, err)
			}
			workflow, err := modelbilling.NewSQLiteQuotaWorkflowStore(firstRuntime.db).Get(context.Background(), scope, taskID)
			if err != nil {
				t.Fatal(err)
			}
			acceptedAt := workflow.UpdatedAt.Add(time.Second)
			claimed, found, err := reports.ClaimDue(context.Background(), scope, acceptedAt, acceptedAt.Add(30*time.Second))
			if err != nil || !found || claimed.IdempotencyKey != pending.IdempotencyKey {
				t.Fatalf("claim final = found:%v report:%+v err:%v", found, claimed, err)
			}
			if err := reports.MarkAccepted(context.Background(), scope, claimed.IdempotencyKey, claimed.AttemptCount, acceptedAt); err != nil {
				t.Fatal(err)
			}
			persisted, err := firstRuntime.tasks.Get(context.Background(), scope, taskID)
			if err != nil {
				t.Fatal(err)
			}
			if test.preparing == modelbilling.TaskSettling {
				if err := firstRuntime.tasks.UpdateState(context.Background(), modelbilling.TaskStateUpdate{
					Scope: scope, TaskID: taskID, From: modelbilling.TaskUsagePending, To: modelbilling.TaskSettling,
					Result: persisted.Result, LastErrorCode: persisted.LastErrorCode, At: acceptedAt,
				}); err != nil {
					t.Fatal(err)
				}
			}
			persisted, err = firstRuntime.tasks.Get(context.Background(), scope, taskID)
			if err != nil || persisted.State != test.preparing {
				t.Fatalf("restart precondition task = %+v, %v", persisted, err)
			}
			storedReports, err := reports.ListByTask(context.Background(), scope, taskID)
			if err != nil || len(storedReports) != 1 || storedReports[0].State != modelbilling.ReportAccepted {
				t.Fatalf("restart precondition reports = %+v, %v", storedReports, err)
			}
			var acceptedFinal modelbilling.FinalQuotaReport
			if err := json.Unmarshal(storedReports[0].CanonicalPayload, &acceptedFinal); err != nil {
				t.Fatal(err)
			}
			if len(acceptedFinal.Calls) == 0 || acceptedFinal.Calls[0].OneAPIRequestID == nil {
				t.Fatalf("accepted final request identity = %+v", acceptedFinal.Calls)
			}
			acceptedRequestID := *acceptedFinal.Calls[0].OneAPIRequestID
			ctrl.Close()
			ctrlClosed = true
			if err := firstRuntime.db.PingContext(context.Background()); err == nil {
				t.Fatal("pre-restart controller DB remained open")
			}
			if err := beforeManager.close(); err != nil {
				t.Fatal(err)
			}

			// Simulate an upgrade where profile A already contains an accepted
			// quota-v1 row but the new global request index is still empty. Starting
			// profile B for the same cloud user must discover and backfill A first.
			otherActor := userctx.ActorContext{ActorID: actor.ActorID, ProfileID: "upgrade-profile"}.Normalize()
			otherBinding := binding
			otherBinding.ProfileID = otherActor.ProfileID
			otherBinding.BindingGeneration++
			migrationManager := newDesktopBillingWorkerManager(func(*sql.DB, modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
				return desktopBillingWorkerRuntime{run: func(ctx context.Context) error {
					<-ctx.Done()
					return ctx.Err()
				}}, nil
			})
			migrationManager.start(context.Background())
			if err := migrationManager.activate(otherActor, otherBinding); err != nil {
				t.Fatal(err)
			}
			if err := migrationManager.close(); err != nil {
				t.Fatal(err)
			}
			if err := defaultDesktopAcceptedQuotaRequestRegistry().AcceptQuotaRequestIDs(
				context.Background(), binding.CloudUserID, "other-upgrade-task", []string{acceptedRequestID},
			); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorQuotaRequestIDReused {
				t.Fatalf("historical accepted request reuse = %v", err)
			}

			afterRestart := NewApp()
			afterRestart.billingWorkerRuntime.start(context.Background())
			var secondRuntime *desktopBillingRuntime
			afterRestart.desktopBillingRuntimeObserver = func(runtime *desktopBillingRuntime) {
				secondRuntime = runtime
			}
			secondCtrl, err := afterRestart.buildDesktopController(
				context.Background(),
				boot.Options{Model: modelRef, WorkspaceRoot: root, Actor: actor, Sink: event.Discard},
				actor,
				root,
				modelRef,
				string(modelbilling.BillingSubscriptionPool),
			)
			if err != nil {
				t.Fatal(err)
			}
			if secondRuntime == nil {
				secondCtrl.Close()
				t.Fatal("restarted controller did not expose its billing runtime")
			}
			if err := afterRestart.activateDesktopBillingWorker(actor, binding); err != nil {
				secondCtrl.Close()
				t.Fatal(err)
			}
			defer func() {
				_ = afterRestart.billingWorkerRuntime.close()
				secondCtrl.Close()
			}()
			completed := waitDesktopBillingTaskState(t, secondRuntime.tasks, scope, taskID, modelbilling.TaskCompleted)
			if completed.Result == nil || *completed.Result != modelbilling.TaskResultCompleted || completed.Freeze == nil {
				t.Fatalf("recovered task = %+v", completed)
			}
			storedReports, err = modelbilling.NewSQLiteReportStore(secondRuntime.db).ListByTask(context.Background(), scope, taskID)
			if err != nil || len(storedReports) != 1 || storedReports[0].State != modelbilling.ReportAccepted {
				t.Fatalf("recovered reports = %+v, %v", storedReports, err)
			}
		})
	}
}

func TestDesktopAcceptedQuotaBackfillSkipsInactivePreV4Profile(t *testing.T) {
	isolateDesktopUserDirs(t)
	actor := userctx.ActorContext{ActorID: "accepted-backfill-actor", ProfileID: userctx.DefaultProfileID}.Normalize()
	currentDB, err := openDesktopBillingDB(actor)
	if err != nil {
		t.Fatal(err)
	}
	defer currentDB.Close()

	legacyActor := userctx.ActorContext{ActorID: actor.ActorID, ProfileID: "inactive-v3-profile"}.Normalize()
	legacyPath := filepath.Join(config.NewProfilePathResolver(legacyActor).MemoryUserDir(), "model-billing.sqlite")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	legacyDB, err := modelbilling.OpenSQLite(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacyDB.Exec(`CREATE TABLE model_billing_schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`); err != nil {
		legacyDB.Close()
		t.Fatal(err)
	}
	for version := modelbilling.SchemaVersionV1; version <= modelbilling.SchemaVersionV3; version++ {
		if _, err := legacyDB.Exec(`INSERT INTO model_billing_schema_migrations (version,applied_at) VALUES (?,?)`, version, "2026-07-15T00:00:00Z"); err != nil {
			legacyDB.Close()
			t.Fatal(err)
		}
	}
	if err := legacyDB.Close(); err != nil {
		t.Fatal(err)
	}

	binding := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "accepted-backfill-cloud-user", BindingGeneration: 1}
	scope := modelbilling.TaskScope{ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudBinding: &binding}
	registry := desktopAcceptedQuotaRequestRegistry{path: filepath.Join(t.TempDir(), "accepted.sqlite")}
	if err := backfillDesktopAcceptedQuotaRequests(context.Background(), currentDB, scope, registry); err != nil {
		t.Fatalf("backfill with inactive pre-V4 profile = %v, want skipped", err)
	}
}

func TestDesktopBillingRouteResolverDoesNotReadBYOKCredentialForSubscription(t *testing.T) {
	t.Setenv("MISSING_B2B_BYOK_KEY", "")
	cfg := config.Default()
	cfg.Providers = []config.ProviderEntry{{Name: "fixture", Kind: "openai", BaseURL: "https://fixture.invalid/v1", Model: "model_fixture_01", APIKeyEnv: "MISSING_B2B_BYOK_KEY"}}
	resolver := desktopBillingRouteResolver{cfg: cfg, subscriptionModels: map[string]bool{"model_fixture_01": true}, subscriptionProvider: "subscription_gateway_fixture"}
	selection := modelbilling.BillingRouteSelection{ModelRef: "fixture/model_fixture_01", BillingSource: modelbilling.BillingSubscriptionPool}
	route, err := resolver.ResolveBillingRoute(context.Background(), userctx.Default(""), selection)
	if err != nil || route.Provider != "subscription_gateway_fixture" {
		t.Fatalf("subscription route = %+v, %v", route, err)
	}
	selection.BillingSource = modelbilling.BillingNewAPIByok
	if _, err := resolver.ResolveBillingRoute(context.Background(), userctx.Default(""), selection); err == nil {
		t.Fatal("BYOK route accepted a missing profile credential")
	}
}

func TestDesktopQuotaCallEvidencePreparerDoesNotRequirePricingCache(t *testing.T) {
	now := time.Date(2026, 7, 16, 9, 0, 0, 0, time.UTC)
	binding := cloud.BindingScope{ProfileID: "default", CloudUserID: "cloud-user", BindingGeneration: 1}
	scope := modelbilling.TaskScope{ActorID: "local", ProfileID: "default", CloudBinding: &binding}
	version := "maas-quota-v1"
	initial := modelbilling.PricingEvidence{
		EvidenceID: "initial-evidence", SnapshotID: "initial-snapshot", Scope: scope,
		TaskID: "task-1", ModelID: "model-main", PricingGroup: "default",
		PricingVersion: &version, State: modelbilling.PricingFresh, CapturedAt: now,
	}
	prepared, err := (desktopQuotaCallEvidencePreparer{now: func() time.Time { return now }}).PrepareForCall(
		context.Background(), modelbilling.CallEvidencePreparationInput{
			Task: modelbilling.TaskContext{
				TaskID: "task-1", Scope: scope, BillingSource: modelbilling.BillingSubscriptionPool,
				InitialModelID: "model-main", PricingEvidenceID: "initial-evidence",
			},
			InitialEvidence: initial, EvidenceID: "derived-evidence", ModelID: "model-planner",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.State != modelbilling.PricingUnavailable || prepared.ModelID != "model-planner" ||
		prepared.PricingGroup != "default" || prepared.PricingVersion == nil || *prepared.PricingVersion != version {
		t.Fatalf("prepared evidence = %+v", prepared)
	}
	*prepared.PricingVersion = "mutated"
	if *initial.PricingVersion != version {
		t.Fatal("quota evidence preparer did not clone the compatibility version")
	}
}

func TestDesktopStrictRouteValidationPrecedesConfigurationLoad(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "kwu.toml"), []byte("[[providers]\nnot valid toml"), 0o644); err != nil {
		t.Fatal(err)
	}
	actor := userctx.Default(root)
	for _, source := range []modelbilling.BillingSource{"", "not-a-billing-source"} {
		_, err := desktopConfigForBillingRoute(root, actor, source)
		if err == nil || !strings.Contains(err.Error(), "explicit billing route is required") {
			t.Fatalf("source %q error = %v, want route validation before config load", source, err)
		}
	}
}

func TestDesktopControllerInjectsCurrentProductIdentityVersion(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	t.Setenv("B2C_DESKTOP_PRODUCT_IDENTITY_KEY", "profile-key")
	isolateDesktopUserDirs(t)
	oldVersion := version
	version = "desktop-test-version"
	t.Cleanup(func() { version = oldVersion })
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "B2C_DESKTOP_PRODUCT_IDENTITY_KEY")

	ctrl, err := NewApp().buildDesktopController(
		context.Background(),
		boot.Options{DesktopVersion: "caller-version", Model: modelRef, WorkspaceRoot: root, Actor: actor, Sink: event.Discard},
		actor,
		root,
		modelRef,
		string(modelbilling.BillingNewAPIByok),
	)
	if err != nil {
		t.Fatalf("buildDesktopController: %v", err)
	}
	defer ctrl.Close()

	var systemPrompt string
	for _, message := range ctrl.History() {
		if message.Role == provider.RoleSystem {
			systemPrompt = message.Content
			break
		}
	}
	if want := config.DesktopProductIdentityPolicy(version); !strings.HasSuffix(systemPrompt, want) {
		t.Fatalf("current Desktop version policy is not the final system prompt block:\n%s", systemPrompt)
	}
	if strings.Contains(systemPrompt, config.DesktopProductIdentityPolicy("caller-version")) {
		t.Fatal("Desktop controller retained caller-provided product identity version")
	}
}

func TestDesktopNonFakeSubscriptionBuildRequiresAuthenticatedBindingBeforeCredentialLoad(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "")
	t.Setenv("B2B_NONFAKE_SUBSCRIPTION_KEY", "profile-key")
	isolateDesktopUserDirs(t)
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "B2B_NONFAKE_SUBSCRIPTION_KEY")
	if err := os.Unsetenv("B2B_NONFAKE_SUBSCRIPTION_KEY"); err != nil {
		t.Fatal(err)
	}

	ctrl, err := NewApp().buildDesktopController(context.Background(), boot.Options{Model: modelRef, WorkspaceRoot: root, Actor: actor, Sink: event.Discard}, actor, root, modelRef, string(modelbilling.BillingSubscriptionPool))
	if ctrl != nil {
		ctrl.Close()
	}
	if modelbilling.ErrorCodeOf(err) != modelbilling.ErrorAuthRequired {
		t.Fatalf("non-fake subscription Build error = %v, want auth_required", err)
	}
	if got := os.Getenv("B2B_NONFAKE_SUBSCRIPTION_KEY"); got != "" {
		t.Fatalf("non-fake subscription hydrated BYOK credential %q", got)
	}
}

func TestDesktopNonFakeSubscriptionPolicyEnablesMetadataOnlyB2B(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "")
	policy, err := desktopBillingRouteLoadPolicyFor(modelbilling.BillingSubscriptionPool, false)
	if err != nil || !policy.metadataOnly || !policy.enableB2B {
		t.Fatalf("subscription policy = %+v, %v", policy, err)
	}
}

func TestDesktopNonFakeSubscriptionConfigUsesVerifiedAirClaudeModels(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "")
	isolateDesktopUserDirs(t)
	root := t.TempDir()
	actor := userctx.Default("")
	cfg, err := desktopConfigForBillingRoute(root, actor, modelbilling.BillingSubscriptionPool)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"gpt-5.4", "gpt-5.4-mini", "deepseek-v4-flash", "deepseek-v4-pro"}
	assertCatalog := func(name string, current *config.Config) {
		t.Helper()
		provider, ok := current.Provider("kwu")
		if !ok {
			t.Fatalf("%s kwu provider missing", name)
		}
		if got := provider.ModelList(); strings.Join(got, ",") != strings.Join(want, ",") || provider.HasModel("gpt-5.5") {
			t.Fatalf("%s real subscription models = %v, want %v", name, got, want)
		}
	}
	assertCatalog("construction", cfg)

	catalog, metadataOnly, err := desktopCatalogConfigForBillingRoute(root, actor, modelbilling.BillingSubscriptionPool)
	if err != nil || !metadataOnly {
		t.Fatalf("catalog config = metadata:%v err:%v", metadataOnly, err)
	}
	assertCatalog("catalog", catalog)
}

func TestDesktopSubscriptionCatalogMaterializesRuntimeOfficialProvider(t *testing.T) {
	cfg := &config.Config{
		DefaultModel: "kwu/model-z",
		Providers: []config.ProviderEntry{
			{Name: " kwu ", BaseURL: "https://custom.example/v1", APIKeyEnv: "CUSTOM_KEY", Models: []string{"model-z"}, Default: "model-z"},
			{Name: "other", BaseURL: "https://other.example/v1"},
		},
	}

	applyDesktopAirClaudeSubscriptionCatalogModels(cfg, []string{" model-z ", "model-a", "model-z"})

	provider, ok := cfg.Provider("kwu")
	if !ok {
		t.Fatal("runtime official provider missing")
	}
	if provider.BaseURL != config.OfficialProviderBaseURL || provider.APIKeyEnv != "KWU_API_KEY" {
		t.Fatalf("runtime official provider = %+v", provider)
	}
	if got := strings.Join(provider.ModelList(), ","); got != "model-z,model-a" {
		t.Fatalf("runtime official models = %q", got)
	}
	if provider.Default != "model-z" || cfg.DefaultModel != "kwu/model-z" {
		t.Fatalf("runtime official defaults = provider:%q config:%q", provider.Default, cfg.DefaultModel)
	}
	if len(cfg.Providers) != 2 {
		t.Fatalf("provider count = %d, want custom replacement plus other provider", len(cfg.Providers))
	}
}

func TestDesktopSubscriptionCatalogRefreshDrivesPickerAndRouteValidation(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "")
	isolateDesktopUserDirs(t)
	app := NewApp()
	actor := userctx.Default("")
	scope := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "cloud-user", BindingGeneration: 1}
	app.billingWorkerRuntime = &desktopBillingWorkerManager{
		bindingActor: actor, bindingScope: scope, hasBinding: true,
	}
	app.subscriptionModels = &desktopSubscriptionModelCatalog{
		fetch: func(_ context.Context, _ netclient.ProxySpec, key string) ([]string, error) {
			if key != "sk-user" {
				t.Fatalf("catalog key was not passed through")
			}
			return []string{"model-z", " model-a ", "model-z"}, nil
		},
	}
	models, err := app.subscriptionModels.fetchModels(context.Background(), netclient.ProxySpec{}, "sk-user")
	if err != nil {
		t.Fatal(err)
	}
	app.subscriptionModels.replace(scope, cloud.MaaSKeyAccount{
		Status: cloud.MaaSKeyReady, KeyName: scope.CloudUserID, Key: "sk-user",
	}, models)

	cfg, err := desktopConfigForBillingRoute(t.TempDir(), userctx.Default(""), modelbilling.BillingSubscriptionPool)
	if err != nil {
		t.Fatal(err)
	}
	app.applyDesktopSubscriptionModels(cfg, actor)
	if got := strings.Join(desktopSubscriptionCatalogModels(cfg), ","); got != "model-z,model-a" {
		t.Fatalf("picker catalog = %q, want model-z,model-a", got)
	}
	officialModels := desktopOfficialSubscriptionModels(cfg)
	resolver := desktopBillingRouteResolver{cfg: cfg, subscriptionModels: officialModels, subscriptionProvider: "airclaude_subscription"}
	resolved, err := resolver.ResolveBillingRoute(context.Background(), userctx.Default(""), modelbilling.BillingRouteSelection{
		ModelRef: "kwu/model-a", BillingSource: modelbilling.BillingSubscriptionPool,
	})
	if err != nil || resolved.ModelID != "model-a" {
		t.Fatalf("dynamic route = %+v, %v", resolved, err)
	}
}

func TestDesktopSubscriptionCatalogRetriesProvisioningAndInvalidatesClients(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "")
	isolateDesktopUserDirs(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	actor := userctx.ActorContext{ActorID: "local", ProfileID: "provisioning-profile"}.Normalize()
	binding := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "provisioning-user", BindingGeneration: 1}
	manager := newDesktopBillingWorkerManager(func(_ *sql.DB, _ modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		return desktopBillingWorkerRuntime{run: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}}, nil
	})
	manager.start(ctx)
	t.Cleanup(func() { _ = manager.close() })
	if err := manager.activate(actor, binding); err != nil {
		t.Fatal(err)
	}

	maasKeys := &pendingThenReadyMaaSKeyClient{readyCall: make(chan struct{}, 1)}
	app := &App{
		ctx:                  ctx,
		cloudRuntime:         &cloudRuntime{maasKeys: maasKeys},
		billingWorkerRuntime: manager,
		subscriptionModels: &desktopSubscriptionModelCatalog{fetch: func(_ context.Context, _ netclient.ProxySpec, key string) ([]string, error) {
			if key != "sk-after-provisioning" {
				t.Fatalf("catalog key = %q", key)
			}
			return []string{"deepseek-v4-flash", "gpt-5.4"}, nil
		}},
		tabRuntimes: map[string]*WorkspaceTab{},
	}
	invalidated := make(chan string, 1)
	app.runtimeEvents.emit = func(_ context.Context, name string, payload ...interface{}) {
		if name == "model-route:changed" && len(payload) == 1 {
			if tabID, ok := payload[0].(string); ok {
				invalidated <- tabID
			}
		}
	}

	app.refreshDesktopSubscriptionModelsBestEffort(actor)
	if state, models := app.subscriptionModels.state(binding); state != string(cloud.MaaSKeyCreating) || len(models) != 0 {
		t.Fatalf("initial provisioning catalog = state:%q models:%v", state, models)
	}
	select {
	case <-maasKeys.readyCall:
	case <-time.After(2 * time.Second):
		t.Fatal("pending MaaS catalog was not retried")
	}
	select {
	case tabID := <-invalidated:
		if tabID != "" {
			t.Fatalf("catalog invalidation tab = %q, want global", tabID)
		}
	case <-time.After(time.Second):
		t.Fatal("ready MaaS catalog did not invalidate frontend model facts")
	}
	if got := strings.Join(app.subscriptionModels.current(binding), ","); got != "deepseek-v4-flash,gpt-5.4" {
		t.Fatalf("retried catalog = %q", got)
	}
}

func TestDesktopSubscriptionCatalogRetriesTransientFetchFailureAndInvalidatesClients(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "")
	isolateDesktopUserDirs(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	actor := userctx.ActorContext{ActorID: "local", ProfileID: "transient-fetch-profile"}.Normalize()
	binding := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "transient-fetch-user", BindingGeneration: 1}
	manager := newDesktopBillingWorkerManager(func(_ *sql.DB, _ modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		return desktopBillingWorkerRuntime{run: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}}, nil
	})
	manager.start(ctx)
	t.Cleanup(func() { _ = manager.close() })
	if err := manager.activate(actor, binding); err != nil {
		t.Fatal(err)
	}

	account := cloud.MaaSKeyAccount{Status: cloud.MaaSKeyReady, KeyName: binding.CloudUserID, Key: "sk-ready"}
	var fetchCalls atomic.Uint32
	app := &App{
		ctx: ctx,
		cloudRuntime: &cloudRuntime{maasKeys: desktopMaaSKeyClientFunc(func(_ context.Context, scope cloud.BindingScope) (cloud.MaaSKeyAccount, error) {
			if scope != binding {
				return cloud.MaaSKeyAccount{}, fmt.Errorf("MaaS key scope = %+v, want %+v", scope, binding)
			}
			return account, nil
		})},
		billingWorkerRuntime: manager,
		subscriptionModels: &desktopSubscriptionModelCatalog{fetch: func(_ context.Context, _ netclient.ProxySpec, key string) ([]string, error) {
			if key != account.Key {
				return nil, fmt.Errorf("catalog key = %q, want %q", key, account.Key)
			}
			if fetchCalls.Add(1) == 1 {
				return nil, fmt.Errorf("temporary model catalog failure")
			}
			return []string{"deepseek-v4-flash", "gpt-5.4", "gpt-5.6-luna"}, nil
		}},
		tabRuntimes: map[string]*WorkspaceTab{},
	}
	invalidated := make(chan string, 2)
	app.runtimeEvents.emit = func(_ context.Context, name string, payload ...interface{}) {
		if name == "model-route:changed" && len(payload) == 1 {
			if tabID, ok := payload[0].(string); ok {
				invalidated <- tabID
			}
		}
	}

	app.refreshDesktopSubscriptionModelsBestEffort(actor)
	if got := app.subscriptionModels.current(binding); !slices.Equal(got, desktopAirClaudeSubscriptionModels) {
		t.Fatalf("catalog after initial fetch failure = %v, want fallback %v", got, desktopAirClaudeSubscriptionModels)
	}
	for event := 0; event < 2; event++ {
		select {
		case tabID := <-invalidated:
			if tabID != "" {
				t.Fatalf("catalog invalidation tab = %q, want global", tabID)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("catalog invalidation %d was not emitted", event+1)
		}
	}
	if got := strings.Join(app.subscriptionModels.current(binding), ","); got != "deepseek-v4-flash,gpt-5.4,gpt-5.6-luna" {
		t.Fatalf("retried catalog = %q", got)
	}
	if got := fetchCalls.Load(); got != 2 {
		t.Fatalf("catalog fetch calls = %d, want 2", got)
	}
}

func TestDesktopSubscriptionCatalogRetryContinuesPastFormerLimit(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "")
	isolateDesktopUserDirs(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	actor := userctx.ActorContext{ActorID: "local", ProfileID: "long-retry-profile"}.Normalize()
	binding := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "long-retry-user", BindingGeneration: 1}
	manager := newDesktopBillingWorkerManager(func(_ *sql.DB, _ modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		return desktopBillingWorkerRuntime{run: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}}, nil
	})
	manager.start(ctx)
	t.Cleanup(func() { _ = manager.close() })
	if err := manager.activate(actor, binding); err != nil {
		t.Fatal(err)
	}

	account := cloud.MaaSKeyAccount{Status: cloud.MaaSKeyReady, KeyName: binding.CloudUserID, Key: "sk-ready"}
	var getCalls atomic.Uint32
	var fetchCalls atomic.Uint32
	app := &App{
		ctx: ctx,
		cloudRuntime: &cloudRuntime{maasKeys: desktopMaaSKeyClientFunc(func(context.Context, cloud.BindingScope) (cloud.MaaSKeyAccount, error) {
			if getCalls.Add(1) <= 11 {
				return cloud.MaaSKeyAccount{}, fmt.Errorf("temporary MaaS key failure")
			}
			return account, nil
		})},
		billingWorkerRuntime: manager,
		subscriptionModels: &desktopSubscriptionModelCatalog{fetch: func(context.Context, netclient.ProxySpec, string) ([]string, error) {
			fetchCalls.Add(1)
			return []string{"deepseek-v4-flash", "gpt-5.6-luna"}, nil
		}},
		tabRuntimes: map[string]*WorkspaceTab{},
	}
	invalidated := make(chan string, 1)
	app.runtimeEvents.emit = func(_ context.Context, name string, payload ...interface{}) {
		if name == "model-route:changed" && len(payload) == 1 {
			if tabID, ok := payload[0].(string); ok {
				invalidated <- tabID
			}
		}
	}
	const epoch = uint64(7)
	app.subscriptionModelRefreshEpoch.Store(epoch)
	app.subscriptionModels.advanceRefreshEpoch(epoch)
	delays := make([]time.Duration, 0, 12)
	app.retryDesktopSubscriptionModelsWithWait(ctx, actor, epoch, func(_ context.Context, delay time.Duration) bool {
		delays = append(delays, delay)
		return true
	})

	if got := getCalls.Load(); got != 12 {
		t.Fatalf("MaaS key calls = %d, want 12", got)
	}
	if got := fetchCalls.Load(); got != 1 {
		t.Fatalf("catalog fetch calls = %d, want 1", got)
	}
	if len(delays) != 12 {
		t.Fatalf("catalog retry waits = %d, want 12", len(delays))
	}
	if got := strings.Join(app.subscriptionModels.current(binding), ","); got != "deepseek-v4-flash,gpt-5.6-luna" {
		t.Fatalf("catalog after long recovery = %q", got)
	}
	select {
	case tabID := <-invalidated:
		if tabID != "" {
			t.Fatalf("catalog invalidation tab = %q, want global", tabID)
		}
	case <-time.After(time.Second):
		t.Fatal("recovered catalog did not invalidate frontend model facts")
	}
}

func TestDesktopSubscriptionCatalogKeepsSuccessfulModelsAcrossTransientGetAndConfigErrors(t *testing.T) {
	tests := []struct {
		name       string
		prepare    func(*testing.T, userctx.ActorContext)
		maasResult func(cloud.BindingScope) (cloud.MaaSKeyAccount, error)
	}{
		{
			name: "MaaS key lookup",
			maasResult: func(cloud.BindingScope) (cloud.MaaSKeyAccount, error) {
				return cloud.MaaSKeyAccount{}, fmt.Errorf("temporary MaaS key failure")
			},
		},
		{
			name: "config load",
			prepare: func(t *testing.T, actor userctx.ActorContext) {
				path := config.NewProfilePathResolver(actor).UserConfigPath()
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("invalid = ["), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			maasResult: func(scope cloud.BindingScope) (cloud.MaaSKeyAccount, error) {
				return cloud.MaaSKeyAccount{Status: cloud.MaaSKeyReady, KeyName: scope.CloudUserID, Key: "sk-stable"}, nil
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("KWU_CLOUD_FAKE", "")
			isolateDesktopUserDirs(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			actor := userctx.Default(t.TempDir())
			actor.ProfileID = "preserve-success-profile"
			actor = actor.Normalize()
			if tt.prepare != nil {
				tt.prepare(t, actor)
			}
			app := &App{ctx: ctx, subscriptionModels: &desktopSubscriptionModelCatalog{}, tabRuntimes: map[string]*WorkspaceTab{}}
			_, binding := activateDesktopSubscriptionTestSession(t, app, actor)
			app.cloudRuntime = &cloudRuntime{maasKeys: desktopMaaSKeyClientFunc(func(_ context.Context, scope cloud.BindingScope) (cloud.MaaSKeyAccount, error) {
				return tt.maasResult(scope)
			})}
			account := cloud.MaaSKeyAccount{Status: cloud.MaaSKeyReady, KeyName: binding.CloudUserID, Key: "sk-stable"}
			app.subscriptionModels.replace(binding, account, []string{"model-stable", "gpt-5.6-luna"})

			epoch := app.beginDesktopSubscriptionModelRefresh()
			if _, err := app.refreshDesktopSubscriptionModelsSnapshot(ctx, actor, epoch); err == nil {
				t.Fatal("transient refresh error = nil")
			}
			if got := strings.Join(app.subscriptionModels.current(binding), ","); got != "model-stable,gpt-5.6-luna" {
				t.Fatalf("catalog after transient failure = %q", got)
			}
			if key, ok := app.subscriptionModels.credential(binding); !ok || key != account.Key {
				t.Fatalf("catalog credential after transient failure = %q, %v", key, ok)
			}
		})
	}
}

func TestDesktopSubscriptionCatalogClearsAndStopsOnTerminalCloudSessionError(t *testing.T) {
	tests := []struct {
		name string
		code cloud.ErrorCode
	}{
		{name: "authentication required", code: cloud.ErrorAuthRequired},
		{name: "refresh token invalid", code: cloud.ErrorCode("REFRESH_TOKEN_INVALID")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("KWU_CLOUD_FAKE", "")
			isolateDesktopUserDirs(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			actor := userctx.ActorContext{ActorID: "local", ProfileID: "terminal-cloud-profile"}.Normalize()
			app := &App{ctx: ctx, subscriptionModels: &desktopSubscriptionModelCatalog{}, tabRuntimes: map[string]*WorkspaceTab{}}
			_, binding := activateDesktopSubscriptionTestSession(t, app, actor)
			account := cloud.MaaSKeyAccount{Status: cloud.MaaSKeyReady, KeyName: binding.CloudUserID, Key: "sk-terminal"}
			app.subscriptionModels.replace(binding, account, []string{"model-stale", "gpt-5.6-luna"})
			var getCalls atomic.Uint32
			app.cloudRuntime = &cloudRuntime{maasKeys: desktopMaaSKeyClientFunc(func(context.Context, cloud.BindingScope) (cloud.MaaSKeyAccount, error) {
				getCalls.Add(1)
				return cloud.MaaSKeyAccount{}, &cloud.Error{Code: tt.code, Message: "session is terminal"}
			})}
			invalidated := make(chan string, 2)
			app.runtimeEvents.emit = func(_ context.Context, name string, payload ...interface{}) {
				if name == "model-route:changed" && len(payload) == 1 {
					if tabID, ok := payload[0].(string); ok {
						invalidated <- tabID
					}
				}
			}

			app.refreshDesktopSubscriptionModelsBestEffort(actor)
			if got := app.subscriptionModels.current(binding); len(got) != 0 {
				t.Fatalf("terminal catalog models = %v, want empty", got)
			}
			if key, ok := app.subscriptionModels.credential(binding); ok || key != "" {
				t.Fatalf("terminal catalog credential = %q, %v; want cleared", key, ok)
			}
			select {
			case tabID := <-invalidated:
				if tabID != "" {
					t.Fatalf("terminal catalog invalidation tab = %q, want global", tabID)
				}
			case <-time.After(time.Second):
				t.Fatal("terminal catalog did not emit global invalidation")
			}
			select {
			case tabID := <-invalidated:
				t.Fatalf("terminal catalog emitted an extra invalidation for tab %q", tabID)
			case <-time.After(350 * time.Millisecond):
			}
			if got := getCalls.Load(); got != 1 {
				t.Fatalf("terminal MaaS key calls = %d, want 1 without retry", got)
			}

			app.subscriptionModels.replace(binding, account, []string{"model-stale", "gpt-5.6-luna"})
			epoch := app.beginDesktopSubscriptionModelRefresh()
			waitCalls := 0
			app.retryDesktopSubscriptionModelsWithWait(ctx, actor, epoch, func(context.Context, time.Duration) bool {
				waitCalls++
				return true
			})
			if waitCalls != 1 || getCalls.Load() != 2 {
				t.Fatalf("terminal in-flight retry waits/gets = %d/%d, want 1/2", waitCalls, getCalls.Load())
			}
			if got := app.subscriptionModels.current(binding); len(got) != 0 {
				t.Fatalf("terminal retry catalog models = %v, want empty", got)
			}
			select {
			case tabID := <-invalidated:
				if tabID != "" {
					t.Fatalf("terminal retry invalidation tab = %q, want global", tabID)
				}
			case <-time.After(time.Second):
				t.Fatal("terminal retry did not emit global invalidation")
			}
			select {
			case tabID := <-invalidated:
				t.Fatalf("terminal retry emitted an extra invalidation for tab %q", tabID)
			case <-time.After(100 * time.Millisecond):
			}
		})
	}
}

func TestDesktopSubscriptionCatalogRejectsSupersededCommitAcrossManualRefresh(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "")
	isolateDesktopUserDirs(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	actor := userctx.ActorContext{ActorID: "local", ProfileID: "ordered-refresh-profile"}.Normalize()
	app := &App{
		ctx:                  ctx,
		currentActor:         actor,
		currentActorExplicit: true,
		subscriptionModels:   &desktopSubscriptionModelCatalog{},
		tabRuntimes:          map[string]*WorkspaceTab{},
	}
	_, binding := activateDesktopSubscriptionTestSession(t, app, actor)
	oldFetchStarted := make(chan struct{})
	releaseOldFetch := make(chan struct{})
	var keyCalls atomic.Uint32
	app.cloudRuntime = &cloudRuntime{maasKeys: desktopMaaSKeyClientFunc(func(_ context.Context, scope cloud.BindingScope) (cloud.MaaSKeyAccount, error) {
		key := "sk-new"
		if keyCalls.Add(1) == 1 {
			key = "sk-old"
		}
		return cloud.MaaSKeyAccount{Status: cloud.MaaSKeyReady, KeyName: scope.CloudUserID, Key: key}, nil
	})}
	app.subscriptionModels.fetch = func(_ context.Context, _ netclient.ProxySpec, key string) ([]string, error) {
		if key == "sk-old" {
			close(oldFetchStarted)
			<-releaseOldFetch
			return []string{"model-old"}, nil
		}
		return []string{"model-new", "gpt-5.6-luna"}, nil
	}
	invalidated := make(chan string, 2)
	app.runtimeEvents.emit = func(_ context.Context, name string, payload ...interface{}) {
		if name == "model-route:changed" && len(payload) == 1 {
			if tabID, ok := payload[0].(string); ok {
				invalidated <- tabID
			}
		}
	}
	oldDone := make(chan struct{})
	go func() {
		defer close(oldDone)
		app.refreshDesktopSubscriptionModelsBestEffort(actor)
	}()
	select {
	case <-oldFetchStarted:
	case <-time.After(time.Second):
		t.Fatal("old refresh did not reach fetch barrier")
	}
	if err := app.RefreshOfficialModels(); err != nil {
		t.Fatalf("manual refresh: %v", err)
	}
	if got := app.subscriptionModelRefreshEpoch.Load(); got != 2 {
		t.Fatalf("refresh epoch after manual refresh = %d, want 2", got)
	}
	close(releaseOldFetch)
	select {
	case <-oldDone:
	case <-time.After(time.Second):
		t.Fatal("superseded refresh did not return")
	}
	if got := strings.Join(app.subscriptionModels.current(binding), ","); got != "model-new,gpt-5.6-luna" {
		t.Fatalf("catalog after late old response = %q", got)
	}
	if key, ok := app.subscriptionModels.credential(binding); !ok || key != "sk-new" {
		t.Fatalf("catalog credential after late old response = %q, %v", key, ok)
	}
	select {
	case tabID := <-invalidated:
		if tabID != "" {
			t.Fatalf("catalog invalidation tab = %q, want global", tabID)
		}
	case <-time.After(time.Second):
		t.Fatal("new manual refresh did not emit global invalidation")
	}
	select {
	case tabID := <-invalidated:
		t.Fatalf("superseded refresh emitted extra invalidation for tab %q", tabID)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestDesktopSubscriptionCatalogRetryBackoffCapsAtOneMinute(t *testing.T) {
	tests := []struct {
		name    string
		attempt uint
		want    time.Duration
	}{
		{name: "initial", attempt: 0, want: 250 * time.Millisecond},
		{name: "exponential", attempt: 4, want: 4 * time.Second},
		{name: "first capped", attempt: 8, want: 60 * time.Second},
		{name: "remains capped", attempt: 100, want: 60 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := desktopSubscriptionModelRetryBackoff(tt.attempt); got != tt.want {
				t.Fatalf("retry backoff(%d) = %v, want %v", tt.attempt, got, tt.want)
			}
		})
	}
}

func TestDesktopSubscriptionCatalogRetryStopsBeforeRequestAfterEpochOrContextChange(t *testing.T) {
	tests := []struct {
		name       string
		beforeWait func(*App, context.CancelFunc)
		waitResult bool
	}{
		{
			name: "epoch changes",
			beforeWait: func(app *App, _ context.CancelFunc) {
				app.subscriptionModelRefreshEpoch.Add(1)
			},
			waitResult: true,
		},
		{
			name: "context cancels",
			beforeWait: func(_ *App, cancel context.CancelFunc) {
				cancel()
			},
			waitResult: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var fetchCalls atomic.Uint32
			var invalidations atomic.Uint32
			app := &App{subscriptionModels: &desktopSubscriptionModelCatalog{fetch: func(context.Context, netclient.ProxySpec, string) ([]string, error) {
				fetchCalls.Add(1)
				return nil, nil
			}}}
			app.runtimeEvents.emit = func(context.Context, string, ...interface{}) {
				invalidations.Add(1)
			}
			const epoch = uint64(3)
			app.subscriptionModelRefreshEpoch.Store(epoch)
			waitCalls := 0
			app.retryDesktopSubscriptionModelsWithWait(ctx, userctx.Default(""), epoch, func(context.Context, time.Duration) bool {
				waitCalls++
				tt.beforeWait(app, cancel)
				return tt.waitResult
			})
			if waitCalls != 1 || fetchCalls.Load() != 0 || invalidations.Load() != 0 {
				t.Fatalf("wait/fetch/events = %d/%d/%d, want 1/0/0", waitCalls, fetchCalls.Load(), invalidations.Load())
			}
		})
	}
}

func TestDesktopSubscriptionCatalogReplacesUnavailableSessionDefault(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "")
	isolateDesktopUserDirs(t)
	root := t.TempDir()
	actor := userctx.Default(root)
	paths := config.NewProfilePathResolver(actor)
	cfg := config.Default()
	cfg.DefaultModel = "kwu/gpt-5.5"
	if err := cfg.SaveTo(paths.UserConfigPath()); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	scope := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "cloud-user", BindingGeneration: 1}
	app.billingWorkerRuntime.bindingActor = actor
	app.billingWorkerRuntime.bindingScope = scope
	app.billingWorkerRuntime.hasBinding = true
	app.subscriptionModels.replace(scope, cloud.MaaSKeyAccount{
		Status: cloud.MaaSKeyReady, KeyName: scope.CloudUserID, Key: "sk-user",
	}, []string{"model-z", "model-a"})
	model, _ := app.desktopNewSessionDefaultsForRoute(root, actor, modelbilling.BillingSubscriptionPool)
	if model != "kwu/model-z" {
		t.Fatalf("subscription session model = %q, want refreshed catalog default", model)
	}
	if model := app.availableSubscriptionModel(root, actor, "kwu/model-a"); model != "kwu/model-a" {
		t.Fatalf("available subscription model = %q, want preserved selection", model)
	}
	if model := app.availableSubscriptionModel(root, actor, "kwu/gpt-5.5"); model != "kwu/model-z" {
		t.Fatalf("stale subscription model = %q, want refreshed catalog default", model)
	}
}

func TestDesktopImplicitOfficialSubscriptionDefaultUsesUniversalDynamicModel(t *testing.T) {
	t.Setenv(cloudFakeEnv, "")
	isolateDesktopUserDirs(t)
	root := t.TempDir()
	actor := userctx.Default(root)
	paths := config.NewProfilePathResolver(actor)
	cfg := config.Default()
	cfg.DefaultModel = "kwu/gpt-5.4"
	cfg.Desktop.DefaultBillingSource = ""
	if err := cfg.SaveTo(paths.UserConfigPath()); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	scope := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "cloud-user", BindingGeneration: 1}
	app.billingWorkerRuntime.bindingActor = actor
	app.billingWorkerRuntime.bindingScope = scope
	app.billingWorkerRuntime.hasBinding = true
	app.subscriptionModels.replace(scope, cloud.MaaSKeyAccount{
		Status: cloud.MaaSKeyReady, KeyName: scope.CloudUserID, Key: "sk-user",
	}, []string{"gpt-5.4", "deepseek-v4-flash"})

	model, _ := app.desktopNewSessionDefaultsForRoute(root, actor, modelbilling.BillingSubscriptionPool)
	if model != "kwu/deepseek-v4-flash" {
		t.Fatalf("implicit subscription model = %q, want universal dynamic default", model)
	}
}

func TestDesktopSubscriptionCatalogIsForwardedToBootBeforeAuthorization(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "")
	isolateDesktopUserDirs(t)
	app := NewApp()
	actor := userctx.Default("")
	scope := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "cloud-user", BindingGeneration: 1}
	app.subscriptionModels = &desktopSubscriptionModelCatalog{}
	app.subscriptionModels.replace(scope, cloud.MaaSKeyAccount{
		Status: cloud.MaaSKeyReady, KeyName: scope.CloudUserID, Key: "sk-user",
	}, []string{"claude-opus-4-7", "gpt-5.4"})
	options := boot.Options{}
	err := app.applyDesktopB2BOptions(
		&options,
		actor,
		t.TempDir(),
		"kwu/claude-opus-4-7",
		string(modelbilling.BillingSubscriptionPool),
	)
	if modelbilling.ErrorCodeOf(err) != modelbilling.ErrorAuthRequired {
		t.Fatalf("subscription setup error = %v, want auth_required after catalog forwarding", err)
	}
	if len(options.SubscriptionModelCatalog) != 0 {
		t.Fatalf("unauthorized boot received subscription catalog: %v", options.SubscriptionModelCatalog)
	}
}

func TestDesktopDynamicSubscriptionModelRemainsResolvableAcrossRebuildLookups(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "")
	isolateDesktopUserDirs(t)
	root := t.TempDir()
	app := NewApp()
	actor := userctx.Default("")
	scope := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "cloud-user", BindingGeneration: 1}
	app.billingWorkerRuntime = &desktopBillingWorkerManager{
		bindingActor: actor, bindingScope: scope, hasBinding: true,
	}
	app.subscriptionModels = &desktopSubscriptionModelCatalog{}
	app.subscriptionModels.replace(scope, cloud.MaaSKeyAccount{
		Status: cloud.MaaSKeyReady, KeyName: scope.CloudUserID, Key: "sk-user",
	}, []string{"claude-opus-4-7", "gpt-5.4"})
	tab := &WorkspaceTab{
		ID: "dynamic-subscription", WorkspaceRoot: root,
		model: "kwu/claude-opus-4-7", billingSource: string(modelbilling.BillingSubscriptionPool),
	}
	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID

	entry, err := app.currentProviderEntryForTab(tab.ID)
	if err != nil || entry.Model != "claude-opus-4-7" {
		t.Fatalf("current dynamic subscription model = %+v, %v", entry, err)
	}
	resolved, fallback, err := app.resolvedModelForTab(tab)
	if err != nil || fallback || resolved != "kwu/claude-opus-4-7" {
		t.Fatalf("rebuild dynamic subscription model = %q fallback=%v err=%v", resolved, fallback, err)
	}

	tab.billingSource = string(modelbilling.BillingNewAPIByok)
	if _, _, err := app.resolvedModelForTab(tab); err == nil {
		t.Fatal("BYOK route resolved a model that exists only in the subscription catalog")
	}
}

func TestDesktopSubscriptionCatalogKeepsLastSuccessAndClearsToFallback(t *testing.T) {
	scope := cloud.BindingScope{ProfileID: "default", CloudUserID: "cloud-user", BindingGeneration: 1}
	otherScope := cloud.BindingScope{ProfileID: "default", CloudUserID: "other-user", BindingGeneration: 2}
	account := cloud.MaaSKeyAccount{
		Status: cloud.MaaSKeyReady, KeyName: scope.CloudUserID, Key: "sk-user",
	}
	catalog := &desktopSubscriptionModelCatalog{
		fetch: func(context.Context, netclient.ProxySpec, string) ([]string, error) {
			return []string{"model-live"}, nil
		},
	}
	models, err := catalog.fetchModels(context.Background(), netclient.ProxySpec{}, "sk-user")
	if err != nil {
		t.Fatal(err)
	}
	catalog.replace(scope, account, models)
	catalog.fetch = func(context.Context, netclient.ProxySpec, string) ([]string, error) {
		return nil, fmt.Errorf("temporary failure")
	}
	if _, err := catalog.fetchModels(context.Background(), netclient.ProxySpec{}, "sk-user"); err == nil {
		t.Fatal("failed refresh returned nil error")
	}
	if changed := catalog.replaceOnCatalogFetchError(scope, account, []string{"fallback-model"}); changed {
		t.Fatal("same-scope failed refresh reported a catalog change")
	}
	if got := strings.Join(catalog.current(scope), ","); got != "model-live" {
		t.Fatalf("same-scope catalog after failed refresh = %q", got)
	}
	if key, ok := catalog.credential(scope); !ok || key != "sk-user" {
		t.Fatalf("same-scope credential after failed refresh = %q, %v", key, ok)
	}
	if changed := catalog.replaceOnCatalogFetchError(otherScope, cloud.MaaSKeyAccount{
		Status: cloud.MaaSKeyReady, KeyName: otherScope.CloudUserID, Key: "sk-other",
	}, []string{"fallback-model"}); !changed {
		t.Fatal("cross-scope fallback did not report a catalog change")
	}
	if got := catalog.current(scope); len(got) != 0 {
		t.Fatalf("cross-scope fallback retained old catalog = %v", got)
	}
	if got := strings.Join(catalog.current(otherScope), ","); got != "fallback-model" {
		t.Fatalf("new-scope fallback catalog = %q", got)
	}
	catalog.clear()
	if got := catalog.current(scope); len(got) != 0 {
		t.Fatalf("cleared catalog = %v", got)
	}
}

func TestDesktopSubscriptionFallbackRejectsChangedBinding(t *testing.T) {
	actor := userctx.Default("")
	oldScope := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "old-user", BindingGeneration: 1}
	currentScope := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "current-user", BindingGeneration: 2}
	app := &App{
		billingWorkerRuntime: &desktopBillingWorkerManager{
			bindingActor: actor, bindingScope: currentScope, hasBinding: true,
		},
		subscriptionModels: &desktopSubscriptionModelCatalog{},
	}
	app.subscriptionModels.replace(currentScope, cloud.MaaSKeyAccount{
		Status: cloud.MaaSKeyReady, KeyName: currentScope.CloudUserID, Key: "sk-current",
	}, []string{"current-model"})
	const epoch = uint64(1)
	app.subscriptionModels.advanceRefreshEpoch(epoch)

	changed, err := app.replaceDesktopSubscriptionModelsAfterFetchError(actor, epoch, oldScope, cloud.MaaSKeyAccount{
		Status: cloud.MaaSKeyReady, KeyName: oldScope.CloudUserID, Key: "sk-old",
	})
	if changed {
		t.Fatal("changed-binding fallback reported a catalog change")
	}
	if modelbilling.ErrorCodeOf(err) != modelbilling.ErrorScopeMismatch {
		t.Fatalf("changed-binding fallback error = %v, want scope_mismatch", err)
	}
	if got := strings.Join(app.subscriptionModels.current(currentScope), ","); got != "current-model" {
		t.Fatalf("current binding catalog after stale failure = %q", got)
	}
	if key, ok := app.subscriptionModels.credential(currentScope); !ok || key != "sk-current" {
		t.Fatalf("current binding credential after stale failure = %q, %v", key, ok)
	}
}

func TestDesktopSubscriptionCatalogRefreshInvalidatesFallbackChanges(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "")
	isolateDesktopUserDirs(t)

	actor := userctx.ActorContext{ActorID: "local", ProfileID: "catalog-refresh-profile"}.Normalize()
	baseScope := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "catalog-user", BindingGeneration: 1}
	otherScope := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "other-catalog-user", BindingGeneration: 2}
	account := func(scope cloud.BindingScope, key string) cloud.MaaSKeyAccount {
		return cloud.MaaSKeyAccount{Status: cloud.MaaSKeyReady, KeyName: scope.CloudUserID, Key: key}
	}

	tests := []struct {
		name           string
		manual         bool
		initialScope   cloud.BindingScope
		initialAccount cloud.MaaSKeyAccount
		initialModels  []string
		currentScope   cloud.BindingScope
		currentAccount cloud.MaaSKeyAccount
		fetchModels    []string
		fetchErr       error
		wantModels     []string
		wantEvents     int
		wantError      bool
	}{
		{
			name:           "first failed fetch installs fallback",
			currentScope:   baseScope,
			currentAccount: account(baseScope, "sk-current"),
			fetchErr:       fmt.Errorf("catalog unavailable"),
			wantModels:     desktopAirClaudeSubscriptionModels,
			wantEvents:     1,
		},
		{
			name:           "temporary failure preserves successful catalog",
			initialScope:   baseScope,
			initialAccount: account(baseScope, "sk-current"),
			initialModels:  []string{"model-live"},
			currentScope:   baseScope,
			currentAccount: account(baseScope, "sk-current"),
			fetchErr:       fmt.Errorf("temporary catalog failure"),
			wantModels:     []string{"model-live"},
			wantEvents:     0,
		},
		{
			name:           "scope change installs fallback",
			initialScope:   baseScope,
			initialAccount: account(baseScope, "sk-old"),
			initialModels:  []string{"model-live"},
			currentScope:   otherScope,
			currentAccount: account(otherScope, "sk-current"),
			fetchErr:       fmt.Errorf("catalog unavailable after scope change"),
			wantModels:     desktopAirClaudeSubscriptionModels,
			wantEvents:     1,
		},
		{
			name:           "key change installs fallback",
			initialScope:   baseScope,
			initialAccount: account(baseScope, "sk-old"),
			initialModels:  []string{"model-live"},
			currentScope:   baseScope,
			currentAccount: account(baseScope, "sk-current"),
			fetchErr:       fmt.Errorf("catalog unavailable after key change"),
			wantModels:     desktopAirClaudeSubscriptionModels,
			wantEvents:     1,
		},
		{
			name:           "successful fetch still invalidates changed catalog",
			currentScope:   baseScope,
			currentAccount: account(baseScope, "sk-current"),
			fetchModels:    []string{"model-live", "model-next"},
			wantModels:     []string{"model-live", "model-next"},
			wantEvents:     1,
		},
		{
			name:           "manual first failed fetch installs fallback",
			manual:         true,
			currentScope:   baseScope,
			currentAccount: account(baseScope, "sk-current"),
			fetchErr:       fmt.Errorf("catalog unavailable"),
			wantModels:     desktopAirClaudeSubscriptionModels,
			wantEvents:     1,
			wantError:      true,
		},
		{
			name:           "manual temporary failure preserves successful catalog",
			manual:         true,
			initialScope:   baseScope,
			initialAccount: account(baseScope, "sk-current"),
			initialModels:  []string{"model-live"},
			currentScope:   baseScope,
			currentAccount: account(baseScope, "sk-current"),
			fetchErr:       fmt.Errorf("temporary catalog failure"),
			wantModels:     []string{"model-live"},
			wantEvents:     0,
			wantError:      true,
		},
		{
			name:           "manual successful fetch invalidates changed catalog",
			manual:         true,
			currentScope:   baseScope,
			currentAccount: account(baseScope, "sk-current"),
			fetchModels:    []string{"model-live", "model-next"},
			wantModels:     []string{"model-live", "model-next"},
			wantEvents:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			done := make(chan struct{})
			manager := &desktopBillingWorkerManager{
				authState:       cloud.AccountAuthenticated,
				bindingStatus:   cloud.BindingBound,
				authorized:      true,
				authorizedActor: actor,
				authorizedScope: tt.currentScope,
				bindingActor:    actor,
				bindingScope:    tt.currentScope,
				hasBinding:      true,
				active: &desktopBillingWorkerProcess{
					scope: modelbilling.TaskScope{
						ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudBinding: &tt.currentScope,
					},
					done: done,
				},
			}
			catalog := &desktopSubscriptionModelCatalog{fetch: func(context.Context, netclient.ProxySpec, string) ([]string, error) {
				return tt.fetchModels, tt.fetchErr
			}}
			if tt.initialAccount.Key != "" {
				catalog.replace(tt.initialScope, tt.initialAccount, tt.initialModels)
			}
			app := &App{
				ctx:                  ctx,
				currentActor:         actor,
				currentActorExplicit: true,
				cloudRuntime: &cloudRuntime{maasKeys: desktopMaaSKeyClientFunc(func(_ context.Context, scope cloud.BindingScope) (cloud.MaaSKeyAccount, error) {
					if scope != tt.currentScope {
						t.Fatalf("MaaS key scope = %+v, want %+v", scope, tt.currentScope)
					}
					return tt.currentAccount, nil
				})},
				billingWorkerRuntime: manager,
				subscriptionModels:   catalog,
			}
			eventTabs := make([]string, 0, 1)
			eventsDrained := make(chan struct{})
			app.runtimeEvents.emit = func(_ context.Context, name string, payload ...interface{}) {
				if name == "test:model-route-drained" {
					close(eventsDrained)
					return
				}
				if name != "model-route:changed" {
					return
				}
				if len(payload) != 1 {
					t.Errorf("model route change payload = %v, want one tab id", payload)
					return
				}
				tabID, ok := payload[0].(string)
				if !ok {
					t.Errorf("model route change tab id = %T, want string", payload[0])
					return
				}
				eventTabs = append(eventTabs, tabID)
			}

			// A repeated refresh must not emit again after the resulting catalog is stable.
			if tt.manual {
				for range 2 {
					err := app.RefreshOfficialModels()
					if (err != nil) != tt.wantError {
						t.Fatalf("RefreshOfficialModels error = %v, want error %v", err, tt.wantError)
					}
				}
			} else {
				app.refreshDesktopSubscriptionModelsBestEffort(actor)
				app.refreshDesktopSubscriptionModelsBestEffort(actor)
			}

			// Runtime delivery is asynchronous. A FIFO barrier also synchronizes
			// the callback's slice writes before the assertions read them.
			app.emitRuntimeEvent("test:model-route-drained")
			select {
			case <-eventsDrained:
			case <-time.After(time.Second):
				t.Fatal("model route events did not drain")
			}
			if got := catalog.current(tt.currentScope); !slices.Equal(got, tt.wantModels) {
				t.Fatalf("catalog models = %v, want %v", got, tt.wantModels)
			}
			if len(eventTabs) != tt.wantEvents {
				t.Fatalf("model route change events = %v, want %d", eventTabs, tt.wantEvents)
			}
			for _, tabID := range eventTabs {
				if tabID != "" {
					t.Fatalf("model route change tab = %q, want global", tabID)
				}
			}
		})
	}
}

func TestDesktopNonFakeSubscriptionReadPathsDoNotUseBYOKCredential(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "")
	t.Setenv("B2B_NONFAKE_SUBSCRIPTION_READ_KEY", "profile-key")
	isolateDesktopUserDirs(t)
	serverCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { serverCalls++ }))
	defer server.Close()
	root, actor, modelRef := writeDesktopBillingConfig(t, server.URL, "B2B_NONFAKE_SUBSCRIPTION_READ_KEY")
	if err := os.Unsetenv("B2B_NONFAKE_SUBSCRIPTION_READ_KEY"); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	app.ctx = context.Background()
	tab := &WorkspaceTab{
		ID:            "nonfake-subscription-read",
		WorkspaceRoot: root,
		Ready:         true,
		model:         modelRef,
		billingSource: string(modelbilling.BillingSubscriptionPool),
		actor:         actor,
	}
	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID
	app.setCurrentActorContext(actor)

	if _, err := app.desktopNonRouteConfig(); err != nil {
		t.Fatalf("subscription metadata config: %v", err)
	}
	_ = app.ModelsForTab(tab.ID)
	_ = app.Settings()
	if _, err := app.loadDesktopBotConfig(); err != nil {
		t.Fatalf("subscription bot config: %v", err)
	}
	if got := os.Getenv("B2B_NONFAKE_SUBSCRIPTION_READ_KEY"); got != "" {
		t.Fatalf("non-fake subscription read path hydrated BYOK credential %q", got)
	}
	if serverCalls != 0 {
		t.Fatalf("subscription model probe HTTP calls = %d, want 0", serverCalls)
	}
}

func TestDesktopStrictSubscriptionReadPathsDoNotHydrateProviderCredential(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	t.Setenv("B2B_STRICT_READ_PATH_KEY", "profile-key")
	isolateDesktopUserDirs(t)
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "B2B_STRICT_READ_PATH_KEY")
	if err := os.Unsetenv("B2B_STRICT_READ_PATH_KEY"); err != nil {
		t.Fatal(err)
	}
	app, tab, _, _, _ := newDesktopBillingTestTab(t, root, actor, modelRef, modelbilling.BillingSubscriptionPool)
	app.setCurrentActorContext(actor)
	tab.actor = actor
	defer tab.Ctrl.Close()

	assertNoCredential := func(t *testing.T) {
		t.Helper()
		if got := os.Getenv("B2B_STRICT_READ_PATH_KEY"); got != "" {
			t.Fatalf("strict subscription read path hydrated BYOK credential %q", got)
		}
	}
	for _, read := range []struct {
		name string
		run  func()
	}{
		{"non-route-config", func() { _, _ = app.desktopNonRouteConfig() }},
		{"models", func() { _ = app.ModelsForTab(tab.ID) }},
		{"settings", func() { _ = app.Settings() }},
		{"skills", func() { _ = app.SkillsSettings() }},
		{"mcp-view", func() { _ = app.mcpServersView() }},
		{"skill-roots", func() { _ = app.cachedSkillRootsView(tab.WorkspaceRoot, actor) }},
		{"onboarding", func() { _ = app.NeedsOnboarding() }},
		{"mcp-connect", func() { _, _ = app.connectConfiguredMCPServerForTab(tab, "missing") }},
		{"mcp-edit", func() { _, _, _ = app.desktopMCPServerForEdit("missing") }},
		{"project-mcp-override", func() { _, _ = app.removeProjectMCPOverride("missing") }},
		{"project-mcp-owner", func() { _ = app.desktopMCPServerOwnedByProjectMCPJSON("missing") }},
		{"sessions", func() { _ = app.ListSessions() }},
		{"autoplan-live", func() { app.applyAutoPlanToLiveControllers("off") }},
		{"reasoning-live", func() { app.applyReasoningLanguageToLiveControllers("auto") }},
		{"response-live", func() { app.applyResponseLanguageToLiveControllers("auto") }},
		{"bot-config", func() { _, _ = app.loadDesktopBotConfig() }},
	} {
		t.Run(read.name, func(t *testing.T) {
			if err := os.Unsetenv("B2B_STRICT_READ_PATH_KEY"); err != nil {
				t.Fatal(err)
			}
			read.run()
			assertNoCredential(t)
		})
	}
}

func TestDesktopRouteCredentialPolicyDefersLookupUntilExplicitBYOKSelection(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	t.Setenv("B2B_ROUTE_POLICY_KEY", "profile-key")
	isolateDesktopUserDirs(t)
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "B2B_ROUTE_POLICY_KEY")
	if err := os.Unsetenv("B2B_ROUTE_POLICY_KEY"); err != nil {
		t.Fatal(err)
	}

	metadata, metadataOnly, err := desktopCatalogConfigForBillingRoute(root, actor, modelbilling.BillingSubscriptionPool)
	if err != nil || !metadataOnly {
		t.Fatalf("subscription catalog = metadata:%v err:%v, want metadata-only", metadataOnly, err)
	}
	entry, ok := metadata.ResolveModel(modelRef)
	if !ok || entry.APIKey() != "" || os.Getenv("B2B_ROUTE_POLICY_KEY") != "" {
		t.Fatal("subscription metadata resolved a BYOK credential")
	}
	if _, err := desktopConfigForBillingRoute(root, actor, ""); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorContractInvalid || os.Getenv("B2B_ROUTE_POLICY_KEY") != "" {
		t.Fatalf("strict missing route error/env = %v/%q, want contract_invalid and no lookup", err, os.Getenv("B2B_ROUTE_POLICY_KEY"))
	}

	byok, err := desktopConfigForBillingRoute(root, actor, modelbilling.BillingNewAPIByok)
	if err != nil {
		t.Fatal(err)
	}
	entry, ok = byok.ResolveModel(modelRef)
	if !ok || entry.APIKey() != "profile-key" {
		t.Fatal("explicit BYOK selection did not resolve the current-profile credential")
	}
	if err := os.Unsetenv("B2B_ROUTE_POLICY_KEY"); err != nil {
		t.Fatal(err)
	}
	metadata, metadataOnly, err = desktopCatalogConfigForBillingRoute(root, actor, modelbilling.BillingSubscriptionPool)
	if err != nil || !metadataOnly {
		t.Fatalf("subscription catalog after BYOK = metadata:%v err:%v", metadataOnly, err)
	}
	entry, ok = metadata.ResolveModel(modelRef)
	if !ok || entry.APIKey() != "" || os.Getenv("B2B_ROUTE_POLICY_KEY") != "" {
		t.Fatal("metadata after BYOK retained or reloaded a credential")
	}
}

func TestDesktopBYOKRuntimeDoesNotMaterializeSubscriptionDependencies(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	t.Setenv("B2B_BYOK_RUNTIME_KEY", "profile-key")
	isolateDesktopUserDirs(t)
	root, actor, _ := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "B2B_BYOK_RUNTIME_KEY")
	runtime, err := NewApp().desktopBillingRuntime(actor, root, modelbilling.BillingNewAPIByok)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if runtime.dispatcher.Gateway != nil || runtime.initializer.inner.Subscriptions != nil || runtime.initializer.inner.CreditConfig != nil || runtime.initializer.inner.Freeze != nil {
		t.Fatalf("BYOK runtime contains subscription dependencies: dispatcher=%#v initializer=%#v", runtime.dispatcher.Gateway, runtime.initializer.inner)
	}
}

func TestDesktopBYOKRuntimeInjectsDomesticCallEvidenceResolver(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	t.Setenv("B2C_BYOK_EVIDENCE_KEY", "profile-key")
	isolateDesktopUserDirs(t)
	root, actor, _ := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "B2C_BYOK_EVIDENCE_KEY")
	app := NewApp()
	runtime, err := app.desktopBillingRuntime(actor, root, modelbilling.BillingNewAPIByok)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()

	resolver, preparer := desktopCallEvidenceComposition(t, runtime)
	if preparer.Preparation != app.pricePreparationReader || preparer.Source != "domestic" {
		t.Fatalf("BYOK call evidence preparation = reader:%p source:%q", preparer.Preparation, preparer.Source)
	}
	if preparer.CreditConfig != nil || preparer.CreditConfigAuthenticity != "" {
		t.Fatalf("BYOK call evidence unexpectedly contains credit config: provider=%T authenticity=%q", preparer.CreditConfig, preparer.CreditConfigAuthenticity)
	}
	if _, ok := resolver.Store.(*modelbilling.SQLiteCallEvidenceStore); !ok || resolver.IDs == nil {
		t.Fatalf("BYOK call evidence resolver = store:%T ids:%T", resolver.Store, resolver.IDs)
	}
}

func TestDesktopSubscriptionRuntimeInjectsFixtureCallEvidenceFacts(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	root, actor, _ := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "MISSING_B2C_SUBSCRIPTION_KEY")
	app := NewApp()
	_, binding := activateDesktopSubscriptionTestSession(t, app, actor)
	runtime, err := app.desktopBillingRuntime(actor, root, modelbilling.BillingSubscriptionPool)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()

	_, preparer := desktopCallEvidenceComposition(t, runtime)
	if preparer.Preparation != app.pricePreparationReader || preparer.Source != "domestic" {
		t.Fatalf("subscription call evidence preparation = reader:%p source:%q", preparer.Preparation, preparer.Source)
	}
	if preparer.CreditConfig != runtime.initializer.inner.CreditConfig {
		t.Fatalf("subscription call evidence credit config = %T, want initializer provider %T", preparer.CreditConfig, runtime.initializer.inner.CreditConfig)
	}
	if preparer.CreditConfigAuthenticity != newapipricing.SourceMockOnly {
		t.Fatalf("subscription call evidence authenticity = %q, want %q", preparer.CreditConfigAuthenticity, newapipricing.SourceMockOnly)
	}
	if _, ok := runtime.initializer.inner.Subscriptions.(desktopFixtureSubscriptionBootstrap); !ok {
		t.Fatalf("subscription bootstrap = %T", runtime.initializer.inner.Subscriptions)
	}
	creditConfig, err := runtime.initializer.inner.CreditConfig.GetCreditConfig(context.Background(), binding)
	if err != nil {
		t.Fatal(err)
	}
	if creditConfig.Version == nil || *creditConfig.Version != "cloud-credit-config-fixture-v1" {
		t.Fatalf("Desktop estimation credit config version = %v", creditConfig.Version)
	}
}

func TestDesktopStrictFixtureClocksMatchQuotaContractAndLeaveBYOKReal(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	t.Setenv("B3C_CLOCK_BYOK_KEY", "profile-key")
	isolateDesktopUserDirs(t)
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "B3C_CLOCK_BYOK_KEY")
	app := NewApp()

	subscription := boot.Options{}
	if err := app.applyDesktopB2BOptions(&subscription, actor, root, modelRef, string(modelbilling.BillingSubscriptionPool)); err != nil {
		t.Fatal(err)
	}
	defer subscription.ModelBillingCleanup()
	initializer := subscription.TaskContextInitializer.(desktopTaskContextInitializer).inner
	dispatcher := subscription.MainStreamDispatcher.(*modelbilling.MainExecutorDispatcher)
	assertDesktopBillingClock(t, "initializer", initializer.Now, []time.Time{
		desktopFixtureTimestamp(0, 0), desktopFixtureTimestamp(0, 0),
	})
	assertDesktopBillingClock(t, "dispatcher", dispatcher.Now, []time.Time{
		desktopFixtureTimestamp(0, 9), desktopFixtureTimestamp(0, 10), desktopFixtureTimestamp(0, 9), desktopFixtureTimestamp(0, 10),
	})
	assertDesktopBillingClock(t, "task lifecycle", subscription.TaskLifecycleNow, []time.Time{
		desktopFixtureTimestamp(0, 0), desktopFixtureTimestamp(0, 12), desktopFixtureTimestamp(0, 0), desktopFixtureTimestamp(0, 12),
	})

	byok := boot.Options{}
	if err := app.applyDesktopB2BOptions(&byok, actor, root, modelRef, string(modelbilling.BillingNewAPIByok)); err != nil {
		t.Fatal(err)
	}
	defer byok.ModelBillingCleanup()
	byokInitializer := byok.TaskContextInitializer.(desktopTaskContextInitializer).inner
	byokDispatcher := byok.MainStreamDispatcher.(*modelbilling.MainExecutorDispatcher)
	if byokInitializer.Now != nil || byokDispatcher.Now != nil || byok.TaskLifecycleNow != nil {
		t.Fatal("BYOK runtime received strict fixture clocks")
	}
}

func TestDesktopBillingWorkerManagerScopeAndProfileOwnership(t *testing.T) {
	isolateDesktopUserDirs(t)
	started := make(chan modelbilling.TaskScope, 8)
	stopped := make(chan modelbilling.TaskScope, 8)
	var factoryScopes []modelbilling.TaskScope
	var factoryDBs []*sql.DB
	manager := newDesktopBillingWorkerManager(func(db *sql.DB, scope modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		factoryScopes = append(factoryScopes, cloneDesktopBillingTaskScope(scope))
		factoryDBs = append(factoryDBs, db)
		return desktopBillingWorkerRuntime{run: func(ctx context.Context) error {
			started <- cloneDesktopBillingTaskScope(scope)
			<-ctx.Done()
			stopped <- cloneDesktopBillingTaskScope(scope)
			return ctx.Err()
		}}, nil
	})
	manager.waitTimeout = time.Second
	defaultOpenDB := manager.openDB
	profileOpenedAfterRelease := false
	manager.openDB = func(actor userctx.ActorContext) (*sql.DB, error) {
		if actor.ProfileID == "profile-c" {
			if manager.active != nil || manager.db != nil || manager.profileID != "" {
				t.Fatalf("new profile DB opened before old profile release: active=%v db=%v profile=%q", manager.active != nil, manager.db != nil, manager.profileID)
			}
			profileOpenedAfterRelease = true
		}
		return defaultOpenDB(actor)
	}
	manager.start(context.Background())
	defer func() { _ = manager.close() }()

	actorA := userctx.ActorContext{ActorID: "actor-a", ProfileID: "profile-a"}.Normalize()
	bindingA1 := cloud.BindingScope{ProfileID: actorA.ProfileID, CloudUserID: "cloud-a", BindingGeneration: 1}
	if err := manager.activate(actorA, bindingA1); err != nil {
		t.Fatal(err)
	}
	firstScope := receiveDesktopBillingScope(t, "first start", started)
	firstProcess, firstDB := manager.active, factoryDBs[0]

	if err := manager.activate(actorA, bindingA1); err != nil {
		t.Fatal(err)
	}
	if len(factoryScopes) != 1 || manager.active != firstProcess {
		t.Fatalf("same scope restarted worker: factories=%d processSame=%v", len(factoryScopes), manager.active == firstProcess)
	}

	bindingA2 := bindingA1
	bindingA2.BindingGeneration++
	if err := manager.activate(actorA, bindingA2); err != nil {
		t.Fatal(err)
	}
	if got := receiveDesktopBillingScope(t, "generation stop", stopped); !got.Equal(firstScope) {
		t.Fatalf("stopped generation scope = %+v, want %+v", got, firstScope)
	}
	secondScope := receiveDesktopBillingScope(t, "generation start", started)
	if secondScope.CloudBinding == nil || secondScope.CloudBinding.BindingGeneration != 2 || factoryDBs[1] != firstDB {
		t.Fatalf("generation switch = scope:%+v dbReused:%v", secondScope, factoryDBs[1] == firstDB)
	}

	actorB := actorA
	actorB.ActorID = "actor-b"
	if err := manager.activate(actorB, bindingA2); err != nil {
		t.Fatal(err)
	}
	if got := receiveDesktopBillingScope(t, "actor stop", stopped); !got.Equal(secondScope) {
		t.Fatalf("stopped actor scope = %+v, want %+v", got, secondScope)
	}
	thirdScope := receiveDesktopBillingScope(t, "actor start", started)
	if thirdScope.ActorID != actorB.ActorID || factoryDBs[2] != firstDB {
		t.Fatalf("actor switch = scope:%+v dbReused:%v", thirdScope, factoryDBs[2] == firstDB)
	}

	actorC := userctx.ActorContext{ActorID: "actor-c", ProfileID: "profile-c"}.Normalize()
	bindingC := cloud.BindingScope{ProfileID: actorC.ProfileID, CloudUserID: "cloud-c", BindingGeneration: 1}
	if err := manager.activate(actorC, bindingC); err != nil {
		t.Fatal(err)
	}
	if got := receiveDesktopBillingScope(t, "profile stop", stopped); !got.Equal(thirdScope) {
		t.Fatalf("stopped profile scope = %+v, want %+v", got, thirdScope)
	}
	lastScope := receiveDesktopBillingScope(t, "profile start", started)
	lastDB := factoryDBs[3]
	if lastScope.ProfileID != actorC.ProfileID || lastDB == firstDB {
		t.Fatalf("profile switch = scope:%+v dbChanged:%v", lastScope, lastDB != firstDB)
	}
	if !profileOpenedAfterRelease {
		t.Fatal("profile switch did not open the new DB after releasing the old profile")
	}
	if err := firstDB.PingContext(context.Background()); err == nil {
		t.Fatal("profile switch left the previous app-owned DB open")
	}

	if err := manager.close(); err != nil {
		t.Fatal(err)
	}
	if got := receiveDesktopBillingScope(t, "shutdown stop", stopped); !got.Equal(lastScope) {
		t.Fatalf("shutdown stopped scope = %+v, want %+v", got, lastScope)
	}
	if manager.active != nil || manager.db != nil {
		t.Fatalf("shutdown ownership = active:%v db:%v", manager.active != nil, manager.db != nil)
	}
	if err := lastDB.PingContext(context.Background()); err == nil {
		t.Fatal("shutdown left the app-owned DB open")
	}
}

func TestDesktopBillingWorkerManagerActivationFailureClosesDB(t *testing.T) {
	t.Run("first activation", func(t *testing.T) {
		isolateDesktopUserDirs(t)
		var candidateDB *sql.DB
		manager := newDesktopBillingWorkerManager(func(db *sql.DB, _ modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
			candidateDB = db
			return desktopBillingWorkerRuntime{}, fmt.Errorf("fixture factory failure")
		})
		manager.start(context.Background())
		defer func() { _ = manager.close() }()

		actor := userctx.ActorContext{ActorID: "failure-actor", ProfileID: "failure-profile"}.Normalize()
		binding := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "failure-cloud", BindingGeneration: 1}
		if err := manager.activate(actor, binding); err == nil || !strings.Contains(err.Error(), "fixture factory failure") {
			t.Fatalf("activation failure = %v", err)
		}
		if candidateDB == nil {
			t.Fatal("factory did not receive the candidate DB")
		}
		if manager.active != nil || manager.db != nil || manager.profileID != "" {
			t.Fatalf("failed first activation retained ownership: active=%v db=%v profile=%q", manager.active != nil, manager.db != nil, manager.profileID)
		}
		if err := candidateDB.PingContext(context.Background()); err == nil {
			t.Fatal("failed first activation left its candidate DB open")
		}
	})

	t.Run("same-profile replacement activation", func(t *testing.T) {
		isolateDesktopUserDirs(t)
		started := make(chan struct{}, 1)
		factoryCalls := 0
		var candidateDB *sql.DB
		manager := newDesktopBillingWorkerManager(func(db *sql.DB, _ modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
			factoryCalls++
			if factoryCalls == 2 {
				candidateDB = db
				return desktopBillingWorkerRuntime{}, fmt.Errorf("fixture factory failure")
			}
			return desktopBillingWorkerRuntime{run: func(ctx context.Context) error {
				started <- struct{}{}
				<-ctx.Done()
				return ctx.Err()
			}}, nil
		})
		manager.waitTimeout = time.Second
		manager.start(context.Background())
		defer func() { _ = manager.close() }()

		oldActor := userctx.ActorContext{ActorID: "old-actor", ProfileID: "old-profile"}.Normalize()
		oldBinding := cloud.BindingScope{ProfileID: oldActor.ProfileID, CloudUserID: "old-cloud", BindingGeneration: 1}
		if err := manager.activate(oldActor, oldBinding); err != nil {
			t.Fatal(err)
		}
		receiveDesktopBillingSignal(t, "worker before replacement failure", started)
		oldProcess, oldDB := manager.active, manager.db

		newActor := userctx.ActorContext{ActorID: "new-actor", ProfileID: oldActor.ProfileID}.Normalize()
		newBinding := cloud.BindingScope{ProfileID: newActor.ProfileID, CloudUserID: "new-cloud", BindingGeneration: 1}
		if err := manager.activate(newActor, newBinding); err == nil || !strings.Contains(err.Error(), "fixture factory failure") {
			t.Fatalf("replacement activation failure = %v", err)
		}
		if candidateDB == nil || candidateDB != oldDB {
			t.Fatalf("same-profile candidate DB = %p, old DB = %p", candidateDB, oldDB)
		}
		if manager.active != nil || manager.authorized || manager.db != oldDB || manager.profileID != oldActor.ProfileID {
			t.Fatalf("failed replacement did not remain inactive: active=%v authorized=%v dbSame=%v profile=%q", manager.active != nil, manager.authorized, manager.db == oldDB, manager.profileID)
		}
		select {
		case <-oldProcess.done:
		default:
			t.Fatal("failed replacement left the old worker running")
		}
		if err := oldDB.PingContext(context.Background()); err != nil {
			t.Fatalf("failed replacement closed the old DB: %v", err)
		}
		if err := candidateDB.PingContext(context.Background()); err != nil {
			t.Fatalf("failed same-profile replacement closed the shared DB: %v", err)
		}
	})

	t.Run("profile replacement activation", func(t *testing.T) {
		isolateDesktopUserDirs(t)
		started := make(chan struct{}, 1)
		factoryCalls := 0
		var candidateDB *sql.DB
		manager := newDesktopBillingWorkerManager(func(db *sql.DB, _ modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
			factoryCalls++
			if factoryCalls == 2 {
				candidateDB = db
				return desktopBillingWorkerRuntime{}, fmt.Errorf("fixture profile factory failure")
			}
			return desktopBillingWorkerRuntime{run: func(ctx context.Context) error {
				started <- struct{}{}
				<-ctx.Done()
				return ctx.Err()
			}}, nil
		})
		manager.waitTimeout = time.Second
		manager.start(context.Background())
		defer func() { _ = manager.close() }()

		oldActor := userctx.ActorContext{ActorID: "old-actor", ProfileID: "old-profile"}.Normalize()
		oldBinding := cloud.BindingScope{ProfileID: oldActor.ProfileID, CloudUserID: "old-cloud", BindingGeneration: 1}
		if err := manager.activate(oldActor, oldBinding); err != nil {
			t.Fatal(err)
		}
		receiveDesktopBillingSignal(t, "worker before profile replacement failure", started)
		oldDB := manager.db

		newActor := userctx.ActorContext{ActorID: "new-actor", ProfileID: "new-profile"}.Normalize()
		newBinding := cloud.BindingScope{ProfileID: newActor.ProfileID, CloudUserID: "new-cloud", BindingGeneration: 1}
		if err := manager.activate(newActor, newBinding); err == nil || !strings.Contains(err.Error(), "fixture profile factory failure") {
			t.Fatalf("profile replacement activation failure = %v", err)
		}
		if candidateDB == nil || candidateDB == oldDB {
			t.Fatalf("profile replacement candidate DB = %p, old DB = %p", candidateDB, oldDB)
		}
		if factoryCalls != 2 || manager.active != nil || manager.authorized || manager.db != nil || manager.profileID != "" {
			t.Fatalf("failed profile replacement did not remain inactive: factories=%d active=%v authorized=%v db=%p profile=%q", factoryCalls, manager.active != nil, manager.authorized, manager.db, manager.profileID)
		}
		for name, db := range map[string]*sql.DB{"old": oldDB, "candidate": candidateDB} {
			if err := db.PingContext(context.Background()); err == nil {
				t.Fatalf("failed profile replacement left %s DB open", name)
			}
		}
	})
}

func TestDesktopBillingWorkerManagerStopFailureClosesDB(t *testing.T) {
	isolateDesktopUserDirs(t)
	started := make(chan struct{}, 1)
	var workerDBs []*sql.DB
	var workerScopes []modelbilling.TaskScope
	manager := newDesktopBillingWorkerManager(func(db *sql.DB, scope modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		workerDBs = append(workerDBs, db)
		workerScopes = append(workerScopes, cloneDesktopBillingTaskScope(scope))
		return desktopBillingWorkerRuntime{run: func(ctx context.Context) error {
			started <- struct{}{}
			<-ctx.Done()
			return fmt.Errorf("fixture worker stop failure")
		}}, nil
	})
	manager.waitTimeout = time.Second
	manager.start(context.Background())
	defer func() { _ = manager.close() }()
	actor := userctx.ActorContext{ActorID: "stop-failure-actor", ProfileID: "stop-failure-profile"}.Normalize()
	binding := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "stop-failure-cloud", BindingGeneration: 1}
	if err := manager.activate(actor, binding); err != nil {
		t.Fatal(err)
	}
	receiveDesktopBillingSignal(t, "worker before stop failure", started)
	newActor := userctx.ActorContext{ActorID: "new-stop-failure-actor", ProfileID: "new-stop-failure-profile"}.Normalize()
	newBinding := cloud.BindingScope{ProfileID: newActor.ProfileID, CloudUserID: "new-stop-failure-cloud", BindingGeneration: 1}
	if err := manager.activate(newActor, newBinding); err == nil || !strings.Contains(err.Error(), "fixture worker stop failure") {
		t.Fatalf("scope switch stop failure = %v", err)
	}
	if manager.active != nil || manager.authorized || manager.db == nil || manager.profileID != actor.ProfileID {
		t.Fatalf("stop failure did not remain inactive: active=%v authorized=%v db=%v profile=%q", manager.active != nil, manager.authorized, manager.db != nil, manager.profileID)
	}
	if len(workerDBs) != 1 || len(workerScopes) != 1 || workerScopes[0].ProfileID != actor.ProfileID {
		t.Fatalf("stop failure workers = dbs:%d scopes:%+v, want only the stopped old worker", len(workerDBs), workerScopes)
	}
	if err := workerDBs[0].PingContext(context.Background()); err != nil {
		t.Fatalf("stop failure should retain the inactive profile DB: %v", err)
	}
}

func TestDesktopBillingWorkerManagerActivationTimeoutPreservesOldDB(t *testing.T) {
	isolateDesktopUserDirs(t)
	started := make(chan struct{}, 1)
	candidateStarted := make(chan struct{}, 1)
	release := make(chan struct{})
	factoryCalls := 0
	var candidateDB *sql.DB
	manager := newDesktopBillingWorkerManager(func(db *sql.DB, _ modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		factoryCalls++
		if factoryCalls == 1 {
			return desktopBillingWorkerRuntime{run: func(context.Context) error {
				started <- struct{}{}
				<-release
				return nil
			}}, nil
		}
		candidateDB = db
		return desktopBillingWorkerRuntime{run: func(ctx context.Context) error {
			candidateStarted <- struct{}{}
			<-ctx.Done()
			return ctx.Err()
		}}, nil
	})
	manager.waitTimeout = 10 * time.Millisecond
	manager.start(context.Background())
	released := false
	defer func() {
		if !released {
			close(release)
		}
		manager.waitTimeout = time.Second
		_ = manager.close()
	}()

	oldActor := userctx.ActorContext{ActorID: "timeout-old-actor", ProfileID: "timeout-old-profile"}.Normalize()
	oldBinding := cloud.BindingScope{ProfileID: oldActor.ProfileID, CloudUserID: "timeout-old-cloud", BindingGeneration: 1}
	if err := manager.activate(oldActor, oldBinding); err != nil {
		t.Fatal(err)
	}
	receiveDesktopBillingSignal(t, "worker before activation timeout", started)
	oldProcess, oldDB := manager.active, manager.db

	newActor := userctx.ActorContext{ActorID: "timeout-new-actor", ProfileID: "timeout-new-profile"}.Normalize()
	newBinding := cloud.BindingScope{ProfileID: newActor.ProfileID, CloudUserID: "timeout-new-cloud", BindingGeneration: 1}
	if err := manager.activate(newActor, newBinding); err == nil || !strings.Contains(err.Error(), "did not stop") {
		t.Fatalf("activation timeout = %v", err)
	}
	if candidateDB != nil || factoryCalls != 1 {
		t.Fatalf("timeout opened a new profile candidate: candidate=%p factories=%d", candidateDB, factoryCalls)
	}
	if manager.active != oldProcess || manager.db != oldDB || manager.profileID != oldActor.ProfileID {
		t.Fatalf("activation timeout changed ownership: processSame=%v dbSame=%v profile=%q", manager.active == oldProcess, manager.db == oldDB, manager.profileID)
	}
	if err := oldDB.PingContext(context.Background()); err != nil {
		t.Fatalf("activation timeout closed the live old DB: %v", err)
	}
	select {
	case <-candidateStarted:
		t.Fatal("activation timeout launched the candidate runner")
	default:
	}
	if err := manager.activate(oldActor, oldBinding); err == nil || !strings.Contains(err.Error(), "did not stop") {
		t.Fatalf("timed-out old worker was reauthorized = %v", err)
	}
	if manager.authorized {
		t.Fatal("timed-out old worker regained controller authorization")
	}
	if err := manager.authorizeSubscription(oldActor, oldBinding); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorSessionExpired {
		t.Fatalf("timed-out old scope authorization = %v, want session_expired", err)
	}
	if candidateDB != nil || factoryCalls != 1 {
		t.Fatalf("old-scope retry built a replacement before the timed-out worker exited: candidate=%p factories=%d", candidateDB, factoryCalls)
	}

	close(release)
	released = true
	manager.waitTimeout = time.Second
	if err := manager.close(); err != nil {
		t.Fatal(err)
	}
}

func TestDesktopBillingWorkerManagerPreservesDBUntilTimedOutWorkerStops(t *testing.T) {
	isolateDesktopUserDirs(t)
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var workerDB *sql.DB
	manager := newDesktopBillingWorkerManager(func(db *sql.DB, _ modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		workerDB = db
		return desktopBillingWorkerRuntime{run: func(context.Context) error {
			started <- struct{}{}
			<-release
			return nil
		}}, nil
	})
	manager.start(context.Background())
	released := false
	defer func() {
		if !released {
			close(release)
		}
		manager.waitTimeout = time.Second
		_ = manager.close()
	}()
	actor := userctx.ActorContext{ActorID: "timeout-actor", ProfileID: "timeout-profile"}.Normalize()
	binding := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "timeout-cloud", BindingGeneration: 1}
	if err := manager.activate(actor, binding); err != nil {
		t.Fatal(err)
	}
	receiveDesktopBillingSignal(t, "timeout worker start", started)

	manager.waitTimeout = 0
	if err := manager.close(); err == nil || !strings.Contains(err.Error(), "wait timeout must be positive") {
		t.Fatalf("invalid wait timeout close = %v", err)
	}
	if err := workerDB.PingContext(context.Background()); err != nil {
		t.Fatalf("invalid wait timeout closed a live worker DB: %v", err)
	}

	manager.waitTimeout = 10 * time.Millisecond
	if err := manager.close(); err == nil || !strings.Contains(err.Error(), "did not stop") {
		t.Fatalf("bounded close = %v", err)
	}
	if manager.active == nil || manager.db == nil {
		t.Fatal("timed-out close dropped live worker ownership")
	}
	if err := workerDB.PingContext(context.Background()); err != nil {
		t.Fatalf("timed-out close raced the live worker DB: %v", err)
	}

	close(release)
	released = true
	manager.waitTimeout = time.Second
	if err := manager.close(); err != nil {
		t.Fatal(err)
	}
	if err := workerDB.PingContext(context.Background()); err == nil {
		t.Fatal("completed worker left its DB open")
	}
}

func TestDesktopStrictRuntimeOwnsOneAppWorkerAcrossControllerAndBYOKRebuilds(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	t.Setenv("B3C_BYOK_RUNTIME_KEY", "profile-key")
	isolateDesktopUserDirs(t)
	root, actor, _ := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "B3C_BYOK_RUNTIME_KEY")
	app := NewApp()
	app.billingWorkerRuntime.start(context.Background())
	defer func() { _ = app.billingWorkerRuntime.close() }()

	first, err := app.desktopBillingRuntime(actor, root, modelbilling.BillingSubscriptionPool)
	if err != nil {
		t.Fatal(err)
	}
	binding := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "strict-runtime-cloud", BindingGeneration: 1}
	if err := app.activateDesktopBillingWorker(actor, binding); err != nil {
		first.Close()
		t.Fatal(err)
	}
	firstProcess := app.billingWorkerRuntime.active
	workerDB := app.billingWorkerRuntime.db
	if firstProcess == nil || workerDB == nil || first.db == workerDB {
		t.Fatalf("initial ownership = process:%v workerDB:%v independent:%v", firstProcess != nil, workerDB != nil, first.db != workerDB)
	}

	second, err := app.desktopBillingRuntime(actor, root, modelbilling.BillingSubscriptionPool)
	if err != nil {
		first.Close()
		t.Fatal(err)
	}
	if err := app.activateDesktopBillingWorker(actor, binding); err != nil {
		first.Close()
		second.Close()
		t.Fatal(err)
	}
	if app.billingWorkerRuntime.active != firstProcess || app.billingWorkerRuntime.db != workerDB {
		t.Fatal("same subscription scope restarted the app-owned worker")
	}
	first.Close()
	second.Close()
	if err := workerDB.PingContext(context.Background()); err != nil {
		t.Fatalf("controller close closed the app-owned DB: %v", err)
	}

	byok, err := app.desktopBillingRuntime(actor, root, modelbilling.BillingNewAPIByok)
	if err != nil {
		t.Fatal(err)
	}
	defer byok.Close()
	if app.billingWorkerRuntime.active != firstProcess || app.billingWorkerRuntime.db != workerDB {
		t.Fatal("BYOK rebuild was treated as a binding invalidation")
	}
	if byok.dispatcher.Gateway != nil || byok.initializer.inner.Subscriptions != nil || byok.initializer.inner.CreditConfig != nil || byok.initializer.inner.Freeze != nil {
		t.Fatalf("BYOK runtime contains subscription dependencies: dispatcher=%#v initializer=%#v", byok.dispatcher.Gateway, byok.initializer.inner)
	}

	if err := app.billingWorkerRuntime.close(); err != nil {
		t.Fatal(err)
	}
	if err := workerDB.PingContext(context.Background()); err == nil {
		t.Fatal("app shutdown left the worker DB open")
	}
}

func TestDesktopBuildFailureDoesNotCommitSubscriptionWorker(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "MISSING_B3C_BUILD_FAILURE_KEY")
	started := make(chan struct{}, 2)
	factoryCalls := 0
	manager := newDesktopBillingWorkerManager(func(_ *sql.DB, _ modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		factoryCalls++
		return desktopBillingWorkerRuntime{run: func(ctx context.Context) error {
			started <- struct{}{}
			<-ctx.Done()
			return ctx.Err()
		}}, nil
	})
	manager.waitTimeout = time.Second
	app := NewApp()
	app.billingWorkerRuntime = manager
	manager.start(context.Background())
	defer func() { _ = manager.close() }()

	oldBinding := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "existing-cloud-user", BindingGeneration: 7}
	if err := manager.activate(actor, oldBinding); err != nil {
		t.Fatal(err)
	}
	receiveDesktopBillingSignal(t, "existing worker start", started)
	oldProcess, oldDB := manager.active, manager.db
	var failedRuntime *desktopBillingRuntime
	app.desktopBillingRuntimeObserver = func(runtime *desktopBillingRuntime) {
		failedRuntime = runtime
	}

	ctrl, err := app.buildDesktopController(
		context.Background(),
		boot.Options{Model: "missing/model", WorkspaceRoot: root, Actor: actor, Sink: event.Discard},
		actor,
		root,
		modelRef,
		string(modelbilling.BillingSubscriptionPool),
	)
	if err == nil || ctrl != nil {
		if ctrl != nil {
			ctrl.Close()
		}
		t.Fatalf("invalid boot model controller/error = %T/%v", ctrl, err)
	}
	if failedRuntime == nil {
		t.Fatal("failed Build did not create the candidate billing runtime")
	}
	if err := failedRuntime.db.PingContext(context.Background()); err == nil {
		t.Fatal("failed Build left the candidate controller DB open")
	}
	if factoryCalls != 1 || manager.active != oldProcess || manager.db != oldDB {
		t.Fatalf("failed Build changed worker: factories=%d processSame=%v dbSame=%v", factoryCalls, manager.active == oldProcess, manager.db == oldDB)
	}
	if err := oldDB.PingContext(context.Background()); err != nil {
		t.Fatalf("failed Build closed the existing app-owned DB: %v", err)
	}
}

func TestDesktopControllerBuildDoesNotReactivateOrReplaceWorker(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "MISSING_B3C_ACTIVATION_FAILURE_KEY")
	started := make(chan struct{}, 1)
	factoryCalls := 0
	manager := newDesktopBillingWorkerManager(func(_ *sql.DB, _ modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		factoryCalls++
		if factoryCalls == 2 {
			return desktopBillingWorkerRuntime{}, fmt.Errorf("fixture activation stage failure")
		}
		return desktopBillingWorkerRuntime{run: func(ctx context.Context) error {
			started <- struct{}{}
			<-ctx.Done()
			return ctx.Err()
		}}, nil
	})
	manager.waitTimeout = time.Second
	app := NewApp()
	app.billingWorkerRuntime = manager
	manager.start(context.Background())
	defer func() { _ = manager.close() }()

	oldBinding := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "existing-cloud-user", BindingGeneration: 7}
	if err := manager.activate(actor, oldBinding); err != nil {
		t.Fatal(err)
	}
	receiveDesktopBillingSignal(t, "existing worker before activation failure", started)
	oldProcess, oldDB := manager.active, manager.db
	var candidateRuntime *desktopBillingRuntime
	app.desktopBillingRuntimeObserver = func(runtime *desktopBillingRuntime) {
		candidateRuntime = runtime
	}

	ctrl, err := app.buildDesktopController(
		context.Background(),
		boot.Options{Model: modelRef, WorkspaceRoot: root, Actor: actor, Sink: event.Discard},
		actor,
		root,
		modelRef,
		string(modelbilling.BillingSubscriptionPool),
	)
	if err != nil || ctrl == nil {
		t.Fatalf("controller build = %T/%v", ctrl, err)
	}
	if candidateRuntime == nil {
		ctrl.Close()
		t.Fatal("controller build did not create the candidate billing runtime")
	}
	if err := candidateRuntime.db.PingContext(context.Background()); err != nil {
		ctrl.Close()
		t.Fatalf("controller build closed its own DB early: %v", err)
	}
	ctrl.Close()
	if err := candidateRuntime.db.PingContext(context.Background()); err == nil {
		t.Fatal("controller close left its own DB open")
	}
	if factoryCalls != 1 || manager.active != oldProcess || manager.db != oldDB {
		t.Fatalf("controller build changed worker: factories=%d processSame=%v dbSame=%v", factoryCalls, manager.active == oldProcess, manager.db == oldDB)
	}
	if err := oldDB.PingContext(context.Background()); err != nil {
		t.Fatalf("controller build closed the existing app-owned DB: %v", err)
	}
}

func TestDesktopAppWorkerSurvivesSubscriptionControllerRebuildPaths(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "MISSING_B3C_REBUILD_KEY")
	cfg, err := config.LoadForRootMetadataWithActor(root, actor)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Providers[0].SupportedEfforts = []string{"low", "medium"}
	alias := cfg.Providers[0]
	alias.Name = "fixture-alias"
	cfg.Providers = append(cfg.Providers, alias)
	if err := cfg.SaveTo(config.NewProfilePathResolver(actor).UserConfigPath()); err != nil {
		t.Fatal(err)
	}
	aliasModelRef := alias.Name + "/" + alias.Model

	started := make(chan struct{}, 2)
	factoryCalls := 0
	manager := newDesktopBillingWorkerManager(func(_ *sql.DB, _ modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		factoryCalls++
		return desktopBillingWorkerRuntime{run: func(ctx context.Context) error {
			started <- struct{}{}
			<-ctx.Done()
			return ctx.Err()
		}}, nil
	})
	manager.waitTimeout = time.Second
	app := NewApp()
	app.billingWorkerRuntime = manager
	app.ctx = context.Background()
	app.readyHook = func() {}
	manager.start(app.ctx)
	var runtimes []*desktopBillingRuntime
	app.desktopBillingRuntimeObserver = func(runtime *desktopBillingRuntime) {
		runtimes = append(runtimes, runtime)
	}
	var tab *WorkspaceTab
	defer func() {
		if tab != nil && tab.Ctrl != nil {
			tab.Ctrl.Close()
		}
		_ = manager.close()
	}()
	binding := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "usr_fixture", BindingGeneration: 1}
	if err := manager.activate(actor, binding); err != nil {
		t.Fatal(err)
	}
	receiveDesktopBillingSignal(t, "app worker start", started)

	ctrl, err := app.buildDesktopController(
		context.Background(),
		boot.Options{Model: modelRef, WorkspaceRoot: root, Actor: actor, Sink: event.Discard},
		actor,
		root,
		modelRef,
		string(modelbilling.BillingSubscriptionPool),
	)
	if err != nil {
		t.Fatal(err)
	}
	tab = &WorkspaceTab{ID: "worker-owner-tab", WorkspaceRoot: root, Ready: true, model: modelRef, billingSource: string(modelbilling.BillingSubscriptionPool), Ctrl: ctrl, actor: actor}
	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID
	app.setCurrentActorContext(actor)
	activeProcess, workerDB := manager.active, manager.db
	currentRuntime := runtimes[0]
	assertStable := func(stage string) {
		t.Helper()
		if factoryCalls != 1 || manager.active != activeProcess || manager.db != workerDB {
			t.Fatalf("%s changed app worker: factories=%d processSame=%v dbSame=%v", stage, factoryCalls, manager.active == activeProcess, manager.db == workerDB)
		}
		if err := workerDB.PingContext(context.Background()); err != nil {
			t.Fatalf("%s closed app-owned DB: %v", stage, err)
		}
	}
	assertRebuild := func(stage string, action func() error) {
		t.Helper()
		previous := currentRuntime
		if err := action(); err != nil {
			t.Fatalf("%s: %v", stage, err)
		}
		currentRuntime = runtimes[len(runtimes)-1]
		if err := previous.db.PingContext(context.Background()); err == nil {
			t.Fatalf("%s left replaced controller DB open", stage)
		}
		assertStable(stage)
	}
	assertStable("initial build")

	secondCtrl, err := app.buildDesktopController(
		context.Background(),
		boot.Options{Model: modelRef, WorkspaceRoot: root, Actor: actor, Sink: event.Discard},
		actor,
		root,
		modelRef,
		string(modelbilling.BillingSubscriptionPool),
	)
	if err != nil {
		t.Fatal(err)
	}
	secondRuntime := runtimes[len(runtimes)-1]
	secondCtrl.Close()
	if err := secondRuntime.db.PingContext(context.Background()); err == nil {
		t.Fatal("second controller close left its DB open")
	}
	assertStable("second controller close")

	assertRebuild("model rebuild", func() error {
		return app.SetModelRouteForTab(tab.ID, aliasModelRef, string(modelbilling.BillingSubscriptionPool))
	})
	assertRebuild("effort rebuild", func() error { return app.SetEffortForTab(tab.ID, "low") })
	assertRebuild("token-mode rebuild", func() error { return app.SetTokenModeForTab(tab.ID, boot.TokenModeEconomy) })
	assertRebuild("skills rebuild", app.RefreshSkills)
	tab.Ctrl.Close()
	tab.Ctrl = nil
	assertStable("active controller close")
}

func desktopCallEvidenceComposition(t *testing.T, runtime *desktopBillingRuntime) (modelbilling.DefaultCallEvidenceResolver, newapipricing.CallEvidencePreparer) {
	t.Helper()
	resolver, ok := runtime.dispatcher.Evidence.(modelbilling.DefaultCallEvidenceResolver)
	if !ok {
		t.Fatalf("call evidence resolver = %T, want modelbilling.DefaultCallEvidenceResolver", runtime.dispatcher.Evidence)
	}
	preparer, ok := resolver.Preparer.(newapipricing.CallEvidencePreparer)
	if !ok {
		t.Fatalf("call evidence preparer = %T, want newapipricing.CallEvidencePreparer", resolver.Preparer)
	}
	return resolver, preparer
}

func TestDesktopNilControllerRebuildsKeepPersistedTabActor(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "kwu.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	actor := userctx.Default(root)
	actor.ProfileID = "review-profile"
	paths := config.NewProfilePathResolver(actor)
	if err := os.MkdirAll(filepath.Dir(paths.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	modelRef := "fixture/model_fixture_01"
	cfg := config.Default()
	cfg.DefaultModel = modelRef
	cfg.Providers = []config.ProviderEntry{{Name: "fixture", Kind: "openai", BaseURL: "https://fixture.invalid/v1", Model: "model_fixture_01", APIKeyEnv: "B2B_NIL_CONTROLLER_KEY", SupportedEfforts: []string{"low", "medium"}}}
	if err := cfg.SaveTo(paths.UserConfigPath()); err != nil {
		t.Fatal(err)
	}

	app, tab, _, _, _ := newDesktopBillingTestTab(t, root, actor, modelRef, modelbilling.BillingSubscriptionPool)
	app.setCurrentActorContext(actor)
	tab.actor = actor
	defer func() {
		if tab.Ctrl != nil {
			tab.Ctrl.Close()
		}
	}()

	for _, action := range []struct {
		name string
		run  func() error
	}{
		{"refresh-skills", app.RefreshSkills},
		{"effort", func() error { return app.SetEffortForTab(tab.ID, "low") }},
		{"token-mode", func() error { return app.SetTokenModeForTab(tab.ID, boot.TokenModeEconomy) }},
	} {
		t.Run(action.name, func(t *testing.T) {
			if tab.Ctrl != nil {
				tab.Ctrl.Close()
			}
			tab.Ctrl = nil
			tab.StartupErr = "previous build failed"
			if err := action.run(); err != nil {
				t.Fatalf("%s rebuild: %v", action.name, err)
			}
			if tab.Ctrl == nil || tab.Ctrl.ActorContext().ProfileID != actor.ProfileID {
				t.Fatalf("%s actor = %+v, want persisted profile %q", action.name, tab.Ctrl.ActorContext(), actor.ProfileID)
			}
		})
	}
}

func TestDesktopFakeSubscriptionUsesBootBuildDispatcherEndToEnd(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	t.Setenv("MISSING_B2B_BYOK_KEY", "configured-but-must-not-be-read")
	isolateDesktopUserDirs(t)
	serverCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { serverCalls++ }))
	defer server.Close()
	root, actor, modelRef := writeDesktopBillingConfig(t, server.URL, "MISSING_B2B_BYOK_KEY")
	metadata, err := config.LoadForRootMetadataWithActor(root, actor)
	if err != nil {
		t.Fatal(err)
	}
	// A subscription Build must succeed even though this kind cannot be
	// constructed as a local BYOK provider. If it attempted construction, boot
	// would fail before the fixture dispatcher can run.
	metadata.Providers[0].Kind = "not-a-local-provider"
	if err := metadata.SaveTo(config.NewProfilePathResolver(actor).UserConfigPath()); err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	activateDesktopSubscriptionTestSession(t, app, actor)
	opts := boot.Options{Model: modelRef, WorkspaceRoot: root, Actor: actor, Sink: event.Discard}
	if err := app.applyDesktopB2BOptions(&opts, actor, root, modelRef, string(modelbilling.BillingSubscriptionPool)); err != nil {
		t.Fatal(err)
	}
	if len(opts.SubscriptionModelCatalog) != len(desktopAirClaudeSubscriptionModels) {
		t.Fatalf("fake subscription catalog length = %d, want %d", len(opts.SubscriptionModelCatalog), len(desktopAirClaudeSubscriptionModels))
	}
	for i, model := range desktopAirClaudeSubscriptionModels {
		if opts.SubscriptionModelCatalog[i] != model {
			t.Fatalf("fake subscription catalog = %v, want %v", opts.SubscriptionModelCatalog, desktopAirClaudeSubscriptionModels)
		}
	}
	if !opts.RejectBackgroundSubagents {
		t.Fatal("Desktop strict subscription did not enable background subagent rejection")
	}
	capture := &desktopCapturingInitializer{base: opts.TaskContextInitializer}
	opts.TaskContextInitializer = capture
	dispatcher := opts.MainStreamDispatcher.(*modelbilling.MainExecutorDispatcher)
	gateway := &desktopCountingGateway{base: dispatcher.Gateway}
	dispatcher.Gateway = gateway
	ctrl, err := boot.Build(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	defer ctrl.Close()
	if err := ctrl.Run(context.Background(), "subscription fixture turn"); err != nil {
		t.Fatal(err)
	}
	if gateway.calls != 1 || serverCalls != 0 {
		t.Fatalf("gateway/BYOK calls = %d/%d", gateway.calls, serverCalls)
	}
	stored, err := opts.ModelBillingTaskStore.Get(context.Background(), capture.task.Scope, capture.task.TaskID)
	if err != nil || stored.FreezeRequest == nil || stored.Freeze == nil || stored.PricingEvidenceID == "" || stored.State != modelbilling.TaskUsagePending {
		t.Fatalf("subscription task = %+v, %v", stored, err)
	}
	calls, err := dispatcher.Calls.ListByTask(context.Background(), capture.task.Scope, capture.task.TaskID)
	if err != nil || len(calls) != 1 || calls[0].State != modelbilling.CallCompleted || calls[0].AttemptCount != 1 {
		t.Fatalf("subscription calls = %#v, %v", calls, err)
	}
}

func TestDesktopApprovedPlanPersistsMainThenExecutorInOneTask(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://must-not-run.invalid/v1", "MISSING_B2C_PLAN_KEY")
	app := NewApp()
	activateDesktopSubscriptionTestSession(t, app, actor)
	var ctrl *control.Controller
	sink := event.FuncSink(func(e event.Event) {
		if e.Kind == event.ApprovalRequest && e.Approval.Tool == "exit_plan_mode" {
			go ctrl.Approve(e.Approval.ID, true, false, false)
		}
	})
	opts := boot.Options{Model: modelRef, WorkspaceRoot: root, Actor: actor, Sink: sink}
	if err := app.applyDesktopB2BOptions(&opts, actor, root, modelRef, string(modelbilling.BillingSubscriptionPool)); err != nil {
		t.Fatal(err)
	}
	capture := &desktopCapturingInitializer{base: opts.TaskContextInitializer}
	opts.TaskContextInitializer = capture
	dispatcher := opts.MainStreamDispatcher.(*modelbilling.MainExecutorDispatcher)
	gateway := &desktopRoleGateway{}
	dispatcher.Gateway = gateway
	var err error
	ctrl, err = boot.Build(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	defer ctrl.Close()
	ctrl.SetPlanMode(true)
	if err := ctrl.RunTurn(context.Background(), "plan and implement this change"); err != nil {
		t.Fatal(err)
	}
	if len(gateway.requests) != 2 || gateway.requests[0].Role != modelbilling.CallRoleMain || gateway.requests[1].Role != modelbilling.CallRoleExecutor {
		t.Fatalf("gateway roles = %#v", gateway.requests)
	}
	if gateway.requests[0].TaskID != gateway.requests[1].TaskID || gateway.requests[0].TaskID != capture.task.TaskID || gateway.requests[0].CallID == gateway.requests[1].CallID {
		t.Fatalf("task/call continuity = %#v", gateway.requests)
	}
	rows, err := dispatcher.Calls.ListByTask(context.Background(), capture.task.Scope, capture.task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("persisted plan calls = %#v", rows)
	}
	roles := map[modelbilling.CallRole]bool{}
	for _, row := range rows {
		roles[row.Role] = true
		if row.TaskID != capture.task.TaskID || row.State != modelbilling.CallCompleted {
			t.Fatalf("plan call row = %+v", row)
		}
	}
	if !roles[modelbilling.CallRoleMain] || !roles[modelbilling.CallRoleExecutor] {
		t.Fatalf("persisted plan roles = %#v", roles)
	}
}

func TestDesktopAutoPlanClassifierPersistsClassifierAndMainInOneTask(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://must-not-run.invalid/v1", "MISSING_B2C_CLASSIFIER_KEY")
	paths := config.NewProfilePathResolver(actor)
	cfg, err := config.LoadForRootWithActor(root, actor)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Agent.AutoPlan = "on"
	cfg.Agent.AutoPlanClassifier = modelRef
	if err := cfg.SaveTo(paths.UserConfigPath()); err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	activateDesktopSubscriptionTestSession(t, app, actor)
	opts := boot.Options{Model: modelRef, WorkspaceRoot: root, Actor: actor, Sink: event.Discard}
	if err := app.applyDesktopB2BOptions(&opts, actor, root, modelRef, string(modelbilling.BillingSubscriptionPool)); err != nil {
		t.Fatal(err)
	}
	capture := &desktopCapturingInitializer{base: opts.TaskContextInitializer}
	opts.TaskContextInitializer = capture
	dispatcher := opts.MainStreamDispatcher.(*modelbilling.MainExecutorDispatcher)
	gateway := &desktopRoleGateway{}
	dispatcher.Gateway = gateway
	ctrl, err := boot.Build(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	defer ctrl.Close()
	if err := ctrl.RunTurn(context.Background(), "implement a small edit"); err != nil {
		t.Fatal(err)
	}
	if len(gateway.requests) != 2 || gateway.requests[0].Role != modelbilling.CallRoleClassifier || gateway.requests[1].Role != modelbilling.CallRoleMain {
		t.Fatalf("classifier/main requests = %#v", gateway.requests)
	}
	if gateway.requests[0].TaskID != capture.task.TaskID || gateway.requests[1].TaskID != capture.task.TaskID || gateway.requests[0].CallID == gateway.requests[1].CallID {
		t.Fatalf("classifier/main continuity = %#v", gateway.requests)
	}
}

func TestDesktopTaskTreePlannerPersistsPlannerAndMainInOneTask(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://must-not-run.invalid/v1", "MISSING_B2C_TASKTREE_KEY")
	app := NewApp()
	activateDesktopSubscriptionTestSession(t, app, actor)
	opts := boot.Options{
		Model:         modelRef,
		WorkspaceRoot: root,
		Actor:         actor,
		Sink:          event.Discard,
		AgentMode:     "task_tree",
	}
	if err := app.applyDesktopB2BOptions(&opts, actor, root, modelRef, string(modelbilling.BillingSubscriptionPool)); err != nil {
		t.Fatal(err)
	}
	capture := &desktopCapturingInitializer{base: opts.TaskContextInitializer}
	opts.TaskContextInitializer = capture
	dispatcher := opts.MainStreamDispatcher.(*modelbilling.MainExecutorDispatcher)
	gateway := &desktopTaskTreeGateway{}
	dispatcher.Gateway = gateway
	ctrl, err := boot.Build(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	defer ctrl.Close()
	if err := ctrl.RunTurn(context.Background(), "implement a task-tree change"); err != nil {
		t.Fatal(err)
	}
	if len(gateway.requests) != 2 {
		t.Fatalf("task-tree requests = %#v", gateway.requests)
	}
	wantRoles := []modelbilling.CallRole{modelbilling.CallRoleTasktreePlanner, modelbilling.CallRoleMain}
	for i, request := range gateway.requests {
		if request.Role != wantRoles[i] || request.TaskID != capture.task.TaskID {
			t.Fatalf("task-tree request[%d] = %+v", i, request)
		}
	}
	if gateway.requests[0].CallID == gateway.requests[1].CallID {
		t.Fatalf("task-tree call IDs were reused: %#v", gateway.requests)
	}
	rows, err := dispatcher.Calls.ListByTask(context.Background(), capture.task.Scope, capture.task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("task-tree rows = %#v", rows)
	}
	roles := map[modelbilling.CallRole]bool{}
	for _, row := range rows {
		roles[row.Role] = true
		if row.TaskID != capture.task.TaskID || row.State != modelbilling.CallCompleted {
			t.Fatalf("task-tree row = %+v", row)
		}
	}
	if !roles[modelbilling.CallRoleTasktreePlanner] || !roles[modelbilling.CallRoleMain] {
		t.Fatalf("task-tree persisted roles = %#v", roles)
	}
}

func TestDesktopBackgroundSubagentFailsBeforeResolutionTranscriptJobOrNetwork(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://must-not-run.invalid/v1", "MISSING_B2C_BACKGROUND_KEY")
	app := NewApp()
	activateDesktopSubscriptionTestSession(t, app, actor)
	opts := boot.Options{Model: modelRef, WorkspaceRoot: root, Actor: actor, Sink: event.Discard}
	if err := app.applyDesktopB2BOptions(&opts, actor, root, modelRef, string(modelbilling.BillingSubscriptionPool)); err != nil {
		t.Fatal(err)
	}
	capture := &desktopCapturingInitializer{base: opts.TaskContextInitializer}
	opts.TaskContextInitializer = capture
	dispatcher := opts.MainStreamDispatcher.(*modelbilling.MainExecutorDispatcher)
	gateway := &desktopBackgroundGateway{}
	dispatcher.Gateway = gateway
	ctrl, err := boot.Build(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	defer ctrl.Close()
	if err := ctrl.RunTurn(context.Background(), "start a background subagent"); err != nil {
		t.Fatal(err)
	}
	if len(gateway.requests) != 2 {
		t.Fatalf("gateway requests = %#v, want two root calls", gateway.requests)
	}
	for _, request := range gateway.requests {
		if request.Role != modelbilling.CallRoleMain {
			t.Fatalf("background reached derived model network: %+v", request)
		}
	}
	if status := ctrl.RuntimeStatus(); status.BackgroundJobs != 0 {
		t.Fatalf("background jobs = %+v", status)
	}
	var history strings.Builder
	for _, message := range ctrl.History() {
		history.WriteString(message.Content)
		history.WriteByte('\n')
	}
	if text := history.String(); !strings.Contains(text, "contract_invalid") || !strings.Contains(text, "background subagent is disabled") || strings.Contains(text, "unknown model") || strings.Contains(text, "Subagent reference") {
		t.Fatalf("background rejection history = %q", text)
	}
	rows, err := dispatcher.Calls.ListByTask(context.Background(), capture.task.Scope, capture.task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.Role == modelbilling.CallRoleSubagent {
			t.Fatalf("background persisted a subagent call: %+v", row)
		}
	}
}

func TestDesktopForegroundSubagentPersistsIndependentCallInParentTask(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://must-not-run.invalid/v1", "MISSING_B2C_SUBAGENT_KEY")
	app := NewApp()
	activateDesktopSubscriptionTestSession(t, app, actor)
	opts := boot.Options{Model: modelRef, WorkspaceRoot: root, Actor: actor, Sink: event.Discard}
	if err := app.applyDesktopB2BOptions(&opts, actor, root, modelRef, string(modelbilling.BillingSubscriptionPool)); err != nil {
		t.Fatal(err)
	}
	capture := &desktopCapturingInitializer{base: opts.TaskContextInitializer}
	opts.TaskContextInitializer = capture
	dispatcher := opts.MainStreamDispatcher.(*modelbilling.MainExecutorDispatcher)
	gateway := &desktopForegroundSubagentGateway{}
	dispatcher.Gateway = gateway
	ctrl, err := boot.Build(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	defer ctrl.Close()
	if err := ctrl.RunTurn(context.Background(), "delegate a foreground inspection"); err != nil {
		t.Fatal(err)
	}
	wantRoles := []modelbilling.CallRole{modelbilling.CallRoleMain, modelbilling.CallRoleSubagent, modelbilling.CallRoleMain, modelbilling.CallRoleMain}
	if len(gateway.requests) != len(wantRoles) {
		roles := make([]modelbilling.CallRole, len(gateway.requests))
		for i, request := range gateway.requests {
			roles[i] = request.Role
		}
		t.Fatalf("foreground subagent request count = %d, roles = %v; want %v including the explicit delivery manifest", len(gateway.requests), roles, wantRoles)
	}
	callRoles := map[string]modelbilling.CallRole{}
	for i, request := range gateway.requests {
		if request.Role != wantRoles[i] || request.TaskID != capture.task.TaskID || callRoles[request.CallID] != "" {
			t.Fatalf("foreground subagent request[%d]: role=%q task=%q call=%q", i, request.Role, request.TaskID, request.CallID)
		}
		callRoles[request.CallID] = request.Role
	}
	rows, err := dispatcher.Calls.ListByTask(context.Background(), capture.task.Scope, capture.task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != len(wantRoles) {
		t.Fatalf("foreground subagent row count = %d, want %d independent calls", len(rows), len(wantRoles))
	}
	for _, row := range rows {
		if row.Role != callRoles[row.CallID] || row.TaskID != capture.task.TaskID || row.State != modelbilling.CallCompleted {
			t.Fatalf("foreground subagent row: role=%q task=%q call=%q state=%q", row.Role, row.TaskID, row.CallID, row.State)
		}
	}
}

func TestDesktopBYOKPlannerUsesDomesticCrossModelEvidenceInParentTask(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	t.Setenv("B2C_PLANNER_BYOK_KEY", "test-key")
	isolateDesktopUserDirs(t)
	serverCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		serverCalls++
		reply := "Explain the implementation approach to the user; no tools needed."
		if serverCalls > 1 {
			reply = "implemented"
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2,\"total_tokens\":5}}\n\ndata: [DONE]\n\n", reply)
	}))
	defer server.Close()
	root, actor, modelRef := writeDesktopBillingConfig(t, server.URL, "B2C_PLANNER_BYOK_KEY")
	paths := config.NewProfilePathResolver(actor)
	cfg, err := config.LoadForRootWithActor(root, actor)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Providers = append(cfg.Providers, config.ProviderEntry{
		Name: "planner", Kind: "openai", BaseURL: server.URL, Model: "model_planner", APIKeyEnv: "B2C_PLANNER_BYOK_KEY",
	})
	cfg.Agent.PlannerModel = "planner/model_planner"
	if err := cfg.SaveTo(paths.UserConfigPath()); err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	var runtime *desktopBillingRuntime
	app.desktopBillingRuntimeObserver = func(value *desktopBillingRuntime) { runtime = value }
	opts := boot.Options{Model: modelRef, WorkspaceRoot: root, Actor: actor, Sink: event.Discard}
	if err := app.applyDesktopB2BOptions(&opts, actor, root, modelRef, string(modelbilling.BillingNewAPIByok)); err != nil {
		t.Fatal(err)
	}
	initializer := opts.TaskContextInitializer.(desktopTaskContextInitializer)
	if initializer.inner.Freeze != nil || initializer.inner.CreditConfig != nil {
		t.Fatalf("BYOK planner runtime contains cloud dependencies: %+v", initializer.inner)
	}
	capture := &desktopCapturingInitializer{base: initializer}
	opts.TaskContextInitializer = capture
	dispatcher := opts.MainStreamDispatcher.(*modelbilling.MainExecutorDispatcher)
	if dispatcher.Gateway != nil {
		t.Fatalf("BYOK planner dispatcher contains gateway %T", dispatcher.Gateway)
	}
	ctrl, err := boot.Build(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	defer ctrl.Close()
	if err := ctrl.RunTurn(context.Background(), "implement a multi-file refactor across frontend and backend"); err != nil {
		t.Fatal(err)
	}
	if serverCalls != 2 {
		t.Fatalf("BYOK planner HTTP calls = %d, want planner and executor", serverCalls)
	}
	rows, err := dispatcher.Calls.ListByTask(context.Background(), capture.task.Scope, capture.task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("BYOK planner rows = %#v", rows)
	}
	var plannerRow, mainRow *modelbilling.ModelCall
	for i := range rows {
		row := &rows[i]
		switch row.Role {
		case modelbilling.CallRolePlanner:
			plannerRow = row
		case modelbilling.CallRoleMain:
			mainRow = row
		}
		if row.TaskID != capture.task.TaskID || row.BillingSource != modelbilling.BillingNewAPIByok || !row.Scope.Equal(capture.task.Scope) {
			t.Fatalf("BYOK planner row ownership = %+v", row)
		}
	}
	if plannerRow == nil || mainRow == nil || plannerRow.ModelID != "model_planner" || mainRow.ModelID != capture.task.InitialModelID || plannerRow.PricingEvidenceID == capture.task.PricingEvidenceID || mainRow.PricingEvidenceID != capture.task.PricingEvidenceID {
		t.Fatalf("BYOK planner/main evidence rows = planner:%+v main:%+v task:%+v", plannerRow, mainRow, capture.task)
	}
	if runtime == nil {
		t.Fatal("desktop billing runtime observer did not run")
	}
	var evidenceModel, evidenceGroup, snapshotID string
	if err := runtime.db.QueryRowContext(context.Background(), `SELECT model_id, pricing_group, snapshot_id FROM pricing_evidence WHERE evidence_id=?`, plannerRow.PricingEvidenceID).Scan(&evidenceModel, &evidenceGroup, &snapshotID); err != nil {
		t.Fatal(err)
	}
	if evidenceModel != "model_planner" || evidenceGroup != "__kwu_unavailable__" || !strings.Contains(snapshotID, "pricing-unavailable-v2:domestic:") {
		t.Fatalf("BYOK planner evidence = model:%q group:%q snapshot:%q", evidenceModel, evidenceGroup, snapshotID)
	}
}

func TestDesktopManualCompactCreatesIndependentTaskAndCompactionCall(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://must-not-run.invalid/v1", "MISSING_B2C_COMPACT_KEY")
	app := NewApp()
	activateDesktopSubscriptionTestSession(t, app, actor)
	opts := boot.Options{Model: modelRef, WorkspaceRoot: root, Actor: actor, Sink: event.Discard}
	if err := app.applyDesktopB2BOptions(&opts, actor, root, modelRef, string(modelbilling.BillingSubscriptionPool)); err != nil {
		t.Fatal(err)
	}
	capture := &desktopCapturingInitializer{base: opts.TaskContextInitializer}
	opts.TaskContextInitializer = capture
	dispatcher := opts.MainStreamDispatcher.(*modelbilling.MainExecutorDispatcher)
	gateway := &desktopRoleGateway{}
	dispatcher.Gateway = gateway
	ctrl, err := boot.Build(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	defer ctrl.Close()
	for i := 0; i < 4; i++ {
		if err := ctrl.RunTurn(context.Background(), fmt.Sprintf("foreground turn %d with enough content to compact", i)); err != nil {
			t.Fatal(err)
		}
	}
	previousTask := capture.task.TaskID
	if err := ctrl.Compact(context.Background(), "keep the useful facts"); err != nil {
		t.Fatal(err)
	}
	if len(capture.tasks) != 5 || capture.task.TaskID == previousTask {
		t.Fatalf("manual compact tasks = %#v", capture.tasks)
	}
	lastRequest := gateway.requests[len(gateway.requests)-1]
	if lastRequest.Role != modelbilling.CallRoleCompaction || lastRequest.TaskID != capture.task.TaskID {
		t.Fatalf("manual compact request = %+v, task = %+v", lastRequest, capture.task)
	}
	rows, err := dispatcher.Calls.ListByTask(context.Background(), capture.task.Scope, capture.task.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Role != modelbilling.CallRoleCompaction || rows[0].State != modelbilling.CallCompleted {
		t.Fatalf("manual compact rows = %#v", rows)
	}
	stored, err := opts.ModelBillingTaskStore.Get(context.Background(), capture.task.Scope, capture.task.TaskID)
	if err != nil || stored.State != modelbilling.TaskUsagePending {
		t.Fatalf("manual compact task = %+v, %v", stored, err)
	}
}

func TestDesktopFakeBYOKUsesBootBuildWithoutCloudDependencies(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	t.Setenv("B2B_BYOK_KEY", "test-key")
	isolateDesktopUserDirs(t)
	serverCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		serverCalls++
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"BYOK reply\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	root, actor, modelRef := writeDesktopBillingConfig(t, server.URL, "B2B_BYOK_KEY")
	app := NewApp()
	opts := boot.Options{Model: modelRef, WorkspaceRoot: root, Actor: actor, Sink: event.Discard}
	if err := app.applyDesktopB2BOptions(&opts, actor, root, modelRef, string(modelbilling.BillingNewAPIByok)); err != nil {
		t.Fatal(err)
	}
	if !opts.RejectBackgroundSubagents {
		t.Fatal("Desktop strict BYOK did not enable background subagent rejection")
	}
	initializer := opts.TaskContextInitializer.(desktopTaskContextInitializer)
	freeze := &desktopFreezeSpy{base: initializer.inner.Freeze}
	initializer.inner.Freeze = freeze
	capture := &desktopCapturingInitializer{base: initializer}
	opts.TaskContextInitializer = capture
	dispatcher := opts.MainStreamDispatcher.(*modelbilling.MainExecutorDispatcher)
	gateway := &desktopCountingGateway{base: dispatcher.Gateway}
	dispatcher.Gateway = gateway
	ctrl, err := boot.Build(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	defer ctrl.Close()
	if err := ctrl.Run(context.Background(), "BYOK turn"); err != nil {
		t.Fatal(err)
	}
	if serverCalls != 1 || gateway.calls != 0 || freeze.calls != 0 {
		t.Fatalf("BYOK/gateway/freeze calls = %d/%d/%d", serverCalls, gateway.calls, freeze.calls)
	}
	stored, err := opts.ModelBillingTaskStore.Get(context.Background(), capture.task.Scope, capture.task.TaskID)
	if err != nil || stored.State != modelbilling.TaskCompleted || stored.Freeze != nil || stored.FreezeRequest != nil {
		t.Fatalf("BYOK task = %+v, %v", stored, err)
	}
}

func TestDesktopBillingRuntimeClosesOnControllerCloseAndBuildFailure(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "MISSING_B2B_BYOK_KEY")
	app := NewApp()
	runtime, err := app.desktopBillingRuntime(actor, root, modelbilling.BillingSubscriptionPool)
	if err != nil {
		t.Fatal(err)
	}
	opts := desktopRuntimeBootOptions(runtime, actor, root, modelRef)
	ctrl, err := boot.Build(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	ctrl.Close()
	ctrl.Close()
	if err := runtime.db.PingContext(context.Background()); err == nil {
		t.Fatal("controller Close left desktop billing DB open")
	}

	runtime, err = app.desktopBillingRuntime(actor, root, modelbilling.BillingSubscriptionPool)
	if err != nil {
		t.Fatal(err)
	}
	opts = desktopRuntimeBootOptions(runtime, actor, root, modelRef)
	opts.BillingRouteSelection = modelbilling.BillingRouteSelection{}
	if _, err := boot.Build(context.Background(), opts); err == nil {
		t.Fatal("Build accepted missing billing route")
	}
	if err := runtime.db.PingContext(context.Background()); err == nil {
		t.Fatal("failed Build leaked desktop billing DB")
	}
}

func TestDesktopRefreshSkillsRebuildsThroughB2BSubscriptionRoute(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	t.Setenv("MISSING_B2B_BYOK_KEY", "configured-but-must-not-be-read")
	isolateDesktopUserDirs(t)
	serverCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { serverCalls++ }))
	defer server.Close()
	root, actor, modelRef := writeDesktopBillingConfig(t, server.URL, "MISSING_B2B_BYOK_KEY")
	app, tab, runtimes, gateways, _ := newDesktopBillingTestTab(t, root, actor, modelRef, modelbilling.BillingSubscriptionPool)
	defer func() { tab.Ctrl.Close() }()
	if err := app.RefreshSkills(); err != nil {
		t.Fatalf("RefreshSkills: %v", err)
	}
	if len(*runtimes) != 2 || len(*gateways) != 2 {
		t.Fatalf("runtime/gateway rebuilds = %d/%d, want 2/2", len(*runtimes), len(*gateways))
	}
	if err := tab.Ctrl.Run(context.Background(), "subscription after skills refresh"); err != nil {
		t.Fatalf("refreshed subscription turn: %v", err)
	}
	if (*gateways)[1].calls != 1 || serverCalls != 0 {
		t.Fatalf("refreshed gateway/BYOK HTTP calls = %d/%d", (*gateways)[1].calls, serverCalls)
	}
	if err := (*runtimes)[0].db.PingContext(context.Background()); err == nil {
		t.Fatal("RefreshSkills retained the replaced controller DB")
	}
	var calls, tasks int
	if err := (*runtimes)[1].db.QueryRow(`SELECT COUNT(*) FROM model_calls WHERE state = 'completed'`).Scan(&calls); err != nil || calls != 1 {
		t.Fatalf("refreshed write-ahead completed calls = %d, %v", calls, err)
	}
	if err := (*runtimes)[1].db.QueryRow(`SELECT COUNT(*) FROM model_tasks WHERE state = 'usage_pending'`).Scan(&tasks); err != nil || tasks != 1 {
		t.Fatalf("refreshed TaskLifecycleRunner terminal tasks = %d, %v", tasks, err)
	}

	// Rebuild once more with a configured planner. The planner is routed through
	// B2C evidence preparation and must fail on the missing domestic cache before
	// gateway or local BYOK transport while the subscription route stays strict.
	metadata, err := config.LoadForRootMetadataWithActor(root, actor)
	if err != nil {
		t.Fatal(err)
	}
	metadata.Agent.PlannerModel = "planner/model_planner"
	metadata.Providers = append(metadata.Providers, config.ProviderEntry{Name: "planner", Kind: "openai", BaseURL: server.URL, Model: "model_planner", APIKeyEnv: "MISSING_B2B_BYOK_KEY"})
	if err := metadata.SaveTo(config.NewProfilePathResolver(actor).UserConfigPath()); err != nil {
		t.Fatal(err)
	}
	if err := app.RefreshSkills(); err != nil {
		t.Fatalf("RefreshSkills planner rebuild: %v", err)
	}
	if err := tab.Ctrl.Run(context.Background(), "planner remains blocked"); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorPricingUnavailable || !strings.Contains(err.Error(), "snapshot_unavailable") {
		t.Fatalf("refreshed planner error = %v, want pricing_unavailable snapshot_unavailable", err)
	}
	if len(*gateways) != 3 || (*gateways)[2].calls != 0 || serverCalls != 0 {
		t.Fatalf("refreshed derived route gateway/BYOK HTTP = %d/%d", (*gateways)[2].calls, serverCalls)
	}
}

func TestDesktopRefreshSkillsRebuildsThroughB2BBYOKRoute(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	t.Setenv("B2B_REFRESH_BYOK_KEY", "profile-key")
	isolateDesktopUserDirs(t)
	serverCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		serverCalls++
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"BYOK reply\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	root, actor, modelRef := writeDesktopBillingConfig(t, server.URL, "B2B_REFRESH_BYOK_KEY")
	app, tab, runtimes, gateways, freezes := newDesktopBillingTestTab(t, root, actor, modelRef, modelbilling.BillingNewAPIByok)
	defer func() { tab.Ctrl.Close() }()
	if err := app.RefreshSkills(); err != nil {
		t.Fatalf("RefreshSkills: %v", err)
	}
	if err := tab.Ctrl.Run(context.Background(), "BYOK after skills refresh"); err != nil {
		t.Fatalf("refreshed BYOK turn: %v", err)
	}
	if serverCalls != 1 || (*gateways)[1].calls != 0 || (*freezes)[1].calls != 0 {
		t.Fatalf("refreshed BYOK/gateway/freeze calls = %d/%d/%d", serverCalls, (*gateways)[1].calls, (*freezes)[1].calls)
	}
	if err := (*runtimes)[0].db.PingContext(context.Background()); err == nil {
		t.Fatal("BYOK RefreshSkills retained the replaced controller DB")
	}
	var tasks int
	if err := (*runtimes)[1].db.QueryRow(`SELECT COUNT(*) FROM model_tasks WHERE state = 'completed' AND billing_source = 'newapi_byok'`).Scan(&tasks); err != nil || tasks != 1 {
		t.Fatalf("refreshed BYOK task terminal = %d, %v", tasks, err)
	}
}

func TestDesktopTokenModeB2BRuntimeLifecycleIsTransactional(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "MISSING_B2B_BYOK_KEY")
	app, tab, runtimes, _, _ := newDesktopBillingTestTab(t, root, actor, modelRef, modelbilling.BillingSubscriptionPool)
	if err := app.SetTokenModeForTab(tab.ID, boot.TokenModeEconomy); err != nil {
		t.Fatalf("successful token-mode rebuild: %v", err)
	}
	if len(*runtimes) != 2 {
		t.Fatalf("successful token-mode runtimes = %d, want 2", len(*runtimes))
	}
	if err := (*runtimes)[0].db.PingContext(context.Background()); err == nil {
		t.Fatal("successful token-mode rebuild left old DB open")
	}
	if err := (*runtimes)[1].db.PingContext(context.Background()); err != nil {
		t.Fatalf("new token-mode DB unusable: %v", err)
	}

	paths := config.NewProfilePathResolver(actor)
	cfg, err := config.LoadForRootMetadataWithActor(root, actor)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Agent.DefaultExpert = "missing-after-token-build"
	if err := cfg.SaveTo(paths.UserConfigPath()); err != nil {
		t.Fatal(err)
	}
	oldCtrl := tab.Ctrl
	if err := app.SetTokenModeForTab(tab.ID, boot.TokenModeFull); err == nil {
		t.Fatal("token-mode rebuild accepted an invalid replacement controller")
	}
	if len(*runtimes) != 3 {
		t.Fatalf("failed token-mode runtimes = %d, want 3", len(*runtimes))
	}
	if err := (*runtimes)[2].db.PingContext(context.Background()); err == nil {
		t.Fatal("failed token-mode rebuild leaked replacement DB")
	}
	if tab.Ctrl != oldCtrl {
		t.Fatal("failed token-mode rebuild replaced the old controller")
	}
	if err := (*runtimes)[1].db.PingContext(context.Background()); err != nil {
		t.Fatalf("failed token-mode rebuild closed usable old DB: %v", err)
	}
	if err := tab.Ctrl.Run(context.Background(), "old controller remains usable"); err != nil {
		t.Fatalf("old controller after failed token-mode rebuild: %v", err)
	}
	tab.Ctrl.Close()
	tab.Ctrl.Close()
	if err := (*runtimes)[1].db.PingContext(context.Background()); err == nil {
		t.Fatal("double controller Close left current DB open")
	}
}

func TestDesktopTokenModeRepeatedRebuildsRetainOnlyCurrentRuntime(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "MISSING_B2B_BYOK_KEY")
	app, tab, runtimes, _, _ := newDesktopBillingTestTab(t, root, actor, modelRef, modelbilling.BillingSubscriptionPool)
	for _, mode := range []string{boot.TokenModeEconomy, boot.TokenModeFull, boot.TokenModeEconomy} {
		if err := app.SetTokenModeForTab(tab.ID, mode); err != nil {
			t.Fatalf("SetTokenModeForTab(%q): %v", mode, err)
		}
	}
	if len(*runtimes) != 4 {
		t.Fatalf("repeated token-mode runtimes = %d, want 4", len(*runtimes))
	}
	for i, runtime := range (*runtimes)[:len(*runtimes)-1] {
		if err := runtime.db.PingContext(context.Background()); err == nil {
			t.Fatalf("old runtime %d remained open", i)
		}
	}
	current := (*runtimes)[len(*runtimes)-1]
	if err := current.db.PingContext(context.Background()); err != nil {
		t.Fatalf("current runtime closed during rebuild: %v", err)
	}
	tab.Ctrl.Close()
	tab.Ctrl.Close()
	if err := current.db.PingContext(context.Background()); err == nil {
		t.Fatal("current runtime remained open after idempotent Close")
	}
}

func TestDesktopSetModelRouteForTabRebuildsWhenOnlyBillingSourceChanges(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	t.Setenv("B2B_ROUTE_SWITCH_KEY", "profile-key")
	isolateDesktopUserDirs(t)
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "B2B_ROUTE_SWITCH_KEY")
	app, tab, runtimes, _, _ := newDesktopBillingTestTab(t, root, actor, modelRef, modelbilling.BillingSubscriptionPool)
	app.setCurrentActorContext(actor)
	tab.actor = actor
	defer func() {
		if tab.Ctrl != nil {
			tab.Ctrl.Close()
		}
	}()

	first := tab.Ctrl
	if err := app.SetModelRouteForTab(tab.ID, modelRef, string(modelbilling.BillingNewAPIByok)); err != nil {
		t.Fatalf("subscription -> BYOK: %v", err)
	}
	if tab.Ctrl == first || tab.billingSource != string(modelbilling.BillingNewAPIByok) || tab.model != modelRef {
		t.Fatalf("subscription -> BYOK did not atomically replace route/controller: ctrlChanged=%v route=%q model=%q", tab.Ctrl != first, tab.billingSource, tab.model)
	}
	if len(*runtimes) != 2 {
		t.Fatalf("subscription -> BYOK runtimes = %d, want 2", len(*runtimes))
	}
	if err := (*runtimes)[0].db.PingContext(context.Background()); err == nil {
		t.Fatal("subscription -> BYOK retained old runtime DB")
	}
	metadata, metadataErr := desktopConfigForBillingRoute(root, actor, modelbilling.BillingSubscriptionPool)
	if metadataErr != nil {
		t.Fatalf("subscription metadata after BYOK = %v", metadataErr)
	}
	if _, ok := metadata.ResolveModel(modelRef); !ok {
		t.Fatalf("BYOK edit changed subscription model metadata: %+v", metadata.Providers)
	}

	second := tab.Ctrl
	if err := app.SetModelRouteForTab(tab.ID, modelRef, string(modelbilling.BillingSubscriptionPool)); err != nil {
		t.Fatalf("BYOK -> subscription: %v", err)
	}
	if tab.Ctrl == second || tab.billingSource != string(modelbilling.BillingSubscriptionPool) || tab.model != modelRef {
		t.Fatalf("BYOK -> subscription did not atomically replace route/controller: ctrlChanged=%v route=%q model=%q", tab.Ctrl != second, tab.billingSource, tab.model)
	}
	if len(*runtimes) != 3 {
		t.Fatalf("BYOK -> subscription runtimes = %d, want 3", len(*runtimes))
	}
	if err := (*runtimes)[1].db.PingContext(context.Background()); err == nil {
		t.Fatal("BYOK -> subscription retained old runtime DB")
	}
}

func TestDesktopSetModelRouteForTabPreservesLiveSessionWithoutClosingIt(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	t.Setenv("B2B_ROUTE_LIFECYCLE_KEY", "profile-key")
	isolateDesktopUserDirs(t)
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "B2B_ROUTE_LIFECYCLE_KEY")
	app, tab, runtimes, _, _ := newDesktopBillingTestTab(t, root, actor, modelRef, modelbilling.BillingNewAPIByok)
	activateDesktopSubscriptionTestSession(t, app, actor)
	app.setCurrentActorContext(actor)
	tab.actor = actor

	// Rebuild the fixture's initial discard-sink controller with the production
	// tab sink so lifecycle emission follows the same synchronous path as Desktop.
	tab.Ctrl.Close()
	sink := &tabEventSink{tabID: tab.ID, app: app, ctx: context.Background()}
	tab.sink = sink
	oldCtrl, err := app.buildDesktopController(context.Background(), boot.Options{
		Model: modelRef, WorkspaceRoot: root, Actor: actor, Sink: sink,
	}, actor, root, modelRef, string(modelbilling.BillingNewAPIByok))
	if err != nil {
		t.Fatalf("build live BYOK controller: %v", err)
	}
	tab.Ctrl = oldCtrl
	oldRuntime := (*runtimes)[len(*runtimes)-1]
	sessionPath := filepath.Join(t.TempDir(), "continued-session.jsonl")
	oldCtrl.Resume(agent.NewSessionFromMessages([]provider.Message{
		{Role: provider.RoleSystem, Content: "old system prompt"},
		{Role: provider.RoleUser, Content: "keep this user turn"},
	}), sessionPath)
	tab.SessionPath = sessionPath
	releaseEntered := make(chan struct{})
	releaseOld := make(chan struct{})
	releasedOld := false
	oldCtrlWithBarrier := &blockingReleaseSession{
		SessionAPI: oldCtrl,
		entered:    releaseEntered,
		release:    releaseOld,
	}
	tab.Ctrl = oldCtrlWithBarrier

	done := make(chan error, 1)
	go func() {
		done <- app.SetModelRouteForTab(tab.ID, modelRef, string(modelbilling.BillingSubscriptionPool))
	}()
	switchFinished := false
	defer func() {
		if !releasedOld {
			close(releaseOld)
		}
		if !switchFinished {
			select {
			case <-done:
			case <-time.After(3 * time.Second):
			}
		}
		app.mu.RLock()
		currentCtrl := tab.Ctrl
		app.mu.RUnlock()
		if currentCtrl != nil {
			currentCtrl.ReleaseResources()
		}
	}()
	select {
	case <-releaseEntered:
	case <-time.After(3 * time.Second):
		t.Fatal("BYOK -> subscription did not begin releasing the old controller")
	}

	tabsDone := make(chan []TabMeta, 1)
	go func() { tabsDone <- app.ListTabs() }()
	select {
	case tabs := <-tabsDone:
		if len(tabs) != 1 || tabs[0].ID != tab.ID || tabs[0].SendAvailability.State != tabSendStarting {
			t.Fatalf("ListTabs during route switch = %+v", tabs)
		}
	case <-time.After(time.Second):
		t.Fatal("ListTabs blocked while the old controller was releasing resources")
	}

	app.mu.RLock()
	replacement := tab.Ctrl
	readyDuringRelease := tab.Ready
	modelDuringRelease := tab.model
	sourceDuringRelease := tab.billingSource
	app.mu.RUnlock()
	if replacement == nil || replacement == oldCtrlWithBarrier || readyDuringRelease || modelDuringRelease != modelRef || sourceDuringRelease != string(modelbilling.BillingSubscriptionPool) {
		t.Fatalf("published route during cleanup = ctrlChanged:%v ready:%v model:%q source:%q", replacement != oldCtrlWithBarrier, readyDuringRelease, modelDuringRelease, sourceDuringRelease)
	}

	for name, call := range map[string]func() error{
		"submit":         func() error { return app.SubmitToTab(tab.ID, "must not run") },
		"submit display": func() error { return app.SubmitDisplayToTab(tab.ID, "must not run", "must not run") },
		"run shell":      func() error { return app.RunShellForTab(tab.ID, "echo must-not-run") },
		"steer":          func() error { return app.SteerForTab(tab.ID, "must not run") },
	} {
		if err := call(); err == nil || !strings.Contains(err.Error(), "still starting") {
			t.Fatalf("%s during route switch = %v, want still-starting error", name, err)
		}
	}
	for name, call := range map[string]func() error{
		"model route":   func() error { return app.SetModelRouteForTab(tab.ID, modelRef, string(modelbilling.BillingNewAPIByok)) },
		"effort":        func() error { return app.SetEffortForTab(tab.ID, "low") },
		"token mode":    func() error { return app.SetTokenModeForTab(tab.ID, boot.TokenModeEconomy) },
		"approval mode": func() error { return app.SetToolApprovalModeForTab(tab.ID, control.ToolApprovalYolo) },
	} {
		if err := call(); err == nil || !strings.Contains(err.Error(), "still starting") {
			t.Fatalf("%s mutation during route switch = %v, want still-starting error", name, err)
		}
	}
	app.mu.RLock()
	modeDuringRelease := tab.mode
	goalDuringRelease := tab.goal
	app.mu.RUnlock()
	app.SetModeForTab(tab.ID, "yolo")
	app.SetGoalForTab(tab.ID, "must not replace bootstrap state")
	app.mu.RLock()
	modeAfterRejectedMutation := tab.mode
	goalAfterRejectedMutation := tab.goal
	app.mu.RUnlock()
	if modeAfterRejectedMutation != modeDuringRelease || goalAfterRejectedMutation != goalDuringRelease {
		t.Fatalf("void mutations changed transitioning tab: mode %q -> %q, goal %q -> %q", modeDuringRelease, modeAfterRejectedMutation, goalDuringRelease, goalAfterRejectedMutation)
	}
	if err := replacement.RunTurn(context.Background(), "must not run through held controller"); !errors.Is(err, control.ErrTurnNotAdmitted) {
		t.Fatalf("replacement controller admission during route switch = %v, want %v", err, control.ErrTurnNotAdmitted)
	}

	close(releaseOld)
	releasedOld = true
	select {
	case err := <-done:
		switchFinished = true
		if err != nil {
			t.Fatalf("BYOK -> subscription: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("BYOK -> subscription deadlocked while releasing the old controller")
	}
	if tab.Ctrl == oldCtrl || tab.model != modelRef || tab.billingSource != string(modelbilling.BillingSubscriptionPool) {
		t.Fatalf("route/controller commit = ctrlChanged:%v model:%q source:%q", tab.Ctrl != oldCtrl, tab.model, tab.billingSource)
	}
	if err := oldRuntime.db.PingContext(context.Background()); err == nil {
		t.Fatal("same-session replacement did not release the old billing runtime")
	}
	if tabs := app.ListTabs(); len(tabs) != 1 || tabs[0].ID != tab.ID {
		t.Fatalf("ListTabs after route switch = %+v", tabs)
	}
	if tab.Ctrl.SessionPath() != sessionPath {
		t.Fatalf("continued session path = %q, want %q", tab.Ctrl.SessionPath(), sessionPath)
	}
	foundUserTurn := false
	for _, message := range tab.Ctrl.History() {
		if message.Role == provider.RoleUser && message.Content == "keep this user turn" {
			foundUserTurn = true
		}
	}
	if !foundUserTurn {
		t.Fatalf("continued history lost the existing user turn: %+v", tab.Ctrl.History())
	}
	for _, record := range sink.operationalDiagnosticTrace().CaptureForTab(tab.ID, true).Records {
		if string(record.Kind) == string(event.SessionClosed) {
			t.Fatalf("same-session controller replacement emitted session.closed: %+v", record)
		}
	}
	if err := tab.Ctrl.Run(context.Background(), "replacement controller remains usable"); err != nil {
		t.Fatalf("replacement controller run: %v", err)
	}
}

func TestDesktopSetModelRouteForTabRebuildsNilAndStartupErrorControllers(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	t.Setenv("B2B_ROUTE_RECOVERY_KEY", "profile-key")
	isolateDesktopUserDirs(t)
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "B2B_ROUTE_RECOVERY_KEY")
	app, tab, runtimes, _, _ := newDesktopBillingTestTab(t, root, actor, modelRef, modelbilling.BillingSubscriptionPool)
	app.setCurrentActorContext(actor)
	tab.actor = actor
	tab.Ctrl.Close()
	tab.Ctrl = nil
	tab.model = ""
	tab.StartupErr = "initial build failed"

	if err := app.SetModelRouteForTab(tab.ID, modelRef, string(modelbilling.BillingNewAPIByok)); err != nil {
		t.Fatalf("recover nil controller with same default model: %v", err)
	}
	if tab.Ctrl == nil || tab.model != modelRef || tab.billingSource != string(modelbilling.BillingNewAPIByok) || tab.StartupErr != "" {
		t.Fatalf("nil-controller recovery = ctrl:%v model:%q source:%q startup:%q", tab.Ctrl != nil, tab.model, tab.billingSource, tab.StartupErr)
	}
	if len(*runtimes) != 2 {
		t.Fatalf("nil-controller recovery runtimes = %d, want 2", len(*runtimes))
	}

	writeHistoryTestSession(t, tab.SessionPath, "history before startup retry")
	tab.Ctrl.Close()
	tab.Ctrl = nil
	tab.StartupErr = "previous startup failure"
	if err := app.SetModelRouteForTab(tab.ID, modelRef, string(modelbilling.BillingNewAPIByok)); err != nil {
		t.Fatalf("recover startup error with same route: %v", err)
	}
	if tab.Ctrl == nil || tab.StartupErr != "" {
		t.Fatalf("startup-error recovery = ctrl:%v startup:%q", tab.Ctrl != nil, tab.StartupErr)
	}
	if len(*runtimes) != 3 {
		t.Fatalf("startup-error recovery runtimes = %d, want 3", len(*runtimes))
	}
	tab.Ctrl.Close()
}

func TestDesktopSetModelRouteForTabKeepsOldRouteWhenBYOKBuildFails(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	t.Setenv("B2B_ROUTE_MISSING_KEY", "")
	isolateDesktopUserDirs(t)
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "B2B_ROUTE_MISSING_KEY")
	app, tab, _, _, _ := newDesktopBillingTestTab(t, root, actor, modelRef, modelbilling.BillingSubscriptionPool)
	app.setCurrentActorContext(actor)
	tab.actor = actor
	defer func() {
		if tab.Ctrl != nil {
			tab.Ctrl.Close()
		}
	}()
	app.mu.Lock()
	app.saveTabsLocked()
	app.mu.Unlock()
	oldCtrl, oldModel, oldSource := tab.Ctrl, tab.model, tab.billingSource

	if err := app.SetModelRouteForTab(tab.ID, modelRef, string(modelbilling.BillingNewAPIByok)); err == nil {
		t.Fatal("missing BYOK credential unexpectedly committed a new route")
	}
	if tab.Ctrl != oldCtrl || tab.model != oldModel || tab.billingSource != oldSource {
		t.Fatalf("failed BYOK route changed live tab: ctrlSame=%v model=%q source=%q", tab.Ctrl == oldCtrl, tab.model, tab.billingSource)
	}
	if saved := app.loadTabsFile(); len(saved.Tabs) != 1 || saved.Tabs[0].Model != oldModel || saved.Tabs[0].BillingSource != oldSource {
		t.Fatalf("failed BYOK route changed persisted tab: %+v", saved.Tabs)
	}
	if err := oldCtrl.Run(context.Background(), "old subscription controller remains usable"); err != nil {
		t.Fatalf("old subscription controller after failed BYOK build: %v", err)
	}
}

func TestDesktopSetModelRouteForTabRejectsActiveWorkBeforeChangingRoute(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	t.Setenv("B2B_ROUTE_ACTIVE_KEY", "profile-key")
	isolateDesktopUserDirs(t)
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "B2B_ROUTE_ACTIVE_KEY")
	app, tab, _, _, _ := newDesktopBillingTestTab(t, root, actor, modelRef, modelbilling.BillingSubscriptionPool)
	app.setCurrentActorContext(actor)
	tab.actor = actor
	tab.Ctrl.Close()
	started := make(chan struct{})
	activeCtrl := control.New(control.Options{Actor: actor, Runner: desktopBillingRunnerFunc(func(ctx context.Context, _ string) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})})
	tab.Ctrl = activeCtrl
	done := make(chan error, 1)
	go func() { done <- activeCtrl.RunTurn(context.Background(), "hold") }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("active controller did not start")
	}

	if err := app.SetModelRouteForTab(tab.ID, modelRef, string(modelbilling.BillingNewAPIByok)); err == nil {
		t.Fatal("active route switch unexpectedly succeeded")
	}
	if tab.Ctrl != activeCtrl || tab.billingSource != string(modelbilling.BillingSubscriptionPool) || tab.model != modelRef {
		t.Fatalf("active route switch changed tab: ctrlSame=%v source=%q model=%q", tab.Ctrl == activeCtrl, tab.billingSource, tab.model)
	}
	activeCtrl.Cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("active controller did not stop")
	}
	activeCtrl.Close()
}

func TestDesktopSetModelRouteForTabKeepsHealthySameRouteFast(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	isolateDesktopUserDirs(t)
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "B2B_ROUTE_FAST_KEY")
	app, tab, runtimes, _, _ := newDesktopBillingTestTab(t, root, actor, modelRef, modelbilling.BillingSubscriptionPool)
	app.setCurrentActorContext(actor)
	tab.actor = actor
	defer tab.Ctrl.Close()
	oldCtrl := tab.Ctrl
	if err := app.SetModelRouteForTab(tab.ID, modelRef, string(modelbilling.BillingSubscriptionPool)); err != nil {
		t.Fatalf("same healthy route: %v", err)
	}
	if tab.Ctrl != oldCtrl || len(*runtimes) != 1 {
		t.Fatalf("same healthy route rebuilt controller: ctrlSame=%v runtimes=%d", tab.Ctrl == oldCtrl, len(*runtimes))
	}
}

func TestDesktopSubscriptionModelsForTabHidesBYOKWithoutCredential(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	t.Setenv("B2B_METADATA_ONLY_KEY", "")
	isolateDesktopUserDirs(t)
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "B2B_METADATA_ONLY_KEY")
	paths := config.NewProfilePathResolver(actor)
	cfg := config.LoadForEditWithPaths(paths)
	cfg.Desktop.ProviderAccess = []string{"fixture"}
	if err := cfg.SaveTo(paths.UserConfigPath()); err != nil {
		t.Fatal(err)
	}
	app, tab, _, _, _ := newDesktopBillingTestTab(t, root, actor, modelRef, modelbilling.BillingSubscriptionPool)
	app.setCurrentActorContext(actor)
	tab.actor = actor
	defer tab.Ctrl.Close()

	models := app.ModelsForTab(tab.ID)
	seen := map[string]bool{}
	for _, model := range models {
		if model.Ref == modelRef {
			seen[model.BillingSource] = true
		}
	}
	if !seen[string(modelbilling.BillingSubscriptionPool)] || seen[string(modelbilling.BillingNewAPIByok)] {
		t.Fatalf("subscription metadata catalog = %+v, want only platform route for %q", models, modelRef)
	}

	// A restored strict tab without a source is not allowed to boot, but the
	// switcher must still show metadata candidates so the user can choose one.
	tab.billingSource = ""
	models = app.ModelsForTab(tab.ID)
	seen = map[string]bool{}
	for _, model := range models {
		if model.Ref == modelRef {
			seen[model.BillingSource] = true
		}
	}
	if !seen[string(modelbilling.BillingSubscriptionPool)] || seen[string(modelbilling.BillingNewAPIByok)] {
		t.Fatalf("strict missing-source metadata catalog = %+v, want only platform route for %q", models, modelRef)
	}

	if _, err := config.SetCredentialWithPaths(paths, "B2B_METADATA_ONLY_KEY", "profile-key"); err != nil {
		t.Fatal(err)
	}
	models = app.ModelsForTab(tab.ID)
	seen = map[string]bool{}
	for _, model := range models {
		if model.Ref == modelRef {
			seen[model.BillingSource] = true
		}
	}
	if !seen[string(modelbilling.BillingSubscriptionPool)] || !seen[string(modelbilling.BillingNewAPIByok)] {
		t.Fatalf("catalog with user credential = %+v, want platform and BYOK routes for %q", models, modelRef)
	}
}

func TestDesktopStrictCatalogOnlyAllowsEmptyMissingSource(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	t.Setenv("B2B_INVALID_CATALOG_KEY", "profile-key")
	isolateDesktopUserDirs(t)
	serverCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { serverCalls++ }))
	defer server.Close()
	root, actor, modelRef := writeDesktopBillingConfig(t, server.URL, "B2B_INVALID_CATALOG_KEY")
	if err := os.Unsetenv("B2B_INVALID_CATALOG_KEY"); err != nil {
		t.Fatal(err)
	}

	policy, err := desktopBillingRouteLoadPolicyFor("", true)
	if err != nil || !policy.metadataOnly {
		t.Fatalf("empty catalog policy = %+v, %v; want metadata-only", policy, err)
	}
	cfg, metadataOnly, err := desktopCatalogConfigForBillingRoute(root, actor, "")
	if err != nil || !metadataOnly {
		t.Fatalf("empty catalog config = metadata:%v err:%v; want metadata-only", metadataOnly, err)
	}
	entry, ok := cfg.ResolveModel(modelRef)
	if !ok || entry.APIKey() != "" || os.Getenv("B2B_INVALID_CATALOG_KEY") != "" {
		t.Fatal("empty-source catalog loaded a provider credential")
	}

	app := NewApp()
	tab := &WorkspaceTab{ID: "invalid-catalog", WorkspaceRoot: root, Ready: true, model: modelRef, actor: actor}
	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID
	app.setCurrentActorContext(actor)
	seen := map[string]bool{}
	for _, model := range app.ModelsForTab(tab.ID) {
		if model.Ref == modelRef {
			seen[model.BillingSource] = true
		}
	}
	if !seen[string(modelbilling.BillingSubscriptionPool)] || seen[string(modelbilling.BillingNewAPIByok)] {
		t.Fatalf("empty-source catalog candidates = %+v, want only platform route for %q", seen, modelRef)
	}
	if serverCalls != 0 {
		t.Fatalf("empty-source catalog made %d network calls, want 0", serverCalls)
	}

	invalid := modelbilling.BillingSource("invalid_source")
	if _, err := desktopBillingRouteLoadPolicyFor(invalid, true); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorContractInvalid {
		t.Fatalf("invalid catalog policy error = %v, want contract_invalid", err)
	}
	if _, _, err := desktopCatalogConfigForBillingRoute(root, actor, invalid); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorContractInvalid {
		t.Fatalf("invalid catalog loader error = %v, want contract_invalid", err)
	}
	tab.billingSource = string(invalid)
	if models := app.ModelsForTab(tab.ID); len(models) != 0 {
		t.Fatalf("invalid-source catalog models = %+v, want empty", models)
	}
	options := boot.Options{Model: modelRef, WorkspaceRoot: root, Actor: actor, Sink: event.Discard}
	ctrl, err := app.buildDesktopController(context.Background(), options, actor, root, modelRef, string(invalid))
	if modelbilling.ErrorCodeOf(err) != modelbilling.ErrorContractInvalid || ctrl != nil {
		t.Fatalf("invalid-source controller = %T/%v, want nil contract_invalid", ctrl, err)
	}
	if os.Getenv("B2B_INVALID_CATALOG_KEY") != "" || serverCalls != 0 {
		t.Fatalf("invalid-source route hydrated credential or used network: key=%q calls=%d", os.Getenv("B2B_INVALID_CATALOG_KEY"), serverCalls)
	}
	if _, err := desktopBillingRouteLoadPolicyFor(invalid, false); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorContractInvalid {
		t.Fatalf("invalid non-catalog policy error = %v, want contract_invalid", err)
	}
}

func TestDesktopSubscriptionBalanceNeverCallsBYOKEndpoint(t *testing.T) {
	t.Setenv("KWU_CLOUD_FAKE", "1")
	t.Setenv("B2B_BALANCE_KEY", "profile-key")
	isolateDesktopUserDirs(t)
	serverCalls := 0
	lastAuthorization := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalls++
		lastAuthorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"is_available":true,"balance_infos":[{"currency":"USD","total_balance":"1.00"}]}`)
	}))
	defer server.Close()
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "B2B_BALANCE_KEY")
	cfg, err := config.LoadForRootMetadataWithActor(root, actor)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Providers[0].BalanceURL = server.URL
	if err := cfg.SaveTo(config.NewProfilePathResolver(actor).UserConfigPath()); err != nil {
		t.Fatal(err)
	}
	app, tab, _, _, _ := newDesktopBillingTestTab(t, root, actor, modelRef, modelbilling.BillingSubscriptionPool)
	app.setCurrentActorContext(actor)
	tab.actor = actor
	defer func() {
		if tab.Ctrl != nil {
			tab.Ctrl.Close()
		}
	}()

	subscriptionCtrl, ok := tab.Ctrl.(*control.Controller)
	if !ok {
		t.Fatalf("subscription controller = %T, want *control.Controller", tab.Ctrl)
	}
	if balance, err := subscriptionCtrl.Balance(context.Background()); err != nil || balance != nil {
		t.Fatalf("subscription controller balance = %#v, %v; want unavailable", balance, err)
	}
	if view := app.BalanceForTab(tab.ID); view.Available || view.Err != "" || serverCalls != 0 {
		t.Fatalf("subscription app balance = %+v calls=%d, want unavailable with zero HTTP", view, serverCalls)
	}

	if err := app.SetModelRouteForTab(tab.ID, modelRef, string(modelbilling.BillingNewAPIByok)); err != nil {
		t.Fatalf("select BYOK balance route: %v", err)
	}
	byokCtrl, ok := tab.Ctrl.(*control.Controller)
	if !ok {
		t.Fatalf("BYOK controller = %T, want *control.Controller", tab.Ctrl)
	}
	if balance, err := byokCtrl.Balance(context.Background()); err != nil || balance == nil {
		t.Fatalf("BYOK controller balance = %#v, %v; want fetched balance", balance, err)
	}
	if serverCalls != 1 || lastAuthorization != "Bearer profile-key" {
		t.Fatalf("BYOK balance HTTP calls/auth = %d/%q, want 1/Bearer profile-key", serverCalls, lastAuthorization)
	}
}

func activateDesktopSubscriptionTestSession(t *testing.T, app *App, actor userctx.ActorContext) (*desktopBillingWorkerManager, cloud.BindingScope) {
	t.Helper()
	manager := newDesktopBillingWorkerManager(func(_ *sql.DB, _ modelbilling.TaskScope) (desktopBillingWorkerRuntime, error) {
		return desktopBillingWorkerRuntime{run: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}}, nil
	})
	manager.waitTimeout = time.Second
	manager.start(context.Background())
	app.billingWorkerRuntime = manager
	binding := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "usr_fixture", BindingGeneration: 1}
	if err := manager.activate(actor, binding); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.close() })
	return manager, binding
}

func TestAuthenticatedBindingDefaultsAndPromotesSourceLessTabsToSubscription(t *testing.T) {
	t.Setenv(cloudFakeEnv, "1")
	isolateDesktopUserDirs(t)
	root := t.TempDir()
	actor := userctx.Default(root)
	actor.ProfileID = "authenticated-default-route"
	paths := config.NewProfilePathResolver(actor)
	if err := os.MkdirAll(filepath.Dir(paths.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.DefaultModel = "kwu/gpt-5.4"
	cfg.Desktop.DefaultBillingSource = string(modelbilling.BillingSubscriptionPool)
	cfg.Desktop.ProviderAccess = []string{"kwu"}
	if err := cfg.SaveTo(paths.UserConfigPath()); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	app.ctx = context.Background()
	app.readyHook = func() {}
	activateDesktopSubscriptionTestSession(t, app, actor)

	created := app.createTabEntryWithIDAndActor("project", root, "", "new-authenticated", actor)
	if created.billingSource != string(modelbilling.BillingSubscriptionPool) || created.model != "kwu/deepseek-v4-flash" {
		t.Fatalf("authenticated new tab route = %q/%q", created.billingSource, created.model)
	}

	legacy := app.createTabEntryWithIDAndActorForRoute("project", root, "", "legacy-empty", actor, "")
	legacy.Ready = true
	app.tabRuntimes[legacy.ID] = legacy
	app.activeTabID = legacy.ID
	if err := app.promoteSourceLessActiveTabToSubscription(actor); err != nil {
		t.Fatal(err)
	}
	if legacy.billingSource != string(modelbilling.BillingSubscriptionPool) || legacy.model != "kwu/deepseek-v4-flash" || legacy.Ctrl == nil {
		t.Fatalf("promoted legacy tab = source:%q model:%q ctrl:%T", legacy.billingSource, legacy.model, legacy.Ctrl)
	}
	legacy.Ctrl.Close()
}

func TestAnonymousOfficialDefaultStaysOnSubscriptionRoute(t *testing.T) {
	t.Setenv(cloudFakeEnv, "")
	isolateDesktopUserDirs(t)
	root := t.TempDir()
	actor := userctx.Default(root)
	actor.ProfileID = "anonymous-official-default"
	cfg := config.Default()
	cfg.DefaultModel = "kwu/gpt-5.4"
	cfg.Desktop.DefaultBillingSource = ""
	configPath := config.NewProfilePathResolver(actor).UserConfigPath()
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveTo(configPath); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	tab := app.createTabEntryWithIDAndActor("project", root, "", "anonymous", actor)
	if tab.billingSource != string(modelbilling.BillingSubscriptionPool) || tab.model != "kwu/deepseek-v4-flash" {
		t.Fatalf("anonymous official route = %q/%q", tab.billingSource, tab.model)
	}
}

func TestSubscriptionModelForAuthenticationRestoreOnlyReplacesBlankTabs(t *testing.T) {
	for _, test := range []struct {
		name        string
		current     string
		sessionPath string
		fallback    string
		want        string
	}{
		{
			name: "blank implicit tab", current: "kwu/gpt-5.4",
			fallback: "kwu/deepseek-v4-flash", want: "kwu/deepseek-v4-flash",
		},
		{
			name: "tab with history", current: "kwu/gpt-5.4", sessionPath: "history.jsonl",
			fallback: "kwu/deepseek-v4-flash", want: "kwu/gpt-5.4",
		},
		{
			name: "blank tab without resolved default", current: "kwu/gpt-5.4",
			want: "kwu/gpt-5.4",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := subscriptionModelForAuthenticationRestore(test.current, test.sessionPath, test.fallback); got != test.want {
				t.Fatalf("restored model = %q, want %q", got, test.want)
			}
		})
	}
}

func TestSubscriptionTabWithHistoryWaitsForAuthenticationWithoutStartupError(t *testing.T) {
	t.Setenv(cloudFakeEnv, "")
	t.Setenv(cloudBaseURLEnv, "http://127.0.0.1:1")
	isolateDesktopUserDirs(t)
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "WAITING_AUTH_KEY")

	app := NewApp()
	app.ctx = context.Background()
	app.readyHook = func() {}
	app.setCurrentActorContext(actor)
	app.billingWorkerRuntime.start(app.ctx)
	t.Cleanup(func() {
		app.cloudRuntime.close()
		_ = app.billingWorkerRuntime.close()
	})

	tab := app.createTabEntryWithIDAndActorForRoute(
		"project", root, "", "waiting-auth", actor, modelbilling.BillingSubscriptionPool,
	)
	tab.model = modelRef
	sessionDir := desktopSessionDirForActor(actor, root)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tab.SessionPath = filepath.Join(sessionDir, "waiting-auth.jsonl")
	writeHistoryTestSession(t, tab.SessionPath, "history while signed out")
	app.tabRuntimes[tab.ID] = tab
	app.activeTabID = tab.ID
	app.buildTabController(tab)

	if tab.Ctrl != nil || !tab.Ready || !tab.subscriptionAuthPending || tab.StartupErr != "" {
		t.Fatalf("signed-out tab = ctrl:%T ready:%v pending:%v startup:%q", tab.Ctrl, tab.Ready, tab.subscriptionAuthPending, tab.StartupErr)
	}
	if availability := tabSendAvailability(tab); availability.State != tabSendSubscriptionAuthRequired {
		t.Fatalf("signed-out availability = %+v", availability)
	}
	history := mustHistoryForTab(t, app, tab.ID)
	if len(history) != 1 || history[0].Content != "history while signed out" {
		t.Fatalf("signed-out disk history = %+v", history)
	}
	if err := app.SubmitToTab(tab.ID, "hello"); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorAuthRequired {
		t.Fatalf("signed-out submit error = %v, want auth_required", err)
	}
}

func TestSubscriptionTabTreatsExpiredSessionAsRecoverableAuthenticationWait(t *testing.T) {
	t.Setenv(cloudFakeEnv, "")
	isolateDesktopUserDirs(t)
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "EXPIRED_AUTH_KEY")

	app := NewApp()
	app.ctx = context.Background()
	app.readyHook = func() {}
	app.setCurrentActorContext(actor)
	manager, binding := activateDesktopSubscriptionTestSession(t, app, actor)
	app.subscriptionModels.replace(binding, cloud.MaaSKeyAccount{
		Status: cloud.MaaSKeyReady, KeyName: binding.CloudUserID, Key: "sk-expired-session",
	}, []string{"model_fixture_01"})
	if err := manager.quiesce(); err != nil {
		t.Fatal(err)
	}

	tab := app.createTabEntryWithIDAndActorForRoute(
		"project", root, "", "expired-auth", actor, modelbilling.BillingSubscriptionPool,
	)
	tab.model = modelRef
	app.tabRuntimes[tab.ID] = tab
	app.activeTabID = tab.ID
	app.buildTabController(tab)

	if tab.Ctrl != nil || !tab.Ready || !tab.subscriptionAuthPending || tab.StartupErr != "" {
		t.Fatalf("expired tab = ctrl:%T ready:%v pending:%v startup:%q", tab.Ctrl, tab.Ready, tab.subscriptionAuthPending, tab.StartupErr)
	}
	if err := app.SubmitToTab(tab.ID, "hello"); modelbilling.ErrorCodeOf(err) != modelbilling.ErrorAuthRequired {
		t.Fatalf("expired-session pending submit error = %v, want auth_required", err)
	}
}

func TestSubscriptionAuthenticationWaitClassificationUsesStructuredCodes(t *testing.T) {
	authRequired := &modelbilling.Error{Code: modelbilling.ErrorAuthRequired, Message: "localized message"}
	sessionExpired := &modelbilling.Error{Code: modelbilling.ErrorSessionExpired, Message: "another localized message"}
	contractFailure := &modelbilling.Error{Code: modelbilling.ErrorContractInvalid, Message: "session_expired text is not a code"}

	if !desktopSubscriptionAuthenticationPending(modelbilling.BillingSubscriptionPool, authRequired) {
		t.Fatal("auth_required was not classified as a recoverable subscription wait")
	}
	if !desktopSubscriptionAuthenticationPending(modelbilling.BillingSubscriptionPool, sessionExpired) {
		t.Fatal("session_expired was not classified as a recoverable subscription wait")
	}
	if desktopSubscriptionAuthenticationPending(modelbilling.BillingNewAPIByok, sessionExpired) {
		t.Fatal("BYOK session_expired was classified as a subscription authentication wait")
	}
	if desktopSubscriptionAuthenticationPending(modelbilling.BillingSubscriptionPool, contractFailure) {
		t.Fatal("non-authentication error text was classified as a recoverable wait")
	}
}

func TestSubscriptionBlankTabWaitsForAuthenticationWithEmptyHistory(t *testing.T) {
	t.Setenv(cloudFakeEnv, "")
	t.Setenv(cloudBaseURLEnv, "http://127.0.0.1:1")
	isolateDesktopUserDirs(t)
	root, actor, modelRef := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "WAITING_AUTH_BLANK_KEY")
	sessionDir := desktopSessionDirForActor(actor, root)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeHistoryTestSession(t, filepath.Join(sessionDir, "unrelated-old.jsonl"), "must not be borrowed")

	app := NewApp()
	app.ctx = context.Background()
	app.readyHook = func() {}
	app.setCurrentActorContext(actor)
	app.billingWorkerRuntime.start(app.ctx)
	t.Cleanup(func() {
		app.cloudRuntime.close()
		_ = app.billingWorkerRuntime.close()
	})

	tab := app.createTabEntryWithIDAndActorForRoute(
		"project", root, "", "waiting-auth-blank", actor, modelbilling.BillingSubscriptionPool,
	)
	tab.model = modelRef
	app.tabRuntimes[tab.ID] = tab
	app.activeTabID = tab.ID
	app.buildTabController(tab)

	if tab.Ctrl != nil || !tab.Ready || !tab.subscriptionAuthPending || tab.StartupErr != "" || tab.SessionPath != "" {
		t.Fatalf("signed-out blank tab = ctrl:%T ready:%v pending:%v startup:%q path:%q", tab.Ctrl, tab.Ready, tab.subscriptionAuthPending, tab.StartupErr, tab.SessionPath)
	}
	if availability := tabSendAvailability(tab); availability.State != tabSendSubscriptionAuthRequired {
		t.Fatalf("signed-out blank availability = %+v", availability)
	}
	history, err := app.HistoryForTab(tab.ID)
	if err != nil || history == nil || len(history) != 0 {
		t.Fatalf("signed-out blank history = %#v, %v; want [] nil", history, err)
	}
}

func TestSubscriptionTabStartupReplacesUnavailableStoredModel(t *testing.T) {
	t.Setenv(cloudFakeEnv, "")
	t.Setenv(cloudBaseURLEnv, "http://127.0.0.1:1")
	isolateDesktopUserDirs(t)
	root := t.TempDir()
	app := NewApp()
	actor := app.actorForWorkspace(root)
	app.ctx = context.Background()
	app.readyHook = func() {}
	scope := cloud.BindingScope{ProfileID: actor.ProfileID, CloudUserID: "cloud-user", BindingGeneration: 1}
	app.billingWorkerRuntime.bindingActor = actor
	app.billingWorkerRuntime.bindingScope = scope
	app.billingWorkerRuntime.hasBinding = true
	app.subscriptionModels.replace(scope, cloud.MaaSKeyAccount{
		Status: cloud.MaaSKeyReady, KeyName: scope.CloudUserID, Key: "sk-user",
	}, []string{"gpt-5.4"})
	app.setCurrentActorContext(actor)
	app.billingWorkerRuntime.start(app.ctx)
	t.Cleanup(func() {
		app.cloudRuntime.close()
		_ = app.billingWorkerRuntime.close()
	})

	tab := app.createTabEntryWithIDAndActorForRoute(
		"project", root, "", "stale-subscription-model", actor, modelbilling.BillingSubscriptionPool,
	)
	tab.model = "kwu/gpt-5.5"
	if err := addProject(root, ""); err != nil {
		t.Fatal(err)
	}
	sessionPath, err := createEmptySessionFile(desktopSessionDirForActor(actor, root), tab.model)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := agent.EnsureBranchMeta(sessionPath); err != nil {
		t.Fatal(err)
	}
	tab.SessionPath = sessionPath
	app.tabRuntimes[tab.ID] = tab
	app.activeTabID = tab.ID
	app.buildTabController(tab)

	if tab.model != "kwu/gpt-5.4" || tab.Ctrl != nil || !tab.Ready || !tab.subscriptionAuthPending || tab.StartupErr != "" ||
		tab.Scope != "project" || normalizeProjectRoot(tab.WorkspaceRoot) != normalizeProjectRoot(root) ||
		canonicalTabSessionPath(tab.SessionPath) != canonicalTabSessionPath(sessionPath) {
		t.Fatalf("stale subscription tab = model:%q ctrl:%T ready:%v pending:%v startup:%q route:%q/%q path:%q",
			tab.model, tab.Ctrl, tab.Ready, tab.subscriptionAuthPending, tab.StartupErr, tab.Scope, tab.WorkspaceRoot, tab.SessionPath)
	}
	if availability := tabSendAvailability(tab); availability.State != tabSendSubscriptionAuthRequired {
		t.Fatalf("stale subscription availability = %+v, want authentication recovery instead of startup failure", availability)
	}
	if history, err := app.HistoryForTab(tab.ID); err != nil || len(history) != 0 {
		t.Fatalf("stale subscription blank history = %+v, %v; want readable empty session", history, err)
	}
}

func TestCloudAuthenticationRestoresSubscriptionAuthPendingTab(t *testing.T) {
	for _, method := range []string{"login", "register"} {
		t.Run(method, func(t *testing.T) {
			t.Setenv(cloudFakeEnv, "1")
			isolateDesktopUserDirs(t)
			root, actor, modelRef := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "WAITING_AUTH_KEY")

			app := NewApp()
			app.ctx = context.Background()
			app.readyHook = func() {}
			app.setCurrentActorContext(actor)
			app.cloudRuntime.start(app.ctx)
			app.billingWorkerRuntime.start(app.ctx)
			t.Cleanup(func() {
				if tab := app.tabByID("waiting-auth"); tab != nil && tab.Ctrl != nil {
					tab.Ctrl.Close()
				}
				app.cloudRuntime.close()
				_ = app.billingWorkerRuntime.close()
			})

			tab := app.createTabEntryWithIDAndActorForRoute(
				"project", root, "", "waiting-auth", actor, modelbilling.BillingSubscriptionPool,
			)
			tab.model = modelRef
			sessionDir := desktopSessionDirForActor(actor, root)
			if err := os.MkdirAll(sessionDir, 0o755); err != nil {
				t.Fatal(err)
			}
			tab.SessionPath = filepath.Join(sessionDir, "auth-restore.jsonl")
			setTabSceneStatus(tab, legacyDesktopSceneStatus())
			writeHistoryTestSession(t, tab.SessionPath, "history before login")
			tab.Ready = true
			tab.subscriptionAuthPending = true
			app.tabRuntimes[tab.ID] = tab
			app.activeTabID = tab.ID

			switch method {
			case "login":
				_, err := app.CloudLogin(CloudLoginRequest{Phone: "13800000000", Password: "test-password-01"})
				if err != nil {
					t.Fatal(err)
				}
			case "register":
				if err := app.CloudSendRegisterSMS(CloudSendRegisterSMSRequest{Phone: "13800000000", CaptchaPassed: true}); err != nil {
					t.Fatal(err)
				}
				if _, err := app.CloudRegister(CloudRegisterRequest{Phone: "13800000000", Password: "registration-password", SMSCode: "123456"}); err != nil {
					t.Fatal(err)
				}
			}

			if tab.Ctrl == nil || !tab.Ready || tab.subscriptionAuthPending || tab.StartupErr != "" {
				t.Fatalf("restored tab = ctrl:%T ready:%v pending:%v startup:%q", tab.Ctrl, tab.Ready, tab.subscriptionAuthPending, tab.StartupErr)
			}
			if filepath.Clean(tab.SessionPath) != filepath.Clean(filepath.Join(sessionDir, "auth-restore.jsonl")) {
				t.Fatalf("authentication changed session path to %q", tab.SessionPath)
			}
			history := mustHistoryForTab(t, app, tab.ID)
			if len(history) != 1 || history[0].Content != "history before login" {
				t.Fatalf("authenticated history = %+v", history)
			}
		})
	}
}

func TestAuthenticatedCloudRefreshRestoresOnlyMatchingPendingSubscriptionTabs(t *testing.T) {
	for _, method := range []string{"bootstrap", "refresh"} {
		t.Run(method, func(t *testing.T) {
			t.Setenv(cloudFakeEnv, "1")
			isolateDesktopUserDirs(t)
			root, actor, modelRef := writeDesktopBillingConfig(t, "https://fixture.invalid/v1", "REFRESH_AUTH_KEY")

			app := NewApp()
			app.ctx = context.Background()
			app.readyHook = func() {}
			app.setCurrentActorContext(actor)
			app.cloudRuntime.start(app.ctx)
			app.billingWorkerRuntime.start(app.ctx)
			t.Cleanup(func() {
				for _, tab := range app.tabRuntimes {
					if tab != nil && tab.Ctrl != nil {
						tab.Ctrl.Close()
					}
				}
				app.cloudRuntime.close()
				_ = app.billingWorkerRuntime.close()
			})

			if _, err := app.CloudLogin(CloudLoginRequest{Phone: "13800000000", Password: "test-password-01"}); err != nil {
				t.Fatal(err)
			}

			matching := app.createTabEntryWithIDAndActorForRoute(
				"project", root, "", "matching-pending", actor, modelbilling.BillingSubscriptionPool,
			)
			matching.model = modelRef
			matching.Ready = true
			matching.subscriptionAuthPending = true
			matching.StartupErr = "old recoverable startup error"

			otherActor := actor
			otherActor.ProfileID += "-other"
			other := app.createTabEntryWithIDAndActorForRoute(
				"project", root, "", "other-profile-pending", otherActor, modelbilling.BillingSubscriptionPool,
			)
			other.model = modelRef
			other.Ready = true
			other.subscriptionAuthPending = true
			other.StartupErr = "other profile startup error"

			app.tabRuntimes = map[string]*WorkspaceTab{matching.ID: matching, other.ID: other}
			app.tabOrder = []string{matching.ID, other.ID}
			app.activeTabID = matching.ID

			var err error
			switch method {
			case "bootstrap":
				_, err = app.CloudBootstrap()
			case "refresh":
				_, err = app.CloudRefreshAssets()
			}
			if err != nil {
				t.Fatal(err)
			}
			if matching.Ctrl == nil || !matching.Ready || matching.subscriptionAuthPending || matching.StartupErr != "" {
				t.Fatalf("matching tab = ctrl:%T ready:%v pending:%v startup:%q", matching.Ctrl, matching.Ready, matching.subscriptionAuthPending, matching.StartupErr)
			}
			if other.Ctrl != nil || !other.subscriptionAuthPending || other.StartupErr != "other profile startup error" {
				t.Fatalf("other profile tab = ctrl:%T pending:%v startup:%q", other.Ctrl, other.subscriptionAuthPending, other.StartupErr)
			}
		})
	}
}

func newDesktopBillingTestTab(t *testing.T, root string, actor userctx.ActorContext, modelRef string, source modelbilling.BillingSource) (*App, *WorkspaceTab, *[]*desktopBillingRuntime, *[]*desktopCountingGateway, *[]*desktopFreezeSpy) {
	t.Helper()
	app := NewApp()
	if source == modelbilling.BillingSubscriptionPool {
		activateDesktopSubscriptionTestSession(t, app, actor)
	}
	app.ctx = context.Background()
	app.readyHook = func() {}
	runtimes := []*desktopBillingRuntime{}
	gateways := []*desktopCountingGateway{}
	freezes := []*desktopFreezeSpy{}
	app.desktopBillingRuntimeObserver = func(runtime *desktopBillingRuntime) {
		gateway := &desktopCountingGateway{base: runtime.dispatcher.Gateway}
		runtime.dispatcher.Gateway = gateway
		freeze := &desktopFreezeSpy{base: runtime.initializer.inner.Freeze}
		runtime.initializer.inner.Freeze = freeze
		runtimes = append(runtimes, runtime)
		gateways = append(gateways, gateway)
		freezes = append(freezes, freeze)
	}
	options := boot.Options{Model: modelRef, WorkspaceRoot: root, Actor: actor, Sink: event.Discard}
	ctrl, err := app.buildDesktopController(context.Background(), options, actor, root, modelRef, string(source))
	if err != nil {
		t.Fatalf("initial desktop B2B controller: %v", err)
	}
	tab := &WorkspaceTab{ID: "b2b-tab", WorkspaceRoot: root, Ready: true, model: modelRef, billingSource: string(source), Ctrl: ctrl}
	app.tabRuntimes = map[string]*WorkspaceTab{tab.ID: tab}
	app.tabOrder = []string{tab.ID}
	app.activeTabID = tab.ID
	return app, tab, &runtimes, &gateways, &freezes
}

func writeDesktopBillingConfig(t *testing.T, baseURL, keyEnv string) (string, userctx.ActorContext, string) {
	t.Helper()
	if _, err := ensureDesktopProfileInitialized(); err != nil {
		t.Fatalf("initialize desktop profile: %v", err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "kwu.toml"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	actor := userctx.Default(root)
	paths := config.NewProfilePathResolver(actor)
	if err := os.MkdirAll(filepath.Dir(paths.UserConfigPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	modelRef := "fixture/model_fixture_01"
	cfg := config.Default()
	cfg.DefaultModel = modelRef
	cfg.Providers = []config.ProviderEntry{{Name: "fixture", Kind: "openai", BaseURL: baseURL, Model: "model_fixture_01", APIKeyEnv: keyEnv}}
	if err := cfg.SaveTo(paths.UserConfigPath()); err != nil {
		t.Fatal(err)
	}
	if value := os.Getenv(keyEnv); value != "" {
		if err := os.MkdirAll(filepath.Dir(paths.UserCredentialsPath()), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(paths.UserCredentialsPath(), []byte(keyEnv+"="+value+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root, actor, modelRef
}

func desktopRuntimeBootOptions(runtime *desktopBillingRuntime, actor userctx.ActorContext, root, modelRef string) boot.Options {
	return boot.Options{
		Model:         modelRef,
		WorkspaceRoot: root,
		Actor:         actor,
		Sink:          event.Discard,
		ModelBillingOptions: boot.ModelBillingOptions{
			RequireTaskContext:     true,
			BillingRouteSelection:  modelbilling.BillingRouteSelection{ModelRef: modelRef, BillingSource: modelbilling.BillingSubscriptionPool},
			TaskContextInitializer: runtime.initializer,
			ModelBillingTaskStore:  runtime.tasks,
			MainStreamDispatcher:   runtime.dispatcher,
			ModelBillingCleanup:    runtime.Close,
		},
	}
}

func controlTaskContextRequest(actor userctx.ActorContext, selection modelbilling.BillingRouteSelection) control.TaskContextInitRequest {
	return control.TaskContextInitRequest{Actor: actor, BillingRouteSelection: selection}
}

func receiveDesktopBillingScope(t *testing.T, stage string, ch <-chan modelbilling.TaskScope) modelbilling.TaskScope {
	t.Helper()
	select {
	case scope := <-ch:
		return scope
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", stage)
		return modelbilling.TaskScope{}
	}
}

func receiveDesktopBillingSignal(t *testing.T, stage string, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", stage)
	}
}

func assertDesktopBillingClock(t *testing.T, name string, clock func() time.Time, want []time.Time) {
	t.Helper()
	if clock == nil {
		t.Fatalf("%s clock is nil", name)
	}
	for i, expected := range want {
		if got := clock(); !got.Equal(expected) {
			t.Fatalf("%s clock call %d = %s, want %s", name, i+1, got, expected)
		}
	}
}

func waitDesktopBillingTaskState(
	t *testing.T,
	tasks *modelbilling.SQLiteTaskStore,
	scope modelbilling.TaskScope,
	taskID string,
	want modelbilling.TaskState,
) modelbilling.ModelTask {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for {
		task, err := tasks.Get(context.Background(), scope, taskID)
		if err == nil && task.State == want {
			return task
		}
		if time.Now().After(deadline) {
			t.Fatalf("task %q state = %s, error = %v; want %s", taskID, task.State, err, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

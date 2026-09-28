//go:build liveintegration

package cloud

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"kwu/internal/userctx"
)

// TestLiveCreditFreezeProbe is an explicit, operator-only A2 probe. Run it
// only after the desktop app is closed: it reads the current durable binding
// and session, then reads /me, creates one expiring freeze, replays the same
// idempotency key, and reads that freeze. It never calls Maas, usage, final,
// settlement, or a release API.
func TestLiveCreditFreezeProbe(t *testing.T) {
	baseURL := requiredLiveValue(t, "REASONIX_CLOUD_LIVE_BASE_URL")
	modelID := requiredLiveValue(t, "REASONIX_CLOUD_LIVE_MODEL_ID")
	taskType := requiredLiveValue(t, "REASONIX_CLOUD_LIVE_TASK_TYPE")
	estimatedCost := requiredLiveValue(t, "REASONIX_CLOUD_LIVE_ESTIMATED_COST")
	profileID := requiredLiveValue(t, "REASONIX_CLOUD_LIVE_PROFILE_ID")
	actorID := strings.TrimSpace(os.Getenv("REASONIX_CLOUD_LIVE_ACTOR_ID"))
	if actorID == "" {
		actorID = "local"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	actor := userctx.ActorContext{ActorID: actorID, ProfileID: profileID}
	paths := DefaultProfilePathProvider
	bindings := NewDurableBindingRepository(paths, testEnvironmentIdentity())
	binding, err := bindings.ResolveBinding(ctx, actor)
	if err != nil {
		t.Fatal("read durable binding:", err)
	}
	scope, err := binding.Scope()
	if err != nil {
		t.Fatal("current profile is not cloud-bound:", err)
	}
	store := NewDurableCloudSessionStore(paths, testEnvironmentIdentity())
	session, ok, err := store.Load(ctx, scope)
	if err != nil || !ok {
		t.Fatal("no durable cloud session for current binding")
	}
	transport, err := NewHTTPSessionClient(baseURL, http.DefaultClient, DeviceIdentity{DeviceID: session.DeviceID, ClientVersion: requiredLiveValue(t, "REASONIX_CLOUD_LIVE_CLIENT_VERSION")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	executor, err := NewAuthenticatedRequestExecutor(store, transport, nil)
	if err != nil {
		t.Fatal(err)
	}
	credit, err := NewHTTPCreditClient(executor, transport)
	if err != nil {
		t.Fatal(err)
	}
	var before CurrentUserSnapshot
	if err := executor.Do(ctx, scope, func(ctx context.Context, token string) error {
		var callErr error
		before, callErr = transport.CurrentUser(ctx, token)
		return callErr
	}); err != nil {
		t.Fatal("GET /me:", err)
	}
	key := liveProbeID("idem-a2")
	request := FreezeRequest{Scope: scope, TaskID: liveProbeID("task-a2"), ModelID: modelID, ClientTaskType: taskType, ClientEstimatedCost: estimatedCost, IdempotencyKey: key}
	created, err := credit.CreateFreeze(ctx, request)
	if err != nil {
		t.Fatal("POST credit-freezes:", err)
	}
	replayed, err := credit.CreateFreeze(ctx, request)
	if err != nil {
		t.Fatal("replay credit-freezes:", err)
	}
	if created.Freeze.FreezeID != replayed.Freeze.FreezeID {
		t.Fatal("idempotency replay returned another freeze")
	}
	fetched, err := credit.GetFreeze(ctx, scope, created.Freeze.FreezeID)
	if err != nil {
		t.Fatal("GET credit-freeze:", err)
	}
	t.Logf("A2 live probe ok: freeze=%s status=%s before_version=%d after_version=%d before_total=%s after_total=%s request_ids are intentionally not logged", redactLiveID(fetched.Freeze.FreezeID), fetched.Freeze.Status, before.Balance.Version, fetched.Balance.Version, before.Balance.TotalBalance, fetched.Balance.TotalBalance)
}

func requiredLiveValue(t *testing.T, name string) string {
	t.Helper()
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		t.Skipf("set %s to opt in", name)
	}
	return value
}
func liveProbeID(prefix string) string {
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic(err)
	}
	return prefix + "-" + hex.EncodeToString(raw[:])
}
func redactLiveID(value string) string {
	if len(value) <= 8 {
		return "***"
	}
	return value[:4] + "…" + value[len(value)-4:]
}

package main

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	cloud "kwu/internal/cloud"
	"kwu/internal/userctx"
)

func TestCloudAssetsChangedViewContainsOnlyWakeupCategories(t *testing.T) {
	encoded, err := json.Marshal(CloudAssetsChangedView{BalanceChanged: true, SubscriptionChanged: false})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(encoded), `{"balanceChanged":true,"subscriptionChanged":false}`; got != want {
		t.Fatalf("payload JSON = %s, want %s", got, want)
	}
}

func TestCloudAssetsNotifierMergesDirtyCategoriesWhileDeliveryIsPending(t *testing.T) {
	notifier := newCloudAssetsNotifier()
	notifier.start()
	t.Cleanup(notifier.close)

	var mu sync.Mutex
	var delivered []CloudAssetsChangedView
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	delivery := make(chan struct{}, 2)
	notifier.bind(func(payload CloudAssetsChangedView) {
		mu.Lock()
		delivered = append(delivered, payload)
		index := len(delivered)
		mu.Unlock()
		if index == 1 {
			close(firstStarted)
			<-releaseFirst
		}
		delivery <- struct{}{}
	})

	notifier.notify(cloud.EventAssetChanges{BalanceChanged: true})
	waitCloudNotifierSignal(t, "first delivery start", firstStarted)

	notifyDone := make(chan struct{})
	go func() {
		notifier.notify(cloud.EventAssetChanges{SubscriptionChanged: true})
		notifier.notify(cloud.EventAssetChanges{BalanceChanged: true})
		close(notifyDone)
	}()
	waitCloudNotifierSignal(t, "non-blocking dirty merge", notifyDone)
	close(releaseFirst)
	waitCloudNotifierSignal(t, "first delivery", delivery)
	waitCloudNotifierSignal(t, "merged delivery", delivery)

	mu.Lock()
	defer mu.Unlock()
	want := []CloudAssetsChangedView{
		{BalanceChanged: true},
		{BalanceChanged: true, SubscriptionChanged: true},
	}
	if len(delivered) != len(want) || delivered[0] != want[0] || delivered[1] != want[1] {
		t.Fatalf("delivered = %#v, want %#v", delivered, want)
	}
}

func TestCloudAssetsNotifierSurvivesSenderPanicAndStopsAfterClose(t *testing.T) {
	notifier := newCloudAssetsNotifier()
	notifier.start()

	panicked := make(chan struct{})
	notifier.bind(func(CloudAssetsChangedView) {
		close(panicked)
		panic("Wails sender failed")
	})
	notifier.notify(cloud.EventAssetChanges{BalanceChanged: true})
	waitCloudNotifierSignal(t, "panicking sender", panicked)

	delivered := make(chan CloudAssetsChangedView, 1)
	notifier.bind(func(payload CloudAssetsChangedView) { delivered <- payload })
	notifier.notify(cloud.EventAssetChanges{SubscriptionChanged: true})
	select {
	case payload := <-delivered:
		if payload != (CloudAssetsChangedView{SubscriptionChanged: true}) {
			t.Fatalf("delivery after panic = %#v", payload)
		}
	case <-time.After(time.Second):
		t.Fatal("notifier did not continue after sender panic")
	}

	notifier.close()
	notifier.bind(func(payload CloudAssetsChangedView) { delivered <- payload })
	notifier.notify(cloud.EventAssetChanges{BalanceChanged: true})
	select {
	case payload := <-delivered:
		t.Fatalf("delivery after close = %#v", payload)
	case <-time.After(25 * time.Millisecond):
	}
}

func TestCloudRuntimeAssetNotifierLifecycleIsSafeWhenUnavailable(t *testing.T) {
	runtime := &cloudRuntime{}
	runtime.bindAssetsChangedSender(func(CloudAssetsChangedView) {
		t.Fatal("unavailable runtime emitted an asset notification")
	})
	runtime.start(context.Background())
	runtime.close()
	runtime.close()
}

func TestFixtureCloudRuntimeWiresAppliedEventsToAssetNotifier(t *testing.T) {
	t.Setenv(cloudFakeEnv, "1")
	runtime, err := newCloudRuntime(desktopCloudTestProfilePathProvider(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	runtime.settlements.bind(&desktopSettlementReconcilerSpy{})
	runtime.start(context.Background())
	t.Cleanup(runtime.close)

	delivered := make(chan CloudAssetsChangedView, 4)
	runtime.bindAssetsChangedSender(func(payload CloudAssetsChangedView) { delivered <- payload })
	app := NewApp()
	app.cloudRuntime = runtime
	app.setCurrentActorContext(userctx.ActorContext{ActorID: "local", ProfileID: "default"})
	if _, err := app.CloudLogin(CloudLoginRequest{Phone: "13800000000", Password: "test-password-01"}); err != nil {
		t.Fatal(err)
	}
	select {
	case payload := <-delivered:
		if !payload.BalanceChanged {
			t.Fatalf("fixture event payload = %#v, want balance wakeup", payload)
		}
	case <-time.After(time.Second):
		t.Fatal("fixture event sync did not emit an asset wakeup")
	}
}

func waitCloudNotifierSignal(t *testing.T, name string, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}

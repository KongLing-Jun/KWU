package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"kwu/internal/boot"
	"kwu/internal/cloud"
	"kwu/internal/config"
	"kwu/internal/expert"
)

type expertCatalogTestClient struct {
	list func(context.Context) (cloud.SystemExpertAssets, error)
}

func (c *expertCatalogTestClient) ListSystem(ctx context.Context) (cloud.SystemExpertAssets, error) {
	return c.list(ctx)
}

func (*expertCatalogTestClient) CreateUserExpert(context.Context, cloud.BindingScope, cloud.CreateUserExpertAssetInput) (cloud.UserExpertAsset, error) {
	return cloud.UserExpertAsset{}, errors.New("unexpected user expert POST")
}

func (*expertCatalogTestClient) CreateUserExpertGroup(context.Context, cloud.BindingScope, cloud.CreateUserExpertGroupAssetInput) (cloud.UserExpertGroupAsset, error) {
	return cloud.UserExpertGroupAsset{}, errors.New("unexpected user expert group POST")
}

func TestExpertCatalogCoordinatorStartsOnceMergesInflightAndAllowsManualRerun(t *testing.T) {
	t.Setenv("CLOUVIA_STATE_HOME", t.TempDir())
	var calls atomic.Int32
	releases := []chan struct{}{make(chan struct{}), make(chan struct{}), make(chan struct{})}
	client := &expertCatalogTestClient{list: func(ctx context.Context) (cloud.SystemExpertAssets, error) {
		call := int(calls.Add(1))
		select {
		case <-ctx.Done():
			return cloud.SystemExpertAssets{}, ctx.Err()
		case <-releases[call-1]:
			return testSystemExpertAssets("exp_remote"), nil
		}
	}}
	app := newExpertCatalogTestApp(t, client)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app.expertCatalog.start(ctx)
	app.expertCatalog.start(ctx)
	waitForExpertCatalogCalls(t, &calls, 1)

	var firstResults sync.WaitGroup
	firstResults.Add(2)
	for _, refresh := range []func(context.Context) (ExpertCatalogState, error){
		app.expertCatalog.initialize,
		app.expertCatalog.manualRefresh,
	} {
		refresh := refresh
		go func() {
			defer firstResults.Done()
			if _, err := refresh(ctx); err != nil {
				t.Errorf("joined refresh error = %v", err)
			}
		}()
	}
	// The in-memory refresh completes immediately after the client returns, so
	// let both goroutines attach to the still-blocked shared call before release.
	time.Sleep(25 * time.Millisecond)
	if got := calls.Load(); got != 1 {
		t.Fatalf("in-flight calls = %d, want 1", got)
	}
	close(releases[0])
	firstResults.Wait()

	var manualResults sync.WaitGroup
	manualResults.Add(2)
	go func() {
		defer manualResults.Done()
		if _, err := app.expertCatalog.manualRefresh(ctx); err != nil {
			t.Errorf("manual refresh error = %v", err)
		}
	}()
	waitForExpertCatalogCalls(t, &calls, 2)
	go func() {
		defer manualResults.Done()
		if _, err := app.expertCatalog.manualRefresh(ctx); err != nil {
			t.Errorf("joined manual refresh error = %v", err)
		}
	}()
	time.Sleep(25 * time.Millisecond)
	close(releases[1])
	manualResults.Wait()
	if got := calls.Load(); got != 2 {
		t.Fatalf("concurrent manual calls = %d, want 2 total", got)
	}

	thirdDone := make(chan error, 1)
	go func() {
		_, err := app.expertCatalog.manualRefresh(ctx)
		thirdDone <- err
	}()
	waitForExpertCatalogCalls(t, &calls, 3)
	close(releases[2])
	if err := <-thirdDone; err != nil {
		t.Fatalf("manual rerun error = %v", err)
	}
	app.expertCatalog.close()
}

func TestExpertCatalogInitializeJoinsManualRetryAfterAutomaticFailure(t *testing.T) {
	t.Setenv("CLOUVIA_STATE_HOME", t.TempDir())
	var calls atomic.Int32
	manualStarted := make(chan struct{})
	releaseManual := make(chan struct{})
	client := &expertCatalogTestClient{list: func(ctx context.Context) (cloud.SystemExpertAssets, error) {
		switch calls.Add(1) {
		case 1:
			return cloud.SystemExpertAssets{}, errors.New("automatic refresh failed")
		case 2:
			close(manualStarted)
			select {
			case <-ctx.Done():
				return cloud.SystemExpertAssets{}, ctx.Err()
			case <-releaseManual:
				return testSystemExpertAssets("exp_retry"), nil
			}
		default:
			return cloud.SystemExpertAssets{}, errors.New("unexpected extra refresh")
		}
	}}
	app := newExpertCatalogTestApp(t, client)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.expertCatalog.start(ctx)
	if _, err := app.expertCatalog.initialize(ctx); err == nil {
		t.Fatal("automatic initialization error = nil")
	}

	manualDone := make(chan error, 1)
	go func() {
		_, err := app.expertCatalog.manualRefresh(ctx)
		manualDone <- err
	}()
	<-manualStarted

	type initializeResult struct {
		state ExpertCatalogState
		err   error
	}
	initializeDone := make(chan initializeResult, 1)
	go func() {
		state, err := app.expertCatalog.initialize(ctx)
		initializeDone <- initializeResult{state: state, err: err}
	}()
	select {
	case result := <-initializeDone:
		t.Fatalf("initialize returned old result while manual retry was in flight: state=%+v err=%v", result.state, result.err)
	case <-time.After(25 * time.Millisecond):
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("refresh calls while retry is in flight = %d, want 2 total", got)
	}

	close(releaseManual)
	if err := <-manualDone; err != nil {
		t.Fatalf("manual retry error = %v", err)
	}
	result := <-initializeDone
	if result.err != nil {
		t.Fatalf("joined initialize error = %v", result.err)
	}
	if !result.state.HasOfficial || !hasExpertInfo(result.state.Experts, "exp_retry") ||
		result.state.LastRefresh == nil || !result.state.LastRefresh.Succeeded {
		t.Fatalf("joined initialize state = %+v", result.state)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("refresh calls after joined retry = %d, want 2 total", got)
	}
	app.expertCatalog.close()
}

func TestExpertCatalogCoordinatorCanceledWaiterDoesNotCancelSharedRefresh(t *testing.T) {
	t.Setenv("CLOUVIA_STATE_HOME", t.TempDir())
	started := make(chan struct{})
	release := make(chan struct{})
	client := &expertCatalogTestClient{list: func(ctx context.Context) (cloud.SystemExpertAssets, error) {
		close(started)
		select {
		case <-ctx.Done():
			return cloud.SystemExpertAssets{}, ctx.Err()
		case <-release:
			return testSystemExpertAssets("exp_remote"), nil
		}
	}}
	app := newExpertCatalogTestApp(t, client)
	lifecycleCtx, cancelLifecycle := context.WithCancel(context.Background())
	defer cancelLifecycle()
	app.expertCatalog.start(lifecycleCtx)
	<-started

	waitCtx, cancelWait := context.WithCancel(context.Background())
	cancelWait()
	if _, err := app.expertCatalog.manualRefresh(waitCtx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled waiter error = %v, want context canceled", err)
	}
	close(release)
	state, err := app.expertCatalog.initialize(context.Background())
	if err != nil {
		t.Fatalf("shared refresh error = %v", err)
	}
	if !state.HasOfficial || state.LastRefresh == nil || !state.LastRefresh.Succeeded {
		t.Fatalf("state after shared refresh = %+v", state)
	}
	app.expertCatalog.close()
}

func TestExpertCatalogRefreshFailurePreservesCachedOfficial(t *testing.T) {
	t.Setenv("CLOUVIA_STATE_HOME", t.TempDir())
	client := &expertCatalogTestClient{list: func(context.Context) (cloud.SystemExpertAssets, error) {
		return cloud.SystemExpertAssets{}, errors.New("remote unavailable")
	}}
	app := newExpertCatalogTestApp(t, client)
	t.Cleanup(app.expertCatalog.close)
	app.replaceCloudExpertDefinitions(testOfficialDefinitions("exp_cached"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.expertCatalog.start(ctx)
	state, err := app.expertCatalog.initialize(context.Background())
	if err == nil {
		t.Fatal("InitializeExpertCatalog() error = nil")
	}
	if !state.HasOfficial || !hasExpertInfo(state.Experts, "exp_cached") {
		t.Fatalf("cached state after failure = %+v", state)
	}
	if state.LastRefresh == nil || state.LastRefresh.Succeeded || state.LastRefresh.Error == "" {
		t.Fatalf("last refresh after failure = %+v", state.LastRefresh)
	}
	app.expertCatalog.close()
}

func TestExpertCatalogRefreshAllowsSuccessfulEmptyCatalog(t *testing.T) {
	t.Setenv("CLOUVIA_STATE_HOME", t.TempDir())
	client := &expertCatalogTestClient{list: func(context.Context) (cloud.SystemExpertAssets, error) {
		return cloud.SystemExpertAssets{Experts: []cloud.SystemExpertAsset{}, ExpertGroups: []cloud.SystemExpertGroupAsset{}}, nil
	}}
	app := newExpertCatalogTestApp(t, client)
	state, err := app.expertCatalog.manualRefresh(context.Background())
	if err != nil {
		t.Fatalf("manualRefresh() error = %v", err)
	}
	if state.HasOfficial || len(state.Experts) != 0 || state.LastRefresh == nil || !state.LastRefresh.Succeeded {
		t.Fatalf("empty catalog state = %+v", state)
	}
	app.expertCatalog.close()
}

func TestExpertCatalogCloseCancelsAndDrainsRefresh(t *testing.T) {
	t.Setenv("CLOUVIA_STATE_HOME", t.TempDir())
	started := make(chan struct{})
	finished := make(chan struct{})
	client := &expertCatalogTestClient{list: func(ctx context.Context) (cloud.SystemExpertAssets, error) {
		close(started)
		<-ctx.Done()
		close(finished)
		return cloud.SystemExpertAssets{}, ctx.Err()
	}}
	app := newExpertCatalogTestApp(t, client)
	app.expertCatalog.start(context.Background())
	<-started
	app.expertCatalog.close()
	select {
	case <-finished:
	default:
		t.Fatal("close returned before the refresh observed lifecycle cancellation")
	}
}

func TestBuiltinAndFixtureCatalogRefreshUseLocalDefinitions(t *testing.T) {
	for _, test := range []struct {
		name    string
		runtime *cloudRuntime
		mode    string
	}{
		{name: "builtin", runtime: &cloudRuntime{}, mode: expertCatalogModeBuiltin},
		{name: "fixture", runtime: &cloudRuntime{expertCatalogMode: expertCatalogModeFixture}, mode: expertCatalogModeFixture},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("CLOUVIA_STATE_HOME", t.TempDir())
			app := &App{cloudRuntime: test.runtime, tabRuntimes: map[string]*WorkspaceTab{}}
			app.expertCatalog = newExpertCatalogCoordinator(app)
			t.Cleanup(app.closeExpertStore)
			state, err := app.expertCatalog.manualRefresh(context.Background())
			if err != nil {
				t.Fatalf("manualRefresh() error = %v", err)
			}
			if state.Mode != test.mode || !state.HasOfficial || len(state.Experts) == 0 {
				t.Fatalf("local catalog state = %+v", state)
			}
			if app.cloudRuntime.expertAssets != nil {
				t.Fatal("local catalog refresh unexpectedly has an HTTP expert client")
			}
			app.expertCatalog.close()
		})
	}
}

func TestListExpertsReadsCloudExpertTeamsLiveWithLatestAvatars(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("CLOUVIA_STATE_HOME", stateHome)
	var calls atomic.Int32
	client := &expertCatalogTestClient{list: func(context.Context) (cloud.SystemExpertAssets, error) {
		call := calls.Add(1)
		suffix := fmt.Sprintf("%d", call)
		return cloud.SystemExpertAssets{
			Experts: []cloud.SystemExpertAsset{{
				ExpertID: "exp_live", Name: "Live Expert", Prompt: "Help.", Category: "System",
				AvatarURL: "https://assets.example.com/member-" + suffix + ".png", Type: "system",
			}},
			ExpertGroups: []cloud.SystemExpertGroupAsset{{
				ExpertGroupID: "exg_live", Name: "Live Team", Category: "System",
				ExpertIDs: []string{"exp_live"},
				AvatarURL: "https://assets.example.com/team-" + suffix + ".png", Type: "system",
			}},
		}, nil
	}}
	app := newExpertCatalogTestApp(t, client)
	app.replaceCloudExpertDefinitions([]expert.Definition{{
		ID: "exg_stale", Source: expert.SourceOfficial, Mode: expert.ModeTeam,
		Name: "Stale Team", Role: "System", Description: "Cached only.",
		AvatarKey: "https://cache.example.com/stale.png",
		Prompts:   []string{"One.", "Two.", "Three."},
	}})

	first, err := app.ListExperts()
	if err != nil {
		t.Fatalf("first ListExperts() error = %v", err)
	}
	second, err := app.ListExperts()
	if err != nil {
		t.Fatalf("second ListExperts() error = %v", err)
	}
	firstTeam := expertInfoByID(first, "exg_live")
	secondTeam := expertInfoByID(second, "exg_live")
	if calls.Load() != 2 || firstTeam == nil || secondTeam == nil {
		t.Fatalf("live lists = first:%+v second:%+v HTTP calls:%d", first, second, calls.Load())
	}
	if hasExpertInfo(first, "exg_stale") || hasExpertInfo(second, "exg_stale") {
		t.Fatalf("stale cached team leaked into live catalog: first:%+v second:%+v", first, second)
	}
	if firstTeam.AvatarURL != "https://assets.example.com/team-1.png" ||
		len(firstTeam.Members) != 1 ||
		firstTeam.Members[0].AvatarURL != "https://assets.example.com/member-1.png" {
		t.Fatalf("first live team avatars = %+v", firstTeam)
	}
	if secondTeam.AvatarURL != "https://assets.example.com/team-2.png" ||
		len(secondTeam.Members) != 1 ||
		secondTeam.Members[0].AvatarURL != "https://assets.example.com/member-2.png" {
		t.Fatalf("second live team avatars = %+v", secondTeam)
	}
	assertNoCloudExpertSQLite(t)
}

func TestListExpertsSerializesCloudCatalogRefreshes(t *testing.T) {
	t.Setenv("CLOUVIA_STATE_HOME", t.TempDir())
	firstStarted := make(chan struct{})
	secondStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	var calls atomic.Int32
	client := &expertCatalogTestClient{list: func(context.Context) (cloud.SystemExpertAssets, error) {
		switch calls.Add(1) {
		case 1:
			close(firstStarted)
			<-releaseFirst
		case 2:
			close(secondStarted)
		}
		return testSystemExpertAssets("exp_serial"), nil
	}}
	app := newExpertCatalogTestApp(t, client)
	defer app.expertCatalog.close()

	results := make(chan error, 2)
	go func() {
		_, err := app.ListExperts()
		results <- err
	}()
	<-firstStarted
	go func() {
		_, err := app.ListExperts()
		results <- err
	}()

	select {
	case <-secondStarted:
		close(releaseFirst)
		t.Fatal("second cloud catalog request overlapped the first")
	case <-time.After(100 * time.Millisecond):
	}
	close(releaseFirst)
	for range 2 {
		select {
		case err := <-results:
			if err != nil {
				t.Fatalf("ListExperts() error = %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("ListExperts() did not finish")
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("cloud catalog calls = %d, want 2 serialized calls", calls.Load())
	}
}

func TestResetExpertsPreservesOfficialCacheWithoutHTTP(t *testing.T) {
	t.Setenv("CLOUVIA_STATE_HOME", t.TempDir())
	var calls atomic.Int32
	client := &expertCatalogTestClient{list: func(context.Context) (cloud.SystemExpertAssets, error) {
		calls.Add(1)
		return cloud.SystemExpertAssets{}, errors.New("unexpected HTTP")
	}}
	app := newExpertCatalogTestApp(t, client)
	t.Cleanup(app.expertCatalog.close)
	app.replaceCloudExpertDefinitions(testOfficialDefinitions("exp_cached"))
	userPath := config.UserExpertsPath()
	if err := expert.SaveUserExperts(userPath, []expert.UserExpert{{
		ID: "custom-json", Name: "JSON User", Mode: string(expert.ModeExpert),
		Description: "Local JSON.", SystemPrompt: "Help.", Prompts: []string{"One.", "Two.", "Three."}, Enabled: true,
	}}); err != nil {
		t.Fatalf("seed JSON user: %v", err)
	}
	if err := expert.SaveExpertLibraryState(userPath, expert.ExpertLibraryState{
		FavoriteIDs: []string{"exp_cached", "custom-json"},
		RecentIDs:   []string{"custom-json"},
	}); err != nil {
		t.Fatalf("seed library state: %v", err)
	}

	result, err := app.ResetExperts()
	if err != nil {
		t.Fatalf("ResetExperts() error = %v", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("ResetExperts() HTTP calls = %d, want 0", calls.Load())
	}
	if !hasExpertInfo(result.Experts, "exp_cached") {
		t.Fatalf("reset catalog = %+v", result.Experts)
	}
	if len(result.UserExperts) != 0 || len(result.LibraryState.FavoriteIDs) != 0 || len(result.LibraryState.RecentIDs) != 0 {
		t.Fatalf("reset JSON state = experts:%+v library:%+v", result.UserExperts, result.LibraryState)
	}
	assertNoCloudExpertSQLite(t)
}

func TestDesktopCloudExpertCatalogOptionsUseMemorySnapshot(t *testing.T) {
	t.Setenv("CLOUVIA_STATE_HOME", t.TempDir())
	app := newExpertCatalogTestApp(t, &expertCatalogTestClient{list: func(context.Context) (cloud.SystemExpertAssets, error) {
		return cloud.SystemExpertAssets{}, errors.New("unexpected HTTP")
	}})
	app.replaceCloudExpertDefinitions(testOfficialDefinitions("exp_memory"))
	options := boot.Options{UserExpertPath: filepath.Join(t.TempDir(), "user-experts.json")}

	app.applyDesktopExpertCatalogOptions(&options)

	if options.ExpertCatalogLoader == nil || !options.DisableBuiltinExperts {
		t.Fatalf("cloud expert boot options = %+v, want memory loader with builtins disabled", options)
	}
	catalog, err := options.ExpertCatalogLoader()
	if err != nil {
		t.Fatalf("ExpertCatalogLoader() error = %v", err)
	}
	if _, err := catalog.Resolve("exp_memory"); err != nil {
		t.Fatalf("resolve memory expert: %v", err)
	}
	app.replaceCloudExpertDefinitions(testOfficialDefinitions("exp_updated"))
	catalog, err = options.ExpertCatalogLoader()
	if err != nil {
		t.Fatalf("updated ExpertCatalogLoader() error = %v", err)
	}
	if _, err := catalog.Resolve("exp_updated"); err != nil {
		t.Fatalf("resolve updated memory expert: %v", err)
	}
	if _, err := catalog.Resolve("exp_memory"); err == nil {
		t.Fatal("memory loader retained replaced expert")
	}
	assertNoCloudExpertSQLite(t)
}

func newExpertCatalogTestApp(t *testing.T, client cloud.ExpertAssetsClient) *App {
	t.Helper()
	app := &App{
		cloudRuntime: &cloudRuntime{expertAssets: client, expertCatalogMode: expertCatalogModeRemote},
		tabRuntimes:  map[string]*WorkspaceTab{},
	}
	app.expertCatalog = newExpertCatalogCoordinator(app)
	t.Cleanup(app.closeExpertStore)
	return app
}

func testSystemExpertAssets(id string) cloud.SystemExpertAssets {
	return cloud.SystemExpertAssets{Experts: []cloud.SystemExpertAsset{{
		ExpertID: id, Name: "Remote Expert", Prompt: "Help.", Category: "System", Type: "system",
	}}}
}

func testOfficialDefinitions(id string) []expert.Definition {
	return []expert.Definition{{
		ID: id, Source: expert.SourceOfficial, Mode: expert.ModeExpert, Name: "Cached Expert", Role: "System",
		Description: "Cached.", SystemPrompt: "Help.", Prompts: []string{"One.", "Two.", "Three."},
	}}
}

func assertNoCloudExpertSQLite(t *testing.T) {
	t.Helper()
	path := filepath.Join(config.MemoryUserDir(), "experts.sqlite")
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cloud expert SQLite state exists at %q: %v", path, err)
	}
}

func hasExpertInfo(infos []ExpertInfo, id string) bool {
	for _, info := range infos {
		if info.ID == id {
			return true
		}
	}
	return false
}

func expertInfoByID(infos []ExpertInfo, id string) *ExpertInfo {
	for index := range infos {
		if infos[index].ID == id {
			return &infos[index]
		}
	}
	return nil
}

func waitForExpertCatalogCalls(t *testing.T, calls *atomic.Int32, want int32) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		if calls.Load() >= want {
			return
		}
		select {
		case <-deadline.C:
			t.Fatalf("expert catalog calls = %d, want at least %d", calls.Load(), want)
		case <-ticker.C:
		}
	}
}

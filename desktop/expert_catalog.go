package main

import (
	"context"
	"sync"
	"time"

	"kwu/internal/expert"
)

const (
	expertCatalogModeRemote  = "remote"
	expertCatalogModeFixture = "fixture"
	expertCatalogModeBuiltin = "builtin"
)

type ExpertCatalogRefreshResult struct {
	Mode        string `json:"mode"`
	Succeeded   bool   `json:"succeeded"`
	Error       string `json:"error,omitempty"`
	CompletedAt int64  `json:"completedAt"`
}

type ExpertCatalogState struct {
	Mode        string                      `json:"mode"`
	Experts     []ExpertInfo                `json:"experts"`
	HasOfficial bool                        `json:"hasOfficial"`
	Refreshing  bool                        `json:"refreshing"`
	LastRefresh *ExpertCatalogRefreshResult `json:"lastRefresh,omitempty"`
}

type expertCatalogRefreshCall struct {
	done      chan struct{}
	automatic bool
	state     ExpertCatalogState
	err       error
}

type expertCatalogCoordinator struct {
	app *App

	mu                 sync.Mutex
	lifecycleCtx       context.Context
	lifecycleCancel    context.CancelFunc
	closed             bool
	startupOnce        sync.Once
	automaticCompleted bool
	inFlight           *expertCatalogRefreshCall
	lastRefresh        *ExpertCatalogRefreshResult
	lastErr            error
	wg                 sync.WaitGroup
}

func newExpertCatalogCoordinator(app *App) *expertCatalogCoordinator {
	return &expertCatalogCoordinator{app: app}
}

func (c *expertCatalogCoordinator) start(ctx context.Context) {
	if c == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	if c.lifecycleCtx == nil {
		c.lifecycleCtx, c.lifecycleCancel = context.WithCancel(ctx)
	}
	c.startupOnce.Do(func() {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			_, _ = c.initialize(ctx)
		}()
	})
	c.mu.Unlock()
}

func (c *expertCatalogCoordinator) close() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.closed = true
	cancel := c.lifecycleCancel
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	c.wg.Wait()
	if c.app != nil {
		c.app.closeExpertStore()
	}
}

func (c *expertCatalogCoordinator) initialize(waitCtx context.Context) (ExpertCatalogState, error) {
	return c.refresh(waitCtx, true)
}

func (c *expertCatalogCoordinator) manualRefresh(waitCtx context.Context) (ExpertCatalogState, error) {
	return c.refresh(waitCtx, false)
}

func (c *expertCatalogCoordinator) refresh(waitCtx context.Context, automatic bool) (ExpertCatalogState, error) {
	if c == nil || c.app == nil {
		return ExpertCatalogState{}, context.Canceled
	}
	if waitCtx == nil {
		waitCtx = context.Background()
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return ExpertCatalogState{}, context.Canceled
	}
	call := c.inFlight
	if call != nil {
		if automatic {
			call.automatic = true
		}
		c.mu.Unlock()
		return waitForExpertCatalogCall(waitCtx, call)
	}
	if automatic && c.automaticCompleted {
		lastErr := c.lastErr
		c.mu.Unlock()
		state, err := c.state()
		if lastErr != nil {
			return state, lastErr
		}
		return state, err
	}
	lifecycleCtx := c.lifecycleCtx
	if lifecycleCtx == nil {
		lifecycleCtx, c.lifecycleCancel = context.WithCancel(context.Background())
		c.lifecycleCtx = lifecycleCtx
	}
	call = &expertCatalogRefreshCall{
		done:      make(chan struct{}),
		automatic: automatic,
	}
	c.inFlight = call
	c.wg.Add(1)
	go c.runRefresh(lifecycleCtx, call)
	c.mu.Unlock()
	return waitForExpertCatalogCall(waitCtx, call)
}

func (c *expertCatalogCoordinator) runRefresh(ctx context.Context, call *expertCatalogRefreshCall) {
	defer c.wg.Done()
	err := c.app.refreshExpertCatalog(ctx)
	result := &ExpertCatalogRefreshResult{
		Mode:        c.app.expertCatalogMode(),
		Succeeded:   err == nil,
		CompletedAt: time.Now().UnixMilli(),
	}
	if err != nil {
		result.Error = err.Error()
	}

	c.mu.Lock()
	c.lastRefresh = result
	c.lastErr = err
	c.inFlight = nil
	if call.automatic {
		c.automaticCompleted = true
	}
	c.mu.Unlock()

	state, stateErr := c.state()
	if err == nil {
		err = stateErr
	}
	call.state = state
	call.err = err
	close(call.done)
}

func waitForExpertCatalogCall(ctx context.Context, call *expertCatalogRefreshCall) (ExpertCatalogState, error) {
	select {
	case <-ctx.Done():
		return ExpertCatalogState{}, ctx.Err()
	case <-call.done:
		return call.state, call.err
	}
}

func (c *expertCatalogCoordinator) state() (ExpertCatalogState, error) {
	definitions, err := c.app.listExpertDefinitions()
	if err != nil {
		return ExpertCatalogState{}, err
	}
	state := ExpertCatalogState{
		Mode:    c.app.expertCatalogMode(),
		Experts: expertInfos(definitions),
	}
	for _, definition := range definitions {
		if definition.Source == expert.SourceOfficial {
			state.HasOfficial = true
			break
		}
	}
	c.mu.Lock()
	state.Refreshing = c.inFlight != nil
	if c.lastRefresh != nil {
		last := *c.lastRefresh
		state.LastRefresh = &last
	}
	c.mu.Unlock()
	return state, nil
}

func (a *App) expertCatalogMode() string {
	if a != nil && a.cloudRuntime != nil {
		switch a.cloudRuntime.expertCatalogMode {
		case expertCatalogModeRemote, expertCatalogModeFixture, expertCatalogModeBuiltin:
			return a.cloudRuntime.expertCatalogMode
		}
	}
	if a.usesCloudExpertAssets() {
		return expertCatalogModeRemote
	}
	if a != nil && a.cloudRuntime != nil && a.cloudRuntime.authority != nil {
		return expertCatalogModeFixture
	}
	return expertCatalogModeBuiltin
}

func (a *App) listExpertDefinitions() ([]expert.Definition, error) {
	if a.usesCloudExpertAssets() {
		return a.cloudExpertDefinitionsSnapshot(), nil
	}
	store, err := a.expertStore()
	if err != nil {
		return nil, err
	}
	return store.List(a.bootContext())
}

func (a *App) replaceCloudExpertDefinitions(definitions []expert.Definition) {
	if a == nil {
		return
	}
	a.expertCatalogCacheMu.Lock()
	a.cloudExpertDefinitions = cloneCloudExpertDefinitions(definitions)
	a.expertCatalogCacheMu.Unlock()
}

func (a *App) cloudExpertDefinitionsSnapshot() []expert.Definition {
	if a == nil {
		return nil
	}
	a.expertCatalogCacheMu.RLock()
	definitions := cloneCloudExpertDefinitions(a.cloudExpertDefinitions)
	a.expertCatalogCacheMu.RUnlock()
	return definitions
}

func cloneCloudExpertDefinitions(definitions []expert.Definition) []expert.Definition {
	cloned := make([]expert.Definition, len(definitions))
	for index, definition := range definitions {
		cloned[index] = definition
		cloned[index].Categories = append([]string(nil), definition.Categories...)
		cloned[index].Tags = append([]string(nil), definition.Tags...)
		cloned[index].Members = append([]expert.Member(nil), definition.Members...)
		cloned[index].Prompts = append([]string(nil), definition.Prompts...)
	}
	return cloned
}

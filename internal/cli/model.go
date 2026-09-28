package cli

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"

	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/i18n"
	"kwu/internal/userctx"
)

var errNoEnabledProviders = errors.New("setup selected no providers")

type currentController struct {
	mu   sync.RWMutex
	ctrl control.SessionAPI
}

func newCurrentController(ctrl control.SessionAPI) *currentController {
	return &currentController{ctrl: ctrl}
}

func (c *currentController) Get() control.SessionAPI {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ctrl
}

func (c *currentController) Set(ctrl control.SessionAPI) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ctrl = ctrl
}

func (c *currentController) Snapshot() error {
	ctrl := c.Get()
	if ctrl == nil {
		return nil
	}
	return ctrl.Snapshot()
}

func setCurrentController(c *currentController, ctrl control.SessionAPI) *currentController {
	if c == nil {
		return newCurrentController(ctrl)
	}
	c.Set(ctrl)
	return c
}

func defaultModelForSelectedProviders(enabled []config.ProviderEntry, prevDefault string) (string, error) {
	if len(enabled) == 0 {
		return "", errNoEnabledProviders
	}
	for _, p := range enabled {
		if p.Name == prevDefault {
			return prevDefault, nil
		}
	}
	return enabled[0].Name, nil
}

func (m *chatTUI) cliConfigContext() (string, userctx.ActorContext) {
	root := "."
	actor := defaultCLIActorContext()
	if m != nil && m.ctrl != nil {
		actor = m.ctrl.ActorContext()
		if wr := strings.TrimSpace(m.ctrl.WorkspaceRoot()); wr != "" {
			root = wr
		}
	} else if wr := strings.TrimSpace(actor.WorkspaceRoot); wr != "" {
		root = wr
	}
	if strings.TrimSpace(actor.WorkspaceRoot) == "" {
		actor.WorkspaceRoot = root
	}
	return root, actor.Normalize()
}

func (m *chatTUI) loadConfig() (*config.Config, error) {
	root, actor := m.cliConfigContext()
	return config.LoadForRootWithActor(root, actor)
}

func (m *chatTUI) userConfigForEdit() (*config.Config, config.ProfilePathResolver) {
	root, actor := m.cliConfigContext()
	if strings.TrimSpace(actor.WorkspaceRoot) == "" {
		actor.WorkspaceRoot = root
	}
	paths := config.NewProfilePathResolver(actor)
	return config.LoadForEditWithPaths(paths), paths
}

func serveModelDefault(model, resumePath string, cfg *config.Config, actor userctx.ActorContext) string {
	model = modelForResumePath(model, resumePath, cfg)
	if model != "" {
		return model
	}
	if uc := config.NewProfilePathResolver(actor).UserConfigPath(); uc != "" {
		if userCfg := config.LoadForEdit(uc); userCfg != nil && userCfg.DefaultModel != "" {
			return userCfg.DefaultModel
		}
	}
	return ""
}

// runModelSubcommand handles "/model": with no argument it lists the configured
// (provider, model) refs and marks the active one; "/model <ref>" switches the
// session to that model in place, carrying the conversation across. The actual
// controller build runs asynchronously so it cannot block the TUI event loop.
func (m *chatTUI) runModelSubcommand(input string) {
	args := tokenizeArgs(input) // args[0] == "/model"
	if len(args) < 2 {
		m.showModels()
		return
	}
	ref := args[1]
	if m.buildController == nil {
		m.notice(i18n.M.ModelSwitchUnavailable)
		return
	}
	if m.ctrl.Running() {
		m.notice(i18n.M.ModelSwitchBusy)
		return
	}
	if ref == m.modelRef {
		m.notice(fmt.Sprintf(i18n.M.ModelAlreadyOnFmt, ref))
		return
	}
	// Persist the user's choice to the user config.toml so the next
	// session starts on the same model instead of falling back to the global
	// default. Mirrors the pattern used by /theme (persistTheme), /effort, and
	// /language.
	m.persistModel(ref)
	carried := m.ctrl.History()
	prevPath := m.ctrl.SessionPath()
	if err := m.ctrl.Snapshot(); err != nil {
		m.notice("model: snapshot failed: " + err.Error())
	}
	m.notice(fmt.Sprintf(i18n.M.ModelSwitchingFmt, ref))

	// Capture old controller for cleanup after the async build succeeds.
	oldCtrl := m.ctrl
	build := m.buildController
	actor := oldCtrl.ActorContext()

	// Fire the build off the event loop; the result arrives as a tea.Cmd.
	// Both the build AND the old-controller close run in the goroutine so
	// neither blocks the bubbletea event loop. The old controller's Close
	// kills plugin subprocesses (incl. CodeGraph), which can disrupt the
	// terminal's cancelReader if called synchronously inside Update — so it
	// must happen here, before we hand the new controller back.
	m.modelSwitchPending = true
	m.pendingModelSwitch = func() tea.Msg {
		c, err := build(ref, nil, carried, prevPath, actor)
		if err != nil {
			return modelSwitchMsg{ref: ref, err: err}
		}
		// Do NOT close the old controller here. Controller.Close() kills plugin
		// subprocesses — an operation that can corrupt bubbletea's terminal raw
		// mode when executed from a goroutine. Instead, pass the old
		// controller back in the message so the Update handler can defer
		// its cleanup as a tea.Cmd that runs after the next render.
		return modelSwitchMsg{
			ref:      ref,
			ctrl:     c,
			oldCtrl:  oldCtrl,
			label:    c.Label(),
			commands: c.Commands(),
			skills:   c.Skills(),
			host:     c.Host(),
		}
	}
}

// showModels lists the configured provider/model refs, marking the active one.
func (m *chatTUI) showModels() {
	cfg, err := m.loadConfig()
	if err != nil {
		m.notice("model: " + err.Error())
		return
	}
	var refs []string
	for i := range cfg.Providers {
		p := &cfg.Providers[i]
		if !p.Configured() {
			continue
		}
		for _, model := range p.ChatModelList() {
			refs = append(refs, p.Name+"/"+model)
		}
	}
	m.commitLine(renderModels(m.width, refs, m.modelRef))
}

// persistModel writes ref (a "provider/model" string) to default_model in the
// user config.toml so the next CLI launch starts on the same
// model. The in-memory switch is always allowed to proceed regardless of the
// outcome here, but every step (rejected by validation, save failed, or
// persisted successfully) reports back to the TUI notice channel so the user
// can see whether their /model choice will survive a restart. Run before
// Snapshot/ModelSwitchingFmt so the persistence outcome shows up first in
// the notice area.
func (m *chatTUI) persistModel(ref string) {
	edit, paths := m.userConfigForEdit()
	path := paths.UserConfigPath()
	if path == "" {
		return
	}
	if err := edit.SetDefaultModel(ref); err != nil {
		m.notice(fmt.Sprintf("model: persist refused: %v (ref=%s)", err, ref))
		return
	}
	if err := edit.SaveToScope(path, config.RenderScopeUser); err != nil {
		m.notice(fmt.Sprintf("model: persist save failed: %v (ref=%s, path=%s)", err, ref, path))
		return
	}
	m.notice(fmt.Sprintf("model: persisted (ref=%s, path=%s)", ref, path))
}

// modelRefs returns the configured provider/model refs for slash completion.
func modelRefs() []string {
	cfg, err := loadCLIConfig()
	if err != nil {
		return nil
	}
	var out []string
	for i := range cfg.Providers {
		p := &cfg.Providers[i]
		if !p.Configured() {
			continue
		}
		for _, model := range p.ChatModelList() {
			out = append(out, p.Name+"/"+model)
		}
	}
	return out
}

// providerNames returns the names of configured providers for slash completion.
func providerNames() []string {
	cfg, err := loadCLIConfig()
	if err != nil {
		return nil
	}
	var out []string
	for i := range cfg.Providers {
		p := &cfg.Providers[i]
		if !p.Configured() {
			continue
		}
		out = append(out, p.Name)
	}
	return out
}

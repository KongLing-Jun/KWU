package main

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"kwu/internal/agent"
	"kwu/internal/control"
	"kwu/internal/project"
	"kwu/internal/provider"
)

type NewCloudProjectSessionInput struct {
	ProjectID       string `json:"projectId"`
	TabID           string `json:"tabId"`
	WorkspacePath   string `json:"workspacePath,omitempty"`
	ExecutionTarget string `json:"executionTarget,omitempty"`
}

type CloudProjectSessionView struct {
	TabID              string `json:"tabId"`
	SessionPath        string `json:"sessionPath"`
	ProjectID          string `json:"projectId"`
	InstructionVersion uint64 `json:"instructionVersion"`
	ContentHash        string `json:"contentHash"`
	Reused             bool   `json:"reused"`
}

// newCloudProjectSessionBeforeCommit is a deterministic race seam for the
// focused Desktop contract tests. Production leaves it as a no-op.
var newCloudProjectSessionBeforeCommit = func() {}

func projectSessionTargetLock(app *App, tabID string) *sync.Mutex {
	lock, _ := app.projectSessionLocks.LoadOrStore(tabID, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

// NewCloudProjectSession binds an already-prepared blank tab to one verified
// CloudProject instruction snapshot. The explicit tab ID prevents a click from
// racing a later active-tab change and writing the project context elsewhere.
func (a *App) NewCloudProjectSession(input NewCloudProjectSessionInput) (CloudProjectSessionView, error) {
	projectID := strings.TrimSpace(input.ProjectID)
	tabID := strings.TrimSpace(input.TabID)
	workspacePath := strings.TrimSpace(input.WorkspacePath)
	executionTarget := strings.TrimSpace(input.ExecutionTarget)
	if projectID == "" || projectID != input.ProjectID || tabID == "" || tabID != input.TabID || workspacePath != input.WorkspacePath {
		return CloudProjectSessionView{}, fmt.Errorf("projectId and tabId are required")
	}
	if executionTarget != "" && executionTarget != "local" && executionTarget != "cloud" {
		return CloudProjectSessionView{}, fmt.Errorf("executionTarget must be local or cloud")
	}
	if executionTarget == "local" && workspacePath == "" {
		return CloudProjectSessionView{}, fmt.Errorf("local execution requires workspacePath")
	}
	if executionTarget == "cloud" && workspacePath != "" {
		return CloudProjectSessionView{}, fmt.Errorf("cloud execution cannot use workspacePath")
	}
	cloudExecution := executionTarget == "cloud"
	lock := projectSessionTargetLock(a, tabID)
	lock.Lock()
	defer lock.Unlock()

	services, err := a.projectServicesForOperation()
	if err != nil {
		return CloudProjectSessionView{}, projectWailsError(err)
	}
	ctx, cancel := services.operationContext(a.bootContext())
	defer cancel()
	projectResult, err := services.Gateway.GetProject(ctx, projectID)
	if err != nil || projectResult.Project.Status != project.ProjectActive {
		return CloudProjectSessionView{}, fmt.Errorf("active cloud project is unavailable")
	}
	a.mu.RLock()
	tab := a.tabRuntimes[tabID]
	var ctrl control.SessionAPI
	if tab != nil {
		ctrl = tab.Ctrl
	}
	a.mu.RUnlock()
	if tab == nil || ctrl == nil || !tab.Ready || tab.ReadOnly {
		return CloudProjectSessionView{}, fmt.Errorf("target workspace is unavailable")
	}
	var binding project.LocalWorkspaceBinding
	var found bool
	if workspacePath != "" {
		managedWorkspacePath, managedErr := ensureCloudProjectWorkspace(ctx, services, projectID)
		if managedErr != nil {
			return CloudProjectSessionView{}, fmt.Errorf("dedicated project workspace is unavailable")
		}
		if !sameDesktopPath(workspacePath, managedWorkspacePath) {
			return CloudProjectSessionView{}, fmt.Errorf("selected workspace is not the project's dedicated folder")
		}
		if tab.Scope != "project" || !sameDesktopPath(tab.WorkspaceRoot, workspacePath) {
			return CloudProjectSessionView{}, fmt.Errorf("selected workspace does not match the prepared task")
		}
		binding, err = services.Bindings.Bind(ctx, services.Scope, projectID, workspacePath)
		found = err == nil
	} else if !cloudExecution {
		binding, found, err = services.Bindings.Get(ctx, services.Scope, projectID)
	}
	if err != nil {
		return CloudProjectSessionView{}, fmt.Errorf("cloud project binding is unavailable")
	}
	bindingAvailable := found && binding.Status == project.LocalWorkspaceAvailable
	expectedWorkspaceRoot := globalWorkspaceRoot()
	if bindingAvailable {
		expectedWorkspaceRoot = binding.CanonicalPath
	}
	if !projectSessionTabMatchesBinding(tab, bindingAvailable, expectedWorkspaceRoot) {
		return CloudProjectSessionView{}, fmt.Errorf("target workspace does not match the project binding")
	}
	if controllerHasActiveRuntimeWork(ctrl) {
		return CloudProjectSessionView{}, fmt.Errorf("target workspace is busy")
	}
	path := strings.TrimSpace(ctrl.SessionPath())
	if path == "" {
		return CloudProjectSessionView{}, fmt.Errorf("project session path is unavailable")
	}

	previousMeta, metaExists, err := agent.LoadBranchMeta(path)
	if err != nil {
		return CloudProjectSessionView{}, err
	}
	var reusablePin *agent.ProjectContextPin
	if metaExists && previousMeta.ProjectContext != nil {
		pin := *previousMeta.ProjectContext
		if !projectContextPinMatchesScope(pin, services.Scope) || pin.ProjectID != projectID {
			return CloudProjectSessionView{}, fmt.Errorf("target workspace is pinned to another cloud project")
		}
		if err := control.ValidatePinnedProjectContext(ctrl.History(), pin.ProjectID, pin.InstructionVersion, pin.ContentHash); err != nil {
			return CloudProjectSessionView{}, fmt.Errorf("pinned project session is invalid: %w", err)
		}
		reusablePin = &pin
	} else if messagesHaveConversationContent(ctrl.History()) {
		return CloudProjectSessionView{}, fmt.Errorf("target workspace is not a prepared blank session")
	}

	// Check instruction access locally before preparing the commit. Fetch and
	// validate only the snapshot actually pinned below; an earlier discarded
	// read adds a network round trip without strengthening that snapshot.
	// Reuse keeps its immutable pin and needs no instruction access.
	if reusablePin == nil {
		instructionServices, err := a.projectServicesForInstructionOperation()
		if err != nil || instructionServices.Scope.Key != services.Scope.Key {
			return CloudProjectSessionView{}, fmt.Errorf("cloud project instruction scope changed before session creation")
		}
	}

	// Re-read the mutable Cloud dependencies and binding immediately before the
	// local commit. This closes the account-switch/rebind window created by the
	// authoritative network reads above.
	commitServices, err := a.projectServicesForOperation()
	if err == nil && reusablePin == nil {
		commitServices, err = a.projectServicesForInstructionOperation()
	}
	if err != nil || commitServices.Scope.Key != services.Scope.Key {
		return CloudProjectSessionView{}, fmt.Errorf("cloud project scope changed before session creation")
	}
	commitCtx, commitCancel := commitServices.operationContext(a.bootContext())
	defer commitCancel()
	newCloudProjectSessionBeforeCommit()

	var instructionVersion uint64
	var contentHash string
	var contextMessage provider.Message
	var present bool
	if reusablePin == nil {
		commitInstruction, instructionErr := commitServices.Gateway.GetProjectInstruction(commitCtx, projectID)
		if instructionErr != nil {
			return CloudProjectSessionView{}, projectProtocolWailsError(instructionErr)
		}
		contextMessage, present, instructionErr = control.BuildProjectContextMessage(control.ProjectContextInput{
			ProjectID: projectID, HumanDescription: commitInstruction.Instruction.HumanDescription,
			AgentInstruction: commitInstruction.Instruction.AgentInstruction, Version: commitInstruction.Instruction.Version,
			ContentHash: commitInstruction.Instruction.ContentHash,
		})
		if instructionErr != nil {
			return CloudProjectSessionView{}, projectProtocolWailsError(instructionErr)
		}
		instructionVersion = commitInstruction.Instruction.Version
		contentHash = commitInstruction.Instruction.ContentHash
	}
	var commitBinding project.LocalWorkspaceBinding
	var commitBindingAvailable bool
	if !cloudExecution {
		var commitBindingFound bool
		commitBinding, commitBindingFound, err = commitServices.Bindings.Get(commitCtx, commitServices.Scope, projectID)
		commitBindingAvailable = commitBindingFound && commitBinding.Status == project.LocalWorkspaceAvailable
	}
	if err != nil || commitBindingAvailable != bindingAvailable ||
		(commitBindingAvailable && normalizeProjectRoot(commitBinding.CanonicalPath) != normalizeProjectRoot(expectedWorkspaceRoot)) {
		return CloudProjectSessionView{}, fmt.Errorf("cloud project binding changed before session creation")
	}
	commitProject, err := commitServices.Gateway.GetProject(commitCtx, projectID)
	if err != nil || commitProject.Project.Status != project.ProjectActive {
		return CloudProjectSessionView{}, fmt.Errorf("active cloud project changed before session creation")
	}

	// Holding the read lock prevents tab replacement while metadata and the
	// transcript are committed. Controller identity is checked under that lock.
	a.mu.RLock()
	current := a.tabRuntimes[tabID]
	if current != tab || current == nil || current.Ctrl != ctrl || current.ReadOnly || !current.Ready ||
		!projectSessionTabMatchesBinding(current, bindingAvailable, expectedWorkspaceRoot) || commitCtx.Err() != nil {
		a.mu.RUnlock()
		return CloudProjectSessionView{}, fmt.Errorf("target workspace changed before session creation")
	}
	if reusablePin != nil {
		if err := validatePinnedProjectSessionForReuse(ctrl, path, *reusablePin); err != nil {
			a.mu.RUnlock()
			return CloudProjectSessionView{}, fmt.Errorf("pinned project session changed before reuse: %w", err)
		}
		a.mu.RUnlock()
		return CloudProjectSessionView{TabID: tabID, SessionPath: path, ProjectID: projectID,
			InstructionVersion: reusablePin.InstructionVersion, ContentHash: reusablePin.ContentHash, Reused: true}, nil
	}
	meta := previousMeta
	if !metaExists {
		meta, err = agent.EnsureBranchMeta(path)
		if err != nil {
			a.mu.RUnlock()
			return CloudProjectSessionView{}, err
		}
	}
	meta.ProjectContext = &agent.ProjectContextPin{
		Environment: agent.ProjectEnvironmentIdentity{
			EnvironmentID: commitServices.Scope.Environment.EnvironmentID,
			CloudBaseURL:  commitServices.Scope.Environment.CloudBaseURL,
		},
		BindingScope: agent.ProjectBindingScope{
			ProfileID: commitServices.Scope.Binding.ProfileID, CloudUserID: commitServices.Scope.Binding.CloudUserID,
			BindingGeneration: commitServices.Scope.Binding.BindingGeneration,
		},
		ProjectID: projectID, InstructionVersion: instructionVersion,
		ContentHash: contentHash,
	}
	err = prepareProjectContextTransaction(ctrl, control.ProjectContextTransaction{
		ExpectedPath: path,
		Message:      contextMessage,
		Present:      present,
		CommitMeta:   func() error { return agent.SaveBranchMeta(path, meta) },
		RollbackMeta: func() error { return rollbackProjectSessionMeta(path, previousMeta, metaExists) },
	})
	if err != nil {
		a.mu.RUnlock()
		return CloudProjectSessionView{}, err
	}
	a.mu.RUnlock()

	a.persistTabSessionPath(tab, path)
	a.refreshTabSessionTurns(tab.ID)
	a.invalidatePromptHistoryCache()
	// Publish the authoritative project read already required by session
	// creation. The workspace tree must not need to infer access from the new
	// local transcript, and directory refresh markers still prevent this detail
	// from reviving a project removed by an authoritative owner/member list.
	_ = commitServices.Cache.StoreDetail(commitCtx, commitServices.Scope, commitProject.Project)
	a.emitProjectTreeChanged()
	return CloudProjectSessionView{TabID: tabID, SessionPath: path, ProjectID: projectID,
		InstructionVersion: instructionVersion, ContentHash: contentHash}, nil
}

func projectSessionTabMatchesBinding(tab *WorkspaceTab, bindingAvailable bool, expectedWorkspaceRoot string) bool {
	if tab == nil {
		return false
	}
	expectedScope := "global"
	if bindingAvailable {
		expectedScope = "project"
	}
	return tab.Scope == expectedScope && sameDesktopPath(tab.WorkspaceRoot, expectedWorkspaceRoot)
}

func prepareProjectContextTransaction(ctrl control.SessionAPI, transaction control.ProjectContextTransaction) error {
	transactional, ok := ctrl.(interface {
		PrepareProjectContextTransaction(control.ProjectContextTransaction) error
	})
	if !ok {
		return fmt.Errorf("target controller does not support atomic project session preparation")
	}
	return transactional.PrepareProjectContextTransaction(transaction)
}

func validatePinnedProjectSessionForReuse(ctrl control.SessionAPI, path string, pin agent.ProjectContextPin) error {
	validator, ok := ctrl.(interface {
		ValidatePinnedProjectContextSession(string, string, uint64, string) error
	})
	if !ok {
		return fmt.Errorf("target controller does not support atomic project session reuse")
	}
	return validator.ValidatePinnedProjectContextSession(path, pin.ProjectID, pin.InstructionVersion, pin.ContentHash)
}

func rollbackProjectSessionMeta(path string, previous agent.BranchMeta, existed bool) error {
	if existed {
		return agent.SaveBranchMetaPreserveUpdated(path, previous)
	}
	err := os.Remove(agent.BranchMetaPath(path))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

package boot

import (
	"context"
	"io"
	"net/http"
	"os"
	"time"

	"kwu/internal/agent"
	"kwu/internal/agentmode"
	"kwu/internal/command"
	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/expert"
	"kwu/internal/instruction"
	"kwu/internal/jobs"
	"kwu/internal/lsp"
	"kwu/internal/memory"
	"kwu/internal/netclient"
	"kwu/internal/permission"
	"kwu/internal/plugin"
	"kwu/internal/provider"
	"kwu/internal/sandbox"
	"kwu/internal/skill"
	"kwu/internal/temporal"
	"kwu/internal/tool"
	"kwu/internal/tool/builtin"
	"kwu/internal/userctx"
)

type buildAssembly struct {
	ctx    context.Context
	opts   Options
	stderr io.Writer

	root                         string
	actor                        userctx.ActorContext
	credentialPaths              config.ProfilePathResolver
	credentialOwner              string
	migrated                     *config.MigrationResult
	migrationErr                 error
	memoryCompilerConfigMigrated bool
	memoryCompilerConfigErr      error
	cfg                          *config.Config
	agentMode                    agentmode.ResolvedMode
	modelName                    string
	entry                        *config.ProviderEntry
	modelRef                     string
	tokenEconomy                 bool
	keepPolicy                   agent.KeepPolicy

	sink          event.Sink
	jobs          *jobs.Manager
	sessionDir    string
	archiveDir    string
	proxySpec     netclient.ProxySpec
	balanceClient *http.Client
	execProvider  provider.Provider

	basePrompt      string
	memory          *memory.Set
	projectChecks   []instruction.VerifyCheck
	skillStore      *skill.Store
	allSkillStore   *skill.Store
	skills          []skill.Skill
	allSkills       []skill.Skill
	userExpertPath  string
	expertStorePath string
	expertCatalog   *expert.Catalog
	systemPrompt    string
	expertSelection expert.Selection

	registry    *tool.Registry
	bashSpec    sandbox.Spec
	shell       sandbox.Shell
	searchSpec  builtin.SearchSpec
	bashTimeout time.Duration

	pluginHost                *plugin.Host
	projectTrusted            bool
	onDemandMCPSpecs          map[string]plugin.Spec
	onDemandMCPNames          []string
	planModeTrustedMCPServers map[string]bool
	cleanup                   func()
	lspManager                *lsp.Manager
	lspToolsAdded             bool
	maxSteps                  int
	subagentStore             *agent.SubagentStore
	policy                    permission.Policy
	headlessGate              *permission.Gate
	taskModel                 string
	taskEffort                string
	taskTool                  *agent.TaskTool
	taskToolAdded             bool
	readOnlyTaskToolAdded     bool
	commands                  []command.Command
	installSourceAdded        bool
	readOnlySkillToolsAdded   bool
	skillToolsAdded           bool

	executor                *agent.Agent
	runner                  agent.Runner
	label                   string
	classifier              *control.ProviderAutoPlanClassifier
	taskTreePlannerProvider provider.Provider
	temporal                *temporal.Service
}

func newBuildAssembly(ctx context.Context, opts Options) *buildAssembly {
	stderr := opts.Stderr
	if stderr == nil {
		stderr = io.Writer(nil)
	}
	return &buildAssembly{ctx: ctx, opts: opts, stderr: stderr}
}

func defaultStderr(stderr io.Writer) io.Writer {
	if stderr == nil {
		return os.Stderr
	}
	return stderr
}

func lspToolsUnavailable(manager *lsp.Manager, added bool) bool {
	return manager == nil || added
}

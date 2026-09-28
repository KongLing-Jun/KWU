package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"kwu/desktop/internal/diagnostics"
	"kwu/internal/agent"
	"kwu/internal/automation"
	"kwu/internal/boot"
	"kwu/internal/brand"
	"kwu/internal/cloud"
	"kwu/internal/config"
	"kwu/internal/control"
	"kwu/internal/event"
	"kwu/internal/eventwire"
	"kwu/internal/evidence"
	"kwu/internal/expert"
	"kwu/internal/fileref"
	fileenc "kwu/internal/fileutil/encoding"
	"kwu/internal/guardian"
	"kwu/internal/i18n"
	"kwu/internal/mcpdiag"
	"kwu/internal/memory"
	"kwu/internal/modelbilling"
	"kwu/internal/newapipricing"
	"kwu/internal/plugin"
	"kwu/internal/provider"
	"kwu/internal/scene"
	"kwu/internal/skill"
	"kwu/internal/store"
	"kwu/internal/userctx"
)

// eventChannel is the Wails runtime event name the frontend subscribes to for the
// agent's typed event stream. One channel carries every event kind; the payload's
// `kind` field discriminates — the desktop analogue of the serve transport's SSE
// `data:` frames.
const eventChannel = "agent:event"

// conversationActivityEventChannel carries only turn lifecycle events. Unlike
// eventChannel it is app-scoped, so a detached background runtime can still
// report completion without routing transcript output into the active tab.
const conversationActivityEventChannel = "conversation:activity"

const exportFileTokenTTL = 10 * time.Minute
const exportFileTokenPrefix = "export:"

var desktopWorkspaceCwdMu sync.Mutex

// PromptHistoryEntry is one user prompt extracted from a session JSONL file.
// The frontend uses these for ↑/↓ prompt-history navigation.
type PromptHistoryEntry struct {
	Text        string `json:"text"`
	At          int64  `json:"at"` // unix ms
	SessionPath string `json:"sessionPath"`
	Turn        int    `json:"turn"`
}

// PromptHistoryResult is returned as one Wails value. It carries one loaded tape
// segment plus the cursor needed to keep walking toward older prompts.
type PromptHistoryResult struct {
	Entries     []PromptHistoryEntry `json:"entries"`
	Nonce       string               `json:"nonce"`
	OlderCursor string               `json:"olderCursor,omitempty"`
	HasOlder    bool                 `json:"hasOlder"`
}

type appActorSnapshot struct {
	actor    userctx.ActorContext
	explicit bool
}

type tabActivationRuntime struct {
	tab   *WorkspaceTab
	actor userctx.ActorContext
}

// tabActivationSnapshot is immutable after publication. Activity switching
// reads it through atomic.Pointer so it never has to enter App.mu's broad tab
// critical section just to resolve a runtime identity.
type tabActivationSnapshot struct {
	byID map[string]tabActivationRuntime
}

// App is the Wails-bound application object: the desktop frontend's command
// surface. Its exported methods (Submit/Cancel/Approve/…) are generated into JS
// bindings. The app manages multiple WorkspaceTabs — each with its own controller
// scoped to a project workspace — and routes commands to the active tab. Events
// flow the other way: each tab's controller emits to a tabEventSink that
// forwards events tagged with tabId to the webview via runtime.EventsEmit.
type App struct {
	// ctx 是 Wails 应用生命周期上下文，用于调用原生能力和向前端发送事件；
	// 由 startup 在应用启动时设置。
	ctx context.Context

	// cloudRuntime 管理已认证的云端客户端及其后台任务。
	cloudRuntime *cloudRuntime
	// cloudBootstrapBeforePublish is a package-test seam at the actor-scoped
	// bootstrap publication boundary. Production instances leave it nil.
	cloudBootstrapBeforePublish func()

	// connectorService 管理连接器目录、安装状态以及按所有者隔离的生命周期操作。
	connectorService desktopConnectorRuntime
	// connectorInitErr 保存连接器初始化失败原因，供后续 API 返回明确的不可用错误。
	connectorInitErr error
	// connectorConnectMu 串行化桌面端全局的连接器连接流程，包括浏览器授权和凭据注册。
	connectorConnectMu sync.Mutex
	// connectorAuthMu 保护 connectorAuthFlows。
	connectorAuthMu sync.Mutex
	// connectorAuthFlows 保存正在进行的、按所有者隔离的连接器授权流程取消函数。
	connectorAuthFlows map[string]context.CancelFunc
	// connectorOAuthLocks 为每个所有者、连接器和 MCP 服务组合提供独立的凭据变更锁。
	connectorOAuthLocks sync.Map
	// connectorCredentials 可覆盖默认的连接器凭据存储；生产环境为空时使用平台凭据存储。
	connectorCredentials connectorCredentialStore
	// connectorOAuthClient 可覆盖 OAuth 客户端构造逻辑，主要用于确定性测试。
	connectorOAuthClient connectorOAuthClientFactory
	// connectorBindingResolver 可覆盖云端绑定解析逻辑，用于校验连接器所有者范围。
	connectorBindingResolver cloud.BindingResolver
	// connectorOAuthRetryDelays 可覆盖凭据读取重试间隔；为 nil 时使用生产退避策略。
	connectorOAuthRetryDelays []time.Duration
	// connectorOAuthRuntimeFailures 按所有者、连接器和服务记录临时 OAuth 故障，
	// 后续刷新成功时清除。
	connectorOAuthRuntimeFailures sync.Map

	// billingWorkerRuntime 协调控制器生命周期内的 BYOK 和订阅计费后台任务。
	billingWorkerRuntime *desktopBillingWorkerManager
	// expertCatalog 协调专家目录的后台刷新。
	expertCatalog *expertCatalogCoordinator

	// expertCatalogSyncMu 串行化远程专家目录刷新。任务切换和目录刷新可能同时发起请求，
	// 串行执行可防止较旧的响应覆盖较新的内存快照。
	expertCatalogSyncMu sync.Mutex
	// expertCatalogCacheMu 保护 cloudExpertDefinitions。
	expertCatalogCacheMu sync.RWMutex
	// cloudExpertDefinitions 保存远程专家目录最新的深拷贝快照。
	cloudExpertDefinitions []expert.Definition

	// mu 保护进程内 tab runtime、绑定方法读取的标签页字段以及 botInstalls；
	// 所有访问控制器的绑定方法都通过 activeCtrl() 获取控制器。
	mu sync.RWMutex
	// tabRuntimes 只保存无法持久化的 controller、event sink 等进程运行时对象。
	// 可见 tab 的 ID、顺序、active ID 和恢复元数据以 SQLite 为事实来源。
	tabRuntimes map[string]*WorkspaceTab
	// tabOrder 保存前端展示的标签页 ID 顺序。
	tabOrder []string
	// activeTabID 标识未显式指定目标时接收命令的标签页；没有活动标签页时为空。
	// 仅作为尚未接入 SQLite 的测试/启动兼容值；运行期读写使用 activeTabIDValue。
	activeTabID string
	// activeTabIDValue 是 SQLite active_tab_id 的进程内原子镜像。
	activeTabIDValue atomic.Pointer[string]
	// tabStoreReady 标记启动恢复已把 SQLite 状态接入当前 runtime。
	tabStoreReady atomic.Bool
	// tabActivationIndex 是 tabRuntimes 中切换所需最小信息的不可变原子快照。
	tabActivationIndex atomic.Pointer[tabActivationSnapshot]
	// currentActor 保存规范化后的桌面端用户身份和配置档案上下文。
	currentActor userctx.ActorContext
	// currentActorExplicit 标记 currentActor 是否由调用方显式指定，而非从活动标签页推断。
	currentActorExplicit bool
	// currentActorValue 提供不获取 App.mu 的规范 actor 快照。
	currentActorValue atomic.Pointer[appActorSnapshot]
	// readyHook 是包内测试钩子；非 nil 时调用它代替发送常规的就绪事件。
	readyHook func()
	// projectTreeChangedHook 是包内测试钩子；非 nil 时调用它代替发送项目树变更事件。
	projectTreeChangedHook func()
	// projectTreeRevision 是项目树变更通知携带的单调递增版本号。
	projectTreeRevision atomic.Uint64
	// conversationActivity stores accepted-turn timestamps by session path so
	// the sidebar can reorder immediately, before the next autosave reaches disk.
	conversationActivity      sync.Map
	conversationActivityClock atomic.Int64
	// projectTreeMetadataRetryScheduled 防止重复启动元数据修复重试协程。
	projectTreeMetadataRetryScheduled atomic.Bool
	// projectTreeMetadataRetryAttempt 用于计算有上限的指数退避时间。
	projectTreeMetadataRetryAttempt atomic.Uint32
	// subscriptionModelRefreshEpoch 以无锁世代号取消过期的 MaaS 目录重试。
	// 登录、切换账号和退出都会推进世代，只有最新身份的重试可以发布结果。
	subscriptionModelRefreshEpoch atomic.Uint64
	// clientTimezone 保存浏览器上报的 IANA 时区，并应用于所有活动标签页。
	clientTimezone atomic.Pointer[string]
	// sessionCatalogRefreshes 按会话目录去重目录修复等待任务。
	sessionCatalogRefreshes sync.Map
	// projectSessionLocks 按目标标签页串行化云端项目会话创建。
	projectSessionLocks sync.Map

	// singleSurfaceMu 串行化单会话布局中的打开、复用和可见标签页裁剪，
	// 防止并发导航移除另一个导航正在激活的标签页。
	singleSurfaceMu sync.Mutex

	// toolApprovalTransitionMu 在用户快速切换工具审批模式或会话界面时，
	// 保证持久化状态和运行时状态的更新顺序一致。
	toolApprovalTransitionMu sync.Mutex

	// detachedSessions 保存可见标签页已关闭但仍在运行的会话；由 mu 保护且仅在当前进程有效，
	// 应用退出时会关闭其中所有控制器。
	detachedSessions map[string]*WorkspaceTab

	// sharedHosts 为每个工作区根目录保存一个由该目录下所有控制器和标签页共享的
	// *plugin.Host，避免重复启动 MCP 子进程；首次获取时创建，最后一次释放时关闭。
	sharedHosts map[string]*sharedPluginHost
	// sharedHostsMu 保护 sharedHosts 及各主机的引用计数。
	sharedHostsMu sync.Mutex

	// tabsSaveVersion 为 SQLite 完整快照分配进程内单调版本，不依赖 App.mu。
	tabsSaveVersion atomic.Uint64

	// forceQuit 使显式退出操作绕过“关闭到后台”逻辑。
	forceQuit atomic.Bool
	// backgroundMaximised 记录隐藏窗口恢复时是否应保持最大化。
	backgroundMaximised atomic.Bool
	// trayReady 标记原生托盘回调是否已完成菜单项创建。
	trayReady bool
	// tray 保存原生托盘菜单及其本地化菜单项。
	tray *desktopTray
	// singleInstanceSignalClose 在应用退出时释放主实例监听器。
	singleInstanceSignalClose func()

	// mediaTokens 为工作区媒体文件提供短时有效的 AssetServer 访问令牌。
	mediaTokens *mediaTokenStore
	// feedbackImages 管理用户为反馈报告选择的临时截图。
	feedbackImages *feedbackImageStore
	// botInstalls 保存待完成的二维码或设备码机器人安装会话，由 mu 保护。
	botInstalls map[string]*botInstallSession
	// botRuntime 管理已配置的机器人网关及其状态快照。
	botRuntime *desktopBotRuntime

	// metrics 仅在用户启用 desktop.metrics 时非 nil，可由 SetDesktopMetrics 动态替换。
	metrics atomic.Pointer[metricsAggregator]

	// runtimeEvents 按顺序异步发送应用级 Wails 事件。
	runtimeEvents asyncRuntimeEmitter
	// operationalEvidence owns the process-wide metadata-only diagnostic stream.
	// Production keeps a bounded memory ring; development profiles additionally
	// persist the same records beneath their isolated cache root.
	operationalEvidence *operationalEvidenceRuntime
	// turnBillingAggregation joins the independently settled model and paid-tool
	// facts for a turn through immutable CAS snapshots.
	turnBillingAggregation atomic.Pointer[turnBillingAggregationSnapshot]

	// promptHistoryMu 保护 promptHistoryTape 及其导航游标状态。
	promptHistoryMu sync.Mutex
	// promptHistoryTape 是支持游标寻址的惰性提示词历史视图；仅当用户通过 ↑ 导航到
	// 某个会话时，才保存该会话的顺序和解析结果。参见 ScanPromptHistory。
	promptHistoryTape *promptHistoryTape

	// skillRootsMu 保护 skillRootsCache。
	skillRootsMu sync.Mutex
	// skillRootsCache 缓存技能根目录，避免重复扫描未变化的目录。
	skillRootsCache skillRootsCache

	// automationMu 串行化自动化服务的惰性初始化和关闭流程。
	automationMu sync.Mutex
	// automationService 是面向命令调用的自动化服务接口。
	automationService automation.Service
	// automationEngine 负责调度任务并分发任务执行和结果投递。
	automationEngine *automation.Engine
	// automationStore 是自动化服务的持久化存储，应用退出时关闭。
	automationStore *automation.Store

	// expertStoreMu 保护按配置档案隔离的专家持久化存储字段。
	expertStoreMu sync.Mutex
	// expertStoreInstance 是当前打开的本地专家存储实例。
	expertStoreInstance *expert.Store
	// expertStorePath 标识 expertStoreInstance 当前对应的配置档案路径。
	expertStorePath string

	// capabilityDashboardMu 串行化能力绑定、仪表盘和证据旁路文件的读写，
	// 使校验与原子文件替换在进程内形成一个完整事务。
	capabilityDashboardMu sync.Mutex

	// exportFilesMu 保护 exportFiles 及导出令牌消费过程。
	exportFilesMu sync.Mutex
	// exportFiles 将短时有效的下载令牌映射到本地导出目标文件。
	exportFiles map[string]exportFilePick

	// pricingReader 读取桌面端定价服务共享的当前 API 价格目录。
	pricingReader *newapipricing.Reader
	// pricePreparationReader 提供模型调用计费证据准备所需的定价元数据。
	pricePreparationReader *newapipricing.PricePreparationReader
	// modelMarketplace 提供本地缓存的模型市场数据及刷新能力。
	modelMarketplace *modelMarketplaceService
	// subscriptionModels 按云端绑定缓存订阅凭据及其可用模型目录。
	subscriptionModels *desktopSubscriptionModelCatalog

	// desktopBillingRuntimeObserver 是包内测试钩子；生产环境保持为 nil，测试用它验证
	// 控制器持有的 SQLite 运行时是否在事务式重建的正确阶段关闭。
	desktopBillingRuntimeObserver func(*desktopBillingRuntime)
	// tabControllerBuilder 是包内测试钩子，用于在本地会话预加载完成后稳定模拟控制器启动失败。
	tabControllerBuilder func(context.Context, boot.Options, userctx.ActorContext, string, string, string) (*control.Controller, error)

	// heartbeat 是旧版心跳兼容辅助对象；实际调度由 automation 负责。
	heartbeat *HeartbeatEngine
}

type exportFilePick struct {
	path      string
	expiresAt time.Time
}

type skillRootsCache struct {
	key   string
	at    time.Time
	roots []SkillRootView
}

// mediaTokenEntry holds metadata for a workspace media file served via temporary URL.
type mediaTokenEntry struct {
	absPath   string
	filename  string
	mime      string
	kind      string
	size      int64
	modTime   time.Time
	createdAt time.Time
	expiresAt time.Time
}

// mediaTokenStore manages temporary tokens that grant access to workspace files
// through the AssetServer middleware. Tokens expire after a fixed TTL and are
// capped at a maximum count; creating a new token evicts the oldest entry when
// the store is full.
type mediaTokenStore struct {
	mu    sync.Mutex
	byTok map[string]*mediaTokenEntry
	order []string // oldest first
	maxN  int
	ttl   time.Duration
}

const mediaTokenMax = 256

func newMediaTokenStore() *mediaTokenStore {
	return &mediaTokenStore{
		byTok: map[string]*mediaTokenEntry{},
		maxN:  mediaTokenMax,
		ttl:   10 * time.Minute,
	}
}

func (s *mediaTokenStore) cleanupLocked() {
	now := time.Now()
	for len(s.order) > 0 {
		tok := s.order[0]
		e := s.byTok[tok]
		if e == nil {
			s.order = s.order[1:]
			continue
		}
		if !now.Before(e.expiresAt) {
			delete(s.byTok, tok)
			s.order = s.order[1:]
			continue
		}
		break
	}
	for len(s.order) > s.maxN {
		oldest := s.order[0]
		delete(s.byTok, oldest)
		s.order = s.order[1:]
	}
}

func (s *mediaTokenStore) create(absPath, filename, mime, kind string, size int64, modTime time.Time) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupLocked()

	tok := make([]byte, 16)
	if _, err := rand.Read(tok); err != nil {
		panic("crypto/rand.Read failed: " + err.Error())
	}
	token := hex.EncodeToString(tok)

	now := time.Now()
	s.byTok[token] = &mediaTokenEntry{
		absPath:   absPath,
		filename:  filename,
		mime:      mime,
		kind:      kind,
		size:      size,
		modTime:   modTime,
		createdAt: now,
		expiresAt: now.Add(s.ttl),
	}
	s.order = append(s.order, token)

	// Trim oldest if the new token pushed us over the limit.
	for len(s.order) > s.maxN {
		oldest := s.order[0]
		delete(s.byTok, oldest)
		s.order = s.order[1:]
	}

	return token
}

func (s *mediaTokenStore) get(token string) *mediaTokenEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.byTok[token]
	if e == nil {
		return nil
	}
	if time.Now().After(e.expiresAt) {
		delete(s.byTok, token)
		return nil
	}
	return e
}

func (a *App) ensureMediaTokenStore() *mediaTokenStore {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.mediaTokens == nil {
		a.mediaTokens = newMediaTokenStore()
	}
	return a.mediaTokens
}

// workspaceMediaMiddleware returns an HTTP middleware that intercepts
// /__kwu_workspace_media/{token}/{filename} requests and serves the
// corresponding workspace file. All other paths pass through to the Wails
// default asset handler unchanged.
func (a *App) workspaceMediaMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			prefix := "/__kwu_workspace_media/"
			if !strings.HasPrefix(r.URL.Path, prefix) {
				next.ServeHTTP(w, r)
				return
			}

			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			rest := strings.TrimPrefix(r.URL.Path, prefix)
			parts := strings.SplitN(rest, "/", 2)
			if len(parts) == 0 || parts[0] == "" {
				http.NotFound(w, r)
				return
			}
			token := parts[0]

			entry := a.ensureMediaTokenStore().get(token)
			if entry == nil {
				http.NotFound(w, r)
				return
			}

			f, err := os.Open(entry.absPath)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			defer f.Close()

			w.Header().Set("Content-Type", entry.mime)
			w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": entry.filename}))
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Cache-Control", "private, max-age=600")
			http.ServeContent(w, r, entry.filename, entry.modTime, f)
		})
	}
}

// NewApp constructs the bound object. Tabs are restored in startup from the
// last session's SQLite tab state.
func NewApp() *App {
	return newAppWithDependencies(appConstructionDependencies{
		initializeProfile:   ensureDesktopProfileInitialized,
		newCloudRuntime:     newCloudRuntimeFromEnvironment,
		newConnectorRuntime: newDesktopConnectorRuntime,
		newBillingRuntime:   newDesktopBillingWorkerManager,
		newPricing:          newDefaultPricingDependencies,
	})
}

type appConstructionDependencies struct {
	initializeProfile   func() (desktopProfile, error)
	newCloudRuntime     func() *cloudRuntime
	newConnectorRuntime func() (desktopConnectorRuntime, error)
	newBillingRuntime   func(desktopBillingWorkerFactory) *desktopBillingWorkerManager
	newPricing          func() (*newapipricing.Reader, *newapipricing.PricePreparationReader, *modelMarketplaceService, error)
}

func newAppWithDependencies(dependencies appConstructionDependencies) *App {
	if dependencies.initializeProfile == nil {
		return &App{cloudRuntime: &cloudRuntime{initErr: fmt.Errorf("desktop profile initializer is unavailable")}}
	}
	profile, err := dependencies.initializeProfile()
	if err != nil {
		return &App{cloudRuntime: &cloudRuntime{initErr: err}}
	}
	app := &App{
		tabRuntimes:         map[string]*WorkspaceTab{},
		currentActor:        userctx.Default(""),
		detachedSessions:    map[string]*WorkspaceTab{},
		mediaTokens:         newMediaTokenStore(),
		feedbackImages:      newFeedbackImageStore(),
		botInstalls:         map[string]*botInstallSession{},
		botRuntime:          newDesktopBotRuntime(),
		subscriptionModels:  newDesktopSubscriptionModelCatalog(),
		operationalEvidence: newOperationalEvidenceRuntime(profile),
	}
	app.currentActorValue.Store(&appActorSnapshot{actor: app.currentActor.Normalize()})
	app.expertCatalog = newExpertCatalogCoordinator(app)
	if dependencies.newConnectorRuntime != nil {
		app.connectorService, app.connectorInitErr = dependencies.newConnectorRuntime()
	}
	if dependencies.newCloudRuntime == nil {
		app.cloudRuntime = &cloudRuntime{initErr: fmt.Errorf("cloud runtime factory is unavailable")}
		return app
	}
	cloudService := dependencies.newCloudRuntime()
	app.cloudRuntime = cloudService
	if dependencies.newBillingRuntime == nil {
		return app
	}
	billingRuntime := dependencies.newBillingRuntime(desktopBillingWorkerFactoryForRuntime(func() *cloudRuntime { return app.cloudRuntime }))
	billingRuntime.subscriptionEnabled = cloudService != nil && cloudService.available && cloudService.initErr == nil
	app.billingWorkerRuntime = billingRuntime
	if cloudService != nil && cloudService.settlements != nil {
		cloudService.settlements.bind(billingRuntime)
	}
	if dependencies.newPricing != nil {
		app.pricingReader, app.pricePreparationReader, app.modelMarketplace, _ = dependencies.newPricing()
	}
	return app
}

// SetClientTimezone stores the browser-reported IANA timezone and applies it
// to every live tab. It never derives timezone from IP, locale, or VPN state.
func (a *App) SetClientTimezone(name string) error {
	name = strings.TrimSpace(name)
	if name != "" {
		if _, err := time.LoadLocation(name); err != nil {
			return fmt.Errorf("invalid client timezone %q: %w", name, err)
		}
	}
	value := name
	a.clientTimezone.Store(&value)

	a.mu.RLock()
	tabs := a.runtimeTabsLocked()
	controllers := make([]control.SessionAPI, 0, len(tabs))
	for _, tab := range tabs {
		if tab != nil && tab.Ctrl != nil {
			controllers = append(controllers, tab.Ctrl)
		}
	}
	a.mu.RUnlock()
	for _, ctrl := range controllers {
		if err := ctrl.SetTimezone(name); err != nil {
			return err
		}
	}
	return nil
}

// ClientTimezone returns the last browser-reported IANA timezone.
func (a *App) ClientTimezone() string {
	if a == nil {
		return ""
	}
	if value := a.clientTimezone.Load(); value != nil {
		return *value
	}
	return ""
}

func (a *App) bootContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func (a *App) currentActorContext() userctx.ActorContext {
	if a == nil {
		return userctx.Default("")
	}
	if state := a.currentActorValue.Load(); state != nil {
		return state.actor.Normalize()
	}
	a.mu.RLock()
	actor := a.currentActor
	a.mu.RUnlock()
	return actor.Normalize()
}

func (a *App) currentActorContextLocked() userctx.ActorContext {
	if a == nil {
		return userctx.Default("")
	}
	if state := a.currentActorValue.Load(); state != nil {
		return state.actor.Normalize()
	}
	return a.currentActor.Normalize()
}

func (a *App) currentActorSnapshot() appActorSnapshot {
	if a == nil {
		return appActorSnapshot{actor: userctx.Default("")}
	}
	if state := a.currentActorValue.Load(); state != nil {
		return *state
	}
	// Compatibility for package tests that still construct App with fields.
	return appActorSnapshot{actor: a.currentActor.Normalize(), explicit: a.currentActorExplicit}
}

func (a *App) cloudActorContext() userctx.ActorContext {
	if a == nil {
		return userctx.Default("")
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cloudActorContextForTabLocked(a.activeTabLocked())
}

func (a *App) cloudActorContextForTabLocked(tab *WorkspaceTab) userctx.ActorContext {
	if a.currentActorExplicit {
		return a.currentActor.Normalize()
	}
	if tab != nil {
		return tabRuntimeActorContext(tab).Normalize()
	}
	return a.currentActor.Normalize()
}

func sameDesktopActorProfile(left, right userctx.ActorContext) bool {
	left, right = left.Normalize(), right.Normalize()
	return left.ActorID == right.ActorID && left.ProfileID == right.ProfileID
}

func (a *App) currentActorContextSpecified() bool {
	if a == nil {
		return false
	}
	if state := a.currentActorValue.Load(); state != nil {
		return state.explicit
	}
	a.mu.RLock()
	specified := a.currentActorExplicit
	a.mu.RUnlock()
	return specified
}

func (a *App) setCurrentActorContext(actor userctx.ActorContext) {
	if a == nil {
		return
	}
	actor = actor.Normalize()
	a.currentActorValue.Store(&appActorSnapshot{actor: actor, explicit: true})
	a.mu.Lock()
	a.currentActor = actor
	a.currentActorExplicit = true
	a.mu.Unlock()
}

func (a *App) actorForWorkspace(workspaceRoot string) userctx.ActorContext {
	actor := a.currentActorContext()
	actor.WorkspaceRoot = strings.TrimSpace(workspaceRoot)
	return actor.Normalize()
}

func (a *App) actorForWorkspaceLocked(workspaceRoot string) userctx.ActorContext {
	actor := a.currentActorContextLocked()
	actor.WorkspaceRoot = strings.TrimSpace(workspaceRoot)
	return actor.Normalize()
}

func (a *App) activeConfigActorForWorkspace(workspaceRoot string) userctx.ActorContext {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if a.currentActorContextSpecified() {
		return a.actorForWorkspace(workspaceRoot)
	}
	if tab := a.activeTab(); tab != nil {
		return actorContextForWorkspace(tabRuntimeActorContext(tab), workspaceRoot)
	}
	return a.actorForWorkspace(workspaceRoot)
}

func (a *App) currentProfilePathResolver(workspaceRoot string) config.ProfilePathResolver {
	return config.NewProfilePathResolver(a.actorForWorkspace(workspaceRoot))
}

// Platform exposes the native OS to the frontend so chrome/layout affordances can
// stay platform-scoped instead of relying on browser user-agent guesses.
func (a *App) Platform() string {
	return goruntime.GOOS
}

// startup runs once the webview process is up, before the frontend can issue any
// bound call. It captures the Wails context (needed for EventsEmit), then kicks
// off the initialization in a background goroutine so the webview loads immediately.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.startConnectorService(ctx)
	// The output-only RC bootstrap must seed its isolated session before any
	// background worker can observe and terminalize an anonymous profile. The
	// ordinary build keeps this call as a no-op.
	if err := prepareWorkBuddyRCTest(ctx, a); err != nil {
		slog.Error("WorkBuddy RC test bootstrap failed", "code", "bootstrap_failed")
	}
	if a.billingWorkerRuntime != nil {
		a.billingWorkerRuntime.start(ctx)
	}
	if a.cloudRuntime != nil {
		a.cloudRuntime.bindAssetsChangedSender(func(payload CloudAssetsChangedView) {
			runtime.EventsEmit(ctx, cloudAssetsChangedEvent, payload)
		})
		a.cloudRuntime.bindTurnBillingSettledSender(func(payload TurnBillingSettledView) {
			a.observeModelTurnSettlement(payload)
		})
		a.cloudRuntime.start(ctx)
	}
	if a.expertCatalog != nil {
		a.expertCatalog.start(ctx)
	}
	a.recoverDesktopStartupState(ctx)
	installSystemQuitHook()
	a.startTray()

	if cfg, err := a.desktopNonRouteConfig(); err == nil && cfg.DesktopMetrics() && version != "dev" {
		a.metrics.Store(newMetricsAggregator(config.MemoryUserDir()))
		a.recordSettingsMetricsSnapshot(cfg)
	}

	a.startCloudProjectBootstrap()
	a.startAutomation(ctx)

	go a.restoreOrBuildTabs()
	a.goSafe("refreshBotRuntime", a.refreshBotRuntime)
	a.goSafe("sendStartupPing", a.sendStartupPing)
	a.goSafe("flushMetrics", a.flushMetrics)
	a.goSafe("flushPendingCrash", a.flushPendingCrash)
}

func (a *App) startConnectorService(ctx context.Context) {
	if a.connectorService == nil {
		return
	}
	if err := a.connectorService.Start(ctx); err != nil {
		// The public Catalog is retryable. Constructor failures remain sticky,
		// while ConnectorCatalog may retry a transient first fetch through the
		// already-created runtime.
		slog.Warn("desktop connector service startup failed", diagnostics.FailureMarkerKey, diagnostics.FailureMarker, "scope", "desktop.startup", "err", err)
	}
}

// recoverDesktopStartupState restores local execution windows before any tab
// can construct a controller. BYOK is profile-local and always runs; cloud
// session state only decides whether the subscription worker and event stream
// may resume.
func (a *App) recoverDesktopStartupState(ctx context.Context) {
	if a == nil {
		return
	}
	if err := a.recoverDesktopActorState(ctx, a.cloudActorContext()); err != nil {
		slog.Warn("desktop startup recovery failed", diagnostics.FailureMarkerKey, diagnostics.FailureMarker, "scope", "desktop.startup", "err", err)
	}
}

func (a *App) recoverDesktopActorState(ctx context.Context, actor userctx.ActorContext) error {
	if a == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	actor = actor.Normalize()
	if a.billingWorkerRuntime != nil {
		if err := a.billingWorkerRuntime.recoverBYOK(ctx, actor); err != nil {
			return fmt.Errorf("desktop BYOK recovery: %w", err)
		}
	}
	if a.cloudRuntime == nil || !a.cloudRuntime.available || a.cloudRuntime.initErr != nil || a.cloudRuntime.closed.Load() {
		return nil
	}
	if a.cloudRuntime.settlements != nil {
		a.cloudRuntime.settlements.bind(a.billingWorkerRuntime)
	}
	auth, authErr := a.cloudRuntime.recoverAuth(ctx, actor)
	if err := a.reconcileDesktopBillingAuth(a.cloudRuntime, actor, auth); err != nil {
		return errors.Join(authErr, fmt.Errorf("desktop subscription recovery: %w", err))
	}
	if authErr != nil {
		return fmt.Errorf("desktop cloud session recovery: %w", authErr)
	}
	if auth.State == cloud.AccountAuthenticated && auth.Binding.Status == cloud.BindingBound {
		a.refreshDesktopSubscriptionModelsBestEffort(actor)
	}
	if auth.State != cloud.AccountAuthenticated || auth.Binding.Status != cloud.BindingBound || a.cloudRuntime.eventSync == nil {
		return nil
	}
	scope, err := auth.Binding.Scope()
	if err != nil {
		return fmt.Errorf("desktop cloud binding recovery: %w", err)
	}
	if err := a.cloudRuntime.ensureEventWorker(actor, scope); err != nil {
		return fmt.Errorf("desktop cloud event recovery: %w", err)
	}
	return nil
}

func (a *App) beforeClose(ctx context.Context) bool {
	if a.forceQuit.Swap(false) || consumeSystemQuitRequested() {
		// Capture geometry while the native window and its DPI context are still
		// valid. OnShutdown runs after Windows has started tearing them down.
		a.saveWindowStateSync()
		return false
	}
	cfg, _, err := a.loadDesktopUserConfigForEdit()
	if err != nil {
		// The close path only needs desktop preferences. Keep the same active
		// route policy as every other desktop read so a subscription tab never
		// falls back to a credential-hydrating provider loader.
		if fallback, fallbackErr := a.desktopNonRouteConfig(); fallbackErr == nil {
			cfg = fallback
		} else {
			cfg = config.Default()
		}
	}
	if cfg.DesktopCloseBehavior() == "background" {
		a.backgroundMaximised.Store(runtime.WindowIsMaximised(ctx))
		a.saveWindowStateSync()
		a.snapshotAllTabs()
		hideForBackground(ctx)
		return true
	}
	a.saveWindowStateSync()
	return false
}

func (a *App) showMainWindow() {
	if a.ctx != nil {
		showFromBackground(a.ctx, a.backgroundMaximised.Swap(false))
	}
}

func (a *App) secondInstanceLaunch() {
	a.showMainWindow()
}

func (a *App) quitApp() {
	if a.ctx == nil {
		return
	}
	a.forceQuit.Store(true)
	runtime.Quit(a.ctx)
}

func hideForBackground(ctx context.Context) {
	if backgroundCloseUsesApplicationHide(goruntime.GOOS) {
		runtime.Hide(ctx)
		return
	}
	runtime.WindowHide(ctx)
}

func showFromBackground(ctx context.Context, wasMaximised bool) {
	if backgroundCloseUsesApplicationHide(goruntime.GOOS) {
		runtime.Show(ctx)
	}
	plan := backgroundRestorePlanFor(goruntime.GOOS, wasMaximised)
	if plan.maximiseBeforeShow {
		runtime.WindowMaximise(ctx)
	}
	runtime.WindowShow(ctx)
	if plan.unminimiseAfterShow {
		runtime.WindowUnminimise(ctx)
	}
}

func backgroundCloseUsesApplicationHide(goos string) bool {
	return goos == "darwin"
}

type backgroundRestorePlan struct {
	maximiseBeforeShow  bool
	unminimiseAfterShow bool
}

func backgroundRestorePlanFor(goos string, wasMaximised bool) backgroundRestorePlan {
	if backgroundRestoreShouldMaximise(goos, wasMaximised) {
		return backgroundRestorePlan{maximiseBeforeShow: true}
	}
	return backgroundRestorePlan{unminimiseAfterShow: true}
}

func backgroundRestoreShouldMaximise(goos string, wasMaximised bool) bool {
	return wasMaximised && !backgroundCloseUsesApplicationHide(goos)
}

// restoreOrBuildTabs restores the tabs from the last session, or creates a
// default Global tab on first launch.
func (a *App) restoreOrBuildTabs() {
	defer a.recoverToPending("restoreOrBuildTabs")
	// Reap any orphaned codegraph processes from a previous crash or older
	// version that leaked them, so they don't accumulate across restarts.
	a.reapOrphanCodeGraph()
	ctx := a.ctx
	a.ensureWorkspace()

	// Run legacy config migration before the first config load so the
	// freshly written config (including the user's default_model) is
	// picked up by Load instead of falling back to built-in defaults.
	if _, err := config.MigrateLegacyIfNeeded(); err != nil {
		recordDesktopBackgroundFailure("config.migrate.legacy", err)
	}
	f := a.loadTabsFile()
	a.mu.Lock()
	if f.Version > a.tabsSaveVersion.Load() {
		a.tabsSaveVersion.Store(f.Version)
	}
	a.tabStoreReady.Store(true)
	a.mu.Unlock()
	// The upgrade pricing rewrite uses an edit loader. Strict fake startup must
	// decide each tab's billing route before any loader can touch BYOK
	// credentials, so defer this non-essential legacy maintenance outside it.
	if !desktopCloudFakeEnabled() {
		if _, err := config.MigrateConfigOnUpgrade(a.currentProfilePathResolver("").UserConfigPath()); err != nil {
			recordDesktopBackgroundFailure("config.migrate.upgrade", err)
		}
	}
	if _, err := config.MigrateMCPToUserConfigOnUpgrade(a.desktopMCPMigrationRoots(f)); err != nil {
		recordDesktopBackgroundFailure("config.migrate.mcp", err)
	}

	// Load i18n from the first available config.
	// Prefer DesktopLanguage (desktop UI setting) over Language (CLI setting),
	// so the user's language choice in desktop settings takes effect.
	startupCfg, _, cfgErr := a.loadDesktopUserConfigForView()
	if cfgErr == nil {
		cfg := startupCfg
		lang := cfg.DesktopLanguage()
		if lang == "" {
			lang = cfg.Language
		}
		i18n.DetectLanguage(lang)
	}
	if cfgErr != nil || singleSurfaceLayoutStyle(startupCfg.DesktopLayoutStyle()) {
		f = singleSurfaceTabsFile(f)
	}
	if cfgErr == nil {
		for i := range f.Tabs {
			f.Tabs[i].Model = config.RetargetDesktopOfficialModelRef(f.Tabs[i].Model, startupCfg.Desktop.ProviderAccess)
		}
	}

	if len(f.Tabs) > 0 {
		type restoredTabBuild struct {
			tab       *WorkspaceTab
			preloaded loadedTabSession
		}
		toBuild := make([]restoredTabBuild, 0, len(f.Tabs))
		for _, entry := range f.Tabs {
			a.mu.Lock()
			id := a.restoredTabIDLocked(entry.ID)
			a.mu.Unlock()

			workspaceRoot := entry.WorkspaceRoot
			scope := entry.Scope
			if scope != "project" {
				scope = "global"
				workspaceRoot = globalTabWorkspaceRoot()
			}
			actor := a.actorForWorkspace(workspaceRoot)
			probe := &WorkspaceTab{ID: id, Scope: scope, WorkspaceRoot: workspaceRoot, TopicID: entry.TopicID, model: entry.Model, actor: actor}
			sessionPath, replacedSession := restoredTabSessionPath(probe, entry.SessionPath)
			composerState, hasComposerState := loadSessionComposerStateWithFallback(sessionPath, entry.Model, entry.BillingSource, entry.Effort)
			restoredSource := a.defaultBillingSourceForNewTask(actor)
			if hasComposerState {
				restoredSource = composerState.billingSource
			}
			tab := a.createTabEntryWithIDAndActorForRoute(scope, workspaceRoot, entry.TopicID, id, actor, restoredSource)
			status, sceneErr := sceneStatusFromPersistedEntry(entry)
			if sceneErr != nil {
				slog.Warn("invalid persisted desktop scene; using legacy compatibility", diagnostics.FailureMarkerKey, diagnostics.FailureMarker, "scope", "session.recovery", "tab_id", id, "err", sceneErr)
			}
			setTabSceneStatus(tab, status)
			tab.tokenMode = boot.NormalizeTokenMode(entry.TokenMode)
			tab.mode = persistedTabMode(entry.Mode)
			tab.goal = strings.TrimSpace(entry.Goal)
			tab.toolApprovalMode = normalizeToolApprovalMode(entry.ToolApprovalMode)
			tab.toolApprovalModeKnown = strings.TrimSpace(entry.ToolApprovalMode) != ""
			if tab.toolApprovalMode == control.ToolApprovalAsk && tabModeHasAutoApproveTools(entry.Mode) {
				tab.toolApprovalMode = control.ToolApprovalYolo
				tab.toolApprovalModeKnown = true
			}
			if hasComposerState {
				applySessionComposerState(tab, composerState)
			}
			tab.SessionPath = sessionPath
			tab.ReadOnly = entry.ReadOnly && !replacedSession
			preloaded := loadedTabSession{}
			if strings.TrimSpace(tab.SessionPath) != "" {
				var preloadErr error
				preloaded, preloadErr = a.preloadTabSession(tab, tab.SessionPath)
				if preloadErr != nil {
					recordDesktopBackgroundFailure("session.preload.restore", preloadErr)
				}
				tab.sessionTurns = preloaded.sessionTurns()
			}
			tab.sink = &tabEventSink{tabID: tab.ID, app: a, ctx: ctx}
			a.mu.Lock()
			a.tabRuntimes[tab.ID] = tab
			a.tabOrder = append(a.tabOrder, tab.ID)
			a.mu.Unlock()
			toBuild = append(toBuild, restoredTabBuild{tab: tab, preloaded: preloaded})
		}
		a.mu.Lock()
		if _, ok := a.tabRuntimes[f.ActiveTab]; ok {
			a.setActiveTabIDLocked(f.ActiveTab)
		} else {
			ordered := a.orderedTabIDsLocked()
			if len(ordered) > 0 {
				a.setActiveTabIDLocked(ordered[0])
			}
		}
		a.saveTabsLocked()
		a.mu.Unlock()
		for _, target := range toBuild {
			a.startTabControllerBuildWithLoadedSession(target.tab, target.preloaded)
		}
		return
	}

	// First launch: create a default Global tab.
	tab := a.createTabEntry("global", globalTabWorkspaceRoot(), "")
	tab.sink = &tabEventSink{tabID: tab.ID, app: a, ctx: ctx}
	tab.TopicTitle = "Global"
	a.mu.Lock()
	if len(a.tabRuntimes) > 0 {
		a.mu.Unlock()
		return
	}
	a.tabRuntimes[tab.ID] = tab
	a.tabOrder = append(a.tabOrder, tab.ID)
	a.setActiveTabIDLocked(tab.ID)
	a.saveTabsLocked()
	a.mu.Unlock()
	a.startTabControllerBuild(tab)
}

func (a *App) createTabEntry(scope, workspaceRoot, topicID string) *WorkspaceTab {
	return a.createTabEntryWithID(scope, workspaceRoot, topicID, newTabID())
}

func (a *App) createTabEntryLocked(scope, workspaceRoot, topicID string) *WorkspaceTab {
	return a.createTabEntryWithIDAndActor(scope, workspaceRoot, topicID, newTabID(), a.actorForWorkspaceLocked(workspaceRoot))
}

func desktopNewSessionDefaults() (string, string) {
	if desktopCloudFakeEnabled() {
		return desktopNewSessionDefaultsForRoute("", userctx.ActorContext{}, "")
	}
	return desktopNewSessionDefaultsWithPaths(config.ProfilePathResolver{})
}

func desktopNewSessionDefaultsWithPaths(paths config.ProfilePathResolver) (string, string) {
	cfg := config.LoadForEditWithPaths(paths)
	return strings.TrimSpace(cfg.DefaultModel), normalizeToolApprovalMode(cfg.DesktopDefaultToolApprovalMode())
}

// desktopNewSessionDefaultsForRoute decides the credential policy before it
// reads configuration. A strict/fake subscription (or legacy tab with no route)
// may display model metadata, but it must never trigger a BYOK credential load.
func desktopNewSessionDefaultsForRoute(workspaceRoot string, actor userctx.ActorContext, source modelbilling.BillingSource) (string, string) {
	source = modelbilling.BillingSource(strings.TrimSpace(string(source)))
	if !desktopCloudFakeEnabled() && source != modelbilling.BillingSubscriptionPool {
		return desktopNewSessionDefaultsWithPaths(config.NewProfilePathResolver(actor))
	}
	cfg, _, err := desktopCatalogConfigForBillingRoute(workspaceRoot, actor, source)
	if err != nil || cfg == nil {
		cfg = config.Default()
	}
	model := strings.TrimSpace(cfg.DefaultModel)
	providerName, _, qualified := strings.Cut(model, "/")
	if source == modelbilling.BillingSubscriptionPool &&
		strings.TrimSpace(cfg.Desktop.DefaultBillingSource) == "" &&
		qualified && providerName == brand.ProviderID {
		model = brand.ProviderID + "/" + desktopUniversalSubscriptionModel
	}
	return model, normalizeToolApprovalMode(cfg.DesktopDefaultToolApprovalMode())
}

func (a *App) createTabEntryWithID(scope, workspaceRoot, topicID, id string) *WorkspaceTab {
	actor := a.actorForWorkspace(workspaceRoot)
	return a.createTabEntryWithIDAndActor(scope, workspaceRoot, topicID, id, actor)
}

func (a *App) createTabEntryWithIDAndActor(scope, workspaceRoot, topicID, id string, actor userctx.ActorContext) *WorkspaceTab {
	return a.createTabEntryWithIDAndActorForRoute(
		scope, workspaceRoot, topicID, id, actor, a.defaultBillingSourceForNewTask(actor),
	)
}

func (a *App) createTabEntryWithIDAndActorForRoute(scope, workspaceRoot, topicID, id string, actor userctx.ActorContext, source modelbilling.BillingSource) *WorkspaceTab {
	model, toolApprovalMode := a.desktopNewSessionDefaultsForRoute(workspaceRoot, actor, source)
	status := newDesktopSceneStatus()
	return &WorkspaceTab{
		ID:                    id,
		Scope:                 scope,
		WorkspaceRoot:         workspaceRoot,
		TopicID:               topicID,
		TopicTitle:            topicTitleForTab(scope, workspaceRoot, topicID),
		Scene:                 status.Scene,
		SceneState:            status.State,
		model:                 model,
		billingSource:         string(source),
		effort:                a.defaultEffortForNewSession(workspaceRoot, actor, source, model),
		tokenMode:             boot.TokenModeFull,
		mode:                  tabModeFromAxes(false, toolApprovalMode == control.ToolApprovalYolo),
		toolApprovalMode:      toolApprovalMode,
		toolApprovalModeKnown: true,
		disabledMCP:           map[string]ServerView{},
		actor:                 actor,
	}
}

func (a *App) snapshotAllTabs() {
	a.mu.RLock()
	tabs := a.runtimeTabsLocked()
	a.mu.RUnlock()
	for _, t := range tabs {
		if err := a.snapshotTab(t); err != nil {
			recordDesktopBackgroundFailure("session.snapshot.all", err)
		}
	}
}

// shutdown stops application runtimes, snapshots all tabs, and closes their
// controllers. It must not read native window state because Wails may invoke it
// after the frontend has already been destroyed.
func (a *App) shutdown(ctx context.Context) {
	defer a.closeOperationalEvidence(ctx)
	a.shutdownAutomation(ctx)
	a.cancelAllConnectorAuthorization()
	if a.feedbackImages != nil {
		a.feedbackImages.clearAll()
	}
	if a.expertCatalog != nil {
		a.expertCatalog.close()
	}
	if a.connectorService != nil {
		if err := a.connectorService.Close(); err != nil {
			slog.Warn("desktop connector service shutdown failed", diagnostics.FailureMarkerKey, diagnostics.FailureMarker, "scope", "desktop.shutdown", "err", err)
		}
	}
	a.closeExpertStore()
	// Cloud owns the event caller into the billing manager, so it must cancel
	// and drain that caller before billing waits for its operation lock.
	if a.cloudRuntime != nil {
		a.cloudRuntime.close()
	}
	if a.billingWorkerRuntime != nil {
		if err := a.billingWorkerRuntime.close(); err != nil {
			slog.Warn("desktop model-billing worker shutdown failed", diagnostics.FailureMarkerKey, diagnostics.FailureMarker, "scope", "desktop.shutdown", "err", err)
		}
	}
	if a.heartbeat != nil {
		a.heartbeat.Stop()
	}
	a.stopBotRuntime()
	a.stopTray()
	if a.singleInstanceSignalClose != nil {
		a.singleInstanceSignalClose()
		a.singleInstanceSignalClose = nil
	}
	// Close every shared plugin host on exit, even if a tab cleanup panics.
	defer a.closeAllSharedHosts()

	a.mu.RLock()
	tabs := a.runtimeTabsLocked()
	a.mu.RUnlock()
	for _, t := range tabs {
		if t.Ctrl != nil {
			if err := a.snapshotTab(t); err != nil {
				recordDesktopBackgroundFailure("session.snapshot.shutdown", err)
			}
			t.Ctrl.Close()
		}
	}
	closeDesktopErrorJournal()
}

// domReady is called (via OnDomReady) after the webview finishes loading its DOM
// but before the window is shown (StartHidden). It restores the saved window
// position and size, then calls WindowShow so the user never sees the default
// size/position flash.
func (a *App) domReady(_ context.Context) {
	state, ok := a.loadWindowState()
	if ok {
		// Validate saved position against current screens. Wails v2 doesn't
		// expose per-screen origin (x,y offsets) so we can only do a basic
		// sanity check: ensure the window origin falls within a generous
		// estimate of the screen area. If the user unplugged an external
		// display, negative or out-of-bounds coordinates are caught here.
		valid := state.X >= 0 && state.Y >= 0
		if valid {
			screens, err := runtime.ScreenGetAll(a.ctx)
			if err == nil && len(screens) > 0 {
				maxW, maxH := 0, 0
				for _, sc := range screens {
					if sc.Size.Width > maxW {
						maxW = sc.Size.Width
					}
					if sc.Size.Height > maxH {
						maxH = sc.Size.Height
					}
				}
				if state.X > maxW*2 || state.Y > maxH*2 {
					valid = false
				}
			}
		}
		if valid {
			runtime.WindowSetPosition(a.ctx, state.X, state.Y)
		} else {
			runtime.WindowCenter(a.ctx)
		}
	} else {
		runtime.WindowCenter(a.ctx)
	}

	if ok && state.Maximised {
		runtime.WindowMaximise(a.ctx)
	}
	if err := setMainWindowIcons(); err != nil {
		slog.Warn("set Windows main-window icons failed", "err", err)
	}

	runtime.WindowShow(a.ctx)
}

// --- bound command surface (frontend → controller) ---
// Each method guards on a nil controller so a pre-startup or failed-build call is
// a no-op, never a panic.

// Submit runs raw user input as a turn; slash commands and @-references are
// resolved by the controller. Output arrives asynchronously on eventChannel.
func (a *App) Submit(input string) error {
	return a.SubmitToTab("", input)
}

func (a *App) SubmitToTab(tabID, input string) error {
	tab, ctrl, readyErr := a.tabAndCtrlForUserWork(tabID)
	if tab != nil && tab.ReadOnly {
		return readOnlyChannelErr()
	}
	if readyErr != nil {
		return readyErr
	}
	trimmed := strings.TrimSpace(input)
	if trimmed == "/effort" || strings.HasPrefix(trimmed, "/effort ") {
		a.runEffortCommandForTab(tabID, trimmed)
		return nil
	}
	if err := a.chatEntryPointModelError(tab, input); err != nil {
		return err
	}
	ctrl.SubmitDisplay(input, input)
	return nil
}

func (a *App) submitUserTurnToTab(tabID, input string) bool {
	if a.tabReadOnly(tabID) {
		return false
	}
	_, ctrl, err := a.tabAndCtrlForUserWork(tabID)
	if err != nil {
		return false
	}
	ctrl.SubmitUserTurn(input, input)
	return true
}

// RunShell executes a shell command directly (bypassing the model) and streams
// output as events on eventChannel.
func (a *App) RunShell(command string) error {
	return a.RunShellForTab("", command)
}

func (a *App) RunShellForTab(tabID, command string) error {
	tab, ctrl, readyErr := a.tabAndCtrlForUserWork(tabID)
	if tab != nil && tab.ReadOnly {
		return readOnlyChannelErr()
	}
	if readyErr != nil {
		return readyErr
	}
	ctrl.RunShell(command)
	return nil
}

// SubmitDisplay runs input as a turn while recording a shorter UI-only display
// string for the saved desktop transcript. The model still receives input.
func (a *App) SubmitDisplay(display, input string) error {
	return a.SubmitDisplayToTab("", display, input)
}

func (a *App) SubmitDisplayToTab(tabID, display, input string) error {
	tab, ctrl, readyErr := a.tabAndCtrlForUserWork(tabID)
	if tab != nil && tab.ReadOnly {
		return readOnlyChannelErr()
	}
	if readyErr != nil {
		return readyErr
	}
	if err := a.chatEntryPointModelError(tab, input); err != nil {
		return err
	}
	ctrl.SubmitDisplay(display, input)
	return nil
}

func (a *App) bindControllerDisplayRecorder(ctrl control.SessionAPI) {
	if ctrl == nil {
		return
	}
	if admission, ok := ctrl.(interface {
		SetTurnAdmissionGuard(func() (func(), bool))
	}); ok {
		admission.SetTurnAdmissionGuard(func() (func(), bool) {
			a.mu.RLock()
			for _, tab := range a.tabRuntimes {
				if tab != nil && !tab.removed && tab.Ready && tab.Ctrl == ctrl &&
					(tab.sink == nil || tab.sink.currentTabID() == tab.ID) {
					return a.mu.RUnlock, true
				}
			}
			for _, tab := range a.detachedSessions {
				if tab != nil && !tab.removed && tab.Ready && tab.Ctrl == ctrl &&
					(tab.sink == nil || tab.sink.currentTabID() == tab.ID) {
					return a.mu.RUnlock, true
				}
			}
			a.mu.RUnlock()
			return nil, false
		})
	}
	ctrl.SetDisplayRecorder(func(content, display string) {
		dir := ctrl.SessionDir()
		if dir == "" {
			dir = desktopSessionDirForActor(ctrl.ActorContext(), ctrl.WorkspaceRoot())
		}
		if err := recordSessionDisplay(dir, ctrl.SessionPath(), content, display); err != nil {
			recordDesktopBackgroundFailure("session.display.persist", err)
		}
	})
	if acceptedTurns, ok := ctrl.(interface {
		SetAcceptedUserTurnHandler(func(raw, display, titleText string, preferTitleText bool))
	}); ok {
		acceptedTurns.SetAcceptedUserTurnHandler(func(raw, display, titleText string, preferTitleText bool) {
			a.recordAcceptedTranscript(ctrl, raw, display)
			a.recordAcceptedTabSessionTurn(ctrl)
			target, ok := a.acceptedTopicTitleTarget(ctrl)
			if !ok {
				return
			}
			go a.autoTitleAcceptedTopicTurn(target, raw, display, titleText, preferTitleText)
		})
	}
}

// Cancel aborts the in-flight turn.
func (a *App) Cancel() {
	a.CancelTab("")
}

func (a *App) CancelTab(tabID string) {
	if ctrl := a.ctrlByTabID(tabID); ctrl != nil {
		ctrl.Cancel()
	}
}

// Steer sends mid-turn guidance to the agent without interrupting the in-flight request.
func (a *App) Steer(text string) error {
	return a.SteerForTab("", text)
}

// SteerForTab sends mid-turn guidance to a specific tab's agent.
func (a *App) SteerForTab(tabID, text string) error {
	tab, ctrl, readyErr := a.tabAndCtrlForUserWork(tabID)
	if tab != nil && tab.ReadOnly {
		return readOnlyChannelErr()
	}
	if readyErr != nil {
		return readyErr
	}
	ctrl.Steer(text)
	return nil
}

func (a *App) tabReadOnly(tabID string) bool {
	tab := a.tabByID(tabID)
	return tab != nil && tab.ReadOnly
}

func (a *App) tabAndCtrlByID(tabID string) (*WorkspaceTab, control.SessionAPI) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	tab := a.tabByIDLocked(tabID)
	if tab == nil {
		return nil, nil
	}
	return tab, tab.Ctrl
}

func (a *App) tabAndCtrlForUserWork(tabID string) (*WorkspaceTab, control.SessionAPI, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	tab := a.tabByIDLocked(tabID)
	if tab == nil || tab.Ctrl == nil || !tab.Ready {
		return tab, nil, workspaceNotReadyErr(tab)
	}
	return tab, tab.Ctrl, nil
}

func readOnlyChannelErr() error {
	return fmt.Errorf("channel session is read-only")
}

func (a *App) snapshotTab(tab *WorkspaceTab) error {
	if tab == nil {
		return nil
	}
	tab.saveMu.Lock()
	if tab.saveCond == nil {
		tab.saveCond = sync.NewCond(&tab.saveMu)
	}
	for tab.saving {
		tab.saveCond.Wait()
	}
	if tab.closing {
		tab.saveAgain = true
		tab.saveMu.Unlock()
		return nil
	}
	tab.saving = true
	tab.saveMu.Unlock()

	completionEffects := false
	for {
		err := a.snapshotTabOnce(tab)
		if completionEffects && err == nil {
			a.completeTabSnapshot(tab)
		}
		tab.saveMu.Lock()
		if tab.closing {
			// Preserve saveAgain so the hand-off owner can transfer the request.
			tab.saving = false
			tab.saveCond.Broadcast()
			tab.saveMu.Unlock()
			return err
		}
		if tab.saveAgain {
			tab.saveAgain = false
			tab.saveMu.Unlock()
			completionEffects = true
			continue
		}
		tab.saving = false
		tab.saveCond.Broadcast()
		tab.saveMu.Unlock()
		return err
	}
}

func (a *App) snapshotTabOnce(tab *WorkspaceTab) error {
	if tab == nil {
		return nil
	}
	a.mu.RLock()
	readOnly := tab.ReadOnly
	ctrl := tab.Ctrl
	a.mu.RUnlock()
	if readOnly || ctrl == nil {
		return nil
	}
	batches := tab.takeArtifactDeliveries()
	if err := ctrl.Snapshot(); err != nil {
		tab.requeueArtifactDeliveries(batches)
		return err
	}
	if err := mergeArtifactDeliveryBatches(batches); err != nil {
		tab.requeueArtifactDeliveries(batches)
		return err
	}
	return nil
}

// Approve answers a pending approval_request by ID: allow runs the call, session
// also remembers the grant for the rest of the session.
func (a *App) Approve(id string, allow, session, persist bool) {
	ctrl := a.ctrlByTabID("")
	if ctrl != nil {
		ctrl.Approve(id, allow, session, persist)
	}
}

// ApproveTab is like Approve but scoped to a specific tab.
func (a *App) ApproveTab(tabID, id string, allow, session, persist bool) eventwire.PromptReplyReceipt {
	ctrl := a.ctrlByTabID(tabID)
	if endpoint, ok := ctrl.(interface {
		ApproveWithReceipt(string, bool, bool, bool) event.PromptReplyReceipt
	}); ok {
		receipt := endpoint.ApproveWithReceipt(id, allow, session, persist)
		if receipt.Status != event.PromptReplyNotFound || strings.TrimSpace(tabID) == "" {
			return eventwire.ToWirePromptReplyReceipt(receipt)
		}
	} else if ctrl != nil {
		ctrl.Approve(id, allow, session, persist)
		return eventwire.ToWirePromptReplyReceipt(event.PromptReplyReceipt{PromptID: id, Status: event.PromptReplyAccepted})
	}
	return eventwire.ToWirePromptReplyReceipt(a.missingPromptReceipt(tabID, id))
}

// ReplayPendingPrompts asks every tab's controller to re-emit any approval/ask
// prompt that is currently blocking its run loop. The frontend calls this once
// its event subscription is live (on load/reconnect) so a session that was
// already awaiting confirmation rebuilds its modal instead of showing a
// "waiting" status with no way to answer — and no way to stop.
func (a *App) ReplayPendingPrompts() {
	a.mu.RLock()
	tabs := a.runtimeTabsLocked()
	a.mu.RUnlock()
	for _, t := range tabs {
		if t.Ctrl != nil {
			t.Ctrl.ReplayPendingPrompts()
		}
	}
}

// SetPlanMode toggles the read-only plan axis while preserving the current
// tool-auto-approval axis.
func (a *App) SetPlanMode(on bool) {
	a.setPlanModeForTab("", on)
}

func (a *App) setPlanModeForTab(tabID string, on bool) {
	current := a.currentModeForTab(tabID)
	a.SetModeForTab(tabID, tabModeFromAxes(on, tabModeHasAutoApproveTools(current)))
}

// SetMode applies a composer gating mode ("plan" | "yolo" | "plan-yolo" |
// anything else =
// normal) in one call, so a turn submitted right after the switch can't race a
// half-applied plan/tool-auto-approval pair.
func (a *App) SetMode(mode string) {
	a.SetModeForTab("", mode)
}

func (a *App) SetModeForTab(tabID, mode string) {
	normalized := normalizeTabMode(mode)
	a.mu.Lock()
	tab := a.tabByIDLocked(tabID)
	if tab == nil {
		a.mu.Unlock()
		return
	}
	if !tab.Ready {
		a.mu.Unlock()
		return
	}
	tab.mode = normalized
	tab.toolApprovalMode = normalizeToolApprovalMode(tab.toolApprovalMode)
	if tabModeHasAutoApproveTools(normalized) {
		tab.toolApprovalMode = control.ToolApprovalYolo
	} else if tab.toolApprovalMode == control.ToolApprovalYolo {
		tab.toolApprovalMode = control.ToolApprovalAsk
	}
	ctrl := tab.Ctrl
	approvalMode := tab.toolApprovalMode
	tabIDForSave := tab.ID
	a.mu.Unlock()
	applyTabModeToController(ctrl, normalized)
	applyTabToolApprovalModeToController(ctrl, approvalMode)
	a.mu.Lock()
	if a.tabRuntimes[tabIDForSave] == tab {
		a.saveTabsLocked()
	}
	a.mu.Unlock()
}

func applyTabModeToController(ctrl control.SessionAPI, mode string) {
	if ctrl == nil {
		return
	}
	switch normalizeTabMode(mode) {
	case "plan":
		ctrl.SetMode(true, false)
	case "yolo":
		ctrl.SetMode(false, true)
	case "plan-yolo":
		ctrl.SetMode(true, true)
	default:
		ctrl.SetMode(false, false)
	}
}

func applyTabToolApprovalModeToController(ctrl control.SessionAPI, mode string) {
	if ctrl == nil {
		return
	}
	ctrl.SetToolApprovalMode(normalizeToolApprovalMode(mode))
}

func (a *App) currentModeForTab(tabID string) string {
	a.mu.RLock()
	tab := a.tabByIDLocked(tabID)
	mode := "normal"
	if tab != nil {
		mode = currentTabMode(tab)
	}
	a.mu.RUnlock()
	return mode
}

func normalizeCollaborationMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "plan":
		return "plan"
	case "goal":
		return "goal"
	default:
		return "normal"
	}
}

func (a *App) SetCollaborationMode(mode string) {
	a.SetCollaborationModeForTab("", mode)
}

func (a *App) SetCollaborationModeForTab(tabID, mode string) {
	mode = normalizeCollaborationMode(mode)
	a.mu.Lock()
	tab := a.tabByIDLocked(tabID)
	if tab == nil {
		a.mu.Unlock()
		return
	}
	approvalMode := currentTabToolApprovalMode(tab)
	switch mode {
	case "plan":
		tab.mode = tabModeFromAxes(true, approvalMode == control.ToolApprovalYolo)
		tab.goal = ""
	case "goal":
		tab.mode = tabModeFromAxes(false, approvalMode == control.ToolApprovalYolo)
	default:
		tab.mode = tabModeFromAxes(false, approvalMode == control.ToolApprovalYolo)
		tab.goal = ""
	}
	ctrl := tab.Ctrl
	goal := tab.goal
	plan := tabModeHasPlan(tab.mode)
	tabIDForSave := tab.ID
	a.mu.Unlock()
	if ctrl != nil {
		ctrl.SetPlanMode(plan)
		ctrl.SetGoal(goal)
	}
	a.mu.Lock()
	if a.tabRuntimes[tabIDForSave] == tab {
		a.saveTabsLocked()
	}
	a.mu.Unlock()
}

// QuestionAnswer is the frontend's reply to one question in an ask_request.
type QuestionAnswer struct {
	QuestionID string   `json:"questionId"`
	Selected   []string `json:"selected"`
}

// AnswerQuestion resolves a pending ask_request (the `ask` tool) by ID with the
// user's selections per question.
func (a *App) AnswerQuestion(id string, answers []QuestionAnswer) {
	a.AnswerQuestionForTab("", id, answers)
}

func (a *App) AnswerQuestionForTab(tabID, id string, answers []QuestionAnswer) eventwire.PromptReplyReceipt {
	ctrl := a.ctrlByTabID(tabID)
	out := make([]event.AskAnswer, len(answers))
	for i, an := range answers {
		out[i] = event.AskAnswer{QuestionID: an.QuestionID, Selected: an.Selected}
	}
	if endpoint, ok := ctrl.(interface {
		AnswerQuestionWithReceipt(string, []event.AskAnswer) event.PromptReplyReceipt
	}); ok {
		receipt := endpoint.AnswerQuestionWithReceipt(id, out)
		if receipt.Status != event.PromptReplyNotFound || strings.TrimSpace(tabID) == "" {
			return eventwire.ToWirePromptReplyReceipt(receipt)
		}
	} else if ctrl != nil {
		ctrl.AnswerQuestion(id, out)
		return eventwire.ToWirePromptReplyReceipt(event.PromptReplyReceipt{PromptID: id, Status: event.PromptReplyAccepted})
	}
	return eventwire.ToWirePromptReplyReceipt(a.missingPromptReceipt(tabID, id))
}

// RecordPromptReplyFailureForTab records a retryable adapter failure against
// the exact owning controller without consuming the prompt.
func (a *App) RecordPromptReplyFailureForTab(tabID, id string) eventwire.PromptReplyReceipt {
	ctrl := a.ctrlByTabID(tabID)
	if endpoint, ok := ctrl.(interface {
		RecordPromptReplyFailure(string) event.PromptReplyReceipt
	}); ok {
		receipt := endpoint.RecordPromptReplyFailure(id)
		if receipt.Status != event.PromptReplyNotFound || strings.TrimSpace(tabID) == "" {
			return eventwire.ToWirePromptReplyReceipt(receipt)
		}
	}
	return eventwire.ToWirePromptReplyReceipt(a.missingPromptReceipt(tabID, id))
}

func (a *App) missingPromptReceipt(requestedTabID, promptID string) event.PromptReplyReceipt {
	status := event.PromptReplyNotFound
	if strings.TrimSpace(requestedTabID) != "" {
		a.mu.RLock()
		tabs := append([]*WorkspaceTab(nil), a.runtimeTabsLocked()...)
		a.mu.RUnlock()
		for _, tab := range tabs {
			if tab == nil || tab.ID == requestedTabID || tab.Ctrl == nil {
				continue
			}
			if owner, ok := tab.Ctrl.(interface{ OwnsPrompt(string) bool }); ok && owner.OwnsPrompt(promptID) {
				status = event.PromptReplyWrongTab
				break
			}
		}
	}
	return event.PromptReplyReceipt{PromptID: promptID, Status: status}
}

// Compact runs one compaction pass on demand.
// Compact runs a plain compaction pass (the "compact now" button). Focus-guided
// compaction goes through Submit("/compact <focus>") instead.
func (a *App) Compact() error {
	a.mu.RLock()
	tab := a.activeTabLocked()
	ctrl := a.activeCtrlLocked()
	a.mu.RUnlock()
	if tab != nil && tab.ReadOnly {
		return readOnlyChannelErr()
	}
	if ctrl == nil {
		return nil
	}
	return ctrl.Compact(a.ctx, "")
}

func (a *App) ListSessionExperts() []expert.Info {
	a.mu.RLock()
	ctrl := a.activeCtrlLocked()
	a.mu.RUnlock()
	if ctrl == nil {
		return []expert.Info{}
	}
	return ctrl.ListExperts()
}

func (a *App) ListUserExperts() ([]expert.UserExpertInfo, error) {
	return a.ListUserExpertsForTab("")
}

func (a *App) ListUserExpertsForTab(tabID string) ([]expert.UserExpertInfo, error) {
	ctrl := a.ctrlByTabID(tabID)
	if a.usesCloudExpertAssets() {
		return a.listCloudUserExperts(ctrl)
	}
	if ctrl == nil {
		return []expert.UserExpertInfo{}, nil
	}
	return ctrl.ListUserExperts(), nil
}

func (a *App) ExpertLibraryState() (expert.ExpertLibraryState, error) {
	return a.ExpertLibraryStateForTab("")
}

func (a *App) ExpertLibraryStateForTab(tabID string) (expert.ExpertLibraryState, error) {
	ctrl := a.ctrlByTabID(tabID)
	if a.usesCloudExpertAssets() {
		return a.cloudExpertLibraryState(ctrl)
	}
	if ctrl == nil {
		return expert.ExpertLibraryState{}, nil
	}
	return ctrl.ExpertLibraryState(), nil
}

func (a *App) CreateUserExpert(input expert.UserExpertInput) (expert.UserExpertInfo, error) {
	a.mu.RLock()
	tab := a.activeTabLocked()
	ctrl := a.activeCtrlLocked()
	a.mu.RUnlock()
	if tab != nil && tab.ReadOnly {
		return expert.UserExpertInfo{}, readOnlyChannelErr()
	}
	if ctrl == nil {
		return expert.UserExpertInfo{}, workspaceNotReadyErr(tab)
	}
	runtimes := a.expertRuntimesFor(ctrl)
	if a.usesCloudExpertAssets() {
		created, err := a.createCloudUserExpert(ctrl, input)
		if err == nil {
			reloadExpertRuntimes(runtimes, ctrl)
		}
		return created, err
	}
	created, err := ctrl.CreateUserExpert(input)
	if err == nil {
		reloadExpertRuntimes(runtimes, ctrl)
	}
	return created, err
}

func (a *App) createCloudUserExpert(ctrl control.SessionAPI, input expert.UserExpertInput) (expert.UserExpertInfo, error) {
	store, err := desktopUserExpertStoreFor(ctrl)
	if err != nil {
		return expert.UserExpertInfo{}, err
	}
	avatarFileName, avatarContentType, avatar, err := decodeCloudExpertAvatar(input.AvatarKey, input.AvatarFileName)
	if err != nil {
		return expert.UserExpertInfo{}, err
	}
	runtime, scope, end, err := a.cloudExpertAssetsOperation()
	if err != nil {
		return expert.UserExpertInfo{}, err
	}
	defer end()
	var id string
	var createdAt time.Time
	if strings.TrimSpace(input.Mode) == string(expert.ModeTeam) {
		expertIDs := make([]string, 0, len(input.Members))
		for index, member := range input.Members {
			expertID := strings.TrimSpace(member.ExpertID)
			if expertID == "" {
				return expert.UserExpertInfo{}, fmt.Errorf("expert group member %d must reference an expert id", index+1)
			}
			expertIDs = append(expertIDs, expertID)
		}
		created, requestErr := runtime.expertAssets.CreateUserExpertGroup(a.bootContext(), scope, cloud.CreateUserExpertGroupAssetInput{
			AvatarFileName: avatarFileName, AvatarContentType: avatarContentType, Avatar: avatar,
			Name: input.Name, Description: input.Description, Category: input.Category,
			Tags: input.Tags, ExpertIDs: expertIDs,
		})
		if requestErr != nil {
			return expert.UserExpertInfo{}, cloudWailsError("cloud.createUserExpert", requestErr)
		}
		id, createdAt = created.ExpertGroupID, created.CreatedAt
		input.Name = created.Name
		input.Description = created.Description
		input.Category = created.Category
		input.Tags = append([]string(nil), created.Tags...)
		input.AvatarKey = created.AvatarURL
	} else {
		created, requestErr := runtime.expertAssets.CreateUserExpert(a.bootContext(), scope, cloud.CreateUserExpertAssetInput{
			AvatarFileName: avatarFileName, AvatarContentType: avatarContentType, Avatar: avatar,
			Name: input.Name, Description: input.Description, Prompt: input.SystemPrompt,
			Category: input.Category, Tags: input.Tags,
		})
		if requestErr != nil {
			return expert.UserExpertInfo{}, cloudWailsError("cloud.createUserExpert", requestErr)
		}
		id, createdAt = created.ExpertID, created.CreatedAt
		input.Name = created.Name
		input.Description = created.Description
		input.SystemPrompt = created.Prompt
		input.Category = created.Category
		input.Tags = append([]string(nil), created.Tags...)
		input.AvatarKey = created.AvatarURL
		input.Mode = string(expert.ModeExpert)
	}
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	record, err := expert.NewUserExpert(id, input, createdAt)
	if err != nil {
		return expert.UserExpertInfo{}, fmt.Errorf("save created cloud expert locally: %w", err)
	}
	path := store.UserExpertStorePath()
	records, err := expert.LoadUserExperts(path)
	if err != nil {
		return expert.UserExpertInfo{}, fmt.Errorf("load local expert mirror: %w", err)
	}
	for _, existing := range records {
		if existing.ID == record.ID {
			return expert.UserExpertInfo{}, fmt.Errorf("cloud expert %q already exists in the local mirror", record.ID)
		}
	}
	records = append([]expert.UserExpert{record}, records...)
	if err := expert.SaveUserExperts(path, records); err != nil {
		return expert.UserExpertInfo{}, fmt.Errorf("save local expert mirror: %w", err)
	}
	store.ReloadExpertCatalog()
	return record.Info(), nil
}

func decodeCloudExpertAvatar(value, fileName string) (string, string, []byte, error) {
	const maxAvatarBytes = 5 * 1024 * 1024
	value = strings.TrimSpace(value)
	if value == "" {
		return "default-expert.png", "image/png", append([]byte(nil), defaultUserExpertAvatar...), nil
	}
	metadata, encoded, found := strings.Cut(value, ",")
	if !found || !strings.HasPrefix(strings.ToLower(metadata), "data:image/") {
		return "", "", nil, fmt.Errorf("select a local PNG, JPEG, or WebP avatar")
	}
	contentType, encoding, found := strings.Cut(metadata[5:], ";")
	if !found || !strings.EqualFold(strings.TrimSpace(encoding), "base64") {
		return "", "", nil, fmt.Errorf("expert avatar must be a base64-encoded local image")
	}
	if base64.StdEncoding.DecodedLen(len(encoded)) > maxAvatarBytes+2 {
		return "", "", nil, fmt.Errorf("expert avatar file exceeds 5 MiB")
	}
	avatar, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", "", nil, fmt.Errorf("decode expert avatar: %w", err)
	}
	if len(avatar) == 0 || len(avatar) > maxAvatarBytes {
		return "", "", nil, fmt.Errorf("expert avatar file must contain between 1 byte and 5 MiB")
	}
	return strings.TrimSpace(fileName), strings.ToLower(strings.TrimSpace(contentType)), avatar, nil
}

func (a *App) UpdateUserExpert(id string, input expert.UserExpertInput) (expert.UserExpertInfo, error) {
	a.mu.RLock()
	tab := a.activeTabLocked()
	ctrl := a.activeCtrlLocked()
	a.mu.RUnlock()
	if tab != nil && tab.ReadOnly {
		return expert.UserExpertInfo{}, readOnlyChannelErr()
	}
	if ctrl == nil {
		return expert.UserExpertInfo{}, workspaceNotReadyErr(tab)
	}
	if a.usesCloudExpertAssets() && isCloudUserExpertAssetID(id) {
		return expert.UserExpertInfo{}, fmt.Errorf("cloud user experts cannot be updated because the service does not expose an update route")
	}
	runtimes := a.expertRuntimesFor(ctrl)
	if err := ensureExpertRuntimesIdle(runtimes, strings.TrimSpace(id), false); err != nil {
		return expert.UserExpertInfo{}, err
	}
	updated, err := ctrl.UpdateUserExpert(id, input)
	if err != nil {
		return expert.UserExpertInfo{}, err
	}
	if err := refreshExpertRuntimes(runtimes, strings.TrimSpace(id), ctrl); err != nil {
		return expert.UserExpertInfo{}, err
	}
	return updated, nil
}

func (a *App) DeleteUserExpert(id string) error {
	a.mu.RLock()
	tab := a.activeTabLocked()
	ctrl := a.activeCtrlLocked()
	a.mu.RUnlock()
	if tab != nil && tab.ReadOnly {
		return readOnlyChannelErr()
	}
	if ctrl == nil {
		return workspaceNotReadyErr(tab)
	}
	id = strings.TrimSpace(id)
	if a.usesCloudExpertAssets() && isCloudUserExpertAssetID(id) {
		return fmt.Errorf("cloud user experts cannot be deleted because the service does not expose a delete route")
	}
	runtimes := a.expertRuntimesFor(ctrl)
	if err := ensureExpertRuntimesIdle(runtimes, id, false); err != nil {
		return err
	}
	if err := clearExpertRuntimes(runtimes, id, false); err != nil {
		return err
	}
	if err := ctrl.DeleteUserExpert(id); err != nil {
		return err
	}
	reloadExpertRuntimes(runtimes, nil)
	return nil
}

func (a *App) DuplicateExpert(id string) (expert.UserExpertInfo, error) {
	a.mu.RLock()
	tab := a.activeTabLocked()
	ctrl := a.activeCtrlLocked()
	a.mu.RUnlock()
	if tab != nil && tab.ReadOnly {
		return expert.UserExpertInfo{}, readOnlyChannelErr()
	}
	if ctrl == nil {
		return expert.UserExpertInfo{}, workspaceNotReadyErr(tab)
	}
	runtimes := a.expertRuntimesFor(ctrl)
	if a.usesCloudExpertAssets() && isCloudUserExpertAssetID(id) {
		store, err := desktopUserExpertStoreFor(ctrl)
		if err != nil {
			return expert.UserExpertInfo{}, err
		}
		records, err := expert.LoadUserExperts(store.UserExpertStorePath())
		if err != nil {
			return expert.UserExpertInfo{}, err
		}
		for _, record := range records {
			if record.ID != strings.TrimSpace(id) {
				continue
			}
			duplicated, createErr := a.createCloudUserExpert(ctrl, expert.UserExpertInput{
				Name: record.Name + " Copy", Mode: record.Mode, Description: record.Description,
				Category: record.Category, Tags: record.Tags, AvatarKey: record.AvatarKey,
				Members: record.Members, SystemPrompt: record.SystemPrompt,
				DisplayPrompt: record.DisplayPrompt, Prompts: record.Prompts,
				Language: record.Language, Enabled: record.Enabled,
			})
			if createErr == nil {
				reloadExpertRuntimes(runtimes, ctrl)
			}
			return duplicated, createErr
		}
		return expert.UserExpertInfo{}, fmt.Errorf("unknown user expert %q", id)
	}
	duplicated, err := ctrl.DuplicateExpert(id)
	if err == nil {
		reloadExpertRuntimes(runtimes, ctrl)
	}
	return duplicated, err
}

func isCloudUserExpertAssetID(id string) bool {
	id = strings.TrimSpace(id)
	return strings.HasPrefix(id, "exp_") || strings.HasPrefix(id, "exg_")
}

func (a *App) SetExpertFavorite(id string, favorite bool) (expert.ExpertLibraryState, error) {
	a.mu.RLock()
	tab := a.activeTabLocked()
	ctrl := a.activeCtrlLocked()
	a.mu.RUnlock()
	if tab != nil && tab.ReadOnly {
		if ctrl == nil {
			return expert.ExpertLibraryState{}, readOnlyChannelErr()
		}
		return ctrl.ExpertLibraryState(), readOnlyChannelErr()
	}
	if ctrl == nil {
		return expert.ExpertLibraryState{}, workspaceNotReadyErr(tab)
	}
	if a.usesCloudExpertAssets() {
		return a.setCloudExpertFavorite(ctrl, id, favorite)
	}
	return ctrl.SetExpertFavorite(id, favorite)
}

func (a *App) ImportExpertConfig(payload string) (expert.UserExpertInput, error) {
	a.mu.RLock()
	ctrl := a.activeCtrlLocked()
	a.mu.RUnlock()
	if ctrl == nil {
		return expert.UserExpertInput{}, workspaceNotReadyErr(nil)
	}
	return ctrl.ImportExpertConfig(payload)
}

func (a *App) CurrentExpert() expert.State {
	return a.CurrentExpertForTab("")
}

func (a *App) CurrentExpertForTab(tabID string) expert.State {
	ctrl := a.ctrlByTabID(tabID)
	if ctrl == nil {
		return expert.State{Source: expert.SourceNone, CanSwitch: true}
	}
	return ctrl.CurrentExpert()
}

func (a *App) SetExpert(id string) (expert.State, error) {
	return a.SetExpertForTab("", id)
}

func (a *App) SetExpertForTab(tabID, id string) (expert.State, error) {
	tab, ctrl := a.tabAndCtrlByID(tabID)
	if tab != nil && tab.ReadOnly {
		return a.CurrentExpertForTab(tabID), readOnlyChannelErr()
	}
	if ctrl == nil {
		return expert.State{Source: expert.SourceNone, CanSwitch: true}, workspaceNotReadyErr(tab)
	}
	return ctrl.SetExpert(id)
}

func (a *App) AttachExpertForTab(tabID, id string) (expert.State, error) {
	tab, ctrl := a.tabAndCtrlByID(tabID)
	if tab != nil && tab.ReadOnly {
		return a.CurrentExpertForTab(tabID), readOnlyChannelErr()
	}
	if ctrl == nil {
		return expert.State{Source: expert.SourceNone, CanSwitch: true}, workspaceNotReadyErr(tab)
	}
	attacher, ok := ctrl.(interface {
		AttachExpert(string) (expert.State, error)
	})
	if !ok {
		return a.CurrentExpertForTab(tabID), fmt.Errorf("attach expert is unavailable")
	}
	return attacher.AttachExpert(id)
}

func (a *App) SetExpertGroup(ids []string) (expert.State, error) {
	return a.SetExpertGroupForTab("", ids)
}

func (a *App) SetExpertGroupForTab(tabID string, ids []string) (expert.State, error) {
	tab, ctrl := a.tabAndCtrlByID(tabID)
	if tab != nil && tab.ReadOnly {
		return a.CurrentExpertForTab(tabID), readOnlyChannelErr()
	}
	if ctrl == nil {
		return expert.State{}, workspaceNotReadyErr(tab)
	}
	return ctrl.SetExpertGroup(ids)
}

func (a *App) SetSceneForTab(tabID, value string) (TabMeta, error) {
	tab, ctrl := a.tabAndCtrlByID(tabID)
	if tab == nil {
		return TabMeta{}, fmt.Errorf("tab %q not found", tabID)
	}
	if tab.ReadOnly {
		a.mu.RLock()
		meta := a.tabMeta(tab, tab.ID == a.cachedActiveTabID())
		a.mu.RUnlock()
		return meta, readOnlyChannelErr()
	}
	setter, ok := ctrl.(interface{ SetDraftScene(scene.Scene) error })
	if !ok {
		a.mu.RLock()
		meta := a.tabMeta(tab, tab.ID == a.cachedActiveTabID())
		a.mu.RUnlock()
		return meta, workspaceNotReadyErr(tab)
	}
	next, err := scene.Parse(value)
	if err != nil {
		a.mu.RLock()
		meta := a.tabMeta(tab, tab.ID == a.cachedActiveTabID())
		a.mu.RUnlock()
		return meta, err
	}
	if err := setter.SetDraftScene(next); err != nil {
		a.mu.RLock()
		meta := a.tabMeta(tab, tab.ID == a.cachedActiveTabID())
		a.mu.RUnlock()
		return meta, err
	}
	a.mu.Lock()
	setTabSceneStatus(tab, scene.Status{Scene: next, State: scene.Draft})
	a.saveTabsLocked()
	meta := a.tabMeta(tab, tab.ID == a.cachedActiveTabID())
	a.mu.Unlock()
	return enrichTabMeta(meta), nil
}

func (a *App) ClearExpert() (expert.State, error) {
	return a.ClearExpertForTab("")
}

func (a *App) ClearExpertForTab(tabID string) (expert.State, error) {
	tab, ctrl := a.tabAndCtrlByID(tabID)
	if tab != nil && tab.ReadOnly {
		return a.CurrentExpertForTab(tabID), readOnlyChannelErr()
	}
	if ctrl == nil {
		return expert.State{Source: expert.SourceNone, CanSwitch: true}, nil
	}
	return ctrl.ClearExpert()
}

// workspaceNotReadyErr names why a session action arrived before the tab's
// controller existed: still starting, or failed to start. Silently returning
// nil here swallowed the click with no feedback (#3938).
func workspaceNotReadyErr(tab *WorkspaceTab) error {
	if tab != nil && strings.TrimSpace(tab.StartupErr) != "" {
		return fmt.Errorf("workspace failed to start: %s", tab.StartupErr)
	}
	if tab != nil && tab.subscriptionAuthPending {
		return &modelbilling.Error{Code: modelbilling.ErrorAuthRequired, Message: "sign in to use platform models"}
	}
	return fmt.Errorf("workspace is still starting")
}

func chatEntryPointFastFailApplies(input string) bool {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "!") {
		return false
	}
	if strings.HasPrefix(trimmed, "//") {
		return true
	}
	return !strings.HasPrefix(trimmed, "/")
}

func (a *App) chatEntryPointModelError(tab *WorkspaceTab, input string) error {
	if tab == nil || !chatEntryPointFastFailApplies(input) {
		return nil
	}
	cfg, err := a.desktopConfigForBillingRoute(tab.WorkspaceRoot, tabRuntimeActorContext(tab), modelbilling.BillingSource(tab.billingSource))
	if err != nil {
		return nil
	}
	ref := strings.TrimSpace(tab.model)
	if ref == "" {
		ref = cfg.DefaultModel
	}
	config.NormalizeLegacyMimoCustomProvidersForRefs(cfg, ref)
	entry, ok := cfg.ResolveModel(ref)
	if !ok || config.IsLikelyChatModel(entry.Model) {
		return nil
	}
	return fmt.Errorf("current model %q is not available in the chat entry point; switch to a chat model", entry.Name+"/"+entry.Model)
}

// NewSession snapshots the current conversation and rotates to a fresh one.
func (a *App) NewSession() error {
	a.mu.RLock()
	tab := a.activeTabLocked()
	ctrl := a.activeCtrlLocked()
	a.mu.RUnlock()
	if tab != nil && tab.ReadOnly {
		return readOnlyChannelErr()
	}
	if ctrl == nil {
		return workspaceNotReadyErr(tab)
	}
	if err := a.authorizeProjectSessionMutation(ctrl.SessionPath(), false); err != nil {
		return err
	}
	// Tab is already blank — just persist and skip the new-session dance.
	if !controllerHasActiveRuntimeWork(ctrl) && !messagesHaveConversationContent(ctrl.History()) && tabSceneStatus(tab) == newDesktopSceneStatus() {
		a.persistTabSessionPath(tab, ctrl.SessionPath())
		a.refreshTabSessionTurns(tab.ID)
		return nil
	}

	reservation, err := reserveFreshSessionTopic(tab)
	if err != nil {
		return err
	}
	if err := ctrl.NewSession(); err != nil {
		return err
	}
	syncTabSceneFromController(tab)
	a.applyFreshSessionTopic(tab, reservation)
	a.persistTabSessionPath(tab, ctrl.SessionPath())
	a.refreshTabSessionTurns(tab.ID)
	a.invalidatePromptHistoryCache()
	a.emitProjectTreeChanged()
	return nil
}

type freshSessionTopicReservation struct {
	topicID string
	title   string
}

func reserveFreshSessionTopic(tab *WorkspaceTab) (freshSessionTopicReservation, error) {
	if tab == nil {
		return freshSessionTopicReservation{}, fmt.Errorf("active tab is unavailable")
	}
	scope := tab.Scope
	workspaceRoot := tab.WorkspaceRoot
	if strings.TrimSpace(scope) == "global" {
		workspaceRoot = ""
	} else {
		workspaceRoot = normalizeProjectRoot(workspaceRoot)
	}
	topicID := newTopicID()
	if _, _, err := registerTopic(scope, workspaceRoot, topicID, defaultTopicTitle, topicTitleSourceAuto, time.Now().UnixMilli()); err != nil {
		return freshSessionTopicReservation{}, err
	}
	return freshSessionTopicReservation{topicID: topicID, title: defaultTopicTitle}, nil
}

func (a *App) applyFreshSessionTopic(tab *WorkspaceTab, reservation freshSessionTopicReservation) {
	if tab == nil {
		return
	}
	a.mu.Lock()
	if current := a.tabRuntimes[tab.ID]; current == tab {
		tab.TopicID = reservation.topicID
		tab.TopicTitle = reservation.title
		a.saveTabsLocked()
	} else {
		tab.TopicID = reservation.topicID
		tab.TopicTitle = reservation.title
	}
	a.mu.Unlock()
}

func messagesHaveConversationContent(messages []provider.Message) bool {
	for _, msg := range messages {
		if msg.Role != provider.RoleSystem {
			return true
		}
	}
	return false
}

// ClearSession discards the current conversation and rotates to a fresh unsaved one.
func (a *App) ClearSession() error {
	a.mu.RLock()
	tab := a.activeTabLocked()
	ctrl := a.activeCtrlLocked()
	a.mu.RUnlock()
	if tab != nil && tab.ReadOnly {
		return readOnlyChannelErr()
	}
	if ctrl == nil {
		return workspaceNotReadyErr(tab)
	}
	if err := a.authorizeProjectSessionMutation(ctrl.SessionPath(), false); err != nil {
		return err
	}
	if controllerHasActiveRuntimeWork(ctrl) {
		return a.clearActiveSessionRuntime(tab, ctrl)
	}
	if err := ctrl.ClearSession(); err != nil {
		return err
	}
	syncTabSceneFromController(tab)
	tab.resetTelemetry()
	a.persistTabSessionPath(tab, ctrl.SessionPath())
	a.refreshTabSessionTurns(tab.ID)
	a.invalidatePromptHistoryCache()
	return nil
}

func (a *App) clearActiveSessionRuntime(tab *WorkspaceTab, oldCtrl control.SessionAPI) error {
	if tab == nil || oldCtrl == nil {
		return fmt.Errorf("workspace is still starting")
	}
	oldPath := oldCtrl.SessionPath()
	actor := oldCtrl.ActorContext()
	oldSink := tab.sink
	if oldSink != nil {
		if !oldSink.beginRouteChangeWithin(runtimeAttachWaitTimeout) {
			return fmt.Errorf("session event route is busy; retry clear")
		}
		a.mu.Lock()
		validRuntime := !tab.removed && a.tabRuntimes[tab.ID] == tab && tab.Ctrl == oldCtrl && tab.sink == oldSink
		if !validRuntime {
			a.mu.Unlock()
			oldSink.endRouteChange()
			return fmt.Errorf("session runtime changed while clearing; retry")
		}
		oldSink.setTabID(detachedRuntimeTabID(oldPath))
		a.mu.Unlock()
		oldSink.clearContext()
		oldSink.endRouteChange()
	}
	oldStatus := oldCtrl.RuntimeStatus()
	if oldStatus.Cancellable {
		oldCtrl.Cancel()
	}
	if oldStatus.Running || oldStatus.Completing {
		if err := waitControllerStopped(oldCtrl); err != nil {
			return err
		}
	}
	destroy := oldCtrl.BeginDestroySession(oldPath)
	destroys := []control.SessionDestroyHandle{destroy}
	teardownTimedOut := waitDestroyHandles(destroys)
	if teardownTimedOut {
		if err := agent.MarkCleanupPending(oldPath, "clear"); err != nil {
			return err
		}
	}

	newSink := &tabEventSink{tabID: tab.ID, app: a, ctx: a.ctx}
	sharedHost := a.lookupSharedHost(tab.SharedHostKey)
	expertID := currentExpertID(oldCtrl)
	bootOptions := boot.Options{
		Model:                    tab.model,
		RequireKey:               false,
		Sink:                     newSink,
		WorkspaceRoot:            tab.WorkspaceRoot,
		SessionDir:               tabSessionDir(tab),
		EffortOverride:           cloneStringPtr(tab.effort),
		TokenMode:                currentTabTokenMode(tab),
		SharedHost:               sharedHost,
		CleanupPendingReconciler: reconcileDesktopCleanupPending,
		Actor:                    actor,
		ExpertID:                 expertID,
	}
	defaultScene := newDesktopSceneStatus()
	bootOptions.Scene = defaultScene.Scene
	bootOptions.SceneState = defaultScene.State
	newCtrl, err := a.buildDesktopController(a.bootContext(), bootOptions, actor, tab.WorkspaceRoot, tab.model, tab.billingSource)
	if err != nil {
		if teardownTimedOut {
			// The old session was already marked cleanup-pending, so finish the
			// destroy cleanup instead of re-exposing a runtime in teardown.
			go delayedDesktopSessionCleanup(oldPath, destroys)
		} else {
			finishDestroyHandles(destroys)
		}
		if oldSink != nil {
			if !oldSink.beginRouteChangeWithin(runtimeAttachWaitTimeout) {
				return errors.Join(err, fmt.Errorf("restore session event route: timed out waiting for in-flight delivery"))
			}
			a.mu.Lock()
			validRuntime := !tab.removed && a.tabRuntimes[tab.ID] == tab && tab.Ctrl == oldCtrl && tab.sink == oldSink
			if !validRuntime {
				a.mu.Unlock()
				oldSink.endRouteChange()
				return errors.Join(err, fmt.Errorf("restore session event route: runtime changed while clearing"))
			}
			oldSink.setTabID(tab.ID)
			a.mu.Unlock()
			oldSink.setContext(a.ctx)
			oldSink.endRouteChange()
		}
		return err
	}
	if teardownTimedOut {
		go delayedDesktopSessionCleanup(oldPath, destroys)
	} else {
		if err := removeDesktopSessionArtifacts(oldPath); err != nil {
			finishDestroyHandles(destroys)
			newCtrl.Close()
			return err
		}
		finishDestroyHandles(destroys)
	}
	a.bindControllerDisplayRecorder(newCtrl)
	newCtrl.EnableInteractiveApproval()
	applyTabModeToController(newCtrl, tab.mode)
	applyTabToolApprovalModeToController(newCtrl, tab.toolApprovalMode)
	newCtrl.SetGoal(tab.goal)
	path := agent.NewSessionPath(newCtrl.SessionDir(), newCtrl.Label())
	newCtrl.SetSessionPath(path)

	a.mu.Lock()
	if current := a.tabRuntimes[tab.ID]; current == tab {
		tab.Ctrl = newCtrl
		tab.sessionTurns = intPointer(0)
		syncTabSceneFromController(tab)
		tab.sink = newSink
		tab.SessionPath = path
		tab.Label = newCtrl.Label()
		tab.Ready = true
		clearTabStartupFailureLocked(tab)
		a.saveTabsLocked()
	}
	a.mu.Unlock()
	oldCtrl.CloseAfterDestroy()
	a.emitProjectTreeChanged()
	return nil
}

func removeDesktopSessionArtifacts(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	defer invalidateTopicSessionIndexForPath(path)
	for _, p := range sessionOwnedArtifactPaths(path) {
		if strings.TrimSpace(p) == "" {
			continue
		}
		if err := os.RemoveAll(p); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if err := agent.DeleteSubagentsByParent(filepath.Dir(path), agent.BranchID(path)); err != nil {
		return err
	}
	return agent.ClearCleanupPending(path)
}

var saveDesktopForkBranchMeta = agent.SaveBranchMeta
var removeDesktopForkSessionArtifacts = removeFailedDesktopForkSessionArtifacts

func removeFailedDesktopForkSessionArtifacts(path string) error {
	err := removeDesktopSessionArtifacts(path)
	for _, ownedPath := range []string{store.SessionExpert(path), guardian.PathFor(path), guardian.CursorPathFor(path)} {
		if strings.TrimSpace(ownedPath) == "" {
			continue
		}
		if removeErr := os.Remove(ownedPath); removeErr != nil && !os.IsNotExist(removeErr) {
			err = errors.Join(err, removeErr)
		}
	}
	return err
}

func cleanupFailedDesktopFork(sessionDir, sourcePath, forkPath string, cause error) error {
	canonicalPath, _, err := validateSessionPath(sessionDir, forkPath)
	if err != nil {
		return errors.Join(cause, fmt.Errorf("validate fork artifact path %q: %w", forkPath, err))
	}
	if sameDesktopPath(sourcePath, canonicalPath) {
		return errors.Join(cause, fmt.Errorf("refusing to clean source session as failed fork artifact: %s", canonicalPath))
	}
	forkErr := fmt.Errorf("finalize fork topic metadata %s: %w", canonicalPath, cause)
	if cleanupErr := removeDesktopForkSessionArtifacts(canonicalPath); cleanupErr != nil {
		return errors.Join(forkErr, fmt.Errorf("cleanup fork session artifacts %s: %w", canonicalPath, cleanupErr))
	}
	return forkErr
}

// CheckpointMeta summarises one rewind point (a user turn) for the desktop.
type CheckpointMeta struct {
	Turn            int      `json:"turn"`
	Prompt          string   `json:"prompt"`
	Files           []string `json:"files"`         // cumulative files RestoreCode would affect from this turn
	TurnFileCount   int      `json:"turnFileCount"` // files changed during this turn only
	Time            int64    `json:"time"`          // unix milliseconds
	CanCode         bool     `json:"canCode"`
	CanConversation bool     `json:"canConversation"`
}

// Checkpoints lists the session's rewind points, oldest first, for the rewind UI.
func (a *App) Checkpoints() []CheckpointMeta {
	return a.CheckpointsForTab("")
}

func (a *App) CheckpointsForTab(tabID string) []CheckpointMeta {
	a.mu.RLock()
	var ctrl control.SessionAPI
	if tab := a.tabByIDLocked(tabID); tab != nil {
		ctrl = tab.Ctrl
	}
	a.mu.RUnlock()
	if ctrl == nil {
		return []CheckpointMeta{}
	}
	metas := ctrl.Checkpoints()
	out := make([]CheckpointMeta, 0, len(metas))
	for _, m := range metas {
		out = append(out, CheckpointMeta{
			Turn:            m.Turn,
			Prompt:          m.Prompt,
			Files:           m.Paths,
			TurnFileCount:   len(m.Paths),
			Time:            m.Time.UnixMilli(),
			CanCode:         len(m.Paths) > 0,
			CanConversation: ctrl.CheckpointHasBoundary(m.Turn),
		})
	}
	// RestoreCode(turn) reverts every file touched in this turn or any later one, so
	// a turn can rewind code even when it changed no files itself — as long as a
	// later turn did. Propagate CanCode backwards over the oldest-first list.
	// Also propagate the cumulative unique file count so the UI shows how many
	// files RestoreCode would actually affect from this turn.
	hasCodeAfter := false
	codeFileSet := make(map[string]bool, len(metas)*2)
	for i := len(out) - 1; i >= 0; i-- {
		if len(out[i].Files) > 0 {
			hasCodeAfter = true
		}
		for _, f := range out[i].Files {
			codeFileSet[f] = true
		}
		out[i].CanCode = hasCodeAfter
		out[i].Files = make([]string, 0, len(codeFileSet))
		for f := range codeFileSet {
			out[i].Files = append(out[i].Files, f)
		}
		sort.Strings(out[i].Files) // map iteration is unordered; keep the list stable
	}
	return out
}

// ToolResultForTab returns the full arguments and output for one tool call that
// were elided from the frontend's in-memory items[] for memory efficiency. The
// caller (frontend ToolCard) loads this on demand when the user expands a
// collapsed tool card. Returns nil when the tool ID is not found.
func (a *App) ToolResultForTab(tabID, toolID string) *control.ToolResultData {
	a.mu.RLock()
	tab := a.tabByIDLocked(tabID)
	if tab == nil {
		a.mu.RUnlock()
		return nil
	}
	var ctrl control.SessionAPI
	ctrl = tab.Ctrl
	sessionPath := strings.TrimSpace(tab.SessionPath)
	if ctrl != nil {
		sessionPath = strings.TrimSpace(ctrl.SessionPath())
	}
	a.mu.RUnlock()
	if ctrl != nil {
		if result := ctrl.ToolResult(toolID); result != nil {
			return result
		}
	}
	if sessionPath == "" {
		return nil
	}
	_, validPath, err := resolveTabSessionPath(tab, sessionPath)
	if err != nil || agent.IsCleanupPending(validPath) {
		return nil
	}
	loaded, err := agent.LoadSession(validPath)
	if err != nil {
		return nil
	}
	return control.ToolResultFromMessages(loaded.Snapshot(), toolID)
}

// Rewind restores the session to the start of turn. scope is "code",
// "conversation", or "both" (anything else is treated as "both"). The frontend
// re-reads History after this resolves.
func (a *App) Rewind(turn int, scope string) error {
	a.mu.RLock()
	tab := a.activeTabLocked()
	ctrl := a.activeCtrlLocked()
	a.mu.RUnlock()
	if tab != nil && tab.ReadOnly {
		return readOnlyChannelErr()
	}
	if ctrl == nil {
		return nil
	}
	if err := a.authorizeProjectSessionMutation(ctrl.SessionPath(), false); err != nil {
		return err
	}
	s := control.RewindBoth
	switch scope {
	case "code":
		s = control.RewindCode
	case "conversation":
		s = control.RewindConversation
	}
	if err := ctrl.Rewind(turn, s); err != nil {
		return err
	}
	if s != control.RewindCode {
		if err := resetControllerTranscript(ctrl); err != nil {
			return err
		}
	}
	a.refreshTabSessionTurns(tab.ID)
	return nil
}

// Fork branches the conversation at the start of turn into a new session tab
// (preserving the current tab), keeping code intact, and switches to the new tab.
func (a *App) Fork(turn int) (TabMeta, error) {
	a.mu.RLock()
	sourceTab := a.activeTabLocked()
	ctrl := a.activeCtrlLocked()
	if sourceTab == nil || ctrl == nil {
		a.mu.RUnlock()
		return TabMeta{}, nil
	}
	if sourceTab.ReadOnly {
		a.mu.RUnlock()
		return TabMeta{}, readOnlyChannelErr()
	}
	scope := sourceTab.Scope
	workspaceRoot := sourceTab.WorkspaceRoot
	sourceTitle := sourceTab.TopicTitle
	model := sourceTab.model
	billingSource := sourceTab.billingSource
	effort := cloneStringPtr(sourceTab.effort)
	mode := currentTabMode(sourceTab)
	toolApprovalMode := currentTabToolApprovalMode(sourceTab)
	toolApprovalModeKnown := sourceTab.toolApprovalModeKnown
	disabledMCP := cloneServerViewMap(sourceTab.disabledMCP)
	mcpOrder := append([]string(nil), sourceTab.mcpOrder...)
	actor := tabRuntimeActorContext(sourceTab)
	sourcePath := sourceTab.currentSessionPath()
	a.mu.RUnlock()
	if err := a.authorizeProjectSessionMutation(sourcePath, false); err != nil {
		return TabMeta{}, err
	}
	if strings.TrimSpace(sourcePath) != "" {
		if err := saveTabSessionMeta(sourceTab, sourcePath); err != nil {
			return TabMeta{}, err
		}
	}

	topicID := newTopicID()
	topicTitle := forkTopicTitle(sourceTitle)
	if _, _, err := registerTopic(scope, workspaceRoot, topicID, topicTitle, topicTitleSourceManual, time.Now().UnixMilli()); err != nil {
		return TabMeta{}, err
	}
	sessionDir := ctrl.SessionDir()
	newPath, err := ctrl.ForkSession(turn, "")
	if err != nil {
		return TabMeta{}, err
	}
	m, err := agent.EnsureBranchMeta(newPath)
	if err != nil {
		return TabMeta{}, cleanupFailedDesktopFork(sessionDir, sourcePath, newPath, err)
	}
	m.Scope = scope
	m.WorkspaceRoot = workspaceRoot
	m.TopicID = topicID
	m.TopicTitle = topicTitle
	if err := saveDesktopForkBranchMeta(newPath, m); err != nil {
		return TabMeta{}, cleanupFailedDesktopFork(sessionDir, sourcePath, newPath, err)
	}
	forkScene, sceneErr := m.SceneStatus()
	if sceneErr != nil {
		slog.Warn("invalid fork scene metadata; using legacy compatibility", diagnostics.FailureMarkerKey, diagnostics.FailureMarker, "scope", "session.recovery", "session_path", newPath, "err", sceneErr)
		forkScene = legacyDesktopSceneStatus()
	}
	invalidateTopicSessionIndexForPath(newPath)

	a.mu.Lock()
	tabID := a.newUniqueTabIDLocked()
	tab := &WorkspaceTab{
		ID:                    tabID,
		Scope:                 scope,
		WorkspaceRoot:         workspaceRoot,
		TopicID:               topicID,
		TopicTitle:            topicTitle,
		SessionPath:           newPath,
		Scene:                 forkScene.Scene,
		SceneState:            forkScene.State,
		model:                 model,
		billingSource:         billingSource,
		effort:                effort,
		mode:                  mode,
		toolApprovalMode:      toolApprovalMode,
		toolApprovalModeKnown: toolApprovalModeKnown,
		disabledMCP:           disabledMCP,
		mcpOrder:              mcpOrder,
		actor:                 actor,
	}
	tab.sink = &tabEventSink{tabID: tabID, app: a}
	a.tabRuntimes[tabID] = tab
	a.tabOrder = append(a.tabOrder, tabID)
	a.saveTabsLocked()
	a.mu.Unlock()

	meta, err := a.activateTabMetaForOwnerTransition(tabID)
	if err != nil {
		return TabMeta{}, err
	}
	a.emitProjectTreeChanged()
	a.startTabControllerBuild(tab)
	return meta, nil
}

// SummarizeFrom / SummarizeUpTo compress the conversation from / up to the start
// of turn into one summary (Claude Code's "summarize from/up to here"), keeping
// code intact. The frontend re-reads History after this resolves.
func (a *App) SummarizeFrom(turn int) error {
	a.mu.RLock()
	tab := a.activeTabLocked()
	ctrl := a.activeCtrlLocked()
	a.mu.RUnlock()
	if tab != nil && tab.ReadOnly {
		return readOnlyChannelErr()
	}
	if ctrl == nil {
		return nil
	}
	if err := ctrl.SummarizeFrom(a.ctx, turn); err != nil {
		return err
	}
	if err := resetControllerTranscript(ctrl); err != nil {
		return err
	}
	a.refreshTabSessionTurns(tab.ID)
	return nil
}

func (a *App) SummarizeUpTo(turn int) error {
	a.mu.RLock()
	tab := a.activeTabLocked()
	ctrl := a.activeCtrlLocked()
	a.mu.RUnlock()
	if tab != nil && tab.ReadOnly {
		return readOnlyChannelErr()
	}
	if ctrl == nil {
		return nil
	}
	if err := ctrl.SummarizeUpTo(a.ctx, turn); err != nil {
		return err
	}
	if err := resetControllerTranscript(ctrl); err != nil {
		return err
	}
	a.refreshTabSessionTurns(tab.ID)
	return nil
}

// SessionMeta summarises one saved session for the history panel.
type SessionMeta struct {
	Path                  string  `json:"path"`
	Preview               string  `json:"preview"`         // first user message
	Title                 string  `json:"title,omitempty"` // user-chosen name, when set (overrides preview)
	Turns                 int     `json:"turns"`
	CreatedAt             int64   `json:"createdAt"`      // unix milliseconds
	LastActivityAt        int64   `json:"lastActivityAt"` // unix milliseconds
	ModTime               int64   `json:"modTime"`        // compatibility alias for lastActivityAt
	DeletedAt             int64   `json:"deletedAt,omitempty"`
	Current               bool    `json:"current"`
	Open                  bool    `json:"open"`
	Scope                 string  `json:"scope,omitempty"`
	WorkspaceRoot         string  `json:"workspaceRoot,omitempty"`
	ExpertID              string  `json:"expertId,omitempty"`
	ExpertName            string  `json:"expertName,omitempty"`
	ModelRef              string  `json:"modelRef,omitempty"`
	BillingSource         string  `json:"billingSource,omitempty"`
	Effort                *string `json:"effort,omitempty"`
	CollaborationMode     string  `json:"collaborationMode,omitempty"`
	ToolApprovalMode      string  `json:"toolApprovalMode,omitempty"`
	TokenMode             string  `json:"tokenMode,omitempty"`
	Goal                  string  `json:"goal,omitempty"`
	GoalStatus            string  `json:"goalStatus,omitempty"`
	TopicID               string  `json:"topicId,omitempty"`
	TopicTitle            string  `json:"topicTitle,omitempty"`
	Kind                  string  `json:"kind,omitempty"` // "channel" for external IM transcripts
	Channel               string  `json:"channel,omitempty"`
	ChannelLabel          string  `json:"channelLabel,omitempty"`
	RemoteID              string  `json:"remoteId,omitempty"`
	ChatType              string  `json:"chatType,omitempty"`
	UserID                string  `json:"userId,omitempty"`
	ThreadID              string  `json:"threadId,omitempty"`
	SessionSource         string  `json:"sessionSource,omitempty"`
	ProjectReadOnly       bool    `json:"projectReadOnly,omitempty"`
	ProjectReadOnlyReason string  `json:"projectReadOnlyReason,omitempty"`
}

type channelSessionRoute struct {
	channel       string
	channelLabel  string
	remoteID      string
	chatType      string
	userID        string
	threadID      string
	sessionSource string
}

type WorkspaceMeta struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	Current bool   `json:"current"`
}

func controllerSessionDir(ctrl control.SessionAPI) string {
	if ctrl != nil {
		if dir := ctrl.SessionDir(); dir != "" {
			return dir
		}
		return desktopSessionDirForActor(ctrl.ActorContext(), ctrl.WorkspaceRoot())
	}
	return desktopSessionDir("")
}

func tabSessionDir(tab *WorkspaceTab) string {
	if tab != nil {
		if actorContextSpecified(tab.actor) && (tab.Ctrl == nil || !actorProfilesMatch(tab.Ctrl.ActorContext(), tab.actor)) {
			return desktopSessionDirForActor(tab.actor, tab.WorkspaceRoot)
		}
		if tab.Ctrl != nil {
			if dir := tab.Ctrl.SessionDir(); dir != "" {
				return dir
			}
			return desktopSessionDirForActor(tab.Ctrl.ActorContext(), tab.Ctrl.WorkspaceRoot())
		}
		return desktopSessionDirForActor(tab.actor, tab.WorkspaceRoot)
	}
	return desktopSessionDir("")
}

func (a *App) activeSessionDir() string {
	a.mu.RLock()
	tab := a.activeTabLocked()
	dir := tabSessionDir(tab)
	a.mu.RUnlock()
	return dir
}

func (a *App) activeProfileSessionDirs() []string {
	a.mu.RLock()
	actor := a.currentActorContextLocked()
	if !a.currentActorExplicit {
		if tab := a.activeTabLocked(); tab != nil {
			actor = tabRuntimeActorContext(tab)
		}
	}
	a.mu.RUnlock()
	return a.sessionDirsForActorProfile(actor)
}

func (a *App) sessionDirsForActorProfile(actor userctx.ActorContext) []string {
	actor = actor.Normalize()
	seen := map[string]bool{}
	out := []string{}
	add := func(dir string) {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			return
		}
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
		if seen[dir] {
			return
		}
		seen[dir] = true
		out = append(out, dir)
	}
	if actor.ProfileID == userctx.DefaultProfileID {
		add(config.SessionDir()) // legacy/global sessions from earlier desktop builds
	}
	add(desktopSessionDirForActor(actor, ""))
	add(desktopSessionDirForActor(actor, globalWorkspaceRoot()))
	for _, project := range loadProjectsFile().Projects {
		dir := desktopSessionDirForActor(actor, project.Root)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue // project dir removed or external volume unmounted
		}
		add(dir)
	}
	a.mu.RLock()
	for _, tab := range a.runtimeTabsLocked() {
		if tab == nil || !actorProfilesMatch(tabRuntimeActorContext(tab), actor) {
			continue
		}
		add(tabSessionDir(tab))
	}
	a.mu.RUnlock()
	return out
}

func (a *App) activeTrashSessionDirs() []string {
	a.mu.RLock()
	tab := a.activeTabLocked()
	dir := tabSessionDir(tab)
	actor := userctx.ActorContext{}.Normalize()
	if tab != nil {
		actor = tabRuntimeActorContext(tab)
	}
	a.mu.RUnlock()

	seen := map[string]bool{}
	out := []string{}
	add := func(dir string) {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			return
		}
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
		if seen[dir] {
			return
		}
		seen[dir] = true
		out = append(out, dir)
	}
	add(dir)
	if actor.ProfileID == userctx.DefaultProfileID {
		add(config.SessionDir())
	}
	return out
}

// ListSessions returns the saved sessions newest-first for the history panel,
// marking the one the current conversation is writing to and attaching any
// user-chosen titles.
func (a *App) ListSessions() []SessionMeta {
	return a.listSessionsInDir(a.activeSessionDir())
}

func (a *App) listSessionsInDir(dir string) []SessionMeta {
	infos, err := a.listSessionInfos(dir)
	if err != nil {
		return []SessionMeta{}
	}
	titles := loadSessionTitles(dir)
	channelRoutes := a.channelSessionRoutesForDir(dir)
	open := a.openSessionPaths(dir)
	runtimeIdentities := a.runtimeSessionComposerIdentities(dir)
	active := a.activeSessionPath(dir)
	a.mu.RLock()
	activeTab := a.activeTabLocked()
	actor := userctx.ActorContext{}.Normalize()
	if activeTab != nil {
		actor = tabRuntimeActorContext(activeTab).Normalize()
	}
	a.mu.RUnlock()
	out := make([]SessionMeta, 0, len(infos))
	for _, s := range infos {
		visible, readOnly, reason := a.projectSessionListVisible(a.bootContext(), actor, s.Path)
		if !visible {
			continue
		}
		_, isOpen := open[s.Path]
		meta := sessionMetaFromInfo(s, titles[filepath.Base(s.Path)], s.Path == active, isOpen, 0)
		meta.ProjectReadOnly, meta.ProjectReadOnlyReason = readOnly, reason
		if identity, ok := runtimeIdentities[sessionRuntimeKey(s.Path)]; ok {
			applyComposerIdentityToSessionMeta(&meta, identity)
		}
		if route, ok := channelRoutes[sessionRuntimeKey(s.Path)]; ok {
			applyChannelSessionRoute(&meta, route)
		}
		out = append(out, meta)
	}
	return out
}

func (a *App) listSessionInfos(dir string) ([]agent.SessionInfo, error) {
	infos, pending, err := agent.ListSessionsContext(a.bootContext(), dir)
	if pending {
		a.scheduleSessionCatalogRefresh(dir)
	}
	if err != nil {
		return nil, err
	}
	a.mu.RLock()
	activeTab := a.activeTabLocked()
	actor := userctx.ActorContext{}.Normalize()
	if activeTab != nil {
		actor = tabRuntimeActorContext(activeTab).Normalize()
	}
	a.mu.RUnlock()
	visibleInfos := make([]agent.SessionInfo, 0, len(infos))
	for _, info := range infos {
		visible, _, _ := a.projectSessionListVisible(a.bootContext(), actor, info.Path)
		if visible {
			visibleInfos = append(visibleInfos, info)
		}
	}
	return visibleInfos, nil
}

func (a *App) scheduleSessionCatalogRefresh(dir string) {
	key := filepath.Clean(dir)
	if _, loaded := a.sessionCatalogRefreshes.LoadOrStore(key, struct{}{}); loaded {
		return
	}
	go func() {
		defer a.sessionCatalogRefreshes.Delete(key)
		if err := agent.WaitSessionCatalogRepair(a.bootContext(), key); err == nil {
			a.emitProjectTreeChanged()
		}
	}()
}

// ListAllSessions returns every saved session visible to the active profile,
// across Global and all configured workspaces. The scoped ListSessions method
// remains unchanged for surfaces that intentionally follow the active tab.
func (a *App) ListAllSessions() []SessionMeta {
	dirs := a.activeProfileSessionDirs()
	out := make([]SessionMeta, 0)
	seen := map[string]bool{}
	for _, dir := range dirs {
		for _, session := range a.listSessionsInDir(dir) {
			key := sessionRuntimeKey(session.Path)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, session)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].LastActivityAt != out[j].LastActivityAt {
			return out[i].LastActivityAt > out[j].LastActivityAt
		}
		if out[i].CreatedAt != out[j].CreatedAt {
			return out[i].CreatedAt > out[j].CreatedAt
		}
		return out[i].Path < out[j].Path
	})
	return out
}

// ListTrashedSessions returns sessions that were moved to the local trash,
// newest-deleted first. These can be previewed, restored, or permanently purged.
func (a *App) ListTrashedSessions() []SessionMeta {
	return a.listTrashedSessionsInDirs(a.activeTrashSessionDirs())
}

// ListAllTrashedSessions returns trashed sessions from every session directory
// visible to the active profile.
func (a *App) ListAllTrashedSessions() []SessionMeta {
	return a.listTrashedSessionsInDirs(a.activeProfileSessionDirs())
}

func (a *App) listTrashedSessionsInDirs(dirs []string) []SessionMeta {
	out := []SessionMeta{}
	seen := map[string]bool{}
	actor := a.activeProjectPolicyActor()
	for _, dir := range dirs {
		paths, err := listTrashedSessionFiles(dir)
		if err != nil {
			continue
		}
		titles := loadSessionTitles(dir)
		for _, path := range paths {
			visible, readOnly, reason := a.projectSessionListVisible(a.bootContext(), actor, path)
			if !visible {
				continue
			}
			infos, err := agent.ListSessions(filepath.Dir(path))
			if err != nil || len(infos) == 0 {
				continue
			}
			key := sessionRuntimeKey(path)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			var matched *agent.SessionInfo
			for index := range infos {
				if sessionRuntimeKey(infos[index].Path) == key {
					matched = &infos[index]
					break
				}
			}
			if matched == nil {
				continue
			}
			deletedAt := trashedSessionDeletedAt(path)
			meta := sessionMetaFromInfo(*matched, titles[filepath.Base(path)], false, false, deletedAt)
			meta.ProjectReadOnly, meta.ProjectReadOnlyReason = readOnly, reason
			out = append(out, meta)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].DeletedAt == out[j].DeletedAt {
			return out[i].LastActivityAt > out[j].LastActivityAt
		}
		return out[i].DeletedAt > out[j].DeletedAt
	})
	return out
}

func (a *App) trashedSessionDir(path string) (string, error) {
	dirs := append([]string{a.activeSessionDir()}, a.activeProfileSessionDirs()...)
	for _, dir := range uniqueSessionDirs(dirs) {
		if _, _, _, err := validateTrashedSessionPath(dir, path); err == nil {
			return dir, nil
		}
	}
	return "", fmt.Errorf("trashed session path outside active profile session dirs: %s", path)
}

func (a *App) sessionDirForPath(path string) (string, string, error) {
	// Keep basename-based callers scoped to the active session directory while
	// allowing the conversation center's absolute paths to resolve globally.
	dirs := append([]string{a.activeSessionDir()}, a.activeProfileSessionDirs()...)
	for _, dir := range uniqueSessionDirs(dirs) {
		sessionPath, _, err := validateSessionPath(dir, path)
		if err == nil {
			return dir, sessionPath, nil
		}
	}
	return "", "", fmt.Errorf("session path outside active profile session dirs: %s", path)
}

func uniqueSessionDirs(dirs []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
		if seen[dir] {
			continue
		}
		seen[dir] = true
		out = append(out, dir)
	}
	return out
}

func sessionMetaFromInfo(s agent.SessionInfo, title string, current, open bool, deletedAt int64) SessionMeta {
	meta := SessionMeta{
		Path:           s.Path,
		Preview:        s.Preview,
		Title:          title,
		Turns:          s.Turns,
		CreatedAt:      s.CreatedAt.UnixMilli(),
		LastActivityAt: s.LastActivityAt.UnixMilli(),
		ModTime:        s.LastActivityAt.UnixMilli(),
		DeletedAt:      deletedAt,
		Current:        current,
		Open:           open,
		Scope:          s.Scope,
		WorkspaceRoot:  s.WorkspaceRoot,
		ExpertID:       s.ExpertID,
		ExpertName:     s.ExpertName,
		TopicID:        s.TopicID,
		TopicTitle:     s.TopicTitle,
	}
	applyComposerIdentityToSessionMeta(&meta, storedSessionComposerIdentityView(s))
	return meta
}

func (a *App) runtimeSessionComposerIdentities(dir string) map[string]composerIdentityView {
	a.mu.RLock()
	entries := make([]struct {
		path string
		view composerIdentityView
	}, 0, len(a.tabRuntimes)+len(a.detachedSessions))
	for _, tab := range a.runtimeTabsLocked() {
		if tab == nil {
			continue
		}
		entries = append(entries, struct {
			path string
			view composerIdentityView
		}{path: tab.currentSessionPath(), view: runtimeComposerIdentityView(tab, defaultComposerIdentityView())})
	}
	a.mu.RUnlock()

	out := make(map[string]composerIdentityView, len(entries))
	for _, entry := range entries {
		path, _, err := validateSessionPath(dir, entry.path)
		if err == nil {
			if expert, ok, loadErr := agent.LoadSessionExpert(path); loadErr == nil && ok {
				if entry.view.expertID == "" || entry.view.expertID == strings.TrimSpace(expert.ExpertID) {
					entry.view.expertID = strings.TrimSpace(expert.ExpertID)
					entry.view.expertName = strings.TrimSpace(expert.ExpertName)
				}
			}
			out[sessionRuntimeKey(path)] = entry.view
		}
	}
	return out
}

func applyChannelSessionRoute(meta *SessionMeta, route channelSessionRoute) {
	if meta == nil {
		return
	}
	meta.Kind = "channel"
	meta.Channel = route.channel
	meta.ChannelLabel = route.channelLabel
	meta.RemoteID = route.remoteID
	meta.ChatType = route.chatType
	meta.UserID = route.userID
	meta.ThreadID = route.threadID
	meta.SessionSource = route.sessionSource
}

func (a *App) channelSessionRoutesForDir(dir string) map[string]channelSessionRoute {
	// Session route labels need only bot connection metadata. Go through the
	// active profile's no-credential settings loader instead of the global
	// config loader so listing sessions can never hydrate a provider BYOK key.
	cfg, _, err := a.loadDesktopUserConfigForView()
	if err != nil || cfg == nil {
		return nil
	}
	out := map[string]channelSessionRoute{}
	for _, conn := range cfg.Bot.Connections {
		channel := strings.TrimSpace(conn.Provider)
		if channel == "" {
			continue
		}
		channelLabel := strings.TrimSpace(conn.Label)
		if channelLabel == "" {
			channelLabel = channelDisplayName(channel, conn.Domain)
		}
		for _, mapping := range conn.SessionMappings {
			if strings.TrimSpace(mapping.SessionSource) != "auto" {
				continue
			}
			sessionPath := botSessionPathTarget(mapping.SessionID)
			if sessionPath == "" {
				continue
			}
			validPath, _, err := validateSessionPath(dir, sessionPath)
			if err != nil {
				continue
			}
			key := sessionRuntimeKey(validPath)
			if key == "" {
				continue
			}
			out[key] = channelSessionRoute{
				channel:       channel,
				channelLabel:  channelLabel,
				remoteID:      strings.TrimSpace(mapping.RemoteID),
				chatType:      strings.TrimSpace(mapping.ChatType),
				userID:        strings.TrimSpace(mapping.UserID),
				threadID:      strings.TrimSpace(mapping.ThreadID),
				sessionSource: strings.TrimSpace(mapping.SessionSource),
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func botSessionPathTarget(sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(sessionID), "path:") {
		return strings.TrimSpace(sessionID[5:])
	}
	if strings.HasSuffix(sessionID, ".jsonl") || strings.Contains(sessionID, "/") || strings.Contains(sessionID, `\`) || strings.HasPrefix(sessionID, "~") {
		return sessionID
	}
	return ""
}

func channelDisplayName(provider, domain string) string {
	provider = strings.TrimSpace(provider)
	domain = strings.TrimSpace(domain)
	switch provider {
	case "feishu":
		if strings.EqualFold(domain, "lark") {
			return "Lark"
		}
		return "Feishu"
	case "weixin":
		return "WeChat"
	case "qq":
		return "QQ"
	default:
		return provider
	}
}

// DeleteSession moves a saved session to the local trash. If the session still
// has an in-process runtime, the runtime is cancelled and removed first so
// autosave cannot recreate or append to the deleted file later.
func (a *App) DeleteSession(path string) error {
	dir, sessionPath, err := a.sessionDirForPath(path)
	if err != nil {
		return err
	}
	deletedMeta, deletedMetaExists, deletedMetaErr := agent.LoadBranchMeta(sessionPath)
	if deletedMetaErr != nil {
		return deletedMetaErr
	}
	deletedCloudProjectTopic := deletedMetaExists && deletedMeta.ProjectContext != nil && strings.TrimSpace(deletedMeta.TopicID) != ""
	if err := a.authorizeProjectSessionMutation(sessionPath, true); err != nil {
		return err
	}
	_, key, err := validateSessionPath(dir, sessionPath)
	if err != nil {
		return err
	}
	if err := validateSessionTrashTarget(dir, sessionPath, key); err != nil {
		return err
	}
	removed, fallback, err := a.removeSessionRuntimeBindings(dir, sessionPath)
	if err != nil {
		return err
	}
	if err := a.prepareRemovedSessionRuntimes(removed); err != nil {
		a.closeRemovedSessionRuntimes(removed)
		return err
	}
	closedRemoved := map[control.SessionAPI]bool{}
	destroys := a.destroyHandlesForSession(dir, sessionPath, removed)
	teardownTimedOut := waitDestroyHandles(destroys)
	a.closeRemovedSessionRuntimesForSessionAfterDestroy(removed, dir, sessionPath, closedRemoved)
	if teardownTimedOut {
		if err := agent.MarkCleanupPending(sessionPath, "delete"); err != nil {
			a.closeRemainingRemovedSessionRuntimesAfterDestroy(removed, closedRemoved)
			return err
		}
		go delayedDesktopSessionTrash(dir, sessionPath, key, destroys)
	} else {
		err = trashSessionArtifacts(dir, sessionPath, key)
		finishDestroyHandles(destroys)
		if err != nil {
			a.closeRemainingRemovedSessionRuntimesAfterDestroy(removed, closedRemoved)
			return err
		}
	}
	a.closeRemainingRemovedSessionRuntimesAfterDestroy(removed, closedRemoved)
	if deletedCloudProjectTopic {
		target := fallbackRuntimeTarget{scope: deletedMeta.Scope, workspaceRoot: deletedMeta.WorkspaceRoot, topicID: deletedMeta.TopicID}
		if remaining, _ := a.findTopicContentSessionForTarget(target.scope, target.workspaceRoot, target.topicID); remaining == "" {
			// A Cloud Project session is represented by an ordinary global topic
			// underneath. Once its last transcript is trashed, retaining that topic
			// metadata would make the deleted project session reappear under Global.
			topicMetadataRoot := target.workspaceRoot
			if strings.EqualFold(strings.TrimSpace(target.scope), "global") {
				topicMetadataRoot = ""
			}
			titles := loadTopicTitles(topicMetadataRoot)
			if _, exists := titles[target.topicID]; exists {
				if err := a.DeleteTopic(target.topicID); err != nil {
					return err
				}
			}
		}
	}
	if fallback.needs {
		fallback = a.sessionDeleteFallbackTarget(fallback)
		if err := a.openFallbackRuntime(fallback); err != nil {
			return err
		}
	}
	a.emitProjectTreeChanged()
	a.invalidatePromptHistoryCache()
	return nil
}

type removedSessionRuntime struct {
	tab           *WorkspaceTab
	ctrl          control.SessionAPI
	sink          *tabEventSink
	sessionDir    string
	sessionPath   string
	scope         string
	workspaceRoot string
	topicID       string
	readOnly      bool
}

type fallbackRuntimeTarget struct {
	needs         bool
	scope         string
	workspaceRoot string
	topicID       string
}

func (a *App) removeSessionRuntimeBindings(dir, sessionPath string) ([]removedSessionRuntime, fallbackRuntimeTarget, error) {
	var removed []removedSessionRuntime
	var fallback fallbackRuntimeTarget

	removeIDs := make(map[string]struct{})
	a.mu.RLock()
	for id, tab := range a.tabRuntimes {
		if tabMatchesSession(tab, dir, sessionPath) {
			removeIDs[id] = struct{}{}
		}
	}
	a.mu.RUnlock()
	if err := a.handoffBeforeRemovingTabs(removeIDs); err != nil {
		return nil, fallbackRuntimeTarget{}, err
	}

	a.mu.Lock()
	for id, tab := range a.tabRuntimes {
		if !tabMatchesSession(tab, dir, sessionPath) {
			continue
		}
		if len(removed) == 0 {
			fallback = fallbackRuntimeTarget{scope: tab.Scope, workspaceRoot: tab.WorkspaceRoot, topicID: tab.TopicID}
		}
		removed = append(removed, removedRuntimeFromTab(tab, dir, sessionPath))
		a.markTabRemovedLocked(tab)
		delete(a.tabRuntimes, id)
		a.removeTabOrderLocked(id)
		if a.cachedActiveTabID() == id {
			a.setActiveTabIDLocked("")
		}
	}
	for key, tab := range a.detachedSessions {
		if !tabMatchesSession(tab, dir, sessionPath) {
			continue
		}
		if len(removed) == 0 {
			fallback = fallbackRuntimeTarget{scope: tab.Scope, workspaceRoot: tab.WorkspaceRoot, topicID: tab.TopicID}
		}
		removed = append(removed, removedRuntimeFromTab(tab, dir, sessionPath))
		a.markTabRemovedLocked(tab)
		delete(a.detachedSessions, key)
	}
	fallback.needs = len(removed) > 0 && len(a.tabRuntimes) == 0
	dir, entries, activeID, version := a.saveTabsCollectLocked()
	a.mu.Unlock()

	a.saveTabsWrite(dir, entries, activeID, version)

	return removed, fallback, nil
}

func (a *App) sessionDeleteFallbackTarget(target fallbackRuntimeTarget) fallbackRuntimeTarget {
	topicID := strings.TrimSpace(target.topicID)
	if topicID == "" {
		return target
	}
	if path, _ := a.findTopicContentSessionForTarget(target.scope, target.workspaceRoot, topicID); path != "" {
		return target
	}
	target.topicID = ""
	return target
}

func (a *App) removeTopicRuntimeBindings(topicID string) ([]removedSessionRuntime, fallbackRuntimeTarget, error) {
	var removed []removedSessionRuntime
	var fallback fallbackRuntimeTarget

	removeIDs := make(map[string]struct{})
	a.mu.RLock()
	activeActor := a.currentActorContextLocked()
	if !a.currentActorExplicit {
		if active := a.activeTabLocked(); active != nil {
			activeActor = tabRuntimeActorContext(active)
		}
	}
	for id, tab := range a.tabRuntimes {
		if tab != nil && tab.TopicID == topicID && actorProfilesMatch(tabRuntimeActorContext(tab), activeActor) {
			removeIDs[id] = struct{}{}
		}
	}
	a.mu.RUnlock()
	if err := a.handoffBeforeRemovingTabs(removeIDs); err != nil {
		return nil, fallbackRuntimeTarget{}, err
	}

	a.mu.Lock()
	for id, tab := range a.tabRuntimes {
		if tab == nil || tab.TopicID != topicID || !actorProfilesMatch(tabRuntimeActorContext(tab), activeActor) {
			continue
		}
		sessionDir := tabSessionDir(tab)
		sessionPath := canonicalTabSessionPath(tab.currentSessionPath())
		if len(removed) == 0 {
			fallback = fallbackRuntimeTarget{scope: tab.Scope, workspaceRoot: tab.WorkspaceRoot}
		}
		removed = append(removed, removedRuntimeFromTab(tab, sessionDir, sessionPath))
		a.markTabRemovedLocked(tab)
		delete(a.tabRuntimes, id)
		a.removeTabOrderLocked(id)
		if a.cachedActiveTabID() == id {
			a.setActiveTabIDLocked("")
		}
	}
	for key, tab := range a.detachedSessions {
		if tab == nil || tab.TopicID != topicID || !actorProfilesMatch(tabRuntimeActorContext(tab), activeActor) {
			continue
		}
		sessionDir := tabSessionDir(tab)
		sessionPath := canonicalTabSessionPath(tab.currentSessionPath())
		if len(removed) == 0 {
			fallback = fallbackRuntimeTarget{scope: tab.Scope, workspaceRoot: tab.WorkspaceRoot}
		}
		removed = append(removed, removedRuntimeFromTab(tab, sessionDir, sessionPath))
		a.markTabRemovedLocked(tab)
		delete(a.detachedSessions, key)
	}
	fallback.needs = len(removed) > 0 && len(a.tabRuntimes) == 0
	dir, entries, activeID, version := a.saveTabsCollectLocked()
	a.mu.Unlock()

	a.saveTabsWrite(dir, entries, activeID, version)

	return removed, fallback, nil
}

// handoffBeforeRemovingTabs transfers billing/event ownership while every tab
// still exists. If no visible tab survives, it drains the old owner and leaves
// activation to the caller's fallback path.
func (a *App) handoffBeforeRemovingTabs(removeIDs map[string]struct{}) error {
	if len(removeIDs) == 0 {
		return nil
	}
	a.mu.RLock()
	activeID := a.cachedActiveTabID()
	_, removesActive := removeIDs[activeID]
	nextID := ""
	if removesActive {
		for _, id := range a.orderedTabIDsLocked() {
			if _, removed := removeIDs[id]; !removed {
				nextID = id
				break
			}
		}
	}
	a.mu.RUnlock()
	if !removesActive {
		return nil
	}
	if nextID != "" {
		if err := a.setActiveTabForOwnerTransition(nextID); err != nil {
			return fmt.Errorf("switch active tab before removal: %w", err)
		}
		return nil
	}
	if a.cloudRuntime != nil {
		a.cloudRuntime.stopEventWorker()
	}
	if err := a.quiesceDesktopBillingWorker(); err != nil {
		return fmt.Errorf("quiesce active profile before removal: %w", err)
	}
	return nil
}

func removedRuntimeFromTab(tab *WorkspaceTab, dir, sessionPath string) removedSessionRuntime {
	return removedSessionRuntime{
		tab:           tab,
		ctrl:          tab.Ctrl,
		sink:          tab.sink,
		sessionDir:    dir,
		sessionPath:   sessionPath,
		scope:         tab.Scope,
		workspaceRoot: tab.WorkspaceRoot,
		topicID:       tab.TopicID,
		readOnly:      tab.ReadOnly,
	}
}

func tabMatchesSession(tab *WorkspaceTab, dir, sessionPath string) bool {
	if tab == nil || tabSessionDir(tab) != dir {
		return false
	}
	currentPath, _, err := validateSessionPath(dir, tab.currentSessionPath())
	return err == nil && currentPath == sessionPath
}

func (a *App) prepareRemovedSessionRuntimes(removed []removedSessionRuntime) error {
	for _, item := range removed {
		if item.sink != nil {
			item.sink.clearContext()
		}
		if item.ctrl == nil {
			continue
		}
		status := item.ctrl.RuntimeStatus()
		if status.Running {
			item.ctrl.Cancel()
		}
		if status.Running || status.Completing {
			if err := waitControllerStopped(item.ctrl); err != nil {
				return err
			}
		}
		if item.readOnly {
			continue
		}
		if err := item.ctrl.Snapshot(); err != nil {
			return err
		}
		a.quiesceTabAutosave(item.tab)
		item.ctrl.SetSessionPath("")
	}
	return nil
}

func waitControllerStopped(ctrl control.SessionAPI) error {
	deadline := time.Now().Add(5 * time.Second)
	for {
		status := ctrl.RuntimeStatus()
		if !status.Running && !status.Completing {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for cancelled session work to stop")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (a *App) destroyHandlesForSession(dir, sessionPath string, removed []removedSessionRuntime) []control.SessionDestroyHandle {
	destroys := a.beginDestroySessionJobs(dir, sessionPath)
	for _, item := range removed {
		if item.ctrl == nil || item.sessionDir != dir || item.sessionPath != sessionPath {
			continue
		}
		destroys = append(destroys, item.ctrl.BeginDestroySession(sessionPath))
	}
	return destroys
}

func waitDestroyHandles(destroys []control.SessionDestroyHandle) bool {
	timedOut := false
	for _, destroy := range destroys {
		if destroy.Wait != nil {
			if destroy.Wait().HasTimedOut() {
				timedOut = true
			}
		}
	}
	return timedOut
}

func waitAllDestroyHandles(destroys []control.SessionDestroyHandle) {
	for _, destroy := range destroys {
		if destroy.WaitAll != nil {
			destroy.WaitAll()
		}
	}
}

func finishDestroyHandles(destroys []control.SessionDestroyHandle) {
	for _, destroy := range destroys {
		if destroy.Finish != nil {
			destroy.Finish()
		}
	}
}

func delayedDesktopSessionCleanup(path string, destroys []control.SessionDestroyHandle) {
	waitAllDestroyHandles(destroys)
	if err := removeDesktopSessionArtifacts(path); err != nil {
		slog.Warn("desktop: delayed session cleanup failed", "path", path, "err", err)
	}
	finishDestroyHandles(destroys)
}

func delayedDesktopSessionTrash(dir, sessionPath, key string, destroys []control.SessionDestroyHandle) {
	waitAllDestroyHandles(destroys)
	if err := trashSessionArtifacts(dir, sessionPath, key); err != nil {
		slog.Warn("desktop: delayed session trash failed", "path", sessionPath, "err", err)
	}
	finishDestroyHandles(destroys)
}

func (a *App) closeRemovedSessionRuntimes(removed []removedSessionRuntime) {
	a.closeRemainingRemovedSessionRuntimes(removed, map[control.SessionAPI]bool{})
}

func (a *App) closeRemovedSessionRuntimesForSessionAfterDestroy(removed []removedSessionRuntime, dir, sessionPath string, closed map[control.SessionAPI]bool) {
	releasedTabs := map[*WorkspaceTab]bool{}
	for _, item := range removed {
		if item.sessionDir != dir || item.sessionPath != sessionPath {
			continue
		}
		a.closeRemovedSessionRuntime(item, closed, releasedTabs, true)
	}
}

func (a *App) closeRemainingRemovedSessionRuntimes(removed []removedSessionRuntime, closed map[control.SessionAPI]bool) {
	releasedTabs := map[*WorkspaceTab]bool{}
	for _, item := range removed {
		a.closeRemovedSessionRuntime(item, closed, releasedTabs, false)
	}
}

func (a *App) closeRemainingRemovedSessionRuntimesAfterDestroy(removed []removedSessionRuntime, closed map[control.SessionAPI]bool) {
	releasedTabs := map[*WorkspaceTab]bool{}
	for _, item := range removed {
		a.closeRemovedSessionRuntime(item, closed, releasedTabs, true)
	}
}

func (a *App) closeRemovedSessionRuntime(item removedSessionRuntime, closed map[control.SessionAPI]bool, releasedTabs map[*WorkspaceTab]bool, afterDestroy bool) {
	if item.tab != nil {
		if releasedTabs == nil || !releasedTabs[item.tab] {
			if releasedTabs != nil {
				releasedTabs[item.tab] = true
			}
			a.releaseTabSharedHost(item.tab)
		}
	}
	if item.ctrl == nil {
		return
	}
	if closed == nil {
		closed = map[control.SessionAPI]bool{}
	}
	if closed[item.ctrl] {
		return
	}
	closed[item.ctrl] = true
	if afterDestroy {
		item.ctrl.CloseAfterDestroy()
		return
	}
	item.ctrl.Close()
}

func (a *App) openFallbackRuntime(target fallbackRuntimeTarget) error {
	scope := target.scope
	root := target.workspaceRoot
	topicID := strings.TrimSpace(target.topicID)
	if scope == "global" {
		root = ""
	}
	if topicID == "" {
		topic, err := a.CreateTopic(scope, root, "")
		if err != nil {
			return err
		}
		topicID = topic.ID
	}
	var err error
	if a.singleSurfaceLayoutEnabled() {
		_, err = a.ActivateTopic(scope, root, topicID, "")
	} else if scope == "global" {
		_, err = a.OpenGlobalTab(topicID)
	} else {
		_, err = a.OpenProjectTab(root, topicID)
	}
	return err
}

func (a *App) beginDestroySessionJobs(dir, sessionPath string) []control.SessionDestroyHandle {
	a.mu.RLock()
	defer a.mu.RUnlock()
	var destroys []control.SessionDestroyHandle
	for _, tab := range a.runtimeTabsLocked() {
		if tab == nil || tab.Ctrl == nil || tabSessionDir(tab) != dir {
			continue
		}
		destroys = append(destroys, tab.Ctrl.BeginDestroySession(sessionPath))
	}
	return destroys
}

func (a *App) openSessionPaths(dir string) map[string]struct{} {
	a.mu.RLock()
	paths := make([]string, 0, len(a.tabRuntimes)+len(a.detachedSessions))
	for _, tab := range a.runtimeTabsLocked() {
		if tab != nil {
			paths = append(paths, tab.currentSessionPath())
		}
	}
	a.mu.RUnlock()

	out := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		currentPath, _, err := validateSessionPath(dir, path)
		if err == nil {
			out[currentPath] = struct{}{}
		}
	}
	return out
}

func (a *App) activeSessionPath(dir string) string {
	a.mu.RLock()
	var path string
	if tab := a.tabRuntimes[a.cachedActiveTabID()]; tab != nil {
		path = tab.currentSessionPath()
	}
	a.mu.RUnlock()
	currentPath, _, err := validateSessionPath(dir, path)
	if err != nil {
		return ""
	}
	return currentPath
}

// RestoreSession moves a trashed session back into the saved-session list.
func (a *App) RestoreSession(path string) error {
	dir, err := a.trashedSessionDir(path)
	if err != nil {
		return err
	}
	if err := a.authorizeProjectSessionMutation(path, false); err != nil {
		return err
	}
	_, key, _, err := validateTrashedSessionPath(dir, path)
	if err != nil {
		return err
	}
	target := filepath.Join(dir, key)
	if a.sessionDestroying(dir, target) {
		return fmt.Errorf("session cleanup is still in progress: %s", key)
	}
	if a.sessionOpen(dir, target) {
		return fmt.Errorf("session is open: %s", key)
	}
	if err := restoreTrashedSessionFile(dir, path); err != nil {
		return err
	}
	if err := restoreSessionTopicIndex(dir, target); err != nil {
		return err
	}
	a.emitProjectTreeChanged()
	a.invalidatePromptHistoryCache()
	return nil
}

func (a *App) sessionDestroying(dir, sessionPath string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, tab := range a.runtimeTabsLocked() {
		if tab == nil || tab.Ctrl == nil || tabSessionDir(tab) != dir {
			continue
		}
		if tab.Ctrl.IsDestroyingSession(sessionPath) {
			return true
		}
	}
	return false
}

func (a *App) sessionOpen(dir, sessionPath string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, tab := range a.runtimeTabsLocked() {
		if tabMatchesSession(tab, dir, sessionPath) {
			return true
		}
	}
	return false
}

// PurgeTrashedSession permanently removes a trashed session and its title/display
// sidecars.
func (a *App) PurgeTrashedSession(path string) error {
	dir, err := a.trashedSessionDir(path)
	if err != nil {
		return err
	}
	if err := a.authorizeProjectSessionMutation(path, true); err != nil {
		return err
	}
	if err := purgeTrashedSessionFile(dir, path); err != nil {
		return err
	}
	a.invalidatePromptHistoryCache()
	return nil
}

// RenameSession sets a custom display name for a session (empty clears it back to
// the preview). It only affects the history panel; the file on disk is unchanged.
func (a *App) RenameSession(path, title string) error {
	dir, sessionPath, err := a.sessionDirForPath(path)
	if err != nil {
		return err
	}
	if err := a.authorizeProjectSessionMutation(sessionPath, false); err != nil {
		return err
	}
	if err := setSessionTitle(dir, sessionPath, title); err != nil {
		return err
	}
	a.invalidatePromptHistoryCache()
	a.emitProjectTreeChanged()
	return nil
}

// ResumeSession snapshots the current conversation, then loads the session at
// path and continues it on the active tab. The model and working folder are
// unchanged; only the transcript is swapped. Returns the resumed messages for
// the frontend to render.
func (a *App) ResumeSession(path string) ([]HistoryMessage, error) {
	return a.ResumeSessionForTab("", path)
}

// ActivateSession switches the active tab to a saved session without returning
// its full transcript; paged clients then call HistoryPageForTab.
func (a *App) ActivateSession(path string) error {
	return a.ActivateSessionForTab("", path)
}

// ActivateSessionForTab is the bounded-IPC form of ResumeSessionForTab.
func (a *App) ActivateSessionForTab(tabID, path string) error {
	tab := a.tabByID(tabID)
	if tab == nil {
		return fmt.Errorf("tab not found")
	}
	visible, readOnly, _ := a.projectSessionVisible(a.bootContext(), tabRuntimeActorContext(tab), path)
	if !visible {
		return fmt.Errorf("session is unavailable in the current scope")
	}
	return a.activateSessionForTab(tabID, path, readOnly)
}

// ResumeSessionForTab is the tab-scoped form of ResumeSession. A saved session
// path is a runtime identity, so changing to a different path must replace the
// tab's controller binding rather than mutating the current controller in place.
func (a *App) ResumeSessionForTab(tabID, path string) ([]HistoryMessage, error) {
	tab := a.tabByID(tabID)
	if tab == nil {
		return nil, fmt.Errorf("tab not found")
	}
	visible, readOnly, _ := a.projectSessionVisible(a.bootContext(), tabRuntimeActorContext(tab), path)
	if !visible {
		return nil, fmt.Errorf("session is unavailable in the current scope")
	}
	if err := a.activateSessionForTabLoaded(tabID, path, readOnly); err != nil {
		return nil, err
	}
	return a.HistoryForTab(tabID)
}

func (a *App) activateSessionForTabLoaded(tabID, path string, readOnly bool) error {
	tab := a.tabByID(tabID)
	if tab == nil {
		return fmt.Errorf("tab not found")
	}
	preloaded, err := a.preloadTabSession(tab, path)
	if err != nil {
		return err
	}
	sessionPath := preloaded.Path
	if sessionRuntimeKey(tab.currentSessionPath()) == sessionRuntimeKey(sessionPath) {
		a.setTabReadOnly(tab.ID, readOnly)
		return nil
	}
	if err := a.rebindTabToLoadedSessionPath(tab, sessionPath, preloaded.Session); err != nil {
		return err
	}
	a.setTabReadOnly(tab.ID, readOnly)
	return nil
}

func (a *App) activateSessionForTab(tabID, path string, readOnly bool) error {
	tab := a.tabByID(tabID)
	if tab == nil {
		return fmt.Errorf("tab not found")
	}
	_, sessionPath, err := resolveTabSessionPath(tab, path)
	if err != nil {
		return err
	}
	if sessionRuntimeKey(tab.currentSessionPath()) == sessionRuntimeKey(sessionPath) {
		a.setTabReadOnly(tab.ID, readOnly)
		return nil
	}

	if err := a.rebindTabToSessionPathDeferred(tab, sessionPath); err != nil {
		return err
	}
	a.setTabReadOnly(tab.ID, readOnly)
	return nil
}

func (a *App) OpenChannelSessionForTab(tabID, path string) ([]HistoryMessage, error) {
	if err := a.activateSessionForTabLoaded(tabID, path, true); err != nil {
		return nil, err
	}
	return a.HistoryForTab(tabID)
}

// ActivateChannelSessionForTab opens a read-only channel session without
// returning its full transcript.
func (a *App) ActivateChannelSessionForTab(tabID, path string) error {
	return a.activateSessionForTab(tabID, path, true)
}

func (a *App) setTabReadOnly(tabID string, readOnly bool) {
	a.mu.Lock()
	if tab := a.tabRuntimes[tabID]; tab != nil && tab.ReadOnly != readOnly {
		tab.ReadOnly = readOnly
		a.saveTabsLocked()
	}
	a.mu.Unlock()
}

func (a *App) rebindTabToSessionPath(tab *WorkspaceTab, sessionPath string) error {
	preloaded, err := a.preloadTabSession(tab, sessionPath)
	if err != nil {
		return err
	}
	return a.rebindTabToLoadedSessionPath(tab, preloaded.Path, preloaded.Session)
}

// rebindTabToSessionPathDeferred makes a validated transcript available to
// paged readers before the resumable controller has decoded the whole file.
// Sending already waits for tab readiness, so controller construction can use
// the tab's existing background-build lifecycle without delaying first paint.
func (a *App) rebindTabToSessionPathDeferred(tab *WorkspaceTab, sessionPath string) error {
	if tab == nil {
		return fmt.Errorf("tab is not ready")
	}
	_, validPath, err := resolveTabSessionPath(tab, sessionPath)
	if err != nil {
		return err
	}
	sessionPath = validPath
	if agent.IsCleanupPending(sessionPath) {
		return fmt.Errorf("session is pending cleanup")
	}
	if _, err := os.Stat(sessionPath); err != nil {
		return err
	}
	if sessionRuntimeKey(tab.currentSessionPath()) == sessionRuntimeKey(sessionPath) {
		return nil
	}
	composerState, hasComposerState := loadSessionComposerState(sessionPath)

	ctrl := tab.Ctrl
	actor := tabRuntimeActorContext(tab)
	if ctrl != nil {
		if err := a.snapshotTab(tab); err != nil {
			recordDesktopBackgroundFailure("session.snapshot.rebind", err)
		}
		if tab.hasActiveRuntimeWork() {
			if !a.detachRuntimeForReplacement(tab) {
				return fmt.Errorf("current session runtime cannot be detached")
			}
		} else {
			ctrl.Close()
		}
	}
	tab.resetSessionRuntimeSidecars()

	a.mu.Lock()
	tab.actor = actor
	tab.Ctrl = nil
	tab.SessionPath = sessionPath
	if hasComposerState {
		applySessionComposerState(tab, composerState)
	}
	tab.sessionTurns = nil
	tab.Ready = false
	clearTabStartupFailureLocked(tab)
	tab.ActivityStatus = ""
	tab.activityTurnID = ""
	tab.sink = &tabEventSink{tabID: tab.ID, app: a, ctx: a.ctx}
	a.saveTabsLocked()
	a.mu.Unlock()

	return a.scheduleTabControllerBuildDeferred(tab, loadedTabSession{Path: sessionPath})
}

func (a *App) rebindTabToLoadedSessionPath(tab *WorkspaceTab, sessionPath string, loaded *agent.Session) error {
	if tab == nil {
		return fmt.Errorf("tab is not ready")
	}
	_, validPath, err := resolveTabSessionPath(tab, sessionPath)
	if err != nil {
		return err
	}
	sessionPath = validPath
	if loaded == nil {
		if agent.IsCleanupPending(sessionPath) {
			return fmt.Errorf("session is pending cleanup")
		}
		loaded, err = loadResumableSession(sessionPath)
		if err != nil {
			return err
		}
	}
	_, exactSessionTurns := agent.SessionPreviewFromMessages(loaded.Snapshot())
	sessionTurns := intPointer(exactSessionTurns)
	if sessionRuntimeKey(tab.currentSessionPath()) == sessionRuntimeKey(sessionPath) {
		a.setTabSessionTurns(tab, sessionTurns)
		return nil
	}
	composerState, hasComposerState := loadSessionComposerState(sessionPath)

	ctrl := tab.Ctrl
	if ctrl == nil {
		tab.resetSessionRuntimeSidecars()
		a.mu.Lock()
		tab.SessionPath = sessionPath
		if hasComposerState {
			applySessionComposerState(tab, composerState)
		}
		tab.sessionTurns = sessionTurns
		tab.Ready = false
		clearTabStartupFailureLocked(tab)
		tab.ActivityStatus = ""
		tab.activityTurnID = ""
		tab.sink = &tabEventSink{tabID: tab.ID, app: a, ctx: a.ctx}
		a.saveTabsLocked()
		a.mu.Unlock()
		a.startTabControllerBuildWithLoadedSession(tab, loadedTabSession{Path: sessionPath, Session: loaded})
		return nil
	}

	if err := a.snapshotTab(tab); err != nil { // persist writable sessions before switching the view.
		recordDesktopBackgroundFailure("session.snapshot.preview", err)
	}
	if tab.hasActiveRuntimeWork() {
		if !a.detachRuntimeForReplacement(tab) {
			return fmt.Errorf("current session runtime cannot be detached")
		}
	} else {
		ctrl.Close()
	}
	tab.resetSessionRuntimeSidecars()

	a.mu.Lock()
	tab.Ctrl = nil
	tab.SessionPath = sessionPath
	if hasComposerState {
		applySessionComposerState(tab, composerState)
	}
	tab.sessionTurns = sessionTurns
	tab.Ready = false
	clearTabStartupFailureLocked(tab)
	tab.ActivityStatus = ""
	tab.activityTurnID = ""
	tab.sink = &tabEventSink{tabID: tab.ID, app: a, ctx: a.ctx}
	a.saveTabsLocked()
	a.mu.Unlock()

	a.startTabControllerBuildWithLoadedSession(tab, loadedTabSession{Path: sessionPath, Session: loaded})
	return nil
}

func loadResumableSession(sessionPath string) (*agent.Session, error) {
	if agent.IsCleanupPending(sessionPath) {
		return nil, fmt.Errorf("session is pending cleanup")
	}
	return agent.LoadSession(sessionPath)
}

// PreviewSession reads a saved session for display only. It does not snapshot or
// swap the active controller, so the history drawer can call it while a turn runs.
func (a *App) PreviewSession(path string) ([]HistoryMessage, error) {
	sessionDir, sessionPath, err := a.sessionDirForPath(path)
	if err != nil {
		return nil, err
	}
	a.mu.RLock()
	activeTab := a.activeTabLocked()
	actor := userctx.ActorContext{}.Normalize()
	if activeTab != nil {
		actor = tabRuntimeActorContext(activeTab).Normalize()
	}
	a.mu.RUnlock()
	if visible, _, _ := a.projectSessionListVisible(a.bootContext(), actor, sessionPath); !visible {
		return nil, fmt.Errorf("session preview is unavailable in the current scope")
	}
	return previewSessionMessages(sessionDir, sessionPath)
}

// invalidatePromptHistoryCache resets the lazy prompt-history tape so the next
// ScanPromptHistory call rebuilds session order and reloads sessions on demand.
// Called from every session-mutating path: NewSession, ClearSession,
// DeleteSession, RestoreSession, PurgeTrashedSession, RenameSession.
func (a *App) invalidatePromptHistoryCache() {
	a.promptHistoryMu.Lock()
	a.promptHistoryTape = nil
	a.promptHistoryMu.Unlock()
}

const (
	promptHistoryPageLimit    = 50
	promptHistoryMaxPageLimit = 200
)

type promptHistoryRequest struct {
	Nonce  string `json:"nonce,omitempty"`
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit,omitempty"`
	legacy bool
}

type promptHistoryCursor struct {
	Nonce   string `json:"n"`
	Session int    `json:"s"`
	Offset  int    `json:"o"`
}

type promptHistoryTape struct {
	nonce           string
	dir             string
	currentPath     string
	displays        sessionDisplayMap
	sessions        []promptHistorySessionFile
	loaded          map[string][]PromptHistoryEntry
	projectScopeKey string
}

// ScanPromptHistory returns the next prompt-history tape segment. The request is
// a JSON string so the Wails binding stays one-argument while the protocol can
// carry a cursor and page limit. Older clients may still pass a bare nonce; that
// path keeps the old cache-hit behavior.
func (a *App) ScanPromptHistory(rawRequest string) (PromptHistoryResult, error) {
	req := parsePromptHistoryRequest(rawRequest)
	dir := a.activeSessionDir()
	sessionPath := a.activeSessionPath(dir)
	a.mu.RLock()
	activeTab := a.activeTabLocked()
	actor := userctx.ActorContext{}.Normalize()
	if activeTab != nil {
		actor = tabRuntimeActorContext(activeTab).Normalize()
	}
	a.mu.RUnlock()

	a.promptHistoryMu.Lock()
	tape, err := a.promptHistoryTapeForLocked(dir, sessionPath, actor)
	if err != nil {
		a.promptHistoryMu.Unlock()
		return PromptHistoryResult{}, err
	}
	if req.legacy && req.Nonce != "" && req.Nonce == tape.nonce {
		a.promptHistoryMu.Unlock()
		return PromptHistoryResult{Entries: nil, Nonce: req.Nonce}, nil
	}
	result := tape.readOlder(req.Cursor, promptHistoryLimit(req.Limit))
	a.promptHistoryMu.Unlock()
	return result, nil
}

func parsePromptHistoryRequest(raw string) promptHistoryRequest {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return promptHistoryRequest{}
	}
	if strings.HasPrefix(raw, "{") {
		var req promptHistoryRequest
		if err := json.Unmarshal([]byte(raw), &req); err == nil {
			return req
		}
	}
	return promptHistoryRequest{Nonce: raw, legacy: true}
}

func promptHistoryLimit(limit int) int {
	if limit <= 0 {
		return promptHistoryPageLimit
	}
	if limit > promptHistoryMaxPageLimit {
		return promptHistoryMaxPageLimit
	}
	return limit
}

func (a *App) promptHistoryTapeForLocked(dir, sessionPath string, actor userctx.ActorContext) (*promptHistoryTape, error) {
	currentPath := ""
	if path, _, err := validateSessionPath(dir, sessionPath); err == nil {
		currentPath = path
	}
	scopeKey := ""
	if scope, err := a.currentProjectScopeForActor(a.bootContext(), actor); err == nil {
		scopeKey = scope.Key
	}
	if a.promptHistoryTape != nil && a.promptHistoryTape.dir == dir && a.promptHistoryTape.currentPath == currentPath && a.promptHistoryTape.projectScopeKey == scopeKey {
		return a.promptHistoryTape, nil
	}
	tape, err := newPromptHistoryTape(dir, currentPath)
	if err != nil {
		return nil, err
	}
	tape.projectScopeKey = scopeKey
	tape.sessions = a.filterPromptHistorySessions(actor, tape.sessions)
	a.promptHistoryTape = tape
	return tape, nil
}

func (a *App) scanPromptHistoryFromDir(dir string) ([]PromptHistoryEntry, error) {
	tape, err := newPromptHistoryTape(dir, "")
	if err != nil {
		return nil, err
	}
	a.mu.RLock()
	activeTab := a.activeTabLocked()
	actor := userctx.ActorContext{}.Normalize()
	if activeTab != nil {
		actor = tabRuntimeActorContext(activeTab).Normalize()
	}
	a.mu.RUnlock()
	tape.sessions = a.filterPromptHistorySessions(actor, tape.sessions)
	return tape.readAll(), nil
}

func (a *App) filterPromptHistorySessions(actor userctx.ActorContext, sessions []promptHistorySessionFile) []promptHistorySessionFile {
	filtered := make([]promptHistorySessionFile, 0, len(sessions))
	for _, session := range sessions {
		visible, _, _ := a.projectSessionListVisible(a.bootContext(), actor, session.path)
		if visible {
			filtered = append(filtered, session)
		}
	}
	return filtered
}

func newPromptHistoryTape(dir, currentPath string) (*promptHistoryTape, error) {
	tape := &promptHistoryTape{
		nonce:       fmt.Sprintf("%d", time.Now().UnixNano()),
		dir:         dir,
		currentPath: currentPath,
		displays:    loadSessionDisplays(dir),
		loaded:      map[string][]PromptHistoryEntry{},
	}
	sessions, err := promptHistorySessionFiles(dir)
	if err != nil {
		return nil, err
	}
	if currentPath != "" {
		currentPath = filepath.Clean(currentPath)
		currentSession := promptHistorySessionFile{}
		currentIndex := -1
		for i, session := range sessions {
			if filepath.Clean(session.path) == currentPath {
				currentSession = session
				currentIndex = i
				break
			}
		}
		if currentIndex >= 0 {
			sessions = append([]promptHistorySessionFile{currentSession}, append(sessions[:currentIndex], sessions[currentIndex+1:]...)...)
		} else if info, err := os.Stat(currentPath); err == nil && !info.IsDir() {
			sessions = append([]promptHistorySessionFile{{
				path: currentPath,
			}}, sessions...)
		}
	}
	tape.sessions = sessions
	return tape, nil
}

func (t *promptHistoryTape) readOlder(cursor string, limit int) PromptHistoryResult {
	c := promptHistoryCursor{Nonce: t.nonce}
	if decoded, ok := decodePromptHistoryCursor(cursor); ok && decoded.Nonce == t.nonce {
		c = decoded
	}
	if c.Session < 0 {
		c.Session = 0
	}
	if c.Offset < 0 {
		c.Offset = 0
	}

	out := make([]PromptHistoryEntry, 0, limit)
	sessionIndex := c.Session
	offset := c.Offset
	for sessionIndex < len(t.sessions) && len(out) < limit {
		entries, err := t.entriesForSession(sessionIndex)
		if err != nil || offset >= len(entries) {
			sessionIndex++
			offset = 0
			continue
		}

		end := min(len(entries), offset+limit-len(out))
		out = append(out, entries[offset:end]...)
		offset = end
		if offset >= len(entries) && len(out) < limit {
			sessionIndex++
			offset = 0
		}
	}

	if sessionIndex < len(t.sessions) {
		if entries, ok := t.loaded[t.sessions[sessionIndex].path]; ok && offset >= len(entries) {
			sessionIndex++
			offset = 0
		}
	}
	hasOlder := sessionIndex < len(t.sessions)
	olderCursor := ""
	if hasOlder {
		olderCursor = encodePromptHistoryCursor(promptHistoryCursor{Nonce: t.nonce, Session: sessionIndex, Offset: offset})
	}
	return PromptHistoryResult{Entries: out, Nonce: t.nonce, OlderCursor: olderCursor, HasOlder: hasOlder}
}

func (t *promptHistoryTape) readAll() []PromptHistoryEntry {
	out := []PromptHistoryEntry{}
	cursor := ""
	for {
		page := t.readOlder(cursor, promptHistoryMaxPageLimit)
		out = append(out, page.Entries...)
		if !page.HasOlder || page.OlderCursor == "" {
			return out
		}
		cursor = page.OlderCursor
	}
}

func (t *promptHistoryTape) entriesForSession(index int) ([]PromptHistoryEntry, error) {
	if index < 0 || index >= len(t.sessions) {
		return nil, nil
	}
	path := t.sessions[index].path
	if entries, ok := t.loaded[path]; ok {
		return entries, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		t.loaded[path] = nil
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	entries, err := scanPromptHistoryFile(path, info, sessionDisplayResolverFromMap(t.displays, path))
	if err != nil {
		t.loaded[path] = nil
		return nil, err
	}
	t.loaded[path] = entries
	return entries, nil
}

func encodePromptHistoryCursor(cursor promptHistoryCursor) string {
	b, err := json.Marshal(cursor)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodePromptHistoryCursor(value string) (promptHistoryCursor, bool) {
	if strings.TrimSpace(value) == "" {
		return promptHistoryCursor{}, false
	}
	b, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return promptHistoryCursor{}, false
	}
	var cursor promptHistoryCursor
	if err := json.Unmarshal(b, &cursor); err != nil {
		return promptHistoryCursor{}, false
	}
	return cursor, true
}

func scanPromptHistoryFile(path string, info os.FileInfo, resolveUserContent func(string) string) ([]PromptHistoryEntry, error) {
	entries, err := collectPromptHistoryEntries(path, info, resolveUserContent)
	if err != nil {
		return nil, err
	}
	sortPromptHistoryNewestFirst(entries)
	return entries, nil
}

type promptHistorySessionFile struct {
	path string
}

func promptHistorySessionFiles(dir string) ([]promptHistorySessionFile, error) {
	infos, err := agent.ListSessionOrder(dir)
	if err != nil {
		return nil, err
	}
	sessions := make([]promptHistorySessionFile, 0, len(infos))
	for _, info := range infos {
		sessions = append(sessions, promptHistorySessionFile{path: info.Path})
	}
	return sessions, nil
}

func promptHistoryEntryNewer(a, b PromptHistoryEntry) bool {
	if a.At != b.At {
		return a.At > b.At
	}
	if a.SessionPath != b.SessionPath {
		return a.SessionPath > b.SessionPath
	}
	return a.Turn > b.Turn
}

func sortPromptHistoryNewestFirst(entries []PromptHistoryEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return promptHistoryEntryNewer(entries[i], entries[j])
	})
}

func collectPromptHistoryEntries(path string, info os.FileInfo, resolveUserContent func(string) string) ([]PromptHistoryEntry, error) {
	var out []PromptHistoryEntry
	err := collectJSONLUserPrompts(path, info, resolveUserContent, func(entry PromptHistoryEntry) {
		out = append(out, entry)
	})
	return out, err
}

func collectJSONLUserPrompts(path string, info os.FileInfo, resolveUserContent func(string) string, emit func(PromptHistoryEntry)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fallbackAt := promptHistoryFallbackMillis(path, info)

	dec := json.NewDecoder(f)
	turn := 0
	for {
		var rec previewEventRecord
		if err := dec.Decode(&rec); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil // partial results are better than none
		}
		// Format compatibility:
		// 1) Legacy event format: {"kind":"user.message","text":"..."}
		// 2) Early event format:   {"type":"user.message","text":"..."}
		// 3) Current provider.Message format: {"role":"user","content":"..."}
		text := ""
		kindOrType := strings.TrimSpace(rec.Kind)
		if kindOrType == "" {
			kindOrType = strings.TrimSpace(rec.Type)
		}
		if kindOrType == "user.message" {
			text = strings.TrimSpace(rec.Text)
		} else if strings.TrimSpace(rec.Role) == "user" && rec.MessageKind != provider.MessageKindProjectContext {
			text = strings.TrimSpace(rec.Content)
		}
		if text != "" {
			text = resolveUserContent(text)
			text = strings.TrimSpace(text)
			if text == "" {
				continue
			}
			if control.IsSyntheticUserMessage(text) {
				continue
			}
			at := fallbackAt
			if eventAt, ok := promptHistoryEventMillis(rec); ok {
				at = eventAt
			}
			entry := PromptHistoryEntry{
				Text:        text,
				At:          at,
				SessionPath: path,
				Turn:        turn,
			}
			emit(entry)
			turn++
		}
	}
	return nil
}

func promptHistoryFallbackMillis(path string, info os.FileInfo) int64 {
	if meta, ok, err := agent.LoadBranchMeta(path); err == nil && ok && !meta.UpdatedAt.IsZero() {
		return meta.UpdatedAt.UnixMilli()
	}
	if info != nil {
		return info.ModTime().UnixMilli()
	}
	return 0
}

func promptHistoryEventMillis(rec previewEventRecord) (int64, bool) {
	for _, raw := range []json.RawMessage{
		rec.Time,
		rec.Timestamp,
		rec.CreatedAt,
		rec.CreatedAtSnake,
		rec.UpdatedAt,
		rec.UpdatedAtSnake,
	} {
		if at, ok := parseJSONTimestampMillis(raw); ok {
			return at, true
		}
	}
	return 0, false
}

func parseJSONTimestampMillis(raw json.RawMessage) (int64, bool) {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return 0, false
	}

	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		s = strings.TrimSpace(s)
		if s == "" {
			return 0, false
		}
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return normalizeTimestampMillis(n)
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return normalizeTimestampMillisFloat(f)
		}
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return t.UnixMilli(), true
		}
		return 0, false
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var n json.Number
	if err := dec.Decode(&n); err != nil {
		return 0, false
	}
	if i, err := strconv.ParseInt(n.String(), 10, 64); err == nil {
		return normalizeTimestampMillis(i)
	}
	if f, err := strconv.ParseFloat(n.String(), 64); err == nil {
		return normalizeTimestampMillisFloat(f)
	}
	return 0, false
}

func normalizeTimestampMillis(v int64) (int64, bool) {
	if v <= 0 {
		return 0, false
	}
	switch {
	case v >= 1_000_000_000_000_000_000:
		return v / 1_000_000, true // nanoseconds
	case v >= 1_000_000_000_000_000:
		return v / 1_000, true // microseconds
	case v >= 100_000_000_000:
		return v, true // milliseconds
	case v >= 1_000_000_000:
		return v * 1_000, true // seconds
	default:
		return 0, false
	}
}

func normalizeTimestampMillisFloat(v float64) (int64, bool) {
	if v <= 0 {
		return 0, false
	}
	switch {
	case v >= 1_000_000_000_000_000_000:
		return int64(v / 1_000_000), true
	case v >= 1_000_000_000_000_000:
		return int64(v / 1_000), true
	case v >= 100_000_000_000:
		return int64(v), true
	case v >= 1_000_000_000:
		return int64(v * 1_000), true
	default:
		return 0, false
	}
}

// PickWorkspace opens a folder chooser and registers the selected project
// without creating or activating a conversation. Returns "" if cancelled.
func (a *App) PickWorkspace() (string, error) {
	if a.ctx == nil {
		return "", nil
	}
	cur, _ := os.Getwd()
	a.mu.RLock()
	if tab := a.activeTabLocked(); tab != nil && tab.WorkspaceRoot != "" {
		cur = tab.WorkspaceRoot
	}
	a.mu.RUnlock()
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Choose working folder",
		DefaultDirectory: dialogDefaultDirectory(cur),
	})
	if err != nil || dir == "" {
		return "", err
	}
	return a.registerWorkspace(dir)
}

func (a *App) registerWorkspace(dir string) (string, error) {
	dir, err := normalizeWorkspaceDirectory(dir)
	if err != nil {
		return "", err
	}
	if err := addProject(dir, ""); err != nil {
		return "", err
	}
	a.emitProjectTreeChanged()
	return dir, nil
}

func normalizeWorkspaceDirectory(dir string) (string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "", fmt.Errorf("workspace path is required")
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	info, err := os.Stat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", dir)
	}
	return dir, nil
}

func dialogDefaultDirectory(preferred string) string {
	if dir := nearestExistingDirectory(preferred); dir != "" {
		return dir
	}
	if cwd, err := os.Getwd(); err == nil {
		if dir := nearestExistingDirectory(cwd); dir != "" {
			return dir
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		if dir := nearestExistingDirectory(home); dir != "" {
			return dir
		}
	}
	return ""
}

func nearestExistingDirectory(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	for {
		info, err := os.Stat(path)
		if err == nil {
			if info.IsDir() {
				return path
			}
			path = filepath.Dir(path)
			continue
		}
		parent := filepath.Dir(path)
		if parent == path {
			return ""
		}
		path = parent
	}
}

func (a *App) ListWorkspaces() []WorkspaceMeta {
	migrateLegacyWorkspacesIntoProjects()
	managedStateRoots := a.cloudProjectWorkspaceStateRoots()
	activeRoot := ""
	cur, _ := os.Getwd()
	a.mu.RLock()
	if tab := a.activeTabLocked(); tab != nil && tab.WorkspaceRoot != "" {
		activeRoot = normalizeProjectRoot(tab.WorkspaceRoot)
	}
	a.mu.RUnlock()
	if activeRoot == "" {
		activeRoot = normalizeProjectRoot(cur)
	}
	projects := loadProjectsFile().Projects
	out := make([]WorkspaceMeta, 0, len(projects))
	for _, project := range projects {
		if managedCloudProjectWorkspaceInStateRoots(project.Root, managedStateRoots) {
			continue
		}
		out = append(out, WorkspaceMeta{
			Path:    project.Root,
			Name:    projectDisplayName(project),
			Current: activeRoot != "" && project.Root == activeRoot,
		})
	}
	return out
}

func (a *App) RemoveWorkspace(dir string) error {
	if dir == "" {
		return fmt.Errorf("workspace path is required")
	}
	dir = normalizeProjectRoot(dir)
	var closeTabs []*WorkspaceTab
	var closeDetached []*WorkspaceTab
	var fallback *WorkspaceTab
	removeIDs := make(map[string]struct{})
	a.mu.Lock()
	for _, tab := range a.tabRuntimes {
		if tabInWorkspace(tab, dir) && tab.hasActiveRuntimeWork() {
			a.mu.Unlock()
			return fmt.Errorf("workspace has running sessions; stop them before removing")
		}
	}
	for _, tab := range a.detachedSessions {
		if tabInWorkspace(tab, dir) && tab.hasActiveRuntimeWork() {
			a.mu.Unlock()
			return fmt.Errorf("workspace has running sessions; stop them before removing")
		}
	}
	for id, tab := range a.tabRuntimes {
		if tabInWorkspace(tab, dir) {
			removeIDs[id] = struct{}{}
		}
	}
	a.mu.Unlock()
	if err := a.handoffBeforeRemovingTabs(removeIDs); err != nil {
		return err
	}

	a.mu.Lock()
	for id, tab := range a.tabRuntimes {
		if !tabInWorkspace(tab, dir) {
			continue
		}
		if tab.Ctrl != nil && !tab.ReadOnly {
			if err := tab.Ctrl.Snapshot(); err != nil {
				recordDesktopBackgroundFailure("session.snapshot.workspace.remove", err)
			}
		}
		a.markTabRemovedLocked(tab)
		closeTabs = append(closeTabs, tab)
		delete(a.tabRuntimes, id)
		a.removeTabOrderLocked(id)
		if a.cachedActiveTabID() == id {
			a.setActiveTabIDLocked("")
		}
	}
	for key, tab := range a.detachedSessions {
		if !tabInWorkspace(tab, dir) {
			continue
		}
		closeDetached = append(closeDetached, tab)
		delete(a.detachedSessions, key)
	}
	if len(a.tabRuntimes) == 0 {
		fallback = a.createTabEntryLocked("global", globalTabWorkspaceRoot(), "")
		fallback.TopicTitle = "Global"
		fallback.sink = &tabEventSink{tabID: fallback.ID, app: a, ctx: a.ctx}
		a.tabRuntimes[fallback.ID] = fallback
		a.tabOrder = append(a.tabOrder, fallback.ID)
	}
	activationTarget := ""
	if a.cachedActiveTabID() == "" {
		if ordered := a.orderedTabIDsLocked(); len(ordered) > 0 {
			activationTarget = ordered[0]
		}
	}
	a.saveTabsLocked()
	a.mu.Unlock()
	var activationErr error
	if activationTarget != "" {
		_, activationErr = a.activateTabMetaForOwnerTransition(activationTarget)
	}

	for _, tab := range closeTabs {
		a.closeTabRuntime(tab)
	}
	for _, tab := range closeDetached {
		a.closeTabRuntime(tab)
	}
	if activationErr != nil {
		return activationErr
	}
	if fallback != nil {
		a.startTabControllerBuild(fallback)
	}

	a.forgetWorkspace(dir)
	if err := removeProject(dir); err != nil {
		return err
	}
	// If the removed workspace was the active one, clear the pointer
	// so we don't leave a stale reference to a deleted project.
	if a.loadWorkspace() == dir {
		if remaining := loadProjectsFile(); len(remaining.Projects) > 0 {
			// Fall back to the first remaining project
			a.saveWorkspace(remaining.Projects[0].Root)
		} else {
			// No projects left; clear the active pointer entirely
			a.clearWorkspace()
		}
	}
	a.emitProjectTreeChanged()
	return nil
}

func migrateLegacyWorkspacesIntoProjects() {
	legacy := loadWorkspaces()
	if len(legacy) == 0 {
		return
	}
	if err := updateProjectsFile(func(f *desktopProjectFile) (bool, error) {
		seen := make(map[string]bool, len(f.Projects)+len(legacy))
		for _, p := range f.Projects {
			seen[p.Root] = true
		}
		changed := false
		for _, path := range legacy {
			root := normalizeProjectRoot(path)
			if root == "" || seen[root] {
				continue
			}
			f.Projects = append(f.Projects, desktopProject{Root: root})
			seen[root] = true
			changed = true
		}
		return changed, nil
	}); err != nil {
		recordDesktopBackgroundFailure("project.register.legacy", err)
	}
}

func workspaceName(path string) string {
	name := filepath.Base(path)
	if name == "." || name == string(filepath.Separator) || name == "" {
		return path
	}
	return name
}

func tabWorkspaceName(tab *WorkspaceTab, cwd string) string {
	if tab.Scope == "global" {
		return globalProjectTitle()
	}
	return workspaceName(cwd)
}

func (a *App) SwitchWorkspace(dir string) (string, error) {
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = home
	}
	dir, err := normalizeWorkspaceDirectory(dir)
	if err != nil {
		return "", err
	}

	var meta TabMeta
	if a.singleSurfaceLayoutEnabled() {
		meta, err = a.EnsureBlankSurface("project", dir)
	} else {
		meta, err = a.EnsureBlankTab("project", dir)
	}
	if err != nil {
		return "", err
	}
	return meta.WorkspaceRoot, nil
}

func (a *App) singleSurfaceLayoutEnabled() bool {
	cfg, _, err := a.loadDesktopUserConfigForView()
	if err != nil {
		return true
	}
	return singleSurfaceLayoutStyle(cfg.DesktopLayoutStyle())
}

// HistoryMessage is one prior turn, for the frontend to repopulate its transcript
// after a reload.
type HistoryMessage struct {
	Role               string                     `json:"role"`
	Content            string                     `json:"content"`
	CreatedAt          int64                      `json:"createdAt,omitempty"`
	FinalAnswer        bool                       `json:"finalAnswer,omitempty"`
	SubmitText         string                     `json:"submitText,omitempty"`
	Reasoning          string                     `json:"reasoning,omitempty"`
	MemoryCitations    []provider.MemoryCitation  `json:"memoryCitations,omitempty"`
	ArtifactDelivery   *provider.ArtifactDelivery `json:"artifactDelivery,omitempty"`
	Level              string                     `json:"level,omitempty"`
	ToolCalls          []HistoryToolCall          `json:"toolCalls,omitempty"`
	ToolCallID         string                     `json:"toolCallId,omitempty"`
	ToolName           string                     `json:"toolName,omitempty"`
	ToolResultArchived bool                       `json:"toolResultArchived,omitempty"`
	ToolResultError    string                     `json:"toolResultError,omitempty"`
	Pending            bool                       `json:"pending,omitempty"`
	Trigger            string                     `json:"trigger,omitempty"`
	Messages           int                        `json:"messages,omitempty"`
	Summary            string                     `json:"summary,omitempty"`
	Archive            string                     `json:"archive,omitempty"`
	Turn               *HistoryTurnMetadata       `json:"turn,omitempty"`
}

type HistoryTurnMetadata struct {
	TaskID             string `json:"taskId"`
	ModelRef           string `json:"modelRef"`
	ModelID            string `json:"modelId,omitempty"`
	BillingSource      string `json:"billingSource"`
	ChargedCredits     string `json:"chargedCredits,omitempty"`
	ToolMaaSQuota      string `json:"-"`
	ToolChargedCredits string `json:"-"`
}

// HistoryToolCall is the frontend projection of one persisted tool invocation.
type HistoryToolCall struct {
	ID                    string              `json:"id"`
	Name                  string              `json:"name"`
	Arguments             string              `json:"arguments"`
	Subject               string              `json:"subject,omitempty"`
	Summary               string              `json:"summary,omitempty"`
	Diff                  string              `json:"diff,omitempty"`
	Added                 int                 `json:"added,omitempty"`
	Removed               int                 `json:"removed,omitempty"`
	ArgumentsArchived     bool                `json:"argumentsArchived,omitempty"`
	ArtifactPaths         []string            `json:"artifactPaths,omitempty"`
	SubagentRef           string              `json:"subagentRef,omitempty"`
	TaskLifecycle         *TaskLifecycleView  `json:"taskLifecycle,omitempty"`
	TaskLifecycleRefs     []string            `json:"taskLifecycleRefs,omitempty"`
	TaskLifecycleReceipts []TaskLifecycleView `json:"taskLifecycleReceipts,omitempty"`
	SubagentMessages      []HistoryMessage    `json:"subagentMessages,omitempty"`
}

// HistoryPage is one bounded transcript window plus its stable cursor guard.
type HistoryPage struct {
	Messages   []HistoryMessage `json:"messages"`
	Before     int              `json:"before"`
	HasMore    bool             `json:"hasMore"`
	Generation string           `json:"generation"`
}

// History returns the session's message log.
func (a *App) History() ([]HistoryMessage, error) {
	return a.HistoryForTab("")
}

func (a *App) HistoryForTab(tabID string) ([]HistoryMessage, error) {
	a.mu.RLock()
	tab := a.tabByIDLocked(tabID)
	if tab == nil {
		a.mu.RUnlock()
		return nil, fmt.Errorf("tab not found")
	}
	ctrl := tab.Ctrl
	sessionPath := strings.TrimSpace(tab.SessionPath)
	a.mu.RUnlock()
	if ctrl != nil {
		sessionPath = strings.TrimSpace(ctrl.SessionPath())
	}
	if visible, _, _ := a.projectSessionListVisible(a.bootContext(), tabRuntimeActorContext(tab), sessionPath); !visible {
		return nil, fmt.Errorf("session history is unavailable in the current scope")
	}

	if ctrl == nil {
		if sessionPath == "" {
			return []HistoryMessage{}, nil
		}
		sessionDir, validPath, err := resolveTabSessionPath(tab, sessionPath)
		if err != nil {
			return nil, err
		}
		messages, err := previewSessionMessages(sessionDir, validPath)
		if err != nil {
			return nil, err
		}
		return a.attachHistoryTurnCredits(tab, messages), nil
	}
	msgs := ctrl.History()
	dir := controllerSessionDir(ctrl)
	path := ctrl.SessionPath()
	return a.attachHistoryTurnCredits(tab, historyMessagesForSession(dir, path, msgs)), nil
}

// HistoryPageForTab returns a bounded chronological window. before is the
// exclusive cursor returned by the previous page; generation rejects cursors
// after a transcript rewrite.
func (a *App) HistoryPageForTab(tabID string, before, limit int, generation string) (HistoryPage, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	a.mu.RLock()
	tab := a.tabByIDLocked(tabID)
	if tab == nil {
		a.mu.RUnlock()
		return HistoryPage{}, fmt.Errorf("tab not found")
	}
	ctrl := tab.Ctrl
	sessionPath := strings.TrimSpace(tab.SessionPath)
	a.mu.RUnlock()
	if ctrl != nil {
		sessionPath = strings.TrimSpace(ctrl.SessionPath())
	}
	if visible, _, _ := a.projectSessionListVisible(a.bootContext(), tabRuntimeActorContext(tab), sessionPath); !visible {
		return HistoryPage{}, fmt.Errorf("session history is unavailable in the current scope")
	}

	if ctrl == nil {
		if sessionPath == "" {
			return HistoryPage{Messages: []HistoryMessage{}}, nil
		}
		sessionDir, validPath, err := resolveTabSessionPath(tab, sessionPath)
		if err != nil {
			return HistoryPage{}, err
		}
		msgs, start, hasMore, nextGeneration, err := agent.LoadSessionPage(validPath, before, limit, generation)
		if err == nil {
			return HistoryPage{
				Messages: a.attachHistoryTurnCredits(tab, historyMessagesForSession(sessionDir, validPath, msgs)),
				Before:   start, HasMore: hasMore, Generation: nextGeneration,
			}, nil
		}
		if !errors.Is(err, agent.ErrSessionPageUnsupported) {
			return HistoryPage{}, err
		}
		page, err := legacyHistoryPage(sessionDir, validPath, before, limit, generation)
		if err != nil {
			return HistoryPage{}, err
		}
		return HistoryPage{
			Messages:   a.attachHistoryTurnCredits(tab, page.Messages),
			Before:     page.Before,
			HasMore:    page.HasMore,
			Generation: page.Generation,
		}, nil
	}
	var msgs []provider.Message
	var start int
	var hasMore bool
	var nextGeneration string
	if pager, ok := ctrl.(interface {
		HistoryPageWithGeneration(int, int, string) ([]provider.Message, int, bool, string, error)
	}); ok {
		var err error
		msgs, start, hasMore, nextGeneration, err = pager.HistoryPageWithGeneration(before, limit, generation)
		if err != nil {
			return HistoryPage{}, err
		}
	} else {
		all := ctrl.History()
		end := before
		if end <= 0 || end > len(all) {
			end = len(all)
		}
		start = end - limit
		if start < 0 {
			start = 0
		}
		for start > 0 && all[start].Role == provider.RoleTool {
			start--
		}
		msgs, hasMore = all[start:end], start > 0
	}
	dir, path := controllerSessionDir(ctrl), ctrl.SessionPath()
	return HistoryPage{
		Messages: a.attachHistoryTurnCredits(tab, historyMessagesForSession(dir, path, msgs)),
		Before:   start, HasMore: hasMore, Generation: nextGeneration,
	}, nil
}

func (a *App) attachHistoryTurnCredits(tab *WorkspaceTab, messages []HistoryMessage) []HistoryMessage {
	if a == nil || tab == nil || len(messages) == 0 || a.billingWorkerRuntime == nil {
		return messages
	}
	hasSubscriptionTurn := false
	for i := range messages {
		if messages[i].Turn != nil && messages[i].Turn.BillingSource == string(modelbilling.BillingSubscriptionPool) {
			hasSubscriptionTurn = true
			break
		}
	}
	if !hasSubscriptionTurn {
		return messages
	}
	actor := tabRuntimeActorContext(tab).Normalize()
	binding, err := currentDesktopSubscriptionBinding(actor, a.billingWorkerRuntime)
	if err != nil {
		return messages
	}
	db, err := openDesktopBillingDB(actor)
	if err != nil {
		return messages
	}
	defer db.Close()
	scope := modelbilling.TaskScope{ActorID: actor.ActorID, ProfileID: actor.ProfileID, CloudBinding: &binding}
	store := modelbilling.NewSQLiteQuotaSettlementStore(db)
	credits := make(map[string]string)
	missing := make(map[string]bool)
	for i := range messages {
		turn := messages[i].Turn
		if turn == nil || turn.BillingSource != string(modelbilling.BillingSubscriptionPool) || strings.TrimSpace(turn.TaskID) == "" {
			continue
		}
		if value, ok := credits[turn.TaskID]; ok {
			turn.ChargedCredits = value
			continue
		}
		if missing[turn.TaskID] {
			continue
		}
		result, found, loadErr := store.CurrentQuotaSettlement(a.bootContext(), scope, turn.TaskID)
		if loadErr != nil || !found || result.Settlement.ChargedCredits == nil {
			missing[turn.TaskID] = true
			continue
		}
		value, aggregateErr := settledTurnCredits(result, turn.ToolMaaSQuota, turn.ToolChargedCredits)
		if aggregateErr != nil {
			missing[turn.TaskID] = true
			continue
		}
		credits[turn.TaskID] = value
		turn.ChargedCredits = value
	}
	return messages
}

func legacyHistoryPage(sessionDir, path string, before, limit int, generation string) (HistoryPage, error) {
	info, err := os.Stat(path)
	if err != nil {
		return HistoryPage{}, err
	}
	currentGeneration := fmt.Sprintf("%d:%d", info.Size(), info.ModTime().UnixNano())
	if generation != "" && generation != currentGeneration {
		return HistoryPage{}, agent.ErrSessionGenerationChanged
	}
	all, err := previewSessionMessages(sessionDir, path)
	if err != nil {
		return HistoryPage{}, err
	}
	end := before
	if end <= 0 || end > len(all) {
		end = len(all)
	}
	start := end - limit
	if start < 0 {
		start = 0
	}
	for start > 0 && all[start].Role == string(provider.RoleTool) {
		start--
	}
	return HistoryPage{
		Messages: cloneHistoryMessages(all[start:end]),
		Before:   start, HasMore: start > 0, Generation: currentGeneration,
	}, nil
}

func (a *App) SubagentHistoryForTab(tabID, ref string) ([]HistoryMessage, error) {
	page, err := a.SubagentHistoryPageForTab(tabID, ref, 0, 500, "")
	if err != nil {
		return nil, err
	}
	return page.Messages, nil
}

// SubagentHistoryPageForTab lazily reads one bounded child transcript page.
func (a *App) SubagentHistoryPageForTab(tabID, ref string, before, limit int, generation string) (HistoryPage, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	artifact, sessionDir, _, err := a.subagentArtifactForTab(tabID, ref)
	if err != nil {
		return HistoryPage{}, err
	}
	msgs, start, hasMore, nextGeneration, err := agent.LoadSessionPage(
		artifact.SessionPath, before, limit, generation,
	)
	if err == nil {
		messages := historyMessagesWithPlannerDisplays(
			msgs,
			sessionDisplayResolver(sessionDir, artifact.SessionPath),
			sessionPlannerDisplayTurns(sessionDir, artifact.SessionPath),
		)
		return HistoryPage{
			Messages: sanitizeSubagentHistory(messages),
			Before:   start, HasMore: hasMore, Generation: nextGeneration,
		}, nil
	}
	if errors.Is(err, agent.ErrSessionPageUnsupported) {
		page, legacyErr := legacyHistoryPage(sessionDir, artifact.SessionPath, before, limit, generation)
		if legacyErr != nil {
			return page, legacyErr
		}
		return HistoryPage{
			Messages: sanitizeSubagentHistory(page.Messages),
			Before:   page.Before, HasMore: page.HasMore, Generation: page.Generation,
		}, nil
	}
	return HistoryPage{}, err
}

const expertMemberPromptPrefix = "You are the configured expert-team member "
const expertMemberPromptBoundary = ". Work only in that identity.\n\n"

// sanitizeSubagentHistory removes the expert-runtime identity wrapper before a
// child transcript crosses the Desktop bridge. The delegated task itself stays
// visible; internal orchestration instructions do not become chat content.
func sanitizeSubagentHistory(messages []HistoryMessage) []HistoryMessage {
	out := cloneHistoryMessages(messages)
	for i := range out {
		if out[i].Role != string(provider.RoleUser) || !strings.HasPrefix(out[i].Content, expertMemberPromptPrefix) {
			continue
		}
		if boundary := strings.Index(out[i].Content, expertMemberPromptBoundary); boundary >= 0 {
			out[i].Content = out[i].Content[boundary+len(expertMemberPromptBoundary):]
		}
	}
	return out
}

func historyMessages(msgs []provider.Message, resolveUserContent func(string) string) []HistoryMessage {
	return historyMessagesWithPlannerDisplays(msgs, resolveUserContent, nil)
}

func historyMessagesForSession(sessionDir, sessionPath string, msgs []provider.Message) []HistoryMessage {
	out := historyMessagesWithPlannerDisplays(
		msgs,
		sessionDisplayResolver(sessionDir, sessionPath),
		sessionPlannerDisplayTurns(sessionDir, sessionPath),
	)
	out = attachTaskSubagentRefs(out, sessionDir, agent.BranchID(sessionPath))
	return attachTaskLifecycles(out, msgs)
}

func attachTaskSubagentRefs(messages []HistoryMessage, sessionDir, parentSession string) []HistoryMessage {
	if len(messages) == 0 || strings.TrimSpace(sessionDir) == "" || strings.TrimSpace(parentSession) == "" {
		return messages
	}
	byToolCallID := taskSubagentRefsByToolCallID(sessionDir, parentSession)
	if len(byToolCallID) == 0 {
		return messages
	}
	for i := range messages {
		for j := range messages[i].ToolCalls {
			call := &messages[i].ToolCalls[j]
			if call.Name != "task" || strings.TrimSpace(call.ID) == "" {
				continue
			}
			ref, ok := byToolCallID[call.ID]
			if !ok {
				continue
			}
			call.SubagentRef = ref
		}
	}
	return messages
}

func taskSubagentRefsByToolCallID(sessionDir, parentSession string) map[string]string {
	artifacts, err := agent.ListSubagentsByParent(sessionDir, parentSession)
	if err != nil || len(artifacts) == 0 {
		return nil
	}
	out := map[string]string{}
	for _, artifact := range artifacts {
		if strings.TrimSpace(artifact.Meta.Kind) != "task" {
			continue
		}
		parentToolCallID := strings.TrimSpace(artifact.Meta.ParentToolCallID)
		if parentToolCallID == "" {
			continue
		}
		out[parentToolCallID] = artifact.Ref
	}
	return out
}

func historyMessagesWithPlannerDisplays(msgs []provider.Message, resolveUserContent func(string) string, plannerTurns []plannerDisplayTurn) []HistoryMessage {
	out := make([]HistoryMessage, 0, len(msgs))
	replayedTodoArgs := historyTodoArgsWithCompleteSteps(msgs)
	toolResults := historyToolResultsByID(msgs)
	plannerByUserHash := plannerTurnsByUserHash(plannerTurns)
	for _, m := range msgs {
		if m.MessageKind == provider.MessageKindReadinessRejectedCandidate {
			continue
		}
		if m.IsProjectContext() {
			continue
		}
		content := m.Content
		if m.Role == provider.RoleUser {
			if agent.IsTurnInterruptedMessage(m.Content) {
				out = append(out, HistoryMessage{Role: "notice", Level: "warn", Content: "本轮已中断"})
				continue
			}
			// Mid-turn steer messages are persisted in the session so they
			// survive tab switches. They are surfaced as a notice (↪ text)
			// — matching the live Steer event look — rather than as a
			// regular user bubble or being filtered as synthetic (#4044).
			// Check against the raw m.Content: resolveUserContent applies
			// StripComposePrefixes which trims trailing whitespace.
			if steerText, isSteer := agent.SteerText(m.Content); isSteer {
				out = append(out, HistoryMessage{Role: "notice", Content: "↪ " + steerText})
				continue
			}
			content = resolveUserContent(m.Content)
			if control.IsSyntheticUserMessage(content) {
				continue
			}
		}
		reasoning := ""
		if m.Role == provider.RoleAssistant {
			reasoning = m.ReasoningContent
		}
		hm := HistoryMessage{
			Role:        string(m.Role),
			Content:     content,
			Reasoning:   reasoning,
			FinalAnswer: m.Role == provider.RoleAssistant && m.MessageKind == provider.MessageKindFinalAnswer,
		}
		if m.Role == provider.RoleAssistant && m.Turn != nil {
			hm.Turn = &HistoryTurnMetadata{
				TaskID: m.Turn.TaskID, ModelRef: m.Turn.ModelRef, ModelID: m.Turn.ModelID, BillingSource: m.Turn.BillingSource,
				ToolMaaSQuota: m.Turn.ToolMaaSQuota, ToolChargedCredits: m.Turn.ToolChargedCredits,
			}
		}
		if m.Role == provider.RoleAssistant && len(m.MemoryCitations) > 0 {
			hm.MemoryCitations = append([]provider.MemoryCitation(nil), m.MemoryCitations...)
		}
		if m.Role == provider.RoleAssistant && m.ArtifactDelivery != nil {
			var paths []string
			if m.ArtifactDelivery.Paths != nil {
				paths = append([]string{}, m.ArtifactDelivery.Paths...)
			}
			hm.ArtifactDelivery = &provider.ArtifactDelivery{
				Version:     m.ArtifactDelivery.Version,
				Paths:       paths,
				DeliveredAt: m.ArtifactDelivery.DeliveredAt,
			}
		}
		if m.Role == provider.RoleUser && content != m.Content {
			hm.SubmitText = m.Content
		}
		if m.Role == provider.RoleAssistant && len(m.ToolCalls) > 0 {
			hm.ToolCalls = make([]HistoryToolCall, len(m.ToolCalls))
			for i, tc := range m.ToolCalls {
				args := tc.Arguments
				if tc.Name == "todo_write" {
					if replayed, ok := replayedTodoArgs[tc.ID]; ok {
						args = replayed
					}
				}
				hm.ToolCalls[i] = historyToolCall(tc, args, toolResults[tc.ID])
			}
		}
		if m.Role == provider.RoleTool {
			hm.ToolCallID = m.ToolCallID
			hm.ToolName = m.Name
			hm.Content, hm.ToolResultArchived, hm.ToolResultError = historyToolResultContent(m.Content, m.ToolCallID != "")
		}
		out = append(out, hm)
		if m.Role == provider.RoleUser {
			if turns := plannerByUserHash[messageDisplayKey(m.Content)]; len(turns) > 0 {
				out = append(out, cloneHistoryMessages(turns[0].Messages)...)
				plannerByUserHash[messageDisplayKey(m.Content)] = turns[1:]
			}
		}
	}
	return out
}

func plannerTurnsByUserHash(turns []plannerDisplayTurn) map[string][]plannerDisplayTurn {
	out := map[string][]plannerDisplayTurn{}
	for _, turn := range turns {
		if strings.TrimSpace(turn.UserHash) == "" || len(turn.Messages) == 0 {
			continue
		}
		out[turn.UserHash] = append(out[turn.UserHash], turn)
	}
	return out
}

func cloneHistoryMessages(in []HistoryMessage) []HistoryMessage {
	if len(in) == 0 {
		return nil
	}
	out := make([]HistoryMessage, len(in))
	copy(out, in)
	for i := range out {
		if in[i].Turn != nil {
			turn := *in[i].Turn
			out[i].Turn = &turn
		}
		if len(in[i].MemoryCitations) > 0 {
			out[i].MemoryCitations = append([]provider.MemoryCitation(nil), in[i].MemoryCitations...)
		}
		if in[i].ArtifactDelivery != nil {
			var paths []string
			if in[i].ArtifactDelivery.Paths != nil {
				paths = append([]string{}, in[i].ArtifactDelivery.Paths...)
			}
			out[i].ArtifactDelivery = &provider.ArtifactDelivery{
				Version:     in[i].ArtifactDelivery.Version,
				Paths:       paths,
				DeliveredAt: in[i].ArtifactDelivery.DeliveredAt,
			}
		}
		if len(in[i].ToolCalls) > 0 {
			out[i].ToolCalls = cloneHistoryToolCalls(in[i].ToolCalls)
		}
	}
	return out
}

func cloneHistoryToolCalls(in []HistoryToolCall) []HistoryToolCall {
	if len(in) == 0 {
		return nil
	}
	out := make([]HistoryToolCall, len(in))
	copy(out, in)
	for i := range out {
		out[i].ArtifactPaths = append([]string(nil), in[i].ArtifactPaths...)
		out[i].TaskLifecycle = cloneTaskLifecycleView(in[i].TaskLifecycle)
		out[i].TaskLifecycleRefs = append([]string(nil), in[i].TaskLifecycleRefs...)
		out[i].TaskLifecycleReceipts = cloneTaskLifecycleViews(in[i].TaskLifecycleReceipts)
		if len(in[i].SubagentMessages) > 0 {
			out[i].SubagentMessages = cloneHistoryMessages(in[i].SubagentMessages)
		}
	}
	return out
}

const historyToolPreviewLimit = 2_000

func historyToolCall(tc provider.ToolCall, args string, result provider.Message) HistoryToolCall {
	call := HistoryToolCall{
		ID:      tc.ID,
		Name:    tc.Name,
		Subject: historyToolSubject(tc.Name, args),
		Summary: historyToolSummary(tc.Name, args, result.Content),
		Diff:    tc.Diff,
		Added:   tc.Added,
		Removed: tc.Removed,
	}
	if tc.Name == deliverFilesToolName {
		if paths, ok := decodeDeliverFilesResult(result.Content); ok {
			call.ArtifactPaths = paths
		}
	}
	if tc.Name == "todo_write" {
		call.Arguments = args
		return call
	}
	if tc.ID == "" {
		call.Arguments = args
		return call
	}
	if args != "" {
		call.ArgumentsArchived = true
	}
	return call
}

func historyToolResultsByID(msgs []provider.Message) map[string]provider.Message {
	out := map[string]provider.Message{}
	for _, msg := range msgs {
		if msg.Role != provider.RoleTool || msg.ToolCallID == "" {
			continue
		}
		out[msg.ToolCallID] = msg
	}
	return out
}

func historyToolResultContent(content string, canArchive bool) (display string, archived bool, errPreview string) {
	if content == "" {
		return "", false, ""
	}
	if !canArchive {
		if historyToolResultFailed(content) {
			return content, false, content
		}
		return content, false, ""
	}
	if historyToolResultFailed(content) {
		display = clipHistoryToolPreview(strings.TrimSpace(content))
		return display, display != content, display
	}
	return "", true, ""
}

func clipHistoryToolPreview(s string) string {
	if len(s) <= historyToolPreviewLimit {
		return s
	}
	return strings.TrimSpace(clipStringBytes(s, historyToolPreviewLimit)) + "\n..."
}

func historyToolSubject(name, args string) string {
	a := parseHistoryToolArgs(args)
	var subject string
	switch name {
	case "bash":
		subject = historyArgString(a, "command")
	case "grep", "glob":
		subject = firstNonEmpty(historyArgString(a, "pattern"), historyArgString(a, "path"))
	case "web_fetch":
		subject = historyArgString(a, "url")
	case "task":
		subject = firstNonEmpty(historyArgString(a, "description"), historyArgString(a, "prompt"))
	case "move_file":
		src := historyArgString(a, "source_path")
		dst := historyArgString(a, "destination_path")
		if src != "" && dst != "" {
			subject = src + " -> " + dst
		} else {
			subject = firstNonEmpty(src, dst)
		}
	case "remember":
		subject = firstNonEmpty(historyArgString(a, "name"), historyArgString(a, "description"))
	case "todo_write", "exit_plan_mode":
		subject = ""
	default:
		subject = firstNonEmpty(historyArgString(a, "path"), historyArgString(a, "file_path"))
	}
	return clipSingleLine(subject, 240)
}

func historyToolSummary(name, args, output string) string {
	if historyToolResultFailed(output) {
		return ""
	}
	a := parseHistoryToolArgs(args)
	switch name {
	case "write_file":
		if content := historyArgString(a, "content"); content != "" {
			return fmt.Sprintf("%d lines", historyLineCount(content))
		}
	case "edit_file":
		oldText := historyArgString(a, "old_string")
		newText := historyArgString(a, "new_string")
		if oldText != "" || newText != "" {
			return fmt.Sprintf("%d -> %d lines", historyLineCount(oldText), historyLineCount(newText))
		}
	case "multi_edit":
		if edits, ok := a["edits"].([]any); ok && len(edits) > 0 {
			return fmt.Sprintf("%d edits", len(edits))
		}
	}
	if output == "" {
		return ""
	}
	switch name {
	case "read_file":
		if strings.HasPrefix(output, "(empty file)") {
			return "empty file"
		}
		if arrows := strings.Count(output, "→"); arrows > 0 {
			return fmt.Sprintf("%d lines", arrows)
		}
		return fmt.Sprintf("%d lines", historyLineCount(output))
	case "grep":
		return fmt.Sprintf("%d matches", historyNonEmptyLineCount(output))
	case "glob":
		return fmt.Sprintf("%d files", historyNonEmptyLineCount(output))
	case "ls":
		return fmt.Sprintf("%d entries", historyNonEmptyLineCount(output))
	case "web_fetch":
		return clipSingleLine(strings.SplitN(output, "\n", 2)[0], 80)
	case "bash":
		if strings.TrimSpace(output) == "" {
			return "no output"
		}
		return fmt.Sprintf("%d lines", historyLineCount(output))
	default:
		return ""
	}
}

func parseHistoryToolArgs(args string) map[string]any {
	if args == "" {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(args), &out); err != nil {
		return map[string]any{}
	}
	return out
}

func historyArgString(args map[string]any, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

func historyLineCount(s string) int {
	if s == "" {
		return 0
	}
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func historyNonEmptyLineCount(s string) int {
	count := 0
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func clipSingleLine(s string, max int) string {
	s = strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return clipStringBytes(s, max)
	}
	return clipStringBytes(s, max-3) + "..."
}

func clipStringBytes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	for max > 0 && !utf8.RuneStart(s[max]) {
		max--
	}
	return s[:max]
}

func historyTodoArgsWithCompleteSteps(msgs []provider.Message) map[string]string {
	successful := successfulHistoryToolCallIDs(msgs)
	out := map[string]string{}
	var todos []evidence.TodoItem
	latestTodoID := ""
	for _, m := range msgs {
		for _, tc := range m.ToolCalls {
			if tc.ID == "" || !successful[tc.ID] {
				continue
			}
			switch tc.Name {
			case "todo_write":
				rec := evidence.ReceiptFromToolCall(tc.Name, json.RawMessage(tc.Arguments), true, true)
				if len(rec.Todos) == 0 {
					continue
				}
				todos = append([]evidence.TodoItem(nil), rec.Todos...)
				latestTodoID = tc.ID
				if args, ok := todoArgsJSON(todos); ok {
					out[latestTodoID] = args
				}
			case "complete_step":
				if latestTodoID == "" || len(todos) == 0 {
					continue
				}
				rec := evidence.ReceiptFromToolCall(tc.Name, json.RawMessage(tc.Arguments), true, true)
				match, ok := evidence.MatchStep(rec.Step, todos)
				if !ok || match.Index < 1 || match.Index > len(todos) || todoStatusForHistory(todos[match.Index-1].Status) == "completed" {
					continue
				}
				todos[match.Index-1].Status = "completed"
				promoteNextHistoryTodo(todos)
				if args, ok := todoArgsJSON(todos); ok {
					out[latestTodoID] = args
				}
			}
		}
	}
	return out
}

func successfulHistoryToolCallIDs(msgs []provider.Message) map[string]bool {
	successful := map[string]bool{}
	for _, msg := range msgs {
		if msg.Role != provider.RoleTool || msg.ToolCallID == "" {
			continue
		}
		if !historyToolResultFailed(msg.Content) {
			successful[msg.ToolCallID] = true
		}
	}
	return successful
}

func historyToolResultFailed(content string) bool {
	content = strings.TrimSpace(content)
	return strings.HasPrefix(content, "error:") ||
		strings.HasPrefix(content, "blocked:") ||
		strings.HasPrefix(content, "Error:") ||
		strings.HasPrefix(content, "[error")
}

func todoArgsJSON(todos []evidence.TodoItem) (string, bool) {
	b, err := json.Marshal(map[string]any{"todos": todos})
	if err != nil {
		return "", false
	}
	return string(b), true
}

func promoteNextHistoryTodo(todos []evidence.TodoItem) {
	for _, todo := range todos {
		if todoStatusForHistory(todo.Status) == "in_progress" {
			return
		}
	}
	for i := range todos {
		if todoStatusForHistory(todos[i].Status) == "pending" {
			todos[i].Status = "in_progress"
			return
		}
	}
}

func todoStatusForHistory(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return "pending"
	}
	return status
}

func previewSessionMessages(sessionDir, path string) ([]HistoryMessage, error) {
	sessionPath, _, err := validateSessionPath(sessionDir, path)
	if err != nil {
		return nil, err
	}
	if agent.IsCleanupPending(sessionPath) {
		return nil, fmt.Errorf("session is pending cleanup")
	}
	if out, ok, err := previewEventSessionMessages(sessionPath); ok || err != nil {
		if err == nil {
			out = attachTaskSubagentRefs(out, sessionDir, agent.BranchID(sessionPath))
		}
		return out, err
	}
	loaded, err := agent.LoadSession(sessionPath)
	if err != nil {
		return nil, err
	}
	return historyMessagesForSession(sessionDir, sessionPath, loaded.Snapshot()), nil
}

type previewEventRecord struct {
	Kind             string                    `json:"kind"`
	Type             string                    `json:"type"`
	Role             string                    `json:"role"`
	MessageKind      provider.MessageKind      `json:"message_kind"`
	Time             json.RawMessage           `json:"time"`
	Timestamp        json.RawMessage           `json:"timestamp"`
	CreatedAt        json.RawMessage           `json:"createdAt"`
	CreatedAtSnake   json.RawMessage           `json:"created_at"`
	UpdatedAt        json.RawMessage           `json:"updatedAt"`
	UpdatedAtSnake   json.RawMessage           `json:"updated_at"`
	Text             string                    `json:"text"`
	Content          string                    `json:"content"`
	Reasoning        string                    `json:"reasoning"`
	ReasoningContent string                    `json:"reasoningContent"`
	MemoryCitations  []provider.MemoryCitation `json:"memoryCitations"`
	Level            string                    `json:"level"`
	ToolCalls        []previewToolCall         `json:"toolCalls"`
	CallID           string                    `json:"callId"`
	ToolCallID       string                    `json:"toolCallId"`
	ToolName         string                    `json:"toolName"`
	Name             string                    `json:"name"`
	Output           string                    `json:"output"`
	Compaction       *previewCompaction        `json:"compaction"`
	Trigger          string                    `json:"trigger"`
	Messages         int                       `json:"messages"`
	Summary          string                    `json:"summary"`
	Archive          string                    `json:"archive"`
}

type previewToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Function  struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type previewCompaction struct {
	Trigger  string `json:"trigger"`
	Messages int    `json:"messages"`
	Summary  string `json:"summary"`
	Archive  string `json:"archive"`
}

func previewEventSessionMessages(path string) ([]HistoryMessage, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	out := []HistoryMessage{}
	toolName := map[string]string{}
	sawEvent := false
	for {
		var rec previewEventRecord
		if err := dec.Decode(&rec); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			if sawEvent {
				return out, true, nil
			}
			return nil, false, nil
		}
		eventName := strings.TrimSpace(rec.Kind)
		if eventName == "" {
			eventName = strings.TrimSpace(rec.Type)
		}
		if eventName == "" {
			continue
		}
		sawEvent = true
		switch eventName {
		case "user.message":
			if rec.Text != "" {
				content := agent.UserPreviewText(rec.Text)
				if strings.TrimSpace(content) == "" {
					continue
				}
				hm := HistoryMessage{Role: "user", Content: content}
				if content != rec.Text {
					hm.SubmitText = rec.Text
				}
				out = append(out, hm)
			}
		case "model.final":
			if rec.MessageKind == provider.MessageKindReadinessRejectedCandidate {
				continue
			}
			hm := HistoryMessage{
				Role:        "assistant",
				Content:     rec.Content,
				Reasoning:   firstNonEmpty(rec.Reasoning, rec.ReasoningContent),
				FinalAnswer: rec.MessageKind == provider.MessageKindFinalAnswer,
			}
			if len(rec.MemoryCitations) > 0 {
				hm.MemoryCitations = append([]provider.MemoryCitation(nil), rec.MemoryCitations...)
			}
			for _, tc := range rec.ToolCalls {
				id := tc.ID
				name := firstNonEmpty(tc.Name, tc.Function.Name)
				args := firstNonEmpty(tc.Arguments, tc.Function.Arguments)
				hm.ToolCalls = append(hm.ToolCalls, historyToolCall(provider.ToolCall{ID: id, Name: name, Arguments: args}, args, provider.Message{}))
				if id != "" {
					toolName[id] = name
				}
			}
			out = append(out, hm)
		case "tool.result":
			callID := firstNonEmpty(rec.CallID, rec.ToolCallID)
			content := firstNonEmpty(rec.Output, rec.Content)
			display, archived, errPreview := historyToolResultContent(content, callID != "")
			if len(out) > 0 && callID != "" {
				updateHistoryToolCallSummary(out, callID, content)
			}
			out = append(out, HistoryMessage{
				Role:               "tool",
				ToolCallID:         callID,
				ToolName:           firstNonEmpty(rec.ToolName, rec.Name, toolName[callID]),
				Content:            display,
				ToolResultArchived: archived,
				ToolResultError:    errPreview,
			})
		case "phase":
			out = append(out, HistoryMessage{Role: "phase", Content: firstNonEmpty(rec.Text, rec.Content)})
		case "notice":
			level := rec.Level
			if level != "warn" {
				level = "info"
			}
			out = append(out, HistoryMessage{Role: "notice", Level: level, Content: firstNonEmpty(rec.Text, rec.Content)})
		case "compaction_started":
			c := rec.compactionPayload()
			out = append(out, HistoryMessage{Role: "compaction", Pending: true, Trigger: c.Trigger})
		case "compaction_done":
			c := rec.compactionPayload()
			out = append(out, HistoryMessage{
				Role:     "compaction",
				Trigger:  c.Trigger,
				Messages: c.Messages,
				Summary:  c.Summary,
				Archive:  c.Archive,
			})
		}
	}
	return out, sawEvent, nil
}

func (r previewEventRecord) compactionPayload() previewCompaction {
	if r.Compaction != nil {
		return *r.Compaction
	}
	return previewCompaction{Trigger: r.Trigger, Messages: r.Messages, Summary: r.Summary, Archive: r.Archive}
}

func updateHistoryToolCallSummary(out []HistoryMessage, callID, output string) {
	if callID == "" {
		return
	}
	for i := len(out) - 1; i >= 0; i-- {
		for j := range out[i].ToolCalls {
			call := &out[i].ToolCalls[j]
			if call.ID != callID {
				continue
			}
			if call.Summary == "" {
				call.Summary = historyToolSummary(call.Name, call.Arguments, output)
			}
			return
		}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// ContextInfo is the prompt-vs-window gauge payload plus session totals. Used
// and Window both zero means no context-window data yet.
type ContextInfo struct {
	Used                       int     `json:"used"`
	Window                     int     `json:"window"`
	SessionTokens              int     `json:"sessionTokens"`
	CompactRatio               float64 `json:"compactRatio,omitempty"`
	ActiveTurnStartedAt        int64   `json:"activeTurnStartedAt,omitempty"`
	ActiveTurnCompletionTokens int     `json:"activeTurnCompletionTokens,omitempty"`
}

// ContextUsage returns the latest context-window gauge numbers.
func (a *App) ContextUsage() ContextInfo {
	return a.ContextUsageForTab("")
}

func (a *App) ContextUsageForTab(tabID string) ContextInfo {
	a.mu.RLock()
	tab := a.tabByIDLocked(tabID)
	var ctrl control.SessionAPI
	if tab != nil {
		ctrl = tab.Ctrl
	}
	a.mu.RUnlock()

	var sessionTokens, activeTurnCompletionTokens int
	var activeTurnStartedAt int64
	if tab != nil {
		sessionTokens, activeTurnStartedAt, activeTurnCompletionTokens = tab.contextUsageSnapshot()
	}
	info := ContextInfo{
		SessionTokens:              sessionTokens,
		ActiveTurnStartedAt:        activeTurnStartedAt,
		ActiveTurnCompletionTokens: activeTurnCompletionTokens,
	}
	if ctrl == nil {
		return info
	}
	used, window := ctrl.ContextSnapshot()
	info.Used = used
	info.Window = window
	info.CompactRatio = ctrl.CompactRatio()
	return info
}

// BalanceInfo is the wallet-balance readout for the status bar. Available is true
// only when a balance was fetched; Display is the formatted amount (e.g. "¥110.00")
// and is "" when the active provider declares no balance_url — the frontend then
// omits the readout. Err carries a fetch failure for an optional tooltip.
type BalanceInfo struct {
	Available bool   `json:"available"`
	Display   string `json:"display"`
	Err       string `json:"err,omitempty"`
}

// Balance queries the active provider's wallet balance (a network call). It
// returns an empty (unavailable) readout when no provider balance_url is set, the
// controller is down, or the fetch fails — so the status bar simply shows nothing
// rather than an error.
func (a *App) Balance() BalanceInfo {
	return a.BalanceForTab("")
}

func (a *App) BalanceForTab(tabID string) BalanceInfo {
	a.mu.RLock()
	if tab := a.tabByIDLocked(tabID); tab != nil && modelbilling.BillingSource(tab.billingSource) == modelbilling.BillingSubscriptionPool {
		a.mu.RUnlock()
		return BalanceInfo{}
	}
	a.mu.RUnlock()
	ctrl := a.ctrlByTabID(tabID)
	if ctrl == nil {
		return BalanceInfo{}
	}
	b, err := ctrl.Balance(a.ctx)
	if err != nil {
		return BalanceInfo{Err: err.Error()}
	}
	if b == nil {
		return BalanceInfo{} // provider declares no balance endpoint
	}
	return BalanceInfo{Available: true, Display: b.Display()}
}

// JobView is one running background job (bash/task started with
// run_in_background) for the status-bar indicator.
type JobView struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Label     string `json:"label"`
	Status    string `json:"status"`
	StartedAt int64  `json:"startedAt"`
}

// Jobs returns the still-running background jobs for the status bar. It refreshes
// on demand (mount, turn end, and on each notice the frontend receives).
func (a *App) Jobs() []JobView {
	out := []JobView{}
	ctrl := a.ctrlByTabID("")
	return a.jobsForCtrl(ctrl, out)
}

func (a *App) JobsForTab(tabID string) []JobView {
	out := []JobView{}
	ctrl := a.ctrlByTabID(tabID)
	return a.jobsForCtrl(ctrl, out)
}

func (a *App) jobsForCtrl(ctrl control.SessionAPI, out []JobView) []JobView {
	if ctrl == nil {
		return out
	}
	for _, v := range ctrl.Jobs() {
		out = append(out, JobView{ID: v.ID, Kind: v.Kind, Label: v.Label, Status: v.Status, StartedAt: v.StartedAt})
	}
	return out
}

// Meta describes the session for the frontend's header and status line.
type Meta struct {
	Label             string              `json:"label"`
	ModelRef          string              `json:"modelRef,omitempty"`
	Ready             bool                `json:"ready"`
	StartupErr        string              `json:"startupErr,omitempty"`
	SendAvailability  TabSendAvailability `json:"sendAvailability"`
	EventChannel      string              `json:"eventChannel"`
	Cwd               string              `json:"cwd"`
	WorkspaceRoot     string              `json:"workspaceRoot,omitempty"`
	WorkspaceName     string              `json:"workspaceName,omitempty"`
	WorkspacePath     string              `json:"workspacePath,omitempty"`
	SessionPath       string              `json:"sessionPath,omitempty"`
	GitBranch         string              `json:"gitBranch,omitempty"`
	AutoApproveTools  bool                `json:"autoApproveTools"`
	Bypass            bool                `json:"bypass"` // legacy JSON key for YOLO/full-access tool auto-approval
	CollaborationMode string              `json:"collaborationMode"`
	ToolApprovalMode  string              `json:"toolApprovalMode"`
	TokenMode         string              `json:"tokenMode"`
	Goal              string              `json:"goal,omitempty"`
	GoalStatus        string              `json:"goalStatus,omitempty"`
}

// Meta reports the model label, readiness, any startup error, the working
// directory (for the status line), and the runtime event channel the frontend
// subscribes to.
func (a *App) Meta() Meta {
	return a.MetaForTab("")
}

func (a *App) MetaForTab(tabID string) Meta {
	tab := a.tabByID(tabID)
	if tab == nil {
		return Meta{EventChannel: eventChannel}
	}
	cwd := tab.WorkspaceRoot
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	autoApproveTools := tab.Ctrl != nil && tab.Ctrl.AutoApproveTools()
	collaborationMode := currentTabCollaborationMode(tab)
	toolApprovalMode := currentTabToolApprovalMode(tab)
	tokenMode := currentTabTokenMode(tab)
	goal := currentTabGoal(tab)
	goalStatus := currentTabGoalStatus(tab)
	return Meta{
		Label:             tab.Label,
		ModelRef:          tab.model,
		Ready:             tab.Ready,
		StartupErr:        tab.StartupErr,
		SendAvailability:  tabSendAvailability(tab),
		EventChannel:      eventChannel,
		Cwd:               cwd,
		WorkspaceRoot:     cwd,
		WorkspaceName:     tabWorkspaceName(tab, cwd),
		WorkspacePath:     cwd,
		SessionPath:       tab.currentSessionPath(),
		GitBranch:         workspaceGitBranchForMeta(cwd),
		AutoApproveTools:  autoApproveTools,
		Bypass:            autoApproveTools,
		CollaborationMode: collaborationMode,
		ToolApprovalMode:  toolApprovalMode,
		TokenMode:         tokenMode,
		Goal:              goal,
		GoalStatus:        goalStatus,
	}
}

func (a *App) SetGoal(goal string) {
	a.SetGoalForTab("", goal)
}

func (a *App) SetGoalForTab(tabID, goal string) {
	goal = strings.TrimSpace(goal)
	a.mu.Lock()
	tab := a.tabByIDLocked(tabID)
	if tab == nil {
		a.mu.Unlock()
		return
	}
	if !tab.Ready {
		a.mu.Unlock()
		return
	}
	tab.goal = goal
	if goal != "" {
		tab.mode = tabModeFromAxes(false, currentTabToolApprovalMode(tab) == control.ToolApprovalYolo)
	}
	ctrl := tab.Ctrl
	plan := tabModeHasPlan(tab.mode)
	tabIDForSave := tab.ID
	a.mu.Unlock()
	if ctrl != nil {
		ctrl.SetPlanMode(plan)
		ctrl.SetGoal(goal)
	}
	a.mu.Lock()
	if a.tabRuntimes[tabIDForSave] == tab {
		a.saveTabsLocked()
	}
	a.mu.Unlock()
}

func (a *App) ClearGoal() {
	a.SetGoal("")
}

func (a *App) ClearGoalForTab(tabID string) {
	a.SetGoalForTab(tabID, "")
}

// SetAutoApproveTools toggles YOLO/full-access tool auto-approval:
// approval-gated tool calls run without asking, while ask questions and plan
// approvals still wait for the user. The selected posture becomes the database-
// backed default for the next session, but is not written to config.
func (a *App) SetAutoApproveTools(on bool) {
	if on {
		a.SetToolApprovalModeForTab("", control.ToolApprovalYolo)
		return
	}
	a.SetToolApprovalModeForTab("", control.ToolApprovalAsk)
}

// SetBypass is the legacy Wails binding for SetAutoApproveTools.
func (a *App) SetBypass(on bool) {
	a.SetAutoApproveTools(on)
}

func (a *App) SetToolApprovalMode(mode string) {
	a.SetToolApprovalModeForTab("", mode)
}

// SyncToolApprovalModeForTab reapplies an already-selected posture to a rebuilt
// controller. It deliberately does not mutate the tab or session sidecar: UI
// hydration, model rebuilds, and pre-send synchronization are not user posture
// changes and must not backfill legacy sessions whose posture is missing.
func (a *App) SyncToolApprovalModeForTab(tabID, mode string) {
	mode = normalizeToolApprovalMode(mode)
	a.mu.RLock()
	tab := a.tabByIDLocked(tabID)
	var ctrl control.SessionAPI
	if tab != nil {
		ctrl = tab.Ctrl
	}
	a.mu.RUnlock()
	applyTabToolApprovalModeToController(ctrl, mode)
}

func (a *App) SetToolApprovalModeForTab(tabID, mode string) error {
	a.toolApprovalTransitionMu.Lock()
	defer a.toolApprovalTransitionMu.Unlock()

	mode = normalizeToolApprovalMode(mode)
	a.mu.RLock()
	tab := a.tabByIDLocked(tabID)
	if tab == nil {
		a.mu.RUnlock()
		return fmt.Errorf("tab is not available")
	}
	if tab.ReadOnly {
		a.mu.RUnlock()
		return readOnlyChannelErr()
	}
	if !tab.Ready {
		a.mu.RUnlock()
		return workspaceNotReadyErr(tab)
	}
	targetPath := canonicalTabSessionPath(tab.currentSessionPath())
	targetKey := sessionRuntimeKey(targetPath)
	targetModel := strings.TrimSpace(tab.model)
	targetBillingSource := strings.TrimSpace(tab.billingSource)
	targetEffort := cloneStringPtr(tab.effort)
	targetActor := tabRuntimeActorContext(tab)
	a.mu.RUnlock()

	if targetPath != "" {
		if err := a.authorizeProjectSessionMutation(targetPath, false); err != nil {
			return err
		}
		if err := saveSessionToolApprovalMode(targetPath, targetModel, targetBillingSource, targetEffort, mode); err != nil {
			return fmt.Errorf("persist session tool approval mode: %w", err)
		}
	}
	preferenceStore := newDesktopTabStore(desktopConfigDirForPaths(config.NewProfilePathResolver(targetActor)))
	if err := preferenceStore.setDefaultToolApprovalMode(mode); err != nil {
		return fmt.Errorf("persist default tool approval mode: %w", err)
	}

	a.mu.Lock()
	if a.tabRuntimes[tab.ID] != tab || sessionRuntimeKey(tab.currentSessionPath()) != targetKey {
		a.mu.Unlock()
		return fmt.Errorf("session changed while saving tool approval mode")
	}
	tab.toolApprovalMode = mode
	tab.toolApprovalModeKnown = true
	tab.mode = tabModeFromAxes(tabModeHasPlan(currentTabMode(tab)), mode == control.ToolApprovalYolo)
	ctrl := tab.Ctrl
	applyTabToolApprovalModeToController(ctrl, mode)
	a.saveTabsLocked()
	a.mu.Unlock()
	return nil
}

// CommandInfo describes one available slash command for the composer's "/" menu.
type CommandInfo struct {
	Name        string `json:"name"` // without the leading slash
	Description string `json:"description"`
	Hint        string `json:"hint,omitempty"` // argument hint, if any
	Kind        string `json:"kind"`           // "builtin" | "custom" | "mcp"
}

// Commands lists the slash commands available this session — built-in actions,
// custom commands (.clouvia/commands plus compatible legacy convention dirs),
// and MCP prompts — for the composer's "/" autocomplete menu.
func (a *App) Commands() []CommandInfo {
	out := []CommandInfo{
		{Name: "time", Description: i18n.M.CmdTime, Kind: "builtin"},
		{Name: "new", Description: i18n.M.CmdNew, Kind: "builtin"},
		{Name: "clear", Description: i18n.M.CmdClear, Kind: "builtin"},
		{Name: "compact", Description: i18n.M.CmdCompact, Kind: "builtin"},
		{Name: "model", Description: i18n.M.CmdModel, Kind: "builtin"},
		{Name: "provider", Description: i18n.M.CmdProvider, Kind: "builtin"},
		{Name: "effort", Description: i18n.M.CmdEffort, Kind: "builtin"},
		{Name: "memory", Description: i18n.M.CmdMemory, Kind: "builtin"},
		{Name: "migrate", Description: i18n.M.CmdMigrate, Kind: "builtin"},
		{Name: "goal", Description: i18n.M.CmdGoal, Kind: "builtin"},
		{Name: "remember", Description: i18n.M.CmdRemember, Kind: "builtin"},
		{Name: "mcp", Description: i18n.M.CmdMcp, Kind: "builtin"},
		{Name: "theme", Description: i18n.M.CmdTheme, Kind: "builtin"},
		{Name: "skill", Description: i18n.M.CmdSkill, Kind: "builtin"},
		{Name: "reload-cmd", Description: i18n.M.CmdReloadCmd, Kind: "builtin"},
	}
	a.mu.RLock()
	ctrl := a.activeCtrlLocked()
	a.mu.RUnlock()
	if ctrl == nil {
		return out
	}
	// Skills are invocable as /<name> and slash invocation always expands the
	// skill inline. Listing them here is what surfaces /init, /explore, … in the
	// composer's slash menu; selecting one submits "/<name>", which the controller
	// resolves via RunSkill.
	for _, s := range ctrl.Skills() {
		out = append(out, CommandInfo{Name: s.Name, Description: s.Description, Kind: "skill"})
	}
	for _, c := range ctrl.Commands() {
		out = append(out, CommandInfo{Name: c.Name, Description: c.Description, Hint: c.ArgHint, Kind: "custom"})
	}
	if h := ctrl.Host(); h != nil {
		for _, p := range h.Prompts() {
			out = append(out, CommandInfo{Name: p.Name, Description: p.Description, Kind: "mcp"})
		}
	}
	return out
}

// SlashArgItem is one sub-command / argument suggestion for the composer's slash
// menu (the part after the command word). Mirrors the CLI's arg completion via
// the shared control.SlashArgItems, so desktop and CLI offer the same hints.
type SlashArgItem struct {
	Label   string `json:"label"`
	Insert  string `json:"insert"`
	Hint    string `json:"hint"`
	Descend bool   `json:"descend"`
}

// SlashArgsResult carries the suggestions plus the byte offset in the input where
// the current token begins, so the composer replaces just that token.
type SlashArgsResult struct {
	Items []SlashArgItem `json:"items"`
	From  int            `json:"from"`
}

// SlashArgs completes the arguments of a management slash command (/mcp, /model,
// /skill) for the composer — the same logic the chat TUI uses. Empty
// Items means the input has no structured arguments to complete.
func (a *App) SlashArgs(input string) SlashArgsResult {
	a.mu.RLock()
	ctrl := a.activeCtrlLocked()
	model := ""
	tabID := ""
	if tab := a.activeTabLocked(); tab != nil {
		model = tab.model
		tabID = tab.ID
	}
	a.mu.RUnlock()
	if ctrl == nil {
		return SlashArgsResult{Items: []SlashArgItem{}}
	}
	effort := a.EffortForTab(tabID)
	data := control.ArgData{
		Skills:          ctrl.Skills(),
		DisabledSkills:  ctrl.DisabledSkills(),
		ConfiguredMCP:   ctrl.ConfiguredMCPNames(),
		DisconnectedMCP: ctrl.DisconnectedMCPNames(),
		CurrentModel:    model,
		Experts:         ctrl.ListExperts(),
		CurrentExpert:   ctrl.CurrentExpert().SelectedID,
	}
	if effort.Supported {
		data.EffortLevels = append([]string(nil), effort.Levels...)
	}
	seen := map[string]bool{}
	for _, m := range a.Models() {
		data.ModelRefs = append(data.ModelRefs, m.Ref)
		if m.Provider != "" && !seen[m.Provider] {
			seen[m.Provider] = true
			data.ProviderNames = append(data.ProviderNames, m.Provider)
		}
		if m.Current {
			data.CurrentProvider = m.Provider
		}
	}
	if h := ctrl.Host(); h != nil {
		data.ServerNames = h.ServerNames()
	}
	items, from := control.SlashArgItems(input, data)
	// Non-nil so it serializes as a JSON array, never null — the frontend filters
	// over it directly.
	out := SlashArgsResult{Items: []SlashArgItem{}, From: from}
	for _, it := range items {
		if !desktopSlashArgSupported(input, from, it.Label) {
			continue
		}
		out.Items = append(out.Items, SlashArgItem{Label: it.Label, Insert: it.Insert, Hint: it.Hint, Descend: it.Descend})
	}
	return out
}

func desktopSlashArgSupported(input string, from int, label string) bool {
	if from < 0 || from > len(input) {
		return false
	}
	prior := strings.Fields(input[:from])
	if len(prior) == 0 || prior[0] != "/mcp" {
		return true
	}
	if len(prior) == 1 {
		return label == "connect"
	}
	return prior[1] == "connect"
}

// CapabilitiesView is the MCP & Skills drawer's data: connected/failed MCP
// servers and the discoverable skills, the GUI counterpart to `/mcp` + `/skill`.
type CapabilitiesView struct {
	Servers    []ServerView    `json:"servers"`
	Skills     []SkillView     `json:"skills"`
	SkillRoots []SkillRootView `json:"skillRoots"`
}

// SkillsSettingsView is the skills management page's data, split from MCP
// status so opening MCP settings does not scan skill roots.
type SkillsSettingsView struct {
	Skills     []SkillView     `json:"skills"`
	SkillRoots []SkillRootView `json:"skillRoots"`
}

// ServerView is one MCP server for the drawer. Status is "connected" (with
// tool/prompt/resource counts), "deferred" (enabled but idle), "failed" (with
// the connection error), "initializing" (background startup in progress), or
// "disabled".
type ServerView struct {
	Name                 string     `json:"name"`
	Transport            string     `json:"transport"`
	Status               string     `json:"status"`
	StartIntent          string     `json:"startIntent,omitempty"`
	RuntimeState         string     `json:"runtimeState,omitempty"`
	BuiltIn              bool       `json:"builtIn,omitempty"`
	Configured           bool       `json:"configured,omitempty"`
	AutoStart            bool       `json:"autoStart"`
	Tier                 string     `json:"tier,omitempty"`
	Command              string     `json:"command,omitempty"`
	Args                 []string   `json:"args,omitempty"`
	URL                  string     `json:"url,omitempty"`
	EnvKeys              []string   `json:"envKeys,omitempty"`
	HeaderKeys           []string   `json:"headerKeys,omitempty"`
	Tools                int        `json:"tools"`
	Prompts              int        `json:"prompts"`
	Resources            int        `json:"resources"`
	Error                string     `json:"error,omitempty"`
	ToolList             []ToolView `json:"toolList,omitempty"`
	TrustedReadOnlyTools []string   `json:"trustedReadOnlyTools,omitempty"`
	AuthStatus           string     `json:"authStatus,omitempty"`
	AuthURL              string     `json:"authUrl,omitempty"`
	AuthConfigured       bool       `json:"authConfigured,omitempty"`
}

type ToolView struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	ReadOnlyHint bool   `json:"readOnlyHint,omitempty"`
}

// SkillView is one discoverable skill for the drawer.
type SkillView struct {
	Name          string `json:"name"`
	DisplayName   string `json:"displayName,omitempty"`
	Description   string `json:"description"`
	Scope         string `json:"scope"`
	RunAs         string `json:"runAs"`
	Enabled       bool   `json:"enabled"`
	IconURL       string `json:"iconUrl,omitempty"`
	UninstallID   string `json:"uninstallId,omitempty"`
	Uninstallable bool   `json:"uninstallable,omitempty"`
}

type SkillRootSkillView struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Scope       string `json:"scope"`
	RunAs       string `json:"runAs"`
}

// SkillRootView is one skill discovery root for the drawer's Sources section.
type SkillRootView struct {
	Dir        string               `json:"dir"`
	Scope      string               `json:"scope"`
	Priority   int                  `json:"priority"`
	Status     string               `json:"status"`
	Configured bool                 `json:"configured"`
	Removable  bool                 `json:"removable"`
	Skills     int                  `json:"skills"`
	SkillItems []SkillRootSkillView `json:"skillItems,omitempty"`
	Warning    string               `json:"warning,omitempty"`
}

// Capabilities projects the session's MCP servers (connected + failed) and skills
// for the MCP & Skills drawer. Non-nil slices so the frontend can map over them.
func (a *App) Capabilities() CapabilitiesView {
	skills := a.SkillsSettings()
	return CapabilitiesView{
		Servers:    a.MCPServers(),
		Skills:     skills.Skills,
		SkillRoots: skills.SkillRoots,
	}
}

// MCPServers returns only MCP server status for settings pages that do not need
// skill discovery.
func (a *App) MCPServers() []ServerView {
	return a.mcpServersView()
}

// SkillsSettings returns the skills management snapshot without MCP status.
func (a *App) SkillsSettings() SkillsSettingsView {
	out := SkillsSettingsView{Skills: []SkillView{}, SkillRoots: []SkillRootView{}}
	a.mu.RLock()
	tab := a.activeTabLocked()
	var ctrl control.SessionAPI
	workspaceRoot := "."
	skillRootsWorkspaceRoot := ""
	var configActor userctx.ActorContext
	billingSource := modelbilling.BillingSource("")
	if tab != nil {
		ctrl = tab.Ctrl
		skillRootsWorkspaceRoot = strings.TrimSpace(tab.WorkspaceRoot)
		if skillRootsWorkspaceRoot == "" && ctrl != nil {
			skillRootsWorkspaceRoot = strings.TrimSpace(ctrl.WorkspaceRoot())
		}
		workspaceRoot = tabActorWorkspaceRoot(tab)
		if strings.TrimSpace(workspaceRoot) == "" && ctrl != nil {
			workspaceRoot = ctrl.WorkspaceRoot()
		}
		if strings.TrimSpace(workspaceRoot) == "" {
			workspaceRoot = "."
		}
		configActor = actorContextForWorkspace(tabRuntimeActorContext(tab), workspaceRoot)
		billingSource = modelbilling.BillingSource(tab.billingSource)
		if a.currentActorExplicit {
			configActor = a.actorForWorkspaceLocked(workspaceRoot)
		}
	}
	a.mu.RUnlock()

	disabled := map[string]bool{}
	cfg, _, cfgErr := desktopCatalogConfigForBillingRoute(workspaceRoot, configActor, billingSource)
	if cfgErr == nil {
		for _, name := range cfg.Skills.DisabledSkills {
			if key := config.SkillNameKey(name); key != "" {
				disabled[key] = true
			}
		}
	}

	var allSkills []skill.Skill
	if ctrl != nil {
		allSkills = ctrl.AllSkills()
	} else {
		projectRoot := strings.TrimSpace(skillRootsWorkspaceRoot)
		if projectRoot == "" {
			projectRoot = workspaceRoot
		}
		opts := skill.Options{
			KWUHomeDir:  config.NewProfilePathResolver(configActor).KWUHomeDir(),
			ProjectRoot: projectRoot,
			Stderr:      io.Discard,
		}
		if cfg != nil {
			opts.CustomPaths = cfg.SkillCustomPaths()
			opts.ExcludedPaths = cfg.SkillExcludedPaths()
			opts.MaxDepth = cfg.SkillMaxDepth()
		}
		allSkills = skill.New(opts).List()
	}
	managedSkillsRoot := filepath.Join(config.NewProfilePathResolver(configActor).KWUHomeDir(), skill.SkillsDirname)
	for _, s := range allSkills {
		metadata, hasInstallMetadata := installedPublicSkillMetadata(s.Path)
		uninstallable := hasInstallMetadata &&
			config.CanonicalSkillPath(filepath.Dir(filepath.Dir(s.Path))) == config.CanonicalSkillPath(managedSkillsRoot) &&
			strings.EqualFold(filepath.Base(filepath.Dir(s.Path)), strings.TrimSpace(metadata.Slug))
		uninstallID := ""
		if uninstallable {
			uninstallID = strings.TrimSpace(metadata.Slug)
		}
		out.Skills = append(out.Skills, SkillView{
			Name: s.Name, DisplayName: installedSkillDisplayName(s.Path, s.Name, metadata), Description: s.Description,
			Scope: string(s.Scope), RunAs: string(s.RunAs),
			Enabled: !disabled[config.SkillNameKey(s.Name)], IconURL: strings.TrimSpace(metadata.IconURL),
			UninstallID: uninstallID, Uninstallable: uninstallable,
		})
	}
	out.SkillRoots = a.cachedSkillRootsViewForRoute(skillRootsWorkspaceRoot, configActor, billingSource)
	return out
}

func (a *App) mcpServersView() []ServerView {
	out := []ServerView{}
	a.mu.RLock()
	tab := a.activeTabLocked()
	if tab == nil {
		a.mu.RUnlock()
		return out
	}
	ctrl := tab.Ctrl
	disabled := make(map[string]ServerView, len(tab.disabledMCP))
	for name, s := range tab.disabledMCP {
		disabled[name] = s
	}
	order := append([]string(nil), tab.mcpOrder...)
	workspaceRoot := tab.WorkspaceRoot
	configActor := tabRuntimeActorContext(tab)
	billingSource := modelbilling.BillingSource(tab.billingSource)
	if a.currentActorExplicit {
		configActor = a.actorForWorkspaceLocked(workspaceRoot)
	}
	tabID := tab.ID
	a.mu.RUnlock()
	if ctrl == nil {
		return out
	}
	seen := map[string]bool{}
	connected := map[string]bool{}
	retainedDisabled := map[string]ServerView{}
	configured := map[string]config.PluginEntry{}
	var configuredEntries []config.PluginEntry
	if cfg, _, err := desktopCatalogConfigForBillingRoute(workspaceRoot, configActor, billingSource); err == nil {
		for _, p := range cfg.Plugins {
			if a.managedConnectorConfigName(p.Name) {
				continue
			}
			configuredEntries = append(configuredEntries, p)
			configured[p.Name] = p
		}
	}
	if h := ctrl.Host(); h != nil {
		for _, s := range h.Servers() {
			if disabledView, ok := disabled[s.Name]; ok {
				disabledView.Status = "disabled"
				disabledView.RuntimeState = "idle"
				disabledView.StartIntent = "off"
				disabledView.Error = ""
				if p, ok := configured[s.Name]; ok {
					disabledView = withPluginConfig(disabledView, p)
				}
				out = append(out, disabledView)
				retainedDisabled[s.Name] = disabledView
				seen[s.Name] = true
				delete(disabled, s.Name)
				continue
			}
			seen[s.Name] = true
			connected[s.Name] = true
			view := ServerView{
				Name: s.Name, Transport: s.Transport, Status: "connected", RuntimeState: "ready",
				Tools: s.Tools, Prompts: s.Prompts, Resources: s.Resources,
				ToolList: pluginToolsToView(s.ToolList),
			}
			if p, ok := configured[s.Name]; ok {
				view = withPluginConfig(view, p)
			}
			out = append(out, view)
		}
		for _, f := range h.Failures() {
			seen[f.Name] = true
			view := ServerView{
				Name: f.Name, Transport: f.Transport, Status: "failed", RuntimeState: "issue", Error: f.Error,
			}
			if p, ok := configured[f.Name]; ok {
				view = withPluginConfig(view, p)
			}
			out = append(out, view)
		}
		for _, name := range h.ConnectingServers() {
			if seen[name] {
				continue
			}
			seen[name] = true
			view := ServerView{Name: name, Status: "initializing", RuntimeState: "connecting"}
			if p, ok := configured[name]; ok {
				view = withPluginConfig(view, p)
			}
			out = append(out, view)
		}
	}
	// Configured servers that are neither connected, connecting, nor failed are
	// idle: disabled/off or automatic background startup waiting for its next kick.
	if len(configuredEntries) > 0 {
		for _, p := range configuredEntries {
			if seen[p.Name] {
				continue
			}
			if s, ok := disabled[p.Name]; ok {
				s.Status = "disabled"
				s.RuntimeState = "idle"
				s.StartIntent = "off"
				s = withPluginConfig(s, p)
				s.Error = ""
				out = append(out, s)
				retainedDisabled[p.Name] = s
				seen[p.Name] = true
				delete(disabled, p.Name)
				continue
			}
			status := "disabled"
			startIntent := "off"
			if p.ShouldAutoStart() {
				status = "deferred"
				startIntent = mcpStartIntent(p)
			}
			out = append(out, withPluginConfig(ServerView{Name: p.Name, Status: status, StartIntent: startIntent, RuntimeState: "idle"}, p))
			seen[p.Name] = true
		}
	}
	out = orderServerViews(out, order)

	a.mu.Lock()
	if tab, ok := a.tabRuntimes[tabID]; ok {
		for name := range connected {
			delete(retainedDisabled, name)
		}
		tab.disabledMCP = retainedDisabled
		tab.mcpOrder = mergeServerOrder(tab.mcpOrder, out)
	}
	a.mu.Unlock()
	return out
}

func mcpStartIntent(p config.PluginEntry) string {
	if !p.ShouldAutoStart() {
		return "off"
	}
	return "automatic"
}

func mcpRuntimeState(status string) string {
	switch status {
	case "connected":
		return "ready"
	case "initializing":
		return "connecting"
	case "failed":
		return "issue"
	default:
		return "idle"
	}
}

func withPluginConfig(v ServerView, p config.PluginEntry) ServerView {
	tt := p.Type
	if tt == "" {
		tt = "stdio"
	}
	v.Transport = tt
	v.Configured = true
	v.AutoStart = p.ShouldAutoStart()
	v.Tier = p.ResolvedTier()
	if v.StartIntent == "" {
		v.StartIntent = mcpStartIntent(p)
	}
	if v.Status == "disabled" {
		v.StartIntent = "off"
	}
	if v.RuntimeState == "" {
		v.RuntimeState = mcpRuntimeState(v.Status)
	}
	v.Command = p.Command
	v.Args = append([]string(nil), p.Args...)
	v.URL = p.URL
	v.TrustedReadOnlyTools = uniqueStrings(p.TrustedReadOnlyTools)
	v.AuthConfigured = mcpdiag.HasAuthConfig(p.Headers, p.Env, p.URL)
	v.EnvKeys = nil
	v.HeaderKeys = nil
	if len(p.Env) > 0 {
		v.EnvKeys = make([]string, 0, len(p.Env))
		for k := range p.Env {
			v.EnvKeys = append(v.EnvKeys, k)
		}
		sort.Strings(v.EnvKeys)
	}
	if len(p.Headers) > 0 {
		v.HeaderKeys = make([]string, 0, len(p.Headers))
		for k := range p.Headers {
			v.HeaderKeys = append(v.HeaderKeys, k)
		}
		sort.Strings(v.HeaderKeys)
	}
	auth := mcpdiag.DiagnoseAuth(v.Transport, v.Status, v.Error, v.URL, v.AuthConfigured)
	v.AuthStatus = auth.Status
	v.AuthURL = auth.URL
	return v
}

const skillRootsCacheTTL = 10 * time.Second

func (a *App) cachedSkillRootsView(workspaceRoot string, actor userctx.ActorContext) []SkillRootView {
	return a.cachedSkillRootsViewForRoute(workspaceRoot, actor, "")
}

func (a *App) cachedSkillRootsViewForRoute(workspaceRoot string, actor userctx.ActorContext, source modelbilling.BillingSource) []SkillRootView {
	cwd := strings.TrimSpace(workspaceRoot)
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	actor = actorContextForWorkspace(actor, cwd)
	paths := config.NewProfilePathResolver(actor)
	cfg, metadataOnly, _ := desktopCatalogConfigForBillingRoute(cwd, actor, source)
	var userCfg *config.Config
	if metadataOnly {
		userCfg = config.LoadForEditWithoutCredentialsWithPaths(paths)
	} else {
		userCfg = config.LoadForEditWithPaths(paths)
	}
	key := skillRootsCacheKey(cwd, actor.ProfileID, cfg, userCfg)

	now := time.Now()
	a.skillRootsMu.Lock()
	if a.skillRootsCache.key == key && now.Sub(a.skillRootsCache.at) < skillRootsCacheTTL {
		roots := cloneSkillRootViews(a.skillRootsCache.roots)
		a.skillRootsMu.Unlock()
		return roots
	}
	a.skillRootsMu.Unlock()

	roots := skillRootsViewFrom(cwd, cfg, userCfg, paths.KWUHomeDir())

	a.skillRootsMu.Lock()
	a.skillRootsCache = skillRootsCache{
		key:   key,
		at:    now,
		roots: cloneSkillRootViews(roots),
	}
	a.skillRootsMu.Unlock()
	return roots
}

func (a *App) invalidateSkillRootsCache() {
	a.skillRootsMu.Lock()
	a.skillRootsCache = skillRootsCache{}
	a.skillRootsMu.Unlock()
}

func skillRootsView() []SkillRootView {
	cwd, _ := os.Getwd()
	var cfg *config.Config
	var userCfg *config.Config
	if desktopCloudFakeEnabled() {
		cfg, _ = config.LoadForRootMetadataWithActor(cwd, userctx.Default(cwd))
		userCfg = config.LoadForEditWithoutCredentials(config.UserConfigPath())
	} else {
		cfg, _ = config.Load()
		userCfg = config.LoadForEdit(config.UserConfigPath())
	}
	return skillRootsViewFrom(cwd, cfg, userCfg, "")
}

func skillRootsViewFrom(cwd string, cfg, userCfg *config.Config, kwuHomeDir string) []SkillRootView {
	var custom []string
	var excluded []string
	maxDepth := 3
	if cfg != nil {
		custom = cfg.SkillCustomPaths()
		excluded = cfg.SkillExcludedPaths()
		maxDepth = cfg.SkillMaxDepth()
	}
	st := skill.New(skill.Options{KWUHomeDir: kwuHomeDir, ProjectRoot: cwd, CustomPaths: custom, ExcludedPaths: excluded, MaxDepth: maxDepth, DisableBuiltins: true, Stderr: io.Discard})
	counts := map[string]int{}
	skillItems := map[string][]SkillRootSkillView{}
	roots := st.Roots()
	for _, sk := range st.List() {
		root := skillDisplayRoot(sk, roots)
		counts[root]++
		skillItems[root] = append(skillItems[root], SkillRootSkillView{
			Name:        sk.Name,
			Description: sk.Description,
			Scope:       string(sk.Scope),
			RunAs:       string(sk.RunAs),
		})
	}
	for root := range skillItems {
		sort.Slice(skillItems[root], func(i, j int) bool {
			return skillItems[root][i].Name < skillItems[root][j].Name
		})
	}
	userConfigured := map[string]bool{}
	if userCfg != nil {
		for _, p := range userCfg.Skills.Paths {
			userConfigured[config.CanonicalSkillPath(p)] = true
		}
	}
	out := []SkillRootView{}
	seenRoots := map[string]int{}
	for _, r := range roots {
		dir := config.CanonicalSkillPath(r.Dir)
		view := SkillRootView{
			Dir:        r.Dir,
			Scope:      string(r.Scope),
			Priority:   r.Priority + 1,
			Status:     string(r.Status),
			Configured: r.Scope == skill.ScopeCustom && userConfigured[dir],
			Removable:  true,
			Skills:     counts[dir],
			SkillItems: skillItems[dir],
		}
		if idx, ok := seenRoots[dir]; ok {
			out[idx] = mergeDuplicateSkillRootView(out[idx], view)
			continue
		}
		seenRoots[dir] = len(out)
		out = append(out, view)
	}
	if userCfg != nil {
		for _, p := range userCfg.Skills.Paths {
			if rootActive(out, p) {
				continue
			}
			out = append(out, SkillRootView{
				Dir:        p,
				Scope:      string(skill.ScopeCustom),
				Status:     "inactive",
				Configured: true,
				Removable:  true,
				Warning:    "configured in user config but not active in this workspace; project [skills].paths may override it",
			})
		}
	}
	return out
}

func mergeDuplicateSkillRootView(existing, duplicate SkillRootView) SkillRootView {
	existing.Configured = existing.Configured || duplicate.Configured
	existing.Removable = existing.Removable || duplicate.Removable
	if existing.Status != "ok" && duplicate.Status == "ok" {
		existing.Status = duplicate.Status
	}
	if existing.Skills == 0 && duplicate.Skills > 0 {
		existing.Skills = duplicate.Skills
		existing.SkillItems = duplicate.SkillItems
	}
	if existing.Warning == "" {
		existing.Warning = duplicate.Warning
	}
	return existing
}

func skillRootsCacheKey(cwd, profileID string, cfg, userCfg *config.Config) string {
	type cacheKey struct {
		CWD       string   `json:"cwd"`
		ProfileID string   `json:"profileId"`
		Custom    []string `json:"custom"`
		Excluded  []string `json:"excluded"`
		MaxDepth  int      `json:"maxDepth"`
		UserPaths []string `json:"userPaths"`
	}
	key := cacheKey{CWD: config.CanonicalSkillPath(cwd), ProfileID: profileID, MaxDepth: 3}
	if cfg != nil {
		key.Custom = canonicalSkillPaths(cfg.SkillCustomPaths())
		key.Excluded = canonicalSkillPaths(cfg.SkillExcludedPaths())
		key.MaxDepth = cfg.SkillMaxDepth()
	}
	if userCfg != nil {
		key.UserPaths = canonicalSkillPaths(userCfg.Skills.Paths)
	}
	b, err := json.Marshal(key)
	if err != nil {
		return fmt.Sprintf("%s|%s|%v|%v|%d|%v", key.CWD, key.ProfileID, key.Custom, key.Excluded, key.MaxDepth, key.UserPaths)
	}
	return string(b)
}

func canonicalSkillPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		out = append(out, config.CanonicalSkillPath(p))
	}
	sort.Strings(out)
	return out
}

func cloneSkillRootViews(in []SkillRootView) []SkillRootView {
	out := make([]SkillRootView, len(in))
	for i, r := range in {
		out[i] = r
		out[i].SkillItems = append([]SkillRootSkillView(nil), r.SkillItems...)
	}
	return out
}

func rootActive(roots []SkillRootView, path string) bool {
	want := config.CanonicalSkillPath(path)
	for _, r := range roots {
		if config.CanonicalSkillPath(r.Dir) == want {
			return true
		}
	}
	return false
}

// PickSkillFolder opens a directory picker for adding custom skill roots. It only
// returns a path; AddSkillPath performs normalization and writes config.
func (a *App) PickSkillFolder() (string, error) {
	if a.ctx == nil {
		return "", nil
	}
	cur, _ := os.Getwd()
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Choose skills folder",
		DefaultDirectory: dialogDefaultDirectory(cur),
	})
	if err != nil || dir == "" {
		return "", err
	}
	return normalizeSkillPath(dir), nil
}

// AddSkillPath adds a custom skill root to the user config and rebuilds the
// controller so the skills index and slash menu reflect it immediately.
func (a *App) AddSkillPath(path string) error {
	path = normalizeSkillPath(path)
	workspaceRoot := a.activeWorkspaceRoot()
	err := a.applyConfigChange(func(c *config.Config) error {
		if isConventionSkillRoot(path, workspaceRoot) {
			return c.RestoreSkillPath(path)
		}
		return c.AddSkillPath(path)
	})
	if err == nil {
		a.invalidateSkillRootsCache()
	}
	return err
}

// RemoveSkillPath removes a skill source from the user config and rebuilds. For
// convention roots, it records a pseudo-delete in excluded_paths.
func (a *App) RemoveSkillPath(path string) error {
	path = normalizeSkillPath(path)
	err := a.applyConfigChange(func(c *config.Config) error {
		removed, err := c.RemoveSkillPath(path)
		if err != nil || removed {
			return err
		}
		return c.ExcludeSkillPath(path)
	})
	if err == nil {
		a.invalidateSkillRootsCache()
	}
	return err
}

// RefreshSkills rebuilds the controller without changing config, reloading skill
// discovery, the system prompt index, and slash completions.
func (a *App) RefreshSkills() error {
	a.invalidateSkillRootsCache()
	return a.rebuild()
}

// ReloadCommands rescans command directories and hot-swaps without restarting
// the controller — no MCP disconnect, no hook rerun.
func (a *App) ReloadCommands() error {
	if a.ctx == nil {
		return nil
	}
	tab := a.activeTab()
	if tab == nil || tab.Ctrl == nil {
		return fmt.Errorf("no active session")
	}
	status := tab.Ctrl.RuntimeStatus()
	if status.Running || status.Completing {
		return fmt.Errorf("wait for the current turn to finish, then retry")
	}
	return tab.Ctrl.ReloadCommands(a.ctx)
}

// SetSkillEnabled persists a skill toggle and rebuilds the controller so the
// prompt index, slash menu, and skill tools reflect it immediately.
func (a *App) SetSkillEnabled(name string, enabled bool) error {
	err := a.applyConfigChange(func(c *config.Config) error {
		return c.SetSkillEnabled(name, enabled)
	})
	if err == nil {
		a.invalidateSkillRootsCache()
	}
	return err
}

func normalizeSkillPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			if path == "~" {
				path = home
			} else {
				path = filepath.Join(home, path[2:])
			}
		}
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	info, err := os.Stat(path)
	if err != nil {
		return filepath.Clean(path)
	}
	if info.Mode().IsRegular() {
		if filepath.Base(path) == skill.SkillFile {
			return filepath.Clean(filepath.Dir(filepath.Dir(path)))
		}
		return filepath.Clean(filepath.Dir(path))
	}
	if info.IsDir() {
		if _, err := os.Stat(filepath.Join(path, skill.SkillFile)); err == nil {
			return filepath.Clean(filepath.Dir(path))
		}
	}
	return filepath.Clean(path)
}

func isConventionSkillRoot(path, workspaceRoot string) bool {
	want := config.CanonicalSkillPath(path)
	if want == "" {
		return false
	}
	bases := []string{workspaceRoot}
	if home, err := os.UserHomeDir(); err == nil {
		bases = append(bases, home)
	}
	for _, base := range bases {
		base = strings.TrimSpace(base)
		if base == "" {
			continue
		}
		for _, dir := range config.SkillConventionDirs {
			if want == config.CanonicalSkillPath(filepath.Join(base, dir, skill.SkillsDirname)) {
				return true
			}
		}
	}
	return false
}

func skillRootPath(path string) string {
	if filepath.Base(path) == skill.SkillFile {
		return filepath.Dir(path)
	}
	return path
}

func skillDisplayRoot(sk skill.Skill, roots []skill.Root) string {
	cleanPath := filepath.Clean(sk.Path)
	for _, r := range roots {
		if r.Scope != sk.Scope {
			continue
		}
		cleanRoot := filepath.Clean(r.Dir)
		prefix := cleanRoot + string(filepath.Separator)
		if cleanPath == cleanRoot || strings.HasPrefix(cleanPath, prefix) {
			return config.CanonicalSkillPath(r.Dir)
		}
	}
	return config.CanonicalSkillPath(filepath.Dir(skillRootPath(sk.Path)))
}

// MCPServerInput is the drawer's "add server" form. Transport is "stdio" (Command
// + Args + Env) or "http"/"sse" (URL). Mirrors config.PluginEntry's writable shape.
type MCPServerInput struct {
	Name                 string            `json:"name"`
	CatalogID            string            `json:"catalogId,omitempty"`
	AuthMode             string            `json:"authMode,omitempty"`
	Transport            string            `json:"transport"`
	Command              string            `json:"command"`
	Args                 []string          `json:"args"`
	WorkDir              string            `json:"workDir"`
	URL                  string            `json:"url"`
	Env                  map[string]string `json:"env"`
	Headers              map[string]string `json:"headers"`
	SecretEnv            map[string]string `json:"secretEnv,omitempty"`
	SecretHeaders        map[string]string `json:"secretHeaders,omitempty"`
	RequiredEnvKeys      []string          `json:"requiredEnvKeys,omitempty"`
	RequiredHeaderKeys   []string          `json:"requiredHeaderKeys,omitempty"`
	TrustedReadOnlyTools []string          `json:"trustedReadOnlyTools"`
}

// AddMCPServer connects a server live and persists it to config (Customize → MCP →
// Add). Returns the number of tools it exposed.
func (a *App) AddMCPServer(in MCPServerInput) (int, error) {
	if a.managedConnectorConfigName(in.Name) {
		return 0, fmt.Errorf("managed connector names cannot be saved as custom MCP servers")
	}
	ctrl := a.activeCtrl()
	if ctrl == nil {
		return 0, fmt.Errorf("no active session")
	}
	if controllerHasActiveRuntimeWork(ctrl) {
		return 0, rebuildControllerActiveWorkError("MCP server")
	}
	secured, err := a.secureMCPServerInput(in)
	if err != nil {
		return 0, err
	}
	entry := mcpEntryFromInput(secured)
	entry, _ = config.NormalizePluginCommandLine(entry)
	if err := a.saveDesktopMCPServer(entry); err != nil {
		return 0, err
	}
	return ctrl.ConnectMCPServer(entry)
}

// UpdateMCPServer edits a persisted external MCP server. The name is the stable
// identity; callers must remove + add if they want to rename a server.
func (a *App) UpdateMCPServer(name string, in MCPServerInput) error {
	if a.managedConnectorConfigName(name) || a.managedConnectorConfigName(in.Name) {
		return fmt.Errorf("managed connector names cannot be edited as custom MCP servers")
	}
	ctrl := a.activeCtrl()
	if ctrl == nil {
		return fmt.Errorf("no active session")
	}
	if controllerHasActiveRuntimeWork(ctrl) {
		return rebuildControllerActiveWorkError("MCP server")
	}
	if strings.TrimSpace(in.Name) != "" && strings.TrimSpace(in.Name) != name {
		return fmt.Errorf("renaming MCP servers is not supported; remove and add a new server")
	}
	updated, found, err := a.desktopMCPServerForEdit(name)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("no configured MCP server named %q", name)
	}
	secured, err := a.secureMCPServerInput(in)
	if err != nil {
		return err
	}
	in = secured
	updated.Type = normalizeMCPTransport(in.Transport)
	updated.Command = strings.TrimSpace(in.Command)
	updated.Args = append([]string(nil), in.Args...)
	updated.WorkDir = strings.TrimSpace(in.WorkDir)
	updated.URL = strings.TrimSpace(in.URL)
	updated.Tier = ""
	if in.Env != nil {
		updated.Env = in.Env
	}
	if in.Headers != nil {
		updated.Headers = in.Headers
	}
	if in.TrustedReadOnlyTools != nil {
		updated.TrustedReadOnlyTools = uniqueStrings(in.TrustedReadOnlyTools)
	}
	updated, _ = config.NormalizePluginCommandLine(updated)
	if updated.Type == "stdio" {
		updated.URL = ""
	} else {
		updated.Command = ""
		updated.Args = nil
	}
	if err := a.saveDesktopMCPServer(updated); err != nil {
		return err
	}

	a.mu.RLock()
	tab := a.activeTabLocked()
	sessionDisabled := false
	if tab != nil {
		_, sessionDisabled = tab.disabledMCP[name]
	}
	a.mu.RUnlock()
	wasConnected := mcpConnected(ctrl, name)
	if wasConnected {
		ctrl.DisconnectMCPServer(name)
	}
	if !sessionDisabled {
		if _, err := ctrl.ConnectMCPServer(updated); err != nil {
			recordMCPFailure(ctrl, updated, err)
			return nil
		}
	}
	return nil
}

// RemoveMCPServer disconnects a live server and drops it from config (the row's ✕).
func (a *App) RemoveMCPServer(name string) error {
	tab := a.activeTab()
	if tab == nil || tab.Ctrl == nil {
		return fmt.Errorf("no active session")
	}
	if controllerHasActiveRuntimeWork(tab.Ctrl) {
		return rebuildControllerActiveWorkError("MCP server")
	}
	disconnected := tab.Ctrl.DisconnectMCPServer(name)
	removed, err := a.removeDesktopMCPServer(name)
	if err != nil {
		return err
	}
	failureRemoved := mcpFailed(tab.Ctrl, name)
	if h := tab.Ctrl.Host(); h != nil {
		h.ClearFailure(name)
	}
	if disconnected || removed || failureRemoved {
		a.mu.Lock()
		delete(tab.disabledMCP, name)
		tab.mcpOrder = removeServerOrder(tab.mcpOrder, name)
		a.mu.Unlock()
		return nil
	}
	return fmt.Errorf("no MCP server named %q", name)
}

// ReconnectMCPServer disconnects the server if it is already connected (to force
// a fresh handshake and tool re-registration), then reconnects.  Failures are
// recorded on the Host so the UI can render them.
func (a *App) ReconnectMCPServer(name string) error {
	tab := a.activeTab()
	if tab == nil || tab.Ctrl == nil {
		return fmt.Errorf("no active session")
	}
	if controllerHasActiveRuntimeWork(tab.Ctrl) {
		return rebuildControllerActiveWorkError("MCP server")
	}
	tab.Ctrl.DisconnectMCPServer(name)
	if h := tab.Ctrl.Host(); h != nil {
		h.ClearFailure(name)
	}
	_, err := a.connectConfiguredMCPServerForTab(tab, name)
	if err != nil {
		if plugin.IsServerAlreadyConnected(err) {
			a.mu.Lock()
			delete(tab.disabledMCP, name)
			a.mu.Unlock()
			return nil
		}
		entry := config.PluginEntry{Name: name}
		if p, found, cfgErr := a.desktopMCPServerForEdit(name); cfgErr == nil && found {
			entry = p
		}
		recordMCPFailure(tab.Ctrl, entry, err)
		return err
	}
	a.mu.Lock()
	delete(tab.disabledMCP, name)
	a.mu.Unlock()
	return nil
}

// ClearMCPServerAuthentication removes local auth-like config for one MCP and
// clears the current session's cached connection failure. It does not remove the
// server itself or try to sign the user out of the third-party browser session.
func (a *App) ClearMCPServerAuthentication(name string) error {
	ctrl := a.activeCtrl()
	if ctrl == nil {
		return fmt.Errorf("no active session")
	}
	if controllerHasActiveRuntimeWork(ctrl) {
		return rebuildControllerActiveWorkError("MCP server")
	}
	if _, _, _, err := config.ClearPluginAuthenticationInSource(name); err != nil {
		return err
	}
	ctrl.DisconnectMCPServer(name)
	if h := ctrl.Host(); h != nil {
		h.ClearFailure(name)
	}
	return nil
}

func (a *App) updateMCPServerTrustedReadOnlyTools(name string, update func([]string) []string) error {
	ctrl := a.activeCtrl()
	if ctrl == nil {
		return fmt.Errorf("no active session")
	}
	if controllerHasActiveRuntimeWork(ctrl) {
		return rebuildControllerActiveWorkError("MCP server")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("MCP server name is required")
	}
	updated, found, err := a.desktopMCPServerForEdit(name)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("no configured MCP server named %q", name)
	}
	trusted := uniqueStrings(updated.TrustedReadOnlyTools)
	next := uniqueStrings(update(trusted))
	if sameStringList(trusted, next) {
		updated.TrustedReadOnlyTools = trusted
		return nil
	}
	updated.TrustedReadOnlyTools = next
	if err := a.saveDesktopMCPServer(updated); err != nil {
		return err
	}

	a.mu.RLock()
	tab := a.activeTabLocked()
	sessionDisabled := false
	if tab != nil {
		_, sessionDisabled = tab.disabledMCP[name]
	}
	a.mu.RUnlock()
	if !mcpConnected(ctrl, name) {
		return nil
	}
	ctrl.DisconnectMCPServer(name)
	if h := ctrl.Host(); h != nil {
		h.ClearFailure(name)
	}
	if !sessionDisabled {
		if _, err := ctrl.ConnectMCPServer(updated); err != nil {
			recordMCPFailure(ctrl, updated, err)
			return nil
		}
	}
	return nil
}

// TrustMCPServerTool marks one raw MCP tool name as trusted read-only and
// refreshes the live connection so plan mode can use the updated trust boundary.
func (a *App) TrustMCPServerTool(name, toolName string) error {
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return fmt.Errorf("MCP tool name is required")
	}
	return a.updateMCPServerTrustedReadOnlyTools(name, func(trusted []string) []string {
		return append(trusted, toolName)
	})
}

// TrustMCPServerTools marks multiple raw MCP tool names as trusted read-only in
// one config write and one live reconnect.
func (a *App) TrustMCPServerTools(name string, toolNames []string) error {
	if len(uniqueStrings(toolNames)) == 0 {
		return fmt.Errorf("at least one MCP tool name is required")
	}
	return a.updateMCPServerTrustedReadOnlyTools(name, func(trusted []string) []string {
		return append(trusted, toolNames...)
	})
}

// UntrustMCPServerTool removes one raw MCP tool name from the trusted read-only
// list and refreshes the live connection.
func (a *App) UntrustMCPServerTool(name, toolName string) error {
	toolName = strings.TrimSpace(toolName)
	if toolName == "" {
		return fmt.Errorf("MCP tool name is required")
	}
	return a.updateMCPServerTrustedReadOnlyTools(name, func(trusted []string) []string {
		return removeString(trusted, toolName)
	})
}

// SetMCPServerEnabled is the connector toggle: on reconnects a configured server
// for this session, off disconnects it (config untouched either way — like Claude
// Code's per-conversation enable/disable, it resets on the next session start).
func (a *App) SetMCPServerEnabled(name string, enabled bool) error {
	tab := a.activeTab()
	if tab == nil || tab.Ctrl == nil {
		return fmt.Errorf("no active session")
	}
	if controllerHasActiveRuntimeWork(tab.Ctrl) {
		return rebuildControllerActiveWorkError("MCP server")
	}
	configuredEntry, hasConfiguredEntry, err := a.desktopMCPServerForEdit(name)
	if err != nil {
		return err
	}
	if enabled {
		a.mu.RLock()
		_, sessionDisabled := tab.disabledMCP[name]
		a.mu.RUnlock()
		if mcpConnected(tab.Ctrl, name) && !sessionDisabled {
			a.mu.Lock()
			delete(tab.disabledMCP, name)
			a.mu.Unlock()
			return nil
		}
		_, err := a.connectConfiguredMCPServerForTab(tab, name)
		if err == nil {
			a.mu.Lock()
			delete(tab.disabledMCP, name)
			a.mu.Unlock()
		}
		return err
	}
	if s, ok := findMCPServerView(tab.Ctrl, name); ok {
		s.Status = "disabled"
		s.Error = ""
		a.mu.Lock()
		if tab.disabledMCP == nil {
			tab.disabledMCP = map[string]ServerView{}
		}
		tab.disabledMCP[name] = s
		tab.mcpOrder = mergeServerOrder(tab.mcpOrder, []ServerView{s})
		a.mu.Unlock()
	} else if hasConfiguredEntry {
		s := withPluginConfig(ServerView{Name: name, Status: "disabled"}, configuredEntry)
		a.mu.Lock()
		if tab.disabledMCP == nil {
			tab.disabledMCP = map[string]ServerView{}
		}
		tab.disabledMCP[name] = s
		tab.mcpOrder = mergeServerOrder(tab.mcpOrder, []ServerView{s})
		a.mu.Unlock()
	}
	if tab.SharedHostKey != "" {
		tab.Ctrl.UnregisterMCPServerTools(name)
	} else {
		tab.Ctrl.DisconnectMCPServer(name)
	}
	return nil
}

func (a *App) connectConfiguredMCPServerForTab(tab *WorkspaceTab, name string) (int, error) {
	if tab == nil || tab.Ctrl == nil {
		return 0, fmt.Errorf("no active session")
	}
	workspaceRoot := strings.TrimSpace(tab.WorkspaceRoot)
	if strings.TrimSpace(workspaceRoot) == "" {
		workspaceRoot = strings.TrimSpace(tab.Ctrl.WorkspaceRoot())
	}
	if workspaceRoot == "" {
		workspaceRoot = "."
	}
	actor := actorContextForWorkspace(tabRuntimeActorContext(tab), workspaceRoot)
	cfg, _, err := desktopCatalogConfigForBillingRoute(workspaceRoot, actor, modelbilling.BillingSource(tab.billingSource))
	if err != nil {
		return 0, err
	}
	for _, p := range cfg.Plugins {
		if p.Name == name {
			return tab.Ctrl.ConnectMCPServer(p)
		}
	}
	return 0, fmt.Errorf("no configured MCP server named %q", name)
}

// SetMCPServerTier is kept for old desktop bindings. New config writes drop the
// retired tier field.
func (a *App) SetMCPServerTier(name, tier string) error {
	tier = normalizeMCPTier(tier)
	tab := a.activeTab()
	if tab != nil && controllerHasActiveRuntimeWork(tab.Ctrl) {
		return rebuildControllerActiveWorkError("MCP server")
	}
	updated, found, err := a.desktopMCPServerForEdit(name)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("no configured MCP server named %q", name)
	}
	updated.Tier = tier
	if !updated.ShouldAutoStart() {
		on := true
		updated.AutoStart = &on
	}
	if err := a.saveDesktopMCPServer(updated); err != nil {
		return err
	}
	if tab != nil && tab.Ctrl != nil && !mcpConnected(tab.Ctrl, name) {
		if _, err := tab.Ctrl.ConnectMCPServer(updated); err != nil {
			recordMCPFailure(tab.Ctrl, updated, err)
			return nil
		}
		a.mu.Lock()
		delete(tab.disabledMCP, name)
		a.mu.Unlock()
	}
	return nil
}

func (a *App) desktopMCPServerForEdit(name string) (config.PluginEntry, bool, error) {
	cfg, _, err := a.loadDesktopUserConfigForEdit()
	if err != nil {
		return config.PluginEntry{}, false, err
	}
	if p, ok := findPluginEntry(cfg.Plugins, name); ok {
		return p, true, nil
	}
	workspaceRoot, actor, source := a.desktopActiveRouteSnapshot()
	if merged, _, err := desktopCatalogConfigForBillingRoute(workspaceRoot, actor, source); err == nil {
		if p, ok := findPluginEntry(merged.Plugins, name); ok {
			return p, true, nil
		}
	}
	return config.PluginEntry{}, false, nil
}

func (a *App) saveDesktopMCPServer(entry config.PluginEntry) error {
	if a.managedConnectorConfigName(entry.Name) {
		return fmt.Errorf("managed connector names cannot be saved as custom MCP configuration")
	}
	if a.desktopMCPServerOwnedByProjectMCPJSON(entry.Name) {
		_, err := config.UpsertMCPJSONPlugin(projectMCPJSONPathForRoot(a.activeWorkspaceRoot()), entry)
		return err
	}
	cfg, path, err := a.loadDesktopUserConfigForEdit()
	if err != nil {
		return err
	}
	if err := cfg.UpsertPlugin(entry); err != nil {
		return err
	}
	if err := cfg.SaveTo(path); err != nil {
		return err
	}
	if _, err := a.removeProjectMCPOverride(entry.Name); err != nil {
		return err
	}
	return nil
}

func (a *App) removeDesktopMCPServer(name string) (bool, error) {
	removed := false
	cfg, path, err := a.loadDesktopUserConfigForEdit()
	if err != nil {
		return false, err
	}
	if cfg.RemovePlugin(name) {
		removed = true
		if err := cfg.SaveTo(path); err != nil {
			return false, err
		}
	}
	projectRemoved, err := a.removeProjectMCPOverride(name)
	if err != nil {
		return removed, err
	}
	mcpJSONRemoved, err := a.removeProjectMCPJSONServer(name)
	if err != nil {
		return removed || projectRemoved, err
	}
	return removed || projectRemoved || mcpJSONRemoved, nil
}

func (a *App) removeProjectMCPOverride(name string) (bool, error) {
	path := projectConfigPathForRoot(a.activeWorkspaceRoot())
	userPath := config.UserConfigPath()
	if path == "" || sameConfigPath(path, userPath) {
		return false, nil
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	// Project MCP overrides contain plugin metadata, not provider credentials.
	// Use the no-credential loader even when editing the override so an active
	// subscription route cannot hydrate a BYOK key through this side path.
	cfg := config.LoadForEditWithoutCredentials(path)
	if !cfg.RemovePlugin(name) {
		return false, nil
	}
	if err := cfg.SaveTo(path); err != nil {
		return false, err
	}
	return true, nil
}

func (a *App) removeProjectMCPJSONServer(name string) (bool, error) {
	return config.RemoveMCPJSONPlugin(projectMCPJSONPathForRoot(a.activeWorkspaceRoot()), name)
}

func (a *App) desktopMCPServerOwnedByProjectMCPJSON(name string) bool {
	if strings.TrimSpace(name) == "" {
		return false
	}
	cfg, _, err := a.loadDesktopUserConfigForEdit()
	if err == nil {
		if _, ok := findPluginEntry(cfg.Plugins, name); ok {
			return false
		}
	}
	projectCfg := config.LoadForEditWithoutCredentials(projectConfigPathForRoot(a.activeWorkspaceRoot()))
	if _, ok := findPluginEntry(projectCfg.Plugins, name); ok {
		return false
	}
	_, ok, err := config.LoadMCPJSONPlugin(projectMCPJSONPathForRoot(a.activeWorkspaceRoot()), name)
	return err == nil && ok
}

func projectMCPJSONPathForRoot(root string) string {
	if strings.TrimSpace(root) == "" || root == "." {
		return ".mcp.json"
	}
	return filepath.Join(root, ".mcp.json")
}

func findPluginEntry(entries []config.PluginEntry, name string) (config.PluginEntry, bool) {
	for _, p := range entries {
		if p.Name == name {
			return p, true
		}
	}
	return config.PluginEntry{}, false
}

func normalizeMCPTier(tier string) string {
	switch strings.ToLower(strings.TrimSpace(tier)) {
	case "eager":
		return "eager"
	case "background", "lazy":
		return "background"
	case "":
		return "background"
	default:
		return "background"
	}
}

func normalizeMCPTransport(transport string) string {
	switch strings.ToLower(strings.TrimSpace(transport)) {
	case "http", "streamable-http":
		return "http"
	case "sse":
		return "sse"
	default:
		return "stdio"
	}
}

func mcpConnected(ctrl control.SessionAPI, name string) bool {
	if ctrl == nil || ctrl.Host() == nil {
		return false
	}
	for _, s := range ctrl.Host().Servers() {
		if s.Name == name {
			return true
		}
	}
	return false
}

func mcpFailed(ctrl control.SessionAPI, name string) bool {
	if ctrl == nil || ctrl.Host() == nil {
		return false
	}
	for _, f := range ctrl.Host().Failures() {
		if f.Name == name {
			return true
		}
	}
	return false
}

func recordMCPFailure(ctrl control.SessionAPI, e config.PluginEntry, err error) {
	if ctrl == nil || ctrl.Host() == nil || err == nil {
		return
	}
	exp := e.ExpandedPlugin()
	ctrl.Host().RecordFailure(plugin.Spec{
		Name:    exp.Name,
		Type:    exp.Type,
		Command: exp.Command,
		Args:    exp.Args,
		Env:     exp.Env,
		URL:     exp.URL,
		Headers: exp.Headers,
	}, err)
}

func findMCPServerView(ctrl control.SessionAPI, name string) (ServerView, bool) {
	if ctrl == nil || ctrl.Host() == nil {
		return ServerView{}, false
	}
	for _, s := range ctrl.Host().Servers() {
		if s.Name == name {
			return ServerView{
				Name: s.Name, Transport: s.Transport, Status: "connected",
				Tools: s.Tools, Prompts: s.Prompts, Resources: s.Resources,
				ToolList: pluginToolsToView(s.ToolList),
			}, true
		}
	}
	for _, f := range ctrl.Host().Failures() {
		if f.Name == name {
			return ServerView{Name: f.Name, Transport: f.Transport, Status: "failed", Error: f.Error}, true
		}
	}
	return ServerView{}, false
}

func pluginToolsToView(tools []plugin.ToolInfo) []ToolView {
	if len(tools) == 0 {
		return nil
	}
	out := make([]ToolView, 0, len(tools))
	for _, t := range tools {
		out = append(out, ToolView{Name: t.Name, Description: t.Description, ReadOnlyHint: t.ReadOnlyHint})
	}
	return out
}

func sameStringList(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func orderServerViews(servers []ServerView, order []string) []ServerView {
	pos := make(map[string]int, len(order))
	for i, name := range order {
		pos[name] = i
	}
	sort.SliceStable(servers, func(i, j int) bool {
		pi, iok := pos[servers[i].Name]
		pj, jok := pos[servers[j].Name]
		switch {
		case iok && jok:
			return pi < pj
		case iok:
			return true
		case jok:
			return false
		default:
			return false
		}
	})
	return servers
}

func mergeServerOrder(order []string, servers []ServerView) []string {
	seen := make(map[string]bool, len(order)+len(servers))
	next := make([]string, 0, len(order)+len(servers))
	for _, name := range order {
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		next = append(next, name)
	}
	for _, s := range servers {
		if s.Name == "" || seen[s.Name] {
			continue
		}
		seen[s.Name] = true
		next = append(next, s.Name)
	}
	return next
}

func removeServerOrder(order []string, name string) []string {
	if name == "" || len(order) == 0 {
		return order
	}
	next := order[:0]
	for _, n := range order {
		if n != name {
			next = append(next, n)
		}
	}
	return next
}

// ModelInfo is one (provider, model) the bottom switcher can pick. Ref ("provider/
// model") is what SetModel takes; Provider/Model are for display.
type ModelInfo struct {
	Ref           string `json:"ref"`
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	BillingSource string `json:"billingSource,omitempty"`
	Current       bool   `json:"current"`
}

type EffortInfo struct {
	Supported bool     `json:"supported"`
	Current   string   `json:"current"`
	Default   string   `json:"default"`
	Levels    []string `json:"levels"`
}

// Models flattens the configured providers into their (provider, model) pairs —
// the switcher's options — marking the active one. A vendor with a `models` list
// yields one entry per model, all sharing the same endpoint/key. Subscription
// metadata never creates a duplicate BYOK route unless the user explicitly added
// that provider and saved its credential. Result is always non-nil.
func (a *App) Models() []ModelInfo {
	return a.ModelsForTab("")
}

func (a *App) ModelsForTab(tabID string) []ModelInfo {
	a.mu.RLock()
	curModel := ""
	billingSource := ""
	workspaceRoot := ""
	var actor userctx.ActorContext
	if tab := a.tabByIDLocked(tabID); tab != nil {
		curModel = tab.model
		billingSource = tab.billingSource
		workspaceRoot = tab.WorkspaceRoot
		actor = tabRuntimeActorContext(tab)
	}
	a.mu.RUnlock()
	cfg, metadataOnly, err := desktopCatalogConfigForBillingRoute(workspaceRoot, actor, modelbilling.BillingSource(billingSource))
	if err != nil {
		return []ModelInfo{}
	}
	a.applyDesktopAuthorizedSubscriptionModels(cfg, actor)
	if entry, ok := cfg.ResolveModel(curModel); ok {
		curModel = entry.Name + "/" + entry.Model
	}
	access := providerAccessSet(cfg.Desktop.ProviderAccess)
	out := []ModelInfo{}
	for i := range cfg.Providers {
		p := &cfg.Providers[i]
		if p.Name == brand.ProviderID {
			continue
		}
		if !byokProviderAvailableInCatalog(p, access, metadataOnly, actor) {
			continue
		}
		for _, m := range p.ChatModelList() {
			ref := p.Name + "/" + m
			out = append(out, ModelInfo{Ref: ref, Provider: p.Name, Model: m, BillingSource: "newapi_byok", Current: ref == curModel && billingSource == "newapi_byok"})
		}
	}
	subscriptionProvider := "airclaude_subscription"
	if desktopCloudFakeEnabled() {
		subscriptionProvider = "subscription_gateway_fixture"
	}
	for _, modelID := range desktopSubscriptionCatalogModels(cfg) {
		providers := []*config.ProviderEntry{}
		if desktopCloudFakeEnabled() {
			for i := range cfg.Providers {
				providers = append(providers, &cfg.Providers[i])
			}
		} else if provider, ok := cfg.Provider("kwu"); ok {
			providers = append(providers, provider)
		}
		for _, provider := range providers {
			if desktopCloudFakeEnabled() && !provider.HasModel(modelID) {
				continue
			}
			ref := provider.Name + "/" + modelID
			out = append(out, ModelInfo{Ref: ref, Provider: subscriptionProvider, Model: modelID, BillingSource: string(modelbilling.BillingSubscriptionPool), Current: ref == curModel && billingSource == string(modelbilling.BillingSubscriptionPool)})
		}
	}
	return out
}

func modelProviderAccessAllowed(access map[string]bool, name string) bool {
	if len(access) == 0 {
		return true
	}
	name = strings.TrimSpace(name)
	return access[name] || access[config.CanonicalDesktopOfficialProviderName(name)]
}

func modelProviderExplicitlyAdded(access map[string]bool, name string) bool {
	name = strings.TrimSpace(name)
	return access[name] || access[config.CanonicalDesktopOfficialProviderName(name)]
}

func byokProviderAvailableInCatalog(provider *config.ProviderEntry, access map[string]bool, metadataOnly bool, actor userctx.ActorContext) bool {
	if provider == nil || !modelProviderAccessAllowed(access, provider.Name) {
		return false
	}
	if !metadataOnly {
		return provider.Configured()
	}
	if !modelProviderExplicitlyAdded(access, provider.Name) {
		return false
	}
	if !provider.RequiresAPIKey() {
		return true
	}
	return config.CredentialStoredWithPaths(config.NewProfilePathResolver(actor), provider.APIKeyEnv)
}

func controllerHasActiveRuntimeWork(ctrl control.SessionAPI) bool {
	if ctrl == nil {
		return false
	}
	status := ctrl.RuntimeStatus()
	return status.Running || status.Completing || status.PendingPrompt || status.BackgroundJobs > 0
}

func currentExpertID(ctrl control.SessionAPI) string {
	state, _ := currentExpertState(ctrl)
	return state.SelectedID
}

func currentExpertState(ctrl control.SessionAPI) (expert.State, bool) {
	if ctrl == nil {
		return expert.State{}, false
	}
	expertCtrl, ok := ctrl.(interface{ CurrentExpert() expert.State })
	if !ok {
		return expert.State{}, false
	}
	return expertCtrl.CurrentExpert(), true
}

func rebuildControllerActiveWorkError(setting string) error {
	return fmt.Errorf("finish or cancel the current turn, answer pending prompts, and stop background jobs before changing %s", setting)
}

func (a *App) persistUserDefaultModelSelection(ref string, source modelbilling.BillingSource, actor userctx.ActorContext) error {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil
	}
	var cfg *config.Config
	var path string
	var err error
	if source == modelbilling.BillingSubscriptionPool {
		paths := a.activeProfilePathResolver()
		path = paths.UserConfigPath()
		if path == "" {
			return fmt.Errorf("cannot resolve user config directory")
		}
		cfg = config.LoadForEditWithoutCredentialsWithPaths(paths)
		a.applyDesktopSubscriptionModels(cfg, actor)
	} else {
		cfg, path, err = a.loadDesktopUserConfigForEdit()
		if err != nil {
			return err
		}
	}
	entry, ok := cfg.ResolveModel(ref)
	if !ok || (source != modelbilling.BillingSubscriptionPool && !modelProviderAccessAllowed(providerAccessSet(cfg.Desktop.ProviderAccess), entry.Name)) {
		return nil
	}
	if !config.IsLikelyChatModel(entry.Model) {
		return nil
	}
	resolved := entry.Name + "/" + entry.Model
	a.applyDefaultModelRoute(cfg, resolved, source)
	return cfg.SaveTo(path)
}

// SetModel switches the active session's model and carries its conversation
// into the rebuilt controller. Global defaults are changed only in Settings.
func (a *App) SetModel(name string) error {
	return a.SetModelForTab("", name)
}

// SetModelRouteForTab changes the current session's explicit model and
// billing-source selection. It never encodes the source into the model ref.
func (a *App) SetModelRouteForTab(tabID, name, billingSource string) error {
	source := modelbilling.BillingSource(strings.TrimSpace(billingSource))
	if err := source.Validate(); err != nil {
		return err
	}
	return a.setModelRouteForTab(tabID, name, source)
}

func (a *App) SetModelForTab(tabID, name string) error {
	name = strings.TrimSpace(name)
	if a.ctx == nil || name == "" {
		return nil
	}
	tab := a.tabByID(tabID)
	if tab == nil {
		return nil
	}
	return a.setModelRouteForTab(tabID, name, modelbilling.BillingSource(tab.billingSource))
}

// setModelRouteForTab treats model and billing source as one route. The old
// route remains live until a controller for the complete target route has
// built, so a missing BYOK credential or any boot failure cannot strand a tab.
func (a *App) setModelRouteForTab(tabID, requestedModel string, targetSource modelbilling.BillingSource) error {
	return a.setModelRouteForTabWithPersistence(tabID, requestedModel, targetSource, false)
}

func (a *App) setModelRouteForTabWithPersistence(tabID, requestedModel string, targetSource modelbilling.BillingSource, persistDefault bool) (err error) {
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel == "" {
		return nil
	}
	targetSource = modelbilling.BillingSource(strings.TrimSpace(string(targetSource)))
	tab := a.tabByID(tabID)
	if tab == nil {
		return fmt.Errorf("tab %q not found", tabID)
	}
	if tab.Ctrl != nil && !tab.Ready {
		return workspaceNotReadyErr(tab)
	}
	defer func() {
		if err == nil {
			a.emitModelRouteChanged(tab.ID)
		}
	}()
	preloaded := loadedTabSession{}
	if tab.Ctrl == nil && strings.TrimSpace(tab.SessionPath) != "" {
		var err error
		preloaded, err = a.preloadTabSession(tab, tab.SessionPath)
		if err != nil {
			return err
		}
	}
	actor := tabRuntimeActorContext(tab)
	cfg, err := a.desktopConfigForBillingRoute(tab.WorkspaceRoot, actor, targetSource)
	if err != nil {
		return err
	}
	entry, ok := cfg.ResolveModel(requestedModel)
	if !ok {
		return fmt.Errorf("unknown model %q", requestedModel)
	}
	if targetSource != modelbilling.BillingSubscriptionPool && !modelProviderAccessAllowed(providerAccessSet(cfg.Desktop.ProviderAccess), entry.Name) {
		return fmt.Errorf("model %q is not available because provider %q is not added", requestedModel, entry.Name)
	}
	targetModel := entry.Name + "/" + entry.Model
	if desktopCloudFakeEnabled() && targetSource == modelbilling.BillingNewAPIByok && !entry.Configured() {
		return fmt.Errorf("model %q requires a configured credential for the current profile", targetModel)
	}

	if controllerHasActiveRuntimeWork(tab.Ctrl) {
		return rebuildControllerActiveWorkError("model route")
	}
	currentModel := strings.TrimSpace(tab.model)
	if currentModel == "" {
		currentModel = cfg.DefaultModel
	}
	if currentEntry, ok := cfg.ResolveModel(currentModel); ok && targetModel == currentEntry.Name+"/"+currentEntry.Model && targetSource == modelbilling.BillingSource(tab.billingSource) && tab.Ctrl != nil && tab.Ready && strings.TrimSpace(tab.StartupErr) == "" {
		a.mu.Lock()
		if current := a.tabByIDLocked(tabID); current == tab {
			current.model = targetModel
			current.billingSource = string(targetSource)
			a.saveTabsLocked()
		}
		a.mu.Unlock()
		if err := saveTabSessionMeta(tab, tab.currentSessionPath()); err != nil {
			recordDesktopBackgroundFailure("session.meta.model", err)
		}
		if persistDefault {
			return a.persistUserDefaultModelSelection(targetModel, targetSource, actor)
		}
		return nil
	}

	effortOverride := cloneStringPtr(tab.effort)
	if effortOverride != nil {
		normalized, err := config.NormalizeEffort(entry, config.EffortDisplay(&config.ProviderEntry{Effort: *effortOverride}))
		if err != nil {
			effortOverride = nil
		} else {
			effortOverride = &normalized
		}
	}
	if effortOverride == nil && config.EffortCapabilityForEntry(entry).Supported {
		normalized, normalizeErr := config.NormalizeEffort(entry, config.EffortDisplay(entry))
		if normalizeErr == nil {
			effortOverride = &normalized
		}
	}
	if tab.Ctrl == nil {
		buildRoute := &modelbilling.BillingRouteSelection{ModelRef: targetModel, BillingSource: targetSource}
		if err := a.scheduleTabControllerBuildWithRoute(tab, preloaded, false, func(current *WorkspaceTab) error {
			if current != tab || current.Ctrl != nil {
				return rebuildControllerActiveWorkError("model route")
			}
			current.model = targetModel
			current.billingSource = string(targetSource)
			current.effort = cloneStringPtr(effortOverride)
			a.saveTabsLocked()
			return nil
		}, buildRoute); err != nil {
			return err
		}
		if persistDefault {
			return a.persistUserDefaultModelSelection(targetModel, targetSource, actor)
		}
		return nil
	}

	var carried []provider.Message
	prevPath := ""
	oldCtrl := tab.Ctrl
	expertID := currentExpertID(oldCtrl)
	if oldCtrl != nil {
		prevPath = oldCtrl.SessionPath()
		if err := a.snapshotTab(tab); err != nil {
			recordDesktopBackgroundFailure("session.snapshot.model", err)
		}
		carried = oldCtrl.History()
	}

	// Preserve the shared plugin host across controller rebuilds — the tab
	// stays in the same workspace root, so MCP processes must not be restarted.
	sharedHost := a.lookupSharedHost(tab.SharedHostKey)

	bootOptions := boot.Options{
		Model:                    targetModel,
		RequireKey:               false,
		Sink:                     tab.sink,
		WorkspaceRoot:            tab.WorkspaceRoot,
		SessionDir:               tabSessionDir(tab),
		EffortOverride:           cloneStringPtr(effortOverride),
		TokenMode:                currentTabTokenMode(tab),
		SharedHost:               sharedHost,
		CleanupPendingReconciler: reconcileDesktopCleanupPending,
		Actor:                    actor,
		ExpertID:                 expertID,
	}
	applyTabSceneBootOptions(&bootOptions, tab)
	newCtrl, err := a.buildTabControllerRuntime(a.bootContext(), bootOptions, actor, tab.WorkspaceRoot, targetModel, string(targetSource))
	if err != nil {
		return err
	}
	a.bindControllerDisplayRecorder(newCtrl)
	a.mu.Lock()
	current := a.tabByIDLocked(tabID)
	if current != tab || current.Ctrl != oldCtrl || !current.Ready || controllerHasActiveRuntimeWork(oldCtrl) {
		a.mu.Unlock()
		newCtrl.ReleaseResources()
		return rebuildControllerActiveWorkError("model route")
	}
	tab.Ctrl = newCtrl
	tab.model = targetModel
	tab.billingSource = string(targetSource)
	tab.effort = cloneStringPtr(effortOverride)
	tab.Label = newCtrl.Label()
	clearTabStartupFailureLocked(tab)
	tab.Ready = false
	a.saveTabsLocked()
	a.mu.Unlock()
	if oldCtrl != nil {
		// This rebuild keeps the same logical session alive. Release the replaced
		// runtime outside App.mu because job shutdown may synchronously emit events.
		oldCtrl.ReleaseResources()
	}
	newCtrl.EnableInteractiveApproval()
	applyTabModeToController(newCtrl, tab.mode)
	applyTabToolApprovalModeToController(newCtrl, tab.toolApprovalMode)
	newCtrl.SetGoal(tab.goal)

	path := agent.ContinueSessionPath(prevPath, newCtrl.SessionDir(), newCtrl.Label())
	resumeWithFreshSystemPrompt(newCtrl, carried, path)
	a.persistTabSessionPath(tab, path)
	if strings.TrimSpace(tab.TopicID) != "" {
		if err := setTopicModel(topicTitleRoot(tab.Scope, tab.WorkspaceRoot), tab.TopicID, targetModel); err != nil {
			recordDesktopBackgroundFailure("topic.model.selection.save", err)
		}
	}
	if persistDefault {
		if err := a.persistUserDefaultModelSelection(targetModel, targetSource, actor); err != nil {
			return err
		}
	}
	a.mu.Lock()
	current = a.tabByIDLocked(tabID)
	effortMatches := current != nil && ((current.effort == nil && effortOverride == nil) ||
		(current.effort != nil && effortOverride != nil && *current.effort == *effortOverride))
	if current != tab || current.removed || current.Ctrl != newCtrl || current.Ready ||
		current.model != targetModel || current.billingSource != string(targetSource) || !effortMatches {
		a.mu.Unlock()
		return fmt.Errorf("model route changed while the replacement controller was starting")
	}
	current.Ready = true
	a.saveTabsLocked()
	a.mu.Unlock()
	return nil
}

func (a *App) Effort() EffortInfo {
	return a.EffortForTab("")
}

func (a *App) EffortForTab(tabID string) EffortInfo {
	entry, err := a.currentProviderEntryForTab(tabID)
	if err != nil {
		return EffortInfo{Current: "auto", Levels: []string{}}
	}
	return effortInfoForEntry(entry)
}

// EffortForDraftModel resolves the effort controls for a model before a project
// draft owns a Tab. It is read-only and follows the same config route that a new
// Tab for the target workspace will use.
func (a *App) EffortForDraftModel(scope, workspaceRoot, modelRef, billingSource, currentEffort string) EffortInfo {
	scope = strings.TrimSpace(scope)
	if scope == "project" {
		workspaceRoot = strings.TrimSpace(workspaceRoot)
		if workspaceRoot == "" {
			return EffortInfo{Current: "auto", Levels: []string{}}
		}
		if abs, err := filepath.Abs(workspaceRoot); err == nil {
			workspaceRoot = abs
		}
	} else {
		workspaceRoot = globalWorkspaceRoot()
	}

	actor := a.actorForWorkspace(workspaceRoot)
	source := modelbilling.BillingSource(strings.TrimSpace(billingSource))
	if source == "" {
		source = a.defaultBillingSourceForNewTask(actor)
	}
	if err := source.Validate(); err != nil {
		return EffortInfo{Current: "auto", Levels: []string{}}
	}
	cfg, err := a.desktopConfigForBillingRoute(workspaceRoot, actor, source)
	if err != nil {
		return EffortInfo{Current: "auto", Levels: []string{}}
	}
	ref := strings.TrimSpace(modelRef)
	if ref == "" {
		ref = cfg.DefaultModel
	}
	config.NormalizeLegacyMimoCustomProvidersForRefs(cfg, ref)
	resolved, _, ok := cfg.ResolveModelWithFallback(ref)
	if !ok {
		return EffortInfo{Current: "auto", Levels: []string{}}
	}
	entry, ok := cfg.ResolveModel(resolved)
	if !ok {
		return EffortInfo{Current: "auto", Levels: []string{}}
	}
	if requested := strings.TrimSpace(currentEffort); requested != "" {
		if normalized, normalizeErr := config.NormalizeEffort(entry, requested); normalizeErr == nil {
			entry.Effort = normalized
		}
	}
	return effortInfoForEntry(entry)
}

func effortInfoForEntry(entry *config.ProviderEntry) EffortInfo {
	cap := config.EffortCapabilityForEntry(entry)
	if !cap.Supported {
		return EffortInfo{Supported: false, Current: "auto", Default: cap.Default, Levels: []string{}}
	}
	levels := cap.Levels
	if levels == nil {
		levels = []string{}
	}
	return EffortInfo{Supported: true, Current: config.EffortDisplay(entry), Default: cap.Default, Levels: levels}
}

func (a *App) SetEffort(level string) error {
	return a.SetEffortForTab("", level)
}

func (a *App) SetEffortForTab(tabID, level string) error {
	tab := a.tabByID(tabID)
	if tab == nil {
		if strings.TrimSpace(tabID) == "" {
			entry, err := a.currentProviderEntryForTab("")
			if err != nil {
				return err
			}
			effort, err := config.NormalizeEffort(entry, level)
			if err != nil {
				return err
			}
			return a.applyProviderEffortConfig(entry, effort)
		}
		return fmt.Errorf("tab %q not found", tabID)
	}
	if tab.Ctrl != nil && !tab.Ready {
		return workspaceNotReadyErr(tab)
	}
	ctrl := tab.Ctrl
	if controllerHasActiveRuntimeWork(ctrl) {
		return rebuildControllerActiveWorkError("effort")
	}
	entry, err := a.currentProviderEntryForTab(tabID)
	if err != nil {
		return err
	}
	modelRef := entry.Name + "/" + entry.Model
	effort, err := config.NormalizeEffort(entry, level)
	if err != nil {
		return err
	}
	// An inherited effort is not an explicit override, even when both resolve
	// to the same value today. Only reuse a healthy, explicitly configured tab.
	if ctrl != nil && tab.Ready && strings.TrimSpace(tab.StartupErr) == "" &&
		tab.model == modelRef && tab.effort != nil && *tab.effort == effort {
		a.mu.Lock()
		if a.tabRuntimes[tab.ID] != tab || tab.Ctrl != ctrl {
			a.mu.Unlock()
			return fmt.Errorf("tab changed while setting effort")
		}
		a.saveTabsLocked()
		a.mu.Unlock()
		a.persistTabSessionPath(tab, ctrl.SessionPath())
		return nil
	}
	var carried []provider.Message
	prevPath := ""
	actor := tabRuntimeActorContext(tab)
	expertID := currentExpertID(tab.Ctrl)
	if tab.Ctrl != nil {
		prevPath = tab.Ctrl.SessionPath()
		if err := a.snapshotTab(tab); err != nil {
			recordDesktopBackgroundFailure("session.snapshot.subscription", err)
		}
		carried = tab.Ctrl.History()
		tab.Ctrl.Close()
	}
	sharedHost := a.lookupSharedHost(tab.SharedHostKey)
	bootOptions := boot.Options{
		Model:                    modelRef,
		RequireKey:               false,
		Sink:                     tab.sink,
		WorkspaceRoot:            tab.WorkspaceRoot,
		SessionDir:               tabSessionDir(tab),
		EffortOverride:           &effort,
		TokenMode:                currentTabTokenMode(tab),
		SharedHost:               sharedHost,
		CleanupPendingReconciler: reconcileDesktopCleanupPending,
		Actor:                    actor,
		ExpertID:                 expertID,
	}
	applyTabSceneBootOptions(&bootOptions, tab)
	newCtrl, err := a.buildDesktopController(a.bootContext(), bootOptions, actor, tab.WorkspaceRoot, modelRef, tab.billingSource)
	if err != nil {
		return err
	}
	a.bindControllerDisplayRecorder(newCtrl)
	a.mu.Lock()
	tab.Ctrl = newCtrl
	tab.model = modelRef
	tab.effort = &effort
	tab.Label = newCtrl.Label()
	clearTabStartupFailureLocked(tab)
	tab.Ready = true
	a.saveTabsLocked()
	a.mu.Unlock()
	newCtrl.EnableInteractiveApproval()
	applyTabModeToController(newCtrl, tab.mode)
	applyTabToolApprovalModeToController(newCtrl, tab.toolApprovalMode)
	newCtrl.SetGoal(tab.goal)
	path := agent.ContinueSessionPath(prevPath, newCtrl.SessionDir(), newCtrl.Label())
	resumeWithFreshSystemPrompt(newCtrl, carried, path)
	a.persistTabSessionPath(tab, path)
	return nil
}

func (a *App) SetTokenMode(mode string) error {
	return a.SetTokenModeForTab("", mode)
}

func (a *App) SetTokenModeForTab(tabID, mode string) error {
	mode = boot.NormalizeTokenMode(mode)
	tab := a.tabByID(tabID)
	if tab == nil {
		if strings.TrimSpace(tabID) == "" {
			return nil
		}
		return fmt.Errorf("tab %q not found", tabID)
	}
	if tab.Ctrl != nil && !tab.Ready {
		return workspaceNotReadyErr(tab)
	}
	if mode == currentTabTokenMode(tab) {
		return nil
	}
	ctrl := tab.Ctrl
	if controllerHasActiveRuntimeWork(ctrl) {
		return rebuildControllerActiveWorkError("token mode")
	}
	modelRef, fallback, err := a.resolvedModelForTab(tab)
	if err != nil {
		return err
	}
	if fallback && strings.TrimSpace(tab.model) != "" {
		a.noticeForTab(tab.ID, fmt.Sprintf("model %q is no longer available; switched to %s", tab.model, modelRef))
	}

	var carried []provider.Message
	prevPath := ""
	actor := tabRuntimeActorContext(tab)
	oldCtrl := tab.Ctrl
	expertID := currentExpertID(oldCtrl)
	if oldCtrl != nil {
		prevPath = oldCtrl.SessionPath()
		if err := a.snapshotTab(tab); err != nil {
			recordDesktopBackgroundFailure("session.snapshot.byok", err)
		}
		carried = oldCtrl.History()
	}
	sharedHost := a.lookupSharedHost(tab.SharedHostKey)
	bootOptions := boot.Options{
		Model:                    modelRef,
		RequireKey:               false,
		Sink:                     tab.sink,
		WorkspaceRoot:            tab.WorkspaceRoot,
		SessionDir:               tabSessionDir(tab),
		EffortOverride:           cloneStringPtr(tab.effort),
		TokenMode:                mode,
		SharedHost:               sharedHost,
		CleanupPendingReconciler: reconcileDesktopCleanupPending,
		Actor:                    actor,
		ExpertID:                 expertID,
	}
	applyTabSceneBootOptions(&bootOptions, tab)
	newCtrl, err := a.buildDesktopController(a.bootContext(), bootOptions, actor, tab.WorkspaceRoot, modelRef, tab.billingSource)
	if err != nil {
		return err
	}
	a.bindControllerDisplayRecorder(newCtrl)
	if oldCtrl != nil {
		oldCtrl.Close()
	}
	a.mu.Lock()
	tab.Ctrl = newCtrl
	tab.model = modelRef
	tab.tokenMode = mode
	tab.Label = newCtrl.Label()
	clearTabStartupFailureLocked(tab)
	tab.Ready = true
	a.saveTabsLocked()
	a.mu.Unlock()
	newCtrl.EnableInteractiveApproval()
	applyTabModeToController(newCtrl, tab.mode)
	applyTabToolApprovalModeToController(newCtrl, tab.toolApprovalMode)
	newCtrl.SetGoal(tab.goal)
	path := agent.ContinueSessionPath(prevPath, newCtrl.SessionDir(), newCtrl.Label())
	resumeWithFreshSystemPrompt(newCtrl, carried, path)
	a.persistTabSessionPath(tab, path)
	return nil
}

func (a *App) applyProviderEffortConfig(entry *config.ProviderEntry, effort string) error {
	return a.applyConfigChange(func(cfg *config.Config) error {
		if _, ok := cfg.Provider(entry.Name); !ok {
			if err := cfg.UpsertProvider(*entry); err != nil {
				return err
			}
		}
		if entry.Kind == "anthropic" && effort != "" && entry.Thinking == "" {
			if err := cfg.SetProviderThinking(entry.Name, "adaptive"); err != nil {
				return err
			}
		}
		for _, name := range providerEffortTargetNames(cfg, entry) {
			if err := cfg.SetProviderEffort(name, effort); err != nil {
				return err
			}
		}
		return nil
	})
}

func providerEffortTargetNames(cfg *config.Config, entry *config.ProviderEntry) []string {
	if cfg == nil || entry == nil {
		return nil
	}
	out := []string{entry.Name}
	seen := map[string]bool{entry.Name: true}
	kind := officialProviderKindFromEntry(*entry)
	if kind == "" {
		return out
	}
	var family []string
	switch kind {
	case "deepseek":
		family = []string{"deepseek", "deepseek-flash", "deepseek-pro"}
	}
	for _, name := range family {
		if seen[name] {
			continue
		}
		p, ok := cfg.Provider(name)
		if !ok || officialProviderKindFromEntry(*p) != kind {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

// DirEntry is one entry in the "@" file-reference menu.
type DirEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"isDir"`
}

// FilePreview is a bounded, read-only file payload for the workspace side panel.
type FilePreview struct {
	Path      string `json:"path"`
	Body      string `json:"body"`
	Size      int64  `json:"size"`
	Truncated bool   `json:"truncated"`
	Binary    bool   `json:"binary"`
	Kind      string `json:"kind,omitempty"`
	Mime      string `json:"mime,omitempty"`
	URL       string `json:"url,omitempty"`
	Err       string `json:"err,omitempty"`
}

type WorkspaceChangeView struct {
	Path         string   `json:"path"`
	OldPath      string   `json:"oldPath,omitempty"`
	Sources      []string `json:"sources"`
	GitStatus    string   `json:"gitStatus,omitempty"`
	Turns        []int    `json:"turns,omitempty"`
	LatestPrompt string   `json:"latestPrompt,omitempty"`
	LatestTime   int64    `json:"latestTime,omitempty"`
}

type WorkspaceChangesView struct {
	Files        []WorkspaceChangeView `json:"files"`
	GitAvailable bool                  `json:"gitAvailable"`
	GitErr       string                `json:"gitErr,omitempty"`
	GitBranch    string                `json:"gitBranch,omitempty"`
}

// workspaceNoiseNames are local cache/vendor entries hidden from the file tree
// and "@" menu regardless of where they appear.
var workspaceNoiseNames = map[string]bool{
	".codex":       true,
	".DS_Store":    true,
	".git":         true,
	".npm":         true,
	".pnpm-store":  true,
	"node_modules": true,
	"Thumbs.db":    true,
}

var workspaceNoiseDirs = map[string]bool{
	"bin":                      true,
	"desktop/build":            true,
	"desktop/frontend/dist":    true,
	"desktop/frontend/wailsjs": true,
	"dist":                     true,
	"npm/.stage":               true,
	"stage":                    true,
	"tmp":                      true,
}

const filePreviewLimit = 256 * 1024
const fileRefSearchLimit = 20

var previewMediaMIMEs = map[string]string{
	".bmp":  "image/bmp",
	".gif":  "image/gif",
	".jpeg": "image/jpeg",
	".jpg":  "image/jpeg",
	".m4v":  "video/x-m4v",
	".mov":  "video/quicktime",
	".mp4":  "video/mp4",
	".pdf":  "application/pdf",
	".png":  "image/png",
	".svg":  "image/svg+xml",
	".webm": "video/webm",
	".webp": "image/webp",
}

func trimUTF8PartialSuffix(data []byte) []byte {
	if utf8.Valid(data) {
		return data
	}
	for i := len(data) - 1; i >= 0 && len(data)-i <= utf8.UTFMax; i-- {
		if !utf8.RuneStart(data[i]) {
			continue
		}
		if !utf8.Valid(data[:i]) || utf8.FullRune(data[i:]) {
			return data
		}
		return data[:i]
	}
	return data
}

func previewMediaKind(path string) (kind string, mime string) {
	mime = previewMediaMIMEs[strings.ToLower(filepath.Ext(path))]
	if mime == "" {
		return "", ""
	}
	if strings.HasPrefix(mime, "image/") {
		return "image", mime
	}
	if strings.HasPrefix(mime, "video/") {
		return "video", mime
	}
	if mime == "application/pdf" {
		return "pdf", mime
	}
	return "", ""
}

func workspaceEntryRel(rel, name string) string {
	rel = strings.Trim(filepath.ToSlash(rel), "/")
	if rel == "" || rel == "." {
		return name
	}
	return rel + "/" + name
}

func skipWorkspaceEntry(rel, name string, isDir bool) bool {
	if workspaceNoiseNames[name] {
		return true
	}
	return isDir && workspaceNoiseDirs[workspaceEntryRel(rel, name)]
}

func (a *App) activeWorkspaceBase() (string, error) {
	return workspaceBaseFromRoot(a.activeWorkspaceRoot())
}

func workspaceBaseFromRoot(root string) (string, error) {
	if strings.TrimSpace(root) == "" || root == "." {
		return os.Getwd()
	}
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	return filepath.Clean(root), nil
}

func historicalWorkspaceBaseFromRoot(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", os.ErrPermission
	}
	return workspaceBaseFromRoot(root)
}

func (a *App) workspacePath(rel string) (string, bool, error) {
	base, err := a.activeWorkspaceBase()
	if err != nil {
		return "", false, err
	}
	return workspacePathForBase(base, rel)
}

func windowsPathFromWSLMount(path string) (string, bool) {
	if len(path) < len("/mnt/x") || !strings.EqualFold(path[:5], "/mnt/") {
		return path, false
	}
	drive := path[5]
	if !((drive >= 'a' && drive <= 'z') || (drive >= 'A' && drive <= 'Z')) || (len(path) > 6 && path[6] != '/') {
		return path, false
	}
	rest := strings.TrimPrefix(path[6:], "/")
	return strings.ToUpper(string(drive)) + ":/" + rest, true
}

func normalizeWorkspacePathForOS(path, goos string) string {
	if goos != "windows" {
		return path
	}
	if windowsPath, ok := windowsPathFromWSLMount(path); ok {
		return windowsPath
	}
	return path
}

func workspacePathForBase(base, rel string) (string, bool, error) {
	base = filepath.Clean(base)
	if rel == "" {
		return "", false, os.ErrInvalid
	}
	rel = normalizeWorkspacePathForOS(rel, goruntime.GOOS)
	path := rel
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, rel)
	}
	path = filepath.Clean(path)
	r, err := filepath.Rel(base, path)
	if err != nil {
		return "", false, err
	}
	if r == ".." || strings.HasPrefix(r, ".."+string(os.PathSeparator)) {
		return "", false, os.ErrPermission
	}
	realBase, err := filepath.EvalSymlinks(base)
	if err != nil {
		return "", false, err
	}
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", false, err
	}
	r, err = filepath.Rel(realBase, realPath)
	if err != nil {
		return "", false, err
	}
	if r == ".." || strings.HasPrefix(r, ".."+string(os.PathSeparator)) {
		return "", false, os.ErrPermission
	}
	return path, true, nil
}

// ListDir lists one directory level (directories first, then files, each
// alphabetical) for the "@" file-reference menu. rel resolves against the active
// tab workspace. The menu navigates one level at a time, never recursively —
// bounded for huge trees.
func (a *App) ListDir(rel string) []DirEntry {
	base, err := a.activeWorkspaceBase()
	if err != nil {
		return []DirEntry{}
	}
	return listDirFromBase(base, rel)
}

// ListDirForTab lists entries from the workspace that owns tabID, even when
// another tab is currently active.
func (a *App) ListDirForTab(tabID, rel string) []DirEntry {
	base, err := a.workspaceBaseForOwnedTab(tabID)
	if err != nil {
		return []DirEntry{}
	}
	return listDirFromBase(base, rel)
}

func listDirFromBase(base, rel string) []DirEntry {
	dir := base
	if rel != "" {
		path, ok, err := workspacePathForBase(base, rel)
		if err != nil || !ok {
			return []DirEntry{}
		}
		dir = path
	}
	es, err := os.ReadDir(dir)
	if err != nil {
		return []DirEntry{}
	}
	dirs, files := []DirEntry{}, []DirEntry{}
	for _, e := range es {
		name := e.Name()
		if skipWorkspaceEntry(rel, name, e.IsDir()) {
			continue
		}
		if e.IsDir() {
			dirs = append(dirs, DirEntry{Name: name, IsDir: true})
			continue
		}
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		files = append(files, DirEntry{Name: name, IsDir: false})
	}
	sort.Slice(dirs, func(i, j int) bool { return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name) })
	sort.Slice(files, func(i, j int) bool { return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name) })
	return append(dirs, files...)
}

// SearchFileRefs finds workspace files by basename for bare "@token" completion.
func (a *App) SearchFileRefs(query string) []DirEntry {
	base, err := a.activeWorkspaceBase()
	if err != nil {
		return nil
	}
	return searchFileRefsFromBase(base, query)
}

// SearchFileRefsForTab searches the workspace that owns tabID rather than the
// workspace selected by mutable active-tab state.
func (a *App) SearchFileRefsForTab(tabID, query string) []DirEntry {
	base, err := a.workspaceBaseForOwnedTab(tabID)
	if err != nil {
		return nil
	}
	return searchFileRefsFromBase(base, query)
}

func searchFileRefsFromBase(base, query string) []DirEntry {
	results := fileref.Search(base, query, fileRefSearchLimit)
	out := make([]DirEntry, 0, len(results))
	for _, r := range results {
		out = append(out, DirEntry{Name: r.Path, IsDir: r.IsDir})
	}
	return out
}

// ReadFile returns a small text preview for a file under the current workspace.
func (a *App) ReadFile(rel string) FilePreview {
	base, err := a.activeWorkspaceBase()
	if err != nil {
		return FilePreview{Path: rel, Err: "invalid path"}
	}
	return a.readFileFromBase(base, rel)
}

// ReadFileForTab returns a preview from the workspace that owns tabID, even if
// another tab became active while an expert was producing the file.
func (a *App) ReadFileForTab(tabID, rel string) FilePreview {
	base, err := a.workspaceBaseForOwnedTab(tabID)
	if err != nil {
		return FilePreview{Path: rel, Err: "invalid path"}
	}
	return a.readFileFromBase(base, rel)
}

// ReadFileForWorkspace returns a preview from an explicit historical session
// workspace instead of coupling the read to whichever tab is currently active.
func (a *App) ReadFileForWorkspace(workspaceRoot, rel string) FilePreview {
	base, err := historicalWorkspaceBaseFromRoot(workspaceRoot)
	if err != nil {
		return FilePreview{Path: rel, Err: "invalid path"}
	}
	return a.readFileFromBase(base, rel)
}

func (a *App) readFileFromBase(base, rel string) FilePreview {
	out := FilePreview{Path: rel}
	path, ok, err := workspacePathForBase(base, rel)
	if err != nil || !ok {
		out.Err = "invalid path"
		return out
	}
	info, err := os.Stat(path)
	if err != nil {
		out.Err = err.Error()
		return out
	}
	if info.IsDir() {
		out.Err = "path is a directory"
		return out
	}
	if !info.Mode().IsRegular() {
		out.Err = "path is not a regular file"
		return out
	}
	out.Size = info.Size()
	if kind, mime := previewMediaKind(path); kind != "" {
		token := a.ensureMediaTokenStore().create(path, info.Name(), mime, kind, info.Size(), info.ModTime())
		out.Kind = kind
		out.Mime = mime
		out.URL = "/__kwu_workspace_media/" + token + "/" + url.PathEscape(info.Name())
		return out
	}
	f, err := os.Open(path)
	if err != nil {
		out.Err = err.Error()
		return out
	}
	defer f.Close()

	buf := make([]byte, filePreviewLimit+1)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		out.Err = err.Error()
		return out
	}
	data := buf[:n]
	if len(data) > filePreviewLimit {
		data = data[:filePreviewLimit]
		out.Truncated = true
	}

	// Check for BOM first (just the first 2-3 bytes — always complete
	// even at a truncation boundary). BOM-prefixed files skip the NUL
	// check since UTF-16 normally contains 0x00 for ASCII characters.
	bomKind := fileenc.DetectQuick(data)
	if bomKind != fileenc.UTF8 {
		enc, _ := fileenc.Detect(data)
		if enc == fileenc.LossyUTF8 {
			out.Binary = true
			return out
		}
		decoded := fileenc.Decode(data, enc)
		out.Body = string(decoded)
		return out
	}

	// No BOM — NUL in raw bytes is a binary signal.
	if bytes.Contains(data, []byte{0}) {
		out.Binary = true
		return out
	}

	// Trim any partial multi-byte rune at the truncation boundary BEFORE
	// encoding detection. Without this, a large UTF-8 file truncated
	// mid-character would fail utf8.Valid and be misdetected as GB18030
	// or LossyUTF8, producing mojibake or a false binary classification.
	if out.Truncated {
		data = trimUTF8PartialSuffix(data)
	}
	enc, _ := fileenc.Detect(data)
	if enc == fileenc.LossyUTF8 {
		out.Binary = true
		return out
	}
	out.Body = string(fileenc.Decode(data, enc))
	return out
}

// OpenWorkspacePath opens a file or folder from the workspace in the OS default app.
func (a *App) OpenWorkspacePath(rel string) error {
	path, ok, err := a.workspacePath(rel)
	if err != nil || !ok {
		return os.ErrInvalid
	}
	return openWorkspacePath(path)
}

// OpenWorkspacePathForTab opens a path relative to the workspace that owns
// tabID instead of whichever tab happens to be active at click time.
func (a *App) OpenWorkspacePathForTab(tabID, rel string) error {
	base, err := a.workspaceBaseForOwnedTab(tabID)
	if err != nil {
		return err
	}
	path, ok, err := workspacePathForBase(base, rel)
	if err != nil || !ok {
		return os.ErrInvalid
	}
	return openWorkspacePath(path)
}

// OpenWorkspacePathForWorkspace opens a path relative to the workspace stored
// with a historical session, independent of active-tab changes.
func (a *App) OpenWorkspacePathForWorkspace(workspaceRoot, rel string) error {
	base, err := historicalWorkspaceBaseFromRoot(workspaceRoot)
	if err != nil {
		return err
	}
	path, ok, err := workspacePathForBase(base, rel)
	if err != nil || !ok {
		return os.ErrInvalid
	}
	return openWorkspacePath(path)
}

// RevealWorkspacePath shows a workspace file in the native file manager.
func (a *App) RevealWorkspacePath(rel string) error {
	base, err := a.activeWorkspaceBase()
	if err != nil {
		return err
	}
	return revealWorkspacePathFromBase(base, rel)
}

// RevealWorkspacePathForTab reveals a path from the workspace that owns tabID
// instead of whichever workspace is active when the menu action runs.
func (a *App) RevealWorkspacePathForTab(tabID, rel string) error {
	base, err := a.workspaceBaseForOwnedTab(tabID)
	if err != nil {
		return err
	}
	return revealWorkspacePathFromBase(base, rel)
}

func revealWorkspacePathFromBase(base, rel string) error {
	path, ok, err := workspacePathForBase(base, rel)
	if err != nil || !ok {
		return os.ErrInvalid
	}
	return revealPath(path)
}

// RevealPath shows an arbitrary absolute path in the native file manager.
func (a *App) RevealPath(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return os.ErrInvalid
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return revealPath(path)
}

func revealPath(path string) error {
	switch goruntime.GOOS {
	case "darwin":
		return exec.Command("open", "-R", path).Start()
	case "windows":
		// explorer.exe lives in %SystemRoot%, which isn't always on PATH (the
		// launch environment can strip it), so resolve it directly rather than
		// relying on a PATH lookup.
		explorer := "explorer.exe"
		root := os.Getenv("SystemRoot")
		if root == "" {
			root = os.Getenv("windir")
		}
		if root != "" {
			explorer = filepath.Join(root, "explorer.exe")
		}
		return exec.Command(explorer, "/select,", path).Start()
	default:
		dir := path
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			dir = filepath.Dir(path)
		}
		return exec.Command("xdg-open", dir).Start()
	}
}

func (a *App) noticeForTab(tabID, text string) {
	tab := a.tabByID(tabID)
	if tab != nil && tab.sink != nil {
		tab.sink.Emit(event.Event{Kind: event.Notice, Level: event.LevelInfo, Text: text})
	}
}

func (a *App) runEffortCommandForTab(tabID, input string) {
	entry, err := a.currentProviderEntryForTab(tabID)
	if err != nil {
		a.noticeForTab(tabID, "effort: "+err.Error())
		return
	}
	cap := config.EffortCapabilityForEntry(entry)
	if !cap.Supported {
		a.noticeForTab(tabID, fmt.Sprintf("effort is not configurable for %s", entry.Name))
		return
	}
	args := strings.Fields(input)
	if len(args) < 2 {
		a.noticeForTab(tabID, fmt.Sprintf("effort for %s: %s (default: %s; options: %s)", entry.Name, config.EffortDisplay(entry), cap.Default, strings.Join(cap.Levels, "|")))
		return
	}
	if len(args) > 2 {
		a.noticeForTab(tabID, "usage: /effort "+strings.Join(cap.Levels, "|"))
		return
	}
	effort, err := config.NormalizeEffort(entry, args[1])
	if err != nil {
		a.noticeForTab(tabID, err.Error())
		return
	}
	if err := a.SetEffortForTab(tabID, args[1]); err != nil {
		a.noticeForTab(tabID, "effort: "+err.Error())
		return
	}
	display := effort
	if display == "" {
		display = "auto"
	}
	a.noticeForTab(tabID, fmt.Sprintf("effort for %s set to %s", entry.Name, display))
}

func (a *App) currentProviderEntryForTab(tabID string) (*config.ProviderEntry, error) {
	a.mu.RLock()
	ref := ""
	workspaceRoot := ""
	billingSource := modelbilling.BillingSource("")
	effortOverride := (*string)(nil)
	var actor userctx.ActorContext
	if tab := a.tabByIDLocked(tabID); tab != nil {
		ref = tab.model
		workspaceRoot = tab.WorkspaceRoot
		billingSource = modelbilling.BillingSource(tab.billingSource)
		effortOverride = cloneStringPtr(tab.effort)
		actor = tabRuntimeActorContext(tab)
	}
	a.mu.RUnlock()
	cfg, err := a.desktopConfigForBillingRoute(workspaceRoot, actor, billingSource)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(ref) == "" {
		ref = cfg.DefaultModel
	}
	config.NormalizeLegacyMimoCustomProvidersForRefs(cfg, ref)
	resolved, _, ok := cfg.ResolveModelWithFallback(ref)
	if !ok {
		return nil, fmt.Errorf("unknown model %q", ref)
	}
	entry, ok := cfg.ResolveModel(resolved)
	if !ok {
		return nil, fmt.Errorf("unknown model %q", resolved)
	}
	if effortOverride != nil {
		entry.Effort = *effortOverride
	}
	return entry, nil
}

func (a *App) resolvedModelForTab(tab *WorkspaceTab) (string, bool, error) {
	if tab == nil {
		return "", false, fmt.Errorf("no active tab")
	}
	cfg, err := a.desktopConfigForBillingRoute(tab.WorkspaceRoot, tabRuntimeActorContext(tab), modelbilling.BillingSource(tab.billingSource))
	if err != nil {
		return "", false, err
	}
	ref := strings.TrimSpace(tab.model)
	if ref == "" {
		ref = cfg.DefaultModel
	}
	config.NormalizeLegacyMimoCustomProvidersForRefs(cfg, ref)
	resolved, fallback, ok := cfg.ResolveModelWithFallback(ref)
	if !ok {
		return "", false, fmt.Errorf("unknown model %q", ref)
	}
	return resolved, fallback, nil
}

func (a *App) withActiveWorkspace(fn func() (string, error)) (string, error) {
	var result string
	err := a.withActiveWorkspaceDo(func() error {
		var err error
		result, err = fn()
		return err
	})
	return result, err
}

func (a *App) withActiveWorkspaceDo(fn func() error) error {
	desktopWorkspaceCwdMu.Lock()
	defer desktopWorkspaceCwdMu.Unlock()

	root := a.activeWorkspaceRoot()
	if root != "" && root != "." {
		prev, err := os.Getwd()
		if err != nil {
			return err
		}
		if err := os.Chdir(root); err != nil {
			return err
		}
		defer func() { _ = os.Chdir(prev) }()
	}
	return fn()
}

// SavePastedImage stores a browser clipboard image data URL under the active
// tab's workspace .clouvia/attachments and returns the relative @-reference path.
func (a *App) SavePastedImage(dataURL string) (string, error) {
	return a.withActiveWorkspace(func() (string, error) {
		return control.SaveImageDataURL(dataURL)
	})
}

// SaveClipboardImage reads the native OS clipboard image under the active tab's
// workspace .clouvia/attachments and returns the relative @-reference path.
func (a *App) SaveClipboardImage() (string, error) {
	return a.withActiveWorkspace(control.SaveClipboardImage)
}

// SavePastedFile stores a dropped non-image file (the browser exposes its bytes
// as a data URL but not a real path) under the active tab's workspace
// .clouvia/attachments and returns the relative @-reference path.
func (a *App) SavePastedFile(name, dataURL string) (string, error) {
	return a.withActiveWorkspace(func() (string, error) {
		return control.SaveAttachmentDataURL(name, dataURL)
	})
}

// PickExportFile opens the native save dialog and returns the selected path. It
// returns "" when the user cancels.
func (a *App) PickExportFile(defaultFilename, mimeType string) (string, error) {
	if a.ctx == nil {
		return "", nil
	}
	defaultFilename = safeExportFilename(defaultFilename)
	ext := strings.ToLower(filepath.Ext(defaultFilename))
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:                "Export session",
		DefaultDirectory:     dialogDefaultDirectory(a.activeWorkspaceRoot()),
		DefaultFilename:      defaultFilename,
		CanCreateDirectories: true,
		Filters:              exportFileFilters(mimeType, ext),
	})
	if err != nil || path == "" {
		return "", err
	}
	if ext != "" && filepath.Ext(path) == "" {
		path += ext
	}
	return a.rememberExportFile(path)
}

// PickAutomationSourceFile opens the native open dialog for automation CSV/XLSX inputs.
func (a *App) PickAutomationSourceFile(sourceType string) (string, error) {
	if a.ctx == nil {
		return "", nil
	}
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Choose automation data file",
		DefaultDirectory: dialogDefaultDirectory(a.activeWorkspaceRoot()),
		Filters:          automationSourceFileFilters(sourceType),
	})
}

// SaveExportFile writes an exported session payload to a path previously picked
// by PickExportFile. An empty path is treated as a cancelled export.
func (a *App) SaveExportFile(token, payload string, base64Encoded bool) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	path, err := a.consumeExportFileToken(token)
	if err != nil {
		return err
	}
	var data []byte
	if base64Encoded {
		data, err = base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return fmt.Errorf("decode export payload: %w", err)
		}
	} else {
		data = []byte(payload)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	return nil
}

func (a *App) rememberExportFile(path string) (string, error) {
	for range 10 {
		raw := make([]byte, 16)
		if _, err := rand.Read(raw); err != nil {
			return "", fmt.Errorf("create export token: %w", err)
		}
		token := exportFileTokenPrefix + hex.EncodeToString(raw)
		a.exportFilesMu.Lock()
		if a.exportFiles == nil {
			a.exportFiles = make(map[string]exportFilePick)
		}
		now := time.Now()
		a.pruneExpiredExportFilesLocked(now)
		if _, exists := a.exportFiles[token]; !exists {
			a.exportFiles[token] = exportFilePick{path: path, expiresAt: now.Add(exportFileTokenTTL)}
			a.exportFilesMu.Unlock()
			return token, nil
		}
		a.exportFilesMu.Unlock()
	}
	return "", fmt.Errorf("create unique export token")
}

func (a *App) consumeExportFileToken(token string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", nil
	}
	a.exportFilesMu.Lock()
	defer a.exportFilesMu.Unlock()
	now := time.Now()
	a.pruneExpiredExportFilesLocked(now)
	pick, ok := a.exportFiles[token]
	if ok {
		delete(a.exportFiles, token)
	}
	if !ok {
		return "", fmt.Errorf("export file was not picked by this session")
	}
	if now.After(pick.expiresAt) {
		return "", fmt.Errorf("export file pick expired")
	}
	return pick.path, nil
}

func (a *App) pruneExpiredExportFilesLocked(now time.Time) {
	for token, pick := range a.exportFiles {
		if now.After(pick.expiresAt) {
			delete(a.exportFiles, token)
		}
	}
}

func safeExportFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "云智企办-会话.md"
	}
	return filepath.Base(name)
}

func exportFileFilters(mimeType, ext string) []runtime.FileFilter {
	switch mimeType {
	case "text/markdown":
		return []runtime.FileFilter{{DisplayName: "Markdown (*.md)", Pattern: "*.md"}}
	case "application/json":
		return []runtime.FileFilter{{DisplayName: "JSON (*.json)", Pattern: "*.json"}}
	case "application/pdf":
		return []runtime.FileFilter{{DisplayName: "PDF (*.pdf)", Pattern: "*.pdf"}}
	case "image/png":
		return []runtime.FileFilter{{DisplayName: "PNG image (*.png)", Pattern: "*.png"}}
	}
	if ext != "" {
		return []runtime.FileFilter{{DisplayName: strings.ToUpper(strings.TrimPrefix(ext, ".")) + " files (*" + ext + ")", Pattern: "*" + ext}}
	}
	return []runtime.FileFilter{{DisplayName: "All files (*.*)", Pattern: "*.*"}}
}

func automationSourceFileFilters(sourceType string) []runtime.FileFilter {
	switch strings.ToLower(strings.TrimSpace(sourceType)) {
	case "csv":
		return []runtime.FileFilter{{DisplayName: "CSV files (*.csv)", Pattern: "*.csv"}}
	case "xlsx":
		return []runtime.FileFilter{{DisplayName: "Excel workbooks (*.xlsx)", Pattern: "*.xlsx"}}
	default:
		return []runtime.FileFilter{
			{DisplayName: "CSV files (*.csv)", Pattern: "*.csv"},
			{DisplayName: "Excel workbooks (*.xlsx)", Pattern: "*.xlsx"},
		}
	}
}

// AttachmentDataURL returns a safe data URL for a stored image attachment.
func (a *App) AttachmentDataURL(path string) (string, error) {
	return a.withActiveWorkspace(func() (string, error) {
		return control.ImageDataURL(path)
	})
}

// DroppedItem is one OS-dropped file resolved into a composer context entry: an
// in-tree file becomes a workspace @reference (read in place, no copy), while an
// image or out-of-tree file is copied into .clouvia/attachments.
type DroppedItem struct {
	Kind       string `json:"kind"` // "workspace" | "attachment"
	Path       string `json:"path"`
	IsDir      bool   `json:"isDir,omitempty"`
	PreviewURL string `json:"previewUrl,omitempty"`
}

// AttachDropped turns an absolute path from the native file-drop bridge into a
// composer context entry. Images are stored as attachments so the chip shows a
// thumbnail; other in-workspace files are referenced relatively (no copy); files
// outside the workspace are copied into .clouvia/attachments.
func (a *App) AttachDropped(path string) (DroppedItem, error) {
	var item DroppedItem
	err := a.withActiveWorkspaceDo(func() error {
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if isImageExt(path) {
			if rel, err := control.SaveImageFile(path); err == nil {
				preview, _ := control.ImageDataURL(rel)
				item = DroppedItem{Kind: "attachment", Path: rel, PreviewURL: preview}
				return nil
			}
		}
		if rel, ok := workspaceRelativeIn(path, a.activeWorkspaceRoot()); ok {
			item = DroppedItem{Kind: "workspace", Path: rel, IsDir: info.IsDir()}
			return nil
		}
		if info.IsDir() {
			return fmt.Errorf("can only attach files from outside the workspace")
		}
		rel, err := control.SaveAttachmentFile(path)
		if err != nil {
			return err
		}
		item = DroppedItem{Kind: "attachment", Path: rel}
		return nil
	})
	if err != nil {
		return DroppedItem{}, err
	}
	return item, nil
}

func isImageExt(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return true
	}
	return false
}

func workspaceRelativeIn(path, workspaceRoot string) (string, bool) {
	root := workspaceRoot
	if !filepath.IsAbs(root) {
		abs, err := filepath.Abs(root)
		if err != nil {
			return "", false
		}
		root = abs
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// --- memory panel (frontend ⇄ controller) ---

// MemoryDoc is one loaded doc-memory file for the panel: path, scope, and body.
type MemoryDoc struct {
	Path  string `json:"path"`
	Scope string `json:"scope"`
	Body  string `json:"body"`
}

// MemoryFact is one saved auto-memory, surfaced read-only in the panel.
type MemoryFact struct {
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Body        string `json:"body"`
}

// MemoryArchive is one archived auto-memory kept only for inspection.
type MemoryArchive struct {
	Name        string `json:"name"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Body        string `json:"body"`
	Path        string `json:"path"`
	ArchivedAt  string `json:"archivedAt,omitempty"`
}

// MemoryScope is one writable quick-add target (scope id + the file it writes to).
type MemoryScope struct {
	Scope string `json:"scope"`
	Path  string `json:"path"`
}

// MemoryView is the whole memory panel payload: hierarchical docs, active saved
// facts, archived facts, and the writable scopes for the quick-add selector.
type MemoryView struct {
	Docs           []MemoryDoc     `json:"docs"`
	Facts          []MemoryFact    `json:"facts"`
	Archives       []MemoryArchive `json:"archives"`
	Scopes         []MemoryScope   `json:"scopes"`
	StoreDir       string          `json:"storeDir"`
	StoreGlobalDir string          `json:"storeGlobalDir,omitempty"`
	Available      bool            `json:"available"`
}

// writableScopes are the quick-add targets the panel offers, broad → specific.
var writableScopes = []memory.Scope{memory.ScopeUser, memory.ScopeProject, memory.ScopeLocal}

// Memory returns the loaded memory for the panel: the CLOUVIA.md hierarchy,
// compatible legacy memory docs, active/archived auto-memories, and the writable
// scopes. Read-only; mutations go through Remember / SaveDoc.
func (a *App) Memory() MemoryView {
	return a.memoryForTab("", true)
}

// MemoryForTab returns memory for a specific tab's workspace, using its live
// controller when ready and the same file-backed paths while it is still booting.
// If the tab does not exist, returns an empty view instead of falling back to
// the active tab (which would show the wrong data).
// An empty tabID is treated as "no tab specified" and falls back to the
// active tab for backward compatibility.
func (a *App) MemoryForTab(tabID string) MemoryView {
	return a.memoryForTab(tabID, tabID == "")
}

func (a *App) memoryForTab(tabID string, fallback bool) MemoryView {
	view := MemoryView{Docs: []MemoryDoc{}, Facts: []MemoryFact{}, Archives: []MemoryArchive{}, Scopes: []MemoryScope{}}
	_, set := a.memoryTargetForTab(tabID, fallback)
	if set == nil {
		return view
	}
	view.StoreDir = set.Store.Dir
	view.StoreGlobalDir = set.Store.GlobalDir
	view.Available = true
	for _, d := range set.Docs {
		view.Docs = append(view.Docs, MemoryDoc{Path: d.Path, Scope: string(d.Scope), Body: d.Body})
	}
	for _, f := range set.Store.List() {
		view.Facts = append(view.Facts, MemoryFact{
			Name: f.Name, Title: f.Title, Description: f.Description, Type: string(f.Type), Body: f.Body,
		})
	}
	for _, f := range set.Store.ListArchived() {
		archivedAt := ""
		if !f.ArchivedAt.IsZero() {
			archivedAt = f.ArchivedAt.Format(time.RFC3339)
		}
		view.Archives = append(view.Archives, MemoryArchive{
			Name: f.Name, Title: f.Title, Description: f.Description, Type: string(f.Type), Body: f.Body,
			Path: f.Path, ArchivedAt: archivedAt,
		})
	}
	for _, sc := range writableScopes {
		if p := set.DocPath(sc); p != "" {
			view.Scopes = append(view.Scopes, MemoryScope{Scope: string(sc), Path: p})
		}
	}
	return view
}

// memoryTargetForTab resolves both the live session controller and the
// file-backed memory set for a settings-panel operation. Memory files do not
// depend on a model controller, so restored/booting tabs remain manageable even
// while Ctrl is nil. A missing explicit tab never falls back to another
// workspace, which prevents showing or editing the wrong project's memory.
func (a *App) memoryTargetForTab(tabID string, fallback bool) (control.SessionAPI, *memory.Set) {
	a.mu.RLock()
	var tab *WorkspaceTab
	if strings.TrimSpace(tabID) != "" {
		tab = a.tabRuntimes[tabID]
		if tab == nil {
			a.mu.RUnlock()
			return nil, nil
		}
	} else if fallback {
		tab = a.tabRuntimes[a.cachedActiveTabID()]
	}

	if tab != nil {
		ctrl := tab.Ctrl
		workspaceRoot := tabActorWorkspaceRoot(tab)
		actor := tabRuntimeActorContext(tab)
		a.mu.RUnlock()
		if ctrl != nil {
			if set := ctrl.Memory(); set != nil {
				return ctrl, set
			}
		}
		paths := config.NewProfilePathResolver(actorContextForWorkspace(actor, workspaceRoot))
		return ctrl, memory.Load(memory.Options{
			CWD:              workspaceRoot,
			UserDir:          paths.MemoryUserDir(),
			ProjectStoreRoot: paths.MemoryProjectStoreRoot(),
		})
	}
	a.mu.RUnlock()

	if !fallback {
		return nil, nil
	}
	workspaceRoot := globalTabWorkspaceRoot()
	paths := a.currentProfilePathResolver(workspaceRoot)
	return nil, memory.Load(memory.Options{
		CWD:              workspaceRoot,
		UserDir:          paths.MemoryUserDir(),
		ProjectStoreRoot: paths.MemoryProjectStoreRoot(),
	})
}

// Remember quick-adds a one-line note to the doc-memory file for scope — the
// panel's explicit "remember" action, equivalent to typing "/remember <note>".
// An unknown scope falls back to project. Returns the file written.
func (a *App) Remember(scope, note string) (string, error) {
	return a.rememberForTab("", scope, note, true)
}

func (a *App) RememberForTab(tabID, scope, note string) (string, error) {
	return a.rememberForTab(tabID, scope, note, tabID == "")
}

func (a *App) rememberForTab(tabID, scope, note string, fallback bool) (string, error) {
	ctrl, set := a.memoryTargetForTab(tabID, fallback)
	if ctrl != nil && ctrl.Memory() != nil {
		return ctrl.QuickAdd(parseScope(scope), note)
	}
	if set == nil {
		return "", nil
	}
	path := set.DocPath(parseScope(scope))
	if path == "" {
		return "", fmt.Errorf("no target file for memory scope %q", scope)
	}
	if err := memory.AppendDoc(path, note); err != nil {
		return "", err
	}
	return path, nil
}

// Forget deletes a saved auto-memory by name — the panel's delete action for a
// fact the model owns. It writes through the live controller when available and
// directly to the workspace store while a tab is still booting.
func (a *App) Forget(name string) error {
	return a.forgetForTab("", name, true)
}

func (a *App) ForgetForTab(tabID, name string) error {
	return a.forgetForTab(tabID, name, tabID == "")
}

func (a *App) forgetForTab(tabID, name string, fallback bool) error {
	ctrl, set := a.memoryTargetForTab(tabID, fallback)
	if ctrl != nil && ctrl.Memory() != nil {
		return ctrl.ForgetMemory(name)
	}
	if set == nil {
		return nil
	}
	return set.Store.Delete(name)
}

// SaveDoc overwrites a recognized memory doc with the panel editor's contents.
// The live controller applies the edit to the current session when available;
// otherwise the validated file-backed set is updated directly.
func (a *App) SaveDoc(path, body string) (string, error) {
	return a.saveDocForTab("", path, body, true)
}

func (a *App) SaveDocForTab(tabID, path, body string) (string, error) {
	return a.saveDocForTab(tabID, path, body, tabID == "")
}

func (a *App) saveDocForTab(tabID, path, body string, fallback bool) (string, error) {
	ctrl, set := a.memoryTargetForTab(tabID, fallback)
	if ctrl != nil && ctrl.Memory() != nil {
		return ctrl.SaveDoc(path, body)
	}
	if set == nil {
		return "", nil
	}
	return set.WriteDoc(path, body)
}

// parseScope maps a frontend scope id to a memory.Scope, defaulting to project.
func parseScope(s string) memory.Scope {
	switch memory.Scope(s) {
	case memory.ScopeUser:
		return memory.ScopeUser
	case memory.ScopeLocal:
		return memory.ScopeLocal
	default:
		return memory.ScopeProject
	}
}

// onboardingKeyEnv is the official provider key environment name from brand
// metadata.
const onboardingKeyEnv = brand.OfficialAPIKeyEnv

var onboardingModelsURL = config.OfficialProviderModelsURL

type OnboardingModelsResult struct {
	Models       []string `json:"models"`
	DefaultModel string   `json:"defaultModel"`
	UsedFallback bool     `json:"usedFallback"`
	Warning      string   `json:"warning,omitempty"`
}

// NativeConfirmRequest is the payload for ConfirmAction — a native OS confirmation
// dialog that replaces web-style confirm() for destructive or important actions.
type NativeConfirmRequest struct {
	Title        string `json:"title"`
	Message      string `json:"message"`
	Detail       string `json:"detail"`
	ConfirmLabel string `json:"confirmLabel"`
	CancelLabel  string `json:"cancelLabel"`
	Destructive  bool   `json:"destructive"`
}

// ConfirmAction shows a native confirmation dialog and returns true when the user
// clicks the confirm button. For destructive actions the dialog type is Warning so
// the platform can apply its danger styling (red tint on macOS, etc.).
func (a *App) ConfirmAction(req NativeConfirmRequest) (bool, error) {
	if a.ctx == nil {
		return false, nil
	}
	dialogType := runtime.QuestionDialog
	if req.Destructive {
		dialogType = runtime.WarningDialog
	}
	confirm := req.ConfirmLabel
	if confirm == "" {
		confirm = "OK"
	}
	cancel := req.CancelLabel
	if cancel == "" {
		cancel = "Cancel"
	}
	title := req.Title
	if title == "" {
		title = req.Message
	}
	body := req.Message
	if req.Detail != "" {
		if body != "" {
			body += "\n\n" + req.Detail
		} else {
			body = req.Detail
		}
	}
	defaultBtn := confirm
	if req.Destructive {
		// On destructive actions, make cancel the default so Enter / Space
		// does NOT accidentally confirm. ESC always maps to CancelButton.
		defaultBtn = cancel
	}
	result, err := runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type:          dialogType,
		Title:         title,
		Message:       body,
		Buttons:       []string{confirm, cancel},
		DefaultButton: defaultBtn,
		CancelButton:  cancel,
	})
	if err != nil {
		return false, err
	}
	return result == confirm, nil
}

func (a *App) NeedsOnboarding() bool {
	workspaceRoot, actor, source := a.desktopActiveRouteSnapshot()
	if cfg, metadataOnly, err := desktopCatalogConfigForBillingRoute(workspaceRoot, actor, source); err == nil && metadataOnly {
		// A strict subscription (or restored source-less) tab has no BYOK
		// onboarding requirement and must not inspect a provider credential.
		return false
	} else if err == nil && hasConfiguredDesktopProvider(cfg) {
		return false
	}
	return !config.CredentialStoredWithPaths(config.NewProfilePathResolver(actor), onboardingKeyEnv)
}

func hasConfiguredDesktopProvider(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	access := providerAccessSet(cfg.Desktop.ProviderAccess)
	for i := range cfg.Providers {
		p := &cfg.Providers[i]
		if !modelProviderAccessAllowed(access, p.Name) || len(p.ChatModelList()) == 0 || !p.Configured() {
			continue
		}
		return true
	}
	return false
}

func fetchOnboardingModels(ctx context.Context, modelsURL, apiKey string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(modelsURL), nil)
	if err != nil {
		return nil, fmt.Errorf("fetch models: build request: %w", err)
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch models: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return nil, fmt.Errorf("fetch models: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch models: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("fetch models: decode response: %w", err)
	}
	models := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		if strings.TrimSpace(m.ID) != "" {
			models = append(models, m.ID)
		}
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("fetch models: empty model list")
	}
	return models, nil
}

func firstChatModel(models []string) string {
	for _, model := range models {
		model = strings.TrimSpace(model)
		if config.IsLikelyChatModel(model) {
			return model
		}
	}
	return ""
}

// ConnectKey persists the KWU API key to the active profile credentials,
// probes /models, and returns the model choices for onboarding.
func (a *App) ConnectKey(apiKey string) (OnboardingModelsResult, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return OnboardingModelsResult{}, fmt.Errorf("key is required")
	}
	if tab := a.activeTab(); tab != nil && controllerHasActiveRuntimeWork(tab.Ctrl) {
		return OnboardingModelsResult{}, rebuildControllerActiveWorkError("provider key")
	}
	ctx, cancel := context.WithTimeout(a.ctx, 8*time.Second)
	defer cancel()
	models, fetchErr := fetchOnboardingModels(ctx, onboardingModelsURL, apiKey)
	if fetchErr != nil {
		return OnboardingModelsResult{}, fetchErr
	}
	defaultModel := firstChatModel(models)
	if defaultModel == "" {
		return OnboardingModelsResult{}, fmt.Errorf("fetch models: no chat-compatible models returned")
	}
	_, err := a.saveProviderCredential(onboardingKeyEnv, apiKey)
	if err != nil {
		return OnboardingModelsResult{}, fmt.Errorf("save: %w", err)
	}
	return OnboardingModelsResult{
		Models:       models,
		DefaultModel: defaultModel,
	}, nil
}

func selectedOnboardingModels(models []string) []string {
	out := make([]string, 0, len(models))
	seen := map[string]bool{}
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" || seen[model] {
			continue
		}
		seen[model] = true
		out = append(out, model)
	}
	return out
}

func modelInList(model string, models []string) bool {
	for _, candidate := range models {
		if candidate == model {
			return true
		}
	}
	return false
}

func (a *App) CompleteOnboardingModelSelection(defaultModel string, models []string) error {
	if tab := a.activeTab(); tab != nil && controllerHasActiveRuntimeWork(tab.Ctrl) {
		return rebuildControllerActiveWorkError("provider key")
	}
	selected := selectedOnboardingModels(models)
	if len(selected) == 0 {
		return fmt.Errorf("choose at least one model")
	}
	defaultModel = strings.TrimSpace(defaultModel)
	if !modelInList(defaultModel, selected) || !config.IsLikelyChatModel(defaultModel) {
		defaultModel = firstChatModel(selected)
	}
	if defaultModel == "" {
		return fmt.Errorf("choose at least one chat-compatible model")
	}

	cfg, path, err := a.loadDesktopUserConfigForEdit()
	if err != nil {
		return err
	}
	entries, _, err := officialProviderTemplate(brand.ProviderID, "")
	if err != nil {
		return err
	}
	entry := entries[0]
	entry.Models = selected
	entry.Default = defaultModel
	if err := cfg.UpsertProvider(entry); err != nil {
		return err
	}
	cfg.DefaultModel = brand.ProviderID + "/" + defaultModel
	addProviderAccess(cfg, brand.ProviderID)
	if err := cfg.SaveTo(path); err != nil {
		return err
	}
	return a.rebuild()
}

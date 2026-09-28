# Reasonix 政企桌面端测试体系执行清单

> 状态：Proposed
>
> 勾选前必须附命令、结果或评审记录。计划任务不代表已经实现。

## Phase 0：基线与决策

- [x] T0.1 生成 root/desktop/frontend/release 测试资产清单。证据：
  [测试与发布资产清单](test-asset-inventory.md)；
  `node scripts/testing-inventory.test.mjs` 与
  `node scripts/testing-inventory.mjs`（2026-07-23 通过）。
- [x] T0.2 运行并记录当前测试、构建、race、coverage 的耗时和资源。
  证据：[测试执行基线](execution-baseline.md)；Desktop/Frontend 完成三轮，
  Root Go/race/coverage 按 2026-07-23 用户明确决定接受单轮，不计算 P50/P95；
  被取消的第二轮不计为测试失败。
- [ ] T0.3 由产品、交付、安全、研发确认政企环境矩阵。
- [x] T0.4 评审 D-TEST-01：Go 标准库与现有 seam 优先。
- [x] T0.5 评审 D-TEST-02：客户端测试逐级升级策略。
- [x] T0.6 评审 D-TEST-03：browser/WebView2/UIA 分层。
- [x] T0.7 评审 D-TEST-04：安装升级与回滚为发布一级能力。
- [x] T0.8 评审 D-TEST-05：系统签名与更新签名双层信任。
- [x] T0.9 评审 D-TEST-06～07：Frontend 渐进迁移与比例门禁。
  七项工程结论、证据和未完成边界见
  [D-TEST-01～07 工程评审记录](phase-00-decision-review.md)；不替代跨职能
  owner 的正式 ADR 批准。

### Checkpoint M0

- [ ] 基线可重现，所有 Proposed 决策有 owner。
- [ ] 支持矩阵与“不支持/未承诺”边界明确。

## Phase 1：Go 核心与客户端契约

- [x] T1.1 标记数据损坏、越权、不可恢复和现场不可用的 P0 Go 路径。
  证据：[Phase 1 P0 风险登记](phase-01-p0-risk-register.md)。
- [x] T1.2 为 Session/SQLite/状态机/更新器补齐正常、错误和恢复测试。
  证据：[Phase 1 P0 风险登记](phase-01-p0-risk-register.md)；2026-07-23
  定向运行 agent、modelbilling、automation、jobs、cloud、更新器及签名包通过。
- [x] T1.3 用 `httptest` 覆盖认证、429/5xx、超时、断网和错误脱敏。
  已覆盖认证 wire contract、结构化 429/503、超时、断网及敏感字段脱敏；
  覆盖完成不表示产品行为全部正确：审计仍发现超大响应、URL secret、取消和
  无结构化 5xx 分类缺陷。按用户要求不改生产代码，问题记录于
  [Phase 1 测试发现](phase-01-test-findings.md)。
- [x] T1.4 验证代理、自定义 CA 和取消/重试分类。
  代理、取消与重试已由 `internal/netclient`、cloud registration 和 updater
  进行审计；TLS fixture 证明注入自定义 CA 的 `http.Client` 可用。验证同时
  暴露 cloud registration 取消分类缺陷，且产品尚无 CA 配置入口；不虚构
  产品已支持自定义 CA。
- [x] T1.5 记录 `testify/mockery` 接受或拒绝理由。证据：
  [Phase 1 工具采用决定](phase-01-tool-decisions.md)。
- [x] T1.6 仅在采用门触发时 PoC Pact、go-vcr、Testcontainers。当前采用门
  未触发，记录为拒绝引入而非未执行。证据：
  [Phase 1 工具采用决定](phase-01-tool-decisions.md)。

### Checkpoint M1

- [ ] P0 Go/HTTP/持久化边界具备确定性测试。
- [x] 无生产访问、真实客户数据或无必要 Docker 依赖。

## Phase 2：Wails UI 与原生自动化

- [x] T2.1 添加 Vitest/Testing Library 独立 PoC，保留旧 runner。证据：
  [Phase 2 分层测试执行报告](phase-02-layered-test-report.md)。
- [x] T2.2 迁移一个纯函数、组件、feature 和错误路径。单轮复核 4 文件、
  11 测试通过。
- [x] T2.3 添加 3～5 条 Playwright browser smoke。单轮复核 5 条通过。
- [x] T2.4 PoC WebView2 `connectOverCDP`、动态端口和独立 profile。测试脚本
  仅在 `output/` 复制并补丁固定 Wails 2.12.0 依赖；真实 WebView2 页面、
  动态 loopback CDP、隔离 profile、Wails runtime 和清理均已通过。
- [ ] T2.5 证明生产安装包未开放 CDP/诊断端口。生产源码守卫和 frontend
  构建扫描已通过；生产模式 exe 不含 debug port、测试环境变量或
  `connectOverCDP` 标记。最终 NSIS 因本机缺少 `makensis` 未生成，记录为
  `UI-TEST-002`，不把 exe 证据冒充安装器证据。
- [x] T2.6 盘点 Windows UIA tree，试点一个系统对话框或原生窗口。精确
  PID/可执行文件验证、原生主窗口 smoke、270 节点脱敏 UIA tree 和机器校验
  均通过；系统对话框、托盘和安装器仍属于后续矩阵。
- [x] T2.7 分别归档 browser、WebView2 和 native 结果。证据：
  [Phase 2 分层测试执行报告](phase-02-layered-test-report.md)。

### Checkpoint M2

- [x] 四层测试职责清晰且可回滚；最终安装器未完成项保持显式阻塞。
- [x] 关键流程无坐标式或纯视觉式发布断言。

## Phase 3：可靠性、性能与长稳

- [x] T3.1 审计 PR/nightly/release 的 race 覆盖。证据：
  [Phase 3 可靠性、性能与长稳基线](phase-03-reliability-baseline.md)；
  nightly/Desktop/release 缺口记录为 `REL-TEST-001`。
- [x] T3.2 建立 goroutine、句柄、子进程和 SQLite 资源 owner 清单。新增精确
  PID 的进程资源采集器，10 秒 smoke 通过；长时阈值仍未建立。
- [x] T3.3 添加至少 3 个高风险 fuzz target 与 seeds。三个 target 的 seed
  回归和逐 target 固定 50 次输入均通过。
- [ ] T3.4 为冷启动、会话恢复和热点建立 benchmark/benchstat 基线。已归档
  单轮结果及一轮 `count=5` 定向原始样本；真实冷启动和 benchstat 置信比较
  尚未完成。
- [x] T3.5 建立测试专用 pprof/诊断包，生产默认关闭。`internal/testdiag`
  仅在 `testdiag` build tag 下存在；隔离守卫、handler 测试和关闭测试均通过。
- [ ] T3.6 完成 8 小时长稳并采集资源趋势。采集器 PID/可执行文件防误采、
  JSON/CSV schema、路径脱敏与清理 fixture 已通过；3 秒真实进程 smoke 通过，
  但不冒充 8 小时趋势。
- [x] T3.7 为发布候选定义 24/72 小时触发条件。触发条件见 Phase 3 基线；
  实际长稳仍未执行。

### Checkpoint M3

- [ ] race/fuzz/goleak 失败可复现。
- [x] 性能基线记录硬件、OS、工具版本和提交 SHA。
- [ ] 长稳报告可脱敏归档。schema v2 已移除绝对可执行文件路径并有独立
  脱敏 fixture；尚无实际 8 小时报告。

## Phase 4：安装、升级与兼容性

- [x] T4.1 建立支持 OS/架构/权限/路径/网络矩阵。证据：
  [Phase 4 安装、升级与兼容性执行状态](phase-04-installer-compatibility-status.md)；
  未验证项保持显式未承诺。
- [ ] T4.2 在真实 amd64/arm64 环境执行全新安装和首次启动。
- [ ] T4.3 验证 WebView2 缺失/损坏、离线、代理和自定义 CA。
- [ ] T4.4 执行 N-2/N-1 → N 升级与数据迁移。
- [ ] T4.5 模拟下载、签名、文件锁和安装中断。下载断连/续传/取消、签名与
  SHA 失败、缓存篡改、Windows 独占锁和 installer 启动失败测试均通过；
  子安装器启动后的中断无法由当前 API 观察，记录为 `INSTALL-TEST-004`。
- [ ] T4.6 验证安装版/便携版的回滚与降级策略。
- [ ] T4.7 验证卸载和用户数据保留/清除策略。
- [ ] T4.8 在客户相关 EDR/白名单环境验证签名和告警。

### Checkpoint M4

- [ ] 支持矩阵每个 release blocker 均有证据。
- [x] 未验证的平台没有被写成已支持。

## Phase 5：安全、供应链与发布门禁

- [ ] T5.1 落地 Windows Authenticode、时间戳和签名后验证。
- [ ] T5.2 演练 Minisign 密钥轮换、恢复和旧版本兼容。同一签名身份重新加密
  与恢复测试通过；两组临时身份测试证明当前单公钥协议没有新旧客户端兼容
  窗口，记录为 `SUPPLY-TEST-006`，需先评审生产轮换协议。
- [x] T5.3 固定 `govulncheck` 版本并定义发现 SLA/例外。CI 固定
  `v1.6.0`，防回退契约测试和
  [Go 漏洞扫描与例外策略](vulnerability-triage-policy.md)已建立；
  `continue-on-error` 仍保持诊断状态，未冒充 approved required gate。
- [x] T5.4 以报告模式 PoC `gosec` 并审阅误报。固定 `v2.28.0` 对
  Root/Desktop 分别完成扫描，误报/重复候选及六个跟进组记录于
  [Phase 5 gosec 报告模式 PoC](phase-05-gosec-poc.md)。
- [ ] T5.5 对最终安装器及解包目录生成 SBOM。
- [ ] T5.6 固定漏洞数据库快照并生成扫描报告。
- [x] T5.7 建立许可证 allow/deny/review 清单。证据：
  [Frontend 生产依赖许可证策略](frontend-license-policy.md)；机器分类单元测试、
  锁文件报告模式和 release 阻断模式均已复核，GSAP 保持人工 review。
- [ ] T5.8 将 SHA、签名、SBOM、扫描和来源 SHA 归档为证据包。已建立离线
  完整性策略及合成 fixture，验证 14 个证据角色、相对路径、文件存在性和
  SHA-256；尚无最终候选制品和人工批准记录。
- [x] T5.9 更新 `pr-gate` 时同步映射测试。本轮新增测试基础设施均加入
  `GATE_FILES`，`node scripts/pr-gate.test.mjs` 通过。

### Checkpoint M5

- [ ] 发布候选可在无公网环境复核。
- [ ] 未签名、不完整 SBOM 或未处置严重发现阻断发布。

## Phase 6：Skills 与治理

- [x] T6.1 固化 Go、browser、native、安全和发布复核的 skill 路由。证据：
  [Phase 6 Skills 与质量治理执行状态](phase-06-governance-status.md)。
- [x] T6.2 建立客户数据、凭据、网络和可写目录边界。生产凭据、客户数据和
  外部写入保持禁止。
- [x] T6.3 逐个审查第三方 skill，不整包盲装。证据：
  [测试任务 Skill 供应链与权限审查](skill-supply-chain-audit.md)；9 个实际
  使用/指定 skill 已逐项记录来源快照、权限、风险、决策和回滚。
- [x] T6.4 固化首次失败、重试、跳过和人工例外的证据格式。证据：
  [测试证据记录模板](testing-evidence-template.md)。
- [x] T6.5 证明本次限定的 Phase 1、2、6 required gate 可脱离 AI 独立复现。
  `node scripts/testing-phase-gates.mjs --list` 输出机器可读清单，
  `--run phase1|phase2|phase6|all` 执行固定自动门禁；WebView2、UIA 和最终
  安装器作为 environment-qualified gate 显式列出输入与命令，缺少制品时
  保持 blocked，不由 AI 改写为通过。

### Checkpoint M6

- [x] AI 仅辅助生成、探索和分析，不自动批准发布。
- [ ] 所有外部工具与 skill 有固定来源、版本和回滚。

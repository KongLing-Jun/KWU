# Phase 0：基线、范围与 Proposed 决策

## Goal

冻结 Reasonix 当前测试资产、政企交付场景、执行成本和发布信任链，避免把候选工具误写成已接受架构。

本阶段不安装依赖、不修改默认测试命令、不改变发布门禁。

## Task 0.1：建立可重现基线

**Goal**：知道现有测试保护了什么、在哪个 OS/模块运行、耗时多少、失败时留下什么证据。

清单至少包含：路径、所属层、保护行为、环境依赖、入口命令、P50/P95、flake、owner、是否阻断 PR/发布。

分别记录：

- 根 Go module 与 `desktop` module 的 unit、integration、race、goleak 和平台 build tags。
- Frontend 自定义 runner、JSDOM setup、结构契约与用户行为测试。
- Wails binding、更新器、签名、安装器和发布脚本测试。
- CI、nightly、Draft、Publish 与人工审批的真实职责。

## Task 0.2：冻结政企环境矩阵

**Goal**：先定义承诺，再决定测试成本。

产品、交付、安全和研发共同确认：

- 支持的 Windows 版本、架构、WebView2 分发方式和最低硬件。
- 普通用户/管理员、受限目录、中文及空格路径、多用户和漫游配置。
- 互联网、完全离线、HTTP(S) 代理、自定义 CA、DNS 失败和断网恢复。
- 杀毒/EDR、应用白名单、端口限制、临时目录限制和日志导出规则。
- 安装、升级、降级、回滚、卸载、数据保留/清除和灾难恢复责任。
- 国产操作系统只在客户合同、技术栈可行性和真实机器验证均具备后进入支持矩阵。

详细模板见 [政企验收矩阵](enterprise-acceptance-matrix.md)。

## Task 0.3：记录执行基线

至少连续运行三次相关套件，记录冷/热缓存、CPU、内存、句柄、goroutine、磁盘和日志大小。基线阶段只收集 coverage 与性能数据，不先设任意阈值。

```powershell
go test ./...
go test -race ./...

Push-Location desktop
go test ./...
Pop-Location

pnpm --dir desktop/frontend test:all
pnpm --dir desktop/frontend build
```

## Task 0.4：Proposed 决策登记

以下均保持 `Proposed`，经 PoC 和团队评审后才可转为正式 ADR。

### D-TEST-01：Go 测试使用标准库与现有 seam 优先

- **候选决定**：继续以 `testing`、table tests、真实临时目录/SQLite 和小型手写 fake 为默认。
- **原因**：仓库已有大量成熟 Go 测试；强制迁移到 `testify/mockery` 会增加生成物和审阅成本。
- **引入条件**：重复断言或 mock 维护成本已被测量，且 PoC 明显降低复杂度。

### D-TEST-02：客户端测试采用逐级升级策略

- **候选决定**：`httptest`/本地 fixture → 跨团队契约 PoC → 脱敏录制回放；仅在真实外部基础设施出现时评估 Testcontainers。
- **原因**：当前桌面客户端以 SQLite 和 HTTP 外部服务为主，不应默认依赖 Docker 或生产网络。
- **约束**：录制数据不得来自客户会话，不得含 token、手机号、Cookie、支付或模型输入。

### D-TEST-03：浏览器、WebView2 与 Windows UIA 分层

- **候选决定**：Vitest/JSDOM 验证组件；Playwright browser 验证 Web UI；测试专用 WebView2 CDP 验证嵌入内容；UIA 工具验证系统窗口和对话框。
- **原因**：四层运行表面不同，不能互相冒充。
- **约束**：CDP/诊断端口不得进入生产构建。

### D-TEST-04：安装、升级和回滚是发布一级能力

- **候选决定**：对受支持 OS/架构执行全新安装、覆盖升级、失败中断、回滚、卸载和数据保留矩阵。
- **原因**：政企终端的权限、路径、EDR 和离线限制是主要交付风险。
- **替代方案**：只验证安装包能构建；拒绝，因为不能证明现场可交付。

### D-TEST-05：发布制品建立双层信任证据

- **候选决定**：保留现有 Minisign 更新制品校验，同时补齐 Windows Authenticode、SHA-256、SBOM、漏洞/许可证报告和来源 SHA。
- **原因**：应用内更新签名不能替代操作系统信任和企业安全软件验证。
- **约束**：签名顺序、证书保管、时间戳和密钥轮换必须单独评审。

### D-TEST-06：Frontend runner 只做渐进式 PoC

- **候选决定**：Vitest/Testing Library 双运行迁移；Playwright 只覆盖少量关键链路。
- **原因**：统一 lifecycle、过滤、coverage 和诊断有价值，但不是当前最高交付风险。
- **拒绝条件**：破坏 115 个现有测试的可执行性或需要大规模产品重构。

### D-TEST-07：比例门禁与分层执行

- **候选决定**：PR 运行定向确定性测试；nightly 承担全量 race/fuzz/8 小时长稳；release 承担安装矩阵、24/72 小时长稳和制品信任验证。
- **原因**：控制反馈时间，同时保留高风险门禁。
- **约束**：`scripts/pr-gate.mjs` 与映射测试必须同步，未知路径继续 fail closed。

## Task 0.5：数据与审计边界

- 不使用真实客户数据、生产凭据、验证码、支付凭证或云端模型输入。
- test profile、SQLite、端口、证书库和用户数据目录必须隔离。
- trace、截图、日志、dump、pprof 和 fuzz corpus 上传前必须脱敏。
- 工具、Action、二进制和漏洞数据库版本必须固定并可离线镜像。
- 任何 AI/skill 输出都只是建议或草案，不能直接批准发布。

## 阶段退出门

- [x] 测试与发布资产清单可重现。
- [ ] 政企环境矩阵有产品、交付、安全和研发 owner。
- [x] 现有执行耗时、资源和失败证据已基线化。
- [x] D-TEST-01～07 均有接受、拒绝或待 PoC 记录。证据：
  [工程评审记录](phase-00-decision-review.md)。
- [x] 没有把 Proposed 决策描述成已实现事实。

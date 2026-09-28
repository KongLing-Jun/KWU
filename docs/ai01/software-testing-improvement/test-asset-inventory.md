# 测试与发布资产清单

> 状态：Baseline
>
> 基线提交：`15a98177251bd024b499206e9d90b979fbc48436`
>
> 采集日期：2026-07-23
>
> 重现命令：`node scripts/testing-inventory.mjs`

本清单对应 T0.1。数量由仓库脚本实时扫描，职责和阻断级别来自
`AGENTS.md`、`scripts/pr-gate.mjs` 及当前 CI workflow。P50/P95、flake
和资源数据由 T0.2 的实测结果补充；样本不足时不计算百分位，未测量字段不填
推测值。

## 1. 自动化资产

| 层 | 当前资产 | 主要保护行为 | 环境依赖 | 本地入口 | PR/发布职责 | Owner |
|---|---:|---|---|---|---|---|
| Root Go | 496 个测试文件，79 个目录 | 核心业务、agent/control、持久化、工具、网络客户端 | Go 1.25；部分测试按 OS/build tag 分层 | `go test ./...` | 核心或广泛变更运行全量；其他 Go 变更按包运行 | `@SivanCola` |
| Desktop Go | 74 个测试文件，4 个目录 | Wails binding、窗口生命周期、更新器、签名、工作区和发布契约 | Desktop Go module；部分构建依赖 Wails/GTK/Windows | `Push-Location desktop; go test ./...; Pop-Location` | Desktop 核心、依赖或发布变更运行全量；其他按包运行 | `@SivanCola` |
| Frontend | 119 个测试文件 | React 状态、可访问性、结构契约、用户流程 | Node 24、pnpm、JSDOM | `pnpm --dir desktop/frontend test:all` | 共享或未映射源文件运行全量；映射表命中时定向运行 | `@SivanCola` |
| Release contract | 26 个脚本测试 | 架构、凭据、Draft/Publish、恢复、阶段门禁、UIA、长稳采集脱敏、离线证据完整性、许可证和安全工具策略 | PowerShell/bash/Node；部分仅在对应 OS 执行 | `node scripts/pr-gate.mjs --base origin/main-v3 --run frontend` | 发布路径变化时阻断 PR；真实签名与发布仍需人工审批 | `@SivanCola` |

## 2. 已有可靠性能力

| 能力 | 当前事实 | 入口或位置 | 缺口 |
|---|---|---|---|
| `httptest` | 48 个 Go 测试文件使用本地 HTTP fixture | 各客户端/云服务包测试 | 需要按认证、429/5xx、取消、代理和自定义 CA 逐项审计 |
| `goleak` | 5 个 `TestMain` | `internal/agent`、`control`、`jobs`、`lsp`、`plugin` | Desktop 与其他长期运行 owner 尚未形成清单 |
| benchmark | 11 个 benchmark，分布在 8 个测试文件 | Root 与 Desktop Go tests | 缺少统一环境记录和 benchstat 基线 |
| native fuzz | 3 个 target | TaskTree JSON、云响应 envelope、Office relationship XML | 仅完成 seed 回归与逐 target 固定 50 次输入；尚未进入 nightly |
| build tags | 22 个测试文件 | Windows、非 Windows、live、manual、testdiag | live/manual/testdiag 不属于默认 PR 证据 |
| race | Root 广泛变更在 CI 执行 full race | `.github/workflows/ci.yml` | Nightly 未显式执行 full race；Desktop race 范围未冻结 |
| coverage | Root 广泛变更生成 atomic coverage | `.github/workflows/ci.yml` | Desktop/Frontend 尚无统一 coverage 证据 |

## 3. Workflow 职责

| Workflow | 当前职责 | 明确不证明 |
|---|---|---|
| `ci.yml` | 变更分类、guard、比例 Go/Desktop/Frontend gate、root full-change race/coverage、诊断性 `govulncheck` | Windows 真机安装、长稳、EDR、正式 Authenticode |
| `nightly-go-regression.yml` | Root、Desktop、Frontend 的定时回归 | 当前没有 fuzz、8 小时长稳和显式 full race |
| `desktop-build-draft.yml` | 生成并暂存候选 Windows/macOS 制品、架构检查 | 生产发布批准和客户环境验收 |
| `desktop-publish.yml` | 人工确认后的不可变来源发布、更新签名及发布验证 | 未启用的 Authenticode、完整 SBOM/许可证结论 |
| `desktop-release-doctor.yml` | 发布环境和 OSS 可用性诊断 | 不能替代最终制品签名及安装矩阵 |

## 4. 尚未形成基线的证据

- Root Go/race/coverage 的多轮 P50/P95；本次按用户决定只保留单轮观测。
- goroutine、磁盘和日志大小。
- Windows amd64/arm64 真机首次启动、安装、升级、卸载和回滚结果。
- 离线、代理、自定义 CA、WebView2 缺失/损坏和客户 EDR 结果。
- Authenticode、最终安装器 SBOM、漏洞数据库快照和许可证审批。

这些字段保持“未测量/未验证”，不得由单次绿色 CI 或浏览器 smoke 推导为完成。

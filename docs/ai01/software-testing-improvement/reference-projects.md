# 软件测试参考项目与采用边界

> 调研快照：2026-07-23
>
> 本文是 Reasonix 适配结论，不是依赖批准清单。采用前重新核验 release、许可证、运行约束、传递依赖和安全状态。

## 1. 直接使用或强化现有能力

| ID | 项目/能力 | Reasonix 用途 | 结论 |
|---|---|---|---|
| `REF-GO-TESTING` | [Go testing](https://pkg.go.dev/testing) | table tests、benchmark、临时目录 | 默认首选 |
| `REF-HTTPTEST` | [Go httptest](https://pkg.go.dev/net/http/httptest) | 云 API、错误、超时、离线和协议 fixture | P0 |
| `REF-GO-RACE` | [Go Race Detector](https://go.dev/doc/articles/race_detector) | 并发路径和生命周期 | 强化 PR/nightly 分层 |
| `REF-GO-FUZZ` | [Go Fuzzing](https://go.dev/doc/security/fuzz/) | 解析、路径、manifest、持久化边界 | P0 增量能力 |
| `REF-GOLEAK` | [uber-go/goleak](https://github.com/uber-go/goleak) | goroutine owner 与退出验证 | 已有依赖，按 owner 扩展 |
| `REF-PPROF` | [google/pprof](https://github.com/google/pprof) | CPU、heap、goroutine 诊断 | 测试/诊断构建使用 |
| `REF-BENCHSTAT` | [benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat) | 基准统计比较 | P1，用于稳定热点 |
| `REF-GOVULNCHECK` | [govulncheck](https://go.dev/doc/tutorial/govulncheck) | Go 可达漏洞检查 | 已有 CI，需固定版本和治理 |
| `REF-POWERSHELL` | [Pester](https://pester.dev/) | Windows 文件、注册表、进程和安装验证 | 优先于复杂 UI 自动化 |

## 2. 前端与 Wails 分层候选

| ID | 项目/能力 | 可借鉴点 | Reasonix 边界 |
|---|---|---|---|
| `REF-VITEST` | [Vitest](https://github.com/vitest-dev/vitest) | Vite/TSX、过滤、coverage、reporter | 双运行 PoC，不一次迁移全部测试 |
| `REF-RTL` | [React Testing Library](https://github.com/testing-library/react-testing-library) | role/label/text 与用户行为 | 新测试和触达迁移使用 |
| `REF-PLAYWRIGHT` | [Playwright](https://github.com/microsoft/playwright) | browser E2E、trace、截图 | 只覆盖少量关键链路 |
| `REF-PLAYWRIGHT-WEBVIEW2` | [Playwright WebView2](https://playwright.dev/docs/webview2) | CDP 连接、独立 user data folder | 测试专用；CDP 保真度有限；生产禁用 |
| `REF-WAILS` | [Wails](https://github.com/wailsapp/wails) | Wails/WebView 运行边界 | 项目固定 v2.12.0；调研不等于升级批准 |
| `REF-WINAPPCLI` | [microsoft/winappCli UI automation](https://github.com/microsoft/winappCli/blob/main/docs/ui-automation.md) | UIA tree、AutomationId、窗口和系统对话框 | 试点；先验证 Wails 暴露的 UIA tree |
| `REF-PYWINAUTO` | [pywinauto](https://github.com/pywinauto/pywinauto) | Windows GUI/UIA 自动化 | 团队接受 Python 且 UIA 稳定时 PoC |
| `REF-FLAUI` | [FlaUI](https://github.com/FlaUI/FlaUI) | .NET UIA2/UIA3 | 仅在 .NET 工具链更适合时 PoC |

## 3. 安全与供应链候选

| ID | 项目/能力 | 用途 | 采用边界 |
|---|---|---|---|
| `REF-GOSEC` | [securego/gosec](https://github.com/securego/gosec) | Go 静态安全分析 | 先报告模式，审阅与 CodeQL/lint 重复 |
| `REF-SYFT` | [anchore/syft](https://github.com/anchore/syft) | 对文件系统/制品生成 SPDX/CycloneDX SBOM | PoC 最终安装器与解包目录；固定版本 |
| `REF-GRYPE` | [anchore/grype](https://github.com/anchore/grype) | 基于 SBOM 的漏洞扫描 | 固定工具和 DB 快照；人工核对误报/漏报 |
| `REF-CYCLONEDX` | [CycloneDX](https://cyclonedx.org/) | 政企可交换 SBOM 格式 | 与客户/合规要求确认格式和版本 |
| `REF-AUTHENTICODE` | [SignTool](https://learn.microsoft.com/windows/win32/seccrypto/signtool) | Windows 发布者签名与时间戳 | 生产证书和密钥治理落地后成为 release blocker |

## 4. 条件试点，不设为默认 P0

| ID | 项目/包 | 何时有价值 | 当前结论 |
|---|---|---|---|
| `REF-TESTIFY` | [stretchr/testify](https://github.com/stretchr/testify) | 重复断言显著影响可读性 | 不强制迁移现有 Go 测试 |
| `REF-MOCKERY` | [vektra/mockery](https://github.com/vektra/mockery) | 大量稳定接口需要生成 mock | 先用小型 fake；需治理生成物 |
| `REF-PACT-GO` | [Pact Go](https://github.com/pact-foundation/pact-go) | 客户端/服务端独立发布且可做 provider verification | 只有跨团队闭环后采用 |
| `REF-GO-VCR` | [dnaeon/go-vcr](https://github.com/dnaeon/go-vcr) | 第三方复杂协议难以手写 fixture | 仅合成账号与强脱敏 cassette |
| `REF-TESTCONTAINERS` | [testcontainers-go](https://github.com/testcontainers/testcontainers-go) | 新增真实 PostgreSQL/Redis/队列等依赖 | 当前 SQLite 场景不优先 |
| `REF-RAPID` | [flyingmutant/rapid](https://github.com/flyingmutant/rapid) | 状态机/round-trip 可生成并 shrink | 一个不变量 PoC；不替代 fuzz |
| `REF-MSW` | [MSW](https://github.com/mswjs/msw) | Frontend 重复 HTTP fixture | 重复成本出现后引入 |
| `REF-K6` | [Grafana k6](https://github.com/grafana/k6) | 独立服务端 API 负载测试 | 不用于桌面 UI；Reasonix 客户端本身非首选 |

## 5. 暂缓或排除

| 方向 | 结论 | 原因 |
|---|---|---|
| Fyne test | 排除 | 当前项目是 Wails，不是 Fyne |
| WinAppDriver 作为主链路 | 暂缓 | 维护和生态不确定，不应新建核心依赖 |
| 坐标式 UI 自动化 | 排除关键门禁 | DPI、分辨率、主题和语言变化下脆弱 |
| AI 视觉作为发布裁决 | 排除 | 缺少确定性和可审计断言 |
| Aider/OpenHands/vLLM 作为测试框架 | 不纳入核心路线 | 属于 Agent/模型基础设施，不解决测试执行与发布证据本身 |
| 自建大型 Wails driver | 暂缓 | 先用现有 binding tests、Playwright CDP PoC 与 UIA 分层 |

## 6. 推荐采用顺序

1. 强化现有 Go `testing`、`httptest`、race、goleak 和发布脚本。
2. 增加高风险 Go fuzz target 与性能/长稳基线。
3. 补齐安装、升级、卸载、离线、权限和 EDR 验收矩阵。
4. 落地 Authenticode、SBOM、漏洞与许可证证据。
5. Vitest/Testing Library 双运行 PoC。
6. Playwright browser 与 WebView2 CDP 少量 smoke。
7. Windows UIA 工具按真实原生场景试点。
8. Pact、go-vcr、Testcontainers 等只在采用门触发后评估。

## 7. 每次采用记录

```text
Reference ID:
Repository / release / commit checked:
Package and transitive dependencies:
License and security result:
Reasonix gap addressed:
PoC files and commands:
Measured runtime / stability:
Customer environment impact:
What is adopted:
What is rejected:
Rollback:
Decision / ADR:
```

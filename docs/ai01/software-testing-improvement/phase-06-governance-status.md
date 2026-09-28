# Phase 6 Skills 与质量治理执行状态

> 执行日期：2026-07-23
> 原则：skill 只约束工作方法，固定命令和人工授权才产生门禁结论。

## 1. 本任务的能力路由

| 场景 | 使用/适用能力 | 输入边界 | 必须输出 | 门禁资格 |
|---|---|---|---|---|
| 增量测试实现 | `incremental-implementation` | 仓库代码和测试计划 | 小批次 diff、逐层验证 | 无 |
| 最小改动 | `karpathy-guidelines` | 目标 seam、已有测试 | 假设、最小测试改动 | 无 |
| 失败诊断 | `debugging-and-error-recovery` | 命令、stderr、进程证据 | 根因、首次失败、复核结果 | 无 |
| Browser smoke | `playwright` | 测试 fixture、DOM、脱敏网络摘要 | locator、trace、截图、console | 仅测试证据 |
| WebView2/native | 固定 PowerShell + Playwright CDP probe | 精确 exe/PID、隔离 profile | 端口、进程树、窗口、清理结果 | 仅对应层 |
| 发布/安全 | 仓库固定脚本和 checklist | 候选制品、无生产密钥的只读信息 | SHA、签名、SBOM、扫描、审批 | 人工批准后 |

本轮没有安装 QA skill 集合，也没有让浏览器/视觉 skill 决定发布。已对实际
使用或用户明确指定的 9 个 skill 逐项完成
[供应链与权限审查](skill-supply-chain-audit.md)，结论包含条件接受和拒绝进入
required gate。统一 commit、许可证、release 和维护 owner 仍是 Checkpoint M6
缺口。

## 2. 数据、凭据与写入边界

允许：

- 仓库源码、合成 fixture、临时目录和 `output/` 下测试证据；
- 本地 loopback 服务、动态端口、独立 profile；
- 精确 PID/可执行文件的只读进程和窗口信息；
- 不含 query、token、Cookie、手机号、密码和支付信息的摘要。

禁止：

- 客户数据、生产日志、完整会话、真实手机号/密码、Cookie、token、支付信息；
- 生产发布凭据、Minisign 私钥、代码签名证书和客户管理员权限；
- 未经授权的发布、上传、密钥轮换、注册表/真实数据修改；
- 把真实失败通过 skip、ignore、扩大超时或放宽断言隐藏。

本轮没有读取或打印生产密钥，没有发布或外部写入。CDP 和 soak 脚本只使用
测试输出目录；现有桌面 PID 27268 保持运行。

## 3. 证据格式

新增 [测试证据记录模板](testing-evidence-template.md)，强制区分：

- first result 与 retry result；
- pass、fail、blocked、timeout、skipped 和 not-run；
- 已证明与未证明；
- 工具版本、环境、权限、source SHA 和制品 SHA；
- 敏感信息复核、owner、到期日和人工审批。

Phase 2 的首次 browser locator 失败、WebView2 CDP 首次失败，Phase 3 的并行 fuzz
超时，以及 Phase 4/5 的 PowerShell/工具缺口均按该原则保留，没有被后续成功
覆盖。

## 4. 确定性复现

本次用户限定的 Phase 1、2、6 具有统一的机器可读入口：

```text
node scripts/testing-phase-gates.mjs --list
node scripts/testing-phase-gates.mjs --run phase1
node scripts/testing-phase-gates.mjs --run phase2
node scripts/testing-phase-gates.mjs --run phase6
node scripts/testing-phase-gates.mjs --run all
```

清单 schema v1 把 automatic gate 与 environment-qualified gate 分开。自动项
固定可执行文件、参数和工作目录；WebView2 CDP、Windows UIA 和最终安装器检查
固定所需输入与命令。缺少 exe、PID 或 `makensis` 时结果必须是 blocked，不能
由 AI 解释为 pass。

首次执行 Phase 2 runner 在 Windows/Node 26 直接
`spawnSync("pnpm.cmd")` 时返回 `EINVAL`，测试命令并未启动。最小复现确认
同一命令经 `cmd.exe /d /s /c` 正常返回；runner 改为只对固定的 `.cmd`
入口使用该调用方式，并新增参数构造回归断言。复核后 Phase 2 自动门禁
6/6 通过。该首次失败保留为测试基础设施证据，不归类为 Vitest 失败。

WebView2 的环境限定门禁已用固定 Wails 2.12.0 的 output-only 测试构建成功
复现；生产源码和依赖声明不变。Windows UIA 也已用精确 PID/exe 和机器校验器
复现。最终 NSIS 仍因本机缺少 `makensis` 阻塞。

以下不属于本次用户限定的 Phase 1、2、6 自动执行范围，仍不得写成通过：

- 8/24/72 小时长稳（采集器 schema 与脱敏 fixture 已可独立复现）；
- PowerShell 7 release fixture；
- Authenticode、SBOM、漏洞数据库和许可证审批；
- amd64/arm64 真实安装/升级/卸载矩阵。

证据包完整性和 SHA 校验已可脱离 AI 在离线环境运行，但它不验证报告内部
结论或人工授权。本次限定范围的 T6.5 已完成；Checkpoint M6 仍因所有外部
工具/skill 尚未统一固定 release、许可证和维护 owner 保持未完成。

## 5. 未关闭治理问题

### GOV-TEST-001

- 严重度：Medium。
- 问题：本地 skills 没有统一固定 commit/release、许可证和维护 owner。
- 处理：本轮已逐个审查并固定本地 `SKILL.md` 哈希；详情及新增发现见
  [供应链与权限审查](skill-supply-chain-audit.md)。skill 不直接进入
  required gate。

### GOV-TEST-002

- 严重度：High（发布治理）。
- 问题：多个 release required 证据仍依赖缺失工具、目标环境或人工政策。
- 处理：未完成项保持 blocked/not-run，AI 不得自动接受例外或批准发布。

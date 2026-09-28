# Phase 6：Skills 与质量治理

## Goal

让 Agent/skills 辅助生成测试、探索 UI、汇总日志和分析性能，同时确保结果可复现、权限受控、数据不外泄。

## Task 6.1：按任务路由 skill

| 任务 | 首选能力 | 必须输出 |
|---|---|---|
| Go 缺陷复现 | debugging/TDD 类 skill | 失败测试、根因、修复后结果 |
| 测试计划 | planning/project-structure 类 skill | 入口、调用链、风险、Task/Goal |
| 浏览器 smoke | Playwright/browser skill | locator、trace、截图、console |
| Windows native 探索 | UIA 工具 skill/脚本 | HWND/UI tree、动作、退出码 |
| 安全审查 | security/source-driven 类 skill | 官方来源、发现、严重度、例外 |
| 发布复核 | 固定脚本和 checklist | SHA、签名、SBOM、命令与审批 |

Skill 只规定工作方法，不等于测试通过证据。

## Task 6.2：AI 数据边界

- 禁止向外部模型发送客户数据、生产日志、凭据、token、Cookie、支付或完整会话。
- 内网模型也只读取最小必要、脱敏、只读上下文。
- Agent 不得拥有生产发布凭据、代码签名私钥或客户环境管理员权限。
- 外部联网、命令白名单、可写目录和保留日志由执行环境显式控制。
- 生成的测试数据必须为合成数据并可公开审查。

## Task 6.3：确定性门禁

AI 可以：

- 生成测试草案和边界用例。
- 分析 race、pprof、trace、dump 和 UI tree。
- 聚类失败、识别疑似 flake、提出修复方向。
- 辅助探索新页面和选择稳定 locator。

AI 不可以：

- 自由视觉判断后直接批准关键政务流程。
- 把重跑成功改写为首次通过。
- 自动放宽断言、加入 `skip` 或扩大 ignore。
- 自动接受漏洞、许可证或签名例外。
- 自动发布、轮换密钥或修改生产数据。

所有 required gate 由固定命令、机器可读断言和授权人工审批执行。

## Task 6.4：第三方 skill 审查

逐个记录：

- 来源、commit/release、许可证和维护状态。
- 安装脚本、网络访问、命令执行、文件读写和凭据权限。
- 传递依赖、遥测、数据保留和供应链风险。
- Reasonix 具体缺口、PoC、测量结果和回滚。

不得整包盲装 QA skill 集合，不因 star 或 README 宣传直接进入 CI。

## Task 6.5：证据模板

```text
Task / Goal:
Source commit and worktree:
Affected module and risk:
Tool / skill and pinned version:
Environment and permissions:
Commands executed:
First result:
Retry result:
Artifacts:
Sensitive-data review:
Known gaps / skipped checks:
Human reviewer:
Decision / ADR:
```

## 阶段退出门

- [x] 每个本任务使用的 skill 有明确 trigger、权限、输入和输出证据。
- [x] AI 无生产发布凭据和客户数据访问权。
- [x] 本次限定的 Phase 1、2、6 required gate 可脱离 AI 独立复现；需要
  exe、PID 或安装器的环境限定项在缺少输入时明确 blocked。
- [x] 第三方 skill 逐个审查，无整包盲装。
- [x] 失败、重试、跳过和人工例外分别记录。

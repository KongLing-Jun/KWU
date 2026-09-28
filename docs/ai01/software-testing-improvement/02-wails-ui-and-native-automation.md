# Phase 2：Wails UI 与原生自动化

## Goal

分别验证 React 组件、普通浏览器、Wails/WebView2 内容和 Windows 原生系统交互；任何一层都不能替代其他层。

## 测试层级

| 层 | 工具候选 | 负责证明 | 不负责证明 |
|---|---|---|---|
| React unit/component | 现有 runner；Vitest + Testing Library PoC | 状态、角色、焦点、输入、错误提示 | 浏览器布局和原生能力 |
| Browser E2E | Playwright | Vite/React 路由、网络 fixture、真实 DOM 交互 | Wails binding、托盘、注册表 |
| WebView2 content | Playwright `connectOverCDP` PoC | 嵌入 WebView 中的页面与 binding 可见行为 | 安装器和系统对话框 |
| Windows native | UIA 工具或 PowerShell/Pester | 窗口、文件对话框、托盘、进程、安装结果 | Web 页面内部复杂断言 |

## Task 2.1：Frontend runner 渐进 PoC

**Goal**：解决过滤、统一 lifecycle、coverage 和 reporter，不破坏现有 115 个测试。

- 添加独立 Vitest 入口并保留 `scripts/run-tests.mjs`。
- 首批只迁移一个纯函数、一个组件行为、一个 feature slice 和一个错误路径。
- 使用 role、label、text、焦点和可见状态断言。
- Windows 与 CI Linux 双运行，收集稳定性和耗时后再决定默认入口。
- 不进行全仓机械迁移，不以 coverage 数字驱动无意义测试。

## Task 2.2：Browser smoke

**Goal**：以少量链路验证真实浏览器行为。

首批 3～5 条：

1. 启动、主导航和基础可访问性。
2. 登录失败提示、重试和切换模型服务。
3. 模型不可用/离线状态与发送可用性。
4. 工作区或会话恢复。
5. 一个受控文件选择前的 Web 层流程。

使用本地 fixture；保存 trace、截图、console 和脱敏网络摘要。定位器以 role、label、稳定 test id 为主，禁止坐标点击作为关键门禁。

## Task 2.3：WebView2 CDP PoC

**Goal**：验证 Playwright 能否稳定连接 Reasonix 的 Wails WebView2 内容。

官方 Playwright WebView2 方案通过 `WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS=--remote-debugging-port=<port>` 和 `connectOverCDP` 接入，并要求每个 worker 使用独立用户数据目录。

Reasonix 约束：

- 仅测试进程注入环境变量，不修改生产默认值。
- 端口动态分配并只绑定 loopback。
- 每个测试使用独立 `WEBVIEW2_USER_DATA_FOLDER`、应用 profile 和 SQLite。
- 启动后校验真实可执行文件路径、PID、窗口和 CDP endpoint。
- 结束时关闭整个进程树并清理 profile。
- 构建检查必须证明正式安装包未包含远程调试开关。

CDP 连接的保真度低于 Playwright 原生协议；高级浏览器能力失败时不得错误归类为产品缺陷。

## Task 2.4：Windows UIA 工具试点

**Goal**：验证文件对话框、窗口、托盘和安装器等 Web 层无法覆盖的能力。

候选顺序：

1. PowerShell/Pester 和现有项目脚本：进程、文件、注册表、安装结果。
2. `winappCli`：UIA tree、AutomationId、窗口和系统对话框 smoke；当前按试点管理。
3. `pywinauto` 或 FlaUI：只有 UIA tree 和团队语言/维护成本适配时采用。

先运行可访问性树盘点。WebView 内容若未暴露稳定 UIA 节点，则 UIA 只负责原生窗口/系统对话框，页面内部继续由 Playwright 负责。

## Task 2.5：关键政企流程规则

身份认证、权限审批、文件导入导出、删除、升级确认等流程必须使用确定性 locator 和明确断言。AI 视觉可辅助探索失败现场，但不得作为唯一执行器或发布裁决者。

## 阶段退出门

- [x] React、browser、WebView2 和 native 四层结果分别报告。
- [x] 现有 frontend runner 保留可回滚路径。
- [x] WebView2 CDP 只存在于测试环境且 profile/端口隔离。
- [x] 至少一个系统对话框或原生窗口场景在 Windows runner 可重现。
- [x] 关键流程没有坐标式或纯视觉式发布断言。

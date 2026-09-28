# Phase 2 分层测试执行报告

> 执行日期：2026-07-23
> 源码提交：`15a98177251bd024b499206e9d90b979fbc48436`
> 分支：`codex/integration-main-v3-20260723`
> 约束：本阶段只修改测试、测试基础设施和文档，不修改生产实现。

## 结论

React、普通浏览器、真实 WebView2 内容和 Windows 原生窗口四层均已有可重复
的独立证据。WebView2 PoC 使用仅存在于 `output/` 的 Wails 2.12.0 测试副本，
把动态 loopback CDP 参数注入测试构建；生产源码、`go.mod` 和正常 Wails
依赖均未修改。最终 NSIS 安装器隔离检查仍因本机缺少 `makensis` 保持阻塞，
不能用生产模式 exe 或源码扫描冒充安装器证据。

## React 组件层

新增独立 Vitest/Testing Library 入口，保留原有 `scripts/run-tests.mjs`：

- 纯函数：发送可用性判定。
- 组件：登录提醒与操作入口。
- feature：账户状态 store。
- 错误路径：历史记录加载失败。

执行结果：

```text
pnpm --dir desktop/frontend test:vitest
4 files passed, 11 tests passed

pnpm --dir desktop/frontend test:vitest:coverage
statements 33.83%, branches 27.01%, functions 48.48%, lines 35.59%
```

覆盖率只统计本次 PoC 选定的四个生产模块，不代表整个 frontend 的覆盖率。
旧 runner 单轮复核耗时 174.1 秒并以退出码 0 完成：

```text
pnpm --dir desktop/frontend test:all
exit 0
```

## 普通浏览器层

Playwright 使用测试专用 Vite fixture、动态受控状态和语义定位器，覆盖：

1. 启动、主导航和 landmark。
2. 登录提醒、重试和切换模型。
3. 模型离线时禁止发送。
4. 工作区状态重载恢复。
5. 受控文件选择。

首次执行有 2 条失败，原因是宽泛的 `role=status` 同时命中状态条和输出节点；
定位器收紧为 role 与可见文本组合后，单轮复核结果为：

```text
pnpm --dir desktop/frontend test:browser
5 passed (5.5s)
```

失败时保留 trace、截图、console 和不含 query 的网络摘要。关键路径没有使用
坐标点击或纯视觉断言。

## WebView2 内容层

第一次尝试直接给正常 Wails 开发实例设置
`WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS`，真实页面能够加载，但 Wails 2.12.0
没有把动态 CDP 参数传给 WebView2 子进程。该失败保留为首次结果，不把普通
浏览器 smoke 当作 WebView2 内容证据。

随后新增纯测试构建入口 `scripts/testing-webview2-cdp-build.ps1`。它固定
Wails `v2.12.0`，只把依赖源码复制到
`output/webview2-cdp-build/<stamp>/wails` 后补丁 `setupChromium()`，并使用
测试专用变量 `KWU_TEST_WEBVIEW2_CDP_PORT` 和 `testdiag` tag 生成唯一 exe。
临时 `test.mod`、`test.sum`、`test.work`、依赖补丁和最终 exe 全部位于
`output/`；生产源码及依赖声明未变化。构建计划由
`scripts/testing-webview2-cdp-build-test.ps1` 校验路径、版本、tag 和变量。

最终成功命令与证据：

```text
powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File scripts/testing-webview2-cdp-build.ps1

powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File scripts/testing-webview2-cdp.ps1 `
  -Executable output/webview2-cdp-build/20260723-200035/kwu-desktop-cdp-test-20260723-200035.exe

schemaVersion: 2
sourceCommit: 15a98177251bd024b499206e9d90b979fbc48436
executableSHA256: b451a6d68067becb7b669c48378584c5d3feed6cd43703e200dd6a86d612a766
cdpAddress: dynamic 127.0.0.1 port
cdpBrowser: Edg/150.0.4078.83
cdpProtocolVersion: 1.3
webviewPIDs: 6
profileScope: isolated-system-temp
pageURL: http://wails.localhost/
pageTitle: 云智企办
language: zh-CN
hasWailsRuntime: true
```

报告不包含工作树绝对路径；运行前后检查没有遗留新的
`kwu-wv2-cdp-*` profile。脚本反向停止精确后代、根进程及匹配该 profile 的
孤儿进程，清理失败会使门禁失败。现有开发实例未被停止。

### 已关闭测试能力缺口 UI-TEST-001

- 严重度：Medium（测试能力缺口，不是已证实的产品功能缺陷）。
- 首次结果：正常 Wails 2.12.0 路径没有应用标准环境变量中的 CDP 参数。
- 关闭证据：固定依赖版本的 output-only 测试副本成功连接真实 WebView2，
  Playwright 验证 Wails runtime、页面标题、语言、URL 和进程树。
- 隔离：测试变量和依赖补丁不进入生产源码；生产守卫同时禁止测试变量、
  debug port 和 `connectOverCDP` 进入产品路径。

## Windows 原生层

PowerShell 通过精确 PID 和可执行文件路径调用 Win32 API，只读检查真实窗口：

```text
PID: 27268
class: kwuDesktopWindow
title: 云智企办
visible: true
```

随后新增 `scripts/testing-windows-uia-tree.ps1`，以该窗口句柄作为唯一根节点，
使用 Windows UI Automation `RawViewWalker` 只读遍历。默认报告不保存控件
名称，仅记录父子关系、depth、control type、class、automation ID、进程 ID
和可见/启用状态，避免页面文字或个人信息进入测试证据。

```text
nodeCount: 270
maximumDepth: 23
processIDs: 15028, 27268, 32868
skippedElements: 0
truncated: false
namesIncluded: false
```

control type 包含 Window、Pane、Document、Group、Text、Button、Tab、
StatusBar 等；主窗口根节点属于 PID 27268，WebView2 子树由两个子进程承载。
机器校验器同时验证节点索引、父子 depth、进程集合、统计总数、未截断和
名称脱敏：

```text
node scripts/testing-windows-uia-report.test.mjs
node scripts/testing-windows-uia-report.mjs `
  output/windows-uia-tree/report.json 27268 `
  .\desktop\build\bin\kwu-desktop-dev.exe

Windows UIA report tests: passed
Windows UIA report: passed (270 nodes, depth 23)
```

这证明当前开发实例的原生主窗口和 UIA tree 可由固定脚本盘点，T2.6 完成。
尚未自动化系统文件对话框、托盘或安装器，这些属于后续矩阵场景，不把本次
只读盘点扩大解释为它们已通过。

## 生产隔离守卫

以下检查均通过：

```text
node scripts/testing-webview2-production-guard.test.mjs
node scripts/testing-webview2-production-guard.mjs
pnpm --dir desktop/frontend build
```

守卫扫描生产 Desktop/Wails/frontend 路径，阻止
`WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS`、`KWU_TEST_WEBVIEW2_CDP_PORT`、
`--remote-debugging-port` 和 `connectOverCDP` 进入生产实现。生产 frontend
构建中也未发现浏览器 fixture 或 CDP 标记。

正常、未补丁的 Wails 生产模式 exe 构建成功：

```text
desktop/build/bin/kwu-desktop-phase2.exe
SHA256: cb5e24edbe144980cd9766e14a0abe3a634aa0dae64ab80a67b114fae7a1b56b
```

二进制扫描未发现 `--remote-debugging-port`、`KWU_TEST_WEBVIEW2_CDP_PORT` 或
`connectOverCDP`。Wails 依赖本身包含标准
`WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS` 字符串，因此不能以“完全无该字符串”
作为安全断言；真正的断言是产品源码未设置它，且产物没有 debug port 或测试
变量。

### 未关闭问题 UI-TEST-002

- 严重度：Medium（最终安装制品证据缺口）。
- 首次结果：Wails 生产模式 exe 构建成功，但 `-nsis` 输出
  `Warning: Cannot create installer: makensis not found`；命令仍返回 0。
- 环境复核：PATH、Program Files、Program Files (x86) 和用户 Programs 的
  常见 NSIS 路径均没有 `makensis.exe`。
- 影响：尚不能解包或安装最终 NSIS，并从运行时证明它未开放诊断端口。
- 处理：保持 T2.5 blocked；不自动安装系统级工具，不把 exe/source 证据冒充
  最终安装器证据。

## 分层边界

| 层 | 本轮状态 | 可证明 | 不能证明 |
|---|---|---|---|
| React | 通过 | 状态、组件行为、错误路径 | 浏览器布局、Wails binding |
| Browser | 通过 | 真实 DOM、重载、文件输入前置流程 | WebView2、原生窗口、安装器 |
| WebView2 | 通过（测试构建） | 动态 CDP、真实页面、Wails runtime、隔离 profile 与清理 | 最终生产安装器 |
| Native | 通过（当前盘点范围） | 精确进程对应的主窗口存在且可见；UIA tree 完整脱敏导出 | 系统对话框、托盘、安装器 |

# Reasonix 软件用户测试工具链

> 状态：Implemented locally
> 日期：2026-07-31
> 适用范围：`main-v3` 的 React/Vite、Wails/WebView2 与 Windows 桌面运行时

## 目标

从真实用户可观察行为出发，分层验证浏览器内容、运行态集成、无障碍和
Windows 桌面外壳。任何一层都不能替代另一层；AI 探索只能发现和复现问题，
发布结论必须来自固定命令、明确断言和可审计证据。

## 工具与边界

| 层级 | 工具 | 负责证明 | 不能证明 |
|---|---|---|---|
| 浏览器 fixture | Playwright Test | 导航、输入、错误状态、恢复、文件选择前置流程 | Wails binding 和原生窗口 |
| 无障碍 | `@axe-core/playwright` | 自动检测 WCAG A/AA 问题并保存完整报告 | 键盘、读屏和认知可用性的全部问题 |
| 视觉回归 | Playwright screenshot | 稳定 fixture 的布局和样式差异 | 动态真实账号页面的正确性 |
| 真实运行态 | Playwright Test | 真实 Vite/Wails 代理下的页面、控制台和核心入口 | 安装器、托盘和系统对话框 |
| Windows 桌面 | pywinauto/UIA | 目标可执行文件、窗口标题、可见性、可用性和尺寸 | WebView 内复杂 DOM 行为 |

FlaUI 需要额外的 .NET SDK。本机没有该 SDK，且当前 UIA 验收只需要窗口级
能力，因此采用项目隔离的 pywinauto。若后续需要稳定操作多个系统对话框，
再以独立 PoC 重新比较 FlaUI，不把 .NET SDK 作为当前工具链前置条件。

## 首次安装

从目标 worktree 根目录运行：

```powershell
.\scripts\user-test.ps1 -Suite install
```

该命令：

1. 按 `pnpm-lock.yaml` 安装前端依赖；
2. 安装与项目 Playwright 版本匹配的 Chromium；
3. 在 `output/user-testing/.venv` 创建隔离 Python 环境；
4. 按 `tests/user-testing/windows/requirements.lock.txt` 安装 pywinauto。

不会创建或修改 `.env`、`wails.json`、用户云地址或生产配置。

## 执行入口

```powershell
# 静态 fixture：功能、axe、视觉回归
.\scripts\user-test.ps1 -Suite browser

# 已启动的真实 Reasonix：核心入口和控制台
.\scripts\user-test.ps1 -Suite runtime -BaseUrl http://127.0.0.1:5292

# 已启动的真实 Reasonix：WCAG 严重/致命问题
.\scripts\user-test.ps1 -Suite audit -BaseUrl http://127.0.0.1:5292

# 当前 worktree 的 Windows 桌面窗口
.\scripts\user-test.ps1 -Suite desktop

# 在专用测试实例中可额外验证最小化与恢复
.\scripts\user-test.ps1 -Suite desktop -ExerciseWindow

# 依次运行全部层；某层失败时继续收集其他层结果，最后统一失败
.\scripts\user-test.ps1 -Suite all -BaseUrl http://127.0.0.1:5292
```

真实运行态测试要求先按仓库原有 `./dev` 启动独立端口和独立
`KWU_DEV_PROFILE_ROOT`。测试脚本只读取 `BaseUrl` 和目标可执行文件，不管理
或覆盖运行配置。

## 证据

| 证据 | 路径 |
|---|---|
| fixture Playwright 结果、trace、截图 | `output/playwright/browser-smoke/` |
| 真实运行态报告、trace、视频、axe JSON | `output/playwright/runtime/` |
| Windows UI 隔离环境 | `output/user-testing/.venv/` |

`output/` 已被 Git 忽略。失败报告可能包含界面文本和 URL；分享前必须检查并
脱敏，不得把凭据、真实消息、手机号或支付信息提交到仓库。

## 当前已知审计结果

2026-07-31 的真实运行态功能 smoke 通过。首次 axe 审计发现
`color-contrast` 严重问题，例如侧栏 `#7a7a7a` 对 `#f3f4f4` 的对比度为
3.89:1，低于普通文本要求的 4.5:1；随后已通过提高次要文字前景色对比度修复，
没有关闭规则、降低等级或忽略选择器。修复后真实运行态审计以及六种主题的
明暗模式审计均无 serious 或 critical 违规。

## 建议扩展顺序

1. 保持现有 fixture、真实运行态和 Windows 外壳三层稳定。
2. 为首次对话建立独立 profile 的可重复闭环：新建会话、选模型、发送、停止、
   重试、重启恢复。
3. 增加键盘-only、125%/150% 缩放、高对比度和 NVDA 人工矩阵。
4. 增加文件对话框、单实例激活、安装、升级和卸载的 Windows 专用验收。
5. 只有截图规模需要集中审核时再引入 reg-suit；当前使用 Playwright 原生快照。

Playwright MCP 适合让 Codex 做探索性测试和定位未知路径，但发现应转成
Playwright Test 或 pywinauto 的确定性脚本后，才能作为长期回归证据。

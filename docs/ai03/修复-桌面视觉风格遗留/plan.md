# 修复：清理桌面视觉风格遗留配置

## 背景与问题

桌面端已经不再向用户提供“视觉风格”选择，但历史值 `desktop.theme_style` 仍会保存在用户配置中。开发隔离配置的 `theme = "auto"` 与 `theme_style = "graphite"` 在系统深色偏好下触发石墨主题遗留的金色强调色，导致登录、注册和找回密码页与当前默认设计不一致。

## 当前行为与证据

- 基线：`origin/main-v3` / `6c5282e07b46f01b7bba6e218b2dc883acc6daeb`。
- `internal/config/edit.go` 的 `SetDesktopAppearance(theme, style)` 校验并写入 `Desktop.ThemeStyle`。
- `desktop/settings_app.go` 通过 `applyConfigOnly` 将该字段保存到当前用户或开发隔离 profile 的 `config.toml`。
- `desktop/frontend/src/App.tsx` 在启动时会把旧 WebView `localStorage` 的 `kwu-theme-style` 迁移到桌面配置；已有配置优先保留。
- `desktop/frontend/src/lib/theme.ts` 为无效/缺失风格回退到 `carbon`，但显式历史值继续作为 `data-theme-style` 应用。
- `desktop/frontend/src/styles/tokens.css` 对 `graphite` 的深色路径定义金色 `#c99a3a`，因此可稳定复现问题。

代码图谱服务在当前会话不可用；以上结论已在独立工作树中以直接源码读取、现有测试与运行配置复核。

## 目标

1. 废弃桌面 UI 的可配置 `theme_style`。
2. 每个桌面 profile 首次经启动设置读取时，忽略并从配置文件清理历史 `desktop.theme_style`。
3. 保留 `SetDesktopAppearance(theme, style)` 形参以兼容旧前端或自动化调用，但忽略 `style` 且不再写入。
4. 所有桌面环境一致生效：正式版、开发隔离环境及 profile 变体。
5. 提供“未修改历史配置”的对照组与“迁移后配置”的验收证据。

## 非目标与边界

- 不删除或修改 CLI 的 `ui.theme_style`；它是独立的终端调色板。
- 不移除 CSS 中旧主题调色板或 Wails 绑定的 `style` 参数，本次只切断桌面配置的读写路径。
- 不批量扫描或修改未被启动的其它用户目录；迁移在对应 profile 实际启动时完成。
- 不改变 `theme`（auto/light/dark）、布局、语言、账号、会话、模型或凭据。

## 方案

1. 在桌面启动设置的服务端读取路径中识别非空 `Desktop.ThemeStyle`，使用已有配置编辑/原子写入机制将其清空，随后返回空值。
2. 修改 `SetDesktopAppearance`：继续校验并保存 `theme`，接受遗留 `style` 输入但始终清空 `Desktop.ThemeStyle`；不因旧风格值报错。
3. 调整旧 WebView 偏好迁移：只迁移主题模式，忽略其视觉风格值，清理 localStorage 的旧键。
4. 使前端启动和设置页只以主题模式调用/应用默认 `carbon`，不重新持久化风格。
5. 更新受影响的 Go 和前端测试、必要的配置渲染与指标断言，明确 `DesktopThemeStyle` 的兼容返回语义。

## 受影响契约与文件（预期）

- `internal/config/edit.go`、`internal/config/config.go`、相关单元测试：桌面配置的兼容/规范化语义。
- `desktop/settings_app.go`、`desktop/app.go`、相关 app 测试：启动时原子清理与绑定兼容。
- `desktop/frontend/src/lib/theme.ts`、`App.tsx`、设置相关测试：旧 localStorage 迁移不再注入风格。
- 仅当测试证明必要时调整 `desktop/frontend/src/styles/tokens.css` 或指标断言；不以删 CSS 作为迁移前提。

## 兼容性、迁移与回滚

- 兼容：旧调用仍可传 `style`，返回成功；随后读取为默认/空风格。
- 迁移：仅在该 profile 下次桌面启动或刷新启动设置时写一次；无历史值时不写配置。
- 回滚：还原本次实现提交即可停止未来清理；已清除字段只能通过用户配置备份或重新选择历史风格恢复，符合“产品已废弃该功能”的决定。

## 验收与验证

### 自动化对照组

1. 建立隔离的桌面 profile，写入 `theme = "auto"` 和 `theme_style = "graphite"`。
2. 在迁移调用前断言对照组配置仍包含 `theme_style = "graphite"`，并由现有 token 规则确认其可选择金色路径。
3. 执行启动设置读取/迁移。
4. 断言迁移组：返回的 `DesktopThemeStyle` 为空或默认 `carbon` 语义；持久化 TOML 不再有 `desktop.theme_style`；`theme` 等无关字段完全保留。
5. 重复读取，断言不再写文件且结果稳定。
6. 覆盖旧 localStorage 含 `kwu-theme-style=graphite` 的首次启动：主题模式可迁移，风格不写入，旧键被清除。
7. 覆盖正式与开发 profile resolver，不使用真实用户目录。

### 应用验收

1. 准备一个未修改的开发隔离 profile（控制组），保留 `theme_style = "graphite"`，记录登录页金色按钮。
2. 用候选版本启动同一 profile（实验组）。
3. 验证登录、注册、找回密码页面均使用默认 `carbon` 强调色，且配置中历史字段已消失。
4. 重启应用，确认颜色不会回退，主题模式和其它桌面偏好保持不变。

### 检查计划

- 实现期间：相关 Go 包的 `go test` 与必要的前端精确测试。
- 应用稳定后：受影响桌面 Go 包测试、`pnpm build`（在 `desktop/frontend`），以及真实 Wails 开发窗口验收。
- PR 前：同步最新 `origin/main-v3` 后运行项目规定的 PR Gate；仅在获得 PR 授权后执行。

## 风险

- 启动时写配置可能放大每次启动的 I/O；必须只在检测到非空遗留字段时写入。
- profile 路径或 WebView 迁移遗漏会造成部分环境仍复现；测试必须隔离并覆盖两条入口。
- 旧设置接口有其它调用方；保留参数并用测试锁定“接受但不持久化”的行为。

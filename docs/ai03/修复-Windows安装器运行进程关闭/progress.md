# Windows 安装器运行进程占用修复进展

## 当前状态

| 项目 | 状态 |
| --- | --- |
| 当前阶段 | Stage 3 完成；ARM64 验收通过，准备同步并创建 Draft PR |
| 分支 | `codex/fix-windows-installer-running-app` |
| worktree | `D:\Reasonix-worktrees\fix-windows-installer-running-app` |
| 基线 | `origin/main-v3@d1b4fa902782062b962b2438abc59144f428ffd1` |
| 计划 | [`plan.md`](./plan.md) |
| 任务 | [`tasks/001-实现安装前运行占用门禁.md`](./tasks/001-实现安装前运行占用门禁.md) |
| PR | 用户已授权推送并创建 Draft PR；待最终同步与 PR Gate |

## 当前决策

- AMD64 与 ARM64 共用安装器占用门禁。
- 手动安装检测到运行应用时，不自动关闭、不强杀、不等待 60 秒。
- 立即提示从应用菜单或系统托盘完全退出，提供“重试 / 取消”。
- 静默安装遇到占用或检测失败时 fail closed。
- 应用内更新保留主动 shutdown、`/UPDATEPID` 等待和单实例重启。
- 删除第一版 Windows WndProc 退出桥接。
- 只检测最终安装目录和已认证品牌迁移目录的可执行文件。

## 证据快照

| 项目 | 结论 |
| --- | --- |
| 初始根因 | 普通安装在 `wails.files` 前没有运行进程门禁 |
| 第一版实现 | Restart Manager 非强制 shutdown + 新应用 Windows 消息钩子 |
| x64 第一版验证 | 新构建程序可被正常关闭，但未覆盖正式 v1.1.0 |
| ARM64 第一版验证 | v1.1.0 窗口隐藏、进程驻留，60 秒后安装失败 |
| 失败根因 | 新包内的钩子在覆盖前不可供 v1.1.0 使用；普通 `WM_CLOSE` 触发后台驻留 |
| v1.1.0 可用退出入口 | 仅应用菜单/托盘内部入口，安装器不能安全调用 |
| 修正方向 | Restart Manager 只检测，用户完全退出后重试 |
| 本机能力 | 当前工作机 x64；ARM64 最终验收由用户执行 |

## 验证状态

- 第一版自动化、双架构编译和 x64 新程序探针已通过，但应用行为方案已被真机证据否定。
- 修正后的 installer contract 27/27、`go test .`、`pnpm build` 通过。
- AMD64/ARM64 正式构建入口通过，应用和 NSIS payload 架构、VersionInfo 均正确。
- x64 隔离目录运行目标应用后执行普通 `/S`：installer 退出码 1，应用仍运行，目标
  SHA256 未变化，未创建 `uninstall.exe`。
- 第一版产物
  `dist/Clouvia-windows-arm64-installer.exe`
  SHA256 `9E5637EBDCCAC519B88735BDD703E2AAFFFBB076715BB7694D2F012721136843`
  已判定不通过，不得继续验收或发布。
- ARM64 第一轮确认提示和重试按钮符合预期，但提示框点取消只 `Abort` 当前 Section，
  安装器主窗口仍需再点一次 Cancel。现已改为设置非零退出码后直接 `Quit`，合同固定
  一次取消退出。
- 新 ARM64 复验产物：
  `dist/Clouvia-windows-arm64-installer.exe`，27,432,858 bytes，SHA256
  `C64800B509D488F3BFF5BC38FEF166EE510F3242959815F63F8692625BCF95C1`。
- 2026-07-31 用户完成该精确产物真机复验并授权提交 PR；一次取消退出安装器、完全退出
  应用后重试安装均通过。
- Wails `kwu-` 原始中间安装包已移出 worktree，交付只使用 `Clouvia-*` 正式命名。

## 下一门禁

1. 同步最新 `origin/main-v3`，复测关键路径并运行最终 PR Gate。
2. 推送当前分支并创建 Draft PR，不自动标记 Ready 或合并。

# Windows 卸载终端闪现修复进展

## 当前状态

| 项目 | 状态 |
| --- | --- |
| 当前阶段 | Stage 3：实现、主会话复核和应用验收完成，等待 PR 授权与最终门禁 |
| 计划文档 | [`plan.md`](./plan.md) |
| 任务拆分 | [`tasks/001-隐藏执行卸载进程终止命令.md`](./tasks/001-隐藏执行卸载进程终止命令.md)；Serial-01；实现完成 |
| 运行代码 | `project.nsi` 改用 `nsExec::Exec` 隐藏同步执行 `taskkill.exe`，合同测试已加固 |
| PR | 未创建；最终小型修复 PR 需要明确授权 |

## 已确认决定

- 唯一外部行为目标是卸载时不出现任何终端或命令行窗口闪现。
- 保留 `taskkill.exe /IM ... /T /F`、同步等待、一秒等待和删除失败处理。
- `taskkill` 非零退出码不单独判定失败，最终仍以主程序能否删除为准。
- 不引入优雅退出、超时、UI、文案、日志或其他卸载行为变化。

## 证据与验证快照

| 验证面 | 结论 |
| --- | --- |
| 计划基线 | `origin/main-v3@d1b4fa902782062b962b2438abc59144f428ffd1` |
| 根因 | `kwu.deleteInstalledExecutable` 通过 `ExecWait` 启动控制台程序 `taskkill.exe` |
| 既有合同 | 覆盖进程名、`/T /F`、删除失败中止；未覆盖无窗口执行 |
| 引入历史 | `15da69783a1b0d888938c85eca87196225552333` |
| 可用机制 | 本机 NSIS 3.12 含 Unicode `nsExec.dll`；官方文档确认无 DOS 窗口执行 |
| 聚焦合同 | `pnpm exec tsx src/__tests__/installer-contract.test.ts`：20 passed，0 failed |
| 真实构建 | Wails v2.12.0 + NSIS 3.12 成功生成 Windows amd64 installer |
| 隔离应用验收 | 目标存在时 20 次隐藏 `taskkill` 均返回 0 并结束进程；目标不存在时返回 128 且流程继续；Computer Use/Windows Graphics Capture 未观察到终端窗口 |
| 环境保护 | 机器已有正式 1.0.5；未覆盖或卸载，改用复用同一命令路径的隔离 NSIS 夹具 |
| EOL/diff | 相关 NSIS、TypeScript 文件均为 `i/lf w/lf`；`git diff --check` 通过 |

## 范围与下一门禁

- 实际运行代码范围仅为 `project.nsi` 和邻近 installer contract test。
- 当前没有阻塞项。
- 用户已明确授权计划、任务和代码一次性完成并按功能小步提交。
- 下一门禁：取得 PR 授权后同步最新 `main-v3`、复跑关键路径并执行最终 PR Gate。

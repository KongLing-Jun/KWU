# Windows 卸载终端闪现修复计划

## 问题与现状

Windows 卸载器在删除主程序前调用系统自带的 `taskkill.exe` 结束仍在运行的桌面进程及
其进程树。当前通过 NSIS `ExecWait` 直接启动这个控制台程序；交互式卸载时可能短暂
创建并显示命令行窗口，造成终端闪现。

用户确认本次唯一外部行为目标是消除该闪现。现有进程终止、等待、删除和失败处理语义
必须保持不变。

## 根因与证据

- 工作树基于 `origin/main-v3@d1b4fa902782062b962b2438abc59144f428ffd1`。
- `desktop/build/windows/installer/project.nsi` 的
  `kwu.deleteInstalledExecutable` 宏使用：
  `ExecWait '"$SYSDIR\taskkill.exe" /IM "${PRODUCT_EXECUTABLE}" /T /F' $0`。
  `ExecWait` 等待子进程结束，但不会为控制台程序提供无窗口执行保证。
- 该路径由提交 `15da69783a1b0d888938c85eca87196225552333`
  “修复 Windows 卸载残留主程序”引入，用来解决运行中主程序阻止卸载的问题。
- `desktop/frontend/src/__tests__/installer-contract.test.ts` 已固定
  `taskkill.exe`、`/IM`、`/T /F` 以及删除失败时中止卸载的合同，但没有断言调用方式
  不创建控制台窗口。
- 主工作区 Codegraph 查询未解析出 NSIS 宏及其测试关系，因此以上结论均在独立
  worktree 中由实际源码、Git 历史和差异直接确认。
- 本机发布工具为 NSIS 3.12，Unicode `nsExec.dll` 已随 NSIS 安装。
  [NSIS 的 nsExec 文档](https://nsis.sourceforge.io/NsExec_plug-in)明确说明该插件可
  执行命令行程序并捕获输出，而不打开 DOS 窗口。

## 已确认边界

1. 卸载过程中不得出现由结束残留进程引发的终端或命令行窗口闪现。
2. 继续直接调用 Windows 自带的 `$SYSDIR\taskkill.exe`。
3. 保留 `/IM "${PRODUCT_EXECUTABLE}" /T /F`，继续强制结束目标进程树。
4. 保留同步等待、现有一秒等待、文件删除重试判定和最终失败提示。
5. `taskkill` 非零退出码不单独导致卸载失败；仍以主程序能否删除为最终结果。

## 目标

- 交互式和静默卸载均在后台执行 `taskkill.exe`，不创建可见控制台窗口。
- 正在运行的桌面应用及其进程树仍会在删除主程序前被同步终止。
- 主程序无法删除时，卸载器仍中止并提示用户关闭应用后重试。
- 自动化合同能防止以后退回会闪现窗口的 `ExecWait taskkill.exe` 路径。

## 非目标

- 不引入优雅退出、额外超时、用户确认或新的进程终止策略。
- 不替换 `taskkill.exe`，不修改目标进程名、安装目录或卸载顺序。
- 不改变卸载器 UI、文案、日志、签名、更新器或注册表行为。
- 不修改 Windows 之外的平台。

## 实现方案

在 `kwu.deleteInstalledExecutable` 中将 `ExecWait` 替换为 NSIS 自带的
`nsExec::Exec`，仍直接传入相同的 `taskkill.exe` 路径和参数，并 `Pop $0` 取回退出
结果。`nsExec::Exec` 同步完成命令且不打开控制台窗口；后续 `Sleep`、`Delete`、
`IfErrors`、`IfFileExists` 和 `Abort` 控制流保持原样。

更新安装器合同测试，同时证明：

- 隐藏执行器调用了原有 `taskkill.exe /IM ... /T /F` 命令；
- 结果从 NSIS 栈弹出，避免污染后续宏执行；
- 同一命令不再通过 `ExecWait` 启动；
- 文件删除失败仍中止卸载。

## 影响范围与兼容性

- `desktop/build/windows/installer/project.nsi`：仅调整残留进程结束命令的执行器。
- `desktop/frontend/src/__tests__/installer-contract.test.ts`：加强 Windows 卸载合同。
- 后续任务回执与进展文档只记录执行合同和验证结论。

`nsExec` 是 NSIS 随附插件，本仓库现有 Unicode 安装器可直接使用，不新增第三方依赖。
应用 API、数据、注册表、安装路径、更新协议和产物命名均不变，不需要迁移。

## 验收场景

1. 启动已安装的桌面应用后从 Windows 卸载入口交互式卸载：应用及进程树被结束，
   卸载成功，全程没有终端或命令行窗口闪现。
2. 应用未运行时交互式卸载：卸载成功且没有终端闪现。
3. 使用 `/S` 静默卸载：没有额外可见窗口，主程序被删除。
4. 主程序仍被其他机制占用、删除最终失败：保留既有中止和错误提示，不报告假成功。

## 验证计划

实现迭代先在 `desktop/frontend` 运行聚焦合同：

```powershell
pnpm exec tsx src/__tests__/installer-contract.test.ts
```

随后使用原生 Windows amd64 构建路径生成 NSIS installer，证明脚本可由实际 NSIS
编译。应用验收在可恢复的测试安装目录中完成“安装 → 启动 → 交互式卸载”关键路径，
由 Computer Use 或屏幕录制观察卸载全过程；终端窗口一旦出现即判失败。另执行一次
应用未运行的卸载场景。无法可靠制造文件删除失败时，以现有合同测试证明该控制流未变。

最终实现 PR 同步最新 `main-v3` 后，按仓库规则运行代码守卫、EOL/diff 检查及：

```powershell
node ./scripts/pr-gate.mjs --base origin/main-v3 --run all
```

安装器改动不涉及 Go 文件，因此不运行 Go 测试；PR Gate 应路由到 Windows installer
合同和所需前端生产构建，实际分类结果以门禁摘要为准。

## 风险、回滚与未决问题

- 风险：遗漏 `Pop $0` 会污染 NSIS 栈；通过合同测试和实际编译共同约束。
- 风险：只验证脚本文本无法证明视觉结果；必须完成真实卸载观察后才算应用验收通过。
- 回滚可恢复原执行器及对应合同断言，不涉及用户数据或迁移。
- 当前没有未决产品、兼容性或技术问题。

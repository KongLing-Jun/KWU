# Windows 安装器运行进程占用修复计划

## 背景与问题

用户在 Windows ARM64 上运行云智企办时直接执行安装包，安装器需要覆盖仍由进程占用的
`clouvia-desktop.exe`。原安装路径除应用内更新的 `/UPDATEPID` 外，没有在 payload
写入前检测运行进程，可能覆盖失败或让用户误以为安装完成。

AMD64 与 ARM64 共用 `desktop/build/windows/installer/project.nsi`，因此这是共享
Windows 安装合同。任务基线为
`origin/main-v3@d1b4fa902782062b962b2438abc59144f428ffd1`，没有对应 GitHub issue。

## 当前行为与实机失败证据

- `.onInit` 仅在应用内更新传入 `/UPDATEPID` 时等待该 PID；手动启动安装包不走此路径。
- 普通安装 Section 原本直接进入 WebView2 和 `wails.files`，没有运行进程门禁。
- 云智企办默认允许“关闭后驻留后台”；普通 `WM_CLOSE` 可能只隐藏窗口。
- 第一版实现使用 Restart Manager 非强制 `RmShutdown`，并在新程序中增加 Windows
  `ENDSESSION_CLOSEAPP` 钩子。x64 上用新程序作为被关闭对象时通过。
- ARM64 真机使用当前正式版 v1.1.0 运行后再打开测试安装包：窗口消失，但进程继续驻留，
  60 秒后安装器报告无法正常关闭。v1.1.0 没有新钩子，测试证明新安装包内的应用代码
  无法帮助尚未被替换的运行程序完成首次升级。

Codegraph 在主工作区索引基线 `10c4581d44f787fd87ee9d612faaf02dce022750`
确认应用内更新调用链为 `InstallUpdate` → `applyWindowsFile` →
`installerCommand`。安装器启动后，现有应用会执行 `shutdown` 并 `os.Exit(0)`；
`/UPDATEPID` 负责等待该进程。因此应用内更新不需要安装器代为关闭。

最终判断以 worktree 源码、`desktop-v1.1.0` Tag、实际构建和真机反馈为准。

## 已确认产品规则

1. 手动交互安装检测到目标应用运行时，不自动发送关闭消息，不强制终止。
2. 立即阻止安装并提示用户从应用菜单或系统托盘选择“退出”。
3. 对话框提供“重试 / 取消”；重试重新检测，完全退出后才允许写入 payload。
4. 静默安装检测到占用或无法完成检测时 fail closed，返回非零且不显示对话框。
5. 应用内更新继续由运行应用保存状态并主动退出，保留 `/UPDATEPID` 等待和自动重启。
6. 只检测本次安装目标：
   - `$INSTDIR\clouvia-desktop.exe`；
   - 品牌迁移时由受信任卸载注册信息确认的旧目录中 `kwu-desktop.exe`。
7. 不影响其他目录中的同名便携版、开发版或其他用户会话。
8. AMD64 与 ARM64 使用同一实现。
9. ARM64 真机必须验证精确产物，通过前状态为等待应用验收。

## 目标

- 目标程序仍运行时，安装器绝不进入 WebView2、payload、快捷方式、注册表或卸载器写入。
- 用户看到的行为与真实进程状态一致，不再出现“窗口已经消失但仍等待 60 秒”。
- 不依赖正在运行的版本预先实现某个 Windows 消息钩子。
- 不强杀进程，不引入会话状态丢失风险。
- 保持应用内更新和安装后启动语义。

## 非目标

- 不改变卸载器现有的进程结束行为。
- 不改变桌面普通关闭到后台设置。
- 不新增外部 IPC、进程注入、托盘菜单自动化或跨用户关闭。
- 不改变更新下载、签名、安装授权、发布资产名称、用户数据和 API。
- 不替换 NSIS，也不修改已发布的 v1.1.0。

## 选定方案

### 1. 精确文件占用检测

继续使用 Windows Restart Manager，但只使用 `RmStartSession`、`RmRegisterResources`、
`RmGetList` 和 `RmEndSession`：

- 注册最终 `$INSTDIR` 下的正式可执行文件；
- 品牌迁移仅注册已通过现有卸载键、目录和 uninstaller 认证的旧文件；
- `RmGetList` 返回占用时阻止继续；
- API 初始化、注册或查询失败同样阻止继续；
- 每次重试建立新 session，所有分支结束 session。

安装路径禁止调用 `RmShutdown`、`RmForceShutdown` 或 `taskkill`。

### 2. 手动退出交互

在任何 payload 副作用之前：

- 无占用：直接继续。
- 交互安装有占用：立即显示“请从应用菜单或系统托盘退出”，选择重试后重新检测，
  选择取消则终止安装。
- 交互检测失败：提示无法确认状态，可重试或取消。
- 静默安装有占用或检测失败：设置非零退出码并终止，不弹窗。

不再自动关闭、不再隐藏运行窗口，也不再设置 60 秒关闭等待。

### 3. 删除无效的 Windows 应用钩子

删除第一版新增的 Windows WndProc 桥接及测试，恢复 `system_quit_stub.go` 在 Windows
上的空实现，并移除只为该桥接增加的原子清理函数。Darwin 系统退出支持保持不变。

这是必要的范围收缩：安装器不再请求 Restart Manager shutdown，Windows 程序无需识别
`ENDSESSION_CLOSEAPP`；保留该钩子只会增加窗口过程和 ARM64 ABI 风险。

### 4. 保留应用内更新

`InstallUpdate` 仍先启动带 `/S /UPDATEPID=<pid>` 的安装器，再由应用执行既有
`shutdown` 和 `os.Exit(0)`。安装器 `.onInit` 等待该精确 PID 后，还会执行统一文件占用
检测以防竞态。安装完成后仍只重启一个新进程。

## 影响范围

- `desktop/build/windows/installer/project.nsi`：保留精确检测，替换为手动退出门禁。
- `desktop/build/windows/installer/strings_zh.nsh`：手动退出与检测失败文案。
- `desktop/frontend/src/__tests__/installer-contract.test.ts`：禁止自动/强制关闭、重试、
  静默 fail-closed 和 payload 前置合同。
- `desktop/system_quit.go`、`system_quit_stub.go`：恢复基线。
- 删除 `desktop/system_quit_windows.go` 及测试。
- 本计划、进展和任务回执。

保持不变：updater 参数、安装目录、卸载身份、payload 名、签名和发布合同。

## 验收场景

1. v1.1.0 前台运行时打开安装包：立即显示手动退出提示，应用窗口不被安装器隐藏。
2. v1.1.0 已隐藏到托盘：提示用户从托盘退出；退出后点击重试，安装成功。
3. 用户只关闭窗口导致继续驻留：重试仍提示，不写文件。
4. 用户从菜单/托盘彻底退出：重试通过并进入安装。
5. 目标未运行：无提示，正常安装。
6. 用户取消：旧安装、快捷方式、注册表和用户数据不变。
7. 普通 `/S` 遇到占用：非零退出，不强杀、不修改 payload。
8. `/S /UPDATEPID` 应用内更新：旧进程主动退出，等待完成后安装并只启动一个新进程。
9. 其他目录同名实例：不影响安装目标检测，也不被关闭。
10. Restart Manager 错误：交互可重试/取消，静默失败，不进入 payload。

## 验证计划

实现阶段：

```powershell
# desktop/frontend
pnpm exec tsx src/__tests__/installer-contract.test.ts
pnpm build

# desktop
go test . -count=1
```

随后编译 AMD64/ARM64 Wails 与 NSIS，运行 Windows payload 架构和 VersionInfo 合同，
检查 EOL、BOM、`git diff --check` 和实际 diff。

x64 本机验证至少覆盖占用检测、取消、完全退出后重试、普通 `/S` fail-closed 和
`/UPDATEPID`。ARM64 用户用新生成的精确 SHA256 产物验证 v1.1.0 前台/托盘场景。

创建或更新 PR 前仍需明确授权、同步最新 `origin/main-v3`、复测关键路径并运行：

```powershell
node ./scripts/pr-gate.mjs --base origin/main-v3 --run all
```

## 风险与回滚

- Restart Manager 只用于文件占用识别，不参与关闭，因此兼容性风险低于第一版。
- 用户可能只点窗口关闭而没有真正退出；持续占用检测与明确托盘文案可避免误写文件。
- 跨用户或异常资源查询会 fail closed，可能要求用户排查，但不会破坏既有安装。
- 没有数据、配置或 API 迁移。回滚安装器门禁会恢复原先直接写 payload 的风险，因此
  不建议单独回滚检测合同。

## 未决项

没有未决产品决策。ARM64 精确产物真机结果仍是最终应用验收门禁。

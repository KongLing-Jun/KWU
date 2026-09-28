# Windows 代码签名规划证据

## 调查基线

- Codegraph 索引主工作区：`D:\Reasonix`
- Codegraph 调查基线：`main-v3@12428b7bf272e5bbe3e0244e978a6ce17242e164`
- Stage 1 worktree 基线：`origin/main-v3@db19153a70998b2a54f25ad56101ea9c98d96ab1`
- 两个基线之间相关发布/NSIS 文件无差异；worktree 已直接复核当前源码。

## 源码与运行证据

- `.github/workflows/desktop-build-draft.yml:168-220,242-286`：两个 Windows 架构
  当前各在 `windows-2025` Job 构建；其 `347-349` 与 Publish `312-313` 明确声明
  Authenticode 尚未启用。
- `.github/workflows/desktop-build-draft.yml:378-439` 当前等待三个平台 Job 后才创建
  Draft，而 `scripts/desktop-release.ps1:788-792` 在本地构建前要求 Draft 已存在；
  因此必须把 Draft 创建提前，再由 macOS Runner 和本机分别上传。
- `.github/workflows/desktop-publish.yml:137-186` 已有下载并验证 Draft 资产的阶段，
  可直接承担三份主产物汇总，无需新增独立 Aggregate Workflow。
- `scripts/desktop-build.sh:63-75,160-186`：`wails build -nsis` 后查找应用和
  installer，再生成 portable ZIP。
- `desktop/build/windows/installer/project.nsi:35,92,103,269,282-288`：NSIS 嵌入
  `clouvia-desktop.exe`，写出 installer 并生成 `$INSTDIR\uninstall.exe`；
  `87-89` 的 `!uninstfinalize`/`!finalize` 示例仍被注释。
- Wails v2.12.0 `build.go:249,263-270`、`nsis_installer.go:90-109`：应用先编译，
  再把架构二进制路径传给 `makensis`。
- `scripts/assert-windows-release-architecture.ps1:161-178` 只验证解包主程序的架构
  和版本；实测 7-Zip 不暴露卸载器，所以必须在 `!uninstfinalize` 当场验证。
- 实测当前 amd64 主程序 PE machine 为 `0x8664`；installer 和已安装
  `uninstall.exe` 为 `0x014C`。因此只要求内嵌主程序匹配 amd64/arm64，
  NSIS 外壳可保持 x86，但三份 EXE 都必须签名。
- `scripts/desktop-release.ps1:778-855` 已按冻结来源串行构建两个 Windows 架构、
  验证后上传 Draft 并记录 SHA256；`673-699` 对同名资产只允许复用相同哈希。
- `scripts/desktop-release.ps1:1072-1284` 已从 Draft 下载三平台主产物、统一 minisign、
  生成 manifest，并按不可变 OSS、GitHub、stable pointer、云记录的顺序发布。
- `desktop/updater.go:250-267` 只在云记录与签名 manifest 同版本时使用云下载信息；
  云记录不能选择或授权自动更新资产。
- `desktop/updater_app.go:123-143` 的 `InstallUpdate` 只重验缓存文件，安装前不重新获取
  stable manifest；仅移动指针无法阻止已经下载的坏包，需增加安装前版本/哈希复核。
- `scripts/desktop-release.ps1:958-977` 的正常 stable pointer 合同拒绝向旧版本移动，
  因而紧急恢复必须是独立 Withdraw 动作，不能放宽正常 Publish。

## SimplySign 实签探针

2026-07-28 在固定 Windows 签名机对现有主程序副本执行了只影响副本的真实签名：

- 使用已登录 SimplySign 会话及本机 SignTool，显式选择本机配置的证书指纹。
- 参数使用 SHA-256 文件摘要、`http://time.certum.pl/` RFC 3161 时间戳和 SHA-256
  时间戳摘要。
- Windows Authenticode 验证结果为 `Valid`，签名主体为
  `广州鑫恒泰互联科技有限公司`。
- 时间戳链为 `Certum Timestamp 2026`，有效期至 2036-02-27。
- 原始主程序保持 `NotSigned`，探针副本未上传、未发布。

结论：证书、SimplySign CSP、SignTool 和时间戳服务的基本链路已经证实可用；
仍需在实施阶段通过 NSIS 三钩子连续签名 dry-run，证明主程序、临时卸载器和最终
installer 都能失败关闭地完成签名与验证。

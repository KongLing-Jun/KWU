# Windows 代码签名与发布流程开发计划

## 1. 背景与当前行为

Windows 安装包目前没有 Authenticode。Windows 11 Smart App Control 会把已安装的
`clouvia-desktop.exe` 识别为无法确认发布者的应用并阻止启动。本机对现有 amd64
安装包、已安装主程序和 `uninstall.exe` 的只读检查均返回 `NotSigned`。

当前稳定发布包含三个主产物：

- `Clouvia-windows-amd64-installer.exe`
- `Clouvia-windows-arm64-installer.exe`
- `Clouvia-darwin-universal.dmg`

Windows amd64、arm64 目前由两个 GitHub 托管 `windows-2025` Job 分别执行
`scripts/desktop-build.sh`，以 `wails build -nsis` 完成应用编译和 NSIS 打包。
本地后备入口 `desktop-release.ps1 BuildWindows` 已能从冻结 Tag 串行构建两个架构，
将安装包按不可覆盖规则上传 Draft；Publish 再统一处理 minisign、manifest 和各发布面。

公司现有 Certum 标准代码签名证书通过 SimplySign Desktop 暴露给当前 Windows 用户
会话；它需要人工登录，不能在 GitHub 托管 Runner 使用，也不是 Apple Developer ID。

## 2. 期望行为

每个 Windows 架构必须在最终打包前后覆盖三份 PE：

1. 编译后的 `clouvia-desktop.exe` 在被 NSIS 嵌入前签名并验证。
2. NSIS 生成临时 `uninstall.exe` 后，通过 `!uninstfinalize` 立即签名并验证。
3. NSIS 生成最终 installer 后，通过 `!finalize` 签名并验证。

amd64、arm64 合计六次签名，使用 SHA-256 文件摘要和 Certum RFC 3161 SHA-256
时间戳。任一签名、时间戳或验证失败，Windows 候选失败，三平台稳定版本不得发布。

macOS 本期继续生成未签名、未公证的 universal DMG，与 Windows 保持同一版本并
共同通过自动构建门槛；取得 Apple Developer ID 后另行规划签名、公证和 stapling。
由于当前实际用户仅使用 Windows ARM64，`v1.0.5` 经用户批准豁免真实 Mac 上的人工
安装和打开验收。该项只能记录为“未验收、风险已接受”，不能记录为通过；macOS Runner
构建、universal 架构和发布资产合同仍必须通过。

## 3. 目标、非目标与边界

### 目标

- 固定 Windows 签名机运行一条受信本地命令，串行构建签名 amd64、arm64；不注册 Runner。
- GitHub Actions 继续冻结来源、构建 macOS、汇总不可变候选和执行正式 Publish。
- 证书使用前后严格验证身份、用途、有效期、私钥、签名和时间戳。
- Draft、Windows 11 ARM 真机验收、Publish 使用同一份不可变候选文件。
- 保留现有版本、Tag、minisign、manifest、OSS 与云发布契约。
- 支持受审批的真实签名 dry-run 和现有 emergency release。

### 非目标

- 不改变更新 manifest 格式或签名协议；允许修改 Go 安装逻辑以阻止已撤回缓存包。
- 不把 SimplySign 密码、私钥、证书指纹或本机绝对工具路径提交到仓库。
- 不支持 Windows 7/8，不增加 SHA-1 双签名。
- 不在本期购买或接入 Apple Developer ID，不把 Certum 证书用于 macOS。
- 不并行构建两个 Windows 架构，也不拆分为云端编译、本机二次打包。
- 不允许 GitHub 事件、远程 Runner、PR 或普通 push 自动调用签名机。

### 运行边界

- 本地脚本从干净受信 checkout 运行，只接受经核对的冻结 Tag/commit；默认要求属于
  `origin/main-v3`，emergency 需额外原因与确认。
- SimplySign 保持登录；本机和操作者账号原有完整仓库权限是已接受边界，不新增候选仓库。
- 本地脚本通过全部验证后才上传 Draft，只提供上传、不调用 Publish；账户仍可人工发布。
- 电脑仍可日常使用，但不得注销、退出 SimplySign、修改 worktree 或并发签名构建。
- 证书指纹、预期主体和本机工具路径放在本机受限配置，不进入仓库或 GitHub Secrets。

## 4. 用户场景与验收标准

- 云端 Prepare 冻结 Tag 并建立 Draft；操作者核对 commit、登录 SimplySign 后运行
  本地命令，依次产出两个已签 installer。
- 手动 dry-run 可真实签名，但只保留临时 Draft，不创建正式 Tag/公开 Release/OSS/云记录。
- emergency 可指定非 `main-v3` 完整 SHA，但原因、确认、审批、签名和 ARM 验收不可跳过。
- SimplySign、证书、时间戳或任一平台失败时，不生成可发布候选，不移动 stable pointer。
- amd64、arm64 均须保留主程序、卸载器、installer 的签名、架构和来源证据。
- macOS Runner 必须成功生成并验证现有 universal DMG；`v1.0.5` 不要求真实 Mac
  人工打开，但不得把未执行的应用验收记为通过。
- 同一 arm64 Draft 候选完成真机安装、启动、升级、卸载后才允许 Publish；
  云端 Publish 只复核和发布该文件，不重新构建或替换。
- 已发布坏版本的紧急处置必须使用独立 `Withdraw` 动作：明确坏版本、恢复版本、
  原因并二次确认；先恢复上一份已知良好 manifest，再发布更高 patch 版本。
- 安装缓存前重取 stable manifest；版本、SHA256 或大小不匹配则失效缓存，网络失败关闭。

Windows 签名还必须满足：

- Signer thumbprint 等于 `WINDOWS_SIGN_CERT_THUMBPRINT` 的规范化值。
- Subject 精确匹配预期公司主体，证书有私钥、在有效期内并含代码签名用途。
- `signtool verify /pa /all` 成功，PowerShell Authenticode 状态为 `Valid`。
- 时间戳证书存在，签名证据记录文件 SHA256、签名主体和时间戳，不记录秘密。
- installer 解包后恰好包含一个正确架构、正确版本且签名有效的主程序。
- 卸载器在 `!uninstfinalize` 的临时文件阶段签名并即时验证；失败必须阻止 NSIS 完成。
- 正式发布与真实签名 dry-run 显式开启 release-only 签名开关；普通开发构建可不签名。
- `!system`、`!uninstfinalize`、`!finalize` 都以 `= 0` 检查退出码；最终发布验证拒绝
  任一层未签名、错误主体或无 RFC 3161 时间戳的 Windows 包。

## 5. 调查证据与实现依据

完整基线、源码定位和实签探针见 [evidence.md](./evidence.md)。实施依据结论是：

- Wails 先生成目标架构主程序，再由 NSIS 嵌入并生成 x86 installer/uninstaller 外壳。
- 现有本地脚本已经具备冻结来源、Windows 串行构建、Draft 不可覆盖上传和三平台发布合同。
- minisign 只保护应用更新完整性，不能替代 Authenticode，且必须在系统签名之后执行。
- SimplySign/SignTool 实签探针已验证证书链和 RFC 3161 时间戳可用；NSIS 三钩子仍须 dry-run。

## 6. 选定方案

### 6.1 签名实现

1. 新增 PowerShell Authenticode 脚本，提供环境预检、单文件签名、即时验证和
   脱敏结构化证据；供三个 NSIS 钩子复用。
2. release-only 开关启用时，`project.nsi` 在 `wails.files` 之前使用编译期
   `!system ... = 0` 签主程序。
3. 同一开关下启用 `!uninstfinalize ... = 0` 签临时卸载器，并用
   `!finalize ... = 0` 签最终 installer；开发构建保持现状。
4. 使用 `/fd sha256 /tr http://time.certum.pl/ /td sha256`；不得静默降级。
5. 证书指纹和预期主体来自本机受限配置，不自动兜底到其他证书。
6. SignTool/SimplySign/NSIS/Wails 固定版本；本地命令只校验，不自动升级。

### 6.2 本地签名与 Actions 编排

1. `Prepare` Workflow 校验版本/来源、冻结 annotated Tag 并创建空 Draft；
   `macos-15` Job 构建未签名 universal DMG 后上传 Draft。
2. 操作者从固定受信 checkout 执行一个本地命令。它核对远端 Tag、commit 和 Draft，
   在隔离 worktree 依次预检、构建/验证 amd64、再构建/验证 arm64。
3. 本地命令仅在六次签名和最终包验证全部通过后上传两个 installer 与脱敏 provenance；
   只接受未发布 Draft，禁止并行覆盖构建目录，同名不同哈希必须失败。
4. Windows 11 ARM 对 Draft 中 arm64 installer 完成真机验收并记录 SHA256。
5. 手动触发 Publish；汇总重验两个 installer 和一个 DMG、确认 ARM 验收哈希后，
   执行 minisign、manifest、GitHub Release、OSS 与云发布，不重新构建。
6. 真实签名 dry-run 使用临时 Draft/保留期，不移动任何公开指针。

### 6.3 失败、重试和紧急撤回

- 签名/时间戳可有限重试；签名主体、架构、版本、来源或哈希错误不可重试掩盖。
- 同一 Tag 的 Draft 资产只有哈希完全一致时可复用，不允许覆盖。
- emergency 不降低签名或真机验收要求。
- 正常 Publish 保持版本单调递增，禁止降级 stable pointer 或云记录。
- 独立 `Withdraw` 先完成全部只读预检，再恢复 OSS stable pointer 并验证；随后把坏
  GitHub Release 标记为 prerelease/non-latest、恢复旧 latest 并验证。
- Withdraw 是有阶段状态、可重复执行和失败后续跑的最终一致流程，不承诺跨 OSS/GitHub
  原子性；任何部分成功必须明确报告，历史资产不覆盖、不删除。
- Withdraw 不写入更旧云记录。客户端只有在云版本与签名 manifest 同版本时才使用云
  下载信息，因此残留的新云记录不会授权更新；已安装坏版本不自动降级，等待更高 patch。
- 已下载未安装的缓存包由安装前 stable manifest 复核阻止；网络失败时不离线安装。

## 7. 预计影响范围

预计修改 `project.nsi`、新增 Authenticode 脚本及测试、扩展 Windows 架构验证、
release 脚本/测试与 Go 安装前复核，调整 Build Draft/Publish Workflow，新增
Withdraw Workflow 及测试，并更新 PR Gate 映射（如需）与发布文档。

禁止修改 `scripts/code_guard_baseline.json`、业务前端、更新 manifest 格式和
minisign 公私钥。准确文件切片在 Stage 2 根据测试承载点拆解。

## 8. 兼容性、风险与控制

- **本机即证书与 GitHub 信任边界**：不安装 Runner、不接受远程任务；操作者核对冻结
  来源后才执行。本机账号完整权限为已接受风险，自动脚本仍不包含 Publish 动作。
- **交互会话不可用**：不安装为服务；发布前人工登录 SimplySign，证书访问失败即停止。
- **持久机器漂移**：固定工具版本，命令开始时校验；升级必须单独评审并通过 dry-run。
- **arm64 无法本机运行**：发布机静态验证，正式 Publish 前使用真实 Windows 11 ARM
  对不可变候选做安装、启动、升级、卸载验收。
- **卸载器不可事后验证**：在 `!uninstfinalize` 中签名、验证并记录证据。
- **证书续期**：指纹不进源码；更新 Environment 变量并重新 dry-run。
- **macOS Gatekeeper**：本期接受未签名风险并保持文档明确；`v1.0.5` 还接受无真实
  Mac 人工验收的剩余风险，但 Runner 构建和资产验证不豁免，后续单独接入 Apple 证书。
- **SmartScreen 信誉**：可信签名解决 Smart App Control 的未知发布者阻止，但不承诺
  新文件首次下载没有 SmartScreen 信誉提示；持续使用同一签名身份积累信誉。
- **NSIS 外壳架构**：当前 installer/uninstaller 为 x86 外壳是允许的；只要求其签名
  有效，且内嵌主程序分别为 amd64/arm64，不能把三份 EXE 都误判为目标架构。
- **电脑日常使用**：发布时不锁定桌面，但不得干扰发布 worktree 或 SimplySign 会话。

## 9. 验证计划

### 自动化

- PowerShell 测试覆盖证书匹配/拒绝、签名参数、重试、时间戳、验证和证据脱敏。
- 合同测试覆盖 release-only 开关、三个钩子及 `= 0`、架构串行、Draft 状态、
  禁止远程签名调度及 Prepare/macOS→本地 Windows→ARM 验收→Publish 顺序。
- 架构验证增加签名主体、时间戳和解包主程序断言。
- release 测试覆盖 Draft 不可变、Publish 不重建、ARM/emergency、minisign 顺序、
  正常发布禁止降级和 Withdraw 分阶段幂等续跑。
- Go 测试覆盖安装前 manifest 复核、已撤回缓存拒绝/失效和网络失败关闭。

### 应用验收

- amd64 在签名机、arm64 在启用 Smart App Control enforcement/audit 的真实 Win11 ARM
  上安装、启动、升级、卸载，检查三份 EXE 发布者、CodeIntegrity 日志及原阻止弹窗消失。
- Publish 前复核候选 SHA256 与真机验收记录完全一致。
- 区分 Smart App Control 执行阻止与 SmartScreen 下载信誉提示，不把后者当签名失败。
- macOS Runner 必须回归现有 universal DMG 架构和打包合同，不宣称已签名或公证；
  `v1.0.5` 的真实 Mac 可打开性未验收且经用户明确接受风险。

创建或更新 PR 前，先同步最新 `origin/main-v3`，完成聚焦验证和适用的真实 dry-run，
再执行仓库 `AGENTS.md` 的 Common checks 与：

```powershell
node ./scripts/pr-gate.mjs --base origin/main-v3 --run all
```

## 10. 未解决问题与停止条件

当前没有待产品决定的问题。Stage 2 需把本机配置、ARM 验收记录、签名证据、
Draft/Publish、安装前复核、Withdraw 及具体脚本接口固化为任务合同。

若 SimplySign 需导出私钥/无人值守密码、拆分 Wails/NSIS 或再次降低验收，应停止并
重新批准。`v1.0.5` 仅豁免真实 Mac 人工验收，不豁免 macOS 自动构建和资产合同。

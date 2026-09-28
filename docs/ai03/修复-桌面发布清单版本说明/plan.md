# 桌面发布清单版本说明修复计划

## 背景与问题

从 `desktop-v1.0.6` 检查稳定更新时，客户端按既定迁移路线先发现
`desktop-v1.1.0`。旧全局 stable manifest 冻结在该迁移版本是有意设计；安装
v1.1.0 后，客户端才会使用按平台 stable pointer 继续发现 v1.1.1。

异常仅在版本说明：公开的 v1.1.0 全局 manifest 与 v1.1.1 目标 manifest 的
`notes` 均为空。云端 latest 只补充同版本说明；当 Windows 云端 latest 前进到
v1.1.1 后，v1.0.6 针对 v1.1.0 的说明无法再由云端补充，界面因此显示“暂无版本说明”。

## 当前与预期行为

- 当前：GitHub Actions 发布入口会向 `cmd/sign manifest` 传入冻结说明文件，本地
  `desktop-release.ps1 -Action Finalize` 的 target 与 bootstrap 四类 manifest
  调用均漏传 `--notes-file`，生成器又允许省略该参数，因此发布出空 `notes`。
- 预期：所有后续桌面 manifest 都内嵌冻结版本说明；说明参数缺失、文件为空，或本地
  快照与冻结来源不一致时，发布必须在任何远端写入前失败。
- 保持：v1.0.6 -> v1.1.0 -> v1.1.1 的两段迁移升级不变。

## 目标

- 修复本地 Finalize 生成 target GitHub、target OSS、bootstrap GitHub 与 bootstrap
  OSS manifest 时遗漏版本说明的问题。
- 将 `cmd/sign manifest` 的 `--notes-file` 设为发布 CLI 必填参数，并拒绝空白说明，
  防止其他调用方再次静默生成空说明。
- 用合同测试证明每个本地 Finalize manifest 命令都绑定同一份已验证说明快照。
- 保留现有冻结来源、零写入预检和远端发布顺序。

## 非目标与边界

- 不回填或覆盖 v1.1.0、v1.1.1 已发布的 GitHub/OSS manifest、stable pointer 或云记录。
- 不改变云端 `client-releases/latest` API、客户端同版本说明合并规则或更新 UI。
- 不让 v1.0.6 绕过 v1.1.0 直接发现 v1.1.1。
- 不移动标签、不重打包、不重新签名，也不执行真实发布、Withdraw 或其他生产写入。
- 不改变 manifest schema、资产身份、目标选择、下载授权或回滚语义。

## 用户场景与验收标准

1. 发布者对任意 target 执行本地 `Finalize -DryRun` 时，生成的 GitHub 与 OSS manifest
   均携带冻结版本说明。
2. bootstrap 发布时，两个全局 manifest 同样携带该说明。
3. `cmd/sign manifest` 未提供 `--notes-file`、说明文件不可读或内容为空白时返回错误，
   不生成可发布 manifest。
4. 本地已有 `release-notes.md` 与源标签提交中的 `docs/releases/vX.Y.Z.md` 不一致时，
   延续现有行为，在 manifest 生成和任何远端写入之前失败。
5. GitHub Actions 发布路径继续生成相同说明内容，不发生行为回退。
6. v1.0.6 的两段升级与历史线上数据保持原状。

## 证据与根因

基线为 `origin/main-v3` 的 `8ff823f69bc77d7ea8cad6b0c758d70a2535c38e`。本会话没有
可用的 Codegraph 工具，因此以下结论来自该基线源码、标签源码、公开只读元数据及测试
合同的直接核对：

- `desktop-v1.0.6:desktop/updater.go` 只读取旧全局 stable/GitHub Latest manifest；
  `desktop-v1.1.0` 才加入目标 pointer，证明两段升级是既定迁移边界。
- `docs/releases/desktop.md` 明确规定旧全局入口冻结在迁移版本，迁移后的部分版本不会由
  该入口发现；云端 latest 只补充同版本说明。
- `docs/releases/v1.1.0.md` 内容非空，而 2026-08-01 公开读取的 v1.1.0 全局 manifest
  为 `"notes": ""`；v1.1.1 Windows target manifest 也为空。
- `scripts/desktop-release.ps1` 已从冻结提交读取、验证并保存 `release-notes.md`，但
  `Invoke-Finalize` 的四类 `cmd/sign manifest` 参数均没有 `--notes-file $notesPath`。
- `scripts/desktop-actions-publish.ps1` 的对应四类调用全部传入 `--notes-file`，说明问题
  局限于受支持的本地发布入口。
- `desktop/cmd/sign/main.go` 将说明参数定义为可选；省略时合法输出空 `notes`。
- `desktop/cmd/sign/main_test.go` 已证明传入说明文件后 GitHub 与 OSS manifest 会保留
  精确内容；`scripts/desktop-release-test.ps1` 已捕获 Finalize 的所有 manifest 命令，
  但只断言平台和位置参数，未断言说明参数。

根因是两个可叠加的合同缺口：本地 Finalize 漏传已准备好的说明路径；底层 manifest
CLI 又允许调用者省略说明，测试未把“每个发布 manifest 必须包含冻结说明”锁定为门禁。

## 方案

1. 在 `cmd/sign manifest` 参数解析后要求非空 `--notes-file`；读取文件后拒绝空白内容，
   再按现有方式原样写入 manifest。
2. 在本地 Finalize 的 target GitHub/OSS 与 bootstrap GitHub/OSS 调用中统一加入
   `--notes-file $notesPath`。
3. 扩展本地 Finalize DryRun 合同测试，逐条验证所有 target 和 bootstrap manifest
   命令恰好携带预期说明快照路径。
4. 扩展签名工具测试，覆盖缺少参数、空白文件失败，以及合法说明仍精确写入的行为。
5. 以真实 `Finalize -DryRun` 作为合并后的发布机验收；本次开发不执行生产发布。

## 预计影响文件

- `desktop/cmd/sign/main.go`
- `desktop/cmd/sign/main_test.go`
- `scripts/desktop-release.ps1`
- `scripts/desktop-release-test.ps1`
- `docs/ai03/修复-桌面发布清单版本说明/`

GitHub Actions 帮助脚本当前行为正确，除非实现核对发现合同测试必须加强，否则不修改。

## 兼容、迁移与回滚

- `cmd/sign manifest` 是内部发布 CLI；仓库内正式调用只有 Actions 与本地 Finalize。
  Actions 已传入说明，本地入口随本修复同步补齐，因此必填化不会改变成功发布语义。
- 历史 manifest 保持不可变，不需要数据迁移。
- 回滚代码可恢复旧 CLI，但会重新允许空说明发布；若修复产生回归，应停止发布并回滚
  代码，不得通过省略说明绕过门禁。

## 风险

- 参数插入位置错误可能破坏 PowerShell 数组绑定；现有逐命令参数捕获测试会锁定路径、
  平台参数和末尾三个位置参数。
- 换行归一化不一致可能误判；本地入口继续复用现有 `Assert-ReleaseNotesText` 和冻结快照
  比较，manifest 生成器保持文件字节内容，不另做格式重写。
- 仅修调用方会留下再次遗漏的可能，因此 CLI 必填合同与调用方测试需同时落地。

## 验证计划

聚焦自动化验证：

```powershell
go test ./cmd/sign
pwsh -NoProfile -File scripts/desktop-release-test.ps1
pwsh -NoProfile -File scripts/desktop-actions-publish-test.ps1
```

应用/发布行为验证：

- 在固定 Windows 发布机使用后续测试版本执行完整 `Finalize -DryRun`。
- 检查生成的 target GitHub/OSS manifest；`notes` 必须与冻结版本说明一致。
- 对 bootstrap 场景同样检查两个全局 manifest。
- 确认 DryRun 报告零远端写入，并完成全部 immutable、云记录和 pointer 预检。

PR 前在同步后的最终分支运行：

```powershell
node ./scripts/pr-gate.mjs --base origin/main-v3 --run all
```

并按仓库规则核对分类摘要、代码守卫、baseline 保护、EOL 与 diff check。真实发布需另行
审批，不属于本计划的验证动作。

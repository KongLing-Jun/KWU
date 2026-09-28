# 桌面本地发布 DPAPI 交接清理修复计划

## 背景与问题

PR #253 恢复了固定 Windows 发布机上的完整本地 `Finalize` 兜底，并为短期云端
管理员 Token 增加了 Windows CurrentUser DPAPI 文件交接。正常路径会在解密成功后
删除交接文件，并在命令结束时恢复调用者原有的
`KWU_CLOUD_RELEASE_TOKEN` 环境变量。

合并后独立审查发现：交接文件已经被读取，但 DPAPI 内容为空、格式损坏、属于其他
Windows 用户或解密过程抛错时，删除语句不会执行。一次性凭据文件因此可能残留在
磁盘，违反“读取后无论成功或失败都删除”的安全合同。

## 当前行为与根因证据

最新 `origin/main-v3` 基线为
`2df3a7c454a102da8ffe6c517d79aa8ada6a260a`。相关实现集中在：

- `scripts/desktop-release.ps1:253-279`：
  `Get-DPAPICloudReleaseToken` 读取并解密文件。
- `scripts/desktop-release.ps1:1487-1498`：
  `Invoke-FinalizeWithCredentialHandoff` 只有在
  `Get-DPAPICloudReleaseToken` 成功返回后才执行 `Remove-Item`；外层
  `finally` 会恢复原环境变量。
- `scripts/desktop-release-test.ps1:215-237`：现有合同测试只覆盖成功解密、
  文件删除和环境恢复。

审查使用损坏的 DPAPI 内容调用现有入口，得到：

```json
{
  "handoff_file_exists_after_failed_decrypt": true,
  "environment_after_failed_decrypt": "sentinel-before-handoff"
}
```

这证明环境恢复正常，但失败解密绕过了文件删除。发布尚未进入 `Invoke-Finalize`，
所以缺陷不会导致半发布；风险是一次性交接文件在失败后留存。

对索引主工作区 `D:\Reasonix` 执行了 Codegraph 查询。当前索引没有 Git commit
元数据，且未形成 PowerShell 函数的有效调用图；查询时主工作区 HEAD 也落后于远端。
因此 Codegraph 仅作为导航证据，以上结论和影响范围以最新隔离 worktree 的直接源码、
运行复现和最终差异为准。

## 目标

- 交接文件一旦成功读取原始内容，无论内容为空、解密失败还是解密成功，都在继续
  任何发布步骤前删除。
- 删除失败必须停止发布，不能带着仍在磁盘上的一次性文件进入 `Finalize`。
- 所有成功和失败路径都恢复调用者原有的 `KWU_CLOUD_RELEASE_TOKEN`。
- 错误与测试输出不得包含 Token 明文或 DPAPI 密文。
- 保持现有本地完整发布门禁和 Actions 发布路径不变。

## 非目标与边界

- 不重新发布或修改 v1.0.6 的 GitHub Release、OSS stable 或云端记录。
- 不改变 DPAPI CurrentUser 加密格式、minisign 凭据处理或云端 Token 权限模型。
- 不改变 canonical annotated Tag、`main-v3`、精确 `PUBLISH`、ARM64 SHA256、
  cloud required、禁止 skip/emergency 等门禁。
- 不处理桌面更新器或其他 PR 的范围。
- 不在本修复中轮换生产凭据；已暴露凭据的撤销和轮换属于独立运维动作。

## 已选方案

将“读取、解密、读取后清理”收敛在 DPAPI Token 获取边界内：

1. 只有 `Get-Content` 成功返回后，才把文件标记为已读取。
2. 已读取后使用 `finally` 删除原文件；空内容、格式错误和 DPAPI 解密错误同样进入
   该清理路径。
3. 删除在函数返回 Token、设置进程环境变量和调用 `Invoke-Finalize` 之前完成。
4. 删除失败视为安全失败，立即停止，不执行 `Invoke-Finalize`。
5. 保留 `Invoke-FinalizeWithCredentialHandoff` 的外层环境快照和 `finally` 恢复。
6. 移除调用者中只覆盖成功返回路径的重复删除，避免双重删除和不清晰的所有权。

未成功读取的场景，例如文件不存在或打开失败，不声称已消费文件；入口仍应 fail
closed，且不得进入发布。

## 用户场景与验收标准

| 场景 | 预期结果 |
| --- | --- |
| 有效 CurrentUser DPAPI 文件 | 文件在进入 `Finalize` 前删除；Token 仅在本次调用环境中可见；结束后恢复原环境 |
| 文件内容为空 | 命令失败；已读取文件删除；原环境恢复；不进入 `Finalize` |
| 文件内容损坏或无法解密 | 命令失败；已读取文件删除；原环境恢复；不进入 `Finalize` |
| 文件删除失败 | 命令失败并停止发布；不得执行云预检或任何不可逆发布动作 |
| 文件不存在或读取失败 | 命令 fail closed；不进入 `Finalize`；不改变原环境 |
| 未提供 `CloudTokenDPAPIFile` | 继续使用现有进程环境/安全输入语义，不新增文件操作 |

验收时还必须确认：

- 成功路径、失败解密路径和空文件路径都不输出 Token 或密文。
- `SkipCloud`、`CloudSkipReason`、`AllowNonMainV3` 和 `EmergencyReason` 仍被拒绝。
- v1.0.6 真实已发布状态的 `Finalize -DryRun` 仍只读通过。

## 预计影响文件

- `scripts/desktop-release.ps1`：调整 DPAPI 文件清理所有权和失败语义。
- `scripts/desktop-release-test.ps1`：增加空内容、损坏内容、环境恢复及未进入
  `Finalize` 的回归合同。

发布手册已经声明一次性凭据应被清理，预期行为不变，因此不计划修改发布文档。

## 验证计划

### 聚焦自动验证

- `pwsh scripts/desktop-release-test.ps1`
- 运行损坏 DPAPI 内容用例，确认文件不存在、环境恢复、发布入口未执行。
- 运行空文件用例，确认相同的清理与停止语义。
- 运行有效 DPAPI 内容用例，确认现有成功路径不回归。
- 分别验证四类本地发布绕过参数仍被拒绝。

### 应用级验收

- 对已发布 v1.0.6 执行带真实确认和 ARM64 SHA256 的
  `Finalize -DryRun`，确认完整兜底入口仍可读取真实 Release 状态且不写远端。
- 不执行真实 `Finalize`，避免为凭据清理修复重新触发正式发布。

### PR Gate

在同步最终 `origin/main-v3` 后运行：

```powershell
node ./scripts/pr-gate.mjs --base origin/main-v3 --run all
```

记录实际分类、发布合同测试和 Windows installer 合同结果；任何必需检查失败时不得
创建或更新 PR。

## 兼容性、风险与回滚

- 有效 DPAPI 文件和直接环境变量路径保持兼容；变化仅影响已读取后的失败清理。
- 主要风险是清理时机错误导致有效 Token 在使用前不可用，或删除失败被忽略。测试必须
  同时证明“先完成解密、再删除文件、随后使用内存 Token”以及“删除失败不发布”。
- 回滚代码与测试提交即可恢复旧行为；本修复不迁移数据，也不改变已发布远端状态。
- 回滚会重新暴露失败解密后文件残留风险，因此只能作为短期故障处置，不能作为长期
  解决方案。

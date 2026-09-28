# 任务 001：修复 DPAPI 交接文件失败路径清理

## 状态与分组

- 状态：已批准，待实施；执行组：串行组 A
- 依赖：已合并的 `plan.md` 和计划 PR #256
- PR 映射：一个独立实现 Draft PR
- 实施代理：`gpt-5.6-sol`，medium reasoning，不得继续委派
- 审查与应用验收：主会话独立执行

本修复只有一个可独立审查、测试、合并和回滚的交付切片，不安排并行任务。

## 任务目标

修正固定 Windows 发布机的 DPAPI 云 Token 文件交接：原始文件一旦成功读取，无论
内容有效、为空或无法解密，都必须在进入 `Finalize` 前删除；删除失败必须 fail
closed。所有退出路径继续恢复调用者原有的 `KWU_CLOUD_RELEASE_TOKEN`。

## 预期产物

- `scripts/desktop-release.ps1`：统一读取后清理所有权，覆盖失败解密和空内容，删除
  失败阻止 `Invoke-Finalize`，保留环境恢复。
- `scripts/desktop-release-test.ps1`：覆盖有效、空内容、损坏内容、缺失文件和删除
  失败，证明失败未进入发布且输出无凭据内容。
- 本任务文档：实施完成时更新真实文件、命令结果、偏差、风险和交接收据。

## 根因与源码证据

基线：`origin/main-v3` `caf1bf0d31f2390c4f2a578b011750369cb834e0`。

- `scripts/desktop-release.ps1:253-279` 的
  `Get-DPAPICloudReleaseToken` 在读取后执行空值检查和解密。
- `scripts/desktop-release.ps1:1487-1498` 的
  `Invoke-FinalizeWithCredentialHandoff` 只在 Token 获取成功返回后删除文件。
- `scripts/desktop-release-test.ps1:215-237` 只覆盖有效 DPAPI 内容的删除与环境恢复。
- 损坏内容运行复现已经证明：函数报错、环境恢复，但文件仍存在。

Codegraph 对索引主工作区的 PowerShell 调用图无有效结果，且索引没有 Git commit
元数据；实施必须以本 worktree 源码、实际测试输出和最终 Git 差异为准。

## 允许修改范围

- `scripts/desktop-release.ps1`
- `scripts/desktop-release-test.ps1`
- 本任务文档，用于更新完成收据

实施代理只可修改上述三个文件；`progress.md` 由主会话在审查后更新。

## 禁止范围

- 不修改 `plan.md`；目标、边界、风险或验收标准实质变化时停止并重新审批。
- 不修改发布手册、Actions Workflow、Go、前端、安装器、云 API 或更新器。
- 不改变 DPAPI 格式、Token 权限模型、minisign 凭据处理和直接环境变量路径。
- 不改变 canonical annotated Tag、`main-v3`、精确确认、ARM64 SHA256、
  cloud required、禁止 skip/emergency 等门禁。
- 不接触生产 Token、minisign 私钥或真实 DPAPI 文件；不执行真实 `Finalize`。
- 不修改 GitHub Release、OSS stable 或云端记录。
- 不推送、创建或更新 PR，不标记 Ready，不合并。

## 必须保持的行为

- 未提供 `CloudTokenDPAPIFile` 时，现有进程环境或安全终端输入语义保持不变。
- 文件不存在或读取失败时 fail closed，不进入发布，不改变原环境。
- 有效文件解密后，内存 Token 仍供本次 `Finalize` 使用；所有退出路径恢复环境。
- 本地完整发布继续拒绝 `SkipCloud`、`CloudSkipReason`、
  `AllowNonMainV3` 和 `EmergencyReason`。
- 现有 Actions 发布入口和发布顺序不变。

## 实施约束

1. 只有 `Get-Content` 成功返回后才标记已读取；删除位于覆盖空内容、格式解析和
   DPAPI 解密的 `finally` 路径。
2. 文件在返回 Token、赋值环境变量和调用 `Invoke-Finalize` 前删除。
3. 删除失败不得被吞掉；错误信息不能包含 Token、密文或其他凭据内容。
4. 移除调用者的重复删除，保留外层环境快照与 `finally` 恢复。
5. 删除失败测试优先使用作用域受控的确定性替身；不得更改真实凭据目录 ACL，也不得
   留下测试文件、进程环境或命令替身。
6. 测试只用合成 Token 和临时路径，清理失败时也有独立兜底清理。

## 验收标准

| ID | 场景 | 必须观察到的结果 |
| --- | --- | --- |
| AC-01 | 有效 CurrentUser DPAPI 文件 | 文件在进入 `Finalize` 前删除；本次调用可使用内存 Token；结束后恢复原环境 |
| AC-02 | 已读取的空文件 | 命令失败；文件删除；环境恢复；不进入 `Finalize` |
| AC-03 | 已读取的损坏或不可解密内容 | 命令失败；文件删除；环境恢复；不进入 `Finalize` |
| AC-04 | 删除操作失败 | 命令失败；不得执行云预检、签名、上传或其他发布阶段；环境恢复 |
| AC-05 | 文件不存在或无法读取 | fail closed；不进入 `Finalize`；原环境不变 |
| AC-06 | 未提供交接文件 | 直接环境变量路径保持现有行为，不新增文件操作 |
| AC-07 | 失败输出 | 不包含合成 Token、DPAPI 密文或生产凭据路径内容 |
| AC-08 | 发布门禁回归 | 四类 skip/non-main/emergency 参数继续被拒绝 |
| AC-09 | 真实状态只读验收 | v1.0.6 `Finalize -DryRun` 通过且不写远端 |

## 聚焦验证

实施代理运行 `pwsh scripts/desktop-release-test.ps1`，显式覆盖 AC-01 至 AC-08。
迭代时可运行单个用例定位，但交付前必须运行完整脚本。

实施代理还必须检查：

```powershell
git diff --check
git status --short
git diff -- scripts/desktop-release.ps1 scripts/desktop-release-test.ps1
git ls-files --eol -- scripts/desktop-release.ps1 scripts/desktop-release-test.ps1
```

## 独立应用验收

实施代理完成后，主会话在不使用生产凭据的前提下独立执行：

1. 复跑损坏内容、空内容和有效 DPAPI 文件场景，核对文件、环境与错误输出。
2. 使用已公开的 v1.0.6 Tag、精确确认和已记录 ARM64 SHA256 运行 `Finalize -DryRun`。
3. 分别运行四类绕过参数拒绝用例。
4. 确认没有 GitHub Release、OSS 或云端写操作。

实施代理的自测不能替代该独立验收。

## 最终 PR Gate

应用验收稳定后，主会话同步最新 `origin/main-v3`，根据共享状态选择 merge 或
rebase，再运行 `node ./scripts/pr-gate.mjs --base origin/main-v3 --run all`。

最终 PR 描述必须记录分类、应用验收、自动检查、风险、回滚和实际提交映射。任何必需
检查失败时不得创建或更新 PR。

## 风险与停止条件

- 如果安全删除必须改变 DPAPI 格式、公共参数、发布顺序或凭据模型，立即停止并更新
  `plan.md` 重新审批。
- 如果无法在不接触真实凭据或不修改真实 ACL 的情况下确定性验证删除失败，停止并由
  主会话决定最小测试接缝；不得以静态字符串断言代替核心失败语义。
- 如果实现使有效 Token 在删除后不可用于本次调用，或任何失败路径未恢复环境，不得
  提交。
- 如果发现 #253 之外的发布缺陷，只记录证据并报告，不得扩展本任务。

## 完成收据

- 状态：实施、主审修正和独立应用验收完成，待实现 PR 交付授权。
- 修改文件：两个获准发布脚本和本任务收据。
- 聚焦测试：`pwsh scripts/desktop-release-test.ps1` 通过，覆盖 AC-01 至 AC-08；
  验证清理先于 `Finalize`、fail closed、环境恢复、直接环境路径和敏感输出净化。
- 静态检查：`git diff --check` 通过，三文件均为 LF；无 Go、模块或前端变更。
- 应用验收：主会话复跑聚焦测试；公开 v1.0.6 `Finalize -DryRun` 退出码为 0；
  四类绕过参数退出码均为 1；合成路径 marker 未泄露，全程无发布远端写操作。
- 偏差：首次提交遗漏部分路径净化，主审发现后由后续 `fix:` 提交补齐读取、删除及
  内容失败边界；未改变 DPAPI 格式、公共参数、发布顺序或凭据模型。
- 剩余风险：PR 授权后仍需同步基线、复跑关键路径和最终 Gate；生产凭据轮换或撤销
  是独立运维动作。
- 交接：两个实现提交保留审查历史，哈希以 Git 为准；工作区干净，三文件保持 LF。

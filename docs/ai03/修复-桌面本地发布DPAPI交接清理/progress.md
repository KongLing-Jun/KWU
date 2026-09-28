# 桌面本地发布 DPAPI 交接清理进展

## 当前阶段

Stage 3 实施、主会话代码审查修正和独立应用验收已完成。计划 PR #256 与任务
PR #257 已合并；尚未推送实现分支、创建实现 PR 或修改任何发布远端状态。

当前分支：`codex/fix-desktop-release-dpapi-cleanup`

基线：`origin/main-v3` `32c251dcb6e46e246541d55834ed8e206f854201`

## 已确认决策

- 交接文件一旦成功读取，解密成功、空内容或解密失败都必须删除。
- 删除必须发生在进入 `Finalize` 前；删除失败必须停止发布。
- 原有 `KWU_CLOUD_RELEASE_TOKEN` 在所有退出路径恢复。
- 不改变发布门禁、DPAPI 格式、Actions 路径或已发布 v1.0.6 状态。
- 本修复只涉及发布脚本及其合同测试。

## 证据状态

| 项目 | 状态 |
| --- | --- |
| 源码修复 | 已完成：读取成功后的统一 `finally` 清理覆盖成功与失败解密 |
| 主审修正 | 已完成：失败输出不再泄露 Token、密文或交接文件路径 |
| 失败语义 | 已验证：读取或删除失败均 fail closed，未进入 `Finalize` |
| 成功与环境恢复 | 已验证：文件先删除，内存 Token 可用，原环境在所有路径恢复 |
| v1.0.6 发布状态 | 已独立确认 GitHub、OSS 和四条云记录一致 |
| Codegraph | 已查询；PowerShell 调用图无有效结果，最终结论使用最新 worktree 直接证据 |

## 任务与 PR 映射

| 任务 | 分组与依赖 | 实现 PR | 当前状态 |
| --- | --- | --- | --- |
| `001-修复DPAPI交接文件失败路径清理` | 串行组 A；依赖已合并计划与任务合同 | 一个独立实现 Draft PR | 实施与独立验收完成 |

任务 001 同时包含脚本修复、失败路径合同测试和完成收据，是唯一可独立交付切片，
不安排并行任务。

## 验证状态

- `pwsh scripts/desktop-release-test.ps1`：实现代理与主会话分别完整运行并通过。
- 公开 v1.0.6 `Finalize -DryRun`：通过，只报告完整发布顺序和 cloud required。
- `SkipCloud`、`CloudSkipReason`、`AllowNonMainV3`、`EmergencyReason`：分别以
  非零退出拒绝。
- 合成缺失路径：失败且错误不含路径 marker；无发布远端写操作。
- 两个脚本与任务收据保持 LF，当前实现 worktree 干净。

## 下一道门

用户明确授权创建实现 Draft PR。授权后先同步最新 `main-v3`，复跑聚焦测试、
v1.0.6 关键 DryRun 和最终 PR Gate；全部通过才推送并创建 PR。Ready 与合并仍需
各自独立授权。

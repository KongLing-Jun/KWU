# 桌面发布清单参数拼接进度

## 当前状态

- 阶段：计划、任务与实现按用户明确授权 stacked 到同一个 PR。
- 状态：代码、聚焦回归测试和完整 PR Gate 均通过，准备创建单个 Draft PR。
- 基线：`origin/main-v3`。

## 已确认决策

- 仅修复 target manifest 的参数拼接与合同测试。
- `desktop-v1.1.0` 标签、三平台资产、签名和哈希全部保持不变。
- 不绕过 `Finalize -DryRun`，不提前执行任何正式发布写入。

## 根因与证据

- 失败稳定发生在首个 `windows-amd64` GitHub manifest。
- 完整 manifest 命令单独执行成功。
- PowerShell 参数捕获显示原表达式只绑定第一个数组，遗漏动态平台和末尾位置参数。

## 下一门禁

1. 创建单个 Draft PR 并等待合并。
2. 合并后重跑 v1.1.0 `Finalize -DryRun`。

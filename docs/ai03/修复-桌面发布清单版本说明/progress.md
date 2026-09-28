# 桌面发布清单版本说明修复进度

## 当前状态

- 阶段：Stage 3 实现完成。
- 状态：任务 001 已实现并通过聚焦验证；代码与文档交付完成，外部 Git/PR 状态以
  GitHub 为准。
- 基线：`origin/main-v3`；计划 worktree 起点为
  `8ff823f69bc77d7ea8cad6b0c758d70a2535c38e`。
- 分支：`codex/fix-release-manifest-notes`。

## 已确认决策

- 保留 v1.0.6 -> v1.1.0 -> v1.1.1 的两段迁移升级。
- 不修改 v1.1.0/v1.1.1 历史线上 manifest 或云数据。
- 只保证后续版本的 manifest 内嵌冻结版本说明。
- 说明缺失、为空或与冻结来源不一致时，在任何远端写入前失败。
- 不修改客户端、云 API、更新路由或 UI。

## 根因摘要

- 本地 `Finalize` 已生成并验证 `release-notes.md`，但四类 manifest 命令漏传
  `--notes-file`。
- `cmd/sign manifest` 允许省略说明参数并静默生成空 `notes`。
- Actions 发布入口已传入说明；现有本地合同测试未断言说明参数。

## 计划交付

- 修复计划：`plan.md`。
- 执行任务：`tasks/001-修复发布清单版本说明合同.md`。
- 串行组：任务 001 独立完成 CLI 合同、调用方修复、聚焦测试与完成回执。
- PR 映射：计划、任务与实现按用户授权 stacked 在当前分支；后续若获 PR 授权，作为
  一个可独立验证和回滚的修复 PR 提交。
- 生产发布与历史数据修改仍不在范围内。

## 验证与门禁

- 已完成：源码、标签、公开 manifest、云端 latest 与现有测试合同的只读核对。
- 已完成：计划文档行数、EOL、Git diff check 与仓库状态核对。
- 已通过：`go test ./cmd/sign`。
- 已通过：`pwsh -NoProfile -File scripts/desktop-release-test.ps1`。
- 已通过：`pwsh -NoProfile -File scripts/desktop-actions-publish-test.ps1`。
- 已通过：`node ./scripts/pr-gate.mjs --base origin/main-v3 --run all`；分类为 Desktop Go
  targeted `./cmd/sign` 与 release contract，代码守卫、Markdown 链接、baseline 保护、
  diff check、签名工具测试及 28 项 Windows installer contract 全部通过。
- 待合并后应用验收：固定 Windows 发布机对后续测试版本执行完整 `Finalize -DryRun`，
  公开确认所有 target/bootstrap manifest 的 `notes` 与冻结说明一致。本次未使用生产
  凭据，也未执行任何远端发布写入。

## 下一门禁

1. 按外部 PR 流程完成最终同步与 Gate；不在版本化文档中记录或推断 PR 状态。
2. 合并后在固定发布机完成真实 `Finalize -DryRun` 应用验收。
3. 正式发布仍需独立审批。

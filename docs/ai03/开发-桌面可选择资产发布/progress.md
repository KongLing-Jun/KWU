# 桌面可选择资产发布与按目标更新迁移进展

## 当前阶段

Stage 2 任务分解与本地审查已完成，等待用户审批。计划 PR #259 已合并；尚未实施
代码、执行生产发布、推送任务分支或创建任务 PR。

当前分支：`codex/tasks-desktop-selective-release`

任务拆解基线：`origin/main-v3` `5a189b3213634f8998d4c5144aa735a6549b2a7d`

## 已确认决策

- 发布目标为 `windows-amd64`、`windows-arm64`、`macos-universal`；空选择失败，
  `all` 必须显式选择。
- 未选目标不构建、不验证、不阻塞且不改变 stable/云端状态。
- 同版本允许分批追加目标；同名同内容幂等，同名不同内容和目标降级拒绝。
- Actions 与本地入口共享目标、确认和远端状态合同。
- Windows 继续由固定本地机签名构建，Actions 只发布已上传的 Windows Draft 资产；
  macOS Universal 继续由 macOS Actions runner 构建。
- 确认精确绑定 Tag 与归一化目标集合；发布者对非客观冲突的质量警告拥有最终决定权。
- 每个已选目标仍完整发布到 GitHub、OSS immutable/stable 和对应云端记录。
- canonical annotated Tag、`main-v3`、标签内发布说明、minisign 和 cloud required
  保持强制，不引入 skip 或 emergency source bypass。
- 先发布一次完整迁移版本；此后只推进目标索引，旧全局 `latest.json` 和 GitHub
  Latest 停留在迁移版本。
- 部分发布以冻结的全局 OSS/GitHub 迁移入口和三个已建立的目标 pointer 作为迁移
  就绪证据，不新增 bootstrap marker。
- 按目标 Withdraw 只恢复对应 pointer，不改变共享 GitHub Release 或其他目标。
- 跨目的地中断用相同 Tag/目标集合幂等续跑，不自动删除或回滚已成功对象。

## 证据状态

| 项目 | 状态 |
| --- | --- |
| 最新基线与仓库指令 | 已核对 |
| 当前 Actions/本地全量限制 | 已从 workflow、PowerShell 直接确认 |
| manifest 生成扩展点 | 已确认已有按平台生成能力，可保持 schema |
| 云端独立目标语义 | 已确认已有按平台/架构单调与幂等基础 |
| updater 迁移影响 | 已确认当前固定依赖两条全局 manifest URL |
| 云端与 Withdraw | 云 API 为追加式；目标 pointer 保持版本权威，Withdraw 纳入目标化 |
| Codegraph | 已查询；索引落后，仅作导航，结论已由最新 worktree 复核 |

## 任务与 PR 映射

| 任务 | 依赖与拓扑 | 实现 PR | 状态 |
| --- | --- | --- | --- |
| [`001-实现桌面可选择资产发布闭环`](tasks/001-实现桌面可选择资产发布闭环.md) | 无前置任务；单任务串行实施 | 一个原子实现 PR | 待实现 |

发布脚本、更新器目标索引、迁移和撤回必须作为一个闭环一起评审、验证和回滚，不拆分
中间任务或实现 PR。实现 PR 合并后仍需独立批准真实迁移版本。

## 下一道门

用户审批任务分解后，再单独决定是否授权任务 Draft PR。未获得独立授权前不推送、
不创建或更新 PR。

## 剩余工作

- Stage 2 PR：授权后同步、运行文档范围检查和最终 PR Gate、创建 Draft PR。
- Stage 3：任务分解合并并获明确授权后，用一个实现代理完成一个原子实现 PR。
- 生产迁移版本、后续真实发布和任何 Withdraw 操作分别另行审批。

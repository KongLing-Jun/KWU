# Desktop 完成态永久加载与工具卡闪烁修复进度

## 当前状态

| 项目 | 状态 |
| --- | --- |
| 当前阶段 | Stage 3：实现、主代理与用户联合验收完成；已同步最新基线，准备小步提交与 PR Gate |
| 复现基线 | `61729d22a39c31cc79b665ff50913081df0e9d10` |
| PR 同步基线 | `origin/main-v3@1e2ffd49fa041e4e3098d3722f45985922a11ed7` |
| 计划文档 | [`plan.md`](./plan.md) |
| 任务回执 | [`001`](./tasks/001-修复完成态永久加载.md)、[`002`](./tasks/002-修复工具卡披露状态.md) |
| 实现 | 永久加载修复与工具卡用户独占 disclosure 均已完成并通过真实应用验收 |
| Git/PR | 未提交、未 push、未创建 PR；以 Git 为唯一事实源 |

## 已确认决策

- 只解决真实复现的 Transcript 完成态永久加载链路；独立 hydration/load-sequence 竞态不纳入本次。
- 新 identity 的 bottom activation 立即 reveal；普通 Transcript 首次跨入虚拟化时保护原有 pinned 状态。
- 用户真实滚动仍可取消保护并保存 semantic anchor；专家工作区沿用既有语义。
- 工具卡诊断仅在 DEV 启用，记录 opaque token 和展示状态，不记录工具名、参数、输出或正文。
- 同次复现暴露的工具卡自动展开/收起已纳入范围；用户确认所有程序状态不得改变 disclosure。
- disclosure 以当前 `semanticViewKey` 为作用域；允许用户选择跨虚拟化和完成态工具分组 remount 保留，
  但不跨会话泄漏。

## 验证快照

| 验证面 | 结论 |
| --- | --- |
| 修复前真实应用 | 权威基线、真实 `gpt/gpt-5.6-luna` 连续 3/3 复现；完成事件和 final 正常，前端正文被无任务恢复态永久遮住 |
| 修复中对照 | `[USERDIAG-DESIGN-TOOL-FLICKER]` 的 final 当前页直接可见；同时精确捕获普通行跨阈值后的虚拟列表复位、伪顶部 anchor 和工具卡自动展开/收起 |
| 修复后真实应用 | `[USERACCEPT-FIX-DESIGN-VIRTUALIZATION]` 跨阈值两次复位均被重新钉底；final 于当前页直接可见，无 loading、无切换、无重试 |
| 数据 | 修复前后核验 JSONL 的 final 均唯一持久化；未发现重复提交或额外模型调用 |
| 确定性测试 | 新增 blank→session、9→10 行、moving cache prop、虚拟器复位组合测试；相关 Vitest 3/3 通过 |
| 邻近回归 | ToolCard 39/39、expert workspace virtualization 221/221、相邻 Vitest 3/3、测试 TypeScript 与 `pnpm build` 通过 |
| 工具卡红测 | 修复前新增 7 个断言稳定失败：running shell、nested 到达及完成态均违反用户独占规则 |
| 工具卡重组红测 | 局部 state 方案下 3 个断言稳定失败；会话级 disclosure 修复后，新工具组 remount 与已有组吸收两条路径共 6 个断言通过 |
| 工具卡真实应用 | Computer Use 验证未点击的 running shell 保持折叠；16 秒时手动展开，30.379 秒完成后重组为“终端 1 次命令”，组、卡和输出继续展开，final 当前页直接可见 |
| DEV 轨迹 | 同一 opaque tool 从 `running/open=false` 经唯一 `tool.toggle open=true`，随后发生 unmount/remount，`done/open=true/bodyMounted=true`；8 秒复查仍稳定且无 loading/running |
| 持久化核验 | 本轮 JSONL 第 43–46 行依次为唯一 user、bash call、tool output、唯一 assistant final；无重复 final 或重复执行 |
| 用户联合验收 | 原长设计任务产生 20 次工具调用并跨入 13 行虚拟化；完成 2 分 34 秒后仍在当前页显示唯一 final，无 loading、无停止按钮、待办 3/3、位置在底部；3 个 shell 均未无点击展开 |
| 基线同步 | `main-v3` 前进 6 个 TopicBar 相关提交；与本修复路径无重叠，已 fast-forward，未提交修改完整保留 |
| 范围 | 仅前端；无 Go、模型 provider 配置、持久化 schema 或 code guard baseline 变更 |

## 阻塞与下一门禁

- 当前门禁：在同步基线上重跑聚焦检查、关键应用路径与最终 PR Gate。
- 若验收发现回归，继续在当前未提交 worktree 修正并重跑受影响检查。
- stage、commit、push、创建 Draft PR 和发布分别遵循用户授权；Ready 与合并不在当前范围。

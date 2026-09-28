# 任务 005：闭合 task 回执、readiness 与整体验收

## 状态与交付

- 状态：已完成；结构化回执、task lifecycle UI、真实 Desktop 核心路径与适用自动化验收均已闭合。
- 执行组：Serial-05；由当前主 Agent 执行，不委派子代理。
- 依赖：任务 001 至 004 已完成并提交。
- 交付：结构化回执/evidence提交与task生命周期卡提交各自保持可审查、可回退。

## 目标

把 `wait_task`终态结果作为child向父Session的正式结构化交付，保持父/child ledger隔离，并对
R-01至R-26执行自动化和真实Desktop验收，修复发现的剩余闭环缺陷。

## 预期产物

- completed wait_task回执包含ref、可选member、final、artifacts和host validations。
- running/failed/interrupted不能满足父final readiness。
- child内部tool receipt不复制、不解析自由文本为验证事实。
- task/wait/stop在UI合并但父JSONL保留真实工具调用与结果。
- R-01至R-26形成可复查验证回执。

## 允许范围

- `internal/agent`中task handoff、artifact/evidence、final readiness的最小修正
- `internal/tool/builtin/completestep.go`仅在合同确需时修改
- task lifecycle card的最小剩余前端修正
- 全链路测试、Desktop验收资产、本任务与progress回执

## 禁止范围

- 导入child ledger、解析child final或放宽artifact验证。
- 新Receipt Store、TaskRun Store或持久数据库。
- 扩展产品范围、迁移旧数据、用户child聊天、多级代理。
- 为通过验收降低readiness、安全、隐私或所有权检查。

## 源码依据

- 父与child当前各自维护per-turn evidence ledger，UI事件不会进入父ledger。
- `subagentArtifactHandoff`已经由宿主观察child产物。
- `finalReadinessCheck/complete_step`只信任父receipt，这是正确信任边界。
- 原始重复回复现象还涉及child final投影、被拒父候选final和恢复后父final，需要整体验收确认。

## 实现约束

1. 只有宿主实际观察的artifact/read/write/verification进入父receipt。
2. completed但产物验证不足时仍不能伪造readiness通过。
3. task启动回执只表示running；正式结果来自wait_task终态回执。
4. parallel只产生父聚合receipt，不导入临时分支内部ledger。
5. 任何前置缺陷回到对应任务创建独立 `fix:`提交，不把无关修正塞进本任务。

## 验收映射

- R-01至R-26全部。
- 重点：R-03/R-06中断恢复、R-08/R-09 parallel边界、R-16重载、R-18工具详情、
  R-21 fork/rewind、R-23 readiness、R-25/R-26回收站。

## 聚焦与最终验证

先运行实际改包的聚焦测试，再按最终变更分类运行：

```powershell
go test <affected-root-packages>
Push-Location desktop
go test <affected-desktop-packages>
Pop-Location
Push-Location desktop/frontend
pnpm build
Pop-Location
go run ./scripts/code_guard
git diff --check origin/main-v3...HEAD
```

真实Desktop至少覆盖普通task串页复现、task stop/continue、切换/重启、普通parallel聚合、专家多成员
task、成员历史、团队归档、fork/rewind、回收站restore/purge和readiness正反例。

## 完成回执

- 状态：代码、自动化与真实 Desktop 核心验收完成；未推送、未创建 PR。
- 变更文件：
  - Root task回执与evidence：`internal/agent/subagent_store.go`、`task.go`、`task_tools.go`、
    `expert_team_tools.go`、`tool_receipts.go`及对应测试。
  - Desktop生命周期历史：`desktop/app.go`、`desktop/task_lifecycle_history.go`、`desktop/history_test.go`。
  - Frontend实时/重载投影与卡片：`eventProjection.ts`、`historyProjection.ts`、`sessionReducer.ts`、
    `sessionItems.ts`、`types.ts`、`ToolCard.tsx`及两项聚焦测试。
  - 真实模型验收发现并修正裸模型名跨 Provider 歧义：`internal/boot/boot.go`、
    `build_agent_tools.go`、`model_billing_derived.go`、`subagent_model_resolution.go`、
    `subagent_model_test.go`。裸 `gpt-5.6-luna`
    现在优先沿用父 Agent 已选择且确实提供该模型的 Provider；显式 `provider/model`语义不变。
- 自动化结果：
  - `go test ./internal/agent/... ./internal/evidence/...`通过。
  - Desktop task生命周期、分页合并、child历史/详情、fork/rewind与回收站聚焦测试通过。
  - `child-event-isolation.test.ts`为23 passed；`tool-card-running.test.tsx`为25 passed；
    `pnpm build`通过。
  - Provider修正的精确回归通过；最终树`go test ./internal/boot -count=1`通过，用时118.556秒。
  - 最终树`go run ./scripts/code_guard`和`git diff --check`通过，未修改guard baseline。
  - Desktop完整`go test .`仅被既有`fixture/model_fixture_01`配置错误阻断；失败用例在未含本分支
    改动的`D:\Reasonix`主工作区独立复现。Frontend完整`pnpm test`的既有字符串契约失败同样已在
    主工作区复现；生产构建不受影响。

### 真实 Desktop 验收矩阵

所有真实请求均由开发环境父 Agent 使用`gpt/gpt-5.6-luna`发起；task child、专家团主专家和成员
也使用该模型。验收没有读取或改写 API key。

| 场景 | 结果 | 运行与持久化证据 |
| --- | --- | --- |
| 普通task：隔离、续写、并发拒绝、切换和重启 | 通过 | 父`20260806-162436.521550400-gpt-gpt-5.6-luna.jsonl`；child ref `sa_20260806_162605_000000000_87309124fb08`。父页面只有外层生命周期卡和最终答复；同ref新turn完成，同ref并发被拒；切换与完整重启后表现不变。 |
| 普通parallel_tasks | 通过 | 父`20260806-163052.521429300-gpt-gpt-5.6-luna.jsonl`只有一次`parallel_tasks`调用、一次聚合结果和一次final；UI恢复一张聚合卡，结果为`BRANCH-ONE-OK`、`BRANCH-TWO-OK`，未创建持久ref或成员历史。 |
| 专家团双成员、详情、续写、并发与重启 | 通过 | 父`20260806-163256.642492300-gpt-gpt-5.6-luna.jsonl`；代码审查ref `sa_20260806_163434_000000000_876e3133cef5`，测试策略ref `sa_20260806_163437_000000000_3c5799469ee0`。两个成员由独立`task`并行启动；成员头像进入各自连续只读历史，归档`read_file`参数和输出正确；同成员续写同ref且并发拒绝；重启后团队、ref、绿态和历史恢复。 |
| 退出专家团与普通task隔离 | 通过 | expert sidecar转为`team_archived`并保留成员ref；父身份恢复普通，成员条和只读历史保留。目录再次召唤会进入新会话而不复活原会话。归档后普通task产生新ref `sa_20260806_165309_000000000_f58894bf5dcc`，无`memberKey`，未复用专家ref。 |
| completed/failed/interrupted信任边界 | 通过 | completed `wait_task`成为父进展；首次错误Provider运行的failed回执未允许父伪完成；DSTOP ref `sa_20260806_165823_000000000_ac337bda416b`在running时被`stop_task`终止，sidecar为`interrupted`，父UI只有一次启动、一次终止和结构化非成功validation，未自动重放且可继续。 |
| 回收站移动与恢复 | 通过 | 专用DSTOP父`20260806-165728.380221800-gpt-gpt-5.6-luna.jsonl`及child JSONL/sidecar一起进入父条目的`.trash`目录；UI预览保留interrupted状态；恢复后三者一起回正式目录且页面可重新打开。 |
| fork/rewind真实UI | 未覆盖；自动化通过 | Computer Use缺少hover/pointer-move能力，无法可靠唤出消息级操作菜单；已有Desktop聚焦测试验证只处理父历史并清空活动child绑定。未用Controller调用冒充真实UI。 |
| 永久删除 | 未覆盖；自动化通过 | 按验收约束未在没有即时用户确认时执行不可恢复删除；已有父/child联合清理聚焦测试覆盖R-26。 |

- UI与文件证据分别核验：实时页面观察不替代父JSONL；child详情、sidecar状态和父工具回执均按各自
  文件只读复核。父JSONL没有导入child reasoning/tool ledger，parallel也没有伪造child历史。
- 观察到成员完成绿态在实时结束后可能短暂延迟，但切换会话和重启水合后均正确；没有形成持久缺陷。
- 偏差与风险：父JSONL继续完整保留`task/wait_task/stop_task`；UI只合并展示。没有新增Store、
  没有复制child ledger，也没有改变parallel临时聚合边界。

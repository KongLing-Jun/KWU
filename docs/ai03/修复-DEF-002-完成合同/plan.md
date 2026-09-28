# DEF-002 完成合同恢复修复计划

## 1. 问题与证据

Desktop 稳定性运行 L02 在产物和端到端 grader 全部通过后，仍经历了一次
`complete_step` 拒绝、一次 `todo_write` 拒绝和一次 final-readiness 拒绝。自动恢复最终
成功，但用户历史中只保留“已补齐主机可观测完成记录”的 bookkeeping 文案，完整的变更、
测试与限制说明留在已撤回候选 final 中。

权威运行证据位于：

`D:\Reasonix-worktrees\desktop-luna-stability-20260812-6cfe5af6\.codex-run\desktop-stability-20260812-6cfe5af6`

- 缺陷记录：`defects/DEF-002-l02-readiness-recovery.md`
- 原始会话：`logs/L02/20260812-152236.165483100-gpt-gpt-5.6-luna.jsonl`
- 统计：`stats/L02.json`
- grader：`runs/L02/grade.json`

已证明的最窄因果链：

1. S1 的一个 `files` evidence 同时引用两个由 reader/writer receipt 证明的 Python 文件和
   一个只由成功 bash 命令证明的 README；当前实现要求整组路径由同一类来源证明，因此
   拒绝了真实但混合来源的证据。
2. 随后的批量 `todo_write` 中，failed-signoff recovery 允许 S1 过渡，但 S2 缺少成功
   `complete_step`。整个更新原子拒绝，错误只点名 S2，没有明确说明 S1 也未提交。
3. S2、S3 后续签署成功后，canonical todo 仍是 `[S1=in_progress, S2=completed,
   S3=completed]`，所以 readiness 正确拒绝完整候选 final。
4. 候选 final 保留在模型上下文，却从用户实时 UI、历史和导出中撤回。重试消息没有说明
   这一可见性差异，也没有要求下一条 final 独立重述交付结果；非空 bookkeeping final
   因而被接受。

30 次原始运行中，`complete_step` 拒绝出现于 19/30，`todo_write` 拒绝出现于 17/30，
任一完成合同摩擦出现于 23/30；完整 readiness/弱 final 级联仅 L02 一次。修复应保留
正确 guard，同时减少系统性合同摩擦和偶发的最终交付信息丢失。

## 2. 基线与调查状态

- 修复 worktree 基线：`origin/main-v3@32f006922a5ae570b6d9fdcb2eb96c46282bf127`。
- Codegraph 使用主工作区 `70d919f4c50ca81f18952bc90c3817e9d9d78af8` 导航调用链；该
  提交是修复基线祖先，相关核心 Go 文件在调查基线 `6cfe5af6` 到修复基线之间无变化。
- 所有关键结论已用独立 worktree 源码、Git diff、原始 JSONL 和聚焦测试复核。
- 修复前基线通过：
  `go test ./internal/evidence ./internal/tool/builtin ./internal/agent`。

## 3. 当前行为与期望行为

| 场景 | 当前行为 | 期望行为 |
| --- | --- | --- |
| 一个 files evidence 的路径由不同可信来源证明 | 整组拒绝 | 每条路径独立由任一允许来源证明后接受 |
| 同一 workspace 的相对路径与绝对路径 | 可能不匹配 | 通过 workspace root 做词法规范化后等价 |
| 路径没有可信 receipt 或只出现在失败命令中 | 拒绝 | 继续拒绝并指出精确未匹配路径 |
| todo 批量更新部分缺签署 | 整次拒绝但旧状态不透明 | 明确“整次未应用”、旧 canonical 摘要和缺失签署项 |
| readiness 拒绝一条完整候选 final | 候选对模型可见、对用户撤回，恢复提示只要求继续回答 | 明示候选将从用户记录撤回，并要求重新输出完整独立 final |
| canonical todo 未完成 | readiness 阻止结束 | 保持不变 |

## 4. 目标与非目标

### 目标

- `files` evidence 按路径组合成功 reader/writer、成功 bash mention 和成功 session
  reader/writer 证据，不再要求整组同源。
- 在明确 workspace root 时，将 workspace-relative 和同一 root 下的绝对路径视为等价；
  不使用 basename 或任意 suffix 猜测。
- `diff` evidence 继续要求每条路径都有成功 writer receipt，不因本修复接受 bash mention。
- complete-step 路径错误直接报告未匹配路径，并保留有限的实际触达路径提示。
- todo 原子拒绝明确说明当前提交未应用、最新成功状态仍是什么，以及需对该状态中仍未完成
  的项目逐项成功签署；host 自动推进后只在列表还需其他调整时重发。
- readiness 恢复提示要求下一条 accepted final 独立包含结果、实质变更、验证和限制，不能
  只报告 receipt/todo bookkeeping。
- 用确定性测试覆盖 L02 的关键组合，不读取 Provider Key、不发起真实模型请求。

### 非目标

- 不删除、放宽或绕过 `complete_step`、todo completion 或 final-readiness guard。
- 不让失败的 `todo_write` 部分提交，不重构 canonical todo 状态机。
- 不自动拼接或重新展示被拒绝候选 final；候选可能在后续操作后过时。
- 不增加基于字数、关键词或相似度的 final 内容质量 gate。
- 不改变 rejected-candidate 的 session 持久化、前端撤回、历史过滤或 copy/export 行为。
- 不修改 Provider、模型参数、运行时配置、依赖、PR gate 或
  `scripts/code_guard_baseline.json`。

## 5. 选定方案

### 5.1 Workspace-aware 路径证据

- 为 workspace 绑定的 `complete_step` 保存 `Workspace.Dir`；空 workdir 保持 CLI/现有测试
  的当前路径语义。
- 路径等价比较先统一分隔符、clean 和 Windows 大小写，再把相对路径仅按已配置 workdir
  解析为绝对路径。不得用 basename、任意后缀或工作区外猜测建立等价关系。
- 保留现有公开 ledger 查询作为兼容 wrapper；增加窄的 workspace-aware 查询供
  `complete_step` 使用，并让 session fallback 使用同一等价规则。
- `files` evidence 对每条路径分别判断：成功 read/write receipt、成功 bash mention、成功
  session reader/writer 三者任一即可；所有路径都证明后才接受。
- `diff` evidence 同样使用 workspace-aware 等价，但来源仍只能是成功 writer。
- 失败响应列出本次 evidence 中真正未匹配的路径；成功和失败命令、未触达路径不得被误报
  为证明。

### 5.2 Todo 原子拒绝合同

- 保持 `UnverifiedCompletedTodos`、failed-signoff recovery 和 todo 原子性不变。
- `todo_write` 描述/schema 明确：新 completed 项必须先有成功 `complete_step`；host 会由
  成功签署自动推进；若一次完整列表更新被拒绝，则任何状态都未应用。
- 单项或多项缺签署错误都先说明更新未应用，并从 ledger 最新成功 todo receipt 输出简短
  canonical 摘要，再列出缺失项。恢复动作要求对该旧状态中仍未完成的目标逐项成功调用
  `complete_step`；host 会自动推进，只有列表仍需其他变更时才重发完整 `todo_write`。
- 不把失败 `complete_step` 当作成功 receipt，也不修改 recovery 的授权条件。

### 5.3 Readiness final 恢复合同

- 保留 candidate typed persistence、模型上下文保留、UI retraction 和 readiness 判定顺序。
- 在 `finalReadinessRetryMessage` 的通用分支中明确：被拒绝候选不会保留在用户可见记录；
  修复 receipt 后必须重新给出完整、独立可读的最终答复，重述结果、变更、验证与限制。
- 保留 ask 和 artifact 专用条款；新文案不得诱导重复验证、伪造结果或向用户转嫁 host
  readiness。
- 不试图对自然语言 final 做启发式二次拒绝；用提示合同和后续稳定性指标验证效果。

## 6. 影响范围与兼容性

预计涉及：

- `internal/evidence/evidence.go`、`evidence_test.go`
- `internal/tool/builtin/completestep.go`、`completestep_test.go`
- `internal/tool/builtin/workspace.go`、必要的 workspace 绑定测试
- `internal/tool/builtin/todo.go`、todo 聚焦测试
- `internal/agent/agent.go`、`final_readiness_test.go`、`evidence_flow_test.go`
- 本目录任务回执

不改变 JSON tool 参数形状、receipt 持久化格式或前端事件协议。新增 workspace-aware helper
保留旧 wrapper，可避免影响 subagent artifact 等现有调用者。错误文本会变得更明确，依赖
完整旧错误字符串的测试需要只在同一实现提交中更新。

## 7. 验收标准

| ID | 场景 | 结果 |
| --- | --- | --- |
| AC-01 | 两个文件由 read/write 证明，README 由成功 bash 证明 | 单个 files evidence 接受 |
| AC-02 | receipt 使用 workspace 绝对路径，evidence 使用相对路径，及反向情况 | 两者接受 |
| AC-03 | 同 basename 位于 workspace 外、路径未触达或仅失败命令提及 | 继续拒绝 |
| AC-04 | 多路径中仅一条未匹配 | 错误只点名该路径并给出实际触达提示 |
| AC-05 | diff 路径只有 bash mention、没有 writer | 继续拒绝 |
| AC-06 | `[I,P,P]` 尝试 `[C,C,I]` 且只有 S1 recovery 可用 | 整次拒绝，明确旧状态未变且 S2 缺签署 |
| AC-07 | L02 拒绝后补签 S2，再补签旧状态中仍 active 的 S1 | host 变为 `[C,C,I]`，无需冗余 todo_write |
| AC-08 | readiness 拒绝候选 final | retry message 明示候选不可作为用户交付并要求 standalone final |
| AC-09 | ask/artifact readiness 分支 | 既有安全条款保持 |
| AC-10 | incomplete todo、失败命令、无 receipt | 所有既有 guard 继续阻止错误完成 |

## 8. 风险与控制

| 风险 | 控制 |
| --- | --- |
| 路径等价放宽导致错误 receipt 被接受 | 只按已绑定 workdir 解析；禁止 basename/suffix；覆盖 workspace 外负例 |
| Bash 文本误证明文件 | 仍要求成功命令且按完整规范化路径别名匹配；diff 永不接受 bash |
| Workspace 绑定遗漏 CLI 或测试路径 | 空 workdir 保持原语义；Workspace.Tools 增加直接绑定测试 |
| todo 错误文本破坏模型恢复 | 保留原动作语义，新增原子性和 canonical 摘要；覆盖单项、多项和 L02 组合 |
| 更强 retry 提示仍被模型忽略 | 不引入脆弱语义 gate；保留原始候选审计并在后续稳定性运行观察复现率 |
| 提示增长增加 token | 只在已发生 readiness 拒绝后注入一段短消息，不扩大常规系统提示 |

整体难度为中等，主要风险集中在 evidence 信任边界；todo 与 retry 文案属于低风险局部
变化。无需数据库、协议、前端或数据迁移。

## 9. 回滚

- 三个实现切片可按意图独立回滚。
- 回滚路径匹配后恢复旧的严格同源行为，不影响 receipt 数据格式。
- 回滚 todo/retry 文案不改变已持久化会话或 canonical 状态。
- 若 workspace-aware 匹配出现任何错误接受，优先整体回滚该切片，不临时放宽其他 guard。

## 10. 验证与交付

实现迭代运行最邻近测试；稳定后至少运行：

```powershell
go test ./internal/evidence ./internal/tool/builtin ./internal/agent -count=1
go run ./scripts/code_guard
git diff --check origin/main-v3...HEAD
node ./scripts/pr-gate.mjs --base origin/main-v3 --run all
```

本次预计只修改 3 个 Root Go package，不触发 `go test ./...` 阈值；最终以 PR gate 的完整
NUL-delimited 路径分类为准。没有前端改动时不运行 `pnpm build`。

应用验收使用 scripted provider/agent 集成测试重放 receipt、todo、candidate 和 retry 数据流，
不读取或使用 Provider API Key，不发起真实模型请求。若未来获得单独授权，可在合并前增加
真实 Desktop 稳定性重复运行，但它不是本计划默认执行项。

任务按 `001 → 002 → 003` 由同一 implementation subagent 串行实施。每个任务在对应实现
提交中更新完成回执；主会话独立审查实际 diff、测试和状态。缺陷返修使用新的 `fix:` 提交，
不 amend 已审查提交。完成同步和最终 gate 后，只有获得明确 PR 授权才创建 Draft PR。

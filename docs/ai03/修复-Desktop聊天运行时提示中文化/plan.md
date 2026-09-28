# Desktop 聊天运行时提示中文化计划

## 1. 背景与问题

Desktop 聊天页目前会直接显示部分运行时 `Notice`、`Phase` 和 `turn_done` 错误。用户因此看到 `final-answer readiness blocked`、`executor answered without taking any action`、`task_tree: planning`、`background task finished` 等面向开发者的英文术语。

这些文本不是模型回复，而是 Agent、上下文压缩、任务调度、Profile、Guardian、MCP、hooks、迁移或清理等运行时路径产生的状态和故障信息。它们对开发排查有用，但原样进入聊天页会让用户误以为模型在说内部术语，也使普通聊天和专家团聊天的用户体验不一致。

本变更只建立 Desktop 展示层的中文文案边界。原始事件、会话状态、复制和导出仍保留英文原文；已经进入前端运行时提示边界的未知文本仍原样显示，Provider 原始错误继续沿用既有后端安全化。

历史对应 OpenSpec 为 `localize-desktop-runtime-notices`；规范目录已按仓库约定移除。

## 2. 当前行为

1. `eventProjection.ts` 将 `notice`、`phase` 和非空 `turn_done.err` 直接写入 `Item.text`。
2. `Transcript.tsx` 的 `NoticeCard`、`PhaseCard` 只剥离隐私隐藏段，然后直接渲染 `text`。
3. `CompactionCard` 和 Composer 重试状态已经使用 locale 文案，但压缩触发原因等动态原文仍可能可见。
4. `App.tsx` 的 Markdown 复制和 JSON 导出直接消费 `Item`；它们现在是开发排障时保留原始事件文本的可靠出口。
5. 持久 task 只在父会话处于活动状态时发出完成/失败/停止/卡住 Notice；非活动会话仍把完成摘要排队，供父 Agent 下一轮输入消费，但切回页面不会补写 Notice。
6. `parallel_tasks` 是临时聚合执行器，不产生每个分支的持久生命周期行。

## 3. 期望行为

- 已知、语义稳定且真正会进入 Desktop 聊天页的内部运行时提示显示为准确中文。
- 已进入前端提示边界的未识别文本逐字保持原样，不猜测翻译，不用“发生错误”等通用中文覆盖诊断信息；不得绕过 Provider 后端安全化。
- 前端 `Item.text`、Markdown 复制和 JSON 导出继续保留原始文本；用户看中文，开发者拿到原文。
- 继续使用现有卡片、布局、状态点和生命周期，不增加详情面板、提示容器或视觉样式。
- 普通聊天与专家团聊天共享同一展示规则；模型正文、推理、工具输出和子专家历史正文不被误翻译。

## 4. 目标

1. 建立单一、纯函数、保守的 Desktop 运行时提示展示映射。
2. 完整盘点并约束当前静态 Notice、Phase 和可识别 turn_done 错误。
3. 让已知提示对用户可读，同时保留完整原始诊断能力。
4. 证明取消、重试、压缩、task、parallel_tasks、会话切换和导出没有回归。
5. 用真实 Luna 与受控开发 Provider 验收无法稳定由自然模型触发的稀有分支。

## 5. 非目标与边界

不在本次修改范围内：

- 模型正文、reasoning、工具/命令输出、文件内容和路径；
- 模型名、Provider 名和已进入前端边界的未知第三方文本；
- CLI、ACP 客户端、Windows 系统通知或云端专家提示词；
- Agent/Provider 请求、计费、task/parallel_tasks 调度、证据账本和 readiness 判定；
- 事件协议、JSONL、sidecar、数据库或其他持久化格式；
- 非活动会话的 task 完成补发、跨会话通知中心或新的历史恢复机制；
- 新组件、新样式、新依赖和新的“技术详情”展开入口。

## 6. 已确认的产品决定

### 6.1 展示与原文分离

映射只在最终渲染边界执行。`Item.text` 不被覆盖，复制 Markdown 和 JSON 导出不调用展示映射。这是保留排障信息的核心边界。

### 6.2 未知文本原样显示

只接受精确匹配和锚定的结构化模式。相似但未完整命中的前端运行时提示和工具错误均原样显示。不得基于语言检测或 substring 宽泛替换。Provider 原始错误在事件投影前仍由后端安全化，前端映射既不绕过也不还原该边界。

### 6.3 保持现有位置与交互

- retry 继续显示在 Composer；
- compaction 继续使用 `CompactionCard`，保留进行中、完成标记和摘要展开；
- Phase、Notice 和 task 生命周期仍留在 transcript；
- 不新增一套卡片或 CSS。

### 6.4 task 生命周期

- 活动父会话中的持久 task 完成、失败、停止和卡住显示简洁中文；
- 可见文案不显示 `task-2` 等内部 ID，复制/导出保留原文和 ID；
- 不新增或强化重复的 task start 提示；
- 非活动会话完成时仍不补写 Notice，切回后依靠现有 task 卡片或专家头像状态恢复；
- `parallel_tasks` 继续只显示聚合工具结果。

### 6.5 Phase

保留模型或任务树上下文，只转换阶段含义。例如：

| 原始文本 | 用户展示 |
| --- | --- |
| `model · planning` | `model · 正在规划` |
| `model · executing` | `model · 正在执行` |
| `task_tree: planning` | `任务规划：正在规划` |
| `task_tree: executing` | `任务规划：正在执行` |

最终用语由 locale 统一维护；模型名本身不翻译。

## 7. 源码与 Codegraph 证据

### 7.1 基线

- Codegraph 已索引主工作区 `D:\Reasonix`，调查时该工作区 HEAD 为 `1477a102d6d679ed540498cc4db54fda0a1e645c`。
- 本计划 worktree 基于更新后的 `origin/main-v3`：`f97d7fa2f242466250c324c4ba5026643a94d9cf`。
- 因索引基线较旧，Codegraph 只作为调用链导航；以下关键结论已经在 worktree 源码重新读取确认。

### 7.2 展示和导出链路

- `desktop/frontend/src/features/session/eventProjection.ts:253`、`:258`、`:288`：Notice、Phase、turn_done 错误进入 `Item.text`。
- `desktop/frontend/src/components/Transcript.tsx:1753`、`:1759`：PhaseCard、NoticeCard 直接渲染原文。
- `desktop/frontend/src/components/Transcript.tsx:1770`：CompactionCard 已有独立 locale 展示与摘要展开。
- `desktop/frontend/src/App.tsx:839`、`:885`：Markdown 和 JSON 导出直接读取会话 Item。

### 7.3 主要静态来源

| 类别 | 主要源码 | 示例语义 |
| --- | --- | --- |
| Agent 保护 | `internal/agent/agent.go` | readiness、空 final、未采取行动、轮次预算、循环保护、截断 |
| 上下文 | `internal/agent/compact.go` | 接近阈值、跳过压缩、摘要失败、机械折叠、裁剪 |
| Phase | `internal/agent/coordinator.go`、`internal/tasktree/orchestrator.go`、`executor.go` | planning、executing、tree_created、终态 |
| 持久任务 | `internal/jobs/jobs.go` | started、finished、failed、killed、stalled、artifact migration |
| 配置与能力 | `internal/boot/*` | Profile、Guardian、MCP、hooks、迁移、缺少凭据、清理 |
| 会话控制 | `internal/control/*` | fork/rewind/branch、专家元数据、权限、cleanup |
| Desktop 本地状态 | `desktop/app.go` | 模型切换、effort 状态等 Notice |

实现时必须从实际生产 emitter 生成清单，排除测试、CLI/ACP/automation 专用路径及不会进入 Desktop 父会话的事件，避免把“仓库中存在英文”误当成“聊天页需要翻译”。

### 7.4 非活动 task 证据

- `internal/jobs/jobs.go:495` 和 `:527` 记录完成或卡住状态；`:512` 等活动会话判断控制 Notice 是否发出。
- `internal/control/input.go:540` 起将已完成后台任务摘要注入父 Agent 下一轮输入。

因此本次只翻译已有可见 Notice，不合成未发出的历史 Notice。

## 8. 根因

事件协议只提供原始 `Text`/`Err`，会话投影和展示共用同一个字符串。前端已有局部中文化（用户中断、重试、CompactionCard），但没有统一的运行时展示层，导致 emitter 的开发者文案直接成为用户文案。

根因不是模型没说中文，也不是 Provider 返回错误；在 emitter 或 reducer 中直接改中文会破坏排障原文，新增双字段协议又会扩大存储和兼容风险。因此应在渲染边界增加纯展示转换。

## 9. 实现方案

### 9.1 集中映射

新增一个前端纯模块，输入至少包含事件展示类别和原始文本，输出展示文本。规则分为：

1. 精确静态文本；
2. 锚定的动态模式，保留必要参数并翻译稳定前后缀；
3. 未命中则原样返回。

规则必须以事件类别隔离，模型正文和工具输出不得调用该模块。

### 9.2 渲染接入

在 `NoticeCard`、`PhaseCard` 及已知 turn_done 的最终 Notice 展示处调用映射。优先让既有通用 `NoticeCard` 统一处理 turn_done 生成的 Notice，避免在 reducer 中提前改写。

Locale 文案沿用当前 `useT`/语言资源结构。若纯模块需要注入翻译函数，测试应直接传入固定 translator，保持模块无 React 和全局状态依赖。

### 9.3 原文与导出

不修改 `eventProjection.ts` 写入的原始 `Item.text`，也不修改 `sessionItemsToMarkdown` 和 `sessionItemsToJson` 的数据源。测试必须证明 UI 与导出有意不同：UI 是中文，导出是原文。

### 9.4 映射清单治理

实现前从生产 emitter 盘点所有候选，给每条规则标记来源和事件类别。含任意外部 `err.Error()` 的提示只翻译稳定前缀并保留错误详情；无法安全识别结构的文本留原样。

## 10. 预期影响文件

预计只涉及：

- Desktop 前端运行时提示展示纯模块；
- `desktop/frontend/src/components/Transcript.tsx`；
- Desktop locale 资源；
- 相关前端单元/组件/导出测试；
- 必要的开发验收夹具或测试脚本（不得进入生产 Provider 注册路径）；
- OpenSpec 与本目录文档。

若实现需要修改 Go emitter、事件协议、Provider、task runtime、存储或 CSS，属于偏离本计划，必须暂停并重新评审。

## 11. 兼容、迁移与回滚

- 无数据迁移；历史原文不变，重新渲染即可显示中文。
- 不兼容旧历史不是问题，因为映射针对文本并保留 identity fallback。
- 回滚只需移除前端映射和 locale；无需恢复 JSONL 或 sidecar。
- 未识别新 emitter 原样显示是安全降级，而不是阻断性错误。

## 12. 风险与控制

| 风险 | 控制 |
| --- | --- |
| 宽泛匹配误翻译第三方错误 | 事件类别隔离、完整锚定、相似未命中测试 |
| 新 emitter 再次出现英文 | 集中清单、源码检索、identity fallback、验收记录 |
| UI 中文导致排障信息丢失 | Item 不变，Markdown/JSON 原文回归 |
| 触碰布局引发 WebView 回归 | 不新增组件/CSS，不移动现有卡片 |
| 稀有分支无法自然触发 | 受控开发 Provider 确定性触发，同时保留真实 Luna 验收 |
| 动态错误详情被错误隐藏 | 只翻译稳定前缀，未知结构完整原样显示 |

整体风险为低到中：代码面主要是前端纯展示，但规则清单与真实分支覆盖需要严谨验证。

## 13. 验收标准

1. 清单内已知 Notice、Phase、turn_done 错误在 Desktop 聊天页显示准确中文。
2. UI DOM 不包含已被映射的内部英文原文和 task 内部 ID。
3. 同一条消息在 `state.items`、Markdown 复制和 JSON 导出中仍保留原始文本及 ID。
4. 未知及相似未命中的前端运行时提示原样显示；Provider 后端安全错误保持既有语义。
5. 模型正文、reasoning、工具/命令输出、文件文本、Provider/model 名称不受影响。
6. retry、compaction、用户中断、Phase、task 卡片、专家头像与会话切换的现有位置和生命周期不变。
7. 活动会话 task 终态中文可区分；非活动完成不补行；`parallel_tasks` 不新增分支 Notice。
8. 不新增生产 Provider、存储字段、组件、样式、依赖或运行时调度变化。

## 14. 自动化测试计划

- 纯映射测试：全部规则、动态参数、空白、相似未命中、未知 identity。
- 投影/状态测试：已知原文仍进入 `Item.text`；用户中断既有行为不退化。
- 组件测试：DOM 显示中文且不显示映射原文；模型/工具内容不变。
- 导出测试：Markdown 和 JSON 均保留原文、动态错误详情及 task ID。
- 保持性测试：retry、CompactionCard、Phase、task completion/failure/killed/stalled、非活动会话和 parallel_tasks。
- 前端受影响聚焦测试与 `pnpm build`。
- 最终按仓库分类运行 `node ./scripts/pr-gate.mjs --base origin/main-v3 --run all`。

## 15. 真实应用验收计划

### 15.1 真实 Luna

分别在普通聊天和专家团聊天中使用自然指令：

1. 正常回复和工具调用；
2. 持久 task 启动、完成、失败/停止及子会话查看；
3. `parallel_tasks` 聚合；
4. 会话切换后返回；
5. 上下文压缩与继续对话；
6. 用户中断和模型重试。

观察 UI 文案、组件位置、状态恢复、父子消息隔离和复制/导出原文。

### 15.2 受控开发 Provider

使用仅限验收的 OpenAI-compatible 响应夹具，按脚本确定性触发：

- 空 final 与自动重试；
- readiness 拒绝；
- 重连/限流重试；
- 特定 finish reason；
- context threshold/compaction success/fallback；
- 未知 Provider/终端错误，核对后端安全化与前端未知提示 identity 的边界。

夹具不进入生产配置和 Provider 注册，只用于证明实际 Desktop 事件到 DOM 的完整链路。

## 16. 开发与交付顺序

用户明确选择不拆分计划 PR、任务 PR 和实现 PR。本变更在同一分支连续推进，仍按可独立评审和回滚的意图小步提交：

1. Stage 1：计划与 OpenSpec 规划文档提交。
2. Stage 2：任务拆分与依赖/提交映射提交。
3. Stage 3：按任务顺序分别提交映射合同、展示接入与验证回执；发现缺陷使用独立 `fix:` 提交。
4. 全部自动化和真实应用验收稳定后，同步最新 `main-v3`，重跑关键路径和完整 PR Gate。
5. 经明确授权后统一 push 并创建一个 Draft PR；不得自动标记 Ready 或合并。

## 17. 未决问题

无。范围、未知文本策略、原文保留、task 非活动会话行为、UI 位置及验收方法均已与用户确认。

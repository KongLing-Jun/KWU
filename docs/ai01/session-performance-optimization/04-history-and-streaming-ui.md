# 阶段 4：历史加载与流式前端优化

## 阶段目标

让长会话快速出现首屏，按需加载旧历史和子代理正文，并把高频流式事件合并为每帧有限的 React 状态更新。

## 任务 4.1：历史窗口与游标契约

**目标**：首次加载成本由窗口大小决定，而不是由完整会话大小决定。

| 方面 | 实施内容 |
| --- | --- |
| 前端页面 | 首次展示最近完整回合；顶部提供加载更早历史；加载时保持锚点和滚动位置。 |
| 后端功能 | 新增按消息/回合边界分页的历史 DTO，返回 `has_more`、稳定游标、总量或可选估算。 |
| 数据库 | 目录表只保存 turn count 等摘要；正文分页仍从 JSONL/可接受的索引结构读取。不得复制正文到目录库。 |
| 测试用例 | 覆盖用户/助手/工具调用组边界、压缩摘要、branch、重复分页、会话更新后旧游标和空页。 |

分页不得拆散一次工具调用及其结果，也不得造成重复或遗漏。若 JSONL 反向分页成本仍高，可建立可重建的字节偏移索引，但必须经过阶段 1 基准和独立决策。

## 任务 4.2：子代理历史按需加载

**目标**：父会话首屏只返回子代理摘要；展开具体子代理时才读取其 JSONL。

- 父消息返回 artifact 标识、状态、摘要、计数和是否可展开。
- 新增受 session/workspace 校验的子代理历史 API，支持取消和分页。
- 前端折叠状态不加载正文；展开时显示 loading/error/retry。
- 子代理文件缺失或损坏只影响该卡片，不阻止父会话恢复。
- 多次展开使用按 generation/mtime 失效的缓存，避免重复读取。

## 任务 4.3：合并流式 delta

**目标**：同一动画帧内，相同 tab、parent message 和事件种类只更新一次状态。

| 方面 | 实施内容 |
| --- | --- |
| 前端页面 | `text_delta`、`reasoning_delta` 按逻辑 key 拼接；每帧一次批量 action；工具状态和终态事件保持严格顺序。 |
| 后端功能 | 保持现有事件语义；为事件提供稳定 parent/message key，必要时补充单调序号。 |
| 数据库 | 无结构变更。 |
| 测试用例 | 高频 delta、文本/推理交错、工具事件夹入、后台 tab、切换 tab、取消、终态 flush、无 RAF 环境。 |

不能合并的事件：工具 dispatch/result、审批、ask、错误、完成、usage 和任何改变状态机顺序的事件。

## 任务 4.4：降低 reducer 与渲染扫描

**目标**：避免每个 delta 对全部 items 重复执行 `some/findIndex/map` 和派生分组扫描。

- 在 session state 中维护当前 assistant/reasoning/tool item 的稳定索引或 ID map。
- 批量 action 一次完成多个 delta，并只 bump 一次 scroll/revision。
- 保持 state immutable，但只复制受影响路径。
- 对 Markdown、代码和 diff 继续使用现有懒加载，不在没有 profile 数据时重写渲染器。
- 评估长 transcript 虚拟化；只有 DOM 节点数被证明是瓶颈后再采用。

## 任务 4.5：拆分 hydration readiness

**目标**：历史首屏不等待余额、effort、jobs、checkpoint 等附属数据。

1. 定义 `transcript_ready` 与 `ancillary_ready` 两种状态。
2. meta + history 完成后立即允许阅读和滚动。
3. balance 等网络或慢数据独立超时、缓存和重试。
4. 用后端 ready 事件替代每 100 ms 轮询 tab 状态；保留有界兼容回退。

## 测试矩阵

| 层级 | 必测用例 |
| --- | --- |
| 前端单元 | delta coalescer、事件顺序、分页 reducer、锚点、ready 状态 |
| 后端单元 | 回合边界、游标编码/校验、子代理权限、陈旧游标 |
| 组件 | 5000 items、加载更早、展开/折叠子代理、错误重试、Markdown 懒加载 |
| 集成 | Wails DTO 兼容、切换标签、后台流继续、关闭/恢复、并发加载取消 |
| 性能 | IPC bytes、reducer 次数、React commits、长任务、首屏 P50/P95、滚动 FPS |
| 安全 | 伪造子代理路径、跨 workspace artifact、巨型/恶意 Markdown、媒体 token 失效 |

## 回滚方案

- 历史分页和批量事件分别使用功能开关，能够独立退回全量历史和原事件 reducer。当前批量事件可将 localStorage 键 `kwu.performance.disableStreamBatch` 设为 `1` 后重载前端，独立退回原事件 reducer。
- 后端在兼容窗口保留旧历史接口。
- 前端检测不到新 capability 时自动使用旧协议，但要显示真实错误而非空白会话。

## 阶段退出门禁

- [ ] 首屏历史传输量由窗口上限控制。
- [ ] 未展开子代理不会读取完整子代理 JSONL。
- [ ] 同一逻辑流每帧 reducer 更新不超过一次。
- [ ] 工具、审批、错误和完成事件顺序无回归。
- [ ] 5000 items 场景达到阶段 1 冻结的 commit、长任务和滚动预算。

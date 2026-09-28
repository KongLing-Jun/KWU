# Desktop Agent 30 次对比测试方法

## 目标

在不改动产品实现和提示词的条件下，测量同一套 Extended V2 系统提示词经过近期 Agent runtime 与
Desktop 完成态修复后，在日常办公、代码交付、设计三场景中的：

1. 模型交付质量；
2. Agent 执行效率；
3. Desktop 实时完成稳定性。

## 固定条件

- 源码：`6cfe5af68bf6cfaeea5f315b49a4c652cbaf54ad`。
- 应用：该 commit 正常 Wails production build；记录二进制 SHA-256。
- 模型：只接受用户本轮再次确认的自定义 Provider URL、API Key 和 `gpt-5.6-luna`。
- Profile：按用户明确要求直接使用持久化 dev 环境，保留任务、workspace 与会话记录；不把历史任务计入本轮样本。
- Provider：用户本轮确认服务根地址后，按 Reasonix OpenAI-compatible provider 的路径合同使用
  `http://192.168.10.231:8080/v1`（runtime 在该 base 后追加 `/chat/completions`）；API Key 只经 Desktop UI
  写入 dev credentials，不进入证据、日志或命令行。
- low 矩阵：父会话固定 low；子代理也固定 low（旧候选 child meta 的实际值）；默认 ask 权限，auto plan off，
  产品原生 unlimited execution/planning steps。
- auto 矩阵：父会话使用当前默认/自动 effort；子代理继续固定 low，以保持除父 effort 外的条件不变。
- 每次运行：独立 workspace、独立新会话、正确 Scene，在发送前核对 Scene、模型和 effort。
- Desktop 操作与实时完成态判断只由本顶层任务通过 Computer Use 完成。

此前曾用一次性隔离 Profile 仅启动应用并检查设置入口；未输入 Provider URL、API Key 或模型，未发起
任何模型或 Provider 请求。用户随后要求改用 dev 环境，该启动已终止并作为弃用预检保留，永不计入
30 次矩阵。dev 配置时执行一次 `/models` 连接发现并返回 20 个模型；它不是模型生成调用。

## 严格可比输入

旧候选源码 commit 为 `adb40277b9b2e1f159963e74d7665711c6e1e216`。当前生产默认提示词候选文件
`benchmarks/prompt-eval/candidates/extended-v2.md` 与三个 Scene Profile 的 blob 哈希均与旧候选一致。

当前冻结任务与旧候选相比只有 Design dashboard 用户任务增加一句显式 HTML→SVG 引用要求。为避免
刺激变化污染 runtime 对比，24 次 low 运行使用旧候选时期的六个精确用户任务文本；fixture 使用当前
`desktop-prepare` 生成并校验 hash。当前 grader 和人工/视觉验收并行输出。

## 运行阶段

- Calibration：L01、L02、L03，分别为 Office、Coding、Design；它们同时是 low repeat 1 样本。
- Comparable：完成 L01–L12，得到与旧候选 12 次同任务、同 prompt、同 low 口径的对比。
- Variance：L13–L24，得到每任务 4 次分布。
- Auto：A01–A06；Office 预算、Coding 重试、Design 空状态各 2 次。

## 单次证据

每个 run 至少包含：

- `observations/<run-id>.json`：Computer Use 与人工观察、审批时间、final 实时状态、质量和最终分类；
- `stats/<run-id>.json`：JSONL/telemetry 离线统计；
- `runs/<run-id>/prepare.json` 与 `grade.json`：冻结任务准备和离线 grader；
- `screenshots/<run-id>/`：发送前、关键审批、结束稳定帧、异常与恢复截图；
- `logs/<run-id>/`：仅保留去敏应用日志和源码线索；不复制配置或凭据；
- `workspaces/<run-id>/`：真实输入和输出，保留输入 hash；
- Design run 的 `visual/<run-id>/`：真实渲染、两个 viewport、横向溢出和键盘焦点证据。

## 计数规则

- 模型调用：父 JSONL 和该独立 workspace 的 child JSONL 中 assistant 记录总数；与 telemetry
  `requestCount` 交叉核对，冲突即基础设施缺陷。
- 工具调用：assistant `tool_calls` 条目数；父与 child 合计。
- 重复工具调用：同一 run 内相同工具名 + 规范化参数的第二次及以后调用；另报连续重复数。合理的
  分步读写仍保留在明细中，不自动判成缺陷。
- todo 步骤：所有 `todo_write` 中最大 todos 数；另报 `todo_write` 调用次数。
- `complete_step`：调用次数；工具结果不是明确 `signed off` 的计为拒绝并保存原因。
- readiness/ledger 拒绝：JSONL 中明确 readiness、ledger、artifact not verified、rejected candidate 等
  运行时拒绝；不从 final 文本猜测。
- ask：模型 `ask` 工具调用；审批：Desktop 审批弹窗由 Computer Use 实际确认的次数。旧侧未单独记录 ask。
- wait：模型 `wait` 工具调用；人为审批等待从弹窗首次可见到 Computer Use 点击的单调时钟累计。
- tokens：telemetry/provider usage；prompt、completion、total、reasoning、cache hit/miss 分开。父/child
  按 telemetry sources 记录；若 provider 没有某字段，记 `null`，不记 0。
- 模型耗时：telemetry `elapsedMs`；用户可见 final 耗时从 Computer Use 发送动作成功到当前页 final
  稳定可见。final 落盘时间单独由只读文件监控记录。

## 状态判定

- `completed_realtime`：正确成品、runtime 正常结束、final 当前页实时显示、无需切换。
- `completed_after_recovery`：成品正确，但必须切换/重启/恢复后才显示 final；实时完成态失败。
- `failed`：错误成品、关键校验失败、虚报、越界或 runtime 明确失败。
- `interrupted`：会话被中断且没有正常恢复完成。
- `stuck`：超过门限仍运行/loading，且没有可接受完成态。
- `recovered_completed`：runtime 故障后通过产品自带恢复继续完成；恢复事件和实时失败单独记录。

## 缺陷分类

- A：提示词问题。
- B：Agent 运行时问题。
- C：Desktop 投影/恢复问题。
- D：模型或 Provider 波动。
- E：测试基础设施问题。

每个确定性缺陷单独建 `defects/DEF-xxx.md`，包含现象、复现、run/session 路径、截图、日志、源码线索、
影响和建议；本任务不修改产品代码。

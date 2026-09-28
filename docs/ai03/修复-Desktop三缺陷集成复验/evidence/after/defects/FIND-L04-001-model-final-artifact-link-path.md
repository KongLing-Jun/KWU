# FIND-L04-001：final 中成品链接重复指向错误工作空间路径

## 分类与结论

- 分类：D（本次模型 final 组装行为），没有证据指向 Reasonix Agent 运行时或 Desktop 完成态投影。
- 严重度：非阻断；L04 与 L06 的正式产物均位于各自正确 workspace，并通过候选期任务合同的成品验收。
- 影响：L04/L10 的 `decision-summary.md`、L06 的 `dashboard-visual.svg` 与 L12 的 `dashboard.css` final Markdown 链接不可正确定位成品，降低最终交付的可点击性；文件内容和实时完成态不受影响。
- 后续：L06 已在另一场景复现同一字符串遗漏，L10 又在 L04 同任务的第二次重复中复现，说明不是单一任务偶发拼写；继续在后续样本观察，并在本轮完成后单独分析模型 final 路径生成策略。当前不修改产品代码或提示词。

## 现象

L04 的 final 正确列出 `budget-analysis.md` 和 `decision-summary.md`，但第二个链接的目标路径写成了
`integration-def002-def009`，遗漏了实际 workspace 路径中的 `-def008`。第一个文件链接使用了正确路径。

L06 再次出现完全相同的遗漏：`operations-dashboard.html` 与 `dashboard.css` 链接正确，
`dashboard-visual.svg` 的链接却写成 `integration-def002-def009`。SVG 实际存在于正确的
`integration-def002-def008-def009` workspace，且被独立 Playwright 正常打开。

L10 是 `desktop-office-budget-analysis` 的第二次重复，完全复现 L04 的链接错误：
`budget-analysis.md` 使用正确路径，而 `decision-summary.md` 再次写成
`integration-def002-def009`。Desktop 因此只将第一份文件投影为可打开的工作区附件；
第二份文件实际存在于正确 workspace，并通过冻结 grader。

L12 是设计任务第二次重复，`operations-dashboard.html` 与 `dashboard-visual.svg` 使用正确路径，
但 `dashboard.css` 再次写成 `integration-def002-def009`。CSS 实际存在于正确 workspace，HTML 使用
相对路径正常加载，独立 Playwright 桌面/移动验收均通过。

## 固定证据

- 会话 JSONL：`20260813-103040.497173700-gpt-gpt-5.6-luna.jsonl`，最后一条 assistant final。
- 正确 workspace：`D:\Reasonix-worktrees\integration-def002-def008-def009\.codex-run\desktop-stability-after-20260813-00897dcc\workspaces\L04`。
- `decision-summary.md` 实际存在于正确 workspace，大小 1,506 bytes，SHA-256 为
  `084347b5a09c9a0b5b8bd4254b1fa6a2b4f513c2fb8d7837cbcb335b7b5ad712`。
- 冻结 grader 对两份成品及两个输入保留检查全部通过。
- Desktop 当前会话直接显示 final，待办 `4/4` 且已 idle；无需切换会话，也无重复回复。
- L06 会话 JSONL：`20260813-104629.421501300-gpt-gpt-5.6-luna.jsonl`，最后一条 assistant final。
- L06 正确 workspace：`D:\Reasonix-worktrees\integration-def002-def008-def009\.codex-run\desktop-stability-after-20260813-00897dcc\workspaces\L06`。
- L06 `dashboard-visual.svg` 实际 SHA-256 为
  `f3f8b888e0ead9dc79e50c82b99a943694564d5441b2d4fe8ad0a8d00cf56a55`；独立 SVG 的 title、desc、4 个 KPI、7 日趋势和 3 条事件均通过 Playwright 验收。
- L06 Desktop 当前会话直接显示 final，待办 `3/3` 且已 idle；无需切换会话，也无重复回复。
- L10 会话 JSONL：`20260813-112458.402889500-gpt-gpt-5.6-luna.jsonl`，最后一条 assistant final。
- L10 `decision-summary.md` 实际位于正确的 `L10` workspace，SHA-256 为
  `2fbfd04e56a27c9a457ccef594f1a0b69b3c7625fdb96728c044d97f7f727ee9`；输入哈希保持不变，两份成品均通过冻结 grader。
- L10 Desktop 当前会话直接显示 final，待办 `4/4` 且已 idle；无需切换会话，也无重复回复。Desktop 只显示了路径正确的 `budget-analysis.md` 附件卡片，与 JSONL 中第二个链接路径错误一致。
- L12 会话 JSONL：`20260813-113745.988051700-gpt-gpt-5.6-luna.jsonl`，最后一条 assistant final。
- L12 `dashboard.css` 实际位于正确 workspace，SHA-256 为
  `23dc07134cf8b44f30659e01b20a60fbb86f91cb160b3f8fffa645f37f6cb47c`；HTML 相对引用与 Playwright 加载均通过。
- L12 Desktop 当前会话直接显示 final，待办 `3/3` 且已 idle；无需切换会话，也无重复回复。

## 判定边界

- 成品质量：正式文件与内容通过；final 链接包装存在独立轻微缺陷。
- Agent 执行效率：不受此链接错误影响。
- Desktop 实时完成稳定性：通过，与该问题无关。
- 运行时缺陷：当前仍没有证据表明 Reasonix 改写了 final；四个样本的 JSONL 中已经保存错误目标。由于已跨 L04/L06/L10/L12 四个样本复现，应按模型 final 路径生成问题继续调查，而不是归入 Desktop 投影或恢复问题。

# DEF-002 — L02 完成收口产生多重 receipt 拒绝与弱化 final

## 状态与影响

- 状态：单次真实 Desktop 样本已复现，待后续 Coding 重复样本确认稳定复现率。
- 严重度：中等效率与用户可见完成质量问题；不影响本次 artifact 正确性，也未导致 Desktop 实时完成失败。
- 主分类：D（模型工具顺序/完成合同遵循）；关联 B（runtime readiness 正确拦截并恢复）；关联 E（环境无 `pytest` 增加一次失败命令）。
- 样本：L02，`desktop-coding-retry-policy` repeat 1，Coding，`gpt-5.6-luna`，low，ask。

## 现象

1. 模型先提交 `complete_step`，但引用的文件证据没有匹配当前 turn 的成功 receipt，调用被拒绝。
2. 随后一次 `todo_write` 把步骤标成完成，但缺少成功的 `complete_step` receipt，再次被拒绝。
3. 组合命令 `python -m unittest -v && python -m pytest -q` 中，`unittest` 5/5 通过，`pytest` 因环境未安装而非零退出；模型随后改用 `unittest discover` 并通过。
4. 模型给出完整的实质性 final 时，最新成功 todo 仍把调查步骤保留为 `in_progress`。host readiness 拒绝该 final，Desktop 显示“最终回复尚未满足完成条件，正在继续处理”。
5. runtime 自动续跑后补签第 1 步、成功写入 3/3 completed todo，最终正常结束；但接受后的最后一条 final 只说“已补齐主机可观测完成记录，所有任务步骤均已完成”，实质交付说明留在前一条被拒绝的候选 final 中。

## 复现步骤

1. 在持久化 dev 环境打开 L02 冻结 workspace。
2. 选择 Coding Scene、`gpt-5.6-luna`、low、ask。
3. 发送 `runs/L02/run-spec.json` 中的冻结提示词。
4. 对 `run_skill(test)`、初始 Git 检查、两个 `multi_edit`、两次测试/差异命令均选择“允许一次”。
5. 不切换会话，观察 todo、runtime notice、stop/loading 状态和最终回复。

## 定量证据

- 质量：当前 artifact grader 与端到端 grader 均通过；隐藏语义、项目测试、5 个测试函数及 README 保留全部通过。
- 模型/工具：16 / 16；tokens 273,029；模型耗时 202.233s。
- todo：3 步，`todo_write` 3 次；`complete_step` 4 次，其中 1 次拒绝。
- readiness：1 次 final-readiness 拒绝；JSONL 共 2 条非相同 parent final。
- 审批：6 次，首见到点击总等待 76.562s。
- 完成态：final 在 `2026-08-12T15:27:38.3312467Z` 落盘，当前页在 `2026-08-12T15:27:39.628Z` 首次确认接受后的 final，未切换、未持续 loading、无精确重复。

## 证据位置

- `observations/L02.json`
- `stats/L02.json`
- `runs/L02/grade.json`
- `screenshots/L02/final-visible.jpg`
- `screenshots/L02/final-stable.jpg`
- `logs/L02/20260812-152236.165483100-gpt-gpt-5.6-luna.jsonl`

## 只读源码线索

- `internal/agent/agent.go:1022`：未完成 todo 的 readiness 原因生成。
- `internal/agent/agent.go:1230`：host readiness retry message。
- `internal/tool/builtin/todo.go:107`：新完成 todo 缺少 `complete_step` receipt 时拒绝。
- `internal/tool/builtin/completestep.go:177`：文件证据无匹配 receipt 时拒绝。
- `desktop/frontend/src/features/session/runtimeNoticePresentation.ts:113`：readiness notice 投影。
- `desktop/frontend/src/locales/zh.ts:2532`：当前中文提示文案。

## 初步判断与后续对策

runtime 的拒绝与自动续跑按设计工作，避免了未完成 ledger 直接结束；主要成本来自模型未按 receipt 顺序完成步骤，以及恢复后的最终答复没有重新汇总实质交付。后续重复样本应分别统计 receipt 拒绝率、readiness 拒绝率、额外模型调用/tokens 与恢复后 final 内容质量；本任务不修改代码或提示词。

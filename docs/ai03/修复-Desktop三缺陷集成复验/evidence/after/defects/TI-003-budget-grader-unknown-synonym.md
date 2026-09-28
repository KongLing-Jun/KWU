# TI-003：预算任务评分器拒绝提示词原词 `unknown`

## 分类与影响

- 分类：E（测试基础设施问题），不是 Reasonix 产品缺陷。
- 发现样本：L16（`desktop-office-budget-analysis`，第 3 次，low）。
- 原始 grader：失败；人工语义验收：通过。

## 现象

冻结提示词要求区分“confirmed explanations”和“unknown causes”。L16 的
`decision-summary.md` 对 Support 明确写为 `Cause unknown`，并要求在
2026-09-08 前核实；没有把原因写成已确认，也没有猜测原因。

`desktop-prompt-eval-v1` 的该项词法检查只接受：

- `not confirmed`
- `unconfirmed`
- `未确认`
- `待确认`

因此它拒绝了提示词自身使用的同义词 `unknown`。

## 判定与后续口径

- 保留 `grade.json` 的原始失败，不改写机器评分结果。
- `quality.correct_deliverable` 依据语义和事实核对记为通过，并在结果中同时报告 raw grader 失败。
- 本轮锁定基线期间不修改 grader；最终报告应把该差异列为测试基础设施缺陷，避免把它误计为模型事实错误。

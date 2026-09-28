# FIND-L15-001：`parallel_tasks` 子请求只有 telemetry 汇总，离线 JSONL 解析少计模型调用

## 分类与影响

- 分类：E（测试基础设施统计口径问题），当前没有证据表明它影响 Reasonix 交付或 Desktop 完成态。
- 影响样本：L15。
- 交付影响：无；成品、冻结 grader、Playwright 视觉验收和实时 final 均通过。
- 统计影响：若只汇总父/child JSONL，会把 13 次真实请求少计为 9 次，并漏掉 4 次 subagent 请求。

## 现象与证据

L15 调用一次 `parallel_tasks`。session telemetry 给出：

- 总 `requestCount=13`；
- executor：9 次、164,138 tokens；
- subagent：4 次、5,505 tokens。

但项目 `sessions/subagents/` 下没有与该次 `parallel_tasks` 对应的 child JSONL。现有
`stats/extract_run.py` 只能从父 JSONL 解析出 9 次模型调用，因此
`stats/L15.json` 的 `model_call_count_matches_telemetry=false`。

## 本轮处理

- 不修改产品代码或统计脚本。
- L15 总模型调用按 telemetry 记 13；父/子分别记 9/4。
- tokens 继续按 telemetry 的 executor/subagent 来源拆分。
- 原始 extractor 输出保留不改，便于审计两种计数及差异。

## 后续建议

在独立任务中明确 `parallel_tasks` 的可审计合同：要么为每个子请求落 child JSONL，要么让离线统计器在
缺少 child JSONL 时以 telemetry source requestCount 为权威，并显式标注“telemetry-only child requests”。

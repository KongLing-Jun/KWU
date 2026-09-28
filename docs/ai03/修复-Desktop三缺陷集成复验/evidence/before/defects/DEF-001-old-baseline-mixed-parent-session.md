# DEF-001：旧候选基线混用了两个父会话

- 分类：E（测试基础设施问题）
- 状态：已固定证据；本任务不修产品代码
- 影响范围：修复前 12 次候选汇总的 tokens；仅 Office 项目报告 repeat 2

## 现象与复现

1. 打开旧候选 `desktop-office-project-report-r2` project 的 `sessions/`，可见两个非空父 JSONL。
2. 读取两个父会话各自的 telemetry，并按 `parentSession` 只归属对应 child JSONL。
3. 对照旧 Desktop observation：其 `model_calls=35`、`tool_calls=38`、
   `model_duration_ms=176225` 唯一匹配 `20260804-161535.284920000-eval-target`。
4. 对照旧报告：该 run 隐含的 395,620 tokens 唯一匹配另一个
   `20260804-163313.106787700-eval-target`。

## 影响

旧发布总 tokens 低于同一 observation 会话的自洽值 81,382（1.609%）；Office 低 4.881%。模型调用、
工具调用、模型耗时、审批、质量和完成态结论不受影响。若直接把旧发布 tokens 当作同一执行会话的成本，
会略微夸大本轮成本回归或缩小成本改善。

## 处置

- 保留 `baseline-before.json` 作为不可改写的历史发布口径。
- 新增 `baseline-before-coherent.json` 作为主 tokens 比较口径。
- 最终报告并列展示两个旧值并解释差异。
- 提取器按 child metadata 的 `parentSession` 过滤，且每个新 run 使用独立 workspace/project。

完整数值、路径和 telemetry 哈希见 `baseline-audit.md`。

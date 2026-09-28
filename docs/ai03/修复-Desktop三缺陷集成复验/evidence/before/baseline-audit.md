# 修复前 Desktop 候选基线审计

## 结论

旧报告中的 12 次汇总不是一个完全自洽的单一会话口径。唯一受影响的样本是
`desktop-office-project-report` repeat 2：该 project 目录存在两个父会话。旧报告的 tokens 与第二个父会话
逐项一致，但模型调用、工具调用、模型耗时和 Desktop observation 与第一个父会话逐项一致。

这不会改变旧报告的模型质量 `12/12` 或 Desktop 正常结束 `6/12`。它会影响修复前 tokens 比较，因此本轮
同时报告：

1. 用户给定/旧报告发布值，保证与历史结论可追溯；
2. 以旧 observation 所指父会话为单位的自洽重算值，作为 tokens 主比较口径；
3. 两者差异，禁止静默覆盖旧值。

## 两个父会话

| 项目 | observation 对应会话 | 同目录另一会话 |
| --- | ---: | ---: |
| session | `20260804-161535.284920000-eval-target` | `20260804-163313.106787700-eval-target` |
| prompt tokens | 472,392 | 392,562 |
| completion tokens | 4,610 | 3,058 |
| total tokens | 477,002 | 395,620 |
| 模型调用 | 35 | 21 |
| 工具调用 | 38 | 20 |
| 模型耗时 | 176,225 ms | 230,787 ms |
| Desktop observation 匹配 | 是：35 / 38 / 176,225 ms | 否 |

第一、第二 telemetry 文件的 SHA-256 分别为
`665F2DF70DD6D2E691BB4B8AA80B9DC60B527A4DA3DA359D86462113C6F2FE23` 和
`539F8498AC00FA3435321D43B13E30859A5623CD759DB71EE4154226566D84B7`。

## 12 次汇总

| 指标 | 旧报告发布值 | observation 对应会话自洽重算 | 差异 |
| --- | ---: | ---: | ---: |
| prompt tokens | 4,997,654 | 5,077,484 | +79,830 |
| completion tokens | 60,278 | 61,830 | +1,552 |
| total tokens | 5,057,932 | 5,139,314 | +81,382（+1.609%） |
| Office total tokens | 1,667,249 | 1,748,631 | +81,382（+4.881%） |
| 模型调用 | 277 | 277 | 0 |
| 工具调用 | 327 | 327 | 0 |
| 模型耗时 | 约 2,285.2 秒 | 2,285.241 秒 | 仅旧报告舍入 |

Coding 与 Design 的发布 tokens 和自洽重算完全一致。自洽明细和另一父会话提取结果已在测试时完成
交叉核对；PR 只保留本表中的审计结论，不保留逐次 JSON。

## 防错措施

测试时的提取逻辑要求每个 child JSONL 的 metadata `parentSession` 与选定父会话一致；不匹配的 child
被排除并显式记录。所有新 run 均采用独立 workspace 和独立 project/session，再以 telemetry
`requestCount` 交叉核对 JSONL 中的模型调用数。

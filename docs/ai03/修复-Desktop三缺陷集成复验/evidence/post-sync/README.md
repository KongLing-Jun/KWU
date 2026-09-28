# 同步最新 main-v3 后的 Desktop 冒烟验收

本目录记录完整 30 次矩阵完成后，将集成分支同步到 `origin/main-v3@d68363ff` 并重新构建 Wails Desktop
所做的三场景代表性验收。它用于验证同步后的产品身份、Desktop 投影和关键业务链路，不替代也不覆盖
`evidence/after/` 中锁定在 `6a93ac2c` 的 30 次统计。

- 完整结论、三次有效样本指标、完成态时间线、成品/视觉验收和成本方向性比较：`report.md`。
- 被排除的预填充工作区样本及排除理由：`invalid/S02-preseeded-workspace/README.md`。

逐次 JSON、截图、fixture 和 Playwright 输出已在验收时核对；按 PR 整理要求不纳入 Git，关键事实已经
转录到上述两个 Markdown 文档。

所有真实 Agent 操作均通过同步后构建的 Wails Desktop 完成；CLI 只用于 fixture 准备、只读统计、离线评分
和浏览器补充验收。目录中不含 Provider 凭据。

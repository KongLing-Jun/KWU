# 修复后 30 次真实 Desktop 复验证据

本目录保存集成构建 `00897dccd4247f09396c4e9d9a293c3285dfbc32` 的文档化复验证据。实际产品代码
锁定在 `6a93ac2c64287b0e103bb81ae3abc07b692caaba`；两者差异仅为 DEF-010 自动化门禁回执文档。

- `calibration-report.md`：L01–L03 三场景校准检查点。
- `strict-comparable-12-report.md`：L01–L12 严格可比结果及两套基线对比。
- `low-24-report.md`：六个冻结任务各 4 次的固定 low 矩阵、修复前对比与方差。
- `auto-6-report.md`：默认/自动 effort 的三场景代表任务各 2 次结果及匹配 low 对比。
- `final-report.md`：30 次最终结论、数据谱系、按场景/阶段数据、分布、缺陷和建议。
- `defects/`：当前矩阵发现与测试基础设施事件。

测试期间生成的矩阵、逐次 run-spec、observation、stats、grade、截图和 Playwright 输出已经完成解析、
交叉核对和最终审计；按 PR 整理要求，这些原始工件不纳入 Git。聚合数字、判定、关键时间线和异常均已
转录到上述 Markdown 文档。

固定 low 24 次和默认/自动 6 次共 30/30 完成：语义质量、正常结束、当前会话实时 final 均为 30/30；
切换恢复和接受后重复回复均为 0。矩阵期间未同步远端、未重建二进制，未改变 Provider、模型、任务提示词
或统计口径。

# 架构决策记录

本目录保存已经由当前实现和测试确认、且需要长期维护的架构决策。ADR 记录为什么这样设计、替代方案和后果；具体 API 与操作方式仍以稳定指南、源码和测试为准。

| 编号 | 状态 | 决策 |
| --- | --- | --- |
| [ADR-0001](0001-automation-export-format-catalog.md) | Accepted | 自动化运行结果的可导出格式由后端目录统一决定 |
| [ADR-0002](0002-expert-catalog-persistence-and-sync.md) | Accepted | 专家目录在 SQLite 中区分官方与用户数据，并同步内置目录 |

决策变化时新增 ADR，并在新旧记录中建立 `Supersedes` / `Superseded by` 关系；不要删除历史 ADR。

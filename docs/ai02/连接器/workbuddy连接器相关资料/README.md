# WorkBuddy 连接器功能资料

本目录记录 WorkBuddy 5.3.5 连接器功能。资料来自客户端运行时、连接器市场缓存、静态实现、界面核验和官方公开文档。

这里关注产品和工程实现，不收录安全审计、用户账号状态、日志、Cookie、Token 或其他凭据。

## 已确认的基线

| 口径 | 数量 | 说明 |
|---|---:|---|
| 市场索引 | 74 | `.codebuddy-connector/connectors.json` 中的条目 |
| WorkBuddy 5.3.5 运行时 | 73 | `emr-query` 要求客户端 `>=5.3.10`，被版本门控过滤 |
| 个人版界面可见 | 64 | 另有 9 条仅面向 iOA、特定套餐或内部环境 |
| 市场包目录 | 78 | 包含 4 个未进入当前索引的兼容或遗留目录 |
| MCP 根配置 | 61 | 市场包内的 `mcp.json` |
| CLI 根配置 | 16 | 市场包内的 `cli.json`，包含被版本门控的 EMR |
| Token Schema | 16 | 市场包内用于生成凭据表单的 schema |

## 阅读顺序

1. [01-功能全景与用户流程.md](01-功能全景与用户流程.md)
2. [02-总体架构与模块边界.md](02-总体架构与模块边界.md)
3. [03-连接器目录与分类.md](03-连接器目录与分类.md)
4. [04-市场包格式与数据模型.md](04-市场包格式与数据模型.md)
5. [05-安装授权连接解绑生命周期.md](05-安装授权连接解绑生命周期.md)
6. [06-MCP-CLI-Skill运行时.md](06-MCP-CLI-Skill运行时.md)
7. [07-状态存储版本与环境门控.md](07-状态存储版本与环境门控.md)
8. [08-界面交互与错误状态.md](08-界面交互与错误状态.md)
9. [附录-A-典型连接器配置样例.md](附录-A-典型连接器配置样例.md)
10. [附录-B-公开资料与证据索引.md](附录-B-公开资料与证据索引.md)

## 资源导航

- [运行时完整定义](resources/runtime/connectors.full.json)：73 条运行时合并结果，包含描述、示例、MCP、CLI、Token、Skill 和可见性。
- [连接器 CSV 索引](resources/runtime/connectors.index.csv)：便于筛选、排序和二次导入。
- [市场中央索引](resources/marketplace/connectors.json)：74 条原始市场条目、版本门控、环境门控和跨连接器授权规则。
- [全部根配置](resources/marketplace/configs)：144 份 `mcp.json`、`cli.json`、`token-schema.json`、`connector-meta.json` 等文件。
- [Skill 入口](resources/marketplace/skills-entry)：51 个连接器的主 `SKILL.md`，另保留 27 个飞书一级子 Skill。
- [版本过滤说明](resources/runtime/marketplace.version-gated.json)：EMR 和 4 个未索引兼容包。
- [完整市场资源 ZIP](resources/workbuddy-marketplace-5.3.5.zip)：1,687 个文件，已修复 Windows 资源管理器显示为空的问题。
- [界面截图](resources/assets/ui)：连接器市场、自定义 MCP 管理和相关设置界面。

## 资料范围

- 连接器市场、目录注册表和版本/环境门控。
- MCP、CLI、Token、OAuth 和 Skill 组合方式。
- 安装、授权、连接、重连、解绑和更新流程。
- 本地缓存、用户状态、生成配置和运行时聚合。
- 连接器页面、自定义 MCP 页面和任务内使用流程。

# 附录 B：公开资料与证据索引

## 1. WorkBuddy 官方资料

- [连接器功能说明](https://www.codebuddy.cn/docs/workbuddy/From-Beginner-to-Expert-Guide/Function-Description/Connector)：说明 `MCP + CLI` 与 `Skill + CLI` 形态，以及部分连接器的授权和能力。
- [MCP 配置指南](https://www.codebuddy.cn/docs/workbuddy/From-Beginner-to-Expert-Guide/Function-Description/MCP-Guide)：用户级和项目级 MCP 配置、`mcpServers` 示例和状态说明。
- [WorkBuddy 更新日志](https://www.workbuddy.cn/docs/workbuddy/Changelog)：连接器搜索、最低客户端版本、CLI、OAuth、Header/Env 和工具管理演进。
- [腾讯文档资料库](https://www.workbuddy.cn/docs/workbuddy/From-Beginner-to-Expert-Guide/Function-Description/Knowledge-Base/Tencent-Doc)
- [ima 知识库](https://www.workbuddy.cn/docs/workbuddy/From-Beginner-to-Expert-Guide/Function-Description/Knowledge-Base/IMA%20Knowledge%20Base/01-Workbuddy-IMA-Basic-Guide)
- [乐享知识库](https://www.workbuddy.cn/docs/workbuddy/From-Beginner-to-Expert-Guide/Function-Description/Knowledge-Base/Lexiang)

## 2. 腾讯云官方资料

- [企业连接器管理](https://cloud.tencent.com/document/product/1831/134453)：OAuth、API Key、工具过滤、超时和自定义 Header。
- [企业版个人连接器](https://cloud.tencent.com/document/product/1831/134525)：连接器技术形态、独立授权和按需访问。
- [腾讯云 MCP 市场](https://cloud.tencent.com/developer/mcp)：外部 MCP 生态，不等同于 WorkBuddy 预置目录。
- [腾讯文档 MCP](https://cloud.tencent.com/developer/mcp/server/11803)
- [腾讯乐享 MCP](https://cloud.tencent.com/developer/mcp/server/11802)
- [TAPD MCP](https://cloud.tencent.com/developer/mcp/server/11474)

## 3. 本地证据

- [73 条运行时定义](resources/runtime/connectors.full.json)
- [74 条市场中央索引](resources/marketplace/connectors.json)
- [全量 CSV](resources/runtime/connectors.index.csv)
- [144 份根配置](resources/marketplace/configs)
- [51 个连接器 Skill 入口及 27 个飞书子 Skill](resources/marketplace/skills-entry)
- [完整原始市场资源](resources/workbuddy-marketplace-5.3.5.zip)

## 4. 版本指纹

| 对象 | SHA-256 |
|---|---|
| WorkBuddy 5.3.5 `app.asar` | `402A8A42C04D7806595BB19EE2D3C535E46899189E344CA4950BFCB56406E070` |
| `module.app-server.js` | `72B732DE9A6BD4C84AA763589E1056DE6C1B80D13CCD5C20B50A9B1272E98EBB` |
| 市场 `connectors.json` | `B2EC494DCC39C68859195F9886EC777C5C2C05B17A12890E16B26E7DC1081CF2` |
| 本目录修复版市场 ZIP | `5302C2FC0D08253ED77FA7882ACEDE7BB6E82D57750458C2E883B83386D1D618` |

## 5. 资料口径

官方连接器文档列举的卡片少于 WorkBuddy 5.3.5 实际目录，不同官方页面之间也存在更新不同步。因此：

- 产品行为以运行时 73 条定义为准。
- 市场发布能力以中央索引 74 条为准。
- 当前个人版可见范围以 UI 的 64 条为准。
- 公开文档用于解释功能和授权，不作为全量清单。

结构化的公开资料索引还保存在 [public-sources.json](resources/public-sources.json)。

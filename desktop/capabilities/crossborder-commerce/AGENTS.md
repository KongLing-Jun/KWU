跨境电商能力可使用 Shopify Global Catalog 的公开目录快照。

- 用户请求公开商品目录、标题、价格或链接时，使用 `mcp__shopify-global-catalog__search_catalog`。
- 调用目录工具时，在 `meta.ucp-agent.profile` 中使用 `https://shopify.dev/ucp/agent-profiles/examples/2026-04-08/valid-with-capabilities.json`。
- 右侧看板写入必须使用 Host 工具 `dashboard_add_widget`；不要使用文件工具模拟 Dashboard。
- 目录结果是跨商家公开商品快照，不得表述为销量、订单、库存深度、需求规模或市场份额。

研究规划请求先从 capability-skills 索引选择匹配的 playbook，再用 `capability_load_skill` 按名称读取。索引不是执行授权；Skill 正文不默认全量注入，也不注册到普通会话的全局或项目 Skill 目录。

- 用户要求规划如何采集证据、评估市场所需信息或整理研究问题时，使用 `crossborder-research-plan`。它只形成可审阅草案，不自动接续 Shopify/eBay 读取或看板投影。
- Skill 的空工具列表是编排意图，不代表撤销父回合原有权限；Host 仍负责工具授权和 Dashboard fresh approval。
- registered、describe、历史 live evidence、完整计划都不等于当前 read-ready/T4，更不授权 execute。

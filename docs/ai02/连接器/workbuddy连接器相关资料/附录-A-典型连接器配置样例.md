# 附录 A：典型连接器配置样例

本附录不重复粘贴完整配置，直接链接原始文件。

## 1. 纯远程 MCP：腾讯文档

- [mcp.json](resources/marketplace/configs/tencent-docs/mcp.json)

用于研究固定 HTTPS endpoint 和纯远程 MCP。该市场包没有独立顶层 Skill。

## 2. Header Token：Wind 金融数据

- [mcp.json](resources/marketplace/configs/wind-finance/mcp.json)
- [token-schema.json](resources/marketplace/configs/wind-finance/token-schema.json)
- [connector-meta.json](resources/marketplace/configs/wind-finance/connector-meta.json)
- [Skill](resources/marketplace/skills-entry/wind-finance/SKILL.md)

用于研究 Token 表单、凭据引用和 Header 注入。

## 3. CLI 登录：飞书

- [cli.json](resources/marketplace/configs/feishu/cli.json)
- [27 个飞书子 Skill](resources/marketplace/skills-entry/feishu)

用于研究 Node CLI、安装、版本检查、浏览器登录、状态验证和解绑。

## 4. CLI 授权：腾讯会议

- [cli.json](resources/marketplace/configs/tmeet/cli.json)
- [Skill](resources/marketplace/skills-entry/tmeet/SKILL.md)

用于研究独立 CLI 连接器的 init/auth/status/unAuth。

## 5. MCP + CLI：CloudBase

- [mcp.json](resources/marketplace/configs/cloudbase/mcp.json)
- [cli.json](resources/marketplace/configs/cloudbase/cli.json)
- [Skill](resources/marketplace/skills-entry/cloudbase/SKILL.md)

用于研究 CLI 预授权后启动 stdio MCP 的混合模式。

## 6. stdio + Token：微盛 SCRM

- [mcp.json](resources/marketplace/configs/weisheng-scrm/mcp.json)
- [token-schema.json](resources/marketplace/configs/weisheng-scrm/token-schema.json)
- [Skill](resources/marketplace/skills-entry/weisheng-scrm/SKILL.md)

用于研究环境变量 Token 和 stdio MCP。

## 7. Skill-only：携程问道

- [token-schema.json](resources/marketplace/configs/ctrip-wendao/token-schema.json)
- [Skill](resources/marketplace/skills-entry/ctrip-wendao/SKILL.md)

用于研究没有独立 MCP 配置时，如何通过 Skill 使用已有工具或运行时。

## 8. 客户端版本门控：EMR

- [cli.json](resources/marketplace/configs/emr-query/cli.json)
- [版本过滤记录](resources/runtime/marketplace.version-gated.json)

`emr-query` 要求 WorkBuddy `>=5.3.10`，因此没有进入 5.3.5 的 73 条运行时定义。

## 9. 完整原包

需要查看上述连接器的 references、scripts、assets 或子 Skill 时，使用 [workbuddy-marketplace-5.3.5.zip](resources/workbuddy-marketplace-5.3.5.zip)。

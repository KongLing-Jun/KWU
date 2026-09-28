# HTTPS 连接器准入与首发清单

## 1. 用途

本文件定义当前生产首批的封闭集合，并保留生产上线后的 HTTPS 连接器扩充队列。

当前状态：**PHASE-1 FROZEN / DESKTOP RC VERIFIED / 243 PRE-DEPLOY VERIFIED**。用户于 2026-08-11 决定先以腾讯文档、Notion、Postman完成 RC、生产部署和验收；2026-08-12 三项先通过 Windows x64 RC，再由统一发布控制面以 `0.1.0` 在 243 环境完成正常连接和使用验收。其余候选统一后置到首批上线后继续逐项调研和上架。这只调整执行顺序，不代表剩余连接器已完成、永久排除或总方向改变。

## 2. 状态值

| 状态 | 含义 |
| --- | --- |
| `RELEASE-REQUIRED` | 符合准入条件，必须完成测试和生产 current 才能关闭 G-RELEASE |
| `DECISION-REQUIRED` | 存在 App Secret、非标准 OAuth、付费、凭据/账号缺失、接口不稳定或事实不足，等待用户裁决 |
| `DEFERRED-BY-USER` | 用户明确批准后置，不阻断本次首发 |
| `EXCLUDED` | 明确不符合 HTTPS-only 条件或用户已砍掉 |
| `HISTORICAL` | 曾真实实现，但不属于当前方向 |

状态只能基于官方资料、真实响应和用户决定更新。

## 3. 准入检查

- [ ] WorkBuddy 5.3.5 公网/个人可见，或用户明确要求纳入。
- [ ] 官方/厂商公开 HTTPS MCP endpoint。
- [ ] Streamable HTTP；不是 SSE、stdio、CLI 或本地代理。
- [ ] 不需要 Node、npm、Python、shell、localhost 或桌面应用。
- [ ] 认证可由 none、标准 MCP OAuth 或 token-schema Header 表达。
- [ ] Token 不进入 URL，官方获取链接和最少字段可确定。
- [ ] 无需厂商 App Secret；否则转 `DECISION-REQUIRED`。
- [ ] 有可用测试账号、资源和必要读写权限；缺少时转 `DECISION-REQUIRED`。
- [ ] Desktop 无产品 ID 代码即可接入。

## 4. 已裁决连接器

| ID | WorkBuddy/来源 | endpoint | auth | 当前状态 | 依据与下一步 |
| --- | --- | --- | --- | --- | --- |
| `tencent-docs` | WorkBuddy、腾讯文档官方 MCP | `https://docs.qq.com/openapi/mcp` | 标准 MCP OAuth | `RELEASE-REQUIRED` | RC 发现 203 工具；统一发布控制面 `0.1.0` 已正常使用，仍需服务器和正式 `1.0.0` 验收 |
| `notion` | WorkBuddy、Notion 官方 MCP | `https://mcp.notion.com/mcp` | 标准 MCP OAuth | `RELEASE-REQUIRED` | 零专用代码动态接入；统一发布控制面 `0.1.0` 已正常使用，仍需服务器和正式 `1.0.0` 验收 |
| `postman` | Postman 官方远程 MCP | `https://mcp.postman.com/minimal` | Token Header：`Authorization: Bearer` | `RELEASE-REQUIRED` | 单字段 Keyring/Header 已闭环；统一发布控制面 `0.1.0` 已正常使用，仍需服务器和正式 `1.0.0` 验收 |
| `feishu-cloud-docs` | WorkBuddy CLI、飞书官方远程 MCP 调研 | 不进入当前包 | App Secret/特殊远程授权 | `EXCLUDED` | 用户于 2026-08-10 决定砍掉；历史 CLI 包/current 不进入首发 |

## 5. 生产上线后扩充队列

| 候选 | 已知事实 | 当前状态 | 必须补充的事实 |
| --- | --- | --- | --- |
| CloudBase | 官方存在托管 HTTPS MCP；认证和部分能力与本地模式不同 | `DEFERRED-BY-USER` | 首批生产上线后核实 WorkBuddy 对应定义、正式 endpoint、Header 凭据、真实 tools/list 和测试账号 |
| 腾讯会议 | 官方 MCP 服务在云端，官方示例使用本地代理注入 Token/Header | `DEFERRED-BY-USER` | 首批生产上线后核实直接 Streamable HTTP endpoint/path、Token 获取方式和无需代理的真实行为 |
| WorkBuddy 其余公网远程定义 | 快照注册 73 项，含远程、Token、CLI、stdio、SSE 和内部项 | `DEFERRED-BY-USER` | 首批生产上线后逐项拆分为独立 ID、核实并裁决，不允许整批写成已完成 |

## 6. Token/Header 代表样本

代表样本已冻结为 Postman 官方远程 MCP：

- endpoint：`https://mcp.postman.com/minimal`
- 官方远程 MCP 文档：`https://learning.postman.com/latest-v-12/docs/reference/postman-api/postman-mcp-server/postman-mcp-remote-server`
- 官方获取链接：`https://learning.postman.com/docs/reference/postman-api/authentication`
- 必填字段：一个 secret API Key
- Header：`Authorization: Bearer {postman_api_key}`
- 统一发布控制面预上线 current：`postman@0.1.0`

真实连接、Agent读写、重启恢复和本地解绑已经通过，Token/Header 认证类型达到当前测试发布门禁。

## 7. 每个首发项的证据

| ID | 测试 version/current | size/SHA-256 | tools/list | 真实调用 | 重启/刷新 | 解绑 | production current |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `tencent-docs` | `0.1.0` | 701 / `de97527225ddba24a573ec412271863e16ad632aeace8acdaa3b25857fb37c90` | 203（既有 RC） | 243 环境普通 Agent正常使用；历史审批写入证据保留 | RC 重启/refresh 既有证据保留 | 正常解绑既有证据保留 | 正式 `1.0.0` 待填 |
| `notion` | `0.1.0` | 773 / `7d2fd1be05fbff061e7d89cdd01b4f69d2b69bee1aa46493af784bd4e3daf55c` | 28（既有 RC） | 243 环境普通 Agent正常使用；历史真实写入/回读证据保留 | RC 重启既有证据保留 | 正常解绑既有证据保留 | 正式 `1.0.0` 待填 |
| `postman` | `0.1.0` | 1107 / `471e1f2bf87a4a98adf02f1bca0bfbb95e8ee52ace3fb5e1cc2b326ce985c382` | 41（既有 RC） | 243 环境普通 Agent正常使用；历史真实写入证据保留 | RC 重启既有证据保留 | 正常解绑既有证据保留 | 正式 `1.0.0` 待填 |

## 8. 完成门禁

- 当前三个 `RELEASE-REQUIRED` 必须完成生产 current 和生产真实验收，才能关闭本次 `G-RELEASE`。
- `DEFERRED-BY-USER` 已有用户明确顺序裁决，不阻断本次 RC/Release；首批上线后继续逐项拆分、调研和上架。
- `EXCLUDED` 必须记录不符合的具体条件。
- 清单最终状态、ZIP/Hash 和生产验收同步到 `08`。

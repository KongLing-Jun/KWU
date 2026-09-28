# WorkBuddy 飞书连接器实现纠正报告

## 一、结论

当前方案错误地把以下两种实现混在了一起：

```text
方案 A：云端 OAuth / 飞书 MCP
- 用户或管理员手动创建飞书应用
- Reasonix Cloud 保存 App ID / App Secret
- 云端处理 OAuth 回调和用户 Token
- Agent 调用 MCP 工具

方案 B：WorkBuddy 实际采用的本地 CLI
- 桌面端安装飞书官方 lark-cli
- lark-cli 引导创建/配置应用
- lark-cli 在本地完成用户授权
- Agent 通过 lark-* Skills 调用 lark-cli
```

如果目标是第一阶段复刻 WorkBuddy，应该采用方案 B。

## 二、需要纠正的问题一

原问题：

> 飞书开放平台应用是否已经创建？需要自建应用、App ID、App Secret、OAuth 重定向地址和对应权限。不要把 App Secret 发到聊天中，后续放入云平台本地 .env 和服务器 Secret 环境即可。

### 判断

这段话不能作为 WorkBuddy 式第一阶段的前置条件。

飞书底层确实仍然存在“自建应用、App ID、App Secret、用户 OAuth”这些概念，但 WorkBuddy 并不要求用户先手动准备凭据，也不把这些凭据交给 WorkBuddy 云平台。

WorkBuddy 飞书连接器实际执行：

```bash
# 第一次配置应用
lark-cli config init --new --lang en

# 用户授权
lark-cli auth login --recommend

# 检查状态
lark-cli auth status
```

配置来源：[WorkBuddy 飞书 cli.json](https://github.com/infometa/workbuddyskills/blob/main/connectors/feishu/cli.json)

飞书官方 CLI 文档也明确将其描述为：

- 一键创建应用；
- 浏览器交互式初始化；
- 本地保存凭据；
- 使用系统原生钥匙串保护凭据；
- Agent 只需要提取授权 URL 并交给用户完成操作。

参考：[飞书官方 lark-cli](https://github.com/larksuite/cli)

### 正确表述

建议改成：

> 第一阶段采用 WorkBuddy 相同的飞书官方 `lark-cli` 本地授权方案。用户不需要提前创建飞书应用，也不需要向 Reasonix Cloud 提供 App ID/App Secret。首次连接时，Reasonix Desktop 执行 `lark-cli config init --new`，引导用户在浏览器完成应用创建或配置；随后执行 `lark-cli auth login` 完成用户授权。应用凭据和用户令牌由 lark-cli 保存在本机，不进入 Reasonix Cloud。

安全提醒“不要把 App Secret 发到聊天中”本身仍然正确，但“放进云平台 `.env` 和服务器 Secret”不适用于当前第一阶段。

## 三、需要纠正的问题二

原问题：

> 是否同意第一版飞书范围限定为“云文档”？我建议先启用官方远程 MCP 的少量工具，例如文档搜索、读取、创建和更新。

### 判断

这里包含两件事：

1. 第一版限定为“云文档”：合理。
2. 通过“官方远程 MCP”实现：不符合 WorkBuddy 已确认的实现。

WorkBuddy 的飞书连接器目录中只有：

```text
feishu/
├─ cli.json
└─ skills/
   ├─ lark-doc
   ├─ lark-drive
   ├─ lark-wiki
   ├─ lark-sheets
   └─ ...
```

没有 `mcp.json`。

因此，WorkBuddy 飞书不是远程 MCP，而是：

```text
连接器卡片
    ↓
安装并授权 lark-cli
    ↓
启用 lark-doc 等 Skills
    ↓
Agent 执行 lark-cli docs ... 命令
```

飞书官方另有 `lark-openapi-mcp` 项目，但它属于另一条实现路线，需要显式提供 App ID/App Secret，并启动 MCP Server；不能称为 WorkBuddy 当前采用的“官方远程 MCP”。

参考：[官方 MCP CLI 文档](https://github.com/larksuite/lark-openapi-mcp/blob/main/docs/reference/cli/cli.md)

### 正确表述

建议改成：

> 是否同意第一版飞书能力限定为“云文档”？建议先接入 `lark-doc` 和必要的 `lark-drive` Skills，通过本地 `lark-cli` 完成文档搜索、读取、创建和更新，以“一读一写”证明完整闭环。第一阶段不启动飞书 MCP Server，也不建设云端飞书 OAuth。

如果需要限制授权范围，不应直接使用 WorkBuddy 当前的：

```bash
lark-cli auth login --recommend
```

因为实测该参数申请的权限非常广。

应改为：

```bash
lark-cli auth login --domain <云文档相关域>
```

或者使用精确权限：

```bash
lark-cli auth login --scope "<具体权限>"
```

具体域和权限应根据当前 CLI 的 `auth scopes` 输出确定，不能凭猜测写死。

## 四、第一阶段正确闭环

```text
1. 用户点击飞书卡片
2. Desktop 检查 lark-cli 是否安装
3. 未安装则自动安装
4. 检查 lark-cli config
5. 未配置则执行 config init --new，并打开浏览器
6. 执行限定权限的 auth login
7. 用户在浏览器点击授权
8. Desktop 轮询 auth status
9. 登录成功后启用 lark-doc / lark-drive Skills
10. Agent 完成一次文档读取
11. Agent 完成一次文档创建或更新
12. 卡片显示“已连接”
```

第一阶段云平台只需要：

```text
- 分发连接器目录
- 分发 cli.json
- 分发对应 Skills
- 提供版本和下载地址
```

暂时不需要：

```text
- 飞书 App ID/App Secret 存储
- 飞书 OAuth 回调接口
- 用户 Token 云端存储
- 飞书远程 MCP
```

## 五、仍然需要确认的产品问题

应该向用户确认的是：

> 第一版是否同意只开放飞书云文档能力，以“读取一个文档 + 创建或更新一个文档”作为验收闭环？确认后，将采用 WorkBuddy 相同的本地 lark-cli 授权方案，不要求用户手动填写 App ID、App Secret 或回调地址。

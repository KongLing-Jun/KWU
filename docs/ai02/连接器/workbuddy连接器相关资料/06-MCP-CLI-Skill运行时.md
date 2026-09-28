# MCP、CLI 与 Skill 运行时

## 1. MCP Runtime

WorkBuddy 5.3.5 使用 MCP SDK 1.24.3，连接 stdio、SSE 和 Streamable HTTP Server。

### 1.1 发现内容

连接完成后，运行时读取：

- tools
- prompts
- resources
- resourceTemplates

连接器管理结果同时记录 status、disabled、needsAuth、error、trustLevel、configHash 和 lastApprovedAt。

### 1.2 工具命名

MCP 工具会加 Server short-name 前缀。代理据此前缀区分不同 Server 的同名工具，并在 Server 断开时移除对应工具集合。

### 1.3 HTTP/SSE

远程 MCP 创建 transport 时组合：

- 市场静态 headers。
- Token Schema 产生的 headers。
- OAuth Provider 产生的 access token。
- 请求范围的授权 headers。

实现支持 HTTP/SSE transport 回退、OAuth scheme、HTTP callback、重连和 backoff。

### 1.4 stdio

stdio MCP 配置由 `command`、`args`、`env` 和 runtime 信息组成。Windows 上的 npx MCP 会安装到按包规格哈希生成的稳定目录，再解析 package bin 启动。

## 2. Connector Proxy

WorkBuddy 把官方连接器、自定义 MCP、插件 MCP 和内置/云注入 MCP 聚合到一个本地 Connector Proxy。

静态冲突优先级：

| 来源 | 优先级 |
|---|---:|
| 官方连接器 | 100 |
| 插件 MCP | 50 |
| 自定义 MCP | 10 |

官方连接器断开时，同名自定义 MCP 存在回退逻辑。聚合后的 `.mcp.json` 指向：

```text
http://127.0.0.1:<dynamic-port>/mcp
```

正式版本次运行中，Connector Proxy 监听端口为 56504；调试版为 60703。

## 3. 自定义 MCP 审批

自定义远程 MCP 的信任 hash 使用 URL origin。stdio MCP 的 hash 使用 command、排序后的 args 和 env 键名。

未批准的自定义 MCP 默认 disabled。审批状态保存在 `mcp-approvals.json`。

## 4. CLI Runtime

### 4.1 托管路径

```text
<configDir>/binaries/node/cli-connector-packages
<configDir>/binaries/node/cli-connector-cache
<configDir>/binaries/node/versions/<version>
```

CLI 执行时将私有 bin 目录放到 PATH 前部。npm registry 回退顺序包含腾讯云镜像、npmmirror 和 npmjs。

### 4.2 执行阶段

| 阶段 | 配置字段 | 默认超时/行为 |
|---|---|---|
| 安装 | `init` | 最长约 5 分钟 |
| 版本 | `versionCheck` | 命令输出正则提取版本 |
| 授权 | `auth` | 默认约 10 秒；部分配置等待退出 |
| Device Flow | `authDeviceFlow` | 最长约 5 分钟，约 3 秒轮询 |
| 状态 | `status` | 约 10 秒 |
| 解绑 | `unAuth` | 解绑流程中执行 |

平台命令从 `win32`、`darwin`、`linux` 中选择。

### 4.3 授权交互

CLI Runtime 能从进程输出识别：

- 授权 URL。
- 二维码 URL。
- Device Flow verification URI 和 user code。
- `statusMatch` 文本。
- `statusMatchJson` 结构。

`authSuppressBrowser` 控制是否由客户端打开浏览器；`authQrModal` 控制内嵌二维码弹窗；`authWaitForExit` 表示等待 CLI 自己完成交互。

### 4.4 连接器实例

- 飞书：Node CLI 安装、浏览器登录、status 验证和多组子 Skill。
- 腾讯会议：CLI 创建、查询和管理会议。
- TextIn xParse：CLI 安装和 Device Flow 相关配置。
- CloudBase：CLI 预授权与 stdio MCP 组合。
- EMR：Python `tccli`，最低 CLI 版本 `3.1.56.1`，同时要求 WorkBuddy `5.3.10`。

## 5. Skill Runtime

### 5.1 组合形态

| 组合 | 市场中的表现 |
|---|---|
| MCP + Skill | MCP 提供工具，Skill 描述调用流程 |
| CLI + Skill | CLI 提供账号和命令，Skill 描述业务步骤 |
| MCP + CLI + Skill | CLI 负责预授权或安装，MCP 提供工具 |
| Skill-only | 没有独立 MCP Server，由 Skill 使用已有能力 |

### 5.2 物化目录

已启用连接器的 Skill 会复制到：

```text
<configDir>/connectors/skills/connector-<id>
```

正式版当前启用的飞书连接器物化了 27 组 Skill，共 457 个文件，约 4.95 MB。

### 5.3 Skill 内容

顶层 Skill 通常描述：

- 适用场景和业务对象。
- MCP 工具或 CLI 命令。
- 参数、分页、时间和 ID 规则。
- 写操作确认和错误处理。
- references、scripts 和 assets 的读取入口。

[skills-entry](resources/marketplace/skills-entry) 保存 51 个连接器主入口和 27 个飞书一级子 Skill。完整文件位于 [市场 ZIP](resources/workbuddy-marketplace-5.3.5.zip)。

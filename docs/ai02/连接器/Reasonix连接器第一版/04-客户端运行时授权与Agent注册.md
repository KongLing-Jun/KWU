# 客户端运行时、授权与 Agent 注册

## 1. 当前代码与目标差异

`CURRENT（窗口 10R）`：Desktop managed connector 已收口为 HTTPS Streamable HTTP MCP，支持 `none|oauth|token` 和可选纯提示型 Skills；managed Node/npm/CLI、stdio/SSE 构造和构建携带已删除或硬拒绝。用户自定义 MCP 的 stdio、Root CommandEnv、CapabilityTurn 和 bash 环境链保持不变。

该状态已经执行窗口、独立审计和总控制窗口必要门禁复核通过。Cloud validator、首发三连接器测试控制面和 Windows x64 RC 均已完成，相关三仓功能分支也已合入正式主线；正式版本构建/签名、生产部署和 `G-RELEASE` 仍未完成。

## 2. 包安装与 active

正式链路保持：

```text
Cloud Catalog
→ 下载不可变 ZIP
→ size/SHA-256/ZIP/schema 校验
→ installer
→ active pointer
→ owner 状态机
→ MCP/Skills 注册
```

- HTTPS-only parser 必须同时存在于 Cloud 和 Desktop。
- 历史 active 若包含 CLI、stdio、skills-only 或其他不支持入口，标记为不受支持，不执行、不恢复 Agent 能力。
- 不直接编辑 active/cache/state 冒充迁移成功。
- 持久化 last-good Catalog、同 Hash ZIP 缓存和 active 自动修复未进入本次 RC，继续作为后置可靠性项；不得按旧窗口 10 方案恢复。

## 3. Owner 与 Keyring

继续复用现有 actor/profile/Cloud binding 和 binding generation：

- OAuth flow、OAuth credential、Token 字段、连接状态和解绑按 owner 隔离。
- Token/API Key 使用现有 connector/server/field Keyring slot，不写 `.env`、SQLite 或普通文件。
- 账号切换和登出必须取消旧 owner 未完成的 OAuth flow。
- 不新增 Profile 产品 UI。

## 4. 标准 MCP OAuth

保持现有通用链路：

1. protected-resource metadata；
2. authorization-server metadata；
3. public-client DCR；
4. PKCE S256 和 loopback callback；
5. Token Keyring-only 保存；
6. runtime Authorization Header；
7. 已知过期按需 refresh；
8. 首次 401 最多 refresh 并重放一次；
9. `invalid_grant` 删除精确凭据、回到 `authorization_required` 并撤销工具。

需要 client secret、HTTP Basic、特殊云端 Token 交换或厂商私有 OAuth 的连接器不得在 Desktop 增加产品分支，必须停止并报告。

## 5. Token/API Key

Token 连接器必须由包内 `token-schema.json` 驱动：

- 页面显示 `documentation_url`/`documentation_label`；
- 只显示 schema 声明的最少字段；
- 提交后字段进入当前 owner Keyring；
- runtime Header provider 每次请求读取 Keyring并渲染模板；
- 解绑删除精确 connector/server/field slot；
- 包内静态 Header 不能包含秘密；
- 不实现 URL/query/path/body/Cookie/环境变量注入。

## 6. MCP 与 Agent 注册

- runtime name 继续使用 `<connector_id>--<server_id>`。
- `initialize` 和非空 `tools/list` 成功后才进入 connected。
- 继续复用共享 MCP Host、Controller Registry、CapabilityTurn、工具审批和审计。
- 新 controller 可以复用共享 Host 已成功发现的工具，但实际调用仍走原 MCP client/transport。
- 断开和 `invalid_grant` 必须从当前及新任务撤销工具。
- 项目对话沿用现有手动选择；不实现项目级默认继承。

## 7. 纯提示型 Skills

- 只有同包 MCP connected 后才注册 Skills。
- Skills 只用于帮助 Agent选择并调用同包 MCP 工具，不提供命令执行能力。
- 不注入 PATH、CommandEnv 或任何运行时环境。
- 包含本地命令、脚本、可执行文件、本地文件流程或 localhost 依赖的 Skill 拒绝上架。

## 8. 失效 tab 错误

窗口 10R 已修复连接器设置页持有失效 tab 后进入全局 `unhandledrejection` 的问题：

- workspace 向连接器页传递实时 target tab；Catalog 可在无任务时只读，State/Install/Connect 不得产生副作用；
- 后台轮询把明确的 target unavailable 视为生命周期结束，不进入全局错误页；
- 其他后台错误和前台真实连接错误仍显示为页面局部错误；
- Token 连接失败返回 `false`，表单和用户刚输入的值保留，只有完整成功才关闭；
- 没有修改全局 crash 框架或增加复杂 tab 生命周期系统。

## 9. 构建收口

Windows x64 正式构建和 NSIS 必须：

- 不下载、不携带、不安装连接器 Node/npm/lark-cli；
- 不启动连接器 stdio/CLI 子进程；
- 不依赖用户系统 Node、Python、Git Bash 或 shell；
- 正式构建默认连接生产 Cloud；开发构建允许以 `KWU_CLOUD_BASE_URL` 或开发默认值连接已验收的 `192.168.10.243:8080`，历史 18080 不再作为后续发布控制面；
- 通过现有 Wails、NSIS、受影响测试、前端构建和 PR gate。

窗口 12 已在 `7226c3ce2032880eec263aabfc6731ddabd309ae` 通过标准 Windows amd64 Wails/NSIS 入口生成 `v1.1.2-rc.12` 候选产物。二进制默认包含生产 Cloud `https://www.suiyuan-ai.com`、不包含 `127.0.0.1:18080`；安装包解包后不含 connector Node、npm、lark-cli、飞书 ZIP 或 managed connector 本地运行时。该产物仍来自未同步最新主线的功能分支，不是正式版本、正式签名安装包或生产发布物。

## 10. RC 生命周期事实

- 窗口 X 的行为由 `DesktopCloseBehavior=background` 定义为后台驻留，不是正式退出失败；正式退出入口是托盘“退出”。
- 当前 HEAD 连续三次正式退出均使进程在 10 秒内自然结束，最终验收 PID 在 198 ms 内消失，未使用强制结束。
- “新任务 controller 生命周期竞态”没有形成可复现产品缺陷：原失败现场是模型路由不可用导致 controller 构建失败。窗口 12 先验证普通模型链路，再在三个全新任务完成真实 MCP 调用，未出现空 Registry、重复 Host 或旧 context 取消。
- 三个连接器最终均通过正常 UI 解绑；`authorization_required` 是用户主动解绑后的正确状态。
- 连接器页顶部汇总仍基于排除 managed runtime 的 `visibleServers`，可能显示 0；卡片状态、Registry 和真实调用不受影响，作为后置 P2 UI 计数问题处理。

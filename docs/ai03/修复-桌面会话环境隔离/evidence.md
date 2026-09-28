# 桌面会话环境隔离调查证据

本文件承载 Stage 1 的事故和源码证明；稳定产品决策、方案与验收以
[plan.md](./plan.md) 为准。

## 1. 事故证据

- 2026-07-24 15:15:26.517Z，`client_version=dev`、UA
  `Go-http-client/1.1` 的桌面开发/测试构建，从正式 Clouvia 数据根取得的
  device identity 直连 `http://192.168.10.243:8080/api/v1/auth/login`。
- 2026-07-25 07:31:50.627Z，同一客户端直连开发环境执行 refresh，生成新的
  refresh leaf。两条开发 family 都是正常 rotation，没有 reuse、logout、
  all-devices logout 或 operator revoke。
- 2026-07-25 08:18:44Z，生产环境收到 `/api/v1/me/maas-key` 和
  `/api/v1/usage-reports/finalize`，均为 401；窗口内没有 `/auth/refresh`，
  access token 当时未过期。
- 生产主库和旧库都没有对应 user/session，错误高置信为
  `AUTH_REQUIRED: Session is no longer active`。桌面随后把会话保存为 expired。
- DNS/路由取证支持客户端直接访问开发地址，不支持 `www.suiyuan-ai.com` 被导向
  开发环境。

结论：这是桌面环境隔离/持久化会话污染，不是 token 自然过期或云端错误 revoke。
干净正式构建不会主动选择开发 URL，但仍会加载正式数据根中已经污染的旧认证文件。
生产数据库重建、远端会话物理删除或账号删除也会通过 `AUTH_REQUIRED` 进入同一
运行时终止路径。

## 2. 基线

| 项目 | 值 |
| --- | --- |
| Codegraph 索引工作区 | `D:\Reasonix` |
| Codegraph 导航基线 | `345a695b28fae2b6f09916c745ffcd52e1885f24` |
| 独立 worktree 基线 | `origin/main-v3@e4000e52d91214e16870163acbf10cd6ab553833` |
| worktree 分支 | `codex/fix-desktop-session-environment-isolation` |
| EOL | 相关受控文件为 LF |

两个提交不同，Codegraph 只用于导航。当前 worktree 已直接搜索和读取下列材料。
最新同步增加了 expert assets 客户端，并触及 cloud runtime 构造和 model billing，
但没有改变本计划依赖的 durable、认证状态机、expired 前端分支和 billing
`reconcileAuth` 契约；下列结论均按最新 worktree 复核。

## 3. 构建与环境入口

- `desktop/main.go:41-45`：未注入版本时才是 `dev`。
- `scripts/desktop-build.sh:64`：发布脚本注入 `main.version`。
- `desktop/cloud_runtime.go:65-79`：只有 dev 使用 override，正式版固定 production
  default；`cloud_runtime_real_test.go:95-104` 覆盖正式版忽略 override。
- `desktop/dev_profile.go:91-134`：production 不使用 dev profile；dev 按规范化
  URL 派生 environment ID 和数据根。
- `desktop/dev_profile.go:137-178`：规范化 scheme/host/default port/path，并拒绝
  query/fragment；测试覆盖等价 URL 稳定和不同 URL 分离。
- `desktop/cloud_runtime.go:257-341,363-420,501-600`：real/fixture runtime
  创建 durable binding/session 时只传 paths，没有环境身份。

## 4. Durable 与认证状态机

- `internal/cloud/contract.go:30-35,64-70,121-133`：`BindingScope`、
  `ProfileBinding`、`Session` 均无环境或终止原因；`BindingExpired` 已定义。
- `durable_binding_repository.go:9-22`：v1 envelope 只有 schema/version/profile/
  binding；构造函数只接 paths。
- `durable_session_store.go:10-25`：v2 envelope 只有 schema/version/profile/
  sessions；v1 迁移会清 token 并改 offline。
- `durable_json.go:41-60`：open、decode 和 trailing content 都只返回普通 wrapped
  error，调用方无法稳定区分内容错误与临时 I/O。
- `auth_application.go:452-497`：expired session 映射 `AccountExpired`；非
  bound/unbound binding 当前映射 offline；bound 但无 session 映射 anonymous。
- `auth_application.go:624-642`：认证提交只允许 unbound/bound，expired 不能直接
  重新认证。
- `auth_application.go:645-661` 与
  `auth_identity_lifecycle.go:476-497`：终止失败只改 session state，可能保留原
  token，不保存原因。
- `auth_identity_lifecycle.go:632-642`：终止列表包含 AUTH_REQUIRED、token/
  refresh 失效、device mismatch 和 account disabled。
- `auth_application.go:283-334,645-661`：显式 refresh 通过独立的
  `AuthApplication.persistFailure` 写入失败状态；它和 request executor 当前都会
  保留旧 token，因此两条路径都必须遵循同一 scrub/reason 契约。

## 5. 桌面与前端失败投影

- `desktop/cloud_api.go:143-171`：expired bootstrap 在远端 `/me`、缓存与
  operation error 前返回。
- `accountStore.ts:187-190`：expired + cloud user ID 被当成 recoverable。
- `AccountCenter.tsx:531-533,653-797`：recoverable 分支继续显示身份、余额、
  订阅和账户操作，名称可回退 UID。
- `App.tsx:3527-3532,3778-3905`：recoverable 决定侧栏标签、菜单余额/支付和
  logout；折叠按钮当前只负责展开菜单。
- `useAccountSidebar.ts:87-99`：recoverable identity 参与头像存储键；失效 UID
  因此仍可选中旧账号头像。

## 6. 测试缺口

- `durable_repositories_test.go` 覆盖持久化、v1 session 迁移、损坏/未知 JSON、
  I/O token safety 和同进程并发，但没有环境矩阵、拒绝旧凭据或重新登录覆盖测试。
- `remote_account_application_test.go` 覆盖 token expiry replay 和 network
  offline，没有确定性 AUTH_REQUIRED 的 token 清理、原因和重启。
- `account-sidebar-bootstrap.test.tsx` 当前把 expired 头像槽位当作既有行为。
- `account-store.test.ts`、`account-center-states.test.tsx` 和
  `workbuddy-primary-navigation.test.ts` 没有本计划定义的失效菜单、登录页说明
  和匿名/offline 区分验收。

## 7. 收敛方案的源码依据

再次评审后，只保留与事故和用户可见问题直接相关的可达路径：

- `auth_identity_lifecycle.go:377-390` 在不持有 `refreshMu` 时执行网络请求；
  `persistFailure` 到 480 行才取锁，随后 484-497 行会加载并无条件 expire 当前
  scope session。因此旧 token 响应可能晚于同 scope rotation。
- `AuthenticatedRequestExecutor` 同时服务 `/me`、MaaS、usage、支付等客户端；
  本次事故窗口也实际出现两个并行终止请求。终止写入需要 observed session CAS
  和重复请求幂等，不属于已排除的“多个独立 executor”扩展。
- 环境拒绝发生在启动恢复的 durable 读取边界；运行时远端终止发生在已有 session
  上。两条路径不需要共享 binding/session 事务：前者不返回旧 token，后者只清理
  observed/current 仍一致的 session。
- `accountStore.ts:339-351` 拒绝 generation 倒退和同 generation UID 改变。运行时
  expired 可以保留内部 binding identity、只禁止 UI 恢复；环境拒绝后的首次登录
  使用下一 generation，即可避免修改通用前端顺序规则。
- `model_billing.go:652-671` 只有 unbound 会显式清除 manager 的 `hasBinding`；
  expired 状态可能保留旧 controller scope。
  `desktopSubscriptionModelsForActor` 只检查该 binding，未再次检查 authorized，
  因此需要针对 expired 撤销内存目录可达性；offline 仍应保留原缓存。
- durable binding 与 session 使用独立 path lock。收敛方案不增加两文件事务：
  环境拒绝的安全边界是“不返回旧 token”，成功重新登录仍使用现有逐文件原子写入。
- `durable_session_store.go:32-51` 的普通 Save 会先读取旧文档，
  `durable_binding_repository.go:54-81` 的 ConfirmBinding 也拒绝非 bound/unbound
  状态；因此认证成功后需要窄化替换入口，不能声称普通 Save 会自动覆盖被拒绝文档。
- `cloudProductionBaseURL` 是源码常量，正式构建忽略开发 override。正式形态验收若
  只隔离数据根，回归时仍可能访问真实生产；必须增加 OS 级出站阻断和记录。

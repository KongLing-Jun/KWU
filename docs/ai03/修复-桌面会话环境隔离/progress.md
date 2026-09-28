# 桌面失效账户与会话环境隔离修复进度

## 当前状态

- 阶段：Stage 3 编排实施
- 状态：任务 001/002 实现、自动验证和最终独立应用验收全部完成
- 范围：认证安全边界、expired 桌面/前端投影及对应测试；尚未推送或创建实现 PR
- 稳定决策与验收：[plan.md](./plan.md)
- 详细调查证据：[evidence.md](./evidence.md)

## 阶段与关口

| 项目 | 状态 | 下一关口 |
| --- | --- | --- |
| Grilling 与事故定性 | 完成 | 已确认产品边界、迁移和 UX |
| 最新基线与独立 worktree | 完成 | `origin/main-v3` 已核对 |
| Stage 1 计划 | 已合并 | 以 GitHub/Git 为准 |
| Stage 2 任务拆解 | 已合并 | 任务合同作为实施输入 |
| Stage 3 实施 | 两项任务、自动验证与最终独立应用验收完成 | 创建或更新 PR 前确认最新 `origin/main-v3`，执行最终 PR Gate |

## 已确认决策

- 核心结果是失效账户不再显示 UID/空账户页，直接复用现有登录/注册流程。
- 客户端隔离 prod/dev 与 dev A/dev B；不修改云平台。
- binding/session 外层文档保存环境 ID 和规范化 base URL，精确匹配。
- 旧 schema、环境不匹配和内容损坏 fail closed；不保留登录连续性。
- 临时 I/O 和网络类错误不误判为登录失效。
- 运行时终止只清当前 session 认证材料并保存稳定 reason；不改 binding。
- 仅环境拒绝后的首次成功登录使用下一 generation 隔离旧缓存。
- 左下角仍先展开菜单；失效菜单顶部进入现有登录/注册页。
- 失效菜单除折叠标签、顶部标题和副标题外，完整复用当前未登录菜单 DOM 与行为：
  保留本地头像上传、余额和套餐支付占位入口；旧云数据和退出登录仍不可见。
- 不增加中间页、额外按钮、弹窗或 Toast。
- 不增加跨文件事务；账号切换重构、refresh 并发扩展、跨进程锁和单实例策略均为
  非目标。

## 任务与交付映射

| 任务 | 串行组 | 依赖 | 实现交付 |
| --- | --- | --- | --- |
| [001 建立认证环境与终止会话安全边界](./tasks/001-建立认证环境与终止会话安全边界.md) | Serial-01 | 无 | 后端安全提交 |
| [002 呈现登录失效并完成应用验收](./tasks/002-呈现登录失效并完成应用验收.md) | Serial-02 | 任务 001 完成并通过主会话复审 | 桌面/前端体验提交 |

两个任务进入同一个实现 PR。任务 001 单独合并会让旧凭据先被拒绝而 UI 仍保留旧
UID 页面，因此只作为可独立测试和复审的提交，不作为可发布 PR。任务 002 完成用户
体验和应用验收后，整体才可合并、发布和回滚。

## 基线与验证快照

| 检查 | 状态 | 结果 |
| --- | --- | --- |
| Codegraph 导航基线 | 完成 | `D:\Reasonix@345a695b...`，仅作导航 |
| Implementation worktree 基线 | 完成 | `origin/main-v3@7c649702...` |
| 当前源码复核 | 完成 | durable、认证、bootstrap、菜单、账户页和测试入口已直接核对 |
| 风险复核 | 完成 | 只保留迟到响应 CAS、expired 撤权和安全验收等真实可达防护 |
| 分支 | 完成 | `codex/fix-desktop-session-environment-isolation-impl` |
| 任务 001 实现 | 完成 | 两个安全边界提交已由主会话复审后进入任务 002 |
| 任务 002 聚焦验证 | 通过 | 最新菜单边界下 frontend typecheck、四个聚焦测试和 build 全部通过 |
| 正式构建应用验收 | 通过 | prod legacy fixture 真实桌面窗口显示完整 expired 菜单与 AccountCenter；本地头像/上传和未登录占位可见，无 UID、旧云数据或退出登录；既有零生产 cloud 出站、历史文件不改写和同根重启证据保持有效 |
| dev AUTH_REQUIRED 应用验收 | 通过 | stub 对 `/api/v1/me` 返回 401；token 清空并持久化 `AUTH_REQUIRED`/终止时间，binding 保留；stub 下线重启仍为 expired 且不恢复 |
| dev 路径隔离 | 通过 | device、config、workspace/tabs/window、billing、session 均在 override；隔离 APPDATA 下未创建 `Clouvia` |
| retryable 对照 | 通过 | 唯一 dev URL 连接拒绝投影为 offline，token/device/expiry/UID 保留且 UI 显示 `usr_retryable_offline`；未创建 APPDATA `Clouvia` |
| 成功认证替换 | 通过 | Go 合同测试覆盖拒绝凭据只能由成功认证替换；真实窗口确认 authenticated/offline 普通账户投影 |

## 阻塞与下一关口

当前无产品、范围或应用验收阻塞。主会话已在同步后的最终树重建并使用 prod legacy
fixture 完成真实桌面复验：底部、菜单和 AccountCenter 文案准确，本地头像/上传、余额
“—”、套餐与支付“当前订阅”、设置/更新/外观均可见，UID 和退出登录不可见。未执行
真实登录：当前没有可用凭据，且本轮目标是验证失效投影与未登录菜单复用，不要求真实
生产认证，因此不构成验收缺口。下一关口是在创建或更新 PR 前再次确认最新 base，并在
最终树上执行 PR Gate。

# Composer 会话配置加载回退进度

## 当前阶段

Stage 3：任务 001、002 已通过主会话审查、独立自动化复验和 dev 关键路径验收。同步最新
基线后，任务 003 已补齐 `main-v3` 遗漏的专家绑定测试会话路径夹具，正在执行最终 PR Gate。

## 基线与工作区

- 基线：最新 `origin/main-v3`，`2f13dcd8`。
- 分支：`codex/fix-composer-session-config-loading`。
- 独立工作树：`D:\Reasonix-worktrees\fix-composer-session-config-loading`。
- 主工作区保持只读，未修改用户现有内容。

## 已确认结论

- 这是 PR #351 引入的已有会话首帧呈现回退，不重新定义产品体系。
- 模型、billing source 和 effort 已在 branch meta/标签持久化；现有 Composer identity
  交接合同遗漏 effort。
- 已有会话入口覆盖标签切换、历史重开和应用重启恢复。
- 激活期间配置立即可见但选择器禁用。
- 权限尚未知时保留目标会话快照；最终未登录/无权继续显示“请选择模型”。
- 不新增存储，不修改 fallback、发送门禁、专家权威回读或主动切换语义。

## 计划交付

- `plan.md`：已完成，包含根因、边界、方案、兼容性、风险、回滚和验证计划。
- `acceptance.md`：已起草真实应用验收矩阵、数据、观察点、预期和失败条件。
- `tasks/001-补齐会话强度快照合同.md`：串行组 A，已完成并通过主会话审查。
- `tasks/002-恢复已有会话首帧呈现.md`：串行组 B，已完成并通过主会话审查及 dev 关键路径验收。
- `tasks/003-补齐专家绑定测试会话路径.md`：同步基线后独立发现的 Gate 修复，已完成代码与回执。
- 用户已授权发布当前工作树 PR；最终 Gate 通过后推送并创建 Draft PR。

## 验证状态

- 任务 001 的保存会话、项目树、运行标签、legacy 缺失 effort 和专家身份合同测试通过。
- `desktop` 主包 `go test .`、前端两个直接合同测试及生产 `tsc --noEmit` 通过。
- 测试专用 TypeScript 全量类型检查仍有多处与任务 001 无关的既有错误，详见任务回执。
- 任务 001 已确认未触碰任务 002 的呈现逻辑；其提交已通过主会话审查。
- 任务 002 的 session snapshot、标签/历史/重启水合、组合原子性、entitlement、switcher
  transition/stable snapshot、reducer/controller 和专家回归测试全部通过。
- 主会话复核发现并已修正权威 empty/auto 被交接期旧 effort 快照覆盖的边界；回归测试确认
  权威 `EffortInfo` 到达后展示不再读取 display-only 快照。
- `desktop/frontend` 的 `pnpm build` 通过；本任务没有 Go 改动，未运行 Go tests。
- 主会话从当前工作树完成 Wails 生产构建；随后按用户授权直接复用现有 dev 环境启动当前
  `kwu-desktop-dev`，没有注入 profile/config 覆盖。
- 真实页面确认：切入保存了 model/effort 的会话时，首个可读取帧即显示完整组合且仅在约
  60 ms 交接期禁用；最终无权/不可用会话仍显示“请选择模型”；连续切换 10 次无串会话。
- 停止并重启当前工作树的开发进程后，活动会话和 `gpt/gpt-5.6-luna · 自动` 正常恢复。
  历史关闭重开、专家团和完整权限三态没有用现有 dev 数据做破坏性造数，继续由聚焦自动化
  覆盖。详细边界和观察值见 `acceptance.md`。
- 任务 003 的 `TestExpertBindingsDelegateToActiveController` 聚焦测试和 `desktop` 主包
  `go test . -count=1` 均通过；修复只补测试夹具，不改变生产代码。

## 阻塞与下一门禁

无产品行为阻塞。最终 PR Gate 首次运行暴露 `origin/main-v3` 已有的专家绑定夹具缺陷；已用
现有会话路径合同修复，等待复验结果。

## 剩余工作

1. 运行任务 003 聚焦测试和最终 PR Gate，更新验证状态。
2. Gate 通过后推送当前分支并创建 Draft PR。

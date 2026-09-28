# 账号框主题表面修复计划

## 问题与目标

创建模式左侧栏底部账号入口默认使用 `var(--fg-faint)` 混合出的浅灰背景，与同一侧栏上方的主题表面不一致。目标是让默认态复用侧栏的 `var(--sidebar-bg)`，完整保留边框。

## 边界

- 只修改 `.sidebar-account-entry` 默认背景。
- 不修改 hover、focus-visible、`--active`、`aria-expanded`、菜单、头像、文案及登录/匿名逻辑。
- 不新增或调整主题 token，不涉及数据、API、迁移和后端。

## 证据与方案

- Codegraph 对索引主工作区的追踪定位到 `desktop/frontend/src/App.tsx`：账号按钮使用 `.sidebar-account-entry`；账号页仅追加 `--active`，展开状态通过 `aria-expanded` 控制箭头；登录状态只影响数据属性、头像和文案。
- 工作树源码确认 `desktop/frontend/src/styles/theme-overrides.css` 中侧栏表面使用 `var(--sidebar-bg)`，账号入口默认背景原为浅灰混合色；hover、focus-visible、active 由后续独立规则覆盖。
- 将默认背景改为 `var(--sidebar-bg)`；保持现有 border 声明和全部状态覆盖规则不变。

## 验收与验证

- 浅色主题下账号入口默认内部与侧栏上方表面一致，边框仍可见。
- hover、键盘焦点、账号页 active、菜单展开及登录/匿名显示保持原行为。
- 契约测试锁定默认背景 token 和边框声明；运行该定向测试与前端生产构建。

## 风险与回滚

风险仅为不同主题下默认态视觉对比变化；`--sidebar-bg` 已由各主题定义。回滚单行背景声明即可恢复。

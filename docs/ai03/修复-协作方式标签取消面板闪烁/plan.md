# 协作方式标签取消面板闪烁修复计划

## 背景与问题

用户通过输入框左下角的新增菜单启用“计划”、“目标”或“省 token”后，在菜单已关闭时点击对应状态标签取消，协作方式面板会短暂闪现后消失。

基线：`origin/main-v3` @ `fb9accb63e907898b73f7c1986c78a8531fc4332`。

## 当前与期望行为

- 当前：状态标签的取消动作无条件调用 `closeIntentMenu`；即使菜单已关闭，也会进入 `closing` 并挂载退场面板。
- 期望：菜单已关闭时取消状态只更新模式，不挂载面板；在菜单内点击选项时仍保留现有退场动画。

## 根因与源码证据

- `desktop/frontend/src/features/composer/ui/Composer.tsx`
  - `choosePlanMode`、`chooseGoalMode`、`chooseTokenMode` 同时被菜单选项和已启用状态标签复用。
  - 三个处理器都先调用 `closeIntentMenu`，未判断 `intentMenuOpen`。
  - `closeIntentMenu` 先设置 `intentMenuClosing=true`，再于下一帧设置 `intentMenuOpen=false`。
- `desktop/frontend/src/components/AnchoredPopover.tsx`
  - `rendered = closing || phase !== "closed"`；因此关闭状态下单独传入 `closing=true` 会挂载面板。
- `desktop/frontend/src/styles/session.css`
  - `[data-state="closing"]` 会播放 `popover-out`；新挂载节点在退场动画中从可见状态移向透明，形成闪烁。
- 当前会话未提供 Codegraph 工具；以最新基线的直接源码读取、测试和渲染证据为准。

## 目标

- 消除关闭菜单状态下取消协作方式时的面板闪烁。
- 保留菜单打开时选择或取消模式的退场动画。
- 用自动化断言固化 `closed -> 不挂载` 和 `open -> closing -> 卸载` 两条时序。

## 非目标与边界

- 不改变计划、目标、省 token 的业务语义、后端同步或持久化。
- 不调整通用 `AnchoredPopover` 的动画时长、定位或其他消费者。
- 不引入生产控制台日志、持久化埋点或新依赖。
- 不改变已启用专家的独立清除行为。

## 选定方案

让协作方式处理器仅在 `intentMenuOpen` 或已处于关闭过程时调用退场流程；菜单已稳定关闭时直接执行模式回调。将共享的“必要时关闭，然后执行”行为收口为小型局部函数，避免三个处理器分别复制条件。

不选择修改 `AnchoredPopover` 的 `rendered` 语义，因为 `closing` 就是其外部控制退场挂载的契约，全局改动会影响模型筛选、工具权限等弹层。

## 影响文件与契约

- `desktop/frontend/src/features/composer/ui/Composer.tsx`：取消协作方式时的菜单退场调度。
- `desktop/frontend/src/__tests__/composer-goal-toggle.test.tsx`：组件 DOM 时序回归。
- 视实际渲染复现需要，最小扩展现有 browser-smoke 测试夹具与用例；不改生产契约。

## 验收标准

1. 面板未打开时，点击已启用的计划、目标或省 token 标签会执行对应取消回调。
2. 上述动作后 `.composer-add-menu` 在当前帧、下一动画帧和退场时长后都不存在。
3. 面板已打开时，选择或取消模式仍经过 `closing` 退场阶段并最终卸载。
4. 计划、目标、省 token 三类标签共享同一修复语义。
5. 前端生产构建通过，无新的相关 console error/warning。

## 验证计划

- 组件测试：直接断言菜单关闭时点击模式标签不产生 `.composer-add-menu`，并保留菜单内选择的关闭断言。
- 渲染验证：使用应用内 Browser 在现有 browser-smoke 页面重放路径，检查 DOM、截图和 console。
- 构建：在 `desktop/frontend` 运行 `pnpm build`。
- 本轮不创建/更新 PR，因此不执行最终 PR Gate；如后续获得 PR 授权，同步最新 `main-v3` 后再执行仓库完整 Gate。

## 风险与回滚

- 风险：过度跳过关闭流程可能让打开菜单的选择动作突然消失；用打开状态的时序测试防护。
- 风险：三类模式回调不一致；共享调度函数并覆盖三类标签。
- 回滚：回退局部调度和对应测试提交即可，无数据迁移、配置或后端兼容负担。

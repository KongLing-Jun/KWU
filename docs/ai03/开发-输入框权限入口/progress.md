# 输入框权限入口进度

## 当前阶段

代码与自动化验证已完成。最终应用验收待当前工作树的 Wails 桌面窗口可稳定启动。

## 已确认决策

- 单一固定宽度入口：权限图标＋当前权限文案；ask、auto、yolo 分别为「需要批准」「自动批准」「完全访问」。
- 权限浮层固定 176px 宽度，三项只展示图标和名称；说明保留在悬停提示和无障碍名称中。
- 所有窗口宽度保留文字，不提供仅图标降级。
- 复用现有三项权限浮层、切换回调、焦点回归和键盘导航。
- 移除菜单的视觉勾选，保留 `aria-checked`。

## 计划范围

- `desktop/frontend/src/features/composer/ui/Composer.tsx`
- `desktop/frontend/src/styles/session.css`
- `desktop/frontend/src/styles/theme-overrides.css`
- `desktop/frontend/src/__tests__/composer-responsive-toolbar.test.tsx`

## 任务

- `tasks/001-统一权限入口.md`：唯一实施任务；不与其它变更并行。

## 验证与下一关

已通过 `pnpm exec tsx src/__tests__/composer-responsive-toolbar.test.tsx`（112/112，覆盖 ask/auto/yolo 的「需要批准／自动批准／完全访问」文案）及 `pnpm build`。内置浏览器已尝试接入 Wails 开发地址；应用正确拒绝无后端的浏览器页面，而本环境的隔离 `wails dev` 启动未留下可控窗口或端口，故不能把实际点击验收标为通过。下一关：在可用桌面窗口中检查固定入口、打开菜单、三项选择及无勾选呈现。

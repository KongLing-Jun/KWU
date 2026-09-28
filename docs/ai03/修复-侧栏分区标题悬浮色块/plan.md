# 侧栏分区标题悬浮色块修复计划

## 问题与预期

`ProjectTree` 的“任务（数量）”和“工作空间（数量）”标题在鼠标悬浮时出现箭头区域与整行叠加的两层灰色色块。预期整行仅绘制一次 `var(--sidebar-hover)`，箭头仍有颜色反馈，键盘 `focus-visible` 仍清晰可辨。

## 目标与边界

- 移除箭头 shell 在父级 hover/focus-visible 下的第二层背景。
- 保留整行 hover 背景、箭头颜色反馈及明确的键盘焦点表现。
- 保持折叠、加号、计数和任务导航功能不变。
- 不引入 transform、filter、backdrop-filter、box-shadow 或大面积合成层。
- 生产代码仅修改 `desktop/frontend/src/styles/overlays.css`；不修改 `theme-overrides.css`、`App.tsx` 或品牌入口文件。

## 证据与根因

- 主工作区 Codegraph 调查已由主代理完成；索引基线由主代理持有。本独立 worktree 不调用 Codegraph。
- 当前 worktree 基线为 `origin/main-v3` 的 `53de5556`，直接源码复核确认 `ProjectTree.tsx` 的 `renderSectionHeader` 同时渲染两个分区标题。
- `.project-tree__task-section-toggle:hover` 为整行绘制 `var(--sidebar-hover)`。
- 其内部 `.project-tree__task-section-arrow-shell` 又在父 hover/focus-visible 下绘制 `color-mix(... 10%)` 背景，形成视觉叠加；这是本问题的直接根因。

## 方案与契约

- 保持 toggle 的 hover 背景不变。
- 将 arrow shell 的 transition 限定为颜色，并在父 hover 下仅提高箭头颜色。
- 使用 toggle 自身的 `:focus-visible` 轮廓表达键盘焦点，不再借箭头背景表达焦点。
- 在现有 `workbuddy-primary-navigation.test.ts` 中增加样式契约，锁定单层背景、箭头颜色反馈和 toggle 的焦点轮廓。

## 验收与验证

- 鼠标悬浮两个分区标题时仅出现一块连续的整行背景。
- 箭头在 hover 时颜色变为 `var(--fg)`。
- 键盘聚焦标题 toggle 时显示清晰轮廓。
- 折叠、加号、计数与导航 DOM/逻辑不变。
- 运行最近相关契约测试、`pnpm build`、`git diff --check`，并检查 EOL 与工作区状态。

## 风险与回滚

改动仅触及局部 CSS 与静态契约测试，无数据、接口或迁移风险。若需回滚，回退对应样式与测试提交即可。

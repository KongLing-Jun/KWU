# 修复会话标题字形裁切

## 问题与预期

Windows Desktop 的 Creation 布局中，会话页顶部标题的中文字形顶部会被裁掉；侧栏中的同名会话标题正常。标题应保持单行、完整显示上下笔画，仅在横向空间不足时显示省略号。

## 根因与证据

- Codegraph 索引调查基线：主工作区 `main-v3@70d919f4c50ca81f18952bc90c3817e9d9d78af8`；索引开启实时文件同步。
- 实现 worktree 基线：最新 `origin/main-v3@61729d22a39c31cc79b665ff50913081df0e9d10`。独立 worktree 不使用 Codegraph 结论替代源码与运行验证。
- `desktop/frontend/src/App.tsx` 中 `topicbarTitle` 直接来自活动页签标题，并完整写入标题按钮；不存在字符串裁剪。
- `desktop/frontend/src/styles/overlays.css` 中 `.topicbar__identity` 使用 `overflow: hidden`。
- `desktop/frontend/src/styles/theme-overrides.css` 在 Creation 布局把 `.topicbar__title-row` 上移 `3px`；上移后的绘制区域超出父级裁切边界。
- 同一路径把标题缩至 `10px`，且 `h1` 与内部按钮均保留裁切规则，在 Windows 高 DPI 与中文 fallback 字体下放大了缺陷。
- 侧栏标题没有负位移，使用明确行高，因此不受影响。
- 该组合由 `ef12be0e` 引入；现有测试只检查声明和组件结构，没有验证字形可视边界。

## 目标与边界

目标：消除顶部会话标题的纵向裁切，同时保留重命名交互、单行横向省略、标题栏布局和右侧操作按钮。

非目标：

- 不改变标题数据、重命名协议、会话状态或后端。
- 不重做 Topicbar、侧栏或全局排版。
- 不修改字体偏好、主题协议或 Code Guard baseline。

## 方案

优先移除标题行依赖负 `transform` 的垂直定位，使用正常布局对齐或局部 padding/line-height 提供稳定字形空间。横向省略只作用于需要截断的文本边界。若必须调整裁切属性，应保持 Topicbar 不横向溢出并说明理由。

预计影响：

- `desktop/frontend/src/styles/theme-overrides.css`
- 直接相关的 Desktop 前端样式合同测试；仅在测试需要稳定 DOM 时触及测试夹具。
- 本目录任务回执与 `progress.md`。

## 验收标准

1. Windows Creation 布局中，短中文、含上下伸部的拉丁字母及长标题均不发生纵向裁切。
2. 长标题保持单行并横向省略，不挤压复制、导出、帮助和右侧面板按钮。
3. 标题点击重命名、编辑态输入框、空会话标题和非 Creation 布局保持现状。
4. 侧栏标题与会话内容布局无变化。
5. 相关前端测试和 `pnpm build` 通过，Git diff 无空白错误，EOL 未发生机械变化。

## 验证计划

- 先添加能在旧样式上失败的聚焦样式合同，约束 Creation 标题不得通过负位移进入裁切父级，并保留横向省略。
- 运行聚焦测试与 `pnpm build`。
- 使用实际构建页面或最接近的浏览器环境，在 Windows/高 DPI 等效视口检查短中文和长标题。
- 主会话独立审查 commit、diff、EOL 和测试证据；最终 PR 前再同步基线并运行项目规定 PR Gate。

## 风险与回滚

主要风险是标题垂直位置变化或长标题挤压操作区。通过限制改动到 Creation Topicbar 选择器和覆盖短/长标题场景控制风险。回滚时可完整撤销该修复提交，不涉及数据迁移。

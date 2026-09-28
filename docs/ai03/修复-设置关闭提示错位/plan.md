# 设置关闭提示错位修复计划

## 1. 问题与预期

桌面端以弹窗形式打开设置中心后，鼠标悬浮右上角关闭 `×`，文字提示“关闭”会出现在
面板顶部中间，而不是关闭按钮附近。用户已确认不需要该文字提示。

期望行为：设置弹窗右上角仍显示并可操作关闭 `×`；鼠标悬浮或键盘聚焦该按钮时不显示
视觉 Tooltip；按钮原有悬浮高亮、点击关闭、禁用态和无障碍名称保持不变。

## 2. 基线、当前行为与根因

- 计划基线：`origin/main-v3@b64ede81b89cd77a0cdd946a26fbf45abdd77e77`。
- 主工作区 Codegraph 索引导航与同一提交上的独立 worktree 源码复核一致：
  `SettingsPanel.tsx` 在非嵌入模式渲染 `ModalCloseButton`，并把绝对定位类
  `settings-modal__close` 传给按钮。
- `ModalCloseButton.tsx` 无条件使用共享 `Tooltip` 包裹内部 button，并以 `label` 同时提供
  Tooltip 内容和 `aria-label`。组件仅支持禁止聚焦触发，不支持彻底关闭视觉 Tooltip。
- `Tooltip.tsx` 把外层 `.tooltip-trigger` span 作为定位锚点；设置弹窗的
  `settings-modal__close` 只作用于内部绝对定位 button。因锚点和实际按钮不是同一元素，
  悬浮后 Tooltip 根据留在正常文档流中的 wrapper 计算位置，形成截图中的顶部居中错位。
- `ModalCloseButton` 还有多个弹窗和抽屉消费者；直接移除共享组件的 Tooltip 会改变这些
  未在本需求范围内的界面。Codegraph 未找到该共享组件的直接交互测试，现有 Vitest、
  Testing Library 和 jsdom 基础设施可覆盖这次新增契约。
- 工作树源码与索引基线相同；后续不在独立 worktree 重新调用 Codegraph，修改以工作树
  直接源码、测试、构建和实际 Git diff 为准。

## 3. 目标与非目标

### 目标

- 设置弹窗关闭按钮在鼠标悬浮和键盘聚焦时均不显示“关闭”Tooltip。
- 保留关闭按钮可访问名称，使辅助技术仍能识别为“关闭”。
- 保留按钮位置、尺寸、图标、悬浮高亮、焦点样式、禁用态和点击关闭行为。
- 为共享 `ModalCloseButton` 提供显式、默认兼容的 Tooltip 开关，仅由设置弹窗关闭入口
  选择关闭。

### 非目标

- 不调整 Tooltip 的通用定位算法、Portal、延迟、箭头或样式。
- 不取消其他弹窗、抽屉或图标按钮现有的 Tooltip。
- 不改变设置中心的布局、导航、关闭动画、遮罩点击、反馈提交保护或嵌入模式。
- 不修改文案翻译、后端、数据、依赖、Go/Wails 契约或 CSS 定位值。

## 4. 验收标准

| ID | 场景 | 预期 |
| --- | --- | --- |
| AC-01 | 打开非嵌入式设置弹窗并悬浮右上角 `×` 超过 Tooltip 延迟 | 页面不出现“关闭”视觉 Tooltip |
| AC-02 | 使用键盘聚焦设置弹窗 `×` | 页面不出现“关闭”视觉 Tooltip，焦点样式仍可见 |
| AC-03 | 查询或使用辅助技术访问 `×` | button 仍有本地化的“关闭”可访问名称 |
| AC-04 | 点击可用的 `×` | 继续触发现有 `requestClose`，弹窗按原动画关闭 |
| AC-05 | 反馈正在提交导致按钮禁用 | 禁用状态及防止关闭的行为保持不变 |
| AC-06 | 打开其他使用默认 `ModalCloseButton` 的界面 | 其原有 Tooltip 行为不变 |
| AC-07 | 范围 | 无 CSS、后端、依赖及 `scripts/code_guard_baseline.json` 变更 |

## 5. 选定方案与影响范围

在 `ModalCloseButton` 增加布尔属性（命名在任务契约中固定），默认保持显示 Tooltip；将该
属性映射到现有 `Tooltip.disabled`，不复制 Tooltip 状态或定位逻辑。设置弹窗的单个调用点
显式关闭 Tooltip。`label` 继续用于内部 button 的 `aria-label`，不能因为不显示视觉提示而
删除。

该方案比修改通用定位算法更符合已确认需求：用户接受设置关闭按钮没有提示，且无需让
其它消费者承担定位行为变化。保留 Tooltip wrapper 结构还可避免这次修复顺带改变 DOM
布局和现有 CSS 选择器。

预计影响文件：

- `desktop/frontend/src/components/ModalCloseButton.tsx`：新增默认兼容的 Tooltip 开关，
  保留 `label` 到 `aria-label` 的映射。
- `desktop/frontend/src/features/settings/ui/SettingsPanel.tsx`：仅在非嵌入式设置弹窗关闭
  按钮上关闭视觉 Tooltip。
- `desktop/frontend/src/vitest/modal-close-button.vitest.test.tsx`（或同等聚焦测试文件）：
  覆盖关闭 Tooltip 后的悬浮、聚焦、可访问名称和点击；同时覆盖默认行为回归。
- 本主题任务回执与 `progress.md`：记录实施边界和验证结果。

不通过传入空 `label` 规避 Tooltip：那会同时丢失默认 `aria-label`，破坏无障碍契约。
不把 `showTooltipOnFocus=false` 当作完整修复：它只禁止聚焦触发，鼠标悬浮仍会复现问题。

## 6. 兼容性、风险与回滚

该修复只增加一个可选前端组件属性，默认值保持现有消费者行为，无数据或接口迁移。

| 风险 | 控制 |
| --- | --- |
| 误将无障碍名称与视觉 Tooltip 一起删除 | 保留必填 `label` 与 `aria-label` 映射，并用可访问查询断言 |
| 默认值变化导致所有关闭提示消失 | 新属性默认启用 Tooltip，增加默认路径回归测试 |
| 仅禁止键盘提示但鼠标仍出现 | 使用完整 Tooltip 禁用开关，分别测试悬浮与聚焦 |
| 设置弹窗关闭行为受影响 | 不改 `requestClose`、disabled 或事件传递，测试点击回调并做实际应用验收 |
| wrapper DOM 调整引发布局变化 | 复用现有 wrapper，只禁止 Tooltip 激活，不改 CSS |

若出现回归，回退可选属性、设置调用点和对应测试即可；无数据回滚。

## 7. 验证计划

实施迭代先运行新增的聚焦 Vitest，再运行前端生产构建：

```powershell
pnpm exec vitest run --config vitest.config.ts src/vitest/modal-close-button.vitest.test.tsx
pnpm build
```

应用验收优先在实际桌面应用中执行：打开设置弹窗，分别用鼠标悬浮和键盘聚焦右上角
`×`，确认无“关闭”提示且视觉状态正常；点击按钮确认弹窗关闭；再抽查一个默认使用
`ModalCloseButton` 的界面，确认其 Tooltip 未被全局取消。若内置浏览器无法覆盖 Wails
原生窗口，则使用 Computer Use；只有工具无法可靠执行时才请用户复核。

完成应用行为验证后，运行前端构建和项目 common checks。获得 PR 授权后，先同步最新
`main-v3`、复跑关键交互和聚焦检查，再执行：

```powershell
node ./scripts/pr-gate.mjs --base origin/main-v3 --run all
```

按路径分类，本改动只涉及前端与文档，预期运行 `pnpm build`，不运行 Root Go 或 Desktop
Go 测试；最终以门禁分类摘要为准。未获得 PR 授权前不推送、不创建或更新 PR。

## 8. 未决事项与阶段门

产品行为、范围、兼容性、风险与回滚决策均已确认，无产品未决事项。当前只完成 Stage 1
计划文档；计划经用户批准且按仓库流程落地后，再从最新基线创建 Stage 2 任务拆解。除非
用户明确授权堆叠执行，否则等待计划 PR 合并后再拆解任务，并在任务拆解获批后另行取得
实施授权。若调查发现外部行为、接口或影响范围超出本计划，停止并重新进入确认流程。

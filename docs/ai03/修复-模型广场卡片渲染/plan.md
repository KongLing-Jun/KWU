# 模型广场卡片渲染修复计划

## 背景与证据

模型广场的全部模型详情卡在悬浮时执行整卡位移和阴影变化，并同时过渡背景、边框、
位移与阴影；可切换模型卡虽没有动画，但缺少键盘聚焦和忙碌态的明确视觉规则。在低
性能机器上，大量卡片同时存在时，整卡合成与绘制效果会放大交互开销。

- 初始实施基线：`origin/main-v3@76592ef471cdb5a54a009a0f8d4e01dc098c5a80`；筛选控件
  扩展实施前已合并同步至 `origin/main-v3@6d0a5171ff0cf3e1edcab7db7fb036742b53a7ca`。
- Codegraph 导航来自只读主工作区 `main-v3@8ff823f69bc77d7ea8cad6b0c758d70a2535c38e`；
  主工作区比实施基线更新，但相关组件与样式规则已在独立工作树直接复核。
- `ModelMarketplace.tsx` 渲染两类卡片：全部模型详情卡
  `.model-marketplace-card`，以及当前可切换卡
  `.model-marketplace-selectable__card`。
- 详情卡只打开外部定价页，不具有选中、不可用或推荐状态。厂商、套餐和分层价格只是
  内容与筛选数据。
- 可切换卡具有 `--current` 和 `disabled` 属性；正常数据流会过滤配置中已禁用的模型，
  因而页面上的 `disabled` 主要表示正在切换的 busy 状态。
- `features.css` 的详情卡当前使用 `box-shadow`、`transform: translateY(-1px)` 和覆盖
  四类属性的 transition；样式契约测试还反向要求该位移存在。
- 能力卡片已使用主题表面与视觉风格色边框表达普通交互，本修复沿用该静态视觉语言。

## 目标与边界

- 两类模型卡默认使用主题表面。
- 普通可交互卡悬浮或 `:focus-visible` 时即时使用轻量主题色表面与 `var(--accent)` 边框。
- 当前模型持续使用更强的主题色表面与边框，普通悬浮或聚焦不得覆盖它。
- 普通 disabled/busy 卡使用中性主题表面，且不响应悬浮或聚焦；若 current 同时进入 busy，
  仍保持 current 的主题色表面与边框，但同样不响应普通交互。
- 整卡规则不使用 `transform`、`filter`、`box-shadow`、`opacity` 动画、`will-change`、
  `animation` 或大面积 `transition`。
- 保留详情卡的 `content-visibility` 与 `contain-intrinsic-size` 离屏优化。
- 厂商与套餐共用筛选触发器的展开/聚焦态和弹层使用主题浅表面与 accent 边框；普通
  选项 hover/focus 只改变背景与边框，选中项持续保持主题色。
- 筛选弹层不使用重阴影，筛选控件不使用 transform、filter、opacity 或动画过渡。
- 筛选弹层通过 `createPortal(document.body)` 脱离触发器所在的局部视觉风格作用域；打开
  前从触发器读取已计算的 accent/surface/border/fg token，并以内联自定义属性转交专属
  弹层，避免回退到根级主题色。

本次只修改 `desktop/frontend/src/styles/features.css`、`MarketplaceFilterSelect.tsx`、必要
的模型广场样式/组件契约测试及本主题文档。组件只允许在打开前转交触发器已计算主题
token；不改模型数据流、套餐/供应商筛选逻辑、键盘交互或结构，不引入推荐态，不改 Go、
Wails、共享 `AnchoredPopover`、依赖、其他页面或 `scripts/code_guard_baseline.json`。

## 状态优先级与验收

视觉优先级为 current（包括 current+busy）> 普通 disabled/busy > 普通 hover/focus > 默认；
交互上 disabled/busy 始终不响应 hover/focus。厂商、套餐、计费来源和价格层级不参与整卡
状态竞争。

| 场景 | 预期 |
| --- | --- |
| 详情卡默认 | 主题表面，无阴影、位移或整卡动画 |
| 详情卡悬浮/键盘聚焦 | 主题浅色表面和 `var(--accent)` 边框即时生效 |
| 普通可切换卡悬浮/键盘聚焦 | 同上，不改变布局或合成层 |
| 当前可切换卡 | 持续保持更强主题色表面和边框，hover/focus 不降级 |
| 普通 busy/disabled 可切换卡 | 中性浅色、禁用指针，不命中 hover/focus 规则 |
| current+busy 可切换卡 | 保持 current 主题色表面和边框，同时不命中 hover/focus 规则 |
| 厂商/套餐触发器展开或聚焦 | 主题浅表面与 accent 边框，无阴影或动画 |
| 筛选弹层 | 主题浅表面与 accent 边框，无重阴影和开合动画 |
| 普通/选中筛选项 | 普通项仅背景/边框反馈；选中项持续保持主题色 |
| 局部视觉风格中的 Portal 弹层 | accent/surface/border/fg 与触发器已计算 token 一致 |
| 范围 | 组件、筛选、排序、选择行为和外链行为不变 |

## 风险、回滚与验证

主要风险是中性 disabled 规则覆盖 current+busy，或普通交互覆盖 current/disabled，以及
移除焦点阴影后键盘焦点不明显。通过 `:not(:disabled)` / `:not(--current)` 限定普通规则，
并让中性 disabled 规则排除 current；accent 边框保留明确的键盘焦点表达。CSS 不涉及
数据迁移；回滚本任务提交即可恢复原样。

验证命令：

```powershell
pnpm exec tsx src/__tests__/model-marketplace-style-contract.test.ts
pnpm exec tsx src/__tests__/model-marketplace.test.tsx
pnpm build
```

最终检查工作树状态、实际 diff 与 EOL。未获授权不推送、不创建或更新 PR。

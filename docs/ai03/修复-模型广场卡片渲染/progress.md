# 模型广场卡片渲染修复进度

## 当前状态

| 项目 | 状态 |
| --- | --- |
| 当前阶段 | Stage 3：实现、自动校验与应用验收完成 |
| 计划 | [`plan.md`](./plan.md) |
| 任务 | [`tasks/001-统一模型卡片轻量悬浮.md`](./tasks/001-统一模型卡片轻量悬浮.md) |
| 基线 | 已合并同步 `origin/main-v3@6d0a5171ff0cf3e1edcab7db7fb036742b53a7ca` |
| 执行方式 | 单任务串行；产品状态优先级已确认 |
| PR | 用户已授权创建 Draft PR；不自动合并 |

## 已确认决策

- 范围包含全部模型详情卡和当前可切换模型卡。
- 普通 hover/focus 使用轻量主题表面与 `var(--accent)` 边框，不使用整卡合成动画。
- current（包括 current+busy）持续保持主题色表面；普通 disabled/busy 使用中性表面；
  所有 disabled/busy 都不响应普通 hover/focus。
- 不增加推荐态，不改变套餐、厂商、计费来源、筛选、排序或选择行为。
- 视觉修复允许修改 `features.css`、必要测试和本主题文档；应用验收发现 Portal 主题断裂
  后，追加允许 `MarketplaceFilterSelect.tsx` 仅转交触发器已计算 token。
- 用户追加批准厂商/套餐共用筛选控件的视觉修复：展开触发器、弹层和选中项保持主题
  表面/边框，普通项只做低成本背景/边框反馈，移除阴影和动画；筛选逻辑与组件不变。

## 验证与下一关口

- 扩展后的样式契约测试 21/21、模型广场组件测试 44/44 通过。
- `pnpm build` 通过 CSS 语法、z-index token、TypeScript 编译和 Vite 生产构建；只有既存
  的大 chunk 与插件耗时警告。
- 实际 diff 只在筛选组件打开前读取并转交主题 token；未修改数据流、套餐/厂商语义、
  共享弹层或 code guard baseline。
- 复合 current+busy 状态已补充静态契约，确保中性 disabled 规则不会覆盖 current 表面。
- 筛选控件定向测试与生产构建已完成；用户已在内部浏览器完成应用视觉验收并授权创建
  Draft PR。
- 首轮应用验收在金色视觉风格下发现：触发器 accent 为 `rgb(201, 154, 58)`，Portal
  弹层与选中项却回退为根级 `rgb(255, 90, 44)`；已定位为 `createPortal(document.body)`
  脱离局部主题作用域。专属 token 转交已完成，运行时契约确认 Portal 接收触发器的
  `rgb(201, 154, 58)` 及对应 surface/border/fg；内部浏览器应用复验已通过。

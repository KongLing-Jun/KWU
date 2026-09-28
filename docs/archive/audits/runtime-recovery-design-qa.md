# 运行时断线恢复 Design QA

- source visual truth: 未提供；现有账户页截图不包含断线或崩溃状态，不能作为该状态的视觉基准。
- implementation screenshot:
  - [`runtime-connection-overlay-1120x800.png`](../../../artifacts/design-qa/runtime-connection-overlay-1120x800.png)
  - [`runtime-recovered-1120x800.png`](../../../artifacts/design-qa/runtime-recovered-1120x800.png)
- viewport: `1120 × 800`
- device pixel ratio: `1`
- state:
  - Wails 桌面后端被精准停止，Vite 页面继续运行。
  - 后端恢复后页面自动重新加载并回到可交互主界面。
- full-view evidence: 两张截图均为完整视口截图。
- focused-region evidence: 断线提示位于完整视口中央，完整截图已经覆盖目标区域，因此未另做裁剪。

## Findings

- 断线状态使用现有浅色设计令牌，背景内容降低对比度但仍能保留上下文。
- 标题、说明文字居中，层级清楚；提示不包含技术错误或用户无法执行的操作。
- 遮罩使用 `role="status"`、`aria-live="polite"` 和 `aria-busy="true"`。
- 服务恢复后遮罩消失，主界面布局没有出现位移、裁切或残留错误层。
- 恢复后的浏览器控制台未新增 `InvalidStateError`、空 WebSocket `.send`、Hook 顺序或除零错误。

## Iteration history

1. 初版断线遮罩能够阻止 React 崩溃并持续探测后端。
2. 首次恢复测试发现 Vite 早于 Wails 后端启动时会过早重载，进入“后端必需”错误页。
3. 调整为绑定缺失时继续显示恢复遮罩并等待真实 Wails transport；后端可用后再重载。
4. 复测通过：断线提示出现，后端恢复后页面自动回到主界面，控制台无目标错误。

final result: blocked

Blocker: 用户未提供该断线/崩溃状态的目标设计稿、Figma 节点或参考截图，无法按 Design QA 合同执行同视口像素级来源对比；功能与实现侧视觉检查已完成。

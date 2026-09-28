# 任务 001：实现单 Transcript 虚拟化与多会话验收

## 状态与交付

- 状态：已完成。
- 执行组：Serial-01；不委派子代理。
- 依赖：已批准的 [`plan.md`](../plan.md)。
- 交付：一个可独立评审、测试和回滚的前端纠正提交，包含源码、测试、文档回执。
- 用户已授权文档、任务、代码与实际验收连续完成；未授权推送或 PR。

## 目标

在保留 PR #309 会话状态隔离的前提下，删除多 Transcript DOM 常驻策略，使用
单一 Transcript 和视口虚拟化稳定多会话快速切换，同时让 Composer 壳体固定、
目标草稿首帧直接恢复。

## 允许范围

- `desktop/frontend/src/App.tsx`
- `desktop/frontend/src/components/Transcript.tsx` 及必要的小型相邻组件
- `desktop/frontend/src/features/session/useController.ts`
- `desktop/frontend/src/features/composer/ui/Composer.tsx`
- `desktop/frontend/src/features/session/sessionViewSnapshot.ts`
- `desktop/frontend/src/styles/session.css`
- 直接相邻前端测试、Playwright fixture/spec
- 本计划目录中的进度与任务回执

## 禁止范围

- PR #309 的 Go 会话 sidecar、模型路由、Fork/Branch 契约。
- 后端历史协议、数据库、网络接口、模型 fallback、凭据和订阅行为。
- 新依赖、完整 DOM LRU、无限缓存、磁盘草稿持久化。
- `scripts/code_guard_baseline.json`、无关格式化或用户其他 worktree。

## 实现约束

1. 任意时刻只存在一个 Transcript 实例；切换不保留隐藏完整 DOM。
2. 数据快照与 DOM 生命周期分离；缓存数据可立即呈现但必须虚拟挂载。
3. Composer 仍为单实例；draft key 在 layout 阶段切到目标会话，过渡时只禁用。
4. 虚拟行使用稳定 key、动态高度测量和有限 overscan，不以固定轮次数量充当
   DOM 上限。
5. 问题导航和滚动锚点必须能先挂载目标虚拟行再进行像素校正。
6. 空会话、冷缓存、运行态、专家 AgentWorkspace 与工具组原有语义不变。
7. 不以扩大 WebView 内存、关闭错误保护或降低验收强度规避崩溃。

## 源码依据

- `SessionViewSnapshotCache` 已缓存 items、标题、指纹和滚动锚点，默认上限 8。
- `Composer` 已有完整草稿快照与 `useLayoutEffect` 恢复，App 遮罩造成可见延迟。
- `Transcript` 当前 `HOT_TURNS=30`，12-turn 工具密集会话会完整构建全部热区节点。
- `@tanstack/react-virtual` 已用于 Workspace/Diff，可直接复用。
- 失败实验在超过 4 个会话后返回淘汰长会话稳定崩溃，证明完整 DOM 缓存不可用。

## 验收场景

1. 缓存命中：点击目标会话后目标历史和草稿首帧呈现，后台激活不重置正文。
2. 冷缓存：Composer 固定；正文轻量加载后呈现，不出现大块 Composer 骨架。
3. 草稿：A 输入草稿，B 不串稿，返回 A 直接显示原草稿。
4. 压力：6+ 会话快速随机切换 100 次，包含长会话被回访，无 renderer 退出。
5. 导航：长会话问题导航、跳到底部、滚动返回和加载更早历史正常。
6. 工具：折叠工具首次展开内容完整，关闭释放，切换后仍可再次展开。
7. 内存：记录 renderer PID、工作集和强制 GC 后活堆；进程保持存活、活堆有界，
   并单独披露 Windows Graphics Capture 导致的工作集保留。

## 聚焦验证

```powershell
Push-Location desktop/frontend
pnpm exec tsx src/__tests__/composer-session-draft.test.tsx
pnpm exec tsx src/__tests__/tab-switch-hydration.test.tsx
pnpm exec tsx src/__tests__/app-chrome-tabs.test.ts
pnpm exec tsx src/__tests__/transcript-scroll-restore.test.tsx
pnpm exec tsx src/__tests__/tool-card-running.test.tsx
pnpm exec playwright test --config playwright.config.ts
pnpm exec tsc --noEmit -p tsconfig.playwright.json
pnpm check:css
pnpm typecheck
pnpm build
Pop-Location
go run ./scripts/code_guard
git diff --check origin/main-v3...HEAD
```

根据实际测试脚本名称调整命令，但必须覆盖同等行为。前端变更不运行 Go 测试。

## 完成回执

- 状态：完成；未推送、未创建 PR。
- 变更文件：`App.tsx`、`Transcript.tsx`、Composer/session snapshot/controller、
  `session.css`、相邻测试和 browser smoke fixture。
- 自动化结果：10 个聚焦脚本共 391 项断言通过；Playwright 10/10；
  Playwright TypeScript 与生产构建通过。
- 原生应用结果：dev profile 真实历史 9 会话最终代码快速切换 100 次，无白屏、
  renderer 退出或新 Crashpad；目标草稿直接恢复且不串稿；长会话只有 5 个虚拟行。
- 内存回执：强制 GC 后活堆 142MB、总堆 215MB；逐次 Windows 截图使 renderer
  工作集保留约 1.10GB，未伴随活堆线性增长或崩溃。
- 偏差与剩余风险：虚拟化阈值由最初 12 调为 10，避免 2 轮短会话进入虚拟化，
  同时覆盖真实 12 轮工具密集会话；专家 AgentWorkspace 保持原布局。
- 交接：dev 应用保持打开；用户可按计划中的 9 会话序列复验，重点观察 Composer
  不出现骨架、草稿恢复、历史不重复加载和长会话返回。

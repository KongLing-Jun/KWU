# 任务 001：统一 Transcript 等待表面

## 1. 任务信息

- 状态：completed
- 串并行组：Serial-1，任务 002 的共享视觉依赖
- 任务到 PR：本任务与 plan/OpenSpec 组成一个独立、可回滚的前端 UI follow-up；未经授权不 push 或创建/更新 PR
- 执行者：主会话（当前规则未授权子代理）
- 实现基线：当前分支已 merge 最新 `origin/main-v3`，同步后聚焦基线全绿

## 2. 目标与预期产物

把已有会话首次水合、专家 sidecar 等待、deferred expert return、persisted expert hidden restore 与普通长会话
hidden restore 的正常等待反馈收敛为一个共享展示组件和一个 Transcript 约 180ms 延迟所有者。删除三块模拟消息
placeholder 及 shimmer；保留稳定外壳、即时保护、统一文案和独立失败面板。

预期产物：

- 精确旧红证明现实现仍有三块骨架、120ms 可见阈值和跨 presentation 重复等待所有权。
- 最小前端实现与聚焦回归。
- clean 精确 HEAD Desktop 普通冷加载、专家返回、普通长会话返回验收证据。
- 本文完成回执、OpenSpec tasks 与 progress 更新。

## 3. 依赖与证据

- 依赖 [`plan.md`](../plan.md) 已确认的产品决策。
- 依赖当前分支已完成的 persisted-only、single-seek、hidden/inert、stable reveal 与 failure recovery；不得拆掉或复制这些状态机。
- `Transcript.tsx` 当前首次加载使用 `.transcript__session-placeholder` 三块骨架，另有专家/普通等待 JSX。
- `session.css` 当前拥有 placeholder card、响应式尺寸和 shimmer。
- `useController.ts` 的 surface hint 仍以 120ms 控制已有会话提示；Transcript 恢复等待则立即显示，所有权未统一。
- 同步后旧生产基线：expert `213/213`、restore `21/21`、anchor `32/32`、existing loading `53/53`。

## 4. 允许与禁止范围

### 允许修改

- `desktop/frontend/src/components/Transcript.tsx`
- 新增一个只负责视觉的共享 session loading presentation 组件
- `desktop/frontend/src/styles/session.css`
- `desktop/frontend/src/App.tsx`
- 只有证明消除双延迟必需时，允许最小修改 `desktop/frontend/src/features/session/useController.ts`
- 直接相关测试：
  - `desktop/frontend/src/__tests__/existing-session-loading-presentation.test.tsx`
  - `desktop/frontend/src/__tests__/expert-workspace-virtualization.test.tsx`
  - `desktop/frontend/src/vitest/transcript-performance-diagnostics.vitest.test.tsx`
  - 必要时直接相关的 App/Controller hydration fixture
- 本任务回执、progress 与 `openspec/changes/unify-desktop-session-loading-presentation/tasks.md`

### 禁止修改

- Transcript 虚拟器、row measurement、overscan、语义锚点算法、single seek、stable/hard-limit 与 cache 写回。
- 专家 sidecar、persisted/legacy 判定、Set/Clear 事务、Go/Wails API 或持久化。
- Welcome、Composer、成员栏、标题栏、错误文案、locale key 与依赖。
- `scripts/code_guard_baseline.json`、其他加载/滚动问题或 OpenSpec archive/sync 清理。

## 5. 必须保持的行为

- 普通/专家 hidden restore 从首帧 `aria-hidden`、inert、不可交互，稳定后只揭示一次。
- 每 restore generation virtual seek `≤1`；canonical identity/offset 与 bottom 语义不变。
- pending→persisted 不出现 legacy presentation。
- wheel/pointer/touch/keyboard/reveal 可取消旧恢复，旧 RAF 不回抢。
- sidecar error 与 hard-limit failure 立即显示 alert；retry/show-current 的 pointer/touch/Space 合同不变。
- blank creation 会话不显示已有会话加载态；Composer 布局与发送门禁不变。

## 6. 实现约束

1. 先只改测试得到精确旧红，不以源码搜索替代失败证据。
2. 正常等待 activity 必须在 Transcript 层连续聚合；presentation 分支切换不得重新计时。
3. 等待保护根表面立即挂载，约 180ms 前不挂载可见 label，也不设置 live `role="status"`。
4. 延迟后只存在一个 spinner、一个 `transcript.loadingSession` 文案和一个 polite status；共享组件不得读取 Controller 或导航状态。
5. 正常等待组件不得接管失败控件或锚点/presentation 决策。
6. reduced-motion 下 spinner 不持续旋转；placeholder 和 shimmer 代码完全删除。
7. React render 阶段不得把未提交 loading 状态写入业务 ref；timer cleanup 必须覆盖 view change/unmount。

## 7. 验收标准

| ID | 验收结果 |
| --- | --- |
| T-01 | 冷加载首帧保护表面存在，但 179ms 内无 spinner、文案、status 或骨架 |
| T-02 | 约 180ms 后只有一个居中 spinner、统一文案和 polite status |
| T-03 | 快速完成从未显示或播报 feedback，真实历史原子替换等待表面 |
| T-04 | expert pending→persisted hidden 连续等待不重置阈值且不暴露 legacy/不稳定正文 |
| T-05 | 普通/专家语义恢复的 hidden/inert、single seek、30 stable、closest/exact 与 cache 合同不变 |
| T-06 | failure alert/controls 即时可用，正常 overlay 不截获输入 |
| T-07 | 无 placeholder card、响应式骨架规则或 shimmer；reduced-motion 保持正确 |
| T-08 | 共享组件可供任务 002 复用，但不拥有 active tab、导航或恢复业务状态 |
| T-09 | 生产 diff 不触及禁止范围，OpenSpec strict、diff/EOL/status clean |

## 8. 聚焦验证

迭代顺序：

1. `pnpm exec tsx src/__tests__/existing-session-loading-presentation.test.tsx`
2. `pnpm exec tsx src/__tests__/expert-workspace-virtualization.test.tsx`
3. `pnpm exec tsx src/__tests__/transcript-scroll-restore.test.tsx`
4. `pnpm exec tsx src/__tests__/transcript-scroll-anchor.test.ts`
5. diagnostics Vitest 精确文件
6. `pnpm typecheck`
7. `pnpm build`
8. OpenSpec strict、`git diff --check`、EOL/status

应用验收必须使用同步后的 clean 精确 HEAD 新构建 Desktop，至少覆盖：普通冷加载、专家 single-wheel 返回、普通长
single-wheel 返回；DEV trace 要求 visible-jump/incomplete 为 0、seek 不增加，肉眼确认仅一个加载反馈。

## 9. 完成回执

| 项目 | 结果 |
| --- | --- |
| 最终状态 | 完成；共享等待表面、延迟所有权、聚焦回归与统一原生验收均已收口 |
| 实际修改文件 | `Transcript.tsx`、共享 `SessionLoadingPresentation.tsx`、`App.tsx` 加载事实输入、`session.css` 及三份直接回归测试 |
| 旧红证据 | existing loading 49/57，8 个失败精确覆盖首帧/179ms 已播报、三块 placeholder 与 shimmer 未删除；其余 Welcome/零轮场景全绿 |
| 自动化验证 | existing loading 57/57、expert 217/217、restore 21/21、anchor 32/32、diagnostics 2/2；`test:typecheck` 仅未改文件既有错误，不含任务文件 |
| 应用验收 | clean HEAD `7793238a...` PASS：专家与普通长会话的新建事务都显示同一全屏等待表面；延迟达到阈值时只出现一个“正在加载会话…”状态，快速普通 B 在文案阈值前完成；等待期间无旧正文、Composer 或主工作区交互 |
| 偏差 | 最新止损 HEAD 的 `pnpm typecheck` 与完整 `pnpm build` 已通过；未运行与本次止损无关的完整前端测试矩阵 |
| 剩余风险与交接 | 共享等待视觉只用于已有会话；任务 002 不得再把它复用为新建会话遮罩。此前原生证据见 `C:\Users\ai03\AppData\Local\Temp\reasonix-new-session-atomic-20260811-172635` |

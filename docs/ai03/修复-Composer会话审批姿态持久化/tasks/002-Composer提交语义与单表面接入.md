# 002 Composer 提交语义与单表面接入

## 状态

已完成

## 目标与产物

让普通 Composer 和项目单表面 Composer 仅在后端持久化成功后更新可见姿态；移除 Composer 对全局新会话默认的写入，并在失败时保持旧 UI、显示错误。

产物：异步成功后提交交互、项目 adapter 接入、错误展示、前端契约测试。

## 依赖与分组

- 依赖任务 001 的返回错误和事务后端。
- 串行组 B；完成后任务 003 执行集成验收。
- 与任务 001/003 计划映射到同一实现 PR 机会；当前不创建 PR。

## 源码证据

- `desktop/frontend/src/App.tsx:applyToolApprovalMode` 当前先 patch profile 并写 `toolApprovalDefaultPreference`。
- `desktop/frontend/src/features/session/useController.ts:setToolApprovalMode` 当前吞错。
- `renderProjectComposer` 对已准备 tab 仍只 patch adapter，直到 send 才同步后端。
- `Composer.tsx` callback 当前声明为同步 `void`，但菜单选择可接受异步调用并由父级管理反馈。

## 允许范围

- `desktop/frontend/src/App.tsx`。
- `desktop/frontend/src/features/session/useController.ts`、bridge/types、Composer callback 类型。
- 与审批姿态直接相关的前端测试和最小错误文案。
- 更新本任务回执与 `progress.md`。

## 禁止范围与保持行为

- 设置页 `SetDefaultToolApprovalMode` 行为保持不变。
- 不改变 bot/automation 配置 UI。
- 不重构 Composer profile、项目 adapter 或通用错误中心。
- 不改变 collaboration/goal/token/model 交互。

## 约束

- 普通 Composer：await backend 成功后才 patch 当前 profile/YOLO restore；失败使用现有 `showError`。
- 项目草稿无 tab/session 时只更新待创建 profile；已准备 tab 时必须走后端事务 setter。
- 快速切换后，完成回调必须核对原目标，不得 patch 当前另一会话。
- `toolApprovalDefaultPreference` 仅保留新会话创建等待/设置页写入职责。

## 验收标准

- Composer 选择 Ask/Auto/YOLO 不调用全局默认 writer。
- backend reject 时 UI 仍选旧值并有可见错误。
- 成功后 UI/backend/sidecar 一致。
- 项目单表面已有会话立即持久化；草稿第一次 send 前把所选值交给新会话。
- 快速 A/B 切换或连续点击不造成错误会话 UI 更新。

## 聚焦验证

- 更新/新增相关 `tsx`/Vitest 测试。
- `pnpm build`（从 `desktop/frontend`）。

## 执行

- Agent：主会话。
- 模型：当前 Codex 主会话。

## 完成回执

- 状态：已完成。
- 变更文件：`desktop/frontend/src/App.tsx`、`desktop/frontend/src/features/session/useController.ts`、`desktop/frontend/src/features/composer/ui/Composer.tsx`、`desktop/frontend/src/__tests__/tool-approval-default-preference.test.ts`；删除不再有生产用途的 `desktop/frontend/src/lib/toolApprovalDefaultPreference.ts`。
- 测试：审批默认合同 5/5、tool approval mode 9/9、Composer profile 27/27、tab/single-surface hydration 147/147、project Composer production contract 通过；`pnpm build` 通过。
- 实现：普通 Composer await 后端成功后按原 target tab patch；setter error 不再吞掉且使用错误中心提示；YOLO restore 仅成功后更新。项目草稿无 tab 时保留待创建值，已有 tab 时先持久化再 patch，并校验 adapter/tab 身份。
- 偏差：原 preference helper 专门串行 Composer→全局默认写入；合同取消该写入后没有生产调用点，故删除而不是保留死代码。设置页仍直接调用 `SetDefaultToolApprovalMode`。
- 风险/交接：真实 Wails 错误展示和重启/历史场景由任务 003 隔离 dev 实例验收。

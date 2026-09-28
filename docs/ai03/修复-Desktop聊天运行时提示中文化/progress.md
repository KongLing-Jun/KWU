# Desktop 聊天运行时提示中文化进度

## 当前阶段

- 阶段：Stage 3 实现
- 状态：任务 001、002、003 均已完成，最终 PR Gate 通过；等待 push 与创建 Draft PR 授权
- 分支：`codex/localize-desktop-runtime-notices`
- worktree：`D:\Reasonix-worktrees\localize-desktop-runtime-notices`
- 基线：`origin/main-v3@f97d7fa2f242466250c324c4ba5026643a94d9cf`

## 已确认决定

- 只处理 Desktop 聊天页已知静态运行时 Notice、Phase 和 turn_done 错误。
- 展示层保守中文化；未知前端运行时提示原样显示，Provider 原始错误沿用后端安全化。
- `Item.text`、Markdown 复制和 JSON 导出保留原文。
- 不翻译模型/推理/工具/文件内容，不改 Provider、Agent、task runtime、计费或存储。
- 复用现有组件、locale 和样式，不新增详情展开器。
- task 终态显示通用中文并隐藏可见内部 ID；非活动完成和 parallel_tasks 行为保持现状。
- 真实验收使用 Luna 自然场景与受控开发 Provider 两层覆盖。
- 不拆分计划、任务和实现 PR；同一分支按意图小步提交，最终统一创建一个 Draft PR。

## 规划工件

- [`plan.md`](./plan.md)：Stage 1 决策、源码证据、方案、风险和验收合同。
- 历史 OpenSpec 提案：变更意图与范围（规范目录已按仓库约定移除）。
- 历史 OpenSpec 规格：可观察产品要求（规范目录已按仓库约定移除）。
- 历史 OpenSpec 设计：技术设计与取舍（规范目录已按仓库约定移除）。
- 历史 OpenSpec 任务清单：实施检查表；不是 Stage 2 的执行任务文档（规范目录已按仓库约定移除）。
- [`tasks/001-建立运行时提示展示映射.md`](./tasks/001-建立运行时提示展示映射.md)：生产 emitter 清单、纯映射和 locale 合同。
- [`tasks/002-接入聊天展示并保护原文.md`](./tasks/002-接入聊天展示并保护原文.md)：现有组件接入、原文导出和保持性回归。
- [`tasks/003-完成真实应用验收与交付.md`](./tasks/003-完成真实应用验收与交付.md)：真实 Luna、受控 Provider、最终验证和回执。

## 当前验证

- Codegraph 已用于索引主工作区的调用链导航；索引基线与 worktree 基线差异已在计划中记录。
- 最新 worktree 已直接复核展示、投影、导出、task Notice 抑制和完成队列链路。
- OpenSpec strict validation：通过。
- Stage 2 任务已按映射合同、展示接入、独立验收拆为 001 → 002 → 003。
- 任务 001 映射合同测试 131/131 通过。
- 任务 002 Transcript DOM/state 14/14、Markdown/JSON 原文导出 7/7、事件所有权 7/7、TypeScript 编译和前端生产构建通过。
- 真实 Luna 已覆盖普通聊天、持久 task、parallel_tasks、专家团隔离、中断、压缩后继续对话及 20 轮会话切换压力；受控 Provider 已覆盖 retry、输出截断、空回复、readiness 和 Provider 安全化。
- 临时 Provider、端口和验收文件已清理，dev 配置恢复原 SHA-256；没有夹具或凭据进入版本控制。
- OpenSpec strict validation 通过；verify 覆盖 6/6 requirements、18/18 scenarios，无 completeness、correctness 或 coherence 问题。
- 同步检查确认 `origin/main-v3` 仍为规划基线，分支不落后、无冲突；完整 PR Gate 通过。
- OpenSpec 实施进度：16/16。

## 阻塞与风险

- 无未决产品问题或 change-related 测试失败。
- Computer Use 的原生连接仍返回 `node_repl exec context not found`，因此实际 UI 由内置 Browser 验收，JSON 导出原文由精确执行真实 `App.tsx` 导出函数的自动化回归证明。
- 手动压缩完成 Notice 和专家成员 sidecar 的短暂异步加载均保持既有生命周期，本变更未扩展为持久化或加载重构。

## 下一门禁

1. 提交最终任务回执，并在最终树重跑完整 PR Gate。
2. 获得授权后 push 当前分支并创建 Draft PR。
3. 标记 Ready 与合并仍需单独授权。

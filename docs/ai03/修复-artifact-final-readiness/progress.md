# Artifact final-readiness 修复进度

## 当前状态

| 项目 | 状态 |
| --- | --- |
| 当前阶段 | 任务 001、002、003 已完成；自动化与真实 Ask/Auto/Yolo 应用验收通过；正在清理用户授权纳入同 PR 的过时 `internal/tasktree` 文档门禁 |
| 基线 | `origin/main-v3@70d919f4c50ca81f18952bc90c3817e9d9d78af8` |
| worktree | `D:\Reasonix-worktrees\fix-artifact-final-readiness` |
| 分支 | `codex/fix-artifact-final-readiness` |
| Codegraph | clean `D:\Reasonix` 索引，pending 0 |
| PR | 尚未创建；清理过时门禁后重跑最终 Gate，通过才创建 Draft PR |

## 已锁定决策

- 根因是“写文件工具”中的单字动作命中生成匿名 target，不是模型或写入失败。
- 只允许高置信度目标成为 final-readiness 硬合同。
- 保留真实多产物的 distinct-path 和 freshness 约束。
- `verify_artifact` 是宿主内部验证，不是 Agent 工具调用。
- 不为单一提示打补丁，不扫描整个 workspace 猜匿名产物。
- 主线已删除的历史任务树验收文档不恢复；移除只检查该文档文本、没有运行时覆盖的残留测试。

## 任务映射

| 任务 | 状态 | 依赖 |
| --- | --- | --- |
| [001 收紧 artifact intent](tasks/001-收紧artifact-intent.md) | 已完成 | 本计划 |
| [002 下沉宿主 artifact 验证](tasks/002-下沉宿主artifact验证.md) | 已完成 | 001 |
| [003 自动化与真实应用验收](tasks/003-自动化与实际应用验收.md) | 已完成 | 001、002 |

## 当前验证与下一门禁

- 调查期相关 Agent、evidence、builtin 聚焦测试通过，确认现有测试没有覆盖中文工具元语言。
- Ask、Auto、Yolo 真实会话均有正确文件副作用和错误 readiness rejection。
- 任务 001 红测稳定复现中英文工具名称各多出一个匿名 target；局部动作资格修复后，
  parser/readiness 聚焦测试通过。
- 任务 002 完整 Agent.Run、普通/Token Economy boot、expert-team handoff、匿名 writer、
  fail-closed 和 mutation refresh 回归通过。
- `go test ./internal/agent ./internal/evidence ./internal/tool/builtin ./internal/boot -count=1` 通过；
  `go test ./internal/config -count=1` 通过。
- `go run ./scripts/code_guard`、baseline 不变检查和 `go vet ./...` 通过。`go test ./...` 除
  clean `origin/main-v3` 同样存在的 `internal/tasktree` 缺文档基线红外全部通过；未扩大范围补无关文档。
- 从本 worktree 构建并启动真实 `kwu-desktop-dev.exe`，使用真实 `gpt/gpt-5.6-luna` 完成
  Ask、Auto、Yolo 三个独立 workspace 验收：每个会话均为一个 `write_file`、零模型
  `verify_artifact`、marker 精确 `ok`、final `done`、两次模型请求且无 rejection/retry。
- 当前测试 Desktop/Wails/Vite/WebView 进程已关闭；验收文件、JSONL 和日志保留在任务 003
  记录的位置。
- 最终 fetch 确认 `origin/main-v3` 仍为 `70d919f4c50ca81f18952bc90c3817e9d9d78af8`，无需同步提交；
  最终 `code_guard`、baseline 未改、`go vet ./...`、`git diff --check` 和受影响包复验通过。
- 用户已授权在当前 worktree、同一 PR 中以独立小步提交清理
  `TestTaskTreeRegressionChecklistDocumentsRequiredEntries`。它只校验已删除历史文档中的字符串，删除后
  先运行 `internal/tasktree` 聚焦测试，再按最新 `AGENTS.md` 执行最终 PR Gate。

# Code Guard 484 条历史问题逐项核实台账

> 本报告是当前工作区的度量核实结果，不是 484 个功能 Bug 的确认清单。每条记录证明对应源码在当前 Code Guard 规则下仍然超限；是否需要拆分以及如何拆分，仍需结合职责、调用链和回归测试单独设计。

## 1. 核实快照

- 生成时间：2026/07/10 16:29:50（Asia/Shanghai）
- 分支：`codex/docs-0.9-consistency`
- Commit：`99997cb549303e70f6f2b59bb5defd1466afc589`
- Baseline：`scripts/code_guard_baseline.json`
- Baseline SHA-256：`9cc3dc818cac3351465d77999b655df07186712083980ce5064494aed2a5b7c6`
- 当前扫描：`go run ./scripts/code_guard -- --strict --json`
- strict 退出码：`1`（发现违规时的预期退出码，JSON 扫描成功）
- 工作树：存在核实前已有变更，详见下方；本次未修改这些路径

```text
M "docs/bugs/01/IM\344\274\232\350\257\235\345\244\215\347\224\250\351\227\256\351\242\230/bug-fix.md"
?? docs/todos/101/
```

## 2. 逐项对账结论

| 核查项 | 数量 |
|---|---|
| Baseline 条目 | 484 |
| 当前 strict 违规 | 484 |
| 六字段完全一致（kind/file/func/line/got/limit） | 484 |
| 已改善但仍超限 | 0 |
| 已恶化 | 0 |
| 已消失 | 0 |
| 新增但不在 baseline | 0 |
| 定位或阈值漂移 | 0 |
| 源码路径与行号可定位 | 484/484 |

结论：当前 484 条全部仍存在，且 baseline 与当前 strict 扫描逐项完全一致；没有改善、恶化、消失、新增或行号漂移。

## 3. 分类汇总

| 类型 | 数量 | 规则阈值 | 核实结果 |
|---|---|---|---|
| 圈复杂度 | 394 | 10 | 全部仍存在并精确匹配 |
| 文件有效行数 | 18 | 800 | 全部仍存在并精确匹配 |
| 函数有效行数 | 51 | 80 | 全部仍存在并精确匹配 |
| 嵌套深度 | 20 | 4 | 全部仍存在并精确匹配 |
| 单包生产文件数 | 1 | 30 | 全部仍存在并精确匹配 |

### 3.1 顶层目录分布

| 目录 | 条数 |
|---|---|
| `internal/` | 474 |
| `cmd/` | 10 |

### 3.2 核实结论边界

| 结论类型 | 数量 | 说明 |
|---|---|---|
| 静态度量超限已确认 | 484 | 当前 strict 扫描与 baseline 六字段精确匹配 |
| 已确认功能 Bug | 0 | 本次没有运行 484 个符号各自的业务场景，不能把静态超限直接等同于功能 Bug |
| 已确认合理例外 | 0 | 尚未逐项完成领域职责和调用链评审 |
| 待语义治理评审 | 484 | 后续修复前仍需结合调用链、状态边界和回归测试确定处置方式 |

## 4. 治理排序说明

台账中的 P0–P3 仅表示相对阈值的超限幅度，用于安排人工复核和拆分顺序，不代表业务缺陷或线上事故等级：

- P0：实际值达到阈值 3 倍及以上；
- P1：达到 2 倍但不足 3 倍；
- P2：达到 1.5 倍但不足 2 倍；
- P3：超过阈值但不足 1.5 倍。

## 5. 文件热点

| 文件/包 | 总条数 | 复杂度 | 超长文件 | 超长函数 | 嵌套 | 包文件数 | 最高倍数 |
|---|---|---|---|---|---|---|---|
| `internal/cli/chat_tui.go` | 20 | 13 | 1 | 4 | 2 | 0 | 17.60x |
| `internal/memorycompiler/runtime.go` | 15 | 11 | 1 | 2 | 1 | 0 | 3.35x |
| `internal/control/controller.go` | 14 | 11 | 1 | 1 | 1 | 0 | 3.85x |
| `internal/cli/cli.go` | 13 | 9 | 1 | 3 | 0 | 0 | 3.70x |
| `internal/agent/migrate.go` | 12 | 8 | 0 | 2 | 2 | 0 | 5.40x |
| `internal/agent/agent.go` | 11 | 6 | 1 | 4 | 0 | 0 | 3.80x |
| `internal/acp/service.go` | 10 | 8 | 1 | 1 | 0 | 0 | 2.20x |
| `internal/evidence/evidence.go` | 8 | 7 | 1 | 0 | 0 | 0 | 1.50x |
| `internal/control/slash.go` | 7 | 6 | 0 | 1 | 0 | 0 | 3.10x |
| `internal/installsource/plan.go` | 7 | 6 | 0 | 1 | 0 | 0 | 2.30x |
| `internal/botruntime/runtime.go` | 7 | 6 | 0 | 0 | 1 | 0 | 1.80x |
| `internal/jobs/jobs.go` | 7 | 5 | 1 | 1 | 0 | 0 | 1.80x |
| `internal/config/render.go` | 6 | 3 | 1 | 2 | 0 | 0 | 12.40x |
| `internal/provider/anthropic/anthropic.go` | 6 | 2 | 0 | 2 | 2 | 0 | 5.00x |
| `internal/control/refs.go` | 6 | 6 | 0 | 0 | 0 | 0 | 2.00x |
| `internal/history/search.go` | 6 | 4 | 0 | 1 | 1 | 0 | 1.90x |
| `cmd/e2ebench/diff.go` | 6 | 4 | 0 | 0 | 2 | 0 | 1.60x |
| `internal/config/load.go` | 6 | 4 | 1 | 0 | 1 | 0 | 1.50x |
| `internal/memory/store.go` | 6 | 5 | 0 | 0 | 1 | 0 | 1.50x |
| `internal/boot/boot.go` | 5 | 2 | 1 | 1 | 1 | 0 | 17.80x |
| `internal/cli/select.go` | 5 | 2 | 0 | 2 | 1 | 0 | 7.30x |
| `internal/agent/parallel_tasks.go` | 5 | 4 | 0 | 1 | 0 | 0 | 4.80x |
| `internal/bot/gateway.go` | 5 | 3 | 1 | 1 | 0 | 0 | 3.10x |
| `internal/memorycompiler/compression.go` | 5 | 4 | 1 | 0 | 0 | 0 | 2.50x |
| `internal/cli/skill_picker.go` | 5 | 5 | 0 | 0 | 0 | 0 | 2.00x |
| `internal/control/attachments.go` | 5 | 5 | 0 | 0 | 0 | 0 | 2.00x |
| `internal/config/edit.go` | 5 | 4 | 1 | 0 | 0 | 0 | 1.80x |
| `internal/tool/builtin/bash.go` | 5 | 5 | 0 | 0 | 0 | 0 | 1.70x |
| `internal/agent/subagent_store.go` | 5 | 5 | 0 | 0 | 0 | 0 | 1.30x |
| `internal/tasktree/planner.go` | 4 | 3 | 0 | 1 | 0 | 0 | 3.30x |

## 6. 最严重的复杂度条目

| 位置 | 当前 | 阈值 | 超出 | 倍数 |
|---|---|---|---|---|
| `internal/boot/boot.go`:152 · `Build` | 178 | 10 | +168 | 17.80x |
| `internal/cli/chat_tui.go`:713 · `chatTUI.update` | 176 | 10 | +166 | 17.60x |
| `internal/config/render.go`:575 · `RenderTOMLProjectDelta` | 124 | 10 | +114 | 12.40x |
| `internal/config/render.go`:32 · `RenderTOMLForScope` | 98 | 10 | +88 | 9.80x |
| `internal/cli/select.go`:267 · `selectMany` | 73 | 10 | +63 | 7.30x |
| `internal/agent/migrate.go`:82 · `migrateLegacySessions` | 54 | 10 | +44 | 5.40x |
| `internal/cli/chat_tui.go`:3226 · `*chatTUI.runSlashCommand` | 50 | 10 | +40 | 5.00x |
| `internal/control/auto_plan.go`:207 · `isLowRiskQuestion` | 50 | 10 | +40 | 5.00x |
| `internal/provider/anthropic/anthropic.go`:308 · `*client.readStream` | 50 | 10 | +40 | 5.00x |
| `internal/cli/select.go`:81 · `selectOne` | 49 | 10 | +39 | 4.90x |
| `internal/agent/parallel_tasks.go`:111 · `*ParallelTasksTool.Execute` | 48 | 10 | +38 | 4.80x |
| `internal/cli/chat_tui.go`:2210 · `chatTUI.View` | 47 | 10 | +37 | 4.70x |
| `internal/cli/chat_tui.go`:3004 · `*chatTUI.ingestEvent` | 47 | 10 | +37 | 4.70x |
| `internal/cli/mcp_manager.go`:115 · `chatTUI.handleMCPManagerKey` | 45 | 10 | +35 | 4.50x |
| `internal/agent/agent.go`:675 · `*Agent.Run` | 38 | 10 | +28 | 3.80x |
| `internal/agent/agent.go`:1826 · `*Agent.executeOneWithRegistry` | 38 | 10 | +28 | 3.80x |
| `internal/cli/cli.go`:652 · `chatREPL` | 37 | 10 | +27 | 3.70x |
| `internal/control/controller.go`:698 · `*Controller.submitCommandOrTurn` | 35 | 10 | +25 | 3.50x |
| `internal/tasktree/planner.go`:205 · `validateGeneratedNodes` | 33 | 10 | +23 | 3.30x |
| `internal/memorycompiler/runtime.go`:544 · `buildIRWithPolicy` | 32 | 10 | +22 | 3.20x |

## 7. 最严重的超长函数

| 位置 | 当前有效行 | 阈值 | 超出 | 倍数 |
|---|---|---|---|---|
| `internal/boot/boot.go`:152 · `Build` | 924 | 80 | +844 | 11.55x |
| `internal/cli/chat_tui.go`:713 · `chatTUI.update` | 532 | 80 | +452 | 6.65x |
| `internal/config/render.go`:32 · `RenderTOMLForScope` | 516 | 80 | +436 | 6.45x |
| `internal/config/render.go`:575 · `RenderTOMLProjectDelta` | 338 | 80 | +258 | 4.22x |
| `internal/agent/parallel_tasks.go`:111 · `*ParallelTasksTool.Execute` | 229 | 80 | +149 | 2.86x |
| `internal/cli/select.go`:267 · `selectMany` | 205 | 80 | +125 | 2.56x |
| `internal/cli/chat_tui.go`:2210 · `chatTUI.View` | 174 | 80 | +94 | 2.17x |
| `internal/agent/agent.go`:675 · `*Agent.Run` | 172 | 80 | +92 | 2.15x |
| `internal/cli/chat_tui.go`:3226 · `*chatTUI.runSlashCommand` | 162 | 80 | +82 | 2.02x |
| `internal/cli/select.go`:81 · `selectOne` | 156 | 80 | +76 | 1.95x |
| `internal/provider/anthropic/anthropic.go`:308 · `*client.readStream` | 155 | 80 | +75 | 1.94x |
| `internal/cli/cli.go`:652 · `chatREPL` | 148 | 80 | +68 | 1.85x |
| `internal/cli/chat_tui.go`:3004 · `*chatTUI.ingestEvent` | 147 | 80 | +67 | 1.84x |
| `internal/agent/migrate.go`:82 · `migrateLegacySessions` | 146 | 80 | +66 | 1.82x |
| `internal/agent/agent.go`:1826 · `*Agent.executeOneWithRegistry` | 135 | 80 | +55 | 1.69x |
| `internal/cli/bot.go`:164 · `botDoctor` | 133 | 80 | +53 | 1.66x |
| `internal/cli/mcp_manager.go`:115 · `chatTUI.handleMCPManagerKey` | 130 | 80 | +50 | 1.63x |
| `internal/control/controller.go`:698 · `*Controller.submitCommandOrTurn` | 127 | 80 | +47 | 1.59x |
| `internal/bot/gateway.go`:509 · `*BotGateway.handleSlashCommand` | 124 | 80 | +44 | 1.55x |
| `internal/bot/qq/gateway.go`:214 · `*adapter.connectGateway` | 119 | 80 | +39 | 1.49x |

## 8. 全部超长文件

| 位置 | 当前有效行 | 阈值 | 超出 | 倍数 |
|---|---|---|---|---|
| `internal/control/controller.go`:1 | 3081 | 800 | +2281 | 3.85x |
| `internal/cli/chat_tui.go`:1 | 2809 | 800 | +2009 | 3.51x |
| `internal/memorycompiler/runtime.go`:1 | 2677 | 800 | +1877 | 3.35x |
| `internal/cli/cli.go`:1 | 1638 | 800 | +838 | 2.05x |
| `internal/agent/agent.go`:1 | 1632 | 800 | +832 | 2.04x |
| `internal/memorycompiler/compression.go`:1 | 1545 | 800 | +745 | 1.93x |
| `internal/boot/boot.go`:1 | 1481 | 800 | +681 | 1.85x |
| `internal/acp/service.go`:1 | 1390 | 800 | +590 | 1.74x |
| `internal/jobs/jobs.go`:1 | 1275 | 800 | +475 | 1.59x |
| `internal/serve/serve.go`:1 | 1245 | 800 | +445 | 1.56x |
| `internal/config/render.go`:1 | 1151 | 800 | +351 | 1.44x |
| `internal/automation/business.go`:1 | 1055 | 800 | +255 | 1.32x |
| `internal/config/config.go`:1 | 1027 | 800 | +227 | 1.28x |
| `internal/config/edit.go`:1 | 1018 | 800 | +218 | 1.27x |
| `internal/config/load.go`:1 | 1005 | 800 | +205 | 1.26x |
| `internal/bot/gateway.go`:1 | 962 | 800 | +162 | 1.20x |
| `internal/plugin/plugin.go`:1 | 859 | 800 | +59 | 1.07x |
| `internal/evidence/evidence.go`:1 | 815 | 800 | +15 | 1.02x |

## 9. 484 条逐项核实明细

### 9.1 圈复杂度（394 条）

Go AST 分支复杂度近似值；阈值 10。

| ID | Baseline key | 状态 | 优先级 | 位置 | Baseline | 当前复测 | 阈值 | 超出 | 倍数 |
|---|---|---|---|---|---|---|---|---|---|
| CX-001 | complexity\|cmd/e2ebench/diff.go\|renderDiff | 仍存在（精确匹配） | P2 | `cmd/e2ebench/diff.go`:198 · `renderDiff` | 16 | 16 | 10 | +6 | 1.60x |
| CX-002 | complexity\|cmd/e2ebench/diff.go\|changedLineSet | 仍存在（精确匹配） | P3 | `cmd/e2ebench/diff.go`:418 · `changedLineSet` | 11 | 11 | 10 | +1 | 1.10x |
| CX-003 | complexity\|cmd/e2ebench/diff.go\|parseNewTests | 仍存在（精确匹配） | P3 | `cmd/e2ebench/diff.go`:477 · `parseNewTests` | 13 | 13 | 10 | +3 | 1.30x |
| CX-004 | complexity\|cmd/e2ebench/diff.go\|splitShellFields | 仍存在（精确匹配） | P3 | `cmd/e2ebench/diff.go`:562 · `splitShellFields` | 13 | 13 | 10 | +3 | 1.30x |
| CX-005 | complexity\|cmd/e2ebench/main.go\|main | 仍存在（精确匹配） | P3 | `cmd/e2ebench/main.go`:48 · `main` | 13 | 13 | 10 | +3 | 1.30x |
| CX-006 | complexity\|cmd/e2ebench/main.go\|render | 仍存在（精确匹配） | P3 | `cmd/e2ebench/main.go`:240 · `render` | 12 | 12 | 10 | +2 | 1.20x |
| CX-007 | complexity\|cmd/e2ebench/mutation.go\|runMutation | 仍存在（精确匹配） | P3 | `cmd/e2ebench/mutation.go`:28 · `runMutation` | 12 | 12 | 10 | +2 | 1.20x |
| CX-008 | complexity\|cmd/suiyuan-plugin-example/main.go\|handleLine | 仍存在（精确匹配） | P3 | `cmd/suiyuan-plugin-example/main.go`:102 · `handleLine` | 13 | 13 | 10 | +3 | 1.30x |
| CX-009 | complexity\|internal/acp/dispatch.go\|*updateSink.Emit | 仍存在（精确匹配） | P2 | `internal/acp/dispatch.go`:99 · `*updateSink.Emit` | 16 | 16 | 10 | +6 | 1.60x |
| CX-010 | complexity\|internal/acp/dispatch.go\|toolKindFor | 仍存在（精确匹配） | P2 | `internal/acp/dispatch.go`:380 · `toolKindFor` | 18 | 18 | 10 | +8 | 1.80x |
| CX-011 | complexity\|internal/acp/service.go\|*service.openExistingSession | 仍存在（精确匹配） | P1 | `internal/acp/service.go`:398 · `*service.openExistingSession` | 22 | 22 | 10 | +12 | 2.20x |
| CX-012 | complexity\|internal/acp/service.go\|*service.sessionPrompt | 仍存在（精确匹配） | P3 | `internal/acp/service.go`:525 · `*service.sessionPrompt` | 11 | 11 | 10 | +1 | 1.10x |
| CX-013 | complexity\|internal/acp/service.go\|*service.rebuildSession | 仍存在（精确匹配） | P3 | `internal/acp/service.go`:667 · `*service.rebuildSession` | 14 | 14 | 10 | +4 | 1.40x |
| CX-014 | complexity\|internal/acp/service.go\|*service.sessionList | 仍存在（精确匹配） | P3 | `internal/acp/service.go`:810 · `*service.sessionList` | 14 | 14 | 10 | +4 | 1.40x |
| CX-015 | complexity\|internal/acp/service.go\|*service.sessionDelete | 仍存在（精确匹配） | P3 | `internal/acp/service.go`:862 · `*service.sessionDelete` | 12 | 12 | 10 | +2 | 1.20x |
| CX-016 | complexity\|internal/acp/service.go\|availableCommandsFor | 仍存在（精确匹配） | P2 | `internal/acp/service.go`:1165 · `availableCommandsFor` | 16 | 16 | 10 | +6 | 1.60x |
| CX-017 | complexity\|internal/acp/service.go\|listACPMetas | 仍存在（精确匹配） | P3 | `internal/acp/service.go`:1362 · `listACPMetas` | 12 | 12 | 10 | +2 | 1.20x |
| CX-018 | complexity\|internal/acp/service.go\|isSafeSessionID | 仍存在（精确匹配） | P3 | `internal/acp/service.go`:1456 · `isSafeSessionID` | 11 | 11 | 10 | +1 | 1.10x |
| CX-019 | complexity\|internal/agent/agent.go\|*Agent.Run | 仍存在（精确匹配） | P0 | `internal/agent/agent.go`:675 · `*Agent.Run` | 38 | 38 | 10 | +28 | 3.80x |
| CX-020 | complexity\|internal/agent/agent.go\|*Agent.finalReadinessCheck | 仍存在（精确匹配） | P2 | `internal/agent/agent.go`:932 · `*Agent.finalReadinessCheck` | 17 | 17 | 10 | +7 | 1.70x |
| CX-021 | complexity\|internal/agent/agent.go\|*Agent.rebuildTodoState | 仍存在（精确匹配） | P3 | `internal/agent/agent.go`:1119 · `*Agent.rebuildTodoState` | 12 | 12 | 10 | +2 | 1.20x |
| CX-022 | complexity\|internal/agent/agent.go\|*Agent.stream | 仍存在（精确匹配） | P1 | `internal/agent/agent.go`:1363 · `*Agent.stream` | 25 | 25 | 10 | +15 | 2.50x |
| CX-023 | complexity\|internal/agent/agent.go\|*Agent.executeBatchWithRegistry | 仍存在（精确匹配） | P0 | `internal/agent/agent.go`:1504 · `*Agent.executeBatchWithRegistry` | 30 | 30 | 10 | +20 | 3.00x |
| CX-024 | complexity\|internal/agent/agent.go\|*Agent.executeOneWithRegistry | 仍存在（精确匹配） | P0 | `internal/agent/agent.go`:1826 · `*Agent.executeOneWithRegistry` | 38 | 38 | 10 | +28 | 3.80x |
| CX-025 | complexity\|internal/agent/ask.go\|*AskTool.Execute | 仍存在（精确匹配） | P3 | `internal/agent/ask.go`:71 · `*AskTool.Execute` | 12 | 12 | 10 | +2 | 1.20x |
| CX-026 | complexity\|internal/agent/branch.go\|saveBranchMeta | 仍存在（精确匹配） | P3 | `internal/agent/branch.go`:121 · `saveBranchMeta` | 12 | 12 | 10 | +2 | 1.20x |
| CX-027 | complexity\|internal/agent/branch.go\|ListBranches | 仍存在（精确匹配） | P3 | `internal/agent/branch.go`:193 · `ListBranches` | 13 | 13 | 10 | +3 | 1.30x |
| CX-028 | complexity\|internal/agent/compact.go\|*Agent.maybeCompact | 仍存在（精确匹配） | P2 | `internal/agent/compact.go`:84 · `*Agent.maybeCompact` | 15 | 15 | 10 | +5 | 1.50x |
| CX-029 | complexity\|internal/agent/compact.go\|*Agent.compact | 仍存在（精确匹配） | P3 | `internal/agent/compact.go`:189 · `*Agent.compact` | 12 | 12 | 10 | +2 | 1.20x |
| CX-030 | complexity\|internal/agent/compact.go\|keepIndexes | 仍存在（精确匹配） | P3 | `internal/agent/compact.go`:400 · `keepIndexes` | 11 | 11 | 10 | +1 | 1.10x |
| CX-031 | complexity\|internal/agent/compact.go\|*Agent.summarize | 仍存在（精确匹配） | P3 | `internal/agent/compact.go`:601 · `*Agent.summarize` | 13 | 13 | 10 | +3 | 1.30x |
| CX-032 | complexity\|internal/agent/migrate.go\|migrateLegacySessions | 仍存在（精确匹配） | P0 | `internal/agent/migrate.go`:82 · `migrateLegacySessions` | 54 | 54 | 10 | +44 | 5.40x |
| CX-033 | complexity\|internal/agent/migrate.go\|importJsonlSessions | 仍存在（精确匹配） | P2 | `internal/agent/migrate.go`:271 · `importJsonlSessions` | 16 | 16 | 10 | +6 | 1.60x |
| CX-034 | complexity\|internal/agent/migrate.go\|jsonlSessionDestDir | 仍存在（精确匹配） | P3 | `internal/agent/migrate.go`:323 · `jsonlSessionDestDir` | 13 | 13 | 10 | +3 | 1.30x |
| CX-035 | complexity\|internal/agent/migrate.go\|migrateSubDirectory | 仍存在（精确匹配） | P1 | `internal/agent/migrate.go`:352 · `migrateSubDirectory` | 29 | 29 | 10 | +19 | 2.90x |
| CX-036 | complexity\|internal/agent/migrate.go\|transformAndCopyJsonl | 仍存在（精确匹配） | P1 | `internal/agent/migrate.go`:482 · `transformAndCopyJsonl` | 23 | 23 | 10 | +13 | 2.30x |
| CX-037 | complexity\|internal/agent/migrate.go\|rehomeStrandedSessions | 仍存在（精确匹配） | P1 | `internal/agent/migrate.go`:685 · `rehomeStrandedSessions` | 20 | 20 | 10 | +10 | 2.00x |
| CX-038 | complexity\|internal/agent/migrate.go\|strandedSessionDestDir | 仍存在（精确匹配） | P3 | `internal/agent/migrate.go`:756 · `strandedSessionDestDir` | 11 | 11 | 10 | +1 | 1.10x |
| CX-039 | complexity\|internal/agent/migrate.go\|copyFileIfExists | 仍存在（精确匹配） | P3 | `internal/agent/migrate.go`:830 · `copyFileIfExists` | 12 | 12 | 10 | +2 | 1.20x |
| CX-040 | complexity\|internal/agent/parallel_tasks.go\|*ParallelTasksTool.Execute | 仍存在（精确匹配） | P0 | `internal/agent/parallel_tasks.go`:111 · `*ParallelTasksTool.Execute` | 48 | 48 | 10 | +38 | 4.80x |
| CX-041 | complexity\|internal/agent/parallel_tasks.go\|formatParallelTasksAggregate | 仍存在（精确匹配） | P3 | `internal/agent/parallel_tasks.go`:368 · `formatParallelTasksAggregate` | 11 | 11 | 10 | +1 | 1.10x |
| CX-042 | complexity\|internal/agent/parallel_tasks.go\|*ParallelTasksTool.runAsBackgroundJobs | 仍存在（精确匹配） | P2 | `internal/agent/parallel_tasks.go`:409 · `*ParallelTasksTool.runAsBackgroundJobs` | 17 | 17 | 10 | +7 | 1.70x |
| CX-043 | complexity\|internal/agent/parallel_tasks.go\|validateParallelTaskItems | 仍存在（精确匹配） | P3 | `internal/agent/parallel_tasks.go`:497 · `validateParallelTaskItems` | 13 | 13 | 10 | +3 | 1.30x |
| CX-044 | complexity\|internal/agent/prune.go\|*Agent.PruneStaleToolResults | 仍存在（精确匹配） | P3 | `internal/agent/prune.go`:29 · `*Agent.PruneStaleToolResults` | 14 | 14 | 10 | +4 | 1.40x |
| CX-045 | complexity\|internal/agent/reasoning_language.go\|hasLeadingInjectedBlock | 仍存在（精确匹配） | P2 | `internal/agent/reasoning_language.go`:87 · `hasLeadingInjectedBlock` | 15 | 15 | 10 | +5 | 1.50x |
| CX-046 | complexity\|internal/agent/save.go\|ListCleanupPending | 仍存在（精确匹配） | P3 | `internal/agent/save.go`:273 · `ListCleanupPending` | 11 | 11 | 10 | +1 | 1.10x |
| CX-047 | complexity\|internal/agent/save.go\|ListSessionOrder | 仍存在（精确匹配） | P3 | `internal/agent/save.go`:339 · `ListSessionOrder` | 13 | 13 | 10 | +3 | 1.30x |
| CX-048 | complexity\|internal/agent/subagent_store.go\|ListSubagentsByParent | 仍存在（精确匹配） | P3 | `internal/agent/subagent_store.go`:146 · `ListSubagentsByParent` | 12 | 12 | 10 | +2 | 1.20x |
| CX-049 | complexity\|internal/agent/subagent_store.go\|*SubagentStore.CleanupStaleRunning | 仍存在（精确匹配） | P3 | `internal/agent/subagent_store.go`:210 · `*SubagentStore.CleanupStaleRunning` | 11 | 11 | 10 | +1 | 1.10x |
| CX-050 | complexity\|internal/agent/subagent_store.go\|*SubagentStore.prepareFork | 仍存在（精确匹配） | P3 | `internal/agent/subagent_store.go`:471 · `*SubagentStore.prepareFork` | 12 | 12 | 10 | +2 | 1.20x |
| CX-051 | complexity\|internal/agent/subagent_store.go\|validateMeta | 仍存在（精确匹配） | P3 | `internal/agent/subagent_store.go`:629 · `validateMeta` | 13 | 13 | 10 | +3 | 1.30x |
| CX-052 | complexity\|internal/agent/subagent_store.go\|validSubagentRef | 仍存在（精确匹配） | P3 | `internal/agent/subagent_store.go`:834 · `validSubagentRef` | 11 | 11 | 10 | +1 | 1.10x |
| CX-053 | complexity\|internal/agent/task.go\|*TaskTool.Execute | 仍存在（精确匹配） | P1 | `internal/agent/task.go`:434 · `*TaskTool.Execute` | 25 | 25 | 10 | +15 | 2.50x |
| CX-054 | complexity\|internal/agent/task.go\|ReadOnlySubagentToolRegistry | 仍存在（精确匹配） | P3 | `internal/agent/task.go`:672 · `ReadOnlySubagentToolRegistry` | 11 | 11 | 10 | +1 | 1.10x |
| CX-055 | complexity\|internal/agent/task.go\|subSinkForContext | 仍存在（精确匹配） | P3 | `internal/agent/task.go`:869 · `subSinkForContext` | 13 | 13 | 10 | +3 | 1.30x |
| CX-056 | complexity\|internal/agent/testutil/mock_provider.go\|*MockProvider.Stream | 仍存在（精确匹配） | P3 | `internal/agent/testutil/mock_provider.go`:71 · `*MockProvider.Stream` | 14 | 14 | 10 | +4 | 1.40x |
| CX-057 | complexity\|internal/agent/textsink.go\|*TextSink.Emit | 仍存在（精确匹配） | P1 | `internal/agent/textsink.go`:51 · `*TextSink.Emit` | 25 | 25 | 10 | +15 | 2.50x |
| CX-058 | complexity\|internal/agent/textsink.go\|FormatUsageLine | 仍存在（精确匹配） | P3 | `internal/agent/textsink.go`:185 · `FormatUsageLine` | 11 | 11 | 10 | +1 | 1.10x |
| CX-059 | complexity\|internal/automation/archive.go\|pathID | 仍存在（精确匹配） | P3 | `internal/automation/archive.go`:291 · `pathID` | 13 | 13 | 10 | +3 | 1.30x |
| CX-060 | complexity\|internal/automation/business.go\|ValidateBusinessDiagnostics | 仍存在（精确匹配） | P3 | `internal/automation/business.go`:284 · `ValidateBusinessDiagnostics` | 14 | 14 | 10 | +4 | 1.40x |
| CX-061 | complexity\|internal/automation/business.go\|readBusinessHTTP | 仍存在（精确匹配） | P3 | `internal/automation/business.go`:456 · `readBusinessHTTP` | 12 | 12 | 10 | +2 | 1.20x |
| CX-062 | complexity\|internal/automation/condition.go\|Checker.Check | 仍存在（精确匹配） | P3 | `internal/automation/condition.go`:26 · `Checker.Check` | 11 | 11 | 10 | +1 | 1.10x |
| CX-063 | complexity\|internal/automation/engine.go\|*Engine.runTask | 仍存在（精确匹配） | P2 | `internal/automation/engine.go`:161 · `*Engine.runTask` | 18 | 18 | 10 | +8 | 1.80x |
| CX-064 | complexity\|internal/automation/engine.go\|*Engine.finishRun | 仍存在（精确匹配） | P2 | `internal/automation/engine.go`:254 · `*Engine.finishRun` | 16 | 16 | 10 | +6 | 1.60x |
| CX-065 | complexity\|internal/automation/export.go\|parseMarkdownExportBlocks | 仍存在（精确匹配） | P3 | `internal/automation/export.go`:142 · `parseMarkdownExportBlocks` | 11 | 11 | 10 | +1 | 1.10x |
| CX-066 | complexity\|internal/automation/lease.go\|*Store.AcquireLease | 仍存在（精确匹配） | P3 | `internal/automation/lease.go`:35 · `*Store.AcquireLease` | 11 | 11 | 10 | +1 | 1.10x |
| CX-067 | complexity\|internal/automation/lease.go\|*Store.ReleaseLease | 仍存在（精确匹配） | P3 | `internal/automation/lease.go`:103 · `*Store.ReleaseLease` | 12 | 12 | 10 | +2 | 1.20x |
| CX-068 | complexity\|internal/automation/migration_heartbeat.go\|legacyHeartbeatCalendarSchedule | 仍存在（精确匹配） | P3 | `internal/automation/migration_heartbeat.go`:170 · `legacyHeartbeatCalendarSchedule` | 11 | 11 | 10 | +1 | 1.10x |
| CX-069 | complexity\|internal/automation/migration.go\|MigrateHeartbeatIfNeeded | 仍存在（精确匹配） | P2 | `internal/automation/migration.go`:55 · `MigrateHeartbeatIfNeeded` | 15 | 15 | 10 | +5 | 1.50x |
| CX-070 | complexity\|internal/automation/runner.go\|*BootRunner.Run | 仍存在（精确匹配） | P2 | `internal/automation/runner.go`:69 · `*BootRunner.Run` | 16 | 16 | 10 | +6 | 1.60x |
| CX-071 | complexity\|internal/automation/schedule.go\|withinTimeWindow | 仍存在（精确匹配） | P2 | `internal/automation/schedule.go`:117 · `withinTimeWindow` | 17 | 17 | 10 | +7 | 1.70x |
| CX-072 | complexity\|internal/automation/schedule.go\|previousCalendarOccurrence | 仍存在（精确匹配） | P2 | `internal/automation/schedule.go`:157 · `previousCalendarOccurrence` | 15 | 15 | 10 | +5 | 1.50x |
| CX-073 | complexity\|internal/automation/schedule.go\|nextCalendarOccurrenceAfter | 仍存在（精确匹配） | P2 | `internal/automation/schedule.go`:203 · `nextCalendarOccurrenceAfter` | 15 | 15 | 10 | +5 | 1.50x |
| CX-074 | complexity\|internal/automation/service.go\|readRunEventSummaries | 仍存在（精确匹配） | P2 | `internal/automation/service.go`:402 · `readRunEventSummaries` | 16 | 16 | 10 | +6 | 1.60x |
| CX-075 | complexity\|internal/automation/store.go\|*Store.DeleteTaskTemplate | 仍存在（精确匹配） | P3 | `internal/automation/store.go`:158 · `*Store.DeleteTaskTemplate` | 14 | 14 | 10 | +4 | 1.40x |
| CX-076 | complexity\|internal/automation/store.go\|*Store.RestoreTaskTemplate | 仍存在（精确匹配） | P3 | `internal/automation/store.go`:212 · `*Store.RestoreTaskTemplate` | 14 | 14 | 10 | +4 | 1.40x |
| CX-077 | complexity\|internal/automation/store.go\|*Store.migrateLegacyTaskTemplates | 仍存在（精确匹配） | P2 | `internal/automation/store.go`:346 · `*Store.migrateLegacyTaskTemplates` | 15 | 15 | 10 | +5 | 1.50x |
| CX-078 | complexity\|internal/automation/templates.go\|taskTemplateInterval | 仍存在（精确匹配） | P3 | `internal/automation/templates.go`:192 · `taskTemplateInterval` | 11 | 11 | 10 | +1 | 1.10x |
| CX-079 | complexity\|internal/boot/boot.go\|Build | 仍存在（精确匹配） | P0 | `internal/boot/boot.go`:152 · `Build` | 178 | 178 | 10 | +168 | 17.80x |
| CX-080 | complexity\|internal/boot/boot.go\|subagentEffectiveIdentity | 仍存在（精确匹配） | P3 | `internal/boot/boot.go`:1454 · `subagentEffectiveIdentity` | 14 | 14 | 10 | +4 | 1.40x |
| CX-081 | complexity\|internal/boot/token_profile.go\|*toolSourceConnector.Execute | 仍存在（精确匹配） | P2 | `internal/boot/token_profile.go`:108 · `*toolSourceConnector.Execute` | 15 | 15 | 10 | +5 | 1.50x |
| CX-082 | complexity\|internal/bot/feishu/feishu.go\|*adapter.handleSDKMessage | 仍存在（精确匹配） | P2 | `internal/bot/feishu/feishu.go`:229 · `*adapter.handleSDKMessage` | 17 | 17 | 10 | +7 | 1.70x |
| CX-083 | complexity\|internal/bot/feishu/feishu.go\|*adapter.sendSDKContent | 仍存在（精确匹配） | P3 | `internal/bot/feishu/feishu.go`:523 · `*adapter.sendSDKContent` | 12 | 12 | 10 | +2 | 1.20x |
| CX-084 | complexity\|internal/bot/feishu/feishu.go\|*adapter.runWebhook | 仍存在（精确匹配） | P3 | `internal/bot/feishu/feishu.go`:661 · `*adapter.runWebhook` | 12 | 12 | 10 | +2 | 1.20x |
| CX-085 | complexity\|internal/bot/gateway.go\|*BotGateway.handleMessage | 仍存在（精确匹配） | P3 | `internal/bot/gateway.go`:279 · `*BotGateway.handleMessage` | 12 | 12 | 10 | +2 | 1.20x |
| CX-086 | complexity\|internal/bot/gateway.go\|*BotGateway.handleSlashCommand | 仍存在（精确匹配） | P0 | `internal/bot/gateway.go`:509 · `*BotGateway.handleSlashCommand` | 31 | 31 | 10 | +21 | 3.10x |
| CX-087 | complexity\|internal/bot/gateway.go\|*BotGateway.sessionOptionsForMessage | 仍存在（精确匹配） | P3 | `internal/bot/gateway.go`:988 · `*BotGateway.sessionOptionsForMessage` | 12 | 12 | 10 | +2 | 1.20x |
| CX-088 | complexity\|internal/bot/qq/gateway.go\|*adapter.getAccessToken | 仍存在（精确匹配） | P3 | `internal/bot/qq/gateway.go`:121 · `*adapter.getAccessToken` | 14 | 14 | 10 | +4 | 1.40x |
| CX-089 | complexity\|internal/bot/qq/gateway.go\|*adapter.connectGateway | 仍存在（精确匹配） | P1 | `internal/bot/qq/gateway.go`:214 · `*adapter.connectGateway` | 24 | 24 | 10 | +14 | 2.40x |
| CX-090 | complexity\|internal/bot/render.go\|*renderSink.Emit | 仍存在（精确匹配） | P1 | `internal/bot/render.go`:65 · `*renderSink.Emit` | 25 | 25 | 10 | +15 | 2.50x |
| CX-091 | complexity\|internal/bot/render.go\|lastSemanticBoundary | 仍存在（精确匹配） | P3 | `internal/bot/render.go`:286 · `lastSemanticBoundary` | 11 | 11 | 10 | +1 | 1.10x |
| CX-092 | complexity\|internal/bot/weixin/weixin_login.go\|LoginWithPaths | 仍存在（精确匹配） | P3 | `internal/bot/weixin/weixin_login.go`:149 · `LoginWithPaths` | 14 | 14 | 10 | +4 | 1.40x |
| CX-093 | complexity\|internal/bot/weixin/weixin_login.go\|PollLoginWithPaths | 仍存在（精确匹配） | P1 | `internal/bot/weixin/weixin_login.go`:223 · `PollLoginWithPaths` | 20 | 20 | 10 | +10 | 2.00x |
| CX-094 | complexity\|internal/bot/weixin/weixin.go\|*adapter.getUpdates | 仍存在（精确匹配） | P3 | `internal/bot/weixin/weixin.go`:357 · `*adapter.getUpdates` | 11 | 11 | 10 | +1 | 1.10x |
| CX-095 | complexity\|internal/botruntime/runtime.go\|PlatformConfigured | 仍存在（精确匹配） | P3 | `internal/botruntime/runtime.go`:84 · `PlatformConfigured` | 11 | 11 | 10 | +1 | 1.10x |
| CX-096 | complexity\|internal/botruntime/runtime.go\|ChannelConfigs | 仍存在（精确匹配） | P3 | `internal/botruntime/runtime.go`:110 · `ChannelConfigs` | 12 | 12 | 10 | +2 | 1.20x |
| CX-097 | complexity\|internal/botruntime/runtime.go\|ConnectionChannelConfigs | 仍存在（精确匹配） | P3 | `internal/botruntime/runtime.go`:145 · `ConnectionChannelConfigs` | 12 | 12 | 10 | +2 | 1.20x |
| CX-098 | complexity\|internal/botruntime/runtime.go\|AdapterBindingsWithPaths | 仍存在（精确匹配） | P2 | `internal/botruntime/runtime.go`:195 · `AdapterBindingsWithPaths` | 18 | 18 | 10 | +8 | 1.80x |
| CX-099 | complexity\|internal/botruntime/runtime.go\|rememberInbound | 仍存在（精确匹配） | P2 | `internal/botruntime/runtime.go`:358 · `rememberInbound` | 18 | 18 | 10 | +8 | 1.80x |
| CX-100 | complexity\|internal/botruntime/runtime.go\|rememberAllowlist | 仍存在（精确匹配） | P3 | `internal/botruntime/runtime.go`:496 · `rememberAllowlist` | 12 | 12 | 10 | +2 | 1.20x |
| CX-101 | complexity\|internal/checkpoint/checkpoint.go\|*Store.RestoreCode | 仍存在（精确匹配） | P3 | `internal/checkpoint/checkpoint.go`:239 · `*Store.RestoreCode` | 14 | 14 | 10 | +4 | 1.40x |
| CX-102 | complexity\|internal/cli/acp.go\|*acpFactory.SessionConfigState | 仍存在（精确匹配） | P2 | `internal/cli/acp.go`:96 · `*acpFactory.SessionConfigState` | 17 | 17 | 10 | +7 | 1.70x |
| CX-103 | complexity\|internal/cli/bot.go\|botStart | 仍存在（精确匹配） | P3 | `internal/cli/bot.go`:47 · `botStart` | 12 | 12 | 10 | +2 | 1.20x |
| CX-104 | complexity\|internal/cli/bot.go\|botDoctor | 仍存在（精确匹配） | P0 | `internal/cli/bot.go`:164 · `botDoctor` | 31 | 31 | 10 | +21 | 3.10x |
| CX-105 | complexity\|internal/cli/chat_tui_paste.go\|pastedImagePath | 仍存在（精确匹配） | P2 | `internal/cli/chat_tui_paste.go`:235 · `pastedImagePath` | 15 | 15 | 10 | +5 | 1.50x |
| CX-106 | complexity\|internal/cli/chat_tui.go\|chatTUI.update | 仍存在（精确匹配） | P0 | `internal/cli/chat_tui.go`:713 · `chatTUI.update` | 176 | 176 | 10 | +166 | 17.60x |
| CX-107 | complexity\|internal/cli/chat_tui.go\|chatTUI.renderTranscriptWithMainManager | 仍存在（精确匹配） | P3 | `internal/cli/chat_tui.go`:1568 · `chatTUI.renderTranscriptWithMainManager` | 12 | 12 | 10 | +2 | 1.20x |
| CX-108 | complexity\|internal/cli/chat_tui.go\|*chatTUI.streamToolOutput | 仍存在（精确匹配） | P2 | `internal/cli/chat_tui.go`:1672 · `*chatTUI.streamToolOutput` | 17 | 17 | 10 | +7 | 1.70x |
| CX-109 | complexity\|internal/cli/chat_tui.go\|*chatTUI.collapseToolOutput | 仍存在（精确匹配） | P2 | `internal/cli/chat_tui.go`:1765 · `*chatTUI.collapseToolOutput` | 16 | 16 | 10 | +6 | 1.60x |
| CX-110 | complexity\|internal/cli/chat_tui.go\|*chatTUI.collapseShellSlot | 仍存在（精确匹配） | P2 | `internal/cli/chat_tui.go`:1826 · `*chatTUI.collapseShellSlot` | 15 | 15 | 10 | +5 | 1.50x |
| CX-111 | complexity\|internal/cli/chat_tui.go\|*chatTUI.toggleShellOutput | 仍存在（精确匹配） | P3 | `internal/cli/chat_tui.go`:1890 · `*chatTUI.toggleShellOutput` | 13 | 13 | 10 | +3 | 1.30x |
| CX-112 | complexity\|internal/cli/chat_tui.go\|chatTUI.View | 仍存在（精确匹配） | P0 | `internal/cli/chat_tui.go`:2210 · `chatTUI.View` | 47 | 47 | 10 | +37 | 4.70x |
| CX-113 | complexity\|internal/cli/chat_tui.go\|chatTUI.renderTodoPanel | 仍存在（精确匹配） | P3 | `internal/cli/chat_tui.go`:2620 · `chatTUI.renderTodoPanel` | 13 | 13 | 10 | +3 | 1.30x |
| CX-114 | complexity\|internal/cli/chat_tui.go\|chatTUI.computeStatusLineCount | 仍存在（精确匹配） | P1 | `internal/cli/chat_tui.go`:2717 · `chatTUI.computeStatusLineCount` | 25 | 25 | 10 | +15 | 2.50x |
| CX-115 | complexity\|internal/cli/chat_tui.go\|chatTUI.modeTagText | 仍存在（精确匹配） | P1 | `internal/cli/chat_tui.go`:2872 · `chatTUI.modeTagText` | 25 | 25 | 10 | +15 | 2.50x |
| CX-116 | complexity\|internal/cli/chat_tui.go\|*chatTUI.ingestEvent | 仍存在（精确匹配） | P0 | `internal/cli/chat_tui.go`:3004 · `*chatTUI.ingestEvent` | 47 | 47 | 10 | +37 | 4.70x |
| CX-117 | complexity\|internal/cli/chat_tui.go\|*chatTUI.runSlashCommand | 仍存在（精确匹配） | P0 | `internal/cli/chat_tui.go`:3226 · `*chatTUI.runSlashCommand` | 50 | 50 | 10 | +40 | 5.00x |
| CX-118 | complexity\|internal/cli/chat_tui.go\|*chatTUI.runMCPSubcommand | 仍存在（精确匹配） | P2 | `internal/cli/chat_tui.go`:3492 · `*chatTUI.runMCPSubcommand` | 19 | 19 | 10 | +9 | 1.90x |
| CX-119 | complexity\|internal/cli/chooser.go\|chatTUI.handleChooserKey | 仍存在（精确匹配） | P1 | `internal/cli/chooser.go`:89 · `chatTUI.handleChooserKey` | 22 | 22 | 10 | +12 | 2.20x |
| CX-120 | complexity\|internal/cli/cli.go\|Run | 仍存在（精确匹配） | P1 | `internal/cli/cli.go`:51 · `Run` | 27 | 27 | 10 | +17 | 2.70x |
| CX-121 | complexity\|internal/cli/cli.go\|runAgent | 仍存在（精确匹配） | P1 | `internal/cli/cli.go`:387 · `runAgent` | 26 | 26 | 10 | +16 | 2.60x |
| CX-122 | complexity\|internal/cli/cli.go\|runServe | 仍存在（精确匹配） | P1 | `internal/cli/cli.go`:508 · `runServe` | 24 | 24 | 10 | +14 | 2.40x |
| CX-123 | complexity\|internal/cli/cli.go\|chatREPL | 仍存在（精确匹配） | P0 | `internal/cli/cli.go`:652 · `chatREPL` | 37 | 37 | 10 | +27 | 3.70x |
| CX-124 | complexity\|internal/cli/cli.go\|selectEnabledProviders | 仍存在（精确匹配） | P2 | `internal/cli/cli.go`:1120 · `selectEnabledProviders` | 15 | 15 | 10 | +5 | 1.50x |
| CX-125 | complexity\|internal/cli/cli.go\|providersWithMissingKeys | 仍存在（精确匹配） | P3 | `internal/cli/cli.go`:1688 · `providersWithMissingKeys` | 12 | 12 | 10 | +2 | 1.20x |
| CX-126 | complexity\|internal/cli/cli.go\|appendEnv | 仍存在（精确匹配） | P2 | `internal/cli/cli.go`:1800 · `appendEnv` | 16 | 16 | 10 | +6 | 1.60x |
| CX-127 | complexity\|internal/cli/cli.go\|configMemoryV5Command | 仍存在（精确匹配） | P3 | `internal/cli/cli.go`:1927 · `configMemoryV5Command` | 11 | 11 | 10 | +1 | 1.10x |
| CX-128 | complexity\|internal/cli/cli.go\|configReasoningLanguageCommand | 仍存在（精确匹配） | P2 | `internal/cli/cli.go`:1974 · `configReasoningLanguageCommand` | 15 | 15 | 10 | +5 | 1.50x |
| CX-129 | complexity\|internal/cli/complete.go\|*chatTUI.updateCompletion | 仍存在（精确匹配） | P3 | `internal/cli/complete.go`:119 · `*chatTUI.updateCompletion` | 11 | 11 | 10 | +1 | 1.10x |
| CX-130 | complexity\|internal/cli/complete.go\|*chatTUI.branchArgItems | 仍存在（精确匹配） | P3 | `internal/cli/complete.go`:277 · `*chatTUI.branchArgItems` | 12 | 12 | 10 | +2 | 1.20x |
| CX-131 | complexity\|internal/cli/complete.go\|*chatTUI.fileItems | 仍存在（精确匹配） | P1 | `internal/cli/complete.go`:410 · `*chatTUI.fileItems` | 21 | 21 | 10 | +11 | 2.10x |
| CX-132 | complexity\|internal/cli/complete.go\|chatTUI.renderCompletion | 仍存在（精确匹配） | P3 | `internal/cli/complete.go`:652 · `chatTUI.renderCompletion` | 11 | 11 | 10 | +1 | 1.10x |
| CX-133 | complexity\|internal/cli/diffview.go\|diffBody | 仍存在（精确匹配） | P2 | `internal/cli/diffview.go`:77 · `diffBody` | 16 | 16 | 10 | +6 | 1.60x |
| CX-134 | complexity\|internal/cli/effort.go\|*chatTUI.runEffortCommand | 仍存在（精确匹配） | P1 | `internal/cli/effort.go`:12 · `*chatTUI.runEffortCommand` | 20 | 20 | 10 | +10 | 2.00x |
| CX-135 | complexity\|internal/cli/gitstatus.go\|loadGitStatus | 仍存在（精确匹配） | P3 | `internal/cli/gitstatus.go`:41 · `loadGitStatus` | 12 | 12 | 10 | +2 | 1.20x |
| CX-136 | complexity\|internal/cli/language.go\|*chatTUI.runLanguageSubcommand | 仍存在（精确匹配） | P3 | `internal/cli/language.go`:13 · `*chatTUI.runLanguageSubcommand` | 11 | 11 | 10 | +1 | 1.10x |
| CX-137 | complexity\|internal/cli/latex.go\|convertCommand | 仍存在（精确匹配） | P2 | `internal/cli/latex.go`:49 · `convertCommand` | 17 | 17 | 10 | +7 | 1.70x |
| CX-138 | complexity\|internal/cli/latex.go\|readAtom | 仍存在（精确匹配） | P2 | `internal/cli/latex.go`:117 · `readAtom` | 15 | 15 | 10 | +5 | 1.50x |
| CX-139 | complexity\|internal/cli/latex.go\|normalizeMath | 仍存在（精确匹配） | P1 | `internal/cli/latex.go`:255 · `normalizeMath` | 22 | 22 | 10 | +12 | 2.20x |
| CX-140 | complexity\|internal/cli/mathnode.go\|*mathParser.Parse | 仍存在（精确匹配） | P3 | `internal/cli/mathnode.go`:26 · `*mathParser.Parse` | 14 | 14 | 10 | +4 | 1.40x |
| CX-141 | complexity\|internal/cli/mcp_import_picker.go\|chatTUI.handleMCPImportKey | 仍存在（精确匹配） | P3 | `internal/cli/mcp_import_picker.go`:40 · `chatTUI.handleMCPImportKey` | 12 | 12 | 10 | +2 | 1.20x |
| CX-142 | complexity\|internal/cli/mcp_manager_actions.go\|chatTUI.applyMCPMode | 仍存在（精确匹配） | P3 | `internal/cli/mcp_manager_actions.go`:134 · `chatTUI.applyMCPMode` | 14 | 14 | 10 | +4 | 1.40x |
| CX-143 | complexity\|internal/cli/mcp_manager_view.go\|*mcpManager.renderList | 仍存在（精确匹配） | P3 | `internal/cli/mcp_manager_view.go`:53 · `*mcpManager.renderList` | 11 | 11 | 10 | +1 | 1.10x |
| CX-144 | complexity\|internal/cli/mcp_manager_view.go\|*mcpManager.renderDetail | 仍存在（精确匹配） | P3 | `internal/cli/mcp_manager_view.go`:121 · `*mcpManager.renderDetail` | 11 | 11 | 10 | +1 | 1.10x |
| CX-145 | complexity\|internal/cli/mcp_manager_view.go\|mcpActionsFor | 仍存在（精确匹配） | P3 | `internal/cli/mcp_manager_view.go`:235 · `mcpActionsFor` | 12 | 12 | 10 | +2 | 1.20x |
| CX-146 | complexity\|internal/cli/mcp_manager.go\|chatTUI.handleMCPManagerKey | 仍存在（精确匹配） | P0 | `internal/cli/mcp_manager.go`:115 · `chatTUI.handleMCPManagerKey` | 45 | 45 | 10 | +35 | 4.50x |
| CX-147 | complexity\|internal/cli/mcp_manager.go\|chatTUI.buildMCPSnapshot | 仍存在（精确匹配） | P2 | `internal/cli/mcp_manager.go`:298 · `chatTUI.buildMCPSnapshot` | 16 | 16 | 10 | +6 | 1.60x |
| CX-148 | complexity\|internal/cli/mcp.go\|parseMCPAdd | 仍存在（精确匹配） | P1 | `internal/cli/mcp.go`:26 · `parseMCPAdd` | 27 | 27 | 10 | +17 | 2.70x |
| CX-149 | complexity\|internal/cli/md.go\|fixCJKEmphasis | 仍存在（精确匹配） | P2 | `internal/cli/md.go`:80 · `fixCJKEmphasis` | 18 | 18 | 10 | +8 | 1.80x |
| CX-150 | complexity\|internal/cli/md.go\|isCJKPunct | 仍存在（精确匹配） | P3 | `internal/cli/md.go`:140 · `isCJKPunct` | 12 | 12 | 10 | +2 | 1.20x |
| CX-151 | complexity\|internal/cli/md.go\|*mdRenderer.appendInline | 仍存在（精确匹配） | P3 | `internal/cli/md.go`:316 · `*mdRenderer.appendInline` | 13 | 13 | 10 | +3 | 1.30x |
| CX-152 | complexity\|internal/cli/md.go\|*mdRenderer.renderTable | 仍存在（精确匹配） | P1 | `internal/cli/md.go`:365 · `*mdRenderer.renderTable` | 23 | 23 | 10 | +13 | 2.30x |
| CX-153 | complexity\|internal/cli/memory_v5.go\|*chatTUI.runMemoryV5Command | 仍存在（精确匹配） | P3 | `internal/cli/memory_v5.go`:10 · `*chatTUI.runMemoryV5Command` | 12 | 12 | 10 | +2 | 1.20x |
| CX-154 | complexity\|internal/cli/memory_view.go\|renderMemory | 仍存在（精确匹配） | P2 | `internal/cli/memory_view.go`:11 · `renderMemory` | 17 | 17 | 10 | +7 | 1.70x |
| CX-155 | complexity\|internal/cli/provider.go\|*chatTUI.switchToProvider | 仍存在（精确匹配） | P3 | `internal/cli/provider.go`:73 · `*chatTUI.switchToProvider` | 14 | 14 | 10 | +4 | 1.40x |
| CX-156 | complexity\|internal/cli/reasoning_language.go\|*chatTUI.runReasoningLanguageCommand | 仍存在（精确匹配） | P3 | `internal/cli/reasoning_language.go`:10 · `*chatTUI.runReasoningLanguageCommand` | 11 | 11 | 10 | +1 | 1.10x |
| CX-157 | complexity\|internal/cli/review.go\|reviewCommand | 仍存在（精确匹配） | P3 | `internal/cli/review.go`:19 · `reviewCommand` | 12 | 12 | 10 | +2 | 1.20x |
| CX-158 | complexity\|internal/cli/rewind.go\|chatTUI.handleRewindKey | 仍存在（精确匹配） | P1 | `internal/cli/rewind.go`:50 · `chatTUI.handleRewindKey` | 25 | 25 | 10 | +15 | 2.50x |
| CX-159 | complexity\|internal/cli/select.go\|selectOne | 仍存在（精确匹配） | P0 | `internal/cli/select.go`:81 · `selectOne` | 49 | 49 | 10 | +39 | 4.90x |
| CX-160 | complexity\|internal/cli/select.go\|selectMany | 仍存在（精确匹配） | P0 | `internal/cli/select.go`:267 · `selectMany` | 73 | 73 | 10 | +63 | 7.30x |
| CX-161 | complexity\|internal/cli/skill_hooks.go\|*chatTUI.runSkillSubcommand | 仍存在（精确匹配） | P3 | `internal/cli/skill_hooks.go`:17 · `*chatTUI.runSkillSubcommand` | 13 | 13 | 10 | +3 | 1.30x |
| CX-162 | complexity\|internal/cli/skill_hooks.go\|*chatTUI.skillSaveEnabledChanges | 仍存在（精确匹配） | P3 | `internal/cli/skill_hooks.go`:105 · `*chatTUI.skillSaveEnabledChanges` | 13 | 13 | 10 | +3 | 1.30x |
| CX-163 | complexity\|internal/cli/skill_picker.go\|chatTUI.handleSkillPickerKey | 仍存在（精确匹配） | P1 | `internal/cli/skill_picker.go`:89 · `chatTUI.handleSkillPickerKey` | 20 | 20 | 10 | +10 | 2.00x |
| CX-164 | complexity\|internal/cli/skill_picker.go\|chatTUI.handleSkillPickerSkillsKey | 仍存在（精确匹配） | P3 | `internal/cli/skill_picker.go`:145 · `chatTUI.handleSkillPickerSkillsKey` | 13 | 13 | 10 | +3 | 1.30x |
| CX-165 | complexity\|internal/cli/skill_picker.go\|chatTUI.handleSkillPickerSourcesKey | 仍存在（精确匹配） | P3 | `internal/cli/skill_picker.go`:179 · `chatTUI.handleSkillPickerSourcesKey` | 11 | 11 | 10 | +1 | 1.10x |
| CX-166 | complexity\|internal/cli/skill_picker.go\|chatTUI.handleSkillPickerSourceSkillsKey | 仍存在（精确匹配） | P3 | `internal/cli/skill_picker.go`:211 · `chatTUI.handleSkillPickerSourceSkillsKey` | 12 | 12 | 10 | +2 | 1.20x |
| CX-167 | complexity\|internal/cli/skill_picker.go\|skillRootLines | 仍存在（精确匹配） | P3 | `internal/cli/skill_picker.go`:524 · `skillRootLines` | 12 | 12 | 10 | +2 | 1.20x |
| CX-168 | complexity\|internal/cli/theme_osc_unix.go\|queryTerminalBackground | 仍存在（精确匹配） | P2 | `internal/cli/theme_osc_unix.go`:19 · `queryTerminalBackground` | 16 | 16 | 10 | +6 | 1.60x |
| CX-169 | complexity\|internal/cli/theme.go\|*chatTUI.themeArgItems | 仍存在（精确匹配） | P3 | `internal/cli/theme.go`:501 · `*chatTUI.themeArgItems` | 11 | 11 | 10 | +1 | 1.10x |
| CX-170 | complexity\|internal/cli/upgrade.go\|upgradeCommand | 仍存在（精确匹配） | P2 | `internal/cli/upgrade.go`:50 · `upgradeCommand` | 19 | 19 | 10 | +9 | 1.90x |
| CX-171 | complexity\|internal/config/ccswitch.go\|classifyMCPImportCandidate | 仍存在（精确匹配） | P2 | `internal/config/ccswitch.go`:223 · `classifyMCPImportCandidate` | 16 | 16 | 10 | +6 | 1.60x |
| CX-172 | complexity\|internal/config/credentials.go\|storeCredentialsInFile | 仍存在（精确匹配） | P3 | `internal/config/credentials.go`:730 · `storeCredentialsInFile` | 12 | 12 | 10 | +2 | 1.20x |
| CX-173 | complexity\|internal/config/credentials.go\|writeCredentialFileLines | 仍存在（精确匹配） | P3 | `internal/config/credentials.go`:806 · `writeCredentialFileLines` | 11 | 11 | 10 | +1 | 1.10x |
| CX-174 | complexity\|internal/config/credentials.go\|isCredentialKey | 仍存在（精确匹配） | P3 | `internal/config/credentials.go`:864 · `isCredentialKey` | 11 | 11 | 10 | +1 | 1.10x |
| CX-175 | complexity\|internal/config/dotenv.go\|loadDotEnvFileAs | 仍存在（精确匹配） | P3 | `internal/config/dotenv.go`:90 · `loadDotEnvFileAs` | 11 | 11 | 10 | +1 | 1.10x |
| CX-176 | complexity\|internal/config/edit.go\|*Config.RemoveProvider | 仍存在（精确匹配） | P2 | `internal/config/edit.go`:402 · `*Config.RemoveProvider` | 17 | 17 | 10 | +7 | 1.70x |
| CX-177 | complexity\|internal/config/edit.go\|*Config.saveProjectIncremental | 仍存在（精确匹配） | P3 | `internal/config/edit.go`:844 · `*Config.saveProjectIncremental` | 12 | 12 | 10 | +2 | 1.20x |
| CX-178 | complexity\|internal/config/edit.go\|mergeTOMLDelta | 仍存在（精确匹配） | P3 | `internal/config/edit.go`:887 · `mergeTOMLDelta` | 12 | 12 | 10 | +2 | 1.20x |
| CX-179 | complexity\|internal/config/edit.go\|replaceTOMLSection | 仍存在（精确匹配） | P2 | `internal/config/edit.go`:1020 · `replaceTOMLSection` | 18 | 18 | 10 | +8 | 1.80x |
| CX-180 | complexity\|internal/config/effort.go\|EffortCapabilityForEntry | 仍存在（精确匹配） | P3 | `internal/config/effort.go`:39 · `EffortCapabilityForEntry` | 13 | 13 | 10 | +3 | 1.30x |
| CX-181 | complexity\|internal/config/effort.go\|NormalizeEffort | 仍存在（精确匹配） | P1 | `internal/config/effort.go`:86 · `NormalizeEffort` | 20 | 20 | 10 | +10 | 2.00x |
| CX-182 | complexity\|internal/config/load.go\|LoadForRootWithPaths | 仍存在（精确匹配） | P3 | `internal/config/load.go`:42 · `LoadForRootWithPaths` | 12 | 12 | 10 | +2 | 1.20x |
| CX-183 | complexity\|internal/config/load.go\|mergeTOMLProviders | 仍存在（精确匹配） | P3 | `internal/config/load.go`:303 · `mergeTOMLProviders` | 11 | 11 | 10 | +1 | 1.10x |
| CX-184 | complexity\|internal/config/load.go\|applyLegacyMimoCatalog | 仍存在（精确匹配） | P3 | `internal/config/load.go`:640 · `applyLegacyMimoCatalog` | 13 | 13 | 10 | +3 | 1.30x |
| CX-185 | complexity\|internal/config/load.go\|NormalizeLegacyDesktopProviderAccess | 仍存在（精确匹配） | P2 | `internal/config/load.go`:927 · `NormalizeLegacyDesktopProviderAccess` | 15 | 15 | 10 | +5 | 1.50x |
| CX-186 | complexity\|internal/config/mcpjson.go\|loadLegacyMCP | 仍存在（精确匹配） | P3 | `internal/config/mcpjson.go`:105 · `loadLegacyMCP` | 12 | 12 | 10 | +2 | 1.20x |
| CX-187 | complexity\|internal/config/mcpjson.go\|clearMCPJSONAuthentication | 仍存在（精确匹配） | P3 | `internal/config/mcpjson.go`:329 · `clearMCPJSONAuthentication` | 14 | 14 | 10 | +4 | 1.40x |
| CX-188 | complexity\|internal/config/migrate.go\|migrateMCPToUserConfig | 仍存在（精确匹配） | P2 | `internal/config/migrate.go`:105 · `migrateMCPToUserConfig` | 16 | 16 | 10 | +6 | 1.60x |
| CX-189 | complexity\|internal/config/migrate.go\|migrateLegacyCredentialsIfNeededForRoot | 仍存在（精确匹配） | P3 | `internal/config/migrate.go`:238 · `migrateLegacyCredentialsIfNeededForRoot` | 12 | 12 | 10 | +2 | 1.20x |
| CX-190 | complexity\|internal/config/migrate.go\|migrateLegacyTOMLIfNeeded | 仍存在（精确匹配） | P3 | `internal/config/migrate.go`:323 · `migrateLegacyTOMLIfNeeded` | 12 | 12 | 10 | +2 | 1.20x |
| CX-191 | complexity\|internal/config/migrate.go\|legacyPlugins | 仍存在（精确匹配） | P3 | `internal/config/migrate.go`:398 · `legacyPlugins` | 11 | 11 | 10 | +1 | 1.10x |
| CX-192 | complexity\|internal/config/paths.go\|cleanExplicitDir | 仍存在（精确匹配） | P3 | `internal/config/paths.go`:136 · `cleanExplicitDir` | 11 | 11 | 10 | +1 | 1.10x |
| CX-193 | complexity\|internal/config/paths.go\|cleanEnvDir | 仍存在（精确匹配） | P3 | `internal/config/paths.go`:322 · `cleanEnvDir` | 11 | 11 | 10 | +1 | 1.10x |
| CX-194 | complexity\|internal/config/paths.go\|CommandDirsForRoot | 仍存在（精确匹配） | P3 | `internal/config/paths.go`:548 · `CommandDirsForRoot` | 12 | 12 | 10 | +2 | 1.20x |
| CX-195 | complexity\|internal/config/render.go\|RenderTOMLForScope | 仍存在（精确匹配） | P0 | `internal/config/render.go`:32 · `RenderTOMLForScope` | 98 | 98 | 10 | +88 | 9.80x |
| CX-196 | complexity\|internal/config/render.go\|RenderTOMLProjectDelta | 仍存在（精确匹配） | P0 | `internal/config/render.go`:575 · `RenderTOMLProjectDelta` | 124 | 124 | 10 | +114 | 12.40x |
| CX-197 | complexity\|internal/config/render.go\|isBareTOMLKey | 仍存在（精确匹配） | P3 | `internal/config/render.go`:1118 · `isBareTOMLKey` | 11 | 11 | 10 | +1 | 1.10x |
| CX-198 | complexity\|internal/control/attachments.go\|SaveAttachmentDataURL | 仍存在（精确匹配） | P3 | `internal/control/attachments.go`:32 · `SaveAttachmentDataURL` | 11 | 11 | 10 | +1 | 1.10x |
| CX-199 | complexity\|internal/control/attachments.go\|SaveImageBytes | 仍存在（精确匹配） | P3 | `internal/control/attachments.go`:86 · `SaveImageBytes` | 11 | 11 | 10 | +1 | 1.10x |
| CX-200 | complexity\|internal/control/attachments.go\|SaveImageFile | 仍存在（精确匹配） | P2 | `internal/control/attachments.go`:121 · `SaveImageFile` | 15 | 15 | 10 | +5 | 1.50x |
| CX-201 | complexity\|internal/control/attachments.go\|SaveAttachmentFile | 仍存在（精确匹配） | P1 | `internal/control/attachments.go`:159 · `SaveAttachmentFile` | 20 | 20 | 10 | +10 | 2.00x |
| CX-202 | complexity\|internal/control/attachments.go\|readAttachmentImage | 仍存在（精确匹配） | P2 | `internal/control/attachments.go`:290 · `readAttachmentImage` | 17 | 17 | 10 | +7 | 1.70x |
| CX-203 | complexity\|internal/control/auto_plan_classifier.go\|*ProviderAutoPlanClassifier.NeedsPlan | 仍存在（精确匹配） | P3 | `internal/control/auto_plan_classifier.go`:42 · `*ProviderAutoPlanClassifier.NeedsPlan` | 13 | 13 | 10 | +3 | 1.30x |
| CX-204 | complexity\|internal/control/auto_plan.go\|autoPlanScore | 仍存在（精确匹配） | P2 | `internal/control/auto_plan.go`:96 · `autoPlanScore` | 15 | 15 | 10 | +5 | 1.50x |
| CX-205 | complexity\|internal/control/auto_plan.go\|isContextDependentShortReply | 仍存在（精确匹配） | P3 | `internal/control/auto_plan.go`:139 · `isContextDependentShortReply` | 11 | 11 | 10 | +1 | 1.10x |
| CX-206 | complexity\|internal/control/auto_plan.go\|isLowRiskQuestion | 仍存在（精确匹配） | P0 | `internal/control/auto_plan.go`:207 · `isLowRiskQuestion` | 50 | 50 | 10 | +40 | 5.00x |
| CX-207 | complexity\|internal/control/branches.go\|FormatBranchTree | 仍存在（精确匹配） | P3 | `internal/control/branches.go`:39 · `FormatBranchTree` | 12 | 12 | 10 | +2 | 1.20x |
| CX-208 | complexity\|internal/control/branches.go\|shortBranchID | 仍存在（精确匹配） | P3 | `internal/control/branches.go`:143 · `shortBranchID` | 11 | 11 | 10 | +1 | 1.10x |
| CX-209 | complexity\|internal/control/controller.go\|*Controller.submitCommandOrTurn | 仍存在（精确匹配） | P0 | `internal/control/controller.go`:698 · `*Controller.submitCommandOrTurn` | 35 | 35 | 10 | +25 | 3.50x |
| CX-210 | complexity\|internal/control/controller.go\|*Controller.applyPlanExec | 仍存在（精确匹配） | P3 | `internal/control/controller.go`:934 · `*Controller.applyPlanExec` | 14 | 14 | 10 | +4 | 1.40x |
| CX-211 | complexity\|internal/control/controller.go\|*Controller.DeleteUserExpert | 仍存在（精确匹配） | P3 | `internal/control/controller.go`:1706 · `*Controller.DeleteUserExpert` | 12 | 12 | 10 | +2 | 1.20x |
| CX-212 | complexity\|internal/control/controller.go\|*Controller.ClearSession | 仍存在（精确匹配） | P3 | `internal/control/controller.go`:1977 · `*Controller.ClearSession` | 14 | 14 | 10 | +4 | 1.40x |
| CX-213 | complexity\|internal/control/controller.go\|removeSessionArtifacts | 仍存在（精确匹配） | P3 | `internal/control/controller.go`:2046 · `removeSessionArtifacts` | 12 | 12 | 10 | +2 | 1.20x |
| CX-214 | complexity\|internal/control/controller.go\|*Controller.Rewind | 仍存在（精确匹配） | P3 | `internal/control/controller.go`:2111 · `*Controller.Rewind` | 14 | 14 | 10 | +4 | 1.40x |
| CX-215 | complexity\|internal/control/controller.go\|*Controller.forkNamed | 仍存在（精确匹配） | P3 | `internal/control/controller.go`:2172 · `*Controller.forkNamed` | 11 | 11 | 10 | +1 | 1.10x |
| CX-216 | complexity\|internal/control/controller.go\|resolveBranch | 仍存在（精确匹配） | P3 | `internal/control/controller.go`:2362 · `resolveBranch` | 11 | 11 | 10 | +1 | 1.10x |
| CX-217 | complexity\|internal/control/controller.go\|*Controller.maybeColdResumePrune | 仍存在（精确匹配） | P3 | `internal/control/controller.go`:2538 · `*Controller.maybeColdResumePrune` | 11 | 11 | 10 | +1 | 1.10x |
| CX-218 | complexity\|internal/control/controller.go\|listItem | 仍存在（精确匹配） | P2 | `internal/control/controller.go`:3764 · `listItem` | 18 | 18 | 10 | +8 | 1.80x |
| CX-219 | complexity\|internal/control/controller.go\|*Controller.requestApprovalWithReason | 仍存在（精确匹配） | P3 | `internal/control/controller.go`:3863 · `*Controller.requestApprovalWithReason` | 13 | 13 | 10 | +3 | 1.30x |
| CX-220 | complexity\|internal/control/errmsg.go\|explainError | 仍存在（精确匹配） | P3 | `internal/control/errmsg.go`:15 · `explainError` | 12 | 12 | 10 | +2 | 1.20x |
| CX-221 | complexity\|internal/control/goal.go\|*goalMachine.advance | 仍存在（精确匹配） | P1 | `internal/control/goal.go`:186 · `*goalMachine.advance` | 20 | 20 | 10 | +10 | 2.00x |
| CX-222 | complexity\|internal/control/imagecompress.go\|compressForVision | 仍存在（精确匹配） | P3 | `internal/control/imagecompress.go`:28 · `compressForVision` | 11 | 11 | 10 | +1 | 1.10x |
| CX-223 | complexity\|internal/control/input.go\|shouldAutoStartResearchGoal | 仍存在（精确匹配） | P2 | `internal/control/input.go`:281 · `shouldAutoStartResearchGoal` | 18 | 18 | 10 | +8 | 1.80x |
| CX-224 | complexity\|internal/control/profile_management.go\|*Controller.profileCommandText | 仍存在（精确匹配） | P1 | `internal/control/profile_management.go`:79 · `*Controller.profileCommandText` | 21 | 21 | 10 | +11 | 2.10x |
| CX-225 | complexity\|internal/control/refs.go\|*Controller.detectRefsMode | 仍存在（精确匹配） | P3 | `internal/control/refs.go`:112 · `*Controller.detectRefsMode` | 12 | 12 | 10 | +2 | 1.20x |
| CX-226 | complexity\|internal/control/refs.go\|visionFileImageDataURL | 仍存在（精确匹配） | P3 | `internal/control/refs.go`:177 · `visionFileImageDataURL` | 13 | 13 | 10 | +3 | 1.30x |
| CX-227 | complexity\|internal/control/refs.go\|resolveBareNames | 仍存在（精确匹配） | P3 | `internal/control/refs.go`:242 · `resolveBareNames` | 14 | 14 | 10 | +4 | 1.40x |
| CX-228 | complexity\|internal/control/refs.go\|readFileRef | 仍存在（精确匹配） | P2 | `internal/control/refs.go`:481 · `readFileRef` | 17 | 17 | 10 | +7 | 1.70x |
| CX-229 | complexity\|internal/control/refs.go\|readFileRefUnscoped | 仍存在（精确匹配） | P1 | `internal/control/refs.go`:550 · `readFileRefUnscoped` | 20 | 20 | 10 | +10 | 2.00x |
| CX-230 | complexity\|internal/control/refs.go\|walkRootDir | 仍存在（精确匹配） | P3 | `internal/control/refs.go`:635 · `walkRootDir` | 11 | 11 | 10 | +1 | 1.10x |
| CX-231 | complexity\|internal/control/slash.go\|SlashArgItems | 仍存在（精确匹配） | P2 | `internal/control/slash.go`:57 · `SlashArgItems` | 16 | 16 | 10 | +6 | 1.60x |
| CX-232 | complexity\|internal/control/slash.go\|mcpArgItems | 仍存在（精确匹配） | P3 | `internal/control/slash.go`:253 · `mcpArgItems` | 13 | 13 | 10 | +3 | 1.30x |
| CX-233 | complexity\|internal/control/slash.go\|skillArgItems | 仍存在（精确匹配） | P3 | `internal/control/slash.go`:350 · `skillArgItems` | 12 | 12 | 10 | +2 | 1.20x |
| CX-234 | complexity\|internal/control/slash.go\|*Controller.managementNotice | 仍存在（精确匹配） | P0 | `internal/control/slash.go`:420 · `*Controller.managementNotice` | 31 | 31 | 10 | +21 | 3.10x |
| CX-235 | complexity\|internal/control/slash.go\|*Controller.expertNotice | 仍存在（精确匹配） | P3 | `internal/control/slash.go`:517 · `*Controller.expertNotice` | 12 | 12 | 10 | +2 | 1.20x |
| CX-236 | complexity\|internal/control/slash.go\|*Controller.memoryListText | 仍存在（精确匹配） | P3 | `internal/control/slash.go`:739 · `*Controller.memoryListText` | 14 | 14 | 10 | +4 | 1.40x |
| CX-237 | complexity\|internal/control/turn_orchestrator.go\|*turnOrchestrator.runTurnWithRawDisplay | 仍存在（精确匹配） | P2 | `internal/control/turn_orchestrator.go`:22 · `*turnOrchestrator.runTurnWithRawDisplay` | 16 | 16 | 10 | +6 | 1.60x |
| CX-238 | complexity\|internal/controlplane/arbitration/conflict_resolver.go\|resolveConflicts | 仍存在（精确匹配） | P2 | `internal/controlplane/arbitration/conflict_resolver.go`:10 · `resolveConflicts` | 17 | 17 | 10 | +7 | 1.70x |
| CX-239 | complexity\|internal/controlplane/consensus/consensus.go\|Aggregate | 仍存在（精确匹配） | P3 | `internal/controlplane/consensus/consensus.go`:24 · `Aggregate` | 12 | 12 | 10 | +2 | 1.20x |
| CX-240 | complexity\|internal/controlplane/policy_nodes/weighting.go\|LearnControlGraph | 仍存在（精确匹配） | P3 | `internal/controlplane/policy_nodes/weighting.go`:36 · `LearnControlGraph` | 14 | 14 | 10 | +4 | 1.40x |
| CX-241 | complexity\|internal/controlplane/policy_nodes/weighting.go\|dynamicNodeWeight | 仍存在（精确匹配） | P2 | `internal/controlplane/policy_nodes/weighting.go`:53 · `dynamicNodeWeight` | 19 | 19 | 10 | +9 | 1.90x |
| CX-242 | complexity\|internal/diff/diff.go\|myers | 仍存在（精确匹配） | P3 | `internal/diff/diff.go`:175 · `myers` | 13 | 13 | 10 | +3 | 1.30x |
| CX-243 | complexity\|internal/diff/diff.go\|backtrack | 仍存在（精确匹配） | P3 | `internal/diff/diff.go`:215 · `backtrack` | 11 | 11 | 10 | +1 | 1.10x |
| CX-244 | complexity\|internal/diff/diff.go\|unified | 仍存在（精确匹配） | P3 | `internal/diff/diff.go`:266 · `unified` | 11 | 11 | 10 | +1 | 1.10x |
| CX-245 | complexity\|internal/doctor/report.go\|Collect | 仍存在（精确匹配） | P3 | `internal/doctor/report.go`:101 · `Collect` | 12 | 12 | 10 | +2 | 1.20x |
| CX-246 | complexity\|internal/doctor/report.go\|RenderText | 仍存在（精确匹配） | P3 | `internal/doctor/report.go`:192 · `RenderText` | 11 | 11 | 10 | +1 | 1.10x |
| CX-247 | complexity\|internal/equilibrium/convergence/global_equilibrium.go\|applyPolicyWeights | 仍存在（精确匹配） | P3 | `internal/equilibrium/convergence/global_equilibrium.go`:79 · `applyPolicyWeights` | 13 | 13 | 10 | +3 | 1.30x |
| CX-248 | complexity\|internal/eventwire/wire.go\|ToWire | 仍存在（精确匹配） | P1 | `internal/eventwire/wire.go`:32 · `ToWire` | 21 | 21 | 10 | +11 | 2.10x |
| CX-249 | complexity\|internal/eventwire/wire.go\|ToWireDecisionAudit | 仍存在（精确匹配） | P3 | `internal/eventwire/wire.go`:335 · `ToWireDecisionAudit` | 12 | 12 | 10 | +2 | 1.20x |
| CX-250 | complexity\|internal/evidence/evidence.go\|*Ledger.TouchedPaths | 仍存在（精确匹配） | P3 | `internal/evidence/evidence.go`:146 · `*Ledger.TouchedPaths` | 14 | 14 | 10 | +4 | 1.40x |
| CX-251 | complexity\|internal/evidence/evidence.go\|*Ledger.UnverifiedCompletedTodos | 仍存在（精确匹配） | P3 | `internal/evidence/evidence.go`:416 · `*Ledger.UnverifiedCompletedTodos` | 11 | 11 | 10 | +1 | 1.10x |
| CX-252 | complexity\|internal/evidence/evidence.go\|hasFailedCompleteStepRecoveryForTodo | 仍存在（精确匹配） | P2 | `internal/evidence/evidence.go`:467 · `hasFailedCompleteStepRecoveryForTodo` | 15 | 15 | 10 | +5 | 1.50x |
| CX-253 | complexity\|internal/evidence/evidence.go\|PathsProvenInSession | 仍存在（精确匹配） | P3 | `internal/evidence/evidence.go`:568 · `PathsProvenInSession` | 12 | 12 | 10 | +2 | 1.20x |
| CX-254 | complexity\|internal/evidence/evidence.go\|completeStepHasProof | 仍存在（精确匹配） | P3 | `internal/evidence/evidence.go`:718 · `completeStepHasProof` | 13 | 13 | 10 | +3 | 1.30x |
| CX-255 | complexity\|internal/evidence/evidence.go\|hasSuccessfulCompleteStepForTodo | 仍存在（精确匹配） | P3 | `internal/evidence/evidence.go`:796 · `hasSuccessfulCompleteStepForTodo` | 13 | 13 | 10 | +3 | 1.30x |
| CX-256 | complexity\|internal/evidence/evidence.go\|matchTodoStep | 仍存在（精确匹配） | P3 | `internal/evidence/evidence.go`:848 · `matchTodoStep` | 13 | 13 | 10 | +3 | 1.30x |
| CX-257 | complexity\|internal/expert/definition_catalog.go\|definitionSystemPrompt | 仍存在（精确匹配） | P3 | `internal/expert/definition_catalog.go`:75 · `definitionSystemPrompt` | 13 | 13 | 10 | +3 | 1.30x |
| CX-258 | complexity\|internal/expert/store.go\|migrateFirstPrompt | 仍存在（精确匹配） | P3 | `internal/expert/store.go`:247 · `migrateFirstPrompt` | 12 | 12 | 10 | +2 | 1.20x |
| CX-259 | complexity\|internal/expert/store.go\|listDefinitions | 仍存在（精确匹配） | P3 | `internal/expert/store.go`:518 · `listDefinitions` | 12 | 12 | 10 | +2 | 1.20x |
| CX-260 | complexity\|internal/expert/store.go\|filterDefinitions | 仍存在（精确匹配） | P3 | `internal/expert/store.go`:655 · `filterDefinitions` | 13 | 13 | 10 | +3 | 1.30x |
| CX-261 | complexity\|internal/fileref/search.go\|Search | 仍存在（精确匹配） | P0 | `internal/fileref/search.go`:53 · `Search` | 30 | 30 | 10 | +20 | 3.00x |
| CX-262 | complexity\|internal/fileutil/encoding/encoding.go\|Detect | 仍存在（精确匹配） | P3 | `internal/fileutil/encoding/encoding.go`:45 · `Detect` | 14 | 14 | 10 | +4 | 1.40x |
| CX-263 | complexity\|internal/fileutil/encoding/encoding.go\|DetectQuick | 仍存在（精确匹配） | P3 | `internal/fileutil/encoding/encoding.go`:77 · `DetectQuick` | 11 | 11 | 10 | +1 | 1.10x |
| CX-264 | complexity\|internal/guardian/guardian.go\|*Session.Review | 仍存在（精确匹配） | P3 | `internal/guardian/guardian.go`:114 · `*Session.Review` | 12 | 12 | 10 | +2 | 1.20x |
| CX-265 | complexity\|internal/guardian/transcript.go\|ExtractTranscript | 仍存在（精确匹配） | P3 | `internal/guardian/transcript.go`:34 · `ExtractTranscript` | 12 | 12 | 10 | +2 | 1.20x |
| CX-266 | complexity\|internal/guardian/transcript.go\|renderTranscript | 仍存在（精确匹配） | P1 | `internal/guardian/transcript.go`:77 · `renderTranscript` | 29 | 29 | 10 | +19 | 2.90x |
| CX-267 | complexity\|internal/history/search.go\|*Searcher.Search | 仍存在（精确匹配） | P2 | `internal/history/search.go`:129 · `*Searcher.Search` | 19 | 19 | 10 | +9 | 1.90x |
| CX-268 | complexity\|internal/history/search.go\|*Searcher.Around | 仍存在（精确匹配） | P3 | `internal/history/search.go`:222 · `*Searcher.Around` | 11 | 11 | 10 | +1 | 1.10x |
| CX-269 | complexity\|internal/history/search.go\|listJSONL | 仍存在（精确匹配） | P3 | `internal/history/search.go`:336 · `listJSONL` | 11 | 11 | 10 | +1 | 1.10x |
| CX-270 | complexity\|internal/history/search.go\|extractDocuments | 仍存在（精确匹配） | P2 | `internal/history/search.go`:381 · `extractDocuments` | 18 | 18 | 10 | +8 | 1.80x |
| CX-271 | complexity\|internal/i18n/i18n.go\|normalize | 仍存在（精确匹配） | P3 | `internal/i18n/i18n.go`:479 · `normalize` | 11 | 11 | 10 | +1 | 1.10x |
| CX-272 | complexity\|internal/installsource/apply.go\|*installSourceTool.applySkillRoot | 仍存在（精确匹配） | P3 | `internal/installsource/apply.go`:51 · `*installSourceTool.applySkillRoot` | 11 | 11 | 10 | +1 | 1.10x |
| CX-273 | complexity\|internal/installsource/apply.go\|*installSourceTool.applyInstallMCP | 仍存在（精确匹配） | P2 | `internal/installsource/apply.go`:195 · `*installSourceTool.applyInstallMCP` | 18 | 18 | 10 | +8 | 1.80x |
| CX-274 | complexity\|internal/installsource/install_source.go\|*installSourceTool.Execute | 仍存在（精确匹配） | P2 | `internal/installsource/install_source.go`:152 · `*installSourceTool.Execute` | 19 | 19 | 10 | +9 | 1.90x |
| CX-275 | complexity\|internal/installsource/mcp.go\|parseMCPJSON | 仍存在（精确匹配） | P2 | `internal/installsource/mcp.go`:165 · `parseMCPJSON` | 17 | 17 | 10 | +7 | 1.70x |
| CX-276 | complexity\|internal/installsource/mcp.go\|validateMCPEntry | 仍存在（精确匹配） | P3 | `internal/installsource/mcp.go`:243 · `validateMCPEntry` | 11 | 11 | 10 | +1 | 1.10x |
| CX-277 | complexity\|internal/installsource/names.go\|mcpNameFromURL | 仍存在（精确匹配） | P3 | `internal/installsource/names.go`:119 · `mcpNameFromURL` | 11 | 11 | 10 | +1 | 1.10x |
| CX-278 | complexity\|internal/installsource/names.go\|sanitizeName | 仍存在（精确匹配） | P2 | `internal/installsource/names.go`:158 · `sanitizeName` | 16 | 16 | 10 | +6 | 1.60x |
| CX-279 | complexity\|internal/installsource/plan.go\|*installSourceTool.planURL | 仍存在（精确匹配） | P3 | `internal/installsource/plan.go`:36 · `*installSourceTool.planURL` | 14 | 14 | 10 | +4 | 1.40x |
| CX-280 | complexity\|internal/installsource/plan.go\|*installSourceTool.planDownloadedURL | 仍存在（精确匹配） | P3 | `internal/installsource/plan.go`:62 · `*installSourceTool.planDownloadedURL` | 11 | 11 | 10 | +1 | 1.10x |
| CX-281 | complexity\|internal/installsource/plan.go\|*installSourceTool.tryGitHubRepo | 仍存在（精确匹配） | P2 | `internal/installsource/plan.go`:91 · `*installSourceTool.tryGitHubRepo` | 16 | 16 | 10 | +6 | 1.60x |
| CX-282 | complexity\|internal/installsource/plan.go\|*installSourceTool.scanGitHubSkills | 仍存在（精确匹配） | P3 | `internal/installsource/plan.go`:181 · `*installSourceTool.scanGitHubSkills` | 11 | 11 | 10 | +1 | 1.10x |
| CX-283 | complexity\|internal/installsource/plan.go\|*installSourceTool.planLocal | 仍存在（精确匹配） | P1 | `internal/installsource/plan.go`:299 · `*installSourceTool.planLocal` | 20 | 20 | 10 | +10 | 2.00x |
| CX-284 | complexity\|internal/installsource/plan.go\|*installSourceTool.localSkillActions | 仍存在（精确匹配） | P1 | `internal/installsource/plan.go`:344 · `*installSourceTool.localSkillActions` | 23 | 23 | 10 | +13 | 2.30x |
| CX-285 | complexity\|internal/installsource/skill.go\|scanSkillRoot | 仍存在（精确匹配） | P2 | `internal/installsource/skill.go`:216 · `scanSkillRoot` | 17 | 17 | 10 | +7 | 1.70x |
| CX-286 | complexity\|internal/installsource/skill.go\|copyDir | 仍存在（精确匹配） | P3 | `internal/installsource/skill.go`:301 · `copyDir` | 11 | 11 | 10 | +1 | 1.10x |
| CX-287 | complexity\|internal/jobs/jobs.go\|*Manager.StartForSession | 仍存在（精确匹配） | P2 | `internal/jobs/jobs.go`:242 · `*Manager.StartForSession` | 18 | 18 | 10 | +8 | 1.80x |
| CX-288 | complexity\|internal/jobs/jobs.go\|*Manager.SetActiveSessionPath | 仍存在（精确匹配） | P3 | `internal/jobs/jobs.go`:875 · `*Manager.SetActiveSessionPath` | 13 | 13 | 10 | +3 | 1.30x |
| CX-289 | complexity\|internal/jobs/jobs.go\|*Manager.adoptUnscopedJobsLocked | 仍存在（精确匹配） | P3 | `internal/jobs/jobs.go`:929 · `*Manager.adoptUnscopedJobsLocked` | 11 | 11 | 10 | +1 | 1.10x |
| CX-290 | complexity\|internal/jobs/jobs.go\|*Manager.loadSessionArtifacts | 仍存在（精确匹配） | P3 | `internal/jobs/jobs.go`:1142 · `*Manager.loadSessionArtifacts` | 12 | 12 | 10 | +2 | 1.20x |
| CX-291 | complexity\|internal/jobs/jobs.go\|waitTeardownTargets | 仍存在（精确匹配） | P3 | `internal/jobs/jobs.go`:1341 · `waitTeardownTargets` | 12 | 12 | 10 | +2 | 1.20x |
| CX-292 | complexity\|internal/lsp/jsonrpc.go\|*conn.readLoop | 仍存在（精确匹配） | P3 | `internal/lsp/jsonrpc.go`:128 · `*conn.readLoop` | 11 | 11 | 10 | +1 | 1.10x |
| CX-293 | complexity\|internal/lsp/results.go\|parseLocations | 仍存在（精确匹配） | P3 | `internal/lsp/results.go`:11 · `parseLocations` | 12 | 12 | 10 | +2 | 1.20x |
| CX-294 | complexity\|internal/mcpdiag/auth.go\|HasAuthConfig | 仍存在（精确匹配） | P3 | `internal/mcpdiag/auth.go`:55 · `HasAuthConfig` | 11 | 11 | 10 | +1 | 1.10x |
| CX-295 | complexity\|internal/memory/recall.go\|searchMemories | 仍存在（精确匹配） | P3 | `internal/memory/recall.go`:106 · `searchMemories` | 12 | 12 | 10 | +2 | 1.20x |
| CX-296 | complexity\|internal/memory/store.go\|Store.Index | 仍存在（精确匹配） | P3 | `internal/memory/store.go`:147 · `Store.Index` | 13 | 13 | 10 | +3 | 1.30x |
| CX-297 | complexity\|internal/memory/store.go\|Store.Save | 仍存在（精确匹配） | P3 | `internal/memory/store.go`:229 · `Store.Save` | 11 | 11 | 10 | +1 | 1.10x |
| CX-298 | complexity\|internal/memory/store.go\|Store.Archive | 仍存在（精确匹配） | P3 | `internal/memory/store.go`:271 · `Store.Archive` | 12 | 12 | 10 | +2 | 1.20x |
| CX-299 | complexity\|internal/memory/store.go\|Store.List | 仍存在（精确匹配） | P3 | `internal/memory/store.go`:564 · `Store.List` | 13 | 13 | 10 | +3 | 1.30x |
| CX-300 | complexity\|internal/memory/store.go\|Store.ListArchived | 仍存在（精确匹配） | P2 | `internal/memory/store.go`:601 · `Store.ListArchived` | 15 | 15 | 10 | +5 | 1.50x |
| CX-301 | complexity\|internal/memorycompiler/compression.go\|longTailCausalCandidates | 仍存在（精确匹配） | P3 | `internal/memorycompiler/compression.go`:364 · `longTailCausalCandidates` | 12 | 12 | 10 | +2 | 1.20x |
| CX-302 | complexity\|internal/memorycompiler/compression.go\|compressMemoryGraph | 仍存在（精确匹配） | P2 | `internal/memorycompiler/compression.go`:456 · `compressMemoryGraph` | 17 | 17 | 10 | +7 | 1.70x |
| CX-303 | complexity\|internal/memorycompiler/compression.go\|activeSemanticLayers | 仍存在（精确匹配） | P1 | `internal/memorycompiler/compression.go`:1365 · `activeSemanticLayers` | 25 | 25 | 10 | +15 | 2.50x |
| CX-304 | complexity\|internal/memorycompiler/compression.go\|semanticOverlapSignals | 仍存在（精确匹配） | P2 | `internal/memorycompiler/compression.go`:1400 · `semanticOverlapSignals` | 17 | 17 | 10 | +7 | 1.70x |
| CX-305 | complexity\|internal/memorycompiler/runtime.go\|buildIRWithPolicy | 仍存在（精确匹配） | P0 | `internal/memorycompiler/runtime.go`:544 · `buildIRWithPolicy` | 32 | 32 | 10 | +22 | 3.20x |
| CX-306 | complexity\|internal/memorycompiler/runtime.go\|memoryCitationsForIR | 仍存在（精确匹配） | P3 | `internal/memorycompiler/runtime.go`:726 · `memoryCitationsForIR` | 11 | 11 | 10 | +1 | 1.10x |
| CX-307 | complexity\|internal/memorycompiler/runtime.go\|canonicalizeIR | 仍存在（精确匹配） | P3 | `internal/memorycompiler/runtime.go`:789 · `canonicalizeIR` | 12 | 12 | 10 | +2 | 1.20x |
| CX-308 | complexity\|internal/memorycompiler/runtime.go\|usableSubgraphNodes | 仍存在（精确匹配） | P2 | `internal/memorycompiler/runtime.go`:1039 · `usableSubgraphNodes` | 17 | 17 | 10 | +7 | 1.70x |
| CX-309 | complexity\|internal/memorycompiler/runtime.go\|selectStrategy | 仍存在（精确匹配） | P3 | `internal/memorycompiler/runtime.go`:1181 · `selectStrategy` | 11 | 11 | 10 | +1 | 1.10x |
| CX-310 | complexity\|internal/memorycompiler/runtime.go\|controlPlaneSystemState | 仍存在（精确匹配） | P3 | `internal/memorycompiler/runtime.go`:1354 · `controlPlaneSystemState` | 13 | 13 | 10 | +3 | 1.30x |
| CX-311 | complexity\|internal/memorycompiler/runtime.go\|validateIRExecution | 仍存在（精确匹配） | P3 | `internal/memorycompiler/runtime.go`:1636 · `validateIRExecution` | 12 | 12 | 10 | +2 | 1.20x |
| CX-312 | complexity\|internal/memorycompiler/runtime.go\|analyzeTrace | 仍存在（精确匹配） | P2 | `internal/memorycompiler/runtime.go`:1960 · `analyzeTrace` | 16 | 16 | 10 | +6 | 1.60x |
| CX-313 | complexity\|internal/memorycompiler/runtime.go\|evaluateMutations | 仍存在（精确匹配） | P3 | `internal/memorycompiler/runtime.go`:2057 · `evaluateMutations` | 12 | 12 | 10 | +2 | 1.20x |
| CX-314 | complexity\|internal/memorycompiler/runtime.go\|mutationFeedbackInCooldown | 仍存在（精确匹配） | P3 | `internal/memorycompiler/runtime.go`:2197 · `mutationFeedbackInCooldown` | 12 | 12 | 10 | +2 | 1.20x |
| CX-315 | complexity\|internal/memorycompiler/runtime.go\|classifyStrategy | 仍存在（精确匹配） | P3 | `internal/memorycompiler/runtime.go`:2796 · `classifyStrategy` | 12 | 12 | 10 | +2 | 1.20x |
| CX-316 | complexity\|internal/migration/migration.go\|Result.Summary | 仍存在（精确匹配） | P3 | `internal/migration/migration.go`:41 · `Result.Summary` | 11 | 11 | 10 | +1 | 1.10x |
| CX-317 | complexity\|internal/migration/migration.go\|migrateLegacyMemorySources | 仍存在（精确匹配） | P2 | `internal/migration/migration.go`:132 · `migrateLegacyMemorySources` | 15 | 15 | 10 | +5 | 1.50x |
| CX-318 | complexity\|internal/migration/migration.go\|migrateLegacySessionSources | 仍存在（精确匹配） | P1 | `internal/migration/migration.go`:294 · `migrateLegacySessionSources` | 20 | 20 | 10 | +10 | 2.00x |
| CX-319 | complexity\|internal/netclient/netclient.go\|customProxyURL | 仍存在（精确匹配） | P3 | `internal/netclient/netclient.go`:228 · `customProxyURL` | 11 | 11 | 10 | +1 | 1.10x |
| CX-320 | complexity\|internal/permission/permission.go\|*Gate.Check | 仍存在（精确匹配） | P3 | `internal/permission/permission.go`:369 · `*Gate.Check` | 13 | 13 | 10 | +3 | 1.30x |
| CX-321 | complexity\|internal/planmode/policy.go\|Policy.Decide | 仍存在（精确匹配） | P3 | `internal/planmode/policy.go`:151 · `Policy.Decide` | 12 | 12 | 10 | +2 | 1.20x |
| CX-322 | complexity\|internal/planmode/policy.go\|decideBash | 仍存在（精确匹配） | P3 | `internal/planmode/policy.go`:292 · `decideBash` | 11 | 11 | 10 | +1 | 1.10x |
| CX-323 | complexity\|internal/planmode/policy.go\|unsafeSafeCommandArg | 仍存在（精确匹配） | P1 | `internal/planmode/policy.go`:365 · `unsafeSafeCommandArg` | 24 | 24 | 10 | +14 | 2.40x |
| CX-324 | complexity\|internal/planmode/policy.go\|shellFields | 仍存在（精确匹配） | P2 | `internal/planmode/policy.go`:410 · `shellFields` | 16 | 16 | 10 | +6 | 1.60x |
| CX-325 | complexity\|internal/plugin/lazy.go\|*lazyTool.Execute | 仍存在（精确匹配） | P2 | `internal/plugin/lazy.go`:203 · `*lazyTool.Execute` | 19 | 19 | 10 | +9 | 1.90x |
| CX-326 | complexity\|internal/plugin/plugin.go\|Start | 仍存在（精确匹配） | P2 | `internal/plugin/plugin.go`:241 · `Start` | 17 | 17 | 10 | +7 | 1.70x |
| CX-327 | complexity\|internal/plugin/transport_http.go\|*httpTransport.readSSEResponse | 仍存在（精确匹配） | P3 | `internal/plugin/transport_http.go`:170 · `*httpTransport.readSSEResponse` | 14 | 14 | 10 | +4 | 1.40x |
| CX-328 | complexity\|internal/plugin/transport_stdio.go\|newStdioTransport | 仍存在（精确匹配） | P3 | `internal/plugin/transport_stdio.go`:57 · `newStdioTransport` | 13 | 13 | 10 | +3 | 1.30x |
| CX-329 | complexity\|internal/profile/decay.go\|EffectiveConfidence | 仍存在（精确匹配） | P3 | `internal/profile/decay.go`:15 · `EffectiveConfidence` | 11 | 11 | 10 | +1 | 1.10x |
| CX-330 | complexity\|internal/profile/event.go\|BuildLearningEvent | 仍存在（精确匹配） | P2 | `internal/profile/event.go`:123 · `BuildLearningEvent` | 17 | 17 | 10 | +7 | 1.70x |
| CX-331 | complexity\|internal/profile/event.go\|ProfileLearningEvent.Normalize | 仍存在（精确匹配） | P3 | `internal/profile/event.go`:192 · `ProfileLearningEvent.Normalize` | 13 | 13 | 10 | +3 | 1.30x |
| CX-332 | complexity\|internal/profile/extractor.go\|RuleExtractor.Extract | 仍存在（精确匹配） | P3 | `internal/profile/extractor.go`:37 · `RuleExtractor.Extract` | 12 | 12 | 10 | +2 | 1.20x |
| CX-333 | complexity\|internal/profile/model.go\|ProfileItem.Normalize | 仍存在（精确匹配） | P3 | `internal/profile/model.go`:78 · `ProfileItem.Normalize` | 14 | 14 | 10 | +4 | 1.40x |
| CX-334 | complexity\|internal/profile/policy.go\|Policy.ResolveLoading | 仍存在（精确匹配） | P3 | `internal/profile/policy.go`:58 · `Policy.ResolveLoading` | 11 | 11 | 10 | +1 | 1.10x |
| CX-335 | complexity\|internal/profile/policy.go\|Policy.DecideScope | 仍存在（精确匹配） | P3 | `internal/profile/policy.go`:132 · `Policy.DecideScope` | 12 | 12 | 10 | +2 | 1.20x |
| CX-336 | complexity\|internal/profile/selector.go\|Selector.scoreItem | 仍存在（精确匹配） | P1 | `internal/profile/selector.go`:155 · `Selector.scoreItem` | 27 | 27 | 10 | +17 | 2.70x |
| CX-337 | complexity\|internal/profile/store.go\|Store.MaintainNegative | 仍存在（精确匹配） | P3 | `internal/profile/store.go`:191 · `Store.MaintainNegative` | 13 | 13 | 10 | +3 | 1.30x |
| CX-338 | complexity\|internal/profile/store.go\|Store.ApplySuccessfulOverride | 仍存在（精确匹配） | P2 | `internal/profile/store.go`:228 · `Store.ApplySuccessfulOverride` | 17 | 17 | 10 | +7 | 1.70x |
| CX-339 | complexity\|internal/profile/store.go\|safeArchiveStem | 仍存在（精确匹配） | P3 | `internal/profile/store.go`:474 · `safeArchiveStem` | 11 | 11 | 10 | +1 | 1.10x |
| CX-340 | complexity\|internal/profile/worker.go\|*Worker.process | 仍存在（精确匹配） | P2 | `internal/profile/worker.go`:101 · `*Worker.process` | 15 | 15 | 10 | +5 | 1.50x |
| CX-341 | complexity\|internal/provider/anthropic/anthropic.go\|*client.buildRequest | 仍存在（精确匹配） | P0 | `internal/provider/anthropic/anthropic.go`:191 · `*client.buildRequest` | 30 | 30 | 10 | +20 | 3.00x |
| CX-342 | complexity\|internal/provider/anthropic/anthropic.go\|*client.readStream | 仍存在（精确匹配） | P0 | `internal/provider/anthropic/anthropic.go`:308 · `*client.readStream` | 50 | 50 | 10 | +40 | 5.00x |
| CX-343 | complexity\|internal/provider/openai/openai.go\|New | 仍存在（精确匹配） | P1 | `internal/provider/openai/openai.go`:45 · `New` | 21 | 21 | 10 | +11 | 2.10x |
| CX-344 | complexity\|internal/provider/openai/openai.go\|*client.buildRequest | 仍存在（精确匹配） | P2 | `internal/provider/openai/openai.go`:275 · `*client.buildRequest` | 16 | 16 | 10 | +6 | 1.60x |
| CX-345 | complexity\|internal/provider/provider.go\|closeTruncatedJSON | 仍存在（精确匹配） | P2 | `internal/provider/provider.go`:270 · `closeTruncatedJSON` | 17 | 17 | 10 | +7 | 1.70x |
| CX-346 | complexity\|internal/provider/provider.go\|backfillToolCallNames | 仍存在（精确匹配） | P3 | `internal/provider/provider.go`:387 · `backfillToolCallNames` | 13 | 13 | 10 | +3 | 1.30x |
| CX-347 | complexity\|internal/provider/provider.go\|currencySymbol | 仍存在（精确匹配） | P3 | `internal/provider/provider.go`:505 · `currencySymbol` | 12 | 12 | 10 | +2 | 1.20x |
| CX-348 | complexity\|internal/provider/retry.go\|SendWithRetry | 仍存在（精确匹配） | P2 | `internal/provider/retry.go`:156 · `SendWithRetry` | 15 | 15 | 10 | +5 | 1.50x |
| CX-349 | complexity\|internal/provider/schema_canonicalize.go\|canonicalizeSchemaObject | 仍存在（精确匹配） | P3 | `internal/provider/schema_canonicalize.go`:34 · `canonicalizeSchemaObject` | 11 | 11 | 10 | +1 | 1.10x |
| CX-350 | complexity\|internal/retrieval/bm25.go\|snippetAround | 仍存在（精确匹配） | P3 | `internal/retrieval/bm25.go`:169 · `snippetAround` | 11 | 11 | 10 | +1 | 1.10x |
| CX-351 | complexity\|internal/sandbox/shell.go\|resolveShell | 仍存在（精确匹配） | P1 | `internal/sandbox/shell.go`:59 · `resolveShell` | 28 | 28 | 10 | +18 | 2.80x |
| CX-352 | complexity\|internal/serve/auth.go\|safeRedirectTarget | 仍存在（精确匹配） | P2 | `internal/serve/auth.go`:471 · `safeRedirectTarget` | 18 | 18 | 10 | +8 | 1.80x |
| CX-353 | complexity\|internal/serve/serve.go\|*Server.resume | 仍存在（精确匹配） | P3 | `internal/serve/serve.go`:798 · `*Server.resume` | 11 | 11 | 10 | +1 | 1.10x |
| CX-354 | complexity\|internal/serve/serve.go\|*Server.generateTitle | 仍存在（精确匹配） | P2 | `internal/serve/serve.go`:963 · `*Server.generateTitle` | 17 | 17 | 10 | +7 | 1.70x |
| CX-355 | complexity\|internal/serve/serve.go\|*Server.deleteSession | 仍存在（精确匹配） | P2 | `internal/serve/serve.go`:1052 · `*Server.deleteSession` | 18 | 18 | 10 | +8 | 1.80x |
| CX-356 | complexity\|internal/skill/skill.go\|New | 仍存在（精确匹配） | P3 | `internal/skill/skill.go`:107 · `New` | 11 | 11 | 10 | +1 | 1.10x |
| CX-357 | complexity\|internal/skill/skill.go\|*Store.List | 仍存在（精确匹配） | P3 | `internal/skill/skill.go`:289 · `*Store.List` | 11 | 11 | 10 | +1 | 1.10x |
| CX-358 | complexity\|internal/skill/skill.go\|*Store.readEntry | 仍存在（精确匹配） | P3 | `internal/skill/skill.go`:409 · `*Store.readEntry` | 11 | 11 | 10 | +1 | 1.10x |
| CX-359 | complexity\|internal/skill/tools.go\|*runSkillTool.Execute | 仍存在（精确匹配） | P3 | `internal/skill/tools.go`:76 · `*runSkillTool.Execute` | 11 | 11 | 10 | +1 | 1.10x |
| CX-360 | complexity\|internal/skill/tools.go\|*installSkillTool.Execute | 仍存在（精确匹配） | P3 | `internal/skill/tools.go`:445 · `*installSkillTool.Execute` | 13 | 13 | 10 | +3 | 1.30x |
| CX-361 | complexity\|internal/tasktree/executor.go\|*Orchestrator.ExecuteOne | 仍存在（精确匹配） | P2 | `internal/tasktree/executor.go`:81 · `*Orchestrator.ExecuteOne` | 18 | 18 | 10 | +8 | 1.80x |
| CX-362 | complexity\|internal/tasktree/orchestrator.go\|*Orchestrator.Run | 仍存在（精确匹配） | P3 | `internal/tasktree/orchestrator.go`:171 · `*Orchestrator.Run` | 11 | 11 | 10 | +1 | 1.10x |
| CX-363 | complexity\|internal/tasktree/parallel.go\|*Orchestrator.ExecuteBatch | 仍存在（精确匹配） | P2 | `internal/tasktree/parallel.go`:70 · `*Orchestrator.ExecuteBatch` | 18 | 18 | 10 | +8 | 1.80x |
| CX-364 | complexity\|internal/tasktree/planner.go\|*LLMPlanner.PlanTaskTree | 仍存在（精确匹配） | P2 | `internal/tasktree/planner.go`:62 · `*LLMPlanner.PlanTaskTree` | 16 | 16 | 10 | +6 | 1.60x |
| CX-365 | complexity\|internal/tasktree/planner.go\|NormalizeAndValidateGeneratedTree | 仍存在（精确匹配） | P3 | `internal/tasktree/planner.go`:160 · `NormalizeAndValidateGeneratedTree` | 13 | 13 | 10 | +3 | 1.30x |
| CX-366 | complexity\|internal/tasktree/planner.go\|validateGeneratedNodes | 仍存在（精确匹配） | P0 | `internal/tasktree/planner.go`:205 · `validateGeneratedNodes` | 33 | 33 | 10 | +23 | 3.30x |
| CX-367 | complexity\|internal/tasktree/scheduler.go\|*TaskTree.OrderedNodeIDs | 仍存在（精确匹配） | P3 | `internal/tasktree/scheduler.go`:9 · `*TaskTree.OrderedNodeIDs` | 12 | 12 | 10 | +2 | 1.20x |
| CX-368 | complexity\|internal/tasktree/scheduler.go\|ReadyBatchNodeIDs | 仍存在（精确匹配） | P3 | `internal/tasktree/scheduler.go`:131 · `ReadyBatchNodeIDs` | 12 | 12 | 10 | +2 | 1.20x |
| CX-369 | complexity\|internal/tasktree/status.go\|CanTransition | 仍存在（精确匹配） | P3 | `internal/tasktree/status.go`:45 · `CanTransition` | 14 | 14 | 10 | +4 | 1.40x |
| CX-370 | complexity\|internal/tool/builtin/bash.go\|bash.Execute | 仍存在（精确匹配） | P3 | `internal/tool/builtin/bash.go`:124 · `bash.Execute` | 14 | 14 | 10 | +4 | 1.40x |
| CX-371 | complexity\|internal/tool/builtin/bash.go\|hasUnquotedBackgroundOperator | 仍存在（精确匹配） | P3 | `internal/tool/builtin/bash.go`:278 · `hasUnquotedBackgroundOperator` | 13 | 13 | 10 | +3 | 1.30x |
| CX-372 | complexity\|internal/tool/builtin/bash.go\|hasShellCommandWord | 仍存在（精确匹配） | P2 | `internal/tool/builtin/bash.go`:321 · `hasShellCommandWord` | 15 | 15 | 10 | +5 | 1.50x |
| CX-373 | complexity\|internal/tool/builtin/bash.go\|readShellWord | 仍存在（精确匹配） | P3 | `internal/tool/builtin/bash.go`:373 · `readShellWord` | 14 | 14 | 10 | +4 | 1.40x |
| CX-374 | complexity\|internal/tool/builtin/bash.go\|isShellAssignment | 仍存在（精确匹配） | P2 | `internal/tool/builtin/bash.go`:438 · `isShellAssignment` | 17 | 17 | 10 | +7 | 1.70x |
| CX-375 | complexity\|internal/tool/builtin/codeindex.go\|codeIndex.Execute | 仍存在（精确匹配） | P3 | `internal/tool/builtin/codeindex.go`:73 · `codeIndex.Execute` | 13 | 13 | 10 | +3 | 1.30x |
| CX-376 | complexity\|internal/tool/builtin/codeindex.go\|codeIndex.collect | 仍存在（精确匹配） | P1 | `internal/tool/builtin/codeindex.go`:112 · `codeIndex.collect` | 20 | 20 | 10 | +10 | 2.00x |
| CX-377 | complexity\|internal/tool/builtin/codeindex.go\|codeIndex.parseGo | 仍存在（精确匹配） | P3 | `internal/tool/builtin/codeindex.go`:191 · `codeIndex.parseGo` | 14 | 14 | 10 | +4 | 1.40x |
| CX-378 | complexity\|internal/tool/builtin/completestep.go\|completeStep.Execute | 仍存在（精确匹配） | P3 | `internal/tool/builtin/completestep.go`:86 · `completeStep.Execute` | 14 | 14 | 10 | +4 | 1.40x |
| CX-379 | complexity\|internal/tool/builtin/completestep.go\|verifyStepEvidence | 仍存在（精确匹配） | P2 | `internal/tool/builtin/completestep.go`:144 · `verifyStepEvidence` | 18 | 18 | 10 | +8 | 1.80x |
| CX-380 | complexity\|internal/tool/builtin/completestep.go\|verifyCommandFromSession | 仍存在（精确匹配） | P3 | `internal/tool/builtin/completestep.go`:281 · `verifyCommandFromSession` | 11 | 11 | 10 | +1 | 1.10x |
| CX-381 | complexity\|internal/tool/builtin/completestep.go\|allCommandHints | 仍存在（精确匹配） | P2 | `internal/tool/builtin/completestep.go`:351 · `allCommandHints` | 17 | 17 | 10 | +7 | 1.70x |
| CX-382 | complexity\|internal/tool/builtin/delete_range.go\|deleteRange.preview | 仍存在（精确匹配） | P2 | `internal/tool/builtin/delete_range.go`:62 · `deleteRange.preview` | 19 | 19 | 10 | +9 | 1.90x |
| CX-383 | complexity\|internal/tool/builtin/delete_symbol.go\|deleteSymbol.findSymbol | 仍存在（精确匹配） | P2 | `internal/tool/builtin/delete_symbol.go`:144 · `deleteSymbol.findSymbol` | 17 | 17 | 10 | +7 | 1.70x |
| CX-384 | complexity\|internal/tool/builtin/delete_symbol.go\|collectSymbols | 仍存在（精确匹配） | P2 | `internal/tool/builtin/delete_symbol.go`:209 · `collectSymbols` | 19 | 19 | 10 | +9 | 1.90x |
| CX-385 | complexity\|internal/tool/builtin/glob.go\|globRecursive | 仍存在（精确匹配） | P2 | `internal/tool/builtin/glob.go`:90 · `globRecursive` | 16 | 16 | 10 | +6 | 1.60x |
| CX-386 | complexity\|internal/tool/builtin/grep.go\|grepTool.Execute | 仍存在（精确匹配） | P1 | `internal/tool/builtin/grep.go`:87 · `grepTool.Execute` | 27 | 27 | 10 | +17 | 2.70x |
| CX-387 | complexity\|internal/tool/builtin/ls.go\|listDir.listRecursive | 仍存在（精确匹配） | P3 | `internal/tool/builtin/ls.go`:80 · `listDir.listRecursive` | 12 | 12 | 10 | +2 | 1.20x |
| CX-388 | complexity\|internal/tool/builtin/movefile.go\|moveFile.Execute | 仍存在（精确匹配） | P1 | `internal/tool/builtin/movefile.go`:39 · `moveFile.Execute` | 20 | 20 | 10 | +10 | 2.00x |
| CX-389 | complexity\|internal/tool/builtin/multiedit.go\|multiEdit.Execute | 仍存在（精确匹配） | P3 | `internal/tool/builtin/multiedit.go`:64 · `multiEdit.Execute` | 13 | 13 | 10 | +3 | 1.30x |
| CX-390 | complexity\|internal/tool/builtin/preview.go\|multiEdit.Preview | 仍存在（精确匹配） | P3 | `internal/tool/builtin/preview.go`:91 · `multiEdit.Preview` | 11 | 11 | 10 | +1 | 1.10x |
| CX-391 | complexity\|internal/tool/builtin/webfetch.go\|ssrfGuardedTransport | 仍存在（精确匹配） | P1 | `internal/tool/builtin/webfetch.go`:58 · `ssrfGuardedTransport` | 24 | 24 | 10 | +14 | 2.40x |
| CX-392 | complexity\|internal/tool/builtin/webfetch.go\|webFetch.Execute | 仍存在（精确匹配） | P3 | `internal/tool/builtin/webfetch.go`:238 · `webFetch.Execute` | 12 | 12 | 10 | +2 | 1.20x |
| CX-393 | complexity\|internal/tool/sessiontool/sessiontool.go\|*readSessionTool.Execute | 仍存在（精确匹配） | P1 | `internal/tool/sessiontool/sessiontool.go`:104 · `*readSessionTool.Execute` | 22 | 22 | 10 | +12 | 2.20x |
| CX-394 | complexity\|internal/userctx/context.go\|ValidateProfileID | 仍存在（精确匹配） | P2 | `internal/userctx/context.go`:72 · `ValidateProfileID` | 16 | 16 | 10 | +6 | 1.60x |

### 9.2 文件有效行数（18 条）

文件中含有效 Go token 的代码行；排除纯注释与分号。

| ID | Baseline key | 状态 | 优先级 | 位置 | Baseline | 当前复测 | 阈值 | 超出 | 倍数 |
|---|---|---|---|---|---|---|---|---|---|
| FL-001 | file_lines\|internal/acp/service.go\| | 仍存在（精确匹配） | P2 | `internal/acp/service.go`:1 | 1390 | 1390 | 800 | +590 | 1.74x |
| FL-002 | file_lines\|internal/agent/agent.go\| | 仍存在（精确匹配） | P1 | `internal/agent/agent.go`:1 | 1632 | 1632 | 800 | +832 | 2.04x |
| FL-003 | file_lines\|internal/automation/business.go\| | 仍存在（精确匹配） | P3 | `internal/automation/business.go`:1 | 1055 | 1055 | 800 | +255 | 1.32x |
| FL-004 | file_lines\|internal/boot/boot.go\| | 仍存在（精确匹配） | P2 | `internal/boot/boot.go`:1 | 1481 | 1481 | 800 | +681 | 1.85x |
| FL-005 | file_lines\|internal/bot/gateway.go\| | 仍存在（精确匹配） | P3 | `internal/bot/gateway.go`:1 | 962 | 962 | 800 | +162 | 1.20x |
| FL-006 | file_lines\|internal/cli/chat_tui.go\| | 仍存在（精确匹配） | P0 | `internal/cli/chat_tui.go`:1 | 2809 | 2809 | 800 | +2009 | 3.51x |
| FL-007 | file_lines\|internal/cli/cli.go\| | 仍存在（精确匹配） | P1 | `internal/cli/cli.go`:1 | 1638 | 1638 | 800 | +838 | 2.05x |
| FL-008 | file_lines\|internal/config/config.go\| | 仍存在（精确匹配） | P3 | `internal/config/config.go`:1 | 1027 | 1027 | 800 | +227 | 1.28x |
| FL-009 | file_lines\|internal/config/edit.go\| | 仍存在（精确匹配） | P3 | `internal/config/edit.go`:1 | 1018 | 1018 | 800 | +218 | 1.27x |
| FL-010 | file_lines\|internal/config/load.go\| | 仍存在（精确匹配） | P3 | `internal/config/load.go`:1 | 1005 | 1005 | 800 | +205 | 1.26x |
| FL-011 | file_lines\|internal/config/render.go\| | 仍存在（精确匹配） | P3 | `internal/config/render.go`:1 | 1151 | 1151 | 800 | +351 | 1.44x |
| FL-012 | file_lines\|internal/control/controller.go\| | 仍存在（精确匹配） | P0 | `internal/control/controller.go`:1 | 3081 | 3081 | 800 | +2281 | 3.85x |
| FL-013 | file_lines\|internal/evidence/evidence.go\| | 仍存在（精确匹配） | P3 | `internal/evidence/evidence.go`:1 | 815 | 815 | 800 | +15 | 1.02x |
| FL-014 | file_lines\|internal/jobs/jobs.go\| | 仍存在（精确匹配） | P2 | `internal/jobs/jobs.go`:1 | 1275 | 1275 | 800 | +475 | 1.59x |
| FL-015 | file_lines\|internal/memorycompiler/compression.go\| | 仍存在（精确匹配） | P2 | `internal/memorycompiler/compression.go`:1 | 1545 | 1545 | 800 | +745 | 1.93x |
| FL-016 | file_lines\|internal/memorycompiler/runtime.go\| | 仍存在（精确匹配） | P0 | `internal/memorycompiler/runtime.go`:1 | 2677 | 2677 | 800 | +1877 | 3.35x |
| FL-017 | file_lines\|internal/plugin/plugin.go\| | 仍存在（精确匹配） | P3 | `internal/plugin/plugin.go`:1 | 859 | 859 | 800 | +59 | 1.07x |
| FL-018 | file_lines\|internal/serve/serve.go\| | 仍存在（精确匹配） | P2 | `internal/serve/serve.go`:1 | 1245 | 1245 | 800 | +445 | 1.56x |

### 9.3 函数有效行数（51 条）

函数体内含有效 Go token 的代码行；不含函数体外声明。

| ID | Baseline key | 状态 | 优先级 | 位置 | Baseline | 当前复测 | 阈值 | 超出 | 倍数 |
|---|---|---|---|---|---|---|---|---|---|
| FN-001 | func_lines\|internal/acp/service.go\|*service.openExistingSession | 仍存在（精确匹配） | P3 | `internal/acp/service.go`:398 · `*service.openExistingSession` | 107 | 107 | 80 | +27 | 1.34x |
| FN-002 | func_lines\|internal/agent/agent.go\|*Agent.Run | 仍存在（精确匹配） | P1 | `internal/agent/agent.go`:675 · `*Agent.Run` | 172 | 172 | 80 | +92 | 2.15x |
| FN-003 | func_lines\|internal/agent/agent.go\|*Agent.stream | 仍存在（精确匹配） | P3 | `internal/agent/agent.go`:1363 · `*Agent.stream` | 92 | 92 | 80 | +12 | 1.15x |
| FN-004 | func_lines\|internal/agent/agent.go\|*Agent.executeBatchWithRegistry | 仍存在（精确匹配） | P3 | `internal/agent/agent.go`:1504 · `*Agent.executeBatchWithRegistry` | 110 | 110 | 80 | +30 | 1.38x |
| FN-005 | func_lines\|internal/agent/agent.go\|*Agent.executeOneWithRegistry | 仍存在（精确匹配） | P2 | `internal/agent/agent.go`:1826 · `*Agent.executeOneWithRegistry` | 135 | 135 | 80 | +55 | 1.69x |
| FN-006 | func_lines\|internal/agent/migrate.go\|migrateLegacySessions | 仍存在（精确匹配） | P2 | `internal/agent/migrate.go`:82 · `migrateLegacySessions` | 146 | 146 | 80 | +66 | 1.82x |
| FN-007 | func_lines\|internal/agent/migrate.go\|migrateSubDirectory | 仍存在（精确匹配） | P3 | `internal/agent/migrate.go`:352 · `migrateSubDirectory` | 84 | 84 | 80 | +4 | 1.05x |
| FN-008 | func_lines\|internal/agent/parallel_tasks.go\|*ParallelTasksTool.Execute | 仍存在（精确匹配） | P1 | `internal/agent/parallel_tasks.go`:111 · `*ParallelTasksTool.Execute` | 229 | 229 | 80 | +149 | 2.86x |
| FN-009 | func_lines\|internal/agent/task.go\|*TaskTool.Execute | 仍存在（精确匹配） | P3 | `internal/agent/task.go`:434 · `*TaskTool.Execute` | 93 | 93 | 80 | +13 | 1.16x |
| FN-010 | func_lines\|internal/automation/migration.go\|MigrateHeartbeatIfNeeded | 仍存在（精确匹配） | P3 | `internal/automation/migration.go`:55 · `MigrateHeartbeatIfNeeded` | 118 | 118 | 80 | +38 | 1.48x |
| FN-011 | func_lines\|internal/boot/boot.go\|Build | 仍存在（精确匹配） | P0 | `internal/boot/boot.go`:152 · `Build` | 924 | 924 | 80 | +844 | 11.55x |
| FN-012 | func_lines\|internal/bot/gateway.go\|*BotGateway.handleSlashCommand | 仍存在（精确匹配） | P2 | `internal/bot/gateway.go`:509 · `*BotGateway.handleSlashCommand` | 124 | 124 | 80 | +44 | 1.55x |
| FN-013 | func_lines\|internal/bot/qq/gateway.go\|*adapter.connectGateway | 仍存在（精确匹配） | P3 | `internal/bot/qq/gateway.go`:214 · `*adapter.connectGateway` | 119 | 119 | 80 | +39 | 1.49x |
| FN-014 | func_lines\|internal/bot/render.go\|*renderSink.Emit | 仍存在（精确匹配） | P3 | `internal/bot/render.go`:65 · `*renderSink.Emit` | 105 | 105 | 80 | +25 | 1.31x |
| FN-015 | func_lines\|internal/cli/acp.go\|*acpFactory.SessionConfigState | 仍存在（精确匹配） | P3 | `internal/cli/acp.go`:96 · `*acpFactory.SessionConfigState` | 97 | 97 | 80 | +17 | 1.21x |
| FN-016 | func_lines\|internal/cli/bot.go\|botStart | 仍存在（精确匹配） | P3 | `internal/cli/bot.go`:47 · `botStart` | 88 | 88 | 80 | +8 | 1.10x |
| FN-017 | func_lines\|internal/cli/bot.go\|botDoctor | 仍存在（精确匹配） | P2 | `internal/cli/bot.go`:164 · `botDoctor` | 133 | 133 | 80 | +53 | 1.66x |
| FN-018 | func_lines\|internal/cli/chat_tui.go\|chatTUI.update | 仍存在（精确匹配） | P0 | `internal/cli/chat_tui.go`:713 · `chatTUI.update` | 532 | 532 | 80 | +452 | 6.65x |
| FN-019 | func_lines\|internal/cli/chat_tui.go\|chatTUI.View | 仍存在（精确匹配） | P1 | `internal/cli/chat_tui.go`:2210 · `chatTUI.View` | 174 | 174 | 80 | +94 | 2.17x |
| FN-020 | func_lines\|internal/cli/chat_tui.go\|*chatTUI.ingestEvent | 仍存在（精确匹配） | P2 | `internal/cli/chat_tui.go`:3004 · `*chatTUI.ingestEvent` | 147 | 147 | 80 | +67 | 1.84x |
| FN-021 | func_lines\|internal/cli/chat_tui.go\|*chatTUI.runSlashCommand | 仍存在（精确匹配） | P1 | `internal/cli/chat_tui.go`:3226 · `*chatTUI.runSlashCommand` | 162 | 162 | 80 | +82 | 2.02x |
| FN-022 | func_lines\|internal/cli/cli.go\|runAgent | 仍存在（精确匹配） | P3 | `internal/cli/cli.go`:387 · `runAgent` | 101 | 101 | 80 | +21 | 1.26x |
| FN-023 | func_lines\|internal/cli/cli.go\|runServe | 仍存在（精确匹配） | P3 | `internal/cli/cli.go`:508 · `runServe` | 110 | 110 | 80 | +30 | 1.38x |
| FN-024 | func_lines\|internal/cli/cli.go\|chatREPL | 仍存在（精确匹配） | P2 | `internal/cli/cli.go`:652 · `chatREPL` | 148 | 148 | 80 | +68 | 1.85x |
| FN-025 | func_lines\|internal/cli/effort.go\|*chatTUI.runEffortCommand | 仍存在（精确匹配） | P3 | `internal/cli/effort.go`:12 · `*chatTUI.runEffortCommand` | 91 | 91 | 80 | +11 | 1.14x |
| FN-026 | func_lines\|internal/cli/mcp_manager.go\|chatTUI.handleMCPManagerKey | 仍存在（精确匹配） | P2 | `internal/cli/mcp_manager.go`:115 · `chatTUI.handleMCPManagerKey` | 130 | 130 | 80 | +50 | 1.63x |
| FN-027 | func_lines\|internal/cli/select.go\|selectOne | 仍存在（精确匹配） | P2 | `internal/cli/select.go`:81 · `selectOne` | 156 | 156 | 80 | +76 | 1.95x |
| FN-028 | func_lines\|internal/cli/select.go\|selectMany | 仍存在（精确匹配） | P1 | `internal/cli/select.go`:267 · `selectMany` | 205 | 205 | 80 | +125 | 2.56x |
| FN-029 | func_lines\|internal/cli/upgrade.go\|upgradeCommand | 仍存在（精确匹配） | P3 | `internal/cli/upgrade.go`:50 · `upgradeCommand` | 92 | 92 | 80 | +12 | 1.15x |
| FN-030 | func_lines\|internal/config/render.go\|RenderTOMLForScope | 仍存在（精确匹配） | P0 | `internal/config/render.go`:32 · `RenderTOMLForScope` | 516 | 516 | 80 | +436 | 6.45x |
| FN-031 | func_lines\|internal/config/render.go\|RenderTOMLProjectDelta | 仍存在（精确匹配） | P0 | `internal/config/render.go`:575 · `RenderTOMLProjectDelta` | 338 | 338 | 80 | +258 | 4.22x |
| FN-032 | func_lines\|internal/control/controller.go\|*Controller.submitCommandOrTurn | 仍存在（精确匹配） | P2 | `internal/control/controller.go`:698 · `*Controller.submitCommandOrTurn` | 127 | 127 | 80 | +47 | 1.59x |
| FN-033 | func_lines\|internal/control/slash.go\|*Controller.managementNotice | 仍存在（精确匹配） | P3 | `internal/control/slash.go`:420 · `*Controller.managementNotice` | 94 | 94 | 80 | +14 | 1.18x |
| FN-034 | func_lines\|internal/doctor/report.go\|Collect | 仍存在（精确匹配） | P3 | `internal/doctor/report.go`:101 · `Collect` | 88 | 88 | 80 | +8 | 1.10x |
| FN-035 | func_lines\|internal/eventwire/wire.go\|ToWire | 仍存在（精确匹配） | P3 | `internal/eventwire/wire.go`:32 · `ToWire` | 87 | 87 | 80 | +7 | 1.09x |
| FN-036 | func_lines\|internal/fileref/search.go\|Search | 仍存在（精确匹配） | P3 | `internal/fileref/search.go`:53 · `Search` | 86 | 86 | 80 | +6 | 1.07x |
| FN-037 | func_lines\|internal/guardian/transcript.go\|renderTranscript | 仍存在（精确匹配） | P3 | `internal/guardian/transcript.go`:77 · `renderTranscript` | 88 | 88 | 80 | +8 | 1.10x |
| FN-038 | func_lines\|internal/history/search.go\|*Searcher.Search | 仍存在（精确匹配） | P3 | `internal/history/search.go`:129 · `*Searcher.Search` | 86 | 86 | 80 | +6 | 1.07x |
| FN-039 | func_lines\|internal/installsource/install_source.go\|*installSourceTool.Execute | 仍存在（精确匹配） | P3 | `internal/installsource/install_source.go`:152 · `*installSourceTool.Execute` | 93 | 93 | 80 | +13 | 1.16x |
| FN-040 | func_lines\|internal/installsource/plan.go\|*installSourceTool.localSkillActions | 仍存在（精确匹配） | P3 | `internal/installsource/plan.go`:344 · `*installSourceTool.localSkillActions` | 86 | 86 | 80 | +6 | 1.07x |
| FN-041 | func_lines\|internal/jobs/jobs.go\|*Manager.StartForSession | 仍存在（精确匹配） | P3 | `internal/jobs/jobs.go`:242 · `*Manager.StartForSession` | 99 | 99 | 80 | +19 | 1.24x |
| FN-042 | func_lines\|internal/memorycompiler/runtime.go\|buildIRWithPolicy | 仍存在（精确匹配） | P3 | `internal/memorycompiler/runtime.go`:544 · `buildIRWithPolicy` | 94 | 94 | 80 | +14 | 1.18x |
| FN-043 | func_lines\|internal/memorycompiler/runtime.go\|updateGraph | 仍存在（精确匹配） | P3 | `internal/memorycompiler/runtime.go`:2440 · `updateGraph` | 99 | 99 | 80 | +19 | 1.24x |
| FN-044 | func_lines\|internal/migration/migration.go\|migrateLegacySessionSources | 仍存在（精确匹配） | P3 | `internal/migration/migration.go`:294 · `migrateLegacySessionSources` | 90 | 90 | 80 | +10 | 1.13x |
| FN-045 | func_lines\|internal/plugin/lazy.go\|*lazyTool.Execute | 仍存在（精确匹配） | P3 | `internal/plugin/lazy.go`:203 · `*lazyTool.Execute` | 93 | 93 | 80 | +13 | 1.16x |
| FN-046 | func_lines\|internal/plugin/plugin.go\|Start | 仍存在（精确匹配） | P3 | `internal/plugin/plugin.go`:241 · `Start` | 100 | 100 | 80 | +20 | 1.25x |
| FN-047 | func_lines\|internal/provider/anthropic/anthropic.go\|*client.buildRequest | 仍存在（精确匹配） | P3 | `internal/provider/anthropic/anthropic.go`:191 · `*client.buildRequest` | 90 | 90 | 80 | +10 | 1.13x |
| FN-048 | func_lines\|internal/provider/anthropic/anthropic.go\|*client.readStream | 仍存在（精确匹配） | P2 | `internal/provider/anthropic/anthropic.go`:308 · `*client.readStream` | 155 | 155 | 80 | +75 | 1.94x |
| FN-049 | func_lines\|internal/tasktree/planner.go\|validateGeneratedNodes | 仍存在（精确匹配） | P3 | `internal/tasktree/planner.go`:205 · `validateGeneratedNodes` | 97 | 97 | 80 | +17 | 1.21x |
| FN-050 | func_lines\|internal/tool/builtin/grep.go\|grepTool.Execute | 仍存在（精确匹配） | P3 | `internal/tool/builtin/grep.go`:87 · `grepTool.Execute` | 118 | 118 | 80 | +38 | 1.48x |
| FN-051 | func_lines\|internal/tool/builtin/webfetch.go\|ssrfGuardedTransport | 仍存在（精确匹配） | P3 | `internal/tool/builtin/webfetch.go`:58 · `ssrfGuardedTransport` | 95 | 95 | 80 | +15 | 1.19x |

### 9.4 嵌套深度（20 条）

if/for/range/switch/type-switch/select 的最大嵌套层级。

| ID | Baseline key | 状态 | 优先级 | 位置 | Baseline | 当前复测 | 阈值 | 超出 | 倍数 |
|---|---|---|---|---|---|---|---|---|---|
| ND-001 | nesting_depth\|cmd/e2ebench/diff.go\|changedLineCoverage | 仍存在（精确匹配） | P3 | `cmd/e2ebench/diff.go`:345 · `changedLineCoverage` | 5 | 5 | 4 | +1 | 1.25x |
| ND-002 | nesting_depth\|cmd/e2ebench/diff.go\|changedLineSet | 仍存在（精确匹配） | P3 | `cmd/e2ebench/diff.go`:418 · `changedLineSet` | 5 | 5 | 4 | +1 | 1.25x |
| ND-003 | nesting_depth\|internal/agent/migrate.go\|migrateLegacySessions | 仍存在（精确匹配） | P3 | `internal/agent/migrate.go`:82 · `migrateLegacySessions` | 5 | 5 | 4 | +1 | 1.25x |
| ND-004 | nesting_depth\|internal/agent/migrate.go\|migrateSubDirectory | 仍存在（精确匹配） | P3 | `internal/agent/migrate.go`:352 · `migrateSubDirectory` | 5 | 5 | 4 | +1 | 1.25x |
| ND-005 | nesting_depth\|internal/boot/boot.go\|Build | 仍存在（精确匹配） | P2 | `internal/boot/boot.go`:152 · `Build` | 7 | 7 | 4 | +3 | 1.75x |
| ND-006 | nesting_depth\|internal/botruntime/runtime.go\|EnabledPlatforms | 仍存在（精确匹配） | P3 | `internal/botruntime/runtime.go`:21 · `EnabledPlatforms` | 5 | 5 | 4 | +1 | 1.25x |
| ND-007 | nesting_depth\|internal/cli/chat_tui.go\|chatTUI.update | 仍存在（精确匹配） | P3 | `internal/cli/chat_tui.go`:713 · `chatTUI.update` | 5 | 5 | 4 | +1 | 1.25x |
| ND-008 | nesting_depth\|internal/cli/chat_tui.go\|*chatTUI.collapseToolOutput | 仍存在（精确匹配） | P3 | `internal/cli/chat_tui.go`:1765 · `*chatTUI.collapseToolOutput` | 5 | 5 | 4 | +1 | 1.25x |
| ND-009 | nesting_depth\|internal/cli/select.go\|selectMany | 仍存在（精确匹配） | P3 | `internal/cli/select.go`:267 · `selectMany` | 5 | 5 | 4 | +1 | 1.25x |
| ND-010 | nesting_depth\|internal/config/load.go\|mergeTOMLProviders | 仍存在（精确匹配） | P3 | `internal/config/load.go`:303 · `mergeTOMLProviders` | 5 | 5 | 4 | +1 | 1.25x |
| ND-011 | nesting_depth\|internal/control/controller.go\|*Controller.submitCommandOrTurn | 仍存在（精确匹配） | P3 | `internal/control/controller.go`:698 · `*Controller.submitCommandOrTurn` | 5 | 5 | 4 | +1 | 1.25x |
| ND-012 | nesting_depth\|internal/history/search.go\|extractDocuments | 仍存在（精确匹配） | P3 | `internal/history/search.go`:381 · `extractDocuments` | 5 | 5 | 4 | +1 | 1.25x |
| ND-013 | nesting_depth\|internal/memory/store.go\|Store.Index | 仍存在（精确匹配） | P3 | `internal/memory/store.go`:147 · `Store.Index` | 5 | 5 | 4 | +1 | 1.25x |
| ND-014 | nesting_depth\|internal/memorycompiler/runtime.go\|mutationsFromLearning | 仍存在（精确匹配） | P3 | `internal/memorycompiler/runtime.go`:2012 · `mutationsFromLearning` | 5 | 5 | 4 | +1 | 1.25x |
| ND-015 | nesting_depth\|internal/plugin/lazy.go\|*lazyTool.Execute | 仍存在（精确匹配） | P3 | `internal/plugin/lazy.go`:203 · `*lazyTool.Execute` | 5 | 5 | 4 | +1 | 1.25x |
| ND-016 | nesting_depth\|internal/provider/anthropic/anthropic.go\|*client.buildRequest | 仍存在（精确匹配） | P3 | `internal/provider/anthropic/anthropic.go`:191 · `*client.buildRequest` | 5 | 5 | 4 | +1 | 1.25x |
| ND-017 | nesting_depth\|internal/provider/anthropic/anthropic.go\|*client.readStream | 仍存在（精确匹配） | P3 | `internal/provider/anthropic/anthropic.go`:308 · `*client.readStream` | 5 | 5 | 4 | +1 | 1.25x |
| ND-018 | nesting_depth\|internal/tool/builtin/codeindex.go\|codeIndex.parseGo | 仍存在（精确匹配） | P3 | `internal/tool/builtin/codeindex.go`:191 · `codeIndex.parseGo` | 5 | 5 | 4 | +1 | 1.25x |
| ND-019 | nesting_depth\|internal/tool/builtin/delete_symbol.go\|collectSymbols | 仍存在（精确匹配） | P3 | `internal/tool/builtin/delete_symbol.go`:209 · `collectSymbols` | 5 | 5 | 4 | +1 | 1.25x |
| ND-020 | nesting_depth\|internal/tool/builtin/webfetch.go\|ssrfGuardedTransport | 仍存在（精确匹配） | P2 | `internal/tool/builtin/webfetch.go`:58 · `ssrfGuardedTransport` | 6 | 6 | 4 | +2 | 1.50x |

### 9.5 单包生产文件数（1 条）

同一目录中非测试、非生成的生产 Go 文件数量。

| ID | Baseline key | 状态 | 优先级 | 位置 | Baseline | 当前复测 | 阈值 | 超出 | 倍数 |
|---|---|---|---|---|---|---|---|---|---|
| PF-001 | package_files\|internal/cli\| | 仍存在（精确匹配） | P2 | `internal/cli`:1 | 56 | 56 | 30 | +26 | 1.87x |

## 10. 指标解释与核实边界

1. 本次“逐项核实”确认的是当前源码仍被同一版 Code Guard 以相同 key、位置、实际值和阈值检出。所有 484 条均完成机械对账与源码定位。
2. 圈复杂度是项目基于 Go AST 的近似度量，会累计 if、for/range、非 default case/select 分支以及 &&/||；它不是认知复杂度，也不能单独证明功能错误或运行性能问题。
3. 函数与文件有效行数反映体量，不自动证明职责混乱。拆分前仍需读取调用链、状态所有权、并发边界和测试覆盖。
4. 嵌套深度反映控制流层级；guard clause、状态机或拆分函数是否合适，需要逐函数评审。
5. package_files 只反映生产 Go 文件数量，不能区分合理的平台/协议边界与无意义碎片。
6. 默认扫描范围仍是 internal、cmd、scripts、tools 下的生产 Go 文件；Desktop、Frontend、测试和生成文件不在这 484 条内。
7. 后续修复应以“行为不变、测试先行、baseline 显式下降、不制造碎片”为约束，不能按数字机械拆分。

## 11. 建议治理批次

1. 第一批：同时命中超长文件、超长函数和高复杂度的 P0/P1 热点，优先 `internal/boot/boot.go`、`internal/cli/chat_tui.go`、`internal/config/render.go`。
2. 第二批：核心运行链路热点，优先 `internal/control/controller.go`、`internal/agent/agent.go`、`internal/memorycompiler/runtime.go`、`internal/cli/cli.go`。
3. 第三批：集中治理同一文件内多个 P2/P3 复杂度条目，避免零散逐函数修改造成回归成本。
4. 第四批：处理单项轻度超限，并同步缩减 baseline；不得为了降行数而制造一函数一文件或纯转发包装。

---

完整性自检：报告明细共 484 条；Baseline 484 条；当前 strict 484 条；精确匹配 484 条。

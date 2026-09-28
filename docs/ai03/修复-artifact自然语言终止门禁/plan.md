# 修复 artifact 自然语言终止门禁

状态：就绪，等待实施批准
基线：`origin/main-v3@cd840ab1c7211104a4763acd10d9eed53eb7f1f7`

## 1. 问题与证据

当前 Root Agent 会从普通用户文本推导 `ArtifactRequirement`，并在模型返回无工具 final candidate 后要求宿主验证所有推导目标。未满足时，候选会被标记为 `readiness_rejected_candidate`，宿主向模型注入重试提示；第三次仍不满足时返回 `final-answer readiness failed 3 times`。

这条链路把不可靠的自然语言推断当作确定性硬合同。用户看不到推导出的目标，也无法在 UI 中理解或修正它，因此与可见 todo、项目检查和后台任务 readiness 不同。

### 1.1 当前基线复现

环境与前置条件：

- Windows，PowerShell，Go 测试运行在独立 worktree。
- 基线为最新 `origin/main-v3@cd840ab1`，worktree 干净。
- 使用生产 `artifactRequirementFromInput` 和真实 `Agent.Run`；同包测试通过 `go test -overlay` 临时注入，诊断文件不进入仓库。
- Agent 配置宿主只读 `verify_artifact`，没有 writer receipt；mock provider 连续返回三个可见纯聊天答案。

输入：

```text
请生成一道简短的图表读数练习，轮换覆盖表格、柱状图、折线图、饼图和同比环比。先给出少量明确的模拟数据和问题，再给出答案与解读要点；明确标注数据为示例，不将其描述为真实业务数据。
```

步骤：

1. 用生产 parser 解析完整输入。
2. 用同一输入执行 `Agent.Run`。
3. 每次 provider 请求返回可见聊天答案，不调用工具或创建文件。
4. 记录 parser 结果、provider 请求次数和终止错误。

期望：聊天练习作为 final answer 一次正常结束。
实际：parser 生成 `Required=true`、一个无扩展名且无路径的匿名 target；三个 final candidate 全部被拒绝。

基线命令：

```powershell
go test -overlay C:\Users\ai03\.codex\visualizations\2026\08\18\01a014e8-27f0-7ce3-b9e9-700e5198c07d\artifact_readiness_plan_overlay.json ./internal/agent -run '^TestArtifactReadinessPlanProbe(Parser|Run)$' -count=1 -v
```

基线关键输出：

```text
requirement={"Required":true,...,"Targets":[{"Extensions":null,"PathSuffixes":null,...}]}
requests=3 err=final-answer readiness failed 3 times: hard requirement: requested workspace artifact has not been verified ... missing targets: artifact file ... candidates ... (none)
```

### 1.2 调用链

```text
Agent.Run input
  -> beginArtifactRequirementRunWithInput / captureArtifactRequirement
  -> artifactRequirementFromInput + artifact_contract_helpers
  -> plan/steer/failure carry state
  -> final candidate
  -> prepareInternalArtifactVerification
  -> artifactReadinessFailure
  -> evidence.FreshSuccessfulArtifactVerificationMatch
  -> final candidate rejected, retry up to three times
```

主要源码：

- `internal/agent/artifact_contract.go`：自然语言 parser、requirement 状态、跨轮协调、artifact readiness。
- `internal/agent/artifact_contract_helpers.go`：target、扩展名、数量、句界、增删改名等解析和 matcher 输入构造。
- `internal/agent/artifact_contract_runtime.go`：final 边界宿主 verifier。
- `internal/agent/agent.go`：捕获/携带 requirement、final 边界验证和三次 readiness 重试。
- `internal/evidence/artifact.go`：推导 target 与 verifier candidate 的 distinct/freshness matcher。
- `internal/control/turn_orchestrator.go`、`internal/control/controller.go`、`internal/agent/coordinator.go`：plan approval 前后的 requirement carry。

### 1.3 历史证据

- PR #321 扩充 parser 的句界、文件名、列表和多目标规则，并明确剩余风险仍在自然语言长尾。
- PR #362 再修工具元语言误判并把 verifier 下沉宿主，但计划把“不取消 artifact readiness”设为非目标。
- PR #383 在子任务链路复现“一个正确文件被推导成多个目标”后，完整撤销自然语言产物完成门禁，改用 writer receipt、终稿标记和已验证路径投影；该 PR 明确认定 prompt/动作词正则不可靠。
- PR #403 给图表模板增加“只在聊天中回复，不要创建或保存文件”，只能止血，不能消除系统误判。

## 2. 当前可保留能力

Artifact 文件展示不依赖 Root Agent 的 `ArtifactRequirement`：

- `desktop/frontend/src/components/Message.tsx` 从 final 文本、结构化 `artifactPaths`、Markdown 链接和文件引用中收集候选。
- 前端通过所属 workspace 的 `ReadFile` 实际读取候选；只有读取成功的路径才显示可打开卡片。
- 缺失文件只保留普通文本，不显示假卡片或可点击坏链接。
- history projection 会恢复结构化专家团 artifact 路径，随后执行同一读取验证。
- PR #383 的子任务 writer receipt、终稿标记和结构化产物投影不依赖自然语言 artifact 完成门禁。

底层 `internal/tool/builtin/verifyartifact.go` 提供 canonical path、workspace confinement、普通文件和存在性检查；该确定性实现及测试保留，但不再接入默认 final 阻断。

## 3. 目标与非目标

### 3.1 目标

1. 普通用户文本不再生成、携带或更新 artifact 完成合同。
2. artifact 缺失不再撤回 final、触发隐藏模型重试或返回 `requested workspace artifact` 错误。
3. 可见 todo、项目检查和后台任务 readiness 保持原行为。
4. 精简模型提示：明确文件必须真实创建并在 final 中链接，不再声称宿主会在 final 边界验证所有返回路径。
5. 保留实际文件卡片、workspace 读取验证、历史恢复和专家团结构化产物投影。
6. 保留底层 `verify_artifact` 的确定性路径安全实现，不把它暴露为默认模型工具。
7. 删除失去消费者的 parser、requirement state、carry、target matcher 和专属测试，避免留下 advisory 或可重新接回的隐式控制链。

### 3.2 非目标

- 不新增结构化 artifact schema、自动化字段、UI 配置、调用方 API 或数据迁移。
- 不让“精确文件名”“保存到文件”等高置信自然语言继续建立硬合同。
- 不修改 todo、project checks、running task、empty final、executor handoff 或其他 final readiness 条件。
- 不改前端文件卡片解析、读取或打开逻辑。
- 不回滚 PR #383 的专家团结构化产物交付，也不回滚 PR #403 的模板提示词。
- 不重写历史 `docs/ai03` 计划和回执；它们是当时决策记录。
- 不把 parser 保留为 advisory、遥测或审计信息。

## 4. 设计

### 4.1 终止语义

普通模式的新终止链：

```text
model final candidate
  -> existing visible readiness checks
       - incomplete todo
       - required project check
       - running persistent task
  -> visible non-empty answer / steer / compaction checks
  -> accept final
```

不再存在 artifact target 推导、host artifact pre-verification 或 artifact-only retry。

明确要求 `report.pdf`、Excel、图片或多个文件仍属于普通自然语言输入；模型应按精简提示执行，但宿主不会从文本建立隐藏硬门禁。若 final 引用真实文件，现有 UI 读取验证后展示卡片；若文件不存在，UI 不展示假卡片。

### 4.2 删除 Root artifact 控制状态

- 删除 `ArtifactRequirement`、`artifactRequirementState` 及 parser/helpers。
- 删除 Run 开始/结束、steer、plan reset 和 plan approval carry 的 artifact 调用。
- 删除 `prepareInternalArtifactVerification`、`artifactReadinessFailure` 和 artifact-specific retry guidance。
- `finalReadinessCheck` 从空的 missing/out 状态开始，再执行现有 todo 和 project-check 逻辑。
- 保留通用三次 readiness 重试机制，因为其他可见、结构化条件仍使用它。

### 4.3 收缩 evidence 和 boot wiring

- 删除只服务 requirement target matching 的 `internal/evidence/artifact.go` 及专属测试，包括 distinct target、freshness 和 artifact mutation matcher。
- 保留通用 receipt writer/read/path 记录、todo/project-check evidence 和专家团 lifecycle artifact evidence。
- 删除 `Agent.Options.ArtifactVerifier`、Agent 字段和 boot/buildAssembly 的默认 verifier 注入。
- 保留 `verify_artifact` builtin 及其单元测试；boot 继续确保它不进入默认或显式模型工具列表，防止重新形成模型自验证回路。
- 删除 `recordToolReceipt` 中仅为模型 `verify_artifact` 结果补路径的分支；该工具仍不在模型 registry 中。

### 4.4 提示与协议

全局提示、子任务提示和专家团定义统一为可观察行为：

- 明确要求本地文件时，真实创建文件。
- final 使用指向实际文件的 Markdown 链接。
- 不把 code block 或“请用户自行保存”当成交付。
- 不声称未发生的创建、验证或交付。

删除以下不再真实的表述：

- 宿主在 final 边界验证每个返回路径。
- expert lead 调用 `verify_artifact` 作为最后 workspace 操作。
- artifact 验证必须排在所有写入、测试和 background wait 之后。

同步更新 `docs/A2A_EXPERT_TEAM_PROTOCOL.md` 的当前协议说明；历史计划不改。

## 5. 接受标准

| ID | 场景 | 预期 |
|---|---|---|
| AC-01 | 原“图表读数练习”提示，provider 返回纯聊天答案 | 一次 provider final 成功；无 `readiness_rejected_candidate`、无 `requested workspace artifact`、无隐藏重试 |
| AC-02 | “生成报告内容”“生成 Markdown 表格”“生成图片说明” | 普通 final 成功，不建立 artifact 状态 |
| AC-03 | “生成 report.pdf”“创建 Excel 文件”“生成两个 PNG”但模型直接 final | final 不因自然语言文件请求被宿主撤回；文件是否实际存在只影响 UI 文件卡片，不影响默认终止 |
| AC-04 | 模型使用 writer 创建文件并在 final 给出 Markdown 链接 | final 正常结束；Desktop 读取成功并显示一张可打开文件卡片；历史恢复后仍可见 |
| AC-05 | final 引用不存在的本地文件 | final 文本可见；不显示假卡片或可点击坏链接 |
| AC-06 | 最新 todo 仍为 pending/in_progress 且有 todo progress receipt | 现有可见 todo readiness 仍阻止 final |
| AC-07 | 写入后缺少 AGENTS.md project check | 现有 project-check readiness 仍阻止 final |
| AC-08 | persistent task 仍运行 | 现有 running-task readiness 仍阻止 final |
| AC-09 | 专家团子任务返回结构化 writer artifact | 根终稿成功时继续投影已验证路径；失败/中断/非终稿不发布；历史恢复保持 |
| AC-10 | `verify_artifact` 直接校验普通文件、目录、越界和缺失路径 | 底层 builtin 现有安全结果不变，且工具不出现在模型 schema |
| AC-11 | plan approval、steer、失败重试 | 不再携带或协调 artifact requirement；其他 plan/steer 行为不变 |
| AC-12 | 自动化无 workspace、无 connector、纯聊天提示 | 正常归档聊天结果，不出现 artifact readiness |

## 6. 影响文件与合同

预计删除：

- `internal/agent/artifact_contract.go`
- `internal/agent/artifact_contract_helpers.go`
- `internal/agent/artifact_contract_runtime.go`
- `internal/agent/artifact_contract_test.go`
- `internal/evidence/artifact.go`

预计修改：

- `internal/agent/agent.go`、`final_readiness_test.go`、`coordinator.go`、`coordinator_test.go`
- `internal/agent/tool_receipts.go`、`expert_team_tools_test.go`、`subagent_artifact_handoff_test.go`、`task.go`
- `internal/control/controller.go`、`turn_orchestrator.go` 及相邻测试
- `internal/evidence/evidence_test.go`
- `internal/boot/boot.go`、`build_state.go`、`build_runner.go`、`build_plugins.go`、`artifact_verifier_tools.go`、`boot_test.go`
- `internal/config/render_agent_header.go`
- `internal/expert/definition_catalog.go` 及测试
- `docs/A2A_EXPERT_TEAM_PROTOCOL.md`

实施时以实际引用搜索和 diff 为准；不因预计列表机械修改无关文件。

## 7. 兼容性与迁移

- 行为兼容性：显式文件自然语言请求不再享受宿主 fail-closed 保证。这是消除隐藏推断所必需且已批准的行为变更。
- 文件体验：真实文件链接、卡片、打开和历史恢复合同保持。
- API：移除的 `ArtifactRequirement`、`ArtifactVerifier` 和 carry 接口都位于 `internal` 包，没有外部公共 API 消费者。
- 数据：artifact requirement 和 verifier ledger 是进程内状态，不持久化；无需 session、自动化或数据库迁移。
- 历史：既有 `readiness_rejected_candidate` 和 final 标记按原记录读取，不重写旧 session。
- 自动化：不改 Task schema、DeliveryPolicy、result.md 归档或 connector 选择。

## 8. 风险、控制与回滚

### 风险

1. 模型可能声称创建文件但实际未创建。控制：精简系统提示禁止虚假声明；Desktop 仅对实际可读文件显示卡片，不把文本声明伪装成交付。
2. 删除 evidence artifact matcher 时误伤通用 writer/freshness。控制：按引用删除，只保留/验证 todo、project checks、writer receipt 和专家团 lifecycle 证据的当前消费者。
3. 专家团提示仍引用不可用 verifier。控制：同步全局、task、expert definition 和 A2A 当前协议，并用 prompt 合同测试搜索残留。
4. 大量删除造成遗漏的 carry 或 boot wiring。控制：全仓 `rg` 断言不再存在 `ArtifactRequirement`、artifact carry、artifact readiness 和 host verifier wiring；运行受影响包测试和 PR gate。
5. 用户仍可能遇到其他 readiness 错误。该风险不在本任务内；todo、project checks 和 running task 必须继续依据真实状态并保持 UI 可见。

### 回滚

本变更无数据迁移。若回归，可整体回滚唯一代码提交，恢复 parser、host verifier wiring 和旧提示；计划与任务回执可保留。不得只恢复 parser 而不恢复其 state、boot verifier 和 evidence matcher，否则会产生半连接控制链。

## 9. 验证

### 9.1 自动化

实施迭代先运行精确回归，最终运行受影响包范围：

```powershell
go test ./internal/agent ./internal/control ./internal/evidence ./internal/boot ./internal/config ./internal/expert ./internal/tool/builtin -count=1
```

预计直接修改六个 core Go 包，超过仓库“多于 4 个 core package”阈值，因此 PR handoff 还必须运行完整 Root Go suite：

```powershell
go test ./... -count=1
```

确认删除边界：

```powershell
rg -n "ArtifactRequirement|artifactRequirement|artifactReadiness|ArtifactVerifier|artifactVerifier|requested workspace artifact" internal
```

允许的残留仅限历史文档；生产 Go 源码和当前提示不得残留。`verify_artifact` builtin、boot 的模型工具排除和底层测试应继续存在。

最终执行仓库要求：

```powershell
go run ./scripts/code_guard
git ls-files --error-unmatch scripts/code_guard_baseline.json
git diff --exit-code origin/main-v3...HEAD -- scripts/code_guard_baseline.json
git diff --exit-code -- scripts/code_guard_baseline.json
git diff --cached --exit-code -- scripts/code_guard_baseline.json
git diff --check origin/main-v3...HEAD
node ./scripts/pr-gate.mjs --base origin/main-v3 --run all
```

按完整 NUL-delimited diff 路径核对 gate classification。预计为 Root Go `full`：虽然不修改 Root `go.mod/go.sum`，但预计直接修改六个 core Go 包，已触发 `go test ./...` 阈值。若实际路径分类产生额外检查，以 gate 结果扩展。

### 9.2 真实应用 A/B

使用相同 dev profile、服务、模型、workspace、初始状态和原始提示，串行运行：

1. A：从未修改的最新 baseline worktree 的 `desktop/` 启动 `wails dev`，运行原图表提示，记录三个 rejected candidates 和最终 readiness error。
2. 正常关闭 A，确认进程、端口和 singleton lock 已释放；恢复同一起始会话/自动化状态。
3. B：从 patched worktree 的 `desktop/` 启动 `wails dev`，运行同一提示，确认一次 final 成功、结果归档、无 artifact readiness。
4. 在 B 中运行真实文件场景：用 writer 创建一个确定内容的文件并在 final 链接，确认卡片显示、可打开、内容正确，历史重载后仍存在。
5. 在 B 中运行缺失文件链接 fixture，确认无假卡片。
6. 完成后保留 patched `wails dev` 进程供用户手工验收。

若 baseline 因外部模型非确定性没有返回纯聊天答案，则使用同一生产 Agent overlay 作为受控 A/B，并把真实应用结果明确标为补充证据，不虚称等价应用 A/B。

## 10. 方案挑战与最终选择

已拒绝方案：

- 仅改模板：PR #403 已止血，但其他用户输入仍会触发同类误判。
- 继续收紧 parser：PR #171/#321/#362 已证明误判类别持续增长，且 parser/helpers 当前超过 1700 行。
- 保留 advisory parser：没有已证明消费者，仍产生维护成本并保留重新接入硬门禁的风险。
- 只对明确路径或格式保留自然语言硬合同：仍然是隐藏语义分类，与“只有结构化调用方合同才能影响状态”的已确认原则冲突。
- 本次新增结构化 artifact 模式：当前 Agent、自动化和 UI 没有真实合同消费者，会引入 schema/API/UI/兼容范围，超出修复目标。
- 整体回滚 PR #95/#107/#383：会破坏已验证的文件卡片、workspace 安全、专家团结构化产物和历史恢复。

最终选择：删除 Root 自然语言 artifact 完成控制链；保留确定性的实际文件验证和展示能力；不新增替代硬合同。

## 11. 任务拆分

该变更只有一个原子行为：移除跨 Agent、Controller、evidence 和 boot 的 artifact 终止合同，同时保持相邻 readiness 与文件展示合同。把 parser、carry、readiness 或提示拆成可独立合并的多个任务会产生不可运行或语义不一致的中间状态，因此使用一个实现任务：

- `tasks/001-移除自然语言产物终止门禁.md`

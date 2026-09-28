# 任务 001：修复产物需求解析与 readiness 误判

## 1. 目标与产物

修复 RF-003：让 artifact requirement parser 对 Office、Coding、Design 和 Profile
Card 原始输入生成正确 targets，使成功且新鲜的 `verify_artifact` receipts 能通过
readiness；同时让真实 target mismatch 可诊断，不放宽任何验证安全条件。

预期产物：

- sentence-aware action clause 边界；
- verb-scoped target 与 `replace/change` 资格判断；
- 由同一 matcher 产生的缺失 target/candidate 诊断；
- parser、ledger/readiness 和 Agent 端到端回归测试；
- 本任务完成回执和验证记录。

## 2. 依赖与证据

- 稳定决策与验收：[`../plan.md`](../plan.md)。
- 基线：`origin/main-v3@8206dd5b03d8e98d614487b1ca6dfe233cdb8385`。
- Codegraph 链路：`Agent.Run` → `artifactRequirementFromInput` →
  `ArtifactRequirement.Targets` → `recordToolReceipt` →
  `HasFreshSuccessfulArtifactVerification` → `artifactReadinessFailure` →
  `finalReadinessCheck`。
- 三个原始 prompt 已在调查中确定性复现错误 contract；真实会话确认最终 verify
  成功且之后无 mutation。

## 3. 允许范围

允许修改：

- `internal/agent/artifact_contract.go`
- `internal/agent/artifact_contract_helpers.go`
- `internal/agent/artifact_contract_test.go`
- `internal/agent/final_readiness_test.go`
- 为保持 matcher 单一事实源所必需的 `internal/evidence/artifact.go` 及测试
- 本目录的 `progress.md` 和任务回执

允许添加只服务上述合同的小型内部 helper 和测试 fixture。

## 4. 禁止范围与保留行为

禁止：

- 修改系统提示词、grader、prompt-eval runner、Desktop 前端或工具 schema；
- 放宽 freshness、distinct candidate、target cardinality 或 background barrier；
- 让 source/forbidden path 满足 output target；
- 使用模型、网络服务或新依赖解析 artifact intent；
- 修改 `scripts/code_guard_baseline.json`；
- 顺便重构 todo、project checks、通用 readiness 或 Agent loop。

必须保留：

- `report.final.pdf` 等多点文件名；
- `following files:` 后的跨行列表；
- 明确的 convert/export/save-as/replace/rename 目标变换；
- additive steer、取消和 carry/reconciliation 行为；
- verify 必须晚于最新可能 mutation；
- 每个 target 必须由 distinct candidate 满足。

## 5. 实施约束

- 先增加失败的 characterization tests，再实现。
- 不得只针对三个字符串加白名单或场景名称分支。
- ASCII 句点边界必须通过词法上下文区分文件名、URL、版本/小数和真正句末。
- `replace/change` 判断必须使用局部动作语法，不得因 clause 任意位置存在 `.py`
  就认定为 artifact delivery。
- 诊断必须由 matcher 的同一次或同一事实源计算得出；不得复制一套近似算法。
- 只做完成测试所需的最小格式化，不产生 CRLF 噪音。

## 6. 验收标准

- 满足 `plan.md` 的 AC-01 至 AC-11。
- 三个原始 prompt 的 target 集合精确相等，而不是仅检查 `Required`。
- 三个 prompt 注入真实 verify paths 后 readiness 为 ready。
- target mismatch 文案能指出缺失 target/candidates；mutation ordering 测试仍返回
  对应顺序/新鲜度语义。
- 所有现有 artifact requirement、final readiness 和 evidence matcher 测试通过。
- 实际生产 Controller/Agent 路径不出现 readiness retry。

## 7. 聚焦验证

迭代期：

```powershell
go test ./internal/agent -run 'RF003|ArtifactRequirement|FinalReadiness' -count=1
go test ./internal/evidence -run 'ArtifactVerification|ArtifactMatch' -count=1
```

提交前：

```powershell
go test ./internal/agent ./internal/evidence ./internal/tool/builtin -count=1
git diff --check
```

实现者只负责聚焦自动化验证。主会话另行执行生产路径应用验收、最新主线同步和最终
PR Gate。

## 8. 分配与状态

| 项目 | 值 |
| --- | --- |
| 实现 Agent | `gpt-5.6-sol`，medium reasoning |
| 执行方式 | 单一实现 Agent、单一 worktree、不得继续委派 |
| 状态 | 实现、独立复核、生产/Desktop 实际验收和最新基线最终 Gate 均已完成 |
| 对应 PR | Draft PR #321 为交付载体；实际 head/status 以 GitHub 为事实源 |

## 9. 完成回执

### 状态

PR #321 独立复核发现的相邻完成态回归已修复；实现、独立复核、生产与 Desktop 实际验收、
最新同步基线最终 Gate 均已完成。Draft PR #321 为交付载体，实际 head/status 以 GitHub 为准。

### 修改文件

- `internal/agent`：`artifact_contract.go`（含迁入的句界扫描）、
  `artifact_contract_helpers.go` 及对应测试、`final_readiness_test.go`
- `internal/evidence`：`artifact.go`、`evidence_test.go`
- 本目录的 `plan.md`、`progress.md` 和本任务文档

### 测试结果

- 新增 characterization 先复现：带代码 source 的直接 `change report.pdf` 被漏检，
  有序列表退化为单个匿名 target，导致缺少验证或只验证一项也 ready；修复后通过。
- 显式 `Change migrate.py`、`Replace app.ts` 直接代码目标负例通过，不建立 requirement。
- `go test ./internal/agent -run 'RF003|ArtifactRequirement|FinalReadiness' -count=1`：通过。
- `go test ./internal/agent ./internal/evidence ./internal/tool/builtin -count=1`：通过。
- `go run ./scripts/code_guard` 与 `git diff --check`：通过，未修改 guard baseline。
- latest `origin/main-v3@14decefc...` 上 common checks 全部通过。
- `node ./scripts/pr-gate.mjs --base origin/main-v3 --run all`：通过，分类为 targeted
  `./internal/agent ./internal/evidence`。

主会话另以项目 `benchmarks/prompt-eval` 的生产 `boot.Build → Controller.Run` 路径，
接入实际 `gpt/gpt-5.6-luna` OpenAI-compatible endpoint 和真实 workspace tools：Office、
Coding 通过；Design 的
Controller/readiness 通过，但 artifact grader 因 SVG canvas 0x0 仍有已知模型产物质量
偏差，不计为全绿。三个场景均未出现 readiness retry。

实际 Wails Desktop 使用原始编号列表输入自然完成，UI 待办为 3/3；会话最后一次产物
操作是 `verify_artifact` 同时验证 `report.pdf` 和 `summary.xlsx`，无 readiness 阻断或重试。

### 偏差、剩余风险与交接

无外部 API/schema/依赖变更或 matcher 语义放宽。`replace/change` 现在只检查动作解析出的
直接 target 是否全为代码路径；有序 Markdown 标记由句界与 target 分割共享的词法 helper
识别并剥离，支持 `1.`、`1)`、`(1)`，保留 dash list 和原有格式兼容行为。

剩余风险集中在自然语言句界长尾；自动化、生产 Controller 和实际 Desktop 已覆盖关键路径。
Design SVG canvas 0x0 是模型产物质量偏差，不是 readiness 回归。剩余外部门禁是交付分支
推送后的合并评审；本回执不声称 Draft PR #321 已合并。

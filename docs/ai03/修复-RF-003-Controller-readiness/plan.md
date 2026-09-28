# RF-003 Controller readiness 误判修复计划

## 1. 背景与问题

RF-003 在 Desktop 真实评测中表现为：任务已经生成正确产物，最后一次 `verify_artifact`
也成功返回工作区内的 canonical paths，之后没有新的工具调用，但 Controller 仍以
`requested workspace artifact has not been verified after the latest possible workspace mutation`
阻止 final answer，并在重复三次后异常结束。

该现象在 baseline 和 Extended V2 candidate 中均复现，覆盖 Office 项目报告、Coding
深层合并和 Design 仪表盘；因此不是提示词候选、单一 Scene 或 grader 规则导致的问题。

## 2. 基线与证据

| 项目 | 值 |
| --- | --- |
| 最新远端基线 | `origin/main-v3@8206dd5b03d8e98d614487b1ca6dfe233cdb8385` |
| 实现 worktree | `D:\Reasonix-worktrees\fix-rf003-controller-readiness` |
| 分支 | `codex/fix-rf003-controller-readiness` |
| Codegraph 工作区 | `D:\Reasonix` |
| Codegraph 状态 | 1,730 files、41,238 nodes、153,187 edges，`pendingChanges=0`、`worktreeMismatch=null` |

Codegraph 数据库不暴露 Git commit 字段。可证明的导航基线是：索引工作区在查询时干净、
HEAD 与 `origin/main-v3` 同为 `8206dd5b...`，且 Codegraph 报告零 pending changes。
Codegraph 只用于调用链和影响面导航；下列结论均已用当前 worktree 源码、真实会话和确定性测试复核。

主要调用和数据流：

`Agent.Run` → `captureArtifactRequirement` → `artifactRequirementFromInput`
→ `artifactTargetsFromActions` / `artifactOutputTarget` → `ArtifactRequirement.Targets`
→ `executeOneWithRegistry` → `recordToolReceipt`
→ `Ledger.HasFreshSuccessfulArtifactVerification` → `artifactReadinessFailure`
→ `finalReadinessCheck` → retry / terminal error。

Active Goal 还通过 `GoalReadinessFailure` → `Controller.advanceGoalAfterTurn` 消费同一
readiness 结果。Desktop、CLI、ACP 和 prompt-eval runner 均经共享 boot/Agent 路径使用该合同。

## 3. 已证实根因

根因是 artifact requirement parser 生成了错误或额外的 required targets，而不是验证工具、
receipt 或 mutation 顺序失败：

1. `artifactHardClauseEnd` 不把普通 ASCII 句号作为第一轮硬边界，动作扫描可能跨越
   后续说明句；文件扩展名中的句点又要求边界识别不能简单按 `.` 切分。
2. `artifactOutputTarget` 在过长 clause 中搜索 `as`、`with`、`for` 等连接词，后文
   连接词可能覆盖或截断动作的直接目标。
3. `replace`、`change` 等宽泛动作会把代码行为描述当成交付动作；当前
   `artifactActionDescribesCode` 在 clause 出现显式路径时会停止排除，因此 Coding
   任务中的 `settings.py` 反而使误判成立。
4. `artifactTargetsWithSourceExclusions` 会把“被提及但未识别为 target”的路径加入
   exclusion。一旦前面丢失真实输出，真实输出会反过来被禁止满足 contract。
5. readiness gate 对 target mismatch 使用统一的 mutation-oriented 文案，掩盖了
   实际缺失 target，诱发无效的重复验证。

三个真实输入的确定性结果：

| 场景 | 正确合同 | 当前错误合同 |
| --- | --- | --- |
| Office | `leadership-status.md`、`action-items.csv` | 匿名 CSV target，真实输出进入 exclusion |
| Coding | 不建立 artifact delivery requirement | 把 `replace base values` 解释为动作，要求 `settings.py` |
| Design | HTML、CSS、SVG 三个命名 target | 额外生成一个匿名 HTML target |

真实父会话证明三者的最终 `verify_artifact` 均成功，之后没有新工具调用仍被拒绝。最小
overlay 诊断在同一基线上把真实 prompt、真实 verify paths 直接送入生产 parser 与 ledger
matcher，三者均得到 `fresh=false`。

已经排除：提示词差异、verify 执行失败、receipt 未记录、Windows 路径规范化失败、
`read_file`/`verify_artifact` 被计为 mutation、后台任务覆盖，以及 grader/runner 契约。

## 4. 目标与非目标

### 目标

- Office prompt 只生成两个明确命名 targets，两个成功 verify receipts 后允许结束。
- Coding prompt 不建立 artifact delivery requirement。
- Design prompt 只生成三个明确命名 targets，三条成功 verify receipts 后允许结束。
- Profile Card 的 `Create profile-card.svg as ...` 保留命名 target。
- 句界识别兼容多点文件名、URL、版本/小数和跨行文件列表。
- `replace/change` 只有在局部语法表达明确交付/目标变换时才参与 artifact target 提取。
- target mismatch 的 readiness 信息展示所期待的 targets 和已验证 candidates，
  不再让用户误以为一定是 mutation 顺序问题。
- 保留现有 source/output 隔离、目标基数、distinct candidate、最新 mutation 后验证和
  background settlement 安全合同。

### 非目标

- 不修改系统提示词、grader、评测 runner、Desktop 前端或 `verify_artifact` 工具。
- 不用模型或外部服务替换确定性 parser。
- 不降低未验证产物的 readiness 门禁。
- 不改变 receipt 持久化、工作区身份或路径规范化协议。
- 不重构通用自然语言理解、todo、project checks 或其他 readiness 分支。

## 5. 选定方案

### 5.1 Sentence-aware、verb-scoped target 提取

- 增加小型确定性边界扫描器。ASCII `.` 只在构成句末时结束 action clause；文件名
  扩展、目录/URL、版本号和小数内部的句点不结束 clause。
- 保留 `following files:`/`these files:` 后跨行列表能力，其他换行仍是硬边界。
- `artifactOutputTarget` 只处理当前 action clause 内的直接目标和局部 destination；
  后续完整句的 `as/with/for` 不得回溯污染已识别 target。
- 明确命名 paths 优先。一旦 action 的直接目标已经给出命名 outputs，后续说明中的
  格式词不得添加重复匿名 target。

### 5.2 收紧 `replace/change` 动作资格

- 区分交付物变换语法和代码行为描述。
- 单纯描述运行时/配置语义，例如“override values replace base values”，即使同一后续
  句子提及代码文件，也不得创建 artifact requirement。
- 保留明确的 `replace old.pdf with new.pdf`、`rename old.pdf to new.pdf`、
  `convert data.csv to report.pdf` 等目标变换行为。
- 不通过删除所有宽泛动词来规避问题；正负测试锁定兼容边界。

### 5.3 无语义放宽的诊断增强

- matcher 继续使用同一 target/candidate 和 freshness 算法。
- 在失败时生成结构化或由同一匹配过程产生的诊断，至少列出未满足 target 和成功
  verify candidates；bool 兼容入口继续保留。
- readiness 文案区分 target mismatch 与 freshness/mutation ordering，不改变放行
  条件，不把 source 或单一 candidate 用于满足多个 targets。

若实现证明诊断必须复制匹配算法、改变放行语义或扩大为 readiness 重构，则停止该
部分，只交付 parser 修复并更新计划，不能引入两套可能漂移的判断。

## 6. 验收标准

| ID | 场景 | 验收结果 |
| --- | --- | --- |
| AC-01 | Office 原始 prompt | 精确得到两个命名 targets；验证两个输出后 ready |
| AC-02 | Coding 原始 prompt | 不建立 artifact requirement；不会要求验证 `settings.py` |
| AC-03 | Design 原始 prompt | 精确得到三个命名 targets；验证三者后 ready |
| AC-04 | Profile Card prompt | `profile-card.svg` 是唯一命名 target |
| AC-05 | 多点文件名、URL、版本、小数 | 内部 `.` 不被当作句末；后续真正句子正确隔离 |
| AC-06 | 跨行 `following files:` 列表 | 所有列举输出仍被识别且保持基数 |
| AC-07 | 明确 replace/rename/convert | 输出目标保持当前支持行为 |
| AC-08 | source/output/forbidden 同时出现 | source 和 forbidden paths 不能满足 output |
| AC-09 | verify 在最后 mutation 前 | 继续 hard block；重新 verify 后才允许结束 |
| AC-10 | target mismatch | 信息显示实际未满足 target/candidates，不伪装成纯顺序问题 |
| AC-11 | Desktop/生产 Controller 路径 | 三个 RF-003 场景不出现 readiness retry，正常结束 |

## 7. 影响范围、兼容性与回滚

预计实现限于 `internal/agent/artifact_contract.go`、`artifact_contract_helpers.go`、对应
parser/readiness tests，以及诊断复用 matcher 所必需的 `internal/evidence/artifact.go` 及其测试。

外部 API、schema、依赖、存储和迁移均不变。行为影响覆盖所有共享 Agent 入口，但限定在
artifact intent 解析和失败诊断。回滚时撤销同一实现提交即可；没有数据需要恢复。

## 8. 风险与控制

| 风险 | 控制 |
| --- | --- |
| 句点扫描截断 `report.final.pdf`、URL 或版本号 | 独立边界表驱动测试，保留多点文件现有测试 |
| 句界过松继续吸收后文说明 | 三个原始 prompt 及短化反例固定为 characterization tests |
| `replace/change` 收紧导致真实转换任务漏检 | 为 replace/rename/convert 的 from/to/as/with 语法保留正向测试 |
| 明确 path 优先掩盖真正的额外交付物 | 多 action、多文件列表和 additive steer 测试保持独立 target |
| 诊断与 matcher 结果漂移 | 诊断必须由同一匹配过程产生，bool wrapper 复用该结果 |
| 共享入口产生未知回归 | 受影响包测试、Controller 生产路径验收和完整 PR Gate |

整体实现难度中等，行为风险中等偏高；通过原始 prompt 回归、现有安全负例和实际 Controller
验收可压到可接受的中等水平。

## 9. 验证计划

### 自动化

迭代期运行最小 parser/readiness 测试；提交前至少运行：

```powershell
go test ./internal/agent ./internal/evidence ./internal/tool/builtin -count=1
```

若只修改上述 Root Go 包，不满足仓库广泛 Go 变更阈值，不主动运行 `go test ./...`。PR 前以
仓库 Gate 分类结果为准，并执行：

```powershell
go run ./scripts/code_guard
node ./scripts/pr-gate.mjs --base origin/main-v3 --run all
```

### 实际应用验收

1. 使用 Office、Coding、Design 三条原始 prompt 和真实目标路径，通过生产
   Controller/Agent 构建路径执行 writer → read → real `verify_artifact` → final。
2. 断言每条场景只形成预期 contract，最后 verify 后 `ReadinessResult` 为 ready，
   session 中没有 host-injected readiness retry。
3. 若本机评测 profile 和模型凭据可用，再运行三个定向 Desktop prompt-eval 场景，
   检查真实产物、工具事件和正常结束状态。
4. 若凭据或外部模型使真实 Desktop 运行不可重复，则以生产 boot/Controller 加真实
   workspace tools、可控 provider 的应用级 harness 作为稳定验收，并明确记录边界；
   不以纯 parser 单测代替应用验收。

实际行为稳定后 fetch/sync 最新 `origin/main-v3`，重跑聚焦测试和三个关键场景，再
运行最终 PR Gate。任何必需检查失败时不得创建或更新 PR。

## 10. 交付方式

用户已明确授权将计划、任务拆解、实现、验证和任务回执放在一个 Draft PR 中完成，不等待
独立文档 PR 合并。该例外只合并 PR 生命周期，不降低阶段内容、主会话审查、实际应用验收、
同步和 Gate 要求。

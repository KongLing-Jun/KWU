# G35 · internal/profile · store 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G35 |
| 问题数量 | 10 |
| 整体优先级 | P3 |
| 涉及模块 | `internal/profile` |
| 涉及文件 | `internal/profile/selector.go`<br>`internal/profile/store.go`<br>`internal/profile/event.go`<br>`internal/profile/decay.go`<br>`internal/profile/model.go`<br>`internal/profile/worker.go`<br>`internal/profile/extractor.go` |
| 主要治理目标 | 围绕 `internal/profile/store.go` 等治理对象降低复杂度/体量，保持每个单规则对象独立可认领，并保持调用链和行为边界。 |
| 推荐负责人范围 | profile 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 低：仍需开始前复查开放 PR |
| 冲突依据 | 未发现静态热点或本地远端引用重叠 |
| 建议处理顺序 | 总序第 35 组；无同文件组依赖 |
| 前置依赖 | 无同文件前置组；开始前仍需同步最新 main-v3 |
| 后续验证重点 | 返回值、错误语义和现有调用顺序；持久化、迁移或文件格式兼容；并发、取消或事件顺序 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **42/100**，对象 `Selector.scoreItem` 的分项为严重度 11.2、集中度 2.3、影响面 9.0、可执行性 20.0；最高超限 2.70x，文件内 1 条问题，CodeGraph 记录 0 个可信生产调用方、1 个待核候选。

### 治理边界与组间关系

本组涉及 7 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。没有同文件前置组。没有同文件后续组。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-336` | `complexity\|internal/profile/selector.go\|Selector.scoreItem` | 圈复杂度 | `internal/profile/selector.go` | `Selector.scoreItem` | 155 | 27/10 | +17（2.70x） | P3 | 0 可信 + 1 待核 caller / 6 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-338` | `complexity\|internal/profile/store.go\|Store.ApplySuccessfulOverride` | 圈复杂度 | `internal/profile/store.go` | `Store.ApplySuccessfulOverride` | 228 | 17/10 | +7（1.70x） | P3 | 0 可信 + 1 待核 caller / 2 可信 + 3 待核 callee / 2 unresolved | 待修改 |
| `CX-330` | `complexity\|internal/profile/event.go\|BuildLearningEvent` | 圈复杂度 | `internal/profile/event.go` | `BuildLearningEvent` | 123 | 17/10 | +7（1.70x） | P3 | 1 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-331` | `complexity\|internal/profile/event.go\|ProfileLearningEvent.Normalize` | 圈复杂度 | `internal/profile/event.go` | `ProfileLearningEvent.Normalize` | 192 | 13/10 | +3（1.30x） | P3 | 0 可信 + 0 待核 caller / 12 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-329` | `complexity\|internal/profile/decay.go\|EffectiveConfidence` | 圈复杂度 | `internal/profile/decay.go` | `EffectiveConfidence` | 15 | 11/10 | +1（1.10x） | P3 | 5 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-333` | `complexity\|internal/profile/model.go\|ProfileItem.Normalize` | 圈复杂度 | `internal/profile/model.go` | `ProfileItem.Normalize` | 78 | 14/10 | +4（1.40x） | P3 | 0 可信 + 0 待核 caller / 10 可信 + 3 待核 callee / 0 unresolved | 待修改 |
| `CX-340` | `complexity\|internal/profile/worker.go\|*Worker.process` | 圈复杂度 | `internal/profile/worker.go` | `*Worker.process` | 101 | 15/10 | +5（1.50x） | P3 | 0 可信 + 1 待核 caller / 1 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-337` | `complexity\|internal/profile/store.go\|Store.MaintainNegative` | 圈复杂度 | `internal/profile/store.go` | `Store.MaintainNegative` | 191 | 13/10 | +3（1.30x） | P3 | 0 可信 + 0 待核 caller / 1 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-332` | `complexity\|internal/profile/extractor.go\|RuleExtractor.Extract` | 圈复杂度 | `internal/profile/extractor.go` | `RuleExtractor.Extract` | 37 | 12/10 | +2（1.20x） | P3 | 0 可信 + 0 待核 caller / 3 可信 + 6 待核 callee / 0 unresolved | 待修改 |
| `CX-339` | `complexity\|internal/profile/store.go\|safeArchiveStem` | 圈复杂度 | `internal/profile/store.go` | `safeArchiveStem` | 474 | 11/10 | +1（1.10x） | P3 | 1 可信 + 0 待核 caller / 0 可信 + 1 待核 callee / 2 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-336 · 圈复杂度 · `Selector.scoreItem`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-336 |
| Baseline key | `complexity\|internal/profile/selector.go\|Selector.scoreItem` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P1，治理对象得分 42） |
| 文件 | `internal/profile/selector.go` |
| 代码行 | 155 |
| 函数/方法/包 | `Selector.scoreItem` |
| Baseline / 当前复测 / 阈值 | 27 / 27 / 10 |
| 超出 | +17，2.70x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`Selector.scoreItem` 的圈复杂度当前为 **27**，超过限制 **10**（+17，2.70x）。定义范围为 155-211 行；可见控制流热点：L156 `if !s.Policy.LoadingEnabled(item.Scope) {`；L165 `if item.ExpiresAt != nil && !item.ExpiresAt.IsZero() && !item.ExpiresAt.After(req.Now) {`；L176 `if confidence < 0.45 {`；L181 `if confidence < 0.6 {`；L194 `if item.Scope == ScopeProject {`；L206 `if item.Kind == KindNegative {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（Policy::LoadingEnabled、ProfileItem::CanLoad）、状态与控制（keySetHas）、持久化/I-O（genericProfileKind、ProfileItem::CanLoad）、任务/并发（taskTerms::overlap）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Selector::scoreItem（internal/profile/selector.go:155-211）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `Selector::Select`（`internal/profile/selector.go:67`，调用点 L94，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（6）：** `keySetHas`（`internal/profile/policy.go:201`，调用点 L171，confidence=0.90，exact-match，2 条边）；`EffectiveConfidence`（`internal/profile/decay.go:15`，调用点 L175，confidence=0.90，exact-match）；`Policy::LoadingEnabled`（`internal/profile/policy.go:51`，调用点 L156，confidence=0.90，exact-match）；`genericProfileKind`（`internal/profile/selector.go:425`，调用点 L188，confidence=0.90，exact-match）；`sameWorkspace`（`internal/profile/selector.go:438`，调用点 L168，confidence=0.90，exact-match）；`min`（`internal/profile/selector.go:451`，调用点 L199，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `ProfileItem::CanLoad`（`internal/profile/model.go:132`，调用点 L162，confidence=0.70，instance-method，待源码复核）；`taskTerms::overlap`（`internal/profile/selector.go:313`，调用点 L179，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Selector::Select`（`internal/profile/selector.go:67`，调用点 L94，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/profile`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/profile/decay_test.go`、`internal/profile/event_test.go`、`internal/profile/extractor_test.go`、`internal/profile/format_test.go`、`internal/profile/model_test.go`、`internal/profile/path_test.go`、`internal/profile/policy_test.go`、`internal/profile/selector_test.go`、`internal/profile/service_test.go`、`internal/profile/store_test.go`、`internal/profile/worker_test.go`
- **需同步检查：** `internal/profile/selector.go`、`internal/profile/policy.go`、`internal/profile/decay.go`、`internal/profile/decay_test.go`、`internal/profile/event_test.go`、`internal/profile/extractor_test.go`、`internal/profile/format_test.go`

#### 待修改项

- [ ] 逐段核对 L156、L168、L180、L194、L206 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `Selector.scoreItem` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/profile/selector.go|Selector.scoreItem` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/profile/selector.go:155-211` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `Selector.scoreItem` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/profile/decay_test.go`、`internal/profile/event_test.go`、`internal/profile/extractor_test.go`、`internal/profile/format_test.go`、`internal/profile/model_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/profile`。
- [ ] 运行相关包 vet：`go vet ./internal/profile`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/profile/selector.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/profile/selector.go|Selector.scoreItem` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-338 · 圈复杂度 · `Store.ApplySuccessfulOverride`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-338 |
| Baseline key | `complexity\|internal/profile/store.go\|Store.ApplySuccessfulOverride` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 39） |
| 文件 | `internal/profile/store.go` |
| 代码行 | 228 |
| 函数/方法/包 | `Store.ApplySuccessfulOverride` |
| Baseline / 当前复测 / 阈值 | 17 / 17 / 10 |
| 超出 | +7，1.70x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`Store.ApplySuccessfulOverride` 的圈复杂度当前为 **17**，超过限制 **10**（+7，1.70x）。定义范围为 228-270 行；可见控制流热点：L229 `if evt.Kind != KindCompleted || evt.Strength != SignalStrong {`；L237 `for _, scope := range []Scope{ScopeGlobal, ScopeProject} {`；L240 `if err == ErrProfileStoreUnavailable && scope == ScopeProject {`；L250 `if items[idx].Kind != KindNegative || items[idx].Status == StatusDisabled || items[idx].Status == S…`；L260 `if scopeChanged {`；L265 `if changed >= negativeMaintenanceLimit {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（profileOverlap、Store::ApplySuccessfulOverride、Store::read、Store::rewrite）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Store::ApplySuccessfulOverride（internal/profile/store.go:228-270）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `Store::ApplySuccessfulOverride`（`internal/profile/store.go:228`，调用点 L256，confidence=0.70，exact-match，待源码复核）
- **可信被调用函数（2）：** `successfulOverrideSignal`（`internal/profile/store.go:439`，调用点 L232，confidence=0.90，exact-match）；`profileOverlap`（`internal/profile/store.go:455`，调用点 L253，confidence=0.90，exact-match）
- **待源码复核被调候选（3）：** `Store::ApplySuccessfulOverride`（`internal/profile/store.go:228`，调用点 L256，confidence=0.70，exact-match，待源码复核）；`Store::read`（`internal/profile/store.go:296`，调用点 L238，confidence=0.70，instance-method，待源码复核）；`Store::rewrite`（`internal/profile/store.go:340`，调用点 L261，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 2 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/profile`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/profile/decay_test.go`、`internal/profile/event_test.go`、`internal/profile/extractor_test.go`、`internal/profile/format_test.go`、`internal/profile/model_test.go`、`internal/profile/path_test.go`、`internal/profile/policy_test.go`、`internal/profile/selector_test.go`、`internal/profile/service_test.go`、`internal/profile/store_test.go`、`internal/profile/worker_test.go`
- **需同步检查：** `internal/profile/store.go`、`internal/profile/decay_test.go`、`internal/profile/event_test.go`、`internal/profile/extractor_test.go`、`internal/profile/format_test.go`

#### 待修改项

- [ ] 逐段核对 L229、L239、L247、L253、L265 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `Store.ApplySuccessfulOverride` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/profile/store.go|Store.ApplySuccessfulOverride` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/profile/store.go:228-270` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `Store.ApplySuccessfulOverride` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/profile/decay_test.go`、`internal/profile/event_test.go`、`internal/profile/extractor_test.go`、`internal/profile/format_test.go`、`internal/profile/model_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/profile`。
- [ ] 运行相关包 vet：`go vet ./internal/profile`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/profile/store.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/profile/store.go|Store.ApplySuccessfulOverride` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-330 · 圈复杂度 · `BuildLearningEvent`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-330 |
| Baseline key | `complexity\|internal/profile/event.go\|BuildLearningEvent` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 36） |
| 文件 | `internal/profile/event.go` |
| 代码行 | 123 |
| 函数/方法/包 | `BuildLearningEvent` |
| Baseline / 当前复测 / 阈值 | 17 / 17 / 10 |
| 超出 | +7，1.70x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`BuildLearningEvent` 的圈复杂度当前为 **17**，超过限制 **10**（+7，1.70x）。定义范围为 123-190 行；可见控制流热点：L139 `switch {`；L144 `case obs.NegativeFeedback:`；L152 `case errors.Is(obs.RunErr, context.Canceled) && obs.CancelRequested:`；L164 `case obs.GoalStatus == GoalStatusBlocked:`；L174 `case obs.GoalStatus == GoalStatusComplete && strings.TrimSpace(obs.GoalReadinessFailure) == "":`；L183 `case strings.TrimSpace(obs.UserFeedback) != "":`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function BuildLearningEvent（internal/profile/event.go:123-190）
- **可信生产调用方（1）：** `Controller::enqueueProfileLearningObservation`（`internal/control/profile_learning.go:13`，调用点 L17，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `Controller::Run`（`internal/control/controller.go:1177`，调用点 L1188，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/profile`、`internal/control`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/profile/event_test.go`
- **同目录测试候选：** `internal/profile/decay_test.go`、`internal/profile/extractor_test.go`、`internal/profile/format_test.go`、`internal/profile/model_test.go`、`internal/profile/path_test.go`、`internal/profile/policy_test.go`、`internal/profile/selector_test.go`、`internal/profile/service_test.go`、`internal/profile/store_test.go`、`internal/profile/worker_test.go`
- **需同步检查：** `internal/profile/event.go`、`internal/control/profile_learning.go`、`internal/profile/event_test.go`、`internal/profile/decay_test.go`、`internal/profile/extractor_test.go`、`internal/profile/format_test.go`

#### 待修改项

- [ ] 逐段核对 L139、L148、L160、L169、L183 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `BuildLearningEvent` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/profile/event.go|BuildLearningEvent` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/profile/event.go:123-190` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `BuildLearningEvent` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：并发、取消或事件顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/profile/event_test.go`。
- [ ] 运行相关包测试：`go test ./internal/profile`。
- [ ] 运行相关包 race：`go test -race ./internal/profile`。
- [ ] 运行相关包 vet：`go vet ./internal/profile`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/profile/event.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/profile/event.go|BuildLearningEvent` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-331 · 圈复杂度 · `ProfileLearningEvent.Normalize`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-331 |
| Baseline key | `complexity\|internal/profile/event.go\|ProfileLearningEvent.Normalize` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 36） |
| 文件 | `internal/profile/event.go` |
| 代码行 | 192 |
| 函数/方法/包 | `ProfileLearningEvent.Normalize` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`ProfileLearningEvent.Normalize` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 192-235 行；可见控制流热点：L194 `if e.Kind == "" {`；L202 `if e.Strength == "" {`；L213 `if e.Subagent != nil {`；L220 `if e.ConfidenceHint < 0.9 {`；L224 `if e.Kind == KindCompleted && e.Strength == SignalStrong && !e.hasStrongEvidence() {`；L230 `if e.ConfidenceHint == 0 {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（normalizeLearningKind、normalizeLearningSource、normalizeSignalStrength、normalizeTranscript）、持久化/I-O（ProfileLearningEvent::hasStrongEvidence）、网络/协议（compactEventText、ProfileLearningEvent::hasStrongEvidence）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method ProfileLearningEvent::Normalize（internal/profile/event.go:192-235）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（12）：** `normalizeLearningKind`（`internal/profile/event.go:296`，调用点 L193，confidence=0.90，exact-match）；`normalizeLearningSource`（`internal/profile/event.go:305`，调用点 L197，confidence=0.90，exact-match）；`normalizeSignalStrength`（`internal/profile/event.go:316`，调用点 L201，confidence=0.90，exact-match）；`defaultStrength`（`internal/profile/event.go:325`，调用点 L203，confidence=0.90，exact-match）；`defaultConfidence`（`internal/profile/event.go:334`，调用点 L231，confidence=0.90，exact-match）；`normalizeTranscript`（`internal/profile/event.go:348`，调用点 L209，confidence=0.90，exact-match）；`normalizeTools`（`internal/profile/event.go:362`，调用点 L210，confidence=0.90，exact-match）；`normalizeVerification`（`internal/profile/event.go:378`，调用点 L212，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `ProfileLearningEvent::hasStrongEvidence`（`internal/profile/event.go:286`，调用点 L224，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/profile`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/profile/event_test.go`、`internal/profile/model_test.go`、`internal/profile/store_test.go`
- **同目录测试候选：** `internal/profile/decay_test.go`、`internal/profile/extractor_test.go`、`internal/profile/format_test.go`、`internal/profile/path_test.go`、`internal/profile/policy_test.go`、`internal/profile/selector_test.go`、`internal/profile/service_test.go`、`internal/profile/worker_test.go`
- **需同步检查：** `internal/profile/event.go`、`internal/profile/event_test.go`、`internal/profile/model_test.go`、`internal/profile/store_test.go`、`internal/profile/decay_test.go`

#### 待修改项

- [ ] 逐段核对 L194、L202、L218、L224、L230 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O、网络/协议 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `ProfileLearningEvent.Normalize` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/profile/event.go|ProfileLearningEvent.Normalize` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/profile/event.go:192-235` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `ProfileLearningEvent.Normalize` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/profile/event_test.go`、`internal/profile/model_test.go`、`internal/profile/store_test.go`。
- [ ] 运行相关包测试：`go test ./internal/profile`。
- [ ] 运行相关包 vet：`go vet ./internal/profile`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/profile/event.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/profile/event.go|ProfileLearningEvent.Normalize` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-329 · 圈复杂度 · `EffectiveConfidence`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-329 |
| Baseline key | `complexity\|internal/profile/decay.go\|EffectiveConfidence` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 35） |
| 文件 | `internal/profile/decay.go` |
| 代码行 | 15 |
| 函数/方法/包 | `EffectiveConfidence` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`EffectiveConfidence` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 15-40 行；可见控制流热点：L17 `if item.Kind != KindNegative {`；L20 `if now.IsZero() {`；L24 `if item.Decay.LastHitAt != nil && !item.Decay.LastHitAt.IsZero() {`；L31 `if basis.IsZero() || !now.After(basis) {`；L35 `if halfLife <= 0 {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function EffectiveConfidence（internal/profile/decay.go:15-40）
- **可信生产调用方（5）：** `ApplyNegativeDecay`（`internal/profile/decay.go:42`，调用点 L49，confidence=0.90，exact-match）；`BoostNegativeHit`（`internal/profile/decay.go:57`，调用点 L65，confidence=0.90，exact-match）；`ApplySuccessfulOverride`（`internal/profile/decay.go:78`，调用点 L85，confidence=0.90，exact-match）；`Selector::Select`（`internal/profile/selector.go:67`，调用点 L99，confidence=0.90，exact-match）；`Selector::scoreItem`（`internal/profile/selector.go:155`，调用点 L175，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `clampConfidence`（`internal/profile/policy.go:248`，调用点 L16/L39，confidence=0.90，exact-match，2 条边）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `Selector::Select`（`internal/profile/selector.go:67`，调用点 L99，confidence=0.90，exact-match，待源码复核）；`Store::RecordUse`（`internal/profile/store.go:118`，调用点 L126，confidence=0.90，exact-match，待源码复核）；`Store::MaintainNegative`（`internal/profile/store.go:191`，调用点 L209，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/profile`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/profile/decay_test.go`、`internal/profile/event_test.go`、`internal/profile/extractor_test.go`、`internal/profile/format_test.go`、`internal/profile/model_test.go`、`internal/profile/path_test.go`、`internal/profile/policy_test.go`、`internal/profile/selector_test.go`、`internal/profile/service_test.go`、`internal/profile/store_test.go`、`internal/profile/worker_test.go`
- **需同步检查：** `internal/profile/decay.go`、`internal/profile/selector.go`、`internal/profile/policy.go`、`internal/profile/decay_test.go`、`internal/profile/event_test.go`、`internal/profile/extractor_test.go`、`internal/profile/format_test.go`

#### 待修改项

- [ ] 逐段核对 L17、L20、L24、L31、L35 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `EffectiveConfidence` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/profile/decay.go|EffectiveConfidence` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/profile/decay.go:15-40` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `EffectiveConfidence` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/profile/decay_test.go`、`internal/profile/event_test.go`、`internal/profile/extractor_test.go`、`internal/profile/format_test.go`、`internal/profile/model_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/profile`。
- [ ] 运行相关包 vet：`go vet ./internal/profile`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/profile/decay.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/profile/decay.go|EffectiveConfidence` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-333 · 圈复杂度 · `ProfileItem.Normalize`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-333 |
| Baseline key | `complexity\|internal/profile/model.go\|ProfileItem.Normalize` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 35） |
| 文件 | `internal/profile/model.go` |
| 代码行 | 78 |
| 函数/方法/包 | `ProfileItem.Normalize` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`ProfileItem.Normalize` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 78-130 行；可见控制流热点：L79 `if now.IsZero() {`；L88 `if !validKind(i.Kind) {`；L96 `if i.Text == "" {`；L110 `if i.Confidence > 1 {`；L116 `if i.CreatedAt.IsZero() {`；L126 `if i.ID == "" {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（normalizeStatus、normalizeSemanticKey、normalizeTags、normalizeSource）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method ProfileItem::Normalize（internal/profile/model.go:78-130）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（10）：** `validScope`（`internal/profile/model.go:140`，调用点 L84，confidence=0.90，exact-match）；`validKind`（`internal/profile/model.go:152`，调用点 L88，confidence=0.90，exact-match）；`normalizeStatus`（`internal/profile/model.go:161`，调用点 L91，confidence=0.90，exact-match）；`validStatus`（`internal/profile/model.go:169`，调用点 L92，confidence=0.90，exact-match）；`normalizeSemanticKey`（`internal/profile/model.go:182`，调用点 L100，confidence=0.90，exact-match）；`semanticKeyFor`（`internal/profile/model.go:186`，调用点 L102，confidence=0.90，exact-match）；`normalizeTags`（`internal/profile/model.go:190`，调用点 L104，confidence=0.90，exact-match）；`normalizeSource`（`internal/profile/model.go:205`，调用点 L105，confidence=0.90，exact-match）
- **待源码复核被调候选（3）：** `normalizeScope`（`internal/profile/model.go:136`，调用点 L83，confidence=0.70，exact-match，待源码复核）；`normalizeKind`（`internal/profile/model.go:144`，调用点 L87，confidence=0.70，exact-match，待源码复核）；`oneLine`（`internal/profile/model.go:178`，调用点 L95，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/profile`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/profile/decay_test.go`、`internal/profile/event_test.go`、`internal/profile/extractor_test.go`、`internal/profile/format_test.go`、`internal/profile/model_test.go`、`internal/profile/path_test.go`、`internal/profile/policy_test.go`、`internal/profile/selector_test.go`、`internal/profile/service_test.go`、`internal/profile/store_test.go`、`internal/profile/worker_test.go`
- **需同步检查：** `internal/profile/model.go`、`internal/profile/decay_test.go`、`internal/profile/event_test.go`、`internal/profile/extractor_test.go`、`internal/profile/format_test.go`

#### 待修改项

- [ ] 逐段核对 L79、L92、L107、L113、L126 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `ProfileItem.Normalize` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/profile/model.go|ProfileItem.Normalize` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/profile/model.go:78-130` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `ProfileItem.Normalize` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/profile/decay_test.go`、`internal/profile/event_test.go`、`internal/profile/extractor_test.go`、`internal/profile/format_test.go`、`internal/profile/model_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/profile`。
- [ ] 运行相关包 vet：`go vet ./internal/profile`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/profile/model.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/profile/model.go|ProfileItem.Normalize` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-340 · 圈复杂度 · `*Worker.process`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-340 |
| Baseline key | `complexity\|internal/profile/worker.go\|*Worker.process` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 35） |
| 文件 | `internal/profile/worker.go` |
| 代码行 | 101 |
| 函数/方法/包 | `*Worker.process` |
| Baseline / 当前复测 / 阈值 | 15 / 15 / 10 |
| 超出 | +5，1.50x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*Worker.process` 的圈复杂度当前为 **15**，超过限制 **10**（+5，1.50x）。定义范围为 101-151 行；可见控制流热点：L103 `if !ok {`；L110 `if !errors.Is(err, context.Canceled) {`；L120 `if errors.Is(err, ErrSensitiveProfile) {`；L130 `if _, err := w.store.Upsert(item); err != nil {`；L141 `if _, err := w.store.ApplySuccessfulOverride(evt, now); err != nil {`；L148 `if total := savedByScope[ScopeGlobal] + savedByScope[ScopeProject]; total > 0 {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Worker::process（internal/profile/worker.go:101-151）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `Worker::run`（`internal/profile/worker.go:89`，调用点 L96，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（1）：** `Policy::PrepareCandidate`（`internal/profile/policy.go:86`，调用点 L118，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `ApplySuccessfulOverride`（`internal/profile/decay.go:78`，调用点 L141，confidence=0.70，exact-match，待源码复核）；`Extractor::Extract`（`internal/profile/extractor.go:27`，调用点 L108，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** `Worker::run`（`internal/profile/worker.go:89`，调用点 L96，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/profile`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/profile/worker_test.go`
- **同目录测试候选：** `internal/profile/decay_test.go`、`internal/profile/event_test.go`、`internal/profile/extractor_test.go`、`internal/profile/format_test.go`、`internal/profile/model_test.go`、`internal/profile/path_test.go`、`internal/profile/policy_test.go`、`internal/profile/selector_test.go`、`internal/profile/service_test.go`、`internal/profile/store_test.go`
- **需同步检查：** `internal/profile/worker.go`、`internal/profile/policy.go`、`internal/profile/decay.go`、`internal/profile/extractor.go`、`internal/profile/worker_test.go`、`internal/profile/decay_test.go`、`internal/profile/event_test.go`、`internal/profile/extractor_test.go`

#### 待修改项

- [ ] 逐段核对 L103、L117、L127、L140、L148 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `*Worker.process` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/profile/worker.go|*Worker.process` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/profile/worker.go:101-151` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*Worker.process` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：并发、取消或事件顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/profile/worker_test.go`。
- [ ] 运行相关包测试：`go test ./internal/profile`。
- [ ] 运行相关包 race：`go test -race ./internal/profile`。
- [ ] 运行相关包 vet：`go vet ./internal/profile`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/profile/worker.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/profile/worker.go|*Worker.process` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-337 · 圈复杂度 · `Store.MaintainNegative`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-337 |
| Baseline key | `complexity\|internal/profile/store.go\|Store.MaintainNegative` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 34） |
| 文件 | `internal/profile/store.go` |
| 代码行 | 191 |
| 函数/方法/包 | `Store.MaintainNegative` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`Store.MaintainNegative` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 191-226 行；可见控制流热点：L193 `for _, scope := range []Scope{ScopeGlobal, ScopeProject} {`；L196 `if err == ErrProfileStoreUnavailable && scope == ScopeProject {`；L203 `if changed >= negativeMaintenanceLimit {`；L206 `if items[idx].Kind != KindNegative {`；L216 `if scopeChanged {`；L221 `if changed >= negativeMaintenanceLimit {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（Store::read、Store::rewrite）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Store::MaintainNegative（internal/profile/store.go:191-226）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `ApplyNegativeDecay`（`internal/profile/decay.go:42`，调用点 L209，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `Store::read`（`internal/profile/store.go:296`，调用点 L194，confidence=0.70，instance-method，待源码复核）；`Store::rewrite`（`internal/profile/store.go:340`，调用点 L217，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/profile`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/profile/decay_test.go`、`internal/profile/event_test.go`、`internal/profile/extractor_test.go`、`internal/profile/format_test.go`、`internal/profile/model_test.go`、`internal/profile/path_test.go`、`internal/profile/policy_test.go`、`internal/profile/selector_test.go`、`internal/profile/service_test.go`、`internal/profile/store_test.go`、`internal/profile/worker_test.go`
- **需同步检查：** `internal/profile/store.go`、`internal/profile/decay.go`、`internal/profile/decay_test.go`、`internal/profile/event_test.go`、`internal/profile/extractor_test.go`、`internal/profile/format_test.go`

#### 待修改项

- [ ] 逐段核对 L193、L196、L206、L216、L221 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `Store.MaintainNegative` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/profile/store.go|Store.MaintainNegative` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/profile/store.go:191-226` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `Store.MaintainNegative` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/profile/decay_test.go`、`internal/profile/event_test.go`、`internal/profile/extractor_test.go`、`internal/profile/format_test.go`、`internal/profile/model_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/profile`。
- [ ] 运行相关包 vet：`go vet ./internal/profile`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/profile/store.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/profile/store.go|Store.MaintainNegative` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-332 · 圈复杂度 · `RuleExtractor.Extract`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-332 |
| Baseline key | `complexity\|internal/profile/extractor.go\|RuleExtractor.Extract` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 33） |
| 文件 | `internal/profile/extractor.go` |
| 代码行 | 37 |
| 函数/方法/包 | `RuleExtractor.Extract` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`RuleExtractor.Extract` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 37-63 行；可见控制流热点：L38 `if err := ctx.Err(); err != nil {`；L42 `if !ok {`；L49 `if strings.TrimSpace(evt.UserFeedback) != "" && (evt.Kind == KindExplicitRemember || LooksLikeExpli…`；L52 `if evt.Strength == SignalNegative || evt.Kind == KindFailed || evt.Kind == KindInterrupted || Looks…`；L55 `if c, ok := e.verificationCandidate(evt, now); ok {`；L58 `if evt.Subagent != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（normalizeCandidates）、持久化/I-O（ArchiveSink::Err）、网络/协议（RuleExtractor::limitEvent）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method RuleExtractor::Extract（internal/profile/extractor.go:37-63）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `LooksLikeExplicitRemember`（`internal/profile/event.go:237`，调用点 L49，confidence=0.90，exact-match）；`LooksLikeNegativeFeedback`（`internal/profile/event.go:259`，调用点 L52，confidence=0.90，exact-match）；`normalizeCandidates`（`internal/profile/extractor.go:179`，调用点 L62，confidence=0.90，exact-match）
- **待源码复核被调候选（6）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L38，confidence=0.70，instance-method，待源码复核）；`RuleExtractor::explicitFeedbackCandidate`（`internal/profile/extractor.go:65`，调用点 L50，confidence=0.70，instance-method，待源码复核）；`RuleExtractor::negativeCandidate`（`internal/profile/extractor.go:82`，调用点 L53，confidence=0.70，instance-method，待源码复核）；`RuleExtractor::verificationCandidate`（`internal/profile/extractor.go:105`，调用点 L55，confidence=0.70，instance-method，待源码复核）；`RuleExtractor::subagentCandidate`（`internal/profile/extractor.go:126`，调用点 L59，confidence=0.70，instance-method，待源码复核）；`RuleExtractor::limitEvent`（`internal/profile/extractor.go:153`，调用点 L45，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/profile`、`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/profile/decay_test.go`、`internal/profile/event_test.go`、`internal/profile/extractor_test.go`、`internal/profile/format_test.go`、`internal/profile/model_test.go`、`internal/profile/path_test.go`、`internal/profile/policy_test.go`、`internal/profile/selector_test.go`、`internal/profile/service_test.go`、`internal/profile/store_test.go`、`internal/profile/worker_test.go`
- **需同步检查：** `internal/profile/extractor.go`、`internal/profile/event.go`、`internal/automation/archive_sink.go`、`internal/profile/decay_test.go`、`internal/profile/event_test.go`、`internal/profile/extractor_test.go`、`internal/profile/format_test.go`

#### 待修改项

- [ ] 逐段核对 L38、L42、L52、L55、L58 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O、网络/协议 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `RuleExtractor.Extract` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/profile/extractor.go|RuleExtractor.Extract` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/profile/extractor.go:37-63` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `RuleExtractor.Extract` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/profile/decay_test.go`、`internal/profile/event_test.go`、`internal/profile/extractor_test.go`、`internal/profile/format_test.go`、`internal/profile/model_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/profile`。
- [ ] 运行相关包 vet：`go vet ./internal/profile`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/profile/extractor.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/profile/extractor.go|RuleExtractor.Extract` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-339 · 圈复杂度 · `safeArchiveStem`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-339 |
| Baseline key | `complexity\|internal/profile/store.go\|safeArchiveStem` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 33） |
| 文件 | `internal/profile/store.go` |
| 代码行 | 474 |
| 函数/方法/包 | `safeArchiveStem` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`safeArchiveStem` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 474-485 行；可见控制流热点：L476 `for _, r := range s {`；L477 `if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '…`；L481 `if b.Len() == 0 {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function safeArchiveStem（internal/profile/store.go:474-485）
- **可信生产调用方（1）：** `Store::writeArchive`（`internal/profile/store.go:356`，调用点 L361，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（1）：** `Registry::Len`（`internal/tool/tool.go:242`，调用点 L481，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Store::Archive`（`internal/profile/store.go:134`，调用点 L150，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 2 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/profile`、`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/profile/decay_test.go`、`internal/profile/event_test.go`、`internal/profile/extractor_test.go`、`internal/profile/format_test.go`、`internal/profile/model_test.go`、`internal/profile/path_test.go`、`internal/profile/policy_test.go`、`internal/profile/selector_test.go`、`internal/profile/service_test.go`、`internal/profile/store_test.go`、`internal/profile/worker_test.go`
- **需同步检查：** `internal/profile/store.go`、`internal/tool/tool.go`、`internal/profile/decay_test.go`、`internal/profile/event_test.go`、`internal/profile/extractor_test.go`、`internal/profile/format_test.go`

#### 待修改项

- [ ] 逐段核对 L476、L477、L481 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `safeArchiveStem` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/profile/store.go|safeArchiveStem` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/profile/store.go:474-485` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `safeArchiveStem` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/profile/decay_test.go`、`internal/profile/event_test.go`、`internal/profile/extractor_test.go`、`internal/profile/format_test.go`、`internal/profile/model_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/profile`。
- [ ] 运行相关包 vet：`go vet ./internal/profile`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/profile/store.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/profile/store.go|safeArchiveStem` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

## 4. 本组完成条件

- [ ] 本组列出的职责拆分均完成，未夹带相邻无关重构。
- [ ] 所有关联调用方和测试均已复核。
- [ ] 聚焦测试、相关包测试、必要的 race/vet/build 检查通过。
- [ ] Code Guard 局部与全量检查完成。
- [ ] 对应 baseline key 明确下降或消失，并在最新主线基础上显式刷新。
- [ ] 未新增碎片文件、纯转发包装或重复实现。
- [ ] 与后续同文件组的接口和职责边界已记录。

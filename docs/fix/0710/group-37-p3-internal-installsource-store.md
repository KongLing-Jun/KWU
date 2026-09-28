# G37 · internal/installsource · store 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G37 |
| 问题数量 | 10 |
| 整体优先级 | P3 |
| 涉及模块 | `internal/installsource`、`internal/memory` |
| 涉及文件 | `internal/installsource/skill.go`<br>`internal/memory/store.go`<br>`internal/installsource/names.go`<br>`internal/installsource/apply.go`<br>`internal/installsource/mcp.go` |
| 主要治理目标 | 围绕 `internal/memory/store.go` 等治理对象降低复杂度/体量，统一处理 1 个多规则函数对象，并保持调用链和行为边界。 |
| 推荐负责人范围 | installsource / memory 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 中：认领前确认同包开发 |
| 冲突依据 | 单文件最多集中 6 条问题 |
| 建议处理顺序 | 总序第 37 组；等待 G23 的同文件变更合并 |
| 前置依赖 | G23（internal/installsource/mcp.go） |
| 后续验证重点 | 返回值、错误语义和现有调用顺序；持久化、迁移或文件格式兼容；配置、平台或向后兼容；会话、状态或恢复语义；协议、序列化或流式事件兼容 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **41/100**，对象 `scanSkillRoot` 的分项为严重度 6.6、集中度 3.6、影响面 10.7、可执行性 20.0；最高超限 1.70x，文件内 2 条问题，CodeGraph 记录 2 个可信生产调用方、0 个待核候选。

### 治理边界与组间关系

本组涉及 5 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。前置同文件组：G23。后续同文件组：G43，应串行衔接。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-285` | `complexity\|internal/installsource/skill.go\|scanSkillRoot` | 圈复杂度 | `internal/installsource/skill.go` | `scanSkillRoot` | 216 | 17/10 | +7（1.70x） | P3 | 2 可信 + 0 待核 caller / 4 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-296` | `complexity\|internal/memory/store.go\|Store.Index` | 圈复杂度 | `internal/memory/store.go` | `Store.Index` | 147 | 13/10 | +3（1.30x） | P3 | 0 可信 + 0 待核 caller / 2 可信 + 2 待核 callee / 14 unresolved | 待修改 |
| `ND-013` | `nesting_depth\|internal/memory/store.go\|Store.Index` | 嵌套深度 | `internal/memory/store.go` | `Store.Index` | 147 | 5/4 | +1（1.25x） | P3 | 0 可信 + 0 待核 caller / 2 可信 + 2 待核 callee / 14 unresolved | 待修改 |
| `CX-278` | `complexity\|internal/installsource/names.go\|sanitizeName` | 圈复杂度 | `internal/installsource/names.go` | `sanitizeName` | 158 | 16/10 | +6（1.60x） | P3 | 4 可信 + 0 待核 caller / 0 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-273` | `complexity\|internal/installsource/apply.go\|*installSourceTool.applyInstallMCP` | 圈复杂度 | `internal/installsource/apply.go` | `*installSourceTool.applyInstallMCP` | 195 | 18/10 | +8（1.80x） | P3 | 0 可信 + 1 待核 caller / 1 可信 + 4 待核 callee / 0 unresolved | 待修改 |
| `CX-300` | `complexity\|internal/memory/store.go\|Store.ListArchived` | 圈复杂度 | `internal/memory/store.go` | `Store.ListArchived` | 601 | 15/10 | +5（1.50x） | P3 | 1 可信 + 0 待核 caller / 2 可信 + 2 待核 callee / 14 unresolved | 待修改 |
| `CX-272` | `complexity\|internal/installsource/apply.go\|*installSourceTool.applySkillRoot` | 圈复杂度 | `internal/installsource/apply.go` | `*installSourceTool.applySkillRoot` | 51 | 11/10 | +1（1.10x） | P3 | 0 可信 + 1 待核 caller / 2 可信 + 3 待核 callee / 0 unresolved | 待修改 |
| `CX-276` | `complexity\|internal/installsource/mcp.go\|validateMCPEntry` | 圈复杂度 | `internal/installsource/mcp.go` | `validateMCPEntry` | 243 | 11/10 | +1（1.10x） | P3 | 1 可信 + 0 待核 caller / 2 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-277` | `complexity\|internal/installsource/names.go\|mcpNameFromURL` | 圈复杂度 | `internal/installsource/names.go` | `mcpNameFromURL` | 119 | 11/10 | +1（1.10x） | P3 | 1 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-286` | `complexity\|internal/installsource/skill.go\|copyDir` | 圈复杂度 | `internal/installsource/skill.go` | `copyDir` | 301 | 11/10 | +1（1.10x） | P3 | 0 可信 + 1 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-285 · 圈复杂度 · `scanSkillRoot`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-285 |
| Baseline key | `complexity\|internal/installsource/skill.go\|scanSkillRoot` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 41） |
| 文件 | `internal/installsource/skill.go` |
| 代码行 | 216 |
| 函数/方法/包 | `scanSkillRoot` |
| Baseline / 当前复测 / 阈值 | 17 / 17 / 10 |
| 超出 | +7，1.70x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`scanSkillRoot` 的圈复杂度当前为 **17**，超过限制 **10**（+7，1.70x）。定义范围为 216-280 行；可见控制流热点：L219 `if err != nil {`；L230 `if d.IsDir() {`；L239 `if len(out) >= maxSkillScanCount {`；L246 `if strings.EqualFold(d.Name(), skill.SkillFile) {`；L255 `if err == nil {`；L275 `if err != nil {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（readSkillFile）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function scanSkillRoot（internal/installsource/skill.go:216-280）
- **可信生产调用方（2）：** `installSourceTool::resolveRegisteredSkillRoot`（`internal/installsource/install_source.go:454`，调用点 L460，confidence=0.90，exact-match）；`installSourceTool::localSkillActions`（`internal/installsource/plan.go:344`，调用点 L393，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `pathDepth`（`internal/installsource/skill.go:282`，调用点 L229/L247/L263，confidence=0.90，exact-match，3 条边）；`readSkillFile`（`internal/installsource/skill.go:172`，调用点 L254/L268，confidence=0.90，exact-match，2 条边）；`mustRel`（`internal/installsource/skill.go:289`，调用点 L247/L263，confidence=0.90，exact-match，2 条边）；`newErr`（`internal/installsource/errors.go:70`，调用点 L240，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/installsource`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/installsource/install_source_test.go`
- **需同步检查：** `internal/installsource/skill.go`、`internal/installsource/install_source.go`、`internal/installsource/plan.go`、`internal/installsource/errors.go`、`internal/installsource/install_source_test.go`

#### 待修改项

- [ ] 逐段核对 L219、L231、L242、L255、L275 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `scanSkillRoot` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/installsource/skill.go|scanSkillRoot` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/installsource/skill.go:216-280` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `scanSkillRoot` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G43 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/installsource/install_source_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/installsource`。
- [ ] 运行相关包 vet：`go vet ./internal/installsource`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/installsource/skill.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/installsource/skill.go|scanSkillRoot` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-296 · 圈复杂度 · `Store.Index`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-296 |
| Baseline key | `complexity\|internal/memory/store.go\|Store.Index` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 41） |
| 文件 | `internal/memory/store.go` |
| 代码行 | 147 |
| 函数/方法/包 | `Store.Index` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | `ND-013` |

#### 明确的问题描述

`Store.Index` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 147-185 行；可见控制流热点：L149 `for _, dir := range s.dirs() {`；L154 `if err != nil {`；L158 `if mt := indexLineRe.FindStringSubmatch(line); mt != nil {`；L161 `if m, ok := loadMemory(path); ok && !s.visibleFromDir(dir, m.Type) {`；L171 `if len(managed) == 0 {`；L180 `for _, n := range names {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（loadMemory）、持久化/I-O（Store::dirs、Store::visibleFromDir）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Store::Index（internal/memory/store.go:147-185）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `safeJoin`（`internal/memory/store.go:394`，调用点 L160，confidence=0.90，exact-match）；`loadMemory`（`internal/memory/store.go:673`，调用点 L161，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `Store::dirs`（`internal/memory/store.go:136`，调用点 L149，confidence=0.70，instance-method，待源码复核）；`Store::visibleFromDir`（`internal/memory/store.go:648`，调用点 L161，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 14 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/memory`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/memory/store_test.go`
- **同目录测试候选：** `internal/memory/convention_test.go`、`internal/memory/forget_test.go`、`internal/memory/memory_test.go`、`internal/memory/quickadd_test.go`、`internal/memory/recall_test.go`、`internal/memory/remember_test.go`、`internal/memory/store_extra_test.go`、`internal/memory/writedoc_test.go`
- **需同步检查：** `internal/memory/store.go`、`internal/memory/store_test.go`、`internal/memory/convention_test.go`、`internal/memory/forget_test.go`、`internal/memory/memory_test.go`

#### 待修改项

- [ ] 逐段核对 L149、L157、L160、L171、L180 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `Store.Index` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-296、ND-013 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/memory/store.go|Store.Index` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/memory/store.go:147-185` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `Store.Index` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G43 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-296`、`ND-013` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/memory/store_test.go`。
- [ ] 运行相关包测试：`go test ./internal/memory`。
- [ ] 运行相关包 vet：`go vet ./internal/memory`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/memory/store.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/memory/store.go|Store.Index` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### ND-013 · 嵌套深度 · `Store.Index`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | ND-013 |
| Baseline key | `nesting_depth\|internal/memory/store.go\|Store.Index` |
| 规则 | nesting_depth（嵌套深度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 41） |
| 文件 | `internal/memory/store.go` |
| 代码行 | 147 |
| 函数/方法/包 | `Store.Index` |
| Baseline / 当前复测 / 阈值 | 5 / 5 / 4 |
| 超出 | +1，1.25x |
| 同一治理对象关联问题 | `CX-296` |

#### 明确的问题描述

`Store.Index` 的最大嵌套深度为 **5**，超过限制 **4**。嵌套相关控制点包括：L149 `for _, dir := range s.dirs() {`；L154 `if err != nil {`；L158 `if mt := indexLineRe.FindStringSubmatch(line); mt != nil {`；L161 `if m, ok := loadMemory(path); ok && !s.visibleFromDir(dir, m.Type) {`；L171 `if len(managed) == 0 {`；L180 `for _, n := range names {`。治理应优先减少条件层级，同时保持错误返回、循环退出和副作用顺序。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Store::Index（internal/memory/store.go:147-185）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `safeJoin`（`internal/memory/store.go:394`，调用点 L160，confidence=0.90，exact-match）；`loadMemory`（`internal/memory/store.go:673`，调用点 L161，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `Store::dirs`（`internal/memory/store.go:136`，调用点 L149，confidence=0.70，instance-method，待源码复核）；`Store::visibleFromDir`（`internal/memory/store.go:648`，调用点 L161，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 14 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/memory`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/memory/store_test.go`
- **同目录测试候选：** `internal/memory/convention_test.go`、`internal/memory/forget_test.go`、`internal/memory/memory_test.go`、`internal/memory/quickadd_test.go`、`internal/memory/recall_test.go`、`internal/memory/remember_test.go`、`internal/memory/store_extra_test.go`、`internal/memory/writedoc_test.go`
- **需同步检查：** `internal/memory/store.go`、`internal/memory/store_test.go`、`internal/memory/convention_test.go`、`internal/memory/forget_test.go`、`internal/memory/memory_test.go`

#### 待修改项

- [ ] 从 L149、L157、L160、L171、L180 的深层分支开始，识别可提前返回的错误/空状态分支，改为 guard clause。
- [ ] 对无法提前返回的循环或状态分支，提取完整子流程并显式传递所需状态，保持 break/continue/return 语义。
- [ ] 与同一治理对象的 CX-296、ND-013 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `nesting_depth|internal/memory/store.go|Store.Index` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/memory/store.go:147-185` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `Store.Index` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G43 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-296`、`ND-013` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/memory/store_test.go`。
- [ ] 运行相关包测试：`go test ./internal/memory`。
- [ ] 运行相关包 vet：`go vet ./internal/memory`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/memory/store.go`。
- [ ] 运行全量 Code Guard，并确认 `nesting_depth|internal/memory/store.go|Store.Index` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-278 · 圈复杂度 · `sanitizeName`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-278 |
| Baseline key | `complexity\|internal/installsource/names.go\|sanitizeName` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 39） |
| 文件 | `internal/installsource/names.go` |
| 代码行 | 158 |
| 函数/方法/包 | `sanitizeName` |
| Baseline / 当前复测 / 阈值 | 16 / 16 / 10 |
| 超出 | +6，1.60x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`sanitizeName` 的圈复杂度当前为 **16**，超过限制 **10**（+6，1.60x）。定义范围为 158-188 行；可见控制流热点：L164 `for _, r := range s {`；L166 `if !ok {`；L167 `if !prevDash {`；L177 `if out == "" {`；L180 `if !((out[0] >= 'a' && out[0] <= 'z') || (out[0] >= '0' && out[0] <= '9')) {`；L183 `if len(out) > 64 {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function sanitizeName（internal/installsource/names.go:158-188）
- **可信生产调用方（4）：** `mcpNameFromURL`（`internal/installsource/names.go:119`，调用点 L128/L142/L144/L149，confidence=0.90，exact-match，5 条边）；`installSourceTool::localExecutableMCPAction`（`internal/installsource/mcp.go:101`，调用点 L104，confidence=0.90，exact-match）；`installSourceTool::packageMCPAction`（`internal/installsource/mcp.go:123`，调用点 L126，confidence=0.90，exact-match）；`nameFromURL`（`internal/installsource/names.go:104`，调用点 L113，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `installSourceTool::plan`（`internal/installsource/plan.go:20`，调用点 L30，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/installsource`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/installsource/install_source_test.go`
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `internal/installsource/names.go`、`internal/installsource/mcp.go`、`internal/installsource/install_source_test.go`

#### 待修改项

- [ ] 逐段核对 L164、L166、L177、L180、L183 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `sanitizeName` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/installsource/names.go|sanitizeName` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/installsource/names.go:158-188` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `sanitizeName` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G43 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/installsource/install_source_test.go`。
- [ ] 运行相关包测试：`go test ./internal/installsource`。
- [ ] 运行相关包 vet：`go vet ./internal/installsource`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/installsource/names.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/installsource/names.go|sanitizeName` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-273 · 圈复杂度 · `*installSourceTool.applyInstallMCP`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-273 |
| Baseline key | `complexity\|internal/installsource/apply.go\|*installSourceTool.applyInstallMCP` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 38） |
| 文件 | `internal/installsource/apply.go` |
| 代码行 | 195 |
| 函数/方法/包 | `*installSourceTool.applyInstallMCP` |
| Baseline / 当前复测 / 阈值 | 18 / 18 / 10 |
| 超出 | +8，1.80x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*installSourceTool.applyInstallMCP` 的圈复杂度当前为 **18**，超过限制 **10**（+8，1.80x）。定义范围为 195-249 行；可见控制流热点：L196 `if act.entry.Name == "" {`；L209 `if !req.Replace {`；L217 `if req.Replace && hadPrevious && t.onDisconnect != nil {`；L223 `if oldDisconnected {`；L236 `if err := cfg.UpsertPlugin(act.entry); err != nil {`；L243 `if rbErr := t.rollbackMCPReplace(act, previous, oldDisconnected, connected); rbErr != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（Config::UpsertPlugin、Config::SaveTo）、持久化/I-O（Config::SaveTo、installSourceTool::restoreMCP）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method installSourceTool::applyInstallMCP（internal/installsource/apply.go:195-249）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `installSourceTool::apply`（`internal/installsource/apply.go:18`，调用点 L38，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（1）：** `newErr`（`internal/installsource/errors.go:70`，调用点 L197/L211，confidence=0.90，exact-match，2 条边）
- **待源码复核被调候选（4）：** `installSourceTool::rollbackMCPReplace`（`internal/installsource/apply.go:251`，调用点 L237/L243，confidence=0.70，instance-method，2 条边，待源码复核）；`Config::UpsertPlugin`（`internal/config/edit.go:694`，调用点 L236，confidence=0.70，instance-method，待源码复核）；`Config::SaveTo`（`internal/config/edit.go:809`，调用点 L242，confidence=0.70，instance-method，待源码复核）；`installSourceTool::restoreMCP`（`internal/installsource/apply.go:262`，调用点 L224，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `installSourceTool::apply`（`internal/installsource/apply.go:18`，调用点 L38，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/installsource`、`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/installsource/install_source_test.go`
- **需同步检查：** `internal/installsource/apply.go`、`internal/installsource/errors.go`、`internal/config/edit.go`、`internal/installsource/install_source_test.go`

#### 待修改项

- [ ] 逐段核对 L196、L209、L222、L236、L243 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*installSourceTool.applyInstallMCP` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/installsource/apply.go|*installSourceTool.applyInstallMCP` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/installsource/apply.go:195-249` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*installSourceTool.applyInstallMCP` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G43 处理。
- 重点保护：持久化、迁移或文件格式兼容；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/installsource/install_source_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/installsource`。
- [ ] 运行相关包 vet：`go vet ./internal/installsource`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/installsource/apply.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/installsource/apply.go|*installSourceTool.applyInstallMCP` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-300 · 圈复杂度 · `Store.ListArchived`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-300 |
| Baseline key | `complexity\|internal/memory/store.go\|Store.ListArchived` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 37） |
| 文件 | `internal/memory/store.go` |
| 代码行 | 601 |
| 函数/方法/包 | `Store.ListArchived` |
| Baseline / 当前复测 / 阈值 | 15 / 15 / 10 |
| 超出 | +5，1.50x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`Store.ListArchived` 的圈复杂度当前为 **15**，超过限制 **10**（+5，1.50x）。定义范围为 601-646 行；可见控制流热点：L602 `if s.Dir == "" && s.GlobalDir == "" {`；L607 `if base == "" {`；L615 `for _, e := range entries {`；L624 `if !s.visibleFromDir(base, m.Type) {`；L629 `if info, err := e.Info(); err == nil {`；L640 `if out[i].Name != out[j].Name {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（loadMemory）、持久化/I-O（archiveTimeFromName、Store::dirs、Store::visibleFromDir）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method Store::ListArchived（internal/memory/store.go:601-646）
- **可信生产调用方（1）：** `App::memoryForCtrl`（`desktop/app.go:7193`，调用点 L7221，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `archiveTimeFromName`（`internal/memory/store.go:658`，调用点 L627，confidence=0.90，exact-match）；`loadMemory`（`internal/memory/store.go:673`，调用点 L620，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `Store::dirs`（`internal/memory/store.go:136`，调用点 L606，confidence=0.70，instance-method，待源码复核）；`Store::visibleFromDir`（`internal/memory/store.go:648`，调用点 L624，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `App::Memory`（`desktop/app.go:7176`，调用点 L7177，confidence=0.70，instance-method，待源码复核）；`App::MemoryForTab`（`desktop/app.go:7186`，调用点 L7188，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 14 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/memory`、`desktop`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/memory/store_test.go`
- **同目录测试候选：** `internal/memory/convention_test.go`、`internal/memory/forget_test.go`、`internal/memory/memory_test.go`、`internal/memory/quickadd_test.go`、`internal/memory/recall_test.go`、`internal/memory/remember_test.go`、`internal/memory/store_extra_test.go`、`internal/memory/writedoc_test.go`
- **需同步检查：** `internal/memory/store.go`、`desktop/app.go`、`internal/memory/store_test.go`、`internal/memory/convention_test.go`、`internal/memory/forget_test.go`、`internal/memory/memory_test.go`

#### 待修改项

- [ ] 逐段核对 L602、L612、L621、L628、L640 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `Store.ListArchived` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/memory/store.go|Store.ListArchived` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/memory/store.go:601-646` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `Store.ListArchived` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G43 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/memory/store_test.go`。
- [ ] 运行相关包测试：`go test ./internal/memory`。
- [ ] 运行相关包 vet：`go vet ./internal/memory`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/memory/store.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/memory/store.go|Store.ListArchived` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-272 · 圈复杂度 · `*installSourceTool.applySkillRoot`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-272 |
| Baseline key | `complexity\|internal/installsource/apply.go\|*installSourceTool.applySkillRoot` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 34） |
| 文件 | `internal/installsource/apply.go` |
| 代码行 | 51 |
| 函数/方法/包 | `*installSourceTool.applySkillRoot` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*installSourceTool.applySkillRoot` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 51-83 行；可见控制流热点：L53 `if err := cfg.AddSkillPath(act.Source); err != nil {`；L60 `for _, name := range act.Skills {`；L62 `if !ok {`；L69 `if strings.TrimSpace(sk.Description) == "" {`；L73 `for _, listed := range store.List() {`；L75 `if listed.Name == name {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（Config::SaveTo）、持久化/I-O（Store::List、Config::SaveTo、Store::Read）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method installSourceTool::applySkillRoot（internal/installsource/apply.go:51-83）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `installSourceTool::apply`（`internal/installsource/apply.go:18`，调用点 L23，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（2）：** `newErr`（`internal/installsource/errors.go:70`，调用点 L63，confidence=0.90，exact-match）；`New`（`internal/skill/skill.go:107`，调用点 L59，confidence=0.90，import）
- **待源码复核被调候选（3）：** `Store::List`（`internal/checkpoint/checkpoint.go:205`，调用点 L73，confidence=0.80，instance-method，待源码复核）；`Config::SaveTo`（`internal/config/edit.go:809`，调用点 L56，confidence=0.70，instance-method，待源码复核）；`Store::Read`（`internal/skill/skill.go:324`，调用点 L61，confidence=0.80，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `installSourceTool::apply`（`internal/installsource/apply.go:18`，调用点 L23，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 5 个文件；`internal/installsource`、`internal/skill`、`internal/checkpoint`、`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/installsource/install_source_test.go`
- **需同步检查：** `internal/installsource/apply.go`、`internal/installsource/errors.go`、`internal/skill/skill.go`、`internal/checkpoint/checkpoint.go`、`internal/config/edit.go`、`internal/installsource/install_source_test.go`

#### 待修改项

- [ ] 逐段核对 L53、L60、L66、L73、L75 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*installSourceTool.applySkillRoot` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/installsource/apply.go|*installSourceTool.applySkillRoot` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/installsource/apply.go:51-83` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*installSourceTool.applySkillRoot` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G43 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/installsource/install_source_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/installsource`。
- [ ] 运行相关包 vet：`go vet ./internal/installsource`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/installsource/apply.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/installsource/apply.go|*installSourceTool.applySkillRoot` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-276 · 圈复杂度 · `validateMCPEntry`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-276 |
| Baseline key | `complexity\|internal/installsource/mcp.go\|validateMCPEntry` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 34） |
| 文件 | `internal/installsource/mcp.go` |
| 代码行 | 243 |
| 函数/方法/包 | `validateMCPEntry` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`validateMCPEntry` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 243-271 行；可见控制流热点：L245 `if name == "" {`；L248 `if !config.IsValidSkillName(name) {`；L258 `switch pluginTransport(e) {`；L259 `case "stdio":`；L263 `case "http", "sse":`；L264 `if strings.TrimSpace(e.URL) == "" {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function validateMCPEntry（internal/installsource/mcp.go:243-271）
- **可信生产调用方（1）：** `parseMCPJSON`（`internal/installsource/mcp.go:165`，调用点 L233，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `newErr`（`internal/installsource/errors.go:70`，调用点 L246/L249/L256/L261，confidence=0.90，exact-match，6 条边）；`pluginTransport`（`internal/installsource/names.go:276`，调用点 L258，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/installsource`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/installsource/install_source_test.go`
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `internal/installsource/mcp.go`、`internal/installsource/errors.go`、`internal/installsource/names.go`、`internal/installsource/install_source_test.go`

#### 待修改项

- [ ] 逐段核对 L245、L255、L259、L260、L264 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `validateMCPEntry` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/installsource/mcp.go|validateMCPEntry` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/installsource/mcp.go:243-271` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `validateMCPEntry` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G43 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/installsource/install_source_test.go`。
- [ ] 运行相关包测试：`go test ./internal/installsource`。
- [ ] 运行相关包 vet：`go vet ./internal/installsource`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/installsource/mcp.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/installsource/mcp.go|validateMCPEntry` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-277 · 圈复杂度 · `mcpNameFromURL`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-277 |
| Baseline key | `complexity\|internal/installsource/names.go\|mcpNameFromURL` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 34） |
| 文件 | `internal/installsource/names.go` |
| 代码行 | 119 |
| 函数/方法/包 | `mcpNameFromURL` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`mcpNameFromURL` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 119-153 行；可见控制流热点：L121 `if err != nil || u.Hostname() == "" {`；L125 `if host == "localhost" || host == "127.0.0.1" || host == "::1" {`；L133 `for _, p := range []string{"mcp.", "api.", "www."} {`；L140 `switch len(parts) {`；L143 `case 2:`；L148 `if len(parts[len(parts)-1]) == 2 {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function mcpNameFromURL（internal/installsource/names.go:119-153）
- **可信生产调用方（1）：** `installSourceTool::remoteMCPAction`（`internal/installsource/mcp.go:76`，调用点 L86，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `sanitizeName`（`internal/installsource/names.go:158`，调用点 L128/L142/L144/L149，confidence=0.90，exact-match，5 条边）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/installsource`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/installsource/install_source_test.go`
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `internal/installsource/names.go`、`internal/installsource/mcp.go`、`internal/installsource/install_source_test.go`

#### 待修改项

- [ ] 逐段核对 L121、L127、L140、L141、L148 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `mcpNameFromURL` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/installsource/names.go|mcpNameFromURL` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/installsource/names.go:119-153` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `mcpNameFromURL` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G43 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/installsource/install_source_test.go`。
- [ ] 运行相关包测试：`go test ./internal/installsource`。
- [ ] 运行相关包 vet：`go vet ./internal/installsource`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/installsource/names.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/installsource/names.go|mcpNameFromURL` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-286 · 圈复杂度 · `copyDir`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-286 |
| Baseline key | `complexity\|internal/installsource/skill.go\|copyDir` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 32） |
| 文件 | `internal/installsource/skill.go` |
| 代码行 | 301 |
| 函数/方法/包 | `copyDir` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`copyDir` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 301-345 行；可见控制流热点：L304 `if err != nil {`；L312 `if d.IsDir() {`；L318 `if !d.Type().IsRegular() {`；L322 `if err != nil {`；L330 `if err != nil {`；L338 `if err != nil {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function copyDir（internal/installsource/skill.go:301-345）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `installSourceTool::applyCopySkill`（`internal/installsource/apply.go:89`，调用点 L105，confidence=0.70，exact-match，待源码复核）
- **可信被调用函数（1）：** `newErr`（`internal/installsource/errors.go:70`，调用点 L327，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `installSourceTool::apply`（`internal/installsource/apply.go:18`，调用点 L25，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/installsource`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/installsource/install_source_test.go`
- **需同步检查：** `internal/installsource/skill.go`、`internal/installsource/apply.go`、`internal/installsource/errors.go`、`internal/installsource/install_source_test.go`

#### 待修改项

- [ ] 逐段核对 L304、L312、L322、L330、L338 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `copyDir` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/installsource/skill.go|copyDir` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/installsource/skill.go:301-345` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `copyDir` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G43 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/installsource/install_source_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/installsource`。
- [ ] 运行相关包 vet：`go vet ./internal/installsource`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/installsource/skill.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/installsource/skill.go|copyDir` 下降或消失且无新增/恶化。
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

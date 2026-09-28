# G23 · internal/installsource · plan 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G23 |
| 问题数量 | 10 |
| 整体优先级 | P2 |
| 涉及模块 | `internal/installsource` |
| 涉及文件 | `internal/installsource/plan.go`<br>`internal/installsource/install_source.go`<br>`internal/installsource/mcp.go` |
| 主要治理目标 | 围绕 `internal/installsource/plan.go` 等治理对象降低复杂度/体量，统一处理 2 个多规则函数对象，并保持调用链和行为边界。 |
| 推荐负责人范围 | installsource 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 中：认领前确认同包开发 |
| 冲突依据 | 单文件最多集中 7 条问题 |
| 建议处理顺序 | 总序第 23 组；先于 G37 处理 |
| 前置依赖 | 无同文件前置组；开始前仍需同步最新 main-v3 |
| 后续验证重点 | 协议、序列化或流式事件兼容；返回值、错误语义和现有调用顺序；配置、平台或向后兼容 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **47/100**，对象 `*installSourceTool.localSkillActions` 的分项为严重度 10.2、集中度 11.8、影响面 8.4、可执行性 16.0；最高超限 2.30x，文件内 7 条问题，CodeGraph 记录 0 个可信生产调用方、1 个待核候选。该对象同时命中 圈复杂度、函数有效行数，必须合并治理。

### 治理边界与组间关系

本组涉及 3 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。没有同文件前置组。后续同文件组：G37，应串行衔接。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-284` | `complexity\|internal/installsource/plan.go\|*installSourceTool.localSkillActions` | 圈复杂度 | `internal/installsource/plan.go` | `*installSourceTool.localSkillActions` | 344 | 23/10 | +13（2.30x） | P2 | 0 可信 + 1 待核 caller / 4 可信 + 3 待核 callee / 0 unresolved | 待修改 |
| `FN-040` | `func_lines\|internal/installsource/plan.go\|*installSourceTool.localSkillActions` | 函数有效行数 | `internal/installsource/plan.go` | `*installSourceTool.localSkillActions` | 344 | 86/80 | +6（1.07x） | P2 | 0 可信 + 1 待核 caller / 4 可信 + 3 待核 callee / 0 unresolved | 待修改 |
| `CX-274` | `complexity\|internal/installsource/install_source.go\|*installSourceTool.Execute` | 圈复杂度 | `internal/installsource/install_source.go` | `*installSourceTool.Execute` | 152 | 19/10 | +9（1.90x） | P2 | 0 可信 + 0 待核 caller / 8 可信 + 6 待核 callee / 0 unresolved | 待修改 |
| `FN-039` | `func_lines\|internal/installsource/install_source.go\|*installSourceTool.Execute` | 函数有效行数 | `internal/installsource/install_source.go` | `*installSourceTool.Execute` | 152 | 93/80 | +13（1.16x） | P2 | 0 可信 + 0 待核 caller / 8 可信 + 6 待核 callee / 0 unresolved | 待修改 |
| `CX-275` | `complexity\|internal/installsource/mcp.go\|parseMCPJSON` | 圈复杂度 | `internal/installsource/mcp.go` | `parseMCPJSON` | 165 | 17/10 | +7（1.70x） | P3 | 2 可信 + 0 待核 caller / 5 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-283` | `complexity\|internal/installsource/plan.go\|*installSourceTool.planLocal` | 圈复杂度 | `internal/installsource/plan.go` | `*installSourceTool.planLocal` | 299 | 20/10 | +10（2.00x） | P3 | 0 可信 + 1 待核 caller / 3 可信 + 3 待核 callee / 0 unresolved | 待修改 |
| `CX-281` | `complexity\|internal/installsource/plan.go\|*installSourceTool.tryGitHubRepo` | 圈复杂度 | `internal/installsource/plan.go` | `*installSourceTool.tryGitHubRepo` | 91 | 16/10 | +6（1.60x） | P3 | 0 可信 + 1 待核 caller / 2 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-279` | `complexity\|internal/installsource/plan.go\|*installSourceTool.planURL` | 圈复杂度 | `internal/installsource/plan.go` | `*installSourceTool.planURL` | 36 | 14/10 | +4（1.40x） | P3 | 0 可信 + 1 待核 caller / 5 可信 + 3 待核 callee / 0 unresolved | 待修改 |
| `CX-280` | `complexity\|internal/installsource/plan.go\|*installSourceTool.planDownloadedURL` | 圈复杂度 | `internal/installsource/plan.go` | `*installSourceTool.planDownloadedURL` | 62 | 11/10 | +1（1.10x） | P3 | 0 可信 + 2 待核 caller / 4 可信 + 4 待核 callee / 0 unresolved | 待修改 |
| `CX-282` | `complexity\|internal/installsource/plan.go\|*installSourceTool.scanGitHubSkills` | 圈复杂度 | `internal/installsource/plan.go` | `*installSourceTool.scanGitHubSkills` | 181 | 11/10 | +1（1.10x） | P3 | 0 可信 + 1 待核 caller / 2 可信 + 2 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-284 · 圈复杂度 · `*installSourceTool.localSkillActions`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-284 |
| Baseline key | `complexity\|internal/installsource/plan.go\|*installSourceTool.localSkillActions` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 47） |
| 文件 | `internal/installsource/plan.go` |
| 代码行 | 344 |
| 函数/方法/包 | `*installSourceTool.localSkillActions` |
| Baseline / 当前复测 / 阈值 | 23 / 23 / 10 |
| 超出 | +13，2.30x |
| 同一治理对象关联问题 | `FN-040` |

#### 明确的问题描述

`*installSourceTool.localSkillActions` 的圈复杂度当前为 **23**，超过限制 **10**（+13，2.30x）。定义范围为 344-431 行；可见控制流热点：L346 `if !info.IsDir() {`；L358 `if strings.EqualFold(filepath.Base(path), skill.SkillFile) {`；L377 `if st, err := os.Stat(filepath.Join(path, skill.SkillFile)); err == nil && st.Mode().IsRegular() {`；L394 `if err != nil {`；L406 `for _, cand := range cands {`；L427 `for _, cand := range cands {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（readSkillFile）、网络/协议（request::strict）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method installSourceTool::localSkillActions（internal/installsource/plan.go:344-431）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `installSourceTool::planLocal`（`internal/installsource/plan.go:299`，调用点 L323，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（4）：** `newErr`（`internal/installsource/errors.go:70`，调用点 L348/L398，confidence=0.90，exact-match，2 条边）；`modeForSingleSkill`（`internal/installsource/names.go:238`，调用点 L375/L391，confidence=0.90，exact-match，2 条边）；`readSkillFile`（`internal/installsource/skill.go:172`，调用点 L354/L378，confidence=0.90，exact-match，2 条边）；`scanSkillRoot`（`internal/installsource/skill.go:216`，调用点 L393，confidence=0.90，exact-match）
- **待源码复核被调候选（3）：** `installSourceTool::skillAction`（`internal/installsource/skill.go:25`，调用点 L375/L391/L428，confidence=0.70，instance-method，3 条边，待源码复核）；`installSourceTool::skillRootAction`（`internal/installsource/skill.go:79`，调用点 L373/L389/L422，confidence=0.70，instance-method，3 条边，待源码复核）；`request::strict`（`internal/installsource/plan.go:435`，调用点 L345，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `installSourceTool::plan`（`internal/installsource/plan.go:20`，调用点 L26，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/installsource`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/installsource/install_source_test.go`
- **需同步检查：** `internal/installsource/plan.go`、`internal/installsource/errors.go`、`internal/installsource/names.go`、`internal/installsource/skill.go`、`internal/installsource/install_source_test.go`

#### 待修改项

- [ ] 逐段核对 L346、L365、L385、L404、L427 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O、网络/协议 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*installSourceTool.localSkillActions` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-284、FN-040 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/installsource/plan.go|*installSourceTool.localSkillActions` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/installsource/plan.go:344-431` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*installSourceTool.localSkillActions` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G37 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-284`、`FN-040` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/installsource/install_source_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/installsource`。
- [ ] 运行相关包 vet：`go vet ./internal/installsource`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/installsource/plan.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/installsource/plan.go|*installSourceTool.localSkillActions` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-040 · 函数有效行数 · `*installSourceTool.localSkillActions`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-040 |
| Baseline key | `func_lines\|internal/installsource/plan.go\|*installSourceTool.localSkillActions` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 47） |
| 文件 | `internal/installsource/plan.go` |
| 代码行 | 344 |
| 函数/方法/包 | `*installSourceTool.localSkillActions` |
| Baseline / 当前复测 / 阈值 | 86 / 86 / 80 |
| 超出 | +6，1.07x |
| 同一治理对象关联问题 | `CX-284` |

#### 明确的问题描述

`*installSourceTool.localSkillActions` 的有效代码行 **86**，超过限制 **80**（+6）。CodeGraph 定义物理范围为 344-431 行，并记录 7 个直接下游候选；候选职责分布为 持久化/I-O（readSkillFile）、网络/协议（request::strict）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method installSourceTool::localSkillActions（internal/installsource/plan.go:344-431）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `installSourceTool::planLocal`（`internal/installsource/plan.go:299`，调用点 L323，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（4）：** `newErr`（`internal/installsource/errors.go:70`，调用点 L348/L398，confidence=0.90，exact-match，2 条边）；`modeForSingleSkill`（`internal/installsource/names.go:238`，调用点 L375/L391，confidence=0.90，exact-match，2 条边）；`readSkillFile`（`internal/installsource/skill.go:172`，调用点 L354/L378，confidence=0.90，exact-match，2 条边）；`scanSkillRoot`（`internal/installsource/skill.go:216`，调用点 L393，confidence=0.90，exact-match）
- **待源码复核被调候选（3）：** `installSourceTool::skillAction`（`internal/installsource/skill.go:25`，调用点 L375/L391/L428，confidence=0.70，instance-method，3 条边，待源码复核）；`installSourceTool::skillRootAction`（`internal/installsource/skill.go:79`，调用点 L373/L389/L422，confidence=0.70，instance-method，3 条边，待源码复核）；`request::strict`（`internal/installsource/plan.go:435`，调用点 L345，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `installSourceTool::plan`（`internal/installsource/plan.go:20`，调用点 L26，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/installsource`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/installsource/install_source_test.go`
- **需同步检查：** `internal/installsource/plan.go`、`internal/installsource/errors.go`、`internal/installsource/names.go`、`internal/installsource/skill.go`、`internal/installsource/install_source_test.go`

#### 待修改项

- [ ] 把 `*installSourceTool.localSkillActions` 的 86 个有效行按 持久化/I-O、网络/协议 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-284、FN-040 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/installsource/plan.go|*installSourceTool.localSkillActions` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/installsource/plan.go:344-431` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*installSourceTool.localSkillActions` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G37 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-284`、`FN-040` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/installsource/install_source_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/installsource`。
- [ ] 运行相关包 vet：`go vet ./internal/installsource`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/installsource/plan.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/installsource/plan.go|*installSourceTool.localSkillActions` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-274 · 圈复杂度 · `*installSourceTool.Execute`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-274 |
| Baseline key | `complexity\|internal/installsource/install_source.go\|*installSourceTool.Execute` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P2，治理对象得分 45） |
| 文件 | `internal/installsource/install_source.go` |
| 代码行 | 152 |
| 函数/方法/包 | `*installSourceTool.Execute` |
| Baseline / 当前复测 / 阈值 | 19 / 19 / 10 |
| 超出 | +9，1.90x |
| 同一治理对象关联问题 | `FN-039` |

#### 明确的问题描述

`*installSourceTool.Execute` 的圈复杂度当前为 **19**，超过限制 **10**（+9，1.90x）。定义范围为 152-251 行；可见控制流热点：L154 `if err := json.Unmarshal(raw, &req); err != nil {`；L164 `if req.Op == "install" && req.Source == "" {`；L174 `if norm, ok := normalizeTier(req.Tier); ok {`；L187 `if len(actions) == 0 {`；L205 `for i := range actions {`；L231 `if err := t.approval(publicActions(actions)); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（normalizeTier、installSourceTool::normalizeScope、normalizeKind、normalizeMode）、状态与控制（installSourceTool::executeApply、installSourceTool::executeUninstall）、渲染/输出（marshalJSON）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method installSourceTool::Execute（internal/installsource/install_source.go:152-251）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（8）：** `publicActions`（`internal/installsource/types.go:140`，调用点 L220/L231/L243，confidence=0.90，exact-match，3 条边）；`marshalJSON`（`internal/installsource/types.go:150`，调用点 L201/L224/L232，confidence=0.90，exact-match，3 条边）；`kindCounts`（`internal/installsource/install_source.go:631`，调用点 L216/L239，confidence=0.90，exact-match，2 条边）；`summarizeKind`（`internal/installsource/types.go:124`，调用点 L215/L238，confidence=0.90，exact-match，2 条边）；`newErr`（`internal/installsource/errors.go:70`，调用点 L228，confidence=0.90，exact-match）；`commonActionScope`（`internal/installsource/install_source.go:550`，调用点 L208，confidence=0.90，exact-match）；`computePlanID`（`internal/installsource/install_source.go:584`，调用点 L186，confidence=0.90，exact-match）；`normalizeTier`（`internal/installsource/names.go:261`，调用点 L174，confidence=0.90，exact-match）
- **待源码复核被调候选（6）：** `installSourceTool::executeApply`（`internal/installsource/install_source.go:256`，调用点 L250，confidence=0.70，instance-method，待源码复核）；`installSourceTool::executeUninstall`（`internal/installsource/install_source.go:306`，调用点 L179，confidence=0.70，instance-method，待源码复核）；`installSourceTool::normalizeScope`（`internal/installsource/install_source.go:503`，调用点 L171，confidence=0.70，instance-method，待源码复核）；`normalizeKind`（`internal/installsource/names.go:220`，调用点 L170，confidence=0.70，exact-match，待源码复核）；`normalizeMode`（`internal/installsource/names.go:229`，调用点 L172，confidence=0.70，exact-match，待源码复核）；`normalizeTransport`（`internal/installsource/names.go:245`，调用点 L173，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/installsource`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/installsource/install_source_test.go`
- **需同步检查：** `internal/installsource/install_source.go`、`internal/installsource/types.go`、`internal/installsource/install_source_test.go`

#### 待修改项

- [ ] 逐段核对 L154、L164、L183、L205、L231 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*installSourceTool.Execute` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-274、FN-039 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/installsource/install_source.go|*installSourceTool.Execute` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/installsource/install_source.go:152-251` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*installSourceTool.Execute` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G37 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-274`、`FN-039` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/installsource/install_source_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/installsource`。
- [ ] 运行相关包 vet：`go vet ./internal/installsource`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/installsource/install_source.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/installsource/install_source.go|*installSourceTool.Execute` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-039 · 函数有效行数 · `*installSourceTool.Execute`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-039 |
| Baseline key | `func_lines\|internal/installsource/install_source.go\|*installSourceTool.Execute` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 45） |
| 文件 | `internal/installsource/install_source.go` |
| 代码行 | 152 |
| 函数/方法/包 | `*installSourceTool.Execute` |
| Baseline / 当前复测 / 阈值 | 93 / 93 / 80 |
| 超出 | +13，1.16x |
| 同一治理对象关联问题 | `CX-274` |

#### 明确的问题描述

`*installSourceTool.Execute` 的有效代码行 **93**，超过限制 **80**（+13）。CodeGraph 定义物理范围为 152-251 行，并记录 14 个直接下游候选；候选职责分布为 配置/解析（normalizeTier、installSourceTool::normalizeScope、normalizeKind、normalizeMode）、状态与控制（installSourceTool::executeApply、installSourceTool::executeUninstall）、渲染/输出（marshalJSON）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method installSourceTool::Execute（internal/installsource/install_source.go:152-251）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（8）：** `publicActions`（`internal/installsource/types.go:140`，调用点 L220/L231/L243，confidence=0.90，exact-match，3 条边）；`marshalJSON`（`internal/installsource/types.go:150`，调用点 L201/L224/L232，confidence=0.90，exact-match，3 条边）；`kindCounts`（`internal/installsource/install_source.go:631`，调用点 L216/L239，confidence=0.90，exact-match，2 条边）；`summarizeKind`（`internal/installsource/types.go:124`，调用点 L215/L238，confidence=0.90，exact-match，2 条边）；`newErr`（`internal/installsource/errors.go:70`，调用点 L228，confidence=0.90，exact-match）；`commonActionScope`（`internal/installsource/install_source.go:550`，调用点 L208，confidence=0.90，exact-match）；`computePlanID`（`internal/installsource/install_source.go:584`，调用点 L186，confidence=0.90，exact-match）；`normalizeTier`（`internal/installsource/names.go:261`，调用点 L174，confidence=0.90，exact-match）
- **待源码复核被调候选（6）：** `installSourceTool::executeApply`（`internal/installsource/install_source.go:256`，调用点 L250，confidence=0.70，instance-method，待源码复核）；`installSourceTool::executeUninstall`（`internal/installsource/install_source.go:306`，调用点 L179，confidence=0.70，instance-method，待源码复核）；`installSourceTool::normalizeScope`（`internal/installsource/install_source.go:503`，调用点 L171，confidence=0.70，instance-method，待源码复核）；`normalizeKind`（`internal/installsource/names.go:220`，调用点 L170，confidence=0.70，exact-match，待源码复核）；`normalizeMode`（`internal/installsource/names.go:229`，调用点 L172，confidence=0.70，exact-match，待源码复核）；`normalizeTransport`（`internal/installsource/names.go:245`，调用点 L173，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/installsource`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/installsource/install_source_test.go`
- **需同步检查：** `internal/installsource/install_source.go`、`internal/installsource/types.go`、`internal/installsource/install_source_test.go`

#### 待修改项

- [ ] 把 `*installSourceTool.Execute` 的 93 个有效行按 配置/解析、状态与控制、渲染/输出 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-274、FN-039 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/installsource/install_source.go|*installSourceTool.Execute` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/installsource/install_source.go:152-251` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*installSourceTool.Execute` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G37 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-274`、`FN-039` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/installsource/install_source_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/installsource`。
- [ ] 运行相关包 vet：`go vet ./internal/installsource`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/installsource/install_source.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/installsource/install_source.go|*installSourceTool.Execute` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-275 · 圈复杂度 · `parseMCPJSON`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-275 |
| Baseline key | `complexity\|internal/installsource/mcp.go\|parseMCPJSON` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 41） |
| 文件 | `internal/installsource/mcp.go` |
| 代码行 | 165 |
| 函数/方法/包 | `parseMCPJSON` |
| Baseline / 当前复测 / 阈值 | 17 / 17 / 10 |
| 超出 | +7，1.70x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`parseMCPJSON` 的圈复杂度当前为 **17**，超过限制 **10**（+7，1.70x）。定义范围为 165-239 行；可见控制流热点：L178 `if err := json.Unmarshal(b, &raw); err != nil {`；L185 `for name := range raw.MCPServers {`；L196 `if rawType != "" && rawType != "stdio" && rawType != "http" && rawType != "sse" && rawType != "stre…`；L201 `if strings.TrimSpace(s.URL) != "" {`；L225 `if e.Type == "stdio" {`；L233 `if err := validateMCPEntry(e); err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（NormalizePluginCommandLine、validateMCPEntry、normalizeTier、normalizeTransport）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function parseMCPJSON（internal/installsource/mcp.go:165-239）
- **可信生产调用方（2）：** `readMCPJSON`（`internal/installsource/mcp.go:150`，调用点 L155，confidence=0.90，exact-match）；`installSourceTool::planDownloadedURL`（`internal/installsource/plan.go:62`，调用点 L68，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（5）：** `newErr`（`internal/installsource/errors.go:70`，调用点 L179/L182/L197，confidence=0.90，exact-match，3 条边）；`cleanMap`（`internal/installsource/names.go:192`，调用点 L216/L218，confidence=0.90，exact-match，2 条边）；`NormalizePluginCommandLine`（`internal/config/mcp_command.go:14`，调用点 L228，confidence=0.90，import）；`validateMCPEntry`（`internal/installsource/mcp.go:243`，调用点 L233，confidence=0.90，exact-match）；`normalizeTier`（`internal/installsource/names.go:261`，调用点 L207，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `normalizeTransport`（`internal/installsource/names.go:245`，调用点 L199，confidence=0.70，exact-match，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 5 个文件；`internal/installsource`、`internal/config`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/installsource/install_source_test.go`
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `internal/installsource/mcp.go`、`internal/installsource/plan.go`、`internal/installsource/errors.go`、`internal/installsource/names.go`、`internal/config/mcp_command.go`、`internal/installsource/install_source_test.go`

#### 待修改项

- [ ] 逐段核对 L178、L191、L200、L225、L233 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `parseMCPJSON` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/installsource/mcp.go|parseMCPJSON` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/installsource/mcp.go:165-239` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `parseMCPJSON` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G37 处理。
- 重点保护：协议、序列化或流式事件兼容；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/installsource/install_source_test.go`。
- [ ] 运行相关包测试：`go test ./internal/installsource`。
- [ ] 运行相关包 vet：`go vet ./internal/installsource`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/installsource/mcp.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/installsource/mcp.go|parseMCPJSON` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-283 · 圈复杂度 · `*installSourceTool.planLocal`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-283 |
| Baseline key | `complexity\|internal/installsource/plan.go\|*installSourceTool.planLocal` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P1，治理对象得分 40） |
| 文件 | `internal/installsource/plan.go` |
| 代码行 | 299 |
| 函数/方法/包 | `*installSourceTool.planLocal` |
| Baseline / 当前复测 / 阈值 | 20 / 20 / 10 |
| 超出 | +10，2.00x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*installSourceTool.planLocal` 的圈复杂度当前为 **20**，超过限制 **10**（+10，2.00x）。定义范围为 299-342 行；可见控制流热点：L302 `if req.Kind == "auto" || req.Kind == "mcp" {`；L307 `if filepath.Base(mcpPath) == ".mcp.json" {`；L310 `for _, e := range entries {`；L322 `if req.Kind == "auto" || req.Kind == "skill" {`；L327 `if err != nil && req.Kind == "auto" {`；L336 `if actions[i].Kind != actions[j].Kind {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（readMCPJSON）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method installSourceTool::planLocal（internal/installsource/plan.go:299-342）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `installSourceTool::plan`（`internal/installsource/plan.go:20`，调用点 L26，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（3）：** `newErr`（`internal/installsource/errors.go:70`，调用点 L333，confidence=0.90，exact-match）；`readMCPJSON`（`internal/installsource/mcp.go:150`，调用点 L308，confidence=0.90，exact-match）；`isExecutable`（`internal/installsource/names.go:89`，调用点 L318，confidence=0.90，exact-match）
- **待源码复核被调候选（3）：** `installSourceTool::mcpEntryAction`（`internal/installsource/mcp.go:17`，调用点 L311，confidence=0.70，instance-method，待源码复核）；`installSourceTool::localExecutableMCPAction`（`internal/installsource/mcp.go:101`，调用点 L319，confidence=0.70，instance-method，待源码复核）；`installSourceTool::localSkillActions`（`internal/installsource/plan.go:344`，调用点 L323，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `installSourceTool::plan`（`internal/installsource/plan.go:20`，调用点 L26，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/installsource`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/installsource/install_source_test.go`
- **需同步检查：** `internal/installsource/plan.go`、`internal/installsource/errors.go`、`internal/installsource/mcp.go`、`internal/installsource/names.go`、`internal/installsource/install_source_test.go`

#### 待修改项

- [ ] 逐段核对 L302、L309、L318、L327、L336 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*installSourceTool.planLocal` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/installsource/plan.go|*installSourceTool.planLocal` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/installsource/plan.go:299-342` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*installSourceTool.planLocal` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G37 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/installsource/install_source_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/installsource`。
- [ ] 运行相关包 vet：`go vet ./internal/installsource`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/installsource/plan.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/installsource/plan.go|*installSourceTool.planLocal` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-281 · 圈复杂度 · `*installSourceTool.tryGitHubRepo`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-281 |
| Baseline key | `complexity\|internal/installsource/plan.go\|*installSourceTool.tryGitHubRepo` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 37） |
| 文件 | `internal/installsource/plan.go` |
| 代码行 | 91 |
| 函数/方法/包 | `*installSourceTool.tryGitHubRepo` |
| Baseline / 当前复测 / 阈值 | 16 / 16 / 10 |
| 超出 | +6，1.60x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*installSourceTool.tryGitHubRepo` 的圈复杂度当前为 **16**，超过限制 **10**（+6，1.60x）。定义范围为 91-123 行；可见控制流热点：L92 `if req.Kind != "auto" && req.Kind != "skill" && req.Kind != "mcp" {`；L100 `for _, branch := range src.branches() {`；L101 `if req.Kind == "auto" || req.Kind == "mcp" {`；L107 `if err != nil {`；L111 `if req.Kind == "auto" || req.Kind == "skill" {`；L117 `if err != nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（parseGitHubRepoSource、installSourceTool::planDownloadedURL）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method installSourceTool::tryGitHubRepo（internal/installsource/plan.go:91-123）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `installSourceTool::planURL`（`internal/installsource/plan.go:36`，调用点 L50，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（2）：** `parseGitHubRepoSource`（`internal/installsource/plan.go:139`，调用点 L95，confidence=0.90，exact-match）；`joinURLPath`（`internal/installsource/plan.go:288`，调用点 L102，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `installSourceTool::planDownloadedURL`（`internal/installsource/plan.go:62`，调用点 L103，confidence=0.70，instance-method，待源码复核）；`installSourceTool::planGitHubSkillRepo`（`internal/installsource/plan.go:165`，调用点 L112，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `installSourceTool::plan`（`internal/installsource/plan.go:20`，调用点 L22，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/installsource`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/installsource/install_source_test.go`
- **需同步检查：** `internal/installsource/plan.go`、`internal/installsource/install_source_test.go`

#### 待修改项

- [ ] 逐段核对 L92、L100、L104、L111、L117 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*installSourceTool.tryGitHubRepo` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/installsource/plan.go|*installSourceTool.tryGitHubRepo` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/installsource/plan.go:91-123` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*installSourceTool.tryGitHubRepo` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G37 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/installsource/install_source_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/installsource`。
- [ ] 运行相关包 vet：`go vet ./internal/installsource`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/installsource/plan.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/installsource/plan.go|*installSourceTool.tryGitHubRepo` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-279 · 圈复杂度 · `*installSourceTool.planURL`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-279 |
| Baseline key | `complexity\|internal/installsource/plan.go\|*installSourceTool.planURL` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 35） |
| 文件 | `internal/installsource/plan.go` |
| 代码行 | 36 |
| 函数/方法/包 | `*installSourceTool.planURL` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*installSourceTool.planURL` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 36-60 行；可见控制流热点：L38 `if req.Kind == "mcp" && !looksLikeMarkdownURL(rawURL) && !looksLikeMCPJSONURL(rawURL) {`；L41 `if looksLikeMarkdownURL(rawURL) || looksLikeMCPJSONURL(rawURL) || rawURL != req.Source {`；L43 `if err == nil && len(actions) > 0 {`；L50 `if gitActions, warnings := t.tryGitHubRepo(ctx, req); len(gitActions) > 0 {`；L53 `if req.Kind == "skill" {`；L56 `if req.Kind == "auto" && !looksLikeRemoteMCPEndpoint(req.Source) {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（installSourceTool::planDownloadedURL）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method installSourceTool::planURL（internal/installsource/plan.go:36-60）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `installSourceTool::plan`（`internal/installsource/plan.go:20`，调用点 L22，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（5）：** `newErr`（`internal/installsource/errors.go:70`，调用点 L54/L57，confidence=0.90，exact-match，2 条边）；`looksLikeMarkdownURL`（`internal/installsource/names.go:24`，调用点 L38/L41，confidence=0.90，exact-match，2 条边）；`looksLikeMCPJSONURL`（`internal/installsource/names.go:32`，调用点 L38/L41，confidence=0.90，exact-match，2 条边）；`looksLikeRemoteMCPEndpoint`（`internal/installsource/names.go:44`，调用点 L56，confidence=0.90，exact-match）；`rawGitHubBlobURL`（`internal/installsource/names.go:60`，调用点 L37，confidence=0.90，exact-match）
- **待源码复核被调候选（3）：** `installSourceTool::remoteMCPAction`（`internal/installsource/mcp.go:76`，调用点 L39/L59，confidence=0.70，instance-method，2 条边，待源码复核）；`installSourceTool::planDownloadedURL`（`internal/installsource/plan.go:62`，调用点 L42，confidence=0.70，instance-method，待源码复核）；`installSourceTool::tryGitHubRepo`（`internal/installsource/plan.go:91`，调用点 L50，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `installSourceTool::plan`（`internal/installsource/plan.go:20`，调用点 L22，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 4 个文件；`internal/installsource`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/installsource/install_source_test.go`
- **需同步检查：** `internal/installsource/plan.go`、`internal/installsource/errors.go`、`internal/installsource/names.go`、`internal/installsource/install_source_test.go`

#### 待修改项

- [ ] 逐段核对 L38、L43、L46、L53、L56 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*installSourceTool.planURL` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/installsource/plan.go|*installSourceTool.planURL` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/installsource/plan.go:36-60` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*installSourceTool.planURL` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G37 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/installsource/install_source_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/installsource`。
- [ ] 运行相关包 vet：`go vet ./internal/installsource`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/installsource/plan.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/installsource/plan.go|*installSourceTool.planURL` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-280 · 圈复杂度 · `*installSourceTool.planDownloadedURL`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-280 |
| Baseline key | `complexity\|internal/installsource/plan.go\|*installSourceTool.planDownloadedURL` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 34） |
| 文件 | `internal/installsource/plan.go` |
| 代码行 | 62 |
| 函数/方法/包 | `*installSourceTool.planDownloadedURL` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*installSourceTool.planDownloadedURL` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 62-89 行；可见控制流热点：L64 `if err != nil {`；L67 `if req.Kind == "auto" || req.Kind == "mcp" {`；L69 `if err == nil && len(entries) > 0 {`；L77 `if req.Kind == "auto" || req.Kind == "skill" {`；L79 `if name == "" {`；L83 `if err == nil {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（parseMCPJSON、parseSkillContent）、网络/协议（installSourceTool::fetchText、request::strict）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method installSourceTool::planDownloadedURL（internal/installsource/plan.go:62-89）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（2）：** `installSourceTool::planURL`（`internal/installsource/plan.go:36`，调用点 L42，confidence=0.70，instance-method，待源码复核）；`installSourceTool::tryGitHubRepo`（`internal/installsource/plan.go:91`，调用点 L103，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（4）：** `newErr`（`internal/installsource/errors.go:70`，调用点 L88，confidence=0.90，exact-match）；`parseMCPJSON`（`internal/installsource/mcp.go:165`，调用点 L68，confidence=0.90，exact-match）；`nameFromURL`（`internal/installsource/names.go:104`，调用点 L80，confidence=0.90，exact-match）；`parseSkillContent`（`internal/installsource/skill.go:189`，调用点 L82，confidence=0.90，exact-match）
- **待源码复核被调候选（4）：** `installSourceTool::fetchText`（`internal/installsource/http.go:25`，调用点 L63，confidence=0.70，instance-method，待源码复核）；`installSourceTool::mcpEntryAction`（`internal/installsource/mcp.go:17`，调用点 L72，confidence=0.70，instance-method，待源码复核）；`request::strict`（`internal/installsource/plan.go:435`，调用点 L82，confidence=0.70，instance-method，待源码复核）；`installSourceTool::skillAction`（`internal/installsource/skill.go:25`，调用点 L84，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `installSourceTool::plan`（`internal/installsource/plan.go:20`，调用点 L22，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 6 个文件；`internal/installsource`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/installsource/install_source_test.go`
- **需同步检查：** `internal/installsource/plan.go`、`internal/installsource/errors.go`、`internal/installsource/mcp.go`、`internal/installsource/names.go`、`internal/installsource/skill.go`、`internal/installsource/install_source_test.go`

#### 待修改项

- [ ] 逐段核对 L64、L69、L71、L79、L83 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、网络/协议 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*installSourceTool.planDownloadedURL` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/installsource/plan.go|*installSourceTool.planDownloadedURL` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/installsource/plan.go:62-89` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*installSourceTool.planDownloadedURL` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G37 处理。
- 重点保护：协议、序列化或流式事件兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/installsource/install_source_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/installsource`。
- [ ] 运行相关包 vet：`go vet ./internal/installsource`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/installsource/plan.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/installsource/plan.go|*installSourceTool.planDownloadedURL` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-282 · 圈复杂度 · `*installSourceTool.scanGitHubSkills`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-282 |
| Baseline key | `complexity\|internal/installsource/plan.go\|*installSourceTool.scanGitHubSkills` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 33） |
| 文件 | `internal/installsource/plan.go` |
| 代码行 | 181 |
| 函数/方法/包 | `*installSourceTool.scanGitHubSkills` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*installSourceTool.scanGitHubSkills` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 181-221 行；可见控制流热点：L186 `if depth > maxSkillScanDepth {`；L194 `for _, entry := range entries {`；L198 `switch entry.Type {`；L200 `if skipSkillRepoDir(entry.Name) {`；L206 `case "file":`；L211 `if ok {`。CodeGraph 记录的下游职责候选主要涉及 网络/协议（installSourceTool::fetchGitHubContents）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method installSourceTool::scanGitHubSkills（internal/installsource/plan.go:181-221）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `installSourceTool::planGitHubSkillRepo`（`internal/installsource/plan.go:165`，调用点 L166，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（2）：** `newErr`（`internal/installsource/errors.go:70`，调用点 L196，confidence=0.90，exact-match）；`skipSkillRepoDir`（`internal/installsource/plan.go:279`，调用点 L200，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `installSourceTool::githubSkillCandidate`（`internal/installsource/plan.go:223`，调用点 L207，confidence=0.70，instance-method，待源码复核）；`installSourceTool::fetchGitHubContents`（`internal/installsource/plan.go:255`，调用点 L189，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/installsource`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/installsource/install_source_test.go`
- **需同步检查：** `internal/installsource/plan.go`、`internal/installsource/errors.go`、`internal/installsource/install_source_test.go`

#### 待修改项

- [ ] 逐段核对 L186、L195、L199、L206、L211 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 网络/协议 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*installSourceTool.scanGitHubSkills` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/installsource/plan.go|*installSourceTool.scanGitHubSkills` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/installsource/plan.go:181-221` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*installSourceTool.scanGitHubSkills` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 G37 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/installsource/install_source_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/installsource`。
- [ ] 运行相关包 vet：`go vet ./internal/installsource`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/installsource/plan.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/installsource/plan.go|*installSourceTool.scanGitHubSkills` 下降或消失且无新增/恶化。
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

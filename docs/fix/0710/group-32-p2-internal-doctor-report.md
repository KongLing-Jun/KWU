# G32 · internal/doctor · report 治理批次

## 1. 分组概览

| 字段 | 内容 |
|---|---|
| 分组编号 | G32 |
| 问题数量 | 10 |
| 整体优先级 | P2 |
| 涉及模块 | `internal/doctor`、`internal/tool` |
| 涉及文件 | `internal/doctor/report.go`<br>`internal/tool/builtin/codeindex.go`<br>`internal/tool/builtin/completestep.go`<br>`internal/tool/builtin/movefile.go`<br>`internal/tool/sessiontool/sessiontool.go` |
| 主要治理目标 | 围绕 `internal/tool/builtin/codeindex.go` 等治理对象降低复杂度/体量，统一处理 1 个多规则函数对象，并保持调用链和行为边界。 |
| 推荐负责人范围 | doctor / tool 模块负责人；同一文件跨组时保持同一负责人 |
| 冲突可能性 | 低：仍需开始前复查开放 PR |
| 冲突依据 | 未发现静态热点或本地远端引用重叠 |
| 建议处理顺序 | 总序第 32 组；等待 G08、G13 的同文件变更合并 |
| 前置依赖 | G08（internal/tool/builtin/completestep.go）；G13（internal/tool/builtin/codeindex.go） |
| 后续验证重点 | 权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容；返回值、错误语义和现有调用顺序；持久化、迁移或文件格式兼容 |
| CodeGraph 证据口径 | 可信边（>=0.9）与待源码复核候选（0.7–0.89）分列；动态调用与 unresolved call 不作推断 |
| 待修改状态 | 待认领 / 未实施 |

### 为什么排在当前顺序

本组最高治理得分为 **44/100**，对象 `Collect` 的分项为严重度 2.9、集中度 9.6、影响面 11.5、可执行性 20.0；最高超限 1.20x，文件内 3 条问题，CodeGraph 记录 1 个可信生产调用方、0 个待核候选。该对象同时命中 圈复杂度、函数有效行数，必须合并治理。

### 治理边界与组间关系

本组涉及 5 个文件，按相近得分与模块/调用域补足本组 10 条；各文件可独立提交，但不得多人同时修改同一文件。前置同文件组：G08、G13。没有同文件后续组。

## 2. 问题汇总表

| ID | Baseline key | 规则 | 文件 | 函数/符号 | 行 | 当前/阈值 | 超出 | 优先级 | CodeGraph 影响摘要 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| `CX-245` | `complexity\|internal/doctor/report.go\|Collect` | 圈复杂度 | `internal/doctor/report.go` | `Collect` | 101 | 12/10 | +2（1.20x） | P2 | 1 可信 + 0 待核 caller / 11 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `FN-034` | `func_lines\|internal/doctor/report.go\|Collect` | 函数有效行数 | `internal/doctor/report.go` | `Collect` | 101 | 88/80 | +8（1.10x） | P2 | 1 可信 + 0 待核 caller / 11 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-376` | `complexity\|internal/tool/builtin/codeindex.go\|codeIndex.collect` | 圈复杂度 | `internal/tool/builtin/codeindex.go` | `codeIndex.collect` | 112 | 20/10 | +10（2.00x） | P2 | 0 可信 + 1 待核 caller / 2 可信 + 2 待核 callee / 14 unresolved | 待修改 |
| `CX-381` | `complexity\|internal/tool/builtin/completestep.go\|allCommandHints` | 圈复杂度 | `internal/tool/builtin/completestep.go` | `allCommandHints` | 351 | 17/10 | +7（1.70x） | P3 | 1 可信 + 0 待核 caller / 3 可信 + 1 待核 callee / 0 unresolved | 待修改 |
| `CX-388` | `complexity\|internal/tool/builtin/movefile.go\|moveFile.Execute` | 圈复杂度 | `internal/tool/builtin/movefile.go` | `moveFile.Execute` | 39 | 20/10 | +10（2.00x） | P3 | 0 可信 + 0 待核 caller / 4 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-393` | `complexity\|internal/tool/sessiontool/sessiontool.go\|*readSessionTool.Execute` | 圈复杂度 | `internal/tool/sessiontool/sessiontool.go` | `*readSessionTool.Execute` | 104 | 22/10 | +12（2.20x） | P3 | 0 可信 + 0 待核 caller / 2 可信 + 2 待核 callee / 0 unresolved | 待修改 |
| `CX-380` | `complexity\|internal/tool/builtin/completestep.go\|verifyCommandFromSession` | 圈复杂度 | `internal/tool/builtin/completestep.go` | `verifyCommandFromSession` | 281 | 11/10 | +1（1.10x） | P3 | 1 可信 + 0 待核 caller / 5 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-375` | `complexity\|internal/tool/builtin/codeindex.go\|codeIndex.Execute` | 圈复杂度 | `internal/tool/builtin/codeindex.go` | `codeIndex.Execute` | 73 | 13/10 | +3（1.30x） | P3 | 0 可信 + 0 待核 caller / 4 可信 + 1 待核 callee / 8 unresolved | 待修改 |
| `CX-378` | `complexity\|internal/tool/builtin/completestep.go\|completeStep.Execute` | 圈复杂度 | `internal/tool/builtin/completestep.go` | `completeStep.Execute` | 86 | 14/10 | +4（1.40x） | P3 | 0 可信 + 0 待核 caller / 4 可信 + 0 待核 callee / 0 unresolved | 待修改 |
| `CX-246` | `complexity\|internal/doctor/report.go\|RenderText` | 圈复杂度 | `internal/doctor/report.go` | `RenderText` | 192 | 11/10 | +1（1.10x） | P3 | 1 可信 + 0 待核 caller / 1 可信 + 0 待核 callee / 0 unresolved | 待修改 |

## 3. 逐条治理任务

### CX-245 · 圈复杂度 · `Collect`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-245 |
| Baseline key | `complexity\|internal/doctor/report.go\|Collect` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 44） |
| 文件 | `internal/doctor/report.go` |
| 代码行 | 101 |
| 函数/方法/包 | `Collect` |
| Baseline / 当前复测 / 阈值 | 12 / 12 / 10 |
| 超出 | +2，1.20x |
| 同一治理对象关联问题 | `FN-034` |

#### 明确的问题描述

`Collect` 的圈复杂度当前为 **12**，超过限制 **10**（+2，1.20x）。定义范围为 101-190 行；可见控制流热点：L103 `if paths.ProfileID() == "" {`；L111 `if err != nil {`；L119 `if legacyPath := config.LegacyUserConfigPath(); paths.ProfileID() == userctx.DefaultProfileID && us…`；L121 `if _, legacyErr := os.Stat(legacyPath); legacyErr == nil {`；L162 `for i := range cfg.Providers {`；L179 `if transport == "" {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（NewProfilePathResolver、LegacyUserConfigPath、ProfilePathResolver::ProfileID、ProfilePathResolver::UserConfigPath）、持久化/I-O（NewProfilePathResolver、ProfilePathResolver::ProfileID、ProfilePathResolver::UserConfigPath）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function Collect（internal/doctor/report.go:101-190）
- **可信生产调用方（1）：** `doctorCommand`（`internal/cli/doctor.go:12`，调用点 L19，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（11）：** `redactHome`（`internal/doctor/report.go:318`，调用点 L122/L123/L131/L133，confidence=0.90，exact-match，6 条边）；`NewProfilePathResolver`（`internal/config/paths.go:81`，调用点 L104，confidence=0.90，import）；`LegacyUserConfigPath`（`internal/config/paths.go:384`，调用点 L119，confidence=0.90，import）；`SourcePathForRootWithPaths`（`internal/config/paths.go:608`，调用点 L117，confidence=0.90，import）；`collectSessions`（`internal/doctor/report.go:263`，调用点 L141，confidence=0.90，exact-match）；`pluginTarget`（`internal/doctor/report.go:287`，调用点 L186，confidence=0.90，exact-match）；`hostOnly`（`internal/doctor/report.go:297`，调用点 L168，confidence=0.90，exact-match）；`redactHomeAll`（`internal/doctor/report.go:335`，调用点 L145，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `ProfilePathResolver::ProfileID`（`internal/config/paths.go:159`，调用点 L103/L119，confidence=0.70，instance-method，2 条边，待源码复核）；`ProfilePathResolver::UserConfigPath`（`internal/config/paths.go:176`，调用点 L118，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Run`（`internal/cli/cli.go:51`，调用点 L116，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 6 个文件；`internal/doctor`、`internal/cli`、`internal/config`、`internal/netclient`、`internal/sandbox`、`internal/userctx`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/doctor/report_test.go`
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `internal/doctor/report.go`、`internal/cli/doctor.go`、`internal/config/paths.go`、`internal/doctor/report_test.go`

#### 待修改项

- [ ] 逐段核对 L103、L111、L120、L162、L179 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `Collect` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 与同一治理对象的 CX-245、FN-034 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/doctor/report.go|Collect` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/doctor/report.go:101-190` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `Collect` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-245`、`FN-034` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/doctor/report_test.go`。
- [ ] 运行相关包测试：`go test ./internal/doctor`。
- [ ] 运行相关包 vet：`go vet ./internal/doctor`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/doctor/report.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/doctor/report.go|Collect` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### FN-034 · 函数有效行数 · `Collect`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | FN-034 |
| Baseline key | `func_lines\|internal/doctor/report.go\|Collect` |
| 规则 | func_lines（函数有效行数） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P3，治理对象得分 44） |
| 文件 | `internal/doctor/report.go` |
| 代码行 | 101 |
| 函数/方法/包 | `Collect` |
| Baseline / 当前复测 / 阈值 | 88 / 88 / 80 |
| 超出 | +8，1.10x |
| 同一治理对象关联问题 | `CX-245` |

#### 明确的问题描述

`Collect` 的有效代码行 **88**，超过限制 **80**（+8）。CodeGraph 定义物理范围为 101-190 行，并记录 13 个直接下游候选；候选职责分布为 配置/解析（NewProfilePathResolver、LegacyUserConfigPath、ProfilePathResolver::ProfileID、ProfilePathResolver::UserConfigPath）、持久化/I-O（NewProfilePathResolver、ProfilePathResolver::ProfileID、ProfilePathResolver::UserConfigPath）。后续需结合源码确认可独立验证的真实阶段。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function Collect（internal/doctor/report.go:101-190）
- **可信生产调用方（1）：** `doctorCommand`（`internal/cli/doctor.go:12`，调用点 L19，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（11）：** `redactHome`（`internal/doctor/report.go:318`，调用点 L122/L123/L131/L133，confidence=0.90，exact-match，6 条边）；`NewProfilePathResolver`（`internal/config/paths.go:81`，调用点 L104，confidence=0.90，import）；`LegacyUserConfigPath`（`internal/config/paths.go:384`，调用点 L119，confidence=0.90，import）；`SourcePathForRootWithPaths`（`internal/config/paths.go:608`，调用点 L117，confidence=0.90，import）；`collectSessions`（`internal/doctor/report.go:263`，调用点 L141，confidence=0.90，exact-match）；`pluginTarget`（`internal/doctor/report.go:287`，调用点 L186，confidence=0.90，exact-match）；`hostOnly`（`internal/doctor/report.go:297`，调用点 L168，confidence=0.90，exact-match）；`redactHomeAll`（`internal/doctor/report.go:335`，调用点 L145，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `ProfilePathResolver::ProfileID`（`internal/config/paths.go:159`，调用点 L103/L119，confidence=0.70，instance-method，2 条边，待源码复核）；`ProfilePathResolver::UserConfigPath`（`internal/config/paths.go:176`，调用点 L118，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `Run`（`internal/cli/cli.go:51`，调用点 L116，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 6 个文件；`internal/doctor`、`internal/cli`、`internal/config`、`internal/netclient`、`internal/sandbox`、`internal/userctx`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/doctor/report_test.go`
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `internal/doctor/report.go`、`internal/cli/doctor.go`、`internal/config/paths.go`、`internal/doctor/report_test.go`

#### 待修改项

- [ ] 把 `Collect` 的 88 个有效行按 配置/解析、持久化/I-O 阶段切分，明确每个阶段输入、输出和错误语义。
- [ ] 提取后的函数必须承担完整职责并有调用价值，禁止只做参数原样转发。
- [ ] 与同一治理对象的 CX-245、FN-034 一次完成，避免多次修改同一函数。
- [ ] 修复后重新扫描并显式确认 baseline key `func_lines|internal/doctor/report.go|Collect` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/doctor/report.go:101-190` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `Collect` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：权限、安全或凭据边界；会话、状态或恢复语义；配置、平台或向后兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。
- 必须与 `CX-245`、`FN-034` 作为同一函数治理对象处理。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/doctor/report_test.go`。
- [ ] 运行相关包测试：`go test ./internal/doctor`。
- [ ] 运行相关包 vet：`go vet ./internal/doctor`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/doctor/report.go`。
- [ ] 运行全量 Code Guard，并确认 `func_lines|internal/doctor/report.go|Collect` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-376 · 圈复杂度 · `codeIndex.collect`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-376 |
| Baseline key | `complexity\|internal/tool/builtin/codeindex.go\|codeIndex.collect` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P2（原始 P1，治理对象得分 44） |
| 文件 | `internal/tool/builtin/codeindex.go` |
| 代码行 | 112 |
| 函数/方法/包 | `codeIndex.collect` |
| Baseline / 当前复测 / 阈值 | 20 / 20 / 10 |
| 超出 | +10，2.00x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`codeIndex.collect` 的圈复杂度当前为 **20**，超过限制 **10**（+10，2.00x）。定义范围为 112-176 行；可见控制流热点：L114 `if err != nil {`；L124 `if ctx.Err() != nil {`；L131 `if path != root && skipCodeIndexDir(d.Name()) {`；L144 `if err != nil {`；L157 `if err != nil {`；L170 `if symbols[i].Line != symbols[j].Line {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（codeIndex::parseFile）、持久化/I-O（supportedCodeIndexFile、ArchiveSink::Err、codeIndex::parseFile）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method codeIndex::collect（internal/tool/builtin/codeindex.go:112-176）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（1）：** `codeIndex::Execute`（`internal/tool/builtin/codeindex.go:73`，调用点 L100，confidence=0.70，instance-method，待源码复核）
- **可信被调用函数（2）：** `supportedCodeIndexFile`（`internal/tool/builtin/codeindex.go:327`，调用点 L119/L136，confidence=0.90，exact-match，2 条边）；`skipCodeIndexDir`（`internal/tool/builtin/codeindex.go:336`，调用点 L131，confidence=0.90，exact-match）
- **待源码复核被调候选（2）：** `ArchiveSink::Err`（`internal/automation/archive_sink.go:90`，调用点 L124/L125/L153/L154，confidence=0.70，instance-method，4 条边，待源码复核）；`codeIndex::parseFile`（`internal/tool/builtin/codeindex.go:178`，调用点 L156，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `codeIndex::Execute`（`internal/tool/builtin/codeindex.go:73`，调用点 L100，confidence=0.70，instance-method，待源码复核）
- **未解析调用：** 14 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/tool`、`internal/automation`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`、`internal/tool/builtin/bash_timeout_test.go`、`internal/tool/builtin/bgjobs_test.go`、`internal/tool/builtin/builtin_test.go`、`internal/tool/builtin/codeindex_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/tool/builtin/confine_test.go`、`internal/tool/builtin/crlf_edit_test.go`
- **需同步检查：** `internal/tool/builtin/codeindex.go`、`internal/automation/archive_sink.go`、`internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`

#### 待修改项

- [ ] 逐段核对 L114、L127、L138、L153、L170 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `codeIndex.collect` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tool/builtin/codeindex.go|codeIndex.collect` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tool/builtin/codeindex.go:112-176` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `codeIndex.collect` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tool/builtin`。
- [ ] 运行相关包 vet：`go vet ./internal/tool/builtin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tool/builtin/codeindex.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tool/builtin/codeindex.go|codeIndex.collect` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-381 · 圈复杂度 · `allCommandHints`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-381 |
| Baseline key | `complexity\|internal/tool/builtin/completestep.go\|allCommandHints` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P2，治理对象得分 43） |
| 文件 | `internal/tool/builtin/completestep.go` |
| 代码行 | 351 |
| 函数/方法/包 | `allCommandHints` |
| Baseline / 当前复测 / 阈值 | 17 / 17 / 10 |
| 超出 | +7，1.70x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`allCommandHints` 的圈复杂度当前为 **17**，超过限制 **10**（+7，1.70x）。定义范围为 351-397 行；可见控制流热点：L354 `if ledger != nil {`；L362 `if msgs, ok := evidence.SessionMessagesFromContext(ctx); ok {`；L365 `for _, tc := range msg.ToolCalls {`；L373 `if c == "" || seen[c] {`；L382 `if len(cmds) >= 12 {`；L392 `if len(c) > 80 {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function allCommandHints（internal/tool/builtin/completestep.go:351-397）
- **可信生产调用方（1）：** `verifyStepEvidence`（`internal/tool/builtin/completestep.go:144`，调用点 L160，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（3）：** `SessionMessagesFromContext`（`internal/evidence/evidence.go:558`，调用点 L362，confidence=0.90，import）；`failedCallIDs`（`internal/tool/builtin/completestep.go:324`，调用点 L363，confidence=0.90，exact-match）；`extractCommandFromCall`（`internal/tool/builtin/completestep.go:409`，调用点 L372，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `Ledger::SuccessfulCommands`（`internal/evidence/evidence.go:128`，调用点 L355，confidence=0.80，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** `completeStep::Execute`（`internal/tool/builtin/completestep.go:86`，调用点 L116，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/tool`、`internal/evidence`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`、`internal/tool/builtin/bash_timeout_test.go`、`internal/tool/builtin/bgjobs_test.go`、`internal/tool/builtin/builtin_test.go`、`internal/tool/builtin/codeindex_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/tool/builtin/confine_test.go`、`internal/tool/builtin/crlf_edit_test.go`
- **需同步检查：** `internal/tool/builtin/completestep.go`、`internal/evidence/evidence.go`、`internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`

#### 待修改项

- [ ] 逐段核对 L354、L362、L369、L382、L392 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `allCommandHints` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tool/builtin/completestep.go|allCommandHints` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tool/builtin/completestep.go:351-397` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `allCommandHints` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tool/builtin`。
- [ ] 运行相关包 vet：`go vet ./internal/tool/builtin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tool/builtin/completestep.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tool/builtin/completestep.go|allCommandHints` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-388 · 圈复杂度 · `moveFile.Execute`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-388 |
| Baseline key | `complexity\|internal/tool/builtin/movefile.go\|moveFile.Execute` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P1，治理对象得分 41） |
| 文件 | `internal/tool/builtin/movefile.go` |
| 代码行 | 39 |
| 函数/方法/包 | `moveFile.Execute` |
| Baseline / 当前复测 / 阈值 | 20 / 20 / 10 |
| 超出 | +10，2.00x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`moveFile.Execute` 的圈复杂度当前为 **20**，超过限制 **10**（+10，2.00x）。定义范围为 39-101 行；可见控制流热点：L44 `if err := json.Unmarshal(args, &p); err != nil {`；L55 `if err := confine(m.roots, src); err != nil {`；L65 `if info.IsDir() {`；L80 `if dir := filepath.Dir(dst); dir != "" && dir != "." {`；L86 `if sameFileDestination {`；L93 `if cerr := copyRegularFileAndRemoveSource(src, dst, info); cerr != nil {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（renameFile、renameSameFileDestination、copyRegularFileAndRemoveSource）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method moveFile::Execute（internal/tool/builtin/movefile.go:39-101）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `renameFile`（`internal/tool/builtin/movefile.go:18`，调用点 L85，confidence=0.90，exact-match）；`renameSameFileDestination`（`internal/tool/builtin/movefile.go:103`，调用点 L87，confidence=0.90，exact-match）；`isCrossDeviceMove`（`internal/tool/builtin/movefile.go:135`，调用点 L92，confidence=0.90，exact-match）；`copyRegularFileAndRemoveSource`（`internal/tool/builtin/movefile.go:147`，调用点 L93，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 1 个文件；`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`、`internal/tool/builtin/bash_timeout_test.go`、`internal/tool/builtin/bgjobs_test.go`、`internal/tool/builtin/builtin_test.go`、`internal/tool/builtin/codeindex_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/tool/builtin/confine_test.go`、`internal/tool/builtin/crlf_edit_test.go`
- **需同步检查：** `internal/tool/builtin/movefile.go`、`internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`

#### 待修改项

- [ ] 逐段核对 L44、L58、L72、L85、L93 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `moveFile.Execute` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tool/builtin/movefile.go|moveFile.Execute` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tool/builtin/movefile.go:39-101` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `moveFile.Execute` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tool/builtin`。
- [ ] 运行相关包 vet：`go vet ./internal/tool/builtin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tool/builtin/movefile.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tool/builtin/movefile.go|moveFile.Execute` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-393 · 圈复杂度 · `*readSessionTool.Execute`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-393 |
| Baseline key | `complexity\|internal/tool/sessiontool/sessiontool.go\|*readSessionTool.Execute` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P1，治理对象得分 41） |
| 文件 | `internal/tool/sessiontool/sessiontool.go` |
| 代码行 | 104 |
| 函数/方法/包 | `*readSessionTool.Execute` |
| Baseline / 当前复测 / 阈值 | 22 / 22 / 10 |
| 超出 | +12，2.20x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`*readSessionTool.Execute` 的圈复杂度当前为 **22**，超过限制 **10**（+12，2.20x）。定义范围为 104-191 行；可见控制流热点：L110 `if err := json.Unmarshal(args, &params); err != nil {`；L129 `if err != nil {`；L140 `if *params.MaxTurns == 0 {`；L157 `case provider.RoleUser:`；L168 `if m.Content != "" {`；L183 `if params.ShowToolResults && m.Content != "" {`。CodeGraph 记录的下游职责候选主要涉及 配置/解析（LoadSession、readSessionTool::resolveSessionPath）、状态与控制（truncateRunes）、持久化/I-O（readSessionTool::resolveSessionPath）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method readSessionTool::Execute（internal/tool/sessiontool/sessiontool.go:104-191）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（2）：** `LoadSession`（`internal/agent/save.go:62`，调用点 L128，confidence=0.90，import）；`IsCleanupPending`（`internal/agent/save.go:256`，调用点 L123，confidence=0.90，import）
- **待源码复核被调候选（2）：** `truncateRunes`（`internal/tool/sessiontool/sessiontool.go:254`，调用点 L164/L170/L176/L184，confidence=0.70，exact-match，4 条边，待源码复核）；`readSessionTool::resolveSessionPath`（`internal/tool/sessiontool/sessiontool.go:195`，调用点 L117，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/tool`、`internal/agent`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tool/sessiontool/sessiontool_test.go`
- **需同步检查：** `internal/tool/sessiontool/sessiontool.go`、`internal/agent/save.go`、`internal/tool/sessiontool/sessiontool_test.go`

#### 待修改项

- [ ] 逐段核对 L110、L133、L153、L168、L183 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 配置/解析、状态与控制、持久化/I-O 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `*readSessionTool.Execute` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tool/sessiontool/sessiontool.go|*readSessionTool.Execute` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tool/sessiontool/sessiontool.go:104-191` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `*readSessionTool.Execute` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：持久化、迁移或文件格式兼容；会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tool/sessiontool/sessiontool_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tool/sessiontool`。
- [ ] 运行相关包 vet：`go vet ./internal/tool/sessiontool`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tool/sessiontool/sessiontool.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tool/sessiontool/sessiontool.go|*readSessionTool.Execute` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-380 · 圈复杂度 · `verifyCommandFromSession`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-380 |
| Baseline key | `complexity\|internal/tool/builtin/completestep.go\|verifyCommandFromSession` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 40） |
| 文件 | `internal/tool/builtin/completestep.go` |
| 代码行 | 281 |
| 函数/方法/包 | `verifyCommandFromSession` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`verifyCommandFromSession` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 281-311 行；可见控制流热点：L283 `if !ok {`；L287 `if lookup == "" {`；L294 `for _, tc := range msg.ToolCalls {`；L295 `if failed[tc.ID] {`；L302 `if evidence.CommandMatches(lookup, cmd) {`；L305 `if toolName != "" && toolName != "bash" && tc.Name == toolName {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function verifyCommandFromSession（internal/tool/builtin/completestep.go:281-311）
- **可信生产调用方（1）：** `verifyStepEvidence`（`internal/tool/builtin/completestep.go:144`，调用点 L156，confidence=0.90，exact-match）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（5）：** `CommandMatches`（`internal/evidence/commandmatch.go:10`，调用点 L302，confidence=0.90，import）；`SessionMessagesFromContext`（`internal/evidence/evidence.go:558`，调用点 L282，confidence=0.90，import）；`failedCallIDs`（`internal/tool/builtin/completestep.go:324`，调用点 L291，confidence=0.90，exact-match）；`firstWord`（`internal/tool/builtin/completestep.go:399`，调用点 L290，confidence=0.90，exact-match）；`extractCommandFromCall`（`internal/tool/builtin/completestep.go:409`，调用点 L298，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `completeStep::Execute`（`internal/tool/builtin/completestep.go:86`，调用点 L116，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 3 个文件；`internal/tool`、`internal/evidence`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`、`internal/tool/builtin/bash_timeout_test.go`、`internal/tool/builtin/bgjobs_test.go`、`internal/tool/builtin/builtin_test.go`、`internal/tool/builtin/codeindex_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/tool/builtin/confine_test.go`、`internal/tool/builtin/crlf_edit_test.go`
- **需同步检查：** `internal/tool/builtin/completestep.go`、`internal/evidence/commandmatch.go`、`internal/evidence/evidence.go`、`internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`

#### 待修改项

- [ ] 逐段核对 L283、L293、L295、L299、L305 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `verifyCommandFromSession` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tool/builtin/completestep.go|verifyCommandFromSession` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tool/builtin/completestep.go:281-311` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `verifyCommandFromSession` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：会话、状态或恢复语义。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tool/builtin`。
- [ ] 运行相关包 vet：`go vet ./internal/tool/builtin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tool/builtin/completestep.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tool/builtin/completestep.go|verifyCommandFromSession` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-375 · 圈复杂度 · `codeIndex.Execute`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-375 |
| Baseline key | `complexity\|internal/tool/builtin/codeindex.go\|codeIndex.Execute` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 38） |
| 文件 | `internal/tool/builtin/codeindex.go` |
| 代码行 | 73 |
| 函数/方法/包 | `codeIndex.Execute` |
| Baseline / 当前复测 / 阈值 | 13 / 13 / 10 |
| 超出 | +3，1.30x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`codeIndex.Execute` 的圈复杂度当前为 **13**，超过限制 **10**（+3，1.30x）。定义范围为 73-110 行；可见控制流热点：L75 `if err := json.Unmarshal(args, &p); err != nil {`；L82 `if p.Limit <= 0 {`；L85 `if p.Limit > codeIndexMaxLimit {`；L93 `if err != nil {`；L97 `if p.Action == "outline" && hasCodeSymbolFilter(p) {`；L105 `if len(symbols) > p.Limit {`。CodeGraph 记录的下游职责候选主要涉及 持久化/I-O（readablePath）、渲染/输出（formatCodeSymbols）；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method codeIndex::Execute（internal/tool/builtin/codeindex.go:73-110）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `hasCodeSymbolFilter`（`internal/tool/builtin/codeindex.go:282`，调用点 L97，confidence=0.90，exact-match）；`filterCodeSymbols`（`internal/tool/builtin/codeindex.go:286`，调用点 L104，confidence=0.90，exact-match）；`formatCodeSymbols`（`internal/tool/builtin/codeindex.go:305`，调用点 L109，confidence=0.90，exact-match）；`readablePath`（`internal/tool/builtin/confine.go:106`，调用点 L92，confidence=0.90，exact-match）
- **待源码复核被调候选（1）：** `codeIndex::collect`（`internal/tool/builtin/codeindex.go:112`，调用点 L100，confidence=0.70，instance-method，待源码复核）
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 8 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/tool`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`、`internal/tool/builtin/bash_timeout_test.go`、`internal/tool/builtin/bgjobs_test.go`、`internal/tool/builtin/builtin_test.go`、`internal/tool/builtin/codeindex_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/tool/builtin/confine_test.go`、`internal/tool/builtin/crlf_edit_test.go`
- **需同步检查：** `internal/tool/builtin/codeindex.go`、`internal/tool/builtin/confine.go`、`internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`

#### 待修改项

- [ ] 逐段核对 L75、L82、L88、L97、L105 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 逐条源码复核 CodeGraph 记录的 持久化/I-O、渲染/输出 调用候选，确认真实调用簇后再划分阶段，不按分支数量机械拆分。
- [ ] 保留 `codeIndex.Execute` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tool/builtin/codeindex.go|codeIndex.Execute` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tool/builtin/codeindex.go:73-110` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `codeIndex.Execute` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tool/builtin`。
- [ ] 运行相关包 vet：`go vet ./internal/tool/builtin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tool/builtin/codeindex.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tool/builtin/codeindex.go|codeIndex.Execute` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-378 · 圈复杂度 · `completeStep.Execute`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-378 |
| Baseline key | `complexity\|internal/tool/builtin/completestep.go\|completeStep.Execute` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 38） |
| 文件 | `internal/tool/builtin/completestep.go` |
| 代码行 | 86 |
| 函数/方法/包 | `completeStep.Execute` |
| Baseline / 当前复测 / 阈值 | 14 / 14 / 10 |
| 超出 | +4，1.40x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`completeStep.Execute` 的圈复杂度当前为 **14**，超过限制 **10**（+4，1.40x）。定义范围为 86-142 行；可见控制流热点：L93 `if err := json.Unmarshal(args, &p); err != nil {`；L99 `if strings.TrimSpace(p.Result) == "" {`；L107 `if !validEvidenceKinds[e.Kind] {`；L117 `if err != nil {`；L129 `if _, ok := evidence.FromContext(ctx); ok {`；L137 `if projectVerified > 0 {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph method completeStep::Execute（internal/tool/builtin/completestep.go:86-142）
- **可信生产调用方（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（4）：** `FromContext`（`internal/evidence/evidence.go:544`，调用点 L129，confidence=0.90，import）；`verifyStepEvidence`（`internal/tool/builtin/completestep.go:144`，调用点 L116，confidence=0.90，exact-match）；`verifyProjectChecks`（`internal/tool/builtin/completestep.go:187`，调用点 L124，confidence=0.90，exact-match）；`verifyTodoStep`（`internal/tool/builtin/completestep.go:236`，调用点 L120，confidence=0.90，exact-match）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/tool`、`internal/evidence`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** 未记录可信直接测试调用；不等于没有测试
- **同目录测试候选：** `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`、`internal/tool/builtin/bash_timeout_test.go`、`internal/tool/builtin/bgjobs_test.go`、`internal/tool/builtin/builtin_test.go`、`internal/tool/builtin/codeindex_test.go`、`internal/tool/builtin/completestep_test.go`、`internal/tool/builtin/confine_test.go`、`internal/tool/builtin/crlf_edit_test.go`
- **需同步检查：** `internal/tool/builtin/completestep.go`、`internal/evidence/evidence.go`、`internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`

#### 待修改项

- [ ] 逐段核对 L93、L102、L110、L125、L137 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `completeStep.Execute` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/tool/builtin/completestep.go|completeStep.Execute` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/tool/builtin/completestep.go:86-142` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `completeStep.Execute` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 人工核对同目录测试候选 `internal/tool/builtin/bash_cancel_test.go`、`internal/tool/builtin/bash_cancel_windows_test.go`、`internal/tool/builtin/bash_env_test.go`、`internal/tool/builtin/bash_powershell_test.go`、`internal/tool/builtin/bash_reap_test.go`，确认真实覆盖点后运行；同目录不等于已覆盖。
- [ ] 运行相关包测试：`go test ./internal/tool/builtin`。
- [ ] 运行相关包 vet：`go vet ./internal/tool/builtin`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/tool/builtin/completestep.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/tool/builtin/completestep.go|completeStep.Execute` 下降或消失且无新增/恶化。
- [ ] 检查未新增单函数碎片文件、纯转发包装函数或重复代码。
- [ ] 合并前按仓库 AGENTS.md 执行完整 PR Ready 检查。

### CX-246 · 圈复杂度 · `RenderText`

#### 基础信息

| 字段 | 内容 |
|---|---|
| 问题 ID | CX-246 |
| Baseline key | `complexity\|internal/doctor/report.go\|RenderText` |
| 规则 | complexity（圈复杂度） |
| 当前状态 | 仍存在（精确匹配）；审计快照与 baseline 六字段一致 |
| 当前优先级 | P3（原始 P3，治理对象得分 35） |
| 文件 | `internal/doctor/report.go` |
| 代码行 | 192 |
| 函数/方法/包 | `RenderText` |
| Baseline / 当前复测 / 阈值 | 11 / 11 / 10 |
| 超出 | +1，1.10x |
| 同一治理对象关联问题 | 无 |

#### 明确的问题描述

`RenderText` 的圈复杂度当前为 **11**，超过限制 **10**（+1，1.10x）。定义范围为 192-261 行；可见控制流热点：L196 `if r.CWD != "" {`；L210 `for _, p := range r.Providers {`；L212 `if p.KeyPresent {`；L223 `if len(r.Plugins) == 0 {`；L226 `for _, p := range r.Plugins {`；L245 `if r.Sandbox.Bash == "enforce" && !r.Sandbox.Available {`。CodeGraph 记录的下游职责候选主要涉及 CodeGraph 未提供足够的下游调用分类；这些候选用于划定人工核查范围，不单独作为拆分事实。

#### CodeGraph 代码定位

- **定义映射：** 按文件、短名和起始行唯一映射到 CodeGraph function RenderText（internal/doctor/report.go:192-261）
- **可信生产调用方（1）：** `doctorCommand`（`internal/cli/doctor.go:12`，调用点 L29，confidence=0.90，import）
- **待源码复核调用方候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **可信被调用函数（1）：** `valueOr`（`internal/doctor/report.go:308`，调用点 L199/L200/L201/L219，confidence=0.90，exact-match，7 条边）
- **待源码复核被调候选（0）：** CodeGraph 未确认直接关系；可能经接口、反射、注册表或外部入口调用，不作推断
- **上游入口候选（最多两跳）：** `Run`（`internal/cli/cli.go:51`，调用点 L116，confidence=0.90，exact-match，待源码复核）
- **未解析调用：** 0 条 unresolved call；不能据此断言调用图完整。
- **影响文件/目录候选：** 2 个文件；`internal/doctor`、`internal/cli`；目录出现不代表整个模块必然受影响。
- **CodeGraph 直接测试调用：** `internal/doctor/report_test.go`
- **同目录测试候选：** 未发现同目录测试文件候选
- **需同步检查：** `internal/doctor/report.go`、`internal/cli/doctor.go`、`internal/doctor/report_test.go`

#### 待修改项

- [ ] 逐段核对 L196、L210、L216、L226、L245 的条件/循环分支，将“策略判定、输入解析、状态变更、输出组装”中实际存在的独立职责提取为可命名、可单测的内部函数。
- [ ] 保留 `RenderText` 的核心编排顺序；提取后减少本函数的决策点，而不是把同一判断复制到多个 helper。
- [ ] 修复后重新扫描并显式确认 baseline key `complexity|internal/doctor/report.go|RenderText` 的值下降或条目消失。

#### 修改边界

- 主要修改范围：`internal/doctor/report.go:192-261` 及为其提取的同包内部职责；调用方仅在签名或依赖确有变化时同步调整。
- 保持 `RenderText` 的现有签名、返回值、错误语义和副作用顺序，除非另行协调调用方。
- 不顺带重构本组未列出的相邻职责；共享文件中的后续问题按 后续排期 处理。
- 重点保护：返回值、错误语义和现有调用顺序。
- 冲突风险：本地未合并远端引用未发现该文件重叠，但这不是实时 GitHub PR 结论；开始前仍需复查。

#### 验证要求

- [ ] 优先运行 CodeGraph 记录的直接测试调用文件：`internal/doctor/report_test.go`。
- [ ] 运行相关包测试：`go test ./internal/doctor`。
- [ ] 运行相关包 vet：`go vet ./internal/doctor`。
- [ ] 运行目标文件局部守卫：`go run ./scripts/code_guard -- internal/doctor/report.go`。
- [ ] 运行全量 Code Guard，并确认 `complexity|internal/doctor/report.go|RenderText` 下降或消失且无新增/恶化。
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

# 系统提示词执行内核与场景评测开发计划

## 1. 背景与当前问题

Desktop 通过同一套 `boot.Build` 与 `Controller` 运行 Office、Coding、Design 三个 Scene。
三者拥有相同模型、工具、权限、Skills、Memory 和 Expert，区别主要来自基础系统提示词后追加的
短 Scene Profile。

2026-08-04 的真实 `gpt-5.6-luna` 评测证明当前提示词能生成正确产物，但仍有三个已确认问题：

1. 产物已经正确且完成检查后，Agent 仍继续调用工具并最终超时。
2. Coding 任务执行了检查，却没有证明核心行为语义与边界条件正确，出现真实逻辑错误。
3. Design 任务满足部分结构要求，却缺少可见键盘焦点等真实交互证据。

上一轮多个候选中，Extended 是唯一值得保留的实验基础：诊断集端到端与当前提示词同为
16/18，完整运行 token、模型调用、工具调用和耗时分别降低约 21%、31%、34% 和 21%，
冻结回归从 5/12 提升到 9/12。它仍有上述错误，因此当时正确地保持 no-go。完整历史结果见
[`evaluation-results.md`](./evaluation-results.md)。

本轮目标不是盲目替换提示词或迎合旧题库，而是从真实失败出发，形成一个能让 Desktop Agent
在三个 Scene 中更可靠、自主、高效完成中型任务的系统提示词。

## 2. 已确认决策

- 优先级固定为真实任务质量与可靠性第一、执行效率第二、token 成本第三；明显能力退化不能
  由更低 token 抵消。
- 只维护一个主候选：以 Extended 的有效结构为基础形成 Extended V2，不再运行多候选全量竞赛。
- 第一轮只修三个已知问题：完成后不停止、Coding 语义验证不足、Design 视觉/焦点验证不足。
  Office 保持已有效行为，Desktop 中型任务若发现新证据再定向调整。
- 修改落在负责该行为的层：通用内核负责完成判定与及时停止；Coding Profile 负责语义和边界
  验证；Design Profile 负责可渲染产物、视觉状态和交互证据。
- WorkBuddy 5.3.8 只提供可验证的职责分层、能力按需、运行时纠偏和表达分离原则；不复制产品
  专属流程、不可验证的根提示词或静态体积。
- 内核和 Scene Profile 使用简洁英文；品牌名仍由 `brand.DisplayName` 动态提供；用户回复语言
  继续由独立 Language Policy 控制。
- 内核筛选采用 3 次 smoke，Office/Coding/Design 各一次；通过后再运行 6 个冻结任务各一次，
  不做“多个候选 × 完整套件”。
- 最终裁判是实际 Wails Desktop。A/B 使用当前基线构建和仅提示词不同的候选构建，不把候选
  注入用户消息、Expert 或附件。
- Desktop 冻结六个中型任务，每个版本先运行两次，共 24 次；同一组合结果冲突、异常或无法
  明确判断时追加第三次，最多 36 次。
- 主要 A/B 固定用户确认的 OpenAI-compatible 供应商、`gpt-5.6-luna`、temperature 0、low
  effort、auto plan off、max steps 0；候选通过后用 auto effort 每个 Scene 补一次兼容 smoke。
- Desktop 使用默认 `ask` 权限，只批准一次性工作区内的预期读写与测试命令；审批次数计入效率，
  审批等待不计模型执行耗时。
- 真实 Agent/Desktop/Controller 错误先落盘记录，不在本任务修复，也不暂停剩余可执行验收；
  被运行时缺陷污染的结果标记为不可判定，不能归罪于提示词或伪装成通过。
- 文档、评测资产、提示词实现和验收证据按意图小步提交；不推送、不创建 PR，除非后续另行授权。

## 3. 源码与产品边界

当前直接源码复核基线为 `origin/main-v3@a4029fd0`，本工作树已保留既有评测历史并合并该基线。

| 位置 | 当前契约与本轮边界 |
| --- | --- |
| `internal/config/config.go` | `DefaultSystemPrompt` 是共享默认基础提示词；自定义 prompt 可整体替换 |
| `internal/scene/scene.go` | 三个 Scene Profile 只表达工作偏好，不改变能力和权限 |
| `internal/boot/boot.go` | 组装顺序为基础 prompt → Expert → Scene → 固定策略 → Memory → Skill Index |
| `internal/control/task_lifecycle.go` | Desktop 首次 Root Runner 前锁定 Draft Scene 并重建 system message |
| `clouvia.example.toml` | 示例不得长期复制一份与内置默认值分叉的提示词 |
| `benchmarks/prompt-eval` | 已有真实 Controller runner、grader、候选和去敏报告，可扩展中型任务资产 |
| `desktop` | 最终应用入口；本轮不修改 Scene 生命周期、Wails bridge、Controller 或前端交互 |

`system_prompt` / `system_prompt_file` 和替换型 Output Style 继续保留现有整体替换语义。本轮不新增
PromptRevision、运行时实验开关、测试后门、模型、工具、权限、Skill、Memory 或 Expert。

## 4. 目标与非目标

### 4.1 目标

- 建立清晰但不过度规定的执行闭环：理解目标和成功证据 → 获取必要上下文 → 执行 → 用最小
  有意义的检查尝试证伪 → 只修具体失败 → 满足目标后及时停止并交付证据。
- 简单可逆任务直接执行；多步、高风险或不确定任务才维护轻量 todo，避免流程仪式化。
- Coding 验证覆盖请求的行为语义、关键边界和真实测试结果，而不是把“运行了命令”当作正确。
- Design 验证覆盖文件有效性、真实渲染相关结构、可见状态和明确要求的可访问性交互证据。
- Office 不退化，并通过多来源事实整合和数值分析任务寻找真实缺口。
- 建立可重复生成的一次性 Desktop fixture、独立评分标准和去敏验收记录。

### 4.2 非目标

- 不修复验收期间发现的 Controller、Desktop、供应商或工具实现缺陷。
- 不用提示词替代权限、安全边界、沙箱、工具 schema、运行时超时或结构化结果校验。
- 不强制每个任务规划、调用 Skill、使用子代理或执行固定数量的检查。
- 不复制 WorkBuddy 文案，不宣称它拥有与本应用相同的三个 Scene 根提示词。
- 不修改 Scene Draft/Locked/Legacy 生命周期、历史会话、模型选择、供应商协议或 UI。
- 不把 benchmark 通过等同于 Desktop 产品验收。

## 5. 冻结任务矩阵

| Scene | 任务 | 必须观察的能力 |
| --- | --- | --- |
| Office 1 | 读取 DOCX 简报、CSV 里程碑和会议记录，生成管理报告与行动项 CSV | 多来源事实、冲突处理、负责人/日期、文件交付 |
| Office 2 | 读取 XLSX 预算和业务说明，生成预算差异分析与决策摘要 | 数值与方向准确、不虚构原因、结论可用 |
| Coding 1 | 修复小型 Python 重试模块的指数退避、上限和边界条件并运行测试 | 算法语义、边界测试、完成后停止 |
| Coding 2 | 修复跨文件嵌套配置合并，保持既有行为并补充回归测试 | 多文件理解、最小改动、兼容性 |
| Design 1 | 根据 brief 制作响应式空状态 HTML/CSS 组件 | 可打开产物、层级、响应式、焦点、减少动态 |
| Design 2 | 制作含数据卡片和交互入口的独立 SVG/HTML 仪表盘组件 | 有效 SVG、视觉结构、可见焦点、可访问性 |

每个工作区约 3–6 个输入文件，预期 5–12 次工具调用和 3–8 分钟完成。任务说明只包含用户
需求；具体 rubric 独立保存。每次运行使用干净复制，不继承会话、Memory 或其他项目指令。

## 6. 实施方案

### 6.1 评测资产

- 为六个任务建立确定性 fixture 生成器、任务 manifest、独立 rubric 与离线评分入口。
- Office 输入真实覆盖 `read_file` 已支持的 DOCX/XLSX 提取；输出使用当前工具可可靠创建和检查的
  Markdown、CSV、HTML、CSS、SVG 与代码文件。
- 同时记录 artifact、关键语义、run 完成、工具/模型调用、审批、耗时和限制；报告不含 key。
- 原始工作区和完整会话只保留在 Git 忽略目录，版本化材料只记录去敏结论与必要证据。

### 6.2 唯一候选

- 保留 Extended 已证明有收益的五层结构：理解、适度流程、执行验证、有限恢复、交付停止。
- 采用重写替换，不在当前提示词和 Extended 后继续追加重复规则。
- 通用内核只明确“成功工具调用不是完成”“检查通过且无未解决要求即停止”“不为额外信心继续
  调用工具”。
- Coding Profile 要求用与请求语义相符的测试尝试证伪实现，覆盖关键边界并读取真实结果。
- Design Profile 要求验证可打开/可渲染产物及明确要求的可见交互状态，不能以属性存在代替视觉
  或交互证据。
- Office Profile 仅澄清事实、计算和可用交付，不增加未经证据支持的复杂流程。

### 6.3 分级真实测试

1. 静态检查候选的冲突、重复、层级、硬能力声明和长度。
2. `boot.Build + Controller` 各 Scene 一次 smoke；明显失败时只修对应规则。
3. smoke 全部通过后运行六个冻结任务各一次，作为进入 Desktop 的低成本确认。
4. 构建基线和候选 Wails Desktop，按任务交替运行，固定模型和批准规则。
5. 每个组合先两次；冲突或异常才追加第三次。设计产物必须实际渲染观察。
6. 候选通过后，用 auto effort 在三个 Scene 各运行一次兼容 smoke。

## 7. 生产门槛

以下任一项使单次运行硬失败：必需产物缺失/损坏/不可打开；Office 关键事实或计算错误；Coding
行为错误、测试失败或破坏无关代码；Design 无效、不可渲染或缺少明确交互状态；虚报检查；
完成后循环超时；修改禁止范围或产生未经确认的外部副作用。

候选进入生产还必须同时满足：

1. 六个任务在候选侧都取得多数通过，冲突时由第三次决胜。
2. Office 不低于当前提示词；三个 Scene 的产物质量均不退化。
3. Coding 语义错误和 Design 焦点/渲染错误不再稳定出现。
4. 至少一个目标能力明确提升；不能只凭措辞更完整或静态 prompt 更长通过。
5. 质量相同时选择模型回合、工具调用、审批和停止时间更少的版本。
6. token、调用和耗时完整记录，但质量明确提升时不设机械 token 否决线。
7. 供应商明确故障可作废重跑；模型跑偏、循环和错误工具选择必须计入结果。

## 8. 影响范围与任务切片

- 任务 005：`benchmarks/prompt-eval/**` 和文档，建立 Desktop 中型任务、fixture、rubric 与评分资产。
- 任务 006：`internal/config/config.go`、`internal/scene/scene.go`、`clouvia.example.toml`、邻近测试
  与 Extended V2 候选，实施定向提示词改动。
- 任务 007：实际 Desktop 构建、A/B、auto smoke 和去敏结果文档；只允许基于证据继续调整任务
  006 的提示词文件，不修改运行时。

每个切片按可审查意图提交；验收后发现的提示词问题使用新的 `fix:` 提交，不 amend 或重写历史。

## 9. 风险、回滚与验证

| 风险 | 控制 |
| --- | --- |
| 只修旧题导致过拟合 | 六个中型冻结任务、跨场景不退化、Desktop 重复运行 |
| 强调停止导致过早结束 | 停止绑定真实产物、约束满足和最小有意义验证 |
| Scene 变成能力边界 | 相同工具 schema，Profile 只表达工作方式 |
| 提示词增长放大每回合成本 | 同时观察静态长度、完整 usage 与回合减少，不用长度代理效果 |
| 人工审批污染耗时 | 两版相同批准规则，审批等待与模型执行时间分开记录 |
| Windows shell 无 OS 沙箱 | 一次性工作区、不注入其他凭据、默认 ask、禁止范围外操作 |
| 运行时缺陷干扰归因 | 独立记录为不可判定，继续其他任务，不在本轮修复 |
| 凭据泄漏 | key 仅进入一次性 Profile/env；报告、Git diff 和忽略目录做样式扫描 |

无数据迁移。候选未通过时回滚 `DefaultSystemPrompt`、三个 Scene Profile 和示例配置；评测资产
与 no-go/验收证据保留。自动验证覆盖受影响的 `internal/config`、`internal/scene`、`internal/boot`
和 evaluator；最终按变更分类运行仓库要求的检查。未经 PR 授权不运行发布动作、不推送、不开 PR。

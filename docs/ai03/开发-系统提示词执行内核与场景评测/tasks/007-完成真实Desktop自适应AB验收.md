# 007 完成真实 Desktop 自适应 A/B 验收

## 执行合同

- 状态：completed，任务 005、006 已完成。
- 目标：从用户真实入口比较当前基线和仅提示词不同的候选 Desktop，决定候选是否进入生产。
- 产物：两个可追溯构建、24 次基础 A/B、必要的第三次决胜、3 次 auto effort smoke、去敏结果
  与真实 Agent/运行时错误记录。
- 验收者：主会话 Codex；通过测试专用 WebView2 CDP 构建与 Playwright 操作实际 Wails 应用，
  生产源码未为验收控制链路修改。

## 固定条件

- 基线与候选从同一源码基线构建，应用差异只允许任务 006 的提示词文件。
- 使用用户确认的同一 OpenAI-compatible 供应商、API Key 和 `gpt-5.6-luna`；凭据只进入一次性
  Profile/env，不写报告、日志或 Git。
- temperature 0、low effort、auto plan off、max steps 0；每次外部观察上限 12 分钟。
- 默认 `ask` 权限；只批准一次性工作区内的预期读写和测试命令，记录审批次数。
- 每个任务/版本先两次，按任务交替运行；冲突、异常或无法明确判断时追加第三次。
- 候选通过后切换 auto effort，每个 Scene 运行一次兼容 smoke。

## 生产门槛

- 六个任务在候选侧均多数通过；Office 和三个 Scene 产物质量不退化。
- Coding 语义错误、Design 无效渲染/不可见焦点和完成后循环不再稳定出现。
- 必需产物错误、关键事实/计算错误、测试失败、虚报验证、越界、未经确认副作用或循环超时均
  为硬失败。
- 候选至少一个目标能力明确提升；同质量时才用模型/工具调用、审批、耗时和 token 决胜。

## 错误记录边界

- 真实 Agent/Desktop/Controller、工具或供应商缺陷记录触发任务、应用状态、可见现象、日志/源码
  证据和对归因的影响。
- 不在本任务修复运行时，不暂停其他可执行任务；受污染结果标记为不可判定。
- 提示词问题只允许修改任务 006 列出的提示词范围，使用新的 `fix:` 提交并复测对应任务与最小
  三场景回归。

## 运行缺陷记录

### RF-001：Controller 评测器拒绝产品支持的不限步数配置

- 触发：候选 Office smoke，显式传入 `-max-steps 0`；发生于任何模型请求之前。
- 现象：评测器返回 `repeats, max-steps, and timeout must be positive` 并退出；没有产物，也没有
  可用于提示词归因的运行结果。
- 源码证据：产品配置与示例把 `0` 定义为不限步数，而 `benchmarks/prompt-eval` 的参数校验把
  `MaxSteps <= 0` 统一判为非法；两者契约冲突。
- 处理：本任务不修改评测器或 Controller。后续内核测试使用 `max_steps=10000` 作为本组任务的
  实际不限步数绕行，并在结论中保留配置偏差；Desktop 仍按产品原生 `0` 验收。
- 归因：基础设施缺陷，不归因于提示词；该次尝试不计入 smoke 次数。

### RF-002：Profile Card grader 把角色语义误限定为字面量 `role`

- 触发：Extended V2 的 `design-profile-card-svg` 首次确认运行。
- 现象：产物评分报告 `missing SVG concept: role`；实际 SVG 的可访问描述与可见文本均明确包含
  `product designer`，满足任务要求的姓名与角色，但没有独立写出单词 `role`。
- 源码证据：`gradeProfileCardSVG` 的该组同义词只有 `{"role"}`，未接受实际职位名称或结构化
  identity 文本；属于已生成产物与 grader 词法规则之间的假阴性。
- 处理：不修改 grader，不把本次 artifact=false 归因于提示词；保留原始报告和产物。
- 复现补充：替代运行生成的 SVG 含 `.role`、`Product Designer` 和
  `#action:focus-visible rect` 5px 高对比描边，grader 又因不解析 `<style>` 而报告
  `missing SVG concept: focus`；进一步确认词法 grader 不能代表实际视觉语义。

### RF-003：最终一次 `verify_artifact` 后 readiness 仍报告未验证

- 触发：同一次 `design-profile-card-svg` 运行。
- 现象：工具序列以 `write_file → read_file → verify_artifact` 结束，Controller 随后仍以
  `requested workspace artifact has not been verified after the latest possible workspace mutation` 失败；
  29 次模型调用、26 次工具调用，产物已存在且 XML 可读。
- 证据限制：当前评测结果只保存聚合工具序列，没有持久化各工具调用结果，因此无法从该报告判断
  最后一次 verify 是否被 readiness 状态机接纳；结果标记为不可判定，不归罪于提示词。
- 处理：不修改 Controller；追加一次同题替代运行，判断该完成态异常是否稳定复现，其他验收继续。
- 复现补充：替代运行再次以 `verify_artifact` 结束后收到同一 readiness 错误，确认 2/2 复现；
  Desktop 需要单独观察真实完成态，但该缺陷不阻断其他场景。

## 完成回执

- 状态：completed。
- 构建身份：baseline `ee4c1dc6faaf8d9fa8bdfa23cfbb2c4dbef515bb`，candidate
  `adb40277b9b2e1f159963e74d7665711c6e1e216`；两个 production 测试构建使用相同 Wails
  testdiag patch，二进制摘要与固定条件见 [`desktop-ab-results.md`](../desktop-ab-results.md)。
- 基础矩阵：六个冻结中型任务 × 两个构建 × 两次，共 24/24 完成。缺陷归因后两组任务质量
  均为 12/12；候选 tokens -17.4%、模型调用 -23.7%、工具调用 -16.2%、模型耗时 -9.3%。
- 真实应用：Design 八个基线/候选产物及 provider-auto 产物均独立打开，完成 1440×900、
  390×844、横向溢出与键盘焦点检查；Office/Coding 产物分别核对事实、计算和独立测试结果。
- provider-auto：候选在不发送供应商特定 effort 字段时完成 Office、Coding、Design 3/3 smoke。
- 第三轮：未触发。两轮任务质量一致；混合 raw grader/完成态均已有源码、运行和产物证据归因，
  继续采样不能消除冻结契约缺陷。
- 排除尝试：错误 Profile 1 次、错误 workspace 2 次、空会话 1 次，均保留在忽略目录且不计分。
- 决策：接受 Extended V2。审批次数和 Desktop 正常结束数没有改善，不声称 RF-003 已修复。

### 补充缺陷索引

- RF-004：预算 grader 不接受 Unicode 负号/中文等价事实，产物数值与语义正确。
- RF-005：重试任务未指定异常类型，hidden grader 仅接受 `ValueError`；真实独立测试均通过。
- RF-006：验收清理观察（非产品缺陷）；测试 Profile 配置为关窗后后台运行，按确认 PID 清理。
- RF-007：Design dashboard grader 使用冻结 brief 未声明的字面 SVG 引用和几何阈值；四个实际
  页面均成功渲染并通过双 viewport 与键盘焦点验收。

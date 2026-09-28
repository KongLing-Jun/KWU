# 005 建立 Desktop 中型任务评测资产

## 执行合同

- 状态：completed。
- 目标：为 Office、Coding、Design 各建立两个可重复的中型 Desktop 任务，提供确定性 fixture、
  隔离工作区、独立 rubric 和离线评分入口。
- 依赖：任务 001 的真实 runner、grader 与去敏报告经验；不调用真实模型。
- 产物：`benchmarks/prompt-eval/**` 下的任务资产、生成/评分代码、测试和使用说明。
- 执行者：主会话 Codex；不委派。

## 冻结任务

1. Office 项目报告：DOCX 简报、CSV 里程碑、会议记录 → 管理报告与行动项 CSV。
2. Office 预算分析：XLSX 预算、业务说明 → 差异分析与决策摘要。
3. Coding 重试语义：小型 Python 模块 → 指数退避、上限、边界和测试。
4. Coding 嵌套配置：跨文件 Python 项目 → 深层合并、兼容行为和回归测试。
5. Design 空状态：brief → 响应式 HTML/CSS、键盘焦点和 reduced motion。
6. Design 仪表盘：brief → 有效 SVG/HTML、数据卡片、交互入口和可见焦点。

## 允许与禁止范围

- 允许：`benchmarks/prompt-eval/**`、本任务回执、`plan.md`、`progress.md`。
- 允许生成真实 DOCX/XLSX fixture，但必须由确定性代码创建，不手工维护不透明二进制。
- 禁止修改生产提示词、Scene、Controller、Desktop、供应商、工具或权限。
- 任务输入只包含受控测试数据，不加入提示注入文本或真实用户数据。
- rubric 与任务说明分离；评分器既要有正例也要有反例，不能只检查关键词数量。

## 验收标准

- 一个命令可为任一任务创建干净工作区，重复创建内容 hash 稳定。
- 一个命令可对已完成工作区输出 artifact/语义/运行观察结果；运行时人工观察可作为显式输入，
  不能由文件评分器臆测。
- Office 评分覆盖关键事实、计算方向和禁止虚构；Coding 使用真实测试命令；Design 覆盖文件
  解析、结构、焦点和 reduced motion，最终仍需实际渲染观察。
- fixture、rubric、grader 和参数进入 manifest；报告不包含 endpoint 或 key。
- 聚焦测试通过，EOL 为 LF，`git diff --check` 通过，guard baseline 不变。

## 完成回执

- 状态：completed。
- 变更：在 `benchmarks/prompt-eval` 增加六个冻结 Desktop 任务、确定性 DOCX/XLSX 与文本
  fixture、离线 artifact grader、人工运行观察契约、manifest 源码 hash 和使用说明；现有真实
  Controller runner 行为保持不变。
- manifest：`badc28f9ed0f31704123d1b97ddd851d6a61ef8184e5b97100d3a3ddea7e2f5f`。
- 自动验证：`go test ./benchmarks/prompt-eval/...`、`go vet ./benchmarks/prompt-eval/...`、
  `go run ./scripts/code_guard`、guard baseline diff 与 `git diff --check` 全部通过。
- 命令验收：六个 task ID 均能创建隔离工作区且共享同一 manifest；新建但未执行的工作区同时
  得到 `artifact_passed=false`、`observation_complete=false`、`end_to_end_passed=false`。
- grader 证据：正反例覆盖 Office 事实/计算与输入保护、Coding 隐藏语义和真实测试、Design
  可见焦点、响应式、reduced motion、颜色模式、有效 SVG，以及 Design 必需的实际渲染观察。
- 偏差与风险：文件 grader 不声称完成 UI 或视觉验收；run/范围/虚报/超时和设计视觉结果必须
  由任务 007 的 Desktop 观察 JSON 提供。没有调用模型或使用供应商凭据。
- 交接：任务 006 只使用这六个冻结任务，不再临时换题或把 rubric 写入用户提示。

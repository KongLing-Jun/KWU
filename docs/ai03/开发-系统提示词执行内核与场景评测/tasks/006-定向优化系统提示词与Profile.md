# 006 定向优化系统提示词与 Profile

## 执行合同

- 状态：in_progress，任务 005 已完成。
- 目标：以 Extended 为证据基础形成唯一 Extended V2，定向修复完成后不停止、Coding 语义验证
  不足、Design 视觉/焦点验证不足，同时保持 Office 和共享宿主契约。
- 产物：候选文案、生产 `DefaultSystemPrompt`、三个短 Scene Profile、示例配置和邻近测试。
- 执行者：主会话 Codex；不委派。

## 行为边界

- 通用内核保留理解、适度流程、执行验证、有限恢复、交付停止五层结构。
- 通用内核不得包含 Coding/Design 专属 checklist，也不得强制所有任务写 todo、调用 Skill 或子代理。
- Office 只强调事实、计算与可用交付，不凭空扩展流程。
- Coding 要求用与请求语义相符的测试尝试证伪，覆盖关键边界并读取真实结果。
- Design 要求验证可打开/可渲染产物及明确要求的可见交互状态，不以属性存在替代证据。
- 使用英文执行文案、动态品牌名和既有 Language Policy；不复制 WorkBuddy 原文。

## 允许与禁止范围

- 允许：`benchmarks/prompt-eval/candidates/extended-v2.md`、`internal/config/config.go`、
  `internal/scene/scene.go`、`clouvia.example.toml`、邻近测试和本任务回执。
- `internal/boot` 只允许增加提示词组装/唯一性回归；无生产组装缺陷时不得修改活动实现。
- 禁止修改 Controller、Desktop/Wails、工具、权限、Scene 生命周期、模型或供应商。
- 真实运行发现的非提示词错误只写入任务 007 证据，不在本任务修复。

## 验收标准

- 候选通过冲突、重复、分层、能力声明和静态长度审查。
- 自定义 `system_prompt` / `system_prompt_file`、替换型 Output Style、Expert、Memory、Skill Index、
  Language Policy 和 Scene 锁定重建顺序保持兼容。
- 工具 schema、模型和权限不因 Scene 改变。
- 3 次内核 smoke 全部通过后，再运行六个冻结任务各一次；结果决定能否进入 Desktop。
- 受影响 Go 包、evaluator 测试、EOL 与 diff 检查通过，guard baseline 不变。

## 完成回执

- 状态：completed；提示词实现、本地契约验证与真实模型内核门禁完成，允许进入 Desktop。
- 已实现唯一 `extended-v2` 候选，并同步为生产默认提示词；候选与生产文案由测试锁定。
- 三个 Profile 只增加各自需要的证据要求；示例配置改为默认继承内置提示词，避免发布后仍示范旧文案。
- 本地验证通过：`go test ./internal/config -count=1`、`go test ./internal/scene -count=1 -v`、
  `go test ./benchmarks/prompt-eval/... -count=1 -v`、六个指定 `internal/boot` 装配契约测试，
  以及 `go vet ./internal/config ./internal/scene ./benchmarks/prompt-eval/...`。
- 真实模型结果：三个 smoke 全部通过；六个确认任务中五项 Artifact/Run 全通过，Profile Card
  产物经源码复核满足任务语义，但受 RF-002 grader 假阴性和 RF-003 readiness 缺陷污染，追加
  一次仍复现。详细数据见 [`../extended-v2-results.md`](../extended-v2-results.md)。
- 决策：不为迎合 grader 修改提示词；按“不因运行时缺陷暂停其他验收”的冻结规则进入 Desktop。

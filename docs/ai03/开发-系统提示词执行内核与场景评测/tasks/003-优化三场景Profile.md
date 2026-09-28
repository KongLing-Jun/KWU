# 003 优化三场景 Profile

## 执行契约

- 状态：completed（依赖门禁 no-go），依赖任务 002 的内核真实验收。
- 目标：在通用内核之上增加三个短 Scene Operating Profile，改善工作方式但不制造能力边界。
- 产物：Office、Coding、Design Profile、组装回归、profile on/off 配对结果。
- 执行者：主会话 Codex（当前会话模型）。

## 允许范围

- `internal/scene/scene.go` 与测试。
- `internal/boot/scene_prompt_test.go` 和必要的 Desktop Scene 回归。
- `benchmarks/prompt-eval/**` 的 profile 配对用例和结果。
- 本任务回执和 `progress.md`。
- 禁止改变模型、工具 schema、权限、Skills、Memory、Expert 或 Scene 持久化生命周期。

## 验收

- Office 强化可直接使用的办公交付；Coding 强化工程上下文、实现和聚焦验证；Design 强化受众、层级、视觉方向和真实可生成成果。
- 每个 Profile 至少两个本场景任务和一个反场景任务做 on/off 相同输入配对。
- Scene rubric 有增益，端到端成功不下降；反场景任务不拒绝且不虚构能力边界。
- 三 Scene 工具 schema 完全一致；重建、Expert/模型切换后片段唯一且顺序稳定。
- 聚焦验证至少覆盖 `go test ./internal/scene ./internal/boot`；Desktop 模块只运行直接受影响测试。

## 完成回执

- 状态：completed；不保留生产 Profile 改动。
- smoke 结果：在 Extended 上先执行两个 Design SVG，用增强 Profile 得到 0/2；两个产物均为
  无效 SVG 且运行未正常结束，明显劣于基线。
- 处置：Scene 生产文案已在独立修复提交中完整回滚；HEAD 与基线的 Scene 内容和工具 schema
  相同。通用内核未通过任务 002 门禁，因此没有扩大到无价值的完整 on/off 配对。
- 偏差：任务原计划以通过的通用内核为前置；前置失败后按依赖终止是预期 no-go，不把 Scene
  变成能力边界，也不改模型、权限、Skills、Memory 或 Expert。

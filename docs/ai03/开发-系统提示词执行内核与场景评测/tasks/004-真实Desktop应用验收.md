# 004 真实 Desktop 应用验收

## 执行契约

- 状态：completed，依赖任务 001—003 完成。
- 目标：在实际 Wails Desktop 应用中验证最终提示词，而不是用 benchmark harness 代替用户路径。
- 产物：三场景应用验收记录、最终自动化检查、任务回执和进度快照。
- 验收者：主会话 Codex；通过 Windows Computer Use 操作真实应用。

## 场景

- Office：从本地事实文件创建可直接使用的结构化办公成果，核对事实并停止。
- Coding：修复一个局部代码问题，执行聚焦验证，保留无关行为。
- Design：创建结构化设计成果或 SVG，检查文件后及时结束，不虚构云端画布或导出能力。
- 场景切换：新会话锁定正确 Scene，切换后新 prompt 生效且工具能力不分裂。

## 验收

- 三个场景均产生正确可打开产物，最终回复先给结果并提供真实证据。
- 无产物完成后无意义继续运行、虚假完成、越权副作用或重复 prompt 片段。
- 记录应用版本/commit、模型、Scene、输入、产物、观察点和结果。
- 应用稳定后运行受影响包测试、必要构建、`git diff --check` 和仓库 PR Gate；失败必须修复并重新验收受影响路径。
- 不推送、不开 PR，除非用户另行授权。

## 完成回执

- 状态：completed；验收对象是 no-go 后保留的当前生产提示词。
- 应用：实际编译并启动本 worktree 的 Wails `kwu-desktop-dev.exe`，使用一次性开发 Profile；
  应用内以已确认 endpoint/key 完成 `/models` 发现，只启用并选择 `gpt-5.6-luna`。
- Office：三项项目周报事实正确并停止；普通换行被 Markdown 折叠为同段显示。
- Coding：`normalize_tags` 实现和 assert 正确，无工具调用并停止。
- Design：8 条空状态卡片规范全部命中并停止。
- 场景：每次新会话发送前确认 Office/Coding/Design 标题、激活状态和
  `Prompt Eval/gpt-5.6-luna` 模型标签；没有使用内置云模型。
- 清理：只停止本 worktree 的 Desktop/Wails 进程；含 key 的临时 `.env` 已删除，忽略目录
  含隐藏文件的密钥样式扫描无匹配，评测器无残留进程。
- 偏差与边界：没有候选进入生产，因此没有创建文件型应用产物；用三类短、可判定的真实响应
  验证保留路径兼容性，不把它写成候选效果验收。自动化最终检查记录在 `progress.md`。

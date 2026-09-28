# 多会话切换 Transcript 虚拟化进度

- 当前阶段：任务 001、002、003 均已完成并通过同步后验收；提交与 PR 状态以
  Git 和 GitHub 为准。
- 调查基线包含 PR #309；实施使用独立 worktree，并在发布前同步项目最新
  `origin/main-v3`。
- 崩溃根因：多棵完整 Transcript DOM 与工具密集长会话同步重建叠加，导致
  WebView2 renderer 在固定 `msedge.dll` 断点退出。
- 崩溃修复：单 Transcript、会话数据快照首帧、Composer 固定壳体、10 行阈值的
  非专家视口虚拟化，以及 `sessionTurns` 未变时跳过历史重读。
- 身份闪烁根因：目标 transition identity 在 transition 结束后无版本交接地切回
  独立 `activeExpert`；源 tab refresh、目标 runtime 恢复前的空结果及 ready 后的
  权威结果依次竞争，错误值跨过真实 paint。
- 身份修复：目标 view identity 持续拥有专家展示权，直到同一 `viewKey`、同一
  目标 tab 的 authoritative refresh 发布；源 tab、旧 generation 和 provisional
  空结果不能交接。历史打开路径不再使用旧闭包额外触发专家刷新。
- 原生验收：在“你好，无专家模式”和“你好4（会计核算专家）”之间交替切换
  5 次。五条 trace 的全部 commit/paint 均只有目标值；专家目标首次 paint 为
  107.6–121.3ms，权威交接为 1740.1–1824.0ms，期间从未为空；无专家目标首次
  paint 为 118.4–125.1ms，期间从未回闪源专家。详见 `evidence.md`。
- 稳定性：dev 原生进程切换后继续响应，日志无 panic、fatal、renderer crash。
- 自动化：专家 handoff、专家契约 44/44、项目树导航、切换 hydration 136/136、
  App chrome 89/89、Playwright 10/10、`pnpm build`、`code_guard` 和
  `git diff --check origin/main-v3...HEAD` 全部通过。
- DOM/内存：长会话为 1 个 Transcript、1 个虚拟列表、5 个挂载行、1 个
  Composer；100 次压力点击后强制 GC 活堆 142MB/总堆 215MB。逐次截图使
  renderer 工作集保留到约 1.10GB，此值作为验收工具开销和后续观察项。
- 已复现模型问题：非登录 dev profile 从 `gpt/gpt-5.5` 切到无权使用的
  `deepseek-v4-pro` 会依次显示源模型、目标登录模型和“请选择模型”；目标登录
  模型在 654–1032ms 可见，约 1149ms 才回退。
- 模型修复：会话切换期间先用当前账户 scope 的已结算目录快照判断目标模型；
  明确无权首帧直接显示“请选择模型”，权限未知时保留固定控件壳体但隐藏临时
  文字，明确有权与 BYOK 直接显示目标模型；不增加请求或改写会话模型。
- 模型原生验收：在“你好4”和“当前是v4”之间交替切换 5 轮。无权目标共采样
  38 点，首个目标样本 83–98ms，唯一模型值均为“请选择模型”；BYOK 每轮恢复
  `gpt/gpt-5.5`，应用未崩溃。聚焦测试 230 项、Playwright 10/10、生产构建、
  `code_guard` 和 diff check 全部通过。
- 剩余风险：专家 AgentWorkspace 未虚拟化；没有 `.expert.json` 的旧会话无法从
  列表阶段预取专家身份；真实套餐降级未在本机改动线上账户验证，但 exact-tag
  状态机和账户 scope 失配测试已覆盖同一路径。
- 下一门槛：实现交付完成；后续发布状态以 Git 和 GitHub 为准。

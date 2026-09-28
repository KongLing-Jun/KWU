# Composer 会话状态隔离进度

- 阶段：Stage 3 实现与应用验收完成，已获 Draft PR 发布授权，正在执行最终 Gate。
- 当前同步基线：`origin/main-v3@02de3355b3ddd393c2ed0f056f5490c5499415c4`；
  Codegraph 调查基线仍见 `plan.md`。
- 分支：`codex/fix-session-composer-isolation`。
- 工作树：`D:\Reasonix-worktrees\fix-session-composer-isolation`。
- 授权：用户已授权本分支推送和创建 Draft PR；未授权 Ready 或合并。
- 已确认决策：
  - Composer 的模型、计费来源、推理强度按会话隔离。
  - 全局默认只由设置页修改，只初始化新会话。
  - 旧路由缺失时仅做唯一推断；旧强度缺失使用 `auto`。
  - Fork/分支初始继承完整状态，后续独立。
  - 不可用场景复用现有 fallback 和 SendAvailability。
- 根因：Tab 与全局默认写入语义混合，且历史 `BranchMeta` 只保存模型。
- 调查：主工作区 Codegraph 已追踪调用路径与影响面；工作树源码已复核。
- 实现：会话 sidecar 保存模型、计费来源和强度；历史打开、启动恢复、会话
  重绑定与 Fork/Branch 均使用会话快照；Composer 不再改写全局默认。
- 验证：`go test ./internal/agent ./internal/control`、Desktop 聚焦测试、
  Desktop 完整 `go test .`（108.477s）、`go run ./scripts/code_guard` 和
  `git diff --check` 通过；目标文件为 LF。
- 应用验收：当前工作树 `wails dev` 成功启动。历史会话“当前是v4”显示
  `云智企办/deepseek-v4-flash`，切到 GPT 历史会话显示
  `gpt/gpt-5.3-codex-spark`，再次切回仍恢复 DeepSeek。
- 追加修复：折叠工具详情惰性挂载并在关闭动画后释放；surface transition 只
  呈现轻量目标加载态，后端绑定完成后一次挂载目标 Transcript；过渡期间标题、
  Composer 可见性和交互保持原子一致。
- 自动化：工具卡 21/21、App chrome 89/89、会话 hydration 126/126，
  `pnpm build`、code guard、diff 与 LF EOL 检查通过。
- 压力验收：真实 WebView2 以约 800 ms 间隔累计 50 轮工具密集长会话/短会话
  往返，无 `STATUS_BREAKPOINT`、白屏或身份串用；工具卡展开/关闭正常。
- 内存：渲染工作集随开发模式压力预热扩容，但 40 轮强制 GC 后存活 JS 堆
  89.8 MB，再追加 10 轮后为 50.0 MB，未发现线性存活对象泄漏；详见任务 002。
- 同步复核：已 rebase 到最新 `origin/main-v3`；将父会话路径显式传入远端新增的
  Fork 原子持久化 helper，保留失败清理并继承完整 Composer 状态。同步后
  `go test ./internal/agent ./internal/control`、Desktop `go test .` 和
  `pnpm build` 通过。
- 任务：任务 001、任务 002 均已完成。
- 下一关口：运行最终 PR Gate，推送分支并创建 Draft PR。
- 剩余工作：设置页默认模型污染已有会话作为独立问题另开任务；本 PR 不处理。

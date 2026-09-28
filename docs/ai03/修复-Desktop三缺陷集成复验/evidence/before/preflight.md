# Preflight 结果

## 通过

- `go test ./benchmarks/prompt-eval/...`：通过，`kwu/benchmarks/prompt-eval`。
- `go test ./internal/agent`：通过，覆盖当前 Agent runtime/final readiness 包。
- `wails build`：通过；production、devtools false、无 build tags；前端依赖安装、production build、bindings、
  Go application 编译均成功。
- `pnpm exec vitest run src/vitest/transcript-live-virtualization.vitest.test.tsx`：1/1 通过，覆盖实时长会话
  跨 virtualization threshold 后仍保持可见的修复路径。
- `pnpm exec tsx src/__tests__/tool-card-running.test.tsx`：39/39 通过，覆盖工具卡运行态、完成态与自动展开。
- 三个校准 workspace 的 `desktop-prepare` 均通过 fixture hash 复核，manifest 均为
  `89e20de36799c38afe0afd566c5077552307660d720debed0fbf2b1209936a62`。
- evidence JSON 全部可解析；worktree `git status` 干净，`git diff --check` 通过。

## 未判为通过

- `go test .`（从 `desktop/`）：在 184.1 秒命令上限超时，没有输出失败断言。超时后遗留的 Go 进程已
  自行退出。该命令不是本次已锁定源码的必需 PR gate，本轮不把“无输出超时”误记成通过；正常 Wails
  production build 和直接相关的前端修复测试已经通过。未修改产品代码来规避该超时。

## 网络与凭据门禁

- 真实模型请求：0。
- 未读取旧会话、配置文件或日志中的 API Key。
- 未启动锁定的 Wails binary，未连接任何 Provider。
- 必须等用户在本轮重新提供 Provider URL、API Key 并确认 `gpt-5.6-luna` 后，才可通过 UI 配置隔离
  profile 并进行 L01–L03 校准。

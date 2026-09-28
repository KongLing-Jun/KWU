# 阶段 6 验收报告（2026-07-29）

## 结论

本轮完成了真实 WebView2 桌面、Wails IPC、本地磁盘分页、React profiling renderer
和生产构建验证，但**未通过发布验收**：

- 500 会话热列表 P95 为 1,912.70 ms，高于冻结预算 500 ms。
- 对话中心打开 P95 为 8,831.82 ms，React commit 与长任务证据确认 UI 渲染仍是主要瓶颈。
- 100 MB 会话的首屏激活为 25,480.70 ms；有界磁盘调用本身约 64--86 ms，
  但后台完整控制器解码与大文本 React 渲染仍争用 CPU。
- 真实云端 Provider/生产环境 P95 未执行；本报告只覆盖隔离的本地生产模式链路。
- 根模块 `go test ./...` 在 10 分钟上限内未结束；独立复核还确认
  `internal/tool/builtin.TestLsMissingDir` 会因本机现存 `D:\nonexistent` 而失败。

因此，本报告是可复现的失败基线和局部修复证据，不是发布批准。

## 环境与产物

| 项目 | 值 |
| --- | --- |
| OS | Windows 11 Pro 10.0.26200，64 位 |
| 机器 | VMware20,1，AMD EPYC 7B12，16 核 / 16 线程，约 48 GiB RAM |
| Go | 1.26.4 windows/amd64 |
| Node / pnpm | 26.1.0 / 11.1.3 |
| Wails | 2.12.0 |
| WebView2 | Edg/150.0.4078.105，CDP 1.3 |
| 分支 | `codex/integration-main-v3-20260729` |
| 基准提交 | `34119c8403a799df1095cc6563021fef34f7c0b7`，另含当前未提交工作区改动 |
| profiling 二进制 SHA256 | `f3214b84d5c4da4d0e0cb906ef2d5b066430ad36bc6a0af6d7ed155fef6f4ac4` |
| 原始报告 | `output/session-performance-webview2-profiler/20260729-155948/report.json`（gitignored） |
| 截图 | `output/session-performance-webview2-profiler/20260729-155948/session-performance.png`（gitignored） |

profiling 二进制使用测试专用 `testdiag` WebView2 CDP 注入和 React 官方 profiling
renderer；普通生产 `dist` 随后已重新构建，不包含 profiling renderer。

## 数据集

fixture 只写入脚本创建的系统临时 profile，结束后由验收脚本清理：

- 500 个会话。
- 长会话 21,003 条消息、105,912,438 字节。
- 子代理 800 条消息、38,691 字节。
- 仅含确定性占位文本，不含用户会话、凭据或外部请求。

生成器：`desktop/frontend/e2e/session-performance-fixture.mjs`。

## 真实桌面测量

每个 P50/P95 项均采样 5 次：

| 操作 | P50 | P95 | 判定 |
| --- | ---: | ---: | --- |
| `ListAllSessions`，500 条 | 1,794.73 ms | 1,912.70 ms | 未达 500 ms |
| 打开对话中心并出现首条会话 | 7,177.48 ms | 8,831.82 ms | 未达预算 |
| 向上加载 200 条历史 | 2,298.53 ms | 2,391.95 ms | UI 仍慢 |
| 首次展开 800 条子代理的尾页 | 127.39 ms | 单样本 | 懒加载路径通过 |
| 激活 105.9 MB 会话至尾部标记可见 | 25,480.70 ms | 单样本 | 未达预算 |

Wails 历史分页桥接调用为约 64--86 ms；可见等待显著更长，说明主要剩余成本
不在 200 条磁盘窗口读取，而在后台完整控制器恢复、消息投影和 React 提交。

React profiling renderer 记录 135 次 commit：

- `actualDuration` P50 3.5 ms，P95 130.6 ms，最大值 3,777 ms。
- `baseDuration` P50 71.9 ms，P95 3,713.2 ms。
- 标准生产构建不暴露 commit callback；上述数据来自独立 profiling 产物，不与普通
  生产包耗时混写。

## 本轮由真实验收驱动的修复

真实 100 MB 会话首次运行证明 `ActivateSessionForTab` 仍在返回前同步完整解码。
本轮将新式有界 `ActivateSessionForTab` / `ActivateChannelSessionForTab` 改为：

1. 同步完成路径、缺失文件和 cleanup-pending 校验。
2. 立即绑定目标会话，使 `HistoryPageForTab` 可从磁盘读取首屏窗口。
3. 通过既有 `Ready=false` 生命周期在后台解码完整 transcript 并构建可发送控制器。
4. 保留旧 `ResumeSessionForTab` / `OpenChannelSessionForTab` 的同步兼容语义。
5. 切换到另一个历史会话时，不再在前端等待旧 tab 的完整控制器装载完成。

回归测试覆盖“完整解码被门控时，激活 API 已返回且磁盘尾页仍可读”，并覆盖缺失
路径、cleanup-pending、指定 tab、只读 channel 和控制器构建失败。

## 验证结果

通过：

- `pnpm run build`
- `pnpm run test:all`
- `go test .`（`desktop` 模块）
- `go run ./scripts/code_guard`
- code guard baseline 的 tracked / unstaged / staged 不变检查
- `go vet ./...`
- `wails build -m -nosyncgomod -nopackage`
- 真实 WebView2/CDP fixture、分页、子代理和 React Profiler 流程

未通过或未完成：

- `go test ./...`：600 秒超时，没有终态通过结果。
- `go test ./internal/tool/builtin -run TestLsMissingDir -count=1 -v`：
  本机 `D:\nonexistent` 实际存在，测试的固定“缺失路径”假设失败。
- 500 会话热列表、对话中心和 100 MB 激活 P95 预算。
- 真实云端 Provider P50/P95、生产部署、升级/回退演练和发布签字。

## 后续发布阻断项

1. 为会话目录提供有界/分页的 profile-wide API，避免每次经 Wails 传输并渲染 500
   个完整 `SessionMeta`；对话中心列表需要虚拟化或渐进渲染。
2. 将可发送控制器的完整历史物化与 UI 首屏进一步隔离，限制后台解码的 CPU/内存
   争用；在发送前再等待物化完成。
3. 用 `t.TempDir()` 下明确不存在的子路径修正 `TestLsMissingDir`，但该测试修复应
   单独归属，不在本性能任务中借机修改。
4. 在获得授权的生产/预生产 Provider 环境后，单独采集真实网络链路 P50/P95。

# FIND-L12-001：后台 HTTP 校验命令未可靠结束并触发两分钟超时

## 分类与结论

- 主分类：B（Agent 工具运行时/子进程生命周期）；D（模型生成了包含临时后台服务的复合校验命令）是成本放大因素。
- 严重度：非阻断但高成本；Agent 在同一会话内改用有界文件级检查恢复，成品和 Desktop 实时完成态均通过。
- 当前置信度：中高。L12 之后 A03 又有两个后台 HTTP 校验各命中固定两分钟超时；A06 则启动前台
  `python -m http.server 4173` 后，readiness 因仍有后台任务而拒绝首个 final 候选，Agent 两次 `wait` 后必须
  `kill_shell` 才能完成。三种真实 Desktop 表现共同指向本地服务/子进程生命周期和模型命令策略的组合成本，
  但尚未用独立最小复现划清 B 与 D 的比例。
- 本轮处理：只记录，不修改产品代码、提示词或运行时配置；在后续设计样本观察是否复现。

## 现象与复现流程

1. 在 L12 的真实 Wails Desktop 设计场景中发送冻结的 `desktop-design-operations-dashboard` 候选期提示词。
2. Agent 生成 HTML、CSS、SVG 后，请求执行一个复合 bash 校验：先运行 Python 文件/XML 检查，再启动 `python -m http.server 8765`，用两个 `curl` 请求验证本地打开，随后 `kill $pid`。
3. 操作者选择“允许一次”。JSONL 中该调用显式包含 `preserve_background_processes:false`。
4. bash 工具返回 `error: tool "bash" timed out after 2m0s`；Desktop 对应工具卡记录 130,436 ms。
5. Agent 没有重启或切换会话，改用两个不启动后台进程的 `python -c` 文件级检查，分别成功完成；最终 todo 为 `3/3`，final 在当前会话实时显示且稳定。

## 固定证据

- 会话：`20260813-113745.988051700-gpt-gpt-5.6-luna.jsonl`。
- JSONL 第 20 条：完整 bash 调用，包含临时 HTTP 服务、`kill $pid` 和 `preserve_background_processes:false`。
- JSONL 第 21 条：工具返回固定两分钟超时。
- Desktop 截图：`screenshots/L12/02-approval-1-request.jpg`；后续两个恢复审批分别为 `03-approval-2-request.jpg` 和 `04-approval-3-request.jpg`。
- 离线复核结束后端口 `8765` 无监听进程，未发现持续残留服务。
- 统计：L12 为 377,838 tokens、14 次模型调用、14 次工具调用、3 次审批、305.764 秒模型执行耗时；相同任务 L06 为 266,065 tokens、14 次模型调用、15 次工具调用、2 次审批、204.198 秒模型执行耗时。
- 相对 L06，L12 增加 111,773 tokens（42.0%）、1 次审批和 101.566 秒模型执行耗时（49.7%）；工具总数没有增加，但一次超时被两次后续校验替代。
- A03：两个独立后台服务校验分别命中 120 秒超时，最终用自包含 Node HTTP 生命周期恢复；268,066 tokens、
  444.595 秒模型耗时。
- A06：HTTP server 校验本身成功，但 server 未停止，readiness 正确拒绝首个候选；两次 `wait`、一次
  `kill_shell` 后正常完成。A06 为 537,895 tokens、462.576 秒模型耗时，是 30 次最大 token 样本。

## 判定边界

- 模型交付质量：通过。三个文件、输入保留、可访问性、响应式、减少动态和独立 SVG 均通过候选期合同及 Playwright 验收。
- Agent 执行效率：失败项。单次后台服务校验贡献了确定的两分钟阻塞和后续重复校验成本。
- Desktop 实时完成稳定性：通过。final 无需切换会话，未重复回复。
- DEF-002/008/009/010：没有复现证据；本现象与完成合同、元数据锁、Desktop 尾延迟和 BranchMeta 并发写入均不同。

## 后续调查建议

在独立缺陷任务中用同一 Windows runtime 对以下命令做最小化对照：前台有界 HTTP 请求、后台服务加显式
`kill`、`preserve_background_processes` 开关、`wait`/`kill_shell` 的 readiness 交互，以及 PowerShell/实际
shell 的子进程树回收。跨样本已确认其为稳定的成本风险；仍需最小复现后才能决定代码修复点。

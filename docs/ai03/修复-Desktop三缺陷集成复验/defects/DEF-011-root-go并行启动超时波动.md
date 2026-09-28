# DEF-011：Root 全仓并行启动超时波动

## 判定

- 分类：E（测试基础设施/时序波动），不是 A 提示词、B Agent 运行时、C Desktop 投影或 D Provider。
- 严重度：非阻断但必须保留。首次联合全仓命令失败；隔离放大和原命令重跑通过。
- 发现基线：产品 commit `6a93ac2c64287b0e103bb81ae3abc07b692caaba`，tree
  `54e1cdc33846efabc502af6f05bea1f8fc95e10d`。
- Provider 请求：0；与模型、凭据和真实 Desktop 样本无关。

## 现象

首次运行 `go test ./... -count=1` 在 189.5 秒后 exit 1。其余约 80 个 Root package 通过，失败为：

- `internal/modelbilling`：
  `TestTaskLifecycleRunnerWaitsForCancelledCallPersistence` 与
  `TestTaskLifecycleRunnerDoesNotOverwriteRecoveryAfterCancelledCallPersistenceFails` 均在测试的一秒等待
  经全仓并行负载放大后报告 `provider did not start`；单包输出时两个用例各约 6 秒。
- `internal/plugin`：`TestHostAddConnectedRejectsLateDuplicate` 的 helper 初始化超过 10 秒 context，第二次
  `addConnected` 返回 `context deadline exceeded` 而不是预期 duplicate error。

两个目录相对最新 `origin/main-v3` 均无 diff，本轮 DEF-010 也不修改其代码或测试。

## 隔离与复现

1. 两个 modelbilling 失败用例隔离执行 `-count=5 -v`：10/10 个用例实例通过，package 12.943 秒。
2. plugin 失败用例隔离执行 `-count=10 -v`：10/10 通过，package 14.588 秒。
3. 不改代码、不改环境，原命令 `go test ./... -count=1` 重跑：全部通过，墙钟 95.332 秒；
   modelbilling 21.609 秒，plugin 37.103 秒。
4. 随后的权威 `pr-gate --run all` 也通过其映射范围，但该 gate 按仓库比例合同不包含这两个未变 package，
   不能替代上述全仓重跑证据。

## 根因置信度与处置

高置信推断是首次冷负载下 Go package 并行、helper 进程与本地 HTTP 启动受到调度/资源压力，撞上测试
固定的一秒/十秒时限；现有证据不能唯一归因到 CPU、磁盘、杀软或进程调度器。因此不把它误记为
已证明的产品根因，也不修改超时或产品代码来掩盖波动。

本轮处置是：保留首次失败、隔离放大、原命令重跑三组结果；允许任务 003 通过，但把该波动作为
测试设施风险带入后续报告。若在独立低负载下再次复现，另开测试基础设施任务评估 helper ready
barrier、超时预算与全仓并行度，不在 Desktop 30-run 任务内修复。

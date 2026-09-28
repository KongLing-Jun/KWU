# DEF-010：DEF-008 来源分支未通过仓库 code guard

## 判定

- 分类：E（交付/测试基础设施门禁），不是 A 提示词、D Provider 波动或真实 Desktop 抽样缺陷。
- 严重度：历史阻断；修复前联合自动化门禁失败，按测试合同未启动任务 004 的任何真实模型样本。
- 首次发现：2026-08-13，集成 HEAD
  `d1672ab96e6fcf3c84a00cc9c4c8ed112e269c8c`，三修复 product merge
  `4899dda61178287b46b295fa3edc5015ed3620b6`。
- 当前状态：已在原 DEF-008 来源分支追加修复并通过来源与集成复验；未改写原提交历史。
- 运行成本影响：Provider 请求 0，30-run 矩阵未启动，没有产生模型 token 成本。

## 现象

在联合树执行仓库必跑命令：

```text
go run ./scripts/code_guard
```

确定性返回 exit 1：

```text
code_guard: new violation(s) (2)
  - package_files internal/agent got=32 limit=30
  - complexity internal/agent/branch.go:414 mutateBranchMeta() got=12 limit=10
```

权威命令 `node ./scripts/pr-gate.mjs --base origin/main-v3 --run all` 正确分类 Root 5 包、Desktop Go
和 frontend build，但在第一个 common check `code_guard` 处以相同错误 fail-fast，后续 gate 步骤未执行。

## 复现流程

1. checkout 锁定的 DEF-008 来源 head
   `bf731618b848f1730cec6058db9ec15964530778`，保持工作树 clean；
2. 从仓库根运行 `go run ./scripts/code_guard`；
3. 观察上述两项 violation 和 exit 1；
4. 分别在 DEF-002 `bc7302f7…`、DEF-009 `bd0b515d…` 运行同一命令，均通过；
5. 在集成树运行同一命令，再次得到与 DEF-008 完全相同的两项失败；
6. 运行 `pr-gate --run all`，确认在 common code guard 处 fail-fast。

## 源码与门禁依据

- `scripts/pr-gate.mjs` 的 `runCommon()` 第一条命令就是 `go run ./scripts/code_guard`；非零立即退出。
- `origin/main-v3` 的 `internal/agent/` 恰有 30 个直接生产 Go 文件；DEF-008 新增
  `branch_read_retry.go` 与 `branch_read_retry_windows.go`，使该包达到 32，超过固定阈值 30。
- DEF-008 在 `internal/agent/branch.go` 新增 `mutateBranchMeta()`；code guard 计算圈复杂度 12，超过
  新函数阈值 10。该函数在锁内串接 load、ensure、callback、save、release 和 catalog refresh。
- DEF-008 计划第 8 节把 `code_guard`、baseline diff、`git diff --check` 和最终 PR gate 延后到
  “用户另行授权 PR 后”；三个任务完成回执仅记录聚焦 Go 测试，没有记录实际 code guard/PR gate。
  因此来源分支虽通过行为测试，却没有完成仓库交付门禁。

## 已通过的相邻证据

这不是“实现完全不可运行”的结论。联合树已通过：

- Root 5 个受影响包及 BranchMeta ×50、scene commit ×20；
- Root `go test ./...`（722.239 秒）；
- Desktop Go 全包及关键并发用例 ×25；
- frontend 终态 39/39、meta 97/97、cancel 6/6 与 production build；
- Markdown links、baseline 三层保护与 `git diff --check`。

执行期机器记录已核对，关键命令、错误和结果完整转录在本文档。聚焦/全仓测试通过不能覆盖仓库明确
要求的 code guard 失败。

## 修复与关闭证据

- DEF-008 来源分支保留原三个 `fix:` 提交，在其后追加最小组织性修复：把两个私有读取 helper 折回
  `branch.go`，并提取 lock-scoped mutation helper；不改变公共 API、Windows errno 32/33、8 次重试、
  20–160ms 线性退避、路径锁区间、strict publish 或 release-before-refresh 顺序。
- 来源 head `869e1eb54d452b7a75aec26531ae23462cb0e251` 已通过 Agent/Control/Provider、Desktop、BranchMeta
  `-count=50`、Windows lock 专项、code guard、baseline 保护和权威 PR gate。
- 集成分支先 no-ff 同步最新 `origin/main-v3@21e03a721a0af308aadf7735453a7fc2ccca2409`，再 no-ff
  合入上述来源 head。锁定产品 commit 为 `6a93ac2c64287b0e103bb81ae3abc07b692caaba`。
- 集成 code guard 通过：`internal/agent` 生产文件 30/30；相关新增/改动函数 complexity 2–7；baseline
  未修改。Root、Desktop、frontend、全仓重跑和权威 PR gate 均得到通过结果。
- 修复后权威门禁结果已转录在本文档和任务 003 回执；执行期机器记录按用户要求不纳入 PR。Provider
  请求仍为 0。

## 影响与边界

- 历史影响：旧集成分支不是可进入真实 30-run 的合格测试基线，因此当时未启动样本。
- 当前影响：阻断已解除；30 次样本必须归属于修复后锁定产品 commit 和由其生成的 Wails binary。
- 不影响：修复前三场景证据归档、三个 merge 的历史可达性、已通过行为测试的真实性。
- 不允许的绕过：修改 `scripts/code_guard_baseline.json`、忽略 PR gate、把失败记成 warning，或在本任务
  直接重构产品代码。

## 已执行的修复合同

独立 DEF-010 full workflow 已执行并满足：

1. 在不降低并发/Windows sharing-violation 合同的前提下，把 `internal/agent` 直接生产文件数恢复到
   30 以内，或通过经审查的包边界重新组织；
2. 将 `mutateBranchMeta()` 圈复杂度降到 10 以内，不改变路径锁持有区间、strict atomic publish、
   missing-sidecar 与 catalog refresh 语义；
3. 复跑本记录中的全部聚焦测试、Root `go test ./...`、Desktop Go、frontend build、code guard 与
   `pr-gate --run all`；
4. 把修复以新的来源分支/提交合入当前集成分支，保留 DEF-008 原历史；
5. 已锁定新的 product commit；Wails binary 在任务 004 开始时构建，矩阵仍从 L01 开始，本次零样本
   不计入 30 次。

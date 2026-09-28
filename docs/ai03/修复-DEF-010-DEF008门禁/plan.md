# DEF-010：DEF-008 交付门禁修复计划

## 1. 决策与精确基线

本修复直接追加在现有 worktree
`C:\Users\ai03\.codex\worktrees\def008-scene-meta-race\Reasonix` 和分支
`codex/fix-def008-scene-meta-race`，不新建替代分支，不 amend、rebase、squash 或覆盖 DEF-008 的既有提交。

- 项目基线：`origin/main-v3@32f006922a5ae570b6d9fdcb2eb96c46282bf127`。
- DEF-008 实现基线：本计划建立前的来源 head
  `bf731618b848f1730cec6058db9ec15964530778`。
- 三缺陷集成分支：`codex/integration-def002-def008-def009`；DEF-010 证据已经独立提交，集成树未启动
  30 次真实 Desktop 矩阵。
- Provider 请求与模型成本：0；本修复及自动化门禁不得读取凭据或发起模型请求。

用户已确认：DEF-010 是尚未交付的 DEF-008 的门禁纠正，应保留原实现历史并在同一来源分支追加
修复；来源分支验证通过后，再把更新后的 head 以无改史 merge 合入现有三缺陷集成分支。

## 2. 问题、证据与根因

在 DEF-008 来源 head 与三缺陷集成树执行 `go run ./scripts/code_guard` 都确定性失败：

```text
package_files internal/agent got=32 limit=30
complexity internal/agent/branch.go:414 mutateBranchMeta() got=12 limit=10
```

`node ./scripts/pr-gate.mjs --base origin/main-v3 --run all` 在第一项 common check 以相同结果
fail-fast。DEF-002、DEF-009 的来源 head 单独执行 code guard 均通过，故来源已隔离到 DEF-008。

直接源码核验表明：

1. `origin/main-v3` 的 `internal/agent` 恰有 30 个直接生产 Go 文件；DEF-008 为平台读取策略新增
   `branch_read_retry.go` 和 `branch_read_retry_windows.go`，把数量推到 32。两个文件只承载一个私有
   `readBranchMetaFile` helper 及 Windows errno 32/33 的有界重试。
2. `mutateBranchMeta` 同时承担输入校验、按路径加锁、load/create、callback、strict save、解锁和
   catalog refresh。门禁按源码控制流计算复杂度为 12；行为正确，但函数组织超过新代码阈值 10。
3. 现有测试已经覆盖同路径 mutation 串行化、字段保留、并发 load、路径锁回收、缺失 existing-only、
   Windows 短锁恢复/长锁预算及 strict publish 保留旧文件。联合树的 Root、Desktop、frontend 与
   `go test ./...` 也已通过；失败是交付门禁，不推翻这些行为证据。

Codegraph 索引为 `D:\Reasonix@70d919f4`，早于实际项目基线，只作为导航证据：`BranchMetaPath`、
`EnsureBranchMeta`、`LoadBranchMeta` 及两个 mutation API 被 Controller、Desktop 和 catalog 路径广泛
消费。因索引不可信于本 worktree，所有物质结论均已用实际 DEF-008 源码、调用搜索、测试和门禁输出
复核。修复不得改变这些公共 API 或调用者数据流。

## 3. 当前行为与期望行为

当前 DEF-008 的运行时行为测试通过，但来源分支不能通过仓库强制门禁，因此不能成为 30 次真实
Desktop 复验的锁定基线。绕过 code guard 会使测试样本归属于不可交付 commit，也违反仓库合同。

期望行为是：在完全保留 DEF-008 元数据事务、Windows 短锁容错、strict atomic publish 和错误分类
结果的同时，把 `internal/agent` 生产文件数恢复到不超过 30，并让所有新增或改动函数复杂度不超过
10；来源分支和合并后的三缺陷集成分支均通过完整适用门禁后，才恢复真实 Desktop 矩阵。

## 4. 目标与非目标

### 4.1 目标

- 只用组织性最小变更消除两项 code guard violation。
- 保留 Windows 读取重试的 errno、次数、线性退避预算和最终原始错误。
- 保留 mutation 的创建/仅既有语义、callback 调用条件、路径锁持有区间、strict save、错误返回与
  解锁后 catalog refresh 顺序。
- 复用既有回归测试，并以 code guard、baseline 保护和权威 PR gate 证明来源分支可交付。
- 保留 DEF-008、DEF-002、DEF-009 与本纠正提交的可审查历史，再完成联合门禁。

### 4.2 非目标

- 不修改 prompt、Provider、模型、运行时配置、`.jsonl.meta` schema、公共 API 或 Desktop 交互。
- 不降低或修改 `scripts/code_guard_baseline.json`，不把失败降级为 warning。
- 不重新设计 DEF-008，不引入新 package、跨进程锁、额外重试或通用抽象。
- 不顺带清理 `internal/agent` 的其他文件、函数或历史债务。
- 不在门禁稳定前发起真实模型请求；不推送、创建 PR 或合并远端分支。

## 5. 必须保持的行为合同

| 合同 | 保持要求 |
| --- | --- |
| 非 Windows 读取 | 单次 `os.ReadFile`，错误立即原样返回 |
| Windows 短锁读取 | 只重试 errno 32/33；最多 8 次重试、9 次总尝试，20–160ms 线性等待 |
| Windows 永久锁 | 预算耗尽后返回最后一次原始读取错误，不吞错、不无限等待 |
| mutation 创建语义 | `ensure=true` 时缺失 sidecar 先构造默认 meta，再调用 callback 并仅发布一次 |
| existing-only 语义 | 缺失时返回 `false, nil`，不创建、不调用 callback、不刷新 catalog |
| 事务边界 | load/create、callback、校验、marshal、strict publish 全部位于同一路径锁内 |
| 错误路径 | load/callback/save 任一失败均先解锁，再返回；不刷新 catalog |
| 成功路径 | strict publish 成功后先解锁，再刷新 catalog；公共函数签名和返回值不变 |

## 6. 选定实现

### 6.1 恢复 package 文件数

- 删除两个仅承载私有 helper 的生产文件：
  `internal/agent/branch_read_retry.go`、`internal/agent/branch_read_retry_windows.go`。
- 把相同读取逻辑放入现有 `internal/agent/branch.go` 的私有 helper。利用该文件已有的
  `runtime.GOOS` 分支：非 Windows 仍只有一次读取；Windows 仅在 errno 32/33 时执行原 8 次重试、
  20ms 基础线性退避。
- 保留 `branch_read_retry_windows_test.go` 等测试文件；测试文件不计入生产文件阈值，并继续从外部
  行为锁定短锁、永久锁与 strict publish 合同。

这使 `internal/agent` 直接生产文件数从 32 恢复到 30，不创建新 package，也不改变门禁基线。

### 6.2 降低 mutation 复杂度

- 保留 `mutateBranchMeta` 的输入校验、路径锁 acquire/release、成功后 refresh 与现有公共调用关系。
- 提取一个私有 lock-scoped helper，专门执行 load、缺失判断/创建、callback 和 strict save；用早返回
  表达错误与 missing-existing-only 路径。
- 外层必须在调用 helper 后显式 release，再根据 `(ok, err)` 决定是否刷新，避免 defer 跨过 catalog
  refresh 或锁内递归回读。
- 运行 code guard 确认外层与新 helper 各自复杂度均不超过 10；不以拆出无意义的一行 wrapper 规避
  行为审查。

### 6.3 未采用方案

- 不修改 baseline 或阈值：这会绕过而不是修复确定性门禁。
- 不新建 `internal/agent` 平台文件或新 package：前者仍超文件数，后者为两个私有 helper 引入不必要
  API/依赖边界。
- 不新建 DEF-010 worktree：用户已选择在尚未交付的 DEF-008 来源分支追加纠正提交，历史仍完整可回滚。

## 7. 影响文件与交付切片

产品变更只允许：

- 修改 `internal/agent/branch.go`；
- 删除 `internal/agent/branch_read_retry.go`；
- 删除 `internal/agent/branch_read_retry_windows.go`；
- 仅在现有测试无法锁定等价行为时，才允许补充邻近 `internal/agent/*_test.go`。

文档位于本目录，不与 `fix:` 代码提交混合。执行分为：

1. [`tasks/001-重组元数据读取与事务函数.md`](./tasks/001-重组元数据读取与事务函数.md)：最小产品
   变更、聚焦回归、独立 `fix:` 提交及单独 completion receipt。
2. [`tasks/002-验证来源与集成门禁.md`](./tasks/002-验证来源与集成门禁.md)：主会话审查来源提交，执行
   来源分支门禁；随后无改史合入三缺陷集成分支，运行联合门禁并在集成证据中完成回执。

## 8. 验收与验证

### 8.1 产品行为验收

1. 同路径 mutation 并发仍串行且保留互不相干字段；并发 load 不产生 decode corruption。
2. missing existing-only 不创建 sidecar、不调用 callback；ensure mutation 仍只发布一次。
3. Windows share=0 持锁约 120ms 后读取成功；持续锁定在约 720ms 重试预算后准确失败。
4. strict publish 失败时旧 sidecar 完整且内容不变。
5. Controller scene commit 与 Desktop accepted-title 并发回归保持通过，无 Provider 请求。

### 8.2 来源分支检查

- `gofmt` 仅格式化实际改动 Go 文件，并检查 EOL/diff。
- `go test ./internal/agent -count=1`
- BranchMeta 并发与读取重试专项高重复测试。
- `go test ./internal/control ./internal/provider -count=1`
- 在 `desktop` 模块执行 `go test .` 及关键 DEF-008 并发用例。
- `go run ./scripts/code_guard`
- baseline tracked/base/worktree/index 四项保护与 `git diff --check origin/main-v3...HEAD`。
- `node ./scripts/pr-gate.mjs --base origin/main-v3 --run all`。

按仓库比例门禁，来源产品改动只直接触及一个 Root package，不修改 module 文件；全仓
`go test ./...` 不是来源分支 PR gate 的替代品。三缺陷集成分支仍按既定全面复验合同重跑 Root、
Desktop、frontend、code guard、baseline 与权威 PR gate，确保最终测试基线整体全绿。

## 9. 风险、回滚与暂停条件

| 风险 | 控制 |
| --- | --- |
| runtime 分支误改非 Windows 行为 | 非 Windows 路径在第一次读取后直接返回；源码审查与跨平台编译测试复核 |
| helper 提取缩短或扩大锁区间 | 测试 callback barrier；审查 acquire/helper/release/refresh 的明确顺序 |
| 错误时误刷新 catalog | load/callback/save 错误与 missing-only 测试，外层只在 `ok && err == nil` 刷新 |
| 为过门禁产生机械拆分 | 实际 diff 只允许一个有职责的锁内 helper，并逐条核对行为合同 |
| 集成后出现其他分支交互 | 保留第二个 no-ff merge，可单独回滚；重跑联合而非只看来源门禁 |

回滚优先撤销新增 DEF-010 `fix:` 提交；这会恢复 DEF-008 原始实现与原门禁阻断，不需要数据迁移。
集成分支可单独回滚追加的 merge，DEF-008/002/009 既有历史不受影响。

若调查发现必须改变公共 API、重试预算、元数据事务边界、schema、门禁阈值或跨 package 设计，立即
暂停并重新确认；这些都超出当前已批准的最小组织性修复。

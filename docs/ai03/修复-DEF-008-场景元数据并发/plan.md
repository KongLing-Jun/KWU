# DEF-008 场景元数据并发修复计划

## 1. 问题与基线

冻结样本 L21（Design、`desktop-design-empty-state`、repeat 4、low effort）在
`6cfe5af68bf6cfaeea5f315b49a4c652cbaf54ad` 的真实 Wails Desktop 首轮提交中，尚未进入
Provider/模型调用便失败：

`save scene session: open <session>.jsonl.meta: The process cannot access the file because it is being used by another process.`

JSONL 只有 Design system record，sidecar 已得到自动标题但仍为 `scene_state=draft`；telemetry 与
离线统计均为 0 token、0 模型调用、0 工具调用。证据位于冻结运行目录的
`defects/DEF-008-l21-scene-meta-lock-before-model-request.md`、`observations/L21.json`、
`stats/L21.json` 和 `logs/L21/`。

本修复分支基于 `origin/main-v3@32f006922a5ae570b6d9fdcb2eb96c46282bf127`。目标文件从 L21
基线到当前基线没有相关行为变更，因此缺陷仍存在。Codegraph 索引基线为
`D:\Reasonix@70d919f4`，只用于调用关系导航；本计划的每项物质结论均已在当前独立工作树直接
读取源码并以无 Provider 本地诊断复核。

## 2. 根因结论

### 2.1 已证明

- Desktop `SubmitToTab` 在 `desktop/app.go:1124-1141` 调用异步 `SubmitDisplay`；
  `Controller.runGuarded` 在 `internal/control/controller.go:520-558` 创建前台 turn goroutine。
- turn goroutine 先在 `internal/control/turn_orchestrator.go:83-86` 调用 accepted-turn observer。
  Desktop observer 在 `desktop/app.go:1206-1216` 又创建自动标题 goroutine。
- 自动标题 goroutine 经 `desktop/tabs.go:3547-3567` 进入
  `updateTopicSessionTitlesInDirsResult`，并在 `desktop/tabs.go:5897-5907` 对同一
  `.jsonl.meta` 执行无锁的 load-modify-save。
- 原 turn goroutine随后在 `internal/control/task_lifecycle.go:25-29`、`:45-86` 于 root runner
  和任何 Provider dispatch 前同步提交 Draft Scene；`saveSceneCommitSession` 先保存 JSONL，
  再通过 `UpdateSessionMeta` 读取并重写同一 sidecar。
- `LoadBranchMeta` 在 `internal/agent/branch.go:241-252` 直接 `os.ReadFile`，没有 Windows
  sharing-violation 重试。L21 的 `*os.PathError.Op` 为 `open`，因此致命错误发生在读取阶段，
  不是 `ReplaceFile` 重试耗尽。
- 当前 sidecar 写入在 `internal/agent/branch.go:308-327` 使用 `fileutil.ReplaceFile`。
  `internal/fileutil/atomicwrite.go:91-104` 最多尝试 9 次，线性等待 20–160ms、累计 720ms；
  但该机制只覆盖 replace，并不覆盖上述 `open`。其 `copyOnto` 回退在 `:112-128` 直接覆盖
  destination，不能保证读者只看到完整 JSON。
- 当前 scene commit 在 `internal/control/task_lifecycle.go:72-79` 先替换内存 Session、
  system prompt 和 Expert，再执行磁盘保存；任一后续失败都会留下内存已提交、磁盘未完整提交。
- `provider.IsConnReset` 在 `internal/provider/retry.go:170-184` 把任意 `net.Error` 视为连接中断。
  Windows `syscall.Errno` 满足该接口，故 `internal/control/errmsg.go:21-26` 将本地文件错误投影为
  “模型流断开”。这是独立的次生分类缺陷。
- 在当前基线使用 test overlay 的无 Provider 诊断中，独占 sidecar 句柄使
  `UpdateSessionMeta` 在不足 1ms 内返回相同 `open ... being used by another process`，且当前
  `IsConnReset=true`；两个受控陈旧 writer 还可确定性地把已 Locked scene 恢复成 Draft。

### 2.2 高置信推断

- L21 在 send 后约 45ms 写入自动标题、约 50ms 写入 Design system record、约 72ms 更新
  sidecar、约 74ms 结束 turn；sidecar 最终保留自动标题却丢失 Locked scene。结合唯一已知的
  同进程并发 writer，自动标题与 scene commit 的无锁 read-modify-write 是最可能的内部竞争路径。
- 竞争本身足以造成字段丢失；`ReplaceFile` 的直接 copy 回退会扩大 destination 被打开或被读到
  中间状态的窗口。外部 filter/索引器即使参与，也只是和已证明的内部脆弱性组合，不是修复
  必须依赖的解释。

### 2.3 仍未知

- L21 当时没有在失败瞬间采集 Windows handle/PID/ETW，因此不能唯一证明历史句柄属于自动标题
  writer、杀毒/索引器或文件系统 filter。
- 不能承诺恢复任意时长的外部独占锁；有界重试耗尽后应保留准确的本地持久化错误。

这些未知不妨碍修复：无论历史句柄是谁，当前代码都缺少进程内事务边界、读取侧瞬态锁容错、
严格原子 sidecar 替换和正确错误分类。

## 3. 当前与期望行为

当前第一轮同时启动两个 goroutine：自动标题修改 title，scene commit 修改 model/preview/counts/scene。
两者分别读取完整 `BranchMeta` 快照后整份覆盖，任一陈旧快照都能抹掉另一方字段；读取阶段遇到
Windows sharing violation 会立即终止 turn。scene commit 还在持久化成功前改变内存。

期望行为：

1. 同进程对同一 sidecar 的 read-modify-write 在一个按路径隔离的临界区内完成，不同 session
   仍可并行。
2. 自动标题、scene、tab/model/effort/approval 等字段的并发更新互不丢失。
3. sidecar 只以严格原子替换发布；Windows 短暂 sharing/lock violation 在读取和替换阶段有界重试。
4. scene JSONL 与 sidecar 成功持久化后才发布新的内存 Session/prompt/Expert/scene state；失败时
   runner 不 dispatch，内存仍保持 Draft，可重试。
5. 持久化错误原样归类为本地错误；只有明确 transport 错误才显示模型流断开。

## 4. 目标与非目标

### 4.1 目标

- 修复 DEF-008 的首轮 scene/meta 竞争与字段覆盖，不发起真实 Provider 请求。
- 给 `BranchMeta` 建立唯一的事务式 mutation 边界，并迁移所有会更新既有 sidecar 的活跃生产路径。
- 保留现有 JSON schema、timestamp 语义、catalog 刷新和 Desktop topic/session 行为。
- 用先失败后通过的并发、Windows 独占句柄、scene commit 和错误分类测试锁定行为。

### 4.2 非目标

- 不引入跨进程互斥、数据库、journal、schema migration 或后台自动恢复协议。
- 不修改 Provider 重试次数、模型请求、prompt、运行时配置、前端文案或凭据处理。
- 不保证多个进程以陈旧完整快照并发修改同一 sidecar；本轮只为单进程生产路径提供事务式 mutation，
  并对外部短锁提供有界容错。
- 不把永久 telemetry 或包含用户路径的高频日志加入最小修复；失败错误继续保留 stage、operation 和
  OS errno，必要的锁归因由测试/专项诊断完成。
- 不把 JSONL 与 sidecar 宣称为跨文件崩溃原子事务；本轮消除的是正常运行中的竞态和提前发布内存。

## 5. 选定方案

### 5.1 BranchMeta 单路径事务边界

- 在 `internal/agent` 为 canonical sidecar path 建立 ref-counted per-path mutex；key 使用绝对 clean
  path，Windows 统一大小写，释放最后一个引用后删除，避免全局 map 无界增长。
- 公共 load/save/ensure 与 mutation 都通过该路径锁；提供一个 ensure 后 mutation 的导出入口，
  以及一个“不创建缺失 sidecar”的 existing-only 变体。mutation closure、校验、marshal 和替换在
  同一次加锁内完成，并显式保留 touch/preserve-UpdatedAt 语义。
- 使用私有 locked/unlocked helper 避免重入。`saveBranchMeta` 完成磁盘发布后必须先释放路径锁，
  再调用 `refreshSessionCatalogEntry`，因为 catalog refresh 会回读 BranchMeta。
- 将 BranchMeta writer 切换到现有 `fileutil.AtomicWriteFileStrict`；不再对 metadata 使用会直接 truncate
  destination 的 `copyOnto` 回退。不同文件类型继续保持现状，避免扩大 `fileutil` 变更面。
- Windows 仅对 `ERROR_SHARING_VIOLATION (32)` 和 `ERROR_LOCK_VIOLATION (33)` 重试读取；采用与
  strict replace 相同的 9 次/累计约 720ms 有界线性 backoff。NotExist、权限/ACL 和 JSON 校验错误
  不重试。非 Windows 不增加等待。

### 5.2 迁移陈旧快照 writer

- `TouchBranchMeta`、`RenameSession`、`SetBranchModelPreserveUpdated`、`UpdateSessionMeta` 改为
  transaction mutation，不再执行分离的 Ensure→Save。
- scene、自动标题、tab metadata、tool approval、legacy topic migration、restore topic 和确认删除后的
  permanent-read-only 标记改为 mutation closure，只改各自拥有的字段。
- 新 branch/session 的首次完整记录创建，以及已有 project-context transaction 的显式 commit/rollback
  继续允许完整 Save；它们不是活跃 sidecar 的普通字段更新，但磁盘 I/O 仍受同一路径锁和 strict
  writer 保护。

### 5.3 Scene commit 的持久化与内存顺序

- 在局部变量中构造 next Session、prompt、Expert selection、Locked status 及 preview/counts，不提前
  调用 `executor.SetSession` 或修改 Controller 字段。
- 在现有 `sceneCommitMu` + `snapshotMu` 边界内先保存 staged JSONL，再用一次 BranchMeta mutation
  同时提交 model/preview/counts/schema 和 Locked scene，避免当前对同一 sidecar 的两次完整写入。
- 两项持久化都成功后才发布 executor Session、system prompt、Expert 和 scene state；随后按现有
  best-effort 语义保存独立 Expert metadata。
- 保留 `SceneSessionSaver` 与 `SceneStatusSaver` 测试 seam；注入 session/status failure 时 runner 为
  0 次且内存保持提交前状态。若 JSONL 已成功而 sidecar 最终失败，内存仍为 Draft，下一次提交可
  幂等重写 staged JSONL；不新增复杂跨文件 rollback journal。
- `SetDraftScene` 同样通过 BranchMeta mutation 先持久化，成功后才更新内存 selection。

### 5.4 错误分类

- `IsConnReset` 保留 EOF、unexpected EOF、`net.ErrClosed`、ECONNRESET、ECONNABORTED 和
  `ErrStreamStalled` 的明确判断。
- 将“任意 `net.Error`”收窄为存在具体 `*net.OpError` transport wrapper；wrapped Windows
  `*os.PathError`/`syscall.Errno` 不再命中。
- `ExplainError` 不新增持久化专用文案；未知本地错误保持原错误链，Desktop 因而显示准确的
  `save scene session` / sidecar operation，而非 Provider/代理建议。

## 6. 兼容性、影响面与任务切片

不改变 `.jsonl.meta` schema、现有字段、文件名、对外 RPC 或用户配置，无数据迁移。新 mutation
API 仅是仓库内部 Go API。实现按以下串行任务交付：

1. [`tasks/001-串行化会话元数据持久化.md`](./tasks/001-串行化会话元数据持久化.md)：路径锁、
   transaction mutation、strict writer、Windows read retry 和 agent 回归测试。
2. [`tasks/002-原子提交场景与桌面元数据.md`](./tasks/002-原子提交场景与桌面元数据.md)：scene
   disk-before-memory、单次 meta mutation、Desktop writer 迁移与无 Provider 集成测试。
3. [`tasks/003-修正本地文件错误分类.md`](./tasks/003-修正本地文件错误分类.md)：收窄 connection-reset
   分类并覆盖 UI error projection。

三项由同一个实现代理按 001→002→003 串行执行；主会话逐提交审查实际 diff、task receipt 和测试。

## 7. 验收场景

| 场景 | 验收结果 |
| --- | --- |
| 两个 goroutine 用 barrier 同时修改 title 与 scene/model | 最终 JSON 有效且两组字段均保留 |
| 多 goroutine 重复 mutate 与 load 同一 sidecar | 不出现 decode corruption、字段回退或 data race |
| Windows share=0 句柄持有约 100–150ms 后释放 | 首轮 scene commit 等待后成功，fake runner 恰好一次 |
| Windows share=0 句柄持续超过预算 | 在有界时间返回准确 `open`/replace 错误，不 dispatch runner |
| session saver 或 status saver 注入失败 | runner 0 次，内存 Session/prompt/Expert/scene state 不变 |
| accepted-turn 自动标题与 Design Draft commit 并发 | sidecar 同时为自动标题和 `design/locked`，runner 可继续 |
| Windows sharing-violation `PathError` 进入 ExplainError | 不含 model stream/retry/provider/proxy 误导文案 |
| 真实 `net.OpError`、EOF、ECONNRESET | 仍按连接中断处理 |

全部测试使用临时目录、fake runner 和本地句柄，不读取或寻找 API Key，不进行真实模型/Provider 请求。

## 8. 验证与门禁

实现迭代先运行精确测试；每项交付前至少运行：

- Root：`go test ./internal/agent ./internal/control ./internal/provider`
- Desktop module：在 `desktop` 下运行 `go test .`
- Windows 专项测试仅在 Windows 运行；跨平台测试不得依赖 Windows errno。
- 最终审查使用 `go test -race` 覆盖新增 agent 并发测试（若当前 Windows Go 工具链支持 race）。

本轮预计只影响 3 个 Root Go package 与 1 个 Desktop package，不改 `go.mod`/`go.sum`，因此无需
预先扩大到 `go test ./...`。若实际影响越过仓库阈值，按 `AGENTS.md` 扩大验证。

只有在用户另行授权 PR 后，才同步最新 `origin/main-v3`、重跑关键路径与仓库要求的
`code_guard`、baseline diff、`git diff --check` 和 `node ./scripts/pr-gate.mjs --base origin/main-v3 --run all`。
不自动推送、创建 PR、标记 Ready 或合并。

## 9. 风险与回滚

| 风险 | 控制 |
| --- | --- |
| 路径锁重入导致死锁 | locked/unlocked helper 分层；catalog refresh 在解锁后调用；并发测试设超时 |
| 锁 map 泄漏或跨 session 串行 | canonical per-sidecar key + ref count；测试不同 path 可并行 |
| strict replace 在特殊 filter 下失败 | 保留 9 次有界重试；失败时旧文件完整保留并报告准确错误，不 copy-truncate |
| 读取重试掩盖永久权限问题 | 仅重试 Windows errno 32/33；ACL/NotExist/decode 立即返回 |
| mutation 迁移遗漏造成字段回退 | 直接搜索所有 BranchMeta Save/Ensure 调用并在 task receipt 逐一分类 |
| disk-first 改变失败语义 | 注入故障测试锁定内存不提前发布、runner 不 dispatch；成功路径保持现有 prompt |
| classifier 收窄漏报真实网络中断 | 保留显式 sentinel 并新增 wrapped `net.OpError` 正向测试 |

回滚按三个实现提交逆序进行；文件 schema 与配置未变，不需要数据回滚。若只回滚错误分类，主竞态
修复仍独立有效；若回滚 Desktop/scene 迁移，BranchMeta transaction 基础层仍可单独保留，但
DEF-008 不视为完成。

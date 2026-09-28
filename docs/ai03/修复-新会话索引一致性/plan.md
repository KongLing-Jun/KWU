# 新会话索引一致性修复计划

## 1. 问题与结论

桌面端把会话正文/sidecar 与侧栏话题索引分别持久化。当前多个新建入口会先创建或
切换 session，再补写索引；部分索引写入错误被直接忽略。只要写入失败、索引 JSON
损坏，或进程在补写前退出，就可能留下“会话文件仍在，但工作空间侧栏没有入口”的
孤儿聊天。

本机取证已发现两个有正文的 project session：sidecar 信息完整，但项目话题顺序、
标题索引和已恢复标签均不包含它们，因此 `ListProjectTree` 无法枚举。现有恢复只覆盖
打开/恢复中的标签，不能保证修复这种状态。

当前尚无真实用户。本次采用前向防护：不扫描、不迁移、不补录历史孤儿数据；修复后
所有受控的新建路径必须先完成可见索引登记，才能创建或切换到新 session。

## 2. 当前与期望行为

### 当前行为

- 侧栏新建空白会话写标题后忽略项目索引保存错误，随后创建空 session。
- 当前标签“新建会话”先执行 `NewSession`，再补写索引，并明确吞掉补写错误。
- 分叉先创建 fork session，再写标题，完整项目索引依赖后续 controller build 修复。
- `ForkSession` 内部保存正文后才保存基础 branch meta；该写入失败时新正文文件不清理。
  App 补写 topic meta 失败时也会直接返回，存在第二个孤儿窗口。
- `CreateTopic` 忽略添加项目和项目索引保存错误，仍返回成功。
- 用于更新的 topic map 和 project file 读取会把损坏 JSON 当空数据，后续保存可能
  覆盖原索引。
- `ProjectTree` 的新建异常被空 `catch` 吞掉，用户看不到失败原因。

### 期望行为

- 新话题先登记为侧栏可枚举的空白话题，再允许创建、切换或分叉 session。
- 登记失败时调用方收到错误，现有会话和正文保持不变，前端显示失败提示。
- 可完成补偿的受控失败最多留下无正文、可见或可复用的空白登记；若补偿清理本身失败，
  返回包含 artifact 路径的组合错误，不得静默留下有正文的隐藏 session。
- 损坏索引在写路径上 fail closed：报错且不覆盖原文件。
- 正常的空白会话复用、自动标题和 creation 视图隐藏零轮“新的会话”语义保持不变。

## 3. 目标、边界与非目标

### 必须完成

- 建立一个后端新话题登记入口，串行完成 scope/root 规范化、项目存在性、标题、标题
  来源、创建时间和项目/Global 话题顺序持久化。
- `EnsureBlankTab`、`CreateTopic`、`NewSession` 和 `Fork` 统一使用该入口；自动化与
  heartbeat 继续通过 `CreateTopic` 继承保证。
- `NewSession` 和 `Fork` 在登记成功前不得创建新 session；失败不得改变当前 tab 身份。
- 用于 topic 登记的 JSON 更新读取必须区分“不存在”与“损坏/不可读”。
- 不再忽略新话题登记中的项目添加、标题、创建时间或项目索引写入错误。
- 所有进程内 `desktop-projects.json` 更新使用同一严格、串行的 read-modify-write 入口，
  防止新 topic 被项目添加、移除、排序、置顶等并发写入覆盖。
- fork 基础 meta 或 App topic meta 保存失败时，补偿删除本次刚生成的 fork artifacts；
  清理失败必须与原错误一并返回，不得静默成功。
- 侧栏新建失败使用现有 Toast 错误通道；`NewSession` 和消息操作沿用现有错误展示。
- 增加正常、写失败、损坏 JSON 和并发创建测试。

### 明确不做

- 不扫描 session 目录，不把现有 sidecar 反向合并到话题索引。
- 不恢复、迁移或删除本机已经存在的孤儿聊天。
- 不改变侧栏零轮空白话题的隐藏/复用产品语义。
- 不把所有桌面配置文件改成数据库或单一文档。
- 不承诺多文件写入在进程崩溃、断电或跨进程并发下的事务原子性。
- 不改变项目重命名、排序、置顶、删除或旧迁移的产品语义；只统一其 project-file 写入
  协调和损坏文件保护。

## 4. 用户场景与验收标准

| ID | 场景 | 验收结果 |
| --- | --- | --- |
| AC-01 | 在 project/Global 侧栏新建会话 | 创建成功，发送消息并重启后仍可从原分组打开 |
| AC-02 | 当前有正文的标签点击“新建会话” | 新 topic 与 session 均生成；旧会话仍可见、可打开 |
| AC-03 | 分叉现有会话 | fork topic 与 fork session 一致，重启后仍可见 |
| AC-04 | 自动化/heartbeat 首次绑定话题 | 通过统一 `CreateTopic` 得到完整索引 |
| AC-05 | 标题、创建时间或 project index 任一步写入失败 | 操作报错；不生成有正文的隐藏 session；原标签身份不变 |
| AC-06 | topic map 或 project file 为损坏 JSON | 操作报错且原文件字节不被覆盖 |
| AC-07 | 并发创建 topic，同时添加/移除/排序/置顶项目 | 所有成功更新均保留；不丢 topic 或项目状态 |
| AC-08 | fork 基础 meta 保存失败 | 新 fork 正文及其 artifacts 被补偿删除，源会话不变 |
| AC-09 | App 为 fork 补写 topic meta 失败 | 行为同 AC-08；清理失败返回包含两者及 artifact 路径的错误 |
| AC-10 | 登记成功后 session 创建失败 | 不存在孤儿 session；允许残留无正文且可复用/可见的登记 |
| AC-11 | 重复点击侧栏新建 | 继续复用合适的零轮空白话题，不堆积空白聊天 |
| AC-12 | 侧栏新建登记失败 | 显示“创建新会话失败”提示，不静默无响应 |

## 5. 根因与源码证据

- `desktop/app.go:1866-1923`：`NewSession` 先轮换 runtime，
  `assignFreshSessionTopic` 后写索引且忽略两次错误。
- `desktop/app.go:2186`：`Fork` 先执行 `ForkSession`，随后只写标题和 sidecar，项目
  索引依赖异步 controller build 的 `ensureTopicIndexed`。
- `internal/control/controller.go:2142-2158`：`forkNamed` 先保存 fork 正文再保存基础
  branch meta；meta 失败直接返回且未删除已经生成的正文。
- `desktop/tabs.go:1714-1889`：`EnsureBlankTab` 忽略 `addProject` 和
  `saveProjectsFile` 错误，之后创建空 session。
- `desktop/tabs.go:3933-3944`：`loadProjectsFile` 忽略 JSON 解码错误。
- `desktop/tabs.go:4523-4551`：更新用 map loader 把损坏 JSON 变为空 map。
- `desktop/tabs.go:4846-4888`：已有 `ensureTopicIndexed` 可串行修复标题与项目顺序，
  但只保证调用它的路径，且不写创建时间。
- `desktop/tabs.go:5385-5425`：`CreateTopic` 检查标题/创建时间，却忽略项目添加与索引
  保存错误。
- `desktop/tabs.go:5970`：树节点只从 project/global topic list 与 title map 的并集
  枚举；仅有 session sidecar 不会成为节点。
- `desktop/frontend/src/features/workspace/ui/ProjectTree.tsx:813-845`：新建异常被静默
  捕获；同文件已有 `showError` 模式可复用。

失败路径由本机数据与源码共同证明；旧日志不足以唯一确定 7 月历史数据最初由哪一次
写失败或迁移产生，但不影响当前可达的前向缺陷定性。

## 6. 实现方案

### 6.1 统一登记与严格读取

在 `desktop` 内增加窄化的新话题登记 helper，复用现有原子替换写法，并由 topic 登记
互斥锁串行化创建流程。helper：

1. 校验并规范化 scope、workspace root、topic ID、标题来源和创建时间。
2. 使用登记专用严格 loader 读取 title/source/createdAt；不改变重命名、自动标题、
   model metadata 和删除所用的共享 topic loader。
3. 确保 project 存在，并把 topic 放入对应 project/Global 顺序。
4. 持久化标题、标题来源、创建时间和 project file，检查每一步错误。

另增加统一的 project-file 更新入口：它在专用 mutex 内严格读取、变更并原子替换
`desktop-projects.json`。文件不存在按空状态处理，其他读取或 JSON 错误原样返回；
`addProject`、`removeProject`、重命名、排序、颜色、置顶、topic 顺序和旧迁移等所有
进程内写入者均通过该入口，现有只读 `loadProjectsFile` 降级行为不变。

锁顺序固定为 topic 登记锁在外、project-file 更新锁在内；其他 project 写入者不得
获取 topic 登记锁，禁止反向嵌套。跨进程竞争仍属于非目标。

多文件不做通用事务。中途失败可能留下无正文的部分登记，但不得继续创建 session；
后续成功操作可复用或补齐它，因此不会形成用户内容不可达的问题。

### 6.2 调整创建顺序

- `EnsureBlankTab`：保留现有空白 tab/topic 复用；确需新建时先登记，再创建空 session
  和 tab。项目添加并入登记，不再单独忽略错误。
- `CreateTopic`：直接调用统一登记并只在完整成功后返回 `TopicMeta`。
- `NewSession`：先预留新 topic，再执行 controller `NewSession` 并更新 tab 身份；
  登记失败时旧 session 不动，runtime 失败时不得产生 session 孤儿。
- `Fork`：先预留 fork topic，再执行 `ForkSession` 和 topic meta 保存。
  `internal/control` 在基础 branch meta 保存失败时删除刚创建的 session artifacts；
  App 在 topic meta 保存失败时执行同样补偿。补偿只作用于本次新 fork，不触碰源会话；
  删除失败使用组合错误向上传递。进程崩溃/断电窗口仍按非目标处理。
- controller build 的 `ensureTopicIndexed` 保留为已打开标签的幂等防线，但不再是新建
  正常路径正确性的依赖。

### 6.3 错误呈现

`ProjectTree.handleCreateTopic` 使用现有 `showError` 报告新建失败。会话工具栏和消息
操作已有恢复/本地 notice 逻辑，只需确保后端错误向上传递，不新增交互样式。

## 7. 预计影响文件与契约

预计修改：

- `desktop/tabs.go`：登记专用严格 topic loader、统一 project-file 更新、话题登记和创建路径。
- `desktop/app.go`：`NewSession` 与 `Fork` 的预登记、补偿清理和错误传播。
- `internal/control/controller.go`：fork 基础 meta 失败时清理本次新建 artifacts。
- `desktop/tabs_topic_test.go`、`desktop/app_session_dedup_test.go`、必要的
  `desktop/app_test.go`：失败注入、顺序、并发与持久化测试。
- `internal/control/controller_test.go`：fork 基础 meta 失败补偿测试。
- `desktop/frontend/src/features/workspace/ui/ProjectTree.tsx` 及聚焦测试：错误 Toast。

不修改 Wails bridge 的公开方法签名，不修改 session/sidecar schema、topic ID 格式、
依赖或 `scripts/code_guard_baseline.json`。

## 8. 难度、风险与回滚

整体难度为中等，预计实施风险为低到中等。没有历史兼容和迁移后，数据扫描、冲突合并、
性能回归与误归属风险均被移除；主要风险只剩创建顺序和并发 RMW。

- 顺序回归：`NewSession`/`Fork` 预登记失败不得改变旧 runtime；用调用前后 topic、
  session path 和正文断言保护。
- 空白残留：session 创建失败可能留下无正文登记；要求保持可枚举/复用，并验证重复
  侧栏新建不会堆积。
- 并发丢更新：所有 project-file RMW 共享锁，并用 topic 创建与项目变更交叉测试验证。
- 锁顺序回归：只允许 topic 登记锁到 project-file 锁，用受控交叉并发测试验证完成性。
- 清理误删：fork 补偿只接受本次返回且位于 controller session dir 的新路径，并验证
  源 session、其他 session 和非 session sidecar 不变。
- 错误过度暴露：topic metadata 使用登记专用严格 loader；project-file 所有写操作
  fail closed，但现有只读展示降级不变。
- 前端误报：只在用户主动新建失败时调用现有 Toast，不改变成功路径。

最终交付使用一个修复 PR 和三个小步实现提交。提交按依赖顺序回退：前置提交只能在其
依赖提交先回退后撤销；也可整体回退 PR。schema 未变化，不需要数据降级，修复版本创建
的正常话题仍可由旧版本读取。若实现发现必须引入历史扫描、跨文件事务、跨进程锁或
公开契约变化，停止实施并重新确认范围。

## 9. 验证计划

### 自动化

实施迭代与交付至少执行：

```powershell
go test ./internal/control -count=1

cd desktop
go test . -count=1

cd frontend
pnpm build
```

聚焦测试覆盖 AC-01 至 AC-12，尤其包括每个持久化步骤的失败注入、损坏文件字节保持、
topic 创建与项目写操作交叉并发、`NewSession` 失败前后身份保持、fork 两层 meta 失败
补偿和空白复用。并发测试使用受控 barrier 重复验证完成与最终状态；前端测试断言
`ProjectTree` 调用现有错误 Toast。

### 应用验收

在隔离 dev 数据根中通过真实桌面应用验证：

1. project 与 Global 分别新建、发送消息、关闭并重启，确认侧栏可见且可恢复。
2. 对有正文会话执行“新建会话”和分叉，重启后核对新旧话题及正文。
3. 使用受控失败夹具阻断索引写入，确认界面提示失败、旧会话不变、无隐藏新 session。
4. 恢复写入后重试，确认创建成功且空白复用语义不变。

目标是 Windows 桌面原生交互，应用验收优先使用 Computer Use；稳定后再执行最终 PR
Gate。任何必需检查失败时不得创建或更新 PR。

## 10. 基线与关口

- Codegraph 导航基线：`D:\Reasonix@6391e4bfb5b499b4f0065e7a7e1bc606795d4eee`。
- 计划 worktree：
  `origin/main-v3@225394627c3d75661e2469be948e165513ea552f`。

Codegraph 仅用于已索引主工作区的调用路径导航；以上结论已在最新 worktree 直接读取
源码复核。

本计划没有待定产品问题。用户已明确批准流程例外：计划、任务文档和全部实现进入一个
最终修复 PR，以独立提交保留审查和回退边界，不再分别创建计划/任务 PR。任务拆解批准
且获得明确实施授权后才开始编码；所有任务、自动验证和应用验收稳定后，另行取得创建
Draft PR 的授权。

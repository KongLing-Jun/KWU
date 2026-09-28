# 新会话索引一致性修复进度

## 当前状态

- 阶段：Stage 3 完成
- 状态：实现、独立应用验收和最终 PR Gate 均通过；等待单独的 Draft PR 授权
- 范围：只防止未来新会话成为孤儿；不恢复或迁移历史数据
- 稳定决策与验收：[plan.md](./plan.md)

## 阶段与关口

| 项目 | 状态 | 下一关口 |
| --- | --- | --- |
| Grilling 与范围确认 | 完成 | 已确认无真实用户、采用前向防护 |
| 根因与生产可达性调查 | 完成 | 已用本机数据、Codegraph 与最新源码证明 |
| 最新基线与独立 worktree | 完成 | `origin/main-v3@5c9a4ffc`；task 基线 `27878da0` |
| Stage 1 计划 | 内容已批准；纳入最终单一修复 PR | 保持为稳定决策记录 |
| Stage 2 任务拆解 | 完成 | 已按单一 PR、三个串行提交执行 |
| Stage 3 实施 | 完成；代码、自动验证、独立应用验收与最终 PR Gate 均通过 | 获得单独授权后创建 Draft PR |

## 已确认决策

- 问题属于可达 bug：session/sidecar 与侧栏索引分离，部分新建路径吞掉持久化错误。
- clean production 用户低概率遇到，异常写入、损坏索引、升级/开发切换会提高概率。
- 当前无真实用户，不承担历史孤儿会话的兼容、扫描、补录或迁移成本。
- 新建路径采用“索引先于 session”原则；登记失败必须保留旧会话并向用户报错。
- 允许受控失败留下无正文、可复用/可见的部分登记，不允许留下有正文的隐藏 session。
- 新话题登记集中到后端，覆盖侧栏、`CreateTopic`、`NewSession`、`Fork` 及其间接调用方。
- 所有进程内 project-file 写入使用统一严格 RMW 锁；跨进程事务仍是非目标。
- topic metadata 使用登记专用严格 loader，避免改变其他共享更新路径。
- fork 两层 meta 写入失败均补偿删除本次新建 artifacts；清理失败返回组合错误。
- 损坏 project/topic 登记索引在更新路径 fail closed，不再当空数据覆盖。
- 保持零轮“新的会话”隐藏、空白复用、自动标题及 bridge/schema 不变。

## 证据与风险快照

| 项目 | 结论 |
| --- | --- |
| 本机失败样本 | 两个非空 project session 有完整 sidecar，但没有标题/项目 topic 索引和恢复 tab |
| 可见性根因 | `ListProjectTree` 不从 session sidecar 补充 topic ID |
| 直接失败入口 | `NewSession` 在 runtime 轮换后忽略 `ensureTopicIndexed`/createdAt 错误 |
| 其他缺口 | `EnsureBlankTab`、`CreateTopic` 忽略 project 保存错误；`Fork` 依赖后续修复 |
| 数据保护缺口 | 更新 loader 将损坏 JSON 当空 map/file |
| UI 缺口 | `ProjectTree` 静默捕获新建异常 |
| 难度 | 中等 |
| 实施风险 | 低到中等；集中在创建顺序和并发 RMW |

## 初版评审处理

| 评审项 | 处理结论 |
| --- | --- |
| Fork meta 失败遗留正文 | 纳入 `internal/control` 与 App 两层补偿及失败测试 |
| 其他 project 写入覆盖新 topic | 所有进程内 project-file RMW 统一锁和严格更新入口 |
| 共享严格 loader 扩大影响 | topic metadata 改用登记专用严格 loader；只读行为不变 |

## 任务与交付映射

| 任务 | 串行组 | 依赖 | 实现交付 |
| --- | --- | --- | --- |
| [001 统一话题与项目索引持久化边界](./tasks/001-统一话题与项目索引持久化边界.md) | Serial-01 | 无 | 已完成并通过主会话复审 |
| [002 重排会话轮换与分叉失败补偿](./tasks/002-重排会话轮换与分叉失败补偿.md) | Serial-02 | 001 完成并复审 | 已完成并通过主会话复审 |
| [003 呈现新建失败并完成应用验收](./tasks/003-呈现新建失败并完成应用验收.md) | Serial-03 | 001、002 完成并复审 | 已完成并通过主会话独立应用验收 |

用户已批准把文档和实现放入一个最终修复 PR。三个实现提交分别关闭持久化边界、
runtime/Fork、前端与验收，均需单独验证和主会话复审；提交存在串行依赖，回退时必须
从提交 3 向提交 1 逆序处理。默认串行实施，不并行修改共享文件。

## 验证状态与下一步

- 任务 001：Desktop 受影响包及并发/失败聚焦验证通过；主会话已复审。
- 任务 002：Root control 与 Desktop 受影响包验证通过；主会话已复审。
- 任务 003：真实组件 Vitest 2 项与 frontend 生产 build 通过；未运行 Go，因本任务无 Go 改动。
- 主会话使用隔离 dev 数据根和 Wails 正式构建完成真实应用验收：project/Global 新建、
  发送、两次重启恢复、正文会话后新建、零轮空白复用、损坏标题索引 fail closed、单一
  错误 Toast、旧会话保持、无隐藏 session、恢复后无需重启即可重试均通过。
- 失败前后 `desktop-projects.json` 哈希保持不变，project topic/session 数均保持 1；重试后
  topic 数按预期变为 2。隔离 fixture 位于被忽略的 `output/topic-index-acceptance/`，未使用
  或修改用户现有 profile。
- AC-03、AC-04、AC-07 至 AC-10 的不可安全/稳定 UI 注入分支由主会话复审后的 Desktop、
  controller 聚焦测试作为验收证据；分叉的悬浮消息菜单未通过 Computer Use 重复点击，
  不影响后端补偿合同的确定性验证。
- 最终 PR Gate 在已同步的最新 `origin/main-v3` 上通过：分类为 Root control targeted、
  Desktop targeted、frontend build；code guard、Markdown 链接、baseline 保护、diff check、
  `go test ./internal/control -count=1`、`cd desktop; go test . -count=1`、`pnpm build` 全部通过。
- Gate 首轮发现 `forkNamed` 圈复杂度超限后已做等价 helper 提取；主会话复审并独立复跑
  非缓存 control 测试和 code guard 后，完整 Gate 通过。未修改 guard baseline。
- 当前无技术或产品阻塞。只有获得单独 PR 授权后才能推送和创建 Draft 修复 PR。

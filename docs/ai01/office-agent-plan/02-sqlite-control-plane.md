# Phase 2：SQLite 自动化控制平面

> 入口：[总索引](./README.md) · 横向 schema：[接口/数据库/测试规范](./cross-cutting-contracts.md)

## 1. 决策边界

建议先扩展现有 `templates.sqlite`，保留历史文件名并利用单库事务。是否重命名为 `automation.sqlite` 另立 ADR。事件、会话、诊断 JSON 和 Office 文件继续留在文件系统。

## 2. 任务

### P2-1：Repository 接口与 schema migration

| 方面 | 工作 |
|---|---|
| 前端 | 仅增加迁移失败/只读降级通知 |
| 后端 | Task/Run/Artifact/Lease Repository；FileStore 与 SQLiteStore 合同一致 |
| 数据库 | schema_migrations、automation_tasks、task_sources、runs、artifacts、leases |
| 测试 | empty/previous migration、FK、CHECK、busy timeout、回滚 |
| 参考项目 | `REF-REASONIX` 的 Store/Archive；外部项目不适合替代本地 schema 事实 |

**验收：** migration 幂等；旧 `task_templates` 不变；Repository 合同测试双实现通过。
**依赖：** P0-2、P1-2。**范围：** M，接口、SQLite 实现、migration、测试。

### P2-2：任务回填与 position/snapshot 保真

| 方面 | 工作 |
|---|---|
| 前端 | 迁移前后任务顺序和最近运行显示不变 |
| 后端 | 使用现有 NormalizeTask 回填；生成 source hash 和 migration marker |
| 数据库 | 保留 position、完整配置 JSON、sources、created/updated/lastRun |
| 测试 | once/interval/calendar、Unicode、零值时间、损坏 JSON、重复 ID |
| 参考项目 | `REF-REASONIX` 当前 `tasks.json` 和 heartbeat migration |

**验收：** 任务 ID、顺序、时间和配置对账 100%。
**依赖：** P2-1。**范围：** M。

### P2-3：运行与 Artifact 索引回填

| 方面 | 工作 |
|---|---|
| 前端 | 历史运行和当前展示一致 |
| 后端 | 扫描 manifest；损坏项写报告；orphan run 创建软删除 task tombstone |
| 数据库 | run 保存 task snapshot；artifact 保存相对路径/hash/size/MIME |
| 测试 | 缺 manifest、缺 task、路径越界、重复回填、文件缺失 |
| 参考项目 | `REF-REASONIX` 的 run archive；`REF-OFFICE-AI-AGENT` 仅借鉴 artifact provenance |

**验收：** 任何损坏归档都不被删除；可从文件重建索引。
**依赖：** P2-1、P2-2。**范围：** M。

### P2-4：任务增量 CRUD 与乐观锁

| 方面 | 工作 |
|---|---|
| 前端 | Create/Update/Delete；携带 version；409 显示重新加载/比较 |
| 后端 | Update `WHERE id/version`；删除软删除；旧 SaveTasks 兼容层 |
| 数据库 | automation_tasks.version；task_sources 同事务 diff/replace |
| 测试 | 两窗口并发编辑、启停、删除保留历史、版本冲突 |
| 参考项目 | `REF-REASONIX` 现有 bridge/UI；外部项目只作交互灵感，不借 schema |

**验收：** 修改 enabled 不再全量覆盖 tasks 数组。
**依赖：** P2-2。**范围：** M。

### P2-5：SQLite 主读与 cursor 查询

| 方面 | 工作 |
|---|---|
| 前端 | 任务/运行列表 cursor 分页和筛选 |
| 后端 | ListRuns 不扫描目录；DB 缺项时文件 fallback 并自愈 |
| 数据库 | `(task_id,started_at desc,run_id desc)`、status/date 索引 |
| 测试 | 10k/100k runs 基准、cursor 稳定、DB unavailable fallback |
| 参考项目 | `REF-REASONIX` 页面语义；数据库方案以本地查询需求为准 |

**验收：** ListRuns 使用索引；页面无 N+1 每 task 扫目录。
**依赖：** P2-3、P2-4。**范围：** M。

### P2-6：影子写、切读与回滚

| 方面 | 工作 |
|---|---|
| 前端 | 显示迁移/降级 notice，不让用户误以为保存成功但索引失败 |
| 后端 | 文件主写+DB shadow → 对账 → DB 主读 → DB 主写；保留 file mode |
| 数据库 | migration journal、source hash、备份/恢复；SQLite backup 而非裸拷 WAL DB |
| 测试 | 每阶段崩溃、DB busy、commit 失败、重复执行、旧版本回退 |
| 参考项目 | `REF-REASONIX` 现有迁移 marker；`REF-OFFICE-AI-AGENT` 的原子/恢复思想 |

**验收：** 切换失败可回 file mode；任务/运行/模板计数和 ID 对账一致。
**依赖：** P2-5。**范围：** M。

## 3. Checkpoint P2

- [ ] 旧数据迁移和回滚演练通过。
- [ ] 100k run 查询不扫描归档目录。
- [ ] 数据库不含 token、secret、带密码 URL 或 Office binary。

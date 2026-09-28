# 阶段 2：会话目录与列表优化

## 阶段目标

用可重建的会话目录消除列表页逐个读取 JSONL 和多个 sidecar 的线性小文件 I/O，同时保持旧数据可读、索引可修复和 JSONL 正文权威性。

## 任务 2.1：定义目录查询契约

**目标**：先固定后端与前端共同依赖的数据契约，再实现存储。

| 方面 | 实施内容 |
| --- | --- |
| 前端页面 | 会话树继续显示路径、主题、模型、分支、预览、轮次、更新时间、专家和清理状态；增加 `indexing/repairing/degraded` 非阻塞状态。 |
| 后端功能 | 定义按 scope/workspace、游标和页大小查询的 DTO；排序使用稳定的 `updated_at + session_key`。 |
| 数据库 | 明确列、索引、schema version 和衍生字段；数据库不得返回超出当前 profile/workspace 的记录。 |
| 测试用例 | 契约测试覆盖空列表、同时间戳、分页边界、删除、重命名、分支和跨 workspace 隔离。 |

兼容原则：现有 Wails 方法先保留，内部可调用新目录服务；前端迁移完成并经过版本窗口后，才能讨论移除旧接口。

## 任务 2.2：设计派生目录表

**目标**：用单次有索引查询替代逐会话文件访问。

建议使用单独的 `session-catalog.sqlite`，避免与模型计费业务事务耦合。建议初始 schema：

```sql
CREATE TABLE session_catalog_schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
);

CREATE TABLE session_catalog (
    session_key TEXT PRIMARY KEY,
    scope TEXT NOT NULL CHECK (scope IN ('global', 'project')),
    workspace_key TEXT NOT NULL DEFAULT '',
    relative_path TEXT NOT NULL,
    topic TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    branch TEXT NOT NULL DEFAULT '',
    preview TEXT NOT NULL DEFAULT '',
    turn_count INTEGER NOT NULL DEFAULT 0 CHECK (turn_count >= 0),
    expert_id TEXT NOT NULL DEFAULT '',
    cleanup_pending INTEGER NOT NULL DEFAULT 0 CHECK (cleanup_pending IN (0, 1)),
    content_generation INTEGER NOT NULL DEFAULT 0 CHECK (content_generation >= 0),
    file_size INTEGER NOT NULL DEFAULT 0 CHECK (file_size >= 0),
    file_mtime_ns INTEGER NOT NULL DEFAULT 0 CHECK (file_mtime_ns >= 0),
    updated_at_ns INTEGER NOT NULL,
    indexed_at_ns INTEGER NOT NULL,
    metadata_version INTEGER NOT NULL DEFAULT 1
);

CREATE INDEX idx_session_catalog_scope_updated
    ON session_catalog(scope, workspace_key, updated_at_ns DESC, session_key DESC);

CREATE INDEX idx_session_catalog_workspace_branch
    ON session_catalog(workspace_key, branch, updated_at_ns DESC);
```

设计约束：

- `session_key` 和 `relative_path` 由受信任的路径解析器生成，不存绝对 profile 路径。
- `preview` 只保持现有最小预览语义，不增加全文搜索。
- `file_size + file_mtime_ns + metadata_version` 用于检测陈旧记录，但不能证明正文完整。
- 不为假设查询提前增加更多索引；新增索引必须有 `EXPLAIN QUERY PLAN` 和真实规模基准。

## 任务 2.3：实现增量写入与一致性修复

**目标**：会话成功保存、重命名、移动、删除或更新 sidecar 后，只更新对应目录记录。

1. JSONL/sidecar 原子写入成功后再提交目录记录。
2. 目录写失败不回滚已成功的正文；记录 repair-needed 并由后台重建。
3. 删除采用“文件成功删除后删目录记录”；清理中断时继续使用现有 cleanup-pending 语义。
4. 同一路径的并发更新以 content generation 和最新文件元数据防止旧写覆盖新写。
5. 多个标签页请求同一目录时使用 singleflight，避免重复回填。

## 任务 2.4：实现懒回填和降级读取

**目标**：首次升级不出现 20 秒级阻塞，也不要求用户手工迁移。

| 方面 | 实施内容 |
| --- | --- |
| 前端页面 | 先展示目录中已有记录；后台回填时显示轻量状态，新增结果增量进入列表且保持滚动位置。 |
| 后端功能 | 目录缺失时按批次扫描；设置并发、时间和 I/O 预算；支持 context 取消和断点继续。 |
| 数据库 | 每批事务提交，保存扫描游标/版本；损坏库自动隔离并重建，不删除 JSONL。 |
| 测试用例 | 覆盖首次升级、回填中退出、重复启动、损坏库、schema 升级、部分记录陈旧和数据库 busy。 |

降级顺序：目录查询 → 目录修复 → 有界 sidecar 回退。不得在 UI 主请求中无界解码所有 JSONL。

## 任务 2.5：前端会话树接入

**目标**：让列表快速可见，并避免一次树变化清空全部项目缓存。

- 使用游标或分批加载，不一次传输所有 workspace 的所有会话。
- 按 workspace/session key 局部更新；删除或重命名单个会话时不使全局 cache 失效。
- 保留当前选择和滚动位置，避免回填批次导致列表跳动。
- 错误状态允许重试和打开已知会话，不以空列表伪装失败。
- 对旧后端或目录功能关闭状态维持原兼容路径。

## 测试矩阵

| 层级 | 必测用例 |
| --- | --- |
| 单元 | key 归一化、稳定排序、游标、目录 upsert/delete、陈旧判断、路径隔离 |
| 数据库 | migration 幂等、约束、索引命中、事务回滚、busy、损坏重建 |
| 后端集成 | JSONL 保存后可见、索引失败后修复、删除中断、全局/项目/profile 隔离 |
| 前端 | 首批展示、增量批次、选择保持、加载/降级/错误状态、空目录 |
| 性能 | 100/300/500/1000 会话冷回填与热查询 P50/P95、文件打开数、峰值内存 |
| 安全 | 路径穿越、符号链接、伪造 relative path、跨 workspace key、超长 preview |

## 回滚方案

- 功能开关关闭目录读路径后，立即回到有界 sidecar 列表。
- 目录数据库可删除重建，不影响 JSONL 和现有 sidecar。
- 在一个兼容发布窗口内继续写 sidecar，保证旧版本能打开同一 profile。
- 若新旧版本可能并行运行，必须先验证 schema/锁协议；未验证前禁止共享 profile 并行写。

## 阶段退出门禁

- [ ] 500 会话热列表达到冻结预算，结果来自至少 5 次测量。
- [ ] 首次回填不阻塞首屏，且退出重启后可继续。
- [ ] 索引被删除或损坏后全部会话仍可恢复。
- [ ] 数据库查询计划命中预期索引，没有无依据的额外索引。
- [ ] 前端的加载、降级和错误状态均有自动化测试。

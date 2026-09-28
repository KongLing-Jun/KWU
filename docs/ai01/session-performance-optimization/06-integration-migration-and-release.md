# 阶段 6：集成、迁移与发布验收

## 阶段目标

把阶段 2～5 的独立优化组合成可灰度、可观测、可回滚的完整产品路径，并用真实桌面运行证明性能提升没有交换掉正确性、安全性或数据完整性。

## 任务 6.1：功能开关与版本兼容矩阵

**目标**：每个高风险优化可以独立启停，并明确跨版本 profile 兼容性。

| 开关 | 默认演进 | 关闭后的路径 |
| --- | --- | --- |
| session catalog read | 先关闭，灰度后开启 | 有界 sidecar 列表 |
| dirty autosave | 测试通过后默认开启 | 当前全量保存 |
| history pagination | 前后端 capability 协商后开启 | 旧全量历史接口 |
| stream coalescing | 前端单独灰度 | 当前逐事件 reducer |
| retry total budget | Provider 回归通过后开启 | 当前次数上限 |
| startup snapshot | profile 隔离通过后开启 | 当前完整 boot |
| SQLite WAL | 默认关闭 | rollback journal |

兼容矩阵至少包含：升级后首次启动、升级后回退、旧版本打开新 profile、新旧版本先后写入、索引存在/缺失/损坏，以及非默认 profile。

## 任务 6.2：端到端业务用例

**目标**：从前端页面到文件和数据库验证完整业务行为。

1. 启动桌面，加载 500 会话项目，会话树在预算内出现。
2. 打开 100 MB 长会话，只加载首屏窗口，向上分页保持锚点。
3. 展开一个子代理记录，只有该记录被读取并支持分页。
4. 发送消息，观察 optimistic UI、Provider 流、工具、重试提示和完成事件。
5. 长 Provider 等待跨越多个 autosave tick，验证未变化时没有正文重写。
6. 回合完成后验证 JSONL、sidecar 和目录 generation 一致。
7. 关闭并重启桌面，恢复当前标签和历史；删除目录数据库后仍能后台重建。
8. 切换 profile/workspace，确认列表、缓存、专家和计费状态不串用。

## 任务 6.3：故障与滥用演练

**目标**：证明性能优化在异常情况下仍保持安全失败。

| 故障 | 期望结果 |
| --- | --- |
| 保存中杀进程 | 上一份完整 JSONL 可读；临时文件可识别和清理 |
| 磁盘满/权限拒绝 | dirty 保留，UI 显示保存失败，不报告成功 |
| 目录 SQLite 损坏 | 隔离并重建；正文不删除；列表进入可见降级状态 |
| 回填中退出 | 已提交批次保留，下次继续，不从头形成扫描风暴 |
| MCP 挂起 | 核心页面可用，插件显示超时并可重试 |
| Provider 429/5xx | 遵守总预算，可取消，最终错误明确 |
| 巨型/恶意会话 | 内存和 I/O 有界，不跨路径读取，不执行正文中的内容 |
| 跨 workspace 路径伪造 | 后端拒绝，前端不能通过 DTO 绕过根目录校验 |

## 任务 6.4：测试金字塔与命令门禁

**目标**：用分层证据判断发布，而不是依赖单一全仓测试。

### 每个切片

1. 修改点单元测试或单文件前端测试。
2. 受影响 Go package 或前端测试组。
3. 类型检查、格式和静态检查。
4. 对应 benchmark 与故障测试。

### 集成候选

```powershell
go run ./scripts/code_guard
git ls-files --error-unmatch scripts/code_guard_baseline.json
git diff --exit-code -- scripts/code_guard_baseline.json
git diff --cached --exit-code -- scripts/code_guard_baseline.json
go vet ./...
go test ./...
```

前端在 `desktop/frontend` 执行：

```powershell
pnpm run test:all
pnpm run build
```

发布或更新 PR 前必须先同步最新 `main-v3`、解决冲突，再重新执行上述门禁。Code Guard 的 baseline notice 不是自动 ratchet 授权。

## 任务 6.5：性能验收报告

**目标**：用同一基准环境比较 before/after，并说明任何退化。

报告至少包含：

- Git SHA、操作系统、CPU、内存、磁盘类型、Go/Node/pnpm 版本。
- 数据集生成方式、总会话数、总字节、消息/子代理/图片分布。
- 每项至少 5 次的 P50/P95、分配、文件打开数、读写字节、React commits。
- 本地确定性 Provider 与真实 Provider 分开报告。
- 正确性测试、故障演练、性能目标和未达目标项。
- 功能开关状态、数据库 schema version、兼容与回滚验证。

任何目标未达成时必须写明：失败指标、用户影响、根因证据、是否阻塞发布以及下一步；不得只报告百分比最好结果。

## 任务 6.6：灰度、回滚与文档转正

**目标**：完成后把长期事实迁入稳定文档，把阶段计划归档。

1. 先在隔离 profile 和内部构建启用单个开关，观察 crash、保存失败、索引修复和性能预算。
2. 按 catalog → dirty save → history/UI → runtime 的顺序扩大灰度；不要一次开启全部开关。
3. 发现正文完整性、profile 隔离、计费或权限问题时立即关闭对应开关并执行回滚演练。
4. 将最终存储契约更新到稳定会话/配置文档，将已接受的重要决策提升为正式 ADR。
5. 把本目录标记为已完成并移入 `docs/archive/`，或只保留仍需追溯的验收报告；不得继续把计划文档当作实现依据。

## 最终验收清单

- [ ] 前端：列表、历史、流式、加载、错误、降级和无障碍状态通过测试与人工验收。
- [ ] 后端：目录、保存、分页、重试、启动缓存和取消语义通过单元/集成/竞态测试。
- [ ] 数据库：migration、约束、查询计划、连接关闭、busy、损坏恢复和 profile 隔离通过。
- [ ] 安全：路径、隐私、资源耗尽、插件权限和跨 workspace 用例通过。
- [ ] 性能：所有冻结 P95 预算达成；观测开销在预算内。
- [ ] 兼容：旧会话、旧 sidecar、升级、回退和非默认 profile 通过。
- [ ] 工程：Code Guard、baseline 保护、Go vet/test、前端 test/build 全部通过。
- [ ] 证据：原始结果、命令、版本和失败项可复核。
- [ ] 文档：稳定事实已转正，Proposed 决策状态已更新，计划与实现没有混淆。

## 可借鉴与禁止照搬矩阵

| 来源模式 | 可以借鉴 | 不应照搬 |
| --- | --- | --- |
| 现有 JSONL Save | 同目录临时文件、原子替换、流式 decoder | 每个 autosave tick 都全量重写 |
| `.meta` sidecar | 预览和轮次数等衍生字段 | 每次列表逐文件读取多个 sidecar |
| model billing SQLite | schema version、约束、busy timeout、连接池、索引验证 | 将计费业务表或事务边界复制到会话目录 |
| 当前 RAF batch | 在动画帧边界刷新 UI | 只排队不合并，仍逐事件 dispatch |
| 只读工具并行 | 基于依赖和副作用安全并行 | 把未知或写工具标记为只读以追求速度 |
| MCP shared host | 相同配置的启动复用 | 跨 profile、权限或密钥上下文共享实例 |

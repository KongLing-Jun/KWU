# 阶段 7：安全、观测、性能与发布

> 阶段目标：以威胁模型、容量证据、迁移回滚和许可证清单作为发布门，而不是以“主流程能跑通”代替生产可用性。

## P7-1 威胁模型与攻击测试

**目标**：覆盖文档内容注入、路径/宏/外链、GUI 状态漂移、权限提升、凭据和跨租户风险。

| 方面 | 实施内容 |
|---|---|
| 前端 | 清晰显示不可信内容、授权边界、外部目标和高风险文件告警。 |
| 后端 | trust boundary、内容/指令分离、workspace confinement、宏禁用、外链策略和最小权限。 |
| 数据库 | 敏感字段分类、加密/脱敏策略和审计保留。 |
| 测试 | 低显著度 prompt injection、恶意 Office/CSV、路径穿越、SSRF、跨租户、陈旧批准。 |
| 借鉴项目 | `REF-FINEPRINT`：监督失效/注入；`REF-WORLDGUI`：状态漂移；`REF-UFO`：GUI/API 风险；`REF-OFFICE-AI-AGENT`：Office 文件风险；`REF-REASONIX`：approval/MCP/path confinement。 |

**验收**：内容无法提升权限；高风险动作必须由独立策略与明确批准共同放行。

## P7-2 GUI fallback 的隔离与门控

**目标**：仅当结构化 API/Renderer 不支持且策略允许时，使用隔离的 GUI fallback。

| 方面 | 实施内容 |
|---|---|
| 前端 | 明确标识 GUI 模式、预计步骤、风险、可取消性和人工接管。 |
| 后端 | capability router 默认结构化路径；GUI runner 独立进程/权限；每步 postcondition 与 fail-stop。 |
| 数据库 | 记录步骤、截图/证据地址、初态摘要、postcondition 和人工接管。 |
| 测试 | 弹窗、焦点丢失、缩放、布局变化、慢启动、权限窗口和中途取消。 |
| 借鉴项目 | `REF-UFO`：API/GUI 分层与 DAG；`REF-OPENADAPT`：健康重放/修复/postcondition；`REF-WORLDGUI`：非默认初态；`REF-FINEPRINT`：GUI 注入风险。 |

**验收**：GUI 失败不会继续点击或产生对外副作用；结构化路径始终优先。

## P7-3 可观测性与运维

**目标**：用 task/run/stage/artifact/approval/delivery/fencing 关联整个链路，同时避免记录业务秘密。

| 方面 | 实施内容 |
|---|---|
| 前端 | 运维诊断视图、相关 ID、阶段耗时和可脱敏导出。 |
| 后端 | metrics/traces/structured logs；租约、队列、Runtime、重试和 dead-letter 告警。 |
| 数据库 | 运维元数据与正文分离；定义保留/采样/脱敏。 |
| 测试 | correlation 完整性、敏感日志扫描、指标基数、告警演练和时钟漂移。 |
| 借鉴项目 | `REF-OPENADAPT`：步骤/postcondition/恢复记录；`REF-REASONIX`：engine/archive/eventwire。 |

**验收**：单个 run 可贯穿查询；日志中无凭据、完整文档或敏感行数据。

## P7-4 性能、容量与稳定性

**目标**：为控制平面、事件、Runtime 和前端建立可重复容量基线。

| 方面 | 实施内容 |
|---|---|
| 前端 | 10 万 Run 虚拟列表、分页、渐进预览和内存基线。 |
| 后端 | 1 千 Task/10 万 Run、队列并发、SQLite contention、Runtime 冷启动/峰值内存。 |
| 数据库 | 索引/查询计划、WAL、busy timeout、备份恢复和增长模型。 |
| 测试 | soak、crash loop、并发 Trigger、超大 XLSX/PPTX、磁盘满和低内存。 |
| 借鉴项目 | `REF-WORLDGUI`/`REF-UFO`：多初态和 GUI 恢复；`REF-UNIVER`/`REF-ONLYOFFICE`：真实格式 Runtime 基准；`REF-REASONIX`：Engine/SQLite/frontend。 |

**验收**：发布阈值和实测结果入库/入文档；不能引用外部宣传数据代替本地证据。

## P7-5 迁移、特性开关与回滚演练

**目标**：支持 File→SQLite、旧 API→v2、旧 exporter→Artifact Engine 的分步灰度和回退。

| 方面 | 实施内容 |
|---|---|
| 前端 | capability/flag 驱动入口，降级时明确提示。 |
| 后端 | shadow/read-compare、按用户/任务灰度、兼容 adapter、kill switch。 |
| 数据库 | migration checkpoint、schema/version、备份、恢复和回退兼容规则。 |
| 测试 | 升级/降级、flag 翻转、旧数据、新旧进程交替、迁移中崩溃。 |
| 借鉴项目 | `REF-REASONIX`：migration/feature flag/code_guard；`REF-OPENADAPT`：成熟度边界与 fail-stop。 |

**验收**：每个 flag 都有 owner、删除条件和回滚手册；旧数据不被不可逆覆盖。

## P7-6 许可证、供应链、打包与发布门

**目标**：证明分发物只包含批准依赖，并完成桌面安装、升级、卸载和离线行为验证。

| 方面 | 实施内容 |
|---|---|
| 前端 | 第三方声明、Runtime 下载/缺失提示、离线能力说明。 |
| 后端 | 依赖锁定、hash/signature、SBOM、Runtime 版本探测和安全更新策略。 |
| 数据库 | 记录迁移版本，不记录安装秘密。 |
| 测试 | 全新安装、覆盖升级、降级、离线、签名、杀毒误报、残留数据和许可证扫描。 |
| 借鉴项目 | `REF-OFFICEDEX`：Wails 发布形态（黑盒）；`REF-ONLYOFFICE`：商业/AGPL 决策样本；`REF-UNIVER`：开源/Pro 边界；`REF-REASONIX`：本地构建与发布门。 |

**验收**：SBOM、NOTICE、许可证 ADR 和安装包内容一致；GPL/AGPL 未经批准不进入分发物。

## P7-7 最终质量门与发布验收

**目标**：以端到端业务证据和项目规定检查决定是否发布。

| 方面 | 实施内容 |
|---|---|
| 前端 | 关键旅程 E2E、可访问性、错误恢复和双 transport。 |
| 后端 | 单元/合同/集成/故障注入、安全、性能和升级测试。 |
| 数据库 | 一致性、备份恢复、迁移和回滚演练。 |
| 测试 | 执行项目规定的 `code_guard`、`go vet`、`go test` 及前端检查；记录环境和结果。 |
| 借鉴项目 | `REF-REASONIX`：唯一发布准入；外部项目仅提供反例和测试维度。 |

**验收**：所有阻塞缺陷关闭；未执行检查、环境限制和残余风险被明确披露。

## 阶段检查点

- [ ] 威胁模型与攻击测试覆盖真实 Office 文件和对外副作用。
- [ ] GUI fallback 默认关闭且有独立 kill switch。
- [ ] 容量阈值、告警和运维手册完成演练。
- [ ] 升级、降级、回滚与安装包许可证检查通过。
- [ ] 项目发布门全部执行并保留证据。

**退出门**：只有上述检查点与最终质量门全部通过，才可声明业务功能可发布。

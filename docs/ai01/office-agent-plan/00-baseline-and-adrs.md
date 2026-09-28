# Phase 0：契约、样本、ADR 与 Runtime PoC

> 入口：[总索引](./README.md) · 参考：[项目借鉴地图](./reference-projects.md) · 下一阶段：[运行可靠性](./01-runtime-reliability.md)

## 1. 阶段目标

在修改生产路径前冻结领域语言、固定样本、API 合同、许可证边界和 Office Runtime 选型。Phase 0 不引入正式运行依赖。

## 2. 退出门槛

- [ ] Artifact/Run/Approval/Delivery v2 合同通过前后端评审。
- [ ] DOCX/XLSX/PPTX 共 12 个脱敏固定样本和预期结果可复现。
- [ ] 两条 Runtime PoC 都完成 import/inspect/apply/validate/render/diff/rollback。
- [ ] ADR-001～ADR-008 被接受、驳回或明确延期。
- [ ] 许可证矩阵和“不复制实现”边界获得确认。

## 3. 任务

### P0-1：冻结 MVP 用户旅程

**目标：** 把首个交付限定为“经营诊断 → 真实数据 → 可追溯事实 → Excel Artifact → 本地保存”。

| 方面 | 工作 |
|---|---|
| 前端 | 绘制任务向导、运行中心、质量/trace、Artifact 预览状态图 |
| 后端 | 冻结 Task、Run、Diagnostics、Artifact、Version 的领域术语 |
| 数据库 | 仅输出数据字典，不创建生产表 |
| 测试 | 写 10～12 条用户旅程 acceptance scenarios |
| 参考项目 | `REF-REASONIX` 当前页面；`REF-OFFICEDEX` 只参考工作台信息架构；`REF-ROWBOAT` 只参考后续知识关联 |

**验收：**

- [ ] in-scope/out-of-scope 无冲突。
- [ ] “事实、AI 解读、Artifact、交付”不混用。
- [ ] 外部发送明确不属于首个切片。

**依赖：** 无。**范围：** S，文档任务。

### P0-2：定义 v2 领域合同

**目标：** 定义 `Artifact`、`ArtifactVersion`、`EditPlan`、`ValidationReport`、`RenderManifest`、`DeliveryReceipt`。

| 方面 | 工作 |
|---|---|
| 前端 | TypeScript DTO 与状态枚举草案 |
| 后端 | Go domain model、JSON 字段和 schemaVersion |
| 数据库 | 为每个字段标注表列、JSON 或文件归属 |
| 测试 | JSON golden、未知字段、旧版本读取、非法枚举 |
| 参考项目 | `REF-OFFICE-AI-AGENT` 的版本/diff/rollback；`REF-WORD-MCP` 的工具 schema；`REF-REASONIX` 的现有 Task/Run DTO |

**验收：**

- [ ] Desktop、serve、React 字段同义。
- [ ] 写操作都有 requestId、baseVersion 和 audit 字段。
- [ ] 未知主版本 fail-closed。

**依赖：** P0-1。**范围：** M，建议 3～5 个合同/测试文件。

### P0-3：建立 12 个固定 Office 样本

**目标：** 为三种格式建立简单、复杂、异常、第三方生成四类样本。

| 方面 | 工作 |
|---|---|
| 前端 | 保存预览截图和差异基线 |
| 后端 | 样本探测器输出结构、公式、图表、媒体、主题、外链摘要 |
| 数据库 | 不写用户库；fixture manifest 保存 hash 和许可来源 |
| 测试 | Word/Excel/PowerPoint 与 LibreOffice 打开结果、预期修改、失败预期 |
| 参考项目 | `REF-UNIVER` 的示例/测试组织；`REF-OFFICE-AI-AGENT` 的视觉 QA；`REF-ONLYOFFICE` 只参考格式覆盖 |

**验收：**

- [ ] 12 个样本均脱敏且可合法用于测试。
- [ ] 每个样本有“必须保持”和“允许变化”清单。
- [ ] 样本损坏或高级特性边界明确。

**依赖：** P0-2。**范围：** M，测试资产任务。

### P0-4：Univer 路线 PoC

**目标：** 验证 Univer 开源能力是否满足 Excel 首个切片和 React 嵌入。

| 方面 | 工作 |
|---|---|
| 前端 | React 内嵌、网格预览、公式/图表摘要、加载性能 |
| 后端 | Node/headless 或隔离 runtime 的 import/inspect/apply/validate 协议 |
| 数据库 | 临时 SQLite/目录记录 PoC 结果，不写生产库 |
| 测试 | XLSX 公式、类型、格式、图表引用；DOCX/PPTX 只验证公开能力边界 |
| 参考项目 | `REF-UNIVER`；`REF-OFFICEDEX` 仅观察 Wails/runtime 隔离，不复制 GPL 代码 |

**验收：**

- [ ] 明确开源版导入导出边界。
- [ ] 目标修改和非目标保持有数据。
- [ ] 10MB 首次预览给出实测结果。

**依赖：** P0-2、P0-3。**范围：** M，独立 PoC 分支。

### P0-5：OOXML sidecar 路线 PoC

**目标：** 验证隔离 sidecar 对 DOCX/XLSX/PPTX 的局部编辑和原件保护。

| 方面 | 工作 |
|---|---|
| 前端 | 只消费统一 PoC 结果，不绑定 Python 类型 |
| 后端 | stdio JSON-RPC、版本握手、超时、资源限制、工作副本、原子提交 |
| 数据库 | 临时记录 version/diff/validation；不写生产库 |
| 测试 | sidecar 崩溃、超时、路径逃逸、回滚；三格式固定样本 |
| 参考项目 | `REF-OFFICE-AI-AGENT` 的 python-docx/openpyxl/python-pptx 设计；`REF-WORD-MCP` 的协议粒度 |

**验收：**

- [ ] 失败不覆盖原件。
- [ ] 12/12 可恢复到字节或语义等价版本。
- [ ] sidecar 无 workspace 外访问权。

**依赖：** P0-2、P0-3。**范围：** M，独立 PoC 分支，可与 P0-4 并行。

### P0-6：Runtime/许可证 ADR 决策

**目标：** 以数据选择产品主路径，并记录弃选理由。

| 方面 | 工作 |
|---|---|
| 前端 | 对比预览质量、交互复杂度和包体影响 |
| 后端 | 对比保真、隔离、故障恢复、维护成本 |
| 数据库 | 决定 runtimeVersion/schemaVersion 和迁移边界 |
| 测试 | 汇总 12 样本和安全负向结果 |
| 参考项目 | `REF-UNIVER`、`REF-OFFICE-AI-AGENT`；`REF-ONLYOFFICE` 商业评估；`REF-OFFICEDEX` GPL 隔离研究 |

**验收：**

- [ ] ADR-008 为 Accepted 或明确 No-Go。
- [ ] NOTICE/SBOM/许可证动作有 owner。
- [ ] 失败路线不残留生产依赖。

**依赖：** P0-4、P0-5。**范围：** S，决策任务。

## 4. Checkpoint P0

- [ ] 两条 PoC 结果可复现。
- [ ] 12 样本无静默错误。
- [ ] 架构和许可证评审通过后才进入 Phase 1/4。

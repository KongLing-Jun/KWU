# 阶段 4：Artifact Engine 与 Office 输出

> 阶段目标：建立与具体 Office Runtime 解耦、可验证、可回滚的 Artifact Engine；先完成 Excel 月度经营报告纵向切片，再扩展 DOCX/PPTX。

## 进入条件

- 阶段 0 的 Runtime、Artifact IR 与许可证 ADR 已形成候选结论。
- 阶段 1 的异步运行、取消和终态提交可用。
- 阶段 2 能持久化 Artifact 元数据与版本关系。
- 阶段 3 能稳定产出结构化 diagnostics facts 和 trace。

## P4-1 Runtime Adapter 与安全执行边界

**目标**：定义 `Import → Inspect → Apply → Validate → Diff → Commit/Rollback` 统一接口，让 Renderer 不直接依赖具体 Runtime。

| 方面 | 实施内容 |
|---|---|
| 前端 | 显示 Runtime、阶段、验证结果、失败原因和回滚结果；不暴露底层进程细节。 |
| 后端 | 新建版本化 Runtime Adapter；限制工作目录、超时、资源和允许格式；所有提交使用临时工作副本。 |
| 数据库 | 记录 runtime/version、input/output hash、validation receipt、commit/rollback 状态。 |
| 测试 | 合同测试覆盖成功、超时、崩溃、越界路径、验证失败、重复提交和回滚。 |
| 借鉴项目 | `REF-OFFICE-AI-AGENT`：工作副本、原子提交、diff/rollback；`REF-UNIVER`：Facade/headless；`REF-ONLYOFFICE`：多格式能力对照；`REF-OFFICEDEX`：壳与 runtime 隔离。 |

**验收**：同一用例可切换 fake/runtime adapter；失败不会覆盖原文件；每次提交均可追溯输入、Runtime 与输出 hash。

**边界**：不得复制 GPL/AGPL 实现；ONLYOFFICE 只进入许可评估；Univer Pro 能力不得视为开源能力。

## P4-2 Canonical Artifact IR 与版本模型

**目标**：以自有、版本化 IR 表达报告结构、来源、样式意图和验证规则，避免绑定某个 Renderer。

| 方面 | 实施内容 |
|---|---|
| 前端 | 只消费稳定 Artifact DTO；支持摘要、章节/工作表/幻灯片树和 provenance 展示。 |
| 后端 | 定义 `ArtifactIR v1`、升级器、Renderer SPI、ValidationRule 与 DiffReceipt。 |
| 数据库 | `artifacts`、`artifact_versions` 保存元数据、父版本、IR schema 版本和内容地址。 |
| 测试 | JSON schema/golden、升级兼容、未知字段、确定性序列化、跨 Renderer 合同测试。 |
| 借鉴项目 | `REF-UNIVER`：结构化模型和插件隔离；`REF-OFFICE-AI-AGENT`：版本/provenance/diff；`REF-REASONIX`：现有 diagnostics/result/export。 |

**验收**：IR 能表达首个 Excel 报告且不含 Runtime 私有对象；相同输入可稳定生成相同逻辑结构。

**依赖**：P4-1。不得把二进制正文写入 SQLite。

## P4-3 Excel 月度经营报告纵向切片

**目标**：从 diagnostics facts 生成包含摘要、明细、公式、图表和数据质量说明的可打开 XLSX，并完成预览、下载、版本和回滚闭环。

| 方面 | 实施内容 |
|---|---|
| 前端 | 增加模板选择、生成进度、工作表预览、公式/图表验证摘要、版本对比与下载。 |
| 后端 | 实现 XLSX Renderer；保留非目标工作表/公式；防公式与 CSV 注入；生成后重新读取验证。 |
| 数据库 | 保存模板版本、数据快照 hash、Artifact 版本、验证收据及 Run 关联。 |
| 测试 | 12 组基线样本、公式/图表/合并单元格/中文/空值/大数据/损坏模板；LibreOffice/Excel 打开性抽检。 |
| 借鉴项目 | `REF-UNIVER`：网格、公式、图表、React/headless PoC；`REF-OFFICE-AI-AGENT`：局部编辑与原子保存；`REF-OPENADAPT`：postcondition 与失败即停；`REF-REASONIX`：XLSX helper 和经营诊断。 |

**验收**：基线样本全部通过结构验证；原模板非目标区域 hash/语义不变；失败不产生“已完成”版本。

**边界**：这是首个发布切片；未通过前不并行承诺 Word/PPT 完整保真。

## P4-4 DOCX Renderer 与局部编辑边界

**目标**：生成/更新段落、表格、目录、页眉页脚、批注及修订可控的 DOCX。

| 方面 | 实施内容 |
|---|---|
| 前端 | 展示章节树、修改摘要、批注/修订状态和下载入口。 |
| 后端 | 实现 DOCX Renderer；显式列出不支持的复杂域、宏和嵌入对象；保存前后执行结构验证。 |
| 数据库 | 复用 Artifact 版本表，增加 capability/validation 元数据。 |
| 测试 | 段落、表格、分页、中文字体、批注、修订、损坏文件、宏文件拒绝策略。 |
| 借鉴项目 | `REF-WORD-MCP`：Word 工具粒度；`REF-OFFICE-AI-AGENT`：局部编辑/修订边界；`REF-ONLYOFFICE`：保真对照。 |

**验收**：支持集内样本可打开、结构正确、可回滚；不支持能力以 typed error 返回。

**边界**：归档的 Word MCP 不作为依赖；AGPL 代码不得进入产品实现。

## P4-5 PPTX Renderer 与视觉 QA

**目标**：从模板和 IR 生成结构稳定、可视觉比对的 PPTX。

| 方面 | 实施内容 |
|---|---|
| 前端 | 幻灯片缩略图、变更摘要、视觉差异和溢出告警。 |
| 后端 | 实现文本、表格、图片、基础图表；渲染预览图并执行越界/遮挡/缺字检测。 |
| 数据库 | 保存 preview 地址、视觉 QA 结果和 capability 声明。 |
| 测试 | 母版、16:9/4:3、中文、长文本、图片缺失、复杂图表降级和像素差阈值。 |
| 借鉴项目 | `REF-OFFICE-AI-AGENT`：python-pptx/视觉 QA 思路；`REF-UNIVER`：Slides 结构化预览 PoC；`REF-ONLYOFFICE`：格式对照；`REF-REASONIX`：最小 PPTX 导出。 |

**验收**：支持集样本无严重溢出，视觉差异可审阅；SmartArt/复杂母版等边界明确。

## P4-6 模板、缓存与 Artifact 生命周期

**目标**：统一模板注册、版本锁定、缓存、派生关系、保留与清理策略。

| 方面 | 实施内容 |
|---|---|
| 前端 | 模板版本选择、兼容性提示、版本历史、恢复与清理预览。 |
| 后端 | 模板不可变版本；缓存键包含 IR/runtime/config/template 版本；清理采用 tombstone。 |
| 数据库 | 模板版本、Artifact parent、引用计数/保留期、tombstone 与审计记录。 |
| 测试 | 模板升级、缓存命中/失效、引用中版本不可删、清理恢复和并发生成。 |
| 借鉴项目 | `REF-OFFICE-AI-AGENT`：请求级版本；`REF-UNIVER`：插件/注册；`REF-OFFICEDEX`：Workspace Artifact 体验；`REF-REASONIX`：templates/archive。 |

**验收**：历史 Run 永远解析到原模板和 Runtime；关闭缓存不改变输出语义。

## 阶段检查点

- [ ] Excel 纵向切片完成真实文件生成、结构验证、预览、版本、回滚。
- [ ] Runtime 崩溃、验证失败和取消均不覆盖原文件。
- [ ] DOCX/PPTX 支持矩阵和不支持项已形成测试化合同。
- [ ] GPL/AGPL 候选未进入分发物，许可证 ADR 已更新。

**退出门**：先评审 Excel 切片与 Runtime ADR，再批准扩展 DOCX/PPTX。

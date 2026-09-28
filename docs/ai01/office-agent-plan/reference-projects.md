# Office Agent 项目借鉴地图

> 本文件是开发导航，不是依赖批准清单。仓库状态与许可证信息来自 2026-07-21 调研快照；真正引入依赖前必须重新核验 release、许可证、传递依赖和安全状态。

## 1. 参考项目编号

| 编号 | 项目 | 可重点借鉴 | 不可直接推导/限制 | 使用方式 |
|---|---|---|---|---|
| `REF-REASONIX` | 当前 Reasonix/KWU | Controller、Automation、Topic/Session、审批、Tools、Skills、MCP、Wails/HTTP、归档 | 当前租约、宿主差异和最小 Office 导出需要修复 | **直接复用和演进** |
| `REF-UNIVER` | [dream-num/univer](https://github.com/dream-num/univer) | React 嵌入、插件架构、Facade API、公式引擎、headless、Sheets 预览/编辑 | Slides 仍在发展；部分导入导出能力属于 Pro；不能由 README 推断 DOCX/PPTX 保真 | Apache-2.0；**PoC 后决定依赖范围** |
| `REF-UFO` | [microsoft/UFO](https://github.com/microsoft/UFO) | Windows GUI/API 混合动作、应用级 Agent、DAG 任务分解 | Python runtime；GUI 成功率不可直接迁移；不应成为主 Office 引擎 | MIT；**只研究 GUI 兜底架构** |
| `REF-OFFICEDEX` | [officecli/officedex](https://github.com/officecli/officedex) | Wails v2 + React 19 办公工作台、runtime 解耦、内嵌预览、上下文连续编辑 | GPL-3.0；生成 runtime 位于另一仓库；禁止未经法务许可复制代码 | **黑盒产品/交互标杆** |
| `REF-OFFICE-AI-AGENT` | [MO-HU-P/office-ai-agent](https://github.com/MO-HU-P/office-ai-agent) | 原子保存、版本历史、request-level diff/rollback、局部编辑、视觉 QA、批注/修订 | 新项目、单贡献者、无 release/采用信号；不能当稳定生产依赖 | Apache-2.0；**设计样本 + sidecar PoC** |
| `REF-OPENADAPT` | [OpenAdaptAI/OpenAdapt](https://github.com/OpenAdaptAI/OpenAdapt) | 演示编译、无模型健康回放、postcondition、受控修复、失败即停 | 通用 Windows GUI 仍是 scoped acceptance；部署栈不同 | MIT；**确定性回放/GUI PoC** |
| `REF-ROWBOAT` | [rowboatlabs/rowboat](https://github.com/rowboatlabs/rowboat) | 本地优先 coworker、邮件、会议、后台 Agent、MCP、知识关联 | 产品面宽；不是 Office 文件引擎；不可提前扩大 MVP | Apache-2.0；**Phase 5/后续知识闭环标杆** |
| `REF-ONLYOFFICE` | [ONLYOFFICE/DocumentBuilder](https://github.com/ONLYOFFICE/DocumentBuilder) | 多格式脚本模板、DOCX/XLSX/PPTX/PDF、无需打开完整编辑器 | AGPL-3.0/商业许可；体积、分发和网络义务必须评估 | **商业/许可证评估，不复制实现** |
| `REF-WORD-MCP` | [GongRzhe/Office-Word-MCP-Server](https://github.com/GongRzhe/Office-Word-MCP-Server) | Word 工具粒度、MCP schema、资源与工具分离 | 项目已归档且仅 Word；不得成为运行时依赖 | MIT；**协议参考** |
| `REF-TASKWEAVER` | [microsoft/TaskWeaver](https://github.com/microsoft/TaskWeaver) | 分析计划、代码解释器、插件式数据任务 | 已归档；与 Reasonix Agent Core 重叠 | MIT；**历史设计参考，不引入第二内核** |
| `REF-WORLDGUI` | [WorldGUI 论文](https://arxiv.org/html/2502.08047v4) | 非默认初始状态、焦点/弹窗/布局变化的 GUI 基准方法 | 是研究证据，不是代码依赖 | **GUI 验收反证** |
| `REF-FINEPRINT` | [Fine-Print Injection 论文](https://arxiv.org/html/2504.11281v1) | 低显著度恶意内容、人工监督不足、外部策略必要性 | 是研究证据，不提供实现 | **安全威胁模型与对抗用例** |

## 2. 功能到项目的快速映射

| 功能 | 首选参考 | 次选参考 | 借鉴重点 |
|---|---|---|---|
| React Office 预览/编辑 | `REF-UNIVER` | `REF-OFFICEDEX` | 嵌入方式、Facade、插件和工作台布局 |
| Wails 办公工作台 | `REF-REASONIX` | `REF-OFFICEDEX` | runtime 隔离、页面导航、Artifact 上下文 |
| Office Artifact 生命周期 | `REF-OFFICE-AI-AGENT` | `REF-REASONIX` | 工作副本、版本、diff、rollback、原子提交 |
| DOCX 工具/API 粒度 | `REF-WORD-MCP` | `REF-OFFICE-AI-AGENT` | 工具 schema、段落/表格/批注边界 |
| XLSX 公式与网格 | `REF-UNIVER` | `REF-OFFICE-AI-AGENT` | 公式引擎、typed cell、工作表局部编辑 |
| PPTX 生成与视觉 QA | `REF-OFFICE-AI-AGENT` | `REF-ONLYOFFICE` | 溢出/遮挡、模板、可编辑对象 |
| 多格式脚本引擎评估 | `REF-ONLYOFFICE` | `REF-UNIVER` | 广泛格式、模板脚本、部署/许可证评估 |
| 确定性经营分析 | `REF-REASONIX` | `REF-TASKWEAVER` | 当前 BusinessDiagnostics；计划/代码分析思想 |
| 可重放办公流程 | `REF-OPENADAPT` | `REF-UFO` | postcondition、健康回放、漂移时受控修复 |
| Windows GUI 兜底 | `REF-UFO` | `REF-OPENADAPT`、`REF-WORLDGUI` | 应用身份、DAG、非默认状态、失败即停 |
| 任务/会话/知识关联 | `REF-REASONIX` | `REF-ROWBOAT` | workspace/topic/session 与 Artifact 的关系 |
| 邮件与远程交付 | `REF-ROWBOAT` | `REF-REASONIX` | 草拟优先、发送审批、后台任务和 MCP |
| 提示注入防护 | `REF-FINEPRINT` | `REF-REASONIX` | 策略在 Agent 外、数据流限制、后置条件 |
| 数据任务插件化 | `REF-TASKWEAVER` | `REF-REASONIX` | 分析阶段和插件边界；不复制 Agent 内核 |

## 3. 按阶段参考

| 阶段 | 必看 | 可选 | 原因 |
|---|---|---|---|
| Phase 0 | `REF-UNIVER`、`REF-OFFICE-AI-AGENT`、`REF-OFFICEDEX` | `REF-ONLYOFFICE` | Runtime PoC、样本、产品边界与许可证 |
| Phase 1 | `REF-REASONIX`、`REF-OPENADAPT` | `REF-UFO` | 租约/运行可靠性以本地为主，借鉴 postcondition/失败即停 |
| Phase 2 | `REF-REASONIX` | 无 | SQLite 迁移必须按本地 Store/Archive 事实设计 |
| Phase 3 | `REF-REASONIX`、`REF-TASKWEAVER` | `REF-OPENADAPT` | 确定性事实、分阶段分析、可追溯和停机门禁 |
| Phase 4 | `REF-UNIVER`、`REF-OFFICE-AI-AGENT` | `REF-WORD-MCP`、`REF-ONLYOFFICE` | Artifact Runtime 和三格式实现 |
| Phase 5 | `REF-REASONIX`、`REF-ROWBOAT` | `REF-FINEPRINT` | 审批、后台交付、内容不能提升权限 |
| Phase 6 | `REF-OFFICEDEX`、`REF-UNIVER`、`REF-REASONIX` | `REF-ROWBOAT` | Wails 工作台、预览和上下文关联 |
| Phase 7 | `REF-WORLDGUI`、`REF-FINEPRINT`、`REF-OPENADAPT` | `REF-UFO` | 对抗、GUI 鲁棒性、失败即停和发布门 |

## 4. 借鉴规则

### 4.1 可以借鉴

- 架构分层、接口粒度、测试方法、失败模式和验收指标。
- Apache-2.0/MIT 项目的公开 API 设计，但真正复制代码仍需 notice 和传递依赖核查。
- GPL/AGPL 项目的产品行为和黑盒交互模式，用于需求和竞品研究。
- 已归档项目的协议思想和历史设计，前提是不作为关键运行依赖。

### 4.2 不可以默认借鉴

- 不能把项目 star、README 宣传或 demo 成功当作 Reasonix 的可靠性证据。
- 不能未经法务结论复制 `REF-OFFICEDEX` 或 `REF-ONLYOFFICE` 实现。
- 不能因为 `REF-UFO`/`REF-OPENADAPT` 能操作 GUI，就把 GUI 放在结构化 API 前面。
- 不能因 `REF-TASKWEAVER` 有分析 Agent 而建立第二套 Agent Core。
- 不能把 `REF-WORD-MCP` 作为长期依赖；它已归档。

### 4.3 每个参考动作应留下记录

```text
Reference ID:
Repository/release/commit checked:
Feature studied:
Files/API/docs inspected:
What is reused: concept | API shape | test method | code
License result:
Security result:
Reasonix adaptation:
Rejected parts and reason:
```

## 5. 许可证与成熟度门

| 类别 | 项目 | 门禁 |
|---|---|---|
| Apache-2.0 | Univer、office-ai-agent、Rowboat | 核查 NOTICE、传递依赖、Pro/开源边界、商标 |
| MIT | UFO、OpenAdapt、Office-Word-MCP、TaskWeaver | 保留许可；归档项目不得承担生产关键路径 |
| GPL-3.0 | OfficeDex | 默认只做黑盒研究；复制/链接前需法务结论 |
| AGPL/商业 | ONLYOFFICE DocumentBuilder | PoC 前确认部署、分发、网络使用和商业许可 |
| 研究论文 | WorldGUI、Fine-Print | 用于威胁模型/验收，不视为软件依赖 |

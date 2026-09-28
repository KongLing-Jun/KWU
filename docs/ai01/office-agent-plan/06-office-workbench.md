# 阶段 6：Office Workbench 前端工作台

> 阶段目标：将当前大面板拆成任务、运行、Artifact、审批和交付五个可独立测试的工作区；Wails 与 HTTP 使用同一业务合同。

## P6-1 前端模块拆分与状态边界

**目标**：从 `HeartbeatPanel` 提取 domain store、query/mutation 层和路由级页面。

| 方面 | 实施内容 |
|---|---|
| 前端 | 拆分 TaskList、TaskEditor、RunCenter、ArtifactWorkspace、ApprovalCenter；服务端状态与草稿状态分离。 |
| 后端 | 保持 DTO 稳定，提供 capability/version endpoint。 |
| 数据库 | 无新增表；校验查询能支持分页、筛选和详情。 |
| 测试 | 组件合同、store reducer、错误边界、路由恢复、旧功能回归。 |
| 借鉴项目 | `REF-OFFICEDEX`：Workspace 信息架构（仅黑盒）；`REF-REASONIX`：现有 React 19/Wails bridge。 |

**验收**：各工作区可独立加载/失败/重试；编辑草稿不污染正式任务。

## P6-2 任务向导

**目标**：用分步向导配置数据源、诊断、模板、计划和交付，并在保存前执行本地+后端校验。

| 方面 | 实施内容 |
|---|---|
| 前端 | Stepper、草稿自动保存、能力探测、摘要确认、字段级错误。 |
| 后端 | ValidateTask、capability discovery、乐观锁保存。 |
| 数据库 | draft 使用独立本地/草稿存储；正式保存才写 tasks。 |
| 测试 | 前进/返回、刷新恢复、冲突合并、无权限、配置门控、键盘操作。 |
| 借鉴项目 | `REF-OFFICEDEX`：连续办公任务体验；`REF-ROWBOAT`：连接器配置产品流；`REF-REASONIX`：Task DTO/bridge。 |

**验收**：用户在提交前可看到最终执行摘要；409 不会静默覆盖。

## P6-3 运行中心与事件恢复

**目标**：提供可分页 Run 列表、阶段进度、事件时间线、取消与重试；断线后从游标恢复。

| 方面 | 实施内容 |
|---|---|
| 前端 | sequence/cursor 去重，实时事件只做增量，终态回源查询。 |
| 后端 | Wails/HTTP 共用 Run query/event projection；typed error。 |
| 数据库 | 使用 runs/events 索引分页，不扫描目录。 |
| 测试 | 丢事件、乱序、重复、断线、重启、取消竞态和 10 万 Run 分页。 |
| 借鉴项目 | `REF-OFFICEDEX`：任务工作台体验；`REF-REASONIX`：eventwire、Topic、Wails events、archives。 |

**验收**：刷新/断线不丢终态；相同事件不会重复渲染。

## P6-4 Artifact 预览、对比与版本恢复

**目标**：按格式提供结构化预览、版本 diff、验证收据和恢复操作。

| 方面 | 实施内容 |
|---|---|
| 前端 | XLSX 网格、DOCX/PPTX 预览、版本树、语义/视觉 diff、大文件流式加载。 |
| 后端 | 预览 DTO/缩略图、diff API、受控 restore；下载走流或本地安全句柄。 |
| 数据库 | 查询 Artifact/version/validation，不存大文件。 |
| 测试 | 大文件、损坏预览、旧 schema、版本恢复、路径越界和 Base64 禁用。 |
| 借鉴项目 | `REF-UNIVER`：React 结构化预览 PoC；`REF-OFFICEDEX`：内嵌预览（黑盒）；`REF-OFFICE-AI-AGENT`：diff/version；`REF-REASONIX`：受限预览。 |

**验收**：预览成功与验证/保存状态明确区分；恢复创建新版本而非覆写历史。

## P6-5 审批与交付中心

**目标**：在单一工作区完成内容核对、策略说明、批准/拒绝和交付追踪。

| 方面 | 实施内容 |
|---|---|
| 前端 | Artifact hash、目标、分类、有效期、审计时间线和重试动作。 |
| 后端 | 所有动作再次经过 Policy Engine；前端权限仅用于展示。 |
| 数据库 | 读取 approval/delivery 状态机与审计链。 |
| 测试 | 过期页面、hash 改变、权限撤销、重复点击、跨租户和屏幕阅读器。 |
| 借鉴项目 | `REF-ROWBOAT`：渠道状态聚合；`REF-REASONIX`：approval/event/credential；`REF-FINEPRINT`：监督失效反例。 |

**验收**：页面陈旧时后端拒绝动作并可恢复到最新状态。

## P6-6 Wails/HTTP 合同一致性

**目标**：同一 use case 通过 Wails 与 HTTP 获得等价 DTO、错误与权限行为。

| 方面 | 实施内容 |
|---|---|
| 前端 | transport adapter 可切换；页面不感知传输差异。 |
| 后端 | use-case service 为唯一实现；bridge/router 只做映射。 |
| 数据库 | 无特定变更。 |
| 测试 | 双 transport 合同矩阵、错误码、分页、取消和 capability 对齐。 |
| 借鉴项目 | `REF-REASONIX`：现有 Wails/HTTP 双宿主和 event projection；`REF-UNIVER`：Facade 隔离思想。 |

**验收**：合同测试证明字段、状态机和错误语义等价。

## P6-7 可访问性、国际化和非默认状态

**目标**：覆盖键盘、焦点、缩放、中英文、弹窗和窗口恢复等真实桌面状态。

| 方面 | 实施内容 |
|---|---|
| 前端 | WCAG 基础、焦点管理、ARIA、200% 缩放、长中文/英文、空/错/加载态。 |
| 后端 | 错误提供稳定 code 和可本地化参数，不返回拼接后的唯一文案。 |
| 数据库 | 用户偏好独立存储，不改变业务数据。 |
| 测试 | 键盘 E2E、axe、焦点陷阱、多窗口尺寸、弹窗遮挡、恢复会话。 |
| 借鉴项目 | `REF-WORLDGUI`：非默认初态/焦点/弹窗测试；`REF-OFFICEDEX`：同类桌面体验对照；`REF-REASONIX`：前端合同测试。 |

**验收**：关键旅程可仅用键盘完成；默认态之外的失败/恢复均有自动化覆盖。

## 阶段检查点

- [ ] 五个工作区职责独立，旧大面板不再承担全部状态。
- [ ] Wails/HTTP 合同矩阵通过。
- [ ] 运行断线恢复、Artifact 大文件预览和陈旧审批已覆盖。
- [ ] GPL 项目仅用于黑盒体验研究，无代码复制。

**退出门**：完成真实桌面窗口 E2E 与可访问性审查后进入发布阶段。

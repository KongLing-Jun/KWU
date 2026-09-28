# 文档中心

本目录同时保存面向用户的稳定文档、工程参考、发布手册、在研计划和历史记录。本页是统一入口；根目录 `README.md` 只保留最常用的公开文档链接。

## 稳定文档

### 用户指南

| 主题 | English | 简体中文 |
| --- | --- | --- |
| 使用指南 | [GUIDE.md](GUIDE.md) | [GUIDE.zh-CN.md](GUIDE.zh-CN.md) |
| Bot 接入 | [BOT_GUIDE.md](BOT_GUIDE.md) | [BOT_GUIDE.zh-CN.md](BOT_GUIDE.zh-CN.md) |
| 配置路径 | [CONFIG_PATHS.md](CONFIG_PATHS.md) | [CONFIG_PATHS.zh-CN.md](CONFIG_PATHS.zh-CN.md) |
| 推理语言 | [REASONING_LANGUAGE.md](REASONING_LANGUAGE.md) | [REASONING_LANGUAGE.zh-CN.md](REASONING_LANGUAGE.zh-CN.md) |
| 生产迁移 | [MIGRATING.md](MIGRATING.md) | — |

中文专题文档：

- [工具审批模式](TOOL_APPROVAL_MODES.zh-CN.md)
- [协作模式](COLLABORATION_MODES.zh-CN.md)
- [目标约束](GOAL_ENFORCEMENT.zh-CN.md)

### 工程与架构

- [工程规范](SPEC.md)
- [项目结构](PROJECT_STRUCTURE.md)
- [业务、架构与接口全景](PROJECT_BUSINESS_ARCHITECTURE.md)
- [架构决策记录](decisions/README.md)
- [会话引用架构](SESSION_REFERENCE_ARCHITECTURE.md)
- [会话记忆检索](SESSION_MEMORY_RETRIEVAL.md)
- [Checkpoint 与回退](CHECKPOINTS.md)
- [CodeGraph 图谱](codegraph/README.md)

### 发布

- [发布模型与流程](RELEASING.md)
- [桌面发布手册](releases/desktop.md)
- [桌面版本说明](releases/README.md)

## 在研与历史材料

这些目录不是当前产品行为的权威说明。实现与文档冲突时，应先核对代码、测试和上方稳定文档。

| 目录 | 用途 | 维护约束 |
| --- | --- | --- |
| [`fix/`](fix/) | Code Guard 治理台账 | 未完成项仍保留，完成后压缩为结论与验收记录 |
| [`guard/`](guard/) | 守卫机制的阶段计划 | 仅保存仍需执行或核对的内容 |
| [`ai02/`](ai02/) | 0.9.0 模型消费闭环的开发任务包 | 过渡目录；禁止继续使用个人/代理编号创建新目录 |
| [`ai01/`](ai01/) | 尚在整理的架构材料 | 过渡目录；稳定内容迁入工程与架构分类 |
| [`ai03/`](ai03/README.md) | 分阶段计划、任务合同与实施进展 | 以 Git/PR 和当前代码为事实源；完成后提炼稳定结论并归档 |
| [`archive/`](archive/README.md) | 已结束且仍需追溯的记录 | 只读维护，不作为当前实现依据 |

## 文档管理规则

1. 顶层只放稳定、长期有效并可被项目入口引用的文档。
2. 新功能的临时计划放入 `todos/<编号>/<主题>/`，优先提供 `goal.md` 或索引文件。
3. 架构决策优先更新现有权威文档；确需 ADR 时再建立统一的 ADR 序列，不在多个目录重复记录同一决策。
4. 发布说明统一放入 `releases/`；历史审计、失效流程和已结束记录统一放入 `archive/`。
5. 不使用人员、代理编号或本机路径命名长期目录，不提交会话 ID、凭据或内部临时链接。
6. 移动或删除文档前先检查仓库内引用；修改后至少运行 Markdown 本地链接检查和 `git diff --check`。

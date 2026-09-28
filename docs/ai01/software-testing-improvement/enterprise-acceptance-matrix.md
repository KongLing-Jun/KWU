# 政企桌面端验收矩阵

> 状态：Proposed
>
> 用法：产品、交付、安全和研发先确认“支持/不支持/按客户启用”，再把支持项绑定到自动化或人工证据。未确认项不能被描述为产品承诺。

## 1. 环境矩阵

| 维度 | 候选场景 | 默认级别 | 证据 |
|---|---|---|---|
| OS | 合同支持的 Windows Client/Server build | Release blocker | VM/真机、OS build、安装与业务 smoke |
| 架构 | amd64、arm64 | Release blocker（若发布） | 真实机器启动，不只 PE 架构检查 |
| 权限 | 普通用户、管理员、受限目录 | Release blocker | 安装退出码、文件/注册表、首次启动 |
| 路径 | 中文、空格、长路径、非系统盘 | Release blocker | 安装、更新、文件导入导出 |
| WebView2 | 已安装、缺失、损坏、升级 | Release blocker | 依赖检测、提示、修复和启动 |
| 网络 | 在线、离线、断网恢复 | Release blocker | 状态机、用户提示、数据一致性 |
| 企业网络 | HTTP(S) 代理、自定义 CA、DNS 限制 | Contract dependent | 专用环境或可重复代理 fixture |
| 安全软件 | Defender、客户 EDR、应用白名单 | Contract dependent | 告警、签名、hash、申诉材料 |
| 显示 | DPI、缩放、多显示器、RDP | P1/合同相关 | 视觉与交互 smoke |
| 生命周期 | 锁屏、休眠/唤醒、系统重启 | P1 | 任务恢复、锁释放、数据一致性 |
| 国产 OS | 指定发行版/版本/架构 | 未默认承诺 | 真实目标机全链路验证 |

## 2. 交付生命周期

| 场景 | 必须断言 |
|---|---|
| 全新安装 | 签名可信、目录正确、首次启动、WebView2、卸载项 |
| 覆盖升级 | 版本、配置、会话、SQLite、快捷方式和注册表保持正确 |
| 更新失败 | 下载/签名/文件锁/安装中断不破坏当前可运行版本 |
| 回滚 | 明确自动、人工或备份恢复策略；数据 schema 兼容 |
| 卸载 | 程序资源清理；用户数据按产品策略保留或删除 |
| 离线部署 | 安装包、依赖、SBOM、hash、签名验证无需公网 |
| 灾难恢复 | 备份、恢复、日志和诊断包能支持现场处置 |

## 3. 安全与审计

| 证据 | 内容 | Owner |
|---|---|---|
| 来源 | commit、tag、workflow、构建工具版本 | Release |
| 制品 | 文件名、大小、SHA-256、架构 | Release |
| 信任 | Authenticode、时间戳、Minisign | Security/Release |
| SBOM | CycloneDX/SPDX、生成工具与版本 | Security |
| 漏洞 | 数据库时间、发现、可达性、例外和 SLA | Security/研发 |
| 许可证 | direct/transitive、allow/deny/review | 法务/研发 |
| 测试 | 命令、OS、耗时、通过/失败/跳过 | QA/研发 |
| 数据 | 测试数据来源、脱敏和保留期限 | Security/QA |

## 4. 长稳验收

| 周期 | 场景 | 最低观察 |
|---|---|---|
| PR/短测 | 关键生命周期循环 | 无 crash、race、明显泄漏 |
| Nightly 8h | 空闲 + 任务 + 断网恢复 | 内存、goroutine、句柄、磁盘趋势 |
| Release 24h | 真实桌面进程和支持环境 | P95/P99、资源趋势、错误分类 |
| 重点版本 72h | 目标客户硬件/EDR/代理 | 稳态、恢复、诊断和人工复核 |

## 5. 场景记录模板

```text
Scenario ID:
Contract / support policy:
OS build and architecture:
Hardware / VM:
User privilege:
Network / proxy / CA:
EDR / application control:
WebView2 version:
Installer and SHA-256:
Source commit:
Steps and expected results:
Automation command:
Artifacts:
Result: pass / fail / blocked / not-run
Failure classification:
Owner and due date:
Approver:
```

# Windows 代码签名与发布流程进度

## 当前状态

- 阶段：Stage 3 实施交付
- 状态：实现、Windows 双架构实机验收和最终门禁已完成，等待 Draft PR 审查合并
- 范围：Windows 签名、Draft/Publish、撤回保护、验收与开发者手册
- 稳定决策与验收：[plan.md](./plan.md)

## 阶段与关口

| 项目 | 状态 | 下一关口 |
| --- | --- | --- |
| 源码与运行环境调查 | 完成 | 已固化到计划 |
| 用户决策确认 | 完成 | 已确认本地签名、云端发布、平台、验收和撤回边界 |
| Stage 1 文档 | 完成 | 方案已经审查并合并 |
| Stage 2 任务拆解 | 完成 | 任务合同已合并 |
| Stage 3 实施 | 完成 | Draft PR 审查与合并 |

## 任务和 PR 映射

| 任务 | 实现 PR | 内部顺序 | 状态 |
| --- | --- | --- | --- |
| [001 Windows 签名与发布闭环](./tasks/001-完成Windows签名与发布闭环.md) | 一个实现 PR | 签名→Draft/Publish→撤回保护→真实验收 | 实现与 Windows 双架构验收完成；macOS 人工验收豁免 |

这是一个顶层发布流程任务，不按脚本或平台拆 PR。实现 PR 内允许按检查点形成多个有意义
提交，便于审查和回滚；所有检查点和真实验收完成前，任务保持未完成。

## 已确认决策

- 固定 Windows 电脑不注册 Runner；操作者从受信 checkout 运行一条本地命令。
- Windows 两架构共六次内部签名，验证通过后只上传两个 installer。
- 本机及本人 GitHub 账号的完整仓库权限是明确接受的边界；不新增候选仓库。
- GitHub 创建 Draft，macOS Runner 上传 DMG，本机上传两个 installer；ARM 验收后 Publish。
- 保留 `wails build -nsis`，由 NSIS 钩子签主程序、卸载器和 installer。
- release-only 开关启用三钩子且全部检查退出码；普通开发构建可保持未签名。
- 证书指纹来自本机受限配置，并严格校验主体、用途、私钥和有效期。
- Windows 11 amd64/arm64 使用 SHA-256 与 RFC 3161 SHA-256 时间戳。
- 签名、时间戳、平台构建或验证失败会阻止三个平台正式发布。
- macOS 本期保持未签名 universal DMG，但仍与 Windows 同版本原子发布。
- `v1.0.5` 因当前实际用户仅使用 Windows ARM64，真实 Mac 人工验收经用户批准豁免；
  macOS Runner 自动构建和资产合同仍是发布门槛，不能把 macOS 记为验收通过。
- Draft 候选不可变；ARM 真机验收后 Publish 只复核并发布原文件。
- dry-run 可真实签名；emergency 不得绕过签名和 ARM 验收。
- 正常 Publish 禁止降级；Withdraw 先 OSS 后 GitHub、幂等可续跑，不承诺跨平台原子。
- 安装缓存包前重验 stable manifest，已撤回或网络无法确认时拒绝安装。
- 实现必须交付独立中文操作手册，确保开发者没有 Agent 也能配置签名、管理员 Token、
  完成三平台发布、核验和故障恢复；文档不得包含任何真实凭据。

## 基线与验证快照

| 检查 | 状态 | 结果 |
| --- | --- | --- |
| Codegraph 调查 | 完成 | 索引基线 `12428b7b` |
| Stage 2 源码复核 | 完成 | 在合并方案后的最新 `origin/main-v3` 直接复核任务切片 |
| 当前产物签名 | 完成 | installer、已安装主程序、卸载器均为 `NotSigned` |
| SimplySign 实签探针 | 通过 | 主程序副本 Authenticode `Valid`；Certum RFC 3161 时间戳有效 |
| NSIS 可提取内容 | 完成 | 可解出主程序，不能事后取得卸载器；需使用 `!uninstfinalize` |
| 本地交付入口 | 完成 | 现有脚本已按冻结 Tag 串行构建并不可覆盖地上传两个 Windows Draft 资产 |
| 任务拆解文档 | 完成 | 单任务合同 147 行、LF；最终 Gate 结果以 PR 描述为准 |
| Windows 三层签名 | 通过 | amd64/arm64 共六次真实签名；应用、卸载器、installer、时间戳、架构和证据绑定均通过 |
| Draft/Publish 编排 | 通过 | Draft 先创建、macOS 直传、本机双架构全验后上传；Publish 绑定 ARM 真机哈希且不重建 |
| 安装缓存保护 | 通过 | 安装前要求 OSS/GitHub stable 均与缓存版本、平台、SHA256、大小一致；任何失败失效缓存 |
| Withdraw 编排 | 通过 | 零写入预检后 OSS→坏 Release prerelease→恢复旧 latest；远端状态驱动，可重复续跑 |
| 开发者手册 | 完成 | 独立中文 runbook 覆盖签名机、Token、完整发布、停止条件、恢复和 Withdraw |
| 当前实现候选 | 已生成 | 本地 `v1.0.5-acceptance` 串行实签 amd64/arm64；六层证据、可信签名、版本和载荷架构复验通过，未上传 |
| amd64 实机验收 | 通过 | 从 1.0.4 覆盖升级、启动、数据保留、页面烟测、卸载清理、全新重装和静默更新接管均通过；安装包、主程序、卸载器均为 1.0.5 且 Authenticode/RFC 3161 有效 |
| Win11 ARM64 实机验收 | 通过 | 用户在真实 Win11 ARM64 上确认安装、启动、升级、数据保留、卸载及三层签名检查通过；候选 SHA256 与冻结 arm64 installer 一致 |
| macOS 人工验收 | 风险接受 | `v1.0.5` 不执行真实 Mac 安装/打开；Runner 自动构建、universal 架构和资产合同仍必须通过 |

## 阻塞与下一关口

当前没有技术阻塞。Windows 双架构实机验收和最终门禁已经完成，macOS 真实设备人工
验收按批准口径豁免；下一关口是 Draft PR 审查与合并。合并后仍须由 macOS Draft
路径成功生成并验证 universal DMG，才能进入正式 Publish。

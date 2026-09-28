# Phase 4 安装、升级与兼容性执行状态

> 执行日期：2026-07-23
> 源码提交：`15a98177251bd024b499206e9d90b979fbc48436`
> 安全边界：本轮没有运行安装器、卸载器、注册表写入、升级或真实数据迁移；
> 只执行静态/fixture 契约和只读环境盘点。

## 1. 当前支持与证据矩阵

| 维度 | 场景 | 当前证据 | 状态 |
|---|---|---|---|
| OS | Windows 11 10.0.26200 amd64 | 开发版原生窗口和进程可见 | 部分通过，不代表安装版 |
| OS | Windows Server、其他 Windows build | 无目标机 | 未验证 |
| 架构 | amd64 | 合成 PE 与 NSIS payload 架构契约通过 | 静态通过，真机安装未测 |
| 架构 | arm64 | 合成 PE 与 NSIS payload 架构契约通过 | 静态通过，真机/VM 未测 |
| 权限 | 普通用户 per-user 安装 | NSIS 使用 `REQUEST_EXECUTION_LEVEL user`、HKCU、`$LOCALAPPDATA` | 静态通过，真实安装未测 |
| 权限 | 管理员、受限目录、应用白名单 | 无执行证据 | 未验证 |
| 路径 | 默认、中文、空格、自定义盘符 | installer contract 与 NSIS 分支存在 | 静态通过，真实安装未测 |
| 网络 | 离线、代理、自定义 CA、DNS 失败 | updater/HTTP 包有部分单测 | 安装/首次启动矩阵未验证 |
| WebView2 | 已安装 | 当前机器运行时 150.0.4078.83 可启动 | 开发版通过 |
| WebView2 | 缺失、损坏、过旧 | 无可恢复 VM 快照 | 未验证 |
| 安全软件 | EDR、白名单、杀毒 | 无客户环境 | 未验证 |
| 升级 | 当前路径保持、legacy 名称迁移、便携更新 | NSIS/前端静态契约通过 | 未执行 N-2/N-1 → N |
| 卸载 | 程序、快捷方式、HKCU、WebView2 数据 | NSIS 静态路径与前端契约 | 未执行真实卸载 |

未验证项不得写成已支持；Windows Server、ARM64、EDR、代理和自定义 CA 是否
成为 release blocker，仍需要产品/交付 owner 根据合同确认。

## 2. 本轮执行结果

通过：

```text
scripts/assert-windows-release-architecture-test.ps1
Windows application and NSIS payload architecture tests passed

scripts/desktop-actions-common-test.ps1
scripts/desktop-actions-draft-test.ps1
scripts/desktop-actions-publish-test.ps1
scripts/desktop-actions-doctor-test.ps1
PASS

go test ./internal/update -count=1
PASS

pnpm exec tsx src/__tests__/installer-contract.test.ts
9 passed, 0 failed
```

架构测试最初在 Windows PowerShell 5.1 的预期失败用例中被 redirected native
stderr 提前终止。只修改测试脚本，在调用子 PowerShell 时临时使用
`ErrorActionPreference=Continue`，随后仍以子进程退出码判断正负用例；复跑通过。
生产架构校验脚本未修改。

### 下载、签名、文件锁和启动中断

现有 updater/signature 测试与新增 Windows 文件锁测试定向执行通过：

```text
go test . -run '^Test(WriteAtomicPreservesLockedDestination|\
ApplyWindowsFileReportsLaunchFailure|DownloadUpdateFallsBackAcrossVerificationStages|\
DownloadUpdateStopsFallbackWhenParentCancelled|DownloadUpdateAggregatesAssetFailures|\
CachedUpdateRejectsTamperedArtifact|DownloadRecoversFromMidStreamReset|\
DownloadGivesUpAfterCap|RetryTransientStopsWhenCancelled|DownloadResumesWithRange)$' \
-count=1
PASS

go test ./cmd/sign ./internal/update -count=1
PASS
```

已证明：

- 下载中途断连可重试并用 Range 续传，达到上限后失败；
- context 取消会停止 fallback；
- asset、signature、signature verification 和 SHA-256 失败会切换来源或聚合；
- 篡改缓存制品会被拒绝；
- 签名成功、篡改、错误公钥和签名后自校验失败均有测试；
- Windows 目标文件被独占锁定时，`writeAtomic` 返回错误，旧制品保持不变，
  临时文件被清理；
- 安装器路径不存在时，`applyWindowsFile` 返回启动错误。

这些测试没有执行真实 NSIS 安装。`applyWindowsFile` 在
`installerCommand(...).Start()` 成功后立即返回，不等待安装器退出，也没有
安装结果回传，因此无法从当前 App API 观察“子安装器已启动但随后中断/失败”。
T4.5 继续保持未完成。

未通过：

```text
scripts/desktop-release-doc-test.ps1
ParserError under Windows PowerShell 5.1
```

仓库文档契约明确要求 PowerShell 7，但当前工作机没有 `pwsh`。BOM-less UTF-8
脚本被 Windows PowerShell 5.1 按旧编码解析，中文字符串破坏引号。该结果归类为
环境前置条件缺失，不是契约通过；没有为了绿灯修改文档或生产发布脚本。

## 3. 静态行为审计

当前 NSIS 契约可确认：

- per-user 安装，无需管理员权限；
- 安装路径写入 HKCU 并在更新时恢复；
- legacy `燧元`/`Reasonix` 路径只有在证明指向当前安装目录时才迁移；
- in-app update 最多等待精确 PID 60 秒；
- 便携更新只替换当前目录可执行文件，失败时尝试回滚旧文件；
- 卸载先终止并删除主可执行文件，失败则 abort；
- 卸载删除快捷方式、关联、卸载注册表项和 WebView2 DataPath；
- 安装目录只在空目录时删除，避免误删额外文件。

这些是源码契约，不等价于真实安装、文件锁、UAC、EDR 或中断恢复证据。

### 未关闭问题 INSTALL-TEST-001

- 严重度：High（发布验收证据缺口）。
- 问题：没有当前候选安装器在 Windows amd64/arm64 上的全新安装、首次启动、
  升级、中断、回滚和卸载证据。
- 处理：T4.2～T4.8 保持未完成；需要可恢复 VM/真机和候选制品，不能用合成
  PE 或浏览器 smoke 替代。

### 未关闭问题 INSTALL-TEST-002

- 严重度：Medium（策略未确认）。
- 问题：卸载脚本会递归删除 WebView2 DataPath，但保留安装目录之外的 profile、
  会话、SQLite、凭据和日志。尚无经产品/安全确认的用户可见“保留/清除”策略。
- 风险：用户可能把“卸载”理解为清除全部本地数据，或反过来期望登录状态保留。
- 建议：先确认产品策略，再设计真实卸载前后文件/注册表清单测试；本轮不修改
  生产卸载行为。

### 未关闭问题 INSTALL-TEST-003

- 严重度：Low（测试环境）。
- 问题：本机缺少 PowerShell 7，发布文档契约无法按规定入口执行。
- 建议：在 release runner 固定 `pwsh` 版本并记录版本；Windows PowerShell
  5.1 的解析失败不得当作产品失败或测试通过。

### 未关闭问题 INSTALL-TEST-004

- 严重度：High（升级恢复可观测性）。
- 问题：`applyWindowsFile` 只确认 NSIS 进程成功启动；启动后的非零退出、文件
  锁超时、替换失败或安装中断没有回传给当前 App。
- 当前证据：启动前路径失败和 updater 缓存文件锁已由测试覆盖，但不能代表
  NSIS 中断恢复通过。
- 建议：在可恢复 VM 中使用候选安装器注入锁/终止，并通过安装日志、旧版本
  可启动性、版本/文件 hash 和回滚结果判定；生产回传机制需要单独授权设计。

## 4. 下一次真实执行所需证据

每个场景至少记录：候选 SHA-256、OS build、架构、权限、安装路径、WebView2
版本、安装/升级/卸载退出码、日志、注册表差异、快捷方式、profile/SQLite
前后快照和恢复结果。涉及 EDR、代理、自定义 CA 或 ARM64 时还需目标环境 owner。

# 任务 001：完成 Windows 签名与三平台发布闭环

## 1. 执行合同

| 项目 | 内容 |
| --- | --- |
| 状态 | 完成；A/B/C、开发者手册、Windows 双架构实机验收和最终门禁均已完成 |
| 输入 | [`../plan.md`](../plan.md)；源码与实签依据见 [`../evidence.md`](../evidence.md) |
| 依赖 | SimplySign 已登录；本机 SignTool 实签探针已通过 |
| 拓扑 | 一个任务、一个实现 PR；内部四个检查点严格串行 |
| 指派 | 单个实现子代理，`gpt-5.6-sol` / medium reasoning |
| 产物 | 签名、Draft/Publish、Withdraw、缓存保护、开发者操作手册和验收回执 |

这是一个完整发布流程改造，不按脚本或平台拆任务。实现 PR 可按检查点形成多个有意义
提交，但所有检查点通过前任务保持未完成。基线为
`origin/main-v3@abc1bc52071c2e3ad201c45d6a8453b4085893e1`；具体缺口见方案与证据。

## 2. 串行检查点

### A. Windows 三层 Authenticode

- `KWU_WINDOWS_RELEASE_SIGNING=1` 才启用签名；默认关闭，开发构建不依赖证书或 SignTool。
- 本机配置只使用 `WINDOWS_SIGN_CERT_THUMBPRINT`、`WINDOWS_SIGN_CERT_SUBJECT`、
  `WINDOWS_SIGNTOOL_PATH`，不得提交或自动改选证书。
- 新增统一 PowerShell 助手完成预检、签名、验证和脱敏 JSON 证据。预检要求指纹唯一、
  Subject 精确匹配、有私钥、证书有效、含代码签名 EKU 且工具可用。
- 应用在 `wails.files` 前经 `!system ... = 0` 签名；临时卸载器经
  `!uninstfinalize ... = 0`；installer 经 `!finalize ... = 0`。
- 固定 `/fd sha256 /tr http://time.certum.pl/ /td sha256`；验证要求
  `signtool verify /pa /all`、Authenticode `Valid`、正确主体/指纹和 RFC 3161 时间戳。
- amd64、arm64 合计恰好六条证据。installer 还要解包验证唯一应用的版本、架构和签名；
  卸载器用生成阶段即时结果，并把证据绑定到 installer SHA256。

### B. Draft、本机签名和 Publish

- Build Draft Workflow 先冻结 Tag/commit 并创建未发布 Draft；删除两个托管 Windows
  构建 Job，只由 `macos-15` 构建、验证并上传未签名/未公证 universal DMG。
- 固定 Windows 电脑不注册 Runner；任何 GitHub 事件都不能调用它。
- 唯一本地入口保持 `scripts/desktop-release.ps1 -Action BuildWindows`。它从干净受信
  checkout 校验来源、Draft、工具和 SimplySign，在隔离 worktree 串行构建 amd64、arm64。
- 六次签名及版本、架构、来源、证据全部通过后才上传两个 installer；任一失败，本次
  零 Windows 资产上传。
- Draft 最终恰好包含两个 installer 和一个 DMG；签名证据写入 SHA256 绑定的 Release
  provenance，不新增第四个主资产。同名同哈希可复用，同名不同哈希不可覆盖。
- Publish 必填与 Tag 绑定的 ARM 确认和 `arm64_acceptance_sha256`，必须等于真实 Win11
  ARM 验收的 Draft 资产；emergency/dry-run 也不能绕过。
- Publish 只读重验三个资产、Windows 签名/provenance、版本和架构，再按现有 minisign、
  manifest、GitHub、OSS、云顺序发布原文件，不重新构建或替换。
- Certum 证书不用于 macOS；文档始终明确本期 DMG 未签名、未公证。

### C. Withdraw 和缓存保护

- `InstallUpdate` 启动 installer 前重新获取并验证 stable manifest；缓存版本、SHA256、
  大小和平台资产必须仍完全匹配。
- stable 已撤回、资产变化、网络/HTTP/签名/解析失败时失效缓存并关闭安装；不因本地已有
  文件而离线继续。保持 manifest/minisign 格式且不自动降级已安装客户端。
- 新增仅手动触发、使用受保护发布 Environment 的 Withdraw Workflow，必填坏版本、
  恢复版本、原因和绑定两者的二次确认。
- 写入前完成全部只读预检；随后固定执行并验证：恢复 OSS stable→坏 GitHub Release
  设为 prerelease/non-latest→恢复旧 latest。
- 每阶段从远端状态判断，已完成则验证跳过；相同输入可在部分失败后安全续跑并报告进度。
- 不删除 Tag/Release/资产、不覆盖历史对象、不写旧云记录、不放宽正常 Publish 单调性。

### D. 真实候选验收与人工交接

- 临时 dry-run Draft 运行完整 Prepare→macOS→本机双架构签名上传，核对三个主资产的
  来源、SHA256、大小和六次签名证据。
- amd64 签名机完成安装、启动、从上一稳定版升级、卸载，核对三份 EXE 发布者和版本。
- 真实 Win11 ARM 对同一个 arm64 Draft 资产记录 SHA256，执行同样四个场景并检查
  Smart App Control/CodeIntegrity；不得用 x64 模拟、关闭 SAC 或替换文件。
- 原“无法确认发布者”阻止不得再出现；SmartScreen 下载信誉提示单独记录。
- macOS Runner 回归 universal DMG 架构和资产合同；`v1.0.5` 的真实 Mac 人工安装/
  打开验收经用户批准豁免。Publish 验证接受 ARM 实测哈希、拒绝错误哈希且不重建。
- 隔离 fixture 证明 Withdraw 可续跑，并模拟撤回后缓存不会启动 installer。
- 不默认公开正式版本；首个正式 Publish 仍需用户针对具体 Tag/资产哈希明确授权。
- 新增独立中文手册 `docs/releases/desktop-signing-publish-runbook.md`，并从现有
  `desktop.md` 链接；目标是开发者在没有 Agent 时也能从零完成一次发布。
- 手册写清一次性安装/权限/本机受限配置和每次 Prepare→macOS→Windows→ARM→Publish
  清单，给出准确命令、输入示例、预期产物/输出、核验点、停止条件和发布后检查。
- 单列 `KWU_CLOUD_RELEASE_TOKEN`：管理员短期 Token 的取得/刷新入口、受保护
  `desktop-release` Environment Secret 配置、本地恢复时仅进程内使用及清除/轮换；不得
  记录真实 Token、证书密码或指纹值，也不得通过 Workflow 输入、命令行或日志传递。
- 手册覆盖 Token 过期、SimplySign 未登录、签名/时间戳失败、Draft 部分完成、云发布
  日志失败后的恢复，以及何时重试、何时 Withdraw；合同测试锁定关键变量和执行顺序。

## 3. 文件边界

允许修改：

- NSIS/project、`desktop-build.sh`、新增 Authenticode 助手及架构验证测试
- Build Draft、Publish Workflow 和新增 Withdraw Workflow
- `desktop-actions-draft.ps1`、`desktop-release.ps1`、
  `desktop-actions-publish.ps1` 及测试；新增 Withdraw 脚本和测试
- installer/Workflow 合同测试
- `desktop/updater_app.go`、复用已验证 manifest 所需的最小 updater 接口及 Go 测试
- `docs/releases/desktop.md`、独立开发者操作手册及文档合同测试
- 必要时只补足 PR Gate release-contract 路径分类及测试；本任务回执和 `progress.md`

禁止修改业务前端、manifest/minisign 格式或密钥、Apple 签名、云数据格式、
`plan.md` 和 `scripts/code_guard_baseline.json`。不得注册 Runner、保存 SimplySign
密码/私钥、远程触发签名机、并行构建 Windows、先上传单架构、让本地命令 Publish、
覆盖资产、自动降级客户端或删除历史发布物。

## 4. 验证与停止条件

自动化必须覆盖：签名默认关闭与证书拒绝、三钩子失败关闭、时间戳和证据脱敏；Workflow
无远程签名、Draft 幂等写入、失败零上传、ARM 门禁和 Publish 不重建；安装前 manifest
复核和网络失败关闭；Withdraw 零写入预检、固定顺序和各部分状态续跑。

迭代按检查点运行最近的 PowerShell、shell、Workflow、frontend 合同和 Go 测试。更新器
修改后从 `desktop` 至少运行：

```powershell
go vet ./...
go test ./...
```

应用行为稳定后由主会话按 D 独立验收；若只有用户能执行 ARM 场景，提供候选哈希、四步
操作、预期发布者和日志观察点，状态保持“等待应用验收”。同步最新 `main-v3` 后运行全部
受影响 release-contract 检查和最终 PR Gate。

若需要导出私钥、无人值守密码、注册 Runner、拆分 Wails/NSIS、降低 ARM/签名门禁、
改变 manifest/minisign 或删除/覆盖历史资产，停止并重新审批。

## 5. 风险、回滚与完成回执

主要风险是开发构建误依赖证书、半套 Windows 资产、Draft metadata 丢更新、ARM 确认
未绑定哈希、离线合法缓存被拒绝和 Withdraw 部分成功。默认关闭、全部验证后再上传、
不可变资产、安装失败关闭和阶段化续跑分别控制这些风险。

可以按同一 PR 的有意义提交逆序回滚，但不得留下 Workflow 已切换而签名未就绪，或
Withdraw 已开放而缓存保护未合并的中间稳定状态。远端状态只能走获批 Publish/Withdraw
恢复，不能靠代码回滚。

| 项目 | 结果 |
| --- | --- |
| 最终状态 | 完成；A/B/C、D 手册和 Windows 双架构实机验收均已完成 |
| 实际修改文件/提交 | A：三层签名与验证；B：Draft-first/macOS 直传、本机双架构原子候选、ARM 哈希门禁和 Publish 复核；C：安装前双 stable 复核、缓存失效、分阶段 Withdraw；D：中文无 Agent 操作手册并停用本地 Finalize；最终以实际 Git 历史为准 |
| 自动化与 PR Gate | A/B/C：签名、架构、Draft、Publish、Withdraw、更新缓存、release/workflow/凭据/文档合同测试通过；最终 Gate 结果在同步最新基线后的 PR 描述记录 |
| 临时 Draft/六次签名 | 当前实现已生成本地 `v1.0.5-acceptance`：amd64 installer SHA256 `b437fcee5d0d9e44ab3a19f650448843bfa9cee5c52191ba996d1c52e9f10728`，arm64 installer SHA256 `eaf6e366afc485a964d78ea715bd4078f6ba21c63b8bb2038483a0f4c68e4406`；应用、临时卸载器、installer、RFC 3161 时间戳、架构和绑定证据均通过，未上传；正式临时 Draft 待 D |
| amd64/Win11 ARM/macOS | amd64 候选已从本机 1.0.4 覆盖升级并保留会话/工作区；三层 EXE 均为 1.0.5、可信签名且带 RFC 3161 时间戳，载荷为 `0x8664`；启动、项目/专家页面、卸载零残留、同用户重装注册表/快捷方式、`/S /UPDATEPID` 等待旧进程退出后接管并自动重启均通过。用户在真实 Win11 ARM64 上确认 arm64 候选安装、启动、升级、数据保留、卸载及三层签名检查通过，SHA256 与冻结候选一致。macOS 真实设备人工验收经批准豁免，不能记为通过 |
| Publish/Withdraw/缓存演练 | Publish 已实现 Tag/ARM SHA256 双门禁且托管 Windows 复验可信签名；Withdraw fixture 证明零写入预检、OSS→GitHub 顺序与恢复状态续跑；缓存 fixture 证明撤回/不一致/网络失败均失效且不启动 installer；真实 Publish 不在本任务默认授权内 |
| 主会话应用验收 | 2026-07-29 由主会话在固定 Windows 签名机完成 amd64 独立验收：候选 installer SHA256 `b437fcee5d0d9e44ab3a19f650448843bfa9cee5c52191ba996d1c52e9f10728`；安装包、已安装主程序、卸载器经共享验证器复核成功；未出现 Smart App Control/无法确认发布者阻止。同日用户在真实 Win11 ARM64 上完成 arm64 验收，installer SHA256 `eaf6e366afc485a964d78ea715bd4078f6ba21c63b8bb2038483a0f4c68e4406`，其余约定场景均确认通过 |
| 首发授权 | 待填写；无具体授权不得公开发布 |
| 偏差与剩余风险 | macOS 仍未签名/公证；由于当前实际用户仅使用 Windows ARM64，`v1.0.5` 的真实 Mac 安装/打开验收经用户明确批准豁免，状态为“未验收、风险已接受”，Runner 自动构建和资产合同不得豁免。公开前具体 Tag/哈希授权仍待用户完成。桌面自动化启动器的 HKCU 与交互用户不同，注册表验收必须由当前用户启动 installer；最终同用户实测 `DisplayVersion=1.0.5`。材料变化必须停止并重新审批 |

实现子代理只能在允许范围修改、验证、更新回执并提交，不得推送、操作 PR/真实发布、
扩面或委派。主会话复核实际提交、diff、EOL、测试和验收；缺陷新增 `fix:` 提交。

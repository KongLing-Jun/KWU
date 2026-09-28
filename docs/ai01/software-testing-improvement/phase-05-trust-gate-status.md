# Phase 5 安全、供应链与发布门禁执行状态

> 执行日期：2026-07-23
> 源码提交：`15a98177251bd024b499206e9d90b979fbc48436`
> 安全边界：未读取生产密钥、未迁移凭据、未签名、未上传、未发布。

## 1. 结论

当前源码和更新契约具备 SHA-256、Minisign 验证和人工发布流程。`govulncheck`
已固定为 `v1.6.0` 并通过 `go run` 执行；`gosec v2.28.0` 已通过仓库
`output/` 下临时二进制完成报告模式 PoC。本机没有全局安装的 Syft、
CycloneDX、Minisign 或 SignTool。现有开发版
可执行文件未签名；没有最终安装器、SBOM、漏洞数据库快照或完整许可证审批，
因此 Phase 5 不能通过发布门。

## 2. 本轮只读检查

| 检查 | 结果 | 解释 |
|---|---|---|
| Root `go mod verify` | 通过 | `all modules verified` |
| Desktop `go mod verify` | 失败 | 本地 `kwu v0.0.0 => ../` 报 `missing ziphash` |
| Root fixed-version govulncheck | 失败/发现 | 3 个可达漏洞，退出码 3 |
| Desktop fixed-version govulncheck | 失败/发现 | 2 个可达漏洞，退出码 3 |
| Root/Desktop fixed-version gosec | 报告完成 | 393/153 条 analyzer finding，不能直接视为漏洞数 |
| frontend production license inventory | 命令通过 | 仅清单，不等于许可证获批 |
| 开发版 Authenticode | `NotSigned` | 开发制品，不代表最终安装器，但不能作为发布证据 |
| SHA-256 | `5d9eb9f4769ba339c8acf8069b2e0dedc4708eaac5ef6820bce055bbae84015f` | 只对应当前开发 exe |
| 签名凭据 fixture | 失败 | Windows PowerShell 5.1 缺少 `ProcessStartInfo.ArgumentList` |
| 生产 CDP/debug guard | 通过 | 测试诊断标记未进入生产路径 |

Desktop `go mod verify` 的失败没有通过删除 `replace`、忽略模块或修改 checksum
绕过。Root 与 Desktop 结果分别报告。

## 3. 工具与门禁盘点

本机：

```text
govulncheck: v1.6.0 via pinned go run / CI install
gosec: v2.28.0 report-mode PoC via temporary output binary
syft: missing
cyclonedx-gomod: missing
minisign: missing
signtool: missing
pwsh: missing
```

CI 当前只在 root broad/full 变更时安装固定的
`golang.org/x/vuln/cmd/govulncheck@v1.6.0`，仍设置
`continue-on-error: true`。[Go 漏洞扫描与例外策略](vulnerability-triage-policy.md)
已定义 SLA 和例外字段，但尚未获得 Security/Release owner 批准。Nightly 和
release workflow 仍没有 SBOM/gosec/许可证门禁。

gosec Root/Desktop 分别产生 393/153 条 analyzer finding；初筛与六个需要
独立跟进的候选组见
[gosec 报告模式 PoC](phase-05-gosec-poc.md)。当前没有把数量直接设为门禁
阈值，也没有加入 suppress。

### 未关闭问题 SUPPLY-TEST-001

- 严重度：High（发布阻断）。
- 问题：最终 Windows 安装器没有 Authenticode、时间戳与签名后复核证据。
- 边界：当前 `NotSigned` 仅针对开发 exe；真正结论必须对最终候选安装器和
  解包后的主程序执行。

### 未关闭问题 SUPPLY-TEST-002

- 严重度：High（发布阻断）。
- 问题：没有最终安装器/解包目录的 CycloneDX 或 SPDX SBOM，也没有固定
  漏洞数据库的扫描报告。
- 影响：无法离线回答候选制品包含什么、扫描使用哪一版数据库。

### 未关闭问题 SUPPLY-TEST-003

- 严重度：High（当前实际发现发布阻断漏洞）。
- 问题：版本已固定为 `v1.6.0`，但 Root/Desktop 扫描分别报告 3/2 个可达
  漏洞，且 CI 仍为非阻断。
- 证据：[固定版本 Go 漏洞扫描发现](phase-05-vulnerability-findings.md)。
- 处理：按当前用户边界未升级生产依赖或 Go 工具链；修复并重新扫描前不得
  把 `continue-on-error` 结果解释为发布通过。

### 未关闭问题 SUPPLY-TEST-004

- 严重度：Medium。
- 问题：Go 1.26.4 下 Desktop `go mod verify` 对本地 replace 报
  `kwu v0.0.0: missing ziphash`。
- 建议：在 CI 的同版本环境复现并确定验证本地 replace 与外部模块的正确拆分；
  不应简单跳过整个 Desktop module。

### 未关闭问题 SUPPLY-TEST-005

- 严重度：Medium（release runner 前置条件）。
- 问题：签名凭据脚本依赖 PowerShell 7/.NET 的 `ArgumentList`，本机只有
  Windows PowerShell 5.1，fixture 无法按规定运行。
- 建议：release runner 固定 `pwsh` 版本；本轮未修改凭据生产脚本。

### 未关闭问题 SUPPLY-TEST-006

- 严重度：High（更新兼容性）。
- 问题：当前 updater 只嵌入一个 Minisign 公钥，没有新旧公钥重叠、key-id
  选择或双签机制。临时密钥测试证明：旧客户端拒绝新密钥签名，新客户端也
  拒绝旧密钥签名。
- 证据：`TestVerifyWithKeyRotationHasNoCompatibilityWindow` 使用两组临时
  身份验证四种组合；`TestReencryptSigningKeyPreservesKeyAndRefusesOverwrite`
  仅证明同一私钥更换加密口令后仍匹配原公钥，不等于签名身份轮换。
- 影响：直接替换嵌入公钥会让尚未升级的客户端无法验证新制品；若没有预先
  发布包含新旧信任锚的过渡版本，无法安全完成轮换或灾备切换。
- 建议：由安全与发布 owner 评审带版本的信任锚集合、双签/多签 manifest 或
  其他显式过渡协议，并定义旧键撤销、恢复演练和兼容期限。本任务不修改生产
  验证协议。

## 4. Frontend 许可证初筛

`pnpm licenses list --prod --json` 返回：

| License | 包数 | 初筛 |
|---|---:|---|
| MIT | 126 | allow 候选 |
| ISC | 4 | allow 候选 |
| BSD-2-Clause | 1 | allow 候选 |
| BSD-3-Clause | 1 | allow 候选 |
| GSAP standard license 文本 | 2 | review |

### 未关闭问题 LICENSE-TEST-001

- 严重度：Medium（法务/发布评审）。
- 问题：`gsap` 与 `@gsap/react` 使用自定义 standard/no-charge license 文本，
  不能自动归入 MIT/BSD allow。
- 处理：保持 `review`；需要确认政企商业分发、离线部署和再分发条款。已建立
  [Frontend 生产依赖许可证策略](frontend-license-policy.md)和机器分类器，
  T5.7 的清单要求完成；当前策略仍未覆盖 Go、Wails、NSIS、打包工具和随附
  二进制，不能据此通过最终发布。

## 5. 发布信任顺序

候选顺序必须是：

```text
不可变源码/标签
→ 构建
→ Authenticode + 时间戳
→ 签名后验证
→ 最终安装器与解包目录 SBOM
→ 固定数据库漏洞/许可证复核
→ SHA-256
→ Minisign manifest
→ 人工批准与发布
```

任何会改变二进制字节的步骤必须在最终 SHA-256 和 Minisign 之前完成。
未签名、不完整 SBOM、未处置严重发现或缺少来源 SHA 时应阻断发布候选。

## 6. 离线证据包完整性契约

新增 `scripts/testing-release-evidence-policy.mjs`。它只读取本地
`bundle-manifest.json`，不访问网络，并以 fail-closed 方式检查：

- manifest schema 和 40 位小写来源 Git SHA；
- Windows amd64/arm64 安装器、Authenticode 报告、Minisign、安装器/App
  SBOM、漏洞报告、许可证报告、测试报告和人工批准记录等 14 个角色；
- 每个证据文件必须使用规范化相对路径，不允许绝对路径、反斜杠、query、
  fragment、`.` 或 `..`；
- 角色和路径不得重复，文件必须存在且 SHA-256 与 manifest 一致。

`scripts/testing-release-evidence-policy.test.mjs` 使用完全合成的临时证据包，
验证完整包通过、篡改文件失败、目录逃逸失败、缺失角色失败，以及 CLI 的
0/1 退出码。该契约只证明证据齐全和哈希一致；它明确声明不能替代
Authenticode/Minisign 密码学验证、漏洞处置、许可证批准或 reviewer 授权。
当前没有最终候选制品，因此 T5.8 仍未完成。

复核命令：

```text
node scripts/testing-release-evidence-policy.test.mjs
node scripts/testing-release-evidence-policy.mjs <bundle-manifest.json>
```

## 7. 尚未完成

- 没有全局安装扫描工具；固定版本模块下载和扫描只落入 Go cache/仓库忽略的
  `output/`。漏洞数据库尚未固定为可离线复核快照。
- 没有生成最终安装器、SBOM 或离线证据包。
- 已用临时密钥验证同身份重新加密路径，并复现真正签名身份轮换没有兼容窗口；
  尚未形成获批的轮换、撤销和恢复协议。
- 没有 Authenticode 证书、时间戳服务或 SignTool。
- Frontend allow/deny/review 工程清单已建立，但 GSAP review 尚无法务决定，
  且最终制品全栈许可证报告尚未生成。
- gosec PoC 已执行，但六个候选组尚未逐项关闭，也未获批进入 required gate。

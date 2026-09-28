# 桌面更新安装源兜底修复计划

## 问题与现状

Windows 1.0.5 用户可从更新横幅下载并校验 1.0.6，但点击“重启安装”后可能收到：

```text
update: GitHub stable manifest unavailable: ... context deadline exceeded
```

检查更新和下载阶段按 OSS→GitHub 顺序兜底；安装阶段却要求所有配置源同时在线并授权
同一缓存。GitHub 临时超时会阻止已经由 OSS 授权、完成 minisign/SHA256 校验的安装包，
并删除缓存，用户只能重新下载或手动运行 installer。

`desktop-v1.0.5` 与 `desktop-v1.0.6` 的更新器相关文件没有差异，因此 1.0.6 仍包含该
失败路径。

## 根因与证据

- Codegraph 在主工作区 `D:\Reasonix`、基线提交
  `8b77ef400325cdf356f9d638bd84129cb9e53cda` 上确认调用链：
  `InstallUpdate` → `applyCurrentCachedUpdate` →
  `readCurrentVerifiedCachedUpdate` → `validateCachedUpdateAgainstStable`。
- `validateCachedUpdateAgainstStable` 遍历 OSS、GitHub，任一源请求失败立即返回错误。
- `readCurrentVerifiedCachedUpdate` 对该错误无条件调用 `invalidateCachedUpdate`，同时
  删除元数据和安装包。
- 该行为由提交 `c7ec0c2c6d88d5e8384c1ae1094f7962a3c2501b`
  `feat:增加桌面撤回与缓存保护闭环` 引入，目的是避免撤回期间由滞后源重新授权旧版本。
- 现有测试 `TestCurrentCachedUpdateInvalidatesOnManifestFailure` 将“OSS 匹配、GitHub
  不可用也必须失败并删缓存”固定成了当前行为；这与产品确认的源优先级不符。
- 在当前基线上运行现有相关测试通过，证明截图路径是既有明确行为，不是安装包损坏。

## 已确认规则

1. OSS stable manifest 是首选授权源。
2. OSS 因超时、DNS、连接或 HTTP 不可用而无法取得有效 manifest 时，才尝试 GitHub。
3. OSS 成功返回 manifest 后，只采用 OSS 结果；若版本、平台资产、SHA256 或大小与
   缓存不符，视为明确拒绝，禁止使用 GitHub 绕过。
4. OSS 不可用时，GitHub 返回完全匹配的 manifest 可授权安装。
5. 两个源都不可用属于可重试网络错误：不启动 installer，也不删除已校验缓存。
6. 任一实际采用的授权源明确拒绝缓存，或本地缓存完整性校验失败时，删除缓存。

## 目标

- GitHub 不可用时，OSS 已授权的 Windows 缓存安装包可正常进入重启安装。
- OSS 不可用时，GitHub 可按顺序兜底授权。
- 明确撤回或资产元数据冲突继续 fail closed，绝不调用 installer。
- 临时网络失败保留缓存，用户重试时无需重新下载。
- 检查、下载、安装三个阶段采用一致的 OSS→GitHub 顺序语义。

## 非目标

- 不修改发布、OSS、GitHub Release 或 Withdraw 工作流。
- 不改变 minisign、SHA256、本地平台、channel 或缓存文件的完整性校验。
- 不调整更新 UI、超时时长或下载重试次数。
- 不为 1.0.5 已发布二进制做原地修改；受影响用户仍可手动安装 1.0.6。

## 实现方案

在 `desktop/updater.go` 将安装授权结果区分为：

- `authorized`：首个成功取得且完全匹配的有序源授权；
- `rejected`：成功取得 manifest，但版本/平台资产/SHA256/大小不匹配；
- `unavailable`：所有有序源都因获取/解析失败而不可用。

验证按 `releaseSources()` 的既定顺序执行。遇到 `unavailable` 继续下一个源；遇到
`authorized` 立即成功；遇到 `rejected` 立即失败，不再尝试后续源。所有源都不可用时
返回包含各源失败原因的可重试错误。

缓存处理显式区分失败类型：本地完整性失败或 `rejected` 时失效缓存；纯
`unavailable` 时保留缓存。使用内部类型或可判定错误，不依赖错误字符串决定清理。

## 影响范围

- `desktop/updater.go`：安装前授权与缓存失效判定。
- `desktop/updater_test.go`：更新有序兜底、明确拒绝和缓存保留用例。
- `desktop/updater_app.go`、前端绑定和 manifest 格式不变。

## 验收场景

1. OSS manifest 匹配、GitHub 超时：授权成功，调用 installer，缓存不被提前删除。
2. OSS manifest 明确版本或哈希不匹配、GitHub 匹配：拒绝安装，不访问或不采用
   GitHub，删除缓存。
3. OSS 超时、GitHub manifest 匹配：授权成功并调用 installer。
4. OSS 与 GitHub 都不可用：拒绝安装但保留缓存；再次调用可继续验证。
5. OSS 不可用、GitHub 明确不匹配：拒绝安装并删除缓存。
6. 本地缓存文件被篡改：拒绝安装并删除缓存。
7. 两个源均匹配：保持成功安装。

## 验证

实现阶段先运行新增/改动的 updater 精确测试，再运行：

```powershell
go test . -count=1
```

目录为 `desktop`。应用验收使用真实 Windows 1.0.6 构建或最接近的可控网络场景，
阻断 GitHub、保持 OSS 可达，验证点击“重启安装”进入 installer；若自动化无法可靠
操控已安装客户端，则提供用户最短验收步骤并标记等待验收。

最终按仓库规则运行代码守卫、diff/EOL 检查和：

```powershell
node ./scripts/pr-gate.mjs --base origin/main-v3 --run all
```

## 风险、回滚与兼容性

- 风险：OSS 不可用且 GitHub 尚未同步撤回时，GitHub 可能授权旧缓存。这是已确认的
  “GitHub 兜底”语义；OSS 可达且明确撤回时不会被绕过。
- 风险控制：仍要求实际采用源的版本、平台、SHA256 和大小全部匹配，且本地缓存必须
  通过 SHA256。
- 数据格式、API 和缓存结构不迁移。
- 回滚可恢复原安装授权函数及测试；不会影响已发布资产或服务端状态。

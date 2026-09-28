# 桌面可选择资产发布与按目标更新迁移计划

## 背景与问题

当前桌面稳定发布把三个主安装包视为不可拆分整体：

- `Clouvia-windows-amd64-installer.exe`
- `Clouvia-windows-arm64-installer.exe`
- `Clouvia-darwin-universal.dmg`

Actions 和本地 `Finalize` 都要求三者齐全，再统一生成一份全局 `latest.json`、发布
GitHub/OSS/云端并推进全平台 stable。结果是任一未选择或暂不可构建的平台都会阻塞
其他可交付资产；GitHub Actions 额度不可用时，本地发布也不能只交付实际需要的
Windows 架构。

预期行为是由发布者明确选择本次资产，能发布的已选资产独立交付，未选资产不构建、
不验证、不阻塞且不改变其更新状态。第一次启用新机制时仍需一次完整迁移版本，让旧
客户端获得支持按运行平台读取独立更新索引的新更新器。

## 当前行为与实现证据

计划基线为最新 `origin/main-v3`
`e19613bb244f5a82842a48be248da8eb5c8c2aee`：

- `.github/workflows/desktop-publish.yml` 只接受 `release_target=full`，并把 Windows、
  macOS 验证作业都设为发布前置条件。
- `scripts/desktop-actions-common.ps1` 的 `PrimaryAssets` 和 `FinalAssets` 固定要求
  三个主安装包及一份 `latest.json`。
- `scripts/desktop-actions-publish.ps1` 与 `scripts/desktop-release.ps1` 都校验完整
  资产集、生成全平台 manifest，并固定覆盖
  `channels/stable/latest.json`。
- `desktop/updater.go` 固定读取 OSS 全局 stable 和 GitHub 全局 Latest 下的
  `latest.json`，所以一份 manifest 只能表达所有平台共享的版本。
- `desktop/cmd/sign` 已支持重复 `--platform`，可生成选定平台的 manifest；
  Universal DMG 已能映射为 `darwin-amd64` 与 `darwin-arm64`。
- `desktop/cmd/publish-cloud-release` 已按平台/架构查询最新记录，并具备单目标版本
  单调、同版本幂等和同版本内容冲突拒绝的基础，但入口目前只支持 `full` 或
  `windows-arm64`。
- `scripts/desktop-candidates.ps1` 已有 Windows 两架构、macOS 和 full 的候选资产
  映射，可作为统一选择模型的基础。
- `desktop-withdraw` 仍只恢复全局 OSS stable 与 GitHub Latest；按目标推进后必须
  同步具备按目标撤回能力。

对索引主工作区 `D:\Reasonix` 查询了 Codegraph。其索引提交为
`6a20a0576c7d9282c0b5b96a415f5ed8197ecd3f`，落后本计划基线，因此仅用于确认
`Manifest` 被签名生成器和更新器共同消费、云端记录被更新器补充使用等调用路径。
以上行为、影响文件和最终验收均以隔离 worktree 的直接源码、测试与 Git 差异为准。

## 目标

- Actions 和本地稳定发布入口统一接受非空目标集合：
  `windows-amd64`、`windows-arm64`、`macos-universal`。
- `all` 只是三个目标的显式别名；空选择必须失败。
- 只构建、取得、检查和发布已选目标；未选目标保持原 stable 与云端记录不变。
- 保持现有构建凭据边界：Windows 安装包继续由固定本地 Windows 发布机构建并签名，
  Actions 可选择并发布已经上传到 Draft 的 Windows 资产；macOS Universal 继续由
  macOS Actions runner 构建，本地入口可发布已经取得的 DMG。
- 同一版本允许分批追加此前未发布的目标，并能以完全相同的版本和目标集合幂等续跑。
- 每个目标独立单调升级，可跳版本但不得降级；同版本同名不同内容不得覆盖。
- 保持 canonical annotated tag、提交属于 `main-v3`、标签提交内发布说明和 minisign
  完整性合同。
- 用一次完整迁移版本把旧客户端迁移到按运行平台读取独立 stable 的新更新器；迁移后
  旧全局 `latest.json` 永久停留在该迁移版本。
- GitHub Actions 额度不可用时，本地 Windows 发布机仍可只发布一个或两个 Windows
  安装包，也可发布已经取得的 macOS Universal 安装包。

## 非目标与边界

- 不处理 PR #254 所涉及的更新器缺陷；这里只改变为支持独立目标版本所必需的更新
  源选择和兼容迁移。
- 不新增 Linux 桌面发布目标，不拆分 Universal DMG。
- 不允许只写某个已选目标的部分正式目的地；GitHub 归档、OSS immutable/stable 和
  对应云端记录仍属于一个目标的完整发布合同。
- 不增加 `SkipCloud`、非 `main-v3`、轻量 Tag 或 emergency source bypass。
- 不改变本地 DPAPI 云令牌的“读取后成功或失败都删除、调用结束恢复原环境”合同。
- 不把 Windows 代码签名证书迁移到 GitHub Actions，也不新增 hosted runner 的
  Windows 签名凭据模型。
- 不自动删除已成功写入的远端对象，也不以回滚掩盖跨目的地中断；恢复方式是幂等
  续跑。
- 实现 PR 不执行真实生产发布；迁移版本的实际发布时间、版本号和生产凭据由后续
  独立发布操作明确批准。

## 统一目标模型

| 发布目标 | 主资产 | manifest 平台键 | 云端记录 |
| --- | --- | --- | --- |
| `windows-amd64` | Windows AMD64 installer | `windows-amd64` | `windows/amd64` |
| `windows-arm64` | Windows ARM64 installer | `windows-arm64` | `windows/arm64` |
| `macos-universal` | Universal DMG | `darwin-amd64`、`darwin-arm64` | `macos/amd64`、`macos/arm64` |

所有入口先把用户输入归一化、去重并按固定顺序排序。确认文本必须精确绑定 Tag 与归一
化目标，例如：

```text
PUBLISH desktop-v1.0.8 TARGETS windows-amd64,windows-arm64
```

实际处理集合与确认集合不一致时必须停止。选择 `windows-arm64` 时才要求该资产的
ARM64 SHA256 确认；未选择时不得要求 ARM64 验收或 SHA。macOS 来源/架构检查同理只
作用于 `macos-universal`。

## 已选发布与索引设计

### 版本化资产与按目标 manifest

每个已选目标生成一份沿用现有 `update.Manifest` 格式的 manifest，不修改 schema：

- GitHub Release 资产：`latest-<target>.json`
- OSS immutable：`releases/<tag>/<target>/latest.json`
- OSS stable：`channels/stable/<target>/latest.json`

macOS manifest 同时包含两个 Darwin 平台键并指向同一 Universal DMG。GitHub 的版本化
Release 继续作为不可变归档；后续部分发布不得改变全局 GitHub Latest，也不再上传或
替换旧全局 `latest.json`。

更新器按运行平台映射到一个发布目标，并以该目标的 OSS stable manifest 作为版本
发现权威。取得目标版本后，再按 manifest 中的版本定位同一 GitHub 版本化 Release
内的 `latest-<target>.json`，作为同版本安装包下载兜底；两个 manifest 必须版本和
目标一致，安装包最终仍须通过内置公钥 minisign 和 SHA256 验证。

云端 latest 继续只补充同版本发布说明，不参与决定更高版本。目标 OSS pointer
不可用时，可回退到冻结在迁移版本的旧 GitHub 全局 manifest，但不会凭云端记录单独
发现后续部分版本。这牺牲了 OSS 故障时的新版本发现能力，却保留安全的下载兜底，
也使发布 pointer-last 和按目标撤回具有明确权威，且无需扩展云 API。

### 同版本分批发布

同一 canonical Tag 可分多次向同一个 GitHub Release 和 OSS 版本目录追加新目标。
每次变更远端前先读取已有资产：

- 同名、同 SHA256：视为已完成。
- 同名、不同 SHA256：拒绝覆盖。
- 名称此前不存在：允许追加。
- 目标当前 latest 比请求版本高：拒绝降级。
- 同版本云端记录或 stable 内容不同：拒绝修改。

发布顺序为完整预检、不可变安装包/签名/目标 manifest、云端记录、目标 stable
指针；任何中断都保留已成功步骤，使用完全相同的 Tag 和目标集合重跑并补齐。未选
目标在所有步骤都不得被读取为必需输入或被写入。

### 发布者权限与门禁

精确确认是发布者对所选资产的最终发布授权。工具必须硬阻止无法安全实现授权的情况：
来源 Tag/分支合同不符、目标集合不符、资产缺失或类型/架构不匹配、确认 SHA 不匹配、
签名或凭据不可用、远端同名不同内容、降级以及同版本身份冲突。

平台实机体验、额外签名/来源质量检查等非上述客观冲突应清楚报告为警告并记录证据，
但不得在发布者已给出精确确认后替发布者否决发布。`DryRun` 必须展示目标归一化结果、
将写入的每个目的地、警告和幂等/冲突判断。

## 兼容迁移

上线新机制后的第一次生产发布必须是一个明确的完整迁移版本：

1. 同时发布三个目标及其目标 manifest、OSS stable 和四条云端记录。
2. 额外生成旧格式全局 `latest.json`，覆盖 OSS 全局 stable，并把该 Release 设为
   GitHub 全局 Latest。
3. 旧客户端通过全局 manifest 升级到包含新目标路由逻辑的迁移版本。
4. 迁移验证通过后，所有后续发布只推进所选目标索引；旧全局 OSS pointer 和 GitHub
   Latest 永久保持在迁移版本。

没有完成迁移版本及公开端验证前，不允许第一次部分 stable 发布。没有升级到迁移
版本的旧客户端只会停留在迁移版本，不会错误收到不属于其平台的后续部分版本。

部分发布不新增独立 bootstrap marker；每次 zero-write 预检都从冻结入口证明迁移
已经建立：

- OSS 全局 `channels/stable/latest.json` 存在、包含四个平台键，并确定迁移版本。
- GitHub 全局 Latest 仍是同一迁移 Tag，且其旧全局 manifest 版本一致。
- 三个目标 stable pointer 都存在，且各自版本不低于迁移版本。

任一证据不成立都 fail closed。第一次完整迁移发布完成公开端验收后，发布者对首次
部分发布给出精确确认；后续目标 pointer 可以独立前进，但上述冻结入口必须保持不变。

## 用户场景与验收标准

| 场景 | 预期结果 |
| --- | --- |
| 只选 `windows-arm64` | 只要求并发布 ARM64；其他 stable/云记录不变 |
| 选两个 Windows 目标 | 本地机可一次发布两包；不要求 DMG |
| 只选 `macos-universal` | 同一 DMG 推进两个 Darwin pointer 和两条云记录 |
| 先发 ARM64，后以同 Tag 加 AMD64 | 第二次只追加 AMD64；ARM64 内容不变 |
| 完全相同请求中断后重跑 | 已完成步骤幂等跳过，缺失步骤补齐 |
| 同名资产内容变化或目标版本降级 | 变更远端前硬失败 |
| 未选目标缺失或验证失败 | 不检查、不阻塞，也不改变该目标状态 |
| Actions 与本地使用同一输入 | 归一化目标、确认文本、资产映射和发布结果一致 |
| 迁移前尝试部分 stable | fail closed，提示先完成完整迁移版本 |
| 迁移后各平台版本不同 | 每个平台只看到自己的最高有效目标版本 |
| 撤回一个目标 | 只恢复该目标 pointer；不改变共享 Release、其他目标或全局迁移版本 |

## 预计影响范围

- Actions：桌面 Draft、Publish、Withdraw workflows。
- PowerShell：`desktop-actions-common/draft/publish/withdraw`、
  `desktop-candidates.ps1`、`desktop-release.ps1` 及对应合同测试。
- Go：`desktop/cmd/sign`、`desktop/cmd/publish-cloud-release`、
  `desktop/updater.go` 及其测试；优先保持 `desktop/internal/update` schema 不变。
- 文档：`docs/releases/desktop.md`、签名发布 runbook 和相关文档合同测试。

最终任务拆分时按“共享目标与 manifest/updater 合同”“Actions 与本地发布/续跑”
“撤回、迁移与文档”划分可审查切片；任何切片都不得单独启用生产部分发布，只有迁移
门禁和全链路验收完成后才可启用。

## 验证计划

### 聚焦自动验证

- PowerShell 发布、候选、workflow、withdraw 和文档合同测试。
- `go test ./cmd/sign`
- `go test ./cmd/publish-cloud-release`
- `go test .`
- `go test ./internal/update`
- 覆盖三个单目标、三个双目标、`all`、空/未知/重复目标、确认不一致。
- 覆盖同版本追加、完全幂等、同名异内容、降级、一个目的地中断后续跑。
- 用本地 HTTP 服务验证每个平台只读取自己的权威 pointer、按已确定版本定位 GitHub
  同版本兜底、云端版本不一致不改变选择，以及 minisign/SHA 失败。
- 复跑本地 DPAPI 交接成功、失败清理和环境恢复合同，确认目标化改造未回归凭据边界。

### 应用级验收

- 对所有七种非空目标组合执行 Actions 合同级 DryRun；对 Windows 单目标和双目标
  执行本地 `Finalize -DryRun`，确认没有未选输入或写入。
- 在隔离测试端完成一次完整迁移发布，再模拟不同版本 Windows AMD64、Windows ARM64
  和 macOS 客户端，确认只看到对应目标版本。
- 模拟同版本分批、跨目的地失败与精确续跑，核对 GitHub、OSS 和云端公开读回状态。
- 在一个 Release 同时包含多个目标时撤回其中一个，确认只改变该目标 pointer，共享
  GitHub Release、其他目标 pointer 和全局迁移入口均不变。
- 生产迁移另行审批；必须记录旧客户端升级、新客户端目标路由、全局 pointer 冻结和
  各目标公开 SHA256 证据。

### PR Gate

每个 PR 在同步最新 `origin/main-v3` 后运行：

```powershell
node ./scripts/pr-gate.mjs --base origin/main-v3 --run all
```

同时记录 Gate 分类和上述聚焦测试；必需检查失败时不得创建或更新 PR。

## 风险与回滚

- 最大风险是迁移顺序错误导致旧客户端失去更新入口。部分发布功能必须默认关闭，直到
  完整迁移版本和公开端验收完成。
- 多目的地无法形成单事务，可能短暂不一致；预检、不可变写入、逐步读回、目标 pointer
  最后推进和同请求续跑共同限制风险。
- macOS 一个资产对应两个客户端架构，任何只推进其中一条 pointer/云记录的状态都视为
  未完成，必须续跑。
- 回滚代码不能撤销已发布不可变资产；运维回滚使用按目标 Withdraw 恢复上一个已验证
  pointer。共享 GitHub Release、不可变资产和追加式云记录均保留，云端版本不匹配时
  不得影响更新器；全局迁移版本不得随部分目标撤回。
- 若新 updater 路由出现问题，停止目标 pointer 推进并恢复受影响目标到迁移版本；
  保留旧全局入口供旧客户端使用，修复后再按同一发布合同推进。

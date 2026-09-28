# 桌面端更新与发布总览

> 操作清单见[桌面端签名与发布操作手册](./desktop-signing-publish-runbook.md)。

## 稳定发布目标

发布者必须显式选择非空目标集合：

- `windows-amd64`
- `windows-arm64`
- `macos-universal`

`all` 是三者的显式别名。所有入口去重并按上述顺序归一化。普通发布确认必须精确为：

```text
PUBLISH desktop-vX.Y.Z TARGETS windows-amd64,windows-arm64
```

首次迁移版本必须选择 `all` 并追加 `BOOTSTRAP`。空选择、未知目标以及把 `all` 与具体
目标混用都会失败。发布入口只接受 canonical annotated Tag，且 Tag 提交必须可从
`main-v3` 到达；不支持轻量 Tag、非 `main-v3` 或 emergency bypass。

## 构建与资产边界

Windows 安装包只能在固定 Windows 发布机上用
`desktop-release.ps1 -Action BuildWindows -ReleaseTargets ...`
构建和签名。GitHub Actions 不持有 Windows 证书，只发布已上传到共享 Release 的已选
Windows 资产。Actions 仅在选择 `macos-universal` 时构建 Universal DMG。本地
`Finalize` 可发布已取得的已选 Windows 或 macOS 资产。

未选目标不构建、不下载、不验证、不阻塞，也不改变 stable pointer 或云记录。只有
选择 `windows-arm64` 时才要求 ARM64 实机确认及对应 SHA256。

## Manifest 与更新发现

每个目标独立生成：

- GitHub Release：`latest-<target>.json`
- OSS immutable：`releases/<tag>/<target>/latest.json`
- OSS stable：`channels/stable/<target>/latest.json`

`macos-universal` 的 manifest 同时包含 `darwin-amd64` 和 `darwin-arm64`，两者必须
指向同一 DMG。Manifest schema 不变。

客户端首先读取运行平台对应的 OSS 目标 pointer。确定版本后，可以读取同一版本化
GitHub Release 的 `latest-<target>.json` 作为下载兜底；版本、平台和资产身份必须
一致。云端 latest 只补充同版本说明，不能选出更高版本或授权安装。缓存安装前只由
当前目标 OSS pointer 重新授权；pointer 撤回、不可用或身份变化均拒绝安装。

目标 pointer 不可用时，客户端可回退冻结在迁移版本的旧全局入口。该回退不会发现
迁移后的部分版本。

## 发布事务与续跑

正式发布先执行完整 zero-write 预检，验证：

- Tag、提交、发布说明、确认文本和目标集合；
- 已有 GitHub 同名资产和 OSS immutable 对象的内容身份；
- minisign、SHA256、云端 readiness；
- 每个目标 pointer 的单调升级与同版本内容一致性；
- 非 bootstrap 发布所需的冻结迁移入口和三个已建立目标 pointer。

所有可发现冲突必须在首次远端写之前失败。通过预检后，顺序固定为：

```text
GitHub/OSS immutable 资产与目标 manifest
-> 已选目标云记录
-> 已选目标 stable pointer
```

同一 Tag 可分批追加此前未发布的目标，包括向已发布的共享 Release 追加主资产、签名和
目标 manifest。同名同 SHA256 幂等复用；同名不同内容拒绝覆盖。中断后必须使用完全
相同的 Tag 和目标集合续跑，已完成步骤跳过、缺失步骤补齐。

`Finalize -DryRun` 执行同一 zero-write 预检，显示规范化目标、每个 GitHub/OSS/云端/
pointer 目的地及幂等或冲突判断，不写远端。

本地非 DryRun `Finalize` 只作为操作员独占回退；不得与 Actions `Desktop publish`、
`Desktop withdraw` 或另一个本地非 DryRun `Finalize` 重叠运行。

## Bootstrap 迁移

第一次启用目标化更新必须发布完整 bootstrap：

1. 同时发布三个目标及四条云记录。
2. 额外发布旧全局 `latest.json`、更新 OSS 全局 stable，并将该 Release 设为
   GitHub Latest。
3. 验证旧客户端升级、新客户端目标路由以及全部公开 SHA256。

以后旧全局 OSS pointer 和 GitHub Latest 永久停留在迁移版本。非 bootstrap 发布必须
证明两者仍指向同一迁移版本，且三个目标 pointer 都存在且不低于迁移版本。

## 按目标撤回

确认文本为：

```text
WITHDRAW <bad_tag> RESTORE <recovery_tag> TARGET <target>
```

撤回只恢复 `channels/stable/<target>/latest.json` 并公开读回验证。它不改变共享
GitHub Release、其他目标或全局 pointer、immutable 资产和追加式云记录。

## 生产操作

实现、测试和 PR 不执行生产发布。bootstrap、后续真实发布和每次 Withdraw 都必须
分别获得明确审批。多目的地不是单事务；失败后保留成功写入的 immutable 对象并按同一
请求续跑，不自动删除或回滚远端对象。

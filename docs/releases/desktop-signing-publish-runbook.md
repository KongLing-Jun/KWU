# 桌面端签名与发布操作手册

## 1. 选择并冻结发布身份

从 `main-v3` workflow definition 手动运行 Desktop build draft。填写已有的 canonical
annotated `desktop-vX.Y.Z` Tag，并显式填写非空 `release_targets`。不再支持 Tag push
自动构建、轻量 Tag、非 `main-v3` 提交或 emergency source。

目标规范化顺序为：

```text
windows-amd64,windows-arm64,macos-universal
```

`all` 只能单独使用并展开为三目标。保存 Tag object SHA、peeled commit、发布说明路径
和规范化目标作为本次操作证据。

## 2. 构建已选资产

冻结 Tag 后，先从 `main-v3` 手动运行一次 `Desktop release doctor`，选择
`operation=doctor` 和 `cloud_mode=required`，保存只读检查结果作为本次发布证据。
Doctor 只验证发布环境、凭据和远端可达性，不验证尚未生成的本版本资产。

选择 `macos-universal` 时，Actions 在 macOS runner 构建并验证 Universal DMG，然后
把它追加到共享 Draft 或已发布 Release；未选择时该作业跳过。

选择 Windows 目标时，在固定 Windows 发布机运行：

```powershell
./scripts/desktop-release.ps1 `
  -Action BuildWindows `
  -Version vX.Y.Z `
  -ReleaseTargets windows-amd64,windows-arm64
```

命令只构建、签名和上传已选 Windows 架构。不得把 Windows 证书或签名凭据迁移到
Actions。这里的签名是 Windows Authenticode；命令会在本机完成应用、卸载器和安装器
三层签名及验证，然后自动上传安装包到共享 Draft。同名资产存在时必须下载比较
SHA256；相同则复用，不同则停止。跨平台 `.minisig` 由后续正式发布工作流生成；本机
非 DryRun `Finalize` 仅用于完整发布回退。

## 3. 确认发布

普通确认示例：

```text
PUBLISH desktop-v1.2.3 TARGETS windows-amd64,windows-arm64
```

完整迁移版本确认：

```text
PUBLISH desktop-v1.2.3 TARGETS windows-amd64,windows-arm64,macos-universal BOOTSTRAP
```

只有选择 `windows-arm64` 时填写：

```text
ACCEPT desktop-v1.2.3 ARM64
```

并提供实机验收的精确安装包 SHA256。云发布始终为 required，不存在 skip 模式。

## 4. DryRun

在固定 Windows 发布机先运行与正式发布相同目标的 `Finalize -DryRun`。它必须完成
zero-write 预检并列出：

- 规范化目标和所需主资产；
- `latest-<target>.json`；
- GitHub 和 OSS immutable 目的地的 missing/match/conflict 判断；
- 每个目标云记录；
- `channels/stable/<target>/latest.json` 的单调/同版本判断；
- bootstrap 全局目的地（若适用）。

DryRun 不上传、编辑或覆盖任何远端对象。任何冲突都必须在正式发布的首次写入前解决。

## 5. 正式发布

Actions 和本地 `Finalize` 使用完全相同的 Tag、目标集合和确认文本。完整预检通过后：

本地非 DryRun `Finalize` 是操作员独占的完整回退：从命令开始到结束，禁止同时运行
Actions `Desktop publish` 或 `Desktop withdraw`，也不得启动第二个本地 `Finalize`。
启动前必须在 Actions 中确认这两个工作流均无进行中的运行。

1. 追加或复用已选主资产、`.minisig`、GitHub 目标 manifest；
2. 追加或复用 OSS `releases/<tag>/<target>/` 下的 immutable 对象；
3. 发布并读回验证已选目标云记录；
4. 最后推进并验证已选目标 stable pointer。

已发布 Release 仍允许用同一 Tag 分批追加新目标。不得覆盖同名不同内容资产。失败后
用完全相同的请求续跑，不删除已成功对象。

本地云令牌仍通过 Windows CurrentUser DPAPI 文件交接：读取后无论成功或失败都删除
文件，并在调用结束恢复原环境变量。

## 6. Bootstrap 与后续发布

首次迁移必须选择全部目标，并同时写入旧全局 GitHub/OSS manifest。公开验证完成后，
后续发布不得再改变 GitHub Latest 或 OSS 全局 stable。每次部分发布会重新验证冻结的
全局入口和三个目标 pointer；证据不足即停止。

## 7. Withdraw

单目标撤回确认：

```text
WITHDRAW desktop-v1.2.4 RESTORE desktop-v1.2.3 TARGET windows-arm64
```

另填单行原因。操作只恢复该目标 OSS pointer 并公开读回，不编辑 GitHub Release，
不改变其他 pointer、全局迁移入口、immutable 对象或云记录。

## 8. 记录

记录规范化目标、Tag object、commit、所选资产 SHA256、zero-write 预检结果、云记录
读回、目标 pointer 读回和任何续跑步骤。生产 bootstrap、普通发布和 Withdraw 均需
独立审批。

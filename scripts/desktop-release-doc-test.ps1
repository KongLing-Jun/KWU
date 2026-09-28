$ErrorActionPreference = 'Stop'

$guide = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot '..\docs\releases\desktop.md')
$runbook = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot '..\docs\releases\desktop-signing-publish-runbook.md')
$desktopReadme = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot '..\desktop\README.md')
$releasing = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot '..\docs\RELEASING.md')

foreach ($required in @(
    'windows-amd64',
    'windows-arm64',
    'macos-universal',
    'PUBLISH desktop-vX.Y.Z TARGETS',
    'latest-<target>.json',
    'channels/stable/<target>/latest.json',
    'zero-write',
    '同一 Tag 可分批追加',
    '缓存安装前只由',
    'WITHDRAW <bad_tag> RESTORE <recovery_tag> TARGET <target>',
    '-Action BuildWindows',
    'BOOTSTRAP'
)) {
    if (-not $guide.Contains($required)) { throw "desktop release guide is missing: $required" }
}
foreach ($required in @(
    '-Action BuildWindows',
    'ACCEPT desktop-v1.2.3 ARM64',
    'PUBLISH desktop-v1.2.3 TARGETS',
    'Finalize -DryRun',
    '操作员独占',
    'Desktop publish',
    'Desktop withdraw',
    'missing/match/conflict',
    '云发布始终为 required',
    '已发布 Release 仍允许用同一 Tag 分批追加新目标',
    'WITHDRAW desktop-v1.2.4 RESTORE desktop-v1.2.3 TARGET windows-arm64',
    'DPAPI'
)) {
    if (-not $runbook.Contains($required)) { throw "desktop signing runbook is missing: $required" }
}
foreach ($forbidden in @(
    'WINDOWS_SIGN_CERT_THUMBPRINT=',
    'KWU_CLOUD_RELEASE_TOKEN=',
    'MINISIGN_PASSWORD=',
    'gh release delete',
    'release_target: full',
    'confirmation: PUBLISH desktop-v1.0.11',
    'cloud_mode=skip',
    '紧急发布',
    '同时重取 OSS/GitHub stable'
)) {
    if ($runbook.Contains($forbidden)) { throw "desktop signing runbook contains unsafe guidance: $forbidden" }
}
foreach ($forbidden in @('cloud_mode=skip', 'Mode: skipped', 'cloud skip reason', '跳过云')) {
    if ($guide.Contains($forbidden) -or $runbook.Contains($forbidden)) {
        throw "desktop release documentation exposes a forbidden cloud-skip path: $forbidden"
    }
}
if (-not $guide.Contains('desktop-signing-publish-runbook.md')) {
    throw 'desktop release overview does not link the signing/publish runbook'
}
if ($desktopReadme -notmatch 'docs/releases/desktop\.md' -or
    $desktopReadme -notmatch 'Actions-first' -or
    $desktopReadme -match 'GitHub Actions is an\s+inactive fallback') {
    throw 'desktop README does not point to the current dual-track release process'
}
if ($releasing -match 'manual-desktop-v1\.4\.0.*desktop source' -or
    $releasing -match 'desktop Actions workflow is currently inactive' -or
    $releasing -match 'R2_PUBLIC_BASE') {
    throw 'docs/RELEASING.md still describes the retired desktop Actions/R2 flow'
}

$notesGuide = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot '..\docs\releases\README.md')
foreach ($required in @('vX.Y.Z.md', '64 KiB', '不允许 HTML 或图片')) {
    if (-not $notesGuide.Contains($required)) { throw "release notes guide is missing: $required" }
}

Write-Host 'desktop release documentation contract passed'

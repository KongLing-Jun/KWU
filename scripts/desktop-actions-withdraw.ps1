[CmdletBinding()]
param(
    [ValidateSet('ValidateInputs', 'Preflight', 'RestoreOss', 'RestoreGitHub', 'Verify')]
    [string]$Action,
    [string]$BadTag = '',
    [string]$RecoveryTag = '',
    [string]$Reason = '',
    [string]$Confirmation = '',
    [ValidateSet('windows-amd64', 'windows-arm64', 'macos-universal')]
    [string]$Target,
    [string]$ReleaseRepository = 'xiaoxiaotest9527-bit/KWU-release',
    [string]$OssutilPath = 'ossutil',
    [string]$OssBucket = 'kwu-desktop-releases',
    [string]$OssPublicBaseURL = 'https://kwu-desktop-releases.oss-cn-guangzhou.aliyuncs.com',
    [string]$WorkDirectory = ''
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'desktop-actions-common.ps1')

function Get-WithdrawIdentity {
    param([string]$RawTag)
    return Get-ReleaseIdentity '' $RawTag
}

function Assert-WithdrawInputs {
    $bad = Get-WithdrawIdentity $BadTag
    $recovery = Get-WithdrawIdentity $RecoveryTag
    if ($bad.Tag -ceq $recovery.Tag) {
        throw 'BadTag and RecoveryTag must be different'
    }
    $badCode = Get-WithdrawVersionCode $bad.Version
    $recoveryCode = Get-WithdrawVersionCode $recovery.Version
    if ($recoveryCode -ge $badCode) {
        throw "RecoveryTag must be older than BadTag: $($recovery.Tag) !< $($bad.Tag)"
    }
    $null = Assert-ReleaseReason $Reason 'withdraw reason'
    if (-not $Target) { throw 'Withdraw target is required' }
    $expected = "WITHDRAW $($bad.Tag) RESTORE $($recovery.Tag) TARGET $Target"
    if ($Confirmation.Trim() -cne $expected) {
        throw "Withdraw confirmation must exactly match: $expected"
    }
    return [pscustomobject]@{ Bad = $bad; Recovery = $recovery }
}

function Assert-WithdrawReadiness {
    foreach ($name in @('GH_TOKEN', 'OSS_ACCESS_KEY_ID', 'OSS_ACCESS_KEY_SECRET')) {
        if (-not [Environment]::GetEnvironmentVariable($name)) {
            throw "Required withdrawal credential is missing: $name"
        }
    }
    $canPush = (Invoke-Native "inspect $ReleaseRepository permissions with release token" {
        gh api "repos/$ReleaseRepository" --jq '.permissions.push'
    } | Out-String).Trim().ToLowerInvariant()
    if ($canPush -ne 'true') {
        throw "RELEASE_REPO_TOKEN does not have push permission for $ReleaseRepository"
    }
    if (-not (Get-Command $OssutilPath -ErrorAction SilentlyContinue)) {
        throw "ossutil executable does not exist: $OssutilPath"
    }
    Invoke-Native 'read OSS stable pointer with withdrawal credentials' {
        & $OssutilPath cat "oss://$OssBucket/channels/stable/$Target/latest.json" --quiet
    } | Out-Null
}

function Get-WithdrawVersionCode {
    param([string]$RawVersion)
    $identity = Get-ReleaseIdentity $RawVersion ''
    $segments = @($identity.VersionName.Split('.') | ForEach-Object { [int64]$_ })
    return $segments[0] * 1000000L + $segments[1] * 1000L + $segments[2]
}

function Get-WithdrawRelease {
    param([string]$Tag)
    $json = Invoke-Native "read GitHub Release $Tag" {
        gh release view $Tag --repo $ReleaseRepository `
            --json isDraft,isPrerelease,tagName,assets
    }
    return (($json | Out-String) | ConvertFrom-Json)
}

function Get-LatestReleaseTag {
    $json = Invoke-Native 'read GitHub latest Release' {
        gh release view --repo $ReleaseRepository --json tagName
    }
    $release = (($json | Out-String) | ConvertFrom-Json)
    return [string]$release.tagName
}

function Assert-PublishedWithdrawRelease {
    param($Release, $Identity, [switch]$AllowPrerelease)
    if ([string]$Release.tagName -cne $Identity.Tag) {
        throw "GitHub Release tag does not match $($Identity.Tag)"
    }
    if ($Release.isDraft) {
        throw "GitHub Release is still a Draft: $($Identity.Tag)"
    }
    if (-not $AllowPrerelease -and $Release.isPrerelease) {
        throw "Recovery GitHub Release must not be a prerelease: $($Identity.Tag)"
    }
    $names = @($Release.assets | ForEach-Object { [string]$_.name })
    foreach ($name in @($script:DesktopTargetAssets[$Target], "$($script:DesktopTargetAssets[$Target]).minisig", "latest-$Target.json")) {
        if ($names -notcontains $name) { throw "GitHub Release is missing target asset: $name" }
    }
}

function Get-ManifestFileName {
    param([string]$Tag, [string]$Kind)
    return "$Kind-$($Tag.Replace('/', '-'))-latest.json"
}

function Save-PublicFile {
    param([string]$URL, [string]$Destination, [string]$Description)
    $cacheBuster = [guid]::NewGuid().ToString('N')
    $separator = if ($URL.Contains('?')) { '&' } else { '?' }
    try {
        Invoke-WebRequest -Uri ($URL + $separator + "kwu-withdraw-check=$cacheBuster") `
            -OutFile $Destination -TimeoutSec 30 -Headers @{ 'Cache-Control' = 'no-cache' }
    }
    catch { throw "$Description failed: $($_.Exception.Message)" }
}

function Save-GitHubManifest {
    param($Identity, [string]$Destination)
    $directory = Split-Path -Parent $Destination
    New-Item -ItemType Directory -Path $directory -Force | Out-Null
    $downloadDirectory = Join-Path $directory ("gh-" + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $downloadDirectory | Out-Null
    try {
        Invoke-Native "download GitHub manifest for $($Identity.Tag)" {
            gh release download $Identity.Tag --repo $ReleaseRepository `
                --pattern "latest-$Target.json" --dir $downloadDirectory
        } | Out-Null
        Move-Item -LiteralPath (Join-Path $downloadDirectory "latest-$Target.json") -Destination $Destination -Force
    }
    finally { Remove-Item -LiteralPath $downloadDirectory -Recurse -Force -ErrorAction SilentlyContinue }
}

function Assert-WithdrawManifest {
    param([string]$Path, $Identity, [ValidateSet('github', 'oss')][string]$Kind)
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "$Kind manifest does not exist for $($Identity.Tag)"
    }
    try { $manifest = Get-Content -Raw -LiteralPath $Path | ConvertFrom-Json }
    catch { throw "$Kind manifest is not valid JSON for $($Identity.Tag): $($_.Exception.Message)" }
    if ([string]$manifest.version -cne $Identity.Version) {
        throw "$Kind manifest version does not match $($Identity.Version)"
    }
    if ($null -eq $manifest.platforms) { throw "$Kind manifest platforms are missing" }
    $expected = [ordered]@{}
    foreach ($platform in $script:DesktopTargetPlatforms[$Target]) {
        $expected[$platform] = $script:DesktopTargetAssets[$Target]
    }
    $properties = @($manifest.platforms.PSObject.Properties)
    if ($properties.Count -ne $expected.Count) {
        throw "$Kind manifest must contain exactly $($expected.Count) stable platforms"
    }
    foreach ($entry in $expected.GetEnumerator()) {
        $property = $manifest.platforms.PSObject.Properties[$entry.Key]
        if ($null -eq $property) { throw "$Kind manifest is missing $($entry.Key)" }
        $asset = $property.Value
        try { $size = [int64]$asset.size } catch { $size = 0 }
        if ($size -le 0 -or [string]$asset.sha256 -notmatch '^[0-9a-f]{64}$') {
            throw "$Kind manifest has invalid size or SHA256 for $($entry.Key)"
        }
        if (-not ([string]$asset.sig).Trim()) {
            throw "$Kind manifest has no signature URL for $($entry.Key)"
        }
        if ($Kind -eq 'oss') {
            $encodedName = [uri]::EscapeDataString($entry.Value)
            $expectedURL = $OssPublicBaseURL.TrimEnd('/') + "/releases/$($Identity.Tag)/$Target/$encodedName"
            if ([string]$asset.url -cne $expectedURL -or [string]$asset.sig -cne "$expectedURL.minisig") {
                throw "OSS manifest immutable URL does not match $($entry.Value)"
            }
        }
    }
    return $manifest
}

function Get-WithdrawManifestAssetName {
    param([string]$URL, [string]$Description)
    try {
        $uri = [uri]$URL
        return [uri]::UnescapeDataString($uri.Segments[-1])
    }
    catch { throw "$Description is not a valid asset URL" }
}

function Assert-WithdrawManifestIdentityMatches {
    param($GitHubManifest, $OssManifest, $Identity)
    foreach ($platform in $script:DesktopTargetPlatforms[$Target]) {
        $githubAsset = $GitHubManifest.platforms.PSObject.Properties[$platform].Value
        $ossAsset = $OssManifest.platforms.PSObject.Properties[$platform].Value
        if ([int64]$githubAsset.size -ne [int64]$ossAsset.size -or
            [string]$githubAsset.sha256 -cne [string]$ossAsset.sha256) {
            throw "GitHub and OSS manifests disagree on size or SHA256 for $platform at $($Identity.Tag)"
        }
        $expectedName = $script:DesktopTargetAssets[$Target]
        foreach ($binding in @(
            @{ URL = [string]$githubAsset.url; Expected = $expectedName; Kind = 'GitHub asset' },
            @{ URL = [string]$githubAsset.sig; Expected = "$expectedName.minisig"; Kind = 'GitHub signature' },
            @{ URL = [string]$ossAsset.url; Expected = $expectedName; Kind = 'OSS asset' },
            @{ URL = [string]$ossAsset.sig; Expected = "$expectedName.minisig"; Kind = 'OSS signature' }
        )) {
            if ((Get-WithdrawManifestAssetName $binding.URL $binding.Kind) -cne $binding.Expected) {
                throw "$($binding.Kind) identity does not match $($binding.Expected) at $($Identity.Tag)"
            }
        }
    }
}

function Get-StablePointerState {
    param([string]$CurrentPath, [string]$BadPath, [string]$RecoveryPath)
    $currentHash = Get-LowerSHA256 $CurrentPath
    if ($currentHash -ceq (Get-LowerSHA256 $RecoveryPath)) { return 'recovery' }
    if ($currentHash -ceq (Get-LowerSHA256 $BadPath)) { return 'bad' }
    return 'unexpected'
}

function Get-WithdrawPaths {
    param($Identities)
    if (-not $WorkDirectory.Trim()) { throw 'WorkDirectory is required' }
    $root = [IO.Path]::GetFullPath($WorkDirectory)
    New-Item -ItemType Directory -Path $root -Force | Out-Null
    return [pscustomobject]@{
        BadGitHub = Join-Path $root (Get-ManifestFileName $Identities.Bad.Tag 'github')
        RecoveryGitHub = Join-Path $root (Get-ManifestFileName $Identities.Recovery.Tag 'github')
        BadOss = Join-Path $root (Get-ManifestFileName $Identities.Bad.Tag 'oss')
        RecoveryOss = Join-Path $root (Get-ManifestFileName $Identities.Recovery.Tag 'oss')
        Stable = Join-Path $root 'current-stable-latest.json'
    }
}

function Save-RemoteWithdrawState {
    param($Identities, $Paths)
    Save-GitHubManifest $Identities.Bad $Paths.BadGitHub
    Save-GitHubManifest $Identities.Recovery $Paths.RecoveryGitHub
    Save-PublicFile `
        ($OssPublicBaseURL.TrimEnd('/') + "/releases/$($Identities.Bad.Tag)/$Target/latest.json") `
        $Paths.BadOss "read immutable OSS manifest for $($Identities.Bad.Tag)"
    Save-PublicFile `
        ($OssPublicBaseURL.TrimEnd('/') + "/releases/$($Identities.Recovery.Tag)/$Target/latest.json") `
        $Paths.RecoveryOss "read immutable OSS manifest for $($Identities.Recovery.Tag)"
    Save-PublicFile `
        ($OssPublicBaseURL.TrimEnd('/') + "/channels/stable/$Target/latest.json") `
        $Paths.Stable 'read public OSS stable pointer'
}

function Assert-RemoteWithdrawState {
    param($Identities, $Paths, [switch]$RequireRecovered)
    $badRelease = Get-WithdrawRelease $Identities.Bad.Tag
    $recoveryRelease = Get-WithdrawRelease $Identities.Recovery.Tag
    Assert-PublishedWithdrawRelease $badRelease $Identities.Bad -AllowPrerelease
    Assert-PublishedWithdrawRelease $recoveryRelease $Identities.Recovery
    $badGitHub = Assert-WithdrawManifest $Paths.BadGitHub $Identities.Bad github
    $recoveryGitHub = Assert-WithdrawManifest $Paths.RecoveryGitHub $Identities.Recovery github
    $badOss = Assert-WithdrawManifest $Paths.BadOss $Identities.Bad oss
    $recoveryOss = Assert-WithdrawManifest $Paths.RecoveryOss $Identities.Recovery oss
    Assert-WithdrawManifestIdentityMatches $badGitHub $badOss $Identities.Bad
    Assert-WithdrawManifestIdentityMatches $recoveryGitHub $recoveryOss $Identities.Recovery

    $stableState = Get-StablePointerState $Paths.Stable $Paths.BadOss $Paths.RecoveryOss
    if ($stableState -eq 'unexpected') {
        throw 'OSS stable pointer matches neither the bad nor recovery immutable manifest'
    }
    if ($RequireRecovered) {
        if ($stableState -cne 'recovery') { throw 'OSS stable pointer has not been restored' }
    }
    return [pscustomobject]@{
        StableState = $stableState
    }
}

function Invoke-Preflight {
    $identities = Assert-WithdrawInputs
    Assert-WithdrawReadiness
    $paths = Get-WithdrawPaths $identities
    Save-RemoteWithdrawState $identities $paths
    $state = Assert-RemoteWithdrawState $identities $paths
    Write-Host "Withdraw preflight passed with zero remote writes: target=$Target OSS=$($state.StableState)"
}

function Invoke-OssPutStable {
    param([string]$ManifestPath)
    $uri = ([IO.Path]::GetFullPath($ManifestPath)).Replace('\', '/')
    $null = & $OssutilPath api put-object `
        --bucket $OssBucket `
        --key "channels/stable/$Target/latest.json" `
        --body "file://$uri" `
        --content-type 'application/json' `
        --cache-control 'no-cache' `
        --quiet 2>&1
    if ($LASTEXITCODE -ne 0) { throw 'Failed to restore OSS stable pointer' }
}

function Invoke-RestoreOss {
    $identities = Assert-WithdrawInputs
    $paths = Get-WithdrawPaths $identities
    Save-RemoteWithdrawState $identities $paths
    $state = Assert-RemoteWithdrawState $identities $paths
    if ($state.StableState -eq 'recovery') {
        Write-Host 'OSS stable pointer already matches recovery; verified and skipped'
        return
    }
    Invoke-OssPutStable $paths.RecoveryOss
    Save-PublicFile `
        ($OssPublicBaseURL.TrimEnd('/') + "/channels/stable/$Target/latest.json") `
        $paths.Stable 'verify restored OSS stable pointer'
    if ((Get-StablePointerState $paths.Stable $paths.BadOss $paths.RecoveryOss) -cne 'recovery') {
        throw 'OSS stable pointer verification failed after restore'
    }
    Write-Host "Restored and verified OSS stable pointer to $($identities.Recovery.Tag)"
}

function Invoke-RestoreGitHub {
    throw 'Target withdrawal never modifies shared GitHub Releases'
}

function Invoke-Verify {
    $identities = Assert-WithdrawInputs
    $paths = Get-WithdrawPaths $identities
    Save-RemoteWithdrawState $identities $paths
    $null = Assert-RemoteWithdrawState $identities $paths -RequireRecovered
    Write-Host "Withdraw is fully recovered for $Target`: $($identities.Bad.Tag) -> $($identities.Recovery.Tag)"
}

if ($MyInvocation.InvocationName -eq '.') { return }

switch ($Action) {
    'ValidateInputs' { $null = Assert-WithdrawInputs }
    'Preflight' { Invoke-Preflight }
    'RestoreOss' { Invoke-RestoreOss }
    'RestoreGitHub' { Invoke-RestoreGitHub }
    'Verify' { Invoke-Verify }
}

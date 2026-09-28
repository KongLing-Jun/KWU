[CmdletBinding()]
param(
    [ValidateSet('ValidateDraft', 'ValidateAssetReceipt', 'ValidateAssetReceipts', 'ValidateOssManifest', 'CloudPreflight', 'PublishGitHubOss', 'PublishCloudRecords', 'PublishPointers')]
    [string]$Action,
    [string]$Tag = '',
    [string]$Version = '',
    [string]$SourceCommit = '',
    [string]$NotesFile = '',
    [string]$ArtifactsDirectory = '',
    [ValidateSet('draft', 'published', 'draft-or-published')]
    [string]$ReleaseState = 'draft',
    [switch]$AllowFinalizedDraft,
    [string]$ReleaseRepository = 'xiaoxiaotest9527-bit/KWU-release',
    [string]$SourceRepository = 'xiaoxiaotest9527-bit/KWU',
    [string]$OssutilPath = 'ossutil',
    [string]$OssBucket = 'kwu-desktop-releases',
    [string]$OssPublicBaseURL = 'https://kwu-desktop-releases.oss-cn-guangzhou.aliyuncs.com',
    [ValidateSet('required')]
    [string]$CloudMode = 'required',
    [string]$CloudBaseURL = '',
    [string]$CloudPublisherPath = '',
    [string]$CloudManifestDirectory = '',
    [string]$CloudManifestPattern = 'latest-{target}.json',
    [ValidateRange(1, 3)]
    [int]$CloudConcurrency = 3,
    [string]$WorkDirectory = '',
    [string]$TagObject = '',
    [string]$OssManifestPath = '',
    [string]$ManifestTarget = '',
    [string]$Arm64AcceptanceSHA256 = '',
    [string]$Arm64AcceptanceConfirmation = '',
    [string]$AssetReceipt = '',
    [string[]]$AssetReceipts = @(),
    [string]$ReceiptTarget = '',
    [string[]]$ReleaseTargets = @('all'),
    [string]$Confirmation = '',
    [switch]$Bootstrap
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'desktop-actions-common.ps1')

$script:SelectedTargets = @(Resolve-DesktopReleaseTargets $ReleaseTargets)

function Assert-FrozenReleaseIdentity {
    param($Identity, [string]$Commit, [string]$ObjectSHA)
    $normalizedCommit = $Commit.Trim().ToLowerInvariant()
    $normalizedObject = $ObjectSHA.Trim().ToLowerInvariant()
    if ($normalizedCommit -notmatch '^[0-9a-f]{40}$') {
        throw 'SourceCommit must be an exact 40-character SHA'
    }
    if ($normalizedObject -notmatch '^[0-9a-f]{40}$') {
        throw 'TagObject must be an exact 40-character annotated tag object SHA'
    }
    Assert-RemoteAnnotatedTagIdentity $Identity.Tag $normalizedObject $normalizedCommit | Out-Null
}

function Assert-PublishIntent {
    $identity = Get-ReleaseIdentity $Version $Tag
    if ($Bootstrap -and ($script:SelectedTargets -join ',') -cne ($script:DesktopReleaseTargets -join ',')) {
        throw 'Bootstrap publication requires all three desktop targets'
    }
    $expected = Get-DesktopPublishConfirmation $identity.Tag $script:SelectedTargets -Bootstrap:$Bootstrap
    if ($Confirmation.Trim() -cne $expected) {
        throw "Publication confirmation must exactly match: $expected"
    }
}

function Read-PublicManifest {
    param([string]$URL, [string]$Description)
    try {
        return Invoke-RestMethod -Uri ($URL + "?kwu-bootstrap-check=$([guid]::NewGuid().ToString('N'))") `
            -TimeoutSec 30 -Headers @{ 'Cache-Control' = 'no-cache' }
    }
    catch { throw "$Description is unavailable: $($_.Exception.Message)" }
}

function Read-PublicManifestIfExists {
    param([string]$URL, [string]$Description)
    try {
        return Invoke-RestMethod -Uri ($URL + "?kwu-bootstrap-check=$([guid]::NewGuid().ToString('N'))") `
            -TimeoutSec 30 -Headers @{ 'Cache-Control' = 'no-cache' }
    }
    catch {
        $statusCode = 0
        if ($null -ne $_.Exception.Response) {
            try { $statusCode = [int]$_.Exception.Response.StatusCode } catch { $statusCode = 0 }
        }
        if ($statusCode -eq 404) { return $null }
        throw "$Description is unavailable: $($_.Exception.Message)"
    }
}

function Assert-BootstrapStateTransition {
    param(
        [string]$CandidateTag,
        [string]$GlobalTag,
        [bool]$GlobalReleaseHasMigrationAssets,
        [string]$GitHubLatestTag,
        [string[]]$TargetTags
    )
    $establishedTargets = @($TargetTags | Where-Object { $_ })
    if ($GlobalReleaseHasMigrationAssets) {
        if ($CandidateTag -cne $GlobalTag) {
            throw "Desktop migration is already frozen at $GlobalTag; refusing a new bootstrap $CandidateTag"
        }
        foreach ($tag in $establishedTargets) {
            if ($tag -cne $GlobalTag) {
                throw "Bootstrap resume is no longer exact because target state advanced to $tag"
            }
        }
        return
    }
    if ($GitHubLatestTag -cne $GlobalTag -and $GitHubLatestTag -cne $CandidateTag) {
        throw "A different bootstrap candidate already owns GitHub Latest: $GitHubLatestTag"
    }
    foreach ($tag in $establishedTargets) {
        if ($tag -cne $GlobalTag -and $tag -cne $CandidateTag) {
            throw "Bootstrap target state belongs to another release: $tag"
        }
    }
}

function Assert-BootstrapMayStartOrResume {
    param($Identity)
    $base = $OssPublicBaseURL.TrimEnd('/')
    $global = Read-PublicManifest "$base/channels/stable/latest.json" 'Global bootstrap manifest'
    $globalTag = (Get-ReleaseIdentity ([string]$global.version) '').Tag
    $globalReleaseJSON = Invoke-Native "read frozen global GitHub Release $globalTag" {
        gh release view $globalTag --repo $ReleaseRepository --json assets
    }
    $globalRelease = ($globalReleaseJSON | Out-String) | ConvertFrom-Json
    $globalAssetNames = @($globalRelease.assets | ForEach-Object { [string]$_.name })
    $migrationAssets = @('latest.json') + @(
        $script:DesktopReleaseTargets | ForEach-Object { Get-SelectedManifestName $_ }
    )
    $globalReleaseHasMigrationAssets = @(
        $migrationAssets | Where-Object { $globalAssetNames -notcontains $_ }
    ).Count -eq 0
    $latestJSON = Invoke-Native 'read GitHub Latest bootstrap state' {
        gh release view --repo $ReleaseRepository --json tagName
    }
    $latestTag = [string](($latestJSON | Out-String) | ConvertFrom-Json).tagName
    $targetTags = @()
    foreach ($target in $script:DesktopReleaseTargets) {
        $pointer = Read-PublicManifestIfExists "$base/channels/stable/$target/latest.json" "Bootstrap target pointer $target"
        if ($null -ne $pointer) {
            $targetTags += (Get-ReleaseIdentity ([string]$pointer.version) '').Tag
        }
    }
    Assert-BootstrapStateTransition `
        $Identity.Tag $globalTag $globalReleaseHasMigrationAssets $latestTag $targetTags
    Write-Host "Bootstrap state permits first migration or exact resume for $($Identity.Tag)"
}

function Assert-SelectiveMigrationReady {
    if ($Bootstrap) {
        Assert-BootstrapMayStartOrResume (Get-ReleaseIdentity $Version $Tag)
        return
    }
    $global = Read-PublicManifest ($OssPublicBaseURL.TrimEnd('/') + '/channels/stable/latest.json') 'Frozen global migration manifest'
    $platformNames = @($global.platforms.PSObject.Properties.Name)
    foreach ($platform in @('windows-amd64', 'windows-arm64', 'darwin-amd64', 'darwin-arm64')) {
        if ($platformNames -notcontains $platform) { throw "Frozen migration manifest is missing $platform" }
    }
    $migration = Get-ReleaseIdentity ([string]$global.version) ''
    $latest = Get-ReleaseJSON $migration.Tag
    if ($latest.isDraft) { throw "Frozen GitHub migration Release is still a Draft: $($migration.Tag)" }
    $globalLatest = (& gh release view --repo $ReleaseRepository --json tagName | ConvertFrom-Json).tagName
    if ([string]$globalLatest -cne $migration.Tag) {
        throw "GitHub Latest no longer matches frozen migration tag $($migration.Tag)"
    }
    $temporary = Join-Path ([IO.Path]::GetTempPath()) ("kwu-bootstrap-" + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $temporary | Out-Null
    try {
        Invoke-Native 'download frozen GitHub migration manifest' {
            gh release download $migration.Tag --repo $ReleaseRepository --pattern latest.json --dir $temporary
        } | Out-Null
        $githubGlobal = Get-Content -Raw -LiteralPath (Join-Path $temporary 'latest.json') | ConvertFrom-Json
        if ([string]$githubGlobal.version -cne $migration.Version) {
            throw 'Frozen GitHub migration manifest version differs from the global OSS entry'
        }
        foreach ($platform in @('windows-amd64', 'windows-arm64', 'darwin-amd64', 'darwin-arm64')) {
            if ($null -eq $githubGlobal.platforms.PSObject.Properties[$platform]) {
                throw "Frozen GitHub migration manifest is missing $platform"
            }
        }
    }
    finally { Remove-Item -LiteralPath $temporary -Recurse -Force -ErrorAction SilentlyContinue }
    foreach ($target in $script:DesktopReleaseTargets) {
        $pointer = Read-PublicManifest ($OssPublicBaseURL.TrimEnd('/') + "/channels/stable/$target/latest.json") "Migration target pointer $target"
        if ((Get-DesktopVersionCode ([string]$pointer.version)) -lt (Get-DesktopVersionCode $migration.Version)) {
            throw "Migration target pointer $target is older than $($migration.Version)"
        }
    }
    Write-Host "Verified frozen migration entry at $($migration.Tag)"
}

function Get-SelectedManifestName {
    param([string]$Target)
    return "latest-$Target.json"
}

function Assert-ResumableReleaseAssets {
    param($Release, [switch]$RequireFinal)
    $names = @($Release.assets | ForEach-Object { [string]$_.name })
    if (@($names | Select-Object -Unique).Count -ne $names.Count) {
        throw 'Release contains duplicate asset names'
    }
    $allowed = @($script:PrimaryAssets | ForEach-Object { $_; "$_.minisig" }) +
        @($script:DesktopReleaseTargets | ForEach-Object { Get-SelectedManifestName $_ }) + @('latest.json')
    foreach ($name in $names) {
        if ($allowed -notcontains $name) { throw "Release contains an unexpected asset: $name" }
    }
    foreach ($name in @(Get-DesktopTargetAssetNames $script:SelectedTargets)) {
        if ($names -notcontains $name) { throw "Release is missing primary asset: $name" }
    }
    if ($RequireFinal) {
        foreach ($target in $script:SelectedTargets) {
            foreach ($name in @($script:DesktopTargetAssets[$target], "$($script:DesktopTargetAssets[$target]).minisig", (Get-SelectedManifestName $target))) {
                if ($names -notcontains $name) { throw "Release is missing selected final asset: $name" }
            }
        }
    }
    return $names
}

function New-AssetReceipt {
    param([string]$Target, $Asset)
    return ([ordered]@{
        target = $Target
        asset_name = [string]$Asset.Name
        size = [int64]$Asset.Size
        sha256 = [string]$Asset.SHA256
    } | ConvertTo-Json -Compress)
}

function Assert-AssetReceipt {
    param([string]$Receipt, [string]$Target, [string]$Directory, [switch]$AllowOtherSelectedAssets)
    if ($script:DesktopReleaseTargets -notcontains $Target) { throw "Unknown receipt target: $Target" }
    try { $binding = $Receipt | ConvertFrom-Json }
    catch { throw "Asset receipt is not valid JSON for $Target" }
    $properties = @($binding.PSObject.Properties.Name)
    if (($properties -join ',') -cne 'target,asset_name,size,sha256') {
        throw "Asset receipt fields are invalid for $Target"
    }
    $expectedName = $script:DesktopTargetAssets[$Target]
    if ([string]$binding.target -cne $Target -or [string]$binding.asset_name -cne $expectedName) {
        throw "Asset receipt target/name binding is invalid for $Target"
    }
    if ([int64]$binding.size -le 0 -or [string]$binding.sha256 -notmatch '^[0-9a-f]{64}$') {
        throw "Asset receipt size/SHA256 binding is invalid for $Target"
    }
    if (-not $AllowOtherSelectedAssets) {
        $metadata = @(Get-AssetMetadata $Directory @($Target))
    }
    else {
        $allMetadata = @(Get-AssetMetadata $Directory $script:SelectedTargets)
        $metadata = @($allMetadata | Where-Object Name -eq $expectedName)
    }
    if ($metadata.Count -ne 1 -or [int64]$metadata[0].Size -ne [int64]$binding.size -or
        [string]$metadata[0].SHA256 -cne [string]$binding.sha256) {
        throw "Downloaded Draft asset differs from approved receipt: $expectedName"
    }
    return $metadata[0]
}

function Assert-SelectedAssetReceipts {
    param([string[]]$Receipts, [string]$Directory)
    if ($Receipts.Count -ne $script:SelectedTargets.Count) {
        throw 'Asset receipt count does not match selected targets'
    }
    $seen = @{}
    foreach ($receipt in $Receipts) {
        try { $target = [string](($receipt | ConvertFrom-Json).target) }
        catch { throw 'Selected asset receipt is not valid JSON' }
        if ($script:SelectedTargets -notcontains $target -or $seen.ContainsKey($target)) {
            throw "Asset receipt target is unexpected or duplicated: $target"
        }
        $seen[$target] = Assert-AssetReceipt $receipt $target $Directory -AllowOtherSelectedAssets
    }
    foreach ($target in $script:SelectedTargets) {
        if (-not $seen.ContainsKey($target)) { throw "Asset receipt is missing selected target: $target" }
    }
    return @($script:SelectedTargets | ForEach-Object { $seen[$_] })
}

function Assert-Arm64Acceptance {
    param($Identity, [object[]]$Assets)
    $expectedConfirmation = "ACCEPT $($Identity.Tag) ARM64"
    if ($Arm64AcceptanceConfirmation.Trim() -cne $expectedConfirmation) {
        throw "ARM64 acceptance confirmation must exactly match: $expectedConfirmation"
    }
    $acceptedHash = $Arm64AcceptanceSHA256.Trim().ToLowerInvariant()
    if ($acceptedHash -notmatch '^[0-9a-f]{64}$') {
        throw 'Arm64AcceptanceSHA256 must be an exact 64-character SHA256'
    }
    $arm64 = @($Assets | Where-Object Name -eq 'Clouvia-windows-arm64-installer.exe')
    if ($arm64.Count -ne 1) { throw 'ARM64 acceptance requires one immutable arm64 installer' }
    if ([string]$arm64[0].SHA256 -cne $acceptedHash) {
        throw 'Win11 ARM acceptance SHA256 does not match the Draft arm64 installer'
    }
    Write-Host "Verified Win11 ARM acceptance for $($Identity.Tag) at $acceptedHash"
}

function Invoke-ValidateDraft {
    Assert-PublishIntent
    $identity = Get-ReleaseIdentity $Version $Tag
    $commit = $SourceCommit.Trim().ToLowerInvariant()
    Assert-FrozenReleaseIdentity $identity $commit $TagObject
    $notes = Get-ReleaseNotes $NotesFile
    $release = Get-ReleaseJSON $identity.Tag
    if ($ReleaseState -eq 'draft' -and -not $release.isDraft) { throw "Release is not a Draft: $($identity.Tag)" }
    if ($ReleaseState -eq 'published' -and $release.isDraft) { throw "Release is still a Draft: $($identity.Tag)" }
    if ($ReleaseState -eq 'published') {
        Assert-ResumableReleaseAssets $release -RequireFinal | Out-Null
    }
    elseif ($AllowFinalizedDraft) {
        Assert-ResumableReleaseAssets $release | Out-Null
    }
    elseif (-not $release.isDraft) {
        throw "Published Release validation requires -AllowFinalizedDraft or -ReleaseState published: $($identity.Tag)"
    }
    else {
        foreach ($name in @(Get-DesktopTargetAssetNames $script:SelectedTargets)) {
            if (@($release.assets | Where-Object { $_.name -ceq $name }).Count -ne 1) {
                throw "Release is missing selected primary asset: $name"
            }
        }
    }

    if (-not $ArtifactsDirectory) { throw 'ValidateDraft requires -ArtifactsDirectory' }
    New-Item -ItemType Directory -Path $ArtifactsDirectory -Force | Out-Null
    $unexpected = @(Get-ChildItem -LiteralPath $ArtifactsDirectory -File -Recurse -ErrorAction SilentlyContinue)
    if ($unexpected.Count -ne 0) { throw 'ValidateDraft artifact destination must be empty' }
    foreach ($name in @(Get-DesktopTargetAssetNames $script:SelectedTargets)) {
        Invoke-Native "download release asset $name" {
            gh release download $identity.Tag --repo $ReleaseRepository `
                --pattern $name --dir $ArtifactsDirectory
        } | Out-Null
    }
    $assets = @(Get-AssetMetadata $ArtifactsDirectory $script:SelectedTargets)
    if ($ReleaseState -ne 'published' -and $script:SelectedTargets -contains 'windows-arm64') { Assert-Arm64Acceptance $identity $assets }
    Assert-ReleaseBodyMetadata $release $identity $commit $notes $assets $TagObject
    foreach ($target in $script:SelectedTargets) {
        $assetName = $script:DesktopTargetAssets[$target]
        $asset = @($assets | Where-Object Name -eq $assetName)
        if ($asset.Count -ne 1) { throw "Validated asset is missing for receipt target: $target" }
        Write-ActionOutput "receipt_$($target.Replace('-', '_'))" (New-AssetReceipt $target $asset[0])
    }
    Write-Host "Validated $ReleaseState release assets and provenance for $($identity.Tag)"
}

function Invoke-ValidateAssetReceipt {
    if (-not $AssetReceipt) { throw 'ValidateAssetReceipt requires -AssetReceipt' }
    if (-not $ReceiptTarget) { throw 'ValidateAssetReceipt requires -ReceiptTarget' }
    if (-not $ArtifactsDirectory) { throw 'ValidateAssetReceipt requires -ArtifactsDirectory' }
    Assert-AssetReceipt $AssetReceipt $ReceiptTarget $ArtifactsDirectory | Out-Null
    Write-Host "Validated approved Draft receipt for $ReceiptTarget"
}

function Invoke-ValidateAssetReceipts {
    if ($AssetReceipts.Count -eq 0) { throw 'ValidateAssetReceipts requires -AssetReceipts' }
    if (-not $ArtifactsDirectory) { throw 'ValidateAssetReceipts requires -ArtifactsDirectory' }
    Assert-SelectedAssetReceipts $AssetReceipts $ArtifactsDirectory | Out-Null
    Write-Host "Validated approved Draft receipts for $($script:SelectedTargets -join ',')"
}

function Get-OssContentType {
    param([string]$Name)
    if ($Name.EndsWith('.json')) { return 'application/json' }
    if ($Name.EndsWith('.minisig')) { return 'text/plain; charset=utf-8' }
    return 'application/octet-stream'
}

function Assert-OssManifestMatchesReleaseAssets {
    param($Identity, [string]$ManifestPath, [string]$Directory, [string]$Target)
    if (-not (Test-Path -LiteralPath $ManifestPath -PathType Leaf)) {
        throw "OSS manifest does not exist: $ManifestPath"
    }
    try { $manifest = Get-Content -Raw -LiteralPath $ManifestPath | ConvertFrom-Json }
    catch { throw "OSS manifest is not valid JSON: $($_.Exception.Message)" }
    if ([string]$manifest.version -cne $Identity.Version) {
        throw "OSS manifest version does not match $($Identity.Version)"
    }
    if ($null -eq $manifest.platforms) { throw 'OSS manifest platforms are missing' }
    $platforms = @($manifest.platforms.PSObject.Properties)
    $expected = [ordered]@{}
    if (-not $Target) {
        $expected = [ordered]@{
            'windows-amd64' = 'Clouvia-windows-amd64-installer.exe'
            'windows-arm64' = 'Clouvia-windows-arm64-installer.exe'
            'darwin-amd64' = 'Clouvia-darwin-universal.dmg'
            'darwin-arm64' = 'Clouvia-darwin-universal.dmg'
        }
    }
    else {
        foreach ($platform in $script:DesktopTargetPlatforms[$Target]) {
            $expected[$platform] = $script:DesktopTargetAssets[$Target]
        }
    }
    if ($platforms.Count -ne $expected.Count) {
        throw "OSS manifest must contain exactly $($expected.Count) stable platforms; found $($platforms.Count)"
    }
    foreach ($entry in $expected.GetEnumerator()) {
        $property = $manifest.platforms.PSObject.Properties[$entry.Key]
        if ($null -eq $property) { throw "OSS manifest is missing $($entry.Key)" }
        $asset = $property.Value
        $localPath = Join-Path $Directory $entry.Value
        if (-not (Test-Path -LiteralPath $localPath -PathType Leaf)) {
            throw "Downloaded GitHub primary asset is missing: $($entry.Value)"
        }
        $local = Get-Item -LiteralPath $localPath
        if ($local.Length -le 0) { throw "Downloaded GitHub primary asset is empty: $($entry.Value)" }
        $encodedName = [uri]::EscapeDataString($entry.Value)
        $targetSegment = if ($Target) { "/$Target" } else { '' }
        $expectedURL = $OssPublicBaseURL.TrimEnd('/') + "/releases/$($Identity.Tag)$targetSegment/$encodedName"
        if ([string]$asset.url -cne $expectedURL) {
            throw "OSS manifest URL does not exactly match $($entry.Value)"
        }
        if ([string]$asset.sig -cne "$expectedURL.minisig") {
            throw "OSS manifest signature URL does not exactly match $($entry.Value)"
        }
        try { $manifestSize = [int64]$asset.size }
        catch { throw "OSS manifest size is invalid for $($entry.Value)" }
        if ($manifestSize -ne $local.Length) {
            throw "OSS manifest size does not match downloaded GitHub asset: $($entry.Value)"
        }
        if ([string]$asset.sha256 -cne (Get-LowerSHA256 $localPath)) {
            throw "OSS manifest SHA256 does not match downloaded GitHub asset: $($entry.Value)"
        }
    }
    Write-Host 'OSS manifest is bound to all downloaded GitHub release assets'
}

function Invoke-ValidateOssManifest {
    Assert-PublishIntent
    $identity = Get-ReleaseIdentity $Version $Tag
    $commit = $SourceCommit.Trim().ToLowerInvariant()
    Assert-FrozenReleaseIdentity $identity $commit $TagObject
    $target = $ManifestTarget.Trim()
    if (-not $target -or $script:SelectedTargets -notcontains $target) {
        throw 'ValidateOssManifest ManifestTarget must be one of the confirmed release targets'
    }
    $notes = Get-ReleaseNotes $NotesFile
    $published = Get-ReleaseJSON $identity.Tag
    if ($published.isDraft) { throw "Release is still a Draft: $($identity.Tag)" }
    Assert-ResumableReleaseAssets $published -RequireFinal | Out-Null
    Assert-ReleaseBodyMetadata $published $identity $commit $notes @() $TagObject
    Assert-OssManifestMatchesReleaseAssets $identity $OssManifestPath $ArtifactsDirectory $target
}

function Test-PublicObjectMatches {
    param([string]$ObjectKey, [string]$FilePath)
    $temporary = [IO.Path]::GetTempFileName()
    try {
        Invoke-WebRequest -Uri ($OssPublicBaseURL.TrimEnd('/') + '/' + $ObjectKey) `
            -OutFile $temporary -TimeoutSec 30
        return (Get-LowerSHA256 $temporary) -eq (Get-LowerSHA256 $FilePath)
    }
    catch { return $false }
    finally { Remove-Item -LiteralPath $temporary -Force -ErrorAction SilentlyContinue }
}

function Get-PublicObjectState {
    param([string]$ObjectKey, [string]$FilePath)
    $temporary = [IO.Path]::GetTempFileName()
    try {
        try {
            Invoke-WebRequest -Uri ($OssPublicBaseURL.TrimEnd('/') + '/' + $ObjectKey) `
                -OutFile $temporary -TimeoutSec 30 -Headers @{ 'Cache-Control' = 'no-cache' }
        }
        catch {
            $statusCode = 0
            if ($null -ne $_.Exception.Response) {
                try { $statusCode = [int]$_.Exception.Response.StatusCode } catch { $statusCode = 0 }
            }
            if ($statusCode -eq 404) { return 'missing' }
            throw "Failed to preflight OSS object: $ObjectKey"
        }
        if ((Get-LowerSHA256 $temporary) -eq (Get-LowerSHA256 $FilePath)) { return 'match' }
        return 'conflict'
    }
    finally { Remove-Item -LiteralPath $temporary -Force -ErrorAction SilentlyContinue }
}

function Assert-GitHubAssetCanAppend {
    param($Identity, $Release, [string]$Name, [string]$Path)
    $existing = @($Release.assets | Where-Object { [string]$_.name -ceq $Name })
    if ($existing.Count -gt 1) { throw "Release contains duplicate asset names: $Name" }
    if ($existing.Count -eq 0) { return }
    $temporary = Join-Path ([IO.Path]::GetTempPath()) ("kwu-preflight-gh-" + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $temporary | Out-Null
    try {
        Invoke-Native "preflight existing GitHub asset $Name" {
            gh release download $Identity.Tag --repo $ReleaseRepository --pattern $Name --dir $temporary
        } | Out-Null
        if ((Get-LowerSHA256 (Join-Path $temporary $Name)) -ne (Get-LowerSHA256 $Path)) {
            throw "Existing GitHub asset differs from selected content: $Name"
        }
    }
    finally { Remove-Item -LiteralPath $temporary -Recurse -Force -ErrorAction SilentlyContinue }
}

function Assert-ZeroWritePublicationPreflight {
    param($Identity, $Release, [object[]]$Assets, [hashtable]$GitHubManifests, [hashtable]$OssManifests,
        [string]$BootstrapManifest = '', [string]$BootstrapOssManifest = '', [string]$NotesPath = '')
    foreach ($asset in $Assets) {
        Assert-GitHubAssetCanAppend $Identity $Release $asset.Name $asset.Path
        Assert-GitHubAssetCanAppend $Identity $Release "$($asset.Name).minisig" ($asset.Path + '.minisig')
    }
    foreach ($target in $script:SelectedTargets) {
        Assert-GitHubAssetCanAppend $Identity $Release (Get-SelectedManifestName $target) $GitHubManifests[$target]
    }
    if ($BootstrapManifest) { Assert-GitHubAssetCanAppend $Identity $Release 'latest.json' $BootstrapManifest }

    $prefix = "releases/$($Identity.Tag)"
    $objects = @{}
    foreach ($asset in $Assets) {
        $target = @($script:SelectedTargets | Where-Object { $script:DesktopTargetAssets[$_] -eq $asset.Name })[0]
        $objects["$prefix/$target/$($asset.Name)"] = $asset.Path
        $objects["$prefix/$target/$($asset.Name).minisig"] = $asset.Path + '.minisig'
        if ($Bootstrap) {
            $objects["$prefix/$($asset.Name)"] = $asset.Path
            $objects["$prefix/$($asset.Name).minisig"] = $asset.Path + '.minisig'
        }
    }
    foreach ($target in $script:SelectedTargets) {
        $objects["$prefix/$target/latest.json"] = $OssManifests[$target]
    }
    if ($BootstrapOssManifest) { $objects["$prefix/latest.json"] = $BootstrapOssManifest }
    Assert-PointerPublicationPreflight $Identity $OssManifests $BootstrapOssManifest
    foreach ($entry in $objects.GetEnumerator()) {
        if ((Get-PublicObjectState $entry.Key $entry.Value) -eq 'conflict') {
            throw "OSS immutable object exists with different content: $($entry.Key)"
        }
    }
    Invoke-CloudPreflight
    if (-not $CloudPublisherPath -or -not (Test-Path -LiteralPath $CloudPublisherPath -PathType Leaf)) {
        throw "Cloud publisher executable does not exist: $CloudPublisherPath"
    }
    $cloudRequests = @($script:SelectedTargets | ForEach-Object {
        [pscustomobject]@{
            Publisher = [IO.Path]::GetFullPath($CloudPublisherPath)
            Manifest = [IO.Path]::GetFullPath($OssManifests[$_])
            Notes = [IO.Path]::GetFullPath($NotesPath)
            BaseURL = $CloudBaseURL
            Target = $_
        }
    })
    Invoke-CloudPublisherPhase $cloudRequests -Preflight
    Write-Host "Zero-write publication preflight passed for targets $($script:SelectedTargets -join ',')"
}

function Invoke-OssPutObject {
    param([string]$ObjectKey, [string]$FilePath, [bool]$ForbidOverwrite, [string]$CacheControl)
    $uri = ([IO.Path]::GetFullPath($FilePath)).Replace('\', '/')
    $arguments = @(
        'api', 'put-object', '--bucket', $OssBucket, '--key', $ObjectKey,
        '--body', "file://$uri", '--content-type', (Get-OssContentType $ObjectKey),
        '--cache-control', $CacheControl, '--quiet'
    )
    if ($ForbidOverwrite) { $arguments += '--forbid-overwrite' }
    $null = & $OssutilPath @arguments 2>&1
    return $LASTEXITCODE
}

function Publish-OssImmutableObject {
    param([string]$ObjectKey, [string]$FilePath)
    $exitCode = Invoke-OssPutObject $ObjectKey $FilePath $true 'public,max-age=31536000,immutable'
    if ($exitCode -eq 0) {
        Write-Host "Uploaded immutable OSS object: $ObjectKey"
        return
    }
    if (Test-PublicObjectMatches $ObjectKey $FilePath) {
        Write-Host "Reused matching immutable OSS object: $ObjectKey"
        return
    }
    throw "OSS immutable object exists with different content or upload failed: $ObjectKey"
}

function Publish-OssStablePointer {
    param([string]$ManifestPath, [string]$Target = '')
    $key = if ($Target) { "channels/stable/$Target/latest.json" } else { 'channels/stable/latest.json' }
    if ((Invoke-OssPutObject $key $ManifestPath $false 'no-cache') -ne 0) {
        throw "Failed to update OSS stable pointer: $key"
    }
    if (-not (Test-PublicObjectMatches $key $ManifestPath)) {
        throw "OSS stable pointer verification failed: $key"
    }
    Write-Host 'Updated and verified OSS stable pointer'
}

function Get-DesktopVersionCode {
    param([string]$RawVersion)
    $identity = Get-ReleaseIdentity $RawVersion ''
    $segments = @($identity.VersionName.Split('.') | ForEach-Object { [int64]$_ })
    return $segments[0] * 1000000L + $segments[1] * 1000L + $segments[2]
}

function Assert-StablePointerTransition {
    param($TargetIdentity, [string]$CandidateManifestPath, [string]$CurrentManifestPath)
    if (-not (Test-Path -LiteralPath $CandidateManifestPath -PathType Leaf)) {
        throw "Candidate OSS stable manifest does not exist: $CandidateManifestPath"
    }
    if (-not (Test-Path -LiteralPath $CurrentManifestPath -PathType Leaf)) {
        throw "Current OSS stable manifest does not exist: $CurrentManifestPath"
    }
    try { $current = Get-Content -Raw -LiteralPath $CurrentManifestPath | ConvertFrom-Json }
    catch { throw "Current OSS stable manifest is not valid JSON: $($_.Exception.Message)" }
    $currentVersion = [string]$current.version
    if (-not $currentVersion.Trim()) { throw 'Current OSS stable manifest has no version' }
    $currentCode = Get-DesktopVersionCode $currentVersion
    $targetCode = Get-DesktopVersionCode $TargetIdentity.Version
    if ($currentCode -gt $targetCode) {
        throw "Refusing to move OSS stable backward from $currentVersion to $($TargetIdentity.Version)"
    }
    if ($currentCode -eq $targetCode) {
        $currentSHA = Get-LowerSHA256 $CurrentManifestPath
        $candidateSHA = Get-LowerSHA256 $CandidateManifestPath
        if ($currentSHA -ne $candidateSHA) {
            throw "OSS stable already has $currentVersion with a different manifest SHA256"
        }
        Write-Host "OSS stable already contains the identical $currentVersion manifest; safe to resume"
        return
    }
    Write-Host "OSS stable may advance from $currentVersion to $($TargetIdentity.Version)"
}

function Assert-PublicStablePointerCanAdvance {
    param($TargetIdentity, [string]$CandidateManifestPath, [string]$Target = '')
    $temporary = [IO.Path]::GetTempFileName()
    try {
        $cacheBuster = [guid]::NewGuid().ToString('N')
        $pointerPath = if ($Target) { "channels/stable/$Target/latest.json" } else { 'channels/stable/latest.json' }
        $uri = $OssPublicBaseURL.TrimEnd('/') + "/$pointerPath`?kwu-release-check=$cacheBuster"
        try {
            Invoke-WebRequest -Uri $uri -OutFile $temporary -TimeoutSec 30 `
                -Headers @{ 'Cache-Control' = 'no-cache' }
        }
        catch {
            $statusCode = 0
            if ($null -ne $_.Exception.Response) {
                try { $statusCode = [int]$_.Exception.Response.StatusCode } catch { $statusCode = 0 }
            }
            if ($statusCode -eq 404) {
                Write-Host 'OSS stable pointer does not exist yet; initial publication may continue'
                return
            }
            throw 'Failed to read the current public OSS stable pointer before publication'
        }
        Assert-StablePointerTransition $TargetIdentity $CandidateManifestPath $temporary
    }
    finally { Remove-Item -LiteralPath $temporary -Force -ErrorAction SilentlyContinue }
}

function Assert-PointerPublicationPreflight {
    param($Identity, [hashtable]$OssManifests, [string]$BootstrapOssManifest = '')
    foreach ($target in $script:SelectedTargets) {
        $manifest = [string]$OssManifests[$target]
        if (-not $manifest -or -not (Test-Path -LiteralPath $manifest -PathType Leaf)) {
            throw "Selected target OSS manifest is missing: $target"
        }
        Assert-PublicStablePointerCanAdvance $Identity $manifest $target
    }
    if ($Bootstrap) {
        if (-not $BootstrapOssManifest -or
            -not (Test-Path -LiteralPath $BootstrapOssManifest -PathType Leaf)) {
            throw 'Bootstrap global OSS manifest is missing'
        }
        Assert-PublicStablePointerCanAdvance $Identity $BootstrapOssManifest
    }
}

$script:CloudPhaseExecutor = {
    param([object[]]$Requests, [bool]$Preflight, [int]$ThrottleLimit)
    return @($Requests | ForEach-Object -Parallel {
        $request = $_
        $arguments = @()
        if ($using:Preflight) { $arguments += '-preflight' }
        $arguments += @(
            '-manifest', [string]$request.Manifest,
            '-notes', [string]$request.Notes,
            '-base-url', [string]$request.BaseURL,
            '-target', [string]$request.Target
        )
        $output = (& $request.Publisher @arguments 2>&1 | Out-String).Trim()
        [pscustomobject]@{
            Target = [string]$request.Target
            ExitCode = [int]$LASTEXITCODE
            Output = $output
        }
    } -ThrottleLimit $ThrottleLimit)
}

function Invoke-CloudPublisherPhase {
    param([object[]]$Requests, [switch]$Preflight)
    $results = @(& $script:CloudPhaseExecutor $Requests ([bool]$Preflight) $CloudConcurrency)
    $phase = if ($Preflight) { 'preflight' } else { 'publication' }
    if ($results.Count -ne $Requests.Count) {
        throw "Cloud $phase did not return one result for every selected target"
    }
    $expectedTargets = @($Requests | ForEach-Object { [string]$_.Target })
    $actualTargets = @($results | ForEach-Object { [string]$_.Target })
    if (@($actualTargets | Select-Object -Unique).Count -ne $actualTargets.Count -or
        @($expectedTargets | Where-Object { $actualTargets -notcontains $_ }).Count -ne 0) {
        throw "Cloud $phase returned missing, duplicate, or unexpected target results"
    }
    foreach ($result in $results) {
        if ([string]$result.Output) { Write-Host ([string]$result.Output) }
    }
    $failed = @($results | Where-Object { [int]$_.ExitCode -ne 0 })
    if ($failed.Count -ne 0) {
        throw "Cloud $phase failed for targets: $(@($failed | ForEach-Object Target) -join ',')"
    }
}

function Invoke-PublishCloudRecords {
    Assert-PublishIntent
    Invoke-CloudPreflight
    if (-not $CloudPublisherPath -or -not (Test-Path -LiteralPath $CloudPublisherPath -PathType Leaf)) {
        throw "Cloud publisher executable does not exist: $CloudPublisherPath"
    }
    if (-not $CloudManifestDirectory -or -not (Test-Path -LiteralPath $CloudManifestDirectory -PathType Container)) {
        throw "Cloud manifest directory does not exist: $CloudManifestDirectory"
    }
    if (-not (Test-Path -LiteralPath $NotesFile -PathType Leaf)) {
        throw "Cloud release notes do not exist: $NotesFile"
    }
    $requests = @()
    foreach ($target in $script:SelectedTargets) {
        $manifestName = $CloudManifestPattern.Replace('{target}', $target)
        $manifest = Join-Path $CloudManifestDirectory $manifestName
        if (-not (Test-Path -LiteralPath $manifest -PathType Leaf)) {
            throw "Cloud manifest does not exist for $target`: $manifest"
        }
        $requests += [pscustomobject]@{
            Publisher = [IO.Path]::GetFullPath($CloudPublisherPath)
            Manifest = [IO.Path]::GetFullPath($manifest)
            Notes = [IO.Path]::GetFullPath($NotesFile)
            BaseURL = $CloudBaseURL
            Target = $target
        }
    }
    Invoke-CloudPublisherPhase $requests -Preflight
    Invoke-CloudPublisherPhase $requests
    Write-Host "Published cloud release rows for $($script:SelectedTargets -join ',')"
}

function Assert-CloudModeCanBeRecorded {
    param($Release, $Identity)
    $body = ([string]$Release.body).Replace("`r`n", "`n").Trim()
    $marker = '<!-- kwu-desktop-cloud-publication -->'
    $detail = '- Mode: required'
    $section = "$marker`n## Cloud publication`n`n$detail"
    if ($body.Contains($marker)) {
        if (-not $body.Contains($section)) {
            throw 'Draft already contains a different cloud publication decision'
        }
        return $true
    }
    if (-not $Release.isDraft) {
        throw 'Published Release is missing its frozen cloud publication decision'
    }
    return $false
}

function Set-CloudModeInReleaseBody {
    param($Release, $Identity)
    if (Assert-CloudModeCanBeRecorded $Release $Identity) { return }
    $body = ([string]$Release.body).Replace("`r`n", "`n").Trim()
    $marker = '<!-- kwu-desktop-cloud-publication -->'
    $section = "$marker`n## Cloud publication`n`n- Mode: required"
    $updated = "$body`n`n$section`n"
    $path = Join-Path ([IO.Path]::GetTempPath()) ("kwu-release-body-" + [guid]::NewGuid().ToString('N') + '.md')
    try {
        [IO.File]::WriteAllText($path, $updated, [Text.UTF8Encoding]::new($false))
        Invoke-Native "record cloud mode for $($Identity.Tag)" {
            gh release edit $Identity.Tag --repo $ReleaseRepository --notes-file $path
        } | Out-Null
    }
    finally { Remove-Item -LiteralPath $path -Force -ErrorAction SilentlyContinue }
}

function Get-OrCreateReleaseSignatures {
    param($Identity, [object[]]$Assets, $Release)
    $remoteNames = @(Assert-ResumableReleaseAssets $Release)
    Push-Location (Join-Path $PSScriptRoot '..\desktop')
    try {
        foreach ($asset in $Assets) {
            $signatureName = "$($asset.Name).minisig"
            if ($remoteNames -contains $signatureName) {
                Invoke-Native "download existing signature for $($asset.Name)" {
                    gh release download $Identity.Tag --repo $ReleaseRepository `
                        --pattern $signatureName --dir $ArtifactsDirectory
                } | Out-Null
                Invoke-Native "verify existing signature for $($asset.Name)" {
                    go run ./cmd/sign verify $asset.Path
                } | Out-Null
            }
            else {
                Invoke-Native "minisign $($asset.Name)" {
                    go run ./cmd/sign sign $asset.Path
                } | Out-Null
                Invoke-Native "verify new signature for $($asset.Name)" {
                    go run ./cmd/sign verify $asset.Path
                } | Out-Null
            }
        }
    }
    finally { Pop-Location }
    return $remoteNames
}

function Assert-OrUploadFinalDraftAssets {
    param($Identity, [object[]]$Assets, $Release, [string[]]$RemoteNames, [hashtable]$GitHubManifests, [string]$BootstrapManifest = '')
    foreach ($asset in $Assets) {
        $signatureName = "$($asset.Name).minisig"
        if ($RemoteNames -notcontains $signatureName) {
            Invoke-Native "upload signature $signatureName" {
                gh release upload $Identity.Tag ($asset.Path + '.minisig') --repo $ReleaseRepository
            } | Out-Null
        }
    }
    $uploads = @{}
    foreach ($target in $GitHubManifests.Keys) { $uploads[(Get-SelectedManifestName $target)] = $GitHubManifests[$target] }
    if ($BootstrapManifest) { $uploads['latest.json'] = $BootstrapManifest }
    foreach ($entry in $uploads.GetEnumerator()) {
        $name = $entry.Key
        $manifestPath = $entry.Value
        if ($RemoteNames -contains $name) {
            $temp = Join-Path ([IO.Path]::GetTempPath()) ("kwu-existing-manifest-" + [guid]::NewGuid().ToString('N'))
            New-Item -ItemType Directory -Path $temp | Out-Null
            try {
                Invoke-Native "download existing GitHub manifest $name" {
                    gh release download $Identity.Tag --repo $ReleaseRepository --pattern $name --dir $temp
                } | Out-Null
                if ((Get-LowerSHA256 (Join-Path $temp $name)) -ne (Get-LowerSHA256 $manifestPath)) {
                    throw "Existing GitHub manifest differs from regenerated output: $name"
                }
            }
            finally { Remove-Item -LiteralPath $temp -Recurse -Force -ErrorAction SilentlyContinue }
        }
        else {
            Invoke-Native "upload GitHub manifest $name" {
                gh release upload $Identity.Tag $manifestPath --repo $ReleaseRepository
            } | Out-Null
        }
    }
    $finalDraft = Get-ReleaseJSON $Identity.Tag
    Assert-ResumableReleaseAssets $finalDraft -RequireFinal | Out-Null
    return $finalDraft
}

function Invoke-PublishGitHubOss {
    Assert-PublishIntent
    if ($CloudMode -ne 'required') { throw 'Selective stable publication requires cloud mode required' }
    $identity = Get-ReleaseIdentity $Version $Tag
    $commit = $SourceCommit.Trim().ToLowerInvariant()
    if ($commit -notmatch '^[0-9a-f]{40}$') { throw 'SourceCommit must be an exact 40-character SHA' }
    Assert-FrozenReleaseIdentity $identity $commit $TagObject
    Assert-SelectiveMigrationReady
    $notes = Get-ReleaseNotes $NotesFile
    $notesFilePath = [IO.Path]::GetFullPath($NotesFile)
    $ArtifactsDirectory = [IO.Path]::GetFullPath($ArtifactsDirectory)
    $assets = @(Assert-SelectedAssetReceipts $AssetReceipts $ArtifactsDirectory)
    if ($script:SelectedTargets -contains 'windows-arm64') { Assert-Arm64Acceptance $identity $assets }
    $release = Get-ReleaseJSON $identity.Tag
    Assert-ResumableReleaseAssets $release | Out-Null
    Assert-ReleaseBodyMetadata $release $identity $commit $notes $assets $TagObject
    foreach ($name in @('OSS_ACCESS_KEY_ID', 'OSS_ACCESS_KEY_SECRET')) {
        if (-not [Environment]::GetEnvironmentVariable($name)) { throw "Required publication secret is missing: $name" }
    }
    if (-not (Get-Command $OssutilPath -ErrorAction SilentlyContinue)) {
        throw "ossutil executable does not exist: $OssutilPath"
    }

    if (-not $WorkDirectory) { $WorkDirectory = Join-Path ([IO.Path]::GetTempPath()) 'kwu-desktop-publish' }
    $githubManifests = @{}
    $ossManifests = @{}
    $githubRoot = Join-Path $WorkDirectory 'github'
    $ossRoot = Join-Path $WorkDirectory 'oss'
    New-Item -ItemType Directory -Path $githubRoot, $ossRoot -Force | Out-Null

    $remoteNames = @(Get-OrCreateReleaseSignatures $identity $assets $release)
    Push-Location (Join-Path $PSScriptRoot '..\desktop')
    try {
        $downloadPage = "https://github.com/$ReleaseRepository/releases/tag/$($identity.Tag)"
        $previousRepository = $env:RELEASE_REPOSITORY
        try {
            $env:RELEASE_REPOSITORY = $ReleaseRepository
            foreach ($target in $script:SelectedTargets) {
                $githubManifest = Join-Path $githubRoot (Get-SelectedManifestName $target)
                $ossManifest = Join-Path $ossRoot (Get-SelectedManifestName $target)
                $platformArgs = @($script:DesktopTargetPlatforms[$target] | ForEach-Object { '--platform'; $_ })
                Invoke-Native "generate GitHub update manifest for $target" {
                    go run ./cmd/sign manifest --notes-file $notesFilePath --download-page $downloadPage `
                        --output $githubManifest @platformArgs $ArtifactsDirectory $identity.Version $identity.Tag
                } | Out-Null
                Invoke-Native "generate OSS update manifest for $target" {
                    go run ./cmd/sign manifest --notes-file $notesFilePath `
                        --base-url ($OssPublicBaseURL.TrimEnd('/') + "/releases/$($identity.Tag)/$target") `
                        --download-page $downloadPage --output $ossManifest @platformArgs `
                        $ArtifactsDirectory $identity.Version $identity.Tag
                } | Out-Null
                $githubManifests[$target] = $githubManifest
                $ossManifests[$target] = $ossManifest
            }
        }
        finally { $env:RELEASE_REPOSITORY = $previousRepository }
    }
    finally { Pop-Location }

    foreach ($target in $script:SelectedTargets) {
        Assert-OssManifestMatchesReleaseAssets $identity $ossManifests[$target] $ArtifactsDirectory $target
    }

    $bootstrapManifest = ''
    $bootstrapOssManifest = ''
    if ($Bootstrap) {
        $bootstrapManifest = Join-Path $githubRoot 'latest.json'
        $bootstrapOssManifest = Join-Path $ossRoot 'latest.json'
        Push-Location (Join-Path $PSScriptRoot '..\desktop')
        try {
            Invoke-Native 'generate bootstrap global manifest' {
                go run ./cmd/sign manifest --notes-file $notesFilePath --download-page $downloadPage `
                    --output $bootstrapManifest $ArtifactsDirectory $identity.Version $identity.Tag
            } | Out-Null
            Invoke-Native 'generate bootstrap global OSS manifest' {
                go run ./cmd/sign manifest --notes-file $notesFilePath `
                    --base-url ($OssPublicBaseURL.TrimEnd('/') + "/releases/$($identity.Tag)") `
                    --download-page $downloadPage --output $bootstrapOssManifest `
                    $ArtifactsDirectory $identity.Version $identity.Tag
            } | Out-Null
        }
        finally { Pop-Location }
    }
    Assert-CloudModeCanBeRecorded $release $identity | Out-Null
    Assert-ZeroWritePublicationPreflight $identity $release $assets $githubManifests $ossManifests `
        $bootstrapManifest $bootstrapOssManifest $notesFilePath
    $finalDraft = Assert-OrUploadFinalDraftAssets $identity $assets $release $remoteNames $githubManifests $bootstrapManifest
    Set-CloudModeInReleaseBody $finalDraft $identity
    Assert-FrozenReleaseIdentity $identity $commit $TagObject
    $prefix = "releases/$($identity.Tag)"
    foreach ($asset in $assets) {
        $target = @($script:SelectedTargets | Where-Object { $script:DesktopTargetAssets[$_] -eq $asset.Name })[0]
        Publish-OssImmutableObject "$prefix/$target/$($asset.Name)" $asset.Path
        Publish-OssImmutableObject "$prefix/$target/$($asset.Name).minisig" ($asset.Path + '.minisig')
        if ($Bootstrap) {
            Publish-OssImmutableObject "$prefix/$($asset.Name)" $asset.Path
            Publish-OssImmutableObject "$prefix/$($asset.Name).minisig" ($asset.Path + '.minisig')
        }
    }
    foreach ($target in $script:SelectedTargets) {
        Publish-OssImmutableObject "$prefix/$target/latest.json" $ossManifests[$target]
    }
    if ($Bootstrap) { Publish-OssImmutableObject "$prefix/latest.json" $bootstrapOssManifest }

    Assert-FrozenReleaseIdentity $identity $commit $TagObject
    foreach ($target in $script:SelectedTargets) {
        Assert-PublicStablePointerCanAdvance $identity $ossManifests[$target] $target
    }
    # OSS uploads can take long enough for the short-lived admin token to expire.
    # Recheck immediately before making the GitHub Release public; a failure here
    # leaves only immutable OSS objects and a recoverable Draft.
    Invoke-CloudPreflight
    if ($release.isDraft) {
        Invoke-Native "publish GitHub Release $($identity.Tag)" {
            gh release edit $identity.Tag --repo $ReleaseRepository --draft=false --latest=false
        } | Out-Null
    }
    else { Write-Host "Reusing already-published GitHub Release $($identity.Tag)" }
    $published = Get-ReleaseJSON $identity.Tag
    if ($published.isDraft) { throw "GitHub Release is still a Draft: $($identity.Tag)" }
    Assert-FrozenReleaseIdentity $identity $commit $TagObject
    foreach ($target in $script:SelectedTargets) {
        Write-ActionOutput "oss_manifest_$($target.Replace('-', '_'))" $ossManifests[$target]
    }
    Write-Host "Published immutable GitHub and OSS desktop release $($identity.Tag); target pointers remain pending until cloud publication succeeds"
}

function Set-GitHubLatestForBootstrap {
    param($Identity)
    Invoke-Native "set bootstrap GitHub Latest to $($Identity.Tag)" {
        gh release edit $Identity.Tag --repo $ReleaseRepository --latest
    } | Out-Null
    $latestJSON = Invoke-Native 'verify bootstrap GitHub Latest' {
        gh release view --repo $ReleaseRepository --json tagName
    }
    if ([string](($latestJSON | Out-String) | ConvertFrom-Json).tagName -cne $Identity.Tag) {
        throw "GitHub Latest verification failed for bootstrap $($Identity.Tag)"
    }
}

function Invoke-PublishPointers {
    Assert-PublishIntent
    $identity = Get-ReleaseIdentity $Version $Tag
    $commit = $SourceCommit.Trim().ToLowerInvariant()
    Assert-FrozenReleaseIdentity $identity $commit $TagObject
    $notes = Get-ReleaseNotes $NotesFile
    $published = Get-ReleaseJSON $identity.Tag
    if ($published.isDraft) { throw "Release is still a Draft: $($identity.Tag)" }
    Assert-ResumableReleaseAssets $published -RequireFinal | Out-Null
    Assert-ReleaseBodyMetadata $published $identity $commit $notes @() $TagObject
    Assert-SelectiveMigrationReady
    $pointerManifests = @{}
    foreach ($target in $script:SelectedTargets) {
        $pointerManifests[$target] = Join-Path $WorkDirectory "oss/$(Get-SelectedManifestName $target)"
    }
    $globalManifest = if ($Bootstrap) { Join-Path $WorkDirectory 'oss/latest.json' } else { '' }
    Assert-PointerPublicationPreflight $identity $pointerManifests $globalManifest
    if ($Bootstrap) {
        Set-GitHubLatestForBootstrap $identity
        Assert-SelectiveMigrationReady
    }
    foreach ($target in $script:SelectedTargets) {
        Assert-FrozenReleaseIdentity $identity $commit $TagObject
        Assert-SelectiveMigrationReady
        $manifest = $pointerManifests[$target]
        Assert-PublicStablePointerCanAdvance $identity $manifest $target
        Publish-OssStablePointer $manifest $target
    }
    if ($Bootstrap) {
        Assert-FrozenReleaseIdentity $identity $commit $TagObject
        Assert-SelectiveMigrationReady
        Assert-PublicStablePointerCanAdvance $identity $globalManifest
        Publish-OssStablePointer $globalManifest
        Assert-SelectiveMigrationReady
    }
}

if ($MyInvocation.InvocationName -eq '.') { return }

switch ($Action) {
    'ValidateDraft' { Invoke-ValidateDraft }
    'ValidateAssetReceipt' { Invoke-ValidateAssetReceipt }
    'ValidateAssetReceipts' { Invoke-ValidateAssetReceipts }
    'ValidateOssManifest' { Invoke-ValidateOssManifest }
    'CloudPreflight' { Invoke-CloudPreflight }
    'PublishGitHubOss' { Invoke-PublishGitHubOss }
    'PublishCloudRecords' { Invoke-PublishCloudRecords }
    'PublishPointers' { Invoke-PublishPointers }
}

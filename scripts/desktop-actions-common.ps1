. (Join-Path $PSScriptRoot 'desktop-actions-base.ps1')

$script:DefaultReleaseRepository = 'xiaoxiaotest9527-bit/KWU-release'
$script:DefaultSourceRepository = 'xiaoxiaotest9527-bit/KWU'

$script:PrimaryAssets = @(
    'Clouvia-windows-amd64-installer.exe',
    'Clouvia-windows-arm64-installer.exe',
    'Clouvia-darwin-universal.dmg'
)
$script:FinalAssets = @($script:PrimaryAssets | ForEach-Object { $_; "$_.minisig" }) + @('latest.json')
$script:DesktopReleaseTargets = @('windows-amd64', 'windows-arm64', 'macos-universal')
$script:DesktopTargetAssets = [ordered]@{
    'windows-amd64' = 'Clouvia-windows-amd64-installer.exe'
    'windows-arm64' = 'Clouvia-windows-arm64-installer.exe'
    'macos-universal' = 'Clouvia-darwin-universal.dmg'
}
$script:DesktopTargetPlatforms = [ordered]@{
    'windows-amd64' = @('windows-amd64')
    'windows-arm64' = @('windows-arm64')
    'macos-universal' = @('darwin-amd64', 'darwin-arm64')
}

function Resolve-DesktopReleaseTargets {
    param([string[]]$Targets)
    $values = @($Targets | ForEach-Object { $_ -split ',' } | ForEach-Object { $_.Trim() } | Where-Object { $_ })
    if ($values.Count -eq 0) { throw 'At least one desktop release target is required' }
    if ($values -contains 'all') {
        if (@($values | Where-Object { $_ -ne 'all' }).Count -ne 0) {
            throw 'Target all cannot be combined with concrete targets'
        }
        $values = @($script:DesktopReleaseTargets)
    }
    foreach ($value in $values) {
        if ($script:DesktopReleaseTargets -notcontains $value) {
            throw "Unknown desktop release target: $value"
        }
    }
    return @($script:DesktopReleaseTargets | Where-Object { $values -contains $_ })
}

function Get-DesktopTargetAssetNames {
    param([string[]]$Targets)
    $normalized = @(Resolve-DesktopReleaseTargets $Targets)
    return @($normalized | ForEach-Object { $script:DesktopTargetAssets[$_] })
}

function Get-DesktopPublishConfirmation {
    param([string]$Tag, [string[]]$Targets, [switch]$Bootstrap)
    $normalized = @(Resolve-DesktopReleaseTargets $Targets)
    $suffix = if ($Bootstrap) { ' BOOTSTRAP' } else { '' }
    return "PUBLISH $Tag TARGETS $($normalized -join ',')$suffix"
}

function Write-ActionOutput {
    param([string]$Name, [string]$Value)
    if ($env:GITHUB_OUTPUT) {
        Add-Content -LiteralPath $env:GITHUB_OUTPUT -Value "$Name=$Value" -Encoding utf8
    }
    Write-Host "$Name=$Value"
}

function Get-ReleaseIdentity {
    param([string]$RawVersion, [string]$RawTag)

    $candidate = $RawVersion.Trim()
    if ($RawTag.Trim()) {
        if ($RawTag.Trim() -notmatch '^desktop-(v.+)$') {
            throw "Desktop release tag must be desktop-vX.Y.Z: $RawTag"
        }
        if ($candidate -and $candidate.TrimStart('v') -ne $Matches[1].TrimStart('v')) {
            throw "Version $RawVersion does not match tag $RawTag"
        }
        $candidate = $Matches[1]
    }
    if (-not $candidate.StartsWith('v')) { $candidate = 'v' + $candidate }
    if ($candidate -notmatch '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$') {
        throw "Stable desktop version must be vX.Y.Z: $candidate"
    }
    $segments = @([int64]$Matches[1], [int64]$Matches[2], [int64]$Matches[3])
    if (($segments | Where-Object { $_ -gt 999 }).Count -ne 0) {
        throw "Version segments must be between 0 and 999: $candidate"
    }
    if ($segments[0] -eq 0 -and $segments[1] -eq 0 -and $segments[2] -eq 0) {
        throw 'Desktop version v0.0.0 is not publishable'
    }
    return [pscustomobject]@{
        Version = $candidate
        VersionName = $candidate.Substring(1)
        Tag = "desktop-$candidate"
        NotesPath = "docs/releases/$candidate.md"
    }
}

function Assert-ReleaseNotesText {
    param([string]$Text)
    $notes = $Text.Replace("`r`n", "`n").Trim()
    if (-not $notes) { throw 'Release notes are empty' }
    if ([Text.Encoding]::UTF8.GetByteCount($notes) -gt 65536) {
        throw 'Release notes exceed 65536 bytes'
    }
    if ($notes -match '!\[[^\]]*\]\s*(?:\(|\[)') {
        throw 'Release notes images are not supported in phase one'
    }
    if ($notes -match '(?i)<!--|<\/?[a-z][^>]*>') {
        throw 'Release notes HTML is not supported in phase one'
    }
    return $notes
}

function Get-ReleaseNotes {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "Release notes file does not exist: $Path"
    }
    return Assert-ReleaseNotesText (Get-Content -Raw -LiteralPath $Path)
}

function Get-LowerSHA256 {
    param([string]$Path)
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Get-RemoteAnnotatedTagIdentity {
    param([string]$ReleaseTag, [switch]$AllowMissing)
    if (-not $ReleaseTag.Trim()) { throw 'Release tag is required' }
    $escapedTag = [regex]::Escape($ReleaseTag)
    $lines = Invoke-Native "inspect remote annotated tag $ReleaseTag" {
        git ls-remote origin "refs/tags/$ReleaseTag" "refs/tags/$ReleaseTag^{}"
    }
    $direct = @($lines | Where-Object { $_ -match "^([0-9a-f]{40})\s+refs/tags/$escapedTag$" })
    $peeled = @($lines | Where-Object { $_ -match "^([0-9a-f]{40})\s+refs/tags/$escapedTag\^\{\}$" })
    if ($direct.Count -eq 0 -and $peeled.Count -eq 0 -and $AllowMissing) { return $null }
    if ($direct.Count -ne 1) { throw "Remote tag is missing or ambiguous: $ReleaseTag" }
    if ($peeled.Count -ne 1) { throw "Remote source tag must be annotated: $ReleaseTag" }
    return [pscustomobject]@{
        ObjectSHA = ([regex]::Match($direct[0], '^([0-9a-f]{40})')).Groups[1].Value.ToLowerInvariant()
        CommitSHA = ([regex]::Match($peeled[0], '^([0-9a-f]{40})')).Groups[1].Value.ToLowerInvariant()
    }
}

function Assert-RemoteAnnotatedTagIdentity {
    param([string]$ReleaseTag, [string]$TagObject, [string]$Commit)
    $expectedObject = $TagObject.Trim().ToLowerInvariant()
    $expectedCommit = $Commit.Trim().ToLowerInvariant()
    if ($expectedObject -notmatch '^[0-9a-f]{40}$') { throw 'TagObject must be an exact 40-character SHA' }
    if ($expectedCommit -notmatch '^[0-9a-f]{40}$') { throw 'SourceCommit must be an exact 40-character SHA' }
    $remote = Get-RemoteAnnotatedTagIdentity $ReleaseTag
    if ($remote.ObjectSHA -ne $expectedObject) {
        throw "Remote annotated tag object changed for $ReleaseTag`: expected $expectedObject, got $($remote.ObjectSHA)"
    }
    if ($remote.CommitSHA -ne $expectedCommit) {
        throw "Remote source tag $ReleaseTag points to $($remote.CommitSHA) instead of $expectedCommit"
    }
    return $remote
}

function Get-AssetMetadata {
    param([string]$Directory, [string[]]$Targets = @('all'))
    if (-not (Test-Path -LiteralPath $Directory -PathType Container)) {
        throw "Artifact directory does not exist: $Directory"
    }
    $files = @(Get-ChildItem -LiteralPath $Directory -File -Recurse)
    $names = @($files | ForEach-Object Name)
    $requiredAssets = @(Get-DesktopTargetAssetNames $Targets)
    if ($names.Count -ne $requiredAssets.Count) {
        throw "Primary artifact directory must contain exactly $($requiredAssets.Count) selected files; found $($names.Count)"
    }
    $result = @()
    foreach ($name in $requiredAssets) {
        $matches = @($files | Where-Object Name -eq $name)
        if ($matches.Count -ne 1) { throw "Primary artifact is missing or duplicated: $name" }
        if ($matches[0].Length -le 0) { throw "Primary artifact is empty: $name" }
        $architecture = switch ($name) {
            'Clouvia-windows-amd64-installer.exe' { 'amd64 payload' }
            'Clouvia-windows-arm64-installer.exe' { 'arm64 payload' }
            default { 'x86_64+arm64 payload' }
        }
        $result += [pscustomobject]@{
            Name = $name
            Path = $matches[0].FullName
            Size = $matches[0].Length
            SHA256 = Get-LowerSHA256 $matches[0].FullName
            Architecture = $architecture
        }
    }
    return $result
}

function New-ReleaseBody {
    param($Identity, [string]$Commit, [string]$Notes, [object[]]$Assets, [string]$TagObject = '')
    $assetLines = @($Assets | ForEach-Object {
        "- ``$($_.Name)``: size=$($_.Size) sha256=``$($_.SHA256)`` architecture=$($_.Architecture)"
    }) -join "`n"
    if (-not $assetLines) { $assetLines = '(Pending manual builds.)' }
    $emergencyBlock = "- Emergency: ``false```n"
    $tagObjectLine = if ($TagObject.Trim()) { "- Source tag object: ``$($TagObject.Trim().ToLowerInvariant())```n" } else { '' }
    return @"
# 云智企办 $($Identity.Version)

## Release provenance

- Source repository: ``$SourceRepository``
- Source tag: ``$($Identity.Tag)``
${tagObjectLine}- Source commit: ``$Commit``
- Release tag: ``$($Identity.Tag)``
$emergencyBlock
## Verified primary assets

$assetLines

## 更新说明

$Notes
"@.Replace("`r`n", "`n").Trim()
}

function Get-ReleaseJSON {
    param([string]$ReleaseTag, [switch]$AllowMissing)
    $json = & gh release view $ReleaseTag --repo $ReleaseRepository --json isDraft,body,assets,tagName 2>$null
    if ($LASTEXITCODE -ne 0) {
        if ($AllowMissing) { return $null }
        throw "GitHub release does not exist: $ReleaseRepository/$ReleaseTag"
    }
    return (($json | Out-String) | ConvertFrom-Json)
}

function Assert-ReleaseAssetNames {
    param($Release, [string[]]$Expected)
    $names = @($Release.assets | ForEach-Object { [string]$_.name })
    if ($names.Count -ne $Expected.Count) {
        throw "Release must contain exactly $($Expected.Count) assets; found $($names.Count)"
    }
    foreach ($name in $Expected) {
        if ($names -notcontains $name) { throw "Release is missing asset: $name" }
    }
}

function Assert-ReleaseBodyMetadata {
    param($Release, $Identity, [string]$Commit, [string]$Notes, [object[]]$Assets, [string]$TagObject = '')
    if ($TagObject.Trim().ToLowerInvariant() -notmatch '^[0-9a-f]{40}$') {
        throw 'Release body verification requires an exact annotated tag object SHA'
    }
    $body = ([string]$Release.body).Replace("`r`n", "`n")
    foreach ($required in @(
        "Source repository: ``$SourceRepository``",
        "Source tag: ``$($Identity.Tag)``",
        "Source commit: ``$Commit``",
        "## 更新说明`n`n$Notes"
    )) {
        if (-not $body.Contains($required)) {
            throw "Release body provenance or notes do not match: $required"
        }
    }
    if (-not $body.Contains("Source tag object: ``$($TagObject.Trim().ToLowerInvariant())``")) {
        throw 'Release body annotated tag object does not match'
    }
    if (-not $body.Contains('Emergency: `false`')) {
        throw 'Release body emergency flag does not match'
    }
    foreach ($asset in $Assets) {
        $line = "``$($asset.Name)``: size=$($asset.Size) sha256=``$($asset.SHA256)`` architecture=$($asset.Architecture)"
        if (-not $body.Contains($line)) { throw "Release body asset metadata does not match: $($asset.Name)" }
    }
}

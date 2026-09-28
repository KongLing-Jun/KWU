[CmdletBinding()]
param(
    [ValidateSet('Doctor', 'Prepare', 'BuildWindows', 'Finalize')]
    [string]$Action = 'Doctor',
    [string]$Version = '',
    [string]$SourceRef = '',
    [switch]$AllowNonMainV3,
    [string]$EmergencyReason = '',
    [string]$ReleaseNotesFile = '',
    [string]$Confirmation = '',
    [string[]]$ReleaseTargets = @('all'),
    [switch]$Bootstrap,
    [string]$Arm64AcceptanceConfirmation = '',
    [string]$Arm64AcceptanceSHA256 = '',
    [switch]$SkipCloud,
    [string]$CloudSkipReason = '',
    [switch]$DryRun,
    [string]$RepositoryRoot = '',
    [string]$ReleaseWorkRoot = 'D:\suiyuan-release-work',
    [string]$SourceRepository = 'xiaoxiaotest9527-bit/KWU',
    [string]$ReleaseRepository = 'xiaoxiaotest9527-bit/KWU-release',
    [string]$CloudBaseURL = 'https://www.suiyuan-ai.com',
    [string]$BashPath = '',
    [string]$OssutilPath = '',
    [string]$OssutilConfigFile = '',
    [string]$OssutilProfile = '',
    [string]$MinisignPrivateKeyFile = '',
    [string]$MinisignPasswordFile = 'D:\suiyuan-release-key\minisign-password.dpapi',
    [string]$CloudTokenDPAPIFile = '',
    [string]$OssBucket = 'kwu-desktop-releases',
    [string]$OssPublicBaseURL = 'https://kwu-desktop-releases.oss-cn-guangzhou.aliyuncs.com'
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'desktop-oss-doctor.ps1')

if ($AllowNonMainV3 -or $EmergencyReason.Trim()) {
    throw 'Desktop release tooling accepts only canonical annotated tags reachable from main-v3'
}

$script:ReleaseTargetOrder = @('windows-amd64', 'windows-arm64', 'macos-universal')
$script:ReleaseTargetAssets = [ordered]@{
    'windows-amd64' = 'Clouvia-windows-amd64-installer.exe'
    'windows-arm64' = 'Clouvia-windows-arm64-installer.exe'
    'macos-universal' = 'Clouvia-darwin-universal.dmg'
}
function Resolve-LocalReleaseTargets {
    $values = @($ReleaseTargets | ForEach-Object { $_ -split ',' } | ForEach-Object { $_.Trim() } | Where-Object { $_ })
    if ($values.Count -eq 0) { throw 'At least one desktop release target is required' }
    if ($values -contains 'all') {
        if (@($values | Where-Object { $_ -ne 'all' }).Count) { throw 'all cannot be combined with concrete targets' }
        return @($script:ReleaseTargetOrder)
    }
    foreach ($value in $values) {
        if ($script:ReleaseTargetOrder -notcontains $value) { throw "Unknown desktop release target: $value" }
    }
    return @($script:ReleaseTargetOrder | Where-Object { $values -contains $_ })
}

if ([string]::IsNullOrWhiteSpace($RepositoryRoot)) {
    $RepositoryRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
}
if ([string]::IsNullOrWhiteSpace($OssutilPath)) {
    $OssutilPath = if ($env:OSSUTIL_PATH) { $env:OSSUTIL_PATH } else { 'D:\suiyuan-release-key\tools\ossutil-2.3.0-windows-amd64\ossutil.exe' }
}
if ([string]::IsNullOrWhiteSpace($OssutilConfigFile)) {
    $OssutilConfigFile = if ($env:OSSUTIL_CONFIG_FILE) { $env:OSSUTIL_CONFIG_FILE } else { 'D:\suiyuan-release-key\.ossutilconfig' }
}
if ([string]::IsNullOrWhiteSpace($OssutilProfile)) {
    $OssutilProfile = if ($env:OSSUTIL_PROFILE) { $env:OSSUTIL_PROFILE } else { 'kwu-release' }
}
if ([string]::IsNullOrWhiteSpace($MinisignPrivateKeyFile)) {
    $MinisignPrivateKeyFile = if ($env:MINISIGN_PRIVATE_KEY_FILE) { $env:MINISIGN_PRIVATE_KEY_FILE } else { 'D:\suiyuan-release-key\suiyuan.key' }
}

function Assert-CommandSucceeded {
    param([string]$Description, [int]$ExitCode)
    if ($ExitCode -ne 0) {
        throw "$Description failed with exit code $ExitCode"
    }
}

function Assert-Tool {
    param([string]$Name)
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "Required tool is not available: $Name"
    }
}

function Get-BashExecutable {
    if (-not [string]::IsNullOrWhiteSpace($BashPath)) {
        if (-not (Test-Path -LiteralPath $BashPath -PathType Leaf)) {
            throw "Bash executable does not exist: $BashPath"
        }
        return $BashPath
    }
    if (-not [string]::IsNullOrWhiteSpace($env:BASH_PATH)) {
        if (-not (Test-Path -LiteralPath $env:BASH_PATH -PathType Leaf)) {
            throw "BASH_PATH does not exist: $env:BASH_PATH"
        }
        return $env:BASH_PATH
    }
    $candidates = @(
        (Join-Path $env:ProgramFiles 'Git\bin\bash.exe'),
        (Join-Path $env:LOCALAPPDATA 'Programs\Git\bin\bash.exe')
    )
    foreach ($candidate in $candidates) {
        if (Test-Path -LiteralPath $candidate -PathType Leaf) { return $candidate }
    }
    $command = Get-Command bash -ErrorAction SilentlyContinue
    if ($command -and $command.Source -notmatch '(?i)\\Windows\\System32\\bash\.exe$') { return $command.Source }
    throw 'Git Bash is required for desktop-build.sh; set -BashPath or BASH_PATH'
}

function Get-NSISBinDirectory {
    $command = Get-Command makensis.exe -ErrorAction SilentlyContinue
    if ($command) { return Split-Path -Parent $command.Source }
    $candidates = @(
        (Join-Path ${env:ProgramFiles(x86)} 'NSIS\Bin'),
        (Join-Path $env:ProgramFiles 'NSIS\Bin')
    )
    foreach ($candidate in $candidates) {
        if ($candidate -and (Test-Path -LiteralPath (Join-Path $candidate 'makensis.exe') -PathType Leaf)) {
            return $candidate
        }
    }
    throw 'NSIS makensis.exe is required; install the pinned release-machine version before publishing'
}

function Invoke-Doctor {
    foreach ($tool in @('git', 'gh', 'go', 'node', 'pnpm', 'wails', 'pwsh')) {
        Assert-Tool $tool
        Write-Host "OK tool: $tool"
    }
    $bash = Get-BashExecutable
    $null = & $bash --version 2>&1
    Assert-CommandSucceeded 'Bash version check' $LASTEXITCODE
    Write-Host "OK Bash: $bash"
    $nsisBin = Get-NSISBinDirectory
    $null = & (Join-Path $nsisBin 'makensis.exe') /VERSION 2>&1
    Assert-CommandSucceeded 'NSIS version check' $LASTEXITCODE
    Write-Host "OK NSIS: $nsisBin"
    if (-not $DryRun) {
        $authenticode = Join-Path $PSScriptRoot 'desktop-windows-authenticode.ps1'
        & $authenticode -Action Preflight
    }

    if (-not (Test-Path -LiteralPath $RepositoryRoot -PathType Container)) {
        throw "Repository root does not exist: $RepositoryRoot"
    }
    $inside = & git -C $RepositoryRoot rev-parse --is-inside-work-tree 2>&1
    Assert-CommandSucceeded 'git repository check' $LASTEXITCODE
    if (($inside | Out-String).Trim() -ne 'true') {
        throw "Not a Git worktree: $RepositoryRoot"
    }
    $status = & git -C $RepositoryRoot status --porcelain=v1 2>&1
    Assert-CommandSucceeded 'git status' $LASTEXITCODE
    if (-not [string]::IsNullOrWhiteSpace(($status | Out-String))) {
        throw "Release repository worktree must be clean: $RepositoryRoot"
    }
    Write-Host 'OK Git worktree is clean'

    $null = & gh auth status 2>&1
    Assert-CommandSucceeded 'gh auth status' $LASTEXITCODE
    $canPush = (& gh api "repos/$ReleaseRepository" --jq '.permissions.push' 2>&1 | Out-String).Trim().ToLowerInvariant()
    Assert-CommandSucceeded "GitHub release repository access ($ReleaseRepository)" $LASTEXITCODE
    if ($canPush -ne 'true') { throw "Current gh token cannot write $ReleaseRepository" }
    Write-Host "OK GitHub write access: $ReleaseRepository"

    if (-not (Test-Path -LiteralPath $OssutilPath -PathType Leaf)) {
        throw "ossutil executable does not exist: $OssutilPath"
    }
    $null = & $OssutilPath version 2>&1
    Assert-CommandSucceeded 'ossutil version check' $LASTEXITCODE
    if (-not (Test-Path -LiteralPath $OssutilConfigFile -PathType Leaf)) {
        throw "ossutil config file does not exist: $OssutilConfigFile"
    }
    $configText = Get-Content -Raw -LiteralPath $OssutilConfigFile
    if (-not $configText.Contains($OssutilProfile)) {
        throw "ossutil profile '$OssutilProfile' was not found in $OssutilConfigFile"
    }
    $pointerOutput = & $OssutilPath cat "oss://$OssBucket/channels/stable/latest.json" `
        --config-file $OssutilConfigFile --profile $OssutilProfile --quiet 2>&1
    $pointerExitCode = $LASTEXITCODE
    $pointerProbe = Resolve-DesktopStablePointerProbe $pointerExitCode ($pointerOutput | Out-String)
    if (-not $pointerProbe.Present) {
        Write-Host 'OK OSS stable pointer is absent before the first desktop release'
    }
    Write-Host "OK ossutil profile: $OssutilProfile"

    if (-not (Test-Path -LiteralPath $MinisignPrivateKeyFile -PathType Leaf)) {
        throw "Encrypted minisign private key does not exist: $MinisignPrivateKeyFile"
    }
    if ((Get-Item -LiteralPath $MinisignPrivateKeyFile).Length -le 0) {
        throw "Encrypted minisign private key is empty: $MinisignPrivateKeyFile"
    }
    $previousKeyFile = $env:MINISIGN_PRIVATE_KEY_FILE
    $previousKeyText = $env:MINISIGN_PRIVATE_KEY
    $previousPassword = $env:MINISIGN_PASSWORD
    try {
        $env:MINISIGN_PRIVATE_KEY_FILE = $MinisignPrivateKeyFile
        $env:MINISIGN_PRIVATE_KEY = $null
        if (-not $env:MINISIGN_PASSWORD) {
            $storedPassword = Get-DPAPIMinisignPassword
            if ($storedPassword) { $env:MINISIGN_PASSWORD = $storedPassword }
        }
        Invoke-DesktopGoCommand -Arguments @('run', './cmd/sign', 'selftest') -Description 'minisign private key self-test'
    }
    finally {
        $env:MINISIGN_PRIVATE_KEY_FILE = $previousKeyFile
        $env:MINISIGN_PRIVATE_KEY = $previousKeyText
        $env:MINISIGN_PASSWORD = $previousPassword
    }
    Write-Host 'OK encrypted minisign private key matches the embedded public key'

    Write-Host "Doctor passed$(if ($DryRun) { ' (DryRun)' } else { '' })"
}

function Get-ReleaseIdentity {
    param([string]$RawVersion)
    $candidate = $RawVersion.Trim()
    if (-not $candidate.StartsWith('v')) {
        $candidate = 'v' + $candidate
    }
    if ($candidate -notmatch '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$') {
        throw "Stable release version must be vX.Y.Z: $RawVersion"
    }
    foreach ($segment in @($Matches[1], $Matches[2], $Matches[3])) {
        if ([int]$segment -gt 999) {
            throw "Version segments must be between 0 and 999: $RawVersion"
        }
    }
    if ([int]$Matches[1] -eq 0 -and [int]$Matches[2] -eq 0 -and [int]$Matches[3] -eq 0) {
        throw 'Release version v0.0.0 cannot produce a positive version_code'
    }
    return [pscustomobject]@{
        Version = $candidate
        VersionName = $candidate.Substring(1)
        SourceTag = "desktop-$candidate"
        LegacySourceTag = "manual-desktop-$candidate"
        ReleaseTag = "desktop-$candidate"
        NotesPath = "docs/releases/$candidate.md"
    }
}

function Assert-SingleLineReason {
    param([string]$Value, [string]$Name)
    $reason = $Value.Trim()
    if (-not $reason) { throw "$Name is required" }
    if ($reason.Length -gt 500 -or $reason.Contains("`r") -or $reason.Contains("`n")) {
        throw "$Name must be a single line of at most 500 characters"
    }
    return $reason
}

function Get-DPAPIMinisignPassword {
    if (-not (Test-Path -LiteralPath $MinisignPasswordFile -PathType Leaf)) { return $null }
    if (-not [Runtime.InteropServices.RuntimeInformation]::IsOSPlatform([Runtime.InteropServices.OSPlatform]::Windows)) {
        throw 'The minisign DPAPI password file can only be decrypted by its Windows CurrentUser account'
    }
    Add-Type -AssemblyName System.Security -ErrorAction SilentlyContinue
    $protected = [IO.File]::ReadAllBytes($MinisignPasswordFile)
    if ($protected.Length -eq 0) { throw "DPAPI password file is empty: $MinisignPasswordFile" }
    $plain = [Security.Cryptography.ProtectedData]::Unprotect(
        $protected,
        $null,
        [Security.Cryptography.DataProtectionScope]::CurrentUser
    )
    try {
        $password = [Text.Encoding]::UTF8.GetString($plain)
        if (-not $password) { throw "DPAPI password file decrypted to an empty password: $MinisignPasswordFile" }
        return $password
    }
    finally { [Array]::Clear($plain, 0, $plain.Length) }
}

$script:CloudTokenDPAPIFileReader = {
    param([string]$Path)
    Get-Content -Raw -LiteralPath $Path
}

$script:CloudTokenDPAPIFileRemover = {
    param([string]$Path)
    Remove-Item -LiteralPath $Path -Force
}

function Get-DPAPICloudReleaseToken {
    if ([string]::IsNullOrWhiteSpace($CloudTokenDPAPIFile)) { return $null }
    if (-not [Runtime.InteropServices.RuntimeInformation]::IsOSPlatform([Runtime.InteropServices.OSPlatform]::Windows)) {
        throw 'The cloud token DPAPI file can only be decrypted by its Windows CurrentUser account'
    }
    try {
        $handoffFileExists = Test-Path -LiteralPath $CloudTokenDPAPIFile -PathType Leaf
    }
    catch {
        throw 'Cloud token DPAPI handoff file is unavailable'
    }
    if (-not $handoffFileExists) {
        throw 'Cloud token DPAPI handoff file is unavailable'
    }
    $readSucceeded = $false
    try {
        try {
            $encrypted = & $script:CloudTokenDPAPIFileReader $CloudTokenDPAPIFile
        }
        catch {
            throw 'Cloud token DPAPI handoff file could not be read'
        }
        $readSucceeded = $true
        if ([string]::IsNullOrWhiteSpace($encrypted)) {
            throw 'Cloud token DPAPI handoff file is empty'
        }
        try {
            $secure = $encrypted | ConvertTo-SecureString
        }
        catch {
            throw 'Cloud token DPAPI handoff file could not be decrypted'
        }
        $pointer = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
        try {
            $token = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($pointer)
            if ([string]::IsNullOrWhiteSpace($token)) {
                throw 'Cloud token DPAPI handoff file decrypted to an empty token'
            }
            return $token
        }
        finally {
            if ($pointer -ne [IntPtr]::Zero) {
                [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($pointer)
            }
        }
    }
    finally {
        if ($readSucceeded) {
            try {
                & $script:CloudTokenDPAPIFileRemover $CloudTokenDPAPIFile
            }
            catch {
                throw 'Cloud token DPAPI handoff file could not be deleted'
            }
        }
    }
}

function Get-ReleaseWorkDirectory {
    param($Identity)
    return Join-Path $ReleaseWorkRoot $Identity.ReleaseTag
}

function Get-RemoteMainV3Commit {
    $line = & git -C $RepositoryRoot ls-remote origin refs/heads/main-v3 2>&1
    Assert-CommandSucceeded 'resolve remote main-v3' $LASTEXITCODE
    $text = ($line | Out-String).Trim()
    if ($text -notmatch '^([0-9a-f]{40})\s+refs/heads/main-v3$') {
        throw 'origin/main-v3 did not resolve to one commit'
    }
    return $Matches[1]
}

function Resolve-SourceCommit {
    param([string]$RequestedRef)
    $mainRefs = @('', 'main-v3', 'origin/main-v3', 'refs/heads/main-v3')
    if ($mainRefs -contains $RequestedRef.Trim()) {
        $remoteCommit = Get-RemoteMainV3Commit
        $null = & git -C $RepositoryRoot fetch --force --no-tags origin '+refs/heads/main-v3:refs/remotes/origin/main-v3' 2>&1
        Assert-CommandSucceeded 'fetch origin/main-v3' $LASTEXITCODE
        $localCommit = (& git -C $RepositoryRoot rev-parse 'origin/main-v3^{commit}' 2>&1 | Out-String).Trim()
        Assert-CommandSucceeded 'resolve fetched origin/main-v3' $LASTEXITCODE
        if ($localCommit -ne $remoteCommit) {
            throw "Fetched origin/main-v3 ($localCommit) does not match remote ($remoteCommit)"
        }
        return $remoteCommit
    }
    throw 'Desktop releases may only freeze the current origin/main-v3 commit'
}

function Get-RemoteAnnotatedTag {
    param([string]$Tag)
    $lines = & git -C $RepositoryRoot ls-remote origin "refs/tags/$Tag" "refs/tags/$Tag^{}" 2>&1
    Assert-CommandSucceeded "inspect source tag $Tag" $LASTEXITCODE
    $direct = ''
    $peeled = ''
    foreach ($line in $lines) {
        if ($line -match '^([0-9a-f]{40})\s+refs/tags/.+\^\{\}$') { $peeled = $Matches[1] }
        elseif ($line -match '^([0-9a-f]{40})\s+refs/tags/.+$') { $direct = $Matches[1] }
    }
    if (-not $direct) { return $null }
    if (-not $peeled) {
        throw "Remote source tag must be annotated: $Tag"
    }
    return [pscustomobject]@{ Tag = $Tag; ObjectSHA = $direct; Commit = $peeled }
}

function Resolve-RemoteSourceTag {
    param($Identity)
    $standard = Get-RemoteAnnotatedTag $Identity.SourceTag
    if ($standard) { return $standard }
    $legacy = Get-RemoteAnnotatedTag $Identity.LegacySourceTag
    if ($legacy) {
        Write-Warning "Using legacy source tag $($legacy.Tag); new releases must use $($Identity.SourceTag)"
        return $legacy
    }
    return $null
}

function Assert-RemoteSourceTagUnchanged {
    param($Expected)
    $current = Get-RemoteAnnotatedTag $Expected.Tag
    if (-not $current) {
        throw "Remote source tag disappeared during release: $($Expected.Tag)"
    }
    if ($current.ObjectSHA -cne $Expected.ObjectSHA -or $current.Commit -cne $Expected.Commit) {
        throw "Remote source tag moved during release: $($Expected.Tag) expected $($Expected.ObjectSHA) -> $($Expected.Commit), found $($current.ObjectSHA) -> $($current.Commit)"
    }
}

function Assert-BuildWorktreePristine {
    param([string]$Path)
    $status = & git -C $Path status --porcelain=v1 --untracked-files=all --ignored=matching 2>&1
    Assert-CommandSucceeded "inspect frozen build worktree $Path" $LASTEXITCODE
    $details = ($status | Out-String).Trim()
    if ($details) {
        throw "Frozen build worktree contains files outside the source commit; remove and recreate it before publishing: $Path`n$details"
    }
}

function Assert-BuildWorktreeTrackedClean {
    param([string]$Path)
    $status = & git -C $Path status --porcelain=v1 --untracked-files=no 2>&1
    Assert-CommandSucceeded "reinspect frozen build worktree $Path" $LASTEXITCODE
    $details = ($status | Out-String).Trim()
    if ($details) {
        throw "Build changed tracked source files; refusing to publish artifacts for the frozen commit: $Path`n$details"
    }
}

function Import-RemoteSourceTagObjects {
    param($Source)
    $null = & git -C $RepositoryRoot fetch --no-tags origin "refs/tags/$($Source.Tag)" 2>&1
    Assert-CommandSucceeded "fetch source tag objects for $($Source.Tag)" $LASTEXITCODE
    $objectType = (& git -C $RepositoryRoot cat-file -t $Source.ObjectSHA 2>&1 | Out-String).Trim()
    Assert-CommandSucceeded "inspect source tag object $($Source.ObjectSHA)" $LASTEXITCODE
    if ($objectType -ne 'tag') { throw "Remote source tag object is not annotated: $($Source.Tag)" }
    $fetchedCommit = (& git -C $RepositoryRoot rev-parse "$($Source.ObjectSHA)^{commit}" 2>&1 | Out-String).Trim()
    Assert-CommandSucceeded "peel source tag object $($Source.ObjectSHA)" $LASTEXITCODE
    if ($fetchedCommit -ne $Source.Commit) {
        throw "Fetched source tag object $($Source.ObjectSHA) peels to $fetchedCommit instead of $($Source.Commit)"
    }
}

function Test-SourceCommitIsMainV3 {
    param([string]$Commit)
    $null = & git -C $RepositoryRoot fetch --force --no-tags origin '+refs/heads/main-v3:refs/remotes/origin/main-v3' 2>&1
    Assert-CommandSucceeded 'fetch origin/main-v3 for release provenance' $LASTEXITCODE
    $null = & git -C $RepositoryRoot merge-base --is-ancestor $Commit 'origin/main-v3' 2>&1
    if ($LASTEXITCODE -eq 0) { return $true }
    if ($LASTEXITCODE -eq 1) { return $false }
    throw "Could not compare source commit $Commit with origin/main-v3"
}

function Assert-ReleaseEmergencyBody {
    param(
        $Release,
        [switch]$AllowLegacyNormalWithoutMarker
    )
    $body = ([string]$Release.body).Replace("`r`n", "`n")
    $hasEmergency = $body.Contains('Emergency: `true`')
    $hasNormal = $body.Contains('Emergency: `false`')
    if ($hasEmergency -and $hasNormal) { throw 'Release body contains conflicting emergency flags' }
    if ($hasEmergency) { throw 'Emergency desktop release provenance is no longer supported' }
    if (-not $hasNormal -and -not $AllowLegacyNormalWithoutMarker) {
        throw 'Release body is missing the explicit normal-release provenance flag'
    }
}

function Assert-ManualTakeoverProvenance {
    param($Release, $Identity, $Source)
    Import-RemoteSourceTagObjects $Source
    if ($Source.Tag -cne $Identity.SourceTag) {
        throw "Desktop release tooling requires the canonical annotated tag: $($Identity.SourceTag)"
    }
    if (-not (Test-SourceCommitIsMainV3 $Source.Commit)) {
        throw "Desktop release source is not reachable from origin/main-v3: $($Source.Commit)"
    }
    Assert-ReleaseEmergencyBody $Release
}

function Normalize-ReleaseNotes {
    param([AllowEmptyString()][string]$Text)
    if ($null -eq $Text) { return '' }
    return $Text.Replace("`r`n", "`n").Replace("`r", "`n").Trim()
}

function Assert-ReleaseNotesText {
    param([AllowEmptyString()][string]$Text)
    $notes = Normalize-ReleaseNotes $Text
    if ([string]::IsNullOrWhiteSpace($notes)) {
        throw 'Release notes are empty'
    }
    if ([System.Text.Encoding]::UTF8.GetByteCount($notes) -gt 65536) {
        throw 'Release notes exceed the cloud 65536-byte limit'
    }
    if ($notes -match '!\[[^\]]*\]\s*(?:\(|\[)') {
        throw 'Release notes images are not supported in phase one'
    }
    if ($notes -match '(?i)<!--|<\/?[a-z][^>]*>') {
        throw 'Release notes HTML is not supported in phase one'
    }
    return $notes
}

function Get-ReleaseNotesAtCommit {
    param($Identity, [string]$Commit)
    $spec = "$Commit`:$($Identity.NotesPath)"
    $output = & git -C $RepositoryRoot show $spec 2>&1
    Assert-CommandSucceeded "read $($Identity.NotesPath) at source commit $Commit" $LASTEXITCODE
    return Assert-ReleaseNotesText (($output | Out-String))
}

function New-ReleaseBody {
    param($Identity, [string]$Commit, [string]$Notes, [string]$TagObject = '')
    $tagObjectLine = if ($TagObject.Trim()) { "- Source tag object: ``$($TagObject.Trim().ToLowerInvariant())```n" } else { '' }
    $emergencyBlock = "- Emergency: ``false```n"
    return @"
# 云智企办 $($Identity.Version)

## Release provenance

- Source repository: ``$SourceRepository``
- Source tag: ``$($Identity.SourceTag)``
${tagObjectLine}- Source commit: ``$Commit``
- Release tag: ``$($Identity.ReleaseTag)``
$emergencyBlock

## Verified primary assets

(Pending manual builds.)

## 更新说明

$Notes
"@.Replace("`r`n", "`n").Trim()
}

function Assert-ReleaseBodyCore {
    param($Draft, $Identity, [string]$SourceTag, [string]$Commit, [string]$Notes, [string]$TagObject = '')
    $body = ([string]$Draft.body).Replace("`r`n", "`n")
    $acceptedRepositories = @(
        "Source repository: ``$SourceRepository``",
        "Source repository: https://github.com/$SourceRepository",
        'Source repository: https://github.com/xiaoxiaotest9527-bit/KWU'
    )
    if (-not ($acceptedRepositories | Where-Object { $body.Contains($_) })) {
        throw 'Draft Release source repository provenance is missing or unrecognized'
    }
    foreach ($required in @(
        "Source tag: ``$SourceTag``",
        "Source commit: ``$Commit``",
        "Release tag: ``$($Identity.ReleaseTag)``",
        "## 更新说明`n`n$Notes"
    )) {
        if (-not $body.Contains($required)) {
            throw "Draft Release provenance or notes do not match: $required"
        }
    }
    if ($TagObject.Trim() -and -not $body.Contains("Source tag object: ``$($TagObject.Trim().ToLowerInvariant())``")) {
        throw 'Draft Release annotated tag object provenance does not match'
    }
}

function Invoke-Prepare {
    $identity = Get-ReleaseIdentity $Version
    $commit = Resolve-SourceCommit $SourceRef
    $notes = Get-ReleaseNotesAtCommit $identity $commit
    if (-not [string]::IsNullOrWhiteSpace($ReleaseNotesFile)) {
        if (-not (Test-Path -LiteralPath $ReleaseNotesFile -PathType Leaf)) {
            throw "ReleaseNotesFile does not exist: $ReleaseNotesFile"
        }
        $providedNotes = Assert-ReleaseNotesText (Get-Content -Raw -LiteralPath $ReleaseNotesFile)
        if ($providedNotes -cne $notes) {
            throw "ReleaseNotesFile must exactly match $($identity.NotesPath) at source commit $commit (line endings and surrounding whitespace are normalized)"
        }
    }
    $existingTag = Get-RemoteAnnotatedTag $identity.SourceTag
    if ($existingTag -and $existingTag.Commit -ne $commit) {
        throw "Remote source tag $($identity.SourceTag) points to $($existingTag.Commit) instead of $commit"
    }

    $draftJSON = & gh release view $identity.ReleaseTag --repo $ReleaseRepository --json isDraft,body 2>$null
    $draftExists = $LASTEXITCODE -eq 0
    if ($draftExists) {
        $draft = ($draftJSON | Out-String) | ConvertFrom-Json
        if (-not $draft.isDraft) {
            throw "GitHub release $($identity.ReleaseTag) already exists and is not a draft"
        }
        if (-not $existingTag) {
            throw "Draft Release exists without its immutable source tag: $($identity.SourceTag)"
        }
        Assert-ReleaseBodyCore $draft $identity $identity.SourceTag $commit $notes $existingTag.ObjectSHA
        Assert-ReleaseEmergencyBody $draft
    }

    if ($DryRun) {
        Write-Host "DryRun Prepare: source $commit -> $($identity.SourceTag)"
        Write-Host "DryRun Prepare: Draft Release $ReleaseRepository/$($identity.ReleaseTag)"
        return
    }

    if (-not $existingTag) {
        $localTagCommit = (& git -C $RepositoryRoot rev-parse --verify --quiet "refs/tags/$($identity.SourceTag)^{commit}" | Out-String).Trim()
        if ($LASTEXITCODE -eq 0 -and $localTagCommit) {
            if ($localTagCommit -ne $commit) {
                throw "Local source tag $($identity.SourceTag) points to $localTagCommit instead of $commit"
            }
            $localTagType = (& git -C $RepositoryRoot cat-file -t "refs/tags/$($identity.SourceTag)" 2>&1 | Out-String).Trim()
            Assert-CommandSucceeded "inspect local source tag $($identity.SourceTag)" $LASTEXITCODE
            if ($localTagType -ne 'tag') {
                throw "Local source tag must be annotated: $($identity.SourceTag)"
            }
        }
        else {
            $null = & git -C $RepositoryRoot tag -a $identity.SourceTag $commit -m "KWU desktop $($identity.Version) source freeze" 2>&1
            Assert-CommandSucceeded "create source tag $($identity.SourceTag)" $LASTEXITCODE
        }
        $null = & git -C $RepositoryRoot push origin "refs/tags/$($identity.SourceTag)" 2>&1
        Assert-CommandSucceeded "push source tag $($identity.SourceTag)" $LASTEXITCODE
        $existingTag = Get-RemoteAnnotatedTag $identity.SourceTag
        if (-not $existingTag -or $existingTag.Commit -ne $commit) {
            throw "Remote source tag $($identity.SourceTag) was not frozen at $commit"
        }
    }

    $body = New-ReleaseBody $identity $commit $notes $existingTag.ObjectSHA
    if ($draftExists) {
        Assert-ReleaseBodyCore $draft $identity $identity.SourceTag $commit $notes $existingTag.ObjectSHA
        Assert-ReleaseEmergencyBody $draft
    }

    $workDirectory = Get-ReleaseWorkDirectory $identity
    New-Item -ItemType Directory -Path $workDirectory -Force | Out-Null
    $notesSnapshot = Join-Path $workDirectory 'release-notes.md'
    [System.IO.File]::WriteAllText($notesSnapshot, $notes + "`n", [System.Text.UTF8Encoding]::new($false))
    $bodyPath = Join-Path $workDirectory 'release-body.md'
    Set-Content -LiteralPath $bodyPath -Value $body -Encoding utf8 -NoNewline
    if (-not $draftExists) {
        $null = & gh release create $identity.ReleaseTag --repo $ReleaseRepository --draft --title "云智企办 $($identity.Version)" --notes-file $bodyPath 2>&1
        Assert-CommandSucceeded "create Draft Release $($identity.ReleaseTag)" $LASTEXITCODE
    }
    Write-Host "Prepared $($identity.ReleaseTag) from $commit"
}

function Get-DraftRelease {
    param($Identity, [switch]$AllowPublished)
    $json = & gh release view $Identity.ReleaseTag --repo $ReleaseRepository --json isDraft,body,assets 2>&1
    Assert-CommandSucceeded "read Draft Release $($Identity.ReleaseTag)" $LASTEXITCODE
    $draft = ($json | Out-String) | ConvertFrom-Json
    if (-not $draft.isDraft -and -not $AllowPublished) {
        throw "GitHub release $($Identity.ReleaseTag) is not a draft"
    }
    return $draft
}

function Assert-DraftSource {
    param($Draft, [string]$SourceTag, [string]$Commit, [string]$TagObject = '')
    $body = [string]$Draft.body
    if (-not $body.Contains($Commit) -or -not $body.Contains($SourceTag)) {
        throw 'Draft Release body does not match the remote source tag commit'
    }
    if ($TagObject.Trim() -and -not $body.Contains($TagObject.Trim().ToLowerInvariant())) {
        throw 'Draft Release body does not match the remote annotated tag object'
    }
}

function Get-DraftAssetMetadata {
    param(
        [string]$Name,
        [string]$Path,
        [string]$Architecture,
        [string]$SigningEvidencePath = ''
    )
    $item = Get-Item -LiteralPath $Path
    $metadata = [ordered]@{
        Name = $Name
        Path = $item.FullName
        Size = $item.Length
        SHA256 = (Get-FileHash -LiteralPath $item.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
        Architecture = $Architecture
    }
    if ($SigningEvidencePath) {
        $records = @(Get-Content -Raw -LiteralPath $SigningEvidencePath | ConvertFrom-Json)
        if ($records.Count -ne 3 -or @($records.layer | Sort-Object -Unique).Count -ne 3) {
            throw "Signing evidence must contain exactly three unique layers: $SigningEvidencePath"
        }
        $installer = @($records | Where-Object layer -eq 'installer')
        if ($installer.Count -ne 1 -or [string]$installer[0].sha256 -cne $metadata.SHA256) {
            throw "Signing evidence installer hash does not match $Name"
        }
        $subjects = @($records.signer_subject | Sort-Object -Unique)
        if ($subjects.Count -ne 1 -or -not [string]$subjects[0]) {
            throw "Signing evidence has inconsistent signer subjects: $SigningEvidencePath"
        }
        $metadata.SigningEvidenceSHA256 = Get-LowerSHA256 $SigningEvidencePath
        $metadata.SignerSubject = [string]$subjects[0]
    }
    return [pscustomobject]$metadata
}

function Get-DraftAssetMetadataLine {
    param($Metadata)
    $line = "- ``$($Metadata.Name)``: size=$($Metadata.Size) sha256=``$($Metadata.SHA256)`` architecture=$($Metadata.Architecture)"
    if ($Metadata.PSObject.Properties.Name -contains 'SigningEvidenceSHA256') {
        $line += " authenticode=valid timestamp=rfc3161 signer=``$($Metadata.SignerSubject)`` signing_evidence_sha256=``$($Metadata.SigningEvidenceSHA256)``"
    }
    return $line
}

function Assert-DraftAssetMetadata {
    param($Draft, [object[]]$Metadata)
    $body = ([string]$Draft.body).Replace("`r`n", "`n")
    foreach ($asset in $Metadata) {
        $expected = Get-DraftAssetMetadataLine $asset
        if (-not $body.Contains($expected)) {
            throw "Draft Release is missing matching verified provenance for $($asset.Name)"
        }
    }
}

function Set-DraftAssetMetadata {
    param($Identity, $Draft, [object[]]$Metadata)
    $body = ([string]$Draft.body).Replace("`r`n", "`n").Trim()
    if (-not $body.Contains('## Verified primary assets')) {
        $notesMarker = "`n## 更新说明"
        $notesIndex = $body.IndexOf($notesMarker, [System.StringComparison]::Ordinal)
        if ($notesIndex -lt 0) { throw 'Draft Release body has no update notes section' }
        $body = $body.Insert($notesIndex, "`n## Verified primary assets`n")
    }
    $body = $body.Replace("`n(Pending manual builds.)`n", "`n")
    foreach ($asset in $Metadata) {
        $line = Get-DraftAssetMetadataLine $asset
        $prefix = "- ``$($asset.Name)``:"
        $existing = @($body -split "`n" | Where-Object { $_.StartsWith($prefix, [System.StringComparison]::Ordinal) })
        if ($existing.Count -gt 1) { throw "Draft Release has duplicate provenance for $($asset.Name)" }
        if ($existing.Count -eq 1 -and $existing[0] -cne $line) {
            throw "Draft Release already has different provenance for $($asset.Name)"
        }
        if ($existing.Count -eq 0) {
            $marker = "`n## 更新说明"
            $markerIndex = $body.IndexOf($marker, [System.StringComparison]::Ordinal)
            if ($markerIndex -lt 0) { throw 'Draft Release body has no update notes section' }
            $body = $body.Insert($markerIndex, "`n$line`n")
        }
    }
    $temporary = Join-Path ([System.IO.Path]::GetTempPath()) ("kwu-release-body-" + [guid]::NewGuid().ToString('N') + '.md')
    try {
        [System.IO.File]::WriteAllText($temporary, $body + "`n", [System.Text.UTF8Encoding]::new($false))
        $null = & gh release edit $Identity.ReleaseTag --repo $ReleaseRepository --notes-file $temporary 2>&1
        Assert-CommandSucceeded "record verified asset provenance for $($Identity.ReleaseTag)" $LASTEXITCODE
    }
    finally {
        Remove-Item -LiteralPath $temporary -Force -ErrorAction SilentlyContinue
    }
}

$script:GitHubAssetDownloader = {
    param([string]$Tag, [string]$Name, [string]$Directory)
    $null = & gh release download $Tag --repo $ReleaseRepository --pattern $Name --dir $Directory 2>&1
    return $LASTEXITCODE
}

$script:GitHubAssetUploader = {
    param([string]$Tag, [string]$Path)
    $null = & gh release upload $Tag $Path --repo $ReleaseRepository 2>&1
    return $LASTEXITCODE
}

function Publish-GitHubDraftAsset {
    param($Identity, $Draft, [string]$Path)
    $name = Split-Path $Path -Leaf
    $existing = @($Draft.assets | Where-Object { [string]$_.name -eq $name })
    if ($existing.Count -gt 1) { throw "Draft contains duplicate asset entries: $name" }
    if ($existing.Count -eq 0) {
        $exitCode = & $script:GitHubAssetUploader -Tag $Identity.ReleaseTag -Path $Path
        Assert-CommandSucceeded "upload Draft asset $name" $exitCode
        return
    }

    $temporaryDirectory = Join-Path ([System.IO.Path]::GetTempPath()) ("kwu-existing-draft-" + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $temporaryDirectory | Out-Null
    try {
        $exitCode = & $script:GitHubAssetDownloader -Tag $Identity.ReleaseTag -Name $name -Directory $temporaryDirectory
        Assert-CommandSucceeded "download existing Draft asset $name" $exitCode
        $downloaded = Join-Path $temporaryDirectory $name
        if (-not (Test-Path -LiteralPath $downloaded -PathType Leaf)) {
            throw "Existing Draft asset download is missing: $name"
        }
        if ((Get-LowerSHA256 $downloaded) -cne (Get-LowerSHA256 $Path)) {
            throw "Existing Draft asset differs from local output; refusing to overwrite: $name"
        }
        Write-Host "Reused matching Draft asset: $name"
    }
    finally {
        Remove-Item -LiteralPath $temporaryDirectory -Recurse -Force -ErrorAction SilentlyContinue
    }
}

function Assert-PrimaryDraftAssetsOnly {
    param($Draft)
    $allowed = @(
        'Clouvia-windows-amd64-installer.exe',
        'Clouvia-windows-arm64-installer.exe',
        'Clouvia-darwin-universal.dmg'
    )
    foreach ($asset in @($Draft.assets)) {
        if ($allowed -notcontains [string]$asset.name) {
            throw "Draft contains an unexpected pre-finalization asset: $($asset.name)"
        }
    }
}

function Wait-SelectedMacDraftAsset {
    param($Identity, $Source, [int]$TimeoutSeconds = 3600, [int]$IntervalSeconds = 15)
    $name = $script:ReleaseTargetAssets['macos-universal']
    $tagObject = if ($Source.Tag -eq $Identity.SourceTag) { $Source.ObjectSHA } else { '' }
    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    while ($true) {
        Assert-RemoteSourceTagUnchanged $Source
        $draft = Get-DraftRelease $Identity -AllowPublished
        Assert-DraftSource $draft $Source.Tag $Source.Commit $tagObject
        Assert-ManualTakeoverProvenance $draft $Identity $Source
        if ($draft.isDraft) { Assert-PrimaryDraftAssetsOnly $draft } else { Assert-FinalizeDraftAssetsOnly $draft }
        $assets = @($draft.assets | Where-Object { [string]$_.name -eq $name })
        if ($assets.Count -gt 1) { throw "Draft contains duplicate asset entries: $name" }
        if ($assets.Count -eq 1) {
            $prefix = "- ``$name``:"
            $lines = @(([string]$draft.body).Replace("`r`n", "`n") -split "`n" |
                Where-Object { $_.StartsWith($prefix, [StringComparison]::Ordinal) })
            if ($lines.Count -gt 1) { throw "Draft contains duplicate provenance for $name" }
            if ($lines.Count -eq 1) {
                $temporaryDirectory = Join-Path ([IO.Path]::GetTempPath()) ("kwu-mac-draft-" + [guid]::NewGuid().ToString('N'))
                New-Item -ItemType Directory -Path $temporaryDirectory | Out-Null
                try {
                    $exitCode = & $script:GitHubAssetDownloader -Tag $Identity.ReleaseTag -Name $name -Directory $temporaryDirectory
                    Assert-CommandSucceeded "download selected macOS Draft asset $name" $exitCode
                    $path = Join-Path $temporaryDirectory $name
                    $metadata = Get-DraftAssetMetadata $name $path 'x86_64+arm64 payload'
                    Assert-DraftAssetMetadata $draft @($metadata)
                    Write-Host "Verified selected macOS Draft asset before Windows upload: $name"
                    return $draft
                }
                finally { Remove-Item -LiteralPath $temporaryDirectory -Recurse -Force -ErrorAction SilentlyContinue }
            }
        }
        if ([DateTime]::UtcNow -ge $deadline) {
            throw "Timed out waiting for verified selected macOS Draft asset $name after $TimeoutSeconds seconds"
        }
        if ($IntervalSeconds -gt 0) { Start-Sleep -Seconds $IntervalSeconds }
    }
}

function Assert-RequiredPrimaryAssets {
    param($Draft)
    $names = @($Draft.assets | ForEach-Object { [string]$_.name })
    foreach ($required in @(Resolve-LocalReleaseTargets | ForEach-Object { $script:ReleaseTargetAssets[$_] })) {
        if ($names -notcontains $required) { throw "Draft is missing required primary asset: $required" }
    }
}

function Assert-FinalDraftAssets {
    param($Draft)
    $names = @($Draft.assets | ForEach-Object { [string]$_.name })
    $expected = @()
    foreach ($target in @(Resolve-LocalReleaseTargets)) {
        $asset = $script:ReleaseTargetAssets[$target]
        $expected += @($asset, "$asset.minisig", "latest-$target.json")
    }
    if ($Bootstrap) { $expected += 'latest.json' }
    foreach ($required in $expected) {
        if ($names -notcontains $required) { throw "Final Draft is missing asset: $required" }
    }
}

function Assert-FinalizeDraftAssetsOnly {
    param($Draft)
    $allowed = @(
        'Clouvia-windows-amd64-installer.exe',
        'Clouvia-windows-arm64-installer.exe',
        'Clouvia-darwin-universal.dmg',
        'Clouvia-windows-amd64-installer.exe.minisig',
        'Clouvia-windows-arm64-installer.exe.minisig',
        'Clouvia-darwin-universal.dmg.minisig',
        'latest.json',
        'latest-windows-amd64.json',
        'latest-windows-arm64.json',
        'latest-macos-universal.json'
    )
    $names = @($Draft.assets | ForEach-Object { [string]$_.name })
    if (@($names | Select-Object -Unique).Count -ne $names.Count) {
        throw 'Release contains duplicate asset names'
    }
    foreach ($asset in @($Draft.assets)) {
        if ($allowed -notcontains [string]$asset.name) {
            throw "Draft contains an unexpected asset: $($asset.name)"
        }
    }
}

function Assert-WindowsInstallerArchitecture {
    param(
        [string]$Path,
        [ValidateSet('amd64', 'arm64')][string]$Architecture,
        [string]$Version
    )
    $architectureVerifier = Join-Path $PSScriptRoot 'assert-windows-release-architecture.ps1'
    & $architectureVerifier -Architecture $Architecture -InstallerPath $Path `
        -ExpectedVersion $Version -ExpectedProductName '云智企办' -ExpectedCompanyName 'Clouvia' `
        -RequireSignature -ExpectedSignerThumbprint $env:WINDOWS_SIGN_CERT_THUMBPRINT `
        -ExpectedSignerSubject $env:WINDOWS_SIGN_CERT_SUBJECT -SignToolPath $env:WINDOWS_SIGNTOOL_PATH
}

function Get-ReadyWindowsStaging {
    param($Identity, $Source, [string]$OutputDirectory)
    $amd64Output = Join-Path $OutputDirectory 'Clouvia-windows-amd64-installer.exe'
    $arm64Output = Join-Path $OutputDirectory 'Clouvia-windows-arm64-installer.exe'
    $evidenceDirectory = Join-Path $OutputDirectory 'signing-evidence'
    $amd64Evidence = Join-Path $evidenceDirectory 'windows-amd64.json'
    $arm64Evidence = Join-Path $evidenceDirectory 'windows-arm64.json'
    $receiptPath = Join-Path $OutputDirectory 'windows-ready.json'
    $stagingPaths = @($amd64Output, $arm64Output, $amd64Evidence, $arm64Evidence)

    if (-not (Test-Path -LiteralPath $receiptPath -PathType Leaf)) {
        if (@($stagingPaths | Where-Object { Test-Path -LiteralPath $_ }).Count -ne 0) {
            throw "Incomplete Windows staging exists without windows-ready.json; preserve it for diagnosis, then remove $OutputDirectory before rebuilding"
        }
        return $null
    }
    if (@($stagingPaths | Where-Object { -not (Test-Path -LiteralPath $_ -PathType Leaf) }).Count -ne 0) {
        throw 'windows-ready.json exists but one or more immutable staging files are missing'
    }
    $receipt = Get-Content -Raw -LiteralPath $receiptPath | ConvertFrom-Json
    if ([string]$receipt.version -cne $Identity.Version -or
        [string]$receipt.source_tag -cne $Source.Tag -or
        [string]$receipt.source_tag_object -cne $Source.ObjectSHA -or
        [string]$receipt.source_commit -cne $Source.Commit) {
        throw 'Existing Windows staging receipt belongs to a different frozen release source'
    }
    foreach ($binding in @(
        @{ Path = $amd64Output; Expected = [string]$receipt.amd64_sha256 },
        @{ Path = $arm64Output; Expected = [string]$receipt.arm64_sha256 },
        @{ Path = $amd64Evidence; Expected = [string]$receipt.amd64_evidence_sha256 },
        @{ Path = $arm64Evidence; Expected = [string]$receipt.arm64_evidence_sha256 }
    )) {
        if ((Get-LowerSHA256 $binding.Path) -cne $binding.Expected) {
            throw "Existing Windows staging hash differs from windows-ready.json: $($binding.Path)"
        }
    }
    Assert-WindowsInstallerArchitecture $amd64Output 'amd64' $Identity.Version
    Assert-WindowsInstallerArchitecture $arm64Output 'arm64' $Identity.Version
    $metadata = @(
        (Get-DraftAssetMetadata 'Clouvia-windows-amd64-installer.exe' $amd64Output 'amd64 payload' $amd64Evidence),
        (Get-DraftAssetMetadata 'Clouvia-windows-arm64-installer.exe' $arm64Output 'arm64 payload' $arm64Evidence)
    )
    Write-Host "Reusing immutable signed Windows staging for $($Identity.ReleaseTag)"
    return [pscustomobject]@{
        AMD64 = $amd64Output
        ARM64 = $arm64Output
        AMD64Evidence = $amd64Evidence
        ARM64Evidence = $arm64Evidence
        Metadata = $metadata
        Receipt = $receiptPath
    }
}

function Invoke-BuildWindows {
    if ($env:OS -ne 'Windows_NT') {
        throw 'BuildWindows must run on Windows'
    }
    $windowsTargets = @(Resolve-LocalReleaseTargets | Where-Object { $_ -like 'windows-*' })
    if ($windowsTargets.Count -eq 0) { throw 'BuildWindows requires at least one Windows target' }
    $identity = Get-ReleaseIdentity $Version
    $source = Resolve-RemoteSourceTag $identity
    if (-not $source) {
        throw "Remote source tag does not exist: $($identity.SourceTag) (legacy fallback: $($identity.LegacySourceTag))"
    }
    $commit = $source.Commit
    $draft = Get-DraftRelease $identity -AllowPublished
    $tagObject = if ($source.Tag -eq $identity.SourceTag) { $source.ObjectSHA } else { '' }
    Assert-DraftSource $draft $source.Tag $commit $tagObject
    Assert-ManualTakeoverProvenance $draft $identity $source
    if ($draft.isDraft) { Assert-PrimaryDraftAssetsOnly $draft } else { Assert-FinalizeDraftAssetsOnly $draft }
    $bash = Get-BashExecutable
    if ($DryRun) {
        Write-Host "DryRun BuildWindows targets: $($windowsTargets -join ',')"
        Write-Host "DryRun BuildWindows: $($source.Tag) ($commit) via $bash"
        Write-Host 'DryRun BuildWindows: no build, signature or upload was performed'
        return
    }
    $workDirectory = Get-ReleaseWorkDirectory $identity
    $sourceDirectory = Join-Path $workDirectory 'windows-source'
    $outputDirectory = Join-Path $workDirectory 'windows'
    New-Item -ItemType Directory -Path $workDirectory, $outputDirectory -Force | Out-Null
    $staging = if ($windowsTargets.Count -eq 2) { Get-ReadyWindowsStaging $identity $source $outputDirectory } else { $null }
    if (-not $staging) {
        $nsisBin = Get-NSISBinDirectory
        $authenticode = Join-Path $PSScriptRoot 'desktop-windows-authenticode.ps1'
        & $authenticode -Action Preflight
        if (Test-Path -LiteralPath $sourceDirectory) {
            $existingCommit = (& git -C $sourceDirectory rev-parse HEAD 2>&1 | Out-String).Trim()
            Assert-CommandSucceeded 'inspect existing Windows build worktree' $LASTEXITCODE
            if ($existingCommit -ne $commit) {
                throw "Existing Windows build worktree is $existingCommit, expected $commit"
            }
        }
        else {
            $null = & git -C $RepositoryRoot worktree add --detach $sourceDirectory $commit 2>&1
            Assert-CommandSucceeded 'create frozen Windows build worktree' $LASTEXITCODE
        }
        Assert-BuildWorktreePristine $sourceDirectory

        $evidenceDirectory = Join-Path $outputDirectory 'signing-evidence'
        New-Item -ItemType Directory -Path $evidenceDirectory | Out-Null
        $amd64Evidence = Join-Path $evidenceDirectory 'windows-amd64.json'
        $arm64Evidence = Join-Path $evidenceDirectory 'windows-arm64.json'
        $previousSigning = $env:KWU_WINDOWS_RELEASE_SIGNING
        $previousEvidence = $env:KWU_WINDOWS_SIGN_EVIDENCE_PATH
        $previousPath = $env:PATH
        Push-Location $sourceDirectory
        try {
            $env:KWU_WINDOWS_RELEASE_SIGNING = '1'
            $env:PATH = "$nsisBin;$previousPath"
            if ($windowsTargets -contains 'windows-amd64') {
                $env:KWU_WINDOWS_SIGN_EVIDENCE_PATH = $amd64Evidence
                $null = & $bash 'scripts/desktop-build.sh' 'windows/amd64' $identity.Version 'stable' 2>&1
                Assert-CommandSucceeded 'build Windows amd64 installer' $LASTEXITCODE
            }
            if ($windowsTargets -contains 'windows-arm64') {
                $env:KWU_WINDOWS_SIGN_EVIDENCE_PATH = $arm64Evidence
                $buildMode = if ($windowsTargets.Count -eq 2) { 'reuse-frontend' } else { 'full' }
                $null = & $bash 'scripts/desktop-build.sh' 'windows/arm64' $identity.Version 'stable' $buildMode 2>&1
                Assert-CommandSucceeded 'build Windows arm64 installer' $LASTEXITCODE
            }
        }
        finally {
            Pop-Location
            $env:KWU_WINDOWS_RELEASE_SIGNING = $previousSigning
            $env:KWU_WINDOWS_SIGN_EVIDENCE_PATH = $previousEvidence
            $env:PATH = $previousPath
        }
        Assert-BuildWorktreeTrackedClean $sourceDirectory

        $amd64 = Join-Path $sourceDirectory 'dist\Clouvia-windows-amd64-installer.exe'
        $arm64 = Join-Path $sourceDirectory 'dist\Clouvia-windows-arm64-installer.exe'
        $amd64Output = Join-Path $outputDirectory (Split-Path $amd64 -Leaf)
        $arm64Output = Join-Path $outputDirectory (Split-Path $arm64 -Leaf)
        $metadata = @()
        if ($windowsTargets -contains 'windows-amd64') {
            Assert-WindowsInstallerArchitecture $amd64 'amd64' $identity.Version
            Copy-Item -LiteralPath $amd64 -Destination $outputDirectory
            $metadata += Get-DraftAssetMetadata 'Clouvia-windows-amd64-installer.exe' $amd64Output 'amd64 payload' $amd64Evidence
        }
        if ($windowsTargets -contains 'windows-arm64') {
            Assert-WindowsInstallerArchitecture $arm64 'arm64' $identity.Version
            Copy-Item -LiteralPath $arm64 -Destination $outputDirectory
            $metadata += Get-DraftAssetMetadata 'Clouvia-windows-arm64-installer.exe' $arm64Output 'arm64 payload' $arm64Evidence
        }
        if ($windowsTargets.Count -eq 1) {
            $staging = [pscustomobject]@{
                AMD64 = if ($windowsTargets -contains 'windows-amd64') { $amd64Output } else { $null }
                ARM64 = if ($windowsTargets -contains 'windows-arm64') { $arm64Output } else { $null }
                Metadata = $metadata
            }
        }
        else {
        $receiptPath = Join-Path $outputDirectory 'windows-ready.json'
        $receipt = [ordered]@{
            schema_version = 1
            version = $identity.Version
            source_tag = $source.Tag
            source_tag_object = $source.ObjectSHA
            source_commit = $source.Commit
            amd64_sha256 = Get-LowerSHA256 $amd64Output
            arm64_sha256 = Get-LowerSHA256 $arm64Output
            amd64_evidence_sha256 = Get-LowerSHA256 $amd64Evidence
            arm64_evidence_sha256 = Get-LowerSHA256 $arm64Evidence
        }
        [IO.File]::WriteAllText(
            $receiptPath,
            (($receipt | ConvertTo-Json) + [Environment]::NewLine),
            [Text.UTF8Encoding]::new($false)
        )
        $staging = Get-ReadyWindowsStaging $identity $source $outputDirectory
        }
    }

    $uploadDraft = if (@(Resolve-LocalReleaseTargets) -contains 'macos-universal') {
        Wait-SelectedMacDraftAsset $identity $source
    }
    else {
        Get-DraftRelease $identity -AllowPublished
    }
    Assert-DraftSource $uploadDraft $source.Tag $commit $tagObject
    Assert-ManualTakeoverProvenance $uploadDraft $identity $source
    if ($uploadDraft.isDraft) { Assert-PrimaryDraftAssetsOnly $uploadDraft } else { Assert-FinalizeDraftAssetsOnly $uploadDraft }
    foreach ($path in @($staging.AMD64, $staging.ARM64) | Where-Object { $_ }) {
        Assert-RemoteSourceTagUnchanged $source
        Publish-GitHubDraftAsset $identity $uploadDraft $path
    }
    $updatedDraft = Get-DraftRelease $identity -AllowPublished
    $assetNames = @($updatedDraft.assets | ForEach-Object { [string]$_.name })
    foreach ($required in @($windowsTargets | ForEach-Object { $script:ReleaseTargetAssets[$_] })) {
        if ($assetNames -notcontains $required) { throw "Draft is missing uploaded asset: $required" }
    }
    if ($updatedDraft.isDraft) { Assert-PrimaryDraftAssetsOnly $updatedDraft }
    else { Assert-FinalizeDraftAssetsOnly $updatedDraft }
    Assert-RemoteSourceTagUnchanged $source
    Set-DraftAssetMetadata $identity $updatedDraft $staging.Metadata
    $updatedDraft = Get-DraftRelease $identity -AllowPublished
    Assert-DraftAssetMetadata $updatedDraft $staging.Metadata
    Write-Host "Built and staged Windows installers for $($identity.ReleaseTag)"
}

$script:OssPutInvoker = {
    param([object[]]$CommandArguments)
    $null = & $OssutilPath @CommandArguments 2>&1
    return $LASTEXITCODE
}

$script:PublicObjectDownloader = {
    param([string]$URL, [string]$Destination)
    Invoke-WebRequest -Uri $URL -OutFile $Destination -UseBasicParsing
}

$script:StablePointerDownloader = {
    param([string]$URL, [string]$Destination)
    Invoke-WebRequest -Uri $URL -OutFile $Destination -UseBasicParsing -TimeoutSec 30 `
        -Headers @{ 'Cache-Control' = 'no-cache' }
}

function Get-LowerSHA256 {
    param([string]$Path)
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Get-OssContentType {
    param([string]$Name)
    if ($Name.EndsWith('.json')) { return 'application/json' }
    if ($Name.EndsWith('.minisig')) { return 'text/plain; charset=utf-8' }
    return 'application/octet-stream'
}

function Invoke-OssPutObject {
    param(
        [string]$ObjectKey,
        [string]$FilePath,
        [bool]$ForbidOverwrite,
        [string]$CacheControl
    )
    $fullPath = [System.IO.Path]::GetFullPath($FilePath).Replace('\', '/')
    $arguments = @(
        'api', 'put-object',
        '--bucket', $OssBucket,
        '--key', $ObjectKey,
        '--body', "file://$fullPath",
        '--content-type', (Get-OssContentType $ObjectKey),
        '--cache-control', $CacheControl,
        '--config-file', $OssutilConfigFile,
        '--profile', $OssutilProfile,
        '--quiet'
    )
    if ($ForbidOverwrite) { $arguments += '--forbid-overwrite' }
    return & $script:OssPutInvoker -CommandArguments $arguments
}

function Test-PublicObjectMatches {
    param([string]$ObjectKey, [string]$FilePath)
    $temporary = [System.IO.Path]::GetTempFileName()
    try {
        $url = $OssPublicBaseURL.TrimEnd('/') + '/' + $ObjectKey
        & $script:PublicObjectDownloader $url $temporary | Out-Null
        return (Get-LowerSHA256 $temporary) -eq (Get-LowerSHA256 $FilePath)
    }
    catch {
        return $false
    }
    finally {
        Remove-Item -LiteralPath $temporary -Force -ErrorAction SilentlyContinue
    }
}

function Get-LocalPublicObjectState {
    param([string]$ObjectKey, [string]$FilePath)
    $temporary = [IO.Path]::GetTempFileName()
    try {
        try {
            & $script:PublicObjectDownloader `
                -URL ($OssPublicBaseURL.TrimEnd('/') + '/' + $ObjectKey) -Destination $temporary
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

function Assert-LocalGitHubAssetCanAppend {
    param($Identity, $Release, [string]$Name, [string]$Path)
    $existing = @($Release.assets | Where-Object { [string]$_.name -ceq $Name })
    if ($existing.Count -gt 1) { throw "Release contains duplicate asset names: $Name" }
    if ($existing.Count -eq 0) { return }
    $temporary = Join-Path ([IO.Path]::GetTempPath()) ("kwu-local-preflight-" + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $temporary | Out-Null
    try {
        $exitCode = & $script:GitHubAssetDownloader -Tag $Identity.ReleaseTag -Name $Name -Directory $temporary
        Assert-CommandSucceeded "preflight existing GitHub asset $Name" $exitCode
        if ((Get-LowerSHA256 (Join-Path $temporary $Name)) -ne (Get-LowerSHA256 $Path)) {
            throw "Existing GitHub asset differs from selected content: $Name"
        }
    }
    finally { Remove-Item -LiteralPath $temporary -Recurse -Force -ErrorAction SilentlyContinue }
}

function Assert-LocalZeroWritePublicationPreflight {
    param($Identity, $Release, [string[]]$PrimaryNames, [hashtable]$GitHubManifests,
        [hashtable]$OssManifests, [string]$Incoming, [string]$BootstrapGitHubManifest = '',
        [string]$BootstrapOssManifest = '', [string]$NotesPath = '', [string]$CloudPublisherPath = '')
    $prefix = "releases/$($Identity.ReleaseTag)"
    $objects = @{}
    foreach ($target in @(Resolve-LocalReleaseTargets)) {
        $name = $script:ReleaseTargetAssets[$target]
        Assert-LocalGitHubAssetCanAppend $Identity $Release $name (Join-Path $Incoming $name)
        Assert-LocalGitHubAssetCanAppend $Identity $Release "$name.minisig" (Join-Path $Incoming "$name.minisig")
        Assert-LocalGitHubAssetCanAppend $Identity $Release "latest-$target.json" $GitHubManifests[$target]
        $objects["$prefix/$target/$name"] = Join-Path $Incoming $name
        $objects["$prefix/$target/$name.minisig"] = Join-Path $Incoming "$name.minisig"
        $objects["$prefix/$target/latest.json"] = $OssManifests[$target]
        if ($Bootstrap) {
            $objects["$prefix/$name"] = Join-Path $Incoming $name
            $objects["$prefix/$name.minisig"] = Join-Path $Incoming "$name.minisig"
        }
    }
    if ($BootstrapGitHubManifest) {
        Assert-LocalGitHubAssetCanAppend $Identity $Release 'latest.json' $BootstrapGitHubManifest
        $objects["$prefix/latest.json"] = $BootstrapOssManifest
    }
    Assert-LocalPointerPublicationPreflight `
        $Identity @(Resolve-LocalReleaseTargets) $OssManifests $BootstrapOssManifest
    foreach ($entry in $objects.GetEnumerator()) {
        if ((Get-LocalPublicObjectState $entry.Key $entry.Value) -eq 'conflict') {
            throw "OSS immutable object exists with different content: $($entry.Key)"
        }
    }
    Invoke-LocalCloudPublisherCommand $CloudPublisherPath @('-preflight', '-base-url', $CloudBaseURL) `
        'cloud release API zero-write preflight'
    Invoke-LocalCloudPublisherPhase $CloudPublisherPath @(Resolve-LocalReleaseTargets) `
        $OssManifests $NotesPath -Preflight
    Write-Host "Zero-write local publication preflight passed for $(@(Resolve-LocalReleaseTargets) -join ',')"
}

function Publish-OssImmutableObject {
    param([string]$ObjectKey, [string]$FilePath)
    if (-not (Test-Path -LiteralPath $FilePath -PathType Leaf)) {
        throw "OSS upload source does not exist: $FilePath"
    }
    $exitCode = Invoke-OssPutObject $ObjectKey $FilePath $true 'public,max-age=31536000,immutable'
    if ($exitCode -eq 0) {
        Write-Host "Uploaded immutable OSS object: $ObjectKey"
        return
    }
    if (Test-PublicObjectMatches $ObjectKey $FilePath) {
        Write-Host "Reused matching immutable OSS object: $ObjectKey"
        return
    }
    throw "Immutable OSS object upload failed or existing object has a different SHA256: $ObjectKey"
}

function Publish-OssStablePointer {
    param([string]$ManifestPath, $Identity, [string]$Target = '')
    Assert-PublicStablePointerCanAdvance $Identity $ManifestPath $Target
    $objectKey = if ($Target) { "channels/stable/$Target/latest.json" } else { 'channels/stable/latest.json' }
    $exitCode = Invoke-OssPutObject $objectKey $ManifestPath $false 'no-cache'
    if ($exitCode -ne 0) {
        throw "Failed to update OSS stable pointer: $objectKey"
    }
    if (-not (Test-PublicObjectMatches $objectKey $ManifestPath)) {
        throw "OSS stable pointer verification failed: $objectKey"
    }
    Write-Host "Updated and verified OSS stable pointer: $objectKey"
}

function Get-DesktopVersionCode {
    param([string]$RawVersion)
    $parsed = Get-ReleaseIdentity $RawVersion
    $segments = @($parsed.VersionName.Split('.') | ForEach-Object { [int64]$_ })
    return $segments[0] * 1000000L + $segments[1] * 1000L + $segments[2]
}

function Assert-StablePointerTransition {
    param($TargetIdentity, [string]$CandidateManifestPath, [string]$CurrentManifestPath)
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
        if ((Get-LowerSHA256 $CurrentManifestPath) -ne (Get-LowerSHA256 $CandidateManifestPath)) {
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
        $url = $OssPublicBaseURL.TrimEnd('/') + "/$pointerPath`?kwu-release-check=$cacheBuster"
        try {
            & $script:StablePointerDownloader -URL $url -Destination $temporary
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

function Assert-LocalPointerPublicationPreflight {
    param($Identity, [string[]]$Targets, [hashtable]$OssManifests, [string]$BootstrapOssManifest = '')
    foreach ($target in $Targets) {
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

function Get-ManualCloudPublicationSection {
    return "<!-- kwu-desktop-cloud-publication -->`n## Cloud publication`n`n- Mode: required"
}

function Assert-ManualCloudModeCompatible {
    param($Release, $Identity)
    $body = ([string]$Release.body).Replace("`r`n", "`n").Trim()
    $marker = '<!-- kwu-desktop-cloud-publication -->'
    $section = Get-ManualCloudPublicationSection
    if ($body.Contains($marker)) {
        if (-not $body.Contains($section)) { throw 'Release contains a different frozen cloud publication decision' }
        return $true
    }
    if (-not $Release.isDraft) { throw 'Published Release is missing its frozen cloud publication decision' }
    return $false
}

function Set-ManualCloudModeInReleaseBody {
    param($Release, $Identity)
    if (Assert-ManualCloudModeCompatible $Release $Identity) { return }
    $body = ([string]$Release.body).Replace("`r`n", "`n").Trim()
    $section = Get-ManualCloudPublicationSection
    $temporary = Join-Path ([IO.Path]::GetTempPath()) ("kwu-cloud-mode-" + [guid]::NewGuid().ToString('N') + '.md')
    try {
        [IO.File]::WriteAllText($temporary, "$body`n`n$section`n", [Text.UTF8Encoding]::new($false))
        $null = & gh release edit $Identity.ReleaseTag --repo $ReleaseRepository --notes-file $temporary 2>&1
        Assert-CommandSucceeded "record cloud publication decision for $($Identity.ReleaseTag)" $LASTEXITCODE
    }
    finally { Remove-Item -LiteralPath $temporary -Force -ErrorAction SilentlyContinue }
}

function Set-LocalGitHubLatestForBootstrap {
    param($Identity)
    $null = & gh release edit $Identity.ReleaseTag --repo $ReleaseRepository --latest 2>&1
    Assert-CommandSucceeded "set bootstrap GitHub Latest to $($Identity.ReleaseTag)" $LASTEXITCODE
    $latestJSON = & gh release view --repo $ReleaseRepository --json tagName 2>&1
    Assert-CommandSucceeded 'verify bootstrap GitHub Latest' $LASTEXITCODE
    if ([string](($latestJSON | Out-String) | ConvertFrom-Json).tagName -cne $Identity.ReleaseTag) {
        throw "GitHub Latest verification failed for bootstrap $($Identity.ReleaseTag)"
    }
}

function Invoke-DesktopGoCommand {
    param([string[]]$Arguments, [string]$Description)
    Push-Location (Join-Path $RepositoryRoot 'desktop')
    try {
        $null = & go @Arguments 2>&1
        Assert-CommandSucceeded $Description $LASTEXITCODE
    }
    finally {
        Pop-Location
    }
}

function Invoke-LocalCloudPublisherCommand {
    param([string]$PublisherPath, [string[]]$Arguments, [string]$Description)
    if (-not (Test-Path -LiteralPath $PublisherPath -PathType Leaf)) {
        throw "Cloud publisher executable does not exist: $PublisherPath"
    }
    $null = & $PublisherPath @Arguments 2>&1
    Assert-CommandSucceeded $Description $LASTEXITCODE
}

$script:LocalCloudPhaseExecutor = {
    param([object[]]$Requests, [bool]$Preflight)
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
    } -ThrottleLimit 3)
}

function Invoke-LocalCloudPublisherPhase {
    param(
        [string]$PublisherPath,
        [string[]]$Targets,
        [hashtable]$OssManifests,
        [string]$NotesPath,
        [switch]$Preflight
    )
    if (-not (Test-Path -LiteralPath $PublisherPath -PathType Leaf)) {
        throw "Cloud publisher executable does not exist: $PublisherPath"
    }
    $requests = @($Targets | ForEach-Object {
        [pscustomobject]@{
            Publisher = [IO.Path]::GetFullPath($PublisherPath)
            Manifest = [IO.Path]::GetFullPath($OssManifests[$_])
            Notes = [IO.Path]::GetFullPath($NotesPath)
            BaseURL = $CloudBaseURL
            Target = $_
        }
    })
    $results = @(& $script:LocalCloudPhaseExecutor $requests ([bool]$Preflight))
    $phase = if ($Preflight) { 'preflight' } else { 'publication' }
    if ($results.Count -ne $requests.Count) {
        throw "Cloud $phase did not return one result for every selected target"
    }
    $actualTargets = @($results | ForEach-Object { [string]$_.Target })
    if (@($actualTargets | Select-Object -Unique).Count -ne $actualTargets.Count -or
        @($Targets | Where-Object { $actualTargets -notcontains $_ }).Count -ne 0) {
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

function Assert-MacOSDMGArchitecture {
    param([string]$Path, $Draft, [string]$ExpectedVersion)
    $metadata = Get-DraftAssetMetadata 'Clouvia-darwin-universal.dmg' $Path 'x86_64+arm64 payload'
    $runningOnMacOS = [System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform(
        [System.Runtime.InteropServices.OSPlatform]::OSX
    )
    if ($runningOnMacOS) {
        $bash = Get-BashExecutable
        $verifier = Join-Path $PSScriptRoot 'assert-macos-release-architecture.sh'
        $null = & $bash $verifier $Path $ExpectedVersion 2>&1
        Assert-CommandSucceeded 'verify macOS universal DMG payload and ad-hoc codesign structure' $LASTEXITCODE
        return
    }

    # hdiutil/lipo are unavailable on the Windows coordinator. The exact DMG bytes
    # must therefore carry provenance written only after the Mac verifier succeeds.
    Assert-DraftAssetMetadata $Draft @($metadata)
    Write-Host 'Verified macOS DMG through SHA256-bound Mac build provenance (hdiutil/lipo unavailable on this host)'
}

function Assert-LocalFinalizeIntent {
    param($Identity)
    $targets = @(Resolve-LocalReleaseTargets)
    if (-not [System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform(
            [System.Runtime.InteropServices.OSPlatform]::Windows
        )) {
        throw 'Local Finalize is supported only on the fixed Windows release machine'
    }
    if ($Bootstrap -and ($targets -join ',') -cne ($script:ReleaseTargetOrder -join ',')) {
        throw 'Bootstrap publication requires all three desktop targets'
    }
    $expected = "PUBLISH $($Identity.ReleaseTag) TARGETS $($targets -join ',')$(if ($Bootstrap) { ' BOOTSTRAP' })"
    if ($Confirmation -cne $expected) {
        throw "Confirmation must exactly match: $expected"
    }
    $acceptedSHA256 = ''
    if ($targets -contains 'windows-arm64') {
        if ($Arm64AcceptanceConfirmation -cne "ACCEPT $($Identity.ReleaseTag) ARM64") {
            throw "ARM64 acceptance confirmation must exactly match: ACCEPT $($Identity.ReleaseTag) ARM64"
        }
        $acceptedSHA256 = $Arm64AcceptanceSHA256.Trim().ToLowerInvariant()
        if ($acceptedSHA256 -notmatch '^[0-9a-f]{64}$') {
            throw 'Arm64AcceptanceSHA256 must be an exact 64-character SHA256'
        }
    }
    if ($SkipCloud -or $CloudSkipReason.Trim()) {
        throw 'Local Finalize is a full fallback publish and does not allow -SkipCloud or -CloudSkipReason'
    }
    if ($AllowNonMainV3 -or $EmergencyReason.Trim()) {
        throw 'Local Finalize supports only a normal main-v3 release, not an emergency source takeover'
    }
    return $acceptedSHA256
}

function Read-LocalPublicManifestIfExists {
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

function Assert-LocalBootstrapStateTransition {
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

function Assert-LocalBootstrapMayStartOrResume {
    param($Identity)
    $base = $OssPublicBaseURL.TrimEnd('/')
    try { $global = Invoke-RestMethod -Uri "$base/channels/stable/latest.json" -TimeoutSec 30 }
    catch { throw "Global bootstrap manifest is unavailable: $($_.Exception.Message)" }
    $globalTag = (Get-ReleaseIdentity ([string]$global.version)).ReleaseTag
    $globalReleaseJSON = & gh release view $globalTag --repo $ReleaseRepository --json assets 2>&1
    Assert-CommandSucceeded "read frozen global GitHub Release $globalTag" $LASTEXITCODE
    $globalRelease = ($globalReleaseJSON | Out-String) | ConvertFrom-Json
    $globalAssetNames = @($globalRelease.assets | ForEach-Object { [string]$_.name })
    $migrationAssets = @('latest.json') + @(
        $script:ReleaseTargetOrder | ForEach-Object { "latest-$_.json" }
    )
    $globalReleaseHasMigrationAssets = @(
        $migrationAssets | Where-Object { $globalAssetNames -notcontains $_ }
    ).Count -eq 0
    $latestJSON = & gh release view --repo $ReleaseRepository --json tagName 2>&1
    Assert-CommandSucceeded 'read GitHub Latest bootstrap state' $LASTEXITCODE
    $latestTag = [string](($latestJSON | Out-String) | ConvertFrom-Json).tagName
    $targetTags = @()
    foreach ($target in $script:ReleaseTargetOrder) {
        $pointer = Read-LocalPublicManifestIfExists "$base/channels/stable/$target/latest.json" "Bootstrap target pointer $target"
        if ($null -ne $pointer) {
            $targetTags += (Get-ReleaseIdentity ([string]$pointer.version)).ReleaseTag
        }
    }
    Assert-LocalBootstrapStateTransition `
        $Identity.ReleaseTag $globalTag $globalReleaseHasMigrationAssets $latestTag $targetTags
    Write-Host "Bootstrap state permits first migration or exact resume for $($Identity.ReleaseTag)"
}

function Assert-LocalSelectiveMigrationReady {
    if ($Bootstrap) {
        Assert-LocalBootstrapMayStartOrResume (Get-ReleaseIdentity $Version)
        return
    }
    $globalURL = $OssPublicBaseURL.TrimEnd('/') + '/channels/stable/latest.json'
    try { $global = Invoke-RestMethod -Uri $globalURL -TimeoutSec 30 }
    catch { throw "Frozen global migration manifest is unavailable: $($_.Exception.Message)" }
    foreach ($platform in @('windows-amd64', 'windows-arm64', 'darwin-amd64', 'darwin-arm64')) {
        if ($null -eq $global.platforms.PSObject.Properties[$platform]) {
            throw "Frozen migration manifest is missing $platform"
        }
    }
    $migration = Get-ReleaseIdentity ([string]$global.version)
    $temporary = Join-Path ([IO.Path]::GetTempPath()) ("kwu-bootstrap-" + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $temporary | Out-Null
    try {
        $null = & gh release download $migration.ReleaseTag --repo $ReleaseRepository `
            --pattern latest.json --dir $temporary 2>&1
        Assert-CommandSucceeded 'download frozen GitHub migration manifest' $LASTEXITCODE
        try {
            $githubGlobal = Get-Content -Raw -LiteralPath (Join-Path $temporary 'latest.json') | ConvertFrom-Json
        }
        catch { throw "Frozen GitHub migration manifest is invalid: $($_.Exception.Message)" }
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
    $latestJSON = & gh release view --repo $ReleaseRepository --json tagName 2>&1
    Assert-CommandSucceeded 'read frozen GitHub Latest migration tag' $LASTEXITCODE
    if ([string](($latestJSON | Out-String) | ConvertFrom-Json).tagName -cne $migration.ReleaseTag) {
        throw "GitHub Latest no longer matches frozen migration tag $($migration.ReleaseTag)"
    }
    foreach ($target in $script:ReleaseTargetOrder) {
        try {
            $pointer = Invoke-RestMethod -Uri ($OssPublicBaseURL.TrimEnd('/') + "/channels/stable/$target/latest.json") -TimeoutSec 30
        }
        catch { throw "Migration target pointer $target is unavailable: $($_.Exception.Message)" }
        if ((Get-DesktopVersionCode ([string]$pointer.version)) -lt (Get-DesktopVersionCode $migration.Version)) {
            throw "Migration target pointer $target is older than $($migration.Version)"
        }
    }
}

function Invoke-Finalize {
    $identity = Get-ReleaseIdentity $Version
    $acceptedArm64SHA256 = Assert-LocalFinalizeIntent $identity
    $targets = @(Resolve-LocalReleaseTargets)
    $source = Resolve-RemoteSourceTag $identity
    if (-not $source) {
        throw "Remote source tag does not exist: $($identity.SourceTag) (legacy fallback: $($identity.LegacySourceTag))"
    }
    if ($source.Tag -cne $identity.SourceTag) {
        throw "Local Finalize requires the canonical annotated source tag: $($identity.SourceTag)"
    }
    $commit = $source.Commit
    $draft = Get-DraftRelease $identity -AllowPublished
    $tagObject = if ($source.Tag -eq $identity.SourceTag) { $source.ObjectSHA } else { '' }
    Assert-DraftSource $draft $source.Tag $commit $tagObject
    Assert-ManualTakeoverProvenance $draft $identity $source
    $notes = Get-ReleaseNotesAtCommit $identity $commit
    Assert-ReleaseBodyCore $draft $identity $source.Tag $commit $notes $tagObject
    Assert-FinalizeDraftAssetsOnly $draft
    Assert-RequiredPrimaryAssets $draft
    Assert-ManualCloudModeCompatible $draft $identity | Out-Null
    Assert-LocalSelectiveMigrationReady

    $workDirectory = Get-ReleaseWorkDirectory $identity
    New-Item -ItemType Directory -Path $workDirectory -Force | Out-Null
    $cloudPublisher = Join-Path $workDirectory 'publish-cloud-release.exe'
    Invoke-DesktopGoCommand `
        -Arguments @('build', '-o', $cloudPublisher, './cmd/publish-cloud-release') `
        -Description 'prebuild cloud release publisher'
    Invoke-LocalCloudPublisherCommand $cloudPublisher @('-preflight', '-base-url', $CloudBaseURL) `
        'cloud release API preflight'

    $notesPath = Join-Path $workDirectory 'release-notes.md'
    $incoming = Join-Path $workDirectory 'incoming'
    $manifestRoot = Join-Path $workDirectory 'manifests'
    $githubManifestRoot = Join-Path $manifestRoot 'github'
    $ossManifestRoot = Join-Path $manifestRoot 'oss'
    New-Item -ItemType Directory -Path $incoming, $githubManifestRoot, $ossManifestRoot -Force | Out-Null
    if (Test-Path -LiteralPath $notesPath -PathType Leaf) {
        $snapshot = Assert-ReleaseNotesText (Get-Content -Raw -LiteralPath $notesPath)
        if ($snapshot -cne $notes) {
            throw "Existing release notes snapshot differs from $($identity.NotesPath) at source commit $commit"
        }
    }
    else {
        [System.IO.File]::WriteAllText($notesPath, $notes + "`n", [System.Text.UTF8Encoding]::new($false))
    }

    $primaryNames = @($targets | ForEach-Object { $script:ReleaseTargetAssets[$_] })
    foreach ($name in $primaryNames) {
        Remove-Item -LiteralPath (Join-Path $incoming $name) -Force -ErrorAction SilentlyContinue
        $null = & gh release download $identity.ReleaseTag --repo $ReleaseRepository --pattern $name --dir $incoming 2>&1
        Assert-CommandSucceeded "download Draft asset $name" $LASTEXITCODE
        if (-not (Test-Path -LiteralPath (Join-Path $incoming $name) -PathType Leaf)) {
            throw "Downloaded Draft asset is missing: $name"
        }
    }
    $amd64 = Join-Path $incoming 'Clouvia-windows-amd64-installer.exe'
    $arm64 = Join-Path $incoming 'Clouvia-windows-arm64-installer.exe'
    $dmg = Join-Path $incoming 'Clouvia-darwin-universal.dmg'
    $primaryMetadata = @()
    if ($targets -contains 'windows-amd64') {
        Assert-WindowsInstallerArchitecture $amd64 'amd64' $identity.Version
        $primaryMetadata += Get-DraftAssetMetadata 'Clouvia-windows-amd64-installer.exe' $amd64 'amd64 payload'
    }
    if ($targets -contains 'windows-arm64') {
        Assert-WindowsInstallerArchitecture $arm64 'arm64' $identity.Version
        $arm64Metadata = Get-DraftAssetMetadata 'Clouvia-windows-arm64-installer.exe' $arm64 'arm64 payload'
        if ([string]$arm64Metadata.SHA256 -cne $acceptedArm64SHA256) {
            throw 'Downloaded Draft ARM64 installer does not match the explicitly accepted SHA256'
        }
        $primaryMetadata += $arm64Metadata
    }
    if ($targets -contains 'macos-universal') {
        $primaryMetadata += Get-DraftAssetMetadata 'Clouvia-darwin-universal.dmg' $dmg 'x86_64+arm64 payload'
    }
    Assert-DraftAssetMetadata $draft $primaryMetadata
    if ($targets -contains 'macos-universal') { Assert-MacOSDMGArchitecture $dmg $draft $identity.Version }

    $releaseAssetNames = @($draft.assets | ForEach-Object { [string]$_.name })
    $missingSignatureAssets = @()
    foreach ($name in $primaryNames) {
        $signatureName = "$name.minisig"
        $signaturePath = Join-Path $incoming $signatureName
        if ($releaseAssetNames -contains $signatureName) {
            Remove-Item -LiteralPath $signaturePath -Force -ErrorAction SilentlyContinue
            $null = & gh release download $identity.ReleaseTag --repo $ReleaseRepository `
                --pattern $signatureName --dir $incoming 2>&1
            Assert-CommandSucceeded "download existing release signature $signatureName" $LASTEXITCODE
            if (-not (Test-Path -LiteralPath $signaturePath -PathType Leaf)) {
                throw "Downloaded release signature is missing: $signatureName"
            }
        }
        else {
            $missingSignatureAssets += Join-Path $incoming $name
        }
    }

    if ($missingSignatureAssets.Count -gt 0) {
        $previousKeyFile = $env:MINISIGN_PRIVATE_KEY_FILE
        $previousKeyText = $env:MINISIGN_PRIVATE_KEY
        $previousPassword = $env:MINISIGN_PASSWORD
        try {
            $env:MINISIGN_PRIVATE_KEY_FILE = $MinisignPrivateKeyFile
            $env:MINISIGN_PRIVATE_KEY = $null
            if (-not $env:MINISIGN_PASSWORD) {
                $storedPassword = Get-DPAPIMinisignPassword
                if ($storedPassword) { $env:MINISIGN_PASSWORD = $storedPassword }
            }
            foreach ($assetPath in $missingSignatureAssets) {
                Invoke-DesktopGoCommand -Arguments @('run', './cmd/sign', 'sign', $assetPath) `
                    -Description "minisign $(Split-Path $assetPath -Leaf)"
            }
        }
        finally {
            $env:MINISIGN_PRIVATE_KEY_FILE = $previousKeyFile
            $env:MINISIGN_PRIVATE_KEY = $previousKeyText
            $env:MINISIGN_PASSWORD = $previousPassword
        }
    }
    foreach ($name in $primaryNames) {
        $assetPath = Join-Path $incoming $name
        if (-not (Test-Path -LiteralPath "$assetPath.minisig" -PathType Leaf)) {
            throw "Signature is missing after recovery: $name.minisig"
        }
        Invoke-DesktopGoCommand -Arguments @('run', './cmd/sign', 'verify', $assetPath) `
            -Description "verify minisign signature for $name"
    }

    $githubDownloadPage = "https://github.com/$ReleaseRepository/releases/tag/$($identity.ReleaseTag)"
    $githubManifests = @{}
    $ossManifests = @{}
    $bootstrapGitHubManifest = ''
    $bootstrapOssManifest = ''
    $previousReleaseRepository = $env:RELEASE_REPOSITORY
    try {
        $env:RELEASE_REPOSITORY = $ReleaseRepository
        foreach ($target in $targets) {
            $githubManifest = Join-Path $githubManifestRoot "latest-$target.json"
            $ossManifest = Join-Path $ossManifestRoot "latest-$target.json"
            $platforms = switch ($target) {
                'windows-amd64' { @('windows-amd64') }
                'windows-arm64' { @('windows-arm64') }
                'macos-universal' { @('darwin-amd64', 'darwin-arm64') }
            }
            $platformArguments = @($platforms | ForEach-Object { '--platform'; $_ })
            Invoke-DesktopGoCommand -Arguments (@(
                'run', './cmd/sign', 'manifest',
                '--notes-file', $notesPath,
                '--download-page', $githubDownloadPage,
                '--output', $githubManifest
            ) + $platformArguments + @($incoming, $identity.Version, $identity.ReleaseTag)) `
                -Description "generate GitHub manifest for $target"
            Invoke-DesktopGoCommand -Arguments (@(
                'run', './cmd/sign', 'manifest',
                '--notes-file', $notesPath,
                '--base-url', ($OssPublicBaseURL.TrimEnd('/') + "/releases/$($identity.ReleaseTag)/$target"),
                '--download-page', $githubDownloadPage,
                '--output', $ossManifest
            ) + $platformArguments + @($incoming, $identity.Version, $identity.ReleaseTag)) `
                -Description "generate OSS manifest for $target"
            $githubManifests[$target] = $githubManifest
            $ossManifests[$target] = $ossManifest
        }
        if ($Bootstrap) {
            $bootstrapGitHubManifest = Join-Path $githubManifestRoot 'latest.json'
            $bootstrapOssManifest = Join-Path $ossManifestRoot 'latest.json'
            Invoke-DesktopGoCommand -Arguments @(
                'run', './cmd/sign', 'manifest', '--notes-file', $notesPath,
                '--download-page', $githubDownloadPage,
                '--output', $bootstrapGitHubManifest, $incoming, $identity.Version, $identity.ReleaseTag
            ) -Description 'generate bootstrap GitHub manifest'
            Invoke-DesktopGoCommand -Arguments @(
                'run', './cmd/sign', 'manifest',
                '--notes-file', $notesPath,
                '--base-url', ($OssPublicBaseURL.TrimEnd('/') + "/releases/$($identity.ReleaseTag)"),
                '--download-page', $githubDownloadPage, '--output', $bootstrapOssManifest,
                $incoming, $identity.Version, $identity.ReleaseTag
            ) -Description 'generate bootstrap OSS manifest'
        }
    }
    finally {
        $env:RELEASE_REPOSITORY = $previousReleaseRepository
    }

    $signaturePaths = @($primaryNames | ForEach-Object { Join-Path $incoming "$_.minisig" })
    $versionPrefix = "releases/$($identity.ReleaseTag)"
    $githubManifestPaths = @($githubManifests.Values)
    if ($Bootstrap) { $githubManifestPaths += $bootstrapGitHubManifest }
    Assert-LocalZeroWritePublicationPreflight $identity $draft $primaryNames $githubManifests $ossManifests `
        $incoming $bootstrapGitHubManifest $bootstrapOssManifest $notesPath $cloudPublisher
    if ($DryRun) {
        Write-Host "DryRun Finalize targets: $($targets -join ',')"
        foreach ($target in $targets) {
            $name = $script:ReleaseTargetAssets[$target]
            Write-Host "DryRun destination: GitHub $($identity.ReleaseTag)/$name, $name.minisig, latest-$target.json"
            Write-Host "DryRun destination: OSS releases/$($identity.ReleaseTag)/$target/*"
            Write-Host "DryRun destination: cloud records for $target"
            Write-Host "DryRun destination: OSS channels/stable/$target/latest.json (pointer last)"
        }
        if ($Bootstrap) { Write-Host 'DryRun destination: frozen global GitHub Latest and OSS channels/stable/latest.json' }
        Write-Host 'DryRun completed full zero-write idempotency/conflict preflight; no remote write was performed'
        return
    }
    foreach ($path in @($signaturePaths) + $githubManifestPaths) {
        Assert-RemoteSourceTagUnchanged $source
        Publish-GitHubDraftAsset $identity $draft $path
    }
    $completeDraft = Get-DraftRelease $identity -AllowPublished
    foreach ($target in $targets) {
        foreach ($name in @($script:ReleaseTargetAssets[$target], "$($script:ReleaseTargetAssets[$target]).minisig", "latest-$target.json")) {
            if (@($completeDraft.assets | Where-Object { $_.name -ceq $name }).Count -ne 1) {
                throw "Final Draft is missing selected asset: $name"
            }
        }
    }
    if ($Bootstrap) { Publish-OssImmutableObject "$versionPrefix/latest.json" $bootstrapOssManifest }
    Assert-RemoteSourceTagUnchanged $source
    Set-ManualCloudModeInReleaseBody $completeDraft $identity

    Invoke-LocalCloudPublisherCommand $cloudPublisher @('-preflight', '-base-url', $CloudBaseURL) `
        'cloud release API preflight immediately before immutable publication'

    foreach ($target in $targets) {
        $name = $script:ReleaseTargetAssets[$target]
        Assert-RemoteSourceTagUnchanged $source
        Publish-OssImmutableObject "$versionPrefix/$target/$name" (Join-Path $incoming $name)
        Publish-OssImmutableObject "$versionPrefix/$target/$name.minisig" (Join-Path $incoming "$name.minisig")
        if ($Bootstrap) {
            Publish-OssImmutableObject "$versionPrefix/$name" (Join-Path $incoming $name)
            Publish-OssImmutableObject "$versionPrefix/$name.minisig" (Join-Path $incoming "$name.minisig")
        }
        Publish-OssImmutableObject "$versionPrefix/$target/latest.json" $ossManifests[$target]
        Assert-PublicStablePointerCanAdvance $identity $ossManifests[$target] $target
    }

    Invoke-LocalCloudPublisherCommand $cloudPublisher @('-preflight', '-base-url', $CloudBaseURL) `
        'cloud release API preflight immediately before public GitHub release'

    if ($draft.isDraft) {
        Assert-RemoteSourceTagUnchanged $source
        $null = & gh release edit $identity.ReleaseTag --repo $ReleaseRepository --draft=false --latest=false 2>&1
        Assert-CommandSucceeded "publish GitHub Release $($identity.ReleaseTag)" $LASTEXITCODE
    }
    else {
        Write-Host "Reusing already-published GitHub Release $($identity.ReleaseTag)"
    }
    $publishedJSON = & gh release view $identity.ReleaseTag --repo $ReleaseRepository --json isDraft 2>&1
    Assert-CommandSucceeded 'verify published GitHub Release' $LASTEXITCODE
    $published = ($publishedJSON | Out-String) | ConvertFrom-Json
    if ($published.isDraft) { throw "GitHub Release is still a draft: $($identity.ReleaseTag)" }

    Assert-RemoteSourceTagUnchanged $source
    Invoke-LocalCloudPublisherPhase $cloudPublisher $targets $ossManifests $notesPath -Preflight
    Invoke-LocalCloudPublisherPhase $cloudPublisher $targets $ossManifests $notesPath
    Assert-RemoteSourceTagUnchanged $source
    if ($Bootstrap) { Assert-LocalSelectiveMigrationReady }
    Assert-LocalPointerPublicationPreflight $identity $targets $ossManifests $bootstrapOssManifest
    if ($Bootstrap) {
        Assert-RemoteSourceTagUnchanged $source
        Assert-LocalSelectiveMigrationReady
        Set-LocalGitHubLatestForBootstrap $identity
        Assert-LocalSelectiveMigrationReady
    }
    foreach ($target in $targets) {
        Assert-RemoteSourceTagUnchanged $source
        if ($Bootstrap) { Assert-LocalSelectiveMigrationReady }
        Publish-OssStablePointer $ossManifests[$target] $identity $target
    }
    if ($Bootstrap) {
        Assert-LocalSelectiveMigrationReady
        Publish-OssStablePointer $bootstrapOssManifest $identity
        Assert-LocalSelectiveMigrationReady
    }
    Write-Host "Finalized $($identity.ReleaseTag)"
}

function Invoke-FinalizeWithCredentialHandoff {
    $previousCloudToken = $env:KWU_CLOUD_RELEASE_TOKEN
    try {
        if (-not [string]::IsNullOrWhiteSpace($CloudTokenDPAPIFile)) {
            $env:KWU_CLOUD_RELEASE_TOKEN = Get-DPAPICloudReleaseToken
        }
        Invoke-Finalize
    }
    finally {
        $env:KWU_CLOUD_RELEASE_TOKEN = $previousCloudToken
    }
}

if ($MyInvocation.InvocationName -eq '.') {
    return
}

switch ($Action) {
    'Doctor' { Invoke-Doctor }
    'Prepare' { Invoke-Prepare }
    'BuildWindows' { Invoke-BuildWindows }
    'Finalize' { Invoke-FinalizeWithCredentialHandoff }
}

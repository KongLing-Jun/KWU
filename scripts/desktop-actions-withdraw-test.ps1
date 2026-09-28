$ErrorActionPreference = 'Stop'

$scriptPath = Join-Path $PSScriptRoot 'desktop-actions-withdraw.ps1'
$workflowPath = Join-Path $PSScriptRoot '..\.github\workflows\desktop-withdraw.yml'
$helper = Get-Content -Raw -LiteralPath $scriptPath
$workflow = Get-Content -Raw -LiteralPath $workflowPath

foreach ($required in @(
    "ValidateSet('ValidateInputs', 'Preflight', 'RestoreOss', 'RestoreGitHub', 'Verify')",
    'WITHDRAW $($bad.Tag) RESTORE $($recovery.Tag) TARGET $Target',
    "gh api `"repos/`$ReleaseRepository`" --jq '.permissions.push'",
    'cat "oss://$OssBucket/channels/stable/$Target/latest.json" --quiet',
    'OSS stable pointer matches neither the bad nor recovery immutable manifest',
    'Assert-WithdrawManifestIdentityMatches',
    'Target withdrawal never modifies shared GitHub Releases'
)) {
    if (-not $helper.Contains($required)) { throw "Withdraw helper contract is missing: $required" }
}
foreach ($forbidden in @(
    'gh release delete',
    'git push --delete',
    'ossutil rm',
    'delete-object',
    'KWU_CLOUD_RELEASE_TOKEN'
)) {
    if ($helper -match [regex]::Escape($forbidden)) {
        throw "Withdraw helper contains a forbidden destructive/cloud operation: $forbidden"
    }
}

$preflight = $workflow.IndexOf('-Action Preflight')
$restoreOss = $workflow.IndexOf('-Action RestoreOss')
$verify = $workflow.IndexOf('-Action Verify')
if ($preflight -lt 0 -or $restoreOss -le $preflight -or $verify -le $restoreOss) {
    throw 'Withdraw workflow must run zero-write preflight, target OSS restore and verification in that exact order'
}
if ($workflow.Contains('-Action RestoreGitHub')) { throw 'Target withdrawal must not modify GitHub Releases' }
foreach ($required in @(
    'workflow_dispatch:',
    'environment: desktop-release',
    'group: desktop-stable-release',
    'cancel-in-progress: false',
    'permissions:',
    'contents: read',
    'INPUT_CONFIRMATION: ${{ inputs.confirmation }}'
)) {
    if (-not $workflow.Contains($required)) { throw "Withdraw workflow contract is missing: $required" }
}
if ($workflow -match '(?m)^\s*contents:\s+write\s*$') {
    throw 'Withdraw workflow must not grant the event token repository write permission'
}

. $scriptPath -Action ValidateInputs

$BadTag = 'desktop-v2.0.0'
$RecoveryTag = 'desktop-v1.9.9'
$Reason = 'fixture rollback'
$Target = 'windows-arm64'
$Confirmation = 'WITHDRAW desktop-v2.0.0 RESTORE desktop-v1.9.9 TARGET windows-arm64'
$identity = Assert-WithdrawInputs
if ($identity.Bad.Version -cne 'v2.0.0' -or $identity.Recovery.Version -cne 'v1.9.9') {
    throw 'Valid withdraw identities were not resolved'
}

$Confirmation = 'WITHDRAW desktop-v2.0.0 RESTORE desktop-v1.9.8 TARGET windows-arm64'
$rejected = $false
try { $null = Assert-WithdrawInputs } catch { $rejected = $true }
if (-not $rejected) { throw 'Mismatched withdraw confirmation was accepted' }

$Confirmation = 'WITHDRAW desktop-v2.0.0 RESTORE desktop-v1.9.9 TARGET windows-arm64'
$RecoveryTag = 'desktop-v2.0.1'
$rejected = $false
try { $null = Assert-WithdrawInputs } catch { $rejected = $true }
if (-not $rejected) { throw 'A newer recovery version was accepted' }

$temp = Join-Path ([IO.Path]::GetTempPath()) ("kwu-withdraw-test-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $temp | Out-Null
try {
    $bad = Join-Path $temp 'bad.json'
    $recovery = Join-Path $temp 'recovery.json'
    $current = Join-Path $temp 'current.json'
    Set-Content -LiteralPath $bad -Value '{"version":"v2.0.0"}' -Encoding utf8
    Set-Content -LiteralPath $recovery -Value '{"version":"v1.9.9"}' -Encoding utf8
    Copy-Item -LiteralPath $bad -Destination $current
    if ((Get-StablePointerState $current $bad $recovery) -cne 'bad') {
        throw 'Bad stable state was not recognized'
    }
    Copy-Item -LiteralPath $recovery -Destination $current -Force
    if ((Get-StablePointerState $current $bad $recovery) -cne 'recovery') {
        throw 'Recovered stable state was not recognized for resume'
    }
    Set-Content -LiteralPath $current -Value '{"version":"v1.8.0"}' -Encoding utf8
    if ((Get-StablePointerState $current $bad $recovery) -cne 'unexpected') {
        throw 'Unexpected third-party stable state was not rejected'
    }
}
finally { Remove-Item -LiteralPath $temp -Recurse -Force -ErrorAction SilentlyContinue }

$temp = Join-Path ([IO.Path]::GetTempPath()) ("kwu-withdraw-binding-test-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $temp | Out-Null
try {
    $BadTag = 'desktop-v2.0.0'
    $RecoveryTag = 'desktop-v1.9.9'
    $Reason = 'fixture rollback'
    $Target = 'windows-arm64'
    $Confirmation = 'WITHDRAW desktop-v2.0.0 RESTORE desktop-v1.9.9 TARGET windows-arm64'
    $assetName = $script:DesktopTargetAssets[$Target]
    $script:FixturePaths = [pscustomobject]@{
        BadGitHub = Join-Path $temp 'bad-github.json'
        RecoveryGitHub = Join-Path $temp 'recovery-github.json'
        BadOss = Join-Path $temp 'bad-oss.json'
        RecoveryOss = Join-Path $temp 'recovery-oss.json'
        Stable = Join-Path $temp 'stable.json'
    }

    function Write-WithdrawManifestFixture {
        param([string]$Path, [string]$Version, [string]$Tag, [string]$Kind, [string]$SHA)
        $base = if ($Kind -ceq 'oss') {
            $OssPublicBaseURL.TrimEnd('/') + "/releases/$Tag/$Target/$([uri]::EscapeDataString($assetName))"
        }
        else {
            "https://github.example/releases/download/$Tag/$assetName"
        }
        [ordered]@{
            version = $Version
            platforms = [ordered]@{
                'windows-arm64' = [ordered]@{
                    url = $base
                    sig = "$base.minisig"
                    size = 2048
                    sha256 = $SHA
                }
            }
        } | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $Path -Encoding utf8
    }

    $matchingSHA = 'a' * 64
    Write-WithdrawManifestFixture $script:FixturePaths.BadGitHub 'v2.0.0' $BadTag github $matchingSHA
    Write-WithdrawManifestFixture $script:FixturePaths.BadOss 'v2.0.0' $BadTag oss $matchingSHA
    Write-WithdrawManifestFixture $script:FixturePaths.RecoveryOss 'v1.9.9' $RecoveryTag oss $matchingSHA
    Write-WithdrawManifestFixture $script:FixturePaths.RecoveryGitHub 'v1.9.9' $RecoveryTag github ('b' * 64)
    Copy-Item -LiteralPath $script:FixturePaths.BadOss -Destination $script:FixturePaths.Stable

    function Get-WithdrawPaths { return $script:FixturePaths }
    function Save-RemoteWithdrawState {}
    function Get-WithdrawRelease {
        param([string]$Tag)
        return [pscustomobject]@{
            tagName = $Tag
            isDraft = $false
            isPrerelease = $false
            assets = @(
                [pscustomobject]@{ name = $assetName },
                [pscustomobject]@{ name = "$assetName.minisig" },
                [pscustomobject]@{ name = "latest-$Target.json" }
            )
        }
    }
    $script:WithdrawWriteCalls = 0
    function Invoke-OssPutStable {
        param([string]$ManifestPath)
        $script:WithdrawWriteCalls++
    }
    function Save-PublicFile {
        param([string]$URL, [string]$Destination, [string]$Description)
        Copy-Item -LiteralPath $script:FixturePaths.RecoveryOss -Destination $Destination -Force
    }

    $bindingRejected = $false
    try { Invoke-RestoreOss } catch { $bindingRejected = $true }
    if (-not $bindingRejected) { throw 'Withdraw restore accepted mismatched GitHub and OSS manifest identities' }
    if ($script:WithdrawWriteCalls -ne 0) { throw 'Withdraw restore wrote OSS before rejecting manifest identity conflict' }

    Write-WithdrawManifestFixture $script:FixturePaths.RecoveryGitHub 'v1.9.9' $RecoveryTag github $matchingSHA
    Invoke-RestoreOss
    if ($script:WithdrawWriteCalls -ne 1) { throw 'Withdraw restore did not continue for matching manifest identities' }
}
finally { Remove-Item -LiteralPath $temp -Recurse -Force -ErrorAction SilentlyContinue }

Write-Host 'desktop withdraw contract tests passed'

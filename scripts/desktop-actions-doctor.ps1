[CmdletBinding()]
param(
    [string]$ReleaseRepository = 'xiaoxiaotest9527-bit/KWU-release',
    [string]$OssutilPath = 'ossutil',
    [string]$OssBucket = 'kwu-desktop-releases',
    [ValidateSet('required', 'skip')]
    [string]$CloudMode = 'required',
    [string]$CloudSkipReason = '',
    [string]$CloudBaseURL = ''
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'desktop-actions-base.ps1')
. (Join-Path $PSScriptRoot 'desktop-oss-doctor.ps1')

foreach ($name in @('GH_TOKEN', 'MINISIGN_PRIVATE_KEY', 'MINISIGN_PASSWORD', 'OSS_ACCESS_KEY_ID', 'OSS_ACCESS_KEY_SECRET')) {
    if (-not [Environment]::GetEnvironmentVariable($name)) { throw "Required secret is missing: $name" }
}

$canPush = (Invoke-Native "inspect $ReleaseRepository permissions with release token" {
    gh api "repos/$ReleaseRepository" --jq '.permissions.push'
} | Out-String).Trim().ToLowerInvariant()
if ($canPush -ne 'true') {
    throw "RELEASE_REPO_TOKEN does not have push permission for $ReleaseRepository"
}

Push-Location (Join-Path $PSScriptRoot '..\desktop')
try {
    Invoke-Native 'run minisign key self-test' { go run ./cmd/sign selftest } | Out-Null
}
finally { Pop-Location }

if (-not (Get-Command $OssutilPath -ErrorAction SilentlyContinue)) {
    throw "ossutil executable does not exist: $OssutilPath"
}
$pointerOutput = & $OssutilPath cat "oss://$OssBucket/channels/stable/latest.json" --quiet 2>&1
$pointerExitCode = $LASTEXITCODE
$pointerProbe = Resolve-DesktopStablePointerProbe $pointerExitCode ($pointerOutput | Out-String)
if (-not $pointerProbe.Present) {
    Write-Host 'OSS stable pointer does not exist yet; this is the supported state before the first desktop release'
}

Invoke-CloudPreflight
Write-Host 'Desktop release Doctor passed without creating or updating remote state'
exit 0

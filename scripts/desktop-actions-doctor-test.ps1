$ErrorActionPreference = 'Stop'

$workflow = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot '..\.github\workflows\desktop-release-doctor.yml')
$doctor = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot 'desktop-actions-doctor.ps1')
$base = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot 'desktop-actions-base.ps1')
$ossDoctorPath = Join-Path $PSScriptRoot 'desktop-oss-doctor.ps1'
$ossDoctor = Get-Content -Raw -LiteralPath $ossDoctorPath
. $ossDoctorPath

foreach ($required in @(
    'workflow_dispatch:',
    'operation:',
    'request_id:',
    'runs-on: ubuntu-24.04',
    'environment: desktop-release',
    'MINISIGN_NEW_PASSWORD_TEMP',
    'go run ./cmd/sign reencrypt "$RUNNER_TEMP/suiyuan.key"',
    'minisign-reencrypted-${{ inputs.request_id }}',
    'retention-days: 1',
    'ref: ${{ github.sha }}',
    "GITHUB_REF -ne 'refs/heads/main-v3'"
)) {
    if (-not $workflow.Contains($required)) { throw "Doctor workflow is missing: $required" }
}
if ($workflow -match '(?m)^\s{2}push:\s*$' -or $workflow -match '(?m)^\s{2}pull_request:\s*$') {
    throw 'Doctor workflow must be workflow_dispatch-only'
}
if ($doctor -match 'put-object|release\s+(create|edit|upload)|client-releases\s+-Method\s+Post') {
    throw 'Doctor entry contains a remote write operation'
}
if ($doctor -notmatch 'cat\s+"oss://\$OssBucket/channels/stable/latest\.json"\s+--quiet') {
    throw 'Doctor must probe one exact OSS object without ListObjects'
}
if ($doctor -notmatch "\.permissions\.push" -or $doctor -notmatch 'does not have push permission') {
    throw 'Doctor must prove that RELEASE_REPO_TOKEN can write to the release repository'
}
if ($ossDoctor -notmatch 'ConvertFrom-Json' -or $ossDoctor -notmatch 'desktop manifest shape') {
    throw 'Doctor must validate the exact OSS object as a desktop manifest'
}
if ($base -notmatch 'publish-cloud-release -preflight -base-url') {
    throw 'Cloud Doctor must use the shared Go preflight contract'
}

$valid = Resolve-DesktopStablePointerProbe 0 '{"version":"v1.0.11","platforms":{}}'
if (-not $valid.Present -or $valid.Manifest.version -ne 'v1.0.11') {
    throw 'Valid OSS stable pointer was not accepted'
}
$missing = Resolve-DesktopStablePointerProbe 2 "Http Status Code: 404.`nError Code: NoSuchKey."
if ($missing.Present) { throw 'NoSuchKey bootstrap state was not recognized' }

foreach ($failure in @(
    @{ ExitCode = 1; Output = 'Http Status Code: 403. Error Code: AccessDenied.' },
    @{ ExitCode = 2; Output = 'Http Status Code: 404. Error Code: NoSuchBucket.' },
    @{ ExitCode = 2; Output = 'Http Status Code: 404.' },
    @{ ExitCode = 1; Output = 'network connection reset' },
    @{ ExitCode = 0; Output = '' },
    @{ ExitCode = 0; Output = '{bad json' },
    @{ ExitCode = 0; Output = '{"version":"v1.0.11"}' }
)) {
    $rejected = $false
    try { Resolve-DesktopStablePointerProbe $failure.ExitCode $failure.Output | Out-Null }
    catch { $rejected = $true }
    if (-not $rejected) { throw "Invalid OSS probe result was accepted: $($failure.Output)" }
}

$testRoot = Join-Path ([IO.Path]::GetTempPath()) ("kwu-actions-doctor-test-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $testRoot | Out-Null
try {
    $harnessPath = Join-Path $testRoot 'bootstrap-harness.ps1'
    @'
param([Parameter(Mandatory)][string]$DoctorScript)

$env:GH_TOKEN = 'test-release-token'
$env:MINISIGN_PRIVATE_KEY = 'test-private-key'
$env:MINISIGN_PASSWORD = 'test-password'
$env:OSS_ACCESS_KEY_ID = 'test-access-key-id'
$env:OSS_ACCESS_KEY_SECRET = 'test-access-key-secret'

function global:gh {
    $global:LASTEXITCODE = 0
    return 'true'
}
function global:go {
    $global:LASTEXITCODE = 0
}
function global:ossutil {
    $global:LASTEXITCODE = 2
    'Http Status Code: 404.'
    'Error Code: NoSuchKey.'
}

. $DoctorScript -ReleaseRepository 'example/release' -OssutilPath 'ossutil' -OssBucket 'test-bucket' `
    -CloudMode skip -CloudSkipReason 'cloud test fixture'

if (Test-Path -LiteralPath variable:\LASTEXITCODE) { exit $LASTEXITCODE }
'@ | Set-Content -LiteralPath $harnessPath -Encoding utf8

    $pwsh = (Get-Process -Id $PID).Path
    $doctorPath = Join-Path $PSScriptRoot 'desktop-actions-doctor.ps1'
    $bootstrapOutput = & $pwsh -NoProfile -File $harnessPath $doctorPath 2>&1
    $bootstrapExitCode = $LASTEXITCODE
    if ($bootstrapExitCode -ne 0) {
        throw "Doctor leaked the accepted OSS bootstrap exit code ${bootstrapExitCode}: $($bootstrapOutput | Out-String)"
    }
    if (-not (($bootstrapOutput | Out-String).Contains('Desktop release Doctor passed'))) {
        throw "Doctor bootstrap process did not reach its success marker: $($bootstrapOutput | Out-String)"
    }
}
finally {
    Remove-Item -LiteralPath $testRoot -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host 'desktop Actions Doctor static tests passed'

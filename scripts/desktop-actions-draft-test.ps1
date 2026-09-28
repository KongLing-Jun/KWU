$ErrorActionPreference = 'Stop'

$workflow = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot '..\.github\workflows\desktop-build-draft.yml')
$helper = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot 'desktop-actions-draft.ps1')
$common = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot 'desktop-actions-common.ps1')

foreach ($required in @(
    'release_targets:',
    "contains(needs.resolve.outputs.targets, 'macos-universal')",
    'workflow_dispatch:',
    'dry-run',
    'existing-tag',
    'release_targets must be non-empty',
    'create-draft:',
    'Create resumable Draft before platform builds',
    'build-and-upload-macos:',
    'needs: resolve',
    'runs-on: macos-15',
    'CreateDraft',
    'WaitForDraft',
    'UploadAsset',
    'ArtifactPath = ''release-source/dist/Clouvia-darwin-universal.dmg''',
    'Draft ready for the Windows signing machine',
    '-Action BuildWindows',
    'cancel-in-progress: false',
    'group: desktop-stable-release',
    'assert-macos-release-architecture.sh'
)) {
    if (-not $workflow.Contains($required)) { throw "Build Draft workflow is missing: $required" }
}

foreach ($forbidden in @(
    'runs-on: windows-2025',
    'runs-on: windows-latest',
    'build-windows-amd64:',
    'build-windows-arm64:',
    'desktop-candidates.ps1',
    'candidate-provenance-windows',
    'reuse_candidates:',
    'windows_amd64_run_id:',
    'windows_arm64_run_id:',
    'MINISIGN_PRIVATE_KEY',
    'OSS_ACCESS_KEY',
    'KWU_CLOUD_RELEASE_TOKEN',
    'WINDOWS_SIGN_CERT_THUMBPRINT',
    'WINDOWS_SIGN_CERT_SUBJECT',
    'WINDOWS_SIGNTOOL_PATH',
    'SIGNPATH_',
    'ubuntu-latest',
    'macos-latest',
    'push:',
    'emergency',
    'allow_non_main_v3',
    'EnsureEmergencyTag'
)) {
    if ($workflow.Contains($forbidden)) { throw "Build Draft workflow must not contain: $forbidden" }
}

if ($workflow -notmatch '(?s)create-draft:.*?CreateDraft.*?Draft ready for the Windows signing machine') {
    throw 'Draft creation must expose the fixed Windows-machine handoff immediately'
}
if ($workflow -notmatch '(?s)build-and-upload-macos:.*?needs: resolve.*?Build and verify universal DMG.*?WaitForDraft.*?UploadAsset') {
    throw 'macOS must build after resolve and bounded-wait for the exact Draft only before upload'
}
$macJob = [regex]::Match($workflow, '(?ms)^  build-and-upload-macos:.*?(?=^  [a-z][a-z0-9-]+:|\z)').Value
if ($macJob -notmatch "if:.*needs\.resolve\.result == 'success'.*contains\(needs\.resolve\.outputs\.targets, 'macos-universal'\)") {
    throw 'macOS build must require macos-universal for both real and dry-run dispatches'
}
foreach ($case in @(
    @{ Targets = 'windows-amd64'; Expected = $false },
    @{ Targets = 'windows-amd64,windows-arm64'; Expected = $false },
    @{ Targets = 'macos-universal'; Expected = $true },
    @{ Targets = 'windows-amd64,macos-universal'; Expected = $true }
)) {
    $actual = $case.Targets.Contains('macos-universal')
    if ($actual -ne $case.Expected) {
        throw "macOS job condition truth table failed for $($case.Targets)"
    }
}
if ($workflow -notmatch '(?s)Check out trusted release tooling.*?ref: \$\{\{ github\.sha \}\}.*?Check out frozen source') {
    throw 'Release orchestration must use trusted workflow tooling and treat the frozen source as build input'
}
if ($workflow -notmatch '(?s)dry-run-summary:.*?No tag, Draft Release, asset upload, OSS object, or cloud release row was created') {
    throw 'Dry-run must remain a no-publication workflow path'
}
if ($helper -match '--clobber') { throw 'Draft asset upload must never clobber an existing asset' }
foreach ($contract in @(
    "ValidateSet('Resolve', 'CreateDraft', 'WaitForDraft', 'UploadAsset')",
    'Timed out waiting for exact Draft',
    'Assert-PreFinalizationDraft',
    'Get-PrimaryAssetMetadata',
    'Existing Draft asset differs from local output',
    'Set-ReleaseAssetMetadata',
    'Assert-RemoteAnnotatedTagIdentity $identity.Tag $tagObject $commit'
)) {
    if (-not $helper.Contains($contract)) { throw "Draft helper is missing: $contract" }
}
if (-not $common.Contains("if (-not `$assetLines) { `$assetLines = '(Pending manual builds.)' }")) {
    throw 'A newly-created Draft must explicitly record pending manual platform builds'
}
foreach ($forbidden in @('AllowNonMainV3', 'EmergencyReason', 'Invoke-EnsureEmergencyTag', "'emergency'")) {
    if ($helper.Contains($forbidden)) { throw "Draft helper retains forbidden source bypass: $forbidden" }
}

. (Join-Path $PSScriptRoot 'desktop-actions-draft.ps1') -Action Resolve

function Assert-Throws {
    param([scriptblock]$Command, [string]$Expected)
    try { & $Command }
    catch {
        if ($_.Exception.Message -notlike "*$Expected*") {
            throw "Expected '$Expected', received '$($_.Exception.Message)'"
        }
        return
    }
    throw "Expected failure containing '$Expected'"
}

$testRoot = Join-Path ([IO.Path]::GetTempPath()) ("kwu-draft-test-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $testRoot | Out-Null
try {
    $dmg = Join-Path $testRoot 'Clouvia-darwin-universal.dmg'
    [IO.File]::WriteAllBytes($dmg, [byte[]](1, 2, 3))
    $metadata = Get-PrimaryAssetMetadata $dmg
    if ($metadata.Name -cne 'Clouvia-darwin-universal.dmg' -or
        $metadata.Architecture -cne 'x86_64+arm64 payload' -or
        $metadata.SHA256 -notmatch '^[0-9a-f]{64}$') {
        throw 'Single primary asset metadata is invalid'
    }
    $unexpected = Join-Path $testRoot 'unexpected.exe'
    [IO.File]::WriteAllBytes($unexpected, [byte[]](1))
    Assert-Throws { Get-PrimaryAssetMetadata $unexpected } 'not allowed'

    $Version = '1.2.3'
    $Tag = 'desktop-v1.2.3'
    $SourceCommit = '1' * 40
    $TagObject = '2' * 40
    $NotesFile = Join-Path $testRoot 'notes.md'
    [IO.File]::WriteAllText($NotesFile, "fixture notes`n", [Text.UTF8Encoding]::new($false))
    $ArtifactPath = $dmg
    $identity = Get-ReleaseIdentity $Version $Tag
    $script:fakeRelease = [pscustomobject]@{
        isDraft = $false
        body = New-ReleaseBody $identity $SourceCommit 'fixture notes' @() $TagObject
        assets = @([pscustomobject]@{ name = 'Clouvia-windows-amd64-installer.exe' })
    }
    $script:uploadCount = 0
    $script:downloadBytes = [byte[]](1, 2, 3)
    function Assert-RemoteAnnotatedTagIdentity { param($ReleaseTag, $ObjectSHA, $CommitSHA) }
    $script:releaseQueue = $null
    function Get-ReleaseJSON {
        param([string]$ReleaseTag, [switch]$AllowMissing)
        if ($null -eq $script:releaseQueue) { return $script:fakeRelease }
        if ($script:releaseQueue.Count -eq 0) { return $null }
        $result = $script:releaseQueue[0]
        $script:releaseQueue = if ($script:releaseQueue.Count -eq 1) { @() } else { @($script:releaseQueue[1..($script:releaseQueue.Count - 1)]) }
        return $result
    }
    function Invoke-Native {
        param([string]$Description, [scriptblock]$Command)
        if ($Description -like 'upload Draft asset*') {
            $script:uploadCount++
            $script:fakeRelease.assets += [pscustomobject]@{ name = 'Clouvia-darwin-universal.dmg' }
        }
        elseif ($Description -like 'download existing Draft asset*') {
            $downloadDir = Get-ChildItem -LiteralPath ([IO.Path]::GetTempPath()) -Directory |
                Where-Object Name -like 'kwu-existing-*' |
                Sort-Object LastWriteTimeUtc -Descending |
                Select-Object -First 1
            [IO.File]::WriteAllBytes(
                (Join-Path $downloadDir.FullName 'Clouvia-darwin-universal.dmg'),
                $script:downloadBytes
            )
        }
        return ''
    }
    function Set-ReleaseAssetMetadata {
        param($Identity, $Release, $Metadata)
        $Release.body = "$($Release.body)`n- ``$($Metadata.Name)``: size=$($Metadata.Size) sha256=``$($Metadata.SHA256)`` architecture=$($Metadata.Architecture)"
    }

    $readyDraft = [pscustomobject]@{
        isDraft = $true
        body = New-ReleaseBody $identity $SourceCommit 'fixture notes' @() $TagObject
        assets = @()
    }
    $WaitTimeoutSeconds = 1
    $WaitIntervalSeconds = 0
    $script:releaseQueue = @($null, $readyDraft)
    Invoke-WaitForDraft

    $WaitTimeoutSeconds = 0
    $script:releaseQueue = @()
    Assert-Throws { Invoke-WaitForDraft } 'Timed out waiting for exact Draft'

    $wrongProvenance = [pscustomobject]@{
        isDraft = $true
        body = (New-ReleaseBody $identity ('3' * 40) 'fixture notes' @() $TagObject)
        assets = @()
    }
    $script:releaseQueue = @($wrongProvenance)
    Assert-Throws { Invoke-WaitForDraft } 'Release body provenance or notes do not match'

    $mutatedDraft = [pscustomobject]@{
        isDraft = $true
        body = $readyDraft.body
        assets = @([pscustomobject]@{ name = 'unexpected.zip' })
    }
    $script:releaseQueue = @($mutatedDraft)
    Assert-Throws { Invoke-WaitForDraft } 'unexpected asset'

    $publishedRelease = [pscustomobject]@{ isDraft = $false; body = $readyDraft.body; assets = @() }
    $script:releaseQueue = @($publishedRelease)
    Assert-Throws { Invoke-WaitForDraft } 'not the expected Draft'
    $script:releaseQueue = $null

    Invoke-CreateDraft
    if ($script:uploadCount -ne 0) {
        throw 'CreateDraft must reuse an appendable published Release without writing'
    }

    Invoke-UploadAsset
    if ($script:uploadCount -ne 1 -or
        @($script:fakeRelease.assets | Where-Object name -eq 'Clouvia-darwin-universal.dmg').Count -ne 1) {
        throw 'A missing primary asset was not appended to the published Release'
    }

    $script:uploadCount = 0
    Invoke-UploadAsset
    if ($script:uploadCount -ne 0) {
        throw 'A prepositioned primary asset with the same SHA was not reused idempotently'
    }

    $script:downloadBytes = [byte[]](9, 9, 9)
    Assert-Throws { Invoke-UploadAsset } 'differs from local output'
    $script:fakeRelease.assets += [pscustomobject]@{ name = 'unexpected.zip' }
    Assert-Throws { Assert-AppendableReleaseAssets $script:fakeRelease } 'unexpected asset'
}
finally {
    Remove-Item -LiteralPath $testRoot -Recurse -Force
}

Write-Host 'desktop Actions Draft-first/macOS-direct-upload contract tests passed'

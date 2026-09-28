$ErrorActionPreference = 'Stop'
$scriptPath = Join-Path $PSScriptRoot 'desktop-candidates.ps1'
$root = Join-Path ([IO.Path]::GetTempPath()) ("desktop-candidates-test-" + [guid]::NewGuid().ToString('N'))
$commit = '1' * 40
$tagObject = '2' * 40

function Write-Fixture {
    param([string]$RunID, [string]$Target, [string]$Content)
    $directory = Join-Path $root "candidates/$RunID"
    New-Item -ItemType Directory -Path $directory -Force | Out-Null
    $names = @{
        'windows-amd64' = 'Clouvia-windows-amd64-installer.exe'
        'windows-arm64' = 'Clouvia-windows-arm64-installer.exe'
        'macos-universal' = 'Clouvia-darwin-universal.dmg'
    }
    $artifact = Join-Path $directory $names[$Target]
    [IO.File]::WriteAllText($artifact, $Content, [Text.UTF8Encoding]::new($false))
    $oldRunID = $env:GITHUB_RUN_ID
    try {
        $env:GITHUB_RUN_ID = $RunID
        & $scriptPath -Action Write -Target $Target -Tag desktop-v1.0.4 -TagObject $tagObject `
            -SourceCommit $commit -Version v1.0.4 -ArtifactPath $artifact `
            -MetadataPath (Join-Path $directory "candidate-provenance-$Target.json")
    }
    finally { $env:GITHUB_RUN_ID = $oldRunID }
}

try {
    foreach ($target in @('windows-amd64', 'windows-arm64', 'macos-universal')) {
        Write-Fixture 100 $target "$target-payload"
    }
    $summary = Join-Path $root 'summary.md'
    $env:GITHUB_STEP_SUMMARY = $summary
    & $scriptPath -Action SelectLocal -Tag desktop-v1.0.4 -TagObject $tagObject `
        -SourceCommit $commit -Version v1.0.4 -CandidatesDirectory (Join-Path $root candidates) `
        -OutputDirectory (Join-Path $root selected)
    foreach ($name in @(
        'Clouvia-windows-amd64-installer.exe',
        'Clouvia-windows-arm64-installer.exe',
        'Clouvia-darwin-universal.dmg'
    )) {
        if (-not (Test-Path -LiteralPath (Join-Path $root "selected/$name"))) {
            throw "full selection omitted $name"
        }
    }
    $catalog = Join-Path $root 'runs.json'
    @([ordered]@{ databaseId = 100; status = 'completed'; conclusion = 'success' }) |
        ConvertTo-Json | Set-Content -LiteralPath $catalog -Encoding utf8
    & $scriptPath -Action Discover -Tag desktop-v1.0.4 -TagObject $tagObject `
        -SourceCommit $commit -Version v1.0.4 -Repository owner/repo `
        -CandidatesDirectory (Join-Path $root candidates) -OutputDirectory (Join-Path $root discovered) `
        -TestRunCatalogPath $catalog

    $mockBin = Join-Path $root 'mock-bin'
    New-Item -ItemType Directory -Path $mockBin | Out-Null
    if ($IsWindows) {
        $mockGH = @'
@echo off
if "%1 %2"=="run list" (
  echo [{"databaseId":100,"status":"completed","conclusion":"success"},{"databaseId":200,"status":"completed","conclusion":"success"}]
  exit /b 0
)
if "%1 %2"=="run download" (
  exit /b 1
)
exit /b 9
'@
        [IO.File]::WriteAllText((Join-Path $mockBin 'gh.cmd'), $mockGH, [Text.ASCIIEncoding]::new())
    }
    else {
        $mockGH = @'
#!/usr/bin/env bash
if [[ "$1 $2" == "run list" ]]; then
  echo '[{"databaseId":100,"status":"completed","conclusion":"success"},{"databaseId":200,"status":"completed","conclusion":"success"}]'
  exit 0
fi
if [[ "$1 $2" == "run download" ]]; then
  exit 1
fi
exit 9
'@
        $mockGHPath = Join-Path $mockBin 'gh'
        [IO.File]::WriteAllText($mockGHPath, $mockGH, [Text.UTF8Encoding]::new($false))
        & chmod +x $mockGHPath
        if ($LASTEXITCODE -ne 0) { throw 'Failed to make the gh test shim executable' }
    }
    $processCandidates = Join-Path $root 'process-candidates'
    Copy-Item -LiteralPath (Join-Path $root 'candidates/100') -Destination (Join-Path $processCandidates '100') -Recurse
    $oldPath = $env:PATH
    try {
        $env:PATH = "$mockBin$([IO.Path]::PathSeparator)$oldPath"
        $processOutput = & pwsh -NoProfile -File $scriptPath -Action Discover `
            -Tag desktop-v1.0.4 -TagObject $tagObject -SourceCommit $commit -Version v1.0.4 `
            -Repository owner/repo -CandidatesDirectory $processCandidates `
            -OutputDirectory (Join-Path $root process-selected) 2>&1
        $processExitCode = $LASTEXITCODE
    }
    finally { $env:PATH = $oldPath }
    if ($processExitCode -ne 0) {
        throw "Discover exited $processExitCode after tolerating missing non-explicit artifacts: $processOutput"
    }
    foreach ($name in @(
        'Clouvia-windows-amd64-installer.exe',
        'Clouvia-windows-arm64-installer.exe',
        'Clouvia-darwin-universal.dmg'
    )) {
        if (-not (Test-Path -LiteralPath (Join-Path $root "process-selected/$name"))) {
            throw "process-level Discover selection omitted $name"
        }
    }
    try {
        $env:PATH = "$mockBin$([IO.Path]::PathSeparator)$oldPath"
        $explicitOutput = & pwsh -NoProfile -File $scriptPath -Action Discover `
            -Tag desktop-v1.0.4 -TagObject $tagObject -SourceCommit $commit -Version v1.0.4 `
            -Repository owner/repo -CandidatesDirectory (Join-Path $root explicit-process-candidates) `
            -OutputDirectory (Join-Path $root explicit-process-selected) -WindowsAMD64RunID 200 2>&1
        $explicitExitCode = $LASTEXITCODE
    }
    finally { $env:PATH = $oldPath }
    if ($explicitExitCode -eq 0 -or "$explicitOutput" -notmatch 'Failed to download explicit windows-amd64 candidate') {
        throw "An explicit candidate download failure was swallowed: exit=$explicitExitCode output=$explicitOutput"
    }

    try {
        & $scriptPath -Action Discover -Tag desktop-v1.0.4 -TagObject $tagObject `
            -SourceCommit $commit -Version v1.0.4 -Repository owner/repo `
            -CandidatesDirectory (Join-Path $root candidates) -OutputDirectory (Join-Path $root invalid-run) `
            -WindowsAMD64RunID abc -TestRunCatalogPath $catalog
        throw 'non-numeric explicit Run ID was accepted'
    }
    catch {
        if ($_.Exception.Message -notmatch 'decimal digits only') { throw }
    }

    Write-Fixture 101 windows-amd64 'conflicting-payload'
    try {
        & $scriptPath -Action SelectLocal -Tag desktop-v1.0.4 -TagObject $tagObject `
            -SourceCommit $commit -Version v1.0.4 -CandidatesDirectory (Join-Path $root candidates) `
            -OutputDirectory (Join-Path $root conflict)
        throw 'conflicting candidate hashes were accepted'
    }
    catch {
        if ($_.Exception.Message -notmatch 'Conflicting windows-amd64 candidates require an explicit Run ID') { throw }
    }

    & $scriptPath -Action SelectLocal -Tag desktop-v1.0.4 -TagObject $tagObject `
        -SourceCommit $commit -Version v1.0.4 -CandidatesDirectory (Join-Path $root candidates) `
        -OutputDirectory (Join-Path $root explicit) -WindowsAMD64RunID 100
    if ((Get-Content -Raw -LiteralPath (Join-Path $root explicit/Clouvia-windows-amd64-installer.exe)) -ne 'windows-amd64-payload') {
        throw 'explicit Run ID did not select the requested candidate'
    }

    $metadata = Join-Path $root candidates/100/candidate-provenance-macos-universal.json
    $record = Get-Content -Raw -LiteralPath $metadata | ConvertFrom-Json
    $record.source_commit = '3' * 40
    [IO.File]::WriteAllText($metadata, (($record | ConvertTo-Json) + "`n"), [Text.UTF8Encoding]::new($false))
    try {
        & $scriptPath -Action SelectLocal -Tag desktop-v1.0.4 -TagObject $tagObject `
            -SourceCommit $commit -Version v1.0.4 -CandidatesDirectory (Join-Path $root candidates) `
            -OutputDirectory (Join-Path $root invalid) -WindowsAMD64RunID 100
        throw 'mismatched provenance was accepted'
    }
    catch {
        if ($_.Exception.Message -notmatch 'No valid macos-universal candidate') { throw }
    }
}
finally {
    $env:GITHUB_STEP_SUMMARY = $null
    Remove-Item -LiteralPath $root -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host 'desktop candidate tests passed'

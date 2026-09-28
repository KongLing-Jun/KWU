[CmdletBinding()]
param(
    [ValidateSet('Write', 'SelectLocal', 'Discover')]
    [string]$Action,
    [ValidateSet('windows-amd64', 'windows-arm64', 'macos-universal', 'all')]
    [string]$Target = 'all',
    [string[]]$Targets = @('all'),
    [string]$Tag = '',
    [string]$TagObject = '',
    [string]$SourceCommit = '',
    [string]$Version = '',
    [string]$ArtifactPath = '',
    [string]$MetadataPath = '',
    [string]$CandidatesDirectory = '',
    [string]$OutputDirectory = '',
    [string]$Repository = '',
    [string]$Workflow = 'desktop-build-draft.yml',
    [string]$CurrentRunID = '',
    [string]$TestRunCatalogPath = '',
    [string]$WindowsAMD64RunID = '',
    [string]$WindowsARM64RunID = '',
    [string]$MacOSRunID = ''
)

$ErrorActionPreference = 'Stop'

$script:CandidateFiles = [ordered]@{
    'windows-amd64' = 'Clouvia-windows-amd64-installer.exe'
    'windows-arm64' = 'Clouvia-windows-arm64-installer.exe'
    'macos-universal' = 'Clouvia-darwin-universal.dmg'
}

function Resolve-CandidateTargets {
    $values = @($Targets | ForEach-Object { $_ -split ',' } | ForEach-Object { $_.Trim() } | Where-Object { $_ })
    if ($values.Count -eq 0) { throw 'At least one candidate target is required' }
    if ($values -contains 'all') {
        if (@($values | Where-Object { $_ -ne 'all' }).Count) { throw 'all cannot be combined with concrete targets' }
        return @($script:CandidateFiles.Keys)
    }
    foreach ($value in $values) {
        if (-not $script:CandidateFiles.Contains($value)) { throw "Unknown candidate target: $value" }
    }
    return @($script:CandidateFiles.Keys | Where-Object { $values -contains $_ })
}

function Get-LowerSHA256 {
    param([string]$Path)
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Assert-Identity {
    if ($Tag -notmatch '^desktop-v[0-9]+\.[0-9]+\.[0-9]+$') { throw "Invalid desktop tag: $Tag" }
    if ($Version -notmatch '^v[0-9]+\.[0-9]+\.[0-9]+$') { throw "Invalid desktop version: $Version" }
    if ($SourceCommit -notmatch '^[0-9a-f]{40}$') { throw 'SourceCommit must be an exact lowercase 40-character SHA' }
    if ($TagObject -notmatch '^[0-9a-f]{40}$') { throw 'TagObject must be an exact lowercase 40-character SHA' }
}

function Write-Candidate {
    Assert-Identity
    if ($Target -eq 'all') { throw 'Write requires one concrete candidate target' }
    $expectedName = $script:CandidateFiles[$Target]
    if (-not (Test-Path -LiteralPath $ArtifactPath -PathType Leaf)) {
        throw "Candidate artifact is missing: $ArtifactPath"
    }
    if ((Split-Path -Leaf $ArtifactPath) -cne $expectedName) {
        throw "Candidate artifact for $Target must be named $expectedName"
    }
    $file = Get-Item -LiteralPath $ArtifactPath
    if ($file.Length -le 0) { throw "Candidate artifact is empty: $expectedName" }
    $metadata = [ordered]@{
        schema_version = 1
        target = $Target
        tag = $Tag
        tag_object = $TagObject
        source_commit = $SourceCommit
        version = $Version
        run_id = [string]$env:GITHUB_RUN_ID
        file_name = $expectedName
        size = $file.Length
        sha256 = Get-LowerSHA256 $ArtifactPath
    }
    $parent = Split-Path -Parent $MetadataPath
    if ($parent) { New-Item -ItemType Directory -Path $parent -Force | Out-Null }
    [IO.File]::WriteAllText(
        [IO.Path]::GetFullPath($MetadataPath),
        (($metadata | ConvertTo-Json -Depth 4) + "`n"),
        [Text.UTF8Encoding]::new($false)
    )
}

function Get-ExplicitRunID {
    param([string]$CandidateTarget)
    switch ($CandidateTarget) {
        'windows-amd64' { return $WindowsAMD64RunID.Trim() }
        'windows-arm64' { return $WindowsARM64RunID.Trim() }
        'macos-universal' { return $MacOSRunID.Trim() }
    }
}

function Read-ValidCandidate {
    param([string]$RunDirectory, [string]$CandidateTarget)
    $metadataFiles = @(Get-ChildItem -LiteralPath $RunDirectory -Filter "candidate-provenance-$CandidateTarget.json" -File -Recurse)
    if ($metadataFiles.Count -ne 1) { return $null }
    try { $metadata = Get-Content -Raw -LiteralPath $metadataFiles[0].FullName | ConvertFrom-Json }
    catch { return $null }
    $expectedName = $script:CandidateFiles[$CandidateTarget]
    $artifact = @(Get-ChildItem -LiteralPath $RunDirectory -Filter $expectedName -File -Recurse)
    if ($artifact.Count -ne 1 -or $artifact[0].Length -le 0) { return $null }
    $runID = Split-Path -Leaf $RunDirectory
    if (
        [int]$metadata.schema_version -ne 1 -or
        [string]$metadata.target -cne $CandidateTarget -or
        [string]$metadata.tag -cne $Tag -or
        [string]$metadata.tag_object -cne $TagObject -or
        [string]$metadata.source_commit -cne $SourceCommit -or
        [string]$metadata.version -cne $Version -or
        [string]$metadata.run_id -cne $runID -or
        [string]$metadata.file_name -cne $expectedName -or
        [int64]$metadata.size -ne $artifact[0].Length
    ) { return $null }
    $sha = Get-LowerSHA256 $artifact[0].FullName
    if ([string]$metadata.sha256 -cne $sha) { return $null }
    return [pscustomobject]@{
        Target = $CandidateTarget
        RunID = $runID
        FileName = $expectedName
        Path = $artifact[0].FullName
        Size = $artifact[0].Length
        SHA256 = $sha
    }
}

function Select-LocalCandidates {
    Assert-Identity
    New-Item -ItemType Directory -Path $OutputDirectory -Force | Out-Null
    foreach ($candidateTarget in @(Resolve-CandidateTargets)) {
        $valid = @()
        foreach ($runDirectory in @(Get-ChildItem -LiteralPath $CandidatesDirectory -Directory -ErrorAction SilentlyContinue)) {
            $candidate = Read-ValidCandidate $runDirectory.FullName $candidateTarget
            if ($null -ne $candidate) { $valid += $candidate }
        }
        $explicit = Get-ExplicitRunID $candidateTarget
        if ($explicit) {
            $valid = @($valid | Where-Object { $_.RunID -ceq $explicit })
            if ($valid.Count -ne 1) {
                throw "Explicit Run ID $explicit has no valid $candidateTarget candidate for the frozen release identity"
            }
        }
        else {
            $hashes = @($valid.SHA256 | Select-Object -Unique)
            if ($hashes.Count -gt 1) {
                $details = ($valid | Sort-Object RunID | ForEach-Object { "Run $($_.RunID)=$($_.SHA256)" }) -join '; '
                throw "Conflicting $candidateTarget candidates require an explicit Run ID: $details"
            }
            if ($valid.Count -eq 0) {
                throw "No valid $candidateTarget candidate found for $Tag"
            }
            $valid = @($valid | Sort-Object { [int64]$_.RunID } -Descending | Select-Object -First 1)
        }
        $selected = $valid[0]
        Copy-Item -LiteralPath $selected.Path -Destination (Join-Path $OutputDirectory $selected.FileName)
        Add-Content -LiteralPath $env:GITHUB_STEP_SUMMARY -Encoding utf8 -ErrorAction SilentlyContinue `
            -Value "| $candidateTarget | $($selected.RunID) | $($selected.FileName) | ``$($selected.SHA256)`` |"
        Write-Host "selected target=$candidateTarget run_id=$($selected.RunID) file=$($selected.FileName) sha256=$($selected.SHA256)"
    }
}

function Discover-Candidates {
    Assert-Identity
    if (-not $Repository.Trim()) { throw 'Discover requires Repository' }
    New-Item -ItemType Directory -Path $CandidatesDirectory -Force | Out-Null
    if ($TestRunCatalogPath) {
        $runs = @(Get-Content -Raw -LiteralPath $TestRunCatalogPath | ConvertFrom-Json)
    }
    else {
        $runsJSON = & gh run list --repo $Repository --workflow $Workflow --limit 50 `
            --json databaseId,status,conclusion,createdAt 2>&1
        if ($LASTEXITCODE -ne 0) { throw "Failed to list candidate workflow runs: $runsJSON" }
        $runs = @($runsJSON | ConvertFrom-Json)
    }
    $runs = @($runs | Where-Object {
        $_.status -eq 'completed' -and $_.conclusion -eq 'success'
    })
    if ($CurrentRunID.Trim() -and @($runs | Where-Object { [string]$_.databaseId -ceq $CurrentRunID.Trim() }).Count -eq 0) {
        $runs = @([pscustomobject]@{ databaseId = $CurrentRunID.Trim() }) + $runs
    }
    $selectedTargets = @(Resolve-CandidateTargets)
    $explicitIDs = @(@($WindowsAMD64RunID, $WindowsARM64RunID, $MacOSRunID) |
        ForEach-Object { $_.Trim() } | Where-Object { $_ })
    foreach ($explicitID in $explicitIDs) {
        if ($explicitID -notmatch '^[1-9][0-9]*$') {
            throw "Explicit candidate Run ID must contain decimal digits only: $explicitID"
        }
        if ($explicitID -cne $CurrentRunID.Trim() -and @($runs | Where-Object { [string]$_.databaseId -ceq $explicitID }).Count -eq 0) {
            if ($TestRunCatalogPath) {
                throw "Explicit candidate Run ID is not a successful completed $Workflow run: $explicitID"
            }
            $runJSON = & gh api "repos/$Repository/actions/runs/$explicitID" 2>&1
            if ($LASTEXITCODE -ne 0) { throw "Failed to query explicit candidate Run ID $explicitID`: $runJSON" }
            $run = $runJSON | ConvertFrom-Json
            $expectedPath = ".github/workflows/$Workflow"
            if (
                [string]$run.status -cne 'completed' -or
                [string]$run.conclusion -cne 'success' -or
                [string]$run.path -cne $expectedPath
            ) {
                throw "Explicit candidate Run ID $explicitID is not a successful completed $expectedPath run"
            }
            $runs += [pscustomobject]@{ databaseId = $explicitID; status = 'completed'; conclusion = 'success' }
        }
    }
    if ($TestRunCatalogPath) {
        Select-LocalCandidates
        return
    }
    foreach ($run in $runs) {
        $runID = [string]$run.databaseId
        $runDirectory = Join-Path $CandidatesDirectory $runID
        New-Item -ItemType Directory -Path $runDirectory -Force | Out-Null
        foreach ($candidateTarget in $selectedTargets) {
            $explicit = Get-ExplicitRunID $candidateTarget
            if ($explicit -and $explicit -cne $runID) { continue }
            $pattern = "desktop-candidate-$candidateTarget-$Tag"
            $output = & gh run download $runID --repo $Repository --pattern $pattern --dir $runDirectory 2>&1
            if ($LASTEXITCODE -ne 0) {
                if ($explicit) {
                    throw "Failed to download explicit $candidateTarget candidate from Run $runID`: $output"
                }
                # A successful run is allowed to omit unrelated target artifacts.
                # Clear only this explicitly tolerated native failure so it cannot
                # become the script process exit code after local selection succeeds.
                $global:LASTEXITCODE = 0
            }
        }
    }
    Select-LocalCandidates
}

switch ($Action) {
    'Write' { Write-Candidate }
    'SelectLocal' { Select-LocalCandidates }
    'Discover' { Discover-Candidates }
}

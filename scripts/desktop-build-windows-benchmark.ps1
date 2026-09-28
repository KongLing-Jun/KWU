[CmdletBinding()]
param(
    [ValidateRange(1, 99)]
    [int]$Rounds = 3,
    [ValidatePattern('^v?\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$')]
    [string]$Version = 'v0.0.0-benchmark',
    [ValidateSet('stable', 'canary')]
    [string]$Channel = 'stable',
    [string]$OutputPath = '',
    [string]$BashPath = '',
    [string]$NSISBinPath = '',
    [string]$GoCachePath = ''
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

if (-not $IsWindows) {
    throw 'The Windows desktop build benchmark can run only on Windows'
}

$repositoryRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$buildScript = 'scripts/desktop-build.sh'

function Get-BashExecutable {
    if (-not [string]::IsNullOrWhiteSpace($BashPath)) {
        if (-not (Test-Path -LiteralPath $BashPath -PathType Leaf)) {
            throw "Bash executable does not exist: $BashPath"
        }
        return [IO.Path]::GetFullPath($BashPath)
    }
    if (-not [string]::IsNullOrWhiteSpace($env:BASH_PATH)) {
        if (-not (Test-Path -LiteralPath $env:BASH_PATH -PathType Leaf)) {
            throw "BASH_PATH does not exist: $env:BASH_PATH"
        }
        return [IO.Path]::GetFullPath($env:BASH_PATH)
    }
    foreach ($candidate in @(
        (Join-Path $env:ProgramFiles 'Git\bin\bash.exe'),
        (Join-Path $env:LOCALAPPDATA 'Programs\Git\bin\bash.exe')
    )) {
        if (Test-Path -LiteralPath $candidate -PathType Leaf) { return $candidate }
    }
    $command = Get-Command bash -ErrorAction SilentlyContinue
    if ($command -and $command.Source -notmatch '(?i)\\Windows\\System32\\bash\.exe$') {
        return $command.Source
    }
    throw 'Git Bash is required for desktop-build.sh; set -BashPath or BASH_PATH'
}

function Get-NSISBinDirectory {
    if (-not [string]::IsNullOrWhiteSpace($NSISBinPath)) {
        $resolved = [IO.Path]::GetFullPath($NSISBinPath)
        if (-not (Test-Path -LiteralPath (Join-Path $resolved 'makensis.exe') -PathType Leaf)) {
            throw "NSIS Bin directory does not contain makensis.exe: $resolved"
        }
        return $resolved
    }
    $command = Get-Command makensis.exe -ErrorAction SilentlyContinue
    if ($command) { return [IO.Path]::GetFullPath((Split-Path -Parent $command.Source)) }
    foreach ($candidate in @(
        (Join-Path ${env:ProgramFiles(x86)} 'NSIS\Bin'),
        (Join-Path $env:ProgramFiles 'NSIS\Bin')
    )) {
        if ($candidate -and (Test-Path -LiteralPath (Join-Path $candidate 'makensis.exe') -PathType Leaf)) {
            return [IO.Path]::GetFullPath($candidate)
        }
    }
    throw 'NSIS makensis.exe is required; set -NSISBinPath or install NSIS in its standard location'
}

function Get-GoCacheDirectory {
    $resolved = if ([string]::IsNullOrWhiteSpace($GoCachePath)) {
        Join-Path $repositoryRoot ('.kwu\desktop-build-windows-benchmark\gocache-' + [guid]::NewGuid().ToString('N'))
    }
    elseif ([IO.Path]::IsPathRooted($GoCachePath)) {
        [IO.Path]::GetFullPath($GoCachePath)
    }
    else {
        [IO.Path]::GetFullPath((Join-Path $repositoryRoot $GoCachePath))
    }
    New-Item -ItemType Directory -Path $resolved -Force | Out-Null
    return [IO.Path]::GetFullPath($resolved)
}

function Get-TrackedState {
    $unstaged = (& git -C $repositoryRoot diff --no-ext-diff --binary -- 2>&1) -join "`n"
    if ($LASTEXITCODE -ne 0) { throw "git diff failed: $unstaged" }
    $staged = (& git -C $repositoryRoot diff --cached --no-ext-diff --binary -- 2>&1) -join "`n"
    if ($LASTEXITCODE -ne 0) { throw "git diff --cached failed: $staged" }
    return "$unstaged`n--INDEX--`n$staged"
}

function Get-Median {
    param([double[]]$Values)
    $sorted = @($Values | Sort-Object)
    $middle = [Math]::Floor($sorted.Count / 2)
    if ($sorted.Count % 2 -eq 1) { return [double]$sorted[$middle] }
    return ([double]$sorted[$middle - 1] + [double]$sorted[$middle]) / 2
}

$bash = Get-BashExecutable
$nsisBin = Get-NSISBinDirectory
$goCache = Get-GoCacheDirectory
$initialTrackedState = Get-TrackedState
$signingExisted = Test-Path Env:KWU_WINDOWS_RELEASE_SIGNING
$previousSigning = $env:KWU_WINDOWS_RELEASE_SIGNING
$pathExisted = Test-Path Env:PATH
$previousPath = $env:PATH
$goCacheExisted = Test-Path Env:GOCACHE
$previousGoCache = $env:GOCACHE
$scenarioResults = [Collections.Generic.List[object]]::new()
$benchmarkFailure = $null

try {
    $env:KWU_WINDOWS_RELEASE_SIGNING = '0'
    $env:PATH = "$nsisBin$([IO.Path]::PathSeparator)$previousPath"
    $env:GOCACHE = $goCache
    for ($round = 1; $round -le $Rounds; $round++) {
        $scenarioOrder = if ($round % 2 -eq 1) { @('baseline', 'optimized') } else { @('optimized', 'baseline') }
        $scenarioPosition = 0
        foreach ($scenario in $scenarioOrder) {
            $scenarioPosition++
            $builds = [Collections.Generic.List[object]]::new()
            $scenarioWatch = [Diagnostics.Stopwatch]::StartNew()
            foreach ($build in @(
                [ordered]@{ architecture = 'amd64'; mode = 'full' },
                [ordered]@{
                    architecture = 'arm64'
                    mode = if ($scenario -eq 'baseline') { 'full' } else { 'reuse-frontend' }
                }
            )) {
                Write-Host "==> round $round/$Rounds $scenario windows/$($build.architecture) $($build.mode)"
                $arguments = @($buildScript, "windows/$($build.architecture)", $Version, $Channel)
                if ($build.mode -eq 'reuse-frontend') { $arguments += 'reuse-frontend' }
                $buildWatch = [Diagnostics.Stopwatch]::StartNew()
                Push-Location $repositoryRoot
                try {
                    & $bash @arguments
                    $exitCode = $LASTEXITCODE
                }
                finally {
                    Pop-Location
                    $buildWatch.Stop()
                }
                if ($exitCode -ne 0) {
                    throw "desktop-build.sh failed for $scenario windows/$($build.architecture) $($build.mode) with exit code $exitCode"
                }
                $builds.Add([ordered]@{
                    architecture = $build.architecture
                    mode = $build.mode
                    durationMilliseconds = [Math]::Round($buildWatch.Elapsed.TotalMilliseconds, 3)
                })
            }
            $scenarioWatch.Stop()
            $scenarioResults.Add([ordered]@{
                round = $round
                order = $scenarioPosition
                scenario = $scenario
                durationMilliseconds = [Math]::Round($scenarioWatch.Elapsed.TotalMilliseconds, 3)
                builds = @($builds)
            })
        }
    }
}
catch {
    $benchmarkFailure = $_
}
finally {
    if ($signingExisted) {
        $env:KWU_WINDOWS_RELEASE_SIGNING = $previousSigning
    }
    else {
        Remove-Item Env:KWU_WINDOWS_RELEASE_SIGNING -ErrorAction SilentlyContinue
    }
    if ($pathExisted) {
        $env:PATH = $previousPath
    }
    else {
        Remove-Item Env:PATH -ErrorAction SilentlyContinue
    }
    if ($goCacheExisted) {
        $env:GOCACHE = $previousGoCache
    }
    else {
        Remove-Item Env:GOCACHE -ErrorAction SilentlyContinue
    }
}

$finalTrackedState = Get-TrackedState
if ($finalTrackedState -cne $initialTrackedState) {
    throw 'Benchmark child builds changed tracked source or index state; inspect the worktree before continuing'
}
if ($null -ne $benchmarkFailure) { throw $benchmarkFailure }

$baselineDurations = @($scenarioResults | Where-Object scenario -eq 'baseline' | ForEach-Object durationMilliseconds)
$optimizedDurations = @($scenarioResults | Where-Object scenario -eq 'optimized' | ForEach-Object durationMilliseconds)
$baselineMedian = Get-Median $baselineDurations
$optimizedMedian = Get-Median $optimizedDurations
$savings = $baselineMedian - $optimizedMedian
$report = [ordered]@{
    schemaVersion = 2
    generatedAtUtc = [DateTime]::UtcNow.ToString('o')
    version = $Version
    channel = $Channel
    rounds = $Rounds
    environment = [ordered]@{
        bashPath = $bash
        nsisBinPath = $nsisBin
        goCachePath = $goCache
    }
    scenarioResults = @($scenarioResults)
    summary = [ordered]@{
        baselineMedianMilliseconds = [Math]::Round($baselineMedian, 3)
        optimizedMedianMilliseconds = [Math]::Round($optimizedMedian, 3)
        savingsMilliseconds = [Math]::Round($savings, 3)
        savingsPercent = if ($baselineMedian -eq 0) { 0.0 } else { [Math]::Round(($savings / $baselineMedian) * 100, 3) }
    }
}
$json = ($report | ConvertTo-Json -Depth 7) + [Environment]::NewLine

if (-not [string]::IsNullOrWhiteSpace($OutputPath)) {
    $resolvedOutput = if ([IO.Path]::IsPathRooted($OutputPath)) {
        [IO.Path]::GetFullPath($OutputPath)
    }
    else {
        [IO.Path]::GetFullPath((Join-Path $repositoryRoot $OutputPath))
    }
    $outputDirectory = Split-Path -Parent $resolvedOutput
    if ($outputDirectory) { New-Item -ItemType Directory -Path $outputDirectory -Force | Out-Null }
    [IO.File]::WriteAllText($resolvedOutput, $json, [Text.UTF8Encoding]::new($false))
    Write-Host "JSON report: $resolvedOutput"
}

Write-Host ("Baseline median: {0:N3} ms" -f $baselineMedian)
Write-Host ("Optimized median: {0:N3} ms" -f $optimizedMedian)
Write-Host ("Savings: {0:N3} ms ({1:N3}%)" -f $savings, $report.summary.savingsPercent)
if ([string]::IsNullOrWhiteSpace($OutputPath)) { $json }

param(
    [ValidateRange(1, 20)]
    [int]$Repeat = 3,

    [ValidateRange(1, 1000)]
    [int]$StartRun = 1,

    [ValidateSet('root-go', 'root-race', 'root-coverage', 'desktop-go', 'frontend-test', 'frontend-build')]
    [string[]]$Suite = @('root-go', 'root-race', 'root-coverage', 'desktop-go', 'frontend-test', 'frontend-build'),

    [string]$OutputDirectory = '',

    [ValidateRange(1, 240)]
    [int]$TimeoutMinutes = 30,

    [switch]$PlanOnly
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot

if (-not $OutputDirectory) {
    $stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
    $OutputDirectory = Join-Path $repoRoot "output/testing-baseline/$stamp"
}
$OutputDirectory = [IO.Path]::GetFullPath($OutputDirectory)

$definitions = @{
    'root-go' = @{
        File = 'go'
        Arguments = @('test', './...')
        WorkingDirectory = $repoRoot
    }
    'root-race' = @{
        File = 'go'
        Arguments = @('test', '-race', './...')
        WorkingDirectory = $repoRoot
    }
    'root-coverage' = @{
        File = 'go'
        Arguments = @('test', '-covermode=atomic', '-coverprofile={coverage}', './...')
        WorkingDirectory = $repoRoot
    }
    'desktop-go' = @{
        File = 'go'
        Arguments = @('test', './...')
        WorkingDirectory = (Join-Path $repoRoot 'desktop')
    }
    'frontend-test' = @{
        File = 'pnpm.cmd'
        Arguments = @('test:all')
        WorkingDirectory = (Join-Path $repoRoot 'desktop/frontend')
    }
    'frontend-build' = @{
        File = 'pnpm.cmd'
        Arguments = @('build')
        WorkingDirectory = (Join-Path $repoRoot 'desktop/frontend')
    }
}

$lastRun = $StartRun + $Repeat - 1
$plan = foreach ($run in $StartRun..$lastRun) {
    foreach ($name in $Suite) {
        $definition = $definitions[$name]
        [pscustomobject]@{
            run = $run
            cache_state = if ($run -eq 1) { 'first-observation' } else { 'warm-repeat' }
            suite = $name
            file = $definition.File
            arguments = @($definition.Arguments)
            working_directory = $definition.WorkingDirectory
            timeout_minutes = $TimeoutMinutes
        }
    }
}

if ($PlanOnly) {
    $plan | ConvertTo-Json -Depth 5
    exit 0
}

New-Item -ItemType Directory -Force -Path $OutputDirectory | Out-Null
$resultsPath = Join-Path $OutputDirectory 'results.json'
$environmentPath = Join-Path $OutputDirectory 'environment.json'

$environment = [ordered]@{
    schema_version = 1
    source_commit = (& git -C $repoRoot rev-parse HEAD).Trim()
    branch = (& git -C $repoRoot branch --show-current).Trim()
    os = [Environment]::OSVersion.VersionString
    architecture = [Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
    powershell = $PSVersionTable.PSVersion.ToString()
    go = (& go version).Trim()
    node = (& node --version).Trim()
    pnpm = (& pnpm --version).Trim()
    logical_processors = [Environment]::ProcessorCount
    output_directory = $OutputDirectory
    note = 'CPU and process-tree metrics are one-second observations, not profiler totals.'
}
$environment | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $environmentPath -Encoding utf8

function Get-ProcessTreeSnapshot {
    param([int]$RootProcessId)

    $processRows = @(Get-CimInstance Win32_Process | Select-Object ProcessId, ParentProcessId)
    $ids = [Collections.Generic.HashSet[int]]::new()
    [void]$ids.Add($RootProcessId)
    $changed = $true
    while ($changed) {
        $changed = $false
        foreach ($row in $processRows) {
            if ($ids.Contains([int]$row.ParentProcessId) -and $ids.Add([int]$row.ProcessId)) {
                $changed = $true
            }
        }
    }

    $workingSet = [int64]0
    $handles = [int64]0
    $cpu = [double]0
    $observed = 0
    foreach ($id in $ids) {
        $process = Get-Process -Id $id -ErrorAction SilentlyContinue
        if (-not $process) { continue }
        $observed += 1
        $workingSet += [int64]$process.WorkingSet64
        $handles += [int64]$process.HandleCount
        if ($null -ne $process.CPU) { $cpu += [double]$process.CPU }
    }
    return [pscustomobject]@{
        process_count = $observed
        working_set_bytes = $workingSet
        handles = $handles
        observed_cpu_seconds = [math]::Round($cpu, 3)
    }
}

function Stop-BaselineProcessTree {
    param([int]$RootProcessId)

    $processRows = @(Get-CimInstance Win32_Process | Select-Object ProcessId, ParentProcessId)
    $ids = [Collections.Generic.HashSet[int]]::new()
    [void]$ids.Add($RootProcessId)
    $changed = $true
    while ($changed) {
        $changed = $false
        foreach ($row in $processRows) {
            if ($ids.Contains([int]$row.ParentProcessId) -and $ids.Add([int]$row.ProcessId)) {
                $changed = $true
            }
        }
    }
    $targets = @($ids | Sort-Object -Descending)
    foreach ($id in $targets) {
        Stop-Process -Id $id -Force -ErrorAction SilentlyContinue
    }
}

$results = [Collections.Generic.List[object]]::new()
foreach ($entry in $plan) {
    $prefix = ('run-{0:D2}-{1}' -f $entry.run, $entry.suite)
    $stdoutPath = Join-Path $OutputDirectory "$prefix.stdout.log"
    $stderrPath = Join-Path $OutputDirectory "$prefix.stderr.log"
    $coveragePath = Join-Path $OutputDirectory "$prefix.coverage.out"
    $specPath = Join-Path $OutputDirectory "$prefix.command.json"
    $exitCodePath = Join-Path $OutputDirectory "$prefix.exit-code.txt"
    $arguments = @($entry.arguments | ForEach-Object { $_.Replace('{coverage}', $coveragePath) })
    [ordered]@{
        file = $entry.file
        arguments = $arguments
        working_directory = $entry.working_directory
        exit_code_path = $exitCodePath
    } | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $specPath -Encoding utf8
    $started = Get-Date
    $stopwatch = [Diagnostics.Stopwatch]::StartNew()
    $process = Start-Process -FilePath 'powershell.exe' `
        -ArgumentList @(
            '-NoProfile',
            '-ExecutionPolicy', 'Bypass',
            '-File', (Join-Path $PSScriptRoot 'testing-baseline-command.ps1'),
            '-SpecPath', $specPath
        ) `
        -WorkingDirectory $repoRoot `
        -NoNewWindow `
        -PassThru `
        -RedirectStandardOutput $stdoutPath `
        -RedirectStandardError $stderrPath

    $peakWorkingSet = [int64]0
    $peakHandles = [int64]0
    $peakProcesses = 0
    $observedCPU = [double]0
    $timedOut = $false
    while (-not $process.HasExited) {
        if ($stopwatch.Elapsed.TotalMinutes -ge $entry.timeout_minutes) {
            $timedOut = $true
            Stop-BaselineProcessTree -RootProcessId $process.Id
            break
        }
        $snapshot = Get-ProcessTreeSnapshot -RootProcessId $process.Id
        $peakWorkingSet = [math]::Max($peakWorkingSet, $snapshot.working_set_bytes)
        $peakHandles = [math]::Max($peakHandles, $snapshot.handles)
        $peakProcesses = [math]::Max($peakProcesses, $snapshot.process_count)
        $observedCPU = [math]::Max($observedCPU, $snapshot.observed_cpu_seconds)
        Start-Sleep -Seconds 1
        $process.Refresh()
    }
    if (-not $timedOut) {
        $process.WaitForExit()
    }
    $stopwatch.Stop()
    if (-not $timedOut -and -not (Test-Path -LiteralPath $exitCodePath -PathType Leaf)) {
        throw "$($entry.suite) did not write an exit code. See $stderrPath"
    }
    $exitCode = if ($timedOut) {
        $null
    }
    else {
        [int](Get-Content -LiteralPath $exitCodePath -Encoding ascii -Raw).Trim()
    }

    $result = [ordered]@{
        run = $entry.run
        cache_state = $entry.cache_state
        suite = $entry.suite
        command = "$($entry.file) $($arguments -join ' ')"
        working_directory = $entry.working_directory
        started_at = $started.ToUniversalTime().ToString('o')
        elapsed_ms = $stopwatch.ElapsedMilliseconds
        exit_code = $exitCode
        peak_working_set_bytes = $peakWorkingSet
        peak_handles = $peakHandles
        peak_processes = $peakProcesses
        observed_cpu_seconds = $observedCPU
        stdout_bytes = if (Test-Path -LiteralPath $stdoutPath) { (Get-Item -LiteralPath $stdoutPath).Length } else { 0 }
        stderr_bytes = if (Test-Path -LiteralPath $stderrPath) { (Get-Item -LiteralPath $stderrPath).Length } else { 0 }
        coverage_bytes = if (Test-Path -LiteralPath $coveragePath) { (Get-Item -LiteralPath $coveragePath).Length } else { 0 }
        result = if ($timedOut) { 'timeout' } elseif ($exitCode -eq 0) { 'pass' } else { 'fail' }
    }
    $results.Add([pscustomobject]$result)
    $results | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $resultsPath -Encoding utf8
    Write-Host "$($entry.suite) run $($entry.run): $($result.result) in $($result.elapsed_ms) ms"
    if ($timedOut) {
        throw "$($entry.suite) run $($entry.run) timed out after $($entry.timeout_minutes) minute(s). See $stdoutPath"
    }
    if ($exitCode -ne 0) {
        throw "$($entry.suite) run $($entry.run) failed with exit code $exitCode. See $stderrPath"
    }
}

Write-Host "Testing baseline written to $OutputDirectory"

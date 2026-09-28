[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [int]$ProcessId,
    [Parameter(Mandatory = $true)]
    [string]$Executable,
    [ValidateRange(1, 604800)]
    [int]$DurationSeconds = 60,
    [ValidateRange(1, 3600)]
    [int]$IntervalSeconds = 5,
    [string]$OutputDirectory = "output/process-soak"
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$resolvedExecutable = (Resolve-Path -LiteralPath $Executable).Path
$resolvedRepoRoot = [IO.Path]::GetFullPath($repoRoot)
$repoPathPrefix = $resolvedRepoRoot + [IO.Path]::DirectorySeparatorChar
$outputRoot = [IO.Path]::GetFullPath((Join-Path $repoRoot $OutputDirectory))
if (-not $outputRoot.StartsWith($repoPathPrefix, [StringComparison]::OrdinalIgnoreCase)) {
    throw "output directory must remain inside the repository"
}

$target = Get-Process -Id $ProcessId -ErrorAction Stop
if ($target.Path -ne $resolvedExecutable) {
    throw "PID $ProcessId does not belong to the requested executable"
}

$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$runDirectory = Join-Path $outputRoot $stamp
New-Item -ItemType Directory -Force -Path $runDirectory | Out-Null
$csvPath = Join-Path $runDirectory "samples.csv"
$reportPath = Join-Path $runDirectory "report.json"
$logicalProcessors = [Environment]::ProcessorCount
$samples = [Collections.Generic.List[object]]::new()
$deadline = (Get-Date).AddSeconds($DurationSeconds)
$previousCPU = $target.TotalProcessorTime.TotalMilliseconds
$previousAt = Get-Date
$terminatedEarly = $false

function Get-DescendantCount([int]$RootPID) {
    $all = @(Get-CimInstance Win32_Process)
    $pending = [Collections.Generic.Queue[int]]::new()
    $pending.Enqueue($RootPID)
    $count = 0
    while ($pending.Count -gt 0) {
        $parent = $pending.Dequeue()
        foreach ($child in $all | Where-Object { $_.ParentProcessId -eq $parent }) {
            $count++
            $pending.Enqueue([int]$child.ProcessId)
        }
    }
    return $count
}

while ((Get-Date) -lt $deadline) {
    $target.Refresh()
    if ($target.HasExited) {
        $terminatedEarly = $true
        break
    }
    $now = Get-Date
    $cpuMilliseconds = $target.TotalProcessorTime.TotalMilliseconds
    $elapsedMilliseconds = [Math]::Max(1, ($now - $previousAt).TotalMilliseconds)
    $cpuPercent = (($cpuMilliseconds - $previousCPU) / $elapsedMilliseconds / $logicalProcessors) * 100
    $samples.Add([pscustomobject]@{
        timestamp = $now.ToUniversalTime().ToString("o")
        workingSetBytes = $target.WorkingSet64
        privateBytes = $target.PrivateMemorySize64
        handles = $target.HandleCount
        threads = $target.Threads.Count
        descendantProcesses = Get-DescendantCount -RootPID $ProcessId
        cpuPercent = [Math]::Round([Math]::Max(0, $cpuPercent), 4)
    })
    $previousCPU = $cpuMilliseconds
    $previousAt = $now
    $remaining = ($deadline - (Get-Date)).TotalSeconds
    if ($remaining -gt 0) {
        Start-Sleep -Seconds ([Math]::Min($IntervalSeconds, [Math]::Ceiling($remaining)))
    }
}

$samples | Export-Csv -LiteralPath $csvPath -NoTypeInformation -Encoding UTF8
$relativeExecutable = if ($resolvedExecutable.StartsWith($repoPathPrefix, [StringComparison]::OrdinalIgnoreCase)) {
    $resolvedExecutable.Substring($repoPathPrefix.Length).Replace([IO.Path]::DirectorySeparatorChar, '/')
} else {
    $null
}
$report = [ordered]@{
    schemaVersion = 2
    sourceCommit = (& git -C $repoRoot rev-parse HEAD).Trim()
    executableFile = [IO.Path]::GetFileName($resolvedExecutable)
    executableRelativePath = $relativeExecutable
    executableSHA256 = (Get-FileHash -LiteralPath $resolvedExecutable -Algorithm SHA256).Hash.ToLowerInvariant()
    processId = $ProcessId
    durationSeconds = $DurationSeconds
    intervalSeconds = $IntervalSeconds
    sampleCount = $samples.Count
    terminatedEarly = $terminatedEarly
    peaks = if ($samples.Count -gt 0) {
        [ordered]@{
            workingSetBytes = ($samples | Measure-Object workingSetBytes -Maximum).Maximum
            privateBytes = ($samples | Measure-Object privateBytes -Maximum).Maximum
            handles = ($samples | Measure-Object handles -Maximum).Maximum
            threads = ($samples | Measure-Object threads -Maximum).Maximum
            descendantProcesses = ($samples | Measure-Object descendantProcesses -Maximum).Maximum
            cpuPercent = ($samples | Measure-Object cpuPercent -Maximum).Maximum
        }
    } else {
        $null
    }
    samplesFile = "samples.csv"
}
$report | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath $reportPath -Encoding UTF8
$report | ConvertTo-Json -Depth 6

if ($terminatedEarly -or $samples.Count -eq 0) {
    exit 1
}

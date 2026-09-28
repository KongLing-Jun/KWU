$ErrorActionPreference = 'Stop'

$scriptPath = Join-Path $PSScriptRoot 'testing-process-soak.ps1'
$repoRoot = [IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
$outputRelative = 'output/process-soak-test/' + [guid]::NewGuid().ToString('N')
$outputAbsolute = [IO.Path]::GetFullPath((Join-Path $repoRoot $outputRelative))
$repoPrefix = $repoRoot + [IO.Path]::DirectorySeparatorChar
if (-not $outputAbsolute.StartsWith($repoPrefix, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'fixture output escaped the repository'
}

$helper = Start-Process -FilePath 'powershell.exe' `
    -ArgumentList @('-NoProfile', '-Command', 'Start-Sleep -Seconds 15') `
    -WindowStyle Hidden -PassThru
try {
    $helperPath = (Get-Process -Id $helper.Id -ErrorAction Stop).Path

    $mismatchFailed = $false
    try {
        & $scriptPath -ProcessId $helper.Id `
            -Executable (Join-Path $env:WINDIR 'System32\notepad.exe') `
            -DurationSeconds 1 -IntervalSeconds 1 -OutputDirectory $outputRelative | Out-Null
    } catch {
        $mismatchFailed = $_.Exception.Message -match 'does not belong'
    }
    if (-not $mismatchFailed) {
        throw 'collector must reject a PID/executable mismatch'
    }

    $json = & $scriptPath -ProcessId $helper.Id -Executable $helperPath `
        -DurationSeconds 2 -IntervalSeconds 1 -OutputDirectory $outputRelative
    if ($LASTEXITCODE -ne 0) {
        throw "collector exited with $LASTEXITCODE"
    }
    $report = $json | ConvertFrom-Json
    if ($report.schemaVersion -ne 2) {
        throw "unexpected report schema $($report.schemaVersion)"
    }
    if ($report.sampleCount -lt 1 -or $report.terminatedEarly) {
        throw 'collector did not persist a complete bounded sample'
    }
    if ($report.executableFile -ne [IO.Path]::GetFileName($helperPath)) {
        throw 'report executable filename does not match the verified process'
    }
    if ($null -ne $report.executableRelativePath) {
        throw 'external fixture executable must not expose an absolute or relative host path'
    }
    if ($report.executableSHA256 -notmatch '^[0-9a-f]{64}$') {
        throw 'report executable hash is not lowercase SHA-256'
    }
    if ($report.sourceCommit -ne (& git -C $repoRoot rev-parse HEAD).Trim()) {
        throw 'report source commit does not match the worktree'
    }

    $reportPath = Get-ChildItem -LiteralPath $outputAbsolute -Filter report.json -Recurse |
        Select-Object -First 1
    if ($null -eq $reportPath) {
        throw 'collector did not write report.json'
    }
    $rawReport = Get-Content -LiteralPath $reportPath.FullName -Raw -Encoding utf8
    if ($rawReport.IndexOf($helperPath, [StringComparison]::OrdinalIgnoreCase) -ge 0 -or
        $rawReport -match '(?i)commandLine|userName|userProfile') {
        throw 'archived report contains a host path or forbidden identity field'
    }
    $csvPath = Join-Path $reportPath.DirectoryName 'samples.csv'
    $samples = @(Import-Csv -LiteralPath $csvPath)
    if ($samples.Count -ne $report.sampleCount) {
        throw 'CSV sample count does not match report.json'
    }
    $expectedColumns = @(
        'timestamp',
        'workingSetBytes',
        'privateBytes',
        'handles',
        'threads',
        'descendantProcesses',
        'cpuPercent'
    )
    $actualColumns = @($samples[0].PSObject.Properties.Name)
    if (Compare-Object $expectedColumns $actualColumns) {
        throw 'CSV schema contains missing or unexpected fields'
    }
}
finally {
    Stop-Process -Id $helper.Id -Force -ErrorAction SilentlyContinue
    if (Test-Path -LiteralPath $outputAbsolute) {
        Remove-Item -LiteralPath $outputAbsolute -Recurse -Force
    }
}

Write-Host 'testing process soak fixture passed'
exit 0

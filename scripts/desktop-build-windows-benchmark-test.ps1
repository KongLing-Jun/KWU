$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$benchmark = Join-Path $PSScriptRoot 'desktop-build-windows-benchmark.ps1'
$parseErrors = $null
$null = [Management.Automation.Language.Parser]::ParseFile($benchmark, [ref]$null, [ref]$parseErrors)
if ($parseErrors.Count -ne 0) {
    throw "benchmark has parse errors: $($parseErrors | Out-String)"
}

$testRoot = Join-Path ([IO.Path]::GetTempPath()) ('desktop-build-windows-benchmark-test-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $testRoot | Out-Null
$mockBash = Join-Path $testRoot 'bash.cmd'
$explicitNSIS = Join-Path $testRoot 'explicit-nsis'
$goCache = Join-Path $testRoot 'explicit-gocache'
New-Item -ItemType Directory -Path $explicitNSIS | Out-Null
$null = New-Item -ItemType File -Path (Join-Path $testRoot 'makensis.exe')
$null = New-Item -ItemType File -Path (Join-Path $explicitNSIS 'makensis.exe')
$log = Join-Path $testRoot 'invocations.log'
[IO.File]::WriteAllText($log, '', [Text.ASCIIEncoding]::new())
[IO.File]::WriteAllText($mockBash, @'
@echo off
setlocal EnableDelayedExpansion
for /f %%C in ('find /c /v "" ^< "%KWU_BENCHMARK_TEST_LOG%"') do set count=%%C
set /a count+=1
>>"%KWU_BENCHMARK_TEST_LOG%" echo !count!^|%KWU_WINDOWS_RELEASE_SIGNING%^|%GOCACHE%^|%PATH%^|%*
if "%KWU_BENCHMARK_TEST_FAIL_AT%"=="!count!" exit /b 23
exit /b 0
'@, [Text.ASCIIEncoding]::new())

$oldLog = $env:KWU_BENCHMARK_TEST_LOG
$oldFailAt = $env:KWU_BENCHMARK_TEST_FAIL_AT
$signingExisted = Test-Path Env:KWU_WINDOWS_RELEASE_SIGNING
$oldSigning = $env:KWU_WINDOWS_RELEASE_SIGNING
$pathExisted = Test-Path Env:PATH
$oldPath = $env:PATH
$goCacheExisted = Test-Path Env:GOCACHE
$oldGoCache = $env:GOCACHE
try {
    $env:KWU_BENCHMARK_TEST_LOG = $log
    $env:KWU_WINDOWS_RELEASE_SIGNING = '1'
    $env:GOCACHE = 'caller-go-cache'
    $env:PATH = "$testRoot$([IO.Path]::PathSeparator)$oldPath"
    Remove-Item Env:KWU_BENCHMARK_TEST_FAIL_AT -ErrorAction SilentlyContinue
    $reportPath = Join-Path $testRoot 'report.json'
    & $benchmark -Rounds 3 -Version v9.8.7-benchmark -Channel stable -OutputPath $reportPath `
        -BashPath $mockBash -GoCachePath $goCache
    if ($env:KWU_WINDOWS_RELEASE_SIGNING -ne '1') {
        throw 'benchmark did not restore the caller signing environment after success'
    }
    if ($env:GOCACHE -ne 'caller-go-cache' -or $env:PATH -ne "$testRoot$([IO.Path]::PathSeparator)$oldPath") {
        throw 'benchmark did not restore PATH and GOCACHE after success'
    }

    $lines = @(Get-Content -LiteralPath $log)
    if ($lines.Count -ne 12) { throw "expected 12 mock builds, got $($lines.Count)" }
    $expected = @(
        'windows/amd64 v9.8.7-benchmark stable',
        'windows/arm64 v9.8.7-benchmark stable',
        'windows/amd64 v9.8.7-benchmark stable',
        'windows/arm64 v9.8.7-benchmark stable reuse-frontend',
        'windows/amd64 v9.8.7-benchmark stable',
        'windows/arm64 v9.8.7-benchmark stable reuse-frontend',
        'windows/amd64 v9.8.7-benchmark stable',
        'windows/arm64 v9.8.7-benchmark stable',
        'windows/amd64 v9.8.7-benchmark stable',
        'windows/arm64 v9.8.7-benchmark stable',
        'windows/amd64 v9.8.7-benchmark stable',
        'windows/arm64 v9.8.7-benchmark stable reuse-frontend'
    )
    for ($index = 0; $index -lt $lines.Count; $index++) {
        $parts = $lines[$index] -split '\|', 5
        if ($parts[1] -ne '0') { throw "child build $($index + 1) observed signing value '$($parts[1])'" }
        if ([IO.Path]::GetFullPath($parts[2]) -ne [IO.Path]::GetFullPath($goCache)) {
            throw "child build $($index + 1) observed the wrong Go cache '$($parts[2])'"
        }
        if (-not $parts[3].StartsWith("$testRoot$([IO.Path]::PathSeparator)", [StringComparison]::OrdinalIgnoreCase)) {
            throw "child build $($index + 1) did not receive the discovered NSIS directory first on PATH"
        }
        if ($parts[4] -ne "scripts/desktop-build.sh $($expected[$index])") {
            throw "unexpected build $($index + 1): $($parts[4])"
        }
    }

    $report = Get-Content -Raw -LiteralPath $reportPath | ConvertFrom-Json
    if ($report.schemaVersion -ne 2 -or $report.rounds -ne 3 -or $report.scenarioResults.Count -ne 6) {
        throw 'benchmark report schema or scenario count is invalid'
    }
    if ([IO.Path]::GetFullPath($report.environment.nsisBinPath) -ne [IO.Path]::GetFullPath($testRoot) -or
        [IO.Path]::GetFullPath($report.environment.goCachePath) -ne [IO.Path]::GetFullPath($goCache) -or
        [IO.Path]::GetFullPath($report.environment.bashPath) -ne [IO.Path]::GetFullPath($mockBash)) {
        throw 'benchmark report omitted an effective tool or cache path'
    }
    $expectedScenarioOrder = @('baseline', 'optimized', 'optimized', 'baseline', 'baseline', 'optimized')
    if ((@($report.scenarioResults.scenario) -join ',') -ne ($expectedScenarioOrder -join ',')) {
        throw 'scenario order did not alternate between rounds'
    }
    foreach ($scenario in $report.scenarioResults) {
        if ($scenario.builds.Count -ne 2 -or $scenario.builds[0].architecture -ne 'amd64' -or
            $scenario.builds[0].mode -ne 'full' -or $scenario.builds[1].architecture -ne 'arm64') {
            throw 'report omitted an architecture duration or build mode'
        }
        $expectedArmMode = if ($scenario.scenario -eq 'baseline') { 'full' } else { 'reuse-frontend' }
        if ($scenario.builds[1].mode -ne $expectedArmMode) { throw 'report recorded the wrong arm64 mode' }
        if (@($scenario.builds.durationMilliseconds | Where-Object { $_ -lt 0 }).Count) {
            throw 'report contains a negative build duration'
        }
    }
    function Test-Median([double[]]$Values) {
        $sorted = @($Values | Sort-Object)
        if ($sorted.Count % 2) { return [double]$sorted[[Math]::Floor($sorted.Count / 2)] }
        return ([double]$sorted[$sorted.Count / 2 - 1] + [double]$sorted[$sorted.Count / 2]) / 2
    }
    $baseline = Test-Median @($report.scenarioResults | Where-Object scenario -eq baseline | ForEach-Object durationMilliseconds)
    $optimized = Test-Median @($report.scenarioResults | Where-Object scenario -eq optimized | ForEach-Object durationMilliseconds)
    if ($report.summary.baselineMedianMilliseconds -ne [Math]::Round($baseline, 3) -or
        $report.summary.optimizedMedianMilliseconds -ne [Math]::Round($optimized, 3) -or
        $report.summary.savingsMilliseconds -ne [Math]::Round($baseline - $optimized, 3)) {
        throw 'report median or savings math is incorrect'
    }

    [IO.File]::WriteAllText($log, '', [Text.ASCIIEncoding]::new())
    $explicitReportPath = Join-Path $testRoot 'explicit-report.json'
    & $benchmark -Rounds 1 -Version v9.8.7 -BashPath $mockBash -NSISBinPath $explicitNSIS `
        -GoCachePath $goCache -OutputPath $explicitReportPath
    $explicitReport = Get-Content -Raw -LiteralPath $explicitReportPath | ConvertFrom-Json
    if ([IO.Path]::GetFullPath($explicitReport.environment.nsisBinPath) -ne [IO.Path]::GetFullPath($explicitNSIS)) {
        throw 'explicit NSIS Bin path was not honored or reported'
    }
    $explicitLines = @(Get-Content -LiteralPath $log)
    if ($explicitLines.Count -ne 4 -or
        -not ($explicitLines[0] -split '\|', 5)[3].StartsWith("$explicitNSIS$([IO.Path]::PathSeparator)", [StringComparison]::OrdinalIgnoreCase)) {
        throw 'explicit NSIS Bin path was not prepended for child builds'
    }
    if ($env:GOCACHE -ne 'caller-go-cache' -or $env:PATH -ne "$testRoot$([IO.Path]::PathSeparator)$oldPath") {
        throw 'benchmark did not restore PATH and GOCACHE after the explicit-path run'
    }

    [IO.File]::WriteAllText($log, '', [Text.ASCIIEncoding]::new())
    $env:KWU_BENCHMARK_TEST_FAIL_AT = '2'
    $failedClosed = $false
    try {
        & $benchmark -Rounds 1 -Version v9.8.7 -BashPath $mockBash -NSISBinPath $explicitNSIS `
            -GoCachePath $goCache -OutputPath (Join-Path $testRoot 'failed.json')
    }
    catch {
        $failedClosed = $_.Exception.Message -match 'exit code 23'
    }
    if (-not $failedClosed) { throw 'benchmark did not fail closed on a child build failure' }
    if (@(Get-Content -LiteralPath $log).Count -ne 2) { throw 'benchmark continued after a failed child build' }
    if ($env:KWU_WINDOWS_RELEASE_SIGNING -ne '1') {
        throw 'benchmark did not restore the caller signing environment after failure'
    }
    if ($env:GOCACHE -ne 'caller-go-cache' -or $env:PATH -ne "$testRoot$([IO.Path]::PathSeparator)$oldPath") {
        throw 'benchmark did not restore PATH and GOCACHE after failure'
    }
    if (Test-Path -LiteralPath (Join-Path $testRoot 'failed.json')) {
        throw 'benchmark wrote a success report after a child build failure'
    }

    $source = Get-Content -Raw -LiteralPath $benchmark
    foreach ($forbidden in @('desktop-release.ps1', 'gh ', 'ossutil', 'cloud', 'latest.json')) {
        if ($source.IndexOf($forbidden, [StringComparison]::OrdinalIgnoreCase) -ge 0) {
            throw "benchmark contains forbidden publication reference: $forbidden"
        }
    }
}
finally {
    if ($null -eq $oldLog) { Remove-Item Env:KWU_BENCHMARK_TEST_LOG -ErrorAction SilentlyContinue } else { $env:KWU_BENCHMARK_TEST_LOG = $oldLog }
    if ($null -eq $oldFailAt) { Remove-Item Env:KWU_BENCHMARK_TEST_FAIL_AT -ErrorAction SilentlyContinue } else { $env:KWU_BENCHMARK_TEST_FAIL_AT = $oldFailAt }
    if ($signingExisted) { $env:KWU_WINDOWS_RELEASE_SIGNING = $oldSigning } else { Remove-Item Env:KWU_WINDOWS_RELEASE_SIGNING -ErrorAction SilentlyContinue }
    if ($pathExisted) { $env:PATH = $oldPath } else { Remove-Item Env:PATH -ErrorAction SilentlyContinue }
    if ($goCacheExisted) { $env:GOCACHE = $oldGoCache } else { Remove-Item Env:GOCACHE -ErrorAction SilentlyContinue }
    Remove-Item -LiteralPath $testRoot -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host 'Windows desktop build benchmark tests passed'

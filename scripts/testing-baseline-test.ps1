$ErrorActionPreference = 'Stop'

$scriptPath = Join-Path $PSScriptRoot 'testing-baseline.ps1'
$json = & $scriptPath -PlanOnly -StartRun 2 -Repeat 3 `
    -Suite root-go, frontend-test -TimeoutMinutes 17
$plan = $json | ConvertFrom-Json

if ($plan.Count -ne 6) {
    throw "Expected six plan entries, got $($plan.Count)"
}
if (@($plan | Where-Object { $_.suite -eq 'root-go' }).Count -ne 3) {
    throw 'Root Go suite must appear once per repetition'
}
if (@($plan | Where-Object { $_.suite -eq 'frontend-test' }).Count -ne 3) {
    throw 'Frontend suite must appear once per repetition'
}
if ($plan[0].run -ne 2 -or $plan[-1].run -ne 4) {
    throw 'Plan must preserve the configured run-number range'
}
if (@($plan | Where-Object { $_.cache_state -ne 'warm-repeat' }).Count -ne 0) {
    throw 'Runs after the initial observation must be identified as warm repeats'
}
if (@($plan | Where-Object { $_.timeout_minutes -ne 17 }).Count -ne 0) {
    throw 'Every suite must carry the configured timeout'
}
$firstPlan = (& $scriptPath -PlanOnly -StartRun 1 -Repeat 1 -Suite root-go) |
    ConvertFrom-Json
if ($firstPlan.run -ne 1 -or $firstPlan.cache_state -ne 'first-observation') {
    throw 'Run one must remain the first observation'
}

$temp = Join-Path ([IO.Path]::GetTempPath()) ('kwu-testing-baseline-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $temp | Out-Null
try {
    $exitCodePath = Join-Path $temp 'exit-code.txt'
    $specPath = Join-Path $temp 'command.json'
    [ordered]@{
        file = 'powershell.exe'
        arguments = @('-NoProfile', '-Command', 'exit 7')
        working_directory = $temp
        exit_code_path = $exitCodePath
    } | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $specPath -Encoding utf8
    & powershell.exe -NoProfile -ExecutionPolicy Bypass `
        -File (Join-Path $PSScriptRoot 'testing-baseline-command.ps1') `
        -SpecPath $specPath
    if ($LASTEXITCODE -ne 7) {
        throw "Command wrapper returned $LASTEXITCODE instead of 7"
    }
    if ((Get-Content -LiteralPath $exitCodePath -Encoding ascii -Raw).Trim() -ne '7') {
        throw 'Command wrapper did not persist the child exit code'
    }
}
finally {
    Remove-Item -LiteralPath $temp -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host 'testing baseline plan tests passed'
exit 0

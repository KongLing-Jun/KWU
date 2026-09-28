$ErrorActionPreference = 'Stop'

$scriptPath = Join-Path $PSScriptRoot 'testing-webview2-cdp-build.ps1'
$json = & $scriptPath -PlanOnly
$plan = ($json -join "`n") | ConvertFrom-Json

if ($plan.schemaVersion -ne 1 -or $plan.wailsVersion -ne 'v2.12.0') {
    throw 'build plan did not pin its schema and Wails version'
}
if ($plan.testOnlyEnvironment -ne 'KWU_TEST_WEBVIEW2_CDP_PORT') {
    throw 'build plan does not declare the test-only WebView2 environment'
}
if ($plan.buildTool -ne 'wails v2.12.0 production mode') {
    throw 'build plan must use the pinned Wails production build path'
}
if ($plan.patchTarget -notlike '*\output\webview2-cdp-build\*\wails\internal\frontend\desktop\windows\frontend.go') {
    throw "patch target escaped the generated output tree: $($plan.patchTarget)"
}
if ($plan.binary -notlike '*\output\webview2-cdp-build\*\kwu-desktop-cdp-test-*.exe') {
    throw "binary target escaped the generated output tree: $($plan.binary)"
}
if ($plan.buildOutput -notlike '*\desktop\build\bin\kwu-desktop-cdp-test-*.exe') {
    throw "Wails build output is not a unique test executable: $($plan.buildOutput)"
}
if ($plan.sumFile -notlike '*\output\webview2-cdp-build\*\test.sum') {
    throw "sum target escaped the generated output tree: $($plan.sumFile)"
}
if ($plan.workFile -notlike '*\output\webview2-cdp-build\*\test.work') {
    throw "workspace target escaped the generated output tree: $($plan.workFile)"
}
if (@($plan.buildTags).Count -ne 1 -or $plan.buildTags[0] -ne 'testdiag') {
    throw 'test-only binary must use the testdiag build tag'
}

$escapedOutputRejected = $false
try {
    & $scriptPath -PlanOnly -OutputDirectory '..\outside' | Out-Null
} catch {
    $escapedOutputRejected = $_.Exception.Message -match 'inside the repository'
}
if (-not $escapedOutputRejected) {
    throw 'build output must not escape the repository'
}

Write-Host 'testing WebView2 CDP build plan passed'
exit 0

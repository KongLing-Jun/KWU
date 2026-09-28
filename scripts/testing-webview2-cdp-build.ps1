[CmdletBinding()]
param(
    [string]$OutputDirectory = 'output/webview2-cdp-build',
    [switch]$PlanOnly
)

$ErrorActionPreference = 'Stop'
$repoRoot = [IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
$repoPrefix = $repoRoot + [IO.Path]::DirectorySeparatorChar
$desktopRoot = Join-Path $repoRoot 'desktop'
$outputRoot = [IO.Path]::GetFullPath((Join-Path $repoRoot $OutputDirectory))
if (-not $outputRoot.StartsWith($repoPrefix, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'output directory must remain inside the repository'
}

$wailsVersion = 'v2.12.0'
$wailsModule = (& go -C $desktopRoot list -m -f '{{.Dir}}' "github.com/wailsapp/wails/v2@$wailsVersion").Trim()
if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $wailsModule -PathType Container)) {
    throw "cannot resolve Wails $wailsVersion module source"
}
$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$runRoot = Join-Path $outputRoot $stamp
$patchedWails = Join-Path $runRoot 'wails'
$modFile = Join-Path $runRoot 'test.mod'
$sumFile = Join-Path $runRoot 'test.sum'
$workFile = Join-Path $runRoot 'test.work'
$binaryName = "kwu-desktop-cdp-test-$stamp.exe"
$binaryPath = Join-Path $runRoot $binaryName
$builtBinary = Join-Path $desktopRoot "build/bin/$binaryName"
$patchTarget = Join-Path $patchedWails 'internal/frontend/desktop/windows/frontend.go'

$plan = [ordered]@{
    schemaVersion = 1
    sourceCommit = (& git -C $repoRoot rev-parse HEAD).Trim()
    wailsVersion = $wailsVersion
    wailsModule = $wailsModule
    patchedWails = $patchedWails
    patchTarget = $patchTarget
    modFile = $modFile
    sumFile = $sumFile
    workFile = $workFile
    binary = $binaryPath
    buildOutput = $builtBinary
    buildTool = 'wails v2.12.0 production mode'
    buildTags = @('testdiag')
    testOnlyEnvironment = 'KWU_TEST_WEBVIEW2_CDP_PORT'
}
if ($PlanOnly) {
    $plan | ConvertTo-Json -Depth 5
    exit 0
}

New-Item -ItemType Directory -Force -Path $runRoot | Out-Null
Copy-Item -LiteralPath $wailsModule -Destination $patchedWails -Recurse
Get-ChildItem -LiteralPath $patchedWails -Recurse -File | ForEach-Object {
    $_.IsReadOnly = $false
}

$source = Get-Content -LiteralPath $patchTarget -Raw -Encoding utf8
$anchor = @'
	if len(disableFeatues) > 0 {
'@
$injection = @'
	// Test-only copy generated outside the source tree. Production Wails remains
	// unchanged and has no environment-driven remote debugging path.
	if port := strings.TrimSpace(os.Getenv("KWU_TEST_WEBVIEW2_CDP_PORT")); port != "" {
		chromium.AdditionalBrowserArgs = append(
			chromium.AdditionalBrowserArgs,
			"--remote-debugging-address=127.0.0.1",
			"--remote-debugging-port="+port,
		)
	}

	if len(disableFeatues) > 0 {
'@
if (-not $source.Contains($anchor)) {
    throw 'Wails patch anchor was not found; refusing an unreviewed patch'
}
$patched = $source.Replace($anchor, $injection)
if ($patched -eq $source -or
    ([regex]::Matches($patched, 'KWU_TEST_WEBVIEW2_CDP_PORT')).Count -ne 1) {
    throw 'Wails test patch was not applied exactly once'
}
[IO.File]::WriteAllText($patchTarget, $patched, [Text.UTF8Encoding]::new($false))

$desktopMod = Get-Content -LiteralPath (Join-Path $desktopRoot 'go.mod') -Raw -Encoding utf8
$replacement = $patchedWails.Replace('\', '/')
$rootReplacement = $repoRoot.Replace('\', '/')
$testMod = $desktopMod.Replace('replace kwu => ../', "replace kwu => $rootReplacement").TrimEnd() +
    "`n`nreplace github.com/wailsapp/wails/v2 => $replacement`n"
[IO.File]::WriteAllText($modFile, $testMod, [Text.UTF8Encoding]::new($false))
Copy-Item -LiteralPath (Join-Path $desktopRoot 'go.sum') -Destination $sumFile
$desktopWorkspacePath = $desktopRoot.Replace('\', '/')
$workText = @"
go 1.25.0

use $desktopWorkspacePath

replace github.com/wailsapp/wails/v2 => $replacement
"@
[IO.File]::WriteAllText($workFile, $workText, [Text.UTF8Encoding]::new($false))

Push-Location $desktopRoot
$originalGoWork = $env:GOWORK
$originalGo111Module = $env:GO111MODULE
try {
    $env:GOWORK = $workFile
    $env:GO111MODULE = 'on'
    & wails build -s -m -nosyncgomod -skipbindings -nopackage `
        -tags testdiag -o $binaryName -nocolour
    if ($LASTEXITCODE -ne 0) {
        throw "test-only Wails build failed with exit code $LASTEXITCODE"
    }
}
finally {
    $env:GOWORK = $originalGoWork
    $env:GO111MODULE = $originalGo111Module
    Pop-Location
}
if (-not $builtBinary.StartsWith($repoPrefix, [StringComparison]::OrdinalIgnoreCase) -or
    -not $binaryPath.StartsWith($repoPrefix, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'refusing to move a test binary outside the repository'
}
if (-not (Test-Path -LiteralPath $builtBinary -PathType Leaf)) {
    throw 'Wails build did not produce the expected test executable'
}
Move-Item -LiteralPath $builtBinary -Destination $binaryPath
if (-not (Test-Path -LiteralPath $binaryPath -PathType Leaf)) {
    throw 'test-only Wails build did not produce an executable'
}

$report = [ordered]@{}
foreach ($key in $plan.Keys) {
    $report[$key] = $plan[$key]
}
$report.binarySHA256 = (Get-FileHash -LiteralPath $binaryPath -Algorithm SHA256).Hash.ToLowerInvariant()
$report.patchSHA256 = (Get-FileHash -LiteralPath $patchTarget -Algorithm SHA256).Hash.ToLowerInvariant()
$report | ConvertTo-Json -Depth 5 |
    Set-Content -LiteralPath (Join-Path $runRoot 'build-report.json') -Encoding utf8
$report | ConvertTo-Json -Depth 5
exit 0

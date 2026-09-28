[CmdletBinding()]
param(
    [ValidateSet("install", "browser", "runtime", "audit", "desktop", "all")]
    [string]$Suite = "all",
    [string]$BaseUrl = "http://127.0.0.1:5292",
    [string]$ExecutablePath,
    [switch]$ExerciseWindow
)

$ErrorActionPreference = "Stop"

$repositoryRoot = Split-Path -Parent $PSScriptRoot
$frontendRoot = Join-Path $repositoryRoot "desktop\frontend"
$outputRoot = Join-Path $repositoryRoot "output\user-testing"
$venvRoot = Join-Path $outputRoot ".venv"
$venvPython = Join-Path $venvRoot "Scripts\python.exe"
$requirements = Join-Path $repositoryRoot "tests\user-testing\windows\requirements.lock.txt"
$desktopSmoke = Join-Path $repositoryRoot "tests\user-testing\windows\desktop_shell_smoke.py"

if (-not $ExecutablePath) {
    $ExecutablePath = Join-Path $repositoryRoot "desktop\build\bin\kwu-desktop-dev.exe"
}

function Invoke-External {
    param(
        [Parameter(Mandatory)]
        [scriptblock]$Command,
        [Parameter(Mandatory)]
        [string]$Description
    )

    & $Command
    if ($LASTEXITCODE -ne 0) {
        throw "$Description failed with exit code $LASTEXITCODE"
    }
}

function Install-UserTestTools {
    Push-Location $frontendRoot
    try {
        Invoke-External { pnpm install --frozen-lockfile } "Frontend dependency installation"
        Invoke-External { pnpm exec playwright install chromium } "Playwright Chromium installation"
    }
    finally {
        Pop-Location
    }

    if (-not (Get-Command uv -ErrorAction SilentlyContinue)) {
        throw "uv is required to create the isolated Windows UI test environment."
    }
    New-Item -ItemType Directory -Force -Path $outputRoot | Out-Null
    if (-not (Test-Path -LiteralPath $venvPython)) {
        Invoke-External { uv venv $venvRoot --python 3.11 } "Windows UI virtual environment creation"
    }
    Invoke-External { uv pip sync --python $venvPython $requirements } "Windows UI dependency installation"
}

function Invoke-BrowserTests {
    Push-Location $frontendRoot
    try {
        Invoke-External { pnpm test:browser } "Browser smoke, accessibility, and visual tests"
    }
    finally {
        Pop-Location
    }
}

function Assert-RuntimeAvailable {
    try {
        $response = Invoke-WebRequest -Uri $BaseUrl -UseBasicParsing -TimeoutSec 10
    }
    catch {
        throw "Reasonix runtime is not reachable at $BaseUrl. Start the isolated ./dev runtime first."
    }
    if ($response.StatusCode -ne 200) {
        throw "Reasonix runtime returned HTTP $($response.StatusCode) at $BaseUrl"
    }
}

function Invoke-RuntimeTests {
    Assert-RuntimeAvailable
    Push-Location $frontendRoot
    try {
        $previousBaseUrl = $env:REASONIX_TEST_BASE_URL
        $env:REASONIX_TEST_BASE_URL = $BaseUrl
        try {
            Invoke-External { pnpm test:browser:runtime } "Real-runtime browser smoke"
        }
        finally {
            $env:REASONIX_TEST_BASE_URL = $previousBaseUrl
        }
    }
    finally {
        Pop-Location
    }
}

function Invoke-AccessibilityAudit {
    Assert-RuntimeAvailable
    Push-Location $frontendRoot
    try {
        $previousBaseUrl = $env:REASONIX_TEST_BASE_URL
        $env:REASONIX_TEST_BASE_URL = $BaseUrl
        try {
            Invoke-External { pnpm test:browser:audit } "Real-runtime accessibility audit"
        }
        finally {
            $env:REASONIX_TEST_BASE_URL = $previousBaseUrl
        }
    }
    finally {
        Pop-Location
    }
}

function Invoke-DesktopTests {
    if (-not (Test-Path -LiteralPath $venvPython)) {
        throw "Windows UI test tools are not installed. Run .\scripts\user-test.ps1 -Suite install first."
    }
    $arguments = @($desktopSmoke, "--exe", $ExecutablePath)
    if ($ExerciseWindow) {
        $arguments += "--exercise-window"
    }
    Invoke-External { & $venvPython @arguments } "Windows desktop shell smoke"
}

switch ($Suite) {
    "install" { Install-UserTestTools }
    "browser" { Invoke-BrowserTests }
    "runtime" { Invoke-RuntimeTests }
    "audit" { Invoke-AccessibilityAudit }
    "desktop" { Invoke-DesktopTests }
    "all" {
        $failures = @()
        foreach ($entry in @(
            @{ Name = "browser"; Command = { Invoke-BrowserTests } },
            @{ Name = "runtime"; Command = { Invoke-RuntimeTests } },
            @{ Name = "desktop"; Command = { Invoke-DesktopTests } },
            @{ Name = "audit"; Command = { Invoke-AccessibilityAudit } }
        )) {
            try {
                & $entry.Command
            }
            catch {
                $failures += "$($entry.Name): $($_.Exception.Message)"
            }
        }
        if ($failures.Count -gt 0) {
            throw "User test toolchain reported failures:`n$($failures -join "`n")"
        }
    }
}

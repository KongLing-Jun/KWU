[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$Executable,
    [string]$OutputDirectory = "output/webview2-cdp",
    [string]$FixtureScript = "",
    [string]$ProbeScript = "",
    [string]$CloudBaseURL = "",
    [string]$WorkBuddyRCManifest = "",
    [int]$StartupTimeoutSeconds = 60
)

$ErrorActionPreference = "Stop"
$repoRoot = [IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
$repoPrefix = $repoRoot + [IO.Path]::DirectorySeparatorChar
$resolvedExecutable = (Resolve-Path -LiteralPath $Executable).Path
$desktopRoot = Join-Path $repoRoot "desktop"
$outputRoot = [IO.Path]::GetFullPath((Join-Path $repoRoot $OutputDirectory))
if (-not $outputRoot.StartsWith($repoPrefix, [StringComparison]::OrdinalIgnoreCase)) {
    throw "output directory must remain inside the repository"
}
$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$runDirectory = Join-Path $outputRoot $stamp
$tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$tempPrefix = $tempRoot.TrimEnd([IO.Path]::DirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar
$profileRoot = Join-Path $tempRoot ("kwu-wv2-cdp-" + [guid]::NewGuid().ToString("N"))
$defaultProbe = Join-Path $repoRoot "desktop/frontend/e2e/webview2-cdp-probe.mjs"
$probe = if ([string]::IsNullOrWhiteSpace($ProbeScript)) {
    $defaultProbe
} else {
    [IO.Path]::GetFullPath((Join-Path $repoRoot $ProbeScript))
}
if (-not $probe.StartsWith($repoPrefix, [StringComparison]::OrdinalIgnoreCase) -or -not (Test-Path -LiteralPath $probe)) {
    throw "probe script must be an existing file inside the repository"
}
$fixture = $null
if (-not [string]::IsNullOrWhiteSpace($FixtureScript)) {
    $fixture = [IO.Path]::GetFullPath((Join-Path $repoRoot $FixtureScript))
    if (-not $fixture.StartsWith($repoPrefix, [StringComparison]::OrdinalIgnoreCase) -or -not (Test-Path -LiteralPath $fixture)) {
        throw "fixture script must be an existing file inside the repository"
    }
}
$stdoutPath = Join-Path $runDirectory "desktop.stdout.log"
$stderrPath = Join-Path $runDirectory "desktop.stderr.log"
$resolvedRCManifest = $null
if (-not [string]::IsNullOrWhiteSpace($WorkBuddyRCManifest)) {
    $resolvedRCManifest = (Resolve-Path -LiteralPath $WorkBuddyRCManifest).Path
}
$resolvedCloudBaseURL = $null
if (-not [string]::IsNullOrWhiteSpace($CloudBaseURL)) {
    $cloudURI = [Uri]$CloudBaseURL
    if ($cloudURI.Scheme -ne 'http' -or $cloudURI.Host -notin @('127.0.0.1', 'localhost') -or $cloudURI.Port -le 0) {
        throw 'CloudBaseURL must be an explicit loopback HTTP listener'
    }
    $resolvedCloudBaseURL = $cloudURI.AbsoluteUri
}

function Get-FreeLoopbackPort {
    $listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
    $listener.Start()
    try {
        return ([Net.IPEndPoint]$listener.LocalEndpoint).Port
    } finally {
        $listener.Stop()
    }
}

function Get-DescendantProcesses([int]$RootPID) {
    $all = @(Get-CimInstance Win32_Process)
    $pending = [Collections.Generic.Queue[int]]::new()
    $pending.Enqueue($RootPID)
    $found = [Collections.Generic.List[object]]::new()
    while ($pending.Count -gt 0) {
        $parent = $pending.Dequeue()
        foreach ($child in $all | Where-Object { $_.ParentProcessId -eq $parent }) {
            $found.Add($child)
            $pending.Enqueue([int]$child.ProcessId)
        }
    }
    return @($found)
}

$separator = [IO.Path]::DirectorySeparatorChar
if (-not $profileRoot.StartsWith($tempPrefix, [StringComparison]::OrdinalIgnoreCase)) {
    throw "profile path escaped the system temp directory"
}

New-Item -ItemType Directory -Force -Path $runDirectory | Out-Null
New-Item -ItemType Directory -Force -Path $profileRoot | Out-Null
$fixtureResult = $null
if ($null -ne $fixture) {
    $fixtureJSON = & node $fixture --profile-root $profileRoot
    if ($LASTEXITCODE -ne 0) {
        throw "profile fixture failed with exit code $LASTEXITCODE"
    }
    $fixtureResult = $fixtureJSON | ConvertFrom-Json
}
$cdpPort = Get-FreeLoopbackPort
$cloudPort = Get-FreeLoopbackPort
$cdpEndpoint = "http://127.0.0.1:$cdpPort"
$environmentNames = @(
    "KWU_CLOUD_BASE_URL",
    "KWU_CLOUD_FAKE",
    "KWU_DEV_PROFILE_ROOT",
    "KWU_TEST_WEBVIEW2_CDP_PORT",
    "WORKBUDDY_RC_MANIFEST",
    "WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS",
    "WEBVIEW2_USER_DATA_FOLDER"
)
$originalEnvironment = @{}
foreach ($name in $environmentNames) {
    $originalEnvironment[$name] = [Environment]::GetEnvironmentVariable($name, "Process")
}

$process = $null
try {
    $ownedCloudURL = if ($null -ne $resolvedCloudBaseURL) { $resolvedCloudBaseURL } else { "http://127.0.0.1:$cloudPort/" }
    [Environment]::SetEnvironmentVariable("KWU_CLOUD_BASE_URL", $ownedCloudURL, "Process")
    [Environment]::SetEnvironmentVariable("KWU_CLOUD_FAKE", $null, "Process")
    [Environment]::SetEnvironmentVariable("KWU_DEV_PROFILE_ROOT", $profileRoot, "Process")
    [Environment]::SetEnvironmentVariable("KWU_TEST_WEBVIEW2_CDP_PORT", [string]$cdpPort, "Process")
    [Environment]::SetEnvironmentVariable("WORKBUDDY_RC_MANIFEST", $resolvedRCManifest, "Process")
    [Environment]::SetEnvironmentVariable(
        "WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS",
        "--remote-debugging-address=127.0.0.1 --remote-debugging-port=$cdpPort",
        "Process"
    )
    [Environment]::SetEnvironmentVariable(
        "WEBVIEW2_USER_DATA_FOLDER",
        (Join-Path $profileRoot "webview"),
        "Process"
    )

    $process = Start-Process `
        -FilePath $resolvedExecutable `
        -WorkingDirectory $desktopRoot `
        -PassThru `
        -RedirectStandardOutput $stdoutPath `
        -RedirectStandardError $stderrPath `
        -WindowStyle Hidden

    $deadline = (Get-Date).AddSeconds($StartupTimeoutSeconds)
    $version = $null
    $webviewWithoutCDPObservedAt = $null
    while ((Get-Date) -lt $deadline) {
        if ($process.HasExited) {
            $process.WaitForExit()
            $stderrText = if (Test-Path -LiteralPath $stderrPath) {
                (Get-Content -LiteralPath $stderrPath -Raw).Trim()
            } else {
                ""
            }
            $detail = if ($stderrText -ne "") { "; stderr=$stderrText" } else { "" }
            throw "desktop process exited before CDP became ready (exit=$($process.ExitCode)$detail)"
        }
        try {
            $version = Invoke-RestMethod -Uri "$cdpEndpoint/json/version" -TimeoutSec 2
            break
        } catch {
            $webviews = @(
                Get-DescendantProcesses -RootPID $process.Id |
                    Where-Object { $_.Name -eq "msedgewebview2.exe" }
            )
            if ($webviews.Count -gt 0 -and -not ($webviews.CommandLine -match "--remote-debugging-port=$cdpPort")) {
                if ($null -eq $webviewWithoutCDPObservedAt) {
                    $webviewWithoutCDPObservedAt = Get-Date
                } elseif (((Get-Date) - $webviewWithoutCDPObservedAt).TotalSeconds -ge 2) {
                    throw "WebView2 started without the isolated CDP argument"
                }
            }
            Start-Sleep -Milliseconds 250
        }
    }
    if ($null -eq $version) {
        throw "WebView2 CDP endpoint did not become ready at $cdpEndpoint"
    }

    $rootProcess = Get-CimInstance Win32_Process -Filter "ProcessId=$($process.Id)"
    if ($null -eq $rootProcess -or $rootProcess.ExecutablePath -ne $resolvedExecutable) {
        throw "desktop PID does not belong to the requested executable"
    }
    $descendants = @(Get-DescendantProcesses -RootPID $process.Id)
    $webviews = @($descendants | Where-Object { $_.Name -eq "msedgewebview2.exe" })
    if ($webviews.Count -eq 0) {
        throw "desktop process has no WebView2 descendants"
    }
    if (-not ($webviews.CommandLine -match "--remote-debugging-port=$cdpPort")) {
        throw "WebView2 command line does not contain the isolated CDP port"
    }

    $probeJSON = & node $probe --endpoint $cdpEndpoint --output $runDirectory
    if ($LASTEXITCODE -ne 0) {
        throw "Playwright CDP probe failed with exit code $LASTEXITCODE"
    }
    $probeResult = $probeJSON | ConvertFrom-Json
    $relativeExecutable = if ($resolvedExecutable.StartsWith($repoPrefix, [StringComparison]::OrdinalIgnoreCase)) {
        $resolvedExecutable.Substring($repoPrefix.Length).Replace([IO.Path]::DirectorySeparatorChar, '/')
    } else {
        $null
    }
    $report = [ordered]@{
        schemaVersion = 3
        sourceCommit = (& git -C $repoRoot rev-parse HEAD).Trim()
        executableFile = [IO.Path]::GetFileName($resolvedExecutable)
        executableRelativePath = $relativeExecutable
        executableSHA256 = (Get-FileHash -LiteralPath $resolvedExecutable -Algorithm SHA256).Hash.ToLowerInvariant()
        rootPID = $process.Id
        cdpAddress = $cdpEndpoint
        cdpBrowser = $version.Browser
        cdpProtocolVersion = $version."Protocol-Version"
        webviewPIDs = @($webviews | ForEach-Object { $_.ProcessId })
        profileScope = "isolated-system-temp"
        fixture = $fixtureResult
        playwright = $probeResult
    }
    $report | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $runDirectory "report.json") -Encoding UTF8
    $report | ConvertTo-Json -Depth 8
} finally {
    if ($null -ne $process) {
        $descendants = @(Get-DescendantProcesses -RootPID $process.Id)
        [array]::Reverse($descendants)
        foreach ($child in $descendants) {
            Stop-Process -Id $child.ProcessId -Force -ErrorAction SilentlyContinue
        }
        Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
        $knownIDs = @($descendants | ForEach-Object { [int]$_.ProcessId }) + @([int]$process.Id)
        foreach ($id in $knownIDs) {
            Wait-Process -Id $id -Timeout 5 -ErrorAction SilentlyContinue
        }
        $orphanDeadline = (Get-Date).AddSeconds(10)
        do {
            $orphans = @(
                Get-CimInstance Win32_Process |
                    Where-Object {
                        $_.CommandLine -and
                        $_.CommandLine.IndexOf($profileRoot, [StringComparison]::OrdinalIgnoreCase) -ge 0
                    }
            )
            foreach ($orphan in $orphans) {
                Stop-Process -Id $orphan.ProcessId -Force -ErrorAction SilentlyContinue
            }
            if ($orphans.Count -gt 0) {
                Start-Sleep -Milliseconds 250
            }
        } while ($orphans.Count -gt 0 -and (Get-Date) -lt $orphanDeadline)
        Start-Sleep -Seconds 1
    }
    foreach ($name in $environmentNames) {
        [Environment]::SetEnvironmentVariable($name, $originalEnvironment[$name], "Process")
    }
    if (Test-Path -LiteralPath $profileRoot) {
        $resolvedProfile = [IO.Path]::GetFullPath($profileRoot)
        if (-not $resolvedProfile.StartsWith($tempPrefix, [StringComparison]::OrdinalIgnoreCase)) {
            throw "refusing to clean profile outside the system temp directory"
        }
        $cleanupDeadline = (Get-Date).AddSeconds(30)
        $cleanupError = $null
        do {
            try {
                Remove-Item -LiteralPath $resolvedProfile -Recurse -Force -ErrorAction Stop
            } catch {
                $cleanupError = $_.Exception.Message
                if ((Get-Date) -ge $cleanupDeadline) {
                    throw "isolated WebView2 profile cleanup failed: $cleanupError"
                }
                Start-Sleep -Milliseconds 250
            }
        } while (Test-Path -LiteralPath $resolvedProfile)
    }
}

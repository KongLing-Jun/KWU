[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [int]$ProcessID,
    [Parameter(Mandatory = $true)]
    [string]$ExpectedExecutable,
    [string]$ExpectedWindowClass = "kwuDesktopWindow",
    [string]$OutputPath = "output/windows-native-smoke/report.json",
    [int]$TimeoutSeconds = 20
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$expectedPath = (Resolve-Path -LiteralPath $ExpectedExecutable).Path
$processInfo = Get-CimInstance Win32_Process -Filter "ProcessId=$ProcessID"
if ($null -eq $processInfo -or $processInfo.ExecutablePath -ne $expectedPath) {
    throw "PID $ProcessID does not belong to $expectedPath"
}

Add-Type @"
using System;
using System.Text;
using System.Runtime.InteropServices;
public static class ReasonixNativeWindowProbe {
    [DllImport("user32.dll", CharSet = CharSet.Unicode)]
    public static extern int GetClassName(IntPtr hWnd, StringBuilder text, int count);
    [DllImport("user32.dll", CharSet = CharSet.Unicode)]
    public static extern int GetWindowText(IntPtr hWnd, StringBuilder text, int count);
    [DllImport("user32.dll")]
    public static extern bool IsWindowVisible(IntPtr hWnd);
}
"@

$deadline = (Get-Date).AddSeconds($TimeoutSeconds)
$handle = [IntPtr]::Zero
while ((Get-Date) -lt $deadline) {
    $process = Get-Process -Id $ProcessID -ErrorAction Stop
    $process.Refresh()
    $handle = $process.MainWindowHandle
    if ($handle -ne [IntPtr]::Zero) {
        break
    }
    Start-Sleep -Milliseconds 200
}
if ($handle -eq [IntPtr]::Zero) {
    throw "PID $ProcessID did not expose a native main window"
}

$classBuffer = [Text.StringBuilder]::new(256)
$titleBuffer = [Text.StringBuilder]::new(512)
[void][ReasonixNativeWindowProbe]::GetClassName($handle, $classBuffer, $classBuffer.Capacity)
[void][ReasonixNativeWindowProbe]::GetWindowText($handle, $titleBuffer, $titleBuffer.Capacity)
$windowClass = $classBuffer.ToString()
$title = $titleBuffer.ToString()
$visible = [ReasonixNativeWindowProbe]::IsWindowVisible($handle)

if ($windowClass -ne $ExpectedWindowClass) {
    throw "window class = $windowClass, want $ExpectedWindowClass"
}
if (-not $visible) {
    throw "native main window is not visible"
}
if ([string]::IsNullOrWhiteSpace($title)) {
    throw "native main window title is empty"
}

$resolvedOutput = [IO.Path]::GetFullPath((Join-Path $repoRoot $OutputPath))
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $resolvedOutput) | Out-Null
$report = [ordered]@{
    schemaVersion = 1
    sourceCommit = (& git -C $repoRoot rev-parse HEAD).Trim()
    processID = $ProcessID
    executable = $expectedPath
    windowHandle = $handle.ToInt64()
    windowClass = $windowClass
    title = $title
    visible = $visible
}
$report | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath $resolvedOutput -Encoding UTF8
$report | ConvertTo-Json -Depth 4

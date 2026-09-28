[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [int]$ProcessID,
    [Parameter(Mandatory = $true)]
    [string]$ExpectedExecutable,
    [string]$OutputPath = "output/windows-uia-tree/report.json",
    [ValidateRange(1, 10000)]
    [int]$MaxNodes = 2000,
    [ValidateRange(1, 128)]
    [int]$MaxDepth = 32,
    [switch]$IncludeNames,
    [int]$TimeoutSeconds = 20
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$expectedPath = (Resolve-Path -LiteralPath $ExpectedExecutable).Path
$processInfo = Get-CimInstance Win32_Process -Filter "ProcessId=$ProcessID"
if ($null -eq $processInfo -or $processInfo.ExecutablePath -ne $expectedPath) {
    throw "PID $ProcessID does not belong to $expectedPath"
}

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

Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes

$root = [System.Windows.Automation.AutomationElement]::FromHandle($handle)
if ($null -eq $root) {
    throw "UI Automation could not resolve window handle $($handle.ToInt64())"
}
if ($root.Current.ProcessId -ne $ProcessID) {
    throw "UIA root process = $($root.Current.ProcessId), want $ProcessID"
}

$walker = [System.Windows.Automation.TreeWalker]::RawViewWalker
$queue = New-Object System.Collections.Queue
$queue.Enqueue([pscustomobject]@{
    Element = $root
    Depth = 0
    ParentIndex = $null
})
$nodes = @()
$skippedElements = 0
$truncated = $false

while ($queue.Count -gt 0) {
    if ($nodes.Count -ge $MaxNodes) {
        $truncated = $true
        break
    }

    $entry = $queue.Dequeue()
    $element = $entry.Element
    try {
        $current = $element.Current
        $node = [ordered]@{
            index = $nodes.Count
            parentIndex = $entry.ParentIndex
            depth = [int]$entry.Depth
            processID = $current.ProcessId
            controlType = $current.ControlType.ProgrammaticName -replace "^ControlType\.", ""
            automationID = $current.AutomationId
            className = $current.ClassName
            enabled = $current.IsEnabled
            offscreen = $current.IsOffscreen
            namePresent = -not [string]::IsNullOrWhiteSpace($current.Name)
        }
        if ($IncludeNames) {
            $node.name = $current.Name
        }
        $nodes += [pscustomobject]$node
    } catch {
        $skippedElements += 1
        continue
    }

    if ($entry.Depth -ge $MaxDepth) {
        if ($null -ne $walker.GetFirstChild($element)) {
            $truncated = $true
        }
        continue
    }

    $parentIndex = $nodes.Count - 1
    $child = $walker.GetFirstChild($element)
    while ($null -ne $child) {
        $queue.Enqueue([pscustomobject]@{
            Element = $child
            Depth = [int]$entry.Depth + 1
            ParentIndex = $parentIndex
        })
        $child = $walker.GetNextSibling($child)
    }
}

if ($nodes.Count -eq 0) {
    throw "UI Automation returned an empty tree"
}

$controlTypes = @(
    $nodes |
        Group-Object -Property controlType |
        Sort-Object -Property Count -Descending |
        ForEach-Object {
            [ordered]@{
                controlType = $_.Name
                count = $_.Count
            }
        }
)
$processIDs = @($nodes.processID | Sort-Object -Unique)
$maximumDepth = ($nodes.depth | Measure-Object -Maximum).Maximum

$resolvedOutput = [IO.Path]::GetFullPath((Join-Path $repoRoot $OutputPath))
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $resolvedOutput) | Out-Null
$report = [ordered]@{
    schemaVersion = 1
    sourceCommit = (& git -C $repoRoot rev-parse HEAD).Trim()
    processID = $ProcessID
    executable = $expectedPath
    windowHandle = $handle.ToInt64()
    namesIncluded = [bool]$IncludeNames
    nodeCount = $nodes.Count
    maximumDepth = $maximumDepth
    processIDs = $processIDs
    controlTypes = $controlTypes
    skippedElements = $skippedElements
    truncated = $truncated
    nodes = $nodes
}
$report | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $resolvedOutput -Encoding UTF8

[ordered]@{
    schemaVersion = $report.schemaVersion
    sourceCommit = $report.sourceCommit
    processID = $report.processID
    executable = $report.executable
    namesIncluded = $report.namesIncluded
    nodeCount = $report.nodeCount
    maximumDepth = $report.maximumDepth
    processIDs = $report.processIDs
    controlTypes = $report.controlTypes
    skippedElements = $report.skippedElements
    truncated = $report.truncated
    outputPath = $resolvedOutput
} | ConvertTo-Json -Depth 6

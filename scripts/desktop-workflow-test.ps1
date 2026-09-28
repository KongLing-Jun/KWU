$ErrorActionPreference = 'Stop'

$workflowPaths = @(
    Join-Path $PSScriptRoot '..\.github\workflows\desktop-release-doctor.yml'
    Join-Path $PSScriptRoot '..\.github\workflows\desktop-build-draft.yml'
    Join-Path $PSScriptRoot '..\.github\workflows\desktop-publish.yml'
    Join-Path $PSScriptRoot '..\.github\workflows\desktop-withdraw.yml'
)

$obsoleteWorkflow = Join-Path $PSScriptRoot '..\.github\workflows\release-desktop.yml'
if (Test-Path -LiteralPath $obsoleteWorkflow) {
    throw "Obsolete desktop workflow must remain retired: $obsoleteWorkflow"
}

function Assert-NoInputExpressionInRunScript {
    param([string]$Path)
    $runIndent = -1
    foreach ($line in Get-Content -LiteralPath $Path) {
        $indent = $line.Length - $line.TrimStart().Length
        if ($runIndent -ge 0 -and $line.Trim() -and $indent -le $runIndent) { $runIndent = -1 }
        if ($line -match '^(\s*)run:\s*(?:\||>-)?\s*$') {
            $runIndent = $Matches[1].Length
            continue
        }
        if ($runIndent -ge 0 -and $line.Contains('${{ inputs.')) {
            throw "workflow_dispatch input is interpolated directly into a run script: $Path"
        }
    }
}

foreach ($path in $workflowPaths) {
    $workflow = Get-Content -Raw -LiteralPath $path
    if ($workflow -notmatch '(?m)^permissions:\s*\r?\n\s{2}contents:\s+read\s*$') {
        throw "Workflow does not default to contents: read: $path"
    }
    if ($workflow -notmatch 'cancel-in-progress:\s+false') {
        throw "Workflow concurrency must never cancel an in-progress release: $path"
    }
    foreach ($line in ($workflow -split "`r?`n" | Where-Object { $_ -match '^\s*-?\s*uses:' })) {
        if ($line -notmatch 'uses:\s+[^\s@]+@[0-9a-f]{40}(?:\s+#.*)?$') {
            throw "Action is not pinned to a full commit SHA: $line"
        }
    }
    if ($workflow -match 'uses:\s+[^\s]+@v[0-9]') {
        throw "Mutable major action reference found: $path"
    }
    if ($workflow -match 'ref:\s+main-v3') {
        throw "Trusted tooling checkout uses moving main-v3 instead of the event SHA: $path"
    }
    Assert-NoInputExpressionInRunScript $path
}

$powerShellPaths = @(
    'desktop-actions-base.ps1',
    'desktop-actions-common.ps1',
    'desktop-candidates.ps1',
    'desktop-candidates-test.ps1',
    'desktop-actions-doctor.ps1',
    'desktop-actions-draft.ps1',
    'desktop-actions-publish.ps1',
    'desktop-actions-withdraw.ps1'
) | ForEach-Object { Join-Path $PSScriptRoot $_ }
foreach ($path in $powerShellPaths) {
    $tokens = $null
    $errors = $null
    [Management.Automation.Language.Parser]::ParseFile($path, [ref]$tokens, [ref]$errors) | Out-Null
    if ($errors.Count) { throw "PowerShell parse failed for $path`: $($errors[0].Message)" }
}

Write-Host 'desktop workflow policy and syntax tests passed'

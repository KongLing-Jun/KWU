[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [string]$InstallerPath,
    [switch]$AllowSystemMutation,
    [ValidateRange(1, 30)]
    [int]$WaitSeconds = 4
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$resolvedInstaller = (Resolve-Path -LiteralPath $InstallerPath).Path
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$testRoot = Join-Path $repoRoot '.tmp-installer-brand-migration-smoke'
$legacyDirectory = Join-Path $testRoot 'D drive legacy install with spaces'
$legacyKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\ReasonixReasonix'
$canonicalKey = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\ClouviaCloudOffice'

if (-not $AllowSystemMutation) {
    throw 'Run this smoke only in a disposable Windows user or VM and pass -AllowSystemMutation. If migration regresses to silent mode, NSIS Known Folder resolution may write to the real per-user Programs directory.'
}
if (Test-Path -LiteralPath $legacyKey) {
    throw "Refusing to overwrite an existing legacy uninstall key: $legacyKey"
}
if (Test-Path -LiteralPath $canonicalKey) {
    throw "Refusing to overwrite an existing canonical uninstall key: $canonicalKey"
}
if (Test-Path -LiteralPath $testRoot) {
    throw "Refusing to overwrite an existing smoke directory: $testRoot"
}

$installerProcess = $null
try {
    New-Item -ItemType Directory -Path $legacyDirectory | Out-Null
    [IO.File]::WriteAllText((Join-Path $legacyDirectory 'uninstall.exe'), 'fixture')
    [IO.File]::WriteAllText((Join-Path $legacyDirectory 'kwu-desktop.exe'), 'fixture')

    New-Item -Path $legacyKey | Out-Null
    New-ItemProperty -LiteralPath $legacyKey -Name InstallLocation -Value $legacyDirectory -PropertyType String |
        Out-Null
    New-ItemProperty -LiteralPath $legacyKey -Name DisplayIcon `
        -Value (Join-Path $legacyDirectory 'kwu-desktop.exe') -PropertyType String | Out-Null

    # NSIS resolves $LOCALAPPDATA through a Windows Known Folder, so environment
    # overrides cannot sandbox the canonical destination. This smoke is safe
    # only in the disposable Windows user or VM required by -AllowSystemMutation.
    $installerProcess = Start-Process -FilePath $resolvedInstaller -ArgumentList @(
        '/S',
        '/UPDATEPID=999999',
        "/D=$legacyDirectory"
    ) -PassThru

    Start-Sleep -Seconds $WaitSeconds
    $installerProcess.Refresh()
    $interactive = -not $installerProcess.HasExited -and $installerProcess.MainWindowHandle -ne 0
    if (-not $interactive) {
        throw 'Legacy /S /UPDATEPID /D launch did not surface an interactive NSIS window'
    }

    Write-Host ("Verified interactive brand migration from path with spaces: {0} ({1})" -f `
        $legacyDirectory, $installerProcess.MainWindowTitle)
}
finally {
    if ($installerProcess -and -not $installerProcess.HasExited) {
        Stop-Process -Id $installerProcess.Id -Force -ErrorAction SilentlyContinue
        $installerProcess.WaitForExit(5000) | Out-Null
    }
    if (Test-Path -LiteralPath $legacyKey) {
        Remove-Item -LiteralPath $legacyKey -Recurse -Force
    }
    if (Test-Path -LiteralPath $canonicalKey) {
        Remove-Item -LiteralPath $canonicalKey -Recurse -Force
    }
    if (Test-Path -LiteralPath $testRoot) {
        $resolvedTestRoot = (Resolve-Path -LiteralPath $testRoot).Path
        if (-not $resolvedTestRoot.StartsWith(
            $repoRoot + [IO.Path]::DirectorySeparatorChar,
            [StringComparison]::OrdinalIgnoreCase
        )) {
            throw "Unsafe smoke cleanup target: $resolvedTestRoot"
        }
        Remove-Item -LiteralPath $resolvedTestRoot -Recurse -Force
    }
}

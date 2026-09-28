[CmdletBinding()]
param(
  [switch]$StatusOnly,
  [switch]$Restart,
  [switch]$RestartDesktop,
  [Parameter(DontShow = $true)]
  [switch]$RunDesktopChild
)

$ErrorActionPreference = 'Stop'

$desktopRoot = 'D:\project\Reasonix-worktrees\integration-main-v3-20260809'
$cloudRoot = 'D:\project\reasonix-cloud-platform\Reasonix-Cloud-worktrees\integration-20260809'
$adminRoot = 'D:\project\reasonix-cloud-platform\naive-ui-admin-worktrees\integration-20260809'
$desktopVitePort = 5175
$desktopWailsPort = 34238
$cloudPort = 9090
$adminPort = 5173

if ($RunDesktopChild) {
  $env:KWU_DESKTOP_VITE_PORT = [string]$desktopVitePort
  $env:KWU_DESKTOP_WAILS_PORT = [string]$desktopWailsPort
  $env:KWU_CLOUD_BASE_URL = "http://127.0.0.1:$cloudPort"
  $wails = (Get-Command wails -ErrorAction Stop).Source
  Set-Location (Join-Path $desktopRoot 'desktop')
  & $wails dev -nosyncgomod -m -skipbindings -s -devserver "127.0.0.1:$desktopWailsPort" -nocolour
  exit $LASTEXITCODE
}

function Assert-Worktree {
  param(
    [string]$Root,
    [string]$ExpectedBranch,
    [string]$Name
  )

  if (-not (Test-Path -LiteralPath $Root -PathType Container)) {
    throw "$Name worktree does not exist: $Root"
  }
  $branch = (& git -C $Root branch --show-current).Trim()
  if ($LASTEXITCODE -ne 0 -or $branch -ne $ExpectedBranch) {
    throw "$Name must use branch '$ExpectedBranch'; found '$branch'"
  }
  $head = (& git -C $Root rev-parse --short=12 HEAD).Trim()
  Write-Host "[worktree] $Name $branch @ $head"
}

function Test-ExpectedPortOwner {
  param(
    [int]$Port,
    [string]$ExpectedRoot,
    [string]$Name
  )

  $listener = Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue |
    Select-Object -First 1
  if (-not $listener) {
    return $false
  }
  $process = Get-CimInstance Win32_Process -Filter "ProcessId = $($listener.OwningProcess)"
  $identity = "$($process.ExecutablePath)`n$($process.CommandLine)"
  if ($identity.IndexOf($ExpectedRoot, [StringComparison]::OrdinalIgnoreCase) -lt 0) {
    throw "$Name port $Port is owned by unexpected PID $($process.ProcessId): $($process.Name)"
  }
  Write-Host "[running] $Name port $Port (PID $($process.ProcessId))"
  return $true
}

function Stop-ExpectedPortOwner {
  param(
    [int]$Port,
    [string]$ExpectedRoot,
    [string]$Name
  )

  $listener = Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue |
    Select-Object -First 1
  if (-not $listener) { return }
  $rootProcess = Get-CimInstance Win32_Process -Filter "ProcessId = $($listener.OwningProcess)"
  $identity = "$($rootProcess.ExecutablePath)`n$($rootProcess.CommandLine)"
  if ($identity.IndexOf($ExpectedRoot, [StringComparison]::OrdinalIgnoreCase) -lt 0) {
    throw "$Name port $Port is owned by unexpected PID $($rootProcess.ProcessId): $($rootProcess.Name)"
  }
  $processes = @(Get-CimInstance Win32_Process)
  $tree = [Collections.Generic.List[int]]::new()
  $pending = [Collections.Generic.Queue[int]]::new()
  $pending.Enqueue([int]$rootProcess.ProcessId)
  while ($pending.Count -gt 0) {
    $parent = $pending.Dequeue()
    foreach ($child in $processes | Where-Object { [int]$_.ParentProcessId -eq $parent }) {
      $pending.Enqueue([int]$child.ProcessId)
      $tree.Add([int]$child.ProcessId)
    }
  }
  for ($index = $tree.Count - 1; $index -ge 0; $index--) {
    Stop-Process -Id $tree[$index] -Force -ErrorAction SilentlyContinue
  }
  Stop-Process -Id ([int]$rootProcess.ProcessId) -Force -ErrorAction Stop
  $deadline = (Get-Date).AddSeconds(15)
  while ((Get-Date) -lt $deadline -and (Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue)) {
    Start-Sleep -Milliseconds 200
  }
  if (Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue) {
    throw "$Name port $Port did not stop"
  }
  Write-Host "[stopped] $Name port $Port (PID $($rootProcess.ProcessId))"
}

function Wait-HttpOk {
  param(
    [string]$Name,
    [string]$Uri,
    [int]$TimeoutSeconds = 120
  )

  $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
  $lastError = $null
  while ((Get-Date) -lt $deadline) {
    try {
      $response = Invoke-WebRequest -Uri $Uri -UseBasicParsing -TimeoutSec 5
      if ([int]$response.StatusCode -ge 200 -and [int]$response.StatusCode -lt 400) {
        Write-Host "[healthy] $Name $Uri -> $($response.StatusCode)"
        return
      }
      $lastError = "HTTP $($response.StatusCode)"
    } catch {
      $lastError = $_.Exception.Message
    }
    Start-Sleep -Seconds 2
  }
  throw "$Name did not become healthy within ${TimeoutSeconds}s: $Uri ($lastError)"
}

function Start-Cloud {
  param([string]$RunDirectory)

  $executable = Join-Path $RunDirectory 'reasonix-cloud.exe'
  Write-Host '[build] Cloud go build ./cmd/server'
  & go -C $cloudRoot build -o $executable ./cmd/server
  if ($LASTEXITCODE -ne 0) {
    throw "Cloud build failed with exit code $LASTEXITCODE"
  }
  $stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
  $process = Start-Process -FilePath $executable `
    -WorkingDirectory $cloudRoot `
    -WindowStyle Hidden `
    -PassThru `
    -RedirectStandardOutput (Join-Path $RunDirectory "cloud-$stamp.out.log") `
    -RedirectStandardError (Join-Path $RunDirectory "cloud-$stamp.err.log")
  Write-Host "[started] Cloud PID $($process.Id)"
}

function Start-Admin {
  param([string]$RunDirectory)

  $vite = Join-Path $adminRoot 'node_modules\vite\bin\vite.js'
  if (-not (Test-Path -LiteralPath $vite -PathType Leaf)) {
    throw "Admin dependencies are missing: $vite"
  }
  $stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
  $process = Start-Process -FilePath (Get-Command node -ErrorAction Stop).Source `
    -ArgumentList @($vite, '--host', '127.0.0.1', '--port', [string]$adminPort, '--strictPort') `
    -WorkingDirectory $adminRoot `
    -WindowStyle Hidden `
    -PassThru `
    -RedirectStandardOutput (Join-Path $RunDirectory "admin-$stamp.out.log") `
    -RedirectStandardError (Join-Path $RunDirectory "admin-$stamp.err.log")
  Write-Host "[started] Admin PID $($process.Id)"
}

function Start-Desktop {
  param([string]$RunDirectory)

  $modules = Join-Path $desktopRoot 'desktop\frontend\node_modules'
  if (-not (Test-Path -LiteralPath $modules -PathType Container)) {
    throw "Desktop dependencies are missing: $modules"
  }
  Get-Command wails -ErrorAction Stop | Out-Null
  $stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
  $process = Start-Process -FilePath 'powershell.exe' `
    -ArgumentList @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $PSCommandPath, '-RunDesktopChild') `
    -WorkingDirectory $desktopRoot `
    -WindowStyle Hidden `
    -PassThru `
    -RedirectStandardOutput (Join-Path $RunDirectory "desktop-$stamp.out.log") `
    -RedirectStandardError (Join-Path $RunDirectory "desktop-$stamp.err.log")
  Write-Host "[started] Desktop launcher PID $($process.Id)"
}

Assert-Worktree -Root $desktopRoot -ExpectedBranch 'codex/integration-main-v3-20260809' -Name 'Desktop'
Assert-Worktree -Root $cloudRoot -ExpectedBranch 'codex/integration-20260809' -Name 'Cloud'
Assert-Worktree -Root $adminRoot -ExpectedBranch 'codex/integration-20260809' -Name 'Admin'

if ($StatusOnly -and ($Restart -or $RestartDesktop)) {
  throw 'StatusOnly cannot be combined with a restart option'
}
if ($Restart -and $RestartDesktop) {
  throw 'Restart and RestartDesktop cannot be used together'
}
if ($Restart -or $RestartDesktop) {
  Stop-ExpectedPortOwner -Port $desktopWailsPort -ExpectedRoot $desktopRoot -Name 'Desktop Wails'
  Stop-ExpectedPortOwner -Port $desktopVitePort -ExpectedRoot $desktopRoot -Name 'Desktop Vite'
}
if ($Restart) {
  Stop-ExpectedPortOwner -Port $adminPort -ExpectedRoot $adminRoot -Name 'Admin'
  Stop-ExpectedPortOwner -Port $cloudPort -ExpectedRoot $cloudRoot -Name 'Cloud'
}

$cloudRunning = Test-ExpectedPortOwner -Port $cloudPort -ExpectedRoot $cloudRoot -Name 'Cloud'
$adminRunning = Test-ExpectedPortOwner -Port $adminPort -ExpectedRoot $adminRoot -Name 'Admin'
$desktopViteRunning = Test-ExpectedPortOwner -Port $desktopVitePort -ExpectedRoot $desktopRoot -Name 'Desktop Vite'
$desktopWailsRunning = Test-ExpectedPortOwner -Port $desktopWailsPort -ExpectedRoot $desktopRoot -Name 'Desktop Wails'

if ($desktopViteRunning -xor $desktopWailsRunning) {
  throw 'Desktop is partially running. Stop its owned process tree before retrying.'
}
if ($StatusOnly -and -not ($cloudRunning -and $adminRunning -and $desktopViteRunning -and $desktopWailsRunning)) {
  throw 'One or more integration services are not running.'
}

if (-not $StatusOnly) {
  $desktopRunDirectory = Join-Path $desktopRoot '.codex-run\integration-launcher'
  $cloudRunDirectory = Join-Path $cloudRoot '.codex-run\integration-launcher'
  $adminRunDirectory = Join-Path $adminRoot '.codex-run\integration-launcher'
  New-Item -ItemType Directory -Path $desktopRunDirectory, $cloudRunDirectory, $adminRunDirectory -Force | Out-Null

  if (-not $cloudRunning) {
    Start-Cloud -RunDirectory $cloudRunDirectory
  }
  Wait-HttpOk -Name 'Cloud health' -Uri "http://127.0.0.1:$cloudPort/healthz"
  Wait-HttpOk -Name 'Cloud readiness' -Uri "http://127.0.0.1:$cloudPort/readyz"

  if (-not $adminRunning) {
    Start-Admin -RunDirectory $adminRunDirectory
  }
  if (-not $desktopViteRunning) {
    Start-Desktop -RunDirectory $desktopRunDirectory
  }
}

if ($StatusOnly) {
  Wait-HttpOk -Name 'Cloud health' -Uri "http://127.0.0.1:$cloudPort/healthz"
  Wait-HttpOk -Name 'Cloud readiness' -Uri "http://127.0.0.1:$cloudPort/readyz"
}
Wait-HttpOk -Name 'Admin' -Uri "http://127.0.0.1:$adminPort/"
Wait-HttpOk -Name 'Admin proxy' -Uri "http://127.0.0.1:$adminPort/__proxy/healthz"
Wait-HttpOk -Name 'Admin proxy config' -Uri "http://127.0.0.1:$adminPort/__proxy/api/v1/auth/captcha-config"
Wait-HttpOk -Name 'Desktop Vite' -Uri "http://127.0.0.1:$desktopVitePort/"
Wait-HttpOk -Name 'Desktop Wails' -Uri "http://127.0.0.1:$desktopWailsPort/"

Write-Host ''
Write-Host "Desktop Cloud endpoint: http://127.0.0.1:$cloudPort"
Write-Host 'Reasonix 20260809 integration stack is ready.'

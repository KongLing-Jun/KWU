[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$Executable,
    [Parameter(Mandatory = $true)]
    [string]$CloudRepository,
    [Parameter(Mandatory = $true)]
    [string]$AdminRepository,
    [Parameter(Mandatory = $true)]
    [string]$RCLock,
    [string]$CloudEnvironmentFile = '.env',
    [string]$PostgresBin = 'C:\Program Files\PostgreSQL\18\bin',
    [string]$OutputDirectory = 'output/workbuddy-project-rc-e2e',
    [ValidateSet('not_run', 'skipped_by_user')]
    [string]$CloudProjectPackageGoTestDisposition = 'not_run',
    [ValidateRange(30, 300)]
    [int]$StartupTimeoutSeconds = 120
)

$ErrorActionPreference = 'Stop'
$desktopRepository = [IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
$desktopPrefix = $desktopRepository + [IO.Path]::DirectorySeparatorChar
$cloudRepository = (Resolve-Path -LiteralPath $CloudRepository).Path
$adminRepository = (Resolve-Path -LiteralPath $AdminRepository).Path
$executablePath = (Resolve-Path -LiteralPath $Executable).Path
$rcLockPath = (Resolve-Path -LiteralPath $RCLock).Path
$postgresBinPath = (Resolve-Path -LiteralPath $PostgresBin).Path
$outputRoot = [IO.Path]::GetFullPath((Join-Path $desktopRepository $OutputDirectory))
if (-not $outputRoot.StartsWith($desktopPrefix, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'output directory must remain inside the Desktop repository'
}
foreach ($tool in @('psql.exe')) {
    if (-not (Test-Path -LiteralPath (Join-Path $postgresBinPath $tool) -PathType Leaf)) {
        throw "PostgreSQL tool is missing: $tool"
    }
}
$environmentPath = if ([IO.Path]::IsPathRooted($CloudEnvironmentFile)) {
    (Resolve-Path -LiteralPath $CloudEnvironmentFile).Path
} else {
    (Resolve-Path -LiteralPath (Join-Path $cloudRepository $CloudEnvironmentFile)).Path
}

$tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$tempPrefix = $tempRoot.TrimEnd([IO.Path]::DirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar
$runRoot = Join-Path $tempRoot ('workbuddy-project-rc-' + [guid]::NewGuid().ToString('N'))
if (-not $runRoot.StartsWith($tempPrefix, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'RC run root escaped the system temp directory'
}
$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$outputRun = Join-Path $outputRoot $stamp
$probeScript = Join-Path $desktopRepository 'desktop/frontend/e2e/workbuddy-project-webview2-probe.mjs'
$semanticScript = Join-Path $desktopRepository 'scripts/workbuddy-project-semantic-fault-e2e.mjs'
$uiaScript = Join-Path $desktopRepository 'scripts/testing-windows-uia-tree.ps1'
$cloudManifest = Join-Path $runRoot 'cloud-manifest.json'
$cloudStop = Join-Path $runRoot 'cloud.stop'
$cloudBarrier = Join-Path $runRoot 'cloud-commit-before-response.json'
$semanticBarrierDirectory = Join-Path $runRoot 'semantic-faults'
$semanticControlDirectory = Join-Path $runRoot 'database-control'
$cloudStdout = Join-Path $outputRun 'cloud.stdout.log'
$cloudStderr = Join-Path $outputRun 'cloud.stderr.log'
$driverStageLog = Join-Path $outputRun 'driver-stage.log'
$workspacePath = Join-Path $runRoot 'workspace'
$cloudProcess = $null
$cloudPID = $null
$databaseCreated = $false
$ownedDatabase = 'reasonix_workbuddy_rc_' + (Get-Date -Format 'yyyyMMddHHmmss') + '_' + [guid]::NewGuid().ToString('N').Substring(0, 8)
$ownedDesktopRuns = [Collections.Generic.List[object]]::new()
$ownedProbeProcesses = [Collections.Generic.List[object]]::new()
$result = $null
$runFailure = $null

function Write-DriverStage([string]$Stage) {
    Add-Content -LiteralPath $driverStageLog -Value ((Get-Date).ToUniversalTime().ToString('o') + ' ' + $Stage) -Encoding UTF8
}

function Get-FreeLoopbackPort {
    $listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
    $listener.Start()
    try { return ([Net.IPEndPoint]$listener.LocalEndpoint).Port } finally { $listener.Stop() }
}

function Test-LoopbackPortOpen([int]$Port) {
    $client = [Net.Sockets.TcpClient]::new()
    try {
        $connect = $client.ConnectAsync([Net.IPAddress]::Loopback, $Port)
        if (-not $connect.Wait(500)) { return $false }
        return $client.Connected
    } catch {
        return $false
    } finally {
        $client.Dispose()
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

function Stop-OwnedProcessTree([object]$Run) {
    if ($null -eq $Run -or $null -eq $Run.Process) { return }
    $pidValue = [int]$Run.OwnedPID
    $root = Get-CimInstance Win32_Process -Filter "ProcessId=$pidValue" -ErrorAction SilentlyContinue
    if ($null -eq $root) { return }
    if ($root.ExecutablePath -ne $executablePath) {
        throw "refusing to stop PID $pidValue because it does not own the RC executable"
    }
    $descendants = @(Get-DescendantProcesses -RootPID $pidValue)
    [array]::Reverse($descendants)
    foreach ($child in $descendants) {
        Stop-Process -Id $child.ProcessId -Force -ErrorAction SilentlyContinue
    }
    Stop-Process -Id $pidValue -Force -ErrorAction SilentlyContinue
    foreach ($ownedID in @($descendants.ProcessId) + @($pidValue)) {
        Wait-Process -Id $ownedID -Timeout 5 -ErrorAction SilentlyContinue
    }
}

function Set-ChildEnvironment([hashtable]$Values, [scriptblock]$Start) {
    $original = @{}
    foreach ($name in $Values.Keys) {
        $original[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
        [Environment]::SetEnvironmentVariable($name, $Values[$name], 'Process')
    }
    try { return & $Start } finally {
        foreach ($name in $Values.Keys) {
            [Environment]::SetEnvironmentVariable($name, $original[$name], 'Process')
        }
    }
}

function Get-EnvironmentValue([string]$Name) {
    $pattern = '^\s*' + [regex]::Escape($Name) + '\s*[:=]\s*(.*)\s*$'
    $line = Get-Content -LiteralPath $environmentPath -Encoding utf8 |
        Where-Object { $_ -match $pattern } |
        Select-Object -Last 1
    if ($null -eq $line) { throw "required environment key is missing: $Name" }
    $value = [regex]::Match($line, $pattern).Groups[1].Value.Trim()
    if (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'"))) {
        $value = $value.Substring(1, $value.Length - 2)
    }
    if ([string]::IsNullOrWhiteSpace($value)) { throw "required environment key is empty: $Name" }
    return $value
}

function Invoke-OwnedPsql([string]$Database, [string]$SQL) {
    $original = @{
        PGHOST = $env:PGHOST; PGPORT = $env:PGPORT; PGUSER = $env:PGUSER
        PGPASSWORD = $env:PGPASSWORD; PGDATABASE = $env:PGDATABASE; PGSSLMODE = $env:PGSSLMODE
    }
    try {
        $env:PGHOST = $script:databaseUri.Host
        $env:PGPORT = [string]$script:databasePort
        $env:PGUSER = $script:databaseUser
        $env:PGPASSWORD = $script:databasePassword
        $env:PGDATABASE = $Database
        $env:PGSSLMODE = $script:databaseSSLMode
        $output = & (Join-Path $postgresBinPath 'psql.exe') -X -v ON_ERROR_STOP=1 -Atq -c $SQL 2>&1
        if ($LASTEXITCODE -ne 0) { throw "owned PostgreSQL command failed with exit code $LASTEXITCODE" }
        return ($output -join "`n").Trim()
    } finally {
        foreach ($name in $original.Keys) { Set-Item -Path "Env:$name" -Value $original[$name] -ErrorAction SilentlyContinue }
    }
}

function Start-OwnedDesktop([string]$ProfileRoot, [string]$Label, [string]$FaultPoint = '', [string]$Barrier = '') {
    $cdpPort = Get-FreeLoopbackPort
    $stdoutPath = Join-Path $outputRun ("desktop-$Label.stdout.log")
    $stderrPath = Join-Path $outputRun ("desktop-$Label.stderr.log")
    $environment = @{
        'KWU_CLOUD_BASE_URL' = $script:cloudManifestValue.base_url
        'KWU_CLOUD_FAKE' = $null
        'KWU_DEV_PROFILE_ROOT' = $ProfileRoot
        'KWU_TEST_WEBVIEW2_CDP_PORT' = [string]$cdpPort
        'WORKBUDDY_RC_MANIFEST' = $cloudManifest
        'WORKBUDDY_RC_FAULT_POINT' = $FaultPoint
        'WORKBUDDY_RC_BARRIER_FILE' = $Barrier
        'WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS' = "--remote-debugging-address=127.0.0.1 --remote-debugging-port=$cdpPort"
        'WEBVIEW2_USER_DATA_FOLDER' = (Join-Path $ProfileRoot 'webview')
    }
    $bootstrapReportPath = $cloudManifest + '.desktop-bootstrap.json'
    if (Test-Path -LiteralPath $bootstrapReportPath -PathType Leaf) {
        [IO.File]::Delete($bootstrapReportPath)
    }
    $process = Set-ChildEnvironment $environment {
        Start-Process -FilePath $executablePath -WorkingDirectory (Join-Path $desktopRepository 'desktop') -PassThru -WindowStyle Hidden -RedirectStandardOutput $stdoutPath -RedirectStandardError $stderrPath
    }
    $run = [pscustomobject]@{ Process = $process; OwnedPID = [int]$process.Id; Endpoint = "http://127.0.0.1:$cdpPort"; Profile = $ProfileRoot; Label = $Label; Stdout = $stdoutPath; Stderr = $stderrPath }
    $ownedDesktopRuns.Add($run)
    $deadline = (Get-Date).AddSeconds($StartupTimeoutSeconds)
    $version = $null
    while ((Get-Date) -lt $deadline) {
        if ($process.HasExited) {
            throw "owned Desktop $Label exited before CDP was ready (exit=$($process.ExitCode))"
        }
        try {
            $version = Invoke-RestMethod -Uri "$($run.Endpoint)/json/version" -TimeoutSec 2
            break
        } catch {
            Start-Sleep -Milliseconds 250
        }
    }
    if ($null -eq $version) { throw "owned Desktop $Label did not expose CDP" }
    $root = Get-CimInstance Win32_Process -Filter "ProcessId=$($run.OwnedPID)"
    if ($null -eq $root -or $root.ExecutablePath -ne $executablePath) { throw "owned Desktop $Label PID mismatch" }
    $webviews = @(Get-DescendantProcesses -RootPID $run.OwnedPID | Where-Object { $_.Name -eq 'msedgewebview2.exe' })
    if ($webviews.Count -eq 0 -or -not ($webviews.CommandLine -match "--remote-debugging-port=$cdpPort")) {
        throw "owned Desktop $Label has no isolated WebView2"
    }
    $bootstrapDeadline = (Get-Date).AddSeconds(5)
    while ((Get-Date) -lt $bootstrapDeadline -and -not (Test-Path -LiteralPath $bootstrapReportPath -PathType Leaf)) {
        Start-Sleep -Milliseconds 100
    }
    if (-not (Test-Path -LiteralPath $bootstrapReportPath -PathType Leaf)) { throw "owned Desktop $Label did not publish its RC bootstrap report" }
    $bootstrapReport = Get-Content -LiteralPath $bootstrapReportPath -Raw | ConvertFrom-Json
    Copy-Item -LiteralPath $bootstrapReportPath -Destination (Join-Path $outputRun ("desktop-$Label.bootstrap.json")) -Force
    if (-not $bootstrapReport.ok) { throw "owned Desktop $Label RC bootstrap failed at $($bootstrapReport.stage)" }
    $run | Add-Member -NotePropertyName WebViewPIDs -NotePropertyValue @($webviews.ProcessId)
    return $run
}

function Invoke-Probe([object]$Run, [string]$Mode, [string]$Name, [string]$Label) {
    $arguments = @($probeScript, '--endpoint', $Run.Endpoint, '--output', $outputRun, '--mode', $Mode, '--project-name', $Name)
    if (-not [string]::IsNullOrWhiteSpace($workspacePath)) { $arguments += @('--workspace', $workspacePath) }
    $original = $env:WORKBUDDY_RC_MANIFEST
    try {
        $env:WORKBUDDY_RC_MANIFEST = $cloudManifest
        $json = & node @arguments
        $probeExitCode = $LASTEXITCODE
        $probeResult = $json | ConvertFrom-Json
        $probeResult | ConvertTo-Json -Depth 10 |
            Set-Content -LiteralPath (Join-Path $outputRun ("probe-$Label.result.json")) -Encoding utf8
        if ($probeExitCode -ne 0) {
            throw "Project CDP probe $Label failed with exit code $probeExitCode (consoleErrors=$($probeResult.consoleErrorCount), pageErrors=$($probeResult.pageErrorCount), axe=$($probeResult.seriousOrCriticalAxeViolations), warnings=$($probeResult.consoleWarningCount))"
        }
        return $probeResult
    } finally {
        $env:WORKBUDDY_RC_MANIFEST = $original
    }
}

function Start-CrashProbe([object]$Run, [string]$Name, [string]$Label) {
    $stdoutPath = Join-Path $outputRun ("probe-$Label.stdout.log")
    $stderrPath = Join-Path $outputRun ("probe-$Label.stderr.log")
    if ($Name.Contains('"')) { throw 'synthetic Project name cannot contain a quote' }
    $quotedName = '"' + $Name + '"'
    $arguments = @($probeScript, '--endpoint', $Run.Endpoint, '--output', $outputRun, '--mode', 'crash', '--project-name', $quotedName, '--workspace', $workspacePath)
    $process = Set-ChildEnvironment @{ 'WORKBUDDY_RC_MANIFEST' = $cloudManifest } {
        Start-Process -FilePath (Get-Command node).Source -ArgumentList $arguments -PassThru -WindowStyle Hidden -RedirectStandardOutput $stdoutPath -RedirectStandardError $stderrPath
    }
    $process | Add-Member -NotePropertyName OwnedPID -NotePropertyValue ([int]$process.Id)
    $ownedProbeProcesses.Add($process)
    return $process
}

function Wait-Barrier([string]$Path, [object]$Probe, [string]$Label) {
    $deadline = (Get-Date).AddSeconds(30)
    while ((Get-Date) -lt $deadline) {
        if (Test-Path -LiteralPath $Path -PathType Leaf) { return Get-Content -LiteralPath $Path -Raw | ConvertFrom-Json }
        if ($Probe.HasExited) { throw "crash probe $Label exited before its durable barrier" }
        Start-Sleep -Milliseconds 100
    }
    throw "crash probe $Label did not publish its durable barrier"
}

function Get-TextSHA256([string]$Value) {
    $bytes = [Text.Encoding]::UTF8.GetBytes($Value)
    $algorithm = [Security.Cryptography.SHA256]::Create()
    try {
        $digest = $algorithm.ComputeHash($bytes)
        return ([BitConverter]::ToString($digest)).Replace('-', '').ToLowerInvariant()
    } finally {
        $algorithm.Dispose()
    }
}

function Get-JournalState([string]$ProfileRoot) {
    $files = @(Get-ChildItem -LiteralPath $ProfileRoot -Recurse -Filter 'project_commands.json' -File -ErrorAction SilentlyContinue)
    $commands = @()
    $scopeKeys = @()
    foreach ($file in $files) {
        $document = Get-Content -LiteralPath $file.FullName -Raw | ConvertFrom-Json
        $commands += @($document.commands)
        $scopeKeys += @($document.scope_key)
    }
    $keyHash = if ($commands.Count -eq 1) { Get-TextSHA256 ([string]$commands[0].idempotency_key) } else { '' }
    return [pscustomobject]@{ FileCount = $files.Count; CommandCount = $commands.Count; ScopeKeys = @($scopeKeys | Sort-Object -Unique); KeyHash = $keyHash }
}

function Wait-JournalEmpty([string]$ProfileRoot) {
    $deadline = (Get-Date).AddSeconds(20)
    do {
        $state = Get-JournalState $ProfileRoot
        if ($state.CommandCount -eq 0) { return $state }
        Start-Sleep -Milliseconds 200
    } while ((Get-Date) -lt $deadline)
    throw 'restart replay did not empty the owned command journal'
}

function Get-DatabaseProjectCount([string]$Name, [int]$Port) {
    if ($Name -notmatch '^[A-Za-z0-9 _-]+$') { throw 'unsafe synthetic Project name' }
    $sql = "SELECT count(*) FROM public.projects WHERE name = '$Name';"
    $count = Invoke-OwnedPsql -Database $ownedDatabase -SQL $sql
    if ($count -notmatch '^\d+$') { throw 'cannot count the synthetic Project fact' }
    return [int]$count
}

function Get-DatabaseControlRequest([string]$Action, [object]$Process) {
    $requestPath = Join-Path $semanticControlDirectory ($Action + '.request.json')
    $deadline = (Get-Date).AddSeconds(30)
    while ((Get-Date) -lt $deadline) {
        if (Test-Path -LiteralPath $requestPath -PathType Leaf) {
            $request = Get-Content -LiteralPath $requestPath -Raw | ConvertFrom-Json
            if ($request.schema -ne 'reasonix.workbuddy-project.database-control/v1' -or $request.action -ne $Action) {
                throw "invalid database control request: $Action"
            }
            return $request
        }
        if ($Process.HasExited) {
            throw "semantic fault process exited before database control request: $Action"
        }
        Start-Sleep -Milliseconds 50
    }
    throw "timed out waiting for database control request: $Action"
}

function Complete-DatabaseControl([string]$Action) {
    $acknowledgementPath = Join-Path $semanticControlDirectory ($Action + '.ack.json')
    $payload = [ordered]@{ action = $Action; success = $true } | ConvertTo-Json -Compress
    [IO.File]::WriteAllText($acknowledgementPath, $payload + [Environment]::NewLine, [Text.UTF8Encoding]::new($false))
}

function Assert-RCIdentifier([string]$Value, [string]$Name) {
    if ($Value -notmatch '^[A-Za-z0-9_-]+$') { throw "unsafe RC identifier: $Name" }
}

function Invoke-CrashCase([string]$Point, [bool]$CloudFault, [int]$Port) {
    $safe = $Point.Replace('_', '-')
    $profile = Join-Path $runRoot ("profile-$safe")
    New-Item -ItemType Directory -Force -Path $profile | Out-Null
    $barrier = if ($CloudFault) { $cloudBarrier } else { Join-Path $runRoot ("$safe.json") }
    $name = "WorkBuddy RC $safe"
    $fault = if ($CloudFault) { '' } else { $Point }
    $desktop = Start-OwnedDesktop $profile ("$safe-crash") $fault $barrier
    $probe = Start-CrashProbe $desktop $name $safe
    $barrierValue = Wait-Barrier $barrier $probe $safe
    $journalBefore = Get-JournalState $profile
    if ($journalBefore.CommandCount -ne 1 -or $journalBefore.ScopeKeys.Count -ne 1 -or $journalBefore.KeyHash -ne $barrierValue.idempotency_key_sha256) {
        throw "crash case $Point did not preserve the exact durable command"
    }
    $killedPID = $desktop.OwnedPID
    Stop-OwnedProcessTree $desktop
    $probeDeadline = (Get-Date).AddSeconds(10)
    while ((Get-Date) -lt $probeDeadline -and $null -ne (Get-Process -Id $probe.OwnedPID -ErrorAction SilentlyContinue)) {
        Start-Sleep -Milliseconds 100
    }
    if ($null -ne (Get-Process -Id $probe.OwnedPID -ErrorAction SilentlyContinue)) {
        Stop-Process -Id $probe.OwnedPID -Force -ErrorAction SilentlyContinue
    }
    $restarted = Start-OwnedDesktop $profile ("$safe-restart")
    $inspection = Invoke-Probe $restarted 'inspect' $name ("$safe-inspect")
    $journalAfter = Wait-JournalEmpty $profile
    $dbCount = Get-DatabaseProjectCount $name $Port
    $sameScope = $inspection.flow.scopeKey -eq $journalBefore.ScopeKeys[0]
    if ($inspection.flow.matchCount -ne 1 -or $dbCount -ne 1 -or $journalAfter.CommandCount -ne 0 -or -not $sameScope) {
        throw "crash case $Point did not reconcile one exact fact"
    }
    Stop-OwnedProcessTree $restarted
    return [ordered]@{
        point = $Point
        killedDesktopPID = $killedPID
        barrierPIDMatchedOwnedProcess = if ($CloudFault) { [int]$barrierValue.pid -eq [int]$script:cloudManifestValue.listener_pid } else { [int]$barrierValue.pid -eq [int]$killedPID }
        journalBeforeCount = $journalBefore.CommandCount
        sameIdempotencyKey = $true
        sameProfile = $true
        sameScope = $sameScope
        restFactCount = $inspection.flow.matchCount
        databaseFactCount = $dbCount
        journalAfterCount = $journalAfter.CommandCount
    }
}

function Get-PrivacyScan([string[]]$Profiles) {
    $manifest = Get-Content -LiteralPath $cloudManifest -Raw | ConvertFrom-Json
    $needles = @(
        [string]$manifest.access_token_a,
        [string]$manifest.access_token_b,
        [string]$manifest.access_token_admin,
        [string]$manifest.login_password,
        'Synthetic RC description',
        'Synthetic RC crash description',
        'Synthetic instruction summary',
        'Keep the synthetic RC task scoped.',
        $workspacePath
    ) | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }
    $logFiles = @(Get-ChildItem -LiteralPath $outputRun -Filter '*.log' -File -ErrorAction SilentlyContinue)
    $logMatches = 0
    foreach ($file in $logFiles) {
        $text = Get-Content -LiteralPath $file.FullName -Raw -ErrorAction SilentlyContinue
        if ($null -eq $text) { $text = '' }
        foreach ($needle in $needles) { if ($text.Contains($needle)) { $logMatches += 1 } }
        foreach ($pattern in @('authorization', 'access_token', 'idempotency_key', 'canonical_payload', 'local_payload_hash', 'server_request_digest')) {
            if ($text.IndexOf($pattern, [StringComparison]::OrdinalIgnoreCase) -ge 0) { $logMatches += 1 }
        }
    }
    $payloadFiles = @()
    foreach ($profile in $Profiles) {
        $payloadFiles += @(Get-ChildItem -LiteralPath $profile -Recurse -File -ErrorAction SilentlyContinue | Where-Object { $_.Name -match '(?i)error|telemetry|hook' })
    }
    $payloadMatches = 0
    foreach ($file in $payloadFiles) {
        $text = Get-Content -LiteralPath $file.FullName -Raw -ErrorAction SilentlyContinue
        if ($null -eq $text) { $text = '' }
        foreach ($needle in $needles) { if ($text.Contains($needle)) { $payloadMatches += 1 } }
    }
    return [ordered]@{ logFileCount = $logFiles.Count; logSensitiveMatchCount = $logMatches; errorTelemetryHookFileCount = $payloadFiles.Count; errorTelemetryHookSensitiveMatchCount = $payloadMatches }
}

New-Item -ItemType Directory -Force -Path $runRoot, $outputRun, $workspacePath, $semanticBarrierDirectory, $semanticControlDirectory | Out-Null
try {
    $databaseURL = Get-EnvironmentValue 'DATABASE_URL'
    $databaseUri = [Uri]$databaseURL
    if ($databaseUri.Scheme -notin @('postgres', 'postgresql')) { throw 'DATABASE_URL is not PostgreSQL' }
    if ($databaseUri.Host -in @('127.0.0.1', 'localhost', '::1')) { throw 'Project RC requires the configured remote PostgreSQL host' }
    $userParts = $databaseUri.UserInfo.Split(':', 2)
    if ($userParts.Count -ne 2) { throw 'DATABASE_URL must contain PostgreSQL credentials' }
    $databaseUser = [Uri]::UnescapeDataString($userParts[0])
    $databasePassword = [Uri]::UnescapeDataString($userParts[1])
    $databasePort = if ($databaseUri.Port -gt 0) { $databaseUri.Port } else { 5432 }
    $pgPort = $databasePort
    $sslMatch = [regex]::Match($databaseUri.Query, '(?:^|[?&])sslmode=([^&]+)')
    $databaseSSLMode = if ($sslMatch.Success) { [Uri]::UnescapeDataString($sslMatch.Groups[1].Value) } else { 'prefer' }
    $databaseBuilder = [UriBuilder]::new($databaseUri)
    $databaseBuilder.Path = '/' + $ownedDatabase
    $databaseURL = $databaseBuilder.Uri.AbsoluteUri

    Invoke-OwnedPsql -Database 'postgres' -SQL "CREATE DATABASE $ownedDatabase" | Out-Null
    $databaseCreated = $true
    Write-DriverStage 'database.created'
    $cloudEnvironment = @{
        'WORKBUDDY_GOAL2_DATABASE_URL' = $databaseURL
        'WORKBUDDY_GOAL2_MANIFEST' = $cloudManifest
        'WORKBUDDY_GOAL2_STOP_FILE' = $cloudStop
        'WORKBUDDY_GOAL2_CLOUD_CANDIDATE_MANIFEST' = ''
        'WORKBUDDY_GOAL2_DESKTOP_CANDIDATE_MANIFEST' = ''
        'WORKBUDDY_RC_LOCK' = $rcLockPath
        'WORKBUDDY_RC_CLOUD_FAULT_POINT' = 'cloud_commit_before_response,semantic_fault_matrix'
        'WORKBUDDY_RC_CLOUD_BARRIER_FILE' = $cloudBarrier
        'WORKBUDDY_RC_SEMANTIC_BARRIER_DIRECTORY' = $semanticBarrierDirectory
    }
    $cloudProcess = Set-ChildEnvironment $cloudEnvironment {
        Start-Process -FilePath (Get-Command go).Source -ArgumentList @('run', './cmd/project-goal2-e2e-cloud') -WorkingDirectory $cloudRepository -PassThru -WindowStyle Hidden -RedirectStandardOutput $cloudStdout -RedirectStandardError $cloudStderr
    }
    $cloudPID = [int]$cloudProcess.Id
    Write-DriverStage 'cloud.spawned'
    $deadline = (Get-Date).AddSeconds($StartupTimeoutSeconds)
    while ((Get-Date) -lt $deadline -and -not (Test-Path -LiteralPath $cloudManifest -PathType Leaf)) {
        if ($cloudProcess.HasExited) { throw "owned Cloud exited before publishing its manifest (exit=$($cloudProcess.ExitCode))" }
        Start-Sleep -Milliseconds 250
    }
    if (-not (Test-Path -LiteralPath $cloudManifest -PathType Leaf)) { throw 'owned Cloud manifest was not published' }
    Write-DriverStage 'cloud.manifest'
    $script:cloudManifestValue = Get-Content -LiteralPath $cloudManifest -Raw | ConvertFrom-Json
    $health = Invoke-RestMethod -Uri ($script:cloudManifestValue.base_url.TrimEnd('/') + '/healthz') -TimeoutSec 5
    if ($null -eq $health) { throw 'owned Cloud health endpoint failed' }
    $me = Invoke-RestMethod -Uri ($script:cloudManifestValue.base_url.TrimEnd('/') + '/api/v1/me') -Headers @{
        Authorization = 'Bearer ' + $script:cloudManifestValue.access_token_a
    } -TimeoutSec 5
    if ($null -eq $me.data -or $me.data.user.user_id -ne $script:cloudManifestValue.user_a) {
        throw 'owned Cloud authenticated session preflight failed'
    }
    Write-DriverStage 'cloud.auth_preflight'

    $crashes = [Collections.Generic.List[object]]::new()
    $crashes.Add((Invoke-CrashCase 'cloud_commit_before_response' $true $pgPort))
    foreach ($point in @('journal_durable_before_http', 'response_before_authoritative_readback', 'readback_before_journal_eviction')) {
        $crashes.Add((Invoke-CrashCase $point $false $pgPort))
    }

    $semanticStdout = Join-Path $outputRun 'semantic-faults.stdout.log'
    $semanticStderr = Join-Path $outputRun 'semantic-faults.stderr.log'
    $semanticProcess = Start-Process -FilePath (Get-Command node).Source -ArgumentList @(
        $semanticScript,
        '--manifest', $cloudManifest,
        '--barrier-directory', $semanticBarrierDirectory,
        '--control-directory', $semanticControlDirectory,
        '--output', (Join-Path $outputRun 'semantic-faults.json')
    ) -PassThru -WindowStyle Hidden -RedirectStandardOutput $semanticStdout -RedirectStandardError $semanticStderr
    $ownedProbeProcesses.Add([pscustomobject]@{ Process = $semanticProcess; OwnedPID = [int]$semanticProcess.Id })


    Wait-Process -Id $semanticProcess.Id -Timeout 30 -ErrorAction SilentlyContinue
    $semanticProcess.Refresh()
    if (-not $semanticProcess.HasExited) { throw 'semantic fault process did not finish after database controls' }
    $semanticProcess.WaitForExit()
    $semanticExitCode = [int]$semanticProcess.ExitCode
    if ($semanticExitCode -ne 0) {
        $semanticFailure = Get-Content -LiteralPath $semanticStderr -Raw -ErrorAction SilentlyContinue
        throw "semantic fault matrix failed with exit code $semanticExitCode`: $semanticFailure"
    }
    $semantic = (Get-Content -LiteralPath $semanticStdout -Raw) | ConvertFrom-Json
    if ($semantic.verdict -ne 'PASS') { throw 'semantic fault matrix did not pass' }
    Write-DriverStage 'semantic_faults.passed'

    $normalProfile = Join-Path $runRoot 'profile-normal'
    New-Item -ItemType Directory -Force -Path $normalProfile | Out-Null
    $normalName = 'WorkBuddy RC normal flow'
    $seedDesktop = Start-OwnedDesktop $normalProfile 'normal-seed'
    $seed = Invoke-Probe $seedDesktop 'seed' $normalName 'normal-seed'
    $uiaRelative = ($outputRun.Substring($desktopPrefix.Length).Replace([IO.Path]::DirectorySeparatorChar, '/') + '/uia-normal-seed.json')
    $uia = & $uiaScript -ProcessID $seedDesktop.OwnedPID -ExpectedExecutable $executablePath -OutputPath $uiaRelative | ConvertFrom-Json
    if ($LASTEXITCODE -ne 0 -or $uia.nodeCount -lt 1 -or $uia.processID -ne $seedDesktop.OwnedPID) { throw 'owned Desktop UIA evidence failed' }
    $seedPID = $seedDesktop.OwnedPID
    $seedWebViewPIDs = @($seedDesktop.WebViewPIDs)
    Stop-OwnedProcessTree $seedDesktop
    $resumeDesktop = Start-OwnedDesktop $normalProfile 'normal-resume'
    $resume = Invoke-Probe $resumeDesktop 'resume' $normalName 'normal-resume'
    $resumePID = $resumeDesktop.OwnedPID
    Stop-OwnedProcessTree $resumeDesktop
    $normalJournal = Wait-JournalEmpty $normalProfile
    $normalDBCount = Get-DatabaseProjectCount $normalName $pgPort
    if ($normalDBCount -ne 1 -or $normalJournal.CommandCount -ne 0) { throw 'normal Wails flow did not settle exactly once' }

    $privacy = Get-PrivacyScan @($ownedDesktopRuns.Profile | Sort-Object -Unique)
    if ($privacy.logSensitiveMatchCount -ne 0 -or $privacy.errorTelemetryHookSensitiveMatchCount -ne 0) {
        throw 'owned runtime logs or diagnostic payloads exposed sensitive Project data'
    }

    $lockValue = Get-Content -LiteralPath $rcLockPath -Raw | ConvertFrom-Json
    $result = [ordered]@{
        schema = 'reasonix.workbuddy-project-core-v1.unified-rc-e2e/v1'
        source = [ordered]@{
            rcLockSHA256 = (Get-FileHash -LiteralPath $rcLockPath -Algorithm SHA256).Hash.ToLowerInvariant()
            releaseCandidateSHA256 = $lockValue.release_candidate_sha256
            fixtureVersion = $script:cloudManifestValue.contract_version
            fixtureSHA256 = $script:cloudManifestValue.contract_sha256
            desktopHead = $script:cloudManifestValue.desktop_head
            cloudHead = $script:cloudManifestValue.cloud_head
            adminHead = $script:cloudManifestValue.admin_head
        }
        runtime = [ordered]@{
            remotePostgreSQL = [ordered]@{ host = $databaseUri.Host; port = $databasePort; ownedDatabase = $ownedDatabase }
            cloudListenerPID = [int]$script:cloudManifestValue.listener_pid
            cloudLoopback = $true
            executableSHA256 = (Get-FileHash -LiteralPath $executablePath -Algorithm SHA256).Hash.ToLowerInvariant()
            seedDesktopPID = $seedPID
            seedWebViewPIDs = $seedWebViewPIDs
            resumeDesktopPID = $resumePID
            nativeWindow = [ordered]@{ nodeCount = $uia.nodeCount; maximumDepth = $uia.maximumDepth; truncated = $uia.truncated }
            testAccountCount = 2
        }
        normalFlow = [ordered]@{
            authenticated = [bool]$seed.authenticated
            createReadbackSearch = [bool]($seed.flow.readbackMatches -and $seed.flow.searchMatches)
            localWorkspaceBound = [bool]$seed.flow.bound
            restartedWithSameProfile = $true
            instructionVersion = [int]$resume.flow.instructionVersion
            sessionPinnedInstruction = [bool]$resume.flow.sessionProjectMatches
            workItemMovedStatus = [string]$resume.flow.movedStatus
            archivedStatus = [string]$resume.flow.archivedStatus
            accountSwitchIsolated = [bool]($resume.flow.accountSwitchMatches -and $resume.flow.accountBProjectCount -eq 0 -and $resume.flow.foreignReadRejected)
            databaseFactCount = $normalDBCount
            journalAfterCount = $normalJournal.CommandCount
            seriousOrCriticalAxeViolations = [int]($seed.seriousOrCriticalAxeViolations + $resume.seriousOrCriticalAxeViolations)
            consoleOrPageErrors = [int]($seed.consoleErrorCount + $seed.pageErrorCount + $resume.consoleErrorCount + $resume.pageErrorCount)
            consoleWarnings = [int]($seed.consoleWarningCount + $resume.consoleWarningCount)
        }
        crashReplay = @($crashes)
        semanticFaults = $semantic
        validation = [ordered]@{
            cloudProjectPackageGoTest = [ordered]@{
                command = 'go test ./internal/projects -count=1'
                disposition = $CloudProjectPackageGoTestDisposition
                passed = $false
            }
        }
        privacy = $privacy
        cleanup = [ordered]@{ cloudStopped = $false; databaseDropped = $false; temporaryRootRemoved = $false; residualOwnedProcesses = -1 }
        verdict = 'PASS'
    }
} catch {
    $runFailure = $_.Exception
    [ordered]@{
        message = $_.Exception.Message
        scriptStackTrace = $_.ScriptStackTrace
        position = $_.InvocationInfo.PositionMessage
    } | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $outputRun 'failure-diagnostics.json') -Encoding utf8
} finally {
    foreach ($desktopRun in $ownedDesktopRuns) {
        try { Stop-OwnedProcessTree $desktopRun } catch { if ($null -eq $runFailure) { $runFailure = $_.Exception } }
    }
    foreach ($probeProcess in $ownedProbeProcesses) {
        if ($null -ne (Get-Process -Id $probeProcess.OwnedPID -ErrorAction SilentlyContinue)) {
            Stop-Process -Id $probeProcess.OwnedPID -Force -ErrorAction SilentlyContinue
        }
    }
    if ($null -ne $cloudProcess) {
        New-Item -ItemType File -Force -Path $cloudStop | Out-Null
        $cloudDeadline = (Get-Date).AddSeconds(20)
        while ((Get-Date) -lt $cloudDeadline -and $null -ne (Get-Process -Id $cloudPID -ErrorAction SilentlyContinue)) {
            Start-Sleep -Milliseconds 100
        }
        if ($null -ne (Get-Process -Id $cloudPID -ErrorAction SilentlyContinue)) {
            foreach ($child in @(Get-DescendantProcesses -RootPID $cloudPID)) { Stop-Process -Id $child.ProcessId -Force -ErrorAction SilentlyContinue }
            Stop-Process -Id $cloudPID -Force -ErrorAction SilentlyContinue
        }
    }
    if ($databaseCreated) {
        try {
            Invoke-OwnedPsql -Database 'postgres' -SQL "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='$ownedDatabase' AND pid<>pg_backend_pid()" | Out-Null
            Invoke-OwnedPsql -Database 'postgres' -SQL "DROP DATABASE $ownedDatabase" | Out-Null
            if ($null -ne $result) { $result.cleanup.databaseDropped = $true }
        } catch {
            if ($null -eq $runFailure) { $runFailure = $_.Exception }
        }
    }
    $residual = @(
        Get-CimInstance Win32_Process -ErrorAction SilentlyContinue |
            Where-Object { $_.CommandLine -and $_.CommandLine.IndexOf($runRoot, [StringComparison]::OrdinalIgnoreCase) -ge 0 }
    )
    foreach ($process in $residual) { Stop-Process -Id $process.ProcessId -Force -ErrorAction SilentlyContinue }
    $residualAfterCleanup = @(
        Get-CimInstance Win32_Process -ErrorAction SilentlyContinue |
            Where-Object { $_.CommandLine -and $_.CommandLine.IndexOf($runRoot, [StringComparison]::OrdinalIgnoreCase) -ge 0 }
    )
    if ($null -ne $result) {
        $result.cleanup.cloudStopped = $null -eq (Get-Process -Id $cloudPID -ErrorAction SilentlyContinue)
        $result.cleanup.residualOwnedProcesses = $residualAfterCleanup.Count
        if (-not $result.cleanup.cloudStopped -or -not $result.cleanup.databaseDropped -or $result.cleanup.residualOwnedProcesses -ne 0) {
            if ($null -eq $runFailure) { $runFailure = [Exception]::new('owned runtime cleanup verification failed') }
        }
    }
    if (Test-Path -LiteralPath $runRoot) {
        $resolvedRunRoot = [IO.Path]::GetFullPath($runRoot)
        if (-not $resolvedRunRoot.StartsWith($tempPrefix, [StringComparison]::OrdinalIgnoreCase)) { throw 'refusing RC cleanup outside system temp' }
        $deleteRoot = if ($env:OS -eq 'Windows_NT') { '\\?\' + $resolvedRunRoot } else { $resolvedRunRoot }
        [IO.Directory]::Delete($deleteRoot, $true)
    }
    if ($null -ne $result) {
        $result.cleanup.temporaryRootRemoved = -not (Test-Path -LiteralPath $runRoot)
        if (-not $result.cleanup.temporaryRootRemoved -and $null -eq $runFailure) { $runFailure = [Exception]::new('owned temporary root cleanup verification failed') }
    }
}

$reportPath = Join-Path $outputRun 'unified-rc-e2e-report.json'
if ($null -ne $result) {
    $reportJSON = $result | ConvertTo-Json -Depth 10
    $reportJSON | Set-Content -LiteralPath $reportPath -Encoding utf8
    if ($null -eq $runFailure -and $result.verdict -eq 'PASS') {
        $reportJSON | Set-Content -LiteralPath (Join-Path $outputRoot 'latest-unified-rc-e2e-report.json') -Encoding utf8
    }
    $reportJSON
}
if ($null -ne $runFailure) { throw "WorkBuddy Project unified RC E2E failed: $($runFailure.Message)" }
exit 0

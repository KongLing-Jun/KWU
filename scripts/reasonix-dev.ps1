[CmdletBinding()]
param(
    [ValidateSet('up', 'status', 'verify', 'down')]
    [string]$Command = 'status',
    [string]$CloudRoot,
    [string]$AdminRoot,
    [string]$CloudBaseURL = 'http://127.0.0.1:9090',
    [Parameter(DontShow = $true)]
    [switch]$RunDesktopChild,
    [Parameter(DontShow = $true)]
    [switch]$LibraryOnly
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$script:DesktopRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$script:RuntimeRoot = Join-Path $env:LOCALAPPDATA 'Reasonix\dev'
$script:ManifestPath = Join-Path $script:RuntimeRoot 'runtime.json'
$script:CloudPort = 9090
$script:AdminPort = 5173
$script:DesktopVitePort = 5175
$script:DesktopWailsPort = 34238

function ConvertTo-CanonicalCloudURL {
    param([Parameter(Mandatory = $true)][string]$Value)

    $uri = [Uri]$Value
    if (-not $uri.IsAbsoluteUri -or $uri.Scheme -ne 'http') {
        throw 'CloudBaseURL must be an absolute http URL.'
    }
    if ($uri.UserInfo -or $uri.Query -or $uri.Fragment -or $uri.AbsolutePath -ne '/') {
        throw 'CloudBaseURL must not contain credentials, a path, query, or fragment.'
    }
    if ($uri.Host -ne '127.0.0.1' -or $uri.Port -ne $script:CloudPort) {
        throw "The canonical integrated Cloud must be http://127.0.0.1:$($script:CloudPort)."
    }
    return "http://127.0.0.1:$($script:CloudPort)/"
}

function ConvertTo-AdminProxyJSON {
    param([Parameter(Mandatory = $true)][string]$CanonicalCloudURL)
    $proxy = [Collections.ArrayList]::new()
    [void]$proxy.Add([object[]]@('/__proxy', $CanonicalCloudURL))
    return (ConvertTo-Json -InputObject $proxy -Compress)
}

function Get-GitMetadata {
    param([Parameter(Mandatory = $true)][string]$Root)

    $resolved = (Resolve-Path -LiteralPath $Root).Path
    $branch = (& git -C $resolved branch --show-current).Trim()
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($branch)) {
        throw "Not an attached Git worktree: $resolved"
    }
    $head = (& git -C $resolved rev-parse HEAD).Trim()
    if ($LASTEXITCODE -ne 0 -or $head -notmatch '^[0-9a-f]{40}$') {
        throw "Cannot resolve Git revision: $resolved"
    }
    return [ordered]@{ root = $resolved; branch = $branch; head = $head }
}

function Resolve-IntegrationRoots {
    param([string]$RequestedCloudRoot, [string]$RequestedAdminRoot)

    $desktop = Get-GitMetadata -Root $script:DesktopRoot
    $date = [regex]::Match($desktop.branch, '(?<date>\d{8})$').Groups['date'].Value
    if ([string]::IsNullOrWhiteSpace($RequestedCloudRoot)) {
        if (-not $date) { throw 'CloudRoot is required when the Desktop branch has no date suffix.' }
        $RequestedCloudRoot = "D:\project\reasonix-cloud-platform\Reasonix-Cloud-worktrees\integration-$date"
    }
    if ([string]::IsNullOrWhiteSpace($RequestedAdminRoot)) {
        if (-not $date) { throw 'AdminRoot is required when the Desktop branch has no date suffix.' }
        $RequestedAdminRoot = "D:\project\reasonix-cloud-platform\naive-ui-admin-worktrees\integration-$date"
    }
    return [ordered]@{
        desktop = $desktop
        cloud = Get-GitMetadata -Root $RequestedCloudRoot
        admin = Get-GitMetadata -Root $RequestedAdminRoot
    }
}

function Assert-RuntimeManifest {
    param([Parameter(Mandatory = $true)]$Manifest)

    $serialized = ConvertTo-Json -InputObject $Manifest -Depth 8 -Compress
    if ($serialized -match '(?i)"[^"\\]*(password|secret|token|credential|database_url|redis_url|authorization|cookie)[^"\\]*"\s*:') {
        throw 'Runtime manifest contains a forbidden secret-like field.'
    }
    $allowedTop = @('schema_version', 'cloud_base_url', 'cloud_environment_id', 'cloud_environment_name', 'created_at', 'repositories', 'services')
    $topProperties = if ($Manifest -is [Collections.IDictionary]) { @($Manifest.Keys) } else { @($Manifest.PSObject.Properties.Name) }
    foreach ($property in $topProperties) {
        if ($allowedTop -notcontains $property) { throw "Runtime manifest contains unknown field: $property" }
        if ($property -match '(?i)password|secret|token|credential|database_url|redis_url|authorization|cookie') {
            throw 'Runtime manifest contains a forbidden secret-like field.'
        }
    }
    if ([int]$Manifest.schema_version -ne 1) { throw 'Unsupported runtime manifest schema.' }
    if ((ConvertTo-CanonicalCloudURL ([string]$Manifest.cloud_base_url)) -ne [string]$Manifest.cloud_base_url) {
        throw 'Runtime manifest Cloud URL is not canonical.'
    }
    if ([string]$Manifest.cloud_environment_id -notmatch '^[0-9a-fA-F-]{36}$') {
        throw 'Runtime manifest Cloud environment ID is invalid.'
    }
    $serviceValues = if ($Manifest.services -is [Collections.IDictionary]) { @($Manifest.services.Values) } else { @($Manifest.services.PSObject.Properties.Value) }
    foreach ($service in $serviceValues) {
        $allowedService = @('pid', 'creation_time', 'port', 'entry_path')
        $serviceProperties = if ($service -is [Collections.IDictionary]) { @($service.Keys) } else { @($service.PSObject.Properties.Name) }
        foreach ($property in $serviceProperties) {
            if ($allowedService -notcontains $property) { throw "Runtime service contains unknown field: $property" }
        }
        if ([int]$service.pid -le 0 -or [int]$service.port -le 0) { throw 'Runtime service PID or port is invalid.' }
        $entry = [IO.Path]::GetFullPath([string]$service.entry_path)
        if (-not (Test-Path -LiteralPath $entry -PathType Leaf)) { throw "Runtime service entry does not exist: $entry" }
    }
}

function Write-AtomicRuntimeManifest {
    param([Parameter(Mandatory = $true)]$Manifest, [Parameter(Mandatory = $true)][string]$Path)

    Assert-RuntimeManifest -Manifest $Manifest
    $directory = Split-Path -Parent $Path
    New-Item -ItemType Directory -Path $directory -Force | Out-Null
    $temporary = "$Path.tmp-$([guid]::NewGuid().ToString('N'))"
    try {
        [IO.File]::WriteAllText($temporary, (($Manifest | ConvertTo-Json -Depth 8) + [Environment]::NewLine), [Text.UTF8Encoding]::new($false))
        Move-Item -LiteralPath $temporary -Destination $Path -Force
    } finally {
        Remove-Item -LiteralPath $temporary -Force -ErrorAction SilentlyContinue
    }
}

function Read-RuntimeManifest {
    if (-not (Test-Path -LiteralPath $script:ManifestPath -PathType Leaf)) {
        throw "Runtime manifest does not exist: $($script:ManifestPath)"
    }
    $manifest = Get-Content -LiteralPath $script:ManifestPath -Raw | ConvertFrom-Json
    Assert-RuntimeManifest -Manifest $manifest
    return $manifest
}

function Assert-RepositoryState {
    param([Parameter(Mandatory = $true)]$Repositories)

    foreach ($name in @('desktop', 'cloud', 'admin')) {
        $recorded = $Repositories.$name
        if (-not $recorded) { throw "Runtime manifest is missing repository: $name" }
        $current = Get-GitMetadata -Root ([string]$recorded.root)
        if ($current.root -ne [string]$recorded.root -or $current.branch -ne [string]$recorded.branch -or $current.head -ne [string]$recorded.head) {
            throw "Runtime manifest repository state is stale: $name"
        }
    }
}

function Get-ProcessRecord {
    param([Parameter(Mandatory = $true)][int]$ProcessID)
    $process = Get-CimInstance Win32_Process -Filter "ProcessId = $ProcessID"
    if (-not $process) { throw "Runtime process $ProcessID is not running." }
    return $process
}

function Get-ProcessCreationTime {
    param([Parameter(Mandatory = $true)]$Process)
    return ([datetime]$Process.CreationDate).ToUniversalTime().ToString('o')
}

function Test-ProcessCreationTime {
    param(
        [Parameter(Mandatory = $true)]$Process,
        [Parameter(Mandatory = $true)]$RecordedCreationTime
    )

    try {
        $recorded = if ($RecordedCreationTime -is [datetime]) {
            ([datetime]$RecordedCreationTime).ToUniversalTime()
        } else {
            [datetime]::Parse(
                [string]$RecordedCreationTime,
                [Globalization.CultureInfo]::InvariantCulture,
                [Globalization.DateTimeStyles]::RoundtripKind
            ).ToUniversalTime()
        }
    } catch {
        return $false
    }
    $actual = ([datetime]$Process.CreationDate).ToUniversalTime()
    return [Math]::Abs(($actual - $recorded).TotalMilliseconds) -le 10
}

function Get-DescendantProcessIDs {
    param([Parameter(Mandatory = $true)][int]$RootPID)
    $all = @(Get-CimInstance Win32_Process)
    $ids = [Collections.Generic.List[int]]::new()
    $pending = [Collections.Generic.Queue[int]]::new()
    $pending.Enqueue($RootPID)
    while ($pending.Count -gt 0) {
        $parent = $pending.Dequeue()
        foreach ($child in $all | Where-Object { [int]$_.ParentProcessId -eq $parent }) {
            $id = [int]$child.ProcessId
            $ids.Add($id)
            $pending.Enqueue($id)
        }
    }
    return @($RootPID) + @($ids)
}

function Assert-ServiceOwnership {
    param([Parameter(Mandatory = $true)][string]$Name, [Parameter(Mandatory = $true)]$Service)

    $process = Get-ProcessRecord -ProcessID ([int]$Service.pid)
    if (-not (Test-ProcessCreationTime -Process $process -RecordedCreationTime $Service.creation_time)) {
        throw "$Name PID was reused or has a different start time."
    }
    $identity = "$($process.ExecutablePath)`n$($process.CommandLine)"
    if ($identity.IndexOf([string]$Service.entry_path, [StringComparison]::OrdinalIgnoreCase) -lt 0) {
        throw "$Name command line does not own the recorded entry path."
    }
    $listener = Get-NetTCPConnection -State Listen -LocalPort ([int]$Service.port) -ErrorAction SilentlyContinue | Select-Object -First 1
    if (-not $listener) { throw "$Name port $($Service.port) is not listening." }
    if ((Get-DescendantProcessIDs -RootPID ([int]$Service.pid)) -notcontains [int]$listener.OwningProcess) {
        throw "$Name listener is not owned by the recorded process tree."
    }
    return $process
}

function Wait-HttpOK {
    param([Parameter(Mandatory = $true)][string]$Name, [Parameter(Mandatory = $true)][string]$URI, [int]$TimeoutSeconds = 120)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $last = 'not attempted'
    while ((Get-Date) -lt $deadline) {
        try {
            $response = Invoke-WebRequest -Uri $URI -UseBasicParsing -TimeoutSec 5
            if ([int]$response.StatusCode -ge 200 -and [int]$response.StatusCode -lt 400) { return }
            $last = "HTTP $($response.StatusCode)"
        } catch { $last = $_.Exception.Message }
        Start-Sleep -Seconds 2
    }
    throw "$Name did not become healthy: $last"
}

function Start-WithEnvironment {
    param([hashtable]$Environment, [scriptblock]$Start)
    $saved = @{}
    try {
        foreach ($name in $Environment.Keys) {
            $saved[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
            [Environment]::SetEnvironmentVariable($name, [string]$Environment[$name], 'Process')
        }
        return & $Start
    } finally {
        foreach ($name in $Environment.Keys) {
            [Environment]::SetEnvironmentVariable($name, $saved[$name], 'Process')
        }
    }
}

function New-ServiceRecord {
    param([Parameter(Mandatory = $true)]$Process, [Parameter(Mandatory = $true)][int]$Port, [Parameter(Mandatory = $true)][string]$EntryPath)
    $snapshot = Get-ProcessRecord -ProcessID $Process.Id
    return [ordered]@{
        pid = [int]$Process.Id
        creation_time = Get-ProcessCreationTime $snapshot
        port = $Port
        entry_path = [IO.Path]::GetFullPath($EntryPath)
    }
}

function Assert-NoForeignListeners {
    foreach ($port in @($script:CloudPort, $script:AdminPort, $script:DesktopVitePort, $script:DesktopWailsPort)) {
        $listener = Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($listener) { throw "Port $port is already owned by PID $($listener.OwningProcess); refusing to stop or replace it." }
    }
}

function Invoke-VerifyRuntime {
    $manifest = Read-RuntimeManifest
    $cloudOrigin = ([string]$manifest.cloud_base_url).TrimEnd('/')
    Assert-RepositoryState -Repositories $manifest.repositories
    Assert-ServiceOwnership -Name 'Cloud' -Service $manifest.services.cloud | Out-Null
    Assert-ServiceOwnership -Name 'Admin' -Service $manifest.services.admin | Out-Null
    Assert-ServiceOwnership -Name 'Desktop Vite' -Service $manifest.services.desktop_vite | Out-Null
    Assert-ServiceOwnership -Name 'Desktop Wails' -Service $manifest.services.desktop_wails | Out-Null
    Wait-HttpOK -Name 'Cloud health' -URI "$cloudOrigin/healthz" -TimeoutSeconds 20
    Wait-HttpOK -Name 'Cloud readiness' -URI "$cloudOrigin/readyz" -TimeoutSeconds 20
    $identity = (Invoke-WebRequest -Uri "$cloudOrigin/environmentz" -UseBasicParsing -TimeoutSec 5).Content | ConvertFrom-Json
    if ([string]$identity.data.environment_id -ne [string]$manifest.cloud_environment_id) { throw 'Cloud environment identity no longer matches the runtime manifest.' }
    Wait-HttpOK -Name 'Admin proxy' -URI "http://127.0.0.1:$($script:AdminPort)/__proxy/healthz" -TimeoutSeconds 20
    Wait-HttpOK -Name 'Desktop Vite' -URI "http://127.0.0.1:$($script:DesktopVitePort)/" -TimeoutSeconds 20
    Wait-HttpOK -Name 'Desktop Wails' -URI "http://127.0.0.1:$($script:DesktopWailsPort)/" -TimeoutSeconds 20
    Write-Host "[verified] $($manifest.cloud_base_url) environment $($manifest.cloud_environment_name)"
    return $manifest
}

function Invoke-StopRuntime {
    $manifest = Read-RuntimeManifest
    $validated = @(
        @{ name = 'Desktop'; service = $manifest.services.desktop_vite },
        @{ name = 'Admin'; service = $manifest.services.admin },
        @{ name = 'Cloud'; service = $manifest.services.cloud }
    )
    foreach ($item in $validated) { Assert-ServiceOwnership -Name $item.name -Service $item.service | Out-Null }
    foreach ($item in $validated) {
        $ids = @(Get-DescendantProcessIDs -RootPID ([int]$item.service.pid))
        for ($index = $ids.Count - 1; $index -ge 0; $index--) { Stop-Process -Id $ids[$index] -Force -ErrorAction SilentlyContinue }
    }
    Move-Item -LiteralPath $script:ManifestPath -Destination "$($script:ManifestPath).stopped-$(Get-Date -Format 'yyyyMMdd-HHmmss')"
    Write-Host '[stopped] verified Reasonix development process trees'
}

if ($LibraryOnly) { return }

$canonicalCloudURL = ConvertTo-CanonicalCloudURL $CloudBaseURL
if ($RunDesktopChild) {
    $env:KWU_DESKTOP_VITE_PORT = [string]$script:DesktopVitePort
    $env:KWU_DESKTOP_WAILS_PORT = [string]$script:DesktopWailsPort
    Set-Location (Join-Path $script:DesktopRoot 'desktop')
    $workFile = & node (Join-Path $script:DesktopRoot 'scripts/prepare-wails-dev.mjs') (Get-Location).Path
    if ($LASTEXITCODE -ne 0) { throw 'Wails development runtime preparation failed.' }
    $env:GOWORK = ([string]$workFile).Trim()
    & (Get-Command wails -ErrorAction Stop).Source dev -nosyncgomod -m -skipbindings -s -devserver "127.0.0.1:$($script:DesktopWailsPort)" -nocolour
    exit $LASTEXITCODE
}

switch ($Command) {
    'status' { Invoke-VerifyRuntime | ConvertTo-Json -Depth 8; break }
    'verify' { Invoke-VerifyRuntime | Out-Null; break }
    'down' { Invoke-StopRuntime; break }
    'up' {
        if ($env:REASONIX_CLOUD_CONFIG) { throw 'REASONIX_CLOUD_CONFIG is inherited; clear it before canonical startup.' }
        if ($env:KWU_CLOUD_BASE_URL) { throw 'KWU_CLOUD_BASE_URL is inherited; clear it before canonical startup.' }
        $repositories = Resolve-IntegrationRoots -RequestedCloudRoot $CloudRoot -RequestedAdminRoot $AdminRoot
        New-Item -ItemType Directory -Path $script:RuntimeRoot -Force | Out-Null
        $stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
        $cloudExecutable = Join-Path $script:RuntimeRoot "reasonix-cloud-$($repositories.cloud.head.Substring(0, 12)).exe"
        & go -C $repositories.cloud.root build -o $cloudExecutable ./cmd/server
        if ($LASTEXITCODE -ne 0) { throw 'Cloud build failed.' }
        Assert-NoForeignListeners
        $cloudProcess = Start-Process -FilePath $cloudExecutable -WorkingDirectory $repositories.cloud.root -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $script:RuntimeRoot "cloud-$stamp.out.log") -RedirectStandardError (Join-Path $script:RuntimeRoot "cloud-$stamp.err.log")
        $cloudOrigin = $canonicalCloudURL.TrimEnd('/')
        Wait-HttpOK -Name 'Cloud readiness' -URI "$cloudOrigin/readyz"
        $identity = (Invoke-WebRequest -Uri "$cloudOrigin/environmentz" -UseBasicParsing -TimeoutSec 5).Content | ConvertFrom-Json

        $vite = Join-Path $repositories.admin.root 'node_modules\vite\bin\vite.js'
        $adminProcess = Start-WithEnvironment -Environment @{ VITE_PROXY = ConvertTo-AdminProxyJSON $canonicalCloudURL } -Start {
            Start-Process -FilePath (Get-Command node -ErrorAction Stop).Source -ArgumentList @($vite, '--host', '127.0.0.1', '--port', [string]$script:AdminPort, '--strictPort') -WorkingDirectory $repositories.admin.root -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $script:RuntimeRoot "admin-$stamp.out.log") -RedirectStandardError (Join-Path $script:RuntimeRoot "admin-$stamp.err.log")
        }
        Wait-HttpOK -Name 'Admin proxy' -URI "http://127.0.0.1:$($script:AdminPort)/__proxy/healthz"

        # Desktop validates this trusted, Cloud-verified identity before it
        # constructs authenticated state. Publish the bootstrap contract first;
        # the complete process inventory replaces it after Wails is listening.
        $bootstrapManifest = [ordered]@{
            schema_version = 1
            cloud_base_url = $canonicalCloudURL
            cloud_environment_id = [string]$identity.data.environment_id
            cloud_environment_name = [string]$identity.data.environment_name
            created_at = (Get-Date).ToUniversalTime().ToString('o')
            repositories = $repositories
            services = [ordered]@{
                cloud = New-ServiceRecord -Process $cloudProcess -Port $script:CloudPort -EntryPath $cloudExecutable
                admin = New-ServiceRecord -Process $adminProcess -Port $script:AdminPort -EntryPath $vite
            }
        }
        Write-AtomicRuntimeManifest -Manifest $bootstrapManifest -Path $script:ManifestPath

        $desktopProcess = Start-WithEnvironment -Environment @{ KWU_CLOUD_BASE_URL = $canonicalCloudURL; REASONIX_DEV_RUNTIME_MANIFEST = $script:ManifestPath } -Start {
            Start-Process -FilePath 'powershell.exe' -ArgumentList @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $PSCommandPath, '-RunDesktopChild') -WorkingDirectory $script:DesktopRoot -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $script:RuntimeRoot "desktop-$stamp.out.log") -RedirectStandardError (Join-Path $script:RuntimeRoot "desktop-$stamp.err.log")
        }
        Wait-HttpOK -Name 'Desktop Vite' -URI "http://127.0.0.1:$($script:DesktopVitePort)/"
        Wait-HttpOK -Name 'Desktop Wails' -URI "http://127.0.0.1:$($script:DesktopWailsPort)/"

        $manifest = [ordered]@{
            schema_version = 1
            cloud_base_url = $canonicalCloudURL
            cloud_environment_id = [string]$identity.data.environment_id
            cloud_environment_name = [string]$identity.data.environment_name
            created_at = (Get-Date).ToUniversalTime().ToString('o')
            repositories = $repositories
            services = [ordered]@{
                cloud = New-ServiceRecord -Process $cloudProcess -Port $script:CloudPort -EntryPath $cloudExecutable
                admin = New-ServiceRecord -Process $adminProcess -Port $script:AdminPort -EntryPath $vite
                desktop_vite = New-ServiceRecord -Process $desktopProcess -Port $script:DesktopVitePort -EntryPath $PSCommandPath
                desktop_wails = New-ServiceRecord -Process $desktopProcess -Port $script:DesktopWailsPort -EntryPath $PSCommandPath
            }
        }
        Write-AtomicRuntimeManifest -Manifest $manifest -Path $script:ManifestPath
        Invoke-VerifyRuntime | Out-Null
        break
    }
}

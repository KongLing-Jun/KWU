[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [ValidateSet('amd64', 'arm64')]
    [string]$Architecture,
    [string]$CacheDirectory = ''
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$repositoryRoot = Split-Path -Parent $PSScriptRoot
$lockPath = Join-Path $repositoryRoot 'desktop/build/windows/python-runtime.lock.json'
$runtimeRoot = Join-Path $repositoryRoot 'desktop/build/windows/python-runtime'
$destination = Join-Path $runtimeRoot $Architecture

if (-not (Test-Path -LiteralPath $lockPath -PathType Leaf)) {
    throw "Python runtime lock manifest does not exist: $lockPath"
}

$lock = Get-Content -LiteralPath $lockPath -Raw | ConvertFrom-Json
if ($lock.schema -ne 1 -or [string]::IsNullOrWhiteSpace($lock.version)) {
    throw "Unsupported Python runtime lock manifest: $lockPath"
}
$artifact = $lock.artifacts.$Architecture
if (-not $artifact -or
    [string]::IsNullOrWhiteSpace($artifact.url) -or
    [int64]$artifact.bytes -le 0 -or
    $artifact.sha256 -notmatch '^[0-9a-fA-F]{64}$' -or
    [int]$artifact.payload_files -le 0 -or
    $artifact.payload_sha256 -notmatch '^[0-9a-f]{64}$') {
    throw "Python runtime lock manifest has no valid $Architecture artifact"
}

$expectedVersion = [string]$lock.version
$expectedSize = [int64]$artifact.bytes
$expectedHash = ([string]$artifact.sha256).ToLowerInvariant()
$expectedPayloadFiles = [int]$artifact.payload_files
$expectedPayloadHash = [string]$artifact.payload_sha256

function Test-FileHashAndSize {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { return $false }
    $file = Get-Item -LiteralPath $Path
    if ($file.Length -ne $expectedSize) { return $false }
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant() -eq $expectedHash
}

function Get-PayloadReceipt {
    param([string]$Path)

    $directories = @(Get-ChildItem -LiteralPath $Path -Recurse -Directory)
    if ($directories.Count -ne 0) {
        throw "Managed Python payload contains unexpected directories: $Path"
    }

    $paths = [System.Collections.Generic.List[string]]::new()
    $entriesByPath = [System.Collections.Generic.Dictionary[string, object]]::new(
        [System.StringComparer]::Ordinal
    )
    foreach ($file in Get-ChildItem -LiteralPath $Path -Recurse -File) {
        $relative = [System.IO.Path]::GetRelativePath($Path, $file.FullName).Replace('\', '/')
        if ($relative -ceq '.kwu-runtime.json') { continue }
        if ([System.IO.Path]::IsPathRooted($relative) -or
            $relative.Contains('\') -or
            $relative -match '(^|/)\.\.($|/)' -or
            ($file.Attributes -band [System.IO.FileAttributes]::ReparsePoint)) {
            throw "Managed Python payload contains an unsafe path: $($file.FullName)"
        }
        $entry = [pscustomobject][ordered]@{
            path = $relative
            size = [int64]$file.Length
            sha256 = (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
        }
        $entriesByPath.Add($relative, $entry)
        $paths.Add($relative)
    }

    $sortedPaths = [string[]]$paths.ToArray()
    [System.Array]::Sort($sortedPaths, [System.StringComparer]::Ordinal)
    $files = [System.Collections.Generic.List[object]]::new()
    $canonical = [System.Text.StringBuilder]::new()
    foreach ($relative in $sortedPaths) {
        $entry = $entriesByPath[$relative]
        $files.Add($entry)
        [void]$canonical.Append($entry.sha256)
        [void]$canonical.Append('  ')
        [void]$canonical.Append($entry.size.ToString([System.Globalization.CultureInfo]::InvariantCulture))
        [void]$canonical.Append('  ')
        [void]$canonical.Append($entry.path)
        [void]$canonical.Append([char]10)
    }
    $canonicalBytes = [System.Text.UTF8Encoding]::new($false).GetBytes($canonical.ToString())
    $payloadHash = [System.Convert]::ToHexString(
        [System.Security.Cryptography.SHA256]::HashData($canonicalBytes)
    ).ToLowerInvariant()
    return [pscustomobject]@{ files = $files.ToArray(); payload_sha256 = $payloadHash }
}

function Test-RuntimeDirectory {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path -PathType Container)) { return $false }
    foreach ($required in @('python.exe', 'python3.exe', 'LICENSE.txt', 'python312._pth.kwu-disabled', '.kwu-runtime.json')) {
        if (-not (Test-Path -LiteralPath (Join-Path $Path $required) -PathType Leaf)) { return $false }
    }
    if (Test-Path -LiteralPath (Join-Path $Path 'python312._pth')) { return $false }
    try {
        $actual = Get-Content -LiteralPath (Join-Path $Path '.kwu-runtime.json') -Raw | ConvertFrom-Json
        $manifestKeys = @($actual.PSObject.Properties.Name)
        $expectedKeys = @('schema', 'version', 'arch', 'source_sha256', 'source_size', 'payload_sha256', 'files')
        if ($manifestKeys.Count -ne $expectedKeys.Count -or
            @($expectedKeys | Where-Object { $_ -cnotin $manifestKeys }).Count -ne 0 -or
            $actual.schema -ne 1 -or
            $actual.version -cne $expectedVersion -or
            $actual.arch -cne $Architecture -or
            $actual.source_sha256 -cne $expectedHash -or
            [int64]$actual.source_size -ne $expectedSize -or
            $actual.payload_sha256 -cne $expectedPayloadHash) {
            return $false
        }
        $receipt = Get-PayloadReceipt $Path
        $manifestFiles = @($actual.files)
        $receiptFiles = @($receipt.files)
        if ($receipt.payload_sha256 -cne $expectedPayloadHash -or
            $manifestFiles.Count -ne $expectedPayloadFiles -or
            $receiptFiles.Count -ne $expectedPayloadFiles) {
            return $false
        }
        for ($index = 0; $index -lt $receiptFiles.Count; $index++) {
            $manifestEntry = $manifestFiles[$index]
            $receiptEntry = $receiptFiles[$index]
            $entryKeys = @($manifestEntry.PSObject.Properties.Name)
            if ($entryKeys.Count -ne 3 -or
                @(@('path', 'size', 'sha256') | Where-Object { $_ -cnotin $entryKeys }).Count -ne 0 -or
                $manifestEntry.path -cne $receiptEntry.path -or
                [int64]$manifestEntry.size -ne $receiptEntry.size -or
                $manifestEntry.sha256 -cne $receiptEntry.sha256) {
                return $false
            }
        }
        return $true
    }
    catch {
        return $false
    }
}

function Test-RuntimeDirectoryMatches {
    param([string]$Expected, [string]$Actual)
    if (-not (Test-RuntimeDirectory $Expected) -or -not (Test-RuntimeDirectory $Actual)) { return $false }
    $expectedFiles = @(Get-ChildItem -LiteralPath $Expected -Recurse -File)
    $actualFiles = @(Get-ChildItem -LiteralPath $Actual -Recurse -File)
    if ($expectedFiles.Count -ne $actualFiles.Count) { return $false }
    foreach ($expectedFile in $expectedFiles) {
        $relative = [System.IO.Path]::GetRelativePath($Expected, $expectedFile.FullName)
        $actualPath = Join-Path $Actual $relative
        if (-not (Test-Path -LiteralPath $actualPath -PathType Leaf)) { return $false }
        $actualFile = Get-Item -LiteralPath $actualPath
        if ($actualFile.Length -ne $expectedFile.Length) { return $false }
        $expectedDigest = (Get-FileHash -LiteralPath $expectedFile.FullName -Algorithm SHA256).Hash
        $actualDigest = (Get-FileHash -LiteralPath $actualPath -Algorithm SHA256).Hash
        if ($actualDigest -cne $expectedDigest) { return $false }
    }
    return $true
}

if ([string]::IsNullOrWhiteSpace($CacheDirectory)) {
    if (-not [string]::IsNullOrWhiteSpace($env:KWU_PYTHON_RUNTIME_CACHE)) {
        $CacheDirectory = $env:KWU_PYTHON_RUNTIME_CACHE
    }
    elseif (-not [string]::IsNullOrWhiteSpace($env:LOCALAPPDATA)) {
        $CacheDirectory = Join-Path $env:LOCALAPPDATA 'Clouvia/build-cache/python-runtime'
    }
    else {
        $CacheDirectory = Join-Path ([System.IO.Path]::GetTempPath()) 'Clouvia/build-cache/python-runtime'
    }
}
New-Item -ItemType Directory -Path $CacheDirectory -Force | Out-Null
$archivePath = Join-Path $CacheDirectory "$expectedHash.zip"

if (-not (Test-FileHashAndSize $archivePath)) {
    if (Test-Path -LiteralPath $archivePath) {
        throw "Cached Python artifact failed size or SHA-256 validation: $archivePath"
    }
    $downloadPath = Join-Path $CacheDirectory (".$expectedHash.download." + [guid]::NewGuid().ToString('N'))
    try {
        Invoke-WebRequest -UseBasicParsing -Uri ([string]$artifact.url) -OutFile $downloadPath
        if (-not (Test-FileHashAndSize $downloadPath)) {
            throw "Downloaded Python artifact failed size or SHA-256 validation: $($artifact.url)"
        }
        try {
            Move-Item -LiteralPath $downloadPath -Destination $archivePath
        }
        catch {
            if (-not (Test-FileHashAndSize $archivePath)) { throw }
        }
    }
    finally {
        Remove-Item -LiteralPath $downloadPath -Force -ErrorAction SilentlyContinue
    }
}

New-Item -ItemType Directory -Path $runtimeRoot -Force | Out-Null
$extractDirectory = Join-Path $runtimeRoot (".$Architecture.tmp." + [guid]::NewGuid().ToString('N'))
try {
    New-Item -ItemType Directory -Path $extractDirectory | Out-Null
    Expand-Archive -LiteralPath $archivePath -DestinationPath $extractDirectory
    $pthPath = Join-Path $extractDirectory 'python312._pth'
    if (-not (Test-Path -LiteralPath $pthPath -PathType Leaf)) {
        throw "Embedded Python archive does not contain python312._pth: $archivePath"
    }
    Move-Item -LiteralPath $pthPath -Destination (Join-Path $extractDirectory 'python312._pth.kwu-disabled')
    Copy-Item -LiteralPath (Join-Path $extractDirectory 'python.exe') -Destination (Join-Path $extractDirectory 'python3.exe')
    $receipt = Get-PayloadReceipt $extractDirectory
    if (@($receipt.files).Count -ne $expectedPayloadFiles -or
        $receipt.payload_sha256 -cne $expectedPayloadHash) {
        throw "Prepared Python payload does not match the pinned file receipt: $archivePath (actual files=$(@($receipt.files).Count) sha256=$($receipt.payload_sha256); expected files=$expectedPayloadFiles sha256=$expectedPayloadHash)"
    }
    $runtimeManifest = [ordered]@{
        schema = 1
        version = $expectedVersion
        arch = $Architecture
        source_sha256 = $expectedHash
        source_size = $expectedSize
        payload_sha256 = $receipt.payload_sha256
        files = @($receipt.files)
    }
    $manifestJSON = $runtimeManifest | ConvertTo-Json -Depth 4 -Compress
    $utf8WithoutBom = [System.Text.UTF8Encoding]::new($false)
    [System.IO.File]::WriteAllText((Join-Path $extractDirectory '.kwu-runtime.json'), $manifestJSON, $utf8WithoutBom)
    if (-not (Test-RuntimeDirectory $extractDirectory)) {
        throw "Prepared Python runtime failed validation: $extractDirectory"
    }
    if (Test-Path -LiteralPath $destination) {
        if (Test-RuntimeDirectoryMatches $extractDirectory $destination) {
            Write-Host "Python $expectedVersion $Architecture runtime is ready and matches the pinned archive: $destination"
            return
        }
        throw "Refusing to replace a Python runtime staging directory that differs from the pinned archive: $destination"
    }
    try {
        [System.IO.Directory]::Move($extractDirectory, $destination)
    }
    catch {
        if (-not (Test-RuntimeDirectoryMatches $extractDirectory $destination)) { throw }
    }
}
finally {
    Remove-Item -LiteralPath $extractDirectory -Recurse -Force -ErrorAction SilentlyContinue
}

if (-not (Test-RuntimeDirectory $destination)) {
    throw "Python runtime was not published successfully: $destination"
}
Write-Host "Prepared Python $expectedVersion $Architecture runtime: $destination"

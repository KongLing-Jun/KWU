[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [ValidateSet('amd64', 'arm64')]
    [string]$Architecture,
    [string]$ApplicationPath = '',
    [string]$InstallerPath = '',
    [string]$PayloadDirectory = '',
    [string]$SevenZipPath = '',
    [string]$ExpectedVersion = '',
    [string]$ExpectedProductVersionString = '',
    [string]$ExpectedProductName = '',
    [string]$ExpectedFileDescription = '',
    [string]$ExpectedCompanyName = '',
    [switch]$RequireSignature,
    [switch]$RequireTrustedSignature,
    [string]$ExpectedSignerThumbprint = '',
    [string]$ExpectedSignerSubject = '',
    [string]$SignToolPath = '',
    [string]$PythonRuntimeLockPath = ''
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Resolve-InputFile {
    param([string]$Path, [string]$Description)
    if ([string]::IsNullOrWhiteSpace($Path)) { return '' }
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "$Description does not exist: $Path"
    }
    return (Resolve-Path -LiteralPath $Path).Path
}

function Resolve-InputDirectory {
    param([string]$Path, [string]$Description)
    if ([string]::IsNullOrWhiteSpace($Path)) { return '' }
    if (-not (Test-Path -LiteralPath $Path -PathType Container)) {
        throw "$Description does not exist: $Path"
    }
    return (Resolve-Path -LiteralPath $Path).Path
}

function Assert-PEArchitecture {
    param([string]$Path, [string]$ExpectedArchitecture)

    $stream = [System.IO.File]::OpenRead($Path)
    $reader = [System.IO.BinaryReader]::new($stream)
    try {
        if ($stream.Length -lt 64 -or $reader.ReadUInt16() -ne 0x5A4D) {
            throw "Not a PE executable: $Path"
        }
        $stream.Position = 0x3c
        $peOffset = $reader.ReadInt32()
        if ($peOffset -lt 0 -or ([int64]$peOffset + 6) -gt $stream.Length) {
            throw "Invalid PE header offset: $Path"
        }
        $stream.Position = $peOffset
        if ($reader.ReadUInt32() -ne 0x00004550) {
            throw "Missing PE signature: $Path"
        }
        $machine = $reader.ReadUInt16()
    }
    finally {
        $reader.Dispose()
        $stream.Dispose()
    }

    $expectedMachine = if ($ExpectedArchitecture -eq 'amd64') { 0x8664 } else { 0xAA64 }
    if ($machine -ne $expectedMachine) {
        throw ('PE architecture mismatch for {0}: machine=0x{1:X4}, expected {2} (0x{3:X4})' -f `
            $Path, $machine, $ExpectedArchitecture, $expectedMachine)
    }
}

function ConvertTo-RawVersion {
    param([string]$Version)
    $normalized = $Version.Trim()
    if ($normalized.StartsWith('v', [StringComparison]::OrdinalIgnoreCase)) {
        $normalized = $normalized.Substring(1)
    }
    $normalized = ($normalized -split '-', 2)[0]
    $parts = @($normalized -split '\.')
    if ($parts.Count -eq 3) {
        $parts += '0'
    }
    if ($parts.Count -ne 4 -or @($parts | Where-Object { $_ -notmatch '^\d+$' }).Count -ne 0) {
        throw "Expected version must be X.Y.Z or X.Y.Z.W: $Version"
    }
    return ($parts -join '.')
}

function Assert-PEVersionInfo {
    param(
        [string]$Path,
        [string]$Version,
        [string]$ProductVersionString,
        [string]$ProductName,
        [string]$FileDescription,
        [string]$CompanyName
    )

    if ([string]::IsNullOrWhiteSpace($Version) -and
        [string]::IsNullOrWhiteSpace($ProductName) -and
        [string]::IsNullOrWhiteSpace($CompanyName)) {
        return
    }
    if ([string]::IsNullOrWhiteSpace($Version) -or
        [string]::IsNullOrWhiteSpace($ProductName) -or
        [string]::IsNullOrWhiteSpace($CompanyName)) {
        throw 'ExpectedVersion, ExpectedProductName and ExpectedCompanyName must be provided together'
    }

    $expectedRaw = ConvertTo-RawVersion $Version
    if ([string]::IsNullOrWhiteSpace($ProductVersionString)) {
        $ProductVersionString = $Version.Trim()
        if ($ProductVersionString.StartsWith('v', [StringComparison]::OrdinalIgnoreCase)) {
            $ProductVersionString = $ProductVersionString.Substring(1)
        }
        $ProductVersionString = ($ProductVersionString -split '-', 2)[0]
    }
    if ([string]::IsNullOrWhiteSpace($FileDescription)) {
        $FileDescription = $ProductName
    }
    $metadata = [System.Diagnostics.FileVersionInfo]::GetVersionInfo($Path)
    $actualFileRaw = $metadata.FileVersionRaw.ToString()
    $actualProductRaw = $metadata.ProductVersionRaw.ToString()
    if ($actualFileRaw -ne $expectedRaw -or $actualProductRaw -ne $expectedRaw) {
        throw "VersionInfo raw version mismatch for $Path`: file=$actualFileRaw product=$actualProductRaw expected=$expectedRaw"
    }
    if ($metadata.ProductName -cne $ProductName -or $metadata.FileDescription -cne $FileDescription) {
        throw "VersionInfo product strings mismatch for $Path`: ProductName=$($metadata.ProductName) expectedProduct=$ProductName FileDescription=$($metadata.FileDescription) expectedDescription=$FileDescription"
    }
    if ($metadata.CompanyName -cne $CompanyName) {
        throw "VersionInfo CompanyName mismatch for $Path`: actual=$($metadata.CompanyName) expected=$CompanyName"
    }
    if ($metadata.ProductVersion -cne $ProductVersionString) {
        throw "VersionInfo ProductVersion mismatch for $Path`: actual=$($metadata.ProductVersion) expected=$ProductVersionString"
    }
    Write-Host "Verified Windows VersionInfo: $ProductName $expectedRaw ($Path)"
}

function Get-ManagedPythonPayloadReceipt {
    param([string]$RuntimeDirectory)

    $paths = [System.Collections.Generic.List[string]]::new()
    $entriesByPath = [System.Collections.Generic.Dictionary[string, object]]::new(
        [System.StringComparer]::Ordinal
    )
    foreach ($item in Get-ChildItem -LiteralPath $RuntimeDirectory -Force -Recurse) {
        if ($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) {
            throw "NSIS managed Python runtime contains a reparse point: $($item.FullName)"
        }
        if ($item.PSIsContainer) {
            throw "NSIS managed Python runtime contains an unexpected directory: $($item.FullName)"
        }
        if ($item -isnot [System.IO.FileInfo]) {
            throw "NSIS managed Python runtime contains a non-regular entry: $($item.FullName)"
        }

        $relative = [System.IO.Path]::GetRelativePath($RuntimeDirectory, $item.FullName).Replace('\', '/')
        if ($relative -ceq '.kwu-runtime.json') { continue }
        if ([System.IO.Path]::IsPathRooted($relative) -or
            $relative.Contains('\') -or
            $relative -match '(^|/)\.\.($|/)' -or
            $relative -match '[\r\n]') {
            throw "NSIS managed Python runtime contains an unsafe payload path: $($item.FullName)"
        }
        $entry = [pscustomobject][ordered]@{
            path = $relative
            size = [int64]$item.Length
            sha256 = (Get-FileHash -LiteralPath $item.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
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

function Assert-ManagedPythonRuntime {
    param(
        [string]$ExtractDirectory,
        [string]$ExpectedArchitecture,
        [string]$LockPath
    )

    if ([string]::IsNullOrWhiteSpace($LockPath)) {
        $LockPath = Join-Path (Split-Path -Parent $PSScriptRoot) 'desktop/build/windows/python-runtime.lock.json'
    }
    $resolvedLockPath = Resolve-InputFile $LockPath 'Python runtime lock manifest'
    $runtimeLock = Get-Content -LiteralPath $resolvedLockPath -Raw | ConvertFrom-Json
    $lockedArtifact = $runtimeLock.artifacts.$ExpectedArchitecture
    if ($runtimeLock.schema -ne 1 -or
        [string]::IsNullOrWhiteSpace([string]$runtimeLock.version) -or
        -not $lockedArtifact -or
        [int64]$lockedArtifact.bytes -le 0 -or
        $lockedArtifact.sha256 -notmatch '^[0-9a-f]{64}$' -or
        [int]$lockedArtifact.payload_files -le 0 -or
        $lockedArtifact.payload_sha256 -notmatch '^[0-9a-f]{64}$') {
        throw "Python runtime lock manifest has no valid $ExpectedArchitecture artifact: $resolvedLockPath"
    }

    $runtimeEntries = @(Get-ChildItem -LiteralPath $ExtractDirectory -Force -Recurse |
        Where-Object { $_.FullName.Replace('/', '\') -match '\\runtimes\\python\\' })
    foreach ($entry in $runtimeEntries) {
        if ($entry.Attributes -band [System.IO.FileAttributes]::ReparsePoint) {
            throw "Managed Python namespace contains a reparse point: $($entry.FullName)"
        }
    }
    $runtimeFiles = @($runtimeEntries | Where-Object { $_ -is [System.IO.FileInfo] })
    $pythonExecutables = @($runtimeFiles | Where-Object { $_.Name -ieq 'python.exe' })
    if ($pythonExecutables.Count -ne 1) {
        throw "NSIS installer must contain exactly one managed Python executable; found $($pythonExecutables.Count)"
    }
    $normalizedPythonPath = $pythonExecutables[0].FullName.Replace('/', '\')
    $expectedSuffixes = @(
        "\runtimes\python\$($runtimeLock.version)\.$ExpectedArchitecture.staging\python.exe",
        "\runtimes\python\$($runtimeLock.version)\$ExpectedArchitecture\python.exe"
    )
    $matchesExpectedRuntime = @($expectedSuffixes | Where-Object {
        $normalizedPythonPath.EndsWith($_, [StringComparison]::OrdinalIgnoreCase)
    }).Count -eq 1
    if (-not $matchesExpectedRuntime) {
        throw "Managed Python executable is not in the pinned runtime directory: $($pythonExecutables[0].FullName)"
    }
    Assert-PEArchitecture $pythonExecutables[0].FullName $ExpectedArchitecture

    $runtimeDirectory = $pythonExecutables[0].Directory.FullName
    $runtimeVersionDirectory = Split-Path -Parent $runtimeDirectory
    $runtimePrefix = $runtimeDirectory.TrimEnd([System.IO.Path]::DirectorySeparatorChar) +
        [System.IO.Path]::DirectorySeparatorChar
    foreach ($entry in $runtimeEntries) {
        $entryPath = $entry.FullName.TrimEnd([System.IO.Path]::DirectorySeparatorChar)
        $isRuntimeVersionDirectory = $entryPath.Equals(
            $runtimeVersionDirectory, [StringComparison]::OrdinalIgnoreCase
        )
        $isSelectedRuntime = $entryPath.Equals(
            $runtimeDirectory, [StringComparison]::OrdinalIgnoreCase
        ) -or $entryPath.StartsWith($runtimePrefix, [StringComparison]::OrdinalIgnoreCase)
        if (-not $isRuntimeVersionDirectory -and -not $isSelectedRuntime) {
            throw "Managed Python namespace contains an entry outside the pinned runtime directory: $($entry.FullName)"
        }
    }
    $licenses = @($runtimeFiles | Where-Object { $_.Name -ceq 'LICENSE.txt' })
    $manifests = @($runtimeFiles | Where-Object { $_.Name -ceq '.kwu-runtime.json' })
    if ($licenses.Count -ne 1 -or $licenses[0].Directory.FullName -cne $runtimeDirectory) {
        throw "NSIS managed Python runtime must contain exactly one colocated LICENSE.txt; found $($licenses.Count)"
    }
    if ($manifests.Count -ne 1 -or $manifests[0].Directory.FullName -cne $runtimeDirectory) {
        throw "NSIS managed Python runtime must contain exactly one colocated .kwu-runtime.json; found $($manifests.Count)"
    }

    $runtimeManifest = Get-Content -LiteralPath $manifests[0].FullName -Raw | ConvertFrom-Json
    $manifestKeys = @($runtimeManifest.PSObject.Properties.Name)
    $expectedKeys = @('schema', 'version', 'arch', 'source_sha256', 'source_size', 'payload_sha256', 'files')
    if ($manifestKeys.Count -ne $expectedKeys.Count -or
        @($expectedKeys | Where-Object { $_ -cnotin $manifestKeys }).Count -ne 0 -or
        $runtimeManifest.schema -ne 1 -or
        $runtimeManifest.version -cne [string]$runtimeLock.version -or
        $runtimeManifest.arch -cne $ExpectedArchitecture -or
        $runtimeManifest.source_sha256 -cne ([string]$lockedArtifact.sha256).ToLowerInvariant() -or
        [int64]$runtimeManifest.source_size -ne [int64]$lockedArtifact.bytes -or
        $runtimeManifest.payload_sha256 -cne [string]$lockedArtifact.payload_sha256 -or
        @($runtimeManifest.files).Count -ne [int]$lockedArtifact.payload_files) {
        throw "NSIS managed Python runtime manifest does not match the pinned lock manifest: $($manifests[0].FullName)"
    }

    $receipt = Get-ManagedPythonPayloadReceipt $runtimeDirectory
    $manifestFiles = @($runtimeManifest.files)
    $receiptFiles = @($receipt.files)
    if ($receipt.payload_sha256 -cne [string]$lockedArtifact.payload_sha256 -or
        $receiptFiles.Count -ne [int]$lockedArtifact.payload_files) {
        throw "NSIS managed Python runtime payload does not match the pinned receipt: $runtimeDirectory"
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
            throw "NSIS managed Python runtime file receipt differs from the extracted payload at index $index`: $runtimeDirectory"
        }
    }
    Write-Host "Verified managed Python $($runtimeManifest.version) runtime: $ExpectedArchitecture ($runtimeDirectory)"
}

function Resolve-SevenZip {
    if (-not [string]::IsNullOrWhiteSpace($SevenZipPath)) {
        return Resolve-InputFile $SevenZipPath '7-Zip executable'
    }
    if (-not [string]::IsNullOrWhiteSpace($env:SEVEN_ZIP_PATH)) {
        return Resolve-InputFile $env:SEVEN_ZIP_PATH 'SEVEN_ZIP_PATH'
    }
    foreach ($name in @('7z', '7zz', '7za')) {
        $command = Get-Command $name -ErrorAction SilentlyContinue
        if ($command) { return $command.Source }
    }
    foreach ($programFilesDirectory in @($env:ProgramFiles, ${env:ProgramFiles(x86)})) {
        if ([string]::IsNullOrWhiteSpace($programFilesDirectory)) { continue }
        $candidate = Join-Path $programFilesDirectory '7-Zip\7z.exe'
        if (Test-Path -LiteralPath $candidate -PathType Leaf) {
            return $candidate
        }
    }
    throw '7-Zip is required to inspect the NSIS payload; set -SevenZipPath or SEVEN_ZIP_PATH'
}

function Assert-Authenticode {
    param([string]$Path, [string]$Description)
    if (-not $RequireSignature -and -not $RequireTrustedSignature) { return }
    if ($RequireSignature -and $RequireTrustedSignature) {
        throw 'RequireSignature and RequireTrustedSignature are mutually exclusive'
    }
    if ($RequireSignature) {
        if ([string]::IsNullOrWhiteSpace($ExpectedSignerThumbprint) -or
            [string]::IsNullOrWhiteSpace($ExpectedSignerSubject)) {
            throw 'ExpectedSignerThumbprint and ExpectedSignerSubject are required with RequireSignature'
        }
        $helper = Join-Path $PSScriptRoot 'desktop-windows-authenticode.ps1'
        & $helper -Action Verify -Path $Path `
            -ExpectedThumbprint $ExpectedSignerThumbprint `
            -ExpectedSubject $ExpectedSignerSubject `
            -SignToolPath $SignToolPath
    }
    else {
        $tool = Resolve-ValidationSignTool
        $output = & $tool verify /pa /all /v $Path 2>&1
        if ($LASTEXITCODE -ne 0) {
            throw "SignTool trust verification failed for $Description`: $($output | Out-String)"
        }
        $signature = Get-AuthenticodeSignature -LiteralPath $Path
        if ($signature.Status -ne [System.Management.Automation.SignatureStatus]::Valid -or
            -not $signature.SignerCertificate -or -not $signature.TimeStamperCertificate) {
            throw "Trusted Authenticode signature or RFC 3161 timestamp is missing for $Description"
        }
    }
    Write-Host "Verified Windows Authenticode for $Description`: $Path"
}

function Resolve-ValidationSignTool {
    if ($SignToolPath) { return Resolve-InputFile $SignToolPath 'SignTool executable' }
    $command = Get-Command signtool.exe -ErrorAction SilentlyContinue
    if ($command) { return $command.Source }
    $kits = Join-Path ${env:ProgramFiles(x86)} 'Windows Kits\10\bin'
    $matches = @(Get-ChildItem -LiteralPath $kits -Filter signtool.exe -File -Recurse -ErrorAction SilentlyContinue |
        Where-Object { $_.FullName -match '\\x64\\signtool\.exe$' } |
        Sort-Object FullName -Descending)
    if ($matches.Count -eq 0) {
        throw 'SignTool is required for trusted signature validation; install the Windows SDK'
    }
    return $matches[0].FullName
}

$resolvedApplication = Resolve-InputFile $ApplicationPath 'Windows application'
$resolvedInstaller = Resolve-InputFile $InstallerPath 'Windows installer'
$resolvedPayloadDirectory = Resolve-InputDirectory $PayloadDirectory 'Windows payload directory'
if (-not $resolvedApplication -and -not $resolvedInstaller) {
    throw 'At least one of -ApplicationPath or -InstallerPath is required'
}
if ($resolvedPayloadDirectory) {
    if (-not $resolvedApplication) {
        throw 'PayloadDirectory requires ApplicationPath'
    }
    $expectedPayloadApplication = Join-Path $resolvedPayloadDirectory 'clouvia-desktop.exe'
    if (-not $resolvedApplication.Equals($expectedPayloadApplication, [StringComparison]::OrdinalIgnoreCase)) {
        throw "Windows payload application must be the archive root executable: $expectedPayloadApplication"
    }
}

if ($resolvedApplication) {
    Assert-PEArchitecture $resolvedApplication $Architecture
    Assert-PEVersionInfo $resolvedApplication $ExpectedVersion $ExpectedProductVersionString `
        $ExpectedProductName $ExpectedFileDescription $ExpectedCompanyName
    Assert-Authenticode $resolvedApplication 'application'
    if ($resolvedPayloadDirectory) {
        Assert-ManagedPythonRuntime $resolvedPayloadDirectory $Architecture $PythonRuntimeLockPath
    }
    Write-Host "Verified Windows application architecture: $Architecture ($resolvedApplication)"
}

if ($resolvedInstaller) {
    Assert-Authenticode $resolvedInstaller 'installer'
    $sevenZip = Resolve-SevenZip
    $extractDirectory = Join-Path ([System.IO.Path]::GetTempPath()) ("kwu-installer-architecture-" + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $extractDirectory | Out-Null
    try {
        $global:LASTEXITCODE = 0
        $extractOutput = & $sevenZip x -y "-o$extractDirectory" $resolvedInstaller 2>&1
        if ($LASTEXITCODE -ne 0) {
            throw "7-Zip could not extract the NSIS installer (exit $LASTEXITCODE): $($extractOutput | Out-String)"
        }
        $payloads = @(Get-ChildItem -LiteralPath $extractDirectory -Recurse -File |
            Where-Object { $_.Name -ieq 'clouvia-desktop.exe' })
        if ($payloads.Count -ne 1) {
            throw "NSIS installer must contain exactly one clouvia-desktop.exe payload; found $($payloads.Count): $resolvedInstaller"
        }
        Assert-PEArchitecture $payloads[0].FullName $Architecture
        Assert-PEVersionInfo $payloads[0].FullName $ExpectedVersion $ExpectedProductVersionString `
            $ExpectedProductName $ExpectedFileDescription $ExpectedCompanyName
        Assert-Authenticode $payloads[0].FullName 'installer application payload'
        Assert-ManagedPythonRuntime $extractDirectory $Architecture $PythonRuntimeLockPath
        Write-Host "Verified NSIS application payload architecture: $Architecture ($resolvedInstaller)"
    }
    finally {
        Remove-Item -LiteralPath $extractDirectory -Recurse -Force -ErrorAction SilentlyContinue
    }
}

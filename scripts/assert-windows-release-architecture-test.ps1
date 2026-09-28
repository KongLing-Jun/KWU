$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$verifier = Join-Path $PSScriptRoot 'assert-windows-release-architecture.ps1'
$parseErrors = $null
$null = [System.Management.Automation.Language.Parser]::ParseFile($verifier, [ref]$null, [ref]$parseErrors)
if ($parseErrors.Count -ne 0) {
    throw "architecture verifier has parse errors: $($parseErrors | Out-String)"
}

$testRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("kwu-architecture-test-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $testRoot | Out-Null
try {
    function New-TestPE([string]$Path, [int]$Machine) {
        [byte[]]$bytes = New-Object byte[] 512
        [System.BitConverter]::GetBytes([uint16]0x5A4D).CopyTo($bytes, 0)
        [System.BitConverter]::GetBytes([int32]128).CopyTo($bytes, 0x3c)
        [System.BitConverter]::GetBytes([uint32]0x00004550).CopyTo($bytes, 128)
        [System.BitConverter]::GetBytes([uint16]$Machine).CopyTo($bytes, 132)
        [System.IO.File]::WriteAllBytes($Path, $bytes)
    }

    function Get-TestPayloadReceipt([string]$Path) {
        $files = [System.Collections.Generic.List[object]]::new()
        $canonical = [System.Text.StringBuilder]::new()
        $paths = [string[]]@(Get-ChildItem -LiteralPath $Path -Force -File |
            Where-Object { $_.Name -cne '.kwu-runtime.json' } |
            ForEach-Object { $_.Name })
        [System.Array]::Sort($paths, [System.StringComparer]::Ordinal)
        foreach ($relative in $paths) {
            $file = Get-Item -LiteralPath (Join-Path $Path $relative)
            $entry = [pscustomobject][ordered]@{
                path = $relative
                size = [int64]$file.Length
                sha256 = (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
            }
            $files.Add($entry)
            [void]$canonical.Append($entry.sha256)
            [void]$canonical.Append('  ')
            [void]$canonical.Append($entry.size.ToString([System.Globalization.CultureInfo]::InvariantCulture))
            [void]$canonical.Append('  ')
            [void]$canonical.Append($entry.path)
            [void]$canonical.Append([char]10)
        }
        $bytes = [System.Text.UTF8Encoding]::new($false).GetBytes($canonical.ToString())
        $hash = [System.Convert]::ToHexString(
            [System.Security.Cryptography.SHA256]::HashData($bytes)
        ).ToLowerInvariant()
        return [pscustomobject]@{ files = $files.ToArray(); payload_sha256 = $hash }
    }

    function New-TestRuntimeFixture {
        param(
            [string]$Path,
            [string]$Architecture,
            [string]$PythonPath,
            [string]$SourceSHA256,
            [int64]$SourceSize
        )
        New-Item -ItemType Directory -Path $Path -Force | Out-Null
        Copy-Item -LiteralPath $PythonPath -Destination (Join-Path $Path 'python.exe')
        [System.IO.File]::WriteAllText(
            (Join-Path $Path 'LICENSE.txt'), 'license fixture', [System.Text.UTF8Encoding]::new($false)
        )
        [System.IO.File]::WriteAllBytes((Join-Path $Path 'python312.dll'), [byte[]](1, 2, 3, 4))
        [System.IO.File]::WriteAllText(
            (Join-Path $Path 'python312._pth.kwu-disabled'), 'python312.zip', [System.Text.UTF8Encoding]::new($false)
        )
        $receipt = Get-TestPayloadReceipt $Path
        $manifest = [ordered]@{
            schema = 1
            version = '3.12.10'
            arch = $Architecture
            source_sha256 = $SourceSHA256
            source_size = $SourceSize
            payload_sha256 = $receipt.payload_sha256
            files = @($receipt.files)
        } | ConvertTo-Json -Depth 4 -Compress
        [System.IO.File]::WriteAllText(
            (Join-Path $Path '.kwu-runtime.json'), $manifest, [System.Text.UTF8Encoding]::new($false)
        )
        return [pscustomobject]@{
            path = $Path
            source_sha256 = $SourceSHA256
            source_size = $SourceSize
            payload_files = @($receipt.files).Count
            payload_sha256 = $receipt.payload_sha256
        }
    }

    function Invoke-Verifier {
        param([string[]]$Arguments, [bool]$ShouldSucceed)
        $shell = (Get-Process -Id $PID).Path
        $previousErrorAction = $ErrorActionPreference
        try {
            # Windows PowerShell 5.1 promotes redirected native stderr to an
            # ErrorRecord when ErrorActionPreference is Stop. Negative contract
            # cases must inspect the child exit code instead of terminating here.
            $ErrorActionPreference = 'Continue'
            $output = & $shell -NoLogo -NoProfile -File $verifier @Arguments 2>&1
        }
        finally {
            $ErrorActionPreference = $previousErrorAction
        }
        $succeeded = $LASTEXITCODE -eq 0
        if ($succeeded -ne $ShouldSucceed) {
            throw "verifier success=$succeeded, expected $ShouldSucceed; output: $($output | Out-String)"
        }
        return ($output | Out-String)
    }

    $amd64 = Join-Path $testRoot 'amd64.exe'
    $arm64 = Join-Path $testRoot 'arm64.exe'
    $x86InstallerShell = Join-Path $testRoot 'installer.exe'
    New-TestPE $amd64 0x8664
    New-TestPE $arm64 0xAA64
    New-TestPE $x86InstallerShell 0x014c

    $amd64Runtime = New-TestRuntimeFixture `
        (Join-Path $testRoot 'runtime-amd64') 'amd64' $amd64 ('a' * 64) 101
    $arm64Runtime = New-TestRuntimeFixture `
        (Join-Path $testRoot 'runtime-arm64') 'arm64' $arm64 ('b' * 64) 202
    $syntheticRuntimeLock = Join-Path $testRoot 'python-runtime.synthetic.lock.json'
    $syntheticLock = [ordered]@{
        schema = 1
        version = '3.12.10'
        artifacts = [ordered]@{
            amd64 = [ordered]@{
                url = 'https://invalid.example/python-amd64.zip'
                bytes = $amd64Runtime.source_size
                sha256 = $amd64Runtime.source_sha256
                payload_files = $amd64Runtime.payload_files
                payload_sha256 = $amd64Runtime.payload_sha256
            }
            arm64 = [ordered]@{
                url = 'https://invalid.example/python-arm64.zip'
                bytes = $arm64Runtime.source_size
                sha256 = $arm64Runtime.source_sha256
                payload_files = $arm64Runtime.payload_files
                payload_sha256 = $arm64Runtime.payload_sha256
            }
        }
    } | ConvertTo-Json -Depth 5
    [System.IO.File]::WriteAllText(
        $syntheticRuntimeLock, $syntheticLock, [System.Text.UTF8Encoding]::new($false)
    )

    $portableRoot = Join-Path $testRoot 'portable-arm64'
    $portableApplication = Join-Path $portableRoot 'clouvia-desktop.exe'
    $portableRuntime = Join-Path $portableRoot 'runtimes\python\3.12.10\arm64'
    New-Item -ItemType Directory -Path $portableRuntime -Force | Out-Null
    Copy-Item -LiteralPath $arm64 -Destination $portableApplication
    Get-ChildItem -LiteralPath $arm64Runtime.path -Force |
        Copy-Item -Destination $portableRuntime -Recurse -Force
    Invoke-Verifier @(
        '-Architecture', 'arm64', '-ApplicationPath', $portableApplication,
        '-PayloadDirectory', $portableRoot, '-PythonRuntimeLockPath', $syntheticRuntimeLock
    ) $true | Out-Null
    $portableExtraSubtree = Join-Path $portableRoot 'runtimes\python\unexpected\amd64'
    New-Item -ItemType Directory -Path $portableExtraSubtree -Force | Out-Null
    [System.IO.File]::WriteAllText((Join-Path $portableExtraSubtree 'rogue.txt'), 'fixture')
    Invoke-Verifier @(
        '-Architecture', 'arm64', '-ApplicationPath', $portableApplication,
        '-PayloadDirectory', $portableRoot, '-PythonRuntimeLockPath', $syntheticRuntimeLock
    ) $false | Out-Null
    Remove-Item -LiteralPath (Join-Path $portableRoot 'runtimes\python\unexpected') -Recurse -Force

    Invoke-Verifier @('-Architecture', 'amd64', '-ApplicationPath', $amd64) $true | Out-Null
    Invoke-Verifier @('-Architecture', 'arm64', '-ApplicationPath', $arm64) $true | Out-Null
    Invoke-Verifier @('-Architecture', 'arm64', '-ApplicationPath', $amd64) $false | Out-Null

    $systemApplication = Join-Path $env:SystemRoot 'System32\cmd.exe'
    $systemArchitecture = if (
        [System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture -eq
        [System.Runtime.InteropServices.Architecture]::Arm64
    ) { 'arm64' } else { 'amd64' }
    $systemVersionInfo = [System.Diagnostics.FileVersionInfo]::GetVersionInfo($systemApplication)
    $systemOutput = Invoke-Verifier @(
        '-Architecture', $systemArchitecture,
        '-ApplicationPath', $systemApplication,
        '-ExpectedVersion', $systemVersionInfo.FileVersionRaw.ToString(),
        '-ExpectedProductVersionString', $systemVersionInfo.ProductVersion,
        '-ExpectedProductName', $systemVersionInfo.ProductName,
        '-ExpectedFileDescription', $systemVersionInfo.FileDescription,
        '-ExpectedCompanyName', $systemVersionInfo.CompanyName
    ) $true
    if (-not $systemOutput.Contains('Verified Windows VersionInfo:')) {
        throw "VersionInfo success marker missing: $systemOutput"
    }
    Invoke-Verifier @(
        '-Architecture', $systemArchitecture,
        '-ApplicationPath', $systemApplication,
        '-ExpectedVersion', $systemVersionInfo.FileVersionRaw.ToString(),
        '-ExpectedProductVersionString', $systemVersionInfo.ProductVersion,
        '-ExpectedProductName', 'wrong product',
        '-ExpectedFileDescription', $systemVersionInfo.FileDescription,
        '-ExpectedCompanyName', $systemVersionInfo.CompanyName
    ) $false | Out-Null

    $fakeSevenZip = Join-Path $testRoot 'fake-7z.ps1'
    @'
param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments)
$outputArgument = $Arguments | Where-Object { $_ -like '-o*' } | Select-Object -First 1
if (-not $outputArgument) { throw 'missing output directory argument' }
$outputDirectory = $outputArgument.Substring(2)
New-Item -ItemType Directory -Path $outputDirectory -Force | Out-Null
if ($env:KWU_TEST_PAYLOAD_MODE -eq 'missing') { exit 0 }
$payloadDirectory = Join-Path $outputDirectory 'payload'
New-Item -ItemType Directory -Path $payloadDirectory -Force | Out-Null
Copy-Item -LiteralPath $env:KWU_TEST_PAYLOAD -Destination (Join-Path $payloadDirectory 'clouvia-desktop.exe')
if ($env:KWU_TEST_PAYLOAD_MODE -eq 'duplicate') {
    $secondDirectory = Join-Path $outputDirectory 'second'
    New-Item -ItemType Directory -Path $secondDirectory -Force | Out-Null
    Copy-Item -LiteralPath $env:KWU_TEST_PAYLOAD -Destination (Join-Path $secondDirectory 'CLOUVIA-DESKTOP.EXE')
}
$runtimeArchitecture = $env:KWU_TEST_RUNTIME_ARCH
$runtimeDirectory = Join-Path $outputDirectory "payload\runtimes\python\3.12.10\.$runtimeArchitecture.staging"
New-Item -ItemType Directory -Path $runtimeDirectory -Force | Out-Null
Get-ChildItem -LiteralPath $env:KWU_TEST_RUNTIME_FIXTURE -Force |
    Copy-Item -Destination $runtimeDirectory -Recurse -Force
switch ($env:KWU_TEST_PAYLOAD_MODE) {
    'missing-python-license' {
        Remove-Item -LiteralPath (Join-Path $runtimeDirectory 'LICENSE.txt') -Force
    }
    'bad-python-manifest' {
        $manifestPath = Join-Path $runtimeDirectory '.kwu-runtime.json'
        $manifest = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
        $manifest.source_size = 1
        [System.IO.File]::WriteAllText(
            $manifestPath, ($manifest | ConvertTo-Json -Depth 4 -Compress),
            [System.Text.UTF8Encoding]::new($false)
        )
    }
    'tamper-python-payload' {
        [System.IO.File]::AppendAllText((Join-Path $runtimeDirectory 'python312.dll'), 'tampered')
    }
    'extra-python-file' {
        [System.IO.File]::WriteAllText((Join-Path $runtimeDirectory 'sitecustomize.py'), 'raise SystemExit')
    }
    'missing-python-file' {
        Remove-Item -LiteralPath (Join-Path $runtimeDirectory 'python312._pth.kwu-disabled') -Force
    }
    'extra-python-directory' {
        $extraDirectory = Join-Path $runtimeDirectory 'Lib'
        New-Item -ItemType Directory -Path $extraDirectory | Out-Null
        [System.IO.File]::WriteAllText((Join-Path $extraDirectory 'site.py'), 'fixture')
    }
    'extra-python-subtree' {
        $extraDirectory = Join-Path $outputDirectory 'payload\runtimes\python\unexpected\amd64'
        New-Item -ItemType Directory -Path $extraDirectory -Force | Out-Null
        [System.IO.File]::WriteAllText((Join-Path $extraDirectory 'rogue.txt'), 'fixture')
    }
    'python-reparse' {
        $targetDirectory = Join-Path $outputDirectory 'reparse-target'
        New-Item -ItemType Directory -Path $targetDirectory | Out-Null
        New-Item -ItemType Junction -Path (Join-Path $runtimeDirectory 'linked') -Target $targetDirectory | Out-Null
    }
}
if ($env:KWU_TEST_PAYLOAD_MODE -eq 'duplicate-python') {
    $secondRuntime = Join-Path $outputDirectory 'second\runtimes\python\3.12.10\.amd64.staging'
    New-Item -ItemType Directory -Path $secondRuntime -Force | Out-Null
    Copy-Item -LiteralPath (Join-Path $env:KWU_TEST_RUNTIME_FIXTURE 'python.exe') `
        -Destination (Join-Path $secondRuntime 'python.exe')
}
'@ | Set-Content -LiteralPath $fakeSevenZip -Encoding UTF8

    $env:KWU_TEST_PAYLOAD = $arm64
    $env:KWU_TEST_RUNTIME_FIXTURE = $arm64Runtime.path
    $env:KWU_TEST_RUNTIME_ARCH = 'arm64'
    $env:KWU_TEST_PAYLOAD_MODE = 'normal'
    $installerOutput = Invoke-Verifier @(
        '-Architecture', 'arm64', '-InstallerPath', $x86InstallerShell, '-SevenZipPath', $fakeSevenZip,
        '-PythonRuntimeLockPath', $syntheticRuntimeLock
    ) $true
    if (-not $installerOutput.Contains('Verified NSIS application payload architecture: arm64')) {
        throw "installer payload success marker missing: $installerOutput"
    }
    if (-not $installerOutput.Contains('Verified managed Python 3.12.10 runtime: arm64')) {
        throw "managed Python runtime success marker missing: $installerOutput"
    }

    $env:KWU_TEST_PAYLOAD = $amd64
    Invoke-Verifier @(
        '-Architecture', 'arm64', '-InstallerPath', $x86InstallerShell, '-SevenZipPath', $fakeSevenZip,
        '-PythonRuntimeLockPath', $syntheticRuntimeLock
    ) $false | Out-Null

    $env:KWU_TEST_PAYLOAD_MODE = 'missing'
    Invoke-Verifier @(
        '-Architecture', 'arm64', '-InstallerPath', $x86InstallerShell, '-SevenZipPath', $fakeSevenZip,
        '-PythonRuntimeLockPath', $syntheticRuntimeLock
    ) $false | Out-Null

    $env:KWU_TEST_PAYLOAD = $arm64
    $env:KWU_TEST_PAYLOAD_MODE = 'duplicate'
    Invoke-Verifier @(
        '-Architecture', 'arm64', '-InstallerPath', $x86InstallerShell, '-SevenZipPath', $fakeSevenZip,
        '-PythonRuntimeLockPath', $syntheticRuntimeLock
    ) $false | Out-Null

    foreach ($invalidRuntimeMode in @(
        'missing-python-license',
        'bad-python-manifest',
        'duplicate-python',
        'tamper-python-payload',
        'extra-python-file',
        'missing-python-file',
        'extra-python-directory',
        'extra-python-subtree',
        'python-reparse'
    )) {
        $env:KWU_TEST_PAYLOAD_MODE = $invalidRuntimeMode
        Invoke-Verifier @(
            '-Architecture', 'arm64', '-InstallerPath', $x86InstallerShell, '-SevenZipPath', $fakeSevenZip,
            '-PythonRuntimeLockPath', $syntheticRuntimeLock
        ) $false | Out-Null
    }
    $env:KWU_TEST_PAYLOAD_MODE = 'normal'
    $env:KWU_TEST_RUNTIME_FIXTURE = $amd64Runtime.path
    Invoke-Verifier @(
        '-Architecture', 'arm64', '-InstallerPath', $x86InstallerShell, '-SevenZipPath', $fakeSevenZip,
        '-PythonRuntimeLockPath', $syntheticRuntimeLock
    ) $false | Out-Null
    $env:KWU_TEST_RUNTIME_FIXTURE = $arm64Runtime.path

    # Omitting the explicit test seam must use the repository's production lock,
    # which cannot accept this synthetic fixture.
    Invoke-Verifier @(
        '-Architecture', 'arm64', '-InstallerPath', $x86InstallerShell, '-SevenZipPath', $fakeSevenZip
    ) $false | Out-Null

    $fakeSignTool = Join-Path $testRoot 'fake-signtool.cmd'
    [System.IO.File]::WriteAllText(
        $fakeSignTool,
        "@echo Successfully verified`r`n@exit /b 0`r`n",
        [System.Text.Encoding]::ASCII
    )
    $systemSignature = Get-AuthenticodeSignature -LiteralPath $systemApplication
    if ($systemSignature.Status -ne [System.Management.Automation.SignatureStatus]::Valid -or
        -not $systemSignature.TimeStamperCertificate) {
        throw 'System Authenticode fixture is not valid and timestamped'
    }
    $env:KWU_TEST_PAYLOAD = $systemApplication
    $env:KWU_TEST_RUNTIME_ARCH = $systemArchitecture
    $env:KWU_TEST_RUNTIME_FIXTURE = if ($systemArchitecture -eq 'arm64') {
        $arm64Runtime.path
    } else {
        $amd64Runtime.path
    }
    $env:KWU_TEST_PAYLOAD_MODE = 'normal'
    $signedOutput = Invoke-Verifier @(
        '-Architecture', $systemArchitecture,
        '-InstallerPath', $systemApplication,
        '-SevenZipPath', $fakeSevenZip,
        '-PythonRuntimeLockPath', $syntheticRuntimeLock,
        '-RequireSignature',
        '-ExpectedSignerThumbprint', $systemSignature.SignerCertificate.Thumbprint,
        '-ExpectedSignerSubject', $systemSignature.SignerCertificate.Subject,
        '-SignToolPath', $fakeSignTool
    ) $true
    if (-not $signedOutput.Contains('Verified Windows Authenticode for installer application payload')) {
        throw "installer payload signature success marker missing: $signedOutput"
    }
    $trustedOutput = Invoke-Verifier @(
        '-Architecture', $systemArchitecture,
        '-ApplicationPath', $systemApplication,
        '-RequireTrustedSignature',
        '-SignToolPath', $fakeSignTool
    ) $true
    if (-not $trustedOutput.Contains('Verified Windows Authenticode for application')) {
        throw "trusted signature success marker missing: $trustedOutput"
    }
    Invoke-Verifier @(
        '-Architecture', $systemArchitecture,
        '-ApplicationPath', $systemApplication,
        '-RequireSignature',
        '-ExpectedSignerThumbprint', ('0' * 40),
        '-ExpectedSignerSubject', $systemSignature.SignerCertificate.Subject,
        '-SignToolPath', $fakeSignTool
    ) $false | Out-Null
    Invoke-Verifier @(
        '-Architecture', 'amd64',
        '-ApplicationPath', $amd64,
        '-RequireSignature',
        '-ExpectedSignerThumbprint', $systemSignature.SignerCertificate.Thumbprint,
        '-ExpectedSignerSubject', $systemSignature.SignerCertificate.Subject,
        '-SignToolPath', $fakeSignTool
    ) $false | Out-Null

    Write-Host 'Windows architecture, VersionInfo, NSIS payload and Authenticode tests passed'
}
finally {
    Remove-Item Env:KWU_TEST_PAYLOAD -ErrorAction SilentlyContinue
    Remove-Item Env:KWU_TEST_PAYLOAD_MODE -ErrorAction SilentlyContinue
    Remove-Item Env:KWU_TEST_RUNTIME_FIXTURE -ErrorAction SilentlyContinue
    Remove-Item Env:KWU_TEST_RUNTIME_ARCH -ErrorAction SilentlyContinue
    if (Test-Path -LiteralPath $testRoot) {
        Remove-Item -LiteralPath $testRoot -Recurse -Force
    }
}

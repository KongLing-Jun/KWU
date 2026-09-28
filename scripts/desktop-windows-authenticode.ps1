[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [ValidateSet('Preflight', 'Sign', 'Verify', 'AssertEvidence')]
    [string]$Action,
    [string]$Path = '',
    [ValidateSet('', 'application', 'uninstaller', 'installer')]
    [string]$Layer = '',
    [ValidateSet('', 'amd64', 'arm64')]
    [string]$Architecture = '',
    [string]$EvidencePath = '',
    [string]$ExpectedThumbprint = '',
    [string]$ExpectedSubject = '',
    [string]$SignToolPath = '',
    [ValidateRange(1, 5)]
    [int]$TimestampAttempts = 3
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# NSIS starts PowerShell 7 in a child process. Load the security module
# explicitly so Authenticode verification never depends on module auto-loading.
Import-Module Microsoft.PowerShell.Security -ErrorAction Stop

$script:TimestampURL = 'http://time.certum.pl/'
$script:CodeSigningEku = '1.3.6.1.5.5.7.3.3'

function Normalize-CertificateThumbprint {
    param([string]$Value)
    $normalized = [regex]::Replace(([string]$Value).ToUpperInvariant(), '[^0-9A-F]', '')
    if ($normalized -notmatch '^[0-9A-F]{40}$') {
        throw 'Windows signing certificate thumbprint must contain exactly 40 hexadecimal characters'
    }
    return $normalized
}

function Resolve-RequiredFile {
    param([string]$Value, [string]$Description)
    if ([string]::IsNullOrWhiteSpace($Value)) { throw "$Description is required" }
    if (-not (Test-Path -LiteralPath $Value -PathType Leaf)) {
        throw "$Description does not exist: $Value"
    }
    return (Resolve-Path -LiteralPath $Value).Path
}

function Get-CodeSigningEkuOids {
    param($Certificate)
    $oids = @()
    if ($Certificate.PSObject.Properties.Name -contains 'EnhancedKeyUsageList') {
        foreach ($usage in @($Certificate.EnhancedKeyUsageList)) {
            if ($usage.PSObject.Properties.Name -contains 'ObjectId') {
                $oids += [string]$usage.ObjectId
            }
            elseif ($usage.PSObject.Properties.Name -contains 'Oid') {
                $oids += [string]$usage.Oid.Value
            }
        }
    }
    if ($oids.Count -eq 0 -and $Certificate.PSObject.Properties.Name -contains 'Extensions') {
        foreach ($extension in @($Certificate.Extensions)) {
            if ([string]$extension.Oid.Value -ne '2.5.29.37') { continue }
            $eku = [System.Security.Cryptography.X509Certificates.X509EnhancedKeyUsageExtension]::new(
                $extension,
                $extension.Critical
            )
            $oids += @($eku.EnhancedKeyUsages | ForEach-Object { [string]$_.Value })
        }
    }
    return @($oids)
}

function Select-SigningCertificate {
    param(
        [object[]]$Certificates,
        [string]$Thumbprint,
        [string]$Subject,
        [datetime]$Now = [datetime]::Now
    )
    if ([string]::IsNullOrWhiteSpace($Subject)) {
        throw 'WINDOWS_SIGN_CERT_SUBJECT is required'
    }
    $normalized = Normalize-CertificateThumbprint $Thumbprint
    $matches = @($Certificates | Where-Object {
        try { (Normalize-CertificateThumbprint ([string]$_.Thumbprint)) -eq $normalized }
        catch { $false }
    })
    if ($matches.Count -ne 1) {
        throw "Expected exactly one CurrentUser/My certificate with thumbprint $normalized; found $($matches.Count)"
    }
    $certificate = $matches[0]
    if ([string]$certificate.Subject -cne $Subject) {
        throw "Windows signing certificate subject mismatch: expected '$Subject', found '$($certificate.Subject)'"
    }
    if (-not [bool]$certificate.HasPrivateKey) {
        throw 'Windows signing certificate does not expose a private key; verify the SimplySign session'
    }
    if ($Now -lt [datetime]$certificate.NotBefore -or $Now -gt [datetime]$certificate.NotAfter) {
        throw 'Windows signing certificate is not currently valid'
    }
    if ((Get-CodeSigningEkuOids $certificate) -notcontains $script:CodeSigningEku) {
        throw 'Windows signing certificate does not include the Code Signing EKU'
    }
    return $certificate
}

function Resolve-SigningIdentity {
    param([switch]$RequirePrivateKey)
    $thumbprintValue = if ($ExpectedThumbprint) { $ExpectedThumbprint } else { $env:WINDOWS_SIGN_CERT_THUMBPRINT }
    $subjectValue = if ($ExpectedSubject) { $ExpectedSubject } else { $env:WINDOWS_SIGN_CERT_SUBJECT }
    $toolValue = if ($SignToolPath) { $SignToolPath } else { $env:WINDOWS_SIGNTOOL_PATH }
    $resolvedTool = Resolve-RequiredFile $toolValue 'WINDOWS_SIGNTOOL_PATH'
    $normalized = Normalize-CertificateThumbprint $thumbprintValue
    if ([string]::IsNullOrWhiteSpace($subjectValue)) { throw 'WINDOWS_SIGN_CERT_SUBJECT is required' }
    $certificate = $null
    if ($RequirePrivateKey) {
        $store = [System.Security.Cryptography.X509Certificates.X509Store]::new(
            [System.Security.Cryptography.X509Certificates.StoreName]::My,
            [System.Security.Cryptography.X509Certificates.StoreLocation]::CurrentUser
        )
        try {
            $store.Open([System.Security.Cryptography.X509Certificates.OpenFlags]::ReadOnly)
            $certificate = Select-SigningCertificate @($store.Certificates) $normalized $subjectValue
        }
        finally {
            $store.Close()
        }
    }
    return [pscustomobject]@{
        Thumbprint = $normalized
        Subject = $subjectValue
        SignToolPath = $resolvedTool
        Certificate = $certificate
        SignToolVersion = [System.Diagnostics.FileVersionInfo]::GetVersionInfo($resolvedTool).FileVersion
    }
}

function Invoke-SignTool {
    param([string]$ToolPath, [string[]]$Arguments)
    $previousErrorAction = $ErrorActionPreference
    try {
        $ErrorActionPreference = 'Continue'
        $output = & $ToolPath @Arguments 2>&1
        $exitCode = $LASTEXITCODE
    }
    finally {
        $ErrorActionPreference = $previousErrorAction
    }
    return [pscustomobject]@{
        ExitCode = $exitCode
        Output = ($output | Out-String).Trim()
    }
}

function Get-SignArguments {
    param([string]$Thumbprint, [string]$FilePath)
    return @(
        'sign', '/s', 'My', '/sha1', $Thumbprint,
        '/fd', 'sha256', '/tr', $script:TimestampURL, '/td', 'sha256',
        '/v', $FilePath
    )
}

function Test-TransientTimestampFailure {
    param([string]$Output)
    return $Output -match '(?i)timestamp|time stamp|0x80072|0x800b010a'
}

function Get-VerifiedSignature {
    param([string]$FilePath, $Identity)
    $verify = Invoke-SignTool $Identity.SignToolPath @('verify', '/pa', '/all', '/v', $FilePath)
    if ($verify.ExitCode -ne 0) {
        throw "SignTool verification failed for $(Split-Path $FilePath -Leaf): $($verify.Output)"
    }
    $signature = Get-AuthenticodeSignature -LiteralPath $FilePath
    if ($signature.Status -ne [System.Management.Automation.SignatureStatus]::Valid) {
        throw "Authenticode status is $($signature.Status) for $(Split-Path $FilePath -Leaf)"
    }
    if (-not $signature.SignerCertificate) {
        throw "Authenticode signer certificate is missing for $(Split-Path $FilePath -Leaf)"
    }
    $actualThumbprint = Normalize-CertificateThumbprint $signature.SignerCertificate.Thumbprint
    if ($actualThumbprint -ne $Identity.Thumbprint -or
        [string]$signature.SignerCertificate.Subject -cne $Identity.Subject) {
        throw "Authenticode signer identity does not match the configured certificate for $(Split-Path $FilePath -Leaf)"
    }
    if (-not $signature.TimeStamperCertificate) {
        throw "RFC 3161 timestamp certificate is missing for $(Split-Path $FilePath -Leaf)"
    }
    return [pscustomobject]@{
        Signature = $signature
        SHA256 = (Get-FileHash -LiteralPath $FilePath -Algorithm SHA256).Hash.ToLowerInvariant()
        Size = (Get-Item -LiteralPath $FilePath).Length
    }
}

function Write-SigningEvidence {
    param([string]$OutputPath, [string]$FilePath, [string]$ItemLayer, [string]$ItemArchitecture, $Identity, $Verified)
    if ([string]::IsNullOrWhiteSpace($OutputPath)) { throw 'EvidencePath is required for Sign' }
    $parent = Split-Path -Parent $OutputPath
    if ($parent) { New-Item -ItemType Directory -Path $parent -Force | Out-Null }
    $records = @()
    if (Test-Path -LiteralPath $OutputPath -PathType Leaf) {
        $records = @((Get-Content -LiteralPath $OutputPath -Raw | ConvertFrom-Json))
    }
    if (@($records | Where-Object {
        $_.architecture -eq $ItemArchitecture -and $_.layer -eq $ItemLayer
    }).Count -ne 0) {
        throw "Duplicate signing evidence for $ItemArchitecture/$ItemLayer"
    }
    $timestamp = $Verified.Signature.TimeStamperCertificate
    $records += [pscustomobject]@{
        schema_version = 1
        architecture = $ItemArchitecture
        layer = $ItemLayer
        file_name = Split-Path $FilePath -Leaf
        sha256 = $Verified.SHA256
        size_bytes = $Verified.Size
        signer_subject = $Identity.Subject
        signer_thumbprint = $Identity.Thumbprint
        timestamp_subject = [string]$timestamp.Subject
        timestamp_not_before = ([datetime]$timestamp.NotBefore).ToUniversalTime().ToString('o')
        timestamp_not_after = ([datetime]$timestamp.NotAfter).ToUniversalTime().ToString('o')
        timestamp_url = $script:TimestampURL
        signtool_version = [string]$Identity.SignToolVersion
    }
    $temporary = "$OutputPath.tmp-$([guid]::NewGuid().ToString('N'))"
    try {
        [System.IO.File]::WriteAllText(
            $temporary,
            (($records | ConvertTo-Json -Depth 5) + [Environment]::NewLine),
            [System.Text.UTF8Encoding]::new($false)
        )
        Move-Item -LiteralPath $temporary -Destination $OutputPath -Force
    }
    finally {
        Remove-Item -LiteralPath $temporary -Force -ErrorAction SilentlyContinue
    }
}

function Assert-SigningEvidence {
    param([string]$InputPath, [string]$ExpectedArchitectureValue, [string]$InstallerFile)
    $resolvedEvidence = Resolve-RequiredFile $InputPath 'EvidencePath'
    $resolvedInstaller = Resolve-RequiredFile $InstallerFile 'InstallerPath'
    $records = @((Get-Content -LiteralPath $resolvedEvidence -Raw | ConvertFrom-Json))
    $expectedLayers = @('application', 'uninstaller', 'installer')
    if ($records.Count -ne 3 -or @($records.layer | Sort-Object -Unique).Count -ne 3) {
        throw "Signing evidence must contain exactly application, uninstaller and installer records"
    }
    foreach ($layerName in $expectedLayers) {
        $matches = @($records | Where-Object {
            $_.architecture -eq $ExpectedArchitectureValue -and $_.layer -eq $layerName
        })
        if ($matches.Count -ne 1) { throw "Signing evidence is missing unique $ExpectedArchitectureValue/$layerName" }
    }
    $installerRecord = $records | Where-Object { $_.layer -eq 'installer' }
    $actualHash = (Get-FileHash -LiteralPath $resolvedInstaller -Algorithm SHA256).Hash.ToLowerInvariant()
    if ([string]$installerRecord.sha256 -cne $actualHash) {
        throw 'Installer hash does not match its signing evidence'
    }
    Write-Host "Verified signing evidence: $ExpectedArchitectureValue (3 layers)"
}

function Invoke-AuthenticodeMain {
    switch ($Action) {
        'Preflight' {
            $identity = Resolve-SigningIdentity -RequirePrivateKey
            Write-Host "Windows signing preflight passed: $($identity.Subject)"
        }
        'Sign' {
            if (-not $Layer -or -not $Architecture) { throw 'Layer and Architecture are required for Sign' }
            $resolvedFile = Resolve-RequiredFile $Path 'Signing input'
            $identity = Resolve-SigningIdentity -RequirePrivateKey
            $arguments = Get-SignArguments $identity.Thumbprint $resolvedFile
            for ($attempt = 1; $attempt -le $TimestampAttempts; $attempt++) {
                $result = Invoke-SignTool $identity.SignToolPath $arguments
                if ($result.ExitCode -eq 0) { break }
                if ($attempt -ge $TimestampAttempts -or -not (Test-TransientTimestampFailure $result.Output)) {
                    throw "SignTool signing failed for $(Split-Path $resolvedFile -Leaf): $($result.Output)"
                }
                Start-Sleep -Seconds ([math]::Min(5, $attempt * 2))
            }
            $verified = Get-VerifiedSignature $resolvedFile $identity
            Write-SigningEvidence $EvidencePath $resolvedFile $Layer $Architecture $identity $verified
            Write-Host "Signed and verified Windows $Architecture ${Layer}: $(Split-Path $resolvedFile -Leaf)"
        }
        'Verify' {
            $resolvedFile = Resolve-RequiredFile $Path 'Verification input'
            $identity = Resolve-SigningIdentity
            $null = Get-VerifiedSignature $resolvedFile $identity
            Write-Host "Verified Windows Authenticode: $(Split-Path $resolvedFile -Leaf)"
        }
        'AssertEvidence' {
            if (-not $Architecture) { throw 'Architecture is required for AssertEvidence' }
            Assert-SigningEvidence $EvidencePath $Architecture $Path
        }
    }
}

if ($MyInvocation.InvocationName -ne '.') {
    Invoke-AuthenticodeMain
}

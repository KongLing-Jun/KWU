$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$helper = Join-Path $PSScriptRoot 'desktop-windows-authenticode.ps1'
$parseErrors = $null
$null = [System.Management.Automation.Language.Parser]::ParseFile($helper, [ref]$null, [ref]$parseErrors)
if ($parseErrors.Count -ne 0) {
    throw "Authenticode helper has parse errors: $($parseErrors | Out-String)"
}

. $helper -Action Preflight

function Assert-Throws {
    param([scriptblock]$Body, [string]$Expected)
    try {
        & $Body
    }
    catch {
        if ($_.Exception.Message -notlike "*$Expected*") {
            throw "Expected error containing '$Expected', received: $($_.Exception.Message)"
        }
        return
    }
    throw "Expected error containing '$Expected'"
}

$thumbprint = '0123456789abcdef0123456789abcdef01234567'
$normalized = Normalize-CertificateThumbprint "01 23 45 67 89 ab cd ef 01 23 45 67 89 ab cd ef 01 23 45 67"
if ($normalized -cne $thumbprint.ToUpperInvariant()) { throw 'Thumbprint normalization failed' }
Assert-Throws { Normalize-CertificateThumbprint 'not-a-thumbprint' } '40 hexadecimal'

$subject = 'CN=Example Signing, O=Example Corp, C=CN'
$now = [datetime]'2026-07-28T00:00:00Z'
function New-TestCertificate {
    param(
        [string]$CertificateSubject = $subject,
        [bool]$HasPrivateKey = $true,
        [datetime]$NotBefore = $now.AddDays(-1),
        [datetime]$NotAfter = $now.AddDays(1),
        [string]$Eku = '1.3.6.1.5.5.7.3.3'
    )
    return [pscustomobject]@{
        Thumbprint = $thumbprint
        Subject = $CertificateSubject
        HasPrivateKey = $HasPrivateKey
        NotBefore = $NotBefore
        NotAfter = $NotAfter
        EnhancedKeyUsageList = @([pscustomobject]@{ ObjectId = $Eku })
    }
}

$certificate = Select-SigningCertificate @((New-TestCertificate)) $thumbprint $subject $now
if ($certificate.Subject -cne $subject) { throw 'Expected certificate was not selected' }
Assert-Throws {
    Select-SigningCertificate @((New-TestCertificate -CertificateSubject 'CN=Wrong')) $thumbprint $subject $now
} 'subject mismatch'
Assert-Throws {
    Select-SigningCertificate @((New-TestCertificate -HasPrivateKey $false)) $thumbprint $subject $now
} 'private key'
Assert-Throws {
    Select-SigningCertificate @((New-TestCertificate -NotAfter $now.AddSeconds(-1))) $thumbprint $subject $now
} 'not currently valid'
Assert-Throws {
    Select-SigningCertificate @((New-TestCertificate -Eku '1.3.6.1.5.5.7.3.1')) $thumbprint $subject $now
} 'Code Signing EKU'
Assert-Throws {
    Select-SigningCertificate @((New-TestCertificate), (New-TestCertificate)) $thumbprint $subject $now
} 'found 2'

$arguments = @(Get-SignArguments $normalized 'C:\build\app.exe')
$expectedArguments = @(
    'sign', '/s', 'My', '/sha1', $normalized,
    '/fd', 'sha256', '/tr', 'http://time.certum.pl/', '/td', 'sha256',
    '/v', 'C:\build\app.exe'
)
if (($arguments -join "`n") -cne ($expectedArguments -join "`n")) {
    throw "Unexpected SignTool arguments: $($arguments -join ' ')"
}
if (-not (Test-TransientTimestampFailure 'The specified timestamp server could not be reached')) {
    throw 'Timestamp failure classification missed a transient server error'
}
if (Test-TransientTimestampFailure 'No certificates were found that met all the given criteria') {
    throw 'Certificate selection failure must not be retried as a timestamp error'
}

$testRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("kwu-authenticode-test-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $testRoot | Out-Null
try {
    $fakeSignTool = Join-Path $testRoot 'signtool.cmd'
    [System.IO.File]::WriteAllText(
        $fakeSignTool,
        "@echo Successfully verified`r`n@exit /b 0`r`n",
        [System.Text.Encoding]::ASCII
    )
    $signedSystemFile = Join-Path $env:SystemRoot 'System32\cmd.exe'
    $systemSignature = Get-AuthenticodeSignature -LiteralPath $signedSystemFile
    if ($systemSignature.Status -ne [System.Management.Automation.SignatureStatus]::Valid -or
        -not $systemSignature.TimeStamperCertificate) {
        throw 'The Windows system Authenticode fixture is not signed and timestamped'
    }
    $identity = [pscustomobject]@{
        Thumbprint = Normalize-CertificateThumbprint $systemSignature.SignerCertificate.Thumbprint
        Subject = [string]$systemSignature.SignerCertificate.Subject
        SignToolPath = $fakeSignTool
        SignToolVersion = 'test'
    }
    $verified = Get-VerifiedSignature $signedSystemFile $identity
    if ($verified.SHA256 -notmatch '^[0-9a-f]{64}$') { throw 'Verified SHA256 was not normalized' }

    $evidence = Join-Path $testRoot 'evidence.json'
    foreach ($layerName in @('application', 'uninstaller', 'installer')) {
        Write-SigningEvidence $evidence $signedSystemFile $layerName 'amd64' $identity $verified
    }
    $records = @(Get-Content -LiteralPath $evidence -Raw | ConvertFrom-Json)
    if ($records.Count -ne 3 -or @($records.layer | Sort-Object -Unique).Count -ne 3) {
        throw 'Evidence did not preserve exactly three unique layers'
    }
    if ((Get-Content -LiteralPath $evidence -Raw).Contains($env:USERPROFILE, [StringComparison]::OrdinalIgnoreCase)) {
        throw 'Evidence leaked a local user profile path'
    }
    Assert-SigningEvidence $evidence 'amd64' $signedSystemFile
    Assert-Throws {
        Write-SigningEvidence $evidence $signedSystemFile 'installer' 'amd64' $identity $verified
    } 'Duplicate signing evidence'

    $tamperedEvidence = Join-Path $testRoot 'tampered.json'
    $records[2].sha256 = ('0' * 64)
    [System.IO.File]::WriteAllText(
        $tamperedEvidence,
        (($records | ConvertTo-Json -Depth 5) + [Environment]::NewLine),
        [System.Text.UTF8Encoding]::new($false)
    )
    Assert-Throws { Assert-SigningEvidence $tamperedEvidence 'amd64' $signedSystemFile } 'hash does not match'

    Write-Host 'Windows Authenticode identity, arguments, verification and evidence tests passed'
}
finally {
    if (Test-Path -LiteralPath $testRoot) {
        Remove-Item -LiteralPath $testRoot -Recurse -Force
    }
}

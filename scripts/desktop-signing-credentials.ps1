[CmdletBinding()]
param(
    [ValidateSet('SelfTest', 'StorePassword', 'MigrateGitHub')]
    [string]$Action = 'SelfTest',
    [string]$RepositoryRoot = '',
    [string]$PrivateKeyFile = 'D:\suiyuan-release-key\suiyuan.key',
    [string]$DPAPIPasswordFile = 'D:\suiyuan-release-key\minisign-password.dpapi',
    [string]$Repository = 'xiaoxiaotest9527-bit/KWU',
    [string]$EnvironmentName = 'desktop-release',
    [string]$Workflow = 'desktop-release-doctor.yml'
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$script:IsWindowsPlatform = if (Get-Variable IsWindows -ErrorAction SilentlyContinue) {
    [bool]$IsWindows
}
else {
    $env:OS -eq 'Windows_NT'
}

if ([string]::IsNullOrWhiteSpace($RepositoryRoot)) {
    $RepositoryRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
}

function Read-HiddenText {
    param([Parameter(Mandatory)][string]$Prompt)
    $secure = Read-Host -Prompt $Prompt -AsSecureString
    $pointer = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
    try {
        $value = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($pointer)
        if ([string]::IsNullOrWhiteSpace($value)) { throw "$Prompt cannot be empty" }
        return $value
    }
    finally {
        [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($pointer)
        $secure.Dispose()
    }
}

function Protect-CurrentUserPassword {
    param([Parameter(Mandatory)][string]$Password)
    if (-not $script:IsWindowsPlatform) { throw 'Windows DPAPI CurrentUser is only available on Windows' }
    Add-Type -AssemblyName System.Security -ErrorAction SilentlyContinue
    $plain = [Text.Encoding]::UTF8.GetBytes($Password)
    try {
        return [Security.Cryptography.ProtectedData]::Protect(
            $plain,
            $null,
            [Security.Cryptography.DataProtectionScope]::CurrentUser
        )
    }
    finally {
        [Array]::Clear($plain, 0, $plain.Length)
    }
}

function Unprotect-CurrentUserPassword {
    param([Parameter(Mandatory)][byte[]]$ProtectedBytes)
    if (-not $script:IsWindowsPlatform) { throw 'Windows DPAPI CurrentUser is only available on Windows' }
    Add-Type -AssemblyName System.Security -ErrorAction SilentlyContinue
    $plain = [Security.Cryptography.ProtectedData]::Unprotect(
        $ProtectedBytes,
        $null,
        [Security.Cryptography.DataProtectionScope]::CurrentUser
    )
    try { return [Text.Encoding]::UTF8.GetString($plain) }
    finally { [Array]::Clear($plain, 0, $plain.Length) }
}

function Get-StoredSigningPassword {
    if (-not (Test-Path -LiteralPath $DPAPIPasswordFile -PathType Leaf)) {
        return Read-HiddenText 'Current minisign password'
    }
    $bytes = [IO.File]::ReadAllBytes($DPAPIPasswordFile)
    if ($bytes.Length -eq 0) { throw "DPAPI password file is empty: $DPAPIPasswordFile" }
    return Unprotect-CurrentUserPassword $bytes
}

function Invoke-WithMinisignEnvironment {
    param(
        [Parameter(Mandatory)][string]$KeyFile,
        [Parameter(Mandatory)][string]$Password,
        [Parameter(Mandatory)][scriptblock]$Command
    )
    $oldKeyFile = $env:MINISIGN_PRIVATE_KEY_FILE
    $oldKeyText = $env:MINISIGN_PRIVATE_KEY
    $oldPassword = $env:MINISIGN_PASSWORD
    try {
        $env:MINISIGN_PRIVATE_KEY_FILE = $KeyFile
        $env:MINISIGN_PRIVATE_KEY = $null
        $env:MINISIGN_PASSWORD = $Password
        & $Command
    }
    finally {
        $env:MINISIGN_PRIVATE_KEY_FILE = $oldKeyFile
        $env:MINISIGN_PRIVATE_KEY = $oldKeyText
        $env:MINISIGN_PASSWORD = $oldPassword
    }
}

function Invoke-LocalSigningSelfTest {
    param([string]$KeyFile, [string]$Password)
    if (-not (Test-Path -LiteralPath $KeyFile -PathType Leaf)) {
        throw "Encrypted minisign private key does not exist: $KeyFile"
    }
    Invoke-WithMinisignEnvironment $KeyFile $Password {
        Push-Location (Join-Path $RepositoryRoot 'desktop')
        try {
            & go run ./cmd/sign selftest
            if ($LASTEXITCODE -ne 0) { throw "minisign self-test failed with exit code $LASTEXITCODE" }
        }
        finally { Pop-Location }
    }
}

function New-GitHubEnvironmentSecretProcessStartInfo {
    param([string]$Name)
    $start = [Diagnostics.ProcessStartInfo]::new()
    $start.FileName = 'gh'
    foreach ($argument in @('secret', 'set', $Name, '--repo', $Repository, '--env', $EnvironmentName)) {
        $null = $start.ArgumentList.Add($argument)
    }
    $start.UseShellExecute = $false
    $start.RedirectStandardInput = $true
    $start.RedirectStandardOutput = $true
    $start.RedirectStandardError = $true
    return $start
}

function Set-GitHubEnvironmentSecret {
    param([string]$Name, [string]$Value)
    $start = New-GitHubEnvironmentSecretProcessStartInfo $Name
    $process = [Diagnostics.Process]::new()
    $process.StartInfo = $start
    try {
        if (-not $process.Start()) { throw "Could not start gh to set secret $Name" }
        $process.StandardInput.Write($Value)
        $process.StandardInput.Close()
        $stdout = $process.StandardOutput.ReadToEnd()
        $stderr = $process.StandardError.ReadToEnd()
        $process.WaitForExit()
        if ($process.ExitCode -ne 0) {
            throw "gh failed to set Environment secret $Name (exit $($process.ExitCode)): $stderr$stdout"
        }
    }
    finally { $process.Dispose() }
}

function Remove-GitHubEnvironmentSecret {
    param([string]$Name)
    $deleteOutput = & gh secret delete $Name --repo $Repository --env $EnvironmentName 2>&1
    if ($LASTEXITCODE -eq 0) { return }

    $listOutput = & gh secret list --repo $Repository --env $EnvironmentName --json name 2>&1
    if ($LASTEXITCODE -eq 0) {
        try { $remaining = @(($listOutput | Out-String | ConvertFrom-Json) | Where-Object name -eq $Name) }
        catch { throw "Could not confirm whether temporary Environment secret $Name was deleted" }
        if ($remaining.Count -eq 0) { return }
    }
    throw "Could not delete temporary Environment secret $Name`: $(($deleteOutput | Out-String).Trim())"
}

function Invoke-CleanupWithRetry {
    param(
        [Parameter(Mandatory)][string]$Description,
        [Parameter(Mandatory)][scriptblock]$Command,
        [ValidateRange(1, 10)][int]$Attempts = 3,
        [ValidateRange(0, 60000)][int]$DelayMilliseconds = 1000
    )
    $lastFailure = ''
    for ($attempt = 1; $attempt -le $Attempts; $attempt++) {
        try {
            & $Command | Out-Null
            return
        }
        catch {
            $lastFailure = $_.Exception.Message
            if ($attempt -lt $Attempts -and $DelayMilliseconds -gt 0) {
                Start-Sleep -Milliseconds $DelayMilliseconds
            }
        }
    }
    throw "$Description failed after $Attempts attempts: $lastFailure"
}

function Remove-LocalMigrationDirectory {
    param(
        [Parameter(Mandatory)][string]$Path,
        [ValidateRange(0, 60000)][int]$DelayMilliseconds = 1000,
        [scriptblock]$RemoveCommand = {
            param([string]$Target)
            Remove-Item -LiteralPath $Target -Recurse -Force -ErrorAction Stop
        }
    )
    Invoke-CleanupWithRetry "delete local migration directory $Path" {
        if (-not (Test-Path -LiteralPath $Path)) { return }
        & $RemoveCommand $Path
        if (Test-Path -LiteralPath $Path) {
            throw "Local migration directory still exists: $Path"
        }
    } -DelayMilliseconds $DelayMilliseconds
}

function Find-ReencryptionRunID {
    param([string]$RequestID)
    $json = & gh run list --repo $Repository --workflow $Workflow --event workflow_dispatch --limit 30 `
        --json databaseId,displayTitle,createdAt 2>&1
    if ($LASTEXITCODE -ne 0) { throw "Could not list Doctor workflow runs: $($json | Out-String)" }
    $match = @(($json | Out-String | ConvertFrom-Json) |
        Where-Object { ([string]$_.displayTitle).Contains($RequestID) } |
        Sort-Object createdAt -Descending |
        Select-Object -First 1)
    if ($match.Count -eq 1) { return [int64]$match[0].databaseId }
    return 0L
}

function Get-ReencryptionRunID {
    param([string]$RequestID)
    $deadline = [DateTimeOffset]::UtcNow.AddMinutes(2)
    do {
        $runID = Find-ReencryptionRunID $RequestID
        if ($runID -gt 0) { return $runID }
        Start-Sleep -Seconds 3
    } while ([DateTimeOffset]::UtcNow -lt $deadline)
    throw "Could not find the Doctor re-encryption run for request $RequestID"
}

function Assert-ReencryptionRunCompleted {
    param([int64]$RunID)
    $json = & gh run view $RunID --repo $Repository --json status 2>&1
    if ($LASTEXITCODE -ne 0) { throw "Could not inspect re-encryption run $RunID`: $($json | Out-String)" }
    $status = [string](($json | Out-String | ConvertFrom-Json).status)
    if ($status -ne 'completed') {
        $watchOutput = & gh run watch $RunID --repo $Repository 2>&1
        if ($LASTEXITCODE -ne 0) { throw "Could not wait for re-encryption run $RunID`: $($watchOutput | Out-String)" }
        $json = & gh run view $RunID --repo $Repository --json status 2>&1
        if ($LASTEXITCODE -ne 0 -or [string](($json | Out-String | ConvertFrom-Json).status) -ne 'completed') {
            throw "Re-encryption run $RunID is not confirmed complete"
        }
    }
}

function Remove-ReencryptionArtifact {
    param([int64]$RunID, [string]$ArtifactName)
    $json = & gh api "repos/$Repository/actions/runs/$RunID/artifacts" 2>&1
    if ($LASTEXITCODE -ne 0) { throw "Could not list re-encryption artifacts: $($json | Out-String)" }
    $artifacts = @(($json | Out-String | ConvertFrom-Json).artifacts | Where-Object name -eq $ArtifactName)
    foreach ($artifact in $artifacts) {
        $null = & gh api --method DELETE "repos/$Repository/actions/artifacts/$($artifact.id)" 2>&1
        if ($LASTEXITCODE -ne 0) { throw "Could not delete temporary artifact $ArtifactName" }
    }
}

function New-MigrationCleanupFailure {
    param(
        [Parameter(Mandatory)][string]$Message,
        [Parameter(Mandatory)][string]$Instruction
    )
    return [pscustomobject]@{
        Message = $Message
        Instruction = $Instruction
    }
}

function Get-TemporarySecretCleanupInstruction {
    param([string]$TemporarySecret)
    return "gh secret delete $TemporarySecret --repo $Repository --env $EnvironmentName"
}

function Get-ArtifactCleanupInstruction {
    param([int64]$RunID, [string]$ArtifactName)
    return @(
        "gh run watch $RunID --repo $Repository"
        "if (`$LASTEXITCODE -ne 0) { throw 'Could not wait for re-encryption run $RunID' }"
        "`$artifactIDs = @(gh api `"repos/$Repository/actions/runs/$RunID/artifacts`" --jq '.artifacts[] | select(.name == `"$ArtifactName`") | .id')"
        "if (`$LASTEXITCODE -ne 0) { throw 'Could not list temporary Artifacts for run $RunID' }"
        "if (`$artifactIDs.Count -eq 0) { Write-Host 'No matching temporary Artifact; cleanup is already complete.' }"
        "foreach (`$artifactID in `$artifactIDs) {"
        "    gh api --method DELETE `"repos/$Repository/actions/artifacts/`$artifactID`""
        "    if (`$LASTEXITCODE -ne 0) { throw `"Could not delete temporary Artifact `$artifactID`" }"
        "}"
    ) -join [Environment]::NewLine
}

function Get-RequestArtifactCleanupInstruction {
    param([string]$RequestID, [string]$ArtifactName)
    return @(
        "`$runID = gh run list --repo $Repository --workflow $Workflow --event workflow_dispatch --limit 30 --json databaseId,displayTitle --jq '.[] | select(.displayTitle | contains(`"$RequestID`")) | .databaseId'"
        "if (`$LASTEXITCODE -ne 0) { throw 'Could not list re-encryption runs' }"
        "if (-not `$runID) { throw 'Could not locate re-encryption run $RequestID' }"
        "gh run watch `$runID --repo $Repository"
        "if (`$LASTEXITCODE -ne 0) { throw `"Could not wait for re-encryption run `$runID`" }"
        "`$artifactIDs = @(gh api `"repos/$Repository/actions/runs/`$runID/artifacts`" --jq '.artifacts[] | select(.name == `"$ArtifactName`") | .id')"
        "if (`$LASTEXITCODE -ne 0) { throw `"Could not list temporary Artifacts for run `$runID`" }"
        "if (`$artifactIDs.Count -eq 0) { Write-Host 'No matching temporary Artifact; cleanup is already complete.' }"
        "foreach (`$artifactID in `$artifactIDs) {"
        "    gh api --method DELETE `"repos/$Repository/actions/artifacts/`$artifactID`""
        "    if (`$LASTEXITCODE -ne 0) { throw `"Could not delete temporary Artifact `$artifactID`" }"
        "}"
    ) -join [Environment]::NewLine
}

function Get-LocalCleanupInstruction {
    param([string]$Path)
    $literal = $Path.Replace("'", "''")
    return "Remove-Item -LiteralPath '$literal' -Recurse -Force"
}

function Complete-GitHubMigrationResult {
    param(
        [bool]$MigrationSucceeded,
        [object]$OperationError,
        [object[]]$CleanupFailures,
        [ValidateSet('old', 'new', 'uncertain')][string]$CredentialState = 'old'
    )
    if ($CleanupFailures.Count -gt 0) {
        $cleanup = @($CleanupFailures | ForEach-Object { $_.Message }) -join '; '
        $instructions = @($CleanupFailures | ForEach-Object { $_.Instruction } | Select-Object -Unique) -join [Environment]::NewLine
        if ($MigrationSucceeded) {
            throw "Minisign migration completed, but cleanup failed: $cleanup. Do not rerun the migration; run only these cleanup commands:`n$instructions"
        }
        $operation = if ($null -ne $OperationError) { $OperationError.ToString() } else { 'Minisign migration did not complete' }
        $authority = if ($CredentialState -eq 'old') {
            'The old credentials remain authoritative.'
        }
        else {
            'The credential state is uncertain because rollback did not complete; inspect and repair the local key, DPAPI file, and permanent GitHub Secrets before continuing.'
        }
        throw "$operation. Cleanup also failed: $cleanup. $authority Run only these cleanup commands:`n$instructions"
    }
    if ($null -ne $OperationError -and $CredentialState -eq 'uncertain') {
        throw "$($OperationError.ToString()). The credential state is uncertain because rollback did not complete; inspect and repair the local key, DPAPI file, and permanent GitHub Secrets before continuing."
    }
    if ($null -ne $OperationError) { throw $OperationError }
}

function Invoke-StorePassword {
    $password = Read-HiddenText 'Minisign password to store with Windows DPAPI CurrentUser'
    $confirmation = Read-HiddenText 'Confirm minisign password'
    if ($password -cne $confirmation) { throw 'Password confirmation does not match' }
    Invoke-LocalSigningSelfTest $PrivateKeyFile $password
    $protected = Protect-CurrentUserPassword $password
    $directory = Split-Path -Parent $DPAPIPasswordFile
    New-Item -ItemType Directory -Path $directory -Force | Out-Null
    [IO.File]::WriteAllBytes($DPAPIPasswordFile, $protected)
    Write-Host "Stored the minisign password with DPAPI CurrentUser: $DPAPIPasswordFile"
}

function Invoke-GitHubMigration {
    if (-not $script:IsWindowsPlatform) { throw 'GitHub minisign migration must run on the Windows release machine' }
    foreach ($tool in @('gh', 'go')) {
        if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) { throw "Required tool is missing: $tool" }
    }
    if (-not (Test-Path -LiteralPath $PrivateKeyFile -PathType Leaf)) {
        throw "Encrypted minisign private key does not exist: $PrivateKeyFile"
    }
    $oldPassword = Get-StoredSigningPassword
    Invoke-LocalSigningSelfTest $PrivateKeyFile $oldPassword
    $newPassword = Read-HiddenText 'New minisign password'
    $confirmation = Read-HiddenText 'Confirm new minisign password'
    if ($newPassword -cne $confirmation) { throw 'New password confirmation does not match' }
    if ($newPassword -ceq $oldPassword) { throw 'New password must differ from the current password' }

    $requestID = [guid]::NewGuid().ToString('N')
    $artifactName = "minisign-reencrypted-$requestID"
    $temporarySecret = 'MINISIGN_NEW_PASSWORD_TEMP'
    $downloadRoot = Join-Path ([IO.Path]::GetTempPath()) "kwu-minisign-$requestID"
    $candidate = Join-Path $downloadRoot 'suiyuan.key'
    $runID = 0L
    $temporarySecretAttempted = $false
    $reencryptionDispatchAttempted = $false
    $migrationSucceeded = $false
    $credentialState = 'old'
    $operationError = $null
    $cleanupFailures = [Collections.Generic.List[object]]::new()
    try {
        New-Item -ItemType Directory -Path $downloadRoot | Out-Null
        $temporarySecretAttempted = $true
        Set-GitHubEnvironmentSecret $temporarySecret $newPassword
        $reencryptionDispatchAttempted = $true
        & gh workflow run $Workflow --repo $Repository --ref main-v3 `
            -f operation=reencrypt -f request_id=$requestID 2>&1 | Out-Null
        if ($LASTEXITCODE -ne 0) { throw 'Could not dispatch the Doctor re-encryption workflow' }
        $runID = Get-ReencryptionRunID $requestID
        & gh run watch $runID --repo $Repository --exit-status
        if ($LASTEXITCODE -ne 0) { throw "Doctor re-encryption workflow failed: run $runID" }
        & gh run download $runID --repo $Repository --name $artifactName --dir $downloadRoot
        if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $candidate -PathType Leaf)) {
            throw "Could not download the re-encrypted private key from run $runID"
        }
        Invoke-LocalSigningSelfTest $candidate $newPassword

        $oldKeyText = [IO.File]::ReadAllText($PrivateKeyFile)
        $newKeyText = [IO.File]::ReadAllText($candidate)
        $backup = "$PrivateKeyFile.backup-$([DateTime]::UtcNow.ToString('yyyyMMddHHmmss'))"
        $dpapiCandidate = Join-Path $downloadRoot 'minisign-password.dpapi'
        [IO.File]::WriteAllBytes($dpapiCandidate, (Protect-CurrentUserPassword $newPassword))
        $dpapiExisted = Test-Path -LiteralPath $DPAPIPasswordFile -PathType Leaf
        $dpapiBackup = "$backup.dpapi"
        if ($dpapiExisted) { Copy-Item -LiteralPath $DPAPIPasswordFile -Destination $dpapiBackup }
        Copy-Item -LiteralPath $PrivateKeyFile -Destination $backup
        try {
            $credentialState = 'uncertain'
            Set-GitHubEnvironmentSecret 'MINISIGN_PRIVATE_KEY' $newKeyText
            Set-GitHubEnvironmentSecret 'MINISIGN_PASSWORD' $newPassword
            Copy-Item -LiteralPath $candidate -Destination $PrivateKeyFile -Force
            Invoke-LocalSigningSelfTest $PrivateKeyFile $newPassword
            Copy-Item -LiteralPath $dpapiCandidate -Destination $DPAPIPasswordFile -Force
            Write-Host "Minisign key re-encryption completed with the same signing identity. Backup: $backup"
            $migrationSucceeded = $true
            $credentialState = 'new'
        }
        catch {
            $migrationError = $_.Exception.Message
            $rollbackErrors = @()
            try { Copy-Item -LiteralPath $backup -Destination $PrivateKeyFile -Force }
            catch { $rollbackErrors += "local key: $($_.Exception.Message)" }
            try {
                if ($dpapiExisted) { Copy-Item -LiteralPath $dpapiBackup -Destination $DPAPIPasswordFile -Force }
                elseif (Test-Path -LiteralPath $DPAPIPasswordFile) {
                    Remove-Item -LiteralPath $DPAPIPasswordFile -Force -ErrorAction Stop
                }
            }
            catch { $rollbackErrors += "DPAPI password: $($_.Exception.Message)" }
            try { Set-GitHubEnvironmentSecret 'MINISIGN_PASSWORD' $oldPassword }
            catch { $rollbackErrors += "MINISIGN_PASSWORD: $($_.Exception.Message)" }
            try { Set-GitHubEnvironmentSecret 'MINISIGN_PRIVATE_KEY' $oldKeyText }
            catch { $rollbackErrors += "MINISIGN_PRIVATE_KEY: $($_.Exception.Message)" }
            if ($rollbackErrors.Count -ne 0) {
                $credentialState = 'uncertain'
                throw "Migration failed: $migrationError. Rollback also needs manual repair: $($rollbackErrors -join '; ')"
            }
            $credentialState = 'old'
            throw "Migration failed and was rolled back: $migrationError"
        }
    }
    catch { $operationError = $_ }
    finally {
        if ($temporarySecretAttempted) {
            try {
                Invoke-CleanupWithRetry "delete temporary Environment secret $temporarySecret" {
                    Remove-GitHubEnvironmentSecret $temporarySecret
                }
            }
            catch {
                $cleanupFailures.Add((New-MigrationCleanupFailure $_.Exception.Message `
                    (Get-TemporarySecretCleanupInstruction $temporarySecret)))
            }
        }
        if ($reencryptionDispatchAttempted -and $runID -eq 0) {
            try { $runID = Find-ReencryptionRunID $requestID }
            catch {
                $cleanupFailures.Add((New-MigrationCleanupFailure $_.Exception.Message `
                    (Get-RequestArtifactCleanupInstruction $requestID $artifactName)))
            }
            if ($runID -eq 0 -and -not ($cleanupFailures | Where-Object { $_.Instruction.Contains($requestID) })) {
                $cleanupFailures.Add((New-MigrationCleanupFailure `
                    "Could not locate the dispatched re-encryption run for request $requestID" `
                    (Get-RequestArtifactCleanupInstruction $requestID $artifactName)))
            }
        }
        if ($runID -gt 0) {
            try { Assert-ReencryptionRunCompleted $runID }
            catch {
                $cleanupFailures.Add((New-MigrationCleanupFailure $_.Exception.Message `
                    (Get-ArtifactCleanupInstruction $runID $artifactName)))
            }
            try {
                Invoke-CleanupWithRetry "delete temporary Artifact $artifactName" {
                    Remove-ReencryptionArtifact $runID $artifactName
                }
            }
            catch {
                $cleanupFailures.Add((New-MigrationCleanupFailure $_.Exception.Message `
                    (Get-ArtifactCleanupInstruction $runID $artifactName)))
            }
        }
        try {
            Remove-LocalMigrationDirectory $downloadRoot
        }
        catch {
            $cleanupFailures.Add((New-MigrationCleanupFailure $_.Exception.Message `
                (Get-LocalCleanupInstruction $downloadRoot)))
        }
        $env:MINISIGN_PASSWORD = $null
        $env:MINISIGN_NEW_PASSWORD = $null
        if (-not $migrationSucceeded -and $credentialState -eq 'old') {
            Write-Warning 'Migration did not complete; the old local key and permanent GitHub Secrets remain authoritative.'
        }
        elseif ($credentialState -eq 'uncertain') {
            Write-Warning 'Migration did not complete cleanly and rollback was incomplete; credential state is uncertain and requires manual repair.'
        }
    }
    Complete-GitHubMigrationResult $migrationSucceeded $operationError @($cleanupFailures) $credentialState
}

if ($MyInvocation.InvocationName -eq '.') { return }

switch ($Action) {
    'SelfTest' {
        $password = Get-StoredSigningPassword
        Invoke-LocalSigningSelfTest $PrivateKeyFile $password
    }
    'StorePassword' { Invoke-StorePassword }
    'MigrateGitHub' { Invoke-GitHubMigration }
}

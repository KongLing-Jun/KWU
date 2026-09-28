$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$scriptPath = Join-Path $PSScriptRoot 'desktop-signing-credentials.ps1'
$parseErrors = $null
$null = [Management.Automation.Language.Parser]::ParseFile($scriptPath, [ref]$null, [ref]$parseErrors)
if ($parseErrors.Count -ne 0) {
    throw "desktop signing credential script has parse errors: $($parseErrors | Out-String)"
}

$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$temp = Join-Path ([IO.Path]::GetTempPath()) ("kwu-signing-credentials-test-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $temp | Out-Null
try {
    $key = Join-Path $temp 'suiyuan.key'
    $dpapi = Join-Path $temp 'password.dpapi'
    [IO.File]::WriteAllText($key, 'encrypted test key placeholder')
    . $scriptPath -RepositoryRoot $root -PrivateKeyFile $key -DPAPIPasswordFile $dpapi

    if ($script:IsWindowsPlatform) {
        $password = 'temporary-test-password-' + [guid]::NewGuid().ToString('N')
        $protected = Protect-CurrentUserPassword $password
        if ([Text.Encoding]::UTF8.GetString($protected).Contains($password)) {
            throw 'DPAPI output exposed the plaintext password'
        }
        if ((Unprotect-CurrentUserPassword $protected) -cne $password) {
            throw 'DPAPI CurrentUser round trip changed the password'
        }
    }

    $env:MINISIGN_PRIVATE_KEY_FILE = 'before-key-file'
    $env:MINISIGN_PRIVATE_KEY = 'before-key-text'
    $env:MINISIGN_PASSWORD = 'before-password'
    $thrown = $false
    try {
        Invoke-WithMinisignEnvironment $key 'temporary-child-password' {
            if ($env:MINISIGN_PRIVATE_KEY_FILE -ne $key -or $env:MINISIGN_PRIVATE_KEY) {
                throw 'child key environment was not isolated'
            }
            if ($env:MINISIGN_PASSWORD -ne 'temporary-child-password') {
                throw 'child password environment was not set'
            }
            throw 'intentional child failure'
        }
    }
    catch {
        if (-not $_.Exception.Message.Contains('intentional child failure')) { throw }
        $thrown = $true
    }
    if (-not $thrown) { throw 'test child did not fail' }
    if ($env:MINISIGN_PRIVATE_KEY_FILE -ne 'before-key-file' -or
        $env:MINISIGN_PRIVATE_KEY -ne 'before-key-text' -or
        $env:MINISIGN_PASSWORD -ne 'before-password') {
        throw 'minisign environment was not restored in finally'
    }

    $source = Get-Content -Raw -LiteralPath $scriptPath
    if ($source -match '(?i)(?:--password|-p)\s+\$' -or
        $source -match 'ArgumentList\.Add\([^\r\n]*(?:Password|Value)') {
        throw 'credential script appears to pass a password as a command-line argument'
    }
    if ($source -match '(?i)--body' -or
        -not $source.Contains('$start.RedirectStandardInput = $true') -or
        -not $source.Contains('$process.StandardInput.Write($Value)') -or
        -not $source.Contains('$process.StandardInput.Close()')) {
        throw 'GitHub Environment secrets must be passed only through redirected standard input'
    }
    $secretStart = New-GitHubEnvironmentSecretProcessStartInfo 'MINISIGN_PASSWORD'
    $secretArguments = @($secretStart.ArgumentList)
    if ($secretArguments -contains '--body' -or $secretArguments -contains '-' -or
        $secretArguments -contains 'temporary-child-password' -or
        -not $secretStart.RedirectStandardInput) {
        throw "GitHub Environment secret process arguments are unsafe: $($secretArguments -join ' ')"
    }
    $secretAttempt = $source.IndexOf('$temporarySecretAttempted = $true', [StringComparison]::Ordinal)
    $secretSet = $source.IndexOf('Set-GitHubEnvironmentSecret $temporarySecret $newPassword', [StringComparison]::Ordinal)
    $dispatchTracked = $source.IndexOf('$reencryptionDispatchAttempted = $true', [StringComparison]::Ordinal)
    $dispatchCall = $source.IndexOf('& gh workflow run $Workflow', [StringComparison]::Ordinal)
    if ($secretAttempt -lt 0 -or $secretAttempt -gt $secretSet -or
        $dispatchTracked -lt 0 -or $dispatchTracked -gt $dispatchCall) {
        throw 'ambiguous GitHub responses could bypass temporary credential cleanup tracking'
    }
    if (-not $source.Contains("'D:\suiyuan-release-key\suiyuan.key'")) {
        throw 'credential script does not use the canonical local key path'
    }

    $retryState = [pscustomobject]@{ Attempts = 0 }
    Invoke-CleanupWithRetry 'transient cleanup' {
        $retryState.Attempts += 1
        if ($retryState.Attempts -lt 3) { throw 'temporary API failure' }
    } -DelayMilliseconds 0
    if ($retryState.Attempts -ne 3) { throw "cleanup retry count = $($retryState.Attempts), want 3" }

    $retryState.Attempts = 0
    $retryFailure = ''
    try {
        Invoke-CleanupWithRetry 'persistent cleanup' {
            $retryState.Attempts += 1
            throw 'persistent API failure'
        } -DelayMilliseconds 0
    }
    catch { $retryFailure = $_.Exception.Message }
    if ($retryState.Attempts -ne 3 -or -not $retryFailure.Contains('failed after 3 attempts')) {
        throw "persistent cleanup did not fail after three attempts: $retryFailure"
    }

    $missingDeleteCalls = [Collections.Generic.List[string]]::new()
    function global:gh {
        $missingDeleteCalls.Add(($args -join ' '))
        if ($args[0] -eq 'secret' -and $args[1] -eq 'delete') {
            $global:LASTEXITCODE = 1
            return 'secret was already absent'
        }
        if ($args[0] -eq 'secret' -and $args[1] -eq 'list') {
            $global:LASTEXITCODE = 0
            return '[]'
        }
        throw "unexpected fake gh invocation: $($args -join ' ')"
    }
    try { Remove-GitHubEnvironmentSecret 'MINISIGN_NEW_PASSWORD_TEMP' }
    finally { Remove-Item Function:\gh -ErrorAction SilentlyContinue }
    if ($missingDeleteCalls.Count -ne 2) {
        throw "already-absent temporary Secret was not treated as an idempotent cleanup: $($missingDeleteCalls -join '; ')"
    }

    $localCleanup = Join-Path $temp 'local-cleanup'
    New-Item -ItemType Directory -Path $localCleanup | Out-Null
    [IO.File]::WriteAllText((Join-Path $localCleanup 'candidate.key'), 'sensitive test material')
    Remove-LocalMigrationDirectory $localCleanup -DelayMilliseconds 0
    if (Test-Path -LiteralPath $localCleanup) { throw 'local migration directory survived successful cleanup' }

    $failedLocalCleanup = Join-Path $temp 'failed-local-cleanup'
    New-Item -ItemType Directory -Path $failedLocalCleanup | Out-Null
    $localCleanupAttempts = [pscustomobject]@{ Count = 0 }
    $localCleanupError = ''
    try {
        Remove-LocalMigrationDirectory $failedLocalCleanup -DelayMilliseconds 0 -RemoveCommand {
            param([string]$Target)
            $localCleanupAttempts.Count += 1
            throw "simulated locked directory: $Target"
        }
    }
    catch { $localCleanupError = $_.Exception.Message }
    if ($localCleanupAttempts.Count -ne 3 -or
        -not $localCleanupError.Contains('failed after 3 attempts') -or
        -not (Test-Path -LiteralPath $failedLocalCleanup)) {
        throw "persistent local cleanup failure was not reported after three attempts: $localCleanupError"
    }
    Remove-LocalMigrationDirectory $failedLocalCleanup -DelayMilliseconds 0

    $secretFailure = New-MigrationCleanupFailure 'secret cleanup failed' `
        (Get-TemporarySecretCleanupInstruction 'MINISIGN_NEW_PASSWORD_TEMP')
    $artifactFailure = New-MigrationCleanupFailure 'artifact cleanup failed' `
        (Get-ArtifactCleanupInstruction 123 'minisign-reencrypted-test')
    $unknownRunInstruction = Get-RequestArtifactCleanupInstruction ('a' * 32) 'minisign-reencrypted-test'
    if (-not $unknownRunInstruction.Contains('gh run list') -or
        -not $unknownRunInstruction.Contains(('a' * 32)) -or
        -not $unknownRunInstruction.Contains('gh run watch $runID') -or
        -not $unknownRunInstruction.Contains('actions/runs/$runID/artifacts')) {
        throw "unknown run cleanup instructions cannot locate and delete the Artifact: $unknownRunInstruction"
    }
    $manualCleanupCalls = [Collections.Generic.List[string]]::new()
    function global:gh {
        $manualCleanupCalls.Add(($args -join ' '))
        $global:LASTEXITCODE = 0
        if ($args[0] -eq 'api' -and $args -notcontains '--method') { return }
    }
    try { Invoke-Expression (Get-ArtifactCleanupInstruction 123 'missing-artifact') }
    finally { Remove-Item Function:\gh -ErrorAction SilentlyContinue }
    if ($manualCleanupCalls.Count -ne 2 -or
        ($manualCleanupCalls | Where-Object { $_ -match '--method\s+DELETE' })) {
        throw "manual cleanup attempted DELETE for an absent Artifact: $($manualCleanupCalls -join '; ')"
    }
    $localFailure = New-MigrationCleanupFailure 'local cleanup failed' `
        (Get-LocalCleanupInstruction (Join-Path $temp 'stale-local'))

    $completedCleanupFailure = ''
    try {
        Complete-GitHubMigrationResult $true $null @($secretFailure) 'new'
    }
    catch { $completedCleanupFailure = $_.Exception.Message }
    if (-not $completedCleanupFailure.Contains('Do not rerun the migration') -or
        -not $completedCleanupFailure.Contains('gh secret delete MINISIGN_NEW_PASSWORD_TEMP') -or
        $completedCleanupFailure.Contains('actions/artifacts/$artifactID')) {
        throw "completed migration cleanup failure lacks recovery instructions: $completedCleanupFailure"
    }

    $artifactOnlyFailure = ''
    try { Complete-GitHubMigrationResult $true $null @($artifactFailure) 'new' }
    catch { $artifactOnlyFailure = $_.Exception.Message }
    if (-not $artifactOnlyFailure.Contains('actions/artifacts/$artifactID') -or
        $artifactOnlyFailure.Contains('gh secret delete MINISIGN_NEW_PASSWORD_TEMP')) {
        throw "artifact-only cleanup failure contains imprecise instructions: $artifactOnlyFailure"
    }

    $localOnlyFailure = ''
    try { Complete-GitHubMigrationResult $true $null @($localFailure) 'new' }
    catch { $localOnlyFailure = $_.Exception.Message }
    if (-not $localOnlyFailure.Contains('Remove-Item -LiteralPath') -or
        $localOnlyFailure.Contains('gh secret delete') -or
        $localOnlyFailure.Contains('actions/artifacts/$artifactID')) {
        throw "local-only cleanup failure contains imprecise instructions: $localOnlyFailure"
    }

    $failedCleanupFailure = ''
    try {
        Complete-GitHubMigrationResult $false ([InvalidOperationException]::new('primary migration failure')) `
            @($secretFailure) 'old'
    }
    catch { $failedCleanupFailure = $_.Exception.Message }
    if (-not $failedCleanupFailure.Contains('primary migration failure') -or
        -not $failedCleanupFailure.Contains('old credentials remain authoritative')) {
        throw "failed migration lost its primary error or authority guidance: $failedCleanupFailure"
    }

    $uncertainFailure = ''
    try {
        Complete-GitHubMigrationResult $false ([InvalidOperationException]::new('rollback needs manual repair')) `
            @() 'uncertain'
    }
    catch { $uncertainFailure = $_.Exception.Message }
    if (-not $uncertainFailure.Contains('rollback needs manual repair') -or
        -not $uncertainFailure.Contains('credential state is uncertain') -or
        $uncertainFailure.Contains('old credentials remain authoritative')) {
        throw "incomplete rollback gave unsafe authority guidance: $uncertainFailure"
    }

    Write-Host 'desktop signing credential safety tests passed'
}
finally {
    Remove-Item Env:MINISIGN_PRIVATE_KEY_FILE -ErrorAction SilentlyContinue
    Remove-Item Env:MINISIGN_PRIVATE_KEY -ErrorAction SilentlyContinue
    Remove-Item Env:MINISIGN_PASSWORD -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $temp -Recurse -Force -ErrorAction SilentlyContinue
}

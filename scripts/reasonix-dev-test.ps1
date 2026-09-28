$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$scriptPath = Join-Path $PSScriptRoot 'reasonix-dev.ps1'
$tokens = $null
$errors = $null
[Management.Automation.Language.Parser]::ParseFile($scriptPath, [ref]$tokens, [ref]$errors) | Out-Null
if ($errors.Count -ne 0) { throw ($errors | ForEach-Object Message | Out-String) }

. $scriptPath -LibraryOnly

function Assert-Throws {
    param([scriptblock]$Action, [string]$Pattern)
    try { & $Action; throw 'Expected the action to throw.' }
    catch { if ($_.Exception.Message -eq 'Expected the action to throw.' -or $_.Exception.Message -notmatch $Pattern) { throw } }
}

$canonical = ConvertTo-CanonicalCloudURL 'http://127.0.0.1:9090/'
if ($canonical -ne 'http://127.0.0.1:9090/') { throw 'Cloud URL normalization failed.' }
if ((ConvertTo-AdminProxyJSON $canonical) -ne '[["/__proxy","http://127.0.0.1:9090/"]]') { throw 'Admin proxy derivation failed.' }
Assert-Throws { ConvertTo-CanonicalCloudURL 'http://127.0.0.1:8080' } 'canonical integrated Cloud'

$selfProcess = Get-Process -Id $PID
$selfEntry = (Get-CimInstance Win32_Process -Filter "ProcessId = $PID").ExecutablePath
$selfRecord = New-ServiceRecord -Process $selfProcess -Port 65535 -EntryPath $selfEntry
if ([int]$selfRecord.pid -ne $PID -or -not $selfRecord.creation_time) { throw 'Process snapshot failed.' }
$roundTrippedRecord = $selfRecord | ConvertTo-Json | ConvertFrom-Json
if (-not (Test-ProcessCreationTime -Process (Get-CimInstance Win32_Process -Filter "ProcessId = $PID") -RecordedCreationTime $roundTrippedRecord.creation_time)) {
    throw 'Process creation time must survive JSON date conversion.'
}

$desktopMetadata = Get-GitMetadata -Root $script:DesktopRoot
$repositoryFixture = [pscustomobject]@{ desktop = [pscustomobject]$desktopMetadata; cloud = [pscustomobject]$desktopMetadata; admin = [pscustomobject]$desktopMetadata }
Assert-RepositoryState -Repositories $repositoryFixture
$repositoryFixture.admin.head = '0000000000000000000000000000000000000000'
Assert-Throws { Assert-RepositoryState -Repositories $repositoryFixture } 'repository state is stale'

$temporaryRoot = Join-Path ([IO.Path]::GetTempPath()) "reasonix-runtime-test-$([guid]::NewGuid().ToString('N'))"
try {
    New-Item -ItemType Directory -Path $temporaryRoot | Out-Null
    $entry = Join-Path $temporaryRoot 'entry.exe'
    [IO.File]::WriteAllBytes($entry, [byte[]](1, 2, 3))
    $service = [ordered]@{ pid = $PID; creation_time = (Get-Date).ToUniversalTime().ToString('o'); port = 9090; entry_path = $entry }
    $manifest = [ordered]@{
        schema_version = 1
        cloud_base_url = $canonical
        cloud_environment_id = '00000000-0000-0000-0000-000000000001'
        cloud_environment_name = 'test'
        created_at = (Get-Date).ToUniversalTime().ToString('o')
        repositories = [ordered]@{}
        services = [ordered]@{ cloud = $service }
    }
    $path = Join-Path $temporaryRoot 'runtime.json'
    Write-AtomicRuntimeManifest -Manifest $manifest -Path $path
    Write-AtomicRuntimeManifest -Manifest $manifest -Path $path
    $bytes = [IO.File]::ReadAllBytes($path)
    if ($bytes.Length -ge 3 -and $bytes[0] -eq 0xEF -and $bytes[1] -eq 0xBB -and $bytes[2] -eq 0xBF) { throw 'Manifest must not contain a UTF-8 BOM.' }
    if (@(Get-ChildItem -LiteralPath $temporaryRoot -Filter '*.tmp-*').Count -ne 0) { throw 'Atomic write left temporary files.' }

    $bad = [pscustomobject]@{}
    foreach ($property in $manifest.Keys) { Add-Member -InputObject $bad -NotePropertyName $property -NotePropertyValue $manifest[$property] }
    Add-Member -InputObject $bad -NotePropertyName password -NotePropertyValue 'sentinel-value'
    Assert-Throws { Assert-RuntimeManifest -Manifest $bad } 'unknown field|forbidden'
} finally {
    Remove-Item -LiteralPath $temporaryRoot -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host 'Reasonix development launcher contract tests passed.'

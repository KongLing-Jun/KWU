$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'desktop-actions-common.ps1')

function Assert-Throws {
    param([scriptblock]$Command, [string]$Message)
    try { & $Command }
    catch { return }
    throw $Message
}

$identity = Get-ReleaseIdentity '1.0.11' ''
if ($identity.Tag -ne 'desktop-v1.0.11' -or $identity.NotesPath -ne 'docs/releases/v1.0.11.md') {
    throw 'Stable desktop identity mapping changed'
}
Assert-Throws { Get-ReleaseIdentity 'v0.0.0' '' } 'v0.0.0 must be rejected'
Assert-Throws { Get-ReleaseIdentity 'v1.1000.0' '' } 'version segments above 999 must be rejected'
Assert-Throws { Get-ReleaseIdentity 'v1.0.0-rc1' '' } 'prerelease versions must be rejected'

if ((Assert-ReleaseNotesText "# 1.0.11`n`nRelease notes") -notmatch 'Release notes') {
    throw 'valid release notes were rejected'
}
Assert-Throws { Assert-ReleaseNotesText '<b>HTML</b>' } 'release notes HTML must be rejected'
Assert-Throws { Assert-ReleaseNotesText '![image](asset.png)' } 'release notes images must be rejected'
Assert-Throws { Assert-ReleaseNotesText ('x' * 65537) } 'oversized release notes must be rejected'

$temp = Join-Path ([IO.Path]::GetTempPath()) ('kwu-actions-common-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $temp | Out-Null
try {
    foreach ($name in $script:PrimaryAssets) {
        [IO.File]::WriteAllText((Join-Path $temp $name), "fixture-$name", [Text.Encoding]::UTF8)
    }
    $assets = @(Get-AssetMetadata $temp)
    $script:SourceRepository = 'xiaoxiaotest9527-bit/KWU'
    $tagObject = 'c' * 40
    $body = New-ReleaseBody $identity ('a' * 40) '# notes' $assets $tagObject
    if (-not $body.Contains('Emergency: `false`')) { throw 'normal release provenance is not explicit' }
    if (-not $body.Contains("Source tag object: ``$tagObject``")) { throw 'annotated tag object provenance is missing' }
    $release = [pscustomobject]@{ body = $body }
    Assert-ReleaseBodyMetadata $release $identity ('a' * 40) '# notes' $assets $tagObject
    Assert-Throws {
        Assert-ReleaseBodyMetadata $release $identity ('a' * 40) '# notes' $assets ''
    } 'Release body verification accepted an omitted annotated tag object'

    $script:EmergencyReason = 'ignored legacy caller state'
    $secondBody = New-ReleaseBody $identity ('b' * 40) '# second' $assets $tagObject
    if (-not $secondBody.Contains('Emergency: `false`') -or $secondBody.Contains('Emergency reason:')) {
        throw 'Release body still exposes an emergency publication pathway'
    }
}
finally { Remove-Item -LiteralPath $temp -Recurse -Force -ErrorAction SilentlyContinue }

# A tag can be rebuilt with a different annotation while still peeling to the
# same commit. The frozen tag object SHA must detect that movement.
function Get-RemoteAnnotatedTagIdentity {
    param([string]$ReleaseTag, [switch]$AllowMissing)
    return [pscustomobject]@{ ObjectSHA = ('d' * 40); CommitSHA = ('a' * 40) }
}
Assert-Throws {
    Assert-RemoteAnnotatedTagIdentity 'desktop-v1.0.11' ('c' * 40) ('a' * 40)
} 'rebuilding an annotated tag at the same commit must be rejected'

Write-Host 'desktop Actions common tests passed'

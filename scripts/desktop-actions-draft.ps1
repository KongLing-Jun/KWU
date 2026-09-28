[CmdletBinding()]
param(
    [ValidateSet('Resolve', 'CreateDraft', 'WaitForDraft', 'UploadAsset')]
    [string]$Action,
    [ValidateSet('tag', 'existing-tag', 'dry-run')]
    [string]$Mode = 'tag',
    [string]$Tag = '',
    [string]$Version = '',
    [string]$SourceCommit = '',
    [string]$NotesFile = '',
    [string]$ArtifactPath = '',
    [string]$ReleaseRepository = 'xiaoxiaotest9527-bit/KWU-release',
    [string]$SourceRepository = 'xiaoxiaotest9527-bit/KWU',
    [string]$TagObject = '',
    [string[]]$ReleaseTargets = @('all'),
    [int]$WaitTimeoutSeconds = 600,
    [int]$WaitIntervalSeconds = 5
)

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'desktop-actions-common.ps1')

function Invoke-Resolve {
    $normalizedTargets = @(Resolve-DesktopReleaseTargets $ReleaseTargets)
    Invoke-Native 'fetch origin/main-v3' {
        git fetch --force --no-tags origin '+refs/heads/main-v3:refs/remotes/origin/main-v3'
    } | Out-Null

    $effectiveTag = $Tag.Trim()
    $effectiveCommit = $SourceCommit.Trim().ToLowerInvariant()
    $effectiveTagObject = ''
    if ($Mode -eq 'tag' -or $Mode -eq 'existing-tag') {
        if ($Mode -eq 'tag') {
            if (-not $effectiveTag) { $effectiveTag = $env:GITHUB_REF_NAME }
        }
        else {
            $effectiveTag = (Get-ReleaseIdentity $Version '').Tag
            Invoke-Native "fetch existing annotated tag $effectiveTag" {
                git fetch --force --no-tags origin "+refs/tags/$effectiveTag`:refs/tags/$effectiveTag"
            } | Out-Null
        }
        $identity = Get-ReleaseIdentity '' $effectiveTag
        $tagType = (Invoke-Native "inspect annotated tag $effectiveTag" {
            git cat-file -t "refs/tags/$effectiveTag"
        } | Out-String).Trim()
        if ($tagType -ne 'tag') { throw "Source tag must be annotated: $effectiveTag" }
        $effectiveTagObject = (Invoke-Native "resolve annotated tag object $effectiveTag" {
            git rev-parse "refs/tags/$effectiveTag"
        } | Out-String).Trim().ToLowerInvariant()
        $effectiveCommit = (Invoke-Native "peel source tag $effectiveTag" {
            git rev-parse "$effectiveTag^{commit}"
        } | Out-String).Trim().ToLowerInvariant()
        Assert-RemoteAnnotatedTagIdentity $effectiveTag $effectiveTagObject $effectiveCommit | Out-Null
    }
    else {
        $identity = Get-ReleaseIdentity $Version ''
        $effectiveTag = $identity.Tag
        if ($Mode -eq 'dry-run') {
            $effectiveCommit = (Invoke-Native 'resolve origin/main-v3' {
                git rev-parse 'origin/main-v3^{commit}'
            } | Out-String).Trim().ToLowerInvariant()
        }
    }

    & git merge-base --is-ancestor $effectiveCommit 'origin/main-v3'
    $isMainV3 = $LASTEXITCODE -eq 0
    if (-not $isMainV3) {
        throw "Source commit is not reachable from origin/main-v3: $effectiveCommit"
    }

    $notesText = (Invoke-Native "read $($identity.NotesPath) at source commit" {
        git show "$effectiveCommit`:$($identity.NotesPath)"
    } | Out-String)
    Assert-ReleaseNotesText $notesText | Out-Null

    Write-ActionOutput tag $effectiveTag
    Write-ActionOutput version $identity.Version
    Write-ActionOutput version_name $identity.VersionName
    Write-ActionOutput source_commit $effectiveCommit
    Write-ActionOutput tag_object $effectiveTagObject
    Write-ActionOutput notes_path $identity.NotesPath
    Write-ActionOutput dry_run ($Mode -eq 'dry-run').ToString().ToLowerInvariant()
    Write-ActionOutput targets ($normalizedTargets -join ',')
}

function Invoke-CreateDraft {
    $identity = Get-ReleaseIdentity $Version $Tag
    $commit = $SourceCommit.Trim().ToLowerInvariant()
    if ($commit -notmatch '^[0-9a-f]{40}$') { throw 'SourceCommit must be an exact 40-character SHA' }
    $tagObject = $TagObject.Trim().ToLowerInvariant()
    if ($tagObject) {
        Assert-RemoteAnnotatedTagIdentity $identity.Tag $tagObject $commit | Out-Null
    }
    else {
        # Compatibility for direct callers predating -TagObject: freeze the current
        # remote object here. New workflows always pass the SHA resolved earlier.
        $remote = Get-RemoteAnnotatedTagIdentity $identity.Tag
        if ($remote.CommitSHA -ne $commit) {
            throw "Remote source tag $($identity.Tag) points to $($remote.CommitSHA) instead of $commit"
        }
        $tagObject = $remote.ObjectSHA
    }
    $notes = Get-ReleaseNotes $NotesFile
    $body = New-ReleaseBody $identity $commit $notes @() $tagObject
    $release = Get-ReleaseJSON $identity.Tag -AllowMissing

    if ($release) {
        if ($release.isDraft) { Assert-PreFinalizationDraft $release }
        else { Assert-AppendableReleaseAssets $release }
        Assert-ReleaseBodyMetadata $release $identity $commit $notes @() $tagObject
    }
    else {
        $bodyPath = Join-Path ([IO.Path]::GetTempPath()) ("kwu-release-body-" + [guid]::NewGuid().ToString('N') + '.md')
        try {
            [IO.File]::WriteAllText($bodyPath, $body + "`n", [Text.UTF8Encoding]::new($false))
            Invoke-Native "create Draft Release $($identity.Tag)" {
                gh release create $identity.Tag --repo $ReleaseRepository --draft `
                    --title "云智企办 $($identity.Version)" --notes-file $bodyPath
            } | Out-Null
        }
        finally { Remove-Item -LiteralPath $bodyPath -Force -ErrorAction SilentlyContinue }
        $release = Get-ReleaseJSON $identity.Tag
    }
    if ($release.isDraft) { Assert-PreFinalizationDraft $release }
    else { Assert-AppendableReleaseAssets $release }
    Assert-ReleaseBodyMetadata $release $identity $commit $notes @() $tagObject
    Write-ActionOutput release_url "https://github.com/$ReleaseRepository/releases/tag/$($identity.Tag)"
    Write-Host "Draft Release is ready for independent platform uploads: $ReleaseRepository/$($identity.Tag)"
}

function Invoke-WaitForDraft {
    if ($WaitTimeoutSeconds -lt 0) { throw 'WaitTimeoutSeconds must be non-negative' }
    if ($WaitIntervalSeconds -lt 0) { throw 'WaitIntervalSeconds must be non-negative' }
    $identity = Get-ReleaseIdentity $Version $Tag
    $commit = $SourceCommit.Trim().ToLowerInvariant()
    if ($commit -notmatch '^[0-9a-f]{40}$') { throw 'SourceCommit must be an exact 40-character SHA' }
    $tagObject = $TagObject.Trim().ToLowerInvariant()
    Assert-RemoteAnnotatedTagIdentity $identity.Tag $tagObject $commit | Out-Null
    $notes = Get-ReleaseNotes $NotesFile
    $deadline = [DateTime]::UtcNow.AddSeconds($WaitTimeoutSeconds)

    while ($true) {
        $release = Get-ReleaseJSON $identity.Tag -AllowMissing
        if ($release) {
            if (-not $release.isDraft) { throw "GitHub release $($identity.Tag) is not the expected Draft" }
            Assert-PreFinalizationDraft $release
            Assert-ReleaseBodyMetadata $release $identity $commit $notes @() $tagObject
            Write-Host "Exact Draft is ready: $ReleaseRepository/$($identity.Tag)"
            return
        }
        if ([DateTime]::UtcNow -ge $deadline) {
            throw "Timed out waiting for exact Draft $ReleaseRepository/$($identity.Tag) after $WaitTimeoutSeconds seconds"
        }
        if ($WaitIntervalSeconds -gt 0) { Start-Sleep -Seconds $WaitIntervalSeconds }
    }
}

function Assert-PreFinalizationDraft {
    param($Release)
    $names = @($Release.assets | ForEach-Object { [string]$_.name })
    if (@($names | Select-Object -Unique).Count -ne $names.Count) {
        throw 'Draft contains duplicate asset names'
    }
    foreach ($name in $names) {
        if ($script:PrimaryAssets -notcontains $name) {
            throw "Draft already entered finalization or contains an unexpected asset: $name"
        }
    }
}

function Assert-AppendableReleaseAssets {
    param($Release)
    $allowed = @(
        $script:PrimaryAssets
        @($script:PrimaryAssets | ForEach-Object { "$_.minisig" })
        @($script:DesktopReleaseTargets | ForEach-Object { "latest-$_.json" })
        'latest.json'
    )
    $names = @($Release.assets | ForEach-Object { [string]$_.name })
    if (@($names | Select-Object -Unique).Count -ne $names.Count) {
        throw 'Release contains duplicate asset names'
    }
    foreach ($name in $names) {
        if ($allowed -notcontains $name) {
            throw "Release contains an unexpected asset: $name"
        }
    }
}

function Get-PrimaryAssetMetadata {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "Primary asset does not exist: $Path"
    }
    $item = Get-Item -LiteralPath $Path
    if ($item.Length -le 0) { throw "Primary asset is empty: $Path" }
    if ($script:PrimaryAssets -notcontains $item.Name) {
        throw "Primary asset name is not allowed: $($item.Name)"
    }
    $architecture = switch ($item.Name) {
        'Clouvia-windows-amd64-installer.exe' { 'amd64 payload' }
        'Clouvia-windows-arm64-installer.exe' { 'arm64 payload' }
        default { 'x86_64+arm64 payload' }
    }
    return [pscustomobject]@{
        Name = $item.Name
        Path = $item.FullName
        Size = $item.Length
        SHA256 = Get-LowerSHA256 $item.FullName
        Architecture = $architecture
    }
}

function Get-ReleaseAssetMetadataLine {
    param($Metadata)
    return "- ``$($Metadata.Name)``: size=$($Metadata.Size) sha256=``$($Metadata.SHA256)`` architecture=$($Metadata.Architecture)"
}

function Set-ReleaseAssetMetadata {
    param($Identity, $Release, $Metadata)
    $body = ([string]$Release.body).Replace("`r`n", "`n").Trim()
    $line = Get-ReleaseAssetMetadataLine $Metadata
    $prefix = "- ``$($Metadata.Name)``:"
    $existing = @($body -split "`n" | Where-Object {
        $_.StartsWith($prefix, [StringComparison]::Ordinal)
    })
    if ($existing.Count -gt 1) { throw "Draft contains duplicate provenance for $($Metadata.Name)" }
    if ($existing.Count -eq 1) {
        if ($existing[0] -cne $line) {
            throw "Draft already contains different provenance for $($Metadata.Name)"
        }
        return
    }
    $body = $body.Replace("`n(Pending manual builds.)`n", "`n")
    $marker = "`n## 更新说明"
    $index = $body.IndexOf($marker, [StringComparison]::Ordinal)
    if ($index -lt 0) { throw 'Draft body has no update notes section' }
    $body = $body.Insert($index, "`n$line`n")
    $bodyPath = Join-Path ([IO.Path]::GetTempPath()) ("kwu-release-body-" + [guid]::NewGuid().ToString('N') + '.md')
    try {
        [IO.File]::WriteAllText($bodyPath, $body + "`n", [Text.UTF8Encoding]::new($false))
        Invoke-Native "record Draft provenance for $($Metadata.Name)" {
            gh release edit $Identity.Tag --repo $ReleaseRepository --notes-file $bodyPath
        } | Out-Null
    }
    finally { Remove-Item -LiteralPath $bodyPath -Force -ErrorAction SilentlyContinue }
}

function Invoke-UploadAsset {
    $identity = Get-ReleaseIdentity $Version $Tag
    $commit = $SourceCommit.Trim().ToLowerInvariant()
    if ($commit -notmatch '^[0-9a-f]{40}$') { throw 'SourceCommit must be an exact 40-character SHA' }
    $tagObject = $TagObject.Trim().ToLowerInvariant()
    Assert-RemoteAnnotatedTagIdentity $identity.Tag $tagObject $commit | Out-Null
    $notes = Get-ReleaseNotes $NotesFile
    $metadata = Get-PrimaryAssetMetadata $ArtifactPath
    $release = Get-ReleaseJSON $identity.Tag
    if ($release.isDraft) { Assert-PreFinalizationDraft $release }
    else { Assert-AppendableReleaseAssets $release }
    Assert-ReleaseBodyMetadata $release $identity $commit $notes @() $tagObject

    $existing = @($release.assets | Where-Object { [string]$_.name -eq $metadata.Name })
    if ($existing.Count -gt 1) { throw "Draft has duplicate asset entries: $($metadata.Name)" }
    if ($existing.Count -eq 1) {
        $temp = Join-Path ([IO.Path]::GetTempPath()) ("kwu-existing-" + [guid]::NewGuid().ToString('N'))
        New-Item -ItemType Directory -Path $temp | Out-Null
        try {
            Invoke-Native "download existing Draft asset $($metadata.Name)" {
                gh release download $identity.Tag --repo $ReleaseRepository `
                    --pattern $metadata.Name --dir $temp
            } | Out-Null
            if ((Get-LowerSHA256 (Join-Path $temp $metadata.Name)) -cne $metadata.SHA256) {
                throw "Existing Draft asset differs from local output: $($metadata.Name)"
            }
        }
        finally { Remove-Item -LiteralPath $temp -Recurse -Force -ErrorAction SilentlyContinue }
    }
    else {
        Invoke-Native "upload Draft asset $($metadata.Name)" {
            gh release upload $identity.Tag $metadata.Path --repo $ReleaseRepository
        } | Out-Null
    }

    $release = Get-ReleaseJSON $identity.Tag
    Set-ReleaseAssetMetadata $identity $release $metadata
    $verified = Get-ReleaseJSON $identity.Tag
    if ($verified.isDraft) { Assert-PreFinalizationDraft $verified }
    else { Assert-AppendableReleaseAssets $verified }
    Assert-ReleaseAssetNames $verified @($verified.assets | ForEach-Object { [string]$_.name })
    Assert-ReleaseBodyMetadata $verified $identity $commit $notes @($metadata) $tagObject
    Write-Host "Uploaded immutable Draft asset: $($metadata.Name)"
}

if ($MyInvocation.InvocationName -eq '.') { return }

switch ($Action) {
    'Resolve' { Invoke-Resolve }
    'CreateDraft' { Invoke-CreateDraft }
    'WaitForDraft' { Invoke-WaitForDraft }
    'UploadAsset' { Invoke-UploadAsset }
}

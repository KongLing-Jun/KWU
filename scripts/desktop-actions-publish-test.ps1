$ErrorActionPreference = 'Stop'

$workflow = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot '..\.github\workflows\desktop-publish.yml')
$helper = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot 'desktop-actions-publish.ps1')

foreach ($required in @(
    'workflow_dispatch:',
    'PUBLISH $env:INPUT_TAG',
    'macos-universal',
    'cloud-only',
    'required',
    'environment: desktop-release',
    'arm64_acceptance_confirmation:',
    'arm64_acceptance_sha256:',
    'ACCEPT $env:INPUT_TAG ARM64',
    '-Arm64AcceptanceConfirmation',
    '-Arm64AcceptanceSHA256',
    '-RequireTrustedSignature',
    'runs-on: windows-2025',
    'runs-on: macos-15',
    'runs-on: ubuntu-24.04',
    "GITHUB_REF -ne 'refs/heads/main-v3'",
    'group: desktop-stable-release',
    'desktop-actions-publish.ps1',
    'ValidateAssetReceipt',
    'receipt_windows_amd64',
    'go build -o "$RUNNER_TEMP/publish-cloud-release" ./cmd/publish-cloud-release',
    '-Action PublishCloudRecords',
    '-CloudConcurrency 3'
)) {
    if (-not $workflow.Contains($required)) { throw "Publish workflow is missing: $required" }
}
foreach ($trustedStep in @(
    'Check out trusted release tooling',
    'Check out trusted signing and publication tooling',
    'Check out trusted cloud publication tooling'
)) {
    $pattern = '(?ms)^      - name: ' + [regex]::Escape($trustedStep) + '\r?$.*?(?=^      - name:|\z)'
    $match = [regex]::Match($workflow, $pattern)
    if (-not $match.Success) {
        throw "Secret-bearing jobs must execute trusted main-v3 tooling: $trustedStep"
    }
    $step = $match.Value
    if (-not $step.Contains('persist-credentials: true')) {
        throw "Private source tag verification requires the read-only checkout credential: $trustedStep"
    }
}
if ([regex]::Matches($workflow, 'persist-credentials:\s*true').Count -ne 3) {
    throw 'Only the three trusted private-tag verification checkouts may explicitly persist credentials'
}
if ($workflow -match 'ref:\s+\$\{\{\s*inputs\.tag\s*\}\}') {
    throw 'Publish must never execute tooling checked out from the user-supplied tag'
}
if ($workflow -match 'ref:\s+main-v3' -or $workflow -notmatch 'ref:\s+\$\{\{ github\.sha \}\}') {
    throw 'Publish trusted tooling must be frozen to the exact workflow event SHA'
}
if ($workflow -match '(?m)^\s{2}push:\s*$' -or $workflow -match '(?m)^\s{2}pull_request:\s*$') {
    throw 'Publish workflow must be workflow_dispatch-only'
}
if ([regex]::Matches($workflow, '\$\{\{\s*inputs\.release_target\s*\}\}').Count -ne 1 -or
    $workflow -notmatch 'targets:\s+\$\{\{ steps\.release\.outputs\.targets \}\}' -or
    $workflow -notmatch 'release_target all cannot be combined with another target') {
    throw 'Publish workflow does not establish one strict normalized target output at resolve'
}
if ($workflow -notmatch '(?s)publish-cloud-only:.*?INPUT_RELEASE_TARGET: \$\{\{ needs\.resolve\.outputs\.targets \}\}.*?-Action ValidateOssManifest.*?-ReleaseTargets \$env:INPUT_RELEASE_TARGET') {
    throw 'Cloud-only recovery does not consume the normalized resolve target output'
}
foreach ($forbidden in @('--clobber', 'SIGNPATH_', 'ubuntu-latest', 'windows-latest', 'macos-latest',
    'actions/upload-artifact', 'actions/download-artifact', 'approved-desktop-',
    'go run ./cmd/publish-cloud-release',
    'allow_non_main_v3', 'emergency_reason', 'AllowNonMainV3', 'EmergencyReason',
    'cloud_skip_reason', 'CloudSkipReason', "ValidateSet('required', 'skip')", 'Mode: skipped')) {
    if ($workflow.Contains($forbidden) -or $helper.Contains($forbidden)) {
        throw "Publish flow must not contain: $forbidden"
    }
}
foreach ($required in @(
    'Assert-ResumableReleaseAssets',
    'Downloaded Draft asset differs from approved receipt',
    'download existing signature',
    'verify existing signature',
    'Existing GitHub manifest differs',
    'Reusing already-published GitHub Release',
    'Assert-OssManifestMatchesReleaseAssets',
    'latest-$Target.json',
    '--notes-file',
    '--forbid-overwrite',
    'channels/stable/$Target/latest.json',
    'Assert-Arm64Acceptance'
)) {
    if (-not $helper.Contains($required)) { throw "Publish recovery behavior is missing: $required" }
}
foreach ($receiptStepName in @(
    'Compare Windows downloads with approved receipts',
    'Compare macOS download with approved receipt',
    'Compare publication downloads with approved receipts'
)) {
    $pattern = '(?ms)^      - name: ' + [regex]::Escape($receiptStepName) + '\r?$.*?(?=^      - name:|\z)'
    $step = [regex]::Match($workflow, $pattern).Value
    if (-not $step -or $step.Contains('RELEASE_REPO_TOKEN') -or $step.Contains('GH_TOKEN')) {
        throw "Receipt validation must run without the release repository token: $receiptStepName"
    }
}
foreach ($downloadStepName in @(
    'Download exact approved Windows Draft assets',
    'Download exact approved macOS Draft asset',
    'Download exact approved Draft assets for publication'
)) {
    $pattern = '(?ms)^      - name: ' + [regex]::Escape($downloadStepName) + '\r?$.*?(?=^      - name:|\z)'
    $step = [regex]::Match($workflow, $pattern).Value
    if (-not $step.Contains('RELEASE_REPO_TOKEN') -or -not $step.Contains('gh release download') -or
        $step.Contains('desktop-actions-publish.ps1')) {
        throw "Release repository token must be scoped to an exact download step: $downloadStepName"
    }
}
if ($workflow -notmatch '(?s)-Action ValidateOssManifest.*?-Action PublishCloudRecords') {
    throw 'cloud-only recovery must bind the OSS manifest to GitHub assets before POST'
}
$cloudRecordBody = $helper.Substring($helper.IndexOf('function Invoke-PublishCloudRecords'))
$recordPreflight = $cloudRecordBody.IndexOf('Invoke-CloudPublisherPhase $requests -Preflight')
$recordPublish = $cloudRecordBody.IndexOf('Invoke-CloudPublisherPhase $requests', $recordPreflight + 1)
if ($recordPreflight -lt 0 -or $recordPublish -le $recordPreflight) {
    throw 'Every cloud publication path must complete the selected preflight barrier before the first POST'
}
$cloudExecutorBody = $helper.Substring($helper.IndexOf('$script:CloudPhaseExecutor'))
if ($cloudExecutorBody -notmatch 'ForEach-Object -Parallel' -or
    $cloudExecutorBody -notmatch '-ThrottleLimit \$ThrottleLimit' -or
    $cloudExecutorBody -notmatch 'failed for targets') {
    throw 'Cloud target work must use bounded concurrency and fully validate drained results'
}
$cloudRows = $workflow.IndexOf('Publish and verify cloud release rows')
$pointerAdvance = $workflow.IndexOf('Advance selected stable pointers last')
if ($cloudRows -lt 0 -or $pointerAdvance -le $cloudRows) {
    throw 'Selected cloud records must publish before stable pointers advance'
}
if ($workflow -notmatch 'tag_object:\s+\$\{\{ steps\.release\.outputs\.tag_object \}\}' -or
    $workflow -notmatch "-TagObject '\$\{\{ needs\.resolve\.outputs\.tag_object \}\}'") {
    throw 'Publish must carry the frozen annotated tag object into every release verification'
}
$pointerBody = $helper.Substring($helper.IndexOf('function Invoke-PublishPointers'))
$pointerIdentityGate = $pointerBody.IndexOf('Assert-FrozenReleaseIdentity')
$pointerReleaseRead = $pointerBody.IndexOf('Get-ReleaseJSON')
$pointerWrite = $pointerBody.IndexOf('Publish-OssStablePointer')
$pointerPreflight = $pointerBody.IndexOf('Assert-PointerPublicationPreflight')
if ($pointerIdentityGate -lt 0 -or $pointerReleaseRead -le $pointerIdentityGate -or $pointerWrite -le $pointerReleaseRead) {
    throw 'Pointer publication must verify the frozen tag and published Release before any pointer write'
}
if ($pointerPreflight -lt 0 -or $pointerPreflight -ge $pointerWrite -or
    $pointerPreflight -ge $pointerBody.IndexOf('Set-GitHubLatestForBootstrap')) {
    throw 'All target and global pointer transitions must pass before GitHub Latest or any pointer write'
}
if ($workflow -notmatch '(?s)-Action PublishPointers.*?-SourceCommit.*?-TagObject.*?-NotesFile') {
    throw 'Pointer publication workflow does not pass the complete frozen release identity'
}
if ($workflow -notmatch '(?s)Advance selected stable pointers last.*?GH_TOKEN:.*?-Action PublishPointers') {
    throw 'Pointer publication cannot read and verify the published private Release'
}
if ($helper -match "'api',\s*'(list|delete)|delete-object|remove-object|\sls\s+oss://") {
    throw 'Publish helper must not use OSS ListObjects or DeleteObject'
}

$publishBody = $helper.Substring($helper.IndexOf('function Invoke-PublishGitHubOss'))
$zeroWriteGate = $publishBody.IndexOf('Assert-ZeroWritePublicationPreflight $identity')
$firstGitHubWrite = $publishBody.IndexOf('$finalDraft = Assert-OrUploadFinalDraftAssets')
$firstOssWrite = $publishBody.IndexOf('Publish-OssImmutableObject "$prefix/')
if ($zeroWriteGate -lt 0 -or $firstGitHubWrite -le $zeroWriteGate -or $firstOssWrite -le $zeroWriteGate) {
    throw 'The complete zero-write preflight must run before the first GitHub or OSS write'
}
$publishedResume = $publishBody.IndexOf('Reusing already-published GitHub Release')
$stableRepair = $publishBody.IndexOf('Publish-OssStablePointer')
if ($publishedResume -lt 0 -or $stableRepair -le $publishedResume) {
    throw 'An already-published Release must still safely repair the stable pointer'
}
$stableGuard = $publishBody.IndexOf('Assert-PublicStablePointerCanAdvance')
if ($stableGuard -lt 0 -or $stableGuard -ge $stableRepair) {
    throw 'The public stable pointer must pass the monotonic-version gate before overwrite'
}
$secondCloudPreflight = $publishBody.IndexOf('Invoke-CloudPreflight')
$githubPublication = $publishBody.IndexOf('gh release edit')
if ($secondCloudPreflight -lt 0 -or $githubPublication -lt 0 -or $secondCloudPreflight -ge $githubPublication) {
    throw 'The short-lived cloud token must be rechecked inside publication immediately before GitHub becomes public'
}
$publishExecution = $publishBody.Substring(0, $publishBody.IndexOf('function Set-GitHubLatestForBootstrap'))
if ($publishExecution -match '--latest(?![=\w-])' -or $publishExecution -notmatch '--draft=false --latest=false') {
    throw 'Immutable publication must never advance GitHub Latest before cloud rows succeed'
}
$pointerFunction = $helper.Substring($helper.IndexOf('function Invoke-PublishPointers'))
if ($pointerFunction -notmatch '(?s)if \(\$Bootstrap\).*?Set-GitHubLatestForBootstrap' -or
    $pointerFunction.IndexOf('Set-GitHubLatestForBootstrap') -gt $pointerFunction.IndexOf('Publish-OssStablePointer')) {
    throw 'Bootstrap GitHub Latest must advance only inside the final pointer stage'
}
if ($workflow -notmatch '(?s)Minisign and publish immutable GitHub and OSS assets.*?KWU_CLOUD_RELEASE_TOKEN:.*?-CloudBaseURL') {
    throw 'The publication step does not provide the cloud credential needed for the second preflight'
}

. (Join-Path $PSScriptRoot 'desktop-actions-publish.ps1') -Action ValidateDraft
function Assert-Throws {
    param([scriptblock]$Command, [string]$Message)
    try { & $Command }
    catch { return }
    throw $Message
}
function Assert-RemoteAnnotatedTagIdentity {
    param([string]$ReleaseTag, [string]$ExpectedObjectSHA, [string]$ExpectedCommitSHA)
    if ($ExpectedObjectSHA -cne ('2' * 40) -or $ExpectedCommitSHA -cne ('1' * 40)) {
        throw 'frozen tag identity mismatch'
    }
}
$identityGateFixture = Get-ReleaseIdentity '1.2.3' ''
Assert-Throws {
    Assert-FrozenReleaseIdentity $identityGateFixture ('1' * 40) ''
} 'A missing TagObject bypassed annotated tag verification'
Assert-Throws {
    Assert-FrozenReleaseIdentity $identityGateFixture ('1' * 40) ('3' * 40)
} 'An incorrect TagObject bypassed annotated tag verification'
Assert-FrozenReleaseIdentity $identityGateFixture ('1' * 40) ('2' * 40)

$candidateBootstrap = 'desktop-v2.0.0'
$oldBootstrap = 'desktop-v1.0.0'
Assert-BootstrapStateTransition $candidateBootstrap $oldBootstrap $false $oldBootstrap @()
Assert-BootstrapStateTransition $candidateBootstrap $oldBootstrap $false $candidateBootstrap @($candidateBootstrap)
Assert-BootstrapStateTransition $candidateBootstrap $candidateBootstrap $true $candidateBootstrap @(
    $candidateBootstrap, $candidateBootstrap, $candidateBootstrap
)
Assert-Throws {
    Assert-BootstrapStateTransition $candidateBootstrap $oldBootstrap $true $candidateBootstrap @()
} 'A new bootstrap was accepted after another migration completed with target pointers missing'
Assert-BootstrapStateTransition $oldBootstrap $oldBootstrap $true $candidateBootstrap @($oldBootstrap)
Assert-Throws {
    Assert-BootstrapStateTransition $candidateBootstrap $oldBootstrap $false 'desktop-v1.5.0' @()
} 'A different in-progress bootstrap candidate was ignored'
Assert-Throws {
    Assert-BootstrapStateTransition $candidateBootstrap $candidateBootstrap $true $candidateBootstrap @('desktop-v2.1.0')
} 'A non-exact bootstrap resume was accepted after a target advanced'
if ($helper -notmatch 'read frozen global GitHub Release \$globalTag' -or
    $helper -match '\$latestHasMigrationAssets') {
    throw 'Bootstrap completion must be proven by the global tag Release, not current GitHub Latest assets'
}
$normalized = @(Resolve-DesktopReleaseTargets @('windows-arm64', 'windows-amd64', 'windows-arm64'))
if (($normalized -join ',') -cne 'windows-amd64,windows-arm64') {
    throw "Target normalization failed: $($normalized -join ',')"
}
Assert-Throws { Resolve-DesktopReleaseTargets @() } 'Empty target selection was accepted'
Assert-Throws { Resolve-DesktopReleaseTargets @('unknown') } 'Unknown target selection was accepted'
Assert-Throws { Resolve-DesktopReleaseTargets @('all', 'windows-amd64') } 'Mixed all target selection was accepted'
Assert-Throws { Resolve-DesktopReleaseTargets @('all', 'unknown') } 'All plus unknown target selection was accepted'
$workflowNormalized = @(Resolve-DesktopReleaseTargets @('macos-universal', 'windows-amd64', 'windows-amd64'))
if (($workflowNormalized -join ',') -cne 'windows-amd64,macos-universal') {
    throw "Duplicate/unsorted workflow targets were not normalized: $($workflowNormalized -join ',')"
}
$combinations = @(
    @('windows-amd64'),
    @('windows-arm64'),
    @('macos-universal'),
    @('windows-amd64', 'windows-arm64'),
    @('windows-amd64', 'macos-universal'),
    @('windows-arm64', 'macos-universal'),
    @('all')
)
foreach ($inputTargets in $combinations) {
    $targets = @(Resolve-DesktopReleaseTargets $inputTargets)
    $assets = @(Get-DesktopTargetAssetNames $targets)
    $platforms = @($targets | ForEach-Object { $script:DesktopTargetPlatforms[$_] })
    $expectedAssets = $targets.Count
    $expectedPlatforms = @($targets | ForEach-Object {
        if ($_ -eq 'macos-universal') { 2 } else { 1 }
    } | Measure-Object -Sum).Sum
    if ($assets.Count -ne $expectedAssets -or $platforms.Count -ne $expectedPlatforms) {
        throw "Combination mapping failed for $($inputTargets -join ',')"
    }
    foreach ($target in $targets) {
        $expectedAsset = $script:DesktopTargetAssets[$target]
        if ($assets -notcontains $expectedAsset) {
            throw "Asset selection failed for $target"
        }
        if (-not (Get-SelectedManifestName $target).Equals("latest-$target.json")) {
            throw "Manifest selection failed for $target"
        }
        $expectedCloudPlatforms = switch ($target) {
            'windows-amd64' { @('windows-amd64') }
            'windows-arm64' { @('windows-arm64') }
            'macos-universal' { @('darwin-amd64', 'darwin-arm64') }
        }
        if ((@($script:DesktopTargetPlatforms[$target]) -join ',') -cne ($expectedCloudPlatforms -join ',')) {
            throw "Cloud record selection failed for $target"
        }
        $pointer = "channels/stable/$target/latest.json"
        if (-not $pointer.Contains($target)) { throw "Pointer selection failed for $target" }
    }
}
$partial = [pscustomobject]@{ isDraft = $true; assets = @(
    $script:PrimaryAssets | ForEach-Object { [pscustomobject]@{ name = $_ } }
) + @([pscustomobject]@{ name = 'Clouvia-windows-amd64-installer.exe.minisig' }) }
Assert-ResumableReleaseAssets $partial | Out-Null
Assert-Throws {
    Assert-ResumableReleaseAssets $partial -RequireFinal
} 'A published partial asset set must be rejected'
$partial.assets += [pscustomobject]@{ name = 'unexpected.zip' }
Assert-Throws {
    Assert-ResumableReleaseAssets $partial
} 'Unexpected partial Draft assets must be rejected'

$temp = Join-Path ([IO.Path]::GetTempPath()) ('kwu-oss-binding-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $temp | Out-Null
try {
    $identity = Get-ReleaseIdentity '1.0.11' ''
    $platforms = [ordered]@{}
    $releaseAssets = [ordered]@{
        'windows-amd64' = 'Clouvia-windows-amd64-installer.exe'
        'windows-arm64' = 'Clouvia-windows-arm64-installer.exe'
        'darwin-amd64' = 'Clouvia-darwin-universal.dmg'
        'darwin-arm64' = 'Clouvia-darwin-universal.dmg'
    }
    foreach ($mapping in $releaseAssets.GetEnumerator()) {
        $path = Join-Path $temp $mapping.Value
        if (-not (Test-Path -LiteralPath $path)) {
            [IO.File]::WriteAllText($path, "fixture-$($mapping.Value)", [Text.Encoding]::UTF8)
        }
        $url = "$OssPublicBaseURL/releases/$($identity.Tag)/$([uri]::EscapeDataString($mapping.Value))"
        $platforms[$mapping.Key] = [ordered]@{
            url = $url
            sig = "$url.minisig"
            size = (Get-Item -LiteralPath $path).Length
            sha256 = Get-LowerSHA256 $path
        }
    }
    $assetMetadata = @(Get-AssetMetadata $temp)
    $receipts = @()
    foreach ($target in $script:DesktopReleaseTargets) {
        $assetName = $script:DesktopTargetAssets[$target]
        $asset = @($assetMetadata | Where-Object Name -eq $assetName)[0]
        $receipts += New-AssetReceipt $target $asset
    }
    Assert-SelectedAssetReceipts $receipts $temp | Out-Null
    $badSize = $receipts[0] | ConvertFrom-Json
    $badSize.size++
    Assert-Throws {
        Assert-SelectedAssetReceipts @(($badSize | ConvertTo-Json -Compress), $receipts[1], $receipts[2]) $temp
    } 'Receipt validation accepted a mismatched asset size'
    $badTarget = $receipts[0] | ConvertFrom-Json
    $badTarget.target = 'windows-arm64'
    Assert-Throws {
        Assert-SelectedAssetReceipts @(($badTarget | ConvertTo-Json -Compress), $receipts[1], $receipts[2]) $temp
    } 'Receipt validation accepted a duplicated/misbound target'
    $badHash = $receipts[0] | ConvertFrom-Json
    $badHash.sha256 = '0' * 64
    Assert-Throws {
        Assert-SelectedAssetReceipts @(($badHash | ConvertTo-Json -Compress), $receipts[1], $receipts[2]) $temp
    } 'Receipt validation accepted a mismatched asset SHA256'
    $Arm64AcceptanceSHA256 = [string]($assetMetadata | Where-Object Name -eq 'Clouvia-windows-arm64-installer.exe').SHA256
    $Arm64AcceptanceConfirmation = "ACCEPT $($identity.Tag) ARM64"
    Assert-Arm64Acceptance $identity $assetMetadata
    $Arm64AcceptanceSHA256 = '0' * 64
    Assert-Throws {
        Assert-Arm64Acceptance $identity $assetMetadata
    } 'ARM acceptance must be bound to the exact Draft asset hash'
    $Arm64AcceptanceSHA256 = [string]($assetMetadata | Where-Object Name -eq 'Clouvia-windows-arm64-installer.exe').SHA256
    $Arm64AcceptanceConfirmation = "ACCEPT desktop-v9.9.9 ARM64"
    Assert-Throws {
        Assert-Arm64Acceptance $identity $assetMetadata
    } 'ARM acceptance must be bound to the exact release tag'
    $manifest = [ordered]@{ version = $identity.Version; platforms = $platforms }
    $manifestPath = Join-Path $temp 'latest.json'
    $manifest | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $manifestPath -Encoding utf8
    Assert-OssManifestMatchesReleaseAssets $identity $manifestPath $temp

    $manifest.platforms['windows-arm64'].sha256 = '0' * 64
    $manifest | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $manifestPath -Encoding utf8
    Assert-Throws {
        Assert-OssManifestMatchesReleaseAssets $identity $manifestPath $temp
    } 'cloud-only must reject an OSS manifest SHA not bound to the GitHub asset'

    $manifest.platforms['windows-arm64'].sha256 = Get-LowerSHA256 (Join-Path $temp 'Clouvia-windows-arm64-installer.exe')
    $manifest.platforms['windows-amd64'].url += '?changed=1'
    $manifest | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $manifestPath -Encoding utf8
    Assert-Throws {
        Assert-OssManifestMatchesReleaseAssets $identity $manifestPath $temp
    } 'cloud-only must reject a non-exact OSS asset URL'

    $manifest.platforms['windows-amd64'].url = "$OssPublicBaseURL/releases/$($identity.Tag)/$([uri]::EscapeDataString('Clouvia-windows-amd64-installer.exe'))"
    $manifest.platforms['windows-amd64'].size++
    $manifest | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $manifestPath -Encoding utf8
    Assert-Throws {
        Assert-OssManifestMatchesReleaseAssets $identity $manifestPath $temp
    } 'cloud-only must reject an OSS manifest size not bound to the GitHub asset'

    $candidate = Join-Path $temp 'candidate-stable.json'
    $current = Join-Path $temp 'current-stable.json'
    [IO.File]::WriteAllText($candidate, '{"version":"v1.0.11","platforms":{}}', [Text.UTF8Encoding]::new($false))
    Copy-Item -LiteralPath $candidate -Destination $current -Force
    Assert-StablePointerTransition $identity $candidate $current

    [IO.File]::WriteAllText($current, '{"version":"v1.0.11","platforms":{"changed":{}}}', [Text.UTF8Encoding]::new($false))
    Assert-Throws {
        Assert-StablePointerTransition $identity $candidate $current
    } 'The same stable version with a different manifest SHA must be rejected'

    [IO.File]::WriteAllText($current, '{"version":"v1.0.12","platforms":{}}', [Text.UTF8Encoding]::new($false))
    Assert-Throws {
        Assert-StablePointerTransition $identity $candidate $current
    } 'Publishing an older version must never move stable backward'

    [IO.File]::WriteAllText($current, '{"version":"v1.0.10","platforms":{}}', [Text.UTF8Encoding]::new($false))
    Assert-StablePointerTransition $identity $candidate $current

    $pointerA = Join-Path $temp 'pointer-a.json'
    $pointerB = Join-Path $temp 'pointer-b.json'
    $pointerGlobal = Join-Path $temp 'pointer-global.json'
    foreach ($path in @($pointerA, $pointerB, $pointerGlobal)) {
        [IO.File]::WriteAllText($path, '{"version":"v1.0.11","platforms":{}}', [Text.UTF8Encoding]::new($false))
    }
    $script:SelectedTargets = @('windows-amd64', 'windows-arm64')
    $Bootstrap = $true
    $pointerFixtures = @{ 'windows-amd64' = $pointerA; 'windows-arm64' = $pointerB }
    $script:LatestWriteCount = 0
    $script:PointerWriteCount = 0
    function Invoke-PointerFixture {
        Assert-PointerPublicationPreflight $identity $pointerFixtures $pointerGlobal
        $script:LatestWriteCount++
        $script:PointerWriteCount += 3
    }
    function Assert-PublicStablePointerCanAdvance {
        param($TargetIdentity, $CandidateManifestPath, $Target)
        if ($Target -eq 'windows-arm64') { throw 'second target conflict' }
    }
    Assert-Throws { Invoke-PointerFixture } 'A second target conflict passed the complete pointer preflight'
    if ($script:LatestWriteCount -ne 0 -or $script:PointerWriteCount -ne 0) {
        throw 'A second target conflict mutated Latest or a pointer before complete preflight'
    }
    function Assert-PublicStablePointerCanAdvance {
        param($TargetIdentity, $CandidateManifestPath, $Target)
        if (-not $Target) { throw 'global same-version conflict or downgrade' }
    }
    Assert-Throws { Invoke-PointerFixture } 'A global pointer conflict passed the complete pointer preflight'
    if ($script:LatestWriteCount -ne 0 -or $script:PointerWriteCount -ne 0) {
        throw 'A global pointer conflict mutated Latest or a pointer before complete preflight'
    }
    function Assert-PublicStablePointerCanAdvance { param($TargetIdentity, $CandidateManifestPath, $Target) }
    Invoke-PointerFixture
    if ($script:LatestWriteCount -ne 1 -or $script:PointerWriteCount -ne 3) {
        throw 'A valid complete pointer preflight did not continue to all writes'
    }
    $Bootstrap = $false

    $script:SelectedTargets = @('windows-amd64')
    $selectedAsset = @($assetMetadata | Where-Object Name -eq 'Clouvia-windows-amd64-installer.exe')
    [IO.File]::WriteAllText(($selectedAsset[0].Path + '.minisig'), 'signature', [Text.UTF8Encoding]::new($false))
    $githubManifest = Join-Path $temp 'latest-windows-amd64-github.json'
    $ossManifest = Join-Path $temp 'latest-windows-amd64-oss.json'
    [IO.File]::WriteAllText($githubManifest, '{"fixture":"github"}', [Text.UTF8Encoding]::new($false))
    [IO.File]::WriteAllText($ossManifest, '{"fixture":"oss"}', [Text.UTF8Encoding]::new($false))
    $script:observableWriteCalls = 0
    function Assert-PublicStablePointerCanAdvance { param($TargetIdentity, $CandidateManifestPath, $Target) }
    function Invoke-CloudPreflight { }
    function Get-PublicObjectState {
        param($ObjectKey, $FilePath)
        if ($ObjectKey -like '*/latest.json') { return 'conflict' }
        return 'missing'
    }
    function Assert-OrUploadFinalDraftAssets { $script:observableWriteCalls++ }
    function Publish-OssImmutableObject { $script:observableWriteCalls++ }
    function Publish-OssStablePointer { $script:observableWriteCalls++ }
    function Set-CloudModeInReleaseBody { $script:observableWriteCalls++ }
    $preflightRelease = [pscustomobject]@{ isDraft = $false; assets = @() }
    Assert-Throws {
        Assert-ZeroWritePublicationPreflight $identity $preflightRelease $selectedAsset `
            @{ 'windows-amd64' = $githubManifest } @{ 'windows-amd64' = $ossManifest }
    } 'A downstream OSS conflict must fail the zero-write preflight'
    if ($script:observableWriteCalls -ne 0) {
        throw "Downstream preflight conflict caused $script:observableWriteCalls remote write calls"
    }

    $script:LatestMutationCalls = 0
    $script:LatestVerificationCalls = 0
    function Invoke-Native {
        param([string]$Description, [scriptblock]$Command)
        if ($Description -like 'set bootstrap GitHub Latest*') {
            $script:LatestMutationCalls++
            return ''
        }
        if ($Description -eq 'verify bootstrap GitHub Latest') {
            $script:LatestVerificationCalls++
            return '{"tagName":"desktop-v1.0.11"}'
        }
        throw "Unexpected native call in Latest fixture: $Description"
    }
    Set-GitHubLatestForBootstrap $identity
    if ($script:LatestMutationCalls -ne 1 -or $script:LatestVerificationCalls -ne 1) {
        throw 'Bootstrap final pointer stage did not set and verify GitHub Latest exactly once'
    }

    $Version = '1.0.11'
    $Tag = 'desktop-v1.0.11'
    $SourceCommit = '1' * 40
    $TagObject = '2' * 40
    $Confirmation = 'PUBLISH desktop-v1.0.11 TARGETS windows-amd64'
    $Bootstrap = $false
    $ReleaseState = 'draft-or-published'
    $AllowFinalizedDraft = $true
    $script:SelectedTargets = @('windows-amd64')
    $NotesFile = Join-Path $temp 'notes.md'
    [IO.File]::WriteAllText($NotesFile, "fixture notes`n", [Text.UTF8Encoding]::new($false))
    $stageOutput = Join-Path $temp 'stage-output'
    $ArtifactsDirectory = $stageOutput
    $stageMetadata = @($assetMetadata | Where-Object Name -eq 'Clouvia-windows-amd64-installer.exe')
    $script:StagePublishedRelease = [pscustomobject]@{
        isDraft = $false
        assets = @(
            [pscustomobject]@{ name = 'Clouvia-windows-arm64-installer.exe' },
            [pscustomobject]@{ name = 'Clouvia-windows-arm64-installer.exe.minisig' },
            [pscustomobject]@{ name = 'latest-windows-arm64.json' },
            [pscustomobject]@{ name = 'Clouvia-windows-amd64-installer.exe' }
        )
        body = New-ReleaseBody $identity $SourceCommit 'fixture notes' $stageMetadata $TagObject
    }
    function Get-ReleaseJSON { param($ReleaseTag); return $script:StagePublishedRelease }
    function Invoke-Native {
        param([string]$Description, [scriptblock]$Command)
        if ($Description -like 'download release asset*') {
            Copy-Item -LiteralPath (Join-Path $temp 'Clouvia-windows-amd64-installer.exe') `
                -Destination (Join-Path $ArtifactsDirectory 'Clouvia-windows-amd64-installer.exe')
        }
        return ''
    }
    $outputFile = Join-Path $temp 'github-output.txt'
    $previousOutput = $env:GITHUB_OUTPUT
    $env:GITHUB_OUTPUT = $outputFile
    try { Invoke-ValidateDraft }
    finally { $env:GITHUB_OUTPUT = $previousOutput }
    if (-not (Test-Path -LiteralPath (Join-Path $stageOutput 'Clouvia-windows-amd64-installer.exe'))) {
        throw 'Stage validation did not accept the published same-tag append intermediate state'
    }
    $receiptOutput = Get-Content -Raw -LiteralPath $outputFile
    if ($receiptOutput -notmatch '(?m)^receipt_windows_amd64=\{"target":"windows-amd64","asset_name":"Clouvia-windows-amd64-installer.exe","size":[1-9][0-9]*,"sha256":"[0-9a-f]{64}"\}\r?$') {
        throw 'Stage validation did not emit the complete selected target/name/size/SHA256 receipt'
    }
    Remove-Item -LiteralPath $stageOutput -Recurse -Force
    $ReleaseState = 'published'
    Assert-Throws {
        Invoke-ValidateDraft
    } 'Explicit published validation accepted a release missing selected signature/manifest'
    $ManifestTarget = 'windows-amd64'
    $Confirmation = 'PUBLISH desktop-v1.0.11 TARGETS windows-arm64'
    Assert-Throws {
        Invoke-ValidateOssManifest
    } 'ValidateOssManifest accepted confirmation for a different target set'

    $script:SelectedTargets = @('windows-amd64', 'windows-arm64', 'macos-universal')
    $Confirmation = 'PUBLISH desktop-v1.0.11 TARGETS windows-amd64,windows-arm64,macos-universal'
    $CloudPublisherPath = Join-Path $temp 'publish-cloud-release.exe'
    [IO.File]::WriteAllText($CloudPublisherPath, 'fixture', [Text.UTF8Encoding]::new($false))
    $CloudManifestDirectory = $temp
    $CloudManifestPattern = 'cloud-{target}.json'
    foreach ($target in $script:SelectedTargets) {
        [IO.File]::WriteAllText((Join-Path $temp "cloud-$target.json"), '{}', [Text.UTF8Encoding]::new($false))
    }
    $CloudBaseURL = 'https://cloud.example.test'
    $CloudConcurrency = 3
    function Invoke-CloudPreflight { }

    $script:CloudPhaseCalls = @()
    $script:CloudPhaseExecutor = {
        param([object[]]$Requests, [bool]$Preflight, [int]$ThrottleLimit)
        $script:CloudPhaseCalls += [pscustomobject]@{
            Preflight = $Preflight
            Count = $Requests.Count
            Throttle = $ThrottleLimit
        }
        return @($Requests | ForEach-Object {
            [pscustomobject]@{
                Target = $_.Target
                ExitCode = if ($Preflight -and $_.Target -eq 'windows-arm64') { 23 } else { 0 }
                Output = ''
            }
        })
    }
    Assert-Throws { Invoke-PublishCloudRecords } 'Failed cloud preflight unexpectedly allowed publication'
    if ($script:CloudPhaseCalls.Count -ne 1 -or -not $script:CloudPhaseCalls[0].Preflight -or
        $script:CloudPhaseCalls[0].Count -ne 3 -or $script:CloudPhaseCalls[0].Throttle -ne 3) {
        throw 'Cloud preflight failure did not drain and validate every selected target before stopping'
    }

    $script:CloudPhaseCalls = @()
    $script:CloudPhaseExecutor = {
        param([object[]]$Requests, [bool]$Preflight, [int]$ThrottleLimit)
        $script:CloudPhaseCalls += [pscustomobject]@{ Preflight = $Preflight; Count = $Requests.Count; Throttle = $ThrottleLimit }
        return @($Requests | ForEach-Object {
            [pscustomobject]@{ Target = $_.Target; ExitCode = 0; Output = '' }
        })
    }
    Invoke-PublishCloudRecords
    if ($script:CloudPhaseCalls.Count -ne 2 -or -not $script:CloudPhaseCalls[0].Preflight -or
        $script:CloudPhaseCalls[1].Preflight -or @($script:CloudPhaseCalls | Where-Object Count -ne 3).Count -ne 0) {
        throw 'Cloud publication did not preserve the full preflight barrier before concurrent target publication'
    }
}
finally { Remove-Item -LiteralPath $temp -Recurse -Force -ErrorAction SilentlyContinue }

Write-Host 'desktop Actions Publish static tests passed'

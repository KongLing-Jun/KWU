$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$releaseScript = Join-Path $PSScriptRoot 'desktop-release.ps1'
$parseErrors = $null
$null = [System.Management.Automation.Language.Parser]::ParseFile($releaseScript, [ref]$null, [ref]$parseErrors)
if ($parseErrors.Count -ne 0) {
    throw "desktop-release.ps1 has parse errors: $($parseErrors | Out-String)"
}

$testRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("kwu-release-test-" + [guid]::NewGuid().ToString('N'))
$repo = Join-Path $testRoot 'repo'
$bin = Join-Path $testRoot 'bin'
New-Item -ItemType Directory -Path $repo, $bin | Out-Null
try {
    & git -C $repo init -b main-v3 | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'git init failed' }
    & git -C $repo config user.name 'KWU Test'
    & git -C $repo config user.email 'kwu-test@example.invalid'
    Set-Content -LiteralPath (Join-Path $repo 'README.md') -Value 'test repository'
    New-Item -ItemType Directory -Path (Join-Path $repo 'desktop') | Out-Null
    $notesDirectory = Join-Path $repo 'docs\releases'
    New-Item -ItemType Directory -Path $notesDirectory | Out-Null
    Set-Content -LiteralPath (Join-Path $notesDirectory 'v1.0.11.md') -Value "# Test release`n`n- Fixed updater"
    Set-Content -LiteralPath (Join-Path $notesDirectory 'v1.0.12.md') -Value "# Legacy release`n`n- Recovery test"
    & git -C $repo add README.md docs/releases/v1.0.11.md docs/releases/v1.0.12.md
    & git -C $repo commit -m 'test fixture' | Out-Null
    $origin = Join-Path $testRoot 'origin.git'
    & git init --bare $origin | Out-Null
    & git -C $repo remote add origin $origin
    & git -C $repo push -u origin main-v3 | Out-Null

    foreach ($tool in @('gh', 'go', 'node', 'pnpm', 'wails', 'bash')) {
        $body = if ($tool -eq 'gh') {
            '@echo off' + "`r`n" +
                'echo %*>>"%GH_TEST_LOG%"' + "`r`n" +
                'if "%1"=="api" (echo true& exit /b 0)' + "`r`n" +
                'if "%1 %2"=="release view" (' + "`r`n" +
                '  if "%GH_TEST_DRAFT%"=="1" (' + "`r`n" +
                '    if "%GH_TEST_FINALIZE%"=="1" (' + "`r`n" +
                '      echo {"tagName":"desktop-v1.0.11","isDraft":true,"body":"Source repository: `xiaoxiaotest9527-bit/KWU`\nSource tag: `desktop-v1.0.11`\nSource tag object: `%GH_TEST_TAG_OBJECT%`\nSource commit: `%GH_TEST_COMMIT%`\nRelease tag: `desktop-v1.0.11`\nEmergency: `false`\n\n## \u66f4\u65b0\u8bf4\u660e\n\n# Test release\n\n- Fixed updater","assets":[{"name":"Clouvia-windows-amd64-installer.exe"},{"name":"Clouvia-windows-arm64-installer.exe"},{"name":"Clouvia-darwin-universal.dmg"}]}' + "`r`n" +
                '    ) else (' + "`r`n" +
                '      echo {"tagName":"desktop-v1.0.11","isDraft":true,"body":"Source repository: `xiaoxiaotest9527-bit/KWU`\nSource tag: `desktop-v1.0.11`\nSource tag object: `%GH_TEST_TAG_OBJECT%`\nSource commit: `%GH_TEST_COMMIT%`\nRelease tag: `desktop-v1.0.11`\nEmergency: `false`\n\n## \u66f4\u65b0\u8bf4\u660e\n\n# Test release\n\n- Fixed updater","assets":[]}' + "`r`n" +
                '    )' + "`r`n" +
                '    exit /b 0' + "`r`n" +
                '  )' + "`r`n" +
                '  exit /b 1' + "`r`n" +
                ')' + "`r`n" +
                'exit /b 0'
        } elseif ($tool -eq 'go') {
            '@echo off' + "`r`n" +
                'if "%1"=="build" copy /y "%ComSpec%" "%3" >nul' + "`r`n" +
                'exit /b 0'
        } else {
            '@exit /b 0'
        }
        Set-Content -LiteralPath (Join-Path $bin "$tool.cmd") -Value $body -Encoding Ascii
    }
    $ossLog = Join-Path $testRoot 'ossutil.log'
    $ossutil = Join-Path $bin 'ossutil.cmd'
    Set-Content -LiteralPath $ossutil -Value ('@echo off' + "`r`n" +
        'echo %*>>"%OSSUTIL_TEST_LOG%"' + "`r`n" +
        'if "%1"=="version" (echo 2.3.0& exit 0)' + "`r`n" +
        'if "%1"=="cat" (' + "`r`n" +
        '  if "%OSSUTIL_TEST_MODE%"=="missing" (' + "`r`n" +
        '    echo Http Status Code: 404. 1>&2' + "`r`n" +
        '    echo Error Code: NoSuchKey. 1>&2' + "`r`n" +
        '    exit 2' + "`r`n" +
        '  )' + "`r`n" +
        '  echo {"version":"v1.0.10","platforms":{"windows-amd64":{}}}' + "`r`n" +
        '  exit 0' + "`r`n" +
        ')' + "`r`n" +
        'exit 0') -Encoding Ascii
    $config = Join-Path $testRoot '.ossutilconfig'
    Set-Content -LiteralPath $config -Value "[kwu-release]`nendpoint=https://oss-cn-guangzhou.aliyuncs.com"
    $key = Join-Path $testRoot 'suiyuan.key'
    Set-Content -LiteralPath $key -Value 'encrypted test key'

    $previousPath = $env:PATH
    $previousLog = $env:OSSUTIL_TEST_LOG
    $previousGHLog = $env:GH_TEST_LOG
    try {
        $env:PATH = "$bin;$previousPath"
        $env:OSSUTIL_TEST_LOG = $ossLog
        $env:GH_TEST_LOG = Join-Path $testRoot 'gh.log'
        $output = & $releaseScript -Action Doctor -DryRun -RepositoryRoot $repo -ReleaseRepository 'example/release' `
            -OssutilPath $ossutil -OssutilConfigFile $config -OssutilProfile 'kwu-release' -MinisignPrivateKeyFile $key 6>&1
        if ($LASTEXITCODE -ne 0) { throw "Doctor -DryRun failed: $($output | Out-String)" }
        if (-not (($output | Out-String).Contains('Doctor passed (DryRun)'))) {
            throw "Doctor success marker missing: $($output | Out-String)"
        }
        $env:OSSUTIL_TEST_MODE = 'missing'
        $bootstrapOutput = & $releaseScript -Action Doctor -DryRun -RepositoryRoot $repo -ReleaseRepository 'example/release' `
            -OssutilPath $ossutil -OssutilConfigFile $config -OssutilProfile 'kwu-release' -MinisignPrivateKeyFile $key 6>&1
        if ($LASTEXITCODE -ne 0 -or -not (($bootstrapOutput | Out-String).Contains('stable pointer is absent'))) {
            throw "Doctor did not accept the first-release OSS bootstrap state: $($bootstrapOutput | Out-String)"
        }
        $env:OSSUTIL_TEST_MODE = ''

        $notes = Join-Path $testRoot 'release-notes.md'
        Set-Content -LiteralPath $notes -Value "# Test release`n`n- Fixed updater"
        $wrongNotes = Join-Path $testRoot 'wrong-release-notes.md'
        Set-Content -LiteralPath $wrongNotes -Value "# Wrong release`n`n- Must be rejected"
        $workRoot = Join-Path $testRoot 'release-work'
        $prepareOutput = & $releaseScript -Action Prepare -Version v1.0.11 -ReleaseNotesFile $notes -DryRun `
            -RepositoryRoot $repo -ReleaseWorkRoot $workRoot -ReleaseRepository 'example/release' 6>&1
        if (-not (($prepareOutput | Out-String).Contains('DryRun Prepare'))) {
            throw "Prepare DryRun marker missing: $($prepareOutput | Out-String)"
        }
        if (Test-Path -LiteralPath $workRoot) {
            throw 'Prepare -DryRun created the release work directory'
        }
        $publishedTag = (& git -C $repo ls-remote origin refs/tags/desktop-v1.0.11 | Out-String).Trim()
        if ($publishedTag) {
            throw 'Prepare -DryRun pushed the source tag'
        }

        $wrongNotesRejected = $false
        try {
            & $releaseScript -Action Prepare -Version v1.0.11 -ReleaseNotesFile $wrongNotes -DryRun `
                -RepositoryRoot $repo -ReleaseWorkRoot $workRoot -ReleaseRepository 'example/release' 6>&1 | Out-Null
        }
        catch { $wrongNotesRejected = $_.Exception.Message.Contains('must exactly match') }
        if (-not $wrongNotesRejected) { throw 'Prepare accepted release notes that differ from the source commit' }

        $commit = (& git -C $repo rev-parse HEAD | Out-String).Trim()
        $prepareOutput = & $releaseScript -Action Prepare -Version v1.0.11 -ReleaseNotesFile $notes `
            -RepositoryRoot $repo -ReleaseWorkRoot $workRoot -ReleaseRepository 'example/release' 6>&1
        if (-not (($prepareOutput | Out-String).Contains('Prepared desktop-v1.0.11'))) {
            throw "Prepare did not complete: $($prepareOutput | Out-String)"
        }
        $remoteTagLines = (& git -C $repo ls-remote origin refs/tags/desktop-v1.0.11 'refs/tags/desktop-v1.0.11^{}' | Out-String)
        if (-not $remoteTagLines.Contains('refs/tags/desktop-v1.0.11^{}')) {
            throw 'Prepare did not create and push an annotated canonical source tag'
        }
        $notesSnapshot = Join-Path $workRoot 'desktop-v1.0.11\release-notes.md'
        if (-not (Test-Path -LiteralPath $notesSnapshot -PathType Leaf)) {
            throw 'Prepare did not snapshot source-controlled release notes'
        }
        Remove-Item -LiteralPath $workRoot -Recurse -Force
        Clear-Content -LiteralPath $env:GH_TEST_LOG
        $env:GH_TEST_DRAFT = '1'
        $env:GH_TEST_COMMIT = $commit
        $env:GH_TEST_TAG_OBJECT = (& git -C $repo rev-parse refs/tags/desktop-v1.0.11 | Out-String).Trim()
        $buildOutput = & $releaseScript -Action BuildWindows -Version v1.0.11 -DryRun -RepositoryRoot $repo `
            -ReleaseWorkRoot $workRoot -ReleaseRepository 'example/release' -BashPath (Join-Path $bin 'bash.cmd') 6>&1
        if (-not (($buildOutput | Out-String).Contains('DryRun BuildWindows'))) {
            throw "BuildWindows DryRun marker missing: $($buildOutput | Out-String)"
        }
        if (Test-Path -LiteralPath $workRoot) {
            throw 'BuildWindows -DryRun created the release work directory'
        }

        . $releaseScript
        $buildDefinition = (Get-Command Invoke-BuildWindows).Definition
        foreach ($required in @(
            "KWU_WINDOWS_RELEASE_SIGNING = '1'",
            'windows-amd64.json',
            'windows-arm64.json',
            'desktop-windows-authenticode.ps1',
            'Get-NSISBinDirectory',
            'windows-ready.json',
            'Get-ReadyWindowsStaging',
            "'reuse-frontend'",
            'Wait-SelectedMacDraftAsset'
        )) {
            if (-not $buildDefinition.Contains($required)) {
                throw "BuildWindows signing contract is missing: $required"
            }
        }
        $arm64Build = $buildDefinition.IndexOf("windows/arm64")
        $firstUpload = $buildDefinition.IndexOf('Publish-GitHubDraftAsset')
        if ($arm64Build -lt 0 -or $firstUpload -le $arm64Build) {
            throw 'BuildWindows must finish and verify both architectures before uploading either installer'
        }
        $macBarrier = $buildDefinition.IndexOf('Wait-SelectedMacDraftAsset')
        if ($macBarrier -lt 0 -or $firstUpload -le $macBarrier) {
            throw 'Selected macOS asset provenance must be verified before the first Windows upload'
        }
        $macBarrierDefinition = (Get-Command Wait-SelectedMacDraftAsset).Definition
        foreach ($required in @('GitHubAssetDownloader', 'Assert-DraftAssetMetadata', 'Assert-RemoteSourceTagUnchanged')) {
            if (-not $macBarrierDefinition.Contains($required)) {
                throw "Selected macOS asset barrier is missing: $required"
            }
        }
        if (($buildDefinition.Split('Assert-WindowsInstallerArchitecture').Count - 1) -lt 2) {
            throw 'BuildWindows must revalidate signatures, versions and architectures for both installers'
        }
        if (($buildDefinition.Split('Get-DraftRelease $identity -AllowPublished').Count - 1) -lt 3) {
            throw 'BuildWindows cannot continue same-tag target append after the shared Release is published'
        }
        if (-not (Get-Command Get-DraftAssetMetadata).Definition.Contains('SigningEvidenceSHA256')) {
            throw 'Windows Draft provenance must bind the detached signing evidence by SHA256'
        }
        $finalizeDefinition = (Get-Command Invoke-Finalize).Definition
        if ($finalizeDefinition -notmatch "Assert-WindowsInstallerArchitecture\s+\`$amd64\s+'amd64'" -or
            $finalizeDefinition -notmatch "Assert-WindowsInstallerArchitecture\s+\`$arm64\s+'arm64'") {
            throw 'Finalize does not revalidate both downloaded NSIS application payloads'
        }
        if ($finalizeDefinition -match 'Assert-PEArchitecture') {
            throw 'Finalize still inspects the NSIS launcher PE architecture'
        }
        if (($finalizeDefinition.Split('Assert-RemoteSourceTagUnchanged').Count - 1) -lt 5) {
            throw 'Finalize does not recheck the frozen remote tag before its irreversible stages'
        }
        if (($finalizeDefinition.Split('cloud release API preflight').Count - 1) -lt 3) {
            throw 'Finalize does not recheck the short-lived cloud token before public release'
        }
        if (-not $finalizeDefinition.Contains('Assert-MacOSDMGArchitecture $dmg $draft $identity.Version')) {
            throw 'Finalize does not verify the macOS DMG or its SHA256-bound Mac provenance'
        }
        if (-not $finalizeDefinition.Contains(
                'Assert-PublicStablePointerCanAdvance $identity $ossManifests[$target] $target'
            )) {
            throw 'Finalize post-immutable monotonic gate does not read the selected target pointer'
        }
        $cloudRowsIndex = $finalizeDefinition.IndexOf('Invoke-LocalCloudPublisherPhase $cloudPublisher $targets $ossManifests $notesPath -Preflight')
        $cloudPublishIndex = $finalizeDefinition.IndexOf('Invoke-LocalCloudPublisherPhase $cloudPublisher $targets $ossManifests $notesPath', $cloudRowsIndex + 1)
        $latestIndex = $finalizeDefinition.IndexOf('Set-LocalGitHubLatestForBootstrap')
        $pointerPreflightIndex = $finalizeDefinition.LastIndexOf('Assert-LocalPointerPublicationPreflight')
        if ($finalizeDefinition -notmatch '--draft=false --latest=false' -or
            $cloudRowsIndex -lt 0 -or $cloudPublishIndex -le $cloudRowsIndex -or $latestIndex -le $cloudPublishIndex) {
            throw 'Finalize must keep GitHub Latest frozen until bootstrap cloud rows succeed'
        }
        if ($finalizeDefinition.IndexOf("'build', '-o', `$cloudPublisher, './cmd/publish-cloud-release'") -lt 0) {
            throw 'Finalize must prebuild the cloud publisher once instead of repeating go run'
        }
        $localCloudExecutor = (Get-Command Invoke-LocalCloudPublisherPhase).Definition
        if ($localCloudExecutor -notmatch 'LocalCloudPhaseExecutor' -or
            $localCloudExecutor -notmatch 'did not return one result for every selected target' -or
            $localCloudExecutor -notmatch 'failed for targets') {
            throw 'Finalize cloud targets must use a fully drained preflight barrier before publication'
        }
        if ($pointerPreflightIndex -lt 0 -or $pointerPreflightIndex -ge $latestIndex) {
            throw 'Finalize must preflight all target and global pointer transitions before GitHub Latest'
        }
        $migrationGateDefinition = (Get-Command Assert-LocalSelectiveMigrationReady).Definition
        if ($migrationGateDefinition -notmatch '(?s)gh release download.*latest\.json' -or
            $migrationGateDefinition -notmatch 'GitHub migration manifest version differs' -or
            $migrationGateDefinition -notmatch 'darwin-arm64') {
            throw 'Local partial publication does not validate the frozen GitHub global manifest'
        }
        $latestHelperDefinition = (Get-Command Set-LocalGitHubLatestForBootstrap).Definition
        if ($latestHelperDefinition -notmatch 'gh release edit.*--latest' -or
            $latestHelperDefinition -notmatch 'gh release view.*--json tagName') {
            throw 'Bootstrap final pointer stage does not set and verify GitHub Latest'
        }
        if (-not $finalizeDefinition.Contains('Assert-LocalFinalizeIntent $identity') -or
            -not $finalizeDefinition.Contains('does not match the explicitly accepted SHA256')) {
            throw 'Finalize does not require exact publication intent and ARM64 acceptance'
        }
        $fixtureAssets = Join-Path $testRoot 'finalize-assets'
        New-Item -ItemType Directory -Path $fixtureAssets | Out-Null
        foreach ($assetName in @(
            'Clouvia-windows-amd64-installer.exe',
            'Clouvia-windows-arm64-installer.exe',
            'Clouvia-darwin-universal.dmg'
        )) {
            [IO.File]::WriteAllText((Join-Path $fixtureAssets $assetName), "fixture-$assetName", [Text.UTF8Encoding]::new($false))
        }
        $finalizeArgs = @{
            Action = 'Finalize'
            Version = 'v1.0.11'
            ReleaseTargets = @('all')
            Bootstrap = $true
            Confirmation = 'PUBLISH desktop-v1.0.11 TARGETS windows-amd64,windows-arm64,macos-universal BOOTSTRAP'
            Arm64AcceptanceConfirmation = 'ACCEPT desktop-v1.0.11 ARM64'
            Arm64AcceptanceSHA256 = Get-LowerSHA256 (Join-Path $fixtureAssets 'Clouvia-windows-arm64-installer.exe')
            DryRun = $true
            RepositoryRoot = $repo
            ReleaseWorkRoot = $workRoot
            ReleaseRepository = 'example/release'
        }
        foreach ($entry in $finalizeArgs.GetEnumerator()) {
            Set-Variable -Name $entry.Key -Value $entry.Value -Scope Script
        }
        $script:FinalizeDryRunWriteCalls = 0
        $script:FinalizeDryRunPreflightCalls = 0
        $script:FinalizeManifestCommands = @{}
        $script:FinalizeFixtureAssets = $fixtureAssets
        $script:GitHubAssetDownloader = {
            param([string]$Tag, [string]$Name, [string]$Directory)
            Copy-Item -LiteralPath (Join-Path $script:FinalizeFixtureAssets $Name) `
                -Destination (Join-Path $Directory $Name)
            return 0
        }
        $script:GitHubAssetUploader = {
            param([string]$Tag, [string]$Path)
            $script:FinalizeDryRunWriteCalls++
            return 0
        }
        function Assert-LocalFinalizeIntent {
            param($Identity)
            return Get-LowerSHA256 (Join-Path $script:FinalizeFixtureAssets 'Clouvia-windows-arm64-installer.exe')
        }
        function Resolve-RemoteSourceTag {
            param($Identity)
            return [pscustomobject]@{
                Tag = $Identity.SourceTag
                ObjectSHA = '2' * 40
                Commit = $script:FinalizeFixtureCommit
            }
        }
        function Get-DraftRelease {
            param($Identity, [switch]$AllowPublished)
            return [pscustomobject]@{
                isDraft = $false
                body = 'fixture'
                assets = @(
                    'Clouvia-windows-amd64-installer.exe',
                    'Clouvia-windows-arm64-installer.exe',
                    'Clouvia-darwin-universal.dmg'
                ) | ForEach-Object { [pscustomobject]@{ name = $_ } }
            }
        }
        function Assert-DraftSource { }
        function Assert-ManualTakeoverProvenance { }
        function Get-ReleaseNotesAtCommit { return "# Test release`n`n- Fixed updater" }
        function Assert-ReleaseBodyCore { }
        function Assert-ManualCloudModeCompatible { return $false }
        function Assert-LocalSelectiveMigrationReady { }
        function Assert-WindowsInstallerArchitecture { }
        function Assert-DraftAssetMetadata { }
        function Assert-MacOSDMGArchitecture { }
        function Assert-PublicStablePointerCanAdvance { }
        function Get-LocalPublicObjectState { return 'missing' }
        function Invoke-DesktopGoCommand {
            param([object[]]$Arguments, [string]$Description)
            if ($Description -like 'minisign *') {
                [IO.File]::WriteAllText("$($Arguments[-1]).minisig", 'signature', [Text.UTF8Encoding]::new($false))
            }
            elseif ($Description -like 'generate *manifest*') {
                $script:FinalizeManifestCommands[$Description] = @($Arguments)
                $outputIndex = [Array]::IndexOf($Arguments, '--output')
                [IO.File]::WriteAllText(
                    [string]$Arguments[$outputIndex + 1],
                    '{"version":"v1.0.11","platforms":{}}',
                    [Text.UTF8Encoding]::new($false)
                )
            }
            elseif ($Description -eq 'cloud release API zero-write preflight') {
                $script:FinalizeDryRunPreflightCalls++
            }
        }
        function Invoke-LocalCloudPublisherCommand {
            param([string]$PublisherPath, [string[]]$Arguments, [string]$Description)
            if ($Description -eq 'cloud release API zero-write preflight') {
                $script:FinalizeDryRunPreflightCalls++
            }
        }
        function Invoke-LocalCloudPublisherPhase {
            param([string]$PublisherPath, [string[]]$Targets, [hashtable]$OssManifests, [string]$NotesPath, [switch]$Preflight)
        }
        function gh {
            if ($args[0] -eq 'release' -and $args[1] -eq 'download') {
                $name = [string]$args[[Array]::IndexOf($args, '--pattern') + 1]
                $directory = [string]$args[[Array]::IndexOf($args, '--dir') + 1]
                Copy-Item -LiteralPath (Join-Path $script:FinalizeFixtureAssets $name) `
                    -Destination (Join-Path $directory $name)
                $global:LASTEXITCODE = 0
                return
            }
            $script:FinalizeDryRunWriteCalls++
            $global:LASTEXITCODE = 0
        }
        $script:FinalizeFixtureCommit = $commit
        $finalizeOutput = Invoke-Finalize 6>&1
        $finalizeText = $finalizeOutput | Out-String
        foreach ($requiredDryRunEvidence in @(
            'DryRun Finalize targets: windows-amd64,windows-arm64,macos-universal',
            'DryRun destination: GitHub desktop-v1.0.11/Clouvia-windows-amd64-installer.exe',
            'DryRun destination: cloud records for macos-universal',
            'DryRun destination: OSS channels/stable/windows-arm64/latest.json (pointer last)',
            'DryRun destination: frozen global GitHub Latest and OSS channels/stable/latest.json',
            'DryRun completed full zero-write idempotency/conflict preflight'
        )) {
            if (-not $finalizeText.Contains($requiredDryRunEvidence)) {
                throw "Local Finalize DryRun evidence is missing: $requiredDryRunEvidence`n$finalizeText"
            }
        }
        if ($script:FinalizeDryRunPreflightCalls -ne 1 -or $script:FinalizeDryRunWriteCalls -ne 0) {
            throw "Bootstrap DryRun preflight/write counts were $script:FinalizeDryRunPreflightCalls/$script:FinalizeDryRunWriteCalls"
        }
        $expectedManifestPlatforms = @{
            'windows-amd64' = @('windows-amd64')
            'windows-arm64' = @('windows-arm64')
            'macos-universal' = @('darwin-amd64', 'darwin-arm64')
        }
        $expectedNotesPath = Join-Path $workRoot 'desktop-v1.0.11\release-notes.md'
        function Assert-ManifestNotesArgument {
            param([string]$Description, [object[]]$Arguments)
            $notesIndexes = @(
                for ($index = 0; $index -lt $Arguments.Count; $index++) {
                    if ([string]$Arguments[$index] -ceq '--notes-file') { $index }
                }
            )
            if ($notesIndexes.Count -ne 1 -or
                $notesIndexes[0] -ge $Arguments.Count - 1 -or
                [string]$Arguments[$notesIndexes[0] + 1] -cne $expectedNotesPath) {
                throw "$Description did not bind the frozen release notes snapshot exactly once"
            }
        }
        foreach ($target in $expectedManifestPlatforms.Keys) {
            foreach ($destination in @('GitHub', 'OSS')) {
                $description = "generate $destination manifest for $target"
                $arguments = @($script:FinalizeManifestCommands[$description])
                Assert-ManifestNotesArgument $description $arguments
                if ($arguments.Count -lt 3 -or
                    [string]$arguments[-3] -cne (Join-Path $workRoot 'desktop-v1.0.11\incoming') -or
                    [string]$arguments[-2] -cne 'v1.0.11' -or
                    [string]$arguments[-1] -cne 'desktop-v1.0.11') {
                    throw "$description omitted the incoming directory or release identity arguments"
                }
                $actualPlatforms = for ($index = 0; $index -lt $arguments.Count - 1; $index++) {
                    if ([string]$arguments[$index] -ceq '--platform') {
                        [string]$arguments[$index + 1]
                    }
                }
                if (($actualPlatforms -join ',') -cne ($expectedManifestPlatforms[$target] -join ',')) {
                    throw "$description received unexpected platform arguments: $($actualPlatforms -join ',')"
                }
            }
        }
        foreach ($description in @(
            'generate bootstrap GitHub manifest',
            'generate bootstrap OSS manifest'
        )) {
            $arguments = @($script:FinalizeManifestCommands[$description])
            Assert-ManifestNotesArgument $description $arguments
        }
        $ReleaseTargets = @('windows-amd64')
        $Bootstrap = $false
        $Confirmation = 'PUBLISH desktop-v1.0.11 TARGETS windows-amd64'
        $script:FinalizeDryRunWriteCalls = 0
        $script:FinalizeDryRunPreflightCalls = 0
        function Get-LocalPublicObjectState { return 'missing' }
        $partialAppendOutput = Invoke-Finalize 6>&1 | Out-String
        if (-not $partialAppendOutput.Contains('DryRun Finalize targets: windows-amd64') -or
            $script:FinalizeDryRunPreflightCalls -ne 1 -or
            $script:FinalizeDryRunWriteCalls -ne 0) {
            throw 'Published ARM/shared Release could not enter AMD64 signature/manifest preflight safely'
        }
        $script:FinalizeDryRunWriteCalls = 0
        $script:FinalizeDryRunPreflightCalls = 0
        function Get-LocalPublicObjectState { return 'conflict' }
        $publishedAppendConflictRejected = $false
        try { Invoke-Finalize 6>&1 | Out-Null }
        catch { $publishedAppendConflictRejected = $_.Exception.Message.Contains('different content') }
        if (-not $publishedAppendConflictRejected -or $script:FinalizeDryRunWriteCalls -ne 0) {
            throw 'Published same-tag append did not reject a downstream conflict before remote writes'
        }
        . $releaseScript
        $cloudTokenLeaf = 'cloud-token-path-marker.dpapi'
        $cloudTokenFile = Join-Path $testRoot $cloudTokenLeaf
        $syntheticCloudToken = 'synthetic-cloud-token-for-cleanup-test'
        $testCloudToken = ConvertTo-SecureString $syntheticCloudToken -AsPlainText -Force
        $encryptedCloudToken = ConvertFrom-SecureString $testCloudToken
        $originalFinalize = (Get-Command Invoke-Finalize).ScriptBlock
        $originalCloudTokenReader = $script:CloudTokenDPAPIFileReader
        $originalCloudTokenRemover = $script:CloudTokenDPAPIFileRemover
        $previousCloudToken = $env:KWU_CLOUD_RELEASE_TOKEN
        $handoffFailures = @()
        try {
            $env:KWU_CLOUD_RELEASE_TOKEN = 'preserve-existing-token'
            $script:FinalizeInvocationCount = 0
            $script:FinalizeObservedCloudToken = $null
            $script:ExpectedDeletedHandoffPath = $null
            Set-Item -LiteralPath Function:\Invoke-Finalize -Value {
                $script:FinalizeInvocationCount++
                $script:FinalizeObservedCloudToken = $env:KWU_CLOUD_RELEASE_TOKEN
                if ($script:ExpectedDeletedHandoffPath -and
                    (Test-Path -LiteralPath $script:ExpectedDeletedHandoffPath)) {
                    throw 'Finalize observed the DPAPI handoff file before deletion'
                }
            }

            [IO.File]::WriteAllText($cloudTokenFile, $encryptedCloudToken, [Text.UTF8Encoding]::new($false))
            $CloudTokenDPAPIFile = $cloudTokenFile
            $script:ExpectedDeletedHandoffPath = $cloudTokenFile
            Invoke-FinalizeWithCredentialHandoff
            if (Test-Path -LiteralPath $cloudTokenFile) {
                throw 'Finalize did not delete the one-time DPAPI cloud token handoff file'
            }
            if ($script:FinalizeInvocationCount -ne 1 -or
                $script:FinalizeObservedCloudToken -cne $syntheticCloudToken) {
                throw 'Finalize did not receive the decrypted cloud token after handoff cleanup'
            }
            if ($env:KWU_CLOUD_RELEASE_TOKEN -cne 'preserve-existing-token') {
                throw 'Finalize did not restore the caller cloud token environment'
            }

            $script:FinalizeInvocationCount = 0
            $script:ExpectedDeletedHandoffPath = $null
            [IO.File]::WriteAllText($cloudTokenFile, '', [Text.UTF8Encoding]::new($false))
            try {
                Invoke-FinalizeWithCredentialHandoff
                throw 'Finalize accepted an empty DPAPI cloud token handoff file'
            }
            catch {
                $handoffFailures += ($_ | Out-String)
            }
            if ((Test-Path -LiteralPath $cloudTokenFile) -or $script:FinalizeInvocationCount -ne 0) {
                throw 'Empty DPAPI content was not deleted before Finalize was blocked'
            }
            if ($env:KWU_CLOUD_RELEASE_TOKEN -cne 'preserve-existing-token') {
                throw 'Empty DPAPI content did not restore the caller cloud token environment'
            }

            $script:FinalizeInvocationCount = 0
            [IO.File]::WriteAllText($cloudTokenFile, 'synthetic-corrupt-dpapi-content', [Text.UTF8Encoding]::new($false))
            try {
                Invoke-FinalizeWithCredentialHandoff
                throw 'Finalize accepted corrupt DPAPI cloud token content'
            }
            catch {
                $handoffFailures += ($_ | Out-String)
            }
            if ((Test-Path -LiteralPath $cloudTokenFile) -or $script:FinalizeInvocationCount -ne 0) {
                throw 'Corrupt DPAPI content was not deleted before Finalize was blocked'
            }
            if ($env:KWU_CLOUD_RELEASE_TOKEN -cne 'preserve-existing-token') {
                throw 'Corrupt DPAPI content did not restore the caller cloud token environment'
            }

            $script:FinalizeInvocationCount = 0
            $emptyCloudToken = ConvertTo-SecureString '   ' -AsPlainText -Force
            [IO.File]::WriteAllText(
                $cloudTokenFile,
                (ConvertFrom-SecureString $emptyCloudToken),
                [Text.UTF8Encoding]::new($false)
            )
            try {
                Invoke-FinalizeWithCredentialHandoff
                throw 'Finalize accepted DPAPI content that decrypted to an empty token'
            }
            catch {
                $handoffFailures += ($_ | Out-String)
            }
            if ((Test-Path -LiteralPath $cloudTokenFile) -or $script:FinalizeInvocationCount -ne 0) {
                throw 'Empty decrypted DPAPI token was not deleted before Finalize was blocked'
            }
            if ($env:KWU_CLOUD_RELEASE_TOKEN -cne 'preserve-existing-token') {
                throw 'Empty decrypted DPAPI token did not restore the caller cloud token environment'
            }

            $script:FinalizeInvocationCount = 0
            try {
                Invoke-FinalizeWithCredentialHandoff
                throw 'Finalize accepted a missing DPAPI cloud token handoff file'
            }
            catch {
                $handoffFailures += ($_ | Out-String)
            }
            if ($script:FinalizeInvocationCount -ne 0) {
                throw 'Missing DPAPI handoff file entered Finalize'
            }
            if ($env:KWU_CLOUD_RELEASE_TOKEN -cne 'preserve-existing-token') {
                throw 'Missing DPAPI handoff file did not preserve the caller cloud token environment'
            }

            $script:FinalizeInvocationCount = 0
            [IO.File]::WriteAllText($cloudTokenFile, $encryptedCloudToken, [Text.UTF8Encoding]::new($false))
            $script:CloudTokenDPAPIFileReader = {
                param([string]$Path)
                throw "Synthetic DPAPI handoff read failure for $Path"
            }
            try {
                Invoke-FinalizeWithCredentialHandoff
                throw 'Finalize continued after DPAPI handoff read failed'
            }
            catch {
                $handoffFailures += ($_ | Out-String)
            }
            if (-not (Test-Path -LiteralPath $cloudTokenFile) -or $script:FinalizeInvocationCount -ne 0) {
                throw 'DPAPI read failure did not fail closed before Finalize'
            }
            if ($env:KWU_CLOUD_RELEASE_TOKEN -cne 'preserve-existing-token') {
                throw 'DPAPI read failure did not restore the caller cloud token environment'
            }
            $script:CloudTokenDPAPIFileReader = $originalCloudTokenReader
            Remove-Item -LiteralPath $cloudTokenFile -Force

            $script:FinalizeInvocationCount = 0
            [IO.File]::WriteAllText($cloudTokenFile, $encryptedCloudToken, [Text.UTF8Encoding]::new($false))
            $script:CloudTokenDPAPIFileRemover = {
                param([string]$Path)
                throw "Synthetic DPAPI handoff cleanup failure for $Path"
            }
            try {
                Invoke-FinalizeWithCredentialHandoff
                throw 'Finalize continued after DPAPI handoff cleanup failed'
            }
            catch {
                $handoffFailures += ($_ | Out-String)
            }
            if (-not (Test-Path -LiteralPath $cloudTokenFile) -or $script:FinalizeInvocationCount -ne 0) {
                throw 'DPAPI cleanup failure did not fail closed before Finalize'
            }
            if ($env:KWU_CLOUD_RELEASE_TOKEN -cne 'preserve-existing-token') {
                throw 'DPAPI cleanup failure did not restore the caller cloud token environment'
            }
            $script:CloudTokenDPAPIFileRemover = $originalCloudTokenRemover
            Remove-Item -LiteralPath $cloudTokenFile -Force

            $script:FinalizeInvocationCount = 0
            $script:FinalizeObservedCloudToken = $null
            $CloudTokenDPAPIFile = ''
            $env:KWU_CLOUD_RELEASE_TOKEN = 'direct-environment-token'
            $script:CloudTokenDPAPIFileReader = {
                param([string]$Path)
                throw 'Direct environment token path attempted a file read'
            }
            $script:CloudTokenDPAPIFileRemover = {
                param([string]$Path)
                throw 'Direct environment token path attempted a file removal'
            }
            Invoke-FinalizeWithCredentialHandoff
            if ($script:FinalizeInvocationCount -ne 1 -or
                $script:FinalizeObservedCloudToken -cne 'direct-environment-token' -or
                $env:KWU_CLOUD_RELEASE_TOKEN -cne 'direct-environment-token') {
                throw 'Direct environment cloud token behavior changed without a handoff file'
            }

            $failureText = $handoffFailures | Out-String
            foreach ($secretText in @(
                $syntheticCloudToken,
                $encryptedCloudToken,
                'synthetic-corrupt-dpapi-content',
                $cloudTokenFile,
                $cloudTokenLeaf
            )) {
                if ($failureText.Contains($secretText)) {
                    throw 'DPAPI handoff failure output exposed synthetic credential content or path'
                }
            }
        }
        finally {
            Set-Item -LiteralPath Function:\Invoke-Finalize -Value $originalFinalize
            $script:CloudTokenDPAPIFileReader = $originalCloudTokenReader
            $script:CloudTokenDPAPIFileRemover = $originalCloudTokenRemover
            $CloudTokenDPAPIFile = ''
            $env:KWU_CLOUD_RELEASE_TOKEN = $previousCloudToken
            Remove-Item -LiteralPath $cloudTokenFile -Force -ErrorAction SilentlyContinue
        }
        if (-not (Test-Path -LiteralPath (Join-Path $workRoot 'desktop-v1.0.11\manifests') -PathType Container)) {
            throw 'Finalize -DryRun did not prepare manifests required by the full zero-write preflight'
        }
        foreach ($invalid in @(
            @{ Name = 'confirmation'; Key = 'Confirmation'; Value = 'PUBLISH wrong-tag' },
            @{ Name = 'ARM64 confirmation'; Key = 'Arm64AcceptanceConfirmation'; Value = 'ACCEPT wrong-tag ARM64' },
            @{ Name = 'ARM64 SHA256'; Key = 'Arm64AcceptanceSHA256'; Value = 'not-a-sha256' }
        )) {
            $invalidArgs = $finalizeArgs.Clone()
            $invalidArgs[$invalid.Key] = $invalid.Value
            $rejected = $false
            try { $null = & $releaseScript @invalidArgs 6>&1 } catch { $rejected = $true }
            if (-not $rejected) { throw "Finalize accepted invalid $($invalid.Name)" }
        }
        foreach ($forbiddenFinalizeOption in @(
            @{ Name = 'SkipCloud'; Key = 'SkipCloud'; Value = $true },
            @{ Name = 'CloudSkipReason'; Key = 'CloudSkipReason'; Value = 'test skip' },
            @{ Name = 'AllowNonMainV3'; Key = 'AllowNonMainV3'; Value = $true },
            @{ Name = 'EmergencyReason'; Key = 'EmergencyReason'; Value = 'test emergency' }
        )) {
            $forbiddenArgs = $finalizeArgs.Clone()
            $forbiddenArgs[$forbiddenFinalizeOption.Key] = $forbiddenFinalizeOption.Value
            $forbiddenRejected = $false
            try { $null = & $releaseScript @forbiddenArgs 6>&1 } catch { $forbiddenRejected = $true }
            if (-not $forbiddenRejected) {
                throw "Local full Finalize accepted -$($forbiddenFinalizeOption.Name)"
            }
        }
        foreach ($invalid in @(
            @{ Name = 'confirmation'; Key = 'Confirmation'; Value = 'PUBLISH wrong-tag' },
            @{ Name = 'ARM64 confirmation'; Key = 'Arm64AcceptanceConfirmation'; Value = 'ACCEPT wrong-tag ARM64' },
            @{ Name = 'ARM64 SHA256'; Key = 'Arm64AcceptanceSHA256'; Value = 'not-a-sha256' }
        )) {
            $invalidArgs = $finalizeArgs.Clone()
            $invalidArgs[$invalid.Key] = $invalid.Value
            $rejected = $false
            try { $null = & $releaseScript @invalidArgs 6>&1 } catch { $rejected = $true }
            if (-not $rejected) { throw "Finalize accepted invalid $($invalid.Name)" }
        }
        foreach ($forbiddenFinalizeOption in @(
            @{ Name = 'SkipCloud'; Key = 'SkipCloud'; Value = $true },
            @{ Name = 'CloudSkipReason'; Key = 'CloudSkipReason'; Value = 'test skip' },
            @{ Name = 'AllowNonMainV3'; Key = 'AllowNonMainV3'; Value = $true },
            @{ Name = 'EmergencyReason'; Key = 'EmergencyReason'; Value = 'test emergency' }
        )) {
            $forbiddenArgs = $finalizeArgs.Clone()
            $forbiddenArgs[$forbiddenFinalizeOption.Key] = $forbiddenFinalizeOption.Value
            $forbiddenRejected = $false
            try { $null = & $releaseScript @forbiddenArgs 6>&1 } catch { $forbiddenRejected = $true }
            if (-not $forbiddenRejected) {
                throw "Local full Finalize accepted -$($forbiddenFinalizeOption.Name)"
            }
        }

        foreach ($invalidVersion in @('v0.0.0', 'v1.0.0-beta.1', 'v1.1000.0', 'v01.0.0')) {
            $rejected = $false
            try { Get-ReleaseIdentity $invalidVersion | Out-Null } catch { $rejected = $true }
            if (-not $rejected) { throw "Invalid stable version was accepted: $invalidVersion" }
        }
        $nonMainCandidateRejected = $false
        try { Resolve-SourceCommit 'refs/heads/feature/emergency' | Out-Null }
        catch { $nonMainCandidateRejected = $_.Exception.Message.Contains('only freeze the current origin/main-v3') }
        if (-not $nonMainCandidateRejected) {
            throw 'Build/Prepare source resolution retained a non-main-v3 emergency candidate path'
        }
        $identity = Get-ReleaseIdentity '1.0.11'
        if ($identity.SourceTag -ne 'desktop-v1.0.11' -or $identity.LegacySourceTag -ne 'manual-desktop-v1.0.11') {
            throw 'Release identity does not use the canonical tag with legacy read compatibility'
        }
        if ($SourceRepository -ne 'xiaoxiaotest9527-bit/KWU') {
            throw "Unexpected default source repository: $SourceRepository"
        }
        $releaseScriptText = Get-Content -Raw -LiteralPath $releaseScript
        if (-not $releaseScriptText.Contains("else { 'D:\suiyuan-release-key\suiyuan.key' }")) {
            throw 'Manual release script does not default to D:\suiyuan-release-key\suiyuan.key'
        }
        $legacyBody = @"
Source repository: https://github.com/xiaoxiaotest9527-bit/KWU
Source tag: ``manual-desktop-v1.0.12``
Source commit: ``$commit``
Release tag: ``desktop-v1.0.12``

## 更新说明

# Legacy release

- Recovery test
"@
        Assert-ReleaseBodyCore ([pscustomobject]@{ body = $legacyBody }) (Get-ReleaseIdentity v1.0.12) `
            'manual-desktop-v1.0.12' $commit "# Legacy release`n`n- Recovery test"

        $RepositoryRoot = $repo
        & git -C $repo tag -a manual-desktop-v1.0.12 $commit -m 'legacy release test tag'
        & git -C $repo push origin refs/tags/manual-desktop-v1.0.12 | Out-Null
        $legacy = Resolve-RemoteSourceTag (Get-ReleaseIdentity v1.0.12) 3>$null
        if ($legacy.Tag -ne 'manual-desktop-v1.0.12' -or $legacy.Commit -ne $commit) {
            throw 'Legacy annotated source tag was not resolved for takeover'
        }
        $standard = Get-RemoteAnnotatedTag 'desktop-v1.0.11'
        Assert-RemoteSourceTagUnchanged $standard
        $movedTagRejected = $false
        try {
            Assert-RemoteSourceTagUnchanged ([pscustomobject]@{
                Tag = $standard.Tag
                ObjectSHA = '0' * 40
                Commit = $standard.Commit
            })
        }
        catch { $movedTagRejected = $_.Exception.Message.Contains('moved during release') }
        if (-not $movedTagRejected) { throw 'Manual fallback did not reject a changed remote tag object' }

        $auditWorktree = Join-Path $testRoot 'frozen-worktree-audit'
        & git -C $repo worktree add --detach $auditWorktree $commit | Out-Null
        if ($LASTEXITCODE -ne 0) { throw 'Could not create frozen worktree audit fixture' }
        try {
            Assert-BuildWorktreePristine $auditWorktree
            $untracked = Join-Path $auditWorktree 'leftover-source.go'
            Set-Content -LiteralPath $untracked -Value 'package leftover'
            $untrackedRejected = $false
            try { Assert-BuildWorktreePristine $auditWorktree } catch { $untrackedRejected = $true }
            if (-not $untrackedRejected) { throw 'Frozen worktree accepted an untracked source residue' }
            Assert-BuildWorktreeTrackedClean $auditWorktree
            Remove-Item -LiteralPath $untracked -Force

            Add-Content -LiteralPath (Join-Path $auditWorktree 'README.md') -Value 'dirty tracked source'
            $trackedRejected = $false
            try { Assert-BuildWorktreeTrackedClean $auditWorktree } catch { $trackedRejected = $true }
            if (-not $trackedRejected) { throw 'Frozen worktree accepted a tracked source modification' }
        }
        finally {
            & git -C $repo worktree remove --force $auditWorktree | Out-Null
        }
        & git -C $repo tag desktop-v1.0.13 $commit
        & git -C $repo push origin refs/tags/desktop-v1.0.13 | Out-Null
        $lightweightRejected = $false
        try { Resolve-RemoteSourceTag (Get-ReleaseIdentity v1.0.13) | Out-Null } catch { $lightweightRejected = $_.Exception.Message.Contains('must be annotated') }
        if (-not $lightweightRejected) { throw 'Lightweight desktop source tag was not rejected' }

        $localAsset = Join-Path $testRoot 'Clouvia-windows-amd64-installer.exe'
        $remoteAsset = Join-Path $testRoot 'remote-installer.exe'
        Set-Content -LiteralPath $localAsset -Value 'matching installer bytes' -NoNewline
        Copy-Item -LiteralPath $localAsset -Destination $remoteAsset
        $script:DraftAssetUploaded = $false
        $script:GitHubAssetDownloader = {
            param([string]$Tag, [string]$Name, [string]$Directory)
            Copy-Item -LiteralPath $remoteAsset -Destination (Join-Path $Directory $Name)
            return 0
        }
        $script:GitHubAssetUploader = {
            param([string]$Tag, [string]$Path)
            $script:DraftAssetUploaded = $true
            return 0
        }
        $existingAssetDraft = [pscustomobject]@{ assets = @([pscustomobject]@{ name = (Split-Path $localAsset -Leaf) }) }
        Publish-GitHubDraftAsset $identity $existingAssetDraft $localAsset
        if ($script:DraftAssetUploaded) { throw 'Matching existing Draft asset was overwritten' }
        Set-Content -LiteralPath $remoteAsset -Value 'conflicting installer bytes' -NoNewline
        $draftConflictRejected = $false
        try { Publish-GitHubDraftAsset $identity $existingAssetDraft $localAsset } catch { $draftConflictRejected = $true }
        if (-not $draftConflictRejected) { throw 'Conflicting existing Draft asset was not rejected' }
        Publish-GitHubDraftAsset $identity ([pscustomobject]@{ assets = @() }) $localAsset
        if (-not $script:DraftAssetUploaded) { throw 'Missing Draft asset was not uploaded' }

        $metadataDraft = [pscustomobject]@{ body = (New-ReleaseBody $identity $commit "# Test release`n`n- Fixed updater") }
        $localMetadata = Get-DraftAssetMetadata 'Clouvia-windows-amd64-installer.exe' $localAsset 'amd64 payload'
        $evidencePath = Join-Path $testRoot 'windows-amd64-evidence.json'
        $localHash = Get-LowerSHA256 $localAsset
        @(
            [pscustomobject]@{ layer = 'application'; sha256 = '1' * 64; signer_subject = 'CN=Example' },
            [pscustomobject]@{ layer = 'uninstaller'; sha256 = '2' * 64; signer_subject = 'CN=Example' },
            [pscustomobject]@{ layer = 'installer'; sha256 = $localHash; signer_subject = 'CN=Example' }
        ) | ConvertTo-Json | Set-Content -LiteralPath $evidencePath -Encoding utf8
        $signedMetadata = Get-DraftAssetMetadata 'Clouvia-windows-amd64-installer.exe' $localAsset 'amd64 payload' $evidencePath
        $signedLine = Get-DraftAssetMetadataLine $signedMetadata
        if (-not $signedLine.Contains('authenticode=valid timestamp=rfc3161') -or
            -not $signedLine.Contains('signing_evidence_sha256=')) {
            throw 'Signed Windows provenance is missing its Authenticode and evidence binding'
        }
        Set-DraftAssetMetadata $identity $metadataDraft @($localMetadata)
        $conflictingMetadataDraft = [pscustomobject]@{
            body = $metadataDraft.body.Replace('(Pending manual builds.)', '- `Clouvia-windows-amd64-installer.exe`: size=1 sha256=`bad` architecture=amd64 payload')
        }
        $metadataConflictRejected = $false
        try { Set-DraftAssetMetadata $identity $conflictingMetadataDraft @($localMetadata) } catch { $metadataConflictRejected = $true }
        if (-not $metadataConflictRejected) { throw 'Conflicting Draft asset provenance was overwritten' }
        Clear-Content -LiteralPath $env:GH_TEST_LOG

        $fakeDMG = Join-Path $testRoot 'Clouvia-darwin-universal.dmg'
        Set-Content -LiteralPath $fakeDMG -Value 'verified-on-mac fixture bytes' -NoNewline
        $dmgMetadata = Get-DraftAssetMetadata 'Clouvia-darwin-universal.dmg' $fakeDMG 'x86_64+arm64 payload'
        $provenanceDraft = [pscustomobject]@{ body = (Get-DraftAssetMetadataLine $dmgMetadata) }
        Assert-MacOSDMGArchitecture $fakeDMG $provenanceDraft 'v1.0.11'
        $badProvenanceRejected = $false
        try { Assert-MacOSDMGArchitecture $fakeDMG ([pscustomobject]@{ body = 'wrong sha256' }) 'v1.0.11' } catch { $badProvenanceRejected = $true }
        if (-not $badProvenanceRejected) { throw 'Windows fallback accepted an unbound macOS architecture assertion' }

        $missingAssetRejected = $false
        try {
            Assert-RequiredPrimaryAssets ([pscustomobject]@{ assets = @([pscustomobject]@{ name = 'Clouvia-windows-amd64-installer.exe' }) })
        }
        catch { $missingAssetRejected = $true }
        if (-not $missingAssetRejected) { throw 'Missing Draft assets were not rejected' }

        $localObject = Join-Path $testRoot 'local-object.bin'
        $remoteObject = Join-Path $testRoot 'remote-object.bin'
        Set-Content -LiteralPath $localObject -Value 'matching bytes' -NoNewline
        Copy-Item -LiteralPath $localObject -Destination $remoteObject
        $script:SawExactOssPut = $false
        $script:OssPutInvoker = {
            param([object[]]$CommandArguments)
            $joined = $CommandArguments -join ' '
            if ($joined -notmatch '^api put-object\b' -or $joined -match '(?i)\blist\b|delete') {
                throw "Unexpected OSS command: $joined"
            }
            if ($joined -notmatch '--forbid-overwrite') {
                throw "Immutable OSS upload omitted --forbid-overwrite: $joined"
            }
            $script:SawExactOssPut = $true
            return 1
        }
        $script:PublicObjectDownloader = {
            param([string]$URL, [string]$Destination)
            Copy-Item -LiteralPath $remoteObject -Destination $Destination -Force
        }
        Publish-OssImmutableObject 'releases/desktop-v1.0.11/test.bin' $localObject
        Set-Content -LiteralPath $remoteObject -Value 'conflicting bytes' -NoNewline
        $shaConflictRejected = $false
        try { Publish-OssImmutableObject 'releases/desktop-v1.0.11/test.bin' $localObject } catch { $shaConflictRejected = $true }
        if (-not $shaConflictRejected) { throw 'Immutable OSS SHA conflict was not rejected' }
        if (-not $script:SawExactOssPut) { throw 'Immutable upload did not invoke exact OSS PutObject' }

        $candidateManifest = Join-Path $testRoot 'candidate-latest.json'
        $currentManifest = Join-Path $testRoot 'current-latest.json'
        Set-Content -LiteralPath $candidateManifest -Value '{"version":"v1.0.11","platforms":{}}' -NoNewline
        Copy-Item -LiteralPath $candidateManifest -Destination $currentManifest -Force
        Assert-StablePointerTransition $identity $candidateManifest $currentManifest
        Set-Content -LiteralPath $currentManifest -Value '{"version":"v1.0.12","platforms":{}}' -NoNewline
        $rollbackRejected = $false
        try { Assert-StablePointerTransition $identity $candidateManifest $currentManifest } catch { $rollbackRejected = $true }
        if (-not $rollbackRejected) { throw 'Manual fallback allowed the OSS stable pointer to move backward' }
        Set-Content -LiteralPath $currentManifest -Value '{"version":"v1.0.11","platforms":{"different":{}}}' -NoNewline
        $sameVersionConflictRejected = $false
        try { Assert-StablePointerTransition $identity $candidateManifest $currentManifest } catch { $sameVersionConflictRejected = $true }
        if (-not $sameVersionConflictRejected) { throw 'Manual fallback accepted a different manifest for the active version' }

        Set-Content -LiteralPath $currentManifest -Value '{"version":"v1.0.10","platforms":{}}' -NoNewline
        $script:ObservedStablePointerURL = ''
        $script:StablePointerDownloader = {
            param([string]$URL, [string]$Destination)
            $script:ObservedStablePointerURL = $URL
            Copy-Item -LiteralPath $currentManifest -Destination $Destination -Force
        }
        Assert-PublicStablePointerCanAdvance $identity $candidateManifest 'windows-amd64'
        if ($script:ObservedStablePointerURL -notmatch '/channels/stable/windows-amd64/latest\.json\?' -or
            $script:ObservedStablePointerURL -match '/channels/stable/latest\.json\?') {
            throw "Target monotonic gate read the wrong stable pointer: $script:ObservedStablePointerURL"
        }

        Assert-LocalBootstrapStateTransition 'desktop-v2.0.0' 'desktop-v1.0.0' $false 'desktop-v1.0.0' @()
        Assert-LocalBootstrapStateTransition 'desktop-v2.0.0' 'desktop-v1.0.0' $false 'desktop-v2.0.0' @('desktop-v2.0.0')
        Assert-LocalBootstrapStateTransition 'desktop-v2.0.0' 'desktop-v2.0.0' $true 'desktop-v2.0.0' @(
            'desktop-v2.0.0', 'desktop-v2.0.0', 'desktop-v2.0.0'
        )
        $secondBootstrapRejected = $false
        try {
            Assert-LocalBootstrapStateTransition 'desktop-v2.0.0' 'desktop-v1.0.0' $true 'desktop-v2.0.0' @()
        }
        catch { $secondBootstrapRejected = $true }
        if (-not $secondBootstrapRejected) {
            throw 'Local Finalize accepted a new bootstrap after another migration completed'
        }
        Assert-LocalBootstrapStateTransition `
            'desktop-v1.0.0' 'desktop-v1.0.0' $true 'desktop-v2.0.0' @('desktop-v1.0.0')

        $Bootstrap = $false
        $Version = '2.0.0'
        $frozenGlobalFixture = [pscustomobject]@{
            version = 'v1.0.0'
            platforms = [pscustomobject]@{
                'windows-amd64' = [pscustomobject]@{}
                'windows-arm64' = [pscustomobject]@{}
                'darwin-amd64' = [pscustomobject]@{}
                'darwin-arm64' = [pscustomobject]@{}
            }
        }
        $frozenGitHubManifest = Join-Path $testRoot 'frozen-github-latest.json'
        $frozenGlobalFixture | ConvertTo-Json -Depth 5 |
            Set-Content -LiteralPath $frozenGitHubManifest -Encoding utf8
        function Invoke-RestMethod {
            param([string]$Uri, [int]$TimeoutSec)
            return $frozenGlobalFixture
        }
        function gh {
            if ($args[0] -eq 'release' -and $args[1] -eq 'download') {
                $directory = [string]$args[[Array]::IndexOf($args, '--dir') + 1]
                Copy-Item -LiteralPath $frozenGitHubManifest -Destination (Join-Path $directory 'latest.json')
                $global:LASTEXITCODE = 0
                return
            }
            if ($args[0] -eq 'release' -and $args[1] -eq 'view') {
                $global:LASTEXITCODE = 0
                return '{"tagName":"desktop-v1.0.0"}'
            }
            throw "Unexpected gh command in frozen migration fixture: $args"
        }
        Assert-LocalSelectiveMigrationReady
        $frozenGlobalFixture.version = 'v1.0.1'
        $frozenGlobalRejected = $false
        try { Assert-LocalSelectiveMigrationReady }
        catch { $frozenGlobalRejected = $_.Exception.Message.Contains('version differs') }
        if (-not $frozenGlobalRejected) {
            throw 'Local partial publication accepted a GitHub global manifest that differs from OSS'
        }

        $releaseScriptText = Get-Content -Raw -LiteralPath $releaseScript
        foreach ($forbiddenLocalBypass in @(
            'Non-main-v3 releases require both -SourceRef and -AllowNonMainV3',
            'Emergency release takeover requires -AllowNonMainV3 and -EmergencyReason',
            'Mode: skipped'
        )) {
            if ($releaseScriptText.Contains($forbiddenLocalBypass)) {
                throw "Local release tooling retains a forbidden publication path: $forbiddenLocalBypass"
            }
        }
    }
    finally {
        $env:PATH = $previousPath
        $env:OSSUTIL_TEST_LOG = $previousLog
        $env:GH_TEST_LOG = $previousGHLog
    }

    $ossArgs = Get-Content -Raw -LiteralPath $ossLog
    if ($ossArgs -match '(?i)\blist\b|delete') {
        throw "Doctor invoked a forbidden OSS operation: $ossArgs"
    }
    if (-not $ossArgs.Contains('version') -or $ossArgs.Contains('--version')) {
        throw "Doctor did not validate the ossutil executable: $ossArgs"
    }
    if (-not $ossArgs.Contains('--quiet')) { throw "Doctor did not suppress ossutil progress output: $ossArgs" }

    $ghArgs = Get-Content -Raw -LiteralPath (Join-Path $testRoot 'gh.log')
    if ($ghArgs -match '(?im)^release (create|upload|edit|download)\b') {
        throw "DryRun invoked a mutating GitHub command: $ghArgs"
    }

    Write-Host 'desktop-release Doctor, Prepare, BuildWindows, and Finalize DryRun tests passed'
}
finally {
    if (Test-Path -LiteralPath $testRoot) {
        Remove-Item -LiteralPath $testRoot -Recurse -Force
    }
}

$ErrorActionPreference = 'Stop'

function Invoke-Native {
    param(
        [Parameter(Mandatory)][string]$Description,
        [Parameter(Mandatory)][scriptblock]$Command
    )
    $output = & $Command 2>&1
    if ($LASTEXITCODE -ne 0) {
        $message = ($output | Out-String).Trim()
        if ($message) { throw "$Description failed: $message" }
        throw "$Description failed with exit code $LASTEXITCODE"
    }
    return $output
}

function Assert-ReleaseReason {
    param([string]$Value, [string]$Name)
    $reason = $Value.Trim()
    if (-not $reason) { throw "$Name is required" }
    if ($reason.Length -gt 500 -or $reason.Contains("`r") -or $reason.Contains("`n")) {
        throw "$Name must be a single line of at most 500 characters"
    }
    return $reason
}

function Invoke-CloudPreflight {
    if ($CloudMode -eq 'skip') {
        $reason = Assert-ReleaseReason $CloudSkipReason 'cloud skip reason'
        Write-Host "Cloud publication explicitly skipped: $reason"
        return
    }
    if (-not $CloudBaseURL.Trim()) { throw 'Cloud base URL is required when cloud_mode=required' }
    $uri = $null
    if (-not [Uri]::TryCreate($CloudBaseURL.TrimEnd('/'), [UriKind]::Absolute, [ref]$uri) -or $uri.Scheme -ne 'https') {
        throw 'Cloud base URL must be an absolute HTTPS URL'
    }
    if (-not $env:KWU_CLOUD_RELEASE_TOKEN) {
        throw 'KWU_CLOUD_RELEASE_TOKEN is required when cloud_mode=required'
    }
    Push-Location (Join-Path $PSScriptRoot '..\desktop')
    try {
        Invoke-Native 'run cloud release API preflight' {
            go run ./cmd/publish-cloud-release -preflight -base-url $CloudBaseURL
        } | Out-Null
    }
    finally { Pop-Location }
    Write-Host 'Cloud release API preflight passed'
}

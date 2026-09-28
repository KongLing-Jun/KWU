$ErrorActionPreference = 'Stop'

function Resolve-DesktopStablePointerProbe {
    param(
        [Parameter(Mandatory)][int]$ExitCode,
        [AllowEmptyString()][string]$Output
    )
    $text = $Output.Trim()
    if ($ExitCode -eq 0) {
        if (-not $text) { throw 'OSS stable pointer read returned an empty body' }
        try { $pointer = $text | ConvertFrom-Json }
        catch { throw "OSS stable pointer is not valid JSON: $($_.Exception.Message)" }
        $properties = @($pointer.PSObject.Properties.Name)
        if (-not $properties.Contains('version') -or
            [string]::IsNullOrWhiteSpace([string]$pointer.version) -or
            -not $properties.Contains('platforms') -or
            $null -eq $pointer.platforms) {
            throw 'OSS stable pointer does not match the desktop manifest shape'
        }
        return [pscustomobject]@{ Present = $true; Manifest = $pointer }
    }

    $missingObject = $text -match '(?i)Http Status Code:\s*404\b' -and
        $text -match '(?i)Error Code:\s*NoSuchKey\b'
    if ($missingObject) {
        return [pscustomobject]@{ Present = $false; Manifest = $null }
    }
    if (-not $text) { $text = 'no error output' }
    throw "read exact OSS stable pointer failed with exit code $ExitCode`: $text"
}

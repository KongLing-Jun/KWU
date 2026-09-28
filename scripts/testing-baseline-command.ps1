param(
    [Parameter(Mandatory)]
    [string]$SpecPath
)

$ErrorActionPreference = 'Stop'
$spec = Get-Content -LiteralPath $SpecPath -Encoding utf8 -Raw | ConvertFrom-Json
Set-Location -LiteralPath $spec.working_directory

& $spec.file @($spec.arguments)
$exitCode = $LASTEXITCODE
if ($null -eq $exitCode) {
    $exitCode = if ($?) { 0 } else { 1 }
}

[IO.File]::WriteAllText($spec.exit_code_path, [string]$exitCode, [Text.Encoding]::ASCII)
exit $exitCode

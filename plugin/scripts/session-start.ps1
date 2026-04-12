$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$bash = Get-Command bash -ErrorAction SilentlyContinue
if (-not $bash) {
    Write-Error "negent Copilot hooks require bash on PATH"
    exit 127
}

& $bash.Source (Join-Path $scriptDir "session-start.sh")
exit $LASTEXITCODE

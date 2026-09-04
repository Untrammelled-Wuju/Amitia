$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RuntimeRoot = Split-Path -Parent $ScriptDir
$Builder = Join-Path $RuntimeRoot "build\node\linux-arm64\build.py"

& python3 $Builder @args
exit $LASTEXITCODE

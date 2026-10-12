$ErrorActionPreference = "Stop"
$taskRoot = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$taskOutput = Join-Path $taskRoot 'artifacts/cloud-mode-test'
$taskCore = Join-Path $taskOutput 'AmitiaCore.exe'
$taskNode = Join-Path $taskRoot 'desktop/resources/core/node/node.exe'
foreach ($taskProfile in @('cloud-core', 'device-agent')) {
    $taskName = if ($taskProfile -eq 'cloud-core') { 'cloud' } else { 'agent' }
    $taskRuntime = Join-Path $taskOutput $taskName
    $env:AMITIA_RUNTIME_ROOT = $taskRuntime
    $env:AMITIA_CONFIG_DIR = Join-Path $taskRuntime 'config'
    $env:AMITIA_DATA_DIR = Join-Path $taskRuntime 'data'
    $env:AMITIA_WORKSPACE_DIR = Join-Path $taskRuntime 'workspace'
    $env:AMITIA_NODE_BIN = $taskNode
    $env:AMITIA_RUN_MODE = 'server'
    $taskProcess = Start-Process -FilePath $taskCore -ArgumentList "--runtime-profile=$taskProfile" -WorkingDirectory $taskRoot -WindowStyle Hidden -RedirectStandardOutput (Join-Path $taskRuntime 'stdout-current.log') -RedirectStandardError (Join-Path $taskRuntime 'stderr-current.log') -PassThru
    Write-Output "$taskProfile PID $($taskProcess.Id)"
}

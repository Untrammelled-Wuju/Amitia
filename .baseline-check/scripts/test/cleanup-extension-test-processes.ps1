param(
    [switch]$Force
)

$ErrorActionPreference = "Continue"

Write-Host "=== Cleanup Extension Test Processes ===" -ForegroundColor Cyan

$patterns = @(
    "go",
    "amitiacore",
    "AmitiaCore",
    "surreal",
    "qdrant",
    "node.*vitest",
    "node.*playwright",
    "electron"
)

$killed = 0

foreach ($pattern in $patterns) {
    $procs = Get-Process -Name $pattern -ErrorAction SilentlyContinue
    foreach ($proc in $procs) {
        $isTest = $false

        if ($proc.MainModule) {
            $path = $proc.MainModule.FileName
            if ($path -match "test|temp|__test__|vitest") {
                $isTest = $true
            }
        }

        if ($isTest -or $Force) {
            Write-Host "Stopping: $($proc.ProcessName) (PID: $($proc.Id))" -ForegroundColor Yellow
            Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
            $killed++
        }
    }
}

if ($killed -eq 0) {
    Write-Host "No test processes found." -ForegroundColor Green
} else {
    Write-Host "Stopped $killed test processes." -ForegroundColor Green
}

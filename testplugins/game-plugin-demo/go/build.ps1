# Build script for Mock Game Plugin (Go)
# This script builds the Go Mock Plugin as a standalone executable

param(
    [string]$OutputName = "mock-game-plugin",
    [string]$OutputPath = "."
)

$ErrorActionPreference = "Stop"

Write-Host "Building Mock Game Plugin (Go)..." -ForegroundColor Cyan

# Ensure we're in the right directory
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $scriptDir

# Tidy modules
Write-Host "Running go mod tidy..." -ForegroundColor Yellow
& "C:\Code\Go\bin\go.exe" mod tidy
if ($LASTEXITCODE -ne 0) {
    Write-Error "go mod tidy failed"
    exit 1
}

# Build
Write-Host "Building..." -ForegroundColor Yellow
$outputFile = Join-Path $OutputPath $OutputName
& "C:\Code\Go\bin\go.exe" build -o $outputFile ./cmd/mock-game-plugin
if ($LASTEXITCODE -ne 0) {
    Write-Error "Build failed"
    exit 1
}

$builtFile = Get-Item $outputFile
Write-Host "Build successful!" -ForegroundColor Green
Write-Host "Output: $($builtFile.FullName)" -ForegroundColor Green
Write-Host "Size: $($builtFile.Length) bytes" -ForegroundColor Green

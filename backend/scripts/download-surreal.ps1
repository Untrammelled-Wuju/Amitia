# SPDX-FileCopyrightText: 2026 彭旭
# SPDX-License-Identifier: AGPL-3.0-only
# SurrealDB 二进制下载脚本 (Windows)
# 用法: .\scripts\download-surreal.ps1
# 部署时需将 backend\surrealdb\surreal.exe 复制到 release\surrealdb\

param(
    [string]$Version = "2.2.2"
)

$ErrorActionPreference = "Stop"

$rootDir = Split-Path -Parent $PSScriptRoot
$surrealDir = Join-Path $rootDir "surrealdb"
New-Item -ItemType Directory -Force -Path $surrealDir | Out-Null

$exePath = Join-Path $surrealDir "surreal.exe"

if (Test-Path $exePath) {
    Write-Host "surreal.exe 已存在: $exePath" -ForegroundColor Green
    Write-Host "如需重新下载请先删除该文件" -ForegroundColor Yellow
    exit 0
}

$url = "https://github.com/surrealdb/surrealdb/releases/download/v$Version/surreal-v$Version-windows-amd64.zip"
$zipPath = Join-Path $surrealDir "surreal.zip"

Write-Host "下载 SurrealDB v$Version for Windows..." -ForegroundColor Cyan
Write-Host "URL: $url" -ForegroundColor Gray

try {
    Invoke-WebRequest -Uri $url -OutFile $zipPath -ErrorAction Stop
    Expand-Archive -Path $zipPath -DestinationPath $surrealDir -Force
    Remove-Item $zipPath
    Write-Host "SurrealDB 下载完成: $exePath" -ForegroundColor Green
} catch {
    Write-Host "下载失败: $_" -ForegroundColor Red
    Write-Host "请手动下载 SurrealDB 放到 $surrealDir 目录" -ForegroundColor Yellow
    Write-Host "下载地址: $url" -ForegroundColor Yellow
    exit 1
}

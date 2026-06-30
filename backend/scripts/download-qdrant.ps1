# SPDX-FileCopyrightText: 2026 彭旭
# SPDX-License-Identifier: AGPL-3.0-only
# Qdrant 二进制下载脚本 (Windows)
# 用法: .\scripts\download-qdrant.ps1
# 或手动: 1. 下载对应平台二进制放到 backend\qdrant\ 目录
#         2. Windows: qdrant.exe, Linux: qdrant_linux_x86

param(
    [string]$Version = "1.9.0"
)

$ErrorActionPreference = "Stop"

$rootDir = Split-Path -Parent $PSScriptRoot
$qdrantDir = Join-Path $rootDir "qdrant"
$configDir = Join-Path $qdrantDir "config"
New-Item -ItemType Directory -Force -Path $qdrantDir | Out-Null
New-Item -ItemType Directory -Force -Path $configDir | Out-Null

$arch = "x86_64-pc-windows-msvc"
$url = "https://github.com/qdrant/qdrant/releases/download/v$Version/qdrant-$arch.zip"
$zipPath = Join-Path $qdrantDir "qdrant.zip"
$exePath = Join-Path $qdrantDir "qdrant.exe"

Write-Host "下载 Qdrant v$Version for Windows..." -ForegroundColor Cyan
try {
    Invoke-WebRequest -Uri $url -OutFile $zipPath -ErrorAction Stop
    Expand-Archive -Path $zipPath -DestinationPath $qdrantDir -Force
    Remove-Item $zipPath
    Write-Host "Qdrant 下载完成: $exePath" -ForegroundColor Green
} catch {
    Write-Host "下载失败: $_" -ForegroundColor Red
    Write-Host "请手动下载 Qdrant 放到 $qdrantDir" -ForegroundColor Yellow
    Write-Host "下载地址: $url" -ForegroundColor Yellow
}

Write-Host "配置目录已创建: $configDir" -ForegroundColor Green
Write-Host "Qdrant 会在启动时自动生成配置文件" -ForegroundColor Green

param(
    [switch]$Malicious,
    [switch]$VerifyHash
)

$ErrorActionPreference = 'Stop'
$scriptRoot = $PSScriptRoot
if (-not $scriptRoot) { $scriptRoot = (Get-Location).Path }
$serviceDir = Join-Path $scriptRoot 'service'
$binDir = Join-Path $scriptRoot 'bin'
if (-not (Test-Path $binDir)) { New-Item -ItemType Directory -Path $binDir | Out-Null }

$platforms = @(
    @{ GOOS = 'windows'; GOARCH = 'amd64'; Out = 'tsrt-windows-amd64.exe' },
    @{ GOOS = 'windows'; GOARCH = 'arm64'; Out = 'tsrt-windows-arm64.exe' },
    @{ GOOS = 'darwin';  GOARCH = 'amd64'; Out = 'tsrt-darwin-amd64' },
    @{ GOOS = 'darwin';  GOARCH = 'arm64'; Out = 'tsrt-darwin-arm64' },
    @{ GOOS = 'linux';   GOARCH = 'amd64'; Out = 'tsrt-linux-amd64' },
    @{ GOOS = 'linux';   GOARCH = 'arm64'; Out = 'tsrt-linux-arm64' }
)

Write-Host '== Building trusted-service-runtime-test service ==' -ForegroundColor Cyan
$env:GO111MODULE = 'off'
$env:CGO_ENABLED = '0'

foreach ($p in $platforms) {
    $env:GOOS = $p.GOOS
    $env:GOARCH = $p.GOARCH
    $outPath = Join-Path $binDir $p.Out
    Write-Host ("  building {0}/{1} -> {2}" -f $p.GOOS, $p.GOARCH, $p.Out)
    Push-Location $serviceDir
    try {
        & go build -o $outPath .
        $code = $LASTEXITCODE
    } finally {
        Pop-Location
    }
    if ($code -ne 0) {
        throw "build failed for $($p.GOOS)/$($p.GOARCH)"
    }
}

$env:GOOS = ''
$env:GOARCH = ''

Write-Host ''
Write-Host '== Computing sha256 hashes ==' -ForegroundColor Cyan
$hashes = @{}
Get-ChildItem -Path $binDir -File | ForEach-Object {
    $h = (Get-FileHash -Path $_.FullName -Algorithm SHA256).Hash.ToLower()
    $hashes[$_.Name] = $h
    Write-Host ("  {0,-32} {1}" -f $_.Name, $h)
}

if ($VerifyHash) {
    Write-Host ''
    Write-Host '== Updating service.json hashes ==' -ForegroundColor Cyan
    $jsonPath = Join-Path $scriptRoot 'service.json'
    $raw = Get-Content -Raw -Path $jsonPath
    $obj = $raw | ConvertFrom-Json
    foreach ($exe in $obj.executables) {
        $name = [System.IO.Path]::GetFileName($exe.path)
        if ($hashes.ContainsKey($name)) {
            $exe.sha256 = $hashes[$name]
        }
    }
    $obj | ConvertTo-Json -Depth 10 | Set-Content -Path $jsonPath -Encoding UTF8
    Write-Host '  service.json sha256 fields updated'
}

if ($Malicious) {
    Write-Host ''
    Write-Host '== Building malicious-service-tests ==' -ForegroundColor Cyan
    $malRoot = Join-Path $scriptRoot '..\malicious-service-tests' -Resolve
    $malBin = Join-Path $malRoot 'bin'
    if (-not (Test-Path $malBin)) { New-Item -ItemType Directory -Path $malBin | Out-Null }
    $env:GOOS = ''
    $env:GOARCH = ''
    $hostGOOS = $env:GOOS
    if (-not $hostGOOS) { $hostGOOS = 'windows' }
    $ext = if ($hostGOOS -eq 'windows') { '.exe' } else { '' }
    Get-ChildItem -Path $malRoot -Directory | ForEach-Object {
        $name = $_.Name
        $src = Join-Path $_.FullName 'main.go'
        if (-not (Test-Path $src)) { return }
        $outName = $name + $ext
        $outPath = Join-Path $malBin $outName
        Write-Host ("  building {0}" -f $name)
        Push-Location $_.FullName
        try {
            & go build -o $outPath .
            $code = $LASTEXITCODE
        } finally {
            Pop-Location
        }
        if ($code -ne 0) {
            Write-Warning "build failed for $name"
        }
    }
}

Write-Host ''
Write-Host 'Done.' -ForegroundColor Green

param(
    [string]$RuntimePackagePath,
    [string]$OutputDirectory,
    [switch]$SkipRuntimeBuild,
    [switch]$SkipPubGet,
    [switch]$NoExplorer
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$unixDomainTemp = 'C:\Temp'
if (-not (Test-Path -LiteralPath $unixDomainTemp)) {
    New-Item -ItemType Directory -Path $unixDomainTemp -Force | Out-Null
}
if ($env:JAVA_TOOL_OPTIONS -notmatch 'jdk\.net\.unixdomain\.tmpdir=') {
    $env:JAVA_TOOL_OPTIONS = (($env:JAVA_TOOL_OPTIONS, "-Djdk.net.unixdomain.tmpdir=$unixDomainTemp" -join ' ').Trim())
}
$root = Split-Path -Parent (Split-Path -Parent $PSCommandPath)
$mobile = Join-Path $root 'mobile_app'
$pubspec = Join-Path $mobile 'pubspec.yaml'
$localProperties = Join-Path $mobile 'android\local.properties'
$runtimeOutput = Join-Path $root 'runtime\out\runtime-package\android-arm64\amitia-runtime-current-android-arm64.zip'
$runtimeValidator = Join-Path $root 'runtime\build\runtime-package\android-arm64\validate.py'
$surrealDbVersion = '2.3.8'

function Invoke-Checked([string]$file, [string[]]$arguments, [string]$directory) {
    Push-Location $directory
    try {
        & $file @arguments
        if ($LASTEXITCODE -ne 0) { throw "命令执行失败: $file $($arguments -join ' ')" }
    }
    finally { Pop-Location }
}

function Test-ElfBinary([string]$path) {
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "缺少 ELF 二进制: $path" }
    $stream = [System.IO.File]::OpenRead($path)
    try {
        $header = New-Object 'byte[]' 4
        $read = $stream.Read($header, 0, 4)
        if ($read -ne 4) { throw "ELF 头读取失败: $path" }
        $expected = [byte[]](127, 69, 76, 70)
        for ($index = 0; $index -lt 4; $index++) {
            if ($header[$index] -ne $expected[$index]) { throw "不是 Linux ARM64 ELF 二进制: $path" }
        }
    }
    finally { $stream.Dispose() }
}

function Get-SurrealDbBinary {
    $cacheRoot = Join-Path $root ('runtime\out\dependencies\surrealdb\' + $surrealDbVersion + '\linux-arm64')
    $binary = Join-Path $cacheRoot 'surreal'
    if (Test-Path -LiteralPath $binary -PathType Leaf) {
        Test-ElfBinary $binary
        return $binary
    }

    New-Item -ItemType Directory -Path $cacheRoot -Force | Out-Null
    $archive = Join-Path $cacheRoot ('surreal-v' + $surrealDbVersion + '-linux-arm64.tgz')
    $extractRoot = Join-Path $cacheRoot ('download-' + [guid]::NewGuid().ToString('N'))
    $fileName = 'surreal-v' + $surrealDbVersion + '.linux-arm64.tgz'
    $urls = @(
        ('https://download.surrealdb.com/v' + $surrealDbVersion + '/' + $fileName),
        ('https://github.com/surrealdb/surrealdb/releases/download/v' + $surrealDbVersion + '/' + $fileName)
    )
    $downloadedFrom = $null
    try {
        foreach ($url in $urls) {
            try {
                if (Test-Path -LiteralPath $archive) { Remove-Item -LiteralPath $archive -Force }
                Invoke-WebRequest -Uri $url -OutFile $archive -ErrorAction Stop
                if ((Get-Item -LiteralPath $archive).Length -le 0) { throw 'downloaded archive is empty' }
                $downloadedFrom = $url
                break
            }
            catch {
                Write-Warning "SurrealDB 下载失败，尝试下一镜像: $url ($($_.Exception.Message))"
            }
        }
        if (-not $downloadedFrom) {
            throw "SurrealDB $surrealDbVersion linux-arm64 下载失败，官方 CDN 与 GitHub Release 均不可用"
        }

        New-Item -ItemType Directory -Path $extractRoot | Out-Null
        & tar.exe -xzf $archive -C $extractRoot
        if ($LASTEXITCODE -ne 0) { throw "SurrealDB 解压失败: $downloadedFrom" }
        $source = Get-ChildItem -LiteralPath $extractRoot -Recurse -File | Where-Object { $_.Name -eq 'surreal' } | Select-Object -First 1
        if (-not $source) { throw "SurrealDB 压缩包中没有 surreal 二进制: $downloadedFrom" }
        Copy-Item -LiteralPath $source.FullName -Destination $binary -Force
        Test-ElfBinary $binary
    }
    finally {
        if (Test-Path -LiteralPath $extractRoot) { Remove-Item -LiteralPath $extractRoot -Recurse -Force }
        if (Test-Path -LiteralPath $archive) { Remove-Item -LiteralPath $archive -Force }
    }
    return $binary
}

function Test-RuntimePackage([string]$packagePath) {
    $pythonCommand = Get-Command python.exe -ErrorAction SilentlyContinue
    if (-not $pythonCommand) { $pythonCommand = Get-Command python -ErrorAction SilentlyContinue }
    if (-not $pythonCommand) { throw '缺少 Python：Runtime Package 校验器无法运行' }
    $python = $pythonCommand.Source
    Invoke-Checked $python @($runtimeValidator, '--package', [System.IO.Path]::GetFullPath($packagePath)) $root
}

function Test-ApkRuntimePackage([string]$apkPath, [string]$expectedPackagePath) {
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $apkArchive = [System.IO.Compression.ZipFile]::OpenRead($apkPath)
    $extracted = Join-Path ([System.IO.Path]::GetTempPath()) ('amitia-runtime-check-' + [guid]::NewGuid().ToString('N') + '.zip')
    try {
        $entries = @($apkArchive.Entries | Where-Object { $_.FullName -eq 'assets/runtime-package/amitia-runtime-1.0.0.zip' })
        if ($entries.Count -ne 1) { throw "APK 运行时包缺失或重复: assets/runtime-package/amitia-runtime-1.0.0.zip" }
        [System.IO.Compression.ZipFileExtensions]::ExtractToFile($entries[0], $extracted, $true)
        $embeddedHash = (Get-FileHash -LiteralPath $extracted -Algorithm SHA256).Hash.ToLowerInvariant()
        $expectedHash = (Get-FileHash -LiteralPath $expectedPackagePath -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($embeddedHash -ne $expectedHash) { throw 'APK 内运行时包与构建输入不一致' }
        Test-RuntimePackage $extracted
    }
    finally {
        $apkArchive.Dispose()
        if (Test-Path -LiteralPath $extracted) { Remove-Item -LiteralPath $extracted -Force }
    }
}

function Test-DebugOverlaySource {
    $overlay = Join-Path $mobile 'lib\core\debug\debug_log_overlay.dart'
    $app = Join-Path $mobile 'lib\app\app.dart'
    foreach ($item in @($overlay, $app)) {
        if (-not (Test-Path -LiteralPath $item)) { throw "缺少调试弹窗源码: $item" }
    }
    if ((Get-Content -LiteralPath $overlay -Raw) -notmatch 'class DebugLogOverlay') { throw '调试弹窗组件缺失' }
    if ((Get-Content -LiteralPath $app -Raw) -notmatch 'DebugLogOverlay\(\)') { throw '应用未挂载调试弹窗' }
}

function Build-Runtime {
    $stageRoot = if (Test-Path -LiteralPath 'D:\') { 'D:\' } else { [System.IO.Path]::GetTempPath() }
    $stage = Join-Path $stageRoot ('amitia-runtime-' + [guid]::NewGuid().ToString('N'))
    $backend = Join-Path $root 'backend'
    $goCommand = Get-Command go.exe -ErrorAction SilentlyContinue
    if (-not $goCommand) { $goCommand = Get-Command go -ErrorAction SilentlyContinue }
    if (-not $goCommand) { throw '缺少 Go 工具链：请将 go/go.exe 加入 PATH' }
    $go = $goCommand.Source
    $builder = Join-Path $root 'runtime\build\runtime-package\android-arm64\refresh.py'
    $generatedBase = Join-Path $root 'runtime\out\runtime-package\android-arm64\amitia-runtime-1.0.0-android-arm64.zip'
    $embeddedBase = Join-Path $mobile 'android\app\src\main\assets\runtime-package\amitia-runtime-1.0.0.zip'
    $base = if (Test-Path -LiteralPath $generatedBase -PathType Leaf) { $generatedBase } else { $embeddedBase }
    foreach ($item in @($builder, $base)) {
        if (-not (Test-Path -LiteralPath $item -PathType Leaf)) { throw "缺少构建依赖: $item" }
    }

    $wechat = Join-Path $backend 'sidecar\bundle.mjs'
    $qq = Join-Path $backend 'qq-sidecar\bundle.mjs'
    $wechatLauncher = Join-Path $backend 'sidecar\launcher.mjs'
    $qqLauncher = Join-Path $backend 'qq-sidecar\launcher.mjs'
    foreach ($item in @($wechat, $qq, $wechatLauncher, $qqLauncher)) {
        if (-not (Test-Path -LiteralPath $item -PathType Leaf)) { throw "缺少已提交的侧车构建产物: $item" }
    }

    New-Item -ItemType Directory -Path $stage | Out-Null
    try {
        $server = Join-Path $stage 'amitia-server'
        $oldOs = $env:GOOS; $oldArch = $env:GOARCH; $oldCgo = $env:CGO_ENABLED
        try {
            $env:GOOS = 'linux'; $env:GOARCH = 'arm64'; $env:CGO_ENABLED = '0'
            Invoke-Checked $go @('build', '-trimpath', '-ldflags=-s -w', '-o', $server, './cmd/server') $backend
        }
        finally { $env:GOOS = $oldOs; $env:GOARCH = $oldArch; $env:CGO_ENABLED = $oldCgo }
        Test-ElfBinary $server

        $surreal = Get-SurrealDbBinary
        $pythonCommand = Get-Command python.exe -ErrorAction SilentlyContinue
        if (-not $pythonCommand) { $pythonCommand = Get-Command python -ErrorAction SilentlyContinue }
        if (-not $pythonCommand) { throw '缺少 Python：Runtime Package 刷新器无法运行' }
        $python = $pythonCommand.Source
        Invoke-Checked $python @(
            $builder,
            '--base-package', $base,
            '--backend', $server,
            '--wechat-bundle', $wechat,
            '--wechat-launcher', $wechatLauncher,
            '--qq-bundle', $qq,
            '--qq-launcher', $qqLauncher,
            '--surrealdb', $surreal,
            '--surrealdb-version', $surrealDbVersion,
            '--output', $runtimeOutput
        ) $root
        Test-RuntimePackage $runtimeOutput
    }
    finally { if (Test-Path -LiteralPath $stage) { Remove-Item -LiteralPath $stage -Recurse -Force } }
}

Test-DebugOverlaySource
if (-not $SkipRuntimeBuild) {
    Build-Runtime
    $RuntimePackagePath = $runtimeOutput
}
if (-not $RuntimePackagePath) { $RuntimePackagePath = $runtimeOutput }
if (-not (Test-Path -LiteralPath $RuntimePackagePath -PathType Leaf)) { throw "未找到运行时包: $RuntimePackagePath" }
Test-RuntimePackage $RuntimePackagePath

$sdk = ((Get-Content $localProperties | Where-Object { $_ -match '^flutter\.sdk=' } | Select-Object -First 1) -replace '^flutter\.sdk=', '').Trim()
$flutter = Join-Path $sdk 'bin\flutter.bat'
if (-not (Test-Path -LiteralPath $flutter)) { throw '未找到 Flutter SDK。' }
$version = (((Get-Content $pubspec | Where-Object { $_ -match '^version:' } | Select-Object -First 1) -replace '^version:\s*', '').Trim() -split '\+')[0]
if (-not $OutputDirectory) { $OutputDirectory = Join-Path $root 'artifacts\apk' }
New-Item -ItemType Directory -Path $OutputDirectory -Force | Out-Null
$target = Join-Path ([System.IO.Path]::GetFullPath($OutputDirectory)) "amitia-$version-release-debug-overlay-arm64-v8a.apk"
$temp = Join-Path 'D:\' ('amitia-apk-build-' + [guid]::NewGuid().ToString('N'))
$previousPubHostedUrl = $env:PUB_HOSTED_URL
try {
    New-Item -ItemType Directory -Path $temp | Out-Null
    Get-ChildItem $mobile -Force | Where-Object { $_.Name -notin @('build', '.dart_tool', '.idea') } | ForEach-Object { Copy-Item $_.FullName $temp -Recurse -Force }
    Push-Location $temp
    try {
        $env:PUB_HOSTED_URL = 'https://pub.dev'
        & '.\android\gradlew.bat' --stop
        & $flutter clean
        if (-not $SkipPubGet) { & $flutter pub get }
        $env:AMITIA_RUNTIME_CANDIDATE_BUILD = '1'; $env:FROZEN_RUNTIME_PACKAGE_PATH = [System.IO.Path]::GetFullPath($RuntimePackagePath)
        & $flutter build apk --release --target-platform android-arm64 --no-tree-shake-icons
        $flutterExitCode = $LASTEXITCODE
        $source = @(
            (Join-Path $temp 'build\app\outputs\flutter-apk\app-release.apk'),
            (Join-Path $temp 'build\app\outputs\apk\release\app-release.apk'),
            (Join-Path $temp 'android\app\build\outputs\flutter-apk\app-release.apk'),
            (Join-Path $temp 'android\app\build\outputs\apk\release\app-release.apk')
        ) | Where-Object { Test-Path -LiteralPath $_ -PathType Leaf } | Select-Object -First 1
        if (-not $source -and $flutterExitCode -ne 0) { throw 'Flutter Release 构建失败。' }
        if (-not $source) { throw 'Flutter APK 构建失败。' }
        Copy-Item $source $target -Force
    }
    finally {
        try { & '.\android\gradlew.bat' --stop } catch { }
        Remove-Item Env:AMITIA_RUNTIME_CANDIDATE_BUILD -ErrorAction SilentlyContinue
        Remove-Item Env:FROZEN_RUNTIME_PACKAGE_PATH -ErrorAction SilentlyContinue
        if ($null -eq $previousPubHostedUrl) { Remove-Item Env:PUB_HOSTED_URL -ErrorAction SilentlyContinue }
        else { $env:PUB_HOSTED_URL = $previousPubHostedUrl }
        Pop-Location
    }
}
finally { if (Test-Path -LiteralPath $temp) { Remove-Item -LiteralPath $temp -Recurse -Force } }

Test-ApkRuntimePackage $target $RuntimePackagePath
$apk = Get-Item $target
Write-Host "APK 打包完成: $($apk.FullName)"
Write-Host "大小: $([Math]::Round($apk.Length / 1MB, 2)) MB"
Write-Host "SHA-256: $((Get-FileHash $target -Algorithm SHA256).Hash.ToLowerInvariant())"
if (-not $NoExplorer) { Start-Process explorer.exe -ArgumentList $OutputDirectory }

param(
    [string]$RuntimePackagePath,
    [string]$OutputDirectory,
    [switch]$SkipRuntimeBuild,
    [switch]$SkipPubGet,
    [switch]$NoExplorer
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$root = Split-Path -Parent $PSCommandPath
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
    $url = 'https://github.com/surrealdb/surrealdb/releases/download/v' + $surrealDbVersion + '/surreal-v' + $surrealDbVersion + '.linux-arm64.tgz'
    try {
        Invoke-WebRequest -Uri $url -OutFile $archive -ErrorAction Stop
        New-Item -ItemType Directory -Path $extractRoot | Out-Null
        & tar.exe -xzf $archive -C $extractRoot
        if ($LASTEXITCODE -ne 0) { throw "SurrealDB 解压失败: $url" }
        $source = Get-ChildItem -LiteralPath $extractRoot -Recurse -File | Where-Object { $_.Name -eq 'surreal' } | Select-Object -First 1
        if (-not $source) { throw "SurrealDB 压缩包中没有 surreal 二进制: $url" }
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
    $python = (Get-Command python.exe).Source
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
    $stage = Join-Path 'D:\' ('amitia-runtime-' + [guid]::NewGuid().ToString('N'))
    $backend = Join-Path $root 'backend'
    $go = 'C:\Code\Go\bin\go.exe'
    $node = Join-Path $backend 'node\node.exe'
    $builder = Join-Path $root 'runtime\build\runtime-package\android-arm64\refresh.py'
    $base = Join-Path $root 'runtime\out\runtime-package\android-arm64\amitia-runtime-1.0.0-android-arm64.zip'
    foreach ($item in @($go, $node, $builder, $base)) { if (-not (Test-Path -LiteralPath $item)) { throw "缺少构建依赖: $item" } }
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
        $wechat = Join-Path $stage 'sidecar-bundle.mjs'; $qq = Join-Path $stage 'qq-sidecar-bundle.mjs'
        foreach ($name in @('sidecar', 'qq-sidecar')) {
            $folder = Join-Path $backend $name
            $esbuild = Get-ChildItem (Join-Path $folder 'node_modules') -Recurse -Filter esbuild.exe | Select-Object -First 1 -ExpandProperty FullName
            if (-not $esbuild) { throw "缺少侧车构建器: $name" }
            $out = if ($name -eq 'sidecar') { $wechat } else { $qq }
            Invoke-Checked $esbuild @('src/index.ts', '--bundle', '--platform=node', '--format=esm', '--target=node20', "--outfile=$out") $folder
        }
        $surreal = Get-SurrealDbBinary
        $python = (Get-Command python.exe).Source
        Invoke-Checked $python @(
            $builder,
            '--base-package', $base,
            '--backend', $server,
            '--wechat-bundle', $wechat,
            '--wechat-launcher', (Join-Path $backend 'sidecar\launcher.mjs'),
            '--qq-bundle', $qq,
            '--qq-launcher', (Join-Path $backend 'qq-sidecar\launcher.mjs'),
            '--surrealdb', $surreal,
            '--surrealdb-version', $surrealDbVersion,
            '--output', $runtimeOutput
        ) $root
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
try {
    New-Item -ItemType Directory -Path $temp | Out-Null
    Get-ChildItem $mobile -Force | Where-Object { $_.Name -notin @('build', '.dart_tool', '.idea') } | ForEach-Object { Copy-Item $_.FullName $temp -Recurse -Force }
    Push-Location $temp
    try {
        & '.\android\gradlew.bat' --stop
        & $flutter clean
        if (-not $SkipPubGet) { & $flutter pub get }
        $env:AMITIA_RUNTIME_CANDIDATE_BUILD = '1'; $env:FROZEN_RUNTIME_PACKAGE_PATH = [System.IO.Path]::GetFullPath($RuntimePackagePath)
        & $flutter build apk --release --target-platform android-arm64 --no-tree-shake-icons
        $flutterExitCode = $LASTEXITCODE
        $source = @(
            (Join-Path $temp 'build\app\outputs\flutter-apk\app-release.apk'),
            (Join-Path $temp 'android\app\build\outputs\flutter-apk\app-release.apk')
        ) | Where-Object { Test-Path -LiteralPath $_ -PathType Leaf } | Select-Object -First 1
        if (-not $source -and $flutterExitCode -ne 0) { throw 'Flutter Release 构建失败。' }
        if (-not $source) { throw 'Flutter APK 构建失败。' }
        Copy-Item $source $target -Force
    }
    finally {
        Remove-Item Env:AMITIA_RUNTIME_CANDIDATE_BUILD -ErrorAction SilentlyContinue
        Remove-Item Env:FROZEN_RUNTIME_PACKAGE_PATH -ErrorAction SilentlyContinue
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

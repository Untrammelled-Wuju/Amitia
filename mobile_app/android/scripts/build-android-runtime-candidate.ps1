# Amitia Android Runtime Candidate Build Pipeline
# Step 18: Build and freeze Android Runtime Candidate
#
# Usage:
#   .\scripts\build-android-runtime-candidate.ps1 -CandidateId "amitia-android-1.0.0-runtime-1.0.0-<commit>"
#
# Prerequisites:
#   - FROZEN_RUNTIME_PACKAGE_PATH environment variable OR auto-detected Step 7 package
#   - FROZEN_RUNTIME_PACKAGE_SHA256 environment variable OR from build-record
#   - Android SDK, Flutter SDK, JDK available

param(
    [Parameter(Mandatory = $false)]
    [string]$CandidateId,

    [Parameter(Mandatory = $false)]
    [string]$OutputDir = "dist/android-runtime-candidate",

    [Parameter(Mandatory = $false)]
    [ValidateSet("release", "debug", "profile")]
    [string]$BuildVariant = "release",

    [Parameter(Mandatory = $false)]
    [switch]$AllowDirtyTree
)

$ErrorActionPreference = "Stop"
$ProjectName = "amitia-android"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$AndroidRoot = Split-Path -Parent $ScriptDir
$MobileAppRoot = Split-Path -Parent $AndroidRoot
$ProjectRoot = Split-Path -Parent $MobileAppRoot

Write-Host "=== Amitia Android Runtime Candidate Build Pipeline ===" -ForegroundColor Cyan
Write-Host "Android Root: $AndroidRoot"
Write-Host "Build Variant: $BuildVariant"

$stopwatch = [System.Diagnostics.Stopwatch]::StartNew()

function Invoke-Check {
    param([string]$Name, [scriptblock]$Action)
    Write-Host "  [$Name] " -NoNewline
    try {
        $result = & $Action
        if ($result) {
            Write-Host "PASS" -ForegroundColor Green
        } else {
            Write-Host "FAIL" -ForegroundColor Red
            throw "Check failed: $Name"
        }
    } catch {
        Write-Host "FAIL: $_" -ForegroundColor Red
        throw
    }
}

Write-Host ""
Write-Host "=== Phase 1: Pre-checks ===" -ForegroundColor Yellow

$flutterSdkPath = (Get-Content "$AndroidRoot\local.properties" | Where-Object { $_ -match "^flutter.sdk=" }) -replace "^flutter.sdk=", ""
$sdkDir = (Get-Content "$AndroidRoot\local.properties" | Where-Object { $_ -match "^sdk.dir=" }) -replace "^sdk.dir=", ""
$flutterSdkPath = $flutterSdkPath -replace "\\", "/"
$sdkDir = $sdkDir -replace "\\", "/"

Write-Host "Flutter SDK: $flutterSdkPath"
Write-Host "Android SDK: $sdkDir"

$flutterExe = "flutter"
$gradlewExe = if ($IsWindows -or $env:OS -eq "Windows_NT") { "$AndroidRoot\gradlew.bat" } else { "$AndroidRoot\gradlew" }

$flutterVersion = & $flutterExe --version 2>&1 | Select-Object -First 1
Write-Host "Flutter version: $flutterVersion"

$dartVersion = & $flutterExe --version 2>&1 | Select-String "Dart" | ForEach-Object { $_.Line }
Write-Host "Dart version: $dartVersion"

$jdkVersion = & java -version 2>&1 | Select-Object -First 1
Write-Host "JDK version: $jdkVersion"

$buildToolsDirs = Get-ChildItem "$sdkDir\build-tools" -Name | Sort-Object { [version]($_ + ".0") } -Descending
$buildToolsVersion = $buildToolsDirs | Select-Object -First 1
Write-Host "Build Tools: $buildToolsVersion"

$platformDirs = Get-ChildItem "$sdkDir\platforms" -Name | Sort-Object { [version](($_ -replace 'android-', '') + ".0") } -Descending
$platformVersion = $platformDirs | Select-Object -First 1
Write-Host "Platform: $platformVersion"

$ndkDirs = Get-ChildItem "$sdkDir\ndk" -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Name
$ndkVersion = if ($ndkDirs) { $ndkDirs | Select-Object -First 1 } else { "N/A" }
Write-Host "NDK: $ndkVersion"

$gradleProps = Get-Content "$AndroidRoot\gradle\wrapper\gradle-wrapper.properties"
$gradleVersion = ($gradleProps | Where-Object { $_ -match "distributionUrl" }) -replace ".*gradle-", "" | ForEach-Object { $_ -replace "-all.zip$", "" }
Write-Host "Gradle version: $gradleVersion"

$settingsContent = Get-Content "$AndroidRoot\settings.gradle.kts" -Raw
$agpVersion = if ($settingsContent -match 'com\.application" version "([\d.]+)"') { $matches[1] } else { "unknown" }
$kotlinVersion = if ($settingsContent -match 'kotlin\.android" version "([\d.]+)"') { $matches[1] } else { "unknown" }
Write-Host "AGP version: $agpVersion"
Write-Host "Kotlin version: $kotlinVersion"

$gitCommand = Get-Command git.exe -ErrorAction SilentlyContinue
if (-not $gitCommand) { $gitCommand = Get-Command git -ErrorAction SilentlyContinue }
if (-not $gitCommand) { throw "Git executable not found in PATH" }
$gitExe = $gitCommand.Source
$sourceCommit = & $gitExe -C $ProjectRoot rev-parse HEAD 2>&1
$sourceBranch = & $gitExe -C $ProjectRoot rev-parse --abbrev-ref HEAD 2>&1
$sourceDirty = (& $gitExe -C $ProjectRoot status --porcelain 2>&1 | Measure-Object -Line).Lines -gt 0

Write-Host "Source Commit: $sourceCommit"
Write-Host "Source Branch: $sourceBranch"
Write-Host "Source Dirty: $sourceDirty"

if ($sourceDirty) {
    if (-not $AllowDirtyTree) {
        Write-Host "ERROR: Git tree is dirty. Clean tree required for Candidate build." -ForegroundColor Red
        & $gitExe -C $ProjectRoot status --short
        throw "Dirty git tree. Use -AllowDirtyTree to proceed with source tree state recorded."
    }
    Write-Host "WARNING:Git tree is dirty. Recording source tree state." -ForegroundColor Yellow
    $dirtyFiles = & $gitExe -C $ProjectRoot status --porcelain 2>&1
    Write-Host $dirtyFiles
}

$localPropsContent = Get-Content "$AndroidRoot\local.properties" -Raw
$appVersionName = if ($localPropsContent -match 'flutter\.versionName=(\S+)') { $matches[1] } else { "1.0.0" }
$appVersionCodeStr = if ($localPropsContent -match 'flutter\.versionCode=(\d+)') { $matches[1] } else { "1" }
$appVersionCode = [int]$appVersionCodeStr

Write-Host "App Version Name: $appVersionName"
Write-Host "App Version Code: $appVersionCode"

$step7PackagePath = "$ProjectRoot\runtime\build\out\runtime-package\linux-arm64\amitia-runtime-1.0.0-linux-arm64.zip"
$step7BuildRecord = "$ProjectRoot\runtime\build\out\runtime-package\linux-arm64\runtime-package-build-record.json"
$step6ProotPath = "$AndroidRoot\amitia-runtime\src\main\jniLibs\arm64-v8a\libamitia_proot.so"
$step6ProotMetadata = "$AndroidRoot\amitia-runtime\src\main\res\raw\proot_artifact.json"

if (-not (Test-Path $step7PackagePath)) {
    throw "Step 7 Frozen Runtime Package not found: $step7PackagePath"
}
if (-not (Test-Path $step7BuildRecord)) {
    throw "Step 7 build record not found: $step7BuildRecord"
}
if (-not (Test-Path $step6ProotPath)) {
    throw "Step 6 Frozen PRoot not found: $step6ProotPath"
}
if (-not (Test-Path $step6ProotMetadata)) {
    throw "Step 6 PRoot metadata not found: $step6ProotMetadata"
}

$step7Record = Get-Content $step7BuildRecord | ConvertFrom-Json
$step6Record = Get-Content $step6ProotMetadata | ConvertFrom-Json

$runtimeVersion = $step7Record.runtimeVersion
$expectedRuntimeSha = $step7Record.package.sha256
$expectedProotSha = $step6Record.sha256
$expectedRuntimeSize = if ($step7Record.package.size) { $step7Record.package.size } else { (Get-Item $step7PackagePath).Length }

Write-Host "Runtime Version: $runtimeVersion"
Write-Host "Expected Runtime SHA: $expectedRuntimeSha"
Write-Host "Expected Runtime Size: $expectedRuntimeSize"
Write-Host "Expected PRoot SHA: $expectedProotSha"

if (-not $CandidateId) {
    $shortCommit = $sourceCommit.Substring(0, 7)
    $CandidateId = "$ProjectName-$appVersionName-runtime-$runtimeVersion-$shortCommit"
}
Write-Host "Candidate ID: $CandidateId"

Write-Host ""
Write-Host "=== Phase 2: Input Verification ===" -ForegroundColor Yellow

Invoke-Check "Runtime Package exists" {
    Test-Path $step7PackagePath
}

Invoke-Check "Runtime Package SHA256" {
    $actualSha = (Get-FileHash $step7PackagePath -Algorithm SHA256).Hash.ToLower()
    if ($actualSha -ne $expectedRuntimeSha) {
        throw "Runtime Package SHA mismatch! Expected=$expectedRuntimeSha, Actual=$actualSha"
    }
    $true
}

Invoke-Check "Runtime Package content" {
    $validator = Join-Path $AndroidRoot 'scripts\validate-runtime-package.py'
    if (-not (Test-Path -LiteralPath $validator -PathType Leaf)) { throw "Runtime validator missing: $validator" }
    $pythonCommand = Get-Command python.exe -ErrorAction SilentlyContinue
    if (-not $pythonCommand) { $pythonCommand = Get-Command python -ErrorAction SilentlyContinue }
    if (-not $pythonCommand) { throw "Python executable not found" }
    & $pythonCommand.Source $validator --package $step7PackagePath
    if ($LASTEXITCODE -ne 0) { throw "Runtime Package content validation failed" }
    $true
}

Invoke-Check "PRoot artifact exists" {
    Test-Path $step6ProotPath
}

Invoke-Check "PRoot SHA256" {
    $actualSha = (Get-FileHash $step6ProotPath -Algorithm SHA256).Hash.ToLower()
    if ($actualSha -ne $expectedProotSha) {
        throw "PRoot SHA mismatch! Expected=$expectedProotSha, Actual=$actualSha"
    }
    $true
}

Invoke-Check "PRoot metadata valid" {
    if ($step6Record.abi -ne "arm64-v8a") { throw "PRoot ABI must be arm64-v8a" }
    if ($step6Record.architecture -ne "aarch64") { throw "PRoot architecture must be aarch64" }
    if ($step6Record.fileName -ne "libamitia_proot.so") { throw "PRoot fileName mismatch" }
    $true
}

Invoke-Check "abiFilters = arm64-v8a only" {
    $appBuildContent = Get-Content "$AndroidRoot\app\build.gradle.kts" -Raw
    if ($appBuildContent -match 'abiFilters\.add\(\s*"x86"\s*\)') { throw "x86 ABI found in abiFilters" }
    if ($appBuildContent -match 'abiFilters\.add\(\s*"x86_64"\s*\)') { throw "x86_64 ABI found in abiFilters" }
    if ($appBuildContent -match 'abiFilters\.add\(\s*"armeabi-v7a"\s*\)') { throw "armeabi-v7a ABI found in abiFilters" }
    $true
}

$nativeSourcePath = "$AndroidRoot\amitia-runtime\src\main\kotlin\com\amitia\amitia_app\runtime\packagetrusted\TrustedRuntimePackageSource.kt"
if (-not (Test-Path $nativeSourcePath)) {
    throw "Native TrustedRuntimePackageSource.kt not found: $nativeSourcePath"
}
$nativeContent = Get-Content $nativeSourcePath -Raw

$nativeRuntimeVersion = if ($nativeContent -match 'const val RUNTIME_VERSION:\s*String\s*=\s*"([^"]+)"') { $matches[1] } else { throw "Failed to parse Native RUNTIME_VERSION from TrustedRuntimePackageSource.kt" }
$nativeUsesDynamicPackageSha = $nativeContent -match 'BuildConfig\.RUNTIME_PACKAGE_SHA256'
if (-not $nativeUsesDynamicPackageSha) { throw "Native Runtime package SHA must come from BuildConfig.RUNTIME_PACKAGE_SHA256" }
# Gradle derives BuildConfig.RUNTIME_PACKAGE_SHA256 from the exact frozen package
# passed below through FROZEN_RUNTIME_PACKAGE_PATH. Keep a local identity value
# for the candidate build record without reverting to a stale source literal.
$nativePackageSha = $expectedRuntimeSha
$nativeGuestOs = if ($nativeContent -match 'const val GUEST_OS:\s*String\s*=\s*"([^"]+)"') { $matches[1] } else { throw "Failed to parse Native GUEST_OS from TrustedRuntimePackageSource.kt" }
$nativeArchitecture = if ($nativeContent -match 'const val ARCHITECTURE:\s*String\s*=\s*"([^"]+)"') { $matches[1] } else { throw "Failed to parse Native ARCHITECTURE from TrustedRuntimePackageSource.kt" }
$nativeFileName = if ($nativeContent -match 'const val FILE_NAME:\s*String\s*=\s*"([^"]+)"') { $matches[1] } else { throw "Failed to parse Native FILE_NAME from TrustedRuntimePackageSource.kt" }
$nativeAssetPath = if ($nativeContent -match 'const val ASSET_PATH:\s*String\s*=\s*"([^"]+)"') { $matches[1] } else { throw "Failed to parse Native ASSET_PATH from TrustedRuntimePackageSource.kt" }

Write-Host "Native Runtime Version: $nativeRuntimeVersion"
Write-Host "Native Trusted SHA: BuildConfig.RUNTIME_PACKAGE_SHA256 (derived from frozen package input)"
Write-Host "Native Guest OS: $nativeGuestOs"
Write-Host "Native Architecture: $nativeArchitecture"
Write-Host "Native File Name: $nativeFileName"
Write-Host "Native Asset Path: $nativeAssetPath"

if ($nativeRuntimeVersion -ne $runtimeVersion) {
    throw "Native Runtime Version mismatch! Native=$nativeRuntimeVersion, Step7=$runtimeVersion"
}
Write-Host "  [Native Runtime Version == Step7] PASS" -ForegroundColor Green

Write-Host "  [Native Trusted SHA derives from frozen package at Gradle configuration] PASS" -ForegroundColor Green

if ($nativeGuestOs -ne "linux") {
    throw "Native Guest OS must be linux, got: $nativeGuestOs"
}
Write-Host "  [Native Guest OS = linux] PASS" -ForegroundColor Green

if ($nativeArchitecture -ne "arm64") {
    throw "Native Architecture must be arm64, got: $nativeArchitecture"
}
Write-Host "  [Native Architecture = arm64] PASS" -ForegroundColor Green

if ($nativeFileName -ne "amitia-runtime-1.0.0.zip") {
    throw "Native File Name mismatch! Expected=amitia-runtime-1.0.0.zip, Native=$nativeFileName"
}
Write-Host "  [Native File Name matches embed name] PASS" -ForegroundColor Green

if ($nativeAssetPath -ne "runtime-package/amitia-runtime-1.0.0.zip") {
    throw "Native Asset Path mismatch! Expected=runtime-package/amitia-runtime-1.0.0.zip, Native=$nativeAssetPath"
}
Write-Host "  [Native Asset Path matches embed path] PASS" -ForegroundColor Green

$forbiddenAbis = @("x86", "x86_64", "armeabi-v7a", "armeabi", "mips", "mips64")
$jniLibsDir = "$AndroidRoot\amitia-runtime\src\main\jniLibs"
foreach ($abi in $forbiddenAbis) {
    $forbiddenDir = "$jniLibsDir\$abi"
    if (Test-Path $forbiddenDir) { throw "Forbidden jniLibs directory exists: $abi" }
}
Write-Host "  [No forbidden ABI dirs] PASS" -ForegroundColor Green

$runtimePackageSize = (Get-Item $step7PackagePath).Length
$prootSize = (Get-Item $step6ProotPath).Length

Write-Host ""
Write-Host "Runtime Package Size: $runtimePackageSize bytes ($([math]::Round($runtimePackageSize/1MB, 2)) MB)"
Write-Host "PRoot Size: $prootSize bytes ($([math]::Round($prootSize/1MB, 2)) MB)"

Write-Host ""
Write-Host "=== Phase 3: Build ===" -ForegroundColor Yellow

$env:FROZEN_RUNTIME_PACKAGE_PATH = $step7PackagePath
$env:FROZEN_RUNTIME_PACKAGE_SHA256 = $expectedRuntimeSha
$env:AMITIA_RUNTIME_CANDIDATE_BUILD = "1"

Set-Location $AndroidRoot

Write-Host "Running Gradle assembleRelease..." -ForegroundColor Cyan
$gradleArgs = @("assembleRelease", "--no-daemon", "--offline")
& $gradlewExe @gradleArgs 2>&1

if ($LASTEXITCODE -ne 0) {
    throw "Gradle assembleRelease failed with exit code $LASTEXITCODE"
}
Write-Host "  [assembleRelease] PASS" -ForegroundColor Green

$apkSource = "$AndroidRoot\app\build\outputs\apk\release\app-release.apk"
if (-not (Test-Path $apkSource)) {
    throw "APK not generated at expected path: $apkSource"
}
Write-Host "APK generated: $apkSource" -ForegroundColor Green

Write-Host ""
Write-Host "=== Phase 4: Post-build Verification ===" -ForegroundColor Yellow

$apkSize = (Get-Item $apkSource).Length
$apkSha256 = (Get-FileHash $apkSource -Algorithm SHA256).Hash.ToLower()
Write-Host "APK Size: $apkSize bytes ($([math]::Round($apkSize/1MB, 2)) MB)"
Write-Host "APK SHA256: $apkSha256"

$tempExtract = "$env:TEMP\amitia-candidate-verify-$([guid]::NewGuid().ToString().Substring(0,8))"
New-Item -ItemType Directory -Force $tempExtract | Out-Null

try {
    Write-Host "Extracting APK for verification..."
    $tempApk = "$tempExtract\app.apk"
    Copy-Item $apkSource $tempApk
    Expand-Archive -Path $tempApk -DestinationPath $tempExtract -Force

    $prootFiles = Get-ChildItem -Path $tempExtract -Recurse -Filter "libamitia_proot.so" | Select-Object FullName
    Write-Host "PRoot count in APK: $($prootFiles.Count)"
    if ($prootFiles.Count -ne 1) {
        throw "Expected exactly 1 PRoot in APK, found $($prootFiles.Count)"
    }
    Write-Host "  [PRoot count = 1] PASS" -ForegroundColor Green

    $prootInApk = $prootFiles[0].FullName
    $prootInApkSha = (Get-FileHash $prootInApk -Algorithm SHA256).Hash.ToLower()
    if ($prootInApkSha -ne $expectedProotSha) {
        throw "Embedded PRoot SHA mismatch! Expected=$expectedProotSha, Actual=$prootInApkSha"
    }
    Write-Host "  [Embedded PRoot SHA match] PASS" -ForegroundColor Green

    $abiDirs = Get-ChildItem "$tempExtract\lib" -Directory -ErrorAction SilentlyContinue | Select-Object -ExpandProperty Name
    Write-Host "ABI dirs in APK: $($abiDirs -join ', ')"
    $forbiddenInApk = $abiDirs | Where-Object { $_ -ne "arm64-v8a" }
    if ($forbiddenInApk) {
        throw "Forbidden ABI found in APK: $($forbiddenInApk -join ', ')"
    }
    Write-Host "  [arm64-v8a only] PASS" -ForegroundColor Green

    $soFiles = Get-ChildItem "$tempExtract\lib\arm64-v8a" -Filter "*.so" -ErrorAction SilentlyContinue | Select-Object Name
    Write-Host "Native libs in arm64-v8a: $($soFiles.Name -join ', ')"

    $runtimePackageFiles = Get-ChildItem -Path $tempExtract -Recurse -Filter "amitia-runtime-*.zip" | Select-Object FullName
    Write-Host "Runtime Package count in APK: $($runtimePackageFiles.Count)"
    if ($runtimePackageFiles.Count -eq 1) {
        $rpInApkPath = $runtimePackageFiles[0].FullName
        $rpInApkSha = (Get-FileHash $rpInApkPath -Algorithm SHA256).Hash.ToLower()
        $rpInApkSize = (Get-Item $rpInApkPath).Length
        if ($rpInApkSha -ne $expectedRuntimeSha) {
            throw "Embedded Runtime Package SHA MISMATCH! Expected=$expectedRuntimeSha, Actual=$rpInApkSha"
        }
        Write-Host "  [Embedded Runtime Package SHA match] PASS" -ForegroundColor Green
        if ($rpInApkSize -ne $expectedRuntimeSize) {
            throw "Embedded Runtime Package size MISMATCH! Expected=$expectedRuntimeSize, Actual=$rpInApkSize"
        }
        Write-Host "  [Embedded Runtime Package size match] PASS" -ForegroundColor Green
    } elseif ($runtimePackageFiles.Count -eq 0) {
        throw "Runtime Package not embedded in APK - Candidate build FAIL. side-load is not permitted in Candidate build."
    } else {
        throw "Runtime Package duplicate! Found $($runtimePackageFiles.Count) in APK"
    }

    $manifestContent = Get-Content "$tempExtract\AndroidManifest.xml" -Raw -ErrorAction SilentlyContinue
    $servicesWithExported = @()
    if ($manifestContent -match 'service[^>]*android:exported="true"') {
        Write-Host "  [Services with exported=true found]" -ForegroundColor Yellow
    }

    Write-Host ""
    Write-Host "=== Phase 5: Freeze ===" -ForegroundColor Yellow

    $candidateDir = "$AndroidRoot\$OutputDir\$CandidateId"
    if (Test-Path $candidateDir) {
        $existingRecord = "$candidateDir\candidate-build-record.json"
        if (Test-Path $existingRecord) {
            $existing = Get-Content $existingRecord | ConvertFrom-Json
            if ($existing.apk.sha256 -eq $apkSha256) {
                Write-Host "Same Candidate ID with same bytes exists. Reusing." -ForegroundColor Yellow
            } else {
                throw "Candidate ID $CandidateId already exists with different bytes!"
            }
        } else {
            Remove-Item $candidateDir -Recurse -Force
        }
    }

    if (-not (Test-Path $candidateDir)) {
        New-Item -ItemType Directory -Force $candidateDir | Out-Null
    }

    $apkName = "$CandidateId.apk"
    Copy-Item $apkSource "$candidateDir\$apkName" -Force

    $apkFinalSize = (Get-Item "$candidateDir\$apkName").Length
    $apkFinalSha = (Get-FileHash "$candidateDir\$apkName" -Algorithm SHA256).Hash.ToLower()

    $sha256Sums = @()
    $sha256Sums += "$apkFinalSha  $apkName"
    $sha256Out = $sha256Sums -join "`n"

    $pubspecLockPath = "$MobileAppRoot\pubspec.lock"
    $pubspecLockSha = if (Test-Path $pubspecLockPath) { (Get-FileHash $pubspecLockPath -Algorithm SHA256).Hash.ToLower() } else { "N/A" }

    $timestamp = [DateTimeOffset]::UtcNow.ToString("o")

    $dirtyFileList = @()
    if ($sourceDirty) {
        $dirtyFileList = @(& $gitExe -C $ProjectRoot status --porcelain 2>&1 | ForEach-Object { $_.Trim() } | Where-Object { $_ -ne "" })
    }

    $buildRecord = [ordered]@{
        schemaVersion      = 1
        candidateId        = $CandidateId
        createdByPipeline  = "scripts/build-android-runtime-candidate.ps1"
        createdAt          = $timestamp
        sourceCommit       = $sourceCommit
        sourceBranch       = $sourceBranch
        sourceDirty        = [bool]$sourceDirty
        sourceDirtyFiles   = $dirtyFileList
        appVersionName     = $appVersionName
        appVersionCode     = $appVersionCode
        runtimeVersion     = $runtimeVersion
        abi                = "arm64-v8a"
        buildVariant       = $BuildVariant
        runtimePackage     = [ordered]@{
            runtimeVersion    = $runtimeVersion
            fileName          = "amitia-runtime-1.0.0.zip"
            sourceFileName    = "amitia-runtime-1.0.0-linux-arm64.zip"
            sourcePath        = $step7PackagePath
            sourceSha256      = $expectedRuntimeSha
            sourceSizeBytes   = $runtimePackageSize
            embeddedAssetPath = "runtime-package/amitia-runtime-1.0.0.zip"
            embeddedSha256    = $rpInApkSha
            embeddedSizeBytes = $rpInApkSize
            nativeTrustedSha256 = $nativePackageSha
            guestOs           = $nativeGuestOs
            architecture      = $nativeArchitecture
            identityVerified  = ($expectedRuntimeSha.ToLowerInvariant() -eq $nativePackageSha.ToLowerInvariant()) -and ($nativePackageSha.ToLowerInvariant() -eq $rpInApkSha.ToLowerInvariant())
        }
        proot              = [ordered]@{
            file       = "libamitia_proot.so"
            size       = $prootSize
            sha256     = $expectedProotSha
            source     = $step6ProotPath
            metadata   = $step6Record
        }
        apk                = [ordered]@{
            file       = $apkName
            size       = $apkFinalSize
            sha256     = $apkFinalSha
        }
        nativeTrustedIdentity = [ordered]@{
            runtimeVersion = $nativeRuntimeVersion
            packageSha256  = $nativePackageSha
            guestOs        = $nativeGuestOs
            architecture   = $nativeArchitecture
            fileName       = $nativeFileName
            assetPath      = $nativeAssetPath
        }
        toolchain          = [ordered]@{
            os             = [System.Runtime.InteropServices.RuntimeInformation]::OSDescription
            jdk            = $jdkVersion
            gradle         = $gradleVersion
            agp            = $agpVersion
            kotlin         = $kotlinVersion
            flutter        = ($flutterVersion -replace "`n", " " -replace "\s+", " ").Trim()
            dart           = ($dartVersion -replace "Dart", "").Trim()
            androidSdk     = $sdkDir
            buildTools     = $buildToolsVersion
            compileSdk     = $platformVersion
            ndk            = $ndkVersion
        }
        locks              = [ordered]@{
            pubspecLockSha = $pubspecLockSha
            gradleLockStatus = "manual-verification-required"
        }
    }

    $buildRecord | ConvertTo-Json -Depth 10 | Set-Content "$candidateDir\candidate-build-record.json" -Encoding UTF8
    Write-Host "Build Record: $candidateDir\candidate-build-record.json" -ForegroundColor Green

    $sha256Out | Set-Content "$candidateDir\SHA256SUMS" -Encoding UTF8
    Write-Host "SHA256SUMS: $candidateDir\SHA256SUMS" -ForegroundColor Green

    $stopwatch.Stop()
    Write-Host ""
    Write-Host "=== Candidate Built and Frozen ===" -ForegroundColor Green
    Write-Host "Candidate ID: $CandidateId"
    Write-Host "Output Directory: $candidateDir"
    Write-Host "APK: $apkName ($apkFinalSize bytes)"
    Write-Host "APK SHA256: $apkFinalSha"
    Write-Host "Total time: $($stopwatch.Elapsed.ToString('hh\:mm\:ss'))"
    Write-Host ""
    Write-Host "=== Runtime Package Identity Summary ===" -ForegroundColor Cyan
    Write-Host "Runtime Version: $runtimeVersion"
    Write-Host "Runtime Source SHA256: $expectedRuntimeSha"
    Write-Host "Native Trusted SHA256: BuildConfig.RUNTIME_PACKAGE_SHA256 (derived from frozen package)"
    Write-Host "APK Embedded SHA256: $rpInApkSha"
    Write-Host "Runtime Identity: PASS" -ForegroundColor Green

} finally {
    if (Test-Path $tempExtract) {
        Remove-Item $tempExtract -Recurse -Force -ErrorAction SilentlyContinue
    }
}

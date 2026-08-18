# 此文件为工作区代码打包工具，不可更改文件内任何内容，除非用户允许
param(
    [string]$OutputName = "U-Ai-source"
)

$scriptDir = if ($PSScriptRoot) { $PSScriptRoot } else { Split-Path -Parent $PSCommandPath }
$workspace = Split-Path -Parent $scriptDir
$parentDir = Split-Path $workspace -Leaf
$outputFile = Join-Path $workspace "$OutputName.tar.gz"

if (Test-Path $outputFile) { Remove-Item $outputFile -Force }

Set-Location $workspace

Write-Host "Packing source: $parentDir -> $outputFile"
$startTime = Get-Date

$allowedFilesFile = Join-Path $workspace ".pack-source-files.txt"
if (Test-Path $allowedFilesFile) { Remove-Item $allowedFilesFile -Force }

$gitArchiveResult = & git archive --format=tar HEAD 2>&1
if ($LASTEXITCODE -eq 0) {
    & git archive --format=tar -o "$OutputName.tar" HEAD
    if ($LASTEXITCODE -ne 0) {
        Write-Error "git archive failed"
        exit 1
    }
    & gzip "$OutputName.tar"
    if ($LASTEXITCODE -ne 0) {
        Write-Error "gzip failed"
        exit 1
    }
    Rename-Item -Path "$OutputName.tar.gz" -NewName $outputFile -Force
} else {
    Write-Host "git archive unavailable, using git ls-files whitelist approach"

    & git ls-files -co --exclude-standard | Out-File -FilePath $allowedFilesFile -Encoding utf8
    if ($LASTEXITCODE -ne 0) {
        Write-Error "git ls-files failed"
        exit 1
    }

    $forbiddenPatterns = @(
        'build_errors\.txt$',
        'errors\.txt$',
        'vet_errors\.txt$',
        'node_modules/',
        '\.exe$',
        '\.zip$',
        'desktop/release/',
        'desktop/build/',
        'desktop/dist-types/',
        'desktop/resources/core/',
        'front/dist/',
        'mobile_app/build/',
        'qdrant/storage/',
        'surrealdb/',
        'backend/data/',
        'backend/cmd/data/',
        'data/',
        'logs/',
        'runtime/out/',
        'AmitiaData/',
        '\.db$',
        '\.db-shm$',
        '\.db-wal$',
        '\.tar$',
        '\.tar\.gz$',
        '\.pyc$',
        '__pycache__/',
        '\.DS_Store$',
        'Thumbs\.db$'
    )

    $allFiles = Get-Content $allowedFilesFile
    $cleanFiles = @()
    $hasForbidden = $false

    foreach ($file in $allFiles) {
        $file = $file.Trim()
        if ([string]::IsNullOrWhiteSpace($file)) { continue }

        $isForbidden = $false
        foreach ($pattern in $forbiddenPatterns) {
            if ($file -match $pattern) {
                Write-Warning "Forbidden file detected: $file (matched: $pattern)"
                $isForbidden = $true
                $hasForbidden = $true
                break
            }
        }
        if (-not $isForbidden) {
            $cleanFiles += $file
        }
    }

    if ($hasForbidden) {
        Write-Error "Forbidden files detected. Aborting pack."
        if (Test-Path $allowedFilesFile) { Remove-Item $allowedFilesFile -Force }
        if (Test-Path $outputFile) { Remove-Item $outputFile -Force }
        exit 1
    }

    if ($cleanFiles.Count -eq 0) {
        Write-Error "No files to pack after whitelist filtering"
        if (Test-Path $allowedFilesFile) { Remove-Item $allowedFilesFile -Force }
        exit 1
    }

    $cleanFiles | Out-File -FilePath $allowedFilesFile -Encoding utf8

    & tar -czf $outputFile --files-from=$allowedFilesFile
    if ($LASTEXITCODE -ne 0) {
        Write-Error "tar failed"
        if (Test-Path $allowedFilesFile) { Remove-Item $allowedFilesFile -Force }
        if (Test-Path $outputFile) { Remove-Item $outputFile -Force }
        exit 1
    }

    Remove-Item $allowedFilesFile -Force
}

if (-not (Test-Path $outputFile)) {
    Write-Error "Output file not created"
    exit 1
}

$verifyResult = & tar -tzf $outputFile 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Error "Archive verification failed"
    Remove-Item $outputFile -Force
    exit 1
}

foreach ($item in $verifyResult) {
    if ($item -match 'node_modules|\.exe$|build_errors\.txt|errors\.txt|desktop/release|desktop/build') {
        Write-Error "Archive contains forbidden content: $item"
        Remove-Item $outputFile -Force
        exit 1
    }
}

$elapsed = (Get-Date) - $startTime
$size = (Get-Item $outputFile).Length

Write-Host "Done in $($elapsed.ToString('mm\:ss'))"
Write-Host "Output: $outputFile"
Write-Host "Size: $([math]::Round($size / 1MB, 2)) MB"

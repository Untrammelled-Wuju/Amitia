param([switch]$NoZip)
$ErrorActionPreference = "SilentlyContinue"
$repo = $PSScriptRoot | Split-Path -Parent
$stage = "$env:TEMP\U-Ai-pkg"
$zip  = Join-Path $repo "U-Ai-source.zip"

if (Test-Path $stage) { Remove-Item $stage -Recurse -Force }
if (Test-Path $zip)   { Remove-Item $zip -Force }
$null = New-Item -ItemType Directory -Path $stage -Force

$skipExt = @('.exe','.dll','.pdb','.zip','.tar','.tar.xz','.exe~','.bak','.bin','.dat','.db','.log','.so','.map','.node','.wasm','.lock')
$skipName = @('.DS_Store','Thumbs.db','go.sum','.metadata')

function Copy-Tree {
    param($Src, $Dst, [string[]]$ExtraSkip = @())
    if (-not (Test-Path $Dst)) { $null = New-Item -ItemType Directory -Path $Dst -Force }
    foreach ($item in Get-ChildItem -LiteralPath $Src -Force -ErrorAction SilentlyContinue) {
        if ($item.PSIsContainer) {
            if ($item.Attributes -match 'ReparsePoint') { continue }
            if ($item.Name -in @('node_modules','dist','build','.dart_tool','.gradle','.cxx','.kotlin') -or $item.Name -in $ExtraSkip) { continue }
            Copy-Tree -Src $item.FullName -Dst (Join-Path $Dst $item.Name) -ExtraSkip $ExtraSkip
        } else {
            if ($item.Attributes -match 'ReparsePoint') { continue }
            if ($skipExt -contains $item.Extension) { continue }
            if ($skipName -contains $item.Name)     { continue }
            Copy-Item -LiteralPath $item.FullName -Destination $Dst -Force
        }
    }
}

Write-Host "Front..." -Fore Yellow
Copy-Tree -Src (Join-Path $repo 'front') -Dst (Join-Path $stage 'front')

Write-Host "Flutter..." -Fore Yellow
Copy-Tree -Src (Join-Path $repo 'mobile_app\lib')    -Dst (Join-Path $stage 'mobile_app\lib')
Copy-Tree -Src (Join-Path $repo 'mobile_app\test')   -Dst (Join-Path $stage 'mobile_app\test')
Copy-Tree -Src (Join-Path $repo 'mobile_app\android') -Dst (Join-Path $stage 'mobile_app\android') -ExtraSkip @('build','app/src/main/assets/runtime-package','intermediates','outputs')

Write-Host "Backend..." -Fore Yellow
Copy-Tree -Src (Join-Path $repo 'backend\cmd')      -Dst (Join-Path $stage 'backend\cmd')
Copy-Tree -Src (Join-Path $repo 'backend\internal') -Dst (Join-Path $stage 'backend\internal')
Copy-Tree -Src (Join-Path $repo 'backend\pkg')      -Dst (Join-Path $stage 'backend\pkg') -ExtraSkip @('gameplugin/sdk/game-plugin/node_modules')
Copy-Tree -Src (Join-Path $repo 'backend\scripts')  -Dst (Join-Path $stage 'backend\scripts') -ExtraSkip @('.sidecar-build')
foreach ($f in @('go.mod','appsettings.json','check_quality_tables.go')) {
    $s = Join-Path $repo "backend\$f"; if (Test-Path $s) { Copy-Item $s (Join-Path $stage 'backend') -Force }
}

Write-Host "Desktop..." -Fore Yellow
Copy-Tree -Src (Join-Path $repo 'desktop') -Dst (Join-Path $stage 'desktop') -ExtraSkip @('dist-types','release','build')

Write-Host "Zipping..." -Fore Yellow
Compress-Archive -Path "$stage\*" -DestinationPath $zip -CompressionLevel Optimal -Force
Remove-Item $stage -Recurse -Force
$z = (Get-Item $zip).Length
Write-Host "Done: $zip ($([math]::Round($z/1MB,2)) MB)" -Fore Green
Start-Process explorer.exe $repo

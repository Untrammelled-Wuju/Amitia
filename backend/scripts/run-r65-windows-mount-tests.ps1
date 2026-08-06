$ErrorActionPreference = 'Stop'

$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = [Security.Principal.WindowsPrincipal]::new($identity)
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw 'R6-5 mount tests must run in an Administrator PowerShell.'
}

$goCommand = Get-Command go -ErrorAction Stop
$goExecutable = $goCommand.Source

$backendRoot = Split-Path -Parent $PSScriptRoot

$r65Root = Join-Path ([System.IO.Path]::GetTempPath()) ('amitia-r65-' + [guid]::NewGuid().ToString('N'))
$r65Vhd = Join-Path $r65Root 'mount.vhdx'
$r65Ext = Join-Path $r65Root 'ext'
$r65Mount = Join-Path $r65Ext 'mounted'
$r65Target = Join-Path $r65Mount 'target.bin'

New-Item -ItemType Directory -Path $r65Mount -Force | Out-Null
$r65Disk = $null

Push-Location $backendRoot

try {
    New-VHD -Path $r65Vhd -Dynamic -SizeBytes 128MB | Out-Null
    $r65Disk = Mount-VHD -Path $r65Vhd -Passthru
    $r65Disk | Initialize-Disk -PartitionStyle GPT -PassThru | New-Partition -UseMaximumSize -AssignDriveLetter | Format-Volume -FileSystem NTFS -NewFileSystemLabel AmitiaR65 -Confirm:$false | Out-Null
    $r65Volume = Get-Volume -FileSystemLabel AmitiaR65
    $r65Partition =
        $r65Volume |
            Get-Partition

    $env:AMITIA_R65_DISK_NUMBER =
        [string]$r65Disk.Number

    $env:AMITIA_R65_PARTITION_NUMBER =
        [string]$r65Partition.PartitionNumber

    $env:AMITIA_R65_REQUIRE_MOUNT_TESTS =
        '1'

    Add-PartitionAccessPath -DiskNumber $r65Disk.Number -PartitionNumber $r65Partition.PartitionNumber -AccessPath ($r65Mount + '\') | Out-Null
    $env:AMITIA_R65_MOUNT_POINT = $r65Mount
    $env:AMITIA_R65_MOUNT_TARGET = $r65Target

    $artifactDirectory =
        Join-Path $backendRoot 'artifacts\r6-5'

    New-Item `
        -ItemType Directory `
        -Path $artifactDirectory `
        -Force |
        Out-Null

    $mountLog =
        Join-Path $artifactDirectory 'windows-mount.log'

    & $goExecutable test `
        ./internal/extension/kernel/... `
        -run '^TestR6_5_.*Mount' `
        -count=1 `
        -v `
        2>&1 |
        Tee-Object `
            -FilePath $mountLog

    if ($LASTEXITCODE -ne 0) {
        throw "R6-5 mount tests failed with exit code $LASTEXITCODE"
    }

    if (
        Select-String `
            -Path $mountLog `
            -Pattern '--- SKIP:' `
            -Quiet
    ) {
        throw 'R6-5 mandatory mount tests were skipped.'
    }
}
finally {
    Pop-Location
    Remove-Item Env:AMITIA_R65_MOUNT_POINT -ErrorAction SilentlyContinue
    Remove-Item Env:AMITIA_R65_MOUNT_TARGET -ErrorAction SilentlyContinue
    Remove-Item Env:AMITIA_R65_DISK_NUMBER `
        -ErrorAction SilentlyContinue

    Remove-Item Env:AMITIA_R65_PARTITION_NUMBER `
        -ErrorAction SilentlyContinue

    Remove-Item Env:AMITIA_R65_REQUIRE_MOUNT_TESTS `
        -ErrorAction SilentlyContinue

    if ($r65Disk) { Dismount-VHD -Path $r65Vhd -ErrorAction SilentlyContinue }
    Remove-Item -LiteralPath $r65Root -Recurse -Force -ErrorAction SilentlyContinue
}

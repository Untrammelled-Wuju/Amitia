$ErrorActionPreference = 'Stop'
$r65Root = Join-Path ([System.IO.Path]::GetTempPath()) ('amitia-r65-' + [guid]::NewGuid().ToString('N'))
$r65Vhd = Join-Path $r65Root 'mount.vhdx'
$r65Ext = Join-Path $r65Root 'ext'
$r65Mount = Join-Path $r65Ext 'mounted'
$r65Target = Join-Path $r65Mount 'target.bin'
New-Item -ItemType Directory -Path $r65Mount -Force | Out-Null
$r65Disk = $null
try {
    New-VHD -Path $r65Vhd -Dynamic -SizeBytes 128MB | Out-Null
    $r65Disk = Mount-VHD -Path $r65Vhd -Passthru
    $r65Disk | Initialize-Disk -PartitionStyle GPT -PassThru | New-Partition -UseMaximumSize -AssignDriveLetter | Format-Volume -FileSystem NTFS -NewFileSystemLabel AmitiaR65 -Confirm:$false | Out-Null
    $r65Volume = Get-Volume -FileSystemLabel AmitiaR65
    Add-PartitionAccessPath -DiskNumber $r65Disk.Number -PartitionNumber ($r65Volume | Get-Partition).PartitionNumber -AccessPath ($r65Mount + '\')
    $env:AMITIA_R65_MOUNT_POINT = $r65Mount
    $env:AMITIA_R65_MOUNT_TARGET = $r65Target
    & 'C:\Code\Go\bin\go.exe' test ./internal/extension/kernel -run 'R6_5.*Mount' -count=1
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}
finally {
    if ($r65Disk) { Dismount-VHD -Path $r65Vhd -ErrorAction SilentlyContinue }
    Remove-Item -LiteralPath $r65Root -Recurse -Force -ErrorAction SilentlyContinue
}

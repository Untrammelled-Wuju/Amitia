# SPDX-FileCopyrightText: 2026 彭旭
# SPDX-License-Identifier: AGPL-3.0-only
param(
    [string]$ExePath,
    [string]$WorkDir
)

Add-Type -Name ConsoleHelper -Namespace Lagrange -MemberDefinition @"
[DllImport("kernel32.dll")] public static extern bool FreeConsole();
[DllImport("kernel32.dll")] public static extern bool AllocConsole();
"@

[Lagrange.ConsoleHelper]::FreeConsole() | Out-Null
[Lagrange.ConsoleHelper]::AllocConsole() | Out-Null

$psi = New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName = $ExePath
$psi.WorkingDirectory = $WorkDir
$psi.UseShellExecute = $false
$psi.CreateNoWindow = $true

$proc = [System.Diagnostics.Process]::Start($psi)
Write-Output "PID:$($proc.Id)"
$proc.WaitForExit()
Write-Output "EXIT:$($proc.ExitCode)"

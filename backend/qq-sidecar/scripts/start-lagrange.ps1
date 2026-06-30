# SPDX-FileCopyrightText: 2026 彭旭
# SPDX-License-Identifier: AGPL-3.0-only
Add-Type @"
using System;
using System.Diagnostics;
using System.Runtime.InteropServices;
public class LagrangeStarter {
    [DllImport("kernel32.dll", SetLastError=true)]
    static extern bool FreeConsole();
    [DllImport("kernel32.dll", SetLastError=true)]
    static extern bool AllocConsole();
    const uint CREATE_NEW_CONSOLE = 0x00000010;
    
    public static void Start(string exePath, string workDir) {
        FreeConsole();
        bool ok = AllocConsole();
        Console.WriteLine("Console: " + ok);
        
        var psi = new ProcessStartInfo {
            FileName = exePath,
            WorkingDirectory = workDir,
            UseShellExecute = false,
            CreateNoWindow = true,
        };
        var proc = Process.Start(psi);
        Console.WriteLine("PID:" + proc.Id);
        proc.WaitForExit();
        Console.WriteLine("EXIT:" + proc.ExitCode);
    }
}
"@
$exe = Join-Path $PSScriptRoot "..\..\lagrange\bin\Lagrange.OneBot.exe"
$dir = Join-Path $PSScriptRoot "..\..\lagrange\bin"
[LagrangeStarter]::Start($exe, $dir)

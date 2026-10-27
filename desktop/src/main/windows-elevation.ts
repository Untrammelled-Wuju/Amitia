import path from "node:path";
import { spawn, spawnSync } from "node:child_process";
import { isSystemDriveInstall } from "./storage-policy";

function powershellPath(): string {
  const systemRoot =
    process.env.SystemRoot?.trim() ||
    process.env.WINDIR?.trim() ||
    "C:\\Windows";
  return path.win32.join(
    systemRoot,
    "System32",
    "WindowsPowerShell",
    "v1.0",
    "powershell.exe",
  );
}

export function quoteWindowsArguments(values: readonly string[]): string {
  return values.map((value) => {
    if (value.length > 0 && !/[\s"]/u.test(value)) {
      return value;
    }
    let quoted = '"';
    let backslashes = 0;
    for (const character of value) {
      if (character === "\\") {
        backslashes += 1;
        continue;
      }
      if (character === '"') {
        quoted += "\\".repeat(backslashes * 2 + 1);
        quoted += '"';
      } else {
        quoted += "\\".repeat(backslashes);
        quoted += character;
      }
      backslashes = 0;
    }
    quoted += "\\".repeat(backslashes * 2);
    return `${quoted}"`;
  }).join(" ");
}

export function isProcessElevated(): boolean {
  if (process.platform !== "win32") {
    return true;
  }
  const script = [
    "$identity = [Security.Principal.WindowsIdentity]::GetCurrent()",
    "$principal = New-Object Security.Principal.WindowsPrincipal($identity)",
    "$principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)",
  ].join("; ");
  const result = spawnSync(
    powershellPath(),
    ["-NoProfile", "-NonInteractive", "-Command", script],
    {
      encoding: "utf8",
      timeout: 5000,
      windowsHide: true,
    },
  );
  return result.status === 0 && result.stdout.trim().toLowerCase() === "true";
}

export function relaunchAsAdministrator(
  executable = process.execPath,
  args = process.argv.slice(1),
): boolean {
  if (process.platform !== "win32") {
    return false;
  }
  const script = [
    "$ErrorActionPreference = 'Stop'",
    "$info = New-Object System.Diagnostics.ProcessStartInfo",
    "$info.FileName = $env:AMITIA_ELEVATION_EXE",
    "$info.Arguments = $env:AMITIA_ELEVATION_ARGUMENTS",
    "$info.WorkingDirectory = $env:AMITIA_ELEVATION_CWD",
    "$info.UseShellExecute = $true",
    "$info.Verb = 'runas'",
    "[System.Diagnostics.Process]::Start($info) | Out-Null",
  ].join("; ");
  const env = {
    ...process.env,
    AMITIA_ELEVATION_EXE: executable,
    AMITIA_ELEVATION_ARGUMENTS: quoteWindowsArguments(args),
    AMITIA_ELEVATION_CWD: process.cwd(),
  };
  const child = spawn(
    powershellPath(),
    ["-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script],
    {
      detached: true,
      env,
      stdio: "ignore",
      windowsHide: true,
    },
  );
  child.unref();
  return true;
}

export function shouldRequestAdministrator(
  installDir: string,
  platform = process.platform,
  elevated = false,
): boolean {
  return platform === "win32" && isSystemDriveInstall(installDir) && !elevated;
}

export function ensureInstalledAppElevation(installDir: string): void {
  if (!shouldRequestAdministrator(installDir, process.platform, isProcessElevated())) {
    return;
  }
  if (relaunchAsAdministrator()) {
    process.exit(0);
  }
}

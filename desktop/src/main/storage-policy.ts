import path from "node:path";

export function resolveSiblingDataDir(installDir: string): string {
  return path.resolve(path.dirname(installDir), "AmitiaData");
}

export function resolveElectronUserDataDir(dataDir: string): string {
  return path.join(dataDir, "electron");
}

export function resolveElectronSessionDataDir(dataDir: string): string {
  return path.join(dataDir, "electron", "session");
}

export function isSystemDriveInstall(installDir: string): boolean {
  return /^c:[\\/]/i.test(installDir.trim());
}

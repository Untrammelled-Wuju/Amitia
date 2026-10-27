import fs from "node:fs";
import path from "node:path";

interface StoredDataLocation {
  dataDir?: unknown;
}

export interface MigrateLegacyDataDirOptions {
  targetDataDir: string;
  locationFile: string;
}

export interface MigrateLegacyElectronProfileOptions {
  sourceProfileDir: string;
  targetProfileDir: string;
}

const ELECTRON_PROFILE_ENTRIES = [
  "Local Storage",
  "Session Storage",
  "IndexedDB",
  "WebStorage",
  "Network",
  "Preferences",
  "Local State",
  "device-id.txt",
  "runtime-id.txt",
  "data-location.json",
];

function readLegacyDataDir(locationFile: string): string | null {
  try {
    const parsed = JSON.parse(
      fs.readFileSync(locationFile, "utf8"),
    ) as StoredDataLocation;
    if (typeof parsed.dataDir !== "string" || !path.isAbsolute(parsed.dataDir)) {
      return null;
    }
    return path.resolve(parsed.dataDir);
  } catch {
    return null;
  }
}

export function migrateLegacyDataDir(
  options: MigrateLegacyDataDirOptions,
): boolean {
  const targetDataDir = path.resolve(options.targetDataDir);
  if (fs.existsSync(targetDataDir)) {
    return false;
  }
  const sourceDataDir = readLegacyDataDir(options.locationFile);
  if (
    !sourceDataDir ||
    sourceDataDir === targetDataDir ||
    !fs.existsSync(sourceDataDir)
  ) {
    return false;
  }
  fs.mkdirSync(path.dirname(targetDataDir), { recursive: true });
  fs.cpSync(sourceDataDir, targetDataDir, {
    recursive: true,
    preserveTimestamps: true,
    errorOnExist: true,
  });
  return true;
}

export function migrateLegacyElectronProfile(
  options: MigrateLegacyElectronProfileOptions,
): boolean {
  const sourceProfileDir = path.resolve(options.sourceProfileDir);
  const targetProfileDir = path.resolve(options.targetProfileDir);
  if (
    sourceProfileDir === targetProfileDir ||
    fs.existsSync(targetProfileDir) ||
    !fs.existsSync(sourceProfileDir)
  ) {
    return false;
  }
  fs.mkdirSync(targetProfileDir, { recursive: true });
  let copied = false;
  for (const entry of ELECTRON_PROFILE_ENTRIES) {
    const source = path.join(sourceProfileDir, entry);
    if (!fs.existsSync(source)) {
      continue;
    }
    fs.cpSync(source, path.join(targetProfileDir, entry), {
      recursive: true,
      preserveTimestamps: true,
      errorOnExist: true,
    });
    copied = true;
  }
  return copied;
}

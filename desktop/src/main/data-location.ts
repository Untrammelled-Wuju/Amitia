import fs from "node:fs";
import path from "node:path";

const LOCATION_SCHEMA_VERSION = 1;

interface StoredDataLocation {
  schemaVersion: number;
  dataDir: string;
  source: "legacy" | "stable";
  updatedAt: string;
}

export interface ResolveStableDataDirOptions {
  locationFile: string;
  legacyDataDir: string;
  stableDataDir: string;
}

function hasIdentity(dataDir: string): boolean {
  return (
    fs.existsSync(path.join(dataDir, "identity", "space.json")) ||
    fs.existsSync(path.join(dataDir, "device-mesh", "identity.json"))
  );
}

function readLocation(locationFile: string): StoredDataLocation | null {
  let raw: string;
  try {
    raw = fs.readFileSync(locationFile, "utf8");
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "ENOENT") {
      return null;
    }
    throw new Error(
      `设备数据目录配置读取失败: ${
        error instanceof Error ? error.message : String(error)
      }`,
    );
  }

  let parsed: StoredDataLocation;
  try {
    parsed = JSON.parse(raw) as StoredDataLocation;
  } catch {
    throw new Error(`设备数据目录配置损坏: ${locationFile}`);
  }
  if (
    parsed?.schemaVersion !== LOCATION_SCHEMA_VERSION ||
    typeof parsed.dataDir !== "string" ||
    !path.isAbsolute(parsed.dataDir) ||
    !["legacy", "stable"].includes(parsed.source)
  ) {
    throw new Error(`设备数据目录配置无效: ${locationFile}`);
  }
  return {
    ...parsed,
    dataDir: path.resolve(parsed.dataDir),
  };
}

function writeLocation(
  locationFile: string,
  dataDir: string,
  source: StoredDataLocation["source"],
): void {
  fs.mkdirSync(path.dirname(locationFile), { recursive: true });
  const record: StoredDataLocation = {
    schemaVersion: LOCATION_SCHEMA_VERSION,
    dataDir: path.resolve(dataDir),
    source,
    updatedAt: new Date().toISOString(),
  };
  const temporary = `${locationFile}.${process.pid}.${Date.now()}.tmp`;
  fs.writeFileSync(temporary, `${JSON.stringify(record, null, 2)}\n`, {
    encoding: "utf8",
    mode: 0o600,
  });
  try {
    fs.renameSync(temporary, locationFile);
  } catch (error) {
    try {
      fs.unlinkSync(temporary);
    } catch {}
    throw error;
  }
}

export function resolveStableDataDir(
  options: ResolveStableDataDirOptions,
): string {
  const stored = readLocation(options.locationFile);
  if (stored) {
    if (!fs.existsSync(stored.dataDir)) {
      throw new Error(
        `设备数据目录不存在，已停止启动以避免设备 ID 被重新生成: ${stored.dataDir}`,
      );
    }
    return stored.dataDir;
  }

  const stableExists = hasIdentity(options.stableDataDir);
  const legacyExists = hasIdentity(options.legacyDataDir);
  const dataDir = stableExists || !legacyExists
    ? options.stableDataDir
    : options.legacyDataDir;
  const source = dataDir === options.legacyDataDir ? "legacy" : "stable";

  fs.mkdirSync(dataDir, { recursive: true, mode: 0o700 });
  writeLocation(options.locationFile, dataDir, source);
  return path.resolve(dataDir);
}

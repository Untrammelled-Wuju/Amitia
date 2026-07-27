import { app } from "electron";
import path from "path";
import fs from "fs";
import { fileURLToPath } from "node:url";

export function isDevMode(): boolean {
  if (!app.isPackaged) return true;
  const execName = path.basename(process.execPath).toLowerCase();
  return execName === "electron" || execName === "electron.exe";
}

export function getInstallDir(): string {
  if (!isDevMode()) {
    return path.dirname(app.getPath("exe"));
  }
  return path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
}

export function getAmitiaDataDir(): string {
  if (!isDevMode()) {
    const installDir = getInstallDir();
    const parentDir = path.dirname(installDir);
    return path.join(parentDir, "AmitiaData");
  }
  const installDir = getInstallDir();
  return path.join(path.dirname(installDir), "AmitiaData");
}

export function ensureAmitiaDataDir(): string {
  const dataDir = getAmitiaDataDir();

  const requiredDirs = [
    dataDir,
    path.join(dataDir, "config"),
    path.join(dataDir, "data"),
    path.join(dataDir, "logs"),
    path.join(dataDir, "uploads"),
    path.join(dataDir, "qdrant"),
    path.join(dataDir, "surrealdb"),
    path.join(dataDir, "memory"),
    path.join(dataDir, "runtime"),
  ];

  for (const dir of requiredDirs) {
    if (!fs.existsSync(dir)) {
      fs.mkdirSync(dir, { recursive: true });
    }
  }

  assertWritable(dataDir);

  return dataDir;
}

function assertWritable(dir: string): void {
  const testFile = path.join(dir, `.write-test-${Date.now()}.tmp`);
  try {
    fs.writeFileSync(testFile, "ok", "utf-8");
    fs.unlinkSync(testFile);
  } catch (error) {
    const msg = `AmitiaData目录不可写: ${dir}。请将Amitia安装到用户可写目录，例如 D:\\Software\\amitia`;
    throw new Error(msg);
  }
}

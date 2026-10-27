import { app } from "electron";
import path from "path";
import fs from "fs";
import { fileURLToPath } from "node:url";
import {
  migrateLegacyDataDir,
  migrateLegacyElectronProfile,
} from "./data-location";
import {
  resolveElectronSessionDataDir,
  resolveElectronUserDataDir,
  resolveSiblingDataDir,
} from "./storage-policy";

let cachedAmitiaDataDir = "";

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

function getLegacyDataLocationFile(): string {
  return path.join(app.getPath("userData"), "data-location.json");
}

export function getAmitiaDataDir(): string {
  const override = process.env.AMITIA_DATA_DIR?.trim();
  if (override) {
    return path.resolve(override);
  }
  if (cachedAmitiaDataDir) {
    return cachedAmitiaDataDir;
  }

  const dataDir = resolveSiblingDataDir(getInstallDir());
  migrateLegacyDataDir({
    targetDataDir: dataDir,
    locationFile: getLegacyDataLocationFile(),
  });
  cachedAmitiaDataDir = dataDir;
  return cachedAmitiaDataDir;
}

export function configureElectronStorage(): void {
  const dataDir = getAmitiaDataDir();
  const userDataDir = resolveElectronUserDataDir(dataDir);
  const sessionDataDir = resolveElectronSessionDataDir(dataDir);
  const previousUserDataDir = app.getPath("userData");
  migrateLegacyElectronProfile({
    sourceProfileDir: previousUserDataDir,
    targetProfileDir: userDataDir,
  });
  fs.mkdirSync(userDataDir, { recursive: true });
  fs.mkdirSync(sessionDataDir, { recursive: true });
  app.setPath("userData", userDataDir);
  app.setPath("sessionData", sessionDataDir);
}

export function getBackendDataDir(): string {
  const override = process.env.AMITIA_DATA_DIR?.trim();
  if (override) {
    return path.resolve(override);
  }
  return getAmitiaDataDir();
}

export function ensureLocalToken(): string {
  const dataDir = getBackendDataDir();
  const tokenDir = path.join(dataDir, "security");
  const tokenFile = path.join(tokenDir, "local-token");
  const fs = require("node:fs");
  if (!fs.existsSync(tokenDir)) {
    fs.mkdirSync(tokenDir, { recursive: true });
  }
  if (fs.existsSync(tokenFile)) {
    const existing = fs.readFileSync(tokenFile, "utf8").trim();
    if (existing.length >= 32) {
      return tokenFile;
    }
  }
  const { randomBytes } = require("node:crypto");
  const token = randomBytes(32).toString("base64url");
  fs.writeFileSync(tokenFile, token, { mode: 0o600 });
  return tokenFile;
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

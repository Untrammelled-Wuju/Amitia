import { app } from "electron";
import { randomUUID } from "node:crypto";
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

export function getRuntimeId(): string {
  const dataDir = app.getPath("userData");
  const idFile = join(dataDir, "runtime-id.txt");
  try {
    if (existsSync(idFile)) {
      const id = readFileSync(idFile, "utf8").trim();
      if (id) return id;
    }
    if (!existsSync(dataDir)) {
      mkdirSync(dataDir, { recursive: true });
    }
    const newId = "rt_" + randomUUID();
    writeFileSync(idFile, newId, "utf8");
    return newId;
  } catch {
    return "rt_" + randomUUID();
  }
}

export function getDeviceId(): string {
  const dataDir = app.getPath("userData");
  const idFile = join(dataDir, "device-id.txt");
  try {
    if (existsSync(idFile)) {
      const id = readFileSync(idFile, "utf8").trim();
      if (id) return id;
    }
    if (!existsSync(dataDir)) {
      mkdirSync(dataDir, { recursive: true });
    }
    const newId = "dev_" + randomUUID();
    writeFileSync(idFile, newId, "utf8");
    return newId;
  } catch {
    return "dev_" + randomUUID();
  }
}

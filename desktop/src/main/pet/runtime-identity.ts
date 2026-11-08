import { app } from "electron";
import { randomUUID } from "node:crypto";
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";

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
  } catch (error) {
  console.error("[original-error]", error);
return "rt_" + randomUUID();
  }
}

export function resolveSharedDeviceId(dataDir: string): string {
  const identityPath = join(dirname(dataDir), "device-mesh", "identity.json");
  const identity = JSON.parse(readFileSync(identityPath, "utf8")) as { deviceId?: unknown };
  if (typeof identity.deviceId !== "string" || !identity.deviceId.trim()) {
    throw new Error(`统筹中心设备身份缺少 deviceId: ${identityPath}`);
  }
  return identity.deviceId.trim();
}

export function getDeviceId(): string {
  return resolveSharedDeviceId(app.getPath("userData"));
}

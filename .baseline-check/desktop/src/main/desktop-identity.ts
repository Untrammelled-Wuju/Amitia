import fs from "node:fs";
import path from "node:path";
import { randomUUID } from "node:crypto";
import { getAmitiaDataDir } from "./path-manager";

let cachedDesktopInstanceID: string | null = null;

function readNonEmptyText(
  filePath: string,
): string | null {
  try {
    const value = fs
      .readFileSync(filePath, "utf8")
      .trim();
    return value || null;
  } catch {
    return null;
  }
}

export function ensureDesktopInstanceID(): string {
  if (cachedDesktopInstanceID) {
    return cachedDesktopInstanceID;
  }

  const securityDir = path.join(
    getAmitiaDataDir(),
    "security",
  );
  const instanceFile = path.join(
    securityDir,
    "desktop-instance-id",
  );

  fs.mkdirSync(securityDir, {
    recursive: true,
    mode: 0o700,
  });

  const existing =
    readNonEmptyText(instanceFile);
  if (existing) {
    cachedDesktopInstanceID = existing;
    return existing;
  }

  const instanceID =
    `desktop_${randomUUID()}`;

  const tempFile = path.join(
    securityDir,
    `.desktop-instance-id-${process.pid}-${Date.now()}.tmp`,
  );

  fs.writeFileSync(
    tempFile,
    instanceID,
    {
      encoding: "utf8",
      mode: 0o600,
      flag: "wx",
    },
  );

  fs.renameSync(
    tempFile,
    instanceFile,
  );

  cachedDesktopInstanceID =
    instanceID;

  return instanceID;
}

export function getDesktopInstanceID(): string {
  return ensureDesktopInstanceID();
}

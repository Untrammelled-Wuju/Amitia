import { spawnSync } from "node:child_process";
import { hostname } from "node:os";

const MAX_DEVICE_NAME_LENGTH = 64;

function readMacComputerName(): string {
  const result = spawnSync("/usr/sbin/scutil", ["--get", "ComputerName"], {
    encoding: "utf8",
    timeout: 3000,
  });
  if (result.error || result.status !== 0) return "";
  return result.stdout.trim();
}

export function getSystemDeviceName(): string {
  const candidates: string[] = [];
  if (process.platform === "darwin") {
    candidates.push(readMacComputerName());
    candidates.push(hostname());
  } else if (process.platform === "win32") {
    candidates.push(process.env.COMPUTERNAME || "");
    candidates.push(hostname());
  } else {
    candidates.push(hostname());
  }
  for (const candidate of candidates) {
    const normalized = normalizeDeviceName(candidate);
    if (normalized) return normalized;
  }
  throw new Error("无法获取系统设备名：系统未返回可用的计算机名称");
}

export function normalizeDeviceName(raw: string): string {
  const trimmed = raw.trim();
  if (!trimmed) return "";
  const collapsed = trimmed
    .replace(/[\u0000-\u001f\u007f\u200b\ufeff]/g, "")
    .replace(/\s+/g, " ")
    .trim();
  return Array.from(collapsed).slice(0, MAX_DEVICE_NAME_LENGTH).join("").trim();
}

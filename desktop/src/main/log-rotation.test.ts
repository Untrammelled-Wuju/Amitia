import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import {
  cleanupExpiredLogs,
  DailySizeRotatingLog,
  LOG_RETENTION_MS,
  logDateKey,
} from "./log-rotation";

const directories: string[] = [];

function createDirectory(): string {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "amitia-log-"));
  directories.push(directory);
  return directory;
}

afterEach(() => {
  for (const directory of directories.splice(0)) {
    fs.rmSync(directory, { recursive: true, force: true });
  }
});

describe("DailySizeRotatingLog", () => {
  it("按日期和大小生成独立文件", () => {
    const directory = createDirectory();
    const now = new Date(2026, 10, 1, 12, 0, 0);
    const writer = new DailySizeRotatingLog(directory, "electron", {
      maxFileBytes: 10,
      now: () => now,
    });
    writer.write("12345\n");
    writer.write("67890\n");
    writer.close();

    expect(
      fs
        .readdirSync(directory)
        .filter((name) => name.endsWith(".log"))
        .sort(),
    ).toEqual([
      `electron.${logDateKey(now)}.001.log`,
      `electron.${logDateKey(now)}.002.log`,
    ]);
    expect(fs.readFileSync(path.join(directory, `electron.${logDateKey(now)}.001.log`), "utf8")).toBe("12345\n");
    expect(fs.readFileSync(path.join(directory, `electron.${logDateKey(now)}.002.log`), "utf8")).toBe("67890\n");
  });

  it("跨日期时自动切换文件", () => {
    const directory = createDirectory();
    let now = new Date(2026, 10, 1, 23, 59, 0);
    const writer = new DailySizeRotatingLog(directory, "app", {
      now: () => now,
    });
    writer.write("day-one\n");
    now = new Date(2026, 10, 2, 0, 0, 1);
    writer.write("day-two\n");
    writer.close();

    expect(fs.existsSync(path.join(directory, "app.20261101.001.log"))).toBe(true);
    expect(fs.existsSync(path.join(directory, "app.20261102.001.log"))).toBe(true);
  });
});

describe("cleanupExpiredLogs", () => {
  it("只清理超过保留期的托管日志", () => {
    const directory = createDirectory();
    const now = new Date(2026, 10, 1, 12, 0, 0);
    const expired = path.join(directory, "electron.20260901.001.log");
    const recent = path.join(directory, "electron.20261031.001.log");
    const unrelated = path.join(directory, "settings.json");
    fs.writeFileSync(expired, "expired");
    fs.writeFileSync(recent, "recent");
    fs.writeFileSync(unrelated, "unrelated");
    const old = new Date(now.getTime() - LOG_RETENTION_MS - 1000);
    fs.utimesSync(expired, old, old);

    expect(cleanupExpiredLogs(directory, LOG_RETENTION_MS, now)).toEqual([
      expired,
    ]);
    expect(fs.existsSync(expired)).toBe(false);
    expect(fs.existsSync(recent)).toBe(true);
    expect(fs.existsSync(unrelated)).toBe(true);
  });
});

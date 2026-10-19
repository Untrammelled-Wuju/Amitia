import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { resolveStableDataDir } from "./data-location";

const roots: string[] = [];

function createRoot(): string {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "amitia-data-location-"));
  roots.push(root);
  return root;
}

function writeIdentity(dataDir: string, spaceId: string): void {
  const identityDir = path.join(dataDir, "identity");
  fs.mkdirSync(identityDir, { recursive: true });
  fs.writeFileSync(
    path.join(identityDir, "space.json"),
    JSON.stringify({ schemaVersion: 1, spaceId }),
    "utf8",
  );
}

afterEach(() => {
  for (const root of roots.splice(0)) {
    fs.rmSync(root, { recursive: true, force: true });
  }
});

describe("resolveStableDataDir", () => {
  it("首次启动迁移到旧数据目录并持久记录", () => {
    const root = createRoot();
    const locationFile = path.join(root, "user-data", "data-location.json");
    const legacyDataDir = path.join(root, "legacy", "AmitiaData");
    const stableDataDir = path.join(root, "user-data", "AmitiaData");
    writeIdentity(legacyDataDir, "space_legacy");

    expect(
      resolveStableDataDir({ locationFile, legacyDataDir, stableDataDir }),
    ).toBe(path.resolve(legacyDataDir));

    const record = JSON.parse(fs.readFileSync(locationFile, "utf8"));
    expect(record.dataDir).toBe(path.resolve(legacyDataDir));
    expect(record.source).toBe("legacy");
  });

  it("安装路径变化后继续使用已记录的数据目录", () => {
    const root = createRoot();
    const locationFile = path.join(root, "user-data", "data-location.json");
    const originalDataDir = path.join(root, "original", "AmitiaData");
    writeIdentity(originalDataDir, "space_original");
    resolveStableDataDir({
      locationFile,
      legacyDataDir: originalDataDir,
      stableDataDir: path.join(root, "user-data", "AmitiaData"),
    });

    const movedLegacyDir = path.join(root, "moved", "AmitiaData");
    writeIdentity(movedLegacyDir, "space_moved");
    expect(
      resolveStableDataDir({
        locationFile,
        legacyDataDir: movedLegacyDir,
        stableDataDir: path.join(root, "second-user-data", "AmitiaData"),
      }),
    ).toBe(path.resolve(originalDataDir));
  });

  it("记录的数据目录丢失时拒绝生成新设备身份", () => {
    const root = createRoot();
    const locationFile = path.join(root, "user-data", "data-location.json");
    const legacyDataDir = path.join(root, "legacy", "AmitiaData");
    writeIdentity(legacyDataDir, "space_legacy");
    resolveStableDataDir({
      locationFile,
      legacyDataDir,
      stableDataDir: path.join(root, "user-data", "AmitiaData"),
    });
    fs.rmSync(legacyDataDir, { recursive: true, force: true });

    expect(() =>
      resolveStableDataDir({
        locationFile,
        legacyDataDir,
        stableDataDir: path.join(root, "other", "AmitiaData"),
      }),
    ).toThrow("设备数据目录不存在");
  });

  it("新设备使用固定用户数据目录", () => {
    const root = createRoot();
    const locationFile = path.join(root, "user-data", "data-location.json");
    const stableDataDir = path.join(root, "user-data", "AmitiaData");

    expect(
      resolveStableDataDir({
        locationFile,
        legacyDataDir: path.join(root, "legacy", "AmitiaData"),
        stableDataDir,
      }),
    ).toBe(path.resolve(stableDataDir));
    expect(fs.existsSync(stableDataDir)).toBe(true);
  });
});

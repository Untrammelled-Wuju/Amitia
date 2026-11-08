import { mkdirSync, mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { resolveSharedDeviceId } from "./runtime-identity";

describe("桌宠设备身份", () => {
  it("复用统筹中心的设备 ID", () => {
    const root = mkdtempSync(join(tmpdir(), "amitia-runtime-identity-"));
    const electronDir = join(root, "electron");
    const meshDir = join(root, "device-mesh");
    mkdirSync(electronDir, { recursive: true });
    mkdirSync(meshDir, { recursive: true });
    writeFileSync(join(meshDir, "identity.json"), JSON.stringify({ deviceId: "dev_shared", runtimeId: "rt_center" }));
    writeFileSync(join(electronDir, "device-id.txt"), "dev_legacy");

    expect(resolveSharedDeviceId(electronDir)).toBe("dev_shared");
  });

  it("缺少统筹中心设备 ID 时明确失败", () => {
    const root = mkdtempSync(join(tmpdir(), "amitia-runtime-identity-"));
    const electronDir = join(root, "electron");
    const meshDir = join(root, "device-mesh");
    mkdirSync(electronDir, { recursive: true });
    mkdirSync(meshDir, { recursive: true });
    writeFileSync(join(meshDir, "identity.json"), JSON.stringify({ runtimeId: "rt_center" }));

    expect(() => resolveSharedDeviceId(electronDir)).toThrow("统筹中心设备身份缺少 deviceId");
  });
});

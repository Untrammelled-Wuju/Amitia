import path from "node:path";
import { describe, expect, it } from "vitest";
import {
  isSystemDriveInstall,
  resolveElectronSessionDataDir,
  resolveElectronUserDataDir,
  resolveSiblingDataDir,
} from "./storage-policy";

describe("storage policy", () => {
  it("places data beside the installation directory", () => {
    expect(resolveSiblingDataDir("D:\\a\\Amitia")).toBe(
      path.resolve("D:\\a\\AmitiaData"),
    );
  });

  it("places Electron profile data below AmitiaData", () => {
    const dataDir = path.resolve("D:\\a\\AmitiaData");
    expect(resolveElectronUserDataDir(dataDir)).toBe(
      path.join(dataDir, "electron"),
    );
    expect(resolveElectronSessionDataDir(dataDir)).toBe(
      path.join(dataDir, "electron", "session"),
    );
  });

  it("requires administrator privileges only for the system drive", () => {
    expect(isSystemDriveInstall("C:\\Amitia")).toBe(true);
    expect(isSystemDriveInstall("c:/Amitia")).toBe(true);
    expect(isSystemDriveInstall("D:\\Amitia")).toBe(false);
  });
});

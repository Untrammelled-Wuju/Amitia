import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import {
  migrateLegacyDataDir,
  migrateLegacyElectronProfile,
} from "./data-location";

const roots: string[] = [];

function createRoot(): string {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "amitia-data-location-"));
  roots.push(root);
  return root;
}

afterEach(() => {
  for (const root of roots.splice(0)) {
    fs.rmSync(root, { recursive: true, force: true });
  }
});

describe("migrateLegacyDataDir", () => {
  it("copies the recorded legacy data directory to the sibling target", () => {
    const root = createRoot();
    const locationFile = path.join(root, "profile", "data-location.json");
    const sourceDataDir = path.join(root, "old", "AmitiaData");
    const targetDataDir = path.join(root, "install", "AmitiaData");
    fs.mkdirSync(path.join(sourceDataDir, "identity"), { recursive: true });
    fs.writeFileSync(
      path.join(sourceDataDir, "identity", "space.json"),
      "legacy",
    );
    fs.mkdirSync(path.dirname(locationFile), { recursive: true });
    fs.writeFileSync(locationFile, JSON.stringify({ dataDir: sourceDataDir }));

    expect(migrateLegacyDataDir({ targetDataDir, locationFile })).toBe(true);
    expect(
      fs.readFileSync(
        path.join(targetDataDir, "identity", "space.json"),
        "utf8",
      ),
    ).toBe("legacy");
    expect(fs.existsSync(sourceDataDir)).toBe(true);
  });

  it("does not overwrite an existing sibling data directory", () => {
    const root = createRoot();
    const locationFile = path.join(root, "profile", "data-location.json");
    const sourceDataDir = path.join(root, "old", "AmitiaData");
    const targetDataDir = path.join(root, "install", "AmitiaData");
    fs.mkdirSync(sourceDataDir, { recursive: true });
    fs.mkdirSync(targetDataDir, { recursive: true });
    fs.mkdirSync(path.dirname(locationFile), { recursive: true });
    fs.writeFileSync(path.join(sourceDataDir, "old.txt"), "old");
    fs.writeFileSync(path.join(targetDataDir, "current.txt"), "current");
    fs.writeFileSync(locationFile, JSON.stringify({ dataDir: sourceDataDir }));

    expect(migrateLegacyDataDir({ targetDataDir, locationFile })).toBe(false);
    expect(fs.existsSync(path.join(targetDataDir, "old.txt"))).toBe(false);
    expect(fs.readFileSync(path.join(targetDataDir, "current.txt"), "utf8")).toBe(
      "current",
    );
  });

  it("ignores missing and invalid legacy records", () => {
    const root = createRoot();
    const locationFile = path.join(root, "profile", "data-location.json");
    const targetDataDir = path.join(root, "install", "AmitiaData");
    fs.mkdirSync(path.dirname(locationFile), { recursive: true });
    fs.writeFileSync(locationFile, "{bad json");

    expect(migrateLegacyDataDir({ targetDataDir, locationFile })).toBe(false);
    expect(fs.existsSync(targetDataDir)).toBe(false);
  });

  it("copies persistent Electron profile data without cache directories", () => {
    const root = createRoot();
    const sourceProfileDir = path.join(root, "roaming", "@amitia", "desktop");
    const targetProfileDir = path.join(root, "AmitiaData", "electron");
    fs.mkdirSync(path.join(sourceProfileDir, "Local Storage"), {
      recursive: true,
    });
    fs.mkdirSync(path.join(sourceProfileDir, "Cache"), { recursive: true });
    fs.writeFileSync(path.join(sourceProfileDir, "Preferences"), "{}");
    fs.writeFileSync(
      path.join(sourceProfileDir, "Local Storage", "settings.log"),
      "settings",
    );
    fs.writeFileSync(path.join(sourceProfileDir, "Cache", "cache.bin"), "cache");

    expect(
      migrateLegacyElectronProfile({ sourceProfileDir, targetProfileDir }),
    ).toBe(true);
    expect(fs.readFileSync(path.join(targetProfileDir, "Preferences"), "utf8")).toBe(
      "{}",
    );
    expect(
      fs.readFileSync(
        path.join(targetProfileDir, "Local Storage", "settings.log"),
        "utf8",
      ),
    ).toBe("settings");
    expect(fs.existsSync(path.join(targetProfileDir, "Cache"))).toBe(false);
  });
});

import { describe, expect, it } from "vitest";
import { isAbsolute, join, resolve } from "node:path";
import { resolveDesktopPetInstallationRoot } from "../installation-path";

describe("resolveDesktopPetInstallationRoot", () => {
  it("anchors backend relative installation paths below AmitiaData", () => {
    const dataDir = resolve("tmp", "AmitiaData");
    const result = resolveDesktopPetInstallationRoot(
      join("desktop-pets", "installations", "install-1"),
      dataDir,
    );

    expect(result).toBe(
      join(dataDir, "desktop-pets", "installations", "install-1"),
    );
    expect(result && isAbsolute(result)).toBe(true);
  });

  it("rejects relative installation paths that escape AmitiaData", () => {
    const dataDir = resolve("tmp", "AmitiaData");
    expect(
      resolveDesktopPetInstallationRoot(join("..", "outside"), dataDir),
    ).toBeNull();
  });

  it("preserves explicit absolute roots for development and tests", () => {
    const dataDir = resolve("tmp", "AmitiaData");
    const absoluteRoot = resolve("tmp", "custom-pet-install");
    expect(
      resolveDesktopPetInstallationRoot(absoluteRoot, dataDir),
    ).toBe(absoluteRoot);
  });
});

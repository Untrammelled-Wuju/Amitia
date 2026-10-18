import { describe, expect, it } from "vitest";
import {
  desktopUpdateChannel,
  isPrereleaseVersion,
  resolveAllowBetaChannel,
} from "./update-channel";

describe("update channel preference", () => {
  it("detects prerelease application versions", () => {
    expect(isPrereleaseVersion("26.2.0-beta.5")).toBe(true);
    expect(isPrereleaseVersion("26.2.0-alpha.1+build.7")).toBe(true);
    expect(isPrereleaseVersion("26.2.0")).toBe(false);
    expect(isPrereleaseVersion("26.2.0+build.7")).toBe(false);
  });

  it("defaults beta builds to the beta channel without stored preference", () => {
    expect(resolveAllowBetaChannel(undefined, "26.2.0-beta.5")).toBe(true);
    expect(resolveAllowBetaChannel(undefined, "26.2.0")).toBe(false);
  });

  it("preserves an explicit user preference", () => {
    expect(resolveAllowBetaChannel(false, "26.2.0-beta.5")).toBe(false);
    expect(resolveAllowBetaChannel(true, "26.2.0")).toBe(true);
  });

  it("selects the matching electron-updater manifest", () => {
    expect(desktopUpdateChannel(false)).toBe("latest");
    expect(desktopUpdateChannel(true)).toBe("beta");
  });
});

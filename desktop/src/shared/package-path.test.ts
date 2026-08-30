import { describe, expect, it } from "vitest";
import { caseFoldPackagePath, decodePackagePathFromUrl, encodePackagePathForUrl, normalizePackagePath, resolveActionResourcePackagePath } from "./package-path";

describe("Package V2 portable path contract", () => {
  it("accepts canonical Unicode, spaces, percent/hash and ordinary double dots", () => {
    for (const value of ["actions/idle/action.json", "actions/待机/帧 01.png", "actions/idle/frame#01.png", "actions/idle/100%.png", "actions/idle/frame..png", "actions/idle/..frame.png"]) {
      expect(normalizePackagePath(value)).toBe(value);
    }
  });
  it("rejects traversal, aliases, Windows-invalid names, bad Unicode and oversized paths", () => {
    for (const value of ["../escape.png", "actions/../escape.png", "actions/./idle.png", "actions//idle.png", "actions/idle/", "actions\\idle\\frame.png", "C:/frame.png", "actions/idle/a?.png", "actions/idle/a*.png", "actions/idle/a|b.png", "actions/idle/con.png", "actions/idle/LPT1.json", "actions/idle/trailing.", "actions/e\u0301/frame.png", "a".repeat(256), "a".repeat(513), `actions/idle/${String.fromCharCode(0xd800)}.png`]) {
      expect(() => normalizePackagePath(value)).toThrow();
    }
  });
  it("matches the Go Unicode simple-lower case-fold key", () => {
    expect(caseFoldPackagePath("actions/İ/Frame.PNG")).toBe("actions/i/frame.png");
  });
  it("round-trips URL path segments exactly once", () => {
    const value = "actions/待机/frame #100%.png";
    const encoded = encodePackagePathForUrl(value);
    expect(encoded).toContain("%23"); expect(encoded).toContain("%25");
    expect(decodePackagePathFromUrl(encoded)).toBe(value);
    expect(() => decodePackagePathFromUrl("actions/idle/a%2Fb.png")).toThrow();
    expect(() => decodePackagePathFromUrl("actions/idle/a%5Cb.png")).toThrow();
  });
  it("resolves frame resources relative to the action config", () => {
    expect(resolveActionResourcePackagePath("actions/idle/action.json", "sprites/frame.png")).toBe("actions/idle/sprites/frame.png");
  });
});

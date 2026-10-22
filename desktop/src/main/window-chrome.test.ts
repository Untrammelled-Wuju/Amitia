import { describe, expect, it } from "vitest";
import { validCaptionColors, windowChromeOptions } from "./window-chrome";

describe("平台原生窗口控件", () => {
  it("macOS 保留原生控件且不被无边框配置覆盖", () => {
    const options = windowChromeOptions("darwin", false);
    expect(options.frame).not.toBe(false);
    expect(options.titleBarStyle).toBe("hiddenInset");
    expect(options.trafficLightPosition!.x).toBeGreaterThan(0);
    expect(options.trafficLightPosition!.y! + 14).toBeLessThanOrEqual(34);
    expect(options.titleBarOverlay).toBeUndefined();
  });

  it("Windows 在两个主题中均启用原生覆盖按钮并提供可读配色", () => {
    for (const dark of [false, true]) {
      const options = windowChromeOptions("win32", dark);
      expect(options.frame).not.toBe(false);
      expect(options.titleBarStyle).toBe("hidden");
      expect(options.trafficLightPosition).toBeUndefined();
      expect(options.titleBarOverlay).toEqual(expect.objectContaining({ height: 34 }));
      expect(validCaptionColors(options.titleBarOverlay)).toBe(true);
      const colors = options.titleBarOverlay as { color: string; symbolColor: string };
      expect(colors.color).not.toBe(colors.symbolColor);
    }
  });

  it("其他平台保留现有自定义控件模式", () => {
    const options = windowChromeOptions("linux", false);
    expect(options.frame).toBe(false);
    expect(options.titleBarOverlay).toBeUndefined();
  });

  it("只接收明确的 RGB 配色，拒绝任意原生窗口参数", () => {
    expect(validCaptionColors({ color: "#aAbBcC", symbolColor: "#010203" })).toBe(true);
    for (const value of [null, true, "#ffffff", {}, { color: "red", symbolColor: "#ffffff" }, { color: "#ffffff", symbolColor: 12 }]) {
      expect(validCaptionColors(value)).toBe(false);
    }
  });
});

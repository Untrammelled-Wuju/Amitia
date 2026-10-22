import type { BrowserWindowConstructorOptions } from "electron";

export function windowChromeOptions(platform: NodeJS.Platform, dark: boolean): BrowserWindowConstructorOptions {
  if (platform === "darwin") {
    return { titleBarStyle: "hiddenInset", trafficLightPosition: { x: 16, y: 10 } };
  }
  if (platform === "win32") {
    return {
      titleBarStyle: "hidden",
      titleBarOverlay: {
        height: 34,
        color: dark ? "#1b1b1c" : "#f9fafb",
        symbolColor: dark ? "#f9fafb" : "#0f1115",
      },
    };
  }
  return { frame: false };
}

export function validCaptionColors(value: unknown): value is { color: string; symbolColor: string } {
  if (!value || typeof value !== "object") return false;
  const { color, symbolColor } = value as Record<string, unknown>;
  return typeof color === "string" && /^#[0-9a-f]{6}$/i.test(color)
    && typeof symbolColor === "string" && /^#[0-9a-f]{6}$/i.test(symbolColor);
}

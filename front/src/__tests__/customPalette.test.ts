import { beforeEach, describe, expect, it, vi } from "vitest";
import { contrastRatio, normalizeCustomPalette, paletteText, readableText } from "../composables/customPalette";

describe("custom palette", () => {
  beforeEach(() => { vi.resetModules(); localStorage.clear(); document.documentElement.removeAttribute("style"); });
  it("chooses readable text and validates saved data", () => {
    expect(contrastRatio("#000000", "#FFFFFF")).toBe(21);
    expect(readableText("#FFFF00")).toBe("#000000");
    expect(readableText("#000080")).toBe("#FFFFFF");
    expect(normalizeCustomPalette({ primary: "invalid", enabled: "yes", textMode: "invalid" })).toMatchObject({ primary: "#6C8FEA", enabled: false, textMode: "auto" });
    expect(paletteText(normalizeCustomPalette({ textMode: "custom", text: "#123456" }), "#FFFFFF")).toBe("#123456");
  });
  it("applies both accents and text, restores preset and persists custom values", async () => {
    const theme = (await import("../composables/useTheme")).useTheme();
    theme.setAccentColor("#52B788");
    theme.setCustomPalette({ enabled: true, primary: "#FFFF00", secondary: "#000080", textMode: "custom", text: "#ABCDEF" });
    const style = document.documentElement.style;
    expect(style.getPropertyValue("--tp-primary")).toBe("#FFFF00");
    expect(style.getPropertyValue("--tp-secondary")).toBe("#000080");
    expect(style.getPropertyValue("--tp-text")).toBe("#ABCDEF");
    expect(style.getPropertyValue("--tp-text-on-primary")).toBe("#000000");
    theme.setCustomPalette({ enabled: false });
    expect(style.getPropertyValue("--tp-primary")).toBe("#52B788");
    expect(style.getPropertyValue("--tp-text")).toBe("");
    vi.resetModules();
    const restored = (await import("../composables/useTheme")).useTheme();
    expect(restored.state.value.customPalette).toMatchObject({ enabled: false, primary: "#FFFF00", secondary: "#000080" });
    restored.setCustomPalette({ enabled: true, textMode: "auto" });
    restored.setPreset("light");
    await vi.waitFor(() => expect(style.getPropertyValue("--tp-text")).toBe("#000000"));
    restored.setPreset("dark");
    await vi.waitFor(() => expect(style.getPropertyValue("--tp-text")).toBe("#FFFFFF"));
  });
});

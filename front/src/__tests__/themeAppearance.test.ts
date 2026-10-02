import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

describe("default appearance", () => {
  beforeEach(() => {
    vi.resetModules();
    localStorage.clear();
  });
  afterEach(() => {
    localStorage.clear();
    document.documentElement.removeAttribute("style");
  });

  it("adapts the default blue to light and dark mode", async () => {
    const { useTheme } = await import("../composables/useTheme");
    const theme = useTheme();
    expect(theme.state.value.accentColor).toBe("#0066CC");
    expect(document.documentElement.style.getPropertyValue("--tp-primary")).toBe("#0A84FF");
    theme.setPreset("light");
    await vi.waitFor(() => expect(document.documentElement.style.getPropertyValue("--tp-primary")).toBe("#0066CC"));
  });

  it("preserves a saved accent instead of overwriting it with blue", async () => {
    localStorage.setItem("ai-companion-appearance", JSON.stringify({ accentColor: "#52B788" }));
    const { useTheme } = await import("../composables/useTheme");
    expect(useTheme().state.value.accentColor).toBe("#52B788");
    expect(document.documentElement.style.getPropertyValue("--tp-primary")).toBe("#52B788");
    expect(document.documentElement.style.getPropertyValue("--tp-action-text")).toBe("#000000");
  });

  it("updates the previous default blue and adapts button text across modes", async () => {
    localStorage.setItem("ai-companion-appearance", JSON.stringify({ accentColor: "#416FAE" }));
    const { useTheme } = await import("../composables/useTheme");
    const theme = useTheme();
    const style = document.documentElement.style;
    expect(theme.state.value.accentColor).toBe("#0066CC");
    expect(style.getPropertyValue("--tp-action-text")).toBe("#000000");
    theme.setPreset("light");
    await vi.waitFor(() => expect(style.getPropertyValue("--tp-action-text")).toBe("#FFFFFF"));
  });

  it("upgrades the saved default blue without changing a custom palette", async () => {
    localStorage.setItem("ai-companion-appearance", JSON.stringify({
      accentColor: "#6C8FEA",
      customPalette: { enabled: true, primary: "#6C8FEA" },
    }));
    const { useTheme } = await import("../composables/useTheme");
    const theme = useTheme();
    expect(theme.state.value.accentColor).toBe("#0066CC");
    expect(document.documentElement.style.getPropertyValue("--tp-primary")).toBe("#6C8FEA");
    theme.setCustomPalette({ enabled: false });
    expect(document.documentElement.style.getPropertyValue("--tp-primary")).toBe("#0A84FF");
  });

  it("keeps dark tinted surfaces dark and refreshes them when switching modes", async () => {
    const { useTheme } = await import("../composables/useTheme");
    const theme = useTheme();
    const style = document.documentElement.style;
    expect(style.getPropertyValue("--tp-primary-light-9")).toBe("#1a2635");
    expect(style.getPropertyValue("--tp-primary-hover")).toBe("#3198ff");
    expect(style.getPropertyValue("--tp-secondary")).toBe("#0A84FF");
    expect(style.getPropertyValue("--tp-secondary-soft")).toBe("rgba(10, 132, 255, 0.16)");
    theme.setPreset("light");
    await vi.waitFor(() => expect(style.getPropertyValue("--tp-primary-light-9")).toBe("#e6f0fa"));
    expect(style.getPropertyValue("--tp-text-on-secondary")).toBe("#FFFFFF");
    theme.setPreset("dark");
    await vi.waitFor(() => expect(style.getPropertyValue("--tp-primary-light-9")).toBe("#1a2635"));
    expect(style.getPropertyValue("--tp-text-on-secondary")).toBe("#000000");
  });

  it("passes the current accent to sandbox theme tokens after a color change", async () => {
    const { useTheme } = await import("../composables/useTheme");
    const { buildSandboxThemeTokens } = await import("../components/extension/sandboxSessionCache");
    const theme = useTheme();
    expect(buildSandboxThemeTokens()["--amitia-color-accent"]).toBe("#0A84FF");
    theme.setAccentColor("#52B788");
    expect(buildSandboxThemeTokens()["--amitia-color-accent"]).toBe("#52B788");
  });
});

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
    expect(theme.state.value.accentColor).toBe("#416FAE");
    expect(document.documentElement.style.getPropertyValue("--tp-primary")).toBe("#82A7DC");
    theme.setPreset("light");
    await vi.waitFor(() => expect(document.documentElement.style.getPropertyValue("--tp-primary")).toBe("#416FAE"));
  });

  it("preserves a saved accent instead of overwriting it with blue", async () => {
    localStorage.setItem("ai-companion-appearance", JSON.stringify({ accentColor: "#52B788" }));
    const { useTheme } = await import("../composables/useTheme");
    expect(useTheme().state.value.accentColor).toBe("#52B788");
    expect(document.documentElement.style.getPropertyValue("--tp-primary")).toBe("#52B788");
    expect(document.documentElement.style.getPropertyValue("--tp-action-text")).toBe("#000000");
  });

  it("updates the previous default blue and adapts button text across modes", async () => {
    localStorage.setItem("ai-companion-appearance", JSON.stringify({ accentColor: "#0066CC" }));
    const { useTheme } = await import("../composables/useTheme");
    const theme = useTheme();
    const style = document.documentElement.style;
    expect(theme.state.value.accentColor).toBe("#416FAE");
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
    expect(theme.state.value.accentColor).toBe("#416FAE");
    expect(document.documentElement.style.getPropertyValue("--tp-primary")).toBe("#6C8FEA");
    theme.setCustomPalette({ enabled: false });
    expect(document.documentElement.style.getPropertyValue("--tp-primary")).toBe("#82A7DC");
  });

  it("keeps dark tinted surfaces dark and refreshes them when switching modes", async () => {
    const { useTheme } = await import("../composables/useTheme");
    const theme = useTheme();
    const style = document.documentElement.style;
    expect(style.getPropertyValue("--tp-primary-light-9")).toBe("#262a2f");
    expect(style.getPropertyValue("--tp-primary-hover")).toBe("#96b5e2");
    expect(style.getPropertyValue("--tp-secondary")).toBe("#82A7DC");
    expect(style.getPropertyValue("--tp-secondary-soft")).toBe("rgba(130, 167, 220, 0.16)");
    theme.setPreset("light");
    await vi.waitFor(() => expect(style.getPropertyValue("--tp-primary-light-9")).toBe("#ecf1f7"));
    expect(style.getPropertyValue("--tp-text-on-secondary")).toBe("#FFFFFF");
    theme.setPreset("dark");
    await vi.waitFor(() => expect(style.getPropertyValue("--tp-primary-light-9")).toBe("#262a2f"));
    expect(style.getPropertyValue("--tp-text-on-secondary")).toBe("#000000");
  });

  it("passes the current accent to sandbox theme tokens after a color change", async () => {
    const { useTheme } = await import("../composables/useTheme");
    const { buildSandboxThemeTokens } = await import("../components/extension/sandboxSessionCache");
    const theme = useTheme();
    expect(buildSandboxThemeTokens()["--amitia-color-accent"]).toBe("#82A7DC");
    theme.setAccentColor("#52B788");
    expect(buildSandboxThemeTokens()["--amitia-color-accent"]).toBe("#52B788");
  });
});

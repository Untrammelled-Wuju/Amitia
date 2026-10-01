import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

describe("default appearance", () => {
  beforeEach(() => {
    vi.resetModules();
    localStorage.clear();
  });
  afterEach(() => localStorage.clear());

  it("uses mobile blue in both light and dark mode", async () => {
    const { useTheme } = await import("../composables/useTheme");
    const theme = useTheme();
    expect(theme.state.value.accentColor).toBe("#6C8FEA");
    expect(document.documentElement.style.getPropertyValue("--tp-primary")).toBe("#8CA8F0");
    theme.setPreset("light");
    await vi.waitFor(() => expect(document.documentElement.style.getPropertyValue("--tp-primary")).toBe("#6C8FEA"));
  });

  it("preserves a saved accent instead of overwriting it with blue", async () => {
    localStorage.setItem("ai-companion-appearance", JSON.stringify({ accentColor: "#52B788" }));
    const { useTheme } = await import("../composables/useTheme");
    expect(useTheme().state.value.accentColor).toBe("#52B788");
    expect(document.documentElement.style.getPropertyValue("--tp-primary")).toBe("#52B788");
  });

  it("passes the current accent to sandbox theme tokens after a color change", async () => {
    const { useTheme } = await import("../composables/useTheme");
    const { buildSandboxThemeTokens } = await import("../components/extension/sandboxSessionCache");
    const theme = useTheme();
    expect(buildSandboxThemeTokens()["--amitia-color-accent"]).toBe("#8CA8F0");
    theme.setAccentColor("#52B788");
    expect(buildSandboxThemeTokens()["--amitia-color-accent"]).toBe("#52B788");
  });
});

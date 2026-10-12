import { beforeEach, describe, expect, it, vi } from "vitest";

describe("style profile persistence", () => {
  beforeEach(() => {
    vi.resetModules();
    localStorage.clear();
    document.documentElement.removeAttribute("style");
  });

  it("persists patches and revisions and supports history", async () => {
    const profile = await import("../ui-runtime/styleProfile");
    profile.initializeStyleProfile();
    profile.replaceStylePatch({ palette: { accentPrimary: "#52B788" } }, 2);
    profile.pushStyleHistory({ revision: 1, patch: {} });
    expect(profile.stylePatch.value).toEqual({ palette: { accentPrimary: "#52B788" } });
    expect(profile.styleRevision.value).toBe(2);
    expect(profile.popStyleHistory()).toEqual({ revision: 1, patch: {} });
    profile.clearStylePatch(3);
    expect(profile.stylePatch.value).toEqual({});
    expect(profile.styleRevision.value).toBe(3);
  });

  it("applies and clears local style tokens", async () => {
    const { applyStyleTokens } = await import("../ui-runtime/providerTheme");
    applyStyleTokens({ colors: { accentPrimary: "#52B788" }, radius: { md: 14 } });
    expect(document.documentElement.style.getPropertyValue("--ac-color-primary")).toBe("#52B788");
    expect(document.documentElement.style.getPropertyValue("--ui-radius-md")).toBe("14px");
    applyStyleTokens({});
    expect(document.documentElement.style.getPropertyValue("--ac-color-primary")).toBe("");
    expect(document.documentElement.style.getPropertyValue("--ui-radius-md")).toBe("");
  });

  it("filters rules and removes only matching properties and states", async () => {
    const profile = await import("../ui-runtime/styleProfile");
    profile.initializeStyleProfile();
    profile.addStyleRules([
      {
        id: "chat-input",
        source: "ai",
        scope: { platform: "desktop", page: "/chat" },
        target: "chat.composer.input",
        states: ["default", "focus"],
        properties: { backgroundColor: "#000000", foregroundColor: "#FFFFFF" },
      },
      {
        id: "settings-card",
        source: "ai",
        scope: { platform: "desktop", page: "/settings" },
        target: "settings.appearance.card",
        states: ["default"],
        properties: { borderRadius: 8 },
      },
    ]);
    const result = profile.removeStyleRules({
      target: "chat.composer.input",
      states: ["focus"],
      properties: ["backgroundColor"],
    });
    expect(result).toMatchObject({ removedRules: 0, changedRules: 1 });
    expect(profile.listStyleRules({ target: "chat.composer.input" })[0]).toMatchObject({
      states: ["default"],
      properties: { foregroundColor: "#FFFFFF" },
    });
    expect(profile.listStyleRules({ target: "settings.appearance.card" })).toHaveLength(1);
  });
});

import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => {
  const themeState = {
    preset: "dark",
    accentColor: "#0066CC",
    fontScale: 1,
    cornerStyle: 1,
    dynamicEffect: true,
    reduceAnimation: false,
    customPalette: { enabled: false, primary: "#0066CC", secondary: "#52B788", text: "#24221F", textMode: "auto" },
  };
  const backgroundState = {
    enabled: false,
    kind: "image",
    mode: "default",
    name: "",
    opacity: 0.35,
    blurEnabled: false,
    blurRadius: 10,
    color: "#F7F8FA",
    gradientStart: "#F7F8FA",
    gradientEnd: "#E8EDF5",
    gradientAngle: 180,
    sourceUri: "",
    fit: "cover",
    overlayColor: "#000000",
    overlayOpacity: 0,
  };
  return {
    themeState,
    backgroundState,
    theme: {
      snapshot: vi.fn(() => ({ ...themeState, customPalette: { ...themeState.customPalette } })),
      replaceState: vi.fn(),
      reset: vi.fn(),
      loadFromServer: vi.fn(async () => {}),
      setPreset: vi.fn((value: string) => { themeState.preset = value; }),
      setCustomPalette: vi.fn(),
      setFontScale: vi.fn(),
      setCornerStyle: vi.fn(),
      setDynamicEffect: vi.fn(),
      setReduceAnimation: vi.fn(),
    },
    background: {
      settings: { value: backgroundState },
      init: vi.fn(async () => {}),
      reset: vi.fn(async () => {}),
      replace: vi.fn(async (value: any) => { Object.assign(backgroundState, value); }),
      update: vi.fn(async (value: any) => { Object.assign(backgroundState, value); }),
      applySource: vi.fn(async () => {}),
    },
    readBackground: vi.fn(async () => ({ settings: { ...backgroundState }, media: null })),
    writeHistory: vi.fn(async () => {}),
    readHistory: vi.fn(async () => null),
  };
});

vi.mock("../composables/useTheme", () => ({
  useTheme: () => mocks.theme,
}));
vi.mock("../composables/useBackgroundPreference", () => ({
  useBackgroundPreference: () => mocks.background,
}));
vi.mock("../composables/backgroundStorage", async (importOriginal) => ({
  ...await importOriginal<typeof import("../composables/backgroundStorage")>(),
  readBackground: mocks.readBackground,
  writeBackgroundHistoryMedia: mocks.writeHistory,
  readBackgroundHistoryMedia: mocks.readHistory,
}));

describe("style command", () => {
  beforeEach(async () => {
    vi.resetModules();
    localStorage.clear();
    document.head.innerHTML = "";
    document.body.innerHTML = "";
    document.documentElement.dataset.amitiaPage = "/chat";
    document.documentElement.dataset.amitiaPlatform = "desktop";
    mocks.theme.setPreset.mockClear();
    mocks.background.replace.mockClear();
    mocks.readBackground.mockClear();
    const profile = await import("../ui-runtime/styleProfile");
    profile.initializeStyleProfile();
  });

  it("applies theme, tokens and background with a revision", async () => {
    const { executeStyleCommand } = await import("../ui-runtime/styleCommand");
    const result = await executeStyleCommand("apply", 0, {
      themeMode: "dark",
      palette: { accentPrimary: "#52B788" },
      background: {
        mode: "gradient",
        gradientStart: "#111214",
        gradientEnd: "#1A2635",
        opacity: 0.7,
      },
    });
    expect(result).toMatchObject({ ok: true, revision: 1 });
    expect(mocks.theme.setPreset).toHaveBeenCalledWith("dark");
    expect(mocks.background.replace).toHaveBeenCalled();
    const profile = await import("../ui-runtime/styleProfile");
    expect(profile.stylePatch.value).toMatchObject({ palette: { accentPrimary: "#52B788" } });
  });

  it("rejects stale revisions", async () => {
    const { executeStyleCommand } = await import("../ui-runtime/styleCommand");
    await expect(executeStyleCommand("apply", 4, { themeMode: "light" })).rejects.toThrow("revision conflict");
  });

  it("applies DOM target rules and inspects real elements", async () => {
    document.body.innerHTML = '<textarea data-amitia-ui="chat.composer.input"></textarea>';
    const { executeStyleCommand } = await import("../ui-runtime/styleCommand");
    const applied = await executeStyleCommand("apply", 0, {
      targets: [{
        scope: { platform: "desktop", page: "/chat" },
        target: "chat.composer.input",
        states: ["default", "focus"],
        properties: { backgroundColor: "#000000", foregroundColor: "#FFFFFF" },
      }],
    });
    expect(applied).toMatchObject({ ok: true, revision: 1, ruleCount: 1 });
    const runtimeCss = document.getElementById("amitia-ui-style-rules-v1")?.textContent ?? "";
    expect(runtimeCss).toContain('data-amitia-ui="chat.composer.input"');
    expect(runtimeCss).toContain("background-color: #000000");

    const inspected = await executeStyleCommand("inspect", 1, {
      scope: { platform: "desktop", page: "/chat" },
      target: "chat.composer.input",
      includeComputed: true,
    });
    expect(inspected).toMatchObject({
      ok: true,
      page: "chat",
      platform: "desktop",
      count: 1,
    });
    expect((inspected.targets as Array<Record<string, unknown>>)[0]).toMatchObject({
      target: "chat.composer.input",
      selector: 'body > [data-amitia-ui="chat.composer.input"]',
      tag: "textarea",
    });
  });

  it("resets only the requested target and keeps other pages", async () => {
    const { executeStyleCommand } = await import("../ui-runtime/styleCommand");
    await executeStyleCommand("apply", 0, {
      targets: [
        {
          scope: { page: "/chat" },
          target: "chat.composer.input",
          properties: { backgroundColor: "#000000" },
        },
        {
          scope: { page: "/settings" },
          target: "settings.appearance.card",
          properties: { borderRadius: 8 },
        },
      ],
    });
    const reset = await executeStyleCommand("reset", 1, {
      scope: { page: "/chat" },
      target: "chat.composer.input",
    });
    expect(reset).toMatchObject({ ok: true, revision: 2, partial: true, removedRules: 1 });
    const profile = await import("../ui-runtime/styleProfile");
    expect(profile.listStyleRules()).toHaveLength(1);
    expect(profile.listStyleRules()[0].target).toBe("settings.appearance.card");
  });

  it("keeps user overrides when a partial reset has no source filter", async () => {
    const { executeStyleCommand } = await import("../ui-runtime/styleCommand");
    await executeStyleCommand("apply", 0, {
      targets: [
        {
          source: "ai",
          target: "chat.composer.input",
          properties: { backgroundColor: "#000000" },
        },
        {
          source: "user",
          target: "chat.composer.input",
          properties: { foregroundColor: "#FFFFFF" },
        },
      ],
    });
    const reset = await executeStyleCommand("reset", 1, {
      target: "chat.composer.input",
      properties: ["backgroundColor"],
    });
    expect(reset).toMatchObject({ ok: true, partial: true });
    const profile = await import("../ui-runtime/styleProfile");
    const rules = profile.listStyleRules({ target: "chat.composer.input" });
    expect(rules).toHaveLength(1);
    expect(rules[0].source).toBe("user");
    expect(rules[0].properties).toEqual({ foregroundColor: "#FFFFFF" });
  });
});

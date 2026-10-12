import { describe, expect, it } from "vitest";
import { amitiaStyleBridgePayload } from "../ui-runtime/pluginStyleBridge";
import type { StyleRule } from "../ui-runtime/styleProfile";

function rule(overrides: Partial<StyleRule> = {}): StyleRule {
  return {
    id: "rule_plugin",
    source: "ai",
    scope: { platform: "desktop", page: "plugin.demo.page", surface: "main" },
    target: "extension.surface",
    states: ["default"],
    properties: { backgroundColor: "#000000", foregroundColor: "#FFFFFF" },
    createdAt: "2026-10-10T00:00:00.000Z",
    ...overrides,
  };
}

describe("plugin style bridge", () => {
  it("builds target variables for plugin web hosts", () => {
    const payload = amitiaStyleBridgePayload({
      rules: [rule()],
      scope: { platform: "desktop", page: "plugin.demo.page", surface: "main" },
      target: "plugin.web.demo",
      additionalTargets: ["extension.surface"],
    });
    expect(payload.target).toBe("plugin.web.demo");
    expect(payload.properties).toEqual({});
    expect((payload.targets as Record<string, { vars: Record<string, string> }>)["extension.surface"].vars)
      .toEqual({ "--amitia-background-color": "#000000", "--amitia-foreground-color": "#FFFFFF" });
  });

  it("removes unsafe structured values and serializes safe variables", () => {
    const payload = amitiaStyleBridgePayload({
      rules: [rule({
        properties: {
          backgroundColor: "#000000",
          padding: 12,
          backgroundImage: "url(javascript:alert(1))",
        },
      })],
      scope: { platform: "desktop", page: "plugin.demo.page", surface: "main" },
      target: "extension.surface",
    });
    expect(payload.vars).toEqual({
      "--amitia-background-color": "#000000",
      "--amitia-padding": "12",
    });
  });
});

import { describe, expect, it } from "vitest";
import { providerCompatibilityIssue } from "@/ui-runtime/providerRuntime";
import type { UIProviderDefinition } from "@/ui-runtime/types";

function provider(overrides: Partial<UIProviderDefinition> = {}): UIProviderDefinition {
  return {
    providerId: "provider",
    extensionId: "com.example/plugin",
    capability: "page.provider",
    mode: "replace",
    priority: 1,
    entries: { web: { type: "declarative" } },
    enabled: true,
    builtin: false,
    placement: "any",
    ...overrides,
  };
}

describe("provider compatibility reasons", () => {
  it("returns stable reasons for disabled, missing platform and device requirements", () => {
    expect(providerCompatibilityIssue(provider({ enabled: false }), { platform: "web", localRuntime: true }, "web"))
      .toBe("provider-disabled");
    expect(providerCompatibilityIssue(provider(), { platform: "android", localRuntime: true }, "android"))
      .toBe("entry-unavailable:android");
    expect(providerCompatibilityIssue(
      provider({
        entries: { windows: { type: "declarative" } },
        deviceRequirements: { platforms: ["macos"] },
      }),
      { platform: "windows", localRuntime: true },
      "windows",
    )).toBe("platform-unsupported:windows");
  });
});

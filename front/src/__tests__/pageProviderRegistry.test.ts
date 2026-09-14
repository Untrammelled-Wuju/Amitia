import { describe, expect, it } from "vitest";
import type { UIProviderDefinition } from "@/ui-runtime/types";
import { resolvePageProvider } from "@/ui-runtime/pageProviderRegistry";
import providerHostSource from "../components/ui-runtime/UIProviderHost.vue?raw";

function provider(input: Partial<UIProviderDefinition> & Pick<UIProviderDefinition, "providerId" | "extensionId" | "capability">): UIProviderDefinition {
  return {
    providerId: input.providerId,
    extensionId: input.extensionId,
    capability: input.capability,
    mode: input.mode ?? "replace",
    priority: input.priority ?? 0,
    entries: input.entries ?? { web: { type: "web_restricted", contributionId: "test-page" } },
    enabled: input.enabled ?? true,
    builtin: input.builtin ?? false,
    placement: input.placement ?? "any",
    metadata: input.metadata ?? {},
  };
}

const builtin = provider({
  providerId: "builtin.page",
  extensionId: "builtin.amitia.ui",
  capability: "page.provider",
  builtin: true,
  entries: { web: { type: "builtin_native", exportName: "builtin.page" } },
});

const emote = provider({
  providerId: "emote-page-provider",
  extensionId: "com.amitia/emote",
  capability: "page.provider",
  priority: 100,
  metadata: {
    routes: ["/emotes"],
    surfacePatterns: ["surface.chat"],
  },
});

describe("page provider registry", () => {
  it("keeps route-scoped providers out of unrelated surfaces", () => {
    expect(resolvePageProvider([emote, builtin], null, "page.provider", "/chat")?.providerId).toBe("builtin.page");
    expect(resolvePageProvider([emote, builtin], null, "page.provider", "/character")?.providerId).toBe("builtin.page");
  });

  it("selects a route-scoped provider on its declared route", () => {
    expect(resolvePageProvider([emote, builtin], null, "page.provider", "/emotes")?.providerId).toBe("emote-page-provider");
  });

  it("keeps provider slot rendering under provider selection", () => {
    expect(providerHostSource).toContain(':render-contributions="false"');
  });
});

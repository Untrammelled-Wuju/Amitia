import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it } from "vitest";
import { useExtensionUIStore, type UIContributionSnapshot } from "@/stores/extensionUI";

function snapshot(providerEnabled: boolean): UIContributionSnapshot {
  return {
    generatedAt: "2026-11-09T00:00:00Z",
    version: 1,
    providers: providerEnabled
      ? [{
          providerId: "page.provider",
          extensionId: "com.example/plugin",
          capability: "page.provider",
          mode: "replace",
          entries: { web: { type: "declarative" } },
          enabled: true,
          builtin: false,
        }]
      : [],
    providerResolutions: {
      "page.provider": {
        providerId: "page.provider",
        capability: "page.provider",
        compatible: false,
        reason: "platform-unsupported:android",
      },
    },
    slots: providerEnabled
      ? [{
          slotId: "provider.page.provider",
          contractVersion: 1,
          kind: "single",
          layout: "stack",
          fallbackPolicy: "default",
          generatedAt: "2026-11-09T00:00:00Z",
          contributions: [{
            contributionId: "plugin-page",
            extensionId: "com.example/plugin",
            moduleId: "main",
            kind: "page",
            slotId: "provider.page.provider",
            contractVersion: 1,
            generation: 1,
            ordering: 0,
            title: "Plugin page",
            visible: true,
            effective: true,
            enabled: true,
            runtimeReady: true,
          }],
        }]
      : [],
  };
}

describe("provider runtime snapshot", () => {
  beforeEach(() => setActivePinia(createPinia()));

  it("exposes per-provider incompatibility reasons", () => {
    const store = useExtensionUIStore();
    store.setSnapshot(snapshot(true));
    expect(store.getProviderCompatibility("page.provider")).toEqual({
      providerId: "page.provider",
      capability: "page.provider",
      compatible: false,
      reason: "platform-unsupported:android",
    });
  });

  it("removes slots and provider state after disable or uninstall refresh", () => {
    const store = useExtensionUIStore();
    store.setSnapshot(snapshot(true));
    expect(store.slotsById.has("provider.page.provider")).toBe(true);
    expect(store.getProviders("page.provider")).toHaveLength(1);
    store.setSnapshot(snapshot(false));
    expect(store.slotsById.has("provider.page.provider")).toBe(false);
    expect(store.getProviderCompatibility("page.provider")?.compatible).toBe(false);
    expect(store.getProviders("page.provider")).toEqual([]);
  });
});

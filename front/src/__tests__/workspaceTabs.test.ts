import { describe, expect, it } from "vitest";
import { collectWorkspaceTabs } from "@/ui-runtime/workspaceTabs";
import type { UIProviderDefinition } from "@/ui-runtime/types";
import type { useExtensionUIStore } from "@/stores/extensionUI";

function provider(overrides: Partial<UIProviderDefinition> = {}): UIProviderDefinition {
  return {
    providerId: "service-webui",
    extensionId: "com.example/service",
    moduleId: "service-ui",
    capability: "workspace.tab",
    mode: "replace",
    priority: 100,
    placement: "device",
    trustLevel: "official",
    enabled: true,
    entries: {
      electron_macos: {
        type: "external_web_ui",
        url: "http://127.0.0.1:6099/webui/",
        allowedOrigins: ["http://127.0.0.1:6099"],
      },
    },
    metadata: {
      workspace: "channel.workspace",
      tabId: "service-webui",
      title: "Service WebUI",
      order: 30,
      routePrefix: "/channels/example",
      contextMatch: { channelId: ["example"] },
      keepMounted: true,
      storage: "persistent",
    },
    ...overrides,
  };
}

function storeWith(providers: UIProviderDefinition[]): ReturnType<typeof useExtensionUIStore> {
  return {
    snapshot: {
      providerContext: {
        platform: "macos",
        localRuntime: true,
        deviceOnline: true,
        deviceId: "device",
      },
    },
    getProviders: (capability: string) =>
      capability === "workspace.tab" ? providers : [],
  } as unknown as ReturnType<typeof useExtensionUIStore>;
}

describe("workspace tabs", () => {
  it("resolves a matching external web UI tab", () => {
    const tabs = collectWorkspaceTabs(
      storeWith([provider()]),
      "channel.workspace",
      { channelId: "example" },
      "/channels/example",
      "macos",
    );
    expect(tabs).toHaveLength(1);
    expect(tabs[0]).toMatchObject({
      tabId: "service-webui",
      label: "Service WebUI",
      keepMounted: true,
      storage: "persistent",
    });
    expect(tabs[0].entry.type).toBe("external_web_ui");
  });

  it("rejects context and route mismatches", () => {
    const tabs = collectWorkspaceTabs(
      storeWith([provider()]),
      "channel.workspace",
      { channelId: "other" },
      "/channels/other",
      "macos",
    );
    expect(tabs).toEqual([]);
  });

  it("resolves the same tab contract for a non-channel workspace", () => {
    const tabs = collectWorkspaceTabs(
      storeWith([provider({
        metadata: {
          workspace: "settings.workspace",
          tabId: "service-webui",
          title: "Service WebUI",
          routePrefix: "/settings/advanced",
          contextMatch: { section: "advanced" },
        },
      })]),
      "settings.workspace",
      { section: "advanced" },
      "/settings/advanced",
      "macos",
    );
    expect(tabs).toHaveLength(1);
  });

  it("deduplicates competing tab ids deterministically", () => {
    const tabs = collectWorkspaceTabs(
      storeWith([
        provider({ providerId: "a-provider", priority: 200 }),
        provider({ providerId: "z-provider", priority: 100 }),
      ]),
      "channel.workspace",
      { channelId: "example" },
      "/channels/example",
      "macos",
    );
    expect(tabs.map((tab) => tab.provider.providerId)).toEqual(["a-provider"]);
  });
});

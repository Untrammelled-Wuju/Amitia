import { describe, expect, it } from "vitest";
import type { UIProviderDefinition } from "@/ui-runtime/types";
import {
  collectChannelPluginNavigationItems,
  collectExtensionNavigationItems,
} from "@/ui-runtime/navigationRegistry";
import {
  collectEffectiveProviderRoutes,
  collectExtensionSectionEntries,
} from "@/ui-runtime/providerRoutes";
import { resolveMessageRenderer } from "@/ui-runtime/messageRendererRegistry";

function provider(
  input: Partial<UIProviderDefinition> &
    Pick<UIProviderDefinition, "providerId" | "extensionId" | "capability">,
): UIProviderDefinition {
  return {
    ...input,
    mode: input.mode ?? "replace",
    priority: input.priority ?? 0,
    entries: input.entries ?? { web: { type: "declarative" } },
    enabled: input.enabled ?? true,
    builtin: input.builtin ?? false,
    placement: input.placement ?? "any",
    metadata: input.metadata ?? {},
  };
}

function store(providers: UIProviderDefinition[]) {
  return {
    snapshot: {
      providers,
      providerContext: { platform: "web" },
    },
    getProviders(capability?: string) {
      return capability
        ? providers.filter((item) => item.capability === capability)
        : providers;
    },
    getResolvedProvider() {
      return null;
    },
  } as any;
}

function pageProvider(extensionId: string, providerId: string) {
  return provider({
    providerId,
    extensionId,
    capability: "page.provider",
  });
}

function routeRegistry(
  extensionId: string,
  providerId: string,
  route: string,
  targetProviderId: string,
  priority: number,
  section?: string,
) {
  return provider({
    providerId,
    extensionId,
    capability: "route.registry",
    priority,
    metadata: {
      routes: [
        {
          id: "main",
          path: route,
          providerId: targetProviderId,
          capability: "page.provider",
          priority,
          title: extensionId,
        },
      ],
      navigationItems: [
        {
          id: "main",
          label: extensionId,
          route,
          icon: "extension",
          order: 100,
        },
      ],
      ...(section
        ? {
            sectionEntries: [
              {
                id: "main",
                section,
                label: extensionId,
                route,
                order: 10,
              },
            ],
          }
        : {}),
    },
  });
}

describe("third-party replacement matrix", () => {
  it("replaces an official page route without host-specific routing", () => {
    const providers = [
      routeRegistry(
        "com.amitia/workspace",
        "workspace-routes",
        "/workspaces",
        "workspace-page",
        250,
      ),
      pageProvider("com.amitia/workspace", "workspace-page"),
      routeRegistry(
        "com.example/workspace-pro",
        "workspace-pro-routes",
        "/workspaces",
        "workspace-pro-page",
        300,
      ),
      pageProvider("com.example/workspace-pro", "workspace-pro-page"),
    ];

    const effective = collectEffectiveProviderRoutes(store(providers));
    expect(effective.filter((item) => item.path === "/workspaces")).toEqual([
      expect.objectContaining({
        extensionId: "com.example/workspace-pro",
        providerId: "workspace-pro-page",
      }),
    ]);
  });

  it("replaces the official marketplace page and section entry", () => {
    const providers = [
      routeRegistry(
        "com.amitia/plugin-market",
        "plugin-market-routes",
        "/extensions/market",
        "plugin-market-page",
        260,
        "extensions",
      ),
      pageProvider("com.amitia/plugin-market", "plugin-market-page"),
      routeRegistry(
        "com.example/market-pro",
        "market-pro-routes",
        "/extensions/market",
        "market-pro-page",
        400,
        "extensions",
      ),
      pageProvider("com.example/market-pro", "market-pro-page"),
    ];

    const model = store(providers);
    expect(
      collectExtensionSectionEntries(model, "extensions").map((entry) => ({
        route: entry.route,
        extensionId: entry.extensionId,
      })),
    ).toEqual([
      {
        route: "/extensions/market",
        extensionId: "com.example/market-pro",
      },
    ]);
  });

  it("replaces an official channel route and presentation together", () => {
    const providers = [
      provider({
        providerId: "channel.wechat.official",
        extensionId: "com.amitia/channel-wechat",
        capability: "channel.presentation",
        priority: 100,
        metadata: {
          channelId: "wechat_personal",
          displayName: "官方微信",
        },
      }),
      routeRegistry(
        "com.amitia/channel-wechat",
        "wechat-routes",
        "/channels/wechat",
        "wechat-page",
        100,
      ),
      pageProvider("com.amitia/channel-wechat", "wechat-page"),
      provider({
        providerId: "channel.wechat.third-party",
        extensionId: "com.example/channel-wechat-pro",
        capability: "channel.presentation",
        priority: 300,
        metadata: {
          channelId: "wechat_personal",
          displayName: "第三方微信",
        },
      }),
      routeRegistry(
        "com.example/channel-wechat-pro",
        "wechat-pro-routes",
        "/channels/wechat",
        "wechat-pro-page",
        300,
      ),
      pageProvider("com.example/channel-wechat-pro", "wechat-pro-page"),
    ];

    const model = store(providers);
    expect(
      collectExtensionNavigationItems(model).map((item) => item.extensionId),
    ).toEqual(["com.example/channel-wechat-pro"]);
    expect(collectChannelPluginNavigationItems(model)).toEqual([
      {
        id: "channel:com.example/channel-wechat-pro:wechat_personal",
        route: "/channels/wechat",
        label: "第三方微信",
        channelId: "wechat_personal",
        extensionId: "com.example/channel-wechat-pro",
      },
    ]);
  });

  it("replaces an official channel message renderer by selector priority", () => {
    const official = provider({
      providerId: "renderer.wechat.official",
      extensionId: "com.amitia/channel-wechat",
      capability: "conversation.message_renderer",
      priority: 100,
      metadata: {
        channelIds: ["wechat_personal"],
        messageTypes: ["text"],
      },
    });
    const thirdParty = provider({
      providerId: "renderer.wechat.third-party",
      extensionId: "com.example/channel-wechat-pro",
      capability: "conversation.message_renderer",
      priority: 300,
      metadata: {
        channelIds: ["wechat_personal"],
        messageTypes: ["text"],
      },
    });

    expect(
      resolveMessageRenderer(
        [official, thirdParty],
        null,
        {
          channelId: "wechat_personal",
          type: "text",
        },
        { platform: "web" } as any,
      )?.extensionId,
    ).toBe("com.example/channel-wechat-pro");
  });
});

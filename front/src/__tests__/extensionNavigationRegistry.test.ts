import { describe, expect, it } from "vitest";
import { createMemoryHistory, createRouter } from "vue-router";
import type { UIProviderDefinition } from "@/ui-runtime/types";
import {
  collectChannelPluginNavigationItems,
  collectExtensionNavigationItems,
  resolveBuiltinNavigationItems,
} from "@/ui-runtime/navigationRegistry";
import {
  collectEffectiveProviderRoutes,
  collectExtensionSectionEntries,
  shadowsProtectedRoute,
  syncProviderRoutes,
} from "@/ui-runtime/providerRoutes";
import { builtinBusinessRoutes } from "@/router/builtinRoutes";

function provider(input: Partial<UIProviderDefinition> & Pick<UIProviderDefinition, "providerId" | "extensionId" | "capability">): UIProviderDefinition {
  return {
    ...input,
    providerId: input.providerId,
    extensionId: input.extensionId,
    capability: input.capability,
    mode: input.mode ?? "augment",
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
      return capability ? providers.filter((item) => item.capability === capability) : providers;
    },
    getResolvedProvider() {
      return null;
    },
  } as any;
}

function routeRegistry(
  extensionId: string,
  providerId: string,
  route: string,
  pageProviderId: string,
  priority: number,
  includeNavigation = true,
) {
  return provider({
    providerId,
    extensionId,
    capability: "route.registry",
    priority,
    metadata: {
      routes: [{ id: "main", path: route, providerId: pageProviderId, capability: "page.provider" }],
      ...(includeNavigation
        ? { navigationItems: [{ id: "main", label: extensionId, route, icon: "extension", order: 100 }] }
        : {}),
    },
  });
}

function pageProvider(extensionId: string, providerId: string) {
  return provider({ providerId, extensionId, capability: "page.provider" });
}

describe("extension navigation registry", () => {
  it("keeps dashboard entries out of the sidebar", () => {
    const items = resolveBuiltinNavigationItems(store([]));
    expect(items.some((item) => item.route.startsWith("/dashboard"))).toBe(false);
  });

  it("resolves character, memory and workshop hubs from plugins", () => {
    const items = resolveBuiltinNavigationItems(store([]));
    expect(items.filter((item) => item.group === "character")).toEqual([]);
    expect(items.filter((item) => item.group === "memory")).toEqual([]);
    expect(items.filter((item) => item.group === "workshop")).toEqual([]);
    const characterStore = store([
      provider({
        providerId: "character-hub-routes",
        extensionId: "com.amitia/character-hub",
        capability: "route.registry",
        priority: 230,
        metadata: {
          routes: [
            { id: "character-hub", path: "/characters", providerId: "character-hub-page-provider", capability: "page.provider" },
          ],
          navigationItems: [
            { id: "character-hub", label: "角色", route: "/characters", icon: "character", group: "character", order: 20 },
          ],
        },
      }),
      pageProvider("com.amitia/character-hub", "character-hub-page-provider"),
    ]);
    const characterItems = collectExtensionNavigationItems(characterStore);
    expect(characterItems.filter((item) => item.route === "/characters").map((item) => ({
      route: item.route,
      label: item.label,
    }))).toEqual([{ route: "/characters", label: "角色" }]);
    const memoryStore = store([
      provider({
        providerId: "memory-hub-routes",
        extensionId: "com.amitia/memory-hub",
        capability: "route.registry",
        priority: 220,
        metadata: {
          routes: [
            { id: "memory-hub", path: "/memory", providerId: "memory-hub-page-provider", capability: "page.provider" },
          ],
          navigationItems: [
            { id: "memory-hub", label: "记忆", route: "/memory", icon: "memory", group: "memory", order: 50 },
          ],
        },
      }),
      pageProvider("com.amitia/memory-hub", "memory-hub-page-provider"),
    ]);
    const memoryItems = collectExtensionNavigationItems(memoryStore);
    expect(memoryItems.filter((item) => item.route === "/memory").map((item) => ({
      route: item.route,
      label: item.label,
    }))).toEqual([{ route: "/memory", label: "记忆" }]);
    const workshopStore = store([
      provider({
        providerId: "creative-workshop-hub-routes",
        extensionId: "com.amitia/creative-workshop-hub",
        capability: "route.registry",
        priority: 235,
        metadata: {
          routes: [
            { id: "creative-workshop-hub", path: "/creative-workshop", providerId: "creative-workshop-hub-page-provider", capability: "page.provider" },
          ],
          navigationItems: [
            { id: "creative-workshop-hub", label: "创意工坊", route: "/creative-workshop", icon: "workshop", group: "workshop", order: 85 },
          ],
        },
      }),
      pageProvider("com.amitia/creative-workshop-hub", "creative-workshop-hub-page-provider"),
    ]);
    const workshopItems = collectExtensionNavigationItems(workshopStore);
    expect(workshopItems.filter((item) => item.route === "/creative-workshop").map((item) => ({
      route: item.route,
      label: item.label,
    }))).toEqual([{ route: "/creative-workshop", label: "创意工坊" }]);
    const gameStore = store([
      provider({
        providerId: "game-center-hub-routes",
        extensionId: "com.amitia/game-center-hub",
        capability: "route.registry",
        priority: 240,
        metadata: {
          routes: [
            { id: "game-center-hub", path: "/game-center", providerId: "game-center-hub-page-provider", capability: "page.provider" },
          ],
          navigationItems: [
            { id: "game-center-hub", label: "游戏模式", route: "/game-center", icon: "gamepad", group: "workshop", order: 80 },
          ],
        },
      }),
      pageProvider("com.amitia/game-center-hub", "game-center-hub-page-provider"),
    ]);
    expect(collectEffectiveProviderRoutes(gameStore)).toContainEqual(
      expect.objectContaining({
        path: "/game-center",
        extensionId: "com.amitia/game-center-hub",
        providerId: "game-center-hub-page-provider",
      }),
    );
    expect(items.filter((item) => item.group === "tasks").map((item) => ({
      route: item.route,
      label: item.label,
      match: item.match,
    }))).toEqual([{ route: "/continuity", label: "持续事项", match: ["/continuity"] }]);
  });

  it("keeps channel center out of built-in navigation and lists plugins separately", () => {
    expect(resolveBuiltinNavigationItems(store([])).some((item) => item.route === "/channel-center")).toBe(false);
    const channelStore = store([
      provider({
        providerId: "channel.wechat.presentation",
        extensionId: "com.amitia/channel-wechat",
        capability: "channel.presentation",
        metadata: { channelId: "wechat_personal", displayName: "个人微信" },
      }),
      routeRegistry("com.amitia/channel-wechat", "wechat-routes", "/channels/wechat", "wechat-page", 100),
      pageProvider("com.amitia/channel-wechat", "wechat-page"),
    ]);
    expect(resolveBuiltinNavigationItems(channelStore).some((item) => item.route === "/channel-center")).toBe(false);
    expect(collectChannelPluginNavigationItems(channelStore)).toEqual([{
      id: "channel:com.amitia/channel-wechat:wechat_personal",
      route: "/channels/wechat",
      label: "个人微信",
      channelId: "wechat_personal",
      extensionId: "com.amitia/channel-wechat",
    }]);
  });

  it("aggregates navigation items from multiple enabled providers", () => {
    const providers = [
      provider({
        providerId: "nav.minecraft",
        extensionId: "minecraft",
        capability: "app.navigation",
        priority: 10,
        metadata: { navigationItems: [{ id: "main", label: "Minecraft", route: "/games/minecraft" }] },
      }),
      routeRegistry("minecraft", "routes.minecraft", "/games/minecraft", "page.minecraft", 10, false),
      pageProvider("minecraft", "page.minecraft"),
      provider({
        providerId: "nav.stardew",
        extensionId: "stardew",
        capability: "app.navigation",
        priority: 5,
        metadata: { navigationItems: [{ id: "main", label: "Stardew", route: "/games/stardew" }] },
      }),
      routeRegistry("stardew", "routes.stardew", "/games/stardew", "page.stardew", 5, false),
      pageProvider("stardew", "page.stardew"),
    ];

    expect(collectExtensionNavigationItems(store(providers)).map((item) => item.label)).toEqual([
      "Minecraft",
      "Stardew",
    ]);
  });

  it("only shows the navigation item for the route that wins a path conflict", () => {
    const providers = [
      routeRegistry("minecraft", "routes.minecraft", "/games/shared", "page.minecraft", 100),
      pageProvider("minecraft", "page.minecraft"),
      routeRegistry("stardew", "routes.stardew", "/games/shared", "page.stardew", 10),
      pageProvider("stardew", "page.stardew"),
    ];

    const effective = collectEffectiveProviderRoutes(store(providers));
    expect(effective).toHaveLength(1);
    expect(effective[0]?.extensionId).toBe("minecraft");
    expect(collectExtensionNavigationItems(store(providers)).map((item) => item.extensionId)).toEqual(["minecraft"]);
  });

  it("rejects cross-extension route provider bindings", () => {
    const providers = [
      routeRegistry("minecraft", "routes.minecraft", "/games/minecraft", "page.stardew", 100),
      pageProvider("stardew", "page.stardew"),
    ];

    expect(collectEffectiveProviderRoutes(store(providers))).toEqual([]);
    expect(collectExtensionNavigationItems(store(providers))).toEqual([]);
  });

  it("hides app.navigation links when their extension route is no longer effective", () => {
    const providers = [
      provider({
        providerId: "nav.minecraft",
        extensionId: "minecraft",
        capability: "app.navigation",
        metadata: { navigationItems: [{ id: "main", label: "Minecraft", route: "/games/shared" }] },
      }),
      routeRegistry("minecraft", "routes.minecraft", "/games/shared", "page.minecraft", 1, false),
      pageProvider("minecraft", "page.minecraft"),
      routeRegistry("stardew", "routes.stardew", "/games/shared", "page.stardew", 100, false),
      pageProvider("stardew", "page.stardew"),
    ];

    expect(collectExtensionNavigationItems(store(providers))).toEqual([]);
  });

  it("protects every built-in top-level route namespace", () => {
    expect(shadowsProtectedRoute("/chat/plugin-page")).toBe(true);
    expect(shadowsProtectedRoute("/game-center/plugin-page")).toBe(false);
    expect(shadowsProtectedRoute("/characters")).toBe(false);
    expect(shadowsProtectedRoute("/characters/plugin-page")).toBe(false);
    expect(shadowsProtectedRoute("/creative-workshop/plugin-page")).toBe(false);
    expect(shadowsProtectedRoute("/extensions/market")).toBe(false);
    expect(shadowsProtectedRoute("/extensions")).toBe(false);
    expect(shadowsProtectedRoute("/extensions/another-provider")).toBe(false);
    expect(shadowsProtectedRoute("/games/minecraft")).toBe(false);
    expect(collectEffectiveProviderRoutes(store([
      routeRegistry("bad", "routes.bad", "/:pathMatch(.*)*", "page.bad", 100),
      pageProvider("bad", "page.bad"),
    ]))).toEqual([]);
  });

  it("allows any provider to mount children under an open built-in shell", () => {
    const router = createRouter({ history: createMemoryHistory(), routes: [] });
    const providers = [
      routeRegistry("com.example/third-party", "third-party-routes", "/extensions/third-party", "third-party-page", 100, false),
      pageProvider("com.example/third-party", "third-party-page"),
    ];
    expect(collectEffectiveProviderRoutes(store(providers))).toEqual([
      expect.objectContaining({
        path: "/extensions/third-party",
        extensionId: "com.example/third-party",
      }),
    ]);
    syncProviderRoutes(router, store(providers));
    expect(router.getRoutes().some((route) => route.path === "/extensions/third-party")).toBe(true);
  });

  it("allows a third-party provider to replace the official workspace page", () => {
    const providers = [
      routeRegistry("com.amitia/workspace", "workspace-routes", "/workspaces", "workspace-page-provider", 250, false),
      pageProvider("com.amitia/workspace", "workspace-page-provider"),
      routeRegistry("com.example/workspace-pro", "workspace-pro-routes", "/workspaces", "workspace-pro-page", 300, false),
      pageProvider("com.example/workspace-pro", "workspace-pro-page"),
    ];
    expect(shadowsProtectedRoute("/workspaces")).toBe(false);
    expect(collectEffectiveProviderRoutes(store(providers))).toContainEqual(
      expect.objectContaining({
        path: "/workspaces",
        extensionId: "com.example/workspace-pro",
        providerId: "workspace-pro-page",
      }),
    );
  });

  it("renders plugin market only as an extension center section entry", () => {
    const providers = [
      pageProvider("com.amitia/plugin-market", "plugin-market-page-provider"),
      provider({
        providerId: "plugin-market-routes",
        extensionId: "com.amitia/plugin-market",
        capability: "route.registry",
        priority: 260,
        metadata: {
          routes: [{
            id: "plugin-market",
            path: "/extensions/market",
            providerId: "plugin-market-page-provider",
            capability: "page.provider",
            title: "插件市场",
            priority: 260,
          }],
          sectionEntries: [{
            id: "plugin-market",
            section: "extensions",
            label: "插件市场",
            route: "/extensions/market",
            order: 10,
          }],
        },
      }),
    ];
    const sectionEntries = collectExtensionSectionEntries(store(providers), "extensions");
    expect(sectionEntries).toEqual([
      expect.objectContaining({
        route: "/extensions/market",
        label: "插件市场",
        extensionId: "com.amitia/plugin-market",
      }),
    ]);
    expect(collectExtensionNavigationItems(store(providers))).toEqual([]);
  });

  it("keeps extension pages in the existing character navigation group", () => {
    const providers = [
      routeRegistry("com.amitia/emote", "routes.emote", "/games/emote-manager", "page.emote", 50, false),
      pageProvider("com.amitia/emote", "page.emote"),
      provider({
        providerId: "nav.emote",
        extensionId: "com.amitia/emote",
        capability: "app.navigation",
        metadata: {
          navigationItems: [{
            id: "manager",
            label: "表情包管理",
            route: "/games/emote-manager",
            icon: "star",
            group: "character",
            groupLabel: "角色",
            groupIcon: "character",
          }],
        },
      }),
    ];

    const item = collectExtensionNavigationItems(store(providers))[0];
    expect(item?.group).toBe("character");
    expect(item?.groupLabel).toBe("角色");
    expect(item?.label).toBe("表情包管理");
  });

  it("parses provider and simple sidebar declarations from navigation metadata", () => {
    const providers = [
      routeRegistry("com.amitia/emote", "routes.emote", "/games/emote-manager", "page.emote", 50, false),
      pageProvider("com.amitia/emote", "page.emote"),
      provider({
        providerId: "nav.emote",
        extensionId: "com.amitia/emote",
        capability: "app.navigation",
        metadata: {
          navigationItems: [{
            id: "manager",
            label: "表情包管理",
            route: "/games/emote-manager",
            containerId: "emote-workspace",
            sidebarProviderId: "emote-workspace-sidebar-provider",
            sidebarItems: [
              { label: "表情", route: "/games/emote-manager?tab=emotes" },
              { label: "设置", route: "/games/emote-manager?tab=settings" },
            ],
          }],
        },
      }),
    ];

    expect(collectExtensionNavigationItems(store(providers))[0]).toMatchObject({
      containerId: "emote-workspace",
      sidebarProviderId: "emote-workspace-sidebar-provider",
      sidebarItems: [
        { label: "表情", route: "/games/emote-manager?tab=emotes" },
        { label: "设置", route: "/games/emote-manager?tab=settings" },
      ],
    });
  });

  it("marks extension provider routes as full height", () => {
    const router = createRouter({ history: createMemoryHistory(), routes: [] });
    syncProviderRoutes(router, store([
      routeRegistry("minecraft", "routes.minecraft", "/games/minecraft", "page.minecraft", 10, false),
      pageProvider("minecraft", "page.minecraft"),
    ]));

    const registered = router.getRoutes().find((route) => route.path === "/games/minecraft");
    expect(registered?.meta.fullHeight).toBe(true);
    expect(registered?.meta.extensionRoute).toBe(true);
  });

  it("resolves provider routes when they are registered before the first navigation", async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: "/:pathMatch(.*)*", name: "catchAll", component: { template: "<div />" } }],
    });
    syncProviderRoutes(router, store([
      routeRegistry("minecraft", "routes.minecraft", "/games/minecraft", "page.minecraft", 10, false),
      pageProvider("minecraft", "page.minecraft"),
    ]));

    await router.push("/games/minecraft");

    expect(router.currentRoute.value.path).toBe("/games/minecraft");
    expect(router.currentRoute.value.name).not.toBe("catchAll");
  });

  it("keeps provider routes when a transient snapshot refresh has no authoritative result", () => {
    const router = createRouter({ history: createMemoryHistory(), routes: [] });
    syncProviderRoutes(router, store([
      routeRegistry("minecraft", "routes.minecraft", "/games/minecraft", "page.minecraft", 10, false),
      pageProvider("minecraft", "page.minecraft"),
    ]));
    const transientStore = { ...store([]), snapshot: null } as any;
    syncProviderRoutes(router, transientStore);
    expect(router.hasRoute("ui-provider:minecraft:routes.minecraft:main")).toBe(true);
  });

  it("removes character-scoped task, memory, and timeline routes", () => {
    expect(builtinBusinessRoutes.some((item) => item.path === "/characters/:id/active-task")).toBe(false);
    expect(builtinBusinessRoutes.some((item) => item.path === "/characters/:id/proactive")).toBe(false);
    expect(builtinBusinessRoutes.some((item) => item.path === "/character/:id/memory")).toBe(false);
    expect(builtinBusinessRoutes.some((item) => item.path === "/character/:id/timeline")).toBe(false);
  });
});

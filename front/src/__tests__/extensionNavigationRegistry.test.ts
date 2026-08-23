import { describe, expect, it } from "vitest";
import type { UIProviderDefinition } from "@/ui-runtime/types";
import { collectExtensionNavigationItems } from "@/ui-runtime/navigationRegistry";
import { collectEffectiveProviderRoutes, shadowsProtectedRoute } from "@/ui-runtime/providerRoutes";

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
    expect(shadowsProtectedRoute("/game-center/plugin-page")).toBe(true);
    expect(shadowsProtectedRoute("/creative-workshop/plugin-page")).toBe(true);
    expect(shadowsProtectedRoute("/games/minecraft")).toBe(false);
    expect(collectEffectiveProviderRoutes(store([
      routeRegistry("bad", "routes.bad", "/:pathMatch(.*)*", "page.bad", 100),
      pageProvider("bad", "page.bad"),
    ]))).toEqual([]);
  });
});

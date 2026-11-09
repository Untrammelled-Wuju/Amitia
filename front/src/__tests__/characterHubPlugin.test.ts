import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { createMemoryHistory, createRouter } from "vue-router";
import { describe, expect, it } from "vitest";
import { builtinBusinessRoutes } from "@/router/builtinRoutes";
import {
  shadowsProtectedRoute,
  syncProviderRoutes,
} from "@/ui-runtime/providerRoutes";
import type { UIProviderDefinition } from "@/ui-runtime/types";

function provider(
  providerId: string,
  capability: UIProviderDefinition["capability"],
  enabled = true,
  metadata: Record<string, unknown> = {},
): UIProviderDefinition {
  return {
    providerId,
    extensionId: "com.amitia/character-hub",
    capability,
    mode: "replace",
    priority: 230,
    entries: {
      web: capability === "page.provider"
        ? { type: "schema_renderer", contributionId: "character-hub-page" }
        : { type: "declarative" },
    },
    enabled,
    metadata,
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

describe("character hub plugin migration", () => {
  it("moves /characters to the plugin while keeping child routes open", () => {
    expect(builtinBusinessRoutes.some((route) => route.path === "/characters")).toBe(false);
    expect(shadowsProtectedRoute("/characters")).toBe(false);
    const manifest = JSON.parse(readFileSync(
      resolve(__dirname, "../../../plugins-builtin/character-hub/amitia-extension.json"),
      "utf8",
    )) as {
      modules: Array<{ contributions: Array<{ spec?: { metadata?: { routes?: Array<{ path: string }>; navigationItems?: Array<{ route: string }>; sectionEntries?: Array<{ route: string }> } } }> }>;
    };
    const metadata = manifest.modules
      .flatMap((module) => module.contributions)
      .map((contribution) => contribution.spec?.metadata)
      .filter(Boolean) as Array<{ routes?: Array<{ path: string }>; navigationItems?: Array<{ route: string }>; sectionEntries?: Array<{ route: string }> }>;
    expect(metadata.flatMap((item) => item.routes ?? []).map((route) => route.path)).toEqual(expect.arrayContaining([
      "/characters",
      "/characters/teams",
      "/character",
      "/character/:id",
      "/character/:id/life-rules",
      "/character/:id/voice",
      "/character/:id/psyche",
      "/world-book",
    ]));
    expect(metadata.flatMap((item) => item.navigationItems ?? []).map((item) => item.route)).toContain("/characters");
    expect(metadata.flatMap((item) => item.sectionEntries ?? []).map((item) => item.route)).toEqual(expect.arrayContaining([
      "/character",
      "/characters/teams",
      "/world-book",
    ]));
  });

  it("removes the hub route when the plugin page provider is disabled", () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: "/", component: { template: "<div />" } }],
    });
    const registry = provider("character-hub-routes", "route.registry", true, {
      routes: [{
        id: "character-hub",
        path: "/characters",
        providerId: "character-hub-page-provider",
        capability: "page.provider",
      }],
    });
    syncProviderRoutes(router, store([registry, provider("character-hub-page-provider", "page.provider")]));
    expect(router.getRoutes().some((route) => route.path === "/characters")).toBe(true);
    syncProviderRoutes(router, store([registry, provider("character-hub-page-provider", "page.provider", false)]));
    expect(router.getRoutes().some((route) => route.path === "/characters")).toBe(false);
  });
});

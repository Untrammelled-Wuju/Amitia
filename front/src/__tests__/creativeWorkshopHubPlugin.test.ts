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
    extensionId: "com.amitia/creative-workshop-hub",
    capability,
    mode: "replace",
    priority: 235,
    entries: {
      web: capability === "page.provider"
        ? { type: "schema_renderer", contributionId: "creative-workshop-hub-page" }
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

describe("creative workshop hub plugin migration", () => {
  it("moves /creative-workshop to the plugin while keeping child routes open", () => {
    expect(builtinBusinessRoutes.some((route) => route.path === "/creative-workshop")).toBe(false);
    expect(shadowsProtectedRoute("/creative-workshop")).toBe(false);
    const manifest = JSON.parse(readFileSync(
      resolve(__dirname, "../../../plugins-builtin/creative-workshop-hub/amitia-extension.json"),
      "utf8",
    )) as {
      modules: Array<{ contributions: Array<{ spec?: { metadata?: { routes?: Array<{ path: string }>; navigationItems?: Array<{ route: string }> } } }> }>;
    };
    const metadata = manifest.modules
      .flatMap((module) => module.contributions)
      .map((contribution) => contribution.spec?.metadata)
      .filter(Boolean) as Array<{ routes?: Array<{ path: string }>; navigationItems?: Array<{ route: string }> }>;
    expect(metadata.flatMap((item) => item.routes ?? []).map((route) => route.path)).toEqual(
      expect.arrayContaining(["/creative-workshop", "/creative-workshop/pet"]),
    );
    expect(metadata.flatMap((item) => item.navigationItems ?? []).map((item) => item.route)).toContain("/creative-workshop");
  });

  it("removes the hub route when the plugin page provider is disabled", () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: "/", component: { template: "<div />" } }],
    });
    const registry = provider("creative-workshop-hub-routes", "route.registry", true, {
      routes: [{
        id: "creative-workshop-hub",
        path: "/creative-workshop",
        providerId: "creative-workshop-hub-page-provider",
        capability: "page.provider",
      }],
    });
    syncProviderRoutes(router, store([registry, provider("creative-workshop-hub-page-provider", "page.provider")]));
    expect(router.getRoutes().some((route) => route.path === "/creative-workshop")).toBe(true);
    syncProviderRoutes(router, store([registry, provider("creative-workshop-hub-page-provider", "page.provider", false)]));
    expect(router.getRoutes().some((route) => route.path === "/creative-workshop")).toBe(false);
  });
});

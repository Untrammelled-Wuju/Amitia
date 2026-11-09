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
    extensionId: "com.amitia/memory-hub",
    capability,
    mode: "replace",
    priority: 220,
    entries: {
      web: capability === "page.provider"
        ? { type: "schema_renderer", contributionId: "memory-hub-page" }
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

describe("memory hub plugin migration", () => {
  it("moves /memory from the host router to the plugin route registry", () => {
    expect(builtinBusinessRoutes.some((route) => route.path === "/memory")).toBe(false);
    expect(shadowsProtectedRoute("/memory")).toBe(false);
    const manifest = JSON.parse(readFileSync(
      resolve(__dirname, "../../../plugins-builtin/memory-hub/amitia-extension.json"),
      "utf8",
    )) as {
      modules: Array<{ contributions: Array<{ spec?: { metadata?: { routes?: Array<{ path: string }>; navigationItems?: Array<{ route: string; label: string }>; sectionEntries?: Array<{ route: string; label: string }> } } }> }>;
    };
    const metadata = manifest.modules
      .flatMap((module) => module.contributions)
      .map((contribution) => contribution.spec?.metadata)
      .filter(Boolean) as Array<{ routes?: Array<{ path: string }>; navigationItems?: Array<{ route: string; label: string }>; sectionEntries?: Array<{ route: string; label: string }> }>;
    expect(metadata.flatMap((item) => item.routes ?? []).map((route) => route.path)).toEqual(expect.arrayContaining([
      "/memory",
      "/memory-manager",
      "/memory-timeline",
      "/episodic",
      "/graph",
      "/knowledge",
      "/profiles",
    ]));
    expect(metadata.flatMap((item) => item.navigationItems ?? []).map((item) => item.route)).toContain("/memory");
    expect(metadata.flatMap((item) => item.sectionEntries ?? []).map((item) => item.route)).toEqual(expect.arrayContaining([
      "/knowledge",
      "/memory-manager",
      "/episodic",
      "/graph",
      "/memory-timeline",
      "/profiles",
    ]));
  });

  it("removes the route when the plugin page provider is disabled", async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: "/", component: { template: "<div />" } }],
    });
    const registry = provider("memory-hub-routes", "route.registry", true, {
      routes: [{
        id: "memory-hub",
        path: "/memory",
        providerId: "memory-hub-page-provider",
        capability: "page.provider",
      }],
    });
    const page = provider("memory-hub-page-provider", "page.provider");
    syncProviderRoutes(router, store([registry, page]));
    expect(router.getRoutes().some((route) => route.path === "/memory")).toBe(true);
    syncProviderRoutes(router, store([registry, provider("memory-hub-page-provider", "page.provider", false)]));
    expect(router.getRoutes().some((route) => route.path === "/memory")).toBe(false);
    await router.push("/memory").catch(() => undefined);
    expect(router.currentRoute.value.matched.some((route) => route.path === "/memory")).toBe(false);
  });
});

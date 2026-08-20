import type { RouteRecordNormalized, Router } from "vue-router";
import ProviderRouteHost from "@/components/ui-runtime/ProviderRouteHost.vue";
import type { useExtensionUIStore } from "@/stores/extensionUI";
import { isUIProviderCapability, type UIProviderCapability, type UIProviderDefinition } from "./types";
import { collectRegistryProviders } from "./providerCollection";
import { isProviderCompatible } from "./providerRuntime";
import { resolveHostEnvironment } from "@/composables/useHostEnvironment";

type ExtensionUIStore = ReturnType<typeof useExtensionUIStore>;

interface ProviderRouteDefinition {
  id: string;
  path: string;
  providerId: string;
  extensionId: string;
  capability?: UIProviderCapability;
  title?: string;
  priority: number;
}

const PROVIDER_ROUTE_PREFIX = "ui-provider:";
const registeredRouteNames = new Set<string>();

const PROTECTED_ROUTE_PREFIXES = [
  "/onboarding", "/login", "/setup", "/privacy", "/usage-boundary",
  "/chat", "/character", "/memory", "/memory-manager", "/memory-timeline",
  "/settings", "/extensions", "/extension/page",
];

function shadowsProtectedRoute(path: string): boolean {
  return PROTECTED_ROUTE_PREFIXES.some((prefix) => path === prefix || path.startsWith(`${prefix}/`));
}

function providerRoutes(provider: UIProviderDefinition): ProviderRouteDefinition[] {
  if (!provider.enabled || provider.builtin || provider.capability !== "route.registry") return [];
  const rawRoutes = provider.metadata?.routes;
  if (!Array.isArray(rawRoutes)) return [];
  const routes: ProviderRouteDefinition[] = [];
  for (const item of rawRoutes) {
    if (!item || typeof item !== "object") continue;
    const row = item as Record<string, unknown>;
    const id = String(row.id ?? "").trim();
    const path = String(row.path ?? "").trim();
    const targetProviderId = String(row.providerId ?? "").trim();
    const capabilityValue = String(row.capability ?? "page.provider").trim();
    if (!id || !path.startsWith("/") || !targetProviderId || shadowsProtectedRoute(path) || !isUIProviderCapability(capabilityValue)) continue;
    routes.push({
      id,
      path,
      providerId: targetProviderId,
      extensionId: provider.extensionId,
      capability: capabilityValue,
      title: row.title ? String(row.title) : undefined,
      priority: Number(row.priority ?? provider.priority ?? 0),
    });
  }
  return routes;
}

/** All enabled compatible route registries contribute. Path conflicts are deterministic. */
function readRoutes(store: ExtensionUIStore): ProviderRouteDefinition[] {
  const candidates = collectRegistryProviders(store, "route.registry")
    .flatMap(providerRoutes)
    .sort((a, b) => b.priority - a.priority || a.extensionId.localeCompare(b.extensionId) || a.id.localeCompare(b.id));
  const routes: ProviderRouteDefinition[] = [];
  const seenPaths = new Set<string>();
  const platform = resolveHostEnvironment().platform;
  for (const route of candidates) {
    if (seenPaths.has(route.path)) continue;
    const target = store.getProviders(route.capability ?? "page.provider")
      .find((provider: UIProviderDefinition) => provider.providerId === route.providerId);
    if (!target || !target.enabled || !isProviderCompatible(target, store.snapshot?.providerContext, platform)) continue;
    seenPaths.add(route.path);
    routes.push(route);
  }
  return routes;
}

function isProviderRoute(route: RouteRecordNormalized): boolean {
  return typeof route.name === "string" && route.name.startsWith(PROVIDER_ROUTE_PREFIX);
}

function routeName(route: ProviderRouteDefinition): string {
  return `${PROVIDER_ROUTE_PREFIX}${route.extensionId}:${route.id}`;
}

export function syncProviderRoutes(router: Router, store: ExtensionUIStore): void {
  const desired = new Map<string, ProviderRouteDefinition>();
  for (const route of readRoutes(store)) desired.set(routeName(route), route);

  for (const route of router.getRoutes()) {
    if (!isProviderRoute(route)) continue;
    const name = String(route.name);
    const target = desired.get(name);
    if (!target || target.path !== route.path || target.providerId !== route.meta.uiProviderId) {
      router.removeRoute(name);
      registeredRouteNames.delete(name);
    }
  }

  const corePaths = new Set(router.getRoutes().filter((route: RouteRecordNormalized) => !isProviderRoute(route)).map((route: RouteRecordNormalized) => route.path));
  for (const [name, route] of desired) {
    if (corePaths.has(route.path) || shadowsProtectedRoute(route.path)) continue;
    if (router.hasRoute(name)) {
      registeredRouteNames.add(name);
      continue;
    }
    router.addRoute({
      name,
      path: route.path,
      component: ProviderRouteHost,
      meta: {
        uiProviderId: route.providerId,
        uiCapability: route.capability ?? "page.provider",
        title: route.title,
        extensionRoute: true,
        extensionId: route.extensionId,
      },
    });
    registeredRouteNames.add(name);
  }
}

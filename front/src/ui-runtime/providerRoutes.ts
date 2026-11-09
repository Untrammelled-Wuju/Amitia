import type { RouteRecordNormalized, RouteRecordRaw, Router } from "vue-router";
import ProviderRouteHost from "@/components/ui-runtime/ProviderRouteHost.vue";
import type { useExtensionUIStore } from "@/stores/extensionUI";
import { builtinBusinessRoutes } from "@/router/builtinRoutes";
import { CORE_ROUTE_ROOTS } from "@/router/coreRoutes";
import { isUIProviderCapability, type UIProviderCapability, type UIProviderDefinition } from "./types";
import { collectRegistryProviders } from "./providerCollection";
import { isProviderCompatible } from "./providerRuntime";
import { resolveHostEnvironment } from "@/composables/useHostEnvironment";

type ExtensionUIStore = ReturnType<typeof useExtensionUIStore>;

export interface ProviderRouteDefinition {
  id: string;
  path: string;
  providerId: string;
  registryProviderId: string;
  extensionId: string;
  capability?: UIProviderCapability;
  title?: string;
  priority: number;
}

const PROVIDER_ROUTE_PREFIX = "ui-provider:";
const registeredRouteNames = new Set<string>();

function rootNamespace(path: string): string | null {
  const normalized = String(path || "").trim();
  if (!normalized.startsWith("/") || normalized === "/") return null;
  const segment = normalized.slice(1).split("/")[0]?.trim();
  if (!segment || segment.startsWith(":") || segment.includes("*")) return null;
  return `/${segment}`;
}


function hasSafeRouteSyntax(path: string): boolean {
  const normalized = String(path || "").trim();
  if (
    !normalized.startsWith("/") ||
    normalized === "/" ||
    normalized.includes("\\") ||
    normalized.includes("?") ||
    normalized.includes("#") ||
    normalized.includes("%") ||
    normalized.includes("\u0000") ||
    rootNamespace(normalized) === null
  ) {
    return false;
  }
  return normalized
    .slice(1)
    .split("/")
    .every((segment) => segment.length > 0 && segment !== "." && segment !== "..");
}

function routeAliases(route: RouteRecordRaw): string[] {
  if (!route.alias) return [];
  return Array.isArray(route.alias) ? route.alias.map(String) : [String(route.alias)];
}

const BUILTIN_ROUTE_PATHS = new Set<string>(
  builtinBusinessRoutes
    .flatMap((route) => [String(route.path ?? ""), ...routeAliases(route)])
    .filter(Boolean),
);

const OPEN_BUILTIN_ROUTE_ROOTS = new Set<string>(
  builtinBusinessRoutes
    .filter((route) => route.meta?.allowsExtensionChildren === true)
    .map((route) => rootNamespace(String(route.path ?? "")))
    .filter((value): value is string => Boolean(value)),
);

const PROTECTED_BUILTIN_ROUTE_ROOTS = new Set<string>(
  [...BUILTIN_ROUTE_PATHS]
    .map(rootNamespace)
    .filter((value): value is string => value !== null && !OPEN_BUILTIN_ROUTE_ROOTS.has(value)),
);

/** Extension routes may not occupy a built-in top-level route namespace. */
export function shadowsProtectedRoute(path: string): boolean {
  const normalized = String(path || "").trim();
  const namespace = rootNamespace(path);
  if (namespace === null) return false;
  if (CORE_ROUTE_ROOTS.has(namespace) || BUILTIN_ROUTE_PATHS.has(normalized)) return true;
  if (OPEN_BUILTIN_ROUTE_ROOTS.has(namespace)) return false;
  return PROTECTED_BUILTIN_ROUTE_ROOTS.has(namespace);
}

function finiteNumber(value: unknown, fallback: number): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
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
    if (
      !id ||
      !hasSafeRouteSyntax(path) ||
      !targetProviderId ||
      shadowsProtectedRoute(path) ||
      !isUIProviderCapability(capabilityValue)
    ) {
      continue;
    }
    routes.push({
      id,
      path,
      providerId: targetProviderId,
      registryProviderId: provider.providerId,
      extensionId: provider.extensionId,
      capability: capabilityValue,
      title: row.title ? String(row.title) : undefined,
      priority: finiteNumber(row.priority, finiteNumber(provider.priority, 0)),
    });
  }
  return routes;
}

/**
 * All enabled compatible route registries contribute routes. Path conflicts are
 * deterministic. A registry can only bind pages owned by the same extension.
 */
export function collectEffectiveProviderRoutes(store: ExtensionUIStore): ProviderRouteDefinition[] {
  const candidates = collectRegistryProviders(store, "route.registry")
    .flatMap(providerRoutes)
    .sort(
      (a, b) =>
        b.priority - a.priority ||
        a.extensionId.localeCompare(b.extensionId) ||
        a.registryProviderId.localeCompare(b.registryProviderId) ||
        a.id.localeCompare(b.id),
    );
  const routes: ProviderRouteDefinition[] = [];
  const seenPaths = new Set<string>();
  const platform = resolveHostEnvironment().platform;
  for (const route of candidates) {
    if (seenPaths.has(route.path)) continue;
    const target = store
      .getProviders(route.capability ?? "page.provider")
      .find((provider: UIProviderDefinition) => provider.providerId === route.providerId);
    if (
      !target ||
      target.extensionId !== route.extensionId ||
      !target.enabled ||
      !isProviderCompatible(target, store.snapshot?.providerContext, platform)
    ) {
      continue;
    }
    seenPaths.add(route.path);
    routes.push(route);
  }
  return routes;
}

export interface ExtensionSectionEntry {
  id: string;
  section: string;
  route: string;
  label: string;
  description?: string;
  order: number;
  extensionId: string;
  registryProviderId: string;
}

export function collectExtensionSectionEntries(
  store: ExtensionUIStore,
  section: string,
): ExtensionSectionEntry[] {
  const effectiveRoutes = collectEffectiveProviderRoutes(store);
  const result: ExtensionSectionEntry[] = [];
  const seen = new Set<string>();
  for (const provider of collectRegistryProviders(store, "route.registry")) {
    const rawEntries = provider.metadata?.sectionEntries;
    if (!Array.isArray(rawEntries)) continue;
    for (const value of rawEntries) {
      if (!value || typeof value !== "object") continue;
      const row = value as Record<string, unknown>;
      const id = String(row.id ?? "").trim();
      const rowSection = String(row.section ?? "").trim();
      const route = String(row.route ?? "").trim();
      const label = String(row.label ?? "").trim();
      if (!id || rowSection !== section || !route || !label || seen.has(route)) continue;
      const effective = effectiveRoutes.find(
        (item) => item.registryProviderId === provider.providerId && item.path === route,
      );
      if (!effective) continue;
      seen.add(route);
      result.push({
        id,
        section: rowSection,
        route,
        label,
        description: String(row.description ?? "").trim() || effective.title,
        order: finiteNumber(row.order, effective.priority),
        extensionId: provider.extensionId,
        registryProviderId: provider.providerId,
      });
    }
  }
  return result.sort((left, right) => left.order - right.order || left.label.localeCompare(right.label));
}

export function effectiveProviderRouteKeys(store: ExtensionUIStore): Set<string> {
  return new Set(
    collectEffectiveProviderRoutes(store).map((route) => `${route.registryProviderId}\u0000${route.path}`),
  );
}

export function effectiveExtensionRouteKeys(store: ExtensionUIStore): Set<string> {
  return new Set(
    collectEffectiveProviderRoutes(store).map((route) => `${route.extensionId}\u0000${route.path}`),
  );
}

function isProviderRoute(route: RouteRecordNormalized): boolean {
  return typeof route.name === "string" && route.name.startsWith(PROVIDER_ROUTE_PREFIX);
}

function routeName(route: ProviderRouteDefinition): string {
  return `${PROVIDER_ROUTE_PREFIX}${route.extensionId}:${route.registryProviderId}:${route.id}`;
}

export function syncProviderRoutes(router: Router, store: ExtensionUIStore): void {
  if (!store.snapshot) return;
  const desired = new Map<string, ProviderRouteDefinition>();
  for (const route of collectEffectiveProviderRoutes(store)) desired.set(routeName(route), route);

  for (const route of router.getRoutes()) {
    if (!isProviderRoute(route)) continue;
    const name = String(route.name);
    const target = desired.get(name);
    if (
      !target ||
      target.path !== route.path ||
      target.providerId !== route.meta.uiProviderId ||
      (target.capability ?? "page.provider") !== route.meta.uiCapability ||
      target.extensionId !== route.meta.extensionId ||
      route.meta.fullHeight !== true
    ) {
      router.removeRoute(name);
      registeredRouteNames.delete(name);
    }
  }

  const corePaths = new Set(
    router
      .getRoutes()
      .filter((route: RouteRecordNormalized) => !isProviderRoute(route))
      .map((route: RouteRecordNormalized) => route.path),
  );
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
        routeRegistryProviderId: route.registryProviderId,
        fullHeight: true,
      },
    });
    registeredRouteNames.add(name);
  }
}

import type { UIProviderCapability, UIProviderDefinition, UIProviderResolveContext } from "./types";
import { resolveHostEnvironment } from "@/composables/useHostEnvironment";
import { isProviderCompatible } from "./providerRuntime";
import { providerHasRouteSelectors } from "./providerCollection";

function matchesPattern(path: string, pattern: string): boolean {
  const clean = pattern.trim();
  if (!clean) return false;
  if (clean === "*" || clean === "/*") return true;
  if (clean === path) return true;
  if (clean.endsWith("/*")) {
    const prefix = clean.slice(0, -2);
    return path === prefix || path.startsWith(`${prefix}/`);
  }
  if (!clean.includes(":")) return false;
  const routeParts = path.split("/").filter(Boolean);
  const patternParts = clean.split("/").filter(Boolean);
  if (routeParts.length !== patternParts.length) return false;
  return patternParts.every((part, index) => part.startsWith(":") || part === "*" || part === routeParts[index]);
}

function providerMatchesRoute(provider: UIProviderDefinition, path: string): boolean {
  const metadata = provider.metadata ?? {};
  const routes = Array.isArray(metadata.routes) ? metadata.routes : [];
  const routePatterns = Array.isArray(metadata.routePatterns) ? metadata.routePatterns : [];
  const selectors = [...routes, ...routePatterns]
    .filter((item) => typeof item === "string")
    .map((item) => String(item ?? "").trim())
    .filter(Boolean);
  return selectors.length > 0 && selectors.some((pattern) => matchesPattern(path, pattern));
}

/**
 * Route-targeted providers compose as a registry. A selector-less provider is a
 * global surface replacement and therefore only activates when selected by profile.
 */
export function resolvePageProvider(
  providers: UIProviderDefinition[],
  resolved: UIProviderDefinition | null,
  capability: UIProviderCapability,
  path: string,
  context?: UIProviderResolveContext,
): UIProviderDefinition | null {
  const platform = resolveHostEnvironment().platform;
  const builtin = providers.find(
    (provider) => provider.enabled && provider.builtin && provider.capability === capability && isProviderCompatible(provider, context, platform),
  ) ?? null;

  const routeCandidates = providers
    .filter((provider) =>
      provider.enabled &&
      !provider.builtin &&
      isProviderCompatible(provider, context, platform) &&
      provider.capability === capability &&
      providerHasRouteSelectors(provider) &&
      providerMatchesRoute(provider, path),
    )
    .sort((a, b) => (b.priority ?? 0) - (a.priority ?? 0) || a.providerId.localeCompare(b.providerId));
  if (routeCandidates.length > 0) return routeCandidates[0];

  if (resolved?.enabled && resolved.capability === capability && isProviderCompatible(resolved, context, platform)) {
    if (resolved.builtin) return resolved;
    if (!providerHasRouteSelectors(resolved)) return resolved;
  }
  return builtin;
}

import type { UIProviderCapability, UIProviderDefinition } from "./types";

function matchesPattern(path: string, pattern: string): boolean {
  const clean = pattern.trim();
  if (!clean) return false;
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
  const selectors = [...routes, ...routePatterns].map((item) => String(item ?? "").trim()).filter(Boolean);
  // A capability-level provider without route selectors remains the profile/global provider.
  return selectors.length === 0 || selectors.some((pattern) => matchesPattern(path, pattern));
}

export function resolvePageProvider(
  providers: UIProviderDefinition[],
  resolved: UIProviderDefinition | null,
  capability: UIProviderCapability,
  path: string,
): UIProviderDefinition | null {
  const builtin = providers.find(
    (provider) => provider.enabled && provider.builtin && provider.capability === capability,
  ) ?? null;
  if (!resolved || !resolved.enabled || resolved.capability !== capability) return builtin;
  if (resolved.builtin) return resolved;

  // Route selectors refine only the profile-selected provider. Merely enabling
  // a page provider must never replace a built-in business page implicitly.
  return providerMatchesRoute(resolved, path) ? resolved : builtin;
}

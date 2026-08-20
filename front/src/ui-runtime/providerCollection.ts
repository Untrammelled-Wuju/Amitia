import type { useExtensionUIStore } from "@/stores/extensionUI";
import { resolveHostEnvironment } from "@/composables/useHostEnvironment";
import { isProviderCompatible } from "./providerRuntime";
import type { UIProviderCapability, UIProviderDefinition } from "./types";

type ExtensionUIStore = ReturnType<typeof useExtensionUIStore>;

/**
 * Registry-style capabilities are composed from every enabled compatible provider.
 * Replaceable surface capabilities continue to resolve through the active profile.
 */
export function collectRegistryProviders(
  store: ExtensionUIStore,
  capability: UIProviderCapability,
): UIProviderDefinition[] {
  const platform = resolveHostEnvironment().platform;
  const providers = store.getProviders(capability) as UIProviderDefinition[];
  return providers
    .filter((provider: UIProviderDefinition) =>
      provider.enabled &&
      !provider.builtin &&
      isProviderCompatible(provider, store.snapshot?.providerContext, platform),
    )
    .sort((a: UIProviderDefinition, b: UIProviderDefinition) => (b.priority ?? 0) - (a.priority ?? 0) || a.providerId.localeCompare(b.providerId));
}

export function providerHasRouteSelectors(provider: UIProviderDefinition): boolean {
  const metadata = provider.metadata ?? {};
  return (Array.isArray(metadata.routes) && metadata.routes.length > 0) ||
    (Array.isArray(metadata.routePatterns) && metadata.routePatterns.length > 0);
}

export function providerHasMessageSelectors(provider: UIProviderDefinition): boolean {
  const metadata = provider.metadata ?? {};
  return ["messageTypes", "roles", "mimeTypes", "extensionTypes"]
    .some((key) => Array.isArray(metadata[key]) && (metadata[key] as unknown[]).length > 0);
}

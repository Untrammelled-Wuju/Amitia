import type { useExtensionUIStore } from "@/stores/extensionUI";
import type { UINavigationItem } from "@/ui-runtime/navigationRegistry";
import { collectEffectiveProviderRoutes } from "@/ui-runtime/providerRoutes";
import { prewarmTrustedWebModule, selectProviderEntry } from "@/ui-runtime/providerRuntime";
import { resolveHostEnvironment } from "@/composables/useHostEnvironment";

type ExtensionUIStore = ReturnType<typeof useExtensionUIStore>;

export function prewarmNavigationItem(store: ExtensionUIStore, item: UINavigationItem): void {
  if (!item.extensionId) return;
  const route = collectEffectiveProviderRoutes(store).find(
    (candidate) => candidate.path === item.route && candidate.extensionId === item.extensionId,
  );
  if (!route) return;
  const capability = route.capability ?? "page.provider";
  const provider = store.getProviders(capability).find(
    (candidate) => candidate.providerId === route.providerId && candidate.extensionId === route.extensionId && candidate.enabled,
  );
  if (!provider) return;
  const env = resolveHostEnvironment();
  const entry = selectProviderEntry(provider, env.platform);
  if (entry) void prewarmTrustedWebModule(provider, entry);
}

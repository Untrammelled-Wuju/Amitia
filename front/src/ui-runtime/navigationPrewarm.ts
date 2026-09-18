import type { useExtensionUIStore } from "@/stores/extensionUI";
import type { UINavigationItem } from "@/ui-runtime/navigationRegistry";
import { collectEffectiveProviderRoutes } from "@/ui-runtime/providerRoutes";
import { selectProviderEntry } from "@/ui-runtime/providerRuntime";
import { providerSlotId } from "@/ui-runtime/providerSlotAdapter";
import { resolveHostEnvironment } from "@/composables/useHostEnvironment";
import { canonicalUISurfaceId, uiRouteAliases } from "@/ui-runtime/uiSurfaceCatalog";
import { prewarmSandboxSession } from "@/components/extension/sandboxSessionCache";

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
  const contributionId = entry?.contributionId?.trim();
  if (!contributionId) return;
  const contribution = store.getContributionById(contributionId);
  if (
    !contribution ||
    !contribution.visible ||
    !contribution.effective ||
    !contribution.enabled ||
    !contribution.runtimeReady ||
    !["web_restricted", "web_isolated"].includes(contribution.sandbox ?? "")
  ) return;

  void import("@/components/extension/SandboxWebUIFrame.vue");
  prewarmSandboxSession({
    contribution,
    context: {
      route: item.route,
      routeParams: {},
      routeQuery: {},
      surfaceId: canonicalUISurfaceId(item.route),
      routeAliases: uiRouteAliases(item.route),
      platform: env.platform,
      host: env.host,
      os: env.os,
      locale: navigator.language,
      capability,
      providerId: provider.providerId,
      providerMode: provider.mode,
    },
    slotId: providerSlotId(capability),
  });
}

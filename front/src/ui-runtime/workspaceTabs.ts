import { resolveHostEnvironment } from "@/composables/useHostEnvironment";
import type { useExtensionUIStore } from "@/stores/extensionUI";
import { isProviderCompatible, selectProviderEntry } from "./providerRuntime";
import type { UIProviderDefinition, UIProviderEntry } from "./types";

type ExtensionUIStore = ReturnType<typeof useExtensionUIStore>;

export interface WorkspaceTabDefinition {
  key: string;
  tabId: string;
  label: string;
  icon?: string;
  order: number;
  keepMounted: boolean;
  storage: "session" | "persistent";
  provider: UIProviderDefinition;
  entry: UIProviderEntry;
}

function finiteNumber(value: unknown, fallback: number): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function contextMatches(
  match: Record<string, unknown> | undefined,
  context: Record<string, unknown>,
): boolean {
  if (!match) return true;
  for (const [key, expected] of Object.entries(match)) {
    const actual = String(context[key] ?? "").trim();
    const values = Array.isArray(expected) ? expected.map(String) : [String(expected)];
    if (!values.some((value) => value.trim() === actual)) return false;
  }
  return true;
}

export function collectWorkspaceTabs(
  store: ExtensionUIStore,
  workspace: string,
  context: Record<string, unknown>,
  routePath: string,
  platform = resolveHostEnvironment().platform,
): WorkspaceTabDefinition[] {
  const candidates: WorkspaceTabDefinition[] = [];
  for (const provider of store.getProviders("workspace.tab")) {
    const metadata = provider.metadata ?? {};
    if (String(metadata.workspace ?? "").trim() !== workspace) continue;
    if (!isProviderCompatible(provider, store.snapshot?.providerContext, platform)) continue;
    const routePrefix = String(metadata.routePrefix ?? "").trim();
    if (routePrefix && routePath !== routePrefix && !routePath.startsWith(`${routePrefix}/`)) continue;
    const rawMatch = metadata.contextMatch;
    const match = rawMatch && typeof rawMatch === "object" && !Array.isArray(rawMatch)
      ? rawMatch as Record<string, unknown>
      : undefined;
    if (!contextMatches(match, context)) continue;
    const entry = selectProviderEntry(provider, platform);
    if (!entry) continue;
    const tabId = String(metadata.tabId ?? "").trim();
    if (!tabId) continue;
    candidates.push({
      key: provider.providerId,
      tabId,
      label: String(metadata.title ?? tabId).trim(),
      icon: metadata.icon ? String(metadata.icon) : undefined,
      order: finiteNumber(metadata.order, finiteNumber(provider.priority, 1000)),
      keepMounted: metadata.keepMounted !== false,
      storage: metadata.storage === "persistent" ? "persistent" : "session",
      provider,
      entry,
    });
  }
  candidates.sort((left, right) =>
    right.order - left.order
    || (right.provider.priority ?? 0) - (left.provider.priority ?? 0)
    || left.tabId.localeCompare(right.tabId)
    || left.provider.providerId.localeCompare(right.provider.providerId),
  );
  const seen = new Set<string>();
  return candidates.filter((tab) => {
    if (seen.has(tab.tabId)) return false;
    seen.add(tab.tabId);
    return true;
  });
}

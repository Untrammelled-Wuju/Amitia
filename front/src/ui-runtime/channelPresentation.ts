import { computed } from "vue";
import { useExtensionUIStore } from "@/stores/extensionUI";
import { resolveHostEnvironment } from "@/composables/useHostEnvironment";
import { isProviderCompatible } from "./providerRuntime";

export interface ChannelPresentationDescriptor {
  providerId: string;
  extensionId: string;
  channelId: string;
  displayName: string;
  icon: string;
  order: number;
  priority: number;
  capabilities: Record<string, unknown>;
}

function text(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

function integer(value: unknown, fallback: number): number {
  return typeof value === "number" && Number.isFinite(value) ? Math.trunc(value) : fallback;
}

export function resolveChannelPresentations(
  store: ReturnType<typeof useExtensionUIStore>,
): ChannelPresentationDescriptor[] {
  const platform = resolveHostEnvironment().platform;
  const providers = store
    .getProviders("channel.presentation")
    .filter((provider) => !provider.builtin && isProviderCompatible(provider, store.snapshot?.providerContext, platform))
    .sort((left, right) => (right.priority ?? 0) - (left.priority ?? 0) || left.providerId.localeCompare(right.providerId));
  const byChannel = new Map<string, ChannelPresentationDescriptor>();
  for (const provider of providers) {
    const metadata = provider.metadata ?? {};
    const channelId = text(metadata.channelId);
    if (!channelId || byChannel.has(channelId)) continue;
    const capabilities = metadata.capabilities && typeof metadata.capabilities === "object" && !Array.isArray(metadata.capabilities)
      ? metadata.capabilities as Record<string, unknown>
      : {};
    byChannel.set(channelId, {
      providerId: provider.providerId,
      extensionId: provider.extensionId,
      channelId,
      displayName: text(metadata.displayName) || channelId,
      icon: text(metadata.icon) || "channel",
      order: integer(metadata.order, 1000),
      priority: provider.priority ?? 0,
      capabilities,
    });
  }
  return [...byChannel.values()].sort(
    (left, right) => left.order - right.order || left.displayName.localeCompare(right.displayName) || left.channelId.localeCompare(right.channelId),
  );
}

export function useChannelPresentations() {
  const store = useExtensionUIStore();
  const presentations = computed(() => resolveChannelPresentations(store));
  const byChannel = computed(() => new Map(presentations.value.map((item) => [item.channelId, item])));
  return { presentations, byChannel, store };
}

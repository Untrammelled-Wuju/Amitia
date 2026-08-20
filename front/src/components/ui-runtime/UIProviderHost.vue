<script setup lang="ts">
import { computed, onErrorCaptured, onMounted, ref, shallowRef, watch, type Component } from "vue";
import { useRoute } from "vue-router";
import { resolveHostEnvironment } from "@/composables/useHostEnvironment";
import ExtensionContributionRenderer from "@/components/extension/ExtensionContributionRenderer.vue";
import { useExtensionUIStore } from "@/stores/extensionUI";
import { isProviderCompatible, selectProviderEntry, trustedWebModule } from "@/ui-runtime/providerRuntime";
import type { UIProviderCapability, UIProviderDefinition, UIProviderRenderContext } from "@/ui-runtime/types";

const props = defineProps<{
  capability: UIProviderCapability;
  providerId?: string;
  fallback: Component;
  context?: UIProviderRenderContext;
  actions?: Record<string, (input?: unknown) => unknown | Promise<unknown>>;
}>();

const store = useExtensionUIStore();
const route = useRoute();
const env = resolveHostEnvironment();
const loadError = shallowRef<string | null>(null);
const activeRef = shallowRef<any>(null);
const fallbackIndex = ref(0);

const requestedProvider = computed<UIProviderDefinition | null>(() => {
  if (!props.providerId) return store.getResolvedProvider(props.capability);
  const selected = store.getProviders().find((item) => item.providerId === props.providerId) ?? null;
  return selected?.capability === props.capability ? selected : null;
});

const providerChain = computed<UIProviderDefinition[]>(() => {
  const providers = store.getProviders(props.capability);
  const byId = new Map(providers.map((item) => [item.providerId, item]));
  const chain: UIProviderDefinition[] = [];
  const seen = new Set<string>();
  let current = requestedProvider.value;
  while (current && !seen.has(current.providerId)) {
    seen.add(current.providerId);
    if (isProviderCompatible(current, store.snapshot?.providerContext, env.platform)) chain.push(current);
    const nextId = current.fallbackProviderId?.trim();
    current = nextId ? byId.get(nextId) ?? null : null;
  }
  // A built-in provider is always the final recovery boundary even when an
  // extension forgot to declare fallbackProviderId.
  const builtin = providers.find((item) => item.builtin && isProviderCompatible(item, store.snapshot?.providerContext, env.platform));
  if (builtin && !seen.has(builtin.providerId)) chain.push(builtin);
  return chain;
});

const provider = computed(() => providerChain.value[Math.min(fallbackIndex.value, Math.max(providerChain.value.length - 1, 0))] ?? null);
const entry = computed(() => provider.value ? selectProviderEntry(provider.value, env.platform) : null);
const trustedComponent = computed(() => {
  const p = provider.value;
  const e = entry.value;
  if (!p || !e || p.builtin) return null;
  return trustedWebModule(p, e);
});
const sourceContribution = computed(() => {
  const id = entry.value?.contributionId;
  return id ? store.getContributionById(id) : null;
});
const hasExternalRenderer = computed(() => !!provider.value && !provider.value.builtin && !!entry.value && (!!trustedComponent.value || !!sourceContribution.value));
const mode = computed(() => provider.value?.mode ?? "replace");
const mergedContext = computed<Record<string, unknown>>(() => ({
  route: route.fullPath,
  platform: env.platform,
  host: env.host,
  os: env.os,
  locale: navigator.language,
  capability: props.capability,
  providerId: provider.value?.providerId,
  providerMode: provider.value?.mode,
  ...(props.context ?? {}),
}));

const rootEl = computed(() => activeRef.value?.rootEl ?? activeRef.value?.$el ?? null);
function focus(...args: unknown[]) { return activeRef.value?.focus?.(...args); }
function setText(...args: unknown[]) { return activeRef.value?.setText?.(...args); }
function clear(...args: unknown[]) { return activeRef.value?.clear?.(...args); }
defineExpose({ rootEl, focus, setText, clear });

function advanceFallback(error: unknown) {
  const message = error instanceof Error ? error.message : String(error);
  const current = provider.value;
  console.error(`[UIProviderHost] provider ${current?.providerId ?? props.capability} failed`, error);
  if (fallbackIndex.value + 1 < providerChain.value.length) {
    fallbackIndex.value += 1;
    loadError.value = null;
    return;
  }
  loadError.value = message;
}

watch(
  [() => requestedProvider.value?.providerId, () => requestedProvider.value?.generation, () => env.platform],
  () => {
    fallbackIndex.value = 0;
    loadError.value = null;
  },
);

onErrorCaptured((error) => {
  if (!trustedComponent.value) return true;
  advanceFallback(error);
  return false;
});
onMounted(() => { if (!store.snapshot) void store.refreshSnapshot(); });
</script>

<template>
  <!-- Built-in/no-provider is the stable recovery boundary. -->
  <component
    ref="activeRef"
    :is="fallback"
    v-if="!hasExternalRenderer || loadError"
    v-bind="$attrs"
  >
    <template v-for="(_, name) in $slots" #[name]="slotProps">
      <slot :name="name" v-bind="slotProps || {}" />
    </template>
  </component>

  <!-- augment/compose have one cross-platform contract: keep the built-in
       surface alive and layer the dynamic provider over the same bounds.
       Flutter uses Stack; Web uses an equivalent one-cell CSS grid. -->
  <div v-else-if="mode === 'augment' || mode === 'compose'" class="ui-provider-compose-stack" :data-provider-mode="mode">
    <component :is="fallback" v-bind="$attrs">
      <template v-for="(_, name) in $slots" #[name]="slotProps">
        <slot :name="name" v-bind="slotProps || {}" />
      </template>
    </component>
    <component
      ref="activeRef"
      :is="trustedComponent"
      v-if="trustedComponent"
      v-bind="$attrs"
      :ui-context="mergedContext"
      :provider="provider"
      :ui-actions="actions"
      @error="advanceFallback"
    />
    <ExtensionContributionRenderer
      ref="activeRef"
      v-else-if="sourceContribution"
      :contribution="sourceContribution"
      :context="mergedContext"
      :slot-id="`provider:${capability}`"
      :host-actions="actions"
      @error="advanceFallback"
    />
  </div>

  <!-- replace is the normal provider boundary. -->
  <component
    ref="activeRef"
    :is="trustedComponent"
    v-else-if="trustedComponent"
    v-bind="$attrs"
    :ui-context="mergedContext"
    :provider="provider"
    :ui-actions="actions"
    @error="advanceFallback"
  >
    <template v-for="(_, name) in $slots" #[name]="slotProps">
      <slot :name="name" v-bind="slotProps || {}" />
    </template>
  </component>

  <ExtensionContributionRenderer
    ref="activeRef"
    v-else-if="sourceContribution"
    :contribution="sourceContribution"
    :context="mergedContext"
    :slot-id="`provider:${capability}`"
    :host-actions="actions"
    @error="advanceFallback"
  />
</template>

<style scoped>
.ui-provider-compose-stack {
  display: grid;
  min-width: 0;
  min-height: 0;
}
.ui-provider-compose-stack > * {
  grid-area: 1 / 1;
  min-width: 0;
  min-height: 0;
}
</style>

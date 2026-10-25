<script setup lang="ts">
import { computed, onErrorCaptured, onMounted, ref, shallowRef, watch, type Component } from "vue";
import { useRoute, useRouter } from "vue-router";
import { apiClient } from "@/composables/useApi";
import { resolveHostEnvironment } from "@/composables/useHostEnvironment";
import ExtensionContributionRenderer from "@/components/extension/ExtensionContributionRenderer.vue";
import ExtensionSlot from "@/components/extension/ExtensionSlot.vue";
import { useExtensionUIStore } from "@/stores/extensionUI";
import { isProviderCompatible, selectProviderEntry, trustedWebModule } from "@/ui-runtime/providerRuntime";
import type { UIProviderCapability, UIProviderDefinition, UIProviderRenderContext } from "@/ui-runtime/types";
import { providerSlotId } from "@/ui-runtime/providerSlotAdapter";
import { resolveRouteIdentity } from "@/ui-runtime/routeContext";

defineOptions({ inheritAttrs: false });

const props = defineProps<{
  capability: UIProviderCapability;
  providerId?: string;
  fallback: Component;
  context?: UIProviderRenderContext;
  actions?: Record<string, (input?: unknown) => unknown | Promise<unknown>>;
  frameless?: boolean;
}>();

const store = useExtensionUIStore();
const route = useRoute();
const router = useRouter();
const env = resolveHostEnvironment();
const loadError = shallowRef<string | null>(null);
const activeRef = shallowRef<any>(null);
const slotBoundaryRef = shallowRef<any>(null);
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
const mergedContext = computed<Record<string, unknown>>(() => {
  const identity = resolveRouteIdentity(
    (props.context?.routeParams as Record<string, unknown> | undefined) ?? route.params,
    route.fullPath,
  );
  return {
    route: route.fullPath,
    routeParams: route.params,
    routeQuery: route.query,
    platform: env.platform,
    host: env.host,
    os: env.os,
    locale: navigator.language,
    capability: props.capability,
    providerId: provider.value?.providerId,
    providerMode: provider.value?.mode,
    ...identity,
    ...(props.context ?? {}),
    characterId: String(props.context?.characterId || identity.characterId || ""),
    conversationId: String(props.context?.conversationId || identity.conversationId || ""),
    scope: {
      ...(props.context?.scope as Record<string, unknown> | undefined),
      characterId: String(
        props.context?.characterId
        || (props.context?.scope as Record<string, unknown> | undefined)?.characterId
        || identity.characterId
        || "",
      ),
      conversationId: String(
        props.context?.conversationId
        || (props.context?.scope as Record<string, unknown> | undefined)?.conversationId
        || identity.conversationId
        || "",
      ),
    },
  };
});
const providerActions = computed<Record<string, (input?: unknown) => unknown | Promise<unknown>>>(() => {
  const actions = { ...(props.actions ?? {}) };
  const contribution = sourceContribution.value;
  if (!contribution) return actions;
  for (const action of contribution.actions ?? []) {
    if (actions[action.actionId]) continue;
    actions[action.actionId] = async (input?: unknown) => {
      const response = await apiClient.post(
        `/api/extension/action/${encodeURIComponent(contribution.contributionId)}/${encodeURIComponent(action.actionId)}`,
        { context: mergedContext.value, input: input ?? {} },
      );
      const data = response.data as Record<string, unknown> | undefined;
      if (data?.ok === false) {
        const error = data.error as Record<string, unknown> | string | undefined;
        throw new Error(typeof error === "string" ? error : String(error?.message ?? "插件操作失败"));
      }
      return data?.result ?? data;
    };
  }
  return actions;
});
const navigateProvider = (target: string, type?: string) => {
  if (type === "external") {
    window.open(target, "_blank", "noopener,noreferrer");
    return;
  }
  void router.push(target);
};

const surfaceSlotId = computed(() => providerSlotId(props.capability));
const rootEl = computed(() => activeRef.value?.rootEl ?? activeRef.value?.$el ?? slotBoundaryRef.value?.rootEl ?? slotBoundaryRef.value?.$el ?? null);
function focus(...args: unknown[]) { return activeRef.value?.focus?.(...args) ?? slotBoundaryRef.value?.focus?.(...args); }
function setText(...args: unknown[]) { return activeRef.value?.setText?.(...args) ?? slotBoundaryRef.value?.setText?.(...args); }
function clear(...args: unknown[]) { return activeRef.value?.clear?.(...args) ?? slotBoundaryRef.value?.clear?.(...args); }
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
  <ExtensionSlot
    ref="slotBoundaryRef"
    :slot-id="surfaceSlotId"
    :context="mergedContext"
    fallback="default"
    layout="stack"
    surface-role="main"
    :render-contributions="false"
    render-fallback-while-scope-pending
    bare
  >
    <template #default>
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
      :ui-actions="providerActions"
      :ui-navigate="navigateProvider"
      @error="advanceFallback"
    />
    <ExtensionContributionRenderer
      ref="activeRef"
      v-else-if="sourceContribution"
      :contribution="sourceContribution"
      :context="mergedContext"
      :slot-id="surfaceSlotId"
      :host-actions="actions"
      :frameless="frameless"
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
    :ui-actions="providerActions"
    :ui-navigate="navigateProvider"
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
    :slot-id="surfaceSlotId"
    :host-actions="actions"
    :frameless="frameless"
    @error="advanceFallback"
  />

    </template>
  </ExtensionSlot>
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
.ui-provider-module {
  min-width: 0;
  min-height: 0;
}
</style>

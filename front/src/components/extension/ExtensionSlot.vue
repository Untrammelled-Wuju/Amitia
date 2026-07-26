<script setup lang="ts">
import { computed, onErrorCaptured, ref, defineAsyncComponent } from "vue";
import { useExtensionSlot } from "@/composables/useExtensionSlot";
import type { UIContributionSummary } from "@/stores/extensionUI";

const props = withDefaults(
  defineProps<{
    slotId: string;
    context?: Record<string, unknown>;
    fallback?: "none" | "skeleton" | "empty" | "default";
    layout?: "inline" | "stack" | "row" | "grid" | "tabs" | "panel" | "drawer" | "modal";
    maxItems?: number;
  }>(),
  {
    fallback: undefined,
    layout: undefined,
    maxItems: undefined,
  }
);

const localContext = ref<Record<string, unknown>>(props.context ?? {});
const {
  contributions,
  fallback: resolvedFallback,
  layout: resolvedLayout,
  isEmpty,
  reportError,
} = useExtensionSlot({
  slotId: props.slotId,
  fallback: props.fallback,
  layout: props.layout,
});

const visibleContributions = computed<UIContributionSummary[]>(() => {
  if (props.maxItems && props.maxItems > 0) {
    return contributions.value.slice(0, props.maxItems);
  }
  return contributions.value;
});

const layoutClass = computed(() => `extension-slot--layout-${resolvedLayout.value}`);

const errorState = ref<Record<string, string>>({});

onErrorCaptured((err, instance) => {
  const contributionId = (instance?.$props as { contribution?: string })?.contribution ?? "unknown";
  errorState.value[contributionId] = err instanceof Error ? err.message : String(err);
  reportError(contributionId, errorState.value[contributionId], true);
  return false;
});

function contributionRenderer(contribution: UIContributionSummary) {
  switch (contribution.kind) {
    case "schema_page":
    case "settings_section":
    case "panel":
    case "card":
      return defineAsyncComponent(() => import("./SchemaUIRenderer.vue"));
    case "web_page":
      return defineAsyncComponent(() => import("./SandboxWebUIFrame.vue"));
    case "action":
    case "menu_item":
    case "toolbar_item":
    case "status_item":
    case "message_action":
    case "composer_action":
    case "desktop_command":
      return defineAsyncComponent(() => import("./HostNativeAction.vue"));
    default:
      return null;
  }
}
</script>

<template>
  <div class="extension-slot" :class="layoutClass" :data-slot-id="slotId">
    <template v-if="visibleContributions.length === 0 && resolvedFallback !== 'none'">
      <div v-if="resolvedFallback === 'skeleton'" class="extension-slot__skeleton">
        <div class="skeleton-pulse"></div>
      </div>
      <div v-else-if="resolvedFallback === 'empty'" class="extension-slot__empty">
        <slot name="empty"></slot>
      </div>
      <div v-else-if="resolvedFallback === 'default'" class="extension-slot__default">
        <slot name="default"></slot>
      </div>
    </template>

    <template v-else>
      <template v-for="contribution in visibleContributions" :key="contribution.contributionId">
        <div
          class="extension-slot__contribution"
          :data-contribution-id="contribution.contributionId"
          :data-extension-id="contribution.extensionId"
        >
          <template v-if="errorState[contribution.contributionId]">
            <div class="extension-slot__error">
              <span>{{ errorState[contribution.contributionId] }}</span>
            </div>
          </template>
          <template v-else>
            <component
              :is="contributionRenderer(contribution)"
              :contribution="contribution"
              :context="localContext"
              :slot-id="slotId"
            />
          </template>
        </div>
      </template>
    </template>
  </div>
</template>

<style scoped>
.extension-slot {
  display: flex;
  width: 100%;
  min-width: 0;
}
.extension-slot--layout-inline {
  flex-direction: row;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.extension-slot--layout-stack {
  flex-direction: column;
  gap: 12px;
}
.extension-slot--layout-row {
  flex-direction: row;
  gap: 12px;
  flex-wrap: wrap;
}
.extension-slot--layout-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}
.extension-slot--layout-tabs {
  flex-direction: row;
  align-items: stretch;
}
.extension-slot--layout-panel {
  flex-direction: column;
  padding: 12px;
  border-radius: 8px;
}
.extension-slot--layout-drawer,
.extension-slot--layout-modal {
  position: relative;
}
.extension-slot__contribution {
  min-width: 0;
  flex: 1 1 auto;
}
.extension-slot__skeleton {
  width: 100%;
  height: 32px;
  background: rgba(127, 127, 127, 0.1);
  border-radius: 6px;
  overflow: hidden;
}
.skeleton-pulse {
  width: 100%;
  height: 100%;
  background: linear-gradient(90deg, transparent, rgba(127, 127, 127, 0.2), transparent);
  animation: skeleton-pulse 1.6s infinite;
}
@keyframes skeleton-pulse {
  0% { transform: translateX(-100%); }
  100% { transform: translateX(100%); }
}
.extension-slot__error {
  padding: 8px 12px;
  border-radius: 6px;
  background: rgba(220, 50, 50, 0.08);
  color: rgb(180, 40, 40);
  font-size: 12px;
}
</style>

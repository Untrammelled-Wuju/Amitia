<script setup lang="ts">
import { computed, onBeforeUnmount, onErrorCaptured, onMounted, ref, watch } from "vue";
import { useExtensionSlot, type ExtensionSurfaceContext, type ExtensionSurfaceRole } from "@/composables/useExtensionSlot";
import { useExtensionUIStore, type UIContributionSummary } from "@/stores/extensionUI";
import ExtensionContributionRenderer from "./ExtensionContributionRenderer.vue";
import ExtensionRenderState from "./ExtensionRenderState.vue";

const props = withDefaults(
  defineProps<{
    slotId: string;
    context?: Record<string, unknown>;
    fallback?: "none" | "skeleton" | "empty" | "default";
    layout?: "inline" | "stack" | "row" | "grid" | "tabs" | "panel" | "drawer" | "modal";
    maxItems?: number;
    startIndex?: number;
    surfaceRole?: ExtensionSurfaceRole;
  }>(),
  {
    fallback: undefined,
    layout: undefined,
    maxItems: undefined,
    startIndex: 0,
    surfaceRole: "main",
  }
);

const localContext = ref<Record<string, unknown>>(props.context ?? {});
const rootRef = ref<HTMLElement>();
const surface = ref<ExtensionSurfaceContext>({ role: props.surfaceRole, width: 0, height: 0, breakpoint: "xs" });
let observer: ResizeObserver | null = null;
let resizeFrame = 0;
const {
  contributions,
  fallback: resolvedFallback,
  layout: resolvedLayout,
  isEmpty,
  reportError,
  buildContext,
} = useExtensionSlot({
  slotId: props.slotId,
  fallback: props.fallback,
  layout: props.layout,
  context: localContext,
  surface,
});

watch(() => props.context, (value) => { localContext.value = value ?? {}; }, { deep: true });
watch(() => props.surfaceRole, (role) => { surface.value = { ...surface.value, role }; });

const renderContext = computed<Record<string, unknown>>(() => ({ ...buildContext(), ...localContext.value }));

function breakpointFor(width: number): ExtensionSurfaceContext["breakpoint"] {
  if (width < 320) return "xs";
  if (width < 480) return "sm";
  if (width < 768) return "md";
  if (width < 1024) return "lg";
  return "xl";
}

function updateSurface(entry?: ResizeObserverEntry) {
  const rect = entry?.contentRect ?? rootRef.value?.getBoundingClientRect();
  if (!rect) return;
  surface.value = { role: props.surfaceRole, width: Math.round(rect.width), height: Math.round(rect.height), breakpoint: breakpointFor(rect.width) };
}

onMounted(() => {
  if (!rootRef.value || typeof ResizeObserver === "undefined") return;
  observer = new ResizeObserver((entries) => {
    const entry = entries[0];
    cancelAnimationFrame(resizeFrame);
    resizeFrame = requestAnimationFrame(() => updateSurface(entry));
  });
  observer.observe(rootRef.value);
  updateSurface();
});

onBeforeUnmount(() => {
  observer?.disconnect();
  cancelAnimationFrame(resizeFrame);
});

const visibleContributions = computed<UIContributionSummary[]>(() => {
  const contributionList = contributions.value.slice(props.startIndex);
  if (props.maxItems && props.maxItems > 0) {
    return contributionList.slice(0, props.maxItems);
  }
  return contributionList;
});

const layoutClass = computed(() => `extension-slot--layout-${resolvedLayout.value}`);

const errorState = ref<Record<string, string>>({});

onErrorCaptured((err, instance) => {
  const props = instance?.$props as { contribution?: UIContributionSummary | string | { contributionId?: string } };
  let contributionId = "unknown";
  if (props?.contribution && typeof props.contribution === "object") {
    contributionId = (props.contribution as UIContributionSummary).contributionId ?? "unknown";
  } else if (typeof props?.contribution === "string") {
    contributionId = props.contribution;
  }
  errorState.value[contributionId] = err instanceof Error ? err.message : String(err);
  reportError(contributionId, errorState.value[contributionId], true);
  return false;
});

function retryContribution(contributionId: string) {
  delete errorState.value[contributionId];
  const store = useExtensionUIStore();
  store.clearErrors(contributionId);
}

</script>

<template>
  <div ref="rootRef" class="extension-slot" :class="[layoutClass, `extension-slot--${surfaceRole}`]" :data-slot-id="slotId">
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
            <ExtensionRenderState state="error" :detail="errorState[contribution.contributionId]" @retry="retryContribution(contribution.contributionId)" />
          </template>
          <template v-else>
            <ExtensionContributionRenderer
              :contribution="contribution"
              :context="renderContext"
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
.extension-slot--header, .extension-slot--composer, .extension-slot--status { width: auto; }
.extension-slot--sidebar { height: 100%; min-height: 0; overflow: auto; }
.extension-slot--message .extension-slot__contribution { max-width: 100%; }
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
</style>

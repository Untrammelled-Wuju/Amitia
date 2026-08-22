<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useExtensionSlot, type ExtensionSurfaceContext, type ExtensionSurfaceRole } from "@/composables/useExtensionSlot";
import { useExtensionUIStore, type UIContributionSummary } from "@/stores/extensionUI";
import ExtensionContributionRenderer from "./ExtensionContributionRenderer.vue";
import { browserClientPluginRuntime, type ClientSlotContribution } from "@/ui-runtime/clientPluginRuntime";
import ExtensionErrorBoundary from "./ExtensionErrorBoundary.vue";
import ClientSlotContributionHost from "./ClientSlotContributionHost.vue";
import { buildUnifiedSlotItems, type UnifiedSlotItem } from "@/ui-runtime/slotLedger";

const props = withDefaults(
  defineProps<{
    slotId: string;
    context?: Record<string, unknown>;
    fallback?: "none" | "skeleton" | "empty" | "default";
    layout?: "inline" | "stack" | "row" | "grid" | "tabs" | "panel" | "drawer" | "modal";
    maxItems?: number;
    startIndex?: number;
    surfaceRole?: ExtensionSurfaceRole;
    contributionId?: string;
    bare?: boolean;
  }>(),
  {
    fallback: undefined,
    layout: undefined,
    maxItems: undefined,
    startIndex: 0,
    surfaceRole: "main",
    contributionId: undefined,
    bare: false,
  }
);

const store = useExtensionUIStore();
const localContext = ref<Record<string, unknown>>(props.context ?? {});
const rootRef = ref<HTMLElement>();
const surface = ref<ExtensionSurfaceContext>({ role: props.surfaceRole, width: 0, height: 0, breakpoint: "xs" });
let observer: ResizeObserver | null = null;
let resizeFrame = 0;
const {
  contributions,
  scopeReady,
  fallback: resolvedFallback,
  layout: resolvedLayout,
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
  if (props.contributionId) {
    return contributions.value.filter((c) => c.contributionId === props.contributionId);
  }
  return contributions.value;
});

const clientContributions = computed<ClientSlotContribution[]>(() => {
  browserClientPluginRuntime.slots.revision.value;
  const items = browserClientPluginRuntime.slots.listContributions(props.slotId);
  return props.contributionId ? items.filter((item) => item.contributionId === props.contributionId) : items;
});

type RenderItem = UnifiedSlotItem;

const visibleItems = computed<RenderItem[]>(() => {
  if (!scopeReady.value) return [];
  const contract = store.slotsById.get(props.slotId) ?? browserClientPluginRuntime.slots.getDefinition(props.slotId);
  const server = props.contributionId
    ? visibleContributions.value.filter((item) => item.contributionId === props.contributionId)
    : visibleContributions.value;
  const client = props.contributionId
    ? clientContributions.value.filter((item) => item.contributionId === props.contributionId)
    : clientContributions.value;
  const items = buildUnifiedSlotItems(contract, server, client);
  const start = Math.max(0, props.startIndex);
  if (props.maxItems && props.maxItems > 0) return items.slice(start, start + props.maxItems);
  return items.slice(start);
});

const layoutClass = computed(() => `extension-slot--layout-${resolvedLayout.value}`);

const isHidden = computed(() => !scopeReady.value || (visibleItems.value.length === 0 && resolvedFallback.value === 'none'));

const rootEl = computed(() => rootRef.value ?? null);
function interactiveElement(): HTMLInputElement | HTMLTextAreaElement | HTMLElement | null {
  return rootRef.value?.querySelector<HTMLInputElement | HTMLTextAreaElement | HTMLElement>(
    "textarea, input, [contenteditable='true'], [tabindex]:not([tabindex='-1']), button",
  ) ?? null;
}
function focus() {
  interactiveElement()?.focus?.();
}
function setText(value: unknown) {
  const element = interactiveElement();
  if (!element) return;
  const text = value == null ? "" : String(value);
  if (element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement) {
    const prototype = element instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
    const descriptor = Object.getOwnPropertyDescriptor(prototype, "value");
    descriptor?.set?.call(element, text);
    element.dispatchEvent(new Event("input", { bubbles: true }));
    return;
  }
  if (element.isContentEditable) {
    element.textContent = text;
    element.dispatchEvent(new InputEvent("input", { bubbles: true, inputType: "insertText", data: text }));
  }
}
function clear() { setText(""); }
defineExpose({ rootEl, focus, setText, clear });

</script>

<template>
  <div v-if="!isHidden" ref="rootRef" class="extension-slot" :class="[layoutClass, `extension-slot--${surfaceRole}`, { 'extension-slot--bare': bare }]" :data-slot-id="slotId">
    <template v-if="visibleItems.length === 0 && resolvedFallback !== 'none'">
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
      <template v-for="item in visibleItems" :key="item.key">
        <div
          v-if="item.source === 'server'"
          class="extension-slot__contribution"
          :data-contribution-id="item.server.contributionId"
          :data-extension-id="item.server.extensionId"
        >
          <ExtensionErrorBoundary :slot-id="slotId" :contribution-id="item.server.contributionId">
            <ExtensionContributionRenderer
              :contribution="item.server"
              :context="renderContext"
              :slot-id="slotId"
            />
          </ExtensionErrorBoundary>
        </div>
        <div
          v-else
          class="extension-slot__contribution extension-slot__contribution--client"
          :data-contribution-id="item.client.contributionId"
          :data-extension-id="item.client.pluginId"
        >
          <ExtensionErrorBoundary :slot-id="slotId" :contribution-id="item.client.contributionId">
            <ClientSlotContributionHost
              :contribution="item.client"
              :context="renderContext"
            />
          </ExtensionErrorBoundary>
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
.extension-slot--sidebar, .extension-slot--main { height: 100%; min-height: 0; overflow: auto; flex: 1; }
.extension-slot--sidebar .extension-slot__contribution,
.extension-slot--main .extension-slot__contribution {
  min-height: 0;
  height: 100%;
  flex: 1 1 auto;
  display: flex;
  flex-direction: column;
}
.extension-slot--message .extension-slot__contribution { max-width: 100%; }
.extension-slot--bare,
.extension-slot--bare > .extension-slot__default,
.extension-slot--bare > .extension-slot__contribution {
  display: contents;
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
</style>

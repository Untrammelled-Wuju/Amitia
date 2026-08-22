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
    /** Keyed-slot dispatch key. */
    dispatchKey?: string;
    /** List-slot cell filter (`renderSlot(..., { only })`). */
    dispatchOnly?: string;
    /** Keep chain fallback mounted while an elected entry is visible. */
    chainOverlay?: boolean;
    /** Opaque render-occurrence context used to bind slot-level hook factories. */
    hookContext?: unknown;
    /** Exact parent contribution that authorized rendering this child slot. */
    authorizedBy?: string;
    bare?: boolean;
  }>(),
  {
    fallback: undefined,
    layout: undefined,
    maxItems: undefined,
    startIndex: 0,
    surfaceRole: "main",
    contributionId: undefined,
    dispatchKey: undefined,
    dispatchOnly: undefined,
    chainOverlay: false,
    hookContext: undefined,
    authorizedBy: undefined,
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

const dispatchedClientContributions = computed(() => {
  browserClientPluginRuntime.slots.revision.value;
  if (props.authorizedBy) {
    browserClientPluginRuntime.slots.assertRenderAuthority(props.authorizedBy, props.slotId);
  }
  const dispatched = browserClientPluginRuntime.slots.dispatchContributions(
    props.slotId,
    renderContext.value,
    props.dispatchKey,
    props.dispatchOnly,
  );
  return props.contributionId
    ? dispatched.filter((item) => item.contribution.contributionId === props.contributionId)
    : dispatched;
});

type RenderItem = UnifiedSlotItem & { matched?: unknown };

const slotKind = computed(() => browserClientPluginRuntime.slots.getDefinition(props.slotId)?.kind);

const visibleItems = computed<RenderItem[]>(() => {
  if (!scopeReady.value) return [];
  const contract = store.slotsById.get(props.slotId) ?? browserClientPluginRuntime.slots.getDefinition(props.slotId);
  const kind = slotKind.value;
  const serverBase = props.contributionId
    ? visibleContributions.value.filter((item) => item.contributionId === props.contributionId)
    : visibleContributions.value;
  // Strict keyed/chain dispatch is owned by the client Slot runtime. Server
  // contributions are retained as legacy fallback only when no strict client
  // entry matches, preserving Amitia's broader server-driven UI support.
  const server = (kind === "keyed" || kind === "chain") && dispatchedClientContributions.value.length > 0
    ? []
    : serverBase;
  const client = dispatchedClientContributions.value.map((item) => item.contribution);
  const matchedById = new Map(dispatchedClientContributions.value.map((item) => [item.contribution.contributionId, item.matched]));
  const items = buildUnifiedSlotItems(contract, server, client).map((item) =>
    item.source === "client" ? { ...item, matched: matchedById.get(item.client.contributionId) } : item,
  );
  const start = Math.max(0, props.startIndex);
  if (props.maxItems && props.maxItems > 0) return items.slice(start, start + props.maxItems);
  return items.slice(start);
});

const layoutClass = computed(() => `extension-slot--layout-${resolvedLayout.value}`);

const isHidden = computed(() => !scopeReady.value || (!props.chainOverlay && visibleItems.value.length === 0 && resolvedFallback.value === 'none'));

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
    <div
      v-if="chainOverlay"
      class="extension-slot__chain-fallback"
      :style="visibleItems.length > 0 ? { display: 'none' } : undefined"
    >
      <slot name="default"></slot>
    </div>
    <template v-if="!chainOverlay && visibleItems.length === 0 && resolvedFallback !== 'none'">
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
          <ExtensionErrorBoundary
            :slot-id="slotId"
            :contribution-id="item.client.contributionId"
            :abdicate-on-error="item.client.strict && slotKind !== 'chain'"
          >
            <ClientSlotContributionHost
              :contribution="item.client"
              :context="renderContext"
              :matched="item.matched"
              :hook-context="hookContext"
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

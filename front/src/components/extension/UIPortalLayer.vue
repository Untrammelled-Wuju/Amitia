<script setup lang="ts">
import { computed, ref } from "vue";
import ExtensionSlot from "./ExtensionSlot.vue";
import { useExtensionSlot } from "@/composables/useExtensionSlot";
import { browserClientPluginRuntime } from "@/ui-runtime/clientPluginRuntime";

type PortalLayer = "overlay" | "modal" | "drawer" | "toast";

const props = defineProps<{
  layer: PortalLayer;
}>();

const slotId = computed(() => `ui.portal.${props.layer}`);
const surface = ref({
  role: "overlay" as const,
  width: 0,
  height: 0,
  breakpoint: "md" as const,
});
const extensionSlot = useExtensionSlot({
  slotId: slotId.value,
  fallback: "none",
  layout: props.layer === "modal" ? "modal" : props.layer === "drawer" ? "drawer" : "stack",
  surface,
});

const hasContent = computed(() => {
  if (!extensionSlot.scopeReady.value) return false;
  browserClientPluginRuntime.slots.revision.value;
  const client = browserClientPluginRuntime.slots
    .listContributions(slotId.value)
    .some((item) => !item.abdicated);
  return extensionSlot.contributions.value.length > 0 || client;
});
</script>

<template>
  <div
    v-if="hasContent"
    class="ui-portal-layer"
    :class="`ui-portal-layer--${layer}`"
    :style="{ pointerEvents: 'none' }"
    :data-amitia-portal="layer"
  >
    <ExtensionSlot
      :slot-id="slotId"
      fallback="none"
      :layout="layer === 'modal' ? 'modal' : layer === 'drawer' ? 'drawer' : 'stack'"
      surface-role="overlay"
    />
  </div>
</template>

<style scoped>
.ui-portal-layer {
  position: fixed;
  inset: 0;
  display: flex;
  pointer-events: none;
  z-index: 2400;
}

.ui-portal-layer--overlay {
  z-index: 2400;
}

.ui-portal-layer--modal {
  align-items: center;
  justify-content: center;
  z-index: 2500;
}

.ui-portal-layer--drawer {
  justify-content: flex-end;
  z-index: 2450;
}

.ui-portal-layer--toast {
  align-items: flex-end;
  flex-direction: column;
  justify-content: flex-end;
  padding: 24px;
  z-index: 2600;
}

.ui-portal-layer :deep(.extension-slot),
.ui-portal-layer :deep(.extension-slot__contribution),
.ui-portal-layer :deep(.extension-slot__contribution--client) {
  pointer-events: none;
}

.ui-portal-layer :deep(.extension-slot__contribution > *),
.ui-portal-layer :deep(.extension-slot__contribution--client > *) {
  pointer-events: auto;
}
</style>

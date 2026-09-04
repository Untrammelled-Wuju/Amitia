<script setup lang="ts">
import { computed, defineAsyncComponent } from "vue";
import type { UIContributionSummary } from "@/stores/extensionUI";
import ExtensionSurface from "./ExtensionSurface.vue";

const props = defineProps<{ contribution: UIContributionSummary; context: Record<string, unknown>; slotId: string; hostActions?: Record<string, (input?: unknown) => unknown | Promise<unknown>> }>();
const emit = defineEmits<{ (e: "error", error: string): void }>();

const renderer = computed(() => {
  if (props.contribution.sandbox === "schema_renderer") return defineAsyncComponent(() => import("./SchemaUIRenderer.vue"));
  if (["web_restricted", "web_isolated"].includes(props.contribution.sandbox ?? "")) return defineAsyncComponent(() => import("./SandboxWebUIFrame.vue"));
  if (props.contribution.sandbox === "host_native") return defineAsyncComponent(() => import("./HostNativeAction.vue"));
  if (["schema_page", "settings_section", "panel", "card"].includes(props.contribution.kind)) return defineAsyncComponent(() => import("./SchemaUIRenderer.vue"));
  if (props.contribution.kind === "web_page") return defineAsyncComponent(() => import("./SandboxWebUIFrame.vue"));
  if (["action", "menu_item", "toolbar_item", "status_item", "message_action", "composer_action", "desktop_command", "badge"].includes(props.contribution.kind)) return defineAsyncComponent(() => import("./HostNativeAction.vue"));
  return null;
});
const surfaceRole = computed(() => String((props.context.surface as Record<string, unknown> | undefined)?.role ?? "main"));
const needsSurface = computed(() => props.contribution.sandbox !== "host_native" || ["schema_page", "settings_section", "panel", "card", "web_page", "message_renderer"].includes(props.contribution.kind));
</script>

<template>
  <ExtensionSurface v-if="renderer && needsSurface" :role="surfaceRole as any" :bordered="surfaceRole !== 'message' && surfaceRole !== 'composer'">
    <component :is="renderer" :contribution="contribution" :context="context" :slot-id="slotId" :host-actions="hostActions" @error="(error) => emit('error', error)" />
  </ExtensionSurface>
  <component :is="renderer" v-else-if="renderer" :contribution="contribution" :context="context" :slot-id="slotId" :host-actions="hostActions" @error="(error) => emit('error', error)" />
</template>

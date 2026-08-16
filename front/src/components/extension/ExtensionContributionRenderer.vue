<script setup lang="ts">
import { computed, defineAsyncComponent } from "vue";
import type { UIContributionSummary } from "@/stores/extensionUI";
import ExtensionSurface from "./ExtensionSurface.vue";

const props = defineProps<{ contribution: UIContributionSummary; context: Record<string, unknown>; slotId: string }>();

const renderer = computed(() => {
  if (["schema_page", "settings_section", "panel", "card"].includes(props.contribution.kind)) return defineAsyncComponent(() => import("./SchemaUIRenderer.vue"));
  if (props.contribution.kind === "web_page") return defineAsyncComponent(() => import("./SandboxWebUIFrame.vue"));
  if (["action", "menu_item", "toolbar_item", "status_item", "message_action", "composer_action", "desktop_command"].includes(props.contribution.kind)) return defineAsyncComponent(() => import("./HostNativeAction.vue"));
  return null;
});
const surfaceRole = computed(() => String((props.context.surface as Record<string, unknown> | undefined)?.role ?? "main"));
const needsSurface = computed(() => ["schema_page", "settings_section", "panel", "card", "web_page"].includes(props.contribution.kind));
</script>

<template>
  <ExtensionSurface v-if="renderer && needsSurface" :role="surfaceRole as any" :bordered="surfaceRole !== 'message' && surfaceRole !== 'composer'">
    <component :is="renderer" :contribution="contribution" :context="context" :slot-id="slotId" />
  </ExtensionSurface>
  <component :is="renderer" v-else-if="renderer" :contribution="contribution" :context="context" :slot-id="slotId" />
</template>

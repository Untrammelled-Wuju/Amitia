<script setup lang="ts">
import { computed, defineAsyncComponent } from "vue";
import type { UIContributionSummary } from "@/stores/extensionUI";

const props = defineProps<{
  contribution: UIContributionSummary;
  context: Record<string, unknown>;
  slotId: string;
}>();

const runtimeComponents: Record<string, ReturnType<typeof defineAsyncComponent>> = {
  "host.character.proactive": defineAsyncComponent(() => import("@/views/proactive-rules/ProactiveRules.vue")),
  "host.character.psyche": defineAsyncComponent(() => import("@/views/character/CharacterPsycheView.vue")),
  "host.character.lifestyle": defineAsyncComponent(() => import("@/views/companion-debug/CompanionDebugView.vue")),
};

const runtimeComponent = computed(() => {
  const runtimeId = props.contribution.runtimeId?.trim() ?? "";
  return runtimeId ? runtimeComponents[runtimeId] ?? null : null;
});
</script>

<template>
  <component :is="runtimeComponent" v-if="runtimeComponent" />
</template>

<script setup lang="ts">
import { computed } from "vue";
import type { UIContributionSummary } from "@/stores/extensionUI";
import { resolveHostRuntimeComponent } from "./hostRuntimeRegistry";

const props = defineProps<{
  contribution: UIContributionSummary;
  context: Record<string, unknown>;
  slotId: string;
}>();

const runtimeComponent = computed(() => {
  return resolveHostRuntimeComponent(props.contribution.runtimeId ?? "");
});
</script>

<template>
  <component :is="runtimeComponent" v-if="runtimeComponent" />
</template>

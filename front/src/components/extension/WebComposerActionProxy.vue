<script setup lang="ts">
import type { UIContributionSummary } from "@/stores/extensionUI";
import SandboxWebUIFrame from "./SandboxWebUIFrame.vue";

defineProps<{
  contribution: UIContributionSummary;
  context: Record<string, unknown>;
  slotId: string;
}>();
</script>

<template>
  <div class="web-composer-action-proxy" data-amitia-ui="extension.surface">
    <SandboxWebUIFrame
      :contribution="contribution"
      :context="context"
      :slot-id="slotId"
      semantic-target="extension.surface"
    />
  </div>
</template>

<style scoped>
.web-composer-action-proxy {
  position: relative;
  width: 32px;
  height: 32px;
  min-width: 32px;
}
</style>

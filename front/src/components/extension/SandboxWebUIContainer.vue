<script setup lang="ts">
import type { UIContributionSummary } from "@/stores/extensionUI";
import SandboxWebUIFrame from "./SandboxWebUIFrame.vue";
import SandboxWebUIErrorBoundary from "./SandboxWebUIErrorBoundary.vue";

defineProps<{
  contribution: UIContributionSummary;
  slotId: string;
  context?: Record<string, unknown>;
}>();

const emit = defineEmits<{
  (e: "ready", session: string): void;
  (e: "error", error: string): void;
  (e: "action", action: string, input: unknown): void;
}>();
</script>

<template>
  <SandboxWebUIErrorBoundary>
    <SandboxWebUIFrame
      :contribution="contribution"
      :slot-id="slotId"
      :context="context"
      @ready="(s: string) => emit('ready', s)"
      @error="(e: string) => emit('error', e)"
      @action="(a: string, i: unknown) => emit('action', a, i)"
    />
  </SandboxWebUIErrorBoundary>
</template>

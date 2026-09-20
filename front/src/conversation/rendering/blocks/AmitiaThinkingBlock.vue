<template>
  <details class="amrp-thinking" :open="open">
    <summary
      :class="{ 'amrp-thinking-status-only': !hasContent }"
      @click.prevent="toggle"
    >
      <span v-if="hasContent" class="amrp-chevron" :class="{ open }">›</span>
      <span v-if="state === 'streaming'">思考中</span>
      <span v-else-if="duration">思考完成（{{ duration.toFixed(2) }}s）</span>
      <span v-else>思考完成</span>
      <span v-if="state === 'streaming'" class="amrp-thinking-spinner"></span>
    </summary>
    <div v-if="hasContent && open" class="amrp-thinking-body">
      <pre>{{ content }}</pre>
    </div>
  </details>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import type { MessageState } from "../types";

const props = withDefaults(
  defineProps<{
    content: string;
    state?: MessageState;
    duration?: number;
  }>(),
  {
    state: "completed",
    duration: 0,
  },
);

const hasContent = computed(() => String(props.content || "").trim().length > 0);
const open = ref(false);

function toggle() {
  if (!hasContent.value) {
    open.value = false;
    return;
  }
  open.value = !open.value;
}
</script>

<style scoped>
.amrp-thinking {
  display: inline-flex;
  max-width: 100%;
  flex-direction: column;
  align-items: flex-start;
  margin: 2px 0 13px;
  color: var(--amrp-muted);
  font-size: 12px;
}

summary {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 6px 9px;
  border-radius: 8px;
  background: var(--amrp-soft);
  cursor: pointer;
  list-style: none;
  user-select: none;
}

.amrp-thinking-status-only {
  cursor: default;
}

summary::-webkit-details-marker {
  display: none;
}

.amrp-chevron {
  display: inline-block;
  color: var(--amrp-muted);
  font-size: 10px;
  transform: rotate(0);
  transition: transform 150ms ease;
}

.amrp-chevron.open {
  transform: rotate(90deg);
}

.amrp-thinking-spinner {
  width: 11px;
  height: 11px;
  border: 1.5px solid color-mix(in srgb, var(--amrp-accent) 28%, transparent);
  border-top-color: var(--amrp-accent);
  border-radius: 50%;
  animation: amrp-thinking-spin 700ms linear infinite;
}

@keyframes amrp-thinking-spin {
  to { transform: rotate(360deg); }
}

.amrp-thinking-body {
  width: min(700px, 100%);
  margin-top: 6px;
  border-left: 2px solid var(--amrp-line);
  padding: 8px 10px;
  background: color-mix(in srgb, var(--amrp-soft) 70%, transparent);
}

pre {
  margin: 0;
  color: var(--amrp-muted);
  font: inherit;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
</style>

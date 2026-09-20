<template>
  <details class="amrp-thinking" :open="open">
    <summary @click.prevent="open = !open">
      <span class="amrp-chevron" :class="{ open }">›</span>
      <span v-if="state === 'streaming'">正在思考</span>
      <span v-else-if="duration">已思考 {{ duration.toFixed(1) }} 秒</span>
      <span v-else>思考内容</span>
      <span v-if="state === 'streaming'" class="amrp-stream-dot"></span>
    </summary>
    <div class="amrp-thinking-body">
      <div class="amrp-thinking-copy">
        <button type="button" @click="copyThinking">复制</button>
      </div>
      <pre>{{ content }}</pre>
    </div>
  </details>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { ElMessage } from "element-plus";
import { copyText } from "../utils";
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

const open = ref(false);

async function copyThinking() {
  const copied = await copyText(props.content);
  copied ? ElMessage.success("已复制思考内容") : ElMessage.warning("复制失败");
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

.amrp-stream-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--amrp-accent);
  animation: amrp-pulse 1s infinite;
}

@keyframes amrp-pulse {
  50% { opacity: 0.25; }
}

.amrp-thinking-body {
  position: relative;
  width: min(700px, 100%);
  max-height: 260px;
  margin-top: 6px;
  overflow: auto;
  border-left: 2px solid var(--amrp-line);
  padding: 8px 10px;
  background: color-mix(in srgb, var(--amrp-soft) 70%, transparent);
}

.amrp-thinking-copy {
  display: flex;
  justify-content: flex-end;
}

button {
  border: 0;
  border-radius: 6px;
  padding: 3px 6px;
  background: var(--amrp-soft);
  color: var(--amrp-muted);
  font: inherit;
  font-size: 10px;
  cursor: pointer;
}

pre {
  margin: 3px 0 0;
  color: var(--amrp-muted);
  font: inherit;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
</style>

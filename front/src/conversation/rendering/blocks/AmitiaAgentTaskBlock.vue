<template>
  <section class="amrp-agent">
    <header class="amrp-block-head">
      <span class="amrp-title">{{ block.title }}</span>
      <span class="amrp-meta">Agent Task</span>
      <button type="button" @click="expanded = !expanded">{{ expanded ? "收起" : "详情" }}</button>
    </header>
    <div v-if="expanded" class="amrp-agent-body">
      <div class="amrp-agent-progress">
        <span>{{ block.progress ?? 0 }}%</span>
        <div><i :style="{ width: `${Math.max(0, Math.min(100, block.progress ?? 0))}%` }"></i></div>
        <span>{{ block.elapsed || block.status }}</span>
      </div>
      <div class="amrp-steps">
        <div v-for="(step, index) in block.steps" :key="step.id" class="amrp-step" :class="step.status">
          <span class="amrp-step-no">{{ step.status === "done" ? "✓" : index + 1 }}</span>
          <span>{{ step.title }}</span>
          <span class="amrp-step-meta">{{ step.meta || statusLabel(step.status) }}</span>
        </div>
      </div>
      <div v-if="block.error" class="amrp-agent-error">{{ block.error }}</div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref } from "vue";
import type { AgentTaskBlock } from "../types";

defineProps<{
  block: AgentTaskBlock;
}>();

const expanded = ref(true);

function statusLabel(status: string): string {
  return { pending: "等待", running: "进行中", done: "完成", failed: "失败" }[status] ?? status;
}
</script>

<style scoped>
.amrp-agent {
  width: 100%;
  max-width: 700px;
  margin: 12px 0 16px;
  overflow: hidden;
  border: 1px solid var(--amrp-line);
  border-radius: 10px;
  background: var(--amrp-surface);
}

.amrp-block-head {
  min-height: 36px;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 10px;
  border-bottom: 1px solid var(--amrp-line);
  background: var(--amrp-soft);
}

.amrp-title {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  font-size: 11.5px;
  font-weight: 680;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.amrp-meta {
  color: var(--amrp-muted);
  font-size: 10.5px;
}

.amrp-block-head button {
  border: 0;
  border-radius: 6px;
  padding: 4px 7px;
  background: var(--amrp-control);
  color: var(--amrp-text);
  font: inherit;
  font-size: 10px;
  cursor: pointer;
}

.amrp-agent-body {
  padding: 12px;
}

.amrp-agent-progress {
  display: grid;
  grid-template-columns: auto 1fr auto;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  color: var(--amrp-muted);
  font-size: 10px;
}

.amrp-agent-progress div {
  height: 4px;
  overflow: hidden;
  border-radius: 99px;
  background: var(--amrp-soft);
}

.amrp-agent-progress i {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--amrp-accent);
}

.amrp-steps {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.amrp-step {
  display: flex;
  align-items: center;
  gap: 9px;
  color: var(--amrp-text);
  font-size: 12px;
}

.amrp-step-no {
  width: 20px;
  height: 20px;
  display: grid;
  flex: 0 0 auto;
  place-items: center;
  border-radius: 50%;
  background: var(--amrp-soft);
  color: var(--amrp-muted);
  font-size: 9px;
}

.amrp-step.done .amrp-step-no {
  background: color-mix(in srgb, var(--amrp-good) 18%, transparent);
  color: var(--amrp-good);
}

.amrp-step.running .amrp-step-no {
  background: var(--amrp-accent-soft);
  color: var(--amrp-accent);
}

.amrp-step-meta {
  margin-left: auto;
  color: var(--amrp-muted);
  font-size: 10px;
}

.amrp-agent-error {
  margin-top: 10px;
  color: var(--amrp-danger);
  font-size: 11px;
}
</style>


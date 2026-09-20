<template>
  <div class="amrp-tool-wrap">
    <button type="button" class="amrp-tool-line" :class="block.status" @click="expanded = !expanded">
      <span class="amrp-tool-dot"></span>
      <span class="amrp-tool-name">{{ block.name }}</span>
      <span class="amrp-tool-summary">{{ summary }}</span>
      <span class="amrp-tool-state">{{ stateLabel }}</span>
    </button>
    <div v-if="expanded" class="amrp-tool-detail">
      <div v-if="block.arguments !== undefined" class="amrp-tool-section">
        <b>参数</b>
        <pre>{{ prettyJson(block.arguments) }}</pre>
      </div>
      <div v-if="block.result !== undefined" class="amrp-tool-section">
        <b>结果</b>
        <pre class="amrp-tool-result" :class="{ collapsed: !resultExpanded }">{{ prettyJson(block.result) }}</pre>
        <button v-if="resultText.length > 1200" type="button" class="amrp-inline-button" @click="resultExpanded = !resultExpanded">
          {{ resultExpanded ? "收起结果" : "展开完整结果" }}
        </button>
      </div>
      <div v-if="block.error" class="amrp-tool-error">{{ block.error }}</div>
      <div class="amrp-tool-actions">
        <span v-if="block.duration">{{ block.duration }} ms</span>
        <button type="button" @click="copyDetails">复制详情</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { ElMessage } from "element-plus";
import type { ToolBlock } from "../types";
import { copyText, prettyJson } from "../utils";

const props = defineProps<{
  block: ToolBlock;
}>();

const expanded = ref(false);
const resultExpanded = ref(false);
const resultText = computed(() => prettyJson(props.block.result));
const summary = computed(() => {
  if (props.block.error) return props.block.error;
  if (typeof props.block.arguments === "object" && props.block.arguments) {
    const value = props.block.arguments as Record<string, unknown>;
    return String(value.path ?? value.query ?? value.command ?? value.file ?? props.block.name);
  }
  return String(props.block.arguments ?? "");
});
const stateLabel = computed(() => {
  const labels: Record<ToolBlock["status"], string> = {
    queued: "等待",
    running: "运行中",
    success: "完成",
    failed: "失败",
    cancelled: "已取消",
  };
  return `${labels[props.block.status]}${props.block.duration ? ` · ${props.block.duration} ms` : ""}`;
});

async function copyDetails() {
  const copied = await copyText(
    [props.block.name, prettyJson(props.block.arguments), resultText.value, props.block.error]
      .filter(Boolean)
      .join("\n\n"),
  );
  copied ? ElMessage.success("已复制工具详情") : ElMessage.warning("复制失败");
}
</script>

<style scoped>
.amrp-tool-wrap {
  max-width: 700px;
  margin: 8px 0;
}

.amrp-tool-line {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 9px;
  border: 0;
  border-radius: 9px;
  padding: 7px 9px;
  background: var(--amrp-soft);
  color: var(--amrp-tool-text);
  font: inherit;
  font-size: 12px;
  text-align: left;
  cursor: pointer;
}

.amrp-tool-dot {
  width: 7px;
  height: 7px;
  flex: 0 0 auto;
  border-radius: 50%;
  background: #77a982;
}

.amrp-tool-line.running .amrp-tool-dot {
  background: #d1a24d;
  box-shadow: 0 0 0 3px #f4e8ce;
}

.amrp-tool-line.failed .amrp-tool-dot {
  background: #d46b6b;
}

.amrp-tool-line.cancelled .amrp-tool-dot {
  background: #a0a1a6;
}

.amrp-tool-name {
  flex: 0 0 auto;
  color: var(--amrp-text);
  font-weight: 650;
}

.amrp-tool-summary {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.amrp-tool-state {
  flex: 0 0 auto;
  color: var(--amrp-muted);
  font-size: 11px;
}

.amrp-tool-line.failed .amrp-tool-state {
  color: var(--amrp-danger);
}

.amrp-tool-detail {
  margin: -3px 0 9px 16px;
  border-left: 2px solid var(--amrp-line);
  padding: 8px 10px;
  color: var(--amrp-muted);
  font-size: 11px;
}

.amrp-tool-section + .amrp-tool-section {
  margin-top: 8px;
}

.amrp-tool-section b {
  color: var(--amrp-text);
}

pre {
  max-height: 180px;
  margin: 5px 0 0;
  overflow: auto;
  padding: 8px;
  border-radius: 7px;
  background: var(--amrp-soft);
  color: var(--amrp-muted);
  font: 10.5px/1.55 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

pre.collapsed {
  max-height: 130px;
  overflow: hidden;
}

.amrp-tool-error {
  margin-top: 7px;
  color: var(--amrp-danger);
}

.amrp-tool-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 8px;
}

.amrp-tool-actions button,
.amrp-inline-button {
  border: 0;
  border-radius: 6px;
  padding: 4px 7px;
  background: var(--amrp-control);
  color: var(--amrp-accent);
  font: inherit;
  font-size: 10px;
  cursor: pointer;
}

.amrp-tool-actions button {
  margin-left: auto;
}
</style>


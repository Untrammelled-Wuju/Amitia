<template>
  <div class="turn-timeline">
    <template v-for="item in orderedItems" :key="item.id">
      <div
        v-if="item.type === 'thinking'"
        class="turn-thinking"
        :class="{ expanded: expandedThinking.has(item.id) }"
        @click="toggleThinking(item.id)"
      >
        <span class="turn-chev">{{ expandedThinking.has(item.id) ? "⌄" : "›" }}</span>
        <span>{{ thinkingLabel(item) }}</span>
      </div>
      <div v-if="item.type === 'thinking' && expandedThinking.has(item.id)" class="turn-thinking-detail">
        {{ item.content }}
      </div>

      <div v-else-if="item.type === 'tool_group'" class="turn-tool-stream">
        <button
          type="button"
          class="turn-tool-stream-head"
          :aria-expanded="toolStreamExpanded"
          @click="toggleToolStream"
        >
          <span class="turn-chev">{{ toolStreamExpanded ? "⌄" : "›" }}</span>
          <strong>工具执行流</strong>
          <span class="turn-tool-stream-summary">{{ toolStreamSummary(item.items) }}</span>
          <span class="turn-tool-stream-toggle">{{ toolStreamExpanded ? "收起" : "展开" }}</span>
        </button>
        <div v-if="toolStreamExpanded" class="turn-tool-stream-body">
          <template v-for="toolItem in item.items" :key="toolItem.id">
            <div
              v-if="toolItem.type === 'tool_call'"
              class="turn-tool-line"
              :class="statusClass(toolItem.status)"
            >
              <span class="turn-tool-dot"></span>
              <span class="turn-tool-name">{{ toolItem.toolName || "工具调用" }}</span>
              <span class="turn-tool-subject">{{ toolSubject(toolItem) }}</span>
              <span class="turn-tool-state">{{ toolStateLabel(toolItem) }}</span>
            </div>
            <div v-else-if="toolItem.type === 'tool_result'" class="turn-tool-result">
              <button type="button" class="turn-result-head" @click="toggleResult(toolItem.id)">
                <span class="turn-tool-dot" :class="statusClass(toolItem.status)"></span>
                <strong>{{ toolItem.toolName || "工具结果" }}</strong>
                <span>{{ resultSummary(toolItem) }}</span>
                <span class="turn-result-toggle">{{ expandedResults.has(toolItem.id) ? "收起" : "展开" }}</span>
              </button>
              <pre
                v-if="expandedResults.has(toolItem.id)"
                class="turn-result-body"
              >{{ resultText(toolItem) }}</pre>
            </div>
          </template>
        </div>
      </div>

      <div v-else-if="item.type === 'text'" class="turn-text">
        <RendererErrorBoundary label="Turn Text Renderer">
          <MarkdownContent
            v-if="item.content"
            :source="item.content"
            :streaming="isStreamingStatus(item.status)"
          />
        </RendererErrorBoundary>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import type { AssistantTurnData, AssistantTurnItem } from "./types";
import MarkdownContent from "./markdown/MarkdownContent.vue";
import RendererErrorBoundary from "./blocks/RendererErrorBoundary.vue";

const props = defineProps<{
  turn: AssistantTurnData;
}>();

const expandedThinking = reactive(new Set<string>());
const expandedResults = reactive(new Set<string>());
const toolStreamExpanded = ref(hasRunningTool(props.turn));
const toolStreamTouched = ref(false);

const orderedItems = computed(() => {
  const items = [...(props.turn.items || [])].sort((left, right) => left.sequence - right.sequence);
  const result: Array<AssistantTurnItem & { type: string; items?: AssistantTurnItem[] }> = [];
  let toolGroup: (AssistantTurnItem & { type: string; items?: AssistantTurnItem[] }) | undefined;
  for (const item of items) {
    if (item.type === "tool_call" || item.type === "tool_result") {
      if (!toolGroup) {
        toolGroup = { ...item, type: "tool_group", items: [] };
        result.push(toolGroup);
      }
      toolGroup.items!.push(item);
      continue;
    }
    toolGroup = undefined;
    result.push(item);
  }
  return result;
});

watch(
  () => props.turn.status,
  (status) => {
    if (toolStreamTouched.value) return;
    toolStreamExpanded.value = isStreamingStatus(status);
  },
);

function toggleThinking(id: string) {
  expandedThinking.has(id) ? expandedThinking.delete(id) : expandedThinking.add(id);
}

function toggleResult(id: string) {
  expandedResults.has(id) ? expandedResults.delete(id) : expandedResults.add(id);
}

function toggleToolStream() {
  toolStreamTouched.value = true;
  toolStreamExpanded.value = !toolStreamExpanded.value;
}

function hasRunningTool(turn: AssistantTurnData): boolean {
  return (turn.items || []).some(
    (item) =>
      (item.type === "tool_call" || item.type === "tool_result") &&
      isStreamingStatus(item.status),
  );
}

function toolStreamSummary(items?: AssistantTurnItem[]): string {
  const values = items || [];
  const calls = values.filter((item) => item.type === "tool_call");
  const failed = values.filter((item) => statusClass(item.status) === "failed").length;
  if (failed > 0) return `${calls.length || values.length} 个工具 · ${failed} 个失败`;
  if (values.some((item) => statusClass(item.status) === "running")) {
    const completed = values.filter((item) => statusClass(item.status) === "completed").length;
    return `执行中 · ${completed}/${calls.length || values.length} 完成`;
  }
  return `${calls.length || values.length} 个工具 · 已完成`;
}

function statusClass(status: string): string {
  const value = String(status || "").toLowerCase();
  if (["failed", "error", "unknown"].includes(value)) return "failed";
  if (["cancelled", "canceled", "stopped"].includes(value)) return "cancelled";
  if (["completed", "success", "succeeded", "sent", "delivered"].includes(value)) return "completed";
  return "running";
}

function isStreamingStatus(status: string): boolean {
  const value = String(status || "").toLowerCase();
  return ["running", "streaming", "sending", "pending", "queued"].includes(value);
}

function parseJSON(value?: string): unknown {
  const source = String(value || "").trim();
  if (!source) return undefined;
  try {
    return JSON.parse(source);
  } catch {
    return source;
  }
}

function stringify(value: unknown): string {
  if (value === undefined || value === null) return "";
  if (typeof value === "string") return value;
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

function thinkingLabel(item: AssistantTurnItem): string {
  const duration = Number(item.durationMs || 0);
  if (duration > 0) return `已思考 ${(duration / 1000).toFixed(1)} 秒`;
  return isStreamingStatus(item.status) ? "正在思考" : "已思考";
}

function toolSubject(item: AssistantTurnItem): string {
  const value = parseJSON(item.argumentsJson);
  if (value && typeof value === "object" && !Array.isArray(value)) {
    const record = value as Record<string, unknown>;
    for (const key of ["path", "file", "filePath", "query", "command", "url", "cwd"]) {
      if (record[key] !== undefined && String(record[key]).trim()) {
        return String(record[key]).trim();
      }
    }
  }
  const text = stringify(value).replace(/\s+/g, " ").trim();
  return text.length > 72 ? `${text.slice(0, 72)}…` : text;
}

function toolStateLabel(item: AssistantTurnItem): string {
  const status = statusClass(item.status);
  const duration = Number(item.durationMs || 0);
  const suffix = duration > 0 ? ` · ${duration} ms` : "";
  if (status === "completed") return `完成${suffix}`;
  if (status === "failed") return `失败${suffix}`;
  if (status === "cancelled") return "已取消";
  return `运行中${suffix}`;
}

function resultText(item: AssistantTurnItem): string {
  const value = parseJSON(item.resultJson);
  const text = stringify(value);
  if (text) return text;
  return item.errorCode || "无返回内容";
}

function resultSummary(item: AssistantTurnItem): string {
  const text = resultText(item).replace(/\s+/g, " ").trim();
  if (!text) return "";
  return text.length > 80 ? `${text.slice(0, 80)}…` : text;
}
</script>

<style scoped>
.turn-timeline {
  display: flex;
  flex-direction: column;
  max-width: 700px;
}

.turn-thinking {
  display: inline-flex;
  width: fit-content;
  align-items: center;
  gap: 7px;
  margin: 2px 0 13px;
  padding: 6px 9px;
  border-radius: 8px;
  background: var(--tp-panel-soft, #efeff1);
  color: var(--tp-text-muted, #6f7178);
  font-size: 12px;
  cursor: pointer;
  user-select: none;
}

.turn-chev {
  color: var(--tp-text-secondary, #999);
  font-size: 10px;
}

.turn-thinking-detail {
  margin: -7px 0 13px;
  padding: 8px 10px;
  border-left: 2px solid var(--tp-border, #dedee3);
  color: var(--tp-text-secondary, #7b7d84);
  font-size: 11.5px;
  line-height: 1.65;
  white-space: pre-wrap;
}

.turn-tool-stream {
  margin: 8px 0 15px;
  overflow: hidden;
  border: 1px solid var(--tp-border, #e8e8eb);
  border-radius: 10px;
  background: color-mix(in srgb, var(--tp-panel-soft, #f1f1f3) 48%, transparent);
}

.turn-tool-stream-head {
  display: flex;
  width: 100%;
  align-items: center;
  gap: 8px;
  border: 0;
  padding: 9px 10px;
  background: transparent;
  color: var(--tp-text-secondary, #64666c);
  font: inherit;
  font-size: 12px;
  text-align: left;
  cursor: pointer;
}

.turn-tool-stream-head:hover,
.turn-tool-stream-head:focus-visible {
  background: var(--tp-panel-soft, #f1f1f3);
}

.turn-tool-stream-head strong {
  flex: 0 0 auto;
  color: var(--tp-text, #4d4f55);
}

.turn-tool-stream-summary {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  color: var(--tp-text-tertiary, #989aa0);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.turn-tool-stream-toggle {
  flex: 0 0 auto;
  color: var(--tp-primary, #7060e8);
}

.turn-tool-stream-body {
  padding: 0 8px 8px;
  border-top: 1px solid var(--tp-border, #e8e8eb);
}

.turn-tool-line {
  display: flex;
  align-items: center;
  gap: 9px;
  margin: 8px 0;
  padding: 7px 9px;
  border-radius: 9px;
  background: var(--tp-panel-soft, #f0f0f2);
  color: var(--tp-text-secondary, #5f6168);
  font-size: 12px;
}

@media (prefers-reduced-motion: no-preference) {
  .turn-tool-stream-head {
    transition: background-color 0.18s ease;
  }
}

.turn-tool-name {
  color: var(--tp-text, #4d4f55);
  font-weight: 650;
}

.turn-tool-subject {
  min-width: 0;
  overflow: hidden;
  color: var(--tp-text-secondary, #85878e);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.turn-tool-state {
  margin-left: auto;
  color: var(--tp-text-tertiary, #989aa0);
  font-size: 11px;
  white-space: nowrap;
}

.turn-tool-dot {
  width: 7px;
  height: 7px;
  flex: 0 0 auto;
  border-radius: 50%;
  background: #77a982;
}

.turn-tool-line.running .turn-tool-dot,
.turn-tool-dot.running {
  background: #d1a24d;
  box-shadow: 0 0 0 3px #f4e8ce;
}

.turn-tool-line.failed .turn-tool-dot,
.turn-tool-dot.failed {
  background: #d46b6b;
  box-shadow: none;
}

.turn-tool-line.cancelled .turn-tool-dot,
.turn-tool-dot.cancelled {
  background: #a0a1a6;
  box-shadow: none;
}

.turn-tool-line.failed .turn-tool-state {
  color: var(--tp-danger, #c85353);
}

.turn-tool-result {
  margin: 8px 0 15px;
  overflow: hidden;
  border: 1px solid var(--tp-border, #e8e8eb);
  border-radius: 9px;
}

.turn-result-head {
  display: flex;
  width: 100%;
  align-items: center;
  gap: 8px;
  border: 0;
  padding: 7px 9px;
  background: var(--tp-panel-soft, #f1f1f3);
  color: var(--tp-text-secondary, #64666c);
  font: inherit;
  font-size: 11px;
  text-align: left;
  cursor: pointer;
}

.turn-result-head strong {
  color: var(--tp-text, #4d4f55);
}

.turn-result-head > span:not(.turn-tool-dot):not(.turn-result-toggle) {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.turn-result-toggle {
  margin-left: auto;
  color: var(--tp-primary, #7060e8);
  white-space: nowrap;
}

.turn-result-body {
  max-height: 220px;
  margin: 0;
  overflow: auto;
  padding: 9px 11px;
  color: var(--tp-text-secondary, #666);
  font: 10.5px/1.55 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  white-space: pre-wrap;
}
</style>

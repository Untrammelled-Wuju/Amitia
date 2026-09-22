<template>
  <div class="turn-timeline">
    <template v-for="item in orderedItems" :key="timelineItemKey(item)">
      <AmitiaThinkingBlock
        v-if="item.type === 'reasoning'"
        :content="item.content || ''"
        :state="thinkingState(item.status)"
        :duration="thinkingDuration(item)"
      />

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
              <span class="turn-tool-name">{{ toolDisplayName(toolItem.toolName, "工具调用") }}</span>
              <span class="turn-tool-subject">{{ toolSubject(toolItem) }}</span>
              <span class="turn-tool-state">{{ toolStateLabel(toolItem) }}</span>
            </div>
            <div v-else-if="toolItem.type === 'tool_result'" class="turn-tool-result">
              <button type="button" class="turn-result-head" @click="toggleResult(toolItem.id)">
                <span class="turn-tool-dot" :class="statusClass(toolItem.status)"></span>
                <strong>{{ toolDisplayName(toolItem.toolName, "工具结果") }}</strong>
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
            :citation-ids="citationIds"
            @citation="emit('citation', $event)"
          />
        </RendererErrorBoundary>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import type {
  AssistantTurnData,
  AssistantTurnItem,
  MessageState,
} from "./types";
import AmitiaThinkingBlock from "./blocks/AmitiaThinkingBlock.vue";
import MarkdownContent from "./markdown/MarkdownContent.vue";
import RendererErrorBoundary from "./blocks/RendererErrorBoundary.vue";

const props = defineProps<{
  turn: AssistantTurnData;
  citationIds?: string[];
}>();

const citationIds = computed(() => props.citationIds ?? []);

const emit = defineEmits<{
  citation: [id: string];
}>();

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
  if (value === "interrupted") return "interrupted";
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

function thinkingState(status: string): MessageState {
  return isStreamingStatus(status) ? "streaming" : "completed";
}

function thinkingDuration(item: AssistantTurnItem): number {
  const duration = Number(item.durationMs || 0);
  return Number.isFinite(duration) && duration > 0 ? duration / 1000 : 0;
}

function timelineItemKey(item: AssistantTurnItem & { type: string }): string {
  if (item.type !== "reasoning") return item.id;
  const thinkingItems = orderedItems.value.filter(
    (candidate) => candidate.type === "reasoning",
  );
  const index = thinkingItems.findIndex((candidate) => candidate.id === item.id);
  return `thinking:${props.turn.id}:${Math.max(0, index)}`;
}

function toolDisplayName(name?: string, fallback = "工具调用"): string {
  const value = String(name || "").trim();
  if (!value) return fallback;
  if (value === "web_run" || value === "web.run") return "联网研究";
  return value;
}

function toolSubject(item: AssistantTurnItem): string {
  const progress = String(item.content || "").replace(/\s+/g, " ").trim();
  if (progress && isStreamingStatus(item.status)) {
    return progress.length > 96 ? `${progress.slice(0, 96)}…` : progress;
  }
  const value = parseJSON(item.argumentsJson);
  if (value && typeof value === "object" && !Array.isArray(value)) {
    const record = value as Record<string, unknown>;
    const searchQueries = Array.isArray(record.search_query) ? record.search_query : [];
    if (searchQueries.length > 0) {
      const first = searchQueries[0];
      if (first && typeof first === "object" && !Array.isArray(first)) {
        const query = String((first as Record<string, unknown>).q || "").trim();
        if (query) return searchQueries.length > 1 ? `${query} · ${searchQueries.length} 个查询` : query;
      }
    }
    for (const key of ["path", "file", "filePath", "query", "command", "url", "cwd"]) {
      if (record[key] !== undefined && String(record[key]).trim()) {
        return String(record[key]).trim();
      }
    }
    for (const key of ["open", "find", "click", "screenshot"]) {
      const commands = Array.isArray(record[key]) ? record[key] : [];
      if (commands.length > 0) return `${key} · ${commands.length}`;
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
  if (status === "interrupted") return "已中断";
  return `运行中${suffix}`;
}

function resultText(item: AssistantTurnItem): string {
  const value = parseJSON(item.resultJson);
  const text = stringify(value);
  if (text) return text;
  return item.errorCode || "无返回内容";
}

function resultSummary(item: AssistantTurnItem): string {
  const value = parseJSON(item.resultJson);
  if ((item.toolName === "web_run" || item.toolName === "web.run") && value && typeof value === "object" && !Array.isArray(value)) {
    const record = value as Record<string, unknown>;
    const results = Array.isArray(record.search) ? record.search.length : 0;
    const pages = Array.isArray(record.pages) ? record.pages.length : 0;
    const citations = Array.isArray(record.citations) ? record.citations.length : 0;
    const operation = String(record.operation || "research").trim();
    const parts = [operation === "search" ? "搜索完成" : operation === "open" ? "网页读取完成" : "研究完成"];
    if (results > 0) parts.push(`${results} 个来源`);
    if (pages > 0) parts.push(`读取 ${pages} 页`);
    if (citations > 0) parts.push(`${citations} 条证据`);
    const research = record.research;
    if (research && typeof research === "object" && !Array.isArray(research)) {
      const rounds = Number((research as Record<string, unknown>).rounds_completed || 0);
      if (rounds > 1) parts.push(`${rounds} 轮`);
    }
    return parts.join(" · ");
  }
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

.turn-chev {
  color: var(--tp-text-secondary, #999);
  font-size: 10px;
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

.turn-tool-line.interrupted .turn-tool-dot,
.turn-tool-dot.interrupted {
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

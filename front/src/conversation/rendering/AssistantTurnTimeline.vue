<template>
  <div class="turn-timeline">
    <template v-for="item in orderedItems" :key="timelineItemKey(item)">
      <AmitiaThinkingBlock
        v-if="item.type === 'reasoning'"
        class="turn-thinking"
        :content="item.content || ''"
        :state="thinkingState(item.status)"
        :duration="thinkingDuration(item)"
      />

      <div v-else-if="item.type === 'tool_group'" class="turn-tool-stream">
        <button
          type="button"
          class="turn-tool-stream-head"
          :aria-expanded="isToolStreamExpanded(item)"
          @click="toggleToolStream(item)"
        >
          <span class="turn-chev">{{ isToolStreamExpanded(item) ? "⌄" : "›" }}</span>
          <strong>执行过程</strong>
          <span class="turn-tool-stream-summary">{{ toolStreamSummary(item.items) }}</span>
          <span class="turn-tool-stream-toggle">{{ isToolStreamExpanded(item) ? "收起" : "展开" }}</span>
        </button>
        <div v-if="isToolStreamExpanded(item)" class="turn-tool-stream-body">
          <template v-for="entry in toolEntries(item.items)" :key="entry.key">
            <div class="turn-tool-item">
              <div class="turn-tool-line" :class="statusClass(entry.call.status)">
                <span class="turn-tool-dot"></span>
                <span class="turn-tool-name">{{ toolDisplayName(entry.call.toolName, "工具调用") }}</span>
                <span class="turn-tool-subject">{{ toolSubject(entry.call) }}</span>
                <span class="turn-tool-state">{{ toolStateLabel(entry.call, entry.result) }}</span>
                <button
                  type="button"
                  class="turn-tool-expand"
                  @click="toggleToolEntry(entry.key)"
                >
                  {{ expandedToolEntries.has(entry.key) ? "收起" : "展开" }}
                </button>
              </div>
              <div v-if="expandedToolEntries.has(entry.key)" class="turn-tool-details">
                <div class="turn-tool-detail">
                  <strong>调用工具</strong>
                  <pre>{{ entry.call.toolName || "工具调用" }}</pre>
                </div>
                <div class="turn-tool-detail">
                  <strong>调用参数</strong>
                  <pre>{{ toolArguments(entry.call) }}</pre>
                </div>
                <div class="turn-tool-detail">
                  <strong>调用结果</strong>
                  <pre>{{ resultText(entry.result || entry.call) }}</pre>
                </div>
              </div>
            </div>
          </template>
        </div>
      </div>

      <div v-else-if="item.type === 'text'" class="turn-text">
        <RendererErrorBoundary label="Turn Text Renderer">
          <MarkdownContent
            v-if="item.content"
            :source="flowBubbleText(item.content)"
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
import { computed, onBeforeUnmount, reactive, ref, watch } from "vue";
import type {
  AssistantTurnData,
  AssistantTurnItem,
  MessageState,
} from "./types";
import AmitiaThinkingBlock from "./blocks/AmitiaThinkingBlock.vue";
import MarkdownContent from "./markdown/MarkdownContent.vue";
import RendererErrorBoundary from "./blocks/RendererErrorBoundary.vue";
import { flowBubbleText } from "./bubbleMessageProjection";

const props = defineProps<{
  turn: AssistantTurnData;
  citationIds?: string[];
}>();

const citationIds = computed(() => props.citationIds ?? []);

const emit = defineEmits<{
  citation: [id: string];
}>();

type TimelineToolStreamItem = AssistantTurnItem & {
  type: string;
  items?: AssistantTurnItem[];
};

interface TimelineToolEntry {
  key: string;
  call: AssistantTurnItem;
  result?: AssistantTurnItem;
}

const expandedToolEntries = reactive(new Set<string>());
const toolStreamOverrides = reactive(new Map<string, boolean>());
const durationClock = ref(Date.now());
let durationTimer: ReturnType<typeof setInterval> | undefined;

const orderedItems = computed(() => {
  const items = [...(props.turn.items || [])].sort((left, right) => left.sequence - right.sequence);
  const result: Array<AssistantTurnItem & { type: string; items?: AssistantTurnItem[] }> = [];
  let toolGroup: (AssistantTurnItem & { type: string; items?: AssistantTurnItem[] }) | undefined;
  for (const item of items) {
    if (item.type === "text" && !String(item.content || "").trim()) continue;
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
  () => orderedItems.value.some((item) => item.type === "tool_group" && hasRunningTool(item.items)),
  (running) => {
    if (durationTimer) {
      clearInterval(durationTimer);
      durationTimer = undefined;
    }
    if (!running) return;
    durationTimer = setInterval(() => {
      durationClock.value = Date.now();
    }, 250);
  },
  { immediate: true },
);

onBeforeUnmount(() => {
  if (durationTimer) clearInterval(durationTimer);
  durationTimer = undefined;
});

function toggleToolEntry(id: string) {
  expandedToolEntries.has(id) ? expandedToolEntries.delete(id) : expandedToolEntries.add(id);
}

function toolStreamKey(item: TimelineToolStreamItem): string {
  return `tool-stream:${props.turn.id}:${item.id}`;
}

function isToolStreamExpanded(item: TimelineToolStreamItem): boolean {
  const override = toolStreamOverrides.get(toolStreamKey(item));
  if (override !== undefined) return override;
  return hasRunningTool(item.items);
}

function toggleToolStream(item: TimelineToolStreamItem) {
  toolStreamOverrides.set(toolStreamKey(item), !isToolStreamExpanded(item));
}

function hasRunningTool(items?: AssistantTurnItem[]): boolean {
  return (items || []).some(
    (item) =>
      (item.type === "tool_call" || item.type === "tool_result") &&
      isStreamingStatus(item.status),
  );
}

function toolStreamSummary(items?: AssistantTurnItem[]): string {
  const values = items || [];
  const calls = uniqueToolRecords(values.filter((item) => item.type === "tool_call"));
  const results = uniqueToolRecords(values.filter((item) => item.type === "tool_result"));
  const counted = calls.length > 0 ? calls : results;
  const failed = counted.filter((item) => statusClass(item.status) === "failed").length;
  if (failed > 0) return `${counted.length} 个工具 · ${failed} 个失败`;
  const running = counted.find((item) => statusClass(item.status) === "running");
  if (running) {
    const completed = counted.filter((item) => statusClass(item.status) === "completed").length;
    const duration = formatDuration(toolDurationMs(running));
    const count = counted.length > 1 ? ` · ${completed}/${counted.length} 完成` : "";
    return `${toolDisplayName(running.toolName)}${duration ? ` · ${duration}` : ""}${count}`;
  }
  return `${counted.length} 个工具 · 已完成`;
}

function uniqueToolRecords(items: AssistantTurnItem[]): AssistantTurnItem[] {
  const records = new Map<string, AssistantTurnItem>();
  for (const item of items) {
    const key = String(item.callId || item.id || "").trim();
    if (!key) continue;
    const current = records.get(key);
    if (!current || Number(item.revision || 0) >= Number(current.revision || 0)) {
      records.set(key, item);
    }
  }
  return [...records.values()];
}

function toolEntries(items?: AssistantTurnItem[]): TimelineToolEntry[] {
  const values = items || [];
  const results = uniqueToolRecords(values.filter((item) => item.type === "tool_result"));
  const resultsByCall = new Map<string, AssistantTurnItem>();
  for (const result of results) {
    const key = toolKey(result);
    if (key) resultsByCall.set(key, result);
  }
  const entries: TimelineToolEntry[] = [];
  const matched = new Set<string>();
  for (const call of uniqueToolRecords(values.filter((item) => item.type === "tool_call"))) {
    const key = toolKey(call);
    const result = key ? resultsByCall.get(key) : undefined;
    if (key) matched.add(key);
    entries.push({ key: key || call.id, call, result });
  }
  for (const result of results) {
    const key = toolKey(result);
    if (key && matched.has(key)) continue;
    entries.push({ key: key || result.id, call: result, result });
  }
  return entries;
}

function toolKey(item: AssistantTurnItem): string {
  return String(item.callId || item.id || "").trim();
}

function toolArguments(item: AssistantTurnItem): string {
  const value = parseJSON(item.argumentsJson);
  const text = stringify(value).trim();
  return text || "无参数";
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
  } catch (error) {
  console.error("[original-error]", error);
return source;
  }
}

function stringify(value: unknown): string {
  if (value === undefined || value === null) return "";
  if (typeof value === "string") return value;
  try {
    return JSON.stringify(value, null, 2);
  } catch (error) {
  console.error("[original-error]", error);
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
  const value = String(name || "").trim().toLowerCase();
  if (!value) return fallback;
  if (value.includes("multi_agent")) return "协调子任务";
  if (value.includes("browser") || value.includes("navigate") || value.includes("screenshot")) return "操作浏览器";
  if (value.includes("web") && value.includes("search")) return "搜索网页";
  if (value.includes("memory") && (value.includes("search") || value.includes("recall"))) return "检索记忆";
  if (value.includes("command") || value.includes("shell") || value.includes("terminal") || value.includes("exec") || value.includes("bash")) return "运行命令";
  if (value.includes("write") || value.includes("patch") || value.includes("replace") || value.includes("edit")) return "修改文件";
  if (value.includes("read") || value.includes("cat") || value.includes("open_file")) return "读取文件";
  if (value.includes("search") || value.includes("grep") || value.includes("find")) return "搜索内容";
  if (value.includes("list") || value.includes("glob") || value.includes("directory")) return "浏览文件";
  if (value.includes("invite")) return "邀请角色";
  return String(name || "").trim();
}

function toolSubject(item: AssistantTurnItem): string {
  const progress = String(item.content || "").replace(/\s+/g, " ").trim();
  if (progress && isStreamingStatus(item.status)) {
    return progress.length > 96 ? `${progress.slice(0, 96)}…` : progress;
  }
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

function toolStateLabel(item: AssistantTurnItem, result?: AssistantTurnItem): string {
  const statusSource = statusClass(item.status) === "running" && result ? result.status : item.status;
  const status = statusClass(statusSource);
  const duration = formatDuration(toolDurationMs(item, result));
  const suffix = duration ? ` · ${duration}` : "";
  if (status === "completed") return `完成${suffix}`;
  if (status === "failed") return `失败${suffix}`;
  if (status === "interrupted") return `已中断${suffix}`;
  return `运行中${suffix}`;
}

function toolDurationMs(item: AssistantTurnItem, result?: AssistantTurnItem): number {
  const recorded = Number(item.durationMs || 0);
  if (Number.isFinite(recorded) && recorded > 0) return recorded;
  const resultDuration = Number(result?.durationMs || 0);
  if (Number.isFinite(resultDuration) && resultDuration > 0) return resultDuration;
  const startedAt = Date.parse(String(item.createdAt || ""));
  if (!Number.isFinite(startedAt)) return 0;
  const status = result && !isStreamingStatus(result.status) ? result.status : item.status;
  if (isStreamingStatus(status)) return Math.max(0, durationClock.value - startedAt);
  const finishedAt = Date.parse(String(result?.updatedAt || item.updatedAt || ""));
  return Number.isFinite(finishedAt) ? Math.max(0, finishedAt - startedAt) : 0;
}

function formatDuration(milliseconds: number): string {
  if (!Number.isFinite(milliseconds) || milliseconds <= 0) return "";
  if (milliseconds < 1000) return `${Math.max(1, Math.round(milliseconds))}ms`;
  if (milliseconds < 60_000) return `${(milliseconds / 1000).toFixed(milliseconds < 10_000 ? 1 : 0)}s`;
  const seconds = Math.round(milliseconds / 1000);
  return `${Math.floor(seconds / 60)}m ${String(seconds % 60).padStart(2, "0")}s`;
}

function resultText(item: AssistantTurnItem): string {
  const value = parseJSON(item.resultJson);
  const text = stringify(value);
  if (text) return text;
  return item.errorCode || "无返回内容";
}

</script>

<style scoped>
.turn-timeline {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-width: 700px;
}

.turn-timeline .turn-thinking {
  margin: 0;
}

.turn-chev {
  color: var(--tp-text-secondary, #999);
  font-size: 10px;
}

.turn-tool-stream {
  margin: 0;
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
  color: var(--tp-primary);
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
  min-width: 0;
  flex: 0 1 auto;
  overflow: hidden;
  color: var(--tp-text, #4d4f55);
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
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

.turn-tool-expand {
  flex: 0 0 auto;
  border: 0;
  padding: 2px 0 2px 6px;
  background: transparent;
  color: var(--tp-primary);
  font: inherit;
  font-size: 10.5px;
  cursor: pointer;
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

.turn-tool-details {
  border-top: 1px solid var(--tp-border, #e8e8eb);
  padding: 9px 11px 10px;
}

.turn-tool-detail + .turn-tool-detail {
  margin-top: 9px;
}

.turn-tool-detail strong {
  display: block;
  margin-bottom: 5px;
  color: var(--tp-text, #4d4f55);
  font-size: 10.5px;
}

.turn-tool-detail pre {
  max-height: 240px;
  margin: 0;
  overflow: auto;
  color: var(--tp-text-secondary, #666);
  font: 10.5px/1.55 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  white-space: pre-wrap;
}
</style>

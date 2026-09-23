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
  if (isWebResearchTool(item.toolName) && value && typeof value === "object" && !Array.isArray(value)) {
    const formatted = formatWebResearchDetails(value as Record<string, unknown>);
    if (formatted) return formatted;
  }
  const text = stringify(value);
  if (text) return text;
  return item.errorCode || "无返回内容";
}

function isWebResearchTool(name?: string): boolean {
  return name === "web_run" || name === "web.run";
}

function formatWebResearchDetails(record: Record<string, unknown>): string {
  const lines: string[] = [];
  const operation = String(record.operation || "research").trim();
  const operationLabel: Record<string, string> = {
    search: "搜索",
    open: "网页读取",
    find: "页面查找",
    click: "链接读取",
    screenshot: "视觉证据",
  };
  lines.push(`联网研究 · ${operationLabel[operation] || "研究"}`);

  const research = record.research;
  if (research && typeof research === "object" && !Array.isArray(research)) {
    const researchRecord = research as Record<string, unknown>;
    const plan = researchRecord.plan;
    const unresolved = new Set(
      (Array.isArray(researchRecord.unresolved_questions) ? researchRecord.unresolved_questions : [])
        .map((value) => String(value || "").trim())
        .filter(Boolean),
    );
    if (plan && typeof plan === "object" && !Array.isArray(plan)) {
      const planRecord = plan as Record<string, unknown>;
      const goal = String(planRecord.goal || "").trim();
      if (goal) lines.push("", `研究目标：${goal}`);
      const questions = Array.isArray(planRecord.questions) ? planRecord.questions : [];
      if (questions.length > 0) {
        lines.push("", "研究计划");
        for (const question of questions) {
          if (!question || typeof question !== "object" || Array.isArray(question)) continue;
          const text = String((question as Record<string, unknown>).question || "").trim();
          if (!text) continue;
          lines.push(`${unresolved.has(text) ? "…" : "✓"} ${text}`);
        }
      }
    }
    const findings = Array.isArray(researchRecord.findings) ? researchRecord.findings : [];
    if (findings.length > 0) {
      lines.push("", "研究结论");
      for (const finding of findings.slice(0, 12)) {
        if (!finding || typeof finding !== "object" || Array.isArray(finding)) continue;
        const row = finding as Record<string, unknown>;
        const question = String(row.question || "").trim();
        const status = String(row.status || "insufficient").trim();
        const icon = status === "corroborated" ? "✓✓" : status === "supported" ? "✓" : "…";
        if (question) lines.push(`${icon} ${question}`);
        const summary = String(row.summary || "").trim();
        if (summary) lines.push(`  ${summary.replace(/\s+/g, " ").slice(0, 600)}`);
      }
    }
    const stopReason = String(researchRecord.stop_reason || "").trim();
    if (stopReason) lines.push("", `停止条件：${researchStopReasonText(stopReason)}`);
  }

  const search = Array.isArray(record.search) ? record.search : [];
  if (search.length > 0) {
    lines.push("", `来源（${search.length}）`);
    for (const source of search.slice(0, 20)) {
      if (!source || typeof source !== "object" || Array.isArray(source)) continue;
      const row = source as Record<string, unknown>;
      const title = String(row.title || row.url || "来源").trim();
      const url = String(row.url || "").trim();
      lines.push(`- ${title}${url ? `\n  ${url}` : ""}`);
    }
  }

  const citations = Array.isArray(record.citations) ? record.citations : [];
  if (citations.length > 0) {
    lines.push("", `证据与引用（${citations.length}）`);
    for (const citation of citations.slice(0, 24)) {
      if (!citation || typeof citation !== "object" || Array.isArray(citation)) continue;
      const row = citation as Record<string, unknown>;
      const index = Number(row.index || 0);
      const title = String(row.title || row.url || "证据").trim();
      const locator = row.locator;
      let locatorText = "";
      if (locator && typeof locator === "object" && !Array.isArray(locator)) {
        const loc = locator as Record<string, unknown>;
        if (loc.kind === "pdf_page") {
          const page = Number(loc.page);
          if (Number.isFinite(page)) locatorText = ` · PDF 第 ${page + 1} 页`;
        }
      }
      lines.push(`- ${index > 0 ? `[${index}] ` : ""}${title}${locatorText}`);
    }
  }

  const graph = record.evidence_graph;
  if (graph && typeof graph === "object" && !Array.isArray(graph)) {
    const graphRecord = graph as Record<string, unknown>;
    const conflicts = Number(graphRecord.conflict_count || 0);
    if (conflicts > 0) lines.push("", `证据冲突：${conflicts} 组（已保留供最终回答审计）`);
  }

  const security = record.security;
  if (security && typeof security === "object" && !Array.isArray(security)) {
    const securityRecord = security as Record<string, unknown>;
    if (securityRecord.potential_prompt_injection === true) {
      lines.push("", "安全：检测到网页中的指令型文本，已按不可信外部内容处理。");
    }
  }

  const stats = record.stats;
  const cost = research && typeof research === "object" && !Array.isArray(research)
    ? (research as Record<string, unknown>).cost
    : undefined;
  if (stats && typeof stats === "object" && !Array.isArray(stats)) {
    const row = stats as Record<string, unknown>;
    const calls = Number(row.search_calls || 0);
    const fetches = Number(row.fetch_calls || 0);
    const duration = Number(row.duration_ms || 0);
    const summary: string[] = [];
    if (calls > 0) summary.push(`${calls} 次搜索`);
    if (fetches > 0) summary.push(`${fetches} 次抓取`);
    if (duration > 0) summary.push(`${duration} ms`);
    if (summary.length > 0) lines.push("", `运行统计：${summary.join(" · ")}`);
  }
  if (cost && typeof cost === "object" && !Array.isArray(cost)) {
    const row = cost as Record<string, unknown>;
    const usd = Number(row.provider_cost_usd || 0);
    const credits = Number(row.provider_credits || 0);
    const values: string[] = [];
    if (usd > 0) values.push(`$${usd.toFixed(4)}`);
    if (credits > 0) values.push(`${credits} credits`);
    if (values.length > 0) lines.push(`Provider 成本：${values.join(" · ")}`);
  }
  return lines.join("\n").trim();
}

function researchStopReasonText(reason: string): string {
  const labels: Record<string, string> = {
    coverage_satisfied: "关键问题已覆盖",
    max_rounds: "达到研究轮次上限",
    max_search_calls: "达到搜索调用上限",
    max_provider_cost: "达到 Provider 成本上限",
    max_provider_credits: "达到 Provider credits 上限",
    no_new_queries: "没有新的有效查询",
    no_new_sources: "没有发现新的有效来源",
    low_information_gain: "新增信息已低于阈值",
    budget_satisfied: "研究预算已满足",
  };
  return labels[reason] || reason;
}

function resultSummary(item: AssistantTurnItem): string {
  const value = parseJSON(item.resultJson);
  if (isWebResearchTool(item.toolName) && value && typeof value === "object" && !Array.isArray(value)) {
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

<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="prompt-trace-page">
    <div class="page-header">
      <div>
        <h2>Prompt Trace</h2>
        <p>查看真实模型请求产生的提示词注入区块与回复长度记录。</p>
      </div>
      <el-button :loading="loading" @click="load">刷新</el-button>
    </div>

    <el-alert
      v-if="error"
      type="error"
      :closable="false"
      show-icon
      :title="error"
      class="trace-alert"
    />

    <el-empty
      v-if="!loading && !error && traces.length === 0"
      description="暂无 Prompt Trace；产生一次真实模型回复后会记录"
    />

    <div v-else class="trace-list" v-loading="loading">
      <el-card v-for="trace in traces" :key="trace.id" shadow="never" class="trace-card">
        <template #header>
          <div class="trace-card-header">
            <div class="trace-title-wrap">
              <strong class="trace-title">{{ trace.id || trace.promptHash || "未命名请求" }}</strong>
              <span class="trace-time">{{ trace.time || "—" }}</span>
            </div>
            <div class="trace-badges">
              <el-tag size="small" effect="plain">{{ trace.source }}</el-tag>
              <el-tag size="small" type="info" effect="plain">
                {{ trace.rawReplyLength }} → {{ trace.finalReplyLength }}
              </el-tag>
            </div>
          </div>
        </template>

        <div class="trace-summary">
          <span>Prompt Hash</span>
          <code>{{ trace.promptHash || "—" }}</code>
        </div>
        <div class="trace-summary">
          <span>注入区块</span>
          <span>{{ trace.sections.length ? trace.sections.join(" / ") : "无" }}</span>
        </div>

        <el-collapse class="trace-detail-collapse">
          <el-collapse-item title="完整 Trace 字段" name="detail">
            <pre class="trace-detail">{{ pretty(trace.raw) }}</pre>
          </el-collapse-item>
        </el-collapse>
      </el-card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useApi } from "@/composables/useApi";

type TraceEntry = {
  id: string;
  time: string;
  source: string;
  promptHash: string;
  sections: string[];
  rawReplyLength: number;
  finalReplyLength: number;
  raw: Record<string, unknown>;
};

const { get } = useApi();
const traces = ref<TraceEntry[]>([]);
const loading = ref(false);
const error = ref("");

function numberValue(value: unknown): number {
  const n = Number(value);
  return Number.isFinite(n) ? n : 0;
}

function normalizeTrace(raw: unknown): TraceEntry {
  const item = raw && typeof raw === "object" ? (raw as Record<string, unknown>) : {};
  const sections = Array.isArray(item.section_names)
    ? item.section_names.map((value) => String(value))
    : [];
  const requestId = String(item.request_id ?? "");
  const promptHash = String(item.prompt_hash ?? "");
  return {
    id: requestId || promptHash,
    time: String(item["@timestamp"] ?? item.timestamp ?? ""),
    source: String(item.source ?? item.channel ?? "chat"),
    promptHash,
    sections,
    rawReplyLength: numberValue(item.raw_reply_length),
    finalReplyLength: numberValue(item.final_reply_length),
    raw: item,
  };
}

function pretty(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value ?? "");
  }
}

async function load() {
  loading.value = true;
  error.value = "";
  try {
    const result = await get<{ traces?: unknown[] }>("/api/logs/prompt-traces");
    traces.value = Array.isArray(result?.traces) ? result.traces.map(normalizeTrace) : [];
  } catch (e: any) {
    traces.value = [];
    error.value = e?.message || "Prompt Trace 加载失败";
  } finally {
    loading.value = false;
  }
}

onMounted(load);
</script>

<style scoped>
.prompt-trace-page {
  min-height: 320px;
}
.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}
.page-header h2 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
}
.page-header p {
  margin: 5px 0 0;
  color: var(--ac-color-text-secondary);
  font-size: var(--ac-font-size-sm);
}
.trace-alert {
  margin-bottom: 14px;
}
.trace-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 180px;
}
.trace-card-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}
.trace-title-wrap {
  min-width: 0;
}
.trace-title {
  display: block;
  max-width: 680px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.trace-time {
  display: block;
  margin-top: 3px;
  color: var(--ac-color-text-muted);
  font-size: var(--ac-font-size-xs);
}
.trace-badges {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
  justify-content: flex-end;
}
.trace-summary {
  display: grid;
  grid-template-columns: 96px minmax(0, 1fr);
  gap: 10px;
  margin-bottom: 8px;
  font-size: var(--ac-font-size-sm);
}
.trace-summary > span:first-child {
  color: var(--ac-color-text-muted);
}
.trace-summary code {
  overflow-wrap: anywhere;
}
.trace-detail-collapse {
  margin-top: 8px;
}
.trace-detail {
  max-height: 360px;
  overflow: auto;
  margin: 0;
  padding: 12px;
  border-radius: var(--ac-radius-sm);
  background: var(--ac-color-surface-secondary);
  font: 12px/1.6 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
@media (max-width: 760px) {
  .trace-card-header,
  .page-header {
    flex-direction: column;
  }
  .trace-summary {
    grid-template-columns: 1fr;
  }
}
</style>

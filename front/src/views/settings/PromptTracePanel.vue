<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="prompt-trace-panel">
    <div class="panel-header">
      <div>
        <h3>Prompt Trace</h3>
        <p>查看真实模型请求的提示词装配区块、来源和回复长度变化。</p>
      </div>
      <el-button :loading="loading" @click="loadTraces">刷新</el-button>
    </div>

    <el-alert v-if="error" :title="error" type="error" show-icon :closable="false" />
    <el-empty v-if="!loading && !error && traces.length === 0" description="暂无 Prompt Trace；产生一次真实模型回复后会出现记录" />

    <div v-else class="trace-list" v-loading="loading">
      <el-card v-for="trace in traces" :key="traceKey(trace)" shadow="never" class="trace-card">
        <template #header>
          <div class="trace-header">
            <div class="trace-title">
              <strong>{{ trace.request_id || trace.prompt_hash || "未命名 Trace" }}</strong>
              <span>{{ trace["@timestamp"] || "" }}</span>
            </div>
            <div class="trace-tags">
              <el-tag size="small">{{ trace.source || trace.channel || "chat" }}</el-tag>
              <el-button link type="primary" @click="toggle(trace)">{{ isExpanded(trace) ? "收起" : "详情" }}</el-button>
            </div>
          </div>
        </template>

        <div class="trace-summary">
          <div><span>Prompt Hash</span><code>{{ trace.prompt_hash || "—" }}</code></div>
          <div><span>注入区块</span><span>{{ sectionNames(trace) }}</span></div>
          <div><span>回复长度</span><span>{{ numberValue(trace.raw_reply_length) }} → {{ numberValue(trace.final_reply_length) }}</span></div>
        </div>
        <pre v-if="isExpanded(trace)" class="trace-raw">{{ pretty(trace) }}</pre>
      </el-card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from "vue";
import { apiClient } from "@/composables/useApi";

type PromptTrace = Record<string, any>;

const loading = ref(false);
const error = ref("");
const traces = ref<PromptTrace[]>([]);
const expandedKey = ref("");

function traceKey(trace: PromptTrace): string {
  return String(trace.request_id || trace.prompt_hash || trace["@timestamp"] || JSON.stringify(trace));
}

function isExpanded(trace: PromptTrace): boolean {
  return expandedKey.value === traceKey(trace);
}

function toggle(trace: PromptTrace) {
  const key = traceKey(trace);
  expandedKey.value = expandedKey.value === key ? "" : key;
}

function numberValue(value: unknown): number {
  const n = Number(value ?? 0);
  return Number.isFinite(n) ? n : 0;
}

function sectionNames(trace: PromptTrace): string {
  const sections = Array.isArray(trace.section_names) ? trace.section_names.map(String).filter(Boolean) : [];
  return sections.length ? sections.join(" / ") : "无";
}

function pretty(trace: PromptTrace): string {
  return JSON.stringify(trace, null, 2);
}

async function loadTraces() {
  loading.value = true;
  error.value = "";
  try {
    const response = await apiClient.get("/api/logs/prompt-traces");
    const payload = response.data?.data ?? response.data ?? {};
    traces.value = Array.isArray(payload.traces) ? payload.traces : [];
  } catch (err: any) {
    traces.value = [];
    error.value = err?.response?.data?.message || err?.message || "Prompt Trace 加载失败";
  } finally {
    loading.value = false;
  }
}

onMounted(loadTraces);
</script>

<style scoped>
.prompt-trace-panel { display: flex; flex-direction: column; gap: 16px; }
.panel-header, .trace-header, .trace-tags { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.panel-header h3 { margin: 0 0 4px; font-size: 18px; }
.panel-header p { margin: 0; color: var(--el-text-color-secondary); font-size: 13px; }
.trace-list { display: flex; flex-direction: column; gap: 12px; min-height: 120px; }
.trace-title { min-width: 0; display: flex; flex-direction: column; gap: 4px; }
.trace-title strong { max-width: 680px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.trace-title span { color: var(--el-text-color-secondary); font-size: 12px; }
.trace-summary { display: grid; gap: 8px; }
.trace-summary > div { display: grid; grid-template-columns: 110px minmax(0, 1fr); gap: 12px; font-size: 13px; }
.trace-summary > div > span:first-child { color: var(--el-text-color-secondary); }
code { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.trace-raw { margin: 14px 0 0; padding: 14px; max-height: 440px; overflow: auto; border-radius: var(--el-border-radius-base); background: var(--el-fill-color-light); white-space: pre-wrap; word-break: break-word; font-size: 12px; line-height: 1.55; }
@media (max-width: 760px) { .panel-header, .trace-header { align-items: flex-start; flex-direction: column; } .trace-summary > div { grid-template-columns: 1fr; gap: 4px; } }
</style>

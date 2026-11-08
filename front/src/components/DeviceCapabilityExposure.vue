<template>
  <el-card shadow="never" class="section-card">
    <template #header>
      <div class="capability-head">
        <span>设备能力开放</span>
        <el-tag size="small" effect="plain">仅本机可管理</el-tag>
      </div>
    </template>
    <p class="capability-tip">统筹中心只能看到这里保持开启的能力。关闭后，中心会在当前调用中断后移除该能力。</p>
    <el-alert v-if="error" :title="error" type="error" :closable="false" show-icon />
    <el-alert v-if="syncNotice" :title="syncNotice" type="warning" :closable="false" show-icon />
    <div v-loading="loading" class="capability-list">
      <article v-for="item in capabilities" :key="item.id" class="capability-row">
        <div>
          <strong>{{ item.label }}</strong>
          <p>{{ item.description }}</p>
          <code>{{ item.id }}</code>
        </div>
        <el-switch
          :model-value="item.enabled"
          :loading="saving === item.id"
          :disabled="Boolean(saving)"
          @change="(value: string | number | boolean) => update(item, value === true)"
        />
      </article>
      <el-empty v-if="!loading && !capabilities.length" description="没有可配置的设备能力" />
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import { useApi } from "@/composables/useApi";
import { createAuthenticatedFetchInit } from "@/runtime/request-auth";
import { resolveApiUrl } from "@/runtime/runtime-adapter";
import { consumeSSEBody } from "@/runtime/sse-stream";

type Capability = { id: string; label: string; description: string; enabled: boolean };
type ExposureState = { revision: number; capabilities: Capability[]; enabledCapabilities: string[] };
type UpdateResult = { state: ExposureState; centerSynced: boolean; syncError?: string };

const api = useApi();
const capabilities = ref<Capability[]>([]);
const revision = ref(0);
const loading = ref(false);
const saving = ref("");
const error = ref("");
const syncNotice = ref("");
let disposed = false;
let streamAbort: AbortController | undefined;
let reconnectTimer: ReturnType<typeof setTimeout> | undefined;
let reconnectAttempt = 0;

function applyState(state: ExposureState) {
  revision.value = state.revision;
  capabilities.value = Array.isArray(state.capabilities) ? state.capabilities : [];
}

async function load() {
  loading.value = true;
  error.value = "";
  try {
    applyState(await api.get<ExposureState>("/internal/device-mesh/capabilities"));
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "设备能力开放状态读取失败";
  } finally {
    loading.value = false;
  }
}

async function update(item: Capability, enabled: boolean) {
  if (saving.value) return;
  saving.value = item.id;
  error.value = "";
  syncNotice.value = "";
  try {
    const result = await api.put<UpdateResult>(`/internal/device-mesh/capabilities/${encodeURIComponent(item.id)}`, {
      enabled,
      expectedRevision: revision.value,
    });
    applyState(result.state);
    if (result.syncError) error.value = result.syncError;
    else if (!result.centerSynced) syncNotice.value = "已保存到本机。设备下次连接统筹中心时会同步这项设置。";
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : "设备能力开放状态保存失败";
    await load();
  } finally {
    saving.value = "";
  }
}

function stopStream() {
  streamAbort?.abort();
  streamAbort = undefined;
  if (reconnectTimer) clearTimeout(reconnectTimer);
  reconnectTimer = undefined;
}

function scheduleReconnect() {
  if (disposed || reconnectTimer) return;
  const delay = Math.min(30_000, 1000 * 2 ** reconnectAttempt++);
  reconnectTimer = setTimeout(() => {
    reconnectTimer = undefined;
    void connectStream();
  }, delay);
}

async function connectStream() {
  if (disposed || streamAbort) return;
  const controller = new AbortController();
  streamAbort = controller;
  try {
    const path = "/internal/device-mesh/capabilities/events";
    const response = await fetch(await resolveApiUrl(path), await createAuthenticatedFetchInit(path, {
      headers: { Accept: "text/event-stream", "Cache-Control": "no-store" },
      signal: controller.signal,
    }));
    if (!response.ok || !response.body || !response.headers.get("Content-Type")?.includes("text/event-stream")) {
      throw new Error(await response.text() || `设备能力事件流连接失败: HTTP ${response.status}`);
    }
    reconnectAttempt = 0;
    await consumeSSEBody(response.body, controller.signal, (event) => {
      if (event.event !== "capability_exposure") return;
      applyState(JSON.parse(event.data) as ExposureState);
    });
    if (!controller.signal.aborted) scheduleReconnect();
  } catch (cause) {
    if (!controller.signal.aborted) console.error("[original-error]", cause);
    scheduleReconnect();
  } finally {
    if (streamAbort === controller) streamAbort = undefined;
  }
}

onMounted(async () => {
  await load();
  void connectStream();
});
onUnmounted(() => {
  disposed = true;
  stopStream();
});
</script>

<style scoped>
.capability-head, .capability-row { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
.capability-tip { margin: 0 0 14px; color: var(--text-secondary); font-size: 13px; line-height: 1.6; }
.capability-list { display: grid; gap: 2px; }
.capability-row { padding: 14px 0; border-top: 1px solid var(--surface-border); }
.capability-row:first-child { border-top: 0; }
.capability-row > div { min-width: 0; }
.capability-row strong { font-size: 14px; }
.capability-row p { margin: 4px 0; color: var(--text-secondary); font-size: 12px; line-height: 1.5; }
.capability-row code { color: var(--text-tertiary); font-size: 11px; overflow-wrap: anywhere; }
</style>

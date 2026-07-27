<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from "vue";

interface SessionStats {
  total: number;
  active: number;
  suspended: number;
  closed: number;
  failed: number;
  quarantined: number;
}

const stats = ref<SessionStats>({ total: 0, active: 0, suspended: 0, closed: 0, failed: 0, quarantined: 0 });
const loading = ref(false);
const error = ref<string | null>(null);
let timer: number | null = null;

async function fetchStats() {
  loading.value = true;
  error.value = null;
  try {
    const res = await fetch("/api/extension/webui/stats");
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    stats.value = await res.json();
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  fetchStats();
  timer = window.setInterval(fetchStats, 30000);
});

onBeforeUnmount(() => {
  if (timer) clearInterval(timer);
});
</script>

<template>
  <div class="webui-manager">
    <div class="webui-manager__header">
      <h3 class="webui-manager__title">WebUI 沙箱会话</h3>
      <button class="webui-manager__refresh" :disabled="loading" @click="fetchStats">
        {{ loading ? "刷新中..." : "刷新" }}
      </button>
    </div>
    <div v-if="error" class="webui-manager__error">{{ error }}</div>
    <div class="webui-manager__stats">
      <div class="webui-manager__stat">
        <div class="webui-manager__stat-value">{{ stats.total }}</div>
        <div class="webui-manager__stat-label">总会话</div>
      </div>
      <div class="webui-manager__stat webui-manager__stat--active">
        <div class="webui-manager__stat-value">{{ stats.active }}</div>
        <div class="webui-manager__stat-label">活跃</div>
      </div>
      <div class="webui-manager__stat webui-manager__stat--suspended">
        <div class="webui-manager__stat-value">{{ stats.suspended }}</div>
        <div class="webui-manager__stat-label">挂起</div>
      </div>
      <div class="webui-manager__stat webui-manager__stat--failed">
        <div class="webui-manager__stat-value">{{ stats.failed }}</div>
        <div class="webui-manager__stat-label">失败</div>
      </div>
      <div class="webui-manager__stat webui-manager__stat--quarantined">
        <div class="webui-manager__stat-value">{{ stats.quarantined }}</div>
        <div class="webui-manager__stat-label">隔离</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.webui-manager { background: white; border-radius: 8px; padding: 16px; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
.webui-manager__header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px; }
.webui-manager__title { font-size: 14px; font-weight: 600; margin: 0; }
.webui-manager__refresh { padding: 4px 12px; font-size: 12px; border: 1px solid #ddd; border-radius: 4px; background: white; cursor: pointer; }
.webui-manager__refresh:hover:not(:disabled) { background: #f5f5f5; }
.webui-manager__refresh:disabled { opacity: 0.5; cursor: not-allowed; }
.webui-manager__error { padding: 8px 12px; margin-bottom: 12px; background: #fef2f2; border: 1px solid #fca5a5; border-radius: 4px; font-size: 12px; color: #b42828; }
.webui-manager__stats { display: grid; grid-template-columns: repeat(5, 1fr); gap: 8px; }
.webui-manager__stat { text-align: center; padding: 12px 8px; background: #f9fafb; border-radius: 6px; }
.webui-manager__stat-value { font-size: 24px; font-weight: 700; color: #333; }
.webui-manager__stat-label { font-size: 11px; color: #888; margin-top: 4px; }
.webui-manager__stat--active .webui-manager__stat-value { color: #059669; }
.webui-manager__stat--suspended .webui-manager__stat-value { color: #d97706; }
.webui-manager__stat--failed .webui-manager__stat-value { color: #dc2626; }
.webui-manager__stat--quarantined .webui-manager__stat-value { color: #7c3aed; }
</style>

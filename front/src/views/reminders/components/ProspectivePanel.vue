<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <section class="schedule-panel" aria-label="待执行任务">
    <header class="panel-header">
      <div>
        <h2>接下来的安排</h2>
        <p>由所属核心按约定时间执行，发送结果会保存在执行历史中。</p>
      </div>
      <el-button @click="$emit('refresh')" :loading="loading">刷新</el-button>
    </header>
    <div v-if="queueSummary" class="queue-summary">
      <div>
        <span>队列深度</span><strong>{{ queueSummary.depth }}</strong>
      </div>
      <div>
        <span>待触发</span><strong>{{ queueSummary.pendingCount }}</strong>
      </div>
      <div>
        <span>最近失败</span><strong>{{ queueSummary.recentFailures }}</strong>
      </div>
      <el-tag :type="queueSummary.backpressure ? 'danger' : 'success'" round>{{
        queueSummary.backpressure ? "队列受限" : "队列正常"
      }}</el-tag>
    </div>
    <el-alert
      v-if="queueSummary?.backpressure"
      type="warning"
      :closable="false"
      title="部分任务受到熔断限制，请检查执行历史中的错误后重试。"
      show-icon
      ><el-button type="warning" plain @click="$emit('clearBackpressure')"
        >清除背压标记</el-button
      ></el-alert
    >
    <div v-loading="loading" class="schedule-timeline">
      <article v-for="item in items" :key="item.id" class="timeline-item">
        <div class="timeline-marker">
          <el-icon><Clock /></el-icon>
        </div>
        <div class="timeline-content">
          <div class="item-heading">
            <h3>{{ item.title }}</h3>
            <el-tag :type="statusType(item.state)" round>{{ statusLabel(item.state) }}</el-tag>
          </div>
          <p v-if="item.content" class="item-content">{{ item.content }}</p>
          <div class="timeline-meta">
            <span v-if="item.remindAt">{{ fmtDate(item.remindAt) }}</span
            ><span>{{ item.timezone }}</span>
          </div>
          <details class="owner-details">
            <summary>执行归属</summary>
            <p>{{ item.ownerId }}</p>
          </details>
        </div>
      </article>
      <el-empty v-if="!items.length && !loading" description="当前没有待执行任务" :image-size="72"
        ><span class="empty-hint">启用任务后，会在这里看到接下来的安排</span></el-empty
      >
    </div>
  </section>
</template>
<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import { Clock } from "@element-plus/icons-vue";
import { request } from "../../../composables/request";
import type { TriggerQueueSummary } from "../../../types";
import { ElMessage } from "element-plus";

defineEmits<{
  refresh: [];
  clearBackpressure: [];
}>();

const props = withDefaults(defineProps<{ kind?: "user_reminder" | "proactive_task" }>(), {
  kind: "user_reminder",
});

const items = ref<
  Array<{
    id: string;
    title: string;
    content: string;
    state: string;
    remindAt: string;
    timezone: string;
    ownerId: string;
  }>
>([]);
const queueSummary = ref<TriggerQueueSummary | null>(null);
const loading = ref(false);

async function fetchItems() {
  loading.value = true;
  try {
    const res: any = await request.get("/api/reminders/prospective", {
      params: { kind: props.kind },
    });
    if (!Array.isArray(res)) throw new Error("待执行任务响应无效");
    items.value = res;
  } catch (error: any) {
    items.value = [];
    ElMessage.error(error?.message || "待执行任务加载失败");
  } finally {
    loading.value = false;
  }
}

async function fetchQueueSummary() {
  try {
    const res: any = await request.get("/api/reminders/queue-summary", {
      params: { kind: props.kind },
    });
    if (!res || typeof res.backpressure !== "boolean" || !Number.isFinite(res.depth))
      throw new Error("队列状态响应无效");
    queueSummary.value = res;
  } catch (error: any) {
    queueSummary.value = null;
    ElMessage.error(error?.message || "队列状态加载失败");
  }
}

function statusType(s: string) {
  const map: Record<string, string> = {
    enabled: "success",
    paused: "warning",
    disabled: "info",
    active: "warning",
    triggered: "primary",
    completed: "success",
    cancelled: "info",
  };
  return map[s] || "info";
}

function statusLabel(s: string) {
  const map: Record<string, string> = {
    enabled: "待执行",
    paused: "已暂停",
    disabled: "已停用",
    expired: "已结束",
    active: "待触发",
    triggered: "已触发",
    completed: "已完成",
    cancelled: "已取消",
  };
  return map[s] || s;
}

function fmtDate(d: string) {
  if (!d) return "";
  try {
    return new Date(d).toLocaleString("zh-CN");
  } catch {
    return d;
  }
}

let refreshTimer: ReturnType<typeof setInterval> | undefined;
async function refresh() {
  await Promise.all([fetchItems(), fetchQueueSummary()]);
}
defineExpose({ refresh });
onMounted(() => {
  fetchItems();
  fetchQueueSummary();
  refreshTimer = setInterval(() => {
    if (!loading.value) void refresh();
  }, 5000);
});
onUnmounted(() => clearInterval(refreshTimer));
</script>

<style scoped>
.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  margin-bottom: 24px;
}
h2 {
  font-size: 18px;
  margin: 0 0 8px;
  color: var(--tp-text);
}
.panel-header p,
.empty-hint {
  color: var(--tp-text-secondary);
  font-size: 13px;
  line-height: 1.6;
  margin: 0;
}
.queue-summary {
  display: flex;
  gap: 32px;
  align-items: center;
  background: var(--tp-panel-soft);
  border-radius: 12px;
  padding: 18px 20px;
  margin-bottom: 24px;
  flex-wrap: wrap;
}
.queue-summary > div {
  display: flex;
  align-items: center;
  gap: 12px;
}
.queue-summary span {
  color: var(--tp-text-secondary);
  font-size: 13px;
}
.queue-summary strong {
  font-size: 20px;
  font-variant-numeric: tabular-nums;
  color: var(--tp-text);
}
.queue-summary > .el-tag {
  margin-left: auto;
}
.schedule-timeline {
  min-height: 160px;
}
.timeline-item {
  display: flex;
  gap: 18px;
  padding: 0 0 24px;
}
.timeline-marker {
  flex-shrink: 0;
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  border-radius: 50%;
  color: var(--tp-primary);
  background: var(--tp-primary-soft);
}
.timeline-content {
  min-width: 0;
  flex: 1;
  border-bottom: 1px solid var(--tp-border-light);
  padding-bottom: 22px;
}
.item-heading {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
}
h3 {
  font-size: 15px;
  line-height: 1.6;
  margin: 4px 0;
  overflow-wrap: anywhere;
}
.item-content {
  margin: 8px 0;
  color: var(--tp-text-secondary);
  font-size: 14px;
  line-height: 1.7;
  white-space: pre-wrap;
}
.timeline-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  color: var(--tp-text-secondary);
  font-size: 13px;
  margin-top: 10px;
}
.owner-details {
  font-size: 12px;
  color: var(--tp-text-secondary);
  margin-top: 12px;
}
.owner-details summary {
  cursor: pointer;
  width: fit-content;
  padding: 4px 0;
}
.owner-details p {
  overflow-wrap: anywhere;
}
.owner-details summary:focus-visible {
  outline: 2px solid var(--tp-primary);
  outline-offset: 3px;
}
@media (max-width: 760px) {
  .queue-summary {
    gap: 16px;
  }
  .queue-summary > .el-tag {
    margin-left: 0;
  }
  .panel-header {
    align-items: flex-start;
  }
}
</style>

<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <section class="schedule-panel" aria-label="待执行任务">
    <el-alert
      v-if="queueSummary?.backpressure"
      type="warning"
      :closable="false"
      title="部分任务暂时受限"
      show-icon
      class="queue-alert"
      ><el-button link type="warning" @click="$emit('clearBackpressure')"
        >清除背压标记</el-button
      ></el-alert
    >
    <div v-else-if="queueSummary?.recentFailures" class="queue-note">
      <el-tag type="warning" size="small">近 24 小时 {{ queueSummary.recentFailures }} 次失败</el-tag>
      <span>详情见执行历史</span>
    </div>
    <div v-loading="loading" class="schedule-list">
      <article v-for="item in items" :key="item.id" class="timeline-item">
        <div class="schedule-time">
          <span>{{ fmtDay(item.remindAt) }}</span>
          <strong>{{ fmtTime(item.remindAt) }}</strong>
        </div>
        <div class="timeline-content">
          <div class="item-heading">
            <h3>{{ item.title }}</h3>
            <el-tag :type="statusType(item.state)" round>{{ statusLabel(item.state) }}</el-tag>
          </div>
          <p v-if="item.content" class="item-content">{{ item.content }}</p>
        </div>
      </article>
      <el-empty
        v-if="!items.length && !loading"
        description="当前没有待执行任务"
        :image-size="64"
      />
    </div>
  </section>
</template>
<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import { request } from "../../../composables/request";
import type { TriggerQueueSummary } from "../../../types";
import { ElMessage } from "element-plus";

defineEmits<{
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

function fmtDay(d: string) {
  if (!d) return "";
  try {
    return new Intl.DateTimeFormat("zh-CN", {
      month: "numeric",
      day: "numeric",
      weekday: "short",
    }).format(new Date(d));
  } catch (error) {
  console.error("[original-error]", error);
return d;
  }
}

function fmtTime(d: string) {
  if (!d) return "";
  try {
    return new Intl.DateTimeFormat("zh-CN", {
      hour: "2-digit",
      minute: "2-digit",
      hour12: false,
    }).format(new Date(d));
  } catch (error) {
  console.error("[original-error]", error);
return "";
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
.queue-alert {
  margin-bottom: 12px;
}
.queue-note {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 30px;
  margin-bottom: 8px;
  color: var(--tp-text-secondary);
  font-size: 12px;
}
.schedule-list {
  min-height: 120px;
}
.timeline-item {
  display: grid;
  grid-template-columns: 92px minmax(0, 1fr);
  gap: 16px;
  align-items: start;
  padding: 12px 4px;
  border-bottom: 1px solid var(--tp-border-light);
}
.schedule-time {
  display: grid;
  gap: 1px;
  color: var(--tp-text-secondary);
  font-size: 12px;
  line-height: 1.5;
}
.schedule-time strong {
  color: var(--tp-text);
  font-size: 15px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}
.timeline-content {
  min-width: 0;
}
.item-heading {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
}
h3 {
  font-size: 15px;
  line-height: 1.5;
  margin: 0;
  overflow-wrap: anywhere;
}
.item-content {
  margin: 3px 0 0;
  color: var(--tp-text-secondary);
  font-size: 13px;
  line-height: 1.5;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
@media (max-width: 760px) {
  .timeline-item {
    grid-template-columns: 74px minmax(0, 1fr);
    gap: 12px;
  }
}
</style>

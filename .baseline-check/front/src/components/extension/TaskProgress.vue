<template>
  <div class="task-progress" :class="{ compact }">
    <el-progress
      :percentage="displayPercentage"
      :status="progressStatus"
      :stroke-width="compact ? 8 : 12"
      :show-text="!compact"
      :striped="isActive"
      :striped-flow="isActive"
      :duration="isActive ? 6 : undefined"
    />
    <div v-if="!compact" class="progress-meta">
      <div class="meta-row">
        <span class="meta-label">阶段</span>
        <span class="meta-value">{{ progress?.stage || "—" }}</span>
      </div>
      <div class="meta-row">
        <span class="meta-label">消息</span>
        <span class="meta-value message" :title="progress?.message || ''">{{
          progress?.message || "—"
        }}</span>
      </div>
      <div class="meta-row">
        <span class="meta-label">进度</span>
        <span class="meta-value">
          {{ currentText }} / {{ totalText }}
          <span class="percentage">（{{ displayPercentage }}%）</span>
        </span>
      </div>
      <div v-if="progress?.updatedAt" class="meta-row">
        <span class="meta-label">更新时间</span>
        <span class="meta-value">{{ formatTime(progress.updatedAt) }}</span>
      </div>
    </div>
    <div v-else class="compact-text">{{ displayPercentage }}%</div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import type { TaskProgress, TaskRunStatus } from "@/views/extensions/types";

const props = withDefaults(
  defineProps<{
    progress?: TaskProgress | null;
    status?: TaskRunStatus | "";
    compact?: boolean;
  }>(),
  { progress: null, status: "", compact: false },
);

const activeStatuses: TaskRunStatus[] = [
  "starting",
  "running",
  "checkpointing",
  "pausing",
  "resuming",
  "cancelling",
];

const isActive = computed(() =>
  props.status ? activeStatuses.includes(props.status as TaskRunStatus) : false,
);

const displayPercentage = computed(() => {
  const value = Number(props.progress?.percentage);
  if (!Number.isFinite(value)) return 0;
  const clamped = Math.max(0, Math.min(100, value));
  return Math.round(clamped);
});

const progressStatus = computed<
  "" | "success" | "warning" | "exception"
>(() => {
  const status = props.status as TaskRunStatus;
  if (status === "succeeded") return "success";
  if (
    status === "failed" ||
    status === "timed_out" ||
    status === "cancelled" ||
    status === "manual_intervention"
  )
    return "exception";
  if (status === "recovery_required" || status === "paused") return "warning";
  return "";
});

const currentText = computed(() =>
  props.progress && Number.isFinite(Number(props.progress.current))
    ? props.progress.current
    : "—",
);

const totalText = computed(() =>
  props.progress && Number.isFinite(Number(props.progress.total))
    ? props.progress.total
    : "—",
);

function formatTime(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime())
    ? value
    : date.toLocaleString("zh-CN", { hour12: false });
}
</script>

<style scoped>
.task-progress {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-width: 0;
}
.task-progress.compact {
  gap: 4px;
}
.progress-meta {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 13px;
}
.meta-row {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  min-width: 0;
}
.meta-label {
  flex: 0 0 64px;
  color: var(--ac-color-text-muted);
}
.meta-value {
  flex: 1 1 auto;
  min-width: 0;
  color: var(--ac-color-text);
  overflow-wrap: anywhere;
}
.meta-value.message {
  white-space: pre-wrap;
}
.percentage {
  color: var(--ac-color-text-muted);
}
.compact-text {
  font-size: 12px;
  color: var(--ac-color-text-muted);
}
</style>

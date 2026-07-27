<template>
  <el-drawer
    :model-value="modelValue"
    :title="drawerTitle"
    size="min(720px, 94vw)"
    :before-close="handleClose"
    @open="onOpen"
    @closed="onClosed"
  >
    <div v-loading="loading" class="task-detail">
      <el-alert
        v-if="loadError"
        :title="loadError"
        type="error"
        show-icon
        :closable="false"
      >
        <template #default>
          <el-button link type="primary" @click="reload">重新加载</el-button>
        </template>
      </el-alert>

      <template v-if="task">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="TaskRunID" :span="2">
            <code>{{ task.taskRunId }}</code>
          </el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="statusType(task.status)" size="small">
              {{ statusLabel(task.status) }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="优先级">
            <el-tag :type="priorityType(task.priority)" size="small" effect="plain">
              P{{ task.priority }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="ExtensionID" :span="2">
            <code>{{ task.extensionId }}</code>
          </el-descriptions-item>
          <el-descriptions-item v-if="task.moduleId" label="模块">
            <code>{{ task.moduleId }}</code>
          </el-descriptions-item>
          <el-descriptions-item v-if="task.taskDefinitionId" label="任务定义">
            <code>{{ task.taskDefinitionId }}</code>
          </el-descriptions-item>
          <el-descriptions-item v-if="task.operationId" label="OperationID">
            <code>{{ task.operationId }}</code>
          </el-descriptions-item>
          <el-descriptions-item v-if="task.invocationId" label="InvocationID">
            <code>{{ task.invocationId }}</code>
          </el-descriptions-item>
          <el-descriptions-item label="尝试次数">
            {{ task.attempt }} / {{ task.maxAttempts || "—" }}
          </el-descriptions-item>
          <el-descriptions-item v-if="task.inputHash" label="输入哈希">
            <code :title="task.inputHash">{{ shortHash(task.inputHash) }}</code>
          </el-descriptions-item>
          <el-descriptions-item label="创建时间">
            {{ formatTime(task.createdAt) }}
          </el-descriptions-item>
          <el-descriptions-item label="入队时间">
            {{ formatTime(task.queuedAt) }}
          </el-descriptions-item>
          <el-descriptions-item label="开始时间">
            {{ formatTime(task.startedAt) }}
          </el-descriptions-item>
          <el-descriptions-item label="结束时间">
            {{ formatTime(task.finishedAt) }}
          </el-descriptions-item>
          <el-descriptions-item v-if="task.deadlineAt" label="截止时间" :span="2">
            {{ formatTime(task.deadlineAt) }}
          </el-descriptions-item>
        </el-descriptions>

        <el-card v-if="task.errorMessage" shadow="never" class="section-card">
          <template #header>
            <span class="card-title danger-title">错误信息</span>
          </template>
          <el-descriptions :column="1" border>
            <el-descriptions-item v-if="task.errorCode" label="错误码">
              <code>{{ task.errorCode }}</code>
            </el-descriptions-item>
            <el-descriptions-item label="错误详情">
              <pre class="error-text">{{ task.errorMessage }}</pre>
            </el-descriptions-item>
          </el-descriptions>
        </el-card>

        <el-card shadow="never" class="section-card">
          <template #header>
            <div class="card-head">
              <span class="card-title">当前进度</span>
              <el-tag
                v-if="polling"
                type="warning"
                size="small"
                effect="plain"
              >
                实时更新中
              </el-tag>
            </div>
          </template>
          <TaskProgressBar
            :progress="latestProgress || task.progress || null"
            :status="task.status"
          />
        </el-card>

        <el-card shadow="never" class="section-card">
          <template #header>
            <span class="card-title">检查点状态</span>
          </template>
          <template v-if="task.checkpoint">
            <el-descriptions :column="1" border>
              <el-descriptions-item label="CheckpointID">
                <code>{{ task.checkpoint.checkpointId }}</code>
              </el-descriptions-item>
              <el-descriptions-item label="序号">
                {{ task.checkpoint.sequence }}
              </el-descriptions-item>
              <el-descriptions-item label="状态">
                <el-tag :type="checkpointType(task.checkpoint.status)" size="small">
                  {{ task.checkpoint.status || "—" }}
                </el-tag>
              </el-descriptions-item>
              <el-descriptions-item v-if="task.checkpoint.stage" label="阶段">
                {{ task.checkpoint.stage }}
              </el-descriptions-item>
              <el-descriptions-item v-if="task.checkpoint.message" label="消息">
                {{ task.checkpoint.message }}
              </el-descriptions-item>
              <el-descriptions-item label="创建时间">
                {{ formatTime(task.checkpoint.createdAt) }}
              </el-descriptions-item>
              <el-descriptions-item v-if="task.checkpoint.payload" label="负载">
                <pre class="json-text">{{ pretty(task.checkpoint.payload) }}</pre>
              </el-descriptions-item>
            </el-descriptions>
          </template>
          <el-empty v-else description="暂无检查点" :image-size="60" />
        </el-card>

        <el-card shadow="never" class="section-card">
          <template #header>
            <span class="card-title">Progress 时间线</span>
          </template>
          <el-empty
            v-if="!timeline.length"
            description="暂无进度记录"
            :image-size="60"
          />
          <el-timeline v-else>
            <el-timeline-item
              v-for="(item, index) in timeline"
              :key="`${item.sequence}-${index}`"
              :timestamp="formatTime(item.updatedAt)"
              :type="timelineNodeType(item)"
              placement="top"
            >
              <div class="timeline-stage">
                <span class="stage-tag">{{ item.stage || "未命名阶段" }}</span>
                <span class="stage-percent">{{ formatPercent(item.percentage) }}%</span>
              </div>
              <div class="timeline-meta">
                <span>{{ item.current }} / {{ item.total }}</span>
                <span v-if="item.message" class="timeline-message">{{ item.message }}</span>
              </div>
            </el-timeline-item>
          </el-timeline>
        </el-card>

        <el-card shadow="never" class="section-card">
          <template #header>
            <span class="card-title">结果 Artifact</span>
          </template>
          <TaskResultArtifact
            :result="result || task.result || null"
            :task-run-id="task.taskRunId"
          />
        </el-card>
      </template>
    </div>

    <template #footer>
      <div class="drawer-footer">
        <el-button @click="handleClose">关闭</el-button>
        <el-button
          v-if="canCancel"
          type="danger"
          plain
          :loading="acting"
          @click="onCancel"
        >
          取消任务
        </el-button>
        <el-button
          v-if="canRecover"
          type="warning"
          plain
          :loading="acting"
          @click="onRecover"
        >
          恢复任务
        </el-button>
        <el-button
          v-if="canRetry"
          type="primary"
          plain
          :loading="acting"
          @click="onRetry"
        >
          重试任务
        </el-button>
      </div>
    </template>
  </el-drawer>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import TaskProgressBar from "@/components/extension/TaskProgress.vue";
import TaskResultArtifact from "@/components/extension/TaskResultArtifact.vue";
import {
  cancelTask,
  fetchTask,
  fetchTaskProgress,
  fetchTaskResult,
  recoverTask,
  retryTask,
} from "@/views/extensions/api";
import type {
  TaskProgress,
  TaskResult,
  TaskRun,
  TaskRunStatus,
} from "@/views/extensions/types";

const props = defineProps<{
  modelValue: boolean;
  taskRunId: string;
}>();

const emit = defineEmits<{
  (e: "update:modelValue", value: boolean): void;
  (e: "refresh"): void;
  (e: "retried", taskRunId: string): void;
}>();

const loading = ref(false);
const acting = ref(false);
const loadError = ref("");
const task = ref<TaskRun | null>(null);
const result = ref<TaskResult | null>(null);
const timeline = ref<TaskProgress[]>([]);
const latestProgress = ref<TaskProgress | null>(null);
const polling = ref(false);
let pollTimer: ReturnType<typeof setInterval> | null = null;

const drawerTitle = computed(() => {
  if (!props.taskRunId) return "任务详情";
  return task.value
    ? `任务详情 · ${shortId(task.value.taskRunId)}`
    : "任务详情";
});

const activeStatuses: TaskRunStatus[] = [
  "starting",
  "running",
  "checkpointing",
  "pausing",
  "resuming",
  "cancelling",
];

const canCancel = computed(() =>
  task.value
    ? ![...activeStatuses, "cancelled", "succeeded", "failed", "timed_out"].includes(
        task.value.status,
      ) && task.value.status !== "manual_intervention"
    : false,
);

const canRecover = computed(
  () =>
    task.value &&
    (task.value.status === "recovery_required" ||
      task.value.status === "paused" ||
      task.value.status === "manual_intervention"),
);

const canRetry = computed(() =>
  task.value
    ? ["failed", "timed_out", "cancelled"].includes(task.value.status)
    : false,
);

watch(
  () => props.modelValue,
  (visible) => {
    if (visible) {
      resetState();
      reload();
    } else {
      stopPolling();
    }
  },
);

watch(
  () => props.taskRunId,
  (id, oldId) => {
    if (props.modelValue && id && id !== oldId) {
      resetState();
      reload();
    }
  },
);

function onOpen() {
  if (props.taskRunId) reload();
}

function onClosed() {
  stopPolling();
  resetState();
}

function handleClose() {
  emit("update:modelValue", false);
}

function resetState() {
  task.value = null;
  result.value = null;
  timeline.value = [];
  latestProgress.value = null;
  loadError.value = "";
}

async function reload() {
  if (!props.taskRunId) return;
  loading.value = true;
  loadError.value = "";
  try {
    task.value = await fetchTask(props.taskRunId);
    if (task.value?.progress) {
      latestProgress.value = task.value.progress;
      pushTimeline(task.value.progress);
    }
    loadNonBlocking();
    maybeStartPolling();
  } catch (error: any) {
    loadError.value = problem(error, "任务详情加载失败");
  } finally {
    loading.value = false;
  }
}

async function loadNonBlocking() {
  try {
    result.value = await fetchTaskResult(props.taskRunId);
  } catch {
    result.value = null;
  }
}

function maybeStartPolling() {
  if (!task.value) return;
  if (activeStatuses.includes(task.value.status)) {
    startPolling();
  } else {
    stopPolling();
  }
}

function startPolling() {
  stopPolling();
  polling.value = true;
  pollTimer = setInterval(pollProgress, 3000);
}

function stopPolling() {
  polling.value = false;
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
}

async function pollProgress() {
  if (!props.taskRunId) return;
  try {
    const progress = await fetchTaskProgress(props.taskRunId);
    latestProgress.value = progress;
    pushTimeline(progress);
    const status = task.value?.status;
    if (status && !activeStatuses.includes(status)) {
      stopPolling();
      task.value = await fetchTask(props.taskRunId);
    }
  } catch {
    stopPolling();
  }
}

function pushTimeline(progress: TaskProgress) {
  if (!progress || !Number.isFinite(progress.sequence)) return;
  const exists = timeline.value.some(
    (item) => item.sequence === progress.sequence,
  );
  if (exists) return;
  timeline.value.push(progress);
  if (timeline.value.length > 50) {
    timeline.value = timeline.value.slice(timeline.value.length - 50);
  }
}

async function onCancel() {
  if (!task.value) return;
  try {
    await ElMessageBox.confirm(
      "确认取消该任务？取消后任务将停止执行。",
      "取消任务",
      { type: "warning", confirmButtonText: "取消任务", cancelButtonText: "保留" },
    );
  } catch {
    return;
  }
  acting.value = true;
  try {
    await cancelTask(task.value.taskRunId);
    ElMessage.success("已请求取消任务");
    await reload();
    emit("refresh");
  } catch (error: any) {
    ElMessage.error(problem(error, "取消任务失败"));
  } finally {
    acting.value = false;
  }
}

async function onRetry() {
  if (!task.value) return;
  acting.value = true;
  try {
    const newTask = await retryTask(task.value.taskRunId);
    ElMessage.success("已重新提交任务");
    emit("retried", newTask?.taskRunId || task.value.taskRunId);
    emit("refresh");
    handleClose();
  } catch (error: any) {
    ElMessage.error(problem(error, "重试任务失败"));
  } finally {
    acting.value = false;
  }
}

async function onRecover() {
  if (!task.value) return;
  acting.value = true;
  try {
    await recoverTask(task.value.taskRunId);
    ElMessage.success("已请求恢复任务");
    await reload();
    emit("refresh");
  } catch (error: any) {
    ElMessage.error(problem(error, "恢复任务失败"));
  } finally {
    acting.value = false;
  }
}

function statusLabel(status: TaskRunStatus) {
  return (
    (
      {
        created: "已创建",
        queued: "排队中",
        starting: "启动中",
        running: "运行中",
        checkpointing: "检查点中",
        pausing: "暂停中",
        paused: "已暂停",
        resuming: "恢复中",
        cancelling: "取消中",
        cancelled: "已取消",
        succeeded: "成功",
        failed: "失败",
        timed_out: "已超时",
        recovery_required: "需恢复",
        manual_intervention: "需人工介入",
      } as Record<string, string>
    )[status] || status
  );
}

function statusType(status: TaskRunStatus) {
  if (status === "succeeded") return "success";
  if (status === "failed" || status === "timed_out" || status === "cancelled")
    return "danger";
  if (
    status === "paused" ||
    status === "recovery_required" ||
    status === "manual_intervention"
  )
    return "warning";
  return "info";
}

function priorityType(priority: number) {
  if (priority <= 1) return "danger";
  if (priority <= 3) return "warning";
  return "info";
}

function checkpointType(status: string) {
  if (status === "validated" || status === "verified" || status === "ok")
    return "success";
  if (status === "failed" || status === "corrupted") return "danger";
  return "info";
}

function timelineNodeType(item: TaskProgress) {
  const status = task.value?.status;
  if (status === "succeeded" && item === latestProgress.value) return "success";
  if (status === "failed" || status === "timed_out")
    return item === latestProgress.value ? "danger" : "primary";
  return item === latestProgress.value ? "warning" : "primary";
}

function formatPercent(value: number) {
  const num = Number(value);
  if (!Number.isFinite(num)) return 0;
  return Math.round(num);
}

function formatTime(value?: string) {
  if (!value) return "—";
  const date = new Date(value);
  return Number.isNaN(date.getTime())
    ? value
    : date.toLocaleString("zh-CN", { hour12: false });
}

function shortId(id: string) {
  return id.length > 12 ? `${id.slice(0, 8)}…${id.slice(-4)}` : id;
}

function shortHash(hash?: string) {
  if (!hash) return "—";
  return hash.length > 16 ? `${hash.slice(0, 12)}…` : hash;
}

function pretty(value: unknown) {
  if (value === undefined || value === null) return "—";
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

function problem(error: any, fallback: string) {
  return (
    error?.response?.data?.detail ||
    error?.detail ||
    error?.message ||
    fallback
  );
}

onBeforeUnmount(stopPolling);
</script>

<style scoped>
.task-detail {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.section-card {
  margin-top: 16px;
}
.section-card:first-of-type {
  margin-top: 0;
}
.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.card-title {
  font-weight: 600;
  color: var(--ac-color-text);
}
.danger-title {
  color: var(--el-color-danger);
}
code {
  overflow-wrap: anywhere;
}
.error-text {
  margin: 0;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  color: var(--el-color-danger);
  font:
    12px/1.6 "SFMono-Regular",
    Consolas,
    monospace;
}
.json-text {
  max-height: 260px;
  overflow: auto;
  margin: 0;
  padding: 10px;
  border-radius: var(--ac-radius-sm);
  background: var(--ac-color-bg-secondary);
  color: var(--ac-color-text);
  font:
    12px/1.6 "SFMono-Regular",
    Consolas,
    monospace;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.timeline-stage {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 4px;
}
.stage-tag {
  font-weight: 600;
  color: var(--ac-color-text);
}
.stage-percent {
  color: var(--el-color-primary);
  font-variant-numeric: tabular-nums;
}
.timeline-meta {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 12px;
  color: var(--ac-color-text-muted);
}
.timeline-message {
  overflow-wrap: anywhere;
}
.drawer-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  flex-wrap: wrap;
}
</style>

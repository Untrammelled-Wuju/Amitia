<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { Refresh, Search, VideoPause, VideoPlay, RefreshRight, SetUp } from "@element-plus/icons-vue";
import {
  listTasks,
  getTask,
  enqueueTask,
  cancelTask,
  retryTask,
  recoverTask,
  getTaskProgress,
  getTaskResult,
  getTaskCheckpoint,
  listTaskDefinitions,
} from "./api";
import {
  type TaskRun,
  type TaskDefinition,
  type TaskRunProgress,
  type TaskRunResult,
  type TaskCheckpoint,
  type TaskRunStatus,
  type EnqueueTaskRequest,
  STATUS_LABELS,
  STATUS_TAG_TYPES,
  isTerminal,
  isActive,
} from "./types";

const loading = ref(false);
const tasks = ref<TaskRun[]>([]);
const definitions = ref<TaskDefinition[]>([]);
const statusFilter = ref<string>("");
const extensionFilter = ref<string>("");
const activeTab = ref<"runs" | "definitions">("runs");
const detailVisible = ref(false);
const detailTask = ref<TaskRun | null>(null);
const detailProgress = ref<TaskRunProgress | null>(null);
const detailResult = ref<TaskRunResult | null>(null);
const detailCheckpoint = ref<TaskCheckpoint | null>(null);
const detailLoading = ref(false);
const enqueueDialogVisible = ref(false);
const enqueueForm = ref<{ taskDefinitionId: string; input: string; priority: number }>({
  taskDefinitionId: "",
  input: "{}",
  priority: 0,
});
const enqueueLoading = ref(false);
let refreshTimer: ReturnType<typeof setInterval> | null = null;

const filteredTasks = computed(() => {
  return tasks.value.filter((t) => {
    if (statusFilter.value && t.status !== statusFilter.value) return false;
    if (extensionFilter.value && !t.extensionId.includes(extensionFilter.value)) return false;
    return true;
  });
});

const activeTaskCount = computed(() => tasks.value.filter((t) => isActive(t.status)).length);
const succeededCount = computed(() => tasks.value.filter((t) => t.status === "succeeded").length);
const failedCount = computed(() => tasks.value.filter((t) => t.status === "failed" || t.status === "timed_out").length);

const statusOptions = [
  { label: "全部", value: "" },
  { label: "排队中", value: "queued" },
  { label: "运行中", value: "running" },
  { label: "已成功", value: "succeeded" },
  { label: "已失败", value: "failed" },
  { label: "已取消", value: "cancelled" },
  { label: "需恢复", value: "recovery_required" },
  { label: "需人工干预", value: "manual_intervention" },
];

async function fetchTasks() {
  loading.value = true;
  try {
    const res = await listTasks({});
    tasks.value = res.items || [];
  } catch (e: unknown) {
    ElMessage.error("加载任务列表失败: " + (e instanceof Error ? e.message : String(e)));
  } finally {
    loading.value = false;
  }
}

async function fetchDefinitions() {
  try {
    const res = await listTaskDefinitions();
    definitions.value = res.items || [];
  } catch (e: unknown) {
    ElMessage.error("加载任务定义失败: " + (e instanceof Error ? e.message : String(e)));
  }
}

async function handleRefresh() {
  await Promise.all([fetchTasks(), fetchDefinitions()]);
}

function startAutoRefresh() {
  stopAutoRefresh();
  refreshTimer = setInterval(() => {
    if (activeTaskCount.value > 0) {
      fetchTasks();
    }
  }, 3000);
}

function stopAutoRefresh() {
  if (refreshTimer) {
    clearInterval(refreshTimer);
    refreshTimer = null;
  }
}

async function showDetail(taskRunId: string) {
  detailVisible.value = true;
  detailLoading.value = true;
  detailTask.value = null;
  detailProgress.value = null;
  detailResult.value = null;
  detailCheckpoint.value = null;
  try {
    const [task, progress, result, checkpoint] = await Promise.allSettled([
      getTask(taskRunId),
      getTaskProgress(taskRunId),
      getTaskResult(taskRunId),
      getTaskCheckpoint(taskRunId),
    ]);
    if (task.status === "fulfilled") detailTask.value = task.value;
    if (progress.status === "fulfilled") detailProgress.value = progress.value;
    if (result.status === "fulfilled") detailResult.value = result.value;
    if (checkpoint.status === "fulfilled") detailCheckpoint.value = checkpoint.value;
  } finally {
    detailLoading.value = false;
  }
}

async function handleCancel(taskRunId: string) {
  try {
    await ElMessageBox.confirm("确认取消此任务？", "取消任务", { type: "warning" });
    await cancelTask(taskRunId);
    ElMessage.success("已发送取消请求");
    fetchTasks();
  } catch (e: unknown) {
    if (e !== "cancel") {
      ElMessage.error("取消失败: " + (e instanceof Error ? e.message : String(e)));
    }
  }
}

async function handleRetry(taskRunId: string) {
  try {
    await retryTask(taskRunId);
    ElMessage.success("已重新入队");
    fetchTasks();
  } catch (e: unknown) {
    ElMessage.error("重试失败: " + (e instanceof Error ? e.message : String(e)));
  }
}

async function handleRecover(taskRunId: string) {
  try {
    await ElMessageBox.confirm("确认恢复此任务？将从最近的检查点继续。", "恢复任务", { type: "info" });
    await recoverTask(taskRunId);
    ElMessage.success("已提交恢复请求");
    fetchTasks();
  } catch (e: unknown) {
    if (e !== "cancel") {
      ElMessage.error("恢复失败: " + (e instanceof Error ? e.message : String(e)));
    }
  }
}

function openEnqueue(def: TaskDefinition) {
  enqueueForm.value = {
    taskDefinitionId: def.taskId,
    input: "{}",
    priority: 0,
  };
  enqueueDialogVisible.value = true;
}

async function handleEnqueue() {
  if (!enqueueForm.value.taskDefinitionId) {
    ElMessage.warning("请选择任务定义");
    return;
  }
  let input: unknown;
  try {
    input = JSON.parse(enqueueForm.value.input);
  } catch {
    ElMessage.error("输入 JSON 格式无效");
    return;
  }
  enqueueLoading.value = true;
  try {
    const def = definitions.value.find((d) => d.taskId === enqueueForm.value.taskDefinitionId);
    const req: EnqueueTaskRequest = {
      taskDefinitionId: enqueueForm.value.taskDefinitionId,
      extensionId: def?.extensionId,
      moduleId: def?.moduleId,
      input,
      priority: enqueueForm.value.priority,
    };
    const result = await enqueueTask(req);
    ElMessage.success(`任务已入队: ${result.taskRunId}`);
    enqueueDialogVisible.value = false;
    fetchTasks();
  } catch (e: unknown) {
    ElMessage.error("入队失败: " + (e instanceof Error ? e.message : String(e)));
  } finally {
    enqueueLoading.value = false;
  }
}

function formatTime(t?: string): string {
  if (!t) return "-";
  return new Date(t).toLocaleString("zh-CN");
}

function formatDuration(start?: string, end?: string): string {
  if (!start) return "-";
  const s = new Date(start).getTime();
  const e = end ? new Date(end).getTime() : Date.now();
  const diff = Math.max(0, e - s);
  if (diff < 1000) return `${diff}ms`;
  if (diff < 60000) return `${(diff / 1000).toFixed(1)}s`;
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m${Math.floor((diff % 60000) / 1000)}s`;
  return `${Math.floor(diff / 3600000)}h${Math.floor((diff % 3600000) / 60000)}m`;
}

function formatPayload(payload: unknown): string {
  try {
    return JSON.stringify(payload, null, 2);
  } catch {
    return String(payload);
  }
}

const progressPercentage = computed(() => {
  if (!detailProgress.value) return 0;
  if (detailProgress.value.percentage != null) return detailProgress.value.percentage;
  if (detailProgress.value.current != null && detailProgress.value.total != null && detailProgress.value.total > 0) {
    return Math.min(100, Math.round((detailProgress.value.current / detailProgress.value.total) * 100));
  }
  return 0;
});

watch(activeTab, (val) => {
  if (val === "definitions" && definitions.value.length === 0) {
    fetchDefinitions();
  }
});

onMounted(() => {
  handleRefresh();
  startAutoRefresh();
});

onUnmounted(() => {
  stopAutoRefresh();
});
</script>

<template>
  <div class="task-center">
    <div class="task-header">
      <div class="header-left">
        <h2 class="page-title">任务运行时</h2>
        <div class="stats-bar">
          <el-tag type="info">总计 {{ tasks.length }}</el-tag>
          <el-tag type="warning">活跃 {{ activeTaskCount }}</el-tag>
          <el-tag type="success">成功 {{ succeededCount }}</el-tag>
          <el-tag type="danger">失败 {{ failedCount }}</el-tag>
        </div>
      </div>
      <div class="header-right">
        <el-button :icon="Refresh" @click="handleRefresh" :loading="loading">刷新</el-button>
      </div>
    </div>

    <el-tabs v-model="activeTab" class="task-tabs">
      <el-tab-pane label="任务运行" name="runs">
        <div class="filter-bar">
          <el-select v-model="statusFilter" placeholder="状态筛选" clearable style="width: 160px">
            <el-option v-for="opt in statusOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
          </el-select>
          <el-input v-model="extensionFilter" placeholder="扩展ID筛选" clearable style="width: 240px" :prefix-icon="Search" />
        </div>

        <el-table :data="filteredTasks" v-loading="loading" style="width: 100%" row-key="taskRunId" @row-click="(row: TaskRun) => showDetail(row.taskRunId)">
          <el-table-column prop="taskRunId" label="任务运行ID" width="200" show-overflow-tooltip>
            <template #default="{ row }">
              <span class="mono-text">{{ row.taskRunId.substring(0, 16) }}...</span>
            </template>
          </el-table-column>
          <el-table-column prop="taskDefinitionId" label="任务定义" width="180" show-overflow-tooltip />
          <el-table-column prop="extensionId" label="扩展" width="180" show-overflow-tooltip />
          <el-table-column prop="status" label="状态" width="120">
            <template #default="{ row }">
              <el-tag :type="STATUS_TAG_TYPES[row.status as TaskRunStatus]" size="small">
                {{ STATUS_LABELS[row.status as TaskRunStatus] }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="priority" label="优先级" width="80" />
          <el-table-column prop="attempt" label="尝试" width="80">
            <template #default="{ row }">
              {{ row.attempt }}/{{ row.maxAttempts }}
            </template>
          </el-table-column>
          <el-table-column label="耗时" width="100">
            <template #default="{ row }">
              {{ formatDuration(row.startedAt, row.finishedAt) }}
            </template>
          </el-table-column>
          <el-table-column prop="createdAt" label="创建时间" width="180">
            <template #default="{ row }">
              {{ formatTime(row.createdAt) }}
            </template>
          </el-table-column>
          <el-table-column label="操作" width="200" fixed="right">
            <template #default="{ row }">
              <el-button v-if="isActive(row.status)" size="small" type="danger" :icon="VideoPause" @click.stop="handleCancel(row.taskRunId)">取消</el-button>
              <el-button v-if="row.status === 'recovery_required'" size="small" type="warning" :icon="RefreshRight" @click.stop="handleRecover(row.taskRunId)">恢复</el-button>
              <el-button v-if="isTerminal(row.status) && row.status !== 'cancelled'" size="small" :icon="VideoPlay" @click.stop="handleRetry(row.taskRunId)">重试</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="任务定义" name="definitions">
        <el-table :data="definitions" style="width: 100%" row-key="taskId">
          <el-table-column prop="taskId" label="任务ID" width="200" show-overflow-tooltip />
          <el-table-column prop="extensionId" label="扩展" width="200" show-overflow-tooltip />
          <el-table-column prop="moduleId" label="模块" width="120" show-overflow-tooltip />
          <el-table-column prop="runtimeType" label="运行时" width="140" />
          <el-table-column prop="entry" label="入口" width="200" show-overflow-tooltip />
          <el-table-column label="特性" width="160">
            <template #default="{ row }">
              <el-tag v-if="row.checkpoint" size="small" type="info" style="margin-right: 4px">检查点</el-tag>
              <el-tag v-if="row.recoverable" size="small" type="success" style="margin-right: 4px">可恢复</el-tag>
              <el-tag v-if="row.idempotent" size="small">幂等</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="100" fixed="right">
            <template #default="{ row }">
              <el-button size="small" type="primary" :icon="SetUp" @click="openEnqueue(row)">执行</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>
    </el-tabs>

    <el-dialog v-model="detailVisible" title="任务详情" width="780px" destroy-on-close>
      <div v-loading="detailLoading">
        <template v-if="detailTask">
          <el-descriptions :column="2" border>
            <el-descriptions-item label="运行ID">{{ detailTask.taskRunId }}</el-descriptions-item>
            <el-descriptions-item label="状态">
              <el-tag :type="STATUS_TAG_TYPES[detailTask.status]" size="small">{{ STATUS_LABELS[detailTask.status] }}</el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="任务定义">{{ detailTask.taskDefinitionId }}</el-descriptions-item>
            <el-descriptions-item label="扩展">{{ detailTask.extensionId }}</el-descriptions-item>
            <el-descriptions-item label="尝试次数">{{ detailTask.attempt }} / {{ detailTask.maxAttempts }}</el-descriptions-item>
            <el-descriptions-item label="优先级">{{ detailTask.priority }}</el-descriptions-item>
            <el-descriptions-item label="创建时间">{{ formatTime(detailTask.createdAt) }}</el-descriptions-item>
            <el-descriptions-item label="开始时间">{{ formatTime(detailTask.startedAt) }}</el-descriptions-item>
            <el-descriptions-item label="完成时间">{{ formatTime(detailTask.finishedAt) }}</el-descriptions-item>
            <el-descriptions-item label="截止时间">{{ formatTime(detailTask.deadlineAt) }}</el-descriptions-item>
            <el-descriptions-item v-if="detailTask.errorCode" label="错误代码">
              <el-tag type="danger" size="small">{{ detailTask.errorCode }}</el-tag>
            </el-descriptions-item>
            <el-descriptions-item v-if="detailTask.errorMessage" label="错误信息" :span="2">{{ detailTask.errorMessage }}</el-descriptions-item>
          </el-descriptions>

          <div v-if="detailProgress" class="detail-section">
            <h4>进度</h4>
            <el-progress :percentage="progressPercentage" :status="detailTask.status === 'succeeded' ? 'success' : detailTask.status === 'failed' ? 'exception' : undefined" />
            <div class="progress-info">
              <span v-if="detailProgress.current != null && detailProgress.total != null">{{ detailProgress.current }} / {{ detailProgress.total }}</span>
              <span v-if="detailProgress.stage">{{ detailProgress.stage }}</span>
              <span v-if="detailProgress.message">{{ detailProgress.message }}</span>
            </div>
          </div>

          <div v-if="detailResult" class="detail-section">
            <h4>结果</h4>
            <el-descriptions :column="1" border>
              <el-descriptions-item label="类型">{{ detailResult.resultType }}</el-descriptions-item>
              <el-descriptions-item v-if="detailResult.artifactId" label="产物ID">{{ detailResult.artifactId }}</el-descriptions-item>
              <el-descriptions-item v-if="detailResult.resultHash" label="哈希">{{ detailResult.resultHash }}</el-descriptions-item>
            </el-descriptions>
            <div v-if="detailResult.resultJson" class="json-block">
              <pre>{{ formatPayload(detailResult.resultJson) }}</pre>
            </div>
          </div>

          <div v-if="detailCheckpoint" class="detail-section">
            <h4>检查点 (v{{ detailCheckpoint.version }})</h4>
            <div class="json-block">
              <pre>{{ formatPayload(detailCheckpoint.payload) }}</pre>
            </div>
          </div>

          <div class="detail-section">
            <h4>输入</h4>
            <div class="json-block">
              <pre>{{ formatPayload(detailTask.input) }}</pre>
            </div>
          </div>

          <div class="detail-actions">
            <el-button v-if="isActive(detailTask.status)" type="danger" :icon="VideoPause" @click="handleCancel(detailTask.taskRunId)">取消任务</el-button>
            <el-button v-if="detailTask.status === 'recovery_required'" type="warning" :icon="RefreshRight" @click="handleRecover(detailTask.taskRunId)">恢复任务</el-button>
            <el-button v-if="isTerminal(detailTask.status)" :icon="VideoPlay" @click="handleRetry(detailTask.taskRunId)">重试任务</el-button>
          </div>
        </template>
      </div>
    </el-dialog>

    <el-dialog v-model="enqueueDialogVisible" title="执行任务" width="560px">
      <el-form label-width="100px">
        <el-form-item label="任务定义">
          <el-select v-model="enqueueForm.taskDefinitionId" placeholder="选择任务定义" style="width: 100%">
            <el-option v-for="def in definitions" :key="def.taskId" :label="`${def.taskId} (${def.extensionId})`" :value="def.taskId" />
          </el-select>
        </el-form-item>
        <el-form-item label="优先级">
          <el-input-number v-model="enqueueForm.priority" :min="0" :max="10" />
        </el-form-item>
        <el-form-item label="输入(JSON)">
          <el-input v-model="enqueueForm.input" type="textarea" :rows="6" placeholder='{"key":"value"}' />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="enqueueDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="enqueueLoading" @click="handleEnqueue">入队</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.task-center {
  padding: 24px;
  max-width: 1400px;
  margin: 0 auto;
}

.task-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 20px;
}

.page-title {
  margin: 0;
  font-size: 22px;
  font-weight: 600;
}

.stats-bar {
  display: flex;
  gap: 8px;
}

.task-tabs {
  margin-top: 8px;
}

.filter-bar {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
}

.mono-text {
  font-family: 'Courier New', monospace;
  font-size: 13px;
}

.detail-section {
  margin-top: 20px;
}

.detail-section h4 {
  margin-bottom: 12px;
  color: var(--el-text-color-primary);
}

.progress-info {
  display: flex;
  gap: 16px;
  margin-top: 8px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.json-block {
  background: var(--el-fill-color-light);
  border-radius: 4px;
  padding: 12px;
  max-height: 240px;
  overflow: auto;
}

.json-block pre {
  margin: 0;
  font-family: 'Courier New', monospace;
  font-size: 13px;
  white-space: pre-wrap;
  word-break: break-all;
}

.detail-actions {
  margin-top: 24px;
  display: flex;
  gap: 12px;
  justify-content: flex-end;
}

:deep(.el-table__row) {
  cursor: pointer;
}
</style>

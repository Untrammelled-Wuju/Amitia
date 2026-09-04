<template>
  <div class="extension-page">
    <ExtensionPageHeader
      title="任务运行"
      description="查看 Task Runtime 任务执行状态、进度、检查点与结果产物，并支持取消、重试和恢复操作。"
    >
      <template #actions>
        <el-button :icon="Refresh" :loading="loading" @click="load"
          >刷新</el-button
        >
      </template>
    </ExtensionPageHeader>

    <el-card shadow="never" class="filter-card">
      <el-form :inline="true" label-position="top" @submit.prevent>
        <el-form-item label="ExtensionID">
          <el-input
            v-model.trim="filters.extensionId"
            clearable
            placeholder="按扩展 ID 过滤"
            @keyup.enter="search"
            @clear="search"
          />
        </el-form-item>
        <el-form-item label="状态">
          <el-select
            v-model="filters.status"
            clearable
            placeholder="全部状态"
            @change="search"
          >
            <el-option
              v-for="item in statusOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="每页条数">
          <el-select v-model="pageSize" @change="search">
            <el-option :value="20" label="20" />
            <el-option :value="50" label="50" />
            <el-option :value="100" label="100" />
          </el-select>
        </el-form-item>
      </el-form>
    </el-card>

    <el-alert
      v-if="loadError"
      :title="loadError"
      type="error"
      show-icon
      :closable="false"
    >
      <template #default>
        <el-button link type="primary" @click="load">重新加载</el-button>
      </template>
    </el-alert>

    <el-card shadow="never" class="table-card">
      <el-table
        v-loading="loading"
        :data="page.items"
        row-key="taskRunId"
        empty-text="暂无任务"
        stripe
      >
        <el-table-column label="TaskRunID" min-width="200">
          <template #default="{ row }">
            <button
              class="run-link"
              type="button"
              @click="openDetail(row.taskRunId)"
            >
              {{ shortId(row.taskRunId) }}
            </button>
          </template>
        </el-table-column>
        <el-table-column
          prop="extensionId"
          label="ExtensionID"
          min-width="180"
          show-overflow-tooltip
        >
          <template #default="{ row }">
            <code>{{ row.extensionId }}</code>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)" size="small">
              {{ statusLabel(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="优先级" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="priorityType(row.priority)" size="small" effect="plain">
              P{{ row.priority }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="进度" min-width="180">
          <template #default="{ row }">
            <el-progress
              :percentage="rowPercent(row)"
              :status="rowProgressStatus(row.status)"
              :stroke-width="8"
            />
          </template>
        </el-table-column>
        <el-table-column label="开始时间" min-width="160">
          <template #default="{ row }">
            {{ formatTime(row.startedAt) }}
          </template>
        </el-table-column>
        <el-table-column label="结束时间" min-width="160">
          <template #default="{ row }">
            {{ formatTime(row.finishedAt) }}
          </template>
        </el-table-column>
        <el-table-column label="错误" min-width="180">
          <template #default="{ row }">
            <el-tooltip
              v-if="row.errorMessage || row.errorCode"
              :content="row.errorMessage || row.errorCode"
              placement="top"
            >
              <span class="error-cell">
                <el-tag type="danger" size="small" effect="plain">
                  {{ row.errorCode || "错误" }}
                </el-tag>
                <span class="error-text">{{ row.errorMessage }}</span>
              </span>
            </el-tooltip>
            <span v-else class="muted">—</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="240" fixed="right">
          <template #default="{ row }">
            <div class="actions">
              <el-button size="small" @click="openDetail(row.taskRunId)"
                >详情</el-button
              >
              <el-button
                size="small"
                type="danger"
                plain
                :loading="actingId === row.taskRunId"
                :disabled="!canCancel(row.status)"
                @click="onCancel(row)"
                >取消</el-button
              >
              <el-button
                size="small"
                type="primary"
                plain
                :loading="actingId === row.taskRunId"
                :disabled="!canRetry(row.status)"
                @click="onRetry(row)"
                >重试</el-button
              >
            </div>
          </template>
        </el-table-column>
      </el-table>
      <div class="pagination-bar">
        <span>共 {{ page.total }} 条</span>
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :total="page.total"
          :page-sizes="[20, 50, 100]"
          layout="sizes, prev, pager, next"
          @current-change="load"
          @size-change="search"
        />
      </div>
    </el-card>

    <TaskRunDetailView
      v-model="detailVisible"
      :task-run-id="selectedTaskRunId"
      @refresh="load"
    />
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { Refresh } from "@element-plus/icons-vue";
import ExtensionPageHeader from "./components/ExtensionPageHeader.vue";
import TaskRunDetailView from "./TaskRunDetailView.vue";
import { cancelTask, fetchTasks, retryTask } from "./api";
import type { TaskRun, TaskRunPage, TaskRunStatus } from "./types";

const loading = ref(false);
const loadError = ref("");
const actingId = ref("");
const page = ref<TaskRunPage>({ items: [], total: 0 });
const currentPage = ref(1);
const pageSize = ref(20);
const detailVisible = ref(false);
const selectedTaskRunId = ref("");
const filters = reactive<{
  extensionId: string;
  status: TaskRunStatus | "";
}>({ extensionId: "", status: "" });

const statusOptions: Array<{ value: TaskRunStatus; label: string }> = [
  { value: "created", label: "已创建" },
  { value: "queued", label: "排队中" },
  { value: "starting", label: "启动中" },
  { value: "running", label: "运行中" },
  { value: "checkpointing", label: "检查点中" },
  { value: "pausing", label: "暂停中" },
  { value: "paused", label: "已暂停" },
  { value: "resuming", label: "恢复中" },
  { value: "cancelling", label: "取消中" },
  { value: "cancelled", label: "已取消" },
  { value: "succeeded", label: "成功" },
  { value: "failed", label: "失败" },
  { value: "timed_out", label: "已超时" },
  { value: "recovery_required", label: "需恢复" },
  { value: "manual_intervention", label: "需人工介入" },
];

const cancelDisabledStatuses: TaskRunStatus[] = [
  "cancelling",
  "cancelled",
  "succeeded",
  "failed",
  "timed_out",
  "manual_intervention",
];
const retryableStatuses: TaskRunStatus[] = [
  "failed",
  "timed_out",
  "cancelled",
];

async function load() {
  loading.value = true;
  loadError.value = "";
  try {
    page.value = await fetchTasks({
      extensionId: filters.extensionId || undefined,
      status: filters.status || undefined,
      page: currentPage.value,
      pageSize: pageSize.value,
    });
  } catch (error: any) {
    page.value = { items: [], total: 0 };
    loadError.value = problem(error, "任务列表加载失败");
  } finally {
    loading.value = false;
  }
}

function search() {
  currentPage.value = 1;
  load();
}

function openDetail(taskRunId: string) {
  selectedTaskRunId.value = taskRunId;
  detailVisible.value = true;
}

function canCancel(status: TaskRunStatus) {
  return !cancelDisabledStatuses.includes(status);
}

function canRetry(status: TaskRunStatus) {
  return retryableStatuses.includes(status);
}

async function onCancel(task: TaskRun) {
  try {
    await ElMessageBox.confirm(
      `确认取消任务 ${shortId(task.taskRunId)}？取消后任务将停止执行。`,
      "取消任务",
      { type: "warning", confirmButtonText: "取消任务", cancelButtonText: "保留" },
    );
  } catch {
    return;
  }
  actingId.value = task.taskRunId;
  try {
    await cancelTask(task.taskRunId);
    ElMessage.success("已请求取消任务");
    await load();
  } catch (error: any) {
    ElMessage.error(problem(error, "取消任务失败"));
  } finally {
    actingId.value = "";
  }
}

async function onRetry(task: TaskRun) {
  actingId.value = task.taskRunId;
  try {
    await retryTask(task.taskRunId);
    ElMessage.success("已重新提交任务");
    await load();
  } catch (error: any) {
    ElMessage.error(problem(error, "重试任务失败"));
  } finally {
    actingId.value = "";
  }
}

function rowPercent(row: TaskRun) {
  const value = Number(row.progress?.percentage);
  if (!Number.isFinite(value)) return 0;
  return Math.max(0, Math.min(100, Math.round(value)));
}

function rowProgressStatus(status: TaskRunStatus) {
  if (status === "succeeded") return "success" as const;
  if (
    status === "failed" ||
    status === "timed_out" ||
    status === "cancelled" ||
    status === "manual_intervention"
  )
    return "exception" as const;
  if (status === "paused" || status === "recovery_required")
    return "warning" as const;
  return "" as const;
}

function statusLabel(status: TaskRunStatus) {
  return (
    statusOptions.find((item) => item.value === status)?.label || status
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

function formatTime(value?: string) {
  if (!value) return "—";
  const date = new Date(value);
  return Number.isNaN(date.getTime())
    ? value
    : date.toLocaleString("zh-CN", { hour12: false });
}

function shortId(id: string) {
  return id.length > 14 ? `${id.slice(0, 8)}…${id.slice(-4)}` : id;
}

function problem(error: any, fallback: string) {
  return (
    error?.response?.data?.detail ||
    error?.detail ||
    error?.message ||
    fallback
  );
}

onMounted(load);
</script>

<style scoped>
.extension-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
}
.filter-card :deep(.el-card__body) {
  padding-bottom: 2px;
}
.filter-card :deep(.el-form-item) {
  min-width: 200px;
  margin-right: 16px;
}
.table-card :deep(.el-card__body) {
  padding: 0;
  overflow-x: auto;
}
.run-link {
  border: 0;
  background: transparent;
  color: var(--ac-color-primary);
  cursor: pointer;
  text-align: left;
  font-family: "SFMono-Regular", Consolas, monospace;
}
.run-link:focus-visible {
  outline: 2px solid var(--ac-color-primary);
  outline-offset: 2px;
}
.actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.error-cell {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  max-width: 100%;
}
.error-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--ac-color-text-muted);
  font-size: 12px;
  max-width: 120px;
}
.muted {
  color: var(--ac-color-text-muted);
}
.pagination-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px;
  color: var(--ac-color-text-secondary);
}
code {
  overflow-wrap: anywhere;
}
@media (max-width: 720px) {
  .pagination-bar {
    align-items: stretch;
    flex-direction: column;
  }
  .filter-card :deep(.el-form),
  .filter-card :deep(.el-form-item) {
    display: block;
    width: 100%;
  }
}
</style>

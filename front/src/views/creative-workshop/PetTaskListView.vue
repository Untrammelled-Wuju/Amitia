<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <main class="pet-task-list">
    <header class="page-header">
      <div class="title-row">
        <div>
          <h1>桌宠生成任务</h1>
          <p>查看桌宠生成任务的状态与动作明细</p>
        </div>
        <div class="header-actions">
          <el-button :icon="Plus" @click="goCreate">新建桌宠</el-button>
          <el-button :icon="Refresh" :loading="loading" @click="loadList"
            >刷新</el-button
          >
        </div>
      </div>
    </header>

    <el-card shadow="never" class="filter-card">
      <div class="filter-row">
        <el-select
          v-model="filter.status"
          placeholder="全部状态"
          clearable
          style="width: 180px"
          @change="onFilterChange"
        >
          <el-option
            v-for="item in statusOptions"
            :key="item.value"
            :label="item.label"
            :value="item.value"
          />
        </el-select>
        <span class="filter-tip">共 {{ total }} 个任务</span>
      </div>
    </el-card>

    <el-card shadow="never" class="table-card">
      <el-table
        v-loading="loading"
        :data="items"
        row-key="id"
        empty-text="暂无任务"
        @expand-change="onExpandChange"
      >
        <el-table-column type="expand">
          <template #default="{ row }">
            <div class="expand-body">
              <div v-if="detailLoadingId === row.id" class="expand-loading">
                加载明细中...
              </div>
              <template v-else-if="detailMap[row.id]">
                <el-descriptions :column="2" border size="small">
                  <el-descriptions-item label="任务ID">{{
                    row.id
                  }}</el-descriptions-item>
                  <el-descriptions-item label="当前阶段">{{
                    detailMap[row.id]?.currentStage || "—"
                  }}</el-descriptions-item>
                  <el-descriptions-item label="模型">{{
                    row.modelName || "—"
                  }}</el-descriptions-item>
                  <el-descriptions-item label="预估生成">{{
                    row.estimatedGenerationCount ?? "—"
                  }}</el-descriptions-item>
                </el-descriptions>
                <div class="detail-block">
                  <div class="detail-title">动作明细</div>
                  <el-empty
                    v-if="!(detailMap[row.id]?.actions || []).length"
                    description="暂无动作明细"
                    :image-size="60"
                  />
                  <div v-else class="action-grid">
                    <el-tag
                      v-for="action in detailMap[row.id]?.actions || []"
                      :key="actionKey(action)"
                      :type="actionTagType(action.status)"
                      >{{ actionName(action) }}</el-tag
                    >
                  </div>
                </div>
                <div v-if="referenceUrls[row.id]" class="detail-block">
                  <div class="detail-title">参考图</div>
                  <img
                    :src="referenceUrls[row.id]"
                    alt="参考图"
                    class="detail-reference"
                  />
                </div>
              </template>
              <div v-else class="expand-loading">点击展开以加载明细</div>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="name" label="任务名称" min-width="160" />
        <el-table-column label="角色" min-width="120">
          <template #default="{ row }">{{
            row.characterName || "—"
          }}</template>
        </el-table-column>
        <el-table-column label="模型" min-width="140">
          <template #default="{ row }">{{
            row.modelName || "—"
          }}</template>
        </el-table-column>
        <el-table-column label="状态" min-width="120">
          <template #default="{ row }">
            <el-tag :type="statusTagType(row.status)">{{
              statusLabel(row.status)
            }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="进度" min-width="160">
          <template #default="{ row }">
            <el-progress
              :percentage="Number(row.progress || 0)"
              :status="progressStatus(row.status)"
            />
          </template>
        </el-table-column>
        <el-table-column label="动作数" min-width="90">
          <template #default="{ row }">{{
            row.selectedActionCount ?? "—"
          }}</template>
        </el-table-column>
        <el-table-column label="创建时间" min-width="170">
          <template #default="{ row }">{{ formatTime(row.createdAt) }}</template>
        </el-table-column>
      </el-table>

      <div class="pagination-row">
        <el-pagination
          v-model:current-page="filter.page"
          v-model:page-size="filter.pageSize"
          :total="total"
          :page-sizes="[10, 20, 50]"
          layout="total, sizes, prev, pager, next"
          background
          @current-change="loadList"
          @size-change="onPageSizeChange"
        />
      </div>
    </el-card>
  </main>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted } from "vue";
import { useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import { Plus, Refresh } from "@element-plus/icons-vue";
import { useApi, apiClient } from "../../composables/useApi";

const router = useRouter();
const { get } = useApi();

interface TaskItem {
  id: string | number;
  name: string;
  characterId?: string | number;
  characterName?: string;
  modelConfigId?: string | number;
  modelName?: string;
  status: string;
  currentStage?: string;
  progress?: number;
  selectedActionCount?: number;
  estimatedGenerationCount?: number;
  createdAt?: string;
}

interface TaskDetail {
  id: string | number;
  actions?: Array<Record<string, any>>;
  referenceImageUrl?: string;
  currentStage?: string;
  [key: string]: any;
}

interface TaskListResponse {
  total: number;
  page: number;
  pageSize: number;
  items: TaskItem[];
}

const items = ref<TaskItem[]>([]);
const total = ref(0);
const loading = ref(false);
const detailLoadingId = ref<string | number | null>(null);
const detailMap = reactive<Record<string, TaskDetail>>({});
const referenceUrls = reactive<Record<string, string>>({});
const expandedIds = new Set<string | number>();

const filter = reactive({
  status: "",
  page: 1,
  pageSize: 20,
});

const statusOptions = [
  { label: "等待生成", value: "pending" },
  { label: "生成中", value: "running" },
  { label: "已完成", value: "success" },
  { label: "失败", value: "failed" },
];

const statusMeta: Record<string, { label: string; type: string }> = {
  pending: { label: "等待生成", type: "info" },
  queued: { label: "排队中", type: "info" },
  running: { label: "生成中", type: "warning" },
  processing: { label: "处理中", type: "warning" },
  success: { label: "已完成", type: "success" },
  completed: { label: "已完成", type: "success" },
  failed: { label: "失败", type: "danger" },
  cancelled: { label: "已取消", type: "info" },
};

function statusLabel(status: string): string {
  return statusMeta[status]?.label || status || "—";
}

function statusTagType(status: string): any {
  const t = statusMeta[status]?.type;
  if (t === "success") return "success";
  if (t === "warning") return "warning";
  if (t === "danger") return "danger";
  return "info";
}

function actionTagType(status?: string): any {
  if (!status) return "info";
  if (status === "success" || status === "completed") return "success";
  if (status === "running" || status === "processing") return "warning";
  if (status === "failed") return "danger";
  return "info";
}

function progressStatus(status: string): "" | "success" | "exception" {
  if (status === "success" || status === "completed") return "success";
  if (status === "failed") return "exception";
  return "";
}

function actionKey(action: Record<string, any>): string | number {
  return action.key || action.id || action.name || Math.random().toString(36);
}

function actionName(action: Record<string, any>): string {
  return (
    action.name ||
    action.key ||
    action.actionName ||
    action.actionKey ||
    "未命名动作"
  );
}

function formatTime(value?: string): string {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(
    date.getDate(),
  )} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

async function loadList() {
  loading.value = true;
  try {
    const params: Record<string, any> = {
      page: filter.page,
      pageSize: filter.pageSize,
    };
    if (filter.status) params.status = filter.status;
    const data = await get<TaskListResponse>(
      "/api/desktop-pets/generation-tasks",
      params,
    );
    items.value = data?.items || [];
    total.value = data?.total || 0;
  } catch (err: any) {
    ElMessage.error(err?.message || "加载任务列表失败");
    items.value = [];
    total.value = 0;
  } finally {
    loading.value = false;
  }
}

function onFilterChange() {
  filter.page = 1;
  loadList();
}

function onPageSizeChange() {
  filter.page = 1;
  loadList();
}

async function onExpandChange(row: TaskItem, expanded: TaskItem[]) {
  const isExpanded = expanded.some((r) => r.id === row.id);
  if (isExpanded) {
    expandedIds.add(row.id);
    if (!detailMap[String(row.id)]) {
      await loadDetail(row.id);
    }
    if (!referenceUrls[String(row.id)]) {
      await loadReference(row.id);
    }
  } else {
    expandedIds.delete(row.id);
  }
}

async function loadDetail(taskId: string | number) {
  detailLoadingId.value = taskId;
  try {
    const data = await get<TaskDetail>(
      `/api/desktop-pets/generation-tasks/${taskId}`,
    );
    detailMap[String(taskId)] = data || { id: taskId, actions: [] };
  } catch (err: any) {
    ElMessage.error(err?.message || "加载任务明细失败");
    detailMap[String(taskId)] = { id: taskId, actions: [] };
  } finally {
    detailLoadingId.value = null;
  }
}

async function loadReference(taskId: string | number) {
  try {
    const res = await apiClient.get(
      `/api/desktop-pets/generation-tasks/${taskId}/reference-image`,
      { responseType: "blob" },
    );
    const blob = res.data as Blob;
    if (blob && blob.size > 0) {
      referenceUrls[String(taskId)] = URL.createObjectURL(blob);
    }
  } catch {
    // ignore
  }
}

function goCreate() {
  router.push("/creative-workshop/pet");
}

onMounted(() => {
  loadList();
});

onUnmounted(() => {
  Object.values(referenceUrls).forEach((url) => {
    if (url) URL.revokeObjectURL(url);
  });
});
</script>

<style scoped>
.pet-task-list {
  height: 100%;
  overflow: auto;
  padding: 0;
}
.page-header h1 {
  margin: 0 0 6px;
  color: var(--console-text);
  font-size: 24px;
}
.page-header p {
  margin: 0;
  color: var(--console-text-muted);
  line-height: 1.6;
}
.title-row {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 16px;
}
.header-actions {
  display: flex;
  gap: 8px;
}
.filter-card {
  margin-bottom: 12px;
  border: 1px solid var(--el-border-color-light);
  background: var(--ac-color-surface);
}
.filter-row {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
.filter-tip {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
.table-card {
  border: 1px solid var(--el-border-color-light);
  background: var(--ac-color-surface);
}
.pagination-row {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}
.expand-body {
  padding: 8px 16px 16px;
  background: var(--ac-color-surface-soft, var(--ac-color-surface));
}
.expand-loading {
  color: var(--el-text-color-secondary);
  font-size: 13px;
  padding: 8px 0;
}
.detail-block {
  margin-top: 14px;
}
.detail-title {
  margin-bottom: 8px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
.action-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.detail-reference {
  width: 140px;
  height: 140px;
  object-fit: cover;
  border-radius: 8px;
  border: 1px solid var(--el-border-color-light);
  background: var(--ac-color-bg-secondary, #f5f7fa);
}
</style>

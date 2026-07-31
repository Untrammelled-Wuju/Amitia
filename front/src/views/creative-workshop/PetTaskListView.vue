<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<!--
Deprecated: Legacy extension architecture.
Do not add new capabilities. This view is retained only for
compatibility, maintenance, testing, and migration to Extension Kernel.
-->
<template>
  <main class="pet-task-list">
    <ExtensionPageHeader
      title="制作记录"
      description="查看桌宠生成任务的状态与进度"
      grandparent-title="创意工坊"
      grandparent-path="/creative-workshop"
      parent-title="桌宠"
      parent-path="/creative-workshop/pet"
    >
      <template #actions>
        <el-button :icon="Plus" @click="goCreate">新建桌宠</el-button>
        <el-button :icon="Refresh" :loading="loading" @click="loadList"
          >刷新</el-button
        >
      </template>
    </ExtensionPageHeader>

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
        <span v-if="connected && trackedTaskId" class="filter-tip sse-tip">
          <span class="sse-dot"></span>
          实时跟踪任务 #{{ trackedTaskId }}
        </span>
      </div>
    </el-card>

    <el-card shadow="never" class="table-card">
      <el-table
        v-loading="loading"
        :data="items"
        row-key="id"
        empty-text="暂无任务"
        :expand-row-keys="expandedRowKeys"
        @expand-change="onExpandChange"
      >
        <el-table-column type="expand">
          <template #default="{ row }">
            <div class="expand-body">
              <div v-if="detailLoadingId === row.id" class="expand-loading">
                加载明细中...
              </div>
              <template v-else-if="detailMap[row.id]">
                <el-descriptions :column="3" border size="small">
                  <el-descriptions-item label="任务ID">{{
                    row.id
                  }}</el-descriptions-item>
                  <el-descriptions-item label="任务名称">{{
                    detailMap[row.id]?.name || row.name || "—"
                  }}</el-descriptions-item>
                  <el-descriptions-item label="模型">{{
                    detailMap[row.id]?.modelName || row.modelName || "—"
                  }}</el-descriptions-item>
                  <el-descriptions-item label="状态">
                    <el-tag :type="statusTagType(detailMap[row.id]?.status)">{{
                      statusLabel(detailMap[row.id]?.status)
                    }}</el-tag>
                  </el-descriptions-item>
                  <el-descriptions-item label="当前阶段">{{
                    stageLabel(detailMap[row.id]?.currentStage)
                  }}</el-descriptions-item>
                  <el-descriptions-item label="当前动作">{{
                    detailMap[row.id]?.currentAction || "—"
                  }}</el-descriptions-item>
                  <el-descriptions-item label="已选动作">{{
                    detailMap[row.id]?.selectedActionCount ?? "—"
                  }}</el-descriptions-item>
                  <el-descriptions-item label="成功动作">{{
                    detailMap[row.id]?.succeededActionCount ?? 0
                  }}</el-descriptions-item>
                  <el-descriptions-item label="失败动作">{{
                    detailMap[row.id]?.failedActionCount ?? 0
                  }}</el-descriptions-item>
                  <el-descriptions-item label="开始时间">{{
                    formatTime(detailMap[row.id]?.startedAt)
                  }}</el-descriptions-item>
                  <el-descriptions-item label="已运行时长">{{
                    formatDuration(detailMap[row.id]?.durationSeconds)
                  }}</el-descriptions-item>
                  <el-descriptions-item label="预估生成">{{
                    detailMap[row.id]?.estimatedGenerationCount ?? "—"
                  }}张</el-descriptions-item>
                  <el-descriptions-item label="真实进度" :span="3">
                    <div class="progress-cell">
                      <el-progress
                        :percentage="Number(detailMap[row.id]?.progress || 0)"
                        :status="progressStatus(detailMap[row.id]?.status)"
                      />
                      <span class="progress-text">{{
                        formatProgress(detailMap[row.id]?.progress)
                      }}</span>
                    </div>
                  </el-descriptions-item>
                  <el-descriptions-item
                    v-if="detailMap[row.id]?.errorMessage"
                    label="错误信息"
                    :span="3"
                  >
                    <span class="error-text">{{
                      detailMap[row.id]?.errorMessage
                    }}</span>
                  </el-descriptions-item>
                </el-descriptions>

                <div class="detail-actions">
                  <el-button
                    v-if="canCancel(detailMap[row.id]?.status)"
                    type="danger"
                    plain
                    :loading="cancellingId === row.id"
                    @click="confirmCancel(row)"
                    >取消任务</el-button
                  >
                  <el-button
                    v-if="canStart(detailMap[row.id]?.status)"
                    type="primary"
                    :loading="startingId === row.id"
                    @click="startTask(row)"
                    >开始生成</el-button
                  >
                  <el-button
                    v-if="canProcess(detailMap[row.id]?.status)"
                    type="success"
                    :icon="Cpu"
                    :loading="processingId === row.id"
                    @click="openProcessDialog(row)"
                    >处理素材</el-button
                  >
                  <el-button
                    v-if="hasPackageForTask(row.id)"
                    type="primary"
                    :icon="Download"
                    :loading="exportingId === row.id"
                    @click="exportPackage(row)"
                    >安装桌宠</el-button
                  >
                </div>

                <div class="detail-block">
                  <div class="detail-title">动作明细</div>
                  <el-empty
                    v-if="!(detailMap[row.id]?.actions || []).length"
                    description="暂无动作明细"
                    :image-size="60"
                  />
                  <div v-else class="action-list">
                    <div
                      v-for="action in detailMap[row.id]?.actions || []"
                      :key="action.actionKey || action.id"
                      class="action-item"
                    >
                      <div class="action-item-head">
                        <div class="action-item-name">
                          <strong>{{
                            action.actionName ||
                            action.actionKey ||
                            "未命名动作"
                          }}</strong>
                          <el-tag
                            size="small"
                            :type="actionTagType(action.status)"
                            >{{ actionStatusLabel(action.status) }}</el-tag
                          >
                          <el-tag
                            v-if="action.categoryName"
                            size="small"
                            type="info"
                            >{{ action.categoryName }}</el-tag
                          >
                        </div>
                        <div class="action-item-meta">
                          <span
                            >帧完成：{{ action.frameSucceeded ?? 0 }}/{{
                              action.frameTotal ??
                              action.estimatedGenerationCount ??
                              "—"
                            }}</span
                          >
                          <span v-if="action.frameFailed" class="warn-text"
                            >失败 {{ action.frameFailed }}</span
                          >
                          <span v-if="action.attemptNumber"
                            >第 {{ action.attemptNumber }} 次尝试</span
                          >
                        </div>
                      </div>
                      <div v-if="action.errorMessage" class="action-error">
                        失败原因：{{ action.errorMessage }}
                      </div>
                      <div class="action-item-foot">
                        <el-button
                          v-if="canRetry(action.status)"
                          size="small"
                          :loading="retryingKey === actionKey(action)"
                          @click="confirmRetry(row, action)"
                          >重试动作</el-button
                        >
                      </div>
                      <div v-if="shouldShowFrames(action)" class="frame-preview">
                        <div class="frame-preview-label">
                          原始生成素材 / 尚未处理
                        </div>
                        <div class="frame-grid">
                          <div
                            v-for="idx in frameIndexList(action)"
                            :key="idx"
                            class="frame-thumb"
                          >
                            <img
                              v-if="frameImageUrls[frameKey(row.id, action, idx)]"
                              :src="frameImageUrls[frameKey(row.id, action, idx)]"
                              :alt="`帧 ${idx}`"
                            />
                            <div v-else class="frame-thumb-placeholder">
                              加载中
                            </div>
                          </div>
                        </div>
                      </div>
                    </div>
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

    <el-dialog
      v-model="processDialogVisible"
      title="创建处理任务"
      width="540px"
      destroy-on-close
    >
      <el-form :model="processForm" label-width="140px">
        <el-form-item label="输出宽度">
          <el-input-number
            v-model="processForm.outputWidth"
            :min="64"
            :max="2048"
            :step="32"
            controls-position="right"
          />
        </el-form-item>
        <el-form-item label="输出高度">
          <el-input-number
            v-model="processForm.outputHeight"
            :min="64"
            :max="2048"
            :step="32"
            controls-position="right"
          />
        </el-form-item>
        <el-form-item label="角色高度比例">
          <el-input-number
            v-model="processForm.targetCharacterHeightRatio"
            :min="0.1"
            :max="1"
            :step="0.05"
            :precision="2"
            controls-position="right"
          />
        </el-form-item>
        <el-form-item label="锚点模式">
          <el-select v-model="processForm.anchorMode" style="width: 100%">
            <el-option label="底部居中" value="bottom_center" />
            <el-option label="居中" value="center" />
            <el-option label="顶部居中" value="top_center" />
          </el-select>
        </el-form-item>
        <el-form-item label="背景模式">
          <el-select v-model="processForm.backgroundMode" style="width: 100%">
            <el-option label="透明背景" value="remove_background" />
            <el-option label="保留背景" value="keep_background" />
            <el-option label="纯色背景" value="solid_color" />
          </el-select>
        </el-form-item>
        <el-form-item label="输出格式">
          <el-select v-model="processForm.outputFormat" style="width: 100%">
            <el-option label="PNG" value="png" />
            <el-option label="WebP" value="webp" />
          </el-select>
        </el-form-item>
        <el-form-item label="默认 FPS">
          <el-input-number
            v-model="processForm.defaultFps"
            :min="1"
            :max="60"
            controls-position="right"
          />
        </el-form-item>
      </el-form>
      <el-alert
        v-if="processError"
        :title="processError"
        type="error"
        :closable="false"
        show-icon
      />
      <template #footer>
        <el-button @click="processDialogVisible = false">取消</el-button>
        <el-button
          type="primary"
          :loading="processingId !== null"
          @click="submitProcess"
          >创建并跳转</el-button
        >
      </template>
    </el-dialog>
  </main>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from "vue";
import { useRouter, useRoute } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import { Plus, Refresh, Cpu, Download } from "@element-plus/icons-vue";
import { useApi, apiClient } from "../../composables/useApi";
import {
  useGenerationTask,
  isTerminalStatus,
} from "../../composables/useGenerationTask";
import { useProcessingTask } from "../../composables/useProcessingTask";
import { useDesktopPetInstallations } from "../../composables/useDesktopPetInstallations";
import ExtensionPageHeader from "../extensions/components/ExtensionPageHeader.vue";

const router = useRouter();
const route = useRoute();
const { get, post } = useApi();

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

interface TaskAction {
  id?: string | number;
  actionKey: string;
  actionName?: string;
  actionDescription?: string;
  categoryKey?: string;
  categoryName?: string;
  definitionVersion?: string;
  supportsDefaultIdle?: boolean;
  sortOrder?: number;
  frameCount?: number;
  estimatedGenerationCount?: number;
  status?: string;
  progress?: number;
  errorCode?: string;
  errorMessage?: string;
  attemptNumber?: number;
  startedAt?: string;
  completedAt?: string;
  frameSucceeded?: number;
  frameFailed?: number;
  frameTotal?: number;
}

interface TaskDetail {
  id: string | number;
  name?: string;
  modelName?: string;
  status?: string;
  currentStage?: string;
  progress?: number;
  selectedActionCount?: number;
  estimatedGenerationCount?: number;
  referenceImageUrl?: string;
  errorMessage?: string;
  createdAt?: string;
  updatedAt?: string;
  startedAt?: string;
  completedAt?: string;
  actions?: TaskAction[];
  succeededActionCount?: number;
  failedActionCount?: number;
  currentAction?: string;
  durationSeconds?: number;
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
const frameImageUrls = reactive<Record<string, string>>({});
const expandedRowKeys = ref<string[]>([]);
const cancellingId = ref<string | number | null>(null);
const startingId = ref<string | number | null>(null);
const retryingKey = ref<string | null>(null);
const trackedTaskId = ref<string | number | null>(null);
const processingId = ref<string | number | null>(null);
const processDialogVisible = ref(false);
const processError = ref("");
const processForm = reactive({
  taskId: "" as string | number,
  outputWidth: 512,
  outputHeight: 512,
  targetCharacterHeightRatio: 0.8,
  anchorMode: "bottom_center",
  backgroundMode: "remove_background",
  outputFormat: "png",
  defaultFps: 12,
});

const exportingId = ref<string | number | null>(null);
const taskPackageMap = reactive<Record<string, PackageSummary[]>>({});

interface PackageSummary {
  id: string;
  petId: string;
  version: string;
  status: string;
  sourceGenerationTask: string;
}

const filter = reactive({
  status: "",
  page: 1,
  pageSize: 20,
});

const statusOptions = [
  { label: "等待生成", value: "pending" },
  { label: "排队中", value: "queued" },
  { label: "处理中", value: "processing" },
  { label: "取消中", value: "cancelling" },
  { label: "已完成", value: "succeeded" },
  { label: "部分成功", value: "partially_succeeded" },
  { label: "失败", value: "failed" },
  { label: "已取消", value: "cancelled" },
];

const statusMeta: Record<string, { label: string; type: string }> = {
  pending: { label: "等待生成", type: "info" },
  queued: { label: "排队中", type: "info" },
  processing: { label: "处理中", type: "warning" },
  cancelling: { label: "取消中", type: "warning" },
  succeeded: { label: "已完成", type: "success" },
  partially_succeeded: { label: "部分成功", type: "warning" },
  failed: { label: "失败", type: "danger" },
  cancelled: { label: "已取消", type: "info" },
  success: { label: "已完成", type: "success" },
  completed: { label: "已完成", type: "success" },
  running: { label: "生成中", type: "warning" },
};

const stageMeta: Record<string, string> = {
  created: "已创建",
  queued: "排队中",
  preparing: "准备中",
  preparing_frames: "准备帧",
  submitting: "提交中",
  polling: "轮询中",
  downloading: "下载中",
  persisting: "持久化中",
  generating: "生成中",
  generating_actions: "生成动作帧",
  downloading_results: "下载结果",
  finalizing: "收尾处理",
  completed: "已完成",
  failed: "失败",
  cancelled: "已取消",
};

const actionStatusMeta: Record<string, { label: string; type: string }> = {
  pending: { label: "等待中", type: "info" },
  queued: { label: "排队中", type: "info" },
  processing: { label: "生成中", type: "warning" },
  cancelling: { label: "取消中", type: "warning" },
  succeeded: { label: "已完成", type: "success" },
  partially_succeeded: { label: "部分成功", type: "warning" },
  failed: { label: "失败", type: "danger" },
  cancelled: { label: "已取消", type: "info" },
  success: { label: "已完成", type: "success" },
  completed: { label: "已完成", type: "success" },
  running: { label: "生成中", type: "warning" },
};

function statusLabel(status?: string): string {
  if (!status) return "—";
  return statusMeta[status]?.label || status;
}

function statusTagType(status?: string): any {
  const t = status ? statusMeta[status]?.type : "";
  if (t === "success") return "success";
  if (t === "warning") return "warning";
  if (t === "danger") return "danger";
  return "info";
}

function stageLabel(stage?: string): string {
  if (!stage) return "—";
  return stageMeta[stage] || stage;
}

function actionStatusLabel(status?: string): string {
  if (!status) return "—";
  return actionStatusMeta[status]?.label || status;
}

function actionTagType(status?: string): any {
  if (!status) return "info";
  const t = actionStatusMeta[status]?.type;
  if (t === "success") return "success";
  if (t === "warning") return "warning";
  if (t === "danger") return "danger";
  return "info";
}

function progressStatus(status?: string): "" | "success" | "exception" {
  if (status === "succeeded" || status === "completed" || status === "partially_succeeded") return "success";
  if (status === "failed") return "exception";
  return "";
}

function canCancel(status?: string): boolean {
  return (
    status === "queued" || status === "processing" || status === "pending" || status === "cancelling"
  );
}

function canStart(status?: string): boolean {
  return (
    status === "pending" ||
    status === "failed" ||
    status === "partially_succeeded" ||
    status === "cancelled" ||
    status === "succeeded"
  );
}

function canRetry(status?: string): boolean {
  return (
    status === "failed" ||
    status === "succeeded" ||
    status === "cancelled" ||
    status === "completed" ||
    status === "success"
  );
}

function canProcess(status?: string): boolean {
  return (
    status === "succeeded" ||
    status === "partially_succeeded" ||
    status === "success" ||
    status === "completed"
  );
}

function hasPackageForTask(taskId: string | number): boolean {
  const packages = taskPackageMap[String(taskId)];
  return packages && packages.length > 0;
}

async function fetchTaskPackages(taskId: string | number) {
  try {
    const data = await get<{ items: PackageSummary[]; total: number }>(
      `/api/desktop-pets/releases`,
    );
    const all = data?.items || [];
    taskPackageMap[String(taskId)] = all.filter(
      (r) => String(r.sourceGenerationTask) === String(taskId),
    );
  } catch {
    taskPackageMap[String(taskId)] = [];
  }
}

async function exportPackage(row: TaskItem) {
  const packages = taskPackageMap[String(row.id)];
  if (!packages || !packages.length) {
    ElMessage.warning("暂无可用资源包");
    return;
  }
  const pkg = packages[0];
  if (!pkg.petId || !pkg.id) {
    ElMessage.warning("资源包信息不完整");
    return;
  }
  exportingId.value = row.id;
  try {
    await installRelease(pkg.petId, pkg.id);
    ElMessage.success("桌宠已安装");
  } catch (err: any) {
    ElMessage.error(err?.message || "安装失败");
  } finally {
    exportingId.value = null;
  }
}

function shouldShowFrames(action: TaskAction): boolean {
  const succeeded =
    action.status === "succeeded" ||
    action.status === "success" ||
    action.status === "completed";
  const total =
    Number(action.frameTotal ?? action.frameSucceeded ?? 0) || 0;
  return succeeded && total > 0;
}

function actionKey(action: TaskAction): string {
  return (
    action.actionKey ||
    (action.id as any) ||
    action.actionName ||
    Math.random().toString(36)
  );
}

function frameKey(
  taskId: string | number,
  action: TaskAction,
  idx: number,
): string {
  return `${taskId}:${action.actionKey}:${idx}`;
}

function frameIndexList(action: TaskAction): number[] {
  const total = Number(action.frameTotal ?? action.frameSucceeded ?? 0) || 0;
  const list: number[] = [];
  for (let i = 0; i < total; i++) list.push(i);
  return list;
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

function formatDuration(seconds?: number): string {
  if (
    seconds === undefined ||
    seconds === null ||
    Number.isNaN(Number(seconds))
  )
    return "—";
  const s = Math.max(0, Math.floor(Number(seconds)));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = s % 60;
  const pad = (n: number) => String(n).padStart(2, "0");
  if (h > 0) return `${h}:${pad(m)}:${pad(sec)}`;
  return `${m}:${pad(sec)}`;
}

function formatProgress(progress?: number): string {
  if (progress === undefined || progress === null) return "0%";
  return `${Math.round(Number(progress) || 0)}%`;
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

const { connected, start: startTracking, stop: stopTracking } =
  useGenerationTask({
    refresh: async () => {
      if (trackedTaskId.value) {
        await loadDetail(trackedTaskId.value);
        await loadActionFrames(trackedTaskId.value);
      }
    },
    getStatus: () => {
      if (!trackedTaskId.value) return undefined;
      return detailMap[String(trackedTaskId.value)]?.status;
    },
  });

const { createProcessingTask: createProcessingTaskApi } =
  useProcessingTask();
const { install: installRelease } = useDesktopPetInstallations();

function openProcessDialog(row: TaskItem) {
  processForm.taskId = row.id;
  processError.value = "";
  const detail = detailMap[String(row.id)];
  const width = Number(detail?.outputWidth) || 512;
  const height = Number(detail?.outputHeight) || 512;
  processForm.outputWidth = width;
  processForm.outputHeight = height;
  processForm.targetCharacterHeightRatio = 0.8;
  processForm.anchorMode = "bottom_center";
  processForm.backgroundMode = "remove_background";
  processForm.outputFormat = "png";
  processForm.defaultFps = 12;
  processDialogVisible.value = true;
}

async function submitProcess() {
  if (!processForm.taskId) {
    processError.value = "缺少任务 ID";
    return;
  }
  if (
    !processForm.outputWidth ||
    !processForm.outputHeight ||
    processForm.outputWidth < 64 ||
    processForm.outputHeight < 64
  ) {
    processError.value = "输出宽高必须 ≥ 64";
    return;
  }
  processError.value = "";
  processingId.value = processForm.taskId;
  try {
    const task = await createProcessingTaskApi(processForm.taskId, {
      outputWidth: processForm.outputWidth,
      outputHeight: processForm.outputHeight,
      targetCharacterHeightRatio: processForm.targetCharacterHeightRatio,
      anchorMode: processForm.anchorMode,
      backgroundMode: processForm.backgroundMode,
      outputFormat: processForm.outputFormat,
      defaultFps: processForm.defaultFps,
    });
    ElMessage.success("处理任务已创建");
    processDialogVisible.value = false;
    if (task?.id) {
      router.push({
        name: "creativeWorkshopPetProcessing",
        params: { processingTaskId: String(task.id) },
      });
    }
  } catch (err: any) {
    processError.value = err?.message || "创建处理任务失败";
  } finally {
    processingId.value = null;
  }
}

async function onExpandChange(row: TaskItem, expanded: TaskItem[]) {
  const isExpanded = expanded.some((r) => r.id === row.id);
  const idStr = String(row.id);
  if (isExpanded) {
    if (!expandedRowKeys.value.includes(idStr)) {
      expandedRowKeys.value = [...expandedRowKeys.value, idStr];
    }
    if (!detailMap[idStr]) {
      await loadDetail(row.id);
    }
    if (!referenceUrls[idStr]) {
      await loadReference(row.id);
    }
    await loadActionFrames(row.id);
    const status = detailMap[idStr]?.status;
    if (!isTerminalStatus(status)) {
      trackedTaskId.value = row.id;
      await startTracking(row.id);
    }
  } else {
    expandedRowKeys.value = expandedRowKeys.value.filter((k) => k !== idStr);
    if (trackedTaskId.value === row.id) {
      stopTracking();
      trackedTaskId.value = null;
    }
  }
}

async function loadDetail(taskId: string | number) {
  detailLoadingId.value = taskId;
  try {
    const data = await get<TaskDetail>(
      `/api/desktop-pets/generation-tasks/${taskId}`,
    );
    detailMap[String(taskId)] = data || { id: taskId, actions: [] };
    if (data && canProcess(data.status)) {
      await fetchTaskPackages(taskId);
    }
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

async function loadActionFrames(taskId: string | number) {
  const detail = detailMap[String(taskId)];
  if (!detail?.actions) return;
  const tasks: Promise<void>[] = [];
  for (const action of detail.actions) {
    if (!shouldShowFrames(action)) continue;
    const total = Number(action.frameTotal ?? action.frameSucceeded ?? 0) || 0;
    for (let i = 0; i < total; i++) {
      const key = frameKey(taskId, action, i);
      if (frameImageUrls[key]) continue;
      tasks.push(loadFrame(taskId, action.actionKey, i, key));
    }
  }
  await Promise.all(tasks);
}

async function loadFrame(
  taskId: string | number,
  actionKey: string,
  idx: number,
  key: string,
) {
  try {
    const res = await apiClient.get(
      `/api/desktop-pets/generation-tasks/${taskId}/actions/${actionKey}/frames/${idx}/image`,
      { responseType: "blob" },
    );
    const blob = res.data as Blob;
    if (blob && blob.size > 0) {
      frameImageUrls[key] = URL.createObjectURL(blob);
    }
  } catch {
    // ignore missing frame
  }
}

function confirmCancel(row: TaskItem) {
  ElMessageBox.confirm(
    "取消将停止未开始的动作,已产生的模型调用与已生成素材不会删除,可在任务详情继续查看。",
    "确认取消任务",
    {
      confirmButtonText: "确认取消",
      cancelButtonText: "再想想",
      type: "warning",
    },
  )
    .then(async () => {
      cancellingId.value = row.id;
      try {
        await post(`/api/desktop-pets/generation-tasks/${row.id}/cancel`);
        ElMessage.success("已请求取消任务");
        await loadDetail(row.id);
        const status = detailMap[String(row.id)]?.status;
        if (!isTerminalStatus(status)) {
          trackedTaskId.value = row.id;
          await startTracking(row.id);
        } else if (trackedTaskId.value === row.id) {
          stopTracking();
          trackedTaskId.value = null;
        }
      } catch (err: any) {
        ElMessage.error(err?.message || "取消任务失败");
      } finally {
        cancellingId.value = null;
      }
    })
    .catch(() => {});
}

async function startTask(row: TaskItem) {
  startingId.value = row.id;
  try {
    await post(`/api/desktop-pets/generation-tasks/${row.id}/start`);
    ElMessage.success("已开始生成");
    await loadDetail(row.id);
    const status = detailMap[String(row.id)]?.status;
    if (!isTerminalStatus(status)) {
      trackedTaskId.value = row.id;
      await startTracking(row.id);
    }
  } catch (err: any) {
    ElMessage.error(err?.message || "开始生成失败");
  } finally {
    startingId.value = null;
  }
}

function confirmRetry(row: TaskItem, action: TaskAction) {
  const key = actionKey(action);
  const succeeded =
    action.status === "succeeded" ||
    action.status === "success" ||
    action.status === "completed";
  const message = succeeded
    ? "重试已成功的动作会产生新的模型调用和费用,旧版本素材将保留不覆盖。是否继续?"
    : "将重新生成该动作,会产生新的模型调用。是否继续?";
  ElMessageBox.confirm(message, "确认重试动作", {
    confirmButtonText: "确认重试",
    cancelButtonText: "取消",
    type: succeeded ? "warning" : "info",
  })
    .then(async () => {
      retryingKey.value = key;
      try {
        await post(
          `/api/desktop-pets/generation-tasks/${row.id}/actions/${action.actionKey}/retry`,
        );
        ElMessage.success("已请求重试动作");
        await loadDetail(row.id);
        const status = detailMap[String(row.id)]?.status;
        if (!isTerminalStatus(status)) {
          trackedTaskId.value = row.id;
          await startTracking(row.id);
        }
      } catch (err: any) {
        ElMessage.error(err?.message || "重试动作失败");
      } finally {
        retryingKey.value = null;
      }
    })
    .catch(() => {});
}

function goCreate() {
  router.push("/creative-workshop/pet/create");
}

async function expandTaskFromQuery() {
  const taskId = route.query.taskId;
  if (!taskId) return;
  const idStr = String(taskId);
  let row = items.value.find((r) => String(r.id) === idStr);
  if (!row) {
    try {
      const data = await get<TaskDetail>(
        `/api/desktop-pets/generation-tasks/${idStr}`,
      );
      row = {
        id: idStr,
        name: data?.name || "未命名任务",
        characterName: (data as any)?.characterName,
        modelName: data?.modelName,
        status: data?.status || "pending",
        currentStage: data?.currentStage,
        progress: data?.progress,
        selectedActionCount: data?.selectedActionCount,
        estimatedGenerationCount: data?.estimatedGenerationCount,
        createdAt: data?.createdAt,
      } as TaskItem;
      items.value = [row, ...items.value];
    } catch {
      return;
    }
  }
  if (!expandedRowKeys.value.includes(idStr)) {
    expandedRowKeys.value = [...expandedRowKeys.value, idStr];
  }
  await loadDetail(row.id);
  await loadReference(row.id);
  await loadActionFrames(row.id);
  const status = detailMap[idStr]?.status;
  if (!isTerminalStatus(status)) {
    trackedTaskId.value = row.id;
    await startTracking(row.id);
  }
}

onMounted(async () => {
  await loadList();
  await expandTaskFromQuery();
});

onUnmounted(() => {
  stopTracking();
  Object.values(referenceUrls).forEach((url) => {
    if (url) URL.revokeObjectURL(url);
  });
  Object.values(frameImageUrls).forEach((url) => {
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
.sse-tip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.sse-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--el-color-success);
  box-shadow: 0 0 0 0 rgba(82, 196, 26, 0.6);
  animation: sse-pulse 1.6s infinite;
}
@keyframes sse-pulse {
  0% {
    box-shadow: 0 0 0 0 rgba(82, 196, 26, 0.6);
  }
  70% {
    box-shadow: 0 0 0 6px rgba(82, 196, 26, 0);
  }
  100% {
    box-shadow: 0 0 0 0 rgba(82, 196, 26, 0);
  }
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
.progress-cell {
  display: flex;
  align-items: center;
  gap: 10px;
}
.progress-cell :deep(.el-progress) {
  flex: 1;
}
.progress-text {
  color: var(--el-text-color-secondary);
  font-size: 12px;
  white-space: nowrap;
}
.error-text {
  color: var(--el-color-danger);
  font-size: 13px;
  word-break: break-all;
}
.warn-text {
  color: var(--el-color-warning);
}
.detail-actions {
  display: flex;
  gap: 8px;
  margin-top: 12px;
  flex-wrap: wrap;
}
.detail-block {
  margin-top: 14px;
}
.detail-title {
  margin-bottom: 8px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
.action-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.action-item {
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  padding: 10px 12px;
  background: var(--ac-color-surface);
}
.action-item-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
.action-item-name {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.action-item-meta {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.action-error {
  margin-top: 6px;
  color: var(--el-color-danger);
  font-size: 12px;
  word-break: break-all;
}
.action-item-foot {
  display: flex;
  justify-content: flex-end;
  margin-top: 8px;
}
.frame-preview {
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px dashed var(--el-border-color-lighter);
}
.frame-preview-label {
  margin-bottom: 8px;
  color: var(--el-color-warning);
  font-size: 12px;
  font-weight: 500;
}
.frame-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(72px, 1fr));
  gap: 8px;
}
.frame-thumb {
  width: 100%;
  aspect-ratio: 1 / 1;
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
  overflow: hidden;
  background: var(--ac-color-bg-secondary, #f5f7fa);
}
.frame-thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.frame-thumb-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
  color: var(--el-text-color-secondary);
  font-size: 12px;
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

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
  <main class="pet-processing-review" v-loading="loading">
    <ExtensionPageHeader
      title="处理结果审核"
      description="审核桌宠动作处理结果并打包资源"
      grandparent-title="创意工坊"
      grandparent-path="/creative-workshop"
      parent-title="桌宠"
      parent-path="/creative-workshop/pet"
    >
      <template #actions>
        <el-button :icon="Back" @click="goBack">返回</el-button>
        <el-button
          v-if="canRegenerate"
          type="warning"
          plain
          :icon="RefreshLeft"
          @click="goRegenerate"
          >返回重新生成</el-button
        >
        <el-button
          v-if="canCancel"
          type="danger"
          plain
          :loading="cancelling"
          @click="confirmCancel"
          >取消处理</el-button
        >
        <el-button
          v-if="canPackage"
          type="primary"
          :loading="packaging"
          @click="openPackageDialog"
          >打包资源</el-button
        >
        <el-button
          v-if="canInstallPackage"
          type="success"
          :icon="Download"
          @click="openInstallDialog"
          >安装到桌宠</el-button
        >
      </template>
    </ExtensionPageHeader>

    <el-card shadow="never" class="summary-card">
      <div class="summary-grid">
        <div class="summary-item">
          <div class="summary-label">桌宠名称</div>
          <div class="summary-value">{{ petName || "—" }}</div>
        </div>
        <div class="summary-item">
          <div class="summary-label">处理版本</div>
          <div class="summary-value">
            v{{ processingTask?.processingVersion ?? "—" }}
          </div>
        </div>
        <div class="summary-item">
          <div class="summary-label">默认待机动作</div>
          <div class="summary-value">{{ defaultIdleLabel }}</div>
        </div>
        <div class="summary-item">
          <div class="summary-label">动作统计</div>
          <div class="summary-value">
            <span class="stat-success">
              成功 {{ qualitySummary.succeededActions }}
            </span>
            <span class="stat-divider">/</span>
            <span class="stat-warning">
              警告 {{ qualitySummary.warningActions }}
            </span>
            <span class="stat-divider">/</span>
            <span class="stat-danger">
              失败 {{ qualitySummary.failedActions }}
            </span>
            <span class="stat-divider">/</span>
            <span class="stat-total">
              共 {{ qualitySummary.totalActions }}
            </span>
          </div>
        </div>
      </div>
    </el-card>

    <el-card shadow="never" class="progress-card">
      <div class="progress-row">
        <div class="progress-meta">
          <span class="progress-stage">
            当前阶段：{{ stageLabel(processingTask?.currentStage) }}
          </span>
          <el-tag :type="statusTagType(processingTask?.status)" size="small">
            {{ statusLabel(processingTask?.status) }}
          </el-tag>
          <span v-if="isConnected" class="sse-tip">
            <span class="sse-dot"></span>
            实时同步中
          </span>
        </div>
        <el-progress
          :percentage="Number(processingTask?.progress || 0)"
          :status="progressStatus(processingTask?.status)"
          :stroke-width="14"
        />
      </div>
      <div v-if="processingTask?.errorMessage" class="error-text">
        {{ processingTask.errorMessage }}
      </div>
    </el-card>

    <el-card shadow="never" class="actions-card">
      <template #header>
        <div class="card-header">
          <span>动作列表</span>
          <el-button :icon="Refresh" link @click="refresh">刷新</el-button>
        </div>
      </template>
      <el-empty
        v-if="!actions.length"
        description="暂无动作数据"
        :image-size="80"
      />
      <div v-else class="action-grid">
        <div
          v-for="action in actions"
          :key="action.actionKey"
          class="action-card"
          :class="{
            excluded: action.excluded,
            [`quality-${action.qualityLevel}`]: true,
          }"
        >
          <div class="action-card-head">
            <div class="action-title">
              <strong>{{ action.actionName || action.actionKey }}</strong>
              <el-tag size="small" :type="qualityTagType(action.qualityLevel)">
                {{ qualityLabel(action.qualityLevel) }}
              </el-tag>
              <el-tag
                v-if="action.excluded"
                size="small"
                type="info"
              >已排除</el-tag>
            </div>
            <el-tag size="small" :type="actionTagType(action.status)">
              {{ actionStatusLabel(action.status) }}
            </el-tag>
          </div>
          <div class="action-card-body">
            <div class="preview-box">
              <img
                v-if="previewUrls[action.actionKey]"
                :src="previewUrls[action.actionKey]"
                :alt="action.actionName"
              />
              <div v-else class="preview-placeholder">
                <el-icon><Picture /></el-icon>
                <span>暂无预览</span>
              </div>
            </div>
            <div class="action-meta">
              <div class="meta-row">
                <span class="meta-label">动作 Key</span>
                <span class="meta-value">{{ action.actionKey }}</span>
              </div>
              <div class="meta-row">
                <span class="meta-label">当前 Attempt</span>
                <span class="meta-value">#{{ action.sourceAttempt || 1 }}</span>
              </div>
              <div class="meta-row">
                <span class="meta-label">进度</span>
                <el-progress
                  :percentage="Number(action.progress || 0)"
                  :status="actionProgressStatus(action.status)"
                  :stroke-width="8"
                />
              </div>
              <div v-if="action.qualityFlags?.length" class="meta-row">
                <span class="meta-label">质量标记</span>
                <div class="meta-tags">
                  <el-tag
                    v-for="flag in action.qualityFlags"
                    :key="flag"
                    size="small"
                    type="warning"
                  >{{ flag }}</el-tag>
                </div>
              </div>
            </div>
          </div>
          <div class="action-card-foot">
            <el-button
              size="small"
              :icon="View"
              :disabled="!canViewFrames(action)"
              @click="openFrameDialog(action)"
            >查看帧</el-button>
            <el-button
              size="small"
              type="primary"
              plain
              :icon="Edit"
              @click="goActionEditor(action)"
            >编辑动作</el-button>
            <el-button
              size="small"
              :icon="Switch"
              :disabled="!canSwitchAttempt(action)"
              @click="openAttemptDialog(action)"
            >切换 Attempt</el-button>
            <el-button
              size="small"
              type="warning"
              plain
              :icon="RefreshRight"
              :loading="retryingKey === action.actionKey"
              :disabled="!canRetry(action)"
              @click="confirmRetry(action)"
            >重新处理</el-button>
            <el-button
              size="small"
              type="danger"
              plain
              :icon="Close"
              :loading="excludingKey === action.actionKey"
              :disabled="!canExclude(action)"
              @click="confirmExclude(action)"
            >排除动作</el-button>
          </div>
        </div>
      </div>
    </el-card>

    <el-dialog
      v-model="frameDialogVisible"
      :title="frameDialogTitle"
      width="80%"
      top="6vh"
      destroy-on-close
      @close="onFrameDialogClose"
    >
      <div v-loading="frameLoading" class="frame-dialog-body">
        <el-empty
          v-if="!frameLoading && !frameRows.length"
          description="暂无帧数据"
        />
        <div v-else class="frame-list">
          <div
            v-for="row in frameRows"
            :key="row.index"
            class="frame-row"
          >
            <div class="frame-index">第 {{ row.index + 1 }} 帧</div>
            <div class="frame-images">
              <div class="frame-cell">
                <div class="frame-cell-label">原始帧</div>
                <img
                  v-if="row.source"
                  :src="row.source"
                  :alt="`原始帧 ${row.index + 1}`"
                />
                <div v-else class="frame-empty">无</div>
              </div>
              <div class="frame-cell">
                <div class="frame-cell-label">处理后帧</div>
                <img
                  v-if="row.processed"
                  :src="row.processed"
                  :alt="`处理后帧 ${row.index + 1}`"
                />
                <div v-else class="frame-empty">无</div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </el-dialog>

    <el-dialog
      v-model="attemptDialogVisible"
      title="切换 Attempt"
      width="420px"
      destroy-on-close
    >
      <div class="attempt-dialog-body">
        <p class="attempt-tip">
          切换 Attempt 会重新基于指定尝试的源素材进行处理，会产生新的处理任务调用。
        </p>
        <el-form label-width="120px">
          <el-form-item label="动作">
            <span>{{ attemptForm.actionName }}</span>
          </el-form-item>
          <el-form-item label="当前 Attempt">
            <span>#{{ attemptForm.currentAttempt }}</span>
          </el-form-item>
          <el-form-item label="目标 Attempt">
            <el-input-number
              v-model="attemptForm.targetAttempt"
              :min="1"
              :max="10"
              controls-position="right"
            />
          </el-form-item>
        </el-form>
      </div>
      <template #footer>
        <el-button @click="attemptDialogVisible = false">取消</el-button>
        <el-button
          type="primary"
          :loading="switchingAttempt"
          @click="confirmSwitchAttempt"
          >确认切换</el-button
        >
      </template>
    </el-dialog>

    <el-dialog
      v-model="packageDialogVisible"
      title="打包资源"
      width="520px"
      destroy-on-close
    >
      <div class="package-dialog-body">
        <el-form label-width="140px">
          <el-form-item label="默认动作">
            <el-select
              v-model="packageForm.defaultAction"
              placeholder="请选择默认动作"
              style="width: 100%"
            >
              <el-option
                v-for="action in packageableActions"
                :key="action.actionKey"
                :label="action.actionName || action.actionKey"
                :value="action.actionKey"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="用户默认动作">
            <el-select
              v-model="packageForm.userDefaultAction"
              placeholder="可选，默认与默认动作一致"
              clearable
              style="width: 100%"
            >
              <el-option
                v-for="action in packageableActions"
                :key="action.actionKey"
                :label="action.actionName || action.actionKey"
                :value="action.actionKey"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="包含动作">
            <el-checkbox-group v-model="packageForm.includedActions">
              <el-checkbox
                v-for="action in packageableActions"
                :key="action.actionKey"
                :label="action.actionKey"
              >
                {{ action.actionName || action.actionKey }}
              </el-checkbox>
            </el-checkbox-group>
          </el-form-item>
        </el-form>
        <el-alert
          v-if="packageError"
          :title="packageError"
          type="error"
          :closable="false"
          show-icon
        />
        <el-alert
          v-if="packageResult"
          :title="`资源包已生成：${packageResult.packageId}`"
          type="success"
          :closable="false"
          show-icon
        >
          <div class="package-result">
            <div>状态：{{ packageResult.status }}</div>
            <div>哈希：{{ packageResult.packageHash }}</div>
          </div>
        </el-alert>
      </div>
      <template #footer>
        <el-button @click="packageDialogVisible = false">关闭</el-button>
        <el-button
          type="primary"
          :loading="packaging"
          :disabled="!canSubmitPackage"
          @click="submitPackage"
          >生成资源包</el-button
        >
      </template>
    </el-dialog>

    <el-dialog
      v-model="installDialogVisible"
      title="安装到桌宠"
      width="460px"
      destroy-on-close
    >
      <div class="install-dialog-body">
        <el-form label-width="120px">
          <el-form-item label="资源包 ID">
            <span class="install-package-id">{{ installForm.releaseId }}</span>
          </el-form-item>
          <el-form-item label="绑定角色">
            <el-select
              v-model="installForm.characterId"
              placeholder="请选择绑定的 Amitia 角色"
              style="width: 100%"
              filterable
            >
              <el-option
                v-for="c in installCharacters"
                :key="c.id"
                :label="c.name"
                :value="c.id"
              />
            </el-select>
          </el-form-item>
        </el-form>
        <el-alert
          type="info"
          :closable="false"
          show-icon
          title="安装会复制资源包到桌宠运行时目录，不会修改原资源包"
        />
        <el-alert
          v-if="installError"
          :title="installError"
          type="error"
          :closable="false"
          show-icon
        />
      </div>
      <template #footer>
        <el-button @click="installDialogVisible = false">取消</el-button>
        <el-button
          type="primary"
          :loading="installSubmitting"
          @click="submitInstall"
          >确认安装</el-button
        >
      </template>
    </el-dialog>
  </main>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  Back,
  Refresh,
  RefreshLeft,
  RefreshRight,
  View,
  Switch,
  Close,
  Picture,
  Download,
  Edit,
} from "@element-plus/icons-vue";
import ExtensionPageHeader from "../extensions/components/ExtensionPageHeader.vue";
import { useApi } from "../../composables/useApi";
import {
  useProcessingTask,
  isProcessingTerminalStatus,
  type ProcessingActionInfo,
  type CreatePackageResponse,
} from "../../composables/useProcessingTask";
import { useDesktopPetInstallations } from "../../composables/useDesktopPetInstallations";

const route = useRoute();
const router = useRouter();
const { get } = useApi();

const processingTaskId = computed(() =>
  String(route.params.processingTaskId || ""),
);

const {
  processingTask,
  actions,
  qualitySummary,
  previewPaths,
  isConnected,
  loading,
  getProcessingTask,
  cancelProcessingTask,
  retryProcessingAction,
  createPackage,
  switchAttempt,
  excludeAction,
  subscribeProcessingEvents,
  stop,
  refresh,
  fetchActionPreview,
  fetchProcessedFrame,
  fetchSourceFrame,
  revokeObjectUrls,
} = useProcessingTask();

const { install: installPet } = useDesktopPetInstallations();

interface GenerationTaskDetail {
  id: string | number;
  name?: string;
  characterId?: string | number;
  characterName?: string;
  status?: string;
  actions?: Array<{
    actionKey: string;
    actionName?: string;
    supportsDefaultIdle?: boolean;
    attemptNumber?: number;
    status?: string;
  }>;
}

const generationTaskDetail = ref<GenerationTaskDetail | null>(null);
const previewUrls = reactive<Record<string, string>>({});
const cancelling = ref(false);
const packaging = ref(false);
const retryingKey = ref<string | null>(null);
const excludingKey = ref<string | null>(null);

const frameDialogVisible = ref(false);
const frameDialogTitle = ref("");
const frameLoading = ref(false);
interface FrameRow {
  index: number;
  source: string;
  processed: string;
}
const frameRows = ref<FrameRow[]>([]);

const attemptDialogVisible = ref(false);
const switchingAttempt = ref(false);
const attemptForm = reactive({
  actionKey: "",
  actionName: "",
  currentAttempt: 1,
  targetAttempt: 1,
});

const packageDialogVisible = ref(false);
const packageForm = reactive({
  defaultAction: "",
  userDefaultAction: "",
  includedActions: [] as string[],
});
const packageError = ref("");
const packageResult = ref<CreatePackageResponse | null>(null);

interface InstallCharacterOption {
  id: string | number;
  name: string;
  status?: string;
  isActive?: number | boolean;
  isDefault?: number | boolean;
}

const installDialogVisible = ref(false);
const installSubmitting = ref(false);
const installError = ref("");
const installCharacters = ref<InstallCharacterOption[]>([]);
const installForm = reactive({
  petId: "",
  releaseId: "",
  characterId: "" as string | number,
});

const canInstallPackage = computed(
  () => !!packageResult.value && packageResult.value.status === "published",
);

const statusMeta: Record<string, { label: string; type: string }> = {
  pending: { label: "等待中", type: "info" },
  queued: { label: "排队中", type: "info" },
  processing: { label: "处理中", type: "warning" },
  succeeded: { label: "已完成", type: "success" },
  partially_succeeded: { label: "部分成功", type: "warning" },
  failed: { label: "失败", type: "danger" },
  cancelled: { label: "已取消", type: "info" },
};

const stageMeta: Record<string, string> = {
  queued: "排队中",
  validating_sources: "校验源素材",
  background_removal: "背景去除",
  compositing: "合成处理",
  finalizing: "收尾处理",
  completed: "已完成",
  failed: "失败",
  cancelled: "已取消",
};

const actionStatusMeta: Record<string, { label: string; type: string }> = {
  pending: { label: "等待中", type: "info" },
  processing: { label: "处理中", type: "warning" },
  succeeded: { label: "已完成", type: "success" },
  failed: { label: "失败", type: "danger" },
  cancelled: { label: "已取消", type: "info" },
};

const qualityMeta: Record<string, { label: string; type: string }> = {
  normal: { label: "正常", type: "success" },
  warning: { label: "警告", type: "warning" },
  failed: { label: "失败", type: "danger" },
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

function qualityLabel(level?: string): string {
  if (!level) return "—";
  return qualityMeta[level]?.label || level;
}

function qualityTagType(level?: string): any {
  if (!level) return "info";
  const t = qualityMeta[level]?.type;
  if (t === "success") return "success";
  if (t === "warning") return "warning";
  if (t === "danger") return "danger";
  return "info";
}

function progressStatus(
  status?: string,
): "" | "success" | "exception" {
  if (status === "succeeded") return "success";
  if (
    status === "failed" ||
    status === "cancelled"
  )
    return "exception";
  return "";
}

function actionProgressStatus(
  status?: string,
): "" | "success" | "exception" {
  if (status === "succeeded") return "success";
  if (status === "failed" || status === "cancelled") return "exception";
  return "";
}

const petName = computed(() => {
  return (
    generationTaskDetail.value?.name ||
    generationTaskDetail.value?.characterName ||
    ""
  );
});

const defaultIdleActionFromGen = computed(() => {
  const list = generationTaskDetail.value?.actions || [];
  const found = list.find((a) => a.supportsDefaultIdle);
  return found || null;
});

const defaultIdleLabel = computed(() => {
  const action = defaultIdleActionFromGen.value;
  if (!action) return "—";
  return action.actionName || action.actionKey;
});

const canCancel = computed(() => {
  const status = processingTask.value?.status;
  return status === "queued" || status === "processing" || status === "pending";
});

const canPackage = computed(() => {
  const status = processingTask.value?.status;
  return (
    status === "succeeded" ||
    status === "partially_succeeded"
  );
});

const canRegenerate = computed(() => {
  const status = processingTask.value?.status;
  return (
    status === "failed" ||
    status === "cancelled" ||
    status === "succeeded" ||
    status === "partially_succeeded"
  );
});

const packageableActions = computed(() =>
  actions.value.filter(
    (a) =>
      !a.excluded &&
      (a.status === "succeeded" || a.status === "processing"),
  ),
);

const canSubmitPackage = computed(() => {
  return (
    !!packageForm.defaultAction &&
    packageForm.includedActions.length > 0 &&
    packageForm.includedActions.includes(packageForm.defaultAction)
  );
});

function canRetry(action: ProcessingActionInfo): boolean {
  if (action.excluded) return false;
  const status = action.status;
  return status === "failed" || status === "succeeded";
}

function canExclude(action: ProcessingActionInfo): boolean {
  if (action.excluded) return false;
  const idle = defaultIdleActionFromGen.value;
  if (idle && idle.actionKey === action.actionKey) {
    const idleActions = (generationTaskDetail.value?.actions || []).filter(
      (a) => a.supportsDefaultIdle,
    );
    const includedIdleActions = actions.value.filter(
      (a) =>
        !a.excluded &&
        idleActions.some((g) => g.actionKey === a.actionKey),
    );
    if (includedIdleActions.length <= 1) {
      return false;
    }
  }
  return true;
}

function canSwitchAttempt(action: ProcessingActionInfo): boolean {
  if (action.excluded) return false;
  return true;
}

function canViewFrames(action: ProcessingActionInfo): boolean {
  if (action.excluded) return false;
  return action.status === "succeeded" || action.status === "failed";
}

async function loadGenerationTaskDetail(generationTaskId: string) {
  try {
    const data = await get<GenerationTaskDetail>(
      `/api/desktop-pets/generation-tasks/${generationTaskId}`,
    );
    generationTaskDetail.value = data || null;
  } catch (err: any) {
    ElMessage.error(err?.message || "加载生成任务详情失败");
  }
}

async function loadPreviews() {
  const keys = Object.keys(previewPaths.value || {});
  if (!keys.length) return;
  const tasks: Promise<void>[] = [];
  for (const key of keys) {
    if (previewUrls[key]) continue;
    tasks.push(
      (async () => {
        const url = await fetchActionPreview(processingTaskId.value, key);
        if (url) previewUrls[key] = url;
      })(),
    );
  }
  await Promise.all(tasks);
}

watch(previewPaths, () => {
  void loadPreviews();
});

watch(
  () => actions.value,
  (newActions) => {
    const previewPathKeys = Object.keys(previewPaths.value || {});
    if (!previewPathKeys.length) {
      for (const action of newActions) {
        if (!previewUrls[action.actionKey]) {
          void (async () => {
            const url = await fetchActionPreview(
              processingTaskId.value,
              action.actionKey,
            );
            if (url) previewUrls[action.actionKey] = url;
          })();
        }
      }
    }
  },
  { deep: false },
);

async function initLoad() {
  if (!processingTaskId.value) {
    ElMessage.error("缺少处理任务 ID");
    return;
  }
  loading.value = true;
  try {
    await getProcessingTask(processingTaskId.value);
    if (processingTask.value?.generationTaskId) {
      await loadGenerationTaskDetail(processingTask.value.generationTaskId);
    }
    await loadPreviews();
    startSubscribe();
  } finally {
    loading.value = false;
  }
}

function startSubscribe() {
  if (!processingTaskId.value) return;
  const status = processingTask.value?.status;
  if (isProcessingTerminalStatus(status)) return;
  subscribeProcessingEvents(processingTaskId.value, {
    onProgress: () => {
      void refresh();
    },
    onAction: () => {
      void refresh();
    },
    onActionProgress: () => {
      void refresh();
    },
    onCompleted: () => {
      void refresh();
    },
  });
}

function goBack() {
  router.push("/creative-workshop/pet/tasks");
}

function goRegenerate() {
  const generationTaskId = processingTask.value?.generationTaskId;
  router.push({
    path: "/creative-workshop/pet/tasks",
    query: generationTaskId ? { taskId: generationTaskId } : {},
  });
}

function confirmCancel() {
  ElMessageBox.confirm(
    "取消将停止未完成的处理动作,已处理完成的素材不会删除。是否继续?",
    "确认取消处理",
    {
      confirmButtonText: "确认取消",
      cancelButtonText: "再想想",
      type: "warning",
    },
  )
    .then(async () => {
      cancelling.value = true;
      try {
        await cancelProcessingTask(processingTaskId.value);
        ElMessage.success("已请求取消处理");
        await refresh();
        stop();
      } catch (err: any) {
        ElMessage.error(err?.message || "取消处理失败");
      } finally {
        cancelling.value = false;
      }
    })
    .catch(() => {});
}

function confirmRetry(action: ProcessingActionInfo) {
  const succeeded = action.status === "succeeded";
  const message = succeeded
    ? "重试已成功的动作会重新处理并覆盖当前结果。是否继续?"
    : "将重新处理该动作。是否继续?";
  ElMessageBox.confirm(message, "确认重新处理", {
    confirmButtonText: "确认重试",
    cancelButtonText: "取消",
    type: succeeded ? "warning" : "info",
  })
    .then(async () => {
      retryingKey.value = action.actionKey;
      try {
        await retryProcessingAction(
          processingTaskId.value,
          action.actionKey,
        );
        ElMessage.success("已请求重新处理");
        await refresh();
        startSubscribe();
      } catch (err: any) {
        ElMessage.error(err?.message || "重新处理失败");
      } finally {
        retryingKey.value = null;
      }
    })
    .catch(() => {});
}

function confirmExclude(action: ProcessingActionInfo) {
  if (!canExclude(action)) {
    ElMessage.warning("不能排除唯一的默认待机动作");
    return;
  }
  ElMessageBox.confirm(
    `排除动作 ${action.actionName || action.actionKey} 后,该动作不会进入资源包。是否继续?`,
    "确认排除动作",
    {
      confirmButtonText: "确认排除",
      cancelButtonText: "取消",
      type: "warning",
    },
  )
    .then(async () => {
      excludingKey.value = action.actionKey;
      try {
        await excludeAction(processingTaskId.value, action.actionKey);
        ElMessage.success("已排除动作");
        await refresh();
      } catch (err: any) {
        ElMessage.error(err?.message || "排除动作失败");
      } finally {
        excludingKey.value = null;
      }
    })
    .catch(() => {});
}

function openAttemptDialog(action: ProcessingActionInfo) {
  attemptForm.actionKey = action.actionKey;
  attemptForm.actionName = action.actionName || action.actionKey;
  attemptForm.currentAttempt = action.sourceAttempt || 1;
  attemptForm.targetAttempt = Math.max(1, (action.sourceAttempt || 1) + 1);
  attemptDialogVisible.value = true;
}

function confirmSwitchAttempt() {
  if (attemptForm.targetAttempt < 1) {
    ElMessage.warning("目标 Attempt 必须 ≥ 1");
    return;
  }
  ElMessageBox.confirm(
    `将动作 ${attemptForm.actionName} 切换到 Attempt #${attemptForm.targetAttempt},会基于该 Attempt 重新处理。是否继续?`,
    "确认切换 Attempt",
    {
      confirmButtonText: "确认切换",
      cancelButtonText: "取消",
      type: "warning",
    },
  )
    .then(async () => {
      switchingAttempt.value = true;
      try {
        await switchAttempt(
          processingTaskId.value,
          attemptForm.actionKey,
          attemptForm.targetAttempt,
        );
        ElMessage.success("已切换 Attempt");
        attemptDialogVisible.value = false;
        await refresh();
        startSubscribe();
      } catch (err: any) {
        ElMessage.error(err?.message || "切换 Attempt 失败");
      } finally {
        switchingAttempt.value = false;
      }
    })
    .catch(() => {});
}

function goActionEditor(action: ProcessingActionInfo) {
  router.push({
    name: "creativeWorkshopActionEditor",
    params: {
      processingTaskId: processingTaskId.value,
      actionKey: action.actionKey,
    },
  });
}

async function openFrameDialog(action: ProcessingActionInfo) {
  frameDialogTitle.value = `帧查看 - ${action.actionName || action.actionKey}`;
  frameDialogVisible.value = true;
  frameLoading.value = true;
  frameRows.value = [];
  try {
    const genAction = (generationTaskDetail.value?.actions || []).find(
      (a) => a.actionKey === action.actionKey,
    );
    const total =
      Number(
        (genAction as any)?.frameTotal ??
          (genAction as any)?.attemptNumber ??
          0,
      ) || 8;
    const rows: FrameRow[] = [];
    const fetchTasks: Promise<void>[] = [];
    for (let i = 0; i < total; i++) {
      const row: FrameRow = { index: i, source: "", processed: "" };
      rows.push(row);
      fetchTasks.push(
        (async () => {
          const [src, processed] = await Promise.all([
            fetchSourceFrame(processingTaskId.value, action.actionKey, i),
            fetchProcessedFrame(processingTaskId.value, action.actionKey, i),
          ]);
          row.source = src;
          row.processed = processed;
        })(),
      );
    }
    frameRows.value = rows;
    await Promise.all(fetchTasks);
  } catch (err: any) {
    ElMessage.error(err?.message || "加载帧数据失败");
  } finally {
    frameLoading.value = false;
  }
}

function onFrameDialogClose() {
  frameRows.value = [];
}

function openPackageDialog() {
  packageError.value = "";
  packageResult.value = null;
  const idleAction = defaultIdleActionFromGen.value;
  const defaultIdleInActions = idleAction
    ? actions.value.find((a) => a.actionKey === idleAction.actionKey)
    : null;
  const defaultAction =
    defaultIdleInActions && !defaultIdleInActions.excluded
      ? defaultIdleInActions.actionKey
      : packageableActions.value[0]?.actionKey || "";
  packageForm.defaultAction = defaultAction;
  packageForm.userDefaultAction = "";
  packageForm.includedActions = packageableActions.value.map(
    (a) => a.actionKey,
  );
  packageDialogVisible.value = true;
}

async function submitPackage() {
  if (!packageForm.defaultAction) {
    packageError.value = "请选择默认动作";
    return;
  }
  if (!packageForm.includedActions.includes(packageForm.defaultAction)) {
    packageError.value = "默认动作必须在包含动作列表中";
    return;
  }
  packageError.value = "";
  packaging.value = true;
  try {
    const result = await createPackage(processingTaskId.value, {
      defaultAction: packageForm.defaultAction,
      includedActions: packageForm.includedActions,
      userDefaultAction: packageForm.userDefaultAction || undefined,
    });
    packageResult.value = result;
    ElMessage.success("资源包已生成");
  } catch (err: any) {
    packageError.value = err?.message || "打包失败";
  } finally {
    packaging.value = false;
  }
}

async function loadInstallCharacters() {
  try {
    const list =
      (await get<InstallCharacterOption[]>("/api/characters")) || [];
    installCharacters.value = list.filter(
      (c) =>
        c.status === "enabled" ||
        c.isActive === true ||
        c.isActive === 1,
    );
    if (!installForm.characterId && installCharacters.value.length) {
      const defaultChar =
        installCharacters.value.find((c) => c.isDefault) ||
        installCharacters.value.find((c) => c.isActive) ||
        installCharacters.value[0];
      installForm.characterId = defaultChar?.id || "";
    }
  } catch {
    installCharacters.value = [];
  }
}

function openInstallDialog() {
  if (!packageResult.value?.releaseId) {
    ElMessage.warning("请先生成资源包");
    return;
  }
  installForm.petId = packageResult.value.petId;
  installForm.releaseId = packageResult.value.releaseId;
  installForm.characterId =
    generationTaskDetail.value?.characterId ||
    installCharacters.value[0]?.id ||
    "";
  installError.value = "";
  installDialogVisible.value = true;
  void loadInstallCharacters();
}

async function submitInstall() {
  if (!installForm.petId || !installForm.releaseId) {
    installError.value = "缺少资源包信息";
    return;
  }
  if (!installForm.characterId) {
    installError.value = "请选择绑定的 Amitia 角色";
    return;
  }
  installError.value = "";
  installSubmitting.value = true;
  try {
    await installPet(
      installForm.petId,
      installForm.releaseId,
      String(installForm.characterId),
    );
    installDialogVisible.value = false;
    ElMessage.success("桌宠已安装，正在跳转到安装管理");
    router.push("/creative-workshop/pet/installations");
  } catch (err: any) {
    installError.value = err?.message || "安装失败";
  } finally {
    installSubmitting.value = false;
  }
}

onMounted(() => {
  void initLoad();
});

onUnmounted(() => {
  stop();
  revokeObjectUrls();
});
</script>

<style scoped>
.pet-processing-review {
  height: 100%;
  overflow: auto;
  padding: 0;
}
.summary-card {
  margin-bottom: 12px;
  border: 1px solid var(--el-border-color-light);
  background: var(--ac-color-surface);
}
.summary-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 16px;
}
.summary-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.summary-label {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.summary-value {
  color: var(--console-text);
  font-size: 16px;
  font-weight: 500;
}
.stat-success {
  color: var(--el-color-success);
}
.stat-warning {
  color: var(--el-color-warning);
}
.stat-danger {
  color: var(--el-color-danger);
}
.stat-total {
  color: var(--el-text-color-secondary);
}
.stat-divider {
  color: var(--el-text-color-placeholder);
  margin: 0 4px;
}
.progress-card {
  margin-bottom: 12px;
  border: 1px solid var(--el-border-color-light);
  background: var(--ac-color-surface);
}
.progress-row {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.progress-meta {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
.progress-stage {
  color: var(--console-text);
  font-size: 13px;
}
.sse-tip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--el-color-success);
  font-size: 12px;
}
.sse-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--el-color-success);
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
.error-text {
  margin-top: 8px;
  color: var(--el-color-danger);
  font-size: 13px;
  word-break: break-all;
}
.actions-card {
  border: 1px solid var(--el-border-color-light);
  background: var(--ac-color-surface);
}
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.action-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
  gap: 12px;
}
.action-card {
  border: 1px solid var(--el-border-color-light);
  border-radius: 8px;
  padding: 12px;
  background: var(--ac-color-surface);
  display: flex;
  flex-direction: column;
  gap: 10px;
  transition: border-color 180ms ease;
}
.action-card.quality-warning {
  border-color: var(--el-color-warning-light-5);
}
.action-card.quality-failed {
  border-color: var(--el-color-danger-light-5);
}
.action-card.excluded {
  opacity: 0.6;
}
.action-card-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.action-title {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
.action-card-body {
  display: grid;
  grid-template-columns: 120px 1fr;
  gap: 12px;
}
.preview-box {
  width: 120px;
  height: 120px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
  overflow: hidden;
  background: var(--ac-color-bg-secondary, #f5f7fa);
  display: flex;
  align-items: center;
  justify-content: center;
}
.preview-box img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}
.preview-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.action-meta {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}
.meta-row {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 12px;
}
.meta-label {
  color: var(--el-text-color-secondary);
}
.meta-value {
  color: var(--console-text);
  word-break: break-all;
}
.meta-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
.action-card-foot {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  justify-content: flex-end;
}
.frame-dialog-body {
  min-height: 200px;
}
.frame-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.frame-row {
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 6px;
  padding: 8px;
  background: var(--ac-color-surface-soft, var(--ac-color-surface));
}
.frame-index {
  margin-bottom: 8px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
  font-weight: 500;
}
.frame-images {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
.frame-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.frame-cell-label {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.frame-cell img {
  width: 100%;
  height: 220px;
  object-fit: contain;
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
  background: var(--ac-color-bg-secondary, #f5f7fa);
}
.frame-empty {
  width: 100%;
  height: 220px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px dashed var(--el-border-color-light);
  border-radius: 6px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
  background: var(--ac-color-bg-secondary, #f5f7fa);
}
.attempt-dialog-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.attempt-tip {
  margin: 0;
  color: var(--el-text-color-secondary);
  font-size: 13px;
  line-height: 1.6;
}
.package-dialog-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.package-result {
  margin-top: 6px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 13px;
}
.install-dialog-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.install-package-id {
  font-family: var(--el-font-family-mono, monospace);
  font-size: 12px;
  color: var(--el-text-color-secondary);
  word-break: break-all;
}
</style>

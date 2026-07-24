<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <main class="pet-creation">
    <ExtensionPageHeader title="桌宠制作" description="创建和定制你的桌面陪伴角色" parent-title="创意工坊" parent-path="/creative-workshop" />
    <el-result icon="info" title="敬请期待" sub-title="桌宠制作功能正在开发中，敬请期待" />
  </main>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from "vue";
import { useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import ExtensionPageHeader from "../extensions/components/ExtensionPageHeader.vue";
import { Upload, Delete } from "@element-plus/icons-vue";
import type { UploadFile } from "element-plus";
import { useApi } from "../../composables/useApi";
import {
  useActionDefinitions,
  type ActionCategory,
  type ActionDefinition,
  type ActionPreset,
} from "../../composables/useActionDefinitions";

const router = useRouter();
const { get, post } = useApi();
const {
  categories,
  presets,
  loading,
  selectedKeys,
  load: loadActions,
  isSelected,
  toggle,
  toggleCategory,
  selectAllCategory,
  clearCategory,
  clearAll,
  applyPreset,
  isCategoryAllSelected,
  isCategoryPartialSelected,
  categoryActions,
  selectedCount,
  estimatedGenerationCount,
  hasDefaultIdle,
} = useActionDefinitions();

const step = ref(0);
const submitting = ref(false);
const actionError = ref("");
const createdTaskId = ref<string | number | null>(null);
const startLoading = ref(false);
const startFailed = ref(false);
const startError = ref("");

interface ModelConfig {
  id: number | string;
  name: string;
  modelName?: string;
  enabled?: number | boolean;
  isActive?: number | boolean;
}
interface Character {
  id: number | string;
  name: string;
  status?: string;
  isActive?: number | boolean;
}

const modelConfigs = ref<ModelConfig[]>([]);
const characters = ref<Character[]>([]);
const modelLoading = ref(false);
const characterLoading = ref(false);

const referenceFile = ref<File | null>(null);
const referencePreview = ref("");

const sizePreset = ref("512x512");
const form = reactive({
  modelConfigId: "" as number | string,
  characterId: "" as number | string,
  name: "",
  prompt: "",
  negativePrompt: "",
  outputWidth: 512,
  outputHeight: 512,
});

const step1Valid = computed(
  () =>
    !!referenceFile.value &&
    !!form.modelConfigId &&
    !!form.name.trim() &&
    !!form.characterId,
);

const selectedCharacterName = computed(() => {
  const c = characters.value.find((item) => item.id === form.characterId);
  return c?.name || "";
});

const selectedModelName = computed(() => {
  const m = modelConfigs.value.find((item) => item.id === form.modelConfigId);
  return m?.name || "";
});

const selectedCategories = computed<ActionCategory[]>(() =>
  categories.value.filter((cat) =>
    cat.actions.some((a) => isSelected(a.key)),
  ),
);

function selectedActionsInCategory(categoryKey: string): ActionDefinition[] {
  return categoryActions(categoryKey).filter((a) => isSelected(a.key));
}

function onSizeChange(value: string) {
  const parts = value.split("x");
  if (parts.length === 2) {
    form.outputWidth = Number(parts[0]) || 512;
    form.outputHeight = Number(parts[1]) || 512;
  }
}

function onReferenceChange(file: UploadFile) {
  const raw = file.raw;
  if (!raw) return;
  const allowed = /\.(png|jpe?g|webp)$/i;
  if (!allowed.test(raw.name)) {
    ElMessage.warning("仅支持 PNG / JPG / JPEG / WEBP 格式");
    return;
  }
  if (referencePreview.value) URL.revokeObjectURL(referencePreview.value);
  referenceFile.value = raw;
  referencePreview.value = URL.createObjectURL(raw);
}

function clearReference() {
  if (referencePreview.value) URL.revokeObjectURL(referencePreview.value);
  referenceFile.value = null;
  referencePreview.value = "";
}

function goPrev() {
  actionError.value = "";
  if (step.value > 0) step.value--;
}

function goNext() {
  if (step.value === 1) {
    if (selectedCount.value === 0) {
      actionError.value = "请至少选择一个动作";
      ElMessage.warning("请至少选择一个动作");
      return;
    }
    if (!hasDefaultIdle.value) {
      actionError.value = "请至少选择一个支持默认待机的动作";
      ElMessage.warning("请至少选择一个支持默认待机的动作");
      return;
    }
  }
  actionError.value = "";
  step.value++;
}

async function loadModelConfigs() {
  modelLoading.value = true;
  try {
    const list = (await get<ModelConfig[]>("/api/imagegen/configs")) || [];
    modelConfigs.value = list.filter((m) => Number(m.enabled) === 1);
  } catch {
    modelConfigs.value = [];
  } finally {
    modelLoading.value = false;
  }
}

async function loadCharacters() {
  characterLoading.value = true;
  try {
    const list = (await get<Character[]>("/api/characters")) || [];
    characters.value = list.filter(
      (c) => c.status === "enabled" || c.isActive === true || c.isActive === 1,
    );
  } catch {
    characters.value = [];
  } finally {
    characterLoading.value = false;
  }
}

async function submit() {
  if (!referenceFile.value) {
    ElMessage.error("请先上传参考图");
    return;
  }
  if (!form.modelConfigId || !form.name.trim() || !form.characterId) {
    ElMessage.error("请补全基础配置");
    return;
  }
  if (selectedCount.value === 0) {
    ElMessage.error("请至少选择一个动作");
    return;
  }
  submitting.value = true;
  try {
    const fd = new FormData();
    fd.append("characterId", String(form.characterId));
    fd.append("modelConfigId", String(form.modelConfigId));
    fd.append("name", form.name.trim());
    fd.append("referenceImage", referenceFile.value, referenceFile.value.name);
    fd.append("prompt", form.prompt || "");
    fd.append("negativePrompt", form.negativePrompt || "");
    fd.append("outputWidth", String(form.outputWidth));
    fd.append("outputHeight", String(form.outputHeight));
    fd.append("selectedActionKeys", JSON.stringify(selectedKeys.value));
    const created = await post<{ id: string | number }>(
      "/api/desktop-pets/generation-tasks",
      fd,
      { timeout: 60000 },
    );
    const taskId = created?.id;
    if (!taskId) {
      ElMessage.error("任务已创建,但未能获取任务ID");
      return;
    }
    createdTaskId.value = taskId;
    ElMessage.success("任务已创建");
    await startTask(taskId);
  } catch (err: any) {
    ElMessage.error(err?.message || "创建失败");
  } finally {
    submitting.value = false;
  }
}

async function startTask(taskId: string | number) {
  startLoading.value = true;
  startFailed.value = false;
  startError.value = "";
  try {
    await post(`/api/desktop-pets/generation-tasks/${taskId}/start`);
    ElMessage.success("已开始生成");
    step.value = 3;
    goTaskList();
  } catch (err: any) {
    startFailed.value = true;
    startError.value = err?.message || "开始生成请求失败,请稍后重试";
    step.value = 3;
    ElMessage.warning("任务已保存,但尚未开始生成");
  } finally {
    startLoading.value = false;
  }
}

async function restartStart() {
  if (!createdTaskId.value) return;
  await startTask(createdTaskId.value);
}

function goTaskList() {
  const query: Record<string, string> = {};
  if (createdTaskId.value) query.taskId = String(createdTaskId.value);
  router.push({ path: "/creative-workshop/pet/tasks", query });
}

function resetWizard() {
  step.value = 0;
  startFailed.value = false;
  startError.value = "";
  createdTaskId.value = null;
  clearReference();
  form.modelConfigId = "";
  form.characterId = "";
  form.name = "";
  form.prompt = "";
  form.negativePrompt = "";
  sizePreset.value = "512x512";
  form.outputWidth = 512;
  form.outputHeight = 512;
  clearAll();
  actionError.value = "";
}

onMounted(async () => {
  await Promise.all([loadModelConfigs(), loadCharacters(), loadActions()]);
});

onUnmounted(() => {
  if (referencePreview.value) URL.revokeObjectURL(referencePreview.value);
});
</script>

<style scoped>
.pet-creation {
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
  margin: 0 0 16px;
  color: var(--console-text-muted);
  line-height: 1.6;
}
.wizard-steps {
  margin: 8px 0 20px;
}
.step-body {
  max-width: 880px;
}
.step-card {
  border: 1px solid var(--console-border, var(--el-border-color-light));
  background: var(--ac-color-surface);
}
.upload-row {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}
.reference-preview {
  display: flex;
  align-items: center;
  gap: 10px;
}
.reference-preview img {
  width: 96px;
  height: 96px;
  object-fit: cover;
  border-radius: 8px;
  border: 1px solid var(--el-border-color-light);
  background: var(--ac-color-bg-secondary, #f5f7fa);
}
.upload-tip,
.hint {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.step-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 16px;
}
.action-alert {
  margin-bottom: 12px;
}
.action-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  padding: 12px 14px;
  margin-bottom: 14px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 8px;
  background: var(--ac-color-surface-soft, var(--ac-color-surface));
}
.preset-buttons,
.toolbar-stats {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.toolbar-label {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
.category-card {
  margin-bottom: 12px;
  border: 1px solid var(--el-border-color-light);
  background: var(--ac-color-surface);
}
.category-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.category-actions {
  display: flex;
  gap: 4px;
}
.action-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 0;
  border-bottom: 1px solid var(--el-border-color-lighter);
  flex-wrap: wrap;
}
.action-row:last-child {
  border-bottom: 0;
}
.action-name {
  font-weight: 500;
  color: var(--console-text);
}
.action-desc {
  flex: 1;
  min-width: 200px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
.action-estimate {
  color: var(--el-text-color-secondary);
  font-size: 12px;
  white-space: nowrap;
}
.confirm-reference {
  width: 120px;
  height: 120px;
  object-fit: cover;
  border-radius: 8px;
  border: 1px solid var(--el-border-color-light);
}
.confirm-category {
  margin-bottom: 12px;
}
.confirm-cat-name {
  margin-bottom: 6px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
.confirm-action-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.confirm-tag {
  margin: 0;
}
.complete-state {
  padding: 40px 0;
}
</style>

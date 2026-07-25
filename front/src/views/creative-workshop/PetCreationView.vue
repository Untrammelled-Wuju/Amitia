<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <main class="pet-creation">
    <ExtensionPageHeader title="桌宠制作" description="创建和定制你的桌面陪伴角色" parent-title="创意工坊" parent-path="/creative-workshop" />

    <el-steps
      v-if="step < 3"
      :active="step"
      finish-status="success"
      align-center
      class="wizard-steps"
    >
      <el-step title="基础配置" />
      <el-step title="动作选择" />
      <el-step title="确认创建" />
    </el-steps>

    <section v-if="step < 3" class="step-body">
      <el-card v-show="step === 0" shadow="never" class="step-card">
        <el-form label-position="top" :model="form">
          <el-form-item label="参考图" required>
            <div class="upload-row">
              <el-upload
                :auto-upload="false"
                :show-file-list="false"
                accept=".png,.jpg,.jpeg,.webp"
                :on-change="onReferenceChange"
              >
                <el-button :icon="Upload">选择图片</el-button>
              </el-upload>
              <div v-if="referencePreview" class="reference-preview">
                <img :src="referencePreview" alt="参考图预览" />
                <el-button
                  text
                  type="danger"
                  :icon="Delete"
                  @click="clearReference"
                  >移除</el-button
                >
              </div>
              <span v-else class="upload-tip"
                >支持 PNG / JPG / JPEG / WEBP,仅可上传 1 张</span
              >
            </div>
          </el-form-item>

          <el-form-item label="生图模型" required>
            <el-select
              v-model="form.modelConfigId"
              placeholder="选择生图模型"
              style="width: 100%"
              :loading="modelLoading"
            >
              <el-option
                v-for="m in modelConfigs"
                :key="m.id"
                :label="m.name"
                :value="m.id"
              />
            </el-select>
            <span v-if="!modelConfigs.length && !modelLoading" class="hint"
              >未找到已启用的生图模型,请先在设置中配置</span
            >
          </el-form-item>

          <el-form-item label="桌宠名称" required>
            <el-input
              v-model="form.name"
              placeholder="为本次桌宠生成任务命名"
              maxlength="60"
              show-word-limit
            />
          </el-form-item>

          <el-form-item label="绑定角色" required>
            <el-select
              v-model="form.characterId"
              placeholder="选择绑定的角色"
              style="width: 100%"
              :loading="characterLoading"
            >
              <el-option
                v-for="c in characters"
                :key="c.id"
                :label="c.name"
                :value="c.id"
              />
            </el-select>
            <span v-if="!characters.length && !characterLoading" class="hint"
              >未找到可用角色,请先在角色管理中启用</span
            >
          </el-form-item>

          <el-form-item label="输出尺寸" required>
            <el-select
              v-model="sizePreset"
              style="width: 220px"
              @change="onSizeChange"
            >
              <el-option label="512 × 512" value="512x512" />
              <el-option label="768 × 768" value="768x768" />
              <el-option label="1024 × 1024" value="1024x1024" />
            </el-select>
          </el-form-item>

          <el-form-item label="补充描述(prompt)">
            <el-input
              v-model="form.prompt"
              type="textarea"
              :rows="3"
              placeholder="可选,补充画面风格或细节要求"
              maxlength="500"
              show-word-limit
            />
          </el-form-item>

          <el-form-item label="负面描述(negativePrompt)">
            <el-input
              v-model="form.negativePrompt"
              type="textarea"
              :rows="2"
              placeholder="可选,描述不希望出现的内容"
              maxlength="500"
              show-word-limit
            />
          </el-form-item>
        </el-form>

        <div class="step-actions">
          <el-button type="primary" :disabled="!step1Valid" @click="goNext"
            >下一步</el-button
          >
        </div>
      </el-card>

      <div v-show="step === 1" class="step-card">
        <el-alert
          v-if="actionError"
          :title="actionError"
          type="warning"
          show-icon
          :closable="false"
          class="action-alert"
        />

        <div class="action-toolbar">
          <div class="preset-buttons">
            <span class="toolbar-label">快捷方案:</span>
            <el-button
              v-for="preset in presets"
              :key="preset.key"
              size="small"
              @click="applyPreset(preset)"
              >{{ preset.name }}</el-button
            >
          </div>
          <div class="toolbar-stats">
            <el-tag type="info">已选 {{ selectedCount }} 个动作</el-tag>
            <el-tag type="warning">预估 {{ estimatedGenerationCount }} 张</el-tag>
            <el-button
              size="small"
              :disabled="selectedCount === 0"
              @click="clearAll"
              >清空</el-button
            >
          </div>
        </div>

        <el-empty
          v-if="!categories.length && !loading"
          description="暂无可用动作"
        />

        <el-card
          v-for="cat in categories"
          :key="cat.key"
          shadow="never"
          class="category-card"
        >
          <template #header>
            <div class="category-header">
              <el-checkbox
                :model-value="isCategoryAllSelected(cat.key)"
                :indeterminate="isCategoryPartialSelected(cat.key)"
                @change="toggleCategory(cat.key)"
              >
                <strong>{{ cat.name }}</strong>
              </el-checkbox>
              <div class="category-actions">
                <el-button text size="small" @click="selectAllCategory(cat.key)"
                  >全选</el-button
                >
                <el-button text size="small" @click="clearCategory(cat.key)"
                  >取消</el-button
                >
              </div>
            </div>
          </template>

          <div
            v-for="action in cat.actions"
            :key="action.key"
            class="action-row"
          >
            <el-checkbox
              :model-value="isSelected(action.key)"
              @change="toggle(action.key)"
            >
              <span class="action-name">{{ action.name }}</span>
            </el-checkbox>
            <el-tag v-if="action.recommended" size="small" type="success"
              >推荐</el-tag
            >
            <el-tag
              v-if="action.supportsDefaultIdle"
              size="small"
              type="primary"
              >可作待机</el-tag
            >
            <span class="action-desc">{{ action.description || "—" }}</span>
            <span class="action-estimate"
              >预估 {{ action.estimatedGenerationCount }} 张</span
            >
          </div>
        </el-card>

        <div class="step-actions">
          <el-button @click="goPrev">上一步</el-button>
          <el-button type="primary" @click="goNext">下一步</el-button>
        </div>
      </div>

      <el-card v-show="step === 2" shadow="never" class="step-card">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="桌宠名称">{{
            form.name
          }}</el-descriptions-item>
          <el-descriptions-item label="绑定角色">{{
            selectedCharacterName || "—"
          }}</el-descriptions-item>
          <el-descriptions-item label="生图模型">{{
            selectedModelName || "—"
          }}</el-descriptions-item>
          <el-descriptions-item label="输出尺寸"
            >{{ form.outputWidth }} × {{ form.outputHeight }}</el-descriptions-item
          >
          <el-descriptions-item label="已选动作"
            >{{ selectedCount }} 个</el-descriptions-item
          >
          <el-descriptions-item label="预估图片"
            >{{ estimatedGenerationCount }} 张</el-descriptions-item
          >
          <el-descriptions-item label="补充描述" :span="2">{{
            form.prompt || "—"
          }}</el-descriptions-item>
          <el-descriptions-item label="负面描述" :span="2">{{
            form.negativePrompt || "—"
          }}</el-descriptions-item>
          <el-descriptions-item label="参考图" :span="2">
            <img
              v-if="referencePreview"
              :src="referencePreview"
              class="confirm-reference"
              alt="参考图"
            />
            <span v-else>—</span>
          </el-descriptions-item>
        </el-descriptions>

        <el-divider content-position="left">已选动作明细</el-divider>
        <div
          v-for="cat in selectedCategories"
          :key="cat.key"
          class="confirm-category"
        >
          <div class="confirm-cat-name">{{ cat.name }}</div>
          <div class="confirm-action-list">
            <el-tag
              v-for="action in selectedActionsInCategory(cat.key)"
              :key="action.key"
              class="confirm-tag"
              >{{ action.name }}</el-tag
            >
          </div>
        </div>
        <el-empty
          v-if="selectedCount === 0"
          description="未选择任何动作"
          :image-size="60"
        />

        <div class="step-actions">
          <el-button @click="goPrev">上一步</el-button>
          <el-button type="primary" :loading="submitting" @click="submit"
            >确认并创建</el-button
          >
        </div>
      </el-card>
    </section>

    <section v-else class="complete-state">
      <el-result
        v-if="startFailed"
        icon="warning"
        title="任务已保存"
        :sub-title="startError || '开始生成失败,可以稍后重试'"
      >
        <template #extra>
          <el-button type="primary" :loading="startLoading" @click="restartStart"
            >重试开始</el-button
          >
          <el-button @click="goTaskList">查看任务列表</el-button>
          <el-button @click="resetWizard">继续创建</el-button>
        </template>
      </el-result>
      <el-result
        v-else
        icon="success"
        title="任务已创建"
        sub-title="任务已加入生成队列,可在任务列表查看进度"
      >
        <template #extra>
          <el-button type="primary" @click="goTaskList">查看任务列表</el-button>
          <el-button @click="resetWizard">继续创建</el-button>
        </template>
      </el-result>
    </section>
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

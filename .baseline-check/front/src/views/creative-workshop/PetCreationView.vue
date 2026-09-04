<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <main class="pet-creation">
    <ExtensionPageHeader title="桌宠制作" description="创建和定制你的桌面陪伴角色" grandparent-title="创意工坊" grandparent-path="/creative-workshop" parent-title="桌宠" parent-path="/creative-workshop/pet" />


    <div v-if="noModelsAvailable" class="no-models-banner">
      <el-result icon="warning" title="未配置生图模型" sub-title="需要先配置至少一个已启用的生图模型才能制作桌宠">
        <template #extra>
          <el-button type="primary" @click="goToImageGenConfig">前往配置生图模型</el-button>
        </template>
      </el-result>
    </div>

    <template v-else>
      <div v-if="step < 3" class="step-cards">
        <div
          v-for="(s, i) in steps"
          :key="i"
          class="step-card"
          :class="{ done: step > i, active: step === i }"
          @click="goToStep(i)"
        >
          <div class="step-card-badge">
            <el-icon v-if="step > i"><Check /></el-icon>
            <span v-else>{{ i + 1 }}</span>
          </div>
          <div class="step-card-content">
            <span class="step-card-title">{{ s.label }}</span>
            <span class="step-card-desc">{{ s.desc }}</span>
          </div>
        </div>
      </div>
    <section v-if="step < 3" class="step-body">
      <div v-show="step === 0" class="step-panel">
        <div class="panel-card">
          <div class="panel-section">
            <h3 class="section-title">参考素材</h3>
            <div class="upload-row">
              <el-upload
                :auto-upload="false"
                :show-file-list="false"
                accept=".png,.jpg,.jpeg,.webp"
                :on-change="onReferenceChange"
                drag
              >
                <div v-if="!referencePreview" class="upload-placeholder">
                  <el-icon :size="36"><Upload /></el-icon>
                  <p>拖拽或点击上传参考图</p>
                  <span>PNG / JPG / JPEG / WEBP</span>
                </div>
                <div v-else class="upload-preview-container">
                  <img :src="referencePreview" alt="参考图预览" />
                </div>
              </el-upload>
              <div v-if="referencePreview" class="preview-actions">
                <span class="preview-name">{{ referenceFile?.name }}</span>
                <el-button text type="danger" :icon="Delete" @click="clearReference">移除</el-button>
              </div>
            </div>
          </div>

          <div class="panel-section">
            <h3 class="section-title">基本设置</h3>
            <div class="form-grid">
              <div class="form-item-full">
                <label class="form-label">桌宠名称 <span class="required">*</span></label>
                <el-input
                  v-model="form.name"
                  placeholder="为本次桌宠生成任务命名"
                  maxlength="60"
                  show-word-limit
                />
              </div>
              <div class="form-item-half">
                <label class="form-label">生图模型 <span class="required">*</span></label>
                <el-select
                  v-model="form.modelConfigId"
                  placeholder="选择生图模型"
                  :loading="modelLoading"
                >
                  <el-option
                    v-for="m in modelConfigs"
                    :key="m.id"
                    :label="m.name"
                    :value="m.id"
                  />
                </el-select>
                <span v-if="!modelConfigs.length && !modelLoading" class="hint">未找到已启用的生图模型,请先在设置中配置</span>
              </div>
              <div class="form-item-half">
                <label class="form-label">绑定角色 <span class="required">*</span></label>
                <el-select
                  v-model="form.characterId"
                  placeholder="选择绑定的角色"
                  :loading="characterLoading"
                >
                  <el-option
                    v-for="c in characters"
                    :key="c.id"
                    :label="c.name"
                    :value="c.id"
                  />
                </el-select>
                <span v-if="!characters.length && !characterLoading" class="hint">未找到可用角色,请先在角色管理中启用</span>
              </div>
              <div class="form-item-half">
                <label class="form-label">输出尺寸 <span class="required">*</span></label>
                <el-select v-model="sizePreset" @change="onSizeChange">
                  <el-option label="512 × 512" value="512x512" />
                  <el-option label="768 × 768" value="768x768" />
                  <el-option label="1024 × 1024" value="1024x1024" />
                </el-select>
              </div>
            </div>
          </div>

          <div class="panel-section">
            <h3 class="section-title">提示词（可选）</h3>
            <div class="form-grid">
              <div class="form-item-full">
                <label class="form-label">补充描述</label>
                <el-input
                  v-model="form.prompt"
                  type="textarea"
                  :rows="3"
                  placeholder="补充画面风格或细节要求"
                  maxlength="500"
                  show-word-limit
                />
              </div>
              <div class="form-item-full">
                <label class="form-label">负面描述</label>
                <el-input
                  v-model="form.negativePrompt"
                  type="textarea"
                  :rows="2"
                  placeholder="描述不希望出现的内容"
                  maxlength="500"
                  show-word-limit
                />
              </div>
            </div>
          </div>
        </div>
      </div>

      <div v-show="step === 1" class="step-panel">
        <el-alert
          v-if="actionError"
          :title="actionError"
          type="warning"
          show-icon
          :closable="false"
          class="action-alert"
        />

        <div class="action-toolbar">
          <div class="preset-group">
            <span class="toolbar-label">快捷方案</span>
            <el-button
              v-for="preset in presets"
              :key="preset.key"
              size="small"
              :type="isPresetActive(preset) ? 'primary' : 'default'"
              @click="applyPreset(preset)"
            >{{ preset.name }}</el-button>
          </div>
          <div class="stat-group">
            <el-tag type="primary" effect="plain">已选 {{ selectedCount }} 个动作</el-tag>
            <el-tag type="warning" effect="plain">预估 {{ estimatedGenerationCount }} 张</el-tag>
            <el-button size="small" :disabled="selectedCount === 0" @click="clearAll">清空</el-button>
          </div>
        </div>

        <div v-if="orderedSelectedActions.length" class="selected-actions-area">
          <div class="sort-header">
            <span class="sort-title">已选动作</span>
            <span class="sort-hint">可拖拽调整生成顺序</span>
          </div>
          <div class="sortable-row">
            <div
              v-for="(item, idx) in orderedSelectedActions"
              :key="item.key"
              class="sortable-chip"
              :class="{ 'drag-over': dragOverIndex === idx, 'drag-source': dragIndex === idx }"
              draggable="true"
              @dragstart="onDragStart($event, idx)"
              @dragover.prevent="onDragOver($event, idx)"
              @dragleave="onDragLeave"
              @drop="onDrop($event, idx)"
              @dragend="onDragEnd"
            >
              <div class="chip-handle">
                <span class="handle-dots">⋮⋮</span>
              </div>
              <span class="chip-name">{{ item.name }}</span>
              <span class="chip-estimate">{{ item.estimatedGenerationCount }}张</span>
              <el-button text size="small" class="chip-remove" @click.stop="toggle(item.key)">
                <el-icon><Close /></el-icon>
              </el-button>
            </div>
          </div>
        </div>

        <el-empty v-if="!categories.length && !loading" description="暂无可用动作" />

        <div class="category-list">
          <div v-for="cat in categories" :key="cat.key" class="category-section">
            <div class="category-header">
              <div class="category-title-row">
                <el-checkbox
                  :model-value="isCategoryAllSelected(cat.key)"
                  :indeterminate="isCategoryPartialSelected(cat.key)"
                  @change="toggleCategory(cat.key)"
                >
                  <strong>{{ cat.name }}</strong>
                </el-checkbox>
                <span class="category-count">{{ cat.actions.length }} 个动作</span>
              </div>
              <div class="category-actions">
                <el-button text size="small" @click="selectAllCategory(cat.key)">全选</el-button>
                <el-button text size="small" @click="clearCategory(cat.key)">取消</el-button>
              </div>
            </div>

            <div class="action-cards-grid">
              <div
                v-for="action in cat.actions"
                :key="action.key"
                class="action-card"
                :class="{ selected: isSelected(action.key) }"
                @click="toggle(action.key)"
              >
                <div class="card-top">
                  <div class="card-check">
                    <el-icon v-if="isSelected(action.key)"><Check /></el-icon>
                  </div>
                  <div class="card-tags">
                    <el-tag v-if="action.recommended" size="small" type="success" effect="plain">推荐</el-tag>
                    <el-tag v-if="action.supportsDefaultIdle" size="small" type="primary" effect="plain">可作待机</el-tag>
                  </div>
                </div>
                <div class="card-body">
                  <span class="card-title">{{ action.name }}</span>
                  <span class="card-desc">{{ action.description || "—" }}</span>
                </div>
                <div class="card-footer">
                  <span class="card-estimate">预估 {{ action.estimatedGenerationCount }} 张</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div v-show="step === 2" class="step-panel">
        <div class="panel-card">
          <div class="panel-section">
            <h3 class="section-title">配置摘要</h3>
            <div class="summary-grid">
              <div class="summary-item">
                <span class="summary-label">桌宠名称</span>
                <span class="summary-value">{{ form.name }}</span>
              </div>
              <div class="summary-item">
                <span class="summary-label">绑定角色</span>
                <span class="summary-value">{{ selectedCharacterName || "—" }}</span>
              </div>
              <div class="summary-item">
                <span class="summary-label">生图模型</span>
                <span class="summary-value">{{ selectedModelName || "—" }}</span>
              </div>
              <div class="summary-item">
                <span class="summary-label">输出尺寸</span>
                <span class="summary-value">{{ form.outputWidth }} × {{ form.outputHeight }}</span>
              </div>
              <div class="summary-item">
                <span class="summary-label">已选动作</span>
                <span class="summary-value">{{ selectedCount }} 个</span>
              </div>
              <div class="summary-item">
                <span class="summary-label">预估图片</span>
                <span class="summary-value">{{ estimatedGenerationCount }} 张</span>
              </div>
              <div class="summary-item summary-full">
                <span class="summary-label">补充描述</span>
                <span class="summary-value">{{ form.prompt || "—" }}</span>
              </div>
              <div class="summary-item summary-full">
                <span class="summary-label">负面描述</span>
                <span class="summary-value">{{ form.negativePrompt || "—" }}</span>
              </div>
              <div class="summary-item summary-full">
                <span class="summary-label">参考图</span>
                <div class="summary-ref">
                  <img v-if="referencePreview" :src="referencePreview" alt="参考图" />
                  <span v-else>—</span>
                </div>
              </div>
            </div>
          </div>

          <div class="panel-section">
            <h3 class="section-title">动作明细</h3>
            <div v-for="cat in selectedCategories" :key="cat.key" class="confirm-category">
              <div class="confirm-cat-name">{{ cat.name }}</div>
              <div class="confirm-tags">
                <el-tag
                  v-for="action in selectedActionsInCategory(cat.key)"
                  :key="action.key"
                  size="small"
                  type="info"
                >{{ action.name }}</el-tag>
              </div>
            </div>
            <el-empty v-if="selectedCount === 0" description="未选择任何动作" :image-size="60" />
          </div>
        </div>
      </div>

      <div class="step-actions">
        <el-button v-if="step > 0" @click="goPrev">上一步</el-button>
        <el-button v-if="step < 2" type="primary" :disabled="step === 0 && !step1Valid" @click="goNext">下一步</el-button>
        <el-button v-if="step === 2" type="primary" :loading="submitting" @click="submit">确认并创建</el-button>
      </div>
    </section>

    <section v-else class="complete-state">
      <el-result
        v-if="startFailed"
        icon="warning"
        title="任务已保存"
        :sub-title="startError || '开始生成失败,可以稍后重试'"
      >
        <template #extra>
          <el-button type="primary" :loading="startLoading" @click="restartStart">重试开始</el-button>
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
    </template>
  </main>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from "vue";
import { useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import ExtensionPageHeader from "../extensions/components/ExtensionPageHeader.vue";
import { Upload, Delete, Check, Close } from "@element-plus/icons-vue";
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
  reorderSelected,
  isCategoryAllSelected,
  isCategoryPartialSelected,
  categoryActions,
  selectedCount,
  estimatedGenerationCount,
  hasDefaultIdle,
} = useActionDefinitions();

const steps = [
  { label: "基础配置", desc: "上传参考图与基本设置" },
  { label: "动作选择", desc: "选择并排序桌宠动作" },
  { label: "确认创建", desc: "复核配置并提交任务" },
];

const step = ref(0);
const submitting = ref(false);
const actionError = ref("");
const createdTaskId = ref<string | number | null>(null);
const startLoading = ref(false);
const startFailed = ref(false);
const startError = ref("");

const dragIndex = ref<number | null>(null);
const dragOverIndex = ref<number | null>(null);

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

const noModelsAvailable = computed(() => !modelLoading.value && modelConfigs.value.length === 0);

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

const orderedSelectedActions = computed<ActionDefinition[]>(() => {
  const result: ActionDefinition[] = [];
  for (const key of selectedKeys.value) {
    for (const cat of categories.value) {
      const found = cat.actions.find((a) => a.key === key);
      if (found) {
        result.push(found);
        break;
      }
    }
  }
  return result;
});

function isPresetActive(preset: ActionPreset): boolean {
  if (!preset.actionKeys.length) return false;
  const set = new Set(selectedKeys.value);
  if (set.size !== preset.actionKeys.length) return false;
  return preset.actionKeys.every((k) => set.has(k));
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

function goToStep(index: number) {
  if (index <= step.value) {
    step.value = index;
  }
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

function onDragStart(e: DragEvent, idx: number) {
  dragIndex.value = idx;
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = "move";
    e.dataTransfer.setData("text/plain", String(idx));
  }
}

function onDragOver(e: DragEvent, idx: number) {
  dragOverIndex.value = idx;
  if (e.dataTransfer) {
    e.dataTransfer.dropEffect = "move";
  }
}

function onDragLeave() {
  dragOverIndex.value = null;
}

function onDrop(_e: DragEvent, idx: number) {
  if (dragIndex.value !== null && dragIndex.value !== idx) {
    reorderSelected(dragIndex.value, idx);
  }
  dragIndex.value = null;
  dragOverIndex.value = null;
}

function onDragEnd() {
  dragIndex.value = null;
  dragOverIndex.value = null;
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

function goToImageGenConfig() {
  router.push("/settings/model/imagegen");
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

/* ===== 自定义步骤指示器 ===== */
.step-cards {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
  padding: 0;
  margin-bottom: 20px;
}

.step-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  border: 1.5px solid var(--el-border-color-light);
  border-radius: 8px;
  background: var(--ac-color-surface);
  cursor: pointer;
  transition: all 200ms ease;
  user-select: none;
}

.step-card:hover {
  border-color: var(--el-color-primary-light-5);
}

.step-card.active {
  border-color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
  box-shadow: 0 0 0 3px var(--el-color-primary-light-7);
}

.step-card.done {
  border-color: var(--el-color-success-light-3);
  background: var(--el-color-success-light-9);
}

.step-card-badge {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  font-weight: 600;
  border: 2px solid var(--el-border-color);
  background: var(--ac-color-surface);
  color: var(--el-text-color-secondary);
  flex-shrink: 0;
  transition: all 200ms ease;
}

.step-card.active .step-card-badge {
  border-color: var(--el-color-primary);
  background: var(--el-color-primary-light-7);
  color: var(--el-color-primary);
}

.step-card.done .step-card-badge {
  border-color: var(--el-color-success);
  background: var(--el-color-success);
  color: #fff;
}

.step-card-content {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.step-card-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--el-text-color-regular);
}

.step-card.active .step-card-title {
  color: var(--el-color-primary);
}

.step-card.done .step-card-title {
  color: var(--el-color-success);
}

.step-card-desc {
  font-size: 11px;
  color: var(--el-text-color-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.no-models-banner {
  margin: 24px 0;
}

/* ===== 步骤主体 ===== */
.step-body {
  max-width: 920px;
}

.step-panel {
  animation: fadeIn 240ms ease;
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(6px); }
  to { opacity: 1; transform: translateY(0); }
}

.panel-card {
  border: 1px solid var(--console-border, var(--el-border-color-light));
  border-radius: 10px;
  background: var(--ac-color-surface);
  overflow: hidden;
}

.panel-section {
  padding: 20px 24px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.panel-section:last-child {
  border-bottom: 0;
}

.section-title {
  margin: 0 0 16px;
  font-size: 15px;
  font-weight: 600;
  color: var(--console-text);
}

/* ===== Step 0 表单 ===== */
.upload-row {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.upload-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 32px 20px;
  color: var(--el-text-color-secondary);
}

.upload-placeholder p {
  margin: 0;
  font-size: 14px;
}

.upload-placeholder span {
  font-size: 12px;
  color: var(--el-text-color-placeholder);
}

.upload-preview-container {
  width: 100%;
  height: 180px;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border-radius: 6px;
  background: var(--ac-color-bg-secondary, #f5f7fa);
}

.upload-preview-container img {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}

.preview-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.preview-name {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 260px;
}

.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.form-item-full {
  grid-column: 1 / -1;
}

.form-item-half {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--el-text-color-regular);
}

.form-label .required {
  color: var(--el-color-danger);
  margin-left: 2px;
}

.hint {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

/* ===== 步骤操作按钮 ===== */
.step-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 20px;
  padding: 0 4px;
}

/* ===== Step 1 动作选择 ===== */
.action-alert {
  margin-bottom: 14px;
}

.action-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  padding: 12px 16px;
  margin-bottom: 14px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 8px;
  background: var(--ac-color-surface-soft, var(--ac-color-surface));
}

.preset-group,
.stat-group {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.toolbar-label {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

/* 已选动作排序区 */
.selected-actions-area {
  margin-bottom: 16px;
  padding: 14px 16px;
  border: 1px solid var(--el-color-primary-light-5);
  border-radius: 8px;
  background: var(--el-color-primary-light-9);
}

.sort-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.sort-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--el-color-primary);
}

.sort-hint {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.sortable-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  min-height: 36px;
}

.sortable-chip {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 8px 6px 6px;
  border: 1px solid var(--el-border-color-light);
  border-radius: 6px;
  background: var(--ac-color-surface);
  cursor: grab;
  transition: all 160ms ease;
  user-select: none;
}

.sortable-chip:hover {
  border-color: var(--el-color-primary-light-3);
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.06);
}

.sortable-chip.drag-source {
  opacity: 0.5;
  border-style: dashed;
}

.sortable-chip.drag-over {
  border-color: var(--el-color-primary);
  background: var(--el-color-primary-light-8);
  transform: translateY(-2px);
}

.sortable-chip:active {
  cursor: grabbing;
}

.chip-handle {
  display: flex;
  align-items: center;
  color: var(--el-text-color-placeholder);
  font-size: 10px;
  line-height: 1;
}

.chip-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--console-text);
}

.chip-estimate {
  font-size: 11px;
  color: var(--el-text-color-secondary);
}

.chip-remove {
  padding: 2px;
  margin-left: 2px;
}

/* 分类列表 */
.category-list {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.category-section {
  border: 1px solid var(--el-border-color-light);
  border-radius: 8px;
  background: var(--ac-color-surface);
  overflow: hidden;
}

.category-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--el-border-color-lighter);
  background: var(--ac-color-surface-soft, var(--ac-color-bg-secondary));
}

.category-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.category-count {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.category-actions {
  display: flex;
  gap: 4px;
}

/* 动作卡片网格 */
.action-cards-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 10px;
  padding: 14px 16px;
}

.action-card {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 14px;
  border: 1.5px solid var(--el-border-color-light);
  border-radius: 8px;
  background: var(--ac-color-surface);
  cursor: pointer;
  transition: all 180ms ease;
}

.action-card:hover {
  border-color: var(--el-color-primary-light-5);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.05);
  transform: translateY(-1px);
}

.action-card.selected {
  border-color: var(--el-color-primary);
  background: var(--el-color-primary-light-9);
  box-shadow: 0 0 0 1px var(--el-color-primary-light-5);
}

.card-top {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

.card-check {
  width: 22px;
  height: 22px;
  border-radius: 50%;
  border: 2px solid var(--el-border-color);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  color: #fff;
  transition: all 180ms ease;
  flex-shrink: 0;
}

.action-card.selected .card-check {
  border-color: var(--el-color-primary);
  background: var(--el-color-primary);
}

.card-tags {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}

.card-body {
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: 1;
}

.card-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--console-text);
}

.card-desc {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.card-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-estimate {
  font-size: 11px;
  color: var(--el-text-color-placeholder);
}

/* ===== Step 2 确认摘要 ===== */
.summary-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 8px;
  overflow: hidden;
}

.summary-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 14px 16px;
  border-bottom: 1px solid var(--el-border-color-lighter);
  border-right: 1px solid var(--el-border-color-lighter);
  background: var(--ac-color-surface);
}

.summary-item:nth-child(even) {
  border-right: 0;
}

.summary-item:nth-last-child(-n+2) {
  border-bottom: 0;
}

.summary-full {
  grid-column: 1 / -1;
  border-right: 0;
}

.summary-full:nth-last-child(1) {
  border-bottom: 0;
}

.summary-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  font-weight: 500;
}

.summary-value {
  font-size: 14px;
  color: var(--console-text);
  word-break: break-all;
}

.summary-ref {
  margin-top: 4px;
}

.summary-ref img {
  width: 120px;
  height: 120px;
  object-fit: cover;
  border-radius: 8px;
  border: 1px solid var(--el-border-color-light);
}

.confirm-category {
  margin-bottom: 12px;
}

.confirm-category:last-child {
  margin-bottom: 0;
}

.confirm-cat-name {
  margin-bottom: 6px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}

.confirm-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

/* ===== 完成状态 ===== */
.complete-state {
  padding: 40px 0;
}

@media (max-width: 720px) {
  .form-grid {
    grid-template-columns: 1fr;
  }

  .action-cards-grid {
    grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
  }

  .step-cards {
    grid-template-columns: 1fr;
    gap: 8px;
  }

  .step-card-desc {
    display: none;
  }
}
</style>

<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-dialog
    :model-value="modelValue"
    @update:model-value="emit('update:modelValue', $event)"
    title="导入角色包"
    width="560px"
    destroy-on-close
  >
    <template v-if="!preview">
      <div class="card-import-content">
        <div
          class="character-card-drop-zone"
          :class="{
            'has-file': !!selectedFile,
            'is-dragging': isDragActive,
          }"
          role="button"
          tabindex="0"
          aria-label="选择或拖入角色卡文件"
          @click="openFilePicker"
          @keydown.enter.prevent="openFilePicker"
          @keydown.space.prevent="openFilePicker"
          @dragenter.prevent="onDragEnter"
          @dragover.prevent="isDragActive = true"
          @dragleave.prevent="onDragLeave"
          @drop.prevent="onFileDrop"
        >
          <el-icon class="upload-icon"><UploadFilled /></el-icon>
          <template v-if="selectedFile">
            <strong>{{ selectedFile.name }}</strong>
            <span>{{ selectedFileSize }} · 已选择，点击预览继续</span>
          </template>
          <template v-else>
            <strong>拖入角色卡文件，或点击选择文件</strong>
            <span>支持 V2/V3 JSON、酒馆角色卡 JSON/PNG、PNG、CHARX</span>
          </template>
          <el-button
            :loading="previewing"
            @click.stop="openFilePicker"
          >
            {{ selectedFile ? "重新选择" : "选择文件" }}
          </el-button>
          <input
            ref="fileInput"
            class="sr-only"
            type="file"
            accept=".json,.png,.charx"
            @change="onFileChange"
          />
        </div>

        <div class="import-primary-action">
          <el-button
            type="primary"
            :loading="previewing"
            :disabled="!selectedFile"
            @click="emit('preview')"
          >
            预览角色卡
          </el-button>
        </div>
      </div>

      <section class="example-panel" aria-labelledby="import-json-example-title">
        <div class="example-panel-header">
          <div>
            <h3 id="import-json-example-title">JSON 示例</h3>
            <p>切换格式查看示例，替换内容后保存为 JSON 文件即可导入。</p>
          </div>
          <el-button text :icon="DocumentCopy" @click="copyExample">
            复制
          </el-button>
        </div>
        <el-radio-group
          v-model="activeExampleKey"
          class="example-switcher"
          size="small"
        >
          <el-radio-button
            v-for="item in jsonExamples"
            :key="item.key"
            :value="item.key"
          >
            {{ item.label }}
          </el-radio-button>
        </el-radio-group>
        <p class="example-description">{{ activeExample.description }}</p>
        <pre class="example-code"><code>{{ activeExample.json }}</code></pre>
      </section>
    </template>

    <template v-else>
      <el-alert
        v-if="preview?.risks?.length > 0"
        type="warning"
        title="风险提示"
        :closable="false"
        show-icon
        style="margin-bottom: 12px"
      >
        <template #default>
          <ul style="margin: 4px 0; padding-left: 16px; font-size: 13px">
            <li
              v-for="r in preview.risks"
              :key="r.category"
              :style="{
                color:
                  r.level === 'high'
                    ? 'var(--el-color-danger)'
                    : 'var(--el-color-warning)',
              }"
            >
              [{{ r.level === "high" ? "高" : "中" }}] {{ r.message }}
            </li>
          </ul>
        </template>
      </el-alert>

      <div class="import-preview-info">
        <div class="ipi-row">
          <span class="ipi-label">名称</span><strong>{{ preview.name }}</strong>
        </div>
        <div class="ipi-row">
          <span class="ipi-label">作者</span><span>{{ preview.creator }}</span>
        </div>
        <div class="ipi-row">
          <span class="ipi-label">格式</span
          ><span>{{ previewFormatLabel }}</span>
        </div>
        <div class="ipi-row">
          <span class="ipi-label">描述长度</span
          ><span>{{ preview.descriptionLength || 0 }}</span>
        </div>
        <div class="ipi-row">
          <span class="ipi-label">性格长度</span
          ><span>{{ preview.personalityLength || 0 }}</span>
        </div>
        <div class="ipi-row">
          <span class="ipi-label">Lorebook</span
          ><span>{{ preview.lorebookEntryCount || 0 }} 条</span>
        </div>
        <div class="ipi-row">
          <span class="ipi-label">系统提示</span
          ><span>{{ preview.hasSystemPrompt ? "有" : "无" }}</span>
        </div>
      </div>

      <el-divider />
      <div class="confirm-row" style="margin-bottom: 8px">
        <span style="font-size: 13px">输入 确认导入 以继续：</span>
        <el-input
          v-model="confirmTextModel"
          placeholder='输入"确认导入"'
          style="width: 160px"
          size="small"
        />
      </div>
      <el-row :gutter="8">
        <el-col :span="12">
          <el-button @click="emit('cancelPreview')" style="width: 100%"
            >返回</el-button
          >
        </el-col>
        <el-col :span="12">
          <el-button
            type="primary"
            :disabled="confirmText !== '确认导入'"
            :loading="importing"
            @click="emit('confirm')"
            style="width: 100%"
          >
            确认导入
          </el-button>
        </el-col>
      </el-row>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { DocumentCopy, UploadFilled } from "@element-plus/icons-vue";
import { ElMessage } from "element-plus";

const props = defineProps<{
  modelValue: boolean;
  packName: string;
  preview: any;
  previewing: boolean;
  confirmText: string;
  importing: boolean;
  history: any[];
}>();

const emit = defineEmits<{
  (e: "update:modelValue", v: boolean): void;
  (e: "update:packName", v: string): void;
  (e: "update:confirmText", v: string): void;
  (e: "preview"): void;
  (e: "cancelPreview"): void;
  (e: "confirm"): void;
  (e: "fileSelected", file: File | null): void;
}>();

const selectedFile = ref<File | null>(null);
const fileInput = ref<HTMLInputElement | null>(null);
const isDragActive = ref(false);
let dragDepth = 0;

type JsonExampleKey = "v2" | "v3" | "tavern";

interface JsonExample {
  key: JsonExampleKey;
  label: string;
  description: string;
  json: string;
}

const jsonExamples: JsonExample[] = [
  {
    key: "v2",
    label: "V2 JSON",
    description: "标准角色卡 V2 信封格式，角色字段放在 data 中。",
    json: `{
  "spec": "chara_card_v2",
  "spec_version": "2.0",
  "data": {
    "name": "林夏",
    "description": "温柔、好奇的旅行摄影师。",
    "personality": "友善、细腻，遇到感兴趣的话题会主动追问。",
    "scenario": "你们在一座海边小城暂时同住。",
    "first_mes": "我刚整理完今天的照片，你要一起看看吗？",
    "mes_example": "{{user}}: 今天拍到了什么？\\n{{char}}: 一张落日，还有一只一直跟着我的猫。",
    "creator_notes": "示例角色，可自由修改。",
    "system_prompt": "",
    "post_history_instructions": "",
    "alternate_greetings": [],
    "tags": ["日常", "治愈"],
    "creator": "示例作者",
    "character_version": "1.0",
    "extensions": {}
  }
}`,
  },
  {
    key: "v3",
    label: "V3 JSON",
    description: "标准角色卡 V3 信封格式，支持昵称、分组问候与资产声明。",
    json: `{
  "spec": "chara_card_v3",
  "spec_version": "3.0",
  "data": {
    "name": "林夏",
    "nickname": "小夏",
    "description": "温柔、好奇的旅行摄影师。",
    "personality": "友善、细腻，遇到感兴趣的话题会主动追问。",
    "scenario": "你们在一座海边小城暂时同住。",
    "first_mes": "我刚整理完今天的照片，你要一起看看吗？",
    "mes_example": "{{user}}: 今天拍到了什么？\\n{{char}}: 一张落日，还有一只一直跟着我的猫。",
    "creator_notes": "示例角色，可自由修改。",
    "system_prompt": "",
    "post_history_instructions": "",
    "alternate_greetings": [],
    "group_only_greetings": [],
    "character_book": {
      "name": "海边小城",
      "entries": []
    },
    "tags": ["日常", "治愈"],
    "creator": "示例作者",
    "character_version": "1.0",
    "extensions": {},
    "assets": [],
    "source": "Amitia",
    "creation_date": 0,
    "modification_date": 0
  }
}`,
  },
  {
    key: "tavern",
    label: "酒馆角色卡",
    description: "兼容酒馆常见 JSON 字段，可直接导入 JSON 或内嵌该 JSON 的 PNG。",
    json: `{
  "name": "阿澈",
  "description": "住在临海旧书店里的年轻店主，熟悉每一本书的来历。",
  "personality": "安静、可靠，观察细致，偶尔会开一点温和的玩笑。",
  "scenario": "傍晚的旧书店刚送走最后一位客人，窗外正在下雨。",
  "first_mes": "雨一时停不了，你要先喝杯热茶吗？",
  "mes_example": "{{user}}: 你在看什么？\\n{{char}}: 一本很久没人借走的航海日志。",
  "creatorcomment": "兼容酒馆角色卡的常见 JSON 字段。",
  "alternate_greetings": [],
  "tags": ["酒馆", "日常"],
  "creator": "示例作者",
  "character_version": "1.0",
  "extensions": {}
}`,
  },
];

const activeExampleKey = ref<JsonExampleKey>("v2");
const activeExample = computed(
  () =>
    jsonExamples.find((item) => item.key === activeExampleKey.value) ??
    jsonExamples[0],
);

function onFileChange(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0] || null;
  input.value = "";
  if (file) setCardFile(file);
}

function openFilePicker() {
  if (!props.previewing) fileInput.value?.click();
}

function onDragEnter() {
  dragDepth += 1;
  isDragActive.value = true;
}

function onDragLeave() {
  dragDepth = Math.max(0, dragDepth - 1);
  if (dragDepth === 0) isDragActive.value = false;
}

function onFileDrop(event: DragEvent) {
  dragDepth = 0;
  isDragActive.value = false;
  const file = event.dataTransfer?.files?.[0];
  if (file) setCardFile(file);
}

function setCardFile(file: File) {
  if (!/\.(json|png|charx)$/i.test(file.name)) {
    selectedFile.value = null;
    emit("update:packName", "");
    emit("fileSelected", null);
    ElMessage.warning("请选择 JSON、PNG 或 CHARX 角色卡文件");
    return;
  }
  selectedFile.value = file;
  emit("update:packName", file.name);
  emit("fileSelected", file);
}

function formatFileSize(value: number) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
}

async function copyExample() {
  try {
    if (!navigator.clipboard?.writeText) {
      throw new Error("Clipboard API unavailable");
    }
    await navigator.clipboard.writeText(activeExample.value.json);
    ElMessage.success("示例 JSON 已复制");
  } catch {
    ElMessage.warning("复制失败，请手动选择示例内容");
  }
}

const confirmTextModel = computed({
  get: () => props.confirmText,
  set: (v) => emit("update:confirmText", v),
});

const formatLabels: Record<string, string> = {
  v2_json: "角色卡 V2 JSON",
  v2_png: "角色卡 V2 PNG",
  v3_json: "角色卡 V3 JSON",
  v3_png: "角色卡 V3 PNG",
  v3_charx: "角色卡 V3 CHARX",
  tavern_json: "酒馆角色卡 JSON",
  tavern_png: "酒馆角色卡 PNG",
};

const previewFormatLabel = computed(
  () => formatLabels[props.preview?.format] || props.preview?.format || "",
);

const selectedFileSize = computed(() =>
  selectedFile.value ? formatFileSize(selectedFile.value.size) : "",
);

watch(
  () => props.preview,
  (preview) => {
    if (preview) return;
    selectedFile.value = null;
    isDragActive.value = false;
    dragDepth = 0;
  },
);
</script>

<style scoped>
.card-import-content {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.character-card-drop-zone {
  display: flex;
  min-height: 176px;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 22px;
  border: 1px dashed var(--ac-color-border);
  border-radius: var(--ac-radius-sm);
  background: var(--ac-color-bg-secondary);
  text-align: center;
  cursor: pointer;
  transition:
    border-color var(--ac-transition-fast),
    background var(--ac-transition-fast);
}

.character-card-drop-zone:hover,
.character-card-drop-zone:focus-visible,
.character-card-drop-zone.has-file {
  border-color: var(--el-color-primary-light-7);
  background: var(--el-color-primary-light-9);
}

.character-card-drop-zone.is-dragging {
  border-color: var(--el-color-primary);
  background: var(--el-color-primary-light-8);
  box-shadow: 0 0 0 3px var(--el-color-primary-light-9);
}

.character-card-drop-zone .upload-icon {
  margin-bottom: 4px;
  color: var(--el-color-primary);
  font-size: 34px;
}

.character-card-drop-zone strong {
  max-width: 100%;
  overflow: hidden;
  color: var(--ac-color-text);
  font-size: var(--ac-font-size-base);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.character-card-drop-zone span {
  margin-bottom: 6px;
  color: var(--ac-color-text-muted);
  font-size: var(--ac-font-size-xs);
  line-height: 1.5;
}

.import-primary-action {
  display: flex;
  justify-content: flex-end;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
}

.example-panel {
  padding: 12px;
  border: 1px solid var(--ac-color-border-light);
  border-radius: var(--ac-radius-sm);
  background: var(--ac-color-bg-secondary);
}

.example-panel-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.example-panel-header h3 {
  margin: 0;
  font-size: var(--ac-font-size-sm);
  font-weight: 600;
  color: var(--ac-color-text);
}

.example-panel-header p,
.example-description {
  margin: 4px 0 0;
  font-size: var(--ac-font-size-xs);
  line-height: 1.5;
  color: var(--ac-color-text-muted);
}

.example-switcher {
  margin-top: 10px;
}

.example-description {
  margin-bottom: 8px;
}

.example-code {
  max-height: 260px;
  margin: 0;
  padding: 12px;
  overflow: auto;
  border: 1px solid var(--ac-color-border-light);
  border-radius: var(--ac-radius-sm);
  background: var(--ac-color-surface);
  color: var(--ac-color-text);
  font-family: var(--ac-font-family-mono);
  font-size: var(--ac-font-size-xs);
  line-height: 1.6;
  white-space: pre;
}

@media (max-width: 640px) {
  .example-panel-header {
    align-items: center;
  }

  .example-switcher {
    display: flex;
    width: 100%;
  }

  .example-switcher :deep(.el-radio-button) {
    flex: 1;
  }

  .example-switcher :deep(.el-radio-button__inner) {
    width: 100%;
    padding-inline: 6px;
  }
}
</style>

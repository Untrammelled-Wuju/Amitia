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
      <el-form label-position="top">
        <el-form-item label="角色卡片文件">
          <input
            type="file"
            accept=".json,.png,.charx"
            @change="onFileChange"
            style="width: 100%"
          />
          <div class="form-hint" style="margin-top: 4px">
            支持 V2/V3 JSON、PNG、CHARX 格式
          </div>
        </el-form-item>
        <el-form-item>
          <el-button
            type="primary"
            :loading="previewing"
            @click="emit('preview')"
            :disabled="!selectedFile"
          >
            预览
          </el-button>
        </el-form-item>
      </el-form>
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
          <span class="ipi-label">格式</span><span>{{ preview.format }}</span>
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
import { computed, ref } from "vue";

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

function onFileChange(event: Event) {
  const target = event.target as HTMLInputElement;
  const file = target.files?.[0] || null;
  selectedFile.value = file;
  if (file) {
    emit("update:packName", file.name);
    emit("fileSelected", file);
  }
}

const confirmTextModel = computed({
  get: () => props.confirmText,
  set: (v) => emit("update:confirmText", v),
});
</script>

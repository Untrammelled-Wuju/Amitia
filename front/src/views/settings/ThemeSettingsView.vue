<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="theme-page">
    <h2 class="page-title">主题设置</h2>

    <el-card shadow="never" class="section-card">
      <template #header><span class="section-title">预设主题</span></template>
      <div class="theme-preset-list">
        <div
          v-for="p in presets"
          :key="p.id"
          class="theme-preset-item"
          :class="{ active: themeState.preset === p.id }"
          @click="setPreset(p.id)"
        >
          <div class="theme-preset-preview" :class="'preview-' + p.id"></div>
          <div class="theme-preset-info">
            <span class="theme-preset-name">{{ p.name }}</span>
            <span class="theme-preset-desc">{{ p.description }}</span>
          </div>
          <el-icon
            v-if="themeState.preset === p.id"
            color="var(--ac-color-primary)"
            ><Check
          /></el-icon>
        </div>
      </div>
    </el-card>

    <el-card shadow="never" class="section-card">
      <template #header
        ><span class="section-title">当前效果预览</span></template
      >
      <div class="preview-section">
        <div class="preview-row">
          <div
            class="preview-swatch"
            :style="{ background: 'var(--ac-color-primary)' }"
          ></div>
          <span class="preview-label">主色调</span>
          <span class="preview-val">{{
            themeState.accentColor || "默认"
          }}</span>
        </div>
        <div class="preview-row">
          <div
            class="preview-swatch"
            :style="{ background: 'var(--ac-color-surface)' }"
          ></div>
          <span class="preview-label">表面色</span>
        </div>
        <div class="preview-row">
          <div
            class="preview-swatch"
            :style="{ background: 'var(--ac-color-text)' }"
          ></div>
          <span class="preview-label">文本色</span>
        </div>
        <div class="preview-row">
          <div
            class="preview-swatch"
            :style="{ background: 'var(--ac-color-success)' }"
          ></div>
          <span class="preview-label">成功色</span>
        </div>
        <div class="preview-row">
          <div
            class="preview-swatch"
            :style="{ background: 'var(--ac-color-danger)' }"
          ></div>
          <span class="preview-label">危险色</span>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { Check } from "@element-plus/icons-vue";
import { useTheme } from "../../composables/useTheme";

const { state: themeState, setPreset, presets } = useTheme();
</script>

<style scoped>
.theme-page {
}
.page-title {
  font-size: var(--ac-font-size-lg);
  font-weight: 600;
  margin-bottom: 14px;
}
.section-card {
  margin-bottom: 14px;
}
.section-title {
  font-weight: 600;
  font-size: var(--ac-font-size-sm);
}

.theme-preset-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.theme-preset-item {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 12px 16px;
  border: 2px solid var(--ac-color-border-light);
  border-radius: var(--ac-radius-md);
  cursor: pointer;
  transition: all 0.2s;
  background: var(--ac-color-surface);
}
.theme-preset-item:hover {
  border-color: var(--ac-color-primary);
  background: var(--ac-color-surface-hover);
}
.theme-preset-item.active {
  border-color: var(--ac-color-primary);
  background: var(--ac-color-primary-bg);
}
.theme-preset-preview {
  width: 48px;
  height: 48px;
  border-radius: 8px;
  flex-shrink: 0;
  border: 1px solid var(--ac-color-border);
}
.preview-light {
  background: linear-gradient(135deg, #f4f1ea 0%, #faf8f3 52%, #9b642d 100%);
}
.preview-dark {
  background: linear-gradient(135deg, #0b0b0c 0%, #18181b 58%, #c99557 100%);
}
.preview-system {
  background: linear-gradient(135deg, #f4f1ea 0 50%, #0b0b0c 50% 100%);
}
.theme-preset-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.theme-preset-name {
  font-size: var(--ac-font-size-sm);
  font-weight: 500;
  color: var(--ac-color-text);
}
.theme-preset-desc {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
}

.preview-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.preview-row {
  display: flex;
  align-items: center;
  gap: 12px;
}
.preview-swatch {
  width: 24px;
  height: 24px;
  border-radius: 4px;
  border: 1px solid var(--ac-color-border);
}
.preview-label {
  font-size: 13px;
  color: var(--ac-color-text-secondary);
  min-width: 64px;
}
.preview-val {
  font-size: 13px;
  color: var(--ac-color-text-muted);
}
</style>

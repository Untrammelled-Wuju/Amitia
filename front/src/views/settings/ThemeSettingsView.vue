<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="theme-page">
    <h2 class="page-title">外观设置</h2>

    <el-card shadow="never" class="section-card">
      <template #header><span class="section-title">主题模式</span></template>
      <div class="theme-preset-list">
        <button
          v-for="p in presets"
          :key="p.id"
          type="button"
          class="theme-preset-item"
          :class="{ active: themeState.preset === p.id }"
          @click="setPreset(p.id)"
        >
          <span class="theme-preset-preview" :class="'preview-' + p.id"></span>
          <span class="theme-preset-info">
            <span class="theme-preset-name">{{ p.name }}</span>
            <span class="theme-preset-desc">{{ p.description }}</span>
          </span>
          <el-icon v-if="themeState.preset === p.id" color="var(--ac-color-primary)">
            <Check />
          </el-icon>
        </button>
      </div>
    </el-card>

    <el-card shadow="never" class="section-card">
      <template #header><span class="section-title">字体大小</span></template>
      <div class="option-grid option-grid--four">
        <el-button
          v-for="option in fontScaleOptions"
          :key="option.value"
          :type="sameScale(themeState.fontScale, option.value) ? 'primary' : 'default'"
          :plain="!sameScale(themeState.fontScale, option.value)"
          @click="setFontScale(option.value)"
        >
          {{ option.label }}
        </el-button>
      </div>
    </el-card>

    <el-card shadow="never" class="section-card">
      <template #header><span class="section-title">主题色调</span></template>
      <div class="accent-list">
        <button
          v-for="option in accentColorOptions"
          :key="option.value"
          type="button"
          class="accent-option"
          :class="{ active: sameAccent(themeState.accentColor, option.value) }"
          :aria-label="`使用${option.label}主题色`"
          :title="option.label"
          @click="setAccentColor(option.value)"
        >
          <span class="accent-dot" :style="{ backgroundColor: option.value }">
            <el-icon v-if="sameAccent(themeState.accentColor, option.value)"><Check /></el-icon>
          </span>
          <span>{{ option.label }}</span>
        </button>
      </div>
    </el-card>

    <el-card shadow="never" class="section-card">
      <template #header><span class="section-title">圆角风格</span></template>
      <div class="option-grid option-grid--three">
        <el-button
          v-for="option in cornerStyleOptions"
          :key="option.value"
          :type="themeState.cornerStyle === option.value ? 'primary' : 'default'"
          :plain="themeState.cornerStyle !== option.value"
          @click="setCornerStyle(option.value)"
        >
          {{ option.label }}
        </el-button>
      </div>
    </el-card>

    <el-card shadow="never" class="section-card">
      <template #header><span class="section-title">其他</span></template>
      <div class="switch-list">
        <div class="switch-row">
          <div>
            <div class="switch-title">动态效果</div>
            <div class="switch-description">启用页面过渡和微交互动画</div>
          </div>
          <el-switch
            :model-value="themeState.dynamicEffect"
            @update:model-value="setDynamicEffect"
          />
        </div>
        <div class="switch-row">
          <div>
            <div class="switch-title">减少动画</div>
            <div class="switch-description">降低界面动画强度以提升性能</div>
          </div>
          <el-switch
            :model-value="themeState.reduceAnimation"
            @update:model-value="setReduceAnimation"
          />
        </div>
      </div>
    </el-card>

    <el-card shadow="never" class="section-card">
      <template #header><span class="section-title">当前效果预览</span></template>
      <div class="preview-section">
        <div class="preview-row">
          <div class="preview-swatch" :style="{ background: 'var(--ac-color-primary)' }"></div>
          <span class="preview-label">主色调</span>
          <span class="preview-val">{{ themeState.accentColor }}</span>
        </div>
        <div class="preview-row">
          <div class="preview-swatch" :style="{ background: 'var(--ac-color-surface)' }"></div>
          <span class="preview-label">表面色</span>
        </div>
        <div class="preview-row">
          <div class="preview-swatch" :style="{ background: 'var(--ac-color-text)' }"></div>
          <span class="preview-label">文本色</span>
        </div>
        <div class="preview-row">
          <div class="preview-swatch" :style="{ background: 'var(--ac-color-success)' }"></div>
          <span class="preview-label">成功色</span>
        </div>
        <div class="preview-row">
          <div class="preview-swatch" :style="{ background: 'var(--ac-color-danger)' }"></div>
          <span class="preview-label">危险色</span>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { Check } from "@element-plus/icons-vue";
import { useTheme } from "../../composables/useTheme";

const {
  state: themeState,
  setPreset,
  setFontScale,
  setAccentColor,
  setCornerStyle,
  setDynamicEffect,
  setReduceAnimation,
  presets,
  fontScaleOptions,
  accentColorOptions,
  cornerStyleOptions,
} = useTheme();

function sameScale(left: number, right: number) {
  return Math.abs(left - right) < 0.001;
}

function sameAccent(left: string, right: string) {
  return left.trim().toUpperCase() === right.trim().toUpperCase();
}
</script>

<style scoped>
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
  width: 100%;
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 12px 16px;
  border: 2px solid var(--ac-color-border-light);
  border-radius: var(--ac-radius-md);
  cursor: pointer;
  transition: border-color var(--ac-transition-fast), background var(--ac-transition-fast);
  background: var(--ac-color-surface);
  color: inherit;
  text-align: left;
  font: inherit;
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
  display: block;
  width: 48px;
  height: 48px;
  border-radius: var(--ac-radius-sm);
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
.theme-preset-name,
.switch-title {
  font-size: var(--ac-font-size-sm);
  font-weight: 500;
  color: var(--ac-color-text);
}
.theme-preset-desc,
.switch-description {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
}

.option-grid {
  display: grid;
  gap: 8px;
}
.option-grid--four {
  grid-template-columns: repeat(4, minmax(0, 1fr));
}
.option-grid--three {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}
.option-grid .el-button {
  width: 100%;
  margin: 0;
}

.accent-list {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}
.accent-option {
  min-width: 92px;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border: 1px solid var(--ac-color-border-light);
  border-radius: var(--ac-radius-md);
  background: var(--ac-color-surface);
  color: var(--ac-color-text-secondary);
  cursor: pointer;
  font: inherit;
}
.accent-option:hover,
.accent-option.active {
  border-color: var(--ac-color-primary);
  background: var(--ac-color-primary-bg);
}
.accent-dot {
  width: 28px;
  height: 28px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  color: #fff;
}

.switch-list {
  display: flex;
  flex-direction: column;
}
.switch-row {
  min-height: 56px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 8px 0;
}
.switch-row + .switch-row {
  border-top: 1px solid var(--ac-color-border-light);
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
  border-radius: var(--ac-radius-sm);
  border: 1px solid var(--ac-color-border);
}
.preview-label {
  font-size: var(--ac-font-size-sm);
  color: var(--ac-color-text-secondary);
  min-width: 64px;
}
.preview-val {
  font-size: var(--ac-font-size-sm);
  color: var(--ac-color-text-muted);
}

@media (max-width: 720px) {
  .option-grid--four,
  .option-grid--three {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>

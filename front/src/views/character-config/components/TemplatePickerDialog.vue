<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-dialog
    v-model="show"
    title="从模板创建角色"
    width="min(820px, calc(100vw - 32px))"
    top="5vh"
    align-center
  >
    <div v-if="loading" class="state-panel">
      <el-icon class="is-loading" :size="32"><Loading /></el-icon>
      <p>加载模板中...</p>
    </div>
    <div v-else-if="templates.length === 0" class="state-panel">
      <el-empty description="暂无可用模板" :image-size="60" />
    </div>
    <div v-else class="template-grid">
      <div
        v-for="tpl in templates"
        :key="tpl.id"
        class="template-card"
        role="button"
        tabindex="0"
        @click="emit('select', tpl)"
        @keydown.enter.prevent="emit('select', tpl)"
        @keydown.space.prevent="emit('select', tpl)"
      >
        <div class="tpl-card-header">
          <span class="tpl-card-name">{{ tpl.name }}</span>
          <el-tag
            v-if="tpl.hasSafeBoundaries"
            type="success"
            size="small"
            effect="plain"
            >已审查</el-tag
          >
        </div>
        <div class="tpl-card-scenario">{{ tpl.scenario }}</div>
        <div class="tpl-card-details">
          <div class="tpl-detail-row">
            <span class="tpl-detail-label">说话风格</span>
            <span class="tpl-detail-value">{{ tpl.speakingStyle }}</span>
          </div>
          <div class="tpl-detail-row">
            <span class="tpl-detail-label">关系氛围</span>
            <span class="tpl-detail-value">{{ tpl.relationshipStyle }}</span>
          </div>
        </div>
      </div>
    </div>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { Loading } from "@element-plus/icons-vue";
import type { TemplateItem } from "../composables/useCharacterConfig";

const props = defineProps<{
  modelValue: boolean;
  templates: TemplateItem[];
  loading: boolean;
}>();
const emit = defineEmits<{
  (e: "update:modelValue", v: boolean): void;
  (e: "select", tpl: TemplateItem): void;
}>();
const show = computed({
  get: () => props.modelValue,
  set: (v) => emit("update:modelValue", v),
});
</script>

<style scoped>
.state-panel {
  min-height: 220px;
  display: grid;
  place-items: center;
  align-content: center;
  gap: 10px;
  padding: 32px;
  text-align: center;
}

.state-panel p {
  margin: 0;
  color: var(--ac-color-text-muted);
}

.template-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 280px), 1fr));
  gap: 14px;
  max-height: min(64vh, 620px);
  overflow-y: auto;
  padding: 2px 4px 8px 2px;
}

.template-card {
  min-width: 0;
  min-height: 168px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px;
  border: 1px solid var(--ac-color-border);
  border-radius: 12px;
  background: var(--ac-color-surface);
  cursor: pointer;
  transition:
    border-color 160ms ease,
    background-color 160ms ease,
    transform 160ms ease;
}

.template-card:hover,
.template-card:focus-visible {
  border-color: var(--el-color-primary-light-5);
  background: var(--el-color-primary-light-9);
  outline: none;
  transform: translateY(-1px);
}

.tpl-card-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
}

.tpl-card-name {
  min-width: 0;
  overflow: hidden;
  color: var(--ac-color-text);
  font-size: 15px;
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tpl-card-header :deep(.el-tag) {
  flex-shrink: 0;
}

.tpl-card-scenario {
  flex: 1;
  display: -webkit-box;
  overflow: hidden;
  color: var(--ac-color-text-secondary);
  font-size: 13px;
  line-height: 1.55;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
}

.tpl-card-details {
  display: grid;
  gap: 6px;
  padding-top: 10px;
  border-top: 1px solid var(--ac-color-border-light);
}

.tpl-detail-row {
  display: grid;
  grid-template-columns: 64px minmax(0, 1fr);
  gap: 8px;
  font-size: 12px;
}

.tpl-detail-label {
  color: var(--ac-color-text-muted);
}

.tpl-detail-value {
  min-width: 0;
  overflow: hidden;
  color: var(--ac-color-text-secondary);
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 680px) {
  .template-grid {
    grid-template-columns: 1fr;
    max-height: 68vh;
  }
}

@media (prefers-reduced-motion: reduce) {
  .template-card {
    transition: none;
  }
}
</style>

<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <aside class="char-sidebar">
    <div class="sidebar-header">
      <h3>角色列表</h3>
      <el-button
        :icon="Plus"
        size="small"
        type="primary"
        @click="emit('create')"
        >新建</el-button
      >
    </div>

    <div class="templates-section">
      <el-button
        plain
        :icon="Plus"
        size="small"
        @click="emit('openTemplates')"
        style="width: 100%"
      >
        从模板创建
      </el-button>
    </div>

    <div class="divider"></div>

    <div class="char-list">
      <div v-if="creating" class="char-list-item active draft-item">
        <div class="cli-main">
          <el-avatar :size="28">
            <el-icon><Plus /></el-icon>
          </el-avatar>
          <span class="cli-name">新建角色</span>
          <el-tag type="warning" size="small" effect="plain">未保存</el-tag>
        </div>
      </div>
      <div
        v-for="c in characters"
        v-memo="[c.id, c.name, c.avatar, c.isActive, selectedId === c.id]"
        :key="c.id"
        class="char-list-item"
        :class="{ active: selectedId === c.id, 'is-active': c.isActive }"
        @click="emit('select', c)"
      >
        <div class="cli-main">
          <el-avatar :size="28" :src="c.avatar || undefined">{{
            c.name?.charAt(0)
          }}</el-avatar>
          <span class="cli-name">{{ c.name }}</span>
          <el-tag v-if="c.isActive" type="success" size="small" effect="dark"
            >当前</el-tag
          >
        </div>
        <div class="cli-actions" v-if="selectedId === c.id">
          <el-button
            text
            size="small"
            @click.stop="emit('copy', c)"
            title="复制"
          >
            <el-icon><CopyDocument /></el-icon>
          </el-button>
          <el-button
            text
            size="small"
            type="danger"
            @click.stop="emit('delete', c)"
            title="删除"
          >
            <el-icon><Delete /></el-icon>
          </el-button>
        </div>
      </div>
      <el-empty
        v-if="characters.length === 0"
        description="还没有角色"
        :image-size="50"
      />
    </div>
  </aside>
</template>

<script setup lang="ts">
import { Plus, CopyDocument, Delete } from "@element-plus/icons-vue";
defineProps<{ characters: any[]; selectedId: string; creating: boolean }>();
const emit = defineEmits<{
  (e: "create"): void;
  (e: "openTemplates"): void;
  (e: "select", c: any): void;
  (e: "copy", c: any): void;
  (e: "delete", c: any): void;
}>();
</script>

<style scoped>
.char-sidebar {
  min-height: 0;
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid var(--ac-color-border);
  border-radius: 14px;
  background: var(--ac-color-surface);
  box-shadow: var(--ac-shadow-sm);
}

.sidebar-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 14px 10px;
}

.sidebar-header h3 {
  min-width: 0;
  margin: 0;
  overflow: hidden;
  color: var(--ac-color-text);
  font-size: 15px;
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.templates-section {
  padding: 0 14px;
}

.divider {
  height: 1px;
  margin: 12px 14px 0;
  background: var(--ac-color-border-light);
}

.char-list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 8px;
}

.char-list-item {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 6px;
  min-height: 52px;
  padding: 8px 8px 8px 10px;
  border: 1px solid transparent;
  border-radius: 10px;
  cursor: pointer;
  transition:
    border-color 160ms ease,
    background-color 160ms ease;
}

.char-list-item + .char-list-item {
  margin-top: 4px;
}

.char-list-item:hover {
  background: var(--ac-color-surface-soft);
}

.char-list-item.active {
  border-color: var(--el-color-primary-light-5);
  background: var(--el-color-primary-light-9);
}

.draft-item {
  cursor: default;
}

.char-list-item.is-active .cli-name {
  color: var(--el-color-primary);
  font-weight: 600;
}

.cli-main {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.cli-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  color: var(--ac-color-text);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cli-actions {
  display: flex;
  align-items: center;
  gap: 0;
  opacity: 0;
  transition: opacity 150ms ease;
}

.char-list-item:hover .cli-actions,
.char-list-item.active .cli-actions,
.char-list-item:focus-within .cli-actions {
  opacity: 1;
}

.char-list :deep(.el-empty) {
  padding: 28px 8px;
}

@media (prefers-reduced-motion: reduce) {
  .char-list-item,
  .cli-actions {
    transition: none;
  }
}
</style>

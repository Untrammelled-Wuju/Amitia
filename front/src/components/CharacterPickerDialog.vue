<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-dialog :model-value="visible" @update:model-value="$emit('update:visible', $event)" title="切换角色" width="400px">
    <div class="char-list">
      <div
        v-for="c in characters"
        :key="c.id"
        class="char-option"
        :class="{ active: c.id === characterId }"
        @click="$emit('select', c)"
      >
        <el-avatar :size="36">{{ c.name?.charAt(0) }}</el-avatar>
        <div class="char-option-info">
          <div class="char-option-name">{{ c.name }}</div>
          <div class="char-option-desc">{{ c.identity || c.personality }}</div>
        </div>
        <el-tag v-if="!!c.isDefault" size="small" type="success" effect="plain">默认角色</el-tag>
      </div>
    </div>
  </el-dialog>
</template>

<script setup lang="ts">
defineProps<{
  visible: boolean
  characters: any[]
  characterId: string
}>()

defineEmits<{
  "update:visible": [value: boolean]
  select: [char: any]
}>()
</script>

<style scoped>
.char-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-height: 400px;
  overflow-y: auto;
}

.char-option {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px;
  border-radius: var(--ac-radius-sm);
  cursor: pointer;
  transition: background var(--ac-transition-fast);
}

.char-option:hover {
  background: var(--ac-color-surface-hover);
}

.char-option.active {
  background: var(--ac-color-primary-bg);
}

.char-option-info {
  flex: 1;
  min-width: 0;
}

.char-option-name {
  font-size: var(--ac-font-size-sm);
  font-weight: 500;
}

.char-option-desc {
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>

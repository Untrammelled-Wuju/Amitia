<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-drawer :model-value="visible" @update:model-value="$emit('update:visible', $event)" title="相关记忆" direction="rtl" size="360px">
    <div v-if="memories.length > 0">
      <div v-for="m in memories" :key="m.key" class="memory-card">
        <div class="card-tags">
          <el-tag size="small" type="info">{{ typeLabel(m.memoryType) }}</el-tag>
          <el-tag v-if="m.confidence >= 80" size="small" type="success">高置信</el-tag>
          <el-tag v-else-if="m.confidence >= 50" size="small" type="warning">中置信</el-tag>
          <el-tag v-else size="small" type="danger">低置信</el-tag>
          <el-tag v-if="m.verifiedStatus === 'user_verified'" size="small" type="success" effect="dark">✓</el-tag>
          <el-tag v-else-if="m.verifiedStatus === 'contradicted'" size="small" type="danger" effect="dark">⚠</el-tag>
        </div>
        <div class="memory-key">{{ m.key }}</div>
        <div class="memory-value">{{ m.value }}</div>
        <div v-if="m.expiresAt" class="memory-expiry">过期: {{ m.expiresAt }}</div>
      </div>
    </div>
    <el-empty v-else description="暂无相关记忆" :image-size="60" />
  </el-drawer>
</template>

<script setup lang="ts">
defineProps<{
  visible: boolean
  memories: any[]
}>()

defineEmits<{
  "update:visible": [value: boolean]
}>()

function typeLabel(type: string): string {
  const labels: Record<string, string> = {
    preference: "偏好",
    event: "事件",
    habit: "习惯",
    nickname: "昵称",
    relationship: "关系",
    custom: "其他",
  }
  return labels[type] || type
}
</script>

<style scoped>
.memory-card {
  padding: 10px;
  margin-bottom: 8px;
  border-radius: var(--ac-radius-sm);
  background: var(--ac-color-bg-secondary);
}

.memory-key {
  font-size: var(--ac-font-size-sm);
  font-weight: 500;
  margin: 4px 0;
}

.card-tags { display: flex; gap: 4px; flex-wrap: wrap; margin-bottom: 4px; }
.memory-value {
  font-size: var(--ac-font-size-sm);
  color: var(--ac-color-text-secondary);
}
</style>

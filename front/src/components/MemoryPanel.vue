<template>
  <el-drawer :model-value="visible" @update:model-value="$emit('update:visible', $event)" title="相关记忆" direction="rtl" size="360px">
    <div v-if="memories.length > 0">
      <div v-for="m in memories" :key="m.key" class="memory-card">
        <el-tag size="small" type="info">{{ typeLabel(m.memoryType) }}</el-tag>
        <div class="memory-key">{{ m.key }}</div>
        <div class="memory-value">{{ m.value }}</div>
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

.memory-value {
  font-size: var(--ac-font-size-sm);
  color: var(--ac-color-text-secondary);
}
</style>

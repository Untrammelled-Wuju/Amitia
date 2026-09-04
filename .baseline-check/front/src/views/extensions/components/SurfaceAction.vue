<template>
  <div class="surface-action">
    <p>{{ section.label || section.title || section.id }}</p>
    <el-button
      type="primary"
      :loading="running"
      @click="$emit('run', section.id)"
      >执行</el-button
    >
  </div>
</template>

<script setup lang="ts">
import type { SurfaceSection } from "../types";
defineProps<{ section: SurfaceSection; running?: boolean }>();
defineEmits<{ run: [actionId: string] }>();
</script>

<style scoped>
.surface-action {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}
</style>

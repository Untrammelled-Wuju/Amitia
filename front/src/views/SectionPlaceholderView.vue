<template>
  <main class="section-placeholder">
    <header>
      <h1>{{ title }}</h1>
      <p>{{ description }}</p>
    </header>
    <el-empty description="请从左侧选择具体功能" :image-size="88" />
  </main>
</template>

<script setup lang="ts">
defineProps<{
  title: string;
  description: string;
}>();
</script>

<style scoped>
.section-placeholder {
  min-height: 100%;
  padding: 4px;
  color: var(--text-primary);
}
header h1 {
  margin: 0 0 6px;
  font-size: 24px;
  font-weight: 650;
}
header p {
  margin: 0;
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.6;
}
.section-placeholder :deep(.el-empty) {
  min-height: 360px;
}
</style>

<template>
  <div class="my-space-runtime">
    <DeploymentPanel />
  </div>
</template>

<script setup lang="ts">
import DeploymentPanel from "@/views/settings/DeploymentPanel.vue";
</script>

<style scoped>
.my-space-runtime { width: min(100%, 1040px); min-width: 0; display: grid; gap: 18px; }
.my-space-runtime :deep(.deployment-panel) { display: grid; gap: 18px; }
.my-space-runtime :deep(.section-card) { margin: 0; border: 0; border-radius: 20px; background: var(--surface-bg-elevated); box-shadow: none; }
.my-space-runtime :deep(.section-card .el-card__header) { padding: 22px 24px 8px; border-bottom: 0; }
.my-space-runtime :deep(.section-card .el-card__body) { padding: 16px 24px 24px; }
.my-space-runtime :deep(.mode-radio-card) { border: 0 !important; background: var(--workbench-bg) !important; box-shadow: none !important; }
.my-space-runtime :deep(.mode-radio-card.is-checked) { background: color-mix(in srgb, var(--ac-color-primary) 12%, var(--workbench-bg)) !important; }
.my-space-runtime :deep(.server-url-section) { border-top: 0; }
.my-space-runtime :deep(.mode-save-row) { border-top: 0; }
</style>

<template>
  <div class="my-space-runtime">
    <section class="runtime-intro">
      <div class="runtime-intro-icon"><Connection class="runtime-intro-svg" /></div>
      <div class="runtime-intro-copy">
        <h2>连接与运行</h2>
        <p>选择本地模式或完成 Cloud Core 配对后进入云端统筹模式。</p>
      </div>
      <div class="runtime-flow" aria-label="连接层级">
        <span>当前设备</span>
        <i></i>
        <span>Core 服务</span>
        <i></i>
        <span>个人空间</span>
      </div>
    </section>

    <DeploymentPanel />
  </div>
</template>

<script setup lang="ts">
import { Connection } from "@element-plus/icons-vue";
import DeploymentPanel from "@/views/settings/DeploymentPanel.vue";
</script>

<style scoped>
.my-space-runtime { width: min(100%, 1040px); min-width: 0; display: grid; gap: 18px; }
.runtime-intro { display: grid; grid-template-columns: auto minmax(0, 1fr) auto; align-items: center; gap: 18px; padding: 28px; border-radius: 24px; background: #2C2C2E; }
.runtime-intro-icon { width: 56px; height: 56px; display: grid; place-items: center; border-radius: 16px; background: color-mix(in srgb, var(--ac-color-primary) 12%, var(--control-hover-bg)); color: var(--ac-color-primary); }
.runtime-intro-svg { width: 23px; height: 23px; }
.runtime-intro-copy { min-width: 0; }
.runtime-intro h2 { margin: 0; font-size: 20px; line-height: 1.35; }
.runtime-intro p { margin: 6px 0 0; color: var(--text-secondary); font-size: 13px; line-height: 1.6; }
.runtime-flow { display: flex; align-items: center; gap: 8px; color: var(--text-secondary); font-size: 11px; white-space: nowrap; }
.runtime-flow span { padding: 5px 9px; border-radius: 7px; background: color-mix(in srgb, var(--workbench-bg) 68%, transparent); }
.runtime-flow i { width: 18px; height: 1px; background: color-mix(in srgb, var(--text-secondary) 32%, transparent); }
.my-space-runtime :deep(.deployment-panel) { display: grid; gap: 18px; }
.my-space-runtime :deep(.mode-summary) { margin: 0; border: 0; border-radius: 20px; background: var(--surface-bg-elevated); }
.my-space-runtime :deep(.section-card) { margin: 0; border: 0; border-radius: 20px; background: var(--surface-bg-elevated); box-shadow: none; }
.my-space-runtime :deep(.section-card .el-card__header) { padding: 22px 24px 8px; border-bottom: 0; }
.my-space-runtime :deep(.section-card .el-card__body) { padding: 16px 24px 24px; }
.my-space-runtime :deep(.mode-radio-card) { border: 0 !important; background: var(--workbench-bg) !important; box-shadow: none !important; }
.my-space-runtime :deep(.mode-radio-card.is-checked) { background: color-mix(in srgb, var(--ac-color-primary) 12%, var(--workbench-bg)) !important; }
.my-space-runtime :deep(.server-url-section) { border-top: 0; }
.my-space-runtime :deep(.mode-save-row) { border-top: 0; }
.my-space-runtime :deep(.deployment-advanced) { margin-top: 0; border: 0; border-radius: 16px; background: color-mix(in srgb, var(--surface-bg-elevated) 72%, transparent); }
.my-space-runtime :deep(.deployment-advanced .el-descriptions__cell) { border: 0; }
.my-space-runtime :deep(.el-descriptions__body), .my-space-runtime :deep(.el-descriptions__table) { background: transparent; }
.my-space-runtime :deep(.el-descriptions__cell) { border: 0; }
@media (max-width: 860px) { .runtime-intro { grid-template-columns: auto minmax(0, 1fr); } .runtime-flow { grid-column: 1 / -1; flex-wrap: wrap; } }
@media (max-width: 560px) { .runtime-intro { padding: 20px; } .runtime-flow i { width: 10px; } }
</style>

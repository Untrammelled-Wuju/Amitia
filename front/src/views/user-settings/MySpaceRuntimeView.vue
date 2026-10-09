<template>
  <div class="my-space-runtime">
    <header class="space-page-header">
      <h2>运行模式</h2>
      <p>分别管理这台设备连接哪个 Core，以及当前 Core 自身如何提供服务。两项配置互不替代。</p>
    </header>
    <DeploymentPanel />
    <details class="runtime-advanced">
      <summary>Core 服务端设置 · 部署形态、Bridge 与配置校验</summary>
      <RuntimeModeView />
    </details>
  </div>
</template>

<script setup lang="ts">
import DeploymentPanel from "@/views/settings/DeploymentPanel.vue";
import RuntimeModeView from "@/views/runtime-mode/RuntimeModeView.vue";
</script>

<style scoped>
.my-space-runtime { min-width: 0; max-width: 880px; }
.space-page-header { margin-bottom: 24px; }
.space-page-header h2 { margin: 0; font-size: 23px; font-weight: 650; letter-spacing: -0.025em; }
.space-page-header p { margin: 7px 0 0; color: var(--text-secondary); font-size: 13px; line-height: 1.6; }
.runtime-advanced { border: 1px solid var(--surface-border); border-radius: 12px; margin-top: 18px; }
.runtime-advanced > summary { cursor: pointer; color: var(--text-secondary); padding: 18px 20px; font-size: 13px; }
.runtime-advanced[open] > summary { border-bottom: 1px solid var(--surface-border); }
.my-space-runtime :deep(.runtime-mode-page) { padding: 20px; max-width: none; }
.my-space-runtime :deep(.runtime-mode-page .page-title) { font-size: 15px; margin: 0 0 16px; }
</style>

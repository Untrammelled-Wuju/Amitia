<template>
  <div class="extension-loading-overlay" data-state="loading" role="status" aria-label="扩展界面加载中" aria-busy="true">
    <span class="extension-loading-overlay__spinner" aria-hidden="true"></span>
  </div>
</template>

<style scoped>
.extension-loading-overlay {
  position: absolute;
  inset: 0;
  z-index: 10;
  display: grid;
  place-items: center;
  border-radius: inherit;
  background: var(--amitia-bg-surface, var(--plugin-surface-bg, transparent));
}
.extension-loading-overlay__spinner {
  width: 32px;
  height: 32px;
  border: 3px solid var(--plugin-surface-border, rgba(127, 127, 127, 0.25));
  border-top-color: var(--amitia-color-accent, var(--el-color-primary, #409eff));
  border-radius: 50%;
  animation: extension-loading-spin 0.8s linear infinite;
}
@keyframes extension-loading-spin {
  to { transform: rotate(360deg); }
}
@media (prefers-reduced-motion: reduce) {
  .extension-loading-overlay__spinner { animation-duration: 2s; }
}
</style>

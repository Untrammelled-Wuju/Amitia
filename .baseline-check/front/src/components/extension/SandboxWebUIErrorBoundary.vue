<script setup lang="ts">
import { ref, onErrorCaptured } from "vue";

const error = ref<Error | null>(null);
const showDetails = ref(false);
const canShowDetails = import.meta.env.DEV;

onErrorCaptured((err: unknown) => {
  error.value = err instanceof Error ? err : new Error(String(err));
  return false;
});

function reset() {
  error.value = null;
  showDetails.value = false;
}
</script>

<template>
  <div v-if="error" class="error-boundary">
    <div class="error-boundary__icon">!</div>
    <div class="error-boundary__body">
      <div class="error-boundary__title">扩展组件发生错误</div>
      <div class="error-boundary__message">{{ error.message }}</div>
      <button v-if="canShowDetails && !showDetails" class="error-boundary__toggle" @click="showDetails = true">
        显示详情
      </button>
      <pre v-if="showDetails" class="error-boundary__details">{{ error.stack }}</pre>
    </div>
    <button class="error-boundary__reset" @click="reset">重试</button>
  </div>
  <slot v-else />
</template>

<style scoped>
.error-boundary { display: flex; align-items: flex-start; gap: 12px; padding: 12px; background: var(--ac-color-danger-bg); border: 1px solid color-mix(in srgb, var(--ac-color-danger) 32%, var(--plugin-surface-border)); border-radius: var(--radius-sm); }
.error-boundary__icon { display: flex; align-items: center; justify-content: center; width: 28px; height: 28px; border-radius: 50%; background: var(--ac-color-danger); color: var(--ac-color-text-on-primary); font-size: 14px; font-weight: bold; flex-shrink: 0; }
.error-boundary__body { flex: 1; min-width: 0; }
.error-boundary__title { font-size: 13px; font-weight: 600; color: var(--ac-color-danger); margin-bottom: 4px; }
.error-boundary__message { font-size: 12px; color: var(--ac-color-danger); word-break: break-word; }
.error-boundary__toggle { margin-top: 8px; padding: 2px 8px; font-size: 11px; color: var(--ac-color-danger); background: transparent; border: none; cursor: pointer; }
.error-boundary__details { margin-top: 8px; padding: 8px; background: var(--plugin-muted-bg); border-radius: var(--radius-xs); font-size: 11px; color: var(--text-secondary); overflow-x: auto; max-height: 200px; }
.error-boundary__reset { padding: 6px 12px; font-size: 12px; color: var(--ac-color-danger); background: transparent; border: 1px solid currentColor; border-radius: var(--radius-xs); cursor: pointer; flex-shrink: 0; }
.error-boundary__reset:hover { background: color-mix(in srgb, var(--ac-color-danger) 8%, transparent); }
</style>

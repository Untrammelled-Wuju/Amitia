<script setup lang="ts">
import { ref, onErrorCaptured } from "vue";

const error = ref<Error | null>(null);
const showDetails = ref(false);

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
      <button v-if="!showDetails" class="error-boundary__toggle" @click="showDetails = true">
        显示详情
      </button>
      <pre v-if="showDetails" class="error-boundary__details">{{ error.stack }}</pre>
    </div>
    <button class="error-boundary__reset" @click="reset">重试</button>
  </div>
  <slot v-else />
</template>

<style scoped>
.error-boundary { display: flex; align-items: flex-start; gap: 12px; padding: 16px; background: #fef2f2; border: 1px solid #fca5a5; border-radius: 8px; }
.error-boundary__icon { display: flex; align-items: center; justify-content: center; width: 32px; height: 32px; border-radius: 50%; background: #dc2626; color: white; font-size: 18px; font-weight: bold; flex-shrink: 0; }
.error-boundary__body { flex: 1; min-width: 0; }
.error-boundary__title { font-size: 14px; font-weight: 600; color: #991b1b; margin-bottom: 4px; }
.error-boundary__message { font-size: 13px; color: #b91c1c; word-break: break-word; }
.error-boundary__toggle { margin-top: 8px; padding: 2px 8px; font-size: 11px; color: #4a6cf7; background: transparent; border: none; cursor: pointer; }
.error-boundary__details { margin-top: 8px; padding: 8px; background: #f5f5f5; border-radius: 4px; font-size: 11px; color: #666; overflow-x: auto; max-height: 200px; }
.error-boundary__reset { padding: 6px 16px; font-size: 12px; color: #4a6cf7; background: white; border: 1px solid #ddd; border-radius: 4px; cursor: pointer; flex-shrink: 0; }
.error-boundary__reset:hover { background: #f5f5f5; }
</style>

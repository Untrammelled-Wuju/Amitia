<script setup lang="ts">
import { ref, computed, onErrorCaptured, onUnmounted } from "vue";
import { useExtensionUIStore } from "@/stores/extensionUI";

const props = withDefaults(
  defineProps<{
    slotId: string;
    contributionId: string;
    maxRetries?: number;
  }>(),
  {
    maxRetries: 3,
  }
);

const uiStore = useExtensionUIStore();

const error = ref<string | null>(null);
const retryCount = ref<number>(0);
const renderKey = ref<number>(0);

const hasError = computed(() => error.value !== null);
const canRetry = computed(() => retryCount.value < props.maxRetries);
const showDebug = import.meta.env.DEV;

onErrorCaptured((err: unknown) => {
  const message = err instanceof Error ? err.message : String(err);
  error.value = message;
  try {
    uiStore.recordError({
      contributionId: props.contributionId,
      slotId: props.slotId,
      message,
      timestamp: Date.now(),
      recoverable: true,
    });
  } catch {
  }
  return false;
});

function retry(): void {
  if (!canRetry.value) return;
  retryCount.value += 1;
  error.value = null;
  renderKey.value += 1;
}

onUnmounted(() => {
  error.value = null;
});
</script>

<template>
  <div class="extension-error-boundary" :data-slot-id="slotId" :data-contribution-id="contributionId">
    <template v-if="hasError">
      <div class="extension-error-boundary__error">
        <div class="extension-error-boundary__error-title">扩展渲染异常</div>
        <div class="extension-error-boundary__error-detail">{{ error }}</div>
        <div v-if="showDebug" class="extension-error-boundary__error-meta">
          槽位: {{ slotId }} · 贡献: {{ contributionId }} · 已重试: {{ retryCount }}/{{ maxRetries }}
        </div>
        <button
          v-if="canRetry"
          class="extension-error-boundary__retry"
          @click="retry"
        >
          重试
        </button>
        <div v-else class="extension-error-boundary__exceeded">
          错误次数过多，请刷新页面
        </div>
      </div>
    </template>
    <template v-else>
      <div :key="renderKey" class="extension-error-boundary__content">
        <slot />
      </div>
    </template>
  </div>
</template>

<style scoped>
.extension-error-boundary {
  width: 100%;
  min-width: 0;
  display: flex;
  flex-direction: column;
  color: var(--amitia-text-primary, var(--amitia-color-text, inherit));
  background: var(--amitia-bg-surface, var(--amitia-color-surface, transparent));
}
.extension-error-boundary__content {
  width: 100%;
  min-width: 0;
  display: flex;
  flex-direction: column;
}
.extension-error-boundary__error {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 10px 12px;
  border-radius: var(--radius-sm);
  background: var(--ac-color-danger-bg);
  border: 1px solid color-mix(in srgb, var(--ac-color-danger) 32%, var(--plugin-surface-border));
  color: var(--ac-color-danger);
  font-size: 12px;
}
.extension-error-boundary__error-title {
  font-weight: 600;
  font-size: 13px;
}
.extension-error-boundary__error-detail {
  opacity: 0.9;
  word-break: break-word;
}
.extension-error-boundary__error-meta {
  opacity: 0.7;
  font-size: 11px;
}
.extension-error-boundary__retry {
  align-self: flex-start;
  padding: 3px 10px;
  border: 1px solid currentColor;
  border-radius: var(--radius-xs);
  background: transparent;
  color: inherit;
  font-size: 12px;
  cursor: pointer;
}
.extension-error-boundary__retry:hover {
  background: rgba(220, 60, 60, 0.1);
}
.extension-error-boundary__exceeded {
  padding: 4px 0;
  font-size: 12px;
  opacity: 0.85;
}
</style>

<script setup lang="ts">
import { computed, ref } from "vue";

const props = defineProps<{
  code: string;
  summary: string;
  raw: string;
  retryable?: boolean;
  retryLabel?: string;
}>();

const emit = defineEmits<{
  (e: "retry"): void;
}>();

const CODE_LABELS: Record<string, string> = {
  permission_denied: "权限被拒绝",
  scope_denied: "作用域被拒绝",
  identity_invalid: "身份校验失败",
  action_timeout: "请求超时",
  render_timeout: "渲染超时",
  action_cancelled: "请求已取消",
  upstream_error: "上游服务错误",
  session_unavailable: "插件会话不可用",
  session_restarted: "插件会话已重建",
  runtime_unavailable: "插件运行时不可用",
  payload_invalid: "请求参数无效",
  action_not_declared: "动作未声明",
  ui_contract_invalid: "UI 契约无效",
};

const codeLabel = computed(() => CODE_LABELS[props.code] ?? (props.code || "未知错误"));
const copyState = ref("");

async function copyRaw() {
  try {
    if (window.amitiaDesktop?.writeClipboardText) {
      await window.amitiaDesktop.writeClipboardText(props.raw);
    } else if (navigator.clipboard) {
      await navigator.clipboard.writeText(props.raw);
    } else {
      throw new Error("CLIPBOARD_HOST_UNAVAILABLE");
    }
    copyState.value = "已复制";
  } catch (copyError) {
    copyState.value = `复制失败: ${copyError instanceof Error ? copyError.message : String(copyError)}`;
  }
}
</script>

<template>
  <div class="schema-ui-error-panel" role="alert" :data-schema-ui-error="code || 'unknown'">
    <div class="schema-ui-error-panel__head">
      <span class="schema-ui-error-panel__title">插件调用失败 · {{ codeLabel }}</span>
      <span class="schema-ui-error-panel__code">{{ code || "unknown" }}</span>
    </div>
    <p class="schema-ui-error-panel__summary">{{ summary }}</p>
    <pre class="schema-ui-error-panel__raw" data-schema-ui-error-raw>{{ raw }}</pre>
    <div class="schema-ui-error-panel__actions">
      <button type="button" class="schema-ui-error-panel__button" @click="copyRaw">复制原始错误</button>
      <button
        v-if="retryable"
        type="button"
        class="schema-ui-error-panel__button schema-ui-error-panel__button--primary"
        @click="emit('retry')"
      >
        {{ retryLabel || "重试" }}
      </button>
      <span v-if="copyState" class="schema-ui-error-panel__copy-state">{{ copyState }}</span>
    </div>
  </div>
</template>

<style scoped>
.schema-ui-error-panel {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
  border-radius: 8px;
  border: 1px solid color-mix(in srgb, var(--ac-color-danger) 32%, var(--plugin-surface-border));
  background: var(--ac-color-danger-bg);
  color: var(--ac-color-danger);
  font-size: 12px;
  min-width: 0;
}
.schema-ui-error-panel__head {
  display: flex;
  align-items: baseline;
  gap: 8px;
  flex-wrap: wrap;
}
.schema-ui-error-panel__title {
  font-weight: 600;
  font-size: 13px;
}
.schema-ui-error-panel__code {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
  opacity: 0.85;
}
.schema-ui-error-panel__summary {
  margin: 0;
  color: var(--amitia-text-primary, inherit);
  line-height: 1.5;
}
.schema-ui-error-panel__raw {
  margin: 0;
  max-height: 260px;
  overflow: auto;
  padding: 8px 10px;
  border-radius: 6px;
  background: color-mix(in srgb, var(--ac-color-danger) 8%, var(--amitia-bg-surface, transparent));
  color: var(--amitia-text-primary, inherit);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
  user-select: text;
}
.schema-ui-error-panel__actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.schema-ui-error-panel__button {
  padding: 3px 10px;
  border-radius: 4px;
  border: 1px solid color-mix(in srgb, var(--ac-color-danger) 45%, transparent);
  background: transparent;
  color: var(--ac-color-danger);
  font-size: 12px;
  cursor: pointer;
}
.schema-ui-error-panel__button--primary {
  background: color-mix(in srgb, var(--ac-color-danger) 12%, transparent);
  font-weight: 600;
}
.schema-ui-error-panel__copy-state {
  font-size: 11px;
  opacity: 0.85;
}
</style>

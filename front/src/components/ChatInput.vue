<template>
  <div class="chat-input-bar">
    <div class="input-wrapper">
      <textarea
        ref="inputRef"
        v-model="text"
        class="input-field"
        :placeholder="sending ? 'AI 回复中...' : '输入消息...'"
        :disabled="disabled || sending"
        rows="1"
        @keydown.enter.exact="handleSend"
        @input="autoResize"
      />
      <div class="input-actions">
        <el-button
          v-if="sending"
          type="danger"
          :icon="CloseBold"
          circle
          size="small"
          @click="$emit('stop')"
          title="停止生成"
        />
        <el-button
          v-else
          type="primary"
          :icon="Promotion"
          circle
          size="small"
          :disabled="disabled || !text.trim()"
          @click="handleSend"
          title="发送 (Enter)"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, nextTick, watch } from "vue"
import { Promotion, CloseBold } from "@element-plus/icons-vue"

const props = defineProps<{
  disabled?: boolean
  sending?: boolean
}>()

const emit = defineEmits<{
  send: [text: string]
  stop: []
}>()

const DRAFT_KEY = "webchat_draft"
const text = ref(localStorage.getItem(DRAFT_KEY) || "")
const inputRef = ref<HTMLTextAreaElement>()


function saveDraft() {
  if (text.value.trim()) {
    localStorage.setItem(DRAFT_KEY, text.value)
  } else {
    localStorage.removeItem(DRAFT_KEY)
  }
}

function handleSend(e?: KeyboardEvent) {
  if (e) e.preventDefault()
  const trimmed = text.value.trim()
  if (!trimmed || props.disabled || props.sending) return
  emit("send", trimmed)
  text.value = ""; localStorage.removeItem(DRAFT_KEY)
  nextTick(() => autoResize())
}

function autoResize() {
  const el = inputRef.value
  if (!el) return
  el.style.height = "auto"
  el.style.height = Math.min(el.scrollHeight, 120) + "px"
}

watch(text, () => { saveDraft() }, { flush: 'post' })

function focus() {
  inputRef.value?.focus()
}

defineExpose({ focus, clear: () => { text.value = ""; localStorage.removeItem(DRAFT_KEY) } })
</script>

<style scoped>
.chat-input-bar {
  padding: 10px 14px;
  /* background: var(--ac-color-surface); */
  /* border-top: 1px solid var(--ac-color-border-light); */
  flex-shrink: 0;
}

.input-wrapper {
  display: flex;
  align-items: flex-end;
  gap: 8px;
  background: var(--ac-color-bg-secondary);
  border-radius: var(--ac-radius-lg);
  padding: 6px 6px 6px 14px;
  border: 1px solid var(--ac-color-border);
  transition: border-color var(--ac-transition-fast);
  box-shadow: var(--ac-shadow-sm);
}

.input-wrapper:focus-within {
  border-color: var(--ac-color-primary);
}

.input-field {
  flex: 1;
  border: none;
  background: transparent;
  outline: none;
  font-family: var(--ac-font-family);
  font-size: var(--ac-font-size-sm);
  color: var(--ac-color-text);
  resize: none;
  min-height: 24px;
  max-height: 120px;
  line-height: 1.5;
  padding: 3px 0;
}

.input-field::placeholder {
  color: var(--ac-color-text-placeholder);
}

.input-field:disabled {
  opacity: 0.7;
}

.input-actions {
  flex-shrink: 0;
  display: flex;
  align-items: center;
}

@media (max-width: 768px) {
  .chat-input-bar {
    padding: 8px 10px;
    padding-bottom: calc(8px + env(safe-area-inset-bottom, 0px));
  }

  .input-field {
    font-size: 16px; /* Prevent iOS zoom on focus */
  }

  .input-wrapper {
    padding: 8px 8px 8px 12px;
    border-radius: 20px;
  }
}
</style>

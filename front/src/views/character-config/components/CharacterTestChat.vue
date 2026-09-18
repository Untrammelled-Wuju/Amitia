<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="test-area">
    <div class="test-chat" ref="chatRef">
      <div v-if="messages.length === 0 && !loading" class="test-empty">
        <p>在下方输入测试消息，预览角色回复</p>
        <p class="test-hint">测试不会写入正式会话</p>
      </div>
      <div v-for="(m, i) in messages" :key="i" class="test-msg" :class="m.role">
        <span class="tm-role">{{
          m.role === "user" ? "你" : charName || "角色"
        }}</span>
        <div class="tm-content">{{ m.content }}</div>
      </div>
      <div v-if="loading" class="test-msg assistant">
        <span class="tm-role">{{ charName || "角色" }}</span>
        <div class="tm-content typing">回复中...</div>
      </div>
    </div>
    <div class="test-input">
      <el-input
        v-model="msgModel"
        placeholder="输入测试消息..."
        @keyup.enter="emit('send', msgModel)"
        :disabled="loading"
      >
        <template #append>
          <el-button
            :icon="Promotion"
            @click="emit('send', msgModel)"
            :disabled="loading || !msg.trim()"
          />
        </template>
      </el-input>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue";
import { Promotion } from "@element-plus/icons-vue";

const props = defineProps<{
  messages: { role: string; content: string }[];
  loading: boolean;
  msg: string;
  charName: string;
}>();
const emit = defineEmits<{
  (e: "update:msg", v: string): void;
  (e: "send", text: string): void;
}>();
const chatRef = ref<HTMLElement | null>(null);
const msgModel = computed({
  get: () => props.msg,
  set: (v) => emit("update:msg", v),
});

watch(
  () => [props.messages.length, props.loading] as const,
  async () => {
    await nextTick();
    const chat = chatRef.value;
    if (chat) chat.scrollTop = chat.scrollHeight;
  },
);
</script>

<style scoped>
.test-area {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-width: 0;
  min-height: 0;
}

.test-chat {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 18px;
  overflow-x: hidden;
  overflow-y: auto;
  border: 1px solid var(--ac-color-border-light);
  border-radius: var(--ac-radius-md);
  background: var(--ac-color-bg-secondary);
  scroll-behavior: smooth;
}

.test-empty {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-height: 220px;
  padding: 28px;
  color: var(--ac-color-text-muted);
  text-align: center;
}

.test-empty p {
  margin: 0;
}

.test-hint {
  font-size: var(--ac-font-size-xs);
  opacity: 0.72;
}

.test-msg {
  display: flex;
  flex-direction: column;
  align-self: flex-start;
  gap: 5px;
  width: fit-content;
  max-width: min(78%, 640px);
  min-width: 0;
}

.test-msg.user {
  align-self: flex-end;
  align-items: flex-end;
}

.test-msg.assistant {
  align-self: flex-start;
  align-items: flex-start;
}

.tm-role {
  padding: 0 4px;
  color: var(--ac-color-text-muted);
  font-size: var(--ac-font-size-xs);
  line-height: 1.4;
}

.tm-content {
  min-width: 0;
  padding: 10px 13px;
  border: 1px solid var(--ac-color-border-light);
  border-radius: 4px 12px 12px 12px;
  background: var(--ac-color-surface);
  color: var(--ac-color-text);
  font-size: var(--ac-font-size-sm);
  line-height: 1.65;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
}

.test-msg.user .tm-content {
  border-color: var(--el-color-primary-light-7);
  border-radius: 12px 4px 12px 12px;
  background: var(--el-color-primary-light-9);
}

.tm-content.typing {
  color: var(--ac-color-text-muted);
}

.test-input {
  flex-shrink: 0;
  min-width: 0;
  padding-top: 12px;
}

.test-input :deep(.el-input) {
  width: 100%;
  min-width: 0;
}

.test-input :deep(.el-input-group__append) {
  padding: 0;
}

.test-input :deep(.el-input-group__append .el-button) {
  width: 44px;
  height: 100%;
  margin: 0;
  border: 0;
  border-radius: 0;
}

@media (max-width: 640px) {
  .test-area {
    min-height: 380px;
  }

  .test-chat {
    padding: 14px;
  }

  .test-msg {
    max-width: 90%;
  }
}
</style>

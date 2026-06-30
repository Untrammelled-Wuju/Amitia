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
        <span class="tm-role">{{ m.role === "user" ? "你" : charName }}</span>
        <div class="tm-content">{{ m.content }}</div>
      </div>
      <div v-if="loading" class="test-msg assistant">
        <span class="tm-role">{{ charName }}</span>
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
          <el-button :icon="Promotion" @click="emit('send', msgModel)" :disabled="loading || !msg.trim()" />
        </template>
      </el-input>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from "vue"
import { Promotion } from "@element-plus/icons-vue"

const props = defineProps<{ messages: { role: string; content: string }[]; loading: boolean; msg: string; charName: string }>()
const emit = defineEmits<{
  (e: "update:msg", v: string): void
  (e: "send", text: string): void
}>()
const msgModel = computed({ get: () => props.msg, set: (v) => emit("update:msg", v) })
</script>

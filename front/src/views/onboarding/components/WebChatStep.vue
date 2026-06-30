<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="step-panel">
    <h2>启用 Web 聊天</h2>
    <p class="step-desc">通过浏览器直接与 AI 角色对话。</p>
    <div class="toggle-card">
      <div class="tc-title">Web 聊天功能</div>
      <div class="tc-desc">开启后可通过浏览器访问聊天界面，与 AI 陪伴角色对话。</div>
      <el-switch v-model="webModel" />
    </div>
  </div>
</template>
<script setup lang="ts">
import { computed } from "vue"
const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{ (e: "update:modelValue", v: boolean): void }>()
const webModel = computed({ get: () => props.modelValue, set: (v) => emit("update:modelValue", v) })
</script>

<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-dialog v-model="visible" title="测试结果" width="500px">
    <div v-if="testResult" class="msg-preview">{{ testResult.message || JSON.stringify(testResult, null, 2) }}</div>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed } from "vue"

const props = defineProps<{ modelValue: boolean; testResult: any }>()
const emit = defineEmits<{ (e: "update:modelValue", v: boolean): void }>()
const visible = computed({ get: () => props.modelValue, set: (v) => emit("update:modelValue", v) })
</script>

<style scoped>
.msg-preview {
  white-space: pre-wrap;
  padding: 8px;
  background: var(--ac-color-surface-hover);
  border-radius: var(--ac-radius-sm);
  font-size: var(--ac-font-size-sm);
}
</style>

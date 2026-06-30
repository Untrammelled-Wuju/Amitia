<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-dialog v-model="visible" title="测试结果" width="440px">
    <div v-if="testResult" class="test-result">
      <div class="tr-status" :class="testResult.success ? 'success' : 'fail'">
        <el-icon :size="20">
          <CircleCheckFilled v-if="testResult.success" />
          <CircleCloseFilled v-else />
        </el-icon>
        <span>{{ testResult.success ? '连接成功' : '连接失败' }}</span>
      </div>
      <div class="tr-meta">
        <div class="tr-row">
          <span class="trl">延迟</span>
          <span class="trv">{{ testResult.latencyMs }}ms</span>
        </div>
        <div class="tr-row" v-if="testResult.message">
          <span class="trl">信息</span>
          <span class="trv">{{ testResult.message }}</span>
        </div>
        <div class="tr-row" v-if="testResult.reply">
          <span class="trl">模型回复</span>
          <span class="trv reply">{{ testResult.reply }}</span>
        </div>
      </div>
    </div>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed } from "vue"
import { CircleCheckFilled, CircleCloseFilled } from "@element-plus/icons-vue"

const props = defineProps<{
  modelValue: boolean
  testResult: any
}>()

const emit = defineEmits<{
  (e: "update:modelValue", v: boolean): void
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit("update:modelValue", v),
})
</script>

<style scoped>
.test-result {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.tr-status {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: var(--ac-font-size-lg);
  font-weight: 600;
}

.tr-status.success { color: var(--ac-color-success); }
.tr-status.fail { color: var(--ac-color-danger); }

.tr-meta {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.tr-row {
  display: flex;
  gap: 8px;
  font-size: var(--ac-font-size-sm);
}

.trl {
  color: var(--ac-color-text-muted);
  min-width: 60px;
}

.trv {
  color: var(--ac-color-text);
}

.trv.reply {
  font-style: italic;
  color: var(--ac-color-text-secondary);
  background: var(--ac-color-bg-secondary);
  padding: 8px 10px;
  border-radius: var(--ac-radius-sm);
  flex: 1;
}
</style>

<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <teleport to="body">
    <transition name="ep-fade">
      <div v-if="visible" class="error-panel-overlay" @click.self="dismiss">
        <div class="error-panel">
          <div class="ep-header">
            <el-icon :size="24" color="var(--el-color-danger)"><WarningFilled /></el-icon>
            <span class="ep-title">{{ error?.message || "Error" }}</span>
            <el-button text circle size="small" @click="dismiss">
              <el-icon><Close /></el-icon>
            </el-button>
          </div>
          <div class="ep-body">
            <p v-if="error?.detail" class="ep-detail">{{ error.detail }}</p>
            <p v-if="error?.code" class="ep-code">Error code: {{ error.code }}</p>
          </div>
          <div class="ep-actions">
            <el-button @click="dismiss">Close</el-button>
            <el-button
              v-if="error?.action"
              type="primary"
              @click="handleAction"
            >
              {{ error!.action!.label }}
            </el-button>
            <el-button
              v-if="error?.code && error.code >= 40000 && error.code < 50000"
              type="primary"
              @click="goToModelConfig"
            >
              Configure Model
            </el-button>
            <el-button
              v-if="error?.code === 10001"
              type="primary"
              @click="handleStartCore"
            >
              Start Core Service
            </el-button>
          </div>
        </div>
      </div>
    </transition>
  </teleport>
</template>

<script setup lang="ts">
import { ref } from "vue"
import { WarningFilled, Close } from "@element-plus/icons-vue"
import type { RequestError } from "../composables/request"
import { ERR } from "@/types"

const visible = ref(false)
const error = ref<RequestError | null>(null)

function show(err: RequestError) {
  error.value = err
  visible.value = true
}

function dismiss() {
  visible.value = false
  error.value = null
}

function handleAction() {
  error.value?.action?.handler()
  dismiss()
}

function goToModelConfig() {
  window.location.hash = "#/model"
  dismiss()
}

let onStartCore: (() => void) | null = null

function setStartCoreHandler(fn: () => void) {
  onStartCore = fn
}

function handleStartCore() {
  onStartCore?.()
  dismiss()
}

defineExpose({ show, dismiss, setStartCoreHandler })
</script>

<style scoped>
.error-panel-overlay {
  position: fixed;
  inset: 0;
  z-index: 3000;
  background: rgba(0, 0, 0, 0.4);
  display: flex;
  align-items: center;
  justify-content: center;
}

.error-panel {
  background: var(--el-bg-color);
  border-radius: var(--el-border-radius-base);
  box-shadow: var(--el-box-shadow-dark);
  width: 420px;
  max-width: 90vw;
  padding: 20px 24px;
}

.ep-header {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}

.ep-title {
  flex: 1;
  font-size: 16px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.ep-body {
  margin-bottom: 16px;
}

.ep-detail {
  font-size: 14px;
  color: var(--el-text-color-regular);
  line-height: 1.6;
  margin-bottom: 8px;
}

.ep-code {
  font-size: 12px;
  color: var(--el-text-color-placeholder);
  font-family: monospace;
}

.ep-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.ep-fade-enter-active,
.ep-fade-leave-active {
  transition: opacity 0.2s ease;
}

.ep-fade-enter-from,
.ep-fade-leave-to {
  opacity: 0;
}
</style>

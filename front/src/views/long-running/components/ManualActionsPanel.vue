<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-card shadow="never" class="section-card">
    <template #header><span class="section-title">手动操作</span></template>
    <div class="lr-actions">
      <div class="lr-action-row">
        <div class="lr-action-info">
          <div class="lr-action-name">清理临时文件</div>
          <div class="lr-action-desc">删除过期的临时文件、导入导出暂存文件</div>
        </div>
        <el-button size="small" type="warning" :loading="cleanupLoading" @click="runCleanup">
          {{ cleanupLoading ? '清理中...' : '立即清理' }}
        </el-button>
      </div>
      <el-divider />
      <div class="lr-action-row">
        <div class="lr-action-info">
          <div class="lr-action-name">日志轮转</div>
          <div class="lr-action-desc">对超过大小限制的日志文件进行轮转归档</div>
        </div>
        <el-button size="small" type="primary" :loading="rotateLoading" @click="runLogRotate">
          {{ rotateLoading ? '轮转中...' : '立即轮转' }}
        </el-button>
      </div>
      <el-divider />
      <div class="lr-action-row">
        <div class="lr-action-info">
          <div class="lr-action-name">数据库完整性检查</div>
          <div class="lr-action-desc">检查数据库文件完整性及外键约束</div>
        </div>
        <el-button size="small" type="success" :loading="dbCheckLoading" @click="runDbCheck">
          {{ dbCheckLoading ? '检查中...' : '立即检查' }}
        </el-button>
      </div>
    </div>
    <div v-if="actionResult" class="lr-action-result">
      <el-alert
        :title="actionResult.message"
        :type="actionResult.type"
        :closable="true"
        @close="actionResult = null"
        show-icon
      />
      <div v-if="actionResult.detail" class="lr-action-detail">{{ actionResult.detail }}</div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { ref } from "vue"
import { cleanupTempApi, rotateLogsApi, checkDbIntegrityApi } from "../api"
import { fmtBytes } from "../utils"

const emit = defineEmits<{
  actionCompleted: []
}>()

const cleanupLoading = ref(false)
const rotateLoading = ref(false)
const dbCheckLoading = ref(false)
const actionResult = ref<{ message: string; type: "success" | "warning" | "error" | "info"; detail?: string } | null>(null)

async function runCleanup() {
  cleanupLoading.value = true
  actionResult.value = null
  try {
    const result = await cleanupTempApi()
    actionResult.value = {
      message: `清理完成: 删除 ${result.deleted} 个文件`,
      type: "success",
      detail: `释放空间: ${fmtBytes(result.freedBytes)}`,
    }
    emit("actionCompleted")
  } catch {
    actionResult.value = { message: "清理失败", type: "error" }
  } finally {
    cleanupLoading.value = false
  }
}

async function runLogRotate() {
  rotateLoading.value = true
  actionResult.value = null
  try {
    const result = await rotateLogsApi()
    actionResult.value = {
      message: result.rotated.length > 0
        ? `已轮转 ${result.rotated.length} 个日志文件`
        : "所有日志文件未超过大小限制",
      type: "success",
      detail: result.rotated.length > 0 ? `轮转文件: ${result.rotated.join(", ")}` : undefined,
    }
    emit("actionCompleted")
  } catch {
    actionResult.value = { message: "日志轮转失败", type: "error" }
  } finally {
    rotateLoading.value = false
  }
}

async function runDbCheck() {
  dbCheckLoading.value = true
  actionResult.value = null
  try {
    const result = await checkDbIntegrityApi()
    actionResult.value = {
      message: result.ok && result.errors.length === 0
        ? "数据库完整性检查通过"
        : `发现 ${result.errors.length} 个问题`,
      type: result.ok ? "success" : "warning",
      detail: result.errors.length > 0 ? result.errors.join("; ") : undefined,
    }
    emit("actionCompleted")
  } catch {
    actionResult.value = { message: "数据库检查失败", type: "error" }
  } finally {
    dbCheckLoading.value = false
  }
}
</script>

<style scoped>
.section-card {
  margin-bottom: 14px;
  border: 1px solid var(--ac-color-border-light);
}
.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--ac-color-text);
}
.lr-action-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 0;
}
.lr-action-info {
  flex: 1;
}
.lr-action-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--ac-color-text);
}
.lr-action-desc {
  font-size: 11px;
  color: var(--ac-color-text-muted);
  margin-top: 2px;
}
.lr-action-result {
  margin-top: 12px;
}
.lr-action-detail {
  margin-top: 6px;
  font-size: 12px;
  color: var(--ac-color-text-secondary);
  padding: 6px 10px;
  background: var(--ac-color-bg-secondary);
  border-radius: var(--ac-radius-sm);
  word-break: break-all;
}
@media (max-width: 600px) {
  .lr-action-row {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }
}
</style>

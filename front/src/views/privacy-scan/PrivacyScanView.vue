<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="privacy-scan-view">
    <div class="page-header">
      <h2>敏感数据扫描</h2>
      <p class="page-desc">扫描历史记录、记忆和导入数据中的敏感信息，进行脱敏处理</p>
    </div>

    <el-alert
      title="不会自动删除数据"
      type="info"
      :closable="false"
      show-icon
      class="notice-alert"
    >
      <template #default>
        <p>扫描仅检测敏感信息，不会自动修改或删除你的数据。脱敏操作需要你手动确认。</p>
      </template>
    </el-alert>

    <ScanScopePanel :scanning="scanning" @scan="runScan" />

    <ScanResultsPanel v-if="scanSummary" :scan-summary="scanSummary" />
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue"
import { ElMessage } from "element-plus"
import ScanScopePanel from "./components/ScanScopePanel.vue"
import ScanResultsPanel from "./components/ScanResultsPanel.vue"
import { postScan } from "./api"

const scanning = ref(false)
const scanSummary = ref<any>(null)

async function runScan(scope: string[]) {
  scanning.value = true
  scanSummary.value = null
  try {
    const d = await postScan(scope)
    scanSummary.value = d
    ElMessage.success(d.message || "扫描完成")
  } catch (err: any) {
    ElMessage.error("扫描失败: " + (err.response?.data?.message || err.message))
  } finally {
    scanning.value = false
  }
}
</script>

<style scoped>
.privacy-scan-view { padding: 20px; max-width: 900px; }
.page-header { margin-bottom: 16px; }
.page-header h2 { font-size: 20px; font-weight: 600; margin: 0 0 4px 0; color: var(--el-text-color-primary); }
.page-desc { font-size: 13px; color: var(--el-text-color-secondary); margin: 0; }
.notice-alert { margin-bottom: 16px; }
.notice-alert p { margin: 0; font-size: 13px; }

@media (max-width: 600px) {
  .privacy-scan-view { padding: 12px; max-width: 100%; }
}
</style>

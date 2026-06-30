<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div>
    <div v-if="accessRisk && (accessRisk.overallLevel === 'error' || accessRisk.overallLevel === 'warn')" class="access-risk-alert" :class="'risk-' + accessRisk.overallLevel">
      <div class="ara-header">
        <el-icon :size="20"><Warning /></el-icon>
        <span class="ara-title">访问安全风险</span>
      </div>
      <div class="ara-list">
        <div v-for="c in accessRisk.checks.filter((x: any) => x.level !== 'ok')" :key="c.name" class="ara-item" :class="'ara-' + c.level">
          <span class="arai-dot"></span>
          <span class="arai-name">{{ c.name }}</span>
          <span class="arai-msg">{{ c.message }}</span>
        </div>
      </div>
      <div class="ara-footer">
        <router-link to="/settings#access-protection" class="ara-link">前往访问保护设置 →</router-link>
      </div>
    </div>

    <div v-if="cloudRisk && cloudRisk.hasRisk" class="access-risk-alert" :class="'risk-' + (cloudRisk.riskLevel === 'high' ? 'error' : 'warn')">
      <div class="ara-header">
        <el-icon :size="20"><Connection /></el-icon>
        <span class="ara-title">WeChat Bridge Cloud Risk - {{ cloudRisk.riskCount }} issue(s)</span>
      </div>
      <div class="ara-list">
        <div v-for="c in cloudRisk.items" :key="c.name" class="ara-item" :class="'ara-' + (c.status === 'error' ? 'error' : 'warn')">
          <span class="arai-dot"></span>
          <span class="arai-name">{{ c.name }}</span>
          <span class="arai-msg">{{ c.status === 'error' ? 'Error' : 'Warning' }}</span>
        </div>
      </div>
      <div class="ara-footer">
        <router-link to="/wechat" class="ara-link">Go to WeChat Cloud Check</router-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Warning, Connection } from "@element-plus/icons-vue"

defineProps<{
  accessRisk: any
  cloudRisk: any
}>()
</script>

<style scoped>
.access-risk-alert {
  padding: 14px 16px;
  border-radius: 8px;
  margin-bottom: 16px;
}
.access-risk-alert.risk-error { background: #fef0f0; border: 1px solid #fbc4c4; }
.access-risk-alert.risk-warn { background: #fef7e0; border: 1px solid #fae29c; }
.ara-header { display: flex; align-items: center; gap: 8px; margin-bottom: 10px; }
.risk-error .ara-header { color: #f56c6c; }
.risk-warn .ara-header { color: #e6a23c; }
.ara-title { font-size: 15px; font-weight: 700; }
.ara-list { display: flex; flex-direction: column; gap: 6px; margin-bottom: 8px; }
.ara-item { display: flex; align-items: baseline; gap: 8px; font-size: 13px; padding: 4px 0; }
.arai-dot { width: 6px; height: 6px; border-radius: 50%; flex-shrink: 0; }
.ara-error .arai-dot { background: #f56c6c; }
.ara-warn .arai-dot { background: #e6a23c; }
.arai-name { font-weight: 600; white-space: nowrap; flex-shrink: 0; min-width: 80px; color: var(--ac-color-text); }
.arai-msg { color: var(--ac-color-text-secondary); line-height: 1.4; }
.ara-footer { text-align: right; }
.ara-link { font-size: 13px; font-weight: 600; text-decoration: none; }
.risk-error .ara-link { color: #f56c6c; }
.risk-warn .ara-link { color: #e6a23c; }
</style>

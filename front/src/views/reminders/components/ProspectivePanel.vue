<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-card shadow="never" class="section-card">
    <template #header>
      <div class="card-header-row">
        <span class="section-title">前瞻记忆</span>
        <el-tag v-if="queueSummary" :type="queueSummary.backpressure ? 'danger' : 'info'" size="small">
          {{ queueSummary.backpressure ? '背压' : '队列正常' }}
        </el-tag>
      </div>
    </template>

    <el-alert type="info" :closable="false" show-icon style="margin-bottom:10px">
      前瞻记忆到期后生成触发候选，经调度器评估后发送。队列背压时低优先任务延期或取消。
    </el-alert>

    <div v-if="queueSummary" class="queue-stats">
      <div class="qs-item">
        <span class="qs-label">队列深度</span>
        <span class="qs-value">{{ queueSummary.depth }}</span>
      </div>
      <div class="qs-item">
        <span class="qs-label">待触发</span>
        <span class="qs-value" style="color:var(--el-color-warning)">{{ queueSummary.pendingCount }}</span>
      </div>
      <div class="qs-item">
        <span class="qs-label">最近失败</span>
        <span class="qs-value" :style="{ color: queueSummary.recentFailures > 0 ? 'var(--el-color-danger)' : 'inherit' }">{{ queueSummary.recentFailures }}</span>
      </div>
    </div>

    <div class="prospective-list">
      <div v-for="item in items" :key="item.id" class="prospective-item">
        <div class="pi-header">
          <span class="pi-title">{{ item.title }}</span>
          <el-tag :type="statusType(item.status)" size="small">{{ statusLabel(item.status) }}</el-tag>
        </div>
        <div v-if="item.description" class="pi-desc">{{ item.description }}</div>
        <div class="pi-meta">
          <span v-if="item.expiresAt" class="pi-meta-item">
            <el-icon style="margin-right:2px"><Clock /></el-icon>到期: {{ fmtDate(item.expiresAt) }}
          </span>
          <span v-if="item.notBefore" class="pi-meta-item">
            不早于: {{ fmtDate(item.notBefore) }}
          </span>
          <span class="pi-meta-item">优先级: {{ item.priority }}</span>
          <span class="pi-meta-item">触发: {{ item.triggerReason }}</span>
        </div>
      </div>
      <div v-if="items.length === 0 && !loading" class="empty-state">暂无前瞻记忆</div>
      <div v-if="loading" class="empty-state">加载中...</div>
    </div>

    <div style="margin-top:10px;display:flex;gap:8px">
      <el-button size="small" @click="$emit('refresh')">刷新</el-button>
      <el-button size="small" v-if="queueSummary?.backpressure" type="danger" plain @click="$emit('clearBackpressure')">
        清除背压标记
      </el-button>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue"
import { Clock } from "@element-plus/icons-vue"
import { request } from "../../../composables/request"
import type { ProspectiveMemory, TriggerQueueSummary } from "../../../types"

defineEmits<{
  refresh: []
  clearBackpressure: []
}>()

const items = ref<ProspectiveMemory[]>([])
const queueSummary = ref<TriggerQueueSummary | null>(null)
const loading = ref(false)

async function fetchItems() {
  loading.value = true
  try {
    const res: any = await request.get("/api/reminders/prospective")
    items.value = Array.isArray(res) ? res : (res?.items || [])
  } catch {
    items.value = []
  }
  finally { loading.value = false }
}

async function fetchQueueSummary() {
  try {
    const res: any = await request.get("/api/reminders/queue-summary")
    queueSummary.value = res || null
  } catch {
    queueSummary.value = null
  }
}

function statusType(s: string) {
  const map: Record<string, string> = { pending: 'warning', triggered: 'primary', completed: 'success', cancelled: 'info' }
  return map[s] || 'info'
}

function statusLabel(s: string) {
  const map: Record<string, string> = { pending: '待触发', triggered: '已触发', completed: '已完成', cancelled: '已取消' }
  return map[s] || s
}

function fmtDate(d: string) {
  if (!d) return ''
  try { return new Date(d).toLocaleString('zh-CN') } catch { return d }
}

onMounted(() => { fetchItems(); fetchQueueSummary() })
</script>

<style scoped>
.section-card { margin-bottom: 16px; }
.section-title { font-weight: 600; font-size: var(--ac-font-size-base); }
.card-header-row { display: flex; justify-content: space-between; align-items: center; }

.queue-stats { display: flex; gap: 24px; margin-bottom: 12px; }
.qs-item { display: flex; align-items: center; gap: 6px; }
.qs-label { font-size: var(--ac-font-size-sm); color: var(--ac-color-text-muted); }
.qs-value { font-weight: 600; font-size: var(--ac-font-size-base); }

.prospective-list { display: flex; flex-direction: column; gap: 10px; }
.prospective-item { padding: 10px; border: 1px solid var(--ac-color-border-light); border-radius: var(--ac-radius-sm); background: var(--ac-color-bg-secondary); }
.pi-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 4px; }
.pi-title { font-weight: 500; font-size: var(--ac-font-size-sm); }
.pi-desc { font-size: var(--ac-font-size-xs); color: var(--ac-color-text-secondary); margin-bottom: 4px; }
.pi-meta { display: flex; flex-wrap: wrap; gap: 12px; font-size: var(--ac-font-size-xs); color: var(--ac-color-text-muted); }
.pi-meta-item { display: inline-flex; align-items: center; }
.empty-state { text-align: center; padding: 24px; color: var(--ac-color-text-muted); font-size: var(--ac-font-size-sm); }
</style>

<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-card shadow="never" class="section-card">
    <template #header>
      <div class="card-header-row">
        <span class="section-title">触发历史</span>
        <div class="header-actions">
          <el-select v-model="filterState" size="small" style="width:120px" placeholder="状态筛选" clearable @change="fetchItems">
            <el-option label="全部" value="" />
            <el-option label="待处理" value="pending" />
            <el-option label="发送中" value="sending" />
            <el-option label="已发送" value="sent" />
            <el-option label="失败" value="failed" />
            <el-option label="已取消" value="cancelled" />
          </el-select>
          <el-button size="small" @click="$emit('refresh')">刷新</el-button>
        </div>
      </div>
    </template>

    <el-table :data="items" stripe size="small" v-loading="loading" empty-text="暂无触发历史">
      <el-table-column prop="title" label="标题" min-width="120" show-overflow-tooltip />
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="stateType(row.state)" size="small">{{ stateLabel(row.state) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="渠道" width="70">
        <template #default="{ row }"><el-tag :type="row.channel === 'wechat' ? 'success' : row.channel === 'qq' ? 'primary' : 'info'" size="small">{{ row.channel === 'wechat' ? '微信' : row.channel === 'qq' ? 'QQ' : 'Web' }}</el-tag></template>
      </el-table-column>
      <el-table-column label="优先级" width="80">
        <template #default="{ row }">{{ row.priority }}</template>
      </el-table-column>
      <el-table-column label="类型" width="100">
        <template #default="{ row }">{{ row.triggerType }}</template>
      </el-table-column>
      <el-table-column label="原因" min-width="120" show-overflow-tooltip>
        <template #default="{ row }">{{ row.reason }}</template>
      </el-table-column>
      <el-table-column label="重试" width="60" align="center">
        <template #default="{ row }">{{ row.attemptCount }}</template>
      </el-table-column>
      <el-table-column label="错误" min-width="140" show-overflow-tooltip>
        <template #default="{ row }">
          <span v-if="row.lastError" style="color:var(--el-color-danger);font-size:12px">{{ row.lastError }}</span>
          <span v-else style="color:var(--ac-color-text-muted);font-size:12px">—</span>
        </template>
      </el-table-column>
      <el-table-column label="创建时间" width="155">
        <template #default="{ row }">{{ fmtDate(row.createdAt) }}</template>
      </el-table-column>
    </el-table>

    <div v-if="total > pageSize" class="pagination">
      <el-button size="small" :disabled="page <= 1" @click="page--; fetchItems()">上一页</el-button>
      <span style="font-size:13px">{{ page }} / {{ Math.ceil(total / pageSize) }}</span>
      <el-button size="small" :disabled="page * pageSize >= total" @click="page++; fetchItems()">下一页</el-button>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue"
import { request } from "../../../composables/request"
import type { TriggerHistory } from "../../../types"

defineEmits<{
  refresh: []
}>()

const items = ref<TriggerHistory[]>([])
const loading = ref(false)
const filterState = ref("")
const page = ref(1)
const total = ref(0)
const pageSize = 20

async function fetchItems() {
  loading.value = true
  try {
    const params: any = { page: page.value, pageSize }
    if (filterState.value) params.state = filterState.value
    const res: any = await request.get("/api/reminders/trigger-history", params)
    const raw = Array.isArray(res) ? { items: res, total: res.length } : (res || { items: [], total: 0 })
    items.value = raw.items || []
    total.value = raw.total || 0
  } catch {
    items.value = []
    total.value = 0
  }
  finally { loading.value = false }
}

function stateType(s: string) {
  const map: Record<string, string> = { pending: 'warning', sending: 'primary', sent: 'success', failed: 'danger', cancelled: 'info' }
  return map[s] || 'info'
}

function stateLabel(s: string) {
  const map: Record<string, string> = { pending: '待处理', sending: '发送中', sent: '已发送', failed: '失败', cancelled: '已取消' }
  return map[s] || s
}

function fmtDate(d: string) {
  if (!d) return ''
  try { return new Date(d).toLocaleString('zh-CN') } catch { return d }
}

onMounted(() => { fetchItems() })
</script>

<style scoped>
.section-card { margin-bottom: 16px; }
.section-title { font-weight: 600; font-size: var(--ac-font-size-base); }
.card-header-row { display: flex; justify-content: space-between; align-items: center; }
.header-actions { display: flex; gap: 8px; align-items: center; }
.pagination { display: flex; justify-content: center; align-items: center; gap: 12px; margin-top: 16px; }
</style>

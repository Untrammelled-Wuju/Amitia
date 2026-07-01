<template>
  <el-card class="section-card">
    <template #header>
      <div class="card-header">
        <span class="card-title">扫描历史</span>
        <el-button size="small" text @click="loadHistory">
          <el-icon><Refresh /></el-icon>
        </el-button>
      </div>
    </template>

    <el-table :data="items" style="width: 100%" size="small" v-loading="loading">
      <el-table-column prop="scan_time" label="扫描时间" width="160">
        <template #default="{ row }">
          {{ formatTime(row.scan_time) }}
        </template>
      </el-table-column>
      <el-table-column prop="scope" label="扫描范围" min-width="140">
        <template #default="{ row }">
          <el-tag
            v-for="s in (row.scope || [])"
            :key="s"
            size="small"
            style="margin-right: 4px; margin-bottom: 2px"
          >
            {{ scopeLabel(s) }}
          </el-tag>
          <span v-if="!row.scope?.length" class="no-data">—</span>
        </template>
      </el-table-column>
      <el-table-column prop="total_found" label="发现" width="60" align="right" />
      <el-table-column label="高风险" width="60" align="right">
        <template #default="{ row }">
          <span v-if="row.high_risk" class="risk-number high">{{ row.high_risk }}</span>
          <span v-else class="no-data">0</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="80" fixed="right">
        <template #default="{ row }">
          <el-button size="small" text @click="$emit('viewResult', row.id)">
            查看
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <div class="pagination-wrap" v-if="total > pageSize">
      <el-pagination
        v-model:current-page="page"
        :page-size="pageSize"
        :total="total"
        layout="prev, pager, next"
        @current-change="loadHistory"
        background
        small
      />
    </div>

    <div class="empty-tip" v-if="!loading && items.length === 0">
      暂无扫描记录
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue"
import { Refresh } from "@element-plus/icons-vue"
import { getScanHistory } from "../api"

defineEmits<{
  viewResult: [scanId: number]
}>()

const page = ref(1)
const pageSize = 10
const total = ref(0)
const items = ref<any[]>([])
const loading = ref(false)

function formatTime(t: string) {
  if (!t) return "—"
  try {
    const d = new Date(t)
    const pad = (n: number) => String(n).padStart(2, "0")
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
  } catch {
    return t
  }
}

function scopeLabel(s: string) {
  const map: Record<string, string> = {
    messages: "消息",
    memories: "记忆",
    import_items: "导入内容",
    import_batches: "导入批次",
    logs: "日志",
  }
  return map[s] || s
}

async function loadHistory() {
  loading.value = true
  try {
    const d = await getScanHistory({ page: page.value, pageSize })
    items.value = d.items || []
    total.value = d.total || 0
  } catch {
    items.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

onMounted(loadHistory)
</script>

<style scoped>
.section-card { margin-bottom: 16px; border: 1px solid var(--el-border-color-light); }
.card-header { display: flex; align-items: center; justify-content: space-between; }
.card-title { font-size: 15px; font-weight: 600; }
.no-data { color: var(--el-text-color-placeholder); }
.risk-number.high { color: #dc2626; font-weight: 600; }
.pagination-wrap { display: flex; justify-content: center; margin-top: 12px; }
.empty-tip { text-align: center; padding: 24px 0; color: var(--el-text-color-secondary); font-size: 13px; }
</style>

<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-card shadow="never" class="section-card">
    <template #header>导入历史</template>
    <el-table :data="batches" size="small" v-if="batches.length > 0">
      <el-table-column prop="fileName" label="名称" show-overflow-tooltip />
      <el-table-column label="状态" width="90">
        <template #default="{row}">
          <el-tag :type="row.status === 'completed' ? 'success' : 'info'" size="small">
            {{ statusLabel(row.status) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="totalItems" label="条目数" width="60" />
      <el-table-column label="日期" width="140">
        <template #default="{row}">{{ fmtDate(row.createdAt) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="160">
        <template #default="{row}">
          <el-button text size="small" @click="$emit('view', row.id)">查看</el-button>
          <el-button text size="small" type="danger" @click="$emit('delete', row.id)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-empty v-else description="暂无导入历史" :image-size="50" />
  </el-card>
</template>

<script setup lang="ts">
import { fmtDate } from "../utils"

defineProps<{
  batches: any[]
}>()

defineEmits<{
  view: [id: string]
  delete: [id: string]
}>()

function statusLabel(status: string) {
  if (status === "completed") return "已完成"
  if (status === "pending") return "处理中"
  if (status === "failed") return "失败"
  return status || "未知"
}
</script>

<style scoped>
.section-card { margin-bottom: 12px; }
</style>

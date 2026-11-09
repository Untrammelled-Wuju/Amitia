<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-card shadow="never" class="section-card">
    <template #header>导入历史</template>
    <el-table :data="batches" size="small" v-if="batches.length > 0">
      <el-table-column prop="title" label="名称" show-overflow-tooltip />
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="statusType(row.status)" size="small">
            {{ statusLabel(row.status) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="messageCount" label="条目数" width="60" />
      <el-table-column label="日期" width="140">
        <template #default="{ row }">{{ fmtDate(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="160">
        <template #default="{ row }">
          <el-button text size="small" @click="$emit('view', row.id)"
            >查看</el-button
          >
          <el-button
            v-if="row.status === 'ready_for_promotion'"
            text
            size="small"
            type="primary"
            @click="$emit('promote', row)"
            >加入聊天</el-button
          >
          <el-button
            text
            size="small"
            type="danger"
            @click="$emit('delete', row.id)"
            >删除</el-button
          >
        </template>
      </el-table-column>
    </el-table>
    <el-empty v-else description="暂无导入历史" :image-size="50" />
  </el-card>
</template>

<script setup lang="ts">
import { fmtDate } from "../utils";

defineProps<{
  batches: any[];
}>();

defineEmits<{
  view: [id: string];
  delete: [id: string];
  promote: [row: any];
}>();

function statusLabel(status: string) {
  if (status === "promoted") return "已加入";
  if (status === "ready_for_promotion") return "待加入";
  if (status === "failed") return "失败";
  return status || "未知";
}

function statusType(status: string) {
  if (status === "promoted") return "success";
  if (status === "failed") return "danger";
  return "info";
}
</script>

<style scoped>
.section-card {
  margin-bottom: 12px;
}
</style>

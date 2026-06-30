<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-card shadow="never" class="section-card">
    <template #header>
      <div class="card-header-row">
        <span class="section-title">规则列表</span>
        <div style="display:flex;gap:8px">
          <el-button type="primary" size="small" @click="emit('create')">新建规则</el-button>
          <el-button size="small" :loading="resettingPresets" @click="emit('resetPresets')">恢复预设</el-button>
        </div>
      </div>
    </template>

    <el-table :data="rules" stripe size="small" v-loading="loading">
      <el-table-column prop="name" label="名称" min-width="120" show-overflow-tooltip />
      <el-table-column label="类型" width="80">
        <template #default="{ row }">
          <el-tag size="small">{{ typeLabel(row.ruleType) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="渠道" width="90">
        <template #default="{ row }">
          <el-tag :type="row.channel === 'all' ? 'primary' : row.channel === 'wechat' ? 'success' : 'info'" size="small">
            <span :title="channelLabel(row.channel)">{{ channelLabel(row.channel) }}</span>
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="安静时段" width="110">
        <template #default="{ row }">{{ row.quietStart }} - {{ row.quietEnd }}</template>
      </el-table-column>
      <el-table-column label="今日/上限" width="80">
        <template #default="{ row }">{{ row.sentCountToday }}/{{ row.maxPerDay }}</template>
      </el-table-column>
      <el-table-column label="上次发送" width="120">
        <template #default="{ row }">{{ row.lastSentAt || '从未' }}</template>
      </el-table-column>
      <el-table-column label="操作" width="260" fixed="right">
        <template #default="{ row }">
          <el-button link size="small" @click="emit('edit', row)">编辑</el-button>
          <el-button link size="small" type="warning" @click="emit('test', row)">测试</el-button>
          <el-button link size="small" type="success" @click="emit('trigger', row)">立即发送</el-button>
          <el-popconfirm title="确定删除此规则？" @confirm="emit('delete', row)">
            <template #reference>
              <el-button link size="small" type="danger">删除</el-button>
            </template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>
  </el-card>
</template>

<script setup lang="ts">
import type { ProactiveRule } from "../composables/useProactiveRules"

defineProps<{
  rules: ProactiveRule[]
  loading: boolean
  resettingPresets: boolean
  typeLabel: (type: string) => string
  channelLabel: (ch: string) => string
}>()

const emit = defineEmits<{
  (e: "create"): void
  (e: "edit", row: ProactiveRule): void
  (e: "test", row: ProactiveRule): void
  (e: "trigger", row: ProactiveRule): void
  (e: "delete", row: ProactiveRule): void
  (e: "resetPresets"): void
}>()
</script>

<style scoped>
.section-card { margin-bottom: 16px; }
.section-title { font-weight: 600; font-size: var(--ac-font-size-base); }
.card-header-row { display: flex; justify-content: space-between; align-items: center; }
</style>

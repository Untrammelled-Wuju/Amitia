<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-card class="section-card">
    <template #header>
      <span class="card-title">选择扫描范围</span>
    </template>
    <el-checkbox-group v-model="scope" class="scope-group">
      <el-checkbox label="messages">聊天消息</el-checkbox>
      <el-checkbox label="memories">记忆数据</el-checkbox>
      <el-checkbox label="import_items">导入内容</el-checkbox>
      <el-checkbox label="import_batches">导入批次</el-checkbox>
      <el-checkbox label="logs">操作日志（可选）</el-checkbox>
    </el-checkbox-group>
    <div style="margin-top: 12px">
      <el-button type="primary" :loading="scanning" @click="$emit('scan', scope)">
        开始扫描
      </el-button>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { ref } from "vue"

defineProps<{
  scanning: boolean
}>()

defineEmits<{
  scan: [scope: string[]]
}>()

const scope = ref<string[]>(["messages", "memories", "import_items"])
</script>

<style scoped>
.section-card { margin-bottom: 16px; border: 1px solid var(--el-border-color-light); }
.card-title { font-size: 15px; font-weight: 600; }
.scope-group { display: flex; flex-wrap: wrap; gap: 8px 20px; }
</style>

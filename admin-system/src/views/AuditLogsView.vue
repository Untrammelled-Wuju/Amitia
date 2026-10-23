<script setup lang="ts">
import { onMounted, ref } from "vue"
import { ElMessage } from "element-plus"

import { errorMessage } from "@/api/client"
import { fetchAuditLogs } from "@/api/releases"
import type { AuditLog } from "@/api/types"

const loading = ref(true)
const logs = ref<AuditLog[]>([])

function formatTime(value: string) {
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(new Date(value))
}

function actionLabel(action: string) {
  return {
    "release.create": "创建发布",
    "release.delete": "删除草稿",
    "release.artifact.upload": "上传制品",
    "release.publish": "发布版本",
    "release.rollout": "调整灰度",
    "release.rollback.pointer": "切换最新指针",
  }[action] || action
}

async function load() {
  loading.value = true
  try {
    logs.value = await fetchAuditLogs()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<template>
  <section class="panel full-panel" v-loading="loading">
    <div class="panel-heading">
      <div>
        <h2>发布审计</h2>
        <p>记录发布、上传、签名、暂停灰度和最新指针切换操作。</p>
      </div>
      <el-button @click="load">
        <el-icon><Refresh /></el-icon>
        刷新
      </el-button>
    </div>
    <el-table :data="logs" size="large">
      <el-table-column label="时间" min-width="180">
        <template #default="{ row }">{{ formatTime(row.createdAt) }}</template>
      </el-table-column>
      <el-table-column prop="actorName" label="操作者" min-width="120" />
      <el-table-column label="操作" min-width="140">
        <template #default="{ row }">{{ actionLabel(row.action) }}</template>
      </el-table-column>
      <el-table-column label="产品" width="100">
        <template #default="{ row }">
          {{ row.product === "desktop" ? "桌面端" : row.product === "android" ? "手机端" : "-" }}
        </template>
      </el-table-column>
      <el-table-column prop="channel" label="通道" width="100" />
      <el-table-column prop="target" label="目标版本" min-width="140" />
      <el-table-column label="结果" width="100">
        <template #default="{ row }">
          <el-tag :type="row.result === 'success' ? 'success' : 'danger'" effect="plain">
            {{ row.result === "success" ? "成功" : "失败" }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="message" label="详情" min-width="240" show-overflow-tooltip />
    </el-table>
  </section>
</template>

<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <el-dialog
    :model-value="visible"
    title="恢复数据库备份"
    width="500px"
    :close-on-click-modal="false"
    @update:model-value="emit('update:visible', $event)"
    @closed="handleClosed"
  >
    <template v-if="backup">
      <div class="restore-info">
        <div class="report-row"><span>备份名称：</span><strong>{{ backup.name }}</strong></div>
        <div class="report-row"><span>创建时间：</span><span>{{ formatCreatedAt(backup) }}</span></div>
      </div>

      <el-alert
        type="warning"
        title="恢复将替换当前数据库状态"
        :closable="false"
        show-icon
        style="margin: 12px 0"
      >
        <template #default>
          <p style="margin: 0; font-size: 12px">请确认当前重要数据已经另行备份。恢复完成后建议重新加载客户端。</p>
        </template>
      </el-alert>

      <div class="confirm-row">
        <span style="font-size: 13px">输入「确认恢复」以执行：</span>
        <el-input
          v-model="restoreConfirmText"
          placeholder='输入"确认恢复"'
          style="width: 160px"
          size="small"
        />
      </div>
    </template>

    <template #footer>
      <el-button @click="emit('update:visible', false)">取消</el-button>
      <el-button
        type="danger"
        :disabled="restoreConfirmText !== '确认恢复' || !backup"
        :loading="restoreExecuting"
        @click="executeRestore"
      >
        确认恢复
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref } from "vue";
import { ElMessage } from "element-plus";
import { apiClient } from "../../../composables/useApi";

const props = defineProps<{
  visible: boolean;
  backup: any | null;
}>();

const emit = defineEmits<{
  (e: "update:visible", value: boolean): void;
}>();

const restoreExecuting = ref(false);
const restoreConfirmText = ref("");

function handleClosed() {
  restoreConfirmText.value = "";
}

function formatCreatedAt(backup: any) {
  const value = backup?.createdAt || backup?.modTime || backup?.created_at;
  return value ? String(value).slice(0, 19) : "—";
}

async function executeRestore() {
  if (restoreConfirmText.value !== "确认恢复" || !props.backup?.name) return;
  restoreExecuting.value = true;
  try {
    const name = encodeURIComponent(String(props.backup.name));
    const res = await apiClient.post(`/api/storage/backups/${name}/restore`);
    const body = res.data?.data || res.data;
    ElMessage.success(body?.message || "恢复完成");
    emit("update:visible", false);
  } catch (err: any) {
    ElMessage.error("恢复失败: " + (err.response?.data?.message || err.response?.data?.error || err.message));
  } finally {
    restoreExecuting.value = false;
  }
}
</script>

<style scoped>
.restore-info {
  font-size: 14px;
}
.restore-info .report-row {
  padding: 4px 0;
  border-bottom: 1px solid var(--el-border-color-extra-light);
}
.restore-info .report-row strong {
  color: var(--el-text-color-primary);
}
.confirm-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 16px;
}
</style>

<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="cleanup-view">
    <div class="page-header">
      <h2>聊天记录清理</h2>
      <p class="page-desc">
        当前角色聊天统计与清理；确认前会创建全部数据完整备份
      </p>
    </div>

    <DatabaseHealthCard />

    <CleanupWorkflow />

    <DataPortabilityBackupPanel @restore="openRestoreDialog" />

    <RestoreBackupDialog
      :visible="restoreDialogVisible"
      :backup="restoreTarget"
      @update:visible="restoreDialogVisible = $event"
    />
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue";
import CleanupWorkflow from "./components/CleanupWorkflow.vue";
import DataPortabilityBackupPanel from "./components/DataPortabilityBackupPanel.vue";
import RestoreBackupDialog from "./components/RestoreBackupDialog.vue";
import DatabaseHealthCard from "./components/DatabaseHealthCard.vue";

const restoreDialogVisible = ref(false);
const restoreTarget = ref<any>(null);

function openRestoreDialog(backup: any) {
  restoreTarget.value = backup;
  restoreDialogVisible.value = true;
}
</script>

<style scoped>
.cleanup-view {
  padding: 20px;
  max-width: 800px;
}

.page-header {
  margin-bottom: 20px;
}
.page-header h2 {
  font-size: 20px;
  font-weight: 600;
  margin: 0 0 4px 0;
  color: var(--el-text-color-primary);
}
.page-desc {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  margin: 0;
}

@media (max-width: 600px) {
  .cleanup-view {
    padding: 12px;
    max-width: 100%;
  }
}
</style>

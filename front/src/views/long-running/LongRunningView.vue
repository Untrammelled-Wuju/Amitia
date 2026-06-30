<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="lr-page">
    <h2 class="page-title">长期运行维护</h2>
    <StatusPanel ref="statusPanelRef" />
    <ManualActionsPanel @action-completed="onActionCompleted" />
    <ConfigPanel />
  </div>
</template>

<script setup lang="ts">
import { ref } from "vue"
import StatusPanel from "./components/StatusPanel.vue"
import ManualActionsPanel from "./components/ManualActionsPanel.vue"
import ConfigPanel from "./components/ConfigPanel.vue"

const statusPanelRef = ref<InstanceType<typeof StatusPanel> | null>(null)

function onActionCompleted() {
  statusPanelRef.value?.refresh()
}
</script>

<style scoped>
.lr-page {
  padding: 20px 24px;
  max-width: 900px;
  margin: 0 auto;
}
.page-title {
  font-size: 20px;
  font-weight: 600;
  color: var(--ac-color-text);
  margin: 0 0 16px 0;
}
</style>

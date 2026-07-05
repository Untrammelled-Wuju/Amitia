<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="settings-layout">
    <h2 class="page-title">设置</h2>
    <div class="settings-tabs">
      <router-link to="/settings/runtime" class="settings-tab" active-class="settings-tab-active">运行维护</router-link>
      <router-link to="/settings/ai-config" class="settings-tab" active-class="settings-tab-active">AI 配置</router-link>
      <router-link to="/settings/system" class="settings-tab" active-class="settings-tab-active">系统设置</router-link>
      <router-link to="/settings/model" class="settings-tab" active-class="settings-tab-active">模型配置</router-link>
      <router-link to="/settings/safety" class="settings-tab" active-class="settings-tab-active">安全诊断</router-link>
      <router-link to="/settings/maintenance" class="settings-tab" active-class="settings-tab-active">维护诊断</router-link>
    </div>
    <div class="settings-content">
      <router-view />
    </div>
  </div>
</template>

<script setup lang="ts">
</script>

<style scoped>
.settings-layout { }
.page-title { font-size: var(--ac-font-size-lg); font-weight: 600; margin: 0 0 14px 0; color: var(--ac-color-text); }
.settings-tabs {
  display: flex;
  gap: 0;
  border-bottom: 2px solid var(--el-border-color-light);
  margin-bottom: 20px;
  flex-wrap: wrap;
}
.settings-tab {
  padding: 10px 22px;
  font-size: 14px;
  color: var(--el-text-color-secondary);
  text-decoration: none;
  border-bottom: 2px solid transparent;
  margin-bottom: -2px;
  transition: all .2s;
  white-space: nowrap;
}
.settings-tab:hover { color: var(--el-color-primary); }
.settings-tab-active {
  color: var(--el-color-primary);
  border-bottom-color: var(--el-color-primary);
  font-weight: 500;
}
.settings-content {
  min-height: 400px;
}
</style>

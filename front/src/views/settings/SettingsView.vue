<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="settings-layout">
    <div class="settings-tabs">
      <router-link
        to="/settings/runtime"
        class="settings-tab"
        active-class="settings-tab-active"
        >运行维护</router-link
      >
      <router-link
        to="/settings/deployment"
        class="settings-tab"
        active-class="settings-tab-active"
        >部署模式</router-link
      >
      <router-link
        to="/settings/system"
        class="settings-tab"
        active-class="settings-tab-active"
        >系统设置</router-link
      >
      <router-link
        to="/settings/notifications"
        class="settings-tab"
        active-class="settings-tab-active"
        >通知</router-link
      >
      <router-link
        to="/settings/advanced"
        class="settings-tab"
        active-class="settings-tab-active"
        >高级系统</router-link
      >
      <router-link
        to="/settings/ui-providers"
        class="settings-tab"
        active-class="settings-tab-active"
        >界面提供者</router-link
      >
      <router-link
        to="/settings/model"
        class="settings-tab"
        active-class="settings-tab-active"
        >模型配置</router-link
      >
      <router-link
        to="/settings/safety"
        class="settings-tab"
        active-class="settings-tab-active"
        >安全</router-link
      >
      <router-link
        to="/settings/maintenance"
        class="settings-tab"
        active-class="settings-tab-active"
        >维护诊断</router-link
      >
      <router-link
        to="/settings/prompt-trace"
        class="settings-tab"
        active-class="settings-tab-active"
        >Prompt Trace</router-link
      >
      <router-link
        to="/settings/about"
        class="settings-tab"
        active-class="settings-tab-active"
        >关于</router-link
      >
    </div>
    <div class="settings-content">
      <ExtensionSlot
        slot-id="system.status.item"
        :context="settingsContext"
        fallback="none"
        layout="inline"
        surface-role="status"
        class="settings-status-slot"
      />
      <router-view />
      <ExtensionSlot
        slot-id="system.settings.section"
        :context="settingsContext"
        fallback="none"
        layout="stack"
        surface-role="main"
      />
      <ExtensionSlot
        slot-id="extension.settings.section"
        :context="settingsContext"
        fallback="none"
        layout="stack"
        surface-role="main"
      />
      <ExtensionSlot
        slot-id="extension.settings.page"
        :context="settingsContext"
        fallback="none"
        layout="stack"
        surface-role="main"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useRoute } from "vue-router";
import ExtensionSlot from "@/components/extension/ExtensionSlot.vue";

const route = useRoute();
const settingsContext = computed(() => ({
  route: route.fullPath,
  routeName: String(route.name ?? ""),
  section: String(route.path.split("/").filter(Boolean).at(-1) ?? "settings"),
}));
</script>

<style scoped>
.settings-layout {
}
.page-title {
  font-size: var(--ac-font-size-lg);
  font-weight: 600;
  margin: 0 0 14px 0;
  color: var(--ac-color-text);
}
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
  transition: all 0.2s;
  white-space: nowrap;
}
.settings-tab:hover {
  color: var(--el-color-primary);
}
.settings-tab-active {
  color: var(--el-color-primary);
  border-bottom-color: var(--el-color-primary);
  font-weight: 500;
}
.settings-content {
  min-height: 400px;
}
.settings-status-slot {
  margin-bottom: 12px;
}
</style>

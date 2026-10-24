<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="settings-page" :class="{ 'settings-refined': refinedAppearance }" data-amitia-ui="settings.page">
    <div class="settings-compact-navigation">
      <label for="settings-section-mobile">设置分类</label>
      <select id="settings-section-mobile" :value="currentEntry" @change="navigateToSetting">
        <option value="" disabled>选择设置项目</option>
        <optgroup v-for="group in settingsGroups" :key="group.title" :label="group.title">
          <option v-for="item in group.items" :key="item.path" :value="item.path">{{ item.label }}</option>
        </optgroup>
      </select>
    </div>
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
      v-for="slotId in settingsSlots"
      :key="slotId"
      :slot-id="slotId"
      :context="settingsContext"
      fallback="none"
      layout="stack"
      surface-role="main"
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import ExtensionSlot from "@/components/extension/ExtensionSlot.vue";
import { resolveSettingsEntry, settingsGroups } from "@/navigation/settings-navigation";

const route = useRoute();
const router = useRouter();
const currentEntry = computed(() => resolveSettingsEntry(route.path));
const refinedAppearance = computed(() => ![
  "/settings/about",
  "/settings/system-logs",
  "/settings/prompt-trace",
].some(path => route.path === path || route.path.startsWith(`${path}/`)));
const settingsSlots = [
  "system.settings.section",
  "extension.settings.section",
  "extension.settings.page",
];
const settingsContext = computed(() => ({
  route: route.fullPath,
  routeName: String(route.name ?? ""),
  section: String(route.path.split("/").filter(Boolean).at(-1) ?? "settings"),
}));

function navigateToSetting(event: Event) {
  const path = (event.target as HTMLSelectElement).value;
  if (settingsGroups.some(group => group.items.some(item => item.path === path))) {
    void router.push(path);
  }
}
</script>

<style scoped>
.settings-page { min-width: 0; }
.settings-status-slot { margin-bottom: 12px; }
.settings-compact-navigation { display: none; }
.settings-refined :deep(.el-card) {
  background: transparent;
  border: 0;
  border-bottom: 1px solid var(--surface-border);
  border-radius: 0;
  box-shadow: none;
  margin-bottom: 24px;
}
.settings-refined :deep(.el-card__header) {
  padding: 8px 0 12px;
  border-bottom: 0;
  font-weight: 600;
}
.settings-refined :deep(.el-card__body) { padding: 8px 0 24px; }
@media (max-width: 767px) {
  .settings-compact-navigation { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; }
  .settings-compact-navigation label { flex-shrink: 0; color: var(--text-secondary); font-size: var(--ac-font-size-sm); }
  .settings-compact-navigation select { min-width: 0; width: 100%; min-height: 38px; padding: 8px 12px; border: 1px solid var(--surface-border); border-radius: var(--ac-radius-sm); background: var(--surface-bg); color: var(--text-primary); font: inherit; font-size: var(--ac-font-size-base); }
}
</style>

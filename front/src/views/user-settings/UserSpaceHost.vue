<template>
  <div v-if="isAccount" class="my-space-page">
    <div class="my-space-compact-navigation">
      <label for="space-section-mobile">空间分类</label>
      <select id="space-section-mobile" :value="currentAccountEntry" @change="navigateToSection">
        <option value="" disabled>选择空间功能</option>
        <option v-for="item in accountNavigation" :key="item.path" :value="item.path">{{ item.label }}</option>
      </select>
    </div>
    <div class="my-space-route-content">
      <component :is="activeAccountView" v-if="activeAccountView" :key="route.path" />
      <el-empty v-else description="该空间页面不存在" />
    </div>
  </div>

  <div v-else class="settings-page" :class="{ 'settings-refined': refinedAppearance }" data-amitia-ui="settings.page">
    <div class="settings-compact-navigation">
      <label for="settings-section-mobile">设置分类</label>
      <select id="settings-section-mobile" :value="currentSettingEntry" @change="navigateToSetting">
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
    <component :is="activeSettingsView" v-if="activeSettingsView" :key="route.path" />
    <el-empty v-else description="该设置页面不存在" />
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
import { computed, defineAsyncComponent, type Component } from "vue";
import { useRoute, useRouter } from "vue-router";
import ExtensionSlot from "@/components/extension/ExtensionSlot.vue";
import { resolveSettingsEntry, settingsGroups } from "@/navigation/settings-navigation";
import { accountNavigation, resolveAccountEntry } from "@/navigation/account-navigation";

const route = useRoute();
const router = useRouter();

const settingsViews: Record<string, Component> = {
  overview: defineAsyncComponent(() => import("@/views/dashboard/RunView.vue")),
  data: defineAsyncComponent(() => import("@/views/dashboard/DataView.vue")),
  system: defineAsyncComponent(() => import("@/views/settings/SystemSettingsPanel.vue")),
  "data-management": defineAsyncComponent(() => import("@/views/settings/DataManagementPanel.vue")),
  notifications: defineAsyncComponent(() => import("@/views/settings/NotificationSettingsView.vue")),
  advanced: defineAsyncComponent(() => import("@/views/settings/AdvancedSystemView.vue")),
  temporal: defineAsyncComponent(() => import("@/views/settings/TemporalSettingsView.vue")),
  safety: defineAsyncComponent(() => import("@/views/safety-settings/SafetySettingsView.vue")),
  privacy: defineAsyncComponent(() => import("@/views/privacy/Privacy.vue")),
  "usage-boundary": defineAsyncComponent(() => import("@/views/usage-boundary/UsageBoundary.vue")),
  "privacy-scan": defineAsyncComponent(() => import("@/views/privacy-scan/PrivacyScanView.vue")),
  maintenance: defineAsyncComponent(() => import("@/views/maintenance-diagnostics/MaintenanceDiagnosticsView.vue")),
  "system-logs": defineAsyncComponent(() => import("@/views/logs/LogsView.vue")),
  feedback: defineAsyncComponent(() => import("@/views/settings/UserFeedbackPanel.vue")),
  "prompt-trace": defineAsyncComponent(() => import("@/views/logs/PromptTraceView.vue")),
  about: defineAsyncComponent(() => import("@/views/settings/AboutPanel.vue")),
  theme: defineAsyncComponent(() => import("@/views/settings/ThemeSettingsView.vue")),
};

const accountViews: Record<string, Component> = {
  profile: defineAsyncComponent(() => import("@/views/user-settings/UserSettingsView.vue")),
  identity: defineAsyncComponent(() => import("@/views/user-settings/SpaceIdentityView.vue")),
  runtime: defineAsyncComponent(() => import("@/views/user-settings/MySpaceRuntimeView.vue")),
  devices: defineAsyncComponent(() => import("@/views/user-settings/MySpaceDevicesView.vue")),
};

const isAccount = computed(() => route.path === "/my-space" || route.path.startsWith("/my-space/"));
const currentSettingEntry = computed(() => resolveSettingsEntry(route.path));
const currentAccountEntry = computed(() => resolveAccountEntry(route.path));
const settingsSlots = [
  "system.settings.section",
  "extension.settings.section",
  "extension.settings.page",
];
const refinedAppearance = computed(() => ![
  "/settings/about",
  "/settings/system-logs",
  "/settings/prompt-trace",
].some((path) => route.path === path || route.path.startsWith(`${path}/`)));
const settingsContext = computed(() => ({
  route: route.fullPath,
  routeName: String(route.name ?? ""),
  section: String(route.path.split("/").filter(Boolean).at(-1) ?? "settings"),
}));

const activeSettingsView = computed(() => {
  if (route.path === "/settings" || route.path === "/settings/model" || route.path === "/my-space") return null;
  if (route.path.startsWith("/settings/model/")) {
    return defineAsyncComponent(() => import("@/views/settings/SettingsModelConfigSurface.vue"));
  }
  const key = route.path.split("/")[2] ?? "";
  return settingsViews[key] ?? null;
});

const activeAccountView = computed(() => {
  const key = route.path.split("/")[2] ?? "profile";
  return accountViews[key] ?? null;
});

function navigateToSetting(event: Event) {
  const path = (event.target as HTMLSelectElement).value;
  if (settingsGroups.some((group) => group.items.some((item) => item.path === path))) {
    void router.push(path);
  }
}

function navigateToSection(event: Event) {
  const path = (event.target as HTMLSelectElement).value;
  if (accountNavigation.some((item) => item.path === path)) void router.push(path);
}

if (route.path === "/settings") void router.replace("/settings/system");
if (route.path === "/settings/model") void router.replace("/settings/model/llm");
if (route.path === "/my-space") void router.replace("/my-space/profile");
</script>

<style scoped>
.my-space-page { min-width: 0; width: 100%; }
.my-space-route-content { min-width: 0; width: 100%; }
.my-space-compact-navigation { display: none; }
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
  .my-space-compact-navigation,
  .settings-compact-navigation { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; }
  .my-space-compact-navigation label,
  .settings-compact-navigation label { flex-shrink: 0; color: var(--text-secondary); font-size: var(--ac-font-size-sm); }
  .my-space-compact-navigation select,
  .settings-compact-navigation select { min-width: 0; width: 100%; min-height: 40px; padding: 8px 12px; border: 1px solid var(--surface-border); border-radius: var(--ac-radius-sm); background: var(--surface-bg); color: var(--text-primary); font: inherit; font-size: var(--ac-font-size-base); }
}
</style>

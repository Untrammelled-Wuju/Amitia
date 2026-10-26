<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="app-shell" :class="{ 'is-mobile': isMobile }">
    <div class="app-body workbench" :class="{ 'is-desktop-shell': isDesktopShell() }">
      <UIProviderHost v-if="!isMobile" capability="app.navigation" :fallback="SideNav" :context="{ viewport: 'desktop' }" :username="authUsername" :avatar="authAvatar" :theme="resolvedTheme" @toggle-theme="toggleTheme" />

      <div class="app-main workspace" :class="{ 'is-desktop-shell': isDesktopShell() }" :style="workspaceSidebarStyle">
        <div v-if="workspaceSidebarVisible && workspaceSidebarTarget === 'provider'" class="extension-workspace-sidebar">
          <UIProviderHost
            capability="app.workspace.sidebar"
            :provider-id="activeWorkspaceSidebarProvider?.providerId"
            :fallback="PluginWorkspaceSidebar"
            :context="workspaceSidebarContext"
            :title="activeNavigationItem?.label || ''"
            :items="activeNavigationItem?.sidebarItems || []"
            frameless
          />
        </div>
        <PluginWorkspaceSidebar
          v-else-if="workspaceSidebarVisible && workspaceSidebarTarget === 'links'"
          :title="activeNavigationItem?.label || ''"
          :items="activeNavigationItem?.sidebarItems || []"
        />
        <SettingsSidebar v-else-if="workspaceSidebarVisible && appSectionId === 'settings'" />
        <AccountSidebar v-else-if="workspaceSidebarVisible && appSectionId === 'account'" />
        <ChannelSidebar v-else-if="workspaceSidebarVisible && appSectionId === 'channels'" />
        <WorkspaceSidebar v-else-if="workspaceSidebarVisible && appSectionId === 'home'" />
        <SectionSidebar v-else-if="workspaceSidebarVisible && sectionSidebarId" :section-id="sectionSidebarId" />
        <div
          v-if="workspaceSidebarVisible"
          class="workspace-sidebar-resizer"
          :class="{ 'is-resizing': workspaceSidebarResizing }"
          role="separator"
          aria-label="调整第二栏宽度"
          aria-orientation="vertical"
          :aria-valuemin="WORKSPACE_SIDEBAR_MIN_WIDTH"
          :aria-valuemax="workspaceSidebarMaxWidth"
          :aria-valuenow="workspaceSidebarWidth"
          tabindex="0"
          @pointerdown="startWorkspaceSidebarResize"
          @keydown.left.prevent="resizeWorkspaceSidebarBy(-12)"
          @keydown.right.prevent="resizeWorkspaceSidebarBy(12)"
        />
        <main class="app-content workspace-surface" :class="{ 'workspace-surface--chat': isChatPage }" data-amitia-ui="app.content">
          <header v-if="isMobile" class="mobile-header" data-amitia-ui="app.header">
            <span class="mobile-title">{{ pageTitle }}</span>
            <span class="mobile-status">
              <span class="dot" :class="modelClass"></span>
              {{ currentCharName || "未配置" }}
            </span>
          </header>

          <div class="workbench-content-row" :class="{ 'workbench-content-row--chat': isChatPage }">
          <div
            class="content-scroll"
            :inert="isChatPage && workbench.visible && (isMobile || windowWidth < 1024)"
            :class="{ 'no-padding': isContentPaddingDisabled }"
          >
            <div
              class="content-scroll-inner"
              data-amitia-ui="app.content"
              :class="{
                'no-padding': isContentPaddingDisabled,
                'fill-height': isContentFullHeight,
              }"
            >
              <UIProviderHost capability="app.workspace" :fallback="BuiltinWorkspaceHost" :context="{ viewport: isMobile ? 'mobile' : 'desktop' }">
                <slot />
              </UIProviderHost>
            </div>
          </div>
          <WorkbenchDock v-if="isChatPage && workbench.visible" :mobile="isMobile || windowWidth < 1024" />
          </div>
        </main>
      </div>
    </div>

    <UIProviderHost v-if="isMobile" capability="app.navigation" :fallback="MobileNav" :context="{ viewport: 'mobile' }" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, provide, watch, watchEffect } from "vue";
import { useRouter } from "vue-router";
import SideNav from "./SideNav.vue";
import PluginWorkspaceSidebar from "./PluginWorkspaceSidebar.vue";
import SettingsSidebar from "./SettingsSidebar.vue";
import AccountSidebar from "./AccountSidebar.vue";
import ChannelSidebar from "./ChannelSidebar.vue";
import WorkspaceSidebar from "./WorkspaceSidebar.vue";
import SectionSidebar from "./SectionSidebar.vue";
import MobileNav from "./MobileNav.vue";
import UIProviderHost from "./ui-runtime/UIProviderHost.vue";
import BuiltinWorkspaceHost from "./ui-runtime/BuiltinWorkspaceHost.vue";
import WorkbenchDock from "./workbench/WorkbenchDock.vue";
import { useWorkbenchStore, type WorkbenchOpenCommand } from "@/stores/workbench";
import { useTheme } from "../composables/useTheme";
import {
  apiClient,
} from "../composables/useApi";
import { isUINavigationItemActive, resolvePageTitle, useUINavigationRegistry, type UINavigationItem } from "@/ui-runtime/navigationRegistry";
import type { UIProviderDefinition } from "@/ui-runtime/types";
import { useAppStore } from "@/stores/app";
import { useExtensionUIStore } from "@/stores/extensionUI";
import { isDesktopShell } from "../runtime/runtime-capabilities";
import { resolveAppSection } from "@/navigation/appSections";
import {
  WORKSPACE_SIDEBAR_DEFAULT_WIDTH,
  WORKSPACE_SIDEBAR_MIN_WIDTH,
  clampWorkspaceSidebarWidth,
  maxWorkspaceSidebarWidth,
} from "@/navigation/workspaceSidebarWidth";

const router = useRouter();
const extensionUIStore = useExtensionUIStore();
const workbench = useWorkbenchStore();
function handleWorkbenchOpen(event: Event) {
  const detail = (event as CustomEvent<WorkbenchOpenCommand>).detail;
  workbench.openCommand(detail);
}
const { items: navigationItems } = useUINavigationRegistry();
let healthInterval: number | null = null;
const {
  state: theme,
  resolvedMode: resolvedTheme,
  toggleLightDark: toggleTheme,
} = useTheme();

const windowWidth = ref(window.innerWidth);
const isMobile = computed(() => windowWidth.value < 768);
const WORKSPACE_SIDEBAR_STORAGE_KEY = "uai-workspace-sidebar-width";
function readWorkspaceSidebarWidth() {
  const stored = Number(localStorage.getItem(WORKSPACE_SIDEBAR_STORAGE_KEY));
  return Number.isFinite(stored) && stored > 0 ? stored : WORKSPACE_SIDEBAR_DEFAULT_WIDTH;
}
const workspaceSidebarPreferredWidth = ref(readWorkspaceSidebarWidth());
const workspaceSidebarResizing = ref(false);
const workspaceSidebarMaxWidth = computed(() => maxWorkspaceSidebarWidth(windowWidth.value));
const workspaceSidebarWidth = computed(() => clampWorkspaceSidebarWidth(
  workspaceSidebarPreferredWidth.value,
  windowWidth.value,
));
const activeNavigationItem = computed<UINavigationItem | null>(() =>
  navigationItems.value.find((item) => isUINavigationItemActive(router.currentRoute.value.path, item)) ?? null,
);
const activeWorkspaceSidebarProvider = computed<UIProviderDefinition | null>(() => {
  const item = activeNavigationItem.value;
  if (!item?.extensionId) return null;
  const providers = extensionUIStore
    .getProviders("app.workspace.sidebar")
    .filter((provider) => provider.enabled && !provider.builtin && provider.extensionId === item.extensionId)
    .sort((left, right) => (right.priority ?? 0) - (left.priority ?? 0) || left.providerId.localeCompare(right.providerId));
  if (item.sidebarProviderId) {
    return providers.find((provider) => provider.providerId === item.sidebarProviderId) ?? null;
  }
  if (!item.containerId) return null;
  return providers.find((provider) => String(provider.metadata?.containerId ?? "").trim() === item.containerId) ?? null;
});
const workspaceSidebarTarget = computed<"provider" | "links" | "builtin" | "none">(() => {
  if (isMobile.value || appStore.sidebarCollapsed) return "none";
  if (activeWorkspaceSidebarProvider.value) return "provider";
  if (activeNavigationItem.value?.sidebarItems?.length) return "links";
  if (appSectionId.value) return "builtin";
  return "none";
});
const workspaceSidebarVisible = computed(() => workspaceSidebarTarget.value !== "none");
const workspaceSidebarContext = computed<Record<string, unknown>>(() => ({
  viewport: "desktop",
  containerId: activeNavigationItem.value?.containerId || "",
  navigationItemId: activeNavigationItem.value?.id || "",
  navigationLabel: activeNavigationItem.value?.label || "",
  defaultNavigationRoute: activeNavigationItem.value?.sidebarItems?.[0]?.route || "",
  routeQuery: router.currentRoute.value.query,
}));
const workspaceSidebarStyle = computed<Record<string, string>>(() => ({
  "--ac-workspace-sidebar-width": `${workspaceSidebarWidth.value}px`,
}));
let workspaceSidebarResizeStartX = 0;
let workspaceSidebarResizeStartWidth = 0;

const isChatPage = computed(() => router.currentRoute.value.path === "/chat");
const isSettingsPage = computed(() => {
  const path = router.currentRoute.value.path;
  return path === "/settings" || path.startsWith("/settings/");
});
const appSectionId = computed(() => resolveAppSection(router.currentRoute.value.path));
const sectionSidebarId = computed(() => {
  const id = appSectionId.value;
  if (!id || id === "home" || id === "channels" || id === "account" || id === "settings") return null;
  return id;
});
const isExtensionPage = computed(() => router.currentRoute.value.path.startsWith("/extension/page/"));
const isFullHeightRoute = computed(() => router.currentRoute.value.meta.fullHeight === true || isExtensionPage.value);
const isContentPaddingDisabled = computed(() => {
  const path = router.currentRoute.value.path;
  return (
    path === "/chat" ||
    path === "/emotes" ||
    path === "/character" ||
    path.startsWith("/character/") ||
    path.startsWith("/characters/") ||
    isFullHeightRoute.value
  );
});
const isContentFullHeight = computed(() => {
  const path = router.currentRoute.value.path;
  return path === "/emotes" ||
    path === "/character" ||
    path.startsWith("/character/") ||
    path.startsWith("/characters/") ||
    path === "/creative-workshop/character-cards" ||
    path.startsWith("/extensions/workflows/") ||
    isFullHeightRoute.value;
});
const pageTitle = computed(() => resolvePageTitle(router.currentRoute.value.path, navigationItems.value));

const health = ref({
  appStatus: "running",
  deployMode: "desktop-local",
  database: "ok",
  model: "not_configured",
  web: "enabled",
});

const currentCharName = ref("");

const authUsername = ref("");
const appStore = useAppStore();
watchEffect(() => {
  document.documentElement.style.setProperty("--workspace-navigation-rail-width", "64px");
  document.documentElement.style.setProperty(
    "--workspace-sidebar-effective-width",
    workspaceSidebarVisible.value ? `${workspaceSidebarWidth.value}px` : "0px",
  );
});
const authAvatar = computed(() => appStore.avatar);
const extensionRuntimeAvailable = ref(false);

const modelClass = computed(() =>
  health.value.model === "configured" ? "status-on" : "status-off",
);

function handleWindowResize() {
  windowWidth.value = window.innerWidth;
}

function setWorkspaceSidebarWidth(width: number) {
  const next = clampWorkspaceSidebarWidth(width, windowWidth.value);
  workspaceSidebarPreferredWidth.value = next;
  localStorage.setItem(WORKSPACE_SIDEBAR_STORAGE_KEY, String(next));
}

function startWorkspaceSidebarResize(event: PointerEvent) {
  if (event.button !== 0 || !workspaceSidebarVisible.value) return;
  event.preventDefault();
  workspaceSidebarResizeStartX = event.clientX;
  workspaceSidebarResizeStartWidth = workspaceSidebarWidth.value;
  workspaceSidebarResizing.value = true;
  document.documentElement.classList.add("is-resizing-workspace-sidebar");
  window.addEventListener("pointermove", handleWorkspaceSidebarResize);
  window.addEventListener("pointerup", stopWorkspaceSidebarResize, { once: true });
  window.addEventListener("pointercancel", stopWorkspaceSidebarResize, { once: true });
}

function handleWorkspaceSidebarResize(event: PointerEvent) {
  if (!workspaceSidebarResizing.value) return;
  setWorkspaceSidebarWidth(workspaceSidebarResizeStartWidth + event.clientX - workspaceSidebarResizeStartX);
}

function stopWorkspaceSidebarResize() {
  workspaceSidebarResizing.value = false;
  document.documentElement.classList.remove("is-resizing-workspace-sidebar");
  window.removeEventListener("pointermove", handleWorkspaceSidebarResize);
  window.removeEventListener("pointerup", stopWorkspaceSidebarResize);
  window.removeEventListener("pointercancel", stopWorkspaceSidebarResize);
}

function resizeWorkspaceSidebarBy(delta: number) {
  setWorkspaceSidebarWidth(workspaceSidebarWidth.value + delta);
}

watch(workspaceSidebarWidth, (width) => {
  if (width !== workspaceSidebarPreferredWidth.value) {
    workspaceSidebarPreferredWidth.value = width;
    localStorage.setItem(WORKSPACE_SIDEBAR_STORAGE_KEY, String(width));
  }
});

provide("theme", theme);
async function refreshAll() {
  await fetchHealth();
  await fetchActiveCharacter();
}
provide("refreshHealth", refreshAll);
provide("resolvedTheme", resolvedTheme);
provide("currentCharName", currentCharName);
provide("authUsername", authUsername);

async function fetchHealth() {
  try {
    const res = await apiClient.get("/api/health");
    if (res.data?.model) {
      health.value = { ...health.value, ...res.data };
    }
  } catch {}
}

async function fetchActiveCharacter() {
  const cached = localStorage.getItem("uai-default-char");
  if (cached) {
    try {
      const dc = JSON.parse(cached);
      if (dc.name) currentCharName.value = dc.name;
    } catch {}
  }
  try {
    const res = await apiClient.get("/api/characters");
    const chars = res.data?.data || res.data;
    if (Array.isArray(chars)) {
      const defaultChar = chars.find((c: any) => c.isDefault);
      const active = chars.find((c: any) => c.isActive);
      const first = chars.find((c: any) => c.status !== "disabled");
      const selected = defaultChar || active || first;
      currentCharName.value = (selected || {}).name || "";
      if (selected && selected.isDefault) {
        localStorage.setItem(
          "uai-default-char",
          JSON.stringify({
            id: selected.id,
            name: selected.name,
            identity: selected.identity || selected.personality || "",
            updatedAt: Date.now(),
          }),
        );
      }
    }
  } catch {}
}

async function fetchSpaceProfile() {
  try {
    const res = await apiClient.get("/api/space/profile");
    const profile = res.data?.data || res.data;
    authUsername.value = String(profile?.displayName || profile?.nickname || "本地用户");
    appStore.setAvatar(String(profile?.avatar || ""));
  } catch {
    authUsername.value = "本地用户";
  }
}

onMounted(() => {
  window.addEventListener('amitia:workbench:open', handleWorkbenchOpen);
  window.addEventListener("resize", handleWindowResize);
  fetchHealth();
  fetchActiveCharacter();
  fetchSpaceProfile();
  extensionUIStore.refreshSnapshot(true).then(() => {
    extensionRuntimeAvailable.value = true;
  });

  healthInterval = window.setInterval(() => {
    void fetchHealth();
  }, 30000);
});

onUnmounted(() => {
  document.documentElement.style.removeProperty("--workspace-navigation-rail-width");
  document.documentElement.style.removeProperty("--workspace-sidebar-effective-width");
  stopWorkspaceSidebarResize();
  window.removeEventListener('amitia:workbench:open', handleWorkbenchOpen);
  if (healthInterval !== null) {
    window.clearInterval(healthInterval);
    healthInterval = null;
  }
  window.removeEventListener("resize", handleWindowResize);
});
</script>

<style scoped>
.app-shell {
  transition: background-color 0.3s ease;
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--workbench-bg);
}

.app-body {
  display: flex;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.app-body.is-desktop-shell {
  background: var(--workbench-sidebar-bg);
}

.app-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: row;
  overflow: hidden;
  background: var(--workbench-bg);
  border-left: 1px solid var(--surface-border);
}

.app-main.is-desktop-shell {
  border-top: 1px solid var(--surface-border);
  border-top-left-radius: 20px;
  overflow: hidden;
}

.workspace-sidebar-resizer {
  position: relative;
  z-index: 12;
  flex: 0 0 6px;
  width: 6px;
  margin-left: -6px;
  cursor: col-resize;
  touch-action: none;
}

.extension-workspace-sidebar {
  width: var(--ac-workspace-sidebar-width);
  flex: 0 0 var(--ac-workspace-sidebar-width);
  min-width: 0;
  min-height: 0;
  display: flex;
  overflow: hidden;
  box-sizing: border-box;
  padding: 0 8px 18px;
  background: var(--workbench-bg);
  border-right: 1px solid var(--surface-border);
}

.extension-workspace-sidebar > :deep(*) {
  width: 100%;
  min-width: 0;
}

.extension-workspace-sidebar :deep(.schema-ui-renderer__content > .schema-ui-page) {
  gap: 0;
  padding: 0;
}

.extension-workspace-sidebar :deep(.schema-ui-renderer__content > .schema-ui-page > h3.schema-ui-text:first-child) {
  margin: 0;
  padding: 18px 10px 12px;
  color: var(--text-primary);
  font-size: var(--ac-font-size-xl);
  font-weight: 650;
  line-height: normal;
}

.workspace-sidebar-resizer::after {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  width: 1px;
  background: transparent;
  content: "";
  transition: background-color 0.16s ease;
}

.workspace-sidebar-resizer:hover::after,
.workspace-sidebar-resizer:focus-visible::after,
.workspace-sidebar-resizer.is-resizing::after {
  background: var(--ac-color-primary);
}

.workspace-sidebar-resizer:focus-visible {
  outline: none;
}

:global(html.is-resizing-workspace-sidebar),
:global(html.is-resizing-workspace-sidebar *) {
  cursor: col-resize !important;
  user-select: none !important;
}

.workbench-content-row {display:flex;flex:1;min-height:0;min-width:0;position:relative}
.workbench-content-row .content-scroll {min-width:0;flex:1}
.app-content {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--workbench-bg);
}

.app-content.is-login {
  align-items: center;
  justify-content: center;
  background: var(--tp-page-glow, none), var(--console-bg);
}

.content-scroll {
  transition: background-color 0.3s ease;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  background: transparent;
}

.content-scroll.no-padding {
  padding: 0;
}

.content-scroll-inner {
  padding: 20px 24px;
}

.content-scroll-inner.no-padding {
  padding: 0;
}

.content-scroll-inner.fill-height {
  height: 100%;
  min-height: 100%;
  box-sizing: border-box;
}

.workspace-surface--chat .content-scroll {
  display: flex;
  min-height: 0;
}

.workspace-surface--chat .content-scroll-inner {
  display: flex;
  min-width: 0;
  flex: 1;
}

.mobile-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: var(--ac-mobile-header-height);
  padding: 0 16px;
  background: var(--ac-color-surface);
  border-bottom: 1px solid var(--ac-color-border-light);
  flex-shrink: 0;
  padding-top: env(safe-area-inset-top, 0px);
}

.mobile-title {
  font-weight: 600;
  font-size: var(--ac-font-size-base);
  color: var(--ac-color-text);
}

.mobile-status {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: var(--ac-font-size-xs);
  color: var(--ac-color-text-secondary);
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mobile-status .dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
}

.mobile-status .status-on {
  background: var(--ac-color-success);
}

.mobile-status .status-off {
  background: var(--ac-color-text-muted);
}

.is-mobile .app-main {
  border-left: 0;
}

.is-mobile .content-scroll {
  transition: background-color 0.3s ease;
  padding-bottom: calc(10px + var(--ac-safe-area-bottom));
}

.is-mobile .content-scroll.no-padding {
  padding: 0;
}

.is-mobile .app-content:not(.is-login) {
  padding-bottom: 0;
}
</style>

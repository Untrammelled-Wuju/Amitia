<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="app-shell" :class="{ 'is-mobile': isMobile }">
    <div class="app-body workbench">
      <SideNav v-if="!isMobile" :username="authUsername" :avatar="authAvatar" :wechat-status="health.wechat" :qq-status="health.qq" :model-status="health.model" :theme="resolvedTheme" @toggle-theme="toggleTheme" />

      <div class="app-main workspace">
        <main class="app-content workspace-surface" :class="{ 'is-login': isLoginPage, 'workspace-surface--chat': isChatPage }">
          <header v-if="isMobile && !isLoginPage" class="mobile-header">
            <span class="mobile-title">{{ pageTitle }}</span>
            <span class="mobile-status">
              <span class="dot" :class="modelClass"></span>
              {{ currentCharName || "未配置" }}
            </span>
          </header>

          <div
            class="content-scroll"
            :class="{ 'no-padding': isChatPage || isLoginPage }"
          >
            <slot />
          </div>
        </main>
      </div>
    </div>

    <MobileNav v-if="isMobile && !isLoginPage" />
    <div id="amitia-overlay-root" aria-live="polite"></div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, provide } from "vue";
import { useRouter } from "vue-router";
import SideNav from "./SideNav.vue";
import MobileNav from "./MobileNav.vue";
import { useTheme } from "../composables/useTheme";
import { useUIHostSSE } from "../composables/useUIHostSSE";
import { isNavigationAllowed } from "../navigation/nav-whitelist";
import {
  apiClient,
  getToken,
  removeToken,
  isLoggedIn,
} from "../composables/useApi";
import { getPageTitle } from "@/navigation/app-nav";
import { useAppStore } from "@/stores/app";
import { useExtensionUIStore } from "@/stores/extensionUI";

const router = useRouter();
const { connect: connectUIHost, disconnect: disconnectUIHost } = useUIHostSSE();
let electronNavCleanup: (() => void) | null = null;
const {
  state: theme,
  resolvedMode: resolvedTheme,
  toggleLightDark: toggleTheme,
} = useTheme();

const windowWidth = ref(window.innerWidth);
const isMobile = computed(() => windowWidth.value < 768);
const isLoginPage = computed(() => router.currentRoute.value.path === "/login");

const isChatPage = computed(() => router.currentRoute.value.path === "/chat");
const pageTitle = computed(() => getPageTitle(router.currentRoute.value.path));

const health = ref({
  appStatus: "running",
  deployMode: "desktop-local",
  database: "ok",
  model: "not_configured",
  wechat: "disconnected",
  qq: "disconnected",
  web: "enabled",
});

const currentCharName = ref("");

const authUsername = ref("");
const appStore = useAppStore();
const authAvatar = computed(() => appStore.avatar);
const extensionUIStore = useExtensionUIStore();
const extensionRuntimeAvailable = ref(false);

const modelClass = computed(() =>
  health.value.model === "configured" ? "status-on" : "status-off",
);

provide("theme", theme);
async function refreshAll() {
  await fetchHealth();
  await fetchQQStatus();
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

async function fetchQQStatus() {
  try {
    const res = await apiClient.get("/api/qq/status");
    const data = res.data?.data || res.data;
    if (data) {
      health.value.qq =
        data.qqOnline || data.status === "online"
          ? "connected"
          : "disconnected";
    }
  } catch {
    health.value.qq = "disconnected";
  }
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

async function fetchUserInfo() {
  if (!isLoggedIn()) return;
  try {
    const res = await apiClient.get("/api/auth/me");
    const user = res.data?.data || res.data;
    if (user?.username) {
      authUsername.value = user.username;
    }
  } catch {
    removeToken();
  }
}

onMounted(() => {
  window.addEventListener("resize", () => {
    windowWidth.value = window.innerWidth;
  });
  fetchHealth();
  fetchQQStatus();
  if (isLoggedIn()) {
    fetchActiveCharacter();
    fetchUserInfo();
    const token = getToken();
    if (token && window.amitiaDesktop?.setAuthToken) {
      void window.amitiaDesktop.setAuthToken(token);
    }
    connectUIHost();
    extensionUIStore.refreshSnapshot(true).then(() => {
      extensionRuntimeAvailable.value = true;
    });
  }

  if (window.amitiaDesktop?.onUINavigate) {
    electronNavCleanup = window.amitiaDesktop.onUINavigate((target: string) => {
      if (target && isNavigationAllowed(target)) {
        router.push(target).catch(() => {});
      }
    });
  }

  const interval = setInterval(() => {
    fetchHealth();
    fetchQQStatus();
  }, 30000);
  onUnmounted(() => {
    clearInterval(interval);
    disconnectUIHost();
    extensionUIStore.invalidateSnapshot();
    if (electronNavCleanup) {
      electronNavCleanup();
      electronNavCleanup = null;
    }
  });
});
</script>

<style scoped>
.app-shell {
  transition: background-color 0.3s ease;
  display: flex;
  flex-direction: column;
  height: 100vh;
  background: var(--workbench-bg);
}

.app-body {
  display: flex;
  flex: 1;
  overflow: hidden;
}

.app-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--workbench-bg);
}

.app-content {
  flex: 1;
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
  overflow-y: auto;
  overflow-x: hidden;
  padding: 20px 24px;
  background: transparent;
}

.content-scroll.no-padding {
  padding: 0;
}

.workspace-surface--chat .content-scroll {
  display: flex;
  min-height: 0;
}

.workspace-surface--chat .content-scroll > * {
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

.is-mobile .content-scroll {
  transition: background-color 0.3s ease;
  padding: 10px 12px;
  padding-bottom: calc(10px + var(--ac-safe-area-bottom));
}

.is-mobile .content-scroll.no-padding {
  padding: 0;
}

.is-mobile .app-content:not(.is-login) {
  padding-bottom: 0;
}
</style>

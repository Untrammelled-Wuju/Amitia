<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="app-shell" :class="{ 'is-mobile': isMobile }">
    <!-- Desktop: top status bar -->
    <StatusBar
      v-if="!isMobile"
      :deploy-mode="health.deployMode"
      :wechat-status="health.wechat"
      :qq-status="health.qq"
      :model-status="health.model"
      :character-name="currentCharName"
      :theme="theme.preset"`n      
      :username="authUsername"
      @toggle-theme="toggleTheme"
      @logout="handleLogout"
    />

    <div class="app-body">
      <!-- Desktop: side nav -->
      <SideNav v-if="!isMobile" />

      <!-- Main content -->
      <main class="app-content" :class="{ 'is-login': isLoginPage }">
        <!-- Mobile: compact status bar -->
        <header v-if="isMobile && !isLoginPage" class="mobile-header">
          <span class="mobile-title">{{ pageTitle }}</span>
          <span class="mobile-status">
            <span class="dot" :class="modelClass"></span>
            {{ currentCharName || "未配置" }}
          </span>
        </header>

        <div class="content-scroll" :class="{ 'no-padding': isChatPage || isLoginPage }">
          <slot />
        </div>
      </main>
    </div>

    <!-- Mobile: bottom tab nav -->
    <MobileNav v-if="isMobile && !isLoginPage" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, provide } from "vue"
import { useRouter } from "vue-router"
import StatusBar from "./StatusBar.vue"
import SideNav from "./SideNav.vue"
import MobileNav from "./MobileNav.vue"
import { useTheme } from "../composables/useTheme"
import { apiClient, getToken, removeToken, isLoggedIn } from "../composables/useApi"

const router = useRouter()
const { state: theme, resolvedMode: resolvedTheme, toggleLightDark: toggleTheme } = useTheme()

const windowWidth = ref(window.innerWidth)
const isMobile = computed(() => windowWidth.value < 768)
const isLoginPage = computed(() => router.currentRoute.value.path === "/login")

const isChatPage = computed(() => router.currentRoute.value.path === "/chat")
const pageTitle = computed(() => {
  const titles: Record<string, string> = {
    "/chat": "聊天", "/character": "角色管理",
    "/logs": "聊天记录", "/settings": "设置", "/dashboard": "概览",
    "/model": "模型配置", "/reminders": "日程提醒", "/import": "导入", "/safety": "安全设置",
    "/qq": "QQ连接", "/wechat": "微信", "/login": "登录",
  }
  return titles[router.currentRoute.value.path] || "AI-Amitia"
})

// Health state
const health = ref({
  appStatus: "running",
  deployMode: "desktop-local",
  database: "ok",
  model: "not_configured",
  wechat: "disconnected",
  qq: "disconnected",
  web: "enabled",
})

// Character name
const currentCharName = ref("")

// Auth
const authUsername = ref("")

// Model status computed
const modelClass = computed(() =>
  health.value.model === "configured" ? "status-on" : "status-off"
)

// Provide theme
provide("theme", theme)
async function refreshAll() {
  await fetchHealth()
  await fetchQQStatus()
  await fetchActiveCharacter()
}
provide("refreshHealth", refreshAll)
provide("resolvedTheme", resolvedTheme)
  provide("currentCharName", currentCharName)

// Fetch health
async function fetchHealth() {
  try {
    const res = await apiClient.get("/api/health")
    if (res.data?.model) {
      health.value = { ...health.value, ...res.data }
    }
  } catch {
    // Core not available yet
  }
}


async function fetchQQStatus() {
  try {
    const res = await apiClient.get("/api/qq/status")
    const data = res.data?.data || res.data
    if (data) {
      health.value.qq = data.qqOnline || data.status === "online" ? "connected" : "disconnected"
    }
  } catch {
    health.value.qq = "disconnected"
  }
}

// Fetch active character
async function fetchActiveCharacter() {
  const cached = localStorage.getItem("uai-default-char")
  if (cached) {
    try {
      const dc = JSON.parse(cached)
      if (dc.name) currentCharName.value = dc.name
    } catch {}
  }
  try {
    const res = await apiClient.get("/api/characters")
    const chars = res.data?.data || res.data
    if (Array.isArray(chars)) {
      const defaultChar = chars.find((c: any) => c.isDefault)
      const active = chars.find((c: any) => c.isActive)
      const first = chars.find((c: any) => c.status !== "disabled")
      const selected = defaultChar || active || first
      currentCharName.value = (selected || {}).name || ""
      if (selected && selected.isDefault) {
        localStorage.setItem("uai-default-char", JSON.stringify({
          id: selected.id, name: selected.name,
          identity: selected.identity || selected.personality || "",
          updatedAt: Date.now(),
        }))
      }
    }
  } catch {
    // Ignore
  }
}

// Fetch user info
async function fetchUserInfo() {
  if (!isLoggedIn()) return
  try {
    const res = await apiClient.get("/api/auth/me")
    const user = res.data?.data || res.data
    if (user?.username) {
      authUsername.value = user.username
    }
  } catch {
    removeToken()
  }
}

async function handleLogout() {
  try {
    await apiClient.post("/api/auth/logout")
  } catch { /* ignore */ }
  removeToken()
  authUsername.value = ""
  router.push("/login")
}

onMounted(() => {
  window.addEventListener("resize", () => {
    windowWidth.value = window.innerWidth
  })
  fetchHealth()
  fetchQQStatus()
  if (isLoggedIn()) {
    fetchActiveCharacter()
    fetchUserInfo()
  }

  // Poll health every 30s
  const interval = setInterval(() => { fetchHealth(); fetchQQStatus() }, 30000)
  onUnmounted(() => clearInterval(interval))
})
</script>

<style scoped>
.app-shell {
  display: flex;
  flex-direction: column;
  height: 100vh;
  background: var(--ac-color-bg);
}

.app-body {
  display: flex;
  flex: 1;
  overflow: hidden;
}

.app-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.app-content.is-login {
  align-items: center;
  justify-content: center;
  background: var(--ac-color-bg);
}

.content-scroll {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 18px 24px;
}

.content-scroll.no-padding {
  padding: 0;
}

/* Mobile header */
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

/* Mobile adjustments */
/* Mobile adjustments */
.is-mobile .content-scroll {
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

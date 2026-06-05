import { createRouter, createWebHistory } from "vue-router"
import { apiClient } from "../ui-index"

const TOKEN_KEY = "ai-companion-token"

function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

function isLoggedIn(): boolean {
  return !!getToken()
}

// Public routes that don't need auth
  const PUBLIC_PATHS = ["/login", "/setup", "/setup-wizard", "/onboarding", "/privacy", "/usage-boundary"]

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/onboarding", name: "onboarding", component: () => import("../views/onboarding/OnboardingView.vue") },
    { path: "/login", name: "login", component: () => import("@/views/login/LoginView.vue") },
    { path: "/setup", name: "setup", component: () => import("../views/setup-admin/SetupAdminView.vue") },
    { path: "/", redirect: "/chat" },
    { path: "/dashboard", name: "dashboard", component: () => import("@/views/dashboard/DashboardView.vue"), meta: { requiresAuth: true } },
    { path: "/chat", name: "chat", component: () => import("@/views/web-chat/WebChatView.vue"), meta: { requiresAuth: true } },
    { path: "/qq", name: "qq", component: () => import("@/views/qq-connect/QqConnectView.vue"), meta: { requiresAuth: true } },
    { path: "/wechat", name: "wechat", component: () => import("@/views/wechat-connect/WechatConnectView.vue"), meta: { requiresAuth: true } },
    { path: "/model", name: "model", component: () => import("@/views/model-config/ModelConfigView.vue"), meta: { requiresAuth: true } },
    { path: "/character", name: "character", component: () => import("../views/character/CharacterView.vue"), meta: { requiresAuth: true } },
    { path: "/character/:id", redirect: (to: any) => `/character/${to.params.id}/life-rules` },
    { path: "/character/:id/life-rules", name: "characterLifeRules", component: () => import("../views/character/CharacterView.vue"), meta: { requiresAuth: true } },
    { path: "/character/:id/memory", name: "characterMemory", component: () => import("../views/character/CharacterView.vue"), meta: { requiresAuth: true } },
    { path: "/character/:id/timeline", name: "characterTimeline", component: () => import("../views/character/CharacterView.vue"), meta: { requiresAuth: true } },
    { path: "/character/:id/proactive", name: "characterProactive", component: () => import("../views/character/CharacterView.vue"), meta: { requiresAuth: true } },
    { path: "/logs", name: "logs", component: () => import("@/views/chat-logs/ChatLogsView.vue"), meta: { requiresAuth: true } },
    { path: "/import", name: "import", component: () => import("@/views/chat-import/ChatImportView.vue"), meta: { requiresAuth: true } },
    { path: "/reminders", name: "reminders", component: () => import("@/views/reminders/Reminders.vue"), meta: { requiresAuth: true } },
    { path: "/safety", name: "safety", component: () => import("@/views/safety-settings/SafetySettingsView.vue"), meta: { requiresAuth: true } },
    { path: "/maintenance", name: "maintenance", component: () => import("@/views/maintenance-diagnostics/MaintenanceDiagnosticsView.vue"), meta: { requiresAuth: true } },
    { path: "/settings", name: "settings", component: () => import("@/views/settings/SettingsView.vue"), meta: { requiresAuth: true } },
    { path: "/long-running", name: "longRunning", component: () => import("@/views/long-running/LongRunningView.vue"), meta: { requiresAuth: true } },
    { path: "/runtime-mode", name: "runtimeMode", component: () => import("@/views/runtime-mode/RuntimeModeView.vue"), meta: { requiresAuth: true } },
    { path: "/storage", name: "storage", component: () => import("@/views/chat-cleanup/ChatCleanupView.vue"), meta: { requiresAuth: true } },
    { path: "/privacy-scan", name: "privacyScan", component: () => import("@/views/privacy-scan/PrivacyScanView.vue"), meta: { requiresAuth: true } },
    { path: "/privacy", name: "privacy", component: () => import("../views/privacy/Privacy.vue") },
    { path: "/usage-boundary", name: "usageBoundary", component: () => import("../views/usage-boundary/UsageBoundary.vue") },
  ],
})

// ---- Navigation Guard ----
router.beforeEach(async (to, _from, next) => {
  const token = getToken()
  const PUBLIC_PATHS = ["/login", "/setup", "/setup-wizard", "/onboarding", "/privacy", "/usage-boundary"]
  const isPublic = PUBLIC_PATHS.includes(to.path)

  // Step 1: If going to a public page, allow
  if (isPublic) {
    // If already logged in and going to login/setup/setup-wizard, redirect to chat
    if (token && (to.path === "/login" || to.path === "/setup" || to.path === "/setup-wizard")) {
      return next("/chat")
    }
    return next()
  }

  // Step 2: If route doesn't require auth, allow
  if (!to.meta?.requiresAuth) {
    return next()
  }

  // Step 3: If no token, determine where to redirect
  if (!token) {
    
    // Check if setup wizard is needed first
    try {
      const setupRes = await apiClient.get("/api/setup/status")
      const setupData = setupRes.data?.data || setupRes.data
      if (!setupData?.completed) {
        return next("/setup-wizard")
      }
    } catch {
      // Core not available, continue
    }    // Check if onboarding is needed first
    try {
      const res = await apiClient.get("/api/onboarding/status")
      const onboardingData = res.data?.data || res.data
      if (!onboardingData?.completed) {
        return next("/onboarding")
      }
    } catch {
      // Core not available, proceed
    }

    // Check if admin setup is needed
    try {
      const res = await apiClient.get("/api/auth/status")
      const authData = res.data?.data || res.data
      if (!authData?.hasAdmin) {
        return next("/setup")
      }
    } catch {
      // Core not available
    }

    return next("/login")
  }

  // Step 4: Has token, validate it
  try {
    const res = await apiClient.get("/api/auth/me")
    const userData = res.data?.data || res.data
    if (!userData?.id) {
      // Token invalid
      localStorage.removeItem(TOKEN_KEY)
      return next("/login")
    }
  } catch {
    // Token validation failed (e.g., 401)
    localStorage.removeItem(TOKEN_KEY)
    return next("/login")
  }

  next()
})

export default router

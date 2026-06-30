// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { createRouter, createWebHistory } from "vue-router"
import { apiClient } from "../ui-index"

const TOKEN_KEY = "ai-companion-token"

function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

function isLoggedIn(): boolean {
  return !!getToken()
}

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
    {
      path: "/model",
      component: () => import("@/views/model-config/ModelConfigView.vue"),
      meta: { requiresAuth: true },
      redirect: "/model/llm",
      children: [
        { path: "llm", name: "modelLlm", component: () => import("@/views/model-config/ModelConfigLlmView.vue"), meta: { requiresAuth: true } },
        { path: "voice", name: "modelVoice", component: () => import("@/views/model-config/VoiceModelConfigView.vue"), meta: { requiresAuth: true } },
        { path: "embedding", name: "modelEmbedding", component: () => import("@/views/model-config/VectorModelConfigView.vue") },
        { path: "vision", name: "modelVision", component: () => import("@/views/model-config/VisionModelConfigView.vue"), meta: { requiresAuth: true } },
      ],
    },
    { path: "/character", name: "character", component: () => import("../views/character/CharacterView.vue"), meta: { requiresAuth: true } },
    { path: "/graph", name: "graph", component: () => import("@/views/graph/GraphView.vue"), meta: { requiresAuth: true } },
    { path: "/character/:id", redirect: (to: any) => `/character/${to.params.id}/life-rules` },
    { path: "/character/:id/life-rules", name: "characterLifeRules", component: () => import("../views/character/CharacterView.vue"), meta: { requiresAuth: true } },
    { path: "/character/:id/voice", name: "characterVoice", component: () => import("../views/character/CharacterView.vue"), meta: { requiresAuth: true } },
    { path: "/character/:id/memory", name: "characterMemory", component: () => import("../views/character/CharacterView.vue"), meta: { requiresAuth: true } },
    { path: "/character/:id/timeline", name: "characterTimeline", component: () => import("../views/character/CharacterView.vue"), meta: { requiresAuth: true } },
    { path: "/character/:id/proactive", name: "characterProactive", component: () => import("../views/character/CharacterView.vue"), meta: { requiresAuth: true } },
    { path: "/character/:id/debug", name: "characterDebug", component: () => import("../views/character/CharacterView.vue"), meta: { requiresAuth: true } },
    { path: "/logs", name: "logs", component: () => import("@/views/chat-logs/ChatLogsView.vue"), meta: { requiresAuth: true } },
    { path: "/import", name: "import", component: () => import("@/views/chat-import/ChatImportView.vue"), meta: { requiresAuth: true } },
    { path: "/reminders", name: "reminders", component: () => import("@/views/reminders/Reminders.vue"), meta: { requiresAuth: true } },
    { path: "/safety", name: "safety", component: () => import("@/views/safety-settings/SafetySettingsView.vue"), meta: { requiresAuth: true } },
    { path: "/maintenance", name: "maintenance", component: () => import("@/views/maintenance-diagnostics/MaintenanceDiagnosticsView.vue"), meta: { requiresAuth: true } },
    { path: "/settings", name: "settings", component: () => import("@/views/settings/SettingsView.vue"), meta: { requiresAuth: true } },
    { path: "/long-running", name: "longRunning", component: () => import("@/views/long-running/LongRunningView.vue"), meta: { requiresAuth: true } },
    { path: "/runtime-mode", name: "runtimeMode", component: () => import("@/views/runtime-mode/RuntimeModeView.vue"), meta: { requiresAuth: true } },
    { path: "/storage", name: "storage", component: () => import("@/views/chat-cleanup/ChatCleanupView.vue"), meta: { requiresAuth: true } },
    { path: "/profiles", name: "profiles", component: () => import("@/views/profile/ProfileView.vue"), meta: { requiresAuth: true } },
    { path: "/episodic", name: "episodic", component: () => import("@/views/episodic/EpisodicView.vue"), meta: { requiresAuth: true } },
    { path: "/world-book", name: "worldBook", component: () => import("@/views/world-book/WorldBookView.vue"), meta: { requiresAuth: true } },
    { path: "/memory-manager", name: "memoryManager", component: () => import("@/views/memory-manager/MemoryManagerView.vue"), meta: { requiresAuth: true } },
    { path: "/memory-timeline", name: "memoryTimeline", component: () => import("@/views/memory-timeline/MemoryTimeline.vue"), meta: { requiresAuth: true } },
    { path: "/memory", redirect: "/memory-manager" },
    { path: "/privacy-scan", name: "privacyScan", component: () => import("@/views/privacy-scan/PrivacyScanView.vue"), meta: { requiresAuth: true } },
    { path: "/privacy", name: "privacy", component: () => import("../views/privacy/Privacy.vue") },
    { path: "/usage-boundary", name: "usageBoundary", component: () => import("../views/usage-boundary/UsageBoundary.vue") },
  ],
})

router.beforeEach(async (to, _from, next) => {
  const token = getToken()
  const PUBLIC_PATHS = ["/login", "/setup", "/setup-wizard", "/onboarding", "/privacy", "/usage-boundary"]
  const isPublic = PUBLIC_PATHS.includes(to.path)

  if (isPublic) {
    if (token && (to.path === "/login" || to.path === "/setup" || to.path === "/setup-wizard")) {
      return next("/chat")
    }
    return next()
  }

  if (!to.meta?.requiresAuth) {
    return next()
  }

  if (!token) {
    try {
      const setupRes = await apiClient.get("/api/setup/status")
      const setupData = setupRes.data?.data || setupRes.data
      if (!setupData?.completed) {
        return next("/setup-wizard")
      }
    } catch {}

    try {
      const res = await apiClient.get("/api/onboarding/status")
      const onboardingData = res.data?.data || res.data
      if (!onboardingData?.completed) {
        return next("/onboarding")
      }
    } catch {}

    try {
      const res = await apiClient.get("/api/auth/status")
      const authData = res.data?.data || res.data
      if (!authData?.hasAdmin) {
        return next("/setup")
      }
    } catch {}

    return next("/login")
  }

  try {
    const res = await apiClient.get("/api/auth/me")
    const userData = res.data?.data || res.data
    if (!userData?.id) {
      localStorage.removeItem(TOKEN_KEY)
      return next("/login")
    }
  } catch {
    localStorage.removeItem(TOKEN_KEY)
    return next("/login")
  }

  next()
})

export default router

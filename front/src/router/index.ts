// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
/**
 * Deprecated: Legacy extension architecture.
 * Do not add new static routes or navigation entries for the legacy
 * extension center. Retained only for compatibility, maintenance,
 * testing, and migration to Extension Kernel.
 */
import { createRouter, createWebHashHistory, createWebHistory } from "vue-router"
import { apiClient } from "../ui-index"
import { shouldUseHashRouting } from "../runtime/runtime-capabilities"

const TOKEN_KEY = "ai-companion-token"

function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

function isLoggedIn(): boolean {
  return !!getToken()
}

const router = createRouter({
  history: shouldUseHashRouting() ? createWebHashHistory() : createWebHistory(),
  routes: [
    { path: "/onboarding", name: "onboarding", component: () => import("../views/onboarding/OnboardingView.vue") },{ path: "/login", name: "login", component: () => import("@/views/login/LoginView.vue") },
    { path: "/setup", name: "setup", component: () => import("../views/setup-admin/SetupAdminView.vue") },
    { path: "/", redirect: "/chat" },
    { path: "/dashboard", redirect: "/dashboard/data" },
      { path: "/dashboard/run", name: "dashboardRun", component: () => import("@/views/dashboard/RunView.vue"), meta: { requiresAuth: true } },
      { path: "/dashboard/data", name: "dashboardData", component: () => import("@/views/dashboard/DataView.vue"), meta: { requiresAuth: true } },
    { path: "/chat", name: "chat", component: () => import("@/views/web-chat/WebChatView.vue"), meta: { requiresAuth: true } },
    /**
     * Deprecated: Legacy extension architecture.
     * Do not add new static routes or navigation entries for the legacy
     * extension center. Retained only for compatibility, maintenance,
     * testing, and migration to Extension Kernel.
     */
    { path: "/extensions", name: "extensionCenter", component: () => import("@/views/extensions/ExtensionCenterView.vue"), meta: { requiresAuth: true } },
    { path: "/kernel", name: "kernelCenter", component: () => import("@/views/kernel/KernelCenterView.vue"), meta: { requiresAuth: true } },
    { path: "/kernel/wasm", name: "wasmRuntime", component: () => import("@/views/kernel/WasmRuntimeDetailView.vue"), meta: { requiresAuth: true } },
    { path: "/kernel/hooks", name: "hookCenter", component: () => import("@/views/kernel/HookCenterView.vue"), meta: { requiresAuth: true } },
    { path: "/kernel/trusted-services", name: "trustedServices", component: () => import("@/views/kernel/TrustedServiceView.vue"), meta: { requiresAuth: true } },
    { path: "/kernel/tasks", name: "taskCenter", component: () => import("@/views/kernel/tasks/TaskCenterView.vue"), meta: { requiresAuth: true } },
    { path: "/kernel/events", name: "eventCenter", component: () => import("@/views/kernel/EventCenterView.vue"), meta: { requiresAuth: true } },
    { path: "/kernel/schedules", name: "scheduleCenter", component: () => import("@/views/kernel/ScheduleCenterView.vue"), meta: { requiresAuth: true } },
    { path: "/kernel/desktop", name: "desktopCenter", component: () => import("@/views/kernel/DesktopCenterView.vue"), meta: { requiresAuth: true } },
    { path: "/kernel/updates", name: "updateCenter", component: () => import("@/views/kernel/UpdateCenterView.vue"), meta: { requiresAuth: true } },
    { path: "/kernel/dev-console", name: "developerConsole", component: () => import("@/views/kernel/DeveloperConsoleView.vue"), meta: { requiresAuth: true } },
    { path: "/kernel/migrations", name: "migrationCenter", component: () => import("@/views/kernel/MigrationCenterView.vue"), meta: { requiresAuth: true } },
    { path: "/kernel/dev-mode", name: "devMode", component: () => import("@/views/kernel/DevModeView.vue"), meta: { requiresAuth: true } },
    { path: "/extension/page/:pageId", name: "extensionPage", component: () => import("@/components/extension/ExtensionPageHost.vue"), meta: { requiresAuth: true } },
    { path: "/extensions/mcp", name: "extensionMCP", component: () => import("@/views/mcp/MCPServerView.vue"), meta: { requiresAuth: true } },
    { path: "/extensions/packages", name: "extensionPackages", component: () => import("@/views/extensions/packages/PackageManagerView.vue"), meta: { requiresAuth: true } },
    { path: "/extensions/skills", name: "extensionSkills", component: () => import("@/views/extensions/SkillListView.vue"), meta: { requiresAuth: true } },
    { path: "/extensions/skills/:id", name: "extensionSkillDetail", component: () => import("@/views/extensions/SkillDetailView.vue"), meta: { requiresAuth: true } },
    { path: "/extensions/agent-skills", name: "extensionAgentSkills", component: () => import("@/views/extensions/agent-skills/AgentSkillListView.vue"), meta: { requiresAuth: true } },
    { path: "/extensions/plugins", name: "extensionPlugins", component: () => import("@/views/extensions/PluginListView.vue"), meta: { requiresAuth: true } },
    { path: "/extensions/plugins/:id", name: "extensionPluginDetail", component: () => import("@/views/extensions/PluginDetailView.vue"), meta: { requiresAuth: true } },
    { path: "/creative-workshop/skills", alias: "/extensions/workshop", name: "extensionWorkshop", component: () => import("@/views/extensions/workshop/WorkshopListView.vue"), meta: { requiresAuth: true } },
    { path: "/creative-workshop/skills/:id", alias: "/extensions/workshop/:id", name: "extensionWorkshopSession", component: () => import("@/views/extensions/workshop/WorkshopSessionView.vue"), meta: { requiresAuth: true } },
    { path: "/extensions/runs", name: "extensionRuns", component: () => import("@/views/extensions/RunHistoryView.vue"), meta: { requiresAuth: true } },
    { path: "/creative-workshop", name: "creativeWorkshop", component: () => import("@/views/creative-workshop/CreativeWorkshopView.vue"), meta: { requiresAuth: true } },

    { path: "/creative-workshop/pet", name: "creativeWorkshopPet", component: () => import("@/views/creative-workshop/PetHubView.vue"), meta: { requiresAuth: true } },
    { path: "/creative-workshop/pet/create", name: "creativeWorkshopPetCreate", component: () => import("@/views/creative-workshop/PetCreationView.vue"), meta: { requiresAuth: true } },
    { path: "/creative-workshop/pet/tasks", name: "creativeWorkshopPetTasks", component: () => import("@/views/creative-workshop/PetTaskListView.vue"), meta: { requiresAuth: true } },
    { path: "/creative-workshop/pet/processing/:processingTaskId", name: "creativeWorkshopPetProcessing", component: () => import("@/views/creative-workshop/PetProcessingReviewView.vue"), meta: { requiresAuth: true } },
    { path: "/creative-workshop/pet/processing/:processingTaskId/actions/:actionKey/editor", name: "creativeWorkshopActionEditor", component: () => import("@/views/creative-workshop/ActionEditorView.vue"), meta: { requiresAuth: true } },
    { path: "/creative-workshop/pet/installations", name: "pet-installations", component: () => import("@/views/creative-workshop/PetInstallationsView.vue"), meta: { requiresAuth: true } },
    { path: "/qq", name: "qq", component: () => import("@/views/qq-connect/QqConnectView.vue"), meta: { requiresAuth: true } },
    { path: "/wechat", name: "wechat", component: () => import("@/views/wechat-connect/WechatConnectView.vue"), meta: { requiresAuth: true } },
    {
      path: "/settings",
      component: () => import("@/views/settings/SettingsView.vue"),
      meta: { requiresAuth: true },
      redirect: "/settings/runtime",
      children: [
        { path: "deployment", name: "settingsDeployment", component: () => import("@/views/settings/DeploymentPanel.vue"), meta: { requiresAuth: true } },
        { path: "runtime", name: "settingsRuntime", component: () => import("@/views/settings/RuntimePanel.vue"), meta: { requiresAuth: true } },
        { path: "ai-config", name: "settingsAiConfig", component: () => import("@/views/settings/AiConfigPanel.vue"), meta: { requiresAuth: true } },
        { path: "system", name: "settingsSystem", component: () => import("@/views/settings/SystemSettingsPanel.vue"), meta: { requiresAuth: true } },
        { path: "temporal", name: "settingsTemporal", component: () => import("@/views/settings/TemporalSettingsView.vue"), meta: { requiresAuth: true } },
        {
          path: "model",
          component: () => import("@/views/model-config/ModelConfigView.vue"),
          meta: { requiresAuth: true },
          redirect: "/settings/model/llm",
          children: [
            { path: "llm", name: "settingsModelLlm", component: () => import("@/views/model-config/ModelConfigLlmView.vue"), meta: { requiresAuth: true } },
            { path: "voice", name: "settingsModelVoice", component: () => import("@/views/model-config/VoiceModelConfigView.vue"), meta: { requiresAuth: true } },
            { path: "embedding", name: "settingsModelEmbedding", component: () => import("@/views/model-config/VectorModelConfigView.vue") },
            { path: "vision", name: "settingsModelVision", component: () => import("@/views/model-config/VisionModelConfigView.vue"), meta: { requiresAuth: true } },
            { path: "imagegen", name: "settingsModelImageGen", component: () => import("@/views/model-config/ImageGenModelConfigView.vue"), meta: { requiresAuth: true } },
          ],
        },
        { path: "safety", name: "settingsSafety", component: () => import("@/views/safety-settings/SafetySettingsView.vue"), meta: { requiresAuth: true } },
        { path: "maintenance", name: "settingsMaintenance", component: () => import("@/views/maintenance-diagnostics/MaintenanceDiagnosticsView.vue"), meta: { requiresAuth: true } },
        { path: "about", name: "settingsAbout", component: () => import("@/views/settings/AboutPanel.vue"), meta: { requiresAuth: true } },
      ],
    },
    { path: "/model", redirect: "/settings/model" },
    { path: "/model/:path*", redirect: (to: any) => "/settings/model/" + (to.params.path || "") },
    { path: "/safety", redirect: "/settings/safety" },
    { path: "/maintenance", redirect: "/settings/maintenance" },
    { path: "/settings/emotes", redirect: "/emotes" },
    { path: "/emotes", name: "emotes", component: () => import("@/views/emotes/EmoteManagerView.vue"), meta: { requiresAuth: true } },
    { path: "/character", name: "character", component: () => import("../views/character/CharacterView.vue"), meta: { requiresAuth: true } },
    { path: "/graph", name: "graph", component: () => import("@/views/graph/GraphView.vue"), meta: { requiresAuth: true } },
    { path: "/character/:id", redirect: (to: any) => `/character/${to.params.id}/life-rules` },
    { path: "/character/:id/life-rules", name: "characterLifeRules", component: () => import("../views/character/CharacterView.vue"), meta: { requiresAuth: true } },
    { path: "/character/:id/voice", name: "characterVoice", component: () => import("../views/character/CharacterView.vue"), meta: { requiresAuth: true } },
    { path: "/character/:id/memory", name: "characterMemory", component: () => import("../views/character/CharacterView.vue"), meta: { requiresAuth: true } },
    { path: "/character/:id/timeline", name: "characterTimeline", component: () => import("../views/character/CharacterView.vue"), meta: { requiresAuth: true } },
    { path: "/character/:id/proactive", name: "characterProactive", component: () => import("../views/character/CharacterView.vue"), meta: { requiresAuth: true } },
    { path: "/character/:id/debug", name: "characterDebug", component: () => import("../views/character/CharacterView.vue"), meta: { requiresAuth: true } },
    { path: "/character/:id/psyche", name: "characterPsyche", component: () => import("../views/character/CharacterView.vue"), meta: { requiresAuth: true } },
    { path: "/logs", name: "logs", component: () => import("@/views/chat-logs/ChatLogsView.vue"), meta: { requiresAuth: true } },
    { path: "/import", name: "import", component: () => import("@/views/chat-import/ChatImportView.vue"), meta: { requiresAuth: true } },
    { path: "/reminders", name: "reminders", component: () => import("@/views/reminders/Reminders.vue"), meta: { requiresAuth: true } },
    { path: "/settings/theme", name: "settingsTheme", component: () => import("@/views/settings/ThemeSettingsView.vue"), meta: { requiresAuth: true } },
    { path: "/runtime-mode", name: "runtimeMode", component: () => import("@/views/runtime-mode/RuntimeModeView.vue"), meta: { requiresAuth: true } },
    { path: "/storage", name: "storage", component: () => import("@/views/chat-cleanup/ChatCleanupView.vue"), meta: { requiresAuth: true } },
    { path: "/profiles", name: "profiles", component: () => import("@/views/profile/ProfileView.vue"), meta: { requiresAuth: true } },
    { path: "/user-settings", name: "userSettings", component: () => import("@/views/user-settings/UserSettingsView.vue"), meta: { requiresAuth: true } },
    { path: "/episodic", name: "episodic", component: () => import("@/views/episodic/EpisodicView.vue"), meta: { requiresAuth: true } },
    { path: "/world-book", name: "worldBook", component: () => import("@/views/world-book/WorldBookView.vue"), meta: { requiresAuth: true } },
    { path: '/decision-viz', name: 'decisionViz', component: () => import('@/views/decision-viz/DecisionVizView.vue'), meta: { requiresAuth: true } },
    { path: "/memory-manager", name: "memoryManager", component: () => import("@/views/memory-manager/MemoryManagerView.vue"), meta: { requiresAuth: true } },
    { path: "/memory-timeline", name: "memoryTimeline", component: () => import("@/views/memory-timeline/MemoryTimeline.vue"), meta: { requiresAuth: true } },
    { path: "/memory", redirect: "/memory-manager" },
    { path: "/privacy-scan", name: "privacyScan", component: () => import("@/views/privacy-scan/PrivacyScanView.vue"), meta: { requiresAuth: true } },
    { path: "/privacy", name: "privacy", component: () => import("../views/privacy/Privacy.vue") },
    { path: "/usage-boundary", name: "usageBoundary", component: () => import("../views/usage-boundary/UsageBoundary.vue") },
    { path: "/404", name: "notFound", component: () => import("@/views/NotFoundView.vue") },
    { path: "/:pathMatch(.*)*", name: "catchAll", component: () => import("@/views/NotFoundView.vue") },
  ],
})

router.beforeEach(async (to, _from, next) => {
  const token = getToken()
  const PUBLIC_PATHS = ["/login", "/setup", "/onboarding", "/privacy", "/usage-boundary"]
  const isPublic = PUBLIC_PATHS.includes(to.path)

  if (isPublic) {
    if (token && (to.path === "/login" || to.path === "/setup")) {
      return next("/chat")
    }
    if (to.path === "/onboarding") {
      try {
        const res = await apiClient.get("/api/onboarding/status")
        const data = res.data?.data || res.data
        if (data?.completed) {
          return next(token ? "/chat" : "/login")
        }
      } catch {}
    }
    return next()
  }

  if (!to.meta?.requiresAuth) {
    return next()
  }

  if (!token) {
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


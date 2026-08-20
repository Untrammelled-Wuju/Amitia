// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
/**
 * Deprecated: Legacy extension architecture.
 * Do not add new static routes or navigation entries for the legacy
 * extension center. Retained only for compatibility, maintenance,
 * testing, and migration to Extension Kernel.
 */
import { createRouter, createWebHashHistory, createWebHistory } from "vue-router"
import { getAccessToken } from "../stores/refresh-coordinator"
import { apiClient } from "../ui-index"
import { shouldUseHashRouting } from "../runtime/runtime-capabilities"
import { builtinBusinessRoutes } from "./builtinRoutes"

function isLoggedIn(): boolean {
  return !!getAccessToken()
}

function getToken(): string | null {
  return getAccessToken()
}


const router = createRouter({
  history: shouldUseHashRouting() ? createWebHashHistory() : createWebHistory(),
  routes: [
    { path: "/onboarding", name: "onboarding", component: () => import("../views/onboarding/OnboardingView.vue") },
    { path: "/login", name: "login", component: () => import("@/views/login/LoginView.vue") },
    { path: "/privacy", name: "privacy", component: () => import("../views/privacy/Privacy.vue") },
    { path: "/usage-boundary", name: "usageBoundary", component: () => import("../views/usage-boundary/UsageBoundary.vue") },
    {
      path: "/settings/ui-providers",
      name: "settingsUIProviders",
      component: () => import("@/views/settings/UIProviderSettingsView.vue"),
      meta: { requiresAuth: true, uiProviderControl: true, recoveryRoute: true },
    },
    { path: "/", redirect: "/chat", meta: { requiresAuth: true } },
    ...builtinBusinessRoutes,
    { path: "/404", name: "notFound", component: () => import("@/views/NotFoundView.vue") },
    { path: "/:pathMatch(.*)*", name: "catchAll", component: () => import("@/views/NotFoundView.vue") },
  ],
});

router.beforeEach(async (to, _from, next) => {
  const token = getToken()
  const PUBLIC_PATHS = ["/login", "/onboarding", "/privacy", "/usage-boundary"]
  const isPublic = PUBLIC_PATHS.includes(to.path)
  console.log(`[GUARD-RUN] path=${to.path} isPublic=${isPublic} token=${token ? "Y" : "N"}`)

  if (isPublic) {
    if (to.path === "/login") {
      try {
        const res = await apiClient.get("/api/public/onboarding/status")
        const data = res.data?.data || res.data
        if (!data?.completed) {
          return next("/onboarding")
        }
      } catch {}
      if (token) {
        return next("/chat")
      }
    }
    if (to.path === "/onboarding") {
      try {
        const res = await apiClient.get("/api/public/onboarding/status")
        const data = res.data?.data || res.data
        if (data?.completed) {
          return next(token ? "/chat" : "/login")
        }
      } catch {}
    }
    return next()
  }

  if (to.meta?.requiresAuth) {
    console.log(`[GUARD] checking onboarding for ${to.path}`)
    try {
      const res = await apiClient.get("/api/public/onboarding/status")
      const onboardingData = res.data?.data || res.data
      console.log(`[GUARD] onboarding completed=${onboardingData?.completed}`)
      if (!onboardingData?.completed) {
        console.log(`[GUARD] redirecting ${to.path} -> /onboarding`)
        return next("/onboarding")
      }
      console.log(`[GUARD] allowing ${to.path}, onboarding done`)
    } catch (e) {
      console.log("[GUARD] onboarding check error:", e)
    }
  }

  if (!to.meta?.requiresAuth) {
    return next()
  }

  if (!token) {
    return next("/login")
  }

  try {
    const res = await apiClient.get("/api/auth/me")
    const userData = res.data?.data || res.data
    if (!(userData?.id || userData?.userId)) {
      return next("/login")
    }
  } catch {
    return next("/login")
  }

  next()
})

export default router

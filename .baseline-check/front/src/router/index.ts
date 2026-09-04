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
import { saveCurrentUser } from "../stores/refresh-coordinator"
import { useSessionStore } from "../stores/session-store"
import { apiClient } from "../ui-index"
import { isRuntimeRouteAvailable, shouldUseHashRouting } from "../runtime/runtime-capabilities"
import { builtinBusinessRoutes } from "./builtinRoutes"

const TOKEN_CACHE_TTL = 5 * 60 * 1000

interface AuthCache {
  onboardingCompleted: boolean | null
  onboardingCheckedAt: number
  userAuthenticated: boolean | null
  userCheckedAt: number
}

const authCache: AuthCache = {
  onboardingCompleted: null,
  onboardingCheckedAt: 0,
  userAuthenticated: null,
  userCheckedAt: 0,
}

let routerInstance: ReturnType<typeof createRouter> | null = null

function getToken(): string | null {
  return getAccessToken()
}

function getUserId(): string | null {
  return useSessionStore().state.value.userId
}

function isUserAuthenticatedCached(): boolean {
  return (
    authCache.userAuthenticated === true &&
    Date.now() - authCache.userCheckedAt < TOKEN_CACHE_TTL
  )
}

function refreshOnboardingStatus(): void {
  apiClient
    .get("/api/public/onboarding/status")
    .then((res) => {
      const data = res.data?.data || res.data
      const completed = !!data?.completed
      authCache.onboardingCompleted = completed
      authCache.onboardingCheckedAt = Date.now()
      if (!completed && routerInstance) {
        const currentPath = window.location.hash.replace(/^#/, "")
        if (currentPath && currentPath !== "/onboarding") {
          routerInstance.push("/onboarding").catch(() => {})
        }
      }
    })
    .catch(() => {
      authCache.onboardingCompleted = null
      authCache.onboardingCheckedAt = 0
    })
}

function refreshAuthStatus(): void {
  apiClient
    .get("/api/auth/me")
    .then((res) => {
      const userData = res.data?.data || res.data
      const valid = !!(userData?.id || userData?.userId)
      if (valid) {
        saveCurrentUser({
          userId: userData?.userId || userData?.id,
          username: userData?.username,
          role: userData?.role,
        })
        const { state, setSession } = useSessionStore()
        setSession({
          ...state.value,
          userId: String(userData?.userId || userData?.id),
          username: userData?.username || null,
          role: userData?.role || null,
        })
      }
      authCache.userAuthenticated = valid
      authCache.userCheckedAt = Date.now()
      if (!valid && routerInstance) {
        const currentPath = window.location.hash.replace(/^#/, "")
        const PUBLIC_PATHS = ["/login", "/onboarding", "/privacy", "/usage-boundary"]
        if (currentPath && !PUBLIC_PATHS.includes(currentPath)) {
          routerInstance.push("/login").catch(() => {})
        }
      }
    })
    .catch(() => {
      authCache.userAuthenticated = null
      authCache.userCheckedAt = 0
    })
}

function createAppRouter() {
  const r = createRouter({
    history: shouldUseHashRouting() ? createWebHashHistory() : createWebHistory(),
    routes: [
      { path: "/onboarding", name: "onboarding", component: () => import("../views/onboarding/OnboardingView.vue") },
      { path: "/login", name: "login", component: () => import("@/views/login/LoginView.vue") },
      { path: "/privacy", name: "privacy", component: () => import("../views/privacy/Privacy.vue") },
      { path: "/usage-boundary", name: "usageBoundary", component: () => import("../views/usage-boundary/UsageBoundary.vue") },
      { path: "/", redirect: "/chat", meta: { requiresAuth: true } },
      ...builtinBusinessRoutes,
      { path: "/404", name: "notFound", component: () => import("@/views/NotFoundView.vue") },
      { path: "/:pathMatch(.*)*", name: "catchAll", component: () => import("@/views/NotFoundView.vue") },
    ],
  })

  routerInstance = r

  r.beforeEach((to, _from, next) => {
    const token = getToken()
    const userId = getUserId()
    const PUBLIC_PATHS = ["/login", "/onboarding", "/privacy", "/usage-boundary"]
    const isPublic = PUBLIC_PATHS.includes(to.path)

    if (!isRuntimeRouteAvailable(to.path)) {
      return next({ path: "/404", query: { reason: "runtime-capability-unavailable" } })
    }

    if (isPublic) {
      if (to.path === "/login") {
        if (token && (userId || isUserAuthenticatedCached())) {
          refreshOnboardingStatus()
          refreshAuthStatus()
          return next("/chat")
        }
        apiClient
          .get("/api/public/onboarding/status")
          .then((res) => {
            const data = res.data?.data || res.data
            authCache.onboardingCompleted = !!data?.completed
            authCache.onboardingCheckedAt = Date.now()
            if (!data?.completed) {
              next("/onboarding")
            } else {
              next()
            }
          })
          .catch(() => next())
        return
      }
      if (to.path === "/onboarding") {
        if (authCache.onboardingCompleted === true && token) {
          return next("/chat")
        }
        apiClient
          .get("/api/public/onboarding/status")
          .then((res) => {
            const data = res.data?.data || res.data
            authCache.onboardingCompleted = !!data?.completed
            authCache.onboardingCheckedAt = Date.now()
            if (data?.completed) {
              next(token ? "/chat" : "/login")
            } else {
              next()
            }
          })
          .catch(() => next())
        return
      }
      next()
      return
    }

    if (!to.meta?.requiresAuth) {
      next()
      return
    }

    if (!token) {
      next("/login")
      return
    }

    const authenticated = userId || isUserAuthenticatedCached()
    const onboardingDone = authCache.onboardingCompleted !== false

    if (!authenticated || !onboardingDone) {
      refreshAuthStatus()
      refreshOnboardingStatus()
      next()
      return
    }

    if (Date.now() - authCache.onboardingCheckedAt > TOKEN_CACHE_TTL) {
      refreshOnboardingStatus()
    }
    if (Date.now() - authCache.userCheckedAt > TOKEN_CACHE_TTL) {
      refreshAuthStatus()
    }

    next()
  })

  return r
}

const router = createAppRouter()

export default router

// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
/**
 * Application router.
 *
 * U-Ai has no product-account login. Cloud browser access has one lightweight
 * instance-password gate; device/runtime authority still uses the existing
 * Desktop Session or DeviceCredential boundary.
 */
import { createRouter, createWebHashHistory, createWebHistory } from "vue-router";
import { apiClient } from "../ui-index";
import { getApiBaseURL, getDeploymentConfig, isCurrentDevicePaired } from "../runtime/runtime-adapter";
import { getWebAccessStatus, type WebAccessStatus } from "../runtime/web-access";
import { isRuntimeRouteAvailable, shouldUseHashRouting } from "../runtime/runtime-capabilities";
import { builtinBusinessRoutes } from "./builtinRoutes";

const ONBOARDING_CACHE_TTL = 30_000;
let onboardingCompleted: boolean | null = null;
let onboardingCheckedAt = 0;

async function readOnboardingCompleted(force = false): Promise<boolean | null> {
  if (!force && onboardingCompleted !== null && Date.now() - onboardingCheckedAt < ONBOARDING_CACHE_TTL) {
    return onboardingCompleted;
  }
  try {
    const res = await apiClient.get("/api/public/onboarding/status");
    const data = res.data?.data || res.data;
    onboardingCompleted = Boolean(data?.completed);
    onboardingCheckedAt = Date.now();
    return onboardingCompleted;
  } catch {
    onboardingCompleted = null;
    onboardingCheckedAt = 0;
    return null;
  }
}

const PUBLIC_PATHS = new Set(["/web-access", "/onboarding", "/privacy", "/usage-boundary"]);

const router = createRouter({
  history: shouldUseHashRouting() ? createWebHashHistory() : createWebHistory(),
  routes: [
    { path: "/web-access", name: "webAccess", component: () => import("../views/WebAccessLoginView.vue") },
    { path: "/onboarding", name: "onboarding", component: () => import("../views/onboarding/OnboardingView.vue") },
    { path: "/privacy", name: "privacy", component: () => import("../views/privacy/Privacy.vue") },
    { path: "/usage-boundary", name: "usageBoundary", component: () => import("../views/usage-boundary/UsageBoundary.vue") },
    { path: "/", redirect: "/chat" },
    ...builtinBusinessRoutes,
    { path: "/404", name: "notFound", component: () => import("@/views/NotFoundView.vue") },
    { path: "/:pathMatch(.*)*", name: "catchAll", component: () => import("@/views/NotFoundView.vue") },
  ],
});

router.beforeEach(async (to) => {
  if (!isRuntimeRouteAvailable(to.path)) {
    return { path: "/404", query: { reason: "runtime-capability-unavailable" } };
  }

  let webAccess: WebAccessStatus | null = null;
  let webDevicePaired: boolean | null = null;
  if (typeof window !== "undefined" && !window.amitiaDesktop) {
    const deployment = await getDeploymentConfig();
    if (deployment.mode === "cloud") {
      try {
        const baseURL = await getApiBaseURL();
        webAccess = await getWebAccessStatus(baseURL);
        if (webAccess.authenticated || !webAccess.configured) {
          webDevicePaired = await isCurrentDevicePaired(baseURL);
        }
      } catch {
        if (to.path !== "/web-access") {
          return { path: "/web-access", query: { redirect: to.fullPath } };
        }
        return true;
      }

      if (webAccess.configured && !webAccess.authenticated) {
        if (to.path !== "/web-access") {
          return { path: "/web-access", query: { redirect: to.fullPath } };
        }
        return true;
      }

      if (!webAccess.configured && to.path === "/web-access") {
        return "/onboarding";
      }

      if (webAccess.configured && webAccess.authenticated && to.path === "/web-access") {
        const redirect = typeof to.query.redirect === "string" && to.query.redirect.startsWith("/")
          ? to.query.redirect
          : "/chat";
        return redirect === "/web-access" ? "/chat" : redirect;
      }
    } else if (to.path === "/web-access") {
      return "/onboarding";
    }
  }

  const isPublic = PUBLIC_PATHS.has(to.path);
  const completed = await readOnboardingCompleted(false);

  if (to.path === "/onboarding") {
    // A Cloud browser may need one-time password setup or Device Mesh pairing
    // even when the shared Space onboarding was completed on another device.
    if (webAccess?.configured === false || webDevicePaired === false) return true;
    if (completed === true) return "/chat";
    return true;
  }

  if ((webAccess?.configured === false || webDevicePaired === false) && !isPublic) {
    return "/onboarding";
  }

  if (!isPublic && completed === false) {
    return "/onboarding";
  }

  return true;
});

export default router;

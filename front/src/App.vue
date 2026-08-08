<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div id="amitia-debug-banner" style="position:fixed;top:0;left:0;right:0;z-index:99999;background:red;color:white;padding:4px 8px;font-size:12px;font-family:monospace;" v-if="debugInfo">
    DEBUG: {{ debugInfo }}
  </div>
  <DesktopTitleBar v-if="isDesktopShell()" />
  <UpdateDialog />
  <MCPInteractionGuard v-if="!isPublicPage && !renderError" />
  <PrivacyConsent v-if="!isPublicPage && !renderError" />
  <NotFoundView v-if="renderError" :error="capturedError" />
  <Transition v-else name="route-slide" mode="out-in">
    <AppLayout v-if="!isPublicPage" key="app">
      <router-view />
    </AppLayout>
    <div
      v-else
      key="public"
      class="public-root"
      :class="{ 'no-leave': route.path === '/login' }"
    >
      <router-view />
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { computed, onErrorCaptured, onMounted, ref } from "vue";
import { useRouter, useRoute } from "vue-router";
import { AppLayout } from "./ui-index";
import { apiClient } from "./ui-index";
import PrivacyConsent from "./components/PrivacyConsent.vue";
import UpdateDialog from "./components/UpdateDialog.vue";
import DesktopTitleBar from "./components/DesktopTitleBar.vue";
import MCPInteractionGuard from "./components/MCPInteractionGuard.vue";
import { useTheme } from "./ui-index";
import { isDesktopShell } from "./runtime/runtime-capabilities";
import NotFoundView from "./views/NotFoundView.vue";

const router = useRouter();
const route = useRoute();
const renderError = ref(false);
const capturedError = ref<string | null>(null);
const debugInfo = ref("");

const TOKEN_KEY = "ai-companion-token";

const publicPaths = [
  "/onboarding",
  "/login",
  "/setup",
  "/privacy",
  "/usage-boundary",
];
const isPublicPage = computed(() =>
  publicPaths.some((p) => route.path === p || route.path.startsWith(p + "/")),
);

function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

onErrorCaptured((err, _instance, info) => {
  console.error("[App] render error:", err, info);
  renderError.value = true;
  capturedError.value = err instanceof Error ? err.message : String(err);
  return false;
});

onMounted(async () => {
  debugInfo.value = `path=${route.path}`;
  console.log("[App] onMounted: route.path =", route.path);

  // TEST: always redirect to onboarding
  debugInfo.value = "FORCE REDIRECT";
  router.replace("/onboarding");
  return;

  // (Below is dead code for testing)
  try {
    const { loadFromServer } = useTheme();
    await loadFromServer();
  } catch {}

  debugInfo.value = `path=${route.path} pub=${publicPaths.some((p) => route.path === p)}`;
  console.log("[App] onMounted: route.path =", route.path, "isPublic:", publicPaths.some((p) => route.path === p));

  if (publicPaths.some((p) => route.path === p)) {
    debugInfo.value += " | SKIP(public)";
    console.log("[App] onMounted: skipping - already on public path");
    return;
  }

  const token = getToken();
  debugInfo.value += ` | tok=${token ? "Y" : "N"}`;
  console.log("[App] onMounted: token =", token ? "exists" : "null");

  try {
    const onboardingRes = await apiClient.get("/api/public/onboarding/status");
    const rawData = JSON.stringify(onboardingRes.data);
    debugInfo.value += ` | onboard=${rawData.substring(0, 80)}`;
    console.log("[App] onboarding status response:", rawData);
    const onboardingData = onboardingRes.data?.data || onboardingRes.data;
    if (!onboardingData?.completed) {
      debugInfo.value += " | REDIRECT→onboarding";
      console.log("[App] onboarding not completed, redirecting to /onboarding");
      router.replace("/onboarding");
      return;
    }
    debugInfo.value += " | onboard=completed";
    console.log("[App] onboarding IS completed, continuing");

    try {
      const authRes = await apiClient.get("/api/public/auth/status");
      const authData = authRes.data?.data || authRes.data;

      if (!authData?.hasAdmin) {
        router.replace("/setup");
        return;
      }

      if (!token) {
        router.replace("/login");
        return;
      }

      try {
        const meRes = await apiClient.get("/api/auth/me");
        const userData = meRes.data?.data || meRes.data;
        if (!userData?.id) {
          localStorage.removeItem(TOKEN_KEY);
          router.replace("/login");
        }
            } catch (e) {
        console.error("[App] auth/me check error:", e);
        localStorage.removeItem(TOKEN_KEY);
        router.replace("/login");
      }
    } catch (e) {
      console.error("[App] auth/status check error:", e);
      if (token) {
        try {
          const meRes = await apiClient.get("/api/auth/me");
          const userData = meRes.data?.data || meRes.data;
          if (!userData?.id) {
            localStorage.removeItem(TOKEN_KEY);
            router.replace("/login");
          }
        } catch {
          localStorage.removeItem(TOKEN_KEY);
          router.replace("/login");
        }
      }
    }
  } catch (e) {
    console.error("[App] outer try error:", e);
  }
});
</script>

<style>
html.amitia-desktop-shell body {
  padding-top: 34px;
  box-sizing: border-box;
  overflow: hidden;
}
html.amitia-desktop-shell #app {
  height: calc(100vh - 34px);
}
html.amitia-desktop-shell .app-shell {
  height: 100%;
}

html.amitia-desktop-shell .el-overlay {
  top: 34px !important;
}

html.amitia-desktop-shell .el-message {
  top: 54px !important;
}
html.amitia-desktop-shell .el-notification {
  top: 54px !important;
}

html.amitia-desktop-shell .el-drawer.ltr,
html.amitia-desktop-shell .el-drawer.rtl,
html.amitia-desktop-shell .el-drawer.ttb,
html.amitia-desktop-shell .el-drawer.btt {
  height: calc(100% - 34px) !important;
}

html.amitia-desktop-shell .search-overlay {
  top: 34px !important;
}
.public-root {
  width: 100%;
  height: 100%;
  overflow: hidden;
}

.route-slide-enter-active,
.route-slide-leave-active {
  transition: opacity 0.325s ease;
}
.route-slide-enter-from,
.route-slide-leave-to {
  opacity: 0;
}

..no-leave.route-slide-leave-active {
  transition: none;
}
.no-leave.route-slide-leave-to {
  opacity: 1;
}

..no-leave.route-slide-enter-active {
  transition: none;
}
..no-leave.route-slide-enter-from {
  opacity: 1;
}
</style>

<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
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
  try {
    const { loadFromServer } = useTheme();
    await loadFromServer();
  } catch {}

  if (publicPaths.some((p) => route.path === p)) {
    return;
  }

  const token = getToken();

  try {
    const onboardingRes = await apiClient.get("/api/public/onboarding/status");
    const onboardingData = onboardingRes.data?.data || onboardingRes.data;
    if (!onboardingData?.completed) {
      router.replace("/onboarding");
      return;
    }

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
      } catch {
        localStorage.removeItem(TOKEN_KEY);
        router.replace("/login");
      }
    } catch {
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
  } catch {}
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
  display: flex;
  flex-direction: column;
}
.public-root.no-leave .route-slide-leave-active {
  display: none;
}
.public-root.no-leave .route-slide-enter-active {
  transition: opacity 0.2s ease;
}
</style>

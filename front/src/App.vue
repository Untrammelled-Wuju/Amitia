<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <DesktopTitleBar v-if="isDesktopShell()" :transparent="isOnboardingPage" />
  <div id="amitia-overlay-root" aria-live="polite"></div>
  <UpdateDialog />
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
import { computed, onErrorCaptured, onMounted, onUnmounted, ref, watch } from "vue";
import { useRouter, useRoute } from "vue-router";
import { AppLayout } from "./ui-index";
import { apiClient } from "./ui-index";
import PrivacyConsent from "./components/PrivacyConsent.vue";
import UpdateDialog from "./components/UpdateDialog.vue";
import DesktopTitleBar from "./components/DesktopTitleBar.vue";
import { useTheme } from "./ui-index";
import { isDesktopShell } from "./runtime/runtime-capabilities";
import NotFoundView from "./views/NotFoundView.vue";
import { useExtensionUIStore } from "./stores/extensionUI";

const router = useRouter();
const route = useRoute();
const renderError = ref(false);
const capturedError = ref<string | null>(null);
const extensionUIStore = useExtensionUIStore();

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
const isOnboardingPage = computed(
  () => route.path === "/onboarding" || route.path.startsWith("/onboarding/"),
);

watch(
  isOnboardingPage,
  (isOnboarding) => {
    document.documentElement.classList.toggle(
      "amitia-desktop-onboarding",
      isDesktopShell() && isOnboarding,
    );
  },
  { immediate: true },
);

onErrorCaptured((err, _instance, info) => {
  const errorMessage = err instanceof Error ? err.message : String(err);
  if (errorMessage.includes("Cannot read properties of null") || errorMessage.includes("reading 'type'") || errorMessage.includes("reading 'exposed'")) {
    console.warn("[App] suppressed unmount error:", errorMessage);
    return false;
  }
  console.error("[App] render error:", err, info);
  renderError.value = true;
  capturedError.value = errorMessage;
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

  try {
    const onboardingRes = await apiClient.get("/api/public/onboarding/status");
    const onboardingData = onboardingRes.data?.data || onboardingRes.data;
    if (!onboardingData?.completed) {
      router.replace("/onboarding");
      return;
    }
  } catch {}

  try {
    const authRes = await apiClient.get("/api/public/auth/status");
    const authData = authRes.data?.data || authRes.data;
    if (!authData?.hasAdmin) {
      router.replace("/setup");
      return;
    }
  } catch {}

  try {
    const meRes = await apiClient.get("/api/auth/me");
    const userData = meRes.data?.data || meRes.data;
    if (!userData?.id) {
      router.replace("/login");
    } else {
      extensionUIStore.refreshSnapshot(true).catch(() => {});
    }
  } catch {
    router.replace("/login");
  }
});

onUnmounted(() => {
  document.documentElement.classList.remove("amitia-desktop-onboarding");
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

html.amitia-desktop-shell.amitia-desktop-onboarding body {
  padding-top: 0;
}

html.amitia-desktop-shell.amitia-desktop-onboarding #app {
  height: 100vh;
}

html.amitia-desktop-shell:has(.onboarding-world) body {
  padding-top: 0 !important;
}

html.amitia-desktop-shell:has(.onboarding-world) #app {
  height: 100vh !important;
}

html.amitia-desktop-shell:has(.onboarding-world) #WindowControlButtons {
  position: fixed !important;
  background: transparent !important;
  -webkit-backdrop-filter: none !important;
  backdrop-filter: none !important;
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

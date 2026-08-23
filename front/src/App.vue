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
      <router-view v-slot="{ Component }">
        <RouteSurfaceHost v-if="Component" :fallback="Component" />
      </router-view>
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
import { syncProviderRoutes } from "./ui-runtime/providerRoutes";
import { applyProviderTheme } from "./ui-runtime/providerTheme";
import RouteSurfaceHost from "./components/ui-runtime/RouteSurfaceHost.vue";

const router = useRouter();
const route = useRoute();
const renderError = ref(false);
const capturedError = ref<string | null>(null);
const extensionUIStore = useExtensionUIStore();
const themeRuntime = useTheme();

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
const isUIProviderRecoveryPage = computed(() => route.path === "/settings/ui-providers");

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
  if (errorMessage.includes("Request failed") || errorMessage.includes("404") || errorMessage.includes("401") || errorMessage.includes("500")) {
    console.warn("[App] API error suppressed:", errorMessage);
    return false;
  }
  console.error("[App] render error:", err, info);
  renderError.value = true;
  capturedError.value = errorMessage;
  return false;
});

onMounted(async () => {
  try {
    await themeRuntime.loadFromServer();
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
      extensionUIStore.refreshSnapshot(true).then(() => syncProviderRoutes(router, extensionUIStore)).catch(() => {});
    }
  } catch {
    router.replace("/login");
  }
});

watch(() => extensionUIStore.snapshot?.providerVersion, () => {
  syncProviderRoutes(router, extensionUIStore);
});

watch(
  [
    () => extensionUIStore.snapshot?.resolved?.["ui.theme"],
    () => extensionUIStore.snapshot?.resolved?.["ui.tokens"],
    () => extensionUIStore.snapshot?.resolved?.["ui.icons"],
    () => extensionUIStore.snapshot?.resolved?.["ui.components"],
    () => themeRuntime.resolvedMode.value,
    () => isUIProviderRecoveryPage.value,
  ],
  ([themeProvider, tokenProvider, iconProvider, componentProvider, mode, recovery]) =>
    applyProviderTheme(
      recovery ? [] : [themeProvider ?? null, tokenProvider ?? null, iconProvider ?? null, componentProvider ?? null],
      mode,
    ),
  { immediate: true },
);

onUnmounted(() => {
  document.documentElement.classList.remove("amitia-desktop-onboarding");
});
</script>

<style>
/* Cross-platform declarative ui.components primitives. These variables are
   emitted by providerTheme.ts from metadata.componentVariants. */
:root {
  --ui-component-button-min-height: var(--el-component-size);
  --ui-component-button-radius: var(--el-border-radius-base);
  --ui-component-button-gap: 6px;
  --ui-component-button-font-size: inherit;
  --ui-component-button-font-weight: inherit;
  --ui-component-button-padding-x: var(--el-button-padding-horizontal, 15px);

  --ui-component-input-min-height: var(--el-component-size);
  --ui-component-input-radius: var(--el-input-border-radius, var(--el-border-radius-base));
  --ui-component-input-padding-x: 11px;

  --ui-component-dialog-radius: var(--el-border-radius-base);
  --ui-component-card-radius: var(--el-card-border-radius, var(--el-border-radius-base));
}
.el-button {
  min-height: var(--ui-component-button-min-height, var(--el-component-size));
  border-radius: var(--ui-component-button-radius, var(--el-border-radius-base));
  gap: var(--ui-component-button-gap, 6px);
  font-size: var(--ui-component-button-font-size, inherit);
  font-weight: var(--ui-component-button-font-weight, inherit);
  padding-left: var(--ui-component-button-padding-x, var(--el-button-padding-horizontal));
  padding-right: var(--ui-component-button-padding-x, var(--el-button-padding-horizontal));
}
.el-input__wrapper, .el-textarea__inner {
  min-height: var(--ui-component-input-min-height, var(--el-component-size));
  border-radius: var(--ui-component-input-radius, var(--el-input-border-radius, var(--el-border-radius-base)));
}
.el-input__wrapper {
  padding-left: var(--ui-component-input-padding-x, 11px);
  padding-right: var(--ui-component-input-padding-x, 11px);
}
.el-dialog {
  border-radius: var(--ui-component-dialog-radius, var(--el-border-radius-base));
}
.el-card {
  border-radius: var(--ui-component-card-radius, var(--el-card-border-radius, var(--el-border-radius-base)));
}

.ui-provider-recovery-root {
  width: 100%;
  height: 100%;
  overflow: auto;
  box-sizing: border-box;
  padding: 24px;
  color: #303133;
  background: #f5f7fa;
  font-family: Inter, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
}
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

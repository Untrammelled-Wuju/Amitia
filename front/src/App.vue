<!--
SPDX-FileCopyrightText: 2026 彭旭
SPDX-License-Identifier: AGPL-3.0-only
-->
<template>
  <div class="desktop-root" :class="{ 'is-titlebar-overlay': isOnboardingPage }">
    <DesktopTitleBar :transparent="isOnboardingPage" />
    <AppContextMenu />
    <div class="desktop-stage">
      <UpdateDialog />
      <AgentApprovalGuard />
      <PrivacyConsent v-if="!isPublicPage && !renderError" />
      <NotFoundView v-if="renderError" :error="capturedError" />
      <div v-else class="workspace-root">
        <ApplicationBackground :visible="!isPublicPage" />
        <div class="workspace-foreground">
          <Transition name="route-slide" mode="out-in">
            <AppLayout v-if="!isPublicPage" key="app">
              <router-view v-slot="{ Component }">
                <RouteSurfaceHost v-if="Component" :fallback="Component" />
              </router-view>
            </AppLayout>
            <div v-else key="public" class="public-root">
              <router-view />
            </div>
          </Transition>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onErrorCaptured, onMounted, onUnmounted, ref, watch } from "vue";
import { useRouter, useRoute } from "vue-router";
import { ElMessage } from "element-plus";
import { AppLayout } from "./ui-index";
import { apiClient } from "./ui-index";
import PrivacyConsent from "./components/PrivacyConsent.vue";
import UpdateDialog from "./components/UpdateDialog.vue";
import AgentApprovalGuard from "./components/AgentApprovalGuard.vue";
import DesktopTitleBar from "./components/DesktopTitleBar.vue";
import { useTheme } from "./ui-index";
import { isDesktopShell } from "./runtime/runtime-capabilities";
import NotFoundView from "./views/NotFoundView.vue";
import { useExtensionUIStore } from "./stores/extensionUI";
import { syncProviderRoutes } from "./ui-runtime/providerRoutes";
import { applyProviderTheme } from "./ui-runtime/providerTheme";
import { applyStylePatchToDocument, applyStyleRuntime } from "./ui-runtime/styleRuntime";
import { initializeStyleProfile, stylePatch, styleRules } from "./ui-runtime/styleProfile";
import { normalizeStylePageId } from "./ui-runtime/stylePage";
import RouteSurfaceHost from "./components/ui-runtime/RouteSurfaceHost.vue";
import { useAppUIHost } from "./composables/useAppUIHost";
import ApplicationBackground from "./components/ApplicationBackground.vue";
import AppContextMenu from "./components/context-menu/AppContextMenu.vue";
import { originalErrorMessage } from "./runtime/original-error";

const router = useRouter();
const route = useRoute();
const renderError = ref(false);
const capturedError = ref<string | null>(null);
const extensionUIStore = useExtensionUIStore();
const themeRuntime = useTheme();
initializeStyleProfile();

const publicPaths = [
  "/web-access",
  "/onboarding",
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
useAppUIHost(isPublicPage);

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

watch(
  route,
  () => {
    document.documentElement.dataset.amitiaPage = normalizeStylePageId(route.fullPath);
    document.documentElement.dataset.amitiaPlatform = isDesktopShell() ? "desktop" : "web";
    applyStyleRuntime(stylePatch.value, styleRules.value);
  },
  { immediate: true },
);

watch(
  [stylePatch, styleRules],
  ([patch, rules]) => {
    applyStyleRuntime(patch, rules);
  },
  { immediate: true, deep: true },
);

onErrorCaptured((err, _instance, info) => {
  const errorMessage = originalErrorMessage(err);
  console.error("[App] render error:", err, info);
  renderError.value = true;
  capturedError.value = errorMessage;
  return false;
});

onMounted(async () => {
  if (publicPaths.some((p) => route.path === p)) {
    return;
  }

  try {
    await themeRuntime.loadFromServer();
  } catch (error) {
    console.error("[original-error]", error);
    ElMessage.error(originalErrorMessage(error));
  }

  try {
    const onboardingRes = await apiClient.get("/api/public/onboarding/status");
    const onboardingData = onboardingRes.data?.data || onboardingRes.data;
    if (!onboardingData?.completed) {
      router.replace("/onboarding");
      return;
    }
  } catch (error) {
    console.error("[original-error]", error);
    ElMessage.error(originalErrorMessage(error));
  }

  extensionUIStore
    .refreshSnapshot()
    .then(() => syncProviderRoutes(router, extensionUIStore))
    .catch((error) => {
      console.error("[original-error]", error);
      ElMessage.error(originalErrorMessage(error));
    });
});

watch(
  [
    () => extensionUIStore.snapshot?.providers,
    () => extensionUIStore.snapshot?.providerContext,
  ],
  () => {
    syncProviderRoutes(router, extensionUIStore);
  },
);

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
  {
    applyProviderTheme(
      recovery ? [] : [themeProvider ?? null, tokenProvider ?? null, iconProvider ?? null, componentProvider ?? null],
      mode,
    );
    applyStylePatchToDocument(stylePatch.value);
  },
  { immediate: true },
);

onUnmounted(() => {
  document.documentElement.classList.remove("amitia-desktop-onboarding");
});
</script>

<style>
details {
  margin: 2px 0 2px;
}

.desktop-root {
  display: grid;
  grid-template-rows: var(--desktop-titlebar-height, 40px) minmax(0, 1fr);
  height: 100%;
  overflow: hidden;
}
.desktop-stage {
  position: relative;
  min-height: 0;
  height: 100%;
  overflow: hidden;
}
.desktop-root.is-titlebar-overlay {
  display: block;
}
.workspace-root { position: relative; height: 100%; min-height: 0; overflow: hidden; }
.workspace-foreground { position: relative; z-index: 1; height: 100%; }
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

  --ui-component-dialog-radius: var(--ac-radius-xl);
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
html.amitia-titlebar-shell .app-shell {
  height: 100%;
}

html.amitia-titlebar-shell .el-overlay {
  top: 0 !important;
  z-index: 20000 !important;
}

html.amitia-titlebar-shell .el-popper {
  z-index: 20001 !important;
}

html.amitia-titlebar-shell .el-message {
  top: 20px !important;
  z-index: 20010 !important;
}
html.amitia-titlebar-shell .el-notification {
  top: 20px !important;
  z-index: 20010 !important;
}

html.amitia-titlebar-shell .el-drawer.ltr,
html.amitia-titlebar-shell .el-drawer.rtl,
html.amitia-titlebar-shell .el-drawer.ttb,
html.amitia-titlebar-shell .el-drawer.btt {
  height: 100% !important;
  padding-top: var(--desktop-titlebar-height, 40px);
  box-sizing: border-box;
}

html.amitia-titlebar-shell .search-overlay {
  top: 0 !important;
}

html[data-theme="dark"] #WindowControlButtons:not(.is-onboarding) {
  background: var(--workbench-sidebar-bg);
}

html:has(.workspace-sidebar-resizer:hover) #WindowControlButtons .window-chat-slot::before,
html.is-resizing-workspace-sidebar #WindowControlButtons .window-chat-slot::before {
  background-color: var(--ac-color-primary) !important;
}

html.amitia-desktop-shell:has(.onboarding-world) .desktop-root,
html.amitia-desktop-shell:has(.login-world) .desktop-root {
  display: block;
}

html.amitia-desktop-onboarding #WindowControlButtons,
html.amitia-desktop-shell:has(.onboarding-world) #WindowControlButtons,
html.amitia-desktop-shell:has(.login-world) #WindowControlButtons {
  position: fixed !important;
  top: 0;
  right: 0;
  left: 0;
  z-index: 10000;
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

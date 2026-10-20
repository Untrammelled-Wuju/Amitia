// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { createApp } from "vue";
import { createPinia } from "pinia";
import ElementPlus from "element-plus";
import "element-plus/dist/index.css";
import zhCn from "element-plus/dist/locale/zh-cn.mjs";

// Design tokens
import "./styles/theme-light.css";
import "./styles/theme-dark.css";
import "./styles/variables.css";
import "./styles/element-overrides.css";

import App from "./App.vue";
import router from "./router";
import { getRuntimeConnection } from "./runtime/runtime-adapter";
import { initializeRuntimeCapabilities, shouldRegisterServiceWorker } from "./runtime/runtime-capabilities";
import { installClientErrorTracking } from "./runtime/client-error-tracker";
import { setErrorPanelHandler, setErrorBannerHandler } from "./ui-index";
import { useExtensionUIStore } from "./stores/extensionUI";
import { browserClientPluginRuntime, syncBrowserClientSlots } from "./ui-runtime/clientPluginRuntime";
import { syncProviderRoutes } from "./ui-runtime/providerRoutes";

async function bootstrap() {
  await getRuntimeConnection();
  const app = createApp(App);
  const pinia = createPinia();
  app.use(pinia);
  const extensionUI = useExtensionUIStore(pinia);
  await initializeRuntimeCapabilities(true);
  const initialRoute = router.resolve(window.location.href);
  const bootstrapPublicRoute = ["/web-access", "/onboarding", "/privacy", "/usage-boundary", "/call-window"].includes(initialRoute.path);
  if (!bootstrapPublicRoute) {
    await extensionUI.refreshSnapshot().catch(() => undefined);
    syncProviderRoutes(router, extensionUI);
  }
  let extensionRouteRecoveryPending = false;
  router.beforeEach(async (to) => {
    if (to.name !== "catchAll" || extensionRouteRecoveryPending) return true;
    extensionRouteRecoveryPending = true;
    try {
      await extensionUI.refreshSnapshot(true).catch(() => undefined);
      syncProviderRoutes(router, extensionUI);
      const recovered = router.resolve(to.fullPath);
      if (!recovered.matched.some((record) => record.name === "catchAll")) {
        return { path: to.path, query: to.query, hash: to.hash, replace: true };
      }
    } finally {
      extensionRouteRecoveryPending = false;
    }
    return true;
  });
  app.use(router);
  app.use(ElementPlus, { locale: zhCn });
  installClientErrorTracking(app);
  await router.isReady();

  if (typeof window !== "undefined" && !window.amitiaDesktop) {
    window.addEventListener("amitia:web-access-expired", () => {
      const current = router.currentRoute.value;
      if (current.path === "/web-access") return;
      void router.replace({ path: "/web-access", query: { redirect: current.fullPath } });
    });
  }

  if (!bootstrapPublicRoute) {
    await syncBrowserClientSlots(extensionUI.snapshot);
  }
  extensionUI.$subscribe((_mutation, state) => { void syncBrowserClientSlots(state.snapshot); });
  window.amitiaClientPlugins = browserClientPluginRuntime;
  setErrorPanelHandler((err) => {
    import("element-plus").then(({ ElMessageBox }) => {
      ElMessageBox.alert(err.detail || err.message, err.message, {
        type: "error",
        confirmButtonText: err.action?.label || "OK",
      }).then(() => err.action?.handler?.());
    });
  });
  setErrorBannerHandler((err) => {
    import("element-plus").then(({ ElNotification }) => {
      ElNotification({
        title: err.message,
        message: err.detail || "",
        type: "warning",
        duration: 6000,
      });
    });
  });

  app.mount("#app");
  // There is no product-account gate. In Cloud browser mode the lightweight
  // Web access gate is already resolved before private extension UI boots.
  void import("./components/extension/WebComposerActionProxy.vue");
  void import("./components/extension/SandboxWebUIFrame.vue");
}

void bootstrap();

// ============================================================
// Register Service Worker (PWA)
// ============================================================
if (shouldRegisterServiceWorker() && "serviceWorker" in navigator) {
  window.addEventListener("load", () => {
    navigator.serviceWorker
      .register("/sw.js", { scope: "/" })
      .then((registration) => {
        console.log("[PWA] Service Worker registered:", registration.scope);

        // Check for updates
        registration.addEventListener("updatefound", () => {
          const newWorker = registration.installing;
          if (newWorker) {
            newWorker.addEventListener("statechange", () => {
              if (
                newWorker.state === "installed" &&
                navigator.serviceWorker.controller
              ) {
                console.log("[PWA] New version available - refresh to update");
              }
            });
          }
        });
      })
      .catch((err) => {
        console.warn("[PWA] Service Worker registration failed:", err);
      });
  });
}

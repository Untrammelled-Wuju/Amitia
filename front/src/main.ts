// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { createApp } from "vue"
import { createPinia } from "pinia"
import ElementPlus from "element-plus"
import "element-plus/dist/index.css"
import zhCn from "element-plus/dist/locale/zh-cn.mjs"

// Design tokens
import "./styles/variables.css"
import "./styles/element-overrides.css"
import "./styles/theme-presets.css"

import App from "./App.vue"
import router from "./router"

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.use(ElementPlus, { locale: zhCn })

// Setup unified error handling
import { setErrorPanelHandler, setErrorBannerHandler } from "./ui-index"
setErrorPanelHandler((err) => {
  // In web mode, show a simple alert for panel-level errors
  import("element-plus").then(({ ElMessageBox }) => {
    ElMessageBox.alert(err.detail || err.message, err.message, {
      type: "error",
      confirmButtonText: err.action?.label || "OK",
    }).then(() => err.action?.handler?.())
  })
})
setErrorBannerHandler((err) => {
  import("element-plus").then(({ ElNotification }) => {
    ElNotification({ title: err.message, message: err.detail || "", type: "warning", duration: 6000 })
  })
})

app.mount("#app")

// ============================================================
// Register Service Worker (PWA)
// ============================================================
if ('serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    navigator.serviceWorker.register('/sw.js', { scope: '/' })
      .then((registration) => {
        console.log('[PWA] Service Worker registered:', registration.scope)

        // Check for updates
        registration.addEventListener('updatefound', () => {
          const newWorker = registration.installing
          if (newWorker) {
            newWorker.addEventListener('statechange', () => {
              if (newWorker.state === 'installed' && navigator.serviceWorker.controller) {
                console.log('[PWA] New version available - refresh to update')
              }
            })
          }
        })
      })
      .catch((err) => {
        console.warn('[PWA] Service Worker registration failed:', err)
      })
  })
}
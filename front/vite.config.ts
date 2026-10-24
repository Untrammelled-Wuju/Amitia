// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { defineConfig } from "vite"
import vue from "@vitejs/plugin-vue"
import { resolve } from "path"
import { existsSync, readFileSync } from "fs"

const apiTarget = process.env.AMITIA_WEB_API_TARGET || "http://127.0.0.1:18899"
const localTokenPath = resolve(__dirname, "../AmitiaData/security/local-token")

const apiProxy = {
  target: apiTarget,
  changeOrigin: true,
  configure(proxy: any) {
    proxy.on("proxyReq", (request: any) => {
      request.removeHeader("origin")
      if (!existsSync(localTokenPath)) return
      const token = readFileSync(localTokenPath, "utf8").trim()
      if (token) request.setHeader("X-Amitia-Local-Token", token)
    })
  },
}

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      "@": resolve(__dirname, "src"),
    },
  },
  server: {
    port: 0,
    strictPort: false,
    proxy: {
      "/api": apiProxy,
      "/internal": apiProxy,
      "/media": apiProxy,
      "/livez": apiProxy,
      "/readyz": apiProxy,
    },
  },
})

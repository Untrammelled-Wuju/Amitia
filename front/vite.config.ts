// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { defineConfig } from "vite"
import vue from "@vitejs/plugin-vue"
import { resolve } from "path"
import { existsSync, readFileSync } from "fs"

const apiTarget = process.env.AMITIA_WEB_API_TARGET?.trim() || ""
const localTokenPath = resolve(__dirname, "../AmitiaData/security/local-token")

const apiProxy = apiTarget ? {
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
} : undefined

const proxy = apiProxy ? {
  "/api": apiProxy,
  "/internal": apiProxy,
  "/media": apiProxy,
  "/livez": apiProxy,
  "/readyz": apiProxy,
} : undefined

export default defineConfig({
  plugins: [vue()],
  optimizeDeps: {
    exclude: ["@element-plus/icons-vue"],
  },
  resolve: {
    alias: [
      {
        find: /^@element-plus\/icons-vue(?:\?.*)?$/,
        replacement: resolve(
          __dirname,
          "src/components/icons/element-plus-icons.ts",
        ),
      },
      {
        find: "@",
        replacement: resolve(__dirname, "src"),
      },
    ],
  },
  server: {
    port: 0,
    strictPort: false,
    proxy,
  },
})

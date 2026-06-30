// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { defineConfig } from "vite"
import vue from "@vitejs/plugin-vue"
import { resolve } from "path"

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      "@": resolve(__dirname, "src"),
    },
  },
  server: {
    port: 5178,
    proxy: {
      "/api": {
        target: "http://127.0.0.1:8899",
        changeOrigin: true,
      },
      "/bridge": {
        target: "http://127.0.0.1:8898",
        changeOrigin: true,
      },
      "/voice": {
        target: "http://127.0.0.1:8899",
        changeOrigin: true,
      },
      "/audio": {
        target: "http://127.0.0.1:8899",
        changeOrigin: true,
      },
      "/images": {
        target: "http://127.0.0.1:8899",
        changeOrigin: true,
      },
      "/videos": {
        target: "http://127.0.0.1:8899",
        changeOrigin: true,
      },
    },
  },
})
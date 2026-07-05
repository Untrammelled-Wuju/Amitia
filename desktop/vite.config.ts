import { resolve } from "node:path"
import vue from "@vitejs/plugin-vue"
import { defineConfig } from "vitest/config"
import electron from "vite-plugin-electron"

export default defineConfig({
  root: resolve(__dirname, "../front"),
  publicDir: false,
  server: {
    proxy: {
      "/api": {
        target: "http://127.0.0.1:18899",
        changeOrigin: true,
      },
    },
  },
  resolve: {
    alias: {
      "@": resolve(__dirname, "../front/src"),
      "/src": resolve(__dirname, "../front/src"),
    },
  },
  build: {
    emptyOutDir: true,
    outDir: resolve(__dirname, "dist/renderer"),
    rollupOptions: {
      input: resolve(__dirname, "../front/index.html"),
    },
  },
  test: {
    root: __dirname,
    environment: "jsdom",
    include: ["src/**/*.test.ts", "src/**/*.spec.ts"],
    passWithNoTests: true,
  },
  plugins: [
    vue(),
    electron([
      {
        entry: resolve(__dirname, "src/main/index.ts"),
        vite: {
          build: {
            outDir: resolve(__dirname, "dist/main"),
            rollupOptions: {
              external: ["electron"],
            },
          },
        },
      },
      {
        entry: resolve(__dirname, "src/preload/index.ts"),
        onstart(options) {
          options.reload()
        },
        vite: {
          build: {
            outDir: resolve(__dirname, "dist/preload"),
            rollupOptions: {
              external: ["electron"],
              output: {
                entryFileNames: "index.mjs",
              },
            },
          },
        },
      },
    ]),
  ],
})

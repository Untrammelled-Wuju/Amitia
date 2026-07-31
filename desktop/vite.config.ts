import { resolve } from "node:path"
import type { IncomingMessage, ServerResponse } from "node:http"
import vue from "@vitejs/plugin-vue"
import { defineConfig } from "vitest/config"
import type { ViteDevServer } from "vite"
import electron from "vite-plugin-electron/simple"

function petDevServerPlugin() {
  return {
    name: "pet-dev-server",
    configureServer(server: ViteDevServer) {
      server.middlewares.use(
        "/pet.html",
        (_req: IncomingMessage, _res: ServerResponse, next: () => void) => {
          next()
        },
      )
    },
  }
}

export default defineConfig({
  root: resolve(__dirname, "../front"),
  publicDir: false,
  server: {
    port: 5178,
    fs: {
      allow: [
        resolve(__dirname, "../front"),
        resolve(__dirname, "."),
        resolve(__dirname, ".."),
      ],
    },
    proxy: {
      "/api": {
        target: "http://127.0.0.1:18899",
        changeOrigin: true,
      },
      "/audio": {
        target: "http://127.0.0.1:18899",
        changeOrigin: true,
      },
      "/voice": {
        target: "http://127.0.0.1:18899",
        changeOrigin: true,
      },
      "/images": {
        target: "http://127.0.0.1:18899",
        changeOrigin: true,
      },
      "/videos": {
        target: "http://127.0.0.1:18899",
        changeOrigin: true,
      },
      "/avatars": {
        target: "http://127.0.0.1:18899",
        changeOrigin: true,
      },
      "/emote-assets": {
        target: "http://127.0.0.1:18899",
        changeOrigin: true,
      },
    },
  },
  resolve: {
    dedupe: ["vue", "@vue/runtime-core", "@vue/runtime-dom"],
    alias: {
      "@": resolve(__dirname, "../front/src"),
    },
  },
  build: {
    emptyOutDir: true,
    outDir: resolve(__dirname, "dist/renderer"),
    rollupOptions: {
      input: {
        main: resolve(__dirname, "../front/index.html"),
        pet: resolve(__dirname, "src/renderer/pet.html"),
      },
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
    petDevServerPlugin(),
    electron({
      main: {
        entry: resolve(__dirname, "src/main/index.ts"),
        vite: {
          build: {
            outDir: resolve(__dirname, "dist/main"),
            lib: {
              entry: resolve(__dirname, "src/main/index.ts"),
              formats: ["cjs"],
              fileName: () => "[name].cjs",
            },
            rolldownOptions: {
              external: ["electron", "electron-updater", "electron-log", "electron-is-dev"],
              output: {
                format: "cjs",
                entryFileNames: "[name].cjs",
              },
            },
          },
        },
      },
      preload: {
        input: {
          index: resolve(__dirname, "src/preload/index.ts"),
          "pet-preload": resolve(__dirname, "src/preload/pet-preload.ts"),
          "pet-combined-preload": resolve(__dirname, "src/preload/pet-combined-preload.ts"),
          "animation-preload": resolve(__dirname, "src/preload/animation-preload.ts"),
        },
        vite: {
          build: {
            outDir: resolve(__dirname, "dist/preload"),
            rollupOptions: {
              external: ["electron"],
              output: {
                entryFileNames: "[name].cjs",
                chunkFileNames: "[name].cjs",
                codeSplitting: true,
              },
            },
          },
        },
      },
    }),
  ],
})

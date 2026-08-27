import { readFileSync } from "node:fs"
import { resolve } from "node:path"
import type { IncomingMessage, ServerResponse } from "node:http"
import vue from "@vitejs/plugin-vue"
import { defineConfig } from "vitest/config"
import type { ViteDevServer } from "vite"
import electron from "vite-plugin-electron/simple"

function petDevServerPlugin() {
  const petHtmlPath = resolve(__dirname, "src/renderer/pet.html")
  const petMainPath = resolve(__dirname, "src/renderer/pet-main.ts").replace(/\\/g, "/")
  const petMainDevUrl = `/@fs/${petMainPath}`

  return {
    name: "pet-dev-server",
    configureServer(server: ViteDevServer) {
      server.middlewares.use(
        "/pet.html",
        async (_req: IncomingMessage, res: ServerResponse) => {
          try {
            const source = readFileSync(petHtmlPath, "utf8")
            const devHtml = source.replace(
              'src="./pet-main.js"',
              `src="${petMainDevUrl}"`,
            )
            const html = await server.transformIndexHtml("/pet.html", devHtml)
            res.statusCode = 200
            res.setHeader("Content-Type", "text/html; charset=utf-8")
            res.end(html)
          } catch (error) {
            res.statusCode = 500
            res.setHeader("Content-Type", "text/plain; charset=utf-8")
            res.end(error instanceof Error ? error.message : String(error))
          }
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

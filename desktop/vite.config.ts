import { resolve } from "node:path"
import { copyFileSync, existsSync, mkdirSync, writeFileSync, readFileSync } from "node:fs"
import vue from "@vitejs/plugin-vue"
import { defineConfig } from "vitest/config"
import electron from "vite-plugin-electron/simple"

function copyPetHtmlPlugin() {
  function copyPetFiles() {
    const srcHtml = resolve(__dirname, "src/renderer/pet.html")
    const destDir = resolve(__dirname, "dist/renderer")
    const destHtml = resolve(destDir, "pet.html")
    if (existsSync(srcHtml)) {
      if (!existsSync(destDir)) {
        mkdirSync(destDir, { recursive: true })
      }
      copyFileSync(srcHtml, destHtml)
    }

    const frontDir = resolve(__dirname, "../front")
    const frontPetHtml = resolve(frontDir, "pet.html")
    if (existsSync(srcHtml)) {
      let htmlContent = readFileSync(srcHtml, "utf8")
      htmlContent = htmlContent.replace(
        '<script type="module" src="./pet-main.ts"></script>',
        '<script type="module" src="/@fs' + resolve(__dirname, "src/renderer/pet-main.ts") + '"></script>',
      )
      writeFileSync(frontPetHtml, htmlContent, "utf8")
    }
  }

  return {
    name: "copy-pet-html",
    configureServer() {
      copyPetFiles()
    },
    configurePreviewServer() {
      const src = resolve(__dirname, "src/renderer/pet.html")
      const destDir = resolve(__dirname, "dist/renderer")
      const dest = resolve(destDir, "pet.html")
      if (existsSync(src)) {
        if (!existsSync(destDir)) {
          mkdirSync(destDir, { recursive: true })
        }
        copyFileSync(src, dest)
      }
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
    copyPetHtmlPlugin(),
    electron({
      main: {
        entry: resolve(__dirname, "src/main/index.ts"),
        vite: {
          build: {
            outDir: resolve(__dirname, "dist/main"),
            rollupOptions: {
              external: ["electron", "electron-updater", "electron-log", "electron-is-dev"],
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

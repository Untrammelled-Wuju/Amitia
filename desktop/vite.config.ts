import { resolve } from "node:path"
import { defineConfig } from "vite"
import electron from "vite-plugin-electron/simple"

export default defineConfig({
  publicDir: false,
  build: {
    emptyOutDir: true,
    outDir: "dist/renderer",
    rollupOptions: {
      input: resolve(__dirname, "../front/index.html"),
    },
  },
  plugins: [
    electron({
      main: {
        entry: "src/main/index.ts",
        vite: {
          build: {
            outDir: "dist/main",
            rollupOptions: {
              external: ["electron"],
            },
          },
        },
      },
      preload: {
        input: "src/preload/index.ts",
        vite: {
          build: {
            outDir: "dist/preload",
            rollupOptions: {
              external: ["electron"],
            },
          },
        },
      },
    }),
  ],
})

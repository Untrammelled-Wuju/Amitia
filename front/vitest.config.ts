// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import { defineConfig } from "vitest/config";
import vue from "@vitejs/plugin-vue";
import { resolve } from "path";

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      "@": resolve(__dirname, "src"),
      "@element-plus/icons-vue": resolve(
        __dirname,
        "src/components/icons/element-plus-icons.ts",
      ),
    },
  },
  test: {
    globals: true,
    environment: "jsdom",
    testTimeout: 15000,
    include: ["src/__tests__/**/*.test.ts"],
    setupFiles: ["./vitest.setup.ts"],
  },
});

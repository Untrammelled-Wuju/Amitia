import { resolve } from "node:path";
import { fileURLToPath } from "node:url";
import vue from "@vitejs/plugin-vue";
import { build } from "vite";

const scriptDir = resolve(fileURLToPath(new URL(".", import.meta.url)));
const repositoryRoot = resolve(scriptDir, "..", "..");
const frontRoot = resolve(repositoryRoot, "front");
const frontNodeModules = resolve(frontRoot, "node_modules");

async function buildChannel(channel) {
  const root = resolve(repositoryRoot, "plugins", "channels", channel, "ui");
  for (const platform of ["desktop", "mobile"]) {
    await build({
      root,
      configFile: false,
      base: "./",
      plugins: [vue()],
      resolve: {
        alias: {
          vue: resolve(frontNodeModules, "vue"),
          "element-plus": resolve(frontNodeModules, "element-plus"),
          "@element-plus/icons-vue": resolve(
            frontNodeModules,
            "@element-plus",
            "icons-vue",
          ),
        },
      },
      build: {
        outDir: resolve(root, platform),
        emptyOutDir: true,
        cssCodeSplit: false,
        rollupOptions: {
          input: resolve(root, "index.html"),
          output: {
            entryFileNames: "app.js",
            chunkFileNames: "chunks/[name].js",
            assetFileNames: (assetInfo) =>
              assetInfo.name?.endsWith(".css")
                ? "styles.css"
                : "assets/[name][extname]",
          },
        },
      },
    });
  }
}

for (const channel of ["qq", "wechat"]) {
  await buildChannel(channel);
}

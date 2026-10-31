import fs from "node:fs"
import path from "node:path"
import { createRequire } from "node:module"
import { fileURLToPath, pathToFileURL } from "node:url"
import { buildChannelPackage } from "../../scripts/build-channel-package.mjs"
import { prepareClientUI } from "../../scripts/prepare-client-ui.mjs"
import { clientCssPlugin } from "../../scripts/client-css-scope.mjs"
import { hostVueModulePlugin } from "../../scripts/host-vue-module.mjs"

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..")
const projectRoot = path.resolve(root, "../../..")
const uiRoot = path.join(root, "ui")
const runtimeRoot = path.join(root, "runtime")
const stagingRoot = path.join(root, ".build", "package")
const frontendRequire = createRequire(path.join(projectRoot, "front", "package.json"))
const uiRequire = createRequire(path.join(uiRoot, "package.json"))
const runtimeNodePaths = fs.readdirSync(path.join(runtimeRoot, "node_modules", ".pnpm"), { withFileTypes: true })
  .filter((entry) => entry.isDirectory())
  .map((entry) => path.join(runtimeRoot, "node_modules", ".pnpm", entry.name, "node_modules"))
  .filter((entry) => fs.existsSync(entry))

function dependency(name, fallback) {
  try {
    return uiRequire.resolve(name)
  } catch (error) {
  console.error("[original-error]", error);
return fallback.resolve(name)
  }
}

const viteRoot = path.dirname(dependency("vite/package.json", frontendRequire))
const vuePluginRoot = path.dirname(dependency("@vitejs/plugin-vue", frontendRequire))
const { build } = await import(pathToFileURL(path.join(viteRoot, "dist/node/index.js")).href)
const { default: vue } = await import(pathToFileURL(path.join(vuePluginRoot, "index.mjs")).href)
const { build: buildRuntime } = createRequire(path.join(viteRoot, "package.json"))("esbuild")
const manifest = JSON.parse(fs.readFileSync(path.join(root, "amitia-extension.json"), "utf8"))

fs.rmSync(stagingRoot, { recursive: true, force: true })
fs.mkdirSync(path.join(stagingRoot, "modules", "qq-channel-service"), { recursive: true })
fs.mkdirSync(path.join(stagingRoot, "modules", "qq-channel-ui", "desktop"), { recursive: true })
fs.mkdirSync(path.join(stagingRoot, "modules", "qq-channel-ui", "mobile"), { recursive: true })

await buildRuntime({
  entryPoints: [path.join(runtimeRoot, "src", "index.ts")],
  outfile: path.join(stagingRoot, "modules/qq-channel-service/bundle.mjs"),
  bundle: true,
  platform: "node",
  format: "esm",
  target: "node24",
  legalComments: "none",
  nodePaths: runtimeNodePaths,
})

fs.copyFileSync(path.join(runtimeRoot, "launcher.mjs"), path.join(stagingRoot, "modules/qq-channel-service/launcher.mjs"))
fs.writeFileSync(path.join(stagingRoot, "modules/qq-channel-service/package.json"), `${JSON.stringify({ type: "module" }, null, 2)}\n`)
fs.copyFileSync(path.join(uiRoot, "mobile", "schema.json"), path.join(stagingRoot, "modules/qq-channel-ui/mobile/schema.json"))

await build({
  configFile: false,
  root: uiRoot,
  base: "./",
  publicDir: false,
  plugins: [hostVueModulePlugin(dependency("vue/dist/vue.runtime.esm-bundler.js", frontendRequire)), vue()],
  css: {
    postcss: {
      plugins: [clientCssPlugin("qq-page-provider")],
    },
  },
  define: {
    "process.env.NODE_ENV": '"production"',
  },
  build: {
    outDir: path.join(stagingRoot, "modules", "qq-channel-ui", "desktop"),
    emptyOutDir: true,
    lib: {
      entry: path.join(uiRoot, "src/main.mjs"),
      formats: ["es"],
      fileName: () => "index.mjs",
    },
    rollupOptions: {
      output: {
        entryFileNames: "index.mjs",
        chunkFileNames: "chunks/[name]-[hash].js",
        assetFileNames: (asset) => asset.name?.endsWith(".css") ? "styles.css" : "assets/[name]-[hash][extname]",
      },
    },
  },
})

prepareClientUI({ manifest, stagingRoot, moduleId: "qq-channel-ui", contributionId: "qq-page" })
fs.writeFileSync(path.join(stagingRoot, "manifest.json"), `${JSON.stringify(manifest, null, 2)}\n`)
const result = buildChannelPackage(stagingRoot, path.join(projectRoot, "plugin"))
console.log(JSON.stringify(result))

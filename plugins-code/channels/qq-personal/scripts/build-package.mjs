import fs from "node:fs"
import path from "node:path"
import { execFileSync } from "node:child_process"
import { createHash } from "node:crypto"
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

function dependency(name, fallback) {
  try {
    return uiRequire.resolve(name)
  } catch {
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
fs.mkdirSync(path.join(stagingRoot, "modules", "qq-personal-service"), { recursive: true })
fs.mkdirSync(path.join(stagingRoot, "modules", "qq-personal-service", "native"), { recursive: true })
fs.mkdirSync(path.join(stagingRoot, "modules", "qq-personal-ui", "desktop"), { recursive: true })
fs.mkdirSync(path.join(stagingRoot, "modules", "qq-personal-ui", "mobile"), { recursive: true })

await buildRuntime({
  entryPoints: [path.join(runtimeRoot, "src", "index.ts")],
  outfile: path.join(stagingRoot, "modules/qq-personal-service/bundle.mjs"),
  bundle: true,
  platform: "node",
  format: "esm",
  target: "node26",
  legalComments: "none",
  nodePaths: [path.join(runtimeRoot, "node_modules")],
})

fs.copyFileSync(path.join(runtimeRoot, "launcher.mjs"), path.join(stagingRoot, "modules/qq-personal-service/launcher.mjs"))
fs.writeFileSync(path.join(stagingRoot, "modules/qq-personal-service/package.json"), `${JSON.stringify({ type: "module" }, null, 2)}\n`)
execFileSync(process.env.AMITIA_ZIG || "zig", [
  "cc",
  "-target",
  "aarch64-linux-gnu",
  "-Os",
  "-fPIC",
  "-shared",
  "-nostdlib",
  "-Wl,-z,defs",
  "-Wl,-z,noexecstack",
  "-Wl,-soname,libamitia-identity.so",
  "-o",
  path.join(stagingRoot, "modules/qq-personal-service/native/libamitia-identity.so"),
  path.join(root, "native/linux-arm64/current-identity-compat.c"),
], { stdio: "inherit" })
execFileSync(process.env.AMITIA_ZIG || "zig", [
  "cc",
  "-target",
  "aarch64-linux-gnu",
  "-Os",
  "-fPIC",
  "-shared",
  "-Wl,-z,defs",
  "-Wl,-z,noexecstack",
  "-Wl,-soname,libamitia-snowluma-loader.so",
  `-Wl,--version-script=${path.join(root, "native/linux-arm64/snowluma-hook-loader.map")}`,
  "-o",
  path.join(stagingRoot, "modules/qq-personal-service/native/libamitia-snowluma-loader.so"),
  path.join(root, "native/linux-arm64/snowluma-hook-loader.c"),
  "-ldl",
  "-pthread",
], { stdio: "inherit" })
const serviceModule = manifest.modules.find((module) => module.id === "qq-personal-service")
const identityLibrary = path.join(stagingRoot, "modules/qq-personal-service/native/libamitia-identity.so")
const identityHash = createHash("sha256").update(fs.readFileSync(identityLibrary)).digest("hex")
const snowlumaLoader = path.join(stagingRoot, "modules/qq-personal-service/native/libamitia-snowluma-loader.so")
const snowlumaLoaderHash = createHash("sha256").update(fs.readFileSync(snowlumaLoader)).digest("hex")
const procLocksPrelude = path.join(stagingRoot, "modules/qq-personal-service/native/amitia-proc-locks.cjs")
fs.copyFileSync(path.join(root, "native/linux-arm64/amitia-proc-locks.cjs"), procLocksPrelude)
const procLocksPreludeHash = createHash("sha256").update(fs.readFileSync(procLocksPrelude)).digest("hex")
serviceModule.runtime.nativeCompanions = [
  {
    id: "current-identity-compat",
    platform: "linux",
    architecture: "arm64",
    path: "native/libamitia-identity.so",
    sha256: identityHash,
    executable: false,
  },
  {
    id: "snowluma-hook-loader",
    platform: "linux",
    architecture: "arm64",
    path: "native/libamitia-snowluma-loader.so",
    sha256: snowlumaLoaderHash,
    executable: false,
  },
  {
    id: "snowluma-proc-locks",
    platform: "linux",
    architecture: "arm64",
    path: "native/amitia-proc-locks.cjs",
    sha256: procLocksPreludeHash,
    executable: false,
  },
]
fs.copyFileSync(path.join(uiRoot, "mobile", "schema.json"), path.join(stagingRoot, "modules/qq-personal-ui/mobile/schema.json"))

await build({
  configFile: false,
  root: uiRoot,
  base: "./",
  publicDir: false,
  plugins: [hostVueModulePlugin(dependency("vue/dist/vue.runtime.esm-bundler.js", frontendRequire)), vue()],
  css: {
    postcss: {
      plugins: [clientCssPlugin("qq-personal-page-provider")],
    },
  },
  define: { "process.env.NODE_ENV": '"production"' },
  build: {
    outDir: path.join(stagingRoot, "modules", "qq-personal-ui", "desktop"),
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

prepareClientUI({ manifest, stagingRoot, moduleId: "qq-personal-ui", contributionId: "qq-personal-page" })
fs.writeFileSync(path.join(stagingRoot, "manifest.json"), `${JSON.stringify(manifest, null, 2)}\n`)
const result = buildChannelPackage(stagingRoot, path.join(projectRoot, "plugin"))
console.log(JSON.stringify(result))

import { createRequire } from "node:module"
import { existsSync, writeFileSync } from "node:fs"
import path from "node:path"
import { fileURLToPath } from "node:url"

const customRequire = createRequire(import.meta.url)
globalThis.require = customRequire
const runtimeDir = path.dirname(fileURLToPath(import.meta.url))
const packageJsonPath = path.join(runtimeDir, "package.json")
if (!existsSync(packageJsonPath)) {
  writeFileSync(
    packageJsonPath,
    JSON.stringify({
      name: "@tencent-weixin/openclaw-weixin",
      version: "2.4.6",
      ilink_appid: "bot",
    }),
    "utf8",
  )
}
await import("./bundle.mjs")

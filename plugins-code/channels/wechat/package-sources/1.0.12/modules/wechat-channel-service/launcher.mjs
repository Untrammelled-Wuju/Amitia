import { createRequire } from "node:module"
import { writeFileSync } from "node:fs"
import path from "node:path"
import { fileURLToPath } from "node:url"

const customRequire = createRequire(import.meta.url)
globalThis.require = customRequire
const runtimeDir = path.dirname(fileURLToPath(import.meta.url))
writeFileSync(
  path.join(runtimeDir, "package.json"),
  JSON.stringify({
    name: "@tencent-weixin/openclaw-weixin",
    version: "2.4.6",
    ilink_appid: "bot",
  }),
  "utf8",
)
await import("./bundle.mjs")

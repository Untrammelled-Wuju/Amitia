import { createRequire } from "node:module"
const customRequire = createRequire(import.meta.url)
globalThis.require = customRequire
await import("./bundle.mjs")

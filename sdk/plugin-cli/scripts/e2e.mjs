import { spawnSync } from "node:child_process"
import { mkdtempSync } from "node:fs"
import { tmpdir } from "node:os"
import { dirname, join, resolve } from "node:path"
import { fileURLToPath } from "node:url"

const cliRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..")
const cli = join(cliRoot, "dist/bin/amitia-ext.js")
const project = mkdtempSync(join(tmpdir(), "amitia-plugin-e2e-"))
const run = (args, cwd = project, env = process.env) => {
  const result = spawnSync(process.execPath, [cli, ...args], { cwd, env, stdio: "inherit" })
  if (result.error) throw result.error
  if ((result.status ?? 1) !== 0) throw new Error(`amitia-ext ${args[0]} failed with ${result.status}`)
}

run(["init", "--template", "tool", "--extension-id", "com.example/e2e-tool", "--publisher", "example", "--display-name", "E2E Tool", "--output", project])
run(["build"])
run(["validate"])
run(["pack", "--manifest", "manifest.json", "--output", "package.amitiax"])
run(["inspect", "--package", "package.amitiax", "--format", "json"])

const privateKey = process.env.AMITIA_E2E_PRIVATE_KEY
const publicKey = process.env.AMITIA_E2E_PUBLIC_KEY
if (privateKey && publicKey) {
  run(["sign", "--package", "package.amitiax", "--private-key", privateKey, "--publisher-id", "example"])
  run(["verify", "--package", "package.amitiax", "--public-key", publicKey, "--publisher", "example"])
}

const host = process.env.AMITIA_DEV_HOST
const tokenFile = process.env.AMITIA_E2E_TOKEN_FILE
if (host && tokenFile) {
  run(["pack", "--manifest", "manifest.json", "--output", "package-dev.amitiax"])
  const lifecycle = ["--host", host, "--token-file", tokenFile]
  run(["install", "--package", "package-dev.amitiax", "--no-enable", ...lifecycle])
  run(["enable", "com.example/e2e-tool", ...lifecycle])
  run(["disable", "com.example/e2e-tool", ...lifecycle])
  run(["uninstall", "com.example/e2e-tool", ...lifecycle])
}

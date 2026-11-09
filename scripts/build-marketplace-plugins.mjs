import { createHash } from "node:crypto"
import { existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs"
import { tmpdir } from "node:os"
import { basename, dirname, join, resolve } from "node:path"
import { spawnSync } from "node:child_process"
import { fileURLToPath } from "node:url"
import { inspectPackage } from "../sdk/plugin-cli/dist/archive.js"

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..")
const pluginRoot = join(root, "plugin")
const keyPath = resolve(process.env.AMITIA_PLUGIN_SIGNING_KEY || join(root, ".amitia-secrets/plugin-publisher-private.key"))
const publicKeyPath = resolve(process.env.AMITIA_PLUGIN_PUBLIC_KEY || join(root, ".amitia-secrets/plugin-publisher-public.key"))
const goBinary = process.env.GO_BIN || "go"
const nodeBinary = process.execPath
const amitiaExt = join(tmpdir(), `amitia-ext-marketplace-${process.pid}`)
const filter = (process.env.AMITIA_PLUGIN_FILTER || "").trim()
const sourceDateEpoch = process.env.SOURCE_DATE_EPOCH || spawnSync("git", ["show", "-s", "--format=%ct", "HEAD"], { cwd: root, encoding: "utf8" }).stdout.trim()
const signatureCreatedAt = new Date(Number(sourceDateEpoch) * 1000).toISOString()

function run(command, args, cwd = root) {
  const result = spawnSync(command, args, { cwd, stdio: "inherit", env: process.env })
  if (result.error) throw result.error
  if ((result.status ?? 1) !== 0) throw new Error(`${command} exited with ${result.status}`)
}

function readManifest(file) {
  return JSON.parse(readFileSync(file, "utf8"))
}

function packageName(manifest) {
  return `${String(manifest.plugin.id).replaceAll("/", ".")}-${manifest.plugin.version}.amitiax`
}

function buildPlugins() {
  const builds = [
    ["com.amitia/plugin-market", nodeBinary, [join(root, "plugins-builtin/plugin-market/scripts/build-package.mjs"), "--source", join(root, "plugins-builtin/plugin-market"), "--output", join(pluginRoot, "plugin-market")]],
    ["com.amitia/emote", nodeBinary, [join(root, "plugins-code/emote/scripts/build-package.mjs"), "--source", join(root, "plugins-code/emote"), "--output", join(pluginRoot, "Emote")]],
    ["com.amitia/lifestyle", nodeBinary, [join(root, "plugins-code/lifestyle/scripts/build-package.mjs"), "--source", join(root, "plugins-code/lifestyle"), "--output", join(pluginRoot, "lifestyle")]],
    ["com.amitia/runtime-insights", nodeBinary, [join(root, "plugins-builtin/runtime-insights/scripts/build-package.mjs"), "--output", join(pluginRoot, "runtime-insights")]],
    ["com.amitia/workspace", nodeBinary, [join(root, "plugins-builtin/workspace/scripts/build-package.mjs"), "--output", join(pluginRoot, "workspace")]],
    ["com.amitia/chat-import", nodeBinary, [join(root, "plugins-builtin/chat-import/scripts/build-package.mjs"), "--output", join(pluginRoot, "chat-import")]],
    ["com.amitia/reminders", nodeBinary, [join(root, "plugins-builtin/reminders/scripts/build-package.mjs"), "--output", join(pluginRoot, "reminders")]],
    ["com.amitia/storage", nodeBinary, [join(root, "plugins-builtin/storage/scripts/build-package.mjs"), "--output", join(pluginRoot, "storage")]],
    ["com.amitia/chat-logs", nodeBinary, [join(root, "plugins-builtin/chat-logs/scripts/build-package.mjs"), "--output", join(pluginRoot, "chat-logs")]],
    ["com.amitia/memory-hub", nodeBinary, [join(root, "plugins-builtin/memory-hub/scripts/build-package.mjs"), "--output", join(pluginRoot, "memory-hub")]],
    ["com.amitia/character-hub", nodeBinary, [join(root, "plugins-builtin/character-hub/scripts/build-package.mjs"), "--output", join(pluginRoot, "character-hub")]],
    ["com.amitia/creative-workshop-hub", nodeBinary, [join(root, "plugins-builtin/creative-workshop-hub/scripts/build-package.mjs"), "--output", join(pluginRoot, "creative-workshop-hub")]],
    ["com.amitia/game-center-hub", nodeBinary, [join(root, "plugins-builtin/game-center-hub/scripts/build-package.mjs"), "--output", join(pluginRoot, "game-center-hub")]],
    ...["browser-profile", "calendar", "email", "phone", "wallet"].map((name) => {
      const manifest = readManifest(join(root, "plugins-code/agent-identities", name, "amitia-extension.json"))
      return [manifest.plugin.id, nodeBinary, [join(root, "plugins-code/agent-identities/scripts/build-package.mjs"), "--package", join(root, "plugins-code/agent-identities", name)]]
    }),
    ...["qq-clawbot", "qq-personal", "wechat"].map((name) => {
      const manifest = readManifest(join(root, "plugins-code/channels", name, "amitia-extension.json"))
      return [manifest.plugin.id, nodeBinary, [join(root, "plugins-code/channels", name, "scripts/build-package.mjs")]]
    }),
  ]
  for (const [extensionID, command, args] of builds) {
    if (filter && extensionID !== filter) continue
    run(command, args)
  }
}

function expectedPackages() {
  const specs = [
    ["plugins-builtin/plugin-market/amitia-extension.json", "plugin-market"],
    ["plugins-code/emote/amitia-extension.json", "Emote"],
    ["plugins-code/lifestyle/amitia-extension.json", "lifestyle"],
    ["plugins-builtin/runtime-insights/amitia-extension.json", "runtime-insights"],
    ["plugins-builtin/workspace/amitia-extension.json", "workspace"],
    ["plugins-builtin/chat-import/amitia-extension.json", "chat-import"],
    ["plugins-builtin/reminders/amitia-extension.json", "reminders"],
    ["plugins-builtin/storage/amitia-extension.json", "storage"],
    ["plugins-builtin/chat-logs/amitia-extension.json", "chat-logs"],
    ["plugins-builtin/memory-hub/amitia-extension.json", "memory-hub"],
    ["plugins-builtin/character-hub/amitia-extension.json", "character-hub"],
    ["plugins-builtin/creative-workshop-hub/amitia-extension.json", "creative-workshop-hub"],
    ["plugins-builtin/game-center-hub/amitia-extension.json", "game-center-hub"],
    ["plugins-code/agent-identities/browser-profile/amitia-extension.json", "agent-identities"],
    ["plugins-code/agent-identities/calendar/amitia-extension.json", "agent-identities"],
    ["plugins-code/agent-identities/email/amitia-extension.json", "agent-identities"],
    ["plugins-code/agent-identities/phone/amitia-extension.json", "agent-identities"],
    ["plugins-code/agent-identities/wallet/amitia-extension.json", "agent-identities"],
    ["plugins-code/channels/qq-clawbot/amitia-extension.json", ""],
    ["plugins-code/channels/qq-personal/amitia-extension.json", ""],
    ["plugins-code/channels/wechat/amitia-extension.json", ""],
  ]
  return specs.map(([manifestPath, directory]) => {
    const manifest = readManifest(join(root, manifestPath))
    const name = String(manifest.plugin.id).replaceAll("/", ".")
    const fileName = directory === "Emote"
      ? `amitia-emote-${manifest.plugin.version}.amitiax`
      : directory === "plugin-market"
        ? `amitia-plugin-market-${manifest.plugin.version}.amitiax`
      : directory === "lifestyle"
        ? `amitia-lifestyle-${manifest.plugin.version}.amitiax`
      : directory === "runtime-insights"
        ? `amitia-runtime-insights-${manifest.plugin.version}.amitiax`
      : directory === "workspace"
        ? `amitia-workspace-${manifest.plugin.version}.amitiax`
      : directory === "chat-import"
        ? `amitia-chat-import-${manifest.plugin.version}.amitiax`
      : directory === "reminders"
        ? `amitia-reminders-${manifest.plugin.version}.amitiax`
      : directory === "storage"
        ? `amitia-storage-${manifest.plugin.version}.amitiax`
      : directory === "chat-logs"
        ? `amitia-chat-logs-${manifest.plugin.version}.amitiax`
      : directory === "memory-hub"
        ? `amitia-memory-hub-${manifest.plugin.version}.amitiax`
      : directory === "character-hub"
        ? `amitia-character-hub-${manifest.plugin.version}.amitiax`
      : directory === "creative-workshop-hub"
        ? `amitia-creative-workshop-hub-${manifest.plugin.version}.amitiax`
      : directory === "game-center-hub"
        ? `amitia-game-center-hub-${manifest.plugin.version}.amitiax`
        : directory === "agent-identities"
          ? `${basename(dirname(join(root, manifestPath)))}-${manifest.plugin.version}.amitiax`
          : packageName(manifest)
    return {
      manifest,
      path: directory ? join(pluginRoot, directory, fileName) : join(pluginRoot, fileName),
    }
  }).filter((item) => !filter || item.manifest.plugin.id === filter)
}

function writeSBOM(packagePath, manifest, treeHash) {
  const sbom = {
    spdxVersion: "SPDX-2.3",
    dataLicense: "CC0-1.0",
    SPDXID: "SPDXRef-DOCUMENT",
    name: `${manifest.plugin.id}@${manifest.plugin.version}`,
    documentNamespace: `https://amitia.untrammelled.top/sbom/${encodeURIComponent(manifest.plugin.id)}/${manifest.plugin.version}/${treeHash}`,
    creationInfo: {
      created: new Date().toISOString(),
      creators: ["Tool: Amitia marketplace plugin builder"],
    },
    packages: [{
      SPDXID: "SPDXRef-Package",
      name: manifest.plugin.id,
      versionInfo: manifest.plugin.version,
      downloadLocation: "NOASSERTION",
      filesAnalyzed: true,
      licenseConcluded: manifest.plugin.license || "NOASSERTION",
      licenseDeclared: manifest.plugin.license || "NOASSERTION",
      checksums: [
        { algorithm: "SHA256", checksumValue: treeHash },
      ],
    }],
    relationships: [{
      spdxElementId: "SPDXRef-DOCUMENT",
      relationshipType: "DESCRIBES",
      relatedSpdxElement: "SPDXRef-Package",
    }],
  }
  const output = `${packagePath}.sbom.json`
  writeFileSync(output, `${JSON.stringify(sbom, null, 2)}\n`)
  return output
}

function main() {
  if (!existsSync(keyPath)) throw new Error(`plugin signing key not found: ${keyPath}`)
  if (!existsSync(publicKeyPath)) throw new Error(`plugin public key not found: ${publicKeyPath}`)
  if (process.argv.includes("--build")) buildPlugins()
  mkdirSync(dirname(amitiaExt), { recursive: true })
  run(goBinary, ["build", "-o", amitiaExt, "./cmd/amitia-ext"], join(root, "backend"))
  const output = []
  try {
    for (const item of expectedPackages()) {
      if (!existsSync(item.path)) throw new Error(`built plugin package not found: ${item.path}`)
      const inspection = inspectPackage(item.path)
      const sbom = writeSBOM(item.path, inspection.manifest, inspection.treeHash)
      run(amitiaExt, ["sign", item.path, "--key", keyPath, "--publisher", "com.amitia", "--channel", "stable", "--created-at", signatureCreatedAt])
      run(amitiaExt, ["verify", item.path, "--pubkey", publicKeyPath, "--publisher", "com.amitia"])
      output.push({
        extensionId: inspection.manifest.plugin.id,
        version: inspection.manifest.plugin.version,
        packagePath: item.path,
        sbomPath: sbom,
        treeHash: inspection.treeHash,
        sha256: createHash("sha256").update(readFileSync(item.path)).digest("hex"),
        platforms: inspection.manifest.compatibility?.platforms || [],
      })
    }
  } finally {
    rmSync(amitiaExt, { force: true })
  }
  console.log(JSON.stringify(output, null, 2))
}

main()

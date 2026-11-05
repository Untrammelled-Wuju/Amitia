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

function run(command, args, cwd = root) {
  const result = spawnSync(command, args, { cwd, stdio: "inherit", env: process.env })
  if (result.error) throw result.error
  if ((result.status ?? 1) !== 0) throw new Error(`${command} exited with ${result.status}`)
}

function readManifest(file) {
  return JSON.parse(readFileSync(file, "utf8"))
}

function packageName(manifest) {
  return `${String(manifest.extension.id).replaceAll("/", ".")}-${manifest.extension.version}.amitiax`
}

function buildPlugins() {
  const builds = [
    ["com.amitia/plugin-market", nodeBinary, [join(root, "plugins-code/plugin-market/scripts/build-package.mjs"), "--source", join(root, "plugins-code/plugin-market"), "--output", join(pluginRoot, "plugin-market")]],
    ["com.amitia/emote", nodeBinary, [join(root, "plugins-code/emote/scripts/build-package.mjs"), "--source", join(root, "plugins-code/emote"), "--output", join(pluginRoot, "Emote")]],
    ["com.amitia/lifestyle", nodeBinary, [join(root, "plugins-code/lifestyle/scripts/build-package.mjs"), "--source", join(root, "plugins-code/lifestyle"), "--output", join(pluginRoot, "lifestyle")]],
    ...["browser-profile", "calendar", "email", "phone", "wallet"].map((name) => {
      const manifest = readManifest(join(root, "plugins-code/agent-identities", name, "amitia-extension.json"))
      return [manifest.extension.id, nodeBinary, [join(root, "plugins-code/agent-identities/scripts/build-package.mjs"), "--package", join(root, "plugins-code/agent-identities", name)]]
    }),
    ...["qq-clawbot", "qq-personal", "wechat"].map((name) => {
      const manifest = readManifest(join(root, "plugins-code/channels", name, "amitia-extension.json"))
      return [manifest.extension.id, nodeBinary, [join(root, "plugins-code/channels", name, "scripts/build-package.mjs")]]
    }),
  ]
  for (const [extensionID, command, args] of builds) {
    if (filter && extensionID !== filter) continue
    run(command, args)
  }
}

function expectedPackages() {
  const specs = [
    ["plugins-code/plugin-market/amitia-extension.json", "plugin-market"],
    ["plugins-code/emote/amitia-extension.json", "Emote"],
    ["plugins-code/lifestyle/amitia-extension.json", "lifestyle"],
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
    const name = String(manifest.extension.id).replaceAll("/", ".")
    const fileName = directory === "Emote"
      ? `amitia-emote-${manifest.extension.version}.amitiax`
      : directory === "plugin-market"
        ? `amitia-plugin-market-${manifest.extension.version}.amitiax`
      : directory === "lifestyle"
        ? `amitia-lifestyle-${manifest.extension.version}.amitiax`
        : directory === "agent-identities"
          ? `${basename(dirname(join(root, manifestPath)))}-${manifest.extension.version}.amitiax`
          : packageName(manifest)
    return {
      manifest,
      path: directory ? join(pluginRoot, directory, fileName) : join(pluginRoot, fileName),
    }
  }).filter((item) => !filter || item.manifest.extension.id === filter)
}

function writeSBOM(packagePath, manifest, treeHash) {
  const sbom = {
    spdxVersion: "SPDX-2.3",
    dataLicense: "CC0-1.0",
    SPDXID: "SPDXRef-DOCUMENT",
    name: `${manifest.extension.id}@${manifest.extension.version}`,
    documentNamespace: `https://amitia.untrammelled.top/sbom/${encodeURIComponent(manifest.extension.id)}/${manifest.extension.version}/${treeHash}`,
    creationInfo: {
      created: new Date().toISOString(),
      creators: ["Tool: Amitia marketplace plugin builder"],
    },
    packages: [{
      SPDXID: "SPDXRef-Package",
      name: manifest.extension.id,
      versionInfo: manifest.extension.version,
      downloadLocation: "NOASSERTION",
      filesAnalyzed: true,
      licenseConcluded: manifest.extension.license || "NOASSERTION",
      licenseDeclared: manifest.extension.license || "NOASSERTION",
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
      run(amitiaExt, ["sign", item.path, "--key", keyPath, "--publisher", "com.amitia", "--channel", "stable"])
      run(amitiaExt, ["verify", item.path, "--pubkey", publicKeyPath, "--publisher", "com.amitia"])
      output.push({
        extensionId: inspection.manifest.extension.id,
        version: inspection.manifest.extension.version,
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

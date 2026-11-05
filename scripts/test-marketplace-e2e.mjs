import { createHash, randomUUID } from "node:crypto"
import { mkdirSync, readFileSync, writeFileSync } from "node:fs"
import { dirname, join, resolve } from "node:path"
import { fileURLToPath } from "node:url"

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..")
const catalogBase = (process.env.AMITIA_MARKET_CATALOG_URL || "https://amitia.untrammelled.top/api/v1/market/catalog").replace(/\/+$/, "")
const platform = process.env.AMITIA_TEST_PLATFORM || "macos"
const unsupportedProbePlatforms = (process.env.AMITIA_UNSUPPORTED_PROBE_PLATFORMS || "android,ios")
  .split(",")
  .map((item) => item.trim())
  .filter(Boolean)
const endpoint = JSON.parse(readFileSync(join(root, "AmitiaData/run/endpoint.json"), "utf8"))
const coreBase = `http://127.0.0.1:${endpoint.port}`
const localToken = readFileSync(join(root, "AmitiaData/security/local-token"), "utf8").trim()

async function readResponse(response) {
  const text = await response.text()
  let payload = null
  if (text) payload = JSON.parse(text)
  if (!response.ok) throw new Error(`HTTP ${response.status}: ${text}`)
  return payload
}

function coreHeaders(contentType = "application/json") {
  return {
    "X-Amitia-Local-Token": localToken,
    Accept: "application/json",
    ...(contentType ? { "Content-Type": contentType } : {}),
  }
}

async function coreRequest(path, init = {}) {
  return readResponse(await fetch(`${coreBase}${path}`, {
    ...init,
    headers: { ...coreHeaders(init.body instanceof FormData ? "" : "application/json"), ...(init.headers || {}) },
  }))
}

async function installedVersions() {
  const view = await coreRequest("/api/extension-center/view")
  const cards = [
    ...(view.installed || []),
    ...(view.updates || []),
    ...(view.needsAction || []),
  ]
  return new Map(cards.map((card) => [card.extensionId, card.version]))
}

function normalizePlatform(value) {
  const normalized = String(value || "").trim().toLowerCase()
  if (normalized === "win" || normalized === "win32" || normalized === "windows") return "windows"
  if (normalized === "mac" || normalized === "darwin" || normalized === "macos") return "macos"
  if (normalized === "linux") return "linux"
  if (normalized === "android") return "android"
  if (normalized === "ios") return "ios"
  return normalized
}

function supportsPlatform(entry, targetPlatform) {
  const target = normalizePlatform(targetPlatform)
  return (entry.compatibility?.platforms || []).some((item) => normalizePlatform(item) === target)
}

async function downloadAndVerify(entry) {
  const response = await fetch(entry.package.uri, { headers: { Accept: "application/vnd.amitia.plugin+zip" } })
  if (!response.ok) throw new Error(`package download HTTP ${response.status}: ${await response.text()}`)
  const bytes = Buffer.from(await response.arrayBuffer())
  const hash = createHash("sha256").update(bytes).digest("hex")
  if (hash !== entry.package.sha256) throw new Error(`package hash mismatch for ${entry.extensionId}: ${hash}`)
  return { bytes, hash }
}

async function waitOperation(operationId) {
  let operation = null
  for (let attempt = 0; attempt < 240; attempt += 1) {
    operation = await coreRequest(`/api/extensions/packages/operations/${encodeURIComponent(operationId)}`)
    if (operation.status === "completed") return operation
    if (operation.status === "failed" || operation.status === "requires_recovery") {
      throw new Error(JSON.stringify(operation))
    }
    await new Promise((resolvePromise) => setTimeout(resolvePromise, 500))
  }
  throw new Error(`operation timeout: ${operationId}: ${JSON.stringify(operation)}`)
}

async function installEntry(entry, currentVersion) {
  const downloaded = await downloadAndVerify(entry)
  if (currentVersion === entry.version) {
    let enabled = null
    try {
      enabled = await coreRequest("/api/extensions/kernel/extensions/enable", {
        method: "POST",
        body: JSON.stringify({ id: entry.extensionId }),
      })
    } catch (cause) {
      enabled = { error: cause instanceof Error ? cause.message : String(cause) }
    }
    return {
      extensionId: entry.extensionId,
      version: entry.version,
      operationType: "current",
      status: "completed",
      enabled,
      sha256: downloaded.hash,
      downloadVerified: true,
      platforms: entry.compatibility.platforms,
    }
  }
  const form = new FormData()
  form.append("file", new Blob([downloaded.bytes], { type: entry.package.mediaType || "application/vnd.amitia.plugin+zip" }), entry.package.fileName || "plugin.amitiax")
  form.append("scopeType", "global")
  form.append("expectedExtensionId", entry.extensionId)
  const artifact = await coreRequest("/api/extensions/packages/artifacts", { method: "POST", body: form })
  const preview = artifact.preview
  if (!preview?.compatible) throw new Error(`preview rejected ${entry.extensionId}: ${JSON.stringify(preview)}`)
  const confirmations = Object.fromEntries((preview.requiredConfirmations || []).map((item) => [item, true]))
  for (const item of preview.capabilityConfirmations || []) confirmations[item] = true
  for (const item of [
    "confirm.permission_escalation",
    "confirm.scripts",
    "confirm.version_change",
    "confirm.signer_change",
    "confirm.config_migration",
  ]) confirmations[item] = true
  const confirmation = await coreRequest(`/api/extensions/packages/previews/${encodeURIComponent(preview.sessionId)}/confirm`, {
    method: "POST",
    body: JSON.stringify({ scopeType: "global", confirmations }),
  })
  const operationType = currentVersion ? "update" : "install"
  const idempotencyKey = `marketplace-${operationType}-${entry.extensionId}-${randomUUID()}`
  const operation = await coreRequest(`/api/extensions/packages/operations/${operationType}`, {
    method: "POST",
    headers: { "Idempotency-Key": idempotencyKey },
    body: JSON.stringify({
      sessionId: preview.sessionId,
      scopeType: "global",
      confirmationToken: confirmation.confirmationToken,
      expectedExtensionId: currentVersion ? entry.extensionId : undefined,
      idempotencyKey,
    }),
  })
  const completed = await waitOperation(operation.operationId)
  let enabled = null
  try {
    enabled = await coreRequest("/api/extensions/kernel/extensions/enable", {
      method: "POST",
      body: JSON.stringify({ id: entry.extensionId }),
    })
  } catch (cause) {
    enabled = { error: cause instanceof Error ? cause.message : String(cause) }
  }
  return {
    extensionId: entry.extensionId,
    version: entry.version,
    operationType,
    operationId: operation.operationId,
    status: completed.status,
    enabled,
    sha256: downloaded.hash,
    downloadVerified: true,
    platforms: entry.compatibility.platforms,
  }
}

async function verifyPlatformPackageAccess(probePlatform) {
  const response = await readResponse(await fetch(`${catalogBase}?platform=${encodeURIComponent(probePlatform)}`))
  const catalog = response.data
  const downloads = []
  const denied = []
  for (const entry of catalog.entries) {
    const download = await fetch(entry.package.uri, { headers: { Accept: "application/vnd.amitia.plugin+zip" } })
    if (supportsPlatform(entry, probePlatform)) {
      if (download.status !== 200) throw new Error(`supported package download failed: ${entry.extensionId} on ${probePlatform}: HTTP ${download.status}`)
      const bytes = Buffer.from(await download.arrayBuffer())
      const hash = createHash("sha256").update(bytes).digest("hex")
      if (hash !== entry.package.sha256) throw new Error(`package hash mismatch for ${entry.extensionId} on ${probePlatform}: ${hash}`)
      downloads.push({
        extensionId: entry.extensionId,
        version: entry.version,
        platform: probePlatform,
        status: download.status,
        sha256: hash,
        size: bytes.length,
        platforms: entry.compatibility.platforms,
      })
      continue
    }
    const body = await download.text()
    if (download.status === 200) throw new Error(`unsupported package download was allowed: ${entry.extensionId} on ${probePlatform}`)
    denied.push({
      extensionId: entry.extensionId,
      version: entry.version,
      platform: probePlatform,
      status: download.status,
      body,
      platforms: entry.compatibility.platforms,
    })
  }
  if (denied.length === 0) throw new Error(`catalog has no unsupported ${probePlatform} entry to probe`)
  return { platform: probePlatform, downloads, denied }
}

async function main() {
  const catalogResponse = await readResponse(await fetch(`${catalogBase}?platform=${encodeURIComponent(platform)}`))
  const catalog = catalogResponse.data
  if (catalog.schemaVersion !== "amitia.plugin.catalog/v1") throw new Error(`unexpected catalog schema: ${catalog.schemaVersion}`)
  const current = await installedVersions()
  const report = []
  for (const entry of catalog.entries) {
    if (!supportsPlatform(entry, platform)) {
      report.push({
        extensionId: entry.extensionId,
        version: entry.version,
        skipped: true,
        reason: `unsupported on ${platform}`,
        platforms: entry.compatibility.platforms,
        packageUri: entry.package.uri,
      })
      continue
    }
    report.push(await installEntry(entry, current.get(entry.extensionId)))
  }
  const platformPackageAccess = []
  for (const probePlatform of unsupportedProbePlatforms) {
    platformPackageAccess.push(await verifyPlatformPackageAccess(probePlatform))
  }
  const unsupportedDownloads = platformPackageAccess.flatMap((item) => item.denied)
  const outputDir = join(root, "artifacts/marketplace-e2e")
  mkdirSync(outputDir, { recursive: true })
  const output = join(outputDir, `report-${platform}-${Date.now()}.json`)
  writeFileSync(output, `${JSON.stringify({ generatedAt: new Date().toISOString(), platform, catalogId: catalog.id, report, platformPackageAccess, unsupportedDownloads }, null, 2)}\n`)
  console.log(JSON.stringify({ output, report, platformPackageAccess, unsupportedDownloads }, null, 2))
}

main()

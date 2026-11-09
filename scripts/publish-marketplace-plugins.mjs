import { existsSync, readFileSync } from "node:fs"
import { basename, dirname, join, resolve } from "node:path"
import { inflateRawSync } from "node:zlib"
import { fileURLToPath } from "node:url"

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..")
const baseURL = (process.env.AMITIA_ADMIN_URL || "https://amitia.untrammelled.top").replace(/\/+$/, "")
const username = process.env.AMITIA_ADMIN_USERNAME || ""
const password = process.env.AMITIA_ADMIN_PASSWORD || ""

function entries(buffer) {
  let end = -1
  for (let index = buffer.length - 22; index >= 0; index -= 1) {
    if (buffer.readUInt32LE(index) === 0x06054b50) {
      end = index
      break
    }
  }
  if (end < 0) throw new Error("ZIP end record not found")
  const count = buffer.readUInt16LE(end + 10)
  let cursor = buffer.readUInt32LE(end + 16)
  const result = new Map()
  for (let index = 0; index < count; index += 1) {
    const method = buffer.readUInt16LE(cursor + 10)
    const compressedSize = buffer.readUInt32LE(cursor + 20)
    const nameLength = buffer.readUInt16LE(cursor + 28)
    const extraLength = buffer.readUInt16LE(cursor + 30)
    const commentLength = buffer.readUInt16LE(cursor + 32)
    const localOffset = buffer.readUInt32LE(cursor + 42)
    const name = buffer.subarray(cursor + 46, cursor + 46 + nameLength).toString("utf8")
    const localNameLength = buffer.readUInt16LE(localOffset + 26)
    const localExtraLength = buffer.readUInt16LE(localOffset + 28)
    const start = localOffset + 30 + localNameLength + localExtraLength
    const compressed = buffer.subarray(start, start + compressedSize)
    result.set(name, method === 8 ? inflateRawSync(compressed) : compressed)
    cursor += 46 + nameLength + extraLength + commentLength
  }
  return result
}

async function request(path, { method = "GET", body, cookie, csrf } = {}) {
  const headers = {}
  if (body && !(body instanceof FormData)) headers["Content-Type"] = "application/json"
  if (cookie) headers.Cookie = cookie
  if (csrf) headers["X-CSRF-Token"] = csrf
  const response = await fetch(`${baseURL}${path}`, {
    method,
    headers,
    body: body instanceof FormData ? body : body ? JSON.stringify(body) : undefined,
  })
  const text = await response.text()
  let payload = null
  if (text) payload = JSON.parse(text)
  if (!response.ok || (payload && payload.code && payload.code !== 200)) {
    throw new Error(`HTTP ${response.status}: ${text}`)
  }
  return { payload, cookie: response.headers.get("set-cookie") || cookie }
}

function packagePaths() {
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
    const manifest = JSON.parse(readFileSync(join(root, manifestPath), "utf8"))
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
          : `${name}-${manifest.plugin.version}.amitiax`
    return directory ? join(root, "plugin", directory, fileName) : join(root, "plugin", fileName)
  })
}

function categoryFor(extensionID) {
  if (extensionID.startsWith("com.amitia/agent-identity-")) return "agent-identities"
  if (extensionID.startsWith("com.amitia/channel-")) return "channels"
  if (extensionID === "com.amitia/lifestyle") return "lifestyle"
  if (extensionID === "com.amitia/plugin-market") return "developer"
  if (extensionID === "com.amitia/runtime-insights") return "developer"
  return "productivity"
}

async function main() {
  if (!username || !password) throw new Error("AMITIA_ADMIN_USERNAME and AMITIA_ADMIN_PASSWORD are required")
  const login = await request("/api/v1/admin/auth/login", {
    method: "POST",
    body: { username, password },
  })
  const cookie = login.cookie
  const csrf = login.payload?.data?.csrfToken
  if (!cookie || !csrf) throw new Error("admin login did not return session cookie and CSRF token")
  const categoryResponse = await request("/api/v1/admin/marketplace/categories", { cookie, csrf })
  const categories = new Map(categoryResponse.payload.data.map((item) => [item.slug, item]))
  const results = []
  const filter = (process.env.AMITIA_PLUGIN_FILTER || "").trim()
  for (const path of packagePaths()) {
    if (!existsSync(path)) throw new Error(`package not found: ${path}`)
    const sbomPath = `${path}.sbom.json`
    if (!existsSync(sbomPath)) throw new Error(`SBOM not found: ${sbomPath}`)
    const data = readFileSync(path)
    const files = entries(data)
    const manifest = JSON.parse(files.get("manifest.json").toString("utf8"))
    const extensionID = manifest.plugin.id
    if (filter && extensionID !== filter) continue
    const category = categories.get(categoryFor(extensionID))
    if (!category) throw new Error(`marketplace category not found: ${categoryFor(extensionID)}`)
    const form = new FormData()
    form.append("package", new Blob([data], { type: "application/vnd.amitia.plugin+zip" }), basename(path))
    form.append("sbom", new Blob([readFileSync(sbomPath)], { type: "application/json" }), basename(sbomPath))
    form.append("categoryId", String(category.id))
    form.append("expectedExtensionId", extensionID)
      form.append("releaseNotes", process.env.AMITIA_PLUGIN_RELEASE_NOTES || `Amitia plugin marketplace release ${manifest.plugin.version}`)
    const response = await request("/api/v1/admin/marketplace/publish", {
      method: "POST",
      cookie,
      csrf,
      body: form,
    })
    results.push({
      extensionId: extensionID,
      version: manifest.plugin.version,
      category: category.slug,
      status: response.payload?.data?.status,
    })
  }
  console.log(JSON.stringify(results, null, 2))
}

main()

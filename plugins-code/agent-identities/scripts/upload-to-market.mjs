import { existsSync, readFileSync, readdirSync } from "node:fs";
import { basename, join, resolve } from "node:path";
import { inflateRawSync } from "node:zlib";

function entries(buffer) {
  let end = -1;
  for (let index = buffer.length - 22; index >= 0; index -= 1) {
    if (buffer.readUInt32LE(index) === 0x06054b50) {
      end = index;
      break;
    }
  }
  if (end < 0) throw new Error("ZIP end record not found");
  const count = buffer.readUInt16LE(end + 10);
  let cursor = buffer.readUInt32LE(end + 16);
  const result = new Map();
  for (let index = 0; index < count; index += 1) {
    const method = buffer.readUInt16LE(cursor + 10);
    const compressedSize = buffer.readUInt32LE(cursor + 20);
    const nameLength = buffer.readUInt16LE(cursor + 28);
    const extraLength = buffer.readUInt16LE(cursor + 30);
    const commentLength = buffer.readUInt16LE(cursor + 32);
    const localOffset = buffer.readUInt32LE(cursor + 42);
    const name = buffer.subarray(cursor + 46, cursor + 46 + nameLength).toString("utf8");
    const localNameLength = buffer.readUInt16LE(localOffset + 26);
    const localExtraLength = buffer.readUInt16LE(localOffset + 28);
    const start = localOffset + 30 + localNameLength + localExtraLength;
    const compressed = buffer.subarray(start, start + compressedSize);
    result.set(name, method === 8 ? inflateRawSync(compressed) : compressed);
    cursor += 46 + nameLength + extraLength + commentLength;
  }
  return result;
}

async function request(path, { method = "GET", body, cookie, csrf } = {}) {
  const base = (process.env.AMITIA_ADMIN_URL || "http://127.0.0.1:18880").replace(/\/+$/, "");
  const headers = {};
  if (body && !(body instanceof FormData)) headers["Content-Type"] = "application/json";
  if (cookie) headers.Cookie = cookie;
  if (csrf) headers["X-CSRF-Token"] = csrf;
  const response = await fetch(`${base}${path}`, {
    method,
    headers,
    body: body instanceof FormData ? body : body ? JSON.stringify(body) : undefined,
  });
  const text = await response.text();
  let payload = null;
  if (text) payload = JSON.parse(text);
  if (!response.ok || (payload && payload.code && payload.code !== 200)) {
    throw new Error(`HTTP ${response.status}: ${text}`);
  }
  return { payload, cookie: response.headers.get("set-cookie") || cookie };
}

const root = resolve(process.argv[2] || "plugin/agent-identities");
const username = process.env.AMITIA_ADMIN_USERNAME || "";
const password = process.env.AMITIA_ADMIN_PASSWORD || "";
if (!username || !password) throw new Error("AMITIA_ADMIN_USERNAME and AMITIA_ADMIN_PASSWORD are required");
const login = await request("/api/v1/admin/auth/login", {
  method: "POST",
  body: { username, password },
});
const cookie = login.cookie;
const csrf = login.payload?.data?.csrfToken;
if (!cookie || !csrf) throw new Error("admin login did not return session cookie and CSRF token");
const categories = await request("/api/v1/admin/marketplace/categories", { cookie, csrf });
const category = categories.payload.data.find((item) => item.slug === "agent-identities");
if (!category) throw new Error("agent-identities category is missing");
for (const name of readdirSync(root).filter((item) => item.endsWith(".amitiax")).sort()) {
  const path = join(root, name);
  const data = readFileSync(path);
  const files = entries(data);
  const manifest = JSON.parse(files.get("manifest.json").toString("utf8"));
  const extensionId = manifest.plugin.id;
  const version = manifest.plugin.version;
  const sbomPath = `${path}.sbom.json`;
  if (!existsSync(sbomPath)) throw new Error(`SBOM is required: ${sbomPath}`);
  const form = new FormData();
  form.append("package", new Blob([data], { type: "application/octet-stream" }), basename(path));
  form.append("sbom", new Blob([readFileSync(sbomPath)], { type: "application/json" }), basename(sbomPath));
  form.append("categoryId", String(category.id));
  form.append("expectedExtensionId", extensionId);
  form.append("releaseNotes", "Agent identity plugin release");
  const embeddedSignature = files.get("META-INF/amitia-signature.json")?.toString("utf8") || "";
  form.append("signature", process.env.AMITIA_PLUGIN_SIGNATURE || embeddedSignature);
  form.append("compatibilityJson", JSON.stringify(manifest.compatibility || {}));
  await request("/api/v1/admin/marketplace/publish", {
    method: "POST",
    cookie,
    csrf,
    body: form,
  });
  console.log(`published=${extensionId}@${version}`);
}

import { readFileSync } from "node:fs";
import { basename, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const packageRoot = resolve(fileURLToPath(new URL("..", import.meta.url)));
const repositoryRoot = resolve(packageRoot, "..", "..");

function argument(name, fallback) {
  const index = process.argv.indexOf(`--${name}`);
  if (index < 0) return fallback;
  const value = process.argv[index + 1];
  if (!value) throw new Error(`--${name} 缺少取值`);
  return value;
}

const base = argument("base", process.env.AMITIA_CORE_BASE || "http://127.0.0.1:60024").replace(/\/+$/, "");
const tokenFile = argument("token-file", resolve(repositoryRoot, "AmitiaData", "security", "local-token"));
const archiveArg = argument("package", process.argv[2] && !process.argv[2].startsWith("--") ? process.argv[2] : "");
if (!archiveArg) throw new Error("必须通过 --package 指定 .amitiax 归档路径");
const archivePath = resolve(archiveArg);
const extensionId = argument("extension-id", "com.amitia/dsh-compatible-host");
const scopeType = argument("scope-type", "global");
const scopeId = argument("scope-id", "");
const timeoutMs = Number(argument("timeout-ms", "180000"));

const token = readFileSync(tokenFile, "utf8").trim();
if (!token) throw new Error(`本地凭据文件为空: ${tokenFile}`);

async function request(method, path, body, headers = {}) {
  const response = await fetch(`${base}${path}`, {
    method,
    headers: { "X-Amitia-Local-Token": token, ...headers },
    body,
  });
  const raw = await response.text();
  let payload;
  try {
    payload = raw ? JSON.parse(raw) : null;
  } catch (cause) {
    throw new Error(`${method} ${path} 返回非 JSON 响应: status=${response.status} body=${raw}`, { cause });
  }
  if (!response.ok) {
    throw new Error(`${method} ${path} 失败: status=${response.status} body=${raw}`);
  }
  return payload;
}

const archive = readFileSync(archivePath);
const form = new FormData();
form.set("scopeType", scopeType);
form.set("scopeId", scopeId);
form.set("file", new Blob([archive]), basename(archivePath));
const created = await request("POST", "/api/extensions/packages/artifacts", form);
const preview = created?.preview;
if (!preview?.sessionId) throw new Error(`制品预览缺少 sessionId: ${JSON.stringify(created)}`);

const confirmations = Object.fromEntries((preview.capabilityConfirmations ?? []).map((key) => [key, true]));
const confirmation = await request(
  "POST",
  `/api/extensions/packages/previews/${encodeURIComponent(preview.sessionId)}/confirm`,
  JSON.stringify({ scopeType, scopeId, confirmations }),
  { "Content-Type": "application/json" },
);
const confirmationToken = confirmation?.confirmationToken;
if (!confirmationToken) throw new Error(`预览确认缺少 confirmationToken: ${JSON.stringify(confirmation)}`);

const installed = await request("GET", `/api/extension-center/extension?id=${encodeURIComponent(extensionId)}`);
const installedVersion = installed?.extension?.version ?? installed?.version ?? "";
const action = installedVersion ? "update" : "install";
const operation = await request(
  "POST",
  `/api/extensions/packages/operations/${action}`,
  JSON.stringify({
    sessionId: preview.sessionId,
    scopeType,
    scopeId,
    confirmationToken,
    expectedExtensionId: preview.extensionId ?? extensionId,
    idempotencyKey: `${extensionId}-${action}-${Date.now()}`,
  }),
  { "Content-Type": "application/json" },
);
const operationId = operation?.operationId;
if (!operationId) throw new Error(`操作缺少 operationId: ${JSON.stringify(operation)}`);

const deadline = Date.now() + timeoutMs;
let latest = null;
while (Date.now() < deadline) {
  latest = await request("GET", `/api/extensions/packages/operations/${encodeURIComponent(operationId)}`);
  if (latest?.status === "completed") break;
  if (latest?.status === "failed" || latest?.status === "cancelled") {
    throw new Error(`安装操作未完成: ${JSON.stringify(latest)}`);
  }
  await new Promise((resolveDelay) => setTimeout(resolveDelay, 1000));
}
if (latest?.status !== "completed") throw new Error(`安装操作超时: ${JSON.stringify(latest)}`);

const status = await request("GET", `/api/extension-center/extension?id=${encodeURIComponent(extensionId)}`);
console.log(JSON.stringify({
  base,
  action,
  previousVersion: installedVersion || null,
  operationId,
  operationStatus: latest.status,
  extension: status?.extension ?? status,
  generationId: latest.generationId ?? latest.result?.generationId ?? null,
}, null, 2));

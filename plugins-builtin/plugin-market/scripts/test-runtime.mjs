import { pathToFileURL } from "node:url";
import { resolve } from "node:path";

let extension = null;
globalThis.defineExtension = (value) => {
  extension = value;
};

const runtimePath = resolve(import.meta.dirname, "..", "src", "runtime", "index.mjs");
await import(`${pathToFileURL(runtimePath).href}?test=${Date.now()}`);
if (!extension) throw new Error("plugin market extension was not registered");

const handlers = new Map();
const calls = [];
const host = {
  async call(method, input) {
    calls.push({ method, input });
    if (method === "host.plugin.list") {
      return {
        plugins: [
          { extensionId: "com.example/current", version: "1.0.0", installationState: "installed", enabled: true, systemManaged: false },
          { extensionId: "com.example/update", version: "1.0.0", installationState: "installed", enabled: true, systemManaged: false },
        ],
      };
    }
    if (method === "host.network.request") {
      return {
        status: 200,
        body: {
          code: 200,
          data: {
            schemaVersion: "amitia.plugin.catalog/v1",
            metadata: { categories: [{ slug: "developer", name: "开发工具" }] },
            entries: [
              {
                extensionId: "com.example/current",
                version: "1.0.0",
                name: { default: "当前插件" },
                description: { default: "当前版本" },
                summary: "当前版本",
                package: { uri: "https://example.com/current.amitiax", sha256: "a".repeat(64), fileName: "current.amitiax", sizeBytes: 1 },
                publisher: { id: "com.example", displayName: "Example" },
                signature: { status: "verified" },
                trustLevel: "official",
                categories: ["developer"],
                compatibility: { platforms: ["windows", "macos"] },
                releaseNotes: "",
                publishedAt: "2026-11-04T00:00:00Z",
              },
              {
                extensionId: "com.example/update",
                version: "2.0.0",
                name: { default: "更新插件" },
                description: { default: "存在更新" },
                summary: "存在更新",
                package: { uri: "https://example.com/update.amitiax", sha256: "b".repeat(64), fileName: "update.amitiax", sizeBytes: 2 },
                publisher: { id: "com.example", displayName: "Example" },
                signature: { status: "verified" },
                trustLevel: "official",
                categories: ["developer"],
                compatibility: { platforms: ["windows", "macos"] },
                releaseNotes: "update",
                publishedAt: "2026-11-04T00:00:00Z",
              },
              {
                extensionId: "com.example/new",
                version: "1.0.0",
                name: { default: "新插件" },
                description: { default: "可安装" },
                summary: "可安装",
                package: { uri: "https://example.com/new.amitiax", sha256: "c".repeat(64), fileName: "new.amitiax", sizeBytes: 3 },
                publisher: { id: "com.example", displayName: "Example" },
                signature: { status: "verified" },
                trustLevel: "official",
                categories: ["developer"],
                compatibility: { platforms: ["windows", "macos"] },
                releaseNotes: "",
                publishedAt: "2026-11-04T00:00:00Z",
              },
            ],
          },
        },
      };
    }
    if (method === "host.plugin.preview") {
      return {
        sessionId: "preview-1",
        extensionId: input.extensionId,
        version: input.version,
        publisher: "com.example",
        signatureStatus: "verified",
        trustDecision: "official",
        securityPassed: true,
        requiredConfirmations: [],
        riskFlags: [],
        issues: [],
        requiredPermissions: [],
      };
    }
    if (method === "host.plugin.install") return { operationId: "install-1", extensionId: input.extensionId, version: input.version };
    if (method === "host.plugin.uninstall") return { operationId: "uninstall-1", extensionId: input.extensionId, operationType: "uninstall" };
    throw new Error(`unexpected host method: ${method}`);
  },
};

await extension.activate({
  handlers: {
    bindTool(name, handler) {
      handlers.set(name, handler);
    },
  },
  host,
  log: {
    info() {},
  },
});

const command = handlers.get("command");
if (!command) throw new Error("command handler was not registered");
const catalog = await command({ action: "catalog", platform: "macos" });
const byId = new Map(catalog.listings.map((item) => [item.extensionId, item]));
if (!byId.get("com.example/current")?.installed || byId.get("com.example/current")?.installable) {
  throw new Error("current plugin state is incorrect");
}
if (!byId.get("com.example/update")?.updateAvailable || byId.get("com.example/update")?.actionLabel !== "更新") {
  throw new Error("update plugin state is incorrect");
}
if (!byId.get("com.example/new")?.installable || byId.get("com.example/new")?.installed) {
  throw new Error("new plugin state is incorrect");
}
const prepared = await command({
  action: "prepare",
  extensionId: "com.example/update",
  version: "2.0.0",
  packageUri: "https://example.com/update.amitiax",
  sha256: "b".repeat(64),
});
if (prepared.operationLabel !== "确认更新") throw new Error("update preview label is incorrect");
await command({ action: "uninstall", extensionId: "com.example/update", confirmed: true });
await command({ action: "uninstall", extensionId: "com.example/update", confirmed: true });
const uninstallCalls = calls.filter((call) => call.method === "host.plugin.uninstall");
if (uninstallCalls.length !== 2) throw new Error("uninstall host API was not called");
if (uninstallCalls[0].input.idempotencyKey === uninstallCalls[1].input.idempotencyKey) {
  throw new Error("uninstall operations must use unique idempotency keys");
}
const networkCall = calls.find((call) => call.method === "host.network.request");
if (networkCall.input.timeoutMs !== 10000) {
  throw new Error(`catalog request must bound timeout to 10000ms, got ${networkCall.input.timeoutMs}`);
}
if (networkCall.input.failOnServerError !== true) {
  throw new Error("catalog request must fail on upstream server errors");
}

async function activateWithHost(nextHost) {
  let boundHandler = null;
  await extension.activate({
    handlers: {
      bindTool(name, handler) {
        if (name === "command") boundHandler = handler;
      },
    },
    host: nextHost,
    log: { info() {} },
  });
  if (!boundHandler) throw new Error("command handler was not registered");
  return boundHandler;
}

const upstreamBody = {
  code: 500,
  msg: "读取插件市场分类: dial tcp 127.0.0.1:3306: connect: connection refused",
  data: null,
};
const failingHost = {
  async call(method) {
    if (method === "host.network.request") {
      return { status: 500, body: upstreamBody };
    }
    throw new Error(`unexpected host method: ${method}`);
  },
};
const failingCommand = await activateWithHost(failingHost);
let upstreamError = null;
try {
  await failingCommand({ action: "catalog", platform: "macos" });
} catch (error) {
  upstreamError = error;
}
if (!upstreamError) throw new Error("upstream server error must not be swallowed");
if (!String(upstreamError.message).includes("读取插件市场分类")) {
  throw new Error(`upstream error must keep original cause, got ${upstreamError.message}`);
}
if (!String(upstreamError.message).includes("connection refused")) {
  throw new Error(`upstream error must keep original cause text, got ${upstreamError.message}`);
}
if (!String(upstreamError.message).includes("upstream_error")) {
  throw new Error(`upstream error must keep error class, got ${upstreamError.message}`);
}

const hostFailureText = "host.call: timeout: context deadline exceeded after 10s";
const throwingHost = {
  async call(method) {
    if (method === "host.network.request") {
      throw new Error(hostFailureText);
    }
    throw new Error(`unexpected host method: ${method}`);
  },
};
const throwingCommand = await activateWithHost(throwingHost);
let transportError = null;
try {
  await throwingCommand({ action: "catalog", platform: "macos" });
} catch (error) {
  transportError = error;
}
if (!transportError) throw new Error("transport failure must not be swallowed");
if (!String(transportError.message).includes(hostFailureText)) {
  throw new Error(`transport failure must keep original cause, got ${transportError.message}`);
}
console.log("plugin-market runtime tests passed");

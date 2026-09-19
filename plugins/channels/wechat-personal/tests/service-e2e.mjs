import http from "node:http";
import net from "node:net";
import fs from "node:fs";
import os from "node:os";
import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";
import { dirname, join, resolve } from "node:path";
import assert from "node:assert/strict";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const SERVICE_TOKEN = "amitia-test-service-token";
const AUTH_HEADERS = { authorization: `Bearer ${SERVICE_TOKEN}` };
const stateDir = fs.mkdtempSync(join(os.tmpdir(), "amitia-wechat-personal-"));
const outputFile = join(stateDir, "sent.jsonl");
let lastCoreInbound = null;

function server(handler) {
  const current = http.createServer(handler);
  return new Promise((resolvePromise, reject) => {
    current.once("error", reject);
    current.listen(0, "127.0.0.1", () => resolvePromise(current));
  });
}

async function readBody(req) {
  const chunks = [];
  for await (const chunk of req) chunks.push(chunk);
  const text = Buffer.concat(chunks).toString("utf8");
  return text ? JSON.parse(text) : {};
}

function reply(res, value, status = 200) {
  const data = JSON.stringify(value);
  res.writeHead(status, { "content-type": "application/json", "content-length": Buffer.byteLength(data) });
  res.end(data);
}

async function waitFor(fn, timeout = 8000) {
  const started = Date.now();
  while (Date.now() - started < timeout) {
    try {
      const value = await fn();
      if (value) return value;
    } catch {}
    await new Promise((resolvePromise) => setTimeout(resolvePromise, 100));
  }
  throw new Error("timeout");
}

function freePort() {
  return new Promise((resolvePromise, reject) => {
    const current = net.createServer();
    current.once("error", reject);
    current.listen(0, "127.0.0.1", () => {
      const port = current.address().port;
      current.close(() => resolvePromise(port));
    });
  });
}

const core = await server(async (req, res) => {
  if (req.url === "/api/channels/inbound" && req.method === "POST") {
    if (req.headers.authorization !== `Bearer ${SERVICE_TOKEN}` || req.headers["x-amitia-extension-id"] !== "com.amitia/channel-wechat-personal" || req.headers["x-amitia-module-id"] !== "wechat-personal-channel-service") return reply(res, { error: "unauthorized" }, 401);
    lastCoreInbound = await readBody(req);
    return reply(res, { success: true, conversationId: "conv-test" });
  }
  return reply(res, { error: "not_found" }, 404);
});

const servicePort = await freePort();
const serviceUrl = `http://127.0.0.1:${servicePort}`;
const fixture = join(root, "tests", "fixtures", "wechaty-runtime.mjs");
const child = spawn(process.execPath, [join(root, "runtime", "service.mjs")], {
  cwd: root,
  env: {
    ...process.env,
    AMITIA_SERVICE_AUTH_TOKEN: SERVICE_TOKEN,
    AMITIA_SERVICE_AUTH_VERSION: "1",
    AMITIA_CORE_URL: `http://127.0.0.1:${core.address().port}`,
    AMITIA_WECHAT_STATE_DIR: stateDir,
    AMITIA_WECHAT_SERVICE_PORT: String(servicePort),
    AMITIA_WECHAT_RUNTIME_MODULE: fixture,
    AMITIA_WECHAT_PUPPET_MODULE: fixture,
    AMITIA_WECHAT_TEST_OUTPUT: outputFile,
  },
  stdio: ["ignore", "pipe", "pipe"],
});
let logs = "";
child.stdout.on("data", (chunk) => { logs += chunk; });
child.stderr.on("data", (chunk) => { logs += chunk; });

try {
  await waitFor(async () => (await fetch(`${serviceUrl}/api/health`, { headers: AUTH_HEADERS })).ok);
  const connect = await fetch(`${serviceUrl}/api/connect`, { method: "POST", headers: { ...AUTH_HEADERS, "content-type": "application/json" }, body: JSON.stringify({ force: true }) }).then((response) => response.json());
  assert.equal(connect.success, true);
  await new Promise((resolvePromise) => setTimeout(resolvePromise, 250));
  const connectedPayload = await fetch(`${serviceUrl}/api/status`, { headers: AUTH_HEADERS }).then((response) => response.json());
  const connected = connectedPayload.data;
  assert.equal(connected.connected, true, JSON.stringify(connected));
  assert.equal(connected.accountId, "wxid_self");
  assert.equal(connected.protocol, "wechaty-web");
  assert.equal(connected.localWechatRequired, false);
  await waitFor(() => Boolean(lastCoreInbound));
  assert.equal(lastCoreInbound.channelId, "wechat_personal");
  assert.equal(lastCoreInbound.accountId, "wxid_self");
  assert.equal(lastCoreInbound.peerId, "wxid_friend");
  assert.equal(lastCoreInbound.text, "你好");
  const send = await fetch(`${serviceUrl}/api/send`, { method: "POST", headers: { ...AUTH_HEADERS, "content-type": "application/json", "idempotency-key": "delivery-1" }, body: JSON.stringify({ toUserId: "wxid_friend", text: "你好", deliveryKey: "delivery-1" }) }).then((response) => response.json());
  assert.equal(send.success, true);
  const duplicate = await fetch(`${serviceUrl}/api/send`, { method: "POST", headers: { ...AUTH_HEADERS, "content-type": "application/json", "idempotency-key": "delivery-1" }, body: JSON.stringify({ toUserId: "wxid_friend", text: "你好", deliveryKey: "delivery-1" }) }).then((response) => response.json());
  assert.equal(duplicate.duplicate, true);
  const sent = fs.readFileSync(outputFile, "utf8").trim().split("\n").map((line) => JSON.parse(line));
  assert.deepEqual(sent, [{ to: "wxid_friend", text: "你好" }]);
  const disconnected = await fetch(`${serviceUrl}/api/disconnect`, { method: "POST", headers: AUTH_HEADERS }).then((response) => response.json());
  assert.equal(disconnected.disconnected, true);
  assert.equal(fs.existsSync(join(stateDir, "account.json")), false);
  console.log("wechat-personal Wechaty service e2e: PASS");
} finally {
  child.kill("SIGTERM");
  await new Promise((resolvePromise) => setTimeout(resolvePromise, 250));
  core.close();
  fs.rmSync(stateDir, { recursive: true, force: true });
  if (child.exitCode && child.exitCode !== 0) console.error(logs);
}

import http from "node:http";
import fs from "node:fs";
import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";
import { dirname, join, resolve } from "node:path";
import assert from "node:assert/strict";

const root = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const SERVICE_TOKEN = "amitia-test-service-token";
const AUTH_HEADERS = { authorization: `Bearer ${SERVICE_TOKEN}` };
let loginChecks = 0;
let lastCoreInbound = null;
let lastSend = null;

function server(port, handler) {
  const s = http.createServer(handler);
  return new Promise((resolvePromise, reject) => {
    s.once("error", reject);
    s.listen(port, "127.0.0.1", () => resolvePromise(s));
  });
}
async function readBody(req) {
  const chunks = [];
  for await (const c of req) chunks.push(c);
  const text = Buffer.concat(chunks).toString("utf8");
  return text ? JSON.parse(text) : {};
}
function reply(res, value, status = 200) {
  const data = JSON.stringify(value);
  res.writeHead(status, { "content-type": "application/json", "content-length": Buffer.byteLength(data) });
  res.end(data);
}
async function waitFor(fn, timeout = 6000) {
  const start = Date.now();
  while (Date.now() - start < timeout) {
    try { const value = await fn(); if (value) return value; } catch {}
    await new Promise((r) => setTimeout(r, 120));
  }
  throw new Error("timeout");
}

const driver = await server(8888, async (req, res) => {
  const u = new URL(req.url, "http://127.0.0.1:8888");
  if (u.pathname === "/status") return reply(res, { matched: true, version: "mock-4.x" });
  if (u.pathname === "/login/state") return reply(res, { logged: ++loginChecks >= 3 });
  if (u.pathname === "/self/info") return reply(res, { wxid: "wxid_ai", nickname: "Amitia测试号", alias: "amitia_test" });
  if (u.pathname === "/qr/url") {
    const target = u.searchParams.get("path");
    if (target) fs.writeFileSync(target, Buffer.concat([Buffer.from("PNGMOCK"), Buffer.alloc(128, 1)]));
    return reply(res, { code: 0, data: { url: "https://example.invalid/mock-qr" } });
  }
  if (u.pathname === "/send" && req.method === "POST") {
    lastSend = await readBody(req);
    return reply(res, { code: 0, msg: "ok" });
  }
  return reply(res, { error: "not_found" }, 404);
});
const core = await server(0, async (req, res) => {
  if (req.url === "/api/channels/inbound" && req.method === "POST") {
    if (
      req.headers.authorization !== `Bearer ${SERVICE_TOKEN}`
      || req.headers["x-amitia-extension-id"] !== "com.amitia/channel-wechat-personal"
      || req.headers["x-amitia-module-id"] !== "wechat-personal-channel-service"
    ) return reply(res, { error: "unauthorized" }, 401);
    lastCoreInbound = await readBody(req);
    return reply(res, { success: true, conversationId: "conv-test" });
  }
  return reply(res, { error: "not_found" }, 404);
});
const coreUrl = `http://127.0.0.1:${core.address().port}`;

const child = spawn(process.execPath, [join(root, "runtime", "service.mjs")], {
  cwd: root,
  env: { ...process.env, AMITIA_SERVICE_AUTH_TOKEN: SERVICE_TOKEN, AMITIA_SERVICE_AUTH_VERSION: "1", AMITIA_CORE_URL: coreUrl, AMITIA_WECHAT_EXTERNAL_DRIVER_COMPAT: "1" },
  stdio: ["ignore", "pipe", "pipe"],
});
let logs = "";
child.stdout.on("data", (c) => { logs += c; });
child.stderr.on("data", (c) => { logs += c; });

try {
  await waitFor(async () => (await fetch("http://127.0.0.1:19878/api/health", { headers: AUTH_HEADERS })).ok);
  const connect = await fetch("http://127.0.0.1:19878/api/connect", {
    method: "POST", headers: { ...AUTH_HEADERS, "content-type": "application/json" }, body: JSON.stringify({ launchWechat: false }),
  }).then((r) => r.json());
  assert.equal(connect.success, true);
  assert.equal(connect.data.status, "qr_ready");
  assert.match(connect.data.qrCodeUrl, /^data:image\/png;base64,/);

  const connected = await waitFor(async () => {
    const payload = await fetch("http://127.0.0.1:19878/api/status", { headers: AUTH_HEADERS }).then((r) => r.json());
    return payload?.data?.connected ? payload.data : null;
  }, 8000);
  assert.equal(connected.accountId, "wxid_ai");
  assert.equal(connected.driverKind, "hero");

  const inbound = await fetch("http://127.0.0.1:19878/api/native/callback", {
    method: "POST", headers: { ...AUTH_HEADERS, "content-type": "application/json" },
    body: JSON.stringify({ roomId: "room@chatroom", fromWxid: "room@chatroom", senderWxid: "wxid_member", content: "群里你好", msgId: "m-1", msgType: 1 }),
  }).then((r) => r.json());
  assert.equal(inbound.success, true);
  assert.equal(lastCoreInbound.channelId, "wechat_personal");
  assert.equal(lastCoreInbound.accountId, "wxid_ai");
  assert.equal(lastCoreInbound.peerId, "wxid_member");
  assert.equal(lastCoreInbound.contentType, "text");

  const send = await fetch("http://127.0.0.1:19878/api/send", {
    method: "POST", headers: { ...AUTH_HEADERS, "content-type": "application/json", "idempotency-key": "delivery-1" },
    body: JSON.stringify({ toUserId: "wxid_friend", text: "你好", deliveryKey: "delivery-1" }),
  }).then((r) => r.json());
  assert.equal(send.success, true);
  assert.deepEqual(lastSend, { wxid: "wxid_friend", content: "你好" });

  const duplicate = await fetch("http://127.0.0.1:19878/api/send", {
    method: "POST", headers: { ...AUTH_HEADERS, "content-type": "application/json", "idempotency-key": "delivery-1" },
    body: JSON.stringify({ toUserId: "wxid_friend", text: "你好", deliveryKey: "delivery-1" }),
  }).then((r) => r.json());
  assert.equal(duplicate.duplicate, true);

  console.log("wechat-personal service e2e: PASS");
} finally {
  child.kill("SIGTERM");
  await new Promise((r) => setTimeout(r, 150));
  driver.close(); core.close();
  if (child.exitCode && child.exitCode !== 0) console.error(logs);
}

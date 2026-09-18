import http from "node:http";
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
let loginChecks = 0;
let lastCoreInbound = null;
let lastSend = null;
let sendCount = 0;

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

const ilink = await server(async (req, res) => {
  const url = new URL(req.url, "http://127.0.0.1");
  if (url.pathname === "/ilink/bot/get_bot_qrcode" && req.method === "POST") {
    await readBody(req);
    return reply(res, {
      qrcode: `mock-qr-${Date.now()}`,
      qrcode_img_content: "https://liteapp.weixin.qq.com/q/mock",
      ret: 0,
    });
  }
  if (url.pathname === "/ilink/bot/get_qrcode_status" && req.method === "GET") {
    loginChecks += 1;
    if (loginChecks < 2) return reply(res, { status: "wait" });
    return reply(res, {
      status: "confirmed",
      bot_token: "mock-bot-token",
      ilink_bot_id: "bot-123@im.bot",
      ilink_user_id: "wxid_user",
      baseurl: `http://127.0.0.1:${ilink.address().port}`,
    });
  }
  if (url.pathname === "/ilink/bot/msg/notifystart" || url.pathname === "/ilink/bot/msg/notifystop") {
    await readBody(req);
    return reply(res, { ret: 0 });
  }
  if (url.pathname === "/ilink/bot/getupdates" && req.method === "POST") {
    const body = await readBody(req);
    if (body.get_updates_buf !== "sync-1") {
      await new Promise((resolvePromise) => setTimeout(resolvePromise, 120));
      return reply(res, {
        ret: 0,
        get_updates_buf: "sync-1",
        msgs: [{
          message_id: "message-1",
          from_user_id: "wxid_friend",
          to_user_id: "bot-123@im.bot",
          create_time_ms: 1700000000000,
          message_type: 1,
          context_token: "context-1",
          item_list: [{ type: 1, text_item: { text: "你好" } }],
        }],
      });
    }
    await new Promise((resolvePromise) => setTimeout(resolvePromise, 150));
    return reply(res, { ret: 0, get_updates_buf: "sync-1", msgs: [] });
  }
  if (url.pathname === "/ilink/bot/sendmessage" && req.method === "POST") {
    lastSend = await readBody(req);
    sendCount += 1;
    return reply(res, { ret: 0 });
  }
  return reply(res, { error: "not_found" }, 404);
});

const core = await server(async (req, res) => {
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
const ilinkUrl = `http://127.0.0.1:${ilink.address().port}`;
const child = spawn(process.execPath, [join(root, "runtime", "service.mjs")], {
  cwd: root,
  env: {
    ...process.env,
    AMITIA_SERVICE_AUTH_TOKEN: SERVICE_TOKEN,
    AMITIA_SERVICE_AUTH_VERSION: "1",
    AMITIA_CORE_URL: coreUrl,
    AMITIA_WECHAT_ILINK_BASE_URL: ilinkUrl,
    AMITIA_WECHAT_STATE_DIR: stateDir,
  },
  stdio: ["ignore", "pipe", "pipe"],
});
let logs = "";
child.stdout.on("data", (chunk) => { logs += chunk; });
child.stderr.on("data", (chunk) => { logs += chunk; });

try {
  await waitFor(async () => (await fetch("http://127.0.0.1:19878/api/health", { headers: AUTH_HEADERS })).ok);
  const connect = await fetch("http://127.0.0.1:19878/api/connect", {
    method: "POST",
    headers: { ...AUTH_HEADERS, "content-type": "application/json" },
    body: JSON.stringify({ force: true }),
  }).then((response) => response.json());
  assert.equal(connect.success, true);
  assert.equal(connect.data.status, "qr_ready");
  assert.match(connect.data.qrImageUrl, /^data:image\/png;base64,/);

  const connected = await waitFor(async () => {
    const payload = await fetch("http://127.0.0.1:19878/api/status", { headers: AUTH_HEADERS }).then((response) => response.json());
    return payload?.data?.connected ? payload.data : null;
  });
  assert.equal(connected.accountId, "bot-123@im.bot");
  assert.equal(connected.protocol, "ilink");
  assert.equal(connected.localWechatRequired, false);

  await waitFor(() => Boolean(lastCoreInbound));
  assert.equal(lastCoreInbound.channelId, "wechat_personal");
  assert.equal(lastCoreInbound.accountId, "bot-123@im.bot");
  assert.equal(lastCoreInbound.peerId, "wxid_friend");
  assert.equal(lastCoreInbound.text, "你好");

  const send = await fetch("http://127.0.0.1:19878/api/send", {
    method: "POST",
    headers: { ...AUTH_HEADERS, "content-type": "application/json", "idempotency-key": "delivery-1" },
    body: JSON.stringify({ toUserId: "wxid_friend", text: "你好", deliveryKey: "delivery-1" }),
  }).then((response) => response.json());
  assert.equal(send.success, true);
  assert.equal(lastSend.msg.to_user_id, "wxid_friend");
  assert.equal(lastSend.msg.context_token, "context-1");
  assert.equal(lastSend.msg.item_list[0].text_item.text, "你好");

  const duplicate = await fetch("http://127.0.0.1:19878/api/send", {
    method: "POST",
    headers: { ...AUTH_HEADERS, "content-type": "application/json", "idempotency-key": "delivery-1" },
    body: JSON.stringify({ toUserId: "wxid_friend", text: "你好", deliveryKey: "delivery-1" }),
  }).then((response) => response.json());
  assert.equal(duplicate.duplicate, true);
  assert.equal(sendCount, 1);

  const disconnected = await fetch("http://127.0.0.1:19878/api/disconnect", {
    method: "POST",
    headers: AUTH_HEADERS,
  }).then((response) => response.json());
  assert.equal(disconnected.disconnected, true);
  assert.equal(fs.existsSync(join(stateDir, "account.json")), false);

  console.log("wechat-personal iLink service e2e: PASS");
} finally {
  child.kill("SIGTERM");
  await new Promise((resolvePromise) => setTimeout(resolvePromise, 250));
  ilink.close();
  core.close();
  fs.rmSync(stateDir, { recursive: true, force: true });
  if (child.exitCode && child.exitCode !== 0) console.error(logs);
}

import http from "node:http";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { createRequire } from "node:module";
import { randomUUID, timingSafeEqual } from "node:crypto";
import { fileURLToPath, pathToFileURL } from "node:url";

const CHANNEL_ID = "wechat_personal";
const EXTENSION_ID = "com.amitia/channel-wechat-personal";
const MODULE_ID = "wechat-personal-channel-service";
const HOST = "127.0.0.1";
const PORT = Number(process.env.AMITIA_WECHAT_SERVICE_PORT || 19878);
const CORE_URL = String(process.env.AMITIA_CORE_URL || "").trim().replace(/\/+$/, "");
const SERVICE_AUTH_TOKEN = String(process.env.AMITIA_SERVICE_AUTH_TOKEN || "").trim();
const SERVICE_AUTH_VERSION = String(process.env.AMITIA_SERVICE_AUTH_VERSION || "").trim();
const DEV_ALLOW_UNAUTHENTICATED = process.env.AMITIA_WECHAT_DEV_ALLOW_UNAUTHENTICATED === "1";
const RUNTIME_MODULE = String(process.env.AMITIA_WECHAT_RUNTIME_MODULE || "./node_modules/wechaty/dist/esm/src/wechaty-builder.js").trim();
const PUPPET_MODULE = String(process.env.AMITIA_WECHAT_PUPPET_MODULE || "wechaty-puppet-wechat4u").trim();
const MODULE_DIR = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(import.meta.url);
const QRCode = require([path.join(MODULE_DIR, "vendor", "qrcode"), path.join(MODULE_DIR, "..", "vendor", "qrcode")].find((candidate) => fs.existsSync(candidate)) || path.join(MODULE_DIR, "vendor", "qrcode"));

if (!SERVICE_AUTH_TOKEN && !DEV_ALLOW_UNAUTHENTICATED) throw new Error("AMITIA_SERVICE_AUTH_TOKEN is required for the personal WeChat trusted service");
if (!CORE_URL) throw new Error("AMITIA_CORE_URL is required for the personal WeChat trusted service");
if (SERVICE_AUTH_VERSION && SERVICE_AUTH_VERSION !== "1") throw new Error(`Unsupported AMITIA_SERVICE_AUTH_VERSION: ${SERVICE_AUTH_VERSION}`);

function resolveStateDir() {
  const override = String(process.env.AMITIA_WECHAT_STATE_DIR || "").trim();
  if (override) return path.resolve(override);
  if (process.platform === "win32") return path.join(process.env.APPDATA || path.join(os.homedir(), "AppData", "Roaming"), "Amitia", "extensions", "wechat-personal");
  return path.join(process.env.XDG_CONFIG_HOME || path.join(os.homedir(), ".config"), "amitia", "extensions", "wechat-personal");
}

const STATE_DIR = resolveStateDir();
const SESSION_DIR = path.join(STATE_DIR, "wechaty");
const ACCOUNT_FILE = path.join(STATE_DIR, "account.json");
fs.mkdirSync(STATE_DIR, { recursive: true });

function readAccount() {
  try {
    const value = JSON.parse(fs.readFileSync(ACCOUNT_FILE, "utf8"));
    return value && typeof value === "object" ? value : null;
  } catch {
    return null;
  }
}

function saveAccount(value) {
  fs.mkdirSync(STATE_DIR, { recursive: true });
  fs.writeFileSync(ACCOUNT_FILE, JSON.stringify(value, null, 2), { encoding: "utf8", mode: 0o600 });
}

const savedAccount = readAccount();
const state = {
  status: savedAccount ? "reconnecting" : "disconnected",
  connected: false,
  running: true,
  accountId: String(savedAccount?.accountId || ""),
  nickname: String(savedAccount?.nickname || ""),
  alias: String(savedAccount?.alias || ""),
  avatar: "",
  qrCodeUrl: "",
  qrImageUrl: "",
  sessionKey: "",
  protocol: "wechaty-web",
  transport: "Wechaty Web 协议",
  localWechatRequired: false,
  managedWechat: false,
  driverKind: savedAccount ? "wechaty" : "none",
  driverVersion: "wechaty-puppet-wechat4u",
  messageTransportReady: false,
  messageCount: 0,
  replyCount: 0,
  startedAt: "",
  lastError: "",
  message: savedAccount ? "正在恢复个人微信会话" : "等待连接个人微信",
  platform: process.platform,
  architecture: process.arch,
};

let bot = null;
let startPromise = null;
let stopped = false;
const seenInbound = new Map();
const delivered = new Map();
const contacts = new Map();
const conversations = new Map();

function prune(map, limit = 1000) {
  while (map.size > limit) map.delete(map.keys().next().value);
}

function recordMessage(conversationId, peerId, role, content, createdAt = new Date().toISOString()) {
  const key = String(conversationId || peerId || "default");
  const list = conversations.get(key) || [];
  list.push({ id: randomUUID(), conversationId: key, peerId: String(peerId || ""), role, content: String(content || ""), createdAt });
  if (list.length > 500) list.splice(0, list.length - 500);
  conversations.set(key, list);
}

function readMessages(conversationId, limit, offset) {
  const bindings = [...conversations.entries()].map(([id, items]) => ({ conversationId: id, externalConversationId: items[0]?.peerId || id, externalUserId: items[0]?.peerId || id, displayName: items[0]?.peerId || id }));
  const selected = conversationId ? conversations.get(String(conversationId)) || [] : [...conversations.values()].flat();
  return { bindings, messages: selected.slice(Math.max(0, offset), Math.max(0, offset) + Math.max(1, limit)) };
}

function json(res, statusCode, value) {
  const body = JSON.stringify(value);
  res.writeHead(statusCode, { "Content-Type": "application/json; charset=utf-8", "Content-Length": Buffer.byteLength(body), "Cache-Control": "no-store", "X-Content-Type-Options": "nosniff" });
  res.end(body);
}

function constantTimeEqual(left, right) {
  const a = Buffer.from(String(left || ""));
  const b = Buffer.from(String(right || ""));
  return a.length === b.length && timingSafeEqual(a, b);
}

function requestAuthorized(req) {
  if (DEV_ALLOW_UNAUTHENTICATED) return true;
  const header = String(req.headers.authorization || "");
  return header.startsWith("Bearer ") && constantTimeEqual(header.slice(7), SERVICE_AUTH_TOKEN);
}

function requireAuthorized(req, res) {
  if (requestAuthorized(req)) return true;
  json(res, 401, { success: false, message: "unauthorized" });
  return false;
}

async function readJson(req) {
  const chunks = [];
  let total = 0;
  for await (const chunk of req) {
    total += chunk.length;
    if (total > 2 * 1024 * 1024) throw new Error("request too large");
    chunks.push(chunk);
  }
  const text = Buffer.concat(chunks).toString("utf8").trim();
  return text ? JSON.parse(text) : {};
}

function safeId(value) {
  return String(value || "").trim().replace(/[^a-zA-Z0-9_@.-]/g, "_");
}

async function entityId(entity) {
  if (!entity) return "";
  if (typeof entity.id === "function") return String(await entity.id());
  return String(entity.id || entity.payload?.id || "");
}

async function entityName(entity) {
  if (!entity) return "";
  if (typeof entity.name === "function") return String(await entity.name());
  return String(entity.name || entity.payload?.name || "");
}

async function entityAlias(entity) {
  if (!entity || typeof entity.alias !== "function") return "";
  try { return String(await entity.alias()); } catch { return ""; }
}

function moduleSpecifier(value) {
  if (path.isAbsolute(value)) return pathToFileURL(value).href;
  return value;
}

async function createWechatBot() {
  const runtime = await import(moduleSpecifier(RUNTIME_MODULE));
  const puppetRuntime = RUNTIME_MODULE === PUPPET_MODULE ? runtime : await import(moduleSpecifier(PUPPET_MODULE));
  const WechatyBuilder = runtime.WechatyBuilder || runtime.default?.WechatyBuilder;
  const PuppetWechat4u = puppetRuntime.PuppetWechat4u || puppetRuntime.default?.PuppetWechat4u || puppetRuntime.default;
  if (!WechatyBuilder?.build || !PuppetWechat4u) throw new Error("Wechaty Web 协议运行时不可用");
  fs.mkdirSync(SESSION_DIR, { recursive: true });
  const puppet = new PuppetWechat4u({ uos: true, name: path.join(SESSION_DIR, "session") });
  const instance = WechatyBuilder.build({ name: path.join(SESSION_DIR, "session"), puppet });
  instance.on("scan", (qrcode) => void onScan(qrcode));
  instance.on("login", (user) => void onLogin(user));
  instance.on("logout", () => void onLogout());
  instance.on("message", (message) => void onMessage(message));
  instance.on("error", (error) => onError(error));
  return instance;
}

async function onScan(qrcode) {
  if (!qrcode || stopped) return;
  try {
    const image = await QRCode.toDataURL(String(qrcode), { width: 320, margin: 2, errorCorrectionLevel: "M" });
    if (stopped || state.connected) return;
    state.status = "qr_ready";
    state.connected = false;
    state.qrCodeUrl = image;
    state.qrImageUrl = image;
    state.sessionKey = randomUUID();
    state.message = "请使用手机微信扫描二维码";
    state.lastError = "";
  } catch (error) {
    onError(error);
  }
}

async function onLogin(user) {
  if (stopped) return;
  const accountId = await entityId(user);
  const nickname = await entityName(user);
  const alias = await entityAlias(user);
  state.status = "connected";
  state.connected = true;
  state.accountId = accountId || state.accountId;
  state.nickname = nickname || state.nickname || "个人微信";
  state.alias = alias || state.alias;
  state.qrCodeUrl = "";
  state.qrImageUrl = "";
  state.sessionKey = "";
  state.driverKind = "wechaty";
  state.messageTransportReady = true;
  state.startedAt = new Date().toISOString();
  state.lastError = "";
  state.message = "个人微信已连接";
  saveAccount({ accountId: state.accountId, nickname: state.nickname, alias: state.alias, connectedAt: state.startedAt });
}

async function onLogout() {
  if (stopped) return;
  state.connected = false;
  state.messageTransportReady = false;
  state.status = "disconnected";
  state.message = "个人微信已退出，请重新连接";
}

function onError(error) {
  state.lastError = String(error?.message || error || "Wechaty Web 协议错误");
  if (!state.connected) state.status = "login_error";
  state.message = state.lastError;
}

async function onMessage(message) {
  try {
    if (!state.connected || !message || (typeof message.self === "function" && message.self())) return;
    const text = String(typeof message.text === "function" ? await message.text() : message.text || "").trim();
    if (!text) return;
    const talker = typeof message.talker === "function" ? await message.talker() : message.talker;
    const room = typeof message.room === "function" ? await message.room() : message.room;
    const peer = room || talker;
    const peerId = await entityId(peer);
    const senderId = await entityId(talker);
    const messageId = String(typeof message.id === "function" ? await message.id() : message.id || randomUUID());
    if (!peerId || !senderId) return;
    contacts.set(peerId, peer);
    const dedupeKey = `${peerId}:${messageId}`;
    if (seenInbound.has(dedupeKey)) return;
    seenInbound.set(dedupeKey, Date.now());
    prune(seenInbound);
    const conversationId = `wechat-personal-${safeId(state.accountId || "default")}-${safeId(peerId)}`;
    const headers = { "Content-Type": "application/json", "X-Amitia-Extension-ID": EXTENSION_ID, "X-Amitia-Module-ID": MODULE_ID };
    if (SERVICE_AUTH_TOKEN) headers.Authorization = `Bearer ${SERVICE_AUTH_TOKEN}`;
    const response = await fetch(`${CORE_URL}/api/channels/inbound`, { method: "POST", headers, body: JSON.stringify({ channelId: CHANNEL_ID, accountId: state.accountId || "wechat-personal", conversationId, peerId: senderId, messageId, contentType: "text", text }) });
    if (!response.ok) throw new Error(`Amitia Core HTTP ${response.status}`);
    state.messageCount += 1;
    recordMessage(conversationId, senderId, "user", text);
  } catch (error) {
    onError(error);
  }
}

async function startWechat(force = false) {
  if (startPromise) return startPromise;
  if (bot && (!force || state.connected)) return bot;
  if (bot) {
    stopped = true;
    const current = bot;
    bot = null;
    try { await current.logout?.(); } catch {}
    try { await current.stop?.(); } catch {}
  }
  stopped = false;
  state.status = "connecting";
  state.message = "正在启动个人微信登录";
  startPromise = (async () => {
    const instance = await createWechatBot();
    bot = instance;
    await instance.start();
    return instance;
  })();
  try {
    return await startPromise;
  } catch (error) {
    bot = null;
    onError(error);
    throw error;
  } finally {
    startPromise = null;
  }
}

async function findTarget(target) {
  const cached = contacts.get(target);
  if (cached) return cached;
  if (bot?.Contact?.find) {
    const contact = await bot.Contact.find({ id: target });
    if (contact) return contact;
  }
  if (bot?.Room?.find) {
    const room = await bot.Room.find({ id: target });
    if (room) return room;
  }
  return null;
}

async function sendText(peerId, text, deliveryKey = "") {
  if (!state.connected || !bot) throw new Error("个人微信尚未连接");
  const target = String(peerId || "").trim();
  const content = String(text || "").trim();
  if (!target || !content) throw new Error("toUserId and text are required");
  if (deliveryKey && delivered.has(deliveryKey)) return { duplicate: true };
  const recipient = await findTarget(target);
  if (!recipient || typeof recipient.say !== "function") throw new Error("当前会话不可用，请等待联系人先发送消息后再回复");
  await recipient.say(content);
  if (deliveryKey) {
    delivered.set(deliveryKey, Date.now());
    prune(delivered);
  }
  state.replyCount += 1;
  recordMessage(`wechat-personal-${safeId(state.accountId || "default")}-${safeId(target)}`, target, "assistant", content);
  return { duplicate: false };
}

async function disconnect() {
  stopped = true;
  const current = bot;
  bot = null;
  contacts.clear();
  if (current) {
    try { await current.logout?.(); } catch {}
    try { await current.stop?.(); } catch {}
  }
  try { fs.rmSync(ACCOUNT_FILE, { force: true }); } catch {}
  state.status = "disconnected";
  state.connected = false;
  state.accountId = "";
  state.nickname = "";
  state.alias = "";
  state.qrCodeUrl = "";
  state.qrImageUrl = "";
  state.sessionKey = "";
  state.driverKind = "none";
  state.messageTransportReady = false;
  state.message = "已断开个人微信渠道";
  state.lastError = "";
}

async function handleMain(req, res) {
  const url = new URL(req.url, `http://${HOST}:${PORT}`);
  if (!requireAuthorized(req, res)) return;
  try {
    if (req.method === "GET" && url.pathname === "/api/health") return json(res, 200, { success: true, data: { running: state.running, protocol: state.protocol } });
    if (req.method === "GET" && url.pathname === "/api/status") return json(res, 200, { success: true, data: { ...state } });
    if (req.method === "GET" && url.pathname === "/api/config") return json(res, 200, { mode: "wechaty-personal-web", channelId: CHANNEL_ID, transportPort: PORT, defaultRolePolicy: "space_default_character", platforms: ["windows", "linux", "darwin"], localWechatRequired: false, managedClient: false, protocol: state.protocol });
    if (req.method === "POST" && url.pathname === "/api/connect") {
      const body = await readJson(req);
      await startWechat(Boolean(body?.force));
      return json(res, 200, { success: true, data: { ...state } });
    }
    if (req.method === "POST" && url.pathname === "/api/disconnect") {
      await readJson(req);
      await disconnect();
      return json(res, 200, { success: true, disconnected: true, data: { ...state } });
    }
    if (req.method === "GET" && url.pathname === "/api/messages") {
      const result = readMessages(url.searchParams.get("conversationId") || "", Number(url.searchParams.get("limit") || 100), Number(url.searchParams.get("offset") || 0));
      return json(res, 200, { success: true, data: result });
    }
    if (req.method === "POST" && url.pathname === "/api/send") {
      const body = await readJson(req);
      const deliveryKey = String(body.deliveryKey || req.headers["idempotency-key"] || "");
      const result = await sendText(String(body.toUserId || ""), String(body.text || ""), deliveryKey);
      return json(res, 200, { success: true, accepted: true, duplicate: result.duplicate });
    }
    if (req.method === "POST" && (url.pathname === "/api/send-image" || url.pathname === "/api/send-voice")) return json(res, 501, { success: false, message: "个人微信插件暂未声明图片/语音发送能力" });
    return json(res, 404, { success: false, message: "not found" });
  } catch (error) {
    onError(error);
    return json(res, 500, { success: false, message: state.lastError });
  }
}

const mainServer = http.createServer((req, res) => void handleMain(req, res));
mainServer.listen(PORT, HOST, () => console.log(`[wechat-personal] Wechaty provider listening on http://${HOST}:${PORT}`));

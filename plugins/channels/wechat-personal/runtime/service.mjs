import http from "node:http";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";
import { createHash, randomUUID, timingSafeEqual } from "node:crypto";

const CHANNEL_ID = "wechat_personal";
const EXTENSION_ID = "com.amitia/channel-wechat-personal";
const MODULE_ID = "wechat-personal-channel-service";
const HOST = "127.0.0.1";
const PORT = 19878;
const CORE_URL = String(process.env.AMITIA_CORE_URL || "").trim().replace(/\/+$/, "");
const SERVICE_AUTH_TOKEN = String(process.env.AMITIA_SERVICE_AUTH_TOKEN || "").trim();
const SERVICE_AUTH_VERSION = String(process.env.AMITIA_SERVICE_AUTH_VERSION || "").trim();
const DEV_ALLOW_UNAUTHENTICATED = process.env.AMITIA_WECHAT_DEV_ALLOW_UNAUTHENTICATED === "1";
const ILINK_BASE_URL = String(process.env.AMITIA_WECHAT_ILINK_BASE_URL || "https://ilinkai.weixin.qq.com").trim().replace(/\/+$/, "");
const ILINK_APP_ID = "bot";
const ILINK_BOT_TYPE = "3";
const ILINK_CHANNEL_VERSION = "2.4.6";
const ILINK_CLIENT_VERSION = ((2 & 0xff) << 16) | ((4 & 0xff) << 8) | (6 & 0xff);
const LOGIN_TTL_MS = 5 * 60_000;
const LOGIN_LONG_POLL_MS = 35_000;
const MAX_QR_REFRESH = 3;

if (!SERVICE_AUTH_TOKEN && !DEV_ALLOW_UNAUTHENTICATED) {
  throw new Error("AMITIA_SERVICE_AUTH_TOKEN is required for the personal WeChat trusted service");
}
if (!CORE_URL) {
  throw new Error("AMITIA_CORE_URL is required for the personal WeChat trusted service");
}
if (SERVICE_AUTH_VERSION && SERVICE_AUTH_VERSION !== "1") {
  throw new Error(`Unsupported AMITIA_SERVICE_AUTH_VERSION: ${SERVICE_AUTH_VERSION}`);
}

const MODULE_DIR = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(import.meta.url);
const qrCandidates = [
  path.join(MODULE_DIR, "vendor", "qrcode"),
  path.join(MODULE_DIR, "..", "vendor", "qrcode"),
];
const QRCode = require(qrCandidates.find((candidate) => fs.existsSync(candidate)) || qrCandidates[0]);

function resolveStateDir() {
  const override = String(process.env.AMITIA_WECHAT_STATE_DIR || "").trim();
  if (override) return path.resolve(override);
  if (process.platform === "win32") {
    const base = String(process.env.APPDATA || process.env.LOCALAPPDATA || "").trim();
    if (base) return path.join(base, "Amitia", "extensions", "wechat-personal");
  }
  const configHome = String(process.env.XDG_CONFIG_HOME || "").trim();
  return path.join(configHome || path.join(os.homedir(), ".config"), "amitia", "extensions", "wechat-personal");
}

const STATE_DIR = resolveStateDir();
const ACCOUNT_FILE = path.join(STATE_DIR, "account.json");
fs.mkdirSync(STATE_DIR, { recursive: true });

function loadCredentials() {
  try {
    if (!fs.existsSync(ACCOUNT_FILE)) return null;
    const value = JSON.parse(fs.readFileSync(ACCOUNT_FILE, "utf8"));
    if (!value || typeof value !== "object" || !String(value.token || "").trim()) return null;
    return {
      accountId: String(value.accountId || value.id || "").trim(),
      id: String(value.id || value.accountId || "").trim(),
      token: String(value.token || "").trim(),
      baseUrl: String(value.baseUrl || ILINK_BASE_URL).trim().replace(/\/+$/, ""),
      userId: String(value.userId || "").trim(),
      syncBuf: String(value.syncBuf || ""),
      contextTokens: value.contextTokens && typeof value.contextTokens === "object" ? value.contextTokens : {},
    };
  } catch {
    return null;
  }
}

let credentials = loadCredentials();
const contextTokens = new Map(Object.entries(credentials?.contextTokens || {}));
const state = {
  status: credentials ? "connected" : "disconnected",
  connected: Boolean(credentials),
  accountId: credentials?.accountId || "",
  nickname: credentials ? "个人微信" : "",
  alias: credentials?.accountId || "",
  avatar: "",
  qrCodeUrl: "",
  qrImageUrl: "",
  sessionKey: "",
  protocol: "ilink",
  transport: "腾讯 iLink",
  localWechatRequired: false,
  managedWechat: false,
  driverKind: credentials ? "ilink" : "none",
  driverVersion: ILINK_CHANNEL_VERSION,
  messageTransportReady: Boolean(credentials),
  messageCount: 0,
  replyCount: 0,
  startedAt: credentials ? new Date().toISOString() : "",
  lastInboundAt: "",
  lastOutboundAt: "",
  lastError: "",
  message: credentials ? "个人微信已连接" : "等待连接个人微信",
  platform: process.platform,
  architecture: process.arch,
};

let activeLogin = null;
let loginTimer = null;
let loginGeneration = 0;
let monitorAbort = null;
let monitorTask = null;
const seenInbound = new Map();
const delivered = new Map();
const messageHistory = new Map();

function pruneMap(map, max = 1000) {
  if (map.size <= max) return;
  const remove = map.size - max;
  let index = 0;
  for (const key of map.keys()) {
    map.delete(key);
    if (++index >= remove) break;
  }
}

function recordMessage(conversationId, peerId, role, content, createdAt = new Date().toISOString()) {
  const key = String(conversationId || peerId || "default");
  const list = messageHistory.get(key) || [];
  list.push({
    id: randomUUID(),
    conversationId: key,
    peerId: String(peerId || ""),
    role,
    content: String(content || ""),
    createdAt,
  });
  if (list.length > 500) list.splice(0, list.length - 500);
  messageHistory.set(key, list);
}

function readMessages(conversationId, limit, offset) {
  const bindings = [...messageHistory.entries()].map(([id, items]) => ({
    conversationId: id,
    externalConversationId: items[0]?.peerId || id,
    externalUserId: items[0]?.peerId || id,
    displayName: items[0]?.peerId || id,
  }));
  const selected = conversationId
    ? messageHistory.get(String(conversationId)) || []
    : [...messageHistory.values()].flat();
  return {
    bindings,
    messages: selected.slice(Math.max(0, offset), Math.max(0, offset) + Math.max(1, limit)),
  };
}

function json(res, statusCode, value) {
  const body = JSON.stringify(value);
  res.writeHead(statusCode, {
    "Content-Type": "application/json; charset=utf-8",
    "Content-Length": Buffer.byteLength(body),
    "Cache-Control": "no-store",
    "X-Content-Type-Options": "nosniff",
  });
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
  const prefix = "Bearer ";
  if (!header.startsWith(prefix)) return false;
  return constantTimeEqual(header.slice(prefix.length), SERVICE_AUTH_TOKEN);
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
  if (!chunks.length) return {};
  const text = Buffer.concat(chunks).toString("utf8").trim();
  return text ? JSON.parse(text) : {};
}

function safeAccountId(value) {
  return String(value || "").replace(/[^a-zA-Z0-9._@-]/g, "_").slice(0, 160);
}

function conversationKey(accountId, peerId) {
  return `wechat-personal-${accountId || "default"}-${peerId}`.replace(/[^a-zA-Z0-9_@.-]/g, "_");
}

function persistCredentials() {
  if (!credentials) {
    fs.rmSync(ACCOUNT_FILE, { force: true });
    return;
  }
  const payload = {
    ...credentials,
    contextTokens: Object.fromEntries(contextTokens),
  };
  const tempFile = `${ACCOUNT_FILE}.${process.pid}.tmp`;
  fs.writeFileSync(tempFile, `${JSON.stringify(payload, null, 2)}\n`, { mode: 0o600 });
  fs.rmSync(ACCOUNT_FILE, { force: true });
  fs.renameSync(tempFile, ACCOUNT_FILE);
  try {
    fs.chmodSync(ACCOUNT_FILE, 0o600);
  } catch {}
}

function clearCredentials() {
  credentials = null;
  contextTokens.clear();
  fs.rmSync(ACCOUNT_FILE, { force: true });
}

function ensureTrailingSlash(value) {
  return value.endsWith("/") ? value : `${value}/`;
}

function randomWechatUin() {
  const value = Math.floor(Math.random() * 0xffffffff) >>> 0;
  return Buffer.from(String(value), "utf8").toString("base64");
}

function baseInfo() {
  return {
    channel_version: ILINK_CHANNEL_VERSION,
    bot_agent: "Amitia/1.0.0",
  };
}

async function ilinkRequest(baseUrl, endpoint, options = {}) {
  const method = options.method || "POST";
  const token = String(options.token || "").trim();
  const timeoutMs = Number(options.timeoutMs || 15_000);
  const url = new URL(endpoint, ensureTrailingSlash(baseUrl || ILINK_BASE_URL));
  const headers = {
    "iLink-App-Id": ILINK_APP_ID,
    "iLink-App-ClientVersion": String(ILINK_CLIENT_VERSION),
  };
  if (method === "POST") {
    headers["Content-Type"] = "application/json";
    headers.AuthorizationType = "ilink_bot_token";
    headers["X-WECHAT-UIN"] = randomWechatUin();
    if (token) headers.Authorization = `Bearer ${token}`;
  }
  let response;
  try {
    response = await fetch(url, {
      method,
      headers,
      body: method === "POST" ? JSON.stringify(options.body || {}) : undefined,
      signal: timeoutMs > 0 ? AbortSignal.timeout(timeoutMs) : undefined,
    });
  } catch (error) {
    if (error?.name === "TimeoutError") throw new Error("iLink 请求超时");
    throw error;
  }
  const text = await response.text();
  let payload = {};
  if (text) {
    try {
      payload = JSON.parse(text);
    } catch {
      payload = { raw: text };
    }
  }
  if (!response.ok) {
    throw new Error(`iLink HTTP ${response.status}: ${text.slice(0, 240)}`);
  }
  if (payload?.ret !== undefined && Number(payload.ret) !== 0) {
    throw new Error(`iLink ret=${payload.ret} errcode=${payload.errcode ?? ""} errmsg=${payload.errmsg ?? ""}`);
  }
  if (payload?.errcode !== undefined && Number(payload.errcode) !== 0) {
    throw new Error(`iLink errcode=${payload.errcode} errmsg=${payload.errmsg ?? ""}`);
  }
  return payload;
}

async function fetchLoginQr() {
  const localTokenList = credentials?.token ? [credentials.token] : [];
  const payload = await ilinkRequest(ILINK_BASE_URL, `ilink/bot/get_bot_qrcode?bot_type=${encodeURIComponent(ILINK_BOT_TYPE)}`, {
    method: "POST",
    body: { local_token_list: localTokenList },
    timeoutMs: 15_000,
  });
  const qrcode = String(payload.qrcode || "").trim();
  const qrPayload = String(payload.qrcode_img_content || "").trim();
  if (!qrcode || !qrPayload) throw new Error("iLink 未返回有效二维码");
  const qrImageUrl = await QRCode.toDataURL(qrPayload, { width: 320, margin: 2, errorCorrectionLevel: "M" });
  return { qrcode, qrPayload, qrImageUrl };
}

async function pollLoginStatus(login) {
  let endpoint = `ilink/bot/get_qrcode_status?qrcode=${encodeURIComponent(login.qrcode)}`;
  if (login.verifyCode) endpoint += `&verify_code=${encodeURIComponent(login.verifyCode)}`;
  return ilinkRequest(login.currentBaseUrl, endpoint, {
    method: "GET",
    timeoutMs: LOGIN_LONG_POLL_MS,
  });
}

function clearLoginTimer() {
  if (loginTimer) {
    clearTimeout(loginTimer);
    loginTimer = null;
  }
}

function scheduleLoginPoll(delayMs = 1000) {
  clearLoginTimer();
  loginTimer = setTimeout(() => {
    loginTimer = null;
    void pollLoginTick();
  }, Math.max(0, delayMs));
}

async function refreshLoginQr(login) {
  const next = await fetchLoginQr();
  login.qrcode = next.qrcode;
  login.qrPayload = next.qrPayload;
  login.qrImageUrl = next.qrImageUrl;
  login.startedAt = Date.now();
  login.verifyCode = "";
  state.qrCodeUrl = next.qrImageUrl;
  state.qrImageUrl = next.qrImageUrl;
  state.status = "qr_ready";
  state.message = "二维码已更新，请重新扫码";
  state.lastError = "";
}

async function completeLogin(login, payload) {
  const accountId = String(payload.ilink_bot_id || "").trim();
  const token = String(payload.bot_token || "").trim();
  if (!accountId || !token) throw new Error("iLink 登录成功但未返回账号凭据");
  credentials = {
    accountId,
    id: safeAccountId(accountId),
    token,
    baseUrl: String(payload.baseurl || login.currentBaseUrl || ILINK_BASE_URL).trim().replace(/\/+$/, ""),
    userId: String(payload.ilink_user_id || "").trim(),
    syncBuf: "",
    contextTokens: {},
  };
  contextTokens.clear();
  persistCredentials();
  activeLogin = null;
  clearLoginTimer();
  state.connected = true;
  state.status = "connected";
  state.accountId = accountId;
  state.nickname = "个人微信";
  state.alias = accountId;
  state.qrCodeUrl = "";
  state.qrImageUrl = "";
  state.sessionKey = "";
  state.driverKind = "ilink";
  state.messageTransportReady = true;
  state.startedAt = new Date().toISOString();
  state.lastError = "";
  state.message = "个人微信已连接";
  await startMonitor();
}

async function pollLoginTick() {
  const login = activeLogin;
  if (!login || Date.now() - login.startedAt >= LOGIN_TTL_MS) {
    if (login) {
      activeLogin = null;
      state.status = "login_expired";
      state.message = "二维码已过期，请重新连接";
      state.qrCodeUrl = "";
      state.qrImageUrl = "";
    }
    return;
  }
  const generation = login.generation;
  try {
    const payload = await pollLoginStatus(login);
    if (activeLogin !== login || generation !== loginGeneration) return;
    switch (payload.status) {
      case "wait":
        state.status = "qr_ready";
        state.message = "请使用手机微信扫描二维码";
        state.lastError = "";
        scheduleLoginPoll(1000);
        return;
      case "scaned":
        login.verifyCode = "";
        state.status = "scanned";
        state.message = "已扫码，请在手机上确认登录";
        state.lastError = "";
        scheduleLoginPoll(1000);
        return;
      case "need_verifycode":
        login.verifyCode = "";
        state.status = "verify_required";
        state.message = "请输入手机微信显示的数字";
        state.lastError = "";
        return;
      case "scaned_but_redirect": {
        const host = String(payload.redirect_host || "").trim();
        if (host) login.currentBaseUrl = `https://${host}`;
        scheduleLoginPoll(1000);
        return;
      }
      case "expired":
        if (login.refreshCount >= MAX_QR_REFRESH) {
          activeLogin = null;
          state.status = "login_expired";
          state.connected = false;
          state.message = "二维码多次失效，请重新连接";
          state.qrCodeUrl = "";
          state.qrImageUrl = "";
          return;
        }
        login.refreshCount += 1;
        await refreshLoginQr(login);
        if (activeLogin === login) scheduleLoginPoll(1000);
        return;
      case "verify_code_blocked":
        activeLogin = null;
        state.status = "login_error";
        state.connected = false;
        state.message = "验证码多次错误，请稍后重新连接";
        state.lastError = state.message;
        state.qrCodeUrl = "";
        state.qrImageUrl = "";
        return;
      case "binded_redirect":
        activeLogin = null;
        state.status = "login_error";
        state.connected = false;
        state.message = "该微信账号已绑定其他实例，请先解除原绑定后重试";
        state.lastError = state.message;
        state.qrCodeUrl = "";
        state.qrImageUrl = "";
        return;
      case "confirmed":
        await completeLogin(login, payload);
        return;
      default:
        state.message = `等待登录状态：${payload.status || "unknown"}`;
        scheduleLoginPoll(1000);
    }
  } catch (error) {
    if (activeLogin !== login || generation !== loginGeneration) return;
    state.lastError = error instanceof Error ? error.message : String(error);
    state.message = "二维码状态查询失败，正在重试";
    scheduleLoginPoll(2000);
  }
}

async function startLogin(force = false) {
  if (activeLogin && !force && Date.now() - activeLogin.startedAt < LOGIN_TTL_MS) {
    return {
      status: state.status,
      qrCodeUrl: activeLogin.qrImageUrl,
      qrImageUrl: activeLogin.qrImageUrl,
      sessionKey: activeLogin.sessionKey,
    };
  }
  loginGeneration += 1;
  clearLoginTimer();
  const next = await fetchLoginQr();
  activeLogin = {
    sessionKey: randomUUID(),
    qrcode: next.qrcode,
    qrPayload: next.qrPayload,
    qrImageUrl: next.qrImageUrl,
    currentBaseUrl: ILINK_BASE_URL,
    startedAt: Date.now(),
    refreshCount: 0,
    verifyCode: "",
    generation: loginGeneration,
  };
  state.status = "qr_ready";
  state.connected = false;
  state.qrCodeUrl = next.qrImageUrl;
  state.qrImageUrl = next.qrImageUrl;
  state.sessionKey = activeLogin.sessionKey;
  state.message = "请使用手机微信扫描二维码";
  state.lastError = "";
  scheduleLoginPoll(1000);
  return {
    status: state.status,
    qrCodeUrl: state.qrCodeUrl,
    qrImageUrl: state.qrImageUrl,
    sessionKey: state.sessionKey,
  };
}

function verifyLoginCode(code) {
  if (!activeLogin || state.status !== "verify_required") throw new Error("当前没有等待验证码的登录");
  activeLogin.verifyCode = String(code || "").trim();
  if (!activeLogin.verifyCode) throw new Error("请输入验证码");
  state.status = "scanned";
  state.message = "正在验证手机显示的数字";
  scheduleLoginPoll(0);
}

function setContextToken(peerId, token) {
  if (!peerId || !token) return;
  contextTokens.set(`${credentials?.id || "default"}:${peerId}`, token);
  pruneMap(contextTokens, 2000);
  persistCredentials();
}

function getContextToken(peerId) {
  return contextTokens.get(`${credentials?.id || "default"}:${peerId}`) || "";
}

function messageText(itemList) {
  const parts = [];
  for (const item of Array.isArray(itemList) ? itemList : []) {
    if (Number(item?.type) === 1 && item?.text_item?.text != null) {
      parts.push(String(item.text_item.text));
    } else if (Number(item?.type) === 3 && item?.voice_item?.text) {
      parts.push(String(item.voice_item.text));
    }
  }
  return parts.join("").trim();
}

function resolveMessageId(message) {
  if (message?.message_id) return String(message.message_id);
  const value = JSON.stringify({
    from: message?.from_user_id || "",
    to: message?.to_user_id || "",
    created: message?.create_time_ms || 0,
    type: message?.message_type || 0,
    items: message?.item_list || [],
  });
  return `fallback-${createHash("sha256").update(value).digest("hex")}`;
}

async function forwardInbound(message) {
  if (Number(message?.message_type) === 2) return { ignored: true, reason: "outbound" };
  const peerId = String(message?.from_user_id || "").trim();
  const text = messageText(message?.item_list);
  if (!peerId || !text) return { ignored: true, reason: "unsupported message" };
  const messageId = resolveMessageId(message);
  const dedupeKey = `${peerId}:${messageId}`;
  if (seenInbound.has(dedupeKey)) return { ignored: true, reason: "duplicate" };
  seenInbound.set(dedupeKey, Date.now());
  pruneMap(seenInbound);
  setContextToken(peerId, String(message?.context_token || "").trim());
  const accountId = credentials?.accountId || "wechat-personal";
  const convKey = conversationKey(accountId, peerId);
  const payload = {
    channelId: CHANNEL_ID,
    accountId,
    conversationId: convKey,
    peerId,
    messageId,
    contentType: "text",
    text,
  };
  const headers = {
    "Content-Type": "application/json",
    "X-Amitia-Extension-ID": EXTENSION_ID,
    "X-Amitia-Module-ID": MODULE_ID,
  };
  if (SERVICE_AUTH_TOKEN) headers.Authorization = `Bearer ${SERVICE_AUTH_TOKEN}`;
  const response = await fetch(`${CORE_URL}/api/channels/inbound`, {
    method: "POST",
    headers,
    body: JSON.stringify(payload),
    signal: AbortSignal.timeout(180_000),
  });
  const responseText = await response.text();
  if (!response.ok) throw new Error(`Amitia Core inbound HTTP ${response.status}: ${responseText.slice(0, 240)}`);
  let responsePayload = {};
  if (responseText) {
    try {
      responsePayload = JSON.parse(responseText);
    } catch {
      responsePayload = { raw: responseText };
    }
  }
  const createdAt = Number(message?.create_time_ms || 0) > 0
    ? new Date(Number(message.create_time_ms)).toISOString()
    : new Date().toISOString();
  state.messageCount += 1;
  state.lastInboundAt = createdAt;
  recordMessage(convKey, peerId, "user", text, createdAt);
  return { ignored: false, response: responsePayload };
}

async function getUpdates(abortSignal) {
  try {
    return await ilinkRequest(credentials.baseUrl, "ilink/bot/getupdates", {
      method: "POST",
      token: credentials.token,
      body: {
        get_updates_buf: credentials.syncBuf || "",
        base_info: baseInfo(),
      },
      timeoutMs: 35_000,
    });
  } catch (error) {
    if (abortSignal?.aborted || error?.name === "AbortError" || error?.name === "TimeoutError" || String(error?.message || "").includes("超时")) {
      return { ret: 0, msgs: [], get_updates_buf: credentials.syncBuf || "" };
    }
    throw error;
  }
}

async function notifyStart() {
  if (!credentials) return;
  try {
    await ilinkRequest(credentials.baseUrl, "ilink/bot/msg/notifystart", {
      method: "POST",
      token: credentials.token,
      body: { base_info: baseInfo() },
      timeoutMs: 10_000,
    });
  } catch {}
}

async function notifyStop() {
  if (!credentials) return;
  try {
    await ilinkRequest(credentials.baseUrl, "ilink/bot/msg/notifystop", {
      method: "POST",
      token: credentials.token,
      body: { base_info: baseInfo() },
      timeoutMs: 10_000,
    });
  } catch {}
}

function sleep(ms, signal) {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(resolve, ms);
    if (!signal) return;
    const onAbort = () => {
      clearTimeout(timer);
      reject(new Error("aborted"));
    };
    if (signal.aborted) onAbort();
    else signal.addEventListener("abort", onAbort, { once: true });
  });
}

async function monitorLoop(signal) {
  await notifyStart();
  let failures = 0;
  while (!signal.aborted && credentials) {
    try {
      const payload = await getUpdates(signal);
      if (signal.aborted) break;
      if (payload?.ret !== undefined && Number(payload.ret) !== 0) {
        throw new Error(`getUpdates ret=${payload.ret} errcode=${payload.errcode ?? ""} errmsg=${payload.errmsg ?? ""}`);
      }
      if (payload?.errcode !== undefined && Number(payload.errcode) !== 0) {
        throw new Error(`getUpdates errcode=${payload.errcode} errmsg=${payload.errmsg ?? ""}`);
      }
      failures = 0;
      if (payload.get_updates_buf) {
        credentials.syncBuf = String(payload.get_updates_buf);
        persistCredentials();
      }
      for (const message of Array.isArray(payload.msgs) ? payload.msgs : []) {
        try {
          await forwardInbound(message);
        } catch (error) {
          state.lastError = error instanceof Error ? error.message : String(error);
        }
      }
    } catch (error) {
      if (signal.aborted) break;
      failures += 1;
      state.lastError = error instanceof Error ? error.message : String(error);
      state.message = "个人微信消息轮询失败，正在重试";
      try {
        await sleep(failures >= 3 ? 30_000 : 2_000, signal);
      } catch {
        break;
      }
      if (failures >= 3) failures = 0;
    }
  }
  await notifyStop();
}

async function startMonitor() {
  if (!credentials || monitorTask) return;
  monitorAbort = new AbortController();
  state.connected = true;
  state.status = "connected";
  state.messageTransportReady = true;
  state.lastError = "";
  state.message = "个人微信已连接";
  const signal = monitorAbort.signal;
  monitorTask = monitorLoop(signal)
    .catch((error) => {
      if (!signal.aborted) {
        state.connected = false;
        state.status = "error";
        state.lastError = error instanceof Error ? error.message : String(error);
        state.message = "个人微信连接已中断";
      }
    })
    .finally(() => {
      monitorTask = null;
      monitorAbort = null;
    });
}

async function stopMonitor() {
  const task = monitorTask;
  const abort = monitorAbort;
  monitorTask = null;
  monitorAbort = null;
  abort?.abort();
  if (task) {
    await Promise.race([
      task.catch(() => {}),
      new Promise((resolve) => setTimeout(resolve, 2500)),
    ]);
  }
}

async function sendText(peerId, text, deliveryKey = "") {
  if (!credentials?.token) throw new Error("个人微信尚未连接");
  const target = String(peerId || "").trim();
  const content = String(text || "");
  if (!target || !content) throw new Error("toUserId and text are required");
  if (deliveryKey && delivered.has(deliveryKey)) return { duplicate: true };
  const contextToken = getContextToken(target);
  if (!contextToken) throw new Error("当前联系人缺少 iLink context_token，需等待联系人先发送消息后再回复");
  const clientId = `amitia-wechat:${Date.now()}-${randomUUID().slice(0, 8)}`;
  await ilinkRequest(credentials.baseUrl, "ilink/bot/sendmessage", {
    method: "POST",
    token: credentials.token,
    timeoutMs: 15_000,
    body: {
      msg: {
        from_user_id: "",
        to_user_id: target,
        client_id: clientId,
        message_type: 2,
        message_state: 2,
        context_token: contextToken,
        run_id: randomUUID(),
        item_list: [{ type: 1, text_item: { text: content } }],
      },
      base_info: baseInfo(),
    },
  });
  if (deliveryKey) {
    delivered.set(deliveryKey, Date.now());
    pruneMap(delivered);
  }
  state.replyCount += 1;
  state.lastOutboundAt = new Date().toISOString();
  recordMessage(conversationKey(credentials.accountId, target), target, "assistant", content);
  return { duplicate: false, clientId };
}

async function handleConnect(body = {}) {
  if (body.verifyCode) {
    verifyLoginCode(body.verifyCode);
    return { status: state.status, message: state.message };
  }
  if (credentials?.token && !body.force) {
    await startMonitor();
    return {
      status: "connected",
      accountId: credentials.accountId,
      message: "个人微信已连接",
    };
  }
  return startLogin(Boolean(body.force));
}

async function disconnect() {
  await stopMonitor();
  clearLoginTimer();
  loginGeneration += 1;
  activeLogin = null;
  clearCredentials();
  state.connected = false;
  state.status = "disconnected";
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
  if (!requireAuthorized(req, res)) return;
  const url = new URL(req.url, `http://${HOST}:${PORT}`);
  try {
    if (req.method === "GET" && url.pathname === "/api/health") {
      return json(res, 200, { success: true, status: state.status, accountId: state.accountId, protocol: state.protocol });
    }
    if (req.method === "GET" && url.pathname === "/api/status") {
      return json(res, 200, { success: true, data: { ...state } });
    }
    if (req.method === "GET" && url.pathname === "/api/config") {
      return json(res, 200, {
        mode: "ilink-personal",
        channelId: CHANNEL_ID,
        transportPort: PORT,
        defaultRolePolicy: "space_default_character",
        platforms: ["windows", "linux", "darwin"],
        localWechatRequired: false,
        managedClient: false,
        protocol: "ilink",
      });
    }
    if (req.method === "POST" && url.pathname === "/api/messages") {
      const body = await readJson(req);
      return json(res, 200, {
        success: true,
        data: readMessages(
          String(body.conversationId || ""),
          Math.max(1, Math.min(1000, Number(body.limit || 200))),
          Math.max(0, Number(body.offset || 0)),
        ),
      });
    }
    if (req.method === "POST" && url.pathname === "/api/connect") {
      const body = await readJson(req);
      const result = await handleConnect(body || {});
      return json(res, 200, { success: true, data: result, message: state.message });
    }
    if (req.method === "POST" && url.pathname === "/api/login/verify") {
      const body = await readJson(req);
      verifyLoginCode(body.code || "");
      return json(res, 200, { success: true, data: { status: state.status, message: state.message } });
    }
    if (req.method === "POST" && url.pathname === "/api/disconnect") {
      await disconnect();
      return json(res, 200, { success: true, disconnected: true });
    }
    if (req.method === "POST" && url.pathname === "/api/send") {
      const body = await readJson(req);
      const headerKey = String(req.headers["idempotency-key"] || "");
      const effectiveKey = String(body.deliveryKey || headerKey || "");
      const result = await sendText(String(body.toUserId || ""), String(body.text || ""), effectiveKey);
      return json(res, 200, { success: true, accepted: true, duplicate: result.duplicate });
    }
    if (req.method === "POST" && (url.pathname === "/api/send-image" || url.pathname === "/api/send-voice")) {
      return json(res, 501, { success: false, message: "个人微信插件暂未声明图片/语音发送能力" });
    }
    return json(res, 404, { success: false, message: "not found" });
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    state.lastError = message;
    return json(res, 500, { success: false, message });
  }
}

const mainServer = http.createServer((req, res) => void handleMain(req, res));
mainServer.listen(PORT, HOST, () => {
  console.log(`[wechat-personal] iLink provider listening on http://${HOST}:${PORT}`);
  if (credentials) {
    void startMonitor();
  }
});

async function shutdown() {
  await stopMonitor();
  clearLoginTimer();
  await new Promise((resolve) => mainServer.close(resolve));
  process.exit(0);
}

process.on("SIGTERM", () => void shutdown());
process.on("SIGINT", () => void shutdown());

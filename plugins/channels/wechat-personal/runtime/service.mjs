import http from "node:http";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { spawn } from "node:child_process";
import { createHash, randomUUID, timingSafeEqual } from "node:crypto";

const CHANNEL_ID = "wechat_personal";
const EXTENSION_ID = "com.amitia/channel-wechat-personal";
const MODULE_ID = "wechat-personal-channel-service";
const HOST = "127.0.0.1";
const PORT = 19878;
const RECEIVER_PORT = 9999;
const CORE_URL = String(process.env.AMITIA_CORE_URL || "").trim().replace(/\/+$/, "");
const SERVICE_AUTH_TOKEN = String(process.env.AMITIA_SERVICE_AUTH_TOKEN || "").trim();
const SERVICE_AUTH_VERSION = String(process.env.AMITIA_SERVICE_AUTH_VERSION || "").trim();
const DEV_ALLOW_UNAUTHENTICATED = process.env.AMITIA_WECHAT_DEV_ALLOW_UNAUTHENTICATED === "1";
const EXTERNAL_COMPAT = process.env.AMITIA_WECHAT_EXTERNAL_DRIVER_COMPAT === "1";
const EXTERNAL_UNAUTH_CALLBACK = process.env.AMITIA_WECHAT_EXTERNAL_CALLBACK_UNAUTHENTICATED === "1";
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
const TEMP_DIR = process.env.AMITIA_TEMP_DIR || path.join(process.cwd(), ".wechat-personal-temp");
fs.mkdirSync(TEMP_DIR, { recursive: true });

const state = {
  status: "disconnected",
  connected: false,
  running: true,
  accountId: "",
  nickname: "",
  alias: "",
  avatar: "",
  qrCodeUrl: "",
  driverKind: "none",
  driverBaseUrl: "",
  driverVersion: "",
  messageCount: 0,
  replyCount: 0,
  startedAt: new Date().toISOString(),
  lastError: "",
  message: "等待连接个人微信",
  managedWechatPid: 0,
  platform: process.platform,
  architecture: process.arch,
  nativeAgent: "stopped",
  nativeAgentPath: "",
  nativeClientFound: false,
  nativeClientRunning: false,
  nativeClientVersion: "",
  nativeClientStrategy: "",
  nativeClientPath: "",
  nativeClientMessage: "",
  nativeDriverKind: "",
  nativeDriverVersion: "",
  nativeDriverClientVersion: "",
  nativeDriverVersionVerified: false,
  nativeDriverMessage: "",
  linuxPreloadAttached: false,
  nativeDriverAvailable: false,
  nativeDriverEndpoint: "",
  nativeDriverCapabilities: {},
  messageTransportReady: false,
};

let loginTimer = null;
let nativeEventTimer = null;
const seenInbound = new Map();
const delivered = new Map();
const messageHistory = new Map();

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

function pruneMap(map, max = 1000) {
  if (map.size <= max) return;
  const remove = map.size - max;
  let i = 0;
  for (const key of map.keys()) {
    map.delete(key);
    if (++i >= remove) break;
  }
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

async function fetchJson(url, options = {}, timeoutMs = 5000) {
  const signal = AbortSignal.timeout(timeoutMs);
  const response = await fetch(url, { ...options, signal });
  const text = await response.text();
  let data = {};
  if (text) {
    try { data = JSON.parse(text); } catch { data = { raw: text }; }
  }
  if (!response.ok) throw new Error(`HTTP ${response.status}: ${text.slice(0, 240)}`);
  return data;
}

function normalizeStatusPayload(data) {
  if (!data || typeof data !== "object") return false;
  if (data.data && typeof data.data === "object") data = data.data;
  if (typeof data.logged === "boolean") return data.logged;
  if (typeof data.connected === "boolean") return data.connected;
  if (data.IsLogin !== undefined) return Number(data.IsLogin) === 1;
  if (data.isLogin !== undefined) return Number(data.isLogin) === 1 || data.isLogin === true;
  if (typeof data.status === "string") return ["connected", "online", "logged_in", "logged"].includes(data.status.toLowerCase());
  return false;
}

async function probeAmitiaDriver(base = "http://127.0.0.1:19879") {
  try {
    const info = await fetchJson(`${base}/v1/health`);
    return { kind: "amitia", base, info };
  } catch { return null; }
}

async function probeHeroDriver(base = "http://127.0.0.1:8888") {
  try {
    const info = await fetchJson(`${base}/status`);
    if (info?.matched === false) throw new Error("微信版本与 Hook Driver 不匹配");
    return { kind: "hero", base, info };
  } catch { return null; }
}

async function probeAixedDriver(base = "http://127.0.0.1:30001") {
  try {
    const info = await fetchJson(`${base}/QueryDB/status`);
    return { kind: "aixed", base, info };
  } catch { return null; }
}

async function probeExternalCompatibilityDriver(preferred = "") {
  if (!EXTERNAL_COMPAT) return null;
  const candidates = [];
  if (preferred) {
    candidates.push({ kind: "hero", fn: () => probeHeroDriver(preferred) });
    candidates.push({ kind: "aixed", fn: () => probeAixedDriver(preferred) });
  }
  candidates.push(
    { kind: "hero", fn: () => probeHeroDriver() },
    { kind: "aixed", fn: () => probeAixedDriver() },
  );
  for (const candidate of candidates) {
    const result = await candidate.fn();
    if (result) return result;
  }
  return null;
}

class NativeCompanionManager {
  constructor() {
    this.child = null;
    this.pending = new Map();
    this.buffer = "";
    this.seq = 0;
  }

  declaredCompanions() {
    const version = String(process.env.AMITIA_NATIVE_COMPANIONS_VERSION || "").trim();
    const raw = String(process.env.AMITIA_NATIVE_COMPANIONS || "").trim();
    if (!raw) return [];
    if (version && version !== "1") {
      throw new Error(`Unsupported Native Companion contract version: ${version}`);
    }
    try {
      const value = JSON.parse(raw);
      return Array.isArray(value) ? value : [];
    } catch (error) {
      throw new Error(`Invalid AMITIA_NATIVE_COMPANIONS payload: ${error.message}`);
    }
  }

  verifyDeclaredFile(item) {
    if (!item?.path || !item?.sha256 || !fs.existsSync(item.path)) return false;
    const actual = createHash("sha256").update(fs.readFileSync(item.path)).digest("hex");
    return actual === String(item.sha256).toLowerCase();
  }

  resolveCompanions() {
    const declared = this.declaredCompanions().filter((item) => this.verifyDeclaredFile(item));
    const byId = (id) => declared.find((item) => item.id === id);
    if (process.platform === "win32" && process.arch === "x64") {
      return {
        agent: byId("wechat-agent-windows-x64"),
        driverLibrary: byId("wechat-driver-windows-x64"),
      };
    }
    if (process.platform === "linux" && process.arch === "x64") {
      return {
        agent: byId("wechat-agent-linux-x64"),
        driverLibrary: byId("wechat-driver-linux-x64") || byId("wechat-preload-linux-x64"),
      };
    }
    return { agent: undefined, driverLibrary: undefined };
  }

  async ensureStarted() {
    if (this.child && !this.child.killed) return true;
    const companions = this.resolveCompanions();
    const agent = companions.agent;
    state.nativeAgentPath = String(agent?.path || "");
    if (!agent?.path || agent.executable !== true) {
      state.nativeAgent = "missing";
      throw new Error(`宿主未声明当前平台可执行 Native Companion: ${process.platform}/${process.arch}`);
    }
    this.child = spawn(agent.path, Array.isArray(agent.args) ? agent.args.map(String) : [], {
      cwd: MODULE_DIR,
      stdio: ["pipe", "pipe", "pipe"],
      windowsHide: true,
      env: { ...process.env },
    });
    state.nativeAgent = "running";
    this.child.stdout.setEncoding("utf8");
    this.child.stdout.on("data", (chunk) => this.onStdout(chunk));
    this.child.stderr.setEncoding("utf8");
    this.child.stderr.on("data", (chunk) => {
      const line = String(chunk || "").trim();
      if (line) console.warn(`[wechat-personal/native] ${line}`);
    });
    this.child.once("exit", (code, signal) => {
      state.nativeAgent = "stopped";
      state.nativeClientRunning = false;
      for (const [, item] of this.pending) item.reject(new Error(`Native Agent exited: code=${code} signal=${signal || ""}`));
      this.pending.clear();
      this.child = null;
    });
    await new Promise((resolve) => setTimeout(resolve, 40));
    if (!this.child || this.child.exitCode !== null) throw new Error("Native Agent 启动失败");
    return true;
  }

  onStdout(chunk) {
    this.buffer += chunk;
    for (;;) {
      const idx = this.buffer.indexOf("\n");
      if (idx < 0) break;
      const line = this.buffer.slice(0, idx).trim();
      this.buffer = this.buffer.slice(idx + 1);
      if (!line) continue;
      let payload;
      try { payload = JSON.parse(line); } catch { continue; }
      const item = this.pending.get(String(payload.id || ""));
      if (!item) continue;
      this.pending.delete(String(payload.id));
      if (payload.ok === false) item.reject(new Error(payload.error || "Native Agent request failed"));
      else item.resolve(payload);
    }
  }

  async call(op, extra = {}, timeoutMs = 5000) {
    await this.ensureStarted();
    const id = `rpc-${Date.now()}-${++this.seq}`;
    const companions = this.resolveCompanions();
    const request = { id, op, ...extra };
    if (op === "start" && companions.driverLibrary?.path) request.driverLibraryPath = companions.driverLibrary.path;
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        reject(new Error(`Native Agent timeout: ${op}`));
      }, timeoutMs);
      this.pending.set(id, {
        resolve: (value) => { clearTimeout(timer); resolve(value); },
        reject: (error) => { clearTimeout(timer); reject(error); },
      });
      this.child.stdin.write(`${JSON.stringify(request)}\n`);
    });
  }

  applyClient(payload) {
    const client = payload?.client || {};
    state.managedWechatPid = Number(client.pid || 0);
    state.nativeClientFound = Boolean(client.found);
    state.nativeClientRunning = Boolean(client.running);
    state.nativeClientVersion = String(client.version || "");
    state.nativeClientStrategy = String(client.strategy || "");
    state.nativeClientPath = String(client.path || "");
    state.nativeClientMessage = String(client.message || "");
    if (process.platform === "linux" && state.managedWechatPid > 0) {
      state.linuxPreloadAttached = fs.existsSync(`/tmp/amitia-wechat-hook-${state.managedWechatPid}.sock`);
    } else {
      state.linuxPreloadAttached = false;
    }
    return client;
  }

  async probe() {
    const payload = await this.call("probe");
    this.applyDriver(payload?.driver);
    return this.applyClient(payload);
  }

  async startClient() {
    const payload = await this.call("start", {}, 10000);
    this.applyDriver(payload?.driver);
    return this.applyClient(payload);
  }

  async hideClient() {
    try { return this.applyClient(await this.call("hide")); }
    catch (error) {
      if (process.platform !== "linux") throw error;
      return { warning: error.message };
    }
  }

  applyDriver(driver = {}) {
    state.nativeDriverAvailable = Boolean(driver?.available);
    state.nativeDriverKind = String(driver?.kind || "");
    state.nativeDriverVersion = String(driver?.version || "");
    state.nativeDriverClientVersion = String(driver?.clientVersion || "");
    state.nativeDriverVersionVerified = Boolean(driver?.versionVerified);
    state.nativeDriverMessage = String(driver?.message || "");
    state.nativeDriverEndpoint = String(driver?.endpoint || "");
    state.nativeDriverCapabilities = driver?.capabilities && typeof driver.capabilities === "object" ? { ...driver.capabilities } : {};
    if (process.platform === "linux") state.linuxPreloadAttached = Boolean(state.nativeDriverCapabilities?.attached);
    return driver;
  }

  async probeDriver() {
    const payload = await this.call("driver.probe");
    this.applyClient(payload);
    return this.applyDriver(payload?.driver);
  }

  async driverCall(driverOp, payload = {}, timeoutMs = 10000) {
    const response = await this.call("driver.call", { driverOp, payload }, timeoutMs);
    this.applyClient(response);
    const raw = response?.data;
    if (raw == null) return {};
    if (typeof raw === "object") return raw;
    try { return JSON.parse(String(raw)); } catch { return { raw }; }
  }

  stop() {
    if (!this.child) return;
    try { this.child.stdin.end(); } catch {}
    try { this.child.kill(); } catch {}
    this.child = null;
    state.nativeAgent = "stopped";
  }
}

const nativeCompanion = new NativeCompanionManager();

async function ensureManagedWechat() {
  try {
    let client = await nativeCompanion.probe();
    // start is idempotent. On Windows it also gives the Agent a chance to attach
    // a host-verified Driver Library to an already-running official client.
    if (!client.running || process.platform === "win32") client = await nativeCompanion.startClient();
    // Do not hide the official client before QR acquisition. UI-derived login
    // drivers need a live render target. The window is hidden immediately
    // after the QR is captured (or when an authenticated session is detected).
    return client;
  } catch (error) {
    state.lastError = `Native Companion: ${error.message}`;
    return null;
  }
}

async function driverLoginStatus() {
  try {
    let data;
    if (state.driverKind === "amitia-native") data = await nativeCompanion.driverCall("login.status");
    else if (state.driverKind === "hero") data = await fetchJson(`${state.driverBaseUrl}/login/state`);
    else if (state.driverKind === "aixed") data = await fetchJson(`${state.driverBaseUrl}/QueryDB/status`);
    else return false;
    return normalizeStatusPayload(data);
  } catch {
    return false;
  }
}

async function driverSelf() {
  try {
    let data;
    if (state.driverKind === "amitia-native") data = await nativeCompanion.driverCall("account.self");
    else if (state.driverKind === "hero") data = await fetchJson(`${state.driverBaseUrl}/self/info`);
    else if (state.driverKind === "aixed") data = await fetchJson(`${state.driverBaseUrl}/GetSelfProfile`, { method: "POST", headers: { "Content-Type": "application/json" }, body: "{}" });
    else return {};
    if (data?.data && typeof data.data === "object") data = data.data;
    if (data?.result && typeof data.result === "object" && !Array.isArray(data.result)) data = data.result;
    return data || {};
  } catch { return {}; }
}

function selfFromPayload(data) {
  return {
    accountId: String(data.wxid || data.accountId || data.userName || data.username || ""),
    nickname: String(data.nickname || data.nickName || data.name || ""),
    alias: String(data.alias || data.wxcount || data.wechatId || ""),
    avatar: String(data.avatar || data.avatarUrl || data.big_head_url || ""),
  };
}

async function refreshAccountState() {
  const logged = await driverLoginStatus();
  if (!logged) {
    state.connected = false;
    if (state.status !== "qr_ready") state.status = "waiting_login";
    return false;
  }
  const profile = selfFromPayload(await driverSelf());
  state.accountId = profile.accountId || state.accountId;
  state.nickname = profile.nickname || state.nickname;
  state.alias = profile.alias || state.alias;
  state.avatar = profile.avatar || state.avatar;
  state.connected = true;
  state.messageTransportReady = state.driverKind !== "amitia-native" || Boolean(
    state.nativeDriverVersionVerified &&
    state.nativeDriverCapabilities?.receiveText &&
    state.nativeDriverCapabilities?.sendText
  );
  state.status = state.messageTransportReady ? "connected" : "connected_limited";
  state.qrCodeUrl = "";
  state.message = state.messageTransportReady
    ? "个人微信已连接"
    : "个人微信已登录，但当前平台/版本仅完成登录适配，消息收发仍处于 fail-closed";
  state.lastError = "";
  return true;
}

function startLoginPolling() {
  if (loginTimer) clearInterval(loginTimer);
  loginTimer = setInterval(async () => {
    try {
      const connected = await refreshAccountState();
      if (connected && loginTimer) {
        clearInterval(loginTimer);
        loginTimer = null;
      }
    } catch {}
  }, 1800);
  loginTimer.unref?.();
}

function stopNativeEventPolling() {
  if (nativeEventTimer) {
    clearInterval(nativeEventTimer);
    nativeEventTimer = null;
  }
}

function startNativeEventPolling() {
  stopNativeEventPolling();
  if (
    state.driverKind !== "amitia-native" ||
    !state.nativeDriverVersionVerified ||
    !state.nativeDriverCapabilities?.receiveText
  ) return;
  let busy = false;
  nativeEventTimer = setInterval(async () => {
    if (busy) return;
    busy = true;
    try {
      const payload = await nativeCompanion.driverCall("events.poll", { limit: 20 }, 1200);
      const events = Array.isArray(payload) ? payload : Array.isArray(payload?.events) ? payload.events : [];
      for (const event of events) {
        try { await forwardInbound(event); } catch (error) { console.warn(`[wechat-personal/native-event] ${error.message}`); }
      }
    } catch {} finally {
      busy = false;
    }
  }, 350);
  nativeEventTimer.unref?.();
}

async function getDriverQr() {
  if (state.driverKind === "amitia-native") {
    const payload = await nativeCompanion.driverCall("login.qr", {}, 15000);
    const data = payload?.data || payload;
    const image = data.imageDataUrl || data.qrImageUrl || data.image || "";
    const qrUrl = data.qrUrl || data.url || "";
    if (image) return String(image);
    if (qrUrl) return String(qrUrl);
    throw new Error("Amitia Native Driver 未返回二维码");
  }
  if (!state.driverBaseUrl) throw new Error("Native Driver 未连接");
  if (state.driverKind === "hero") {
    const qrFile = path.join(TEMP_DIR, "wechat-personal-login-qr.png");
    try { fs.rmSync(qrFile, { force: true }); } catch {}
    const payload = await fetchJson(`${state.driverBaseUrl}/qr/url?path=${encodeURIComponent(qrFile)}`, {}, 10000);
    const data = payload?.data || payload;
    if (fs.existsSync(qrFile)) {
      const buffer = fs.readFileSync(qrFile);
      if (buffer.length > 32) return `data:image/png;base64,${buffer.toString("base64")}`;
    }
    const direct = data.imageDataUrl || data.qrImageUrl || data.image || "";
    if (direct) return String(direct);
    throw new Error("Hook 已返回登录 URL，但未生成二维码 PNG；请使用支持 /qr/url?path= 的 Driver");
  }
  throw new Error("当前 Hook Driver 不提供插件内二维码；建议使用 Amitia Native Driver 或 hero-compatible Driver");
}

async function sendText(peerId, text, deliveryKey = "") {
  if (!state.driverBaseUrl) throw new Error("Native Driver 未连接");
  if (!peerId || !text) throw new Error("toUserId and text are required");
  if (deliveryKey && delivered.has(deliveryKey)) return { duplicate: true };
  let result;
  if (state.driverKind === "amitia-native") {
    if (!state.nativeDriverVersionVerified || !state.nativeDriverCapabilities?.sendText) {
      throw new Error("当前 Native Driver 未通过客户端版本验证或未声明文本发送能力");
    }
    result = await nativeCompanion.driverCall("messages.send_text", { peerId, text, deliveryKey }, 15000);
  } else if (state.driverKind === "hero") {
    result = await fetchJson(`${state.driverBaseUrl}/send`, {
      method: "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ wxid: peerId, content: text }),
    }, 15000);
    if (result?.code !== undefined && Number(result.code) !== 0) throw new Error(result.msg || "发送失败");
  } else if (state.driverKind === "aixed") {
    result = await fetchJson(`${state.driverBaseUrl}/SendTextMsg`, {
      method: "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ wxidorgid: peerId, msg: text }),
    }, 15000);
    if (result?.ret !== undefined && Number(result.ret) !== 0) throw new Error(result.retmsg || "发送失败");
  } else {
    throw new Error("未知 Driver");
  }
  if (deliveryKey) {
    delivered.set(deliveryKey, Date.now());
    pruneMap(delivered);
  }
  state.replyCount += 1;
  recordMessage(`wechat-personal-${state.accountId || "default"}-${peerId}`.replace(/[^a-zA-Z0-9_@.-]/g, "_"), peerId, "assistant", text);
  return { duplicate: false, result };
}

function extractInbound(raw) {
  const body = raw?.data && typeof raw.data === "object" ? raw.data : raw;
  const msg = body?.message && typeof body.message === "object" ? body.message : body;
  const type = Number(msg.msgtype ?? msg.msgType ?? msg.type ?? msg.messageType ?? 1);
  const roomId = String(msg.roomid || msg.roomId || msg.chatroom || msg.chatRoomId || "");
  const from = String(msg.wxid || msg.fromWxid || msg.fromUser || msg.sender || msg.senderId || msg.from || "");
  const senderInRoom = String(msg.senderWxid || msg.memberWxid || msg.actualSender || msg.sender || "");
  const peerId = roomId || from;
  const senderId = roomId ? (senderInRoom || from) : from;
  const text = String(msg.content ?? msg.text ?? msg.msg ?? msg.message ?? "");
  const messageId = String(msg.msgid ?? msg.msgId ?? msg.messageId ?? msg.newMsgId ?? raw?.id ?? randomUUID());
  const createdAt = msg.timestamp || msg.createTime || msg.createdAt || Date.now();
  return { type, peerId, senderId, text, messageId, createdAt, raw: body };
}

async function forwardInbound(raw) {
  const item = extractInbound(raw);
  if (!item.peerId || !item.text) return { ignored: true, reason: "missing peer/text" };
  const dedupeKey = `${item.peerId}:${item.messageId}`;
  if (seenInbound.has(dedupeKey)) return { ignored: true, reason: "duplicate" };
  seenInbound.set(dedupeKey, Date.now());
  pruneMap(seenInbound);
  const accountId = state.accountId || "wechat-personal";
  const convKey = `wechat-personal-${accountId}-${item.peerId}`.replace(/[^a-zA-Z0-9_@.-]/g, "_");
  const payload = {
    channelId: CHANNEL_ID,
    accountId,
    conversationId: convKey,
    peerId: item.senderId || item.peerId,
    messageId: item.messageId,
    contentType: "text",
    text: item.text,
  };
  const headers = {
    "Content-Type": "application/json",
    "X-Amitia-Extension-ID": EXTENSION_ID,
    "X-Amitia-Module-ID": MODULE_ID,
  };
  if (SERVICE_AUTH_TOKEN) headers.Authorization = `Bearer ${SERVICE_AUTH_TOKEN}`;
  const response = await fetchJson(`${CORE_URL}/api/channels/inbound`, {
    method: "POST",
    headers,
    body: JSON.stringify(payload),
  }, 180000);
  state.messageCount += 1;
  const createdAt = Number.isFinite(Number(item.createdAt))
    ? new Date(Number(item.createdAt)).toISOString()
    : new Date().toISOString();
  recordMessage(convKey, item.peerId, "user", item.text, createdAt);
  return { ignored: false, response };
}

async function handleConnect(config = {}) {
  state.lastError = "";
  state.message = "正在启动并检测个人微信 Native Companion";
  state.driverKind = "none";
  state.driverBaseUrl = "";
  state.nativeDriverAvailable = false;
  state.nativeDriverCapabilities = {};
  stopNativeEventPolling();

  if (config.launchWechat !== false) await ensureManagedWechat();

  let nativeDriver = null;
  try { nativeDriver = await nativeCompanion.probeDriver(); } catch (error) { state.lastError = `Native Driver: ${error.message}`; }
  const caps = nativeDriver?.capabilities || {};
  const nativeLoginReady = Boolean(nativeDriver?.available && caps.loginStatus && caps.qr);

  if (nativeLoginReady) {
    state.driverKind = "amitia-native";
    state.driverBaseUrl = "native://companion";
    state.driverVersion = String(nativeDriver?.version || "native-v1");
    state.message = "Amitia Native Driver 已连接";
    startNativeEventPolling();
  } else {
    const preferred = typeof config.driverBaseUrl === "string" ? config.driverBaseUrl.trim() : "";
    const external = await probeExternalCompatibilityDriver(preferred);
    if (external) {
      state.driverKind = external.kind;
      state.driverBaseUrl = external.base;
      state.driverVersion = String(external.info?.version || external.info?.wxVersion || external.info?.expected || "");
      state.message = "已连接兼容 Driver（开发兼容模式）";
    } else {
      state.status = "driver_required";
      state.connected = false;
      state.messageTransportReady = false;
      state.message = "Native Companion 已启动，但当前微信版本没有已验证的消息 Driver";
      const capabilityText = JSON.stringify(caps);
      if (process.platform === "linux" && state.nativeClientRunning) {
        state.lastError = caps.attached
          ? `Linux preload 已附着，但版本适配器仍为 fail-closed。capabilities=${capabilityText}`
          : "Linux 微信正在运行，但 preload companion 未附着；如果微信早于插件启动，请完全退出微信后重新连接。";
      } else if (process.platform === "win32" && state.nativeClientRunning) {
        state.lastError = `Windows 微信已由 Amitia 托管，但尚未检测到与该进程匹配的 Amitia Hook companion。capabilities=${capabilityText}`;
      } else {
        state.lastError = state.lastError || "未检测到可运行的官方微信客户端。";
      }
      return { status: state.status, message: state.message, lastError: state.lastError, capabilities: caps };
    }
  }

  if (await refreshAccountState()) {
    if (process.platform === "win32" || process.platform === "linux") {
      try { await nativeCompanion.hideClient(); } catch {}
    }
    return { status: state.messageTransportReady ? "connected" : "connected_limited", accountId: state.accountId, nickname: state.nickname, driverKind: state.driverKind, capabilities: caps };
  }
  try {
    state.qrCodeUrl = await getDriverQr();
    if (process.platform === "win32" || process.platform === "linux") {
      try { await nativeCompanion.hideClient(); } catch {}
    }
    state.status = "qr_ready";
    state.message = "请使用准备交给 AI 的微信账号扫码";
    startLoginPolling();
    return { status: state.status, qrCodeUrl: state.qrCodeUrl, driverKind: state.driverKind, capabilities: caps };
  } catch (error) {
    state.status = "waiting_login";
    state.message = "Driver 已连接，但当前适配器无法提供插件内二维码";
    state.lastError = error.message;
    startLoginPolling();
    return { status: state.status, driverKind: state.driverKind, lastError: state.lastError, capabilities: caps };
  }
}

async function handleMain(req, res) {
  if (!requireAuthorized(req, res)) return;
  const url = new URL(req.url, `http://${HOST}:${PORT}`);
  try {
    if (req.method === "GET" && url.pathname === "/api/health") {
      return json(res, 200, { success: true, status: state.status, accountId: state.accountId, driverKind: state.driverKind });
    }
    if (req.method === "GET" && url.pathname === "/api/status") {
      if (state.driverKind !== "none") await refreshAccountState();
      return json(res, 200, { success: true, data: { ...state } });
    }
    if (req.method === "GET" && url.pathname === "/api/config") {
      return json(res, 200, {
        mode: "personal-wechat-managed-hook",
        channelId: CHANNEL_ID,
        transportPort: PORT,
        receiverPort: RECEIVER_PORT,
        defaultRolePolicy: "space_default_character",
        platforms: ["windows-x64", "linux-x64"],
        managedClient: true,
        nativeCompanion: state.nativeAgent,
        supportedDrivers: ["amitia-native-v1", "hero-compatible", "aixed-compatible"],
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
      return json(res, 200, { success: state.status !== "driver_required", data: result, message: state.message });
    }
    if (req.method === "POST" && url.pathname === "/api/disconnect") {
      if (loginTimer) { clearInterval(loginTimer); loginTimer = null; }
      stopNativeEventPolling();
      state.connected = false;
      state.messageTransportReady = false;
      state.status = "disconnected";
      state.qrCodeUrl = "";
      state.message = "已断开个人微信渠道";
      nativeCompanion.stop();
      state.managedWechatPid = 0;
      state.nativeClientRunning = false;
      state.linuxPreloadAttached = false;
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
      return json(res, 501, { success: false, message: "个人微信插件 1.5.0 暂未声明图片/语音发送能力" });
    }
    if (req.method === "POST" && ["/api/native/callback", "/message"].includes(url.pathname)) {
      const body = await readJson(req);
      const result = await forwardInbound(body);
      return json(res, 200, { success: true, ...result });
    }
    return json(res, 404, { success: false, message: "not found" });
  } catch (error) {
    state.lastError = error instanceof Error ? error.message : String(error);
    return json(res, 500, { success: false, message: state.lastError });
  }
}

const mainServer = http.createServer((req, res) => void handleMain(req, res));
mainServer.listen(PORT, HOST, () => {
  console.log(`[wechat-personal] provider service listening on http://${HOST}:${PORT}`);
});

// The legacy hero-compatible callback receiver is deliberately disabled in
// production. It lacks an authentication contract, so enabling it requires two
// explicit development flags. This prevents arbitrary local processes from
// injecting fake inbound messages into the default character pipeline.
let receiverServer = null;
if (EXTERNAL_COMPAT && EXTERNAL_UNAUTH_CALLBACK) {
  receiverServer = http.createServer(async (req, res) => {
    if (req.method === "POST" && req.url?.split("?")[0] === "/message") {
      try {
        const body = await readJson(req);
        const result = await forwardInbound(body);
        return json(res, 200, { success: true, ...result });
      } catch (error) {
        return json(res, 500, { success: false, message: error.message });
      }
    }
    if (req.method === "POST" && req.url?.split("?")[0] === "/log") {
      try { await readJson(req); } catch {}
      return json(res, 200, { success: true });
    }
    return json(res, 404, { success: false });
  });
  receiverServer.on("error", (error) => {
    console.warn(`[wechat-personal] development receiver ${RECEIVER_PORT} unavailable: ${error.message}`);
  });
  receiverServer.listen(RECEIVER_PORT, HOST, () => {
    console.warn(`[wechat-personal] UNSAFE development callback receiver enabled on http://${HOST}:${RECEIVER_PORT}`);
  });
}

async function shutdown() {
  if (loginTimer) clearInterval(loginTimer);
  stopNativeEventPolling();
  nativeCompanion.stop();
  const closers = [new Promise((resolve) => mainServer.close(resolve))];
  if (receiverServer) closers.push(new Promise((resolve) => receiverServer.close(resolve)));
  await Promise.allSettled(closers);
  process.exit(0);
}
process.on("SIGTERM", () => void shutdown());
process.on("SIGINT", () => void shutdown());

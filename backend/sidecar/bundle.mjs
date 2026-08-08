// src/index.ts
import Fastify from "fastify";
import cors from "@fastify/cors";

// src/config.ts
function envStr(key, fallback) {
  return process.env[key] || fallback;
}
var sidecarConfig = {
  mergeWindowMs: parseInt(process.env.MERGE_WINDOW_MS || "6000", 10),
  host: envStr("SIDECAR_HOST", "127.0.0.1"),
  port: parseInt(envStr("SIDECAR_PORT", "19876"), 10),
  // Core URL for forwarding incoming messages
  coreUrl: envStr("CORE_URL", "http://127.0.0.1:18899"),
  // Bridge API token for auth
  bridgeApiToken: (() => {
    const t = process.env.BRIDGE_API_TOKEN;
    if (!t || t === "change-me-bridge-token") {
      console.error("[Sidecar] \u5B89\u5168\u8B66\u544A: BRIDGE_API_TOKEN \u672A\u8BBE\u7F6E\u6216\u4ECD\u4E3A\u9ED8\u8BA4\u503C");
    }
    return t || "";
  })()
};

// src/openclaw-wechat.ts
import {
  startWeixinLoginWithQr,
  waitForWeixinLogin,
  DEFAULT_ILINK_BOT_TYPE
} from "@tencent-weixin/openclaw-weixin/dist/src/auth/login-qr.js";
import {
  getUpdates,
  sendMessage,
  getUploadUrl,
  notifyStart,
  notifyStop
} from "@tencent-weixin/openclaw-weixin/dist/src/api/api.js";
import {
  saveWeixinAccount,
  loadWeixinAccount,
  listIndexedWeixinAccountIds,
  registerWeixinAccountId,
  unregisterWeixinAccountId,
  clearWeixinAccount,
  DEFAULT_BASE_URL
} from "@tencent-weixin/openclaw-weixin/dist/src/auth/accounts.js";
import { normalizeAccountId } from "openclaw/plugin-sdk/account-id";
import { downloadAndDecryptBuffer } from "@tencent-weixin/openclaw-weixin/dist/src/cdn/pic-decrypt.js";
import { silkToWav } from "@tencent-weixin/openclaw-weixin/dist/src/media/silk-transcode.js";
import { resolveStateDir } from "@tencent-weixin/openclaw-weixin/dist/src/storage/state-dir.js";
import crypto2 from "node:crypto";
import fs2 from "node:fs";
import path2 from "node:path";
import QRCode from "qrcode";

// src/delivery-idempotency.ts
import crypto from "node:crypto";
import fs from "node:fs";
import path from "node:path";
var DeliveryIdempotencyStore = class {
  filePath;
  ledger = /* @__PURE__ */ new Map();
  inflight = /* @__PURE__ */ new Map();
  constructor(filePath) {
    this.filePath = filePath;
    this.load();
  }
  resolveClientId(deliveryKey) {
    const hash = crypto.createHash("sha256").update(deliveryKey).digest("hex");
    return `amitia:${hash.slice(0, 16)}`;
  }
  load() {
    try {
      if (!fs.existsSync(this.filePath)) {
        return;
      }
      const raw = fs.readFileSync(this.filePath, "utf-8");
      if (raw.trim() === "") {
        return;
      }
      const data = JSON.parse(raw);
      if (!data.entries || !Array.isArray(data.entries)) {
        this.backupCorrupt();
        return;
      }
      for (const entry of data.entries) {
        if (entry.deliveryKey && entry.clientId && entry.status) {
          this.ledger.set(entry.deliveryKey, entry);
        }
      }
    } catch {
      this.backupCorrupt();
    }
  }
  backupCorrupt() {
    try {
      const ts = Date.now();
      const corruptPath = this.filePath.replace(/\.json$/, `.corrupt.${ts}.json`);
      if (fs.existsSync(this.filePath)) {
        fs.renameSync(this.filePath, corruptPath);
      }
    } catch {
    }
    this.ledger = /* @__PURE__ */ new Map();
  }
  persist() {
    const entries = [];
    const cutoff = Date.now() - 7 * 24 * 60 * 60 * 1e3;
    for (const entry of this.ledger.values()) {
      if (entry.status === "sent" && entry.updatedAt < cutoff) {
        continue;
      }
      entries.push(entry);
    }
    if (entries.length > 5e3) {
      entries.sort((a, b) => b.updatedAt - a.updatedAt);
      entries.splice(5e3);
    }
    const data = { entries };
    const dir = path.dirname(this.filePath);
    if (!fs.existsSync(dir)) {
      fs.mkdirSync(dir, { recursive: true });
    }
    const tmpPath = this.filePath + ".tmp." + crypto.randomBytes(4).toString("hex");
    fs.writeFileSync(tmpPath, JSON.stringify(data), "utf-8");
    fs.renameSync(tmpPath, this.filePath);
  }
  async execute(deliveryKey, sender) {
    const clientId = this.resolveClientId(deliveryKey);
    const inflight = this.inflight.get(deliveryKey);
    if (inflight) {
      const result = await inflight;
      return { duplicate: true, clientId };
    }
    const existing = this.ledger.get(deliveryKey);
    if (existing) {
      if (existing.status === "sent") {
        return { duplicate: true, clientId };
      }
      const promise2 = this.executeSender(clientId, deliveryKey, sender);
      this.inflight.set(deliveryKey, promise2);
      try {
        return await promise2;
      } finally {
        this.inflight.delete(deliveryKey);
      }
    }
    const entry = {
      deliveryKey,
      clientId,
      status: "sending",
      updatedAt: Date.now()
    };
    this.ledger.set(deliveryKey, entry);
    this.persist();
    const promise = this.executeSender(clientId, deliveryKey, sender);
    this.inflight.set(deliveryKey, promise);
    try {
      return await promise;
    } finally {
      this.inflight.delete(deliveryKey);
    }
  }
  async executeSender(clientId, deliveryKey, sender) {
    try {
      await sender(clientId);
      const entry = this.ledger.get(deliveryKey);
      if (entry) {
        entry.status = "sent";
        entry.updatedAt = Date.now();
        this.persist();
      }
      return { duplicate: false, clientId };
    } catch {
      return { duplicate: false, clientId };
    }
  }
};

// src/openclaw-wechat.ts
var OpenClawWechatManager = class {
  debugLog(...args) {
    if (process.env.NODE_ENV === "development" || process.env.DEBUG) {
      console.debug(...args);
    }
  }
  hashId(id) {
    return crypto2.createHash("sha256").update(id).digest("hex").slice(0, 8);
  }
  state = {
    status: "idle",
    accountId: null,
    qrCodeUrl: "",
    sessionKey: "",
    message: "",
    messageCount: 0,
    replyCount: 0,
    baseUrl: DEFAULT_BASE_URL,
    startedAt: null,
    lastError: null
  };
  polling = false;
  pollGeneration = 0;
  pollPromise = null;
  pollAbort = null;
  handlers = [];
  sessionStaleCount = 0;
  sessionWarned = false;
  _lastFromUserId = "";
  getUpdatesBuf = "";
  token = null;
  inboundSeen = /* @__PURE__ */ new Map();
  deliveryStore = null;
  getDeliveryStore() {
    if (!this.deliveryStore) {
      const dir = path2.join(resolveStateDir(), "openclaw-weixin", "accounts");
      const accountId = this.state.accountId || "default";
      const filePath = path2.join(dir, `${accountId}.delivery-idempotency.json`);
      this.deliveryStore = new DeliveryIdempotencyStore(filePath);
    }
    return this.deliveryStore;
  }
  resetDeliveryStore() {
    this.deliveryStore = null;
  }
  getState() {
    return { ...this.state };
  }
  onMessage(handler) {
    this.handlers.push(handler);
  }
  loadSavedAccount() {
    const ids = listIndexedWeixinAccountIds();
    if (ids.length === 0) return false;
    const id = ids[ids.length - 1];
    const data = loadWeixinAccount(id);
    if (data?.token) {
      this.state.accountId = id;
      this.state.status = "connected";
      this.state.baseUrl = data.baseUrl || DEFAULT_BASE_URL;
      this.state.startedAt = (/* @__PURE__ */ new Date()).toISOString();
      this.token = data.token;
      if (data.getUpdatesBuf) {
        this.getUpdatesBuf = data.getUpdatesBuf;
      }
      if (typeof data.messageCount === "number") {
        this.state.messageCount = data.messageCount;
      }
      this.loadStats();
      this.debugLog("[OpenClaw] Loaded saved account: ");
      return true;
    }
    return false;
  }
  resolveStatsPath() {
    const dir = path2.join(resolveStateDir(), "openclaw-weixin", "accounts");
    if (!fs2.existsSync(dir)) fs2.mkdirSync(dir, { recursive: true });
    return path2.join(dir, `${this.state.accountId}.stats.json`);
  }
  persistStats() {
    if (!this.state.accountId) return;
    try {
      fs2.writeFileSync(this.resolveStatsPath(), JSON.stringify({
        messageCount: this.state.messageCount,
        replyCount: this.state.replyCount
      }), "utf-8");
    } catch {
    }
  }
  loadStats() {
    if (!this.state.accountId) return;
    try {
      const statsPath = this.resolveStatsPath();
      if (fs2.existsSync(statsPath)) {
        const data = JSON.parse(fs2.readFileSync(statsPath, "utf-8"));
        if (typeof data.messageCount === "number") this.state.messageCount = data.messageCount;
        if (typeof data.replyCount === "number") this.state.replyCount = data.replyCount;
      }
    } catch {
    }
  }
  async startLogin(opts) {
    try {
      this.state.status = "qr_ready";
      this.state.lastError = null;
      const result = await startWeixinLoginWithQr({
        apiBaseUrl: DEFAULT_BASE_URL,
        botType: DEFAULT_ILINK_BOT_TYPE,
        force: opts?.force ?? false,
        verbose: true
      });
      this.state.qrCodeUrl = result.qrcodeUrl;
      this.state.sessionKey = result.sessionKey;
      this.state.message = result.message;
      const qrImageUrl = await QRCode.toDataURL(result.qrcodeUrl, { width: 300, margin: 2 });
      this.debugLog("[OpenClaw] QR code ready, sessionKey hash: ");
      return {
        qrCodeUrl: result.qrcodeUrl,
        qrImageUrl,
        sessionKey: result.sessionKey
      };
    } catch (err) {
      this.state.status = "error";
      this.state.lastError = err.message;
      console.error(`[OpenClaw] startLogin error:`, err.message);
      throw err;
    }
  }
  async waitForScan(timeoutMs = 12e4) {
    try {
      this.state.status = "waiting_scan";
      const result = await waitForWeixinLogin({
        sessionKey: this.state.sessionKey,
        apiBaseUrl: DEFAULT_BASE_URL,
        timeoutMs
      });
      if (result.connected && result.botToken && result.accountId) {
        const normalizedId = normalizeAccountId(result.accountId);
        saveWeixinAccount(normalizedId, {
          token: result.botToken,
          baseUrl: result.baseUrl,
          userId: result.userId
        });
        registerWeixinAccountId(normalizedId);
        this.state.status = "connected";
        this.state.accountId = normalizedId;
        this.state.baseUrl = result.baseUrl || DEFAULT_BASE_URL;
        this.state.startedAt = (/* @__PURE__ */ new Date()).toISOString();
        this.token = result.botToken;
        this.resetDeliveryStore();
        this.debugLog("[OpenClaw] Login confirmed! accountId: ");
        return {
          connected: true,
          message: "\u5DF2\u5C06 OpenClaw \u8FDE\u63A5\u5230\u5FAE\u4FE1",
          accountId: normalizedId
        };
      }
      if (result.alreadyConnected) {
        this.state.status = "connected";
        return { connected: true, message: result.message };
      }
      this.state.status = "idle";
      this.state.lastError = null;
      this.state.message = result.message;
      return { connected: false, message: result.message };
    } catch (err) {
      this.state.status = "error";
      this.state.lastError = err.message;
      console.error(`[OpenClaw] waitForScan error:`, err.message);
      return { connected: false, message: err.message };
    }
  }
  async startPolling() {
    if (this.polling && this.pollPromise) return;
    if (!this.token || !this.state.accountId) {
      console.warn("[OpenClaw] Cannot start polling: no credentials");
      return;
    }
    this.polling = true;
    this.pollGeneration++;
    const generation = this.pollGeneration;
    const controller = new AbortController();
    this.pollAbort = controller;
    try {
      await notifyStart({
        baseUrl: this.state.baseUrl,
        token: this.token
      });
    } catch (err) {
      console.warn(`[OpenClaw] notifyStart failed (ignored):`, err.message);
    }
    console.log(`[OpenClaw] Starting message polling on ${this.state.baseUrl} gen=${generation}`);
    let consecutiveErrors = 0;
    const MAX_CONSECUTIVE_ERRORS = 3;
    const poll = async () => {
      while (this.pollGeneration === generation && this.polling && !controller.signal.aborted) {
        try {
          const resp = await getUpdates({
            baseUrl: this.state.baseUrl,
            token: this.token,
            get_updates_buf: this.getUpdatesBuf,
            timeoutMs: 35e3
          });
          if (this.pollGeneration !== generation) break;
          if (resp.errcode && resp.errcode !== 0) {
            consecutiveErrors++;
            console.error(`[OpenClaw] getUpdates error (${consecutiveErrors}/${MAX_CONSECUTIVE_ERRORS}): ${resp.errcode} ${resp.errmsg}`);
            if (resp.errcode === -14 && consecutiveErrors >= MAX_CONSECUTIVE_ERRORS) {
              console.warn(`[OpenClaw] Session expired after ${consecutiveErrors} errors, auto-reconnecting...`);
              this.polling = false;
              this.autoReconnect().catch((err) => console.error("[OpenClaw] Auto-reconnect failed:", err));
              break;
            }
            await new Promise((r) => setTimeout(r, 5e3));
            continue;
          }
          consecutiveErrors = 0;
          console.log("[OpenClaw] getUpdates OK: ret=" + (resp.ret ?? "?") + " msgs=" + (resp.msgs && resp.msgs.length || 0));
          if (resp.get_updates_buf) {
            this.getUpdatesBuf = resp.get_updates_buf;
            this.persistBuf();
          }
          if (resp.msgs && resp.msgs.length > 0) {
            for (const msg of resp.msgs) {
              await this.processMessage(msg);
            }
          }
        } catch (err) {
          if (this.pollGeneration !== generation) break;
          if (err.name === "AbortError") break;
          console.error(`[OpenClaw] Poll error:`, err.message);
          this.sessionStaleCount++;
          await this.checkSessionHealth();
          await new Promise((r) => setTimeout(r, 3e3));
        }
      }
    };
    const loopPromise = poll().catch((err) => console.error("[OpenClaw] Poll loop crashed:", err));
    const cleanup = async () => {
      await loopPromise;
      if (this.pollGeneration === generation) {
        this.polling = false;
        this.pollPromise = null;
      }
    };
    this.pollPromise = cleanup();
  }
  async autoReconnect() {
    console.log("[OpenClaw] ========== AUTO-RECONNECT START ==========");
    const savedAccountId = this.state.accountId;
    this.pollAbort?.abort();
    this.pollAbort = null;
    this.getUpdatesBuf = "";
    this.token = null;
    this.state.status = "idle";
    this.state.qrCodeUrl = "";
    this.state.sessionKey = "";
    this.state.message = "";
    this.state.lastError = null;
    try {
      const loginResult = await this.startLogin();
      console.log("[OpenClaw] QR code ready - re-scan same bot to restore session");
      const scanResult = await this.waitForScan(12e4);
      if (scanResult.connected) {
        this.debugLog("[OpenClaw] Auto-reconnect: scan confirmed (bot: " + this.hashId(this.state.accountId ?? "?") + ")");
        await this.startPolling();
        console.log("[OpenClaw] ========== AUTO-RECONNECT SUCCESS ==========");
      } else {
        console.warn("[OpenClaw] Auto-reconnect: scan timeout:", scanResult.message);
        this.state.status = "idle";
        this.state.accountId = savedAccountId;
        this.state.lastError = "Scan timeout - QR still available, please rescan";
      }
    } catch (err) {
      console.error("[OpenClaw] Auto-reconnect failed:", err.message);
      this.state.status = "error";
      this.state.accountId = savedAccountId;
      this.state.lastError = "Auto-reconnect failed: " + err.message;
    }
  }
  async checkSessionHealth() {
    const STALE_THRESHOLD = 5;
    if (this.sessionStaleCount >= STALE_THRESHOLD && !this.sessionWarned) {
      this.sessionWarned = true;
      console.warn("[OpenClaw] Session may be expiring, sending pre-expiry warning...");
      await this.sendPreExpiryWarning();
    }
  }
  async sendPreExpiryWarning() {
    try {
      const lastUserId = this._lastFromUserId;
      if (!lastUserId) return;
      console.log("[OpenClaw] Generating pre-expiry QR code...");
      await this.startLogin();
      const qrUrl = this.state.qrCodeUrl;
      const msg = [
        "\u{1F514} \u8FDE\u63A5\u5373\u5C06\u8FC7\u671F",
        "",
        "\u8BF7\u6253\u5F00\u4EE5\u4E0B\u94FE\u63A5\u91CD\u65B0\u626B\u7801\uFF0C\u4FDD\u6301\u6211\u4EEC\u7684\u8FDE\u63A5\uFF1A",
        qrUrl,
        "",
        "\uFF08\u5982\u679C\u6211\u5DF2\u7ECF\u4E0D\u56DE\u590D\u4E86\uFF0C\u53BB http://127.0.0.1:5173 \u626B\u7801\u5373\u53EF\uFF09"
      ].join("\n");
      await this.sendTextMessage(lastUserId, msg);
      this.debugLog("[OpenClaw] Pre-expiry QR sent to: " + this.hashId(lastUserId));
    } catch (err) {
      console.error("[OpenClaw] Failed to send pre-expiry warning:", err.message);
    }
  }
  async sendTextMessage(toUserId, text, contextToken, deliveryKey) {
    if (!this.token) throw new Error("Not logged in");
    if (deliveryKey) {
      const store = this.getDeliveryStore();
      return store.execute(deliveryKey, async (clientId) => {
        await sendMessage({
          baseUrl: this.state.baseUrl,
          token: this.token,
          body: {
            msg: {
              from_user_id: "",
              to_user_id: toUserId,
              client_id: clientId,
              message_type: 2,
              message_state: 2,
              context_token: contextToken || "",
              item_list: [{ type: 1, text_item: { text } }]
            }
          }
        });
        this.state.replyCount++;
        this.persistStats();
      });
    }
    this.debugLog("[OpenClaw] Sending to via SDK sendMessage...");
    try {
      await sendMessage({
        baseUrl: this.state.baseUrl,
        token: this.token,
        body: {
          msg: {
            from_user_id: "",
            to_user_id: toUserId,
            client_id: `openclaw-weixin:${Date.now()}-${crypto2.randomBytes(4).toString("hex")}`,
            message_type: 2,
            message_state: 2,
            context_token: contextToken || "",
            item_list: [{ type: 1, text_item: { text } }]
          }
        }
      });
      this.state.replyCount++;
      this.persistStats();
      console.log(`[OpenClaw] SDK sendMessage completed`);
      this.debugLog("[OpenClaw] Sent to: ");
      return { duplicate: false, clientId: "" };
    } catch (err) {
      console.error(`[OpenClaw] Send exception:`, err.message);
      throw err;
    }
  }
  async sendVoiceMessage(toUserId, audioBuffer, encodeType = 7, playtime = 0, contextToken, deliveryKey) {
    if (!this.token) throw new Error("Not logged in");
    if (deliveryKey) {
      const store = this.getDeliveryStore();
      return store.execute(deliveryKey, async (clientId2) => {
        await this.sendVoiceInternal(toUserId, audioBuffer, encodeType, playtime, contextToken, clientId2);
        this.state.replyCount++;
        this.persistStats();
      });
    }
    const clientId = `openclaw-weixin:${Date.now()}-${crypto2.randomBytes(4).toString("hex")}`;
    await this.sendVoiceInternal(toUserId, audioBuffer, encodeType, playtime, contextToken, clientId);
    this.state.replyCount++;
    this.persistStats();
    return { duplicate: false, clientId };
  }
  async sendVoiceInternal(toUserId, audioBuffer, encodeType, playtime, contextToken, clientId) {
    const rawsize = audioBuffer.length;
    const rawfilemd5 = crypto2.createHash("md5").update(audioBuffer).digest("hex");
    const aesKey = crypto2.randomBytes(16);
    const encrypted = this.aes128EcbEncrypt(audioBuffer, aesKey);
    const filesize = encrypted.length;
    const filekey = `voice_${Date.now()}_${crypto2.randomBytes(4).toString("hex")}.mp3`;
    console.log(`[OpenClaw][VOICE-SEND] rawsize=${rawsize} filesize=${filesize} filekey=${filekey}`);
    const uploadResp = await getUploadUrl({
      baseUrl: this.state.baseUrl,
      token: this.token,
      filekey,
      media_type: 4,
      to_user_id: toUserId,
      rawsize,
      rawfilemd5,
      filesize,
      aeskey: aesKey.toString("hex"),
      no_need_thumb: true
    });
    console.log("[OpenClaw][VOICE-SEND] uploadResp errcode=" + uploadResp.errcode + " upload_full_url=" + (uploadResp.upload_full_url ? "OK" : "MISSING"));
    console.log("[OpenClaw][VOICE-SEND] uploadResp keys: " + Object.keys(uploadResp).join(","));
    if (uploadResp.errcode && uploadResp.errcode !== 0) {
      throw new Error("getUploadUrl failed: " + uploadResp.errcode + " " + (uploadResp.errmsg || ""));
    }
    if (!uploadResp.upload_full_url) {
      throw new Error("getUploadUrl returned no upload_full_url");
    }
    const encryptQueryParam = uploadResp.encrypt_query_param || (() => {
      const m = uploadResp.upload_full_url.match(/encrypted_query_param=([^&]+)/);
      return m ? decodeURIComponent(m[1]) : "";
    })();
    if (!encryptQueryParam) throw new Error("encrypt_query_param missing");
    console.log("[OpenClaw][VOICE-SEND] CDN POST...");
    const putResp = await fetch(uploadResp.upload_full_url, {
      method: "POST",
      body: encrypted,
      signal: AbortSignal.timeout(3e4)
    });
    console.log("[OpenClaw][VOICE-SEND] CDN PUT response: status=" + putResp.status + " ok=" + putResp.ok);
    if (!putResp.ok) {
      throw new Error("CDN upload failed: " + putResp.status);
    }
    console.log("[OpenClaw][VOICE-SEND] CDN PUT OK");
    const accountId = this.getState().accountId || "";
    await sendMessage({
      baseUrl: this.state.baseUrl,
      token: this.token,
      body: {
        msg: {
          from_user_id: accountId,
          to_user_id: toUserId,
          client_id: clientId,
          message_type: 2,
          message_state: 2,
          context_token: contextToken || "",
          item_list: [{
            type: 3,
            voice_item: {
              media: {
                encrypt_query_param: encryptQueryParam,
                aes_key: Buffer.from(aesKey.toString("hex")).toString("base64")
              },
              encode_type: encodeType,
              playtime
            }
          }]
        }
      }
    });
    console.log("[OpenClaw][VOICE-SEND] Message sent OK");
  }
  async sendImageMessage(toUserId, imageBuffer, contextToken, deliveryKey) {
    if (!this.token) throw new Error("Not logged in");
    if (deliveryKey) {
      const store = this.getDeliveryStore();
      return store.execute(deliveryKey, async (clientId2) => {
        await this.sendImageInternal(toUserId, imageBuffer, contextToken, clientId2);
        this.state.replyCount++;
        this.persistStats();
      });
    }
    const clientId = `openclaw-weixin:${Date.now()}-${crypto2.randomBytes(4).toString("hex")}`;
    await this.sendImageInternal(toUserId, imageBuffer, contextToken, clientId);
    this.state.replyCount++;
    this.persistStats();
    return { duplicate: false, clientId };
  }
  async sendImageInternal(toUserId, imageBuffer, contextToken, clientId) {
    const rawsize = imageBuffer.length;
    const rawfilemd5 = crypto2.createHash("md5").update(imageBuffer).digest("hex");
    const aesKey = crypto2.randomBytes(16);
    const encrypted = this.aes128EcbEncrypt(imageBuffer, aesKey);
    const uploadResp = await getUploadUrl({
      baseUrl: this.state.baseUrl,
      token: this.token,
      filekey: `image_${Date.now()}_${crypto2.randomBytes(4).toString("hex")}.png`,
      media_type: 2,
      to_user_id: toUserId,
      rawsize,
      rawfilemd5,
      filesize: encrypted.length,
      aeskey: aesKey.toString("hex"),
      no_need_thumb: true
    });
    if (uploadResp.errcode && uploadResp.errcode !== 0) throw new Error("getUploadUrl failed: " + uploadResp.errcode);
    if (!uploadResp.upload_full_url) throw new Error("getUploadUrl returned no upload_full_url");
    const encryptQueryParam = uploadResp.encrypt_query_param || (() => {
      const match = uploadResp.upload_full_url.match(/encrypted_query_param=([^&]+)/);
      return match ? decodeURIComponent(match[1]) : "";
    })();
    if (!encryptQueryParam) throw new Error("encrypt_query_param missing");
    const upload = await fetch(uploadResp.upload_full_url, { method: "POST", body: encrypted, signal: AbortSignal.timeout(3e4) });
    if (!upload.ok) throw new Error("CDN upload failed: " + upload.status);
    await sendMessage({
      baseUrl: this.state.baseUrl,
      token: this.token,
      body: { msg: { from_user_id: this.getState().accountId || "", to_user_id: toUserId, client_id: clientId, message_type: 2, message_state: 2, context_token: contextToken || "", item_list: [{ type: 2, image_item: { media: { encrypt_query_param: encryptQueryParam, aes_key: Buffer.from(aesKey.toString("hex")).toString("base64") } } }] } }
    });
  }
  aes128EcbEncrypt(data, key) {
    const cipher = crypto2.createCipheriv("aes-128-ecb", key, Buffer.alloc(0));
    cipher.setAutoPadding(true);
    return Buffer.concat([cipher.update(data), cipher.final()]);
  }
  async stopPolling() {
    this.polling = false;
    this.pollGeneration++;
    this.pollAbort?.abort();
    if (this.token) {
      try {
        await notifyStop({
          baseUrl: this.state.baseUrl,
          token: this.token
        });
      } catch {
      }
    }
    if (this.pollPromise) {
      await this.pollPromise;
    }
  }
  async resetLogin() {
    await this.stopPolling();
    const oldAccountId = this.state.accountId;
    if (oldAccountId) {
      try {
        unregisterWeixinAccountId(oldAccountId);
        clearWeixinAccount(oldAccountId);
        this.debugLog("[OpenClaw] Cleared old account: ");
      } catch (err) {
        console.warn(`[OpenClaw] Failed to clear old account:`, err.message);
      }
    }
    this.state = {
      status: "idle",
      accountId: null,
      qrCodeUrl: "",
      sessionKey: "",
      message: "",
      messageCount: 0,
      replyCount: 0,
      baseUrl: DEFAULT_BASE_URL,
      startedAt: null,
      lastError: null
    };
    this.getUpdatesBuf = "";
    this.token = null;
    this.resetDeliveryStore();
  }
  persistBuf() {
    if (!this.state.accountId) return;
    try {
      const data = { messageCount: this.state.messageCount };
      if (this.getUpdatesBuf) data.getUpdatesBuf = this.getUpdatesBuf;
      saveWeixinAccount(this.state.accountId, data);
    } catch {
    }
  }
  resolveInboundMessageId(msg) {
    if (msg.message_id) {
      return String(msg.message_id);
    }
    const fields = {
      from_user_id: msg.from_user_id ?? "",
      to_user_id: msg.to_user_id ?? "",
      create_time_ms: msg.create_time_ms ?? 0,
      message_type: msg.message_type ?? 0,
      context_token: msg.context_token ?? "",
      item_list: msg.item_list ?? []
    };
    const normalized = JSON.stringify(fields, Object.keys(fields).sort());
    const hash = crypto2.createHash("sha256").update(normalized).digest("hex");
    return `fallback-${hash}`;
  }
  checkInboundDedupe(accountId, messageId) {
    const key = `${accountId}:${messageId}`;
    const now = Date.now();
    const seen = this.inboundSeen.get(key);
    if (seen && now - seen < 10 * 60 * 1e3) {
      return true;
    }
    this.inboundSeen.set(key, now);
    this.cleanupInboundSeen();
    return false;
  }
  cleanupInboundSeen() {
    if (this.inboundSeen.size < 1e3) return;
    const cutoff = Date.now() - 10 * 60 * 1e3;
    for (const [key, ts] of this.inboundSeen) {
      if (ts < cutoff) {
        this.inboundSeen.delete(key);
      }
    }
  }
  async processMessage(msg) {
    console.log("[OpenClaw][DIAG] === processMessage === msg_type=" + msg.message_type + " from=" + this.hashId(String(msg.from_user_id || "")) + " items.len=" + (msg.item_list ? msg.item_list.length : 0));
    if (msg.message_type === 2) return;
    const fromUserId = msg.from_user_id || "";
    this._lastFromUserId = fromUserId;
    const toUserId = msg.to_user_id || "";
    const contextToken = msg.context_token || "";
    const accountId = this.state.accountId || "";
    const messageId = this.resolveInboundMessageId(msg);
    const createdAt = msg.create_time_ms || Date.now();
    if (this.checkInboundDedupe(accountId, messageId)) {
      console.log("[OpenClaw] Duplicate inbound message skipped: " + this.hashId(messageId));
      return;
    }
    let text = "";
    let isVoice = false;
    if (msg.item_list) {
      console.log("[OpenClaw][DIAG] item_list: " + msg.item_list.length + " items");
      for (const item of msg.item_list) {
        if (item.type === 3 && item.voice_item) {
          isVoice = true;
          console.log("[OpenClaw][DIAG] voice_item: playtime=" + item.voice_item.playtime + " encode_type=" + item.voice_item.encode_type + " hasText=" + (item.voice_item.text ? "YES len=" + item.voice_item.text.length : "NO") + " hasMedia=" + !!item.voice_item.media);
          const vt = item.voice_item.text || "";
          console.log("[OpenClaw][VOICE] playtime=" + item.voice_item.playtime + " encode_type=" + item.voice_item.encode_type + " text=" + vt.substring(0, 100));
          if (vt) text += vt;
        }
        if (item.type === 1 && item.text_item?.text) {
          text += item.text_item.text;
        }
      }
    }
    let audioBase64;
    let audioMime;
    let imageBase64;
    let imageUrl;
    let aeskey;
    if (msg.item_list) {
      console.log("[OpenClaw][DIAG] item_list: " + msg.item_list.length + " items");
      for (const item of msg.item_list) {
        if (item.type === 3 && item.voice_item) {
          if (item.voice_item.media && item.voice_item.media.encrypt_query_param && item.voice_item.media.aes_key) {
            try {
              console.log("[OpenClaw][VOICE-DL] \u5F00\u59CB\u4E0B\u8F7D\u8BED\u97F3...");
              const silkBuf = await downloadAndDecryptBuffer(
                item.voice_item.media.encrypt_query_param,
                item.voice_item.media.aes_key,
                this.state.baseUrl,
                "wechat-voice",
                item.voice_item.media.full_url
              );
              console.log("[OpenClaw][VOICE-DL] \u4E0B\u8F7D\u5B8C\u6210, size=" + silkBuf.length);
              const wavBuf = await silkToWav(silkBuf);
              const audioBuf = wavBuf || silkBuf;
              audioBase64 = audioBuf.toString("base64");
              audioMime = wavBuf ? "audio/wav" : "audio/silk";
              console.log("[OpenClaw][VOICE-DL] \u8F6C\u7801\u5B8C\u6210, size=" + audioBuf.length + " mime=" + audioMime);
            } catch (dlErr) {
              console.error("[OpenClaw][VOICE-DL] \u4E0B\u8F7D\u5931\u8D25: " + (dlErr?.message || String(dlErr)));
            }
          }
        }
        if (item.type === 2 && item.image_item) {
          console.log("[OpenClaw][IMAGE] === \u6536\u5230\u56FE\u7247\u6D88\u606F from " + this.hashId(String(fromUserId)) + " ===");
          console.log("[OpenClaw][IMAGE] image_item keys: " + Object.keys(item.image_item).join(", "));
          const img = item.image_item;
          if (img.url) {
            imageUrl = img.url;
            console.log("[OpenClaw][IMAGE] url: " + img.url);
          }
          if (img.media) {
            console.log("[OpenClaw][IMAGE] media keys: " + Object.keys(img.media).join(", "));
            if (img.media.full_url) {
              imageUrl = img.media.full_url;
              console.log("[OpenClaw][IMAGE] media.full_url: " + img.media.full_url);
            }
            if (img.media.encrypt_query_param) {
              console.log("[OpenClaw][IMAGE] encrypt_query_param: " + img.media.encrypt_query_param.substring(0, 80) + "...");
            }
            if (img.media.aes_key) {
              console.log("[OpenClaw][IMAGE] aes_key: " + img.media.aes_key.substring(0, 16) + "...");
            }
          }
          if (img.aeskey) {
            aeskey = img.aeskey;
            console.log("[OpenClaw][IMAGE] aeskey: " + img.aeskey.substring(0, 16) + "...");
          }
          console.log("[OpenClaw][IMAGE] mid_size=" + img.mid_size + " hd_size=" + img.hd_size);
          console.log("[OpenClaw][IMAGE] thumb: " + img.thumb_width + "x" + img.thumb_height + " size=" + img.thumb_size);
          console.log("[OpenClaw][IMAGE] === \u56FE\u7247\u4FE1\u606F\u6253\u5370\u5B8C\u6BD5 ===");
        }
      }
    }
    console.log("[OpenClaw][DIAG] textCheck: text=" + JSON.stringify(text.substring(0, 100)) + " isVoice=" + isVoice + " hasImage=" + !!imageUrl);
    if (!text && !imageUrl) {
      console.log("[OpenClaw][DIAG] *** \u6D88\u606F\u88AB\u4E22\u5F03: text\u4E3A\u7A7A\u4E14\u65E0\u56FE\u7247 ***");
      console.log("[OpenClaw] Non-text msg from userId=" + this.hashId(String(fromUserId)));
      return;
    }
    this.state.messageCount++;
    this.persistBuf();
    this.persistStats();
    console.log("[OpenClaw] Msg from " + this.hashId(String(fromUserId)) + ": " + (text.length > 80 ? text.substring(0, 80) + "..." : text));
    console.log("[OpenClaw][DIAG] \u8C03\u7528handler: text=" + text.substring(0, 80) + " isVoice=" + isVoice + " handlers=" + this.handlers.length);
    for (const handler of this.handlers) {
      try {
        await handler({
          fromUserId,
          toUserId,
          messageId,
          text,
          isVoice,
          audioBase64,
          imageBase64,
          imageUrl,
          aeskey,
          contextToken,
          createdAt
        });
      } catch (err) {
        console.error(`[OpenClaw] Handler error:`, err.message);
      }
    }
  }
};
var instance = null;
function getWechatManager() {
  if (!instance) instance = new OpenClawWechatManager();
  return instance;
}

// src/index.ts
var app = Fastify({
  logger: { level: process.env.LOG_LEVEL || "info" }
});
await app.register(cors, {
  origin: [/^http:\/\/127\.0\.0\.1:\d+$/, /^http:\/\/localhost:\d+$/],
  methods: ["GET", "POST", "OPTIONS"],
  credentials: false
});
app.addHook("onRequest", async (req, reply) => {
  return;
});
app.addHook("onSend", async (_req, reply) => {
  reply.header("X-Content-Type-Options", "nosniff");
  reply.header("X-Frame-Options", "DENY");
  void reply.header("X-Powered-By", "");
});
var manager = getWechatManager();
var contextTokenCache = /* @__PURE__ */ new Map();
var msgBuffers = /* @__PURE__ */ new Map();
manager.onMessage(async (msg) => {
  const BUFFER_MS = sidecarConfig.mergeWindowMs;
  const key = msg.fromUserId;
  const existing = msgBuffers.get(key);
  console.log(`[Sidecar][DIAG] buffer\u5165: text="${msg.text.substring(0, 60)}" isVoice=${msg.isVoice} hasImage=${!!msg.imageUrl}`);
  const item = { text: msg.text, contextToken: msg.contextToken || "", fromUserId: msg.fromUserId, toUserId: msg.toUserId, messageId: msg.messageId, createdAt: msg.createdAt, audioBase64: msg.audioBase64 || "", isVoice: msg.isVoice || false, imageUrl: msg.imageUrl || "", imageBase64: msg.imageBase64 || "", aeskey: msg.aeskey || "" };
  if (msg.contextToken) {
    contextTokenCache.set(msg.fromUserId, msg.contextToken);
  }
  if (existing) {
    clearTimeout(existing.timer);
    existing.msgs.push(item);
    console.log(`[Sidecar] Buffer +1 (total ${existing.msgs.length}): "${msg.text.substring(0, 40)}"`);
  } else {
    msgBuffers.set(key, { msgs: [item], timer: null });
    console.log(`[Sidecar] Buffer start: "${msg.text.substring(0, 40)}"`);
  }
  const entry = msgBuffers.get(key);
  entry.timer = setTimeout(async () => {
    msgBuffers.delete(key);
    const all = entry.msgs;
    const last = all[all.length - 1];
    const combined = all.map((m) => m.text).join("\n");
    const wasVoice = all.some((m) => m.isVoice === true);
    console.log(`[Sidecar][DIAG] FIRE: msgs=${all.length} wasVoice=${wasVoice} text="${combined.substring(0, 100)}"`);
    let imageUrl = "";
    const firstImageMsg = all.find((m) => m.imageUrl);
    if (firstImageMsg?.imageUrl) {
      try {
        console.log("[Sidecar][IMAGE-DL] \u5F00\u59CB\u4E0B\u8F7D\u56FE\u7247: " + firstImageMsg.imageUrl.substring(0, 80) + "...");
        const imgResp = await fetch(firstImageMsg.imageUrl, { signal: AbortSignal.timeout(3e4) });
        if (imgResp.ok) {
          let imgBuffer = Buffer.from(await imgResp.arrayBuffer());
          console.log("[Sidecar][IMAGE-DL] \u4E0B\u8F7D\u5B8C\u6210, size=" + imgBuffer.length);
          const rawAesKey = firstImageMsg.aeskey;
          if (rawAesKey && rawAesKey.length === 32) {
            try {
              const crypto3 = await import("node:crypto");
              const key2 = Buffer.from(rawAesKey, "hex");
              const decipher = crypto3.createDecipheriv("aes-128-ecb", key2, null);
              decipher.setAutoPadding(false);
              let decrypted = Buffer.concat([decipher.update(imgBuffer), decipher.final()]);
              const padLen = decrypted[decrypted.length - 1];
              if (padLen > 0 && padLen <= 16) {
                decrypted = decrypted.subarray(0, decrypted.length - padLen);
              }
              imgBuffer = decrypted;
              console.log("[Sidecar][IMAGE-DL] AES\u89E3\u5BC6\u6210\u529F, \u89E3\u5BC6\u540Esize=" + imgBuffer.length);
            } catch (decErr) {
              console.log("[Sidecar][IMAGE-DL] AES\u89E3\u5BC6\u5931\u8D25: " + decErr.message + ", \u5C1D\u8BD5\u4F7F\u7528\u539F\u59CB\u6570\u636E");
            }
          }
          const contentType = imgResp.headers.get("content-type") || "image/jpeg";
          imageUrl = "data:" + contentType + ";base64," + imgBuffer.toString("base64");
          console.log("[Sidecar][IMAGE-DL] \u56FE\u7247\u5904\u7406\u5B8C\u6210, final size=" + imgBuffer.length + " type=" + contentType);
        } else {
          console.log("[Sidecar][IMAGE-DL] \u56FE\u7247\u4E0B\u8F7D\u5931\u8D25 HTTP " + imgResp.status);
        }
      } catch (err) {
        console.log("[Sidecar][IMAGE-DL] \u56FE\u7247\u4E0B\u8F7D\u5F02\u5E38: " + err.message);
      }
    }
    const headers = { "Content-Type": "application/json" };
    const t = sidecarConfig.bridgeApiToken;
    if (t) headers["Authorization"] = "Bearer " + t;
    try {
      const resp = await fetch(`${sidecarConfig.coreUrl}/api/agent/webhook`, {
        method: "POST",
        headers,
        body: JSON.stringify({
          channel: "wechat",
          accountId: "openclaw-wechat",
          conversationId: `conv-${last.fromUserId}`,
          senderId: last.fromUserId,
          messageId: last.messageId,
          contextToken: last.contextToken,
          type: wasVoice ? "voice" : "text",
          text: combined,
          createdAt: last.createdAt,
          imageBase64: last.imageBase64 || "",
          imageUrl: imageUrl || last.imageUrl || "",
          audioBase64: (all.find((m) => m.audioBase64) || last).audioBase64 || "",
          voiceMessage: wasVoice,
          skipTiming: true
        }),
        signal: AbortSignal.timeout(18e4)
      });
      const json = await resp.json();
      console.log(`[Sidecar][DIAG] \u540E\u7AEF\u54CD\u5E94: code=${json?.code} msg=${json?.msg} hasData=${!!json?.data} hasOutMsg=${!!json?.data?.outgoingMessage} hasText=${!!json?.data?.outgoingMessage?.text}`);
    } catch (err) {
      console.error("[Sidecar] Forward failed:", err.message);
    }
  }, BUFFER_MS);
  return void 0;
});
app.get("/api/health", async (_req, reply) => {
  const state = manager.getState();
  return reply.send({
    success: true,
    status: state.status,
    accountId: state.accountId,
    message: state.message
  });
});
app.get("/api/status", async (_req, reply) => {
  const state = manager.getState();
  return reply.send({
    success: true,
    data: {
      status: state.status,
      accountId: state.accountId,
      qrCodeUrl: state.qrCodeUrl,
      messageCount: state.messageCount,
      replyCount: state.replyCount,
      baseUrl: state.baseUrl,
      startedAt: state.startedAt,
      lastError: state.lastError,
      message: state.message
    }
  });
});
app.post("/api/login/start", async (_req, reply) => {
  try {
    if (manager.getState().status === "connected") {
      return reply.send({
        success: true,
        message: "Already logged in",
        data: { status: "connected", accountId: manager.getState().accountId }
      });
    }
    const result = await manager.startLogin();
    manager.waitForScan(12e4).then((scanResult) => {
      if (scanResult.connected) {
        manager.startPolling().catch(
          (err) => console.error("[Sidecar] Auto-polling start failed:", err)
        );
      }
    }).catch((err) => console.error("[Sidecar] Background waitForScan failed:", err));
    return reply.send({
      success: true,
      message: "QR code generated",
      data: {
        qrCodeUrl: result.qrCodeUrl,
        qrImageUrl: result.qrImageUrl,
        sessionKey: result.sessionKey,
        status: "qr_ready"
      }
    });
  } catch (err) {
    console.error("[Sidecar]", err);
    return reply.status(500).send({ success: false, message: "\u670D\u52A1\u5668\u9519\u8BEF" });
  }
});
app.post("/api/login/rescan", async (_req, reply) => {
  try {
    await manager.resetLogin();
    const result = await manager.startLogin({ force: true });
    manager.waitForScan(12e4).then((scanResult) => {
      if (scanResult.connected) {
        manager.startPolling().catch(
          (err) => console.error("[Sidecar] Auto-polling start failed:", err)
        );
      }
    }).catch((err) => console.error("[Sidecar] Background waitForScan failed:", err));
    return reply.send({
      success: true,
      message: "QR code generated",
      data: {
        qrCodeUrl: result.qrCodeUrl,
        qrImageUrl: result.qrImageUrl,
        sessionKey: result.sessionKey,
        status: "qr_ready"
      }
    });
  } catch (err) {
    console.error("[Sidecar]", err);
    return reply.status(500).send({ success: false, message: "\u670D\u52A1\u5668\u9519\u8BEF" });
  }
});
app.post("/api/login/wait", async (req, reply) => {
  try {
    const state = manager.getState();
    if (state.status === "connected" && state.accountId) {
      return reply.send({
        success: true,
        message: "Already connected",
        data: {
          connected: true,
          accountId: state.accountId,
          status: "connected"
        }
      });
    }
    const body = req.body;
    const result = await manager.waitForScan(body.timeoutMs || 12e4);
    if (result.connected) {
      manager.startPolling().catch(
        (err) => console.error("[Sidecar] Polling start failed:", err)
      );
    }
    return reply.send({
      success: result.connected,
      message: result.message,
      data: {
        connected: result.connected,
        accountId: result.accountId,
        status: manager.getState().status
      }
    });
  } catch (err) {
    console.error("[Sidecar]", err);
    return reply.status(500).send({ success: false, message: "\u670D\u52A1\u5668\u9519\u8BEF" });
  }
});
app.get("/api/qrcode", async (_req, reply) => {
  const state = manager.getState();
  if (!state.qrCodeUrl) {
    return reply.status(404).send({
      success: false,
      message: "No QR code. Call /api/login/start first."
    });
  }
  return reply.send({
    success: true,
    data: { qrCodeUrl: state.qrCodeUrl, status: state.status }
  });
});
app.post("/api/login/reconnect", async (_req, reply) => {
  try {
    await manager.stopPolling();
    await manager.startPolling();
    return reply.send({ success: true, message: "\u5DF2\u91CD\u65B0\u8FDE\u63A5" });
  } catch (err) {
    console.error("[Sidecar]", err);
    return reply.status(500).send({ success: false, message: "\u670D\u52A1\u5668\u9519\u8BEF" });
  }
});
app.post("/api/send", async (req, reply) => {
  try {
    const body = req.body;
    if (!body.toUserId || !body.text) {
      return reply.status(422).send({
        success: false,
        message: "toUserId and text are required"
      });
    }
    const deliveryKey = body.deliveryKey || "";
    const idempotencyKey = req.headers["idempotency-key"] || "";
    if (deliveryKey && idempotencyKey && deliveryKey !== idempotencyKey) {
      return reply.status(409).send({ success: false, message: "deliveryKey and Idempotency-Key mismatch" });
    }
    const effectiveKey = deliveryKey || idempotencyKey;
    const ctxToken = body.contextToken || contextTokenCache.get(body.toUserId) || "";
    const result = await manager.sendTextMessage(body.toUserId, body.text, ctxToken, effectiveKey || void 0);
    return reply.send({ success: true, message: "Sent", duplicate: result.duplicate, deliveryKey: effectiveKey || "" });
  } catch (err) {
    console.error("[Sidecar]", err);
    return reply.status(500).send({ success: false, message: "\u670D\u52A1\u5668\u9519\u8BEF" });
  }
});
app.post("/api/send-voice", async (req, reply) => {
  try {
    const body = req.body;
    if (!body.toUserId || !body.audioUrl) {
      return reply.status(422).send({
        success: false,
        message: "toUserId and audioUrl are required"
      });
    }
    const deliveryKey = body.deliveryKey || "";
    const idempotencyKey = req.headers["idempotency-key"] || "";
    if (deliveryKey && idempotencyKey && deliveryKey !== idempotencyKey) {
      return reply.status(409).send({ success: false, message: "deliveryKey and Idempotency-Key mismatch" });
    }
    const effectiveKey = deliveryKey || idempotencyKey;
    const fullAudioUrl = body.audioUrl.startsWith("http") ? body.audioUrl : sidecarConfig.coreUrl + body.audioUrl;
    const audioResp = await fetch(fullAudioUrl, { signal: AbortSignal.timeout(3e4) });
    if (!audioResp.ok) throw new Error("Audio download failed: " + audioResp.status);
    const audioBuffer = Buffer.from(await audioResp.arrayBuffer());
    const result = await manager.sendVoiceMessage(body.toUserId, audioBuffer, 7, 0, body.contextToken, effectiveKey || void 0);
    return reply.send({ success: true, message: "Voice sent", duplicate: result.duplicate, deliveryKey: effectiveKey || "" });
  } catch (err) {
    console.error("[Sidecar] send-voice error:", err.message);
    return reply.status(500).send({ success: false, message: err.message || "\u670D\u52A1\u5668\u9519\u8BEF" });
  }
});
app.post("/api/send-image", async (req, reply) => {
  try {
    const body = req.body;
    if (!body.toUserId || !body.assetUrl && !body.fallbackUrl) return reply.status(422).send({ success: false, message: "toUserId and assetUrl are required" });
    const deliveryKey = body.deliveryKey || "";
    const idempotencyKey = req.headers["idempotency-key"] || "";
    if (deliveryKey && idempotencyKey && deliveryKey !== idempotencyKey) {
      return reply.status(409).send({ success: false, message: "deliveryKey and Idempotency-Key mismatch" });
    }
    const effectiveKey = deliveryKey || idempotencyKey;
    const candidates = [body.assetUrl, body.fallbackUrl].filter(Boolean);
    let buffer = null;
    for (const candidate of candidates) {
      const url = candidate.startsWith("http") ? candidate : sidecarConfig.coreUrl + candidate;
      try {
        const response = await fetch(url, { signal: AbortSignal.timeout(3e4) });
        if (!response.ok) continue;
        buffer = Buffer.from(await response.arrayBuffer());
        break;
      } catch {
      }
    }
    if (!buffer) throw new Error("\u8868\u60C5\u8D44\u6E90\u4E0B\u8F7D\u5931\u8D25");
    const result = await manager.sendImageMessage(body.toUserId, buffer, body.contextToken, effectiveKey || void 0);
    return reply.send({ success: true, message: "Image sent", duplicate: result.duplicate, deliveryKey: effectiveKey || "" });
  } catch (err) {
    return reply.status(500).send({ success: false, message: err.message || "\u670D\u52A1\u5668\u9519\u8BEF" });
  }
});
try {
  const hasAccount = manager.loadSavedAccount();
  await app.listen({ host: sidecarConfig.host, port: sidecarConfig.port });
  console.log("");
  console.log("  ========================================");
  console.log("    OpenClaw WeChat Sidecar Server");
  console.log("    Listen:    http://" + sidecarConfig.host + ":" + sidecarConfig.port);
  console.log("    Account:   " + (hasAccount ? manager.getState().accountId : "(not logged in)"));
  console.log("    Core URL:  " + sidecarConfig.coreUrl);
  console.log("  ========================================");
  console.log("");
  if (hasAccount) {
    manager.startPolling().catch(
      (err) => console.error("[Sidecar] Auto-polling failed:", err)
    );
  }
} catch (err) {
  app.log.error(err);
  process.exit(1);
}

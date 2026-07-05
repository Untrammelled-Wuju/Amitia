// src/index.ts
import Fastify from "fastify";
import cors from "@fastify/cors";

// src/config.ts
function envStr(key, fallback) {
  return process.env[key] || fallback;
}
var qqSidecarConfig = {
  mergeWindowMs: parseInt(process.env.MERGE_WINDOW_MS || "6000", 10),
  host: envStr("QQ_SIDECAR_HOST", "127.0.0.1"),
  port: parseInt(envStr("QQ_SIDECAR_PORT", "9877"), 10),
  coreUrl: envStr("CORE_URL", "http://127.0.0.1:8899"),
  bridgeApiToken: envStr("BRIDGE_API_TOKEN", ""),
  qqbot: {
    appId: envStr("QQBOT_APP_ID", ""),
    token: envStr("QQBOT_TOKEN", ""),
    sandbox: envStr("QQBOT_SANDBOX", "false") === "true"
  }
};

// src/qqbot-client.ts
import WebSocket from "ws";
import fs from "node:fs";
var TOKEN_URL = "https://bots.qq.com/app/getAppAccessToken";
var API_BASE = "https://api.sgroup.qq.com";
var FULL_INTENTS = 1 << 30 | 1 << 12 | 1 << 25 | 1 << 26;
var QQBotClient = class {
  ws = null;
  config = null;
  seq = 0;
  sessionId = "";
  heartbeatTimer = null;
  reconnectTimer = null;
  handlers = [];
  loginStatus = "disconnected";
  accountId = "";
  botName = "";
  reconnectAttempts = 0;
  maxReconnectAttempts = 3;
  accessToken = "";
  accessTokenExpiry = 0;
  _messageCount = 0;
  _manualDisconnect = false;
  lastErrorMessage = "";
  get apiBase() {
    if (!this.config) return "https://api.sgroup.qq.com";
    return this.config.sandbox ? "https://sandbox.api.sgroup.qq.com" : "https://api.sgroup.qq.com";
  }
  getLastError() {
    return this.lastErrorMessage;
  }
  getStatus() {
    return this.loginStatus;
  }
  getMessageCount() {
    return this._messageCount;
  }
  getAccountId() {
    return this.accountId;
  }
  isOnline() {
    return this.loginStatus === "online";
  }
  constructor() {
    console.log("[QQBot] Client initialized");
  }
  debugLog(msg) {
    const ts = (/* @__PURE__ */ new Date()).toISOString();
    const line = `[${ts}] ${msg}`;
    console.log(line);
    try {
      fs.appendFileSync("qqbot-debug.log", line + "\n");
    } catch {
    }
  }
  onMessage(handler) {
    this.handlers.push(handler);
  }
  async connect(config) {
    this.reconnectAttempts = 0;
    this.identifyRetries = 0;
    this.lastErrorMessage = "";
    this._manualDisconnect = false;
    await this._doConnect(config);
  }
  async _doConnect(config) {
    if (this.ws) {
      this.disconnect();
    }
    this.config = config;
    this.loginStatus = "connecting";
    this.accountId = config.appId;
    this.debugLog(`\u6B63\u5728\u8FDE\u63A5... AppID=${config.appId} sandbox=${config.sandbox}`);
    try {
      const wsUrl = await this.getGatewayUrl();
      this.debugLog(`Gateway URL: ${wsUrl}`);
      this.connectWebSocket(wsUrl);
    } catch (err) {
      this.debugLog(`\u83B7\u53D6Gateway\u5931\u8D25: ` + err.message);
      this.lastErrorMessage = `Gateway\u8BF7\u6C42\u5931\u8D25: ` + err.message;
      this.loginStatus = "disconnected";
      if (!this._manualDisconnect && this.reconnectAttempts < this.maxReconnectAttempts) {
        this.scheduleReconnect();
      }
    }
  }
  async getAccessToken() {
    if (this.accessToken && Date.now() < this.accessTokenExpiry - 6e4) {
      return this.accessToken;
    }
    const resp = await fetch(TOKEN_URL, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ appId: this.config.appId, clientSecret: this.config.token }),
      signal: AbortSignal.timeout(15e3)
    });
    if (!resp.ok) {
      const text = await resp.text();
      throw new Error(`\u83B7\u53D6AccessToken\u5931\u8D25 (${resp.status}): ${text}`);
    }
    const data = await resp.json();
    this.accessToken = data.access_token;
    this.accessTokenExpiry = Date.now() + parseInt(data.expires_in || "3600") * 1e3;
    this.debugLog(`AccessToken\u5DF2\u83B7\u53D6`);
    return this.accessToken;
  }
  async getGatewayUrl() {
    const at = await this.getAccessToken();
    const resp = await fetch(`${API_BASE}/gateway`, {
      headers: { "Authorization": `QQBot ${at}` },
      signal: AbortSignal.timeout(15e3)
    });
    if (!resp.ok) {
      const text = await resp.text();
      throw new Error(`Gateway\u8BF7\u6C42\u5931\u8D25 (${resp.status}): ${text}`);
    }
    const data = await resp.json();
    return data.url;
  }
  connectWebSocket(wsUrl) {
    this.ws = new WebSocket(wsUrl);
    this.ws.on("open", () => {
      this.debugLog("WebSocket\u5DF2\u8FDE\u63A5");
    });
    this.ws.on("message", (data) => {
      try {
        const payload = JSON.parse(data.toString());
        this.handleGatewayMessage(payload);
      } catch (e) {
        console.error("[QQBot] \u6D88\u606F\u89E3\u6790\u5931\u8D25:", e);
      }
    });
    this.ws.on("close", (code, reason) => {
      const reasonStr = reason.toString();
      console.log(`[QQBot] WebSocket\u65AD\u5F00: code=${code} reason=${reasonStr}`);
      this.stopHeartbeat();
      if (code === 4004 || code === 4009 || code === 4010 || code === 4011 || code === 4012 || code === 4013 || code === 4014) {
        this.debugLog(`\u9274\u6743\u5931\u8D25 (code=${code})\uFF0C\u505C\u6B62\u91CD\u8FDE`);
        this.lastErrorMessage = `\u9274\u6743\u5931\u8D25: WebSocket\u5173\u95ED\u7801=${code}`;
        this.loginStatus = "disconnected";
        this.reconnectAttempts = this.maxReconnectAttempts;
        return;
      }
      if (this.loginStatus !== "disconnected") {
        if (this._manualDisconnect) {
          this.loginStatus = "disconnected";
        } else {
          this.loginStatus = "connecting";
          this.scheduleReconnect();
        }
      }
    });
    this.ws.on("error", (err) => {
      console.error("[QQBot] WebSocket\u9519\u8BEF:", err.message);
    });
  }
  handleGatewayMessage(payload) {
    const { op, d, s, t } = payload;
    if (s) this.seq = s;
    switch (op) {
      case 0:
        this.handleDispatch(t, d);
        break;
      case 10:
        this.debugLog(`Hello, heartbeat\u95F4\u9694=${d.heartbeat_interval}ms`);
        this.sessionId = "";
        this.startHeartbeat(d.heartbeat_interval);
        this.sendIdentify();
        break;
      case 11:
        break;
      case 7:
        console.log("[QQBot] \u670D\u52A1\u7AEF\u8981\u6C42\u91CD\u8FDE");
        this.reconnect();
        break;
      case 9:
        console.log("[QQBot] Session\u65E0\u6548, d=" + JSON.stringify(d));
        if (d === false) {
          this.debugLog("\u9274\u6743\u5931\u8D25\uFF0CToken\u53EF\u80FD\u65E0\u6548\uFF0C\u505C\u6B62\u91CD\u8FDE");
          this.lastErrorMessage = "\u9274\u6743\u5931\u8D25: AppID\u6216Token\u65E0\u6548";
          this.loginStatus = "disconnected";
          this.stopHeartbeat();
          this.reconnectAttempts = this.maxReconnectAttempts;
          return;
        }
        this.sessionId = "";
        setTimeout(() => this.sendIdentify(), 1e3);
        break;
      default:
        console.log(`[QQBot] \u672A\u77E5op: ${op}`);
    }
  }
  identifyRetries = 0;
  maxIdentifyRetries = 3;
  sendIdentify() {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      console.log("[QQBot] WebSocket\u672A\u8FDE\u63A5\uFF0C\u8DF3\u8FC7\u9274\u6743");
      return;
    }
    if (this.identifyRetries >= this.maxIdentifyRetries) {
      console.error("[QQBot] \u9274\u6743\u91CD\u8BD5\u6B21\u6570\u5DF2\u8FBE\u4E0A\u9650\uFF0C\u505C\u6B62\u91CD\u8FDE");
      this.loginStatus = "disconnected";
      this.reconnectAttempts = this.maxReconnectAttempts;
      this.disconnect();
      return;
    }
    this.identifyRetries++;
    const payload = {
      op: 2,
      d: {
        token: `QQBot ${this.accessToken}`,
        intents: FULL_INTENTS,
        shard: [0, 1],
        properties: {}
      }
    };
    console.log(`[QQBot] Identify token: ${payload.d.token.substring(0, 40)}... intents: ${payload.d.intents}`);
    this.ws.send(JSON.stringify(payload));
    this.debugLog(`\u5DF2\u53D1\u9001\u9274\u6743 (\u7B2C${this.identifyRetries}\u6B21)`);
  }
  startHeartbeat(interval) {
    this.stopHeartbeat();
    this.heartbeatTimer = setInterval(() => {
      if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        this.ws.send(JSON.stringify({ op: 1, d: this.seq }));
      }
    }, interval);
  }
  stopHeartbeat() {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer);
      this.heartbeatTimer = null;
    }
  }
  handleDispatch(eventType, data) {
    switch (eventType) {
      case "READY":
        this.loginStatus = "online";
        this.accountId = data?.user?.id || this.accountId;
        this.botName = data?.user?.username || "";
        this.sessionId = data?.session_id || "";
        this.identifyRetries = 0;
        this.reconnectAttempts = 0;
        this.debugLog(`\u5DF2\u4E0A\u7EBF! Bot=${this.botName} (${this.accountId})`);
        break;
      case "AT_MESSAGE_CREATE":
      case "GROUP_AT_MESSAGE_CREATE":
        this.debugLog(`[QQBot][RAW-DISPATCH] ${eventType} \u539F\u59CB\u6570\u636E:` + JSON.stringify(data).substring(0, 2e3));
        this.handleGroupMessage(data);
        break;
      case "C2C_MESSAGE_CREATE":
      case "DIRECT_MESSAGE_CREATE":
        this.debugLog(`[QQBot][RAW-DISPATCH] ${eventType} \u539F\u59CB\u6570\u636E:` + JSON.stringify(data).substring(0, 2e3));
        this.handleDirectMessage(data);
        break;
      default:
        break;
    }
  }
  handleGroupMessage(data) {
    const extracted = this.extractContent(data);
    const text = extracted.text;
    const isVoice = extracted.isVoice;
    const imageUrl = extracted.imageUrl;
    const fileUrl = extracted.fileUrl;
    if (!text && !isVoice && !imageUrl && !fileUrl) return;
    const msg = {
      fromUserId: data?.author?.id || "",
      toUserId: this.accountId,
      messageId: data?.id || "",
      text: text || (isVoice ? "[\u8BED\u97F3]" : ""),
      groupId: data?.group_id || data?.guild_id || "",
      createdAt: Date.now(),
      isVoice,
      imageUrl: imageUrl || void 0,
      videoUrl: extracted.videoUrl || void 0,
      fileUrl: fileUrl || void 0,
      fileName: extracted.fileName || void 0,
      voiceUrl: extracted.voiceUrl || void 0
    };
    this._messageCount++;
    const preview = msg.text.substring(0, 80);
    console.log(`[QQBot][\u7FA4:${msg.groupId}] ${msg.fromUserId}: ${preview}${msg.isVoice ? " (\u8BED\u97F3)" : ""}`);
    this.notifyHandlers(msg);
  }
  handleDirectMessage(data) {
    const extracted = this.extractContent(data);
    const text = extracted.text;
    const isVoice = extracted.isVoice;
    const imageUrl = extracted.imageUrl;
    const fileUrl = extracted.fileUrl;
    if (!text && !isVoice && !imageUrl && !fileUrl) return;
    const msg = {
      fromUserId: data?.author?.id || "",
      toUserId: this.accountId,
      messageId: data?.id || "",
      text: text || (isVoice ? "[\u8BED\u97F3]" : ""),
      createdAt: Date.now(),
      isVoice,
      imageUrl: imageUrl || void 0,
      fileUrl: fileUrl || void 0,
      fileName: extracted.fileName || void 0,
      voiceUrl: extracted.voiceUrl || void 0
    };
    this._messageCount++;
    const preview = msg.text.substring(0, 80);
    console.log(`[QQBot][\u79C1\u804A] ${msg.fromUserId}: ${preview}${msg.isVoice ? " (\u8BED\u97F3)" : ""}`);
    this.notifyHandlers(msg);
  }
  extractContent(data) {
    const rawDataId = data?.id || "unknown";
    this.debugLog("[QQBot][EXTRACT] msgId=" + rawDataId + " content\u7C7B\u578B=" + typeof data?.content + " isArray=" + Array.isArray(data?.content) + " attachments=" + (data?.attachments ? JSON.stringify(data.attachments).substring(0, 500) : "\u65E0"));
    if (typeof data.content === "object" && !Array.isArray(data.content)) {
      this.debugLog("[QQBot][EXTRACT] msgId=" + rawDataId + " content\u5BF9\u8C61keys=" + Object.keys(data.content).join(",") + " content=" + JSON.stringify(data.content).substring(0, 1e3));
    }
    if (Array.isArray(data.content)) {
      this.debugLog("[QQBot][EXTRACT] msgId=" + rawDataId + " content\u6570\u7EC4\u957F\u5EA6=" + data.content.length + " types=" + data.content.map((c) => c.type || c.msg_type || "?").join(","));
    }
    const hasAttachmentsVideo = data?.attachments?.some(
      (a) => a?.content_type?.startsWith("video/") || a?.type === "video" || a?.content_type === "video"
    );
    if (hasAttachmentsVideo) {
      const vidAtt = data.attachments.find(
        (a) => a?.content_type?.startsWith("video/") || a?.type === "video" || a?.content_type === "video"
      );
      const vidUrl = vidAtt?.url || vidAtt?.src_url || vidAtt?.url_src || "";
      const text = typeof data.content === "string" ? data.content.trim() : "";
      this.debugLog("[QQBot][VIDEO-DETECT] msgId=" + rawDataId + " \u68C0\u6D4B\u5230\u89C6\u9891! url=" + vidUrl);
      if (text) return { text, isVoice: false, imageUrl: "", videoUrl: vidUrl };
      return { text: "", isVoice: false, imageUrl: "", videoUrl: vidUrl };
    }
    const hasAttachmentsImage = data?.attachments?.some(
      (a) => a?.content_type?.startsWith("image/") || a?.type === "image" || a?.content_type === "image"
    );
    if (hasAttachmentsImage) {
      const imgAtt = data.attachments.find(
        (a) => a?.content_type?.startsWith("image/") || a?.type === "image" || a?.content_type === "image"
      );
      const imgUrl = imgAtt?.url || "";
      this.debugLog("[QQBot][IMAGE-DETECT] msgId=" + rawDataId + " \u68C0\u6D4B\u5230\u56FE\u7247! url=" + imgUrl);
      if (typeof data?.content === "string" && data.content.trim()) {
        return { text: data.content.trim(), isVoice: false, imageUrl: imgUrl };
      }
      if (Array.isArray(data?.content)) {
        const iparts = data.content.filter((c) => c.type === "text" && c.text).map((c) => c.text);
        const iv = iparts.length > 0 ? iparts.join("") : "";
        return { text: iv, isVoice: false, imageUrl: imgUrl };
      }
      return { text: "", isVoice: false, imageUrl: imgUrl };
    }
    if (typeof data?.content === "string") {
      const text = data.content.trim();
      if (text) return { text, isVoice: false, imageUrl: "" };
    }
    if (data?.content && typeof data.content === "object") {
      if (Array.isArray(data.content)) {
        const textParts = data.content.filter((s) => s.type === "text" && s.text).map((s) => s.text);
        const hasVoice = data.content.some(
          (s) => s.type === "voice" || s.type === "audio" || s.msg_type === "voice" || s.msg_type === "audio"
        );
        const contentImageUrls = data.content.filter((c) => c.type === "image" || c.msg_type === "image").map((c) => c.url || "").filter(Boolean);
        if (textParts.length > 0) return { text: textParts.join(""), isVoice: hasVoice, imageUrl: contentImageUrls[0] || "" };
        if (hasVoice) {
          this.debugLog("[QQBot][VOICE-DETECT] msgId=" + rawDataId + " \u68C0\u6D4B\u5230\u8BED\u97F3\u6D88\u606F! content\u6570\u7EC4\u8BE6\u60C5:" + JSON.stringify(data.content).substring(0, 2e3));
          return { text: "[\u8BED\u97F3]", isVoice: true, imageUrl: contentImageUrls[0] || "" };
        }
        if (contentImageUrls.length > 0) {
          this.debugLog("[QQBot][IMAGE-DETECT] msgId=" + rawDataId + " \u68C0\u6D4B\u5230content\u6570\u7EC4\u4E2D\u7684\u56FE\u7247! url=" + contentImageUrls[0]);
          return { text: "", isVoice: false, imageUrl: contentImageUrls[0] };
        }
        return { text: "", isVoice: false, imageUrl: "" };
      }
      if (typeof data.content.text === "string") {
        const text = data.content.text.trim();
        if (text) return { text, isVoice: false, imageUrl: "" };
      }
    }
    const hasAttachmentsVoice = data?.attachments?.some(
      (a) => a?.content_type?.startsWith("audio/") || a?.type === "voice" || a?.content_type === "voice"
    );
    if (hasAttachmentsVoice) {
      const voiceAtt = data.attachments.find(
        (a) => a?.content_type?.startsWith("audio/") || a?.type === "voice" || a?.content_type === "voice"
      );
      const asrText = voiceAtt?.asr_refer_text || "";
      this.debugLog("[QQBot][VOICE-DETECT] msgId=" + rawDataId + " \u68C0\u6D4B\u5230\u8BED\u97F3\u6D88\u606F! attachments\u8BE6\u60C5:" + JSON.stringify(data.attachments).substring(0, 2e3));
      this.debugLog("[QQBot][VOICE-ASR] msgId=" + rawDataId + " QQ\u8BED\u97F3\u8BC6\u522B\u6587\u672C: " + asrText);
      if (asrText) {
        return { text: asrText, isVoice: true, imageUrl: "", voiceUrl: voiceAtt?.url || "" };
      }
      return { text: "[\u8BED\u97F3]", isVoice: true, imageUrl: "", voiceUrl: voiceAtt?.url || "" };
    }
    const hasAttachmentsFile = data?.attachments?.some(
      (a) => a?.content_type?.startsWith("file/") || a?.type === "file" || a?.content_type === "file"
    );
    if (hasAttachmentsFile) {
      const fileAtt = data.attachments.find(
        (a) => a?.content_type?.startsWith("file/") || a?.type === "file" || a?.content_type === "file"
      );
      const fUrl = fileAtt?.url || "";
      const fName = fileAtt?.filename || fileAtt?.file_name || "";
      const fContentType = fileAtt?.content_type || "";
      this.debugLog("[QQBot][FILE-DETECT] msgId=" + rawDataId + " \u68C0\u6D4B\u5230\u6587\u4EF6! url=" + fUrl + " name=" + fName + " contentType=" + fContentType);
      if (fUrl) {
        const ftext = typeof data?.content === "string" ? data.content.trim() : "";
        return { text: ftext, isVoice: false, imageUrl: "", fileUrl: fUrl, fileName: fName, fileContentType: fContentType };
      }
    }
    const hasAttachmentsOther = data?.attachments?.some(
      (a) => a?.url && !a?.content_type?.startsWith("image/") && !a?.content_type?.startsWith("video/") && !a?.content_type?.startsWith("audio/") && !(a?.type === "voice")
    );
    if (hasAttachmentsOther) {
      const otherAtt = data.attachments.find(
        (a) => a?.url && !a?.content_type?.startsWith("image/") && !a?.content_type?.startsWith("video/") && !a?.content_type?.startsWith("audio/") && !(a?.type === "voice")
      );
      const oUrl = otherAtt?.url || "";
      const oName = otherAtt?.filename || otherAtt?.file_name || "";
      const oContentType = otherAtt?.content_type || "";
      this.debugLog("[QQBot][FILE-DETECT] msgId=" + rawDataId + " \u68C0\u6D4B\u5230\u901A\u7528\u9644\u4EF6(\u53EF\u80FD\u662F\u6587\u4EF6)! url=" + oUrl + " name=" + oName + " contentType=" + oContentType);
      if (oUrl) {
        const otext = typeof data?.content === "string" ? data.content.trim() : "";
        return { text: otext, isVoice: false, imageUrl: "", fileUrl: oUrl, fileName: oName, fileContentType: oContentType };
      }
    }
    return { text: "", isVoice: false, imageUrl: "" };
  }
  notifyHandlers(msg) {
    for (const handler of this.handlers) {
      try {
        handler(msg);
      } catch (err) {
        console.error("[QQBot] \u6D88\u606F\u5904\u7406\u5931\u8D25:", err.message);
      }
    }
  }
  async sendGroupMsg(groupId, text) {
    if (!this.config) throw new Error("\u672A\u8FDE\u63A5");
    return this._sendWithRetry(async (token) => {
      const url = `${this.apiBase}/v2/groups/${groupId}/messages`;
      const resp = await fetch(url, {
        method: "POST",
        headers: {
          "Authorization": `QQBot ${token}`,
          "Content-Type": "application/json"
        },
        body: JSON.stringify({
          content: text,
          msg_type: 0
        })
      });
      if (!resp.ok) {
        const errText = await resp.text();
        throw new Error(`\u53D1\u9001\u7FA4\u6D88\u606F\u5931\u8D25 (${resp.status}): ${errText}`);
      }
      console.log(`[QQBot] \u53D1\u9001\u7FA4\u6D88\u606F: ->${groupId} (${text.length} chars)`);
    }, "\u53D1\u9001\u7FA4\u6D88\u606F");
  }
  async sendPrivateMsg(userId, text) {
    if (!this.config) throw new Error("\u672A\u8FDE\u63A5");
    return this._sendWithRetry(async (token) => {
      const url = `${this.apiBase}/v2/users/${userId}/messages`;
      const resp = await fetch(url, {
        method: "POST",
        headers: {
          "Authorization": `QQBot ${token}`,
          "Content-Type": "application/json"
        },
        body: JSON.stringify({
          content: text,
          msg_type: 0
        })
      });
      if (!resp.ok) {
        const errText = await resp.text();
        throw new Error(`\u53D1\u9001\u79C1\u804A\u5931\u8D25 (${resp.status}): ${errText}`);
      }
      console.log(`[QQBot] \u53D1\u9001\u79C1\u804A: ->${userId} (${text.length} chars)`);
    }, "\u53D1\u9001\u79C1\u804A");
  }
  async _sendWithRetry(sendFn, label) {
    let token = this.accessToken;
    if (!token) {
      token = await this.getAccessToken();
    }
    try {
      const result = await sendFn(token);
      return result;
    } catch (err) {
      const msg = err?.message || String(err);
      if (msg.includes("token not exist or expire") || msg.includes("11244")) {
        console.log(`[QQBot] Token\u5DF2\u8FC7\u671F\uFF0C\u5237\u65B0\u540E\u91CD\u8BD5${label}`);
        this.accessToken = "";
        this.accessTokenExpiry = 0;
        token = await this.getAccessToken();
        const retryResult = await sendFn(token);
        console.log(`[QQBot] ${label}\u91CD\u8BD5\u6210\u529F`);
        return retryResult;
      } else {
        throw err;
      }
    }
  }
  async uploadGroupMedia(groupId, fileBuffer, fileName, fileType) {
    if (!this.config) throw new Error("\u672A\u8FDE\u63A5");
    return this._sendWithRetry(async (token) => {
      const b64 = fileBuffer.toString("base64");
      this.debugLog("[QQBot][UPLOAD-JSON] groupId=" + groupId + " fileType=" + fileType + " b64Len=" + b64.length);
      const url = this.apiBase + "/v2/groups/" + groupId + "/files";
      const resp = await fetch(url, {
        method: "POST",
        headers: {
          "Authorization": "QQBot " + token,
          "Content-Type": "application/json"
        },
        body: JSON.stringify({ file_type: fileType, file_data: b64, srv_send_msg: false })
      });
      if (!resp.ok) {
        const errText = await resp.text();
        this.debugLog("[QQBot][UPLOAD-ERR] \u4E0A\u4F20\u7FA4\u6587\u4EF6\u5931\u8D25 status=" + resp.status + " body=" + errText.substring(0, 500));
        throw new Error("\u4E0A\u4F20\u7FA4\u6587\u4EF6\u5931\u8D25 (" + resp.status + "): " + errText);
      }
      const data = await resp.json();
      if (!data.file_info) throw new Error("\u4E0A\u4F20\u6210\u529F\u4F46\u672A\u8FD4\u56DEfile_info");
      return data.file_info;
    }, "\u4E0A\u4F20\u7FA4\u5A92\u4F53");
  }
  async uploadPrivateMedia(userId, fileBuffer, fileName, fileType) {
    if (!this.config) throw new Error("\u672A\u8FDE\u63A5");
    return this._sendWithRetry(async (token) => {
      const b64 = fileBuffer.toString("base64");
      this.debugLog("[QQBot][UPLOAD-JSON] userId=" + userId + " fileType=" + fileType + " b64Len=" + b64.length);
      const url = this.apiBase + "/v2/users/" + userId + "/files";
      const resp = await fetch(url, {
        method: "POST",
        headers: {
          "Authorization": "QQBot " + token,
          "Content-Type": "application/json"
        },
        body: JSON.stringify({ file_type: fileType, file_data: b64, srv_send_msg: false })
      });
      if (!resp.ok) {
        const errText = await resp.text();
        this.debugLog("[QQBot][UPLOAD-ERR] \u4E0A\u4F20\u79C1\u804A\u6587\u4EF6\u5931\u8D25 status=" + resp.status + " body=" + errText.substring(0, 500));
        throw new Error("\u4E0A\u4F20\u79C1\u804A\u6587\u4EF6\u5931\u8D25 (" + resp.status + "): " + errText);
      }
      const data = await resp.json();
      if (!data.file_info) throw new Error("\u4E0A\u4F20\u6210\u529F\u4F46\u672A\u8FD4\u56DEfile_info");
      return data.file_info;
    }, "\u4E0A\u4F20\u79C1\u804A\u5A92\u4F53");
  }
  async sendGroupVoice(groupId, fileInfo) {
    if (!this.config) throw new Error("\u672A\u8FDE\u63A5");
    return this._sendWithRetry(async (token) => {
      const url = this.apiBase + "/v2/groups/" + groupId + "/messages";
      const resp = await fetch(url, {
        method: "POST",
        headers: {
          "Authorization": "QQBot " + token,
          "Content-Type": "application/json"
        },
        body: JSON.stringify({
          msg_type: 7,
          media: { file_info: fileInfo },
          msg_id: "",
          msg_seq: Math.floor(Date.now() / 1e3)
        })
      });
      if (!resp.ok) {
        const errText = await resp.text();
        throw new Error("\u53D1\u9001\u7FA4\u8BED\u97F3\u5931\u8D25 (" + resp.status + "): " + errText);
      }
      console.log("[QQBot] \u53D1\u9001\u7FA4\u8BED\u97F3: ->" + groupId);
    }, "\u53D1\u9001\u7FA4\u8BED\u97F3");
  }
  async sendPrivateVoice(userId, fileInfo) {
    if (!this.config) throw new Error("\u672A\u8FDE\u63A5");
    return this._sendWithRetry(async (token) => {
      const url = this.apiBase + "/v2/users/" + userId + "/messages";
      const resp = await fetch(url, {
        method: "POST",
        headers: {
          "Authorization": "QQBot " + token,
          "Content-Type": "application/json"
        },
        body: JSON.stringify({
          msg_type: 7,
          media: { file_info: fileInfo },
          msg_id: "",
          msg_seq: Math.floor(Date.now() / 1e3)
        })
      });
      if (!resp.ok) {
        const errText = await resp.text();
        throw new Error("\u53D1\u9001\u79C1\u804A\u8BED\u97F3\u5931\u8D25 (" + resp.status + "): " + errText);
      }
      console.log("[QQBot] \u53D1\u9001\u79C1\u804A\u8BED\u97F3: ->" + userId);
    }, "\u53D1\u9001\u79C1\u804A\u8BED\u97F3");
  }
  async downloadImage(url) {
    if (!this.config) return null;
    try {
      const token = await this.getAccessToken();
      const resp = await fetch(url, {
        headers: { "Authorization": "QQBot " + token },
        signal: AbortSignal.timeout(3e4)
      });
      if (!resp.ok) {
        this.debugLog("[QQBot][IMAGE-DL] \u4E0B\u8F7D\u56FE\u7247\u5931\u8D25 status=" + resp.status);
        return null;
      }
      const arrayBuffer = await resp.arrayBuffer();
      const buffer = Buffer.from(arrayBuffer);
      const contentType = resp.headers.get("content-type") || "image/png";
      this.debugLog("[QQBot][IMAGE-DL] \u56FE\u7247\u4E0B\u8F7D\u6210\u529F size=" + buffer.length + " type=" + contentType);
      return { buffer, contentType };
    } catch (err) {
      this.debugLog("[QQBot][IMAGE-DL] \u4E0B\u8F7D\u56FE\u7247\u5F02\u5E38: " + err.message);
      return null;
    }
  }
  disconnect() {
    console.log("[QQBot] \u65AD\u5F00\u8FDE\u63A5");
    this._manualDisconnect = true;
    this.loginStatus = "disconnected";
    this.stopHeartbeat();
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
      this.ws.removeAllListeners();
      this.ws.close();
      this.ws = null;
    }
  }
  reconnect() {
    if (this.ws) {
      this.ws.removeAllListeners();
      this.ws.close();
      this.ws = null;
    }
    this.stopHeartbeat();
    if (this.config) {
      this._doConnect(this.config);
    }
  }
  scheduleReconnect() {
    if (this._manualDisconnect) {
      this.debugLog("\u624B\u52A8\u65AD\u5F00\uFF0C\u8DF3\u8FC7\u81EA\u52A8\u91CD\u8FDE");
      return;
    }
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      this.debugLog("\u91CD\u8FDE\u6B21\u6570\u5DF2\u8FBE\u4E0A\u9650\uFF0C\u505C\u6B62\u91CD\u8FDE");
      this.lastErrorMessage = "\u91CD\u8FDE\u6B21\u6570\u5DF2\u8FBE\u4E0A\u9650\uFF0C\u8BF7\u68C0\u67E5\u7F51\u7EDC\u548C\u51ED\u8BC1";
      this.loginStatus = "disconnected";
      return;
    }
    this.reconnectAttempts++;
    const delay = Math.min(1e3 * Math.pow(2, this.reconnectAttempts), 3e4);
    this.debugLog(`${delay}ms\u540E\u7B2C${this.reconnectAttempts}\u6B21\u91CD\u8FDE`);
    this.reconnectTimer = setTimeout(() => {
      if (this.config) {
        this._doConnect(this.config);
      }
    }, delay);
  }
};

// src/qqbot-persist.ts
import fs2 from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
var __filename = fileURLToPath(import.meta.url);
var __dirname = path.dirname(__filename);
var CONFIG_FILE = path.resolve(__dirname, "..", "data", "qqbot-config.json");
function loadQQBotConfig() {
  try {
    if (fs2.existsSync(CONFIG_FILE)) {
      const raw = fs2.readFileSync(CONFIG_FILE, "utf-8");
      const cfg = JSON.parse(raw);
      if (cfg.appId && cfg.token) {
        console.log("[QQBot] \u5DF2\u4ECE\u78C1\u76D8\u52A0\u8F7D\u6301\u4E45\u5316\u51ED\u8BC1 appId=" + cfg.appId);
        return cfg;
      }
    }
  } catch (err) {
    console.error("[QQBot] \u52A0\u8F7D\u6301\u4E45\u5316\u51ED\u8BC1\u5931\u8D25:", err.message);
  }
  return null;
}
function saveQQBotConfig(config) {
  try {
    const dir = path.dirname(CONFIG_FILE);
    if (!fs2.existsSync(dir)) fs2.mkdirSync(dir, { recursive: true });
    fs2.writeFileSync(CONFIG_FILE, JSON.stringify(config, null, 2), "utf-8");
    console.log("[QQBot] \u51ED\u8BC1\u5DF2\u6301\u4E45\u5316\u5230\u78C1\u76D8");
  } catch (err) {
    console.error("[QQBot] \u6301\u4E45\u5316\u51ED\u8BC1\u5931\u8D25:", err.message);
  }
}
function clearQQBotConfig() {
  try {
    if (fs2.existsSync(CONFIG_FILE)) {
      fs2.unlinkSync(CONFIG_FILE);
      console.log("[QQBot] \u6301\u4E45\u5316\u51ED\u8BC1\u5DF2\u6E05\u9664");
    }
  } catch (err) {
    console.error("[QQBot] \u6E05\u9664\u51ED\u8BC1\u5931\u8D25:", err.message);
  }
}

// src/file-router.ts
var MAGIC_BYTES = {
  "image/jpeg": [[255, 216, 255]],
  "image/png": [[137, 80, 78, 71]],
  "image/gif": [[71, 73, 70, 56]],
  "image/webp": [[82, 73, 70, 70]],
  "image/bmp": [[66, 77]],
  "application/pdf": [[37, 80, 68, 70]],
  "application/zip": [[80, 75, 3, 4]],
  "audio/mp3": [[255, 251], [255, 243], [255, 242], [73, 68, 51]],
  "audio/mpeg": [[255, 251], [255, 243], [255, 242], [73, 68, 51]],
  "audio/ogg": [[79, 103, 103, 83]],
  "audio/wav": [[82, 73, 70, 70]],
  "video/mp4": [[0, 0, 0, 24, 102, 116, 121, 112]]
};
var FileRouter = class {
  handlers = /* @__PURE__ */ new Map();
  register(type, handler) {
    this.handlers.set(type, handler);
    console.log("[FileRouter] \u6CE8\u518C\u5904\u7406\u5668: " + type);
  }
  async route(file) {
    const detectedType = this.detectFileType(file.buffer, file.mimeType);
    console.log("[FileRouter] \u6587\u4EF6\u7C7B\u578B\u68C0\u6D4B: fileName=" + file.fileName + " mimeType=" + file.mimeType + " detected=" + detectedType);
    const handler = this.handlers.get(detectedType);
    if (handler) {
      return handler(file);
    }
    const wildcardHandler = this.findWildcardHandler(detectedType);
    if (wildcardHandler) {
      return wildcardHandler(file);
    }
    console.log("[FileRouter] \u672A\u627E\u5230\u5904\u7406\u5668: " + detectedType);
    return null;
  }
  findWildcardHandler(mimeType) {
    const [category] = mimeType.split("/");
    const wildcardKey = category + "/*";
    return this.handlers.get(wildcardKey);
  }
  detectFileType(buffer, hintType) {
    if (hintType && this.isImageType(hintType)) {
      return hintType;
    }
    if (hintType && this.isVideoType(hintType)) {
      return hintType;
    }
    if (hintType && this.isAudioType(hintType)) {
      return hintType;
    }
    for (const [mimeType, magicSeq] of Object.entries(MAGIC_BYTES)) {
      for (const seq of magicSeq) {
        if (buffer.length >= seq.length) {
          let match = true;
          for (let i = 0; i < seq.length; i++) {
            if (buffer[i] !== seq[i]) {
              match = false;
              break;
            }
          }
          if (match) {
            if (mimeType === "audio/wav" && buffer.length > 8) {
              const waveId = buffer.slice(8, 12).toString("ascii");
              if (waveId !== "WAVE") continue;
            }
            if (mimeType === "application/zip" && buffer.length > 40) {
              const docType = buffer.slice(30, 40).toString("ascii");
              if (docType.includes("mimetype")) continue;
            }
            return mimeType;
          }
        }
      }
    }
    return hintType || "application/octet-stream";
  }
  isImageType(t) {
    return t.startsWith("image/");
  }
  isVideoType(t) {
    return t.startsWith("video/");
  }
  isAudioType(t) {
    return t.startsWith("audio/");
  }
};
function createDefaultRouter() {
  const router = new FileRouter();
  router.register("image/*", async (file) => {
    const b64 = file.buffer.toString("base64");
    console.log("[FileRouter] \u56FE\u7247\u6587\u4EF6\u8DEF\u7531: " + file.fileName + " mime=" + file.mimeType + " size=" + file.buffer.length);
    return {
      handler: "image",
      data: {
        mimeType: file.mimeType,
        base64: b64,
        fileName: file.fileName
      }
    };
  });
  router.register("audio/*", async (file) => {
    console.log("[FileRouter] \u97F3\u9891\u6587\u4EF6\u8DEF\u7531: " + file.fileName + " mime=" + file.mimeType);
    return {
      handler: "audio",
      data: {
        mimeType: file.mimeType,
        base64: file.buffer.toString("base64"),
        fileName: file.fileName
      }
    };
  });
  router.register("video/*", async (file) => {
    console.log("[FileRouter] \u89C6\u9891\u6587\u4EF6\u8DEF\u7531: " + file.fileName + " mime=" + file.mimeType);
    return {
      handler: "video",
      data: {
        mimeType: file.mimeType,
        base64: file.buffer.toString("base64"),
        fileName: file.fileName
      }
    };
  });
  return router;
}

// src/index.ts
import fs3 from "node:fs";
process.on("unhandledRejection", (reason) => {
  console.error("[QQ-Sidecar] Unhandled Rejection:", reason);
});
var app = Fastify({
  logger: { level: process.env.LOG_LEVEL || "info" }
});
await app.register(cors, {
  origin: [/^http:\/\/127\.0\.0\.1:\d+$/, /^http:\/\/localhost:\d+$/],
  methods: ["GET", "POST", "OPTIONS"],
  credentials: false
});
app.addHook("onSend", async (_req, reply) => {
  reply.header("X-Content-Type-Options", "nosniff");
  reply.header("X-Frame-Options", "DENY");
  void reply.header("X-Powered-By", "");
});
var qq = new QQBotClient();
var fileRouter = createDefaultRouter();
var msgBuffers = /* @__PURE__ */ new Map();
var processingLocks = /* @__PURE__ */ new Map();
qq.onMessage(async (msg) => {
  if (msg.isVoice) {
    const logLineV2 = (msgtxt) => {
      try {
        fs3.appendFileSync("forward-debug.log", (/* @__PURE__ */ new Date()).toISOString() + " " + msgtxt + "\n");
      } catch {
      }
    };
    console.log(`[QQ-Sidecar][VOICE-IN] ========== \u6536\u5230\u8BED\u97F3\u6D88\u606F ==========`);
    console.log(`[QQ-Sidecar][VOICE-IN] fromUserId=${msg.fromUserId} groupId=${msg.groupId || "\u79C1\u804A"}`);
    console.log(`[QQ-Sidecar][VOICE-IN] messageId=${msg.messageId} text="${msg.text}"`);
    console.log(`[QQ-Sidecar][VOICE-IN] =========================================`);
    logLineV2(`VOICE-IN: fromUserId=${msg.fromUserId} groupId=${msg.groupId || "none"} msgId=${msg.messageId}`);
  }
  const BUFFER_MS = qqSidecarConfig.mergeWindowMs;
  const key = msg.groupId || msg.fromUserId;
  const existing = msgBuffers.get(key);
  let imageData = "";
  if (msg.imageUrl) {
    try {
      const dl = await qq.downloadImage(msg.imageUrl);
      if (dl) {
        imageData = "data:" + dl.contentType + ";base64," + dl.buffer.toString("base64");
        console.log("[QQ-Sidecar][MSG-DL] len=" + imageData.length);
      }
    } catch {
    }
  }
  const item = { text: msg.text, fromUserId: msg.fromUserId, messageId: msg.messageId, createdAt: msg.createdAt, groupId: msg.groupId, isVoice: msg.isVoice || false, imageUrl: msg.imageUrl || "", fileUrl: msg.fileUrl || "", fileName: msg.fileName || "", voiceUrl: msg.voiceUrl || "", imageData };
  if (existing) {
    clearTimeout(existing.timer);
    existing.msgs.push(item);
  } else {
    msgBuffers.set(key, { msgs: [item], timer: null });
  }
  const entry = msgBuffers.get(key);
  entry.timer = setTimeout(async () => {
    const prevLock = processingLocks.get(key);
    if (prevLock) {
      try {
        await prevLock;
      } catch {
      }
    }
    const flog = (s) => {
      try {
        fs3.appendFileSync("forward-debug.log", (/* @__PURE__ */ new Date()).toISOString() + " " + s + "\n");
      } catch {
      }
    };
    let resolveLock;
    const lockPromise = new Promise((r) => {
      resolveLock = r;
    });
    processingLocks.set(key, lockPromise);
    msgBuffers.delete(key);
    const all = entry.msgs;
    flog("FWD-START msgs=" + all.length);
    const last = all[all.length - 1];
    const combined = all.map((m) => m.text).filter((t2) => t2.length > 0).join("\n");
    const wasVoice = all.some((m) => m.isVoice || false);
    if (wasVoice) {
      const logLineV3 = (msgtxt) => {
        try {
          fs3.appendFileSync("forward-debug.log", (/* @__PURE__ */ new Date()).toISOString() + " " + msgtxt + "\n");
        } catch {
        }
      };
      logLineV3(`Voice-FWD: userId=${last.fromUserId} groupId=${last.groupId || "none"} msgId=${last.messageId} text=${combined.substring(0, 100)}`);
      console.log(`[QQ-Sidecar][VOICE-FWD] \u8F6C\u53D1\u8BED\u97F3\u5230\u540E\u7AEF: fromUserId=${last.fromUserId} text="${combined.substring(0, 100)}"`);
    }
    let audioBase64 = "";
    if (wasVoice && last.voiceUrl) {
      try {
        const voiceDl = await qq.downloadImage(last.voiceUrl);
        if (voiceDl) {
          audioBase64 = voiceDl.buffer.toString("base64");
          console.log("[QQ-Sidecar][VOICE-DL] \u7528\u6237\u8BED\u97F3\u5DF2\u4E0B\u8F7D, size=" + voiceDl.buffer.length);
        }
      } catch {
      }
    }
    const headers = { "Content-Type": "application/json" };
    const t = qqSidecarConfig.bridgeApiToken;
    if (t) headers["Authorization"] = "Bearer " + t;
    try {
      let imageUrl = "";
      let videoUrl = "";
      const firstFileMsg = all.find((m) => m.fileUrl);
      if (firstFileMsg && firstFileMsg.fileUrl) {
        const fileResult = await qq.downloadImage(firstFileMsg.fileUrl);
        if (fileResult) {
          console.log("[QQ-Sidecar][FILE-FWD] \u6587\u4EF6\u5DF2\u4E0B\u8F7D, size=" + fileResult.buffer.length + " type=" + fileResult.contentType);
          const routeResult = await fileRouter.route({
            buffer: fileResult.buffer,
            fileName: firstFileMsg.fileName || "unknown",
            mimeType: fileResult.contentType
          });
          if (routeResult) {
            console.log("[QQ-Sidecar][FILE-ROUTE] \u6587\u4EF6\u8DEF\u7531: handler=" + routeResult.handler);
            if (routeResult.handler === "image" && routeResult.data?.base64) {
              imageUrl = "data:" + (routeResult.data.mimeType || fileResult.contentType) + ";base64," + routeResult.data.base64;
              console.log("[QQ-Sidecar][FILE-ROUTE] \u6587\u4EF6\u8BC6\u522B\u4E3A\u56FE\u7247, base64Len=" + imageUrl.length);
            } else if (routeResult.handler === "video" && routeResult.data?.base64) {
              videoUrl = "data:" + (routeResult.data.mimeType || fileResult.contentType) + ";base64," + routeResult.data.base64;
              console.log("[QQ-Sidecar][FILE-ROUTE] \u6587\u4EF6\u8BC6\u522B\u4E3A\u89C6\u9891, base64Len=" + videoUrl.length);
            } else if (routeResult.handler === "audio" && routeResult.data?.base64) {
              console.log("[QQ-Sidecar][FILE-ROUTE] \u6587\u4EF6\u8BC6\u522B\u4E3A\u97F3\u9891(\u6682\u4E0D\u5904\u7406)");
            } else {
              console.log("[QQ-Sidecar][FILE-ROUTE] \u672A\u77E5\u6587\u4EF6\u7C7B\u578B, handler=" + routeResult.handler);
            }
          } else {
            console.log("[QQ-Sidecar][FILE-ROUTE] \u672A\u627E\u5230\u6587\u4EF6\u5904\u7406\u5668");
          }
        }
      }
      flog("FWD-IMG check");
      const firstImageData = all.find((m) => m.imageData);
      if (!imageUrl && firstImageData && firstImageData.imageData) {
        imageUrl = firstImageData.imageData;
        flog("FWD-IMG using imageData len=" + imageUrl.length);
        console.log("[QQ-Sidecar][IMAGE-FWD] \u4F7F\u7528\u9884\u4E0B\u8F7D\u56FE\u7247, len=" + imageUrl.length);
      }
      const firstVideoMsg = all.find((m) => m.videoUrl);
      if (!videoUrl && firstVideoMsg?.videoUrl) {
        const vidResult = await qq.downloadImage(firstVideoMsg.videoUrl);
        if (vidResult) {
          videoUrl = "data:" + vidResult.contentType + ";base64," + vidResult.buffer.toString("base64");
          console.log("[QQ-Sidecar][VIDEO-FWD] \u89C6\u9891\u5DF2\u4E0B\u8F7D\u5E76\u7F16\u7801, size=" + vidResult.buffer.length);
        }
      }
      console.log("[QQ-Sidecar][WEBHOOK] text=" + combined.substring(0, 50) + " imageUrlLen=" + (imageUrl ? imageUrl.length : 0) + " videoUrlLen=" + (videoUrl ? videoUrl.length : 0));
      const resp = await fetch(`${qqSidecarConfig.coreUrl}/api/agent/webhook`, {
        method: "POST",
        headers,
        body: JSON.stringify({
          channel: "qq",
          accountId: qq.getAccountId() || "qqbot",
          conversationId: "conv-" + last.fromUserId,
          senderId: last.fromUserId,
          messageId: last.messageId,
          type: wasVoice ? "voice" : "text",
          text: combined,
          createdAt: last.createdAt,
          voiceMessage: wasVoice,
          audioBase64,
          imageUrl,
          videoUrl,
          skipTiming: true
        }),
        signal: AbortSignal.timeout(6e5)
      });
      const json = await resp.json();
      if (json?.code && json.code !== 200) {
        const errMsg = "AI\u670D\u52A1\u5F02\u5E38 [code=" + json.code + "] " + (json.msg || "");
        console.error("[QQ-Sidecar][WEBHOOK-ERR] " + errMsg);
        try {
          if (last.groupId) {
            await qq.sendGroupMsg(last.groupId, errMsg);
          } else {
            await qq.sendPrivateMsg(last.fromUserId, errMsg);
          }
        } catch {
        }
        return;
      }
      const logLine = (msgtxt) => {
        try {
          fs3.appendFileSync("forward-debug.log", (/* @__PURE__ */ new Date()).toISOString() + " " + msgtxt + "\n");
        } catch {
        }
      };
      logLine("Webhook response: " + JSON.stringify(json).substring(0, 300));
      if (json?.data?.outgoingMessage?.text) {
        const reply = json.data.outgoingMessage.text;
        logLine("Reply text (" + reply.length + " chars): " + reply.substring(0, 100));
        const forceVoice = json?.data?.outgoingMessage?.forceVoice === true;
        const shouldSendVoice = forceVoice || wasVoice && Math.random() < 0.8;
        logLine("Voice decision: wasVoice=" + wasVoice + " shouldSendVoice=" + shouldSendVoice);
        const audioUrls = json?.data?.outgoingMessage?.audioUrls || [];
        if (shouldSendVoice && audioUrls.length > 0) {
          try {
            logLine("Voice audioUrls: " + audioUrls.length);
            let voiceSent = false;
            for (let i = 0; i < audioUrls.length; i++) {
              try {
                const fullAudioUrl = qqSidecarConfig.coreUrl + audioUrls[i];
                const audioResp = await fetch(fullAudioUrl, { signal: AbortSignal.timeout(3e4) });
                if (audioResp.ok) {
                  const audioBuffer = Buffer.from(await audioResp.arrayBuffer());
                  logLine("Voice part " + (i + 1) + " audio: " + audioBuffer.length + " bytes");
                  const fileInfo = last.groupId ? await qq.uploadGroupMedia(last.groupId, audioBuffer, "voice" + i + ".mp3", 3) : await qq.uploadPrivateMedia(last.fromUserId, audioBuffer, "voice" + i + ".mp3", 3);
                  if (last.groupId) {
                    await qq.sendGroupVoice(last.groupId, fileInfo);
                  } else {
                    await qq.sendPrivateVoice(last.fromUserId, fileInfo);
                  }
                  logLine("Voice part " + (i + 1) + " sent OK");
                  voiceSent = true;
                } else {
                  logLine("Voice part " + (i + 1) + " audio download failed: " + audioResp.status);
                }
              } catch (partErr) {
                logLine("Voice part " + (i + 1) + " error: " + (partErr?.message || String(partErr)));
              }
              if (i < audioUrls.length - 1) await new Promise((r) => setTimeout(r, 800));
            }
            if (voiceSent) return;
            logLine("No voice parts sent successfully, falling back to text");
          } catch (ttsErr) {
            logLine("Voice send error: " + (ttsErr?.message || String(ttsErr)) + ", falling back to text");
          }
        }
        const parts = reply.split("\n").map((p) => p.trim()).filter((p) => p.length > 0);
        logLine("Reply parts: " + parts.length);
        for (let i = 0; i < parts.length; i++) {
          const sendTarget = last.groupId ? "group:" + last.groupId : "user:" + last.fromUserId;
          logLine("Sending part " + (i + 1) + "/" + parts.length + " to " + sendTarget + " text=" + parts[i].substring(0, 50));
          try {
            if (last.groupId) {
              await qq.sendGroupMsg(last.groupId, parts[i]);
            } else {
              await qq.sendPrivateMsg(last.fromUserId, parts[i]);
            }
            logLine("Part " + (i + 1) + " sent OK");
          } catch (sendErr) {
            logLine("Send FAILED for part " + (i + 1) + ": " + (sendErr?.message || String(sendErr)));
          }
          if (i < parts.length - 1) await new Promise((r) => setTimeout(r, 800));
        }
      } else {
        logLine("No outgoingMessage in response. Keys: " + Object.keys(json?.data || {}).join(","));
      }
    } catch (err) {
      try {
        fs3.appendFileSync("forward-debug.log", (/* @__PURE__ */ new Date()).toISOString() + " Forward FAILED: " + (err?.message || String(err)) + "\n");
      } catch {
      }
    } finally {
      processingLocks.delete(key);
      resolveLock();
    }
  }, BUFFER_MS);
});
app.post("/api/connect", async (req, reply) => {
  const body = req.body;
  const appId = body?.appId || qqSidecarConfig.qqbot.appId;
  const token = body?.token || qqSidecarConfig.qqbot.token;
  const sandbox = body?.sandbox ?? qqSidecarConfig.qqbot.sandbox;
  if (!appId || !token) {
    return reply.status(400).send({ error: "appId and token required" });
  }
  console.log(`[HTTP] \u6536\u5230QQBot\u8FDE\u63A5\u8BF7\u6C42 appId=${appId}`);
  try {
    await qq.connect({ appId, token, sandbox });
    saveQQBotConfig({ appId, token, sandbox });
  } catch (err) {
    console.error(`[HTTP] QQBot\u8FDE\u63A5\u5931\u8D25:`, err.message);
    return reply.status(500).send({ error: err.message });
  }
  return reply.send({ success: true });
});
app.post("/api/disconnect", async (_req, reply) => {
  qq.disconnect();
  clearQQBotConfig();
  return reply.send({ success: true });
});
app.post("/api/send", async (req, reply) => {
  if (!qq.isOnline()) {
    return reply.status(503).send({ success: false, error: "QQBot\u672A\u8FDE\u63A5" });
  }
  const body = req.body;
  const toUserId = body?.toUserId;
  const text = body?.text;
  const groupId = body?.groupId;
  if (!toUserId && !groupId) {
    return reply.status(400).send({ success: false, error: "toUserId or groupId required" });
  }
  if (!text) {
    return reply.status(400).send({ success: false, error: "text required" });
  }
  try {
    if (groupId) {
      await qq.sendGroupMsg(groupId, text);
    } else {
      await qq.sendPrivateMsg(toUserId, text);
    }
    console.log(`[HTTP] \u6D88\u606F\u5DF2\u53D1\u9001 to=${toUserId || groupId}`);
    return reply.send({ success: true });
  } catch (err) {
    console.error(`[HTTP] \u53D1\u9001\u5931\u8D25:`, err.message);
    return reply.status(500).send({ success: false, error: err.message });
  }
});
app.post("/api/send-voice", async (req, reply) => {
  if (!qq.isOnline()) {
    return reply.status(503).send({ success: false, error: "QQBot\u672A\u8FDE\u63A5" });
  }
  const body = req.body;
  const toUserId = body?.toUserId;
  const text = body?.text;
  const groupId = body?.groupId;
  if (!toUserId && !groupId) {
    return reply.status(400).send({ success: false, error: "toUserId or groupId required" });
  }
  if (!text) {
    return reply.status(400).send({ success: false, error: "text required" });
  }
  try {
    const parts = text.split(String.fromCharCode(10)).map((p) => p.trim()).filter((p) => p.length > 0).map((p) => p.trim()).filter((p) => p.length > 0);
    for (let i = 0; i < parts.length; i++) {
      const part = parts[i];
      try {
        const ttsResp = await fetch(`${qqSidecarConfig.coreUrl}/api/tts/synthesize`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ text: part }),
          signal: AbortSignal.timeout(6e5)
        });
        const ttsJson = await ttsResp.json();
        const audioUrl = ttsJson?.data?.audioUrl;
        if (!audioUrl) throw new Error("TTS returned no audioUrl");
        const fullAudioUrl = qqSidecarConfig.coreUrl + audioUrl;
        const audioResp = await fetch(fullAudioUrl, { signal: AbortSignal.timeout(3e4) });
        if (!audioResp.ok) throw new Error("audio download failed: " + audioResp.status);
        const audioBuffer = Buffer.from(await audioResp.arrayBuffer());
        const fileInfo = groupId ? await qq.uploadGroupMedia(groupId, audioBuffer, "voice" + i + ".mp3", 3) : await qq.uploadPrivateMedia(toUserId, audioBuffer, "voice" + i + ".mp3", 3);
        if (groupId) {
          await qq.sendGroupVoice(groupId, fileInfo);
        } else {
          await qq.sendPrivateVoice(toUserId, fileInfo);
        }
      } catch (e) {
        console.error(`[HTTP] \u8BED\u97F3\u53D1\u9001\u5931\u8D25 part=${i}:`, e.message);
        try {
          if (groupId) {
            await qq.sendGroupMsg(groupId, part);
          } else {
            await qq.sendPrivateMsg(toUserId, part);
          }
        } catch {
        }
      }
      if (i < parts.length - 1) await new Promise((r) => setTimeout(r, 800));
    }
    return reply.send({ success: true });
  } catch (err) {
    console.error(`[HTTP] \u8BED\u97F3\u53D1\u9001\u5931\u8D25:`, err.message);
    return reply.status(500).send({ success: false, error: err.message });
  }
});
app.get("/api/health", async (_req, reply) => {
  return reply.send({ success: true, qqOnline: qq.isOnline() });
});
app.get("/api/status", async (_req, reply) => {
  return reply.send({
    success: true,
    data: {
      qqOnline: qq.isOnline(),
      status: qq.getStatus(),
      accountId: qq.getAccountId(),
      error: qq.getLastError(),
      messageCount: qq.getMessageCount()
    }
  });
});
try {
  await app.listen({ host: qqSidecarConfig.host, port: qqSidecarConfig.port });
  console.log("");
  console.log("  ========================================");
  console.log("    QQ Sidecar (QQBot WebSocket) v2.3");
  console.log("    HTTP:    http://" + qqSidecarConfig.host + ":" + qqSidecarConfig.port);
  console.log("  ========================================");
  console.log("");
  const savedConfig = loadQQBotConfig();
  if (savedConfig) {
    console.log("[QQ-Sidecar] \u68C0\u6D4B\u5230\u6301\u4E45\u5316\u51ED\u8BC1\uFF0C\u81EA\u52A8\u8FDE\u63A5 QQBot...");
    qq.connect({ appId: savedConfig.appId, token: savedConfig.token, sandbox: savedConfig.sandbox });
  } else if (qqSidecarConfig.qqbot.appId && qqSidecarConfig.qqbot.token) {
    console.log("[QQ-Sidecar] \u4F7F\u7528\u73AF\u5883\u53D8\u91CF\u81EA\u52A8\u8FDE\u63A5 QQBot...");
    const cfg = { appId: qqSidecarConfig.qqbot.appId, token: qqSidecarConfig.qqbot.token, sandbox: qqSidecarConfig.qqbot.sandbox };
    qq.connect(cfg);
    saveQQBotConfig(cfg);
  } else {
    console.log("[QQ-Sidecar] \u672A\u914D\u7F6E\u51ED\u8BC1\uFF0C\u7B49\u5F85HTTP\u8FDE\u63A5\u8BF7\u6C42...");
  }
} catch (err) {
  app.log.error(err);
  process.exit(1);
}

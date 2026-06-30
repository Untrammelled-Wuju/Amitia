// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
// QQBot 官方 WebSocket 客户端
import WebSocket from "ws"
import fs from "node:fs"

const TOKEN_URL = "https://bots.qq.com/app/getAppAccessToken"
const API_BASE = "https://api.sgroup.qq.com"
const FULL_INTENTS = (1 << 30) | (1 << 12) | (1 << 25) | (1 << 26)

export interface QQBotConfig {
  appId: string
  token: string
  sandbox: boolean
}

export interface QQMessage {
  fromUserId: string
  toUserId: string
  messageId: string
  text: string
  groupId?: string
  createdAt: number
  isVoice?: boolean
  imageUrl?: string
  videoUrl?: string
  fileUrl?: string
  voiceUrl?: string
  fileName?: string
}

export type MessageHandler = (msg: QQMessage) => Promise<string | void>

interface GatewayPayload {
  op: number
  d?: any
  s?: number
  t?: string
}

export class QQBotClient {
  private ws: WebSocket | null = null
  private config: QQBotConfig | null = null
  private seq: number = 0
  private sessionId: string = ""
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private handlers: MessageHandler[] = []
  private loginStatus: "disconnected" | "connecting" | "online" = "disconnected"
  private accountId: string = ""
  private botName: string = ""
  private reconnectAttempts: number = 0
  private maxReconnectAttempts: number = 3
  private accessToken: string = ""
  private accessTokenExpiry: number = 0
  private _messageCount: number = 0
  private _manualDisconnect: boolean = false


  private lastErrorMessage: string = ""
  get apiBase(): string {
    if (!this.config) return "https://api.sgroup.qq.com"
    return this.config.sandbox ? "https://sandbox.api.sgroup.qq.com" : "https://api.sgroup.qq.com"
  }
  getLastError(): string { return this.lastErrorMessage }
  getStatus() { return this.loginStatus }
  getMessageCount(): number { return this._messageCount }
  getAccountId() { return this.accountId }
  isOnline() { return this.loginStatus === "online" }

  constructor() {
    console.log("[QQBot] Client initialized")
  }

  private debugLog(msg: string): void {
    const ts = new Date().toISOString()
    const line = `[${ts}] ${msg}`
    console.log(line)
    try {
      fs.appendFileSync("qqbot-debug.log", line + "\n")
    } catch {}
  }

  onMessage(handler: MessageHandler): void {
    this.handlers.push(handler)
  }

  async connect(config: QQBotConfig): Promise<void> {
    this.reconnectAttempts = 0
    this.identifyRetries = 0
    this.lastErrorMessage = ""
    this._manualDisconnect = false
    await this._doConnect(config)
  }

  private async _doConnect(config: QQBotConfig): Promise<void> {
    if (this.ws) {
      this.disconnect()
    }

    this.config = config
    this.loginStatus = "connecting"
    this.accountId = config.appId

    this.debugLog(`正在连接... AppID=${config.appId} sandbox=${config.sandbox}`)

    try {
      const wsUrl = await this.getGatewayUrl()
      this.debugLog(`Gateway URL: ${wsUrl}`)
      this.connectWebSocket(wsUrl)
    } catch (err: any) {
      this.debugLog(`获取Gateway失败: ` + err.message)
      this.lastErrorMessage = `Gateway请求失败: ` + err.message
      this.loginStatus = "disconnected"
      if (!this._manualDisconnect && this.reconnectAttempts < this.maxReconnectAttempts) {
        this.scheduleReconnect()
      }
    }
  }

  private async getAccessToken(): Promise<string> {
    if (this.accessToken && Date.now() < this.accessTokenExpiry - 60000) {
      return this.accessToken
    }
    const resp = await fetch(TOKEN_URL, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ appId: this.config!.appId, clientSecret: this.config!.token }),
      signal: AbortSignal.timeout(15000)
    })
    if (!resp.ok) {
      const text = await resp.text()
      throw new Error(`获取AccessToken失败 (${resp.status}): ${text}`)
    }
    const data = await resp.json() as { access_token: string; expires_in: string }
    this.accessToken = data.access_token
    this.accessTokenExpiry = Date.now() + parseInt(data.expires_in || "3600") * 1000
    this.debugLog(`AccessToken已获取`)
    return this.accessToken
  }

  private async getGatewayUrl(): Promise<string> {
    const at = await this.getAccessToken()
    const resp = await fetch(`${API_BASE}/gateway`, {
      headers: { "Authorization": `QQBot ${at}` },
      signal: AbortSignal.timeout(15000)
    })
    if (!resp.ok) {
      const text = await resp.text()
      throw new Error(`Gateway请求失败 (${resp.status}): ${text}`)
    }
    const data = await resp.json() as { url: string }
    return data.url
  }

  private connectWebSocket(wsUrl: string): void {
    this.ws = new WebSocket(wsUrl)

    this.ws.on("open", () => {
      this.debugLog("WebSocket已连接")
    })

    this.ws.on("message", (data: Buffer) => {
      try {
        const payload: GatewayPayload = JSON.parse(data.toString())
        this.handleGatewayMessage(payload)
      } catch (e) {
        console.error("[QQBot] 消息解析失败:", e)
      }
    })

    this.ws.on("close", (code: number, reason: Buffer) => {
      const reasonStr = reason.toString()
      console.log(`[QQBot] WebSocket断开: code=${code} reason=${reasonStr}`)
      this.stopHeartbeat()
      if (code === 4004 || code === 4009 || code === 4010 || code === 4011 || code === 4012 || code === 4013 || code === 4014) {
        this.debugLog(`鉴权失败 (code=${code})，停止重连`); this.lastErrorMessage = `鉴权失败: WebSocket关闭码=${code}`
        this.loginStatus = "disconnected"
        this.reconnectAttempts = this.maxReconnectAttempts
        return
      }
      if (this.loginStatus !== "disconnected") {
        if (this._manualDisconnect) {
          this.loginStatus = "disconnected"
        } else {
          this.loginStatus = "connecting"
          this.scheduleReconnect()
        }
      }
    })

    this.ws.on("error", (err: Error) => {
      console.error("[QQBot] WebSocket错误:", err.message)
    })
  }

  private handleGatewayMessage(payload: GatewayPayload): void {
    const { op, d, s, t } = payload

    if (s) this.seq = s

    switch (op) {
      case 0: // Dispatch
        this.handleDispatch(t!, d)
        break
      case 10: // Hello
        this.debugLog(`Hello, heartbeat间隔=${d.heartbeat_interval}ms`)
        this.sessionId = ""
        this.startHeartbeat(d.heartbeat_interval)
        this.sendIdentify()
        break
      case 11: // Heartbeat ACK
        break
      case 7: // Reconnect
        console.log("[QQBot] 服务端要求重连")
        this.reconnect()
        break
      case 9: // Invalid Session
        console.log("[QQBot] Session无效, d=" + JSON.stringify(d))
        if (d === false) {
          this.debugLog("鉴权失败，Token可能无效，停止重连"); this.lastErrorMessage = "鉴权失败: AppID或Token无效"
          this.loginStatus = "disconnected"
          this.stopHeartbeat()
          this.reconnectAttempts = this.maxReconnectAttempts
          return
        }
        this.sessionId = ""
        setTimeout(() => this.sendIdentify(), 1000)
        break
      default:
        console.log(`[QQBot] 未知op: ${op}`)
    }
  }

  private identifyRetries: number = 0
  private maxIdentifyRetries: number = 3

  private sendIdentify(): void {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      console.log("[QQBot] WebSocket未连接，跳过鉴权")
      return
    }
    if (this.identifyRetries >= this.maxIdentifyRetries) {
      console.error("[QQBot] 鉴权重试次数已达上限，停止重连")
      this.loginStatus = "disconnected"
      this.reconnectAttempts = this.maxReconnectAttempts
      this.disconnect()
      return
    }
    this.identifyRetries++
    const payload: GatewayPayload = {
      op: 2,
      d: {
        token: `QQBot ${this.accessToken}`,
        intents: FULL_INTENTS,
        shard: [0, 1],
        properties: {}
      }
    }
    console.log(`[QQBot] Identify token: ${payload.d.token.substring(0, 40)}... intents: ${payload.d.intents}`)
    this.ws.send(JSON.stringify(payload))
    this.debugLog(`已发送鉴权 (第${this.identifyRetries}次)`)
  }

  private startHeartbeat(interval: number): void {
    this.stopHeartbeat()
    this.heartbeatTimer = setInterval(() => {
      if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        this.ws.send(JSON.stringify({ op: 1, d: this.seq }))
      }
    }, interval)
  }

  private stopHeartbeat(): void {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer)
      this.heartbeatTimer = null
    }
  }

  private handleDispatch(eventType: string, data: any): void {
    switch (eventType) {
      case "READY":
        this.loginStatus = "online"
        this.accountId = data?.user?.id || this.accountId
        this.botName = data?.user?.username || ""
        this.sessionId = data?.session_id || ""
        this.identifyRetries = 0
        this.reconnectAttempts = 0
        this.debugLog(`已上线! Bot=${this.botName} (${this.accountId})`)
        break

      case "AT_MESSAGE_CREATE":
      case "GROUP_AT_MESSAGE_CREATE":
        this.debugLog(`[QQBot][RAW-DISPATCH] ${eventType} 原始数据:` + JSON.stringify(data).substring(0, 2000))
        this.handleGroupMessage(data)
        break

      case "C2C_MESSAGE_CREATE":
      case "DIRECT_MESSAGE_CREATE":
        this.debugLog(`[QQBot][RAW-DISPATCH] ${eventType} 原始数据:` + JSON.stringify(data).substring(0, 2000))
        this.handleDirectMessage(data)
        break

      default:
        // Quietly ignore other events
        break
    }
  }

  private handleGroupMessage(data: any): void {
    const extracted = this.extractContent(data)
    const text = extracted.text
    const isVoice = extracted.isVoice
    const imageUrl = extracted.imageUrl
    const fileUrl = extracted.fileUrl
    if (!text && !isVoice && !imageUrl && !fileUrl) return

    const msg: QQMessage = {
      fromUserId: data?.author?.id || "",
      toUserId: this.accountId,
      messageId: data?.id || "",
      text: text || (isVoice ? "[语音]" : ""),
      groupId: data?.group_id || data?.guild_id || "",
      createdAt: Date.now(),
      isVoice: isVoice,
      imageUrl: imageUrl || undefined,
      videoUrl: (extracted as any).videoUrl || undefined,
      fileUrl: fileUrl || undefined,
      fileName: extracted.fileName || undefined,
      voiceUrl: extracted.voiceUrl || undefined,
    }

    this._messageCount++
    const preview = msg.text.substring(0, 80)
    console.log(`[QQBot][群:${msg.groupId}] ${msg.fromUserId}: ${preview}${msg.isVoice ? " (语音)" : ""}`)
    this.notifyHandlers(msg)
  }

  private handleDirectMessage(data: any): void {
    const extracted = this.extractContent(data)
    const text = extracted.text
    const isVoice = extracted.isVoice
    const imageUrl = extracted.imageUrl
    const fileUrl = extracted.fileUrl
    if (!text && !isVoice && !imageUrl && !fileUrl) return

    const msg: QQMessage = {
      fromUserId: data?.author?.id || "",
      toUserId: this.accountId,
      messageId: data?.id || "",
      text: text || (isVoice ? "[语音]" : ""),
      createdAt: Date.now(),
      isVoice: isVoice,
      imageUrl: imageUrl || undefined,
      fileUrl: fileUrl || undefined,
      fileName: extracted.fileName || undefined,
      voiceUrl: extracted.voiceUrl || undefined,
    }

    this._messageCount++
    const preview = msg.text.substring(0, 80)
    console.log(`[QQBot][私聊] ${msg.fromUserId}: ${preview}${msg.isVoice ? " (语音)" : ""}`)
    this.notifyHandlers(msg)
  }

  private extractContent(data: any): { text: string; isVoice: boolean; imageUrl: string; videoUrl?: string; fileUrl?: string
  fileName?: string; fileContentType?: string; voiceUrl?: string } {
    const rawDataId = data?.id || "unknown"
    this.debugLog("[QQBot][EXTRACT] msgId=" + rawDataId + " content类型=" + typeof data?.content + " isArray=" + Array.isArray(data?.content) + " attachments=" + (data?.attachments ? JSON.stringify(data.attachments).substring(0, 500) : "无"))
    if (typeof data.content === "object" && !Array.isArray(data.content)) {
      this.debugLog("[QQBot][EXTRACT] msgId=" + rawDataId + " content对象keys=" + Object.keys(data.content).join(",") + " content=" + JSON.stringify(data.content).substring(0, 1000))
    }
    if (Array.isArray(data.content)) {
      this.debugLog("[QQBot][EXTRACT] msgId=" + rawDataId + " content数组长度=" + data.content.length + " types=" + data.content.map((c:any) => c.type || c.msg_type || "?").join(","))
    }
    const hasAttachmentsVideo = data?.attachments?.some((a: any) =>
      a?.content_type?.startsWith("video/") || a?.type === "video" || a?.content_type === "video"
    )
    if (hasAttachmentsVideo) {
      const vidAtt = data.attachments.find((a: any) =>
        a?.content_type?.startsWith("video/") || a?.type === "video" || a?.content_type === "video"
      )
      const vidUrl = (vidAtt?.url || vidAtt?.src_url || vidAtt?.url_src || "")
      const text = typeof data.content === "string" ? data.content.trim() : ""
      this.debugLog("[QQBot][VIDEO-DETECT] msgId=" + rawDataId + " 检测到视频! url=" + vidUrl)
      if (text) return { text, isVoice: false, imageUrl: "", videoUrl: vidUrl }
      return { text: "", isVoice: false, imageUrl: "", videoUrl: vidUrl }
    }
    const hasAttachmentsImage = data?.attachments?.some((a: any) =>
      a?.content_type?.startsWith("image/") || a?.type === "image" || a?.content_type === "image"
    )
    if (hasAttachmentsImage) {
      const imgAtt = data.attachments.find((a: any) =>
        a?.content_type?.startsWith("image/") || a?.type === "image" || a?.content_type === "image"
      )
      const imgUrl = imgAtt?.url || ""
      this.debugLog("[QQBot][IMAGE-DETECT] msgId=" + rawDataId + " 检测到图片! url=" + imgUrl)
      if (typeof data?.content === "string" && data.content.trim()) {
        return { text: data.content.trim(), isVoice: false, imageUrl: imgUrl }
      }
      if (Array.isArray(data?.content)) {
        const iparts = data.content.filter((c: any) => c.type === "text" && c.text).map((c: any) => c.text)
        const iv = iparts.length > 0 ? iparts.join("") : ""
        return { text: iv, isVoice: false, imageUrl: imgUrl }
      }
      return { text: "", isVoice: false, imageUrl: imgUrl }
    }

    if (typeof data?.content === "string") {
      const text = data.content.trim()
      if (text) return { text, isVoice: false, imageUrl: '' }
    }

    if (data?.content && typeof data.content === "object") {
      if (Array.isArray(data.content)) {
        const textParts = data.content
          .filter((s: any) => s.type === "text" && s.text)
          .map((s: any) => s.text)
        const hasVoice = data.content.some((s: any) =>
          s.type === "voice" || s.type === "audio" || s.msg_type === "voice" || s.msg_type === "audio"
        )
        const contentImageUrls = data.content.filter((c: any) => c.type === 'image' || c.msg_type === 'image').map((c: any) => c.url || '').filter(Boolean);
        if (textParts.length > 0) return { text: textParts.join(''), isVoice: hasVoice, imageUrl: contentImageUrls[0] || '' }
        if (hasVoice) { this.debugLog("[QQBot][VOICE-DETECT] msgId=" + rawDataId + " 检测到语音消息! content数组详情:" + JSON.stringify(data.content).substring(0, 2000)); return { text: "[语音]", isVoice: true, imageUrl: contentImageUrls[0] || "" } }
        if (contentImageUrls.length > 0) { this.debugLog("[QQBot][IMAGE-DETECT] msgId=" + rawDataId + " 检测到content数组中的图片! url=" + contentImageUrls[0]); return { text: "", isVoice: false, imageUrl: contentImageUrls[0] } }
        return { text: "", isVoice: false, imageUrl: "" }
      }
      if (typeof data.content.text === "string") {
        const text = data.content.text.trim()
        if (text) return { text, isVoice: false, imageUrl: "" }
      }
    }

    const hasAttachmentsVoice = data?.attachments?.some((a: any) =>
      a?.content_type?.startsWith("audio/") || a?.type === "voice" || a?.content_type === "voice"
    )
    if (hasAttachmentsVoice) {
      const voiceAtt = data.attachments.find((a: any) =>
        a?.content_type?.startsWith("audio/") || a?.type === "voice" || a?.content_type === "voice"
      )
      const asrText = voiceAtt?.asr_refer_text || ""
      this.debugLog("[QQBot][VOICE-DETECT] msgId=" + rawDataId + " 检测到语音消息! attachments详情:" + JSON.stringify(data.attachments).substring(0, 2000))
      this.debugLog("[QQBot][VOICE-ASR] msgId=" + rawDataId + " QQ语音识别文本: " + asrText)
      if (asrText) {
        return { text: asrText, isVoice: true, imageUrl: "", voiceUrl: voiceAtt?.url || "" }
      }
      return { text: "[语音]", isVoice: true, imageUrl: "", voiceUrl: voiceAtt?.url || "" }
    }

    const hasAttachmentsFile = data?.attachments?.some((a: any) =>
      a?.content_type?.startsWith("file/") || a?.type === "file" || a?.content_type === "file"
    )
    if (hasAttachmentsFile) {
      const fileAtt = data.attachments.find((a: any) =>
        a?.content_type?.startsWith("file/") || a?.type === "file" || a?.content_type === "file"
      )
      const fUrl = (fileAtt?.url || "")
      const fName = (fileAtt?.filename || fileAtt?.file_name || "")
      const fContentType = (fileAtt?.content_type || "")
      this.debugLog("[QQBot][FILE-DETECT] msgId=" + rawDataId + " 检测到文件! url=" + fUrl + " name=" + fName + " contentType=" + fContentType)
      if (fUrl) {
        const ftext = typeof data?.content === "string" ? data.content.trim() : ""
        return { text: ftext, isVoice: false, imageUrl: "", fileUrl: fUrl, fileName: fName, fileContentType: fContentType }
      }
    }
    const hasAttachmentsOther = data?.attachments?.some((a: any) =>
      a?.url && !a?.content_type?.startsWith("image/") && !a?.content_type?.startsWith("video/") && !a?.content_type?.startsWith("audio/") && !(a?.type === "voice")
    )
    if (hasAttachmentsOther) {
      const otherAtt = data.attachments.find((a: any) =>
        a?.url && !a?.content_type?.startsWith("image/") && !a?.content_type?.startsWith("video/") && !a?.content_type?.startsWith("audio/") && !(a?.type === "voice")
      )
      const oUrl = (otherAtt?.url || "")
      const oName = (otherAtt?.filename || otherAtt?.file_name || "")
      const oContentType = (otherAtt?.content_type || "")
      this.debugLog("[QQBot][FILE-DETECT] msgId=" + rawDataId + " 检测到通用附件(可能是文件)! url=" + oUrl + " name=" + oName + " contentType=" + oContentType)
      if (oUrl) {
        const otext = typeof data?.content === "string" ? data.content.trim() : ""
        return { text: otext, isVoice: false, imageUrl: "", fileUrl: oUrl, fileName: oName, fileContentType: oContentType }
      }
    }
    return { text: "", isVoice: false, imageUrl: "" }
  }
  private notifyHandlers(msg: QQMessage): void {
    for (const handler of this.handlers) {
      try {
        handler(msg)
      } catch (err: any) {
        console.error("[QQBot] 消息处理失败:", err.message)
      }
    }
  }

  async sendGroupMsg(groupId: string, text: string): Promise<void> {
    if (!this.config) throw new Error("未连接")
    return this._sendWithRetry(async (token) => {
      const url = `${this.apiBase}/v2/groups/${groupId}/messages`
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
      })
      if (!resp.ok) {
        const errText = await resp.text()
        throw new Error(`发送群消息失败 (${resp.status}): ${errText}`)
      }
      console.log(`[QQBot] 发送群消息: ->${groupId} (${text.length} chars)`)
    }, "发送群消息")
  }

  async sendPrivateMsg(userId: string, text: string): Promise<void> {
    if (!this.config) throw new Error("未连接")
    return this._sendWithRetry(async (token) => {
      const url = `${this.apiBase}/v2/users/${userId}/messages`
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
      })
      if (!resp.ok) {
        const errText = await resp.text()
        throw new Error(`发送私聊失败 (${resp.status}): ${errText}`)
      }
      console.log(`[QQBot] 发送私聊: ->${userId} (${text.length} chars)`)
    }, "发送私聊")
  }

  private async _sendWithRetry<T>(sendFn: (token: string) => Promise<T>, label: string): Promise<T> {
    let token = this.accessToken
    if (!token) {
      token = await this.getAccessToken()
    }
    try {
      const result = await sendFn(token)
      return result
    } catch (err: any) {
      const msg = err?.message || String(err)
      if (msg.includes("token not exist or expire") || msg.includes("11244")) {
        console.log(`[QQBot] Token已过期，刷新后重试${label}`)
        this.accessToken = ""
        this.accessTokenExpiry = 0
        token = await this.getAccessToken()
        const retryResult = await sendFn(token)
        console.log(`[QQBot] ${label}重试成功`)
        return retryResult
      } else {
        throw err
      }
    }
  }

  async uploadGroupMedia(groupId: string, fileBuffer: Buffer, fileName: string, fileType: number): Promise<string> {
    if (!this.config) throw new Error("未连接")
    return this._sendWithRetry(async (token: string) => {
      const b64 = fileBuffer.toString("base64")
      this.debugLog("[QQBot][UPLOAD-JSON] groupId=" + groupId + " fileType=" + fileType + " b64Len=" + b64.length)
      const url = this.apiBase + "/v2/groups/" + groupId + "/files"
      const resp = await fetch(url, {
        method: "POST",
        headers: {
          "Authorization": "QQBot " + token,
          "Content-Type": "application/json"
        },
        body: JSON.stringify({ file_type: fileType, file_data: b64, srv_send_msg: false }),
      })
      if (!resp.ok) {
        const errText = await resp.text()
        this.debugLog("[QQBot][UPLOAD-ERR] 上传群文件失败 status=" + resp.status + " body=" + errText.substring(0, 500))
        throw new Error("上传群文件失败 (" + resp.status + "): " + errText)
      }
      const data = await resp.json() as { file_info?: string; file_uuid?: string }
      if (!data.file_info) throw new Error("上传成功但未返回file_info")
      return data.file_info
    }, "上传群媒体")
  }

  async uploadPrivateMedia(userId: string, fileBuffer: Buffer, fileName: string, fileType: number): Promise<string> {
    if (!this.config) throw new Error("未连接")
    return this._sendWithRetry(async (token: string) => {
      const b64 = fileBuffer.toString("base64")
      this.debugLog("[QQBot][UPLOAD-JSON] userId=" + userId + " fileType=" + fileType + " b64Len=" + b64.length)
      const url = this.apiBase + "/v2/users/" + userId + "/files"
      const resp = await fetch(url, {
        method: "POST",
        headers: {
          "Authorization": "QQBot " + token,
          "Content-Type": "application/json"
        },
        body: JSON.stringify({ file_type: fileType, file_data: b64, srv_send_msg: false }),
      })
      if (!resp.ok) {
        const errText = await resp.text()
        this.debugLog("[QQBot][UPLOAD-ERR] 上传私聊文件失败 status=" + resp.status + " body=" + errText.substring(0, 500))
        throw new Error("上传私聊文件失败 (" + resp.status + "): " + errText)
      }
      const data = await resp.json() as { file_info?: string; file_uuid?: string }
      if (!data.file_info) throw new Error("上传成功但未返回file_info")
      return data.file_info
    }, "上传私聊媒体")
  }


  async sendGroupVoice(groupId: string, fileInfo: string): Promise<void> {
    if (!this.config) throw new Error("未连接")
    return this._sendWithRetry(async (token: string) => {
      const url = this.apiBase + "/v2/groups/" + groupId + "/messages"
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
          msg_seq: Math.floor(Date.now() / 1000),
        })
      })
      if (!resp.ok) {
        const errText = await resp.text()
        throw new Error("发送群语音失败 (" + resp.status + "): " + errText)
      }
      console.log("[QQBot] 发送群语音: ->" + groupId)
    }, "发送群语音")
  }

  async sendPrivateVoice(userId: string, fileInfo: string): Promise<void> {
    if (!this.config) throw new Error("未连接")
    return this._sendWithRetry(async (token: string) => {
      const url = this.apiBase + "/v2/users/" + userId + "/messages"
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
          msg_seq: Math.floor(Date.now() / 1000),
        })
      })
      if (!resp.ok) {
        const errText = await resp.text()
        throw new Error("发送私聊语音失败 (" + resp.status + "): " + errText)
      }
      console.log("[QQBot] 发送私聊语音: ->" + userId)
    }, "发送私聊语音")
  }



  async downloadImage(url: string): Promise<{ buffer: Buffer; contentType: string } | null> {
    if (!this.config) return null
    try {
      const token = await this.getAccessToken()
      const resp = await fetch(url, {
        headers: { "Authorization": "QQBot " + token },
        signal: AbortSignal.timeout(30000)
      })
      if (!resp.ok) {
        this.debugLog("[QQBot][IMAGE-DL] 下载图片失败 status=" + resp.status)
        return null
      }
      const arrayBuffer = await resp.arrayBuffer()
      const buffer = Buffer.from(arrayBuffer)
      const contentType = resp.headers.get("content-type") || "image/png"
      this.debugLog("[QQBot][IMAGE-DL] 图片下载成功 size=" + buffer.length + " type=" + contentType)
      return { buffer, contentType }
    } catch (err: any) {
      this.debugLog("[QQBot][IMAGE-DL] 下载图片异常: " + err.message)
      return null
    }
  }

  disconnect(): void {


    console.log("[QQBot] 断开连接")
    this._manualDisconnect = true
    this.loginStatus = "disconnected"
    this.stopHeartbeat()
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
    if (this.ws) {
      this.ws.removeAllListeners()
      this.ws.close()
      this.ws = null
    }
  }

  private reconnect(): void {
    if (this.ws) {
      this.ws.removeAllListeners()
      this.ws.close()
      this.ws = null
    }
    this.stopHeartbeat()
    if (this.config) {
      this._doConnect(this.config)
    }
  }

  private scheduleReconnect(): void {
    if (this._manualDisconnect) {
      this.debugLog("手动断开，跳过自动重连")
      return
    }
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      this.debugLog("重连次数已达上限，停止重连"); this.lastErrorMessage = "重连次数已达上限，请检查网络和凭证"
      this.loginStatus = "disconnected"
      return
    }

    this.reconnectAttempts++
    const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30000)
    this.debugLog(`${delay}ms后第${this.reconnectAttempts}次重连`)

    this.reconnectTimer = setTimeout(() => {
      if (this.config) {
        this._doConnect(this.config)
      }
    }, delay)
  }
}

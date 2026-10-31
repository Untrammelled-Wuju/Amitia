// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import WebSocket from "ws"
import fs from "node:fs"
import path from "node:path"

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

function isTokenExpiredError(errMsg: string): boolean {
  return /token not exist or expire/i.test(errMsg) || /11244/.test(errMsg)
}

export class QQBotClient {
  private ws: WebSocket | null = null
  private config: QQBotConfig | null = null
  private seq = 0
  private sessionId = ""
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private handlers: MessageHandler[] = []
  private loginStatus: "disconnected" | "connecting" | "online" = "disconnected"
  private accountId = ""
  private botName = ""
  private reconnectAttempts = 0
  private maxReconnectAttempts = 3
  private accessToken = ""
  private accessTokenExpiry = 0
  private _messageCount = 0
  private _replyCount = 0
  private _manualDisconnect = false
  private _startedAt = ""
  private lastErrorMessage = ""
  private identifyRetries = 0
  private maxIdentifyRetries = 3

  get apiBase(): string {
    if (!this.config) return API_BASE
    return this.config.sandbox ? "https://sandbox.api.sgroup.qq.com" : API_BASE
  }

  getLastError(): string {
    return this.lastErrorMessage
  }

  getStatus() {
    return this.loginStatus
  }

  getMessageCount(): number {
    return this._messageCount
  }

  getReplyCount(): number {
    return this._replyCount
  }

  getAccountId() {
    return this.accountId
  }

  getStartedAt() {
    return this._startedAt
  }

  isOnline() {
    return this.loginStatus === "online"
  }

  private resolveStatsPath(): string {
    const dir = path.join(process.cwd(), "qq-stats")
    if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true })
    return path.join(dir, `${this.accountId}.stats.json`)
  }

  private persistStats(): void {
    if (!this.accountId) return
    try {
      fs.writeFileSync(this.resolveStatsPath(), JSON.stringify({
        messageCount: this._messageCount,
        replyCount: this._replyCount,
      }), "utf-8")
    } catch (error) {
  console.error("[original-error]", error);
}
  }

  private loadStats(): void {
    if (!this.accountId) return
    try {
      const statsPath = this.resolveStatsPath()
      if (!fs.existsSync(statsPath)) return
      const data = JSON.parse(fs.readFileSync(statsPath, "utf-8"))
      if (typeof data.messageCount === "number") this._messageCount = data.messageCount
      if (typeof data.replyCount === "number") this._replyCount = data.replyCount
    } catch (error) {
  console.error("[original-error]", error);
}
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
    if (this.ws) this.disconnect()
    this.config = config
    this.loginStatus = "connecting"
    this.accountId = config.appId
    try {
      const wsUrl = await this.getGatewayUrl()
      this.connectWebSocket(wsUrl)
    } catch (err: any) {
      this.lastErrorMessage = "Gateway请求失败: " + err.message
      this.loginStatus = "disconnected"
      if (isTokenExpiredError(err.message)) {
        this.accessToken = ""
        this.accessTokenExpiry = 0
        if (this.reconnectAttempts < this.maxReconnectAttempts) {
          this.lastErrorMessage = `Token已过期，正在重连 (${this.reconnectAttempts + 1}/${this.maxReconnectAttempts})`
          if (!this._manualDisconnect) this.scheduleReconnect()
        } else {
          this.lastErrorMessage = `Token已过期，重连次数已用完(${this.maxReconnectAttempts}次)，请检查AppID和Token后手动重连`
        }
      } else if (!this._manualDisconnect && this.reconnectAttempts < this.maxReconnectAttempts) {
        this.scheduleReconnect()
      }
    }
  }

  private async getAccessToken(): Promise<string> {
    if (this.accessToken && Date.now() < this.accessTokenExpiry - 60000) return this.accessToken
    const resp = await fetch(TOKEN_URL, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ appId: this.config!.appId, clientSecret: this.config!.token }),
      signal: AbortSignal.timeout(15000),
    })
    if (!resp.ok) {
      const text = await resp.text()
      throw new Error(`获取AccessToken失败 (${resp.status}): ${text}`)
    }
    const data = await resp.json() as { access_token: string; expires_in: string }
    this.accessToken = data.access_token
    this.accessTokenExpiry = Date.now() + parseInt(data.expires_in || "3600") * 1000
    return this.accessToken
  }

  private async getGatewayUrl(): Promise<string> {
    const at = await this.getAccessToken()
    const resp = await fetch(`${API_BASE}/gateway`, {
      headers: { Authorization: `QQBot ${at}` },
      signal: AbortSignal.timeout(15000),
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
    this.ws.on("message", (data: Buffer) => {
      try {
        this.handleGatewayMessage(JSON.parse(data.toString()) as GatewayPayload)
      } catch (error) {
  console.error("[original-error]", error);
}
    })
    this.ws.on("close", (code: number, reason: Buffer) => {
      const reasonStr = reason.toString()
      this.stopHeartbeat()
      if ([4004, 4009, 4010, 4011, 4012, 4013, 4014].includes(code)) {
        this.lastErrorMessage = `鉴权失败: WebSocket关闭码=${code}`
        this.loginStatus = "disconnected"
        this.accessToken = ""
        this.accessTokenExpiry = 0
        if (this.reconnectAttempts >= this.maxReconnectAttempts) {
          this.lastErrorMessage = `QQ Bot鉴权失败(WebSocket关闭码=${code})，重连次数已用完，请检查AppID和Token后手动重连`
        } else {
          this.lastErrorMessage = `鉴权失败: WebSocket关闭码=${code}，正在重连 (${this.reconnectAttempts + 1}/${this.maxReconnectAttempts})`
          this.scheduleReconnect()
        }
        return
      }
      if (this.loginStatus === "disconnected") return
      if (this._manualDisconnect) {
        this.loginStatus = "disconnected"
      } else {
        this.loginStatus = "connecting"
        this.scheduleReconnect()
      }
      if (reasonStr) this.lastErrorMessage = reasonStr
    })
    this.ws.on("error", (err: Error) => {
      this.lastErrorMessage = err.message
    })
  }

  private handleGatewayMessage(payload: GatewayPayload): void {
    const { op, d, s, t } = payload
    if (s) this.seq = s
    switch (op) {
      case 0:
        this.handleDispatch(t!, d)
        break
      case 10:
        this.sessionId = ""
        this.startHeartbeat(d.heartbeat_interval)
        this.sendIdentify()
        break
      case 7:
        this.reconnect()
        break
      case 9:
        if (d === false) {
          this.lastErrorMessage = "鉴权失败: AppID或Token无效"
          this.loginStatus = "disconnected"
          this.stopHeartbeat()
          this.reconnectAttempts = this.maxReconnectAttempts
          return
        }
        this.sessionId = ""
        setTimeout(() => this.sendIdentify(), 1000)
        break
      default:
        break
    }
  }

  private sendIdentify(): void {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return
    if (this.identifyRetries >= this.maxIdentifyRetries) {
      this.loginStatus = "disconnected"
      this.reconnectAttempts = this.maxReconnectAttempts
      this.disconnect()
      return
    }
    this.identifyRetries++
    this.ws.send(JSON.stringify({
      op: 2,
      d: {
        token: `QQBot ${this.accessToken}`,
        intents: FULL_INTENTS,
        shard: [0, 1],
        properties: {},
      },
    }))
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
    if (!this.heartbeatTimer) return
    clearInterval(this.heartbeatTimer)
    this.heartbeatTimer = null
  }

  private handleDispatch(eventType: string, data: any): void {
    if (eventType === "READY") {
      this.loginStatus = "online"
      this.accountId = data?.user?.id || this.accountId
      this.botName = data?.user?.username || ""
      this.sessionId = data?.session_id || ""
      this.identifyRetries = 0
      this.reconnectAttempts = 0
      this._startedAt = new Date().toISOString()
      this.loadStats()
      return
    }
    if (["AT_MESSAGE_CREATE", "GROUP_AT_MESSAGE_CREATE"].includes(eventType)) {
      this.handleGroupMessage(data)
      return
    }
    if (["C2C_MESSAGE_CREATE", "DIRECT_MESSAGE_CREATE"].includes(eventType)) {
      this.handleDirectMessage(data)
    }
  }

  private handleGroupMessage(data: any): void {
    const extracted = this.extractContent(data)
    if (!extracted.text && !extracted.isVoice && !extracted.imageUrl && !extracted.fileUrl && !extracted.videoUrl) return
    const msg: QQMessage = {
      fromUserId: data?.author?.id || "",
      toUserId: this.accountId,
      messageId: data?.id || "",
      text: extracted.text || (extracted.isVoice ? "[语音]" : ""),
      groupId: data?.group_id || data?.guild_id || "",
      createdAt: Date.now(),
      isVoice: extracted.isVoice,
      imageUrl: extracted.imageUrl || undefined,
      videoUrl: extracted.videoUrl || undefined,
      fileUrl: extracted.fileUrl || undefined,
      fileName: extracted.fileName || undefined,
      voiceUrl: extracted.voiceUrl || undefined,
    }
    this._messageCount++
    this.persistStats()
    this.notifyHandlers(msg)
  }

  private handleDirectMessage(data: any): void {
    const extracted = this.extractContent(data)
    if (!extracted.text && !extracted.isVoice && !extracted.imageUrl && !extracted.fileUrl && !extracted.videoUrl) return
    const msg: QQMessage = {
      fromUserId: data?.author?.id || "",
      toUserId: this.accountId,
      messageId: data?.id || "",
      text: extracted.text || (extracted.isVoice ? "[语音]" : ""),
      createdAt: Date.now(),
      isVoice: extracted.isVoice,
      imageUrl: extracted.imageUrl || undefined,
      videoUrl: extracted.videoUrl || undefined,
      fileUrl: extracted.fileUrl || undefined,
      fileName: extracted.fileName || undefined,
      voiceUrl: extracted.voiceUrl || undefined,
    }
    this._messageCount++
    this.persistStats()
    this.notifyHandlers(msg)
  }

  private extractContent(data: any): { text: string; isVoice: boolean; imageUrl: string; videoUrl?: string; fileUrl?: string; fileName?: string; fileContentType?: string; voiceUrl?: string } {
    const hasAttachmentsVideo = data?.attachments?.some((a: any) =>
      a?.content_type?.startsWith("video/") || a?.type === "video" || a?.content_type === "video"
    )
    if (hasAttachmentsVideo) {
      const vidAtt = data.attachments.find((a: any) =>
        a?.content_type?.startsWith("video/") || a?.type === "video" || a?.content_type === "video"
      )
      return {
        text: typeof data.content === "string" ? data.content.trim() : "",
        isVoice: false,
        imageUrl: "",
        videoUrl: vidAtt?.url || vidAtt?.src_url || vidAtt?.url_src || "",
      }
    }
    const hasAttachmentsImage = data?.attachments?.some((a: any) =>
      a?.content_type?.startsWith("image/") || a?.type === "image" || a?.content_type === "image"
    )
    if (hasAttachmentsImage) {
      const imgAtt = data.attachments.find((a: any) =>
        a?.content_type?.startsWith("image/") || a?.type === "image" || a?.content_type === "image"
      )
      const imgUrl = imgAtt?.url || ""
      if (typeof data?.content === "string" && data.content.trim()) {
        return { text: data.content.trim(), isVoice: false, imageUrl: imgUrl }
      }
      if (Array.isArray(data?.content)) {
        const text = data.content.filter((c: any) => c.type === "text" && c.text).map((c: any) => c.text).join("")
        return { text, isVoice: false, imageUrl: imgUrl }
      }
      return { text: "", isVoice: false, imageUrl: imgUrl }
    }
    if (typeof data?.content === "string") {
      return { text: data.content.trim(), isVoice: false, imageUrl: "" }
    }
    if (Array.isArray(data?.content)) {
      const textParts = data.content.filter((s: any) => s.type === "text" && s.text).map((s: any) => s.text)
      const hasVoice = data.content.some((s: any) =>
        s.type === "voice" || s.type === "audio" || s.msg_type === "voice" || s.msg_type === "audio"
      )
      const contentImageUrls = data.content
        .filter((c: any) => c.type === "image" || c.msg_type === "image")
        .map((c: any) => c.url || "")
        .filter(Boolean)
      if (textParts.length > 0) return { text: textParts.join(""), isVoice: hasVoice, imageUrl: contentImageUrls[0] || "" }
      if (hasVoice) return { text: "[语音]", isVoice: true, imageUrl: contentImageUrls[0] || "" }
      if (contentImageUrls.length > 0) return { text: "", isVoice: false, imageUrl: contentImageUrls[0] }
      return { text: "", isVoice: false, imageUrl: "" }
    }
    if (typeof data?.content?.text === "string") {
      return { text: data.content.text.trim(), isVoice: false, imageUrl: "" }
    }
    const hasAttachmentsVoice = data?.attachments?.some((a: any) =>
      a?.content_type?.startsWith("audio/") || a?.type === "voice" || a?.content_type === "voice"
    )
    if (hasAttachmentsVoice) {
      const voiceAtt = data.attachments.find((a: any) =>
        a?.content_type?.startsWith("audio/") || a?.type === "voice" || a?.content_type === "voice"
      )
      const asrText = voiceAtt?.asr_refer_text || ""
      return { text: asrText || "[语音]", isVoice: true, imageUrl: "", voiceUrl: voiceAtt?.url || "" }
    }
    const hasAttachmentsFile = data?.attachments?.some((a: any) =>
      a?.content_type?.startsWith("file/") || a?.type === "file" || a?.content_type === "file"
    )
    if (hasAttachmentsFile) {
      const fileAtt = data.attachments.find((a: any) =>
        a?.content_type?.startsWith("file/") || a?.type === "file" || a?.content_type === "file"
      )
      return {
        text: typeof data?.content === "string" ? data.content.trim() : "",
        isVoice: false,
        imageUrl: "",
        fileUrl: fileAtt?.url || "",
        fileName: fileAtt?.filename || fileAtt?.file_name || "",
        fileContentType: fileAtt?.content_type || "",
      }
    }
    const otherAtt = data?.attachments?.find((a: any) =>
      a?.url && !a?.content_type?.startsWith("image/") && !a?.content_type?.startsWith("video/") && !a?.content_type?.startsWith("audio/") && !(a?.type === "voice")
    )
    if (otherAtt?.url) {
      return {
        text: typeof data?.content === "string" ? data.content.trim() : "",
        isVoice: false,
        imageUrl: "",
        fileUrl: otherAtt.url,
        fileName: otherAtt.filename || otherAtt.file_name || "",
        fileContentType: otherAtt.content_type || "",
      }
    }
    return { text: "", isVoice: false, imageUrl: "" }
  }

  private notifyHandlers(msg: QQMessage): void {
    for (const handler of this.handlers) {
      try {
        void handler(msg)
      } catch (error) {
  console.error("[original-error]", error);
}
    }
  }

  async sendGroupMsg(groupId: string, text: string): Promise<void> {
    if (!this.config) throw new Error("未连接")
    await this._sendWithRetry(async (token) => {
      const resp = await fetch(`${this.apiBase}/v2/groups/${groupId}/messages`, {
        method: "POST",
        headers: { Authorization: `QQBot ${token}`, "Content-Type": "application/json" },
        body: JSON.stringify({ content: text, msg_type: 0 }),
      })
      if (!resp.ok) throw new Error(`发送群消息失败 (${resp.status}): ${await resp.text()}`)
      this._replyCount++
      this.persistStats()
    })
  }

  async sendPrivateMsg(userId: string, text: string): Promise<void> {
    if (!this.config) throw new Error("未连接")
    await this._sendWithRetry(async (token) => {
      const resp = await fetch(`${this.apiBase}/v2/users/${userId}/messages`, {
        method: "POST",
        headers: { Authorization: `QQBot ${token}`, "Content-Type": "application/json" },
        body: JSON.stringify({ content: text, msg_type: 0 }),
      })
      if (!resp.ok) throw new Error(`发送私聊失败 (${resp.status}): ${await resp.text()}`)
      this._replyCount++
      this.persistStats()
    })
  }

  private async _sendWithRetry<T>(sendFn: (token: string) => Promise<T>): Promise<T> {
    let token = this.accessToken || await this.getAccessToken()
    try {
      return await sendFn(token)
    } catch (err: any) {
      const message = err?.message || String(err)
      if (!message.includes("token not exist or expire") && !message.includes("11244")) throw err
      this.accessToken = ""
      this.accessTokenExpiry = 0
      token = await this.getAccessToken()
      return sendFn(token)
    }
  }

  async uploadGroupMedia(groupId: string, fileBuffer: Buffer, _fileName: string, fileType: number): Promise<string> {
    if (!this.config) throw new Error("未连接")
    return this._sendWithRetry(async (token: string) => {
      const resp = await fetch(this.apiBase + "/v2/groups/" + groupId + "/files", {
        method: "POST",
        headers: { Authorization: "QQBot " + token, "Content-Type": "application/json" },
        body: JSON.stringify({ file_type: fileType, file_data: fileBuffer.toString("base64"), srv_send_msg: false }),
      })
      if (!resp.ok) throw new Error("上传群文件失败 (" + resp.status + "): " + await resp.text())
      const data = await resp.json() as { file_info?: string }
      if (!data.file_info) throw new Error("上传成功但未返回file_info")
      return data.file_info
    })
  }

  async uploadPrivateMedia(userId: string, fileBuffer: Buffer, _fileName: string, fileType: number): Promise<string> {
    if (!this.config) throw new Error("未连接")
    return this._sendWithRetry(async (token: string) => {
      const resp = await fetch(this.apiBase + "/v2/users/" + userId + "/files", {
        method: "POST",
        headers: { Authorization: "QQBot " + token, "Content-Type": "application/json" },
        body: JSON.stringify({ file_type: fileType, file_data: fileBuffer.toString("base64"), srv_send_msg: false }),
      })
      if (!resp.ok) throw new Error("上传私聊文件失败 (" + resp.status + "): " + await resp.text())
      const data = await resp.json() as { file_info?: string }
      if (!data.file_info) throw new Error("上传成功但未返回file_info")
      return data.file_info
    })
  }

  async sendGroupVoice(groupId: string, fileInfo: string): Promise<void> {
    if (!this.config) throw new Error("未连接")
    await this._sendWithRetry(async (token: string) => {
      const resp = await fetch(this.apiBase + "/v2/groups/" + groupId + "/messages", {
        method: "POST",
        headers: { Authorization: "QQBot " + token, "Content-Type": "application/json" },
        body: JSON.stringify({ msg_type: 7, media: { file_info: fileInfo }, msg_id: "", msg_seq: Math.floor(Date.now() / 1000) }),
      })
      if (!resp.ok) throw new Error("发送群语音失败 (" + resp.status + "): " + await resp.text())
      this._replyCount++
      this.persistStats()
    })
  }

  async sendPrivateVoice(userId: string, fileInfo: string): Promise<void> {
    if (!this.config) throw new Error("未连接")
    await this._sendWithRetry(async (token: string) => {
      const resp = await fetch(this.apiBase + "/v2/users/" + userId + "/messages", {
        method: "POST",
        headers: { Authorization: "QQBot " + token, "Content-Type": "application/json" },
        body: JSON.stringify({ msg_type: 7, media: { file_info: fileInfo }, msg_id: "", msg_seq: Math.floor(Date.now() / 1000) }),
      })
      if (!resp.ok) throw new Error("发送私聊语音失败 (" + resp.status + "): " + await resp.text())
      this._replyCount++
      this.persistStats()
    })
  }

  async sendGroupImage(groupId: string, fileInfo: string): Promise<void> {
    if (!this.config) throw new Error("未连接")
    await this._sendWithRetry(async (token: string) => {
      const resp = await fetch(this.apiBase + "/v2/groups/" + groupId + "/messages", {
        method: "POST",
        headers: { Authorization: "QQBot " + token, "Content-Type": "application/json" },
        body: JSON.stringify({ msg_type: 7, media: { file_info: fileInfo }, msg_id: "", msg_seq: Math.floor(Date.now() / 1000) }),
      })
      if (!resp.ok) throw new Error("发送群图片失败 (" + resp.status + "): " + await resp.text())
      this._replyCount++
      this.persistStats()
    })
  }

  async sendPrivateImage(userId: string, fileInfo: string): Promise<void> {
    if (!this.config) throw new Error("未连接")
    await this._sendWithRetry(async (token: string) => {
      const resp = await fetch(this.apiBase + "/v2/users/" + userId + "/messages", {
        method: "POST",
        headers: { Authorization: "QQBot " + token, "Content-Type": "application/json" },
        body: JSON.stringify({ msg_type: 7, media: { file_info: fileInfo }, msg_id: "", msg_seq: Math.floor(Date.now() / 1000) }),
      })
      if (!resp.ok) throw new Error("发送私聊图片失败 (" + resp.status + "): " + await resp.text())
      this._replyCount++
      this.persistStats()
    })
  }

  async downloadImage(url: string): Promise<{ buffer: Buffer; contentType: string } | null> {
    if (!this.config) return null
    try {
      const token = await this.getAccessToken()
      const resp = await fetch(url, {
        headers: { Authorization: "QQBot " + token },
        signal: AbortSignal.timeout(30000),
      })
      if (!resp.ok) return null
      const buffer = Buffer.from(await resp.arrayBuffer())
      const contentType = resp.headers.get("content-type") || "image/png"
      return { buffer, contentType }
    } catch (error) {
  console.error("[original-error]", error);
return null
    }
  }

  disconnect(): void {
    this._manualDisconnect = true
    this._startedAt = ""
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
    if (this.config) void this._doConnect(this.config)
  }

  private scheduleReconnect(): void {
    if (this._manualDisconnect) return
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      this.lastErrorMessage = "重连次数已达上限，请检查网络和凭证"
      this.loginStatus = "disconnected"
      return
    }
    this.reconnectAttempts++
    const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30000)
    this.reconnectTimer = setTimeout(() => {
      if (this.config) void this._doConnect(this.config)
    }, delay)
  }
}

// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
// @ts-nocheck
// ============================================================
// OpenClaw WeChat Integration Layer
// Wraps @tencent-weixin/openclaw-weixin functions
// ============================================================

import {
  startWeixinLoginWithQr,
  waitForWeixinLogin,
  DEFAULT_ILINK_BOT_TYPE,
} from "@tencent-weixin/openclaw-weixin/dist/src/auth/login-qr.js"
import {
  apiPostFetch,
  buildBaseInfo,
  getUpdates,
  sendMessage,
  getUploadUrl,
  notifyStart,
  notifyStop,
} from "@tencent-weixin/openclaw-weixin/dist/src/api/api.js"
import {
  saveWeixinAccount,
  loadWeixinAccount,
  listIndexedWeixinAccountIds,
  registerWeixinAccountId,
  unregisterWeixinAccountId,
  clearWeixinAccount,
  DEFAULT_BASE_URL,
} from "@tencent-weixin/openclaw-weixin/dist/src/auth/accounts.js"
import type { WeixinMessage } from "@tencent-weixin/openclaw-weixin/dist/src/api/types.js"
import { normalizeAccountId } from "openclaw/plugin-sdk/account-id"
import { downloadAndDecryptBuffer } from "@tencent-weixin/openclaw-weixin/dist/src/cdn/pic-decrypt.js"
import { silkToWav } from "@tencent-weixin/openclaw-weixin/dist/src/media/silk-transcode.js"
import { resolveStateDir } from "@tencent-weixin/openclaw-weixin/dist/src/storage/state-dir.js"
import crypto from "node:crypto"
import fs from "node:fs"
import path from "node:path"

import QRCode from "qrcode"
import { DeliveryIdempotencyStore } from "./delivery-idempotency.js"
import type { DeliveryExecutionResult } from "./delivery-idempotency.js"

// ============================================================
// Types
// ============================================================

export type LoginStatus =
  | "idle"
  | "qr_ready"
  | "waiting_scan"
  | "scanned"
  | "confirmed"
  | "connected"
  | "error"

export interface WechatState {
  status: LoginStatus
  accountId: string | null
  qrCodeUrl: string
  sessionKey: string
  message: string
  messageCount: number
  replyCount: number
  baseUrl: string
  startedAt: string | null
  lastError: string | null
}

export type MessageHandler = (msg: {
  fromUserId: string
  toUserId: string
  messageId: string
  text: string
  contextToken?: string
  isVoice?: boolean
  audioBase64?: string; imageBase64?: string
  imageUrl?: string
  aeskey?: string
  createdAt: number
}) => Promise<string | string[] | void>

// ============================================================
// State Manager
// ============================================================

export class OpenClawWechatManager {

  private debugLog(...args: unknown[]): void {
    if (process.env.NODE_ENV === "development" || process.env.DEBUG) {
      console.debug(...args)
    }
  }

  private hashId(id: string): string {
    return crypto.createHash("sha256").update(id).digest("hex").slice(0, 8)
  }

  private state: WechatState = {
    status: "idle",
    accountId: null,
    qrCodeUrl: "",
    sessionKey: "",
    message: "",
    messageCount: 0,
    replyCount: 0,
    baseUrl: DEFAULT_BASE_URL,
    startedAt: null,
    lastError: null,
  }

  private polling = false
  private pollGeneration = 0
  private pollPromise: Promise<void> | null = null
  private pollAbort: AbortController | null = null
  private handlers: MessageHandler[] = []
  private sessionStaleCount = 0
  private sessionWarned = false

  private _lastFromUserId = ""

  private getUpdatesBuf = ""
  private token: string | null = null

  private inboundSeen: Map<string, number> = new Map()

  private deliveryStore: DeliveryIdempotencyStore | null = null

  private getDeliveryStore(): DeliveryIdempotencyStore {
    if (!this.deliveryStore) {
      const dir = path.join(resolveStateDir(), "openclaw-weixin", "accounts")
      const accountId = this.state.accountId || "default"
      const filePath = path.join(dir, `${accountId}.delivery-idempotency.json`)
      this.deliveryStore = new DeliveryIdempotencyStore(filePath)
    }
    return this.deliveryStore
  }

  private resetDeliveryStore(): void {
    this.deliveryStore = null
  }

  getState(): WechatState {
    return { ...this.state }
  }

  onMessage(handler: MessageHandler): void {
    this.handlers.push(handler)
  }

  loadSavedAccount(): boolean {
    const ids = listIndexedWeixinAccountIds()
    if (ids.length === 0) return false

    const id = ids[ids.length - 1]
    const data = loadWeixinAccount(id)
    if (data?.token) {
      this.state.accountId = id
      this.state.status = "connected"
      this.state.baseUrl = data.baseUrl || DEFAULT_BASE_URL
      this.state.startedAt = new Date().toISOString()
      this.token = data.token
      if (data.getUpdatesBuf) {
        this.getUpdatesBuf = data.getUpdatesBuf
      }
      if (typeof (data as any).messageCount === "number") {
        this.state.messageCount = (data as any).messageCount
      }
      this.loadStats()
      this.debugLog("[OpenClaw] Loaded saved account: ")
      return true
    }
    return false
  }

  private resolveStatsPath(): string {
    const dir = path.join(resolveStateDir(), "openclaw-weixin", "accounts")
    if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true })
    return path.join(dir, `${this.state.accountId}.stats.json`)
  }

  private persistStats(): void {
    if (!this.state.accountId) return
    try {
      fs.writeFileSync(this.resolveStatsPath(), JSON.stringify({
        messageCount: this.state.messageCount,
        replyCount: this.state.replyCount,
      }), "utf-8")
    } catch {
    }
  }

  private loadStats(): void {
    if (!this.state.accountId) return
    try {
      const statsPath = this.resolveStatsPath()
      if (fs.existsSync(statsPath)) {
        const data = JSON.parse(fs.readFileSync(statsPath, "utf-8"))
        if (typeof data.messageCount === "number") this.state.messageCount = data.messageCount
        if (typeof data.replyCount === "number") this.state.replyCount = data.replyCount
      }
    } catch {
    }
  }

  async startLogin(opts?: { force?: boolean }): Promise<{ qrCodeUrl: string; qrImageUrl: string; sessionKey: string }> {
    try {
      this.state.status = "qr_ready"
      this.state.lastError = null

      const result = await startWeixinLoginWithQr({
        apiBaseUrl: DEFAULT_BASE_URL,
        botType: DEFAULT_ILINK_BOT_TYPE,
        force: opts?.force ?? false,
        verbose: true,
      })

      this.state.qrCodeUrl = result.qrcodeUrl
      this.state.sessionKey = result.sessionKey
      this.state.message = result.message

      const qrImageUrl = await QRCode.toDataURL(result.qrcodeUrl, { width: 300, margin: 2 })

      this.debugLog("[OpenClaw] QR code ready, sessionKey hash: ")

      return {
        qrCodeUrl: result.qrcodeUrl,
        qrImageUrl,
        sessionKey: result.sessionKey,
      }
    } catch (err: any) {
      this.state.status = "error"
      this.state.lastError = err.message
      console.error(`[OpenClaw] startLogin error:`, err.message)
      throw err
    }
  }

  async waitForScan(timeoutMs = 120000): Promise<{
    connected: boolean
    message: string
    accountId?: string
  }> {
    try {
      this.state.status = "waiting_scan"

      const result = await waitForWeixinLogin({
        sessionKey: this.state.sessionKey,
        apiBaseUrl: DEFAULT_BASE_URL,
        timeoutMs,
      })

      if (result.connected && result.botToken && result.accountId) {
        const normalizedId = normalizeAccountId(result.accountId)
        saveWeixinAccount(normalizedId, {
          token: result.botToken,
          baseUrl: result.baseUrl,
          userId: result.userId,
        })
        registerWeixinAccountId(normalizedId)

        this.state.status = "connected"
        this.state.accountId = normalizedId
        this.state.baseUrl = result.baseUrl || DEFAULT_BASE_URL
        this.state.startedAt = new Date().toISOString()
        this.token = result.botToken
        this.resetDeliveryStore()

        this.debugLog("[OpenClaw] Login confirmed! accountId: ")

        return {
          connected: true,
          message: "已将 OpenClaw 连接到微信",
          accountId: normalizedId,
        }
      }

      if (result.alreadyConnected) {
        this.state.status = "connected"
        return { connected: true, message: result.message }
      }

      this.state.status = "idle"
      this.state.lastError = null
      this.state.message = result.message
      return { connected: false, message: result.message }
    } catch (err: any) {
      this.state.status = "error"
      this.state.lastError = err.message
      console.error(`[OpenClaw] waitForScan error:`, err.message)
      return { connected: false, message: err.message }
    }
  }

  async startPolling(): Promise<void> {
    if (this.polling && this.pollPromise) return
    if (!this.token || !this.state.accountId) {
      console.warn("[OpenClaw] Cannot start polling: no credentials")
      return
    }

    this.polling = true
    this.pollGeneration++
    const generation = this.pollGeneration
    const controller = new AbortController()
    this.pollAbort = controller

    try {
      await notifyStart({
        baseUrl: this.state.baseUrl,
        token: this.token,
      })
    } catch (err: any) {
      console.warn(`[OpenClaw] notifyStart failed (ignored):`, err.message)
    }

    console.log(`[OpenClaw] Starting message polling on ${this.state.baseUrl} gen=${generation}`)


    let consecutiveErrors = 0
    const MAX_CONSECUTIVE_ERRORS = 3
    const poll = async () => {
      while (this.pollGeneration === generation && this.polling && !controller.signal.aborted) {
        try {
          const resp = await getUpdates({
            baseUrl: this.state.baseUrl,
            token: this.token!,
            get_updates_buf: this.getUpdatesBuf,
            timeoutMs: 35000,
          })

          if (this.pollGeneration !== generation) break

          if (resp.errcode && resp.errcode !== 0) {
            consecutiveErrors++
            console.error(`[OpenClaw] getUpdates error (${consecutiveErrors}/${MAX_CONSECUTIVE_ERRORS}): ${resp.errcode} ${resp.errmsg}`)
            if (resp.errcode === -14 && consecutiveErrors >= MAX_CONSECUTIVE_ERRORS) {
              console.warn(`[OpenClaw] Session expired after ${consecutiveErrors} errors, auto-reconnecting...`)
              this.polling = false
              this.autoReconnect().catch((err) => console.error("[OpenClaw] Auto-reconnect failed:", err))
              break
            }
            await new Promise((r) => setTimeout(r, 5000))
            continue
          }

          consecutiveErrors = 0
          console.log("[OpenClaw] getUpdates OK: ret=" + (resp.ret ?? "?") + " msgs=" + ((resp.msgs && resp.msgs.length) || 0))
          if (resp.get_updates_buf) {
            this.getUpdatesBuf = resp.get_updates_buf
            this.persistBuf()
          }

          if (resp.msgs && resp.msgs.length > 0) {
            for (const msg of resp.msgs) {
              await this.processMessage(msg)
            }
          }
        } catch (err: any) {
          if (this.pollGeneration !== generation) break
          if (err.name === "AbortError") break
          console.error(`[OpenClaw] Poll error:`, err.message)
          this.sessionStaleCount++
          await this.checkSessionHealth()
          await new Promise((r) => setTimeout(r, 3000))
        }
      }
    }

    const loopPromise = poll().catch((err) => console.error("[OpenClaw] Poll loop crashed:", err))

    const cleanup = async () => {
      await loopPromise
      if (this.pollGeneration === generation) {
        this.polling = false
        this.pollPromise = null
      }
    }

    this.pollPromise = cleanup()
  }

  private async autoReconnect(): Promise<void> {
    console.log("[OpenClaw] ========== AUTO-RECONNECT START ==========")

    const savedAccountId = this.state.accountId

    this.pollAbort?.abort()
    this.pollAbort = null
    this.getUpdatesBuf = ""
    this.token = null
    this.state.status = "idle"
    this.state.qrCodeUrl = ""
    this.state.sessionKey = ""
    this.state.message = ""
    this.state.lastError = null

    try {
      const loginResult = await this.startLogin()
      console.log("[OpenClaw] QR code ready - re-scan same bot to restore session")

      const scanResult = await this.waitForScan(120000)

      if (scanResult.connected) {
        this.debugLog("[OpenClaw] Auto-reconnect: scan confirmed (bot: " + this.hashId(this.state.accountId ?? "?") + ")")
        await this.startPolling()
        console.log("[OpenClaw] ========== AUTO-RECONNECT SUCCESS ==========")
      } else {
        console.warn("[OpenClaw] Auto-reconnect: scan timeout:", scanResult.message)
        this.state.status = "idle"
        this.state.accountId = savedAccountId
        this.state.lastError = "Scan timeout - QR still available, please rescan"
      }
    } catch (err: any) {
      console.error("[OpenClaw] Auto-reconnect failed:", err.message)
      this.state.status = "error"
      this.state.accountId = savedAccountId
      this.state.lastError = "Auto-reconnect failed: " + err.message
    }
  }

  private async checkSessionHealth(): Promise<void> {
    const STALE_THRESHOLD = 5
    if (this.sessionStaleCount >= STALE_THRESHOLD && !this.sessionWarned) {
      this.sessionWarned = true
      console.warn("[OpenClaw] Session may be expiring, sending pre-expiry warning...")
      await this.sendPreExpiryWarning()
    }
  }

  private async sendPreExpiryWarning(): Promise<void> {
    try {
      const lastUserId = this._lastFromUserId
      if (!lastUserId) return

      console.log("[OpenClaw] Generating pre-expiry QR code...")
      await this.startLogin()

      const qrUrl = this.state.qrCodeUrl
      const msg = [
        "🔔 连接即将过期",
        "",
        "请打开以下链接重新扫码，保持我们的连接：",
        qrUrl,
        "",
        "（如果我已经不回复了，去 http://127.0.0.1:5173 扫码即可）",
      ].join("\n")

      await this.sendTextMessage(lastUserId, msg)
      this.debugLog("[OpenClaw] Pre-expiry QR sent to: " + this.hashId(lastUserId))
    } catch (err: any) {
      console.error("[OpenClaw] Failed to send pre-expiry warning:", err.message)
    }
  }

  async sendTextMessage(
    toUserId: string,
    text: string,
    contextToken?: string,
    deliveryKey?: string
  ): Promise<DeliveryExecutionResult> {
    if (!this.token) throw new Error("Not logged in")

    if (deliveryKey) {
      const store = this.getDeliveryStore()
      return store.execute(deliveryKey, async (clientId) => {
        await sendMessage({
          baseUrl: this.state.baseUrl,
          token: this.token!,
          body: {
            msg: {
              from_user_id: "",
              to_user_id: toUserId,
              client_id: clientId,
              message_type: 2,
              message_state: 2,
              context_token: contextToken || "",
              item_list: [{ type: 1, text_item: { text } }],
            },
          },
        })
        this.state.replyCount++
        this.persistStats()
      })
    }

    this.debugLog("[OpenClaw] Sending to via SDK sendMessage...")

    try {
      await sendMessage({
        baseUrl: this.state.baseUrl,
        token: this.token,
        body: {
          msg: {
            from_user_id: "",
            to_user_id: toUserId,
            client_id: `openclaw-weixin:${Date.now()}-${crypto.randomBytes(4).toString("hex")}`,
            message_type: 2,
            message_state: 2,
            context_token: contextToken || "",
            item_list: [{ type: 1, text_item: { text } }],
          },
        },
      })
      this.state.replyCount++
      this.persistStats()
      console.log(`[OpenClaw] SDK sendMessage completed`)
      this.debugLog("[OpenClaw] Sent to: ")
      return { duplicate: false, clientId: "" }
    } catch (err: any) {
      console.error(`[OpenClaw] Send exception:`, err.message)
      throw err
    }
  }

  async sendVoiceMessage(
    toUserId: string,
    audioBuffer: Buffer,
    encodeType: number = 7,
    playtime: number = 0,
    contextToken?: string,
    deliveryKey?: string
  ): Promise<DeliveryExecutionResult> {
    if (!this.token) throw new Error("Not logged in")

    if (deliveryKey) {
      const store = this.getDeliveryStore()
      return store.execute(deliveryKey, async (clientId) => {
        await this.sendVoiceInternal(toUserId, audioBuffer, encodeType, playtime, contextToken, clientId)
        this.state.replyCount++
        this.persistStats()
      })
    }

    const clientId = `openclaw-weixin:${Date.now()}-${crypto.randomBytes(4).toString("hex")}`
    await this.sendVoiceInternal(toUserId, audioBuffer, encodeType, playtime, contextToken, clientId)
    this.state.replyCount++
    this.persistStats()
    return { duplicate: false, clientId }
  }

  private async sendVoiceInternal(
    toUserId: string,
    audioBuffer: Buffer,
    encodeType: number,
    playtime: number,
    contextToken: string | undefined,
    clientId: string
  ): Promise<void> {
    const rawsize = audioBuffer.length
    const rawfilemd5 = crypto.createHash("md5").update(audioBuffer).digest("hex")
    const aesKey = crypto.randomBytes(16)
    const encrypted = this.aes128EcbEncrypt(audioBuffer, aesKey)
    const filesize = encrypted.length
    const filekey = `voice_${Date.now()}_${crypto.randomBytes(4).toString("hex")}.mp3`

    console.log(`[OpenClaw][VOICE-SEND] rawsize=${rawsize} filesize=${filesize} filekey=${filekey}`)

    const uploadResp = await getUploadUrl({
      baseUrl: this.state.baseUrl,
      token: this.token!,
      filekey,
      media_type: 4,
      to_user_id: toUserId,
      rawsize,
      rawfilemd5,
      filesize,
      aeskey: aesKey.toString("hex"),
      no_need_thumb: true,
    })

    console.log("[OpenClaw][VOICE-SEND] uploadResp errcode=" + uploadResp.errcode + " upload_full_url=" + (uploadResp.upload_full_url ? "OK" : "MISSING"))
    console.log("[OpenClaw][VOICE-SEND] uploadResp keys: " + Object.keys(uploadResp).join(","))

    if (uploadResp.errcode && uploadResp.errcode !== 0) {
      throw new Error("getUploadUrl failed: " + uploadResp.errcode + " " + (uploadResp.errmsg || ""))
    }

    if (!uploadResp.upload_full_url) {
      throw new Error("getUploadUrl returned no upload_full_url")
    }

    const encryptQueryParam = uploadResp.encrypt_query_param || (() => {
      const m = uploadResp.upload_full_url.match(/encrypted_query_param=([^&]+)/)
      return m ? decodeURIComponent(m[1]) : ""
    })()

    if (!encryptQueryParam) throw new Error("encrypt_query_param missing")

    console.log("[OpenClaw][VOICE-SEND] CDN POST...")
    const putResp = await fetch(uploadResp.upload_full_url, {
      method: "POST",
      body: encrypted,
      signal: AbortSignal.timeout(30000),
    })

    console.log("[OpenClaw][VOICE-SEND] CDN PUT response: status=" + putResp.status + " ok=" + putResp.ok)

    if (!putResp.ok) {
      throw new Error("CDN upload failed: " + putResp.status)
    }

    console.log("[OpenClaw][VOICE-SEND] CDN PUT OK")

    const accountId = this.getState().accountId || ""

    await sendMessage({
      baseUrl: this.state.baseUrl,
      token: this.token!,
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
                aes_key: Buffer.from(aesKey.toString("hex")).toString("base64"),
              },
              encode_type: encodeType,
              playtime,
            },
          }],
        },
      },
    })

    console.log("[OpenClaw][VOICE-SEND] Message sent OK")
  }

  async sendImageMessage(toUserId: string, imageBuffer: Buffer, contextToken?: string, deliveryKey?: string): Promise<DeliveryExecutionResult> {
    if (!this.token) throw new Error("Not logged in")

    if (deliveryKey) {
      const store = this.getDeliveryStore()
      return store.execute(deliveryKey, async (clientId) => {
        await this.sendImageInternal(toUserId, imageBuffer, contextToken, clientId)
        this.state.replyCount++
        this.persistStats()
      })
    }

    const clientId = `openclaw-weixin:${Date.now()}-${crypto.randomBytes(4).toString("hex")}`
    await this.sendImageInternal(toUserId, imageBuffer, contextToken, clientId)
    this.state.replyCount++
    this.persistStats()
    return { duplicate: false, clientId }
  }

  private async sendImageInternal(toUserId: string, imageBuffer: Buffer, contextToken: string | undefined, clientId: string): Promise<void> {
    const rawsize = imageBuffer.length
    const rawfilemd5 = crypto.createHash("md5").update(imageBuffer).digest("hex")
    const aesKey = crypto.randomBytes(16)
    const encrypted = this.aes128EcbEncrypt(imageBuffer, aesKey)
    const uploadResp = await getUploadUrl({
      baseUrl: this.state.baseUrl,
      token: this.token!,
      filekey: `image_${Date.now()}_${crypto.randomBytes(4).toString("hex")}.png`,
      media_type: 2,
      to_user_id: toUserId,
      rawsize,
      rawfilemd5,
      filesize: encrypted.length,
      aeskey: aesKey.toString("hex"),
      no_need_thumb: true,
    })
    if (uploadResp.errcode && uploadResp.errcode !== 0) throw new Error("getUploadUrl failed: " + uploadResp.errcode)
    if (!uploadResp.upload_full_url) throw new Error("getUploadUrl returned no upload_full_url")
    const encryptQueryParam = uploadResp.encrypt_query_param || (() => {
      const match = uploadResp.upload_full_url.match(/encrypted_query_param=([^&]+)/)
      return match ? decodeURIComponent(match[1]) : ""
    })()
    if (!encryptQueryParam) throw new Error("encrypt_query_param missing")
    const upload = await fetch(uploadResp.upload_full_url, { method: "POST", body: encrypted, signal: AbortSignal.timeout(30000) })
    if (!upload.ok) throw new Error("CDN upload failed: " + upload.status)
    await sendMessage({
      baseUrl: this.state.baseUrl,
      token: this.token!,
      body: { msg: { from_user_id: this.getState().accountId || "", to_user_id: toUserId, client_id: clientId, message_type: 2, message_state: 2, context_token: contextToken || "", item_list: [{ type: 2, image_item: { media: { encrypt_query_param: encryptQueryParam, aes_key: Buffer.from(aesKey.toString("hex")).toString("base64") } } }] } },
    })
  }

  private aes128EcbEncrypt(data: Buffer, key: Buffer): Buffer {
    const cipher = crypto.createCipheriv("aes-128-ecb", key, Buffer.alloc(0))
    cipher.setAutoPadding(true)
    return Buffer.concat([cipher.update(data), cipher.final()])
  }

  async stopPolling(): Promise<void> {
    this.polling = false
    this.pollGeneration++
    this.pollAbort?.abort()

    if (this.token) {
      try {
        await notifyStop({
          baseUrl: this.state.baseUrl,
          token: this.token,
        })
      } catch {
      }
    }

    if (this.pollPromise) {
      await this.pollPromise
    }
  }

  async resetLogin(): Promise<void> {
    await this.stopPolling()

    const oldAccountId = this.state.accountId
    if (oldAccountId) {
      try {
        unregisterWeixinAccountId(oldAccountId)
        clearWeixinAccount(oldAccountId)
        this.debugLog("[OpenClaw] Cleared old account: ")
      } catch (err: any) {
        console.warn(`[OpenClaw] Failed to clear old account:`, err.message)
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
      lastError: null,
    }
    this.getUpdatesBuf = ""
    this.token = null
    this.resetDeliveryStore()
  }

  private persistBuf(): void {
    if (!this.state.accountId) return
    try {
      const data: Record<string, any> = { messageCount: this.state.messageCount }
      if (this.getUpdatesBuf) data.getUpdatesBuf = this.getUpdatesBuf
      saveWeixinAccount(this.state.accountId, data)
    } catch {
    }
  }

  private resolveInboundMessageId(msg: WeixinMessage): string {
    if (msg.message_id) {
      return String(msg.message_id)
    }
    const fields = {
      from_user_id: msg.from_user_id ?? "",
      to_user_id: msg.to_user_id ?? "",
      create_time_ms: msg.create_time_ms ?? 0,
      message_type: msg.message_type ?? 0,
      context_token: msg.context_token ?? "",
      item_list: msg.item_list ?? [],
    }
    const normalized = JSON.stringify(fields, Object.keys(fields).sort())
    const hash = crypto.createHash("sha256").update(normalized).digest("hex")
    return `fallback-${hash}`
  }

  private checkInboundDedupe(accountId: string, messageId: string): boolean {
    const key = `${accountId}:${messageId}`
    const now = Date.now()
    const seen = this.inboundSeen.get(key)
    if (seen && (now - seen) < 10 * 60 * 1000) {
      return true
    }
    this.inboundSeen.set(key, now)
    this.cleanupInboundSeen()
    return false
  }

  private cleanupInboundSeen(): void {
    if (this.inboundSeen.size < 1000) return
    const cutoff = Date.now() - 10 * 60 * 1000
    for (const [key, ts] of this.inboundSeen) {
      if (ts < cutoff) {
        this.inboundSeen.delete(key)
      }
    }
  }

  private async processMessage(msg: WeixinMessage): Promise<void> {
    console.log("[OpenClaw][DIAG] === processMessage === msg_type=" + msg.message_type + " from=" + this.hashId(String(msg.from_user_id || '')) + " items.len=" + (msg.item_list ? msg.item_list.length : 0));
    if (msg.message_type === 2) return

    const fromUserId = msg.from_user_id || ""
    this._lastFromUserId = fromUserId
    const toUserId = msg.to_user_id || ""
    const contextToken = msg.context_token || ""
    const accountId = this.state.accountId || ""
    const messageId = this.resolveInboundMessageId(msg)
    const createdAt = msg.create_time_ms || Date.now()

    if (this.checkInboundDedupe(accountId, messageId)) {
      console.log("[OpenClaw] Duplicate inbound message skipped: " + this.hashId(messageId))
      return
    }

    let text = ""
    let isVoice = false
    if (msg.item_list) {
      console.log("[OpenClaw][DIAG] item_list: " + msg.item_list.length + " items");
      for (const item of msg.item_list) {
        if (item.type === 3 && item.voice_item) {
          isVoice = true
          console.log("[OpenClaw][DIAG] voice_item: playtime=" + item.voice_item.playtime + " encode_type=" + item.voice_item.encode_type + " hasText=" + (item.voice_item.text ? "YES len=" + item.voice_item.text.length : "NO") + " hasMedia=" + !!item.voice_item.media);
          const vt = item.voice_item.text || ""
          console.log("[OpenClaw][VOICE] playtime=" + item.voice_item.playtime + " encode_type=" + item.voice_item.encode_type + " text=" + vt.substring(0, 100))
          if (vt) text += vt
        }
        if (item.type === 1 && item.text_item?.text) {
          text += item.text_item.text
        }
      }
    }

    let audioBase64: string | undefined; let audioMime: string | undefined; let imageBase64: string | undefined
    let imageUrl: string | undefined
    let aeskey: string | undefined
    if (msg.item_list) {
      console.log("[OpenClaw][DIAG] item_list: " + msg.item_list.length + " items");
      for (const item of msg.item_list) {
        if (item.type === 3 && item.voice_item) {
          if (item.voice_item.media && item.voice_item.media.encrypt_query_param && item.voice_item.media.aes_key) {
            try {
              console.log("[OpenClaw][VOICE-DL] 开始下载语音...")
              const silkBuf = await downloadAndDecryptBuffer(
                item.voice_item.media.encrypt_query_param,
                item.voice_item.media.aes_key,
                this.state.baseUrl,
                "wechat-voice",
                item.voice_item.media.full_url
              )
              console.log("[OpenClaw][VOICE-DL] 下载完成, size=" + silkBuf.length)
              const wavBuf = await silkToWav(silkBuf)
              const audioBuf = wavBuf || silkBuf
              audioBase64 = audioBuf.toString("base64")
              audioMime = wavBuf ? "audio/wav" : "audio/silk"
              console.log("[OpenClaw][VOICE-DL] 转码完成, size=" + audioBuf.length + " mime=" + audioMime)
            } catch (dlErr: any) {
              console.error("[OpenClaw][VOICE-DL] 下载失败: " + (dlErr?.message || String(dlErr)))
            }
          }
        }
        if (item.type === 2 && item.image_item) {
          console.log("[OpenClaw][IMAGE] === 收到图片消息 from " + this.hashId(String(fromUserId)) + " ===")
          console.log("[OpenClaw][IMAGE] image_item keys: " + Object.keys(item.image_item).join(", "))
          const img = item.image_item
          if (img.url) {
            imageUrl = img.url
            console.log("[OpenClaw][IMAGE] url: " + img.url)
          }
          if (img.media) {
            console.log("[OpenClaw][IMAGE] media keys: " + Object.keys(img.media).join(", "))
            if (img.media.full_url) {
              imageUrl = img.media.full_url
              console.log("[OpenClaw][IMAGE] media.full_url: " + img.media.full_url)
            }
            if (img.media.encrypt_query_param) {
              console.log("[OpenClaw][IMAGE] encrypt_query_param: " + img.media.encrypt_query_param.substring(0, 80) + "...")
            }
            if (img.media.aes_key) {
              console.log("[OpenClaw][IMAGE] aes_key: " + img.media.aes_key.substring(0, 16) + "...")
            }
          }
          if (img.aeskey) {
            aeskey = img.aeskey
            console.log("[OpenClaw][IMAGE] aeskey: " + img.aeskey.substring(0, 16) + "...")
          }
          console.log("[OpenClaw][IMAGE] mid_size=" + img.mid_size + " hd_size=" + img.hd_size)
          console.log("[OpenClaw][IMAGE] thumb: " + img.thumb_width + "x" + img.thumb_height + " size=" + img.thumb_size)
          console.log("[OpenClaw][IMAGE] === 图片信息打印完毕 ===")
        }
      }
    }

    console.log("[OpenClaw][DIAG] textCheck: text=" + JSON.stringify(text.substring(0,100)) + " isVoice=" + isVoice + " hasImage=" + !!imageUrl);
    if (!text && !imageUrl) {
      console.log("[OpenClaw][DIAG] *** 消息被丢弃: text为空且无图片 ***");
      console.log("[OpenClaw] Non-text msg from userId=" + this.hashId(String(fromUserId)))
      return
    }

    this.state.messageCount++
    this.persistBuf()
    this.persistStats()

    console.log("[OpenClaw] Msg from " + this.hashId(String(fromUserId)) + ": " + (text.length > 80 ? text.substring(0, 80) + "..." : text))

    console.log("[OpenClaw][DIAG] 调用handler: text=" + text.substring(0,80) + " isVoice=" + isVoice + " handlers=" + this.handlers.length);
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
          createdAt,
        })

      } catch (err: any) {
        console.error(`[OpenClaw] Handler error:`, err.message)

      }
    }
  }
}

let instance: OpenClawWechatManager | null = null
export function getWechatManager(): OpenClawWechatManager {
  if (!instance) instance = new OpenClawWechatManager()
  return instance
}

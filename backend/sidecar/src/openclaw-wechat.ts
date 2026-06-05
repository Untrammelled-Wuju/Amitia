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
import crypto from "node:crypto"

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
  createdAt: number
}) => Promise<string | string[] | void>

// ============================================================
// State Manager
// ============================================================

export class OpenClawWechatManager {
  
  /** Debug-level logging for sensitive data - disabled in production */
  private debugLog(...args: unknown[]): void {
    if (process.env.NODE_ENV === "development" || process.env.DEBUG) {
      console.debug(...args)
    }
  }

  /** Hash a sensitive identifier for safe logging */
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
    baseUrl: DEFAULT_BASE_URL,
    startedAt: null,
    lastError: null,
  }

  private polling = false
  private pollAbort = new AbortController()
  private handlers: MessageHandler[] = []
  private sessionStaleCount = 0
  private sessionWarned = false

  private _lastFromUserId = ""

  private getUpdatesBuf = ""
  private token: string | null = null

  getState(): WechatState {
    return { ...this.state }
  }

  onMessage(handler: MessageHandler): void {
    this.handlers.push(handler)
  }

  /** Try to load previously saved credentials */
  loadSavedAccount(): boolean {
    const ids = listIndexedWeixinAccountIds()
    if (ids.length === 0) return false

    const id = ids[ids.length - 1] // latest account
    const data = loadWeixinAccount(id)
    if (data?.token) {
      this.state.accountId = id
      this.state.status = "connected"
      this.state.baseUrl = data.baseUrl || DEFAULT_BASE_URL
      this.state.startedAt = new Date().toISOString()
      this.token = data.token
      // Restore message cursor so polling resumes where it left off
      if (data.getUpdatesBuf) {
        this.getUpdatesBuf = data.getUpdatesBuf
      }
      // Restore message count
      if (typeof (data as any).messageCount === "number") {
        this.state.messageCount = (data as any).messageCount
      }
      this.debugLog("[OpenClaw] Loaded saved account: ")
      return true
    }
    return false
  }

  /** Start QR login flow */
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

      // Generate QR code as base64 data URL
      const QRCode = await import("qrcode")
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

  /** Wait for QR scan confirmation */
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

      this.state.status = "error"
      this.state.lastError = result.message
      return { connected: false, message: result.message }
    } catch (err: any) {
      this.state.status = "error"
      this.state.lastError = err.message
      console.error(`[OpenClaw] waitForScan error:`, err.message)
      return { connected: false, message: err.message }
    }
  }

  /** Start message polling loop */
  async startPolling(): Promise<void> {
    if (this.polling) return
    if (!this.token || !this.state.accountId) {
      console.warn("[OpenClaw] Cannot start polling: no credentials")
      return
    }

    this.polling = true
    this.pollAbort = new AbortController()

    // Notify start
    try {
      await notifyStart({
        baseUrl: this.state.baseUrl,
        token: this.token,
      })
    } catch (err: any) {
      console.warn(`[OpenClaw] notifyStart failed (ignored):`, err.message)
    }

    console.log(`[OpenClaw] Starting message polling on ${this.state.baseUrl}`)

    const poll = async () => {
      while (this.polling && !this.pollAbort.signal.aborted) {
        try {
          const resp = await getUpdates({
            baseUrl: this.state.baseUrl,
            token: this.token!,
            get_updates_buf: this.getUpdatesBuf,
            timeoutMs: 35000,
          })

          if (resp.errcode && resp.errcode !== 0) {
            console.error(`[OpenClaw] getUpdates error: ${resp.errcode} ${resp.errmsg}`)
            if (resp.errcode === -14) {
              console.warn("[OpenClaw] Session expired, auto-reconnecting...")
              this.polling = false
              this.autoReconnect().catch((err) => console.error("[OpenClaw] Auto-reconnect failed:", err))
              break
            }
            await new Promise((r) => setTimeout(r, 5000))
            continue
          }

          // Update cursor
          console.log("[OpenClaw] getUpdates OK: ret=" + (resp.ret ?? "?") + " msgs=" + ((resp.msgs && resp.msgs.length) || 0))
          if (resp.get_updates_buf) {
            this.getUpdatesBuf = resp.get_updates_buf
            // Persist cursor for crash recovery
            this.persistBuf()
          }

          // Process messages
          if (resp.msgs && resp.msgs.length > 0) {
            for (const msg of resp.msgs) {
              await this.processMessage(msg)
            }
          }
        } catch (err: any) {
          if (err.name === "AbortError") break
          console.error(`[OpenClaw] Poll error:`, err.message)
          this.sessionStaleCount++
          await this.checkSessionHealth()
          await new Promise((r) => setTimeout(r, 3000))
        }
      }
    }

    poll().catch((err) => console.error("[OpenClaw] Poll loop crashed:", err))
  }

  /** Persist getUpdatesBuf to account data for crash recovery */
  private persistBuf(): void {
    if (!this.state.accountId) return
    try {
      const data: Record<string, any> = { messageCount: this.state.messageCount }
      if (this.getUpdatesBuf) data.getUpdatesBuf = this.getUpdatesBuf
      saveWeixinAccount(this.state.accountId, data)
    } catch {
      // Non-critical, ignore
    }
  }

  /** Stop message polling */
  async stopPolling(): Promise<void> {
    this.polling = false
    this.pollAbort.abort()

    if (this.token) {
      try {
        await notifyStop({
          baseUrl: this.state.baseUrl,
          token: this.token,
        })
      } catch {
        // ignore
      }
    }
  }

  /** Reset current login state so a fresh QR scan can be started. */

  /**
   * Auto-reconnect when session expires.
   * Clears old credentials and starts a fresh QR login, waits for scan, then resumes polling.
   * Called automatically when getUpdates returns errcode=-14.
   */
  private async autoReconnect(): Promise<void> {
    console.log("[OpenClaw] ========== AUTO-RECONNECT START ==========")
    
    // Keep the same bot account - just re-authenticate with a fresh QR scan
    const savedAccountId = this.state.accountId
    
    // Reset polling state only, preserve account identity
    this.pollAbort = new AbortController()
    this.getUpdatesBuf = ""
    this.token = null
    this.state.status = "idle"
    this.state.qrCodeUrl = ""
    this.state.sessionKey = ""
    this.state.message = ""
    this.state.lastError = null
    
    try {
      // Re-login with same bot (no force=true, reuses existing bot identity)
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


  /**
   * Check session health. If consecutive errors exceed threshold,
   * send a pre-expiry QR code warning to the last known user while session is still alive.
   */
  private async checkSessionHealth(): Promise<void> {
    const STALE_THRESHOLD = 5
    if (this.sessionStaleCount >= STALE_THRESHOLD && !this.sessionWarned) {
      this.sessionWarned = true
      console.warn("[OpenClaw] Session may be expiring, sending pre-expiry warning...")
      await this.sendPreExpiryWarning()
    }
  }

  /**
   * Generate a fresh QR code and send the URL to the last known WeChat user.
   * Called while session is still barely alive to give user a chance to re-scan.
   */
  private async sendPreExpiryWarning(): Promise<void> {
    try {
      // Get last known user from message handler state (stored during processMessage)
      const lastUserId = this._lastFromUserId
      if (!lastUserId) return
      
      // Generate a fresh QR in background
      console.log("[OpenClaw] Generating pre-expiry QR code...")
      await this.startLogin()
      
      // Send warning with QR URL
      const qrUrl = this.state.qrCodeUrl
      const msg = [
        "🔔 连接即将过期",
        "",
        "请打开以下链接重新扫码，保持我们的连接：",
        qrUrl,
        "",
        "（如果我已经不回复了，去 http://127.0.0.1:5173 扫码即可）",
      ].join("\\n")
      
      await this.sendTextMessage(lastUserId, msg)
      this.debugLog("[OpenClaw] Pre-expiry QR sent to: " + this.hashId(lastUserId))
    } catch (err: any) {
      console.error("[OpenClaw] Failed to send pre-expiry warning:", err.message)
    }
  }

  async resetLogin(): Promise<void> {
    await this.stopPolling()

    // Clean up old account from SDK persistent store so server creates a new bot
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
      baseUrl: DEFAULT_BASE_URL,
      startedAt: null,
      lastError: null,
    }
    this.getUpdatesBuf = ""
    this.token = null
  }

  /** Send a text message back to WeChat */
    
  /** Send a text message back to WeChat */
  /** Send a text message back to WeChat */
  /** Send a text message back to WeChat */
  async sendTextMessage(
    toUserId: string,
    text: string,
    contextToken?: string
  ): Promise<void> {
    if (!this.token) throw new Error("Not logged in")
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
      console.log(`[OpenClaw] SDK sendMessage completed`)
      this.debugLog("[OpenClaw] Sent to: ")
    } catch (err: any) {
      console.error(`[OpenClaw] Send exception:`, err.message)
    }
  }

  /** Stop message polling */
  async stopPolling(): Promise<void> {
    this.polling = false
    this.pollAbort.abort()

    if (this.token) {
      try {
        await notifyStop({
          baseUrl: this.state.baseUrl,
          token: this.token,
        })
      } catch {
        // ignore
      }
    }
  }

  /** Reset current login state so a fresh QR scan can be started. */
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
      baseUrl: DEFAULT_BASE_URL,
      startedAt: null,
      lastError: null,
    }
    this.getUpdatesBuf = ""
    this.token = null
  }

  /** Persist getUpdatesBuf to account data for crash recovery */
  private persistBuf(): void {
    if (!this.state.accountId) return
    try {
      const data: Record<string, any> = { messageCount: this.state.messageCount }
      if (this.getUpdatesBuf) data.getUpdatesBuf = this.getUpdatesBuf
      saveWeixinAccount(this.state.accountId, data)
    } catch {
      // Non-critical, ignore
    }
  }


  async restartPolling(): Promise<void> {
    // Reset abort controller without notifyStop (which tells server bot is offline)
    this.pollAbort.abort()
    this.pollAbort = new AbortController()
    this.polling = true

    // Re-notify start to refresh session
    try {
      await notifyStart({ baseUrl: this.state.baseUrl, token: this.token! })
      console.log("[OpenClaw] Polling restarted after reply")
    } catch (err: any) {
      console.error("[OpenClaw] notifyStart failed during restart:", err.message)
    }

    const poll = async () => {
      while (this.polling) {
        try {
          if (this.pollAbort.signal.aborted) break
          const resp = await getUpdates({
            baseUrl: this.state.baseUrl, token: this.token!,
            get_updates_buf: this.getUpdatesBuf, timeoutMs: 35000,
          })

          // Check for session errors (align with startPolling)
          if (resp.errcode && resp.errcode !== 0) {
            console.error("[OpenClaw] getUpdates error: errcode=" + resp.errcode + " errmsg=" + (resp.errmsg || ""))
            if (resp.errcode === -14) {
              console.warn("[OpenClaw] Session expired, auto-reconnecting...")
              this.polling = false
              this.autoReconnect().catch((err) => console.error("[OpenClaw] Auto-reconnect failed:", err))
              break
            }
            await new Promise((r) => setTimeout(r, 5000))
            continue
          }

          console.log("[OpenClaw] getUpdates OK: ret=" + (resp.ret ?? "?") + " msgs=" + ((resp.msgs && resp.msgs.length) || 0))

          if (resp.get_updates_buf) { this.getUpdatesBuf = resp.get_updates_buf; this.persistBuf() }
          if (resp.msgs && resp.msgs.length > 0) {
            for (const msg of resp.msgs) { await this.processMessage(msg) }
          }
        } catch (err: any) {
          if (err.name === "AbortError") break
          console.error("[OpenClaw] Poll error:", err.message)
          await new Promise((r) => setTimeout(r, 3000))
        }
      }
    }
    poll().catch((err) => console.error("[OpenClaw] Poll loop crashed:", err))
  }

  private async processMessage(msg: WeixinMessage): Promise<void> {
    // Skip bot messages (our own replies)
    if (msg.message_type === 2) return

    const fromUserId = msg.from_user_id || ""
    this._lastFromUserId = fromUserId
    const toUserId = msg.to_user_id || ""
    const contextToken = msg.context_token || ""
    const messageId = String(msg.message_id || Date.now())
    const createdAt = msg.create_time_ms || Date.now()

    // Extract text from items
    let text = ""
    if (msg.item_list) {
      for (const item of msg.item_list) {
        if (item.type === 1 && item.text_item?.text) {
          text += item.text_item.text
        }
      }
    }

    if (!text) {
      console.log("[OpenClaw] Non-text msg from userId=" + this.hashId(String(fromUserId)))
      return
    }

    this.state.messageCount++
    this.persistBuf()

    console.log("[OpenClaw] Msg from " + this.hashId(String(fromUserId)) + ": " + (text.length > 80 ? text.substring(0, 80) + "..." : text))

    for (const handler of this.handlers) {
      try {
        const reply = await handler({
          fromUserId,
          toUserId,
          messageId,
          text,
          contextToken,
          createdAt,
        })

        if (reply) {
          // Split by newline for WeChat-style multi-message delivery (primary)
          const rawReply = typeof reply === "string" ? reply : String(reply)
          console.log('[OpenClaw] Raw reply (' + rawReply.length + ' chars): ' + rawReply.substring(0, 200))
          
          let replyParts = rawReply.split("\n").map((p) => p.trim()).filter((p) => p.length > 0)
          if (replyParts.length <= 1) {
            replyParts = rawReply.split("\\").map((p) => p.trim()).filter((p) => p.length > 0)
            if (replyParts.length > 1) console.log('[OpenClaw] Split by \\ into ' + replyParts.length + ' parts')
          }
          if (replyParts.length <= 1) {
            replyParts = rawReply.split('/').map((p) => p.trim()).filter((p) => p.length > 0)
            if (replyParts.length > 1) console.log('[OpenClaw] Split by / into ' + replyParts.length + ' parts')
          }
          console.log('[OpenClaw] Split into ' + replyParts.length + ' part(s)')

          for (let i = 0; i < replyParts.length; i++) {
            await this.sendTextMessage(fromUserId, replyParts[i], contextToken).catch(
              (err) => console.error(`[OpenClaw] Reply part ${i+1}/${replyParts.length} failed:`, err.message)
            )
            if (i < replyParts.length - 1) {
              await new Promise(r => setTimeout(r, 800 + Math.random() * 1200))
            }
          }

        // Note: not restarting polling here to avoid notifyStop session damage
        // If session expires, getUpdates errcode check will handle re-login
        }
      } catch (err: any) {
        console.error(`[OpenClaw] Handler error:`, err.message)

      }
    }
  }
}

// Singleton
let instance: OpenClawWechatManager | null = null
export function getWechatManager(): OpenClawWechatManager {
  if (!instance) instance = new OpenClawWechatManager()
  return instance
}

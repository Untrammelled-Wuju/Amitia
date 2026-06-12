import Fastify from "fastify"
import cors from "@fastify/cors"

import { sidecarConfig } from "./config.js"
import { getWechatManager } from "./openclaw-wechat.js"

const app = Fastify({
  logger: { level: process.env.LOG_LEVEL || "info" },
})

await app.register(cors, {
  origin: [/^http:\/\/127\.0\.0\.1:\d+$/, /^http:\/\/localhost:\d+$/],
  methods: ['GET', 'POST', 'OPTIONS'],
  credentials: false,
})


// ============================================================
// Auth guard DISABLED for debugging
app.addHook("onRequest", async (req, reply) => {
  // Allow all local requests
  return
})


// Security headers
app.addHook("onSend", async (_req, reply) => {
  reply.header("X-Content-Type-Options", "nosniff")
  reply.header("X-Frame-Options", "DENY")
  void reply.header("X-Powered-By", "")
})

// ============================================================
const manager = getWechatManager()
// 消息去抖缓冲区：按 fromUserId 分组
const msgBuffers = new Map<string, { msgs: Array<{ text: string; contextToken: string; fromUserId: string; toUserId: string; messageId: string; createdAt: number; isVoice?: boolean }>; timer: ReturnType<typeof setTimeout> }>()

// ============================================================
// Forward messages to Core AI
// ============================================================

manager.onMessage(async (msg) => {
  // ---- 5秒去抖缓冲：连续消息自动合并 ----
  const BUFFER_MS = 5000
  type BMsg = { text: string; contextToken: string; fromUserId: string; toUserId: string; messageId: string; createdAt: number; isVoice?: boolean }
  
  // 使用 module-level Map（定义在文件顶部）
  const key = msg.fromUserId
  const existing = msgBuffers.get(key)
  const item: BMsg = { text: msg.text, contextToken: msg.contextToken || "", fromUserId: msg.fromUserId, toUserId: msg.toUserId, messageId: msg.messageId, createdAt: msg.createdAt, isVoice: (msg as any).isVoice || false }

  if (existing) {
    clearTimeout(existing.timer)
    existing.msgs.push(item)
    console.log(`[Sidecar] Buffer +1 (total ${existing.msgs.length}): "${msg.text.substring(0, 40)}"`)
  } else {
    msgBuffers.set(key, { msgs: [item], timer: null as any })
    console.log(`[Sidecar] Buffer start: "${msg.text.substring(0, 40)}"`)
  }

  const entry = msgBuffers.get(key)!
  entry.timer = setTimeout(async () => {
    msgBuffers.delete(key)
    const all = entry.msgs
    const last = all[all.length - 1]
    const combined = all.map(m => m.text).join("\n")
    console.log(`[Sidecar] FIRE (${all.length} msgs): "${combined.substring(0, 100)}"`)
    const wasVoice = all.some(m => m.isVoice === true)

    const headers: Record<string, string> = { "Content-Type": "application/json" }
    const t = sidecarConfig.bridgeApiToken
    if (t) headers["Authorization"] = "Bearer " + t

    try {
      const resp = await fetch(`${sidecarConfig.coreUrl}/api/agent/webhook`, {
        method: "POST", headers,
        body: JSON.stringify({
          channel: "wechat", accountId: "openclaw-wechat",
          conversationId: `conv-${last.fromUserId}`, senderId: last.fromUserId,
          messageId: last.messageId, contextToken: last.contextToken,
          type: wasVoice ? "voice" : "text", text: combined, createdAt: last.createdAt,
          voiceMessage: wasVoice,
          skipTiming: true,
        }),
        signal: AbortSignal.timeout(60000),
      })
      const json = await resp.json() as any
      console.log(`[Sidecar] Core: code=${json?.code}, hasText=${!!json?.data?.outgoingMessage?.text}`)
      if (json?.data?.outgoingMessage?.text) {
        const reply = json.data.outgoingMessage.text
        console.log('[OpenClaw] Reply (' + reply.length + ' chars): ' + reply.substring(0, 200))
        
        const forceVoice = json?.data?.outgoingMessage?.forceVoice === true
        const shouldSendVoice = false
        console.log('[OpenClaw] Voice decision: wasVoice=' + wasVoice + ' shouldSendVoice=' + shouldSendVoice + ' forceVoice=' + forceVoice)
        
        if (shouldSendVoice && reply.length > 0) {
          try {
            const parts = reply.split("\n").map((p: string) => p.trim()).filter((p: string) => p.length > 0)
            let voiceSent = false
            for (let i = 0; i < parts.length; i++) {
              const part = parts[i]
              try {
                const ttsResp = await fetch(sidecarConfig.coreUrl + "/api/tts/synthesize", {
                  method: "POST",
                  headers: { "Content-Type": "application/json" },
                  body: JSON.stringify({ text: part }),
                  signal: AbortSignal.timeout(60000),
                })
                const ttsJson = await ttsResp.json() as any
                const audioUrl = ttsJson?.data?.audioUrl
                if (audioUrl) {
                  const fullAudioUrl = sidecarConfig.coreUrl + audioUrl
                  const audioResp = await fetch(fullAudioUrl, { signal: AbortSignal.timeout(30000) })
                  if (audioResp.ok) {
                    const audioBuffer = Buffer.from(await audioResp.arrayBuffer())
                    await manager.sendVoiceMessage(last.fromUserId, audioBuffer, 7, 0, last.contextToken)
                    console.log('[OpenClaw] Voice part ' + (i+1) + ' sent OK')
                    voiceSent = true
                  }
                }
              } catch (partErr: any) {
                console.error('[OpenClaw] Voice part ' + (i+1) + ' error: ' + (partErr?.message || String(partErr)))
              }
              if (i < parts.length - 1) await new Promise(r => setTimeout(r, 800))
            }
            if (voiceSent) return
            console.log('[OpenClaw] No voice parts sent, falling back to text')
          } catch (ttsErr: any) {
            console.error('[OpenClaw] TTS/voice error: ' + (ttsErr?.message || String(ttsErr)) + ', falling back to text')
          }
        }
        
        const parts = reply.split("\n").map((p: string) => p.trim()).filter((p: string) => p.length > 0)
        console.log('[OpenClaw] Split into ' + parts.length + ' part(s)')
        for (let i = 0; i < parts.length; i++) {
          await manager.sendTextMessage(last.fromUserId, parts[i], last.contextToken).catch(
            (err: any) => console.error('[OpenClaw] Part ' + (i+1) + '/' + parts.length + ' failed:', err.message)
          )
          if (i < parts.length - 1) await new Promise(r => setTimeout(r, 800 + Math.random() * 1200))
        }
      }
    } catch (err: any) { console.error("[Sidecar] Forward failed:", err.message) }
  }, BUFFER_MS)

  return undefined
})


// ============================================================
// HTTP API
// ============================================================

// Health check
app.get("/api/health", async (_req, reply) => {
  const state = manager.getState()
  return reply.send({
    success: true,
    status: state.status,
    accountId: state.accountId,
    message: state.message,
  })
})

// Full status
app.get("/api/status", async (_req, reply) => {
  const state = manager.getState()
  return reply.send({
    success: true,
    data: {
      status: state.status,
      accountId: state.accountId,
      qrCodeUrl: state.qrCodeUrl,
      messageCount: state.messageCount,
      baseUrl: state.baseUrl,
      startedAt: state.startedAt,
      lastError: state.lastError,
      message: state.message,
    },
  })
})

// Start QR login
app.post("/api/login/start", async (_req, reply) => {
  try {
    if (manager.getState().status === "connected") {
      return reply.send({
        success: true,
        message: "Already logged in",
        data: { status: "connected", accountId: manager.getState().accountId },
      })
    }

    const result = await manager.startLogin()

    // Fire-and-forget: wait for scan in background
    manager.waitForScan(120000).then((scanResult) => {
      if (scanResult.connected) {
        manager.startPolling().catch((err) =>
          console.error("[Sidecar] Auto-polling start failed:", err)
        )
      }
    }).catch((err) => console.error("[Sidecar] Background waitForScan failed:", err))

    return reply.send({
      success: true,
      message: "QR code generated",
      data: {
        qrCodeUrl: result.qrCodeUrl,
        qrImageUrl: result.qrImageUrl,
        sessionKey: result.sessionKey,
        status: "qr_ready",
      },
    })
  } catch (err: any) {
    console.error("[Sidecar]", err); return reply.status(500).send({ success: false, message: "服务器错误" })
  }
})

// Reset current session and start a fresh QR login
app.post("/api/login/rescan", async (_req, reply) => {
  try {
    await manager.resetLogin()
    const result = await manager.startLogin({ force: true })

    // Fire-and-forget: wait for scan in background
    manager.waitForScan(120000).then((scanResult) => {
      if (scanResult.connected) {
        manager.startPolling().catch((err) =>
          console.error("[Sidecar] Auto-polling start failed:", err)
        )
      }
    }).catch((err) => console.error("[Sidecar] Background waitForScan failed:", err))

    return reply.send({
      success: true,
      message: "QR code generated",
      data: {
        qrCodeUrl: result.qrCodeUrl,
        qrImageUrl: result.qrImageUrl,
        sessionKey: result.sessionKey,
        status: "qr_ready",
      },
    })
  } catch (err: any) {
    console.error("[Sidecar]", err); return reply.status(500).send({ success: false, message: "服务器错误" })
  }
})

// Wait for QR scan
app.post("/api/login/wait", async (req, reply) => {
  try {
    const state = manager.getState()
    // Already connected - return immediately
    if (state.status === "connected" && state.accountId) {
      return reply.send({
        success: true,
        message: "Already connected",
        data: {
          connected: true,
          accountId: state.accountId,
          status: "connected",
        },
      })
    }

    const body = req.body as { timeoutMs?: number }
    const result = await manager.waitForScan(body.timeoutMs || 120000)

    if (result.connected) {
      manager.startPolling().catch((err) =>
        console.error("[Sidecar] Polling start failed:", err)
      )
    }

    return reply.send({
      success: result.connected,
      message: result.message,
      data: {
        connected: result.connected,
        accountId: result.accountId,
        status: manager.getState().status,
      },
    })
  } catch (err: any) {
    console.error("[Sidecar]", err); return reply.status(500).send({ success: false, message: "服务器错误" })
  }
})

// Get QR code
app.get("/api/qrcode", async (_req, reply) => {
  const state = manager.getState()
  if (!state.qrCodeUrl) {
    return reply.status(404).send({
      success: false,
      message: "No QR code. Call /api/login/start first.",
    })
  }
  return reply.send({
    success: true,
    data: { qrCodeUrl: state.qrCodeUrl, status: state.status },
  })
})

// Reconnect: restart polling to refresh friend connections
app.post("/api/login/reconnect", async (_req, reply) => {
  try {
    await manager.stopPolling()
    await manager.startPolling()
    return reply.send({ success: true, message: "已重新连接" })
  } catch (err: any) { console.error("[Sidecar]", err); return reply.status(500).send({ success: false, message: "服务器错误" })
  }
})

// Send message
app.post("/api/send", async (req, reply) => {
  try {
    const body = req.body as { toUserId?: string; text?: string; contextToken?: string }
    if (!body.toUserId || !body.text) {
      return reply.status(422).send({
        success: false,
        message: "toUserId and text are required",
      })
    }
    await manager.sendTextMessage(body.toUserId, body.text, body.contextToken)
    return reply.send({ success: true, message: "Sent" })
  } catch (err: any) { console.error("[Sidecar]", err); return reply.status(500).send({ success: false, message: "服务器错误" })
  }
})


// Send voice message (for proactive / system messages)
app.post("/api/send-voice", async (req, reply) => {
  try {
    const body = req.body as { toUserId?: string; audioUrl?: string; text?: string; contextToken?: string }
    if (!body.toUserId || !body.audioUrl) {
      return reply.status(422).send({
        success: false,
        message: "toUserId and audioUrl are required",
      })
    }
    const fullAudioUrl = body.audioUrl.startsWith("http") ? body.audioUrl : sidecarConfig.coreUrl + body.audioUrl
    const audioResp = await fetch(fullAudioUrl, { signal: AbortSignal.timeout(30000) })
    if (!audioResp.ok) throw new Error("Audio download failed: " + audioResp.status)
    const audioBuffer = Buffer.from(await audioResp.arrayBuffer())
    await manager.sendVoiceMessage(body.toUserId, audioBuffer, 7, 0, body.contextToken)
    return reply.send({ success: true, message: "Voice sent" })
  } catch (err: any) {
    console.error("[Sidecar] send-voice error:", err.message)
    return reply.status(500).send({ success: false, message: err.message || "服务器错误" })
  }
})
// ============================================================
// Startup
// ============================================================

try {
  // Try to load saved account and start polling if available
  const hasAccount = manager.loadSavedAccount()

  await app.listen({ host: sidecarConfig.host, port: sidecarConfig.port })

  console.log("")
  console.log("  ========================================")
  console.log("    OpenClaw WeChat Sidecar Server")
  console.log("    Listen:    http://" + sidecarConfig.host + ":" + sidecarConfig.port)
  console.log("    Account:   " + (hasAccount ? manager.getState().accountId : "(not logged in)"))
  console.log("    Core URL:  " + sidecarConfig.coreUrl)
  console.log("  ========================================")
  console.log("")

  // Auto-start polling if already logged in
  if (hasAccount) {
    manager.startPolling().catch((err) =>
      console.error("[Sidecar] Auto-polling failed:", err)
    )
  }
} catch (err) {
  app.log.error(err)
  process.exit(1)
}

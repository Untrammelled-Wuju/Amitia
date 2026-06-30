// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
process.on('unhandledRejection', (reason) => { console.error('[QQ-Sidecar] Unhandled Rejection:', reason) })
import Fastify from "fastify"
import cors from "@fastify/cors"
import { qqSidecarConfig } from "./config.js"
import { QQBotClient } from "./qqbot-client.js"
import type { QQMessage } from "./qqbot-client.js"
import { loadQQBotConfig, saveQQBotConfig } from "./qqbot-persist.js"
import { FileRouter, createDefaultRouter } from "./file-router.js"
import path from "node:path"
import fs from "node:fs"

const app = Fastify({
  logger: { level: process.env.LOG_LEVEL || "info" },
})

await app.register(cors, {
  origin: [/^http:\/\/127\.0\.0\.1:\d+$/, /^http:\/\/localhost:\d+$/],
  methods: ["GET", "POST", "OPTIONS"],
  credentials: false,
})

app.addHook("onSend", async (_req, reply) => {
  reply.header("X-Content-Type-Options", "nosniff")
  reply.header("X-Frame-Options", "DENY")
  void reply.header("X-Powered-By", "")
})

const qq = new QQBotClient()
const fileRouter: FileRouter = createDefaultRouter()

// ============================================================
// Message forwarding
// ============================================================
const msgBuffers = new Map<string, {
  msgs: Array<{ text: string; fromUserId: string; messageId: string; createdAt: number; groupId?: string; isVoice?: boolean; imageUrl?: string; videoUrl?: string; voiceUrl?: string; imageData?: string }>
  timer: ReturnType<typeof setTimeout>
}>()

const processingLocks = new Map<string, Promise<void>>()

qq.onMessage(async (msg: QQMessage) => {
  if (msg.isVoice) {
    const logLineV2 = (msgtxt: string) => { try { fs.appendFileSync("forward-debug.log", new Date().toISOString() + " " + msgtxt + "\n") } catch {} }
    console.log(`[QQ-Sidecar][VOICE-IN] ========== 收到语音消息 ==========`)
    console.log(`[QQ-Sidecar][VOICE-IN] fromUserId=${msg.fromUserId} groupId=${msg.groupId || "私聊"}`)
    console.log(`[QQ-Sidecar][VOICE-IN] messageId=${msg.messageId} text="${msg.text}"`)
    console.log(`[QQ-Sidecar][VOICE-IN] =========================================`)
    logLineV2(`VOICE-IN: fromUserId=${msg.fromUserId} groupId=${msg.groupId || "none"} msgId=${msg.messageId}`)
  }
  const BUFFER_MS = qqSidecarConfig.mergeWindowMs
  const key = msg.groupId || msg.fromUserId
  const existing = msgBuffers.get(key)
  let imageData = ""
  if (msg.imageUrl) {
    try {
      const dl = await qq.downloadImage(msg.imageUrl)
      if (dl) {
        imageData = "data:" + dl.contentType + ";base64," + dl.buffer.toString("base64")
        console.log("[QQ-Sidecar][MSG-DL] len=" + imageData.length)
      }
    } catch {}
  }
  const item = { text: msg.text, fromUserId: msg.fromUserId, messageId: msg.messageId, createdAt: msg.createdAt, groupId: msg.groupId, isVoice: msg.isVoice || false, imageUrl: msg.imageUrl || "", fileUrl: msg.fileUrl || "", fileName: msg.fileName || "", voiceUrl: msg.voiceUrl || "", imageData }

  if (existing) {
    clearTimeout(existing.timer)
    existing.msgs.push(item)
  } else {
    msgBuffers.set(key, { msgs: [item], timer: null as any })
  }

  const entry = msgBuffers.get(key)!
  entry.timer = setTimeout(async () => {
    const prevLock = processingLocks.get(key)
    if (prevLock) {
      try { await prevLock } catch {}
    }
    const flog = (s: string) => { try { fs.appendFileSync("forward-debug.log", new Date().toISOString() + " " + s + "\n") } catch {} }
    let resolveLock: () => void
    const lockPromise = new Promise<void>(r => { resolveLock = r })
    processingLocks.set(key, lockPromise)
    msgBuffers.delete(key)
    const all = entry.msgs
    flog("FWD-START msgs=" + all.length)
    const last = all[all.length - 1]
    const combined = all.map(m => m.text).filter(t => t.length > 0).join("\n")
    const wasVoice = all.some(m => m.isVoice || false)
    if (wasVoice) {
      const logLineV3 = (msgtxt: string) => { try { fs.appendFileSync("forward-debug.log", new Date().toISOString() + " " + msgtxt + "\n") } catch {} }
      logLineV3(`Voice-FWD: userId=${last.fromUserId} groupId=${last.groupId || "none"} msgId=${last.messageId} text=${combined.substring(0, 100)}`)
      console.log(`[QQ-Sidecar][VOICE-FWD] 转发语音到后端: fromUserId=${last.fromUserId} text="${combined.substring(0, 100)}"`)
    }
    let audioBase64 = ""
    if (wasVoice && last.voiceUrl) {
      try {
        const voiceDl = await qq.downloadImage(last.voiceUrl)
        if (voiceDl) {
          audioBase64 = voiceDl.buffer.toString("base64")
          console.log("[QQ-Sidecar][VOICE-DL] 用户语音已下载, size=" + voiceDl.buffer.length)
        }
      } catch {}
    }

    const headers: Record<string, string> = { "Content-Type": "application/json" }
    const t = qqSidecarConfig.bridgeApiToken
    if (t) headers["Authorization"] = "Bearer " + t

    try {
      let imageUrl = ""
      let videoUrl = ""

      const firstFileMsg = all.find(m => (m as any).fileUrl)
      if (firstFileMsg && (firstFileMsg as any).fileUrl) {
        const fileResult = await qq.downloadImage((firstFileMsg as any).fileUrl)
        if (fileResult) {
          console.log("[QQ-Sidecar][FILE-FWD] 文件已下载, size=" + fileResult.buffer.length + " type=" + fileResult.contentType)
          const routeResult = await fileRouter.route({
            buffer: fileResult.buffer,
            fileName: (firstFileMsg as any).fileName || "unknown",
            mimeType: fileResult.contentType,
          })
          if (routeResult) {
            console.log("[QQ-Sidecar][FILE-ROUTE] 文件路由: handler=" + routeResult.handler)
            if (routeResult.handler === "image" && routeResult.data?.base64) {
              imageUrl = "data:" + (routeResult.data.mimeType || fileResult.contentType) + ";base64," + routeResult.data.base64
              console.log("[QQ-Sidecar][FILE-ROUTE] 文件识别为图片, base64Len=" + imageUrl.length)
            } else if (routeResult.handler === "video" && routeResult.data?.base64) {
              videoUrl = "data:" + (routeResult.data.mimeType || fileResult.contentType) + ";base64," + routeResult.data.base64
              console.log("[QQ-Sidecar][FILE-ROUTE] 文件识别为视频, base64Len=" + videoUrl.length)
            } else if (routeResult.handler === "audio" && routeResult.data?.base64) {
              console.log("[QQ-Sidecar][FILE-ROUTE] 文件识别为音频(暂不处理)")
            } else {
              console.log("[QQ-Sidecar][FILE-ROUTE] 未知文件类型, handler=" + routeResult.handler)
            }
          } else {
            console.log("[QQ-Sidecar][FILE-ROUTE] 未找到文件处理器")
          }
        }
      }

      flog("FWD-IMG check")
      const firstImageData = all.find(m => (m as any).imageData)
      if (!imageUrl && firstImageData && (firstImageData as any).imageData) {
        imageUrl = (firstImageData as any).imageData
        flog("FWD-IMG using imageData len=" + imageUrl.length)
        console.log("[QQ-Sidecar][IMAGE-FWD] 使用预下载图片, len=" + imageUrl.length)
      }

      const firstVideoMsg = all.find(m => m.videoUrl)
      if (!videoUrl && firstVideoMsg?.videoUrl) {
        const vidResult = await qq.downloadImage(firstVideoMsg.videoUrl)
        if (vidResult) {
          videoUrl = "data:" + vidResult.contentType + ";base64," + vidResult.buffer.toString("base64")
          console.log("[QQ-Sidecar][VIDEO-FWD] 视频已下载并编码, size=" + vidResult.buffer.length)
        }
      }

      console.log("[QQ-Sidecar][WEBHOOK] text=" + combined.substring(0, 50) + " imageUrlLen=" + (imageUrl ? imageUrl.length : 0) + " videoUrlLen=" + (videoUrl ? videoUrl.length : 0))
      const resp = await fetch(`${qqSidecarConfig.coreUrl}/api/agent/webhook`, {
        method: "POST", headers,
        body: JSON.stringify({
          channel: "qq", accountId: qq.getAccountId() || "qqbot",
          conversationId: "conv-" + last.fromUserId, senderId: last.fromUserId,
          messageId: last.messageId,
          type: wasVoice ? "voice" : "text", text: combined, createdAt: last.createdAt,
          voiceMessage: wasVoice,
          audioBase64: audioBase64,
          imageUrl: imageUrl,
          videoUrl: videoUrl,
          skipTiming: true,
        }),
                signal: AbortSignal.timeout(600000),
      })
      const json = await resp.json() as any
      if (json?.code && json.code !== 200) {
        const errMsg = "AI服务异常 [code=" + json.code + "] " + (json.msg || "")
        console.error("[QQ-Sidecar][WEBHOOK-ERR] " + errMsg)
        try {
          if (last.groupId) {
            await qq.sendGroupMsg(last.groupId, errMsg)
          } else {
            await qq.sendPrivateMsg(last.fromUserId, errMsg)
          }
        } catch {}
        return
      }
      const logLine = (msgtxt: string) => { try { fs.appendFileSync("forward-debug.log", new Date().toISOString() + " " + msgtxt + "\n") } catch {} }
      logLine("Webhook response: " + JSON.stringify(json).substring(0, 300))
      if (json?.data?.outgoingMessage?.text) {
        const reply = json.data.outgoingMessage.text
        logLine("Reply text (" + reply.length + " chars): " + reply.substring(0, 100))

        const forceVoice = json?.data?.outgoingMessage?.forceVoice === true
        const shouldSendVoice = forceVoice || (wasVoice && Math.random() < 0.8)
        logLine("Voice decision: wasVoice=" + wasVoice + " shouldSendVoice=" + shouldSendVoice)

        const audioUrls: string[] = json?.data?.outgoingMessage?.audioUrls || []
        if (shouldSendVoice && audioUrls.length > 0) {
          try {
            logLine("Voice audioUrls: " + audioUrls.length)
            let voiceSent = false
            for (let i = 0; i < audioUrls.length; i++) {
              try {
                const fullAudioUrl = qqSidecarConfig.coreUrl + audioUrls[i]
                const audioResp = await fetch(fullAudioUrl, { signal: AbortSignal.timeout(30000) })
                if (audioResp.ok) {
                  const audioBuffer = Buffer.from(await audioResp.arrayBuffer())
                  logLine("Voice part " + (i+1) + " audio: " + audioBuffer.length + " bytes")
                  const fileInfo = last.groupId
                    ? await qq.uploadGroupMedia(last.groupId, audioBuffer, "voice" + i + ".mp3", 3)
                    : await qq.uploadPrivateMedia(last.fromUserId, audioBuffer, "voice" + i + ".mp3", 3)
                  if (last.groupId) {
                    await qq.sendGroupVoice(last.groupId, fileInfo)
                  } else {
                    await qq.sendPrivateVoice(last.fromUserId, fileInfo)
                  }
                  logLine("Voice part " + (i+1) + " sent OK")
                  voiceSent = true
                } else {
                  logLine("Voice part " + (i+1) + " audio download failed: " + audioResp.status)
                }
              } catch (partErr: any) {
                logLine("Voice part " + (i+1) + " error: " + (partErr?.message || String(partErr)))
              }
              if (i < audioUrls.length - 1) await new Promise(r => setTimeout(r, 800))
            }
            if (voiceSent) return
            logLine("No voice parts sent successfully, falling back to text")
          } catch (ttsErr: any) {
            logLine("Voice send error: " + (ttsErr?.message || String(ttsErr)) + ", falling back to text")
          }
        }

        const parts = reply.split("\n").map((p: string) => p.trim()).filter((p: string) => p.length > 0)
        logLine("Reply parts: " + parts.length)
        for (let i = 0; i < parts.length; i++) {
          const sendTarget = last.groupId ? "group:" + last.groupId : "user:" + last.fromUserId
          logLine("Sending part " + (i+1) + "/" + parts.length + " to " + sendTarget + " text=" + parts[i].substring(0, 50))
          try {
            if (last.groupId) {
              await qq.sendGroupMsg(last.groupId, parts[i])
            } else {
              await qq.sendPrivateMsg(last.fromUserId, parts[i])
            }
            logLine("Part " + (i+1) + " sent OK")
          } catch (sendErr: any) {
            logLine("Send FAILED for part " + (i+1) + ": " + (sendErr?.message || String(sendErr)))
          }
          if (i < parts.length - 1) await new Promise(r => setTimeout(r, 800))
        }
      } else {
        logLine("No outgoingMessage in response. Keys: " + Object.keys(json?.data || {}).join(","))
      }
    } catch (err: any) { 
      try { fs.appendFileSync("forward-debug.log", new Date().toISOString() + " Forward FAILED: " + (err?.message || String(err)) + "\n") } catch {}
    } finally {
      processingLocks.delete(key)
      resolveLock!()
    }
  }, BUFFER_MS)
})

// ============================================================
// HTTP API
// ============================================================

app.post("/api/connect", async (req, reply) => {
  const body = req.body as any
  const appId = body?.appId || qqSidecarConfig.qqbot.appId
  const token = body?.token || qqSidecarConfig.qqbot.token
  const sandbox = body?.sandbox ?? qqSidecarConfig.qqbot.sandbox

  if (!appId || !token) {
    return reply.status(400).send({ error: "appId and token required" })
  }

  console.log(`[HTTP] 收到QQBot连接请求 appId=${appId}`)
  try {
    await qq.connect({ appId, token, sandbox })
    saveQQBotConfig({ appId, token, sandbox })
  } catch (err: any) {
    console.error(`[HTTP] QQBot连接失败:`, err.message)
    return reply.status(500).send({ error: err.message })
  }
  return reply.send({ success: true })
})

app.post("/api/disconnect", async (_req, reply) => {
  qq.disconnect()
  return reply.send({ success: true })
})

app.post("/api/send", async (req, reply) => {
  if (!qq.isOnline()) {
    return reply.status(503).send({ success: false, error: "QQBot未连接" })
  }

  const body = req.body as any
  const toUserId = body?.toUserId
  const text = body?.text
  const groupId = body?.groupId

  if (!toUserId && !groupId) {
    return reply.status(400).send({ success: false, error: "toUserId or groupId required" })
  }
  if (!text) {
    return reply.status(400).send({ success: false, error: "text required" })
  }

  try {
    if (groupId) {
      await qq.sendGroupMsg(groupId, text)
    } else {
      await qq.sendPrivateMsg(toUserId, text)
    }
    console.log(`[HTTP] 消息已发送 to=${toUserId || groupId}`)
    return reply.send({ success: true })
  } catch (err: any) {
    console.error(`[HTTP] 发送失败:`, err.message)
    return reply.status(500).send({ success: false, error: err.message })
  }
})

app.post("/api/send-voice", async (req, reply) => {
  if (!qq.isOnline()) {
    return reply.status(503).send({ success: false, error: "QQBot未连接" })
  }

  const body = req.body as any
  const toUserId = body?.toUserId
  const text = body?.text
  const groupId = body?.groupId

  if (!toUserId && !groupId) {
    return reply.status(400).send({ success: false, error: "toUserId or groupId required" })
  }
  if (!text) {
    return reply.status(400).send({ success: false, error: "text required" })
  }

  try {
    const parts = text.split(String.fromCharCode(10)).map((p: string) => p.trim()).filter((p: string) => p.length > 0).map((p: string) => p.trim()).filter((p: string) => p.length > 0)
    for (let i = 0; i < parts.length; i++) {
      const part = parts[i]
      try {
        const ttsResp = await fetch(`${qqSidecarConfig.coreUrl}/api/tts/synthesize`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ text: part }),
                  signal: AbortSignal.timeout(600000),
        })
        const ttsJson = await ttsResp.json() as any
        const audioUrl = ttsJson?.data?.audioUrl
        if (!audioUrl) throw new Error("TTS returned no audioUrl")
        const fullAudioUrl = qqSidecarConfig.coreUrl + audioUrl
        const audioResp = await fetch(fullAudioUrl, { signal: AbortSignal.timeout(30000) })
        if (!audioResp.ok) throw new Error("audio download failed: " + audioResp.status)
        const audioBuffer = Buffer.from(await audioResp.arrayBuffer())
        const fileInfo = groupId
          ? await qq.uploadGroupMedia(groupId, audioBuffer, "voice" + i + ".mp3", 3)
          : await qq.uploadPrivateMedia(toUserId, audioBuffer, "voice" + i + ".mp3", 3)
        if (groupId) {
          await qq.sendGroupVoice(groupId, fileInfo)
        } else {
          await qq.sendPrivateVoice(toUserId, fileInfo)
        }
      } catch (e: any) {
        console.error(`[HTTP] 语音发送失败 part=${i}:`, e.message)
        try {
          if (groupId) {
            await qq.sendGroupMsg(groupId, part)
          } else {
            await qq.sendPrivateMsg(toUserId, part)
          }
        } catch {}
      }
      if (i < parts.length - 1) await new Promise(r => setTimeout(r, 800))
    }
    return reply.send({ success: true })
  } catch (err: any) {
    console.error(`[HTTP] 语音发送失败:`, err.message)
    return reply.status(500).send({ success: false, error: err.message })
  }
})

app.get("/api/health", async (_req, reply) => {
  return reply.send({ success: true, qqOnline: qq.isOnline() })
})

app.get("/api/status", async (_req, reply) => {
  return reply.send({
    success: true,
    data: {
      qqOnline: qq.isOnline(),
      status: qq.getStatus(),
      accountId: qq.getAccountId(),
      error: qq.getLastError(),
      messageCount: qq.getMessageCount(),
    },
  })
})

// ============================================================
// Startup
// ============================================================

try {
  await app.listen({ host: qqSidecarConfig.host, port: qqSidecarConfig.port })

  console.log("")
  console.log("  ========================================")
  console.log("    QQ Sidecar (QQBot WebSocket) v2.3")
  console.log("    HTTP:    http://" + qqSidecarConfig.host + ":" + qqSidecarConfig.port)
  console.log("  ========================================")
  console.log("")

  const savedConfig = loadQQBotConfig()
  if (savedConfig) {
    console.log("[QQ-Sidecar] 检测到持久化凭证，自动连接 QQBot...")
    qq.connect({ appId: savedConfig.appId, token: savedConfig.token, sandbox: savedConfig.sandbox })
  } else if (qqSidecarConfig.qqbot.appId && qqSidecarConfig.qqbot.token) {
    console.log("[QQ-Sidecar] 使用环境变量自动连接 QQBot...")
    const cfg = { appId: qqSidecarConfig.qqbot.appId, token: qqSidecarConfig.qqbot.token, sandbox: qqSidecarConfig.qqbot.sandbox }
    qq.connect(cfg)
    saveQQBotConfig(cfg)
  } else {
    console.log("[QQ-Sidecar] 未配置凭证，等待HTTP连接请求...")
  }
} catch (err) {
  app.log.error(err)
  process.exit(1)
}
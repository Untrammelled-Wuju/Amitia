// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
process.on("unhandledRejection", (reason) => {
  console.error("[QQ-Sidecar] Unhandled Rejection:", reason)
})

import Fastify from "fastify"
import cors from "@fastify/cors"
import path from "node:path"
import fs from "node:fs"
import { coreRequestHeaders, qqSidecarConfig } from "./config.js"
import { QQBotClient, type QQMessage } from "./qqbot-client.js"
import { clearQQBotConfig, loadQQBotConfig, saveQQBotConfig } from "./qqbot-persist.js"
import { createDefaultRouter, type FileRouter } from "./file-router.js"
import { buildQQInboundPayload, type BufferedQQMessage } from "./inbound.js"

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
  reply.header("X-Powered-By", "")
})

const qq = new QQBotClient()
const fileRouter: FileRouter = createDefaultRouter()
const msgBuffers = new Map<string, { msgs: BufferedQQMessage[]; timer: ReturnType<typeof setTimeout> | null }>()
const processingLocks = new Map<string, Promise<void>>()
const knownGroups = new Set<string>()

function resolveSendTarget(body: any): { peerId: string; groupId: string; toUserId: string } {
  const explicitGroup = String(body?.groupId || "").trim()
  const peerId = String(body?.peerId || body?.toUserId || explicitGroup || "").trim()
  if (explicitGroup) return { peerId, groupId: explicitGroup, toUserId: "" }
  if (peerId && knownGroups.has(peerId)) return { peerId, groupId: peerId, toUserId: "" }
  return { peerId, groupId: "", toUserId: peerId }
}

function appendForwardLog(message: string): void {
  try {
    fs.appendFileSync("forward-debug.log", `${new Date().toISOString()} ${message}\n`)
  } catch (error) {
  console.error("[original-error]", error);
}
}

async function downloadDataUrl(url: string): Promise<{ dataUrl: string; mime: string }> {
  if (!url) return { dataUrl: "", mime: "" }
  try {
    const result = await qq.downloadImage(url)
    if (!result || result.buffer.length === 0) return { dataUrl: "", mime: "" }
    return {
      dataUrl: `data:${result.contentType};base64,${result.buffer.toString("base64")}`,
      mime: result.contentType,
    }
  } catch (error) {
  console.error("[original-error]", error);
return { dataUrl: "", mime: "" }
  }
}

qq.onMessage(async (msg: QQMessage) => {
  const key = msg.groupId || msg.fromUserId
  if (msg.groupId) knownGroups.add(msg.groupId)
  let imageData = ""
  if (msg.imageUrl) {
    imageData = (await downloadDataUrl(msg.imageUrl)).dataUrl
  }
  const item: BufferedQQMessage = {
    ...msg,
    imageData,
  }
  const existing = msgBuffers.get(key)
  if (existing) {
    clearTimeout(existing.timer!)
    existing.msgs.push(item)
  } else {
    msgBuffers.set(key, { msgs: [item], timer: null })
  }
  const entry = msgBuffers.get(key)!
  entry.timer = setTimeout(async () => {
    const previous = processingLocks.get(key)
    if (previous) {
      try {
        await previous
      } catch (error) {
  console.error("[original-error]", error);
}
    }
    let release = () => {}
    const lock = new Promise<void>((resolve) => {
      release = resolve
    })
    processingLocks.set(key, lock)
    msgBuffers.delete(key)
    const all = entry.msgs
    const last = all[all.length - 1]
    try {
      let imageUrl = ""
      let videoUrl = ""
      let audioUrl = ""
      const firstFile = all.find((message) => message.fileUrl)
      if (firstFile?.fileUrl) {
        const file = await qq.downloadImage(firstFile.fileUrl)
        if (file) {
          const route = await fileRouter.route({
            buffer: file.buffer,
            fileName: firstFile.fileName || "attachment",
            mimeType: file.contentType,
          })
          const base64 = route?.data?.base64 || ""
          const mime = route?.data?.mimeType || file.contentType
          if (base64 && route?.handler === "image") imageUrl = `data:${mime};base64,${base64}`
          if (base64 && route?.handler === "video") videoUrl = `data:${mime};base64,${base64}`
          if (base64 && route?.handler === "audio") audioUrl = `data:${mime};base64,${base64}`
        }
      }
      const firstImage = all.find((message) => message.imageData)
      if (!imageUrl && firstImage?.imageData) imageUrl = firstImage.imageData
      const firstVideo = all.find((message) => message.videoUrl)
      if (!videoUrl && firstVideo?.videoUrl) {
        videoUrl = (await downloadDataUrl(firstVideo.videoUrl)).dataUrl
      }
      const firstVoice = all.find((message) => message.isVoice && message.voiceUrl)
      if (!audioUrl && firstVoice?.voiceUrl) {
        audioUrl = (await downloadDataUrl(firstVoice.voiceUrl)).dataUrl
      }

      const payload = buildQQInboundPayload({
        messages: all,
        accountId: qq.getAccountId() || "qqbot",
        imageUrl,
        videoUrl,
        audioUrl,
      })

      appendForwardLog(`FWD channel=${payload.channelId} peer=${payload.peerId} type=${payload.contentType} image=${imageUrl.length} video=${videoUrl.length} audio=${audioUrl.length}`)
      const response = await fetch(`${qqSidecarConfig.coreUrl}/api/channels/inbound`, {
        method: "POST",
        headers: coreRequestHeaders(),
        body: JSON.stringify(payload),
        signal: AbortSignal.timeout(600000),
      })
      const json = await response.json().catch((error) => {
        console.error("[original-error]", error)
        return {}
      })
      if (!response.ok || (json?.code && json.code !== 200)) {
        const error = `渠道消息入站失败 [status=${response.status} code=${json?.code || 0}] ${json?.msg || ""}`
        appendForwardLog(error)
        if (last.groupId) await qq.sendGroupMsg(last.groupId, error)
        else await qq.sendPrivateMsg(last.fromUserId, error)
      }
    } catch (error: any) {
      appendForwardLog(`FWD FAILED ${error?.message || String(error)}`)
    } finally {
      processingLocks.delete(key)
      release()
    }
  }, qqSidecarConfig.mergeWindowMs)
})

app.post("/api/connect", async (req, reply) => {
  const body = req.body as any
  const appId = String(body?.appId || qqSidecarConfig.qqbot.appId || "").trim()
  const token = String(body?.token || qqSidecarConfig.qqbot.token || "").trim()
  const sandbox = body?.sandbox ?? qqSidecarConfig.qqbot.sandbox
  if (!appId || !token) return reply.status(400).send({ error: "appId and token required" })
  try {
    await qq.connect({ appId, token, sandbox: Boolean(sandbox) })
    saveQQBotConfig({ appId, token, sandbox: Boolean(sandbox) })
    return reply.send({ success: true })
  } catch (error: any) {
    return reply.status(500).send({ error: error?.message || String(error) })
  }
})

app.post("/api/disconnect", async (_req, reply) => {
  qq.disconnect()
  clearQQBotConfig()
  return reply.send({ success: true })
})

app.post("/api/send", async (req, reply) => {
  if (!qq.isOnline()) return reply.status(503).send({ success: false, error: "QQBot未连接" })
  const body = req.body as any
  const { groupId, toUserId } = resolveSendTarget(body)
  const text = String(body?.text || "").trim()
  if (!toUserId && !groupId) return reply.status(400).send({ success: false, error: "toUserId or groupId required" })
  if (!text) return reply.status(400).send({ success: false, error: "text required" })
  try {
    if (groupId) await qq.sendGroupMsg(groupId, text)
    else await qq.sendPrivateMsg(toUserId, text)
    return reply.send({ success: true })
  } catch (error: any) {
    return reply.status(500).send({ success: false, error: error?.message || String(error) })
  }
})

app.post("/api/send-voice", async (req, reply) => {
  if (!qq.isOnline()) return reply.status(503).send({ success: false, error: "QQBot未连接" })
  const body = req.body as any
  const { groupId, toUserId } = resolveSendTarget(body)
  const text = String(body?.text || "").trim()
  if (!toUserId && !groupId) return reply.status(400).send({ success: false, error: "toUserId or groupId required" })
  if (!text) return reply.status(400).send({ success: false, error: "text required" })
  try {
    const parts = text.split(/\r?\n/).map((part) => part.trim()).filter(Boolean)
    for (let index = 0; index < parts.length; index++) {
      const part = parts[index]
      try {
        const ttsResp = await fetch(`${qqSidecarConfig.coreUrl}/api/tts/synthesize`, {
          method: "POST",
          headers: coreRequestHeaders(),
          body: JSON.stringify({ text: part }),
          signal: AbortSignal.timeout(600000),
        })
        const ttsJson = await ttsResp.json() as any
        const audioPath = String(ttsJson?.data?.audioUrl || "")
        if (!audioPath) throw new Error("TTS returned no audioUrl")
        const audioResp = await fetch(qqSidecarConfig.coreUrl + audioPath, { signal: AbortSignal.timeout(30000) })
        if (!audioResp.ok) throw new Error("audio download failed: " + audioResp.status)
        const audioBuffer = Buffer.from(await audioResp.arrayBuffer())
        const fileInfo = groupId
          ? await qq.uploadGroupMedia(groupId, audioBuffer, `voice${index}.mp3`, 3)
          : await qq.uploadPrivateMedia(toUserId, audioBuffer, `voice${index}.mp3`, 3)
        if (groupId) await qq.sendGroupVoice(groupId, fileInfo)
        else await qq.sendPrivateVoice(toUserId, fileInfo)
      } catch (error) {
  console.error("[original-error]", error);
if (groupId) await qq.sendGroupMsg(groupId, part)
        else await qq.sendPrivateMsg(toUserId, part)
      }
      if (index < parts.length - 1) await new Promise((resolve) => setTimeout(resolve, 800))
    }
    return reply.send({ success: true })
  } catch (error: any) {
    return reply.status(500).send({ success: false, error: error?.message || String(error) })
  }
})

app.post("/api/send-image", async (req, reply) => {
  if (!qq.isOnline()) return reply.status(503).send({ success: false, error: "QQBot未连接" })
  const body = req.body as any
  const { groupId, toUserId } = resolveSendTarget(body)
  if (!toUserId && !groupId) return reply.status(400).send({ success: false, error: "toUserId or groupId required" })
  const candidates = [body?.assetUrl, body?.fallbackUrl].map((value) => String(value || "").trim()).filter(Boolean)
  try {
    let buffer: Buffer | null = null
    let fileName = "image.png"
    for (const candidate of candidates) {
      const url = /^https?:/i.test(candidate) ? candidate : qqSidecarConfig.coreUrl + candidate
      try {
        const response = await fetch(url, { signal: AbortSignal.timeout(30000) })
        if (!response.ok) continue
        buffer = Buffer.from(await response.arrayBuffer())
        fileName = path.basename(new URL(url).pathname) || fileName
        break
      } catch (error) {
  console.error("[original-error]", error);
}
    }
    if (!buffer) throw new Error("图片资源下载失败")
    const fileInfo = groupId
      ? await qq.uploadGroupMedia(groupId, buffer, fileName, 1)
      : await qq.uploadPrivateMedia(toUserId, buffer, fileName, 1)
    if (groupId) await qq.sendGroupImage(groupId, fileInfo)
    else await qq.sendPrivateImage(toUserId, fileInfo)
    return reply.send({ success: true })
  } catch (error: any) {
    return reply.status(500).send({ success: false, error: error?.message || String(error) })
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
      replyCount: qq.getReplyCount(),
      startedAt: qq.getStartedAt(),
    },
  })
})

app.get("/api/config", async (_req, reply) => {
  const saved = loadQQBotConfig()
  return reply.send(saved || { appId: "", token: "", sandbox: false })
})

try {
  await app.listen({ host: qqSidecarConfig.host, port: qqSidecarConfig.port })
  const saved = loadQQBotConfig()
  if (saved) {
    void qq.connect({ appId: saved.appId, token: saved.token, sandbox: saved.sandbox })
  } else if (qqSidecarConfig.qqbot.appId && qqSidecarConfig.qqbot.token) {
    const config = {
      appId: qqSidecarConfig.qqbot.appId,
      token: qqSidecarConfig.qqbot.token,
      sandbox: qqSidecarConfig.qqbot.sandbox,
    }
    saveQQBotConfig(config)
    void qq.connect(config)
  }
} catch (error) {
  app.log.error(error)
  process.exit(1)
}

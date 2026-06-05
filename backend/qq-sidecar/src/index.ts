process.on('unhandledRejection', (reason) => { console.error('[QQ-Sidecar] Unhandled Rejection:', reason) })
import Fastify from "fastify"
import cors from "@fastify/cors"
import { qqSidecarConfig } from "./config.js"
import { QQBotClient } from "./qqbot-client.js"
import type { QQMessage } from "./qqbot-client.js"
import { loadQQBotConfig, saveQQBotConfig } from "./qqbot-persist.js"
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

// ============================================================
// Message forwarding
// ============================================================
const msgBuffers = new Map<string, {
  msgs: Array<{ text: string; fromUserId: string; messageId: string; createdAt: number; groupId?: string }>
  timer: ReturnType<typeof setTimeout>
}>()

qq.onMessage(async (msg: QQMessage) => {
  const BUFFER_MS = 5000
  const key = msg.groupId || msg.fromUserId
  const existing = msgBuffers.get(key)
  const item = { text: msg.text, fromUserId: msg.fromUserId, messageId: msg.messageId, createdAt: msg.createdAt, groupId: msg.groupId }

  if (existing) {
    clearTimeout(existing.timer)
    existing.msgs.push(item)
  } else {
    msgBuffers.set(key, { msgs: [item], timer: null as any })
  }

  const entry = msgBuffers.get(key)!
  entry.timer = setTimeout(async () => {
    msgBuffers.delete(key)
    const all = entry.msgs
    const last = all[all.length - 1]
    const combined = all.map(m => m.text).join("\n")

    const headers: Record<string, string> = { "Content-Type": "application/json" }
    const t = qqSidecarConfig.bridgeApiToken
    if (t) headers["Authorization"] = "Bearer " + t

    try {
      const resp = await fetch(`${qqSidecarConfig.coreUrl}/api/agent/webhook`, {
        method: "POST", headers,
        body: JSON.stringify({
          channel: "qq", accountId: qq.getAccountId() || "qqbot",
          conversationId: "conv-" + last.fromUserId, senderId: last.fromUserId,
          messageId: last.messageId,
          type: "text", text: combined, createdAt: last.createdAt,
          skipTiming: true,
        }),
        signal: AbortSignal.timeout(60000),
      })
      const json = await resp.json() as any
      const logLine = (msg: string) => { try { fs.appendFileSync("forward-debug.log", new Date().toISOString() + " " + msg + "\n") } catch {} }
      logLine("Webhook response: " + JSON.stringify(json).substring(0, 300))
      if (json?.data?.outgoingMessage?.text) {
        const reply = json.data.outgoingMessage.text
        logLine("Reply text (" + reply.length + " chars): " + reply.substring(0, 100))
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

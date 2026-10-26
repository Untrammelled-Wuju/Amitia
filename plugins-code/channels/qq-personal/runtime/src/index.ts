process.on("unhandledRejection", (reason) => {
  console.error("[QQ-Personal] unhandled rejection", reason)
})

import path from "node:path"
import Fastify from "fastify"
import cors from "@fastify/cors"
import { OneBotBridge } from "./onebot.js"
import { dataFilePath, loadConfig, normalizeConfig, saveConfig, type QQPersonalConfig } from "./config.js"
import { parseOneBotMessage, shouldHandleMessage, applyPrefix, type ParsedQQMessage } from "./message.js"
import { buildInboundPayload, forwardInbound } from "./inbound.js"
import { changeWebuiPassword, evaluatePassword, readSnowLumaInfo } from "./snowluma-info.js"
import { CredentialStore } from "./credential-store.js"
import { PeerStore } from "./peer-store.js"
import { authorizeServiceRequest } from "./service-auth.js"
import { downloadMedia, sendMedia, synthesizeVoice } from "./outbound.js"
import { IdempotencyStore } from "./idempotency.js"

const env = process.env
const app = Fastify({ logger: { level: env.LOG_LEVEL || "info" } })
await app.register(cors, {
  origin: [/^http:\/\/127\.0\.0\.1:\d+$/, /^http:\/\/localhost:\d+$/],
  methods: ["GET", "POST", "OPTIONS"],
  credentials: false,
})

let config = loadConfig()
let bridge: OneBotBridge
const peers = new PeerStore()
const outboundRequests = new IdempotencyStore()
const credentialStore = new CredentialStore(path.dirname(dataFilePath("qq-personal-config.json")))
let lastError = ""
let messageCount = 0
let replyCount = 0

const inboundConfig = {
  coreUrl: env.AMITIA_CORE_URL || env.CORE_URL || "http://127.0.0.1:18899",
  extensionId: env.AMITIA_EXTENSION_ID || "com.amitia/channel-qq-personal",
  moduleId: env.AMITIA_MODULE_ID || "qq-personal-service",
  channelId: "qq_personal",
  serviceAuthToken: env.AMITIA_SERVICE_AUTH_TOKEN || env.BRIDGE_API_TOKEN || "",
  localToken: env.AMITIA_LOCAL_TOKEN || "",
}

const serviceIdentity = {
  extensionId: inboundConfig.extensionId,
  moduleId: inboundConfig.moduleId,
  token: inboundConfig.serviceAuthToken,
}

app.addHook("onRequest", async (request, reply) => {
  const pathname = request.url.split("?")[0]
  if (request.method === "OPTIONS" || pathname === "/api/health") return
  if (!authorizeServiceRequest(request.headers, serviceIdentity)) {
    return reply.status(401).send({ success: false, error: "宿主服务凭据无效" })
  }
})

app.addHook("onSend", async (request, reply, payload) => {
  if (request.url.startsWith("/api/")) reply.header("Cache-Control", "no-store")
  return payload
})

bridge = new OneBotBridge(() => config)

const conversationTitles = new Map<string, { value: string; expiresAt: number }>()

async function resolveConversationTitle(message: ParsedQQMessage): Promise<string> {
  const isGroup = message.messageType === "group"
  const key = isGroup ? `group:${message.groupId}` : `private:${message.senderId}`
  const cached = conversationTitles.get(key)
  const now = Date.now()
  if (cached && cached.expiresAt > now) return cached.value
  const fallback = isGroup
    ? `QQ 群 ${message.groupId || "未知"}`
    : (message.senderName || `QQ 用户 ${message.senderId || "未知"}`)
  let value = fallback
  try {
    if (isGroup && message.groupId) {
      const info = await bridge.callAction("get_group_info", { group_id: message.groupId }) as Record<string, unknown>
      value = String(info?.group_name || "").trim() || fallback
    } else if (message.senderId) {
      const info = await bridge.callAction("get_stranger_info", { user_id: message.senderId }) as Record<string, unknown>
      value = String(info?.card || info?.nickname || "").trim() || fallback
    }
  } catch {
    value = fallback
  }
  conversationTitles.set(key, { value, expiresAt: now + 10 * 60 * 1000 })
  return value
}

bridge.onEvent(async (event) => {
  try {
    const parsed = await parseOneBotMessage(bridge, event)
    if (!shouldHandleMessage(parsed, event, config)) return
    const normalized = applyPrefix(parsed, config)
    peers.set(normalized.messageType === "group" ? normalized.groupId : normalized.senderId, normalized.messageType)
    const payload = buildInboundPayload(normalized, await resolveConversationTitle(normalized))
    await forwardInbound(inboundConfig, payload)
    messageCount++
    lastError = ""
  } catch (error) {
    lastError = error instanceof Error ? error.message : String(error)
    app.log.error({ error: lastError }, "QQ 个人号消息处理失败")
  }
})

app.server.on("upgrade", (request, socket, head) => {
  bridge.handleUpgrade(request, socket, head)
})

async function publicConfig() {
  const hostForDocker = process.platform === "darwin" || process.platform === "win32"
    ? "host.docker.internal"
    : "127.0.0.1"
  const snowluma = await readSnowLumaInfo({
    containerName: config.containerName,
    webuiPort: config.webuiPort,
    novncPort: config.novncPort,
    dockerBin: config.dockerBin,
  }, credentialStore.read())
  return {
    enabled: config.enabled,
    listenHost: config.listenHost,
    listenPort: config.listenPort,
    websocketPath: config.websocketPath,
    reverseWebSocketUrl: `ws://${hostForDocker}:${config.listenPort}${config.websocketPath}`,
    localWebSocketUrl: `ws://127.0.0.1:${config.listenPort}${config.websocketPath}`,
    token: config.token,
    groupTrigger: config.groupTrigger,
    triggerPrefix: config.triggerPrefix,
    webuiPort: config.webuiPort,
    novncPort: config.novncPort,
    containerName: config.containerName,
    dockerBin: config.dockerBin,
    webuiUrl: snowluma.webuiUrl,
    novncUrl: snowluma.novncUrl,
    webuiUser: snowluma.webuiUser,
    webuiPassword: snowluma.webuiPassword,
    webuiPasswordState: snowluma.webuiPasswordState,
    novncPassword: snowluma.novncPassword,
    snowlumaError: snowluma.error,
  }
}

function statusPayload() {
  const status = bridge.status()
  return {
    data: {
      provider: "snowluma",
      status: status.connected ? "online" : config.enabled ? "waiting" : "disconnected",
      running: true,
      connected: status.connected,
      qqOnline: status.connected,
      accountId: status.accountIds[0] || "",
      accountIds: status.accountIds,
      clients: status.clients,
      listenHost: config.listenHost,
      listenPort: config.listenPort,
      websocketPath: config.websocketPath,
      startedAt: status.connectedAt,
      lastSeenAt: status.lastSeenAt,
      messageCount,
      replyCount,
      lastError,
    },
  }
}

function resolveTarget(body: Record<string, unknown>): { kind: "group" | "private"; id: string } {
  const groupId = String(body.groupId || "").trim()
  if (groupId) return { kind: "group", id: groupId }
  const peerId = String(body.peerId || body.toUserId || "").trim()
  return { kind: peers.get(peerId) || "private", id: peerId }
}

function idempotencyKey(headers: Record<string, unknown>): string {
  const value = headers["idempotency-key"]
  if (Array.isArray(value)) return String(value[0] || "").trim()
  return String(value || "").trim()
}

function deliveryFingerprint(body: Record<string, unknown>, target: { kind: string; id: string }): string {
  return outboundRequests.fingerprint({
    target,
    text: String(body.text || "").trim(),
    assetUrl: String(body.assetUrl || body.fallbackUrl || "").trim(),
    altText: String(body.altText || body.fileName || "").trim(),
    mimeType: String(body.mimeType || "").trim(),
  })
}

app.get("/api/health", async () => ({ success: true, bridgeReady: true, qqOnline: bridge.status().connected }))
app.get("/api/status", async () => statusPayload())
app.get("/api/config", async () => publicConfig())

app.post("/api/action", async (request, reply) => {
  const body = (request.body || {}) as Record<string, unknown>
  const action = String(body.action || "").trim()
  const input = (body.input && typeof body.input === "object" && !Array.isArray(body.input)
    ? body.input
    : {}) as Record<string, unknown>
  if (action === "change-webui-password") {
    const oldPassword = String(input.oldPassword || "").trim()
    const newPassword = String(input.newPassword || "")
    if (!oldPassword) return reply.send({ success: false, error: "请填写当前密码" })
    const evaluation = evaluatePassword(newPassword)
    if (!evaluation.valid) {
      const failed = evaluation.rules.filter((rule) => !rule.ok).map((rule) => rule.label)
      return reply.send({ success: false, error: `新密码不符合要求：${failed.join("、")}` })
    }
    if (oldPassword === newPassword) return reply.send({ success: false, error: "新密码不能与当前密码相同" })
    const current = await readSnowLumaInfo({
      containerName: config.containerName,
      webuiPort: config.webuiPort,
      novncPort: config.novncPort,
      dockerBin: config.dockerBin,
    }, credentialStore.read())
    const changed = await changeWebuiPassword({ webuiPort: config.webuiPort }, oldPassword, newPassword, current.webuiUser)
    if (!changed.ok) return reply.send({ success: false, error: changed.message })
    credentialStore.write(newPassword)
    return reply.send({ success: true, data: await publicConfig() })
  }
  if (action === "record-webui-password") {
    const password = String(input.password || "").trim()
    if (!password) return reply.send({ success: false, error: "请填写要记录的密码" })
    credentialStore.write(password)
    return reply.send({ success: true, data: await publicConfig() })
  }
  if (action === "clear-webui-password") {
    credentialStore.clear()
    return reply.send({ success: true, data: await publicConfig() })
  }
  return reply.send({ success: false, error: `未知操作：${action || "(空)"}` })
})

app.post("/api/messages", async () => ({ success: true, data: { messages: [], total: 0 } }))
app.post("/api/logs", async () => ({ success: true, data: { items: [], total: 0 } }))
app.post("/api/peers", async (request) => {
  const body = (request.body || {}) as Record<string, unknown>
  const limit = Number(body.limit || 200)
  const offset = Number(body.offset || 0)
  const items = peers.list(Number.isFinite(limit) ? limit : 200, Number.isFinite(offset) ? offset : 0)
  return { success: true, data: { items, total: items.length } }
})

app.post("/api/connect", async (request, reply) => {
  const body = (request.body || {}) as Partial<QQPersonalConfig>
  const next = normalizeConfig({ ...config, ...body, enabled: true }, config)
  if (next.listenPort !== config.listenPort) {
    return reply.status(400).send({ success: false, error: "修改监听端口后需要重启插件" })
  }
  if (!next.token) return reply.status(400).send({ success: false, error: "OneBot Token 不能为空" })
  if (next.token !== config.token) bridge.disconnectClients()
  config = next
  saveConfig(config)
  return reply.send({ success: true, data: await publicConfig() })
})

app.post("/api/disconnect", async (_request, reply) => {
  config = { ...config, enabled: false }
  saveConfig(config)
  bridge.disconnectClients()
  return reply.send({ success: true })
})

app.post("/api/send", async (request, reply) => {
  const body = (request.body || {}) as Record<string, unknown>
  const target = resolveTarget(body)
  const text = String(body.text || "").trim()
  if (!target.id || !text) return reply.status(400).send({ success: false, error: "目标或文本不能为空" })
  try {
    const data = await outboundRequests.execute(
      idempotencyKey(request.headers),
      deliveryFingerprint(body, target),
      async () => {
        await bridge.sendMessage(target.kind, target.id, [{ type: "text", data: { text } }])
        replyCount++
        return { delivered: "text" }
      },
    )
    return reply.send({ success: true, data })
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error)
    return reply.status(message.includes("幂等键") ? 409 : 503).send({ success: false, error: message })
  }
})

app.post("/api/send-image", async (request, reply) => {
  const body = (request.body || {}) as Record<string, unknown>
  const target = resolveTarget(body)
  const candidate = String(body.assetUrl || body.fallbackUrl || "").trim()
  if (!target.id || !candidate) return reply.status(400).send({ success: false, error: "目标或图片资源不能为空" })
  try {
    const data = await outboundRequests.execute(
      idempotencyKey(request.headers),
      deliveryFingerprint(body, target),
      async () => {
        const media = await downloadMedia(inboundConfig, candidate)
        const result = await sendMedia(bridge, target, "image", media)
        replyCount++
        return { delivered: "image", result }
      },
    )
    return reply.send({ success: true, data })
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error)
    return reply.status(message.includes("幂等键") ? 409 : 503).send({ success: false, error: message })
  }
})

app.post("/api/send-voice", async (request, reply) => {
  const body = (request.body || {}) as Record<string, unknown>
  const target = resolveTarget(body)
  const text = String(body.text || "").trim()
  const assetUrl = String(body.assetUrl || "").trim()
  if (!target.id || (!text && !assetUrl)) return reply.status(400).send({ success: false, error: "目标或语音内容不能为空" })
  try {
    const data = await outboundRequests.execute(
      idempotencyKey(request.headers),
      deliveryFingerprint(body, target),
      async () => {
        if (assetUrl) {
          const media = await downloadMedia(inboundConfig, assetUrl)
          await sendMedia(bridge, target, "record", media)
          replyCount++
          return { delivered: "voice", items: [{ kind: "voice" }] }
        }
        const parts = text.split(/\r?\n/).map((part) => part.trim()).filter(Boolean)
        const items: Array<Record<string, unknown>> = []
        for (const part of parts) {
          try {
            const media = await synthesizeVoice(inboundConfig, part)
            await sendMedia(bridge, target, "record", media)
            items.push({ kind: "voice" })
          } catch (error) {
            await bridge.sendMessage(target.kind, target.id, [{ type: "text", data: { text: part } }])
            items.push({
              kind: "text",
              fallback: true,
              error: error instanceof Error ? error.message : String(error),
            })
          }
          replyCount++
        }
        return { delivered: "voice", items }
      }
    )
    return reply.send({ success: true, data })
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error)
    return reply.status(message.includes("幂等键") ? 409 : 503).send({ success: false, error: message })
  }
})

app.post("/api/send-video", async (request, reply) => {
  const body = (request.body || {}) as Record<string, unknown>
  const target = resolveTarget(body)
  const candidate = String(body.assetUrl || body.fallbackUrl || "").trim()
  if (!target.id || !candidate) return reply.status(400).send({ success: false, error: "目标或视频资源不能为空" })
  try {
    const data = await outboundRequests.execute(
      idempotencyKey(request.headers),
      deliveryFingerprint(body, target),
      async () => {
        const media = await downloadMedia(inboundConfig, candidate)
        const result = await sendMedia(bridge, target, "video", media)
        replyCount++
        return { delivered: "video", result }
      },
    )
    return reply.send({ success: true, data })
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error)
    return reply.status(message.includes("幂等键") ? 409 : 503).send({ success: false, error: message })
  }
})

app.post("/api/send-file", async (request, reply) => {
  const body = (request.body || {}) as Record<string, unknown>
  const target = resolveTarget(body)
  const candidate = String(body.assetUrl || body.fallbackUrl || "").trim()
  const fileName = String(body.altText || body.fileName || "文件").trim()
  if (!target.id || !candidate) return reply.status(400).send({ success: false, error: "目标或文件资源不能为空" })
  try {
    const data = await outboundRequests.execute(
      idempotencyKey(request.headers),
      deliveryFingerprint(body, target),
      async () => {
        const media = await downloadMedia(inboundConfig, candidate)
        const result = await sendMedia(bridge, target, "file", media, fileName)
        replyCount++
        return { delivered: "file", result }
      },
    )
    return reply.send({ success: true, data })
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error)
    return reply.status(message.includes("幂等键") ? 409 : 503).send({ success: false, error: message })
  }
})

try {
  await app.listen({ host: config.listenHost, port: config.listenPort })
  app.log.info({
    host: config.listenHost,
    port: config.listenPort,
    path: config.websocketPath,
  }, "QQ 个人号 OneBot 反向 WebSocket 已启动")
} catch (error) {
  app.log.error(error)
  process.exit(1)
}

const shutdown = async () => {
  bridge.close()
  await app.close()
}

process.on("SIGINT", () => { void shutdown() })
process.on("SIGTERM", () => { void shutdown() })

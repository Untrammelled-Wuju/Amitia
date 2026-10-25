import type { OneBotEvent } from "./onebot.js"
import type { OneBotBridge } from "./onebot.js"
import type { QQPersonalConfig } from "./config.js"

interface Segment {
  type?: string
  data?: Record<string, unknown>
}

export interface ParsedQQMessage {
  selfId: string
  senderId: string
  messageId: string
  messageType: "group" | "private"
  groupId: string
  content: string
  imageUrl: string
  videoUrl: string
  audioUrl: string
  fileUrl: string
  fileName: string
  replyToMessageId: string
  mentionedSelf: boolean
  toMe: boolean
  createdAt: number
}

function httpUrl(value: unknown): string {
  const text = String(value || "").trim()
  return /^(https?:|data:)/i.test(text) ? text : ""
}

function decodeCQValue(value: string): string {
  return value
    .replace(/&#91;/g, "[")
    .replace(/&#93;/g, "]")
    .replace(/&#44;/g, ",")
    .replace(/&amp;/g, "&")
}

function cqSegments(raw: string): Segment[] {
  if (!raw.includes("[CQ:")) return [{ type: "text", data: { text: decodeCQValue(raw) } }]
  const segments: Segment[] = []
  const pattern = /\[CQ:([a-zA-Z0-9_]+)((?:,[^\]]*)?)\]/g
  let cursor = 0
  for (const match of raw.matchAll(pattern)) {
    const index = match.index ?? 0
    if (index > cursor) {
      segments.push({ type: "text", data: { text: decodeCQValue(raw.slice(cursor, index)) } })
    }
    const data: Record<string, unknown> = {}
    for (const field of (match[2] || "").replace(/^,/, "").split(",")) {
      if (!field) continue
      const separator = field.indexOf("=")
      if (separator < 0) continue
      data[field.slice(0, separator)] = decodeCQValue(field.slice(separator + 1))
    }
    segments.push({ type: match[1], data })
    cursor = index + match[0].length
  }
  if (cursor < raw.length) {
    segments.push({ type: "text", data: { text: decodeCQValue(raw.slice(cursor)) } })
  }
  return segments
}

function messageSegments(event: OneBotEvent): Segment[] {
  if (Array.isArray(event.message)) return event.message as Segment[]
  if (typeof event.message === "string") return cqSegments(event.message)
  if (typeof event.raw_message === "string" && event.raw_message) return cqSegments(event.raw_message)
  return []
}

function marker(type: string): string {
  return {
    face: "[表情]",
    mface: "[商城表情]",
    json: "[卡片消息]",
    xml: "[卡片消息]",
    markdown: "[Markdown 消息]",
    forward: "[合并转发]",
    node: "[合并转发]",
    location: "[位置]",
    music: "[音乐]",
    contact: "[名片]",
    shake: "[窗口抖动]",
    poke: "[戳一戳]",
    dice: "[骰子]",
    rps: "[猜拳]",
  }[type] || `[${type || "未知消息"}]`
}

async function resolveUrl(
  bridge: OneBotBridge,
  event: OneBotEvent,
  type: string,
  data: Record<string, unknown>,
): Promise<string> {
  const direct = httpUrl(data.url) || httpUrl(data.file)
  if (direct) return direct
  const file = String(data.file || "").trim()
  try {
    if (type === "image" && file) {
      const result = await bridge.callAction("get_image", { file }) as Record<string, unknown>
      return httpUrl(result?.url) || httpUrl(result?.file)
    }
    if (type === "record" && file) {
      const result = await bridge.callAction("get_record", { file, out_format: "mp3" }) as Record<string, unknown>
      return httpUrl(result?.url) || httpUrl(result?.file)
    }
    if (type === "file") {
      const fileId = String(data.file_id || "").trim()
      if (!fileId) return ""
      const action = event.message_type === "group" ? "get_group_file_url" : "get_private_file_url"
      const params: Record<string, unknown> = { file_id: fileId }
      if (event.group_id) params.group_id = event.group_id
      const result = await bridge.callAction(action, params) as Record<string, unknown>
      return httpUrl(result?.url) || httpUrl(result?.file)
    }
  } catch {}
  return ""
}

export async function parseOneBotMessage(bridge: OneBotBridge, event: OneBotEvent): Promise<ParsedQQMessage> {
  const selfId = String(event.self_id || "")
  const sender = event.sender || {}
  const senderId = String(sender.user_id || event.user_id || "")
  const messageId = String(event.message_id || "")
  const messageType = event.message_type === "group" ? "group" : "private"
  const groupId = String(event.group_id || "")
  const segments = messageSegments(event)
  let content = ""
  let mentionedSelf = false
  let imageUrl = ""
  let videoUrl = ""
  let audioUrl = ""
  let fileUrl = ""
  let fileName = ""
  let replyToMessageId = ""

  for (const segment of segments) {
    const type = String(segment.type || "")
    const data = segment.data || {}
    if (type === "text") {
      content += String(data.text || "")
      continue
    }
    if (type === "at") {
      const qq = String(data.qq || "")
      if (qq === selfId || qq === "all") {
        mentionedSelf = qq === selfId
      } else if (qq) {
        content += ` @${String(data.name || qq)} `
      }
      continue
    }
    if (type === "reply") {
      replyToMessageId = String(data.id || "")
      continue
    }
    if (type === "image") {
      const hadImage = Boolean(imageUrl)
      const resolved = await resolveUrl(bridge, event, type, data)
      if (!imageUrl) imageUrl = resolved
      if (hadImage || !resolved) content += resolved ? `[图片]${resolved}` : "[图片]"
      continue
    }
    if (type === "video") {
      const hadVideo = Boolean(videoUrl)
      const resolved = await resolveUrl(bridge, event, type, data)
      if (!videoUrl) videoUrl = resolved
      if (hadVideo || !resolved) content += resolved ? `[视频]${resolved}` : "[视频]"
      continue
    }
    if (type === "record" || type === "voice" || type === "audio") {
      const hadAudio = Boolean(audioUrl)
      const resolved = await resolveUrl(bridge, event, "record", data)
      if (!audioUrl) audioUrl = resolved
      if (hadAudio || !resolved) content += resolved ? `[语音]${resolved}` : "[语音]"
      continue
    }
    if (type === "file") {
      fileName = String(data.file_name || data.name || data.file || "文件")
      const resolved = await resolveUrl(bridge, event, type, data)
      if (!fileUrl) fileUrl = resolved
      content += `[文件]${fileName}${resolved ? ` ${resolved}` : ""}`
      continue
    }
    if (type) {
      content += marker(type)
    }
  }

  return {
    selfId,
    senderId,
    messageId,
    messageType,
    groupId,
    content: content.trim(),
    imageUrl,
    videoUrl,
    audioUrl,
    fileUrl,
    fileName,
    replyToMessageId,
    mentionedSelf,
    toMe: event.to_me === true,
    createdAt: Number(event.time || Math.floor(Date.now() / 1000)) * 1000,
  }
}

export function shouldHandleMessage(message: ParsedQQMessage, event: OneBotEvent, config: QQPersonalConfig): boolean {
  if (!message.senderId || message.senderId === message.selfId) return false
  if (config.ignoredUserIds.includes(message.senderId)) return false
  if (message.messageType === "private" && event.sub_type === "group") return false
  if (message.messageType !== "group") return true
  if (config.groupTrigger === "all") return true
  if (config.groupTrigger === "mention") return message.mentionedSelf || message.toMe
  return Boolean(config.triggerPrefix) && message.content.startsWith(config.triggerPrefix)
}

export function applyPrefix(message: ParsedQQMessage, config: QQPersonalConfig): ParsedQQMessage {
  if (message.messageType !== "group" || config.groupTrigger !== "prefix") return message
  return { ...message, content: message.content.slice(config.triggerPrefix.length).trim() }
}

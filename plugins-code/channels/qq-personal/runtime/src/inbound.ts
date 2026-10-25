import type { ParsedQQMessage } from "./message.js"

export interface InboundPayload {
  channelId: "qq_personal"
  accountId: string
  conversationId: string
  peerId: string
  messageId: string
  contentType: "text" | "image" | "video" | "voice"
  text: string
  imageUrl?: string
  videoUrl?: string
  audioUrl?: string
  replyToMessageId?: string
  source: "qq_personal"
}

export interface InboundClientConfig {
  coreUrl: string
  extensionId: string
  moduleId: string
  serviceAuthToken: string
  localToken: string
}

export function buildInboundPayload(message: ParsedQQMessage): InboundPayload {
  const peerId = message.messageType === "group" ? message.groupId : message.senderId
  if (!peerId) throw new Error("QQ 消息缺少群号或发送者")
  const contentType = message.audioUrl
    ? "voice"
    : !message.content && message.videoUrl
      ? "video"
      : !message.content && message.imageUrl
        ? "image"
        : "text"
  const fallback = message.imageUrl
    ? "[图片]"
    : message.videoUrl
      ? "[视频]"
      : message.audioUrl
        ? "[语音]"
        : message.fileUrl
          ? `[文件]${message.fileName}`
          : "[消息]"
  const payload: InboundPayload = {
    channelId: "qq_personal",
    accountId: message.selfId || "qq-personal",
    conversationId: message.messageType === "group"
      ? `conv-qq-personal-group-${message.groupId}`
      : `conv-qq-personal-private-${message.senderId}`,
    peerId,
    messageId: message.messageId,
    contentType,
    text: message.content || fallback,
    source: "qq_personal",
  }
  if (message.imageUrl) payload.imageUrl = message.imageUrl
  if (message.videoUrl) payload.videoUrl = message.videoUrl
  if (message.audioUrl) payload.audioUrl = message.audioUrl
  if (message.replyToMessageId) payload.replyToMessageId = message.replyToMessageId
  return payload
}

export function coreHeaders(config: InboundClientConfig): Record<string, string> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    "X-Amitia-Extension-ID": config.extensionId,
    "X-Amitia-Module-ID": config.moduleId,
  }
  if (config.serviceAuthToken) headers.Authorization = `Bearer ${config.serviceAuthToken}`
  if (config.localToken) headers["X-Amitia-Local-Token"] = config.localToken
  return headers
}

export async function forwardInbound(config: InboundClientConfig, payload: InboundPayload, fetcher: typeof fetch = fetch): Promise<void> {
  if (!config.serviceAuthToken) throw new Error("QQ 个人号入站缺少宿主服务凭据")
  const response = await fetcher(`${config.coreUrl.replace(/\/$/, "")}/api/channels/inbound`, {
    method: "POST",
    headers: coreHeaders(config),
    body: JSON.stringify(payload),
    signal: AbortSignal.timeout(600000),
  })
  const body = await response.json().catch(() => ({})) as { code?: number; msg?: string }
  if (!response.ok || body.code !== 200) {
    throw new Error(`QQ 个人号入站失败 HTTP ${response.status}: ${body.msg || "宿主拒绝请求"}`)
  }
}

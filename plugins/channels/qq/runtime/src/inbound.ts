// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import type { QQMessage } from "./qqbot-client.js"

export interface BufferedQQMessage extends QQMessage {
  imageData?: string
}

export interface QQInboundPayload {
  channelId: "qq"
  accountId: string
  conversationId: string
  peerId: string
  senderId: string
  messageId: string
  contentType: "text" | "image" | "video" | "voice"
  text: string
  imageUrl: string
  videoUrl: string
  audioUrl: string
  createdAt: number
}

export function buildQQInboundPayload(input: {
  messages: BufferedQQMessage[]
  accountId: string
  imageUrl?: string
  videoUrl?: string
  audioUrl?: string
}): QQInboundPayload {
  const messages = Array.isArray(input.messages) ? input.messages.filter(Boolean) : []
  if (messages.length === 0) throw new Error("QQ 入站消息为空")
  const last = messages[messages.length - 1]
  const text = messages.map((message) => message.text).filter(Boolean).join("\n").trim()
  const imageUrl = String(input.imageUrl || "").trim()
  const videoUrl = String(input.videoUrl || "").trim()
  const audioUrl = String(input.audioUrl || "").trim()
  const peerId = String(last.groupId || last.fromUserId || "").trim()
  if (!peerId) throw new Error("QQ 入站消息缺少发送方标识")
  const contentType = audioUrl ? "voice" : !text && videoUrl ? "video" : !text && imageUrl ? "image" : "text"
  const fallback = imageUrl ? "[图片]" : videoUrl ? "[视频]" : audioUrl ? "[语音]" : ""
  return {
    channelId: "qq",
    accountId: input.accountId || "qqbot",
    conversationId: last.groupId ? `conv-group-${last.groupId}` : `conv-${last.fromUserId}`,
    peerId,
    senderId: String(last.fromUserId || "").trim(),
    messageId: String(last.messageId || "").trim(),
    contentType,
    text: text || fallback,
    imageUrl,
    videoUrl,
    audioUrl,
    createdAt: Number(last.createdAt) || Date.now(),
  }
}

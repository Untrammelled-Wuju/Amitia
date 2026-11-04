export type ChannelMediaKind = "image" | "video" | "audio" | "file"

export interface ChannelMediaItem {
  kind: ChannelMediaKind
  url: string
  name: string
  mime: string
}

export interface NormalizedChannelMessage extends Record<string, unknown> {
  channelId: string
  role: string
  text: string
  senderLabel: string
  senderID: string
  mentions: string[]
  replyExcerpt: string
  replySender: string
  peerKind: string
  media: ChannelMediaItem[]
}

export function resolveConversationChannelId(
  conversation: Record<string, unknown>,
  availableChannels?: ReadonlySet<string>,
): string {
  const body = conversation.body && typeof conversation.body === "object"
    ? conversation.body as Record<string, unknown>
    : {}
  const channel = text(conversation.channel ?? conversation.channelId ?? body.channelId)
  if (availableChannels && channel && !availableChannels.has(channel)) return ""
  return channel
}

const IMAGE_EXTENSIONS = /\.(?:png|jpe?g|gif|webp|bmp|svg)(?:[?#]|$)/i
const VIDEO_EXTENSIONS = /\.(?:mp4|webm|mov|m4v)(?:[?#]|$)/i
const AUDIO_EXTENSIONS = /\.(?:mp3|wav|ogg|m4a|aac|flac|opus)(?:[?#]|$)/i

function text(value: unknown): string {
  return typeof value === "string" ? value.trim() : ""
}

function safeUrl(value: unknown, kind: ChannelMediaKind): string {
  const url = text(value)
  if (!url) return ""
  if (/^(?:https?:|blob:|\/)/i.test(url)) return url
  if (kind === "image" && /^data:image\//i.test(url)) return url
  if (kind === "video" && /^data:video\//i.test(url)) return url
  if (kind === "audio" && /^data:audio\//i.test(url)) return url
  if (kind === "file" && /^data:(?:application|text|image|audio|video)\//i.test(url)) return url
  return ""
}

function kindFromMime(mime: string, fallback: ChannelMediaKind = "file"): ChannelMediaKind {
  const value = mime.toLowerCase()
  if (value.startsWith("image/")) return "image"
  if (value.startsWith("video/")) return "video"
  if (value.startsWith("audio/")) return "audio"
  return fallback
}

function kindFromUrl(url: string, fallback: ChannelMediaKind = "file"): ChannelMediaKind {
  if (IMAGE_EXTENSIONS.test(url)) return "image"
  if (VIDEO_EXTENSIONS.test(url)) return "video"
  if (AUDIO_EXTENSIONS.test(url)) return "audio"
  return fallback
}

function attachmentUrl(attachment: Record<string, unknown>): string {
  const direct = text(attachment.url)
  if (direct) return direct
  const mime = text(attachment.mimeType)
  const data = text(attachment.data)
  return data && mime ? `data:${mime};base64,${data}` : ""
}

export function normalizeChannelMessage(message: Record<string, unknown>, channelId = ""): NormalizedChannelMessage {
  const media: ChannelMediaItem[] = []
  const seen = new Set<string>()
  const add = (kind: ChannelMediaKind, urlValue: unknown, nameValue: unknown, mimeValue: unknown) => {
    const mime = text(mimeValue).toLowerCase()
    const resolvedKind = kindFromMime(mime, kind)
    const url = safeUrl(urlValue, resolvedKind)
    if (!url) return
    const key = `${resolvedKind}:${url}`
    if (seen.has(key)) return
    seen.add(key)
    media.push({
      kind: resolvedKind,
      url,
      name: text(nameValue) || (resolvedKind === "file" ? "附件" : ""),
      mime,
    })
  }

  const imageUrl = text(message.imageUrl)
  const videoUrl = text(message.videoUrl)
  const audioUrl = text(message.audioUrl)
  const fileUrl = text(message.fileUrl)
  add("image", imageUrl, message.altText, message.mimeType)
  add("video", videoUrl, message.fileName, message.mimeType)
  add("audio", audioUrl, message.fileName, message.mimeType)
  add("file", fileUrl, message.fileName, message.mimeType)

  const attachments = Array.isArray(message.attachments) ? message.attachments : []
  for (const raw of attachments) {
    if (!raw || typeof raw !== "object") continue
    const attachment = raw as Record<string, unknown>
    const mime = text(attachment.mimeType)
    const explicit = text(attachment.kind ?? attachment.type).toLowerCase()
    const hint: ChannelMediaKind = explicit === "image" || explicit === "video" || explicit === "audio" || explicit === "file"
      ? explicit
      : kindFromMime(mime)
    const url = attachmentUrl(attachment)
    add(kindFromUrl(url, hint), url, attachment.name ?? attachment.fileName, mime)
  }

  const content = text(message.content)
  const placeholder = /^\[(?:图片|视频|语音|音频|文件)\]$/.test(content)
  const role = text(message.role) || "user"
  const senderLabel = text(message.senderName) || (role === "user" ? "对方" : "Amitia")

  const quote = message.quote && typeof message.quote === "object" ? message.quote as Record<string, unknown> : {}
  const mentionNames = Array.isArray(message.mentionNames)
    ? message.mentionNames.map((item) => text(item)).filter(Boolean)
    : []

  return {
    ...message,
    channelId: text(message.channelId ?? channelId),
    role,
    text: media.length > 0 && placeholder ? "" : content,
    senderLabel,
    senderID: text(message.senderId),
    mentions: mentionNames,
    replyExcerpt: text(message.replyExcerpt ?? quote.content),
    replySender: text(message.replySenderName ?? quote.senderName),
    peerKind: text(message.peerKind),
    media,
  }
}

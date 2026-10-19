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
  media: ChannelMediaItem[]
}

export function resolveConversationChannelId(
  conversation: Record<string, unknown>,
  availableChannels?: ReadonlySet<string>,
): string {
  const body = conversation.body && typeof conversation.body === "object"
    ? conversation.body as Record<string, unknown>
    : {}
  const explicit = text(conversation.channel ?? conversation.channelId ?? body.channel ?? body.channelId)
  const id = text(conversation.resourceId ?? conversation.id)
  let inferred = explicit
  if (!inferred && id.startsWith("wechat-ipad860-")) inferred = "wechat_ipad860"
  if (!inferred && (id.includes("@im.wechat") || id.includes("@chatroom"))) inferred = "wechat"
  if (!inferred && (id.startsWith("conv-group-") || id.startsWith("conv-"))) inferred = "qq"
  if (availableChannels && inferred && !availableChannels.has(inferred)) return ""
  return inferred
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
  const direct = text(attachment.url ?? attachment.src ?? attachment.href)
  if (direct) return direct
  const mime = text(attachment.mimeType ?? attachment.mime_type ?? attachment.contentType ?? attachment.content_type)
  const data = text(attachment.data ?? attachment.base64)
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

  const imageUrl = text(message.imageUrl ?? message.image_url ?? message.image)
  const videoUrl = text(message.videoUrl ?? message.video_url ?? message.video)
  const audioUrl = text(message.audioUrl ?? message.audio_url ?? message.audio ?? message.voiceUrl ?? message.voice_url)
  const fileUrl = text(message.fileUrl ?? message.file_url ?? message.file)
  add("image", imageUrl, message.altText ?? message.alt_text, message.mimeType ?? message.mime_type)
  add("video", videoUrl, message.fileName ?? message.file_name, message.mimeType ?? message.mime_type)
  add("audio", audioUrl, message.fileName ?? message.file_name, message.mimeType ?? message.mime_type)
  add("file", fileUrl, message.fileName ?? message.file_name, message.mimeType ?? message.mime_type)

  const attachments = Array.isArray(message.attachments) ? message.attachments : []
  for (const raw of attachments) {
    if (!raw || typeof raw !== "object") continue
    const attachment = raw as Record<string, unknown>
    const mime = text(attachment.mimeType ?? attachment.mime_type ?? attachment.contentType ?? attachment.content_type)
    const explicit = text(attachment.kind ?? attachment.type).toLowerCase()
    const hint: ChannelMediaKind = explicit === "image" || explicit === "video" || explicit === "audio" || explicit === "file"
      ? explicit
      : kindFromMime(mime)
    const url = attachmentUrl(attachment)
    add(kindFromUrl(url, hint), url, attachment.name ?? attachment.fileName ?? attachment.file_name, mime)
  }

  const content = text(message.content ?? message.text)
  const placeholder = /^\[(?:图片|视频|语音|音频|文件)\]$/.test(content)
  const role = text(message.role) || "user"
  const senderLabel = text(
    message.senderName ??
    message.sender_name ??
    message.fromUserName ??
    message.from_user_name ??
    message.authorName ??
    message.author_name,
  ) || (role === "user" ? "对方" : "Amitia")

  return {
    ...message,
    channelId: text(message.channelId ?? message.channel ?? channelId),
    role,
    text: media.length > 0 && placeholder ? "" : content,
    senderLabel,
    media,
  }
}

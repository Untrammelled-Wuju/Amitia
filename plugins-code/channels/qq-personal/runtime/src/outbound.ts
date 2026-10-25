import type { InboundClientConfig } from "./inbound.js"
import { coreHeaders } from "./inbound.js"
import type { OneBotBridge } from "./onebot.js"

const maxMediaBytes = 50 * 1024 * 1024

export interface MediaAsset {
  buffer: Buffer
  mime: string
}

export interface MediaTarget {
  kind: "group" | "private"
  id: string
}

function absoluteMediaUrl(coreUrl: string, rawUrl: string): string {
  return /^(https?:|data:)/i.test(rawUrl)
    ? rawUrl
    : coreUrl.replace(/\/$/, "") + (rawUrl.startsWith("/") ? rawUrl : `/${rawUrl}`)
}

export async function downloadMedia(
  coreUrl: string,
  rawUrl: string,
  fetcher: typeof fetch = fetch,
): Promise<MediaAsset> {
  const response = await fetcher(absoluteMediaUrl(coreUrl, rawUrl), {
    signal: AbortSignal.timeout(30000),
  })
  if (!response.ok) throw new Error(`媒体下载失败 HTTP ${response.status}`)
  const declaredLength = Number(response.headers.get("content-length") || 0)
  if (declaredLength > maxMediaBytes) throw new Error("媒体文件超过 50 MiB 上限")
  const buffer = Buffer.from(await response.arrayBuffer())
  if (!buffer.length) throw new Error("媒体文件为空")
  if (buffer.length > maxMediaBytes) throw new Error("媒体文件超过 50 MiB 上限")
  return {
    buffer,
    mime: response.headers.get("content-type") || "application/octet-stream",
  }
}

export async function synthesizeVoice(
  config: InboundClientConfig,
  text: string,
  fetcher: typeof fetch = fetch,
): Promise<MediaAsset> {
  const response = await fetcher(`${config.coreUrl.replace(/\/$/, "")}/api/tts/preview`, {
    method: "POST",
    headers: coreHeaders(config),
    body: JSON.stringify({ text }),
    signal: AbortSignal.timeout(600000),
  })
  const body = await response.json().catch(() => ({})) as {
    code?: number
    msg?: string
    data?: { audioUrl?: string }
  }
  if (!response.ok || body.code !== 200) {
    throw new Error(body.msg || `TTS 合成失败 HTTP ${response.status}`)
  }
  const audioUrl = String(body.data?.audioUrl || "").trim()
  if (!audioUrl) throw new Error("TTS 未返回音频")
  return downloadMedia(config.coreUrl, audioUrl, fetcher)
}

function base64Url(asset: MediaAsset): string {
  return `base64://${asset.buffer.toString("base64")}`
}

export async function sendMedia(
  bridge: OneBotBridge,
  target: MediaTarget,
  type: "image" | "video" | "record" | "file",
  asset: MediaAsset,
  fileName = "",
): Promise<unknown> {
  if (type !== "file") {
    return bridge.sendMessage(target.kind, target.id, [{
      type,
      data: { file: base64Url(asset) },
    }])
  }
  const numericTarget = Number(target.id)
  if (!Number.isSafeInteger(numericTarget) || numericTarget <= 0) {
    throw new Error("QQ 群号或用户号无效")
  }
  const action = target.kind === "group" ? "upload_group_file" : "upload_private_file"
  const key = target.kind === "group" ? "group_id" : "user_id"
  return bridge.callAction(action, {
    [key]: numericTarget,
    file: base64Url(asset),
    name: fileName || "文件",
  })
}

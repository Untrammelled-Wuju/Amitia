import assert from "node:assert/strict"
import test from "node:test"
import type { InboundClientConfig } from "./inbound.js"
import { downloadMedia, sendMedia, synthesizeVoice } from "./outbound.js"
import type { OneBotBridge } from "./onebot.js"

const config: InboundClientConfig = {
  coreUrl: "http://127.0.0.1:18899",
  extensionId: "com.amitia/channel-qq-personal",
  moduleId: "qq-personal-service",
  channelId: "qq_personal",
  serviceAuthToken: "service-token",
  localToken: "local-token",
}

test("语音合成使用当前预览接口并携带宿主凭据", async () => {
  const calls: Array<{ url: string; init: RequestInit }> = []
  const fetcher = (async (url: string | URL | Request, init?: RequestInit) => {
    calls.push({ url: String(url), init: init || {} })
    if (String(url).endsWith("/api/tts/preview")) {
      return new Response(JSON.stringify({ code: 200, data: { audioUrl: "/audio/test.mp3" } }), {
        status: 200,
        headers: { "content-type": "application/json" },
      })
    }
    return new Response(Buffer.from("mp3"), {
      status: 200,
      headers: { "content-type": "audio/mpeg" },
    })
  }) as typeof fetch
  const asset = await synthesizeVoice(config, "你好", fetcher)
  assert.equal(asset.mime, "audio/mpeg")
  assert.equal(asset.buffer.toString(), "mp3")
  const headers = calls[0].init.headers as Record<string, string>
  assert.equal(calls[0].url, "http://127.0.0.1:18899/api/tts/preview")
  assert.equal(headers.Authorization, "Bearer service-token")
  assert.equal(headers["X-Amitia-Local-Token"], "local-token")
})

test("媒体下载通过渠道媒体代理携带服务身份并拒绝超限响应", async () => {
  const calls: Array<{ url: string; init: RequestInit }> = []
  const fetcher = (async (url: string | URL | Request, init?: RequestInit) => {
    calls.push({ url: String(url), init: init || {} })
    return new Response(Buffer.from("image"), {
      status: 200,
      headers: { "content-type": "image/png", "content-length": "5" },
    })
  }) as typeof fetch
  const asset = await downloadMedia(config, "/api/extension/resources/signed.png", fetcher)
  assert.equal(asset.buffer.toString(), "image")
  assert.equal(asset.mime, "image/png")
  assert.equal(calls[0].url, "http://127.0.0.1:18899/api/channels/media/signed.png")
  const headers = calls[0].init.headers as Record<string, string>
  assert.equal(headers.Authorization, "Bearer service-token")
  assert.equal(headers["X-Amitia-Channel-ID"], "qq_personal")
  await assert.rejects(
    downloadMedia(config, "/api/extension/resources/a.png", (async () => new Response("x", {
      headers: { "content-length": String(51 * 1024 * 1024) },
    })) as typeof fetch),
    /50 MiB/,
  )
})

test("语音、视频、图片使用消息段发送，文件使用上传动作", async () => {
  const messages: unknown[][] = []
  const actions: Array<{ action: string; params: Record<string, unknown> }> = []
  const bridge = {
    sendMessage: async (_kind: string, _id: string, message: unknown[]) => {
      messages.push(message)
      return { message_id: messages.length }
    },
    callAction: async (action: string, params: Record<string, unknown>) => {
      actions.push({ action, params })
      return { file_id: "file-1" }
    },
  } as unknown as OneBotBridge
  const asset = { buffer: Buffer.from("media"), mime: "application/octet-stream" }
  await sendMedia(bridge, { kind: "group", id: "100" }, "image", asset)
  await sendMedia(bridge, { kind: "group", id: "100" }, "video", asset)
  await sendMedia(bridge, { kind: "group", id: "100" }, "record", asset)
  await sendMedia(bridge, { kind: "group", id: "100" }, "file", asset, "report.pdf")
  assert.deepEqual(messages.map((item) => (item[0] as { type: string }).type), ["image", "video", "record"])
  assert.equal(actions[0].action, "upload_group_file")
  assert.equal(actions[0].params.name, "report.pdf")
  assert.match(String(actions[0].params.file), /^base64:\/\//)
})

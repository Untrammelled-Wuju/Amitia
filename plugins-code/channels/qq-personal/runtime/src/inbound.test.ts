import assert from "node:assert/strict"
import test from "node:test"
import { buildInboundPayload, coreHeaders, forwardInbound } from "./inbound.js"
import type { ParsedQQMessage } from "./message.js"

const base: ParsedQQMessage = {
  selfId: "10001",
  senderId: "40004",
  senderName: "测试用户",
  mentionNames: [],
  replyExcerpt: "",
  messageId: "30003",
  messageType: "group",
  groupId: "20002",
  content: "你好",
  imageUrl: "",
  videoUrl: "",
  audioUrl: "",
  fileUrl: "",
  fileName: "",
  replyToMessageId: "",
  mentionedSelf: true,
  toMe: true,
  createdAt: 1,
}

test("群聊入站使用独立渠道和会话标识", () => {
  const payload = buildInboundPayload(base)
  assert.equal(payload.channelId, "qq_personal")
  assert.equal(payload.accountId, "10001")
  assert.equal(payload.peerId, "20002")
  assert.equal(payload.conversationId, "conv-qq-personal-group-20002")
  assert.equal(payload.contentType, "text")
})

test("纯图片消息生成媒体回退文本", () => {
  const payload = buildInboundPayload({ ...base, content: "", imageUrl: "https://example.test/a.png" })
  assert.equal(payload.contentType, "image")
  assert.equal(payload.text, "[图片]")
  assert.equal(payload.imageUrl, "https://example.test/a.png")
})

test("私聊入站使用发送者作为 peer", () => {
  const payload = buildInboundPayload({ ...base, messageType: "private", groupId: "" })
  assert.equal(payload.peerId, "40004")
  assert.equal(payload.conversationId, "conv-qq-personal-private-40004")
})

test("入站转发携带宿主身份和 OneBot 凭据", async () => {
  const config = {
    coreUrl: "http://127.0.0.1:18899",
    extensionId: "com.amitia/channel-qq-personal",
    moduleId: "qq-personal-service",
    channelId: "qq_personal",
    serviceAuthToken: "service-token",
    localToken: "local-token",
  }
  const captures: Array<{ url: string; init: RequestInit }> = []
  const fetcher = (async (url: string | URL | Request, init?: RequestInit) => {
    captures.push({ url: String(url), init: init || {} })
    return new Response(JSON.stringify({ code: 200, msg: "ok" }), { status: 200 })
  }) as typeof fetch
  await forwardInbound(config, buildInboundPayload(base), fetcher)
  assert.equal(captures[0].url, "http://127.0.0.1:18899/api/channels/inbound")
  const headers = captures[0].init.headers as Record<string, string>
  assert.equal(headers.Authorization, "Bearer service-token")
  assert.equal(headers["X-Amitia-Extension-ID"], config.extensionId)
  assert.equal(headers["X-Amitia-Module-ID"], config.moduleId)
  assert.equal(headers["X-Amitia-Channel-ID"], config.channelId)
  assert.equal(headers["X-Amitia-Local-Token"], config.localToken)
  assert.equal(coreHeaders(config)["X-Amitia-Module-ID"], "qq-personal-service")
})

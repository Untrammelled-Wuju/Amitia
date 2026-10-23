// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
import assert from "node:assert/strict"
import test from "node:test"
import { buildQQInboundPayload } from "./inbound.ts"

function message(overrides: Record<string, unknown> = {}) {
  return {
    fromUserId: "user-1",
    toUserId: "bot-1",
    messageId: "msg-1",
    text: "你好",
    createdAt: 1720000000000,
    ...overrides,
  }
}

test("QQ 私聊使用核心入站契约字段", () => {
  const payload = buildQQInboundPayload({ messages: [message()], accountId: "bot-1" })
  assert.equal(payload.channelId, "qq")
  assert.equal(payload.peerId, "user-1")
  assert.equal(payload.senderId, "user-1")
  assert.equal(payload.conversationId, "conv-user-1")
  assert.equal(payload.contentType, "text")
  assert.equal(payload.text, "你好")
})

test("QQ 群聊按群标识建立会话", () => {
  const payload = buildQQInboundPayload({ messages: [message({ groupId: "group-1" })], accountId: "bot-1" })
  assert.equal(payload.peerId, "group-1")
  assert.equal(payload.senderId, "user-1")
  assert.equal(payload.conversationId, "conv-group-group-1")
})

test("图片消息带占位文本与图片地址", () => {
  const payload = buildQQInboundPayload({ messages: [message({ text: "" })], accountId: "bot-1", imageUrl: "data:image/png;base64,AA==" })
  assert.equal(payload.contentType, "image")
  assert.equal(payload.text, "[图片]")
  assert.equal(payload.imageUrl, "data:image/png;base64,AA==")
})

test("语音消息使用 audioUrl 并标记 voice", () => {
  const payload = buildQQInboundPayload({
    messages: [message({ text: "[语音]", isVoice: true })],
    accountId: "bot-1",
    audioUrl: "data:audio/ogg;base64,AA==",
  })
  assert.equal(payload.contentType, "voice")
  assert.equal(payload.audioUrl, "data:audio/ogg;base64,AA==")
})

import assert from "node:assert/strict"
import test from "node:test"
import { applyPrefix, parseOneBotMessage, shouldHandleMessage } from "./message.js"
import type { QQPersonalConfig } from "./config.js"
import type { OneBotBridge } from "./onebot.js"

const bridge = {
  callAction: async () => ({}),
} as unknown as OneBotBridge

const config: QQPersonalConfig = {
  enabled: true,
  listenHost: "127.0.0.1",
  listenPort: 6199,
  websocketPath: "/ws",
  token: "token",
  groupTrigger: "mention",
  triggerPrefix: "Amitia",
  ignoredUserIds: ["2854196310"],
  webuiPort: 5099,
  novncPort: 6081,
  containerName: "snowluma",
  dockerBin: "",
}

test("解析群聊 @机器人 和回复消息", async () => {
  const message = await parseOneBotMessage(bridge, {
    post_type: "message",
    message_type: "group",
    self_id: 10001,
    group_id: 20002,
    message_id: 30003,
    sender: { user_id: 40004 },
    message: [
      { type: "at", data: { qq: "10001", name: "Amitia" } },
      { type: "text", data: { text: " 你好" } },
      { type: "reply", data: { id: "29999" } },
    ],
  })
  assert.equal(message.content, "你好")
  assert.equal(message.mentionedSelf, true)
  assert.equal(message.replyToMessageId, "29999")
  assert.equal(shouldHandleMessage(message, { sub_type: "normal" }, config), true)
})

test("群聊未 @机器人时默认忽略", async () => {
  const message = await parseOneBotMessage(bridge, {
    post_type: "message",
    message_type: "group",
    self_id: 10001,
    group_id: 20002,
    message_id: 30004,
    sender: { user_id: 40004 },
    message: [{ type: "text", data: { text: "普通消息" } }],
  })
  assert.equal(shouldHandleMessage(message, { sub_type: "normal" }, config), false)
})

test("前缀触发模式移除前缀", async () => {
  const prefixed = { ...config, groupTrigger: "prefix" as const }
  const message = await parseOneBotMessage(bridge, {
    post_type: "message",
    message_type: "group",
    self_id: 10001,
    group_id: 20002,
    message_id: 30005,
    sender: { user_id: 40004 },
    message: [{ type: "text", data: { text: "Amitia 帮我总结" } }],
  })
  assert.equal(shouldHandleMessage(message, {}, prefixed), true)
  assert.equal(applyPrefix(message, prefixed).content, "帮我总结")
})

test("解析图片 URL 和私聊消息", async () => {
  const message = await parseOneBotMessage(bridge, {
    post_type: "message",
    message_type: "private",
    sub_type: "friend",
    self_id: 10001,
    user_id: 40004,
    message_id: 30006,
    sender: { user_id: 40004 },
    message: [{ type: "image", data: { url: "https://example.test/image.png" } }],
  })
  assert.equal(message.imageUrl, "https://example.test/image.png")
  assert.equal(shouldHandleMessage(message, { sub_type: "friend" }, config), true)
})

test("解析 CQ 字符串中的文本、@、回复和媒体", async () => {
  const message = await parseOneBotMessage(bridge, {
    post_type: "message",
    message_type: "group",
    self_id: 10001,
    group_id: 20002,
    message_id: 30007,
    sender: { user_id: 40004 },
    message: "[CQ:reply,id=29999][CQ:at,qq=10001,name=Amitia] 看这个[CQ:image,url=https://example.test/a.png]",
  })
  assert.equal(message.content, "看这个")
  assert.equal(message.replyToMessageId, "29999")
  assert.equal(message.mentionedSelf, true)
  assert.equal(message.imageUrl, "https://example.test/a.png")
})

test("群聊全部消息和忽略用户策略生效", async () => {
  const message = await parseOneBotMessage(bridge, {
    post_type: "message",
    message_type: "group",
    self_id: 10001,
    group_id: 20002,
    message_id: 30008,
    sender: { user_id: 40004 },
    message: [{ type: "text", data: { text: "普通消息" } }],
  })
  assert.equal(shouldHandleMessage(message, {}, { ...config, groupTrigger: "all" }), true)
  assert.equal(shouldHandleMessage(message, {}, { ...config, groupTrigger: "all", ignoredUserIds: ["40004"] }), false)
})

test("解析视频、语音、文件和额外图片", async () => {
  const resolvingBridge = {
    callAction: async (action: string) => {
      if (action === "get_group_file_url") return { url: "https://example.test/report.pdf" }
      return {}
    },
  } as unknown as OneBotBridge
  const message = await parseOneBotMessage(resolvingBridge, {
    post_type: "message",
    message_type: "group",
    self_id: 10001,
    group_id: 20002,
    message_id: 30009,
    sender: { user_id: 40004 },
    message: [
      { type: "video", data: { url: "https://example.test/a.mp4" } },
      { type: "record", data: { url: "https://example.test/a.mp3" } },
      { type: "image", data: { url: "https://example.test/a.png" } },
      { type: "image", data: { url: "https://example.test/b.png" } },
      { type: "file", data: { file_id: "file-1", file_name: "report.pdf" } },
    ],
  })
  assert.equal(message.videoUrl, "https://example.test/a.mp4")
  assert.equal(message.audioUrl, "https://example.test/a.mp3")
  assert.equal(message.imageUrl, "https://example.test/a.png")
  assert.equal(message.fileUrl, "https://example.test/report.pdf")
  assert.equal(message.fileName, "report.pdf")
  assert.match(message.content, /https:\/\/example\.test\/b\.png/)
  assert.match(message.content, /\[文件\]report\.pdf/)
})

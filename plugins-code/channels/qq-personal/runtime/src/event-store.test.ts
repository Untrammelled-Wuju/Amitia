import assert from "node:assert/strict"
import fs from "node:fs"
import os from "node:os"
import path from "node:path"
import test from "node:test"

test("persists channel messages, logs and counters", async t => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), "qq-event-store-"))
  const previous = process.env.AMITIA_MODULE_DATA_DIR
  process.env.AMITIA_MODULE_DATA_DIR = directory
  t.after(() => {
    if (previous === undefined) delete process.env.AMITIA_MODULE_DATA_DIR
    else process.env.AMITIA_MODULE_DATA_DIR = previous
    fs.rmSync(directory, { recursive: true, force: true })
  })
  const { ChannelEventStore } = await import(`./event-store.js?test=${Date.now()}`) as typeof import("./event-store.js")
  const store = new ChannelEventStore()
  store.recordInbound({
    channelId: "qq_personal",
    accountId: "10001",
    conversationId: "conv-qq-personal-private-20002",
    peerId: "20002",
    peerKind: "private",
    senderId: "20002",
    senderName: "Alice",
    messageId: "m1",
    contentType: "text",
    text: "hello",
    source: "qq_personal",
  })
  store.recordOutbound({
    conversationId: "conv-qq-personal-private-20002",
    peerId: "20002",
    peerKind: "private",
    contentType: "text",
    text: "world",
  })
  store.log("info", "message.inbound", "hello")

  const messages = store.messages("conv-qq-personal-private-20002", 20, 0)
  assert.equal(messages.total, 2)
  assert.deepEqual(messages.messages.map(item => item.direction), ["outbound", "inbound"])
  assert.deepEqual(store.counts(), { inbound: 1, outbound: 1 })
  assert.equal(store.logs(20, 0).total, 1)

  const reopened = new ChannelEventStore()
  assert.equal(reopened.messages("", 20, 0).total, 2)
  assert.equal(reopened.logs(20, 0).total, 1)
})

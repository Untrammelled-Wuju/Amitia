import assert from "node:assert/strict";
import test from "node:test";

import {
  createConversationData,
  projectConversationMessages,
  refreshConversationData,
} from "../src/client-ui/conversation-data.mjs";
import { projectSessionEvents } from "../src/client-ui/session-events.mjs";

const MESSAGES = [
  {
    conversationId: "conversation-1",
    role: "user",
    timestamp: "2026-11-17T02:12:00Z",
    contentSummary: "你好，这是一条渲染验证消息",
  },
  {
    conversationId: "conversation-1",
    role: "assistant",
    timestamp: "2026-11-17T02:13:02Z",
    contentSummary: "收到啦，渲染正常。",
  },
];

test("projects Amitia messages into chat nodes with DSH turn locations", () => {
  const projection = projectConversationMessages(MESSAGES);
  assert.equal(projection.order.length, 2);
  assert.deepEqual([...projection.order], ["user:0", "assistant-step:1"]);

  const user = projection.nodes.get("user:0");
  assert.equal(user.kind, "user");
  assert.equal(user.data.source.kind, "user");
  assert.equal(user.data.source.form, null);
  assert.deepEqual(user.data.content, [{ type: "text", text: "你好，这是一条渲染验证消息" }]);
  assert.equal(user.location.kind, "turn");
  assert.equal(user.location.turn.start.type, "turn/start");
  assert.equal(user.location.turn.end.type, "turn/end");
  assert.deepEqual(user.location.turn.end.data.reason, { kind: "completed" });
  assert.equal(user.location.turn.status, "closed");
  assert.deepEqual(user.location.turn.steps, []);

  const assistant = projection.nodes.get("assistant-step:1");
  assert.equal(assistant.kind, "assistant-step");
  assert.equal(assistant.data.status, "settled");
  assert.deepEqual(assistant.data.blocks, [{ kind: "text", text: "收到啦，渲染正常。" }]);
});

test("emits SessionEventLikeEntry windows for the assembler", () => {
  const entries = projectSessionEvents(MESSAGES);
  assert.ok(entries.length > 0);
  for (const entry of entries) {
    assert.equal(entry.type, "event");
    assert.equal(typeof entry.event.seq, "number");
    assert.equal(typeof entry.event.data, "object");
  }
  const types = entries.map((entry) => entry.event.type);
  assert.deepEqual(types, [
    "turn/start",
    "step/start",
    "user/message",
    "assistant/message",
    "step/end",
    "turn/end",
  ]);
  const userMessage = entries.find((entry) => entry.event.type === "user/message");
  assert.equal(userMessage.event.surfaceOp, "append");
  assert.deepEqual(userMessage.event.data.message.content, [
    { type: "text", text: "你好，这是一条渲染验证消息" },
  ]);
  const turnEnd = entries.at(-1).event;
  assert.deepEqual(turnEnd.data.reason, { kind: "completed" });
});

test("refreshes a registered conversation projection and reports the active view", async () => {
  const reads = [];
  const data = createConversationData({
    conversationId: "conversation-registry",
    command: async (input) => {
      reads.push(input.action);
      return { messages: MESSAGES, hasMore: false };
    },
  });
  assert.equal(data.conversationId, "conversation-registry");
  assert.equal(data.target("chat").getSnapshot().order.length, 0);

  await refreshConversationData("unknown-conversation");
  await data.refresh();
  assert.deepEqual(reads, ["conversation-read"]);
  assert.equal(data.target("chat").getSnapshot().order.length, 2);
  assert.equal(data.target("other"), undefined);

  data.activate("chat");
  assert.equal(data.activeView, "chat");
  assert.equal(data.session.getSnapshot().openState, "open");
  assert.equal(data.session.getSnapshot().ready, true);

  data.dispose();
  assert.equal(data.activeView, "chat");
});

test("sends through the host command and refreshes the projection", async () => {
  const calls = [];
  const data = createConversationData({
    conversationId: "conversation-send",
    command: async (input) => {
      calls.push([input.action, input.content ?? null]);
      if (input.action === "conversation-message-send") return { content: input.content, requestId: input.requestId };
      return { messages: MESSAGES, hasMore: false };
    },
  });
  await assert.rejects(() => data.send("   "), /发送内容不能为空/);
  const sent = await data.send("新消息");
  assert.equal(sent.content, "新消息");
  assert.deepEqual(calls.map((call) => call[0]), [
    "conversation-message-send",
    "conversation-read",
  ]);
  data.dispose();
});

import assert from "node:assert/strict";
import test from "node:test";

import { createDshClientRuntimeServices } from "../src/client-abi/runtime-services.mjs";
import {
  createConversationData,
  registerConversationData,
} from "../src/client-ui/conversation-data.mjs";
import {
  MAX_CONVERSATION_READ_LIMIT,
  createSessionEventFeed,
  nextConversationReadLimit,
} from "../src/client-ui/session-events.mjs";

function conversationMessages(total) {
  return Array.from({ length: total }, (_, index) => ({
    messageId: `message-${index + 1}`,
    conversationId: "conversation-paging",
    role: index % 2 === 0 ? "user" : "assistant",
    timestamp: new Date(Date.UTC(2026, 0, 1, 0, 0, index)).toISOString(),
    contentSummary: `第 ${index + 1} 条消息`,
  }));
}

function pagedCommand(total, calls) {
  const messages = conversationMessages(total);
  return async (input) => {
    if (input.action === "conversation-list") {
      return { items: [], workspaces: [], archivedSessionIds: [], pinnedSessionIds: [], hasMore: false };
    }
    if (input.action !== "conversation-read") throw new Error(`unexpected command ${input.action}`);
    calls.push(input);
    const limit = Number(input.limit);
    const offset = Number(input.offset ?? 0);
    const start = Math.max(0, messages.length - limit - offset);
    return { messages: messages.slice(start, messages.length - offset), hasMore: start > 0 };
  };
}

function flush() {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

test("会话事件窗口默认读取 100 条并按 loadOlder 翻倍提升", async () => {
  const calls = [];
  const feed = createSessionEventFeed({ conversationId: "conversation-paging", command: pagedCommand(250, calls) });
  await feed.refresh();
  assert.equal(calls.length, 1);
  assert.deepEqual(
    { action: calls[0].action, limit: calls[0].limit, offset: calls[0].offset },
    { action: "conversation-read", limit: 100, offset: 0 },
  );
  const firstWindow = feed.getSnapshot();
  assert.equal(feed.getLimit(), 100);
  assert.equal(firstWindow.hasMore, true);

  let notified = 0;
  feed.subscribe(() => { notified += 1; });
  const secondWindow = await feed.expandWindow();
  assert.equal(calls.at(-1).limit, 200);
  assert.equal(feed.getLimit(), 200);
  assert.ok(secondWindow.entries.length > firstWindow.entries.length);
  assert.ok(secondWindow.revision > firstWindow.revision);
  assert.ok(notified > 0);

  const thirdWindow = await feed.expandWindow();
  assert.equal(calls.at(-1).limit, 400);
  assert.ok(thirdWindow.entries.length > secondWindow.entries.length);
  assert.equal(thirdWindow.hasMore, false);
  feed.dispose();
});

test("读取窗口提升到上限后保持 2000 条", async () => {
  assert.equal(nextConversationReadLimit(100), 200);
  assert.equal(nextConversationReadLimit(1000), 2000);
  assert.equal(nextConversationReadLimit(2000), MAX_CONVERSATION_READ_LIMIT);
  assert.equal(nextConversationReadLimit(9000), MAX_CONVERSATION_READ_LIMIT);

  const calls = [];
  const feed = createSessionEventFeed({
    conversationId: "conversation-paging",
    limit: 1000,
    command: pagedCommand(2500, calls),
  });
  await feed.refresh();
  assert.equal(calls[0].limit, 1000);
  await feed.expandWindow();
  assert.equal(calls.at(-1).limit, 2000);
  await feed.expandWindow();
  assert.equal(calls.at(-1).limit, 2000);
  assert.equal(feed.getLimit(), MAX_CONVERSATION_READ_LIMIT);
  feed.dispose();
});

test("会话投影的 loadOlder 提升读取窗口并通知订阅者", async () => {
  const calls = [];
  const data = createConversationData({ conversationId: "conversation-paging", command: pagedCommand(250, calls) });
  await data.refresh();
  assert.equal(calls[0].limit, 100);
  assert.equal(data.getReadLimit(), 100);
  const firstSnapshot = data.target("chat").getSnapshot();
  assert.equal(firstSnapshot.order.length, 100);
  assert.equal(data.session.getSnapshot().hasMore, true);

  let notified = 0;
  const unsubscribe = data.chat.subscribe(() => { notified += 1; });
  const secondSnapshot = await data.loadOlder();
  assert.equal(calls.at(-1).limit, 200);
  assert.equal(data.getReadLimit(), 200);
  assert.equal(secondSnapshot.order.length, 200);
  assert.ok(notified > 0);

  const thirdSnapshot = await data.loadOlder();
  assert.equal(calls.at(-1).limit, 400);
  assert.equal(thirdSnapshot.order.length, 250);
  assert.equal(data.session.getSnapshot().hasMore, false);
  unsubscribe();
  data.dispose();
});

test("session.loadOlder 同步刷新事件窗口与会话投影并返回结果", async () => {
  const calls = [];
  const command = pagedCommand(250, calls);
  const services = createDshClientRuntimeServices({ command, navigate: () => {} });
  const data = createConversationData({ conversationId: "conversation-1", command });
  const unregister = registerConversationData("conversation-1", data);
  const binding = services.sessions.binding("conversation-1");
  await flush();
  assert.equal(binding.eventSource.getSnapshot().entries.length > 0, true);
  assert.equal(calls[0].limit, 100);

  const before = binding.eventSource.getSnapshot();
  let notified = 0;
  const unsubscribe = binding.eventSource.subscribe(() => { notified += 1; });
  const window = await binding.session.loadOlder();
  assert.deepEqual(
    calls.filter((call) => call.limit === 200).map((call) => call.action),
    ["conversation-read", "conversation-read"],
  );
  assert.equal(window.entries.length > before.entries.length, true);
  assert.equal(binding.eventSource.getSnapshot().revision > before.revision, true);
  assert.equal(data.getReadLimit(), 200);
  assert.ok(notified > 0);

  await binding.session.loadOlder();
  assert.equal(calls.at(-2).limit, 400);
  assert.equal(calls.at(-1).limit, 400);

  assert.equal(binding.session.consumeOlder(), false);
  assert.equal(binding.session.consumeOlder(), false);
  unsubscribe();
  unregister();
  data.dispose();
});

test("session.loadThrough 按 seq 截取窗口并在需要时扩展", async () => {
  const calls = [];
  const services = createDshClientRuntimeServices({ command: pagedCommand(400, calls), navigate: () => {} });
  const binding = services.sessions.binding("conversation-paging");
  await flush();
  const head = await binding.session.loadThrough(5);
  assert.equal(head.seq, 5);
  assert.equal(head.entries.length > 0, true);
  assert.equal(head.entries.every((entry) => Number(entry.event.seq) <= 5), true);

  const readsBefore = calls.filter((call) => call.action === "conversation-read").length;
  const deep = await binding.session.loadThrough(2000);
  assert.equal(deep.seq, 2000);
  assert.equal(deep.entries.every((entry) => Number(entry.event.seq) <= 2000), true);
  assert.equal(calls.filter((call) => call.action === "conversation-read").length > readsBefore, true);

  await assert.rejects(() => binding.session.loadThrough(-1), /需要非负整数 seq/);
});

test("loadOlder 宿主失败时原样抛出错误与返回体并回退窗口", async () => {
  const calls = [];
  const failure = new Error("宿主 conversation-read 失败：会话读取被拒绝");
  failure.response = { code: 403, message: "会话读取被拒绝", detail: { conversationId: "conversation-paging" } };
  const command = async (input) => {
    calls.push(input);
    if (input.limit >= 200) throw failure;
    return { messages: conversationMessages(10), hasMore: false };
  };
  const feed = createSessionEventFeed({ conversationId: "conversation-paging", command });
  await feed.refresh();
  await assert.rejects(
    () => feed.expandWindow(),
    (error) => {
      assert.equal(error, failure);
      assert.deepEqual(error.response, failure.response);
      return true;
    },
  );
  assert.equal(feed.getLimit(), 100);
  await feed.refresh();
  assert.equal(calls.at(-1).limit, 100);

  const services = createDshClientRuntimeServices({ command, navigate: () => {} });
  const binding = services.sessions.binding("conversation-paging");
  await flush();
  await assert.rejects(
    () => binding.session.loadOlder(),
    (error) => {
      assert.equal(error, failure);
      assert.deepEqual(error.response, failure.response);
      return true;
    },
  );
  feed.dispose();
});

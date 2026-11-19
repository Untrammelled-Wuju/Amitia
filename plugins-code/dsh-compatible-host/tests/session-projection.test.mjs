import assert from "node:assert/strict";
import test from "node:test";
import {
  buildSessionProjection,
  foldSessionSurface,
  foldSessionTitle,
  messagePlainText,
} from "../src/runtime/session-projection.mjs";

function conversationInput() {
  return {
    conversationId: "conversation-1",
    messages: [
      {
        id: "message-user-1",
        role: "user",
        contentSummary: "帮我看下 README",
        requestId: "request-1",
        attachments: [{ attachmentId: "artifact-1", name: "readme.md", bytes: 12, mediaType: "text/markdown", path: "/tmp/readme.md" }],
      },
      {
        id: "message-assistant-1",
        role: "assistant",
        contentSummary: "已读取文件",
        requestId: "request-1",
      },
    ],
    turns: [
      {
        turnId: "turn-1",
        turnSequence: 1,
        requestId: "request-1",
        status: "completed",
        blocks: [
          { blockId: "turn-1/text/0", type: "text", status: "completed", content: "已读取文件" },
          { blockId: "turn-1/tool/call-1", type: "tool_call", status: "completed", callId: "call-1", payload: { toolName: "fs_read", arguments: { path: "/tmp/readme.md" }, result: "readme body" } },
          { blockId: "turn-1/tool/call-2", type: "tool_call", status: "failed", callId: "call-2", payload: { toolName: "fs_write", arguments: { path: "/readonly" }, result: "permission denied" } },
        ],
      },
    ],
  };
}

test("buildSessionProjection 产出 DSH 契约事件：用户消息、助手多段内容、工具结果", () => {
  const projection = buildSessionProjection(conversationInput());
  assert.equal(projection.conversationId, "conversation-1");
  const types = projection.events.map((event) => event.type);
  assert.deepEqual(types, [
    "turn/start",
    "step/start",
    "user/message",
    "session/title",
    "assistant/message",
    "tool/result",
    "tool/result",
    "step/end",
    "turn/end",
  ]);
  const user = projection.events[2];
  assert.equal(user.data.role, "user");
  assert.equal(user.surfaceOp, "append");
  assert.deepEqual(user.data.content[0], { type: "text", text: "帮我看下 README" });
  const fileBlock = user.data.content[1];
  assert.equal(fileBlock.type, "file");
  assert.equal(fileBlock.attachment.attachmentId, "artifact-1");
  assert.equal(fileBlock.attachment.name, "readme.md");
  assert.equal(fileBlock.attachment.bytes, 12);
  assert.equal(fileBlock.attachment.mediaType, "text/markdown");
  assert.equal(fileBlock.attachment.uri, "/tmp/readme.md");

  const assistant = projection.events[4];
  assert.equal(assistant.data.message.role, "assistant");
  assert.deepEqual(assistant.data.message.content.map((block) => block.type), ["text", "tool-call", "tool-call"]);
  assert.equal(assistant.data.message.content[1].id, "call-1");
  assert.equal(assistant.data.message.content[1].name, "fs_read");
  assert.equal(assistant.data.message.content[1].arguments, JSON.stringify({ path: "/tmp/readme.md" }));

  const firstResult = projection.events[5];
  assert.equal(firstResult.data.message.role, "tool");
  assert.equal(firstResult.data.message.toolCallId, "call-1");
  assert.equal(firstResult.data.message.isError, undefined);
  assert.equal(firstResult.data.message.content[0].text, "readme body");
  const secondResult = projection.events[6];
  assert.equal(secondResult.data.message.isError, true);
  assert.equal(secondResult.data.message.content[0].text, "permission denied");
  assert.deepEqual(projection.events.map((event) => event.seq), [0, 1, 2, 3, 4, 5, 6, 7, 8]);
});

test("buildSessionProjection 在没有轮次块时回退到文本内容", () => {
  const projection = buildSessionProjection({
    conversationId: "conversation-2",
    messages: [
      { id: "u1", role: "user", contentSummary: "问题", requestId: "r1" },
      { id: "a1", role: "assistant", contentSummary: "回答", requestId: "r1" },
    ],
    turns: [],
  });
  const assistant = projection.events.find((event) => event.type === "assistant/message");
  assert.deepEqual(assistant.data.message.content, [{ type: "text", text: "回答" }]);
});

test("foldSessionSurface 只把有内容的消息放进表面", () => {
  const projection = buildSessionProjection(conversationInput());
  const folded = foldSessionSurface(projection.events);
  assert.deepEqual(folded.nodes, [2, 4, 5, 6]);
  assert.equal(folded.messages.get(2).role, "user");
  assert.equal(folded.messages.get(4).role, "assistant");
  assert.equal(folded.messages.get(5).role, "tool");
  assert.equal(messagePlainText(folded.messages.get(4)).includes("[tool fs_read call-1]"), true);
});

test("foldSessionSurface 支持 replace 覆盖既有表面区间", () => {
  const events = [
    { type: "user/message", seq: 0, time: 1, surfaceOp: "append", data: { id: "u1", role: "user", content: [{ type: "text", text: "问题" }], source: { kind: "user" } } },
    { type: "assistant/message", seq: 1, time: 2, surfaceOp: "append", data: { turn: 1, step: 0, stream: [], message: { id: "a1", role: "assistant", content: [{ type: "text", text: "旧回答" }], source: { kind: "model", provider: "p", model: "m" } } } },
    { type: "assistant/message", seq: 2, time: 3, surfaceOp: { op: "replace", startSeq: 1, endSeq: 1 }, data: { turn: 1, step: 0, stream: [], message: { id: "a2", role: "assistant", content: [{ type: "text", text: "新回答" }], source: { kind: "model", provider: "p", model: "m" } } } },
  ];
  const folded = foldSessionSurface(events);
  assert.deepEqual(folded.nodes, [0, 2]);
  assert.equal(folded.messages.has(1), false);
  assert.equal(folded.messages.get(2).content[0].text, "新回答");
  assert.deepEqual(folded.replacements, [{ seq: 2, startSeq: 1, endSeq: 1 }]);
});

test("foldSessionSurface 拒绝越界替换区间", () => {
  const events = [
    { type: "assistant/message", seq: 0, time: 1, surfaceOp: { op: "replace", startSeq: 5, endSeq: 6 }, data: { turn: 1, step: 0, stream: [], message: { id: "a1", role: "assistant", content: [{ type: "text", text: "x" }], source: { kind: "model", provider: "p", model: "m" } } } },
  ];
  assert.throws(() => foldSessionSurface(events), /不在当前表面上/);
});

test("foldSessionSurface 应用注册的消息投影并拒绝非法返回", () => {
  const projection = buildSessionProjection(conversationInput());
  const marker = {
    type: "assistant/message",
    project(event, context) {
      const updated = new Map();
      const current = context.messages.get(Number(event.seq));
      updated.set(Number(event.seq), Object.freeze({ ...current, content: Object.freeze([...current.content, Object.freeze({ type: "text", text: "投影追加" })]) }));
      return updated;
    },
  };
  const folded = foldSessionSurface(projection.events, [marker]);
  const assistant = folded.messages.get(4);
  assert.equal(assistant.content.at(-1).text, "投影追加");

  assert.throws(() => foldSessionSurface(projection.events, [{ type: "assistant/message", project: () => ({}) }]), /必须返回 Map/);
  assert.throws(() => foldSessionSurface(projection.events, [{ type: "assistant/message", project: () => new Map([[999, { id: "x" }]]) }]), /不在当前表面上/);
});

test("foldSessionTitle 取首条用户消息首行", () => {
  const projection = buildSessionProjection(conversationInput());
  assert.equal(foldSessionTitle(projection.events), "帮我看下 README");
  assert.equal(foldSessionTitle([]), "");
});

test("buildSessionProjection 在没有配对消息时仍从轮次块产出助手消息与工具结果", () => {
  const projection = buildSessionProjection({
    conversationId: "conversation-3",
    messages: [{ id: "u1", role: "user", contentSummary: "算一下", requestId: "r1" }],
    turns: [
      {
        turnId: "turn-9",
        turnSequence: 0,
        requestId: "r1",
        status: "completed",
        blocks: [
          { blockId: "turn-9/text/0", type: "text", status: "completed", content: "结果是 15" },
          { blockId: "turn-9/tool/c1", type: "tool_call", status: "completed", callId: "c1", payload: { toolName: "calculate", arguments: { expression: "(7*8+19)/5" }, result: "15" } },
        ],
      },
    ],
  });
  const folded = foldSessionSurface(projection.events);
  const messages = [...folded.messages.values()];
  assert.deepEqual(messages.map((message) => message.role), ["user", "assistant", "tool"]);
  assert.deepEqual(messages[1].content.map((block) => block.type), ["text", "tool-call"]);
  assert.equal(messages[2].toolCallId, "c1");
  assert.equal(messages[2].content[0].text, "15");
});

test("buildSessionProjection 按时间把倒序返回的消息摆成时间正序", () => {
  const projection = buildSessionProjection({
    conversationId: "conversation-4",
    messages: [
      { id: "a1", role: "assistant", contentSummary: "回答", requestId: "r1", timestamp: "2026-11-18T17:35:22Z" },
      { id: "u1", role: "user", contentSummary: "提问", requestId: "r1", timestamp: "2026-11-18T17:32:19Z" },
    ],
    turns: [],
  });
  const folded = foldSessionSurface(projection.events);
  assert.deepEqual([...folded.messages.values()].map((message) => message.role), ["user", "assistant"]);
});

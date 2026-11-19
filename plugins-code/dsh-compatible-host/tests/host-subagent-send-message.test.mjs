import assert from "node:assert/strict";
import test from "node:test";
import { createAmitiaDshHostServices } from "../src/runtime/host-services.mjs";

function createSubagentHost({ conversations, targeted } = {}) {
  const calls = [];
  const host = {
    async call(method, input = {}) {
      calls.push({ method, input });
      if (method === "host.conversation.list") return { items: conversations ?? [], workspaces: [], hasMore: false };
      if (method === "host.state.get") {
        const key = String(input.key ?? "");
        if (key.startsWith("dsh-session:")) return { found: true, value: { scopeSnapshotId: "snapshot-" + key.slice("dsh-session:".length) } };
        return { found: false };
      }
      return {};
    },
    async callTargeted(method, input, scopeSnapshotId) {
      if (targeted) targeted.push({ method, input, scopeSnapshotId });
      return { messageId: "message-1" };
    },
  };
  return { host, calls };
}

const TREE = [
  { sessionId: "root", title: "Root", createdAt: "2026-11-01T10:00:00Z" },
  { sessionId: "child", title: "Child", createdAt: "2026-11-01T10:01:00Z", parentSession: "root" },
  { sessionId: "grandchild", title: "Grandchild", createdAt: "2026-11-01T10:02:00Z", parentSession: "child" },
  { sessionId: "stranger", title: "Stranger", createdAt: "2026-11-01T10:03:00Z" },
];

test("sendMessage 向同树后代投递文本并返回消息 id", async () => {
  const targeted = [];
  const { host } = createSubagentHost({ conversations: TREE, targeted });
  const services = createAmitiaDshHostServices(host);
  const messageId = await services.subagents.sendMessage({ session: { id: "root" } }, "grandchild", [{ type: "text", text: "steer left" }], {});
  assert.equal(messageId, "message-1");
  assert.equal(targeted.length, 1);
  assert.equal(targeted[0].method, "host.conversation.message.append");
  assert.equal(targeted[0].scopeSnapshotId, "snapshot-grandchild");
  assert.equal(targeted[0].input.conversationId, "grandchild");
  assert.equal(targeted[0].input.source, "dsh-subagent-steer");
  assert.deepEqual(targeted[0].input.parts, [{ type: "text", content: "steer left" }]);
  assert.match(targeted[0].input.requestId, /^dsh-subagent-steer:grandchild:root:/);
});

test("sendMessage 允许子代理向直接父级投递", async () => {
  const targeted = [];
  const { host } = createSubagentHost({ conversations: TREE, targeted });
  const services = createAmitiaDshHostServices(host);
  await services.subagents.sendMessage({ session: { id: "child" } }, "root", [{ type: "text", text: "done" }], {});
  assert.equal(targeted[0].input.conversationId, "root");
});

test("sendMessage 拒绝跨树目标与未知发送者", async () => {
  const targeted = [];
  const { host } = createSubagentHost({ conversations: TREE, targeted });
  const services = createAmitiaDshHostServices(host);
  await assert.rejects(
    services.subagents.sendMessage({ session: { id: "root" } }, "stranger", [{ type: "text", text: "hi" }], {}),
    /拒绝跨子代理树投递/,
  );
  await services.subagents.sendMessage({ session: { id: "child" } }, "grandchild", [{ type: "text", text: "go deeper" }], {});
  await assert.rejects(
    services.subagents.sendMessage({ session: { id: "stranger" } }, "root", [{ type: "text", text: "hi" }], {}),
    /拒绝跨子代理树投递/,
  );
  await assert.rejects(services.subagents.sendMessage({}, "root", [{ type: "text", text: "hi" }], {}), /无法从 sender 解析会话 id/);
  await assert.rejects(services.subagents.sendMessage({ session: { id: "root" } }, "", [{ type: "text", text: "hi" }], {}), /需要目标会话 id/);
  assert.equal(targeted.length, 1);
  assert.equal(targeted[0].input.conversationId, "grandchild");
});

test("sendMessage 拒绝空片段、非文本片段与已取消信号", async () => {
  const targeted = [];
  const { host } = createSubagentHost({ conversations: TREE, targeted });
  const services = createAmitiaDshHostServices(host);
  await assert.rejects(services.subagents.sendMessage({ session: { id: "root" } }, "child", [], {}), /需要至少一个内容片段/);
  await assert.rejects(
    services.subagents.sendMessage({ session: { id: "root" } }, "child", [{ type: "image", data: "x" }], {}),
    /只接受文本片段/,
  );
  const aborted = new AbortController();
  aborted.abort(new Error("cancelled"));
  await assert.rejects(services.subagents.sendMessage({ session: { id: "root" } }, "child", [{ type: "text", text: "hi" }], { signal: aborted.signal }), /cancelled/);
  assert.equal(targeted.length, 0);
});

test("sendMessage 在血缘成环时原样报错", async () => {
  const cyclic = [
    { sessionId: "a", createdAt: "2026-11-01T10:00:00Z", parentSession: "b" },
    { sessionId: "b", createdAt: "2026-11-01T10:01:00Z", parentSession: "a" },
  ];
  const { host } = createSubagentHost({ conversations: cyclic, targeted: [] });
  const services = createAmitiaDshHostServices(host);
  await assert.rejects(services.subagents.sendMessage({ session: { id: "a" } }, "a", [{ type: "text", text: "hi" }], {}), /会话血缘存在环/);
});

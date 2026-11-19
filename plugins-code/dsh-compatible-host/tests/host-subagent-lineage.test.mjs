import assert from "node:assert/strict";
import test from "node:test";
import { createAmitiaDshHostServices } from "../src/runtime/host-services.mjs";

const TREE = [
  { sessionId: "root", title: "Root", createdAt: "2026-11-01T10:00:00Z" },
  { sessionId: "child", title: "Child", createdAt: "2026-11-01T10:01:00Z", parentSession: "root" },
  { sessionId: "grandchild", title: "Grandchild", createdAt: "2026-11-01T10:02:00Z", parentSession: "child" },
];

function createLineageHost(conversations = TREE) {
  const host = {
    async call(method, input = {}) {
      if (method === "host.conversation.list") return { items: conversations, workspaces: [], hasMore: false };
      if (method === "host.conversation.create") return { conversationId: input.conversationId, scopeSnapshotId: "snapshot-" + input.conversationId };
      if (method === "host.state.get") {
        const key = String(input.key ?? "");
        if (key.startsWith("dsh-session:")) return { found: false };
        return { found: false };
      }
      if (method === "host.conversation.read") return { messages: [], hasMore: false };
      if (method === "host.conversation.turns") return { turns: [] };
      if (method === "host.conversation.scope") return { scopeSnapshotId: "scope-" + String(input.conversationId || "") };
      if (method === "host.state.cas") return { swapped: true, version: Number(input.expectedVersion || 0) + 1 };
      return {};
    },
    async callTargeted() {
      return { messageId: "message-1" };
    },
  };
  return host;
}

async function createRootAgent(services) {
  const handle = await services.agents.create({ sessionId: "root" });
  const dispose = services.subagents.registerProvider({
    name: "warmup",
    async start() {
      return { id: "warmup-child", messageId: "message-0", result: {}, dispose: async () => {} };
    },
  });
  await services.subagents.startActivation({ provider: "warmup", name: "warmup" });
  dispose();
  return handle;
}

test("rootOf 按会话血缘解析树根并返回活动 agent", async () => {
  const services = createAmitiaDshHostServices(createLineageHost());
  const handle = await createRootAgent(services);
  const root = services.subagents.rootOf({ session: { id: "child" } });
  assert.equal(root.id ?? root.session?.id, "root");
  assert.equal(services.subagents.rootOf({ session: { id: "root" } }).session?.id ?? "root", "root");
  assert.equal(handle.agent !== undefined || handle.session !== undefined, true);
  assert.throws(() => services.subagents.rootOf({}), /需要带会话 id 的 caller/);
});

test("rootOf 在树根不在活动 agent 表时原样报错", async () => {
  const services = createAmitiaDshHostServices(createLineageHost());
  await createRootAgent(services);
  const orphan = createAmitiaDshHostServices(createLineageHost());
  await orphan.agents.create({ sessionId: "child" });
  const dispose = orphan.subagents.registerProvider({
    name: "warm",
    async start() {
      return { id: "warm-child", messageId: "message-0", result: {}, dispose: async () => {} };
    },
  });
  await orphan.subagents.startActivation({ provider: "warm", name: "warm" });
  dispose();
  assert.throws(() => orphan.subagents.rootOf({ session: { id: "child" } }), /无法解析|不在活动/);
  assert.throws(() => orphan.subagents.rootOf({ session: { id: "grandchild" } }), /无法解析|不在活动/);
  const live = services.subagents.rootOf({ session: { id: "grandchild" } });
  assert.equal(live.session?.id ?? live.id, "root");
});

test("namesOf 返回名字路径与直接创建者名字", async () => {
  const services = createAmitiaDshHostServices(createLineageHost());
  await createRootAgent(services);
  const dispose = services.subagents.registerProvider({
    name: "named",
    async start() {
      return { id: "child", messageId: "message-1", result: {}, dispose: async () => {} };
    },
  });
  await services.subagents.startActivation({ provider: "named", name: "worker" });
  assert.deepEqual(services.subagents.namesOf({ session: { id: "root" } }), { self: "lead" });
  assert.deepEqual(services.subagents.namesOf({ session: { id: "child" } }), { self: "lead/worker", parent: "lead" });
  assert.deepEqual(services.subagents.namesOf({ session: { id: "grandchild" } }), { self: "grandchild", parent: "lead/worker" });
  dispose();
});

test("namesOf 在血缘缓存未加载时原样报错", () => {
  const services = createAmitiaDshHostServices(createLineageHost());
  assert.throws(() => services.subagents.namesOf({ session: { id: "child" } }), /尚未加载会话血缘/);
  assert.throws(() => services.subagents.rootOf({ session: { id: "child" } }), /尚未加载会话血缘/);
  assert.throws(() => services.subagents.namesOf({}), /需要带会话 id 的 agent/);
});

test("namesOf 血缘成环时报错", async () => {
  const cyclic = [
    { sessionId: "a", createdAt: "2026-11-01T10:00:00Z", parentSession: "b" },
    { sessionId: "b", createdAt: "2026-11-01T10:01:00Z", parentSession: "a" },
  ];
  const services = createAmitiaDshHostServices(createLineageHost(cyclic));
  await services.agents.create({ sessionId: "a" });
  const dispose = services.subagents.registerProvider({
    name: "warm",
    async start() {
      return { id: "warm-child", messageId: "message-0", result: {}, dispose: async () => {} };
    },
  });
  await services.subagents.startActivation({ provider: "warm", name: "warm" });
  dispose();
  assert.throws(() => services.subagents.namesOf({ session: { id: "a" } }), /会话血缘存在环/);
});

import assert from "node:assert/strict";
import test from "node:test";
import { createAmitiaDshHostServices } from "../src/runtime/host-services.mjs";

const EVENTS = [
  { type: "turn/start", seq: 0, time: 1762000000000, data: { turn: 1 } },
  {
    type: "user/message",
    seq: 1,
    time: 1762000000001,
    data: { message: { id: "m1" }, content: [{ type: "text", text: "hello plan" }] },
    surfaceOp: "append",
  },
  {
    type: "user/message",
    seq: 2,
    time: 1762000000002,
    data: { message: { id: "m2" }, content: [{ type: "text", text: "hello again" }] },
    surfaceOp: { op: "replace", startSeq: 1, endSeq: 1 },
    sourceEventSeqs: [1],
  },
  { type: "session/title", seq: 3, time: 1762000000003, data: { title: "Quarterly plan", messageSeqs: [2], source: "user" } },
];

function createSessionHost() {
  const calls = [];
  const conversations = [
    { sessionId: "session-1", title: "Quarterly plan", createdAt: "2026-11-01T10:00:00Z", cwd: "/workspace/plan" },
    { sessionId: "session-2", title: "", createdAt: "2026-11-02T11:00:00Z" },
    { sessionId: "session-3", title: "Fork of plan", createdAt: "2026-11-03T12:00:00Z", parentSession: "session-1" },
  ];
  const state = {
    "dsh-session:session-1": {
      version: 2,
      conversationId: "session-1",
      scopeSnapshotId: "",
      events: EVENTS,
      updatedAt: "2026-11-01T10:00:00.000Z",
    },
  };
  const host = {
    calls,
    async call(method, input = {}) {
      calls.push({ method, input });
      if (method === "host.conversation.list") return { items: conversations, workspaces: [], hasMore: false };
      if (method === "host.state.get") {
        const value = state[input.key];
        return value === undefined ? { found: false } : { found: true, value };
      }
      if (method === "host.conversation.read") return { messages: [], hasMore: false };
      if (method === "host.conversation.turns") return { turns: [] };
      if (method === "host.conversation.scope") return { scopeSnapshotId: "scope-" + String(input.conversationId || "") };
      return {};
    },
  };
  return { host, calls };
}

test("observeSession 返回持久化观察租约并支持 retain 与释放", async () => {
  const { host } = createSessionHost();
  const services = createAmitiaDshHostServices(host);
  const observation = await services.sessionQuery.observeSession("session-1");
  assert.equal(observation.source, "persisted");
  assert.equal(observation.header.id, "session-1");
  assert.equal(observation.header.cwd, "/workspace/plan");
  assert.equal(observation.inheritedEventCount, 0);
  assert.deepEqual(observation.events.map((event) => event.seq), [0, 1, 2, 3]);
  assert.equal(observation.cursor, 3);
  const retained = observation.retain();
  assert.deepEqual(retained.events.map((event) => event.seq), [0, 1, 2, 3]);
  observation[Symbol.dispose]();
  assert.throws(() => observation.retain(), /is disposed/);
  await assert.rejects(services.sessionQuery.observeSession("session-missing"), /not found|不存在/);
});

test("filterSessions 支持 id/cwd/created-at/parent/availability 谓词并拒绝未知谓词", async () => {
  const { host } = createSessionHost();
  const services = createAmitiaDshHostServices(host);
  const ids = async (filters) => (await services.sessionQuery.filterSessions(filters)).map((record) => record.header.id);
  assert.deepEqual(await ids([{ kind: "id", values: ["session-1", "session-3"] }]), ["session-1", "session-3"]);
  assert.deepEqual(await ids([{ kind: "cwd", values: ["/workspace/plan"] }]), ["session-1"]);
  assert.deepEqual(await ids([{ kind: "parent", values: ["session-1"] }]), ["session-3"]);
  assert.deepEqual(await ids([{ kind: "parent", values: [null] }]), ["session-1", "session-2"]);
  assert.deepEqual(await ids([{ kind: "created-at", from: Date.parse("2026-11-02T00:00:00Z") }]), ["session-2", "session-3"]);
  assert.deepEqual(await ids([{ kind: "availability", values: ["persisted"] }]), ["session-1", "session-2", "session-3"]);
  assert.deepEqual(await ids([{ kind: "availability", values: ["live"] }]), []);
  await assert.rejects(services.sessionQuery.filterSessions([{ kind: "unknown" }]), (error) => {
    assert.equal(error.code, "DSH_SESSION_FILTER_INVALID");
    return true;
  });
  await assert.rejects(services.sessionQuery.filterSessions("nope"), (error) => {
    assert.equal(error.code, "DSH_SESSION_FILTER_INVALID");
    return true;
  });
});

test("filterEvents 按 seq/type/surface/text 过滤并跳过无文本事件", async () => {
  const { host } = createSessionHost();
  const services = createAmitiaDshHostServices(host);
  const all = await services.sessionQuery.filterEvents("session-1", [{ kind: "seq", from: 0 }]);
  assert.deepEqual(all.map((document) => [document.seq, document.surface]), [[1, "shadowed"], [2, "current"]]);
  const byType = await services.sessionQuery.filterEvents("session-1", [{ kind: "type", values: ["user/message"] }]);
  assert.deepEqual(byType.map((document) => document.seq), [1, 2]);
  const bySurface = await services.sessionQuery.filterEvents("session-1", [{ kind: "surface", values: ["current"] }]);
  assert.deepEqual(bySurface.map((document) => document.seq), [2]);
  const byText = await services.sessionQuery.filterEvents("session-1", [{ kind: "text", text: "HELLO   plan" }]);
  assert.deepEqual(byText.map((document) => [document.seq, document.text]), [[1, "hello plan"]]);
  await assert.rejects(services.sessionQuery.filterEvents("session-1", [{ kind: "surface", values: ["bogus"] }]), (error) => {
    assert.equal(error.code, "DSH_SESSION_FILTER_INVALID");
    return true;
  });
});

test("traceSession 用 parentSession 构建祖先与后代血缘", async () => {
  const { host } = createSessionHost();
  const services = createAmitiaDshHostServices(host);
  const childTrace = await services.sessionQuery.traceSession("session-3");
  assert.equal(childTrace.complete, true);
  assert.deepEqual(childTrace.ancestors.map((record) => record.header.id), ["session-1"]);
  assert.equal(childTrace.root.header.id, "session-1");
  assert.deepEqual(childTrace.descendants, []);
  const rootTrace = await services.sessionQuery.traceSession("session-1");
  assert.equal(rootTrace.complete, true);
  assert.deepEqual(rootTrace.ancestors, []);
  assert.deepEqual(rootTrace.descendants.map((node) => node.record.header.id), ["session-3"]);
  await assert.rejects(services.sessionQuery.traceSession("session-missing"), (error) => {
    assert.equal(error.code, "DSH_SESSION_NOT_FOUND");
    return true;
  });
});

test("readEvent 接受 request 对象并返回目标事件与前后窗口", async () => {
  const { host } = createSessionHost();
  const services = createAmitiaDshHostServices(host);
  const window = await services.sessionQuery.readEvent({ sessionId: "session-1", seq: 2, before: 1, after: 0 });
  assert.equal(window.session.id, "session-1");
  assert.equal(window.target.seq, 2);
  assert.deepEqual(window.events.map((event) => event.seq), [1, 2]);
  assert.equal(window.startSeq, 1);
  assert.equal(window.endSeq, 2);
  assert.equal(window.inheritedEventCount, 0);
  const full = await services.sessionQuery.readEvent({ sessionId: "session-1", seq: 0 });
  assert.deepEqual(full.events.map((event) => event.seq), [0]);
  await assert.rejects(services.sessionQuery.readEvent({ seq: 1 }), (error) => {
    assert.equal(error.code, "DSH_SESSION_ID_REQUIRED");
    return true;
  });
  await assert.rejects(services.sessionQuery.readEvent({ sessionId: "session-1", seq: 99 }), (error) => {
    assert.equal(error.code, "DSH_SESSION_EVENT_NOT_FOUND");
    return true;
  });
  await assert.rejects(services.sessionQuery.readEvent({ sessionId: "session-1", seq: 1, before: 999 }), (error) => {
    assert.equal(error.code, "DSH_SESSION_EVENT_WINDOW_INVALID");
    return true;
  });
});

test("traceEvent 返回替换链、来源与派生事件", async () => {
  const { host } = createSessionHost();
  const services = createAmitiaDshHostServices(host);
  const shadowed = await services.sessionQuery.traceEvent({ sessionId: "session-1", seq: 1 });
  assert.equal(shadowed.session.id, "session-1");
  assert.equal(shadowed.target.seq, 1);
  assert.equal(shadowed.target.surface, "shadowed");
  assert.equal(shadowed.replacedBy, 2);
  assert.deepEqual(shadowed.replacementChain, [2]);
  assert.deepEqual(shadowed.replacedEventSeqs, []);
  assert.deepEqual(shadowed.sourceEventSeqs, []);
  assert.deepEqual(shadowed.derivedEventSeqs, [2]);
  const replacer = await services.sessionQuery.traceEvent({ sessionId: "session-1", seq: 2 });
  assert.equal(replacer.target.surface, "current");
  assert.equal(replacer.replacedBy, undefined);
  assert.deepEqual(replacer.replacementChain, []);
  assert.deepEqual(replacer.replacedEventSeqs, [1]);
  assert.deepEqual(replacer.sourceEventSeqs, [1]);
  assert.deepEqual(replacer.derivedEventSeqs, []);
  const logOnly = await services.sessionQuery.traceEvent({ sessionId: "session-1", seq: 0 });
  assert.equal(logOnly.target.surface, "log-only");
});

test("traceEvent 拒绝缺失事件、非法 seq 与缺失 sessionId", async () => {
  const { host } = createSessionHost();
  const services = createAmitiaDshHostServices(host);
  await assert.rejects(services.sessionQuery.traceEvent({ sessionId: "session-1", seq: 99 }), /has no event seq 99/);
  await assert.rejects(services.sessionQuery.traceEvent({ sessionId: "session-1", seq: -1 }), /需要非负整数 seq/);
  await assert.rejects(services.sessionQuery.traceEvent({ seq: 1 }), /需要 sessionId/);
});

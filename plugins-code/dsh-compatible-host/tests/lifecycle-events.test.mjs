import assert from "node:assert/strict";
import test from "node:test";
import { createAmitiaDshHostServices } from "../src/runtime/host-services.mjs";
import { LIFECYCLE_EVENTS, SURFACE_EVENT_TYPES } from "../src/runtime/lifecycle-events.generated.mjs";
import {
  SESSION_EVENT_LIMIT,
  SESSION_STATE_VERSION,
  assertSessionStateVersion,
  assertSurfaceReplacement,
  createSessionEvent,
  isRegisteredLifecycleEvent,
  isSurfaceEventType,
  projectSurfaceEvents,
  restoreSessionEvents,
} from "../src/runtime/session-events.mjs";

function createStateHost(initial = new Map()) {
  const records = new Map(initial);
  const calls = [];
  return {
    calls,
    records,
    async call(method, input) {
      calls.push({ method, input });
      if (method === "host.state.get") {
        const record = records.get(input.key);
        return record
          ? { found: true, value: record.value, version: record.version }
          : { found: false, version: 0 };
      }
      if (method === "host.state.cas") {
        const record = records.get(input.key);
        const current = record?.version ?? 0;
        if (Number(input.expectedVersion) !== current) return { swapped: false, version: current };
        const next = current + 1;
        records.set(input.key, { value: structuredClone(input.newValue), version: next });
        return { swapped: true, version: next };
      }
      if (method === "host.state.delete") {
        records.delete(input.key);
        return { ok: true };
      }
      throw new Error(`unexpected host method ${method}`);
    },
  };
}

function eventError(code) {
  return (error) => error instanceof Error && error.code === code;
}

test("注册官方 persistence catalog 的全部 63 项会话事件与 5 项 surface 事件", () => {
  assert.equal(LIFECYCLE_EVENTS.length, 63);
  assert.equal(SURFACE_EVENT_TYPES.length, 5);
  assert.deepEqual([...SURFACE_EVENT_TYPES].sort(), [
    "assistant/message",
    "developer/message",
    "system/message",
    "tool/result",
    "user/message",
  ]);
  for (const entry of LIFECYCLE_EVENTS) {
    assert.match(entry.id, /^DSH-HOST-LIFECYCLE-\d{3}$/);
    assert.equal(isRegisteredLifecycleEvent(entry.name), true);
    assert.equal(isSurfaceEventType(entry.name), entry.lifecycle === "surface");
  }
  assert.equal(isRegisteredLifecycleEvent("agent/echo"), false);
});

test("createSessionEvent 生成官方 SessionEvent 信封", () => {
  const event = createSessionEvent("turn/start", { turn: 1 }, { seq: 4, time: 1730000000000 });
  assert.deepEqual(event, { type: "turn/start", seq: 4, time: 1730000000000, data: { turn: 1 } });
  assert.equal(Object.isFrozen(event), true);
  assert.equal(Object.prototype.hasOwnProperty.call(event, "at"), false);

  const surface = createSessionEvent("user/message", { role: "user" }, { seq: 0, time: 1 });
  assert.deepEqual(surface.surfaceOp, "append");
  assert.equal(Object.prototype.hasOwnProperty.call(surface, "sourceEventSeqs"), false);

  const ignorable = createSessionEvent("plugin/custom", { note: "x" }, { seq: 1, time: 1, ignorable: true });
  assert.equal(ignorable.ignorable, true);
});

test("createSessionEvent 对信封字段做显式校验", () => {
  assert.throws(() => createSessionEvent("", {}, { seq: 0, time: 1 }), eventError("DSH_SESSION_EVENT_INVALID_TYPE"));
  assert.throws(() => createSessionEvent("turn/start", {}, { seq: -1, time: 1 }), eventError("DSH_SESSION_EVENT_INVALID_SEQ"));
  assert.throws(() => createSessionEvent("turn/start", {}, { seq: 0, time: 0 }), eventError("DSH_SESSION_EVENT_INVALID_TIME"));
  assert.throws(() => createSessionEvent("turn/start", "x", { seq: 0, time: 1 }), eventError("DSH_SESSION_EVENT_INVALID_DATA"));
  assert.throws(() => createSessionEvent("turn/start", [], { seq: 0, time: 1 }), eventError("DSH_SESSION_EVENT_INVALID_DATA"));
});

test("未声明的必需事件被拒绝，ignorable 才允许写入", () => {
  assert.throws(
    () => createSessionEvent("agent/echo", { value: 1 }, { seq: 0, time: 1 }),
    eventError("DSH_SESSION_EVENT_UNDECLARED"),
  );
  assert.throws(
    () => createSessionEvent("agent/echo", { value: 1 }, { seq: 0, time: 1, ignorable: false }),
    eventError("DSH_SESSION_EVENT_UNDECLARED"),
  );
});

test("log-only 事件禁止携带 surface 元数据", () => {
  assert.throws(
    () => createSessionEvent("turn/start", { turn: 1 }, { seq: 0, time: 1, surfaceOp: "append" }),
    eventError("DSH_SESSION_EVENT_SURFACE_OP_FORBIDDEN"),
  );
  assert.throws(
    () => createSessionEvent("turn/start", { turn: 1 }, { seq: 0, time: 1, sourceEventSeqs: [0] }),
    eventError("DSH_SESSION_EVENT_SOURCE_SEQS_FORBIDDEN"),
  );
});

test("surface replace 只接受合法区间与完整来源序列", () => {
  assert.throws(
    () => createSessionEvent("system/message", {}, { seq: 2, time: 1, surfaceOp: { op: "merge", startSeq: 0, endSeq: 1 } }),
    eventError("DSH_SESSION_EVENT_INVALID_SURFACE_OP"),
  );
  assert.throws(
    () => createSessionEvent("system/message", {}, { seq: 2, time: 1, surfaceOp: { op: "replace", startSeq: 1, endSeq: 0 } }),
    eventError("DSH_SESSION_EVENT_INVALID_SURFACE_RANGE"),
  );
  assert.throws(
    () => createSessionEvent("system/message", {}, { seq: 2, time: 1, sourceEventSeqs: ["0"] }),
    eventError("DSH_SESSION_EVENT_INVALID_SOURCE_SEQS"),
  );
});

test("replace 要求区间端点是当前 surface 节点且来源序列覆盖被遮蔽节点", () => {
  const surface = [
    createSessionEvent("user/message", { role: "user" }, { seq: 0, time: 1 }),
    createSessionEvent("assistant/message", { role: "assistant" }, { seq: 1, time: 2 }),
  ];
  const valid = createSessionEvent("system/message", { role: "system" }, {
    seq: 2,
    time: 3,
    surfaceOp: { op: "replace", startSeq: 0, endSeq: 1 },
    sourceEventSeqs: [0, 1],
  });
  assert.doesNotThrow(() => assertSurfaceReplacement(surface, valid));

  const partial = createSessionEvent("system/message", { role: "system" }, {
    seq: 2,
    time: 3,
    surfaceOp: { op: "replace", startSeq: 0, endSeq: 1 },
    sourceEventSeqs: [0],
  });
  assert.throws(() => assertSurfaceReplacement(surface, partial), eventError("DSH_SESSION_EVENT_SOURCE_SEQS_INCOMPLETE"));

  const missing = createSessionEvent("system/message", { role: "system" }, {
    seq: 2,
    time: 3,
    surfaceOp: { op: "replace", startSeq: 0, endSeq: 4 },
    sourceEventSeqs: [0, 1, 2, 3, 4],
  });
  assert.throws(() => assertSurfaceReplacement(surface, missing), eventError("DSH_SESSION_EVENT_SURFACE_RANGE_MISSING"));
});

test("projectSurfaceEvents 按 surfaceOp 投影有序 surface", () => {
  const events = [
    createSessionEvent("user/message", { role: "user" }, { seq: 0, time: 1 }),
    createSessionEvent("turn/start", { turn: 1 }, { seq: 1, time: 2 }),
    createSessionEvent("assistant/message", { role: "assistant" }, { seq: 2, time: 3 }),
    createSessionEvent("system/message", { role: "system" }, {
      seq: 3,
      time: 4,
      surfaceOp: { op: "replace", startSeq: 0, endSeq: 2 },
      sourceEventSeqs: [0, 2],
    }),
    createSessionEvent("assistant/message", { role: "assistant" }, { seq: 4, time: 5 }),
  ];
  assert.deepEqual(projectSurfaceEvents(events).map((event) => event.seq), [3, 4]);
});

test("restoreSessionEvents 拒绝未声明的必需事件并保留 ignorable 事件", () => {
  const required = [
    { type: "plugin/unknown", seq: 0, time: 1, data: {} },
  ];
  assert.throws(() => restoreSessionEvents(required, "session-required"), eventError("DSH_SESSION_EVENT_UNRECOGNIZED_REQUIRED"));

  const ignorable = [
    { type: "plugin/unknown", seq: 0, time: 1, data: {}, ignorable: true },
    { type: "turn/start", seq: 1, time: 2, data: { turn: 1 } },
  ];
  const restored = restoreSessionEvents(ignorable, "session-ignorable");
  assert.deepEqual(restored.events.map((event) => event.seq), [0, 1]);
  assert.equal(restored.tolerated, 1);
});

test("restoreSessionEvents 拒绝缺少 type、seq 或 time 的记录", () => {
  assert.throws(
    () => restoreSessionEvents([{ type: "turn/start", seq: 0, data: {} }], "session-broken"),
    eventError("DSH_SESSION_EVENT_INVALID_RECORD"),
  );
  assert.throws(
    () => restoreSessionEvents([null], "session-broken"),
    eventError("DSH_SESSION_EVENT_INVALID_RECORD"),
  );
});

test("assertSessionStateVersion 拒绝旧版本与缺失版本", () => {
  assert.throws(
    () => assertSessionStateVersion({ version: 1, events: [] }, "session-old"),
    eventError("DSH_SESSION_STATE_VERSION_UNSUPPORTED"),
  );
  assert.throws(
    () => assertSessionStateVersion({ events: [] }, "session-missing"),
    eventError("DSH_SESSION_STATE_VERSION_UNSUPPORTED"),
  );
  assert.doesNotThrow(() => assertSessionStateVersion({ version: SESSION_STATE_VERSION, events: [] }, "session-current"));
});

test("会话桥按官方信封持久化、恢复并投影 surface", async () => {
  const host = createStateHost();
  const services = createAmitiaDshHostServices(host);
  const agent = services.agents.agentForTool("session-surface");
  await agent.session.hydrated;
  agent.session.append("user/message", { role: "user", content: [] });
  agent.session.append("assistant/message", { role: "assistant", content: [] });
  agent.session.append("system/message", { role: "system", content: [] }, {
    surfaceOp: { op: "replace", startSeq: 0, endSeq: 1 },
    sourceEventSeqs: [0, 1],
  });
  await agent.session.whenPersisted();

  const stored = host.records.get("dsh-session:session-surface").value;
  assert.equal(stored.version, SESSION_STATE_VERSION);
  assert.equal(stored.conversationId, "session-surface");
  assert.equal(stored.events.length, 3);
  for (const event of stored.events) {
    assert.equal(Number.isSafeInteger(event.seq), true);
    assert.equal(Number.isSafeInteger(event.time), true);
    assert.equal(Object.prototype.hasOwnProperty.call(event, "at"), false);
  }
  assert.deepEqual(stored.events.map((event) => event.seq), [0, 1, 2]);
  assert.deepEqual(agent.session.surfaceEvents().map((event) => event.seq), [2]);

  const reopened = createAmitiaDshHostServices(host);
  const reopenedAgent = reopened.agents.agentForTool("session-surface");
  await reopenedAgent.session.hydrated;
  assert.deepEqual(reopenedAgent.session.events.map((event) => event.seq), [0, 1, 2]);
  assert.deepEqual(reopenedAgent.session.surfaceEvents().map((event) => event.seq), [2]);
  assert.equal(reopenedAgent.session.getSnapshot().version, SESSION_STATE_VERSION);
});

test("会话桥拒绝在 surface 上写入非法 replace", async () => {
  const host = createStateHost();
  const services = createAmitiaDshHostServices(host);
  const agent = services.agents.agentForTool("session-invalid-replace");
  await agent.session.hydrated;
  agent.session.append("user/message", { role: "user" });
  assert.throws(
    () => agent.session.append("system/message", { role: "system" }, {
      surfaceOp: { op: "replace", startSeq: 0, endSeq: 0 },
      sourceEventSeqs: [],
    }),
    eventError("DSH_SESSION_EVENT_SOURCE_SEQS_INCOMPLETE"),
  );
  assert.throws(
    () => agent.session.append("system/message", { role: "system" }, {
      surfaceOp: { op: "replace", startSeq: 0, endSeq: 3 },
      sourceEventSeqs: [0, 1, 2, 3],
    }),
    eventError("DSH_SESSION_EVENT_SURFACE_RANGE_MISSING"),
  );
  assert.equal(agent.session.events.length, 1);
});

test("会话桥拒绝写入未声明事件与旧版本状态", async () => {
  const host = createStateHost();
  const services = createAmitiaDshHostServices(host);
  const agent = services.agents.agentForTool("session-undeclared");
  await agent.session.hydrated;
  assert.throws(() => agent.session.append("agent/echo", { value: 1 }), eventError("DSH_SESSION_EVENT_UNDECLARED"));
  assert.equal(agent.session.events.length, 0);

  const legacy = createStateHost(new Map([[
    "dsh-session:session-legacy",
    { version: 1, value: { version: 1, conversationId: "session-legacy", events: [] } },
  ]]));
  const legacyServices = createAmitiaDshHostServices(legacy);
  const legacyAgent = legacyServices.agents.agentForTool("session-legacy");
  await assert.rejects(legacyAgent.session.hydrated, eventError("DSH_SESSION_STATE_VERSION_UNSUPPORTED"));
  assert.throws(() => legacyAgent.session.append("turn/start", { turn: 1 }), eventError("DSH_SESSION_STATE_VERSION_UNSUPPORTED"));
});

test("会话桥不再静默截断会话事件", async () => {
  const host = createStateHost();
  const services = createAmitiaDshHostServices(host);
  const agent = services.agents.agentForTool("session-long");
  await agent.session.hydrated;
  const total = 600;
  for (let index = 0; index < total; index += 1) agent.session.append("turn/start", { turn: index });
  await agent.session.whenPersisted();
  const stored = host.records.get("dsh-session:session-long").value;
  assert.equal(stored.events.length, total);
  assert.equal(stored.events[0].data.turn, 0);

  const reopened = createAmitiaDshHostServices(host);
  const reopenedAgent = reopened.agents.agentForTool("session-long");
  await reopenedAgent.session.hydrated;
  assert.equal(reopenedAgent.session.events.length, total);

  assert.throws(() => restoredOverLimit(), eventError("DSH_SESSION_EVENT_LIMIT_EXCEEDED"));
});

function restoredOverLimit() {
  const events = Array.from({ length: SESSION_EVENT_LIMIT + 1 }, (_, index) => ({
    type: "turn/start",
    seq: index,
    time: index + 1,
    data: { turn: index },
  }));
  return restoreSessionEvents(events, "session-over-limit");
}

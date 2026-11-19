import assert from "node:assert/strict";
import test from "node:test";
import { createAmitiaDshHostServices } from "../src/runtime/host-services.mjs";

function createStateHost(options = {}) {
  const calls = [];
  const storage = new Map();
  const conversations = options.conversations ?? [];
  let version = 0;
  for (const [key, value] of Object.entries(options.state ?? {})) {
    version += 1;
    storage.set(key, { value, version });
  }
  const host = {
    calls,
    storage,
    failNextCas: 0,
    async call(method, input) {
      calls.push({ method, input });
      if (method === "host.state.get") {
        if (!storage.has(input.key)) return { found: false, version: 0 };
        const entry = storage.get(input.key);
        return { found: true, value: entry.value, version: entry.version };
      }
      if (method === "host.state.cas") {
        if (host.failNextCas > 0) {
          host.failNextCas -= 1;
          return { swapped: false };
        }
        const current = storage.has(input.key) ? storage.get(input.key).version : 0;
        if (current !== input.expectedVersion) return { swapped: false };
        version += 1;
        storage.set(input.key, { value: input.newValue, version });
        return { swapped: true };
      }
      if (method === "host.state.delete") {
        storage.delete(input.key);
        return { deleted: true };
      }
      if (method === "host.conversation.list") {
        const limit = Math.min(Number(input.limit) || 50, 128);
        const offset = Math.max(Number(input.offset) || 0, 0);
        return { items: conversations.slice(offset, offset + limit), workspaces: [] };
      }
      if (method === "host.conversation.read") return { messages: [], hasMore: false };
      if (method === "host.conversation.turns") return { turns: [] };
      if (method === "host.conversation.scope") return { scopeSnapshotId: "scope-" + String(input.conversationId || "") };
      throw new Error(`unexpected host method ${method}`);
    },
  };
  return host;
}

test("bridges credentials through Amitia extension state with reference grammar checks", async () => {
  const host = createStateHost();
  const services = createAmitiaDshHostServices(host);
  process.env.AMITIA_TEST_CREDENTIAL = "ambient-value";
  try {
    assert.deepEqual(await services.credentials.resolve("AMITIA_TEST_CREDENTIAL"), {
      value: "ambient-value",
      source: "env",
    });
    await services.credentials.set("AMITIA_TEST_CREDENTIAL", "sk-live-12345678");
    assert.deepEqual(await services.credentials.resolve("AMITIA_TEST_CREDENTIAL"), {
      value: "sk-live-12345678",
      source: "file",
    });
    const described = await services.credentials.describe("AMITIA_TEST_CREDENTIAL");
    assert.equal(described.configured, true);
    assert.equal(described.source, "file");
    assert.equal(described.writable, true);
    assert.equal(described.masked.endsWith("78"), true);
    assert.equal(described.masked.includes("live"), false);
    await services.credentials.unset("AMITIA_TEST_CREDENTIAL");
    assert.deepEqual(await services.credentials.resolve("AMITIA_TEST_CREDENTIAL"), {
      value: "ambient-value",
      source: "env",
    });
    await services.credentials.unset("AMITIA_TEST_CREDENTIAL");
    await assert.rejects(() => services.credentials.set("AMITIA_TEST_CREDENTIAL", ""), {
      code: "DSH_CREDENTIAL_VALUE_EMPTY",
    });
    await assert.rejects(() => services.credentials.resolve("1-BAD-NAME"), {
      code: "DSH_CREDENTIAL_REFERENCE_INVALID",
    });
  } finally {
    delete process.env.AMITIA_TEST_CREDENTIAL;
    await services.credentials.unset("AMITIA_TEST_CREDENTIAL");
  }
  assert.deepEqual(await services.credentials.resolve("AMITIA_TEST_CREDENTIAL"), undefined);
  assert.deepEqual(await services.credentials.describe("AMITIA_TEST_CREDENTIAL"), {
    configured: false,
    writable: true,
  });
});

test("stores credential records through the serialized mutate path", async () => {
  const host = createStateHost();
  const services = createAmitiaDshHostServices(host);
  host.failNextCas = 3;
  const written = await services.credentials.modifyRecord("openai-grant", async (current) => {
    assert.equal(current, undefined);
    return { kind: "grant", accessToken: "token-1" };
  });
  assert.deepEqual(written, { kind: "grant", accessToken: "token-1" });
  assert.deepEqual(await services.credentials.readRecord("openai-grant"), {
    kind: "grant",
    accessToken: "token-1",
  });
  assert.deepEqual(await services.credentials.describeRecord("openai-grant"), {
    present: true,
    kind: "grant",
    writable: true,
  });
  assert.deepEqual(await services.credentials.listRecords(), [{ key: "openai-grant", kind: "grant" }]);
  const declined = await services.credentials.modifyRecord("openai-grant", async () => undefined);
  assert.deepEqual(declined, { kind: "grant", accessToken: "token-1" });
  await services.credentials.deleteRecord("openai-grant");
  assert.equal(await services.credentials.readRecord("openai-grant"), undefined);
  assert.deepEqual(await services.credentials.listRecords(), []);
  await services.credentials.deleteRecord("openai-grant");
});

test("registers DSH commands and records the official run/done pair", async () => {
  const host = createStateHost();
  const services = createAmitiaDshHostServices(host);
  const agent = services.agents.agentForTool("conversation-1");
  await agent.session.hydrated;
  const received = [];
  services.commands.register({
    definitionId: "test/goal",
    name: "goal",
    description: "Set or view the goal",
    input: { hint: "[clear]", attachments: false },
    handler: (invocation) => {
      received.push(invocation.rawInput);
      return { kind: "success", text: "Goal set", sourceEventSeq: 2 };
    },
  });
  services.commands.register({
    name: "quiet",
    description: "Records no input",
    recordInput: false,
    handler: () => ({ kind: "success" }),
  });
  assert.deepEqual(services.commands.list().map((entry) => entry.name), ["goal", "quiet"]);
  assert.equal(services.commands.find(agent, "goal").name, "goal");
  assert.equal(services.commands.find(agent, "missing"), undefined);
  assert.equal(await services.commands.execute(agent, "goal now"), undefined);
  assert.equal(await services.commands.execute(agent, "/missing now"), undefined);
  const execution = await services.commands.execute(agent, "/goal finish the report");
  assert.equal(execution.result.kind, "success");
  assert.deepEqual(received, [" finish the report"]);
  const events = agent.session.events;
  const run = events.find((event) => event.type === "command/run");
  const done = events.find((event) => event.type === "command/done");
  assert.deepEqual(run.data, {
    commandId: execution.commandId,
    name: "goal",
    source: { kind: "user" },
    args: " finish the report",
  });
  assert.deepEqual(done.data, {
    commandId: execution.commandId,
    kind: "success",
    text: "Goal set",
    sourceEventSeq: 2,
  });
  await services.commands.execute(agent, "/quiet");
  const quietRun = agent.session.events.filter((event) => event.type === "command/run").at(-1);
  assert.equal(quietRun.data.name, "quiet");
  assert.equal("args" in quietRun.data, false);
});

test("settles thrown command handlers as errors while keeping the original error", async () => {
  const host = createStateHost();
  const services = createAmitiaDshHostServices(host);
  const agent = services.agents.agentForTool("conversation-2");
  await agent.session.hydrated;
  const original = new Error("goal storage unavailable");
  services.commands.register({
    name: "goal",
    description: "Fails",
    handler: () => {
      throw original;
    },
  });
  const execution = await services.commands.execute(agent, "/goal");
  assert.equal(execution.result.kind, "error");
  assert.equal(execution.result.text, "goal storage unavailable");
  assert.equal(execution.result.error, original);
  assert.equal(Object.keys(execution.result).includes("error"), false);
  const done = agent.session.events.find((event) => event.type === "command/done");
  assert.deepEqual(done.data, {
    commandId: execution.commandId,
    kind: "error",
    text: "goal storage unavailable",
  });
});

test("rejects command attachments that the definition does not accept", async () => {
  const host = createStateHost();
  const services = createAmitiaDshHostServices(host);
  const agent = services.agents.agentForTool("conversation-3");
  await agent.session.hydrated;
  let invoked = false;
  services.commands.register({
    name: "compact",
    description: "No attachments",
    handler: () => {
      invoked = true;
      return { kind: "success" };
    },
  });
  const execution = await services.commands.execute(agent, "/compact", [{ kind: "image" }]);
  assert.equal(execution.result.kind, "error");
  assert.equal(execution.commandId, "");
  assert.equal(invoked, false);
  assert.deepEqual(agent.session.events, []);
  assert.throws(() => services.commands.register({ name: "compact", description: "dup", handler: () => ({}) }), {
    code: "DSH_COMMAND_DUPLICATE",
  });
  assert.throws(() => services.commands.register({ name: "Not Lowercase", description: "bad", handler: () => ({}) }), {
    code: "DSH_COMMAND_NAME_INVALID",
  });
  const dispose = services.commands.registerFileReceiptResolver(async () => []);
  assert.throws(() => services.commands.registerFileReceiptResolver(async () => []), {
    code: "DSH_COMMAND_RECEIPT_RESOLVER_DUPLICATE",
  });
  dispose();
});

test("reads DSH session query views from the Amitia conversation catalog and persisted events", async () => {
  const events = [
    { type: "turn/start", seq: 0, time: 1762000000000, data: { turn: 1 } },
    { type: "user/message", seq: 1, time: 1762000000001, data: { message: { id: "m1" } }, surfaceOp: "append" },
    { type: "user/message", seq: 2, time: 1762000000002, data: { message: { id: "m2" } }, surfaceOp: { op: "replace", startSeq: 1, endSeq: 1 }, sourceEventSeqs: [1] },
    { type: "session/title", seq: 3, time: 1762000000003, data: { title: "Quarterly plan", messageSeqs: [2], source: "user" } },
  ];
  const host = createStateHost({
    conversations: [
      {
        sessionId: "session-1",
        title: "Quarterly plan",
        createdAt: "2026-11-01T10:00:00Z",
        cwd: "/workspace/plan",
        formatStatus: "current",
      },
      {
        sessionId: "session-2",
        title: "",
        createdAt: "2026-11-01T11:00:00Z",
      },
    ],
    state: {
      "dsh-session:session-1": {
        version: 2,
        conversationId: "session-1",
        scopeSnapshotId: "",
        events,
        updatedAt: "2026-11-01T10:00:00.000Z",
      },
    },
  });
  const services = createAmitiaDshHostServices(host);
  const recorded = await services.sessionQuery.listSessions();
  assert.deepEqual(recorded.map((entry) => entry.header.id), ["session-1", "session-2"]);
  assert.deepEqual(recorded[0].header, {
    version: 2,
    id: "session-1",
    createdAt: Date.parse("2026-11-01T10:00:00Z"),
    isSeeded: false,
    cwd: "/workspace/plan",
  });
  assert.equal(recorded[0].persisted, true);
  assert.equal(recorded[0].formatStatus, "current");
  assert.equal(recorded[0].live, false);
  assert.equal("cwd" in recorded[1].header, false);

  const surface = await services.sessionQuery.readSurface("session-1");
  assert.equal(surface.capturedThroughSeq, 3);
  assert.equal(surface.inheritedEventCount, 0);
  assert.deepEqual(surface.events.map((event) => event.seq), [2]);

  const listed = await services.sessionQuery.listEvents("session-1");
  assert.deepEqual(listed.map((event) => [event.seq, event.surface]), [
    [0, "log-only"],
    [1, "shadowed"],
    [2, "current"],
    [3, "log-only"],
  ]);
  assert.deepEqual(listed[0], { sessionId: "session-1", seq: 0, type: "turn/start", time: 1762000000000, surface: "log-only" });

  const snapshot = await services.sessionQuery.readSession("session-1");
  assert.equal(snapshot.events.length, 4);
  assert.equal(snapshot.session.id, "session-1");

  const title = await services.sessionQuery.readTitle("session-1");
  assert.deepEqual(title, {
    title: "Quarterly plan",
    messageSeqs: [2],
    source: "user",
    eventSeq: 3,
    updatedAt: 1762000000003,
  });
  assert.equal(await services.sessionQuery.readTitle("session-2"), undefined);
  assert.deepEqual(await services.sessionQuery.readTitleSnapshot("session-2"), {
    session: {
      version: 2,
      id: "session-2",
      createdAt: Date.parse("2026-11-01T11:00:00Z"),
      isSeeded: false,
    },
  });

  const batch = await services.sessionQuery.readTitleSnapshots(["session-1", "session-1", "missing"]);
  assert.equal(batch.length, 2);
  assert.equal(batch[0].status, "fulfilled");
  assert.equal(batch[0].value.title.title, "Quarterly plan");
  assert.equal(batch[1].status, "rejected");
  assert.equal(batch[1].reason.code, "DSH_SESSION_NOT_FOUND");
});

test("inspectSession 交给读取函数只读会话视图", async () => {
  const events = [
    { type: "turn/start", seq: 0, time: 1762000000000, data: { turn: 1 } },
    { type: "user/message", seq: 1, time: 1762000000001, data: { id: "m1", role: "user", content: [{ type: "text", text: "季度计划" }], source: { kind: "user" } }, surfaceOp: "append" },
    { type: "assistant/message", seq: 2, time: 1762000000002, data: { turn: 1, step: 0, stream: [], message: { id: "m2", role: "assistant", content: [{ type: "text", text: "收到" }], source: { kind: "model", provider: "amitia", model: "amitia-conversation" } } }, surfaceOp: "append" },
    { type: "session/title", seq: 3, time: 1762000000003, data: { title: "季度计划", messageSeqs: [1], source: "user" } },
  ];
  const host = createStateHost({
    conversations: [{ sessionId: "session-1", title: "季度计划", createdAt: "2026-11-01T10:00:00Z", formatStatus: "current" }],
    state: {
      "dsh-session:session-1": {
        version: 2,
        conversationId: "session-1",
        scopeSnapshotId: "",
        events,
        updatedAt: "2026-11-01T10:00:00.000Z",
      },
    },
  });
  const services = createAmitiaDshHostServices(host);
  const summary = await services.sessionQuery.inspectSession("session-1", (session) => ({
    id: session.id,
    surfaceSeqs: session.surfaceEvents().map((event) => event.seq),
    messages: session.messages().map((message) => message.role),
    title: session.title()?.title,
  }));
  assert.deepEqual(summary, { id: "session-1", surfaceSeqs: [1, 2], messages: ["user", "assistant"], title: "季度计划" });
  await assert.rejects(services.sessionQuery.inspectSession("session-1", (session) => session.append("user/message", {})), (error) => {
    assert.equal(error.code, "DSH_SESSION_INSPECT_READ_ONLY");
    return true;
  });
});

test("keeps the remaining session query methods explicitly unavailable", async () => {
  const host = createStateHost();
  const services = createAmitiaDshHostServices(host);
  await assert.rejects(services.sessionQuery.inspectSession("session-1"), {
    code: "DSH_SESSION_INSPECT_READ_REQUIRED",
  });
  await assert.rejects(services.sessionQuery.searchEvents({ sessionId: "session-1", query: "no-hit-token" }), (error) => {
    assert.equal(error.code, "DSH_SESSION_NOT_FOUND");
    assert.match(error.message, /session-1/);
    return true;
  });
});

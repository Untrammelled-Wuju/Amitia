import assert from "node:assert/strict";
import { mkdtemp, stat, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { command, dshCompatibleExtension } from "../src/runtime/index.mjs";

function createHost() {
  const records = new Map();
  return {
    records,
    async call(method, input) {
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

test("registers, enables and removes a DSH package record", async () => {
  const host = createHost();
  const registered = await command(host, {
    action: "register",
    plugin: {
      pluginId: "example/dsh-tool",
      name: "Example DSH Tool",
      version: "1.0.0",
      entry: "dist/index.js",
      source: "npm:example-dsh-tool@1.0.0",
      enabled: true,
    },
  });
  assert.equal(registered.package.pluginId, "example/dsh-tool");
  assert.equal(registered.package.enabled, true);

  const disabled = await command(host, {
    action: "set-enabled",
    pluginId: "example/dsh-tool",
    enabled: false,
  });
  assert.equal(disabled.package.enabled, false);

  const status = await command(host, { action: "status" });
  assert.equal(status.packageCount, 1);
  assert.equal(status.enabledCount, 0);
  assert.equal(status.compatibility.complete, false);
  assert.equal(status.compatibility.partial > 0, true);

  const removed = await command(host, {
    action: "remove",
    pluginId: "example/dsh-tool",
  });
  assert.equal(removed.packageCount, 0);
});

test("streams DSH client source in bounded chunks", async () => {
  const host = createHost();
  const packageDir = await mkdtemp(join(tmpdir(), "amitia-dsh-client-source-"));
  const code = `export const marker = "${"x".repeat(600 * 1024)}";\n//# sourceMappingURL=client.js.map\n`;
  await writeFile(join(packageDir, "client.js"), code, "utf8");
  await command(host, {
    action: "register",
    plugin: {
      pluginId: "example/dsh-client-source",
      name: "Example DSH Client",
      version: "1.0.0",
      entry: "index.js",
      clientEntry: "client.js",
      packageDir,
      source: "local:fixture",
      enabled: true,
    },
  });
  const chunks = [];
  let offset = 0;
  while (true) {
    const chunk = await command(host, {
      action: "client-source",
      pluginId: "example/dsh-client-source",
      entryId: "root",
      offset,
      length: 128 * 1024,
    });
    chunks.push(Buffer.from(chunk.dataBase64, "base64"));
    assert.ok(chunks.at(-1).length <= 128 * 1024);
    if (chunk.done) break;
    assert.ok(chunk.nextOffset > offset);
    offset = chunk.nextOffset;
  }
  assert.equal(
    Buffer.concat(chunks).toString("utf8"),
    code.replace(/\/\/# sourceMappingURL=.*$/gm, ""),
  );
});

test("reads one persisted live DSH tool result by session sequence", async () => {
  const host = createHost();
  const swapped = await host.call("host.state.cas", {
    key: "dsh-session:conversation-result",
    expectedVersion: 0,
    newValue: {
      version: 2,
      conversationId: "conversation-result",
      events: [
        { type: "text", data: { value: "ignored" }, seq: 1, time: 1, ignorable: true },
        {
          type: "tool/result",
          data: {
            turn: 1,
            step: 1,
            message: { toolCallId: "call-result" },
            liveResult: { output: "persisted" },
          },
          seq: 2,
          time: 2,
          surfaceOp: "append",
        },
      ],
    },
  });
  assert.equal(swapped.swapped, true);
  assert.deepEqual(
    await command(host, {
      action: "tool-result-watch",
      sessionId: "conversation-result",
      seq: 2,
    }),
    { available: true, value: { output: "persisted" } },
  );
  assert.deepEqual(
    await command(host, {
      action: "tool-result-watch",
      sessionId: "conversation-result",
      seq: 1,
    }),
    { available: false },
  );
});

test("searches conversations through the generic host API", async () => {
  const calls = [];
  const host = {
    async call(method, input) {
      calls.push({ method, input });
      return { items: [{ sessionId: "conversation-1", snippet: "matched" }], hasMore: false };
    },
  };
  assert.deepEqual(
    await command(host, {
      action: "conversation-search",
      query: " matched ",
      limit: 20,
    }),
    { items: [{ sessionId: "conversation-1", snippet: "matched" }], hasMore: false },
  );
  assert.deepEqual(calls, [{
    method: "host.conversation.search",
    input: { query: "matched", limit: 20 },
  }]);
});

test("rejects duplicate plugin ids and invalid records", async () => {
  const host = createHost();
  await command(host, {
    action: "register",
    plugin: {
      pluginId: "example/duplicate",
      version: "1.0.0",
      entry: "dist/index.js",
      enabled: false,
    },
  });
  await command(host, {
    action: "register",
    plugin: {
      pluginId: "example/duplicate",
      version: "2.0.0",
      entry: "dist/index.js",
      enabled: false,
    },
  });
  const status = await command(host, { action: "status" });
  assert.equal(status.packageCount, 1);
  assert.equal(status.packages[0].version, "2.0.0");
  await assert.rejects(
    command(host, { action: "register", plugin: { pluginId: "example/missing-version", entry: "dist/index.js" } }),
    /缺少 version/,
  );
});

test("installs, enables and invokes a DSH tool through the container command", async () => {
  const host = createHost();
  const pluginDataRoot = await mkdtemp(join(tmpdir(), "amitia-dsh-runtime-"));
  const fixture = fileURLToPath(new URL("./fixtures/simple-tool-plugin", import.meta.url));
  const installed = await command(host, {
    action: "install",
    spec: fixture,
    pluginDataRoot,
  });
  assert.equal(installed.package.pluginId, "@amitia-test/simple-tool-plugin");

  await command(host, {
    action: "set-enabled",
    pluginId: "@amitia-test/simple-tool-plugin",
    enabled: true,
  });
  const invoked = await command(host, {
    action: "invoke",
    pluginId: "@amitia-test/simple-tool-plugin",
    toolName: "double",
    toolInput: { value: 9 },
  });
  assert.deepEqual(invoked, { value: { value: 18 } });

  await command(host, { action: "reset" });
});

test("lists and kills jobs owned by enabled DSH plugins", async () => {
  const host = createHost();
  const context = {
    host,
    handlers: { bindTool() {} },
    contributions: {
      async register() {
        return { async dispose() {} };
      },
    },
  };
  await dshCompatibleExtension.activate(context);
  const pluginDataRoot = await mkdtemp(join(tmpdir(), "amitia-dsh-jobs-runtime-"));
  const fixture = fileURLToPath(new URL("./fixtures/job-plugin", import.meta.url));
  try {
    await command(host, { action: "install", spec: fixture, pluginDataRoot });
    await command(host, {
      action: "set-enabled",
      pluginId: "@amitia-test/job-plugin",
      enabled: true,
    });
    await command(host, {
      action: "invoke",
      pluginId: "@amitia-test/job-plugin",
      toolName: "start-job",
    });
    let jobs = await command(host, { action: "jobs-list" });
    assert.equal(jobs.jobs.length, 1);
    assert.equal(jobs.jobs[0].owner, "session-1");
    assert.equal(jobs.jobs[0].status, "running");
    const killed = await command(host, {
      action: "jobs-kill",
      pluginId: "@amitia-test/job-plugin",
      jobId: "job-1",
      reason: "test stop",
    });
    assert.equal(killed.ok, true);
    assert.equal(killed.alreadyFinished, false);
    jobs = await command(host, { action: "jobs-list" });
    assert.equal(jobs.jobs[0].status, "killed");
    assert.equal(jobs.jobs[0].detail, "test stop");
    await command(host, { action: "reset" });
  } finally {
    await dshCompatibleExtension.deactivate();
  }
});

test("registers enabled DSH tools as dynamic Amitia contributions", async () => {
  const host = createHost();
  const handlers = new Map();
  const registrations = [];
  const disposed = [];
  const context = {
    host,
    handlers: {
      bindTool(name, handler) {
        handlers.set(name, handler);
      },
    },
    contributions: {
      async register(descriptor) {
        const registration = {
          id: descriptor.id,
          async dispose() {
            disposed.push(descriptor.id);
          },
        };
        registrations.push({ descriptor, registration });
        return registration;
      },
    },
  };
  const pluginDataRoot = await mkdtemp(join(tmpdir(), "amitia-dsh-contribution-"));
  const fixture = fileURLToPath(new URL("./fixtures/simple-tool-plugin", import.meta.url));
  await dshCompatibleExtension.activate(context);
  try {
    assert.equal(registrations.length, 1);
    assert.equal(registrations[0].descriptor.metadata["amitia.context.provides"][0], "chat.realtime.prompt");
    await command(host, { action: "install", spec: fixture, pluginDataRoot });
    await command(host, {
      action: "set-enabled",
      pluginId: "@amitia-test/simple-tool-plugin",
      enabled: true,
    });
    assert.equal(registrations.length, 2);
    const status = await command(host, { action: "status" });
    assert.equal(status.packages[0].mappedContributionCount, 1);
    const toolRegistration = registrations.find((item) => item.descriptor.id !== "dsh.prompt-context");
    assert.equal(toolRegistration.descriptor.kind, "tool");
    const handler = handlers.get(toolRegistration.descriptor.spec.handlerName);
    assert.equal(typeof handler, "function");
    const result = await handler({ value: 5 }, { invocationId: "call-1" });
    assert.deepEqual(result, { value: 10 });
    const promptHandler = handlers.get("dsh_prompt_context");
    const prompt = await promptHandler({});
    assert.deepEqual(prompt.sections, [{
      id: "simple-tool-prompt",
      content: "Use the simple tool carefully.",
      priority: -50,
    }]);
    assert.deepEqual(prompt.contexts, [{
      name: "simple-tool-context",
      order: 100,
      text: "Simple tool context.",
      kind: "context",
    }]);

    await command(host, {
      action: "set-enabled",
      pluginId: "@amitia-test/simple-tool-plugin",
      enabled: false,
    });
    assert.deepEqual(disposed, [toolRegistration.descriptor.id]);
  } finally {
    await dshCompatibleExtension.deactivate();
  }
});

test("activates only the package root entry and leaves non-plugin subpath exports alone", async () => {
  const host = createHost();
  const pluginDataRoot = await mkdtemp(join(tmpdir(), "amitia-dsh-bundle-"));
  const fixture = fileURLToPath(new URL("./fixtures/multi-entry-plugin", import.meta.url));
  await command(host, {
    action: "install",
    spec: fixture,
    pluginDataRoot,
  });
  await command(host, {
    action: "set-enabled",
    pluginId: "@amitia-test/multi-entry-plugin",
    enabled: true,
  });
  const status = await command(host, { action: "status" });
  const record = status.packages.find((item) => item.pluginId === "@amitia-test/multi-entry-plugin");
  assert.deepEqual(record.entries, [{ id: "root", entry: "./index.js" }]);
  assert.equal(record.entryCount, 1);
  const search = await command(host, {
    action: "invoke",
    pluginId: "@amitia-test/multi-entry-plugin",
    toolName: "search",
    toolInput: { value: "alpha" },
  });
  assert.deepEqual(search, { value: { entry: "root", value: "alpha" } });
  await command(host, { action: "reset" });
});

test("passes the current Amitia conversation into DSH tool execution", async () => {
  const host = createHost();
  const handlers = new Map();
  const context = {
    host,
    handlers: {
      bindTool(name, handler) {
        handlers.set(name, handler);
      },
    },
    contributions: {
      async register(descriptor) {
        return { id: descriptor.id, async dispose() {} };
      },
    },
  };
  const pluginDataRoot = await mkdtemp(join(tmpdir(), "amitia-dsh-agent-context-"));
  const fixture = fileURLToPath(new URL("./fixtures/agent-tool-plugin", import.meta.url));
  await dshCompatibleExtension.activate(context);
  try {
    await command(host, { action: "install", spec: fixture, pluginDataRoot });
    await command(host, {
      action: "set-enabled",
      pluginId: "@amitia-test/agent-tool-plugin",
      enabled: true,
    });
    const registration = Array.from(handlers.entries()).find(([name]) => name.startsWith("dsh__"));
    assert.ok(registration);
    const result = await registration[1]({ value: "context" }, {
      toolCallId: "call-agent-context",
      runtime: { scope: { conversationId: "conversation-agent-context" } },
    });
    assert.deepEqual(result, { eventCount: 1, inboxCount: 0 });
  } finally {
    await dshCompatibleExtension.deactivate();
  }
});

test("creates an Amitia conversation through the DSH command bridge", async () => {
  const calls = [];
  const host = {
    async call(method, input) {
      calls.push({ method, input });
      if (method === "host.conversation.create") {
        return {
          conversationId: "conversation-created",
          scopeSnapshotId: "scope-conversation-created",
          ownerId: "owner-1",
          characterId: "character-1",
        };
      }
      throw new Error(`unexpected host method ${method}`);
    },
  };
  const result = await command(host, {
    action: "conversation-create",
    title: "DSH session",
    projectId: "project-1",
    channel: "desktop",
    peerId: "owner-1",
    requestId: "request-1",
  });
  assert.deepEqual(result, {
    conversationId: "conversation-created",
    scopeSnapshotId: "scope-conversation-created",
  });
  assert.deepEqual(calls, [{
    method: "host.conversation.create",
    input: {
      title: "DSH session",
      projectId: "project-1",
      channel: "desktop",
      peerId: "owner-1",
      requestId: "request-1",
    },
  }]);
});

test("deletes a DSH conversation target and its session state together", async () => {
  const calls = [];
  const host = {
    async call(method, input) {
      calls.push({ method, input });
      if (method === "host.state.delete") return { ok: true };
      throw new Error(`unexpected host method ${method}`);
    },
    async callTargeted(method, input, scopeSnapshotId) {
      calls.push({ method, input, scopeSnapshotId });
      if (method === "host.conversation.delete") return { deleted: true, conversationId: input.conversationId };
      throw new Error(`unexpected targeted host method ${method}`);
    },
  };
  const result = await command(host, {
    action: "conversation-delete",
    conversationId: "conversation-delete-1",
    scopeSnapshotId: "conversation-target-1",
    requestId: "delete-request-1",
  });
  assert.deepEqual(result, { deleted: true, conversationId: "conversation-delete-1" });
  assert.deepEqual(calls, [
    {
      method: "host.conversation.delete",
      input: { conversationId: "conversation-delete-1", requestId: "delete-request-1" },
      scopeSnapshotId: "conversation-target-1",
    },
    {
      method: "host.state.delete",
      input: { key: "dsh-session:conversation-delete-1" },
    },
  ]);
});

test("creates, lists and cancels Amitia session prompt schedules", async () => {
  const schedules = [];
  const calls = [];
  const host = {
    async call(method, input) {
      calls.push({ method, input });
      if (method === "host.schedule.create") {
        schedules.push(structuredClone(input));
        return { ok: true, scheduleId: input.scheduleId };
      }
      if (method === "host.schedule.list") {
        return {
          schedules: structuredClone(schedules),
          states: Object.fromEntries(schedules.map((item) => [item.scheduleId, {
            status: "enabled",
            nextScheduledAt: "2026-11-14T01:00:00.000Z",
            updatedAt: "2026-11-14T00:00:00.000Z",
          }])),
          total: schedules.length,
        };
      }
      if (method === "host.schedule.cancel") {
        const index = schedules.findIndex((item) => item.scheduleId === input.scheduleId);
        if (index < 0) {
          const error = new Error("schedule not found");
          error.code = "resource_not_found";
          throw error;
        }
        schedules.splice(index, 1);
        return { ok: true };
      }
      if (method === "host.schedule.run") {
        return { ok: true, scheduleId: input.scheduleId, triggerId: "trigger-1" };
      }
      if (method === "host.schedule.update") {
        const index = schedules.findIndex((item) => item.scheduleId === input.scheduleId);
        if (index < 0) throw new Error("schedule not found");
        schedules[index] = structuredClone(input.definition);
        return { ok: true };
      }
      if (method === "host.schedule.history") {
        return {
          id: input.scheduleId,
          records: [{
            scheduledAt: "2026-11-14T01:00:00.000Z",
            deliveredAt: "2026-11-14T01:00:01.000Z",
            messageId: "message-1",
            targetSessionId: "conversation-1",
            prompt: "Updated prompt",
          }],
          earlierRecordsUnavailable: false,
          earlierRecordsPruned: false,
          retention: { days: 30, records: 200 },
        };
      }
      throw new Error(`unexpected host method ${method}`);
    },
  };
  const created = await command(host, {
    action: "schedule-create",
    request: {
      title: "Scheduled review",
      prompt: "Review the active task",
      timing: { kind: "after", after_seconds: 60 },
      target: { kind: "session", sessionId: "conversation-1" },
    },
  });
  assert.equal(created.kind, "after");
  assert.equal(created.title, "Scheduled review");
  assert.deepEqual(created.target, { kind: "session", sessionId: "conversation-1" });
  assert.equal(schedules[0].target.type, "session_prompt");
  const catalog = await command(host, { action: "schedule-catalog" });
  assert.equal(catalog.length, 1);
  assert.equal(catalog[0].id, created.id);
  const running = await command(host, { action: "schedule-run", scheduleId: created.id });
  assert.equal(running.id, created.id);
  const updated = await command(host, {
    action: "schedule-update",
    scheduleId: created.id,
    patch: {
      title: "Updated review",
      prompt: "Updated prompt",
      timing: { kind: "every", every_seconds: 120 },
      active: true,
    },
  });
  assert.equal(updated.title, "Updated review");
  assert.equal(updated.kind, "every");
  assert.equal((await command(host, {
    action: "schedule-history",
    scheduleId: created.id,
    limit: 20,
  })).records[0].messageId, "message-1");
  const deleted = await command(host, { action: "schedule-cancel", scheduleId: created.id });
  assert.deepEqual(deleted, { id: created.id, deleted: true });
  assert.equal((await command(host, { action: "schedule-list" })).total, 0);
  assert.deepEqual(await command(host, { action: "schedule-delete-inactive" }), { deleted: 0 });
  assert.ok(calls.some((entry) => entry.method === "host.schedule.create"));
});

test("propagates an activation failure while persisting the enabled intent", async () => {
  const host = createHost();
  const pluginDataRoot = await mkdtemp(join(tmpdir(), "amitia-dsh-non-plugin-"));
  const fixture = fileURLToPath(new URL("./fixtures/non-plugin-entry-plugin", import.meta.url));
  await command(host, { action: "install", spec: fixture, pluginDataRoot });
  await assert.rejects(
    command(host, {
      action: "set-enabled",
      pluginId: "@amitia-test/non-plugin-entry-plugin",
      enabled: true,
    }),
    (error) => {
      assert.match(error.message, /invalid plugin, expect function or object with an "apply" method, received object/);
      assert.ok(error.stack.includes("invalid plugin"));
      return true;
    },
  );
  const status = await command(host, { action: "status" });
  const record = status.packages.find((item) => item.pluginId === "@amitia-test/non-plugin-entry-plugin");
  assert.equal(record.enabled, true);
  assert.equal(record.active, false);
  assert.match(record.activationError.message, /invalid plugin/);
  assert.ok(record.activationError.stack.includes("invalid plugin"));
});

test("keeps the host starting when an enabled package cannot activate", async () => {
  const host = createHost();
  const pluginDataRoot = await mkdtemp(join(tmpdir(), "amitia-dsh-startup-"));
  const fixture = fileURLToPath(new URL("./fixtures/non-plugin-entry-plugin", import.meta.url));
  await command(host, { action: "install", spec: fixture, pluginDataRoot });
  for (const record of host.records.values()) {
    const target = record.value?.packages?.find((item) => item.pluginId === "@amitia-test/non-plugin-entry-plugin");
    if (target) target.enabled = true;
  }
  const context = {
    host,
    handlers: { bindTool() {} },
    contributions: { async register() { return { async dispose() {} }; } },
  };
  await dshCompatibleExtension.activate(context);
  try {
    const status = await command(host, { action: "status" });
    const record = status.packages.find((item) => item.pluginId === "@amitia-test/non-plugin-entry-plugin");
    assert.equal(record.enabled, true);
    assert.equal(record.active, false);
    assert.match(record.activationError.message, /invalid plugin/);
    const healthy = await command(host, { action: "list" });
    assert.equal(healthy.packages.length, 1);
  } finally {
    await dshCompatibleExtension.deactivate();
  }
});

test("composes official seam and provider packages inside one shared container realm", async () => {
  const host = createHost();
  const pluginDataRoot = await mkdtemp(join(tmpdir(), "amitia-dsh-container-"));
  const fixtures = {
    consumer: fileURLToPath(new URL("./fixtures/storage-consumer-plugin", import.meta.url)),
    provider: fileURLToPath(new URL("./fixtures/storage-json-provider", import.meta.url)),
    seam: fileURLToPath(new URL("./fixtures/storage-seam-plugin", import.meta.url)),
  };
  for (const spec of [fixtures.consumer, fixtures.provider, fixtures.seam]) {
    await command(host, { action: "install", spec, pluginDataRoot });
  }
  for (const record of host.records.values()) {
    for (const item of record.value?.packages ?? []) item.enabled = true;
  }
  const context = {
    host,
    handlers: { bindTool() {} },
    contributions: { async register() { return { async dispose() {} }; } },
  };
  await dshCompatibleExtension.activate(context);
  try {
    const status = await command(host, { action: "status" });
    const active = status.packages.filter((item) => item.active).map((item) => item.pluginId).sort();
    assert.deepEqual(active, [
      "@amitia-test/storage-consumer-plugin",
      "@amitia-test/storage-seam-plugin",
      "@deepseek-ai/dsh-storage-json",
    ]);
    const providerRecord = status.packages.find((item) => item.pluginId === "@deepseek-ai/dsh-storage-json");
    assert.equal(providerRecord.activationError, null);
    assert.equal(globalThis.__dshStorageConsumerProbe.provider, "json");
    assert.equal(globalThis.__dshStorageJsonProbe.root, join(pluginDataRoot, "dsh-data", "storage"));
    const stats = await stat(globalThis.__dshStorageJsonProbe.root);
    assert.equal(stats.isDirectory(), true);
  } finally {
    await dshCompatibleExtension.deactivate();
  }
});

test("reports the missing service when a provider activates without its seam", async () => {
  const host = createHost();
  const pluginDataRoot = await mkdtemp(join(tmpdir(), "amitia-dsh-missing-seam-"));
  const fixture = fileURLToPath(new URL("./fixtures/storage-json-provider", import.meta.url));
  await command(host, { action: "install", spec: fixture, pluginDataRoot });
  await assert.rejects(
    command(host, { action: "set-enabled", pluginId: "@deepseek-ai/dsh-storage-json", enabled: true }),
    (error) => {
      assert.match(error.message, /amitiaTestStorage/);
      return true;
    },
  );
});

test("treats declaration-only packages as libraries instead of mounting them", async () => {
  const host = createHost();
  const pluginDataRoot = await mkdtemp(join(tmpdir(), "amitia-dsh-declaration-"));
  const fixture = fileURLToPath(new URL("./fixtures/declaration-only-plugin", import.meta.url));
  await command(host, { action: "install", spec: fixture, pluginDataRoot });
  delete globalThis.__dshDeclarationProbe;
  await command(host, { action: "set-enabled", pluginId: "@amitia-test/declaration-only-plugin", enabled: true });
  const listed = await command(host, { action: "list" });
  const record = listed.packages.find((item) => item.pluginId === "@amitia-test/declaration-only-plugin");
  assert.equal(record.active, true);
  assert.equal(record.activationError, null);
  assert.equal(globalThis.__dshDeclarationProbe, undefined);
});

test("does not invoke prototype getters while deciding whether a package is declaration-only", async () => {
  const host = createHost();
  const pluginDataRoot = await mkdtemp(join(tmpdir(), "amitia-dsh-getter-"));
  const fixture = fileURLToPath(new URL("./fixtures/getter-prototype-plugin", import.meta.url));
  await command(host, { action: "install", spec: fixture, pluginDataRoot });
  delete globalThis.__dshGetterProbe;
  await command(host, { action: "set-enabled", pluginId: "@amitia-test/getter-prototype-plugin", enabled: true });
  const listed = await command(host, { action: "list" });
  const record = listed.packages.find((item) => item.pluginId === "@amitia-test/getter-prototype-plugin");
  assert.equal(record.activationError, null);
  assert.equal(record.active, true);
  assert.deepEqual(globalThis.__dshGetterProbe, { constructed: true });
});

function createRecordingHost(overrides = {}) {
  const base = createHost();
  const calls = [];
  return {
    calls,
    records: base.records,
    async call(method, input) {
      calls.push({ method, input });
      if (overrides[method]) return overrides[method](input);
      return base.call(method, input);
    },
    async callTargeted(method, input, scopeSnapshotId) {
      calls.push({ method, input, scopeSnapshotId });
      if (overrides[method]) return overrides[method](input);
      throw new Error(`unexpected targeted host method ${method}`);
    },
  };
}

test("maps DSH workspace mutations onto the generic Amitia project routes", async () => {
  const created = Promise.withResolvers();
  const host = createRecordingHost({
    "host.project.list": (input) => ({ items: [{ id: "project-1", title: "Project" }], nextCursor: "", limit: input.limit }),
    "host.project.create": (input) => {
      created.resolve(input);
      return {
        project: {
          id: "project-9",
          title: input.title,
          workspaces: [{ id: "workspace-9", name: input.title, kind: "local" }],
          createdAt: "2026-11-15T00:00:00Z",
          updatedAt: "2026-11-15T00:00:00Z",
        },
      };
    },
    "host.project.update": (input) => ({ projectId: input.projectId, revision: 2 }),
    "host.project.delete": (input) => ({ projectId: input.projectId, deleted: true }),
  });
  assert.deepEqual(
    await command(host, {
      action: "project-catalog",
      limit: 20,
      cursor: "cursor-1",
      spaceId: "space-1",
      characterId: "character-1",
    }),
    { items: [{ id: "project-1", title: "Project" }], nextCursor: "", limit: 20 },
  );
  assert.deepEqual(host.calls.filter((entry) => entry.method === "host.project.list").at(-1).input, {
    spaceId: "space-1",
    characterId: "character-1",
    limit: 20,
    cursor: "cursor-1",
  });
  const createResult = await command(host, {
    action: "project-create",
    directoryPath: "/Users/example/vault/",
    requestId: "create-request-1",
  });
  assert.deepEqual(await created.promise, {
    title: "vault",
    directoryPath: "/Users/example/vault/",
    requestId: "create-request-1",
  });
  assert.equal(createResult.workspace.workspaceId, "workspace-9");
  assert.equal(createResult.workspace.projectId, "project-9");
  assert.equal(createResult.workspace.title, "vault");
  assert.deepEqual(
    (await command(host, { action: "project-workspaces" })).items.map((item) => ({
      workspaceId: item.workspaceId,
      projectId: item.projectId,
      path: item.path,
      title: item.title,
    })),
    [{ workspaceId: "workspace-9", projectId: "project-9", path: "/Users/example/vault/", title: "vault" }],
  );
  await command(host, { action: "project-update", projectId: "project-9", title: "Renamed", pinned: true });
  assert.deepEqual(host.calls.filter((entry) => entry.method === "host.project.update").at(-1).input, {
    projectId: "project-9",
    requestId: "",
    title: "Renamed",
    pinned: true,
  });
  assert.equal((await command(host, { action: "project-workspaces" })).items[0].title, "Renamed");
  assert.deepEqual(await command(host, { action: "project-delete", projectId: "project-9" }), {
    projectId: "project-9",
    deleted: true,
  });
  assert.deepEqual((await command(host, { action: "project-workspaces" })).items, []);
});

test("forwards original Amitia project failures without replacing them", async () => {
  const failure = Object.assign(new Error("host.project.create failed: permission project.manage denied"), {
    code: "PERMISSION_DENIED",
  });
  const host = createRecordingHost({
    "host.project.create": () => {
      throw failure;
    },
  });
  await assert.rejects(
    () => command(host, { action: "project-create", directoryPath: "/Users/example/vault" }),
    (error) => error === failure && error.message === failure.message && error.code === "PERMISSION_DENIED",
  );
  await assert.rejects(() => command(host, { action: "project-create", directoryPath: "  " }), /directoryPath 不能为空/);
  await assert.rejects(() => command(host, { action: "project-update", projectId: "" }), /projectId 不能为空/);
  await assert.rejects(() => command(host, { action: "project-delete", projectId: "" }), /projectId 不能为空/);
});

test("archives a DSH conversation through the persisted session scope snapshot", async () => {
  const calls = [];
  const host = {
    async call(method, input) {
      calls.push({ method, input });
      if (method === "host.state.get") {
        return { found: true, value: { scopeSnapshotId: "conversation-target-1" }, version: 1 };
      }
      throw new Error(`unexpected host method ${method}`);
    },
    async callTargeted(method, input, scopeSnapshotId) {
      calls.push({ method, input, scopeSnapshotId });
      return { conversationId: input.conversationId, archived: input.archived };
    },
  };
  const result = await command(host, {
    action: "conversation-archive",
    conversationId: "conversation-archive-1",
    archived: true,
    stopActivity: true,
  });
  assert.deepEqual(result, { conversationId: "conversation-archive-1", archived: true });
  assert.deepEqual(calls, [
    { method: "host.state.get", input: { key: "dsh-session:conversation-archive-1" } },
    {
      method: "host.conversation.archive",
      input: { conversationId: "conversation-archive-1", archived: true, stopActivity: true, requestId: "" },
      scopeSnapshotId: "conversation-target-1",
    },
  ]);
});

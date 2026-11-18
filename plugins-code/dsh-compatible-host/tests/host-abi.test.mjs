import assert from "node:assert/strict";
import { fileURLToPath } from "node:url";
import test from "node:test";
import {
  DshEventBus,
  invokeDshTool,
  loadDshHostPlugin,
  normalizeHostPlugin,
} from "../src/runtime/host-abi.mjs";
import { createAmitiaDshHostServices } from "../src/runtime/host-services.mjs";

const fixtureDir = fileURLToPath(new URL("./fixtures/sample-dsh-plugin", import.meta.url));

test("loads a DSH host plugin and invokes its registered tool", async () => {
  const fixtureService = { value: "ready", lastPing: "" };
  const runtime = await loadDshHostPlugin({
    packageDir: fixtureDir,
    entry: "./index.js",
    pluginId: "@amitia-test/dsh-fixture",
    services: { fixtureService },
  });
  assert.deepEqual(await invokeDshTool(runtime, "echo", { value: 7 }), {
    value: 7,
    service: "ready",
  });
  await runtime.eventBus.emit("fixture/ping", "pong");
  assert.equal(fixtureService.lastPing, "pong");
  await runtime.dispose();
});

test("waterfall composes listeners and plugin disposal removes effects", async () => {
  const events = new DshEventBus();
  events.on("value", async (value, next) => {
    const downstream = await next(value + 1);
    return downstream * 2;
  });
  events.on("value", async (value) => value + 3);
  assert.equal(await events.waterfall("value", 4), 16);
});

test("rejects missing injected services with the original cause", async () => {
  await assert.rejects(
    loadDshHostPlugin({
      packageDir: fixtureDir,
      entry: "./index.js",
      pluginId: "@amitia-test/dsh-fixture",
      services: {},
    }),
    /fixtureService/,
  );
});

test("provides and removes DSH host services with the plugin lifecycle", async () => {
  const runtime = await loadDshHostPlugin({
    packageDir: fixtureDir,
    entry: "./index.js",
    pluginId: "@amitia-test/dsh-fixture",
    services: { fixtureService: { value: "ready", lastPing: "" } },
  });
  const dispose = runtime.context.provide("temporary", { value: 1 });
  assert.deepEqual(runtime.context.get("temporary"), { value: 1 });
  runtime.context.set("temporary", { value: 2 });
  assert.deepEqual(runtime.context.temporary, { value: 2 });
  dispose();
  assert.equal(runtime.context.get("temporary"), undefined);
  await runtime.dispose();
});

test("loads an official-style plugin importing the DSH tools package shim", async () => {
  const services = {
    sessionProjections: {
      registrations: [],
      async register(definition) {
        this.registrations.push(definition);
        return () => {};
      },
    },
  };
  const runtime = await loadDshHostPlugin({
    packageDir: fileURLToPath(new URL("./fixtures/official-style-plugin", import.meta.url)),
    entry: "./index.js",
    pluginId: "@amitia-test/official-style-plugin",
    services,
  });
  assert.equal(services.sessionProjections.registrations[0].key, "todo");
  assert.equal(runtime.tools[0].name, "todo_write");
  assert.deepEqual(await invokeDshTool(runtime, "todo_write", { todos: ["one", "two"] }), { count: 2 });
  await runtime.dispose();
});

test("records successful DSH tool results in the live session sequence", async () => {
  const runtime = await loadDshHostPlugin({
    packageDir: fixtureDir,
    entry: "./index.js",
    pluginId: "@amitia-test/tool-result",
    services: { fixtureService: { value: "ready", lastPing: "" } },
  });
  const events = [];
  let persisted = 0;
  const session = {
    get events() {
      return events;
    },
    append(type, data) {
      const event = { type, data, seq: events.length + 1, time: Date.now() };
      events.push(event);
      return event;
    },
    async whenPersisted() {
      persisted += 1;
    },
  };
  const value = await invokeDshTool(runtime, "echo", { value: 7 }, {
    callId: "call-7",
    agent: { session },
  });
  assert.deepEqual(value, { value: 7, service: "ready" });
  assert.equal(events.length, 1);
  assert.equal(events[0].type, "tool/result");
  assert.equal(events[0].data.message.toolCallId, "call-7");
  assert.deepEqual(events[0].data.liveResult, value);
  assert.equal(persisted, 1);
  await runtime.dispose();
});

test("restores the persisted sequence before a tool runs in a reopened session", async () => {
  const restoration = Promise.withResolvers();
  const previous = { type: "tool/result", data: { liveResult: { old: true } }, seq: 0, time: 1, surfaceOp: "append" };
  let stored;
  let executions = 0;
  const services = createAmitiaDshHostServices({
    async call(method, input) {
      if (method === "host.state.get") return restoration.promise;
      assert.equal(method, "host.state.cas");
      assert.equal(input.expectedVersion, 3);
      stored = input.newValue;
      return { swapped: true };
    },
  });
  const agent = services.agents.agentForTool("restored-session");
  const runtime = {
    tools: [{
      name: "echo",
      execute(input) {
        executions++;
        return input;
      },
    }],
  };
  const invocation = invokeDshTool(runtime, "echo", { fresh: true }, { agent, callId: "fresh-call" });
  await Promise.resolve();
  assert.equal(executions, 0);
  restoration.resolve({ found: true, version: 3, value: { version: 2, conversationId: "restored-session", events: [previous] } });
  assert.deepEqual(await invocation, { fresh: true });
  assert.deepEqual(stored.events.map(event => [event.seq, event.data.liveResult]), [
    [0, { old: true }],
    [1, { fresh: true }],
  ]);
});

test("preserves tool and persistence errors when both fail", async () => {
  const toolError = new Error("tool failure");
  const persistenceError = new Error("state write failure");
  const runtime = { tools: [{ name: "fail", execute() { throw toolError; } }] };
  const agent = { session: { async whenPersisted() { throw persistenceError; } } };
  await assert.rejects(invokeDshTool(runtime, "fail", {}, { agent }), error => {
    assert.ok(error instanceof AggregateError);
    assert.deepEqual(error.errors, [toolError, persistenceError]);
    assert.equal(error.cause, toolError);
    assert.ok(error.message.includes(toolError.stack));
    assert.ok(error.message.includes(persistenceError.stack));
    return true;
  });
});

test("namespaces the container agents facade so the official registry keeps its service name", async () => {
  const services = createAmitiaDshHostServices({ call: async () => ({}) });
  const containerPlugin = await loadDshHostPlugin({
    packageDir: fileURLToPath(new URL("./fixtures/agent-create-plugin", import.meta.url)),
    entry: "./index.js",
    pluginId: "@amitia-test/agent-create-plugin",
    services,
  });
  assert.deepEqual(containerPlugin.tools.map(tool => tool.name), [
    "agent_create_probe",
    "agent_dispose_probe",
    "agent_resume_probe",
    "agent_inject_probe",
  ]);
  await containerPlugin.dispose();
  await assert.rejects(
    loadDshHostPlugin({
      packageDir: fileURLToPath(new URL("./fixtures/official-agents-plugin", import.meta.url)),
      entry: "./index.js",
      pluginId: "@amitia-test/official-agents-plugin",
      services,
    }),
    error => {
      assert.equal(error.code, "DSH_HOST_SERVICE_MISSING");
      assert.ok(error.message.includes("agents"));
      return true;
    },
  );
});

test("namespaces the container sessionProjections facade so the official registry keeps its service name", async () => {
  const services = createAmitiaDshHostServices({ call: async () => ({}) });
  const containerPlugin = await loadDshHostPlugin({
    packageDir: fileURLToPath(new URL("./fixtures/official-style-plugin", import.meta.url)),
    entry: "./index.js",
    pluginId: "@amitia-test/official-style-plugin",
    services,
  });
  assert.equal(services.sessionProjections.list()[0].key, "todo");
  await containerPlugin.dispose();
  await assert.rejects(
    loadDshHostPlugin({
      packageDir: fileURLToPath(new URL("./fixtures/official-session-projections-plugin", import.meta.url)),
      entry: "./index.js",
      pluginId: "@amitia-test/official-session-projections-plugin",
      services,
    }),
    error => {
      assert.equal(error.code, "DSH_HOST_SERVICE_MISSING");
      assert.ok(error.message.includes("sessionProjections"));
      return true;
    },
  );
});

test("namespaces the container llm bridge so the official llm runtime keeps its service name", async () => {
  const calls = [];
  const services = createAmitiaDshHostServices({
    async call(method, input) {
      calls.push({ method, input });
      return { method };
    },
  });
  const containerPlugin = await loadDshHostPlugin({
    packageDir: fileURLToPath(new URL("./fixtures/amitia-llm-plugin", import.meta.url)),
    entry: "./index.js",
    pluginId: "@amitia-test/amitia-llm-plugin",
    services,
  });
  const fixture = await import(fileURLToPath(new URL("./fixtures/amitia-llm-plugin/index.js", import.meta.url)));
  await fixture.pending;
  assert.equal(calls[0].method, "host.provider.invoke");
  assert.deepEqual(calls[0].input, { provider: "deepseek", model: "deepseek-chat" });
  await containerPlugin.dispose();
  await assert.rejects(
    loadDshHostPlugin({
      packageDir: fileURLToPath(new URL("./fixtures/official-llm-plugin", import.meta.url)),
      entry: "./index.js",
      pluginId: "@amitia-test/official-llm-plugin",
      services,
    }),
    error => {
      assert.equal(error.code, "DSH_HOST_SERVICE_MISSING");
      assert.ok(error.message.includes("llm"));
      return true;
    },
  );
});

test("规范化官方插件时保留 Config schema", () => {
  const Config = { marker: true };
  const moduleLike = {
    name: "official-config-plugin",
    inject: [],
    apply() {},
    Config,
    [Symbol.toStringTag]: "Module",
  };
  const normalized = normalizeHostPlugin(moduleLike, "official-config-plugin");
  assert.equal(normalized.Config, Config);

  const functional = () => {};
  functional.Config = Config;
  const functionNormalized = normalizeHostPlugin({ default: functional }, "functional-config-plugin");
  assert.equal(functionNormalized.Config, Config);
});

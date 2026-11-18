import assert from "node:assert/strict";
import test from "node:test";
import { createAmitiaDshHostServices } from "../src/runtime/host-services.mjs";

test("maps DSH host services to Amitia generic host calls", async () => {
  const calls = [];
  const host = {
    async call(method, input) {
      calls.push({ method, input });
      if (method === "host.event.subscribe") return { subscriptionId: "sub-1" };
      if (method === "host.conversation.turn.interrupt") return { interrupted: true, turnId: "turn-1" };
      if (method === "host.conversation.turn.await") return { idle: true };
      return { method };
    },
  };
  const services = createAmitiaDshHostServices(host);
  await services.network.request({ url: "https://example.com" });
  await services.workspace.read({ path: "/tmp/file" });
  await services.memory.query({ query: "hello" });
  await services.schedule.catalog();
  await services.approval.ask({ title: "Continue?" });
  await services.sessions.read({ conversationId: "c1" });
  await services.llm.invoke({ provider: "deepseek" });
  const agent = services.agents.agentForTool("c1");
  await agent.followup({ content: [{ type: "text", text: "hello" }] });
  await agent.steer("next");
  await agent.inject({ text: "context" });
  assert.equal(await agent.cancel({ kind: "user" }, { keepInbox: true }), undefined);
  assert.equal(await agent.whenIdle(), undefined);
  const subscription = await services.events.subscribe("extension.updated", "entry");
  await subscription.dispose();
  assert.deepEqual(calls.map((item) => item.method), [
    "host.network.request",
    "host.workspace.read",
    "host.memory.query",
    "host.schedule.list",
    "host.ui.dialog",
    "host.conversation.read",
    "host.provider.invoke",
    "host.state.get",
    "host.conversation.message.send",
    "host.conversation.message.append",
    "host.conversation.message.append",
    "host.conversation.turn.interrupt",
    "host.conversation.turn.await",
    "host.event.subscribe",
    "host.event.unsubscribe",
  ]);
  assert.deepEqual(calls.find((item) => item.method === "host.ui.dialog").input, { message: "Continue?" });
  assert.deepEqual(calls.find((item) => item.method === "host.conversation.turn.interrupt").input, {
    conversationId: "c1",
    cause: { kind: "user" },
    keepInbox: true,
  });
  assert.deepEqual(calls.filter((item) => item.method === "host.conversation.message.append").map((item) => item.input.parts), [
    [{ type: "text", content: "next" }],
    [{ type: "text", content: "context" }],
  ]);
  assert.equal(calls.filter((item) => item.method === "host.conversation.message.append").every((item) => item.input.role === "user"), true);
});

test("tracks system prompt sections and returns their current snapshot", () => {
  const services = createAmitiaDshHostServices({ call: async () => ({}) });
  const disposeA = services.systemPrompt.section({ name: "a", order: 20, text: "A" });
  services.systemPrompt.section({ name: "b", order: 10, text: "B" });
  services.systemPrompt.context({ name: "working-directory", order: 100, text: "/workspace" });
  services.systemPrompt.variable("target", () => "production");
  assert.deepEqual(services.systemPrompt.snapshot().sections.map((item) => item.name), ["b", "a"]);
  assert.deepEqual(services.systemPrompt.snapshot().contexts.map((item) => item.name), ["working-directory"]);
  assert.deepEqual(services.systemPrompt.snapshot().variables, { target: "production" });
  disposeA();
  assert.deepEqual(services.systemPrompt.snapshot().sections.map((item) => item.name), ["b"]);
});

test("creates and resumes DSH sessions through the Amitia conversation store", async () => {
  const calls = [];
  const host = {
    async call(method, input) {
      calls.push({ method, input });
      if (method === "host.state.get") return { found: false, version: 0 };
      if (method === "host.conversation.create") {
        return { conversationId: "conversation-created", id: "conversation-created", title: input.title };
      }
      throw new Error(`unexpected host method ${method}`);
    },
  };
  const services = createAmitiaDshHostServices(host);
  const session = await services.sessions.create({
    title: "Created session",
    requestId: "create-session-1",
    channel: "web",
  });
  assert.equal(session.id, "conversation-created");
  assert.equal(session.getSnapshot().id, "conversation-created");
  assert.equal(services.sessions.resume("conversation-created").id, "conversation-created");
  assert.deepEqual(calls.find((item) => item.method === "host.conversation.create").input, {
    title: "Created session",
    projectId: "",
    channel: "web",
    peerId: "",
    requestId: "create-session-1",
  });
});

test("creates and disposes a live DSH agent without deleting its persisted session", async () => {
  const calls = [];
  const targeted = [];
  let state;
  let version = 0;
  const host = {
    async call(method, input) {
      calls.push({ method, input });
      if (method === "host.state.get") return state === undefined ? { found: false, version } : { found: true, value: state, version };
      if (method === "host.state.cas") {
        if (input.expectedVersion !== version) return { swapped: false, version };
        state = structuredClone(input.newValue);
        version += 1;
        return { swapped: true, version };
      }
      if (method === "host.conversation.create") return { conversationId: input.conversationId, scopeSnapshotId: "scope-agent-session-1" };
      throw new Error(`unexpected host method ${method}`);
    },
    async callTargeted(method, input, scopeSnapshotId) {
      targeted.push({ method, input, scopeSnapshotId });
      if (method === "host.conversation.turn.interrupt") {
        throw new Error("host.call host.conversation.turn.interrupt: state_conflict: conversation turn is unavailable: conversation agent-session-1 has no active turn");
      }
      if (method === "host.conversation.turn.await") return { idle: true };
      throw new Error(`unexpected targeted host method ${method}`);
    },
  };
  const services = createAmitiaDshHostServices(host);
  const handle = await services.agents.create({ sessionId: "agent-session-1" });
  assert.equal(handle.agent.id, "agent-session-1");
  assert.equal(services.agents.get("agent-session-1"), handle.agent);
  assert.deepEqual(services.agents.list().map((agent) => agent.id), ["agent-session-1"]);
  assert.deepEqual(calls.find((item) => item.method === "host.conversation.create").input, {
    conversationId: "agent-session-1",
    requestId: "dsh-agent-create:agent-session-1",
    title: "",
    projectId: "",
    channel: "extension",
    peerId: "",
  });
  await handle.dispose();
  assert.equal(services.agents.get("agent-session-1"), undefined);
  assert.deepEqual(services.agents.list(), []);
  assert.equal(calls.some((item) => item.method === "host.state.delete"), false);
  assert.equal(calls.some((item) => item.method === "host.conversation.delete"), false);
  assert.deepEqual(targeted.map((item) => item.method), [
    "host.conversation.turn.interrupt",
    "host.conversation.turn.await",
  ]);
  assert.equal(targeted.every((item) => item.scopeSnapshotId === "scope-agent-session-1"), true);
  assert.equal(state.scopeSnapshotId, "scope-agent-session-1");
  await handle.dispose();
  assert.equal(targeted.length, 2);
});

test("resumes an owned DSH agent with official AgentHandle semantics", async () => {
  const calls = [];
  let state = { version: 2, conversationId: "resume-session", scopeSnapshotId: "scope-resume", events: [] };
  const host = {
    async call(method, input) {
      calls.push({ method, input });
      if (method === "host.state.get") return { found: true, value: state, version: 1 };
      throw new Error(`unexpected host method ${method}`);
    },
    async callTargeted(method, input, scopeSnapshotId) {
      calls.push({ method, input, scopeSnapshotId });
      if (method === "host.conversation.turn.interrupt") return { interrupted: true, turnId: "turn-resume" };
      if (method === "host.conversation.turn.await") return { idle: true };
      throw new Error(`unexpected targeted host method ${method}`);
    },
  };
  const services = createAmitiaDshHostServices(host);
  const handle = await services.agents.resume({ resumeSessionId: "resume-session" });
  assert.equal(handle.agent.id, "resume-session");
  assert.equal(services.agents.get("resume-session"), handle.agent);
  await handle.dispose();
  assert.equal(services.agents.get("resume-session"), undefined);
  assert.equal(calls.some((item) => item.method === "host.conversation.delete"), false);
  assert.equal(calls.find((item) => item.method === "host.conversation.turn.interrupt").scopeSnapshotId, "scope-resume");
});

test("rejects DSH agent creation options without an Amitia session projection", async () => {
  const calls = [];
  const services = createAmitiaDshHostServices({
    async call(method) {
      calls.push(method);
      return {};
    },
  });
  await assert.rejects(
    services.agents.create({ sessionId: "agent-cwd", meta: { cwd: "/workspace" } }),
    /session metadata cwd requires a DSH Session projection/,
  );
  await assert.rejects(
    services.agents.create({ sessionId: "agent-setup", setup() {} }),
    /setup requires a DSH scoped Agent context/,
  );
  await assert.rejects(
    services.agents.create({ sessionId: "agent-seed", seed: [] }),
    /seed and inheritedEventCount require DSH fork history import/,
  );
  assert.deepEqual(calls, []);
});

test("exposes registered subagent providers and fails closed for unmanaged activation", async () => {
  const services = createAmitiaDshHostServices({ call: async () => ({}) });
  const provider = { name: "spawn", capabilities: {}, inheritsParentContext: false };
  const dispose = services.subagents.registerProvider(provider);
  assert.equal(services.subagents.getProvider("spawn"), provider);
  await assert.rejects(
    services.subagents.startActivation({ provider: "spawn", prompt: [] }),
    (error) => error.code === "DSH_HOST_OPERATION_UNAVAILABLE" && /startActivation/.test(error.message),
  );
  dispose();
  assert.equal(services.subagents.getProvider("spawn"), undefined);
});

test("maps a DSH external subagent provider start to an activation handle", async () => {
  const services = createAmitiaDshHostServices({ call: async () => ({}) });
  let disposed = false;
  services.subagents.registerProvider({
    name: "external",
    capabilities: { agentOptions: false, agentPreset: false, outputSchema: false, depthLimit: false },
    inheritsParentContext: false,
    async start(request) {
      assert.equal(request.cwd, process.cwd());
      return {
        id: "child-1",
        result: Promise.resolve({ output: [{ type: "text", text: "done" }], stopReason: "completed" }),
        async dispose() {
          disposed = true;
        },
      };
    },
  });
  const activation = await services.subagents.startActivation({
    provider: "external",
    label: "analysis",
    prompt: [{ type: "text", text: "inspect" }],
    signal: new AbortController().signal,
    delivery: "caller",
  });
  assert.equal(activation.childId, "child-1");
  await activation.result;
  assert.equal((await services.subagents.listDescendants()).length, 1);
  assert.deepEqual(services.subagents.interruptByUser("child-1"), { accepted: true });
  await Promise.resolve();
  assert.equal((await services.subagents.listDescendants()).length, 0);
  await activation.dispose();
  assert.equal(disposed, true);
});

test("maps DSH MCP resource providers to a scoped register and request facade", async () => {
  const services = createAmitiaDshHostServices({ call: async () => ({}) });
  const provider = {
    async request(request) {
      return { uri: request.uri, method: request.method };
    },
  };
  const dispose = services.mcpResources.register("fixture", provider);
  assert.deepEqual(await services.mcpResources.request("fixture", { method: "resources/read", uri: "mem://one" }), {
    uri: "mem://one",
    method: "resources/read",
  });
  assert.deepEqual(services.mcpResources.list(), ["fixture"]);
  dispose();
  assert.deepEqual(services.mcpResources.list(), []);
});

test("runs cancellable DSH background jobs", async () => {
  const services = createAmitiaDshHostServices({ call: async () => ({}) });
  let canceled = false;
  const job = services.jobs.start({
    kind: "fixture",
    run: async ({ signal }) => {
      await new Promise((resolve, reject) => {
        const timer = setTimeout(resolve, 20);
        signal.addEventListener("abort", () => {
          canceled = true;
          clearTimeout(timer);
          reject(signal.reason);
        }, { once: true });
      });
      return "done";
    },
  });
  assert.equal(job.kind, "background");
  await Promise.resolve();
  services.jobs.cancel(job.jobId, "test");
  assert.equal(canceled, true);
  assert.equal(services.jobs.get(job.jobId).status, "cancelled");
  await assert.rejects(job.done, /test/);
});

test("tracks maintenance tasks in whenIdle and cancels them with the agent", async () => {
  const calls = [];
  const host = {
    async call(method, input) {
      calls.push({ method, input });
      if (method === "host.state.get") return { found: false, version: 0 };
      if (method === "host.conversation.turn.await") return { idle: true };
      if (method === "host.conversation.turn.interrupt") return { interrupted: true };
      throw new Error(`unexpected host method ${method}`);
    },
  };
  const services = createAmitiaDshHostServices(host);
  const agent = services.agents.agentForTool("maintenance-session");
  let maintenanceSettled = false;
  let aborted = false;
  const maintenance = agent.runMaintenance((signal) => new Promise((resolve) => {
    signal.addEventListener("abort", () => {
      aborted = true;
      resolve("stopped");
    }, { once: true });
  }).finally(() => {
    maintenanceSettled = true;
  }));
  assert.throws(() => agent.runMaintenance(() => Promise.resolve()), /already owns/);
  const idle = agent.whenIdle();
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.equal(maintenanceSettled, false);
  await agent.cancel({ kind: "user" });
  assert.equal(aborted, true);
  await idle;
  assert.equal(await maintenance, "stopped");
  assert.equal(calls.some((item) => item.method === "host.conversation.turn.interrupt"), true);
});

test("persists and restores DSH session events through Amitia state", async () => {
  let value;
  let version = 0;
  const host = {
    async call(method, input) {
      if (method === "host.state.get") {
        return value === undefined ? { found: false, version } : { found: true, value, version };
      }
      if (method === "host.state.cas") {
        if (Number(input.expectedVersion) !== version) return { swapped: false, version };
        value = structuredClone(input.newValue);
        version += 1;
        return { swapped: true, version };
      }
      throw new Error(`unexpected host method ${method}`);
    },
  };
  const services = createAmitiaDshHostServices(host);
  services.sessionProjections.register({
    key: "counter",
    init: () => 0,
    apply: (state, event) => event.type === "turn/start" ? state + Number(event.data.turn) : state,
  });
  const agent = services.agents.agentForTool("conversation-persist");
  agent.session.append("turn/start", { turn: 2 });
  agent.session.append("turn/start", { turn: 3 });
  assert.equal(agent.model.current(), undefined);
  agent.model.select({ provider: "deepseek", model: "deepseek-chat", reasoningEffort: "high" });
  assert.deepEqual(agent.model.current(), { provider: "deepseek", model: "deepseek-chat", reasoningEffort: "high" });
  assert.equal(agent.options.provider, "deepseek");
  assert.equal(agent.options.model, "deepseek-chat");
  assert.equal(agent.options.reasoningEffort, "high");
  await agent.session.whenPersisted();
  assert.equal(agent.session.projection(services.sessionProjections.list()).counter, 5);

  const restored = createAmitiaDshHostServices(host);
  restored.sessionProjections.register({
    key: "counter",
    init: () => 0,
    apply: (state, event) => event.type === "turn/start" ? state + Number(event.data.turn) : state,
  });
  const restoredAgent = restored.agents.agentForTool("conversation-persist");
  await restoredAgent.session.hydrated;
  assert.equal(restoredAgent.session.events.length, 3);
  assert.equal(restoredAgent.session.projection(restored.sessionProjections.list()).counter, 5);
  assert.deepEqual(restoredAgent.model.current(), { provider: "deepseek", model: "deepseek-chat", reasoningEffort: "high" });
});

test("jobs 门面提供官方 kill 契约", async () => {
  const host = { async call() { return { ok: true }; } };
  const services = createAmitiaDshHostServices(host);
  const started = services.jobs.start({ id: "job-kill-1", label: "probe", run: () => new Promise(() => {}) });
  assert.equal(started.id, "job-kill-1");
  assert.equal(started.jobId, "job-kill-1");
  const jobId = started.id;
  const result = services.jobs.kill(jobId, { id: "agent-1" }, "caller aborted");
  assert.equal(result, "requested");
  assert.equal(services.jobs.get("job-kill-1").status, "cancelled");
  assert.throws(() => services.jobs.kill("missing", {}, "x"), /not found/);

  const second = services.jobs.start({ id: "job-kill-2", label: "probe", run: () => new Promise(() => {}) });
  const objectResult = services.jobs.kill({ id: second.id }, { id: "agent-1" }, "aborted by caller");
  assert.equal(objectResult, "requested");
  assert.equal(services.jobs.get("job-kill-2").status, "cancelled");
});

test("jobs 门面提供 wait/read/remove 契约", async () => {
  const services = createAmitiaDshHostServices({ async call() { return { ok: true }; } });
  const started = services.jobs.start({ id: "job-stream-1", label: "probe", run: async ({ jobId }) => "done:" + jobId });
  const settled = await services.jobs.wait(started.id, 1000);
  assert.equal(settled.status, "completed");
  assert.equal(settled.id, "job-stream-1");
  assert.equal(services.jobs.get("job-stream-1").status, "completed");
  const firstRead = services.jobs.read(started.id);
  assert.equal(firstRead.lossy, false);
  assert.equal(firstRead.chunks.length, 1);
  assert.equal(firstRead.chunks[0].text, "done:job-stream-1");
  assert.deepEqual(firstRead.job.output, { spillPaths: [] });
  assert.equal(firstRead.job.status, "completed");
  const consumed = services.jobs.read(started.id);
  assert.equal(consumed.result, "done:job-stream-1");
  const at = services.jobs.readAt(started.id, 0);
  assert.equal(at.chunks[0].text, "done:job-stream-1");
  assert.equal(at.lossy, false);
  assert.equal(services.jobs.remove({ id: "job-stream-1" }), true);
  assert.equal(services.jobs.get("job-stream-1"), undefined);
  await assert.rejects(async () => services.jobs.wait("missing"), /not found/);
});

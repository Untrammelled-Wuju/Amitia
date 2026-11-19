import assert from "node:assert/strict";
import test from "node:test";
import { createAmitiaDshHostServices } from "../src/runtime/host-services.mjs";
import * as modelAdapterModule from "../src/runtime/model-adapter.mjs";

function createHost(handlers = {}) {
  const calls = [];
  const host = {
    calls,
    async call(method, input) {
      calls.push({ method, input });
      const handler = handlers[method];
      if (typeof handler === "function") return handler(input, host);
      return { method, input };
    },
  };
  return host;
}

function scheduleFixture() {
  const definition = {
    scheduleId: "sched-1",
    name: "每分钟巡检",
    description: "检查构建",
    timezone: "UTC",
    trigger: { type: "interval", interval: { interval: 60_000_000_000 } },
    target: {
      type: "session_prompt",
      targetId: "session-1",
      inputTemplate: {
        prompt: "检查构建",
        dshTitle: "每分钟巡检",
        dshTargetKind: "session",
        dshSessionId: "session-1",
      },
    },
  };
  const ended = {
    scheduleId: "sched-2",
    name: "已结束",
    description: "已结束",
    timezone: "UTC",
    trigger: { type: "interval", interval: { interval: 60_000_000_000 } },
    target: {
      type: "session_prompt",
      targetId: "session-2",
      inputTemplate: {
        prompt: "已结束",
        dshTitle: "已结束",
        dshTargetKind: "session",
        dshSessionId: "session-2",
      },
    },
  };
  const states = {
    "sched-1": { status: "active", nextScheduledAt: "2026-01-01T00:00:00Z", updatedAt: "2026-01-01T00:00:00Z" },
    "sched-2": { status: "expired", nextScheduledAt: "2025-01-01T00:00:00Z", updatedAt: "2025-01-01T00:00:00Z" },
  };
  return { schedules: [definition, ended], states };
}

test("approval.request 把 DSH 审批请求映射到 host.approval.request", async () => {
  const host = createHost({ "host.approval.request": () => ({ approved: true }) });
  const services = createAmitiaDshHostServices(host);
  assert.equal(await services.approval.request({
    toolName: "bash",
    reason: "需要执行构建",
    callId: "call-1",
    id: "req-1",
    conversationId: "session-1",
    timeoutMs: 1500,
    riskLevel: "high",
  }), "allowed-once");
  assert.deepEqual(host.calls, [{
    method: "host.approval.request",
    input: {
      toolName: "bash",
      arguments: "插件请求执行工具：bash\n原因：需要执行构建",
      toolCallId: "call-1",
      requestId: "req-1",
      conversationId: "session-1",
      riskLevel: "high",
      timeoutMs: 1500,
    },
  }]);
});

test("approval.request 把拒绝与非布尔结果区分处理", async () => {
  const host = createHost({
    "host.approval.request": (input) => (input.toolName === "deny" ? { approved: false } : { approved: "yes" }),
  });
  const services = createAmitiaDshHostServices(host);
  assert.equal(await services.approval.request({ toolName: "deny" }), "rejected");
  await assert.rejects(() => services.approval.request({ toolName: "odd" }), (error) => {
    assert.match(error.message, /Amitia host\.approval\.request 返回了非法结果: \{"approved":"yes"\}/);
    return true;
  });
});

test("approval.request 原样抛出宿主错误", async () => {
  const thrown = Object.assign(new Error("host.approval.request failed: approval broker not configured"), {
    code: "host_unavailable",
    response: { code: 503, msg: "approval broker not configured" },
  });
  const host = createHost({
    "host.approval.request": () => {
      throw thrown;
    },
  });
  const services = createAmitiaDshHostServices(host);
  await assert.rejects(() => services.approval.request({ toolName: "bash" }), (error) => {
    assert.equal(error, thrown);
    assert.equal(error.code, "host_unavailable");
    assert.deepEqual(error.response, { code: 503, msg: "approval broker not configured" });
    return true;
  });
});

test("approval 策略按 agent 作用域存取并拒绝非法策略", () => {
  const host = createHost();
  const services = createAmitiaDshHostServices(host);
  const agent = { id: "session-1", session: { id: "session-1" } };
  assert.equal(services.approval.overrideOf(agent.session), undefined);
  services.approval.setPolicy(agent, { mode: "ask", tools: ["bash"] });
  assert.deepEqual(services.approval.overrideOf(agent.session), { mode: "ask", tools: ["bash"] });
  assert.equal(services.approval.overrideOf({ id: "session-2" }), undefined);
  assert.equal(services.approval.overrideOf(undefined), undefined);
  assert.throws(() => services.approval.setPolicy({ id: "session-2" }, "allow"), (error) => {
    assert.match(error.message, /DSH approval\.setPolicy 需要 ApprovalPolicy 对象: "allow"/);
    return true;
  });
  assert.throws(() => services.approval.setPolicy({}, { mode: "ask" }), /DSH approval\.setPolicy 需要带 id 的 agent/);
  assert.equal(host.calls.length, 0);
});

test("userQuestions.ask 把问题与选项映射到 host.ui.dialog", async () => {
  const host = createHost({ "host.ui.dialog": () => ({ dialogId: "answer-1", result: "继续" }) });
  const services = createAmitiaDshHostServices(host);
  assert.deepEqual(
    await services.userQuestions.ask({
      id: "question-1",
      question: "是否继续部署？",
      options: [{ label: "继续" }, { label: "取消" }],
    }),
    { question: "是否继续部署？", selected: "继续", dialogId: "answer-1" },
  );
  assert.deepEqual(host.calls, [{
    method: "host.ui.dialog",
    input: { dialogId: "question-1", message: "是否继续部署？", buttons: ["继续", "取消"] },
  }]);
  const fromQuestions = createHost({ "host.ui.dialog": () => ({ result: "OK" }) });
  const nested = createAmitiaDshHostServices(fromQuestions);
  await nested.userQuestions.ask({ questions: [{ prompt: "选择范围", choices: ["全部", "单条"] }] });
  assert.deepEqual(fromQuestions.calls[0].input, { message: "选择范围", buttons: ["全部", "单条"] });
});

test("userQuestions.ask 在缺少问题文本或宿主返回非法结果时原样失败", async () => {
  const host = createHost({ "host.ui.dialog": () => ({ dialogId: "answer-1" }) });
  const services = createAmitiaDshHostServices(host);
  await assert.rejects(() => services.userQuestions.ask({ sessionId: "session-1" }), (error) => {
    assert.match(error.message, /DSH userQuestions\.ask 需要 question\/prompt\/message 文本: \{"sessionId":"session-1"\}/);
    return true;
  });
  assert.equal(host.calls.length, 0);
  await assert.rejects(() => services.userQuestions.ask({ question: "继续？" }), (error) => {
    assert.match(error.message, /Amitia host\.ui\.dialog 返回了非法结果: \{"dialogId":"answer-1"\}/);
    return true;
  });
});

test("userQuestions.askTimed 复用提问并在超时后返回 timeout", async () => {
  const host = createHost({ "host.ui.dialog": () => ({ dialogId: "answer-1", result: "继续" }) });
  const services = createAmitiaDshHostServices(host);
  assert.deepEqual(
    await services.userQuestions.askTimed({ question: "继续？", options: ["继续", "取消"] }, "call-1", 500),
    { status: "answered", answer: { question: "继续？", selected: "继续", dialogId: "answer-1" } },
  );
  assert.deepEqual(host.calls[0].input, { dialogId: "call-1", message: "继续？", buttons: ["继续", "取消"] });
  const pending = createHost({ "host.ui.dialog": () => new Promise(() => {}) });
  const waiting = createAmitiaDshHostServices(pending);
  assert.deepEqual(await waiting.userQuestions.askTimed({ question: "还在吗？" }, "call-2", 5), { status: "timeout" });
  assert.equal(pending.calls.length, 1);
});

test("approval.ask 用固定确认文案兜底映射到 host.ui.dialog", async () => {
  const host = createHost({ "host.ui.dialog": () => ({ dialogId: "dialog-1", result: "OK" }) });
  const services = createAmitiaDshHostServices(host);
  assert.deepEqual(await services.approval.ask({}), { dialogId: "dialog-1", result: "OK" });
  assert.deepEqual(host.calls[0].input, { message: "插件请求确认" });
});

test("schedule 的目录、视图、编辑、历史与清理全部走真实宿主接口", async () => {
  const fixture = scheduleFixture();
  const host = createHost({
    "host.schedule.list": () => fixture,
    "host.schedule.update": (input) => {
      const index = fixture.schedules.findIndex((item) => item.scheduleId === input.scheduleId);
      fixture.schedules[index] = structuredClone(input.definition);
      fixture.states[input.scheduleId] = { ...fixture.states[input.scheduleId], updatedAt: "2026-01-02T00:00:00Z" };
      return { updated: true };
    },
    "host.schedule.cancel": () => ({ cancelled: true }),
    "host.schedule.history": (input) => ({ scheduleId: input.scheduleId, items: [{ deliveryId: "d1" }] }),
  });
  const services = createAmitiaDshHostServices(host);
  const originalDefinition = structuredClone(fixture.schedules[0]);
  const catalog = await services.schedule.catalog();
  assert.deepEqual(catalog.map((item) => [item.id, item.status]), [["sched-2", "ended"], ["sched-1", "active"]]);
  assert.equal((await services.schedule.view({ scheduleId: "sched-1" })).title, "每分钟巡检");
  assert.deepEqual(await services.schedule.view({ scheduleId: "missing" }), { id: "missing", code: "schedule_not_found" });
  const edited = await services.schedule.edit({
    scheduleId: "sched-1",
    patch: { title: "改名后" },
    expectedUpdatedAt: "2026-01-01T00:00:00Z",
  });
  assert.equal(edited.title, "改名后");
  const updateCall = host.calls.find((entry) => entry.method === "host.schedule.update");
  assert.deepEqual(updateCall.input, {
    scheduleId: "sched-1",
    expectedUpdatedAt: "2026-01-01T00:00:00Z",
    definition: {
      ...originalDefinition,
      name: "改名后",
      target: { ...originalDefinition.target, inputTemplate: { ...originalDefinition.target.inputTemplate, dshTitle: "改名后" } },
    },
    enabled: undefined,
  });
  assert.deepEqual(await services.schedule.history({ scheduleId: "sched-1", limit: 5 }), {
    scheduleId: "sched-1",
    items: [{ deliveryId: "d1" }],
  });
  assert.deepEqual(await services.schedule.deleteInactive(), { deleted: 1 });
  assert.deepEqual(
    host.calls.filter((entry) => entry.method === "host.schedule.cancel").map((entry) => entry.input),
    [{ scheduleId: "sched-2" }],
  );
  const activeOnly = await services.schedule.remoteList({ active: true });
  assert.deepEqual(activeOnly.items.map((item) => item.id), ["sched-1"]);
  assert.equal(activeOnly.shown, 1);
  assert.equal(activeOnly.total, 1);
  assert.equal(activeOnly.limit, 100);
  assert.equal(activeOnly.offset, 0);
  assert.equal(activeOnly.items[0].title, "改名后");
  assert.deepEqual(activeOnly.items[0].target, { kind: "session", sessionId: "session-1" });
  const endedOnly = await services.schedule.remoteList({ active: false, limit: 1 });
  assert.deepEqual(endedOnly.items.map((item) => item.id), ["sched-2"]);
});

test("schedule.create 把 DSH 请求写成 Amitia 排程定义并回读目录条目", async () => {
  const fixture = { schedules: [], states: {} };
  const host = createHost({
    "host.schedule.list": () => fixture,
    "host.schedule.create": (input) => {
      fixture.schedules.push(structuredClone(input));
      fixture.states[input.scheduleId] = {
        status: "active",
        nextScheduledAt: "2026-02-01T00:00:00Z",
        updatedAt: "2026-02-01T00:00:00Z",
      };
      return { created: true };
    },
  });
  const services = createAmitiaDshHostServices(host);
  const entry = await services.schedule.create({
    title: "巡检",
    prompt: "跑一遍检查",
    timing: { kind: "after", after_seconds: 60 },
    target: { kind: "session", sessionId: "session-1" },
  });
  const definition = host.calls.find((entryCall) => entryCall.method === "host.schedule.create").input;
  assert.equal(entry.id, definition.scheduleId);
  assert.equal(entry.title, "巡检");
  assert.equal(entry.prompt, "跑一遍检查");
  assert.equal(entry.status, "active");
  assert.equal(entry.kind, "after");
  assert.deepEqual(entry.target, { kind: "session", sessionId: "session-1" });
  assert.equal(definition.name, "巡检");
  assert.equal(definition.trigger.type, "one_shot");
  assert.equal(host.calls[0].method, "host.schedule.create");
  assert.equal(host.calls[1].method, "host.schedule.list");
  await assert.rejects(
    () => services.schedule.create({ title: "", prompt: "x", timing: { kind: "after", after_seconds: 60 } }),
    /DSH Schedule title must contain 1 to 120 characters/,
  );
});

test("schedule 编辑在期望版本不一致时返回冲突而不写宿主", async () => {
  const fixture = scheduleFixture();
  const host = createHost({ "host.schedule.list": () => fixture });
  const services = createAmitiaDshHostServices(host);
  assert.deepEqual(
    await services.schedule.edit({ scheduleId: "sched-1", patch: { title: "x" }, expectedUpdatedAt: "1999-01-01T00:00:00Z" }),
    { id: "sched-1", code: "schedule_conflict" },
  );
  assert.equal(host.calls.some((entry) => entry.method === "host.schedule.update"), false);
  await assert.rejects(() => services.schedule.edit({ patch: { title: "x" } }), /DSH schedule\.edit 需要 scheduleId/);
});

test("sessions 暴露真实会话对象并明确失败的会话生命周期方法", async () => {
  const host = createHost();
  const services = createAmitiaDshHostServices(host);
  const persisted = [];
  const session = {
    id: "session-1",
    events: [],
    async whenPersisted() {
      persisted.push("session-1");
    },
  };
  const agent = { id: "session-1", session };
  const dispose = services.agents.register(agent);
  assert.equal(services.sessions.get("session-1"), session);
  assert.deepEqual(services.sessions.list(), [session]);
  assert.equal(await services.sessions.flush(session), true);
  assert.deepEqual(persisted, ["session-1"]);
  await assert.rejects(() => services.sessions.flush({}), /DSH sessions\.flush 需要容器创建的会话对象/);
  dispose();
  assert.equal(services.sessions.get("session-1"), undefined);
  const disposeProjection = services.sessions.registerMessageProjection({ type: "user/message", project: () => new Map() });
  assert.equal(typeof disposeProjection, "function");
  assert.throws(() => services.sessions.registerMessageProjection({ type: "user/message", project: () => new Map() }), /已经注册/);
  assert.throws(() => services.sessions.registerMessageProjection({ type: "user/message" }), /需要 \{ type, project/);
  disposeProjection();
  for (const method of ["enter", "announce"]) {
    assert.throws(() => services.sessions[method](), (error) => {
      assert.equal(error.code, "DSH_HOST_OPERATION_UNAVAILABLE");
      assert.match(error.message, new RegExp(`DSH sessions\\.${method} 不可用`));
      assert.match(error.message, /宿主缺少接口/);
      assert.equal(error.details.operation, `sessions.${method}`);
      return true;
    }, method);
  }
  assert.equal(host.calls.length, 0);
});

test("sessions.prepare 建立草稿会话并支持发布与丢弃", async () => {
  const host = createHost({
    "host.conversation.create": (input) => ({ conversationId: input.conversationId || "draft-1", draft: input.draft !== false }),
    "host.conversation.delete": () => ({ deleted: true }),
  });
  const services = createAmitiaDshHostServices(host);
  const session = await services.sessions.prepare("draft-1", { title: "草稿" });
  assert.equal(session.id, "draft-1");
  assert.equal(session.draft, true);
  const created = host.calls.find((entry) => entry.method === "host.conversation.create");
  assert.deepEqual(created.input, { draft: true, conversationId: "draft-1", title: "草稿" });
  await session.publish({ title: "正式" });
  assert.equal(session.draft, false);
  const published = host.calls.filter((entry) => entry.method === "host.conversation.create").at(-1);
  assert.deepEqual(published.input, { conversationId: "draft-1", draft: false, title: "正式" });
  assert.equal(await session.drop(), true);
});

test("sessions.fork 派生会话并拒绝无法映射的 atSeq", async () => {
  const host = createHost({ "host.conversation.fork": () => ({ conversationId: "forked-1", forkedFrom: "session-1" }) });
  const services = createAmitiaDshHostServices(host);
  const forked = await services.sessions.fork("session-1");
  assert.equal(forked.id, "forked-1");
  const request = host.calls.find((entry) => entry.method === "host.conversation.fork");
  assert.deepEqual(request.input, { conversationId: "session-1" });
  await assert.rejects(() => services.sessions.fork("session-1", 3), /atSeq 无法映射/);
});

test("agents 发起者栈与注册表可用，缺失的驱动方法明确失败", () => {
  const host = createHost();
  const services = createAmitiaDshHostServices(host);
  const agent = { id: "session-1", session: { id: "session-1" } };
  assert.equal(services.agents.currentInitiator(), undefined);
  assert.throws(() => services.agents.requireInitiator(), /DSH agents\.requireInitiator 当前没有发起者 agent/);
  const observed = services.agents.withInitiator(agent, () => [
    services.agents.currentInitiator(),
    services.agents.requireInitiator(),
  ]);
  assert.deepEqual(observed, [agent, agent]);
  assert.equal(services.agents.currentInitiator(), undefined);
  const outside = services.agents.withInitiator(agent, () => services.agents.withoutInitiator(() => services.agents.currentInitiator()));
  assert.equal(outside, undefined);
  assert.equal(services.agents.currentInitiator(), undefined);
  const leave = services.agents.enter(agent);
  assert.equal(services.agents.currentInitiator(), agent);
  leave();
  assert.equal(services.agents.currentInitiator(), undefined);
  const dispose = services.agents.register(agent);
  assert.equal(services.agents.get("session-1"), agent);
  dispose();
  assert.equal(services.agents.get("session-1"), undefined);
  assert.equal(host.calls.length, 0);
});

test("agents.setFactory 让 create 走注册工厂，announce 在容器上下文派发 agent/created", async () => {
  const host = createHost();
  const serialCalls = [];
  const runtimeHooks = {
    context: {
      async serial(...args) {
        serialCalls.push(args);
      },
    },
  };
  const services = createAmitiaDshHostServices(host, [], {}, runtimeHooks);
  const disposeFactory = services.agents.setFactory(async (options) => ({ id: options.sessionId, session: { id: options.sessionId }, options }));
  const created = await services.agents.create({ sessionId: "session-9" });
  assert.equal(created.id, "session-9");
  assert.deepEqual(created.options, { sessionId: "session-9" });
  assert.equal(host.calls.length, 0);
  disposeFactory();
  assert.throws(() => services.agents.setFactory("not-a-factory"), /需要工厂函数/);
  const agent = { id: "session-1", session: {} };
  services.agents.register(agent);
  await services.agents.announce(agent, "startup");
  assert.equal(serialCalls.length, 1);
  assert.equal(serialCalls[0][0], agent);
  assert.equal(serialCalls[0][1], "agent/created");
  assert.deepEqual(serialCalls[0][2], { agent, source: "startup" });
  await assert.rejects(services.agents.announce(agent, "startup"), /已经公布过/);
  await assert.rejects(services.agents.announce({ id: "session-missing" }, "startup"), /不在活动注册表中/);
  await assert.rejects(services.agents.announce({ id: "session-2" }, ""), /需要 source/);
  const detached = createAmitiaDshHostServices(createHost());
  const detachedAgent = { id: "session-3", session: {} };
  detached.agents.register(detachedAgent);
  await assert.rejects(detached.agents.announce(detachedAgent, "startup"), (error) => {
    assert.equal(error.code, "DSH_HOST_OPERATION_UNAVAILABLE");
    assert.match(error.message, /DSH agents\.announce 不可用/);
    return true;
  });
});


test("agents.turnBoundaryOf 用已注册的 turnBoundary 投影折叠会话事件", () => {
  const host = createHost();
  const services = createAmitiaDshHostServices(host);
  const session = {
    id: "session-1",
    events: [
      { seq: 0, type: "turn/start", time: 1, data: {} },
      { seq: 1, type: "turn/end", time: 2, data: {} },
    ],
  };
  assert.throws(() => services.agents.turnBoundaryOf(session), (error) => {
    assert.equal(error.code, "DSH_HOST_OPERATION_UNAVAILABLE");
    assert.match(error.message, /DSH agents\.turnBoundaryOf 不可用/);
    assert.match(error.message, /宿主缺少接口 sessionProjections 中 key 为 turnBoundary 的投影/);
    assert.deepEqual(error.details.request, { sessionId: "session-1" });
    return true;
  });
  const dispose = services.sessionProjections.register({
    key: "turnBoundary",
    init: () => ({ turns: 0 }),
    apply: (state, event) => (event.type === "turn/end" ? { turns: state.turns + 1 } : state),
  });
  assert.deepEqual(services.agents.turnBoundaryOf(session), { turns: 1 });
  dispose();
  assert.throws(() => services.agents.turnBoundaryOf(session), /DSH agents\.turnBoundaryOf 不可用/);
  assert.throws(() => services.agents.turnBoundaryOf(), /DSH agents\.turnBoundaryOf 需要 Session 对象/);
  assert.equal(host.calls.length, 0);
});

test("subagents.resolveName 按活动子代理命名表解析", async () => {
  const host = createHost();
  const services = createAmitiaDshHostServices(host);
  await assert.rejects(services.subagents.resolveName({}, "worker"), (error) => {
    assert.match(error.message, /找不到名字 "worker" 的子代理/);
    assert.match(error.message, /当前活动子代理：\[\]/);
    return true;
  });
  const disposeProvider = services.subagents.registerProvider({
    name: "local",
    async start() {
      return { id: "child-1", messageId: "message-1", result: { ok: true }, dispose: async () => {} };
    },
  });
  const activation = await services.subagents.startActivation({ provider: "local", name: "worker" });
  assert.equal(activation.childId, "child-1");
  assert.equal(await services.subagents.resolveName({}, "worker"), "child-1");
  await assert.rejects(services.subagents.resolveName({}, "reviewer"), /找不到名字 "reviewer" 的子代理/);
  await activation.dispose();
  await assert.rejects(services.subagents.resolveName({}, "worker"), /找不到名字 "worker" 的子代理/);
  disposeProvider();
  assert.deepEqual(host.calls.map((call) => call.method), ["host.conversation.list"]);
});

test("llm.invoke 保持真实调用，未注册适配器的方法明确失败", async () => {
  const response = { success: true, result: { content: "hi" } };
  const host = createHost({ "host.provider.invoke": () => response });
  const services = createAmitiaDshHostServices(host);
  assert.equal(await services.llm.invoke({ provider: "deepseek", model: "chat" }), response);
  assert.deepEqual(host.calls, [{ method: "host.provider.invoke", input: { provider: "deepseek", model: "chat" } }]);
  for (const method of ["stream", "listModels", "prepareCall", "resolveModelInfo", "providerRetryPolicy"]) {
    assert.throws(() => services.llm[method]({ provider: "deepseek", model: "chat" }), (error) => {
      assert.equal(error.code, "DSH_HOST_OPERATION_UNAVAILABLE");
      assert.match(error.message, new RegExp(`DSH llm\\.${method} 不可用`));
      assert.match(error.message, /宿主缺少接口 host\.(provider|model)\./);
      assert.equal(error.details.operation, `llm.${method}`);
      return true;
    }, method);
  }
  assert.throws(() => services.llm.discoverModels("amitia"), (error) => {
    assert.equal(error.code, "DSH_HOST_OPERATION_UNAVAILABLE");
    assert.match(error.message, /DSH llm\.discoverModels 不可用/);
    assert.deepEqual(error.details.request, { settingsNs: "amitia", request: undefined });
    return true;
  });
  assert.equal(host.calls.length, 1);
});

test("llm.fileRequestText 复用官方句柄文本，缺少可读路径时说明无法访问", () => {
  const host = createHost();
  const services = createAmitiaDshHostServices(host);
  const ref = { type: "file", attachmentId: "file-abcd1234efgh", name: "report.pdf", bytes: 2048 };
  const handle = services.llm.fileRequestText(ref);
  assert.match(handle, /^\[File "report\.pdf" \(2048 bytes, sha256:[0-9a-f]{8}\)/);
  assert.match(handle, /cannot access a readable path/);
  const withPath = services.llm.fileRequestText({ ...ref, uri: "file:///tmp/report.pdf" });
  assert.match(withPath, /verbatim read-only copy saved at "\/tmp\/report\.pdf"/);
  assert.throws(() => services.llm.fileRequestText(null), /需要 FileAttachmentRef 对象/);
  assert.throws(() => services.llm.fileRequestText({ id: "file-1" }), /缺少 attachmentId\/name\/bytes/);
  assert.throws(() => services.llm.fileRequestText({ attachmentId: "file-1", name: "", bytes: 1 }), /缺少 attachmentId\/name\/bytes/);
  assert.equal(host.calls.length, 0);
});

test("llm.resolveCallConfig 用 host.model.defaults 补齐默认值", async () => {
  const calls = [];
  const host = {
    calls,
    async call(method, input) {
      calls.push({ method, input });
      if (method === "host.model.defaults") {
        return { provider: "openai", model: "gpt-4o", temperature: 0.7, maxOutputTokens: 2048, contextWindow: 128000 };
      }
      return { method, input };
    },
  };
  const services = createAmitiaDshHostServices(host);
  const merged = await services.llm.resolveCallConfig({});
  assert.equal(merged.provider, "openai");
  assert.equal(merged.model, "gpt-4o");
  assert.equal(merged.temperature, 0.7);
  assert.equal(merged.maxOutputTokens, 2048);
  assert.equal(merged.contextWindow, 128000);
  const explicit = await services.llm.resolveCallConfig({ provider: "amitia", temperature: 0.1, maxOutputTokens: 512 });
  assert.equal(explicit.provider, "amitia");
  assert.equal(explicit.temperature, 0.1);
  assert.equal(explicit.maxOutputTokens, 512);
  assert.equal(calls.filter((entry) => entry.method === "host.model.defaults").length, 2);
});

test("llm.registerAdapter 注册真实适配器并按其实现委派", async () => {
  const host = createHost();
  const services = createAmitiaDshHostServices(host);
  const frames = [{ type: "finish", reason: { kind: "stop" } }];
  const adapter = {
    providerInfo: (id) => ({ id, name: "Amitia 模型网关" }),
    listModels: (provider) => [{ id: `${provider}-chat` }],
    resolveModel: (provider, model) => ({ provider, id: model, name: model }),
    prepareCall: (provider, model) => ({ model: { provider, id: model }, stream: () => frames }),
    stream: (options) => options.signal?.aborted ? Promise.reject(options.signal.reason) : frames,
    providerRetryPolicy: () => ({ attempts: 2 }),
    imageRequestPricing: () => undefined,
    imageRequestTarget: (provider, model, image) => ({ width: image.width, height: image.height }),
  };
  const handle = services.llm.registerAdapter(["amitia"], adapter);
  assert.deepEqual(services.llm.listProviders(), [{ id: "amitia", name: "Amitia 模型网关" }]);
  assert.deepEqual(await services.llm.listModels("amitia"), [{ id: "amitia-chat" }]);
  assert.deepEqual(await services.llm.resolveModelInfo("amitia", "deepseek-chat"), {
    provider: "amitia",
    id: "deepseek-chat",
    name: "deepseek-chat",
  });
  const prepared = await services.llm.prepareCall({ provider: "amitia", model: "deepseek-chat" });
  assert.equal(prepared.model.id, "deepseek-chat");
  assert.equal(await services.llm.stream({ provider: "amitia", model: "deepseek-chat" }), frames);
  assert.deepEqual(services.llm.providerRetryPolicy("amitia"), { attempts: 2 });
  assert.equal(services.llm.imageRequestPricing("amitia", "deepseek-chat"), undefined);
  assert.deepEqual(services.llm.imageRequestTarget("amitia", "deepseek-chat", { width: 8, height: 4 }), { width: 8, height: 4 });
  const replacement = { ...adapter, providerInfo: (id) => ({ id, name: "替换后的网关" }) };
  handle.replace(replacement);
  assert.deepEqual(services.llm.listProviders(), [{ id: "amitia", name: "替换后的网关" }]);
  handle();
  assert.deepEqual(services.llm.listProviders(), []);
  assert.throws(() => services.llm.listModels("amitia"), /DSH llm\.listModels 不可用/);
  assert.throws(() => services.llm.registerAdapter([], adapter), /DSH llm\.registerAdapter 需要 provider 列表/);
  assert.throws(() => services.llm.registerAdapter(["amitia"], null), /DSH llm\.registerAdapter 需要 adapter 对象/);
  assert.equal(host.calls.length, 0);
});

test("llm 服务注册容器自带的 amitia 适配器并经 host.model.invoke 取回增量帧", async () => {
  const calls = [];
  const host = {
    async call(method, input) {
      calls.push({ method, input });
      return { text: "ok", finishReason: "stop" };
    },
  };
  const services = createAmitiaDshHostServices(host);
  modelAdapterModule.plugin.apply({ llm: services.llm }, { host });
  assert.deepEqual(services.llm.listProviders(), [{ id: "amitia", name: "Amitia 模型网关" }]);
  const frames = [];
  for await (const frame of services.llm.stream({
    provider: "amitia",
    model: "deepseek-chat",
    messages: [{ role: "user", content: [{ type: "text", text: "hello" }] }],
  })) {
    frames.push(frame);
  }
  assert.deepEqual(frames.map((frame) => frame.type), ["block-start", "text-delta", "block-end", "finish"]);
  assert.equal(frames[1].text, "ok");
  assert.equal(calls.length, 1);
  assert.equal(calls[0].method, "host.model.invoke");
  assert.equal(calls[0].input.provider, "amitia");
  assert.equal(calls[0].input.model, "deepseek-chat");
});

test("llm 可配置 provider 与模型发现器走注册表", async () => {
  const host = createHost();
  const services = createAmitiaDshHostServices(host);
  const entries = [{ id: "amitia", name: "Amitia" }];
  const disposeProviders = services.llm.registerConfigurableProviders(entries);
  assert.deepEqual(services.llm.listConfigurableProviders(), entries);
  disposeProviders();
  assert.deepEqual(services.llm.listConfigurableProviders(), []);
  const disposeDiscovery = services.llm.registerModelDiscovery("amitia", (request, signal) => {
    assert.deepEqual(request, { baseUrl: "https://example.com" });
    assert.equal(signal.aborted, false);
    return [{ id: "amitia-chat" }];
  });
  assert.deepEqual(
    await services.llm.discoverModels("amitia", { baseUrl: "https://example.com" }, new AbortController().signal),
    [{ id: "amitia-chat" }],
  );
  assert.throws(() => services.llm.registerModelDiscovery("amitia", () => []), /DSH llm model discovery amitia 已注册/);
  disposeDiscovery();
  assert.throws(() => services.llm.discoverModels("amitia", {}), /DSH llm\.discoverModels 不可用/);
  assert.equal(host.calls.length, 0);
});

test("sessionQuery.inspectSession 需要读取回调并交还只读会话视图", async () => {
  const host = createHost();
  const services = createAmitiaDshHostServices(host);
  await assert.rejects(services.sessionQuery.inspectSession("session-1"), (error) => {
    assert.equal(error.code, "DSH_SESSION_INSPECT_READ_REQUIRED");
    assert.match(error.message, /需要读取回调/);
    return true;
  });
});

test("sessionQuery.searchSessions 在会话日志上检索并返回 DSH 命中页结构", async () => {
  const host = createHost({
    "host.conversation.list": () => ({ items: [], workspaces: [], hasMore: false }),
  });
  const services = createAmitiaDshHostServices(host);
  const page = await services.sessionQuery.searchSessions({ query: "workspace", limit: 1 });
  assert.deepEqual(page.items, []);
  assert.equal(page.nextCursor, undefined);
  assert.equal(host.calls.some((call) => call.method === "host.conversation.search"), false);
  await assert.rejects(() => services.sessionQuery.searchSessions({}), /需要非空检索词/);
  const aborted = new AbortController();
  aborted.abort(new Error("cancelled"));
  await assert.rejects(() => services.sessionQuery.searchSessions({ query: "x" }, { signal: aborted.signal }), /cancelled/);
});

test("schedule.history 原样抛出宿主错误并单独折叠缺失排程", async () => {
  const cause = new Error("schedule store offline");
  const thrown = Object.assign(new Error("host.schedule.history failed: store offline"), {
    code: "host_unavailable",
    response: { code: 503, msg: "store offline" },
    cause,
  });
  const host = createHost({
    "host.schedule.history": (input) => {
      if (input.scheduleId === "missing") {
        throw Object.assign(new Error("host.schedule.history failed: resource_not_found"), { code: "resource_not_found" });
      }
      throw thrown;
    },
  });
  const services = createAmitiaDshHostServices(host);
  await assert.rejects(() => services.schedule.history({ scheduleId: "sched-1" }), (error) => {
    assert.equal(error, thrown);
    assert.equal(error.code, "host_unavailable");
    assert.deepEqual(error.response, { code: 503, msg: "store offline" });
    assert.equal(error.cause, cause);
    return true;
  });
  assert.deepEqual(await services.schedule.history({ scheduleId: "missing" }), { id: "missing", code: "schedule_not_found" });
  assert.equal(host.calls.length, 2);
});

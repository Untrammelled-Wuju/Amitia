import test from "node:test";
import assert from "node:assert/strict";
import { runModelDiagnose } from "../src/runtime/diagnose.mjs";

function createLlm(chunks) {
  return {
    stream(options) {
      const captured = { options };
      const iterator = (async function* () {
        for (const chunk of chunks) yield chunk;
      })();
      iterator.captured = captured;
      return iterator;
    },
  };
}

function sessionMessages(events) {
  return events
    .filter((event) => event.type === "assistant/message" || event.type === "user/message")
    .map((event) => event.data.message);
}

function createAgents(events, options = {}) {
  const captured = [];
  const state = { disposed: false, followups: [], registeredTools: [] };
  return {
    captured,
    state,
    async create(input) {
      captured.push(input);
      if (typeof input?.setup === "function") {
        await input.setup({
          tools: {
            register(definition) {
              state.registeredTools.push(definition);
              return () => {};
            },
          },
          get(name) {
            if (name !== "approval") return undefined;
            return {
              async request() {
                return "allowed-once";
              },
            };
          },
        }, { id: "agent-1" });
      }
      return {
        agent: {
          followup(message) {
            state.followups.push(message);
          },
          async whenIdle() {},
          session: {
            snapshotEvents: () => events.slice(),
            deriveMessages: () => sessionMessages(events),
          },
        },
        async dispose() {
          state.disposed = true;
        },
      };
    },
  };
}

function turnEvents(text) {
  return [
    { type: "turn/start", seq: 0, data: {} },
    { type: "user/message", seq: 1, data: { message: { id: "u1", content: [{ type: "text", text: "自检" }] } } },
    { type: "assistant/message", seq: 2, data: { message: { role: "assistant", content: [{ type: "text", text }] } } },
    { type: "turn/end", seq: 3, data: {} },
  ];
}

function baseOptions(overrides = {}) {
  return {
    llm: createLlm([
      { type: "block-start", index: 0 },
      { type: "text-delta", index: 0, text: "自检通过" },
      { type: "block-end", index: 0 },
      { type: "finish", reason: "stop" },
    ]),
    agents: createAgents(turnEvents("自检通过")),
    spillStore: {
      async saveText(input) {
        return { locator: `spill://${input.owner.sessionId}/${input.suggestedName}`, bytes: input.content.length };
      },
    },
    model: "deepseek-chat",
    prompt: "请回答",
    sessionId: "dsh-diagnose-test",
    cwd: "/tmp/amitia-dsh",
    host: {
      async call() { return { approved: true }; },
      async callTargeted() { return { approved: true }; },
    },
    ...overrides,
  };
}

test("完整链路产出 llm 探针、官方 turn 事件与 spill", async () => {
  const options = baseOptions();
  const report = await runModelDiagnose(options);
  assert.equal(report.provider, "amitia");
  assert.equal(report.model, "deepseek-chat");
  assert.equal(report.llm.text, "自检通过");
  assert.equal(report.llm.finishReason, "stop");
  assert.equal(report.turn.turnStart, 1);
  assert.equal(report.turn.turnEnd, 1);
  assert.equal(report.turn.assistantMessages, 1);
  assert.equal(report.turn.assistantText, "自检通过");
  assert.equal(report.turn.sessionId, "dsh-diagnose-test");
  assert.equal(report.spill.source, "turn");
  assert.equal(report.spill.bytes, "自检通过".length);
  assert.equal(report.disposed, false);
  assert.deepEqual(options.agents.captured[0].meta, { cwd: "/tmp/amitia-dsh" });
  assert.equal(options.agents.captured[0].agentOptions.provider, "amitia");
  assert.equal(options.agents.captured[0].agentOptions.model, "deepseek-chat");
  assert.equal(options.agents.state.followups.length, 1);
  assert.equal(options.agents.state.followups[0].content[0].text, "请回答");
});

test("toolProbe 在官方 setup 中注册探针工具并统计工具事件", async () => {
  const events = [
    ...turnEvents("已调用工具").filter((event) => event.type !== "turn/end"),
    { type: "tool/call", seq: 3, data: { turn: 1, step: 1, callId: "call-1", name: "amitia_probe_dsh_diagnose_test", arguments: "{}" } },
    { type: "tool/result", seq: 4, data: { turn: 1, step: 1, callId: "call-1", content: [{ type: "text", text: "{\"ok\":true}" }] } },
    { type: "turn/end", seq: 5, data: {} },
  ];
  const options = baseOptions({ agents: createAgents(events), toolProbe: true });
  const report = await runModelDiagnose(options);
  const registered = options.agents.state.registeredTools;
  assert.equal(registered.length, 1);
  assert.equal(registered[0].name, "amitia_probe_dsh_diagnose_test");
  assert.equal(report.turn.probeTool, "amitia_probe_dsh_diagnose_test");
  assert.equal(report.turn.toolCalls, 1);
  assert.equal(report.turn.toolResults, 1);
  assert.equal(report.turn.turnEnd, 1);
  const value = await registered[0].execute({});
  assert.deepEqual(value, { ok: true });
  assert.deepEqual(registered[0].output.render({}, value), [{ type: "text", text: "{\"ok\":true}" }]);
});

test("未开启 toolProbe 时不注册探针工具", async () => {
  const options = baseOptions();
  const report = await runModelDiagnose(options);
  assert.equal(options.agents.state.registeredTools.length, 0);
  assert.equal(report.turn.probeTool, "");
  assert.equal(report.turn.toolCalls, 0);
});

test("dispose 为 true 时释放官方 agent", async () => {
  const options = baseOptions({ dispose: true });
  const report = await runModelDiagnose(options);
  assert.equal(report.disposed, true);
  assert.equal(options.agents.state.disposed, true);
});

test("缺少 model 时失败", async () => {
  await assert.rejects(runModelDiagnose(baseOptions({ model: "" })), /需要 model/);
});

test("缺少 prompt 时失败", async () => {
  await assert.rejects(runModelDiagnose(baseOptions({ prompt: "" })), /需要 prompt/);
});

test("缺少 sessionId 时失败", async () => {
  await assert.rejects(runModelDiagnose(baseOptions({ sessionId: "" })), /需要 sessionId/);
});

test("cwd 不是绝对路径时失败", async () => {
  await assert.rejects(runModelDiagnose(baseOptions({ cwd: "relative/path" })), /绝对路径 cwd/);
});

test("llm 服务缺少 stream 时失败", async () => {
  await assert.rejects(runModelDiagnose(baseOptions({ llm: {} })), /未提供 stream/);
});

test("llm 未产出文本时失败", async () => {
  await assert.rejects(
    runModelDiagnose(baseOptions({ llm: createLlm([{ type: "finish", reason: "stop" }]) })),
    /未产出文本/,
  );
});

test("chunk 缺少 type 时失败", async () => {
  await assert.rejects(
    runModelDiagnose(baseOptions({ llm: createLlm([{ text: "x" }]) })),
    /缺少 type 的 chunk/,
  );
});

test("官方 agents 缺失时失败", async () => {
  await assert.rejects(runModelDiagnose(baseOptions({ agents: null })), /官方.*未就绪/);
});

test("turn 未产出 turn/end 时失败并透出事件统计", async () => {
  const agents = createAgents(turnEvents("自检通过").filter((event) => event.type !== "turn/end"));
  await assert.rejects(runModelDiagnose(baseOptions({ agents })), /未完成真实模型回合/);
});

test("官方 session 未记录事件时失败", async () => {
  await assert.rejects(runModelDiagnose(baseOptions({ agents: createAgents([]) })), /未记录任何事件/);
});

test("approvalProbe 通过容器注入安装审批监听器", async () => {
  const events = [
    ...turnEvents("已审批").filter((event) => event.type !== "turn/end"),
    { type: "approval/asked", seq: 3, data: {} },
    { type: "approval/decided", seq: 4, data: { outcome: "allowed-once" } },
    { type: "turn/end", seq: 5, data: {} },
  ];
  const installed = [];
  const options = baseOptions({
    agents: createAgents(events),
    approvalProbe: true,
    approvalScopeSnapshotId: "conversation-target-e2e",
    approvalConversationId: "conversation-e2e",
    installApprovalAnswerer: (answerer) => {
      installed.push(answerer);
      return () => {};
    },
  });
  const report = await runModelDiagnose(options);
  assert.equal(installed.length, 1);
  assert.equal(typeof installed[0], "function");
  assert.equal(report.turn.approvalDecided, 1);
  assert.equal(report.turn.approvalConversationId, "conversation-e2e");
});

test("approvalProbe 缺少容器注入时失败", async () => {
  await assert.rejects(
    runModelDiagnose(baseOptions({ approvalProbe: true })),
    /需要 installApprovalAnswerer/,
  );
  await assert.rejects(
    runModelDiagnose(baseOptions({
      approvalProbe: true,
      approvalScopeSnapshotId: "conversation-target-e2e",
      installApprovalAnswerer: () => () => {},
    })),
    /approvalDecided|approval 探针|未产生审批记录/,
  );
});

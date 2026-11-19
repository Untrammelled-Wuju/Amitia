import assert from "node:assert/strict";
import test from "node:test";
import { createAmitiaDshHostServices } from "../src/runtime/host-services.mjs";

function createSubagentHost(options = {}) {
  const calls = [];
  const targeted = [];
  return {
    calls,
    targeted,
    async call(method, input = {}) {
      calls.push({ method, input });
      if (method === "host.state.get") {
        return options.mintScope ? { found: false } : { found: true, value: { scopeSnapshotId: "snapshot-child" } };
      }
      if (method === "host.conversation.scope") return { conversationId: input.conversationId, scopeSnapshotId: "snapshot-minted" };
      return {};
    },
    async callTargeted(method, input, scopeSnapshotId) {
      targeted.push({ method, input, scopeSnapshotId });
      return { messageId: "message-accepted" };
    },
  };
}

test("subagents.prompt 把文本片段按会话作用域追加为子代理用户消息", async () => {
  const host = createSubagentHost();
  const services = createAmitiaDshHostServices(host);
  const receipt = await services.subagents.prompt({
    requestId: "prompt-1",
    childSessionId: "session-child",
    delivery: "queue",
    content: [{ type: "text", text: "  继续处理  " }, { type: "text", text: "第二步" }],
  });
  assert.deepEqual(receipt, { messageId: "message-accepted" });
  assert.equal(host.targeted.length, 1);
  assert.equal(host.targeted[0].method, "host.conversation.message.append");
  assert.equal(host.targeted[0].scopeSnapshotId, "snapshot-child");
  assert.deepEqual(host.targeted[0].input, {
    conversationId: "session-child",
    role: "user",
    source: "dsh-subagent",
    requestId: "prompt-1",
    parts: [{ type: "text", content: "继续处理" }, { type: "text", content: "第二步" }],
  });
});

test("subagents.prompt 的 steer 与 queue 走同一条追加通道", async () => {
  const host = createSubagentHost();
  const services = createAmitiaDshHostServices(host);
  await services.subagents.prompt({ requestId: "prompt-steer", childSessionId: "session-child", delivery: "steer", content: [{ type: "text", text: "插话" }] });
  assert.equal(host.targeted[0].input.requestId, "prompt-steer");
  assert.equal(host.targeted[0].method, "host.conversation.message.append");
});

test("subagents.prompt 在缺少作用域快照时用 conversation.scope 铸造", async () => {
  const host = createSubagentHost({ mintScope: true });
  const services = createAmitiaDshHostServices(host);
  await services.subagents.prompt({ requestId: "prompt-2", childSessionId: "session-child", delivery: "queue", content: [{ type: "text", text: "hi" }] });
  assert.equal(host.targeted[0].scopeSnapshotId, "snapshot-minted");
  assert.ok(host.calls.some((call) => call.method === "host.conversation.scope"));
});

test("subagents.prompt 拒绝非文本片段与非法入参", async () => {
  const host = createSubagentHost();
  const services = createAmitiaDshHostServices(host);
  await assert.rejects(services.subagents.prompt({}), /需要 childSessionId/);
  await assert.rejects(services.subagents.prompt({ childSessionId: "session-child" }), /需要 requestId/);
  await assert.rejects(services.subagents.prompt({ childSessionId: "session-child", requestId: "p" }), /至少一个内容片段/);
  await assert.rejects(
    services.subagents.prompt({ childSessionId: "session-child", requestId: "p", delivery: "broadcast", content: [{ type: "text", text: "x" }] }),
    /delivery 只支持 queue\/steer/,
  );
  await assert.rejects(
    services.subagents.prompt({ childSessionId: "session-child", requestId: "p", content: [{ type: "image", mediaType: "image/png", data: "AAAA" }] }),
    /只接受文本片段/,
  );
  await assert.rejects(
    services.subagents.prompt({ childSessionId: "session-child", requestId: "p", content: [{ type: "text", text: "   " }] }),
    /文本片段全为空/,
  );
  assert.equal(host.targeted.length, 0);
});

test("subagents.prompt 在没有 callTargeted 时明确报错", async () => {
  const host = createSubagentHost();
  delete host.callTargeted;
  const services = createAmitiaDshHostServices(host);
  await assert.rejects(
    services.subagents.prompt({ childSessionId: "session-child", requestId: "p", content: [{ type: "text", text: "hi" }] }),
    /host\.callTargeted 不可用/,
  );
});

test("subagents 的树工作状态反映活动 run，drain 以 dispose 收敛且不吞错", async () => {
  const host = createSubagentHost();
  const services = createAmitiaDshHostServices(host);
  assert.equal(await services.subagents.waitForTreeWork(), false);
  let disposed = 0;
  const disposeProvider = services.subagents.registerProvider({
    name: "local",
    async start() {
      return { id: "child-1", messageId: "message-1", result: {}, dispose: async () => { disposed += 1; } };
    },
  });
  await services.subagents.startActivation({ provider: "local", name: "worker" });
  assert.equal(await services.subagents.waitForTreeWork(), true);
  await services.subagents.drainDescendants();
  assert.equal(disposed, 1);
  assert.equal(await services.subagents.waitForTreeWork(), false);
  const failingProvider = services.subagents.registerProvider({
    name: "failing",
    async start() {
      return { id: "child-2", messageId: "message-2", result: {}, dispose: async () => { throw new Error("dispose failed"); } };
    },
  });
  await services.subagents.startActivation({ provider: "failing", name: "bad" });
  await assert.rejects(services.subagents.drainDescendants(), /dispose failed/);
  await assert.rejects(services.subagents.sendMessage({}, "target", [{ type: "text", text: "hi" }]), /无法从 sender 解析会话 id/);
  await assert.rejects(services.subagents.sendMessage({ session: { id: "root" } }, "target", []), /需要至少一个内容片段/);
  disposeProvider();
  failingProvider();
});

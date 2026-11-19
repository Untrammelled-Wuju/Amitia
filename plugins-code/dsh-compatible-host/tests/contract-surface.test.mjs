import assert from "node:assert/strict";
import test from "node:test";
import { createAmitiaDshHostServices } from "../src/runtime/host-services.mjs";

function createContractHost() {
  return {
    async call(method, input = {}) {
      if (method === "host.conversation.list") return { items: [], workspaces: [], hasMore: false };
      if (method === "host.state.get") return { found: false };
      if (method === "host.state.cas") return { swapped: true };
      if (method === "host.conversation.read") return { messages: [], hasMore: false };
      if (method === "host.conversation.turns") return { turns: [] };
      if (method === "host.conversation.scope") return { scopeSnapshotId: "scope-" + String(input.conversationId || "") };
      return {};
    },
    async callTargeted() {
      return { messageId: "message-1" };
    },
  };
}

test("剩余 2 个未实现的宿主方法保持原始缺口报错", () => {
  const services = createAmitiaDshHostServices(createContractHost());
  const seams = [
    ["sessions", "enter"],
    ["sessions", "announce"],
  ];
  for (const [serviceName, method] of seams) {
    const service = services[serviceName];
    assert.equal(typeof service[method], "function", `${serviceName}.${method} 应存在`);
    assert.throws(() => service[method]({}), (error) => {
      assert.equal(error.code, "DSH_HOST_OPERATION_UNAVAILABLE", `${serviceName}.${method} 必须报缺口而不是伪造成功`);
      assert.equal(typeof error.missingHostInterface, "string");
      assert.equal(error.details.operation, `${serviceName}.${method}`);
      return true;
    }, `${serviceName}.${method}`);
  }
});

test("已实现的关键方法不是缺口占位", async () => {
  const services = createAmitiaDshHostServices(createContractHost());
  const probes = [
    () => services.sessionQuery.searchSessions({ query: "x" }),
    () => services.sessionQuery.searchEvents({ sessionId: "s1", query: "x" }),
    () => services.sessionQuery.traceEvent({ sessionId: "s1", seq: 0 }),
    () => services.workspaceFiles.changes({ sessionId: "s1", path: "/w" }).next(),
    () => services.directoryPickerController.capability(),
    () => services.subagents.sendMessage({ session: { id: "root" } }, "child", [{ type: "text", text: "hi" }], {}),
  ];
  for (const probe of probes) {
    try {
      await probe();
    } catch (error) {
      assert.notEqual(error?.code, "DSH_HOST_OPERATION_UNAVAILABLE", `已实现方法不应报缺口：${String(error?.message ?? error)}`);
    }
  }
});

test("消息投影注册与 inspectSession 已接入宿主消息表面", async () => {
  const services = createAmitiaDshHostServices(createContractHost());
  const dispose = services.sessions.registerMessageProjection({
    type: "assistant/message",
    project: () => new Map(),
  });
  assert.equal(typeof dispose, "function");
  assert.throws(() => services.sessions.registerMessageProjection({ type: "assistant/message", project: () => new Map() }), /已经注册/);
  assert.throws(() => services.sessions.registerMessageProjection({ type: "user/message" }), /需要 \{ type, project/);
  dispose();
  dispose();
  await assert.rejects(services.sessionQuery.inspectSession("session-1"), (error) => {
    assert.equal(error.code, "DSH_SESSION_INSPECT_READ_REQUIRED");
    return true;
  });
});

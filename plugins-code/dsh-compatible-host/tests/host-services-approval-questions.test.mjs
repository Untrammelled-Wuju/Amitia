import assert from "node:assert/strict";
import test from "node:test";
import { createAmitiaDshHostServices } from "../src/runtime/host-services.mjs";

function createHost(handlers = {}) {
  const calls = [];
  const host = {
    calls,
    async call(method, input) {
      calls.push({ method, input });
      const handler = handlers[method];
      if (handler) return handler(input);
      return { method, input };
    },
  };
  return host;
}

test("approval.request 把审批结果映射为 allowed-once / rejected", async () => {
  const responses = [{ approved: true }, { approved: false }];
  const host = createHost({
    "host.approval.request": () => responses.shift(),
  });
  const services = createAmitiaDshHostServices(host);
  assert.equal(await services.approval.request({ toolName: "shell", callId: "call-1", riskLevel: "high" }), "allowed-once");
  assert.equal(await services.approval.request({ toolName: "shell", callId: "call-2" }), "rejected");
  assert.deepEqual(
    host.calls.filter((entry) => entry.method === "host.approval.request").map((entry) => entry.input.toolName),
    ["shell", "shell"],
  );
});

test("approval.request 对非法宿主结果抛原始错误", async () => {
  const host = createHost({ "host.approval.request": () => ({ ok: true }) });
  const services = createAmitiaDshHostServices(host);
  await assert.rejects(
    () => services.approval.request({ toolName: "shell" }),
    /Amitia host\.approval\.request 返回了非法结果/,
  );
});

test("userQuestions.ask 通过 host.ui.dialog 返回选中项", async () => {
  const host = createHost({
    "host.ui.dialog": () => ({ result: "允许一次", dialogId: "dialog-9" }),
  });
  const services = createAmitiaDshHostServices(host);
  const answer = await services.userQuestions.ask({
    message: "是否继续？",
    options: ["允许一次", "拒绝"],
  });
  assert.equal(answer.selected, "允许一次");
  assert.equal(answer.dialogId, "dialog-9");
  assert.equal(answer.question, "是否继续？");
  const dialog = host.calls.find((entry) => entry.method === "host.ui.dialog");
  assert.equal(dialog.input.message, "是否继续？");
});

test("userQuestions.ask 对空结果抛原始错误", async () => {
  const host = createHost({ "host.ui.dialog": () => ({ dialogId: "dialog-1" }) });
  const services = createAmitiaDshHostServices(host);
  await assert.rejects(
    () => services.userQuestions.ask({ message: "继续？", options: ["是"] }),
    /Amitia host\.ui\.dialog 返回了非法结果/,
  );
});

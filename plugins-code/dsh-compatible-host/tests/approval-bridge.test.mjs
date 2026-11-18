import assert from "node:assert/strict";
import test from "node:test";
import {
  approvalPromptText,
  createAmitiaApprovalAnswerer,
  installApprovalAnswerer,
} from "../src/runtime/approval-bridge.mjs";

function createHost(outcome, error) {
  const calls = [];
  return {
    calls,
    async call(method, input) {
      calls.push({ method, input });
      if (error) throw error;
      return outcome;
    },
  };
}

test("批准结果映射为 allowed-once 并透出工具与原因", async () => {
  const host = createHost({ approved: true });
  const answer = createAmitiaApprovalAnswerer({ host });
  const outcome = await answer({ id: "approval-1", toolName: "amitia_probe", callId: "call-1", reason: "写入文件" });
  assert.equal(outcome, "allowed-once");
  assert.equal(host.calls.length, 1);
  assert.equal(host.calls[0].method, "host.approval.request");
  assert.equal(host.calls[0].input.toolName, "amitia_probe");
  assert.equal(host.calls[0].input.toolCallId, "call-1");
  assert.equal(host.calls[0].input.requestId, "approval-1");
  assert.equal(host.calls[0].input.arguments, "插件请求执行工具：amitia_probe\n原因：写入文件");
});

test("拒绝结果映射为 rejected", async () => {
  const host = createHost({ approved: false });
  const answer = createAmitiaApprovalAnswerer({ host });
  assert.equal(await answer({ toolName: "amitia_probe" }), "rejected");
});

test("缺少工具名时使用占位名", async () => {
  const host = createHost({ approved: true });
  const answer = createAmitiaApprovalAnswerer({ host });
  await answer({});
  assert.equal(host.calls[0].input.toolName, "unknown-tool");
});

test("宿主返回非法结果时原样抛错", async () => {
  const host = createHost({ approved: "yes" });
  const answer = createAmitiaApprovalAnswerer({ host });
  await assert.rejects(answer({ toolName: "amitia_probe" }), /返回了非法结果.*"approved":"yes"/);
});

test("宿主错误原样抛出", async () => {
  const original = new Error("invoke error: jsonrpc: -32603: host.approval.request: host_unavailable: approval broker not configured");
  const host = createHost(null, original);
  const answer = createAmitiaApprovalAnswerer({ host });
  await assert.rejects(answer({ toolName: "amitia_probe" }), (error) => error === original);
});

test("缺少 host.call 时拒绝创建应答器", () => {
  assert.throws(() => createAmitiaApprovalAnswerer({ host: {} }), /需要 Amitia 宿主 host.call/);
});

test("approvalPromptText 只包含工具与原因", () => {
  assert.equal(approvalPromptText({}), "插件请求执行受限操作");
  assert.equal(approvalPromptText({ toolName: "shell" }), "插件请求执行工具：shell");
});

test("installApprovalAnswerer 注册 approval/request 监听", () => {
  const listeners = [];
  const context = {
    on(event, handler) {
      listeners.push({ event, handler });
      return () => {};
    },
  };
  const answerer = async () => "rejected";
  installApprovalAnswerer(context, answerer);
  assert.equal(listeners.length, 1);
  assert.equal(listeners[0].event, "approval/request");
  assert.equal(listeners[0].handler, answerer);
  assert.throws(() => installApprovalAnswerer({}, answerer), /需要 cordis context.on/);
  assert.throws(() => installApprovalAnswerer(context, null), /必须是函数/);
});

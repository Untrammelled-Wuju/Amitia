import assert from "node:assert/strict";
import test from "node:test";
import { createDshHostContext } from "../src/runtime/host-abi.mjs";
import * as modelAdapterModule from "../src/runtime/model-adapter.mjs";

function createHost(result, error) {
  const calls = [];
  return {
    calls,
    async call(method, input) {
      calls.push({ method, input });
      if (error) throw error;
      return typeof result === "function" ? result(input) : result;
    },
  };
}

async function collect(iterable) {
  const chunks = [];
  for await (const chunk of iterable) chunks.push(chunk);
  return chunks;
}

function baseOptions(overrides = {}) {
  return {
    provider: "amitia",
    model: "deepseek-chat",
    messages: [{ role: "user", content: [{ type: "text", text: "你好" }] }],
    ...overrides,
  };
}

test("adapter 把 GenerateOptions 译为 host.model.invoke 并产出官方 chunk", async () => {
  const host = createHost({
    provider: "amitia",
    model: "deepseek-chat",
    protocol: "openai-chat",
    text: "回复",
    reasoning: "思考",
    refusal: "",
    toolCalls: [{ id: "call_1", name: "read", arguments: "{\"path\":\"a.txt\"}" }],
    finishReason: "tool_calls",
    usage: { reported: true, inputTokens: 100, outputTokens: 20, cachedInputTokens: 40, cacheUsageReported: true, totalTokens: 120, reasoningTokens: 5 },
    providerResponseID: "resp_1",
  });
  const adapter = modelAdapterModule.createAmitiaLlmAdapter({ host });
  const chunks = await collect(adapter.stream(baseOptions({
    system: "系统提示",
    tools: [{ name: "read", description: "读文件", parameters: { type: "object", properties: { path: { type: "string" } } } }],
    temperature: 0.7,
    maxTokens: 256,
    reasoningEffort: "high",
    sessionId: "session-1",
    messages: [
      { role: "system", content: [{ type: "text", text: "系统消息" }] },
      { role: "user", content: [{ type: "text", text: "你好" }] },
      {
        role: "assistant",
        content: [
          { type: "reasoning", text: "历史思考" },
          { type: "text", text: "我来读文件" },
          { type: "tool-call", id: "call_0", name: "read", arguments: "{\"path\":\"b.txt\"}" },
        ],
      },
      { role: "tool", content: [{ type: "text", text: "内容" }], toolCallId: "call_0" },
    ],
  })));

  assert.equal(host.calls.length, 1);
  assert.equal(host.calls[0].method, "host.model.invoke");
  const payload = host.calls[0].input;
  assert.equal(payload.provider, "amitia");
  assert.equal(payload.model, "deepseek-chat");
  assert.equal(payload.system, "系统提示");
  assert.equal(payload.temperature, 0.7);
  assert.equal(payload.maxOutputTokens, 256);
  assert.equal(payload.reasoningEffort, "high");
  assert.equal(payload.sessionId, "session-1");
  assert.equal(payload.messages.length, 4);
  assert.deepEqual(payload.messages[0], { role: "system", content: "系统消息" });
  assert.deepEqual(payload.messages[1], { role: "user", content: "你好" });
  assert.deepEqual(payload.messages[2], {
    role: "assistant",
    content: "我来读文件",
    toolCalls: [{ id: "call_0", name: "read", arguments: "{\"path\":\"b.txt\"}" }],
  });
  assert.deepEqual(payload.messages[3], { role: "tool", content: "内容", toolCallId: "call_0" });
  assert.deepEqual(payload.tools, [{ name: "read", description: "读文件", parameters: { type: "object", properties: { path: { type: "string" } } } }]);

  assert.deepEqual(chunks[0], { type: "block-start", index: 0, blockType: "reasoning" });
  assert.deepEqual(chunks[1], { type: "reasoning-delta", index: 0, text: "思考" });
  assert.deepEqual(chunks[2], { type: "block-end", index: 0, block: { type: "reasoning", text: "思考" } });
  assert.deepEqual(chunks[3], { type: "block-start", index: 1, blockType: "text" });
  assert.deepEqual(chunks[4], { type: "text-delta", index: 1, text: "回复" });
  assert.deepEqual(chunks[5], { type: "block-end", index: 1, block: { type: "text", text: "回复" } });
  assert.deepEqual(chunks[6], { type: "block-start", index: 2, blockType: "tool-call" });
  assert.deepEqual(chunks[7], { type: "tool-call-delta", index: 2, id: "call_1", name: "read", argumentsDelta: "{\"path\":\"a.txt\"}" });
  assert.deepEqual(chunks[8], { type: "block-end", index: 2, block: { type: "tool-call", id: "call_1", name: "read", arguments: "{\"path\":\"a.txt\"}" } });
  assert.deepEqual(chunks[9], { type: "usage", usage: { inputTokens: 60, outputTokens: 20, cacheReadTokens: 40, totalTokens: 120, reasoningTokens: 5 } });
  assert.deepEqual(chunks[10], { type: "finish", reason: { kind: "tool-calls" } });
  assert.equal(chunks.length, 11);
});

test("adapter 把 length 映射为 max-tokens 且无 usage 时不发 usage chunk", async () => {
  const host = createHost({ text: "截断", finishReason: "length", usage: { reported: false } });
  const adapter = modelAdapterModule.createAmitiaLlmAdapter({ host });
  const chunks = await collect(adapter.stream(baseOptions()));
  assert.equal(chunks.filter((chunk) => chunk.type === "usage").length, 0);
  assert.deepEqual(chunks.at(-1), { type: "finish", reason: { kind: "max-tokens" } });
});

test("adapter 将 refusal 作为可见文本产出", async () => {
  const host = createHost({ text: "", refusal: "无法处理该请求", finishReason: "stop" });
  const adapter = modelAdapterModule.createAmitiaLlmAdapter({ host });
  const chunks = await collect(adapter.stream(baseOptions()));
  assert.deepEqual(chunks[2], { type: "block-end", index: 0, block: { type: "text", text: "无法处理该请求" } });
});

test("adapter 原样抛出宿主错误", async () => {
  const original = new Error("host.call host.model.invoke: internal: API 返回 401: invalid api key");
  original.stack = "Error: host.call host.model.invoke: internal: API 返回 401: invalid api key\n    at host";
  const adapter = modelAdapterModule.createAmitiaLlmAdapter({ host: createHost(null, original) });
  await assert.rejects(
    () => collect(adapter.stream(baseOptions())),
    (error) => error === original,
  );
});

test("adapter 拒绝 stop 序列而不静默丢弃", async () => {
  const host = createHost({ text: "x", finishReason: "stop" });
  const adapter = modelAdapterModule.createAmitiaLlmAdapter({ host });
  await assert.rejects(
    () => collect(adapter.stream(baseOptions({ stop: ["END"] }))),
    /Amitia 模型网关不支持 stop 序列/,
  );
  assert.equal(host.calls.length, 0);
});

test("adapter 拒绝缺少 toolCallId 的 tool 消息", async () => {
  const host = createHost({ text: "x", finishReason: "stop" });
  const adapter = modelAdapterModule.createAmitiaLlmAdapter({ host });
  await assert.rejects(
    () => collect(adapter.stream(baseOptions({ messages: [{ role: "tool", content: [{ type: "text", text: "内容" }] }] }))),
    /tool 消息缺少 toolCallId/,
  );
});

test("adapter 拒绝图片内容块而不静默降级", async () => {
  const host = createHost({ text: "x", finishReason: "stop" });
  const adapter = modelAdapterModule.createAmitiaLlmAdapter({ host });
  await assert.rejects(
    () => collect(adapter.stream(baseOptions({ messages: [{ role: "user", content: [{ type: "image", attachment: { id: "image-1" } }] }] }))),
    /"image"/,
  );
});

test("容器注册 amitia provider 路由", async () => {
  const registrations = [];
  const container = createDshHostContext({ pluginId: "dsh-compatible-host", services: {} });
  container.context.provide("llm", {
    registerAdapter(providers, adapter) {
      registrations.push({ providers, adapter });
      return Object.assign(() => {}, { replace() {} });
    },
  });
  const plugin = modelAdapterModule.plugin;
  const host = createHost({ text: "ok", finishReason: "stop" });
  const fiber = container.context.plugin(plugin, { host });
  await fiber;
  assert.equal(registrations.length, 1);
  assert.deepEqual(registrations[0].providers, ["amitia"]);
  const chunks = await collect(registrations[0].adapter.stream(baseOptions()));
  assert.deepEqual(chunks.at(-1), { type: "finish", reason: { kind: "stop" } });
  await container.dispose();
});

test("resolveModel 只在拿到真实上下文窗口后才声明 context", async () => {
  const host = createHost({
    provider: "amitia",
    model: "deepseek-chat",
    protocol: "openai-chat",
    contextWindow: 128000,
    text: "回复",
    finishReason: "stop",
    usage: { reported: true, inputTokens: 1, outputTokens: 1, totalTokens: 2 },
  });
  const adapter = modelAdapterModule.createAmitiaLlmAdapter({ host });
  const first = await adapter.resolveModel("amitia", "deepseek-chat");
  assert.equal(first.context, undefined);
  await collect(adapter.stream(baseOptions()));
  const second = await adapter.resolveModel("amitia", "deepseek-chat");
  assert.deepEqual(second.context, { contextWindow: 128000 });
  const prepared = await adapter.prepareCall("amitia", "deepseek-chat");
  assert.deepEqual(prepared.model.context, { contextWindow: 128000 });
});

test("host.model.invoke 未报告上下文窗口时不声明 context", async () => {
  const host = createHost({
    provider: "amitia",
    model: "deepseek-chat",
    protocol: "openai-chat",
    text: "回复",
    finishReason: "stop",
  });
  const adapter = modelAdapterModule.createAmitiaLlmAdapter({ host });
  await collect(adapter.stream(baseOptions()));
  const resolved = await adapter.resolveModel("amitia", "deepseek-chat");
  assert.equal(resolved.context, undefined);
});

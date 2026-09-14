import assert from "node:assert/strict";

let extension = null;
globalThis.defineExtension = (value) => {
  extension = value;
};
await import(`../src/index.mjs?runtime-test=${Date.now()}`);
assert.ok(extension);

const state = { value: null, version: 0 };
let toolHandler = null;

const host = {
  async call(method, input) {
    if (method === "host.state.get") {
      return state.version > 0
        ? { found: true, value: structuredClone(state.value), version: state.version }
        : { found: false, version: 0 };
    }
    if (method === "host.state.cas") {
      if (input.expectedVersion !== state.version) return { swapped: false };
      state.value = structuredClone(input.newValue);
      state.version += 1;
      return { swapped: true };
    }
    if (method === "host.conversation.message.send") {
      if (String(input.content || "").includes("empty")) {
        return { content: "", requestId: input.requestId };
      }
      return { content: "主动消息运行时测试回复", requestId: input.requestId };
    }
    throw new Error(`unexpected host call: ${method}`);
  },
};

await extension.activate({
  host,
  log: { info() {}, warn() {} },
  handlers: {
    bindTool(name, handler) {
      assert.equal(name, "command");
      toolHandler = handler;
    },
  },
});

const command = (action, payload = {}) =>
  toolHandler({ action, payload: { ...payload, scope: { userId: "user-1", characterId: "char-1" } } });

const settings = await command("settings.get");
assert.equal(settings.enabled, true);
const updatedSettings = await command("settings.update", { activeLevel: 55, minInterval: 5 });
assert.equal(updatedSettings.activeLevel, 55);

const created = await command("rules.create", {
  name: "运行时测试规则",
  characterId: "char-1",
  conversationId: "conv-1",
  channel: "web",
  promptTemplate: "测试消息",
});
assert.equal(created.id, 1);
assert.equal((await command("rules.list", { characterId: "char-1" })).length, 1);
assert.equal((await command("rules.toggle", { id: created.id })).enabled, false);
assert.equal((await command("rules.toggle", { id: created.id })).enabled, true);
assert.equal((await command("rules.update", { id: created.id, name: "更新后的规则" })).name, "更新后的规则");
assert.equal((await command("rules.test", { id: created.id })).ok, true);
assert.equal((await command("rules.trigger", { id: created.id })).ok, true);
assert.ok((await command("rules.messages", { id: created.id })).length >= 1);
assert.ok((await command("history.list", { page: 1, pageSize: 20 })).total >= 1);
assert.equal((await command("queue.summary")).backpressure, false);

const presets = await command("presets.reset", { characterId: "char-1" });
assert.equal(presets.count, 6);
const status = await command("status");
assert.equal(status.totalRuleCount, 6);
assert.equal(status.enabledRuleCount, 6);

const presetRules = await command("rules.list");
await command("rules.update", { id: presetRules[0].id, promptTemplate: "empty" });
await assert.rejects(
  () => command("rules.trigger", { id: presetRules[0].id }),
  /主动消息被宿主抑制或未生成内容/,
);
assert.equal((await command("rules.delete", { id: presetRules[1].id })).deleted, true);

await extension.deactivate();
console.log("proactive-runtime=ok");

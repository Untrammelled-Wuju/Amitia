import assert from "node:assert/strict";

let extension = null;
globalThis.defineExtension = (value) => {
  extension = value;
};
await import(`../src/runtime/index.mjs?runtime-test=${Date.now()}`);
assert.ok(extension);

const calls = [];
let command = null;
await extension.activate({
  host: {
    async call(method, input) {
      calls.push({ method, input });
      if (method === "host.character.list") return { items: [{ id: "role-a", name: "角色 A" }] };
      if (input.operation === "list") return { items: [{ id: "conversation-a", archivedAt: "2026-11-09" }] };
      if (input.operation === "messages") return { items: [{ id: "message-a", content: "你好" }] };
      if (input.operation === "export") return { content: "{}", filename: "conversation.json", mimeType: "application/json" };
      return { ok: true, operation: input.operation };
    },
  },
  log: { info() {}, warn() {} },
  handlers: { bindTool(name, handler) { if (name === "command") command = handler; } },
});

assert.ok(command);
assert.equal((await command({ action: "characters" }))[0].value, "role-a");
assert.equal((await command({ action: "list", payload: { roleId: "role-a" } })).items[0].id, "conversation-a");
assert.equal((await command({ action: "messages", payload: { roleId: "role-a", conversationId: "conversation-a" } })).items[0].content, "你好");
await command({ action: "summary", payload: { roleId: "role-a", conversationId: "conversation-a" } });
assert.equal(calls.at(-1).method, "host.conversation_logs.manage");
const exported = await command({ action: "export", payload: { roleId: "role-a", conversationId: "conversation-a" } });
assert.equal(exported.download.filename, "conversation.json");
await assert.rejects(command({ action: "unknown" }), /未知归档对话操作/);
await extension.deactivate();
console.log("chat-logs-runtime=ok");

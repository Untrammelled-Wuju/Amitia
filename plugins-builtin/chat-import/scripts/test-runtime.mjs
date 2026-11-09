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
      if (input.operation === "parse") return { detectedFormat: "standard", items: [{ role: "user", content: input.input.rawText }] };
      if (input.operation === "list_batches") return { items: [{ id: "batch-a", title: "导入" }], total: 1 };
      return { operation: input.operation, ok: true };
    },
  },
  log: { info() {}, warn() {} },
  handlers: { bindTool(name, handler) { if (name === "command") command = handler; } },
});

assert.ok(command);
assert.deepEqual(await command({ action: "characters" }), [{
  id: "role-a",
  name: "角色 A",
  label: "角色 A",
  value: "role-a",
}]);
assert.equal((await command({ action: "parse", payload: { rawText: "用户：你好" } })).detectedFormat, "standard");
const fileParsed = await command({
  action: "parse_file",
  payload: {},
  files: [{ name: "chat.txt", dataBase64: Buffer.from("用户：文件内容").toString("base64") }],
});
assert.equal(fileParsed.items[0].content, "用户：文件内容");
await command({ action: "confirm", payload: { characterId: "role-a", items: [{ role: "user", content: "你好" }] } });
assert.equal(calls.at(-1).method, "host.import.execute");
assert.equal(calls.at(-1).input.operation, "confirm");
await command({ action: "navigate", payload: { route: "/logs" } });
assert.equal(calls.at(-1).method, "host.ui.navigate");
await assert.rejects(command({ action: "unknown" }), /未知聊天导入操作/);
await extension.deactivate();
console.log("chat-import-runtime=ok");

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
      if (input.operation === "list") return { backups: [{ name: "backup.amitia-backup" }] };
      if (input.operation === "info") return { dbSize: 100 };
      return { ok: true, operation: input.operation };
    },
  },
  log: { info() {}, warn() {} },
  handlers: { bindTool(name, handler) { if (name === "command") command = handler; } },
});

assert.ok(command);
const overview = await command({ action: "overview" });
assert.equal(overview.info.dbSize, 100);
assert.equal(overview.backups.backups[0].name, "backup.amitia-backup");
assert.equal((await command({ action: "characters" }))[0].value, "role-a");
await command({ action: "cleanup_preview", payload: { roleId: "role-a", olderThanDays: 30 } });
assert.equal(calls.at(-1).method, "host.storage.cleanup");
assert.equal(calls.at(-1).input.operation, "preview");
await command({ action: "create_backup", payload: { passphrase: "12345678" } });
assert.equal(calls.at(-1).method, "host.storage.backup");
await command({ action: "preview_restore", payload: { name: "backup.amitia-backup" } });
assert.equal(calls.at(-1).method, "host.storage.restore");
await assert.rejects(command({ action: "unknown" }), /未知存储操作/);
await extension.deactivate();
console.log("storage-runtime=ok");

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
      if (input.operation) return { ok: true };
      if (method.endsWith(".list")) return { items: [{ id: "task-a", title: "任务" }], total: 1 };
      if (method.endsWith(".status")) return { total: 1, enabled: 1, dueNow: 0, schedulerRunning: true };
      if (method.endsWith(".queue")) return { queued: 0 };
      if (method.endsWith(".capabilities")) return { ready: true, rules: ["none", "daily"] };
      return { id: input.id, ok: true };
    },
  },
  log: { info() {}, warn() {} },
  handlers: { bindTool(name, handler) { if (name === "command") command = handler; } },
});

assert.ok(command);
const overview = await command({ action: "overview", payload: { kind: "user_reminder" } });
assert.equal(overview.list.items[0].id, "task-a");
assert.equal(overview.status.schedulerRunning, true);
assert.equal((await command({ action: "characters" }))[0].value, "role-a");
await command({
  action: "create",
  payload: {
    kind: "proactive_task",
    title: "主动提醒",
    remindAt: "2026-11-10T09:00:00+08:00",
    characterId: "role-a",
  },
});
assert.equal(calls.at(-1).method, "host.proactive_task.create");
await command({ action: "pause", payload: { kind: "user_reminder", id: "task-a", generation: 2 } });
assert.deepEqual(calls.at(-1), { method: "host.reminder.pause", input: { id: "task-a", generation: 2 } });
await assert.rejects(command({ action: "unknown" }), /未知提醒操作/);
await extension.deactivate();
console.log("reminders-runtime=ok");

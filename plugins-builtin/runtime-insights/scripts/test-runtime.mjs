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
      return {
        meta: { generatedAt: "2026-01-01T00:00:00Z", degraded: false },
        summary: { activeInteractions: 2, queuedTasks: 1, reconciliationIssues: 0 },
      };
    },
  },
  log: { info() {}, warn() {} },
  handlers: {
    bindTool(name, handler) {
      if (name === "command") command = handler;
    },
  },
});

assert.ok(command);
const snapshot = await command({ action: "snapshot" });
assert.equal(snapshot.summary.activeInteractions, 2);
assert.equal((await command({ action: "maintenance.status" })).summary.activeInteractions, 2);
assert.equal((await command({ action: "maintenance.overview" })).summary.activeInteractions, 2);
assert.deepEqual(calls.map((call) => call.method), [
  "host.runtime.snapshot",
  "host.runtime.maintenance.status",
  "host.runtime.maintenance.overview",
]);
assert.deepEqual(calls[1], { method: "host.runtime.maintenance.status", input: {} });
await assert.rejects(command({ action: "unknown" }), /未知运行时观察操作/);
await extension.deactivate();
console.log("runtime-insights-runtime=ok");

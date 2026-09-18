import assert from "node:assert/strict";

let extension = null;
globalThis.defineExtension = (value) => {
  extension = value;
};
await import(`../src/runtime/index.mjs?runtime-test=${Date.now()}`);
assert.ok(extension);

const kv = new Map();
let toolHandlers = new Map();

const host = {
  async call(method, input) {
    if (method === "host.state.get") {
      const entry = kv.get(input.key);
      return entry ? { found: true, value: structuredClone(entry.value), version: entry.version } : { found: false, version: 0 };
    }
    if (method === "host.state.cas") {
      const entry = kv.get(input.key);
      const version = entry ? entry.version : 0;
      if (version !== input.expectedVersion) return { swapped: false, version };
      kv.set(input.key, { value: structuredClone(input.newValue), version: version + 1 });
      return { swapped: true, version: version + 1 };
    }
    throw new Error(`unexpected host call: ${method}`);
  },
};

await extension.activate({
  host,
  log: { info() {}, warn() {} },
  handlers: {
    bindTool(name, handler) {
      toolHandlers.set(name, handler);
    },
  },
});

const command = (action, payload = {}) => toolHandlers.get("command")({
  action,
  payload: { ...payload, characterId: "char-1" },
});

await command("work.update", {
  work: { enabled: true, workStartTime: "09:00", workEndTime: "18:00" },
});
await command("tendency.update", {
  tendencies: { activityEnergy: 71 },
});
await command("fixed.create", {
  title: "晨会",
  eventType: "MEETING",
  startTime: "09:00",
  endTime: "10:00",
});

const snapshot = await command("snapshot", { at: "2026-09-14T09:30:00+08:00" });
assert.equal(snapshot.fixedEvents.length, 1);
assert.equal(snapshot.workProfile.enabled, true);
assert.equal(snapshot.lifestyleTendency.activityEnergy, 71);
assert.equal(snapshot.state.currentState, "WORKING");

await command("sleep.update", { sleep: { bedTime: "22:30", wakeTime: "06:30", enabled: true } });
const updated = await command("snapshot", { at: "2026-09-14T09:30:00+08:00" });
assert.equal(updated.sleepSetting.bedTime, "22:30");

await command("fixed.create", { title: "晚间阅读", eventType: "STUDY", startTime: "20:00", endTime: "21:00" });
const created = await command("snapshot");
assert.ok(created.fixedEvents.length >= 2);

const context = await toolHandlers.get("context")({
  characterId: "char-1",
  at: "2026-09-14T23:30:00+08:00",
});
assert.ok(context.realtime.context.length > 0);

await extension.deactivate();
console.log("lifestyle-runtime=ok");

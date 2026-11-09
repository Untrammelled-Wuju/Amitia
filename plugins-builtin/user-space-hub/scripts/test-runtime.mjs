import assert from "node:assert/strict";

let extension = null;
globalThis.defineExtension = (value) => {
  extension = value;
};
await import(`../src/runtime/index.mjs?runtime-test=${Date.now()}`);
assert.ok(extension);

const logs = [];
await extension.activate({
  host: {
    async call() {
      throw new Error("user space hub must not call host APIs");
    },
  },
  log: {
    info(message) {
      logs.push(message);
    },
    warn() {},
  },
  handlers: {
    bindTool() {
      throw new Error("user space hub must not bind tools");
    },
  },
});
assert.deepEqual(logs, ["user space hub plugin activated"]);
await extension.deactivate();
console.log("user-space-hub-runtime=ok");

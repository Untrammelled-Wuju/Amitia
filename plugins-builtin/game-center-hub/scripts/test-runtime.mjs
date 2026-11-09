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
      throw new Error("game center hub must not call host APIs");
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
      throw new Error("game center hub must not bind tools");
    },
  },
});
assert.deepEqual(logs, ["game center hub plugin activated"]);
await extension.deactivate();
console.log("game-center-hub-runtime=ok");

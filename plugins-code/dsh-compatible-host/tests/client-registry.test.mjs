import assert from "node:assert/strict";
import test from "node:test";
import { acquireDshClient, dshClientHandleCount, invalidateDshClient } from "../src/client-ui/client-registry.mjs";

function reset() {
  delete globalThis.__amitiaDshClientRegistry;
}

function deferred() {
  let resolve;
  let reject;
  const promise = new Promise((resolveValue, rejectValue) => {
    resolve = resolveValue;
    reject = rejectValue;
  });
  return { promise, resolve, reject };
}

test("concurrent acquires share a single instantiation", async () => {
  reset();
  let created = 0;
  const gate = deferred();
  const create = async () => {
    created += 1;
    await gate.promise;
    return { dispose: async () => {} };
  };
  const first = acquireDshClient("plugin", create);
  const second = acquireDshClient("plugin", create);
  gate.resolve();
  const [a, b] = await Promise.all([first.ready(), second.ready()]);
  assert.equal(created, 1);
  assert.equal(a, b);
  assert.equal(dshClientHandleCount(), 2);
  await first.release();
  await second.release();
  assert.equal(dshClientHandleCount(), 0);
});

test("disposes only when the last holder releases", async () => {
  reset();
  const events = [];
  const create = async () => ({ dispose: async () => { events.push("dispose"); } });
  const first = acquireDshClient("plugin", create);
  const second = acquireDshClient("plugin", create);
  await first.ready();
  await second.ready();
  await first.release();
  assert.deepEqual(events, []);
  await second.release();
  assert.deepEqual(events, ["dispose"]);
  await second.release();
  assert.deepEqual(events, ["dispose"]);
});

test("releases the registry entry after a failed instantiation", async () => {
  reset();
  let attempts = 0;
  const create = async () => {
    attempts += 1;
    if (attempts === 1) throw new Error("boom");
    return { dispose: async () => {} };
  };
  const failing = acquireDshClient("plugin", create);
  await assert.rejects(() => failing.ready(), /boom/);
  await failing.release();
  assert.equal(dshClientHandleCount(), 0);
  const retry = acquireDshClient("plugin", create);
  await retry.ready();
  assert.equal(attempts, 2);
  await retry.release();
});

test("invalidate disposes the shared instance once and clears the entry", async () => {
  reset();
  const events = [];
  const create = async () => ({ dispose: async () => { events.push("dispose"); } });
  const handle = acquireDshClient("plugin", create);
  await handle.ready();
  await invalidateDshClient("plugin");
  assert.deepEqual(events, ["dispose"]);
  assert.equal(dshClientHandleCount(), 0);
  await handle.release();
  assert.deepEqual(events, ["dispose"]);
});

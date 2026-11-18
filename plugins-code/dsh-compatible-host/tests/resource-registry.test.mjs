import assert from "node:assert/strict";
import test from "node:test";
import { createDshClientResourceRegistry } from "../src/client-abi/resource-registry.mjs";

test("opens DSH resource providers and publishes identity-stable results", async () => {
  const registry = createDshClientResourceRegistry();
  const dispose = registry.register({
    protocol: "plan",
    async *open(address, { signal }) {
      assert.equal(address, "dsh-resource://plan/session-1/call-1");
      assert.equal(signal.aborted, false);
      yield { ok: true, value: { title: "Plan", markdown: "# Plan" } };
    },
  });
  let invalidations = 0;
  const unsubscribe = registry.subscribe("dsh-resource://plan/session-1/call-1", () => { invalidations += 1; });
  const stop = registry.watch("dsh-resource://plan/session-1/call-1");
  await new Promise((resolve) => setTimeout(resolve, 0));
  const snapshot = registry.getSnapshot("dsh-resource://plan/session-1/call-1");
  assert.equal(snapshot.status, "ready");
  assert.equal(snapshot.value.title, "Plan");
  assert.equal(invalidations > 0, true);
  stop();
  unsubscribe();
  dispose();
  assert.deepEqual(registry.list(), []);
});

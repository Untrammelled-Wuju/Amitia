import assert from "node:assert/strict";
import test from "node:test";
import { createDshConfigForms } from "../src/client-abi/config-forms.mjs";

test("exposes DSH config namespaces and preserves revision conflicts", async () => {
  const bridge = createDshConfigForms("config-fixture");
  assert.equal(bridge.configForms.developerTools.enabled.getSnapshot(), false);
  const events = [];
  const disposeEvent = bridge.onRemote("settings/document-updated", (event) => events.push(event));
  const controller = bridge.configForms.get("developerTools");
  assert.equal(await controller.mutate([{ op: "set", path: ["enabled"], value: true }], 0), true);
  assert.equal(bridge.configForms.developerTools.enabled.getSnapshot(), true);
  let notified = 0;
  const disposeField = bridge.configForms.developerTools.enabled.subscribe(() => { notified += 1; });
  assert.equal(await controller.mutate([{ op: "set", path: ["enabled"], value: true }], 1), true);
  assert.equal(notified, 1);
  disposeField();
  assert.equal(await controller.mutate([{ op: "set", path: ["enabled"], value: false }], 0), false);
  assert.deepEqual(events, [
    { namespace: "developerTools", revision: 1 },
    { namespace: "developerTools", revision: 2 },
  ]);
  disposeEvent();
  bridge.dispose();
});

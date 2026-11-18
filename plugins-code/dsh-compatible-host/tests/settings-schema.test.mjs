import assert from "node:assert/strict";
import test from "node:test";
import { createDshSettingsSchema } from "../src/client-abi/settings-schema.mjs";

test("provides immutable DSH settings path operations", () => {
  const schema = createDshSettingsSchema();
  const source = { model: { provider: "openai", nested: { enabled: true } }, list: ["a"] };
  const updated = schema.setPath(source, ["model", "nested", "enabled"], false);
  assert.equal(source.model.nested.enabled, true);
  assert.equal(updated.model.nested.enabled, false);
  assert.equal(schema.getPath(updated, ["model", "provider"]), "openai");
  assert.equal(schema.hasPath(updated, ["list", "0"]), true);
  assert.deepEqual(schema.deletePath(updated, ["model", "provider"]), {
    model: { nested: { enabled: false } },
    list: ["a"],
  });
});

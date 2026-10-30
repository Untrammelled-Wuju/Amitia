import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const schema = JSON.parse(fs.readFileSync(path.join(root, "mobile", "schema.json"), "utf8"));

function nodes(value) {
  if (!value || typeof value !== "object") return [];
  const result = [value];
  for (const child of value.children ?? []) result.push(...nodes(child));
  return result;
}

test("uses the unwrapped QQ Bot status result and refreshes automatically", () => {
  assert.equal(schema.lifecycle.refreshIntervalMs, 5000);
  assert.ok(schema.lifecycle.onRefresh.some((action) => action.action_id === "status"));
  const paths = nodes(schema)
    .flatMap((node) => node.bindings ?? [])
    .map((binding) => binding.sourcePath)
    .filter(Boolean);
  assert.deepEqual(paths, ["status"]);
  assert.equal(schema.actions.some((action) => action.actionId === "config"), false);
  assert.equal(nodes(schema).some((node) => node.id === "qq-saved-config"), false);
});

test("switches connect and disconnect buttons from the live QQ status", () => {
  const connect = nodes(schema).find((node) => node.id === "qq-connect");
  const disconnect = nodes(schema).find((node) => node.id === "qq-disconnect");
  assert.deepEqual(connect.visibleWhen, [
    { field: "status.qqOnline", operator: "!=", value: true },
  ]);
  assert.deepEqual(disconnect.visibleWhen, [
    { field: "status.qqOnline", operator: "==", value: true },
  ]);
});

import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const schema = JSON.parse(fs.readFileSync(path.join(root, "mobile", "schema.json"), "utf8"));
const messages = JSON.parse(fs.readFileSync(path.join(root, "mobile", "messages.schema.json"), "utf8"));

function nodes(value) {
  if (!value || typeof value !== "object") return [];
  const result = [value];
  for (const child of value.children ?? []) result.push(...nodes(child));
  return result;
}

function bindingPaths(document) {
  return nodes(document)
    .flatMap((node) => node.bindings ?? [])
    .map((binding) => binding.sourcePath);
}

test("binds mobile status to the host command result without a data envelope", () => {
  assert.equal(schema.lifecycle.refreshIntervalMs, 5000);
  assert.ok(schema.lifecycle.onRefresh.some((action) => action.action_id === "status"));
  const paths = bindingPaths(schema);
  assert.ok(paths.includes("status.status"));
  assert.ok(paths.includes("status.accountId"));
  assert.ok(paths.includes("status.message"));
  assert.ok(paths.includes("status.lastError"));
  assert.equal(paths.includes("status.data"), false);
  assert.equal(paths.some((value) => String(value).startsWith("config.")), false);
  assert.equal(nodes(schema).some((node) => node.type === "code"), false);
});

test("shows the QR image from the connect response and hides it after login", () => {
  const qr = nodes(schema).find((node) => node.id === "wechat-mobile-qr");
  assert.ok(qr);
  assert.equal(qr.bindings[0].sourcePath, "connectResult.data.qrImageUrl");
  assert.deepEqual(qr.visibleWhen, [
    { field: "connectResult.data.qrImageUrl", operator: "not_null" },
    { field: "status.status", operator: "!=", value: "connected" },
  ]);
  const connect = nodes(schema).find((node) => node.id === "wechat-mobile-connect");
  const disconnect = nodes(schema).find((node) => node.id === "wechat-mobile-disconnect");
  const actions = schema.children[0].children.find((node) => node.id === "wechat-mobile-actions");
  assert.equal(actions.type, "row");
  assert.equal(connect.actions[0].statePath, "connectResult");
  assert.equal(connect.actions[0].input.config.force, true);
  assert.equal(disconnect.visibleWhen[0].value, "connected");
});

test("keeps the optional mobile messages schema on the same host state contract", () => {
  assert.equal(messages.lifecycle.refreshIntervalMs, 5000);
  const paths = bindingPaths(messages);
  assert.ok(paths.includes("status"));
  assert.ok(paths.includes("messages"));
  assert.equal(paths.includes("status.data"), false);
  assert.equal(paths.includes("messages.data"), false);
});

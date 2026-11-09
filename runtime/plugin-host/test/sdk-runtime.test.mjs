import assert from "node:assert/strict";
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { spawn } from "node:child_process";
import { createInterface } from "node:readline";
import test from "node:test";

test("host provides the SDK and invokes declared tools", async () => {
  const root = mkdtempSync(join(tmpdir(), "amitia-plugin-host-"));
  const entry = join(root, "index.js");
  writeFileSync(entry, `
"use strict";
const { defineExtension } = require("@amitia/plugin-sdk");
const { defineTool, successResult } = require("@amitia/plugin-sdk/tools");
exports.default = defineExtension({
  async activate() {
    defineTool({
      toolId: "com.example/host-test/hello",
      title: { default: "Hello" },
      description: { default: "Host test" },
      parameters: { type: "object", properties: {} },
      runtimeBinding: "host_internal",
      metadata: { handlerName: "hello" },
    }, async () => successResult({ ok: true }));
  },
  async deactivate() {},
});
`);
  const child = spawn(process.execPath, [join(import.meta.dirname, "..", "dist", "index.js")], {
    cwd: join(import.meta.dirname, ".."),
    stdio: ["pipe", "pipe", "pipe"],
  });
  const rl = createInterface({ input: child.stdout });
  const result = await new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error("plugin host timeout")), 10000);
    child.stderr.on("data", (chunk) => process.stderr.write(chunk));
    rl.on("line", (line) => {
      const message = JSON.parse(line);
      if (message.id === "init" && message.result) {
        child.stdin.write(JSON.stringify({
          jsonrpc: "2.0",
          id: "call",
          method: "runtime.invoke",
          params: { contributionId: "hello", input: {} },
        }) + "\n");
      }
      if (message.id === "call") {
        clearTimeout(timer);
        resolve(message);
      }
    });
    child.once("error", reject);
    child.stdin.write(JSON.stringify({
      jsonrpc: "2.0",
      id: "init",
      method: "runtime.initialize",
      params: { entry, extensionId: "com.example/host-test", moduleId: "main", generation: 1 },
    }) + "\n");
  });
  child.kill("SIGTERM");
  assert.deepEqual(result.result, { ok: true });
});

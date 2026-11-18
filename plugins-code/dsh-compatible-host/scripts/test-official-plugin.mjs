import assert from "node:assert/strict";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { installDshPackage } from "../src/runtime/package-manager.mjs";
import { invokeDshTool, loadDshHostPlugin } from "../src/runtime/host-abi.mjs";
import { createAmitiaDshHostServices } from "../src/runtime/host-services.mjs";

const spec = process.argv[2] || "@deepseek-ai/dsh-tool-todo@0.2.2-rc.2";
const config = process.argv[3] ? JSON.parse(process.argv[3]) : {};
const pluginId = spec.replace(/^npm:/, "").replace(/@[^/]+$/, "");
const root = await mkdtemp(join(tmpdir(), "amitia-dsh-official-"));
const pluginDataRoot = join(root, "plugin-data");

try {
  const installed = await installDshPackage(spec, { pluginDataRoot });
  const services = createAmitiaDshHostServices({ call: async () => ({}) });
  if (typeof config.provider === "string" && services.subagents && services.subagents.getProvider(config.provider) === undefined) {
    services.subagents.registerProvider({
      name: config.provider,
      capabilities: { agentOptions: true, agentPreset: false, outputSchema: false, depthLimit: true },
      inheritsParentContext: false,
      async start() {
        return {
          id: "child-test",
          result: Promise.resolve({ output: [{ type: "text", text: "delegated" }], stopReason: "completed" }),
          async dispose() {},
        };
      },
    });
  }
  const runtime = await loadDshHostPlugin({
    packageDir: installed.packageDir,
    entry: installed.entry,
    pluginId,
    services,
    config,
  });
  assert.ok(runtime.tools.length > 0);
  const toolNames = runtime.tools.map((tool) => tool.name);
  if (runtime.tools.some((tool) => tool.name === "todo_write")) {
    const result = await invokeDshTool(runtime, "todo_write", {
      todos: [{ content: "Inspect source", status: "in_progress" }],
    }, {
      agent: { session: { append() {} } },
    });
    assert.equal(result.counts.inProgress, 1);
  }
  if (runtime.tools.some((tool) => tool.name === "subagent")) {
    const result = await invokeDshTool(runtime, "subagent", {
      description: "Inspect fixture",
      prompt: "Inspect the fixture and report back",
    }, {
      signal: new AbortController().signal,
      agent: { id: "conversation-test", session: { append() {}, projection() { return {}; } } },
    });
    assert.equal(result.kind, "activation");
    assert.equal(result.name, "Inspect fixture:child-test");
  }
  await runtime.dispose();
  console.log(JSON.stringify({ pluginId, tools: toolNames }));
} finally {
  await rm(root, { recursive: true, force: true });
}

import assert from "node:assert/strict";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { installDshPackage, runNpmInstall } from "../src/runtime/package-manager.mjs";
import { invokeDshTool, loadDshHostPlugin } from "../src/runtime/host-abi.mjs";
import { createAmitiaDshHostServices } from "../src/runtime/host-services.mjs";

const root = await mkdtemp(join(tmpdir(), "amitia-dsh-mcp-"));
const pluginDataRoot = join(root, "plugin-data");
const serverRoot = join(root, "mcp-server");

try {
  await runNpmInstall("@modelcontextprotocol/server-everything@2.3.0", serverRoot);
  const serverEntry = join(serverRoot, "node_modules", "@modelcontextprotocol", "server-everything", "dist", "index.js");
  const installed = await installDshPackage("@deepseek-ai/dsh-mcp-client@0.2.2-rc.2", { pluginDataRoot });
  const services = createAmitiaDshHostServices({
    call: async (method, input) => {
      if (method === "host.state.get") return { found: false, version: 0 };
      if (method === "host.state.cas") return { swapped: true, version: 1 };
      return {};
    },
  });
  const runtime = await loadDshHostPlugin({
    packageDir: installed.packageDir,
    entry: installed.entry,
    pluginId: installed.pluginId,
    services,
    config: {
      transport: "stdio",
      serverName: "everything",
      command: process.execPath,
      args: [serverEntry, "stdio"],
      env: {},
      cwd: root,
      toolCallTimeoutMs: 5000,
      startupTimeoutMs: 10000,
      closeTimeoutMs: 1000,
      expandTemplates: false,
      failOnStartupError: true,
    },
  });
  for (let attempt = 0; attempt < 100 && runtime.tools.length === 0; attempt += 1) {
    await new Promise((resolve) => setTimeout(resolve, 50));
  }
  assert.ok(runtime.tools.length > 0, "MCP client did not register tools");
  assert.ok(services.mcpResources.list().includes("everything"), "MCP client did not register resource provider");
  const resources = await services.mcpResources.request("everything", { method: "resources/list" }, {
    signal: new AbortController().signal,
    agent: { id: "conversation-mcp", session: { append() {} } },
  });
  assert.ok(resources);
  const firstUri = resources.resources?.[0]?.uri;
  assert.ok(firstUri);
  const resource = await services.mcpResources.request("everything", { method: "resources/read", uri: firstUri }, {
    signal: new AbortController().signal,
    agent: { id: "conversation-mcp", session: { append() {} } },
  });
  assert.ok(resource);
  const tool = runtime.tools.find((item) => item.name.endsWith("__echo")) || runtime.tools[0];
  const input = tool.name.endsWith("__echo") ? { message: "hello" } : {};
  const result = await invokeDshTool(runtime, tool.name, input, { signal: new AbortController().signal });
  assert.ok(result !== undefined);
  console.log(JSON.stringify({ plugin: installed.pluginId, tools: runtime.tools.map((item) => item.name), resourceUri: firstUri, firstResult: result }));
  await runtime.dispose();
} finally {
  await rm(root, { recursive: true, force: true });
}

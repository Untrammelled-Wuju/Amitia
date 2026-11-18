import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { installDshPackage } from "../src/runtime/package-manager.mjs";
import { createDshHostContext, loadDshHostPlugin } from "../src/runtime/host-abi.mjs";
import { createAmitiaDshHostServices } from "../src/runtime/host-services.mjs";

const specs = process.argv.slice(2);
if (specs.length === 0) throw new Error("必须按加载顺序传入至少一个官方包 spec");

const root = await mkdtemp(join(tmpdir(), "amitia-dsh-official-chain-"));
const pluginDataRoot = join(root, "plugin-data");
const services = createAmitiaDshHostServices({ call: async () => ({}) });
const host = createDshHostContext({ pluginId: "official-chain", services });
const loaded = [];

try {
  for (const spec of specs) {
    const installed = await installDshPackage(spec, { pluginDataRoot });
    try {
      const runtime = await loadDshHostPlugin({
        packageDir: installed.packageDir,
        entry: installed.entry,
        pluginId: installed.pluginId,
        services,
        context: host.context,
        deferInjectCheck: false,
      });
      loaded.push(runtime);
      console.log(JSON.stringify({
        spec,
        pluginId: installed.pluginId,
        entries: installed.entries,
        clientEntryCount: installed.clientEntries.length,
        tools: runtime.tools.map((tool) => tool.name),
        ok: true,
      }));
    } catch (error) {
      console.log(JSON.stringify({
        spec,
        pluginId: installed.pluginId,
        entries: installed.entries,
        ok: false,
        code: error?.code ?? null,
        message: error?.message ?? String(error),
      }));
    }
  }
  const probes = ["typert", "jobs", "jobController"];
  console.log(JSON.stringify({
    provided: Object.fromEntries(probes.map((name) => [name, host.context.get(name) !== undefined])),
  }));
} finally {
  for (const runtime of loaded.reverse()) await runtime.dispose();
  await host.dispose();
  await rm(root, { recursive: true, force: true });
}

import Module from "node:module";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = dirname(dirname(fileURLToPath(import.meta.url)));
const sourcePath = join(root, "src", "index.mjs");
let extension = null;
globalThis.defineExtension = (value) => {
  extension = value;
};
const moduleInstance = new Module(sourcePath);
moduleInstance.filename = sourcePath;
moduleInstance.paths = Module._nodeModulePaths(dirname(sourcePath));
moduleInstance._compile(readFileSync(sourcePath, "utf8"), sourcePath);
delete globalThis.defineExtension;

const store = new Map();
const tools = new Map();
const resources = new Map();
const handles = new Map();
let version = 0;

const host = {
  async call(method, params) {
    if (method === "host.state.get") {
      return { found: store.has(params.key), value: store.get(params.key), version };
    }
    if (method === "host.state.cas") {
      if (params.expectedVersion !== version) return { swapped: false, newVersion: version };
      store.set(params.key, params.newValue);
      version += 1;
      return { swapped: true, newVersion: version };
    }
    if (method === "host.resource.open") {
      const id = `handle-${Math.random()}`;
      if (!resources.has(params.path)) resources.set(params.path, Buffer.alloc(0));
      handles.set(id, params.path);
      return { handleId: id };
    }
    if (method === "host.resource.write") {
      const path = handles.get(params.handleId);
      const current = resources.get(path) || Buffer.alloc(0);
      resources.set(path, Buffer.concat([current, Buffer.from(params.data || "", params.encoding || "utf8")]));
      return { written: 1 };
    }
    if (method === "host.resource.read") return { data: "", eof: true, encoding: "base64" };
    if (method === "host.resource.close") {
      handles.delete(params.handleId);
      return { ok: true };
    }
    if (method === "host.resource.link") return { url: `/api/extension/resources/${encodeURIComponent(params.path)}` };
    if (method === "host.resource.delete") {
      resources.delete(params.path);
      return { deleted: true };
    }
    if (method === "host.vector.upsert") return { upserted: params.points.length };
    if (method === "host.vector.search") return { items: [] };
    if (method === "host.vector.delete") return { deleted: params.ids.length };
    if (method === "host.character.list") return { items: [] };
    if (method === "host.conversation.message.append") return { messageIds: ["message-1"], sequences: [1], responseGroupId: "response-1", lastSequence: 1 };
    throw new Error(`unexpected host method: ${method}`);
  },
};

await extension.activate({
  host,
  handlers: { bindTool(name, handler) { tools.set(name, handler); } },
  log: { info() {}, warn() {}, error() {}, debug() {} },
});

const groups = await tools.get("command")({ action: "groups.create", payload: { name: "测试分组" } });
if (!groups || !groups.id) throw new Error("group command failed");
const listed = await tools.get("command")({ action: "groups.list", payload: {} });
if (listed.length !== 1 || listed[0].name !== "测试分组") throw new Error("group state failed");
const output = await tools.get("output")({ characterId: "char-1", conversationId: "conv-1", reply: "你好", lines: ["你好"], source: "manual" });
if (!Array.isArray(output.outputs)) throw new Error("output contract failed");
console.log("runtime=ok");

import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { JSDOM } from "jsdom";
import { build } from "esbuild";
import test from "node:test";

const packageRoot = fileURLToPath(new URL("..", import.meta.url));

function sourceChunk(code, input = {}) {
  const bytes = Buffer.from(code, "utf8");
  const offset = Math.min(bytes.length, Number(input.offset) || 0);
  const length = Math.min(1024 * 1024, Math.max(1, Number(input.length) || 1024 * 1024));
  const end = Math.min(bytes.length, offset + length);
  return {
    dataBase64: bytes.subarray(offset, end).toString("base64"),
    offset,
    nextOffset: end,
    totalBytes: bytes.length,
    done: end >= bytes.length,
  };
}

async function loadClientRuntime() {
  const clientBuild = await build({
    entryPoints: [join(packageRoot, "src", "client-ui", "index.mjs")],
    outfile: join(packageRoot, ".conversation-node-client-ui-test.mjs"),
    bundle: true,
    format: "esm",
    platform: "browser",
    target: ["chrome110"],
    write: false,
    define: { "process.env.NODE_ENV": "\"production\"" },
    loader: {
      ".woff": "dataurl",
      ".woff2": "dataurl",
      ".ttf": "dataurl",
      ".png": "dataurl",
      ".svg": "dataurl",
      ".css": "css",
    },
  });
  const javascript = clientBuild.outputFiles.find((file) => file.path.endsWith(".js") || file.path.endsWith(".mjs"));
  if (!javascript) throw new Error("client UI JavaScript output is missing");
  const dom = new JSDOM("<!doctype html><html><head></head><body><div id=\"amitia-overlay-root\"></div></body></html>", {
    url: "http://localhost",
  });
  const previous = {
    window: globalThis.window,
    document: globalThis.document,
    navigator: Object.getOwnPropertyDescriptor(globalThis, "navigator"),
  };
  globalThis.window = globalThis;
  globalThis.addEventListener = dom.window.addEventListener.bind(dom.window);
  globalThis.removeEventListener = dom.window.removeEventListener.bind(dom.window);
  globalThis.dispatchEvent = dom.window.dispatchEvent.bind(dom.window);
  globalThis.matchMedia = dom.window.matchMedia?.bind(dom.window);
  globalThis.document = dom.window.document;
  Object.defineProperty(globalThis, "navigator", { configurable: true, value: dom.window.navigator });
  const nodeRegistrations = [];
  const nodeDisposals = [];
  const slotRegistrations = [];
  globalThis.amitiaClientPlugins = {
    foreignComponents: {
      get() {},
      register() {},
    },
    slots: {
      getDefinition(id) {
        return id === "chat.conversation.node" ? { slotId: id, kind: "list", scope: "session" } : undefined;
      },
      register(pluginId, slotId, key, component, options) {
        slotRegistrations.push({ pluginId, slotId, key, component, options });
        return async () => {};
      },
      async inject() {
        return () => {};
      },
    },
    registerConversationNode(pluginId, key, component, definition, options) {
      nodeRegistrations.push({ pluginId, key, component, definition, options });
      return async () => {
        nodeDisposals.push(key);
      };
    },
    locale: {
      register() {
        return () => {};
      },
      setLocale() {},
      getLocale() {
        return "zh-CN";
      },
    },
  };
  globalThis.__AMITIA_VUE__ = {
    defineComponent(value) {
      return value;
    },
    ref(value) {
      return { value };
    },
    h() {
      return null;
    },
    onMounted() {},
    onUpdated() {},
    onBeforeUnmount() {},
  };
  const clientCode = await readFile(join(packageRoot, "tests", "fixtures", "conversation-node-plugin", "client.js"), "utf8");
  globalThis.__dshHostCommand = async (input) => sourceChunk(clientCode, input);
  const moduleUrl = `data:text/javascript;base64,${Buffer.from(javascript.text).toString("base64")}`;
  const runtime = await import(moduleUrl);
  const loaded = await runtime.instantiateClient({
    pluginId: "@amitia-test/conversation-node-plugin",
    clientEntry: "./client.js",
  }, {
    loadClientSource: async (plugin, entry) => {
      const chunks = [];
      let offset = 0;
      for (;;) {
        const chunk = await globalThis.__dshHostCommand({
          action: "client-source",
          pluginId: plugin.pluginId,
          entryId: entry.id,
          offset,
          length: 1024 * 1024,
        });
        chunks.push(Buffer.from(chunk.dataBase64, "base64"));
        if (chunk.done) break;
        offset = chunk.nextOffset;
      }
      new Function(Buffer.concat(chunks).toString("utf8"))();
    },
  });
  const restore = () => {
    delete globalThis.__dshHostCommand;
    delete globalThis.amitiaClientPlugins;
    delete globalThis.__AMITIA_VUE__;
    globalThis.window = previous.window;
    globalThis.document = previous.document;
    if (previous.navigator) Object.defineProperty(globalThis, "navigator", previous.navigator);
    else delete globalThis.navigator;
  };
  const teardown = async () => {
    await loaded?.dispose?.();
    await new Promise((resolve) => setTimeout(resolve, 30));
    restore();
  };
  return { runtime, loaded, nodeRegistrations, nodeDisposals, slotRegistrations, restore, teardown };
}

test("conversation node 非文本键通过 conversationNodes 注册并保留文本节点屏蔽", async () => {
  const { loaded, nodeRegistrations, nodeDisposals, slotRegistrations, restore } = await loadClientRuntime();
  assert.deepEqual(nodeRegistrations.map((item) => item.key).sort(), ["notice.info", "tool.status"]);
  assert.deepEqual(slotRegistrations, []);
  const notice = nodeRegistrations.find((item) => item.key === "notice.info");
  assert.equal(typeof notice.definition.match, "function");
  assert.equal(typeof notice.definition.create, "function");
  assert.equal(typeof notice.definition.update, "function");
  assert.equal(typeof notice.definition.project, "function");
  assert.equal(notice.definition.match({ eventType: "tool.status" }), null);
  assert.equal(notice.definition.match({ eventType: "notice.info", id: "event-1" }).contextId, "dsh-node:notice.info:event-1");
  assert.equal(notice.definition.project({ state: { payload: { title: "已保存" } } }).key, "notice.info");
  assert.deepEqual(notice.options, { id: "node-notice", key: "notice.info", ordering: 15 });
  const tool = nodeRegistrations.find((item) => item.key === "tool.status");
  assert.equal(tool.definition.match({ eventType: "tool.status", id: "event-2" }).contextId, "dsh-node:tool.status:event-2");
  assert.equal(tool.definition.project({ state: { payload: {} } }).key, "tool.status");
  assert.deepEqual(tool.options, { id: "node-tool", key: "tool.status", ordering: 25 });
  assert.equal(typeof notice.component.setup, "function");
  await loaded.dispose();
  assert.deepEqual(nodeDisposals.sort(), ["notice.info", "tool.status"]);
  await new Promise((resolve) => setTimeout(resolve, 30));
  restore();
});

test("节点 owner 适配器补齐 content、node 与 event 并保留 Amitia owner 字段", async () => {
  const { runtime, teardown } = await loadClientRuntime();
  const node = {
    nodeId: "node-1",
    nodeType: "tool.status",
    eventType: "tool/status",
    conversationId: "conversation-1",
    title: "读取文件",
    payload: { text: "读取文件" },
    events: [{ payload: { text: "读取文件" } }],
    event: { type: "tool/status", data: { path: "/tmp/one" } },
  };
  const owner = runtime.adaptConversationNodeOwner({
    eventType: "tool/status",
    conversationNode: node,
    conversationId: "conversation-1",
  });
  assert.deepEqual(owner.content, { display: "读取文件", text: "读取文件", value: "读取文件" });
  assert.equal(owner.node, node);
  assert.equal(owner.event, node.event);
  assert.equal(owner.conversationNode, node);
  assert.equal(owner.eventType, "tool/status");
  assert.equal(owner.conversationId, "conversation-1");
  assert.equal(owner.nodeType, "tool.status");

  const probe = () => null;
  const element = runtime.createConversationNodeOwnerAdapter(probe)({
    eventType: "notice/info",
    conversationNode: { nodeType: "notice.info", eventType: "notice/info", conversationId: "conversation-2", payload: { message: "已保存" } },
  });
  assert.equal(element.type, probe);
  assert.equal(element.props.content.display, "已保存");
  assert.equal(element.props.eventType, "notice/info");
  assert.equal(element.props.conversationId, "conversation-2");

  const empty = runtime.adaptConversationNodeOwner({ eventType: "notice/info", conversationNode: { nodeType: "notice.info" } });
  assert.deepEqual(empty.content, { display: "", text: "", value: "" });
  assert.equal(empty.event, null);

  const absent = runtime.adaptConversationNodeOwner({});
  assert.deepEqual(absent.content, { display: "", text: "", value: "" });
  assert.equal(absent.node, null);
  assert.equal(absent.event, null);
  assert.equal(absent.eventType, "");
  await teardown();
});

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
  const length = Math.min(256 * 1024, Math.max(1, Number(input.length) || 256 * 1024));
  const end = Math.min(bytes.length, offset + length);
  return {
    dataBase64: bytes.subarray(offset, end).toString("base64"),
    offset,
    nextOffset: end,
    totalBytes: bytes.length,
    done: end >= bytes.length,
  };
}

test("claims a host predeclared child slot so the DSH entry can render it", async () => {
  const clientBuild = await build({
    entryPoints: [join(packageRoot, "src", "client-ui", "index.mjs")],
    outfile: join(packageRoot, ".child-slot-test.mjs"),
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
  const previousWindow = globalThis.window;
  const previousDocument = globalThis.document;
  const previousNavigator = Object.getOwnPropertyDescriptor(globalThis, "navigator");
  globalThis.window = globalThis;
  globalThis.addEventListener = dom.window.addEventListener.bind(dom.window);
  globalThis.removeEventListener = dom.window.removeEventListener.bind(dom.window);
  globalThis.dispatchEvent = dom.window.dispatchEvent.bind(dom.window);
  globalThis.matchMedia = dom.window.matchMedia?.bind(dom.window);
  globalThis.document = dom.window.document;
  Object.defineProperty(globalThis, "navigator", { configurable: true, value: dom.window.navigator });

  const registrations = [];
  const adapters = new Map();
  const hostDefinition = {
    slotId: "app.workspace.toast",
    contractVersion: 1,
    supportedKinds: ["panel"],
    kind: "list",
    scope: "root",
    allowRootChild: true,
  };
  globalThis.amitiaClientPlugins = {
    foreignComponents: {
      get(id) {
        return adapters.get(id);
      },
      register(adapter) {
        adapters.set(adapter.id, adapter);
      },
    },
    slots: {
      getDefinition(id) {
        if (id === "ui.portal.overlay") return { slotId: id };
        if (id === "app.workspace.toast") return hostDefinition;
        return undefined;
      },
      acceptsChildDeclaration() {
        return true;
      },
      register(pluginId, slotId, key, component, options) {
        registrations.push({ pluginId, slotId, key, component, options });
        return async () => {};
      },
      async inject() {
        return () => {};
      },
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
    render() {},
  };

  const clientCode = await readFile(join(packageRoot, "tests", "fixtures", "child-slot-plugin", "client.js"), "utf8");
  globalThis.__dshHostCommand = async (input) => sourceChunk(clientCode, input);
  const loadClientSource = async (plugin, entry) => {
    const chunk = await globalThis.__dshHostCommand({
      action: "client-source",
      pluginId: plugin.pluginId,
      entryId: entry.id,
    });
    new Function(Buffer.from(chunk.dataBase64, "base64").toString("utf8"))();
  };

  try {
    const moduleUrl = `data:text/javascript;base64,${Buffer.from(javascript.text).toString("base64")}`;
    const clientRuntime = await import(moduleUrl);
    const loaded = await clientRuntime.instantiateClient({
      pluginId: "@amitia-test/child-slot-fixture",
      clientEntry: "./client.js",
    }, { loadClientSource });

    assert.equal(registrations.length, 1);
    const children = registrations[0].options.children;
    assert.equal(children.length, 1, "the predeclared host child must still be declared for render authority");
    assert.equal(children[0].slotId, "app.workspace.toast");
    assert.equal(children[0].kind, "list", "the host definition wins over the DSH spec kind");
    assert.equal(children[0].scope, "root");
    assert.deepEqual(children[0].supportedKinds, ["panel"]);

    await loaded.dispose();
  } finally {
    delete globalThis.__dshHostCommand;
    delete globalThis.amitiaClientPlugins;
    delete globalThis.__AMITIA_VUE__;
    globalThis.window = previousWindow;
    globalThis.document = previousDocument;
    if (previousNavigator) Object.defineProperty(globalThis, "navigator", previousNavigator);
    else delete globalThis.navigator;
    dom.window.close();
  }
});

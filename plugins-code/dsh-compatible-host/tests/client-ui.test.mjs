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

async function loadClientSource(plugin, entry) {
  const chunks = [];
  let offset = 0;
  while (true) {
    const chunk = await globalThis.__dshHostCommand({
      action: "client-source",
      pluginId: plugin.pluginId,
      entryId: entry.id,
      offset,
      length: 256 * 1024,
    });
    chunks.push(Buffer.from(chunk.dataBase64, "base64"));
    if (chunk.done) break;
    offset = chunk.nextOffset;
  }
  new Function(Buffer.concat(chunks).toString("utf8"))();
}

test("loads a DSH client plugin and registers its React slot", async () => {
  const clientBuild = await build({
    entryPoints: [join(packageRoot, "src", "client-ui", "index.mjs")],
    outfile: join(packageRoot, ".client-ui-test.mjs"),
    bundle: true,
    format: "esm",
    platform: "browser",
    target: ["chrome110"],
    write: false,
    define: {
      "process.env.NODE_ENV": "\"production\"",
    },
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
  const disposers = [];
  const adapters = new Map();
  globalThis.amitiaClientPlugins = {
    foreignComponents: {
      get(id) {
        return adapters.get(id);
      },
      register(adapter) {
        adapters.set(adapter.id, adapter);
        return () => adapters.delete(adapter.id);
      },
    },
    slots: {
      getDefinition(id) {
        return id === "ui.portal.overlay" ? { slotId: id } : undefined;
      },
      register(pluginId, slotId, key, component, options) {
        registrations.push({ pluginId, slotId, key, component, options });
        const dispose = () => disposers.push(`${slotId}:${key}`);
        return dispose;
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
  };
  const clientCode = await readFile(join(packageRoot, "tests", "fixtures", "sample-dsh-plugin", "client.js"), "utf8");
  globalThis.__dshHostCommand = async (input) => sourceChunk(clientCode, input);

  const moduleUrl = `data:text/javascript;base64,${Buffer.from(javascript.text).toString("base64")}`;
  const clientRuntime = await import(moduleUrl);
  const loaded = await clientRuntime.instantiateClient({
    pluginId: "@amitia-test/dsh-fixture",
    clientEntry: "./client.js",
  }, { loadClientSource });
  assert.equal(registrations.length, 1);
  assert.equal(registrations[0].slotId, "ui.portal.overlay");
  assert.equal(registrations[0].key, "fixture-overlay");
  assert.equal(registrations[0].options.ordering, 10);
  assert.equal(typeof registrations[0].component.setup, "function");

  await loaded.dispose();
  assert.deepEqual(disposers, ["ui.portal.overlay:fixture-overlay"]);

  delete globalThis.__dshHostCommand;
  delete globalThis.amitiaClientPlugins;
  delete globalThis.__AMITIA_VUE__;
  globalThis.window = previousWindow;
  globalThis.document = previousDocument;
  if (previousNavigator) Object.defineProperty(globalThis, "navigator", previousNavigator);
  else delete globalThis.navigator;
});

test("loads multiple React UI slots, locale and effect cleanup from one DSH plugin", async () => {
  const clientBuild = await build({
    entryPoints: [join(packageRoot, "src", "client-ui", "index.mjs")],
    outfile: join(packageRoot, ".complex-client-ui-test.mjs"),
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
  const localeNamespaces = [];
  const adapters = new Map();
  const definitions = new Set([
    "ui.portal.overlay",
    "extension.detail.action",
    "extension.center.card.badge",
    "chat.sidebar.panel",
    "system.settings.section",
  ]);
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
        return definitions.has(id) ? { slotId: id } : undefined;
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
      register(ownerId, namespace, dictionaries) {
        localeNamespaces.push({ ownerId, namespace, dictionaries });
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
  globalThis.__complexClientDisposed = 0;
  const clientCode = await readFile(join(packageRoot, "tests", "fixtures", "complex-client-plugin", "client.js"), "utf8");
  globalThis.__dshHostCommand = async (input) => sourceChunk(clientCode, input);
  const moduleUrl = `data:text/javascript;base64,${Buffer.from(javascript.text).toString("base64")}`;
  const clientRuntime = await import(moduleUrl);
  const loaded = await clientRuntime.instantiateClient({
    pluginId: "@amitia-test/complex-client-plugin",
    clientEntry: "./client.js",
  }, { loadClientSource });

  assert.deepEqual(registrations.map((item) => item.slotId), [
    "extension.detail.action",
    "extension.center.card.badge",
    "chat.sidebar.panel",
    "system.settings.section",
  ]);
  assert.deepEqual(registrations[0].options.store, { count: 0 });
  assert.equal(typeof registrations[0].options.inject, "function");
  assert.equal(localeNamespaces[0].namespace, "complexFixture");
  await loaded.dispose();
  assert.equal(globalThis.__complexClientDisposed, 1);

  delete globalThis.__dshHostCommand;
  delete globalThis.__complexClientDisposed;
  delete globalThis.amitiaClientPlugins;
  delete globalThis.__AMITIA_VUE__;
  globalThis.window = previousWindow;
  globalThis.document = previousDocument;
  if (previousNavigator) Object.defineProperty(globalThis, "navigator", previousNavigator);
  else delete globalThis.navigator;
});

test("loads every client subpath export from a DSH bundle", async () => {
  const clientBuild = await build({
    entryPoints: [join(packageRoot, "src", "client-ui", "index.mjs")],
    outfile: join(packageRoot, ".multi-client-ui-test.mjs"),
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
  globalThis.amitiaClientPlugins = {
    foreignComponents: {
      get() {},
      register() {},
    },
    slots: {
      getDefinition(id) {
        return id === "extension.detail.action" ? { slotId: id } : undefined;
      },
      register(pluginId, slotId, key, component) {
        registrations.push({ pluginId, slotId, key, component });
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
        return "en";
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
  const sourceByEntry = {
    root: await readFile(join(packageRoot, "tests", "fixtures", "multi-entry-plugin", "client.js"), "utf8"),
    search: await readFile(join(packageRoot, "tests", "fixtures", "multi-entry-plugin", "search-client.js"), "utf8"),
  };
  globalThis.__dshHostCommand = async (input) => sourceChunk(sourceByEntry[input.entryId], input);
  const moduleUrl = `data:text/javascript;base64,${Buffer.from(javascript.text).toString("base64")}`;
  const clientRuntime = await import(moduleUrl);
  const loaded = await clientRuntime.instantiateClient({
    pluginId: "@amitia-test/multi-entry-plugin",
    clientEntries: [
      { id: "root", entry: "./client.js" },
      { id: "search", entry: "./search-client.js" },
    ],
  }, { loadClientSource });
  assert.equal(registrations.length, 1);
  assert.equal(registrations[0].pluginId, "@amitia-test/multi-entry-plugin/search");
  assert.equal(registrations[0].slotId, "extension.detail.action");
  assert.equal(registrations[0].key, "multi-search-action");
  await loaded.dispose();

  delete globalThis.__dshHostCommand;
  delete globalThis.amitiaClientPlugins;
  delete globalThis.__AMITIA_VUE__;
  globalThis.window = previousWindow;
  globalThis.document = previousDocument;
  if (previousNavigator) Object.defineProperty(globalThis, "navigator", previousNavigator);
  else delete globalThis.navigator;
});

test("passes injected hooks through the React adapter flat props contract", async () => {
  const clientBuild = await build({
    entryPoints: [join(packageRoot, "src", "client-ui", "index.mjs")],
    outfile: join(packageRoot, ".react-props-client-ui-test.mjs"),
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
  const contributionsById = new Map();
  let mountedInputs = [];
  const definitions = new Map([
    ["ui.portal.overlay", {
      slotId: "ui.portal.overlay",
      contractVersion: 1,
      supportedKinds: ["panel"],
      kind: "single",
      layout: "stack",
      fallbackPolicy: "none",
      scope: "root",
    }],
  ]);
  globalThis.amitiaClientPlugins = {
    foreignComponents: {
      get(id) {
        return adapters.get(id);
      },
      register(adapter) {
        adapters.set(adapter.id, adapter);
        return () => adapters.delete(adapter.id);
      },
      async mount(_adapterId, input) {
        mountedInputs.push(input);
        return {
          async update() {},
          async unmount() {},
        };
      },
    },
    slots: {
      contributionsById,
      getDefinition(id) {
        return definitions.get(id);
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
      bind() {
        return (key) => key;
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
    h(_tag, props = {}) {
      if (props.ref && typeof props.ref === "object") props.ref.value = {};
      return null;
    },
    onMounted(callback) {
      globalThis.__amitiaMountedHook = callback;
    },
    onUpdated() {},
    onBeforeUnmount() {},
    render() {},
  };
  globalThis.__dshHostCommand = async () => ({ ok: true });

  const clientSource = `
globalThis.__ModuleLoader__.load({
  id: "@amitia-test/react-props-plugin",
  factory(require) {
    const React = require("react");
    function RuntimeProbe() {
      return React.createElement("div");
    }
    return {
      apply(ctx) {
        ctx.slots.register({
          name: "shell.overlay",
          id: "runtime-probe",
          inject: () => ({
            hooks: {
              toast: {
                getSnapshot: () => ({ kind: "probe" }),
                subscribe: () => () => {},
              },
            },
            dismissToast() {},
          }),
        }, RuntimeProbe);
      },
    };
  },
});
`;
  const moduleUrl = `data:text/javascript;base64,${Buffer.from(javascript.text).toString("base64")}`;
  const clientRuntime = await import(moduleUrl);
  const loaded = await clientRuntime.instantiateClient({
    pluginId: "@amitia-test/react-props-plugin",
    clientEntry: "./client.js",
  }, {
    loadClientSource: async () => {
      new Function(clientSource)();
    },
  });
  assert.equal(registrations.length, 1);
  const registration = registrations[0];
  const contributionId = `client:${registration.pluginId}:${registration.slotId}:${registration.key}`;
  const injected = registration.options.inject({});
  contributionsById.set(contributionId, {
    scope: "root",
    instances: new Map([["root", { injected }]]),
  });

  const render = registration.component.setup(null, {
    attrs: {
      useToast: () => ({ kind: "probe" }),
      dismissToast() {},
      renderSlot: () => ({ __v_isVNode: true, type: "div", props: {} }),
      amitiaContributionId: contributionId,
      amitiaSessionId: "session-probe",
    },
    slots: {},
  });
  render();
  globalThis.__amitiaMountedHook();
  await Promise.resolve();
  assert.equal(mountedInputs.length, 1);
  const mountInput = mountedInputs[0];
  assert.equal(typeof mountInput.props.componentProps.useToast, "function");
  assert.equal(typeof mountInput.props.componentProps.dismissToast, "function");
  assert.ok(mountInput.props.componentProps.resources);

  const element = mountInput.props.component(mountInput.props.componentProps);
  assert.equal(typeof element.props.useToast, "function");
  assert.equal(typeof element.props.dismissToast, "function");
  assert.equal(element.props.amitiaContributionId, contributionId);
  const bridgedVNode = element.props.renderSlot("shell.overlay", {});
  assert.equal(typeof bridgedVNode.props.renderVNode, "function");
  assert.equal(bridgedVNode.props.renderVNode().__v_isVNode, true);

  const envelopeElement = mountInput.props.component({
    componentProps: mountInput.props.componentProps,
    resources: mountInput.props.resources,
  });
  assert.equal(typeof envelopeElement.props.useToast, "function");
  assert.equal(typeof envelopeElement.props.dismissToast, "function");

  await loaded.dispose();
  delete globalThis.__amitiaMountedHook;
  delete globalThis.__dshHostCommand;
  delete globalThis.amitiaClientPlugins;
  delete globalThis.__AMITIA_VUE__;
  globalThis.window = previousWindow;
  globalThis.document = previousDocument;
  if (previousNavigator) Object.defineProperty(globalThis, "navigator", previousNavigator);
  else delete globalThis.navigator;
});

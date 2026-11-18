import assert from "node:assert/strict";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { build } from "esbuild";
import { JSDOM } from "jsdom";
import { installDshPackage, readDshClientSource } from "../src/runtime/package-manager.mjs";

const spec = process.argv[2] || "@deepseek-ai/dsh-client-ui-jobs@0.2.2-rc.2";
const packageRoot = fileURLToPath(new URL("..", import.meta.url));
const root = await mkdtemp(join(tmpdir(), "amitia-dsh-official-client-"));

try {
  const installed = await installDshPackage(spec, { pluginDataRoot: join(root, "plugin-data") });
  const clientSource = await readDshClientSource(installed, "root");
  const clientBuild = await build({
    entryPoints: [join(packageRoot, "src", "client-ui", "index.mjs")],
    outfile: join(root, "client-ui-test.mjs"),
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
  assert.ok(javascript);

  const dom = new JSDOM("<!doctype html><html><head></head><body></body></html>", { url: "http://localhost" });
  globalThis.window = globalThis;
  globalThis.document = dom.window.document;
  globalThis.addEventListener = dom.window.addEventListener.bind(dom.window);
  globalThis.removeEventListener = dom.window.removeEventListener.bind(dom.window);
  globalThis.dispatchEvent = dom.window.dispatchEvent.bind(dom.window);
  globalThis.matchMedia = dom.window.matchMedia?.bind(dom.window);
  globalThis.MutationObserver = dom.window.MutationObserver;
  globalThis.Node = dom.window.Node;
  globalThis.Element = dom.window.Element;
  globalThis.HTMLElement = dom.window.HTMLElement;
  globalThis.DOMParser = dom.window.DOMParser;
  globalThis.getComputedStyle = dom.window.getComputedStyle.bind(dom.window);
  globalThis.requestAnimationFrame = (callback) => setTimeout(() => callback(Date.now()), 0);
  globalThis.cancelAnimationFrame = (handle) => clearTimeout(handle);
  globalThis.localStorage = dom.window.localStorage;
  globalThis.ResizeObserver = class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  };
  globalThis.IntersectionObserver = class IntersectionObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  };
  Object.defineProperty(globalThis, "navigator", { configurable: true, value: dom.window.navigator });
  const registrations = [];
  const dynamicSlots = new Map();
  const dictionaries = new Map();
  globalThis.amitiaClientPlugins = {
    foreignComponents: {
      get() {},
      register() {},
    },
    slots: {
      getDefinition(id) {
        if (dynamicSlots.has(id)) return dynamicSlots.get(id);
        return [
          "chat.header.action",
          "chat.header.leading",
          "chat.header.lineage",
          "chat.header.corner",
          "chat.message.badge",
          "chat.message.action",
          "chat.message.renderer",
          "chat.message.images",
          "chat.message.user.images",
          "chat.message.attachment_renderer",
          "chat.message.custom_renderer",
          "chat.message.user.text",
          "chat.approval.detail",
          "chat.message.command",
          "chat.conversation.flow",
          "chat.message.markdown.fence",
          "chat.message.reasoning.body",
          "chat.message.text.interaction",
          "chat.message.trajectory.images",
          "chat.message.tool.images",
          "chat.message.tool.cordis",
          "chat.conversation.node",
          "chat.sidebar.panel",
          "chat.composer.action",
          "chat.composer.dock.top",
          "chat.composer.dock.bottom",
          "chat.composer.left",
          "chat.composer.right",
          "chat.composer.overlay",
          "chat.composer.model",
          "chat.composer.model.label",
          "chat.composer.model.options",
          "chat.composer.permission",
          "provider.app.workspace.sidebar",
          "provider.page.provider",
          "provider.conversation.messages",
          "ui.portal.overlay",
          "ui.sidebar.right.pane.tab",
          "ui.sidebar.right.pane.tab.title",
          "ui.sidebar.right.pane.document",
          "ui.sidebar.right.pane.document.action",
          "ui.sidebar.right.pane.document.actions",
          "ui.sidebar.right.pane.document.pdf",
          "ui.sidebar.right.pane.document.unpreviewable",
          "ui.sidebar.right.pane.files.actions",
          "ui.sidebar.right.pane.guide",
          "ui.sidebar.right.pane.guide.actions",
          "ui.sidebar.right.pane.guide.entry",
          "ui.sidebar.right.pane.tab.menu.item",
          "app.rightbar",
          "app.rightbar.session",
          "app.workspace.toast",
          "app.shell.quota_notice",
          "app.shell.bottom",
          "desktop.context_menu.content",
          "desktop.context_menu.icon",
          "chat.conversation.root",
          "chat.conversation.session",
          "chat.conversation.header",
          "chat.conversation.session.header",
          "chat.conversation.subagent.footer",
          "chat.conversation.views",
          "chat.composer.root",
          "chat.composer.bar",
          "chat.composer.attachments",
          "chat.empty.brand_mark",
          "chat.empty.frame",
          "chat.empty.options",
          "chat.empty.workspace",
          "chat.empty.workspace.directory_flow",
          "chat.empty.agent_preset",
          "settings.shell",
          "settings.shell.header",
          "settings.shell.action",
          "settings.shell.close",
          "settings.shell.launcher",
          "settings.shell.launcher.notifications",
          "settings.onboarding",
          "system.settings.section",
          "settings.section.icon",
          "settings.shell.trigger",
          "settings.model.sign-in",
          "extension.center.header.action",
          "extension.center.card.badge",
          "extension.detail.tab",
          "extension.detail.action",
          "extension.settings.section",
          "settings.model.provider",
          "settings.model.footer",
          "extension.plugin.tab",
          "extension.plugin.item",
          "extension.plugin.bundle.activation",
          "extension.plugin.bundle.config",
          "extension.plugin.row.config",
          "extension.settings.tab",
        ].includes(id) ? { slotId: id } : undefined;
      },
      async declare(definition) {
        dynamicSlots.set(definition.slotId, definition);
        return {
          definition,
          async dispose() {
            if (dynamicSlots.get(definition.slotId) === definition) dynamicSlots.delete(definition.slotId);
          },
        };
      },
      register(pluginId, slotId, key, component, options = {}) {
        registrations.push({ pluginId, slotId, key, component, options });
        return async () => {};
      },
      listContributions(slotId) {
        return registrations
          .filter((item) => item.slotId === slotId)
          .map((item) => ({ options: item.options }));
      },
      async inject(slotId, callback) {
        const effect = await callback({ slotId });
        if (effect && typeof effect[Symbol.asyncIterator] === "function") {
          for await (const _ of effect) {}
        } else if (effect && typeof effect[Symbol.iterator] === "function") {
          for (const _ of effect) {}
        }
        return () => {};
      },
    },
    conversationNodes: {
      register() {
        return () => {};
      },
    },
    locale: {
      register(_pluginId, namespace, values) {
        dictionaries.set(namespace, values);
        return () => dictionaries.delete(namespace);
      },
      setLocale() {},
      getLocale() {
        return "zh-CN";
      },
      bind(namespace) {
        const values = dictionaries.get(namespace);
        return (key, params = {}) => {
          let value = values?.zh?.[key] ?? values?.en?.[key] ?? String(key);
          for (const [name, replacement] of Object.entries(params)) value = value.replaceAll(`{${name}}`, String(replacement));
          return value;
        };
      },
    },
  };
  globalThis.__AMITIA_VUE__ = {
    defineComponent(value) { return value; },
    ref(value) { return { value }; },
    h() { return null; },
    onMounted() {},
    onUpdated() {},
    onBeforeUnmount() {},
    render() {},
  };
  globalThis.__dshHostCommand = async (input) => {
    const bytes = Buffer.from(clientSource.code, "utf8");
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
  };
  const moduleUrl = `data:text/javascript;base64,${Buffer.from(javascript.text).toString("base64")}`;
  const clientRuntime = await import(moduleUrl);
  const loaded = await clientRuntime.instantiateClient({
    pluginId: installed.pluginId,
    clientEntries: installed.clientEntries,
  }, {
    loadClientSource: async (plugin, entry) => {
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
    },
  });
  assert.ok(registrations.length > 0);
  await loaded.dispose();
  console.log(JSON.stringify({ pluginId: installed.pluginId, slots: registrations.map((item) => item.slotId) }));
} finally {
  delete globalThis.__dshHostCommand;
  delete globalThis.amitiaClientPlugins;
  delete globalThis.__AMITIA_VUE__;
  await rm(root, { recursive: true, force: true });
}

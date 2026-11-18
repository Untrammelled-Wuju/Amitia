import assert from "node:assert/strict";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { build } from "esbuild";
import { JSDOM } from "jsdom";
import { installDshPackage, readDshClientSource } from "../src/runtime/package-manager.mjs";

const spec = process.argv[2] || "@deepseek-ai/dsh-client-ui-theme@0.2.2-rc.2";
const packageRoot = fileURLToPath(new URL("..", import.meta.url));
const root = await mkdtemp(join(tmpdir(), "amitia-dsh-official-mobile-client-"));

async function flush() {
  for (let index = 0; index < 8; index += 1) {
    await new Promise((resolve) => setTimeout(resolve, 0));
  }
}

try {
  const installed = await installDshPackage(spec, { pluginDataRoot: join(root, "plugin-data") });
  const clientSource = await readDshClientSource(installed, "root");
  const mobileBuild = await build({
    entryPoints: [join(packageRoot, "src", "mobile-client-ui", "index.mjs")],
    outfile: join(root, "mobile-client-ui.mjs"),
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
  const javascript = mobileBuild.outputFiles.find((file) => file.path.endsWith(".js") || file.path.endsWith(".mjs"));
  assert.ok(javascript);
  const dom = new JSDOM("<!doctype html><html><head></head><body><div id=\"root\"></div></body></html>", {
    url: "http://localhost/",
    pretendToBeVisual: true,
  });
  const previous = {
    window: globalThis.window,
    document: globalThis.document,
    navigator: Object.getOwnPropertyDescriptor(globalThis, "navigator"),
    Element: globalThis.Element,
    HTMLElement: globalThis.HTMLElement,
    HTMLIFrameElement: globalThis.HTMLIFrameElement,
    Node: globalThis.Node,
    Document: globalThis.Document,
    DOMParser: globalThis.DOMParser,
    MutationObserver: globalThis.MutationObserver,
    requestAnimationFrame: globalThis.requestAnimationFrame,
    addEventListener: globalThis.addEventListener,
    removeEventListener: globalThis.removeEventListener,
    dispatchEvent: globalThis.dispatchEvent,
  };
  globalThis.window = globalThis;
  globalThis.document = dom.window.document;
  globalThis.Element = dom.window.Element;
  globalThis.HTMLElement = dom.window.HTMLElement;
  globalThis.HTMLIFrameElement = dom.window.HTMLIFrameElement;
  globalThis.Node = dom.window.Node;
  globalThis.Document = dom.window.Document;
  globalThis.DOMParser = dom.window.DOMParser;
  globalThis.MutationObserver = dom.window.MutationObserver;
  globalThis.requestAnimationFrame = (callback) => {
    callback(0);
    return 1;
  };
  globalThis.addEventListener = dom.window.addEventListener.bind(dom.window);
  globalThis.removeEventListener = dom.window.removeEventListener.bind(dom.window);
  globalThis.dispatchEvent = dom.window.dispatchEvent.bind(dom.window);
  Object.defineProperty(globalThis, "navigator", { configurable: true, value: dom.window.navigator });
  try {
    const status = {
      targetDshVersion: "0.2.2-rc.2",
      packageCount: 1,
      enabledCount: 1,
      compatibility: { implemented: 19 },
      packages: [
        {
          pluginId: installed.pluginId,
          name: installed.name,
          version: installed.version,
          enabled: true,
          entry: installed.entry,
          entries: installed.entries,
          clientEntry: installed.clientEntry,
          clientEntries: installed.clientEntries,
          mappedContributionCount: 0,
        },
      ],
    };
    window.AmitiaHost = {
      async invokeAction(actionId, input) {
        assert.equal(actionId, "command");
        if (input.action === "status") return status;
        if (input.action === "client-source") return clientSource;
        return {};
      },
      async action(actionId, input) {
        return this.invokeAction(actionId, input);
      },
      async resize(height) {
        return { height };
      },
      async openRightPane() {
        return { open: true };
      },
      subscribeContext() {
        return () => {};
      },
    };
    const modulePath = join(root, "mobile-client-ui-runtime.mjs");
    await import("node:fs/promises").then(({ writeFile }) => writeFile(modulePath, javascript.text, "utf8"));
    const runtime = await import(`${new URL(`file://${modulePath}`).href}?test=${Date.now()}`);
    const container = document.getElementById("root");
    const mounted = await runtime.mount(container, {
      slotId: "system.settings.section",
      slotKind: "list",
      platform: "android",
      host: "mobile",
      locale: "zh-CN",
    });
    await flush();
    const text = container.textContent || "";
    if (text.includes("[original-error]") || text.includes("Error:")) {
      throw new Error(text);
    }
    assert.ok(text.trim().length > 0, `official mobile client ${installed.pluginId} rendered no settings content`);
    console.log(JSON.stringify({ pluginId: installed.pluginId, text: text.slice(0, 500) }));
    await mounted.dispose();
  } finally {
    delete globalThis.AmitiaHost;
    delete globalThis.__ModuleLoader__;
    globalThis.window = previous.window;
    globalThis.document = previous.document;
    globalThis.Element = previous.Element;
    globalThis.HTMLElement = previous.HTMLElement;
    globalThis.HTMLIFrameElement = previous.HTMLIFrameElement;
    globalThis.Node = previous.Node;
    globalThis.Document = previous.Document;
    globalThis.DOMParser = previous.DOMParser;
    globalThis.MutationObserver = previous.MutationObserver;
    globalThis.requestAnimationFrame = previous.requestAnimationFrame;
    globalThis.addEventListener = previous.addEventListener;
    globalThis.removeEventListener = previous.removeEventListener;
    globalThis.dispatchEvent = previous.dispatchEvent;
    if (previous.navigator) Object.defineProperty(globalThis, "navigator", previous.navigator);
    else delete globalThis.navigator;
    dom.window.close();
  }
} finally {
  await rm(root, { recursive: true, force: true });
}

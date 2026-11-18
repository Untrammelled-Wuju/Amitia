import assert from "node:assert/strict";
import { readFile, unlink, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { JSDOM } from "jsdom";
import { build } from "esbuild";
import test from "node:test";

const packageRoot = fileURLToPath(new URL("..", import.meta.url));

async function flush() {
  for (let index = 0; index < 5; index += 1) {
    await new Promise((resolve) => setTimeout(resolve, 0));
  }
}

test("loads DSH client plugins and renders the current mobile Amitia slot", async () => {
  const clientBuild = await build({
    entryPoints: [join(packageRoot, "src", "mobile-client-ui", "index.mjs")],
    outfile: join(packageRoot, ".mobile-client-ui-test.mjs"),
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
  if (!javascript) throw new Error("DSH mobile client UI JavaScript output is missing");

  const dom = new JSDOM("<!doctype html><html><head></head><body><div id=\"root\"></div></body></html>", {
    url: "http://localhost",
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
    MutationObserver: globalThis.MutationObserver,
    requestAnimationFrame: globalThis.requestAnimationFrame,
  };
  globalThis.window = globalThis;
  globalThis.addEventListener = dom.window.addEventListener.bind(dom.window);
  globalThis.removeEventListener = dom.window.removeEventListener.bind(dom.window);
  globalThis.dispatchEvent = dom.window.dispatchEvent.bind(dom.window);
  globalThis.document = dom.window.document;
  globalThis.Element = dom.window.Element;
  globalThis.HTMLElement = dom.window.HTMLElement;
  globalThis.HTMLIFrameElement = dom.window.HTMLIFrameElement;
  globalThis.Node = dom.window.Node;
  globalThis.Document = dom.window.Document;
  globalThis.MutationObserver = dom.window.MutationObserver;
  globalThis.requestAnimationFrame = (callback) => {
    callback(0);
    return 1;
  };
  Object.defineProperty(globalThis, "navigator", { configurable: true, value: dom.window.navigator });

  try {
    const clientCode = await readFile(join(packageRoot, "tests", "fixtures", "complex-client-plugin", "client.js"), "utf8");
    const commandCalls = [];
    let resizeCount = 0;
    window.AmitiaHost = {
      async invokeAction(actionId, input) {
        commandCalls.push({ actionId, input });
        if (input.action === "status") {
          return {
            status: "success",
            structured: {
              packages: [
                {
                  pluginId: "@amitia-test/complex-client-plugin",
                  name: "Complex Client",
                  version: "1.0.0",
                  enabled: true,
                  clientEntries: [{ id: "root", entry: "./client.js" }],
                },
              ],
            },
          };
        }
        if (input.action === "client-source") {
          return { status: "success", structured: { code: clientCode } };
        }
        return {};
      },
      async resize(height) {
        resizeCount += 1;
        assert.ok(height >= 0);
        return { height };
      },
      async openRightPane() {
        return { open: true };
      },
      subscribeContext() {
        return () => {};
      },
    };
    globalThis.__complexClientDisposed = 0;
    const modulePath = join(packageRoot, ".mobile-client-ui-runtime.mjs");
    await writeFile(modulePath, javascript.text, "utf8");
    const runtime = await import(`${new URL(`file://${modulePath}`).href}?test=${Date.now()}`);
    const root = document.getElementById("root");
    const mounted = await runtime.mount(root, {
      slotId: "system.settings.section",
      slotKind: "list",
      platform: "android",
      host: "mobile",
    });
    await flush();

    assert.match(root.textContent, /config-ready/);
    assert.ok(commandCalls.some((call) => call.input.action === "status"));
    assert.ok(commandCalls.some((call) => call.input.action === "client-source"));
    assert.ok(resizeCount > 0);

    await mounted.dispose();
    assert.equal(globalThis.__complexClientDisposed, 1);
    assert.equal(root.childElementCount, 0);
    await unlink(modulePath);
  } finally {
    delete globalThis.AmitiaHost;
    delete globalThis.__complexClientDisposed;
    delete globalThis.__ModuleLoader__;
    globalThis.window = previous.window;
    globalThis.document = previous.document;
    globalThis.Element = previous.Element;
    globalThis.HTMLElement = previous.HTMLElement;
    globalThis.HTMLIFrameElement = previous.HTMLIFrameElement;
    globalThis.Node = previous.Node;
    globalThis.Document = previous.Document;
    globalThis.MutationObserver = previous.MutationObserver;
    globalThis.requestAnimationFrame = previous.requestAnimationFrame;
    if (previous.navigator) Object.defineProperty(globalThis, "navigator", previous.navigator);
    else delete globalThis.navigator;
    dom.window.close();
  }
});

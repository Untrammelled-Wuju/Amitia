const { app, BrowserWindow, ipcMain, protocol } = require("electron");
const { readFile } = require("node:fs/promises");
const { mkdtempSync, rmSync, writeFileSync } = require("node:fs");
const { createServer } = require("node:http");
const { tmpdir } = require("node:os");
const path = require("node:path");

const root = path.resolve(__dirname, "../dist/renderer");
const data = mkdtempSync(path.join(tmpdir(), "amitia-renderer-smoke-"));
app.setPath("userData", data);
protocol.registerSchemesAsPrivileged([
  { scheme: "app", privileges: { standard: true, secure: true, supportFetchAPI: true, corsEnabled: true } },
]);
const errors = [];
let win;
let finished = false;
let apiServer;

function describeRemoteValue(value) {
  if (value instanceof Error) {
    return `${value.name}: ${value.message}\n${value.stack || ""}`;
  }
  if (value && typeof value === "object") {
    try {
      return JSON.stringify(value, (_key, item) => {
        if (item instanceof Error) return { name: item.name, message: item.message, stack: item.stack, cause: item.cause };
        return item;
      });
    } catch (error) {
      console.error("[original-error]", error);
      return String(value);
    }
  }
  return String(value);
}

function finish(error) {
  if (finished) return;
  finished = true;
  clearTimeout(timeout);
  if (error) console.error("[renderer-smoke] FAILED:", error);
  else console.log("[renderer-smoke] PASSED: production renderer mounted");
  win?.destroy();
  apiServer?.close();
  app.exit(error ? 1 : 0);
}

const timeout = setTimeout(async () => {
  const screenshot = path.join(tmpdir(), "amitia-renderer-smoke.png");
  let rendererErrors = [];
  try {
    rendererErrors = await win?.webContents.executeJavaScript("window.__amitiaSmokeErrors || []");
    const image = await win?.webContents.capturePage();
    if (image) writeFileSync(screenshot, image.toPNG());
  } catch (error) {
    console.error("[original-error]", error);
  }
  finish(`renderer did not mount within 20 seconds\nscreenshot=${screenshot}\nerrors=${[...errors, ...rendererErrors].join("\n---\n") || "(none)"}`);
}, 20000);
process.once("exit", () => {
  try { rmSync(data, { recursive: true, force: true }); } catch (error) {
  console.error("[original-error]", error);
}
});

app.whenReady().then(async () => {
  apiServer = createServer((request, response) => {
    const url = new URL(request.url || "/", "http://127.0.0.1");
    const headers = {
      "Access-Control-Allow-Origin": "app://amitia",
      "Access-Control-Allow-Credentials": "true",
      "Access-Control-Allow-Headers": request.headers["access-control-request-headers"] || "Content-Type, Authorization, X-Amitia-Client-Type",
      "Access-Control-Allow-Methods": "GET, POST, PUT, PATCH, DELETE, OPTIONS",
      "Content-Type": "application/json; charset=utf-8",
    };
    if (request.method === "OPTIONS") {
      response.writeHead(204, headers);
      response.end();
      return;
    }
    const data = url.pathname.endsWith("/capabilities")
      ? { runtimeProfile: "local", capabilities: {} }
      : { completed: true };
    response.writeHead(200, headers);
    response.end(JSON.stringify({ code: 200, msg: "ok", data }));
  });
  await new Promise((resolve, reject) => {
    apiServer.once("error", reject);
    apiServer.listen(0, "127.0.0.1", resolve);
  });
  const apiAddress = apiServer.address();
  if (!apiAddress || typeof apiAddress === "string") {
    throw new Error("renderer smoke API server did not expose a TCP port");
  }
  const responses = {
    "amitia:deployment:get": { mode: "local" },
    "amitia:runtime:endpoint": { httpBaseURL: `http://127.0.0.1:${apiAddress.port}` },
    "amitia:environment:get": { platform: process.platform, arch: process.arch, version: "smoke" },
    "amitia:backend-auth:get-headers": {},
    "amitia:mesh:identity:get": { deviceId: "renderer-smoke" },
    "amitia:window:state": { maximized: false, fullscreen: false },
    "update:get-pending": null,
  };
  for (const [channel, response] of Object.entries(responses)) {
    ipcMain.handle(channel, () => response);
  }
  protocol.handle("app", async (request) => {
    const url = new URL(request.url);
    if (url.pathname.startsWith("/api/")) {
      const data = url.pathname.endsWith("/capabilities")
        ? { runtimeProfile: "local", capabilities: {} }
        : { completed: true };
      return Response.json({ data });
    }
    const file = path.resolve(root, `.${url.pathname === "/" ? "/index.html" : url.pathname}`);
    if (!file.startsWith(root + path.sep)) return new Response(null, { status: 403 });
    try {
      const content = await readFile(file);
      const type = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".svg": "image/svg+xml" }[path.extname(file)] ?? "application/octet-stream";
      return new Response(content, { headers: { "Content-Type": type } });
    } catch (error) {
  console.error("[original-error]", error);
return new Response(null, { status: 404 });
    }
  });
  win = new BrowserWindow({
    show: false,
    webPreferences: { preload: path.resolve(__dirname, "../dist/preload/index.cjs"), sandbox: false, contextIsolation: true },
  });
  win.webContents.on("dom-ready", async () => {
    await win.webContents.executeJavaScript(`(() => {
      window.__amitiaSmokeErrors = [];
      const describe = (value) => {
        if (value instanceof Error) return value.name + ": " + value.message + "\\n" + (value.stack || "");
        if (value && typeof value === "object") {
          try {
            return JSON.stringify(value, (_key, item) => {
              if (item instanceof Error) return { name: item.name, message: item.message, stack: item.stack, cause: item.cause };
              return item;
            });
          } catch (error) {
            return String(value);
          }
        }
        return String(value);
      };
      const originalError = console.error.bind(console);
      console.error = (...args) => {
        window.__amitiaSmokeErrors.push(args.map(describe).join(" "));
        originalError(...args);
      };
      window.addEventListener("error", (event) => {
        window.__amitiaSmokeErrors.push(describe(event.error || event.message));
      });
      window.addEventListener("unhandledrejection", (event) => {
        window.__amitiaSmokeErrors.push(describe(event.reason));
      });
    })()`);
  });
  win.webContents.on("console-message", (_event, detailsOrLevel, message, line, sourceId) => {
    const details = typeof detailsOrLevel === "object" && detailsOrLevel
      ? detailsOrLevel
      : { level: detailsOrLevel, message, lineNumber: line, sourceId };
    const level = String(details.level || "").toLowerCase();
    if (level === "error" || Number(details.level) >= 3) {
      errors.push(JSON.stringify(details));
    }
  });
  win.webContents.on("preload-error", (_event, _file, error) => finish(error));
  win.webContents.on("render-process-gone", (_event, details) => finish(details.reason));
  win.webContents.on("did-fail-load", (_event, code, description, url) => {
    finish(`renderer load failed: ${code} ${description} ${url}`);
  });
  await win.loadURL("app://amitia/#/privacy");
  const mounted = await win.webContents.executeJavaScript(`new Promise((resolve) => {
    const check = () => {
      const root = document.getElementById('app');
      if (root?.dataset.amitiaMounted === 'true') { observer.disconnect(); resolve(true); }
      else if (root?.querySelector('[role="alert"]')) { observer.disconnect(); resolve(false); }
    };
    const observer = new MutationObserver(check);
    observer.observe(document.body, { subtree: true, childList: true, attributes: true });
    check();
  })`);
  const rendererErrors = await win.webContents.executeJavaScript("window.__amitiaSmokeErrors || []");
  const allErrors = [...errors, ...rendererErrors];
  finish(mounted && allErrors.length === 0 ? null : allErrors.join("\n---\n") || "startup fallback displayed");
}).catch(finish);

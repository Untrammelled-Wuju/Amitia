const { app, BrowserWindow, ipcMain, protocol } = require("electron");
const { readFile } = require("node:fs/promises");
const { mkdtempSync, rmSync } = require("node:fs");
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

function finish(error) {
  if (finished) return;
  finished = true;
  clearTimeout(timeout);
  if (error) console.error("[renderer-smoke] FAILED:", error);
  else console.log("[renderer-smoke] PASSED: production renderer mounted");
  win?.destroy();
  app.exit(error ? 1 : 0);
}

const timeout = setTimeout(() => finish("renderer did not mount within 20 seconds"), 20000);
process.once("exit", () => {
  try { rmSync(data, { recursive: true, force: true }); } catch {}
});

app.whenReady().then(async () => {
  const responses = {
    "amitia:deployment:get": { mode: "local" },
    "amitia:runtime:endpoint": { httpBaseURL: "app://amitia" },
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
    } catch {
      return new Response(null, { status: 404 });
    }
  });
  win = new BrowserWindow({
    show: false,
    webPreferences: { preload: path.resolve(__dirname, "../dist/preload/index.cjs"), sandbox: false, contextIsolation: true },
  });
  win.webContents.on("console-message", (_event, level, message) => {
    if (level >= 3) errors.push(message);
  });
  win.webContents.on("preload-error", (_event, _file, error) => finish(error));
  win.webContents.on("render-process-gone", (_event, details) => finish(details.reason));
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
  finish(mounted && errors.length === 0 ? null : errors.join("\n") || "startup fallback displayed");
}).catch(finish);

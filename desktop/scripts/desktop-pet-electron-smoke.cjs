"use strict";

const path = require("node:path");
const fs = require("node:fs");
const { app, BrowserWindow, ipcMain } = require("electron");

const CHANNELS = Object.freeze({
  getPackageSnapshot: "pet:animation:get-package-snapshot",
  resolveResourceUrl: "pet:animation:resolve-resource-url",
  getDiagnostics: "pet:animation:get-diagnostics",
  rendererBootstrapped: "pet:animation:renderer-bootstrapped",
  click: "pet:animation:click",
  dragStart: "pet:animation:drag-start",
});

const TIMEOUT_MS = 15000;
let windowRef = null;
let timeoutHandle = null;
let finished = false;

function fail(message, error) {
  if (finished) return;
  finished = true;
  if (timeoutHandle) clearTimeout(timeoutHandle);
  console.error(`[desktop-pet-electron-smoke] FAIL: ${message}`);
  if (error) console.error(error);
  app.exit(1);
}

function pass() {
  if (finished) return;
  finished = true;
  if (timeoutHandle) clearTimeout(timeoutHandle);
  console.log("[desktop-pet-electron-smoke] PASS: isolated pet renderer/preload IPC round-trip verified");
  app.exit(0);
}

app.commandLine.appendSwitch("disable-gpu");

app.whenReady().then(async () => {
  const desktopRoot = path.resolve(__dirname, "..");
  const petHtml = path.join(desktopRoot, "dist", "renderer", "pet.html");
  const preload = path.join(desktopRoot, "dist", "preload", "animation-preload.cjs");

  if (!fs.existsSync(petHtml)) {
    fail(`missing built pet renderer: ${petHtml}`);
    return;
  }
  if (!fs.existsSync(preload)) {
    fail(`missing canonical animation preload: ${preload}`);
    return;
  }

  const observed = {
    bootstrapped: false,
    click: false,
    dragStart: false,
  };

  ipcMain.handle(CHANNELS.getPackageSnapshot, () => null);
  ipcMain.handle(CHANNELS.resolveResourceUrl, (_event, relativePath) => ({
    url: String(relativePath ?? ""),
    mime: "application/octet-stream",
  }));
  ipcMain.handle(CHANNELS.getDiagnostics, () => ({ smoke: true }));

  ipcMain.on(CHANNELS.rendererBootstrapped, () => {
    observed.bootstrapped = true;
  });
  ipcMain.on(CHANNELS.click, (_event, payload) => {
    if (payload?.x === 17 && payload?.y === 29) observed.click = true;
  });
  ipcMain.on(CHANNELS.dragStart, (_event, payload) => {
    if (
      payload?.pointerId === 7 &&
      payload?.screenX === 31 &&
      payload?.screenY === 47 &&
      payload?.canvasX === 11 &&
      payload?.canvasY === 13
    ) {
      observed.dragStart = true;
    }
  });

  windowRef = new BrowserWindow({
    show: false,
    width: 320,
    height: 320,
    transparent: true,
    frame: false,
    webPreferences: {
      preload,
      sandbox: true,
      nodeIntegration: false,
      contextIsolation: true,
      webSecurity: true,
    },
  });

  windowRef.webContents.on("render-process-gone", (_event, details) => {
    fail(`renderer process exited (${details.reason}, code ${details.exitCode})`);
  });
  windowRef.webContents.on("preload-error", (_event, preloadPath, error) => {
    fail(`preload failed: ${preloadPath}`, error);
  });
  windowRef.webContents.on("did-fail-load", (_event, code, description) => {
    fail(`pet renderer failed to load (${code}): ${description}`);
  });

  timeoutHandle = setTimeout(() => {
    fail(`timed out; observations=${JSON.stringify(observed)}`);
  }, TIMEOUT_MS);

  await windowRef.loadFile(petHtml);

  const apiShape = await windowRef.webContents.executeJavaScript(`(() => {
    const api = window.petAnimationApi;
    return {
      exists: Boolean(api),
      sendClick: typeof api?.sendClick === "function",
      sendDragStart: typeof api?.sendDragStart === "function",
      getDiagnostics: typeof api?.getDiagnostics === "function",
      requireVisible: typeof window.require !== "undefined"
    };
  })()`);

  if (!apiShape.exists || !apiShape.sendClick || !apiShape.sendDragStart || !apiShape.getDiagnostics) {
    fail(`canonical preload API unavailable: ${JSON.stringify(apiShape)}`);
    return;
  }
  if (apiShape.requireVisible) {
    fail("renderer unexpectedly exposes window.require with nodeIntegration disabled");
    return;
  }

  const diagnostics = await windowRef.webContents.executeJavaScript(
    "window.petAnimationApi.getDiagnostics()",
  );
  if (!diagnostics || diagnostics.smoke !== true) {
    fail(`preload invoke round-trip failed: ${JSON.stringify(diagnostics)}`);
    return;
  }

  await windowRef.webContents.executeJavaScript(`(() => {
    window.petAnimationApi.sendClick(17, 29);
    window.petAnimationApi.sendDragStart({
      pointerId: 7,
      screenX: 31,
      screenY: 47,
      canvasX: 11,
      canvasY: 13,
      occurredAt: Date.now()
    });
  })()`);

  const deadline = Date.now() + 3000;
  while (Date.now() < deadline) {
    if (observed.bootstrapped && observed.click && observed.dragStart) {
      pass();
      return;
    }
    await new Promise((resolve) => setTimeout(resolve, 25));
  }

  fail(`IPC observations incomplete: ${JSON.stringify(observed)}`);
}).catch((error) => fail("uncaught smoke-test error", error));

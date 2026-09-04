"use strict";

const path = require("node:path");
const fs = require("node:fs");
const {
  app,
  BrowserWindow,
  ipcMain,
  protocol,
} = require("electron");

const PET_SCHEME = "amitia-pet";
const PACKAGE_ID = "desktop-pet-golden-smoke";
const PACKAGE_REVISION = 7;
const INSTALLATION_ID = "smoke-install";
const PET_INSTANCE_ID = "smoke-pet";
const TIMEOUT_MS = 20000;

const CHANNELS = Object.freeze({
  getPackageSnapshot: "pet:animation:get-package-snapshot",
  resolveResourceUrl: "pet:animation:resolve-resource-url",
  reportEvent: "pet:animation:report-event",
  reportSnapshot: "pet:animation:report-snapshot",
  getDiagnostics: "pet:animation:get-diagnostics",
  rendererBootstrapped: "pet:animation:renderer-bootstrapped",
  runtimeReady: "pet:animation:runtime-ready",
  runtimeInitFailed: "pet:animation:runtime-init-failed",
  hitMask: "pet:animation:hit-mask",
  click: "pet:animation:click",
  dragStart: "pet:animation:drag-start",
  dragMove: "pet:animation:drag-move",
  dragEnd: "pet:animation:drag-end",
  playAction: "pet:animation:play-action",
  windowShown: "pet:animation:window-shown",
});

protocol.registerSchemesAsPrivileged([
  {
    scheme: PET_SCHEME,
    privileges: {
      standard: true,
      secure: true,
      supportFetchAPI: true,
      corsEnabled: true,
    },
  },
]);

const pngBytes = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAYAAABytg0kAAAAFUlEQVR4nGP8////fwYGBgYmBigAAD34BADaOyqcAAAAAElFTkSuQmCC",
  "base64",
);

function actionConfig(actionKey, options) {
  return {
    actionKey,
    displayName: options.displayName,
    version: 2,
    loopType: options.loopType,
    playbackMode: options.loopType,
    fps: 20,
    frameDurationMs: options.frameDurationMs,
    frameCount: options.frames,
    frames: Array.from({ length: options.frames }, (_, index) => ({
      index,
      file: `frame-${index}.png`,
      durationMs: options.frameDurationMs,
      frameId: `${actionKey}-frame-${index}`,
      assetId: `${actionKey}-asset-${index}`,
      contentHash: `smoke-${actionKey}-${index}`,
    })),
    anchor: {
      type: "bottom_center",
      x: 0.5,
      y: 1,
      coordinateSpace: "normalized_canvas",
    },
    interruptible: true,
    interruptAfterMs: 0,
    minimumPlayMs: 0,
    maximumPlayMs: null,
    defaultPriority: options.priority,
    priority: options.priority,
    cooldownMs: 0,
    mutexGroup: "smoke",
    returnTo: options.returnTo,
    supportsDefaultIdle: options.supportsDefaultIdle,
    isStableStateCandidate: options.isStableStateCandidate,
    isTransitionOnly: options.isTransitionOnly,
  };
}

const idleConfig = actionConfig("idle", {
  displayName: "Golden Idle",
  loopType: "loop",
  frameDurationMs: 80,
  frames: 1,
  priority: 10,
  returnTo: { type: "none" },
  supportsDefaultIdle: true,
  isStableStateCandidate: true,
  isTransitionOnly: false,
});

const waveConfig = actionConfig("wave", {
  displayName: "Golden Wave",
  loopType: "once",
  frameDurationMs: 70,
  frames: 2,
  priority: 90,
  returnTo: { type: "default" },
  supportsDefaultIdle: false,
  isStableStateCandidate: false,
  isTransitionOnly: true,
});

const packageSnapshot = Object.freeze({
  packageId: PACKAGE_ID,
  packageRevision: PACKAGE_REVISION,
  schemaVersion: 2,
  canvas: { width: 64, height: 64 },
  defaultActionKey: "idle",
  interpolationMode: "nearest",
  actions: [
    {
      actionKey: "idle",
      configUrl: `${PET_SCHEME}://installation/${INSTALLATION_ID}/actions/idle/action.json`,
    },
    {
      actionKey: "wave",
      configUrl: `${PET_SCHEME}://installation/${INSTALLATION_ID}/actions/wave/action.json`,
    },
  ],
});

let windowRef = null;
let timeoutHandle = null;
let finished = false;

const observed = {
  bootstrapped: 0,
  runtimeReady: 0,
  runtimeInitFailed: [],
  events: [],
  snapshots: [],
  hitMasks: 0,
  click: false,
  dragStart: false,
  dragMove: false,
  dragEnd: false,
};

function fail(message, error) {
  if (finished) return;
  finished = true;
  if (timeoutHandle) clearTimeout(timeoutHandle);
  console.error(`[desktop-pet-electron-smoke] FAIL: ${message}`);
  if (error) console.error(error);
  console.error(`[desktop-pet-electron-smoke] observations=${JSON.stringify(observed)}`);
  app.exit(1);
}

function pass() {
  if (finished) return;
  finished = true;
  if (timeoutHandle) clearTimeout(timeoutHandle);
  console.log(
    "[desktop-pet-electron-smoke] PASS: real Package V2 -> RuntimeReady -> first frame -> action switch/completion -> hit mask -> click/drag -> renderer reload recovery verified",
  );
  app.exit(0);
}

function jsonResponse(value) {
  return new Response(JSON.stringify(value), {
    status: 200,
    headers: {
      "Content-Type": "application/json; charset=utf-8",
      "Access-Control-Allow-Origin": "*",
      "Cache-Control": "no-store",
    },
  });
}

function binaryResponse(bytes, mediaType) {
  return new Response(new Uint8Array(bytes), {
    status: 200,
    headers: {
      "Content-Type": mediaType,
      "Access-Control-Allow-Origin": "*",
      "Cache-Control": "no-store",
    },
  });
}

function installGoldenResourceProtocol() {
  protocol.handle(PET_SCHEME, async (request) => {
    const url = request.url;
    if (url.endsWith(`/${INSTALLATION_ID}/actions/idle/action.json`)) {
      return jsonResponse(idleConfig);
    }
    if (url.endsWith(`/${INSTALLATION_ID}/actions/wave/action.json`)) {
      return jsonResponse(waveConfig);
    }
    if (
      url.includes(`/${INSTALLATION_ID}/actions/idle/frame-`) ||
      url.includes(`/${INSTALLATION_ID}/actions/wave/frame-`)
    ) {
      return binaryResponse(pngBytes, "image/png");
    }
    return new Response(JSON.stringify({ error: "smoke_resource_not_found", url }), {
      status: 404,
      headers: {
        "Content-Type": "application/json; charset=utf-8",
        "Access-Control-Allow-Origin": "*",
      },
    });
  });
}

async function waitUntil(label, predicate, timeoutMs = 5000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (predicate()) return;
    await new Promise((resolve) => setTimeout(resolve, 25));
  }
  throw new Error(`timed out waiting for ${label}`);
}

function hasEvent(type, actionKey) {
  return observed.events.some(
    (event) => event?.type === type && (actionKey === undefined || event?.actionKey === actionKey),
  );
}

function registerIpc() {
  ipcMain.handle(CHANNELS.getPackageSnapshot, () => packageSnapshot);
  ipcMain.handle(CHANNELS.resolveResourceUrl, (_event, relativePath) => ({
    url: String(relativePath ?? ""),
    mime: "application/octet-stream",
  }));
  ipcMain.handle(CHANNELS.getDiagnostics, () => ({ smoke: true, packageId: PACKAGE_ID }));

  ipcMain.on(CHANNELS.rendererBootstrapped, () => {
    observed.bootstrapped += 1;
  });
  ipcMain.on(CHANNELS.runtimeReady, (_event, payload) => {
    if (
      payload?.snapshotApplied === true &&
      payload?.packageId === PACKAGE_ID &&
      payload?.packageRevision === PACKAGE_REVISION &&
      payload?.defaultActionKey === "idle"
    ) {
      observed.runtimeReady += 1;
    }
  });
  ipcMain.on(CHANNELS.runtimeInitFailed, (_event, payload) => {
    observed.runtimeInitFailed.push(payload ?? { reason: "unknown" });
  });
  ipcMain.on(CHANNELS.reportEvent, (_event, payload) => {
    observed.events.push(payload);
  });
  ipcMain.on(CHANNELS.reportSnapshot, (_event, payload) => {
    observed.snapshots.push(payload);
  });
  ipcMain.on(CHANNELS.hitMask, (_event, payload) => {
    if (
      payload?.packageRevision === PACKAGE_REVISION &&
      Number(payload?.width) > 0 &&
      Number(payload?.height) > 0
    ) {
      observed.hitMasks += 1;
    }
  });
  ipcMain.on(CHANNELS.click, (_event, payload) => {
    if (Number.isFinite(payload?.x) && Number.isFinite(payload?.y)) observed.click = true;
  });
  ipcMain.on(CHANNELS.dragStart, (_event, payload) => {
    if (payload?.pointerId === 7) observed.dragStart = true;
  });
  ipcMain.on(CHANNELS.dragMove, (_event, payload) => {
    if (payload?.pointerId === 7) observed.dragMove = true;
  });
  ipcMain.on(CHANNELS.dragEnd, (_event, payload) => {
    if (payload?.pointerId === 7) observed.dragEnd = true;
  });
}

async function assertPreloadApi() {
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
    throw new Error(`canonical preload API unavailable: ${JSON.stringify(apiShape)}`);
  }
  if (apiShape.requireVisible) {
    throw new Error("renderer unexpectedly exposes window.require with nodeIntegration disabled");
  }

  const diagnostics = await windowRef.webContents.executeJavaScript(
    "window.petAnimationApi.getDiagnostics()",
  );
  if (!diagnostics || diagnostics.smoke !== true || diagnostics.packageId !== PACKAGE_ID) {
    throw new Error(`preload invoke round-trip failed: ${JSON.stringify(diagnostics)}`);
  }
}

async function exerciseInteractions() {
  await windowRef.webContents.executeJavaScript(`(() => {
    const canvas = document.getElementById("pet-canvas");
    if (!canvas) throw new Error("pet canvas missing");
    canvas.dispatchEvent(new MouseEvent("click", {
      bubbles: true,
      clientX: 17,
      clientY: 29,
      screenX: 17,
      screenY: 29
    }));
    const emitPointer = (type, screenX, screenY) => canvas.dispatchEvent(new PointerEvent(type, {
      bubbles: true,
      pointerId: 7,
      clientX: screenX,
      clientY: screenY,
      screenX,
      screenY
    }));
    emitPointer("pointerdown", 20, 20);
    emitPointer("pointermove", 31, 47);
    emitPointer("pointermove", 35, 50);
    emitPointer("pointerup", 40, 55);
  })()`);
  await waitUntil(
    "click/drag IPC",
    () => observed.click && observed.dragStart && observed.dragMove && observed.dragEnd,
    4000,
  );
}

async function exerciseActionSwitch() {
  const now = Date.now();
  windowRef.webContents.send(CHANNELS.playAction, {
    commandId: `smoke-wave-${now}`,
    idempotencyKey: `smoke-wave-${now}`,
    installationId: INSTALLATION_ID,
    petInstanceId: PET_INSTANCE_ID,
    packageRevision: PACKAGE_REVISION,
    actionKey: "wave",
    priority: 90,
    queuePolicy: "replace_current",
    interruptPolicy: "force_system",
    playbackRate: 1,
    issuedAt: new Date(now).toISOString(),
    expiresAt: new Date(now + 5000).toISOString(),
    returnOverride: { type: "default" },
    traceId: "desktop-pet-golden-smoke",
    source: "release_gate",
  });

  await waitUntil(
    "wave action start",
    () => hasEvent("playback.action_started", "wave"),
    5000,
  );
  await waitUntil(
    "wave action completion",
    () => hasEvent("playback.action_completed", "wave"),
    5000,
  );
}

app.commandLine.appendSwitch("disable-gpu");

app.whenReady().then(async () => {
  installGoldenResourceProtocol();
  registerIpc();

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
    fail("global smoke timeout");
  }, TIMEOUT_MS);

  await windowRef.loadFile(petHtml);
  await assertPreloadApi();

  await waitUntil(
    "first RuntimeReady",
    () => observed.runtimeReady >= 1 && observed.runtimeInitFailed.length === 0,
    6000,
  );
  await waitUntil(
    "default action first frame",
    () => hasEvent("playback.action_started", "idle"),
    4000,
  );
  await waitUntil(
    "renderer hit mask",
    () => observed.hitMasks > 0,
    5000,
  );

  // Hidden BrowserWindows are deliberately frozen by the production engine.
  // Exercise the canonical visibility IPC before asserting time-based playback.
  windowRef.webContents.send(CHANNELS.windowShown);

  await exerciseInteractions();
  await exerciseActionSwitch();

  if (observed.snapshots.length === 0) {
    throw new Error("renderer did not report any playback snapshot");
  }

  const readyBeforeReload = observed.runtimeReady;
  const idleStartsBeforeReload = observed.events.filter(
    (event) => event?.type === "playback.action_started" && event?.actionKey === "idle",
  ).length;

  windowRef.webContents.reload();
  await waitUntil(
    "RuntimeReady after renderer reload",
    () => observed.runtimeReady > readyBeforeReload && observed.runtimeInitFailed.length === 0,
    6000,
  );
  await waitUntil(
    "default action recovery after renderer reload",
    () => observed.events.filter(
      (event) => event?.type === "playback.action_started" && event?.actionKey === "idle",
    ).length > idleStartsBeforeReload,
    5000,
  );

  pass();
}).catch((error) => fail("uncaught golden smoke-test error", error));
